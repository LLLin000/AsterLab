package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"asterlab-mygo-prototype/internal/asterdoc"
	"asterlab-mygo-prototype/internal/runworkspace"
	"asterlab-mygo-prototype/internal/workflowtree"
	"asterlab-mygo-prototype/modules/runs"
)

// TestPathBoundRunTreeProjection reads the committed deterministic RT-qPCR Run
// fixture. The fixture is copied to a temporary data directory so the test
// never depends on an untracked local acceptance database and never mutates
// the committed file.
func rnaFixtureDataDir(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "rna-case", "labworkbench.sqlite"))
	if err != nil {
		t.Fatalf("path-bound fixture missing: %v", err)
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "labworkbench.sqlite"), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestPathBoundRunTreeProjection(t *testing.T) {
	dir := rnaFixtureDataDir(t)
	t.Setenv("LABWORKBENCH_DATA_DIR", dir)
	ctx := context.Background()
	runSvc, err := runs.NewRunService()
	if err != nil {
		t.Fatal(err)
	}
	defer runSvc.Close()
	designSvc, err := runs.NewDesignService()
	if err != nil {
		t.Fatal(err)
	}
	defer designSvc.Close()

	snap, err := runSvc.LoadRun(ctx, "ad9b452e-f8ea-404b-9a2a-7124f9403b02")
	if err != nil {
		t.Fatal(err)
	}
	if snap.AsterDoc == nil || snap.AsterDoc.SourceHash == "" {
		t.Fatal("run has no frozen AsterDoc ref")
	}
	frozen, err := asterdoc.LoadFrozenAsterDocAt(ctx, designSvc, snap.AsterDoc.DesignID, snap.AsterDoc.Revision, snap.AsterDoc.SourceHash)
	if err != nil {
		t.Fatal(err)
	}
	vm := runworkspace.BuildViewModel(snap, &frozen)
	if vm.Tree == nil {
		t.Fatalf("tree not projected; issues=%v", vm.TreeIssues)
	}
	if len(vm.Tree.Roots) != 4 {
		t.Fatalf("roots = %d, want 4", len(vm.Tree.Roots))
	}
	rna := vm.Tree.Roots[0]
	if rna.Title != "01 RNA 提取" || !rna.Composite || rna.Total != 10 || rna.Completed != 5 || rna.Status != "current" {
		t.Fatalf("RNA root = %+v", rna)
	}
	if rna.Identity != workflowtree.IdentityUse || rna.Children[0].Identity != workflowtree.IdentityOccurrence {
		t.Fatalf("identity = %q / %q", rna.Identity, rna.Children[0].Identity)
	}
	if len(rna.Children) != 10 {
		t.Fatalf("RNA children = %d, want 10", len(rna.Children))
	}
	if rna.Children[0].Title != "01 收集细胞" {
		t.Fatalf("first RNA child = %+v", rna.Children[0])
	}
	if vm.Tree.Roots[1].Status != "completed" || vm.Tree.Roots[2].Status != "pending" || vm.Tree.Roots[3].Status != "blocked" {
		t.Fatalf("leaf statuses = %q %q %q", vm.Tree.Roots[1].Status, vm.Tree.Roots[2].Status, vm.Tree.Roots[3].Status)
	}

	// Every frozen task belongs to exactly one node. The workflowtree package
	// owns the recursive uniqueness tests; here the real run is checked for
	// duplicate child IDs.
	seen := map[string]int{}
	for _, child := range rna.Children {
		seen[child.ID]++
	}
	if len(seen) != 10 {
		t.Fatalf("RNA child IDs = %d, want 10", len(seen))
	}
	for _, task := range snap.Tasks {
		if seen[task.Key] > 1 {
			t.Fatalf("task %s assigned more than once", task.Key)
		}
		if strings.HasPrefix(task.Path, "root/rna-extraction/") && task.SourceBlockVersionID != "c23df8e1-3f2a-4449-8ac9-a4bb7256a113" {
			t.Fatalf("task %s source block = %q", task.Path, task.SourceBlockVersionID)
		}
	}
}
