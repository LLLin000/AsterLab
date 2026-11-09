package resultgrid

import (
	"encoding/csv"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
)

// Schema is the serializable description of a rectangular result table.
type Schema struct {
	ID           string            `json:"id"`
	Version      int               `json:"version"`
	AssayID      string            `json:"assayId"`
	Layout       string            `json:"layout"`
	RowLabels    []string          `json:"rowLabels"`
	ColumnLabels []string          `json:"columnLabels"`
	Fields       []Field           `json:"fields"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}
type Field struct {
	Key      string `json:"key"`
	Label    string `json:"label"`
	Unit     string `json:"unit"`
	DataType string `json:"dataType"`
	Required bool   `json:"required"`
}
type Cell struct {
	Row       int      `json:"row"`
	Column    int      `json:"column"`
	SampleID  string   `json:"sampleId"`
	Group     string   `json:"group"`
	Replicate int      `json:"replicate"`
	Field     string   `json:"field"`
	Value     *float64 `json:"value"`
	Status    string   `json:"status"`
}
type Selection struct{ StartRow, StartColumn, EndRow, EndColumn int }
type SaveState string

const (
	StateDraft  SaveState = "Draft"
	StateSaving SaveState = "Saving"
	StateSaved  SaveState = "Saved"
	StateError  SaveState = "Error"
)

type Options struct{ FullWidth bool }
type PastePreview struct {
	Cells         []Cell
	Overwrites    int
	Rows, Columns int
}
type historyEntry struct {
	key       string
	old, next Cell
}
type Grid struct {
	Schema           Schema
	Cells            []Cell
	Selection        Selection
	State            SaveState
	Error            string
	Preview          *PastePreview
	ConfirmOverwrite bool
	undo, redo       []historyEntry
	inputs           []string
}

func New(s Schema, cells []Cell) *Grid {
	g := &Grid{Schema: s, Cells: make([]Cell, len(s.RowLabels)*len(s.ColumnLabels)), inputs: make([]string, len(s.RowLabels)*len(s.ColumnLabels)), State: StateDraft}
	for r := range s.RowLabels {
		for c := range s.ColumnLabels {
			g.Cells[g.index(r, c)] = Cell{Row: r, Column: c, Status: "missing"}
			if len(s.Fields) > 0 {
				g.Cells[g.index(r, c)].Field = s.Fields[0].Key
			}
		}
	}
	for _, cell := range cells {
		if g.validCoord(cell.Row, cell.Column) {
			if cell.Status == "" {
				if cell.Value == nil {
					cell.Status = "missing"
				} else {
					cell.Status = "raw"
				}
			}
			g.Cells[g.index(cell.Row, cell.Column)] = cell
		}
	}
	for i, cell := range g.Cells {
		if cell.Value != nil {
			g.inputs[i] = strconv.FormatFloat(*cell.Value, 'g', -1, 64)
		}
	}
	return g
}
func (g *Grid) Rows() int                { return len(g.Schema.RowLabels) }
func (g *Grid) Columns() int             { return len(g.Schema.ColumnLabels) }
func (g *Grid) validCoord(r, c int) bool { return r >= 0 && r < g.Rows() && c >= 0 && c < g.Columns() }
func (g *Grid) index(r, c int) int       { return r*g.Columns() + c }
func (g *Grid) CellAt(r, c int) (Cell, bool) {
	if !g.validCoord(r, c) {
		return Cell{}, false
	}
	return g.Cells[g.index(r, c)], true
}
func (g *Grid) input(r, c int) *string { return &g.inputs[g.index(r, c)] }
func (g *Grid) SetCell(cell Cell) error {
	if !g.validCoord(cell.Row, cell.Column) {
		return fmt.Errorf("cell coordinate out of range")
	}
	if cell.Value != nil && (math.IsNaN(*cell.Value) || math.IsInf(*cell.Value, 0)) {
		return fmt.Errorf("value must be finite")
	}
	if cell.Value == nil {
		if cell.Status == "" {
			cell.Status = "missing"
		}
		if cell.Status != "missing" {
			return fmt.Errorf("empty value must have missing status")
		}
	} else if cell.Status == "" || cell.Status == "missing" {
		cell.Status = "raw"
	}
	idx := g.index(cell.Row, cell.Column)
	old := g.Cells[idx]
	key := fmt.Sprintf("%d:%d", cell.Row, cell.Column)
	g.undo = append(g.undo, historyEntry{key, old, cell})
	g.redo = nil
	g.Cells[idx] = cell
	if cell.Value == nil {
		g.inputs[idx] = ""
	} else {
		g.inputs[idx] = strconv.FormatFloat(*cell.Value, 'g', -1, 64)
	}
	g.State = StateDraft
	g.Error = ""
	return nil
}
func (g *Grid) Undo() bool {
	if len(g.undo) == 0 {
		return false
	}
	e := g.undo[len(g.undo)-1]
	g.undo = g.undo[:len(g.undo)-1]
	idx := g.index(e.old.Row, e.old.Column)
	g.Cells[idx] = e.old
	g.inputs[idx] = ""
	if e.old.Value != nil {
		g.inputs[idx] = strconv.FormatFloat(*e.old.Value, 'g', -1, 64)
	}
	g.redo = append(g.redo, e)
	g.State = StateDraft
	return true
}
func (g *Grid) Redo() bool {
	if len(g.redo) == 0 {
		return false
	}
	e := g.redo[len(g.redo)-1]
	g.redo = g.redo[:len(g.redo)-1]
	idx := g.index(e.next.Row, e.next.Column)
	g.Cells[idx] = e.next
	g.inputs[idx] = ""
	if e.next.Value != nil {
		g.inputs[idx] = strconv.FormatFloat(*e.next.Value, 'g', -1, 64)
	}
	g.undo = append(g.undo, e)
	g.State = StateDraft
	return true
}
func (g *Grid) SetSelection(s Selection) {
	if g.Rows() == 0 || g.Columns() == 0 {
		return
	}
	clamp := func(v, n int) int {
		if v < 0 {
			return 0
		}
		if v >= n {
			return n - 1
		}
		return v
	}
	s.StartRow = clamp(s.StartRow, g.Rows())
	s.EndRow = clamp(s.EndRow, g.Rows())
	s.StartColumn = clamp(s.StartColumn, g.Columns())
	s.EndColumn = clamp(s.EndColumn, g.Columns())
	g.Selection = s
}

// MoveSelection applies a bounded keyboard movement; Tab/Enter wrap at table edges.
func (g *Grid) MoveSelection(key string, extend bool) {
	r, c := g.Selection.EndRow, g.Selection.EndColumn
	switch key {
	case "left":
		c--
	case "right", "tab":
		c++
	case "up":
		r--
	case "down", "enter":
		r++
	}
	if key == "tab" && c >= g.Columns() {
		c = 0
		r++
	}
	if key == "tab" && c < 0 {
		c = g.Columns() - 1
		r--
	}
	if key == "enter" && r >= g.Rows() {
		r = 0
		c++
	}
	if key == "enter" && r < 0 {
		r = g.Rows() - 1
		c--
	}
	next := Selection{r, c, r, c}
	if extend {
		next.StartRow = g.Selection.StartRow
		next.StartColumn = g.Selection.StartColumn
	}
	g.SetSelection(next)
}

// SelectedCells returns the inclusive rectangular selection in row-major order.
func (g *Grid) SelectedCells() []Cell {
	s := g.Selection
	min := func(a, b int) int {
		if a < b {
			return a
		}
		return b
	}
	max := func(a, b int) int {
		if a > b {
			return a
		}
		return b
	}
	out := make([]Cell, 0)
	for r := min(s.StartRow, s.EndRow); r <= max(s.StartRow, s.EndRow); r++ {
		for c := min(s.StartColumn, s.EndColumn); c <= max(s.StartColumn, s.EndColumn); c++ {
			if x, ok := g.CellAt(r, c); ok {
				out = append(out, x)
			}
		}
	}
	return out
}

// PasteTSV parses clipboard data and returns a preview; it never mutates cells.
func (g *Grid) PasteTSV(text string) (*PastePreview, error) {
	text = strings.TrimSuffix(text, "\n")
	rd := csv.NewReader(strings.NewReader(text))
	rd.Comma = '\t'
	rd.FieldsPerRecord = -1
	rows := [][]string{}
	for {
		row, err := rd.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		rows = append(rows, row)
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("clipboard is empty")
	}
	p := &PastePreview{Rows: len(rows)}
	for _, r := range rows {
		if len(r) > p.Columns {
			p.Columns = len(r)
		}
	}
	for ri, row := range rows {
		for ci, raw := range row {
			rr, cc := g.Selection.StartRow+ri, g.Selection.StartColumn+ci
			if !g.validCoord(rr, cc) {
				return nil, fmt.Errorf("paste exceeds grid at row %d column %d", ri+1, ci+1)
			}
			cell, _ := g.CellAt(rr, cc)
			if raw == "" {
				cell.Value = nil
				cell.Status = "missing"
			} else {
				v, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
				if err != nil {
					return nil, fmt.Errorf("invalid number at row %d column %d: %q", ri+1, ci+1, raw)
				}
				cell.Value = &v
				cell.Status = "raw"
			}
			if current, _ := g.CellAt(rr, cc); current.Value != nil {
				p.Overwrites++
			}
			p.Cells = append(p.Cells, cell)
		}
	}
	g.Preview = p
	g.ConfirmOverwrite = p.Overwrites > 0
	return p, nil
}
func (g *Grid) CommitPaste() error {
	if g.Preview == nil {
		return fmt.Errorf("no paste preview")
	}
	if g.ConfirmOverwrite {
		return fmt.Errorf("overwrite confirmation required")
	}
	p := g.Preview
	for _, cell := range p.Cells {
		if err := g.SetCell(cell); err != nil {
			return err
		}
	}
	g.Preview = nil
	return nil
}
func (g *Grid) ConfirmPasteOverwrite() error {
	if g.Preview == nil {
		return fmt.Errorf("no paste preview")
	}
	g.ConfirmOverwrite = false
	p := g.Preview
	for _, cell := range p.Cells {
		if err := g.SetCell(cell); err != nil {
			return err
		}
	}
	g.Preview = nil
	return nil
}
