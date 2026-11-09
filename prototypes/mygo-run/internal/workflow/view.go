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
	c.OnShortcut(0, ui.KeyDelete, func() { e.DeleteSelected() })
	c.OnShortcut(ui.Shift, ui.KeyLeft, func() { e.PanX += 32 })
	c.OnShortcut(ui.Shift, ui.KeyRight, func() { e.PanX -= 32 })
	c.OnShortcut(ui.Shift, ui.KeyUp, func() { e.PanY += 32 })
	c.OnShortcut(ui.Shift, ui.KeyDown, func() { e.PanY -= 32 })
	c.OnShortcut(0, ui.KeyEscape, func() {
		e.ConnectMode, e.ConnectFrom = false, ""
		e.ClearSelection()
	})
	c.OnShortcut(ui.Cmd, ui.KeyZ, func() { e.Undo() })
	c.OnShortcut(ui.Cmd, ui.KeyY, func() { e.Redo() })
	ui.Column(c).Fill().Background(t.Background).Children(func() {
		ui.Row(c).Height(40).Padding(8, 12).Gap(6).AlignItems(ui.Center).Background(t.Surface).Children(func() {
			if ui.Button(c, "＋ 材料").Clicked() {
				e.AddNode(KindFlask, "细胞悬液", Position{X: 64 + float32(len(e.Doc.Nodes)*210), Y: 96})
			}
			if ui.Button(c, "＋ 操作").Clicked() {
				e.AddNode(KindPipette, "加样操作", Position{X: 64 + float32(len(e.Doc.Nodes)*210), Y: 96})
			}
			if ui.Button(c, "＋ 仪器").Clicked() {
				e.AddNode(KindPlate, "酶标仪读板", Position{X: 64 + float32(len(e.Doc.Nodes)*210), Y: 96})
			}
			ui.Divider(c)
			if ui.Button(c, "连线").Clicked() {
				e.ConnectMode = !e.ConnectMode
				if !e.ConnectMode {
					e.ConnectFrom = ""
				}
			}
			if ui.Button(c, "撤销").Clicked() {
				e.Undo()
			}
			if ui.Button(c, "重做").Clicked() {
				e.Redo()
			}
			if ui.Button(c, "−").Clicked() && e.Zoom > .5 {
				e.Zoom -= .1
			}
			ui.Text(c, fmt.Sprintf("%.0f%%", e.zoomOr1()*100)).FontSize(11).TextColor(t.TextMuted)
			if ui.Button(c, "+").Clicked() && e.Zoom < 2 {
				e.Zoom += .1
			}
			if ui.Button(c, "适应").Clicked() {
				e.PanX, e.PanY, e.Zoom = 0, 0, 1
			}
			ui.Spacer(c)
			ui.Text(c, "拖动节点 · 连线模式下依次选择起点与终点").FontSize(11).TextColor(t.TextMuted)
		})
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
				p.Line(x0+142*z, y0+45*z, x1, y1+45*z, 2, t.Accent)
			}
		})
		for i := range e.Doc.Nodes {
			n := &e.Doc.Nodes[i]
			z := e.zoomOr1()
			off := ui.Point{X: (n.Pos.X + e.PanX) * z, Y: (n.Pos.Y + e.PanY) * z}
			selected := n.ID == e.SelectedNode
			nodeID, title, kind := n.ID, n.Title, n.Kind
			nodeElement := ui.Box(c).Absolute().Left(off.X).Top(off.Y).Size(148*z, 92*z).Draw(func(p *ui.Painter, r ui.Rect) {
				p.Fill(r, t.SurfaceHover, 8)
				border := t.Border
				if selected {
					border = t.Accent
				}
				p.Stroke(r, border, 8, 2)
				drawHero(p, kind, r.X+9*z, r.Y+25*z, z, t.Accent)
				p.Text(r.X+45*z, r.Y+41*z, title, 13*z, t.Text)
				p.Text(r.X+45*z, r.Y+62*z, nodeSummary(kind), 10*z, t.TextMuted)
				p.Fill(ui.Rect{X: r.X - 3*z, Y: r.Y + 41*z, W: 7 * z, H: 7 * z}, t.Accent, 4)
				p.Fill(ui.Rect{X: r.X + 144*z, Y: r.Y + 41*z, W: 7 * z, H: 7 * z}, t.Accent, 4)
			}).OnClick(func() {
				if !e.ConnectMode {
					e.SelectNode(nodeID)
					return
				}
				if e.ConnectFrom == "" {
					e.ConnectFrom = nodeID
				} else if e.ConnectFrom != nodeID {
					e.Connect(e.ConnectFrom, nodeID)
					e.ConnectFrom = ""
					e.ConnectMode = false
				}
			}).Draggable(&off, ui.DraggableOptions{}).Key(nodeID)
			pos := Position{X: off.X/z - e.PanX, Y: off.Y/z - e.PanY}
			if pos != n.Pos || e.MovingNode == nodeID {
				e.dragNode(nodeID, pos, nodeElement.Dragging())
			}
		}
	})
}

func nodeKindLabel(kind NodeKind) string {
	switch kind {
	case KindFlask:
		return "材料 / MATERIAL"
	case KindPlate:
		return "仪器 / INSTRUMENT"
	default:
		return "操作 / OPERATION"
	}
}

func drawHero(p *ui.Painter, kind NodeKind, x, y, scale float32, color ui.Color) {
	var path ui.Path
	switch kind {
	case KindFlask:
		path.MoveTo(x+10*scale, y).LineTo(x+24*scale, y).LineTo(x+24*scale, y+5*scale).
			LineTo(x+21*scale, y+8*scale).LineTo(x+21*scale, y+17*scale).
			LineTo(x+31*scale, y+34*scale).LineTo(x+3*scale, y+34*scale).
			LineTo(x+13*scale, y+17*scale).LineTo(x+13*scale, y+8*scale).
			LineTo(x+10*scale, y+5*scale).Close()
		path.MoveTo(x+8*scale, y+26*scale).LineTo(x+26*scale, y+26*scale)
	case KindPlate:
		path.MoveTo(x+2*scale, y+4*scale).LineTo(x+32*scale, y+4*scale).
			LineTo(x+32*scale, y+32*scale).LineTo(x+2*scale, y+32*scale).Close()
		path.MoveTo(x+9*scale, y+11*scale).LineTo(x+25*scale, y+11*scale).
			LineTo(x+25*scale, y+25*scale).LineTo(x+9*scale, y+25*scale).Close()
	default:
		path.MoveTo(x+28*scale, y+2*scale).LineTo(x+31*scale, y+5*scale).
			LineTo(x+14*scale, y+22*scale).LineTo(x+10*scale, y+18*scale).Close()
		path.MoveTo(x+8*scale, y+20*scale).LineTo(x+2*scale, y+33*scale).
			LineTo(x+15*scale, y+27*scale)
	}
	p.StrokePath(&path, 2*scale, color)
}

func nodeSummary(kind NodeKind) string {
	switch kind {
	case KindFlask:
		return "细胞悬液 · 1×10⁴ cells/mL"
	case KindPlate:
		return "OD450 · 96 孔板"
	default:
		return "100 µL/孔 · 轻柔混匀"
	}
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
