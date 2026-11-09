package designer

import (
	"context"
	"encoding/json"
	"image/png"
	"os"
	"testing"

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
	var got runs.DesignSaveInput
	fake := &state.FakeDesignService{
		Snapshot: runs.DesignSnapshot{DesignID: "design-1", Revision: 5, Title: "Test Design"},
	}
	d.Service = nil
	d.Callbacks.SaveDesign = func(ctx context.Context, input runs.DesignSaveInput) (runs.DesignSnapshot, error) {
		got = input
		return fake.SaveDesign(ctx, input)
	}
	d.Dirty = true
	d.Save(context.Background())
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
