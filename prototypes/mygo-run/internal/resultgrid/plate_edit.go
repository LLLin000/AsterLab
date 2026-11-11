package resultgrid

import (
	"fmt"
	"strconv"
	"strings"
)

// BeginEdit prepares an editable draft for one cell. The cell itself is not
// mutated until CommitEdit, so typing "0." does not get reformatted mid-entry.
func (g *Grid) BeginEdit(r, c int) (*string, bool) {
	if !g.validCoord(r, c) {
		return nil, false
	}
	if g.editRow == r && g.editCol == c {
		return &g.editDraft, true
	}
	g.editRow, g.editCol = r, c
	g.editDraft = g.inputs[g.index(r, c)]
	g.editDirty = false
	return &g.editDraft, true
}

// EditDirty reports whether the current draft differs from the stored cell.
func (g *Grid) EditDirty() bool { return g.editDirty }

// MarkEditDirty records that the bound input changed. It never parses.
func (g *Grid) MarkEditDirty() { g.editDirty = true }

// CommitEdit parses the draft and writes it through SetCell. An invalid number
// keeps the draft and returns an error so the caller can surface it.
func (g *Grid) CommitEdit() error {
	if !g.validCoord(g.editRow, g.editCol) {
		return nil
	}
	cell, _ := g.CellAt(g.editRow, g.editCol)
	raw := strings.TrimSpace(g.editDraft)
	if raw == "" {
		cell.Value, cell.Status = nil, "missing"
		if err := g.SetCell(cell); err != nil {
			return err
		}
		g.editDirty = false
		return nil
	}
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return fmt.Errorf("请输入数字")
	}
	cell.Value, cell.Status = &value, "raw"
	if err := g.SetCell(cell); err != nil {
		return err
	}
	g.editDirty = false
	return nil
}

// SelectCell commits any pending edit and moves the single-cell selection.
func (g *Grid) SelectCell(r, c int) error {
	if g.editDirty {
		if err := g.CommitEdit(); err != nil {
			return err
		}
	}
	g.SetSelection(Selection{StartRow: r, StartColumn: c, EndRow: r, EndColumn: c})
	g.BeginEdit(r, c)
	return nil
}

// CommitPendingEdit is the save-path guard: callers must commit before they
// hand cells to Core.
func (g *Grid) CommitPendingEdit() error {
	if !g.editDirty {
		return nil
	}
	return g.CommitEdit()
}
