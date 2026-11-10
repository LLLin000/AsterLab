package workflow

import (
	"image/png"
	"os"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestWorkflowAddSelectUndo(t *testing.T) {
	e := NewEditor()
	e.OnSelect = func(string) {}
	tt := ui.NewTester(func(c *ui.Context) { e.View(c) }, 800, 600)
	tt.Frame()
	// add via model (UI button click could be used)
	id := e.AddNode(KindFlask, "F1", Position{X: 10, Y: 10})
	if id == "" {
		t.Fatal("add failed")
	}
	if len(e.Doc.Nodes) != 1 {
		t.Fatalf("nodes=%d", len(e.Doc.Nodes))
	}
	e.SelectNode(id)
	if e.SelectedNode != id {
		t.Fatal("select failed")
	}
	// undo add
	if !e.Undo() {
		t.Fatal("undo failed")
	}
	if len(e.Doc.Nodes) != 0 {
		t.Fatalf("after undo nodes=%d", len(e.Doc.Nodes))
	}
	// redo
	if !e.Redo() {
		t.Fatal("redo failed")
	}
	if len(e.Doc.Nodes) != 1 {
		t.Fatalf("after redo nodes=%d", len(e.Doc.Nodes))
	}
	// drag/move + undo
	e.MoveNode(id, Position{X: 50, Y: 60})
	n := e.Doc.Nodes[0]
	if n.Pos.X != 50 || n.Pos.Y != 60 {
		t.Fatalf("move not applied: %+v", n.Pos)
	}
	if !e.Undo() {
		t.Fatal("undo move failed")
	}
	if e.Doc.Nodes[0].Pos.X == 50 && e.Doc.Nodes[0].Pos.Y == 60 {
		t.Fatal("undo did not revert move")
	}
	// nested hook
	called := false
	e.OnEnterNested = func(string) { called = true }
	e.EnterNested(id)
	if !called {
		t.Fatal("nested hook not called")
	}
}

func TestWorkflowHeadlessPNG(t *testing.T) {
	e := NewEditor()
	e.AddNode(KindFlask, "Flask", Position{X: 40, Y: 40})
	e.AddNode(KindPlate, "Plate", Position{X: 160, Y: 80})
	e.AddNode(KindPipette, "Pipette", Position{X: 280, Y: 120})
	e.Doc.AddEdge(e.Doc.Nodes[0].ID, e.Doc.Nodes[1].ID)
	tt := ui.NewTester(func(c *ui.Context) { e.View(c) }, 900, 600)
	tt.Frame()
	img := tt.Image()
	f, err := os.Create("workflow-headless.png")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
}

func TestWorkflowCanvasEmptyState(t *testing.T) {
	e := NewEditor()
	tt := ui.NewTester(func(c *ui.Context) { e.View(c) }, 800, 600)
	tt.Frame()
	if !tt.HasText("空白工作流") {
		t.Fatalf("missing empty state: %q", tt.Texts())
	}
}

func TestWorkflowCanvasShowsStepCards(t *testing.T) {
	e := NewEditor()
	e.AddNode(KindFlask, "GelMA 水凝胶", Position{X: 40, Y: 40})
	e.AddNode(KindPipette, "铺板 96 孔", Position{X: 300, Y: 40})
	tt := ui.NewTester(func(c *ui.Context) { e.View(c) }, 800, 600)
	tt.Frame()
	for _, title := range []string{"GelMA 水凝胶", "铺板 96 孔", "未填写参数"} {
		if !tt.HasText(title) {
			t.Fatalf("missing %q in %q", title, tt.Texts())
		}
	}
	if tt.HasText("空白工作流") {
		t.Fatal("empty state shown with steps")
	}
}
func TestDragUsesSingleUndoEntry(t *testing.T) {
	e := NewEditor()
	id := e.AddNode(KindFlask, "F1", Position{X: 10, Y: 10})
	e.dragNode(id, Position{X: 20, Y: 20}, true)
	e.dragNode(id, Position{X: 30, Y: 30}, true)
	e.dragNode(id, Position{X: 40, Y: 40}, false)
	if len(e.UndoStack) != 2 {
		t.Fatalf("undo entries=%d, want add + one drag", len(e.UndoStack))
	}
	if !e.Undo() || e.Doc.Nodes[0].Pos != (Position{X: 10, Y: 10}) {
		t.Fatal("undo did not revert the whole drag")
	}
}
