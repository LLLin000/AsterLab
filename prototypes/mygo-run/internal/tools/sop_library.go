package tools

import (
	"fmt"
	"strings"

	"asterlab-mygo-prototype/internal/asterdoc"
	"asterlab-mygo-prototype/internal/theme"
	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/onevra/source/components"
)

// SOPEntry represents a reusable standard laboratory protocol authored in
// unified AsterML v2 syntax inside 资料与工具 → 标准方案库.
type SOPEntry struct {
	ID       string
	Title    string
	Category string
	Summary  string
	Source   string
}

var fallbackBuiltInSOPs = []SOPEntry{
	{
		ID:       "sop-rt-qpcr",
		Title:    "RT-qPCR 多靶点相对定量标准方案",
		Category: "分子生物学",
		Summary:  "96 孔板三复孔 × 靶基因正交排板、SYBR Green 预混液余量自动配液及 ΔΔCt 记录表",
		Source: "```asterlab\nkind: experiment\nid: exp-sop-qpcr\nschema: 1\ntitle: RT-qPCR 多靶点相对定量标准方案\nobjective: 评估处理组与对照组关键基因相对表达水平，标配 NTC 无模板对照与三复孔体系。\nprimary_endpoint: 基因相对表达量 (2^-ddCt)\n```\n\n```asterlab\nkind: group_table\nid: gt-sop-qpcr\nschema: 1\ntitle: qPCR 标准分组对照\nrows:\n  - {id: g-ctrl, name: 空白对照, role: control, independent_n: 3, technical_repeats: 3}\n  - {id: g-treat, name: 干预实验组, role: treatment, independent_n: 3, technical_repeats: 3}\n  - {id: g-ntc, name: 无模板阴参 (NTC), role: control, independent_n: 1, technical_repeats: 3}\n```\n\n```asterlab\nkind: step\nid: step-sop-qpcr-1\nschema: 1\nexecution: executable\ntitle: 01 SYBR 预混液配制与 96 孔加样\npath: sop/qpcr/01\nblocks:\n  - kind: prose\n    id: blk-sop-qpcr-note\n    schema: 1\n    title: 防气溶胶与避光规范\n    tone: tip\n    body: 使用带滤芯吸头分装 MasterMix；含荧光染料管全程冰盒避光。\n  - kind: calculator\n    id: blk-sop-qpcr-calc\n    schema: 1\n    title: 预混液体系测算 (20 uL/孔)\n    mode: group_mastermix\n    container_ref: plate-sop-qpcr\n    volume_per_unit: {value: 20, unit: uL}\n    excess_ratio: 0.15\n    components:\n      - {name: 2× SYBR Green 预混液, volume_per_unit: {value: 10, unit: uL}}\n      - {name: 上下游引物 (10 uM), volume_per_unit: {value: 1.6, unit: uL}}\n      - {name: 无核酸酶水, volume_per_unit: {value: 6.4, unit: uL}}\n  - kind: plate\n    id: plate-sop-qpcr\n    schema: 1\n    title: 96 孔 qPCR 标准排板\n    format: 96\n    groups:\n      g-ctrl: [A1, A2, A3]\n      g-treat: [B1, B2, B3]\n      g-ntc: [H1, H2, H3]\n    samples: {A1: Ctrl, A2: Ctrl, A3: Ctrl, B1: Treat, B2: Treat, B3: Treat, H1: NTC, H2: NTC, H3: NTC}\n    roles: {H1: ntc, H2: ntc, H3: ntc}\n```\n",
	},
	{
		ID:       "sop-inhibition-zone",
		Title:    "90 mm 琼脂平皿四象限抑菌圈测定",
		Category: "微生物与抗菌",
		Summary:  "符合 CLSI 间距规范（1 mm = 2.4 px 纯矢量 SVG 示意图 + 游标卡尺十字双径计算表）",
		Source: "```asterlab\nkind: experiment\nid: exp-sop-iz\nschema: 1\ntitle: 90 mm 琼脂平皿四象限抑菌圈测定\nobjective: 依据 CLSI 琼脂扩散法定量评估抗菌样片抑菌活性。\nprimary_endpoint: 抑菌圈平均直径 (mm)\n```\n\n```asterlab\nkind: group_table\nid: gt-sop-iz\nschema: 1\ntitle: 四象限分组设计\nrows:\n  - {id: g-pbs, name: Q1 空白 PBS, role: control, independent_n: 3}\n  - {id: g-sample, name: Q2 待测样片, role: treatment, independent_n: 3}\n  - {id: g-pos, name: Q3 阳性对照 (Van), role: comparator, independent_n: 3}\n```\n\n```asterlab\nkind: step\nid: step-sop-iz-1\nschema: 1\nexecution: executable\ntitle: 01 90 mm 平板涂布与药敏贴片\npath: sop/iz/01\nblocks:\n  - kind: figure\n    id: blk-sop-iz-svg\n    schema: 1\n    title: 90 mm 琼脂皿四象限排布参考\n    caption: 标准比例尺 1 mm = 2.4 px（滤纸片 6 mm；片心距 ≥ 24 mm）\n    max_height: 240\n    svg: |\n      <svg viewBox=\"0 0 360 280\" xmlns=\"http://www.w3.org/2000/svg\">\n        <g transform=\"translate(180, 140)\">\n          <circle cx=\"0\" cy=\"0\" r=\"108\" fill=\"none\" stroke=\"currentColor\" stroke-width=\"2.5\"/>\n          <circle cx=\"0\" cy=\"0\" r=\"102\" fill=\"none\" stroke=\"currentColor\" stroke-width=\"0.9\" stroke-dasharray=\"4 4\"/>\n          <line x1=\"-102\" y1=\"0\" x2=\"102\" y2=\"0\" stroke=\"currentColor\" stroke-width=\"1.1\" stroke-dasharray=\"6 4\"/>\n          <line x1=\"0\" y1=\"-102\" x2=\"0\" y2=\"102\" stroke=\"currentColor\" stroke-width=\"1.1\" stroke-dasharray=\"6 4\"/>\n          <circle cx=\"-44\" cy=\"-44\" r=\"7.2\" fill=\"none\" stroke=\"currentColor\" stroke-width=\"1.6\"/>\n          <circle cx=\"44\" cy=\"-44\" r=\"20\" fill=\"none\" stroke=\"currentColor\" stroke-width=\"1.2\" stroke-dasharray=\"3 3\"/>\n          <circle cx=\"44\" cy=\"-44\" r=\"7.2\" fill=\"none\" stroke=\"currentColor\" stroke-width=\"1.8\"/>\n          <circle cx=\"-44\" cy=\"44\" r=\"28\" fill=\"none\" stroke=\"currentColor\" stroke-width=\"1.4\" stroke-dasharray=\"3 3\"/>\n          <circle cx=\"-44\" cy=\"44\" r=\"7.2\" fill=\"none\" stroke=\"currentColor\" stroke-width=\"2.0\"/>\n        </g>\n        <text x=\"16\" y=\"28\" font-size=\"11\" text-anchor=\"start\">Q1 空白 PBS</text>\n        <text x=\"344\" y=\"28\" font-size=\"11\" text-anchor=\"end\">Q2 待测样片</text>\n        <text x=\"16\" y=\"254\" font-size=\"11\" text-anchor=\"start\">Q3 阳性对照 (+)</text>\n      </svg>\n```\n",
	},
}

func (w *Workspace) availableSOPs() []SOPEntry {
	var list []SOPEntry
	if w.Callbacks.ListSOPs != nil {
		list = append(list, w.Callbacks.ListSOPs()...)
	}
	if len(list) == 0 {
		list = append(list, fallbackBuiltInSOPs...)
	}
	return list
}

func (w *Workspace) activeSOPEntry() (SOPEntry, []SOPEntry) {
	list := w.availableSOPs()
	if len(list) == 0 {
		return SOPEntry{}, nil
	}
	for _, it := range list {
		if it.ID == w.ActiveSOPID {
			return it, list
		}
	}
	w.ActiveSOPID = list[0].ID
	return list[0], list
}

func (w *Workspace) ensureSOPSheetView(entry SOPEntry) *asterdoc.View {
	if entry.ID == "" || strings.TrimSpace(entry.Source) == "" {
		return nil
	}
	if w.sopView != nil && w.sopViewKey == entry.ID && w.sopView.Doc != nil && w.sopView.Doc.Source == entry.Source {
		return w.sopView
	}
	doc, err := asterdoc.Parse(entry.Source)
	if err != nil {
		w.sopDocErr = err.Error()
		return nil
	}
	w.sopDocErr = ""
	w.sopView = &asterdoc.View{
		Doc:      doc,
		Registry: asterdoc.BuiltinRegistry(),
		Mode:     asterdoc.ModeProtocol,
		ReadOnly: true,
	}
	w.sopViewKey = entry.ID
	return w.sopView
}

func sopLibraryView(c *ui.Context, w *Workspace, tok theme.Tokens) {
	t := c.Theme()
	active, list := w.activeSOPEntry()

	ui.Row(c).FillWidth().Wrap().AlignItems(ui.Center).Gap(t.Space(2)).Children(func() {
		ui.Column(c).Grow(1).MinWidth(0).Gap(float32(tok.Spacing4)*0.5).Children(func() {
			ui.Text(c, "标准方案库 (SOP)").FontSize(float32(tok.TextSection)).Bold().TextColor(tok.Text)
			ui.Text(c, "沉淀经验证的 AsterML v2 标准化方案；整篇渲染即所见，支持一键导出 PDF 或克隆为新课题设计。").
				FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
		})
		if active.ID != "" {
			if components.Button(c, "导出标准方案 PDF", components.ButtonSecondary).Clicked() {
				if w.Callbacks.ExportSOPPDF != nil {
					w.Callbacks.ExportSOPPDF(active.Title, active.Source)
					w.Error, w.Status = "", "已发起标准方案 PDF 导出"
				} else {
					w.Status = fmt.Sprintf("「%s」PDF 导出就绪", active.Title)
				}
			}
			if components.Button(c, "以此新建实验设计", components.ButtonPrimary).Clicked() {
				if w.Callbacks.CreateDesignFromSOP != nil {
					w.Callbacks.CreateDesignFromSOP(active.Title, active.Source)
					w.Error, w.Status = "", "已基于标准方案创建实验设计"
				}
			}
		}
	})

	// Horizontal selector cards for reusable standard protocols
	ui.Row(c).FillWidth().Wrap().Gap(float32(tok.Spacing8)).Children(func() {
		for _, item := range list {
			item := item
			selected := item.ID == active.ID
			bg := tok.Elevated
			border := tok.Border
			if selected {
				bg = tok.AccentSoftBg
				border = tok.Accent
			}
			card := ui.ButtonBase(c.Key("sop-item:"+item.ID)).Padding(float32(tok.Spacing8), float32(tok.Spacing12)).
				Radius(float32(tok.ControlRadius)).Background(bg).Border(1, border).Label(item.Title)
			card.Children(func() {
				ui.Column(c).Gap(float32(tok.Spacing4)*0.5).Children(func() {
					ui.Row(c).AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
						ui.Text(c, item.Title).FontSize(float32(tok.TextMeta)).Bold().TextColor(tok.Text)
						if item.Category != "" {
							components.Badge(c, item.Category)
						}
					})
					if item.Summary != "" {
						ui.Text(c, item.Summary).FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted).MaxWidth(float32(tok.Spacing48)*5.5)
					}
				})
			})
			if card.Clicked() {
				w.ActiveSOPID = item.ID
				w.Error, w.Status = "", ""
			}
		}
	})

	if active.ID == "" {
		ui.Box(c).FillWidth().Padding(float32(tok.Spacing16)).Radius(float32(tok.ControlRadius)).
			Background(tok.Elevated).Border(1, tok.Border).Children(func() {
			ui.Text(c, "暂无可用标准方案。").FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
		})
		return
	}

	view := w.ensureSOPSheetView(active)
	if w.sopDocErr != "" {
		ui.Row(c).FillWidth().Padding(float32(tok.Spacing8), float32(tok.Spacing12)).
			Radius(float32(tok.ControlRadius)).Background(tok.DangerBg).Border(1, tok.Danger).Children(func() {
			ui.Text(c, "标准方案解析失败："+w.sopDocErr).FontSize(float32(tok.TextMeta)).TextColor(tok.DangerText)
		})
		return
	}
	if view != nil {
		view.RenderDocumentSheet(c)
	}
}
