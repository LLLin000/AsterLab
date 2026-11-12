package main

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"asterlab-mygo-prototype/internal/asterdoc"
	"asterlab-mygo-prototype/internal/workflowtree"
	"asterlab-mygo-prototype/modules/runs"
)

func TestInspectTreeContractJSON(t *testing.T) {
	tree := &workflowtree.Tree{
		Design: true,
		Roots: []*workflowtree.Node{{
			ID: "step-rna-extract", Path: "root/rna-extraction", Kind: workflowtree.KindStep,
			Identity: workflowtree.IdentityUse, UseID: "root/rna-extraction",
			BlockVersionID: "v1", DefinitionRef: "v1", Composite: true, Total: 1,
			Children: []*workflowtree.Node{{
				ID: "root/rna-extraction/01", Path: "root/rna-extraction/01",
				Identity: workflowtree.IdentityDefinition, Instruction: "加 500 µL",
				Parameters: []workflowtree.BlockParameter{{Key: "volume", Label: "体积", Value: "500", Unit: "µL"}},
			}},
		}},
		Order: []string{"step-rna-extract"}, Precedes: map[string][]string{},
	}
	raw, err := json.Marshal(tree)
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, want := range []string{`"useId":"root/rna-extraction"`, `"instruction":"加 500 µL"`, `"parameters"`, `"design":true`} {
		if !strings.Contains(text, want) {
			t.Fatalf("contract missing %s: %s", want, text)
		}
	}
}

func TestEnrichRunTreeKeepsOccurrenceIdentity(t *testing.T) {
	design := &workflowtree.Tree{Roots: []*workflowtree.Node{{
		Path: "root/rna-extraction", UseID: "root/rna-extraction",
		Children: []*workflowtree.Node{{
			Path: "root/rna-extraction/01", UseID: "root/rna-extraction/01",
			Instruction: "加 500 µL", Parameters: []workflowtree.BlockParameter{{Key: "volume", Value: "500"}},
		}},
	}}}
	run := &workflowtree.Tree{Roots: []*workflowtree.Node{{
		Path: "root/rna-extraction", UseID: "root/rna-extraction",
		Children: []*workflowtree.Node{{
			Path: "root/rna-extraction/01", Identity: workflowtree.IdentityOccurrence,
			TaskKey: "task-1", Status: "completed",
		}},
	}}}
	if err := enrichRunTree(run, design, "run-1"); err != nil {
		t.Fatal(err)
	}
	child := run.Roots[0].Children[0]
	if child.Identity != workflowtree.IdentityOccurrence || child.TaskKey != "task-1" || child.Status != "completed" {
		t.Fatalf("occurrence identity changed: %+v", child)
	}
	if child.Instruction != "加 500 µL" || len(child.Parameters) != 1 || child.UseID != "root/rna-extraction/01" {
		t.Fatalf("definition metadata missing: %+v", child)
	}
}

func TestEnrichRunTreeRejectsAmbiguousIdentity(t *testing.T) {
	design := &workflowtree.Tree{Roots: []*workflowtree.Node{
		{Path: "root/step", AssayID: "assay-a", Instruction: "A"},
		{Path: "root/step", AssayID: "assay-a", Instruction: "B"},
	}}
	run := &workflowtree.Tree{Roots: []*workflowtree.Node{
		{Path: "root/step", AssayID: "assay-a"},
	}}
	if err := enrichRunTree(run, design, "run-1"); err != nil {
		t.Fatal(err)
	}
	if len(run.Roots[0].Issues) == 0 || !strings.Contains(run.Roots[0].Issues[0], "ambiguous") {
		t.Fatalf("ambiguous identity not reported: %+v", run.Roots[0].Issues)
	}
	if run.Roots[0].Instruction != "" {
		t.Fatalf("ambiguous identity copied instruction: %q", run.Roots[0].Instruction)
	}
}

func TestEnrichRunTreeMatchesAssayScope(t *testing.T) {
	design := &workflowtree.Tree{Roots: []*workflowtree.Node{
		{Path: "root/step", AssayID: "assay-a", Instruction: "A"},
		{Path: "root/step", AssayID: "assay-b", Instruction: "B"},
	}}
	run := &workflowtree.Tree{Roots: []*workflowtree.Node{
		{Path: "root/step", AssayID: "assay-b"},
	}}
	if err := enrichRunTree(run, design, "run-1"); err != nil {
		t.Fatal(err)
	}
	if run.Roots[0].Instruction != "B" {
		t.Fatalf("assay scope not used: %q", run.Roots[0].Instruction)
	}
}

func TestInspectTreeRejectsRunDesignMismatch(t *testing.T) {
	dir := rnaFixtureDataDir(t)
	t.Setenv("LABWORKBENCH_DATA_DIR", dir)
	ctx := context.Background()
	ds, err := runs.NewDesignService()
	if err != nil {
		t.Fatal(err)
	}
	defer ds.Close()
	rs, err := runs.NewRunService()
	if err != nil {
		t.Fatal(err)
	}
	defer rs.Close()
	// Run ad9b belongs to d552; query with the RNA extraction design d2d3.
	_, err = inspectTreeFor(ctx, ds, rs, "d2d393a8-9af4-452c-beba-c3687503ffee", 0, "ad9b452e-f8ea-404b-9a2a-7124f9403b02")
	if err == nil || !strings.Contains(err.Error(), "belongs to design") {
		t.Fatalf("mismatch not rejected: %v", err)
	}
}

func TestInspectTreeRunUsesFrozenRevisionNotLatest(t *testing.T) {
	// d2d has AsterDoc rev 1 and latest rev 3; run 357c froze rev 2. A query
	// without an explicit revision must return rev 2's source, never the
	// latest rev 3 source.
	dir := rnaFixtureDataDir(t)
	t.Setenv("LABWORKBENCH_DATA_DIR", dir)
	ctx := context.Background()
	ds, err := runs.NewDesignService()
	if err != nil {
		t.Fatal(err)
	}
	defer ds.Close()
	rs, err := runs.NewRunService()
	if err != nil {
		t.Fatal(err)
	}
	defer rs.Close()
	result, err := inspectTreeFor(ctx, ds, rs, "d2d393a8-9af4-452c-beba-c3687503ffee", 0, "357c2f94-d5bf-4002-951e-f84c73e2d0b7")
	if err != nil {
		t.Fatal(err)
	}
	if result.Revision != 2 {
		t.Fatalf("revision = %d, want frozen 2", result.Revision)
	}
	if len(result.Roots) != 10 {
		t.Fatalf("roots = %d, want 10", len(result.Roots))
	}
}

func TestInspectTreeRejectsWrongRevisionForRun(t *testing.T) {
	dir := rnaFixtureDataDir(t)
	t.Setenv("LABWORKBENCH_DATA_DIR", dir)
	ctx := context.Background()
	ds, err := runs.NewDesignService()
	if err != nil {
		t.Fatal(err)
	}
	defer ds.Close()
	rs, err := runs.NewRunService()
	if err != nil {
		t.Fatal(err)
	}
	defer rs.Close()
	_, err = inspectTreeFor(ctx, ds, rs, "d5525203-56bb-4b52-82c4-0e05f0ed2ff8", 2, "ad9b452e-f8ea-404b-9a2a-7124f9403b02")
	if err == nil || !strings.Contains(err.Error(), "does not match requested revision") {
		t.Fatalf("revision mismatch not rejected: %v", err)
	}
}

func TestInspectTreeRunUsesFrozenRevision(t *testing.T) {
	dir := rnaFixtureDataDir(t)
	t.Setenv("LABWORKBENCH_DATA_DIR", dir)
	ctx := context.Background()
	ds, err := runs.NewDesignService()
	if err != nil {
		t.Fatal(err)
	}
	defer ds.Close()
	rs, err := runs.NewRunService()
	if err != nil {
		t.Fatal(err)
	}
	defer rs.Close()
	result, err := inspectTreeFor(ctx, ds, rs, "d5525203-56bb-4b52-82c4-0e05f0ed2ff8", 0, "ad9b452e-f8ea-404b-9a2a-7124f9403b02")
	if err != nil {
		t.Fatal(err)
	}
	if result.Design {
		t.Fatal("run query marked design")
	}
	if result.Revision != 1 {
		t.Fatalf("revision = %d, want frozen 1", result.Revision)
	}
	for _, root := range result.Roots {
		walkNode(root, func(n *workflowtree.Node) {
			if n.Identity == workflowtree.IdentityOccurrence && n.TaskKey == "" {
				t.Fatalf("occurrence without taskKey: %+v", n)
			}
		})
	}
}

func walkNode(n *workflowtree.Node, fn func(*workflowtree.Node)) {
	if n == nil {
		return
	}
	fn(n)
	for _, c := range n.Children {
		walkNode(c, fn)
	}
}

func TestBuildDesignKeepsTwoUsesOfSameBlockIndependent(t *testing.T) {
	provider := workflowtree.BlockProviderFunc(func(versionID string) (workflowtree.BlockDefinition, error) {
		return workflowtree.BlockDefinition{BlockVersionID: versionID, Ready: true,
			Steps: []workflowtree.BlockStep{
				{UseID: "a", Path: "root/a", Title: "A", SourceBlockVersionID: versionID},
				{UseID: "b", Path: "root/b", Title: "B", SourceBlockVersionID: versionID},
			}}, nil
	})
	source := "# two uses\n\n```asterlab\nkind: experiment\nid: exp\nschema: 1\ntitle: T\n```\n\n" +
		"```asterlab\nkind: group\nid: g\nschema: 1\nname: G\n```\n\n" +
		"```asterlab\nkind: assay\nid: assay\nschema: 1\ntitle: A\n```\n\n" +
		"```asterlab\nkind: reference\nid: ref\nschema: 1\nref: block-v1\nview: protocol_block\n```\n\n" +
		"```asterlab\nkind: step\nid: s1\nschema: 2\ntitle: Use one\npath: root/one\norder: 1\nassay: assay\nuses:\n  - {ref: ref, role: other}\n```\n\n" +
		"```asterlab\nkind: step\nid: s2\nschema: 2\ntitle: Use two\npath: root/two\norder: 2\nassay: assay\nuses:\n  - {ref: ref, role: other}\n```\n\n" +
		"```asterlab\nkind: workflow\nid: wf\nschema: 1\nsteps: [s1, s2]\nbusiness_order: [s1, s2]\n```\n"
	tree, err := workflowtree.BuildDesign(&asterdoc.FrozenAsterDoc{DesignID: "d", Revision: 1, Source: source}, provider)
	if err != nil {
		t.Fatal(err)
	}
	if len(tree.Roots) != 2 {
		t.Fatalf("roots = %d, want 2", len(tree.Roots))
	}
	if tree.Roots[0].UseID == tree.Roots[1].UseID {
		t.Fatalf("uses merged: %q", tree.Roots[0].UseID)
	}
	if tree.Roots[0].Children[0].Path == tree.Roots[1].Children[0].Path {
		t.Fatalf("definition paths merged")
	}
	for _, r := range tree.Roots {
		if r.BlockVersionID != "block-v1" || r.Total != 2 {
			t.Fatalf("use = %+v", r)
		}
	}
}
