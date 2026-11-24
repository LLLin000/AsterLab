package main

import (
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/onevra/source/icons/lucide"

	"asterlab-mygo-prototype/internal/asterdoc"
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

func (a *app) buildStorybookExtraSections() []kit.StorySection {
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
	}
}
