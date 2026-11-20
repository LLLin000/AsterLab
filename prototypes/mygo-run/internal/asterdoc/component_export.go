package asterdoc

import (
	"fmt"
	"html"
	"sort"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/onevra/source/components"

	"asterlab-mygo-prototype/internal/theme"
	plateui "asterlab-mygo-prototype/internal/uikit/plate"
)

// RenderPlateSVG renders a crisp publication-grade standalone SVG vector diagram
// of a plate, tube rack, dish, or cohort layout with well coordinates and legend.
func RenderPlateSVG(p *Plate, g Geometry, doc *Document, colorBy, factorKey string, dark bool) string {
	if p == nil || g.Rows <= 0 || g.Columns <= 0 {
		return ""
	}
	if colorBy == "" {
		colorBy = "group"
	}
	tok := theme.LightTokens()
	if dark {
		tok = theme.DarkTokens()
	}

	cell := 34
	gap := 6
	if g.Columns > 12 {
		cell = 20
		gap = 4
	}
	padX := 34
	headerH := 48
	colHdrH := 20
	gridW := g.Columns*cell + (g.Columns-1)*gap
	gridH := g.Rows*cell + (g.Rows-1)*gap

	wellGroups := p.WellGroups()
	idxMap := plateGroupPaletteMap(doc, p)
	lblMap := PlateGroupLabelMap(doc)
	excluded := map[string]bool{}
	for _, ex := range p.ExcludedWells {
		excluded[strings.ToUpper(strings.TrimSpace(ex))] = true
	}

	factorVals := []string{}
	if colorBy == "factor" && factorKey != "" {
		seen := map[string]bool{}
		for r := 0; r < g.Rows; r++ {
			for col := 0; col < g.Columns; col++ {
				id := plateWellIDFor(g, p.Topology, r, col)
				if fv := plateFactorValue(p, id, factorKey); fv != "" && !seen[fv] {
					seen[fv] = true
					factorVals = append(factorVals, fv)
				}
			}
		}
		sort.Strings(factorVals)
	}
	factorIdx := map[string]int{}
	for i, fv := range factorVals {
		factorIdx[fv] = i
	}
	palette := theme.CategoricalPalette(dark)

	// Collect legend entries in deterministic order
	type legendEntry struct {
		label string
		hex   string
		count int
	}
	legendMap := map[string]*legendEntry{}
	var legendOrder []string
	recordLegend := func(key, label string, col ui.Color) {
		ent := legendMap[key]
		if ent == nil {
			ent = &legendEntry{label: label, hex: svgHex(col)}
			legendMap[key] = ent
			legendOrder = append(legendOrder, key)
		}
		ent.count++
	}

	var cellsSVG strings.Builder
	for r := 0; r < g.Rows; r++ {
		if p.Topology != "rack_1d" {
			rowLbl := string(rune('A' + r))
			if r >= 26 {
				rowLbl = formatInt(r + 1)
			}
			ry := headerH + colHdrH + r*(cell+gap) + cell/2 + 4
			fmt.Fprintf(&cellsSVG, `<text x="%d" y="%d" font-size="11" font-family="ui-monospace,monospace" fill="%s" text-anchor="end">%s</text>`,
				padX-8, ry, svgHex(tok.TextMuted), html.EscapeString(rowLbl))
		}
		for col := 0; col < g.Columns; col++ {
			if r == 0 && p.Topology != "rack_1d" {
				cxHdr := padX + col*(cell+gap) + cell/2
				fmt.Fprintf(&cellsSVG, `<text x="%d" y="%d" font-size="11" font-family="ui-monospace,monospace" fill="%s" text-anchor="middle">%d</text>`,
					cxHdr, headerH+13, svgHex(tok.TextMuted), col+1)
			}
			id := plateWellIDFor(g, p.Topology, r, col)
			cx := padX + col*(cell+gap) + cell/2
			cy := headerH + colHdrH + r*(cell+gap) + cell/2
			x0 := padX + col*(cell+gap)
			y0 := headerH + colHdrH + r*(cell+gap)

			fillHex := svgHex(tok.Elevated)
			strokeHex := svgHex(tok.BorderStrong)
			textCol := svgHex(tok.TextMuted)
			hasFill := false
			cellLbl := ""

			if colorBy == "factor" && factorKey != "" {
				if fv := plateFactorValue(p, id, factorKey); fv != "" {
					uc := palette[factorIdx[fv]%len(palette)]
					fillHex = svgHex(uc)
					strokeHex = fillHex
					textCol = "#ffffff"
					hasFill = true
					cellLbl = abbreviateCell(fv, 4)
					recordLegend("f:"+fv, fv, uc)
				}
			} else if grp := wellGroups[id]; grp != "" {
				uc := plateui.GroupColorIndexed(grp, idxMap, dark)
				fillHex = svgHex(uc)
				strokeHex = fillHex
				textCol = "#ffffff"
				hasFill = true
				disp := lblMap[grp]
				if disp == "" {
					disp = grp
				}
				cellLbl = abbreviateCell(disp, 4)
				recordLegend("g:"+grp, disp, uc)
			}

			if p.Topology == "rack_1d" {
				fmt.Fprintf(&cellsSVG, `<rect x="%d" y="%d" width="%d" height="%d" rx="8" fill="%s" stroke="%s" stroke-width="1.3"/>`,
					x0, y0, cell, cell, fillHex, strokeHex)
			} else {
				fmt.Fprintf(&cellsSVG, `<circle cx="%d" cy="%d" r="%.1f" fill="%s" stroke="%s" stroke-width="1.3"/>`,
					cx, cy, float64(cell)/2.0-1.0, fillHex, strokeHex)
			}
			if p.Topology == "rack_1d" && !hasFill {
				cellLbl = id
			}
			if cellLbl != "" && cell >= 24 {
				fmt.Fprintf(&cellsSVG, `<text x="%d" y="%d" font-size="9" font-family="ui-sans-serif,system-ui,sans-serif" font-weight="%s" fill="%s" text-anchor="middle">%s</text>`,
					cx, cy+3, map[bool]string{true: "600", false: "400"}[hasFill], textCol, html.EscapeString(cellLbl))
			}
			if excluded[id] {
				off := cell / 4
				fmt.Fprintf(&cellsSVG, `<path d="M%d %d L%d %d M%d %d L%d %d" stroke="%s" stroke-width="2" stroke-linecap="round"/>`,
					cx-off, cy-off, cx+off, cy+off, cx+off, cy-off, cx-off, cy+off, svgHex(tok.Danger))
			}
		}
	}

	legendH := 28
	if len(legendOrder) > 0 {
		legendH = 38
	}
	w := padX*2 + gridW
	if w < 380 {
		w = 380
	}
	h := headerH + colHdrH + gridH + legendH + 18

	title := firstNonEmpty(p.Title, fmt.Sprintf("%d 孔板布局 (%s)", p.Format, p.ID))
	sub := fmt.Sprintf("%d×%d · %s", g.Rows, g.Columns, colorBy)
	if colorBy == "factor" && factorKey != "" {
		sub = fmt.Sprintf("%d×%d · 维度: %s", g.Rows, g.Columns, factorKey)
	}

	var b strings.Builder
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 %d %d">`, w, h, w, h)
	fmt.Fprintf(&b, `<rect width="%d" height="%d" rx="12" fill="%s" stroke="%s" stroke-width="1"/>`, w, h, svgHex(tok.Canvas), svgHex(tok.Border))
	fmt.Fprintf(&b, `<text x="20" y="25" font-size="14" font-weight="600" font-family="system-ui,-apple-system,sans-serif" fill="%s">%s</text>`, svgHex(tok.Text), html.EscapeString(title))
	fmt.Fprintf(&b, `<text x="%d" y="25" font-size="11" font-family="ui-monospace,monospace" fill="%s" text-anchor="end">%s</text>`, w-20, svgHex(tok.TextMuted), html.EscapeString(sub))
	fmt.Fprintf(&b, `<line x1="20" y1="36" x2="%d" y2="36" stroke="%s" stroke-width="1"/>`, w-20, svgHex(tok.Border))
	b.WriteString(cellsSVG.String())

	ly := headerH + colHdrH + gridH + 18
	lx := 20
	for _, key := range legendOrder {
		ent := legendMap[key]
		lbl := fmt.Sprintf("%s (%d)", ent.label, ent.count)
		chipW := len([]rune(lbl))*7 + 28
		if lx+chipW > w-20 {
			break
		}
		fmt.Fprintf(&b, `<circle cx="%d" cy="%d" r="5" fill="%s"/>`, lx+5, ly, ent.hex)
		fmt.Fprintf(&b, `<text x="%d" y="%d" font-size="11" font-family="system-ui,-apple-system,sans-serif" fill="%s">%s</text>`,
			lx+14, ly+4, svgHex(tok.TextMuted), html.EscapeString(lbl))
		lx += chipW
	}
	fmt.Fprintf(&b, `<text x="%d" y="%d" font-size="10" font-family="ui-monospace,monospace" fill="%s" text-anchor="end">AsterLab · .asterml</text>`,
		w-20, h-10, svgHex(tok.TextMuted))
	b.WriteString(`</svg>`)
	return b.String()
}

// RenderModuleSVG renders any structured AsterML block (plate, table, checklist,
// calculator, timer, figure) as a standalone sharable vector SVG card.
func (v *View) RenderModuleSVG(kind string, data map[string]any, dark bool) string {
	if data == nil {
		return ""
	}
	moduleID := textValue(data, "id")
	if kind == "plate" {
		if pl, _ := v.findPlate(moduleID); pl != nil {
			geom, _ := geometryForPlate(pl.Format, pl.Topology, pl.Rows, pl.Columns)
			return RenderPlateSVG(pl, geom, v.Doc, v.plateColorBy[pl.ID], v.plateFactorKey[pl.ID], dark)
		}
	}
	if kind == "figure" {
		if rawSVG := strings.TrimSpace(textValue(data, "svg")); strings.Contains(rawSVG, "<svg") {
			return rawSVG
		}
	}

	tok := theme.LightTokens()
	if dark {
		tok = theme.DarkTokens()
	}

	title, headers, rows := v.extractTabularModule(kind, data)
	if len(headers) > 0 {
		return renderVectorTableSVG(title, kind, headers, rows, tok)
	}
	md := v.FormatModuleAsMarkdown(kind, data)
	return renderVectorTextCardSVG(firstNonEmpty(title, kind), kind, md, tok)
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
		title := firstNonEmpty(textValue(data, "title"), "操作核对清单")
		var b strings.Builder
		fmt.Fprintf(&b, "### %s\n\n", title)
		for _, item := range mapSlice(data["items"]) {
			id := textValue(item, "id")
			lbl := firstNonEmpty(textValue(item, "label"), textValue(item, "text"), id)
			checked := asBool(item["checked"])
			if v != nil && v.CheckedItems != nil && v.CheckedItems[id] {
				checked = true
			}
			mark := " "
			if checked {
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

func (v *View) extractTabularModule(kind string, data map[string]any) (title string, headers []string, rows [][]string) {
	moduleID := textValue(data, "id")
	switch kind {
	case "group_table":
		title = firstNonEmpty(textValue(data, "title"), "实验分组表")
		headers = []string{"组别", "角色", "处理条件", "独立样本量 n"}
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
		title = firstNonEmpty(textValue(data, "title"), "关键参数与实测偏离")
		headers = []string{"参数", "计划值", "实测值", "单位"}
		for _, rm := range mapSlice(data["rows"]) {
			key := firstNonEmpty(textValue(rm, "key"), textValue(rm, "id"), textValue(rm, "name"))
			label := firstNonEmpty(textValue(rm, "label"), textValue(rm, "name"), key)
			planned := firstNonEmpty(textValue(rm, "value"), textValue(rm, "target"), textValue(rm, "planned"), "—")
			unit := DisplayUnit(textValue(rm, "unit"))
			actual := firstNonEmpty(textValue(rm, "actual"), "—")
			if v != nil && v.ActualParams != nil && key != "" && v.ActualParams[key] != "" {
				actual = v.ActualParams[key]
			}
			rows = append(rows, []string{label, planned, actual, unit})
		}
	case "data_table":
		title = firstNonEmpty(textValue(data, "title"), "二维数据记录表")
		cols := mapSlice(data["columns"])
		colIDs := make([]string, 0, len(cols))
		for _, cm := range cols {
			cid := textValue(cm, "id")
			hdr := firstNonEmpty(textValue(cm, "title"), textValue(cm, "name"), cid)
			if u := DisplayUnit(textValue(cm, "unit")); u != "" {
				hdr += " (" + u + ")"
			}
			headers = append(headers, hdr)
			colIDs = append(colIDs, cid)
		}
		var liveCells map[string]string
		if v != nil && v.TableCells != nil {
			liveCells = v.TableCells[moduleID]
		}
		authored := mapSlice(data["rows"])
		for rIdx, rm := range authored {
			row := make([]string, len(colIDs))
			for cIdx, cid := range colIDs {
				val := fmt.Sprintf("%v", rm[cid])
				if rm[cid] == nil {
					val = ""
				}
				if liveCells != nil {
					k := fmt.Sprintf("%d:%s", rIdx, cid)
					if lv := strings.TrimSpace(liveCells[k]); lv != "" {
						val = lv
					}
				}
				row[cIdx] = firstNonEmpty(val, "—")
			}
			rows = append(rows, row)
		}
	case "calculator":
		title = firstNonEmpty(textValue(data, "title"), "配方与稀释计算")
		headers = []string{"项目", "数值 / 定义"}
		if f := textValue(data, "formula"); f != "" {
			rows = append(rows, []string{"计算类型", f})
		}
		for _, vm := range mapSlice(data["inputs"]) {
			k := firstNonEmpty(textValue(vm, "label"), textValue(vm, "key"), textValue(vm, "name"))
			val := firstNonEmpty(textValue(vm, "value"), "—")
			if u := DisplayUnit(textValue(vm, "unit")); u != "" {
				val += " " + u
			}
			rows = append(rows, []string{k, val})
		}
	case "timer":
		title = firstNonEmpty(textValue(data, "title"), "步骤计时器")
		headers = []string{"步骤", "时长"}
		sec, _ := numericValue(data["duration_sec"])
		dur := textValue(data, "duration")
		if dur == "" && sec > 0 {
			dur = fmt.Sprintf("%d 秒", int(sec))
		}
		rows = append(rows, []string{firstNonEmpty(textValue(data, "label"), title), firstNonEmpty(dur, "—")})
	case "checklist":
		title = firstNonEmpty(textValue(data, "title"), "操作核对清单")
		headers = []string{"状态", "检查项"}
		for _, item := range mapSlice(data["items"]) {
			id := textValue(item, "id")
			lbl := firstNonEmpty(textValue(item, "label"), textValue(item, "text"), id)
			state := "待执行"
			if asBool(item["checked"]) || (v != nil && v.CheckedItems != nil && v.CheckedItems[id]) {
				state = "已完成"
			} else if v != nil && v.SkippedItems != nil && v.SkippedItems[id] != "" {
				state = "跳过: " + v.SkippedItems[id]
			}
			rows = append(rows, []string{state, lbl})
		}
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

func renderVectorTableSVG(title, kind string, headers []string, rows [][]string, tok theme.Tokens) string {
	colCount := len(headers)
	colW := 136
	if colCount <= 2 {
		colW = 240
	} else if colCount == 3 {
		colW = 176
	}
	w := 40 + colCount*colW
	if w < 440 {
		w = 440
	}
	rowH := 30
	headerH := 48
	tblHdrH := 30
	h := headerH + tblHdrH + len(rows)*rowH + 34

	var b strings.Builder
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 %d %d">`, w, h, w, h)
	fmt.Fprintf(&b, `<rect width="%d" height="%d" rx="10" fill="%s" stroke="%s" stroke-width="1"/>`, w, h, svgHex(tok.Canvas), svgHex(tok.Border))
	fmt.Fprintf(&b, `<text x="20" y="27" font-size="14" font-weight="600" font-family="system-ui,-apple-system,sans-serif" fill="%s">%s</text>`, svgHex(tok.Text), html.EscapeString(title))
	fmt.Fprintf(&b, `<text x="%d" y="27" font-size="11" font-family="ui-monospace,monospace" fill="%s" text-anchor="end">%s</text>`, w-20, svgHex(tok.Accent), html.EscapeString(kind))

	fmt.Fprintf(&b, `<rect x="20" y="%d" width="%d" height="%d" rx="6" fill="%s"/>`, headerH, w-40, tblHdrH, svgHex(tok.Elevated))
	for cIdx, hdr := range headers {
		fmt.Fprintf(&b, `<text x="%d" y="%d" font-size="11" font-weight="600" font-family="system-ui,-apple-system,sans-serif" fill="%s">%s</text>`,
			30+cIdx*colW, headerH+19, svgHex(tok.TextMuted), html.EscapeString(abbreviateCell(hdr, 20)))
	}
	for rIdx, r := range rows {
		y := headerH + tblHdrH + rIdx*rowH
		fmt.Fprintf(&b, `<line x1="20" y1="%d" x2="%d" y2="%d" stroke="%s" stroke-width="1"/>`, y+rowH, w-20, y+rowH, svgHex(tok.Border))
		for cIdx := 0; cIdx < colCount; cIdx++ {
			val := "—"
			if cIdx < len(r) && strings.TrimSpace(r[cIdx]) != "" {
				val = r[cIdx]
			}
			weight := "400"
			if cIdx == 0 {
				weight = "600"
			}
			fmt.Fprintf(&b, `<text x="%d" y="%d" font-size="12" font-weight="%s" font-family="system-ui,-apple-system,sans-serif" fill="%s">%s</text>`,
				30+cIdx*colW, y+20, weight, svgHex(tok.Text), html.EscapeString(abbreviateCell(val, 24)))
		}
	}
	fmt.Fprintf(&b, `<text x="%d" y="%d" font-size="10" font-family="ui-monospace,monospace" fill="%s" text-anchor="end">AsterLab · .asterml</text>`,
		w-20, h-10, svgHex(tok.TextMuted))
	b.WriteString(`</svg>`)
	return b.String()
}

func renderVectorTextCardSVG(title, kind, body string, tok theme.Tokens) string {
	lines := strings.Split(strings.TrimSpace(body), "\n")
	if len(lines) > 14 {
		lines = append(lines[:13], "…")
	}
	w := 560
	h := 64 + len(lines)*20 + 24
	var b strings.Builder
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 %d %d">`, w, h, w, h)
	fmt.Fprintf(&b, `<rect width="%d" height="%d" rx="10" fill="%s" stroke="%s" stroke-width="1"/>`, w, h, svgHex(tok.Canvas), svgHex(tok.Border))
	fmt.Fprintf(&b, `<text x="20" y="27" font-size="14" font-weight="600" font-family="system-ui,-apple-system,sans-serif" fill="%s">%s</text>`, svgHex(tok.Text), html.EscapeString(title))
	fmt.Fprintf(&b, `<text x="%d" y="27" font-size="11" font-family="ui-monospace,monospace" fill="%s" text-anchor="end">%s</text>`, w-20, svgHex(tok.Accent), html.EscapeString(kind))
	fmt.Fprintf(&b, `<line x1="20" y1="38" x2="%d" y2="38" stroke="%s" stroke-width="1"/>`, w-20, svgHex(tok.Border))
	for i, line := range lines {
		fmt.Fprintf(&b, `<text x="20" y="%d" font-size="12" font-family="system-ui,-apple-system,sans-serif" fill="%s">%s</text>`,
			58+i*20, svgHex(tok.TextMuted), html.EscapeString(abbreviateCell(line, 64)))
	}
	fmt.Fprintf(&b, `<text x="%d" y="%d" font-size="10" font-family="ui-monospace,monospace" fill="%s" text-anchor="end">AsterLab · .asterml</text>`,
		w-20, h-10, svgHex(tok.TextMuted))
	b.WriteString(`</svg>`)
	return b.String()
}

// buildComponentShareMenu populates standardized standalone copy & export actions
// for any AsterML block or plate inside Edit, Read, or Run modes.
func (v *View) buildComponentShareMenu(c *ui.Context, m *components.Menu, kind string, data map[string]any) {
	if v == nil || data == nil {
		return
	}
	moduleID := textValue(data, "id")
	dark := c.Theme() != nil && c.Theme().Dark
	baseName := firstNonEmpty(sanitizeIDFrag(moduleID), kind, "component")

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
	if kind == "plate" {
		if pl, _ := v.findPlate(moduleID); pl != nil {
			geom, _ := geometryForPlate(pl.Format, pl.Topology, pl.Rows, pl.Columns)
			if v.OnCopyImage != nil && m.Item("复制孔板 PNG 图片").Chosen() {
				if pngBytes, err := RenderPlatePNG(pl, geom, v.Doc, v.plateColorBy[pl.ID], v.plateFactorKey[pl.ID], dark); err == nil && len(pngBytes) > 0 {
					if err := v.OnCopyImage(pngBytes); err == nil {
						c.AddToast(ui.Toast{Title: "已复制孔板图片到剪贴板", Type: components.ToastSuccess})
					}
				}
			}
			if m.Item("导出孔板 CSV").Chosen() {
				csvText := formatPlateAsCSV(pl, geom)
				c.Services().WriteClipboard(csvText)
				if v.OnSavePlateExport != nil {
					_ = v.OnSavePlateExport(baseName+".csv", "csv", []byte(csvText))
				}
			}
			if v.OnSavePlateExport != nil && m.Item("导出孔板 PNG...").Chosen() {
				if pngBytes, err := RenderPlatePNG(pl, geom, v.Doc, v.plateColorBy[pl.ID], v.plateFactorKey[pl.ID], dark); err == nil && len(pngBytes) > 0 {
					_ = v.OnSavePlateExport(baseName+".png", "png", pngBytes)
				}
			}
		}
	}
	if v.OnSavePlateExport != nil && m.Item("导出单张 SVG...").Chosen() {
		if svgText := v.RenderModuleSVG(kind, data, dark); svgText != "" {
			_ = v.OnSavePlateExport(baseName+".svg", "svg", []byte(svgText))
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

func plateFactorValue(p *Plate, well, factorKey string) string {
	if p == nil || factorKey == "" {
		return ""
	}
	if lm, ok := p.Layers[factorKey]; ok && lm[well] != "" {
		return lm[well]
	}
	return p.WellFactors(well)[factorKey]
}
