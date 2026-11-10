package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/onevra/source/components"
	"github.com/oneo-me/onevra/source/icons/lucide"

	"asterlab-mygo-prototype/internal/asterdoc"
	"asterlab-mygo-prototype/internal/workflow"
)

// renderWorkflowCanvas is the doc-backed professional Workflow editor. The
// canvas is built from AsterDoc step/workflow modules; add/delete/connect and
// inspector updates go through Core commands, never a second writable graph.
func (a *app) renderWorkflowCanvas(c *ui.Context) {
	t := c.Theme()
	doc, err := asterdoc.Parse(a.designDoc.Source)
	if err != nil {
		ui.Text(c, "AsterDoc 语法错误："+err.Error()).Padding(t.Space(6)).TextColor(t.Danger)
		return
	}
	a.ensureDesignCanvas(doc)
	if a.designCanvas == nil {
		return
	}
	ui.Column(c).Grow(1).FillWidth().Children(func() {
		ui.Row(c).FillWidth().Padding(t.Space(2), t.Space(3)).AlignItems(ui.Center).
			Gap(t.Space(2)).Children(func() {
			ui.Row(c).Padding(t.Space(.75), t.Space(1)).Radius(t.Space(1.5)).
				Background(t.Surface).Border(1, t.Border).
				Shadow(0, 2, 14, 0, t.Text.Alpha(.08)).Gap(t.Space(.5)).
				AlignItems(ui.Center).Children(func() {
				if components.IconButton(c, lucide.Boxes(), "添加模块", components.IconButtonPlain).Clicked() {
					a.addModuleOpen = true
				}
				if components.IconButton(c, lucide.Plus(), "添加步骤", components.IconButtonPlain).Clicked() {
					a.canvasAddStep(doc)
				}
				del := components.IconButton(c, lucide.Trash(), "删除所选步骤", components.IconButtonPlain)
				del.Disabled(a.designSelected == "")
				if del.Clicked() {
					a.confirmDeleteStep = a.designSelected
				}
				ui.Box(c).Width(1).Height(18).Background(t.Border)
				connect := components.IconButton(c, lucide.Link2(), "连接步骤", components.IconButtonPlain)
				if a.designCanvas.ConnectMode {
					connect.Background(t.Accent.Alpha(.14)).TextColor(t.Accent)
				}
				if connect.Clicked() {
					a.designCanvas.ConnectMode = !a.designCanvas.ConnectMode
					if !a.designCanvas.ConnectMode {
						a.designCanvas.ConnectFrom = ""
					}
				}
				if components.IconButton(c, lucide.Maximize2(), "适应画布", components.IconButtonPlain).Clicked() {
					a.designCanvas.PanX, a.designCanvas.PanY, a.designCanvas.Zoom = 0, 0, 1
				}
				zoom := a.designCanvas.Zoom
				if zoom <= 0 {
					zoom = 1
				}
				components.Text(c, fmt.Sprintf("%.0f%%", zoom*100)).FontSize(t.Rem(.75)).
					TextColor(t.TextMuted).FontFeatures("tnum")
			})
			ui.Spacer(c)
			if a.designDocErr != "" {
				ui.Text(c, a.designDocErr).FontSize(t.Rem(.78)).TextColor(t.Danger).
					SingleLine().Ellipsis("…").MaxWidth(320)
			}
		})
		ui.Row(c).Grow(1).FillWidth().MinHeight(0).AlignItems(ui.Stretch).Children(func() {
			ui.Box(c).Grow(1).MinWidth(0).MinHeight(0).Fill().Children(func() {
				a.designCanvas.View(c)
			})
			if a.designSelected != "" {
				a.canvasInspector(c, doc)
			}
		})
	})
	a.deleteStepDialog(c, doc)
	a.renderAddModuleDialog(c, doc)
}

func (a *app) deleteStepDialog(c *ui.Context, doc *asterdoc.Document) {
	if a.confirmDeleteStep == "" {
		return
	}
	id := a.confirmDeleteStep
	edges := 0
	tasks := 0
	checks := 0
	for _, module := range doc.Modules {
		if module.Kind == "step" && module.ID == id {
			if items, ok := module.Data["tasks"].([]any); ok {
				tasks = len(items)
			}
			if items, ok := module.Data["checks"].([]any); ok {
				checks = len(items)
			}
		}
		if module.Kind == "workflow" {
			if items, ok := module.Data["edges"].([]any); ok {
				for _, raw := range items {
					edge, _ := raw.(map[string]any)
					if edge["from"] == id || edge["to"] == id {
						edges++
					}
				}
			}
		}
	}
	open := a.confirmDeleteStep != ""
	components.Dialog(c, &open, components.DialogOptions{Label: "删除步骤"}, func(panel ui.Element) {
		panel.Width(460).Children(func() {
			ui.Text(c, "删除这个步骤？").FontSize(15).Bold()
			ui.Text(c, fmt.Sprintf("该步骤有 %d 条连接、%d 个任务、%d 个检查项。删除会同时移除相关连接。", edges, tasks, checks)).
				FontSize(12).TextColor(c.Theme().TextMuted)
			ui.Row(c).FillWidth().Wrap().Justify(ui.End).Gap(8).Children(func() {
				if components.Button(c, "取消", components.ButtonSecondary).Clicked() {
					a.confirmDeleteStep = ""
				}
				if components.Button(c, "删除步骤", components.ButtonPrimary).Clicked() {
					a.confirmDeleteStep = ""
					a.canvasDeleteSelected()
				}
			})
		})
	})
	if !open {
		a.confirmDeleteStep = ""
	}
}

func (a *app) ensureDesignCanvas(doc *asterdoc.Document) {
	if a.designCanvas != nil && a.designCanvasSource == a.designDoc.Source {
		return
	}
	ed := workflow.NewEditor()
	ed.ExternalWrites = true
	ed.OnSelect = func(id string) {
		a.designSelected = id
		a.canvasSelLoaded = ""
	}
	ed.BeforeDelete = func(string) bool { return false }
	ed.OnChange = func() { a.syncCanvasEdges(ed) }
	positions := a.designNodePos
	if positions == nil {
		positions = map[string]workflow.Position{}
	}
	steps := canvasSteps(doc)
	for index, step := range steps {
		pos, ok := positions[step.id]
		if !ok {
			pos = workflow.Position{X: 80 + float32(index%4)*240, Y: 80 + float32(index/4)*140}
		}
		ed.Doc.Nodes = append(ed.Doc.Nodes, workflow.Node{
			ID: step.id, Kind: workflow.KindFlask, Title: step.title, Pos: pos,
		})
		ed.Doc.BusinessOrder = append(ed.Doc.BusinessOrder, step.id)
	}
	for _, edge := range canvasEdges(doc) {
		ed.Doc.Edges = append(ed.Doc.Edges, workflow.Edge{ID: edge.id, From: edge.from, To: edge.to})
	}
	if old := a.designCanvas; old != nil {
		ed.PanX, ed.PanY, ed.Zoom = old.PanX, old.PanY, old.Zoom
	}
	a.designCanvas = ed
	a.designCanvasSource = a.designDoc.Source
}

type canvasStepInfo struct{ id, title, assay, path, timepoints string }

type canvasTaskDraft struct {
	ID, Title, Kind, Readout string
}

type canvasCheckDraft struct {
	ID, Title string
	Blocking  bool
}

type canvasUseDraft struct {
	Ref, Role string
}
type canvasEdgeInfo struct{ id, from, to string }

func canvasSteps(doc *asterdoc.Document) []canvasStepInfo {
	out := []canvasStepInfo{}
	for _, module := range doc.Modules {
		if module.Kind != "step" {
			continue
		}
		out = append(out, canvasStepInfo{
			id: module.ID, title: moduleString(module.Data, "title"),
			assay: moduleString(module.Data, "assay"), path: moduleString(module.Data, "path"),
			timepoints: strings.Join(moduleStrings(module.Data, "timepoints"), ", "),
		})
	}
	return out
}

func canvasEdges(doc *asterdoc.Document) []canvasEdgeInfo {
	out := []canvasEdgeInfo{}
	for _, module := range doc.Modules {
		if module.Kind != "workflow" {
			continue
		}
		edges, _ := module.Data["edges"].([]any)
		for index, raw := range edges {
			edge, _ := raw.(map[string]any)
			id, _ := edge["id"].(string)
			if id == "" {
				id = fmt.Sprintf("e%d", index+1)
			}
			from, _ := edge["from"].(string)
			to, _ := edge["to"].(string)
			out = append(out, canvasEdgeInfo{id: id, from: from, to: to})
		}
	}
	return out
}

func workflowModuleID(doc *asterdoc.Document) string {
	for _, module := range doc.Modules {
		if module.Kind == "workflow" {
			return module.ID
		}
	}
	return ""
}

func firstAssayID(doc *asterdoc.Document) string {
	for _, module := range doc.Modules {
		if module.Kind == "assay" {
			return module.ID
		}
	}
	return ""
}

func moduleString(data map[string]any, key string) string {
	value, _ := data[key].(string)
	return strings.TrimSpace(value)
}

func moduleStrings(data map[string]any, key string) []string {
	items, _ := data[key].([]any)
	out := make([]string, 0, len(items))
	for _, item := range items {
		if value, ok := item.(string); ok {
			out = append(out, value)
		}
	}
	return out
}

func (a *app) canvasAddStep(doc *asterdoc.Document) {
	assay := firstAssayID(doc)
	if assay == "" {
		a.designDocErr = "文档没有 assay，不能添加步骤"
		return
	}
	id := fmt.Sprintf("step-%d", time.Now().UnixNano())
	module := map[string]any{
		"kind": "step", "id": id, "schema": 1, "title": "新步骤",
		"assay": assay, "timepoints": []any{}, "path": "",
	}
	command := map[string]any{"type": "add_step", "module": module}
	if wf := workflowModuleID(doc); wf != "" {
		command["workflowId"] = wf
	}
	if a.applyDesignRaw(command) {
		a.designSelected = id
		a.canvasSelLoaded = ""
	}
}

func (a *app) canvasDeleteSelected() {
	if a.designSelected == "" {
		return
	}
	if a.applyDesignRaw(map[string]any{"type": "remove_step", "moduleId": a.designSelected}) {
		a.designSelected = ""
		a.canvasSelLoaded = ""
	}
}

func (a *app) syncCanvasEdges(ed *workflow.Editor) {
	if a.canvasSyncing || a.designDoc == nil {
		return
	}
	doc, err := asterdoc.Parse(a.designDoc.Source)
	if err != nil {
		return
	}
	existing := map[string]bool{}
	for _, edge := range canvasEdges(doc) {
		existing[edge.from+"\x00"+edge.to] = true
	}
	type pair struct{ from, to string }
	newEdges := []pair{}
	for _, edge := range ed.Doc.Edges {
		if !existing[edge.From+"\x00"+edge.To] {
			newEdges = append(newEdges, pair{edge.From, edge.To})
		}
	}
	// Node moves are layout-only; remember them for the next rebuild.
	a.designNodePos = map[string]workflow.Position{}
	for _, node := range ed.Doc.Nodes {
		a.designNodePos[node.ID] = node.Pos
	}
	if len(newEdges) == 0 {
		return
	}
	a.canvasSyncing = true
	defer func() { a.canvasSyncing = false }()
	wf := workflowModuleID(doc)
	for _, edge := range newEdges {
		ok := a.applyDesignRaw(map[string]any{
			"type": "connect_steps", "moduleId": wf, "from": edge.from, "to": edge.to,
		})
		if !ok {
			return
		}
	}
}

func (a *app) canvasInspector(c *ui.Context, doc *asterdoc.Document) {
	t := c.Theme()
	var selected *asterdoc.Module
	for i := range doc.Modules {
		if doc.Modules[i].Kind == "step" && doc.Modules[i].ID == a.designSelected {
			selected = &doc.Modules[i]
		}
	}
	if selected == nil {
		return
	}
	if a.canvasSelLoaded != a.designSelected {
		a.canvasSelLoaded = a.designSelected
		a.canvasTitle = moduleString(selected.Data, "title")
		a.canvasAssay = moduleString(selected.Data, "assay")
		a.canvasPath = moduleString(selected.Data, "path")
		a.canvasTimepoints = strings.Join(moduleStrings(selected.Data, "timepoints"), ", ")
		a.canvasTasks = nil
		if tasks, ok := selected.Data["tasks"].([]any); ok {
			for _, raw := range tasks {
				task, _ := raw.(map[string]any)
				a.canvasTasks = append(a.canvasTasks, canvasTaskDraft{
					ID: moduleString(task, "id"), Title: moduleString(task, "title"),
					Kind: moduleString(task, "kind"), Readout: moduleString(task, "readout"),
				})
			}
		}
		a.canvasUses = nil
		if uses, ok := selected.Data["uses"].([]any); ok {
			for _, raw := range uses {
				use, _ := raw.(map[string]any)
				a.canvasUses = append(a.canvasUses, canvasUseDraft{
					Ref: moduleString(use, "ref"), Role: moduleString(use, "role"),
				})
			}
		}
		a.canvasUsePick = ""
		a.canvasChecks = nil
		if checks, ok := selected.Data["checks"].([]any); ok {
			for _, raw := range checks {
				check, _ := raw.(map[string]any)
				blocking, _ := check["blocking"].(bool)
				a.canvasChecks = append(a.canvasChecks, canvasCheckDraft{
					ID: moduleString(check, "checkId"), Title: moduleString(check, "title"),
					Blocking: blocking,
				})
			}
		}
	}
	ui.Column(c).Width(t.Space(38)).Shrink(0).FillWidth().Padding(t.Space(3)).
		Gap(t.Space(1.5)).Background(t.Surface).Border(1, t.Border).Children(func() {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
			ui.Text(c, "步骤属性").FontSize(t.Rem(.95)).Bold().TextColor(t.Text)
			ui.Spacer(c)
			if components.IconButton(c, lucide.Save(), "保存步骤属性", components.IconButtonCompact).Clicked() {
				timepoints := []any{}
				for _, point := range strings.Split(a.canvasTimepoints, ",") {
					if point = strings.TrimSpace(point); point != "" {
						timepoints = append(timepoints, point)
					}
				}
				tasks := make([]any, 0, len(a.canvasTasks))
				for _, task := range a.canvasTasks {
					tasks = append(tasks, map[string]any{
						"id": task.ID, "title": task.Title, "kind": task.Kind, "readout": task.Readout,
					})
				}
				checks := make([]any, 0, len(a.canvasChecks))
				for _, check := range a.canvasChecks {
					checks = append(checks, map[string]any{
						"checkId": check.ID, "title": check.Title, "blocking": check.Blocking,
					})
				}
				uses := make([]any, 0, len(a.canvasUses))
				for _, use := range a.canvasUses {
					uses = append(uses, map[string]any{"ref": use.Ref, "role": use.Role})
				}
				schema := selected.Schema
				if len(uses) > 0 {
					schema = 2
				}
				_ = a.applyDesignRaw(map[string]any{
					"type": "update_module", "moduleId": a.designSelected,
					"properties": map[string]any{
						"schema": schema,
						"title":  a.canvasTitle, "assay": a.canvasAssay,
						"path": a.canvasPath, "timepoints": timepoints,
						"tasks": tasks, "checks": checks, "uses": uses,
					},
				})
				a.canvasSelLoaded = ""
			}
		})
		field := func(label string, value *string) {
			ui.Text(c, label).FontSize(t.Rem(.7)).TextColor(t.TextMuted)
			ui.TextInput(c, value).FillWidth()
		}
		field("标题", &a.canvasTitle)
		ui.Text(c, "检测").FontSize(t.Rem(.7)).TextColor(t.TextMuted)
		ui.Row(c).Wrap().Gap(t.Space(.75)).Children(func() {
			for _, assay := range doc.Modules {
				if assay.Kind != "assay" {
					continue
				}
				label := moduleString(assay.Data, "title")
				if label == "" {
					label = assay.ID
				}
				active := assay.ID == a.canvasAssay
				btn := ui.ButtonBase(c).Padding(3, 8).Radius(t.Space(1)).
					Background(map[bool]ui.Color{true: t.Accent.Alpha(.16), false: t.Background}[active]).
					OnClick(func() { a.canvasAssay = assay.ID })
				btn.Children(func() {
					ui.Text(c, label).FontSize(t.Rem(.72)).TextColor(t.Text)
				})
			}
		})
		// Protocol belongs to the assay, not the step. Selecting one updates
		// the assay module so readiness can become executable.
		var assayModule *asterdoc.Module
		for i := range doc.Modules {
			if doc.Modules[i].Kind == "assay" && doc.Modules[i].ID == a.canvasAssay {
				assayModule = &doc.Modules[i]
			}
		}
		ui.Text(c, "Protocol").FontSize(t.Rem(.7)).TextColor(t.TextMuted)
		ui.Row(c).Wrap().Gap(t.Space(.75)).Children(func() {
			for _, protocol := range a.protocols {
				protocol := protocol
				active := assayModule != nil && moduleString(assayModule.Data, "protocol_version_id") == protocol.ProtocolVersionID
				label := fmt.Sprintf("%s v%d", protocol.Title, protocol.Version)
				btn := ui.ButtonBase(c).Padding(3, 8).Radius(t.Space(1)).
					Background(map[bool]ui.Color{true: t.Accent.Alpha(.16), false: t.Background}[active]).
					OnClick(func() {
						if a.canvasAssay == "" {
							return
						}
						_ = a.applyDesignRaw(map[string]any{
							"type": "update_module", "moduleId": a.canvasAssay,
							"properties": map[string]any{"protocol_version_id": protocol.ProtocolVersionID},
						})
						a.canvasSelLoaded = ""
					})
				btn.Children(func() {
					ui.Text(c, label).FontSize(t.Rem(.7)).TextColor(t.Text)
				})
			}
			if len(a.protocols) == 0 {
				ui.Text(c, "尚无 protocol 版本").FontSize(t.Rem(.72)).TextColor(t.TextMuted)
			}
		})
		field("Protocol path", &a.canvasPath)
		field("时间点", &a.canvasTimepoints)
		ui.Divider(c)
		ui.Text(c, "步骤资源").FontSize(t.Rem(.75)).Bold().TextColor(t.Text)
		roles := []string{"instrument", "calculator", "labware", "material", "sample", "other"}
		for i := range a.canvasUses {
			i := i
			use := &a.canvasUses[i]
			ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(t.Space(.5)).Children(func() {
				ui.Text(c, use.Ref).FontSize(t.Rem(.72)).TextColor(t.Text).Width(180)
				ui.Select(c, &use.Role, roles).FillWidth()
				if components.IconButton(c, lucide.X(), "移除绑定", components.IconButtonCompact).Clicked() {
					a.canvasUses = append(a.canvasUses[:i], a.canvasUses[i+1:]...)
				}
			})
		}
		ui.Row(c).FillWidth().Wrap().Gap(t.Space(.5)).Children(func() {
			for _, module := range doc.Modules {
				switch module.Kind {
				case "step", "experiment", "group", "assay", "workflow", "plate", "plate96", "reference":
					continue
				}
				module := module
				btn := ui.ButtonBase(c).Padding(3, 8).Radius(t.Space(1)).
					Background(t.Background).Border(1, t.Border)
				btn.Children(func() {
					ui.Text(c, "+ "+module.ID).FontSize(t.Rem(.7)).TextColor(t.Text)
				})
				if btn.Clicked() {
					role := "other"
					if module.Kind == "instrument_profile" {
						role = "instrument"
					} else if module.Kind == "dilution" {
						role = "calculator"
					}
					exists := false
					for _, existing := range a.canvasUses {
						if existing.Ref == module.ID {
							exists = true
						}
					}
					if !exists {
						a.canvasUses = append(a.canvasUses, canvasUseDraft{Ref: module.ID, Role: role})
					}
				}
			}
		})
		ui.Divider(c)
		ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
			ui.Text(c, "执行任务").FontSize(t.Rem(.75)).Bold().TextColor(t.Text)
			ui.Spacer(c)
			if components.IconButton(c, lucide.Plus(), "添加任务", components.IconButtonCompact).Clicked() {
				a.canvasTasks = append(a.canvasTasks, canvasTaskDraft{
					ID: fmt.Sprintf("task-%d", len(a.canvasTasks)+1), Title: "新任务", Kind: "measurement",
				})
			}
		})
		for i := range a.canvasTasks {
			i := i
			ui.Column(c).Gap(2).Padding(5, 6).Radius(t.Space(1)).
				Background(t.Background).Border(1, t.Border).Children(func() {
				ui.Row(c).Gap(4).Children(func() {
					ui.TextInput(c, &a.canvasTasks[i].Title).Placeholder("任务标题").FillWidth()
					if components.IconButton(c, lucide.X(), "删除任务", components.IconButtonCompact).Clicked() {
						a.canvasTasks = append(a.canvasTasks[:i], a.canvasTasks[i+1:]...)
					}
				})
				ui.Row(c).Gap(4).Children(func() {
					ui.TextInput(c, &a.canvasTasks[i].Kind).Placeholder("类型").FillWidth()
					ui.TextInput(c, &a.canvasTasks[i].Readout).Placeholder("Readout").FillWidth()
				})
			})
		}
		ui.Divider(c)
		ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
			ui.Text(c, "检查项").FontSize(t.Rem(.75)).Bold().TextColor(t.Text)
			ui.Spacer(c)
			if components.IconButton(c, lucide.Plus(), "添加检查项", components.IconButtonCompact).Clicked() {
				a.canvasChecks = append(a.canvasChecks, canvasCheckDraft{
					ID: fmt.Sprintf("check-%d", len(a.canvasChecks)+1), Title: "新检查项", Blocking: true,
				})
			}
		})
		for i := range a.canvasChecks {
			i := i
			ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
				components.Checkbox(c, &a.canvasChecks[i].Blocking, "")
				ui.TextInput(c, &a.canvasChecks[i].Title).Placeholder("检查项").FillWidth()
				if components.IconButton(c, lucide.X(), "删除检查项", components.IconButtonCompact).Clicked() {
					a.canvasChecks = append(a.canvasChecks[:i], a.canvasChecks[i+1:]...)
				}
			})
		}
		ui.Text(c, "拖动只改变布局；连接与属性修改写入 AsterDoc。").
			FontSize(t.Rem(.68)).TextColor(t.TextMuted)
	})
}
