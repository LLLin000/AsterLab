package astercore

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"asterlab-mygo-prototype/internal/analysis"
	"asterlab-mygo-prototype/internal/asterdoc"
)

// AssaySummary holds deterministic derived statistics extracted from a document's
// plate layout + data_table observations without ever mutating raw observations.
type AssaySummary struct {
	Kind       string              `json:"kind"` // "cck8" | "qpcr" | "descriptive"
	Title      string              `json:"title"`
	Control    string              `json:"control,omitempty"`
	CCK8       *analysis.CCK8Result `json:"cck8,omitempty"`
	QPCR       *analysis.QPCRResult `json:"qpcr,omitempty"`
	Groups     []DescriptiveGroup  `json:"groups,omitempty"`
	ExcludedN  int                 `json:"excluded_wells,omitempty"`
}

// DescriptiveGroup is a fallback deterministic group summary (N, Mean, SD).
type DescriptiveGroup struct {
	Group string  `json:"group"`
	N     int     `json:"n"`
	Mean  float64 `json:"mean"`
	SD    float64 `json:"sd"`
}

// AnalyzeDocument extracts numeric observations from `data_table@1` modules, maps
// them through `plate@1` group/control/excluded_wells metadata, and invokes
// `analysis.AnalyzeCCK8` or `analysis.AnalyzeQPCR` where applicable.
func AnalyzeDocument(doc *asterdoc.Document, docID string) []AssaySummary {
	if doc == nil {
		return nil
	}
	if strings.TrimSpace(docID) == "" {
		docID = "vault-doc"
	}
	prov := analysis.Provenance{BatchID: docID, EventID: "live-projection"}

	groupLabels := asterdoc.PlateGroupLabelMap(doc)
	controlGroup := ""
	for _, g := range doc.GroupList() {
		if g.Role == "control" && controlGroup == "" {
			controlGroup = firstNonEmpty(g.Name, g.ID)
		}
	}

	var firstPlate *asterdoc.Plate
	var dataTables []map[string]any
	for _, m := range doc.Modules {
		if m.Kind == "plate" && m.Plate != nil && firstPlate == nil {
			firstPlate = m.Plate
		}
		if m.Kind == "data_table" {
			dataTables = append(dataTables, m.Data)
		}
		if m.Kind == "step" {
			if blks, ok := m.Data["blocks"].([]any); ok {
				for _, raw := range blks {
					bm, _ := raw.(map[string]any)
					if bm == nil {
						continue
					}
					bk, _ := bm["kind"].(string)
					if bk == "plate" && firstPlate == nil {
						if p, err := parsePlateBlock(bm); err == nil {
							firstPlate = p
						}
					} else if bk == "data_table" {
						dataTables = append(dataTables, bm)
					}
				}
			}
		}
	}

	excluded := map[string]bool{}
	wellToGroup := map[string]string{}
	if firstPlate != nil {
		for _, w := range firstPlate.ExcludedWells {
			excluded[strings.ToUpper(strings.TrimSpace(w))] = true
		}
		for w, gid := range firstPlate.WellGroups() {
			lbl := firstNonEmpty(groupLabels[gid], gid)
			wellToGroup[strings.ToUpper(w)] = lbl
			if controlGroup == "" && strings.EqualFold(firstPlate.Roles[w], "negative_control") {
				controlGroup = lbl
			}
		}
	}

	var summaries []AssaySummary
	for _, dt := range dataTables {
		title := firstNonEmpty(stringVal(dt["title"]), "数据表统计")
		rows, _ := dt["rows"].([]any)
		if len(rows) == 0 {
			continue
		}
		// Extract cells from row entries
		var cells []analysis.Cell
		excludedCount := 0
		genes := map[string]bool{}
		samples := map[string]bool{}
		for _, rRaw := range rows {
			rm, _ := rRaw.(map[string]any)
			if rm == nil {
				continue
			}
			well := strings.ToUpper(strings.TrimSpace(stringVal(rm["well"])))
			if well != "" && excluded[well] {
				excludedCount++
				continue
			}
			grp := strings.TrimSpace(stringVal(rm["group"]))
			if grp == "" && well != "" {
				grp = wellToGroup[well]
			}
			sample := firstNonEmpty(stringVal(rm["sample"]), grp)
			if grp == "" {
				grp = sample
			}
			if grp == "" {
				continue
			}
			gene := strings.TrimSpace(stringVal(rm["gene"]))
			if gene == "" {
				gene = strings.TrimSpace(stringVal(rm["target"]))
			}
			val, hasVal := firstNumericReading(rm)
			if !hasVal {
				cells = append(cells, analysis.Cell{
					Group: grp, SampleID: sample, Gene: gene, Status: analysis.Missing,
				})
				continue
			}
			vCopy := val
			cells = append(cells, analysis.Cell{
				Group: grp, SampleID: sample, Gene: gene, Value: &vCopy, Status: analysis.Raw,
			})
			if gene != "" {
				genes[gene] = true
			}
			if sample != "" {
				samples[sample] = true
			}
		}
		if len(cells) == 0 {
			continue
		}

		// Try RT-qPCR ΔΔCq if >=2 genes (target + reference) and calibrator exists
		if len(genes) >= 2 && len(samples) >= 2 {
			geneList := sortedKeys(genes)
			refGene := pickReferenceGene(geneList)
			calSample := firstNonEmpty(controlGroup, sortedKeys(samples)[0])
			var targetGene string
			for _, g := range geneList {
				if g != refGene {
					targetGene = g
					break
				}
			}
			if targetGene != "" && refGene != "" {
				if qpcrRes, err := analysis.AnalyzeQPCR(cells, targetGene, refGene, calSample, prov); err == nil {
					summaries = append(summaries, AssaySummary{
						Kind:      "qpcr",
						Title:     fmt.Sprintf("%s · ΔΔCq (%s / %s)", title, targetGene, refGene),
						Control:   calSample,
						QPCR:      &qpcrRes,
						ExcludedN: excludedCount,
					})
					continue
				}
			}
		}

		// Try CCK-8 relative viability if a control group is present
		if controlGroup != "" {
			if cckRes, err := analysis.AnalyzeCCK8(cells, controlGroup, prov); err == nil {
				summaries = append(summaries, AssaySummary{
					Kind:      "cck8",
					Title:     fmt.Sprintf("%s · 相对活力分析", title),
					Control:   controlGroup,
					CCK8:      &cckRes,
					ExcludedN: excludedCount,
				})
				continue
			}
		}

		// Descriptive fallback (group N, Mean, SD)
		byGroup := map[string][]float64{}
		for _, c := range cells {
			if c.Status == analysis.Raw && c.Value != nil {
				byGroup[c.Group] = append(byGroup[c.Group], *c.Value)
			}
		}
		if len(byGroup) > 0 {
			var desc []DescriptiveGroup
			for _, gName := range sortedKeysMap(byGroup) {
				vs := byGroup[gName]
				desc = append(desc, DescriptiveGroup{
					Group: gName,
					N:     len(vs),
					Mean:  meanSlice(vs),
					SD:    sdSlice(vs),
				})
			}
			summaries = append(summaries, AssaySummary{
				Kind:      "descriptive",
				Title:     title,
				Groups:    desc,
				ExcludedN: excludedCount,
			})
		}
	}
	return summaries
}

func parsePlateBlock(bm map[string]any) (*asterdoc.Plate, error) {
	p := &asterdoc.Plate{
		ID:     stringVal(bm["id"]),
		Format: int(numericFromAny(bm["format"])),
	}
	if p.Format == 0 {
		p.Format = 96
	}
	if ex, ok := bm["excluded_wells"].([]any); ok {
		for _, w := range ex {
			if ws := strings.TrimSpace(stringVal(w)); ws != "" {
				p.ExcludedWells = append(p.ExcludedWells, ws)
			}
		}
	}
	if grps, ok := bm["groups"].(map[string]any); ok {
		p.Groups = map[string][]string{}
		for gid, raw := range grps {
			if arr, ok := raw.([]any); ok {
				for _, w := range arr {
					if ws := strings.TrimSpace(stringVal(w)); ws != "" {
						p.Groups[gid] = append(p.Groups[gid], ws)
					}
				}
			}
		}
	}
	if roles, ok := bm["roles"].(map[string]any); ok {
		p.Roles = map[string]string{}
		for k, v := range roles {
			p.Roles[k] = stringVal(v)
		}
	}
	return p, nil
}

func firstNumericReading(rm map[string]any) (float64, bool) {
	for _, key := range []string{"reading", "value", "od", "od450", "cq", "ct", "viability"} {
		if v, ok := rm[key]; ok {
			if f, fok := parseFloatAny(v); fok {
				return f, true
			}
		}
	}
	for k, v := range rm {
		switch k {
		case "id", "well", "group", "sample", "gene", "target", "note", "unit":
			continue
		}
		if f, fok := parseFloatAny(v); fok {
			return f, true
		}
	}
	return 0, false
}

func parseFloatAny(v any) (float64, bool) {
	switch n := v.(type) {
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case float64:
		if !math.IsNaN(n) && !math.IsInf(n, 0) {
			return n, true
		}
	case string:
		if f, err := strconv.ParseFloat(strings.TrimSpace(n), 64); err == nil && !math.IsNaN(f) && !math.IsInf(f, 0) {
			return f, true
		}
	}
	return 0, false
}

func pickReferenceGene(genes []string) string {
	for _, g := range genes {
		low := strings.ToLower(g)
		if low == "gapdh" || low == "actb" || low == "18s" || strings.Contains(low, "ref") {
			return g
		}
	}
	if len(genes) > 0 {
		return genes[len(genes)-1]
	}
	return ""
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func sortedKeysMap(m map[string][]float64) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func meanSlice(v []float64) float64 {
	if len(v) == 0 {
		return 0
	}
	s := 0.0
	for _, x := range v {
		s += x
	}
	return s / float64(len(v))
}

func sdSlice(v []float64) float64 {
	if len(v) < 2 {
		return 0
	}
	m := meanSlice(v)
	ss := 0.0
	for _, x := range v {
		d := x - m
		ss += d * d
	}
	return math.Sqrt(ss / float64(len(v)-1))
}

func stringVal(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprint(v)
}
