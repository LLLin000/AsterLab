package tools

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"asterlab-mygo-prototype/internal/theme"
	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/onevra/source/components"
)

// MasterMixComponent describes one single-reaction component row in the PCR MasterMix calculator.
type MasterMixComponent struct {
	Name       string
	PerWellUL  string
	IsTemplate bool
}

// BenchCalcState holds inputs for the 4-in-1 wet-lab bench calculator.
type BenchCalcState struct {
	Mode int // 0=摩尔配母液, 1=带单位稀释, 2=细胞计数铺板, 3=PCR 预混液

	// Mode 0: Molar stock prep (m = C * V * MW)
	SoluteName string
	MolarConc  string
	MolarUnit  string // M, mM, uM
	MolVol     string
	MolVolUnit string // mL, uL
	MolWeight  string // g/mol

	// Mode 1: Unit-aware C1*V1 = C2*V2 dilution
	DilName      string
	DilStockVal  string
	DilStockUnit string // M, mM, uM, nM, mg/mL, ug/mL, ng/mL
	DilFinalVal  string
	DilFinalUnit string
	DilFinalVol  string
	DilVolUnit   string // mL, uL

	// Mode 2: Cell seeding density calculator
	CellName      string
	CellStock     string // e.g. "120"
	CellStockUnit string // "×10⁴/mL", "×10⁶/mL"
	CellsPerWell  string // e.g. "5000"
	WellCount     string // e.g. "24"
	VolPerWellUL  string // e.g. "500"
	CellExcessPct string // e.g. "15"

	// Mode 3: PCR MasterMix scaling
	MMTitle      string
	MMReactions  string
	MMExcessPct  string
	MMComponents []MasterMixComponent
}

func defaultBenchCalcState() BenchCalcState {
	return BenchCalcState{
		Mode:          0,
		SoluteName:    "万古霉素母液",
		MolarConc:     "10",
		MolarUnit:     "mM",
		MolVol:        "5",
		MolVolUnit:    "mL",
		MolWeight:     "1449.25",
		DilName:       "VEGF 刺激工作液",
		DilStockVal:   "100",
		DilStockUnit:  "ug/mL",
		DilFinalVal:   "50",
		DilFinalUnit:  "ng/mL",
		DilFinalVol:   "4",
		DilVolUnit:    "mL",
		CellName:      "HUVEC 原代细胞",
		CellStock:     "120",
		CellStockUnit: "×10⁴/mL",
		CellsPerWell:  "15000",
		WellCount:     "12",
		VolPerWellUL:  "500",
		CellExcessPct: "15",
		MMTitle:       "RT-qPCR SYBR 预混体系 (20 uL)",
		MMReactions:   "24",
		MMExcessPct:   "15",
		MMComponents: []MasterMixComponent{
			{Name: "2× SYBR Green Premix", PerWellUL: "10"},
			{Name: "正向引物 (10 uM)", PerWellUL: "0.8"},
			{Name: "反向引物 (10 uM)", PerWellUL: "0.8"},
			{Name: "无核酸酶水", PerWellUL: "6.4"},
			{Name: "cDNA 模板（每孔单独加）", PerWellUL: "2.0", IsTemplate: true},
		},
	}
}

func (w *Workspace) ensureBenchCalcInit() {
	if w.BenchCalc.MolarUnit == "" {
		w.BenchCalc = defaultBenchCalcState()
	}
}

func molarFactor(unit string) (float64, bool) {
	switch unit {
	case "M":
		return 1, true
	case "mM":
		return 1e-3, true
	case "uM", "µM":
		return 1e-6, true
	case "nM":
		return 1e-9, true
	default:
		return 0, false
	}
}

func massConcFactorMgPerML(unit string) (float64, bool) {
	switch unit {
	case "mg/mL":
		return 1, true
	case "ug/mL", "µg/mL":
		return 1e-3, true
	case "ng/mL":
		return 1e-6, true
	default:
		return 0, false
	}
}

func volFactorML(unit string) (float64, bool) {
	switch unit {
	case "mL":
		return 1, true
	case "uL", "µL":
		return 1e-3, true
	default:
		return 0, false
	}
}

// ComputeMolarStock calculates solute mass in mg from target molarity, volume and MW.
func ComputeMolarStock(concVal float64, concUnit string, volVal float64, volUnit string, mw float64) (massMg float64, volML float64, warn string, err error) {
	cf, okC := molarFactor(concUnit)
	vf, okV := volFactorML(volUnit)
	if !okC || !okV || concVal <= 0 || volVal <= 0 || mw <= 0 {
		return 0, 0, "", fmt.Errorf("请输入大于 0 的目标浓度、配制体积与分子量 (MW)")
	}
	volML = volVal * vf
	volL := volML * 1e-3
	massG := (concVal * cf) * volL * mw
	massMg = massG * 1000.0
	if massMg < 1.0 && massMg > 0 {
		factor := 2.0 / massMg
		suggestedVol := volML * factor
		warn = fmt.Sprintf("称量量不足 1 mg 易受分析天平误差影响：建议称取 2.00 mg 溶于 %.2f mL 配成 %.4g× 浓缩母液后再稀释", suggestedVol, factor)
	}
	return massMg, volML, warn, nil
}

// ComputeUnitDilution solves C1*V1 = C2*V2 cleanly across mismatched molar or mass-concentration units.
func ComputeUnitDilution(c1 float64, c1Unit string, c2 float64, c2Unit string, v2 float64, v2Unit string) (v1Out float64, dilOut float64, unitOut string, err error) {
	if c1 <= 0 || c2 <= 0 || v2 <= 0 {
		return 0, 0, "", fmt.Errorf("浓度与终体积必须大于 0")
	}
	if _, okV := volFactorML(v2Unit); !okV {
		return 0, 0, "", fmt.Errorf("不支持的体积单位 %s", v2Unit)
	}
	f1M, isM1 := molarFactor(c1Unit)
	f2M, isM2 := molarFactor(c2Unit)
	f1W, isW1 := massConcFactorMgPerML(c1Unit)
	f2W, isW2 := massConcFactorMgPerML(c2Unit)

	var ratio float64
	switch {
	case isM1 && isM2:
		ratio = (c2 * f2M) / (c1 * f1M)
	case isW1 && isW2:
		ratio = (c2 * f2W) / (c1 * f1W)
	default:
		return 0, 0, "", fmt.Errorf("母液单位 (%s) 与终浓度单位 (%s) 维度不一致，请同选摩尔浓度或同选质量浓度", c1Unit, c2Unit)
	}
	if ratio > 1.0+1e-9 {
		return 0, 0, "", fmt.Errorf("目标终浓度不能高于母液浓度")
	}
	v1Out = v2 * ratio
	dilOut = v2 - v1Out
	if dilOut < 0 {
		dilOut = 0
	}
	return v1Out, dilOut, v2Unit, nil
}

// ComputeCellSeeding calculates suspension volume & extra medium for cell seeding across N wells.
func ComputeCellSeeding(stock float64, stockUnit string, cellsPerWell, wells, volPerWellUL, excessPct float64) (suspML float64, mediaML float64, totalML float64, err error) {
	if stock <= 0 || cellsPerWell <= 0 || wells <= 0 || volPerWellUL <= 0 || excessPct < 0 {
		return 0, 0, 0, fmt.Errorf("请输入合法的细胞密度、孔数、单孔体积与余量百分比")
	}
	cellsPerML := stock * 1e4
	if stockUnit == "×10⁶/mL" {
		cellsPerML = stock * 1e6
	}
	effWells := wells * (1.0 + excessPct/100.0)
	totalCells := cellsPerWell * effWells
	totalML = (volPerWellUL * effWells) / 1000.0
	suspML = totalCells / cellsPerML
	mediaML = totalML - suspML
	if mediaML < -1e-6 {
		return 0, 0, 0, fmt.Errorf("所需细胞悬液 (%.2f mL) 超出总体系 (%.2f mL)，请离心重悬提高原悬液密度", suspML, totalML)
	}
	if mediaML < 0 {
		mediaML = 0
	}
	return suspML, mediaML, totalML, nil
}

// MasterMixRow summarizes one scaled reagent volume for PCR MasterMix prep.
type MasterMixRow struct {
	Name       string
	PerWellUL  float64
	TotalUL    float64
	IsTemplate bool
}

// ComputeMasterMix scales each per-reaction component by N * (1 + excessPct/100).
func ComputeMasterMix(reactions, excessPct float64, comps []MasterMixComponent) (rows []MasterMixRow, masterMixUL, singleReactionUL float64, err error) {
	if reactions <= 0 || excessPct < 0 {
		return nil, 0, 0, fmt.Errorf("反应孔数必须大于 0 且余量不能为负")
	}
	eff := reactions * (1.0 + excessPct/100.0)
	for _, cp := range comps {
		name := strings.TrimSpace(cp.Name)
		v, pErr := strconv.ParseFloat(strings.TrimSpace(cp.PerWellUL), 64)
		if name == "" || pErr != nil || v <= 0 {
			continue
		}
		singleReactionUL += v
		tot := v * eff
		if cp.IsTemplate {
			tot = 0
		} else {
			masterMixUL += tot
		}
		rows = append(rows, MasterMixRow{
			Name:       name,
			PerWellUL:  v,
			TotalUL:    tot,
			IsTemplate: cp.IsTemplate,
		})
	}
	if len(rows) == 0 {
		return nil, 0, 0, fmt.Errorf("请至少填妥一行有效体系组分及体积")
	}
	return rows, masterMixUL, singleReactionUL, nil
}

func formatScaledVolume(valML float64) string {
	if valML < 0.1 {
		return fmt.Sprintf("%.2f uL (%.4f mL)", valML*1000.0, valML)
	}
	return fmt.Sprintf("%.3f mL (%.1f uL)", valML, valML*1000.0)
}

func benchUnitSelect(c *ui.Context, tok theme.Tokens, ptr *string, opts []string, w float32) {
	ui.Box(c).Width(w).Children(func() {
		sel := components.Select(c, ptr)
		sel.Trigger.Children(func() {
			ui.Text(c, *ptr).FontSize(float32(tok.TextMeta)).TextColor(tok.Text).FontFeatures("tnum")
		})
		sel.Popup(func(panel ui.Element) {
			for _, o := range opts {
				o := o
				sel.Item(o).Children(func() {
					ui.Text(c, o).FontSize(float32(tok.TextMeta)).TextColor(tok.Text).FontFeatures("tnum")
				})
			}
		})
	})
}

func dilutionView(c *ui.Context, w *Workspace, tok theme.Tokens) {
	w.ensureBenchCalcInit()
	st := &w.BenchCalc

	ui.Row(c).FillWidth().Wrap().AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
		ui.Column(c).Grow(1).MinWidth(0).Gap(float32(tok.Spacing4)*0.5).Children(func() {
			ui.Text(c, "实验台速算（4 合 1 配液中心）").FontSize(float32(tok.TextSection)).Bold().TextColor(tok.Text)
			ui.Text(c, "摩尔母液称量、跨单位 C₁V₁=C₂V₂ 稀释、细胞铺板配平与 qPCR 预混液放大；计算结果可一键插入实验步骤。").
				FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
		})
		components.Tabs(c, &st.Mode, components.TabsSegmented, "摩尔配母液", "单位稀释", "细胞铺板", "PCR 预混液")
	})

	var insertBlock map[string]any
	idSuffix := fmt.Sprintf("bench-%d", time.Now().Unix()%100000)

	switch st.Mode {
	case 0:
		ui.Row(c).FillWidth().Wrap().Gap(float32(tok.Spacing8)).Children(func() {
			ui.Column(c).Grow(1).MinWidth(float32(tok.Spacing48)*3).Gap(float32(tok.Spacing4)).Children(func() {
				ui.Text(c, "溶质 / 药物名称").FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
				components.TextBox(c, &st.SoluteName, components.TextBoxOptions{}).Placeholder("如 万古霉素")
			})
			ui.Column(c).Width(float32(tok.Spacing48)*2.5).Gap(float32(tok.Spacing4)).Children(func() {
				ui.Text(c, "分子量 MW (g/mol)").FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
				components.TextBox(c, &st.MolWeight, components.TextBoxOptions{}).Placeholder("如 1449.25")
			})
		})
		ui.Row(c).FillWidth().Wrap().Gap(float32(tok.Spacing8)).Children(func() {
			ui.Column(c).Grow(1).MinWidth(float32(tok.Spacing48)*3).Gap(float32(tok.Spacing4)).Children(func() {
				ui.Text(c, "目标母液摩尔浓度").FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
				ui.Row(c).FillWidth().Gap(float32(tok.Spacing4)).Children(func() {
					ui.Box(c).Grow(1).MinWidth(0).Children(func() { components.TextBox(c, &st.MolarConc, components.TextBoxOptions{}) })
					benchUnitSelect(c, tok, &st.MolarUnit, []string{"M", "mM", "uM"}, float32(tok.Spacing48)*1.7)
				})
			})
			ui.Column(c).Grow(1).MinWidth(float32(tok.Spacing48)*3).Gap(float32(tok.Spacing4)).Children(func() {
				ui.Text(c, "配制终体积").FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
				ui.Row(c).FillWidth().Gap(float32(tok.Spacing4)).Children(func() {
					ui.Box(c).Grow(1).MinWidth(0).Children(func() { components.TextBox(c, &st.MolVol, components.TextBoxOptions{}) })
					benchUnitSelect(c, tok, &st.MolVolUnit, []string{"mL", "uL"}, float32(tok.Spacing48)*1.7)
				})
			})
		})
		cVal, _ := strconv.ParseFloat(strings.TrimSpace(st.MolarConc), 64)
		vVal, _ := strconv.ParseFloat(strings.TrimSpace(st.MolVol), 64)
		mwVal, _ := strconv.ParseFloat(strings.TrimSpace(st.MolWeight), 64)
		massMg, volML, warn, err := ComputeMolarStock(cVal, st.MolarUnit, vVal, st.MolVolUnit, mwVal)
		renderBenchResultBox(c, tok, err, warn, func() {
			massStr := fmt.Sprintf("%.2f mg", massMg)
			if massMg >= 1000 {
				massStr = fmt.Sprintf("%.3f g (%.1f mg)", massMg/1000.0, massMg)
			} else if massMg < 1 {
				massStr = fmt.Sprintf("%.3f mg (%.1f ug)", massMg, massMg*1000.0)
			}
			ui.Text(c, fmt.Sprintf("精确称取 %s → 溶于 %s %s 定容（m = C·V·MW）", massStr, st.MolVol, st.MolVolUnit)).
				FontSize(float32(tok.TextBody)).Bold().TextColor(tok.Accent).FontFeatures("tnum")
			_ = volML
		}, "插入配液表到设计", func() {
			insertBlock = map[string]any{
				"kind": "param_table", "id": idSuffix + "-mol", "schema": 1,
				"title": strings.TrimSpace(st.SoluteName) + " 母液配制参数",
				"rows": []any{
					map[string]any{"key": "stock_conc", "label": "目标母液浓度", "value": cVal, "unit": st.MolarUnit},
					map[string]any{"key": "prep_vol", "label": "定容体积", "value": vVal, "unit": st.MolVolUnit},
					map[string]any{"key": "solute_mass", "label": "应称取溶质质量", "value": math.Round(massMg*1000) / 1000, "unit": "mg"},
				},
			}
			w.insertBlockToDesign(insertBlock)
		})

	case 1:
		ui.Row(c).FillWidth().Wrap().Gap(float32(tok.Spacing8)).Children(func() {
			ui.Column(c).Grow(1).MinWidth(float32(tok.Spacing48)*4).Gap(float32(tok.Spacing4)).Children(func() {
				ui.Text(c, "试剂 / 工作液名称").FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
				components.TextBox(c, &st.DilName, components.TextBoxOptions{})
			})
		})
		ui.Row(c).FillWidth().Wrap().Gap(float32(tok.Spacing8)).Children(func() {
			ui.Column(c).Grow(1).MinWidth(float32(tok.Spacing48)*3).Gap(float32(tok.Spacing4)).Children(func() {
				ui.Text(c, "母液浓度 (C₁)").FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
				ui.Row(c).FillWidth().Gap(float32(tok.Spacing4)).Children(func() {
					ui.Box(c).Grow(1).MinWidth(0).Children(func() { components.TextBox(c, &st.DilStockVal, components.TextBoxOptions{}) })
					benchUnitSelect(c, tok, &st.DilStockUnit, []string{"mg/mL", "ug/mL", "ng/mL", "M", "mM", "uM", "nM"}, float32(tok.Spacing48)*2.1)
				})
			})
			ui.Column(c).Grow(1).MinWidth(float32(tok.Spacing48)*3).Gap(float32(tok.Spacing4)).Children(func() {
				ui.Text(c, "目标终浓度 (C₂)").FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
				ui.Row(c).FillWidth().Gap(float32(tok.Spacing4)).Children(func() {
					ui.Box(c).Grow(1).MinWidth(0).Children(func() { components.TextBox(c, &st.DilFinalVal, components.TextBoxOptions{}) })
					benchUnitSelect(c, tok, &st.DilFinalUnit, []string{"mg/mL", "ug/mL", "ng/mL", "M", "mM", "uM", "nM"}, float32(tok.Spacing48)*2.1)
				})
			})
			ui.Column(c).Grow(1).MinWidth(float32(tok.Spacing48)*3).Gap(float32(tok.Spacing4)).Children(func() {
				ui.Text(c, "配制终体积 (V₂)").FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
				ui.Row(c).FillWidth().Gap(float32(tok.Spacing4)).Children(func() {
					ui.Box(c).Grow(1).MinWidth(0).Children(func() { components.TextBox(c, &st.DilFinalVol, components.TextBoxOptions{}) })
					benchUnitSelect(c, tok, &st.DilVolUnit, []string{"mL", "uL"}, float32(tok.Spacing48)*1.6)
				})
			})
		})
		c1, _ := strconv.ParseFloat(strings.TrimSpace(st.DilStockVal), 64)
		c2, _ := strconv.ParseFloat(strings.TrimSpace(st.DilFinalVal), 64)
		v2, _ := strconv.ParseFloat(strings.TrimSpace(st.DilFinalVol), 64)
		v1, dilVol, vUnit, err := ComputeUnitDilution(c1, st.DilStockUnit, c2, st.DilFinalUnit, v2, st.DilVolUnit)
		renderBenchResultBox(c, tok, err, "", func() {
			v1UL := v1
			if vUnit == "mL" {
				v1UL = v1 * 1000.0
			}
			ui.Text(c, fmt.Sprintf("吸取母液：%.4g %s（%.2f uL） · 补加稀释液：%.4g %s", v1, vUnit, v1UL, dilVol, vUnit)).
				FontSize(float32(tok.TextBody)).Bold().TextColor(tok.Accent).FontFeatures("tnum")
		}, "插入稀释计算器到设计", func() {
			w.insertBlockToDesign(map[string]any{
				"kind":         "calculator",
				"id":           idSuffix + "-dil",
				"schema":       1,
				"title":        firstNonEmptyBench(st.DilName, "工作液稀释配液"),
				"mode":         "dilution",
				"stock":        map[string]any{"value": c1, "unit": st.DilStockUnit},
				"target":       map[string]any{"value": c2, "unit": st.DilFinalUnit},
				"final_volume": map[string]any{"value": v2, "unit": st.DilVolUnit},
			})
		})

	case 2:
		ui.Row(c).FillWidth().Wrap().Gap(float32(tok.Spacing8)).Children(func() {
			ui.Column(c).Grow(1).MinWidth(float32(tok.Spacing48)*3).Gap(float32(tok.Spacing4)).Children(func() {
				ui.Text(c, "细胞名称").FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
				components.TextBox(c, &st.CellName, components.TextBoxOptions{})
			})
			ui.Column(c).Grow(1).MinWidth(float32(tok.Spacing48)*3).Gap(float32(tok.Spacing4)).Children(func() {
				ui.Text(c, "计数原始细胞密度").FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
				ui.Row(c).FillWidth().Gap(float32(tok.Spacing4)).Children(func() {
					ui.Box(c).Grow(1).MinWidth(0).Children(func() { components.TextBox(c, &st.CellStock, components.TextBoxOptions{}) })
					benchUnitSelect(c, tok, &st.CellStockUnit, []string{"×10⁴/mL", "×10⁶/mL"}, float32(tok.Spacing48)*2.1)
				})
			})
		})
		ui.Row(c).FillWidth().Wrap().Gap(float32(tok.Spacing8)).Children(func() {
			ui.Column(c).Grow(1).MinWidth(float32(tok.Spacing48)*2.5).Gap(float32(tok.Spacing4)).Children(func() {
				ui.Text(c, "目标细胞数 / 孔").FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
				components.TextBox(c, &st.CellsPerWell, components.TextBoxOptions{})
			})
			ui.Column(c).Grow(1).MinWidth(float32(tok.Spacing48)*2).Gap(float32(tok.Spacing4)).Children(func() {
				ui.Text(c, "铺板孔数").FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
				components.TextBox(c, &st.WellCount, components.TextBoxOptions{})
			})
			ui.Column(c).Grow(1).MinWidth(float32(tok.Spacing48)*2.2).Gap(float32(tok.Spacing4)).Children(func() {
				ui.Text(c, "每孔体系 (uL)").FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
				components.TextBox(c, &st.VolPerWellUL, components.TextBoxOptions{})
			})
			ui.Column(c).Grow(1).MinWidth(float32(tok.Spacing48)*2).Gap(float32(tok.Spacing4)).Children(func() {
				ui.Text(c, "移液余量 (%)").FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
				components.TextBox(c, &st.CellExcessPct, components.TextBoxOptions{})
			})
		})
		cs, _ := strconv.ParseFloat(strings.TrimSpace(st.CellStock), 64)
		cpw, _ := strconv.ParseFloat(strings.TrimSpace(st.CellsPerWell), 64)
		wc, _ := strconv.ParseFloat(strings.TrimSpace(st.WellCount), 64)
		vpw, _ := strconv.ParseFloat(strings.TrimSpace(st.VolPerWellUL), 64)
		ex, _ := strconv.ParseFloat(strings.TrimSpace(st.CellExcessPct), 64)
		suspML, mediaML, totalML, err := ComputeCellSeeding(cs, st.CellStockUnit, cpw, wc, vpw, ex)
		renderBenchResultBox(c, tok, err, "", func() {
			ui.Text(c, fmt.Sprintf("取原细胞悬液：%s  +  完全培养基：%.3f mL  =  总悬液 %.3f mL",
				formatScaledVolume(suspML), mediaML, totalML)).
				FontSize(float32(tok.TextBody)).Bold().TextColor(tok.Accent).FontFeatures("tnum")
		}, "插入铺板配液参数到设计", func() {
			w.insertBlockToDesign(map[string]any{
				"kind": "param_table", "id": idSuffix + "-cells", "schema": 1,
				"title": firstNonEmptyBench(st.CellName, "细胞计数铺板方案"),
				"rows": []any{
					map[string]any{"key": "seeding_density", "label": "每孔接种密度", "value": cpw, "unit": "cells/well"},
					map[string]any{"key": "susp_vol", "label": "所需原细胞悬液", "value": math.Round(suspML*1000) / 1000, "unit": "mL"},
					map[string]any{"key": "media_vol", "label": "补加完全培养基", "value": math.Round(mediaML*1000) / 1000, "unit": "mL"},
				},
			})
		})

	case 3:
		ui.Row(c).FillWidth().Wrap().Gap(float32(tok.Spacing8)).Children(func() {
			ui.Column(c).Grow(1).MinWidth(float32(tok.Spacing48)*3.5).Gap(float32(tok.Spacing4)).Children(func() {
				ui.Text(c, "预混液体系名称").FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
				components.TextBox(c, &st.MMTitle, components.TextBoxOptions{})
			})
			ui.Column(c).Width(float32(tok.Spacing48)*2).Gap(float32(tok.Spacing4)).Children(func() {
				ui.Text(c, "反应孔数 N").FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
				components.TextBox(c, &st.MMReactions, components.TextBoxOptions{})
			})
			ui.Column(c).Width(float32(tok.Spacing48)*2).Gap(float32(tok.Spacing4)).Children(func() {
				ui.Text(c, "死体积余量 (%)").FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
				components.TextBox(c, &st.MMExcessPct, components.TextBoxOptions{})
			})
		})
		for i := range st.MMComponents {
			row := &st.MMComponents[i]
			ui.Row(c).FillWidth().Gap(float32(tok.Spacing8)).AlignItems(ui.Center).Children(func() {
				ui.Box(c).Grow(2).MinWidth(0).Children(func() {
					components.TextBox(c, &row.Name, components.TextBoxOptions{}).Placeholder("组分名称")
				})
				ui.Box(c).Width(float32(tok.Spacing48)*2.2).Children(func() {
					components.TextBox(c, &row.PerWellUL, components.TextBoxOptions{}).Placeholder("单孔 uL")
				})
				components.Checkbox(c, &row.IsTemplate, "单孔独加(不进Mix)")
			})
		}
		if components.Button(c, "+ 增加组分行", components.ButtonGhost).Clicked() {
			st.MMComponents = append(st.MMComponents, MasterMixComponent{})
		}
		nVal, _ := strconv.ParseFloat(strings.TrimSpace(st.MMReactions), 64)
		exVal, _ := strconv.ParseFloat(strings.TrimSpace(st.MMExcessPct), 64)
		mmRows, mixTotal, singleTot, err := ComputeMasterMix(nVal, exVal, st.MMComponents)
		renderBenchResultBox(c, tok, err, "", func() {
			ui.Column(c).Gap(float32(tok.Spacing4)).Children(func() {
				ui.Text(c, fmt.Sprintf("MasterMix 总配制量：%.2f uL（单管 %.1f uL · 已含 %.0f%% 余量）", mixTotal, singleTot, exVal)).
					FontSize(float32(tok.TextBody)).Bold().TextColor(tok.Accent).FontFeatures("tnum")
				for _, r := range mmRows {
					line := fmt.Sprintf("· %s：%.2f uL（每孔 %.2f uL）", r.Name, r.TotalUL, r.PerWellUL)
					if r.IsTemplate {
						line = fmt.Sprintf("· %s：每孔单独加入 %.2f uL（不预混）", r.Name, r.PerWellUL)
					}
					ui.Text(c, line).FontSize(float32(tok.TextMeta)).TextColor(tok.Text).FontFeatures("tnum")
				}
			})
		}, "插入 MasterMix 计算器到设计", func() {
			var comps []any
			for _, r := range mmRows {
				if !r.IsTemplate {
					comps = append(comps, map[string]any{
						"name":            r.Name,
						"volume_per_unit": map[string]any{"value": r.PerWellUL, "unit": "uL"},
					})
				}
			}
			w.insertBlockToDesign(map[string]any{
				"kind":            "calculator",
				"id":              idSuffix + "-mm",
				"schema":          1,
				"title":           firstNonEmptyBench(st.MMTitle, "PCR MasterMix 放大配液"),
				"mode":            "group_mastermix",
				"sample_count":    int(math.Round(nVal)),
				"volume_per_unit": map[string]any{"value": singleTot, "unit": "uL"},
				"excess_ratio":    exVal / 100.0,
				"components":      comps,
			})
		})
	}
}

func renderBenchResultBox(c *ui.Context, tok theme.Tokens, err error, warn string, body func(), insertLabel string, onInsert func()) {
	ui.Column(c).FillWidth().Padding(float32(tok.Spacing12)).Radius(float32(tok.PanelRadius)).
		Background(tok.Elevated).Border(1, tok.Border).Gap(float32(tok.Spacing8)).Children(func() {
		if err != nil {
			ui.Text(c, err.Error()).FontSize(float32(tok.TextMeta)).TextColor(tok.DangerText)
			return
		}
		ui.Row(c).FillWidth().Wrap().AlignItems(ui.Center).Gap(float32(tok.Spacing12)).Children(func() {
			ui.Box(c).Grow(1).MinWidth(0).Children(body)
			if components.Button(c, insertLabel, components.ButtonPrimary).Clicked() && onInsert != nil {
				onInsert()
			}
		})
		if warn != "" {
			ui.Row(c).FillWidth().Padding(float32(tok.Spacing8), float32(tok.Spacing12)).
				Radius(float32(tok.ControlRadius)).Background(tok.WarningBg).Border(1, tok.Warning).Children(func() {
				ui.Text(c, warn).FontSize(float32(tok.TextMeta)).TextColor(tok.WarningText)
			})
		}
	})
}

func (w *Workspace) insertBlockToDesign(block map[string]any) {
	if len(block) == 0 {
		return
	}
	w.Error, w.Status = "", "已生成并插入当前实验设计步骤"
	if w.Callbacks.InsertAsterBlock != nil {
		w.Callbacks.InsertAsterBlock(block)
		return
	}
	w.handoff()
}

func firstNonEmptyBench(values ...string) string {
	for _, v := range values {
		if s := strings.TrimSpace(v); s != "" {
			return s
		}
	}
	return ""
}
