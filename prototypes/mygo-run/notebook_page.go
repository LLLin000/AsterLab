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

// notebookPage follows Obsidian's shape: one paper sheet, a floating toolbar,
// and three modes over the same stored Markdown — live rich rendering, source
// editing, and reading. Each save appends one notebook-stage event.
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
				if a.noteDirty {
					ui.Box(c).Size(6, 6).Radius(3).Background(t.Warning)
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
	})
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
	if a.noteMode == 0 { a.noteDoc = richtext.NewDocument(richtext.ParseMarkdown(a.noteDraft)) }
	a.noteDirty = true
	a.noteState = ""
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
