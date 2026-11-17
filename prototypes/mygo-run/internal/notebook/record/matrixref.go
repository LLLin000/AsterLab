package record

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// MatrixRef is one parsed `@matrix(...)` dynamic reference in Notebook prose.
// The arguments name timepoints (`@matrix(6h,12h,24h)`); `@matrix(*)` selects
// every recorded timepoint. References never carry values themselves: they
// resolve against the Core RecordProjection at render or export time, so the
// prose stays a citation instead of a second copy of the facts.
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

// MatrixTimepoints is the ordered union of the projection's plate timepoints.
// Empty slots (assays without a timepoint axis) never become columns.
func MatrixTimepoints(proj Projection) []string {
	var out []string
	seen := map[string]bool{}
	for _, plate := range proj.Images.Plates {
		for _, tp := range plate.Timepoints {
			if tp == nil || strings.TrimSpace(*tp) == "" {
				continue
			}
			if !seen[*tp] {
				seen[*tp] = true
				out = append(out, *tp)
			}
		}
	}
	return out
}

// MatrixWells is the ordered union of the projection's well coordinates.
func MatrixWells(proj Projection) []string {
	var out []string
	seen := map[string]bool{}
	for _, plate := range proj.Images.Plates {
		for _, well := range plate.Wells {
			coord := strings.ToUpper(strings.TrimSpace(well.Coordinate))
			if coord == "" || seen[coord] {
				continue
			}
			seen[coord] = true
			out = append(out, coord)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return wellLess(out[i], out[j]) })
	return out
}

// MatrixCell is one Well x Timepoint cell adapted from the Core matrix:
// filled and planned FOV slot counts, never a second copy of the slots.
type MatrixCell struct {
	Well      string
	Timepoint string
	Filled    int
	Planned   int
}

// MatrixCells aggregates the Core slots into Well x Timepoint cells in
// well-major order.
func MatrixCells(proj Projection) []MatrixCell {
	type key struct{ well, tp string }
	agg := map[key]*MatrixCell{}
	var order []key
	for _, plate := range proj.Images.Plates {
		for _, well := range plate.Wells {
			coord := strings.ToUpper(strings.TrimSpace(well.Coordinate))
			for _, slot := range well.Slots {
				tp := ""
				if slot.Timepoint != nil {
					tp = strings.TrimSpace(*slot.Timepoint)
				}
				k := key{coord, tp}
				cell, ok := agg[k]
				if !ok {
					cell = &MatrixCell{Well: coord, Timepoint: tp}
					agg[k] = cell
					order = append(order, k)
				}
				cell.Planned++
				if slot.AssetID != nil && strings.TrimSpace(*slot.AssetID) != "" {
					cell.Filled++
				}
			}
		}
	}
	out := make([]MatrixCell, 0, len(order))
	for _, k := range order {
		out = append(out, *agg[k])
	}
	return out
}

// ResolveMatrixRef returns the matrix cells the reference selects, in
// well-major order. Unknown timepoints select nothing; they are reported by
// name instead of being silently dropped.
func ResolveMatrixRef(proj Projection, ref MatrixRef) (cells []MatrixCell, unknown []string) {
	wanted := map[string]bool{}
	if ref.All {
		for _, tp := range MatrixTimepoints(proj) {
			wanted[tp] = true
		}
	} else {
		known := map[string]bool{}
		for _, tp := range MatrixTimepoints(proj) {
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
	for _, cell := range MatrixCells(proj) {
		if wanted[cell.Timepoint] {
			cells = append(cells, cell)
		}
	}
	return cells, unknown
}

// MatrixCellText is the one-line reading of a cell: the filled FOV count, or
// an explicit gap marker. Missing facts read as 缺, never as zero or blank.
func MatrixCellText(cell MatrixCell) string {
	if cell.Planned == 0 {
		return "缺"
	}
	if cell.Filled == 0 {
		return "缺"
	}
	return fmt.Sprintf("图%d/%d", cell.Filled, cell.Planned)
}

// PlacedImage locates one filled matrix slot for preview, export and asset
// opening. The file itself is resolved through the asset roots, never from
// the label.
type PlacedImage struct {
	AssetID   string
	Well      string
	Timepoint string
	FOV       int
}

// SlotImage returns the placed image for one Well x Timepoint x FOV slot, or
// nil when the slot is a gap.
func SlotImage(proj Projection, well, timepoint string, fov int) *PlacedImage {
	well = strings.ToUpper(strings.TrimSpace(well))
	for _, plate := range proj.Images.Plates {
		for _, row := range plate.Wells {
			if strings.ToUpper(strings.TrimSpace(row.Coordinate)) != well {
				continue
			}
			for _, slot := range row.Slots {
				tp := ""
				if slot.Timepoint != nil {
					tp = strings.TrimSpace(*slot.Timepoint)
				}
				if tp != timepoint || int(slot.FOVIndex) != fov {
					continue
				}
				if slot.AssetID == nil || strings.TrimSpace(*slot.AssetID) == "" {
					return nil
				}
				return &PlacedImage{AssetID: *slot.AssetID, Well: well, Timepoint: tp, FOV: fov}
			}
		}
	}
	return nil
}

// AssetByID returns the located asset Core links to an id, or nil.
func AssetByID(proj Projection, assetID string) *RecordAsset {
	for i := range proj.Assets {
		if proj.Assets[i].AssetID == assetID {
			asset := proj.Assets[i]
			return &asset
		}
	}
	return nil
}

// MatrixGroups names every assay group Core reports in the matrix, sorted,
// with the ungrouped bucket ("") last when any well has no recorded group.
// Under blinding Core masks every group label, so the board shows one masked
// row instead of inventing groups.
func MatrixGroups(proj Projection) []string {
	if proj.View == "blind" {
		return []string{""}
	}
	var out []string
	seen := map[string]bool{}
	ungrouped := false
	for _, plate := range proj.Images.Plates {
		for _, well := range plate.Wells {
			if well.GroupLabel == nil || strings.TrimSpace(*well.GroupLabel) == "" {
				ungrouped = true
				continue
			}
			if !seen[*well.GroupLabel] {
				seen[*well.GroupLabel] = true
				out = append(out, *well.GroupLabel)
			}
		}
	}
	sort.Strings(out)
	if ungrouped {
		out = append(out, "")
	}
	return out
}

// GroupDisplayName names a group row. Well identity is never masked: wells
// are coordinates, not group identity.
func GroupDisplayName(proj Projection, group string) string {
	if proj.View == "blind" {
		return "已设盲"
	}
	if group == "" {
		return "未分组"
	}
	return group
}

// RepresentativeGroup picks the cell a group x timepoint slot shows: the
// first well (in matrix order) whose selected FOV slot is filled, falling
// back to the first well of the slot. ok is false when the slot has no well
// at all. The returned image is nil for a gap.
func RepresentativeGroup(proj Projection, group, timepoint string, fov int) (well string, img *PlacedImage, ok bool) {
	blinded := proj.View == "blind"
	for _, plate := range proj.Images.Plates {
		for _, row := range plate.Wells {
			label := ""
			if row.GroupLabel != nil {
				label = *row.GroupLabel
			}
			if !blinded && label != group {
				continue
			}
			coord := strings.ToUpper(strings.TrimSpace(row.Coordinate))
			slot := SlotImage(proj, coord, timepoint, fov)
			if !ok {
				well, img, ok = coord, slot, true
			}
			if slot != nil {
				return coord, slot, true
			}
		}
	}
	return well, img, ok
}

// ExpandMatrixRefs flattens every `@matrix(...)` token into a deterministic
// Markdown table for archive export. Prose without tokens is untouched.
func ExpandMatrixRefs(prose string, proj Projection) string {
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
func RenderMatrixTable(proj Projection, ref MatrixRef) string {
	cells, unknown := ResolveMatrixRef(proj, ref)
	tps := []string{}
	if ref.All {
		tps = append(tps, MatrixTimepoints(proj)...)
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

// wellLess orders well coordinates the way operators read plates: row 0 is
// A, column 0 is 1.
func wellLess(a, b string) bool {
	ra, ca := splitWell(a)
	rb, cb := splitWell(b)
	if ra != rb {
		return ra < rb
	}
	return ca < cb
}

func splitWell(well string) (string, int) {
	well = strings.ToUpper(strings.TrimSpace(well))
	i := 0
	for i < len(well) && (well[i] < '0' || well[i] > '9') {
		i++
	}
	head, tail := well[:i], well[i:]
	num, err := strconv.Atoi(tail)
	if err != nil {
		return head, 0
	}
	return head, num
}
