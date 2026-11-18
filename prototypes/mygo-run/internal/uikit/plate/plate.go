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
	Factors   map[string]string
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
	ColorBy     string // "group" | "sample" | "factor" | "replicate" | "role"
	FactorKey   string
	Blind       bool
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
	legendCounts := make(map[string]int)
	legendColors := make(map[string]ui.Color)
	legendOrder := make([]string, 0)
	recordLegend := func(label string, col ui.Color) {
		if label == "" {
			return
		}
		if _, seen := legendCounts[label]; !seen {
			legendOrder = append(legendOrder, label)
			legendColors[label] = col
		}
		legendCounts[label]++
	}
	for id, well := range props.Wells {
		if well.Ambiguous {
			continue
		}
		role := effectiveRole(well, id, props.Roles)
		lbl, col := wellLegendCategory(props, well, role, dark, t)
		if lbl != "" {
			recordLegend(lbl, col)
		}
	}
	sort.Strings(legendOrder)

	ui.Column(c).Gap(float32(t.Spacing8)).AlignItems(ui.Center).Children(func() {
		var el ui.Element
		el = ui.Box(c).Size(width, height).Draw(func(p *ui.Painter, r ui.Rect) {
			for col := 0; col < cols; col++ {
				cx := r.X + leftLabel + float32(col)*(wellSize+gap) + wellSize/2 - float32(t.Spacing4)*0.8
				p.Text(cx, r.Y+float32(t.Spacing4)*0.5, fmt.Sprintf("%d", col+1), labelSize, t.TextMuted)
			}
			for row := 0; row < rows; row++ {
				ry := r.Y + topLabel + float32(row)*(wellSize+gap) + (wellSize-labelSize)*0.42
				p.Text(r.X+float32(t.Spacing4), ry, rowLabel(row), labelSize, t.TextMuted)
				for col := 0; col < cols; col++ {
					id := wellID(row, col)
					x := r.X + leftLabel + float32(col)*(wellSize+gap)
					y := r.Y + topLabel + float32(row)*(wellSize+gap)
					wr := ui.Rect{X: x, Y: y, W: wellSize, H: wellSize}
					well, assigned := props.Wells[id]
					role := effectiveRole(well, id, props.Roles)
					fill := WellFillColor(props, well, assigned, role, dark, t)
					radius := wellSize / 2
					if props.Topology == "cohort" || props.Topology == "rack_1d" {
						radius = float32(t.ControlRadius)
					}
					p.Fill(wr, fill, radius)
					strokeCol := t.Border
					if assigned && (len(well.Factors) > 0 || well.SampleID != "") && !props.Blind {
						strokeCol = fill.Mix(t.Text, 0.28)
					}
					p.Stroke(wr, strokeCol, radius, 1)
					ink := contrastInkForFill(fill, assigned, t)
					if !props.Blind && assigned && well.SampleID != "" && len(well.Factors) > 0 && well.Replicate > 0 && wellSize >= 20 {
						inner := ui.Rect{X: x + 2.5, Y: y + 2.5, W: wellSize - 5, H: wellSize - 5}
						innerR := radius - 2.5
						if innerR < 2 {
							innerR = 2
						}
						p.Stroke(inner, ink.Alpha(.38), innerR, 1)
					}
					if well.Ambiguous {
						fontPx := float32(t.TextMeta) * 0.9
						p.Text(x+(wellSize-fontPx*0.4)/2, y+(wellSize-fontPx)*0.44, "!", fontPx, ink)
					} else if !props.Blind && assigned {
						badge := wellGlyph(props, well, role)
						if badge != "" {
							fontPx := float32(t.TextMeta) * 0.68
							if len([]rune(badge)) == 1 {
								fontPx = float32(t.TextMeta) * 0.82
							}
							glyphW := float32(len([]rune(badge))) * fontPx * 0.56
							tx := x + (wellSize-glyphW)/2
							ty := y + (wellSize-fontPx)*0.42
							if tx < x+2 {
								tx = x + 2
							}
							p.Text(tx, ty, badge, fontPx, ink)
						}
					} else if roleBadge := roleShort(role); roleBadge != "" {
						fontPx := float32(t.TextMeta) * 0.78
						p.Text(x+(wellSize-fontPx*0.6)/2, y+(wellSize-fontPx)*0.44, roleBadge, fontPx, t.Text)
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
				role := effectiveRole(well, lastHoverWell, props.Roles)
				parts := []string{lastHoverWell}
				switch {
				case !assigned:
					parts = append(parts, "未分配")
				case well.Ambiguous:
					parts = append(parts, "分配冲突", well.Timepoint)
				case props.Blind:
					parts = append(parts, "盲态编号")
					if well.Replicate > 0 {
						parts = append(parts, fmt.Sprintf("复孔 #%d", well.Replicate))
					}
					if well.Timepoint != "" {
						parts = append(parts, well.Timepoint)
					}
				default:
					if well.Group != "" {
						parts = append(parts, well.Group)
					}
					if well.SampleID != "" {
						parts = append(parts, "样本 "+well.SampleID)
					}
					if len(well.Factors) > 0 {
						fdims := make([]string, 0, len(well.Factors))
						for dk := range well.Factors {
							fdims = append(fdims, dk)
						}
						sort.Strings(fdims)
						for _, dk := range fdims {
							parts = append(parts, dk+"="+well.Factors[dk])
						}
					}
					if well.Replicate > 0 {
						parts = append(parts, fmt.Sprintf("复孔 #%d", well.Replicate))
					}
					if rl := RoleLabel(role); rl != "" && role != "" && role != "sample" {
						parts = append(parts, rl)
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

		if len(legendOrder) > 0 {
			ui.Row(c).Wrap().Gap(float32(t.Spacing8)).AlignItems(ui.Center).Children(func() {
				for _, label := range legendOrder {
					ui.Row(c).Gap(float32(t.Spacing4)).AlignItems(ui.Center).Children(func() {
						ui.Box(c).Size(float32(t.Spacing8), float32(t.Spacing8)).Radius(float32(t.ControlRadius)).Background(legendColors[label])
						ui.Text(c, fmt.Sprintf("%s %d", label, legendCounts[label])).FontSize(float32(t.TextMeta)).TextColor(t.TextMuted).FontFeatures("tnum")
					})
				}
			})
		}
		ui.Text(c, fmt.Sprintf("%d / %d 孔已分配 · 单击选择，Shift+单击多选", len(props.Wells), rows*cols)).FontSize(float32(t.TextMeta)).TextColor(t.TextMuted)
	})
}

func effectiveRole(well WellState, id string, roles map[string]string) string {
	if well.Role != "" {
		return strings.ToLower(strings.TrimSpace(well.Role))
	}
	if roles != nil {
		return strings.ToLower(strings.TrimSpace(roles[id]))
	}
	return ""
}

// RoleLabel returns the researcher-facing Chinese label for a well role.
func RoleLabel(role string) string {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "sample":
		return "常规样本"
	case "blank":
		return "空白对照 Blank"
	case "ntc":
		return "无模板阴参 NTC"
	case "standard":
		return "标准品 Std"
	case "positive_control":
		return "阳性对照"
	case "negative_control":
		return "阴性对照"
	case "vehicle":
		return "溶剂对照"
	default:
		return role
	}
}

// WellFillColor computes the circle fill color according to active ColorBy mode.
func WellFillColor(props Props, well WellState, assigned bool, role string, dark bool, t theme.Tokens) ui.Color {
	if !assigned {
		return t.Canvas
	}
	if well.Ambiguous {
		return t.Warning
	}
	if props.Blind {
		return t.BorderStrong
	}
	switch strings.ToLower(strings.TrimSpace(props.ColorBy)) {
	case "sample":
		if strings.TrimSpace(well.SampleID) != "" {
			return theme.CategoricalColor("sample:"+well.SampleID, dark)
		}
		return t.Elevated
	case "factor":
		if props.FactorKey != "" && well.Factors != nil {
			if val := strings.TrimSpace(well.Factors[props.FactorKey]); val != "" {
				return theme.CategoricalColor(props.FactorKey+":"+val, dark)
			}
		}
		return t.Elevated
	case "replicate":
		if well.Replicate > 0 {
			palette := theme.CategoricalPalette(dark)
			return palette[(well.Replicate-1)%len(palette)]
		}
		return t.Elevated
	case "role":
		r := role
		if r == "" {
			r = "sample"
		}
		return theme.WellRoleColor(r, dark)
	default:
		return GroupColor(well.Group, dark)
	}
}

func wellLegendCategory(props Props, well WellState, role string, dark bool, t theme.Tokens) (string, ui.Color) {
	if props.Blind {
		return "盲态孔位", t.BorderStrong
	}
	switch strings.ToLower(strings.TrimSpace(props.ColorBy)) {
	case "sample":
		if s := strings.TrimSpace(well.SampleID); s != "" {
			return s, theme.CategoricalColor("sample:"+s, dark)
		}
		return "未标样本", t.Elevated
	case "factor":
		if props.FactorKey != "" && well.Factors != nil {
			if v := strings.TrimSpace(well.Factors[props.FactorKey]); v != "" {
				return v, theme.CategoricalColor(props.FactorKey+":"+v, dark)
			}
		}
		return "未标因子", t.Elevated
	case "replicate":
		if well.Replicate > 0 {
			palette := theme.CategoricalPalette(dark)
			return fmt.Sprintf("复孔 #%d", well.Replicate), palette[(well.Replicate-1)%len(palette)]
		}
		return "未标复孔", t.Elevated
	case "role":
		r := role
		if r == "" {
			r = "sample"
		}
		return RoleLabel(r), theme.WellRoleColor(r, dark)
	default:
		if well.Group == "" {
			return "", ui.Color{}
		}
		return well.Group, GroupColor(well.Group, dark)
	}
}

func wellGlyph(props Props, well WellState, role string) string {
	if rs := roleShort(role); rs != "" {
		return rs
	}
	switch strings.ToLower(strings.TrimSpace(props.ColorBy)) {
	case "replicate":
		if well.Replicate > 0 {
			return fmt.Sprintf("#%d", well.Replicate)
		}
	case "sample":
		if s := strings.TrimSpace(well.SampleID); s != "" {
			return compactTag(s, 3)
		}
	case "factor":
		if props.FactorKey != "" && well.Factors != nil {
			if v := strings.TrimSpace(well.Factors[props.FactorKey]); v != "" {
				return compactTag(v, 3)
			}
		}
	default:
		if well.Replicate > 0 {
			return fmt.Sprintf("%d", well.Replicate)
		}
	}
	return ""
}

func compactTag(s string, maxRunes int) string {
	s = strings.TrimSpace(s)
	for _, prefix := range []string{"cDNA-", "sample-", "Std-"} {
		if strings.HasPrefix(s, prefix) && len(s) > len(prefix) {
			s = strings.TrimPrefix(s, prefix)
			break
		}
	}
	rs := []rune(s)
	if len(rs) <= maxRunes {
		return s
	}
	return string(rs[:maxRunes])
}

func contrastInkForFill(fill ui.Color, assigned bool, t theme.Tokens) ui.Color {
	if !assigned || fill.A == 0 {
		return t.Text
	}
	lum := (0.299*float64(fill.R) + 0.587*float64(fill.G) + 0.114*float64(fill.B)) / 255.0
	if lum > 0.54 {
		return theme.LightTokens().Text
	}
	return theme.DarkTokens().Text
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
