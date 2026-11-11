package plate

import (
	"fmt"
	"sort"
	"strings"

	"github.com/egoist/mygo/ui"

	"asterlab-mygo-prototype/internal/theme"
)

type WellState struct {
	Group     string
	SampleID  string
	Replicate int
	Timepoint string
	Ambiguous bool
}

type Props struct {
	Wells       map[string]WellState
	SelectedIDs []string
	OnSelect    func(wellIDs []string)
	Interactive bool
}

const (
	rows = 8
	cols = 12
)

var rowLabels = [rows]string{"A", "B", "C", "D", "E", "F", "G", "H"}

func wellID(row, col int) string {
	return fmt.Sprintf("%s%d", rowLabels[row], col+1)
}

func GroupColor(group string, dark bool) ui.Color {
	g := strings.ToLower(strings.TrimSpace(group))
	switch {
	case g == "":
		return ui.Color{}
	case strings.Contains(g, "nc"), strings.Contains(g, "对照"), strings.Contains(g, "control"):
		if dark {
			return ui.Hex("#4a463c")
		}
		return ui.Hex("#dcd6c8")
	case strings.Contains(g, "pc"), strings.Contains(g, "阳性"), strings.Contains(g, "positive"):
		if dark {
			return ui.Hex("#5d4a23")
		}
		return ui.Hex("#f2e2bf")
	case g == "c1":
		if dark {
			return ui.Hex("#433a63")
		}
		return ui.Hex("#cfc4f0")
	case g == "c2":
		if dark {
			return ui.Hex("#3a4460")
		}
		return ui.Hex("#c9d6f0")
	case g == "c3":
		if dark {
			return ui.Hex("#5a3a44")
		}
		return ui.Hex("#f0c8ce")
	case g == "c4":
		if dark {
			return ui.Hex("#3c5148")
		}
		return ui.Hex("#c9e3d2")
	case g == "c5":
		if dark {
			return ui.Hex("#5a4a2c")
		}
		return ui.Hex("#efdab4")
	default:
		palette := [...]string{"#d9d0f2", "#eddcc0", "#ccd9ee", "#eed3dc"}
		darkPalette := [...]string{"#3f3a55", "#4d4331", "#333f52", "#4a3742"}
		index := 0
		for _, r := range g {
			index = (index*31 + int(r)) % len(palette)
		}
		if dark {
			return ui.Hex(darkPalette[uint32(index)%uint32(len(darkPalette))])
		}
		return ui.Hex(palette[index%len(palette)])
	}
}

func selectedSet(props Props) map[string]bool {
	out := make(map[string]bool, len(props.SelectedIDs))
	for _, id := range props.SelectedIDs {
		out[id] = true
	}
	return out
}

// View renders the plate. Drag-rectangle selection is not implemented:
// MyGo exposes no pointer-coordinate API for a marquee without a separate model.
// Click toggles one well; Shift+click adds to the current selection.

func View(c *ui.Context, props Props) {
	th := c.Theme()
	t := theme.GetTokens(th)
	dark := th != nil && th.Dark
	var lastHitWell, lastHoverWell string

	wellSize := float32(22)
	gap := float32(5)
	labelSize := th.Rem(.78)
	leftLabel := float32(20)
	topLabel := float32(18)
	gridW := float32(cols)*wellSize + float32(cols-1)*gap
	gridH := float32(rows)*wellSize + float32(rows-1)*gap
	width := leftLabel + gridW + 4
	height := topLabel + gridH + 28
	selected := selectedSet(props)
	groupCounts := make(map[string]int)
	for _, well := range props.Wells {
		if !well.Ambiguous && well.Group != "" {
			groupCounts[well.Group]++
		}
	}

	ui.Column(c).Gap(6).AlignItems(ui.Center).Children(func() {
		var el ui.Element
		el = ui.Box(c).Size(width, height).Draw(func(p *ui.Painter, r ui.Rect) {
			for col := 0; col < cols; col++ {
				x := r.X + leftLabel + float32(col)*(wellSize+gap) + wellSize/2 - 4
				p.Text(x, r.Y+2, fmt.Sprintf("%d", col+1), labelSize, t.TextMuted)
			}
			for row := 0; row < rows; row++ {
				p.Text(r.X+2, r.Y+topLabel+float32(row)*(wellSize+gap)+wellSize/2+3, rowLabels[row], labelSize, t.TextMuted)
				for col := 0; col < cols; col++ {
					id := wellID(row, col)
					x := r.X + leftLabel + float32(col)*(wellSize+gap)
					y := r.Y + topLabel + float32(row)*(wellSize+gap)
					wr := ui.Rect{X: x, Y: y, W: wellSize, H: wellSize}
					well, assigned := props.Wells[id]
					fill := t.Canvas
					if assigned && well.Ambiguous {
						fill = t.Warning
					} else if assigned {
						fill = GroupColor(well.Group, dark)
					}
					p.Fill(wr, fill, wellSize/2)
					p.Stroke(wr, t.Border, wellSize/2, 1)
					if well.Ambiguous {
						p.Text(x+7, y+3, "!", th.Rem(.75), t.Text)
					}
					if selected[id] {
						p.Stroke(wr, t.Accent, wellSize/2, 2)
						p.Line(x+5, y+11, x+9, y+15.5, 2, t.Accent)
						p.Line(x+9, y+15.5, x+17, y+6.5, 2, t.Accent)
					}
				}
			}
			hudY := r.Y + topLabel + gridH + 6
			if lastHoverWell != "" {
				well, assigned := props.Wells[lastHoverWell]
				parts := []string{lastHoverWell}
				switch {
				case !assigned:
					parts = append(parts, "未分配")
				case well.Ambiguous:
					parts = append(parts, "分配冲突", well.Timepoint)
				default:
					parts = append(parts, well.SampleID, well.Group)
					if well.Replicate > 0 {
						parts = append(parts, fmt.Sprintf("重复 %d", well.Replicate))
					}
					if well.Timepoint != "" {
						parts = append(parts, well.Timepoint)
					}
				}
				p.Text(r.X+leftLabel, hudY+10, strings.Join(parts, " · "), th.Rem(.75), t.Text)
			} else if len(props.Wells) == 0 {
				p.Text(r.X+leftLabel, hudY+10, "尚无计划孔位分配", th.Rem(.75), t.TextMuted)
			}
		}).HitTest(func(x, y float32, r ui.Rect) bool {
			lastHitWell, lastHoverWell = "", ""
			gx := x - (r.X + leftLabel)
			gy := y - (r.Y + topLabel)
			if gx < 0 || gy < 0 {
				return false
			}
			col := int(gx / (wellSize + gap))
			row := int(gy / (wellSize + gap))
			if col < 0 || col >= cols || row < 0 || row >= rows {
				return false
			}
			cx := gx - float32(col)*(wellSize+gap)
			cy := gy - float32(row)*(wellSize+gap)
			if cx > wellSize || cy > wellSize {
				return false
			}
			lastHitWell = wellID(row, col)
			lastHoverWell = lastHitWell
			return true
		}).OnClick(func() {
			if !props.Interactive || lastHitWell == "" {
				return
			}
			next := make(map[string]bool, len(selected)+1)
			if el.ClickModifiers()&ui.Shift != 0 {
				for id := range selected {
					next[id] = true
				}
				next[lastHitWell] = true
			} else {
				next[lastHitWell] = true
			}
			ids := make([]string, 0, len(next))
			for id := range next {
				ids = append(ids, id)
			}
			sort.Strings(ids)
			if props.OnSelect != nil {
				props.OnSelect(ids)
			}
		}).Focusable()

		if len(groupCounts) > 0 {
			groups := make([]string, 0, len(groupCounts))
			for group := range groupCounts {
				groups = append(groups, group)
			}
			sort.Strings(groups)
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				for _, group := range groups {
					ui.Row(c).Gap(3).AlignItems(ui.Center).Children(func() {
						ui.Box(c).Size(7, 7).Radius(4).Background(GroupColor(group, dark))
						ui.Text(c, fmt.Sprintf("%s %d", group, groupCounts[group])).FontSize(th.Rem(.75)).TextColor(t.TextMuted)
					})
				}
			})
		}
		ui.Text(c, fmt.Sprintf("%d / 96 孔已分配 · 单击选择，Shift+单击多选", len(props.Wells))).FontSize(th.Rem(.75)).TextColor(t.TextMuted)
	})
}
