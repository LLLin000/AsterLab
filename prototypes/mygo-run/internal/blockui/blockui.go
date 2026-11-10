// Package blockui renders design blocks as reusable, data-bound modules. The
// same module serves Design (planned values, editable) and Run (recorded
// values, read-only), so both surfaces read and write one information source:
// the blocks.Instance stored in the design.
package blockui

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/onevra/source/components"

	"asterlab-mygo-prototype/internal/blocks"
	plateui "asterlab-mygo-prototype/internal/uikit/plate"
)

// Mode selects what a module shows. Design edits the instance's planned values;
// Run renders the values a frozen design carries, without offering edits.
type Mode uint8

const (
	// Design edits the planned values on the design's own block instance.
	Design Mode = iota
	// Run shows the frozen plan and leaves execution records to the Run
	// surface's validated readout batches; it never edits the plan.
	Run
	// Review compares the frozen plan with recorded results, read-only.
	Review
)

// ModeLabel names a mode for a module's status strip.
func ModeLabel(m Mode) string {
	switch m {
	case Run:
		return "冻结计划"
	case Review:
		return "对照"
	}
	return "计划"
}

// View renders one block instance as a module card and reports whether the
// user changed its props this frame. Callers own persistence.
func View(c *ui.Context, inst *blocks.Instance, mode Mode) bool {
	if inst == nil {
		return false
	}
	switch inst.Type {
	case blocks.TypeNote:
		return note(c, inst, mode)
	case blocks.TypeDilutionCalculator:
		return dilution(c, inst, mode)
	case blocks.TypePlate96:
		return plateModule(c, inst, mode)
	case blocks.TypeParameterTable:
		return parameters(c, inst, mode)
	case blocks.TypeChecklist:
		return checklist(c, inst, mode)
	case blocks.TypeMeasurementTable:
		return measurements(c, inst, mode)
	case blocks.TypeNestedWorkflow:
		card(c, "嵌套工作流", nil, func() {
			components.Text(c, "嵌套工作流模块尚未有原生视图。").TextColor(c.Theme().TextMuted).
				FontSize(c.Theme().Rem(.85))
		})
	}
	return false
}

// Title names a module in lists and its card header.
func Title(inst blocks.Instance) string { return TypeTitle(inst.Type) }

// TypeTitle names a module type before an instance exists.
func TypeTitle(t blocks.BlockType) string {
	switch t {
	case blocks.TypeNote:
		return "备注"
	case blocks.TypeDilutionCalculator:
		return "稀释计算器"
	case blocks.TypePlate96:
		return "96 孔板"
	case blocks.TypeParameterTable:
		return "参数表"
	case blocks.TypeChecklist:
		return "检查清单"
	case blocks.TypeMeasurementTable:
		return "读数表"
	case blocks.TypeNestedWorkflow:
		return "嵌套工作流"
	}
	return string(t)
}

// Addable lists the module types a design step can add today.
func Addable() []blocks.BlockType {
	return []blocks.BlockType{
		blocks.TypeNote, blocks.TypeDilutionCalculator, blocks.TypePlate96,
		blocks.TypeParameterTable, blocks.TypeChecklist,
	}
}

// NewDefinition builds a valid definition with empty defaults for a type.
func NewDefinition(t blocks.BlockType) blocks.Definition {
	def := blocks.Definition{ID: "module-" + string(t), Version: 1, Type: t}
	switch t {
	case blocks.TypeNote:
		def.Props = blocks.NoteProps{}
	case blocks.TypeDilutionCalculator:
		def.Props = blocks.DilutionProps{Input: blocks.DilutionInput{StockConc: 1, FinalConc: 1, FinalVol: 1}, Valid: true}
	case blocks.TypePlate96:
		def.Props = blocks.NewPlate().Props
	case blocks.TypeParameterTable:
		def.Props = blocks.ParameterTableProps{Rows: []blocks.ParamRow{}}
	case blocks.TypeChecklist:
		def.Props = blocks.ChecklistProps{Items: []blocks.ChecklistItem{}}
	case blocks.TypeMeasurementTable:
		def.Props = blocks.MeasurementTableProps{Columns: []string{}, Rows: []blocks.MeasurementRow{}}
	default:
		def.Props = blocks.NestedWorkflowProps{}
	}
	return def
}

// card is the module frame shared by every type: a muted heading, an optional
// trailing status, then the body, all on one bordered surface.
func card(c *ui.Context, title string, trailing func(), body func()) {
	t := c.Theme()
	ui.Column(c).FillWidth().Radius(float32(14)).Border(1, t.Border).Background(t.Surface).Children(func() {
		ui.Row(c).FillWidth().MinWidth(0).Padding(t.Space(2.5), t.Space(3), t.Space(1), t.Space(3)).
			Gap(t.Space(2)).AlignItems(ui.Center).Children(func() {
			components.Text(c, title).FontSize(t.Rem(.75)).Bold().TextColor(t.TextMuted).SingleLine()
			ui.Spacer(c)
			if trailing != nil {
				trailing()
			}
		})
		ui.Column(c).FillWidth().Padding(0, t.Space(3), t.Space(3), t.Space(3)).
			Gap(t.Space(2)).Children(body)
	})
}

// row places a label and description above its control, matching the inspector.
func row(c *ui.Context, label, desc string, control func()) {
	t := c.Theme()
	ui.Column(c).FillWidth().Gap(t.Space(1.5)).Children(func() {
		ui.Column(c).Gap(t.Space(.5)).Children(func() {
			components.Text(c, label).FontSize(t.Rem(.82)).FontWeight(500)
			if desc != "" {
				components.Text(c, desc).FontSize(t.Rem(.72)).TextColor(t.TextMuted)
			}
		})
		control()
	})
}

// write stores props on the instance, reporting whether anything changed.
func write(inst *blocks.Instance, props any) bool {
	raw, err := json.Marshal(props)
	if err != nil {
		return false
	}
	if string(raw) == string(inst.RawProps) {
		return false
	}
	inst.RawProps = raw
	return true
}

func note(c *ui.Context, inst *blocks.Instance, mode Mode) bool {
	t := c.Theme()
	var p blocks.NoteProps
	if inst.DecodeProps(&p) != nil {
		return false
	}
	changed := false
	card(c, Title(*inst), nil, func() {
		if mode != Design {
			if p.Title == "" && p.Text == "" {
				components.Text(c, "未记录内容").TextColor(t.TextMuted).FontSize(t.Rem(.85))
				return
			}
			if p.Title != "" {
				components.Text(c, p.Title).FontWeight(500)
			}
			components.Text(c, p.Text).TextColor(t.TextMuted)
			return
		}
		ui.Box(c).Key("note-title").Children(func() {
			before := p.Title
			components.TextBox(c, &p.Title, components.TextBoxOptions{}).
				Label("备注标题").Placeholder("标题").FillWidth()
			if p.Title != before {
				changed = true
			}
		})
		ui.Box(c).Key("note-text").Children(func() {
			before := p.Text
			components.TextBox(c, &p.Text, components.TextBoxOptions{Multiline: true}).
				Label("备注内容").Placeholder("记录这一步的目的、条件或注意事项").Lines(3, 6).FillWidth()
			if p.Text != before {
				changed = true
			}
		})
	})
	if changed && write(inst, p) {
		return true
	}
	return false
}

func dilution(c *ui.Context, inst *blocks.Instance, mode Mode) bool {
	t := c.Theme()
	var p blocks.DilutionProps
	if inst.DecodeProps(&p) != nil {
		return false
	}
	changed := false
	stock, final, volume := p.Input.StockConc, p.Input.FinalConc, p.Input.FinalVol
	if p.Input.StockConc <= 0 {
		stock = 0
	}
	card(c, Title(*inst), func() {
		if mode == Design {
			components.Text(c, "只计算，不写入计划值").TextColor(t.TextMuted).FontSize(t.Rem(.72))
		}
	}, func() {
		if mode != Design {
			components.Text(c, fmt.Sprintf("母液 %g → 目标 %g · 终体积 %g µL",
				p.Input.StockConc, p.Input.FinalConc, p.Input.FinalVol)).TextColor(t.TextMuted)
			components.Text(c, fmt.Sprintf("需取母液 %.1f µL · 稀释液 %.1f µL",
				p.Result.StockVol, p.Result.DiluentVol)).FontFeatures("tnum")
			return
		}
		row(c, "母液浓度", "与目标浓度同一单位", func() {
			if components.NumberInput(c, &stock, 0, 1e6, 0.1).
				Label("母液浓度").FillWidth().Changed() {
				changed = true
			}
		})
		row(c, "目标浓度", "与母液浓度同一单位", func() {
			if components.NumberInput(c, &final, 0, 1e6, 0.1).
				Label("目标浓度").FillWidth().Changed() {
				changed = true
			}
		})
		row(c, "终体积", "单位 µL", func() {
			if components.NumberInput(c, &volume, 0, 1e6, 1).
				Label("终体积").FillWidth().Changed() {
				changed = true
			}
		})
		if changed {
			p.Input = blocks.DilutionInput{StockConc: stock, FinalConc: final, FinalVol: volume}
			if res, err := blocks.CalculateDilution(p.Input); err == nil {
				p.Result, p.Valid, p.Error = res, true, ""
			} else {
				p.Valid, p.Error = false, err.Error()
			}
		}
		if p.Valid {
			ui.Divider(c)
			ui.Row(c).FillWidth().Children(func() {
				components.Text(c, "需取母液").TextColor(t.TextMuted)
				ui.Spacer(c)
				components.Text(c, fmt.Sprintf("%.1f µL", p.Result.StockVol)).FontFeatures("tnum")
			})
			ui.Row(c).FillWidth().Children(func() {
				components.Text(c, "稀释液").TextColor(t.TextMuted)
				ui.Spacer(c)
				components.Text(c, fmt.Sprintf("%.1f µL", p.Result.DiluentVol)).FontFeatures("tnum")
			})
		} else if p.Error != "" {
			components.Text(c, p.Error).TextColor(t.Danger).FontSize(t.Rem(.8))
		}
	})
	if changed && write(inst, p) {
		return true
	}
	return false
}

func plateModule(c *ui.Context, inst *blocks.Instance, mode Mode) bool {
	var p blocks.Plate96Props
	if inst.DecodeProps(&p) != nil {
		return false
	}
	if p.Rows == 0 {
		p.Rows = 8
	}
	if p.Cols == 0 {
		p.Cols = 12
	}
	changed := false
	wells := make(map[string]plateui.WellState, len(p.Selected))
	for _, idx := range p.Selected {
		wells[wellID(idx, p.Cols)] = plateui.WellState{
			Group: p.Groups[idx], SampleID: p.Labels[idx],
		}
	}
	selected := make([]string, 0, len(p.Selected))
	for _, idx := range p.Selected {
		selected = append(selected, wellID(idx, p.Cols))
	}
	sort.Strings(selected)
	card(c, Title(*inst), func() {
		components.Text(c, fmt.Sprintf("%d 孔", len(p.Selected))).TextColor(c.Theme().TextMuted).
			FontSize(c.Theme().Rem(.75)).FontFeatures("tnum")
	}, func() {
		ui.Box(c).FillWidth().AlignItems(ui.Center).Children(func() {
			plateui.View(c, plateui.Props{
				Wells:       wells,
				SelectedIDs: selected,
				Interactive: mode == Design,
				OnSelect: func(ids []string) {
					next := make([]int, 0, len(ids))
					for _, id := range ids {
						if idx, ok := wellIndex(id, p.Cols); ok {
							next = append(next, idx)
						}
					}
					sort.Ints(next)
					p.Selected = next
					changed = true
				},
			})
		})
		if mode == Design {
			components.Text(c, "选择孔位只记录在本模块；计划组别与样本仍由检测的孔位映射维护。").
				TextColor(c.Theme().TextMuted).FontSize(c.Theme().Rem(.72))
		}
	})
	if changed && write(inst, p) {
		return true
	}
	return false
}

func parameters(c *ui.Context, inst *blocks.Instance, mode Mode) bool {
	t := c.Theme()
	var p blocks.ParameterTableProps
	if inst.DecodeProps(&p) != nil {
		return false
	}
	changed := false
	card(c, Title(*inst), func() {
		components.Text(c, fmt.Sprintf("%d 项", len(p.Rows))).TextColor(t.TextMuted).
			FontSize(t.Rem(.75)).FontFeatures("tnum")
	}, func() {
		if len(p.Rows) == 0 {
			components.Text(c, "尚无参数").TextColor(t.TextMuted).FontSize(t.Rem(.85))
		}
		for i := range p.Rows {
			i := i
			ui.Row(c).FillWidth().Gap(t.Space(2)).AlignItems(ui.Center).Children(func() {
				ui.Box(c).Key(fmt.Sprintf("key-%d", i)).Grow(1).MinWidth(0).Children(func() {
					if mode != Design {
						components.Text(c, labelOr(p.Rows[i].Key, "未命名参数"))
						return
					}
					before := p.Rows[i].Key
					components.TextBox(c, &p.Rows[i].Key, components.TextBoxOptions{}).
						Label("参数名").Placeholder("参数名").FillWidth()
					if p.Rows[i].Key != before {
						changed = true
					}
				})
				ui.Box(c).Key(fmt.Sprintf("value-%d", i)).Width(90).Shrink(0).Children(func() {
					if mode != Design {
						components.Text(c, p.Rows[i].Value).FontFeatures("tnum")
						return
					}
					before := p.Rows[i].Value
					components.TextBox(c, &p.Rows[i].Value, components.TextBoxOptions{}).
						Label("值").Placeholder("值").FillWidth()
					if p.Rows[i].Value != before {
						changed = true
					}
				})
				ui.Box(c).Key(fmt.Sprintf("unit-%d", i)).Width(64).Shrink(0).Children(func() {
					if mode != Design {
						components.Text(c, p.Rows[i].Unit).TextColor(t.TextMuted)
						return
					}
					before := p.Rows[i].Unit
					components.TextBox(c, &p.Rows[i].Unit, components.TextBoxOptions{}).
						Label("单位").Placeholder("单位").FillWidth()
					if p.Rows[i].Unit != before {
						changed = true
					}
				})
			})
		}
		if mode == Design {
			ui.Row(c).Children(func() {
				if components.Button(c, "添加参数", components.ButtonGhost).Clicked() {
					p.Rows = append(p.Rows, blocks.ParamRow{})
					changed = true
				}
				ui.Spacer(c)
			})
		} else {
			components.Text(c, "执行完成状态记录在运行事件中，不写回冻结计划。").
				TextColor(t.TextMuted).FontSize(t.Rem(.72))
		}
	})
	if changed && write(inst, p) {
		return true
	}
	return false
}

func checklist(c *ui.Context, inst *blocks.Instance, mode Mode) bool {
	t := c.Theme()
	var p blocks.ChecklistProps
	if inst.DecodeProps(&p) != nil {
		return false
	}
	changed := false
	card(c, Title(*inst), func() {
		done := 0
		for _, item := range p.Items {
			if item.Checked {
				done++
			}
		}
		components.Text(c, fmt.Sprintf("%d/%d", done, len(p.Items))).TextColor(t.TextMuted).
			FontSize(t.Rem(.75)).FontFeatures("tnum")
	}, func() {
		if len(p.Items) == 0 {
			components.Text(c, "尚无检查项").TextColor(t.TextMuted).FontSize(t.Rem(.85))
		}
		for i := range p.Items {
			i := i
			ui.Row(c).FillWidth().Gap(t.Space(2)).AlignItems(ui.Center).Children(func() {
				ui.Box(c).Key(fmt.Sprintf("check-%d", i)).Shrink(0).Children(func() {
					if components.Checkbox(c, &p.Items[i].Checked, "").Changed() {
						changed = true
					}
				})
				ui.Box(c).Key(fmt.Sprintf("text-%d", i)).Grow(1).MinWidth(0).Children(func() {
					if mode != Design {
						components.Text(c, labelOr(p.Items[i].Text, "未命名检查项"))
						return
					}
					before := p.Items[i].Text
					components.TextBox(c, &p.Items[i].Text, components.TextBoxOptions{}).
						Label("检查项").Placeholder("检查项").FillWidth()
					if p.Items[i].Text != before {
						changed = true
					}
				})
			})
		}
		if mode == Design {
			ui.Row(c).Children(func() {
				if components.Button(c, "添加检查项", components.ButtonGhost).Clicked() {
					p.Items = append(p.Items, blocks.ChecklistItem{ID: fmt.Sprintf("check-%d", len(p.Items)+1)})
					changed = true
				}
				ui.Spacer(c)
			})
		} else {
			components.Text(c, "执行完成状态记录在运行事件中，不写回冻结计划。").
				TextColor(t.TextMuted).FontSize(t.Rem(.72))
		}
	})
	if changed && write(inst, p) {
		return true
	}
	return false
}

func measurements(c *ui.Context, inst *blocks.Instance, mode Mode) bool {
	t := c.Theme()
	var p blocks.MeasurementTableProps
	if inst.DecodeProps(&p) != nil {
		return false
	}
	card(c, Title(*inst), func() {
		components.Text(c, fmt.Sprintf("%d 行", len(p.Rows))).TextColor(t.TextMuted).
			FontSize(t.Rem(.75)).FontFeatures("tnum")
	}, func() {
		if len(p.Rows) == 0 {
			components.Text(c, "尚无读数").TextColor(t.TextMuted).FontSize(t.Rem(.85))
		}
		for _, r := range p.Rows {
			ui.Row(c).FillWidth().Gap(t.Space(2)).Children(func() {
				components.Text(c, r.SampleID).Grow(1).MinWidth(0).SingleLine().Ellipsis("…")
				components.Text(c, fmt.Sprintf("%g %s", r.Value, r.Unit)).FontFeatures("tnum")
			})
		}
		if mode == Design {
			components.Text(c, "执行读数属于 Run；设计里只保留计划值。").
				TextColor(t.TextMuted).FontSize(t.Rem(.72))
		}
		if mode == Review && len(p.Rows) > 0 {
			ui.Divider(c)
			components.Text(c, summary(p.Rows)).FontFeatures("tnum")
		}
	})
	return false
}

// summary reports the recorded count and the mean and sample standard
// deviation across every recorded row, labelled so it cannot be read as a
// per-group result.
func summary(rows []blocks.MeasurementRow) string {
	n := float64(len(rows))
	var sum float64
	for _, r := range rows {
		sum += r.Value
	}
	mean := sum / n
	var sq float64
	for _, r := range rows {
		d := r.Value - mean
		sq += d * d
	}
	sd := 0.0
	if n > 1 {
		sd = math.Sqrt(sq / (n - 1))
	}
	return fmt.Sprintf("全部读数 %d 条 · 均值 %.3f · SD %.3f", len(rows), mean, sd)
}

func labelOr(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

var rowLabels = [8]string{"A", "B", "C", "D", "E", "F", "G", "H"}

func wellID(idx, cols int) string {
	row, col := idx/cols, idx%cols
	if row < 0 || row >= len(rowLabels) || col < 0 || col >= cols {
		return ""
	}
	return fmt.Sprintf("%s%d", rowLabels[row], col+1)
}

func wellIndex(id string, cols int) (int, bool) {
	if id == "" {
		return 0, false
	}
	row := strings.IndexByte(strings.Join(rowLabels[:], ""), id[0])
	col, err := strconv.Atoi(id[1:])
	if row < 0 || err != nil || col < 1 || col > cols {
		return 0, false
	}
	return row*cols + col - 1, true
}
