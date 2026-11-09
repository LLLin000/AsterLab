package blocks

import (
	"fmt"

	"github.com/egoist/mygo/ui"
)

// View renders a read-only preview of an instance into the given context.
func View(c *ui.Context, inst Instance) {
	t := c.Theme()
	switch inst.Type {
	case TypeNote:
		var p NoteProps
		_ = inst.DecodeProps(&p)
		ui.Column(c).Gap(4).Children(func() {
			if p.Title != "" {
				ui.Text(c, p.Title).Bold()
			}
			ui.Text(c, p.Text)
		})
	case TypeParameterTable:
		var p ParameterTableProps
		_ = inst.DecodeProps(&p)
		ui.Column(c).Gap(2).Children(func() {
			for _, r := range p.Rows {
				ui.Text(c, fmt.Sprintf("%s = %s %s", r.Key, r.Value, r.Unit))
			}
		})
	case TypeMeasurementTable:
		var p MeasurementTableProps
		_ = inst.DecodeProps(&p)
		ui.Column(c).Gap(2).Children(func() {
			for _, r := range p.Rows {
				ui.Text(c, fmt.Sprintf("%s: %g %s", r.SampleID, r.Value, r.Unit))
			}
		})
	case TypePlate96:
		var p Plate96Props
		_ = inst.DecodeProps(&p)
		if p.Rows == 0 {
			p.Rows = 8
		}
		if p.Cols == 0 {
			p.Cols = 12
		}
		ui.Column(c).Gap(2).Children(func() {
			ui.Text(c, fmt.Sprintf("Plate %dx%d selected=%d", p.Rows, p.Cols, len(p.Selected)))
			for i := range p.Rows {
				ui.Row(c).Gap(2).Children(func() {
					for j := range p.Cols {
						idx := i*p.Cols + j
						mark := "."
						if containsInt(p.Selected, idx) {
							mark = "X"
						}
						if lbl, ok := p.Labels[idx]; ok && lbl != "" {
							mark = lbl
						}
						ui.Text(c, mark)
					}
				})
			}
		})
	case TypeDilutionCalculator:
		var p DilutionProps
		_ = inst.DecodeProps(&p)
		ui.Column(c).Gap(2).Children(func() {
			ui.Text(c, fmt.Sprintf("stock=%g final=%g vol=%g", p.Input.StockConc, p.Input.FinalConc, p.Input.FinalVol))
			if p.Valid {
				ui.Text(c, fmt.Sprintf("stockVol=%g diluent=%g", p.Result.StockVol, p.Result.DiluentVol))
			} else {
				ui.Text(c, "invalid: "+p.Error)
			}
		})
	case TypeChecklist:
		var p ChecklistProps
		_ = inst.DecodeProps(&p)
		ui.Column(c).Gap(2).Children(func() {
			for _, it := range p.Items {
				mark := "[ ]"
				if it.Checked {
					mark = "[x]"
				}
				ui.Text(c, mark+" "+it.Text)
			}
		})
	case TypeNestedWorkflow:
		var p NestedWorkflowProps
		_ = inst.DecodeProps(&p)
		ui.Text(c, fmt.Sprintf("workflow=%s step=%s", p.WorkflowID, p.StepRef))
	}
	_ = t
}

// Gallery renders all block types for a PNG snapshot.
func Gallery(c *ui.Context, insts []Instance) {
	ui.Column(c).Gap(8).Padding(12).Children(func() {
		ui.Text(c, "Blocks Gallery").Bold()
		for _, inst := range insts {
			ui.Row(c).Gap(8).Children(func() {
				ui.Text(c, string(inst.Type)).TextColor(c.Theme().TextMuted)
				View(c, inst)
			})
		}
	})
}

func containsInt(xs []int, v int) bool {
	for _, x := range xs {
		if x == v {
			return true
		}
	}
	return false
}
