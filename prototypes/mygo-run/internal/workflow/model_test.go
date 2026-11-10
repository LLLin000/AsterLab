package workflow

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestWheelNavigationZoomsCanvas(t *testing.T) {
	ed := NewEditor()
	tt := ui.NewTester(func(c *ui.Context) { ed.View(c) }, 900, 600)
	tt.Frame()
	before := ed.Zoom
	tt.Wheel(400, 300, 0, -120)
	tt.Frame()
	if ed.Zoom <= before {
		t.Fatalf("wheel did not zoom: %v -> %v", before, ed.Zoom)
	}
	panX := ed.PanX
	tt.Wheel(400, 300, 40, 0)
	tt.Frame()
	if ed.PanX == panX {
		t.Fatal("horizontal wheel did not pan")
	}
}

func TestSelectInBoundsAndToggleMultiSelection(t *testing.T) {
	ed := NewEditor()
	a := ed.AddNode(KindPipette, "A", Position{X: 100, Y: 100})
	b := ed.AddNode(KindFlask, "B", Position{X: 500, Y: 100})
	ed.ClearSelection()
	ed.SelectInBounds(50, 50, 300, 180, false)
	if got := ed.SelectedIDs(); len(got) != 1 || got[0] != a {
		t.Fatalf("bounds selection = %v", got)
	}
	ed.ToggleNode(b)
	if got := ed.SelectedIDs(); len(got) != 2 {
		t.Fatalf("toggle selection = %v", got)
	}
	ed.ToggleNode(a)
	if got := ed.SelectedIDs(); len(got) != 1 || got[0] != b {
		t.Fatalf("toggle removal = %v", got)
	}
}

func TestDeleteSelectedRemovesWholeMultiSelectionOnce(t *testing.T) {
	ed := NewEditor()
	a := ed.AddNode(KindPipette, "A", Position{})
	b := ed.AddNode(KindFlask, "B", Position{X: 300})
	ed.SelectedNodes = map[string]bool{a: true, b: true}
	ed.SelectedNode = a
	if !ed.DeleteSelected() {
		t.Fatal("delete failed")
	}
	if len(ed.Doc.Nodes) != 0 {
		t.Fatalf("nodes after delete = %+v", ed.Doc.Nodes)
	}
	if !ed.Undo() || len(ed.Doc.Nodes) != 2 {
		t.Fatalf("undo did not restore both nodes: %+v", ed.Doc.Nodes)
	}
}

func TestMarqueeOnEmptyCanvasSelectsNode(t *testing.T) {
	ed := NewEditor()
	id := ed.AddNode(KindPipette, "A", Position{X: 120, Y: 100})
	ed.ClearSelection()
	tt := ui.NewTester(func(c *ui.Context) { ed.View(c) }, 900, 600)
	tt.Frame()
	tt.Press(10, 10)
	tt.Frame()
	tt.Move(420, 300)
	tt.Frame()
	tt.Release(420, 300)
	tt.Frame()
	if ids := ed.SelectedIDs(); len(ids) != 1 || ids[0] != id {
		t.Fatalf("marquee selected = %v", ids)
	}
}

func TestDraggingSelectedNodeMovesWholeSelection(t *testing.T) {
	ed := NewEditor()
	a := ed.AddNode(KindPipette, "A", Position{X: 100, Y: 100})
	b := ed.AddNode(KindFlask, "B", Position{X: 300, Y: 200})
	ed.SelectedNodes = map[string]bool{a: true, b: true}
	ed.SelectedNode = a
	ed.dragNode(a, Position{X: 140, Y: 130}, true)
	ed.dragNode(a, Position{X: 140, Y: 130}, false)
	ia, ib := ed.Doc.findNode(a), ed.Doc.findNode(b)
	if ed.Doc.Nodes[ia].Pos != (Position{X: 140, Y: 130}) || ed.Doc.Nodes[ib].Pos != (Position{X: 340, Y: 230}) {
		t.Fatalf("positions = %+v / %+v", ed.Doc.Nodes[ia].Pos, ed.Doc.Nodes[ib].Pos)
	}
	if len(ed.UndoStack) == 0 {
		t.Fatal("batch move did not create one undo entry")
	}
}

func TestShiftActivationExtendsNodeSelection(t *testing.T) {
	ed := NewEditor()
	a := ed.AddNode(KindPipette, "Alpha", Position{X: 80, Y: 80})
	b := ed.AddNode(KindFlask, "Beta", Position{X: 380, Y: 80})
	ed.activateNode(a, 0)
	ed.activateNode(b, ui.Shift)
	got := ed.SelectedIDs()
	if len(got) != 2 || got[0] != a || got[1] != b {
		t.Fatalf("shift selection = %v", got)
	}
}
