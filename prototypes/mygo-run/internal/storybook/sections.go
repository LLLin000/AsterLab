package storybook

import (
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/onevra/source/icons/lucide"

	"asterlab-mygo-prototype/internal/asterdoc"
	"asterlab-mygo-prototype/internal/shell"
	"asterlab-mygo-prototype/internal/theme"
	"asterlab-mygo-prototype/internal/tools"
	"asterlab-mygo-prototype/internal/uikit/kit"
	"asterlab-mygo-prototype/internal/uikit/labsticker"
)

const storybookAsterDocSample = `---
title: AsterML v2 全量模块速览（只读标本）
date: 2026-11-24
status: in_progress
---

#@ STEP 1 · 实验分组与核心配方
- [x] 完成超净台灭菌与 PBS 预热
- [ ] 配制 10% GelMA 与双靶向抗菌工作液

` + "```asterlab" + `
kind: group_table
id: sb-gt
schema: 1
title: 受试实验分组表
rows:
  - id: g-ctrl
    name: PBS 空白对照
    role: control
    condition: 无菌 PBS
    independent_n: 3
  - id: g-tx
    name: HAMA-Van 复合水凝胶
    role: treatment
    condition: 20 ug/mL
    independent_n: 3
` + "```" + `

` + "```asterlab" + `
kind: param_table
id: sb-pt
schema: 1
title: 关键操作参数与实测偏离
rows:
  - key: temp
    label: 孵育温度
    value: 37.0
    unit: °C
  - key: uv
    label: 光交联时间
    value: 45
    unit: s
` + "```" + `

` + "```asterlab" + `
kind: timer
id: sb-tm
schema: 1
title: 紫外光交联倒计时
mode: countdown
status: idle
duration:
  value: 45
  unit: min
` + "```" + `

` + "```asterlab" + `
kind: data_table
id: sb-dt
schema: 1
title: 抑菌圈双径原始测定表
columns:
  - key: group
    label: 分组
    value_kind: text
  - key: d1
    label: 横径
    value_kind: number
    unit: mm
  - key: d2
    label: 纵径
    value_kind: number
    unit: mm
rows:
  - {group: "PBS 对照", d1: 6.0, d2: 6.0}
  - {group: "HAMA-Van", d1: 18.4, d2: 18.2}
` + "```" + `

` + "```asterlab" + `
# @vessel: plate96
kind: calculator
id: sb-sd96
schema: 1
title: 96 孔板 10 倍连续梯度稀释（与孔板联动）
mode: serial_dilution
stock:
  value: 1000000
  unit: CFU/mL
dilution_factor: 10
steps_count: 5
volume_per_unit:
  value: 200
  unit: uL
` + "```" + `

` + "```asterlab" + `
kind: figure
id: sb-fig
schema: 1
title: 菌落抑菌圈模式示意图（暗色模式自动 0.86 柔光）
caption: 悬停图片恢复 1.0 亮度
svg: |
  <svg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 260 110'>
    <rect width='260' height='110' rx='8' fill='#F8FAFC'/>
    <circle cx='78' cy='55' r='34' fill='#FEF3C7' stroke='#B45309' stroke-width='2'/>
    <circle cx='78' cy='55' r='7' fill='#92400E'/>
    <circle cx='182' cy='55' r='34' fill='#DBEAFE' stroke='#1D4ED8' stroke-width='2'/>
    <circle cx='182' cy='55' r='16' fill='#FFFFFF' stroke='#2563EB' stroke-width='1.5'/>
  </svg>
` + "```" + `
`

// BuildExtraSections returns Sections 5-7 (AsterDoc modules, Vivarium & Edge cases, Editorial & Steward)
// shared by visual regression tests and the standalone Storybook desktop app.
func BuildExtraSections() []kit.StorySection {
	editorialSample := `---
title: L929 CCK-8 细胞活力测定（批注与 @ 联动标本）
date: 2026-11-25
status: in_progress
---

#@ STEP 1 · 细胞接种铺板
按每孔 5,000 个细胞接种至 [@96孔板](#sb-ed-plate)，加入 [@DMEM 培养基](asterlab://pantry?code=R-001) 至每孔 100 uL。若先前使用 [@旧批号胰酶](asterlab://pantry?code=R-003) 消化需注意细胞贴壁状态，最外圈 36 孔建议做边缘封闭保护[^1]。

#@ STEP 2 · 梯度加药与显色
参考 [[Fig4/Van-Lys-抗菌测定.asterml|Van-Lys 抗菌测定]] 配制浓度梯度后，配制好的 [@CCK-8 试剂盒](asterlab://pantry?code=R-002) 混合工作液注意避光操作[^2]，37°C 孵育后在 450 nm 测定吸光度[^3]。

` + "```asterlab" + `
kind: group_table
id: sb-ed-gt
schema: 1
title: 实验分组
rows:
  - {id: g1, name: 对照组, role: control, independent_n: 3}
` + "```" + `

` + "```asterlab" + `
kind: plate
id: sb-ed-plate
schema: 1
title: 96孔板
format: 12
groups:
  g1: [A1, A2, A3]
` + "```" + `

[^1]: 边缘孔仅加等体积无菌 PBS，防 48h 蒸发边缘效应。
[^2]: CCK-8 避光分装，37°C 孵育 1~1.5h 勿超 2h。
[^3]: 若对照组 OD450 > 2.0 需先用完全培养基 1:1 稀释重测。
`

	sbPantry := []asterdoc.PantryMentionItem{
		{ID: "p-dmem", Name: "DMEM 培养基", LabelCode: "R-001", Location: "4°C 上层", ItemType: "reagent", Status: "normal"},
		{ID: "p-cck8", Name: "CCK-8 试剂盒", LabelCode: "R-002", Location: "-20°C A1", ItemType: "reagent", Status: "near_expiry"},
		{ID: "p-old", Name: "旧批号胰酶", LabelCode: "R-003", Location: "-20°C B2", ItemType: "reagent", Status: "expired"},
	}

	var parsedEditorial *asterdoc.Document
	var wideEditorialView, narrowEditorialView *asterdoc.View
	if docEd, err := asterdoc.Parse(editorialSample); err == nil {
		parsedEditorial = docEd
		wideEditorialView = &asterdoc.View{
			Doc:                    parsedEditorial,
			Mode:                   asterdoc.ModeDesign,
			UnifiedLive:            true,
			ShowRightSidenotes:     true,
			SidenoteWidth:          224,
			PantryCompletionItems:  sbPantry,
			VaultCompletionTargets: []string{"Fig4/Van-Lys-抗菌测定.asterml"},
			OnOpenVaultLink:        func(string) {},
			OnLocatePantryCode:     func(string) {},
			Allowed:                map[string]bool{asterdoc.PermDocumentEdit: true},
		}
		narrowEditorialView = &asterdoc.View{
			Doc:                    parsedEditorial,
			Mode:                   asterdoc.ModeDesign,
			UnifiedLive:            true,
			ShowRightSidenotes:     false,
			PantryCompletionItems:  sbPantry,
			VaultCompletionTargets: []string{"Fig4/Van-Lys-抗菌测定.asterml"},
			OnOpenVaultLink:        func(string) {},
			OnLocatePantryCode:     func(string) {},
			Allowed:                map[string]bool{asterdoc.PermDocumentEdit: true},
		}
	}

	var parsedDoc *asterdoc.Document
	var docView *asterdoc.View
	var parseErrStr string
	if doc, err := asterdoc.Parse(storybookAsterDocSample); err == nil {
		parsedDoc = doc
		docView = &asterdoc.View{
			Doc:         parsedDoc,
			Mode:        asterdoc.ModeDesign,
			UnifiedLive: true,
			Allowed:     map[string]bool{asterdoc.PermDocumentEdit: true},
		}
	} else {
		parseErrStr = err.Error()
	}

	sampleCages := []tools.VivariumCage{
		{
			ID: "sb-v1", CageCode: "SPF-M101", Species: labsticker.AnimalMouseWhite,
			Strain: "BALB/c", Genotype: "WT", MaleCount: 4, FemaleCount: 0,
			FacilityGrade: "SPF", Room: "屏障 302", RackPos: "IVC-A1-01",
			DOB: "2026-10-10", WeightG: 22.5, IACUCNo: "IACUC-2026-088",
			Status: "in_study", ExperimentRef: "HAMA 凝胶皮下移植瘤实验",
		},
		{
			ID: "sb-v2", CageCode: "SPF-M204", Species: labsticker.AnimalMouseBlack,
			Strain: "C57BL/6J", Genotype: "Rosa26-CreERT2 +/- · 超长基因型命名边界验证截断效果", MaleCount: 3, FemaleCount: 4,
			FacilityGrade: "SPF", Room: "屏障 302", RackPos: "IVC-B2-04",
			DOB: "2026-09-25", WeightG: 24.8, IACUCNo: "IACUC-2026-091",
			Status: "breeding",
		},
	}

	return []kit.StorySection{
		{
			ID:       "asterdoc-modules",
			Group:    "文档模块",
			NavTitle: "AsterDoc 文档模块",
			Title:    "5. AsterML v2 实验文档模块矩阵",
			Subtitle: "Live Preview 正文 · checklist · group_table · param_table · timer · data_table",
			Icon:     lucide.FileText(),
			Render: func(c *ui.Context, tok theme.Tokens) {
				card := ui.Column(c).FillWidth().Padding(float32(tok.Spacing12), float32(tok.Spacing16)).Gap(float32(tok.Spacing8))
				kit.ApplyElevation(card, tok, kit.ElevationCard)
				card.Children(func() {
					kit.SectionHeader(c, tok, lucide.FileText(), "5A. AsterDoc 单真源文档组件实战预览",
						"同源消费 .asterml 解析树，所见即所得", nil)
					if docView != nil {
						docView.Render(c)
					} else if parseErrStr != "" {
						kit.InlineBanner(c, tok, lucide.CircleAlert(), kit.BannerDanger, "Storybook 示例解析失败："+parseErrStr, "", nil)
					}
				})
			},
		},
		{
			ID:       "vivarium-edge",
			Group:    "边界压测",
			NavTitle: "动物房与边界压测",
			Title:    "6. 动物房笼卡与边界/错误压测",
			Subtitle: "GB 14925 单笼密度预警 · 超长中英文混排截断 · Danger/Empty 状态",
			Icon:     lucide.PawPrint(),
			Render: func(c *ui.Context, tok theme.Tokens) {
				card := ui.Column(c).FillWidth().Padding(float32(tok.Spacing12), float32(tok.Spacing16)).Gap(float32(tok.Spacing12))
				kit.ApplyElevation(card, tok, kit.ElevationCard)
				card.Children(func() {
					kit.SectionHeader(c, tok, lucide.PawPrint(), "6A. 动物房合规卡片与边界压测态",
						"含正常在养与超密度告警卡片对照", nil)
					tools.RenderVivariumPreviewCards(c, tok, sampleCages, time.Now())
					kit.InlineBanner(c, tok, lucide.TriangleAlert(), kit.BannerWarning,
						"SPF-M204 当前存栏 7 只（超出 GB 14925-2023 小鼠单笼 ≤ 5 只推荐上限），请安排分笼。", "", nil)
					kit.InlineBanner(c, tok, lucide.CircleAlert(), kit.BannerDanger,
						"保存失败模拟：目标 Vault 目录仅只读或处于 Low Mandatory Level 受限路径，请检查目录权限。", "重试保存", func() {})
				})
			},
		},
		{
			ID:       "editorial-steward",
			Group:    "编辑与管家",
			NavTitle: "批注、@引用与管家",
			Title:    "7. 极简 Markdown 边注、@实体胶囊与隐形实验管家",
			Subtitle: "标准 [^1] Tufte 右边注 · @试剂(正常/临期/过期)/组件/文档胶囊 · 窄屏上标降级 · 关联体检抽屉",
			Icon:     lucide.MessageSquarePlus(),
			Render: func(c *ui.Context, tok theme.Tokens) {
				cardA := ui.Column(c).FillWidth().Padding(float32(tok.Spacing12), float32(tok.Spacing16)).Gap(float32(tok.Spacing12))
				kit.ApplyElevation(cardA, tok, kit.ElevationCard)
				cardA.Children(func() {
					kit.SectionHeader(c, tok, lucide.MessageSquarePlus(),
						"7A. 宽屏编辑态：极简 Tufte 右侧边注（默认展开 · Y 轴防碰撞）+ @ 行内药丸",
						"隐藏底部 [^1]: 定义区，悬停边注才露出折叠/删除；光标移入该行自动展开源码", nil)
					if wideEditorialView != nil {
						wideEditorialView.RenderFloatingToolbar(c, false)
						wideEditorialView.Render(c)
					}
				})

				cardB := ui.Column(c).FillWidth().Padding(float32(tok.Spacing12), float32(tok.Spacing16)).Gap(float32(tok.Spacing12))
				kit.ApplyElevation(cardB, tok, kit.ElevationCard)
				cardB.Children(func() {
					kit.SectionHeader(c, tok, lucide.Sparkles(),
						"7B. 窄屏/开抽屉自动降级（上标气泡预览）× 隐形实验管家「关联体检」抽屉",
						"右侧抽屉展开或窗口变窄时自动收起侧边注，保留行内角标与体检卡片", nil)
					ui.Row(c).FillWidth().AlignItems(ui.Start).Gap(float32(tok.Spacing16)).Children(func() {
						ui.Column(c).Grow(1).MinWidth(0).Padding(float32(tok.Spacing12)).
							Radius(float32(tok.ControlRadius)).Background(tok.Canvas).
							Border(1, tok.BorderSubtle).Children(func() {
							if narrowEditorialView != nil {
								narrowEditorialView.Render(c)
							}
						})
						ui.Box(c).Width(float32(tok.Spacing48)*7.2).Shrink(0).Children(func() {
							shell.RenderStewardPreviewPanel(c, tok)
						})
					})
				})
			},
		},
	}
}

// BuildAllSections returns Sections 4-8 (Shell, AsterDoc modules, Vivarium & Edge cases,
// Editorial & Steward, and Mobile 402dp iOS Spec) for the standalone UI Gallery app.
func BuildAllSections() []kit.StorySection {
	sh := &shell.Shell{TodayDate: time.Now().Format("2006-01-02")}
	out := make([]kit.StorySection, 0, 5)
	out = append(out, sh.BuiltinShellStorySection())
	out = append(out, BuildExtraSections()...)
	out = append(out, sh.BuiltinMobileStorySection())
	return out
}
