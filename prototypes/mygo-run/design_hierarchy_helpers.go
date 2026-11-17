package main

import (
	"fmt"
	"strings"

	"asterlab-mygo-prototype/internal/asterdoc"
	"asterlab-mygo-prototype/internal/workflow"
	"asterlab-mygo-prototype/internal/workflowtree"

	"github.com/egoist/mygo/ui"
)

func designNarrow(c *ui.Context) bool {
	bounds := c.Root().LayoutBounds()
	return bounds.W > 0 && bounds.W < 960
}

type canvasStepInfo struct {
	id, title, assay, path, timepoints string
	composite                          bool
	childCount                         int
}

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

func canvasSteps(doc *asterdoc.Document, hierarchy *workflowtree.Tree) []canvasStepInfo {
	out := []canvasStepInfo{}
	for _, module := range doc.Modules {
		if module.Kind != "step" {
			continue
		}
		step := canvasStepInfo{
			id: module.ID, title: moduleString(module.Data, "title"),
			assay: moduleString(module.Data, "assay"), path: moduleString(module.Data, "path"),
			timepoints: strings.Join(moduleStrings(module.Data, "timepoints"), ", "),
		}
		if root := designRootNode(hierarchy, module.ID); root != nil {
			step.composite = root.Composite
			step.childCount = root.Total
		}
		out = append(out, step)
	}
	return out
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

// enterDesignSubflow builds a read-only canvas from definition children when
// inspecting nested composite protocol blocks.
func (a *app) enterDesignSubflow(parentID string) {
	tree := a.ensureDesignHierarchy()
	node := designRootNode(tree, parentID)
	if node == nil || !node.Composite || len(node.Children) == 0 {
		return
	}
	ed := workflow.NewEditor()
	ed.ExternalWrites = true
	ed.ReadOnly = true
	ed.Zoom = 0.8
	ed.OnSelect = func(id string) { a.designSubflowSelected = id }
	previous := ""
	for index, child := range node.Children {
		row := index / 3
		column := index % 3
		if row%2 == 1 {
			column = 2 - column
		}
		pos := workflow.Position{
			X: 60 + float32(column)*250,
			Y: 60 + float32(row)*150,
		}
		params, paramStatus := definitionNodeParams(child)
		ed.Doc.Nodes = append(ed.Doc.Nodes, workflow.Node{
			ID: child.ID, Kind: workflow.KindPipette, Title: child.Title, Pos: pos,
			Composite: child.Composite, ChildCount: child.Total,
			Instruction: child.Instruction, Params: params, ParamStatus: paramStatus,
		})
		ed.Doc.BusinessOrder = append(ed.Doc.BusinessOrder, child.ID)
		if previous != "" {
			ed.Sequence = append(ed.Sequence, workflow.Edge{
				ID: fmt.Sprintf("reading-%d", len(ed.Sequence)+1), From: previous, To: child.ID,
			})
		}
		previous = child.ID
	}
	a.designSubflow = ed
	a.designSubflowParent = parentID
	a.designSubflowTitle = node.Title
	a.designSubflowSelected = ""
	a.designSubflowFitted = false
	a.saveSession()
}

func (a *app) leaveDesignSubflow() {
	a.designSubflow = nil
	a.designSubflowParent = ""
	a.designSubflowTitle = ""
	a.designSubflowSelected = ""
	a.designSubflowFitted = false
	a.saveSession()
}

func definitionNodeParams(node *workflowtree.Node) ([]workflow.NodeParam, string) {
	params := make([]workflow.NodeParam, 0, len(node.Parameters))
	for _, parameter := range node.Parameters {
		value := parameter.Value
		if parameter.Unit != "" {
			value += " " + parameter.Unit
		}
		params = append(params, workflow.NodeParam{Key: parameter.Label, Val: value})
	}
	switch {
	case len(params) > 0:
		return params, ""
	case len(node.UnresolvedParameters) > 0:
		return nil, fmt.Sprintf("%d 个参数未解析", len(node.UnresolvedParameters))
	default:
		return nil, "无参数"
	}
}
