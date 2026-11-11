package workflow

import (
	"fmt"
	"math"

	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/onevra/source/components"
	"github.com/oneo-me/onevra/source/components/gesture"
	"github.com/oneo-me/onevra/source/icons/lucide"

	"asterlab-mygo-prototype/internal/uikit/workflownode"
)

func (e *Editor) View(c *ui.Context) {
	t := c.Theme()

	if !e.ExternalWrites {
		c.OnShortcut(0, ui.KeyDelete, func() { e.DeleteSelected() })
	}
	c.OnShortcut(ui.Shift, ui.KeyLeft, func() { e.PanX += 32 })
	c.OnShortcut(ui.Shift, ui.KeyRight, func() { e.PanX -= 32 })
	c.OnShortcut(ui.Shift, ui.KeyUp, func() { e.PanY += 32 })
	c.OnShortcut(ui.Shift, ui.KeyDown, func() { e.PanY -= 32 })
	c.OnShortcut(0, ui.KeyEscape, func() {
		e.ConnectMode, e.ConnectFrom = false, ""
		e.ClearSelection()
	})
	if !e.ExternalWrites {
		c.OnShortcut(ui.Cmd, ui.KeyZ, func() { e.Undo() })
		c.OnShortcut(ui.Cmd, ui.KeyY, func() { e.Redo() })
	}

	ui.Box(c).Fill().Background(t.Background).Children(func() {
		canvas(c, e, t)
	})
}

// canvas draws the workflow surface: a dot grid, the edges between steps, and
// one card per step. Steps keep their business order, never their position.
func canvas(c *ui.Context, e *Editor, t *ui.Theme) {
	z := e.zoomOr1()
	root := ui.Box(c).Fill().Draw(func(p *ui.Painter, r ui.Rect) {
		dot := t.Text.Alpha(.14)
		step := float32(20) * z
		startX := float32(math.Mod(float64(e.PanX*z), float64(step)))
		startY := float32(math.Mod(float64(e.PanY*z), float64(step)))
		for x := r.X + startX; x < r.X+r.W; x += step {
			for y := r.Y + startY; y < r.Y+r.H; y += step {
				p.Fill(ui.Rect{X: x, Y: y, W: 1.5, H: 1.5}, dot, 0.75)
			}
		}

		nodeW := workflownode.CardWidth * z
		nodeH := workflownode.CardHeight * z
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

			edgeColor := t.Text.Alpha(.22)
			lineWidth := float32(1.4) * z
			if to.ID == e.SelectedNode || from.ID == e.SelectedNode {
				edgeColor = t.Accent
				lineWidth = 2 * z
			}
			p.StrokePath(&path, lineWidth, edgeColor)
		}

		// Port nubs sit on each card's border so a connection reads as a join.
		for _, node := range e.Doc.Nodes {
			x := r.X + (node.Pos.X+e.PanX)*z
			y := r.Y + (node.Pos.Y+e.PanY)*z + nodeH/2
			for _, px := range []float32{x, x + nodeW} {
				port := ui.Rect{X: px - 3.5*z, Y: y - 3.5*z, W: 7 * z, H: 7 * z}
				p.Fill(port, t.Surface, 3.5*z)
				p.Stroke(port, t.Border, 3.5*z, z)
			}
		}
	}).Children(func() {
		if len(e.Doc.Nodes) == 0 {
			emptyState(c, t)
		}

		nodeW := workflownode.CardWidth * z
		nodeH := workflownode.CardHeight * z
		hovered := ""
		for i := range e.Doc.Nodes {
			n := &e.Doc.Nodes[i]
			off := ui.Point{X: (n.Pos.X + e.PanX) * z, Y: (n.Pos.Y + e.PanY) * z}
			order := n.Order
			if order == "" {
				order = fmt.Sprintf("%02d", i+1)
			}
			state := workflownode.StateDefault
			switch {
			case e.selectedSet()[n.ID]:
				state = workflownode.StateSelected
			case n.ID == e.HoveredNode:
				state = workflownode.StateHover
			}
			props := adaptWorkflowNode(*n, order, state)
			nodeID := n.ID
			nodeElement := ui.Box(c).Absolute().Left(off.X).Top(off.Y).Size(nodeW, nodeH).
				Children(func() {
					workflownode.View(c, props, z, func() { e.activateNode(nodeID, c.Modifiers()) })
				})
			if !e.ReadOnly {
				nodeElement = nodeElement.Draggable(&off, ui.DraggableOptions{})
			}
			nodeElement = nodeElement.Key(nodeID)
			if nodeElement.Hovered() {
				hovered = nodeID
			}

			pos := Position{X: off.X/z - e.PanX, Y: off.Y/z - e.PanY}
			if pos != n.Pos || e.MovingNode == nodeID {
				e.dragNode(nodeID, pos, nodeElement.Dragging())
			}
		}
		e.HoveredNode = hovered
	}).DrawOver(func(p *ui.Painter, r ui.Rect) {
		if !e.MarqueeActive {
			return
		}
		x0 := min32(e.MarqueeStartX, e.MarqueeCurrentX) + r.X
		y0 := min32(e.MarqueeStartY, e.MarqueeCurrentY) + r.Y
		w := abs32(e.MarqueeCurrentX - e.MarqueeStartX)
		h := abs32(e.MarqueeCurrentY - e.MarqueeStartY)
		rect := ui.Rect{X: x0, Y: y0, W: w, H: h}
		p.Fill(rect, t.Accent.Alpha(.12), 0)
		p.Stroke(rect, t.Accent, 1, 0)
	})
	e.Navigation.Handle(c, root, func(event gesture.Event) {
		z := e.zoomOr1()
		switch event.Kind {
		case gesture.Pan:
			e.PanX += event.DX / z
			e.PanY += event.DY / z
		case gesture.Zoom:
			if event.Scale <= 0 {
				return
			}
			next := z * float32(event.Scale)
			if next < 0.35 {
				next = 0.35
			}
			if next > 2.5 {
				next = 2.5
			}
			if next == z {
				return
			}
			e.PanX += event.X * (1/next - 1/z)
			e.PanY += event.Y * (1/next - 1/z)
			e.Zoom = next
		}
	}, func(event ui.InputEvent) bool { return e.handleMarqueeInput(event) })
}

// emptyState explains the canvas without inventing a design.
func emptyState(c *ui.Context, t *ui.Theme) {
	ui.Box(c).Absolute().Left(0).Right(0).Top(0).Bottom(0).Center().Children(func() {
		ui.Column(c).Width(360).AlignItems(ui.Center).Gap(t.Space(1)).Children(func() {
			components.Icon(c, lucide.Workflow).Size(t.Space(7), t.Space(7)).TextColor(t.TextMuted)
			components.Text(c, "空白工作流").FontWeight(600)
			components.Text(c, "从上方添加材料、操作或仪器节点，再用连线表达步骤顺序。").
				TextColor(t.TextMuted).FontSize(t.Rem(.85)).TextAlign(ui.Center).FillWidth()
		})
	})
}

func adaptWorkflowNode(n Node, order string, state workflownode.NodeState) workflownode.NodeProps {
	kind := workflownode.KindOperation
	switch n.Kind {
	case KindFlask:
		kind = workflownode.KindMaterial
	case KindPlate:
		kind = workflownode.KindInstrument
	}
	params := make([]workflownode.ParamItem, len(n.Params))
	for i, param := range n.Params {
		params[i] = workflownode.ParamItem{Key: param.Key, Val: param.Val}
	}
	return workflownode.NodeProps{
		ID: n.ID, Kind: kind, Order: order, Tag: n.Tag, Title: n.Title,
		Composite: n.Composite, ChildCount: n.ChildCount,
		Params: params, State: state,
	}
}

// handleMarqueeInput owns empty-canvas drag selection. It is passed through
// gesture.Navigation because Navigation installs the root's single input
// handler; a second HandleInput would replace it.
func (e *Editor) handleMarqueeInput(event ui.InputEvent) bool {
	if event.Button != 0 {
		return false
	}
	switch event.Kind {
	case ui.InputPointerDown:
		if e.ConnectMode {
			return false
		}
		z := e.zoomOr1()
		docX, docY := event.X/z-e.PanX, event.Y/z-e.PanY
		for _, node := range e.Doc.Nodes {
			if docX >= node.Pos.X && docX <= node.Pos.X+NodeWidth &&
				docY >= node.Pos.Y && docY <= node.Pos.Y+NodeHeight {
				return false
			}
		}
		e.MarqueeActive = true
		e.MarqueeStartX, e.MarqueeStartY = event.X, event.Y
		e.MarqueeCurrentX, e.MarqueeCurrentY = event.X, event.Y
		e.MarqueeAdditive = event.Mods&(ui.Shift|ui.Cmd) != 0
		e.MarqueeBase = map[string]bool{}
		for id := range e.selectedSet() {
			e.MarqueeBase[id] = true
		}
		return true
	case ui.InputPointerMove:
		if !e.MarqueeActive {
			return false
		}
		e.MarqueeCurrentX, e.MarqueeCurrentY = event.X, event.Y
		e.SelectedNodes = map[string]bool{}
		for id := range e.MarqueeBase {
			e.SelectedNodes[id] = true
		}
		z := e.zoomOr1()
		x0 := min32(e.MarqueeStartX, e.MarqueeCurrentX)/z - e.PanX
		y0 := min32(e.MarqueeStartY, e.MarqueeCurrentY)/z - e.PanY
		w := abs32(e.MarqueeCurrentX-e.MarqueeStartX) / z
		h := abs32(e.MarqueeCurrentY-e.MarqueeStartY) / z
		e.SelectInBounds(x0, y0, w, h, true)
		return true
	case ui.InputPointerUp:
		if !e.MarqueeActive {
			return false
		}
		e.MarqueeActive = false
		if event.Canceled {
			e.SelectedNodes = e.MarqueeBase
		} else if abs32(e.MarqueeCurrentX-e.MarqueeStartX) < 3 && abs32(e.MarqueeCurrentY-e.MarqueeStartY) < 3 {
			if !e.MarqueeAdditive {
				e.ClearSelection()
			}
		}
		e.MarqueeBase = nil
		return true
	}
	return false
}

// activateNode selects a step, or completes a connection when 连线 is on.
func (e *Editor) activateNode(id string, mods ui.Modifiers) {
	if !e.ConnectMode {
		if mods&(ui.Shift|ui.Cmd) != 0 {
			e.ToggleNode(id)
		} else {
			e.SelectNode(id)
		}
		return
	}
	if e.ConnectFrom == "" {
		e.ConnectFrom = id
		return
	}
	if e.ConnectFrom != id {
		e.Connect(e.ConnectFrom, id)
	}
	e.ConnectFrom = ""
	e.ConnectMode = false
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

func min32(a, b float32) float32 {
	if a < b {
		return a
	}
	return b
}
func abs32(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}
