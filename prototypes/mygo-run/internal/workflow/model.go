package workflow

import (
	"fmt"
)

type NodeKind string

const (
	KindFlask   NodeKind = "flask"
	KindPlate   NodeKind = "plate"
	KindPipette NodeKind = "pipette"
)

type Position struct {
	X, Y float32
}

type Node struct {
	ID    string
	Kind  NodeKind
	Title string
	Pos   Position
	// ComponentRefs are block/component definition ids attached to node.
	ComponentRefs []string
}

type Edge struct {
	ID   string
	From string
	To   string
}

// BusinessOrder is protocol/business order, independent of layout positions.
type WorkflowDocument struct {
	Nodes         []Node
	Edges         []Edge
	BusinessOrder []string
	NodeSeq       uint64
	EdgeSeq       uint64
}

func (d *WorkflowDocument) nextNodeID() string {
	d.NodeSeq++
	return fmt.Sprintf("n%d", d.NodeSeq)
}

func (d *WorkflowDocument) nextEdgeID() string {
	d.EdgeSeq++
	return fmt.Sprintf("e%d", d.EdgeSeq)
}

func (d WorkflowDocument) clone() WorkflowDocument {
	out := d
	out.Nodes = append([]Node(nil), d.Nodes...)
	out.Edges = append([]Edge(nil), d.Edges...)
	out.BusinessOrder = append([]string(nil), d.BusinessOrder...)
	for i := range out.Nodes {
		out.Nodes[i].ComponentRefs = append([]string(nil), d.Nodes[i].ComponentRefs...)
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
	// edges
	filtered := d.Edges[:0]
	for _, e := range d.Edges {
		if e.From != id && e.To != id {
			filtered = append(filtered, e)
		}
	}
	d.Edges = filtered
	// business order
	bo := d.BusinessOrder[:0]
	for _, x := range d.BusinessOrder {
		if x != id {
			bo = append(bo, x)
		}
	}
	d.BusinessOrder = bo
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
	n.Title = n.Title + " copy"
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
	Doc           WorkflowDocument
	SelectedNode  string
	SelectedEdge  string
	UndoStack     []WorkflowDocument
	RedoStack     []WorkflowDocument
	PanX, PanY    float32
	Zoom          float32
	ConnectMode   bool
	ConnectFrom   string
	OnSelect      func(nodeID string)
	OnEnterNested func(nodeID string)
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
	return id
}

func (e *Editor) DeleteSelected() bool {
	if e.SelectedNode == "" && e.SelectedEdge == "" {
		return false
	}
	e.pushUndo()
	ok := false
	if e.SelectedNode != "" {
		ok = e.Doc.DeleteNode(e.SelectedNode)
		e.SelectedNode = ""
	} else {
		ok = e.Doc.DeleteEdge(e.SelectedEdge)
		e.SelectedEdge = ""
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
	return id
}

func (e *Editor) Connect(from, to string) string {
	e.pushUndo()
	return e.Doc.AddEdge(from, to)
}

func (e *Editor) MoveNode(id string, pos Position) bool {
	e.pushUndo()
	return e.Doc.MoveNode(id, pos)
}

func (e *Editor) SelectNode(id string) {
	e.SelectedNode = id
	e.SelectedEdge = ""
	if e.OnSelect != nil {
		e.OnSelect(id)
	}
}

func (e *Editor) ClearSelection() {
	e.SelectedNode = ""
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
	return true
}

func (e *Editor) EnterNested(nodeID string) {
	if e.OnEnterNested != nil {
		e.OnEnterNested(nodeID)
	}
}