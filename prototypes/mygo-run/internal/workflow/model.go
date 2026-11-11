package workflow

import (
	"fmt"

	"github.com/oneo-me/onevra/source/components/gesture"
)

type NodeKind string

const (
	KindFlask   NodeKind = "flask"
	KindPlate   NodeKind = "plate"
	KindPipette NodeKind = "pipette"
)

type Position struct{ X, Y float32 }

// NodeWidth/NodeHeight mirror workflownode's unscaled card size for hit
// testing and marquee selection. The canvas scales them at render time.
const (
	NodeWidth  float32 = 224
	NodeHeight float32 = 128
)

type NodeParam struct {
	Key string `json:"key"`
	Val string `json:"val"`
}

type Node struct {
	ID            string   `json:"id"`
	Kind          NodeKind `json:"kind"`
	Title         string   `json:"title"`
	Pos           Position `json:"pos"`
	ComponentRefs []string `json:"component_refs,omitempty"`
	Order         string   `json:"order,omitempty"`
	Tag           string   `json:"tag,omitempty"`
	// Composite marks a design step that references a ProtocolBlock
	// definition. ChildCount is the derived number of definition operations.
	Composite  bool        `json:"composite,omitempty"`
	ChildCount int         `json:"child_count,omitempty"`
	Params     []NodeParam `json:"params,omitempty"`
}

type Edge struct{ ID, From, To string }

// BusinessOrder is protocol/business order, independent of layout positions.
type WorkflowDocument struct {
	Nodes         []Node
	Edges         []Edge
	BusinessOrder []string
	NodeSeq       uint64
	EdgeSeq       uint64
}

func (d *WorkflowDocument) nextNodeID() string { d.NodeSeq++; return fmt.Sprintf("n%d", d.NodeSeq) }
func (d *WorkflowDocument) nextEdgeID() string { d.EdgeSeq++; return fmt.Sprintf("e%d", d.EdgeSeq) }

func (d WorkflowDocument) clone() WorkflowDocument {
	out := d
	out.Nodes = append([]Node(nil), d.Nodes...)
	out.Edges = append([]Edge(nil), d.Edges...)
	out.BusinessOrder = append([]string(nil), d.BusinessOrder...)
	for i := range out.Nodes {
		out.Nodes[i].ComponentRefs = append([]string(nil), d.Nodes[i].ComponentRefs...)
		out.Nodes[i].Params = append([]NodeParam(nil), d.Nodes[i].Params...)
	}
	return out
}

func (d *WorkflowDocument) findNode(id string) int {
	for i := range d.Nodes {
		if d.Nodes[i].ID == id {
			return i
		}
	}
	return -1
}

func (d *WorkflowDocument) AddNode(kind NodeKind, title string, pos Position) string {
	id := d.nextNodeID()
	d.Nodes = append(d.Nodes, Node{ID: id, Kind: kind, Title: title, Pos: pos})
	d.BusinessOrder = append(d.BusinessOrder, id)
	return id
}

func (d *WorkflowDocument) DeleteNode(id string) bool {
	i := d.findNode(id)
	if i < 0 {
		return false
	}
	d.Nodes = append(d.Nodes[:i], d.Nodes[i+1:]...)
	filtered := d.Edges[:0]
	for _, edge := range d.Edges {
		if edge.From != id && edge.To != id {
			filtered = append(filtered, edge)
		}
	}
	d.Edges = filtered
	order := d.BusinessOrder[:0]
	for _, nodeID := range d.BusinessOrder {
		if nodeID != id {
			order = append(order, nodeID)
		}
	}
	d.BusinessOrder = order
	return true
}

func (d *WorkflowDocument) DuplicateNode(id string) string {
	i := d.findNode(id)
	if i < 0 {
		return ""
	}
	n := d.Nodes[i]
	n.ID = d.nextNodeID()
	n.Pos.X += 24
	n.Pos.Y += 24
	n.Title += " copy"
	d.Nodes = append(d.Nodes, n)
	d.BusinessOrder = append(d.BusinessOrder, n.ID)
	return n.ID
}

func (d *WorkflowDocument) MoveNode(id string, pos Position) bool {
	i := d.findNode(id)
	if i < 0 {
		return false
	}
	d.Nodes[i].Pos = pos
	return true
}

func (d *WorkflowDocument) AddEdge(from, to string) string {
	if d.findNode(from) < 0 || d.findNode(to) < 0 || from == to {
		return ""
	}
	id := d.nextEdgeID()
	d.Edges = append(d.Edges, Edge{ID: id, From: from, To: to})
	return id
}

func (d *WorkflowDocument) DeleteEdge(id string) bool {
	for i := range d.Edges {
		if d.Edges[i].ID == id {
			d.Edges = append(d.Edges[:i], d.Edges[i+1:]...)
			return true
		}
	}
	return false
}

type Editor struct {
	Doc              WorkflowDocument
	SelectedNode     string
	SelectedNodes    map[string]bool
	SelectedEdge     string
	HoveredNode      string
	UndoStack        []WorkflowDocument
	RedoStack        []WorkflowDocument
	PanX, PanY       float32
	Zoom             float32
	ConnectMode      bool
	ConnectFrom      string
	MovingNode       string
	MoveStart        WorkflowDocument
	MoveOrigins      map[string]Position
	MarqueeBase      map[string]bool
	MarqueeActive    bool
	MarqueeStartX    float32
	MarqueeStartY    float32
	MarqueeCurrentX  float32
	MarqueeCurrentY  float32
	MarqueeAdditive  bool
	BeforeDelete     func(nodeID string) bool
	OnSelect         func(nodeID string)
	OnEnterNested    func(nodeID string)
	OnChange         func()
	OnCommandPalette func()
	// ExternalWrites tells View not to bind delete/undo shortcuts that would
	// mutate this local graph without the document command layer.
	ExternalWrites bool
	// ReadOnly disables layout drags in derived views such as a ProtocolBlock
	// definition. Selection remains available.
	ReadOnly bool
	// Navigation turns wheel/pinch/two-finger scroll into canvas pan and zoom.
	Navigation gesture.Navigation
}

func NewEditor() *Editor {
	return &Editor{Zoom: 1}
}

func (e *Editor) pushUndo() {
	e.UndoStack = append(e.UndoStack, e.Doc.clone())
	if len(e.UndoStack) > 100 {
		e.UndoStack = e.UndoStack[1:]
	}
	e.RedoStack = nil
}

func (e *Editor) AddNode(kind NodeKind, title string, pos Position) string {
	e.pushUndo()
	id := e.Doc.AddNode(kind, title, pos)
	e.SelectedNode = id
	e.SelectedEdge = ""
	if e.OnSelect != nil {
		e.OnSelect(id)
	}
	if e.OnChange != nil {
		e.OnChange()
	}
	return id
}

func (e *Editor) DeleteSelected() bool {
	ids := e.SelectedIDs()
	if len(ids) == 0 && e.SelectedEdge == "" {
		return false
	}
	for _, id := range ids {
		if e.BeforeDelete != nil && !e.BeforeDelete(id) {
			return false
		}
	}
	e.pushUndo()
	ok := false
	if len(ids) > 0 {
		for _, id := range ids {
			ok = e.Doc.DeleteNode(id) || ok
		}
		e.ClearSelection()
	} else {
		ok = e.Doc.DeleteEdge(e.SelectedEdge)
		e.SelectedEdge = ""
	}
	if ok && e.OnChange != nil {
		e.OnChange()
	}
	return ok
}

func (e *Editor) DuplicateSelected() string {
	if e.SelectedNode == "" {
		return ""
	}
	e.pushUndo()
	id := e.Doc.DuplicateNode(e.SelectedNode)
	e.SelectedNode = id
	if e.OnSelect != nil {
		e.OnSelect(id)
	}
	if id != "" && e.OnChange != nil {
		e.OnChange()
	}
	return id
}

func (e *Editor) Connect(from, to string) string {
	if e.Doc.findNode(from) < 0 || e.Doc.findNode(to) < 0 || from == to {
		return ""
	}
	e.pushUndo()
	id := e.Doc.AddEdge(from, to)
	if id != "" && e.OnChange != nil {
		e.OnChange()
	}
	return id
}

func (e *Editor) MoveNode(id string, pos Position) bool {
	i := e.Doc.findNode(id)
	if i < 0 || e.Doc.Nodes[i].Pos == pos {
		return false
	}
	e.pushUndo()
	e.Doc.Nodes[i].Pos = pos
	if e.OnChange != nil {
		e.OnChange()
	}
	return true
}
func (e *Editor) dragNode(id string, pos Position, dragging bool) {
	if dragging {
		if e.MovingNode != id {
			if !e.selectedSet()[id] {
				e.SelectNode(id)
			}
			e.MovingNode, e.MoveStart = id, e.Doc.clone()
			e.MoveOrigins = map[string]Position{}
			for _, selected := range e.SelectedIDs() {
				if i := e.Doc.findNode(selected); i >= 0 {
					e.MoveOrigins[selected] = e.Doc.Nodes[i].Pos
				}
			}
		}
		origin := e.MoveOrigins[id]
		dx, dy := pos.X-origin.X, pos.Y-origin.Y
		for nodeID, start := range e.MoveOrigins {
			e.Doc.MoveNode(nodeID, Position{X: start.X + dx, Y: start.Y + dy})
		}
		return
	}
	if e.MovingNode == id {
		changed := false
		for nodeID, start := range e.MoveOrigins {
			if i := e.Doc.findNode(nodeID); i >= 0 && e.Doc.Nodes[i].Pos != start {
				changed = true
			}
		}
		if changed {
			e.UndoStack = append(e.UndoStack, e.MoveStart)
			if len(e.UndoStack) > 100 {
				e.UndoStack = e.UndoStack[1:]
			}
			e.RedoStack = nil
			if e.OnChange != nil {
				e.OnChange()
			}
		}
		e.MovingNode, e.MoveStart = "", WorkflowDocument{}
		e.MoveOrigins = nil
		return
	}
	e.MoveNode(id, pos)
}

func (e *Editor) SelectNode(id string) {
	e.SelectedNode = id
	e.SelectedNodes = nil
	if id != "" {
		e.SelectedNodes = map[string]bool{id: true}
	}
	e.SelectedEdge = ""
	if e.OnSelect != nil {
		e.OnSelect(id)
	}
}

// ToggleNode adds or removes one node from a multi-selection. The last
// toggled node stays the primary selection for the inspector.
func (e *Editor) ToggleNode(id string) {
	if e.SelectedNodes == nil {
		e.SelectedNodes = map[string]bool{}
		if e.SelectedNode != "" {
			e.SelectedNodes[e.SelectedNode] = true
		}
	}
	if e.SelectedNodes[id] {
		delete(e.SelectedNodes, id)
		if e.SelectedNode == id {
			e.SelectedNode = ""
			for _, node := range e.Doc.Nodes {
				if e.SelectedNodes[node.ID] {
					e.SelectedNode = node.ID
					break
				}
			}
		}
	} else {
		e.SelectedNodes[id] = true
		e.SelectedNode = id
	}
	e.SelectedEdge = ""
	if e.OnSelect != nil {
		e.OnSelect(e.SelectedNode)
	}
}

func (e *Editor) selectedSet() map[string]bool {
	if len(e.SelectedNodes) == 0 {
		out := map[string]bool{}
		if e.SelectedNode != "" {
			out[e.SelectedNode] = true
		}
		return out
	}
	return e.SelectedNodes
}

// SelectedIDs returns selected node IDs in document order.
func (e *Editor) SelectedIDs() []string {
	selected := e.selectedSet()
	out := make([]string, 0, len(selected))
	for _, node := range e.Doc.Nodes {
		if selected[node.ID] {
			out = append(out, node.ID)
		}
	}
	return out
}

// SelectInBounds replaces or extends the selection with nodes whose card
// bounds intersect x/y/w/h in document coordinates.
func (e *Editor) SelectInBounds(x, y, w, h float32, additive bool) {
	if w < 0 {
		x += w
		w = -w
	}
	if h < 0 {
		y += h
		h = -h
	}
	next := map[string]bool{}
	if additive {
		for id := range e.selectedSet() {
			next[id] = true
		}
	}
	for _, node := range e.Doc.Nodes {
		if node.Pos.X < x+w && node.Pos.X+NodeWidth > x &&
			node.Pos.Y < y+h && node.Pos.Y+NodeHeight > y {
			next[node.ID] = true
		}
	}
	e.SelectedNodes = next
	e.SelectedNode = ""
	for _, node := range e.Doc.Nodes {
		if next[node.ID] {
			e.SelectedNode = node.ID
			break
		}
	}
	e.SelectedEdge = ""
	if e.OnSelect != nil {
		e.OnSelect(e.SelectedNode)
	}
}

func (e *Editor) ClearSelection() {
	e.SelectedNode = ""
	e.SelectedNodes = nil
	e.SelectedEdge = ""
	if e.OnSelect != nil {
		e.OnSelect("")
	}
}

func (e *Editor) Undo() bool {
	if len(e.UndoStack) == 0 {
		return false
	}
	e.RedoStack = append(e.RedoStack, e.Doc.clone())
	last := e.UndoStack[len(e.UndoStack)-1]
	e.UndoStack = e.UndoStack[:len(e.UndoStack)-1]
	e.Doc = last
	if e.OnChange != nil {
		e.OnChange()
	}
	return true
}

func (e *Editor) Redo() bool {
	if len(e.RedoStack) == 0 {
		return false
	}
	e.UndoStack = append(e.UndoStack, e.Doc.clone())
	last := e.RedoStack[len(e.RedoStack)-1]
	e.RedoStack = e.RedoStack[:len(e.RedoStack)-1]
	e.Doc = last
	if e.OnChange != nil {
		e.OnChange()
	}
	return true
}

func (e *Editor) EnterNested(nodeID string) {
	if e.OnEnterNested != nil {
		e.OnEnterNested(nodeID)
	}
}
