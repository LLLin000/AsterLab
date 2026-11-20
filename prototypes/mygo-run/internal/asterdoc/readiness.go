package asterdoc

import (
	"fmt"
	"strings"
	"time"
)

// ExperimentMeta summarizes an AsterML document's scheduling metadata,
// objective completeness status, and scientific budget without inventing defaults.
type ExperimentMeta struct {
	Title            string
	ScheduledDate    string // YYYY-MM-DD when declared in experiment@1.scheduled_date
	Objective        string
	PrimaryEndpoint  string
	FirstGapStepID   string
	Complete         bool
	IncompleteReason string
	Summary          string
	Budget           BudgetSummary
}

var placeholderTitles = map[string]bool{
	"":        true,
	"新实验":     true,
	"新建实验设计":  true,
	"未命名实验":   true,
	"未命名实验方案": true,
	"未命名设计":   true,
}

// InspectExperiment inspects an AsterML v2 document and returns objective
// completeness criteria plus scheduling metadata.
func InspectExperiment(source string, coreErrors []string) ExperimentMeta {
	var meta ExperimentMeta
	doc, err := Parse(source)
	if err != nil {
		meta.Complete = false
		meta.IncompleteReason = "方案语法待修复"
		meta.Summary = "存在语法问题，需在源码中修正"
		return meta
	}
	meta.Budget = ComputeBudgetSummary(doc)

	for _, m := range doc.Modules {
		if m.Kind == "experiment" {
			meta.Title = strings.TrimSpace(textValue(m.Data, "title"))
			meta.ScheduledDate = NormalizeDateYMD(textValue(m.Data, "scheduled_date"))
			meta.Objective = strings.TrimSpace(textValue(m.Data, "objective"))
			meta.PrimaryEndpoint = strings.TrimSpace(textValue(m.Data, "primary_endpoint"))
			break
		}
	}

	if len(coreErrors) > 0 && strings.TrimSpace(coreErrors[0]) != "" {
		meta.Complete = false
		meta.IncompleteReason = strings.TrimSpace(coreErrors[0])
		meta.Summary = formatBudgetSummaryLine(meta.Budget, meta.IncompleteReason)
		return meta
	}

	if placeholderTitles[meta.Title] {
		meta.Complete = false
		meta.IncompleteReason = "待填写实验标题"
		meta.Summary = formatBudgetSummaryLine(meta.Budget, meta.IncompleteReason)
		return meta
	}

	var execSteps []*Module
	var allPlates []*Module
	var allDataTables []map[string]any
	var allParamTables []map[string]any

	for i := range doc.Modules {
		m := &doc.Modules[i]
		switch m.Kind {
		case "step":
			execMode := firstNonEmpty(textValue(m.Data, "execution"), "executable")
			if execMode != "organizational" {
				execSteps = append(execSteps, m)
			}
			for _, bm := range mapSlice(m.Data["blocks"]) {
				k := textValue(bm, "kind")
				switch k {
				case "plate":
					cp := Module{Kind: "plate", ID: textValue(bm, "id"), Data: bm}
					allPlates = append(allPlates, &cp)
				case "data_table":
					allDataTables = append(allDataTables, bm)
				case "param_table":
					allParamTables = append(allParamTables, bm)
				}
			}
		case "plate":
			allPlates = append(allPlates, m)
		case "data_table":
			allDataTables = append(allDataTables, m.Data)
		case "param_table":
			allParamTables = append(allParamTables, m.Data)
		}
	}

	if len(execSteps) == 0 {
		meta.Complete = false
		meta.IncompleteReason = "待添加实验步骤"
		meta.Summary = formatBudgetSummaryLine(meta.Budget, meta.IncompleteReason)
		return meta
	}

	for _, step := range execSteps {
		sTitle := strings.TrimSpace(textValue(step.Data, "title"))
		if sTitle == "" {
			meta.Complete = false
			meta.FirstGapStepID = step.ID
			meta.IncompleteReason = "存在未命名步骤"
			meta.Summary = formatBudgetSummaryLine(meta.Budget, meta.IncompleteReason)
			return meta
		}
		blocks := mapSlice(step.Data["blocks"])
		refs := stringList(step.Data, "block_refs")
		if len(blocks) == 0 && len(refs) == 0 {
			meta.Complete = false
			meta.FirstGapStepID = step.ID
			meta.IncompleteReason = fmt.Sprintf("步骤「%s」待补充实验模块", stripLeadingStepNumber(sTitle))
			meta.Summary = formatBudgetSummaryLine(meta.Budget, meta.IncompleteReason)
			return meta
		}
	}

	for _, pm := range allPlates {
		assigned := 0
		if pm.Plate != nil {
			for _, wells := range pm.Plate.Groups {
				assigned += len(wells)
			}
		} else if rawGroups, ok := pm.Data["groups"].(map[string]any); ok {
			for _, rawList := range rawGroups {
				if arr, ok := rawList.([]any); ok {
					assigned += len(arr)
				}
			}
		}
		if assigned == 0 {
			meta.Complete = false
			meta.IncompleteReason = "孔板尚未分配样本孔位"
			meta.Summary = formatBudgetSummaryLine(meta.Budget, meta.IncompleteReason)
			return meta
		}
	}

	for _, dt := range allDataTables {
		if len(mapSlice(dt["columns"])) == 0 {
			meta.Complete = false
			meta.IncompleteReason = "表格模块字段待完善"
			meta.Summary = formatBudgetSummaryLine(meta.Budget, meta.IncompleteReason)
			return meta
		}
	}

	for _, pt := range allParamTables {
		if len(mapSlice(pt["rows"])) == 0 {
			meta.Complete = false
			meta.IncompleteReason = "表格模块字段待完善"
			meta.Summary = formatBudgetSummaryLine(meta.Budget, meta.IncompleteReason)
			return meta
		}
	}

	if crossWarns := CrossModuleWarnings(doc); len(crossWarns) > 0 {
		meta.Complete = false
		meta.IncompleteReason = crossWarns[0]
		meta.Summary = formatBudgetSummaryLine(meta.Budget, meta.IncompleteReason)
		return meta
	}

	meta.Complete = true
	meta.Summary = formatBudgetSummaryLine(meta.Budget, "")
	return meta
}

// CrossModuleWarnings checks quantitative consistency across modules (such as
// calculator.sample_count vs referenced plate assigned wells, and group independent_n
// vs plate well counts).
func CrossModuleWarnings(doc *Document) []string {
	if doc == nil {
		return nil
	}
	flat, _ := flattenModules(doc.Modules)
	type plateStat struct {
		total      int
		hasFactors bool
		byGroup    map[string]int
	}
	plates := map[string]plateStat{}
	for _, m := range flat {
		if m.Kind != "plate" {
			continue
		}
		hasFactors := false
		if fm, ok := m.Data["factors"].(map[string]any); ok && len(fm) > 0 {
			hasFactors = true
		}
		all := map[string]bool{}
		byG := map[string]int{}
		if rawGroups, ok := m.Data["groups"].(map[string]any); ok {
			for gk, gv := range rawGroups {
				gkey := strings.TrimSpace(gk)
				if gkey == "" {
					continue
				}
				gset := map[string]bool{}
				if arr, ok := gv.([]any); ok {
					for _, rawW := range arr {
						if ws, ok := rawW.(string); ok {
							w := strings.ToUpper(strings.TrimSpace(ws))
							if w != "" {
								gset[w] = true
								all[w] = true
							}
						}
					}
				}
				if len(gset) > 0 {
					byG[gkey] += len(gset)
				}
			}
		}
		plates[m.ID] = plateStat{total: len(all), hasFactors: hasFactors, byGroup: byG}
	}

	var warnings []string
	for _, m := range flat {
		if m.Kind != "calculator" {
			continue
		}
		pref := firstNonEmpty(textValue(m.Data, "plate_ref"), textValue(m.Data, "container_ref"))
		scF, _ := numericValue(m.Data["sample_count"])
		sc := int(scF)
		if pref != "" && sc > 0 {
			if ps, ok := plates[pref]; ok && ps.total > 0 && sc != ps.total {
				warnings = append(warnings, fmt.Sprintf("calculator %s sample_count (%d) does not match plate %s assigned wells (%d)", m.ID, sc, pref, ps.total))
			}
		}
	}

	checkGroup := func(row map[string]any) {
		gid := textValue(row, "id")
		gname := textValue(row, "name")
		indNF, _ := numericValue(row["independent_n"])
		indN := int(indNF)
		if indN <= 0 {
			return
		}
		repF, hasRep := numericValue(row["technical_repeats"])
		rep := int(repF)
		matched := 0
		hasFactors := false
		for _, ps := range plates {
			if gid != "" && ps.byGroup[gid] > 0 {
				matched += ps.byGroup[gid]
				if ps.hasFactors {
					hasFactors = true
				}
				continue
			}
			if gname != "" && ps.byGroup[gname] > 0 {
				matched += ps.byGroup[gname]
				if ps.hasFactors {
					hasFactors = true
				}
			}
		}
		if matched == 0 {
			return
		}
		label := firstNonEmpty(gname, gid)
		if matched < indN {
			warnings = append(warnings, fmt.Sprintf("group %s (%s) declares independent_n=%d, but plate assigns only %d well(s)", gid, label, indN, matched))
		} else if hasRep && rep >= 1 {
			expected := indN * rep
			ok := matched == expected || (hasFactors && matched > expected && matched%expected == 0)
			if !ok {
				warnings = append(warnings, fmt.Sprintf("group %s (%s) declares independent_n=%d x technical_repeats=%d (%d wells), but plate assigns %d well(s)", gid, label, indN, rep, expected, matched))
			}
		} else if !hasRep && matched%indN != 0 {
			warnings = append(warnings, fmt.Sprintf("group %s (%s) declares independent_n=%d, which does not evenly divide plate assigned %d well(s)", gid, label, indN, matched))
		}
	}

	for _, m := range flat {
		switch m.Kind {
		case "group":
			checkGroup(m.Data)
		case "group_table":
			for _, row := range mapSlice(m.Data["rows"]) {
				checkGroup(row)
			}
		}
	}
	return warnings
}

func formatBudgetSummaryLine(b BudgetSummary, gap string) string {
	parts := []string{
		fmt.Sprintf("%d 步", b.ExecutableSteps),
	}
	if b.Groups > 0 {
		parts = append(parts, fmt.Sprintf("%d 组", b.Groups))
	}
	if b.Wells > 0 {
		parts = append(parts, fmt.Sprintf("%d 孔", b.Wells))
	}
	if gap != "" {
		return strings.Join(parts, " · ") + " · " + gap
	}
	return strings.Join(parts, " · ")
}

// NormalizeDateYMD extracts a YYYY-MM-DD date prefix if valid.
func NormalizeDateYMD(raw string) string {
	s := strings.TrimSpace(raw)
	if len(s) < 10 {
		return ""
	}
	cand := s[:10]
	if _, err := time.Parse("2006-01-02", cand); err != nil {
		return ""
	}
	return cand
}

// StampScheduledDate sets experiment@1.scheduled_date in an AsterML source document.
// If dateYMD is invalid or the source cannot be parsed, source is returned unchanged.
func StampScheduledDate(source, dateYMD string) string {
	norm := NormalizeDateYMD(dateYMD)
	if norm == "" {
		return source
	}
	doc, err := Parse(source)
	if err != nil {
		return source
	}
	for i := range doc.Modules {
		if doc.Modules[i].Kind == "experiment" {
			updated, upErr := doc.UpdateModule(doc.Modules[i].ID, map[string]any{
				"scheduled_date": norm,
			})
			if upErr == nil && strings.TrimSpace(updated) != "" {
				return updated
			}
			break
		}
	}
	return source
}
