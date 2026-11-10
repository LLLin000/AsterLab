package asterdoc

import "github.com/egoist/mygo/ui"

// WellRect is the hit box of one well in the enlarged plate, relative to the
// well grid's top-left corner.
func WellRect(geometry Geometry, cell, gap float32, row, col int) ui.Rect {
	return ui.Rect{
		X: float32(col) * (cell + gap),
		Y: float32(row) * (cell + gap),
		W: cell,
		H: cell,
	}
}

// WellsInRect returns the wells whose centers fall inside rect, using only the
// geometry and the current transform. It is pure so marquee behaviour can be
// tested without pointer input.
func WellsInRect(geometry Geometry, cell, gap float32, rect ui.Rect) []string {
	if rect.W < 0 {
		rect.X += rect.W
		rect.W = -rect.W
	}
	if rect.H < 0 {
		rect.Y += rect.H
		rect.H = -rect.H
	}
	out := []string{}
	for row := 0; row < geometry.Rows; row++ {
		for col := 0; col < geometry.Columns; col++ {
			well := WellRect(geometry, cell, gap, row, col)
			cx := well.X + well.W/2
			cy := well.Y + well.H/2
			if cx >= rect.X && cx <= rect.X+rect.W && cy >= rect.Y && cy <= rect.Y+rect.H {
				out = append(out, rowLabel(row)+itoa(col+1))
			}
		}
	}
	return out
}

func itoa(value int) string {
	if value == 0 {
		return "0"
	}
	digits := []byte{}
	for value > 0 {
		digits = append([]byte{byte('0' + value%10)}, digits...)
		value /= 10
	}
	return string(digits)
}
