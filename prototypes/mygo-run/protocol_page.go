package main

// Protocol is surface 6 (UX-IA-5): a read-only projection of a plan. It renders
// two distinct objects (UX-STATE): a Run-backed document is 已冻结; preview is 预览.

import (
	"context"
	"fmt"
	"time"

	"asterlab-mygo-prototype/internal/asterdoc"
	"asterlab-mygo-prototype/internal/notebook/record"
	"asterlab-mygo-prototype/internal/protocol"
	"asterlab-mygo-prototype/modules/runs"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"
	"github.com/oneo-me/onevra/source/components"
)

// protocolArchives keys the document/archive view toggle by Run, so switching
// Runs never leaks one Run's 终态归档 view into another.
var protocolArchives = map[string]bool{}

type protocolPageState struct {
	selectedVersionID string
	selectedByUser    bool
	designID          string
	revision          uint64
	source            string
	loaded            bool
}

var protocolPageStates = map[*app]*protocolPageState{}

func protocolState(a *app) *protocolPageState {
	state := protocolPageStates[a]
	if state == nil {
		state = &protocolPageState{}
		protocolPageStates[a] = state
	}
	return state
}

func protocolSummaries(items []runs.ProtocolSummary) []protocol.VersionSummary {
	versions := make([]protocol.VersionSummary, 0, len(items))
	for _, item := range items {
		versions = append(versions, protocol.VersionSummary{
			ID: item.ProtocolVersionID, Title: item.Title, Version: item.Version,
		})
	}
	return versions
}

func protocolVersionID(source string) string {
	doc, err := asterdoc.Parse(source)
	if err != nil {
		return ""
	}
	for _, module := range doc.Modules {
		if module.Kind == "assay" {
			if id, ok := module.Data["protocol_version_id"].(string); ok {
				return id
			}
		}
	}
	return ""
}

func hasProtocolVersion(source, versionID string) bool {
	if versionID == "" {
		return false
	}
	doc, err := asterdoc.Parse(source)
	if err != nil {
		return false
	}
	for _, module := range doc.Modules {
		if module.Kind == "assay" && module.Data["protocol_version_id"] == versionID {
			return true
		}
	}
	return false
}

func protocolRevision(a *app) (string, uint64) {
	designID := ""
	if a.shell != nil {
		designID = a.shell.Selected
	}
	if a.designDoc != nil && a.designDoc.DesignID == designID {
		return designID, a.designDoc.Revision
	}
	if a.designer != nil {
		return designID, a.designer.Revision
	}
	return designID, a.snap.DesignRevision
}

func loadProtocolPreview(a *app, state *protocolPageState, designID string, revision uint64) {
	if state.loaded && state.designID == designID && state.revision == revision {
		return
	}
	state.loaded = true
	state.designID, state.revision, state.source = designID, revision, ""
	if a.designSvc == nil || designID == "" || revision == 0 {
		return
	}
	doc, err := a.designSvc.LoadAsterDocAt(context.Background(), designID, revision)
	if err == nil && doc.Revision == revision && doc.DesignID == designID {
		state.source = doc.Source
	}
}

func shortID(id string) string {
	if len(id) > 8 {
		return id[:8]
	}
	return id
}

func protocolPage(c *ui.Context, a *app) {
	state := protocolState(a)
	versions := protocolSummaries(a.protocols)
	frozen := a.snap.RunID != "" && a.snap.AsterDoc != nil
	frozenSource := ""
	if frozen && a.asterDoc != nil {
		frozenSource = a.asterDoc.Source
	}

	designID, revision := protocolRevision(a)
	if !frozen && a.snap.RunID == "" {
		loadProtocolPreview(a, state, designID, revision)
	}
	resolvedSource := frozenSource
	if !frozen {
		resolvedSource = state.source
	}
	referencedVersion := protocolVersionID(resolvedSource)
	if !state.selectedByUser {
		if referencedVersion != "" {
			state.selectedVersionID = referencedVersion
		} else if state.selectedVersionID == "" && len(versions) > 0 {
			state.selectedVersionID = versions[0].ID
		}
	}

	t := c.Theme()
	ui.Column(c).Fill().Children(func() {
		ui.Row(c).FillWidth().Padding(t.Space(2), t.Space(4)).Gap(t.Space(2)).
			AlignItems(ui.Center).Children(func() {
			ui.Text(c, "Protocol versions · 只读").FontSize(t.Rem(.75)).TextColor(t.TextMuted)
			protocol.RenderVersions(c, versions, state.selectedVersionID, func(id string) {
				state.selectedByUser = true
				state.selectedVersionID = id
			})
		})

		// UX-STATE: a Run remains frozen even when a listed version is unresolved.
		if frozen {
			protocol.RenderVersionState(c, "frozen", shortID(a.snap.RunID), a.snap.AsterDoc.Revision)
			showArchive := protocolArchives[a.snap.RunID]
			ui.Row(c).FillWidth().Padding(t.Space(1), t.Space(4)).Gap(t.Space(1.5)).
				AlignItems(ui.Center).Children(func() {
				docStyle, archStyle := components.ButtonSecondary, components.ButtonGhost
				if showArchive {
					docStyle, archStyle = archStyle, docStyle
				}
				if components.Button(c, "冻结文档", docStyle).Clicked() {
					protocolArchives[a.snap.RunID] = false
				}
				if components.Button(c, "终态归档", archStyle).Clicked() {
					protocolArchives[a.snap.RunID] = true
				}
			})
			if showArchive {
				a.renderProtocolArchive(c)
				return
			}
			if a.asterDoc == nil && a.asterErr == "" {
				ui.Text(c, "正在读取冻结文档…").FontSize(t.Rem(.85)).TextColor(t.TextMuted)
				return
			}
			if !hasProtocolVersion(resolvedSource, state.selectedVersionID) {
				renderUnresolvedProtocol(c)
				return
			}
			a.documentExportButton(c)
			if a.asterErr != "" {
				ui.Column(c).Grow(1).Padding(t.Space(8)).Gap(t.Space(2)).Children(func() {
					ui.Text(c, "冻结文档无法读取。").FontSize(t.Rem(.95)).Bold().TextColor(t.Danger)
					ui.Text(c, a.asterErr).FontSize(t.Rem(.82)).TextColor(t.TextMuted)
				})
				return
			}
			renderProtocolDocument(c, a.asterDoc.Source, asterdoc.ModeProtocol, a.asterDoc.Revision)
			return
		}

		if a.snap.RunID == "" {
			// UX-STATE: every design-only view names its revision as a preview.
			protocol.RenderVersionState(c, "preview", "", revision)
			if state.selectedVersionID == "" || state.source == "" || !hasProtocolVersion(state.source, state.selectedVersionID) {
				renderUnresolvedProtocol(c)
				return
			}
			renderProtocolDocument(c, state.source, asterdoc.ModeProtocol, revision)
			return
		}

		protocol.RenderVersionState(c, "frozen", shortID(a.snap.RunID), a.snap.DesignRevision)
		if a.snap.DesignData == "" {
			ui.Column(c).Grow(1).Padding(t.Space(8)).Gap(t.Space(2)).Children(func() {
				ui.Text(c, "已选择 Run，但它的冻结设计缺失。").FontSize(t.Rem(.95)).Bold().TextColor(t.Danger)
				ui.Text(c, "不会改用实时设计；请检查该运行的设计快照。").
					FontSize(t.Rem(.82)).TextColor(t.TextMuted)
			})
			return
		}
		// Runs without the frozen AsterDoc pointer cannot resolve Protocol content.
		renderUnresolvedProtocol(c)
	})
}

func renderUnresolvedProtocol(c *ui.Context) {
	// UX-P5 / UX-STATE: missing content is explicit, not a fallback or error banner.
	protocol.RenderVersionState(c, "unresolved", "", 0)
	ui.Text(c, "Protocol 内容查询缺失。").FontSize(c.Theme().Rem(.82)).TextColor(c.Theme().TextMuted)
}

func renderProtocolDocument(c *ui.Context, source string, mode asterdoc.Mode, revision uint64) {
	doc, err := asterdoc.Parse(source)
	if err != nil {
		renderUnresolvedProtocol(c)
		return
	}
	t := c.Theme()
	// UX-P4 / UX-LAYOUT: document mode is centered and capped at 820px.
	ui.Scroll(c).Fill().Children(func() {
		ui.Column(c).FillWidth().Padding(t.Space(6), t.Space(6)).AlignItems(ui.Center).
			Children(func() {
				ui.Box(c).FillWidth().MaxWidth(820).MaxWidthPercent(100).
					Radius(t.Radius).Border(1, t.Border).Background(t.Surface).
					Padding(t.Space(7), t.Space(8)).Children(func() {
					view := &asterdoc.View{
						Doc: doc, Registry: asterdoc.BuiltinRegistry(), Mode: mode,
						ReadOnly: true, DocumentRevision: fmt.Sprint(revision),
					}
					index := 0
					protocol.RenderMarkdownWith(c, source, func(c *ui.Context, block *markdown.CodeBlockView) {
						if block.Language != "asterlab" {
							markdown.DefaultCodeBlock(c, block)
							return
						}
						if index >= len(doc.Modules) {
							return
						}
						module := doc.Modules[index]
						index++
						// UX-P2: references and workflow IDs are navigation metadata, not prose.
						if module.Kind == "reference" || module.Kind == "workflow" {
							return
						}
						data := make(map[string]any, len(module.Data))
						for key, value := range module.Data {
							data[key] = value
						}
						delete(data, "id")
						if module.Kind == "step" {
							delete(data, "assay")
							delete(data, "path")
						}
						view.RenderModule(c, module.Kind, module.Schema, data)
					})
				})
			})
	})
}

// renderProtocolArchive renders the 终态归档: the shared RecordProjection as
// an exportable end state, with the signed prose (dynamic references
// flattened) and the derived design draft. It reads the same projection the
// Notebook board and the Run history read from; it writes nothing.
func (a *app) renderProtocolArchive(c *ui.Context) {
	t := c.Theme()
	proj := record.Project(a.snap, a.runAssets)
	prose := ""
	if a.noteRunID == a.snap.RunID {
		prose = a.noteSource()
	} else if latest, ok := latestNotebookRevision(a.snap.Events); ok {
		prose = latest
	}
	_, sigOK := record.Verify(prose)
	arch := protocol.BuildArchive(proj, prose, time.Now())
	state := record.ClassifyRecordState(a.noteDirty, sigOK)
	ui.Column(c).Grow(1).FillWidth().Padding(t.Space(2), t.Space(4)).Gap(t.Space(2)).Children(func() {
		ui.Row(c).FillWidth().Wrap().AlignItems(ui.Center).Gap(t.Space(1.5)).Children(func() {
			ui.Text(c, "终态归档").FontSize(t.Rem(.95)).Bold().TextColor(t.Text)
			ui.Text(c, string(state)).FontSize(t.Rem(.8)).Bold().TextColor(t.TextMuted)
			ui.Spacer(c)
			if components.Button(c, "导出归档包", components.ButtonSecondary).Clicked() {
				a.exportRecordFile(c, "导出归档包", "record-"+a.snap.RunID+".md", arch.Markdown())
			}
		})
		if sigOK {
			if arch.Signature != nil {
				ui.Text(c, "签核 "+arch.Signature.Actor+" · "+arch.Signature.At).
					FontSize(t.Rem(.8)).TextColor(t.TextMuted)
			}
		} else {
			ui.Text(c, "未签核：签核在 Notebook 图板完成，归档包随之锁定。").
				FontSize(t.Rem(.8)).TextColor(t.TextMuted)
		}
		ui.Scroll(c).Grow(1).Children(func() {
			ui.Column(c).FillWidth().AlignItems(ui.Center).Children(func() {
				ui.Box(c).FillWidth().MaxWidth(820).MaxWidthPercent(100).
					Radius(t.Radius).Border(1, t.Border).Background(t.Surface).
					Padding(t.Space(7), t.Space(8)).Children(func() {
					protocol.RenderMarkdown(c, arch.Markdown())
				})
			})
		})
	})
}
