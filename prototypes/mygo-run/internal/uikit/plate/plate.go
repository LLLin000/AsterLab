package plate

import (
	"fmt"
	"math"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/mobile"
	"github.com/oneo-me/onevra/source/components"
	"github.com/oneo-me/onevra/source/icons/lucide"

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

// TouchMode selects how touch/pointer interaction behaves inside the mobile plate sheet.
type TouchMode uint8

const (
	TouchModeTap    TouchMode = iota // 点选
	TouchModeBrush                   // 涂抹框选
	TouchModeRowCol                  // 按行·按列快选
)

// InteractionState persists pointer drag-marquee and hover state across frames
// for interactive inline, overlay, and tool plates.
type InteractionState struct {
	pointerDown     bool
	pointerStart    ui.Point
	dragStart       ui.Point
	dragLast        ui.Point
	dragging        bool
	additive        bool
	suppressClick   bool
	baseSelected    []string
	hoverWell       string
	stickyHoverWell string

	TouchMode       TouchMode
	RowColAxis      string // ""|"row"|"col"
	MobileSheetOpen bool
	ZoomState       mobile.ZoomState
	brushDragDummy  ui.Point
}

type DilutionColumn struct {
	ColIndex int
	Header   string
	SubLabel string
	Alpha    float32
}

type DilutionOverlay struct {
	Columns       []DilutionColumn
	TransferLabel string
	ActiveRows    int
}

type Props struct {
	Wells            map[string]WellState
	SelectedIDs      []string
	OnSelect         func(wellIDs []string)
	Interactive      bool
	Rows             int
	Cols             int
	Topology         string
	Roles            map[string]string
	ColorBy          string // "group" | "sample" | "factor" | "replicate" | "role"
	FactorKey        string
	Blind            bool
	GroupIndex       map[string]int
	GroupLabels      map[string]string
	ExcludedIDs      map[string]bool
	HideFooter       bool
	Zoom             float32
	State            *InteractionState
	DilutionFlow     *DilutionOverlay
	MobileCompact    bool
	FullscreenSheet  bool
	OnOpenFullscreen func()
	TouchMode        *TouchMode
	OnAssignGroup    func(groupID string, wellIDs []string)
	OnClearWells     func(wellIDs []string)
}

const (
	defaultRows = 8
	defaultCols = 12
)

// supportedPlateSizes are the geometries AsterLab renders: 4, 6, 12, 24, 48, 96
// and 384 wells as rows×cols. Unsupported dimensions fall back to 96 wells.
var supportedPlateSizes = [...]struct{ Wells, Rows, Cols int }{
	{4, 2, 2},
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
	return GroupColorIndexed(group, nil, dark)
}

// GroupColorIndexed returns a deterministic semantic color for a group,
// preferring sequential CategoricalPalette indexing when groupIndex is supplied
// so adjacent groups never collide.
func GroupColorIndexed(group string, groupIndex map[string]int, dark bool) ui.Color {
	g := strings.ToLower(strings.TrimSpace(group))
	t := theme.GetTokens(theme.Palette(dark))
	switch {
	case g == "":
		return ui.Color{}
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
	}
	if groupIndex != nil {
		if idx, ok := groupIndex[strings.TrimSpace(group)]; ok && idx >= 0 {
			pal := theme.CategoricalPalette(dark)
			return pal[idx%len(pal)]
		}
	}
	switch {
	case strings.Contains(g, "nc"), strings.Contains(g, "空白"), strings.Contains(g, "blank"), strings.Contains(g, "对照"), strings.Contains(g, "control"):
		return t.BorderStrong
	case strings.Contains(g, "pc"), strings.Contains(g, "阳性"), strings.Contains(g, "positive"):
		return t.WarningBg
	default:
		return theme.CategoricalColor("group:"+g, dark)
	}
}

// AdaptiveMetrics returns responsive well/lane pixel dimensions and gap based
// on total well count, container topology, and zoom factor.
func AdaptiveMetrics(rows, cols int, topology string, zoom float32) (cellW, cellH, gap float32) {
	if zoom <= 0 {
		zoom = 1
	}
	gapScale := float32(1)
	if zoom < 1 {
		gapScale = zoom
	}
	scaleGap := func(base float32) float32 {
		g := base * gapScale
		if g < 2 {
			g = 2
		}
		return g
	}
	if topology == "rack_1d" {
		cellW, cellH, gap = 40*zoom, 58*zoom, scaleGap(6)
		if cols > 12 {
			cellW = (560*zoom - float32(cols-1)*gap) / float32(cols)
			if cellW < 18*zoom {
				cellW = 18 * zoom
			}
		}
		return cellW, cellH, gap
	}
	total := rows * cols
	if strings.EqualFold(topology, "dish") && total > 4 {
		switch {
		case total <= 12:
			return 58 * zoom, 58 * zoom, scaleGap(10)
		case total <= 24:
			return 48 * zoom, 48 * zoom, scaleGap(8)
		}
	}
	switch {
	case total <= 4:
		cellW, cellH, gap = 56*zoom, 56*zoom, scaleGap(10)
	case total <= 6:
		cellW, cellH, gap = 48*zoom, 48*zoom, scaleGap(8)
	case total <= 12:
		cellW, cellH, gap = 42*zoom, 42*zoom, scaleGap(8)
	case total <= 24:
		cellW, cellH, gap = 36*zoom, 36*zoom, scaleGap(6)
	case total <= 48:
		cellW, cellH, gap = 28*zoom, 28*zoom, scaleGap(5)
	case total <= 96:
		cellW, cellH, gap = 26*zoom, 26*zoom, scaleGap(4)
	default:
		cellW, cellH, gap = 17*zoom, 17*zoom, scaleGap(3)
	}
	return cellW, cellH, gap
}

// ResolveDimensions normalizes plate/dish/rack row & column counts.
func ResolveDimensions(props Props) (rows, cols int) {
	rows, cols = props.Rows, props.Cols
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
	return rows, cols
}

// LegendItem describes one category swatch rendered below a plate grid or export.
type LegendItem struct {
	Label string
	Color ui.Color
	Count int
}

// ComputeLegend returns the sorted legend entries shared across interactive views,
// read-only summaries, and SVG/PNG exports. Numeric factor layers sort high-to-low
// with monotonic sequential ramp swatches instead of unordered string hashes.
func ComputeLegend(props Props, dark bool) []LegendItem {
	t := theme.GetTokens(theme.Palette(dark))
	counts := make(map[string]int)
	colors := make(map[string]ui.Color)
	order := make([]string, 0)
	scale := buildFactorColorScale(props, dark, t)
	for id, well := range props.Wells {
		if well.Ambiguous {
			continue
		}
		role := effectiveRole(well, id, props.Roles)
		lbl, col := wellLegendCategoryWithScale(props, well, role, dark, t, scale)
		if lbl == "" {
			continue
		}
		if _, seen := counts[lbl]; !seen {
			order = append(order, lbl)
			colors[lbl] = col
		}
		counts[lbl]++
	}
	if scale != nil && scale.isNumeric {
		sort.Slice(order, func(i, j int) bool {
			vi, oki := parseNumericFactorValue(order[i])
			vj, okj := parseNumericFactorValue(order[j])
			if oki && okj {
				if vi != vj {
					return vi > vj
				}
				return order[i] < order[j]
			}
			if oki != okj {
				return oki
			}
			return order[i] < order[j]
		})
	} else {
		sort.Strings(order)
	}
	out := make([]LegendItem, 0, len(order))
	for _, lbl := range order {
		out = append(out, LegendItem{Label: lbl, Color: colors[lbl], Count: counts[lbl]})
	}
	return out
}

type factorColorScale struct {
	isNumeric bool
	colors    map[string]ui.Color
}

func parseNumericFactorValue(raw string) (float64, bool) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return 0, false
	}
	end := 0
	for end < len(s) {
		ch := s[end]
		if (ch >= '0' && ch <= '9') || ch == '.' || (end == 0 && (ch == '+' || ch == '-')) {
			end++
			continue
		}
		break
	}
	if end == 0 {
		return 0, false
	}
	v, err := strconv.ParseFloat(s[:end], 64)
	if err != nil || math.IsNaN(v) || math.IsInf(v, 0) {
		return 0, false
	}
	rest := strings.TrimSpace(s[end:])
	if rest != "" {
		for _, r := range rest {
			if unicode.IsDigit(r) && !strings.ContainsAny(rest, "uUmMnNpPkKgGLl%") {
				return 0, false
			}
		}
	}
	return v, true
}

func buildFactorColorScale(props Props, dark bool, t theme.Tokens) *factorColorScale {
	if !strings.EqualFold(strings.TrimSpace(props.ColorBy), "factor") || props.FactorKey == "" {
		return nil
	}
	seen := map[string]bool{}
	var vals []string
	var numVals []float64
	for _, well := range props.Wells {
		if well.Factors == nil {
			continue
		}
		v := strings.TrimSpace(well.Factors[props.FactorKey])
		if v == "" || seen[v] {
			continue
		}
		seen[v] = true
		vals = append(vals, v)
		if num, ok := parseNumericFactorValue(v); ok && num > 0 {
			numVals = append(numVals, num)
		}
	}
	if len(vals) == 0 {
		return nil
	}
	colors := make(map[string]ui.Color, len(vals))
	isNumeric := len(numVals) >= 2
	if isNumeric {
		minV, maxV := numVals[0], numVals[0]
		for _, nv := range numVals[1:] {
			if nv < minV {
				minV = nv
			}
			if nv > maxV {
				maxV = nv
			}
		}
		useLog := minV > 0 && maxV/minV >= 8
		for _, v := range vals {
			nv, ok := parseNumericFactorValue(v)
			if !ok || nv <= 0 {
				colors[v] = t.BorderStrong
				continue
			}
			norm := 0.5
			if maxV > minV {
				if useLog {
					norm = (math.Log10(nv) - math.Log10(minV)) / (math.Log10(maxV) - math.Log10(minV))
				} else {
					norm = (nv - minV) / (maxV - minV)
				}
			}
			colors[v] = theme.GradientRampColor(norm, dark)
		}
		return &factorColorScale{isNumeric: true, colors: colors}
	}
	sort.Strings(vals)
	pal := theme.CategoricalPalette(dark)
	for i, v := range vals {
		if len(pal) > 0 {
			colors[v] = pal[i%len(pal)]
		} else {
			colors[v] = theme.CategoricalColor(props.FactorKey+":"+v, dark)
		}
	}
	return &factorColorScale{isNumeric: false, colors: colors}
}

// WellsInRectCells returns every well/lane ID overlapping rect in row-major order.
func WellsInRectCells(rows, cols int, cellW, cellH, gap float32, rect ui.Rect) []string {
	var selected []string
	for row := 0; row < rows; row++ {
		for col := 0; col < cols; col++ {
			cellRect := ui.Rect{
				X: float32(col) * (cellW + gap),
				Y: float32(row) * (cellH + gap),
				W: cellW,
				H: cellH,
			}
			if cellRect.X <= rect.X+rect.W && cellRect.X+cellRect.W >= rect.X &&
				cellRect.Y <= rect.Y+rect.H && cellRect.Y+cellRect.H >= rect.Y {
				selected = append(selected, wellID(row, col))
			}
		}
	}
	return selected
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

func isNarrowPlateViewport(c *ui.Context) bool {
	if runtime.GOOS == "android" || runtime.GOOS == "ios" {
		return true
	}
	w, _ := c.Size()
	return w > 0 && w < 680
}

func shouldRenderCompactMobileCard(c *ui.Context, props Props, rows, cols int, mini bool) bool {
	if props.FullscreenSheet || props.HideFooter || mini {
		return false
	}
	if props.Topology == "rack_1d" {
		return false
	}
	if cols < 6 && rows*cols < 24 {
		return false
	}
	return props.MobileCompact || isNarrowPlateViewport(c)
}

type mobilePlateStateKey struct {
	rows, cols int
	topology   string
}

func plateGeometryLabel(rows, cols int, topology string) string {
	switch strings.TrimSpace(topology) {
	case "dish":
		return fmt.Sprintf("%d 区培养皿 · %d×%d", rows*cols, rows, cols)
	case "rack_1d":
		return fmt.Sprintf("%d 联管 / 泳道", rows*cols)
	case "cohort":
		return fmt.Sprintf("%d×%d 队列", rows, cols)
	default:
		return fmt.Sprintf("%d 孔板 · %d×%d", rows*cols, rows, cols)
	}
}

// ComputeFullscreenFitZoom returns a zoom factor so the entire plate fits cleanly
// inside a mobile sheet viewport at 1x Zoomable scale before two-finger pinching.
func ComputeFullscreenFitZoom(viewportW float32, rows, cols int, topology string, t theme.Tokens) float32 {
	if viewportW <= 0 {
		return 1
	}
	cellW, _, gap := AdaptiveMetrics(rows, cols, topology, 1)
	naturalW := float32(t.Spacing24) + float32(cols)*cellW + float32(max(0, cols-1))*gap + float32(t.Spacing24)
	availW := viewportW - float32(t.Spacing32)
	if availW <= 0 || naturalW <= 0 {
		return 1
	}
	if naturalW <= availW {
		return 1
	}
	z := availW / naturalW
	if z < 0.55 {
		z = 0.55
	}
	return z
}

// View renders the plate. Drag-rectangle selection is not implemented:
// MyGo exposes no pointer-coordinate API for a marquee without a separate model.
// Click selects one well; clicking it again clears it. Shift+click toggles a well in the selection.

func View(c *ui.Context, props Props) {
	th := c.Theme()
	t := theme.GetTokens(th)
	dark := th != nil && th.Dark
	var lastHoverWell string

	rows, cols := ResolveDimensions(props)
	isDish := strings.EqualFold(props.Topology, "dish")
	isDishMatrix := isDish && rows*cols > 4
	isSingleDish := isDish && !isDishMatrix
	cellW, cellH, gap := AdaptiveMetrics(rows, cols, props.Topology, props.Zoom)
	mini := !props.Interactive && props.Zoom > 0 && props.Zoom < 0.75
	selected := selectedSet(props)
	legend := ComputeLegend(props, dark)

	if shouldRenderCompactMobileCard(c, props, rows, cols, mini) {
		st := props.State
		if st == nil {
			stPtr := ui.Local(c.Root(), mobilePlateStateKey{rows, cols, props.Topology}, func() *InteractionState {
				return &InteractionState{}
			})
			st = *stPtr
		}
		renderCompactMobileCard(c, props, rows, cols, legend, selected, st, t)
		if props.OnOpenFullscreen == nil {
			renderMobilePlateSheet(c, props, rows, cols, legend, st, t, dark)
		}
		return
	}
	labelSize := float32(t.TextMeta)
	leftLabel := float32(t.Spacing24)
	topLabel := float32(t.Spacing16)
	dilHeaderH := float32(0)
	if !props.Interactive && props.DilutionFlow != nil && len(props.DilutionFlow.Columns) > 0 {
		dilHeaderH = float32(t.Spacing32) + float32(t.Spacing8)
	}
	dishPad := float32(0)
	footerPad := float32(t.Spacing32)
	if isSingleDish {
		dishPad = float32(t.Spacing24)
	}
	if mini {
		if labelSize = labelSize * (0.7 + 0.3*props.Zoom); labelSize < 9 {
			labelSize = 9
		}
		if leftLabel = float32(t.Spacing24) * (0.65 + 0.35*props.Zoom); leftLabel < 15 {
			leftLabel = 15
		}
		if topLabel = float32(t.Spacing16) * (0.65 + 0.35*props.Zoom); topLabel < 11 {
			topLabel = 11
		}
		if isSingleDish {
			dishPad = float32(t.Spacing24) * (0.55 + 0.45*props.Zoom)
			if dishPad < 20 {
				dishPad = 20
			}
		}
		footerPad = float32(t.Spacing8)
	}
	gridW := float32(cols)*cellW + float32(cols-1)*gap
	gridH := float32(rows)*cellH + float32(rows-1)*gap
	width := leftLabel + gridW + dishPad*2 + float32(t.Spacing4)
	height := dilHeaderH + topLabel + gridH + dishPad*2 + footerPad
	legendCounts := make(map[string]int, len(legend))
	legendColors := make(map[string]ui.Color, len(legend))
	legendOrder := make([]string, 0, len(legend))
	for _, item := range legend {
		legendOrder = append(legendOrder, item.Label)
		legendColors[item.Label] = item.Color
		legendCounts[item.Label] = item.Count
	}

	wellRadius := func() float32 {
		if props.Topology == "rack_1d" {
			return float32(t.ControlRadius) * 2
		}
		if props.Topology == "cohort" {
			return float32(t.ControlRadius)
		}
		return cellW / 2
	}

	drawDishRim := func(p *ui.Painter, gx0, gy0 float32) {
		if !isSingleDish {
			return
		}
		cx := gx0 + gridW/2
		cy := gy0 + gridH/2
		rOuter := gridW
		if gridH > rOuter {
			rOuter = gridH
		}
		rimScale := float32(0.72)
		rimPad := float32(t.Spacing8)
		if mini {
			rimScale = 0.65
			rimPad = float32(t.Spacing4)
		}
		rOuter = rOuter*rimScale + rimPad
		outerRect := ui.Rect{X: cx - rOuter, Y: cy - rOuter, W: rOuter * 2, H: rOuter * 2}
		p.Fill(outerRect, t.Elevated.Alpha(.65), rOuter)
		p.Stroke(outerRect, t.BorderStrong, rOuter, 1.5)
		innerR := rOuter - rimPad
		if innerR > 8 {
			innerRect := ui.Rect{X: cx - innerR, Y: cy - innerR, W: innerR * 2, H: innerR * 2}
			p.Stroke(innerRect, t.Border, innerR, 1)
		}
		if rows == 2 && cols == 2 {
			p.Line(cx-rOuter+float32(t.Spacing8), cy, cx+rOuter-float32(t.Spacing8), cy, 1, t.BorderStrong.Alpha(.55))
			p.Line(cx, cy-rOuter+float32(t.Spacing8), cx, cy+rOuter-float32(t.Spacing8), 1, t.BorderStrong.Alpha(.55))
		}
	}

	ui.Column(c).FillWidth().Gap(float32(t.Spacing8)).AlignItems(ui.Center).Children(func() {
		if props.Interactive {
			st := props.State
			if st == nil {
				st = &InteractionState{}
			}
			suppressGridClick := st.suppressClick
			st.suppressClick = false
			st.hoverWell = ""

			touchActive := props.FullscreenSheet || props.TouchMode != nil
			activeMode := st.TouchMode
			if props.TouchMode != nil {
				activeMode = *props.TouchMode
			}

			// UI-02: Place color legend cleanly in the top-right above the plate/dish so it never overlaps the dish rim.
			if len(legendOrder) > 0 && !props.FullscreenSheet {
				ui.Row(c).FillWidth().Wrap().Justify(ui.End).Gap(float32(t.Spacing8)).AlignItems(ui.Center).Children(func() {
					for _, label := range legendOrder {
						dispLbl := formatPlateSuperLabel(label)
						ui.Row(c).Key("leg-"+label).Gap(float32(t.Spacing4)).AlignItems(ui.Center).
							Transition(theme.AccordionTransition()).Children(func() {
							sw := ui.Box(c).Size(float32(t.Spacing8), float32(t.Spacing8)).Radius(float32(t.ControlRadius))
							sw.Background(sw.AnimateColor("lcol", legendColors[label], theme.MotionSnappy))
							ui.Text(c, fmt.Sprintf("%s (%d)", dispLbl, legendCounts[label])).FontSize(labelSize).
								TextColor(t.TextMuted).FontFeatures("tnum").ContentTransition(theme.NumericRoll(false))
						})
					}
				})
			}

			// Compute exact padding around single dish rim so the outer circle never spills over bottom text (UI-02).
			dishPadH := dishPad
			dishPadV := dishPad
			if isSingleDish {
				rOuter := gridW
				if gridH > rOuter {
					rOuter = gridH
				}
				rimScale := float32(0.72)
				rimPad := float32(t.Spacing8)
				if mini {
					rimScale = 0.65
					rimPad = float32(t.Spacing4)
				}
				rOuter = rOuter*rimScale + rimPad
				needH := rOuter - gridW/2 + float32(t.Spacing8)
				needV := rOuter - gridH/2 + float32(t.Spacing8)
				if needH > dishPadH {
					dishPadH = needH
				}
				if needV > dishPadV {
					dishPadV = needV
				}
			}

			notify := func(ids []string) {
				sort.Strings(ids)
				if props.OnSelect != nil {
					props.OnSelect(ids)
				}
			}
			mergeSelect := func(ids []string, additive bool) {
				seen := map[string]bool{}
				out := make([]string, 0, len(props.SelectedIDs)+len(ids))
				if additive {
					for _, id := range props.SelectedIDs {
						if !seen[id] {
							seen[id] = true
							out = append(out, id)
						}
					}
				}
				for _, id := range ids {
					if !seen[id] {
						seen[id] = true
						out = append(out, id)
					}
				}
				notify(out)
			}
			toggleBatchSelection := func(ids []string) {
				allSelected := len(ids) > 0
				for _, id := range ids {
					if !selected[id] {
						allSelected = false
						break
					}
				}
				next := make(map[string]bool, len(selected)+len(ids))
				for id := range selected {
					next[id] = true
				}
				if allSelected {
					for _, id := range ids {
						delete(next, id)
					}
				} else {
					for _, id := range ids {
						next[id] = true
					}
				}
				out := make([]string, 0, len(next))
				for id := range next {
					out = append(out, id)
				}
				notify(out)
			}
			rowWellIDs := func(rIdx int) []string {
				ids := make([]string, 0, cols)
				for cc := 0; cc < cols; cc++ {
					ids = append(ids, wellID(rIdx, cc))
				}
				return ids
			}
			colWellIDs := func(cIdx int) []string {
				ids := make([]string, 0, rows)
				for rr := 0; rr < rows; rr++ {
					ids = append(ids, wellID(rr, cIdx))
				}
				return ids
			}
			applyMarquee := func() {
				x0, y0 := st.dragStart.X, st.dragStart.Y
				x1, y1 := st.dragLast.X, st.dragLast.Y
				if x0 > x1 {
					x0, x1 = x1, x0
				}
				if y0 > y1 {
					y0, y1 = y1, y0
				}
				hit := WellsInRectCells(rows, cols, cellW, cellH, gap, ui.Rect{X: x0, Y: y0, W: x1 - x0, H: y1 - y0})
				seen := map[string]bool{}
				out := make([]string, 0, len(st.baseSelected)+len(hit))
				if st.additive {
					for _, id := range st.baseSelected {
						if !seen[id] {
							seen[id] = true
							out = append(out, id)
						}
					}
				}
				for _, id := range hit {
					if !seen[id] {
						seen[id] = true
						out = append(out, id)
					}
				}
				notify(out)
			}

			gridBox := ui.Column(c).AlignItems(ui.Center).Padding(dishPadV, dishPadH, dishPadV, dishPadH).Children(func() {
				ui.Column(c).Gap(gap).AlignItems(ui.Center).Children(func() {
					ui.Row(c).Gap(gap).AlignItems(ui.Center).Children(func() {
						ui.Box(c).Size(leftLabel, topLabel)
						for col := 0; col < cols; col++ {
							cIdx := col
							hdrBtn := ui.ButtonBase(c).Width(cellW).Height(topLabel).
								Label(fmt.Sprintf("选择第 %d 列", cIdx+1))
							hdrBtn.Children(func() {
								ui.Box(c).Fill().Center().Children(func() {
									ui.Text(c, fmt.Sprintf("%d", cIdx+1)).FontSize(labelSize).TextColor(t.TextMuted).SingleLine()
								})
							})
							if hdrBtn.Clicked() && !st.dragging && !suppressGridClick {
								ids := colWellIDs(cIdx)
								if touchActive {
									toggleBatchSelection(ids)
									mygo.Haptics.Selection()
								} else {
									mergeSelect(ids, hdrBtn.ClickModifiers()&(ui.Shift|ui.Cmd) != 0)
								}
							}
						}
					})
					for row := 0; row < rows; row++ {
						rIdx := row
						ui.Row(c).Gap(gap).AlignItems(ui.Center).Children(func() {
							rBtn := ui.ButtonBase(c).Width(leftLabel).Height(cellH).
								Label("选择第 " + rowLabel(rIdx) + " 行")
							rBtn.Children(func() {
								ui.Box(c).Fill().Center().Children(func() {
									ui.Text(c, rowLabel(rIdx)).FontSize(labelSize).TextColor(t.TextMuted).SingleLine()
								})
							})
							if rBtn.Clicked() && !st.dragging && !suppressGridClick {
								ids := rowWellIDs(rIdx)
								if touchActive {
									toggleBatchSelection(ids)
									mygo.Haptics.Selection()
								} else {
									mergeSelect(ids, rBtn.ClickModifiers()&(ui.Shift|ui.Cmd) != 0)
								}
							}
							for col := 0; col < cols; col++ {
								cIdx := col
								id := wellID(rIdx, cIdx)
								well, assigned := props.Wells[id]
								role := effectiveRole(well, id, props.Roles)
								fill := WellFillColor(props, well, assigned, role, dark, t)
								strokeCol := t.Border
								var strokeW float32 = 1
								if assigned && (len(well.Factors) > 0 || well.SampleID != "") && !props.Blind {
									strokeCol = fill.Mix(t.Text, 0.28)
								}
								if selected[id] {
									strokeCol, strokeW = t.Accent, 2
								}
								ink := contrastInkForFill(fill, assigned, t)
								badge := ""
								if !props.ExcludedIDs[id] {
									if well.Ambiguous {
										badge = "!"
									} else if !props.Blind && assigned {
										badge = wellGlyphAdaptive(props, well, role, cellW)
									} else if rb := roleShort(role); rb != "" {
										badge = rb
									}
								}
								btn := ui.ButtonBase(c).Key(id).Size(cellW, cellH).Radius(wellRadius()).
									Label("孔 " + id)
								animFill := btn.AnimateColor("wfill", fill, theme.MotionSnappy)
								animStroke := btn.AnimateColor("wstroke", strokeCol, theme.MotionSnappy)
								btn.Background(animFill).Border(strokeW, animStroke)
								if !assigned {
									theme.ApplyRecessedWell(btn, t)
								}
								if isDishMatrix && cellW >= 24 {
									innerRimCol := strokeCol.Alpha(0.45)
									btn.Draw(func(p *ui.Painter, r ui.Rect) {
										inset := float32(3.5)
										if r.W > inset*2+8 {
											ir := ui.Rect{X: r.X + inset, Y: r.Y + inset, W: r.W - inset*2, H: r.H - inset*2}
											p.Stroke(ir, innerRimCol, ir.W/2, 1)
										}
									})
								}
								targetScale := float32(1.0)
								if btn.Pressed() {
									targetScale = 0.93
								} else if selected[id] && st.hoverWell == id {
									targetScale = 1.04
								}
								if sc := btn.AnimateTo("wscale", targetScale, theme.MotionSnappy); sc > 0 && sc != 1.0 {
									btn.Scale(sc, sc)
								}
								if badge != "" || (selected[id] && cellW >= 32) {
									fontPx := float32(t.TextMeta) * 0.72
									if cellW >= 40 {
										fontPx = float32(t.TextMeta) * 0.86
									} else if len([]rune(badge)) == 1 {
										fontPx = float32(t.TextMeta) * 0.84
									}
									btn.Children(func() {
										ui.Column(c).Fill().Center().Gap(1).Children(func() {
											if badge != "" {
												ui.Text(c, badge).FontSize(fontPx).Bold().TextColor(ink).SingleLine()
											}
											if !props.Blind && assigned && well.Replicate > 0 && cellW >= 38 && props.ColorBy != "replicate" {
												ui.Text(c, fmt.Sprintf("#%d", well.Replicate)).FontSize(float32(t.TextMeta)*0.64).TextColor(ink.Alpha(.82)).SingleLine()
											}
										})
									})
								}
								if props.ExcludedIDs[id] {
									btn.DrawOver(func(p *ui.Painter, r ui.Rect) {
										pad := r.W * 0.28
										cross := &ui.Path{}
										cross.MoveTo(r.X+pad, r.Y+pad)
										cross.LineTo(r.X+r.W-pad, r.Y+r.H-pad)
										cross.MoveTo(r.X+r.W-pad, r.Y+pad)
										cross.LineTo(r.X+pad, r.Y+r.H-pad)
										p.StrokePath(cross, 2.2, t.Danger)
									})
								}
								if btn.Hovered() {
									st.hoverWell = id
								}
								cellPt := func(x, y float32) ui.Point {
									return ui.Point{
										X: float32(cIdx)*(cellW+gap) + x,
										Y: float32(rIdx)*(cellH+gap) + y,
									}
								}
								allowBrushDrag := !touchActive || activeMode == TouchModeBrush
								if allowBrushDrag {
									brushExclusive := touchActive && activeMode == TouchModeBrush
									if brushExclusive {
										st.brushDragDummy = ui.Point{}
										btn.Draggable(&st.brushDragDummy, ui.DraggableOptions{
											Bounds: &ui.Rect{},
											Rigid:  true,
										})
									}
									btn = btn.HandleInput(func(event ui.InputEvent) bool {
										if (event.Kind == ui.InputPointerDown || event.Kind == ui.InputPointerUp) && event.Button != 0 {
											return false
										}
										switch event.Kind {
										case ui.InputPointerDown:
											pt := cellPt(event.X, event.Y)
											st.pointerDown = true
											st.pointerStart = ui.Point{X: event.X, Y: event.Y}
											st.dragStart, st.dragLast = pt, pt
											st.dragging = false
											st.additive = touchActive || event.Mods&(ui.Shift|ui.Cmd) != 0
											st.baseSelected = append(st.baseSelected[:0], props.SelectedIDs...)
											return brushExclusive
										case ui.InputPointerMove:
											if !st.pointerDown {
												return false
											}
											dx, dy := event.X-st.pointerStart.X, event.Y-st.pointerStart.Y
											if !st.dragging {
												if dx > -3 && dx < 3 && dy > -3 && dy < 3 {
													return brushExclusive
												}
												st.dragging = true
											}
											st.dragLast = cellPt(event.X, event.Y)
											applyMarquee()
											return true
										case ui.InputPointerUp:
											if !st.pointerDown {
												return false
											}
											st.pointerDown = false
											if !st.dragging {
												st.baseSelected = nil
												if brushExclusive && !event.Canceled {
													mygo.Haptics.Selection()
													next := nextSelection(selected, id, true)
													out := make([]string, 0, len(next))
													for k := range next {
														out = append(out, k)
													}
													notify(out)
													return true
												}
												return false
											}
											if event.Canceled {
												notify(append([]string(nil), st.baseSelected...))
											} else {
												st.dragLast = cellPt(event.X, event.Y)
												applyMarquee()
												if touchActive {
													mygo.Haptics.Selection()
												}
											}
											st.dragging = false
											st.suppressClick = true
											st.baseSelected = nil
											return true
										}
										return false
									})
								}
								if btn.Clicked() && !st.dragging && !suppressGridClick {
									if touchActive {
										mygo.Haptics.Selection()
										switch activeMode {
										case TouchModeRowCol:
											if st.RowColAxis == "col" {
												toggleBatchSelection(colWellIDs(cIdx))
											} else {
												toggleBatchSelection(rowWellIDs(rIdx))
											}
										default:
											next := nextSelection(selected, id, true)
											out := make([]string, 0, len(next))
											for k := range next {
												out = append(out, k)
											}
											notify(out)
										}
									} else {
										next := nextSelection(selected, id, btn.ClickModifiers()&(ui.Shift|ui.Cmd) != 0)
										out := make([]string, 0, len(next))
										for k := range next {
											out = append(out, k)
										}
										notify(out)
									}
								}
							}
						})
					}
				})
			})
			if isSingleDish {
				gridBox.Draw(func(p *ui.Painter, r ui.Rect) {
					drawDishRim(p, r.X+dishPadH+leftLabel+gap, r.Y+dishPadV+topLabel+gap)
				})
			}
			gridBox.DrawOver(func(p *ui.Painter, r ui.Rect) {
				if !st.dragging {
					return
				}
				ox := r.X + dishPadH + leftLabel + gap
				oy := r.Y + dishPadV + topLabel + gap
				x0, y0 := ox+st.dragStart.X, oy+st.dragStart.Y
				x1, y1 := ox+st.dragLast.X, oy+st.dragLast.Y
				if x0 > x1 {
					x0, x1 = x1, x0
				}
				if y0 > y1 {
					y0, y1 = y1, y0
				}
				mr := ui.Rect{X: x0, Y: y0, W: x1 - x0, H: y1 - y0}
				p.Fill(mr, t.Accent.Alpha(.14), 2)
				p.Stroke(mr, t.Accent, 2, 1)
			})

			// Sticky hover prevents flashing/jumping when moving across inter-well gaps (UI-11).
			if st.hoverWell != "" {
				st.stickyHoverWell = st.hoverWell
			} else if !gridBox.Hovered() {
				st.stickyHoverWell = ""
			}

			// UI-02: Single concise bottom status/hover bar strictly below the dish rim.
			ui.Row(c).FillWidth().Height(float32(t.Spacing24)).Padding(0, float32(t.Spacing8)).
				AlignItems(ui.Center).Gap(float32(t.Spacing12)).Children(func() {
				allocText := fmt.Sprintf("%d / %d 孔已分配", len(props.Wells), rows*cols)
				ui.Text(c, allocText).FontSize(labelSize).TextColor(t.TextMuted).FontFeatures("tnum").
					ContentTransition(theme.NumericRoll(false)).Shrink(0).SingleLine()
				ui.Box(c).Width(1).Height(float32(t.Spacing12)).Background(t.Border)
				hoverText, hoverColor := "点击/框选或右键排板", t.TextMuted
				if st.stickyHoverWell != "" {
					hoverText, hoverColor = formatPlateSuperLabel(wellHoverSummary(props, st.stickyHoverWell)), t.Text
				} else if len(props.Wells) == 0 {
					hoverText = "尚无计划孔位分配 · 点击或拖拽框选"
				}
				ui.Text(c, hoverText).FontSize(labelSize).TextColor(hoverColor).
					SingleLine().Ellipsis("…").Grow(1).MinWidth(0)
			})
			return
		}

		dilByCol := map[int]DilutionColumn{}
		if props.DilutionFlow != nil {
			for _, dc := range props.DilutionFlow.Columns {
				dilByCol[dc.ColIndex] = dc
			}
		}
		dilActiveRows := 0
		if props.DilutionFlow != nil {
			dilActiveRows = props.DilutionFlow.ActiveRows
			if dilActiveRows <= 0 {
				dilActiveRows = rows
			}
		}

		canvas := ui.Box(c).Size(width, height).Draw(func(p *ui.Painter, r ui.Rect) {
			gx0 := r.X + leftLabel + dishPad
			gy0 := r.Y + dilHeaderH + topLabel + dishPad
			drawDishRim(p, gx0, gy0)
			if dilHeaderH > 0 && props.DilutionFlow != nil && len(props.DilutionFlow.Columns) > 0 {
				minCol, maxCol := cols, -1
				for colIdx := range dilByCol {
					if colIdx >= 0 && colIdx < cols {
						if colIdx < minCol {
							minCol = colIdx
						}
						if colIdx > maxCol {
							maxCol = colIdx
						}
					}
				}
				if props.DilutionFlow.TransferLabel != "" && minCol <= maxCol {
					spanX0 := gx0 + float32(minCol)*(cellW+gap)
					spanX1 := gx0 + float32(maxCol)*(cellW+gap) + cellW
					banner := fmt.Sprintf("排枪逐列混匀移液 %s →", props.DilutionFlow.TransferLabel)
					bfSize := labelSize - 1
					if bfSize < 9.5 {
						bfSize = 9.5
					}
					bw := estimatedBadgeWidth(banner, bfSize)
					bx := spanX0 + (spanX1-spanX0-bw)/2
					if bx < gx0 {
						bx = gx0
					}
					p.Text(bx, r.Y+dishPad+1, banner, bfSize, t.Accent)
				}
				hdrY := r.Y + dishPad + float32(t.Spacing16) + 2
				for col := 0; col < cols; col++ {
					dc, ok := dilByCol[col]
					if !ok || dc.Header == "" {
						continue
					}
					hf := labelSize - 1.5
					if hf < 8.5 {
						hf = 8.5
					}
					hw := estimatedBadgeWidth(dc.Header, hf)
					maxAllow := cellW + gap - 1
					if hw > maxAllow && hw > 0 {
						hf *= maxAllow / hw
						if hf < 7.5 {
							hf = 7.5
						}
						hw = estimatedBadgeWidth(dc.Header, hf)
					}
					cx := gx0 + float32(col)*(cellW+gap) + (cellW-hw)/2
					colInk := t.Text
					if col == minCol {
						colInk = t.Accent
					}
					p.Text(cx, hdrY, dc.Header, hf, colInk)
					if _, hasNext := dilByCol[col+1]; hasNext && col+1 < cols {
						ax0 := gx0 + float32(col)*(cellW+gap) + cellW + 0.5
						ax1 := gx0 + float32(col+1)*(cellW+gap) - 0.5
						ay := gy0 + float32(dilActiveRows)*(cellH+gap)/2 - gap/2
						if ax1 > ax0+1 {
							arrow := &ui.Path{}
							arrow.MoveTo(ax0, ay)
							arrow.LineTo(ax1, ay)
							arrow.MoveTo(ax1-2.2, ay-2.0)
							arrow.LineTo(ax1, ay)
							arrow.LineTo(ax1-2.2, ay+2.0)
							p.StrokePath(arrow, 1.3, t.Accent.Alpha(0.85))
						}
					}
				}
			}
			for col := 0; col < cols; col++ {
				cx := gx0 + float32(col)*(cellW+gap) + cellW/2 - float32(t.Spacing4)*0.8
				p.Text(cx, r.Y+dilHeaderH+dishPad+float32(t.Spacing4)*0.4, fmt.Sprintf("%d", col+1), labelSize, t.TextMuted)
			}
			for row := 0; row < rows; row++ {
				ry := gy0 + float32(row)*(cellH+gap) + (cellH-labelSize)*0.42
				p.Text(r.X+dishPad+float32(t.Spacing4), ry, rowLabel(row), labelSize, t.TextMuted)
				for col := 0; col < cols; col++ {
					id := wellID(row, col)
					x := gx0 + float32(col)*(cellW+gap)
					y := gy0 + float32(row)*(cellH+gap)
					wr := ui.Rect{X: x, Y: y, W: cellW, H: cellH}
					well, assigned := props.Wells[id]
					role := effectiveRole(well, id, props.Roles)
					fill := WellFillColor(props, well, assigned, role, dark, t)
					if assigned {
						if dc, ok := dilByCol[col]; ok && row < dilActiveRows && dc.Alpha > 0 {
							targetHue := fill
							if targetHue == t.Canvas || targetHue == t.Elevated {
								targetHue = t.Accent
							}
							fill = t.Elevated.Mix(targetHue, dc.Alpha)
						}
					}
					radius := wellRadius()
					p.Fill(wr, fill, radius)
					strokeCol := t.Border
					strokeWidth := float32(1)
					if isDishMatrix {
						strokeCol = t.BorderStrong
						strokeWidth = 1.5
					}
					if assigned && (len(well.Factors) > 0 || well.SampleID != "") && !props.Blind {
						strokeCol = fill.Mix(t.Text, 0.28)
					}
					p.Stroke(wr, strokeCol, radius, strokeWidth)
					if isDishMatrix && cellW >= 24 {
						inset := float32(3.5)
						innerRect := ui.Rect{X: x + inset, Y: y + inset, W: cellW - inset*2, H: cellH - inset*2}
						p.Stroke(innerRect, strokeCol.Alpha(0.42), innerRect.W/2, 1)
					}
					ink := contrastInkForFill(fill, assigned, t)
					if !props.Blind && assigned && well.SampleID != "" && len(well.Factors) > 0 && well.Replicate > 0 && cellW >= 20 {
						inner := ui.Rect{X: x + 2.5, Y: y + 2.5, W: cellW - 5, H: cellH - 5}
						innerR := radius - 2.5
						if innerR < 2 {
							innerR = 2
						}
						p.Stroke(inner, ink.Alpha(.38), innerR, 1)
					}
					if props.ExcludedIDs[id] {
						pad := cellW * 0.28
						cross := &ui.Path{}
						cross.MoveTo(x+pad, y+pad)
						cross.LineTo(x+cellW-pad, y+cellH-pad)
						cross.MoveTo(x+cellW-pad, y+pad)
						cross.LineTo(x+pad, y+cellH-pad)
						p.StrokePath(cross, 2.2, t.Danger)
					} else if well.Ambiguous {
						fontPx := labelSize * 0.9
						p.Text(x+(cellW-fontPx*0.4)/2, y+(cellH-fontPx)*0.44, "!", fontPx, ink)
					} else if !props.Blind && assigned {
						badge := wellGlyphAdaptive(props, well, role, cellW)
						if badge != "" {
							fontPx := labelSize * 0.72
							if cellW >= 40 {
								fontPx = labelSize * 0.88
							} else if len([]rune(badge)) == 1 {
								fontPx = labelSize * 0.84
							}
							glyphW := estimatedBadgeWidth(badge, fontPx)
							if maxW := cellW - 6; glyphW > maxW && maxW > 4 {
								fontPx *= maxW / glyphW
								if fontPx < 7.5 {
									fontPx = 7.5
								}
								glyphW = estimatedBadgeWidth(badge, fontPx)
							}
							tx := x + (cellW-glyphW)/2
							ty := y + (cellH-fontPx)*0.42
							if tx < x+2 {
								tx = x + 2
							}
							p.Text(tx, ty, badge, fontPx, ink)
						}
					} else if roleBadge := roleShort(role); roleBadge != "" {
						fontPx := labelSize * 0.78
						p.Text(x+(cellW-fontPx*0.6)/2, y+(cellH-fontPx)*0.44, roleBadge, fontPx, t.Text)
					}
					if selected[id] {
						p.Stroke(wr, t.Accent, radius, 2)
					}
				}
			}
			if !mini {
				hudY := gy0 + gridH + dishPad + float32(t.Spacing8)
				if lastHoverWell != "" {
					p.Text(r.X+leftLabel, hudY+float32(t.Spacing12), wellHoverSummary(props, lastHoverWell), labelSize, t.Text)
				} else if len(props.Wells) == 0 {
					p.Text(r.X+leftLabel, hudY+float32(t.Spacing12), "尚无计划孔位分配", labelSize, t.TextMuted)
				}
			}
		})
		if !mini {
			canvas.HitTest(func(x, y float32, r ui.Rect) bool {
				lastHoverWell = ""
				gx := x - (r.X + leftLabel + dishPad)
				gy := y - (r.Y + dilHeaderH + topLabel + dishPad)
				if gx < 0 || gy < 0 {
					return false
				}
				col := int(gx / (cellW + gap))
				row := int(gy / (cellH + gap))
				if col < 0 || col >= cols || row < 0 || row >= rows {
					return false
				}
				cx := gx - float32(col)*(cellW+gap)
				cy := gy - float32(row)*(cellH+gap)
				if cx > cellW || cy > cellH {
					return false
				}
				lastHoverWell = wellID(row, col)
				return true
			})
		}

		if len(legendOrder) > 0 {
			ui.Row(c).Wrap().Gap(float32(t.Spacing8)).AlignItems(ui.Center).Children(func() {
				for _, label := range legendOrder {
					ui.Row(c).Gap(float32(t.Spacing4)).AlignItems(ui.Center).Children(func() {
						ui.Box(c).Size(float32(t.Spacing8), float32(t.Spacing8)).Radius(float32(t.ControlRadius)).Background(legendColors[label])
						ui.Text(c, fmt.Sprintf("%s %d", label, legendCounts[label])).FontSize(labelSize).
							TextColor(t.TextMuted).FontFeatures("tnum").ContentTransition(theme.NumericRoll(false))
					})
				}
			})
		}
		if !mini && !props.HideFooter {
			ui.Text(c, fmt.Sprintf("%d / %d 孔已分配 · 悬停查看分组与样本明细", len(props.Wells), rows*cols)).
				FontSize(labelSize).TextColor(t.TextMuted).FontFeatures("tnum").
				ContentTransition(theme.NumericRoll(false))
		}
	})
}

func estimatedBadgeWidth(badge string, fontPx float32) float32 {
	var units float32
	for _, r := range badge {
		if r > 0x7f {
			units += 1.0
		} else {
			units += 0.56
		}
	}
	return units * fontPx
}

func wellHoverSummary(props Props, id string) string {
	well, assigned := props.Wells[id]
	role := effectiveRole(well, id, props.Roles)
	parts := []string{id}
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
			if gl := strings.TrimSpace(props.GroupLabels[well.Group]); gl != "" {
				parts = append(parts, gl)
			} else {
				parts = append(parts, well.Group)
			}
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
	return strings.Join(parts, " · ")
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
	scale := buildFactorColorScale(props, dark, t)
	switch strings.ToLower(strings.TrimSpace(props.ColorBy)) {
	case "sample":
		if strings.TrimSpace(well.SampleID) != "" {
			return theme.CategoricalColor("sample:"+well.SampleID, dark)
		}
		return t.Elevated
	case "factor":
		if props.FactorKey != "" && well.Factors != nil {
			if val := strings.TrimSpace(well.Factors[props.FactorKey]); val != "" {
				if scale != nil {
					if c, ok := scale.colors[val]; ok {
						return c
					}
				}
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
		return GroupColorIndexed(well.Group, props.GroupIndex, dark)
	}
}

func wellLegendCategory(props Props, well WellState, role string, dark bool, t theme.Tokens) (string, ui.Color) {
	return wellLegendCategoryWithScale(props, well, role, dark, t, buildFactorColorScale(props, dark, t))
}

func wellLegendCategoryWithScale(props Props, well WellState, role string, dark bool, t theme.Tokens, scale *factorColorScale) (string, ui.Color) {
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
				if scale != nil {
					if c, ok := scale.colors[v]; ok {
						return v, c
					}
				}
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
		display := well.Group
		if gl := strings.TrimSpace(props.GroupLabels[well.Group]); gl != "" {
			display = gl
		}
		return display, GroupColorIndexed(well.Group, props.GroupIndex, dark)
	}
}

func wellGlyph(props Props, well WellState, role string) string {
	return wellGlyphAdaptive(props, well, role, 26)
}

func wellGlyphAdaptive(props Props, well WellState, role string, cellW float32) string {
	if rs := roleShort(role); rs != "" {
		return rs
	}
	maxRunes := 3
	if cellW >= 48 {
		maxRunes = 7
	} else if cellW >= 38 {
		maxRunes = 6
	}
	switch strings.ToLower(strings.TrimSpace(props.ColorBy)) {
	case "replicate":
		if well.Replicate > 0 {
			return fmt.Sprintf("#%d", well.Replicate)
		}
	case "sample":
		if s := strings.TrimSpace(well.SampleID); s != "" {
			return compactTag(s, maxRunes)
		}
	case "factor":
		if props.FactorKey != "" && well.Factors != nil {
			if v := strings.TrimSpace(well.Factors[props.FactorKey]); v != "" {
				return compactTag(v, maxRunes)
			}
		}
	default:
		if cellW >= 38 {
			if s := strings.TrimSpace(well.SampleID); s != "" {
				return compactTag(s, maxRunes)
			}
			if well.Group != "" {
				lbl := well.Group
				if gl := strings.TrimSpace(props.GroupLabels[well.Group]); gl != "" {
					lbl = gl
				}
				return compactTag(strings.TrimPrefix(lbl, "g-"), maxRunes)
			}
		}
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
	// Preserve trailing identifier (e.g. "Donor-1" -> "Don-1" instead of "Donor-")
	if maxRunes >= 4 {
		last := rs[len(rs)-1]
		if unicode.IsDigit(last) {
			// Find contiguous trailing digits (and optional short separator like '-').
			tailStart := len(rs) - 1
			for tailStart > 0 && unicode.IsDigit(rs[tailStart-1]) {
				tailStart--
			}
			tail := rs[tailStart:]
			if len(tail) <= 2 && maxRunes-len(tail)-1 >= 2 {
				return string(rs[:maxRunes-len(tail)-1]) + "·" + string(tail)
			}
		}
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

// formatPlateSuperLabel prettifies scientific powers (10^-3 -> 10⁻³) and UCUM Cel -> °C
// inside plate legend/hover labels without importing asterdoc.
func formatPlateSuperLabel(s string) string {
	if s == "" {
		return ""
	}
	s = strings.ReplaceAll(s, " Cel", " °C")
	if !strings.Contains(s, "^") {
		return s
	}
	var out strings.Builder
	rs := []rune(s)
	for i := 0; i < len(rs); i++ {
		if rs[i] == '^' && i+1 < len(rs) {
			j := i + 1
			if rs[j] == '{' {
				end := j + 1
				for end < len(rs) && rs[end] != '}' {
					end++
				}
				if end < len(rs) {
					if sup, ok := plateToSup(string(rs[j+1 : end])); ok {
						out.WriteString(sup)
						i = end
						continue
					}
				}
			}
			end := j
			if end < len(rs) && (rs[end] == '-' || rs[end] == '+') {
				end++
			}
			for end < len(rs) && unicode.IsDigit(rs[end]) {
				end++
			}
			if end > j {
				if sup, ok := plateToSup(string(rs[j:end])); ok {
					out.WriteString(sup)
					i = end - 1
					continue
				}
			}
		}
		out.WriteRune(rs[i])
	}
	return out.String()
}

func plateToSup(s string) (string, bool) {
	if s == "" {
		return "", false
	}
	var b strings.Builder
	for _, r := range s {
		switch r {
		case '0':
			b.WriteRune('⁰')
		case '1':
			b.WriteRune('¹')
		case '2':
			b.WriteRune('²')
		case '3':
			b.WriteRune('³')
		case '4':
			b.WriteRune('⁴')
		case '5':
			b.WriteRune('⁵')
		case '6':
			b.WriteRune('⁶')
		case '7':
			b.WriteRune('⁷')
		case '8':
			b.WriteRune('⁸')
		case '9':
			b.WriteRune('⁹')
		case '-':
			b.WriteRune('⁻')
		case '+':
			b.WriteRune('⁺')
		default:
			return "", false
		}
	}
	return b.String(), true
}

// RenderCompactSummaryCard renders a mobile-friendly summary card with plate geometry,
// a proportional group color bar, well counts, and a full-width action button.
func RenderCompactSummaryCard(
	c *ui.Context,
	props Props,
	rows, cols int,
	legend []LegendItem,
	selected map[string]bool,
	onOpen func(),
) {
	t := theme.GetTokens(c.Theme())
	totalWells := rows * cols
	assignedWells := len(props.Wells)
	geomLabel := plateGeometryLabel(rows, cols, props.Topology)

	ui.Column(c).FillWidth().Padding(float32(t.Spacing12)).Radius(float32(t.PanelRadius)).
		Background(t.Elevated).Border(1, t.Border).Gap(float32(t.Spacing12)).Children(func() {
		ui.Row(c).FillWidth().Wrap().AlignItems(ui.Center).Gap(float32(t.Spacing8)).Children(func() {
			components.Badge(c, geomLabel, components.BadgeOptions{Tone: components.BadgeAccent})
			ui.Text(c, fmt.Sprintf("已排 %d / %d 孔", assignedWells, totalWells)).
				FontSize(float32(t.TextMeta)).FontFeatures("tnum").TextColor(t.TextMuted).
				ContentTransition(theme.NumericRoll(false))
			if len(selected) > 0 {
				ui.Row(c).Padding(float32(t.Spacing4)*0.5, float32(t.Spacing8)).
					Radius(float32(t.PillRadius)).
					Background(t.AccentSoftBg).
					Border(1, t.BorderSelected).
					AlignItems(ui.Center).Children(func() {
					ui.Text(c, fmt.Sprintf("已选 %d 孔", len(selected))).
						FontSize(float32(t.TextMeta)).Bold().FontFeatures("tnum").
						TextColor(t.Accent).ContentTransition(theme.NumericRoll(false)).SingleLine()
				})
			}
		})

		// Proportional group color bar
		propBar := ui.Row(c).FillWidth().Height(float32(t.Spacing12)).Radius(float32(t.ControlRadius)).
			Clip().Background(t.Surface).Border(1, t.Border).Gap(0)
		theme.ApplyRecessedWell(propBar, t)
		propBar.Children(func() {
			sumCounts := 0
			for _, item := range legend {
				if item.Count > 0 {
					sumCounts += item.Count
					ui.Box(c).FillHeight().Grow(float32(item.Count)).Basis(0).Background(item.Color)
				}
			}
			if rem := totalWells - sumCounts; rem > 0 {
				ui.Box(c).FillHeight().Grow(float32(rem)).Basis(0).Background(t.Canvas)
			}
		})

		if len(legend) > 0 {
			ui.Row(c).FillWidth().Wrap().AlignItems(ui.Center).Gap(float32(t.Spacing8)).Children(func() {
				for _, item := range legend {
					dispLbl := formatPlateSuperLabel(item.Label)
					ui.Row(c).Gap(float32(t.Spacing4)).AlignItems(ui.Center).Children(func() {
						ui.Box(c).Size(float32(t.Spacing8), float32(t.Spacing8)).
							Radius(float32(t.ControlRadius)).Background(item.Color)
						ui.Text(c, fmt.Sprintf("%s (%d)", dispLbl, item.Count)).
							FontSize(float32(t.TextMeta)).TextColor(t.TextMuted).FontFeatures("tnum").
							ContentTransition(theme.NumericRoll(false))
					})
				}
			})
		} else {
			ui.Text(c, "尚无孔位分组，打开全屏排板开始点选或涂抹标注").
				FontSize(float32(t.TextMeta)).TextColor(t.TextMuted)
		}

		openBtn := components.Button(c, "全屏排板 / 读数", components.ButtonPrimary).
			FillWidth().MinHeight(float32(t.Spacing48))
		if openBtn.Clicked() {
			mygo.Haptics.Selection()
			if onOpen != nil {
				onOpen()
			}
		}
	})
}

func renderCompactMobileCard(
	c *ui.Context,
	props Props,
	rows, cols int,
	legend []LegendItem,
	selected map[string]bool,
	st *InteractionState,
	t theme.Tokens,
) {
	_ = t
	RenderCompactSummaryCard(c, props, rows, cols, legend, selected, func() {
		if props.OnOpenFullscreen != nil {
			props.OnOpenFullscreen()
		} else if st != nil {
			st.ZoomState.Reset()
			st.MobileSheetOpen = true
		}
	})
}

// RenderTouchModeSelector renders the bottom 3-mode touch selection bar
// (点选 / 涂抹框选 / 按行·按列快选) and optional row/column axis pill.
func RenderTouchModeSelector(c *ui.Context, st *InteractionState, onClear func()) {
	if st == nil {
		return
	}
	t := theme.GetTokens(c.Theme())
	modes := []struct {
		mode  TouchMode
		label string
	}{
		{TouchModeTap, "点选"},
		{TouchModeBrush, "涂抹框选"},
		{TouchModeRowCol, "按行·按列快选"},
	}

	ui.Column(c).FillWidth().Gap(float32(t.Spacing8)).Children(func() {
		modeTrack := ui.Row(c).FillWidth().Padding(float32(t.Spacing4)).Radius(float32(t.PanelRadius)).
			Background(t.Elevated).Border(1, t.Border).Gap(float32(t.Spacing4))
		theme.ApplyRecessedWell(modeTrack, t)
		modeTrack.Children(func() {
			for _, m := range modes {
				m := m
				active := st.TouchMode == m.mode
				bg, fg, bd := ui.Transparent, t.TextMuted, ui.Transparent
				if active {
					bg, fg, bd = t.AccentSoftBg, t.Accent, t.Accent
				}
				btn := ui.ButtonBase(c.Key("touch-mode-"+m.label)).Label(m.label).
					Grow(1).Basis(0).MinWidth(0).MinHeight(float32(t.Spacing48)).
					Padding(float32(t.Spacing4), float32(t.Spacing8)).
					Radius(float32(t.ControlRadius)).Background(bg).Border(1, bd).
					Center().OnClick(func() {
					st.TouchMode = m.mode
					mygo.Haptics.Selection()
				})
				theme.ApplyTactilePress(btn)
				btn.Children(func() {
					txt := ui.Text(c, m.label).FontSize(float32(t.TextMeta)).TextColor(fg).SingleLine()
					if active {
						txt.Bold()
					}
				})
			}
		})

		if st.TouchMode == TouchModeRowCol {
			ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(float32(t.Spacing8)).Children(func() {
				ui.Text(c, "点击孔位快选方向：").FontSize(float32(t.TextMeta)).TextColor(t.TextMuted)
				for _, ax := range []struct {
					key, lbl string
				}{
					{"row", "整行"},
					{"col", "整列"},
				} {
					ax := ax
					isCur := (st.RowColAxis == "col") == (ax.key == "col")
					variant := components.ButtonGhost
					if isCur {
						variant = components.ButtonSecondary
					}
					if components.Button(c, ax.lbl, variant).Clicked() {
						st.RowColAxis = ax.key
						mygo.Haptics.Selection()
					}
				}
				ui.Spacer(c)
				if onClear != nil {
					if components.Button(c, "清空选区", components.ButtonGhost).Clicked() {
						onClear()
						mygo.Haptics.Light()
					}
				}
			})
		}
	})
}

func renderMobilePlateSheet(
	c *ui.Context,
	props Props,
	rows, cols int,
	legend []LegendItem,
	st *InteractionState,
	t theme.Tokens,
	dark bool,
) {
	if st == nil {
		return
	}
	mobile.Sheet(c, &st.MobileSheetOpen, mobile.SheetOptions{
		Detents: []float32{0.94},
		Label:   "全屏孔板排板",
	}, func() {
		vw, winH := c.Size()
		_, _, bottomSafe, _ := c.SafeArea()
		sheetBodyH := winH*0.94 - float32(t.Spacing24) - bottomSafe
		if sheetBodyH < float32(t.Spacing48)*6 {
			sheetBodyH = float32(t.Spacing48) * 6
		}
		fitZoom := ComputeFullscreenFitZoom(vw, rows, cols, props.Topology, t)
		maxZoom := float32(4)
		if rows*cols > 96 {
			maxZoom = 6
		}
		if st.ZoomState.Scale == 0 {
			st.ZoomState.Scale = 1
		}

		ui.Column(c).FillWidth().Height(sheetBodyH).Padding(float32(t.Spacing12)).Gap(float32(t.Spacing8)).Children(func() {
			// Header Row 1: Geometry title + well count + Done CTA (never clipped on narrow phones)
			ui.Row(c).FillWidth().Shrink(0).AlignItems(ui.Center).Gap(float32(t.Spacing8)).Children(func() {
				ui.Column(c).Grow(1).MinWidth(0).Gap(float32(t.Spacing4)*0.5).Children(func() {
					ui.Text(c, plateGeometryLabel(rows, cols, props.Topology)).
						FontSize(float32(t.TextSection)).Bold().TextColor(t.Text).SingleLine().Ellipsis("…")
					sub := fmt.Sprintf("已排 %d / %d 孔 · 已选 %d 孔", len(props.Wells), rows*cols, len(props.SelectedIDs))
					ui.Text(c, sub).FontSize(float32(t.TextMeta)).FontFeatures("tnum").TextColor(t.TextMuted).
						ContentTransition(theme.NumericRoll(false)).SingleLine().Ellipsis("…")
				})
				doneBtn := components.Button(c, "完成", components.ButtonPrimary).MinHeight(float32(t.Spacing32) + float32(t.Spacing8))
				if doneBtn.Clicked() {
					mygo.Haptics.Light()
					st.MobileSheetOpen = false
				}
			})
			// Header Row 2: Dedicated thumb-friendly Zoom Bar [- | 100% | + | 重置]
			zoomTrack := ui.Row(c).FillWidth().Shrink(0).AlignItems(ui.Center).Justify(ui.Center).Gap(float32(t.Spacing8)).
				Padding(float32(t.Spacing4), float32(t.Spacing8)).
				Radius(float32(t.ControlRadius)).Background(t.Elevated).Border(1, t.BorderSubtle)
			theme.ApplyRecessedWell(zoomTrack, t)
			zoomTrack.Children(func() {
				zoomBtnH := float32(t.Spacing32) + float32(t.Spacing4)
				zoomOutBtn := ui.ButtonBase(c.Key("plate-zoom-out")).Size(zoomBtnH, zoomBtnH).Radius(float32(t.ControlRadius)).Center().Label("缩小孔板")
				zoomOutBtn.Children(func() {
					ui.Icon(c, lucide.ZoomOut()).Size(float32(t.IconMd), float32(t.IconMd)).TextColor(t.Text)
				})
				if zoomOutBtn.Clicked() {
					st.ZoomState.Scale = max(1, st.ZoomState.Scale-0.5)
				}
				ui.Text(c, fmt.Sprintf("%.0f%%", st.ZoomState.Scale*100)).
					FontSize(float32(t.TextData)).Bold().FontFeatures("tnum").TextColor(t.Text).
					ContentTransition(theme.NumericRoll(false)).
					MinWidth(float32(t.Spacing48)).Center()
				zoomInBtn := ui.ButtonBase(c.Key("plate-zoom-in")).Size(zoomBtnH, zoomBtnH).Radius(float32(t.ControlRadius)).Center().Label("放大孔板")
				zoomInBtn.Children(func() {
					ui.Icon(c, lucide.ZoomIn()).Size(float32(t.IconMd), float32(t.IconMd)).TextColor(t.Text)
				})
				if zoomInBtn.Clicked() {
					st.ZoomState.Scale = min(maxZoom, st.ZoomState.Scale+0.5)
				}
				if components.Button(c, "重置", components.ButtonGhost).MinHeight(zoomBtnH).Clicked() {
					st.ZoomState.Reset()
				}
			})

			// Pinch-zoomable plate canvas
			zoomBox := ui.Box(c).FillWidth().Grow(1).Basis(0).MinHeight(0).
				Radius(float32(t.PanelRadius)).Background(t.Canvas).Border(1, t.Border).Clip()
			zoomBox.Children(func() {
				mobile.Zoomable(c, &st.ZoomState, mobile.ZoomOptions{
					Min:       1,
					Max:       maxZoom,
					DoubleTap: 2.2,
				}, func() {
					ui.Box(c).Fill().Center().Padding(float32(t.Spacing8)).Children(func() {
						sheetProps := props
						sheetProps.FullscreenSheet = true
						sheetProps.MobileCompact = false
						sheetProps.Zoom = fitZoom
						sheetProps.State = st
						sheetProps.TouchMode = &st.TouchMode
						View(c, sheetProps)
					})
				}).Fill()
			})

			// 3-mode touch selector
			RenderTouchModeSelector(c, st, func() {
				if props.OnSelect != nil {
					props.OnSelect(nil)
				}
			})

			// Thumb-friendly group assignment bar
			if props.Interactive {
				grpScroll := ui.ScrollHorizontal(c.Key("pl-sheet-grp-scroll")).FillWidth()
				grpScrollSt := ui.Local(grpScroll, "scroll-st", func() ui.ScrollState { return ui.ScrollState{} })
				grpScroll.TrackScroll(grpScrollSt)
				theme.ApplyEdgeFadeX(grpScroll, t, grpScrollSt.X > 2, grpScrollSt.MaxX > 2 && grpScrollSt.X < grpScrollSt.MaxX-2)
				grpScroll.Children(func() {
					ui.Row(c).AlignItems(ui.Center).Gap(float32(t.Spacing8)).Children(func() {
						seenGrp := map[string]bool{}
						var groupIDs []string
						for gid := range props.GroupLabels {
							if gid != "" && !seenGrp[gid] {
								seenGrp[gid] = true
								groupIDs = append(groupIDs, gid)
							}
						}
						for _, w := range props.Wells {
							if w.Group != "" && !seenGrp[w.Group] {
								seenGrp[w.Group] = true
								groupIDs = append(groupIDs, w.Group)
							}
						}
						sort.Strings(groupIDs)
						for _, gid := range groupIDs {
							gid := gid
							lbl := gid
							if gl := strings.TrimSpace(props.GroupLabels[gid]); gl != "" {
								lbl = gl
							}
							col := GroupColorIndexed(gid, props.GroupIndex, dark)
							chip := ui.ButtonBase(c.Key("sheet-grp-"+gid)).Label("归入分组 "+lbl).
								MinHeight(float32(t.Spacing48)).Padding(float32(t.Spacing8), float32(t.Spacing12)).
								Radius(float32(t.ControlRadius)).Background(t.Surface).Border(1, t.Border).
								OnClick(func() {
									if len(props.SelectedIDs) > 0 && props.OnAssignGroup != nil {
										mygo.Haptics.Success()
										props.OnAssignGroup(gid, append([]string(nil), props.SelectedIDs...))
									}
								})
							chip.Children(func() {
								ui.Row(c).AlignItems(ui.Center).Gap(float32(t.Spacing8)).Children(func() {
									ui.Box(c).Size(float32(t.Spacing12), float32(t.Spacing12)).
										Radius(float32(t.ControlRadius)).Background(col)
									ui.Text(c, lbl).FontSize(float32(t.TextMeta)).Bold().TextColor(t.Text).SingleLine()
								})
							})
						}
						if len(props.SelectedIDs) > 0 {
							if props.OnClearWells != nil {
								clrBtn := components.Button(c, "清空所选", components.ButtonSecondary).MinHeight(float32(t.Spacing48))
								if clrBtn.Clicked() {
									mygo.Haptics.Light()
									props.OnClearWells(append([]string(nil), props.SelectedIDs...))
								}
							}
							deselBtn := components.Button(c, "取消选区", components.ButtonGhost).MinHeight(float32(t.Spacing48))
							if deselBtn.Clicked() && props.OnSelect != nil {
								props.OnSelect(nil)
							}
						}
					})
				})
			}
		})
	})
}
