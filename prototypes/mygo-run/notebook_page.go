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
	"github.com/egoist/mygo/ui/markdown"
	"github.com/egoist/mygo/ui/richtext"
	"github.com/oneo-me/onevra/source/components"
	"github.com/oneo-me/onevra/source/icons/lucide"
	"gopkg.in/yaml.v3"
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
// is in front (prose paper or Run companion modules/timeline), and the entry
// a citation jumped to.
type noteViewState struct {
	split               int
	splitPct            float32
	rightTab            int // 0 = 执行时间线, 1 = 实测模块
	rightTabInitialized bool
	highlightID         string
	scrolledID          string
	expanded            map[string]bool
	hydratedView        *asterdoc.View
	hydratedKey         string
	readView            *asterdoc.View
	readKey             string
	lastSavedRunID      string
	lastSavedSrc        string
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

// recordProjection is the single shared read model for hydrated Run data.
func (a *app) recordProjection() (record.Projection, []record.TaskStatus, string) {
	_, _, _ = a.fetchRecord()
	if a.recProj != nil {
		return *a.recProj, a.recTasks, a.recErr
	}
	return record.Projection{}, a.recTasks, a.recErr
}

// recordSealed reports the persisted Core lifecycle: only signed/archived
// state locks editing.
func (a *app) recordSealed() bool {
	_, _, _ = a.recordProjection()
	return a.recordArchived()
}

func noteModeToTab(mode int) int {
	switch mode {
	case 0:
		return 0 // 编辑
	case 2:
		return 1 // 阅读
	case 1:
		return 2 // 源码
	default:
		return 1
	}
}

func noteTabToMode(tab int) int {
	switch tab {
	case 0:
		return 0 // 编辑
	case 1:
		return 2 // 阅读
	case 2:
		return 1 // 源码
	default:
		return 2
	}
}

// notebookPage offers three streamlined modes over stored Markdown + embedded AsterML blocks:
// 编辑 (0), 阅读 (2), and 源码 (1), unified right-aligned left of 导出 PDF.
func notebookPage(c *ui.Context, a *app) {
	t := c.Theme()
	tokens := theme.GetTokens(t)
	if a.snap.RunID == "" {
		ui.Column(c).Fill().Center().Gap(float32(tokens.Spacing8)).Children(func() {
			ui.Text(c, "实验记录").FontSize(float32(tokens.TextPage)).Bold().TextColor(t.Text)
			ui.Text(c, "先开始或进入一次实验运行，实测记录与实验笔记会自动关联到这里。").
				FontSize(float32(tokens.TextMeta)).TextColor(t.TextMuted)
		})
		return
	}
	if a.noteRunID != a.snap.RunID {
		a.loadNotebook()
	}
	if a.noteMode < 0 || a.noteMode > 2 {
		a.noteMode = 2
	}
	sealed := a.recordSealed()
	proj, _, _ := a.recordProjection()
	hydratedView := a.ensureHydratedNotebookAsterView(proj)
	readAsterView := a.ensureReadAsterView(hydratedView)
	defer func() {
		if hydratedView != nil {
			hydratedView.RenderPlateOverlay(c)
		}
		if readAsterView != nil {
			readAsterView.RenderPlateOverlay(c)
		}
	}()
	if c.Shortcut(ui.Cmd, ui.KeyE) && !sealed {
		if a.noteMode == 2 {
			a.setNoteMode(a.noteEditMode)
		} else {
			a.setNoteMode(2)
		}
	}
	ui.Column(c).Fill().Background(tokens.Canvas).Padding(float32(tokens.Spacing12), float32(tokens.Spacing16)).Gap(float32(tokens.Spacing8)).Children(func() {
		ui.Row(c).FillWidth().Wrap().AlignItems(ui.Center).Gap(float32(tokens.Spacing8)).Padding(0, float32(tokens.Spacing4)).Children(func() {
			ui.Text(c, a.snap.Title).FontSize(float32(tokens.TextSection)).Bold().TextColor(tokens.Text).
				SingleLine().Ellipsis("…").MaxWidth(float32(tokens.Spacing48) * 7)
			if stWord := notebookStatusWord(a.snap.Status); stWord != "" {
				components.Badge(c, stWord, components.BadgeOptions{Tone: components.BadgeAccent})
			}
			if a.snap.AsterDoc != nil || a.snap.DesignRevision > 0 {
				ui.Text(c, "方案快照已锁定").FontSize(float32(tokens.TextMeta)).TextColor(tokens.TextMuted)
			}
			stateLabel, stateTone := a.notebookToolbarBadgeTone()
			components.Badge(c, stateLabel, components.BadgeOptions{Tone: stateTone})
			ui.Spacer(c)

			oldMode := a.noteMode
			tabIdx := noteModeToTab(a.noteMode)
			parts := components.Tabs(c, &tabIdx, components.TabsSegmented, "编辑", "阅读", "源码")
			if parts.List.Changed() {
				nextMode := noteTabToMode(tabIdx)
				if sealed && nextMode != 2 {
					a.noteMode = oldMode
					c.AddToast(ui.Toast{Title: "记录已锁定", Description: "该实验记录为只读状态，仅可在阅读模式下查看。", Type: components.ToastWarning})
				} else {
					a.noteMode = nextMode
					a.applyNoteModeChange(oldMode)
				}
			}

			recPDF := components.Button(c, "导出 PDF", components.ButtonSecondary)
			recPDF.Disabled(a.pdfExporting || (a.snap.AsterDoc == nil && a.asterDoc == nil))
			if recPDF.Clicked() && !a.pdfExporting {
				a.startRunRecordPDFExport(c)
			}
			if components.Button(c, "导出 Markdown", components.ButtonGhost).Clicked() {
				fname := strings.TrimSuffix(pdfFilename(a.snap.Title), ".pdf") + "-记录.md"
				a.exportRecordFile(c, "导出实验笔记 Markdown", fname, a.noteSource())
			}
			components.Badge(c, fmt.Sprintf("%d / 5000 字", len([]rune(a.noteSource()))))
		})
		// Slim inline editor toolbar shown only when actively editing/writing source.
		if !sealed && a.noteMode != 2 {
			ui.Row(c).FillWidth().Wrap().
				Padding(float32(tokens.Spacing4), float32(tokens.Spacing12)).Radius(float32(tokens.ControlRadius)).
				Background(tokens.Surface).Border(1, tokens.Border).
				Gap(float32(tokens.Spacing8)).AlignItems(ui.Center).Children(func() {
				live := a.noteMode == 0
				doc := a.noteDoc
				if live && doc != nil {
					formats := doc.Formatting()
					a.iconCmd(c, lucide.Undo2(), "撤销", false, doc.CanUndo(), doc.Undo)
					a.iconCmd(c, lucide.Redo2(), "重做", false, doc.CanRedo(), doc.Redo)
					ui.Box(c).Width(1).Height(float32(tokens.Spacing16)).Background(tokens.Border)
					a.iconCmd(c, lucide.Bold(), "粗体", formats.Bold == richtext.FormatOn, true, doc.ToggleBold)
					a.iconCmd(c, lucide.Italic(), "斜体", formats.Italic == richtext.FormatOn, true, doc.ToggleItalic)
					a.iconCmd(c, lucide.Strikethrough(), "删除线", formats.Strikethrough == richtext.FormatOn, true, doc.ToggleStrikethrough)
					a.iconCmd(c, lucide.Code(), "行内代码", formats.Code == richtext.FormatOn, true, doc.ToggleCode)
					ui.Box(c).Width(1).Height(float32(tokens.Spacing16)).Background(tokens.Border)
					a.iconCmd(c, lucide.Heading1(), "一级标题", formats.Paragraph.Heading == 1, true, func() { doc.SetHeading(1) })
					a.iconCmd(c, lucide.Heading2(), "二级标题", formats.Paragraph.Heading == 2, true, func() { doc.SetHeading(2) })
					a.iconCmd(c, lucide.List(), "项目列表", formats.Paragraph.List == richtext.Bullets, true, func() { doc.ToggleList(richtext.Bullets) })
					a.iconCmd(c, lucide.ListOrdered(), "编号列表", formats.Paragraph.List == richtext.Numbers, true, func() { doc.ToggleList(richtext.Numbers) })
					a.iconCmd(c, lucide.Quote(), "引用", formats.Paragraph.Quote, true, doc.ToggleQuote)
					ui.Box(c).Width(1).Height(float32(tokens.Spacing16)).Background(tokens.Border)
				}
				if a.noteMode == 1 {
					modMenuBtn := components.Button(c, "＋ 插入模块", components.ButtonOutline)
					components.MenuButton(c, modMenuBtn, func(m *components.Menu) {
						items := []struct{ kind, label string }{
							{"prose", "文本 / 注意事项"},
							{"param_table", "实验参数表"},
							{"figure", "示意图 / SVG"},
							{"data_table", "数据记录表"},
							{"group_table", "实验分组表"},
							{"checklist", "执行检查清单"},
							{"calculator", "稀释 / 配液计算器"},
							{"timer", "步骤计时器"},
							{"attachment", "附件 / 显微采图"},
							{"subflow", "详细子步骤"},
							{"plate", "标准孔板"},
						}
						for _, it := range items {
							if m.Item(it.label).Chosen() {
								a.insertGenericModuleIntoDraft(it.kind)
							}
						}
					})
				}
				// Citations point to existing Run facts; no fact-writing controls live here (UX-P1, UX-P2, UX-P3).
				st := a.citePicker()
				cite := components.Button(c, "插入实验数据", components.ButtonGhost)
				if cite.Clicked() {
					st.manual = !st.open
					st.open = !st.open
					st.filter = "all"
					if !st.open {
						if q, active := notebook.ExtractAtQuery(a.noteSource()); active {
							st.dismissedToken = "@" + q
						}
					}
				}
				ui.Spacer(c)
				ui.Text(c, "Ctrl+E 切换阅读").FontSize(float32(tokens.TextMeta)).TextColor(tokens.TextMuted)
			})
		}
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
		view := a.noteView()
		if hydratedView != nil && hydratedView.Doc != nil && !view.rightTabInitialized {
			view.rightTab = 1
			view.rightTabInitialized = true
		}
		projection := notebook.Build(a.snap)
		width, _ := c.Size()
		if width >= 1100 {
			if view.splitPct <= 0 {
				view.splitPct = 62
			}
			ui.Box(c).Grow(1).MinHeight(0).FillWidth().Children(func() {
				components.SplitPercent(c, &view.splitPct,
					func() {
						ui.Column(c).Fill().Padding(0, float32(tokens.Spacing12), 0, 0).Children(func() {
							a.notebookProse(c, t)
						})
					},
					func() {
						ui.Column(c).Fill().Padding(0, 0, 0, float32(tokens.Spacing12)).Children(func() {
							notebookTimeline(c, a, t, projection, hydratedView)
						})
					},
					components.SplitOptions{FirstMin: 460, SecondMin: 320, Default: 62},
				).Label("实验记录与同源运行面板分隔条").Fill()
			})
			return
		}
		// Narrow (covers 900×600): switch between ELN sheet and Run companion.
		ui.Row(c).FillWidth().Children(func() {
			components.Tabs(c, &view.split, components.TabsSegmented, "笔记", "实测对照")
		})
		if view.split == 1 {
			ui.Row(c).Grow(1).FillWidth().Children(func() {
				ui.Column(c).Grow(1).MinWidth(0).Children(func() { notebookTimeline(c, a, t, projection, hydratedView) })
			})
			return
		}
		a.notebookProse(c, t)
	})
}

// notebookProse renders the Craft/Notion-styled ELN paper sheet over live rich rendering (0),
// Markdown + inline AsterML source editing (1), or homomorphic reading mode (2).
func (a *app) notebookProse(c *ui.Context, t *ui.Theme) {
	tok := theme.GetTokens(t)
	if a.recordSealed() && a.noteMode != 2 {
		a.recordLockedNotice(c, t)
		return
	}
	sealed := a.recordSealed()
	view := a.noteView()
	renderMasthead := func() {
		ui.Column(c).FillWidth().Gap(float32(tok.Spacing8)).Children(func() {
			ui.Row(c).FillWidth().Wrap().AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
				ui.Text(c, firstNonEmpty(a.snap.Title, "实验记录")).FontSize(float32(tok.TextSection)).Bold().TextColor(tok.Text)
				ui.Spacer(c)
				ui.Text(c, fmt.Sprintf("事件 %d · 图像 %d", len(a.snap.Events), len(a.runAssets))).
					FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted).FontFeatures("tnum")
			})
			if !sealed && strings.TrimSpace(a.noteSource()) == "" && (a.noteMode == 0 || a.noteMode == 1) {
				ui.Row(c).FillWidth().Wrap().Gap(float32(tok.Spacing4)).Children(func() {
					for _, tmpl := range []struct{ label, body string }{
						{"＋ 标准实验记录提纲", "# 实验记录与观察\n\n## 1. 实验条件与偏差核对\n- 参数确认：\n\n## 2. 关键现象与样本表现\n- \n\n## 3. 阶段结论与后续动作\n- \n"},
						{"＋ 异常偏差排查单", "# 异常偏差与排查记录\n\n- **发生步骤**：\n- **现象描述**：\n- **影响评估与处理**：\n"},
						{"＋ 定量结果研判摘要", "# 结果研判摘要\n\n- **主要指标变化**：\n- **质控样本核查**：\n- **下一步优化建议**：\n"},
					} {
						tmpl := tmpl
						if components.Button(c, tmpl.label, components.ButtonGhost).Clicked() {
							a.noteDraft = tmpl.body
							if a.noteMode == 0 {
								a.noteDoc = richtext.NewDocument(richtext.ParseMarkdown(a.noteDraft))
							}
							a.autoSaveNotebook()
						}
					}
				})
			}
			ui.Divider(c)
		})
	}
	paper := func(children func()) {
		ui.Column(c).Grow(1).FillWidth().AlignItems(ui.Center).Children(func() {
			ui.Box(c).Grow(1).FillWidth().MaxWidth(840).MaxWidthPercent(100).
				Background(tok.Surface).Border(1, tok.Border).
				Radius(float32(tok.PanelRadius)).Shadow(0, 2, 10, 0, tok.Text.Alpha(.04)).
				Padding(float32(tok.Spacing24), float32(tok.Spacing32)).Children(children)
		})
	}
	switch a.noteMode {
	case 2: // reading: homomorphic rendering for markdown + inline asterlab widgets
		ui.Scroll(c).Grow(1).Children(func() {
			ui.Column(c).FillWidth().Padding(0, float32(tok.Spacing8), float32(tok.Spacing16), float32(tok.Spacing8)).
				AlignItems(ui.Center).Children(func() {
				ui.Box(c).FillWidth().MaxWidth(840).MaxWidthPercent(100).
					Background(tok.Surface).Border(1, tok.Border).Radius(float32(tok.PanelRadius)).
					Shadow(0, 2, 10, 0, tok.Text.Alpha(.04)).
					Padding(float32(tok.Spacing24), float32(tok.Spacing32)).Children(func() {
					ui.Column(c).FillWidth().Gap(float32(tok.Spacing12)).Children(func() {
						renderMasthead()
						if strings.TrimSpace(a.noteDraft) == "" {
							stateWord := "未开始"
							if a.noteDirty {
								stateWord = "草稿"
							}
							ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
								components.Badge(c, stateWord, components.BadgeOptions{Tone: components.BadgeNeutral})
								ui.Text(c, "尚未记录笔记内容，切换到「编辑」或「源码」撰写实验记录。").
									FontSize(float32(tok.TextBody)).TextColor(tok.TextMuted)
							})
							return
						}
						readView := view.readView
						protocol.RenderMarkdownWith(c, a.noteDraft, func(c *ui.Context, block *markdown.CodeBlockView) {
							if block.Language == "asterlab" && readView != nil {
								src := "```asterlab\n" + block.Code() + "\n```"
								if parsed, err := asterdoc.Parse(src); err == nil && len(parsed.Modules) > 0 {
									readView.Doc = parsed
									for i := range parsed.Modules {
										m := parsed.Modules[i]
										readView.RenderModule(c, m.Kind, m.Schema, m.Data)
									}
									return
								}
							}
							markdown.DefaultCodeBlock(c, block)
						})
					})
				})
			})
		})
	case 1: // Markdown source
		paper(func() {
			renderMasthead()
			if !sealed {
				ui.Row(c).FillWidth().Wrap().Gap(float32(tok.Spacing4)).AlignItems(ui.Center).Children(func() {
					ui.Text(c, "Markdown 与内嵌组件源码").FontSize(float32(tok.TextMeta)).Bold().TextColor(tok.TextMuted)
					ui.Spacer(c)
					if components.Button(c, "＋ 小节标题", components.ButtonGhost).Clicked() {
						a.appendSnippetToDraft("\n## 阶段小节\n\n")
					}
					if components.Button(c, "＋ 嵌入参数表", components.ButtonGhost).Clicked() {
						a.insertGenericModuleIntoDraft("param_table")
					}
					if components.Button(c, "＋ 嵌入示意图", components.ButtonGhost).Clicked() {
						a.insertGenericModuleIntoDraft("figure")
					}
				})
			}
			ui.Box(c).FillWidth().Grow(1).MinHeight(0).Padding(float32(tok.Spacing12)).
				Radius(float32(tok.ControlRadius)).Background(tok.CodeBg).Border(1, tok.Border).Children(func() {
				area := ui.TextAreaBase(c, &a.noteDraft)
				area.Fill().Grow(1).MinWidth(0).MinHeight(0)
				if area.Changed() {
					a.autoSaveNotebook()
				}
			})
		})
	default: // live rich rendering
		paper(func() {
			renderMasthead()
			if a.noteDoc == nil {
				a.noteDoc = richtext.NewDocument(richtext.ParseMarkdown(a.noteDraft))
			}
			editor := richtext.EditorBase(c, a.noteDoc, richtext.EditorOptions{
				Label: "Notebook", Placeholder: "开始记录实验现象、关键读数与阶段结论…",
			})
			editor.Fill().Grow(1).MinWidth(0).MinHeight(0)
			if editor.Changed() {
				a.autoSaveNotebook()
			}
		})
	}
}

func (a *app) appendSnippetToDraft(snippet string) {
	if strings.TrimSpace(a.noteDraft) != "" {
		a.noteDraft = strings.TrimRight(a.noteDraft, "\n") + "\n" + strings.TrimLeft(snippet, "\n")
	} else {
		a.noteDraft = strings.TrimLeft(snippet, "\n")
	}
	if a.noteMode == 0 {
		a.noteDoc = richtext.NewDocument(richtext.ParseMarkdown(a.noteDraft))
	}
	a.autoSaveNotebook()
}

func (a *app) insertGenericModuleIntoDraft(kind string) {
	a.noteAster = nil
	existingDoc, _ := asterdoc.Parse(a.noteDraft)
	blk := asterdoc.DefaultStarterBlock(existingDoc, "note", kind)
	raw, err := yaml.Marshal(blk)
	if err != nil {
		return
	}
	fence := "\n```asterlab\n" + string(raw) + "```\n"
	a.appendSnippetToDraft(fence)
}

func (a *app) ensureHydratedNotebookAsterView(proj record.Projection) *asterdoc.View {
	if a.asterDoc == nil || strings.TrimSpace(a.asterDoc.Source) == "" {
		return nil
	}
	st := a.noteView()
	key := fmt.Sprintf("%s|%d|%d|%d|%d", a.snap.RunID, a.snap.Revision, len(a.runAssets), len(proj.Readouts), len(a.asterDoc.Source))
	if st.hydratedView != nil && st.hydratedKey == key {
		return st.hydratedView
	}
	vm := runworkspace.BuildViewModel(a.snap, &asterdoc.FrozenAsterDoc{
		DesignID: a.asterDoc.DesignID, Revision: a.asterDoc.Revision, Source: a.asterDoc.Source,
	})
	if vm == nil || vm.FrozenDoc == nil {
		return nil
	}
	resolve := func(asset runworkspace.LinkedAsset) string {
		if strings.TrimSpace(asset.RootID) == "" || strings.TrimSpace(asset.RelativePath) == "" {
			return ""
		}
		p, err := assets.Roots(a.assetRoots).Resolve(assets.Asset{
			RootID: asset.RootID, RelativePath: asset.RelativePath,
		})
		if err != nil {
			return ""
		}
		return p
	}
	v := runworkspace.HydrateReadOnlyAsterDocView(vm, record.DecodeReadouts(proj.Readouts), a.runAssetsForTask, resolve)
	if v != nil {
		v.AsyncDecode = a.windowReady
		v.OnAsyncInvalidate = func() {
			if a.shell != nil {
				a.shell.Invalidate()
			}
		}
	}
	st.hydratedView = v
	st.hydratedKey = key
	return v
}

func (a *app) ensureReadAsterView(hydrated *asterdoc.View) *asterdoc.View {
	st := a.noteView()
	key := a.snap.RunID + "|" + fmt.Sprint(a.snap.Revision) + "|" + fmt.Sprint(hydrated != nil)
	if st.readView == nil || st.readKey != key {
		rv := &asterdoc.View{
			Registry: asterdoc.BuiltinRegistry(),
			Mode:     asterdoc.ModeNotebook,
			Density:  asterdoc.DensityCompact,
			RunID:    a.snap.RunID,
			ReadOnly: true,
			AsyncDecode: a.windowReady,
			OnAsyncInvalidate: func() {
				if a.shell != nil {
					a.shell.Invalidate()
				}
			},
		}
		if hydrated != nil {
			rv.ActualParams = hydrated.ActualParams
			rv.CalcActuals = hydrated.CalcActuals
			rv.CheckedItems = hydrated.CheckedItems
			rv.SkippedItems = hydrated.SkippedItems
			rv.TableCells = hydrated.TableCells
			rv.SampleStatuses = hydrated.SampleStatuses
			rv.LinkedAssets = hydrated.LinkedAssets
		}
		st.readView = rv
		st.readKey = key
	}
	return st.readView
}

func (a *app) notebookToolbarBadgeTone() (string, components.BadgeTone) {
	if a.recordSealed() {
		return "记录已锁定", components.BadgeSuccess
	}
	switch {
	case a.noteState == "saving":
		return "保存中", components.BadgeNeutral
	case a.noteState == "error" || a.noteErr != "":
		return notebook.SaveStateLabel(true), components.BadgeDanger
	case a.noteDirty:
		return notebook.SaveStateLabel(true), components.BadgeWarning
	case a.noteState == "saved" || hasSavedNotebookEvent(a.snap.Events):
		return "实时已保存", components.BadgeSuccess
	default:
		return "实时保存", components.BadgeNeutral
	}
}

// notebookTimeline renders the Right Companion Panel of the Run ELN:
// Tab 0 shows carded timeline events with one-click citation;
// Tab 1 (when frozen AsterML is available) renders hydrated Design/Run v2 widgets side-by-side.
func notebookTimeline(c *ui.Context, a *app, t *ui.Theme, projection notebook.Notebook, hydratedView *asterdoc.View) {
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
	sealed := a.recordSealed()
	ui.Column(c).Fill().Padding(float32(tokens.Spacing12)).Radius(float32(tokens.PanelRadius)).
		Background(tokens.Surface).Border(1, tokens.Border).Gap(float32(tokens.Spacing8)).Children(func() {
		ui.Row(c).FillWidth().Wrap().AlignItems(ui.Center).Gap(float32(tokens.Spacing8)).Children(func() {
			ui.Text(c, "运行记录").FontSize(float32(tokens.TextSection)).FontWeight(600).TextColor(tokens.Text)
			components.Badge(c, "只读", components.BadgeOptions{Tone: components.BadgeNeutral})
			ui.Spacer(c)
			if hydratedView != nil && hydratedView.Doc != nil {
				tlStyle, modStyle := components.ButtonGhost, components.ButtonGhost
				if view.rightTab == 0 {
					tlStyle = components.ButtonSecondary
				} else {
					modStyle = components.ButtonSecondary
				}
				if components.Button(c, "实测模块", modStyle).Clicked() {
					view.rightTab = 1
					view.rightTabInitialized = true
				}
				if components.Button(c, fmt.Sprintf("执行时间线 (%d)", len(projection.Entries)), tlStyle).Clicked() {
					view.rightTab = 0
					view.rightTabInitialized = true
				}
			} else {
				ui.Text(c, fmt.Sprintf("共 %d 条记录", len(projection.Entries))).
					FontSize(float32(tokens.TextMeta)).TextColor(tokens.TextMuted).FontFeatures("tnum")
			}
		})
		ui.Divider(c)
		ui.Scroll(c).Grow(1).FillWidth().Children(func() {
			ui.Column(c).FillWidth().Padding(float32(tokens.Spacing4), 0).Gap(float32(tokens.Spacing8)).Children(func() {
				if view.rightTab == 1 && hydratedView != nil && hydratedView.Doc != nil {
					renderCompanionModules(c, a, tokens, hydratedView, sealed)
					return
				}
				if len(projection.Entries) == 0 {
					ui.Text(c, "此运行还没有可追溯的记录。").FontSize(float32(tokens.TextMeta)).TextColor(tokens.TextMuted)
					return
				}
				for _, entry := range projection.Entries {
					entry := entry
					highlighted := entry.ID == view.highlightID
					bg, borderCol := tokens.Elevated, tokens.Border
					if highlighted {
						bg, borderCol = tokens.WarningBg, tokens.Warning
					}
					row := ui.Column(c).FillWidth().Padding(float32(tokens.Spacing8), float32(tokens.Spacing12)).
						Radius(float32(tokens.ControlRadius)).Background(bg).Border(1, borderCol).
						Gap(float32(tokens.Spacing4))
					rawRef := notebookEntryRawLabel(entry)
					summary := notebookEntrySummary(entry, a.snap, taskTitles)
					row.Children(func() {
						ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(float32(tokens.Spacing8)).Children(func() {
							cat := iconForCategory(entry.Category)
							components.Icon(c, cat).Size(float32(tokens.Spacing16), float32(tokens.Spacing16)).TextColor(tokens.Accent)
							ui.Text(c, entry.Title).FontSize(float32(tokens.TextBody)).FontWeight(600).
								TextColor(tokens.Text).SingleLine().Ellipsis("…").Grow(1).MinWidth(0)
							ui.Text(c, entry.DisplayTime()).FontSize(float32(tokens.TextMeta)).
								TextColor(tokens.TextMuted).FontFeatures("tnum")
						})
						summaryText := ui.Text(c, summary).FontSize(float32(tokens.TextMeta)).
							TextColor(tokens.TextMuted)
						if rawRef != "" {
							summaryText.Tooltip(rawRef)
						}
						ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(float32(tokens.Spacing8)).Children(func() {
							components.Badge(c, notebookCategoryLabel(entry.Category))
							switch entry.Trace {
							case notebook.TraceInvalid:
								components.Badge(c, "来源失效", components.BadgeOptions{Tone: components.BadgeWarning})
							case notebook.TraceMissing:
								components.Badge(c, "来源未记录", components.BadgeOptions{Tone: components.BadgeWarning})
							}
							ui.Spacer(c)
							if !sealed && a.noteMode != 2 {
								if components.Button(c, "引用此条", components.ButtonGhost).Clicked() {
									quote := fmt.Sprintf("\n> [%s] %s：%s\n", entry.DisplayTime(), entry.Title, summary)
									a.appendSnippetToDraft(quote)
								}
							}
						})
						open := view.expanded[entry.ID]
						components.Collapsible(c, "来源详情", &open, components.CollapsibleOptions{}, func() {
							if rawRef != "" {
								ui.Text(c, "原始来源："+rawRef).FontSize(float32(tokens.TextData)).TextColor(tokens.TextMuted)
							}
							if entry.TaskID != "" {
								ui.Text(c, "步骤关联："+firstNonEmpty(taskTitles[entry.TaskID], "执行步骤")).FontSize(float32(tokens.TextData)).TextColor(tokens.TextMuted)
							}
							for _, detail := range entry.Details {
								ui.Text(c, detail.Label+"："+detail.Value).FontSize(float32(tokens.TextData)).TextColor(tokens.TextMuted)
							}
						})
						view.expanded[entry.ID] = open
					})
					rowElement := row
					if highlighted && view.scrolledID != entry.ID {
						rowElement.ScrollIntoView(ui.ScrollIntoViewOptions{Block: ui.ScrollCenter})
						view.scrolledID = entry.ID
					}
				}
			})
		})
	})
}

func renderCompanionModules(c *ui.Context, a *app, tokens theme.Tokens, v *asterdoc.View, sealed bool) {
	if v == nil || v.Doc == nil {
		return
	}
	rendered := 0
	for i := range v.Doc.Modules {
		m := &v.Doc.Modules[i]
		if m.Kind == "plate" || m.Kind == "group_table" {
			rendered++
			m := m
			ui.Column(c).FillWidth().Gap(float32(tokens.Spacing4)).Children(func() {
				v.RenderModule(c, m.Kind, m.Schema, m.Data)
				if !sealed && a.noteMode != 2 {
					ui.Row(c).FillWidth().Justify(ui.End).Children(func() {
						if components.Button(c, "嵌入此模块到笔记", components.ButtonGhost).Clicked() {
							a.appendSnippetToDraft("\n```asterlab\n" + strings.TrimSpace(m.Body) + "\n```\n")
						}
					})
				}
			})
		}
	}
	for i := range v.Doc.Modules {
		m := &v.Doc.Modules[i]
		if m.Kind != "step" {
			continue
		}
		rendered++
		ui.Column(c).FillWidth().Padding(float32(tokens.Spacing8)).Radius(float32(tokens.ControlRadius)).
			Background(tokens.Elevated).Border(1, tokens.Border).Gap(float32(tokens.Spacing8)).Children(func() {
			ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(float32(tokens.Spacing8)).Children(func() {
				ui.Text(c, firstNonEmpty(moduleStringFromMap(m.Data, "title"), "实验步骤")).
					FontSize(float32(tokens.TextBody)).Bold().TextColor(tokens.Text).Grow(1).MinWidth(0)
				if !sealed && a.noteMode != 2 {
					if components.Button(c, "引用本步模块", components.ButtonGhost).Clicked() {
						if rawBlocks, ok := m.Data["blocks"].([]any); ok {
							var b strings.Builder
							for _, rb := range rawBlocks {
								if raw, err := yaml.Marshal(rb); err == nil {
									b.WriteString("\n```asterlab\n" + string(raw) + "```\n")
								}
							}
							if b.Len() > 0 {
								a.appendSnippetToDraft(b.String())
							}
						}
					}
				}
			})
			v.HideStepHeader = true
			asterdoc.StepBlockComposer(v, c, m)
		})
	}
	if rendered == 0 {
		ui.Text(c, "当前方案暂无可预览的通用模块。").FontSize(float32(tokens.TextMeta)).TextColor(tokens.TextMuted)
	}
}

func moduleStringFromMap(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	s, _ := m[key].(string)
	return strings.TrimSpace(s)
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
	if a.noteMode < 0 || a.noteMode > 2 {
		a.noteMode = 2
	}
	if a.noteMode == 0 {
		a.noteDoc = richtext.NewDocument(richtext.ParseMarkdown(a.noteDraft))
	}
	if a.noteMode != 2 {
		a.noteEditMode = a.noteMode
	}
	if a.noteDirty {
		a.persistNotebookDraft()
	}
}

func (a *app) setNoteMode(mode int) {
	old := a.noteMode
	if mode < 0 || mode > 2 {
		mode = 2
	}
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
	if a.noteDirty {
		a.persistNotebookDraft()
	}
}

// iconCmd renders one compact toolbar command. Formatting commands act on the
// live rich document and trigger real-time persistence.
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
		a.autoSaveNotebook()
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
	a.autoSaveNotebook()
}

func (a *app) removeCitation(index int) {
	a.noteDraft = notebook.RemoveCitation(a.noteSource(), index)
	if a.noteMode == 0 {
		a.noteDoc = richtext.NewDocument(richtext.ParseMarkdown(a.noteDraft))
	}
	a.autoSaveNotebook()
}

// jumpToTimeline points the surface at the entry a citation named. At narrow
// widths it brings the timeline projection forward, because the two sections
// switch there; it never changes the prose.
func (a *app) jumpToTimeline(entryID string) {
	view := a.noteView()
	view.highlightID = entryID
	view.scrolledID = ""
	view.split = 1
	view.rightTab = 0
	view.rightTabInitialized = true
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
	if recovered != nil && (recovered.Markdown != latestContent || (!hasSaved && recovered.Markdown != "")) {
		a.noteDraft = recovered.Markdown
		a.noteDirty = true
		a.noteRecovered = true
		if recovered.Mode == 0 || recovered.Mode == 1 {
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
	st := a.noteView()
	st.lastSavedRunID = a.snap.RunID
	st.lastSavedSrc = latestContent
	st.rightTabInitialized = false
}

func (a *app) autoSaveNotebook() {
	a.markNotebookDirty()
	raw := a.noteSource()
	if a.runSvc == nil || a.snap.RunID == "" || strings.TrimSpace(raw) == "" ||
		a.recordArchived() || a.noteState == "saving" || len([]rune(raw)) > 5000 {
		return
	}
	st := a.noteView()
	if st.lastSavedRunID == a.snap.RunID && st.lastSavedSrc == raw {
		return
	}
	a.saveNotebook()
}

func (a *app) saveNotebook() {
	source := a.noteSource()
	a.noteDraft = source
	// Archived guard: Core locked the notes at sign/archive, so no save
	// may land (UX-P5 surfaces the refusal instead of dropping prose).
	if a.recordArchived() {
		a.noteErr = "记录已锁定，笔记只读"
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
	st := a.noteView()
	st.lastSavedRunID = a.snap.RunID
	st.lastSavedSrc = source
	a.clearNotebookDraft(a.snap.RunID)
	a.syncNotebookEnvelopeMirror(source)
	a.applySnap(snap)
}

// saveNotebook continues here...

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
	a.syncNotebookEnvelopeMirror(src)
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
			parts = append(parts, "方案快照已锁定")
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

// recordLockedNotice replaces every editing mode while the record is sealed.
func (a *app) recordLockedNotice(c *ui.Context, t *ui.Theme) {
	tokens := theme.GetTokens(t)
	ui.Column(c).Grow(1).FillWidth().AlignItems(ui.Center).Children(func() {
		ui.Column(c).Grow(1).FillWidth().MaxWidth(820).MaxWidthPercent(100).
			Gap(t.Space(2)).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "记录已锁定").FontSize(float32(tokens.TextBody)).Bold().TextColor(t.Text)
			ui.Text(c, "该实验记录为只读状态，可在「阅读模式」下浏览并导出实验记录 PDF。").
				FontSize(float32(tokens.TextMeta)).TextColor(t.TextMuted)
		})
	})
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
