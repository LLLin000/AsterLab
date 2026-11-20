package asterdoc

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"math"
	"sort"

	"asterlab-mygo-prototype/internal/theme"
	plateui "asterlab-mygo-prototype/internal/uikit/plate"
)

// RenderPlatePNG renders a crisp 2D plate diagram (wells colored by active group or factor layer)
// into PNG bytes using pure Go stdlib so plate PNG export works identically across all platforms.
func RenderPlatePNG(p *Plate, g Geometry, doc *Document, colorBy, factorKey string, dark bool) ([]byte, error) {
	if p == nil || g.Rows <= 0 || g.Columns <= 0 {
		return nil, nil
	}
	cell := 44
	gap := 8
	if g.Columns > 12 {
		cell = 26
		gap = 5
	}
	padX := 32
	padY := 32
	w := padX*2 + g.Columns*cell + (g.Columns-1)*gap
	h := padY*2 + g.Rows*cell + (g.Rows-1)*gap
	img := image.NewRGBA(image.Rect(0, 0, w, h))

	tok := theme.LightTokens()
	if dark {
		tok = theme.DarkTokens()
	}
	bg := color.RGBA{
		R: uint8(math.Round(float64(tok.Canvas.R) * 255)),
		G: uint8(math.Round(float64(tok.Canvas.G) * 255)),
		B: uint8(math.Round(float64(tok.Canvas.B) * 255)),
		A: 255,
	}
	borderCol := color.RGBA{
		R: uint8(math.Round(float64(tok.BorderStrong.R) * 255)),
		G: uint8(math.Round(float64(tok.BorderStrong.G) * 255)),
		B: uint8(math.Round(float64(tok.BorderStrong.B) * 255)),
		A: 255,
	}
	emptyFill := color.RGBA{
		R: uint8(math.Round(float64(tok.Elevated.R) * 255)),
		G: uint8(math.Round(float64(tok.Elevated.G) * 255)),
		B: uint8(math.Round(float64(tok.Elevated.B) * 255)),
		A: 255,
	}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetRGBA(x, y, bg)
		}
	}

	wellGroups := p.WellGroups()
	idxMap := plateGroupPaletteMap(doc, p)
	excluded := map[string]bool{}
	for _, ex := range p.ExcludedWells {
		excluded[ex] = true
	}

	// Deterministic sequential factor color index for non-group layers
	factorVals := []string{}
	if colorBy == "factor" && factorKey != "" {
		seen := map[string]bool{}
		for r := 0; r < g.Rows; r++ {
			for col := 0; col < g.Columns; col++ {
				id := plateWellIDFor(g, p.Topology, r, col)
				if fv := p.WellFactors(id)[factorKey]; fv != "" && !seen[fv] {
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

	for r := 0; r < g.Rows; r++ {
		for col := 0; col < g.Columns; col++ {
			id := plateWellIDFor(g, p.Topology, r, col)
			cx := padX + col*(cell+gap) + cell/2
			cy := padY + r*(cell+gap) + cell/2
			rad := float64(cell) / 2.0

			fill := emptyFill
			hasFill := false
			if colorBy == "factor" && factorKey != "" {
				if fv := p.WellFactors(id)[factorKey]; fv != "" {
					uc := palette[factorIdx[fv]%len(palette)]
					fill = color.RGBA{
						R: uint8(math.Round(float64(uc.R) * 255)),
						G: uint8(math.Round(float64(uc.G) * 255)),
						B: uint8(math.Round(float64(uc.B) * 255)),
						A: 255,
					}
					hasFill = true
				}
			} else if grp := wellGroups[id]; grp != "" {
				uc := plateui.GroupColorIndexed(grp, idxMap, dark)
				fill = color.RGBA{
					R: uint8(math.Round(float64(uc.R) * 255)),
					G: uint8(math.Round(float64(uc.G) * 255)),
					B: uint8(math.Round(float64(uc.B) * 255)),
					A: 255,
				}
				hasFill = true
			}
			stroke := borderCol
			if hasFill {
				stroke = fill
			}
			drawFilledCircle(img, cx, cy, rad, fill, stroke)
			if excluded[id] {
				drawCross(img, cx, cy, int(rad*0.55), color.RGBA{R: 220, G: 53, B: 69, A: 255})
			}
		}
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func plateWellIDFor(g Geometry, topology string, row, col int) string {
	if topology == "rack_1d" {
		return formatInt(row*g.Columns + col + 1)
	}
	if row >= 0 && row < 26 {
		return string(rune('A'+row)) + formatInt(col+1)
	}
	return formatInt(row+1) + "-" + formatInt(col+1)
}

func formatInt(n int) string {
	if n == 0 {
		return "0"
	}
	var digits [16]byte
	i := len(digits)
	for n > 0 {
		i--
		digits[i] = byte('0' + n%10)
		n /= 10
	}
	return string(digits[i:])
}

func drawFilledCircle(img *image.RGBA, cx, cy int, r float64, fill, stroke color.RGBA) {
	rOut := r
	rIn := r - 1.5
	minX := int(math.Floor(float64(cx) - r - 1))
	maxX := int(math.Ceil(float64(cx) + r + 1))
	minY := int(math.Floor(float64(cy) - r - 1))
	maxY := int(math.Ceil(float64(cy) + r + 1))
	for y := minY; y <= maxY; y++ {
		for x := minX; x <= maxX; x++ {
			dx := float64(x - cx)
			dy := float64(y - cy)
			d := math.Hypot(dx, dy)
			if d <= rIn {
				img.SetRGBA(x, y, fill)
			} else if d <= rOut {
				img.SetRGBA(x, y, stroke)
			}
		}
	}
}

func drawCross(img *image.RGBA, cx, cy, half int, col color.RGBA) {
	for d := -half; d <= half; d++ {
		for t := -1; t <= 1; t++ {
			img.SetRGBA(cx+d, cy+d+t, col)
			img.SetRGBA(cx+d, cy-d+t, col)
		}
	}
}
