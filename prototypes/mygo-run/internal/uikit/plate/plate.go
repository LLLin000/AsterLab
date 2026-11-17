package plate

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"

	"asterlab-mygo-prototype/internal/theme"
)

type WellState struct {
	Group     string
	SampleID  string
	Replicate int
	Timepoint string
	Role      string
	Ambiguous bool
}

type Props struct {
	Wells       map[string]WellState
	SelectedIDs []string
	OnSelect    func(wellIDs []string)
	Interactive bool
	Rows        int
	Cols        int
	Topology    string
	Roles       map[string]string
}

const (
	defaultRows = 8
	defaultCols = 12
)

// supportedPlateSizes are the geometries AsterLab renders: 6, 12, 24, 48, 96
// and 384 wells as rows×cols. Unsupported dimensions fall back to 96 wells.
var supportedPlateSizes = [...]struct{ Wells, Rows, Cols int }{
	{6, 2, 3},
	{12, 3, 4},
	{24, 4, 6},
	{48, 6, 8},
	{96, 8, 12},
	{384, 16, 24},
}

func wellGrid(rows, cols int) bool {
	for _, size := range supportedPlateSizes {
		if size.Rows == rows && size.Cols == cols {
			return true
		}
	}
	return false
}

func rowLabel(row int) string {
	if row < 26 {
		return string(rune('A' + row))
	}
	return fmt.Sprintf("A%c", rune('A'+row-26))
}

func wellID(row, col int) string {
	return fmt.Sprintf("%s%d", rowLabel(row), col+1)
}

func wellPosition(id string, rows, cols int) (int, int, bool) {
	id = strings.ToUpper(strings.TrimSpace(id))
	row, i := 0, 0
	for i < len(id) && id[i] >= 'A' && id[i] <= 'Z' {
		row = row*26 + int(id[i]-'A'+1)
		i++
	}
	if i == 0 || i == len(id) {
		return 0, 0, false
	}
	row--
	col, err := strconv.Atoi(id[i:])
	if err != nil || col < 1 {
		return 0, 0, false
	}
	col--
	return row, col, row >= 0 && row < rows && col >= 0 && col < cols
}

func GroupColor(group string, dark bool) ui.Color {
	g := strings.ToLower(strings.TrimSpace(group))
	t := theme.GetTokens(theme.Palette(dark))
	switch {
	case g == "":
		return ui.Color{}
	case strings.Contains(g, "nc"), strings.Contains(g, "对照"), strings.Contains(g, "control"):
		return t.NeutralBg
	case strings.Contains(g, "pc"), strings.Contains(g, "阳性"), strings.Contains(g, "positive"):
		return t.WarningBg
	case g == "c1":
		return t.Accent
	case g == "c2":
		return t.Success
	case g == "c3":
		return t.Warning
	case g == "c4":
		return t.Danger
	case g == "c5":
		return t.AccentHover
	default:
		palette := [...]ui.Color{t.Accent, t.Success, t.Warning, t.Danger}
		index := 0
		for _, r := range g {
			index = (index*31 + int(r)) % len(palette)
		}
		return palette[index]
	}
}

func selectedSet(props Props) map[string]bool {
	out := make(map[string]bool, len(props.SelectedIDs))
	for _, id := range props.SelectedIDs {
		out[id] = true
	}
	return out
}

func nextSelection(selected map[string]bool, id string, additive bool) map[string]bool {
	next := make(map[string]bool, len(selected)+1)
	if additive {
		for selectedID := range selected {
			next[selectedID] = true
		}
	} else if len(selected) == 1 && selected[id] {
		return next
	}
	if additive && next[id] {
		delete(next, id)
	} else {
		next[id] = true
	}
	return next
}

// View renders the plate. Drag-rectangle selection is not implemented:
// MyGo exposes no pointer-coordinate API for a marquee without a separate model.
// Click selects one well; clicking it again clears it. Shift+click toggles a well in the selection.

func View(c *ui.Context, props Props) {
	th := c.Theme()
	t := theme.GetTokens(th)
	dark := th != nil && th.Dark
	var lastHitWell, lastHoverWell string

	rows, cols := props.Rows, props.Cols
	if rows == 0 {
		rows = defaultRows
	}
	if cols == 0 {
		cols = defaultCols
	}
	isCustomTopology := props.Topology != "" && props.Topology != "plate"
	if !isCustomTopology && !wellGrid(rows, cols) {
		rows, cols = defaultRows, defaultCols
	}
	if rows < 1 || rows > 16 || cols < 1 || cols > 24 {
		rows, cols = defaultRows, defaultCols
	}
	wellSize := float32(t.Spacing24)
	gap := float32(t.Spacing4)
	labelSize := float32(t.TextMeta)
	leftLabel := float32(t.Spacing24)
	topLabel := float32(t.Spacing16)
	gridW := float32(cols)*wellSize + float32(cols-1)*gap
	gridH := float32(rows)*wellSize + float32(rows-1)*gap
	width := leftLabel + gridW + float32(t.Spacing4)
	height := topLabel + gridH + float32(t.Spacing32)
	selected := selectedSet(props)
	groupCounts := make(map[string]int)
	for _, well := range props.Wells {
		if !well.Ambiguous && well.Group != "" {
			groupCounts[well.Group]++
		}
	}

	ui.Column(c).Gap(float32(t.Spacing8)).AlignItems(ui.Center).Children(func() {
		var el ui.Element
		el = ui.Box(c).Size(width, height).Draw(func(p *ui.Painter, r ui.Rect) {
			for col := 0; col < cols; col++ {
				x := r.X + leftLabel + float32(col)*(wellSize+gap) + wellSize/2 - float32(t.Spacing4)
				p.Text(x, r.Y+float32(t.Spacing4), fmt.Sprintf("%d", col+1), labelSize, t.TextMuted)
			}
			for row := 0; row < rows; row++ {
				p.Text(r.X+float32(t.Spacing4), r.Y+topLabel+float32(row)*(wellSize+gap)+wellSize/2+float32(t.Spacing4), rowLabel(row), labelSize, t.TextMuted)
				for col := 0; col < cols; col++ {
					id := wellID(row, col)
					x := r.X + leftLabel + float32(col)*(wellSize+gap)
					y := r.Y + topLabel + float32(row)*(wellSize+gap)
					wr := ui.Rect{X: x, Y: y, W: wellSize, H: wellSize}
					well, assigned := props.Wells[id]
					role := well.Role
					if role == "" && props.Roles != nil {
						role = props.Roles[id]
					}
					fill := t.Canvas
					if assigned && well.Ambiguous {
						fill = t.Warning
					} else if assigned {
						fill = GroupColor(well.Group, dark)
					}
					radius := wellSize / 2
					if props.Topology == "cohort" || props.Topology == "rack_1d" {
						radius = float32(t.ControlRadius)
					}
					p.Fill(wr, fill, radius)
					p.Stroke(wr, t.Border, radius, 1)
					if well.Ambiguous {
						p.Text(x+float32(t.Spacing8), y+float32(t.Spacing4), "!", float32(t.TextMeta), t.Text)
					} else if roleBadge := roleShort(role); roleBadge != "" {
						p.Text(x+float32(t.Spacing4), y+float32(t.Spacing4), roleBadge, float32(t.TextMeta), t.Text)
					}
					if selected[id] {
						p.Stroke(wr, t.Accent, radius, 2)
						p.Line(x+float32(t.Spacing4), y+float32(t.Spacing12), x+float32(t.Spacing8), y+float32(t.Spacing16), 2, t.Accent)
						p.Line(x+float32(t.Spacing8), y+float32(t.Spacing16), x+float32(t.Spacing16), y+float32(t.Spacing8), 2, t.Accent)
					}
				}
			}
			hudY := r.Y + topLabel + gridH + float32(t.Spacing8)
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
				p.Text(r.X+leftLabel, hudY+float32(t.Spacing12), strings.Join(parts, " · "), float32(t.TextMeta), t.Text)
			} else if len(props.Wells) == 0 {
				p.Text(r.X+leftLabel, hudY+float32(t.Spacing12), "尚无计划孔位分配", float32(t.TextMeta), t.TextMuted)
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
			next := nextSelection(selected, lastHitWell, el.ClickModifiers()&ui.Shift != 0)
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
			ui.Row(c).Gap(float32(t.Spacing8)).AlignItems(ui.Center).Children(func() {
				for _, group := range groups {
					ui.Row(c).Gap(float32(t.Spacing4)).AlignItems(ui.Center).Children(func() {
						ui.Box(c).Size(float32(t.Spacing8), float32(t.Spacing8)).Radius(float32(t.ControlRadius)).Background(GroupColor(group, dark))
						ui.Text(c, fmt.Sprintf("%s %d", group, groupCounts[group])).FontSize(float32(t.TextMeta)).TextColor(t.TextMuted)
					})
				}
			})
		}
		ui.Text(c, fmt.Sprintf("%d / %d 孔已分配 · 单击选择，Shift+单击多选", len(props.Wells), rows*cols)).FontSize(float32(t.TextMeta)).TextColor(t.TextMuted)
	})
}

func roleShort(role string) string {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "blank":
		return "B"
	case "positive_control":
		return "+"
	case "negative_control":
		return "-"
	case "standard":
		return "S"
	case "vehicle":
		return "V"
	case "ntc":
		return "N"
	default:
		return ""
	}
}
