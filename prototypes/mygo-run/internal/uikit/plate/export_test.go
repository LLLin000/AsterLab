package plate

import (
	"bytes"
	"encoding/xml"
	"image/png"
	"strings"
	"testing"
)

func TestPlateExportSVGAndPNG(t *testing.T) {
	cases := []struct {
		name      string
		title     string
		dark      bool
		props     Props
		wantTexts []string
	}{
		{
			name:  "qpcr_96_light",
			title: "RT-qPCR 双靶点复孔板",
			dark:  false,
			props: Props{
				Rows: 8, Cols: 12, ColorBy: "sample",
				GroupIndex: map[string]int{"Ctrl": 0, "VEGF": 1, "NTC": 2},
				Wells: map[string]WellState{
					"A1": {Group: "Ctrl", SampleID: "Ctrl", Replicate: 1, Factors: map[string]string{"靶点": "GAPDH"}},
					"A2": {Group: "Ctrl", SampleID: "Ctrl", Replicate: 2, Factors: map[string]string{"靶点": "GAPDH"}},
					"A3": {Group: "VEGF", SampleID: "VEGF-50", Replicate: 1, Factors: map[string]string{"靶点": "VEGFA"}},
					"H12": {Group: "NTC", Role: "ntc"},
				},
			},
			wantTexts: []string{"RT-qPCR 双靶点复孔板", "96 孔板", "Ctrl 2", "VEGF-50 1"},
		},
		{
			name:  "dish_4_dark",
			title: "抑菌圈四象限滤纸片皿",
			dark:  true,
			props: Props{
				Rows: 2, Cols: 2, Topology: "dish", ColorBy: "group",
				GroupIndex: map[string]int{"万古霉素": 0, "空白滤纸": 1},
				Wells: map[string]WellState{
					"A1": {Group: "万古霉素", SampleID: "Van-30", Replicate: 1},
					"B2": {Group: "空白滤纸", Role: "blank"},
				},
			},
			wantTexts: []string{"抑菌圈四象限滤纸片皿", "培养皿分区", "<circle", "<line"},
		},
		{
			name:  "rack_8_light",
			title: "8联管加样排布",
			dark:  false,
			props: Props{
				Rows: 1, Cols: 8, Topology: "rack_1d", ColorBy: "group",
				GroupIndex: map[string]int{"Ladder": 0, "Sample": 1},
				Wells: map[string]WellState{
					"A1": {Group: "Ladder", Role: "standard"},
					"A2": {Group: "Sample", SampleID: "S1", Replicate: 1},
				},
			},
			wantTexts: []string{"8联管加样排布", "8 位联管", "<rect"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svg := RenderSVG(tc.props, tc.title, tc.dark)
			if err := xml.Unmarshal([]byte(svg), new(any)); err != nil {
				t.Fatalf("invalid SVG XML: %v\nsvg:\n%s", err, svg)
			}
			for _, want := range tc.wantTexts {
				if !strings.Contains(svg, want) {
					t.Fatalf("SVG missing %q", want)
				}
			}

			raw, err := RenderPNG(tc.props, tc.title, tc.dark)
			if err != nil {
				t.Fatalf("RenderPNG failed: %v", err)
			}
			img, err := png.Decode(bytes.NewReader(raw))
			if err != nil {
				t.Fatalf("png.Decode failed: %v", err)
			}
			b := img.Bounds()
			if b.Dx() < 800 || b.Dy() < 400 {
				t.Fatalf("unexpected PNG size: %dx%d", b.Dx(), b.Dy())
			}
			first := img.At(b.Min.X, b.Min.Y)
			varied := false
			for y := b.Min.Y; y < b.Max.Y && !varied; y += 8 {
				for x := b.Min.X; x < b.Max.X; x += 8 {
					if img.At(x, y) != first {
						varied = true
						break
					}
				}
			}
			if !varied {
				t.Fatalf("exported PNG appears uniform/blank")
			}
		})
	}
}
