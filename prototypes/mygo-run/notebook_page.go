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
	"asterlab-mygo-prototype/internal/protocol"
	"asterlab-mygo-prototype/internal/shell"
	"asterlab-mygo-prototype/internal/state"
	"asterlab-mygo-prototype/internal/theme"
	"asterlab-mygo-prototype/modules/runs"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/richtext"
	"github.com/oneo-me/onevra/source/components"
	"github.com/oneo-me/onevra/source/icons/lucide"
)

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
		ui.Row(c).FillWidth().AlignItems(ui.Center).Padding(0, t.Space(1)).Children(func() {
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
			ui.Row(c).Padding(t.Space(1), t.Space(1.25)).Radius(t.Space(2.5)).
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
				ui.Spacer(c)
				old := a.noteMode
				parts := components.TabsWithLeading(c, &a.noteMode, components.TabsSegmented,
					func(c *ui.Context, i int, size float32) {
						icon := []*ui.SVG{lucide.Sparkles(), lucide.PenLine(), lucide.BookOpen(), lucide.Blocks(), lucide.ListClock()}[i]
						components.Icon(c, icon).Size(size, size)
					}, "实时", "编辑", "阅读", "文档", "时间线")
				if parts.List.Changed() {
					a.applyNoteModeChange(old)
				}
				if a.noteDirty {
					ui.Box(c).Size(6, 6).Radius(3).Background(t.Warning)
				}
				save := components.IconButton(c, lucide.Save(), "保存笔记", components.IconButtonCompact)
				save.Disabled(a.noteMode == 2 || a.noteMode == 4)
				if save.Clicked() && a.noteMode != 2 && a.noteMode != 4 {
					a.saveNotebook()
				}
			})
		})
		if a.noteErr != "" {
			ui.Row(c).FillWidth().Padding(6, 10).Radius(6).
				Background(theme.GetTokens(t).DangerBg).Children(func() {
				ui.Text(c, a.noteErr).FontSize(12).TextColor(theme.GetTokens(t).DangerText)
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
		case 4: // frozen Run timeline, read-only
			a.renderNotebookTimeline(c)
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

// renderNotebookTimeline is a read-only view over the authoritative Run
// events. It never copies events into the editable Markdown draft.
func (a *app) renderNotebookTimeline(c *ui.Context) {
	t := c.Theme()
	ui.Scroll(c).Grow(1).Children(func() {
		ui.Column(c).FillWidth().Padding(t.Space(3), t.Space(5)).Gap(t.Space(2)).
			AlignItems(ui.Center).Children(func() {
			if a.snap.AsterDoc != nil {
				ui.Box(c).FillWidth().MaxWidth(980).MaxWidthPercent(100).
					Background(t.Surface).Border(1, t.Border).Radius(t.Space(2)).
					Padding(t.Space(4), t.Space(5)).Children(func() {
					ui.Text(c, fmt.Sprintf("冻结 AsterDoc · Revision %d", a.snap.AsterDoc.Revision)).
						FontSize(t.Rem(1)).Bold().TextColor(t.Text)
					a.renderFrozenAsterDoc(c, asterdoc.ModeNotebook)
				})
			}
			ui.Box(c).FillWidth().MaxWidth(980).MaxWidthPercent(100).
				Background(t.Surface).Border(1, t.Border).Radius(t.Space(2)).
				Padding(t.Space(4), t.Space(5)).Children(func() {
				ui.Text(c, "运行时间线 · 只读").FontSize(t.Rem(1.15)).Bold().TextColor(t.Text)
				ui.Text(c, "读取 Core Event Store；这里不重新保存事件事实。").
					FontSize(t.Rem(.75)).TextColor(t.TextMuted)
				if len(a.snap.Events) == 0 {
					ui.Text(c, "本次运行还没有事件。").FontSize(t.Rem(.85)).TextColor(t.TextMuted)
					return
				}
				for _, event := range a.snap.Events {
					event := event
					ui.Column(c).FillWidth().Gap(t.Space(.5)).Padding(t.Space(2)).
						Border(1, t.Border).Radius(t.Space(1)).Children(func() {
						ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(t.Space(1)).Children(func() {
							ui.Text(c, event.EventType).FontSize(t.Rem(.78)).Bold().TextColor(t.Text)
							ui.Text(c, event.Content).FontSize(t.Rem(.78)).TextColor(t.TextMuted)
							ui.Spacer(c)
							if event.BatchID != "" {
								if components.Button(c, "打开原始孔板", components.ButtonGhost).Clicked() {
									a.batchView = event.BatchID
									if a.shell != nil {
										a.shell.Page = shell.Run
									}
								}
							}
						})
						label := func(value string) string {
							if strings.TrimSpace(value) == "" {
								return "未记录"
							}
							return value
						}
						ui.Text(c, fmt.Sprintf("event %s · batch %s · task %s · %s",
							event.ID, label(event.BatchID), label(event.TaskKey), label(event.StepStatus))).
							FontSize(t.Rem(.68)).TextColor(t.TextMuted)
						if len(event.Checks) > 0 || len(event.ResultCells) > 0 {
							ui.Text(c, fmt.Sprintf("checks %d · cells %d", len(event.Checks), len(event.ResultCells))).
								FontSize(t.Rem(.68)).TextColor(t.TextMuted)
						}
					})
				}
			})
		})
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
