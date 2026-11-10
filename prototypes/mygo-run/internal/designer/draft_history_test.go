package designer

import (
	"testing"

	"asterlab-mygo-prototype/internal/workflow"

	"github.com/egoist/mygo/ui"
)

func TestDraftUndoRedoRestoresFieldsAndCanvas(t *testing.T) {
	d := New("", "", 0, nil)
	d.captureDraft()
	d.Title = "first"
	d.captureDraft()
	d.Editor.AddNode(workflow.KindPipette, "step", workflow.Position{X: 10, Y: 20})
	d.captureDraft()
	if len(d.Editor.Doc.Nodes) != 1 {
		t.Fatal("node not added")
	}
	if !d.UndoDraft() {
		t.Fatal("undo failed")
	}
	if len(d.Editor.Doc.Nodes) != 0 || d.Title != "first" || d.SelectedNode != "" {
		t.Fatalf("undo = title %q nodes %d selected %q", d.Title, len(d.Editor.Doc.Nodes), d.SelectedNode)
	}
	if !d.RedoDraft() {
		t.Fatal("redo failed")
	}
	if len(d.Editor.Doc.Nodes) != 1 || d.Title != "first" {
		t.Fatalf("redo = title %q nodes %d", d.Title, len(d.Editor.Doc.Nodes))
	}
}

func TestSelectionReloadDoesNotCreateDraftHistory(t *testing.T) {
	d := New("", "", 0, nil)
	id := d.Editor.AddNode(workflow.KindFlask, "material", workflow.Position{})
	d.captureDraft()
	before := len(d.draftUndo)
	d.Editor.SelectNode(id)
	d.captureDraft()
	if len(d.draftUndo) != before {
		t.Fatalf("selection created %d history entries", len(d.draftUndo)-before)
	}
}

func TestCaptureSkipsActiveNodeDrag(t *testing.T) {
	d := New("", "", 0, nil)
	id := d.Editor.AddNode(workflow.KindFlask, "material", workflow.Position{})
	d.captureDraft()
	before := len(d.draftUndo)
	d.Editor.MovingNode = id
	d.Editor.Doc.Nodes[0].Pos = workflow.Position{X: 50, Y: 50}
	d.captureDraft()
	if len(d.draftUndo) != before {
		t.Fatal("active drag created a history entry")
	}
	d.Editor.MovingNode = ""
	d.captureDraft()
	if len(d.draftUndo) != before+1 {
		t.Fatalf("released drag history = %d, want %d", len(d.draftUndo), before+1)
	}
}

func TestDesignerViewCapturesDraftChanges(t *testing.T) {
	d := New("", "start", 0, nil)
	tt := ui.NewTester(func(c *ui.Context) { View(c, d) }, 1200, 760)
	tt.Frame()
	d.Title = "changed"
	tt.Frame()
	if !d.UndoDraft() || d.Title != "start" {
		t.Fatalf("undo after View = %q", d.Title)
	}
	if !d.RedoDraft() || d.Title != "changed" {
		t.Fatalf("redo after View = %q", d.Title)
	}
}
