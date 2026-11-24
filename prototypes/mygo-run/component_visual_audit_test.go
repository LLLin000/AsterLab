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
	"asterlab-mygo-prototype/internal/tools"
	"asterlab-mygo-prototype/internal/uikit/kit"
	"asterlab-mygo-prototype/internal/uikit/labsticker"
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

func TestStage1And2HighCraftVisualAudit(t *testing.T) {
	outDir := os.Getenv("ASTERLAB_HIGHCRAFT_OUT")
	if outDir == "" {
		outDir = `C:\Users\Lin\.gemini\antigravity-cli\brain\16e71d44-0152-40cd-8151-d8a1d2df13bb\scratch\highcraft_visual\r1`
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatalf("mkdir highcraft out: %v", err)
	}

	// 1. UI-01 Context Menu Dismiss: right-click inside component card -> open -> left-click inside SAME card -> must dismiss!
	ctxSrc := "---\ntitle: UI-01 右键菜单点击组件内部关闭回归\n---\n\n" +
		"```asterlab\nkind: param_table\nid: pt-ctx\nschema: 1\ntitle: 培养基配方参数\nrows:\n" +
		"  - key: serum\n    label: 胎牛血清 (FBS)\n    value: 10\n    unit: \"%\"\n" +
		"  - key: pen_strep\n    label: 青链霉素双抗\n    value: 1\n    unit: \"%\"\n" +
		"  - key: glucose\n    label: 高糖 DMEM 基底\n    value: 4.5\n    unit: g/L\n" +
		"  - key: co2\n    label: 孵育二氧化碳浓度\n    value: 5\n    unit: \"%\"\n```\n"
	ctxDoc, err := asterdoc.Parse(ctxSrc)
	if err != nil {
		t.Fatalf("parse ctxSrc: %v", err)
	}
	ctxView := &asterdoc.View{
		Doc: ctxDoc, Mode: asterdoc.ModeDesign, UnifiedLive: true,
		Allowed: map[string]bool{asterdoc.PermDocumentEdit: true},
	}
	renderVisualPNGSteps(t, outDir, "hc_01a_ctx_open.png", 920, 480, false, func(c *ui.Context) {
		tok := theme.GetTokens(c.Theme())
		ui.Column(c).Fill().Padding(float32(tok.Spacing16)).Children(func() {
			ui.Column(c).FillWidth().MaxWidth(780).Children(func() {
				ctxView.Render(c)
			})
		})
	}, func(st *ui.Tester) {
		st.Frame()
		hdrRect, ok := st.Find("培养基配方参数")
		if !ok {
			t.Fatalf("expected card title inside component card, texts=%v", st.Texts())
		}
		st.RightClickAt(hdrRect.X+8, hdrRect.Y+hdrRect.H/2)
		st.Advance(200 * time.Millisecond)
		st.Frame()
		if !st.HasText("复制为 SVG") {
			t.Fatalf("expected context menu item '复制为 SVG' to be open after right-click, texts=%v menu=%v", st.Texts(), st.Menu())
		}
	})

	renderVisualPNGSteps(t, outDir, "hc_01b_ctx_dismissed_inside.png", 920, 480, false, func(c *ui.Context) {
		tok := theme.GetTokens(c.Theme())
		ui.Column(c).Fill().Padding(float32(tok.Spacing16)).Children(func() {
			ui.Column(c).FillWidth().MaxWidth(780).Children(func() {
				ctxView.Render(c)
			})
		})
	}, func(st *ui.Tester) {
		st.Frame()
		hdrRect, okHdr := st.Find("培养基配方参数")
		if !okHdr {
			t.Fatalf("expected card title inside component card, texts=%v", st.Texts())
		}
		metaRect, okMeta := st.Find("4 项")
		if !okMeta {
			t.Fatalf("expected '4 项' inside same component card, texts=%v", st.Texts())
		}
		// Right-click left side of card inside component
		st.RightClickAt(hdrRect.X+8, hdrRect.Y+hdrRect.H/2)
		st.Advance(200 * time.Millisecond)
		st.Frame()
		if !st.HasText("复制为 SVG") {
			t.Fatalf("expected context menu open before inside-card click, texts=%v menu=%v", st.Texts(), st.Menu())
		}
		// Left-click "4 项" on the far right INSIDE the same component card (strictly outside menu panel)
		st.ClickAt(metaRect.X+metaRect.W/2, metaRect.Y+metaRect.H/2)
		st.Advance(200 * time.Millisecond)
		st.Frame()
		if st.HasText("复制为 SVG") {
			t.Fatalf("UI-01 regression: clicking inside same component card failed to dismiss context menu!")
		}
	})

	// 2. UI-02 Real 8x12 96-well Serial Dilution Plate (Light & Dark)
	sdSrc := "---\ntitle: UI-02 96 孔板排枪梯度稀释视觉审计\n---\n\n" +
		"```asterlab\nkind: group_table\nid: gt-sd\nschema: 1\ntitle: 抑菌梯度受试组\nrows:\n" +
		"  - id: g-a\n    name: 样品组 A\n    role: treatment\n" +
		"  - id: g-b\n    name: 样品组 B\n    role: treatment\n" +
		"  - id: g-c\n    name: 阳性对照\n    role: comparator\n```\n\n" +
		"```asterlab\n# @vessel: plate96\nkind: calculator\nid: sd-96-hc\nschema: 1\ntitle: 96 孔板 10 倍连续梯度稀释\n" +
		"mode: serial_dilution\nstock:\n  value: 1000000\n  unit: CFU/mL\n" +
		"dilution_factor: 10\nsteps_count: 7\n" +
		"volume_per_unit:\n  value: 200\n  unit: uL\n```\n"
	renderAuditDoc(t, outDir, "hc_02a_sd96_light.png", sdSrc, 940, 780, false, 860)
	renderAuditDoc(t, outDir, "hc_02b_sd96_dark.png", sdSrc, 940, 780, true, 860)

	// 3. UI-03 Independent 90mm Agar Plates Matrix (12 dishes) vs Single 4-Quadrant Petri Dish
	cfuMatrixSrc := "---\ntitle: UI-03 独立平皿矩阵 vs 四分区单皿\n---\n\n" +
		"```asterlab\nkind: group_table\nid: gt-cfu\nschema: 1\ntitle: 涂板实验组\nrows:\n" +
		"  - id: g-pbs\n    name: PBS\n    role: control\n" +
		"  - id: g-van\n    name: Van\n    role: comparator\n" +
		"  - id: g-vl3\n    name: VL3\n    role: treatment\n```\n\n" +
		"```asterlab\nkind: plate\nid: dish-matrix-12\nschema: 1\ntitle: 12 块独立 CFU 涂布琼脂平板\nformat: 12\ntopology: dish\nrows: 3\ncolumns: 4\n" +
		"groups:\n" +
		"  g-pbs: [A1, A2, A3, A4]\n" +
		"  g-van: [B1, B2, B3, B4]\n" +
		"  g-vl3: [C1, C2, C3, C4]\n" +
		"layers:\n" +
		"  稀释度: {A1: '10^-2', A2: '10^-3', A3: '10^-4', A4: '10^-5', B1: '10^-2', B2: '10^-3', B3: '10^-4', B4: '10^-5', C1: '10^-2', C2: '10^-3', C3: '10^-4', C4: '10^-5'}\n```\n"
	renderAuditDoc(t, outDir, "hc_03a_cfu_dish_matrix_12.png", cfuMatrixSrc, 860, 700, false, 780)

	quadDishSrc := "---\ntitle: 2x2 四分区单培养皿保留\n---\n\n" +
		"```asterlab\nkind: group_table\nid: gt-quad\nschema: 1\nrows:\n" +
		"  - id: q1\n    name: C(PBS)\n    role: control\n" +
		"  - id: q2\n    name: L3(Lys)\n    role: treatment\n" +
		"  - id: q3\n    name: V3(Van)\n    role: treatment\n" +
		"  - id: q4\n    name: VL3\n    role: comparator\n```\n\n" +
		"```asterlab\nkind: plate\nid: dish-quad-4\nschema: 1\ntitle: 单培养皿四分区抑菌圈测试\nformat: 4\ntopology: dish\nrows: 2\ncolumns: 2\n" +
		"groups:\n  q1: [A1]\n  q2: [A2]\n  q3: [B1]\n  q4: [B2]\n```\n"
	renderAuditDoc(t, outDir, "hc_03b_quad_dish_4.png", quadDishSrc, 760, 520, false, 720)

	// 4 & 5. Docked Toolbar (scrolled doc at 1365x872) & Split Pane Active Indicator (Light & Dark)
	root := filepath.Join(t.TempDir(), "hc_vault")
	v, err := vault.Open(root)
	if err != nil {
		t.Fatalf("vault.Open: %v", err)
	}
	_ = v.SeedTemplates(defaultTemplateSeeds())
	longDocSrc := "---\ntitle: HAMA 水凝胶双靶向抗菌及 CFU 涂板实验\ndate: 2026-11-24\nstatus: in_progress\n---\n\n" +
		"## 1. 实验设计概要\n\n" +
		"本实验评估 GelMA/HAMA 复合水凝胶在体外对耐甲氧西林金黄色葡萄球菌的抑菌活性与剂量依赖性梯度稀释结果。\n\n" +
		"#@ STEP 1 · 材料预备与分组\n" +
		"- [x] 超净台紫外灭菌 30 min\n" +
		"- [ ] 配制 10 mg/mL HAMA 储备液并 0.22 um 滤膜除菌\n\n" +
		"```asterlab\nkind: group_table\nid: gt-main\nschema: 1\ntitle: 实验分组矩阵\nrows:\n" +
		"  - id: g-ctrl\n    name: PBS 空白对照\n    role: control\n    condition: 无菌 PBS\n    independent_n: 3\n" +
		"  - id: g-tx\n    name: HAMA-Van 复合组\n    role: treatment\n    condition: 20 ug/mL\n    independent_n: 3\n```\n\n" +
		"#@ STEP 2 · 96 孔板倍比梯度稀释\n" +
		"- [ ] 每孔预加 180 uL TSB 肉汤液体培养基\n" +
		"- [ ] 第 1 列加入原液，排枪逐列混匀转移 20 uL\n\n" +
		"```asterlab\n# @vessel: plate96\nkind: calculator\nid: sd-main\nschema: 1\ntitle: 96 孔板逐级稀释流程\n" +
		"mode: serial_dilution\ndilution_factor: 10\nsteps_count: 6\nvolume_per_unit:\n  value: 200\n  unit: uL\n```\n\n" +
		"#@ STEP 3 · 涂板培养与菌落计数（滚动后不得被工具栏遮挡）\n" +
		"- [ ] 各梯度吸取 100 uL 均匀涂布至 TSA 琼脂平板\n" +
		"- [ ] 37°C 倒置培养 18 h 后拍照计数\n\n" +
		"```asterlab\nkind: plate\nid: pl-scrolled\nschema: 1\ntitle: 涂板菌落分布\nformat: 12\ntopology: dish\nrows: 3\ncolumns: 4\ngroups:\n  g-ctrl: [A1, A2, A3, A4]\n  g-tx: [B1, B2, B3, B4]\n```\n"
	expMain, err := v.CreateExperiment("", "HAMA 水凝胶双靶向抗菌实验", "2026-11-24", longDocSrc)
	if err != nil {
		t.Fatalf("CreateExperiment main: %v", err)
	}
	expAlt, err := v.CreateExperiment("", "CCK-8 细胞毒性复核", "2026-11-24", ctxSrc)
	if err != nil {
		t.Fatalf("CreateExperiment alt: %v", err)
	}
	ds := vault.NewDocStore(v)
	sh := &shell.Shell{
		TodayDate:        "2026-11-24",
		SelectedDate:     "2026-11-24",
		SelectedTreePath: expMain.RelPath,
		AppVersion:       "0.1.10-devdebug-r11",
		LiquidGlass:      true,
	}
	a := &app{
		shell:      sh,
		vault:      v,
		vaultIndex: vault.NewVaultIndex(v),
		docStore:   ds,
	}
	sh.RenderTabContent = func(c *ui.Context, pane *shell.PaneNode, tab shell.WorkspaceTab) {
		a.renderVaultTabContent(c, pane, tab)
	}
	sh.EnsureTabs()
	sh.OpenFileInPane(sh.ActivePaneID, expMain.RelPath, expMain.Title, shell.ModeEdit, true)

	renderVisualPNGSteps(t, outDir, "hc_04a_docked_toolbar_scrolled_light.png", 1365, 872, false, func(c *ui.Context) {
		sh.RenderSplitView(c)
	}, func(st *ui.Tester) {
		st.Frame()
		for i := 0; i < 6; i++ {
			st.Wheel(750, 480, 0, -140)
			st.Frame()
		}
		st.Advance(220 * time.Millisecond)
		st.Frame()
	})
	renderVisualPNGSteps(t, outDir, "hc_04b_docked_toolbar_scrolled_dark.png", 1365, 872, true, func(c *ui.Context) {
		sh.RenderSplitView(c)
	}, func(st *ui.Tester) {
		st.Frame()
		for i := 0; i < 6; i++ {
			st.Wheel(750, 480, 0, -140)
			st.Frame()
		}
		st.Advance(220 * time.Millisecond)
		st.Frame()
	})

	// 5. Split Pane Active Accent Top Bar (Light & Dark)
	sh.SplitPaneWithTab(sh.ActivePaneID, shell.WorkspaceTab{
		Kind: shell.TabFile, Title: expAlt.Title, FilePath: expAlt.RelPath, Mode: shell.ModeEdit,
	}, "right")
	renderVisualPNGSteps(t, outDir, "hc_05a_split_pane_light.png", 1365, 872, false, func(c *ui.Context) {
		sh.RenderSplitView(c)
	}, func(st *ui.Tester) {
		st.Frame()
		st.Advance(220 * time.Millisecond)
		st.Frame()
	})
	renderVisualPNGSteps(t, outDir, "hc_05b_split_pane_dark.png", 1365, 872, true, func(c *ui.Context) {
		sh.RenderSplitView(c)
	}, func(st *ui.Tester) {
		st.Frame()
		st.Advance(220 * time.Millisecond)
		st.Frame()
	})
}

func TestStage3UIKitAndVivariumVisualAudit(t *testing.T) {
	outDir := os.Getenv("ASTERLAB_VISUAL_OUT")
	if outDir == "" {
		outDir = t.TempDir()
	} else if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatalf("mkdir visual out: %v", err)
	}

	// 1. UI Kit Component Gallery (Light & Dark)
	galleryLight := &kit.GalleryState{SampleNum: 12, SelectedIdx: 1}
	renderVisualPNGSteps(t, outDir, "s3_01a_kit_gallery_light.png", 1120, 860, false, func(c *ui.Context) {
		kit.RenderComponentGallery(c, galleryLight)
	}, func(st *ui.Tester) {
		st.Frame()
		st.Advance(180 * time.Millisecond)
		st.Frame()
	})
	galleryDark := &kit.GalleryState{SampleNum: 12, SelectedIdx: 1}
	renderVisualPNGSteps(t, outDir, "s3_01b_kit_gallery_dark.png", 1120, 860, true, func(c *ui.Context) {
		kit.RenderComponentGallery(c, galleryDark)
	}, func(st *ui.Tester) {
		st.Frame()
		st.Advance(180 * time.Millisecond)
		st.Frame()
	})

	// 2. Pantry -> Vivarium Cage Rack (Light & Dark) + Edit Dialog (Light)
	w := tools.New(tools.Callbacks{})
	w.VivariumCages = []tools.VivariumCage{
		{
			ID:            "viv-a1",
			CageCode:      "SPF-M101",
			Species:       labsticker.AnimalMouseWhite,
			Strain:        "BALB/c",
			Genotype:      "WT",
			MaleCount:     4,
			FemaleCount:   0,
			FacilityGrade: "SPF",
			Room:          "屏障 302",
			RackPos:       "IVC-A1-01",
			DOB:           "2026-10-10",
			WeightG:       22.5,
			IACUCNo:       "IACUC-2026-088",
			Vendor:        "维通利华",
			CertNo:        "SCXK-2026-0011",
			IDMethod:      "耳标",
			AnimalIDs:     "#01~#04",
			Status:        "in_study",
			ExperimentRef: "HAMA 凝胶皮下移植瘤实验",
			HealthNotes:   "体温饮水正常，皮下移植 7 天状态稳定",
		},
		{
			ID:            "viv-a2",
			CageCode:      "SPF-M204",
			Species:       labsticker.AnimalMouseBlack,
			Strain:        "C57BL/6J",
			Genotype:      "Rosa26-CreERT2 +/-",
			MaleCount:     3,
			FemaleCount:   4,
			FacilityGrade: "SPF",
			Room:          "屏障 302",
			RackPos:       "IVC-B2-04",
			DOB:           "2026-09-25",
			WeightG:       24.8,
			IACUCNo:       "IACUC-2026-091",
			Vendor:        "集萃药康",
			Status:        "breeding",
			HealthNotes:   "含 2 只待分笼子代，触发 GB 14925 单笼密度复核提醒",
		},
		{
			ID:            "viv-a3",
			CageCode:      "SPF-R012",
			Species:       labsticker.AnimalRat,
			Strain:        "SD 大鼠",
			Genotype:      "WT",
			MaleCount:     2,
			FemaleCount:   0,
			FacilityGrade: "SPF",
			Room:          "大鼠屏障 201",
			RackPos:       "Rack-R1-02",
			DOB:           "2026-08-20",
			WeightG:       265.0,
			IACUCNo:       "IACUC-2026-104",
			Vendor:        "斯贝福",
			Status:        "active",
			ExperimentRef: "骨缺损水凝胶修复",
			HealthNotes:   "毛色洁白顺滑，精神活跃",
		},
		{
			ID:            "viv-a4",
			CageCode:      "AQ-ZF08",
			Species:       labsticker.AnimalZebrafish,
			Strain:        "AB 品系斑马鱼",
			Genotype:      "Tg(fli1a:EGFP)",
			MaleCount:     6,
			FemaleCount:   6,
			FacilityGrade: "aquatic",
			Room:          "水生房 A",
			RackPos:       "Tank-Z3-08",
			DOB:           "2026-07-01",
			WeightG:       0.6,
			IACUCNo:       "IACUC-2026-112",
			Status:        "in_study",
			ExperimentRef: "血管新生荧光活体观察",
		},
	}

	renderVisualPNGSteps(t, outDir, "s3_02a_vivarium_rack_light.png", 1280, 860, false, func(c *ui.Context) {
		tools.ViewPantry(c, w)
	}, func(st *ui.Tester) {
		st.Frame()
		st.Click("动物房")
		st.Advance(200 * time.Millisecond)
		st.Frame()
		if !st.HasText("SPF-M101") {
			t.Fatalf("expected vivarium cage SPF-M101 after clicking 动物房 tab, texts=%v", st.Texts())
		}
	})

	renderVisualPNGSteps(t, outDir, "s3_02b_vivarium_rack_dark.png", 1280, 860, true, func(c *ui.Context) {
		tools.ViewPantry(c, w)
	}, func(st *ui.Tester) {
		st.Frame()
		st.Advance(200 * time.Millisecond)
		st.Frame()
	})

	// 3. Vivarium IACUC + ARRIVE 2.0 + GB 14925-2023 Form Dialog
	w.OpenEditVivariumDialog(w.VivariumCages[0])
	renderVisualPNGSteps(t, outDir, "s3_03_vivarium_dialog_light.png", 1120, 840, false, func(c *ui.Context) {
		tools.ViewPantry(c, w)
	}, func(st *ui.Tester) {
		st.Frame()
		st.Advance(200 * time.Millisecond)
		st.Frame()
		if !st.HasText("编辑笼卡 · SPF-M101") {
			t.Fatalf("expected vivarium edit dialog open, texts=%v", st.Texts())
		}
	})

	// 4 & 5. Command Palette (Dark, Keycap + ListItem + ElevationModal) & Settings Density (Light)
	root := filepath.Join(t.TempDir(), "s3_vault")
	v, err := vault.Open(root)
	if err != nil {
		t.Fatalf("vault.Open: %v", err)
	}
	_ = v.SeedTemplates(defaultTemplateSeeds())
	exp1, err := v.CreateExperiment("", "BALB-c 小鼠肿瘤模型接种记录", "2026-11-24", "---\ntitle: BALB/c 小鼠肿瘤模型\n---\n\n#@ STEP 1 · 接种与称重\n")
	if err != nil {
		t.Fatalf("CreateExperiment: %v", err)
	}
	ds := vault.NewDocStore(v)
	sh := &shell.Shell{
		TodayDate:        "2026-11-24",
		SelectedDate:     "2026-11-24",
		SelectedTreePath: exp1.RelPath,
		AppVersion:       "0.1.10-devdebug-r12",
		LiquidGlass:      true,
		Density:          theme.DensityDefault,
	}
	a := &app{
		shell:      sh,
		vault:      v,
		vaultIndex: vault.NewVaultIndex(v),
		docStore:   ds,
	}
	sh.RenderTabContent = func(c *ui.Context, pane *shell.PaneNode, tab shell.WorkspaceTab) {
		a.renderVaultTabContent(c, pane, tab)
	}
	sh.EnsureTabs()
	sh.OpenFileInPane(sh.ActivePaneID, exp1.RelPath, exp1.Title, shell.ModeEdit, true)

	sh.PaletteOpen = true
	renderVisualPNGSteps(t, outDir, "s3_04_palette_keycaps_dark.png", 1280, 820, true, func(c *ui.Context) {
		sh.RenderSplitView(c)
	}, func(st *ui.Tester) {
		st.Frame()
		st.Advance(200 * time.Millisecond)
		st.Frame()
	})

	sh.PaletteOpen = false
	sh.VaultSettingsOpen = true
	renderVisualPNGSteps(t, outDir, "s3_05_settings_density_light.png", 1160, 780, false, func(c *ui.Context) {
		sh.RenderSplitView(c)
	}, func(st *ui.Tester) {
		st.Frame()
		st.Click("外观")
		st.Advance(200 * time.Millisecond)
		st.Frame()
		if !st.HasText("界面密度") {
			t.Fatalf("expected 界面密度 in Appearance settings, texts=%v", st.Texts())
		}
	})
}
