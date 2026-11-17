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

func (v *View) renderGroupTable(c *ui.Context, data map[string]any) {
	t := c.Theme()
	tok := theme.GetTokens(t)
	moduleID := textValue(data, "id")
	title := firstNonEmpty(textValue(data, "title"), "实验分组表")
	editable := v.Mode == ModeDesign && !v.ReadOnly && v.Can(PermDocumentEdit)

	rows := mapSlice(data["rows"])
	moduleCardFrame(v, c, lucide.Table2(), title, fmt.Sprintf("%d 组", len(rows)), nil, func() {
		ui.Column(c).FillWidth().Gap(float32(tok.Spacing8)).Children(func() {
			cols := []ui.TableColumn{
				{ID: "name", Title: "组别名称", MinWidth: 140},
				{ID: "role", Title: "角色", Width: float32(tok.Spacing48) * 2.5},
				{ID: "cond", Title: "处理条件", MinWidth: 160},
				{ID: "rep", Title: "n / 重复", Width: float32(tok.Spacing48) * 1.6, Align: ui.End},
			}
			if editable && len(rows) > 1 {
				cols = append(cols, ui.TableColumn{ID: "act", Title: "", Width: float32(tok.Spacing32) + float32(tok.Spacing8), Fixed: true})
			}
			components.Table(c, nil, cols, func() {
				for idx, rm := range rows {
					idx, rm := idx, rm
					gid := textValue(rm, "id")
					gname := firstNonEmpty(textValue(rm, "name"), gid)
					role := textValue(rm, "role")
					cond := textValue(rm, "condition")
					indN, _ := numericValue(rm["independent_n"])
					techR, _ := numericValue(rm["technical_repeats"])
					if techR == 0 {
						techR, _ = numericValue(rm["replicate_count"])
					}
					components.TableRow(c, idx, gname, false, func(col int) {
						switch col {
						case 0:
							ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
								ui.Box(c).Size(float32(tok.Spacing12), float32(tok.Spacing12)).
									Radius(float32(tok.ControlRadius)).Background(groupColorForCtx(c, gid))
								ui.Text(c, gname).FontSize(float32(tok.TextBody)).Bold().TextColor(tok.Text)
							})
						case 1:
							if role != "" {
								groupRoleBadge(c, role)
							} else {
								ui.Text(c, "—").FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
							}
						case 2:
							ui.Text(c, firstNonEmpty(cond, "—")).FontSize(float32(tok.TextBody)).TextColor(tok.TextMuted)
						case 3:
							nLabel := "—"
							if indN > 0 && techR > 0 {
								nLabel = fmt.Sprintf("n=%d × %d", int(indN), int(techR))
							} else if indN > 0 {
								nLabel = fmt.Sprintf("n=%d", int(indN))
							} else if techR > 0 {
								nLabel = fmt.Sprintf("rep=%d", int(techR))
							}
							ui.Text(c, nLabel).FontSize(float32(tok.TextMeta)).TextColor(tok.Text).FontFeatures("tnum")
						case 4:
							delBtn := components.IconButton(c, lucide.Trash(), "删除组别", components.IconButtonCompact)
							components.Tooltip(c, delBtn, "删除该分组")
							if delBtn.Clicked() {
								next := removeSliceIndex(rows, idx)
								v.emitAnyModuleEdit(moduleID, map[string]any{"rows": next})
							}
						}
					})
				}
			})

			if editable {
				newName := v.ensureWidgetDraft("gt:"+moduleID+":name", "")
				newRole := v.ensureWidgetDraft("gt:"+moduleID+":role", "treatment")
				newCond := v.ensureWidgetDraft("gt:"+moduleID+":cond", "")
				newN := v.ensureWidgetDraft("gt:"+moduleID+":n", "3")
				ui.Row(c).FillWidth().Gap(float32(tok.Spacing8)).Padding(float32(tok.Spacing8), 0, 0, 0).
					AlignItems(ui.Center).Children(func() {
					ui.Box(c).Width(float32(tok.Spacing48)*2.5).Children(func() {
						components.TextBox(c, newName, components.TextBoxOptions{}).Placeholder("新组别名称")
					})
					sel := components.Select(c, newRole)
					sel.Trigger.Children(func() {
						ui.Text(c, groupRoleLabel(*newRole)).FontSize(float32(tok.TextMeta)).TextColor(tok.Text)
					})
					sel.Popup(func(panel ui.Element) {
						for _, opt := range []string{"control", "treatment", "comparator", "reference"} {
							opt := opt
							sel.Item(opt).Children(func() {
								ui.Text(c, groupRoleLabel(opt)).FontSize(float32(tok.TextMeta)).TextColor(tok.Text)
							})
						}
					})
					ui.Box(c).FillWidth().Children(func() {
						components.TextBox(c, newCond, components.TextBoxOptions{}).Placeholder("处理/浓度条件")
					})
					ui.Box(c).Width(float32(tok.Spacing48)*1.2).Children(func() {
						components.TextBox(c, newN, components.TextBoxOptions{}).Placeholder("n")
					})
					if components.Button(c, "添加组", components.ButtonSecondary).Clicked() && strings.TrimSpace(*newName) != "" {
						next := append([]map[string]any{}, rows...)
						nVal, _ := strconv.Atoi(strings.TrimSpace(*newN))
						if nVal <= 0 {
							nVal = 3
						}
						gid := fmt.Sprintf("g-%s-%d", sanitizeIDFrag(*newName), len(rows)+1)
						rowObj := map[string]any{
							"id":            gid,
							"name":          strings.TrimSpace(*newName),
							"role":          *newRole,
							"condition":     strings.TrimSpace(*newCond),
							"independent_n": nVal,
						}
						next = append(next, rowObj)
						v.emitAnyModuleEdit(moduleID, map[string]any{"rows": next})
						*newName = ""
						*newCond = ""
					}
				})
			}
		})
	})
}

func groupRoleBadge(c *ui.Context, role string) {
	switch role {
	case "treatment":
		components.Badge(c, groupRoleLabel(role), components.BadgeOptions{Tone: components.BadgeAccent})
	default:
		components.Badge(c, groupRoleLabel(role))
	}
}

func groupRoleLabel(role string) string {
	switch role {
	case "control":
		return "对照 control"
	case "treatment":
		return "处理 treatment"
	case "comparator":
		return "阳参 comparator"
	case "reference":
		return "参照 reference"
	default:
		return role
	}
}

func (v *View) renderParamTable(c *ui.Context, data map[string]any) {
	t := c.Theme()
	tok := theme.GetTokens(t)
	moduleID := textValue(data, "id")
	title := firstNonEmpty(textValue(data, "title"), "参数表")
	editableDesign := v.Mode == ModeDesign && !v.ReadOnly && v.Can(PermDocumentEdit)
	interactiveRun := v.Mode == ModeRun && !v.ReadOnly
	rows := mapSlice(data["rows"])

	moduleCardFrame(v, c, lucide.SlidersHorizontal(), title, fmt.Sprintf("%d 项参数", len(rows)), nil, func() {
		ui.Column(c).FillWidth().Gap(float32(tok.Spacing8)).Children(func() {
			pCols := []ui.TableColumn{
				{ID: "param", Title: "参数", MinWidth: 160},
				{ID: "planned", Title: "计划值", Width: float32(tok.Spacing48) * 2.5, Align: ui.End},
			}
			if v.Mode == ModeRun {
				pCols = append(pCols, ui.TableColumn{ID: "actual", Title: "实测 / 记录", Width: float32(tok.Spacing48) * 3.8})
			} else if editableDesign {
				pCols = append(pCols, ui.TableColumn{ID: "act", Title: "", Width: float32(tok.Spacing32) + float32(tok.Spacing8), Fixed: true})
			}
			components.Table(c, nil, pCols, func() {
				for idx, rm := range rows {
					idx, rm := idx, rm
					key := firstNonEmpty(textValue(rm, "key"), fmt.Sprintf("p-%d", idx))
					label := firstNonEmpty(textValue(rm, "label"), key)
					unit := textValue(rm, "unit")
					note := textValue(rm, "note")
					valStr := formatAnyValue(rm["value"])
					recActual, hasRA := rm["record_actual"].(bool)
					allowActual := !hasRA || recActual

					components.TableRow(c, idx, label, false, func(col int) {
						switch col {
						case 0:
							ui.Column(c).FillWidth().Gap(float32(tok.Spacing4)*0.5).Children(func() {
								ui.Text(c, label).FontSize(float32(tok.TextBody)).Bold().TextColor(tok.Text)
								if note != "" {
									ui.Text(c, note).FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
								}
							})
						case 1:
							plannedDisplay := valStr
							if unit != "" {
								plannedDisplay += " " + unit
							}
							ui.Text(c, plannedDisplay).FontSize(float32(tok.TextData)).Bold().TextColor(tok.Text).FontFeatures("tnum")
						case 2:
							if v.Mode == ModeRun {
								stateKey := v.StepID + ":" + moduleID + ":" + key
								recorded := v.ActualParams[stateKey]
								if recorded == "" {
									recorded = v.ActualParams[key]
								}
								if allowActual && interactiveRun {
									buf := v.ensureWidgetDraft("pt:"+stateKey, recorded)
									ui.Row(c).FillWidth().Gap(float32(tok.Spacing4)).AlignItems(ui.Center).Children(func() {
										ui.Box(c).FillWidth().Children(func() {
											components.TextBox(c, buf, components.TextBoxOptions{}).Placeholder(firstNonEmpty(unit, "实测值"))
										})
										if components.Button(c, "记录", components.ButtonSecondary).Clicked() && strings.TrimSpace(*buf) != "" {
											if v.OnRecordActualParam != nil {
												v.OnRecordActualParam(v.StepID, moduleID, key, strings.TrimSpace(*buf), unit)
											}
										}
									})
								} else if recorded != "" {
									components.Badge(c, "实测 "+recorded+" "+unit, components.BadgeOptions{Tone: components.BadgeSuccess})
								} else {
									ui.Text(c, "—").FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
								}
							} else if editableDesign {
								delBtn := components.IconButton(c, lucide.Trash(), "删除参数", components.IconButtonCompact)
								components.Tooltip(c, delBtn, "删除此参数项")
								if delBtn.Clicked() {
									next := removeSliceIndex(rows, idx)
									v.emitAnyModuleEdit(moduleID, map[string]any{"rows": next})
								}
							}
						}
					})
				}
			})

			if editableDesign {
				newLabel := v.ensureWidgetDraft("pt:"+moduleID+":label", "")
				newVal := v.ensureWidgetDraft("pt:"+moduleID+":val", "")
				newUnit := v.ensureWidgetDraft("pt:"+moduleID+":unit", "")
				ui.Row(c).FillWidth().Gap(float32(tok.Spacing8)).Padding(float32(tok.Spacing8), 0, 0, 0).
					AlignItems(ui.Center).Children(func() {
					ui.Box(c).FillWidth().Children(func() {
						components.TextBox(c, newLabel, components.TextBoxOptions{}).Placeholder("新参数名（如 OD600）")
					})
					ui.Box(c).Width(float32(tok.Spacing48)*2).Children(func() {
						components.TextBox(c, newVal, components.TextBoxOptions{}).Placeholder("计划值")
					})
					ui.Box(c).Width(float32(tok.Spacing48)*1.5).Children(func() {
						components.TextBox(c, newUnit, components.TextBoxOptions{}).Placeholder("单位")
					})
					if components.Button(c, "添加参数", components.ButtonSecondary).Clicked() &&
						strings.TrimSpace(*newLabel) != "" && strings.TrimSpace(*newVal) != "" {
						next := append([]map[string]any{}, rows...)
						var typedVal any = strings.TrimSpace(*newVal)
						if f, err := strconv.ParseFloat(strings.TrimSpace(*newVal), 64); err == nil {
							typedVal = f
						}
						rowObj := map[string]any{
							"key":           sanitizeIDFrag(*newLabel),
							"label":         strings.TrimSpace(*newLabel),
							"value":         typedVal,
							"record_actual": true,
						}
						if strings.TrimSpace(*newUnit) != "" {
							rowObj["unit"] = strings.TrimSpace(*newUnit)
						}
						next = append(next, rowObj)
						v.emitAnyModuleEdit(moduleID, map[string]any{"rows": next})
						*newLabel = ""
						*newVal = ""
						*newUnit = ""
					}
				})
			}
		})
	})
}

func (v *View) renderDataTable(c *ui.Context, data map[string]any) {
	t := c.Theme()
	tok := theme.GetTokens(t)
	moduleID := textValue(data, "id")
	title := firstNonEmpty(textValue(data, "title"), "数据记录表")
	schemaID := textValue(data, "schema_id")
	endpoint := textValue(data, "endpoint")
	cols := mapSlice(data["columns"])
	authoredRows := mapSlice(data["rows"])
	editableDesign := v.Mode == ModeDesign && !v.ReadOnly && v.Can(PermDocumentEdit)
	interactiveRun := v.Mode == ModeRun && !v.ReadOnly

	extraRowsKey := "dt_extra_rows:" + moduleID
	extraRowsStr := v.ensureWidgetDraft(extraRowsKey, "0")
	extraRows, _ := strconv.Atoi(*extraRowsStr)
	rowCount := len(authoredRows) + extraRows
	if rowCount == 0 && len(authoredRows) == 0 {
		rowCount = 2
	}

	badge := firstNonEmpty(endpoint, schemaID, fmt.Sprintf("%d 列", len(cols)))
	moduleCardFrame(v, c, lucide.Table2(), title, badge, nil, func() {
		ui.Column(c).FillWidth().Gap(float32(tok.Spacing8)).Children(func() {
			if interactiveRun {
				ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
					if iconButton(c, lucide.ClipboardPaste(), "粘贴 CSV/TSV", components.ButtonOutline).Clicked() {
						clip := c.Services().ReadClipboard()
						if strings.TrimSpace(clip) != "" {
							v.applyClipboardToDataTable(moduleID, cols, clip)
						}
					}
					if iconButton(c, lucide.Plus(), "新增行", components.ButtonOutline).Clicked() {
						*extraRowsStr = strconv.Itoa(extraRows + 1)
					}
					ui.Spacer(c)
					if iconButton(c, lucide.Check(), "提交表格读数", components.ButtonSecondary).Clicked() {
						if v.OnSubmitTableReadout != nil {
							payload := v.buildDataTableSubmission(moduleID, schemaID, endpoint, cols, authoredRows, rowCount)
							v.OnSubmitTableReadout(v.StepID, moduleID, payload)
						}
					}
				})
			}

			// Column headers with unit & threshold pills
			ui.Row(c).FillWidth().Padding(float32(tok.Spacing4), float32(tok.Spacing8)).
				Radius(float32(tok.ControlRadius)).Background(tok.Elevated).Gap(float32(tok.Spacing8)).Children(func() {
				for _, cm := range cols {
					clabel := textValue(cm, "label")
					cunit := textValue(cm, "unit")
					vkind := textValue(cm, "value_kind")
					minV, hasMin := numericValue(cm["min"])
					maxV, hasMax := numericValue(cm["max"])
					ui.Column(c).FillWidth().Gap(2).Children(func() {
						hdr := clabel
						if cunit != "" {
							hdr += " (" + cunit + ")"
						}
						if vkind == "formula" {
							hdr = "fx " + hdr
						}
						ui.Text(c, hdr).FontSize(float32(tok.TextMeta)).Bold().TextColor(tok.TextMuted)
						if hasMin || hasMax {
							rng := "["
							if hasMin {
								rng += formatNumber(minV)
							}
							rng += "–"
							if hasMax {
								rng += formatNumber(maxV)
							}
							rng += "]"
							ui.Text(c, rng).FontSize(float32(tok.TextMeta)).TextColor(tok.Accent).FontFeatures("tnum")
						}
					})
				}
				if v.Mode == ModeRun {
					ui.Box(c).Width(float32(tok.Spacing48)*2.4).Children(func() {
						ui.Text(c, "修饰 / 样本状态").FontSize(float32(tok.TextMeta)).Bold().TextColor(tok.TextMuted)
					})
				}
			})

			for rIdx := 0; rIdx < rowCount; rIdx++ {
				rIdx := rIdx
				var pre map[string]any
				if rIdx < len(authoredRows) {
					pre = authoredRows[rIdx]
				}
				// Collect numeric variables across columns for formula evaluation
				rowVars := map[string]float64{}
				for _, cm := range cols {
					ckey := textValue(cm, "key")
					if textValue(cm, "value_kind") == "formula" {
						continue
					}
					rawCell := v.dataTableCellVal(moduleID, rIdx, ckey, pre)
					if f, err := strconv.ParseFloat(strings.TrimSpace(rawCell), 64); err == nil {
						rowVars[ckey] = f
					}
				}
				sampleID := firstNonEmpty(v.dataTableCellVal(moduleID, rIdx, "sample_id", pre), fmt.Sprintf("row-%d", rIdx+1))

				ui.Row(c).FillWidth().Padding(float32(tok.Spacing8), float32(tok.Spacing8)).
					Radius(float32(tok.ControlRadius)).Border(1, tok.Border).
					AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
					for _, cm := range cols {
						ckey := textValue(cm, "key")
						vkind := textValue(cm, "value_kind")
						cunit := textValue(cm, "unit")
						minV, hasMin := numericValue(cm["min"])
						maxV, hasMax := numericValue(cm["max"])

						ui.Column(c).FillWidth().Gap(float32(tok.Spacing4)).Children(func() {
							if vkind == "formula" {
								expr := textValue(cm, "formula")
								res, err := EvalFormula(expr, rowVars)
								if err != nil {
									ui.Text(c, "—").FontSize(float32(tok.TextData)).TextColor(tok.TextMuted).FontFeatures("tnum")
								} else {
									txt := formatNumber(round4(res))
									if cunit != "" {
										txt += " " + cunit
									}
									ui.Text(c, txt).FontSize(float32(tok.TextData)).Bold().TextColor(tok.Accent).FontFeatures("tnum")
									if (hasMin && res < minV) || (hasMax && res > maxV) {
										components.Badge(c, rangeWarningText(hasMin, minV, hasMax, maxV, cunit), components.BadgeOptions{Tone: components.BadgeWarning})
									}
								}
								return
							}

							initStr := formatAnyValue(nil)
							if pre != nil && pre[ckey] != nil {
								initStr = formatAnyValue(pre[ckey])
							}
							cellKey := v.cellDraftKey(moduleID, rIdx, ckey)
							buf := v.ensureWidgetDraft(cellKey, initStr)
							if v.TableCells != nil && v.TableCells[moduleID] != nil {
								if saved, exists := v.TableCells[moduleID][cellKey]; exists {
									*buf = saved
								}
							}
							if interactiveRun {
								components.TextBox(c, buf, components.TextBoxOptions{}).Placeholder("输入")
								if v.TableCells == nil {
									v.TableCells = map[string]map[string]string{}
								}
								if v.TableCells[moduleID] == nil {
									v.TableCells[moduleID] = map[string]string{}
								}
								v.TableCells[moduleID][cellKey] = *buf
							} else {
								ui.Text(c, firstNonEmpty(*buf, "—")).FontSize(float32(tok.TextData)).TextColor(tok.Text).FontFeatures("tnum")
							}
							if vkind == "number" && strings.TrimSpace(*buf) != "" {
								if numVal, err := strconv.ParseFloat(strings.TrimSpace(*buf), 64); err == nil {
									if (hasMin && numVal < minV) || (hasMax && numVal > maxV) {
										components.Badge(c, rangeWarningText(hasMin, minV, hasMax, maxV, cunit), components.BadgeOptions{Tone: components.BadgeWarning})
									}
								}
							}
						})
					}

					if v.Mode == ModeRun {
						ui.Column(c).Width(float32(tok.Spacing48)*2.4).Gap(float32(tok.Spacing4)).Children(func() {
							qKey := fmt.Sprintf("dt_qual:%s:%d", moduleID, rIdx)
							qDraft := v.ensureWidgetDraft(qKey, "正常")
							if interactiveRun {
								qSel := components.Select(c, qDraft)
								qSel.Trigger.Children(func() {
									ui.Text(c, *qDraft).FontSize(float32(tok.TextMeta)).TextColor(tok.Text)
								})
								qSel.Popup(func(panel ui.Element) {
									for _, opt := range []string{"正常", "<LOD", ">ULOQ", "未检出"} {
										opt := opt
										qSel.Item(opt).Children(func() {
											ui.Text(c, opt).FontSize(float32(tok.TextMeta)).TextColor(tok.Text)
										})
									}
								})
							}
							sStatus := v.SampleStatuses[sampleID]
							if sStatus == "" {
								sStatus = "正常"
							}
							sDraft := v.ensureWidgetDraft("dt_stat:"+moduleID+":"+sampleID, sStatus)
							if interactiveRun {
								sSel := components.Select(c, sDraft)
								sSel.Trigger.Children(func() {
									ui.Text(c, *sDraft).FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
								})
								sSel.Popup(func(panel ui.Element) {
									for _, opt := range []string{"正常", "QC剔除", "删失", "重做"} {
										opt := opt
										sSel.Item(opt).Children(func() {
											ui.Text(c, opt).FontSize(float32(tok.TextMeta)).TextColor(tok.Text)
										})
									}
								})
								if sSel.Trigger.Changed() && v.OnExcludeSample != nil {
									v.OnExcludeSample(sampleID, *sDraft)
								}
							} else if sStatus != "正常" {
								components.Badge(c, sStatus, components.BadgeOptions{Tone: components.BadgeWarning})
							}
						})
					}
				})
			}

			if editableDesign {
				newKey := v.ensureWidgetDraft("dt:"+moduleID+":ckey", "")
				newLbl := v.ensureWidgetDraft("dt:"+moduleID+":clbl", "")
				newKind := v.ensureWidgetDraft("dt:"+moduleID+":ckind", "number")
				newUnit := v.ensureWidgetDraft("dt:"+moduleID+":cunit", "")
				newFormula := v.ensureWidgetDraft("dt:"+moduleID+":cform", "")
				ui.Row(c).FillWidth().Wrap().Gap(float32(tok.Spacing8)).Padding(float32(tok.Spacing8), 0, 0, 0).
					AlignItems(ui.Center).Children(func() {
					ui.Box(c).Width(float32(tok.Spacing48)*3).Children(func() {
						components.TextBox(c, newLbl, components.TextBoxOptions{}).Placeholder("列名（如菌落数）")
					})
					kSel := components.Select(c, newKind)
					kSel.Trigger.Children(func() {
						ui.Text(c, *newKind).FontSize(float32(tok.TextMeta)).TextColor(tok.Text)
					})
					kSel.Popup(func(panel ui.Element) {
						for _, opt := range []string{"number", "text", "categorical", "formula"} {
							opt := opt
							kSel.Item(opt).Children(func() {
								ui.Text(c, opt).FontSize(float32(tok.TextMeta)).TextColor(tok.Text)
							})
						}
					})
					ui.Box(c).Width(float32(tok.Spacing48)*1.5).Children(func() {
						components.TextBox(c, newUnit, components.TextBoxOptions{}).Placeholder("单位")
					})
					if *newKind == "formula" {
						ui.Box(c).Width(float32(tok.Spacing48)*2.5).Children(func() {
							components.TextBox(c, newFormula, components.TextBoxOptions{}).Placeholder("公式 如 (d1+d2)/2")
						})
					} else {
						ui.Box(c).Width(float32(tok.Spacing48)*1.8).Children(func() {
							components.TextBox(c, newKey, components.TextBoxOptions{}).Placeholder("列键 key")
						})
					}
					if components.Button(c, "添加列", components.ButtonSecondary).Clicked() && strings.TrimSpace(*newLbl) != "" {
						next := append([]map[string]any{}, cols...)
						keyVal := strings.TrimSpace(*newKey)
						if keyVal == "" {
							keyVal = sanitizeIDFrag(*newLbl)
						}
						colObj := map[string]any{
							"key":        keyVal,
							"label":      strings.TrimSpace(*newLbl),
							"value_kind": *newKind,
						}
						if strings.TrimSpace(*newUnit) != "" {
							colObj["unit"] = strings.TrimSpace(*newUnit)
						}
						if *newKind == "formula" {
							colObj["formula"] = strings.TrimSpace(*newFormula)
						}
						next = append(next, colObj)
						v.emitAnyModuleEdit(moduleID, map[string]any{"columns": next})
						*newLbl = ""
						*newKey = ""
						*newUnit = ""
						*newFormula = ""
					}
				})
			}
		})
	})
}

func (v *View) cellDraftKey(moduleID string, rowIdx int, colKey string) string {
	return fmt.Sprintf("dt:%s:%d:%s", moduleID, rowIdx, colKey)
}

func (v *View) dataTableCellVal(moduleID string, rowIdx int, colKey string, pre map[string]any) string {
	ck := v.cellDraftKey(moduleID, rowIdx, colKey)
	if v.TableCells != nil && v.TableCells[moduleID] != nil {
		if saved, ok := v.TableCells[moduleID][ck]; ok {
			return saved
		}
	}
	if d, ok := v.widgetTextDrafts[ck]; ok && d != nil {
		return *d
	}
	if pre != nil && pre[colKey] != nil {
		return formatAnyValue(pre[colKey])
	}
	return ""
}

func (v *View) applyClipboardToDataTable(moduleID string, cols []map[string]any, raw string) {
	if v.TableCells == nil {
		v.TableCells = map[string]map[string]string{}
	}
	if v.TableCells[moduleID] == nil {
		v.TableCells[moduleID] = map[string]string{}
	}
	lines := strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n")
	rIdx := 0
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		sep := ","
		if strings.Contains(line, "\t") {
			sep = "\t"
		}
		parts := strings.Split(line, sep)
		cPtr := 0
		for _, cm := range cols {
			if textValue(cm, "value_kind") == "formula" {
				continue
			}
			if cPtr >= len(parts) {
				break
			}
			ckey := textValue(cm, "key")
			val := strings.TrimSpace(parts[cPtr])
			ck := v.cellDraftKey(moduleID, rIdx, ckey)
			v.TableCells[moduleID][ck] = val
			buf := v.ensureWidgetDraft(ck, val)
			*buf = val
			cPtr++
		}
		rIdx++
	}
}

func (v *View) buildDataTableSubmission(moduleID, schemaID, endpoint string, cols, authoredRows []map[string]any, rowCount int) map[string]any {
	var submittedRows []map[string]any
	for rIdx := 0; rIdx < rowCount; rIdx++ {
		var pre map[string]any
		if rIdx < len(authoredRows) {
			pre = authoredRows[rIdx]
		}
		rowOut := map[string]any{}
		rowVars := map[string]float64{}
		anyValue := false
		for _, cm := range cols {
			ckey := textValue(cm, "key")
			if textValue(cm, "value_kind") == "formula" {
				continue
			}
			raw := v.dataTableCellVal(moduleID, rIdx, ckey, pre)
			if strings.TrimSpace(raw) != "" {
				anyValue = true
				if f, err := strconv.ParseFloat(strings.TrimSpace(raw), 64); err == nil {
					rowOut[ckey] = f
					rowVars[ckey] = f
				} else {
					rowOut[ckey] = strings.TrimSpace(raw)
				}
			}
		}
		for _, cm := range cols {
			if textValue(cm, "value_kind") == "formula" {
				ckey := textValue(cm, "key")
				if res, err := EvalFormula(textValue(cm, "formula"), rowVars); err == nil {
					rowOut[ckey] = round4(res)
				}
			}
		}
		if anyValue {
			submittedRows = append(submittedRows, rowOut)
		}
	}
	return map[string]any{
		"module_id": moduleID,
		"schema_id": firstNonEmpty(schemaID, "data_table.generic.v1"),
		"endpoint":  firstNonEmpty(endpoint, "measurement"),
		"rows":      submittedRows,
	}
}

func rangeWarningText(hasMin bool, minV float64, hasMax bool, maxV float64, unit string) string {
	switch {
	case hasMin && hasMax:
		return fmt.Sprintf("超出质控 %s–%s %s", formatNumber(minV), formatNumber(maxV), unit)
	case hasMin:
		return fmt.Sprintf("低于下限 %s %s", formatNumber(minV), unit)
	default:
		return fmt.Sprintf("高于上限 %s %s", formatNumber(maxV), unit)
	}
}

func mapSlice(raw any) []map[string]any {
	items, _ := raw.([]any)
	out := make([]map[string]any, 0, len(items))
	for _, it := range items {
		if m, ok := it.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}

func removeSliceIndex(rows []map[string]any, idx int) []map[string]any {
	out := make([]map[string]any, 0, len(rows))
	for i, r := range rows {
		if i != idx {
			out = append(out, r)
		}
	}
	return out
}

func formatAnyValue(v any) string {
	if v == nil {
		return ""
	}
	if n, ok := numericValue(v); ok {
		return formatNumber(n)
	}
	return strings.TrimSpace(fmt.Sprint(v))
}

func sanitizeIDFrag(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(strings.TrimSpace(s)) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		} else if r == ' ' || r == '-' || r == '_' {
			b.WriteByte('-')
		}
	}
	res := strings.Trim(b.String(), "-")
	if res == "" {
		return "item"
	}
	return res
}
