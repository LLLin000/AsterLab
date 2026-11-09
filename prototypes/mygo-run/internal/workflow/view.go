package workflow

import (
	"fmt"
	"math"

	"github.com/egoist/mygo/ui"
)

func (e *Editor) View(c *ui.Context) {
	t := c.Theme()
	z := e.zoomOr1()

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

	ui.Box(c).Fill().Background(ui.Hex("#0B0C0E")).Draw(func(p *ui.Painter, r ui.Rect) {
		// 1. Draw dot grid (20px spacing)
		dotColor := ui.Hex("#1C212B")
		step := float32(20) * z
		startX := float32(math.Mod(float64(e.PanX*z), float64(step)))
		startY := float32(math.Mod(float64(e.PanY*z), float64(step)))
		for x := r.X + startX; x < r.X+r.W; x += step {
			for y := r.Y + startY; y < r.Y+r.H; y += step {
				p.Fill(ui.Rect{X: x, Y: y, W: 1.5, H: 1.5}, dotColor, 0.75)
			}
		}

		// 2. Draw Bezier connections between nodes
		nodeW := float32(182) * z
		nodeH := float32(88) * z
		for _, edge := range e.Doc.Edges {
			from, to := e.Doc.findNodePtr(edge.From), e.Doc.findNodePtr(edge.To)
			if from == nil || to == nil {
				continue
			}
			x0 := r.X + (from.Pos.X+e.PanX)*z + nodeW
			y0 := r.Y + (from.Pos.Y+e.PanY)*z + nodeH/2
			x1 := r.X + (to.Pos.X+e.PanX)*z
			y1 := r.Y + (to.Pos.Y+e.PanY)*z + nodeH/2

			dx := (x1 - x0) * 0.5
			var path ui.Path
			path.MoveTo(x0, y0).CubeTo(x0+dx, y0, x1-dx, y1, x1, y1)

			edgeColor := ui.Hex("#2D333F")
			lineWidth := float32(1.75) * z
			if to.ID == e.SelectedNode || from.ID == e.SelectedNode {
				edgeColor = ui.Hex("#4B88FF")
				lineWidth = 2.2 * z
			}
			p.StrokePath(&path, lineWidth, edgeColor)

			// Midpoint pulse indicator
			midX := (x0 + x1) * 0.5
			midY := (y0 + y1) * 0.5
			dotC := ui.Hex("#5A606F")
			if to.ID == e.SelectedNode {
				dotC = ui.Hex("#4B88FF")
			}
			p.Fill(ui.Rect{X: midX - 2.5*z, Y: midY - 2.5*z, W: 5 * z, H: 5 * z}, dotC, 2.5*z)
		}
	}).Children(func() {
		// 3. Floating Command Strip (Raycast / Linear style pill centered horizontally)
		w, _ := c.Size()
		pillLeft := float32(260)
		if w > 700 {
			pillLeft = (w - 48 - 560) * 0.5
			if pillLeft < 80 {
				pillLeft = 80
			}
		}
		ui.Row(c).Absolute().Top(14).Left(pillLeft).Height(34).Radius(17).Background(ui.Hex("#16191F")).Border(1, ui.Hex("#2A313D")).Padding(0, 10).Gap(4).AlignItems(ui.Center).Children(func() {
			if ui.Button(c, "＋ 材料").Radius(12).Clicked() {
				e.AddNode(KindFlask, "细胞悬液", Position{X: 60 + float32(len(e.Doc.Nodes)*220), Y: 70})
			}
			if ui.Button(c, "＋ 操作").Radius(12).Clicked() {
				e.AddNode(KindPipette, "加样操作", Position{X: 60 + float32(len(e.Doc.Nodes)*220), Y: 70})
			}
			if ui.Button(c, "＋ 仪器").Radius(12).Clicked() {
				e.AddNode(KindPlate, "酶标仪读板", Position{X: 60 + float32(len(e.Doc.Nodes)*220), Y: 70})
			}
			ui.Divider(c)
			btnConnect := ui.Button(c, "连线").Radius(12)
			if e.ConnectMode {
				btnConnect.Background(ui.Hex("#4B88FF"))
			}
			if btnConnect.Clicked() {
				e.ConnectMode = !e.ConnectMode
				if !e.ConnectMode {
					e.ConnectFrom = ""
				}
			}
			if ui.Button(c, "撤销").Radius(12).Clicked() {
				e.Undo()
			}
			if ui.Button(c, "重做").Radius(12).Clicked() {
				e.Redo()
			}
			ui.Divider(c)
			if ui.Button(c, "自动对齐").Radius(12).Clicked() {
				for i := range e.Doc.Nodes {
					e.Doc.Nodes[i].Pos = Position{X: 60 + float32(i*220), Y: 70}
				}
				e.PanX, e.PanY, e.Zoom = 0, 0, 1
			}
			ui.Text(c, fmt.Sprintf("%.0f%%", z*100)).FontSize(11).TextColor(ui.Hex("#9DA3AF"))
			if ui.Button(c, "适应").Radius(12).Clicked() {
				e.PanX, e.PanY, e.Zoom = 0, 0, 1
			}
			ui.Text(c, "命令 ⌘K").FontSize(10).TextColor(ui.Hex("#5A606F"))
		})

		// 4. DAG Node Cards
		nodeW := float32(182) * z
		nodeH := float32(88) * z
		for i := range e.Doc.Nodes {
			n := &e.Doc.Nodes[i]
			off := ui.Point{X: (n.Pos.X + e.PanX) * z, Y: (n.Pos.Y + e.PanY) * z}
			selected := n.ID == e.SelectedNode
			nodeID := n.ID
			order := n.Order
			if order == "" {
				order = fmt.Sprintf("%02d", i+1)
			}
			tag := n.Tag
			if tag == "" {
				tag = nodeKindTag(n.Kind)
			}
			title := n.Title
			kind := n.Kind
			params := n.Params

			nodeElement := ui.Box(c).Absolute().Left(off.X).Top(off.Y).Size(nodeW, nodeH).Draw(func(p *ui.Painter, r ui.Rect) {
				// Base Card
				p.Fill(r, ui.Hex("#161920"), 8*z)
				borderColor := ui.Hex("#262D3A")
				borderWidth := float32(1)
				if selected {
					borderColor = ui.Hex("#4B88FF")
					borderWidth = 1.5 * z
				}
				p.Stroke(r, borderColor, 8*z, borderWidth)

				// Header Tag and Order
				tagColor := ui.Hex("#5A606F")
				if selected {
					tagColor = ui.Hex("#4B88FF")
				}
				p.Text(r.X+10*z, r.Y+14*z, tag, 9*z, tagColor)
				p.Text(r.X+nodeW-24*z, r.Y+14*z, order, 9*z, ui.Hex("#5A606F"))

				// Icon Box and Title
				iconBox := ui.Rect{X: r.X + 10*z, Y: r.Y + 20*z, W: 22 * z, H: 22 * z}
				p.Fill(iconBox, ui.Hex("#1E232C"), 5*z)
				p.Stroke(iconBox, ui.Hex("#2A313D"), 5*z, 1)
				drawHero(p, kind, iconBox.X+2*z, iconBox.Y+2*z, z*0.55, ui.Hex("#C8CDD8"))

				p.Text(r.X+36*z, r.Y+35*z, title, 12*z, ui.Hex("#F0F2F5"))

				// Divider Line
				p.Line(r.X+10*z, r.Y+48*z, r.X+nodeW-10*z, r.Y+48*z, 1, ui.Hex("#1E232C"))

				// Meta Key-Value Rows
				if len(params) > 0 {
					yOffset := r.Y + 60*z
					for idx, param := range params {
						if idx >= 2 {
							break
						}
						p.Text(r.X+10*z, yOffset, param.Key, 8.5*z, ui.Hex("#5A606F"))
						p.Text(r.X+60*z, yOffset, param.Val, 8.5*z, ui.Hex("#9DA3AF"))
						yOffset += 12 * z
					}
				} else {
					p.Text(r.X+10*z, r.Y+62*z, nodeSummary(kind), 9*z, ui.Hex("#9DA3AF"))
				}

				// Left and Right Connection Ports
				portRadius := float32(4) * z
				p.Fill(ui.Rect{X: r.X - portRadius, Y: r.Y + nodeH/2 - portRadius, W: portRadius * 2, H: portRadius * 2}, ui.Hex("#161920"), portRadius)
				portBorder := ui.Hex("#5A606F")
				if selected {
					portBorder = ui.Hex("#4B88FF")
				}
				p.Stroke(ui.Rect{X: r.X - portRadius, Y: r.Y + nodeH/2 - portRadius, W: portRadius * 2, H: portRadius * 2}, portBorder, portRadius, 1.5)

				p.Fill(ui.Rect{X: r.X + nodeW - portRadius, Y: r.Y + nodeH/2 - portRadius, W: portRadius * 2, H: portRadius * 2}, ui.Hex("#161920"), portRadius)
				p.Stroke(ui.Rect{X: r.X + nodeW - portRadius, Y: r.Y + nodeH/2 - portRadius, W: portRadius * 2, H: portRadius * 2}, portBorder, portRadius, 1.5)
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
	_ = t
}

func nodeKindTag(kind NodeKind) string {
	switch kind {
	case KindFlask:
		return "[材料 · 样本]"
	case KindPlate:
		return "[分析仪器]"
	default:
		return "[操作步骤]"
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
