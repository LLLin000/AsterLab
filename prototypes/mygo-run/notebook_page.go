package main

// Notebook surface: prose over the stored Markdown of one Run. Owned by the
// Notebook/Protocol Owner after Wave 1 extraction.

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"asterlab-mygo-prototype/internal/asterdoc"
	"asterlab-mygo-prototype/internal/notebook"
	"asterlab-mygo-prototype/internal/protocol"
	"asterlab-mygo-prototype/internal/resultgrid"
	"asterlab-mygo-prototype/internal/state"
	"asterlab-mygo-prototype/internal/theme"
	"asterlab-mygo-prototype/modules/runs"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/richtext"
	"github.com/oneo-me/onevra/source/components"
	"github.com/oneo-me/onevra/source/icons/lucide"
)

type citePickerState struct {
	open           bool
	manual         bool
	filter         string
	dismissedToken string
}

var citePickers = map[*app]*citePickerState{}

func (a *app) citePicker() *citePickerState {
	st := citePickers[a]
	if st == nil {
		st = &citePickerState{filter: "all"}
		citePickers[a] = st
	}
	return st
}

// noteViewState holds the Notebook surface's own view state: which projection
// is in front (prose paper or Run timeline), and the entry a citation jumped
// to. It is derived state only — no prose, no facts are stored here.
type noteViewState struct {
	// split is the active projection at narrow widths. At >=1100 both sections
	// are shown side by side, so split is ignored there.
	split       int
	highlightID string
	scrolledID  string
}

var noteViews = map[*app]*noteViewState{}

func (a *app) noteView() *noteViewState {
	st := noteViews[a]
	if st == nil {
		st = &noteViewState{}
		noteViews[a] = st
	}
	return st
}

// notebookPage follows Obsidian's shape: one paper sheet, a floating toolbar,
// and four modes over the same stored Markdown — live rich rendering, source
// editing, reading, and the AsterML document. Each save appends one notebook-stage event.
func notebookPage(c *ui.Context, a *app) {
	t := c.Theme()
	if a.snap.RunID == "" {
		ui.Column(c).Fill().Center().Gap(t.Space(2)).Children(func() {
			ui.Text(c, "Notebook").FontSize(20).Bold().TextColor(t.Text)
			ui.Text(c, "先选择或加载一个 Run，笔记会记录在它的执行历史中。").
				FontSize(13).TextColor(t.TextMuted)
		})
		return
	}
	if a.noteRunID != a.snap.RunID {
		a.loadNotebook()
	}
	ui.Column(c).Fill().Padding(t.Space(2), t.Space(3)).Gap(t.Space(2)).Children(func() {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(t.Space(1.5)).Padding(0, t.Space(1)).Children(func() {
			ui.Text(c, "Notebook").FontSize(11).Bold().TextColor(t.TextMuted)
			ui.Text(c, "/").FontSize(11).TextColor(t.Border)
			ui.Text(c, a.snap.Title).FontSize(11).TextColor(t.TextMuted).
				SingleLine().Ellipsis("…").MaxWidth(420)
			if a.snap.AsterDoc == nil && a.snap.DesignRevision > 0 {
				ui.Text(c, fmt.Sprintf("Document Revision: %d", a.snap.DesignRevision)).
					FontSize(10).TextColor(t.TextMuted)
			}
			ui.Spacer(c)
			a.documentExportButton(c)
			ui.Text(c, fmt.Sprintf("%d / 5000", len([]rune(a.noteSource())))).
				FontSize(10).TextColor(t.TextMuted).FontFeatures("tnum")
		})
		// Floating toolbar above the sheet, following Obsidian's editing
		// toolbar: mode switch, editing commands, then one save action.
		if c.Shortcut(ui.Cmd, ui.KeyE) {
			if a.noteMode == 2 {
				a.setNoteMode(a.noteEditMode)
			} else {
				a.setNoteMode(2)
			}
		}
		ui.Row(c).FillWidth().Justify(ui.Center).Children(func() {
			ui.Row(c).Wrap().MaxWidth(960).MaxWidthPercent(100).Padding(t.Space(1), t.Space(1.25)).Radius(t.Space(2.5)).
				Background(t.Surface).Border(1, t.Border).
				Shadow(0, 2, 16, 0, t.Text.Alpha(.10)).Gap(t.Space(.75)).
				AlignItems(ui.Center).Children(func() {
				live := a.noteMode == 0
				doc := a.noteDoc
				if live && doc != nil {
					formats := doc.Formatting()
					a.iconCmd(c, lucide.Undo2(), "撤销", false, doc.CanUndo(), doc.Undo)
					a.iconCmd(c, lucide.Redo2(), "重做", false, doc.CanRedo(), doc.Redo)
					ui.Box(c).Width(1).Height(18).Background(t.Border)
					a.iconCmd(c, lucide.Bold(), "粗体", formats.Bold == richtext.FormatOn, true, doc.ToggleBold)
					a.iconCmd(c, lucide.Italic(), "斜体", formats.Italic == richtext.FormatOn, true, doc.ToggleItalic)
					a.iconCmd(c, lucide.Strikethrough(), "删除线", formats.Strikethrough == richtext.FormatOn, true, doc.ToggleStrikethrough)
					a.iconCmd(c, lucide.Code(), "行内代码", formats.Code == richtext.FormatOn, true, doc.ToggleCode)
					ui.Box(c).Width(1).Height(18).Background(t.Border)
					a.iconCmd(c, lucide.Heading1(), "一级标题", formats.Paragraph.Heading == 1, true, func() { doc.SetHeading(1) })
					a.iconCmd(c, lucide.Heading2(), "二级标题", formats.Paragraph.Heading == 2, true, func() { doc.SetHeading(2) })
					a.iconCmd(c, lucide.List(), "项目列表", formats.Paragraph.List == richtext.Bullets, true, func() { doc.ToggleList(richtext.Bullets) })
					a.iconCmd(c, lucide.ListOrdered(), "编号列表", formats.Paragraph.List == richtext.Numbers, true, func() { doc.ToggleList(richtext.Numbers) })
					a.iconCmd(c, lucide.Quote(), "引用", formats.Paragraph.Quote, true, doc.ToggleQuote)
				} else {
					a.iconCmd(c, lucide.Bold(), "实时渲染中可格式化", false, false, func() {})
				}
				if a.noteMode == 3 {
					add := components.IconButton(c, lucide.Plus(), "插入孔板模块", components.IconButtonCompact)
					if add.Clicked() {
						a.insertPlateModule()
					}
				}
				// Citations only point to existing Run facts; no fact-writing controls live here (UX-P1, UX-P2, UX-P3).
				st := a.citePicker()
				cite := components.Button(c, "@引用", components.ButtonGhost)
				cite.Disabled(a.noteMode == 2)
				if cite.Clicked() && a.noteMode != 2 {
					st.manual = !st.open
					st.open = !st.open
					st.filter = "all"
					if !st.open {
						if q, active := notebook.ExtractAtQuery(a.noteSource()); active {
							st.dismissedToken = "@" + q
						}
					}
				}

				ui.Box(c).Width(t.Space(1))
				old := a.noteMode
				parts := components.TabsWithLeading(c, &a.noteMode, components.TabsSegmented,
					func(c *ui.Context, i int, size float32) {
						icon := []*ui.SVG{lucide.Sparkles(), lucide.PenLine(), lucide.BookOpen(), lucide.Blocks()}[i]
						components.Icon(c, icon).Size(size, size)
					}, "实时", "编辑", "阅读", "文档")
				if parts.List.Changed() {
					a.applyNoteModeChange(old)
				}
				// UX-STATE / UX-06: one explicit state word beside Save — never
				// a bare dot. It is the same vocabulary as the other surfaces.
				switch {
				case a.noteErr != "":
					components.Badge(c, "未保存", components.BadgeOptions{Tone: components.BadgeDanger})
				case a.noteDirty:
					components.Badge(c, "未保存", components.BadgeOptions{Tone: components.BadgeWarning})
				case a.noteState == "saved":
					components.Badge(c, "已保存", components.BadgeOptions{Tone: components.BadgeSuccess})
				default:
					components.Badge(c, "就绪", components.BadgeOptions{Tone: components.BadgeNeutral})
				}
				save := components.IconButton(c, lucide.Save(), "保存笔记", components.IconButtonCompact)
				save.Disabled(a.noteMode == 2)
				if save.Clicked() && a.noteMode != 2 {
					a.saveNotebook()
				}
			})
		})
		src := a.noteSource()
		atQuery, atActive := notebook.ExtractAtQuery(src)
		st := a.citePicker()
		if a.noteMode != 2 && atActive {
			tok := "@" + atQuery
			if st.dismissedToken != tok {
				st.open = true
				st.manual = false
			}
		} else if !st.manual {
			st.open = false
			st.dismissedToken = ""
		}
		var gridForCite *resultgrid.Grid
		if a.ws != nil {
			gridForCite = a.ws.Grid
		}
		frozenSrc := ""
		if a.asterDoc != nil {
			frozenSrc = a.asterDoc.Source
		}
		if st.open && a.noteMode != 2 {
			projName := a.snap.Title
			if a.shell != nil && strings.TrimSpace(a.shell.Project) != "" {
				projName = a.shell.Project
			}
			catalog := notebook.BuildCitationCatalog(projName, a.snap, a.runAssets, gridForCite, frozenSrc, atQuery)
			ui.Column(c).FillWidth().AlignItems(ui.Center).Children(func() {
				ui.Column(c).FillWidth().MaxWidth(820).MaxWidthPercent(100).
					Padding(t.Space(2), t.Space(2.5)).Radius(t.Space(2)).
					Background(t.Surface).Border(1, t.Border).
					Shadow(0, 4, 14, 0, t.Text.Alpha(.08)).Gap(t.Space(1.25)).Children(func() {
					ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(t.Space(1)).Children(func() {
						ui.Text(c, "项目 · "+catalog.Project).FontSize(12).Bold().TextColor(t.Text)
						if atActive {
							ui.Text(c, "@"+atQuery).FontSize(11).TextColor(t.Accent)
						}
						ui.Spacer(c)
						if components.Button(c, "关闭", components.ButtonGhost).Clicked() {
							st.open = false
							st.manual = false
							if atActive {
								st.dismissedToken = "@" + atQuery
							}
						}
					})
					ui.Row(c).FillWidth().Wrap().AlignItems(ui.Center).Gap(t.Space(.75)).Children(func() {
						for _, tab := range []struct{ key, label string }{
							{"all", "全部"},
							{"plate", "板图"},
							{"summary", "统计结果"},
							{"event", "总结语句"},
							{"step", "步骤板块"},
							{"asset", "文件"},
							{"readout", "读数"},
						} {
							tab := tab
							style := components.ButtonGhost
							if st.filter == tab.key {
								style = components.ButtonSecondary
							}
							if components.Button(c, tab.label, style).Clicked() {
								st.filter = tab.key
							}
						}
					})
					ui.Scroll(c).MaxHeight(220).MinHeight(90).Children(func() {
						ui.Column(c).FillWidth().Gap(t.Space(1)).Children(func() {
							shownGroups := 0
							for _, grp := range catalog.Groups {
								if st.filter != "all" && grp.Key != st.filter {
									continue
								}
								shownGroups++
								ui.Text(c, "▾ "+grp.Label).FontSize(11).Bold().TextColor(t.TextMuted)
								ui.Column(c).FillWidth().Padding(0, 0, 0, t.Space(2)).Gap(t.Space(.5)).Children(func() {
									for _, item := range grp.Items {
										item := item
										title := item.Label
										if item.Detail != "" && item.Detail != item.Label {
											title += " · " + item.Detail
										}
										if components.Button(c, title, components.ButtonSecondary).Clicked() {
											a.insertCitation(item.Citation)
											st.open = false
											st.manual = false
											st.dismissedToken = ""
										}
									}
								})
							}
							if shownGroups == 0 {
								ui.Text(c, "暂无可引用的记录").FontSize(12).TextColor(t.TextMuted)
							}
						})
					})
				})
			})
		}
		if a.noteErr != "" {
			ui.Row(c).FillWidth().Padding(6, 10).Radius(6).
				Background(theme.GetTokens(t).DangerBg).Children(func() {
				ui.Text(c, a.noteErr).FontSize(12).TextColor(theme.GetTokens(t).DangerText)
			})
		}
		// Snapshot details stay read-only; removing a token changes prose only (UX-STATE, UX-P5, UX-P1, UX-P2, UX-P3, UX-LAYOUT).
		citations := notebook.ResolveCitationsWith(src, a.snap, a.runAssets, gridForCite, frozenSrc)
		if len(citations) > 0 {
			ui.Column(c).FillWidth().AlignItems(ui.Center).Children(func() {
				ui.Column(c).FillWidth().MaxWidth(820).MaxWidthPercent(100).Padding(t.Space(2), t.Space(3)).
					Radius(t.Space(1.5)).Background(t.Surface).Border(1, t.Border).Gap(t.Space(1)).Children(func() {
					ui.Text(c, "引用快照").FontSize(11).Bold().TextColor(t.TextMuted)
					for i, resolved := range citations {
						i, resolved := i, resolved
						ui.Row(c).FillWidth().Wrap().AlignItems(ui.Center).Gap(t.Space(1.5)).Children(func() {
							ui.Text(c, resolved.Citation.ID).FontSize(11).TextColor(t.TextMuted)
							ui.Text(c, resolved.Citation.Snapshot).FontSize(12).TextColor(t.Text)
							if resolved.Status != "current" {
								warning := "已过期"
								if resolved.Status == "missing" {
									warning = "已过期 / 原记录不可用"
								}
								ui.Text(c, warning).FontSize(10).Bold().TextColor(t.Warning)
							}
							ui.Spacer(c)
							// A citation that names a stored entry can jump to that
							// entry in the timeline. It is a pointer, not a copy: the
							// target is resolved from the same snapshot (UX-IA-4).
							target, hasTarget := notebook.CitationTargetOf(
								notebook.Citation{Kind: resolved.Citation.Kind, ID: resolved.Citation.ID},
								a.snap, a.runAssets)
							if hasTarget {
								if components.Button(c, "定位到运行记录", components.ButtonGhost).Clicked() {
									a.jumpToTimeline(target.EntryID)
								}
							} else {
								ui.Text(c, "无可定位记录").FontSize(10).TextColor(t.TextMuted)
							}
							remove := components.Button(c, "移除引用", components.ButtonGhost)
							remove.Disabled(a.noteMode == 2)
							if remove.Clicked() && a.noteMode != 2 {
								a.removeCitation(i)
							}
						})
					}
				})
			})
		}
		// UI-REDESIGN §2.3 / UX-IA-4: two projections of one source. The prose
		// paper is the editable interpretation; the Run record is a read-only
		// timeline built from the same RunSnapshot. No third history store and
		// no fact-writing control exist in either projection.
		view := a.noteView()
		projection := notebook.Build(a.snap)
		width, _ := c.Size()
		if width >= 1100 {
			ui.Row(c).Grow(1).FillWidth().Gap(t.Space(3)).Children(func() {
				ui.Column(c).Grow(1).MinWidth(0).Children(func() { a.notebookProse(c, t) })
				ui.Column(c).Grow(1).MinWidth(0).Children(func() {
					notebookTimeline(c, a, t, projection)
				})
			})
			return
		}
		// Narrow (covers 900×600): the two sections switch instead of sitting
		// side by side, so prose keeps a readable measure and Save (above, in
		// the toolbar) stays in the first viewport (UX-P1, UX-LAYOUT).
		ui.Row(c).FillWidth().Children(func() {
			components.Tabs(c, &view.split, components.TabsSegmented, "笔记", "运行记录")
		})
		if view.split == 1 {
			ui.Row(c).Grow(1).FillWidth().Children(func() {
				ui.Column(c).Grow(1).MinWidth(0).Children(func() { notebookTimeline(c, a, t, projection) })
			})
			return
		}
		a.notebookProse(c, t)
	})
}

// notebookProse renders the editable paper of one mode: live rich text,
// source editing, reading, or the AsterML document. It writes prose only.
func (a *app) notebookProse(c *ui.Context, t *ui.Theme) {
	paper := func(children func()) {
		ui.Column(c).Grow(1).FillWidth().AlignItems(ui.Center).Children(func() {
			ui.Box(c).Grow(1).FillWidth().MaxWidth(820).MaxWidthPercent(100).
				Background(t.Surface).Border(1, t.Border).
				Radius(t.Space(2.5)).Shadow(0, 1, 3, 0, t.Text.Alpha(.04)).
				Padding(t.Space(5), t.Space(7)).Children(children)
		})
	}
	switch a.noteMode {
	case 3: // AsterML document
		paper(func() {
			if a.noteAster == nil || a.noteAster.Doc == nil || a.noteAster.Doc.Source != a.noteDraft {
				doc, err := asterdoc.Parse(a.noteDraft)
				if err != nil {
					ui.Text(c, "文档语法错误："+err.Error()).FontSize(t.Rem(.85)).TextColor(t.Danger)
					return
				}
				a.noteAster = &asterdoc.View{Doc: doc, Mode: asterdoc.ModeNotebook,
					Allowed: map[string]bool{asterdoc.PermDocumentEdit: true},
					OnChange: func(source string) {
						a.noteDraft = source
						a.noteDirty = true
						a.noteState = ""
					}}
			}
			a.noteAster.Render(c)
		})
	case 2: // reading
		ui.Scroll(c).Grow(1).Children(func() {
			ui.Column(c).FillWidth().Padding(0, t.Space(2), t.Space(4), t.Space(2)).
				AlignItems(ui.Center).Children(func() {
				ui.Box(c).FillWidth().MaxWidth(820).MaxWidthPercent(100).
					Background(t.Surface).Border(1, t.Border).Radius(t.Space(2.5)).
					Padding(t.Space(7), t.Space(8)).Children(func() {
					if strings.TrimSpace(a.noteDraft) == "" {
						ui.Text(c, "还没有内容。切换到“编辑”开始写。").
							FontSize(13).TextColor(t.TextMuted)
						return
					}
					protocol.RenderMarkdown(c, a.noteDraft)
				})
			})
		})
	case 1: // source
		paper(func() {
			area := ui.TextAreaBase(c, &a.noteDraft)
			area.Fill().Grow(1).MinWidth(0).MinHeight(0)
			if area.Changed() {
				a.noteDirty = true
				a.noteState = ""
			}
		})
	default: // live rich rendering
		paper(func() {
			if a.noteDoc == nil {
				a.noteDoc = richtext.NewDocument(richtext.ParseMarkdown(a.noteDraft))
			}
			editor := richtext.EditorBase(c, a.noteDoc, richtext.EditorOptions{
				Label: "Notebook", Placeholder: "开始写实验笔记…",
			})
			editor.Fill().Grow(1).MinWidth(0).MinHeight(0)
			if editor.Changed() {
				a.noteDirty = true
				a.noteState = ""
			}
		})
	}
}

// notebookTimeline renders the Run record: a read-only projection of the same
// RunSnapshot, one row per stored entry, in recorded order. It never edits a
// fact and holds no copy of one; a row can only jump to the prose citation
// that named it (UX-IA-4, UX-STATE).
func notebookTimeline(c *ui.Context, a *app, t *ui.Theme, projection notebook.Notebook) {
	tokens := theme.GetTokens(t)
	view := a.noteView()
	ui.Scroll(c).Grow(1).FillWidth().Children(func() {
		ui.Column(c).FillWidth().Padding(t.Space(2), 0, t.Space(4), 0).Gap(t.Space(1.5)).Children(func() {
			ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(t.Space(2)).Children(func() {
				ui.Text(c, "运行记录").FontSize(t.Rem(1.0)).FontWeight(600).TextColor(t.Text)
				components.Badge(c, "只读", components.BadgeOptions{Tone: components.BadgeNeutral})
				ui.Text(c, fmt.Sprintf("Design rev %d · %d 条", projection.DesignRevision, len(projection.Entries))).
					FontSize(t.Rem(.78)).TextColor(t.TextMuted).FontFeatures("tnum")
			})
			if len(projection.Entries) == 0 {
				ui.Text(c, "此运行还没有可追溯的记录。").FontSize(t.Rem(.85)).TextColor(t.TextMuted)
				return
			}
			for _, entry := range projection.Entries {
				entry := entry
				highlighted := entry.ID == view.highlightID
				row := ui.Column(c).FillWidth().Padding(t.Space(1.5), t.Space(2)).Radius(t.Space(1.5))
				if highlighted {
					row.Background(tokens.WarningBg).Border(1, t.Warning)
				}
				row.Children(func() {
					ui.Row(c).FillWidth().AlignItems(ui.Start).Gap(t.Space(1.5)).Children(func() {
						cat := iconForCategory(entry.Category)
						components.Icon(c, cat).Size(t.Space(3), t.Space(3)).TextColor(t.TextMuted)
						ui.Column(c).Grow(1).MinWidth(0).Gap(t.Space(.5)).Children(func() {
							ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(t.Space(1)).Children(func() {
								ui.Text(c, entry.Title).FontSize(t.Rem(.85)).FontWeight(600).
									SingleLine().Ellipsis("…")
								ui.Spacer(c)
								ui.Text(c, entry.DisplayTime()).FontSize(t.Rem(.75)).
									TextColor(t.TextMuted).FontFeatures("tnum")
							})
							ui.Text(c, entry.SourceType+" "+entry.SourceID).FontSize(t.Rem(.7)).
								TextColor(t.TextMuted).SingleLine().Ellipsis("…")
							switch entry.Trace {
							case notebook.TraceInvalid:
								ui.Text(c, "来源失效").FontSize(t.Rem(.7)).TextColor(t.Warning)
							case notebook.TraceMissing:
								ui.Text(c, "来源未记录").FontSize(t.Rem(.7)).TextColor(t.Warning)
							}
						})
					})
				})
				rowElement := row
				if highlighted && view.scrolledID != entry.ID {
					rowElement.ScrollIntoView(ui.ScrollIntoViewOptions{Block: ui.ScrollCenter})
					view.scrolledID = entry.ID
				}
			}
		})
	})
}

func iconForCategory(category notebook.Category) *ui.SVG {
	switch category {
	case notebook.Execution:
		return lucide.Play()
	case notebook.Checklist:
		return lucide.ListChecks()
	case notebook.Measurement:
		return lucide.Table2()
	case notebook.Observation:
		return lucide.Eye()
	case notebook.Deviation:
		return lucide.TriangleAlert()
	}
	return lucide.Settings()
}

// applyNoteModeChange finishes a switch the Tabs control already made.
func (a *app) applyNoteModeChange(old int) {
	if old == a.noteMode {
		return
	}
	if old == 0 {
		a.noteDraft = a.noteSource()
	}
	if a.noteMode == 0 {
		a.noteDoc = richtext.NewDocument(richtext.ParseMarkdown(a.noteDraft))
	}
	if a.noteMode != 2 {
		a.noteEditMode = a.noteMode
	}
}

func (a *app) insertPlateModule() {
	a.noteAster = nil
	stamp := time.Now().UnixNano()
	groupID := fmt.Sprintf("group-%d", stamp)
	block := fmt.Sprintf("\n```asterlab\nkind: group\nid: %s\nschema: 1\nname: Control\nrole: control\n```\n\n```asterlab\nkind: plate\nid: plate-%d\nschema: 1\nformat: 96\ngroups:\n  %s: [A1]\n```\n", groupID, stamp, groupID)
	if strings.TrimSpace(a.noteDraft) != "" {
		a.noteDraft = strings.TrimRight(a.noteDraft, "\n") + "\n" + block
	} else {
		a.noteDraft = strings.TrimLeft(block, "\n")
	}
	a.noteDirty = true
	a.noteState = ""
}

func (a *app) setNoteMode(mode int) {
	old := a.noteMode
	if old == mode {
		return
	}
	if old == 0 {
		a.noteDraft = a.noteSource()
	}
	if mode == 0 {
		a.noteDoc = richtext.NewDocument(richtext.ParseMarkdown(a.noteDraft))
	}
	if mode != 2 {
		a.noteEditMode = mode
	}
	a.noteMode = mode
}

// iconCmd renders one compact toolbar command. Formatting commands act on the
// live rich document and mark the note dirty.
func (a *app) iconCmd(c *ui.Context, icon *ui.SVG, label string, active, enabled bool, act func()) {
	t := c.Theme()
	b := components.IconButton(c, icon, label, components.IconButtonCompact)
	if active {
		b.Background(t.Accent.Alpha(.14))
		b.TextColor(t.Accent)
	}
	b.Disabled(!enabled)
	if b.Clicked() && enabled {
		act()
		a.noteDirty = true
		a.noteState = ""
	}
}

// noteSource returns the canonical Markdown for the current mode.
func (a *app) noteSource() string {
	if a.noteMode == 0 && a.noteDoc != nil {
		return a.noteDoc.Text().Markdown()
	}
	return a.noteDraft
}

func (a *app) insertCitation(citation notebook.Citation) {
	a.noteDraft = notebook.ApplyAtCitation(a.noteSource(), citation)
	if a.noteMode == 0 {
		a.noteDoc = richtext.NewDocument(richtext.ParseMarkdown(a.noteDraft))
	}
	a.noteDirty = true
	a.noteState = ""
}

func (a *app) removeCitation(index int) {
	a.noteDraft = notebook.RemoveCitation(a.noteSource(), index)
	if a.noteMode == 0 {
		a.noteDoc = richtext.NewDocument(richtext.ParseMarkdown(a.noteDraft))
	}
	a.noteDirty = true
	a.noteState = ""
}

// jumpToTimeline points the surface at the entry a citation named. At narrow
// widths it brings the timeline projection forward, because the two sections
// switch there; it never changes the prose.
func (a *app) jumpToTimeline(entryID string) {
	view := a.noteView()
	view.highlightID = entryID
	view.scrolledID = ""
	view.split = 1
}

func (a *app) loadNotebook() {
	a.noteRunID = a.snap.RunID
	a.noteDraft = ""
	a.noteDirty = false
	a.noteState = ""
	a.noteErr = ""
	var latest *runs.RunEvent
	for i := range a.snap.Events {
		event := &a.snap.Events[i]
		if event.Stage != "notebook" {
			continue
		}
		if latest == nil || event.RecordedAt >= latest.RecordedAt {
			latest = event
		}
	}
	if latest != nil {
		a.noteDraft = latest.Content
	}
	a.noteMode = 0
	if os.Getenv("ASTERLAB_NOTE_MODE") == "doc" {
		a.noteMode = 3
	}
	a.noteEditMode = 0
	a.noteDoc = richtext.NewDocument(richtext.ParseMarkdown(a.noteDraft))
}

func (a *app) saveNotebook() {
	source := a.noteSource()
	a.noteDraft = source
	if a.runSvc == nil {
		a.noteErr = "运行服务不可用"
		a.noteState = "error"
		return
	}
	if strings.TrimSpace(source) == "" {
		a.noteErr = "Notebook 不能为空"
		a.noteState = "error"
		return
	}
	if len([]rune(source)) > 5000 {
		a.noteErr = "Notebook 超过 5000 个字符；请拆分。"
		a.noteState = "error"
		return
	}
	snap, err := a.runSvc.SaveNotebook(context.Background(), runs.NotebookInput{
		RunID: a.snap.RunID, Markdown: source, OperationID: a.nextOp(),
	})
	if err != nil {
		a.noteErr = state.MapError(err).Error()
		a.noteState = "error"
		return
	}
	a.noteErr = ""
	a.noteState = "saved"
	a.noteDirty = false
	a.applySnap(snap)
}
