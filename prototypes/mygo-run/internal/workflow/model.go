package workflow

import "fmt"

type NodeKind string

const (
	KindFlask   NodeKind = "flask"
	KindPlate   NodeKind = "plate"
	KindPipette NodeKind = "pipette"
)

type Position struct{ X, Y float32 }

type NodeParam struct {
	Key string `json:"key"`
	Val string `json:"val"`
}

type Node struct {
	ID            string      `json:"id"`
	Kind          NodeKind    `json:"kind"`
	Title         string      `json:"title"`
	Pos           Position    `json:"pos"`
	ComponentRefs []string    `json:"component_refs,omitempty"`
	Order         string      `json:"order,omitempty"`
	Tag           string      `json:"tag,omitempty"`
	Params        []NodeParam `json:"params,omitempty"`
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
	Doc           WorkflowDocument
	SelectedNode  string
	SelectedEdge  string
	UndoStack     []WorkflowDocument
	RedoStack     []WorkflowDocument
	PanX, PanY    float32
	Zoom          float32
	ConnectMode   bool
	ConnectFrom   string
	MovingNode    string
	MoveStart     WorkflowDocument
	OnSelect      func(nodeID string)
	OnEnterNested func(nodeID string)
	OnChange      func()
	OnCommandPalette func()
}

func NewEditor() *Editor {
	return &Editor{Zoom: 1}
}

func (d *WorkflowDocument) SeedStandardCCK8() {
	d.Nodes = []Node{
		{
			ID:    "n1",
			Kind:  KindFlask,
			Title: "细胞悬液准备",
			Order: "01",
			Tag:   "[材料 · 样本]",
			Pos:   Position{X: 60, Y: 70},
			Params: []NodeParam{
				{Key: "细胞系", Val: "L929 (成纤维)"},
				{Key: "密度", Val: "1.0×10⁵/mL"},
				{Key: "代数 / 活力", Val: "P4 · 97.4%"},
			},
		},
		{
			ID:    "n2",
			Kind:  KindPipette,
			Title: "96 孔板接种铺板",
			Order: "02",
			Tag:   "[操作步骤]",
			Pos:   Position{X: 280, Y: 70},
			Params: []NodeParam{
				{Key: "接种体积", Val: "100 µL/孔"},
				{Key: "目标密度", Val: "5,000 cells/孔"},
				{Key: "贴壁培养", Val: "37°C, 24h"},
			},
		},
		{
			ID:    "n3",
			Kind:  KindPipette,
			Title: "梯度药物暴露处理",
			Order: "03",
			Tag:   "[操作步骤]",
			Pos:   Position{X: 500, Y: 70},
			Params: []NodeParam{
				{Key: "药物受试物", Val: "ZnO-NP 浸提液"},
				{Key: "浓度跨度", Val: "0~100 µg/mL"},
				{Key: "梯度/复孔", Val: "6 组 · 4 复孔"},
			},
		},
		{
			ID:    "n4",
			Kind:  KindFlask,
			Title: "CCK-8 试剂孵育",
			Order: "04",
			Tag:   "[反应孵育]",
			Pos:   Position{X: 720, Y: 70},
			Params: []NodeParam{
				{Key: "加液量", Val: "10 µL/孔 (10%)"},
				{Key: "孵育条件", Val: "37°C 避光"},
				{Key: "显色时长", Val: "2.5 h"},
			},
		},
		{
			ID:    "n5",
			Kind:  KindPlate,
			Title: "酶标仪吸光度读板",
			Order: "05",
			Tag:   "[分析仪器 · 当前选中]",
			Pos:   Position{X: 940, Y: 70},
			Params: []NodeParam{
				{Key: "测定波长", Val: "OD450 nm"},
				{Key: "参比波长", Val: "650 nm (校正)"},
				{Key: "预处理", Val: "中速震板 5s"},
			},
		},
	}
	d.Edges = []Edge{
		{ID: "e1", From: "n1", To: "n2"},
		{ID: "e2", From: "n2", To: "n3"},
		{ID: "e3", From: "n3", To: "n4"},
		{ID: "e4", From: "n4", To: "n5"},
	}
	d.BusinessOrder = []string{"n1", "n2", "n3", "n4", "n5"}
	d.NodeSeq = 5
	d.EdgeSeq = 4
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
			e.MovingNode, e.MoveStart = id, e.Doc.clone()
		}
		e.Doc.MoveNode(id, pos)
		return
	}
	if e.MovingNode == id {
		e.Doc.MoveNode(id, pos)
		start, current := e.MoveStart.findNode(id), e.Doc.findNode(id)
		changed := start >= 0 && current >= 0 && e.MoveStart.Nodes[start].Pos != e.Doc.Nodes[current].Pos
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
		return
	}
	e.MoveNode(id, pos)
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
