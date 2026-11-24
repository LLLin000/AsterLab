//go:build e2e

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/onevra/source/components"

	"asterlab-mygo-prototype/internal/asterdoc"
	"asterlab-mygo-prototype/internal/shell"
	"asterlab-mygo-prototype/internal/theme"
	"asterlab-mygo-prototype/internal/vault"
)

func renderAuditDoc(t *testing.T, outDir, name, src string, w, h int, dark bool, maxColW float32) {
	t.Helper()
	doc, err := asterdoc.Parse(src)
	if err != nil {
		t.Fatalf("%s parse error: %v", name, err)
	}
	v := &asterdoc.View{
		Doc:         doc,
		Mode:        asterdoc.ModeDesign,
		UnifiedLive: true,
		Allowed:     map[string]bool{asterdoc.PermDocumentEdit: true},
		OnPickFigureFile:     func(stepID, moduleID string) {},
		OnPickAttachmentFile: func(stepID, blockID string) {},
	}
	renderVisualPNGSteps(t, outDir, name, w, h, dark, func(c *ui.Context) {
		tok := theme.GetTokens(c.Theme())
		ui.Row(c).FillWidth().Justify(ui.Center).Padding(float32(tok.Spacing16)).Children(func() {
			col := ui.Column(c).FillWidth()
			if maxColW > 0 {
				col.MaxWidth(maxColW)
			}
			col.Children(func() {
				v.Render(c)
			})
		})
	}, func(tt *ui.Tester) {
		tt.Frame()
	})
}

func TestComponentVisualAuditGallery(t *testing.T) {
	outDir := os.Getenv("ASTERLAB_VISUAL_OUT")
	if outDir == "" {
		outDir = t.TempDir()
	} else if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatalf("mkdir visual out: %v", err)
	}

	const petriSVG = `<svg viewBox="0 0 240 160"><circle cx="120" cy="80" r="64" fill="#F2EAD3" stroke="#9A8F70" stroke-width="2"/><line x1="120" y1="16" x2="120" y2="144" stroke="#B6AC8E" stroke-dasharray="4 3"/><line x1="56" y1="80" x2="184" y2="80" stroke="#B6AC8E" stroke-dasharray="4 3"/><circle cx="92" cy="56" r="10" fill="#FFFFFF" stroke="#64748B"/><circle cx="148" cy="56" r="16" fill="#DBEAFE" fill-opacity="0.55" stroke="#2563EB"/><circle cx="92" cy="104" r="22" fill="#DCFCE7" fill-opacity="0.6" stroke="#16A34A"/><text x="92" y="42" text-anchor="middle" font-size="11" fill="#1E293B">PBS</text><text x="148" y="36" text-anchor="middle" font-size="11" fill="#1E293B">HAMA</text><text x="92" y="136" text-anchor="middle" font-size="11" fill="#1E293B">Vanc</text></svg>`

	tablesSrc := `---
title: 组件视觉审计 — 表格类
---

` + "```asterlab" + `
kind: group_table
id: gt-full
schema: 1
title: 抗菌与细胞毒性分组
rows:
  - id: g-ctrl
    name: PBS 空白对照
    role: control
    condition: 等体积 PBS
    independent_n: 3
    technical_repeats: 3
  - id: g-low
    name: 低浓度处理
    role: treatment
    condition: 10 ug/mL HAMA
    independent_n: 3
    technical_repeats: 3
  - id: g-high
    name: 高浓度处理
    role: treatment
    condition: 50 ug/mL HAMA
    independent_n: 3
    technical_repeats: 3
  - id: g-pos
    name: 阳性对照
    role: comparator
    condition: 1 ug/mL 万古霉素
    independent_n: 3
    technical_repeats: 2
` + "```" + `

` + "```asterlab" + `
kind: group_table
id: gt-empty
schema: 1
title: 新建分组表（空态）
rows: []
` + "```" + `

` + "```asterlab" + `
kind: param_table
id: pt-1
schema: 1
title: 培养与孵育参数
rows:
  - key: cell_line
    label: 细胞系
    value: HUVEC-C (P4)
  - key: incubator
    label: 培养环境
    value: 37℃, 5% CO2 恒温培养箱
  - key: seed_density
    label: 接种密度
    value: 1.5e4
    unit: cells/well
  - key: vol_per_well
    label: 每孔加样体积
    value: 100
    unit: uL
` + "```" + `

` + "```asterlab" + `
kind: data_table
id: dt-1
schema: 1
title: CCK-8 酶标仪读数 (OD450)
columns:
  - key: well
    label: 孔位
    value_kind: text
  - key: group
    label: 组别
    value_kind: text
  - key: od450
    label: OD450
    value_kind: number
    unit: AU
  - key: note
    label: 备注
    value_kind: text
rows:
  - well: A1
    group: PBS 对照
    od450: 1.42
    note: 贴壁良好
  - well: B1
    group: 低浓度
    od450: 1.38
    note: —
  - well: C1
    group: 高浓度
    od450: 1.19
    note: 偶见悬浮
` + "```" + `
`
	renderAuditDoc(t, outDir, "01_tables.png", tablesSrc, 860, 1220, false, 820)

	platesSrc := `---
title: 组件视觉审计 — 孔板类
---

` + "```asterlab" + `
kind: group_table
id: gt-pl
schema: 1
title: 孔板分组
rows:
  - id: g-pbs
    name: PBS 对照
    role: control
  - id: g-hama
    name: HAMA
    role: treatment
  - id: g-pos
    name: 阳性对照
    role: comparator
` + "```" + `

` + "```asterlab" + `
kind: plate
id: pl-12
schema: 1
title: 12 孔板（已排布，无额外图层）
format: 12
groups:
  g-pbs: [A1, A2, A3]
  g-hama: [B1, B2, B3]
  g-pos: [C1, C2, C3]
` + "```" + `

` + "```asterlab" + `
kind: plate
id: pl-12-empty
schema: 1
title: 12 孔板（空态 0 孔）
format: 12
groups: {}
` + "```" + `

` + "```asterlab" + `
kind: plate
id: pl-24-layers
schema: 1
title: 24 孔梯度板（含自定义图层）
format: 24
groups:
  g-pbs: [A1, A2, A3]
  g-hama: [B1, B2, B3, C1, C2, C3]
layers:
  浓度梯度: {A1: 0 uM, A2: 0 uM, A3: 0 uM, B1: 10 uM, B2: 20 uM, B3: 40 uM}
` + "```" + `

` + "```asterlab" + `
kind: plate
id: pl-96
schema: 1
title: 96 孔高通量测定板
format: 96
groups:
  g-pbs: [A1, A2, A3]
  g-hama: [B1, B2, B3, B4, B5, B6]
` + "```" + `
`
	renderAuditDoc(t, outDir, "02_plates.png", platesSrc, 860, 1380, false, 820)

	opsSrc := `---
title: 组件视觉审计 — 计算/计时/检查清单
---

` + "```asterlab" + `
kind: calculator
id: calc-c1v1
schema: 1
title: C1V1 母液稀释配制
mode: formula
rows:
  - key: c1
    label: 母液浓度 C1
    value: 10
    unit: mg/mL
  - key: c2
    label: 目标浓度 C2
    value: 0.5
    unit: mg/mL
  - key: v2
    label: 配制总体积 V2
    value: 2000
    unit: uL
  - key: v1
    label: 所需母液体积 V1
    formula: c2 * v2 / c1
    unit: uL
` + "```" + `

` + "```asterlab" + `
kind: calculator
id: calc-sd
schema: 1
title: 抑菌倍比梯度稀释
mode: serial_dilution
dilution_factor: 2
steps_count: 6
stock: {value: 1024, unit: ug/mL}
volume_per_unit: {value: 200, unit: uL}
` + "```" + `

` + "```asterlab" + `
kind: timer
id: tm-1
schema: 1
title: 室温避光孵育
mode: countdown
status: idle
duration:
  value: 30
  unit: min
` + "```" + `

` + "```asterlab" + `
kind: checklist
id: chk-1
schema: 1
title: 涂板前质控核查
items:
  - id: c1
    label: 超净台紫外灭菌 30 min 并拭净台面
    done: true
    required: true
  - id: c2
    label: 确认 HUVEC 融合度在 80%~90%，无支原体污染迹象
    done: false
    required: true
  - id: c3
    label: 预热完全培养基至 37℃
    done: true
  - id: c4
    label: 核对 12 孔板分组标记与移液器校准量程
    done: false
` + "```" + `
`
	renderAuditDoc(t, outDir, "03_ops_calc_timer_check.png", opsSrc, 860, 1220, false, 820)

	mediaSrc := `---
title: 组件视觉审计 — 图示与附件
---

` + "```asterlab" + `
kind: plate
id: pl-att
schema: 1
title: 12 孔板（关联采图）
format: 12
groups: {}
` + "```" + `

` + "```asterlab" + `
kind: figure
id: fig-empty
schema: 1
title: 空白示意图占位
svg: "<svg viewBox='0 0 240 160'></svg>"
caption: ""
` + "```" + `

` + "```asterlab" + `
kind: figure
id: fig-svg
schema: 1
title: 四象限抑菌圈布片示意图
caption: 90 mm MH 琼脂平板四象限牛津杯布片位置
svg: |
  ` + petriSVG + `
` + "```" + `

` + "```asterlab" + `
kind: attachment
id: att-ev
schema: 1
title: 涂板操作现场照片
scope: step_evidence
` + "```" + `

` + "```asterlab" + `
kind: attachment
id: att-grid
schema: 1
title: 荧光显微镜网格采图
scope: container_grid
container_ref: pl-att
slots_per_cell: 3
media_hint: 488nm GFP 通道 10x 物镜
` + "```" + `
`
	renderAuditDoc(t, outDir, "04_media_attachment.png", mediaSrc, 860, 1220, false, 820)

	pairingSrc := `---
title: 组件视觉审计 — 矩形双栏自动并排
---

` + "```asterlab" + `
kind: timer
id: tm-a
schema: 1
title: 一抗孵育（常温）
mode: countdown
status: idle
duration:
  value: 60
  unit: min
` + "```" + `

` + "```asterlab" + `
kind: timer
id: tm-b
schema: 1
title: 二抗避光孵育
mode: countdown
status: idle
duration:
  value: 45
  unit: min
` + "```" + `

` + "```asterlab" + `
kind: calculator
id: cal-a
schema: 1
title: 细胞悬液重悬配比
mode: formula
rows:
  - key: n
    label: 目标孔数
    value: 12
    unit: 孔
  - key: v
    label: 每孔体积
    value: 100
    unit: uL
  - key: tot
    label: 配制总体积
    formula: n * v * 1.1
    unit: uL
` + "```" + `

` + "```asterlab" + `
kind: calculator
id: cal-b
schema: 1
title: 基质胶稀释配比
mode: formula
rows:
  - key: c1
    label: 原液比例
    value: 100
    unit: "%"
  - key: v2
    label: 目标体积
    value: 600
    unit: uL
  - key: v1
    label: 所需原液
    formula: v2 * 0.5
    unit: uL
` + "```" + `

` + "```asterlab" + `
kind: group_table
id: gt-pair
schema: 1
title: 实验分组
rows:
  - id: g-ctrl
    name: PBS 对照
    role: control
    independent_n: 3
  - id: g-low
    name: 低剂量
    role: treatment
    independent_n: 3
  - id: g-high
    name: 高剂量
    role: treatment
    independent_n: 3
` + "```" + `

` + "```asterlab" + `
kind: plate
id: pl-pair-12
schema: 1
title: 12 孔接种布局
format: 12
groups:
  g-ctrl: [A1, A2, A3]
  g-low: [B1, B2, B3]
  g-high: [C1, C2, C3]
` + "```" + `

` + "```asterlab" + `
kind: figure
id: fig-pair-1
schema: 1
title: 布片方案 A（低剂量组）
caption: 象限 1-4 对照与实验组
svg: |
  ` + petriSVG + `
` + "```" + `

` + "```asterlab" + `
kind: figure
id: fig-pair-2
schema: 1
title: 布片方案 B（复核组）
caption: 18 h 抑菌圈对比参考
svg: |
  ` + petriSVG + `
` + "```" + `

` + "```asterlab" + `
kind: calculator
id: cal-sd-left
schema: 1
title: 标准单步配液
mode: formula
rows:
  - key: v
    label: 总量
    value: 500
    unit: uL
` + "```" + `

` + "```asterlab" + `
kind: calculator
id: cal-sd-right
schema: 1
title: 连续倍比稀释（不并排）
mode: serial_dilution
dilution_factor: 2
steps_count: 5
stock: {value: 512, unit: ug/mL}
volume_per_unit: {value: 200, unit: uL}
` + "```" + `

` + "```asterlab" + `
# @layout: full
kind: timer
id: tm-full
schema: 1
title: 独占整行计时器
mode: countdown
status: idle
duration:
  value: 15
  unit: min
` + "```" + `

` + "```asterlab" + `
kind: timer
id: tm-after-full
schema: 1
title: 后续计时器（不应与独占整行并排）
mode: countdown
status: idle
duration:
  value: 10
  unit: min
` + "```" + `
`
	renderAuditDoc(t, outDir, "05_pairing_wide_light.png", pairingSrc, 940, 1750, false, 860)
	renderAuditDoc(t, outDir, "05_pairing_wide_dark.png", pairingSrc, 940, 1750, true, 860)
	renderAuditDoc(t, outDir, "05_pairing_narrow_520.png", pairingSrc, 560, 1750, false, 520)

	// Confirm files exist
	for _, fn := range []string{
		"01_tables.png",
		"02_plates.png",
		"03_ops_calc_timer_check.png",
		"04_media_attachment.png",
		"05_pairing_wide_light.png",
		"05_pairing_wide_dark.png",
		"05_pairing_narrow_520.png",
	} {
		if st, err := os.Stat(filepath.Join(outDir, fn)); err != nil || st.Size() < 1024 {
			t.Fatalf("missing/empty visual output %s: %v", fn, err)
		}
	}
}

func TestFB113233AndSettingsVisualAudit(t *testing.T) {
	outDir := os.Getenv("ASTERLAB_VISUAL_OUT")
	if outDir == "" {
		outDir = t.TempDir()
	} else if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatalf("mkdir visual out: %v", err)
	}

	// L1 + L2: FB-20261124-113233 UI-01 serial_dilution 96-well click-through & zero T6 clip
	sdSrc := "---\ntitle: 系列稀释回归验证\n---\n\n```asterlab\n" +
		"kind: calculator\nid: calc-sd-audit\nschema: 1\n" +
		"title: 10 倍系列稀释梯度参考（每管 900 uL PBS + 100 uL 上级）\n" +
		"mode: serial_dilution\ndilution_factor: 10\nsteps_count: 6\n" +
		"stock: {value: 100000000, unit: CFU/mL}\nvolume_per_unit: {value: 1, unit: mL}\n" +
		"```\n"
	curSrc := sdSrc
	docSD, err := asterdoc.Parse(curSrc)
	if err != nil {
		t.Fatalf("Parse serial_dilution: %v", err)
	}
	vSD := &asterdoc.View{
		Doc:         docSD,
		Mode:        asterdoc.ModeDesign,
		UnifiedLive: true,
		Allowed:     map[string]bool{asterdoc.PermDocumentEdit: true},
	}
	vSD.OnChange = func(nextSrc string) {
		curSrc = nextSrc
		if parsed, perr := asterdoc.Parse(nextSrc); perr == nil {
			vSD.Doc = parsed
		}
	}

	var ttSD *ui.Tester
	ttSD = ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.LightTheme())
		tok := theme.GetTokens(c.Theme())
		ui.Row(c).Fill().Background(tok.Canvas).Justify(ui.Center).Padding(float32(tok.Spacing16)).Children(func() {
			ui.Column(c).FillWidth().MaxWidth(820).Children(func() {
				vSD.Render(c)
			})
		})
	}, 880, 360)
	ttSD.SetDark(false)
	ttSD.Frame()
	ttSD.Frame()
	if !ttSD.HasText("T6") {
		t.Fatalf("expected T6 visible cleanly, got: %q", ttSD.Texts())
	}
	if err := ttSD.Click("调整稀释参数"); err != nil {
		t.Fatalf("click 参数 trigger: %v", err)
	}
	ttSD.Advance(250 * time.Millisecond)
	ttSD.Frame()
	if err := ttSD.Click("96 孔板排枪"); err != nil {
		t.Fatalf("click 96 孔板排枪 inside 参数 popover: %v", err)
	}
	ttSD.Advance(250 * time.Millisecond)
	ttSD.Frame()
	if !strings.Contains(curSrc, "# @vessel: plate96") {
		t.Fatalf("expected # @vessel: plate96 persisted in source, got:\n%s", curSrc)
	}
	if !ttSD.HasText("Col 6") {
		t.Fatalf("expected 96-well multichannel Col 6 after toggle, got: %q", ttSD.Texts())
	}

	// Render combined Tube + Plate96 view for visual inspection
	combinedSDSrc := "---\ntitle: FB-20261124-113233 UI-01 修复对比（离心管 vs 96 孔板排枪）\n---\n\n" +
		"```asterlab\nkind: calculator\nid: sd-tube\nschema: 1\n" +
		"title: 10 倍系列稀释梯度（离心管 T1~T6 零裁切）\n" +
		"mode: serial_dilution\ndilution_factor: 10\nsteps_count: 6\n" +
		"stock: {value: 100000000, unit: CFU/mL}\nvolume_per_unit: {value: 1, unit: mL}\n```\n\n" +
		"```asterlab\n# @vessel: plate96\nkind: calculator\nid: sd-plate96\nschema: 1\n" +
		"title: 96 孔板排枪系列稀释（已可点击持久化）\n" +
		"mode: serial_dilution\ndilution_factor: 10\nsteps_count: 6\n" +
		"stock: {value: 100000000, unit: CFU/mL}\nvolume_per_unit: {value: 200, unit: uL}\n```\n"
	renderAuditDoc(t, outDir, "fb113233_01_sd_tube_vs_plate96.png", combinedSDSrc, 880, 620, false, 820)

	// L2: FB-20261124-113233 UI-02 exact 90mm petri dish SVG + long caption wrapping
	const exactDishSVG = `<svg viewBox="0 0 360 280">` +
		`<circle cx="180" cy="148" r="102" fill="#F8F9FA" stroke="#1E293B" stroke-width="2.5"/>` +
		`<circle cx="180" cy="148" r="90" fill="none" stroke="#64748B" stroke-dasharray="4 3"/>` +
		`<line x1="180" y1="46" x2="180" y2="250" stroke="#475569" stroke-dasharray="4 3"/>` +
		`<line x1="78" y1="148" x2="282" y2="148" stroke="#475569" stroke-dasharray="4 3"/>` +
		`<polygon points="180,38 175,48 185,48" fill="#1E293B"/>` +
		`<text x="180" y="28" text-anchor="middle" font-size="12" fill="#1E293B">↑ 12点方向基准记号</text>` +
		`<circle cx="132" cy="108" r="9" fill="#FFFFFF" stroke="#1E293B" stroke-width="1.5"/>` +
		`<circle cx="228" cy="108" r="9" fill="#FFFFFF" stroke="#1E293B" stroke-width="1.5"/>` +
		`<circle cx="228" cy="108" r="22" fill="none" stroke="#334155" stroke-dasharray="3 2"/>` +
		`<circle cx="132" cy="192" r="9" fill="#FFFFFF" stroke="#1E293B" stroke-width="1.5"/>` +
		`<circle cx="132" cy="192" r="26" fill="none" stroke="#334155" stroke-dasharray="3 2"/>` +
		`<circle cx="228" cy="192" r="9" fill="#FFFFFF" stroke="#1E293B" stroke-width="1.5"/>` +
		`<circle cx="228" cy="192" r="38" fill="none" stroke="#334155" stroke-dasharray="3 2"/>` +
		`<text x="114" y="62" text-anchor="end" font-size="11" fill="#1E293B">Q1: C (PBS 10 µL)</text>` +
		`<text x="246" y="62" text-anchor="start" font-size="11" fill="#1E293B">Q2: L3 (Lys 20 µg)</text>` +
		`<text x="114" y="264" text-anchor="end" font-size="11" fill="#1E293B">Q3: V3 (Van 1 µg)</text>` +
		`<text x="246" y="264" text-anchor="start" font-size="11" fill="#1E293B">Q4: VL3 (Van+Lys)</text>` +
		`</svg>`
	dishSrc := "---\ntitle: FB-20261124-113233 UI-02 SVG 文字与长图注修复\n---\n\n" +
		"```asterlab\nkind: figure\nid: fig-dish-audit\nschema: 1\n" +
		"caption: 固定方向约定：皿底 12 点钟画「↑」箭头；左上 Q1=C(PBS)、右上 Q2=L3(Lys 20 µg)、左下 Q3=V3(Van 1 µg)、右下 Q4=VL3(Van+Lys)\n" +
		"svg: |\n  " + exactDishSVG + "\n```\n"
	renderAuditDoc(t, outDir, "fb113233_02_svg_and_caption_light.png", dishSrc, 820, 420, false, 780)
	renderAuditDoc(t, outDir, "fb113233_02_svg_and_caption_dark.png", dishSrc, 820, 420, true, 780)

	// L2+L3: Obsidian-minimal Settings screenshots
	sh := &shell.Shell{
		TodayDate:       "2026-11-24",
		VaultPathInput:  `D:\LabVault\AsterLab`,
		VaultFiles:        []vault.FileEntry{{RelPath: "a.asterml"}, {RelPath: "templates/t.asterml", IsTemplate: true}, {RelPath: "assets/p.png", IsAsset: true}},
		AppVersion:      "0.1.10-devdebug-r9",
		BuildCommit:     "9fa8ff9",
		UpdateStatus:    "up_to_date",
		LiquidGlass:     true,
		VaultSettingsOpen: true,
		SettingsView:    components.SettingsState{Category: "vault"},
	}
	renderVisualPNGSteps(t, outDir, "fb113233_03_settings_vault.png", 1040, 660, false, func(c *ui.Context) {
		sh.RenderSplitView(c)
	}, func(st *ui.Tester) {
		st.Frame()
		st.Advance(220 * time.Millisecond)
		st.Frame()
	})
	sh.SettingsView = components.SettingsState{Category: "appearance"}
	renderVisualPNGSteps(t, outDir, "fb113233_04_settings_appearance_light.png", 1040, 660, false, func(c *ui.Context) {
		sh.RenderSplitView(c)
	}, func(st *ui.Tester) {
		st.Frame()
		st.Advance(220 * time.Millisecond)
		st.Frame()
	})
	renderVisualPNGSteps(t, outDir, "fb113233_04_settings_appearance_dark.png", 1040, 660, true, func(c *ui.Context) {
		sh.RenderSplitView(c)
	}, func(st *ui.Tester) {
		st.Frame()
		st.Advance(220 * time.Millisecond)
		st.Frame()
	})
	sh.SettingsView = components.SettingsState{Category: "about"}
	renderVisualPNGSteps(t, outDir, "fb113233_05_settings_about.png", 1040, 660, false, func(c *ui.Context) {
		sh.RenderSplitView(c)
	}, func(st *ui.Tester) {
		st.Frame()
		st.Advance(220 * time.Millisecond)
		st.Frame()
	})
}
