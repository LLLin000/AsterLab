package main

import (
	"context"
	"fmt"
	"strings"

	"asterlab-mygo-prototype/internal/state"
	"asterlab-mygo-prototype/internal/workflowtree"
	"asterlab-mygo-prototype/modules/runs"
)

// protocolBlockProvider reads immutable ProtocolBlock versions through Core.
// It is read-only and caches only within one app session.
type protocolBlockProvider struct {
	svc   runs.DesignService
	cache map[string]workflowtree.BlockDefinition
}

func (p *protocolBlockProvider) BlockDefinition(versionID string) (workflowtree.BlockDefinition, error) {
	if cached, ok := p.cache[versionID]; ok {
		return cached, nil
	}
	if p.svc == nil {
		return workflowtree.BlockDefinition{}, fmt.Errorf("design service unavailable")
	}
	preview, err := p.svc.PreviewProtocolBlock(context.Background(), versionID)
	if err != nil {
		return workflowtree.BlockDefinition{}, err
	}
	steps := make([]workflowtree.BlockStep, 0, len(preview.Steps))
	for _, step := range preview.Steps {
		parameters := make([]workflowtree.BlockParameter, 0, len(step.Parameters))
		for _, parameter := range step.Parameters {
			parameters = append(parameters, workflowtree.BlockParameter{
				Key: parameter.Key, Label: parameter.Label,
				Value: parameter.Value, Unit: parameter.Unit,
			})
		}
		steps = append(steps, workflowtree.BlockStep{
			UseID: step.UseID, Path: step.Path, Title: step.Title,
			Instruction: step.Instruction, SourceBlockVersionID: step.SourceBlockVersionID,
			Parameters: parameters, UnresolvedParameters: step.UnresolvedParameterRefs,
		})
	}
	definition := workflowtree.BlockDefinition{
		BlockVersionID: preview.BlockVersionID, Ready: preview.Ready, Steps: steps,
	}
	if p.cache == nil {
		p.cache = map[string]workflowtree.BlockDefinition{}
	}
	p.cache[versionID] = definition
	return definition, nil
}

// ensureDesignHierarchy builds the definition/use hierarchy through Core. It
// never needs a Run and never copies block steps into the AsterDoc. Go keeps
// the projection only for rendering; Core owns every identity decision.
func (a *app) ensureDesignHierarchy() *workflowtree.Tree {
	if a.designDoc == nil {
		a.designHierarchy = nil
		a.designHierarchySource = ""
		a.designHierarchyErr = ""
		return nil
	}
	key := fmt.Sprintf("%s|%d|%s", a.designDoc.DesignID, a.designDoc.Revision, a.designDoc.Source)
	if a.designHierarchy != nil && a.designHierarchySource == key {
		return a.designHierarchy
	}
	a.designHierarchy = nil
	a.designHierarchyErr = ""
	if a.designSvc == nil {
		a.designHierarchyErr = "设计服务不可用，无法读取层级"
		return nil
	}
	raw, err := a.designSvc.WorkflowTree(context.Background(), a.designDoc.DesignID, a.designDoc.Revision)
	if err != nil {
		a.designHierarchyErr = state.MapError(err).Error()
		return nil
	}
	tree, err := workflowtree.ParseJSON(raw)
	if err != nil {
		a.designHierarchyErr = err.Error()
		return nil
	}
	a.designHierarchySource = key
	a.designHierarchy = tree
	if len(tree.Issues) > 0 {
		a.designHierarchyErr = strings.Join(tree.Issues, "；")
	}
	return tree
}

func designNodeByID(tree *workflowtree.Tree, id string) *workflowtree.Node {
	if tree == nil || id == "" {
		return nil
	}
	var find func(*workflowtree.Node) *workflowtree.Node
	find = func(node *workflowtree.Node) *workflowtree.Node {
		if node == nil {
			return nil
		}
		if node.ID == id {
			return node
		}
		for _, child := range node.Children {
			if found := find(child); found != nil {
				return found
			}
		}
		return nil
	}
	for _, root := range tree.Roots {
		if found := find(root); found != nil {
			return found
		}
	}
	return nil
}

func designRootNode(tree *workflowtree.Tree, id string) *workflowtree.Node {
	if tree == nil {
		return nil
	}
	for _, root := range tree.Roots {
		if root.ID == id {
			return root
		}
	}
	return nil
}
