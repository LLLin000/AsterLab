package main

import (
	"context"
	"fmt"
	"os"
	"testing"

	"asterlab-mygo-prototype/internal/asterdoc"
	"asterlab-mygo-prototype/internal/workflowtree"
	"asterlab-mygo-prototype/modules/runs"
)

func TestDesignHierarchyWithoutRunFixture(t *testing.T) {
	dir := rnaFixtureDataDir(t)
	t.Setenv("LABWORKBENCH_DATA_DIR", dir)
	svc, err := runs.NewDesignService()
	if err != nil {
		t.Fatal(err)
	}
	defer svc.Close()
	doc, err := svc.LoadAsterDoc(context.Background(), "d5525203-56bb-4b52-82c4-0e05f0ed2ff8")
	if err != nil {
		t.Fatal(err)
	}
	provider := &protocolBlockProvider{svc: svc, cache: map[string]workflowtree.BlockDefinition{}}
	tree, err := workflowtree.BuildDesign(&asterdoc.FrozenAsterDoc{
		DesignID: doc.DesignID, Revision: doc.Revision, Source: doc.Source,
	}, provider)
	if err != nil {
		t.Fatal(err)
	}
	if err := tree.Validate(); err != nil {
		t.Fatal(err)
	}
	if !tree.Design || len(tree.Roots) != 4 {
		t.Fatalf("design tree roots = %d design=%v", len(tree.Roots), tree.Design)
	}
	rna := tree.Roots[0]
	if !rna.Composite || rna.Total != 10 || len(rna.Children) != 10 {
		t.Fatalf("RNA hierarchy = %+v", rna)
	}
	if rna.Children[0].Identity != workflowtree.IdentityDefinition {
		t.Fatalf("child identity = %q", rna.Children[0].Identity)
	}
}

func TestDesignHierarchyUsesNativeCore(t *testing.T) {
	dir := rnaFixtureDataDir(t)
	t.Setenv("LABWORKBENCH_DATA_DIR", dir)
	svc, err := runs.NewDesignService()
	if err != nil {
		t.Fatal(err)
	}
	defer svc.Close()
	doc, err := svc.LoadAsterDoc(context.Background(), "d5525203-56bb-4b52-82c4-0e05f0ed2ff8")
	if err != nil {
		t.Fatal(err)
	}
	a := &app{designSvc: svc, designDoc: &doc}
	tree := a.ensureDesignHierarchy()
	if tree == nil || a.designHierarchyErr != "" {
		t.Fatalf("native hierarchy failed: %v", a.designHierarchyErr)
	}
	if !tree.Design || len(tree.Roots) != 4 || !tree.Roots[0].Composite {
		t.Fatalf("native hierarchy = design:%v roots:%d", tree.Design, len(tree.Roots))
	}
}

func TestCanvasStepsCarryCompositeMarker(t *testing.T) {
	raw, err := os.ReadFile("../../docs/sdk/rna-extraction-case/rtqpcr-parent-path-bound.asterml")
	if err != nil {
		t.Fatal(err)
	}
	doc, err := asterdoc.Parse(string(raw))
	if err != nil {
		t.Fatal(err)
	}
	steps := make([]workflowtree.BlockStep, 0, 10)
	for i := 1; i <= 10; i++ {
		steps = append(steps, workflowtree.BlockStep{
			UseID: "op", Path: "root/" + fmt.Sprintf("%02d-op", i),
			Title: "操作", SourceBlockVersionID: "c23df8e1-3f2a-4449-8ac9-a4bb7256a113",
		})
	}
	provider := workflowtree.BlockProviderFunc(func(string) (workflowtree.BlockDefinition, error) {
		return workflowtree.BlockDefinition{
			BlockVersionID: "c23df8e1-3f2a-4449-8ac9-a4bb7256a113",
			Ready:          true,
			Steps:          steps,
		}, nil
	})
	tree, err := workflowtree.BuildDesign(&asterdoc.FrozenAsterDoc{DesignID: "d", Revision: 1, Source: string(raw)}, provider)
	if err != nil {
		t.Fatal(err)
	}
	var rna *canvasStepInfo
	for _, step := range canvasSteps(doc, tree) {
		if step.id == "step-rna-extract" {
			step := step
			rna = &step
		}
	}
	if rna == nil || !rna.composite || rna.childCount != 10 {
		t.Fatalf("composite marker = %+v", rna)
	}
}

func TestDefinitionNodeParamsStatus(t *testing.T) {
	params, status := definitionNodeParams(&workflowtree.Node{
		Parameters: []workflowtree.BlockParameter{{Label: "体积", Value: "500", Unit: "µL"}},
	})
	if len(params) != 1 || params[0].Val != "500 µL" || status != "" {
		t.Fatalf("resolved = %+v %q", params, status)
	}
	params, status = definitionNodeParams(&workflowtree.Node{
		UnresolvedParameters: []string{"volume"},
	})
	if len(params) != 0 || status != "1 个参数未解析" {
		t.Fatalf("unresolved = %+v %q", params, status)
	}
	params, status = definitionNodeParams(&workflowtree.Node{})
	if len(params) != 0 || status != "无参数" {
		t.Fatalf("empty = %+v %q", params, status)
	}
}
