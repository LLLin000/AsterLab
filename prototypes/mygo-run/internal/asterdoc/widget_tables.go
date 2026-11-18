package asterdoc

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/onevra/source/components"
	"github.com/oneo-me/onevra/source/icons/lucide"

	"asterlab-mygo-prototype/internal/theme"
	plateui "asterlab-mygo-prototype/internal/uikit/plate"
)

func (v *View) renderGroupTable(c *ui.Context, data map[string]any) {
	t := c.Theme()
	tok := theme.GetTokens(t)
	dark := t != nil && t.Dark
	moduleID := textValue(data, "id")
	title := firstNonEmpty(textValue(data, "title"), "实验分组表")
	editable := v.Mode == ModeDesign && !v.ReadOnly && v.Can(PermDocumentEdit)

	rows := mapSlice(data["rows"])
	idxMap := plateGroupPaletteMap(v.Doc, nil)
	for i, rm := range rows {
		if gid := textValue(rm, "id"); gid != "" {
			if _, seen := idxMap[gid]; !seen {
				idxMap[gid] = i
			}
		}
	}
	moduleCardFrame(v, c, lucide.Table2(), title, fmt.Sprintf("%d 组", len(rows)), nil, func() {
		ui.Column(c).FillWidth().Gap(float32(tok.Spacing8)).Children(func() {
			cols := []ui.TableColumn{
				{ID: "name", Title: "组别名称", MinWidth: 96},
				{ID: "role", Title: "角色", Width: float32(tok.Spacing48) * 2.5},
				{ID: "cond", Title: "处理条件", MinWidth: 112},
				{ID: "rep", Title: "n / 重复", Width: float32(tok.Spacing48) * 1.9, Align: ui.End},
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
									Radius(float32(tok.ControlRadius)).Background(plateui.GroupColorIndexed(gid, idxMap, dark))
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
		return "对照组"
	case "treatment":
		return "处理组"
	case "comparator":
		return "阳性对照"
	case "reference":
		return "参照组"
	default:
		return role
	}
}

func (v *View) renderParamTable(c *ui.Context, data map[string]any) {
	t := c.Theme()
	tok := theme.GetTokens(t)
	moduleID := textValue(data, "id")
	title := firstNonEmpty(textValue(data, "title"), "实验条件")
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
									disp := strings.TrimSpace(recorded)
									if unit != "" && !strings.HasSuffix(disp, unit) {
										disp += " " + unit
									}
									components.Badge(c, "实测 "+disp, components.BadgeOptions{Tone: components.BadgeSuccess})
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
	dark := t != nil && t.Dark
	moduleID := textValue(data, "id")
	title := firstNonEmpty(textValue(data, "title"), "数据记录表")
	schemaID := textValue(data, "schema_id")
	endpoint := textValue(data, "endpoint")
	containerRef := textValue(data, "container_ref")
	cols := mapSlice(data["columns"])
	authoredRows := mapSlice(data["rows"])
	editableDesign := v.Mode == ModeDesign && !v.ReadOnly && v.Can(PermDocumentEdit)
	interactiveRun := v.Mode == ModeRun && !v.ReadOnly

	pl, _ := v.findPlate(containerRef)
	var geom Geometry
	hasGeom := false
	if pl != nil {
		geom, hasGeom = geometryForPlate(pl.Format, pl.Topology, pl.Rows, pl.Columns)
	}
	layoutDraft := v.ensureWidgetDraft("dt_layout:"+moduleID, firstNonEmpty(textValue(data, "layout"), "rows"))
	if !hasGeom {
		*layoutDraft = "rows"
	}

	var numericCols []map[string]any
	for _, cm := range cols {
		if textValue(cm, "value_kind") == "number" {
			numericCols = append(numericCols, cm)
		}
	}
	defaultValCol := ""
	if len(numericCols) > 0 {
		defaultValCol = textValue(numericCols[0], "key")
	} else {
		for _, cm := range cols {
			ck := textValue(cm, "key")
			if textValue(cm, "value_kind") != "formula" && ck != "well" && ck != "well_id" && ck != "sample_id" {
				defaultValCol = ck
				break
			}
		}
		if defaultValCol == "" && len(cols) > 0 {
			defaultValCol = textValue(cols[0], "key")
		}
	}
	valColDraft := v.ensureWidgetDraft("dt_mcol:"+moduleID, defaultValCol)

	extraRowsKey := "dt_extra_rows:" + moduleID
	extraRowsStr := v.ensureWidgetDraft(extraRowsKey, "0")
	extraRows, _ := strconv.Atoi(*extraRowsStr)
	rowCount := len(authoredRows) + extraRows
	if rowCount == 0 && len(authoredRows) == 0 {
		rowCount = 2
	}
	submitRowCount := rowCount
	if hasGeom && *layoutDraft == "matrix" && geom.Rows*geom.Columns > submitRowCount {
		submitRowCount = geom.Rows * geom.Columns
	}
	var editableColKeys []string
	editableColPos := map[string]int{}
	for _, cm := range cols {
		if textValue(cm, "value_kind") == "formula" {
			continue
		}
		ck := textValue(cm, "key")
		editableColPos[ck] = len(editableColKeys)
		editableColKeys = append(editableColKeys, ck)
	}

	badge := fmt.Sprintf("%d 列记录", len(cols))
	if ep := firstNonEmpty(endpoint, schemaID); ep != "" && !IsRawIdentifier(ep) {
		badge = ep
	}
	moduleCardFrame(v, c, lucide.Table2(), title, badge, nil, func() {
		ui.Column(c).FillWidth().Gap(float32(tok.Spacing8)).Children(func() {
			if interactiveRun || hasGeom {
				ui.Row(c).FillWidth().Wrap().AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
					if hasGeom {
						rowVar := map[bool]components.ButtonVariant{true: components.ButtonSecondary, false: components.ButtonOutline}[*layoutDraft != "matrix"]
						matVar := map[bool]components.ButtonVariant{true: components.ButtonSecondary, false: components.ButtonOutline}[*layoutDraft == "matrix"]
						if components.Button(c, "列表", rowVar).Clicked() && *layoutDraft != "rows" {
							*layoutDraft = "rows"
							if editableDesign {
								v.emitAnyModuleEdit(moduleID, map[string]any{"layout": "rows"})
							}
						}
						if components.Button(c, "孔板矩阵", matVar).Clicked() && *layoutDraft != "matrix" {
							*layoutDraft = "matrix"
							if editableDesign {
								v.emitAnyModuleEdit(moduleID, map[string]any{"layout": "matrix"})
							}
						}
						if *layoutDraft == "matrix" && len(numericCols) > 1 {
							for _, ncm := range numericCols {
								nKey := textValue(ncm, "key")
								nLbl := firstNonEmpty(textValue(ncm, "label"), nKey)
								cv := map[bool]components.ButtonVariant{true: components.ButtonSecondary, false: components.ButtonGhost}[*valColDraft == nKey]
								if components.Button(c, nLbl, cv).Clicked() {
									*valColDraft = nKey
								}
							}
						}
					}
					if interactiveRun {
						if hasGeom && *layoutDraft == "matrix" {
							if iconButton(c, lucide.Grid3x3(), "粘贴整板矩阵", components.ButtonOutline).Clicked() {
								clip := c.Services().ReadClipboard()
								if strings.TrimSpace(clip) != "" {
									v.ApplyMatrixClipboardToDataTable(moduleID, pl, cols, authoredRows, *valColDraft, clip)
								}
							}
						} else {
							if iconButton(c, lucide.ClipboardPaste(), "粘贴 CSV/TSV", components.ButtonOutline).Clicked() {
								clip := c.Services().ReadClipboard()
								if strings.TrimSpace(clip) != "" {
									v.applyClipboardToDataTableAt(moduleID, cols, len(authoredRows), 0, 0, clip)
								}
							}
							if iconButton(c, lucide.Plus(), "新增行", components.ButtonOutline).Clicked() {
								*extraRowsStr = strconv.Itoa(extraRows + 1)
							}
						}
					}
					ui.Spacer(c)
					if interactiveRun {
						if st := v.TableSubmitStatus[moduleID]; st != "" {
							ui.Row(c.Key("tbl-status:" + moduleID)).AlignItems(ui.Center).Children(func() {
								components.Badge(c, st, components.BadgeOptions{Tone: components.BadgeSuccess})
							})
						}
						if iconButton(c, lucide.Check(), "保存本表数据", components.ButtonSecondary).Clicked() {
							payload, buildErr := v.buildDataTableSubmission(moduleID, schemaID, endpoint, cols, authoredRows, submitRowCount)
							if v.TableSubmitErrors == nil {
								v.TableSubmitErrors = map[string]string{}
							}
							if v.TableSubmitStatus == nil {
								v.TableSubmitStatus = map[string]string{}
							}
							if buildErr != nil {
								v.TableSubmitStatus[moduleID] = ""
								v.TableSubmitErrors[moduleID] = buildErr.Error()
							} else if v.OnSubmitTableReadout != nil {
								if subErr := v.OnSubmitTableReadout(v.StepID, moduleID, payload); subErr != nil {
									v.TableSubmitStatus[moduleID] = ""
									v.TableSubmitErrors[moduleID] = subErr.Error()
								} else {
									v.TableSubmitErrors[moduleID] = ""
									v.TableSubmitStatus[moduleID] = "数据已记录"
								}
							}
						}
					}
				})
				if subErr := v.TableSubmitErrors[moduleID]; subErr != "" {
					ui.Row(c).FillWidth().Padding(float32(tok.Spacing8), float32(tok.Spacing12)).
						Radius(float32(tok.ControlRadius)).Background(tok.Danger.Alpha(.08)).Border(1, tok.Danger).
						AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
						ui.Text(c, "提交失败："+subErr).FontSize(float32(tok.TextMeta)).TextColor(tok.Danger)
					})
				}
			}

			if hasGeom && *layoutDraft == "matrix" && *valColDraft != "" {
				v.renderDataTableMatrix(c, tok, dark, moduleID, pl, geom, cols, authoredRows, *valColDraft, interactiveRun)
				if editableDesign {
					v.renderDataTableAddColBar(c, tok, moduleID, cols)
				}
				return
			}

			// Column headers with unit & threshold pills
			ui.Row(c).FillWidth().Padding(float32(tok.Spacing4), float32(tok.Spacing8)).
				Radius(float32(tok.ControlRadius)).Background(tok.Elevated).Gap(float32(tok.Spacing8)).Children(func() {
				for cIdx, cm := range cols {
					cIdx, cm := cIdx, cm
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
						ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(float32(tok.Spacing4)).Children(func() {
							ui.Box(c).Grow(1).MinWidth(0).Children(func() {
								ui.Text(c, hdr).FontSize(float32(tok.TextMeta)).Bold().TextColor(tok.TextMuted)
							})
							if editableDesign && len(cols) > 1 {
								delCol := components.IconButton(c, lucide.Trash(), "删除列 "+clabel, components.IconButtonCompact)
								components.Tooltip(c, delCol, "删除列 "+clabel)
								if delCol.Clicked() {
									nextCols := removeSliceIndex(cols, cIdx)
									v.emitAnyModuleEdit(moduleID, map[string]any{"columns": nextCols})
								}
							}
						})
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
					ui.Box(c).Width(float32(tok.Spacing48)*2.9).Children(func() {
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
								tb := components.TextBox(c, buf, components.TextBoxOptions{}).Placeholder("输入")
								tb.FocusBind(&v.FocusedTableCell, cellKey)
								eColIdx := editableColPos[ckey]
								rowIdxSnap := rIdx
								colKeySnap := ckey
								tb.OnPaste(func(text string) bool {
									if !strings.ContainsAny(text, "\t\n\r") {
										return false
									}
									v.applyClipboardToDataTableAt(moduleID, cols, len(authoredRows), rowIdxSnap, eColIdx, text)
									return true
								})
								tb.OnTextKey(func(k ui.TextKey) bool {
									switch k.Key {
									case ui.KeyDown:
										if rowIdxSnap+1 < rowCount {
											*buf = k.Text()
											v.setTableCell(moduleID, v.cellDraftKey(moduleID, rowIdxSnap, colKeySnap), *buf)
											v.FocusedTableCell = v.cellDraftKey(moduleID, rowIdxSnap+1, colKeySnap)
											return true
										}
									case ui.KeyUp:
										if rowIdxSnap > 0 {
											*buf = k.Text()
											v.setTableCell(moduleID, v.cellDraftKey(moduleID, rowIdxSnap, colKeySnap), *buf)
											v.FocusedTableCell = v.cellDraftKey(moduleID, rowIdxSnap-1, colKeySnap)
											return true
										}
									case ui.KeyEnter:
										if k.Mods == 0 {
											*buf = k.Text()
											v.setTableCell(moduleID, v.cellDraftKey(moduleID, rowIdxSnap, colKeySnap), *buf)
											if rowIdxSnap+1 >= rowCount {
												*extraRowsStr = strconv.Itoa(extraRows + 1)
											}
											v.FocusedTableCell = v.cellDraftKey(moduleID, rowIdxSnap+1, colKeySnap)
											return true
										}
									case ui.KeyTab:
										*buf = k.Text()
										v.setTableCell(moduleID, v.cellDraftKey(moduleID, rowIdxSnap, colKeySnap), *buf)
										if len(editableColKeys) == 0 {
											return false
										}
										if k.Mods&ui.Shift != 0 {
											if eColIdx > 0 {
												v.FocusedTableCell = v.cellDraftKey(moduleID, rowIdxSnap, editableColKeys[eColIdx-1])
												return true
											} else if rowIdxSnap > 0 {
												v.FocusedTableCell = v.cellDraftKey(moduleID, rowIdxSnap-1, editableColKeys[len(editableColKeys)-1])
												return true
											}
										} else {
											if eColIdx+1 < len(editableColKeys) {
												v.FocusedTableCell = v.cellDraftKey(moduleID, rowIdxSnap, editableColKeys[eColIdx+1])
												return true
											} else if rowIdxSnap+1 < rowCount {
												v.FocusedTableCell = v.cellDraftKey(moduleID, rowIdxSnap+1, editableColKeys[0])
												return true
											}
										}
									}
									return false
								})
								v.setTableCell(moduleID, cellKey, *buf)
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
						ui.Column(c).Width(float32(tok.Spacing48)*2.9).Gap(float32(tok.Spacing4)).Children(func() {
							qKey := fmt.Sprintf("dt_qual:%s:%d", moduleID, rIdx)
							qDraft := v.ensureWidgetDraft(qKey, "正常")
							if interactiveRun {
								qSel := components.Select(c, qDraft)
								qSel.Trigger.Children(func() {
									ui.Text(c, "读数: "+*qDraft).FontSize(float32(tok.TextMeta)).TextColor(tok.Text)
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
									ui.Text(c, "样本: "+*sDraft).FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
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
				v.renderDataTableAddColBar(c, tok, moduleID, cols)
			}
		})
	})
}

func (v *View) renderDataTableAddColBar(c *ui.Context, tok theme.Tokens, moduleID string, cols []map[string]any) {
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

func (v *View) renderDataTableMatrix(
	c *ui.Context,
	tok theme.Tokens,
	dark bool,
	moduleID string,
	pl *Plate,
	geom Geometry,
	cols, authoredRows []map[string]any,
	valColKey string,
	interactiveRun bool,
) {
	wellGroups := pl.WellGroups()
	idxMap := plateGroupPaletteMap(v.Doc, pl)
	wellToRow, wellColKey, sampleColKey := dataTableWellRowMap(cols, authoredRows, geom)

	var colMeta map[string]any
	for _, cm := range cols {
		if textValue(cm, "key") == valColKey {
			colMeta = cm
			break
		}
	}
	minV, hasMin := numericValue(colMeta["min"])
	maxV, hasMax := numericValue(colMeta["max"])
	colUnit := textValue(colMeta, "unit")

	cellW := float32(66)
	if geom.Columns <= 6 {
		cellW = 92
	} else if geom.Columns > 12 {
		cellW = 54
	}
	rowHdrW := float32(tok.Spacing24)

	drawGrid := func() {
		ui.Column(c).Gap(float32(tok.Spacing4)).Children(func() {
			ui.Row(c).Gap(float32(tok.Spacing4)).AlignItems(ui.Center).Children(func() {
				ui.Box(c).Width(rowHdrW)
				for col := 0; col < geom.Columns; col++ {
					ui.Box(c).Width(cellW).Center().Children(func() {
						ui.Text(c, fmt.Sprintf("%d", col+1)).FontSize(float32(tok.TextMeta)).Bold().TextColor(tok.TextMuted).FontFeatures("tnum")
					})
				}
			})
			for r := 0; r < geom.Rows; r++ {
				r := r
				ui.Row(c).Gap(float32(tok.Spacing4)).AlignItems(ui.Center).Children(func() {
					ui.Box(c).Width(rowHdrW).Center().Children(func() {
						ui.Text(c, rowLabel(r)).FontSize(float32(tok.TextMeta)).Bold().TextColor(tok.TextMuted)
					})
					for col := 0; col < geom.Columns; col++ {
						col := col
						wID := fmt.Sprintf("%s%d", rowLabel(r), col+1)
						rIdx := wellToRow[wID]
						var pre map[string]any
						if rIdx < len(authoredRows) {
							pre = authoredRows[rIdx]
						}
						cellKey := v.cellDraftKey(moduleID, rIdx, valColKey)
						initStr := ""
						if pre != nil && pre[valColKey] != nil {
							initStr = formatAnyValue(pre[valColKey])
						}
						buf := v.ensureWidgetDraft(cellKey, initStr)
						if v.TableCells != nil && v.TableCells[moduleID] != nil {
							if saved, exists := v.TableCells[moduleID][cellKey]; exists {
								*buf = saved
							}
						}
						if wellColKey != "" && strings.TrimSpace(v.dataTableCellVal(moduleID, rIdx, wellColKey, pre)) == "" {
							v.setTableCell(moduleID, v.cellDraftKey(moduleID, rIdx, wellColKey), wID)
						}
						if sampleColKey != "" && pl.Samples[wID] != "" && strings.TrimSpace(v.dataTableCellVal(moduleID, rIdx, sampleColKey, pre)) == "" {
							v.setTableCell(moduleID, v.cellDraftKey(moduleID, rIdx, sampleColKey), pl.Samples[wID])
						}

						outOfRange := false
						if strings.TrimSpace(*buf) != "" {
							if nv, err := strconv.ParseFloat(strings.TrimSpace(*buf), 64); err == nil {
								outOfRange = (hasMin && nv < minV) || (hasMax && nv > maxV)
							}
						}
						gid := wellGroups[wID]
						dotCol := plateui.GroupColorIndexed(gid, idxMap, dark)
						if gid == "" {
							dotCol = tok.Border
						}
						borderCol := tok.Border
						cellBg := tok.Elevated
						if strings.TrimSpace(*buf) != "" {
							cellBg = tok.Surface
						}
						if outOfRange {
							borderCol = tok.Warning
						}
						ui.Column(c).Width(cellW).Padding(float32(tok.Spacing4)).
							Radius(float32(tok.ControlRadius)).Background(cellBg).Border(1, borderCol).
							Gap(2).Children(func() {
							ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(float32(tok.Spacing4)).Children(func() {
								ui.Box(c).Size(6, 6).Radius(3).Background(dotCol)
								lbl := wID
								if sid := pl.Samples[wID]; sid != "" && geom.Columns <= 12 {
									rs := []rune(sid)
									if len(rs) > 4 {
										sid = string(rs[:4])
									}
									lbl = wID + " " + sid
								}
								ui.Text(c, lbl).FontSize(float32(tok.TextMeta)*0.82).TextColor(tok.TextMuted).SingleLine()
							})
							if interactiveRun {
								tb := components.TextBox(c, buf, components.TextBoxOptions{}).Placeholder("—")
								tb.FocusBind(&v.FocusedTableCell, cellKey)
								tb.OnPaste(func(text string) bool {
									if !strings.ContainsAny(text, "\t\n\r") {
										return false
									}
									v.ApplyMatrixClipboardToDataTable(moduleID, pl, cols, authoredRows, valColKey, text)
									return true
								})
								v.setTableCell(moduleID, cellKey, *buf)
							} else {
								disp := firstNonEmpty(*buf, "—")
								ui.Text(c, disp).FontSize(float32(tok.TextData)).Bold().TextColor(tok.Text).FontFeatures("tnum").SingleLine()
							}
						})
					}
				})
			}
			if colUnit != "" || hasMin || hasMax {
				meta := firstNonEmpty(textValue(colMeta, "label"), valColKey)
				if colUnit != "" {
					meta += " (" + colUnit + ")"
				}
				if hasMin || hasMax {
					meta += " · " + rangeWarningText(hasMin, minV, hasMax, maxV, colUnit)
				}
				ui.Text(c, meta).FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
			}
		})
	}
	if geom.Columns > 8 {
		ui.ScrollHorizontal(c).FillWidth().Children(drawGrid)
	} else {
		drawGrid()
	}
}

func dataTableWellRowMap(cols, authoredRows []map[string]any, geom Geometry) (wellToRow map[string]int, wellColKey, sampleColKey string) {
	wellToRow = make(map[string]int, geom.Rows*geom.Columns)
	for _, cm := range cols {
		k := textValue(cm, "key")
		switch k {
		case "well", "well_id":
			if wellColKey == "" {
				wellColKey = k
			}
		case "sample_id":
			if sampleColKey == "" {
				sampleColKey = k
			}
		}
	}
	usedRow := map[int]bool{}
	for idx, rm := range authoredRows {
		w := strings.ToUpper(strings.TrimSpace(firstNonEmpty(textValue(rm, "well"), textValue(rm, "well_id"))))
		if w != "" {
			wellToRow[w] = idx
			usedRow[idx] = true
		}
	}
	nextIdx := len(authoredRows)
	for r := 0; r < geom.Rows; r++ {
		for col := 0; col < geom.Columns; col++ {
			wID := fmt.Sprintf("%s%d", rowLabel(r), col+1)
			if _, ok := wellToRow[wID]; ok {
				continue
			}
			if len(authoredRows) == 0 {
				wellToRow[wID] = r*geom.Columns + col
			} else {
				for usedRow[nextIdx] {
					nextIdx++
				}
				wellToRow[wID] = nextIdx
				usedRow[nextIdx] = true
				nextIdx++
			}
		}
	}
	return wellToRow, wellColKey, sampleColKey
}

// ApplyMatrixClipboardToDataTable parses an R×C TSV/CSV plate matrix (optionally
// including a 1..C column header row and A..P row header column) into well rows.
func (v *View) ApplyMatrixClipboardToDataTable(moduleID string, pl *Plate, cols, authoredRows []map[string]any, valColKey, raw string) int {
	if pl == nil || valColKey == "" {
		return 0
	}
	geom, ok := geometryForPlate(pl.Format, pl.Topology, pl.Rows, pl.Columns)
	if !ok {
		return 0
	}
	lines := strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n")
	var parsed [][]string
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		sep := ","
		if strings.Contains(line, "\t") {
			sep = "\t"
		}
		parts := strings.Split(line, sep)
		for i := range parts {
			parts[i] = strings.TrimSpace(parts[i])
		}
		parsed = append(parsed, parts)
	}
	if len(parsed) == 0 {
		return 0
	}
	// Strip optional column header row ("1\t2\t3" or "<corner>\t1\t2\t3" from SoftMax/Bio-Rad/Excel)
	isSeq1ToN := func(cells []string) bool {
		if len(cells) < 2 {
			return false
		}
		for idx, cell := range cells {
			if cell != strconv.Itoa(idx+1) {
				return false
			}
		}
		return true
	}
	if len(parsed) > 1 {
		if isSeq1ToN(parsed[0]) {
			parsed = parsed[1:]
		} else if len(parsed[0]) > 2 && isSeq1ToN(parsed[0][1:]) {
			if _, err := strconv.ParseFloat(parsed[0][0], 64); err != nil {
				parsed = parsed[1:]
			}
		}
	}
	// Strip optional A..P row label first column when consistent
	if len(parsed) > 0 && len(parsed[0]) > 1 {
		allRowHdr := true
		for r, parts := range parsed {
			if len(parts) < 2 || !strings.EqualFold(parts[0], rowLabel(r)) {
				allRowHdr = false
				break
			}
		}
		if allRowHdr {
			for r := range parsed {
				parsed[r] = parsed[r][1:]
			}
		}
	}

	wellToRow, wellColKey, sampleColKey := dataTableWellRowMap(cols, authoredRows, geom)
	filled := 0
	maxRowReached := -1
	for r, parts := range parsed {
		if r >= geom.Rows {
			break
		}
		for col, val := range parts {
			if col >= geom.Columns || val == "" {
				continue
			}
			wID := fmt.Sprintf("%s%d", rowLabel(r), col+1)
			rIdx := wellToRow[wID]
			ck := v.cellDraftKey(moduleID, rIdx, valColKey)
			v.setTableCell(moduleID, ck, val)
			buf := v.ensureWidgetDraft(ck, val)
			*buf = val
			if wellColKey != "" {
				wck := v.cellDraftKey(moduleID, rIdx, wellColKey)
				v.setTableCell(moduleID, wck, wID)
				wbuf := v.ensureWidgetDraft(wck, wID)
				*wbuf = wID
			}
			if sampleColKey != "" {
				var pre map[string]any
				if rIdx < len(authoredRows) {
					pre = authoredRows[rIdx]
				}
				if strings.TrimSpace(v.dataTableCellVal(moduleID, rIdx, sampleColKey, pre)) == "" {
					sid := firstNonEmpty(pl.Samples[wID], wID)
					sck := v.cellDraftKey(moduleID, rIdx, sampleColKey)
					v.setTableCell(moduleID, sck, sid)
					sbuf := v.ensureWidgetDraft(sck, sid)
					*sbuf = sid
				}
			}
			filled++
			if rIdx > maxRowReached {
				maxRowReached = rIdx
			}
		}
	}
	if maxRowReached >= 0 {
		extraPtr := v.ensureWidgetDraft("dt_extra_rows:"+moduleID, "0")
		curExtra, _ := strconv.Atoi(*extraPtr)
		curTotal := len(authoredRows) + curExtra
		if curTotal == 0 {
			curTotal = 2
		}
		if maxRowReached+1 > curTotal {
			needed := maxRowReached + 1 - len(authoredRows)
			if needed < 0 {
				needed = 0
			}
			*extraPtr = strconv.Itoa(needed)
		}
	}
	return filled
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

func (v *View) setTableCell(moduleID, cellKey, val string) {
	if v.TableCells == nil {
		v.TableCells = map[string]map[string]string{}
	}
	if v.TableCells[moduleID] == nil {
		v.TableCells[moduleID] = map[string]string{}
	}
	v.TableCells[moduleID][cellKey] = val
}

func (v *View) applyClipboardToDataTable(moduleID string, cols []map[string]any, raw string) {
	v.applyClipboardToDataTableAt(moduleID, cols, 0, 0, 0, raw)
}

func (v *View) applyClipboardToDataTableAt(moduleID string, cols []map[string]any, authoredRowCount, startRow, startEditableCol int, raw string) int {
	var editableCols []map[string]any
	for _, cm := range cols {
		if textValue(cm, "value_kind") == "formula" {
			continue
		}
		editableCols = append(editableCols, cm)
	}
	if len(editableCols) == 0 {
		return 0
	}
	lines := strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n")
	var parsedLines [][]string
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		sep := ","
		if strings.Contains(line, "\t") {
			sep = "\t"
		}
		parts := strings.Split(line, sep)
		for i := range parts {
			parts[i] = strings.TrimSpace(parts[i])
		}
		parsedLines = append(parsedLines, parts)
	}
	if len(parsedLines) == 0 {
		return 0
	}
	// Auto-skip header row when pasted from Excel/CSV starting at first editable column
	if startEditableCol == 0 && len(parsedLines) > 1 {
		hdrMatches := 0
		for i, cm := range editableCols {
			if i >= len(parsedLines[0]) {
				break
			}
			cell := parsedLines[0][i]
			key := textValue(cm, "key")
			lbl := textValue(cm, "label")
			if strings.EqualFold(cell, key) || (lbl != "" && strings.EqualFold(cell, lbl)) {
				hdrMatches++
			}
		}
		if hdrMatches >= 2 || (len(editableCols) == 1 && hdrMatches == 1) {
			parsedLines = parsedLines[1:]
		}
	}
	filledRows := 0
	maxRowReached := startRow - 1
	for rowOff, parts := range parsedLines {
		rIdx := startRow + rowOff
		wrote := false
		for cOff, val := range parts {
			colIdx := startEditableCol + cOff
			if colIdx >= len(editableCols) {
				break
			}
			ckey := textValue(editableCols[colIdx], "key")
			ck := v.cellDraftKey(moduleID, rIdx, ckey)
			v.setTableCell(moduleID, ck, val)
			buf := v.ensureWidgetDraft(ck, val)
			*buf = val
			wrote = true
		}
		if wrote {
			filledRows++
			if rIdx > maxRowReached {
				maxRowReached = rIdx
			}
		}
	}
	if maxRowReached >= 0 {
		extraPtr := v.ensureWidgetDraft("dt_extra_rows:"+moduleID, "0")
		curExtra, _ := strconv.Atoi(*extraPtr)
		curTotal := authoredRowCount + curExtra
		if curTotal == 0 {
			curTotal = 2
		}
		if maxRowReached+1 > curTotal {
			neededExtra := maxRowReached + 1 - authoredRowCount
			if neededExtra < 0 {
				neededExtra = 0
			}
			*extraPtr = strconv.Itoa(neededExtra)
		}
	}
	return filledRows
}

func resolveSafeTableSchemaID(moduleID, schemaID string) string {
	id := strings.TrimSpace(schemaID)
	switch id {
	case "":
		return "data_table." + sanitizeIDFrag(moduleID) + ".v1"
	case "cck8_od450", "rna_concentration", "rna_purity", "cfu_count":
		return id + ".table"
	default:
		return id
	}
}

func (v *View) buildDataTableSubmission(moduleID, schemaID, endpoint string, cols, authoredRows []map[string]any, rowCount int) (map[string]any, error) {
	if len(cols) == 0 {
		return nil, fmt.Errorf("数据表未定义任何列")
	}
	colKeys := make([]string, 0, len(cols))
	schemaCols := make([]map[string]any, 0, len(cols))
	seenCols := map[string]bool{}
	for _, cm := range cols {
		ckey := textValue(cm, "key")
		if ckey == "" {
			return nil, fmt.Errorf("数据表存在缺少 key 的列")
		}
		if seenCols[ckey] {
			return nil, fmt.Errorf("数据表列名重复：%s", ckey)
		}
		seenCols[ckey] = true
		colKeys = append(colKeys, ckey)

		clabel := firstNonEmpty(textValue(cm, "label"), ckey)
		vkind := textValue(cm, "value_kind")
		schemaKind := "categorical"
		if vkind == "number" || vkind == "formula" {
			schemaKind = "numeric"
		}
		sc := map[string]any{
			"key":        ckey,
			"label":      clabel,
			"value_kind": schemaKind,
		}
		if unit := textValue(cm, "unit"); unit != "" {
			sc["unit"] = unit
		}
		schemaCols = append(schemaCols, sc)
	}

	resolvedSchemaID := resolveSafeTableSchemaID(moduleID, schemaID)
	resolvedEndpoint := firstNonEmpty(endpoint, "measurement")
	schemaDef := map[string]any{
		"schema_id":      resolvedSchemaID,
		"schema_version": 1,
		"endpoint":       resolvedEndpoint,
		"value_kind":     "table",
		"unit":           map[string]any{"required": false},
		"missing": map[string]any{
			"allowed":      []string{"not_detected", "below_lod", "above_uloq", "invalid", "other"},
			"null_allowed": true,
		},
		"source": map[string]any{"raw": true, "derived": false},
		"link":   map[string]any{"timepoint_required": false, "sample_required": false},
		"table":  map[string]any{"columns": schemaCols},
	}
	schemaBytes, err := json.Marshal(schemaDef)
	if err != nil {
		return nil, fmt.Errorf("整理表格结构失败：%w", err)
	}

	var rowsOut [][]any
	for rIdx := 0; rIdx < rowCount; rIdx++ {
		var pre map[string]any
		if rIdx < len(authoredRows) {
			pre = authoredRows[rIdx]
		}
		rowVars := map[string]float64{}
		for _, cm := range cols {
			if textValue(cm, "value_kind") == "formula" {
				continue
			}
			ckey := textValue(cm, "key")
			raw := strings.TrimSpace(v.dataTableCellVal(moduleID, rIdx, ckey, pre))
			if f, err := strconv.ParseFloat(raw, 64); err == nil {
				rowVars[ckey] = f
			}
		}
		rowVals := make([]any, len(cols))
		anyNonNil := false
		for cIdx, cm := range cols {
			ckey := textValue(cm, "key")
			clabel := firstNonEmpty(textValue(cm, "label"), ckey)
			vkind := textValue(cm, "value_kind")
			if vkind == "formula" {
				if val, err := EvalFormula(textValue(cm, "formula"), rowVars); err == nil {
					rowVals[cIdx] = round4(val)
					anyNonNil = true
				}
				continue
			}
			raw := strings.TrimSpace(v.dataTableCellVal(moduleID, rIdx, ckey, pre))
			if raw == "" {
				rowVals[cIdx] = nil
				continue
			}
			if vkind == "number" {
				f, err := strconv.ParseFloat(raw, 64)
				if err != nil {
					return nil, fmt.Errorf("第 %d 行列 %s 需要数值（当前为 %q）", rIdx+1, clabel, raw)
				}
				rowVals[cIdx] = f
			} else {
				// Categorical/text cells must stay JSON string (even if text looks like "1")
				rowVals[cIdx] = raw
			}
			anyNonNil = true
		}
		if anyNonNil {
			rowsOut = append(rowsOut, rowVals)
		}
	}
	if len(rowsOut) == 0 {
		return nil, fmt.Errorf("请至少填写一行数据再提交")
	}
	return map[string]any{
		"module_id":       moduleID,
		"schema_id":       resolvedSchemaID,
		"schema_def_json": string(schemaBytes),
		"endpoint":        resolvedEndpoint,
		"columns":         colKeys,
		"rows":            rowsOut,
	}, nil
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

// IsRawIdentifier reports whether s is a raw technical ASCII identifier or path
// (containing '_', '-', or '.') rather than human-readable prose or unit text.
func IsRawIdentifier(s string) bool {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return false
	}
	hasSep := false
	for _, r := range trimmed {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '/':
		case r == '_' || r == '-' || r == '.':
			hasSep = true
		default:
			return false
		}
	}
	return hasSep
}
