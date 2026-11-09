package workflow

import (
	"fmt"
	"github.com/egoist/mygo/ui"
)

var (
	pathFlask   = ui.MustParsePath("M10,30 L10,10 Q10,4 16,4 L44,4 Q50,4 50,10 L50,30 L46,52 Q44,60 36,62 L24,62 Q16,60 14,52 Z")
	pathPlate   = ui.MustParsePath("M6,20 L6,44 Q6,52 14,52 L46,52 Q54,52 54,44 L54,20 Z M10,20 L10,12 L50,12 L50,20")
	pathPipette = ui.MustParsePath("M26,4 L34,4 L34,28 L46,52 L46,56 L14,56 L14,52 L26,28 Z")
)

func iconPath(k NodeKind) *ui.Path {
	switch k {
	case KindPlate:
		return pathPlate
	case KindPipette:
		return pathPipette
	default:
		return pathFlask
	}
}

func (e *Editor) View(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Fill().Background(t.Background).Children(func() {
		ui.Row(c).Padding(8).Gap(6).Children(func() {
			if ui.Button(c, "Add Flask").Clicked() {
				e.AddNode(KindFlask, "Flask", Position{X: 80 + float32(len(e.Doc.Nodes)*20), Y: 80})
			}
			if ui.Button(c, "Add Plate").Clicked() {
				e.AddNode(KindPlate, "Plate", Position{X: 80 + float32(len(e.Doc.Nodes)*20), Y: 120})
			}
			if ui.Button(c, "Add Pipette").Clicked() {
				e.AddNode(KindPipette, "Pipette", Position{X: 80 + float32(len(e.Doc.Nodes)*20), Y: 160})
			}
			if ui.Button(c, "Delete").Clicked() {
				e.DeleteSelected()
			}
			if ui.Button(c, "Duplicate").Clicked() {
				e.DuplicateSelected()
			}
			if ui.Button(c, "Undo").Clicked() {
				e.Undo()
			}
			if ui.Button(c, "Redo").Clicked() {
				e.Redo()
			}
			if ui.Button(c, "Connect").Clicked() {
				e.ConnectMode = !e.ConnectMode
				if !e.ConnectMode {
					e.ConnectFrom = ""
				}
			}
			if ui.Button(c, "−").Clicked() && e.Zoom > .5 {
				e.Zoom -= .1
			}
			if ui.Button(c, "+").Clicked() && e.Zoom < 2 {
				e.Zoom += .1
			}
			if ui.Button(c, "Pan ←").Clicked() {
				e.PanX += 32
			}
			if ui.Button(c, "Pan →").Clicked() {
				e.PanX -= 32
			}
			if ui.Button(c, "Pan ↑").Clicked() {
				e.PanY += 32
			}
			if ui.Button(c, "Pan ↓").Clicked() {
				e.PanY -= 32
			}
			ui.Text(c, fmt.Sprintf("%.0f%%", e.zoomOr1()*100)).FontSize(11).TextColor(t.TextMuted)
		})
		ui.Divider(c)
		ui.Box(c).Grow(1).Fill().Draw(func(p *ui.Painter, r ui.Rect) {
			p.Fill(r, t.Surface, 0)
			for _, edge := range e.Doc.Edges {
				from, to := e.Doc.findNodePtr(edge.From), e.Doc.findNodePtr(edge.To)
				if from == nil || to == nil {
					continue
				}
				z := e.zoomOr1()
				x0, y0 := r.X+(from.Pos.X+e.PanX)*z, r.Y+(from.Pos.Y+e.PanY)*z
				x1, y1 := r.X+(to.Pos.X+e.PanX)*z, r.Y+(to.Pos.Y+e.PanY)*z
				p.Line(x0+36*z, y0+36*z, x1+36*z, y1+36*z, 2, t.Border)
			}
		})
		for i := range e.Doc.Nodes {
			n := &e.Doc.Nodes[i]
			z := e.zoomOr1()
			off := ui.Point{X: (n.Pos.X + e.PanX) * z, Y: (n.Pos.Y + e.PanY) * z}
			selected := n.ID == e.SelectedNode
			ui.Box(c).Absolute().Left(off.X).Top(off.Y).Size(72*z, 72*z).Draw(func(p *ui.Painter, r ui.Rect) {
				fill := t.Surface
				if selected {
					fill = t.Accent
				}
				p.Fill(r, fill, 8)
				p.Stroke(r, t.Border, 8, 1)
				p.Text(r.X+4, r.Y+22*z, string(n.Kind), 10*z, t.Text)
				p.Text(r.X+4, r.Y+58*z, n.Title, 10*z, t.Text)
			}).OnClick(func() {
				if !e.ConnectMode {
					e.SelectNode(n.ID)
					return
				}
				if e.ConnectFrom == "" {
					e.ConnectFrom = n.ID
				} else if e.ConnectFrom != n.ID {
					e.Connect(e.ConnectFrom, n.ID)
					e.ConnectFrom = ""
					e.ConnectMode = false
				}
			}).Draggable(&off, ui.DraggableOptions{}).Key(n.ID)
			pos := Position{X: off.X/z - e.PanX, Y: off.Y/z - e.PanY}
			if pos != n.Pos {
				e.MoveNode(n.ID, pos)
			}
		}
	})
}

func (d *WorkflowDocument) findNodePtr(id string) *Node {
	for i := range d.Nodes {
		if d.Nodes[i].ID == id {
			return &d.Nodes[i]
		}
	}
	return nil
}
func (e *Editor) zoomOr1() float32 {
	if e.Zoom != 0 {
		return e.Zoom
	}
	return 1
}
