package designer

import (
	"context"
	"encoding/json"
	"image/png"
	"os"
	"testing"

	"asterlab-mygo-prototype/internal/assays"
	"asterlab-mygo-prototype/internal/blocks"
	"asterlab-mygo-prototype/internal/state"
	"asterlab-mygo-prototype/internal/workflow"
	"asterlab-mygo-prototype/modules/runs"

	"github.com/egoist/mygo/ui"
)

func sampleDesigner(t *testing.T) *Designer {
	reg := blocks.NewRegistry()
	def := blocks.Definition{ID: "note1", Type: blocks.TypeNote, Version: 1, Props: map[string]any{"text": "hello"}}
	if err := reg.Register(def); err != nil {
		t.Fatal(err)
	}
	inst, err := reg.FreezeInstance("i1", "note1", 1)
	if err != nil {
		t.Fatal(err)
	}
	d := New("design-1", "Test Design", 3, &state.FakeDesignService{
		Snapshot: runs.DesignSnapshot{DesignID: "design-1", Revision: 4, Title: "Test Design", Data: ""},
	})
	d.Instances = map[string][]blocks.Instance{"n1": {inst}}
	d.Editor.Doc.Nodes = append(d.Editor.Doc.Nodes, workflow.Node{ID: "n1", Kind: workflow.NodeKind("step"), Title: "Step 1"})
	d.SelectedNode = "n1"
	return d
}

func TestSaveCallback(t *testing.T) {
	d := sampleDesigner(t)
	d.Assays["cck8"] = assays.Design{Timepoints: []string{"24 h"}, ControlGroup: "Control"}
	d.StepAssays["n1"] = "cck8"
	fake := &state.FakeDesignService{
		Snapshot: runs.DesignSnapshot{DesignID: "design-1", Revision: 5, Title: "Test Design"},
	}
	d.Service = nil
	var got runs.DesignSaveInput
	d.Callbacks.SaveDesign = func(ctx context.Context, input runs.DesignSaveInput) (runs.DesignSnapshot, error) {
		got = input
		return fake.SaveDesign(ctx, input)
	}
	d.Dirty = true
	d.Save(context.Background())
	var saved designDoc
	if err := json.Unmarshal([]byte(got.Changes), &saved); err != nil {
		t.Fatal(err)
	}
	if saved.MyGo.Assays["cck8"].ControlGroup != "Control" || saved.MyGo.StepAssays["n1"] != "cck8" {
		t.Fatalf("assay mapping not persisted in mygo extension: %+v", saved.MyGo)
	}
	if got.ExpectedRevision != 3 {
		t.Fatalf("expected revision = %d", got.ExpectedRevision)
	}
	if got.OperationID == "" {
		t.Fatalf("operation id empty")
	}
	if d.Revision != 5 {
		t.Fatalf("revision = %d", d.Revision)
	}
	if d.Dirty {
		t.Fatalf("dirty not cleared")
	}
	if d.SaveState != SaveSaved {
		t.Fatalf("save state = %s", d.SaveState)
	}
}

func TestErrorDraftRetention(t *testing.T) {
	d := sampleDesigner(t)
	d.Service = nil
	d.Callbacks.SaveDesign = func(ctx context.Context, input runs.DesignSaveInput) (runs.DesignSnapshot, error) {
		return runs.DesignSnapshot{}, &typedErr{msg: "CONFLICT: stale"}
	}
	before := len(d.Editor.Doc.Nodes)
	d.Save(context.Background())
	if d.SaveState != SaveError {
		t.Fatalf("state = %s", d.SaveState)
	}
	if len(d.Editor.Doc.Nodes) != before {
		t.Fatalf("draft mutated")
	}
	if d.Error == "" {
		t.Fatalf("no error")
	}
}

type typedErr struct{ msg string }

func (e *typedErr) Error() string { return e.msg }

func TestLoadApply(t *testing.T) {
	doc := map[string]any{
		"designId": "design-1",
		"title":    "Loaded",
		"workflow": map[string]any{"nodes": []any{}, "edges": []any{}},
	}
	b, _ := json.Marshal(doc)
	fake := &state.FakeDesignService{
		Snapshot: runs.DesignSnapshot{DesignID: "design-1", Revision: 7, Title: "Loaded", Data: string(b)},
	}
	d := New("design-1", "X", 1, fake)
	d.Load(context.Background(), "design-1")
	if d.Revision != 7 {
		t.Fatalf("rev=%d", d.Revision)
	}
	if d.Title != "Loaded" {
		t.Fatalf("title=%s", d.Title)
	}
	if d.Loading {
		t.Fatalf("loading true")
	}
}

func TestEmptyState(t *testing.T) {
	fake := &state.FakeDesignService{
		Snapshot: runs.DesignSnapshot{DesignID: "design-empty", Revision: 1, Title: "Empty", Data: ""},
	}
	d := New("design-empty", "Empty", 0, fake)
	d.Load(context.Background(), "design-empty")
	if !d.Empty {
		t.Fatalf("not empty")
	}
	tt := ui.NewTester(func(c *ui.Context) { View(c, d) }, 1200, 760)
	tt.Frame()
	_ = tt.Image()
}

func TestScreenshot(t *testing.T) {
	d := sampleDesigner(t)
	tt := ui.NewTester(func(c *ui.Context) { View(c, d) }, 1200, 760)
	tt.Frame()
	f, err := os.Create("designer.png")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, tt.Image()); err != nil {
		t.Fatal(err)
	}
}

type persistedDesignService struct {
	runs.DesignService
	snapshot runs.DesignSnapshot
	created  bool
}

func (s *persistedDesignService) ListDesigns(context.Context) ([]runs.DesignSummary, error) {
	return nil, nil
}

func (s *persistedDesignService) LoadDesign(_ context.Context, _ string) (runs.DesignSnapshot, error) {
	return s.snapshot, nil
}

func (s *persistedDesignService) SaveDesign(_ context.Context, input runs.DesignSaveInput) (runs.DesignSnapshot, error) {
	s.snapshot = runs.DesignSnapshot{
		DesignID: input.DesignID, Revision: input.ExpectedRevision + 1,
		Title: "Saved design", Data: input.Changes,
	}
	return s.snapshot, nil
}

func (s *persistedDesignService) CreateRun(_ context.Context, designID, _ string) (runs.RunSnapshot, error) {
	s.created = true
	return runs.RunSnapshot{DesignID: designID, RunID: "run-created"}, nil
}

func (s *persistedDesignService) Close() error { return nil }

func TestDesignerUIMapSaveReopenAndCreateRun(t *testing.T) {
	svc := &persistedDesignService{snapshot: runs.DesignSnapshot{DesignID: "design-1", Revision: 2, Title: "Test"}}
	d := New("design-1", "Test", 2, svc)
	tt := ui.NewTester(func(c *ui.Context) { View(c, d) }, 1200, 760)
	tt.Frame()
	d.Editor.AddNode(workflow.KindFlask, "Flask", workflow.Position{X: 80, Y: 80})
	d.SelectedNode = d.Editor.SelectedNode
	if d.SelectedNode == "" {
		t.Fatal("workflow editor did not create/select a step")
	}
	d.setNodeTitle(d.SelectedNode, "Incubate cells")
	d.AssayIDDraft = assays.CCK8
	d.StepAssays[d.SelectedNode] = assays.CCK8
	d.Assays[assays.CCK8] = assays.Design{Timepoints: []string{"24 h"}}
	d.SampleIDDraft, d.GroupIDDraft, d.WellTimepointDraft = "sample-1", "control", "24 h"
	p := d.Assays[assays.CCK8]
	p.Wells = []assays.WellAssignment{{WellID: "A1", SampleID: "sample-1", GroupID: "control", Replicate: 1, Timepoint: "24 h"}}
	d.Assays[assays.CCK8] = p
	d.Save(context.Background())
	if d.SaveState != SaveSaved {
		t.Fatalf("save state = %s (%s)", d.SaveState, d.Error)
	}

	reopened := New("design-1", "Test", 0, svc)
	reopened.Load(context.Background(), "design-1")
	if got := reopened.nodeTitle(reopened.Editor.Doc.Nodes[0].ID); got != "Incubate cells" {
		t.Fatalf("reopened step title = %q", got)
	}
	if got := reopened.Assays[assays.CCK8].Wells; len(got) != 1 || got[0].WellID != "A1" {
		t.Fatalf("reopened mapping = %+v", got)
	}
	tt = ui.NewTester(func(c *ui.Context) { View(c, reopened) }, 1200, 760)
	tt.Frame()
	f, err := os.Create("designer-mapping-roundtrip.png")
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(f, tt.Image()); err != nil {
		f.Close()
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	reopened.setNodeTitle(reopened.Editor.Doc.Nodes[0].ID, "Updated step")
	var created runs.RunSnapshot
	reopened.OnCreateRun = func(s runs.RunSnapshot) { created = s }
	reopened.CreateRun(context.Background())
	if !svc.created || created.RunID != "run-created" {
		t.Fatalf("Core CreateRun not reached: created=%v run=%+v", svc.created, created)
	}
	if reopened.Dirty || reopened.SaveState != SaveSaved {
		t.Fatalf("CreateRun did not save current draft: state=%s dirty=%v", reopened.SaveState, reopened.Dirty)
	}
}
