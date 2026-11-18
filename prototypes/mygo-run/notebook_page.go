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

	"asterlab-mygo-prototype/internal/assets"
	"asterlab-mygo-prototype/internal/asterdoc"
	"asterlab-mygo-prototype/internal/notebook"
	"asterlab-mygo-prototype/internal/notebook/record"
	"asterlab-mygo-prototype/internal/protocol"
	"asterlab-mygo-prototype/internal/resultgrid"
	"asterlab-mygo-prototype/internal/runworkspace"
	"asterlab-mygo-prototype/internal/state"
	"asterlab-mygo-prototype/internal/theme"
	"asterlab-mygo-prototype/modules/runs"
	mygo "github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/richtext"
	"github.com/oneo-me/onevra/source/components"
	"github.com/oneo-me/onevra/source/icons/lucide"
	_ "golang.org/x/image/tiff" // preview TIFFs: ui.DecodeImage has no TIFF format until registered
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
	// imgs caches decoded previews by asset ID so frames never re-read the
	// files; imgFailed remembers unloadable assets to avoid retrying them
	// every frame; imgLoading marks loads a worker has not finished yet so
	// the board reads "loading" instead of retrying or flashing unreadable.
	// runID and fovCached name the Run and FOV layer the cache belongs to;
	// gen counts their changes so late worker results for an older layer
	// drop instead of landing in the wrong cell. All maps move on the view
	// thread only; the worker carries plain values, never the maps.
	imgs       map[string]ui.ImageSource
	imgFailed  map[string]bool
	imgLoading map[string]bool
	runID      string
	fovCached  int
	gen        uint64
}

// recordBoards keys board state by Run so switching Runs never leaks one
// Run's FOV selection into another.
var recordBoards = map[string]*recordBoardView{}

// releaseBoardPreviews drops decoded CPU pixels when a cache stops being the
// active Run/FOV layer. It runs on the view thread, where ReleasePixels is
// safe per the MyGo docs (call it once the bitmap has shown, on the main
// thread); non-bitmap sources have nothing to release.
func releaseBoardPreviews(board *recordBoardView) {
	for id, src := range board.imgs {
		if bmp, ok := src.(*ui.Bitmap); ok && bmp != nil {
			bmp.ReleasePixels()
		}
		delete(board.imgs, id)
	}
}

func (a *app) recordBoard() *recordBoardView {
	// Bound the cache to the active Run: leaving a Run releases its
	// previews instead of retaining every visited Run's bitmaps.
	for id, other := range recordBoards {
		if id != a.snap.RunID {
			releaseBoardPreviews(other)
			delete(recordBoards, id)
		}
	}
	st := recordBoards[a.snap.RunID]
	if st == nil {
		st = &recordBoardView{fov: 1, imgs: map[string]ui.ImageSource{}, imgFailed: map[string]bool{}}
		recordBoards[a.snap.RunID] = st
	}
	if limit := max(6, a.plannedDocFOV()); st.fov < 1 || st.fov > limit {
		st.fov = 1
	}
	if st.imgs == nil {
		st.imgs = map[string]ui.ImageSource{}
	}
	if st.imgFailed == nil {
		st.imgFailed = map[string]bool{}
	}
	if st.imgLoading == nil {
		st.imgLoading = map[string]bool{}
	}
	// Bound the cache to the selected FOV layer: switching layers releases
	// the old layer's previews, and gen retires its in-flight loads.
	if st.runID != a.snap.RunID || st.fovCached != st.fov {
		releaseBoardPreviews(st)
		st.imgFailed = map[string]bool{}
		st.imgLoading = map[string]bool{}
		st.runID = a.snap.RunID
		st.fovCached = st.fov
		st.gen++
	}
	return st
}

// recordProjection is the single shared read model for this surface: the
// automatic board, the protocol archive, and the Run history audit rendering
// all project from Core through the app memo (fetchRecord), so neither
// surface re-derives summaries from the snapshot on its own. The error
// string is non-empty when Core refused; surfaces show it (UX-P5).
func (a *app) recordProjection() (record.Projection, []record.TaskStatus, string) {
	_, _, _ = a.fetchRecord()
	if a.recProj != nil {
		return *a.recProj, a.recTasks, a.recErr
	}
	return record.Projection{}, a.recTasks, a.recErr
}

// recordSealed reports the persisted Core lifecycle: only signed/archived
// state locks editing. The prose trailer is history-only, never a lock.
func (a *app) recordSealed() bool {
	_, _, _ = a.recordProjection()
	return a.recordArchived()
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
		sealed := a.recordSealed()
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
					// The sealed record locks the editing modes; only the
					// read-only board (mode 4) and reading stay reachable.
					// Sign/archive is final: there is no unlock.
					if sealed && a.noteMode != 2 && a.noteMode != 4 {
						a.noteMode = old
						c.AddToast(ui.Toast{Title: "已签核归档", Description: "记录已锁定为终态归档，不可撤销。切换到阅读查看正文。", Type: components.ToastWarning})
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
	if a.recordSealed() && a.noteMode != 2 && a.noteMode != 4 {
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
	// A signed/archived record opens in reading mode: only persistent Core
	// state locks editing, never the prose trailer. Legacy trailers are
	// stripped for display at the board/archive boundaries.
	if a.recordArchived() {
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
	// Archived guard: Core locked the notes at sign/archive, so no save
	// may land (UX-P5 surfaces the refusal instead of dropping prose).
	if a.recordArchived() {
		a.noteErr = "记录已签核归档，笔记已锁定"
		a.noteState = "error"
		return
	}
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
	// A signed/archived Core state outranks everything: the notes are
	// locked. Only persistent Core state is authority (see recordSealed).
	if a.recordSealed() {
		return "已签核归档", tokens.SuccessText
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
	// A signed/archived record refuses to dirty: only persistent Core
	// state locks editing.
	if !a.noteDirty && a.recordArchived() {
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

// recordBoardPage renders the automatic record board (mode 图板): Core's
// shared RecordProjection as planned-vs-actual summary plus the
// representative Well x Timepoint x FOV matrix, with dynamic references,
// derived design parameters, and sign/lock controls. It writes no facts:
// inserting a matrix token edits prose, and sign/archive is a Core command.
func (a *app) recordBoardPage(c *ui.Context, t *ui.Theme) {
	tokens := theme.GetTokens(t)
	proj, tasks, fetchErr := a.recordProjection()
	board := a.recordBoard()
	counts := record.Summarize(proj, tasks)
	sealed := a.recordArchived()
	state := record.ArchiveStatusWord(proj.Archive.Status)
	ui.Scroll(c).Grow(1).Children(func() {
		ui.Column(c).FillWidth().AlignItems(ui.Center).Children(func() {
			// UX-P4 / UX-LAYOUT: the board is a structured table (summary rows
			// plus a Well x Timepoint matrix), not a prose paper, so it takes
			// a wider measure than the 820 prose cap. Counts use tabular
			// numerals like the rest of the surface.
			ui.Column(c).FillWidth().MaxWidth(960).MaxWidthPercent(100).
				Padding(t.Space(4), t.Space(5)).Gap(t.Space(2.5)).Children(func() {
				ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(t.Space(1.5)).Children(func() {
					ui.Text(c, "记录图板").FontSize(float32(tokens.TextBody)).Bold().TextColor(t.Text)
					components.Badge(c, state, components.BadgeOptions{Tone: recordBoardTone(proj.Archive.Status)})
					ui.Spacer(c)
					ui.Text(c, record.Headline(counts)).
						FontSize(float32(tokens.TextData)).TextColor(t.TextMuted).FontFeatures("tnum")
				})
				if strings.TrimSpace(fetchErr) != "" {
					ui.Text(c, "记录读取失败："+fetchErr).FontSize(float32(tokens.TextData)).TextColor(t.Danger)
				}
				a.recordSummarySection(c, t, tokens, proj, tasks)
				a.recordTypedReadoutsSection(c, t, tokens, proj, tasks)
				a.recordGroupBoardSection(c, t, tokens, proj, board)
				a.recordMatrixSection(c, t, tokens, proj, board)
				a.recordRefsSection(c, t, tokens, proj, sealed)
				a.recordDerivedSection(c, t, tokens, proj)
				a.recordSignSection(c, t, tokens, proj, tasks, sealed)
			})
		})
	})
}

func recordBoardTone(status string) components.BadgeTone {
	switch status {
	case "signed", "archived":
		return components.BadgeSuccess
	case "summarizing":
		return components.BadgeWarning
	default:
		return components.BadgeNeutral
	}
}

func recordBoardSection(c *ui.Context, t *ui.Theme, title string) {
	tokens := theme.GetTokens(t)
	ui.Text(c, title).FontSize(float32(tokens.TextBody)).Bold().TextColor(t.Text)
}

// recordSummarySection renders the frozen plan skeleton plus Core's
// planned-vs-actual parameter groups. Tasks come from the Core-delivered
// snapshot; comparisons come from the Core projection. No surface
// re-derives either.
func (a *app) recordSummarySection(c *ui.Context, t *ui.Theme, tokens theme.Tokens, proj record.Projection, tasks []record.TaskStatus) {
	recordBoardSection(c, t, "计划 vs 实际")
	if len(tasks) == 0 {
		ui.Text(c, "无冻结任务。").FontSize(float32(tokens.TextMeta)).TextColor(t.TextMuted)
	} else {
		for _, task := range tasks {
			task := task
			head := strings.TrimSpace(task.Title)
			if head == "" {
				head = task.Key
			}
			if strings.TrimSpace(task.Timepoint) != "" {
				head += " · " + task.Timepoint
			}
			if word := record.TaskStatusWord(task.Status); word != "" {
				head += " · " + word
			}
			ui.Text(c, head).FontSize(float32(tokens.TextMeta)).Bold().TextColor(t.Text)
		}
	}
	for _, group := range record.GroupParameters(proj.Parameters) {
		group := group
		ui.Text(c, group.Title).FontSize(float32(tokens.TextData)).Bold().TextColor(t.Text)
		for _, row := range group.Rows {
			row := row
			ui.Text(c, row.Label+"：计划 "+row.Planned+" → 实际 "+row.Actual+"（"+row.Status+"）").
				FontSize(float32(tokens.TextData)).TextColor(t.TextMuted)
		}
	}
	for _, issue := range proj.Issues {
		issue := issue
		ui.Text(c, "待核实："+firstNonEmpty(strings.TrimSpace(issue.Message), issue.Code)).FontSize(float32(tokens.TextData)).TextColor(t.Warning)
	}
}

func (a *app) recordTypedReadoutsSection(c *ui.Context, t *ui.Theme, tokens theme.Tokens, proj record.Projection, tasks []record.TaskStatus) {
	decoded := record.DecodeReadouts(proj.Readouts)
	if len(decoded) == 0 {
		return
	}
	taskTitleByKey := map[string]string{}
	for _, tk := range tasks {
		if strings.TrimSpace(tk.Key) != "" && strings.TrimSpace(tk.Title) != "" {
			taskTitleByKey[tk.Key] = strings.TrimSpace(tk.Title)
		}
	}
	recordBoardSection(c, t, "结构化表格与分类读数")
	for _, dr := range decoded {
		dr := dr
		cardTitle := dr.SchemaID
		if cardTitle == "" || asterdoc.IsRawIdentifier(cardTitle) {
			switch dr.Kind {
			case "table":
				cardTitle = "实验测定数据表"
			case "categorical":
				cardTitle = "定性观测记录"
			default:
				cardTitle = "定量测定读数"
			}
		}
		stepLabel := ""
		if title := taskTitleByKey[dr.StepPath]; title != "" {
			stepLabel = title
		} else if dr.StepPath != "" {
			for k, title := range taskTitleByKey {
				if strings.HasSuffix(dr.StepPath, k) {
					stepLabel = title
					break
				}
			}
			if stepLabel == "" {
				if asterdoc.IsRawIdentifier(dr.StepPath) {
					stepLabel = "实测记录步骤"
				} else {
					stepLabel = dr.StepPath
				}
			}
		}
		ui.Column(c).FillWidth().Padding(float32(tokens.Spacing12)).Radius(float32(tokens.PanelRadius)).
			Background(tokens.Surface).Border(1, tokens.Border).Gap(float32(tokens.Spacing8)).Children(func() {
			ui.Row(c).FillWidth().Wrap().AlignItems(ui.Center).Gap(float32(tokens.Spacing8)).Children(func() {
				ui.Text(c, cardTitle).FontSize(float32(tokens.TextData)).Bold().TextColor(t.Text)
				if stepLabel != "" {
					components.Badge(c, stepLabel)
				}
				if dr.Timepoint != "" {
					components.Badge(c, dr.Timepoint, components.BadgeOptions{Tone: components.BadgeAccent})
				}
				ui.Spacer(c)
				switch dr.Kind {
				case "table":
					components.Badge(c, fmt.Sprintf("表格 · %d 行", len(dr.Rows)))
				case "categorical":
					components.Badge(c, "分类 · "+orDash(dr.Category), components.BadgeOptions{Tone: components.BadgeSuccess})
				case "numeric":
					if dr.Number != nil {
						val := fmt.Sprintf("%g", *dr.Number)
						if dr.Unit != "" {
							val += " " + dr.Unit
						}
						components.Badge(c, val, components.BadgeOptions{Tone: components.BadgeSuccess})
					}
				}
			})
			if dr.Kind == "table" && len(dr.Columns) > 0 && len(dr.Rows) > 0 {
				cols := make([]ui.TableColumn, 0, len(dr.Columns))
				for _, cname := range dr.Columns {
					cols = append(cols, ui.TableColumn{ID: cname, Title: cname, MinWidth: 96})
				}
				components.Table(c, nil, cols, func() {
					for rIdx, row := range dr.Rows {
						rIdx, row := rIdx, row
						rowKey := fmt.Sprintf("%s-r%d", dr.SchemaID, rIdx)
						components.TableRow(c, rIdx, rowKey, false, func(col int) {
							cellTxt := "—"
							if col < len(row) {
								cellTxt = row[col]
							}
							ui.Text(c, cellTxt).FontSize(float32(tokens.TextData)).TextColor(t.Text).FontFeatures("tnum")
						})
					}
				})
			}
		})
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
// read as gaps, never placeholders. Slots come from Core's planned matrix,
// never from uploaded assets.
func (a *app) recordMatrixSection(c *ui.Context, t *ui.Theme, tokens theme.Tokens, proj record.Projection, board *recordBoardView) {
	recordBoardSection(c, t, "代表视野矩阵（孔位 × 时间点 × FOV）")
	ui.Row(c).FillWidth().Wrap().AlignItems(ui.Center).Gap(t.Space(1)).Children(func() {
		maxFOV := max(6, max(int(proj.Images.FOVPerWell), a.plannedDocFOV()))
		for fov := 1; fov <= maxFOV; fov++ {
			fov := fov
			style := components.ButtonGhost
			if board.fov == fov {
				style = components.ButtonSecondary
			}
			if components.Button(c, fmt.Sprintf("FOV %d", fov), style).Clicked() {
				board.fov = fov
				a.invalidateRecord()
			}
		}
	})
	cells := record.MatrixCells(proj)
	if len(cells) == 0 {
		ui.Text(c, "无实测孔位数据。").FontSize(float32(tokens.TextMeta)).TextColor(t.TextMuted)
	} else {
		for _, cell := range cells {
			cell := cell
			main := record.MatrixCellText(cell)
			img := record.SlotImage(proj, cell.Well, cell.Timepoint, board.fov)
			sub := fmt.Sprintf("FOV %d 缺", board.fov)
			if img != nil {
				sub = "图·" + img.AssetID
			}
			ui.Row(c).FillWidth().Wrap().AlignItems(ui.Center).Gap(t.Space(1.5)).Children(func() {
				ui.Text(c, cell.Well+" · "+cell.Timepoint).FontSize(float32(tokens.TextData)).Bold().TextColor(t.Text)
				ui.Text(c, main).FontSize(float32(tokens.TextData)).TextColor(t.TextMuted)
				ui.Text(c, sub).FontSize(float32(tokens.TextData)).TextColor(t.TextMuted)
				if img != nil {
					if components.Button(c, "打开原图", components.ButtonGhost).Clicked() {
						if err := a.openPlacedImage(*img); err != nil {
							c.AddToast(ui.Toast{Title: "打开原图失败", Description: err.Error(), Type: components.ToastError})
						}
					}
				}
			})
		}
	}
	if len(proj.Images.UnlocatedAssetIDs) > 0 {
		ui.Text(c, fmt.Sprintf("未定位图 %d 张：%s", len(proj.Images.UnlocatedAssetIDs), strings.Join(proj.Images.UnlocatedAssetIDs, "、"))).
			FontSize(float32(tokens.TextData)).TextColor(t.Warning)
	}
	if len(proj.Images.UnplannedAssetIDs) > 0 {
		ui.Text(c, fmt.Sprintf("计划外图 %d 张：%s", len(proj.Images.UnplannedAssetIDs), strings.Join(proj.Images.UnplannedAssetIDs, "、"))).
			FontSize(float32(tokens.TextData)).TextColor(t.Warning)
	}
}

// openPlacedImage opens one Core-placed matrix image through the linked
// asset with the same ID. Coordinates are never inferred from names: the
// asset must be linked, or the open names it instead of guessing.
func (a *app) openPlacedImage(img record.PlacedImage) error {
	for _, asset := range a.runAssets {
		if asset.ID != img.AssetID {
			continue
		}
		return a.openRunAsset(runworkspace.LinkedAsset{
			AssetID: asset.ID, RootID: asset.RootID, RelativePath: asset.RelativePath,
			MediaType: "image",
			LinkedTo:  runworkspace.AssetLink{RunID: a.snap.RunID},
		})
	}
	return fmt.Errorf("asset %q 未关联到本 Run", img.AssetID)
}

// recordRefsSection lists the dynamic references in the current prose and
// offers one-tap insertion of timepoint tokens. Insertion edits prose only.
// Legacy signature trailers never count as content: display strips them.
func (a *app) recordRefsSection(c *ui.Context, t *ui.Theme, tokens theme.Tokens, proj record.Projection, sealed bool) {
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
	timepoints := record.MatrixTimepoints(proj)
	if sealed || len(timepoints) == 0 {
		return
	}
	ui.Row(c).FillWidth().Wrap().AlignItems(ui.Center).Gap(t.Space(1)).Children(func() {
		for _, tp := range timepoints {
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

// recordDerivedSection reviews the next-design draft grown from measured
// data, with its provenance: DeriveDesignFromRun writes nothing, so the
// operator reviews evidence before SaveDesign persists it. Surface a Core
// refusal (e.g. AsterDoc-managed designs) instead of failing silently.
func (a *app) recordDerivedSection(c *ui.Context, t *ui.Theme, tokens theme.Tokens, proj record.Projection) {
	recordBoardSection(c, t, "派生设计（源自实测）")
	if a.deriveEv == nil {
		if components.Button(c, "从实测派生新设计", components.ButtonSecondary).Clicked() {
			a.deriveDesignFromRun()
		}
		if strings.TrimSpace(a.deriveErr) != "" {
			ui.Text(c, "派生失败："+a.deriveErr).FontSize(float32(tokens.TextData)).TextColor(t.Danger)
		}
		ui.Text(c, "派生只读取已冻结 Run 的实测参数与计时，不写入任何内容；保存前可逐条核对。").
			FontSize(float32(tokens.TextData)).TextColor(t.TextMuted)
		return
	}
	ev := a.deriveEv
	ui.Text(c, fmt.Sprintf("源自 Run %s · 设计 %s rev %d", ev.SourceRunID, ev.DesignID, ev.SourceDesignRevision)).
		FontSize(float32(tokens.TextData)).TextColor(t.TextMuted)
	if strings.TrimSpace(ev.Title) != "" {
		ui.Text(c, ev.Title).FontSize(float32(tokens.TextData)).Bold().TextColor(t.Text)
	}
	for _, param := range ev.Params {
		label := param.ParameterKey
		if param.Label != nil && strings.TrimSpace(*param.Label) != "" {
			label = *param.Label
		}
		line := label + "：实测 " + strings.TrimSpace(string(param.Actual))
		if strings.TrimSpace(string(param.Planned)) != "" {
			line += "（计划 " + strings.TrimSpace(string(param.Planned)) + "）"
		}
		line += " · 事件 " + param.EventID
		ui.Text(c, line).FontSize(float32(tokens.TextData)).TextColor(t.Text)
	}
	for _, dur := range ev.Durations {
		ui.Text(c, dur.TaskKey+" · "+dur.TimerID+"：实测 "+fmt.Sprint(dur.ActualS)+"s").
			FontSize(float32(tokens.TextData)).TextColor(t.Text)
	}
	if strings.TrimSpace(ev.Note) != "" {
		ui.Text(c, ev.Note).FontSize(float32(tokens.TextData)).TextColor(t.TextMuted)
	}
	if strings.TrimSpace(a.deriveErr) != "" {
		ui.Text(c, "保存失败："+a.deriveErr).FontSize(float32(tokens.TextData)).TextColor(t.Danger)
	}
	ui.Row(c).FillWidth().Wrap().AlignItems(ui.Center).Gap(t.Space(1.5)).Children(func() {
		if components.Button(c, "保存为新设计版本", components.ButtonSecondary).Clicked() {
			a.persistDerivedDesign()
		}
		if components.Button(c, "重新派生", components.ButtonGhost).Clicked() {
			a.deriveEv = nil
			a.deriveDesign = nil
			a.deriveErr = ""
			a.deriveConfirm = false
		}
	})
}

// deriveDesignFromRun asks Core for the next-design draft and decodes its
// evidence for review. It writes nothing; persistence is a separate,
// explicit step.
func (a *app) deriveDesignFromRun() {
	a.deriveErr = ""
	a.deriveEv = nil
	a.deriveDesign = nil
	if a.designSvc == nil || a.snap.RunID == "" {
		a.deriveErr = "设计服务不可用"
		return
	}
	derived, err := a.designSvc.DeriveDesignFromRun(context.Background(), runs.DeriveDesignInput{RunID: a.snap.RunID})
	if err != nil {
		a.deriveErr = state.MapError(err).Error()
		return
	}
	ev, err := record.DecodeDerived(derived)
	if err != nil {
		a.deriveErr = err.Error()
		return
	}
	cp := derived
	ecp := ev
	a.deriveDesign = &cp
	a.deriveEv = &ecp
}

// persistDerivedDesign shapes the reviewed draft into SaveDesign changes and
// persists it, then opens the new design revision. A Core refusal names the
// cause; nothing is partially written.
func (a *app) persistDerivedDesign() {
	a.deriveErr = ""
	if a.designSvc == nil || a.deriveDesign == nil || a.deriveEv == nil {
		a.deriveErr = "无可保存的派生草案"
		return
	}
	snap, err := a.designSvc.LoadDesign(context.Background(), a.deriveDesign.DesignID)
	if err != nil {
		a.deriveErr = state.MapError(err).Error()
		return
	}
	changes, err := record.ChangesForDerived(snap.Data, *a.deriveDesign, *a.deriveEv, time.Now())
	if err != nil {
		a.deriveErr = err.Error()
		return
	}
	asterSrc := strings.TrimSpace(a.deriveEv.AsterDocSource)
	saved, err := a.designSvc.SaveDesign(context.Background(), runs.DesignSaveInput{
		DesignID: a.deriveDesign.DesignID, ExpectedRevision: uint64(a.deriveDesign.ExpectedRevision),
		Changes: changes, OperationID: a.nextOp(),
	})
	if err != nil {
		a.deriveErr = state.MapError(err).Error()
		return
	}
	if asterSrc != "" {
		var expRev uint64
		if curDoc, loadErr := a.designSvc.LoadAsterDoc(context.Background(), saved.DesignID); loadErr == nil {
			expRev = curDoc.Revision
		}
		if _, docErr := a.designSvc.SaveAsterDoc(context.Background(), runs.AsterDocSaveInput{
			DesignID: saved.DesignID, Source: asterSrc, ExpectedRevision: expRev,
			OperationID: a.nextOp(), ActorID: os.Getenv("USERNAME"),
		}); docErr != nil {
			a.deriveErr = state.MapError(docErr).Error()
			return
		}
	}
	a.deriveEv = nil
	a.deriveDesign = nil
	a.loadDesign(saved.DesignID)
}

// recordSignSection hosts sign/lock and archive export. Sign/archive is one
// Core command (signed and archived states are never re-signed); the editing
// lock is the persisted Core state, never prose. The confirmation names the
// 缺/计划外 counts the operator acknowledges.
func (a *app) recordSignSection(c *ui.Context, t *ui.Theme, tokens theme.Tokens, proj record.Projection, tasks []record.TaskStatus, sealed bool) {
	recordBoardSection(c, t, "签核与归档")
	counts := record.Summarize(proj, tasks)
	if sealed {
		ui.Row(c).FillWidth().Wrap().AlignItems(ui.Center).Gap(t.Space(1.5)).Children(func() {
			line := "已签核归档"
			if sig := proj.Archive.Signature; sig != nil {
				if strings.TrimSpace(sig.SignedBy) != "" {
					line += " · " + sig.SignedBy
				}
				if strings.TrimSpace(sig.SignedAtUTC) != "" {
					line += " · " + sig.SignedAtUTC
				}
			}
			ui.Text(c, line).FontSize(float32(tokens.TextData)).Bold().TextColor(tokens.SuccessText)
		})
		ui.Text(c, "记录已锁定：签核归档不可撤销，归档包在实验方案页导出。").
			FontSize(float32(tokens.TextData)).TextColor(t.TextMuted)
	} else if a.signConfirm {
		ui.Text(c, fmt.Sprintf("确认签核？实测 %d 个 · 缺 %d 个 · 排除 %d 个（含计划外），签核后记录锁定、不可撤销。",
			counts.Measurements, counts.Missing, counts.Excluded)).
			FontSize(float32(tokens.TextData)).TextColor(t.Text)
		if strings.TrimSpace(a.signErr) != "" {
			ui.Text(c, "签核失败："+a.signErr).FontSize(float32(tokens.TextData)).TextColor(t.Danger)
		}
		ui.Row(c).FillWidth().Wrap().AlignItems(ui.Center).Gap(t.Space(1.5)).Children(func() {
			if components.Button(c, "确认签核归档", components.ButtonSecondary).Clicked() {
				a.signAndArchiveRun()
			}
			if components.Button(c, "取消", components.ButtonGhost).Clicked() {
				a.signConfirm = false
				a.signErr = ""
			}
		})
	} else {
		ui.Row(c).FillWidth().Wrap().AlignItems(ui.Center).Gap(t.Space(1.5)).Children(func() {
			if components.Button(c, "签核归档", components.ButtonSecondary).Clicked() {
				a.signConfirm = true
				a.signErr = ""
			}
			ui.Text(c, "签核归档后实验记录将锁定不可修改；完整归档包可在「实验方案」页导出。").
				FontSize(float32(tokens.TextData)).TextColor(t.TextMuted)
		})
	}
}

// signAndArchiveRun ends the Run's record with one Core command, then drops
// the shared memo so the next frame reads the signed state. Signed and
// archived states are rejected by Core, never re-signed.
func (a *app) signAndArchiveRun() {
	a.signErr = ""
	if a.runSvc == nil || a.snap.RunID == "" {
		a.signErr = "运行服务不可用"
		return
	}
	view := ""
	if a.recordBlind() {
		view = "blind"
	}
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := a.runSvc.SignAndArchiveRun(context.Background(), runs.SignAndArchiveInput{
		RunID: a.snap.RunID, View: view, FOVPerWell: uint32(a.recordFOV()),
		NowUTC: now, SignedBy: recordActor(), SignedAtUTC: now,
	})
	if err != nil {
		a.signErr = state.MapError(err).Error()
		return
	}
	a.signConfirm = false
	a.invalidateRecord()
	a.setNoteMode(2)
}

// recordLockedNotice replaces every editing mode while the record is sealed.
// There is no unlock: sign/archive is final, and the notice says so.
func (a *app) recordLockedNotice(c *ui.Context, t *ui.Theme) {
	tokens := theme.GetTokens(t)
	ui.Column(c).Grow(1).FillWidth().AlignItems(ui.Center).Children(func() {
		ui.Column(c).Grow(1).FillWidth().MaxWidth(820).MaxWidthPercent(100).
			Gap(t.Space(2)).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "已签核归档").FontSize(float32(tokens.TextBody)).Bold().TextColor(t.Text)
			ui.Text(c, "记录已锁定为只读，不可撤销。切换到“阅读”查看正文，归档包在图板与实验方案页导出。").
				FontSize(float32(tokens.TextMeta)).TextColor(t.TextMuted)
		})
	})
}

// recordActor names the signing operator with the same session identity the
// app already uses for operator writes (os.Getenv("USERNAME"), as in the
// Core command paths in main.go), falling back to the asset-link convention
// "local-user" when the environment names no user. It invents no identity:
// an empty environment reads exactly as the local fallback.
func recordActor() string {
	if name := strings.TrimSpace(os.Getenv("USERNAME")); name != "" {
		return name
	}
	return "local-user"
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

// decodePreviewImage decodes board preview bytes through the pinned MyGo
// decoder. The TIFF import above is what makes .tif/.tiff assets decode
// instead of landing in the unreadable bucket; both load paths below share
// this one helper so they cannot drift apart.
func decodePreviewImage(data []byte) (ui.ImageSource, error) {
	return ui.DecodeImage(data)
}

// recordImageSource resolves a matrix image to a decoded preview, cached on
// the board view for the active Run and selected FOV layer. The file read
// and decode run off the view callback: with a window they move to a worker
// whose result lands through mygo.RunOnMain plus a shell invalidation (the
// startSaveBatch pattern in main.go); headless (tests) they run inline on
// the same thread, since no event loop drains the main queue there. While a
// load is in flight the board reads an explicit loading state. Anything
// unloadable (unconfigured root, missing file, undecodable bytes) returns
// nil and is remembered, so the board shows an explicit gap instead of
// retrying every frame.
func (a *app) recordImageSource(board *recordBoardView, img record.PlacedImage) ui.ImageSource {
	if src, ok := board.imgs[img.AssetID]; ok {
		return src
	}
	if board.imgFailed[img.AssetID] || board.imgLoading[img.AssetID] {
		return nil
	}
	fail := func() ui.ImageSource {
		board.imgFailed[img.AssetID] = true
		return nil
	}
	// The file resolves through the linked asset with the same ID, never
	// from the label or an inferred path.
	var linked *runs.Asset
	for i := range a.runAssets {
		if a.runAssets[i].ID == img.AssetID {
			linked = &a.runAssets[i]
			break
		}
	}
	if linked == nil || strings.TrimSpace(linked.RootID) == "" || strings.TrimSpace(linked.RelativePath) == "" {
		return fail()
	}
	path, err := assets.Roots(a.assetRoots).Resolve(assets.Asset{RootID: linked.RootID, RelativePath: linked.RelativePath})
	if err != nil {
		return fail()
	}
	if !a.windowReady {
		data, err := os.ReadFile(path)
		var src ui.ImageSource
		if err == nil {
			src, err = decodePreviewImage(data)
		}
		a.finishPreviewLoad(board, a.snap.RunID, board.gen, board.fov, img.AssetID, src, err)
		return board.imgs[img.AssetID]
	}
	board.imgLoading[img.AssetID] = true
	gen, runID, fov, assetID := board.gen, a.snap.RunID, board.fov, img.AssetID
	go func() {
		data, err := os.ReadFile(path)
		var src ui.ImageSource
		if err == nil {
			src, err = decodePreviewImage(data)
		}
		mygo.RunOnMain(func() {
			a.finishPreviewLoad(board, runID, gen, fov, assetID, src, err)
		})
	}()
	return nil
}

// finishPreviewLoad applies one worker's decoded preview on the view thread.
// Results for a Run or FOV layer the board has since left drop (releasing
// their pixels) instead of landing in the wrong cell; only the active layer
// inserts or records failure, then asks for a redraw.
func (a *app) finishPreviewLoad(board *recordBoardView, runID string, gen uint64, fov int, assetID string, src ui.ImageSource, err error) {
	delete(board.imgLoading, assetID)
	if err != nil || src == nil {
		if a.snap.RunID == runID && board.gen == gen && board.fov == fov {
			board.imgFailed[assetID] = true
		}
		if a.shell != nil {
			a.shell.Invalidate()
		}
		return
	}
	if a.snap.RunID != runID || board.gen != gen || board.fov != fov {
		if bmp, ok := src.(*ui.Bitmap); ok && bmp != nil {
			bmp.ReleasePixels()
		}
		return
	}
	board.imgs[assetID] = src
	if a.shell != nil {
		a.shell.Invalidate()
	}
}

// recordGroupBoardSection renders the representative-view matrix the design
// doc requires: assay group rows (ResultCell.Group only; wells without a
// recorded group read as an explicit 未分组 row) by timepoint columns, each
// slot showing the selected FOV's inline preview or an explicit gap. Group
// identity is never inferred from filenames or samples.
func (a *app) recordGroupBoardSection(c *ui.Context, t *ui.Theme, tokens theme.Tokens, proj record.Projection, board *recordBoardView) {
	recordBoardSection(c, t, fmt.Sprintf("代表视野（分组 × 时间点 · FOV %d）", board.fov))
	groups := record.MatrixGroups(proj)
	timepoints := record.MatrixTimepoints(proj)
	if len(groups) == 0 || len(timepoints) == 0 {
		ui.Text(c, "无分组矩阵数据。").FontSize(float32(tokens.TextMeta)).TextColor(t.TextMuted)
		return
	}
	ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(t.Space(1)).Children(func() {
		ui.Text(c, "分组 ＼ 时间点").FontSize(float32(tokens.TextData)).Bold().TextColor(t.TextMuted)
		for _, tp := range timepoints {
			tp := tp
			ui.Text(c, tp).FontSize(float32(tokens.TextData)).Bold().TextColor(t.TextMuted)
		}
	})
	for _, group := range groups {
		group := group
		label := record.GroupDisplayName(proj, group)
		ui.Row(c).FillWidth().Wrap().AlignItems(ui.Center).Gap(t.Space(1.5)).Children(func() {
			ui.Text(c, label).FontSize(float32(tokens.TextData)).Bold().TextColor(t.Text)
			for _, tp := range timepoints {
				tp := tp
				well, img, ok := record.RepresentativeGroup(proj, group, tp, board.fov)
				if !ok {
					ui.Text(c, "—").FontSize(float32(tokens.TextData)).TextColor(t.TextMuted)
					continue
				}
				ui.Column(c).Width(140).Gap(t.Space(.5)).Children(func() {
					if img != nil {
						if src := a.recordImageSource(board, *img); src != nil {
							ui.Image(c, src).Size(132, 96).Fit(ui.Contain)
						} else if board.imgLoading[img.AssetID] {
							ui.Text(c, "图加载中…").FontSize(float32(tokens.TextData)).TextColor(t.TextMuted)
						} else {
							ui.Text(c, "图不可读").FontSize(float32(tokens.TextData)).TextColor(t.Warning)
						}
					} else {
						ui.Text(c, "缺").FontSize(float32(tokens.TextData)).TextColor(t.TextMuted)
					}
					ui.Text(c, well).FontSize(float32(tokens.TextData)).TextColor(t.TextMuted)
				})
			}
		})
	}
}
