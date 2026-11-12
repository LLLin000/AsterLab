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
		sourceBlock := "c23df8e1-3f2a-4449-8ac9-a4bb7256a113"
		if i >= 10 {
			sourceBlock = "b41381b7-cd4c-44ca-9efa-97d43f9d66ea"
		}
		out = append(out, runs.RunTask{
			Key:                  `{"path":"` + path + `"}`,
			Path:                 path,
			Title:                path,
			Status:               status,
			ProtocolVersionID:    "protocol-rtqpcr-v1",
			SourceBlockVersionID: sourceBlock,
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
	if got := tree.Order; len(got) != 4 || got[1] != "step-rna-qc" {
		t.Fatalf("business order = %+v", got)
	}
	if got := tree.Precedes["step-rna-extract"]; len(got) != 1 || got[0] != "step-rna-qc" {
		t.Fatalf("precedes = %+v", tree.Precedes)
	}
}

func TestProjectionKeepsDisplayOrderAndDependenciesIndependent(t *testing.T) {
	source := strings.Replace(parentSourceText(t),
		"business_order: [step-rna-extract, step-rna-qc, step-rt, step-qpcr]",
		"business_order: [step-rna-extract, step-qpcr, step-rna-qc, step-rt]", 1)
	tree, err := Build(parentTasks(), &asterdoc.FrozenAsterDoc{DesignID: "d", Revision: 1, Source: source})
	if err != nil {
		t.Fatal(err)
	}
	wantOrder := []string{"step-rna-extract", "step-qpcr", "step-rna-qc", "step-rt"}
	for i, want := range wantOrder {
		if tree.Order[i] != want {
			t.Fatalf("order[%d] = %q, want %q", i, tree.Order[i], want)
		}
	}
	if got := tree.Roots[1].ID; got != "step-qpcr" {
		t.Fatalf("root display order was not applied: %q", got)
	}
	if got := tree.Precedes["step-rna-extract"]; len(got) != 1 || got[0] != "step-rna-qc" {
		t.Fatalf("dependency was replaced by display order: %+v", tree.Precedes)
	}
	if got := tree.Precedes["step-rna-qc"]; len(got) != 1 || got[0] != "step-rt" {
		t.Fatalf("qPCR edge lost: %+v", tree.Precedes)
	}
	if err := tree.Validate(); err != nil {
		t.Fatal(err)
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

func TestProjectionBuildsNestedContainment(t *testing.T) {
	source := strings.Replace(parentSourceText(t), "ref: c23df8e1-3f2a-4449-8ac9-a4bb7256a113", "ref: purify-block", 1)
	tasks := []runs.RunTask{
		{Key: `{"path":"root/rna-extraction/purification/01-wash"}`, Path: "root/rna-extraction/purification/01-wash", Title: "01 Wash", Status: "completed", ProtocolVersionID: "pv", SourceBlockVersionID: "purify-block"},
		{Key: `{"path":"root/rna-extraction/purification/02-elute"}`, Path: "root/rna-extraction/purification/02-elute", Title: "02 Elute", Status: "in_progress", ProtocolVersionID: "pv", SourceBlockVersionID: "purify-block"},
		{Key: `{"path":"root/rna-qc"}`, Path: "root/rna-qc", Title: "QC", Status: "completed", ProtocolVersionID: "pv"},
		{Key: `{"path":"root/reverse-transcription"}`, Path: "root/reverse-transcription", Title: "RT", Status: "pending", ProtocolVersionID: "pv"},
		{Key: `{"path":"root/qpcr"}`, Path: "root/qpcr", Title: "qPCR", Status: "pending", ProtocolVersionID: "pv"},
	}
	tree, err := Build(tasks, &asterdoc.FrozenAsterDoc{DesignID: "d", Revision: 1, Source: source})
	if err != nil {
		t.Fatal(err)
	}
	if !tree.HasComposite() {
		t.Fatal("nested tree was not marked composite")
	}
	root := tree.Roots[0]
	if len(root.Children) != 1 || root.Children[0].Title != "purification" {
		t.Fatalf("root children = %+v", root.Children)
	}
	nested := root.Children[0]
	if len(nested.Children) != 2 || nested.Completed != 1 || nested.Total != 2 || nested.Status != "current" {
		t.Fatalf("nested node = %+v", nested)
	}
	if err := tree.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestProjectionRejectsWrongBlockVersion(t *testing.T) {
	tasks := parentTasks()
	tasks[0].SourceBlockVersionID = "wrong-block-version"
	tree, err := Build(tasks, parentDoc(t))
	if err != nil {
		t.Fatal(err)
	}
	if tree.Validate() == nil || !strings.Contains(tree.Validate().Error(), "does not match referenced block") {
		t.Fatalf("Validate() = %v", tree.Validate())
	}
}

// A frozen task path is the protocol block's own path, which is not the
// author-chosen design step path. The recorded design step identity is the
// only join, and the protocol's steps must hang directly under the design
// step instead of under a stray protocol root level.
func TestProjectionJoinsByDesignStepIdentity(t *testing.T) {
	source := strings.Join([]string{
		"```asterlab",
		"kind: experiment",
		"id: exp",
		"schema: 1",
		"title: CCK-8",
		"```",
		"```asterlab",
		"kind: assay",
		"id: assay-cck8",
		"schema: 1",
		"title: CCK-8",
		"endpoint: OD450",
		"method: absorbance",
		"unit: AU",
		"```",
		"```asterlab",
		"kind: reference",
		"id: ref-block",
		"schema: 1",
		"ref: block-v1",
		"view: protocol_block",
		"```",
		"```asterlab",
		"kind: step",
		"id: step-read",
		"schema: 2",
		"title: 02 CCK-8 检测",
		"assay: assay-cck8",
		"path: cck8/readout",
		"order: 1",
		"uses:",
		"  - {ref: ref-block, role: other}",
		"```",
		"```asterlab",
		"kind: workflow",
		"id: wf",
		"schema: 1",
		"steps: [step-read]",
		"business_order: [step-read]",
		"```",
	}, "\n")
	tasks := []runs.RunTask{
		{Key: `{"path":"root/seed"}`, Path: "root/seed", Title: "接种细胞", Status: "completed",
			DesignStepID: "step-read", SourceBlockVersionID: "block-v1", AssayID: "assay-cck8"},
		{Key: `{"path":"root/read"}`, Path: "root/read", Title: "读取 OD450", Status: "not_started",
			DesignStepID: "step-read", SourceBlockVersionID: "block-v1", AssayID: "assay-cck8"},
	}
	tree, err := Build(tasks, &asterdoc.FrozenAsterDoc{DesignID: "d", Revision: 1, Source: source})
	if err != nil {
		t.Fatal(err)
	}
	if len(tree.Orphans) != 0 || len(tree.Ambiguous) != 0 {
		t.Fatalf("orphans=%d ambiguous=%d issues=%v", len(tree.Orphans), len(tree.Ambiguous), tree.Issues)
	}
	if len(tree.Roots) != 1 {
		t.Fatalf("roots = %d, want 1", len(tree.Roots))
	}
	root := tree.Roots[0]
	if !root.Composite || len(root.Children) != 2 {
		t.Fatalf("root = %+v", root)
	}
	// The protocol root ("root") is not a containment level of the design.
	seen := map[string]bool{}
	for _, child := range root.Children {
		if child.Kind != KindTask {
			t.Fatalf("child %+v is not a frozen task", child)
		}
		seen[child.Title] = true
	}
	if !seen["接种细胞"] || !seen["读取 OD450"] {
		t.Fatalf("children = %+v", root.Children)
	}
	if root.Total != 2 || root.Completed != 1 {
		t.Fatalf("aggregate = %d/%d, want 1/2", root.Completed, root.Total)
	}
	if err := tree.Validate(); err != nil {
		t.Fatalf("Validate() = %v", err)
	}
}
