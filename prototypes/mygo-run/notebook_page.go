package main

// Notebook surface: prose over the stored Markdown of one Run. Owned by the
// Notebook/Protocol Owner after Wave 1 extraction.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"asterlab-mygo-prototype/internal/asterdoc"
	"asterlab-mygo-prototype/internal/notebook"
	"asterlab-mygo-prototype/internal/notebook/record"
	"asterlab-mygo-prototype/internal/protocol"
	"asterlab-mygo-prototype/internal/resultgrid"
	"asterlab-mygo-prototype/internal/state"
	"asterlab-mygo-prototype/internal/theme"
	"asterlab-mygo-prototype/modules/runs"
	mygo "github.com/egoist/mygo"
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
	expanded    map[string]bool
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

// recordBoardView holds the record board's UI-only state: which FOV layer the
// Well x Timepoint matrix shows. It names no fact; the projection owns the data.
type recordBoardView struct {
	fov int
}

// recordBoards keys board state by Run so switching Runs never leaks one
// Run's FOV selection into another.
var recordBoards = map[string]*recordBoardView{}

func (a *app) recordBoard() *recordBoardView {
	st := recordBoards[a.snap.RunID]
	if st == nil {
		st = &recordBoardView{fov: 1}
		recordBoards[a.snap.RunID] = st
	}
	if st.fov < 1 || st.fov > record.FOVSlots {
		st.fov = 1
	}
	return st
}

// recordProjection is the single shared read model for this surface: the
// automatic board, the protocol archive, and the Run history audit rendering
// all project from it, so neither surface re-derives summaries from the
// snapshot on its own.
func (a *app) recordProjection() record.RecordProjection {
	return record.Project(a.snap, a.runAssets)
}

// recordSeal verifies the current prose against its signature trailer.
// ok means the prose is signed, unmodified, and locked for editing.
func (a *app) recordSeal() (record.Signature, bool) {
	return record.Verify(a.noteSource())
}

// notebookPage follows Obsidian's shape: one paper sheet, a floating toolbar,
// and five modes over the same stored Markdown — live rich rendering, source
// editing, reading, the AsterML document, and the record board. Each save
// appends one notebook-stage event.
func notebookPage(c *ui.Context, a *app) {
	t := c.Theme()
	tokens := theme.GetTokens(t)
	if a.snap.RunID == "" {
		ui.Column(c).Fill().Center().Gap(t.Space(2)).Children(func() {
			ui.Text(c, "Notebook").FontSize(float32(tokens.TextPage)).Bold().TextColor(t.Text)
			ui.Text(c, "先选择或加载一个 Run，笔记会记录在它的执行历史中。").
				FontSize(float32(tokens.TextMeta)).TextColor(t.TextMuted)
		})
		return
	}
	if a.noteRunID != a.snap.RunID {
		a.loadNotebook()
	}
	ui.Column(c).Fill().Padding(t.Space(2), t.Space(3)).Gap(t.Space(2)).Children(func() {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(t.Space(1.5)).Padding(0, t.Space(1)).Children(func() {
			ui.Text(c, "Notebook").FontSize(float32(tokens.TextMeta)).Bold().TextColor(t.TextMuted)
			ui.Text(c, "/").FontSize(float32(tokens.TextMeta)).TextColor(t.Border)
			ui.Text(c, a.snap.Title).FontSize(float32(tokens.TextBody)).TextColor(t.TextMuted).
				SingleLine().Ellipsis("…").MaxWidth(420)
			if a.snap.AsterDoc == nil && a.snap.DesignRevision > 0 {
				ui.Text(c, fmt.Sprintf("设计版本 %d", a.snap.DesignRevision)).
					FontSize(float32(tokens.TextData)).TextColor(t.TextMuted)
			}
			ui.Spacer(c)
			a.documentExportButton(c)
			ui.Text(c, fmt.Sprintf("%d / 5000", len([]rune(a.noteSource())))).
				FontSize(float32(tokens.TextData)).TextColor(t.TextMuted).FontFeatures("tnum")
		})
		// Floating toolbar above the sheet, following Obsidian's editing
		// toolbar: mode switch, editing commands, then one save action. A
		// sealed (signed) record locks editing until explicitly unlocked.
		sealed := false
		if !a.noteDirty {
			_, sealed = a.recordSeal()
		}
		if c.Shortcut(ui.Cmd, ui.KeyE) && !sealed {
			if a.noteMode == 2 {
				a.setNoteMode(a.noteEditMode)
			} else {
				a.setNoteMode(2)
			}
		}
		if c.Shortcut(ui.Cmd, ui.KeyS) && a.noteMode != 2 && !sealed && a.noteState != "saving" {
			a.saveNotebook()
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
				cite.Disabled(a.noteMode == 2 || sealed)
				if cite.Clicked() && a.noteMode != 2 && !sealed {
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
						icon := []*ui.SVG{lucide.Sparkles(), lucide.PenLine(), lucide.BookOpen(), lucide.Blocks(), lucide.LayoutGrid()}[i]
						components.Icon(c, icon).Size(size, size)
					}, "实时", "编辑", "阅读", "文档", "图板")
				if parts.List.Changed() {
					// The sealed record locks the editing modes, but the
					// read-only board stays reachable: it hosts 解锁.
					if sealed && a.noteMode != 2 && a.noteMode != 4 {
						a.noteMode = old
						c.AddToast(ui.Toast{Title: "已签核归档", Description: "记录已锁定，解锁后可编辑。", Type: components.ToastWarning})
					} else {
						a.applyNoteModeChange(old)
					}
				}
				// UX-STATE / UX-06: one explicit state word beside Save, from the
				// projection's single vocabulary — never a bare colour dot.
				stateLabel, stateColor := a.notebookToolbarState(tokens)
				ui.Text(c, stateLabel).FontSize(float32(tokens.TextData)).Bold().TextColor(stateColor)
				save := components.IconButton(c, lucide.Save(), "保存笔记", components.IconButtonCompact)
				save.Disabled(a.noteMode == 2 || sealed || a.noteState == "saving")
				if save.Clicked() && a.noteMode != 2 && !sealed && a.noteState != "saving" {
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
						ui.Text(c, "项目 · "+catalog.Project).FontSize(float32(tokens.TextMeta)).Bold().TextColor(t.Text)
						if atActive {
							ui.Text(c, "@"+atQuery).FontSize(float32(tokens.TextData)).TextColor(t.Accent)
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
								ui.Text(c, "▾ "+grp.Label).FontSize(float32(tokens.TextMeta)).Bold().TextColor(t.TextMuted)
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
								ui.Text(c, "暂无可引用的记录").FontSize(float32(tokens.TextMeta)).TextColor(t.TextMuted)
							}
						})
					})
				})
			})
		}
		if a.noteState == "error" || a.noteErr != "" {
			reason := strings.TrimSpace(a.noteErr)
			if reason == "" {
				reason = "保存失败"
			}
			ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(t.Space(2)).
				Padding(t.Space(1.5), t.Space(2.5)).Radius(float32(tokens.ControlRadius)).
				Background(tokens.DangerBg).Border(1, t.Danger).Children(func() {
				ui.Text(c, fmt.Sprintf("未保存（%s）", reason)).
					FontSize(float32(tokens.TextMeta)).TextColor(tokens.DangerText).Grow(1).MinWidth(0)
				retry := components.Button(c, "重试", components.ButtonSecondary)
				retry.Disabled(a.noteState == "saving")
				if retry.Clicked() && a.noteState != "saving" {
					a.saveNotebook()
				}
			})
		} else if a.noteRecovered && a.noteDirty {
			ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(t.Space(2)).
				Padding(t.Space(1), t.Space(2)).Radius(float32(tokens.ControlRadius)).
				Background(tokens.WarningBg).Border(1, t.Border).Children(func() {
				ui.Text(c, "已从本地草稿恢复未保存修改。").
					FontSize(float32(tokens.TextMeta)).TextColor(tokens.WarningText).Grow(1).MinWidth(0)
				if components.Button(c, "丢弃草稿", components.ButtonGhost).Clicked() {
					a.discardNotebookDraft()
				}
			})
		}
		// Snapshot details stay read-only; removing a token changes prose only (UX-STATE, UX-P5, UX-P1, UX-P2, UX-P3, UX-LAYOUT).
		citations := notebook.ResolveCitationsWith(src, a.snap, a.runAssets, gridForCite, frozenSrc)
		if len(citations) > 0 {
			ui.Column(c).FillWidth().AlignItems(ui.Center).Children(func() {
				ui.Column(c).FillWidth().MaxWidth(820).MaxWidthPercent(100).Padding(t.Space(2), t.Space(3)).
					Radius(t.Space(1.5)).Background(t.Surface).Border(1, t.Border).Gap(t.Space(1)).Children(func() {
					ui.Text(c, "引用快照").FontSize(float32(tokens.TextMeta)).Bold().TextColor(t.TextMuted)
					for i, resolved := range citations {
						i, resolved := i, resolved
						ui.Row(c).FillWidth().Wrap().AlignItems(ui.Center).Gap(t.Space(1.5)).Children(func() {
							citeLabel := strings.TrimSpace(resolved.Citation.Title)
							if citeLabel == "" {
								citeLabel = citationKindLabel(resolved.Citation.Kind)
							}
							ui.Text(c, citeLabel).FontSize(float32(tokens.TextData)).TextColor(t.TextMuted).
								Tooltip(resolved.Citation.ID)
							ui.Text(c, resolved.Citation.Snapshot).FontSize(float32(tokens.TextMeta)).TextColor(t.Text)
							if resolved.Status != "current" {
								warning := "已过期"
								if resolved.Status == "missing" {
									warning = "已过期 / 原记录不可用"
								}
								ui.Text(c, warning).FontSize(float32(tokens.TextData)).Bold().TextColor(t.Warning)
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
								ui.Text(c, "无可定位记录").FontSize(float32(tokens.TextMeta)).TextColor(t.TextMuted)
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
		// UI-REDESIGN §2.3 / UX-IA-4: the prose paper is the editable
		// interpretation; the Run record is a read-only timeline built from
		// the same RunSnapshot; the board renders the same snapshot through
		// the shared RecordProjection (planned-vs-actual plus the Well x
		// Timepoint x FOV matrix). No third history store and no
		// fact-writing control exist in any projection.
		if a.noteMode == 4 {
			a.recordBoardPage(c, t)
			return
		}
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
	// A sealed record locks every editing mode: the prose verifies against
	// its signature, so the paper shows the lock instead of an editor until
	// explicitly unlocked. Reading stays available.
	if _, ok := a.recordSeal(); ok && !a.noteDirty && a.noteMode != 2 && a.noteMode != 4 {
		a.recordLockedNotice(c, t)
		return
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
						a.markNotebookDirty()
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
						tokens := theme.GetTokens(t)
						stateWord := "未开始"
						if a.noteDirty {
							stateWord = "草稿"
						}
						ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(t.Space(1.5)).Children(func() {
							components.Badge(c, stateWord, components.BadgeOptions{Tone: components.BadgeNeutral})
							ui.Text(c, "尚未记录笔记内容，切换到“实时”或“编辑”开始撰写。").
								FontSize(float32(tokens.TextBody)).TextColor(t.TextMuted)
						})
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
				a.markNotebookDirty()
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
				a.markNotebookDirty()
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
	if view.expanded == nil {
		view.expanded = map[string]bool{}
	}
	taskTitles := make(map[string]string, len(a.snap.Tasks))
	for _, task := range a.snap.Tasks {
		title := strings.TrimSpace(task.Title)
		if title == "" {
			title = "执行步骤"
		}
		if tp := strings.TrimSpace(task.Timepoint); tp != "" {
			title += " · " + tp
		}
		taskTitles[task.Key] = title
	}
	ui.Scroll(c).Grow(1).FillWidth().Children(func() {
		ui.Column(c).FillWidth().Padding(t.Space(2), 0, t.Space(4), 0).Gap(t.Space(1.5)).Children(func() {
			ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(t.Space(2)).Children(func() {
				ui.Text(c, "运行记录").FontSize(t.Rem(1.0)).FontWeight(600).TextColor(t.Text)
				components.Badge(c, "只读", components.BadgeOptions{Tone: components.BadgeNeutral})
				ui.Text(c, fmt.Sprintf("设计版本 %d · %d 条", projection.DesignRevision, len(projection.Entries))).
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
				rawRef := notebookEntryRawLabel(entry)
				summary := notebookEntrySummary(entry, a.snap, taskTitles)
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
							summaryText := ui.Text(c, summary).FontSize(float32(tokens.TextData)).
								TextColor(t.TextMuted).SingleLine().Ellipsis("…")
							if rawRef != "" {
								summaryText.Tooltip(rawRef)
							}
							switch entry.Trace {
							case notebook.TraceInvalid:
								ui.Text(c, "来源失效").FontSize(t.Rem(.7)).TextColor(t.Warning)
							case notebook.TraceMissing:
								ui.Text(c, "来源未记录").FontSize(t.Rem(.7)).TextColor(t.Warning)
							}
							open := view.expanded[entry.ID]
							components.Collapsible(c, "来源详情", &open, components.CollapsibleOptions{}, func() {
								if rawRef != "" {
									ui.Text(c, "原始来源："+rawRef).FontSize(float32(tokens.TextData)).TextColor(t.TextMuted)
								}
								if entry.TaskID != "" {
									ui.Text(c, "任务标识："+entry.TaskID).FontSize(float32(tokens.TextData)).TextColor(t.TextMuted)
								}
								if entry.DesignStepID != "" {
									ui.Text(c, "设计步骤："+entry.DesignStepID).FontSize(float32(tokens.TextData)).TextColor(t.TextMuted)
								}
								for _, detail := range entry.Details {
									ui.Text(c, detail.Label+"："+detail.Value).FontSize(float32(tokens.TextData)).TextColor(t.TextMuted)
								}
								for _, ref := range entry.References {
									ui.Text(c, "关联引用："+ref.Label+" "+ref.ID).FontSize(float32(tokens.TextData)).TextColor(t.TextMuted)
								}
							})
							view.expanded[entry.ID] = open
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
	if a.noteMode == 3 {
		a.noteAster = nil
	}
	if a.noteMode != 2 && a.noteMode != 4 {
		a.noteEditMode = a.noteMode
	}
	if a.noteDirty {
		a.persistNotebookDraft()
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
	a.markNotebookDirty()
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
	if mode == 3 {
		a.noteAster = nil
	}
	if mode != 2 && mode != 4 {
		a.noteEditMode = mode
	}
	a.noteMode = mode
	if a.noteDirty {
		a.persistNotebookDraft()
	}
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
		a.markNotebookDirty()
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
	a.markNotebookDirty()
}

func (a *app) removeCitation(index int) {
	a.noteDraft = notebook.RemoveCitation(a.noteSource(), index)
	if a.noteMode == 0 {
		a.noteDoc = richtext.NewDocument(richtext.ParseMarkdown(a.noteDraft))
	}
	a.markNotebookDirty()
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
	if a.noteRunID != "" && a.noteRunID != a.snap.RunID && (a.noteDirty || a.noteErr != "") {
		a.persistNotebookDraft()
	}
	a.noteRunID = a.snap.RunID
	a.noteDraft = ""
	a.noteDirty = false
	a.noteRecovered = false
	a.noteState = ""
	a.noteErr = ""
	a.noteAster = nil

	latestContent, hasSaved := latestNotebookRevision(a.snap.Events)
	var recovered *notebookDraft
	if a.snap.RunID != "" {
		if d, ok := a.noteDrafts[a.snap.RunID]; ok {
			recovered = &d
		} else if d, ok := loadNotebookDraftFile(a.snap.RunID); ok {
			recovered = &d
		}
	}

	mode := 0
	if os.Getenv("ASTERLAB_NOTE_MODE") == "doc" {
		mode = 3
	}
	if recovered != nil && (recovered.Markdown != latestContent || (!hasSaved && recovered.Markdown != "")) {
		a.noteDraft = recovered.Markdown
		a.noteDirty = true
		a.noteRecovered = true
		if recovered.Mode >= 0 && recovered.Mode <= 3 && recovered.Mode != 2 {
			mode = recovered.Mode
		}
	} else {
		if recovered != nil {
			a.clearNotebookDraft(a.snap.RunID)
		}
		a.noteDraft = latestContent
		if hasSaved {
			a.noteState = "saved"
		}
	}
	// A sealed record opens in reading mode: the prose verifies against its
	// signature, so no editing mode is shown until explicitly unlocked.
	if _, ok := record.Verify(a.noteDraft); ok {
		mode = 2
	}
	a.noteMode = mode
	if mode != 2 {
		a.noteEditMode = mode
	} else {
		a.noteEditMode = 0
	}
	a.noteDoc = richtext.NewDocument(richtext.ParseMarkdown(a.noteDraft))
}

func (a *app) saveNotebook() {
	source := a.noteSource()
	a.noteDraft = source
	if a.runSvc == nil {
		a.noteErr = "运行服务不可用"
		a.noteState = "error"
		if strings.TrimSpace(source) != "" || a.noteDirty {
			a.noteDirty = true
			a.persistNotebookDraft()
		}
		return
	}
	if strings.TrimSpace(source) == "" {
		a.noteErr = "Notebook 不能为空"
		a.noteState = "error"
		if a.noteDirty {
			a.persistNotebookDraft()
		}
		return
	}
	if len([]rune(source)) > 5000 {
		a.noteErr = "Notebook 超过 5000 个字符；请拆分。"
		a.noteState = "error"
		a.noteDirty = true
		a.persistNotebookDraft()
		return
	}
	a.noteState = "saving"
	a.noteErr = ""
	snap, err := a.runSvc.SaveNotebook(context.Background(), runs.NotebookInput{
		RunID: a.snap.RunID, Markdown: source, OperationID: a.nextOp(),
	})
	if err != nil {
		a.noteErr = state.MapError(err).Error()
		a.noteState = "error"
		a.noteDirty = true
		a.persistNotebookDraft()
		return
	}
	a.noteErr = ""
	a.noteState = "saved"
	a.noteDirty = false
	a.noteRecovered = false
	a.clearNotebookDraft(a.snap.RunID)
	a.applySnap(snap)
}

func (a *app) notebookToolbarState(tokens theme.Tokens) (string, ui.Color) {
	// A verified signature outranks everything: the prose is signed,
	// unmodified, and locked.
	if _, ok := record.Verify(a.noteSource()); ok && !a.noteDirty {
		return string(record.StateSealed), tokens.SuccessText
	}
	switch {
	case a.noteState == "saving":
		return "保存中", tokens.TextMuted
	case a.noteState == "error" || a.noteErr != "":
		return notebook.SaveStateLabel(true), tokens.DangerText
	case a.noteDirty:
		return notebook.SaveStateLabel(true), tokens.WarningText
	case a.noteState == "saved" || hasSavedNotebookEvent(a.snap.Events):
		return notebook.SaveStateLabel(false), tokens.SuccessText
	default:
		return "草稿", tokens.TextMuted
	}
}

func latestNotebookRevision(events []runs.RunEvent) (string, bool) {
	var latest *runs.RunEvent
	for i := range events {
		event := &events[i]
		if event.Stage != "notebook" {
			continue
		}
		if latest == nil || event.RecordedAt >= latest.RecordedAt {
			latest = event
		}
	}
	if latest == nil {
		return "", false
	}
	return latest.Content, true
}

func hasSavedNotebookEvent(events []runs.RunEvent) bool {
	_, ok := latestNotebookRevision(events)
	return ok
}

func (a *app) markNotebookDirty() {
	// A sealed record refuses to dirty: unlocking strips the signature
	// explicitly, so no edit path can silently break the lock.
	if _, ok := record.Verify(a.noteSource()); ok {
		return
	}
	if a.noteMode == 0 && a.noteDoc != nil {
		a.noteDraft = a.noteDoc.Text().Markdown()
	}
	a.noteDirty = true
	a.noteState = ""
	a.noteErr = ""
	a.persistNotebookDraft()
}

func (a *app) persistNotebookDraft() {
	if strings.TrimSpace(a.noteRunID) == "" {
		return
	}
	src := a.noteSource()
	if a.noteDrafts == nil {
		a.noteDrafts = map[string]notebookDraft{}
	}
	draft := notebookDraft{
		RunID:     a.noteRunID,
		Markdown:  src,
		Mode:      a.noteMode,
		UpdatedAt: time.Now().UTC().Format(time.RFC3339),
	}
	a.noteDrafts[a.noteRunID] = draft
	_ = saveNotebookDraftFile(a.noteRunID, src, a.noteMode)
}

func (a *app) clearNotebookDraft(runID string) {
	if strings.TrimSpace(runID) == "" {
		return
	}
	if a.noteDrafts != nil {
		delete(a.noteDrafts, runID)
	}
	clearNotebookDraftFile(runID)
}

func (a *app) discardNotebookDraft() {
	if a.snap.RunID != "" {
		a.clearNotebookDraft(a.snap.RunID)
	}
	content, hasSaved := latestNotebookRevision(a.snap.Events)
	a.noteDraft = content
	a.noteDirty = false
	a.noteRecovered = false
	a.noteErr = ""
	a.noteAster = nil
	if hasSaved {
		a.noteState = "saved"
	} else {
		a.noteState = ""
	}
	a.noteDoc = richtext.NewDocument(richtext.ParseMarkdown(a.noteDraft))
}

type notebookDraft struct {
	RunID     string `json:"runId"`
	Markdown  string `json:"markdown"`
	Mode      int    `json:"mode,omitempty"`
	UpdatedAt string `json:"updatedAt"`
}

func notebookDraftDataDir() string {
	if dir := strings.TrimSpace(os.Getenv("LABWORKBENCH_DATA_DIR")); dir != "" {
		return dir
	}
	if testing.Testing() {
		return ""
	}
	if dir, err := os.UserConfigDir(); err == nil && dir != "" {
		return filepath.Join(dir, "AsterLab")
	}
	return ""
}

func notebookDraftPath(runID string) string {
	dir := notebookDraftDataDir()
	if dir == "" || strings.TrimSpace(runID) == "" {
		return ""
	}
	sum := sha256.Sum256([]byte("notebook\x00" + runID))
	return filepath.Join(dir, "drafts", "notebook-"+hex.EncodeToString(sum[:16])+".json")
}

func saveNotebookDraftFile(runID, markdown string, mode int) error {
	p := notebookDraftPath(runID)
	if p == "" {
		return nil
	}
	raw, err := json.Marshal(notebookDraft{
		RunID:     runID,
		Markdown:  markdown,
		Mode:      mode,
		UpdatedAt: time.Now().UTC().Format(time.RFC3339),
	})
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, p)
}

func loadNotebookDraftFile(runID string) (notebookDraft, bool) {
	p := notebookDraftPath(runID)
	if p == "" {
		return notebookDraft{}, false
	}
	raw, err := os.ReadFile(p)
	if err != nil {
		return notebookDraft{}, false
	}
	var draft notebookDraft
	if err := json.Unmarshal(raw, &draft); err != nil || draft.RunID != runID {
		return notebookDraft{}, false
	}
	return draft, true
}

func clearNotebookDraftFile(runID string) {
	p := notebookDraftPath(runID)
	if p != "" {
		_ = os.Remove(p)
	}
}

func notebookEntryRawLabel(entry notebook.Entry) string {
	return strings.TrimSpace(entry.SourceType + " " + entry.SourceID)
}

func notebookEntrySummary(entry notebook.Entry, snap runs.RunSnapshot, taskTitles map[string]string) string {
	if entry.SourceType == "run" {
		if strings.TrimSpace(entry.Note) != "" {
			return "实验运行 · " + entry.Note
		}
		parts := []string{"实验运行"}
		if st := notebookStatusWord(snap.Status); st != "" {
			parts = append(parts, st)
		}
		if snap.DesignRevision > 0 {
			parts = append(parts, fmt.Sprintf("冻结设计版本 %d", snap.DesignRevision))
		}
		return strings.Join(parts, " · ")
	}
	parts := []string{notebookCategoryLabel(entry.Category)}
	if entry.TaskID != "" {
		if title := taskTitles[entry.TaskID]; title != "" {
			parts = append(parts, title)
		}
	}
	for _, detail := range entry.Details {
		switch detail.Label {
		case "单元格":
			parts = append(parts, detail.Value+"单元格")
		case "步骤状态":
			if st := notebookStatusWord(detail.Value); st != "" {
				parts = append(parts, st)
			}
		case "时间点":
			if taskTitle := taskTitles[entry.TaskID]; !strings.Contains(taskTitle, detail.Value) {
				parts = append(parts, detail.Value)
			}
		}
	}
	return strings.Join(parts, " · ")
}

func notebookCategoryLabel(category notebook.Category) string {
	switch category {
	case notebook.Measurement:
		return "测量记录"
	case notebook.Observation:
		return "观察记录"
	case notebook.Deviation:
		return "偏差记录"
	case notebook.Checklist:
		return "检查记录"
	case notebook.System:
		return "系统记录"
	default:
		return "执行记录"
	}
}

func notebookStatusWord(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "not_started", "pending", "idle", "created":
		return "未开始"
	case "running", "in_progress", "active":
		return "进行中"
	case "completed", "done", "passed":
		return "已完成"
	case "blocked", "failed", "error":
		return "阻塞"
	case "frozen":
		return "已冻结"
	case "draft":
		return "草稿"
	default:
		return ""
	}
}

func citationKindLabel(kind string) string {
	switch kind {
	case "plate":
		return "板图"
	case "batch":
		return "读数批次"
	case "readout":
		return "测量读数"
	case "event":
		return "运行事件"
	case "step":
		return "执行步骤"
	case "asset":
		return "实验文件"
	default:
		return "运行引用"
	}
}
// recordBoardPage renders the automatic record board (mode 图板): the shared
// RecordProjection as planned-vs-actual summary plus the representative
// Well x Timepoint x FOV matrix, with dynamic references, derived design
// parameters, and sign/lock controls. It writes no facts: inserting a matrix
// token edits prose, and signing persists prose through the existing save.
func (a *app) recordBoardPage(c *ui.Context, t *ui.Theme) {
	tokens := theme.GetTokens(t)
	proj := a.recordProjection()
	board := a.recordBoard()
	_, sigOK := record.Verify(a.noteSource())
	sealed := sigOK && !a.noteDirty
	state := record.ClassifyRecordState(a.noteDirty, sigOK)
	ui.Scroll(c).Grow(1).Children(func() {
		ui.Column(c).FillWidth().AlignItems(ui.Center).Children(func() {
			ui.Column(c).FillWidth().MaxWidth(960).MaxWidthPercent(100).
				Padding(t.Space(4), t.Space(5)).Gap(t.Space(2.5)).Children(func() {
				ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(t.Space(1.5)).Children(func() {
					ui.Text(c, "记录图板").FontSize(float32(tokens.TextBody)).Bold().TextColor(t.Text)
					components.Badge(c, string(state), components.BadgeOptions{Tone: recordBoardTone(state)})
					ui.Spacer(c)
					ui.Text(c, fmt.Sprintf("任务完成 %d/%d · 实测 %d 个 · 缺 %d 个 FOV 槽",
						proj.CompletedTasks(), len(proj.Summary), proj.MeasurementCount(), proj.Gaps)).
						FontSize(float32(tokens.TextData)).TextColor(t.TextMuted)
				})
				a.recordSummarySection(c, t, tokens, proj)
				a.recordMatrixSection(c, t, tokens, proj, board)
				a.recordRefsSection(c, t, tokens, proj, sealed)
				a.recordDerivedSection(c, t, tokens, proj)
				a.recordSignSection(c, t, tokens, proj, sealed)
			})
		})
	})
}

func recordBoardTone(state record.RecordState) components.BadgeTone {
	switch state {
	case record.StateSealed:
		return components.BadgeSuccess
	case record.StateUnsaved:
		return components.BadgeWarning
	default:
		return components.BadgeNeutral
	}
}

func recordBoardSection(c *ui.Context, t *ui.Theme, title string) {
	tokens := theme.GetTokens(t)
	ui.Text(c, title).FontSize(float32(tokens.TextBody)).Bold().TextColor(t.Text)
}

// recordSummarySection renders one planned-vs-actual row per frozen task from
// the shared projection. No surface re-derives it.
func (a *app) recordSummarySection(c *ui.Context, t *ui.Theme, tokens theme.Tokens, proj record.RecordProjection) {
	recordBoardSection(c, t, "计划 vs 实际")
	if len(proj.Summary) == 0 {
		ui.Text(c, "无冻结任务。").FontSize(float32(tokens.TextMeta)).TextColor(t.TextMuted)
		return
	}
	for _, sum := range proj.Summary {
		sum := sum
		title := strings.TrimSpace(sum.Title)
		if title == "" {
			title = strings.TrimSpace(sum.TaskKey)
		}
		head := title
		if strings.TrimSpace(sum.Timepoint) != "" {
			head += " · " + sum.Timepoint
		}
		if word := notebookStatusWord(sum.Status); word != "" {
			head += " · " + word
		}
		ui.Text(c, head).FontSize(float32(tokens.TextMeta)).Bold().TextColor(t.Text)
		actual := map[string]record.Param{}
		order := []string{}
		for _, p := range sum.Actual {
			if _, ok := actual[p.Key]; !ok {
				order = append(order, p.Key)
			}
			actual[p.Key] = p
		}
		for _, p := range sum.Planned {
			if _, ok := actual[p.Key]; !ok {
				order = append(order, p.Key)
				actual[p.Key] = record.Param{Key: p.Key, Label: p.Label, Unit: p.Unit}
			}
		}
		planned := map[string]record.Param{}
		for _, p := range sum.Planned {
			planned[p.Key] = p
		}
		for _, key := range order {
			act, plan := actual[key], planned[key]
			label := plan.Label
			if strings.TrimSpace(label) == "" {
				label = act.Label
			}
			if strings.TrimSpace(label) == "" {
				label = key
			}
			line := label + "：计划 " + orDash(plan.Value) + " → 实际 " + orDash(act.Value)
			unit := act.Unit
			if unit == "" {
				unit = plan.Unit
			}
			if unit != "" {
				line += " " + unit
			}
			ui.Text(c, line).FontSize(float32(tokens.TextData)).TextColor(t.TextMuted)
		}
		if sum.Skipped {
			ui.Text(c, "条件跳过：按规则跳过").FontSize(float32(tokens.TextData)).TextColor(t.Warning)
		}
		for _, dev := range sum.Deviations {
			ui.Text(c, "偏差："+dev).FontSize(float32(tokens.TextData)).TextColor(t.Danger)
		}
	}
}

func orDash(value string) string {
	if strings.TrimSpace(value) == "" {
		return "—"
	}
	return value
}

// recordMatrixSection renders the representative Well x Timepoint matrix for
// the selected FOV layer. Clicking FOV 1–6 switches the layer; empty slots
// read as gaps, never placeholders.
func (a *app) recordMatrixSection(c *ui.Context, t *ui.Theme, tokens theme.Tokens, proj record.RecordProjection, board *recordBoardView) {
	recordBoardSection(c, t, "代表视野矩阵（孔位 × 时间点 × FOV）")
	ui.Row(c).FillWidth().Wrap().AlignItems(ui.Center).Gap(t.Space(1)).Children(func() {
		for fov := 1; fov <= record.FOVSlots; fov++ {
			fov := fov
			style := components.ButtonGhost
			if board.fov == fov {
				style = components.ButtonSecondary
			}
			if components.Button(c, fmt.Sprintf("FOV %d", fov), style).Clicked() {
				board.fov = fov
			}
		}
	})
	if len(proj.Matrix) == 0 {
		ui.Text(c, "无实测孔位数据。").FontSize(float32(tokens.TextMeta)).TextColor(t.TextMuted)
		return
	}
	for _, cell := range proj.Matrix {
		cell := cell
		main := record.MatrixCellText(cell)
		img := cell.Representative(board.fov)
		sub := fmt.Sprintf("FOV %d 缺", board.fov)
		if img != nil {
			sub = "图·" + img.Label
		}
		ui.Row(c).FillWidth().Wrap().AlignItems(ui.Center).Gap(t.Space(1.5)).Children(func() {
			ui.Text(c, cell.Well+" · "+cell.Timepoint).FontSize(float32(tokens.TextData)).Bold().TextColor(t.Text)
			ui.Text(c, main).FontSize(float32(tokens.TextData)).TextColor(t.TextMuted)
			ui.Text(c, sub).FontSize(float32(tokens.TextData)).TextColor(t.TextMuted)
			if img != nil {
				if components.Button(c, "打开原图", components.ButtonGhost).Clicked() {
					err := a.openRunAsset(runworkspace.LinkedAsset{
						AssetID: img.AssetID, RootID: img.RootID, RelativePath: img.RelativePath,
						MediaType: "image",
						LinkedTo:  runworkspace.AssetLink{RunID: a.snap.RunID},
					})
					if err != nil {
						c.AddToast(ui.Toast{Title: "打开原图失败", Description: err.Error(), Type: components.ToastError})
					}
				}
			}
		})
	}
	if len(proj.UnassignedLabels) > 0 {
		ui.Text(c, fmt.Sprintf("未定位图 %d 张：%s", proj.UnassignedImages, strings.Join(proj.UnassignedLabels, "、"))).
			FontSize(float32(tokens.TextData)).TextColor(t.Warning)
	}
}

// recordRefsSection lists the dynamic references in the current prose and
// offers one-tap insertion of timepoint tokens. Insertion edits prose only.
func (a *app) recordRefsSection(c *ui.Context, t *ui.Theme, tokens theme.Tokens, proj record.RecordProjection, sealed bool) {
	recordBoardSection(c, t, "动态引用 @matrix(…)")
	refs := record.FindMatrixRefs(a.noteSource())
	if len(refs) == 0 {
		ui.Text(c, "正文暂无矩阵引用。").FontSize(float32(tokens.TextMeta)).TextColor(t.TextMuted)
	} else {
		for _, ref := range refs {
			ref := ref
			cells, unknown := record.ResolveMatrixRef(proj, ref)
			line := ref.Raw + " → " + fmt.Sprintf("%d 个单元格", len(cells))
			if len(unknown) > 0 {
				line += "（未记录时间点 " + strings.Join(unknown, "、") + "）"
			}
			ui.Text(c, line).FontSize(float32(tokens.TextData)).TextColor(t.TextMuted)
		}
	}
	if sealed || len(proj.Timepoints) == 0 {
		return
	}
	ui.Row(c).FillWidth().Wrap().AlignItems(ui.Center).Gap(t.Space(1)).Children(func() {
		for _, tp := range proj.Timepoints {
			tp := tp
			if components.Button(c, "插入 @matrix("+tp+")", components.ButtonGhost).Clicked() {
				tok := "@matrix(" + tp + ")"
				src := strings.TrimSpace(a.noteSource())
				if src != "" {
					src += "\n\n" + tok
				} else {
					src = tok
				}
				a.noteDraft = src
				a.markNotebookDirty()
			}
		}
	})
}

// recordDerivedSection renders the new-design draft grown from measured data,
// with its provenance, plus a JSON handoff export.
func (a *app) recordDerivedSection(c *ui.Context, t *ui.Theme, tokens theme.Tokens, proj record.RecordProjection) {
	recordBoardSection(c, t, "派生设计（源自实测）")
	derived := record.DeriveDesign(proj)
	if len(derived.Parameters) == 0 {
		ui.Text(c, "无实测参数可派生。").FontSize(float32(tokens.TextMeta)).TextColor(t.TextMuted)
		return
	}
	ui.Text(c, derived.Title+" · 源自 "+derived.SourceRunID+" · 设计 rev "+fmt.Sprintf("%d", derived.SourceDesignRevision)).
		FontSize(float32(tokens.TextData)).TextColor(t.TextMuted)
	for _, param := range derived.Parameters {
		param := param
		line := param.Label + "：计划 " + orDash(param.Planned) + " → 实测 " + orDash(param.Measured)
		if param.Unit != "" {
			line += " " + param.Unit
		}
		ui.Row(c).FillWidth().Wrap().AlignItems(ui.Center).Gap(t.Space(1.5)).Children(func() {
			ui.Text(c, line).FontSize(float32(tokens.TextData)).TextColor(t.Text)
			ui.Text(c, param.Source).FontSize(float32(tokens.TextData)).TextColor(t.TextMuted)
		})
	}
	if components.Button(c, "导出派生参数", components.ButtonSecondary).Clicked() {
		a.exportRecordFile(c, "导出派生参数", "derived-design-"+a.snap.RunID+".json", derived.JSON())
	}
}

// recordSignSection hosts sign/lock and archive export. Signing persists the
// prose through the existing save path with a signature trailer; the lock is
// the verified signature, not a separate flag.
func (a *app) recordSignSection(c *ui.Context, t *ui.Theme, tokens theme.Tokens, proj record.RecordProjection, sealed bool) {
	recordBoardSection(c, t, "签核与归档")
	if sealed {
		sig, _ := a.recordSeal()
		ui.Row(c).FillWidth().Wrap().AlignItems(ui.Center).Gap(t.Space(1.5)).Children(func() {
			ui.Text(c, "已签核归档 · "+sig.Actor+" · "+sig.At).
				FontSize(float32(tokens.TextData)).Bold().TextColor(tokens.SuccessText)
			ui.Spacer(c)
			if components.Button(c, "解锁", components.ButtonSecondary).Clicked() {
				a.unlockRecord()
			}
		})
		ui.Text(c, "记录已锁定：切换到笔记页查看只读正文，或解锁后继续编辑。").
			FontSize(float32(tokens.TextData)).TextColor(t.TextMuted)
	} else {
		ui.Row(c).FillWidth().Wrap().AlignItems(ui.Center).Gap(t.Space(1.5)).Children(func() {
			if components.Button(c, "签核归档", components.ButtonSecondary).Clicked() {
				a.signRecord()
			}
			ui.Text(c, "签核保存正文并锁定编辑；归档包在实验方案页导出。").
				FontSize(float32(tokens.TextData)).TextColor(t.TextMuted)
		})
	}
	if components.Button(c, "导出归档包", components.ButtonGhost).Clicked() {
		arch := protocol.BuildArchive(proj, a.noteSource(), time.Now())
		a.exportRecordFile(c, "导出归档包", "record-"+a.snap.RunID+".md", arch.Markdown())
	}
}

// recordLockedNotice replaces every editing mode while the record is sealed.
func (a *app) recordLockedNotice(c *ui.Context, t *ui.Theme) {
	tokens := theme.GetTokens(t)
	ui.Column(c).Grow(1).FillWidth().AlignItems(ui.Center).Children(func() {
		ui.Column(c).Grow(1).FillWidth().MaxWidth(820).MaxWidthPercent(100).
			Gap(t.Space(2)).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "已签核归档").FontSize(float32(tokens.TextBody)).Bold().TextColor(t.Text)
			ui.Text(c, "记录已锁定为只读。切换到“阅读”查看正文，或解锁后继续编辑。").
				FontSize(float32(tokens.TextMeta)).TextColor(t.TextMuted)
			if components.Button(c, "解锁", components.ButtonSecondary).Clicked() {
				a.unlockRecord()
			}
		})
	})
}

// signRecord signs the current prose and persists it through the existing
// Notebook save path, then moves to reading mode under the lock.
func (a *app) signRecord() {
	if a.runSvc == nil {
		a.noteErr = "运行服务不可用"
		a.noteState = "error"
		return
	}
	source := strings.TrimSpace(a.noteSource())
	if source == "" {
		a.noteErr = "Notebook 不能为空"
		a.noteState = "error"
		return
	}
	if len([]rune(source)) > 4800 {
		a.noteErr = "内容过长，签核前请精简（签核附带约 200 字符）。"
		a.noteState = "error"
		return
	}
	signed, _ := record.Sign(source, a.snap.RunID, a.snap.DesignRevision, "local-user", time.Now())
	a.noteDraft = signed
	a.noteDoc = richtext.NewDocument(richtext.ParseMarkdown(signed))
	a.saveNotebook()
	if a.noteState == "saved" {
		a.setNoteMode(2)
	}
}

// unlockRecord strips the signature trailer and returns the prose to the
// editable 未保存 state. Unlocking never edits the prose itself.
func (a *app) unlockRecord() {
	a.noteDraft = record.StripSignature(a.noteSource())
	a.noteDoc = richtext.NewDocument(richtext.ParseMarkdown(a.noteDraft))
	a.markNotebookDirty()
}

// exportRecordFile writes a record artifact (derived JSON or archive
// Markdown) through a save dialog, following the document export pattern.
func (a *app) exportRecordFile(c *ui.Context, title, filename, content string) {
	home, _ := os.UserHomeDir()
	path, err := mygo.Dialog.Save(mygo.SaveDialogOptions{
		Parent: mygo.FocusedWindow(), Title: title, DefaultPath: filepath.Join(home, filename),
	})
	if err != nil {
		c.AddToast(ui.Toast{Title: title + "失败", Description: err.Error(), Type: components.ToastError})
		return
	}
	if path == "" {
		return
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		c.AddToast(ui.Toast{Title: title + "失败", Description: err.Error(), Type: components.ToastError})
		return
	}
	c.AddToast(ui.Toast{Title: filename + " 已保存", Description: path, Type: components.ToastSuccess})
}
