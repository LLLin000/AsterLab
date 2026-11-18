package asterdoc

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/onevra/source/components"
	"github.com/oneo-me/onevra/source/icons/lucide"

	"asterlab-mygo-prototype/internal/theme"
)

// TimerRuntimeState passes live timer execution status from RunWorkspace into asterdoc.View.
type TimerRuntimeState struct {
	TimerID     string
	Status      string // "idle" | "running" | "paused" | "completed"
	PlannedS    int
	RemainingS  int
	AccumulatedS int
	PhaseIndex  int
	RoundIndex  int
}

// AssetThumb describes a linked experimental file/image for attachment@1 in Run mode.
type AssetThumb struct {
	AssetID   string
	Label     string
	FilePath  string
	WellID    string
	FovIndex  int
}

func (v *View) renderChecklist(c *ui.Context, data map[string]any) {
	t := c.Theme()
	tok := theme.GetTokens(t)
	moduleID := textValue(data, "id")
	title := firstNonEmpty(textValue(data, "title"), "操作检查清单")
	items := mapSlice(data["items"])
	editableDesign := v.Mode == ModeDesign && !v.ReadOnly && v.Can(PermDocumentEdit)
	interactiveRun := v.Mode == ModeRun && !v.ReadOnly

	doneCount := 0
	for _, it := range items {
		cid := textValue(it, "id")
		if v.CheckedItems[cid] || v.CheckedItems[v.StepID+":"+cid] || v.SkippedItems[cid] != "" {
			doneCount++
		}
	}

	moduleCardFrame(v, c, lucide.SquareCheck(), title, fmt.Sprintf("%d / %d", doneCount, len(items)), nil, func() {
		ui.Column(c).FillWidth().Gap(float32(tok.Spacing8)).Children(func() {
			for idx, it := range items {
				idx := idx
				cid := textValue(it, "id")
				label := textValue(it, "label")
				required, _ := it["required"].(bool)
				allowSkip, _ := it["allow_rule_skip"].(bool)
				skipRule := textValue(it, "skip_rule")
				skipReason := v.SkippedItems[cid]

				key := cid
				if v.CheckedItems[v.StepID+":"+cid] {
					key = v.StepID + ":" + cid
				}
				checkedVal := v.CheckedItems[key]
				chkPtr := v.ensureWidgetBool("chk:" + moduleID + ":" + cid)
				if v.CheckedItems != nil {
					*chkPtr = checkedVal
				}

				ui.Row(c).FillWidth().Padding(float32(tok.Spacing8), float32(tok.Spacing8)).
					Radius(float32(tok.ControlRadius)).Background(tok.Elevated).Border(1, tok.Border).
					AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
					if interactiveRun && skipReason == "" {
						if components.Checkbox(c, chkPtr, label).Changed() {
							if v.CheckedItems == nil {
								v.CheckedItems = map[string]bool{}
							}
							v.CheckedItems[cid] = *chkPtr
							if v.OnToggleCheck != nil {
								v.OnToggleCheck(v.StepID, cid, *chkPtr)
							}
						}
					} else {
						mark := "○ "
						if *chkPtr {
							mark = "✓ "
						}
						ui.Text(c, mark+label).FontSize(float32(tok.TextBody)).TextColor(tok.Text)
					}
					ui.Spacer(c)
					if required {
						components.Badge(c, "必检", components.BadgeOptions{Tone: components.BadgeWarning})
					}
					if skipReason != "" {
						components.Badge(c, "已豁免 · "+skipReason, components.BadgeOptions{Tone: components.BadgeAccent})
					} else if allowSkip && interactiveRun && !*chkPtr {
						btnLabel := "按规则跳过"
						if skipRule != "" {
							btnLabel = "按规则跳过（" + skipRule + "）"
						}
						if components.Button(c, btnLabel, components.ButtonOutline).Clicked() && v.OnRuleSkip != nil {
							v.OnRuleSkip(v.StepID, cid, firstNonEmpty(skipRule, "rule_skip"))
						}
					} else if allowSkip && !interactiveRun {
						components.Badge(c, firstNonEmpty(skipRule, "可规则豁免"))
					}
					if editableDesign && len(items) > 1 {
						if components.IconButton(c, lucide.Trash(), "删除检查项", components.IconButtonCompact).Clicked() {
							next := removeSliceIndex(items, idx)
							v.emitAnyModuleEdit(moduleID, map[string]any{"items": next})
						}
					}
				})
			}

			if editableDesign {
				newLabel := v.ensureWidgetDraft("chk:"+moduleID+":new", "")
				reqBool := v.ensureWidgetBool("chk:" + moduleID + ":req")
				ui.Row(c).FillWidth().Gap(float32(tok.Spacing8)).AlignItems(ui.Center).Children(func() {
					ui.Box(c).FillWidth().Children(func() {
						components.TextBox(c, newLabel, components.TextBoxOptions{}).Placeholder("新增检查项描述…")
					})
					components.Checkbox(c, reqBool, "必检")
					if components.Button(c, "添加项", components.ButtonSecondary).Clicked() && strings.TrimSpace(*newLabel) != "" {
						next := append([]map[string]any{}, items...)
						next = append(next, map[string]any{
							"id":       fmt.Sprintf("chk-%s-%d", sanitizeIDFrag(*newLabel), len(items)+1),
							"label":    strings.TrimSpace(*newLabel),
							"required": *reqBool,
						})
						v.emitAnyModuleEdit(moduleID, map[string]any{"items": next})
						*newLabel = ""
					}
				})
			}
		})
	})
}

func (v *View) renderCalculator(c *ui.Context, data map[string]any) {
	t := c.Theme()
	tok := theme.GetTokens(t)
	moduleID := textValue(data, "id")
	title := firstNonEmpty(textValue(data, "title"), "科学计算器")
	mode := textValue(data, "mode")
	editableDesign := v.Mode == ModeDesign && !v.ReadOnly && v.Can(PermDocumentEdit)

	// Apply interactive scratch overrides cleanly without mutating frozen source
	evalData := make(map[string]any, len(data))
	for k, val := range data {
		evalData[k] = val
	}
	switch mode {
	case "dilution":
		v.applyQuantityScratch(moduleID, evalData, "stock")
		v.applyQuantityScratch(moduleID, evalData, "target")
		v.applyQuantityScratch(moduleID, evalData, "final_volume")
	case "serial_dilution":
		if fStr, ok := v.calcScratchFloat(moduleID, "dilution_factor"); ok && fStr > 1 {
			evalData["dilution_factor"] = fStr
		}
		if sStr, ok := v.calcScratchFloat(moduleID, "steps_count"); ok && sStr >= 1 {
			evalData["steps_count"] = int(sStr)
		}
	case "group_mastermix":
		v.applyQuantityScratch(moduleID, evalData, "volume_per_unit")
		if ex, ok := v.calcScratchFloat(moduleID, "excess_ratio"); ok && ex >= 0 {
			evalData["excess_ratio"] = ex
		}
	case "sample_normalize":
		v.applyQuantityScratch(moduleID, evalData, "target_mass")
		v.applyQuantityScratch(moduleID, evalData, "volume_per_unit")
	case "formula":
		if s, ok := v.calcScratchString(moduleID, "formula"); ok {
			evalData["formula"] = s
		}
		if s, ok := v.calcScratchString(moduleID, "unit"); ok {
			evalData["unit"] = s
		}
	}

	res := EvalCalculatorWithActuals(v.Doc, evalData, v.CalcActuals)
	moduleCardFrame(v, c, lucide.Calculator(), title, calcModeLabel(mode), nil, func() {
		ui.Column(c).FillWidth().Gap(float32(tok.Spacing8)).Children(func() {
			if editableDesign {
				modeDraft := v.ensureWidgetDraft("calc_mode:"+moduleID, mode)
				if *modeDraft != mode && mode != "" {
					*modeDraft = mode
				}
				ui.Row(c).FillWidth().Wrap().AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
					ui.Text(c, "计算模式").FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
					mSel := components.Select(c, modeDraft)
					mSel.Trigger.Children(func() {
						ui.Text(c, calcModeLabel(*modeDraft)).FontSize(float32(tok.TextMeta)).TextColor(tok.Text)
					})
					mSel.Popup(func(panel ui.Element) {
						for _, opt := range []string{"dilution", "serial_dilution", "group_mastermix", "sample_normalize", "formula"} {
							opt := opt
							mSel.Item(opt).Children(func() {
								ui.Text(c, calcModeLabel(opt)).FontSize(float32(tok.TextMeta)).TextColor(tok.Text)
							})
						}
					})
					if mSel.Trigger.Changed() && *modeDraft != "" && *modeDraft != mode {
						v.emitAnyModuleEdit(moduleID, map[string]any{"mode": *modeDraft})
					}
					ui.Spacer(c)
					if components.Button(c, "保存计算设定", components.ButtonSecondary).Clicked() {
						props := map[string]any{"mode": *modeDraft}
						switch *modeDraft {
						case "dilution":
							props["stock"] = evalData["stock"]
							props["target"] = evalData["target"]
							props["final_volume"] = evalData["final_volume"]
						case "serial_dilution":
							props["dilution_factor"] = evalData["dilution_factor"]
							props["steps_count"] = evalData["steps_count"]
						case "group_mastermix":
							props["volume_per_unit"] = evalData["volume_per_unit"]
							props["excess_ratio"] = evalData["excess_ratio"]
						case "sample_normalize":
							props["target_mass"] = evalData["target_mass"]
							props["volume_per_unit"] = evalData["volume_per_unit"]
						case "formula":
							if s := textValue(evalData, "formula"); s != "" {
								props["formula"] = s
							}
							if u := textValue(evalData, "unit"); u != "" {
								props["unit"] = u
							}
						}
						v.emitAnyModuleEdit(moduleID, props)
					}
				})
			}

			// Interactive scratchpad inputs row
			ui.Row(c).FillWidth().Wrap().Gap(float32(tok.Spacing8)).Children(func() {
				switch mode {
				case "dilution":
					v.renderCalcQtyInput(c, moduleID, evalData, "stock", "母液")
					v.renderCalcQtyInput(c, moduleID, evalData, "target", "目标")
					v.renderCalcQtyInput(c, moduleID, evalData, "final_volume", "终体积")
				case "serial_dilution":
					v.renderCalcNumInput(c, moduleID, "dilution_factor", "稀释倍数", evalData["dilution_factor"])
					v.renderCalcNumInput(c, moduleID, "steps_count", "级数", evalData["steps_count"])
				case "group_mastermix":
					v.renderCalcQtyInput(c, moduleID, evalData, "volume_per_unit", "单孔/单份体积")
					v.renderCalcNumInput(c, moduleID, "excess_ratio", "余量比例", evalData["excess_ratio"])
				case "sample_normalize":
					v.renderCalcQtyInput(c, moduleID, evalData, "target_mass", "目标上样量")
					v.renderCalcQtyInput(c, moduleID, evalData, "volume_per_unit", "终体积")
				case "formula":
					v.renderCalcTextInput(c, moduleID, "formula", "计算公式", textValue(evalData, "formula"), float32(tok.Spacing48)*4.5)
					v.renderCalcTextInput(c, moduleID, "unit", "结果单位", textValue(evalData, "unit"), float32(tok.Spacing48)*2.0)
				}
			})

			if len(res.UsedActuals) > 0 {
				ui.Row(c).FillWidth().Wrap().Gap(float32(tok.Spacing4)).Children(func() {
					components.Badge(c, "已按实测值换算 · "+strings.Join(res.UsedActuals, ", "), components.BadgeOptions{Tone: components.BadgeSuccess})
				})
			}

			if res.Err != nil {
				v.moduleError(c, res.Err)
				return
			}
			if res.Summary != "" {
				ui.Row(c).FillWidth().Padding(float32(tok.Spacing8), float32(tok.Spacing12)).
					Radius(float32(tok.ControlRadius)).Background(tok.AccentSoftBg).Children(func() {
					ui.Text(c, res.Summary).FontSize(float32(tok.TextData)).Bold().TextColor(tok.Accent).FontFeatures("tnum")
				})
			}
			if len(res.Rows) > 0 {
				calcCols := []ui.TableColumn{
					{ID: "item", Title: "分项", MinWidth: 140},
					{ID: "amount", Title: "计算值", Width: float32(tok.Spacing48) * 2.5, Align: ui.End},
					{ID: "note", Title: "计算说明", MinWidth: 160},
				}
				components.Table(c, nil, calcCols, func() {
					for rIdx, row := range res.Rows {
						rIdx, row := rIdx, row
						amtStr := formatNumber(row.Amount)
						if row.Unit != "" {
							amtStr += " " + row.Unit
						}
						components.TableRow(c, rIdx, row.Label, false, func(col int) {
							switch col {
							case 0:
								ui.Text(c, row.Label).FontSize(float32(tok.TextBody)).Bold().TextColor(tok.Text)
							case 1:
								ui.Text(c, amtStr).FontSize(float32(tok.TextData)).Bold().TextColor(tok.Text).FontFeatures("tnum")
							case 2:
								ui.Text(c, firstNonEmpty(row.Note, "—")).FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
							}
						})
					}
				})
			}
		})
	})
}

func (v *View) applyQuantityScratch(moduleID string, data map[string]any, field string) {
	if val, ok := v.calcScratchFloat(moduleID, field); ok && val > 0 {
		if qm, ok := data[field].(map[string]any); ok {
			copyMap := make(map[string]any, len(qm))
			for k, mv := range qm {
				copyMap[k] = mv
			}
			copyMap["value"] = val
			data[field] = copyMap
		}
	}
}

func (v *View) calcScratchFloat(moduleID, field string) (float64, bool) {
	if v.calcScratch == nil || v.calcScratch[moduleID] == nil {
		return 0, false
	}
	ptr := v.calcScratch[moduleID][field]
	if ptr == nil || strings.TrimSpace(*ptr) == "" {
		return 0, false
	}
	f, err := strconv.ParseFloat(strings.TrimSpace(*ptr), 64)
	return f, err == nil
}

func (v *View) ensureCalcScratch(moduleID, field, initVal string) *string {
	if v.calcScratch == nil {
		v.calcScratch = map[string]map[string]*string{}
	}
	if v.calcScratch[moduleID] == nil {
		v.calcScratch[moduleID] = map[string]*string{}
	}
	if v.calcScratch[moduleID][field] == nil {
		s := initVal
		v.calcScratch[moduleID][field] = &s
	}
	return v.calcScratch[moduleID][field]
}

func (v *View) renderCalcQtyInput(c *ui.Context, moduleID string, data map[string]any, field, label string) {
	tok := theme.GetTokens(c.Theme())
	qv, qu, _ := quantityParts(data[field])
	buf := v.ensureCalcScratch(moduleID, field, formatNumber(qv))
	ui.Column(c).Width(float32(tok.Spacing48)*2.5).Gap(float32(tok.Spacing4)).Children(func() {
		ui.Text(c, label+" ("+qu+")").FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
		components.TextBox(c, buf, components.TextBoxOptions{})
	})
}

func (v *View) renderCalcNumInput(c *ui.Context, moduleID, field, label string, raw any) {
	tok := theme.GetTokens(c.Theme())
	n, _ := numericValue(raw)
	buf := v.ensureCalcScratch(moduleID, field, formatNumber(n))
	ui.Column(c).Width(float32(tok.Spacing48)*2.2).Gap(float32(tok.Spacing4)).Children(func() {
		ui.Text(c, label).FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
		components.TextBox(c, buf, components.TextBoxOptions{})
	})
}

func (v *View) calcScratchString(moduleID, field string) (string, bool) {
	if v.calcScratch == nil || v.calcScratch[moduleID] == nil {
		return "", false
	}
	ptr := v.calcScratch[moduleID][field]
	if ptr == nil {
		return "", false
	}
	return strings.TrimSpace(*ptr), true
}

func (v *View) renderCalcTextInput(c *ui.Context, moduleID, field, label, initVal string, width float32) {
	tok := theme.GetTokens(c.Theme())
	buf := v.ensureCalcScratch(moduleID, field, initVal)
	ui.Column(c).Width(width).Gap(float32(tok.Spacing4)).Children(func() {
		ui.Text(c, label).FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
		components.TextBox(c, buf, components.TextBoxOptions{})
	})
}

func calcModeLabel(mode string) string {
	switch mode {
	case "dilution":
		return "稀释 C1V1=C2V2"
	case "group_mastermix":
		return "分组母液混配"
	case "serial_dilution":
		return "梯度稀释"
	case "sample_normalize":
		return "样本归一化"
	case "formula":
		return "自由公式"
	default:
		return mode
	}
}

func (v *View) renderTimer(c *ui.Context, data map[string]any) {
	tok := theme.GetTokens(c.Theme())
	moduleID := textValue(data, "id")
	title := firstNonEmpty(textValue(data, "title"), "实验计时与周期")
	mode := textValue(data, "mode")
	editableDesign := v.Mode == ModeDesign && !v.ReadOnly && v.Can(PermDocumentEdit)
	interactiveRun := v.Mode == ModeRun && !v.ReadOnly && v.OnTimerAction != nil

	plannedSec, spanSec, _ := timerBudgetSec(data)
	rt, hasRT := v.TimerStates[moduleID]
	displaySec := plannedSec
	if displaySec == 0 {
		displaySec = spanSec
	}
	statusText := "待启动"
	if hasRT {
		if rt.RemainingS > 0 {
			displaySec = rt.RemainingS
		} else if rt.AccumulatedS > 0 {
			displaySec = rt.AccumulatedS
		}
		if rt.Status != "" {
			statusText = timerStatusLabel(rt.Status)
		}
	}

	moduleCardFrame(v, c, lucide.Timer(), title, timerModeLabel(mode), nil, func() {
		ui.Column(c).FillWidth().Gap(float32(tok.Spacing8)).Children(func() {
			ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(float32(tok.Spacing12)).Children(func() {
				ui.Text(c, formatClockHMS(displaySec)).FontSize(float32(tok.TextPage)).Bold().
					TextColor(tok.Text).FontFeatures("tnum")
				components.Badge(c, statusText)
				if mode == "cycle" {
					rounds := int(numOrZero(data["rounds"]))
					if rounds < 1 {
						rounds = 1
					}
					curRound := 1
					if hasRT && rt.RoundIndex > 0 {
						curRound = rt.RoundIndex + 1
					}
					components.Badge(c, fmt.Sprintf("第 %d / %d 轮", curRound, rounds), components.BadgeOptions{Tone: components.BadgeAccent})
				}
				if mode == "window" {
					if anchor := textValue(data, "anchor"); anchor != "" {
						ui.Text(c, "相对锚点 · "+anchor).FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
					} else if win := textValue(data, "schedule_window"); win != "" {
						ui.Text(c, "时间窗 · "+win).FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
					}
				}
			})

			if mode == "cycle" {
				phases := mapSlice(data["phases"])
				ui.Row(c).Wrap().Gap(float32(tok.Spacing8)).Children(func() {
					for pIdx, ph := range phases {
						pname := textValue(ph, "name")
						psec := quantitySeconds(ph["duration"])
						active := hasRT && rt.PhaseIndex == pIdx
						bg := tok.Elevated
						fg := tok.TextMuted
						if active {
							bg = tok.AccentSoftBg
							fg = tok.Accent
						}
						ui.Row(c).Padding(float32(tok.Spacing4), float32(tok.Spacing8)).
							Radius(float32(tok.ControlRadius)).Background(bg).Border(1, tok.Border).
							Gap(float32(tok.Spacing4)).Children(func() {
							ui.Text(c, pname).FontSize(float32(tok.TextMeta)).Bold().TextColor(fg)
							if psec > 0 {
								ui.Text(c, formatClockHMS(psec)).FontSize(float32(tok.TextMeta)).TextColor(fg).FontFeatures("tnum")
							} else {
								ui.Text(c, "手动推进").FontSize(float32(tok.TextMeta)).TextColor(fg)
							}
						})
					}
				})
			}

			if editableDesign {
				qRaw := data["duration"]
				if mode == "window" && data["offset"] != nil {
					qRaw = data["offset"]
				}
				qv, qu, qErr := quantityParts(qRaw)
				if qErr != nil || qu == "" {
					qv, qu = 10, "min"
				}
				roundsInit := int(numOrZero(data["rounds"]))
				if roundsInit < 1 {
					roundsInit = 1
				}
				tmMode := v.ensureWidgetDraft("tm:"+moduleID+":mode", firstNonEmpty(mode, "countdown"))
				if *tmMode != mode && mode != "" {
					*tmMode = mode
				}
				tmVal := v.ensureWidgetDraft("tm:"+moduleID+":val", formatNumber(qv))
				tmUnit := v.ensureWidgetDraft("tm:"+moduleID+":unit", qu)
				tmRounds := v.ensureWidgetDraft("tm:"+moduleID+":rounds", strconv.Itoa(roundsInit))
				ui.Row(c).FillWidth().Wrap().AlignItems(ui.Center).Gap(float32(tok.Spacing8)).
					Padding(float32(tok.Spacing4), 0, 0, 0).Children(func() {
					ui.Text(c, "模式").FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
					tSel := components.Select(c, tmMode)
					tSel.Trigger.Children(func() {
						ui.Text(c, timerModeLabel(*tmMode)).FontSize(float32(tok.TextMeta)).TextColor(tok.Text)
					})
					tSel.Popup(func(panel ui.Element) {
						for _, opt := range []string{"countdown", "stopwatch", "cycle", "window"} {
							opt := opt
							tSel.Item(opt).Children(func() {
								ui.Text(c, timerModeLabel(opt)).FontSize(float32(tok.TextMeta)).TextColor(tok.Text)
							})
						}
					})
					if tSel.Trigger.Changed() && *tmMode != mode {
						v.emitAnyModuleEdit(moduleID, map[string]any{"mode": *tmMode})
					}
					if *tmMode != "stopwatch" && *tmMode != "cycle" {
						ui.Box(c).Width(float32(tok.Spacing48)*1.5).Children(func() {
							components.TextBox(c, tmVal, components.TextBoxOptions{}).Placeholder("数值")
						})
						uSel := components.Select(c, tmUnit)
						uSel.Trigger.Children(func() {
							ui.Text(c, *tmUnit).FontSize(float32(tok.TextMeta)).TextColor(tok.Text)
						})
						uSel.Popup(func(panel ui.Element) {
							for _, opt := range []string{"s", "min", "h", "d"} {
								opt := opt
								uSel.Item(opt).Children(func() {
									ui.Text(c, opt).FontSize(float32(tok.TextMeta)).TextColor(tok.Text)
								})
							}
						})
					}
					if *tmMode == "cycle" {
						ui.Text(c, "轮数").FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
						ui.Box(c).Width(float32(tok.Spacing48)*1.2).Children(func() {
							components.TextBox(c, tmRounds, components.TextBoxOptions{}).Placeholder("轮数")
						})
					}
					ui.Spacer(c)
					if components.Button(c, "保存计时设定", components.ButtonSecondary).Clicked() {
						props := map[string]any{"mode": *tmMode}
						numVal, err := strconv.ParseFloat(strings.TrimSpace(*tmVal), 64)
						if err != nil || numVal <= 0 {
							numVal = 10
						}
						switch *tmMode {
						case "window":
							props["offset"] = map[string]any{"value": numVal, "unit": *tmUnit}
						case "cycle":
							rCnt, _ := strconv.Atoi(strings.TrimSpace(*tmRounds))
							if rCnt < 1 {
								rCnt = 1
							}
							props["rounds"] = rCnt
						case "countdown":
							props["duration"] = map[string]any{"value": numVal, "unit": *tmUnit}
						}
						v.emitAnyModuleEdit(moduleID, props)
					}
				})
			}

			if interactiveRun {
				ui.Row(c).Gap(float32(tok.Spacing8)).Children(func() {
					if iconButton(c, lucide.Play(), "开始", components.ButtonSecondary).Clicked() {
						v.OnTimerAction(v.StepID, moduleID, "start")
					}
					if iconButton(c, lucide.Pause(), "暂停", components.ButtonOutline).Clicked() {
						v.OnTimerAction(v.StepID, moduleID, "pause")
					}
					if mode == "cycle" {
						if iconButton(c, lucide.SkipForward(), "下一阶段", components.ButtonOutline).Clicked() {
							v.OnTimerAction(v.StepID, moduleID, "advance")
						}
					}
					if iconButton(c, lucide.Check(), "完成", components.ButtonOutline).Clicked() {
						v.OnTimerAction(v.StepID, moduleID, "complete")
					}
				})
			}
		})
	})
}

func timerStatusLabel(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "", "idle":
		return "待启动"
	case "running":
		return "计时中"
	case "paused":
		return "已暂停"
	case "completed":
		return "已完成"
	default:
		return status
	}
}

func timerModeLabel(mode string) string {
	switch mode {
	case "countdown":
		return "倒计时"
	case "stopwatch":
		return "正计时"
	case "cycle":
		return "多阶段循环"
	case "window":
		return "时间窗提醒"
	default:
		return mode
	}
}

func formatClockHMS(sec int) string {
	if sec < 0 {
		sec = 0
	}
	h := sec / 3600
	m := (sec % 3600) / 60
	s := sec % 60
	if h > 0 {
		return fmt.Sprintf("%02d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%02d:%02d", m, s)
}

func (v *View) renderAttachment(c *ui.Context, data map[string]any) {
	tok := theme.GetTokens(c.Theme())
	moduleID := textValue(data, "id")
	title := firstNonEmpty(textValue(data, "title"), "附件与现场拍摄")
	scope := textValue(data, "scope")
	containerRef := textValue(data, "container_ref")
	slots := int(numOrZero(data["slots_per_cell"]))
	if slots < 1 {
		slots = 1
	}
	mediaHint := textValue(data, "media_hint")
	thumbs := v.LinkedAssets[moduleID]
	if len(thumbs) == 0 && v.StepID != "" {
		thumbs = v.LinkedAssets[v.StepID]
	}

	scopeBadge := "步骤证据"
	switch scope {
	case "design_ref":
		scopeBadge = "设计参考文档"
	case "container_grid":
		scopeBadge = fmt.Sprintf("网格采图 %s × %d 视野", firstNonEmpty(containerRef, "容器"), slots)
	}

	moduleCardFrame(v, c, lucide.Paperclip(), title, scopeBadge, nil, func() {
		ui.Column(c).FillWidth().Gap(float32(tok.Spacing8)).Children(func() {
			if mediaHint != "" {
				ui.Text(c, "采集要求："+mediaHint).FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
			}
			if scope == "container_grid" {
				if pl, _ := v.findPlate(containerRef); pl != nil {
					if geom, ok := geometryForPlate(pl.Format, pl.Topology, pl.Rows, pl.Columns); ok {
						countsByWell := map[string]int{}
						for _, th := range thumbs {
							if th.WellID != "" {
								countsByWell[strings.ToUpper(th.WellID)]++
							}
						}
						wellGroups := pl.WellGroups()
						var previewWells []string
						for r := 0; r < geom.Rows; r++ {
							for col := 0; col < geom.Columns; col++ {
								wID := fmt.Sprintf("%s%d", rowLabel(r), col+1)
								if wellGroups[wID] != "" || countsByWell[wID] > 0 {
									previewWells = append(previewWells, wID)
								}
							}
						}
						if len(previewWells) == 0 {
							for r := 0; r < geom.Rows && len(previewWells) < 8; r++ {
								for col := 0; col < geom.Columns && len(previewWells) < 8; col++ {
									previewWells = append(previewWells, fmt.Sprintf("%s%d", rowLabel(r), col+1))
								}
							}
						}
						if len(previewWells) > 12 {
							previewWells = previewWells[:12]
						}
						ui.Row(c).FillWidth().Wrap().AlignItems(ui.Center).Gap(float32(tok.Spacing4)).Children(func() {
							for _, wID := range previewWells {
								wID := wID
								nCnt := countsByWell[wID]
								label := fmt.Sprintf("%s %d/%d", wID, nCnt, slots)
								bg := tok.Elevated
								fg := tok.TextMuted
								if nCnt >= slots {
									bg = tok.SuccessBg
									fg = tok.Success
								} else if nCnt > 0 {
									bg = tok.AccentSoftBg
									fg = tok.Accent
								}
								if v.Mode == ModeRun && !v.ReadOnly && v.OnLinkAsset != nil {
									pill := ui.ButtonBase(c).Padding(float32(tok.Spacing4), float32(tok.Spacing8)).
										Radius(float32(tok.ControlRadius)).Background(bg).Border(1, tok.Border).Label(label)
									pill.Children(func() {
										ui.Text(c, label).FontSize(float32(tok.TextMeta)).TextColor(fg).FontFeatures("tnum")
									})
									if pill.Clicked() {
										v.OnLinkAsset(v.StepID, moduleID, wID)
									}
								} else {
									ui.Row(c).Padding(float32(tok.Spacing4), float32(tok.Spacing8)).
										Radius(float32(tok.ControlRadius)).Background(bg).Border(1, tok.Border).Children(func() {
										ui.Text(c, label).FontSize(float32(tok.TextMeta)).TextColor(fg).FontFeatures("tnum")
									})
								}
							}
						})
					}
				}
			}
			if len(thumbs) > 0 {
				ui.Row(c).FillWidth().Wrap().Gap(float32(tok.Spacing8)).Children(func() {
					for _, th := range thumbs {
						th := th
						resolvedPath := resolveFigurePath(v.BaseDir, th.FilePath)
						thumbBtn := ui.ButtonBase(c).Width(float32(tok.Spacing48)*2.7).Padding(float32(tok.Spacing8)).
							Radius(float32(tok.ControlRadius)).Background(tok.Elevated).Border(1, tok.Border).Children(func() {
							ui.Column(c).FillWidth().Gap(float32(tok.Spacing4)).Children(func() {
								if resolvedPath != "" {
									if imgSrc, errText := v.cachedImageFile(resolvedPath); errText == "" && imgSrc != nil {
										ui.Image(c, imgSrc).Height(float32(tok.Spacing48) * 1.5)
									} else {
										ui.Row(c).FillWidth().Height(float32(tok.Spacing48)*1.5).Center().
											Radius(float32(tok.ControlRadius)).Background(tok.Surface).Children(func() {
											ui.Text(c, firstNonEmpty(errText, "图片不可用")).FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
										})
									}
								}
								lbl := firstNonEmpty(th.Label, th.AssetID)
								if th.WellID != "" {
									lbl = fmt.Sprintf("%s #%d · %s", th.WellID, th.FovIndex, lbl)
								}
								ui.Text(c, lbl).FontSize(float32(tok.TextMeta)).TextColor(tok.Text).FontFeatures("tnum")
							})
						})
						if thumbBtn.Clicked() {
							cp := th
							v.ExpandedAsset = &cp
							v.AssetZoom = 1
							v.assetOpen = true
						}
					}
				})
			} else {
				emptyMsg := "Run 执行时可关联原始培养皿照片、显微视野或文件"
				if scope == "container_grid" {
					emptyMsg = fmt.Sprintf("计划每孔 %d 个采图槽位 · 当前 0 个已关联", slots)
				}
				ui.Text(c, emptyMsg).FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted).FontFeatures("tnum")
			}
			if v.Mode == ModeRun && !v.ReadOnly && v.OnLinkAsset != nil {
				if iconButton(c, lucide.Paperclip(), "关联图片 / 文件", components.ButtonSecondary).Clicked() {
					v.OnLinkAsset(v.StepID, moduleID, "")
				}
			}
			if v.assetOpen && v.ExpandedAsset != nil {
				exp := *v.ExpandedAsset
				if v.AssetZoom <= 0 {
					v.AssetZoom = 1
				}
				components.Dialog(c.Key("asset-dlg:"+moduleID), &v.assetOpen, components.DialogOptions{Label: firstNonEmpty(exp.Label, exp.AssetID)}, func(panel ui.Element) {
					panel.Width(float32(tok.Spacing48)*14).Gap(float32(tok.Spacing12)).Children(func() {
						ui.Row(c).FillWidth().Wrap().AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
							ui.Text(c, firstNonEmpty(exp.Label, exp.AssetID)).FontSize(float32(tok.TextSection)).Bold().TextColor(tok.Text)
							if exp.WellID != "" {
								components.Badge(c, fmt.Sprintf("%s #%d", exp.WellID, exp.FovIndex))
							}
							ui.Spacer(c)
							for _, z := range []float32{1.0, 1.5, 2.0} {
								z := z
								style := components.ButtonOutline
								if v.AssetZoom == z {
									style = components.ButtonSecondary
								}
								if components.Button(c, fmt.Sprintf("%d%%", int(z*100)), style).Clicked() {
									v.AssetZoom = z
								}
							}
							if v.OnOpenExternalAsset != nil {
								if components.Button(c, "系统打开", components.ButtonOutline).Clicked() {
									v.OnOpenExternalAsset(exp)
								}
							}
							if components.Button(c, "关闭", components.ButtonOutline).Clicked() {
								v.assetOpen = false
							}
						})
						if resolvedExp := resolveFigurePath(v.BaseDir, exp.FilePath); resolvedExp != "" {
							imgSrc, errText := v.cachedImageFile(resolvedExp)
							if errText == "" && imgSrc != nil {
								ui.ScrollBoth(c).FillWidth().MaxHeight(460).Children(func() {
									ui.Image(c, imgSrc).Height(300 * v.AssetZoom)
								})
							} else {
								ui.Text(c, "文件预览不可用："+firstNonEmpty(errText, resolvedExp)).FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
							}
						}
						if exp.AssetID != "" {
							ui.Text(c, fmt.Sprintf("资产 ID: %s · 路径: %s", exp.AssetID, exp.FilePath)).
								FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
						}
					})
				})
			}
		})
	})
}

func (v *View) renderSubflow(c *ui.Context, data map[string]any) {
	tok := theme.GetTokens(c.Theme())
	moduleID := textValue(data, "id")
	title := firstNonEmpty(textValue(data, "title"), "子流程")
	items := mapSlice(data["items"])
	collapsedDefault, _ := data["collapsed"].(bool)
	openPtr := v.ensureWidgetBoolInit("sf:"+moduleID, !collapsedDefault)
	editableDesign := v.Mode == ModeDesign && !v.ReadOnly && v.Can(PermDocumentEdit)

	moduleCardFrame(v, c, lucide.GitBranch(), title, fmt.Sprintf("%d 项", len(items)), nil, func() {
		components.Collapsible(c, fmt.Sprintf("展开子步骤明细（%d）", len(items)), openPtr, components.CollapsibleOptions{}, func() {
			ui.Column(c).FillWidth().Gap(float32(tok.Spacing4)).Padding(float32(tok.Spacing4), 0, 0, 0).Children(func() {
				for idx, it := range items {
					idx := idx
					sid := textValue(it, "id")
					stitle := textValue(it, "title")
					snote := textValue(it, "note")
					donePtr := v.ensureWidgetBool("sf_chk:" + moduleID + ":" + sid)
					ui.Row(c).FillWidth().Padding(float32(tok.Spacing8), float32(tok.Spacing8)).
						Radius(float32(tok.ControlRadius)).Background(tok.Elevated).Border(1, tok.Border).
						AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
						ui.Text(c, fmt.Sprintf("%d.", idx+1)).FontSize(float32(tok.TextMeta)).Bold().
							TextColor(tok.Accent).FontFeatures("tnum")
						if v.Mode == ModeRun && !v.ReadOnly {
							if components.Checkbox(c, donePtr, stitle).Changed() && v.OnToggleCheck != nil {
								v.OnToggleCheck(v.StepID, sid, *donePtr)
							}
						} else {
							ui.Text(c, stitle).FontSize(float32(tok.TextBody)).TextColor(tok.Text)
						}
						if snote != "" {
							ui.Spacer(c)
							ui.Text(c, snote).FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
						} else if editableDesign {
							ui.Spacer(c)
						}
						if editableDesign && len(items) > 1 {
							if components.IconButton(c, lucide.Trash(), "删除子步骤", components.IconButtonCompact).Clicked() {
								next := removeSliceIndex(items, idx)
								v.emitAnyModuleEdit(moduleID, map[string]any{"items": next})
							}
						}
					})
				}
				if editableDesign {
					newTitle := v.ensureWidgetDraft("sf:"+moduleID+":new_title", "")
					newNote := v.ensureWidgetDraft("sf:"+moduleID+":new_note", "")
					ui.Row(c).FillWidth().Gap(float32(tok.Spacing8)).Padding(float32(tok.Spacing4), 0, 0, 0).
						AlignItems(ui.Center).Children(func() {
						ui.Box(c).FillWidth().Children(func() {
							components.TextBox(c, newTitle, components.TextBoxOptions{}).Placeholder("新增子步骤描述…")
						})
						ui.Box(c).Width(float32(tok.Spacing48)*2.5).Children(func() {
							components.TextBox(c, newNote, components.TextBoxOptions{}).Placeholder("备注（可选）")
						})
						if components.Button(c, "添加子步", components.ButtonSecondary).Clicked() && strings.TrimSpace(*newTitle) != "" {
							next := append([]map[string]any{}, items...)
							entry := map[string]any{
								"id":    fmt.Sprintf("sf-%s-%d", sanitizeIDFrag(*newTitle), len(items)+1),
								"title": strings.TrimSpace(*newTitle),
							}
							if strings.TrimSpace(*newNote) != "" {
								entry["note"] = strings.TrimSpace(*newNote)
							}
							next = append(next, entry)
							v.emitAnyModuleEdit(moduleID, map[string]any{"items": next})
							*newTitle = ""
							*newNote = ""
						}
					})
				}
			})
		})
	})
}
