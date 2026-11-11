package main

import (
	"context"
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
