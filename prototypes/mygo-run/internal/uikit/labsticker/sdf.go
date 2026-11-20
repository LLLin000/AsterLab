// Package labsticker renders BioRender-style laboratory vessels as crisp
// die-cut vinyl stickers with opaque white borders, subtle shelf shadows, and
// proportional shelf-rack layouts.
package labsticker

import (
	"image"
	"image/color"
	"math"
	"sync"

	"github.com/egoist/mygo/ui"
)

// StickerOptions configures die-cut vinyl sticker rasterization.
type StickerOptions struct {
	// TargetHeightDIP is the unpadded height of the vessel artwork itself
	// (the white border & shelf shadow add predictable padding around it).
	TargetHeightDIP float32
	// BorderDIP controls the width of the die-cut white vinyl outline.
	// Defaults to 4.0 DIP when <= 0.
	BorderDIP float32
	// Exhausted applies subtle grayscale & reduced opacity to the vessel art
	// while preserving the crisp white die-cut sticker silhouette.
	Exhausted bool
}

// StickerMetrics records the final DIP dimensions and padding of a rendered sticker.
type StickerMetrics struct {
	WidthDIP      float32
	HeightDIP     float32
	ArtWidthDIP   float32
	ArtHeightDIP  float32
	TopPadDIP     float32
	BottomPadDIP  float32
	LeftPadDIP    float32
	RightPadDIP   float32
}

type cachedSticker struct {
	bmp     *ui.Bitmap
	metrics StickerMetrics
}

var (
	stickerCacheMu sync.Mutex
	stickerCache   = map[stickerCacheKey]cachedSticker{}
	tightBoundsMu  sync.Mutex
	tightBoundsMap = map[*ui.SVG]normBounds{}
)

type stickerCacheKey struct {
	id        VesselID
	customPtr image.Image
	heightPx  int
	borderPx  int
	exhausted bool
}

type normBounds struct {
	u0, v0, u1, v1 float64
	aspect         float64 // tightW / tightH
}

const (
	superSample    = 2
	defaultBorder  = 4.0
	shadowOffsetY  = 2.2
	shadowBlurDIP  = 4.0
	probeDim       = 440
	infDistBarrier = 1e9
)

// measureSVGTightBounds probes a parsed SVG once to find the tight non-transparent
// fraction inside its viewBox aspect square so all vessels align flush to shelves.
func measureSVGTightBounds(s *ui.SVG) normBounds {
	if s == nil {
		return normBounds{0, 0, 1, 1, 1}
	}
	tightBoundsMu.Lock()
	if nb, ok := tightBoundsMap[s]; ok {
		tightBoundsMu.Unlock()
		return nb
	}
	tightBoundsMu.Unlock()

	sw, sh := s.Size()
	if sw <= 0 || sh <= 0 {
		return normBounds{0, 0, 1, 1, 1}
	}
	pw := probeDim
	ph := int(math.Round(float64(probeDim) * float64(sh) / float64(sw)))
	if ph < 32 {
		ph = 32
	}
	probe := s.Image(pw, ph, nil)
	x0, y0, x1, y1, ok := alphaBoundingBox(probe, 10)
	if !ok {
		return normBounds{0, 0, 1, 1, float64(sw) / float64(sh)}
	}
	cw := float64(x1 - x0)
	ch := float64(y1 - y0)
	nb := normBounds{
		u0:     float64(x0) / float64(pw),
		v0:     float64(y0) / float64(ph),
		u1:     float64(x1) / float64(pw),
		v1:     float64(y1) / float64(ph),
		aspect: cw / maxFloat(ch, 1),
	}
	tightBoundsMu.Lock()
	tightBoundsMap[s] = nb
	tightBoundsMu.Unlock()
	return nb
}

func maxFloat(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func alphaBoundingBox(img image.Image, minA uint8) (x0, y0, x1, y1 int, ok bool) {
	b := img.Bounds()
	minX, minY := b.Max.X, b.Max.Y
	maxX, maxY := b.Min.X, b.Min.Y
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			_, _, _, a := img.At(x, y).RGBA()
			if uint8(a>>8) >= minA {
				if x < minX {
					minX = x
				}
				if y < minY {
					minY = y
				}
				if x+1 > maxX {
					maxX = x + 1
				}
				if y+1 > maxY {
					maxY = y + 1
				}
				ok = true
			}
		}
	}
	return minX, minY, maxX, maxY, ok
}

// composeSVGSticker vector-rasterizes an SVG directly at crisp 2x target scale,
// tight-crops its artboard margins, flood-fills exterior air so transparent glass
// gains an opaque white vinyl backing, applies 2D EDT to draw a smooth die-cut
// white border + subtle outer rim + shelf shadow, and 2x2-downsamples to DIPs.
func composeSVGSticker(s *ui.SVG, heightDIP, borderDIP float32, exhausted bool) (*ui.Bitmap, StickerMetrics) {
	nb := measureSVGTightBounds(s)
	innerH := max(16, int(math.Round(float64(heightDIP))))
	workH := innerH * superSample

	// Render full SVG canvas directly at target scale so there is zero raster stretch blur.
	fracH := maxFloat(nb.v1-nb.v0, 0.15)
	sw, sh := s.Size()
	canvasH := int(math.Round(float64(workH) / fracH))
	if canvasH < workH {
		canvasH = workH
	}
	if canvasH > 1200 {
		canvasH = 1200
	}
	canvasW := int(math.Round(float64(canvasH) * float64(sw) / maxFloat(float64(sh), 1)))
	if canvasW < 32 {
		canvasW = 32
	}

	raw := s.Image(canvasW, canvasH, nil)
	rx0, ry0, rx1, ry1, ok := alphaBoundingBox(raw, 8)
	if !ok {
		return nil, StickerMetrics{}
	}
	cw := rx1 - rx0
	ch := ry1 - ry0

	// Compute work dimensions matching exact rasterized aspect to prevent distortion.
	exactW := cw
	exactH := ch
	if ch != workH && ch > 0 {
		exactW = max(12, int(math.Round(float64(cw)*float64(workH)/float64(ch))))
		exactH = workH
	}

	bDIP := float64(borderDIP)
	if bDIP <= 0 {
		bDIP = defaultBorder
	}
	padSideDIP := int(math.Ceil(bDIP + shadowBlurDIP + 1.5))
	padBotDIP := int(math.Ceil(bDIP + 2.5))
	padTopDIP := int(math.Ceil(bDIP + 2.5))

	outW := (exactW+superSample-1)/superSample + padSideDIP*2
	outH := (exactH+superSample-1)/superSample + padTopDIP + padBotDIP
	ssW := outW * superSample
	ssH := outH * superSample
	offX := padSideDIP * superSample
	offY := padTopDIP * superSample

	artBuf := image.NewRGBA(image.Rect(0, 0, ssW, ssH))
	if cw == exactW && ch == exactH {
		for y := 0; y < ch; y++ {
			srcOff := raw.PixOffset(rx0, ry0+y)
			dstOff := artBuf.PixOffset(offX, offY+y)
			copy(artBuf.Pix[dstOff:dstOff+cw*4], raw.Pix[srcOff:srcOff+cw*4])
		}
	} else {
		resampleRGBA(raw, image.Rect(rx0, ry0, rx1, ry1), artBuf, image.Rect(offX, offY, offX+exactW, offY+exactH))
	}

	finalImg := compositeDieCutFromArt(artBuf, ssW, ssH, outW, outH, bDIP, exhausted)
	bmp := ui.NewBitmap(finalImg)
	m := StickerMetrics{
		WidthDIP:     float32(outW),
		HeightDIP:    float32(outH),
		ArtWidthDIP:  float32(exactW) / superSample,
		ArtHeightDIP: float32(exactH) / superSample,
		TopPadDIP:    float32(padTopDIP),
		BottomPadDIP: float32(padBotDIP),
		LeftPadDIP:   float32(padSideDIP),
		RightPadDIP:  float32(padSideDIP),
	}
	return bmp, m
}

// StickerFromImage converts any arbitrary user-imported PNG/RGBA illustration
// into a matching white-bordered BioRender-style die-cut shelf sticker.
func StickerFromImage(src image.Image, opts StickerOptions) (*ui.Bitmap, StickerMetrics) {
	if src == nil {
		return nil, StickerMetrics{}
	}
	hDIP := opts.TargetHeightDIP
	if hDIP <= 0 {
		hDIP = 80
	}
	bDIP := opts.BorderDIP
	if bDIP <= 0 {
		bDIP = defaultBorder
	}
	hPx := max(16, int(math.Round(float64(hDIP))))
	bPx := max(1, int(math.Round(float64(bDIP)*10)))

	stickerCacheMu.Lock()
	key := stickerCacheKey{customPtr: src, heightPx: hPx, borderPx: bPx, exhausted: opts.Exhausted}
	if hit, ok := stickerCache[key]; ok {
		stickerCacheMu.Unlock()
		return hit.bmp, hit.metrics
	}
	stickerCacheMu.Unlock()

	x0, y0, x1, y1, ok := alphaBoundingBox(src, 12)
	if !ok {
		return nil, StickerMetrics{}
	}
	cw := x1 - x0
	ch := y1 - y0
	aspect := float64(cw) / maxFloat(float64(ch), 1)
	workH := hPx * superSample
	workW := max(12, int(math.Round(float64(workH)*aspect)))

	padSide := int(math.Ceil(float64(bDIP) + shadowBlurDIP + 1.5))
	padBot := int(math.Ceil(float64(bDIP) + shadowOffsetY + shadowBlurDIP + 1.5))
	padTop := int(math.Ceil(float64(bDIP) + 2.5))
	outW := (workW+superSample-1)/superSample + padSide*2
	outH := (workH+superSample-1)/superSample + padTop + padBot
	ssW := outW * superSample
	ssH := outH * superSample
	offX := padSide * superSample
	offY := padTop * superSample

	artBuf := image.NewRGBA(image.Rect(0, 0, ssW, ssH))
	resampleRGBA(src, image.Rect(x0, y0, x1, y1), artBuf, image.Rect(offX, offY, offX+workW, offY+workH))

	finalImg := compositeDieCutFromArt(artBuf, ssW, ssH, outW, outH, float64(bDIP), opts.Exhausted)
	bmp := ui.NewBitmap(finalImg)
	m := StickerMetrics{
		WidthDIP:     float32(outW),
		HeightDIP:    float32(outH),
		ArtWidthDIP:  float32(workW) / superSample,
		ArtHeightDIP: float32(workH) / superSample,
		TopPadDIP:    float32(padTop),
		BottomPadDIP: float32(padBot),
		LeftPadDIP:   float32(padSide),
		RightPadDIP:  float32(padSide),
	}
	stickerCacheMu.Lock()
	stickerCache[key] = cachedSticker{bmp: bmp, metrics: m}
	stickerCacheMu.Unlock()
	return bmp, m
}

func compositeDieCutFromArt(artBuf *image.RGBA, ssW, ssH, outW, outH int, borderDIP float64, exhausted bool) *image.RGBA {
	n := ssW * ssH
	interior := computeClosedSilhouette(artBuf, ssW, ssH)

	// Exact 2D Euclidean Distance Transform from the closed interior silhouette.
	distSq := make([]float64, n)
	hasInside := false
	for i := 0; i < n; i++ {
		if interior[i] {
			distSq[i] = 0
			hasInside = true
		} else {
			distSq[i] = infDistBarrier
		}
	}
	if hasInside {
		edt2D(distSq, ssW, ssH)
	}

	bSS := borderDIP * superSample
	shOffY := shadowOffsetY * superSample
	shBlur := shadowBlurDIP * superSample
	fadeTop := int(math.Round(float64(ssH) * 0.28))

	ssOut := image.NewRGBA(image.Rect(0, 0, ssW, ssH))
	for y := 0; y < ssH; y++ {
		for x := 0; x < ssW; x++ {
			idx := y*ssW + x
			d := math.Sqrt(distSq[idx])

			// Soft downward drop shadow behind the die-cut sticker (anchored toward shelf base)
			var shA float64
			sy := int(math.Round(float64(y) - shOffY))
			if sy >= 0 && sy < ssH && y > fadeTop {
				sd := math.Sqrt(distSq[sy*ssW+x])
				if sd > bSS-0.6 && sd < bSS+shBlur {
					t := 1.0 - (sd-(bSS-0.6))/shBlur
					if t > 0 {
						shA = 0.14 * t * t
					}
				}
			}

			// Start with shadow in premultiplied slate (15, 23, 42)
			r := 15.0 * shA
			g := 23.0 * shA
			b := 42.0 * shA
			a := shA

			// Subtle 1px outer cutline rim around white vinyl edge (slate #94A3B8)
			rimCov := 0.0
			if d >= bSS-0.2 && d <= bSS+1.5 {
				rimCov = (1.0 - math.Abs(d-(bSS+0.55))/1.05)
				if rimCov < 0 {
					rimCov = 0
				}
				rimCov *= 0.36
			}
			if rimCov > 0 {
				inv := 1.0 - rimCov
				r = 148.0*rimCov + r*inv
				g = 163.0*rimCov + g*inv
				b = 184.0*rimCov + b*inv
				a = rimCov + a*inv
			}

			// Anti-aliased solid #FFFFFF die-cut vinyl sticker body
			whiteCov := smoothstep(bSS+0.95, bSS-0.85, d)
			if whiteCov > 0 {
				inv := 1.0 - whiteCov
				r = 255.0*whiteCov + r*inv
				g = 255.0*whiteCov + g*inv
				b = 255.0*whiteCov + b*inv
				a = whiteCov + a*inv
			}

			// Vessel artwork Porter-Duff OVER opaque white sticker body
			p := y*artBuf.Stride + x*4
			ar := float64(artBuf.Pix[p])
			ag := float64(artBuf.Pix[p+1])
			ab := float64(artBuf.Pix[p+2])
			aa := float64(artBuf.Pix[p+3]) / 255.0
			if aa > 0 {
				if exhausted {
					lum := 0.299*ar + 0.587*ag + 0.114*ab
					ar = (ar*0.28 + lum*0.72) * 0.62
					ag = (ag*0.28 + lum*0.72) * 0.62
					ab = (ab*0.28 + lum*0.72) * 0.62
					aa *= 0.62
				}
				invA := 1.0 - aa
				r = ar + r*invA
				g = ag + g*invA
				b = ab + b*invA
				a = aa + a*invA
			}

			op := y*ssOut.Stride + x*4
			ssOut.Pix[op] = clampByte(r)
			ssOut.Pix[op+1] = clampByte(g)
			ssOut.Pix[op+2] = clampByte(b)
			ssOut.Pix[op+3] = clampByte(a * 255.0)
		}
	}

	if superSample == 1 {
		return ssOut
	}
	return downsample2x(ssOut, outW, outH)
}

// computeClosedSilhouette marks pixels as interior if they belong to the vessel
// artwork (alpha >= 26) OR are enclosed transparent glass cavities unreachable
// from the image border.
func computeClosedSilhouette(art *image.RGBA, w, h int) []bool {
	n := w * h
	solid := make([]bool, n)
	for y := 0; y < h; y++ {
		row := y * art.Stride
		for x := 0; x < w; x++ {
			if art.Pix[row+x*4+3] >= 26 {
				solid[y*w+x] = true
			}
		}
	}

	exterior := make([]bool, n)
	queue := make([]int, 0, (w+h)*2)
	push := func(idx int) {
		if !solid[idx] && !exterior[idx] {
			exterior[idx] = true
			queue = append(queue, idx)
		}
	}
	for x := 0; x < w; x++ {
		push(x)
		push((h-1)*w + x)
	}
	for y := 0; y < h; y++ {
		push(y * w)
		push(y*w + w - 1)
	}
	for head := 0; head < len(queue); head++ {
		cur := queue[head]
		cx := cur % w
		cy := cur / w
		if cx > 0 {
			push(cur - 1)
		}
		if cx+1 < w {
			push(cur + 1)
		}
		if cy > 0 {
			push(cur - w)
		}
		if cy+1 < h {
			push(cur + w)
		}
	}

	interior := make([]bool, n)
	for i := 0; i < n; i++ {
		interior[i] = !exterior[i]
	}
	return interior
}

func edt2D(d []float64, w, h int) {
	colIn := make([]float64, max(w, h))
	colOut := make([]float64, max(w, h))
	v := make([]int, max(w, h))
	z := make([]float64, max(w, h)+1)

	for x := 0; x < w; x++ {
		for y := 0; y < h; y++ {
			colIn[y] = d[y*w+x]
		}
		edt1D(colIn[:h], colOut[:h], h, v[:h], z[:h+1])
		for y := 0; y < h; y++ {
			d[y*w+x] = colOut[y]
		}
	}
	for y := 0; y < h; y++ {
		row := y * w
		edt1D(d[row:row+w], colOut[:w], w, v[:w], z[:w+1])
		copy(d[row:row+w], colOut[:w])
	}
}

// edt1D implements the Felzenszwalb-Huttenlocher exact O(n) lower-envelope EDT.
func edt1D(f, out []float64, n int, v []int, z []float64) {
	if n == 0 {
		return
	}
	const bigInf = 1e18
	k := 0
	v[0] = 0
	z[0] = -bigInf
	z[1] = bigInf
	for q := 1; q < n; q++ {
		fq := f[q] + float64(q*q)
		var s float64
		for {
			vk := v[k]
			s = (fq - (f[vk] + float64(vk*vk))) / float64(2*(q-vk))
			if s <= z[k] && k > 0 {
				k--
				continue
			}
			break
		}
		k++
		v[k] = q
		z[k] = s
		z[k+1] = bigInf
	}
	k = 0
	for q := 0; q < n; q++ {
		for z[k+1] < float64(q) {
			k++
		}
		dx := float64(q - v[k])
		out[q] = dx*dx + f[v[k]]
	}
}

func resampleRGBA(src image.Image, sr image.Rectangle, dst *image.RGBA, dr image.Rectangle) {
	sw := sr.Dx()
	sh := sr.Dy()
	dw := dr.Dx()
	dh := dr.Dy()
	if sw <= 0 || sh <= 0 || dw <= 0 || dh <= 0 {
		return
	}
	for dy := 0; dy < dh; dy++ {
		syF := (float64(dy)+0.5)*float64(sh)/float64(dh) - 0.5
		y0 := int(math.Floor(syF))
		wy := syF - float64(y0)
		if y0 < 0 {
			y0, wy = 0, 0
		}
		y1 := y0 + 1
		if y1 >= sh {
			y1 = sh - 1
		}
		for dx := 0; dx < dw; dx++ {
			sxF := (float64(dx)+0.5)*float64(sw)/float64(dw) - 0.5
			x0 := int(math.Floor(sxF))
			wx := sxF - float64(x0)
			if x0 < 0 {
				x0, wx = 0, 0
			}
			x1 := x0 + 1
			if x1 >= sw {
				x1 = sw - 1
			}
			c00 := rgbaPremulAt(src, sr.Min.X+x0, sr.Min.Y+y0)
			c10 := rgbaPremulAt(src, sr.Min.X+x1, sr.Min.Y+y0)
			c01 := rgbaPremulAt(src, sr.Min.X+x0, sr.Min.Y+y1)
			c11 := rgbaPremulAt(src, sr.Min.X+x1, sr.Min.Y+y1)
			w00 := (1 - wx) * (1 - wy)
			w10 := wx * (1 - wy)
			w01 := (1 - wx) * wy
			w11 := wx * wy
			p := (dr.Min.Y+dy)*dst.Stride + (dr.Min.X+dx)*4
			dst.Pix[p] = clampByte(float64(c00.R)*w00 + float64(c10.R)*w10 + float64(c01.R)*w01 + float64(c11.R)*w11)
			dst.Pix[p+1] = clampByte(float64(c00.G)*w00 + float64(c10.G)*w10 + float64(c01.G)*w01 + float64(c11.G)*w11)
			dst.Pix[p+2] = clampByte(float64(c00.B)*w00 + float64(c10.B)*w10 + float64(c01.B)*w01 + float64(c11.B)*w11)
			dst.Pix[p+3] = clampByte(float64(c00.A)*w00 + float64(c10.A)*w10 + float64(c01.A)*w01 + float64(c11.A)*w11)
		}
	}
}

func rgbaPremulAt(img image.Image, x, y int) color.RGBA {
	if rgba, ok := img.(*image.RGBA); ok {
		return rgba.RGBAAt(x, y)
	}
	r, g, b, a := img.At(x, y).RGBA()
	return color.RGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8), A: uint8(a >> 8)}
}

func downsample2x(ss *image.RGBA, outW, outH int) *image.RGBA {
	out := image.NewRGBA(image.Rect(0, 0, outW, outH))
	for y := 0; y < outH; y++ {
		sy := y * 2
		for x := 0; x < outW; x++ {
			sx := x * 2
			var sr, sg, sb, sa uint32
			for dy := 0; dy < 2; dy++ {
				yy := min(sy+dy, ss.Rect.Max.Y-1)
				for dx := 0; dx < 2; dx++ {
					xx := min(sx+dx, ss.Rect.Max.X-1)
					p := yy*ss.Stride + xx*4
					sr += uint32(ss.Pix[p])
					sg += uint32(ss.Pix[p+1])
					sb += uint32(ss.Pix[p+2])
					sa += uint32(ss.Pix[p+3])
				}
			}
			dp := y*out.Stride + x*4
			out.Pix[dp] = uint8((sr + 2) >> 2)
			out.Pix[dp+1] = uint8((sg + 2) >> 2)
			out.Pix[dp+2] = uint8((sb + 2) >> 2)
			out.Pix[dp+3] = uint8((sa + 2) >> 2)
		}
	}
	return out
}

func smoothstep(edge0, edge1, x float64) float64 {
	if edge0 == edge1 {
		if x >= edge0 {
			return 1
		}
		return 0
	}
	t := (x - edge0) / (edge1 - edge0)
	if t <= 0 {
		return 0
	}
	if t >= 1 {
		return 1
	}
	return t * t * (3 - 2*t)
}

func clampByte(v float64) uint8 {
	if v <= 0 {
		return 0
	}
	if v >= 255 {
		return 255
	}
	return uint8(math.Round(v))
}
