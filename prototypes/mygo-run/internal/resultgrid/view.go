package resultgrid

import (
	"fmt"
	"strconv"

	"github.com/egoist/mygo/ui"
)

// View renders the grid, keeping all draft and selection state in Grid.
func View(c *ui.Context, g *Grid, opts Options) {
	if g == nil {
		return
	}
	width := float32(96)
	if opts.FullWidth {
		width = 0
	}
	ui.Column(c).Fill().Padding(8).Gap(4).Children(func() {
		ui.Row(c).Gap(8).Children(func() {
			ui.Text(c, fmt.Sprintf("%s · %s", g.Schema.ID, g.State)).Bold()
			if g.Error != "" {
				ui.Text(c, g.Error).TextColor(c.Theme().Danger)
			}
			ui.Spacer(c)
			if ui.Button(c, "粘贴").Clicked() {
				if _, err := g.PasteTSV(c.ReadClipboard()); err != nil {
					g.Error = err.Error()
					g.State = StateError
				}
			}
		})
		if g.Preview != nil {
			ui.Row(c).Gap(8).Children(func() {
				message := fmt.Sprintf("Paste preview: %d × %d, %d overwrite(s)", g.Preview.Rows, g.Preview.Columns, g.Preview.Overwrites)
				ui.Text(c, message)
				if g.ConfirmOverwrite {
					if ui.Button(c, "Confirm overwrite").Clicked() {
						_ = g.ConfirmPasteOverwrite()
					}
				} else if ui.Button(c, "Apply paste").Clicked() {
					_ = g.CommitPaste()
				}
			})
		}
		ui.ScrollBoth(c).Fill().Children(func() {
			ui.Column(c).Gap(2).Children(func() {
				ui.Row(c).Gap(2).Children(func() {
					ui.Text(c, "").Width(38)
					for _, label := range g.Schema.ColumnLabels {
						ui.Text(c, label).Width(width).Padding(4).Background(c.Theme().Surface).Bold()
					}
				})
				for r, label := range g.Schema.RowLabels {
					rr := r
					ui.Row(c).Gap(2).Children(func() {
						ui.Text(c, label).Width(38).Padding(4).Bold()
						for col := range g.Schema.ColumnLabels {
							cc := col
							cell, _ := g.CellAt(rr, cc)
							commit := func() {
								raw := *g.input(rr, cc)
								v, err := strconv.ParseFloat(raw, 64)
								if raw == "" {
									cell.Value = nil
									cell.Status = "missing"
									_ = g.SetCell(cell)
								} else if err != nil {
									g.Error = "Invalid numeric value"
									g.State = StateError
								} else {
									cell.Value = &v
									cell.Status = "raw"
									g.Error = ""
									_ = g.SetCell(cell)
								}
							}
							e := ui.TextInput(c, g.input(rr, cc)).Width(width).Padding(4).Placeholder("—").OnSubmit(commit).OnPaste(func(text string) bool {
								if _, err := g.PasteTSV(text); err != nil {
									g.Error = err.Error()
									g.State = StateError
									return false
								}
								return true
							})
							// A click on Save must persist the text currently in
							// the input, not only values confirmed with Enter.
							if e.Changed() {
								commit()
							}
							e.OnClick(func() { g.SetSelection(Selection{rr, cc, rr, cc}) })
							if rr >= min(g.Selection.StartRow, g.Selection.EndRow) && rr <= max(g.Selection.StartRow, g.Selection.EndRow) && cc >= min(g.Selection.StartColumn, g.Selection.EndColumn) && cc <= max(g.Selection.StartColumn, g.Selection.EndColumn) {
								e.Background(c.Theme().Accent)
							}
						}
					})
				}
			})
		})
	})
	c.OnShortcut(ui.Cmd, ui.KeyC, func() {
		var lines []string
		for r := min(g.Selection.StartRow, g.Selection.EndRow); r <= max(g.Selection.StartRow, g.Selection.EndRow); r++ {
			line := ""
			for col := min(g.Selection.StartColumn, g.Selection.EndColumn); col <= max(g.Selection.StartColumn, g.Selection.EndColumn); col++ {
				if col > min(g.Selection.StartColumn, g.Selection.EndColumn) {
					line += "\t"
				}
				cell, _ := g.CellAt(r, col)
				if cell.Value != nil {
					line += strconv.FormatFloat(*cell.Value, 'g', -1, 64)
				}
			}
			lines = append(lines, line)
		}
		c.WriteClipboard(stringsJoin(lines, "\n"))
	})
	c.OnShortcut(ui.Cmd, ui.KeyV, func() {
		_, err := g.PasteTSV(c.ReadClipboard())
		if err != nil {
			g.Error = err.Error()
			g.State = StateError
		}
	})
	c.OnShortcut(ui.Cmd, ui.KeyZ, func() { g.Undo() })
	c.OnShortcut(ui.Cmd, ui.KeyY, func() { g.Redo() })
	c.OnShortcut(0, ui.KeyTab, func() { g.MoveSelection("tab", false) })
	c.OnShortcut(0, ui.KeyEnter, func() { g.MoveSelection("enter", false) })
	c.OnShortcut(0, ui.KeyLeft, func() { g.MoveSelection("left", false) })
	c.OnShortcut(0, ui.KeyRight, func() { g.MoveSelection("right", false) })
	c.OnShortcut(0, ui.KeyUp, func() { g.MoveSelection("up", false) })
	c.OnShortcut(0, ui.KeyDown, func() { g.MoveSelection("down", false) })
	c.OnShortcut(ui.Shift, ui.KeyLeft, func() { g.MoveSelection("left", true) })
	c.OnShortcut(ui.Shift, ui.KeyRight, func() { g.MoveSelection("right", true) })
	c.OnShortcut(ui.Shift, ui.KeyUp, func() { g.MoveSelection("up", true) })
	c.OnShortcut(ui.Shift, ui.KeyDown, func() { g.MoveSelection("down", true) })
}
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
func stringsJoin(s []string, sep string) string {
	out := ""
	for i, v := range s {
		if i > 0 {
			out += sep
		}
		out += v
	}
	return out
}
