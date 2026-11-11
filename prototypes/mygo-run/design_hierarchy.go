package main

import (
	"context"
	"fmt"
	"strings"

	"asterlab-mygo-prototype/internal/asterdoc"
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
		steps = append(steps, workflowtree.BlockStep{
			UseID: step.UseID, Path: step.Path, Title: step.Title,
			Instruction: step.Instruction, SourceBlockVersionID: step.SourceBlockVersionID,
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

// ensureDesignHierarchy builds the definition/use hierarchy from the current
// AsterDoc and referenced immutable ProtocolBlock versions. It never needs a
// Run and never copies block steps into the AsterDoc.
func (a *app) ensureDesignHierarchy() *workflowtree.Tree {
	if a.designDoc == nil {
		a.designHierarchy = nil
		a.designHierarchySource = ""
		a.designHierarchyErr = ""
		return nil
	}
	if a.designHierarchy != nil && a.designHierarchySource == a.designDoc.Source {
		return a.designHierarchy
	}
	provider := &protocolBlockProvider{svc: a.designSvc, cache: a.blockPreviewCache}
	tree, err := workflowtree.BuildDesign(&asterdoc.FrozenAsterDoc{
		DesignID: a.designDoc.DesignID, Revision: a.designDoc.Revision, Source: a.designDoc.Source,
	}, provider)
	a.blockPreviewCache = provider.cache
	a.designHierarchySource = a.designDoc.Source
	a.designHierarchy = tree
	a.designHierarchyErr = ""
	if err != nil {
		a.designHierarchyErr = err.Error()
	} else if tree != nil && len(tree.Issues) > 0 {
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
