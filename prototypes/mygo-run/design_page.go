package main

// Design surface: AsterML v2 Scientific Design Workbench (`编辑`, `阅读`, `源码`)
// backed by Core immutable revisions and reusable generic content modules.

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

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

type designOverviewState struct {
	open              bool
	loadedKey         string
	lastSavedDesignID string
	lastSavedBody     string
	title             string
	scheduledDate     string
	objective         string
	endpoint          string
}

var designOverviewStates = map[*app]*designOverviewState{}
var stepSavedBodies = map[string]string{}

func ensureDesignOverviewState(a *app, doc *asterdoc.Document) *designOverviewState {
	st := designOverviewStates[a]
	if st == nil {
		st = &designOverviewState{}
		designOverviewStates[a] = st
	}
	designID := ""
	if a.designDoc != nil {
		designID = a.designDoc.DesignID
	}
	exp := findExperimentModule(doc)
	expBody := ""
	if exp != nil {
		expBody = exp.Body
	}
	if st.loadedKey == "" || st.lastSavedDesignID != designID || (exp != nil && expBody != st.lastSavedBody) {
		st.loadedKey = designID + ":" + expBody
		st.lastSavedDesignID = designID
		st.lastSavedBody = expBody
		if exp != nil {
			st.title = moduleString(exp.Data, "title")
			st.scheduledDate = asterdoc.NormalizeDateYMD(moduleString(exp.Data, "scheduled_date"))
			st.objective = moduleString(exp.Data, "objective")
			st.endpoint = moduleString(exp.Data, "primary_endpoint")
		} else {
			st.title = ""
			st.scheduledDate = ""
			st.objective = ""
			st.endpoint = ""
		}
	}
	return st
}

func findExperimentModule(doc *asterdoc.Document) *asterdoc.Module {
	if doc == nil {
		return nil
	}
	for i := range doc.Modules {
		if doc.Modules[i].Kind == "experiment" {
			return &doc.Modules[i]
		}
	}
	return nil
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
			})
		})
		return
	}
	message := "请从实验日程页选择一个实验，或点击左上角加入新实验。"
	if a.designSvc == nil {
		message = "设计服务不可用。"
	}
	ui.Column(c).Fill().Center().Gap(float32(tok.Spacing8)).Children(func() {
		components.Text(c, "还没有打开的设计").FontSize(float32(tok.TextPage)).FontWeight(600)
		components.Text(c, message).FontSize(float32(tok.TextBody)).TextColor(t.TextMuted)
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
	baseDir := a.designBaseDir()
	if a.designView == nil || a.designView.Doc == nil || a.designView.Doc.Source != a.designDoc.Source {
		a.designView = &asterdoc.View{
			Doc:              doc,
			Mode:             asterdoc.ModeDesign,
			DocumentRevision: rev,
			BaseDir:          baseDir,
			Allowed:          map[string]bool{asterdoc.PermDocumentEdit: true},
			OnAssignWells: func(moduleID, group string, wells []string) {
				a.applyDesignCommand("assign_wells", moduleID, group, wells)
			},
			OnClearWells: func(moduleID string, wells []string) {
				a.applyDesignCommand("clear_wells", moduleID, "", wells)
			},
			OnAddModule: func(mod map[string]any) {
				_ = a.applyDesignRaw(map[string]any{
					"type": "add_module", "module": mod,
				})
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
				curBase := a.designBaseDir()
				srcVal := asterdoc.PortableFigureAssetPath(curBase, picked)
				props := map[string]any{"src": srcVal}
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
	a.designView.BaseDir = baseDir
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

	if a.designTab < 0 || a.designTab > 2 {
		a.designTab = 0
	}
	srcEd := a.ensureAsterSrcEditor(a.designDoc.Source, a.designDoc.Revision)
	doc, err := asterdoc.Parse(a.designDoc.Source)
	if err != nil {
		if a.designTab != 2 {
			ui.Column(c).Fill().Padding(float32(tok.Spacing24)).Gap(float32(tok.Spacing12)).Children(func() {
				renderDesignErrorBanner(c, "AsterML 语法错误："+err.Error(), a.designDocRetryFunc())
				if components.Button(c, "切换到源码修复", components.ButtonSecondary).Clicked() {
					a.designTab = 2
				}
			})
			return
		}
		ui.Column(c).Fill().Children(func() {
			ui.Row(c).FillWidth().Wrap().Padding(float32(tok.Spacing8), float32(tok.Spacing16)).
				Background(tok.Surface).AlignItems(ui.Center).Gap(float32(tok.Spacing12)).Children(func() {
				ui.Text(c, "源码语法修复").FontSize(float32(tok.TextSection)).Bold().TextColor(tok.Text)
				components.Badge(c, "语法待修复", components.BadgeOptions{Tone: components.BadgeDanger})
				ui.Spacer(c)
				tab := a.designTab
				components.Tabs(c, &tab, components.TabsSegmented, "编辑", "阅读", "源码")
				a.designTab = tab
			})
			ui.Divider(c)
			ui.Box(c).FillWidth().Grow(1).MinHeight(0).Children(func() {
				renderAsterMLSourceTab(c, a, nil, srcEd)
			})
		})
		return
	}

	var coreErrors []string
	if a.readiness != nil {
		coreErrors = a.readiness.Errors
	}
	expMeta := asterdoc.InspectExperiment(a.designDoc.Source, coreErrors)
	_, srcParseErr := asterdoc.Parse(strings.ReplaceAll(srcEd.Buffer.String(), "\r\n", "\n"))
	srcDirty := strings.ReplaceAll(srcEd.Buffer.String(), "\r\n", "\n") != strings.ReplaceAll(a.designDoc.Source, "\r\n", "\n")

	ui.Column(c).Fill().Children(func() {
		// Workbench Header
		ui.Row(c).FillWidth().Wrap().Padding(float32(tok.Spacing8), float32(tok.Spacing16)).
			Background(tok.Surface).
			AlignItems(ui.Center).Gap(float32(tok.Spacing12)).Children(func() {
			ui.Row(c).AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
				ui.Text(c, designExperimentTitle(doc)).FontSize(float32(tok.TextSection)).Bold().TextColor(tok.Text)
				if a.readinessLoading {
					components.Badge(c, "校验中", components.BadgeOptions{Tone: components.BadgeNeutral})
				} else if expMeta.Complete {
					components.Badge(c, "方案就绪", components.BadgeOptions{Tone: components.BadgeSuccess})
				} else {
					components.Badge(c, "设计中 · "+expMeta.IncompleteReason, components.BadgeOptions{Tone: components.BadgeWarning})
				}
				if srcDirty && srcParseErr != nil {
					components.Badge(c, "待语法闭合", components.BadgeOptions{Tone: components.BadgeNeutral})
				} else {
					ui.Text(c, "实时保存").FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
				}
			})

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

			tab := a.designTab
			components.Tabs(c, &tab, components.TabsSegmented, "编辑", "阅读", "源码")
			a.designTab = tab

			pdfBtn := components.Button(c, "导出 PDF", components.ButtonSecondary)
			pdfBtn.Disabled(a.pdfExporting)
			if pdfBtn.Clicked() && !a.pdfExporting {
				a.startDesignPDFExport(c, doc)
			}

			createBtn := components.Button(c, "开始实验", components.ButtonPrimary)
			createBtn.Disabled(a.readinessLoading)
			if createBtn.Clicked() && !a.readinessLoading {
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
			renderDesignErrorBanner(c, "启动实验失败："+a.createRunErr, func() { a.createRunFromDesignDoc() })
		}

		ui.Box(c).FillWidth().Grow(1).MinHeight(0).Children(func() {
			switch a.designTab {
			case 0:
				renderStepWorkbenchTab(c, a, doc)
			case 1:
				renderSinglePageSheetTab(c, a, doc)
			case 2:
				renderAsterMLSourceTab(c, a, doc, srcEd)
			}
		})
	})

	a.renderPreflight(c)
	if a.designView != nil {
		a.designView.RenderPlateOverlay(c)
	}
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

	if a.shell != nil && a.shell.SplitOpen {
		ui.Column(c).Fill().MinWidth(0).MinHeight(0).Children(func() {
			tok := theme.GetTokens(c.Theme())
			ui.Row(c).FillWidth().Wrap().Padding(float32(tok.Spacing8), float32(tok.Spacing12)).
				Background(tok.Surface).Border(1, tok.Border).AlignItems(ui.Center).
				Gap(float32(tok.Spacing4)).Children(func() {
				ui.Text(c, "步骤：").FontSize(float32(tok.TextMeta)).Bold().TextColor(tok.TextMuted).SingleLine()
				for i, id := range order {
					i, id := i, id
					step := steps[id]
					if step == nil {
						continue
					}
					label := firstNonEmpty(moduleString(step.Data, "title"), fmt.Sprintf("%02d 步骤", i+1))
					style := components.ButtonGhost
					if a.designSelected == id {
						style = components.ButtonSecondary
					}
					if components.Button(c, label, style).Clicked() {
						a.designSelected = id
					}
				}
				if components.Button(c, "＋ 步骤", components.ButtonOutline).Clicked() {
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
			ui.Box(c).FillWidth().Grow(1).MinHeight(0).Children(func() {
				detailFn()
			})
		})
		return
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
		ui.Row(c).FillWidth().Wrap().Padding(float32(tok.Spacing12)).
			AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
			ui.Row(c).AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
				ui.Text(c, "步骤大纲").FontSize(float32(tok.TextSection)).Bold().TextColor(tok.Text).SingleLine()
				components.Badge(c, fmt.Sprintf("%d 步", len(order)))
			})
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
							if moduleString(step.Data, "execution") != "organizational" && len(blocks) == 0 && len(moduleStrings(step.Data, "block_refs")) == 0 {
								components.Badge(c, "待补模块", components.BadgeOptions{Tone: components.BadgeWarning})
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
	stepStateKey := fmt.Sprintf("%p:%s", a, step.ID)
	if a.canvasSelLoaded != step.ID || stepSavedBodies[stepStateKey] != step.Body {
		a.loadStepDraft(step)
		stepSavedBodies[stepStateKey] = step.Body
	}

	components.ScrollView(c, components.ScrollViewOptions{}).Fill().Children(func() {
		ui.Column(c).FillWidth().Padding(float32(tok.Spacing16), float32(tok.Spacing24)).
			Gap(float32(tok.Spacing16)).Children(func() {
			view := a.ensureDesignView(doc, step.ID)
			view.HideStepHeader = true

			// Compact collapsible experiment overview & top-level group_table header card
			renderDesignOverviewHeaderCard(c, a, doc, view)

			// Editable Step Header & Metadata Card (real-time auto-saved to .asterml)
			ui.Column(c).FillWidth().Padding(float32(tok.Spacing16)).Radius(float32(tok.PanelRadius)).
				Background(tok.Surface).Border(1, tok.Border).Gap(float32(tok.Spacing12)).Children(func() {
				ui.Row(c).FillWidth().Wrap().AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
					ui.Box(c).Grow(1).MinWidth(float32(tok.Spacing48)*4).Children(func() {
						components.TextBox(c, &a.canvasTitle, components.TextBoxOptions{}).Placeholder("步骤标题")
					})
					ui.Box(c).Width(float32(tok.Spacing48)*3.5).Children(func() {
						components.TextBox(c, &a.canvasTimepoints, components.TextBoxOptions{}).Placeholder("时间点，例如 6 h, 12 h")
					})
					ui.Text(c, "已实时写入 .asterml").FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
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
					if assayRef := moduleString(step.Data, "assay"); assayRef != "" && doc != nil {
						assayLabel := ""
						for i := range doc.Modules {
							if doc.Modules[i].Kind == "assay" && doc.Modules[i].ID == assayRef {
								if title := moduleString(doc.Modules[i].Data, "title"); title != "" && !asterdoc.IsRawIdentifier(title) {
									assayLabel = title
								}
								break
							}
						}
						if assayLabel != "" {
							components.Badge(c, assayLabel)
						}
					}
				})
				if a.isStepHeaderModified(step) {
					a.saveStepPropertiesRealtime(step, stepStateKey)
				}
			})

			// Composable Generic Module Stream + Palette
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

func (a *app) isStepHeaderModified(step *asterdoc.Module) bool {
	if step == nil || a.canvasSelLoaded != step.ID {
		return false
	}
	curTitle := strings.TrimSpace(a.canvasTitle)
	if curTitle == "" {
		curTitle = "未命名步骤"
	}
	baseTitle := strings.TrimSpace(moduleString(step.Data, "title"))
	if baseTitle == "" {
		baseTitle = "未命名步骤"
	}
	if curTitle != baseTitle {
		return true
	}
	curTimes := normalizeTimepointsCSV(a.canvasTimepoints)
	baseTimes := strings.Join(moduleStrings(step.Data, "timepoints"), ", ")
	return curTimes != baseTimes
}

func normalizeTimepointsCSV(raw string) string {
	var parts []string
	for _, value := range strings.Split(raw, ",") {
		if v := strings.TrimSpace(value); v != "" {
			parts = append(parts, v)
		}
	}
	return strings.Join(parts, ", ")
}

func (a *app) saveStepPropertiesRealtime(step *asterdoc.Module, stateKey string) {
	times := []any{}
	for _, value := range strings.Split(a.canvasTimepoints, ",") {
		if value = strings.TrimSpace(value); value != "" {
			times = append(times, value)
		}
	}
	titleVal := strings.TrimSpace(a.canvasTitle)
	if titleVal == "" {
		titleVal = "未命名步骤"
	}
	properties := map[string]any{"title": titleVal, "timepoints": times}
	if a.canvasAssay != "" {
		properties["assay"] = a.canvasAssay
	}
	if a.applyDesignRaw(map[string]any{
		"type": "update_module", "moduleId": step.ID, "properties": properties,
	}) {
		delete(a.stepDrafts, step.ID)
		a.stepPropSaved = step.ID
		if a.designDoc != nil {
			if updatedDoc, err := asterdoc.Parse(a.designDoc.Source); err == nil {
				for _, m := range updatedDoc.Modules {
					if m.Kind == "step" && m.ID == step.ID {
						stepSavedBodies[stateKey] = m.Body
						break
					}
				}
			}
		}
	}
}

func (a *app) saveStepProperties(step *asterdoc.Module) {
	a.saveStepPropertiesRealtime(step, fmt.Sprintf("%p:%s", a, step.ID))
}

func renderSinglePageSheetTab(c *ui.Context, a *app, doc *asterdoc.Document) {
	view := a.ensureDesignView(doc, "")
	components.ScrollView(c, components.ScrollViewOptions{}).Fill().Children(func() {
		view.RenderDocumentSheet(c)
	})
}

func renderDesignOverviewHeaderCard(c *ui.Context, a *app, doc *asterdoc.Document, view *asterdoc.View) {
	tok := theme.GetTokens(c.Theme())
	st := ensureDesignOverviewState(a, doc)
	expMod := findExperimentModule(doc)
	var groupTables []asterdoc.Module
	if doc != nil {
		for _, m := range doc.Modules {
			if m.Kind == "group_table" {
				groupTables = append(groupTables, m)
			}
		}
	}
	ui.Column(c).FillWidth().Padding(float32(tok.Spacing12), float32(tok.Spacing16)).
		Radius(float32(tok.PanelRadius)).Background(tok.Surface).Border(1, tok.Border).
		Gap(float32(tok.Spacing8)).Children(func() {
		components.Collapsible(c, "实验概览与全局分组", &st.open, components.CollapsibleOptions{}, func() {
			ui.Column(c).FillWidth().Gap(float32(tok.Spacing12)).Padding(float32(tok.Spacing8), 0, 0, 0).Children(func() {
				if expMod != nil {
					ui.Row(c).FillWidth().Wrap().Gap(float32(tok.Spacing8)).AlignItems(ui.End).Children(func() {
						ui.Column(c).Grow(1).MinWidth(float32(tok.Spacing48)*4).Gap(float32(tok.Spacing4)).Children(func() {
							ui.Text(c, "实验标题").FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
							components.TextBox(c, &st.title, components.TextBoxOptions{}).Placeholder("输入实验研究题目")
						})
						ui.Column(c).Width(float32(tok.Spacing48)*3.8).Gap(float32(tok.Spacing4)).Children(func() {
							ui.Text(c, "计划日期").FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
							ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(float32(tok.Spacing4)).Children(func() {
								ui.Box(c).Grow(1).MinWidth(0).Children(func() {
									components.TextBox(c, &st.scheduledDate, components.TextBoxOptions{}).Placeholder("YYYY-MM-DD")
								})
								if components.Button(c, "今天", components.ButtonGhost).Clicked() {
									st.scheduledDate = time.Now().Format("2006-01-02")
								}
								if components.Button(c, "明天", components.ButtonGhost).Clicked() {
									st.scheduledDate = time.Now().AddDate(0, 0, 1).Format("2006-01-02")
								}
							})
						})
						ui.Column(c).Width(float32(tok.Spacing48)*4.0).Gap(float32(tok.Spacing4)).Children(func() {
							ui.Text(c, "主要观测指标").FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
							components.TextBox(c, &st.endpoint, components.TextBoxOptions{}).Placeholder("例如 抑菌圈直径 (mm)")
						})
					})
					ui.Column(c).FillWidth().Gap(float32(tok.Spacing4)).Children(func() {
						ui.Text(c, "实验目的 / 核心假说").FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
						components.TextBox(c, &st.objective, components.TextBoxOptions{}).Placeholder("简述实验目标、比较条件与科学问题")
					})
					targetTitle := strings.TrimSpace(st.title)
					if targetTitle == "" {
						targetTitle = "未命名实验"
					}
					curTitle := firstNonEmpty(strings.TrimSpace(moduleString(expMod.Data, "title")), "未命名实验")
					curDate := asterdoc.NormalizeDateYMD(moduleString(expMod.Data, "scheduled_date"))
					trimDateInput := strings.TrimSpace(st.scheduledDate)
					normDate := asterdoc.NormalizeDateYMD(trimDateInput)
					dateChanged := (trimDateInput == "" && curDate != "") || (normDate != "" && normDate != curDate)
					trimObjective := strings.TrimSpace(st.objective)
					trimEndpoint := strings.TrimSpace(st.endpoint)
					if targetTitle != curTitle ||
						dateChanged ||
						trimObjective != strings.TrimSpace(moduleString(expMod.Data, "objective")) ||
						trimEndpoint != strings.TrimSpace(moduleString(expMod.Data, "primary_endpoint")) {
						props := map[string]any{
							"title":            targetTitle,
							"objective":        trimObjective,
							"primary_endpoint": trimEndpoint,
						}
						if trimDateInput == "" {
							props["scheduled_date"] = ""
						} else if normDate != "" {
							props["scheduled_date"] = normDate
						}
						if a.applyDesignRaw(map[string]any{
							"type": "update_module", "moduleId": expMod.ID, "properties": props,
						}) {
							if a.designDoc != nil {
								if updatedDoc, parseErr := asterdoc.Parse(a.designDoc.Source); parseErr == nil {
									if updatedExp := findExperimentModule(updatedDoc); updatedExp != nil {
										st.lastSavedBody = updatedExp.Body
									}
								}
							}
						}
					}
				}

				if len(groupTables) > 0 {
					for i := range groupTables {
						m := groupTables[i]
						view.RenderModule(c, m.Kind, m.Schema, m.Data)
					}
				} else {
					ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
						ui.Text(c, "当前方案尚未包含顶层实验分组表，可按需添加一个。").FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
						ui.Spacer(c)
						if components.Button(c, "+ 添加实验分组表", components.ButtonSecondary).Clicked() {
							n := 1
							if doc != nil {
								n = len(doc.Modules) + 1
							}
							_ = a.applyDesignRaw(map[string]any{"type": "add_module", "module": map[string]any{
								"kind": "group_table", "id": fmt.Sprintf("gt-%d", n), "schema": 1, "title": "实验分组表",
								"rows": []any{map[string]any{"id": fmt.Sprintf("g-%d", n), "name": "对照组", "role": "control", "independent_n": 3}},
							}})
						}
					})
				}
			})
		})
	})
}

func renderAsterMLSourceTab(c *ui.Context, a *app, lastValidDoc *asterdoc.Document, srcEd *editor.State) {
	tok := theme.GetTokens(c.Theme())
	bufText := strings.ReplaceAll(srcEd.Buffer.String(), "\r\n", "\n")
	curSource := strings.ReplaceAll(a.designDoc.Source, "\r\n", "\n")
	parsedDoc, parseErr := asterdoc.Parse(bufText)
	dirty := bufText != curSource
	if dirty && parseErr == nil {
		if a.saveAsterDocSourceInPlace(bufText) {
			dirty = false
			srcEd.DiffBase = editor.NewBuffer(a.designDoc.Source)
		}
	}

	editorPane := func() {
		ui.Column(c).Fill().Background(tok.Surface).Children(func() {
			ui.Row(c).FillWidth().Padding(float32(tok.Spacing8), float32(tok.Spacing12)).
				AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
				if parseErr != nil {
					components.Badge(c, "待语法闭合后自动保存", components.BadgeOptions{Tone: components.BadgeWarning})
				} else {
					components.Badge(c, "已实时同步 .asterml", components.BadgeOptions{Tone: components.BadgeSuccess})
				}
				ui.Spacer(c)
				if dirty && parseErr != nil {
					if components.Button(c, "还原", components.ButtonGhost).Clicked() {
						srcEd.Buffer = editor.NewBuffer(a.designDoc.Source)
						if lang := editor.LanguageFor("design.md"); lang != nil {
							srcEd.Buffer.SetLanguage(lang)
						}
						srcEd.DiffBase = editor.NewBuffer(a.designDoc.Source)
					}
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
		if docToShow == nil {
			ui.Column(c).Fill().Center().Padding(float32(tok.Spacing24)).Children(func() {
				ui.Text(c, "修正左侧 AsterML 语法后，此处将实时恢复组件预览。").
					FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
			})
			return
		}
		baseDir := a.designBaseDir()
		previewView := &asterdoc.View{
			Doc:      docToShow,
			Registry: asterdoc.BuiltinRegistry(),
			Mode:     asterdoc.ModeProtocol,
			BaseDir:  baseDir,
			ReadOnly: true,
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
	return a.saveAsterDocSourceWithMode(source, false)
}

func (a *app) saveAsterDocSourceInPlace(source string) bool {
	return a.saveAsterDocSourceWithMode(source, true)
}

func (a *app) saveAsterDocSourceWithMode(source string, inPlace bool) bool {
	if a.designSvc == nil || a.designDoc == nil {
		return false
	}
	previous := *a.designDoc
	expRev := a.designDoc.Revision
	updated, err := a.designSvc.SaveAsterDoc(context.Background(), runs.AsterDocSaveInput{
		DesignID:         a.designDoc.DesignID,
		Source:           source,
		ExpectedRevision: expRev,
		OperationID:      a.nextOp(),
		ActorID:          os.Getenv("USERNAME"),
	})
	if err != nil && isConflictErr(err) {
		if latest, loadErr := a.designSvc.LoadAsterDoc(context.Background(), a.designDoc.DesignID); loadErr == nil {
			updated, err = a.designSvc.SaveAsterDoc(context.Background(), runs.AsterDocSaveInput{
				DesignID:         a.designDoc.DesignID,
				Source:           source,
				ExpectedRevision: latest.Revision,
				OperationID:      a.nextOp(),
				ActorID:          os.Getenv("USERNAME"),
			})
		}
	}
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
	if inPlace {
		a.asterSrcLoadedRev = updated.Revision
		a.asterSrcLoadedSrc = updated.Source
	} else {
		a.asterSrcLoadedRev = 0
	}
	a.designHierarchy = nil
	a.designHierarchySource = ""
	a.syncDesignDiskMirror(updated.DesignID, updated.Source)
	a.refreshReadiness()
	a.syncShellExperiments()
	return true
}

func isConflictErr(err error) bool {
	if err == nil {
		return false
	}
	low := strings.ToLower(err.Error())
	return strings.Contains(low, "conflict") || strings.Contains(low, "expected revision") || strings.Contains(low, "stale")
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
