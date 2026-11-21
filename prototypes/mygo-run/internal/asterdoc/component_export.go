package asterdoc

import (
	"bytes"
	"fmt"
	"html"
	"image"
	"image/color"
	"image/png"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/onevra/source/components"

	"asterlab-mygo-prototype/internal/theme"
	plateui "asterlab-mygo-prototype/internal/uikit/plate"
)

// buildPlateUIProps constructs the exact plateui.Props used by on-screen plateui.View
// so exported SVG and 2x Retina PNG images match AsterLab's live plate rendering 1:1.
func (v *View) buildPlateUIProps(p *Plate, colorByOverride, factorKeyOverride string) (plateui.Props, string) {
	if p == nil {
		return plateui.Props{}, ""
	}
	geometry, _ := geometryForPlate(p.Format, p.Topology, p.Rows, p.Columns)
	excludedSet := map[string]bool{}
	for _, ew := range p.ExcludedWells {
		excludedSet[strings.ToUpper(strings.TrimSpace(ew))] = true
	}

	wells := map[string]plateui.WellState{}
	for well, group := range p.WellGroups() {
		fMap := copyStringMap(p.WellFactors(well))
		for lName, lVals := range p.Layers {
			if lv := strings.TrimSpace(lVals[well]); lv != "" {
				fMap[lName] = lv
			}
		}
		roleVal := p.Roles[well]
		wells[well] = plateui.WellState{
			Group:     group,
			SampleID:  p.Samples[well],
			Replicate: p.Replicates[well],
			Role:      roleVal,
			Factors:   fMap,
		}
	}
	for lName, lVals := range p.Layers {
		for well, lv := range lVals {
			if strings.TrimSpace(lv) == "" {
				continue
			}
			ws := wells[well]
			if ws.Factors == nil {
				ws.Factors = map[string]string{}
			}
			ws.Factors[lName] = lv
			wells[well] = ws
		}
	}

	colorBy := colorByOverride
	if colorBy == "" && v != nil && v.plateColorBy != nil {
		colorBy = v.plateColorBy[p.ID]
	}
	if colorBy == "" {
		colorBy = "group"
	}
	factorKey := factorKeyOverride
	if factorKey == "" && v != nil && v.plateFactorKey != nil {
		factorKey = v.plateFactorKey[p.ID]
	}
	if factorKey == "" {
		if dims := p.FactorDimensions(); len(dims) > 0 {
			factorKey = dims[0]
		} else {
			for k := range p.Layers {
				factorKey = k
				break
			}
		}
	}

	var doc *Document
	if v != nil {
		doc = v.Doc
	}
	idxMap := plateGroupPaletteMap(doc, p)
	lblMap := PlateGroupLabelMap(doc)

	title := strings.TrimSpace(p.Title)
	if title == "" {
		switch p.Topology {
		case "dish":
			title = fmt.Sprintf("培养皿分区（%d 区）", geometry.Rows*geometry.Columns)
		case "rack_1d":
			title = fmt.Sprintf("%d 位样本架 / 泳道", geometry.Rows*geometry.Columns)
		default:
			title = fmt.Sprintf("%d 孔板", p.Format)
		}
	}

	return plateui.Props{
		Wells:       wells,
		Rows:        geometry.Rows,
		Cols:        geometry.Columns,
		Topology:    p.Topology,
		Roles:       p.Roles,
		ColorBy:     colorBy,
		FactorKey:   factorKey,
		GroupIndex:  idxMap,
		GroupLabels: lblMap,
		ExcludedIDs: excludedSet,
	}, title
}

// RenderPlateSVG renders a standalone vector SVG matching plateui.View 1:1.
func RenderPlateSVG(p *Plate, g Geometry, doc *Document, colorBy, factorKey string, dark bool) string {
	if p == nil || g.Rows <= 0 || g.Columns <= 0 {
		return ""
	}
	v := &View{Doc: doc}
	props, title := v.buildPlateUIProps(p, colorBy, factorKey)
	props.Rows, props.Cols = g.Rows, g.Columns
	return plateui.RenderSVG(props, title, dark)
}

// RenderModulePNG renders ANY AsterML component (plate, group_table, param_table,
// data_table, checklist, calculator, timer, figure, subflow, etc.) into a 2x Retina
// PNG image using MyGo's offscreen rasterizer so it matches AsterLab's UI pixel-for-pixel.
func (v *View) RenderModulePNG(kind string, schema int, data map[string]any, dark bool) ([]byte, error) {
	if data == nil {
		return nil, fmt.Errorf("empty component data")
	}
	if schema <= 0 {
		schema = 1
	}
	moduleID := textValue(data, "id")
	if kind == "plate" {
		if pl, _ := v.findPlate(moduleID); pl != nil {
			props, title := v.buildPlateUIProps(pl, "", "")
			return plateui.RenderPNG(props, title, dark)
		}
	}

	var cv View
	if v != nil {
		cv = *v
	}
	cv.ReadOnly = true
	cv.UnifiedLive = false
	cv.AsyncDecode = false
	cv.isExportRender = true
	cv.ExpandedPlate = ""
	cv.ExpandedFigureID = ""
	cv.ExpandedAsset = nil
	if kind == "param_table" && v != nil && (v.Mode == ModeRun || len(v.ActualParams) > 0) {
		cv.Mode = ModeRun
	} else if cv.Mode == "" || cv.Mode == ModeDesign {
		cv.Mode = ModeDesign
	}

	cardW := 700
	estRows := len(mapSlice(data["rows"])) + len(mapSlice(data["items"])) + len(mapSlice(data["components"])) + len(mapSlice(data["phases"]))
	maxH := 340 + estRows*46
	if kind == "figure" || kind == "attachment" {
		maxH = 620
	}
	if maxH < 360 {
		maxH = 360
	}
	if maxH > 2000 {
		maxH = 2000
	}

	var bgCol ui.Color
	tt := ui.NewTesterWith(func(c *ui.Context) {
		if dark {
			c.SetTheme(theme.DarkTheme())
		} else {
			c.SetTheme(theme.LightTheme())
		}
		tok := theme.GetTokens(c.Theme())
		bgCol = tok.Canvas
		ui.Column(c).Fill().Background(tok.Canvas).Padding(float32(tok.Spacing12)).Children(func() {
			cv.RenderModule(c, kind, schema, data)
		})
	}, cardW, maxH, ui.TesterOptions{
		Dark:        dark,
		Preferences: ui.Preferences{ReduceMotion: true},
	})
	tt.SetScale(2.0)
	tt.Frame()

	img := tt.Image()
	if img == nil {
		return nil, fmt.Errorf("failed to rasterize component %s", kind)
	}
	cropped := cropBottomCanvasRows(img, bgCol, 24)
	var buf bytes.Buffer
	if err := png.Encode(&buf, cropped); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func cropBottomCanvasRows(img *image.RGBA, canvasCol ui.Color, bottomPadPx int) image.Image {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= 0 || h <= 0 {
		return img
	}
	bg := color.RGBA{R: canvasCol.R, G: canvasCol.G, B: canvasCol.B, A: 255}
	lastContentY := -1
	for y := b.Max.Y - 1; y >= b.Min.Y; y-- {
		rowDiff := false
		for x := b.Min.X; x < b.Max.X; x += 2 {
			p := img.RGBAAt(x, y)
			if absDiffU8(p.R, bg.R) > 3 || absDiffU8(p.G, bg.G) > 3 || absDiffU8(p.B, bg.B) > 3 {
				rowDiff = true
				break
			}
		}
		if rowDiff {
			lastContentY = y
			break
		}
	}
	if lastContentY < 0 {
		return img
	}
	cropMaxY := lastContentY + 1 + bottomPadPx
	if cropMaxY >= b.Max.Y {
		return img
	}
	if cropMaxY-b.Min.Y < 80 {
		cropMaxY = b.Min.Y + min(h, 80)
	}
	return img.SubImage(image.Rect(b.Min.X, b.Min.Y, b.Max.X, cropMaxY))
}

func absDiffU8(a, b uint8) int {
	if a > b {
		return int(a - b)
	}
	return int(b - a)
}

// RenderModuleSVG renders any structured AsterML block into a publication-ready
// vector SVG card visually aligned with AsterLab's Surface/Tokens design system.
func (v *View) RenderModuleSVG(kind string, data map[string]any, dark bool) string {
	if data == nil {
		return ""
	}
	moduleID := textValue(data, "id")
	if kind == "plate" {
		if pl, _ := v.findPlate(moduleID); pl != nil {
			props, title := v.buildPlateUIProps(pl, "", "")
			return plateui.RenderSVG(props, title, dark)
		}
	}
	if kind == "figure" {
		if rawSVG := strings.TrimSpace(textValue(data, "svg")); strings.Contains(rawSVG, "<svg") {
			return rawSVG
		}
	}

	tok := theme.GetTokens(theme.Palette(dark))
	switch kind {
	case "group_table":
		return v.renderGroupTableSVG(data, tok, dark)
	case "param_table":
		return v.renderParamTableSVG(data, tok)
	case "data_table":
		return v.renderDataTableSVG(data, tok)
	case "checklist":
		return v.renderChecklistSVG(data, tok)
	case "calculator":
		return v.renderCalculatorSVG(data, tok)
	case "timer":
		return v.renderTimerSVG(data, tok)
	default:
		md := v.FormatModuleAsMarkdown(kind, data)
		return renderVectorTextCardSVG(firstNonEmpty(textValue(data, "title"), kind), kind, md, tok)
	}
}

func writeSVGCardShell(b *strings.Builder, w, h int, title, badge string, tok theme.Tokens) {
	fmt.Fprintf(b, `<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 %d %d">`+"\n", w, h, w, h)
	fmt.Fprintf(b, `  <rect x="0" y="0" width="%d" height="%d" fill="%s"/>`+"\n", w, h, svgHex(tok.Canvas))
	fmt.Fprintf(b, `  <rect x="6" y="6" width="%d" height="%d" rx="12" fill="%s" stroke="%s" stroke-width="1"/>`+"\n",
		w-12, h-12, svgHex(tok.Surface), svgHex(tok.Border))
	fmt.Fprintf(b, `  <text x="24" y="34" font-family="Microsoft YaHei, PingFang SC, Inter, sans-serif" font-size="15" font-weight="700" fill="%s">%s</text>`+"\n",
		svgHex(tok.Text), html.EscapeString(title))
	if badge != "" {
		bw := max(52, len([]rune(badge))*9+20)
		bx := w - 24 - bw
		fmt.Fprintf(b, `  <rect x="%d" y="18" width="%d" height="22" rx="11" fill="%s" stroke="%s" stroke-width="1"/>`+"\n",
			bx, bw, svgHex(tok.Surface), svgHex(tok.Border))
		fmt.Fprintf(b, `  <text x="%d" y="33" text-anchor="middle" font-family="Microsoft YaHei, PingFang SC, Inter, sans-serif" font-size="11" font-weight="600" fill="%s">%s</text>`+"\n",
			bx+bw/2, svgHex(tok.Text), html.EscapeString(badge))
	}
}

func (v *View) renderGroupTableSVG(data map[string]any, tok theme.Tokens, dark bool) string {
	title := firstNonEmpty(textValue(data, "title"), "实验分组表")
	rows := mapSlice(data["rows"])
	var doc *Document
	if v != nil {
		doc = v.Doc
	}
	idxMap := plateGroupPaletteMap(doc, nil)
	for i, rm := range rows {
		if gid := textValue(rm, "id"); gid != "" {
			if _, ok := idxMap[gid]; !ok {
				idxMap[gid] = i
			}
		}
	}

	w := 640
	hdrY := 52
	rowH := 36
	h := hdrY + 30 + len(rows)*rowH + 22
	var b strings.Builder
	writeSVGCardShell(&b, w, h, title, fmt.Sprintf("%d 组", len(rows)), tok)

	// Table header bar (clean hairline matching components.Table)
	fmt.Fprintf(&b, `  <line x1="22" y1="%d" x2="%d" y2="%d" stroke="%s" stroke-width="1"/>`+"\n", hdrY+28, w-22, hdrY+28, svgHex(tok.Border))
	fmt.Fprintf(&b, `  <text x="32" y="%d" font-family="Microsoft YaHei, PingFang SC, Inter, sans-serif" font-size="12" font-weight="600" fill="%s">组别名称</text>`+"\n", hdrY+18, svgHex(tok.TextMuted))
	fmt.Fprintf(&b, `  <text x="220" y="%d" font-family="Microsoft YaHei, PingFang SC, Inter, sans-serif" font-size="12" font-weight="600" fill="%s">角色</text>`+"\n", hdrY+18, svgHex(tok.TextMuted))
	fmt.Fprintf(&b, `  <text x="318" y="%d" font-family="Microsoft YaHei, PingFang SC, Inter, sans-serif" font-size="12" font-weight="600" fill="%s">处理条件</text>`+"\n", hdrY+18, svgHex(tok.TextMuted))
	fmt.Fprintf(&b, `  <text x="%d" y="%d" text-anchor="end" font-family="Microsoft YaHei, PingFang SC, Inter, sans-serif" font-size="12" font-weight="600" fill="%s">样本量</text>`+"\n", w-32, hdrY+18, svgHex(tok.TextMuted))

	for idx, rm := range rows {
		y := hdrY + 30 + idx*rowH
		if idx%2 == 1 {
			fmt.Fprintf(&b, `  <rect x="20" y="%d" width="%d" height="%d" rx="6" fill="%s" fill-opacity="0.55"/>`+"\n", y, w-40, rowH, svgHex(tok.Elevated))
		}
		gid := textValue(rm, "id")
		gname := firstNonEmpty(textValue(rm, "name"), gid)
		role := textValue(rm, "role")
		roleLbl := firstNonEmpty(groupRoleLabel(role), role, "—")
		cond := firstNonEmpty(textValue(rm, "condition"), "—")
		indN, _ := numericValue(rm["independent_n"])
		techR, _ := numericValue(rm["technical_repeats"])
		if techR == 0 {
			techR, _ = numericValue(rm["replicate_count"])
		}
		nStr := "—"
		if indN > 0 && techR > 0 {
			nStr = fmt.Sprintf("n=%d × %d", int(indN), int(techR))
		} else if indN > 0 {
			nStr = fmt.Sprintf("n=%d", int(indN))
		} else if techR > 0 {
			nStr = fmt.Sprintf("rep=%d", int(techR))
		}
		swatch := plateui.GroupColorIndexed(gid, idxMap, dark)
		fmt.Fprintf(&b, `  <circle cx="36" cy="%d" r="4.5" fill="%s"/>`+"\n", y+18, svgHex(swatch))
		fmt.Fprintf(&b, `  <text x="49" y="%d" font-family="Microsoft YaHei, PingFang SC, Inter, sans-serif" font-size="13" font-weight="700" fill="%s">%s</text>`+"\n",
			y+22, svgHex(tok.Text), html.EscapeString(abbreviateCell(gname, 14)))

		// Capsule role pill matching groupRoleBadge
		pillBg, pillBorder, pillFg := svgHex(tok.Surface), svgHex(tok.Border), svgHex(tok.Text)
		if strings.EqualFold(strings.TrimSpace(role), "treatment") {
			pillBg, pillBorder, pillFg = svgHex(tok.AccentSoftBg), svgHex(tok.Accent), svgHex(tok.Accent)
		}
		pw := max(54, len([]rune(roleLbl))*11+18)
		fmt.Fprintf(&b, `  <rect x="220" y="%d" width="%d" height="22" rx="11" fill="%s" stroke="%s" stroke-width="0.8"/>`+"\n", y+7, pw, pillBg, pillBorder)
		fmt.Fprintf(&b, `  <text x="%d" y="%d" text-anchor="middle" font-family="Microsoft YaHei, PingFang SC, Inter, sans-serif" font-size="11" font-weight="500" fill="%s">%s</text>`+"\n",
			220+pw/2, y+22, pillFg, html.EscapeString(roleLbl))

		fmt.Fprintf(&b, `  <text x="318" y="%d" font-family="Microsoft YaHei, PingFang SC, Inter, sans-serif" font-size="12.5" fill="%s">%s</text>`+"\n",
			y+22, svgHex(tok.TextMuted), html.EscapeString(abbreviateCell(cond, 22)))
		fmt.Fprintf(&b, `  <text x="%d" y="%d" text-anchor="end" font-family="Inter, ui-monospace, sans-serif" font-size="12.5" font-weight="600" fill="%s">%s</text>`+"\n",
			w-32, y+22, svgHex(tok.Text), html.EscapeString(nStr))
	}
	b.WriteString("</svg>\n")
	return b.String()
}

func (v *View) renderParamTableSVG(data map[string]any, tok theme.Tokens) string {
	moduleID := textValue(data, "id")
	title := firstNonEmpty(textValue(data, "title"), "实验条件")
	rows := mapSlice(data["rows"])

	hasAnyActual := false
	for idx, rm := range rows {
		key := firstNonEmpty(textValue(rm, "key"), fmt.Sprintf("p-%d", idx))
		if v.lookupActualParam(moduleID, key, rm) != "" {
			hasAnyActual = true
			break
		}
	}

	w := 620
	hdrY := 52
	rowH := 38
	h := hdrY + 28 + len(rows)*rowH + 22
	var b strings.Builder
	writeSVGCardShell(&b, w, h, title, fmt.Sprintf("%d 项参数", len(rows)), tok)

	fmt.Fprintf(&b, `  <rect x="20" y="%d" width="%d" height="28" rx="6" fill="%s"/>`+"\n", hdrY, w-40, svgHex(tok.Elevated))
	fmt.Fprintf(&b, `  <text x="32" y="%d" font-family="Microsoft YaHei, PingFang SC, Inter, sans-serif" font-size="11.5" font-weight="600" fill="%s">参数</text>`+"\n", hdrY+18, svgHex(tok.TextMuted))
	if hasAnyActual {
		fmt.Fprintf(&b, `  <text x="390" y="%d" text-anchor="end" font-family="Microsoft YaHei, PingFang SC, Inter, sans-serif" font-size="11.5" font-weight="600" fill="%s">计划值</text>`+"\n", hdrY+18, svgHex(tok.TextMuted))
		fmt.Fprintf(&b, `  <text x="%d" y="%d" text-anchor="end" font-family="Microsoft YaHei, PingFang SC, Inter, sans-serif" font-size="11.5" font-weight="600" fill="%s">实测记录</text>`+"\n", w-32, hdrY+18, svgHex(tok.TextMuted))
	} else {
		fmt.Fprintf(&b, `  <text x="%d" y="%d" text-anchor="end" font-family="Microsoft YaHei, PingFang SC, Inter, sans-serif" font-size="11.5" font-weight="600" fill="%s">计划值</text>`+"\n", w-32, hdrY+18, svgHex(tok.TextMuted))
	}

	for idx, rm := range rows {
		y := hdrY + 28 + idx*rowH
		if idx > 0 {
			fmt.Fprintf(&b, `  <line x1="22" y1="%d" x2="%d" y2="%d" stroke="%s" stroke-width="1"/>`+"\n", y, w-22, y, svgHex(tok.Border))
		}
		key := firstNonEmpty(textValue(rm, "key"), fmt.Sprintf("p-%d", idx))
		label := firstNonEmpty(textValue(rm, "label"), textValue(rm, "name"), key)
		note := textValue(rm, "note")
		unit := DisplayUnit(textValue(rm, "unit"))
		planned := formatAnyValue(rm["value"])
		if planned == "" {
			planned = firstNonEmpty(textValue(rm, "target"), "—")
		}
		if unit != "" && planned != "—" {
			planned += " " + unit
		}
		if note != "" {
			fmt.Fprintf(&b, `  <text x="32" y="%d" font-family="Microsoft YaHei, PingFang SC, Inter, sans-serif" font-size="12.5" font-weight="600" fill="%s">%s</text>`+"\n",
				y+16, svgHex(tok.Text), html.EscapeString(abbreviateCell(label, 22)))
			fmt.Fprintf(&b, `  <text x="32" y="%d" font-family="Microsoft YaHei, PingFang SC, Inter, sans-serif" font-size="10.5" fill="%s">%s</text>`+"\n",
				y+31, svgHex(tok.TextMuted), html.EscapeString(abbreviateCell(note, 28)))
		} else {
			fmt.Fprintf(&b, `  <text x="32" y="%d" font-family="Microsoft YaHei, PingFang SC, Inter, sans-serif" font-size="13" font-weight="600" fill="%s">%s</text>`+"\n",
				y+23, svgHex(tok.Text), html.EscapeString(abbreviateCell(label, 24)))
		}

		if hasAnyActual {
			fmt.Fprintf(&b, `  <text x="390" y="%d" text-anchor="end" font-family="Inter, ui-monospace, sans-serif" font-size="12.5" font-weight="600" fill="%s">%s</text>`+"\n",
				y+23, svgHex(tok.Text), html.EscapeString(planned))
			rec := v.lookupActualParam(moduleID, key, rm)
			if rec != "" {
				if unit != "" && !strings.HasSuffix(rec, unit) {
					rec += " " + unit
				}
				lbl := "实测 " + rec
				pw := max(72, len([]rune(lbl))*9+18)
				fmt.Fprintf(&b, `  <rect x="%d" y="%d" width="%d" height="22" rx="5" fill="%s" stroke="%s" stroke-width="1"/>`+"\n",
					w-32-pw, y+8, pw, svgHex(tok.Elevated), svgHex(tok.Success))
				fmt.Fprintf(&b, `  <text x="%d" y="%d" text-anchor="middle" font-family="Microsoft YaHei, PingFang SC, Inter, sans-serif" font-size="11" font-weight="600" fill="%s">%s</text>`+"\n",
					w-32-pw/2, y+23, svgHex(tok.Success), html.EscapeString(lbl))
			} else {
				fmt.Fprintf(&b, `  <text x="%d" y="%d" text-anchor="end" font-family="Inter, sans-serif" font-size="12" fill="%s">—</text>`+"\n",
					w-32, y+23, svgHex(tok.TextMuted))
			}
		} else {
			fmt.Fprintf(&b, `  <text x="%d" y="%d" text-anchor="end" font-family="Inter, ui-monospace, sans-serif" font-size="13" font-weight="600" fill="%s">%s</text>`+"\n",
				w-32, y+23, svgHex(tok.Text), html.EscapeString(planned))
		}
	}
	b.WriteString("</svg>\n")
	return b.String()
}

func (v *View) renderDataTableSVG(data map[string]any, tok theme.Tokens) string {
	title, headers, rows := v.extractTabularModule("data_table", data)
	if len(headers) == 0 {
		return renderVectorTextCardSVG(title, "data_table", "暂无数据列", tok)
	}
	return renderVectorTableSVG(title, fmt.Sprintf("%d 行", len(rows)), headers, rows, tok)
}

func (v *View) renderChecklistSVG(data map[string]any, tok theme.Tokens) string {
	title := firstNonEmpty(textValue(data, "title"), "操作检查清单")
	items := mapSlice(data["items"])
	doneCnt := 0
	for idx, it := range items {
		cid := firstNonEmpty(textValue(it, "id"), fmt.Sprintf("chk-%d", idx+1))
		if v.isChecklistDone(cid, it) {
			doneCnt++
		}
	}

	w := 600
	startY := 54
	rowH := 34
	h := startY + len(items)*rowH + 20
	var b strings.Builder
	writeSVGCardShell(&b, w, h, title, fmt.Sprintf("%d / %d 已完成", doneCnt, len(items)), tok)

	for idx, it := range items {
		y := startY + idx*rowH
		cid := firstNonEmpty(textValue(it, "id"), fmt.Sprintf("chk-%d", idx+1))
		lbl := firstNonEmpty(textValue(it, "label"), textValue(it, "text"), cid)
		done := v.isChecklistDone(cid, it)
		req := asBool(it["required"]) || asBool(it["blocking"])

		bx, by := 26, y+8
		if done {
			fmt.Fprintf(&b, `  <rect x="%d" y="%d" width="16" height="16" rx="4" fill="%s"/>`+"\n", bx, by, svgHex(tok.Accent))
			fmt.Fprintf(&b, `  <path d="M%d %d L%d %d L%d %d" fill="none" stroke="#ffffff" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/>`+"\n",
				bx+3, by+8, bx+7, by+12, bx+13, by+4)
		} else {
			fmt.Fprintf(&b, `  <rect x="%d" y="%d" width="16" height="16" rx="4" fill="%s" stroke="%s" stroke-width="1.4"/>`+"\n",
				bx, by, svgHex(tok.Surface), svgHex(tok.BorderStrong))
		}
		txtCol := svgHex(tok.Text)
		if done {
			txtCol = svgHex(tok.TextMuted)
		}
		fmt.Fprintf(&b, `  <text x="52" y="%d" font-family="Microsoft YaHei, PingFang SC, Inter, sans-serif" font-size="13" fill="%s">%s</text>`+"\n",
			y+21, txtCol, html.EscapeString(abbreviateCell(lbl, 38)))
		if req && !done {
			fmt.Fprintf(&b, `  <rect x="%d" y="%d" width="40" height="18" rx="4" fill="%s"/>`+"\n", w-64, y+7, svgHex(tok.Elevated))
			fmt.Fprintf(&b, `  <text x="%d" y="%d" text-anchor="middle" font-family="Microsoft YaHei, PingFang SC, Inter, sans-serif" font-size="10.5" font-weight="600" fill="%s">必检</text>`+"\n",
				w-44, y+20, svgHex(tok.Danger))
		}
	}
	b.WriteString("</svg>\n")
	return b.String()
}

func (v *View) renderCalculatorSVG(data map[string]any, tok theme.Tokens) string {
	title := firstNonEmpty(textValue(data, "title"), "科学计算器")
	mode := textValue(data, "mode")
	var doc *Document
	var actuals *CalcActualContext
	if v != nil {
		doc = v.Doc
		actuals = v.CalcActuals
	}
	res := EvalCalculatorWithActuals(doc, data, actuals)

	w := 640
	curY := 54
	summaryH := 0
	if res.Summary != "" {
		summaryH = 38
	}
	rowH := 34
	tblH := 0
	if len(res.Rows) > 0 {
		tblH = 28 + len(res.Rows)*rowH
	}
	h := curY + summaryH + tblH + 26
	if h < 120 {
		h = 120
	}

	var b strings.Builder
	writeSVGCardShell(&b, w, h, title, calcModeLabel(mode), tok)

	if res.Summary != "" {
		fmt.Fprintf(&b, `  <rect x="20" y="%d" width="%d" height="30" rx="6" fill="%s"/>`+"\n", curY, w-40, svgHex(tok.AccentSoftBg))
		fmt.Fprintf(&b, `  <text x="32" y="%d" font-family="Microsoft YaHei, PingFang SC, Inter, sans-serif" font-size="12.5" font-weight="700" fill="%s">%s</text>`+"\n",
			curY+20, svgHex(tok.Accent), html.EscapeString(abbreviateCell(res.Summary, 56)))
		curY += 38
	}
	if len(res.Rows) > 0 {
		fmt.Fprintf(&b, `  <rect x="20" y="%d" width="%d" height="28" rx="6" fill="%s"/>`+"\n", curY, w-40, svgHex(tok.Elevated))
		fmt.Fprintf(&b, `  <text x="32" y="%d" font-family="Microsoft YaHei, PingFang SC, Inter, sans-serif" font-size="11.5" font-weight="600" fill="%s">分项</text>`+"\n", curY+18, svgHex(tok.TextMuted))
		fmt.Fprintf(&b, `  <text x="330" y="%d" text-anchor="end" font-family="Microsoft YaHei, PingFang SC, Inter, sans-serif" font-size="11.5" font-weight="600" fill="%s">计算值</text>`+"\n", curY+18, svgHex(tok.TextMuted))
		fmt.Fprintf(&b, `  <text x="352" y="%d" font-family="Microsoft YaHei, PingFang SC, Inter, sans-serif" font-size="11.5" font-weight="600" fill="%s">计算说明</text>`+"\n", curY+18, svgHex(tok.TextMuted))
		curY += 28
		for idx, row := range res.Rows {
			y := curY + idx*rowH
			if idx > 0 {
				fmt.Fprintf(&b, `  <line x1="22" y1="%d" x2="%d" y2="%d" stroke="%s" stroke-width="1"/>`+"\n", y, w-22, y, svgHex(tok.Border))
			}
			amtStr := formatNumber(row.Amount)
			if row.Unit != "" {
				amtStr += " " + DisplayUnit(row.Unit)
			}
			fmt.Fprintf(&b, `  <text x="32" y="%d" font-family="Microsoft YaHei, PingFang SC, Inter, sans-serif" font-size="13" font-weight="600" fill="%s">%s</text>`+"\n",
				y+22, svgHex(tok.Text), html.EscapeString(abbreviateCell(row.Label, 20)))
			fmt.Fprintf(&b, `  <text x="330" y="%d" text-anchor="end" font-family="Inter, ui-monospace, sans-serif" font-size="13" font-weight="700" fill="%s">%s</text>`+"\n",
				y+22, svgHex(tok.Text), html.EscapeString(amtStr))
			fmt.Fprintf(&b, `  <text x="352" y="%d" font-family="Microsoft YaHei, PingFang SC, Inter, sans-serif" font-size="12" fill="%s">%s</text>`+"\n",
				y+22, svgHex(tok.TextMuted), html.EscapeString(abbreviateCell(firstNonEmpty(row.Note, "—"), 24)))
		}
	}
	b.WriteString("</svg>\n")
	return b.String()
}

func (v *View) renderTimerSVG(data map[string]any, tok theme.Tokens) string {
	moduleID := textValue(data, "id")
	title := firstNonEmpty(textValue(data, "title"), "实验计时与周期")
	mode := textValue(data, "mode")
	plannedSec, spanSec, _ := timerBudgetSec(data)
	displaySec := plannedSec
	if displaySec == 0 {
		displaySec = spanSec
	}
	statusText := "待启动"
	if v != nil && v.TimerStates != nil {
		if rt, ok := v.TimerStates[moduleID]; ok {
			if rt.RemainingS > 0 {
				displaySec = rt.RemainingS
			} else if rt.AccumulatedS > 0 {
				displaySec = rt.AccumulatedS
			}
			if rt.Status != "" {
				statusText = timerStatusLabel(rt.Status)
			}
		}
	}
	w, h := 520, 114
	var b strings.Builder
	writeSVGCardShell(&b, w, h, title, timerModeLabel(mode), tok)
	fmt.Fprintf(&b, `  <text x="24" y="84" font-family="Inter, ui-monospace, monospace" font-size="26" font-weight="700" fill="%s">%s</text>`+"\n",
		svgHex(tok.Text), html.EscapeString(formatClockHMS(displaySec)))
	fmt.Fprintf(&b, `  <rect x="164" y="64" width="68" height="24" rx="6" fill="%s" stroke="%s" stroke-width="1"/>`+"\n",
		svgHex(tok.Elevated), svgHex(tok.Border))
	fmt.Fprintf(&b, `  <text x="198" y="80" text-anchor="middle" font-family="Microsoft YaHei, PingFang SC, Inter, sans-serif" font-size="11.5" font-weight="600" fill="%s">%s</text>`+"\n",
		svgHex(tok.TextMuted), html.EscapeString(statusText))
	b.WriteString("</svg>\n")
	return b.String()
}

// FormatModuleAsMarkdown exports a plate or any structured block as clean GFM Markdown.
func (v *View) FormatModuleAsMarkdown(kind string, data map[string]any) string {
	if data == nil {
		return ""
	}
	moduleID := textValue(data, "id")
	if kind == "plate" {
		if pl, _ := v.findPlate(moduleID); pl != nil {
			geom, _ := geometryForPlate(pl.Format, pl.Topology, pl.Rows, pl.Columns)
			return formatPlateAsMarkdown(pl, geom, v.Doc)
		}
	}

	if kind == "checklist" {
		title := firstNonEmpty(textValue(data, "title"), "操作检查清单")
		var b strings.Builder
		fmt.Fprintf(&b, "### %s\n\n", title)
		for idx, item := range mapSlice(data["items"]) {
			id := firstNonEmpty(textValue(item, "id"), fmt.Sprintf("chk-%d", idx+1))
			lbl := firstNonEmpty(textValue(item, "label"), textValue(item, "text"), id)
			mark := " "
			if v.isChecklistDone(id, item) {
				mark = "x"
			}
			if v != nil && v.SkippedItems != nil && v.SkippedItems[id] != "" {
				fmt.Fprintf(&b, "- [-] %s *(跳过: %s)*\n", lbl, v.SkippedItems[id])
			} else {
				fmt.Fprintf(&b, "- [%s] %s\n", mark, lbl)
			}
		}
		return strings.TrimSpace(b.String())
	}

	if kind == "prose" {
		return strings.TrimSpace(firstNonEmpty(textValue(data, "markdown"), textValue(data, "text"), textValue(data, "body")))
	}

	title, headers, rows := v.extractTabularModule(kind, data)
	if len(headers) > 0 {
		var b strings.Builder
		if title != "" {
			fmt.Fprintf(&b, "### %s\n\n", title)
		}
		if kind == "calculator" {
			var doc *Document
			var actuals *CalcActualContext
			if v != nil {
				doc = v.Doc
				actuals = v.CalcActuals
			}
			if res := EvalCalculatorWithActuals(doc, data, actuals); res.Summary != "" {
				fmt.Fprintf(&b, "> **%s**\n\n", res.Summary)
			}
		}
		b.WriteString("| " + strings.Join(headers, " | ") + " |\n")
		seps := make([]string, len(headers))
		for i := range seps {
			seps[i] = "---"
		}
		b.WriteString("| " + strings.Join(seps, " | ") + " |\n")
		for _, r := range rows {
			escaped := make([]string, len(headers))
			for i := 0; i < len(headers); i++ {
				val := ""
				if i < len(r) {
					val = strings.ReplaceAll(strings.TrimSpace(r[i]), "|", "\\|")
				}
				if val == "" {
					val = "—"
				}
				escaped[i] = val
			}
			b.WriteString("| " + strings.Join(escaped, " | ") + " |\n")
		}
		return strings.TrimSpace(b.String())
	}

	return fmt.Sprintf("```yaml\nkind: %s\nid: %s\ntitle: %s\n```", kind, moduleID, textValue(data, "title"))
}

func (v *View) isChecklistDone(cid string, item map[string]any) bool {
	if asBool(item["done"]) || asBool(item["checked"]) {
		return true
	}
	if v != nil && v.CheckedItems != nil {
		if v.CheckedItems[cid] || (v.StepID != "" && v.CheckedItems[v.StepID+":"+cid]) {
			return true
		}
	}
	return false
}

func (v *View) lookupActualParam(moduleID, key string, rm map[string]any) string {
	if v != nil && v.ActualParams != nil && key != "" {
		if v.StepID != "" {
			if rec := strings.TrimSpace(v.ActualParams[v.StepID+":"+moduleID+":"+key]); rec != "" {
				return rec
			}
		}
		if rec := strings.TrimSpace(v.ActualParams[moduleID+":"+key]); rec != "" {
			return rec
		}
		if rec := strings.TrimSpace(v.ActualParams[key]); rec != "" {
			return rec
		}
	}
	return strings.TrimSpace(textValue(rm, "actual"))
}

func (v *View) extractTabularModule(kind string, data map[string]any) (title string, headers []string, rows [][]string) {
	moduleID := textValue(data, "id")
	switch kind {
	case "group_table":
		title = firstNonEmpty(textValue(data, "title"), "实验分组表")
		headers = []string{"组别名称", "角色", "处理条件", "样本量"}
		for _, rm := range mapSlice(data["rows"]) {
			gid := textValue(rm, "id")
			gname := firstNonEmpty(textValue(rm, "name"), gid)
			role := firstNonEmpty(groupRoleLabel(textValue(rm, "role")), textValue(rm, "role"), "—")
			cond := firstNonEmpty(textValue(rm, "condition"), "—")
			indN, _ := numericValue(rm["independent_n"])
			techR, _ := numericValue(rm["technical_repeats"])
			if techR == 0 {
				techR, _ = numericValue(rm["replicate_count"])
			}
			nStr := "—"
			if indN > 0 && techR > 0 {
				nStr = fmt.Sprintf("n=%d × %d", int(indN), int(techR))
			} else if indN > 0 {
				nStr = fmt.Sprintf("n=%d", int(indN))
			}
			rows = append(rows, []string{gname, role, cond, nStr})
		}
	case "param_table":
		title = firstNonEmpty(textValue(data, "title"), "实验条件")
		headers = []string{"参数", "计划值", "实测记录"}
		for idx, rm := range mapSlice(data["rows"]) {
			key := firstNonEmpty(textValue(rm, "key"), fmt.Sprintf("p-%d", idx))
			label := firstNonEmpty(textValue(rm, "label"), textValue(rm, "name"), key)
			unit := DisplayUnit(textValue(rm, "unit"))
			planned := formatAnyValue(rm["value"])
			if planned == "" {
				planned = firstNonEmpty(textValue(rm, "target"), "—")
			}
			if unit != "" && planned != "—" {
				planned += " " + unit
			}
			rec := firstNonEmpty(v.lookupActualParam(moduleID, key, rm), "—")
			if rec != "—" && unit != "" && !strings.HasSuffix(rec, unit) {
				rec += " " + unit
			}
			rows = append(rows, []string{label, planned, rec})
		}
	case "data_table":
		title = firstNonEmpty(textValue(data, "title"), "数据表")
		cols := mapSlice(data["columns"])
		colKeys := make([]string, 0, len(cols))
		for _, cm := range cols {
			ckey := firstNonEmpty(textValue(cm, "key"), textValue(cm, "id"))
			hdr := firstNonEmpty(textValue(cm, "label"), textValue(cm, "title"), textValue(cm, "name"), ckey)
			if u := DisplayUnit(textValue(cm, "unit")); u != "" {
				hdr += " (" + u + ")"
			}
			headers = append(headers, hdr)
			colKeys = append(colKeys, ckey)
		}
		var liveCells map[string]string
		if v != nil && v.TableCells != nil {
			liveCells = v.TableCells[moduleID]
		}
		authored := mapSlice(data["rows"])
		for rIdx, rm := range authored {
			row := make([]string, len(colKeys))
			for cIdx, ckey := range colKeys {
				val := formatAnyValue(rm[ckey])
				if liveCells != nil {
					k := fmt.Sprintf("%d:%s", rIdx, ckey)
					if lv := strings.TrimSpace(liveCells[k]); lv != "" {
						val = lv
					}
				}
				row[cIdx] = firstNonEmpty(val, "—")
			}
			rows = append(rows, row)
		}
	case "calculator":
		title = firstNonEmpty(textValue(data, "title"), "科学计算器")
		headers = []string{"分项", "计算值", "计算说明"}
		var doc *Document
		var actuals *CalcActualContext
		if v != nil {
			doc = v.Doc
			actuals = v.CalcActuals
		}
		res := EvalCalculatorWithActuals(doc, data, actuals)
		for _, r := range res.Rows {
			amt := formatNumber(r.Amount)
			if r.Unit != "" {
				amt += " " + DisplayUnit(r.Unit)
			}
			rows = append(rows, []string{r.Label, amt, firstNonEmpty(r.Note, "—")})
		}
	case "timer":
		title = firstNonEmpty(textValue(data, "title"), "实验计时与周期")
		headers = []string{"计时名称", "设定时长", "模式"}
		plannedSec, spanSec, _ := timerBudgetSec(data)
		sec := plannedSec
		if sec == 0 {
			sec = spanSec
		}
		rows = append(rows, []string{title, formatClockHMS(sec), timerModeLabel(textValue(data, "mode"))})
	}
	return title, headers, rows
}

func formatPlateAsMarkdown(p *Plate, g Geometry, doc *Document) string {
	wellGroups := p.WellGroups()
	lblMap := PlateGroupLabelMap(doc)
	excluded := map[string]bool{}
	for _, ex := range p.ExcludedWells {
		excluded[strings.ToUpper(strings.TrimSpace(ex))] = true
	}

	var b strings.Builder
	title := firstNonEmpty(p.Title, fmt.Sprintf("%d 孔板布局 (%s)", p.Format, p.ID))
	fmt.Fprintf(&b, "### %s\n\n", title)
	b.WriteString("| |")
	for col := 0; col < g.Columns; col++ {
		fmt.Fprintf(&b, " %d |", col+1)
	}
	b.WriteString("\n|---|")
	for col := 0; col < g.Columns; col++ {
		b.WriteString("---|")
	}
	b.WriteString("\n")
	for r := 0; r < g.Rows; r++ {
		rowLbl := string(rune('A' + r))
		if p.Topology == "rack_1d" {
			rowLbl = formatInt(r + 1)
		}
		fmt.Fprintf(&b, "| **%s** |", rowLbl)
		for col := 0; col < g.Columns; col++ {
			id := plateWellIDFor(g, p.Topology, r, col)
			cell := ""
			if grp := wellGroups[id]; grp != "" {
				cell = lblMap[grp]
				if cell == "" {
					cell = grp
				}
			}
			if excluded[id] {
				if cell != "" {
					cell += " (×)"
				} else {
					cell = "×"
				}
			}
			if cell == "" {
				cell = "·"
			}
			fmt.Fprintf(&b, " %s |", strings.ReplaceAll(cell, "|", "\\|"))
		}
		b.WriteString("\n")
	}
	return strings.TrimSpace(b.String())
}

func renderVectorTableSVG(title, badge string, headers []string, rows [][]string, tok theme.Tokens) string {
	colCount := len(headers)
	colW := 136
	if colCount <= 2 {
		colW = 280
	} else if colCount == 3 {
		colW = 192
	} else if colCount == 4 {
		colW = 148
	}
	w := 48 + colCount*colW
	if w < 520 {
		w = 520
	}
	hdrY := 52
	rowH := 34
	h := hdrY + 28 + len(rows)*rowH + 22

	var b strings.Builder
	writeSVGCardShell(&b, w, h, title, badge, tok)
	fmt.Fprintf(&b, `  <rect x="20" y="%d" width="%d" height="28" rx="6" fill="%s"/>`+"\n", hdrY, w-40, svgHex(tok.Elevated))
	for cIdx, hdr := range headers {
		fmt.Fprintf(&b, `  <text x="%d" y="%d" font-family="Microsoft YaHei, PingFang SC, Inter, sans-serif" font-size="11.5" font-weight="600" fill="%s">%s</text>`+"\n",
			32+cIdx*colW, hdrY+18, svgHex(tok.TextMuted), html.EscapeString(abbreviateCell(hdr, 18)))
	}
	for rIdx, r := range rows {
		y := hdrY + 28 + rIdx*rowH
		if rIdx > 0 {
			fmt.Fprintf(&b, `  <line x1="22" y1="%d" x2="%d" y2="%d" stroke="%s" stroke-width="1"/>`+"\n", y, w-22, y, svgHex(tok.Border))
		}
		for cIdx := 0; cIdx < colCount; cIdx++ {
			val := "—"
			if cIdx < len(r) && strings.TrimSpace(r[cIdx]) != "" {
				val = r[cIdx]
			}
			weight := "400"
			if cIdx == 0 {
				weight = "600"
			}
			fmt.Fprintf(&b, `  <text x="%d" y="%d" font-family="Microsoft YaHei, PingFang SC, Inter, sans-serif" font-size="12.5" font-weight="%s" fill="%s">%s</text>`+"\n",
				32+cIdx*colW, y+21, weight, svgHex(tok.Text), html.EscapeString(abbreviateCell(val, 22)))
		}
	}
	b.WriteString("</svg>\n")
	return b.String()
}

func renderVectorTextCardSVG(title, badge, body string, tok theme.Tokens) string {
	lines := strings.Split(strings.TrimSpace(body), "\n")
	if len(lines) > 14 {
		lines = append(lines[:13], "…")
	}
	w := 580
	h := 62 + len(lines)*21 + 22
	var b strings.Builder
	writeSVGCardShell(&b, w, h, title, badge, tok)
	for i, line := range lines {
		fmt.Fprintf(&b, `  <text x="24" y="%d" font-family="Microsoft YaHei, PingFang SC, Inter, sans-serif" font-size="12.5" fill="%s">%s</text>`+"\n",
			68+i*21, svgHex(tok.TextMuted), html.EscapeString(abbreviateCell(line, 58)))
	}
	b.WriteString("</svg>\n")
	return b.String()
}

// buildComponentShareMenu populates standardized standalone copy & export actions
// (PNG, SVG, Markdown, and CSV for plates) across Edit, Read, and Run modes.
func (v *View) buildComponentShareMenu(c *ui.Context, m *components.Menu, kind string, schema int, data map[string]any) {
	if v == nil || data == nil {
		return
	}
	if schema <= 0 {
		schema = 1
	}
	moduleID := textValue(data, "id")
	dark := c.Theme() != nil && c.Theme().Dark
	baseName := firstNonEmpty(sanitizeIDFrag(moduleID), kind, "component")

	if v.OnCopyImage != nil && m.Item("复制为 PNG 图片").Chosen() {
		if pngBytes, err := v.RenderModulePNG(kind, schema, data, dark); err == nil && len(pngBytes) > 0 {
			if err := v.OnCopyImage(pngBytes); err == nil {
				c.AddToast(ui.Toast{Title: "已复制组件高清 PNG 图片", Type: components.ToastSuccess})
			}
		}
	}
	if m.Item("复制为 SVG").Chosen() {
		if svgText := v.RenderModuleSVG(kind, data, dark); svgText != "" {
			c.Services().WriteClipboard(svgText)
			c.AddToast(ui.Toast{Title: "已复制组件 SVG 矢量代码", Type: components.ToastSuccess})
		}
	}
	if m.Item("复制为 Markdown").Chosen() {
		if mdText := v.FormatModuleAsMarkdown(kind, data); mdText != "" {
			c.Services().WriteClipboard(mdText)
			c.AddToast(ui.Toast{Title: "已复制组件 Markdown", Type: components.ToastSuccess})
		}
	}
	if v.OnSavePlateExport != nil {
		m.Separator()
		if m.Item("导出单张 PNG...").Chosen() {
			if pngBytes, err := v.RenderModulePNG(kind, schema, data, dark); err == nil && len(pngBytes) > 0 {
				_ = v.OnSavePlateExport(baseName+".png", "png", pngBytes)
			}
		}
		if m.Item("导出单张 SVG...").Chosen() {
			if svgText := v.RenderModuleSVG(kind, data, dark); svgText != "" {
				_ = v.OnSavePlateExport(baseName+".svg", "svg", []byte(svgText))
			}
		}
		if kind == "plate" {
			if pl, _ := v.findPlate(moduleID); pl != nil {
				geom, _ := geometryForPlate(pl.Format, pl.Topology, pl.Rows, pl.Columns)
				if m.Item("导出孔板 CSV...").Chosen() {
					csvText := formatPlateAsCSV(pl, geom)
					c.Services().WriteClipboard(csvText)
					_ = v.OnSavePlateExport(baseName+".csv", "csv", []byte(csvText))
				}
			}
		}
	}
}

func svgHex(col ui.Color) string {
	return fmt.Sprintf("#%02x%02x%02x", col.R, col.G, col.B)
}

func asBool(raw any) bool {
	b, _ := raw.(bool)
	return b
}

func abbreviateCell(s string, maxRunes int) string {
	s = strings.TrimSpace(s)
	runes := []rune(s)
	if len(runes) <= maxRunes || maxRunes < 3 {
		return s
	}
	return string(runes[:maxRunes-1]) + "…"
}
