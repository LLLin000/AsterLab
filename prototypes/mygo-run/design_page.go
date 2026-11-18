package main

// Design surface: 3-Tab AsterML v2 Scientific Design Workbench (`步骤工作台`,
// `实验全局设定`, `AsterML 源码`) backed by Core immutable revisions and
// reusable generic content modules.

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"asterlab-mygo-prototype/internal/agentedit"
	"asterlab-mygo-prototype/internal/asterdoc"
	"asterlab-mygo-prototype/internal/designer"
	"asterlab-mygo-prototype/internal/state"
	"asterlab-mygo-prototype/internal/theme"
	"asterlab-mygo-prototype/modules/runs"

	mygo "github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
	"github.com/oneo-me/onevra/source/components"
	"github.com/oneo-me/onevra/source/icons/lucide"
)

var agentEditStates = map[*app]*agentedit.State{}

func proposalState(a *app) *agentedit.State {
	if s := agentEditStates[a]; s != nil {
		return s
	}
	s := &agentedit.State{
		Client:      agentedit.CLIClient{},
		ConfirmedBy: "researcher",
		OnCommitted: func(agentedit.CommitResult) {
			if a.designDoc != nil {
				a.loadDesign(a.designDoc.DesignID)
			}
		},
	}
	agentEditStates[a] = s
	return s
}

// Agent proposal entry stays secondary to Create Run — UX-P1/P2/P3/P5 and UX-LAYOUT.
func renderAgentProposal(c *ui.Context, a *app) {
	t := c.Theme()
	tok := theme.GetTokens(t)
	s := proposalState(a)
	if components.Button(c, "Agent 提案", components.ButtonSecondary).Clicked() {
		s.Open = true
		s.Closed = false
	}
	if s.Open {
		components.Dialog(c, &s.Open, components.DialogOptions{Label: "Agent 提案"}, func(panel ui.Element) {
			panel.FillWidth().MaxWidth(float32(tok.Spacing48)*15).MaxWidthPercent(94).MaxHeightPercent(90).Children(func() {
				agentedit.View(c, s)
			})
		})
		if !s.Open {
			s.Closed = true
		}
	}
}

func renderDesignErrorBanner(c *ui.Context, message string, onRetry func()) {
	if message == "" {
		return
	}
	t := c.Theme()
	tokens := theme.GetTokens(t)
	ui.Row(c).FillWidth().Padding(float32(tokens.Spacing8), float32(tokens.Spacing12)).
		Background(tokens.DangerBg).Border(1, t.Danger.Alpha(.25)).
		AlignItems(ui.Center).Gap(float32(tokens.Spacing8)).Children(func() {
		ui.Box(c).Grow(1).MinWidth(0).Children(func() {
			ui.Text(c, message).FontSize(float32(tokens.TextMeta)).TextColor(tokens.DangerText)
		})
		if onRetry != nil {
			if components.Button(c, "重试", components.ButtonSecondary).Clicked() {
				onRetry()
			}
		}
	})
}

func (a *app) retryDesignFunc() func() {
	id := a.retryDesignID()
	if id == "" {
		return nil
	}
	return func() { a.loadDesign(id) }
}

func (a *app) designDocRetryFunc() func() {
	if a.designDocLoadFailed {
		return a.retryDesignFunc()
	}
	if a.retryDesignDoc != nil {
		return a.retryDesignDoc
	}
	return a.retryDesignFunc()
}

func designReadinessState(a *app) (string, components.BadgeTone) {
	if a.readinessLoading {
		return "校验中", components.BadgeNeutral
	}
	if a.readinessErr != "" {
		return "未解析", components.BadgeWarning
	}
	if a.readiness == nil {
		return "草稿", components.BadgeNeutral
	}
	switch a.readiness.State {
	case "loading", "checking":
		return "校验中", components.BadgeNeutral
	case "unresolved":
		return "未解析", components.BadgeWarning
	case "incomplete":
		return "不完整", components.BadgeDanger
	case "warning":
		return "待确认", components.BadgeWarning
	case "ready":
		return "就绪", components.BadgeSuccess
	default:
		return "草稿", components.BadgeNeutral
	}
}

func designExperimentTitle(doc *asterdoc.Document) string {
	if doc != nil {
		for _, module := range doc.Modules {
			if module.Kind == "experiment" {
				if title := moduleString(module.Data, "title"); title != "" {
					return title
				}
			}
		}
	}
	return "未命名实验方案"
}

// designProvenanceLine names the source Run and design revision when this
// design revision was persisted from a DeriveDesignFromRun draft.
func designProvenanceLine(a *app) string {
	if a == nil || a.designSnap == nil || strings.TrimSpace(a.designSnap.Data) == "" {
		return ""
	}
	var doc struct {
		SourceEvidence []struct {
			ID             string `json:"id"`
			SourceDocument string `json:"source_document"`
		} `json:"source_evidence"`
	}
	if err := json.Unmarshal([]byte(a.designSnap.Data), &doc); err != nil {
		return ""
	}
	for i := len(doc.SourceEvidence) - 1; i >= 0; i-- {
		entry := doc.SourceEvidence[i]
		if strings.HasPrefix(entry.ID, "derived-run-") && strings.TrimSpace(entry.SourceDocument) != "" {
			return "派生自" + entry.SourceDocument
		}
	}
	return ""
}

func designPage(c *ui.Context, a *app) {
	if a.designDoc != nil {
		asterDesignPage(c, a)
		return
	}
	retryLoad := a.retryDesignFunc()
	if a.designer != nil {
		ui.Column(c).Fill().Children(func() {
			if a.designLoadErr != "" {
				renderDesignErrorBanner(c, "设计加载失败："+a.designLoadErr, retryLoad)
			}
			if a.designDocErr != "" {
				renderDesignErrorBanner(c, "实验方案无法读取："+a.designDocErr, retryLoad)
			}
			ui.Box(c).FillWidth().Grow(1).MinHeight(0).Children(func() {
				if a.designer.CanvasMode {
					designer.View(c, a.designer)
				} else {
					designer.ListView(c, a.designer)
				}
			})
			renderAgentProposal(c, a)
		})
		return
	}
	t := c.Theme()
	tok := theme.GetTokens(t)
	if a.designLoadErr != "" || a.designDocErr != "" {
		title := "设计无法读取"
		msg := a.designLoadErr
		if msg == "" {
			title = "实验方案无法读取"
			msg = a.designDocErr
		}
		ui.Column(c).Fill().Center().Gap(float32(tok.Spacing8)).Padding(float32(tok.Spacing16)).Children(func() {
			ui.Text(c, title).FontSize(float32(tok.TextSection)).Bold().TextColor(t.Danger)
			ui.Text(c, msg).FontSize(float32(tok.TextBody)).TextColor(t.TextMuted)
			ui.Row(c).Gap(float32(tok.Spacing8)).AlignItems(ui.Center).Children(func() {
				if retryID := a.retryDesignID(); retryID != "" {
					if components.Button(c, "重试", components.ButtonPrimary).Clicked() {
						a.loadDesign(retryID)
					}
				}
				renderAgentProposal(c, a)
			})
		})
		return
	}
	message := "请从项目页选择一个已保存的设计，或点击左上角新建设计。"
	if a.designSvc == nil {
		message = "设计服务不可用。"
	}
	ui.Column(c).Fill().Center().Gap(float32(tok.Spacing8)).Children(func() {
		components.Text(c, "还没有打开的设计").FontSize(float32(tok.TextPage)).FontWeight(600)
		components.Text(c, message).FontSize(float32(tok.TextBody)).TextColor(t.TextMuted)
		renderAgentProposal(c, a)
	})
}

func (a *app) ensureAsterSrcEditor(source string, rev uint64) *editor.State {
	if a.asterSrcEditor == nil || a.asterSrcLoadedRev != rev || a.asterSrcLoadedSrc != source {
		buf := editor.NewBuffer(source)
		if lang := editor.LanguageFor("design.md"); lang != nil {
			buf.SetLanguage(lang)
		}
		a.asterSrcEditor = &editor.State{
			Buffer:   buf,
			SoftWrap: true,
			DiffBase: editor.NewBuffer(source),
			Label:    "AsterML 源码编辑器",
		}
		a.asterSrcLoadedRev = rev
		a.asterSrcLoadedSrc = source
	}
	return a.asterSrcEditor
}

func (a *app) ensureDesignView(doc *asterdoc.Document, stepID string) *asterdoc.View {
	rev := ""
	if a.designDoc != nil {
		rev = fmt.Sprint(a.designDoc.Revision)
	}
	if a.designView == nil || a.designView.Doc == nil || a.designView.Doc.Source != a.designDoc.Source {
		a.designView = &asterdoc.View{
			Doc:              doc,
			Mode:             asterdoc.ModeDesign,
			DocumentRevision: rev,
			Allowed:          map[string]bool{asterdoc.PermDocumentEdit: true},
			OnAssignWells: func(moduleID, group string, wells []string) {
				a.applyDesignCommand("assign_wells", moduleID, group, wells)
			},
			OnClearWells: func(moduleID string, wells []string) {
				a.applyDesignCommand("clear_wells", moduleID, "", wells)
			},
			OnUpdateModule: func(moduleID string, props map[string]any) {
				_ = a.applyDesignRaw(map[string]any{
					"type": "update_module", "moduleId": moduleID, "properties": props,
				})
			},
			OnInsertStepBlock: func(sID string, idx int, block map[string]any) {
				_ = a.applyDesignRaw(map[string]any{
					"type": "insert_step_block", "stepId": sID, "index": idx, "block": block,
				})
			},
			OnUpdateStepBlock: func(sID, blockID string, props map[string]any) {
				_ = a.applyDesignRaw(map[string]any{
					"type": "update_step_block", "stepId": sID, "blockId": blockID, "properties": props,
				})
			},
			OnMoveStepBlock: func(sID, blockID, direction string) {
				_ = a.applyDesignRaw(map[string]any{
					"type": "move_step_block", "stepId": sID, "blockId": blockID, "direction": direction,
				})
			},
			OnMoveStepBlockIndex: func(sID, blockID string, toIndex int) {
				_ = a.applyDesignRaw(map[string]any{
					"type": "move_step_block", "stepId": sID, "blockId": blockID, "toIndex": toIndex,
				})
			},
			OnRemoveStepBlock: func(sID, blockID string) {
				_ = a.applyDesignRaw(map[string]any{
					"type": "remove_step_block", "stepId": sID, "blockId": blockID,
				})
			},
			OnAsyncInvalidate: func() {
				if a.shell != nil {
					a.shell.Invalidate()
				}
			},
			OnPickFigureFile: func(sID, blockID string) {
				paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
					Parent: mygo.FocusedWindow(),
					Title:  "选择示意图文件（SVG / PNG / JPG）",
				})
				if err != nil || len(paths) == 0 {
					return
				}
				picked := paths[0]
				props := map[string]any{"src": picked}
				if strings.EqualFold(filepath.Ext(picked), ".svg") {
					if rawBytes, readErr := os.ReadFile(picked); readErr == nil && strings.Contains(string(rawBytes), "<svg") {
						props["svg"] = string(rawBytes)
					}
				}
				_ = a.applyDesignRaw(map[string]any{
					"type": "update_step_block", "stepId": sID, "blockId": blockID, "properties": props,
				})
			},
		}
	}
	a.designView.StepID = stepID
	a.designView.AsyncDecode = a.windowReady
	return a.designView
}

// asterDesignPage renders the 3-Tab AsterML v2 Scientific Design Workbench.
func asterDesignPage(c *ui.Context, a *app) {
	t := c.Theme()
	tok := theme.GetTokens(t)
	c.OnShortcut(ui.Cmd, ui.KeyZ, func() { a.designUndo() })
	c.OnShortcut(ui.Cmd|ui.Shift, ui.KeyZ, func() { a.designRedo() })
	c.OnShortcut(ui.Cmd, ui.KeyY, func() { a.designRedo() })

	doc, err := asterdoc.Parse(a.designDoc.Source)
	if err != nil {
		ui.Column(c).Fill().Padding(float32(tok.Spacing24)).Gap(float32(tok.Spacing12)).Children(func() {
			renderDesignErrorBanner(c, "AsterML 语法错误："+err.Error(), a.designDocRetryFunc())
			if components.Button(c, "切换到源码修复", components.ButtonSecondary).Clicked() {
				a.designTab = 2
			}
		})
		return
	}

	srcEd := a.ensureAsterSrcEditor(a.designDoc.Source, a.designDoc.Revision)
	srcText := srcEd.Buffer.String()
	srcDirty := strings.ReplaceAll(srcText, "\r\n", "\n") != strings.ReplaceAll(a.designDoc.Source, "\r\n", "\n")
	stepDirty := a.hasUnsavedStepDrafts(doc)
	anyDirty := srcDirty || stepDirty

	if a.designTab < 0 || a.designTab > 2 {
		a.designTab = 0
	}

	ui.Column(c).Fill().Children(func() {
		// Workbench Header
		ui.Row(c).FillWidth().Wrap().Padding(float32(tok.Spacing8), float32(tok.Spacing16)).
			Background(tok.Surface).
			AlignItems(ui.Center).Gap(float32(tok.Spacing12)).Children(func() {
			ui.Row(c).AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
				ui.Text(c, designExperimentTitle(doc)).FontSize(float32(tok.TextSection)).Bold().TextColor(tok.Text)
				ui.Text(c, fmt.Sprintf("rev %d", a.designDoc.Revision)).FontSize(float32(tok.TextMeta)).
					TextColor(tok.TextMuted).FontFeatures("tnum")
				stateLabel, stateTone := designReadinessState(a)
				components.Badge(c, stateLabel, components.BadgeOptions{Tone: stateTone})
				if anyDirty {
					components.Badge(c, "未保存", components.BadgeOptions{Tone: components.BadgeWarning})
				}
			})

			tab := a.designTab
			components.Tabs(c, &tab, components.TabsSegmented, "步骤工作台", "实验全局设定", "AsterML 源码")
			a.designTab = tab

			ui.Spacer(c)

			undo := components.IconButton(c, lucide.Undo2(), "撤销", components.IconButtonCompact)
			components.Tooltip(c, undo, "撤销上一步修改 (Ctrl+Z)")
			undo.Disabled(len(a.designHistory) == 0 && len(a.asterDraftUndo) == 0)
			if undo.Clicked() {
				a.designUndo()
			}

			redo := components.IconButton(c, lucide.Redo2(), "重做", components.IconButtonCompact)
			components.Tooltip(c, redo, "重做 (Ctrl+Y)")
			redo.Disabled(len(a.designFuture) == 0 && len(a.asterDraftRedo) == 0)
			if redo.Clicked() {
				a.designRedo()
			}

			renderAgentProposal(c, a)

			saveBtn := components.Button(c, "保存修订", components.ButtonSecondary)
			saveBtn.Disabled(!srcDirty)
			if saveBtn.Clicked() && srcDirty {
				a.saveAsterDocSource(srcText)
			}

			stateLabel, _ := designReadinessState(a)
			createBtn := components.Button(c, "创建 Run", components.ButtonPrimary)
			createBtn.Disabled(stateLabel == "校验中")
			if createBtn.Clicked() && stateLabel != "校验中" {
				a.preflightRisk = false
				a.preflightOpen = true
			}
		})
		ui.Divider(c)

		// Scientific BudgetBar computed directly from AsterML v2 document
		renderDesignBudgetBar(c, a, doc)

		if a.designLoadErr != "" {
			renderDesignErrorBanner(c, "设计加载失败："+a.designLoadErr, a.retryDesignFunc())
		}
		if a.designDocErr != "" {
			renderDesignErrorBanner(c, a.designDocErr, a.designDocRetryFunc())
		}
		if a.readinessErr != "" {
			renderDesignErrorBanner(c, "实验方案核查未完成："+a.readinessErr, func() { a.refreshReadiness() })
		}
		if a.createRunErr != "" {
			renderDesignErrorBanner(c, "创建 Run 失败："+a.createRunErr, func() { a.createRunFromDesignDoc() })
		}

		ui.Box(c).FillWidth().Grow(1).MinHeight(0).Children(func() {
			switch a.designTab {
			case 0:
				renderStepWorkbenchTab(c, a, doc)
			case 1:
				renderGlobalSetupTab(c, a, doc)
			case 2:
				renderAsterMLSourceTab(c, a, doc, srcEd)
			}
		})
	})

	a.renderPreflight(c)
	a.captureAsterDraft()
}

func renderDesignBudgetBar(c *ui.Context, a *app, doc *asterdoc.Document) {
	tok := theme.GetTokens(c.Theme())
	b := asterdoc.ComputeBudgetSummary(doc)
	handsOn := formatDurationMinutes(b.HandsOnSec)
	span := formatDurationMinutes(b.TotalSpanSec)

	ui.Row(c).FillWidth().Wrap().Padding(float32(tok.Spacing8), float32(tok.Spacing16)).
		Background(tok.Elevated).
		AlignItems(ui.Center).Gap(float32(tok.Spacing12)).Children(func() {
		budgetMetric(c, "步骤", fmt.Sprintf("%d 步 (%d 可执行)", b.Steps, b.ExecutableSteps))
		budgetMetric(c, "分组", fmt.Sprintf("%d 组", b.Groups))
		budgetMetric(c, "容器位", fmt.Sprintf("%d 孔/皿", b.Wells))
		budgetMetric(c, "预计操作", handsOn)
		budgetMetric(c, "时间跨度", span)
		if b.HasOpenPhase {
			components.Badge(c, "含开放等待段")
		}
		if line := designProvenanceLine(a); line != "" {
			ui.Spacer(c)
			ui.Text(c, line).FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
		}
	})
	ui.Divider(c)
}

func budgetMetric(c *ui.Context, label, value string) {
	tok := theme.GetTokens(c.Theme())
	ui.Row(c).AlignItems(ui.Center).Gap(float32(tok.Spacing4)).Children(func() {
		ui.Text(c, label+"：").FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
		ui.Text(c, value).FontSize(float32(tok.TextMeta)).Bold().TextColor(tok.Text).FontFeatures("tnum")
	})
}

func formatDurationMinutes(sec int) string {
	if sec <= 0 {
		return "—"
	}
	if sec < 60 {
		return fmt.Sprintf("%ds", sec)
	}
	mins := sec / 60
	if mins < 60 {
		return fmt.Sprintf("%dmin", mins)
	}
	hrs := float64(sec) / 3600.0
	return fmt.Sprintf("%.1fh", hrs)
}

func renderStepWorkbenchTab(c *ui.Context, a *app, doc *asterdoc.Document) {
	order, steps := orderedDesignSteps(doc)
	if len(order) > 0 && steps[a.designSelected] == nil {
		a.selectAsterStep(nil, steps[order[0]])
	}

	stepPaths := map[string]bool{}
	var wf *asterdoc.Module
	for i := range doc.Modules {
		if doc.Modules[i].Kind == "workflow" && wf == nil {
			wf = &doc.Modules[i]
		}
	}
	for _, s := range steps {
		if p := moduleString(s.Data, "path"); p != "" {
			stepPaths[p] = true
		}
	}

	railFn := func() {
		renderStepOutlineRail(c, a, doc, wf, order, steps, stepPaths)
	}
	detailFn := func() {
		renderStepDetailCanvas(c, a, doc, steps[a.designSelected])
	}

	if designNarrow(c) {
		components.ScrollView(c, components.ScrollViewOptions{}).Fill().Children(func() {
			ui.Column(c).FillWidth().Children(func() {
				railFn()
				detailFn()
			})
		})
		return
	}

	if a.designSplitW <= 0 {
		a.designSplitW = 280
	}
	components.Split(c, &a.designSplitW, railFn, detailFn, components.SplitOptions{
		FirstMin:  220,
		SecondMin: 420,
		Default:   280,
	}).Label("步骤大纲与设计画布分隔条").Fill()
}

func orderedDesignSteps(doc *asterdoc.Document) ([]string, map[string]*asterdoc.Module) {
	var wf *asterdoc.Module
	steps := map[string]*asterdoc.Module{}
	docOrder := []string{}
	for i := range doc.Modules {
		m := &doc.Modules[i]
		if m.Kind == "workflow" && wf == nil {
			wf = m
		}
		if m.Kind == "step" {
			steps[m.ID] = m
			docOrder = append(docOrder, m.ID)
		}
	}
	order := []string{}
	seen := map[string]bool{}
	if wf != nil {
		key := "business_order"
		if _, ok := wf.Data[key]; !ok {
			key = "steps"
		}
		for _, id := range moduleStrings(wf.Data, key) {
			if steps[id] != nil && !seen[id] {
				order = append(order, id)
				seen[id] = true
			}
		}
	}
	for _, id := range docOrder {
		if !seen[id] {
			order = append(order, id)
		}
	}
	return order, steps
}

func renderStepOutlineRail(c *ui.Context, a *app, doc *asterdoc.Document, wf *asterdoc.Module, order []string, steps map[string]*asterdoc.Module, stepPaths map[string]bool) {
	tok := theme.GetTokens(c.Theme())
	ui.Column(c).Fill().Background(tok.Surface).Children(func() {
		// Rail header + Add Step
		ui.Row(c).FillWidth().Padding(float32(tok.Spacing12)).
			AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
			ui.Text(c, "步骤大纲").FontSize(float32(tok.TextSection)).Bold().TextColor(tok.Text)
			components.Badge(c, fmt.Sprintf("%d 步", len(order)))
			ui.Spacer(c)
			if components.Button(c, "+ 新建步骤", components.ButtonSecondary).Clicked() {
				nextID := fmt.Sprintf("step-%02d", len(order)+1)
				cmd := map[string]any{
					"type":  "add_step",
					"title": fmt.Sprintf("%02d 新实验步骤", len(order)+1),
					"id":    nextID,
				}
				if assayID := defaultFirstAssayID(doc); assayID != "" {
					cmd["assay"] = assayID
				}
				if a.applyDesignRaw(cmd) {
					a.designSelected = nextID
				}
			}
		})
		ui.Divider(c)

		components.ScrollView(c, components.ScrollViewOptions{}).FillWidth().Grow(1).MinHeight(0).Children(func() {
			ui.Column(c).FillWidth().Padding(float32(tok.Spacing8)).Gap(float32(tok.Spacing8)).Children(func() {
				if unsaved := a.otherUnsavedStepIDs(order, steps); len(unsaved) > 0 {
					renderUnsavedDraftWarning(c, a, steps, unsaved)
				}
				if len(order) == 0 {
					ui.Text(c, "暂无步骤，点击右上角「+ 新建步骤」开始设计。").
						FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
					return
				}
				for i, id := range order {
					i, id := i, id
					step := steps[id]
					if step == nil {
						continue
					}
					selected := a.designSelected == id
					title := firstNonEmpty(moduleString(step.Data, "title"), "未命名步骤")
					path := moduleString(step.Data, "path")
					blocks, _ := step.Data["blocks"].([]any)
					bg := tok.Elevated
					border := tok.Border
					if selected {
						bg = tok.AccentSoftBg
						border = tok.Accent
					}
					indent := float32(strings.Count(path, "/")) * float32(tok.Spacing12)
					ui.Row(c.Key("outline-"+id)).FillWidth().Padding(0, 0, 0, indent).Children(func() {
						ui.Row(c).FillWidth().Padding(float32(tok.Spacing8), float32(tok.Spacing12)).
							Radius(float32(tok.ControlRadius)).Background(bg).Border(1, border).
							AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
							ui.Text(c, fmt.Sprintf("%02d", i+1)).FontSize(float32(tok.TextMeta)).Bold().
								TextColor(tok.TextMuted).FontFeatures("tnum")
							btn := ui.ButtonBase(c).Label(title).Grow(1).MinWidth(0).Justify(ui.Start).
								OnClick(func() { a.selectAsterStep(steps[a.designSelected], step) })
							btn.Children(func() {
								ui.Column(c).FillWidth().Gap(float32(tok.Spacing4)*0.5).Children(func() {
									ui.Text(c, title).FontSize(float32(tok.TextBody)).Bold().TextColor(tok.Text)
									meta := fmt.Sprintf("%d 个模块", len(blocks))
									if moduleString(step.Data, "execution") == "organizational" {
										meta = "组织阶段"
									} else if len(blocks) == 0 {
										meta = "待添加模块"
									}
									if times := moduleStrings(step.Data, "timepoints"); len(times) > 0 {
										meta += " · " + strings.Join(times, ", ")
									}
									ui.Text(c, meta).FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted).FontFeatures("tnum")
								})
							})
							if a.stepHasUnsavedDraft(step) {
								components.Badge(c, "未保存", components.BadgeOptions{Tone: components.BadgeWarning})
							}
							moreBtn := components.IconButton(c, lucide.Ellipsis(), "步骤操作", components.IconButtonCompact)
							components.Tooltip(c, moreBtn, "调整顺序或删除步骤")
							components.MenuButton(c, moreBtn, func(m *components.Menu) {
								if m.Item("上移").Disabled(i == 0).Chosen() {
									a.applyStepListCommand("move_up", wf, id, "", order)
								}
								if m.Item("下移").Disabled(i >= len(order)-1).Chosen() {
									a.applyStepListCommand("move_down", wf, id, "", order)
								}
								parent := (*asterdoc.Module)(nil)
								parentPath := ""
								if i > 0 {
									parent = steps[order[i-1]]
									if parent != nil {
										parentPath = moduleString(parent.Data, "path")
									}
								}
								canNest := path != "" && parentPath != "" && parentPath != path && !strings.HasPrefix(parentPath, path+"/")
								parentSlash := strings.LastIndex(path, "/")
								canPromote := parentSlash > 0 && stepPaths[path[:parentSlash]]
								parentID := ""
								if parent != nil {
									parentID = parent.ID
								}
								if m.Item("缩进为子步骤").Disabled(!canNest).Chosen() {
									a.applyStepListCommand("indent", wf, id, parentID, order)
								}
								if m.Item("提升层级").Disabled(!canPromote).Chosen() {
									a.applyStepListCommand("outdent", wf, id, "", order)
								}
								m.Separator()
								if m.Item("删除步骤").Chosen() {
									a.applyStepListCommand("delete", wf, id, "", order)
								}
							})
						})
					})
				}
			})
		})
	})
}

func renderStepDetailCanvas(c *ui.Context, a *app, doc *asterdoc.Document, step *asterdoc.Module) {
	tok := theme.GetTokens(c.Theme())
	if step == nil {
		ui.Column(c).Fill().Center().Padding(float32(tok.Spacing24)).Children(func() {
			ui.Text(c, "请在左侧选择一个实验步骤以编辑其模块组合。").
				FontSize(float32(tok.TextBody)).TextColor(tok.TextMuted)
		})
		return
	}
	if a.canvasSelLoaded != step.ID {
		a.loadStepDraft(step)
	}
	dirty := a.isCurrentDrawerDirty(step)
	if dirty {
		a.stepPropSaved = ""
	}

	components.ScrollView(c, components.ScrollViewOptions{}).Fill().Children(func() {
		ui.Column(c).FillWidth().Padding(float32(tok.Spacing16), float32(tok.Spacing24)).
			Gap(float32(tok.Spacing16)).Children(func() {
			// Editable Step Header & Metadata Card
			ui.Column(c).FillWidth().Padding(float32(tok.Spacing16)).Radius(float32(tok.PanelRadius)).
				Background(tok.Surface).Border(1, tok.Border).Gap(float32(tok.Spacing12)).Children(func() {
				ui.Row(c).FillWidth().Wrap().AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
					ui.Box(c).Grow(1).MinWidth(float32(tok.Spacing48)*4).Children(func() {
						components.TextBox(c, &a.canvasTitle, components.TextBoxOptions{}).Placeholder("步骤标题")
					})
					ui.Box(c).Width(float32(tok.Spacing48)*3.5).Children(func() {
						components.TextBox(c, &a.canvasTimepoints, components.TextBoxOptions{}).Placeholder("时间点，例如 6 h, 12 h")
					})
					if dirty {
						if components.Button(c, "保存步骤属性", components.ButtonSecondary).Clicked() {
							a.saveStepProperties(step)
						}
						if components.Button(c, "放弃修改", components.ButtonGhost).Clicked() {
							delete(a.stepDrafts, step.ID)
							a.applyDrawerDraft(step.ID, stepBaseProps(step))
						}
					} else if a.stepPropSaved == step.ID {
						components.Badge(c, "已保存", components.BadgeOptions{Tone: components.BadgeSuccess})
					}
				})

				metaOpenPtr := a.ensureDesignMetaOpen(step.ID)
				ui.Row(c).FillWidth().Wrap().AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
					components.Collapsible(c, "步骤高级选项与技术标识", metaOpenPtr, components.CollapsibleOptions{}, func() {
						ui.Row(c).FillWidth().Wrap().Gap(float32(tok.Spacing12)).Padding(float32(tok.Spacing8), 0, 0, 0).
							AlignItems(ui.Center).Children(func() {
							ui.Text(c, "步骤标识："+step.ID).
								FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
							ui.Text(c, "执行模式："+firstNonEmpty(moduleString(step.Data, "execution"), "executable")).
								FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
							ui.Text(c, "检测绑定："+firstNonEmpty(moduleString(step.Data, "assay"), "自动默认")).
								FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
							ui.Text(c, "执行路径："+firstNonEmpty(moduleString(step.Data, "path"), "—")).
								FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
						})
					})
					ui.Spacer(c)
					if moduleString(step.Data, "execution") == "organizational" {
						components.Badge(c, "组织分组")
					} else {
						components.Badge(c, "可执行步骤", components.BadgeOptions{Tone: components.BadgeAccent})
					}
					if assayRef := moduleString(step.Data, "assay"); assayRef != "" {
						assayLabel := assayRef
						if doc != nil {
							for i := range doc.Modules {
								if doc.Modules[i].Kind == "assay" && doc.Modules[i].ID == assayRef {
									if title := moduleString(doc.Modules[i].Data, "title"); title != "" {
										assayLabel = title
									}
									break
								}
							}
						}
						components.Badge(c, assayLabel)
					}
				})
				dirty = a.syncCurrentStepDraft(step)
			})

			// Composable Generic Module Stream + Palette
			view := a.ensureDesignView(doc, step.ID)
			view.HideStepHeader = true
			asterdoc.StepBlockComposer(view, c, step)
		})
	})
}

var designMetaOpenState = map[string]*bool{}

func (a *app) ensureDesignMetaOpen(stepID string) *bool {
	k := fmt.Sprintf("%p:%s", a, stepID)
	if p, ok := designMetaOpenState[k]; ok {
		return p
	}
	v := false
	designMetaOpenState[k] = &v
	return &v
}

func (a *app) saveStepProperties(step *asterdoc.Module) {
	times := []any{}
	for _, value := range strings.Split(a.canvasTimepoints, ",") {
		if value = strings.TrimSpace(value); value != "" {
			times = append(times, value)
		}
	}
	properties := map[string]any{"title": a.canvasTitle, "timepoints": times}
	if a.canvasAssay != "" {
		properties["assay"] = a.canvasAssay
	}
	if rawTasks, ok := step.Data["tasks"].([]any); ok && len(rawTasks) > 0 {
		tasks := make([]any, len(rawTasks))
		for i, raw := range rawTasks {
			task, ok := raw.(map[string]any)
			if !ok {
				tasks[i] = raw
				continue
			}
			updated := make(map[string]any, len(task))
			for key, value := range task {
				updated[key] = value
			}
			if i == 0 && len(a.canvasTasks) > 0 {
				updated["title"] = a.canvasTasks[0].Title
			}
			tasks[i] = updated
		}
		properties["tasks"] = tasks
	}
	if a.applyDesignRaw(map[string]any{
		"type": "update_module", "moduleId": step.ID, "properties": properties,
	}) {
		delete(a.stepDrafts, step.ID)
		a.stepPropSaved = step.ID
		a.canvasSelLoaded = ""
	}
}

func renderGlobalSetupTab(c *ui.Context, a *app, doc *asterdoc.Document) {
	tok := theme.GetTokens(c.Theme())
	view := a.ensureDesignView(doc, "")
	components.ScrollView(c, components.ScrollViewOptions{}).Fill().Children(func() {
		ui.Column(c).FillWidth().Padding(float32(tok.Spacing24)).AlignItems(ui.Center).Children(func() {
			ui.Column(c).FillWidth().MaxWidth(float32(tok.Spacing48)*22).MaxWidthPercent(100).
				Gap(float32(tok.Spacing16)).Children(func() {
				ui.Row(c).FillWidth().Wrap().AlignItems(ui.Center).Gap(float32(tok.Spacing12)).Children(func() {
					ui.Column(c).Gap(float32(tok.Spacing4)).Children(func() {
						ui.Text(c, "实验全局设定").FontSize(float32(tok.TextSection)).Bold().TextColor(tok.Text)
						ui.Text(c, "集中维护实验档案、实验分组表、标准容器（孔板/培养皿/试管架）、材料试剂与仪器档案。").
							FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
					})
					ui.Spacer(c)
					addBtn := components.Button(c, "+ 添加全局模块", components.ButtonSecondary)
					components.MenuButton(c, addBtn, func(m *components.Menu) {
						n := len(doc.Modules) + 1
						if m.Item("分组表 (group_table)").Chosen() {
							_ = a.applyDesignRaw(map[string]any{"type": "add_module", "module": map[string]any{
								"kind": "group_table", "id": fmt.Sprintf("gt-%d", n), "schema": 1, "title": "实验分组表",
								"rows": []any{map[string]any{"id": fmt.Sprintf("g-%d", n), "name": "新实验组", "role": "treatment", "replicate_count": 3}},
							}})
						}
						if m.Item("标准容器 (plate / dish / rack)").Chosen() {
							aid := defaultFirstAssayID(doc)
							_ = a.applyDesignRaw(map[string]any{"type": "add_module", "module": map[string]any{
								"kind": "plate", "id": fmt.Sprintf("plate-%d", n), "schema": 1, "format": 96,
								"topology": "plate", "assay": aid, "groups": map[string]any{},
							}})
						}
						if m.Item("试剂与耗材 (material)").Chosen() {
							_ = a.applyDesignRaw(map[string]any{"type": "add_module", "module": map[string]any{
								"kind": "material", "id": fmt.Sprintf("mat-%d", n), "schema": 1,
								"name": "新试剂", "role": "treatment", "source": "lab_imported",
							}})
						}
						if m.Item("仪器能力档案 (instrument_profile)").Chosen() {
							_ = a.applyDesignRaw(map[string]any{"type": "add_module", "module": map[string]any{
								"kind": "instrument_profile", "id": fmt.Sprintf("inst-%d", n), "schema": 1,
								"class": "plate_reader", "model": "多功能酶标仪", "source": "lab_imported",
								"capabilities": []any{map[string]any{"name": "absorbance"}},
							}})
						}
					})
				})

				for i := range doc.Modules {
					m := doc.Modules[i]
					if m.Kind == "step" || m.Kind == "workflow" {
						continue
					}
					view.RenderModule(c, m.Kind, m.Schema, m.Data)
				}
				for i := range doc.Modules {
					if doc.Modules[i].Kind == "workflow" {
						m := doc.Modules[i]
						view.RenderModule(c, m.Kind, m.Schema, m.Data)
					}
				}
			})
		})
	})
}

func renderAsterMLSourceTab(c *ui.Context, a *app, lastValidDoc *asterdoc.Document, srcEd *editor.State) {
	tok := theme.GetTokens(c.Theme())
	bufText := srcEd.Buffer.String()
	parsedDoc, parseErr := asterdoc.Parse(bufText)
	dirty := bufText != a.designDoc.Source

	editorPane := func() {
		ui.Column(c).Fill().Background(tok.Surface).Children(func() {
			ui.Row(c).FillWidth().Padding(float32(tok.Spacing8), float32(tok.Spacing12)).
				AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
				if parseErr != nil {
					components.Badge(c, "方案格式有误", components.BadgeOptions{Tone: components.BadgeDanger})
				} else {
					components.Badge(c, "方案格式正常", components.BadgeOptions{Tone: components.BadgeSuccess})
				}
				ui.Spacer(c)
				if dirty {
					if components.Button(c, "还原", components.ButtonGhost).Clicked() {
						srcEd.Buffer = editor.NewBuffer(a.designDoc.Source)
						if lang := editor.LanguageFor("design.md"); lang != nil {
							srcEd.Buffer.SetLanguage(lang)
						}
						srcEd.DiffBase = editor.NewBuffer(a.designDoc.Source)
					}
				}
				applyBtn := components.Button(c, "应用并保存修订", components.ButtonSecondary)
				applyBtn.Disabled(!dirty || parseErr != nil)
				if applyBtn.Clicked() && dirty && parseErr == nil {
					a.saveAsterDocSource(bufText)
				}
			})
			ui.Divider(c)
			if parseErr != nil {
				renderDesignErrorBanner(c, parseErr.Error(), nil)
			}
			ui.Box(c).FillWidth().Grow(1).MinHeight(0).Background(tok.CodeBg).Children(func() {
				editor.View(c, srcEd).Fill()
			})
		})
	}

	previewPane := func() {
		docToShow := parsedDoc
		if docToShow == nil {
			docToShow = lastValidDoc
		}
		previewView := &asterdoc.View{
			Doc:              docToShow,
			Registry:         asterdoc.BuiltinRegistry(),
			Mode:             asterdoc.ModeProtocol,
			DocumentRevision: fmt.Sprint(a.designDoc.Revision),
			ReadOnly:         true,
		}
		components.ScrollView(c, components.ScrollViewOptions{}).Fill().Children(func() {
			ui.Column(c).FillWidth().Padding(float32(tok.Spacing16)).Gap(float32(tok.Spacing12)).Children(func() {
				previewView.Render(c)
			})
		})
	}

	if designNarrow(c) {
		components.ScrollView(c, components.ScrollViewOptions{}).Fill().Children(func() {
			ui.Column(c).FillWidth().Children(func() {
				ui.Box(c).FillWidth().Height(float32(tok.Spacing48) * 8).Children(editorPane)
				previewPane()
			})
		})
		return
	}

	if a.designSrcSplitPct <= 0 {
		a.designSrcSplitPct = 52
	}
	components.SplitPercent(c, &a.designSrcSplitPct, editorPane, previewPane, components.SplitOptions{
		FirstMin:  280,
		SecondMin: 280,
		Default:   52,
	}).Label("AsterML 源码与实时预览分隔条").Fill()
}

func defaultFirstAssayID(doc *asterdoc.Document) string {
	if doc == nil {
		return ""
	}
	for _, m := range doc.Modules {
		if m.Kind == "assay" {
			return m.ID
		}
	}
	return ""
}

func stepBaseProps(step *asterdoc.Module) stepPropDraft {
	draft := stepPropDraft{
		Title:      moduleString(step.Data, "title"),
		Assay:      moduleString(step.Data, "assay"),
		Timepoints: strings.Join(moduleStrings(step.Data, "timepoints"), ", "),
	}
	if tasks, ok := step.Data["tasks"].([]any); ok {
		for _, raw := range tasks {
			task, _ := raw.(map[string]any)
			draft.Tasks = append(draft.Tasks, canvasTaskDraft{
				ID: moduleString(task, "id"), Title: moduleString(task, "title"),
				Kind: moduleString(task, "kind"), Readout: moduleString(task, "readout"),
			})
		}
	}
	return draft
}

func stepPropDraftEqual(x, y stepPropDraft) bool {
	if x.Title != y.Title || x.Assay != y.Assay || x.Timepoints != y.Timepoints || len(x.Tasks) != len(y.Tasks) {
		return false
	}
	for i := range x.Tasks {
		if x.Tasks[i] != y.Tasks[i] {
			return false
		}
	}
	return true
}

func (a *app) currentDrawerDraft() stepPropDraft {
	return stepPropDraft{
		Title:      a.canvasTitle,
		Assay:      a.canvasAssay,
		Timepoints: a.canvasTimepoints,
		Tasks:      append([]canvasTaskDraft(nil), a.canvasTasks...),
	}
}

func (a *app) applyDrawerDraft(stepID string, d stepPropDraft) {
	a.canvasSelLoaded = stepID
	a.canvasTitle = d.Title
	a.canvasAssay = d.Assay
	a.canvasTimepoints = d.Timepoints
	a.canvasTasks = append([]canvasTaskDraft(nil), d.Tasks...)
}

func (a *app) loadStepDraft(step *asterdoc.Module) {
	if step == nil {
		return
	}
	if draft, ok := a.stepDrafts[step.ID]; ok {
		a.applyDrawerDraft(step.ID, draft)
		return
	}
	a.applyDrawerDraft(step.ID, stepBaseProps(step))
}

func (a *app) isCurrentDrawerDirty(step *asterdoc.Module) bool {
	if step == nil || a.canvasSelLoaded != step.ID {
		return false
	}
	return !stepPropDraftEqual(a.currentDrawerDraft(), stepBaseProps(step))
}

func (a *app) syncCurrentStepDraft(step *asterdoc.Module) bool {
	if step == nil || a.canvasSelLoaded != step.ID {
		return false
	}
	cur := a.currentDrawerDraft()
	base := stepBaseProps(step)
	if stepPropDraftEqual(cur, base) {
		delete(a.stepDrafts, step.ID)
		return false
	}
	if a.stepDrafts == nil {
		a.stepDrafts = map[string]stepPropDraft{}
	}
	a.stepDrafts[step.ID] = cur
	return true
}

func (a *app) selectAsterStep(prev, next *asterdoc.Module) {
	if next == nil {
		return
	}
	if prev != nil && prev.ID == next.ID {
		if a.canvasSelLoaded != next.ID {
			a.loadStepDraft(next)
		}
		return
	}
	if prev != nil {
		a.syncCurrentStepDraft(prev)
	}
	a.designSelected = next.ID
	a.stepPropSaved = ""
	a.loadStepDraft(next)
}

func (a *app) stepHasUnsavedDraft(step *asterdoc.Module) bool {
	if step == nil {
		return false
	}
	if a.canvasSelLoaded == step.ID {
		return !stepPropDraftEqual(a.currentDrawerDraft(), stepBaseProps(step))
	}
	_, ok := a.stepDrafts[step.ID]
	return ok
}

func (a *app) hasUnsavedStepDrafts(doc *asterdoc.Document) bool {
	if doc == nil {
		return false
	}
	for i := range doc.Modules {
		if doc.Modules[i].Kind == "step" && a.stepHasUnsavedDraft(&doc.Modules[i]) {
			return true
		}
	}
	return false
}

func (a *app) otherUnsavedStepIDs(order []string, steps map[string]*asterdoc.Module) []string {
	var out []string
	seen := map[string]bool{}
	for _, id := range order {
		if id == a.designSelected || steps[id] == nil {
			continue
		}
		if _, ok := a.stepDrafts[id]; ok {
			out = append(out, id)
			seen[id] = true
		}
	}
	for id := range a.stepDrafts {
		if id != a.designSelected && !seen[id] && steps[id] != nil {
			out = append(out, id)
		}
	}
	return out
}

func renderUnsavedDraftWarning(c *ui.Context, a *app, steps map[string]*asterdoc.Module, ids []string) {
	t := c.Theme()
	tokens := theme.GetTokens(t)
	titles := make([]string, 0, len(ids))
	for _, id := range ids {
		title := ""
		if step := steps[id]; step != nil {
			title = moduleString(step.Data, "title")
		}
		if title == "" {
			title = "未命名步骤"
		}
		titles = append(titles, title)
	}
	ui.Row(c).FillWidth().Wrap().Padding(float32(tokens.Spacing8), float32(tokens.Spacing12)).
		Radius(float32(tokens.ControlRadius)).Background(tokens.WarningBg).
		Border(1, t.Warning.Alpha(.3)).AlignItems(ui.Center).Gap(float32(tokens.Spacing8)).Children(func() {
		ui.Box(c).Grow(1).MinWidth(0).Children(func() {
			ui.Text(c, fmt.Sprintf("未保存 · 步骤「%s」有未保存的属性草稿，已保留", strings.Join(titles, "、"))).
				FontSize(float32(tokens.TextMeta)).TextColor(tokens.WarningText)
		})
		if target := steps[ids[0]]; target != nil {
			if components.Button(c, "切回草稿", components.ButtonSecondary).Clicked() {
				a.selectAsterStep(steps[a.designSelected], target)
			}
		}
		if components.Button(c, "放弃草稿", components.ButtonGhost).Clicked() {
			delete(a.stepDrafts, ids[0])
		}
	})
}

func (a *app) applyStepListCommand(action string, wf *asterdoc.Module, stepID, parentID string, order []string) {
	workflowID := ""
	if wf != nil {
		workflowID = wf.ID
	}
	if command, ok := designer.StepEditCommand(action, workflowID, stepID, parentID, order); ok && a.applyDesignRaw(command) && action == "delete" {
		delete(a.stepDrafts, stepID)
		if a.designSelected == stepID {
			a.designSelected = ""
			a.canvasSelLoaded = ""
		}
	}
}

func (a *app) saveAsterDocSource(source string) bool {
	if a.designSvc == nil || a.designDoc == nil {
		return false
	}
	previous := *a.designDoc
	updated, err := a.designSvc.SaveAsterDoc(context.Background(), runs.AsterDocSaveInput{
		DesignID:         a.designDoc.DesignID,
		Source:           source,
		ExpectedRevision: a.designDoc.Revision,
		OperationID:      a.nextOp(),
		ActorID:          os.Getenv("USERNAME"),
	})
	if err != nil {
		a.designDocErr = state.MapError(err).Error()
		a.designDocLoadFailed = false
		return false
	}
	a.designDocErr = ""
	a.designDocLoadFailed = false
	a.designHistory = append(a.designHistory, previous)
	a.designFuture = nil
	a.resetAsterDraftHistory()
	a.designDoc = &updated
	a.designView = nil
	a.asterSrcLoadedRev = 0
	a.designHierarchy = nil
	a.designHierarchySource = ""
	a.refreshReadiness()
	return true
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
