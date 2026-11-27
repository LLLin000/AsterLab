package theme

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"sync"

	"github.com/egoist/mygo/ui"
)

const (
	// Small-size optical variant (16px-32px): thinned strokes & widened channels.
	logoSmallPath1 = "M 0 528.588 L 127.468 528.588 C 170.4 528.588 209.641 504.316 228.814 465.904 L 359.678 203.722 C 412.544 97.807 513.118 23.776 629.959 4.774 C 746.8 -14.228 865.65 24.117 949.355 107.821 C 997.149 155.615 1024 220.439 1024 288.031 L 1024 896.711 L 939.048 896.711 L 939.048 288.031 C 939.048 242.969 921.148 199.754 889.285 167.891 C 824.897 103.503 733.474 74.007 643.596 88.624 C 553.719 103.241 476.354 160.187 435.688 241.661 L 304.823 503.843 C 271.271 571.064 202.598 613.539 127.468 613.539 L 0 613.539 L 0 528.588 Z"
	logoSmallPath2 = "M 0 670.174 L 127.468 670.174 C 224.064 670.174 312.357 615.563 355.496 529.136 L 486.36 266.953 C 518.893 201.775 580.785 156.217 652.687 144.524 C 724.589 132.83 797.728 156.427 849.238 207.937 C 870.48 229.179 882.414 257.99 882.414 288.031 L 882.414 896.711 L 797.462 896.711 L 797.462 288.031 C 797.462 280.52 794.479 273.318 789.168 268.007 C 756.974 235.813 711.263 221.065 666.324 228.374 C 621.385 235.682 582.703 264.156 562.37 304.892 L 431.506 567.074 C 373.987 682.311 256.262 755.125 127.468 755.125 L 0 755.125 L 0 670.174 Z"
	logoSmallPath3 = "M 0 811.76 L 127.468 811.76 C 277.728 811.76 415.073 726.81 482.179 592.367 L 559.389 437.678 C 586.62 383.121 652.154 359.912 707.652 385.168 C 727.858 394.364 740.828 414.516 740.828 436.716 L 740.828 896.711 L 655.876 896.711 L 655.876 434.59 L 558.188 630.306 C 476.703 793.558 309.926 896.711 127.468 896.711 L 0 896.711 L 0 811.76 Z"

	// Standard display variant (>=48px, print, About card, large icon frames).
	logoStdPath1 = "M 0 481.392 L 151.025 481.392 C 193.956 481.392 233.198 457.121 252.371 418.709 L 359.678 203.722 C 412.544 97.807 513.118 23.776 629.959 4.774 C 746.8 -14.228 865.65 24.117 949.355 107.821 C 997.149 155.615 1024 220.439 1024 288.031 L 1024 896.711 L 910.731 896.711 L 910.731 375.105 C 910.731 311.645 884.113 251.094 837.355 208.189 C 790.597 165.284 727.986 143.959 664.76 149.403 C 566.506 157.863 479.73 216.831 435.688 305.068 L 330.251 516.307 C 306.285 564.322 257.233 594.661 203.569 594.661 L 0 594.661 L 0 481.392 Z"
	logoStdPath2 = "M 0 632.418 L 226.921 632.418 C 280.586 632.418 329.638 602.078 353.604 554.063 L 461.024 338.85 C 493.871 273.043 553.546 224.634 624.713 206.067 C 684.12 190.567 747.356 203.47 795.94 241.006 C 844.525 278.541 872.975 336.471 872.975 397.866 L 872.975 896.711 L 759.706 896.711 L 759.706 474.637 C 759.706 422.09 723.568 376.444 672.423 364.389 C 621.278 352.334 568.558 377.037 545.091 424.052 L 431.484 651.661 C 402.725 709.279 343.862 745.686 279.465 745.686 L 0 745.686 L 0 632.418 Z"
	logoStdPath3 = "M 0 783.443 L 302.818 783.443 C 367.215 783.443 426.077 747.036 454.837 689.417 L 559.389 479.95 C 580.046 438.564 623.792 413.896 669.896 417.637 C 699.302 420.023 721.95 444.584 721.95 474.086 L 721.95 896.711 L 608.681 896.711 L 608.681 634.823 L 540.539 771.344 C 502.193 848.169 423.71 896.711 337.847 896.711 L 0 896.711 L 0 783.443 Z"
)

// LogoOutlineSVG is the geometric construction wireframe stripped of background paper
// for clean transparent Light/Dark watermark overlays (Settings About & Docs Hero).
var LogoOutlineSVG = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="-16 -24 1056 944" overflow="visible">` +
	`<path d="` + logoStdPath1 + ` ` + logoStdPath2 + ` ` + logoStdPath3 + `" fill="none" stroke="currentColor" stroke-width="2.4" stroke-linejoin="round" stroke-opacity="0.55"/>` +
	`<g stroke="currentColor" stroke-width="1.5" stroke-opacity="0.35" fill="none">` +
	`<line x1="151.03" y1="481.39" x2="193.96" y2="481.39"/><circle cx="193.96" cy="481.39" r="5.27"/>` +
	`<line x1="252.37" y1="418.71" x2="233.2" y2="457.12"/><circle cx="233.2" cy="457.12" r="5.27"/>` +
	`<line x1="359.68" y1="203.72" x2="412.54" y2="97.81"/><circle cx="412.54" cy="97.81" r="5.27"/>` +
	`<line x1="629.96" y1="4.77" x2="513.12" y2="23.78"/><circle cx="513.12" cy="23.78" r="5.27"/>` +
	`<line x1="629.96" y1="4.77" x2="746.8" y2="-14.23"/><circle cx="746.8" cy="-14.23" r="5.27"/>` +
	`<line x1="949.36" y1="107.82" x2="865.65" y2="24.12"/><circle cx="865.65" cy="24.12" r="5.27"/>` +
	`<line x1="837.36" y1="208.19" x2="884.11" y2="251.09"/><circle cx="884.11" cy="251.09" r="5.27"/>` +
	`<line x1="664.76" y1="149.4" x2="727.99" y2="143.96"/><circle cx="727.99" cy="143.96" r="5.27"/>` +
	`<line x1="435.69" y1="305.07" x2="479.73" y2="216.83"/><circle cx="479.73" cy="216.83" r="5.27"/>` +
	`<line x1="226.92" y1="632.42" x2="280.59" y2="632.42"/><circle cx="280.59" cy="632.42" r="5.27"/>` +
	`<line x1="461.02" y1="338.85" x2="493.87" y2="273.04"/><circle cx="493.87" cy="273.04" r="5.27"/>` +
	`<line x1="624.71" y1="206.07" x2="553.55" y2="224.63"/><circle cx="553.55" cy="224.63" r="5.27"/>` +
	`<line x1="795.94" y1="241.01" x2="747.36" y2="203.47"/><circle cx="747.36" cy="203.47" r="5.27"/>` +
	`<line x1="672.42" y1="364.39" x2="723.57" y2="376.44"/><circle cx="723.57" cy="376.44" r="5.27"/>` +
	`<line x1="431.48" y1="651.66" x2="402.73" y2="709.28"/><circle cx="402.73" cy="709.28" r="5.27"/>` +
	`<line x1="559.39" y1="479.95" x2="580.05" y2="438.56"/><circle cx="580.05" cy="438.56" r="5.27"/>` +
	`<line x1="540.54" y1="771.34" x2="502.19" y2="848.17"/><circle cx="502.19" cy="848.17" r="5.27"/>` +
	`</g>` +
	`<g fill="currentColor" fill-opacity="0.55">` +
	`<rect x="-6.27" y="475.12" width="12.55" height="12.55"/>` +
	`<rect x="144.75" y="475.12" width="12.55" height="12.55"/>` +
	`<rect x="353.4" y="197.45" width="12.55" height="12.55"/>` +
	`<rect x="623.68" y="-1.5" width="12.55" height="12.55"/>` +
	`<rect x="1017.73" y="281.76" width="12.55" height="12.55"/>` +
	`<rect x="1017.73" y="890.44" width="12.55" height="12.55"/>` +
	`<rect x="429.41" y="298.79" width="12.55" height="12.55"/>` +
	`<rect x="220.65" y="626.14" width="12.55" height="12.55"/>` +
	`<rect x="666.15" y="358.11" width="12.55" height="12.55"/>` +
	`<rect x="448.56" y="683.14" width="12.55" height="12.55"/>` +
	`<rect x="663.62" y="411.36" width="12.55" height="12.55"/>` +
	`<rect x="331.57" y="890.44" width="12.55" height="12.55"/>` +
	`</g></svg>`

// LogoSmallSVG returns a clean monochrome currentColor SVG tuned for <=32px UI marks.
var LogoSmallSVG = fmt.Sprintf(
	`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1024 896.711" fill="currentColor" fill-rule="evenodd"><path d="%s"/><path d="%s"/><path d="%s"/></svg>`,
	logoSmallPath1, logoSmallPath2, logoSmallPath3,
)

// LogoStandardSVG returns the full-weight monochrome currentColor SVG for >=48px surfaces.
var LogoStandardSVG = fmt.Sprintf(
	`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1024 896.711" fill="currentColor" fill-rule="evenodd"><path d="%s"/><path d="%s"/><path d="%s"/></svg>`,
	logoStdPath1, logoStdPath2, logoStdPath3,
)

// TriStreamSVG returns the 3-channel optical SVG colored across Design (Graphite Ink), Execution (Instrument Amber), and Core (Emerald).
func TriStreamSVG(dark, small bool) string {
	cDesign, cExec, cCore := "#18181b", "#c87010", "#16794b"
	if dark {
		cDesign, cExec, cCore = "#f4f4f5", "#f0a52b", "#65c996"
	}
	p1, p2, p3 := logoStdPath1, logoStdPath2, logoStdPath3
	if small {
		p1, p2, p3 = logoSmallPath1, logoSmallPath2, logoSmallPath3
	}
	return fmt.Sprintf(
		`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1024 896.711" fill-rule="evenodd"><path fill="%s" d="%s"/><path fill="%s" d="%s"/><path fill="%s" d="%s"/></svg>`,
		cDesign, p1, cExec, p2, cCore, p3,
	)
}

var (
	logoSmallOnce, logoStdOnce, logoOutlineOnce sync.Once
	logoSmallSVG, logoStdSVG, logoOutlineSVG    *ui.SVG

	logoTriLightOnce, logoTriDarkOnce sync.Once
	logoTriLightSVG, logoTriDarkSVG   *ui.SVG

	appIconOnce sync.Once
	appIconPNG  []byte
)

// LogoParsedSmall returns the parsed small-size SVG icon (for <=32px ribbon/buttons).
func LogoParsedSmall() *ui.SVG {
	logoSmallOnce.Do(func() { logoSmallSVG = ui.MustParseSVG([]byte(LogoSmallSVG)) })
	return logoSmallSVG
}

// LogoParsedStandard returns the parsed full-weight monochrome brand mark.
func LogoParsedStandard() *ui.SVG {
	logoStdOnce.Do(func() { logoStdSVG = ui.MustParseSVG([]byte(LogoStandardSVG)) })
	return logoStdSVG
}

// LogoParsedTriStream returns the Design-Run-Core 3-color SVG calibrated for Light or Dark mode.
func LogoParsedTriStream(dark bool) *ui.SVG {
	if dark {
		logoTriDarkOnce.Do(func() { logoTriDarkSVG = ui.MustParseSVG([]byte(TriStreamSVG(true, false))) })
		return logoTriDarkSVG
	}
	logoTriLightOnce.Do(func() { logoTriLightSVG = ui.MustParseSVG([]byte(TriStreamSVG(false, false))) })
	return logoTriLightSVG
}

// LogoParsedOutline returns the subtle construction wireframe SVG for About/hero backdrops.
func LogoParsedOutline() *ui.SVG {
	logoOutlineOnce.Do(func() { logoOutlineSVG = ui.MustParseSVG([]byte(LogoOutlineSVG)) })
	return logoOutlineSVG
}

// AppIconPNG returns the 256x256 AsterLab graphite-badge application icon encoded as PNG bytes.
func AppIconPNG() []byte {
	appIconOnce.Do(func() {
		appIconPNG, _ = RenderAppTilePNG(256)
	})
	return appIconPNG
}

// RenderAppTilePNG renders a crisp square application badge in AsterLab C1 graphite aesthetic:
// anodized dark graphite (#111214) rounded squircle with subtle warm-silver/amber studio sheen and centered white mark.
// Uses optical small variant for <=32px and standard variant for >=48px.
func RenderAppTilePNG(size int) ([]byte, error) {
	if size < 16 {
		size = 16
	}
	// Render at 2x when <=64px for crisp subpixel edge antialiasing, then downsample.
	ss := 1
	if size <= 64 {
		ss = 2
	}
	w := size * ss
	dst := image.NewNRGBA(image.Rect(0, 0, w, w))

	radius := float64(w) * 0.225
	center := float64(w) / 2
	half := center
	bgR, bgG, bgB := 17.0, 18.0, 20.0

	for y := 0; y < w; y++ {
		dy := math.Abs(float64(y)+0.5-center) - (half - radius)
		for x := 0; x < w; x++ {
			dx := math.Abs(float64(x)+0.5-center) - (half - radius)
			ox := math.Max(dx, 0)
			oy := math.Max(dy, 0)
			dist := math.Hypot(ox, oy) + math.Min(math.Max(dx, dy), 0) - radius
			alpha := math.Max(0, math.Min(1, 0.5-dist))
			if alpha <= 0 {
				continue
			}
			// Subtle top-left anodized graphite studio sheen with restrained warm amber hint
			nx := float64(x) / float64(w)
			ny := float64(y) / float64(w)
			glow := math.Max(0, 1-math.Hypot(nx-0.25, ny-0.22)*1.25) * 0.14
			r := uint8(math.Min(255, bgR+185*glow))
			g := uint8(math.Min(255, bgG+162*glow))
			b := uint8(math.Min(255, bgB+125*glow))
			dst.SetNRGBA(x, y, color.NRGBA{R: r, G: g, B: b, A: uint8(math.Round(alpha * 255))})
		}
	}

	mark := LogoParsedStandard()
	if size <= 32 {
		mark = LogoParsedSmall()
	}
	// Fit logo nicely inside 56% of tile width (maintaining 1024 : 896.711 aspect ratio).
	markW := int(math.Round(float64(w) * 0.58))
	markH := int(math.Round(float64(markW) * 896.711 / 1024.0))
	ink := color.NRGBA{R: 245, G: 243, B: 238, A: 255}
	fg := mark.Image(markW, markH, ink)
	ox := (w - markW) / 2
	oy := (w - markH) / 2
	draw.Draw(dst, image.Rect(ox, oy, ox+markW, oy+markH), fg, image.Point{}, draw.Over)

	var out image.Image = dst
	if ss > 1 {
		out = downsample2x(dst, size)
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, out); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// LogoWatermarkPNG returns a transparent PNG of the 3-stream brand mark for PDF sheets.
func LogoWatermarkPNG(size int) []byte {
	if size < 24 {
		size = 64
	}
	h := int(math.Round(float64(size) * 896.711 / 1024.0))
	rgba := LogoParsedTriStream(false).Image(size, h, nil)
	var buf bytes.Buffer
	_ = png.Encode(&buf, rgba)
	return buf.Bytes()
}

// BuildMultiResICO packages optical small-size (16/24/32) + standard (48/64/128/256) PNG frames
// into a Windows multi-resolution .ico file.
func BuildMultiResICO() ([]byte, error) {
	sizes := []int{16, 24, 32, 48, 64, 128, 256}
	frames := make([][]byte, len(sizes))
	for i, s := range sizes {
		b, err := RenderAppTilePNG(s)
		if err != nil {
			return nil, err
		}
		frames[i] = b
	}
	var out bytes.Buffer
	_ = binary.Write(&out, binary.LittleEndian, [3]uint16{0, 1, uint16(len(sizes))})
	offset := 6 + 16*len(sizes)
	for i, s := range sizes {
		dim := byte(s)
		if s >= 256 {
			dim = 0
		}
		out.Write([]byte{dim, dim, 0, 0})
		_ = binary.Write(&out, binary.LittleEndian, [2]uint16{1, 32})
		_ = binary.Write(&out, binary.LittleEndian, [2]uint32{uint32(len(frames[i])), uint32(offset)})
		offset += len(frames[i])
	}
	for _, f := range frames {
		out.Write(f)
	}
	return out.Bytes(), nil
}

func downsample2x(src *image.NRGBA, size int) *image.NRGBA {
	dst := image.NewNRGBA(image.Rect(0, 0, size, size))
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			var r, g, b, a uint32
			for dy := 0; dy < 2; dy++ {
				for dx := 0; dx < 2; dx++ {
					p := src.NRGBAAt(x*2+dx, y*2+dy)
					pa := uint32(p.A)
					r += uint32(p.R) * pa
					g += uint32(p.G) * pa
					b += uint32(p.B) * pa
					a += pa
				}
			}
			if a == 0 {
				continue
			}
			dst.SetNRGBA(x, y, color.NRGBA{
				R: uint8(r / a),
				G: uint8(g / a),
				B: uint8(b / a),
				A: uint8((a + 2) / 4),
			})
		}
	}
	return dst
}
