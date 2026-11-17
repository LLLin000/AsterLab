package main

// Design surface: workflow canvas, document editor and Run creation. Owned by
// the Design Owner after Wave 1 extraction; routing lives in main.go.

import (
	"fmt"
	"strings"

	"asterlab-mygo-prototype/internal/agentedit"
	"asterlab-mygo-prototype/internal/asterdoc"
	"asterlab-mygo-prototype/internal/designer"
	"asterlab-mygo-prototype/internal/theme"
	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/onevra/source/components"
	"github.com/oneo-me/onevra/source/icons/lucide"
)
var agentEditStates = map[*app]*agentedit.State{}
func proposalState(a *app) *agentedit.State {
	if s:=agentEditStates[a]; s!=nil{return s}
	s:=&agentedit.State{Client:agentedit.CLIClient{},ConfirmedBy:"researcher",OnCommitted:func(agentedit.CommitResult){if a.designDoc!=nil{a.loadDesign(a.designDoc.DesignID)}}}
	agentEditStates[a]=s;return s
}
// Agent proposal entry stays secondary to Create Run — UX-P1/P2/P3/P5 and UX-LAYOUT.
func renderAgentProposal(c *ui.Context,a *app) {
	s:=proposalState(a)
	if components.Button(c,"Agent 提案",components.ButtonSecondary).Clicked(){s.Open=true;s.Closed=false}
	if s.Open {components.Dialog(c,&s.Open,components.DialogOptions{Label:"Agent 提案"},func(panel ui.Element){panel.FillWidth().MaxWidth(720).MaxWidthPercent(94).MaxHeightPercent(90).Children(func(){agentedit.View(c,s)})});if !s.Open{s.Closed=true}}
}

func renderDesignErrorBanner(c *ui.Context, message string, onRetry func()) {
	if message == "" {
		return
	}
	t := c.Theme()
	tokens := theme.GetTokens(t)
	ui.Row(c).FillWidth().Padding(6, 12).Background(tokens.DangerBg).
		Border(1, t.Danger.Alpha(.25)).AlignItems(ui.Center).Gap(t.Space(2)).Children(func() {
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
				renderDesignErrorBanner(c, "AsterDoc 设计无法读取："+a.designDocErr, retryLoad)
			}
			ui.Box(c).FillWidth().Grow(1).MinHeight(0).Children(func() {
				// The step-card list is the Design surface's default; the workflow
				// canvas stays one toggle away and is not deleted.
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
	if a.designLoadErr != "" || a.designDocErr != "" {
		title := "设计无法读取"
		msg := a.designLoadErr
		if msg == "" {
			title = "AsterDoc 设计无法读取"
			msg = a.designDocErr
		}
		t := c.Theme()
		ui.Column(c).Fill().Center().Gap(t.Space(2)).Padding(t.Space(4)).Children(func() {
			ui.Text(c, title).FontSize(t.Rem(1.14)).Bold().TextColor(t.Danger) // UX-P3: 16px → Rem.
			ui.Text(c, msg).FontSize(t.Rem(0.93)).TextColor(t.TextMuted) // UX-P3: 13px → Rem.
			ui.Row(c).Gap(t.Space(2)).AlignItems(ui.Center).Children(func() {
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
	t := c.Theme()
	message := "请从项目页选择一个已保存的设计。"
	if a.designSvc == nil { message = "设计服务不可用。" }
	ui.Column(c).Fill().Center().Gap(t.Space(2)).Children(func() {
		components.Text(c, "还没有打开的设计").FontSize(t.Rem(1.15)).FontWeight(600)
		components.Text(c, message).TextColor(t.TextMuted)
		renderAgentProposal(c,a)
	})
}

// asterDesignPage is the document-backed Design surface. Professional editors
// issue semantic commands; this slice supports plate editing and Run creation.
func asterDesignPage(c *ui.Context, a *app) {
	t := c.Theme()
	c.OnShortcut(ui.Cmd, ui.KeyZ, func() { a.designUndo() })
	c.OnShortcut(ui.Cmd|ui.Shift, ui.KeyZ, func() { a.designRedo() })
	c.OnShortcut(ui.Cmd, ui.KeyY, func() { a.designRedo() })
	doc, err := asterdoc.Parse(a.designDoc.Source)
	if err != nil {
		ui.Text(c, "AsterDoc 语法错误："+err.Error()).Padding(t.Space(6)).TextColor(t.Danger)
		return
	}
	ui.Column(c).Fill().Children(func() {
		ui.Row(c).FillWidth().Wrap().Padding(t.Space(2), t.Space(4)).Border(1, t.Border).
			Background(t.Surface).AlignItems(ui.Center).Gap(t.Space(2)).Children(func() {
			ui.Text(c, "AsterDoc 设计").FontSize(t.Rem(1.14)).Bold().TextColor(t.Text) // UX-P3: 16px → Rem.
			ui.Text(c, fmt.Sprintf("rev %d", a.designDoc.Revision)).FontSize(t.Rem(0.79)).
				TextColor(t.TextMuted) // UX-P3: 11px → Rem.
			stateLabel, stateTone := designReadinessState(a)
			components.Badge(c, stateLabel, components.BadgeOptions{Tone: stateTone})
			if a.hasUnsavedStepDrafts(doc) {
				components.Badge(c, "未保存", components.BadgeOptions{Tone: components.BadgeWarning})
			}
			tab := a.designTab
			components.Tabs(c, &tab, components.TabsSegmented, "流程", "文档")
			a.designTab = tab
			ui.Spacer(c)
			undo := components.IconButton(c, lucide.Undo2(), "撤销", components.IconButtonCompact)
			undo.Disabled(len(a.designHistory) == 0 && len(a.asterDraftUndo) == 0)
			if undo.Clicked() {
				a.designUndo()
			}
			redo := components.IconButton(c, lucide.Redo2(), "重做", components.IconButtonCompact)
			redo.Disabled(len(a.designFuture) == 0 && len(a.asterDraftRedo) == 0)
			if redo.Clicked() {
				a.designRedo()
			}
			createBtn := components.Button(c, "创建 Run", components.ButtonPrimary)
			createBtn.Disabled(stateLabel == "校验中")
			if createBtn.Clicked() && stateLabel != "校验中" {
				a.preflightRisk = false
				a.preflightOpen = true
			}
			renderAgentProposal(c,a)
		})
		if a.designer != nil {
			designer.BudgetBar(c, a.designer)
		}
		if a.designLoadErr != "" {
			renderDesignErrorBanner(c, "设计加载失败："+a.designLoadErr, a.retryDesignFunc())
		}
		if a.designDocErr != "" {
			renderDesignErrorBanner(c, a.designDocErr, a.designDocRetryFunc())
		}
		if a.readinessErr != "" {
			renderDesignErrorBanner(c, "Core 校验失败："+a.readinessErr, func() { a.refreshReadiness() })
		}
		if a.createRunErr != "" {
			renderDesignErrorBanner(c, "创建 Run 失败："+a.createRunErr, func() { a.createRunFromDesignDoc() })
		}
		if a.designTab == 0 {
			renderAsterStepList(c, a, doc)
			a.renderPreflight(c)
			return
		}
		ui.Scroll(c).Grow(1).Children(func() {
			ui.Column(c).FillWidth().Padding(t.Space(4), t.Space(5)).
				AlignItems(ui.Center).Children(func() {
				ui.Box(c).FillWidth().MaxWidth(980).MaxWidthPercent(100).
					Background(t.Surface).Border(1, t.Border).Radius(t.Space(2)).
					Padding(t.Space(5), t.Space(6)).Children(func() {
					if a.designView == nil || a.designView.Doc == nil || a.designView.Doc.Source != a.designDoc.Source {
						a.designView = &asterdoc.View{
							Doc: doc, Mode: asterdoc.ModeDesign, DocumentRevision: fmt.Sprint(a.designDoc.Revision),
							Allowed: map[string]bool{asterdoc.PermDocumentEdit: true},
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
						}
					}
					a.designView.Render(c)
				})
			})
		})
	})
	a.renderPreflight(c)
	a.captureAsterDraft()
}

// The list keeps Create Run visible and folds identifiers into the inspector:
// UX-P1, UX-P2, UX-P5, UX-STATE, UX-LAYOUT.
func renderAsterStepList(c *ui.Context, a *app, doc *asterdoc.Document) {
	t := c.Theme()
	var wf *asterdoc.Module
	for i := range doc.Modules {
		if doc.Modules[i].Kind == "workflow" {
			wf = &doc.Modules[i]
			break
		}
	}
	order := []string{}
	if wf != nil {
		key := "business_order"
		if _, ok := wf.Data[key]; !ok {
			key = "steps"
		}
		if values, ok := wf.Data[key].([]any); ok {
			for _, value := range values {
				if id, ok := value.(string); ok {
					order = append(order, id)
				}
			}
		}
	}
	if len(order) == 0 {
		for _, module := range doc.Modules {
			if module.Kind == "step" {
				order = append(order, module.ID)
			}
		}
	}
	steps := make(map[string]*asterdoc.Module, len(order))
	for i := range doc.Modules {
		if doc.Modules[i].Kind == "step" {
			steps[doc.Modules[i].ID] = &doc.Modules[i]
		}
	}
	stepPaths := map[string]bool{}
	for _, step := range steps {
		if path := moduleString(step.Data, "path"); path != "" {
			stepPaths[path] = true
		}
	}
	narrow := designNarrow(c)
	drawList := func() {
		ui.Column(c).FillWidth().Padding(t.Space(2), t.Space(3)).Gap(t.Space(1)).
			Children(func() {
				if unsaved := a.otherUnsavedStepIDs(order, steps); len(unsaved) > 0 {
					renderUnsavedDraftWarning(c, a, steps, unsaved)
				}
				for i, id := range order {
					step := steps[id]
					if step == nil {
						continue
					}
					title := moduleString(step.Data, "title")
					if title == "" {
						title = "未命名步骤"
					}
					path := moduleString(step.Data, "path")
					border := t.Border
					if a.designSelected == id {
						border = t.Accent
					}
					ui.Row(c).FillWidth().Wrap().Padding(t.Space(1.5), t.Space(2)).
						Gap(t.Space(1)).AlignItems(ui.Center).Background(t.Surface).
						Border(1, border).Radius(t.Space(1)).Children(func() {
						ui.Text(c, fmt.Sprintf("%02d", i+1)).FontSize(t.Rem(.75)).
							TextColor(t.TextMuted).FontFeatures("tnum")
						ui.Box(c).Width(t.Space(float32(strings.Count(path, "/")) * 2)).Shrink(0)
						if components.Button(c, title, components.ButtonSecondary).Clicked() {
							a.selectAsterStep(steps[a.designSelected], step)
						}
						if a.stepHasUnsavedDraft(step) {
							components.Badge(c, "未保存", components.BadgeOptions{Tone: components.BadgeWarning})
						}
						if summary := stepParameterSummary(doc, step); summary != "" {
							ui.Text(c, summary).FontSize(t.Rem(.78)).TextColor(t.TextMuted).
								SingleLine().Ellipsis("…").Grow(1)
						}
						if i > 0 {
							if components.Button(c, "上移", components.ButtonSecondary).Clicked() {
								a.applyStepListCommand("move_up", wf, id, "", order)
							}
						}
						if i+1 < len(order) {
							if components.Button(c, "下移", components.ButtonSecondary).Clicked() {
								a.applyStepListCommand("move_down", wf, id, "", order)
							}
						}
						parent := (*asterdoc.Module)(nil)
						if i > 0 {
							parent = steps[order[i-1]]
						}
						parentPath := ""
						if parent != nil {
							parentPath = moduleString(parent.Data, "path")
						}
						canNest := path != "" && parentPath != "" && parentPath != path && !strings.HasPrefix(parentPath, path+"/")
						if canNest {
							if components.Button(c, "缩进", components.ButtonSecondary).Clicked() {
								a.applyStepListCommand("indent", wf, id, parent.ID, order)
							}
						} else {
							reason := components.IconButton(c, lucide.Workflow(), "缩进：需要先设置步骤路径", components.IconButtonPlain)
							reason.Disabled(true)
							ui.Text(c, "缩进").FontSize(t.Rem(.68)).TextColor(t.TextMuted)
							ui.Text(c, "需要先设置步骤路径").FontSize(t.Rem(.68)).TextColor(t.TextMuted)
						}
						parentSlash := strings.LastIndex(path, "/")
						canPromote := parentSlash > 0 && stepPaths[path[:parentSlash]]
						if canPromote {
							if components.Button(c, "提升", components.ButtonSecondary).Clicked() {
								a.applyStepListCommand("outdent", wf, id, "", order)
							}
						} else {
							reason := components.IconButton(c, lucide.Workflow(), "提升：需要先设置步骤路径", components.IconButtonPlain)
							reason.Disabled(true)
							ui.Text(c, "提升").FontSize(t.Rem(.68)).TextColor(t.TextMuted)
							ui.Text(c, "需要先设置步骤路径").FontSize(t.Rem(.68)).TextColor(t.TextMuted)
						}
						if components.Button(c, "删除", components.ButtonSecondary).Clicked() {
							a.applyStepListCommand("delete", wf, id, "", order)
						}
					})
				}
			})
	}
	ui.Box(c).Fill().Children(func() {
		if narrow {
			ui.Scroll(c).Fill().Children(func() {
				ui.Column(c).FillWidth().Children(func() {
					drawList()
					if selected := steps[a.designSelected]; selected != nil {
						stepPropertiesDrawer(c, a, doc, selected, true)
					}
				})
			})
			return
		}
		ui.Row(c).Fill().AlignItems(ui.Stretch).Children(func() {
			ui.Scroll(c).Grow(1).MinWidth(0).Children(drawList)
			if selected := steps[a.designSelected]; selected != nil {
				stepPropertiesDrawer(c, a, doc, selected, false)
			}
		})
	})
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
	ui.Row(c).FillWidth().Wrap().Padding(6, 12).Background(tokens.WarningBg).
		Border(1, t.Warning.Alpha(.3)).AlignItems(ui.Center).Gap(t.Space(2)).Children(func() {
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

func stepParameterSummary(doc *asterdoc.Document, step *asterdoc.Module) string {
	parts := []string{}
	assayID := moduleString(step.Data, "assay")
	for _, module := range doc.Modules {
		if module.Kind == "assay" && module.ID == assayID {
			if title := moduleString(module.Data, "title"); title != "" {
				parts = append(parts, title)
			}
			break
		}
	}
	if times := moduleStrings(step.Data, "timepoints"); len(times) > 0 {
		parts = append(parts, strings.Join(times, " · "))
	}
	if tasks, ok := step.Data["tasks"].([]any); ok && len(tasks) > 0 {
		if task, ok := tasks[0].(map[string]any); ok {
			if title := moduleString(task, "title"); title != "" {
				parts = append(parts, title)
			}
		}
	}
	return strings.Join(parts, " · ")
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

func stepPropertiesDrawer(c *ui.Context, a *app, doc *asterdoc.Document, step *asterdoc.Module, bottom bool) {
	t := c.Theme()
	tokens := theme.GetTokens(t)
	if a.canvasSelLoaded != step.ID {
		a.loadStepDraft(step)
	}
	dirty := a.isCurrentDrawerDirty(step)
	if dirty {
		a.stepPropSaved = ""
	}
	panel := ui.Column(c).Width(t.Space(90)).MinWidth(320).Shrink(0).FillHeight().
		Padding(t.Space(3)).Gap(t.Space(1.5)).Background(t.Surface).Border(1, t.Border)
	if bottom {
		panel = ui.Column(c).FillWidth().Padding(t.Space(3)).Gap(t.Space(1.5)).
			Background(t.Surface).Border(1, t.Border)
	}
	panel.Children(func() {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(t.Space(1)).Children(func() {
			ui.Text(c, "步骤属性").FontSize(t.Rem(.95)).Bold().TextColor(t.Text)
			ui.Spacer(c)
			if dirty {
				components.Badge(c, "未保存", components.BadgeOptions{Tone: components.BadgeWarning})
			} else if a.stepPropSaved == step.ID {
				components.Badge(c, "已保存", components.BadgeOptions{Tone: components.BadgeSuccess})
			}
		})
		ui.Text(c, "标题").FontSize(t.Rem(.72)).TextColor(t.TextMuted)
		ui.TextInput(c, &a.canvasTitle).FillWidth()
		if len(a.canvasTasks) > 0 {
			ui.Text(c, "操作摘要").FontSize(t.Rem(.72)).TextColor(t.TextMuted)
			ui.TextInput(c, &a.canvasTasks[0].Title).FillWidth()
		}
		ui.Text(c, "检测").FontSize(t.Rem(.72)).TextColor(t.TextMuted)
		ui.TextInput(c, &a.canvasAssay).FillWidth()
		ui.Text(c, "时间点").FontSize(t.Rem(.72)).TextColor(t.TextMuted)
		ui.TextInput(c, &a.canvasTimepoints).FillWidth()
		dirty = a.syncCurrentStepDraft(step)
		if dirty {
			ui.Text(c, "已修改属性，点击「保存属性」写入 Core").FontSize(t.Rem(.72)).TextColor(tokens.WarningText)
		}
		if summary := stepParameterSummary(doc, step); summary != "" {
			ui.Text(c, "当前参数摘要 · "+summary).FontSize(t.Rem(.75)).TextColor(t.TextMuted)
		}
		ui.Row(c).FillWidth().Wrap().AlignItems(ui.Center).Gap(t.Space(1)).Children(func() {
			if components.Button(c, "保存属性", components.ButtonSecondary).Clicked() {
				times := []any{}
				for _, value := range strings.Split(a.canvasTimepoints, ",") {
					if value = strings.TrimSpace(value); value != "" {
						times = append(times, value)
					}
				}
				properties := map[string]any{"title": a.canvasTitle, "assay": a.canvasAssay, "timepoints": times}
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
			if dirty {
				if components.Button(c, "放弃修改", components.ButtonGhost).Clicked() {
					delete(a.stepDrafts, step.ID)
					a.applyDrawerDraft(step.ID, stepBaseProps(step))
				}
			}
		})
	})
}
