package resultgrid

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"asterlab-mygo-prototype/internal/theme"
	plateui "asterlab-mygo-prototype/internal/uikit/plate"

	"github.com/egoist/mygo/ui"
)

// PlateView renders a grid as a well plate instead of a spreadsheet. The grid
// stays the only data source; selection and the pending edit live in Grid.
//
// The layout follows the plate-map pattern used by Benchling and Labii: row and
// column headers around a square well matrix, a single selected well with a
// numeric editor, a status legend, and recorded/missing counts in the header.
func PlateView(c *ui.Context, g *Grid) {
	if g == nil || g.Rows() == 0 || g.Columns() == 0 {
		return
	}
	t := c.Theme()
	tokens := theme.GetTokens(t)
	rows, cols := g.Rows(), g.Columns()

	field := Field{Key: "value", Label: "读数"}
	if len(g.Schema.Fields) > 0 {
		field = g.Schema.Fields[0]
	}
	recorded, missing, excluded, invalid := 0, 0, 0, 0
	sum := 0.0
	for _, cell := range g.Cells {
		switch {
		case cell.Status == "excluded":
			excluded++
		case cell.Value == nil:
			missing++
		default:
			if cell.Status == "invalid" {
				invalid++
			}
			recorded++
			sum += *cell.Value
		}
	}
	mean := 0.0
	if recorded > 0 {
		mean = sum / float64(recorded)
	}

	ui.Column(c).FillWidth().Gap(t.Space(2)).Children(func() {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(t.Space(2)).Children(func() {
			ui.Column(c).Gap(1).Children(func() {
				ui.Text(c, plateLabel(rows, cols)).FontSize(t.Rem(.95)).Bold().TextColor(t.Text)
				ui.Text(c, fmt.Sprintf("%s · %s (%s)", g.Schema.ID, field.Label, field.Unit)).
					FontSize(t.Rem(.72)).TextColor(t.TextMuted)
			})
			ui.Spacer(c)
			countChip(c, fmt.Sprintf("已记录 %d", recorded), tokens.SuccessBg, tokens.SuccessText, t)
			countChip(c, fmt.Sprintf("缺失 %d", missing), tokens.NeutralBg, tokens.NeutralText, t)
			if excluded > 0 {
				countChip(c, fmt.Sprintf("排除 %d", excluded), tokens.WarningBg, tokens.WarningText, t)
			}
			if invalid > 0 {
				countChip(c, fmt.Sprintf("无效 %d", invalid), tokens.DangerBg, tokens.DangerText, t)
			}
			if recorded > 0 {
				ui.Text(c, fmt.Sprintf("均值 %.3g", mean)).FontSize(t.Rem(.72)).
					TextColor(t.TextMuted).FontFeatures("tnum")
			}
			if ui.Button(c, "粘贴").Clicked() {
				pasteClipboard(c, g)
			}
		})

		if g.Preview != nil {
			ui.Row(c).FillWidth().Padding(6, 10).Radius(6).Background(tokens.WarningBg).
				AlignItems(ui.Center).Gap(t.Space(1.5)).Children(func() {
				ui.Text(c, fmt.Sprintf("粘贴预览：%d × %d，覆盖 %d 个孔位",
					g.Preview.Rows, g.Preview.Columns, g.Preview.Overwrites)).
					FontSize(t.Rem(.74)).TextColor(tokens.WarningText)
				ui.Spacer(c)
				label := "应用粘贴"
				if g.ConfirmOverwrite {
					label = "确认覆盖"
				}
				if ui.Button(c, label).Clicked() {
					var err error
					if g.ConfirmOverwrite {
						err = g.ConfirmPasteOverwrite()
					} else {
						err = g.CommitPaste()
					}
					if err != nil {
						g.Error = err.Error()
					}
				}
				if ui.Button(c, "取消").Clicked() {
					g.Preview, g.ConfirmOverwrite = nil, false
				}
			})
		}

		legendRow(c, g, t, tokens)

		drawPlate(c, g, t, tokens)

		selectedEditor(c, g, t, tokens, field)
	})
}

func plateLabel(rows, cols int) string {
	if rows == 8 && cols == 12 {
		return "96 孔板"
	}
	return fmt.Sprintf("%d × %d 孔板", rows, cols)
}

func countChip(c *ui.Context, label string, bg, fg ui.Color, t *ui.Theme) {
	ui.Row(c).Padding(2, 7).Radius(5).Background(bg).Children(func() {
		ui.Text(c, label).FontSize(t.Rem(.7)).Bold().TextColor(fg)
	})
}

func legendRow(c *ui.Context, g *Grid, t *ui.Theme, tokens theme.Tokens) {
	groups := map[string]int{}
	for _, cell := range g.Cells {
		if cell.Group != "" {
			groups[cell.Group]++
		}
	}
	names := make([]string, 0, len(groups))
	for name := range groups {
		names = append(names, name)
	}
	sort.Strings(names)

	ui.Row(c).FillWidth().Wrap().AlignItems(ui.Center).Gap(t.Space(2)).Children(func() {
		legendItem(c, tokens.Accent, "有读数", t)
		legendItem(c, tokens.Canvas, "缺失", t)
		legendItem(c, tokens.Warning, "排除", t)
		if len(names) > 0 {
			ui.Box(c).Width(1).Height(12).Background(tokens.Border)
		}
		for i, name := range names {
			if i >= 5 {
				ui.Text(c, fmt.Sprintf("+%d 组", len(names)-i)).FontSize(t.Rem(.7)).
					TextColor(t.TextMuted)
				break
			}
			legendItem(c, plateui.GroupColor(name, t.Dark), fmt.Sprintf("%s %d", name, groups[name]), t)
		}
	})
}

func legendItem(c *ui.Context, color ui.Color, label string, t *ui.Theme) {
	ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
		ui.Box(c).Size(8, 8).Radius(4).Background(color).Border(1, t.Border)
		ui.Text(c, label).FontSize(t.Rem(.7)).TextColor(t.TextMuted)
	})
}

func drawPlate(c *ui.Context, g *Grid, t *ui.Theme, tokens theme.Tokens) {
	rows, cols := g.Rows(), g.Columns()
	const (
		maxCell = float32(30)
		minCell = float32(18)
		gap     = float32(4)
		labelW  = float32(22)
		labelH  = float32(18)
		bottom  = float32(6)
	)
	windowW, _ := c.Size()
	rail := float32(272)
	if windowW > 0 && windowW < 1200 {
		rail = 240
	}
	avail := windowW - 232 - rail - 74
	if windowW <= 0 || avail < 300 {
		avail = 470
	}
	cell := (avail - labelW - 8 - float32(cols-1)*gap) / float32(cols)
	if cell > maxCell {
		cell = maxCell
	}
	if cell < minCell {
		cell = minCell
	}
	plateW := labelW + float32(cols)*cell + float32(cols-1)*gap + 8
	height := labelH + float32(rows)*cell + float32(rows-1)*gap + bottom

	var hoverRow, hoverCol = -1, -1
	hitZone, hitRow, hitCol := "", 0, 0

	plate := ui.Box(c).Size(plateW, height).Draw(func(p *ui.Painter, r ui.Rect) {
		x0 := r.X + labelW
		y0 := r.Y + labelH

		sel := g.Selection
		minRow, maxRow := minInt(sel.StartRow, sel.EndRow), maxInt(sel.StartRow, sel.EndRow)
		minCol, maxCol := minInt(sel.StartColumn, sel.EndColumn), maxInt(sel.StartColumn, sel.EndColumn)
		anchorRow, anchorCol := sel.StartRow, sel.StartColumn

		labelSize := t.Rem(.7)
		for col := 0; col < cols; col++ {
			x := x0 + float32(col)*(cell+gap)
			label := g.Schema.ColumnLabels[col]
			full := minRow == 0 && maxRow == rows-1 && col >= minCol && col <= maxCol
			color := t.TextMuted
			if full {
				color = t.Accent
				p.Fill(ui.Rect{X: x + 3, Y: r.Y + labelH - 4, W: cell - 6, H: 2}, t.Accent, 1)
			}
			p.Text(x+cell/2-textWidth(label, labelSize)/2, r.Y+labelH-7, label, labelSize, color)
		}
		for row := 0; row < rows; row++ {
			y := y0 + float32(row)*(cell+gap)
			label := g.Schema.RowLabels[row]
			full := minCol == 0 && maxCol == cols-1 && row >= minRow && row <= maxRow
			color := t.TextMuted
			if full {
				color = t.Accent
				p.Fill(ui.Rect{X: r.X + labelW - 5, Y: y + 3, W: 2, H: cell - 6}, t.Accent, 1)
			}
			p.Text(r.X+labelW-12, y+cell/2+labelSize*.36, label, labelSize, color)
		}

		for row := 0; row < rows; row++ {
			for col := 0; col < cols; col++ {
				cellData, _ := g.CellAt(row, col)
				x := x0 + float32(col)*(cell+gap)
				y := y0 + float32(row)*(cell+gap)
				wr := ui.Rect{X: x, Y: y, W: cell, H: cell}
				radius := cell / 2

				fill := tokens.Canvas
				stroke := t.Border
				switch {
				case cellData.Status == "excluded":
					fill = tokens.WarningBg
				case cellData.Status == "invalid":
					fill = tokens.DangerBg
				case cellData.Value != nil:
					fill = tokens.Accent.Alpha(.10)
				}
				inSelection := row >= minRow && row <= maxRow && col >= minCol && col <= maxCol
				if inSelection {
					fill = fill.Mix(tokens.Accent, .10)
				}
				if row == hoverRow && col == hoverCol {
					stroke = tokens.BorderStrong
				}
				p.Fill(wr, fill, radius)
				p.Stroke(wr, stroke, radius, 1)

				if cellData.Group != "" {
					p.Fill(ui.Rect{X: x + 5, Y: y + 5, W: 4, H: 4},
						plateui.GroupColor(cellData.Group, t.Dark), 2)
				}
				if cellData.Value != nil {
					value := formatWellValue(*cellData.Value)
					size := cell * .28
					if size < 8 {
						size = 8
					}
					if size > 12 {
						size = 12
					}
					p.Text(x+cell/2-textWidth(value, size)/2, y+cell/2+size*.36,
						value, size, t.Text)
				} else if cellData.Status == "excluded" {
					p.Line(x+cell*.32, y+cell*.68, x+cell*.68, y+cell*.32, 1.5, tokens.WarningText)
				} else {
					p.Fill(ui.Rect{X: x + cell/2 - 1.5, Y: y + cell/2 - 1.5, W: 3, H: 3},
						t.Border, 1.5)
				}
				if row == anchorRow && col == anchorCol {
					p.Stroke(wr, tokens.Accent, radius, 2.5)
					outer := ui.Rect{X: x - 2, Y: y - 2, W: cell + 4, H: cell + 4}
					p.Stroke(outer, tokens.Accent.Alpha(.28), radius+2, 1)
				}
			}
		}
	})
	plate.HitTest(func(x, y float32, r ui.Rect) bool {
		hoverRow, hoverCol = -1, -1
		hitZone, hitRow, hitCol = "", 0, 0
		gx := x - (r.X + labelW)
		gy := y - (r.Y + labelH)
		if y >= r.Y && y <= r.Y+labelH && gx >= 0 {
			col := int(gx / (cell + gap))
			if col >= 0 && col < cols && gx-float32(col)*(cell+gap) <= cell {
				hitZone, hitCol = "col", col
				return true
			}
		}
		if x >= r.X && x <= r.X+labelW && gy >= 0 {
			row := int(gy / (cell + gap))
			if row >= 0 && row < rows && gy-float32(row)*(cell+gap) <= cell {
				hitZone, hitRow = "row", row
				return true
			}
		}
		if gx < 0 || gy < 0 {
			return false
		}
		col := int(gx / (cell + gap))
		row := int(gy / (cell + gap))
		if col < 0 || col >= cols || row < 0 || row >= rows {
			return false
		}
		cx := gx - float32(col)*(cell+gap)
		cy := gy - float32(row)*(cell+gap)
		if cx > cell || cy > cell {
			return false
		}
		hitZone, hitRow, hitCol = "well", row, col
		hoverRow, hoverCol = row, col
		return true
	})
	plate.OnClick(func() {
		if err := g.CommitPendingEdit(); err != nil {
			g.Error = err.Error()
			return
		}
		switch hitZone {
		case "col":
			g.SetSelection(Selection{StartRow: 0, StartColumn: hitCol, EndRow: rows - 1, EndColumn: hitCol})
			g.BeginEdit(0, hitCol)
			g.Error = ""
		case "row":
			g.SetSelection(Selection{StartRow: hitRow, StartColumn: 0, EndRow: hitRow, EndColumn: cols - 1})
			g.BeginEdit(hitRow, 0)
			g.Error = ""
		case "well":
			if plate.ClickModifiers()&ui.Shift != 0 {
				g.SetSelection(Selection{
					StartRow: g.Selection.StartRow, StartColumn: g.Selection.StartColumn,
					EndRow: hitRow, EndColumn: hitCol,
				})
				g.BeginEdit(g.Selection.StartRow, g.Selection.StartColumn)
				g.Error = ""
				return
			}
			if err := g.SelectCell(hitRow, hitCol); err != nil {
				g.Error = err.Error()
			}
		}
	})
	plate.Focusable()

	move := func(key string, extend bool) {
		if err := g.CommitPendingEdit(); err != nil {
			g.Error = err.Error()
			return
		}
		g.MoveSelection(key, extend)
		g.BeginEdit(g.Selection.StartRow, g.Selection.StartColumn)
		g.Error = ""
	}
	c.OnShortcut(0, ui.KeyLeft, func() { move("left", false) })
	c.OnShortcut(0, ui.KeyRight, func() { move("right", false) })
	c.OnShortcut(0, ui.KeyUp, func() { move("up", false) })
	c.OnShortcut(0, ui.KeyDown, func() { move("down", false) })
	c.OnShortcut(ui.Shift, ui.KeyLeft, func() { move("left", true) })
	c.OnShortcut(ui.Shift, ui.KeyRight, func() { move("right", true) })
	c.OnShortcut(ui.Shift, ui.KeyUp, func() { move("up", true) })
	c.OnShortcut(ui.Shift, ui.KeyDown, func() { move("down", true) })
	c.OnShortcut(0, ui.KeyTab, func() { move("tab", false) })
	c.OnShortcut(0, ui.KeyEnter, func() { move("enter", false) })
	c.OnShortcut(ui.Cmd, ui.KeyC, func() { copySelection(c, g) })
	c.OnShortcut(ui.Cmd, ui.KeyV, func() { pasteClipboard(c, g) })
	c.OnShortcut(0, ui.KeyDelete, func() { clearSelection(g) })
	c.OnShortcut(0, ui.KeyBackspace, func() { clearSelection(g) })
	c.OnShortcut(ui.Cmd, ui.KeyZ, func() {
		if g.Undo() {
			g.BeginEdit(g.Selection.StartRow, g.Selection.StartColumn)
		}
	})
	c.OnShortcut(ui.Cmd, ui.KeyY, func() {
		if g.Redo() {
			g.BeginEdit(g.Selection.StartRow, g.Selection.StartColumn)
		}
	})
}

func selectedEditor(c *ui.Context, g *Grid, t *ui.Theme, tokens theme.Tokens, field Field) {
	row, col := g.Selection.StartRow, g.Selection.StartColumn
	if !g.validCoord(row, col) {
		row, col = 0, 0
	}
	cell, _ := g.CellAt(row, col)
	draft, ok := g.BeginEdit(row, col)
	if !ok {
		return
	}
	well := ""
	if row < len(g.Schema.RowLabels) && col < len(g.Schema.ColumnLabels) {
		well = g.Schema.RowLabels[row] + g.Schema.ColumnLabels[col]
	}
	sample := cell.SampleID
	if sample == "" {
		sample = "未分配样本"
	}
	meta := cell.Group
	if meta == "" {
		meta = "未分组"
	}
	if cell.Replicate > 0 {
		meta += fmt.Sprintf(" · 重复 %d", cell.Replicate)
	}

	ui.Row(c).FillWidth().Padding(t.Space(2)).Radius(8).
		Background(tokens.Elevated).Border(1, tokens.Border).
		AlignItems(ui.Center).Gap(t.Space(2)).Children(func() {
		ui.Text(c, well).FontSize(t.Rem(.92)).Bold().TextColor(t.Text).Width(34)
		ui.Column(c).Gap(1).Children(func() {
			ui.Text(c, sample).FontSize(t.Rem(.8)).TextColor(t.Text).SingleLine().Ellipsis("…")
			ui.Text(c, meta).FontSize(t.Rem(.7)).TextColor(t.TextMuted).SingleLine().Ellipsis("…")
		})
		ui.Spacer(c)
		ui.Text(c, field.Label).FontSize(t.Rem(.72)).TextColor(t.TextMuted)
		ui.TextInput(c, draft).Width(104).Placeholder("—").
			OnChange(func() { g.MarkEditDirty() }).
			OnSubmit(func() {
				if err := g.CommitEdit(); err != nil {
					g.Error = err.Error()
					return
				}
				g.MoveSelection("enter", false)
				g.BeginEdit(g.Selection.StartRow, g.Selection.StartColumn)
				g.Error = ""
			})
		if field.Unit != "" {
			ui.Text(c, field.Unit).FontSize(t.Rem(.72)).TextColor(t.TextMuted)
		}
		statusPill(c, cell.Status, t, tokens)
	})

	msg := "点行/列标题选中整行整列 · Shift+单击扩展选区 · Ctrl+C 复制 · Ctrl+V 粘贴"
	if g.Error != "" {
		ui.Text(c, g.Error).FontSize(t.Rem(.72)).TextColor(t.Danger)
	} else if g.EditDirty() {
		ui.Text(c, "未提交：按 Enter 或点击其他孔位写入").FontSize(t.Rem(.7)).
			TextColor(t.Warning)
	} else {
		ui.Text(c, msg).FontSize(t.Rem(.7)).TextColor(t.TextMuted)
	}
}

func statusPill(c *ui.Context, status string, t *ui.Theme, tokens theme.Tokens) {
	label, bg, fg := "缺失", tokens.NeutralBg, tokens.NeutralText
	switch status {
	case "raw":
		label, bg, fg = "原始", tokens.SuccessBg, tokens.SuccessText
	case "derived":
		label, bg, fg = "派生", tokens.Accent.Alpha(.12), t.Accent
	case "invalid":
		label, bg, fg = "无效", tokens.DangerBg, tokens.DangerText
	case "undetected":
		label, bg, fg = "未检出", tokens.WarningBg, tokens.WarningText
	case "excluded":
		label, bg, fg = "排除", tokens.WarningBg, tokens.WarningText
	}
	ui.Row(c).Padding(2, 7).Radius(5).Background(bg).Children(func() {
		ui.Text(c, label).FontSize(t.Rem(.68)).Bold().TextColor(fg)
	})
}

func pasteClipboard(c *ui.Context, g *Grid) {
	if err := g.CommitPendingEdit(); err != nil {
		g.Error = err.Error()
		return
	}
	if _, err := g.PasteTSV(c.ReadClipboard()); err != nil {
		g.Error = err.Error()
		g.State = StateError
	}
}

func copySelection(c *ui.Context, g *Grid) {
	sel := g.Selection
	minRow, maxRow := minInt(sel.StartRow, sel.EndRow), maxInt(sel.StartRow, sel.EndRow)
	minCol, maxCol := minInt(sel.StartColumn, sel.EndColumn), maxInt(sel.StartColumn, sel.EndColumn)
	lines := make([]string, 0, maxRow-minRow+1)
	for row := minRow; row <= maxRow; row++ {
		values := make([]string, 0, maxCol-minCol+1)
		for col := minCol; col <= maxCol; col++ {
			cell, _ := g.CellAt(row, col)
			if cell.Value == nil {
				values = append(values, "")
			} else {
				values = append(values, strconv.FormatFloat(*cell.Value, 'g', -1, 64))
			}
		}
		lines = append(lines, strings.Join(values, "\t"))
	}
	c.WriteClipboard(strings.Join(lines, "\n"))
}

func clearSelection(g *Grid) {
	for _, cell := range g.SelectedCells() {
		cell.Value, cell.Status = nil, "missing"
		_ = g.SetCell(cell)
	}
	g.BeginEdit(g.Selection.StartRow, g.Selection.StartColumn)
}

func formatWellValue(v float64) string {
	av := math.Abs(v)
	switch {
	case av >= 100:
		return fmt.Sprintf("%.0f", v)
	case av >= 10:
		return fmt.Sprintf("%.1f", v)
	case av == 0:
		return "0"
	default:
		return fmt.Sprintf("%.2f", v)
	}
}

func textWidth(s string, size float32) float32 {
	return float32(len([]rune(s))) * size * .58
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
