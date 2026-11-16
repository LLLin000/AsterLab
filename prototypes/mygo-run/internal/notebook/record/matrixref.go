package record

import (
	"fmt"
	"sort"
	"strings"
)

// MatrixRef is one parsed `@matrix(...)` dynamic reference in Notebook prose.
// The arguments name timepoints (`@matrix(6h,12h,24h)`); `@matrix(*)` selects
// every recorded timepoint. References never carry values themselves: they
// resolve against the RecordProjection at render or export time, so the prose
// stays a citation instead of a second copy of the facts.
type MatrixRef struct {
	Raw        string
	Timepoints []string
	All        bool
}

// FindMatrixRefs returns every `@matrix(...)` token in reading order.
func FindMatrixRefs(prose string) []MatrixRef {
	var out []MatrixRef
	rest := prose
	for {
		start := strings.Index(rest, "@matrix(")
		if start < 0 {
			return out
		}
		after := rest[start+len("@matrix("):]
		end := strings.Index(after, ")")
		if end < 0 {
			return out
		}
		raw := rest[start : start+len("@matrix(")+end+1]
		args := strings.TrimSpace(after[:end])
		ref := MatrixRef{Raw: raw}
		if args == "*" {
			ref.All = true
		} else {
			for _, part := range strings.Split(args, ",") {
				part = strings.TrimSpace(part)
				if part != "" {
					ref.Timepoints = append(ref.Timepoints, part)
				}
			}
		}
		out = append(out, ref)
		rest = after[end+1:]
	}
}

// ResolveMatrixRef returns the matrix cells the reference selects, in
// well-major order. Unknown timepoints select nothing; they are reported by
// name instead of being silently dropped.
func ResolveMatrixRef(proj RecordProjection, ref MatrixRef) (cells []MatrixCell, unknown []string) {
	wanted := map[string]bool{}
	if ref.All {
		for _, tp := range proj.Timepoints {
			wanted[tp] = true
		}
	} else {
		known := map[string]bool{}
		for _, tp := range proj.Timepoints {
			known[tp] = true
		}
		for _, tp := range ref.Timepoints {
			if known[tp] {
				wanted[tp] = true
			} else {
				unknown = append(unknown, tp)
			}
		}
	}
	for _, cell := range proj.Matrix {
		if wanted[cell.Timepoint] {
			cells = append(cells, cell)
		}
	}
	return cells, unknown
}

// MatrixCellText is the one-line reading of a cell: recorded values, the
// filled FOV count, or an explicit gap marker. Missing facts read as 缺,
// never as zero or blank.
func MatrixCellText(cell MatrixCell) string {
	parts := []string{}
	seen := map[string]bool{}
	for _, v := range cell.Values {
		key := v.Endpoint + "\x00" + v.Value
		if seen[key] {
			continue
		}
		seen[key] = true
		if strings.TrimSpace(v.Endpoint) != "" {
			parts = append(parts, v.Endpoint+" "+v.Value)
		} else {
			parts = append(parts, v.Value)
		}
	}
	if filled := cell.FilledSlots(); filled > 0 {
		parts = append(parts, fmt.Sprintf("图%d/%d", filled, FOVSlots))
	}
	if len(parts) == 0 {
		return "缺"
	}
	return strings.Join(parts, "；")
}

// ExpandMatrixRefs flattens every `@matrix(...)` token into a deterministic
// Markdown table for archive export. Prose without tokens is untouched.
func ExpandMatrixRefs(prose string, proj RecordProjection) string {
	refs := FindMatrixRefs(prose)
	if len(refs) == 0 {
		return prose
	}
	out := prose
	for _, ref := range refs {
		out = strings.Replace(out, ref.Raw, RenderMatrixTable(proj, ref), 1)
	}
	return out
}

// RenderMatrixTable renders the referenced slice as Well x Timepoint text.
func RenderMatrixTable(proj RecordProjection, ref MatrixRef) string {
	cells, unknown := ResolveMatrixRef(proj, ref)
	tps := []string{}
	if ref.All {
		tps = append(tps, proj.Timepoints...)
	} else {
		tps = append(tps, ref.Timepoints...)
	}
	wells := []string{}
	byWell := map[string]map[string]MatrixCell{}
	for _, cell := range cells {
		if byWell[cell.Well] == nil {
			byWell[cell.Well] = map[string]MatrixCell{}
			wells = append(wells, cell.Well)
		}
		byWell[cell.Well][cell.Timepoint] = cell
	}
	sort.SliceStable(wells, func(i, j int) bool { return wellLess(wells[i], wells[j]) })
	var sb strings.Builder
	sb.WriteString("\n\n| 孔位 |")
	for _, tp := range tps {
		sb.WriteString(" " + tp + " |")
	}
	sb.WriteString("\n| --- |")
	for range tps {
		sb.WriteString(" --- |")
	}
	sb.WriteString("\n")
	for _, well := range wells {
		sb.WriteString("| " + well + " |")
		for _, tp := range tps {
			text := "—"
			if cell, ok := byWell[well][tp]; ok {
				text = MatrixCellText(cell)
			}
			sb.WriteString(" " + text + " |")
		}
		sb.WriteString("\n")
	}
	for _, tp := range unknown {
		sb.WriteString("\n> 未记录时间点 " + tp + "：无实测数据。\n")
	}
	return sb.String()
}
