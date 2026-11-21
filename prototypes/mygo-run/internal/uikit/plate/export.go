package plate

import (
	"bytes"
	"fmt"
	"html"
	"image/png"
	"math"
	"strings"

	"github.com/egoist/mygo/ui"

	"asterlab-mygo-prototype/internal/theme"
)

func svgColor(c ui.Color) string {
	if c.A == 0 {
		return "none"
	}
	if c.A == 255 {
		return fmt.Sprintf("#%02x%02x%02x", c.R, c.G, c.B)
	}
	return fmt.Sprintf("rgba(%d,%d,%d,%.3f)", c.R, c.G, c.B, float64(c.A)/255.0)
}

func svgSolidColor(c ui.Color) string {
	return fmt.Sprintf("#%02x%02x%02x", c.R, c.G, c.B)
}

func containerFormatSummary(rows, cols int, topology string) string {
	switch topology {
	case "dish":
		return fmt.Sprintf("培养皿分区（%d 区 · %d×%d）", rows*cols, rows, cols)
	case "rack_1d":
		return fmt.Sprintf("%d 位联管/泳道（%d×%d）", rows*cols, rows, cols)
	case "cohort":
		return fmt.Sprintf("样本队列（%d×%d）", rows, cols)
	default:
		return fmt.Sprintf("%d 孔板（%d×%d）", rows*cols, rows, cols)
	}
}

// RenderSVG serializes a publication-ready standalone vector SVG card containing
// the plate geometry, wells, adaptive labels, and category color legend.
func RenderSVG(props Props, title string, dark bool) string {
	t := theme.GetTokens(theme.Palette(dark))
	rows, cols := ResolveDimensions(props)
	const exportZoom = float32(1.15)
	cellW, cellH, gap := AdaptiveMetrics(rows, cols, props.Topology, exportZoom)
	leftLabel := float32(t.Spacing24)
	topLabel := float32(t.Spacing16 + t.Spacing4)
	dishPad := float32(0)
	if props.Topology == "dish" {
		dishPad = float32(t.Spacing48)
	}
	gridW := float32(cols)*cellW + float32(cols-1)*gap
	gridH := float32(rows)*cellH + float32(rows-1)*gap
	gridBlockW := leftLabel + gridW + dishPad*2
	gridBlockH := topLabel + gridH + dishPad*2

	padX := float32(28)
	padY := float32(24)
	headerH := float32(48)
	legend := ComputeLegend(props, dark)

	cardW := gridBlockW + padX*2
	if cardW < 500 {
		cardW = 500
	}
	// Estimate legend rows for clean multi-line wrapping inside cardW.
	innerW := cardW - padX*2
	legendRows := 0
	if len(legend) > 0 {
		legendRows = 1
		curX := float32(0)
		for _, item := range legend {
			lbl := fmt.Sprintf("%s %d", item.Label, item.Count)
			itemW := float32(18 + len([]rune(lbl))*11 + 18)
			if curX > 0 && curX+itemW > innerW {
				legendRows++
				curX = 0
			}
			curX += itemW
		}
	}
	legendH := float32(0)
	if legendRows > 0 {
		legendH = float32(16 + legendRows*24)
	}
	cardH := padY + headerH + gridBlockH + legendH + padY

	titleText := strings.TrimSpace(title)
	if titleText == "" {
		titleText = containerFormatSummary(rows, cols, props.Topology)
	}
	subText := fmt.Sprintf("%s · 已分配 %d / %d 孔", containerFormatSummary(rows, cols, props.Topology), len(props.Wells), rows*cols)

	var b strings.Builder
	wInt := int(math.Ceil(float64(cardW)))
	hInt := int(math.Ceil(float64(cardH)))
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 %d %d">`, wInt, hInt, wInt, hInt)
	b.WriteByte('\n')
	fmt.Fprintf(&b, `  <rect x="0" y="0" width="%d" height="%d" fill="%s"/>`, wInt, hInt, svgColor(t.Canvas))
	b.WriteByte('\n')
	fmt.Fprintf(&b, `  <rect x="6" y="6" width="%d" height="%d" rx="12" fill="%s" stroke="%s" stroke-width="1"/>`,
		wInt-12, hInt-12, svgColor(t.Surface), svgColor(t.Border))
	b.WriteByte('\n')
	// Title & subtitle
	fmt.Fprintf(&b, `  <text x="%.1f" y="%.1f" font-family="Microsoft YaHei, PingFang SC, Inter, sans-serif" font-size="16" font-weight="700" fill="%s">%s</text>`,
		padX, padY+16, svgColor(t.Text), html.EscapeString(titleText))
	b.WriteByte('\n')
	fmt.Fprintf(&b, `  <text x="%.1f" y="%.1f" font-family="Microsoft YaHei, PingFang SC, Inter, sans-serif" font-size="12" fill="%s">%s</text>`,
		padX, padY+35, svgColor(t.TextMuted), html.EscapeString(subText))
	b.WriteByte('\n')

	blockOriginX := (cardW - gridBlockW) / 2
	gx0 := blockOriginX + leftLabel + dishPad
	gy0 := padY + headerH + topLabel + dishPad

	if props.Topology == "dish" {
		cx := gx0 + gridW/2
		cy := gy0 + gridH/2
		rOuter := gridW
		if gridH > rOuter {
			rOuter = gridH
		}
		rOuter = rOuter*0.72 + float32(t.Spacing8)
		fmt.Fprintf(&b, `  <circle cx="%.1f" cy="%.1f" r="%.1f" fill="%s" fill-opacity="0.65" stroke="%s" stroke-width="1.5"/>`,
			cx, cy, rOuter, svgSolidColor(t.Elevated), svgColor(t.BorderStrong))
		b.WriteByte('\n')
		innerR := rOuter - float32(t.Spacing8)
		if innerR > 8 {
			fmt.Fprintf(&b, `  <circle cx="%.1f" cy="%.1f" r="%.1f" fill="none" stroke="%s" stroke-width="1"/>`,
				cx, cy, innerR, svgColor(t.Border))
			b.WriteByte('\n')
		}
		if rows == 2 && cols == 2 {
			fmt.Fprintf(&b, `  <line x1="%.1f" y1="%.1f" x2="%.1f" y2="%.1f" stroke="%s" stroke-width="1" stroke-opacity="0.6"/>`,
				cx-rOuter+8, cy, cx+rOuter-8, cy, svgColor(t.BorderStrong))
			b.WriteByte('\n')
			fmt.Fprintf(&b, `  <line x1="%.1f" y1="%.1f" x2="%.1f" y2="%.1f" stroke="%s" stroke-width="1" stroke-opacity="0.6"/>`,
				cx, cy-rOuter+8, cx, cy+rOuter-8, svgColor(t.BorderStrong))
			b.WriteByte('\n')
		}
	}

	// Column headers
	for col := 0; col < cols; col++ {
		cx := gx0 + float32(col)*(cellW+gap) + cellW/2
		cy := gy0 - float32(t.Spacing8)
		fmt.Fprintf(&b, `  <text x="%.1f" y="%.1f" text-anchor="middle" font-family="Inter, sans-serif" font-size="11" fill="%s">%d</text>`,
			cx, cy, svgColor(t.TextMuted), col+1)
		b.WriteByte('\n')
	}

	// Rows + Wells
	labelSize := float32(t.TextMeta)
	for row := 0; row < rows; row++ {
		ry := gy0 + float32(row)*(cellH+gap) + cellH/2
		fmt.Fprintf(&b, `  <text x="%.1f" y="%.1f" text-anchor="middle" dominant-baseline="central" font-family="Inter, sans-serif" font-size="11" font-weight="600" fill="%s">%s</text>`,
			gx0-dishPad-leftLabel/2+2, ry, svgColor(t.TextMuted), html.EscapeString(rowLabel(row)))
		b.WriteByte('\n')

		for col := 0; col < cols; col++ {
			id := wellID(row, col)
			x := gx0 + float32(col)*(cellW+gap)
			y := gy0 + float32(row)*(cellH+gap)
			well, assigned := props.Wells[id]
			role := effectiveRole(well, id, props.Roles)
			fill := WellFillColor(props, well, assigned, role, dark, t)
			strokeCol := t.Border
			if assigned && (len(well.Factors) > 0 || well.SampleID != "") && !props.Blind {
				strokeCol = fill.Mix(t.Text, 0.28)
			}
			ink := contrastInkForFill(fill, assigned, t)

			if props.Topology == "rack_1d" || props.Topology == "cohort" || cellW != cellH {
				rx := float32(t.ControlRadius) * 2
				if props.Topology == "cohort" {
					rx = float32(t.ControlRadius)
				}
				fmt.Fprintf(&b, `  <rect x="%.1f" y="%.1f" width="%.1f" height="%.1f" rx="%.1f" ry="%.1f" fill="%s" stroke="%s" stroke-width="1"/>`,
					x, y, cellW, cellH, rx, rx, svgColor(fill), svgColor(strokeCol))
				b.WriteByte('\n')
			} else {
				cx := x + cellW/2
				cy := y + cellH/2
				r := cellW / 2
				fmt.Fprintf(&b, `  <circle cx="%.1f" cy="%.1f" r="%.1f" fill="%s" stroke="%s" stroke-width="1"/>`,
					cx, cy, r, svgColor(fill), svgColor(strokeCol))
				b.WriteByte('\n')
				if !props.Blind && assigned && well.SampleID != "" && len(well.Factors) > 0 && well.Replicate > 0 && cellW >= 20 {
					innerR := r - 2.5
					if innerR < 2 {
						innerR = 2
					}
					fmt.Fprintf(&b, `  <circle cx="%.1f" cy="%.1f" r="%.1f" fill="none" stroke="%s" stroke-opacity="0.4" stroke-width="1"/>`,
						cx, cy, innerR, svgSolidColor(ink))
					b.WriteByte('\n')
				}
			}

			cx := x + cellW/2
			cy := y + cellH/2
			if props.ExcludedIDs[id] {
				pad := cellW * 0.28
				fmt.Fprintf(&b, `  <path d="M%.1f %.1f L%.1f %.1f M%.1f %.1f L%.1f %.1f" stroke="%s" stroke-width="2.2" stroke-linecap="round"/>`,
					x+pad, y+pad, x+cellW-pad, y+cellH-pad, x+cellW-pad, y+pad, x+pad, y+cellH-pad, svgColor(t.Danger))
				b.WriteByte('\n')
			} else if well.Ambiguous {
				fmt.Fprintf(&b, `  <text x="%.1f" y="%.1f" text-anchor="middle" dominant-baseline="central" font-family="Inter, sans-serif" font-size="%.1f" font-weight="700" fill="%s">!</text>`,
					cx, cy, labelSize*0.9, svgColor(ink))
				b.WriteByte('\n')
			} else if !props.Blind && assigned {
				badge := wellGlyphAdaptive(props, well, role, cellW)
				if badge != "" {
					fontPx := labelSize * 0.72
					if cellW >= 40 {
						fontPx = labelSize * 0.88
					} else if len([]rune(badge)) == 1 {
						fontPx = labelSize * 0.84
					}
					showRepSub := well.Replicate > 0 && cellW >= 38 && props.ColorBy != "replicate"
					textY := cy
					if showRepSub {
						textY = cy - fontPx*0.42
					}
					fmt.Fprintf(&b, `  <text x="%.1f" y="%.1f" text-anchor="middle" dominant-baseline="central" font-family="Microsoft YaHei, PingFang SC, Inter, sans-serif" font-size="%.1f" font-weight="700" fill="%s">%s</text>`,
						cx, textY, fontPx, svgColor(ink), html.EscapeString(badge))
					b.WriteByte('\n')
					if showRepSub {
						fmt.Fprintf(&b, `  <text x="%.1f" y="%.1f" text-anchor="middle" dominant-baseline="central" font-family="Inter, sans-serif" font-size="%.1f" fill="%s" fill-opacity="0.85">#%d</text>`,
							cx, cy+fontPx*0.58, labelSize*0.64, svgSolidColor(ink), well.Replicate)
						b.WriteByte('\n')
					}
				}
			} else if rb := roleShort(role); rb != "" {
				fmt.Fprintf(&b, `  <text x="%.1f" y="%.1f" text-anchor="middle" dominant-baseline="central" font-family="Inter, sans-serif" font-size="%.1f" font-weight="700" fill="%s">%s</text>`,
					cx, cy, labelSize*0.78, svgColor(t.Text), html.EscapeString(rb))
				b.WriteByte('\n')
			}
		}
	}

	// Color legend (centered circular swatches matching plateui.View)
	if len(legend) > 0 {
		ly := padY + headerH + gridBlockH + 20
		var totalRowW float32
		if legendRows == 1 {
			for _, item := range legend {
				lbl := fmt.Sprintf("%s %d", item.Label, item.Count)
				totalRowW += float32(16 + len([]rune(lbl))*10 + 16)
			}
		}
		startX := padX
		if legendRows == 1 && totalRowW > 0 && totalRowW < cardW-padX*2 {
			startX = (cardW - totalRowW + 16) / 2
		}
		lx := startX
		for _, item := range legend {
			lbl := fmt.Sprintf("%s %d", item.Label, item.Count)
			itemW := float32(16 + len([]rune(lbl))*10 + 16)
			if lx > startX && lx+itemW > cardW-padX {
				lx = padX
				ly += 24
			}
			fmt.Fprintf(&b, `  <circle cx="%.1f" cy="%.1f" r="4.5" fill="%s"/>`,
				lx+4.5, ly, svgColor(item.Color))
			b.WriteByte('\n')
			fmt.Fprintf(&b, `  <text x="%.1f" y="%.1f" dominant-baseline="central" font-family="Microsoft YaHei, PingFang SC, Inter, sans-serif" font-size="11.5" fill="%s">%s</text>`,
				lx+14, ly, svgColor(t.TextMuted), html.EscapeString(lbl))
			b.WriteByte('\n')
			lx += itemW
		}
	}

	b.WriteString("</svg>\n")
	return b.String()
}

// RenderPNG renders a 2x Retina PNG image of the plate card using an isolated
// offscreen mygo/ui software rasterizer so exported PNGs match on-screen CJK glyphs.
func RenderPNG(props Props, title string, dark bool) ([]byte, error) {
	rows, cols := ResolveDimensions(props)
	const exportZoom = float32(1.1)
	cellW, cellH, gap := AdaptiveMetrics(rows, cols, props.Topology, exportZoom)
	leftLabel := float32(24)
	topLabel := float32(16)
	dishPad := float32(0)
	if props.Topology == "dish" {
		dishPad = 48
	}
	gridW := float32(cols)*cellW + float32(cols-1)*gap
	gridH := float32(rows)*cellH + float32(rows-1)*gap
	cardW := int(math.Ceil(float64(leftLabel + gridW + dishPad*2 + 72)))
	if cardW < 540 {
		cardW = 540
	}
	legend := ComputeLegend(props, dark)
	legendExtra := 0
	if len(legend) > 0 {
		legendExtra = 36 + ((len(legend)*120)/cardW)*22
	}
	cardH := int(math.Ceil(float64(topLabel + gridH + dishPad*2 + 124 + float32(legendExtra))))
	if cardH < 300 {
		cardH = 300
	}

	titleText := strings.TrimSpace(title)
	if titleText == "" {
		titleText = containerFormatSummary(rows, cols, props.Topology)
	}
	subText := fmt.Sprintf("%s · 已分配 %d / %d 孔", containerFormatSummary(rows, cols, props.Topology), len(props.Wells), rows*cols)

	cleanProps := props
	cleanProps.Interactive = false
	cleanProps.HideFooter = true
	cleanProps.SelectedIDs = nil
	cleanProps.OnSelect = nil
	cleanProps.Zoom = exportZoom
	cleanProps.State = nil

	tt := ui.NewTesterWith(func(c *ui.Context) {
		if dark {
			c.SetTheme(theme.DarkTheme())
		} else {
			c.SetTheme(theme.LightTheme())
		}
		tok := theme.GetTokens(c.Theme())
		ui.Box(c).Fill().Background(tok.Canvas).Padding(float32(tok.Spacing12)).Children(func() {
			ui.Column(c).Fill().Padding(float32(tok.Spacing16)).Radius(float32(tok.PanelRadius)).
				Background(tok.Surface).Border(1, tok.Border).Gap(float32(tok.Spacing8)).Children(func() {
				ui.Column(c).Gap(2).Children(func() {
					ui.Text(c, titleText).FontSize(float32(tok.TextSection)).Bold().TextColor(tok.Text)
					ui.Text(c, subText).FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
				})
				ui.Box(c).FillWidth().AlignItems(ui.Center).Children(func() {
					View(c, cleanProps)
				})
			})
		})
	}, cardW, cardH, ui.TesterOptions{Dark: dark})
	tt.SetScale(2.0)
	tt.Frame()

	img := tt.Image()
	if img == nil {
		return nil, fmt.Errorf("导出孔板 PNG 失败：无法生成图像")
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, fmt.Errorf("编码孔板 PNG 失败：%w", err)
	}
	return buf.Bytes(), nil
}
