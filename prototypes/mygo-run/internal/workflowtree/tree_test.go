package workflowtree

import (
	"os"
	"strings"
	"testing"

	"asterlab-mygo-prototype/internal/asterdoc"
	"asterlab-mygo-prototype/modules/runs"
)

func parentSourceText(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile("../../../../docs/sdk/rna-extraction-case/rtqpcr-parent-path-bound.asterml")
	if err != nil {
		t.Fatalf("read path-bound parent case: %v", err)
	}
	return strings.ReplaceAll(string(raw), "\r\n", "\n")
}

func parentDoc(t *testing.T) *asterdoc.FrozenAsterDoc {
	t.Helper()
	return &asterdoc.FrozenAsterDoc{DesignID: "design-rtqpcr", Revision: 2, Source: parentSourceText(t)}
}

func parentTasks() []runs.RunTask {
	paths := []string{
		"root/rna-extraction/01-collect", "root/rna-extraction/02-lyse", "root/rna-extraction/03-ethanol",
		"root/rna-extraction/04-bind", "root/rna-extraction/05-wash1", "root/rna-extraction/06-wash2a",
		"root/rna-extraction/07-wash2b", "root/rna-extraction/08-dry", "root/rna-extraction/09-elute",
		"root/rna-extraction/10-qc", "root/rna-qc", "root/reverse-transcription", "root/qpcr",
	}
	out := make([]runs.RunTask, 0, len(paths))
	for i, path := range paths {
		status := "completed"
		if i == 5 {
			status = "in_progress"
		} else if i > 5 && i < 10 {
			status = "not_started"
		}
		if i == 11 {
			status = "not_started"
		}
		if i == 12 {
			status = "blocked"
		}
		out = append(out, runs.RunTask{
			Key:               `{"path":"` + path + `"}`,
			Path:              path,
			Title:             path,
			Status:            status,
			ProtocolVersionID: "protocol-rtqpcr-v1",
		})
	}
	return out
}

func TestProjectionBuildsFourParentsAndTenRNAOperations(t *testing.T) {
	tree, err := Build(parentTasks(), parentDoc(t))
	if err != nil {
		t.Fatal(err)
	}
	if err := tree.Validate(); err != nil {
		t.Fatal(err)
	}
	if len(tree.Roots) != 4 {
		t.Fatalf("roots = %d, want 4", len(tree.Roots))
	}
	rna := tree.Roots[0]
	if rna.Title != "01 RNA 提取" || rna.Kind != KindStep {
		t.Fatalf("first root = %+v", rna)
	}
	if rna.Total != 10 || rna.Completed != 5 {
		t.Fatalf("RNA aggregate = %d/%d, want 5/10", rna.Completed, rna.Total)
	}
	if rna.Status != "current" {
		t.Fatalf("RNA status = %q, want current", rna.Status)
	}
	if rna.DefinitionRef != "c23df8e1-3f2a-4449-8ac9-a4bb7256a113" {
		t.Fatalf("RNA definition = %q", rna.DefinitionRef)
	}
	if len(rna.Children) != 10 {
		t.Fatalf("RNA children = %d, want 10", len(rna.Children))
	}
	if rna.Children[0].Title != "root/rna-extraction/01-collect" || rna.Children[0].Kind != KindTask {
		t.Fatalf("first child = %+v", rna.Children[0])
	}
	if tree.Roots[1].Title != "02 RNA 质控" || tree.Roots[1].Total != 1 || tree.Roots[1].Status != "completed" {
		t.Fatalf("QC root = %+v", tree.Roots[1])
	}
	if tree.Roots[2].Status != "pending" {
		t.Fatalf("RT status = %q", tree.Roots[2].Status)
	}
	if tree.Roots[3].Status != "blocked" {
		t.Fatalf("qPCR status = %q", tree.Roots[3].Status)
	}
	if got := tree.Precedes["step-rna-extract"]; len(got) != 1 || got[0] != "step-rna-qc" {
		t.Fatalf("precedes = %+v", tree.Precedes)
	}
}

func TestProjectionRejectsOrphansAndAmbiguity(t *testing.T) {
	tasks := append(parentTasks(), runs.RunTask{Key: `{"path":"root/orphan"}`, Path: "root/orphan", Title: "orphan"})
	tree, err := Build(tasks, parentDoc(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(tree.Orphans) != 1 || tree.Orphans[0].Path != "root/orphan" {
		t.Fatalf("orphans = %+v", tree.Orphans)
	}
	if tree.Validate() == nil || !strings.Contains(tree.Validate().Error(), "no design step") {
		t.Fatalf("Validate() = %v", tree.Validate())
	}

	ambiguous := strings.Replace(parentSourceText(t), "path: root/qpcr", "path: root/rna-qc", 1)
	tree, err = Build(parentTasks(), &asterdoc.FrozenAsterDoc{DesignID: "d", Revision: 1, Source: ambiguous})
	if err != nil {
		t.Fatal(err)
	}
	if tree.Validate() == nil || !strings.Contains(tree.Validate().Error(), "share path") {
		t.Fatalf("ambiguous Validate() = %v", tree.Validate())
	}
}

func TestProjectionRejectsCompositeWithoutDefinition(t *testing.T) {
	withoutRef := strings.Replace(parentSourceText(t), "  - {ref: ref-rna-block, role: other}\n", "", 1)
	tree, err := Build(parentTasks(), &asterdoc.FrozenAsterDoc{DesignID: "d", Revision: 1, Source: withoutRef})
	if err != nil {
		t.Fatal(err)
	}
	if tree.Validate() == nil || !strings.Contains(tree.Validate().Error(), "no explicit reference") {
		t.Fatalf("Validate() = %v", tree.Validate())
	}
}
