package workflow

import (
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
		// toolbar
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
		})
		ui.Divider(c)
		// canvas
		ui.Box(c).Grow(1).Fill().Draw(func(p *ui.Painter, r ui.Rect) {
			// grid-ish background
			p.Fill(r, t.Surface, 0)
			// edges
			for _, ed := range e.Doc.Edges {
				fn := e.Doc.findNodePtr(ed.From)
				tn := e.Doc.findNodePtr(ed.To)
				if fn == nil || tn == nil {
					continue
				}
				x0 := r.X + (fn.Pos.X+e.PanX)*e.zoomOr1()
				y0 := r.Y + (fn.Pos.Y+e.PanY)*e.zoomOr1()
				x1 := r.X + (tn.Pos.X+e.PanX)*e.zoomOr1()
				y1 := r.Y + (tn.Pos.Y+e.PanY)*e.zoomOr1()
				p.Line(x0+36, y0+36, x1+36, y1+36, 2, t.Border)
			}
			// nodes drawn via overlay boxes below; but draw selection ring here
		})
		// node overlays
		for i := range e.Doc.Nodes {
			n := &e.Doc.Nodes[i]
			oldPos := n.Pos
			off := ui.Point{X: n.Pos.X + e.PanX, Y: n.Pos.Y + e.PanY}
			// scale not applied to point (int); approximate
			sel := n.ID == e.SelectedNode
			ui.Box(c).Absolute().Left(off.X).Top(off.Y).Size(72, 72).Draw(func(p *ui.Painter, r ui.Rect) {
				col := t.Surface
				if sel {
					col = ui.Hex("#5b45c9")
				}
				p.Fill(r, col, 8)
				p.Stroke(r, t.Border, 8, 1)
				ip := iconPath(n.Kind)
				// center icon
				ir := ui.Rect{X: r.X + 12, Y: r.Y + 8, W: 48, H: 48}
				p.FillPath(ip, t.Text)
				_ = ir
				p.Text(r.X+4, r.Y+58, n.Title, 10, t.Text)
			}).OnClick(func() {
				if e.ConnectMode {
					if e.ConnectFrom == "" {
						e.ConnectFrom = n.ID
					} else if e.ConnectFrom != n.ID {
						e.Connect(e.ConnectFrom, n.ID)
						e.ConnectFrom = ""
						e.ConnectMode = false
					}
				} else {
					e.SelectNode(n.ID)
				}
			}).Draggable(&off, ui.DraggableOptions{}).Key(n.ID)
			// sync position back
			n.Pos = Position{X: float32(off.X) - e.PanX, Y: float32(off.Y) - e.PanY}
			if n.Pos != oldPos && e.OnChange != nil {
				e.OnChange()
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
	if e.Zoom == 0 {
		return 1
	}
	return e.Zoom
}
