package asterdoc

import (
	"bytes"
	"encoding/xml"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRenderComponentSVGPNGAndMarkdownFidelity(t *testing.T) {
	src := `---
title: CCK-8 Export Check
---

` + "```asterlab\nkind: group_table\nid: g1\nschema: 1\ntitle: 分组对照表\nrows:\n  - id: ctrl\n    name: 对照组\n    role: control\n    condition: DMSO\n    independent_n: 3\n    technical_repeats: 6\n  - id: treat\n    name: 给药组\n    role: treatment\n    condition: 10 uM\n    independent_n: 3\n    technical_repeats: 6\n```" + `

` + "```asterlab\nkind: plate\nid: p1\nschema: 1\ntitle: 96 孔药敏板\nformat: 96\ngroups:\n  ctrl: [A1, A2, A3]\n  treat: [B1, B2, B3]\nexcluded_wells: [B3]\n```" + `

` + "```asterlab\nkind: param_table\nid: pt1\nschema: 1\ntitle: 培养与孵育参数\nrows:\n  - key: inc_temp\n    label: 孵育温度\n    value: 37\n    unit: Cel\n  - key: drug_conc\n    label: 给药终浓度\n    value: 10\n    unit: uM\n```" + `

` + "```asterlab\nkind: data_table\nid: dt1\nschema: 1\ntitle: OD450 原始读数\ncolumns:\n  - key: well\n    label: 孔位\n    value_kind: text\n  - key: od450\n    label: 吸光度\n    unit: \"1\"\n    value_kind: number\nrows:\n  - {well: A1, od450: 1.042}\n  - {well: B1, od450: 0.438}\n```" + `

` + "```asterlab\nkind: checklist\nid: chk1\nschema: 1\ntitle: 加样前核查\nitems:\n  - id: c1\n    label: 母液室温平衡并涡旋混匀\n    done: true\n  - id: c2\n    label: 最外圈孔补足 200 uL PBS 防边缘蒸发\n    required: true\n```" + `

` + "```asterlab\nkind: calculator\nid: calc1\nschema: 1\ntitle: VEGF 工作液稀释\nmode: dilution\nstock: {value: 10, unit: ug/mL}\ntarget: {value: 50, unit: ng/mL}\nfinal_volume: {value: 2000, unit: uL}\n```" + `
`
	doc, err := Parse(src)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	v := &View{
		Doc:          doc,
		Mode:         ModeDesign,
		UnifiedLive:  true,
		ActualParams: map[string]string{"inc_temp": "37.2"},
	}

	assertValidXML := func(name, svgStr string) {
		t.Helper()
		dec := xml.NewDecoder(strings.NewReader(svgStr))
		for {
			if _, err := dec.Token(); err != nil {
				if err.Error() == "EOF" {
					break
				}
				t.Fatalf("%s invalid SVG XML: %v\n%s", name, err, svgStr)
			}
		}
	}
	assertValidPNG := func(name string, raw []byte) {
		t.Helper()
		if len(raw) == 0 {
			t.Fatalf("%s returned empty PNG", name)
		}
		img, err := png.Decode(bytes.NewReader(raw))
		if err != nil {
			t.Fatalf("%s invalid PNG: %v", name, err)
		}
		if img.Bounds().Dx() < 400 || img.Bounds().Dy() < 120 {
			t.Fatalf("%s unexpected PNG bounds: %v", name, img.Bounds())
		}
	}

	// 1. Plate (1:1 with plateui.RenderSVG & plateui.RenderPNG + excluded cross)
	plateMod := doc.Modules[1]
	plateSVG := v.RenderModuleSVG(plateMod.Kind, plateMod.Data, false)
	assertValidXML("plate", plateSVG)
	if !strings.Contains(plateSVG, "96 孔药敏板") || !strings.Contains(plateSVG, "已分配 6 / 96 孔") || !strings.Contains(plateSVG, "<path d=\"M") {
		t.Fatalf("plate SVG missing expected title/summary/excluded cross:\n%s", plateSVG)
	}
	platePNG, err := v.RenderModulePNG(plateMod.Kind, plateMod.Schema, plateMod.Data, false)
	if err != nil {
		t.Fatalf("plate RenderModulePNG: %v", err)
	}
	assertValidPNG("plate", platePNG)

	// 2. Group Table (swatches + role badge + n=3 × 6 + 2x Retina PNG)
	gtMod := doc.Modules[0]
	gtSVG := v.RenderModuleSVG(gtMod.Kind, gtMod.Data, false)
	assertValidXML("group_table", gtSVG)
	if !strings.Contains(gtSVG, "分组对照表") || !strings.Contains(gtSVG, "n=3 × 6") {
		t.Fatalf("group_table SVG missing expected content:\n%s", gtSVG)
	}
	gtPNG, err := v.RenderModulePNG(gtMod.Kind, gtMod.Schema, gtMod.Data, false)
	if err != nil {
		t.Fatalf("group_table RenderModulePNG: %v", err)
	}
	assertValidPNG("group_table", gtPNG)

	// 3. Param Table (DisplayUnit °C + recorded actual pill)
	ptMod := doc.Modules[2]
	ptSVG := v.RenderModuleSVG(ptMod.Kind, ptMod.Data, false)
	assertValidXML("param_table", ptSVG)
	if !strings.Contains(ptSVG, "实测 37.2") || !strings.Contains(ptSVG, "°C") {
		t.Fatalf("param_table SVG missing recorded actual or DisplayUnit °C:\n%s", ptSVG)
	}
	ptPNG, err := v.RenderModulePNG(ptMod.Kind, ptMod.Schema, ptMod.Data, true)
	if err != nil {
		t.Fatalf("param_table RenderModulePNG: %v", err)
	}
	assertValidPNG("param_table", ptPNG)

	// 4. Data Table (key/label columns + rows)
	dtMod := doc.Modules[3]
	dtSVG := v.RenderModuleSVG(dtMod.Kind, dtMod.Data, false)
	assertValidXML("data_table", dtSVG)
	dtMD := v.FormatModuleAsMarkdown(dtMod.Kind, dtMod.Data)
	if !strings.Contains(dtMD, "| 孔位 | 吸光度") || !strings.Contains(dtMD, "1.042") {
		t.Fatalf("data_table Markdown mismatch:\n%s", dtMD)
	}
	dtPNG, err := v.RenderModulePNG(dtMod.Kind, dtMod.Schema, dtMod.Data, false)
	if err != nil {
		t.Fatalf("data_table RenderModulePNG: %v", err)
	}
	assertValidPNG("data_table", dtPNG)

	// 5. Checklist (done state + required badge)
	chkMod := doc.Modules[4]
	chkSVG := v.RenderModuleSVG(chkMod.Kind, chkMod.Data, false)
	assertValidXML("checklist", chkSVG)
	if !strings.Contains(chkSVG, "1 / 2 已完成") || !strings.Contains(chkSVG, "必检") {
		t.Fatalf("checklist SVG mismatch:\n%s", chkSVG)
	}
	chkMD := v.FormatModuleAsMarkdown(chkMod.Kind, chkMod.Data)
	if !strings.Contains(chkMD, "- [x] 母液室温平衡") || !strings.Contains(chkMD, "- [ ] 最外圈孔") {
		t.Fatalf("checklist Markdown mismatch:\n%s", chkMD)
	}
	chkPNG, err := v.RenderModulePNG(chkMod.Kind, chkMod.Schema, chkMod.Data, false)
	if err != nil {
		t.Fatalf("checklist RenderModulePNG: %v", err)
	}
	assertValidPNG("checklist", chkPNG)

	// 6. Calculator (evaluated recipe: 10 uL stock + 1990 uL diluent!)
	calcMod := doc.Modules[5]
	calcSVG := v.RenderModuleSVG(calcMod.Kind, calcMod.Data, false)
	assertValidXML("calculator", calcSVG)
	if !strings.Contains(calcSVG, "1990") {
		t.Fatalf("calculator SVG missing solved diluent volume 1990:\n%s", calcSVG)
	}
	calcMD := v.FormatModuleAsMarkdown(calcMod.Kind, calcMod.Data)
	if !strings.Contains(calcMD, "1990") {
		t.Fatalf("calculator Markdown missing solved diluent volume 1990:\n%s", calcMD)
	}
	calcPNG, err := v.RenderModulePNG(calcMod.Kind, calcMod.Schema, calcMod.Data, false)
	if err != nil {
		t.Fatalf("calculator RenderModulePNG: %v", err)
	}
	assertValidPNG("calculator", calcPNG)

	if outDir := os.Getenv("ASTERLAB_EXPORT_SAMPLE_DIR"); outDir != "" {
		_ = os.MkdirAll(outDir, 0o755)
		_ = os.WriteFile(filepath.Join(outDir, "plate.png"), platePNG, 0o644)
		_ = os.WriteFile(filepath.Join(outDir, "plate.svg"), []byte(plateSVG), 0o644)
		_ = os.WriteFile(filepath.Join(outDir, "group_table.png"), gtPNG, 0o644)
		_ = os.WriteFile(filepath.Join(outDir, "group_table.svg"), []byte(gtSVG), 0o644)
		_ = os.WriteFile(filepath.Join(outDir, "calculator.png"), calcPNG, 0o644)
		_ = os.WriteFile(filepath.Join(outDir, "calculator.svg"), []byte(calcSVG), 0o644)
	}
}
