package runworkspace

import (
	"fmt"
	"sort"
	"strings"

	"asterlab-mygo-prototype/internal/asterdoc"
	"asterlab-mygo-prototype/internal/theme"

	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/onevra/source/components"
)

// CoverageCell is one (well, timepoint) capture group rendered from recorded
// links. The grouping mirrors Core's asset_coverage rule — groups keyed by
// (well_id, timepoint), links without a well stay out of every group — and the
// expected slots come from the frozen plan, never from invented wells.
type CoverageCell struct {
	WellID    string
	Timepoint string
	// FOVs holds the recorded indexes, deduplicated ascending. A zero index
	// means "not recorded" per the asset contract and never enters the set.
	FOVs []uint32
	// Gaps holds the planned slots 1..Slots with no recorded image.
	Gaps []uint32
	// Slots is the planned FOV count per well; Filled counts recorded slots.
	Slots  int
	Filled int
}

// CoverageByWell groups assets exactly like Core's asset_coverage, then diffs
// each group against the planned slot count. Wells lists the planned wells in
// fill order; recorded wells outside the plan are appended after them.
// Only links with a well and a nonzero FOV form groups.
func CoverageByWell(assets []LinkedAsset, wells []string, timepoint string, fovPerWell int) []CoverageCell {
	type key struct{ well, timepoint string }
	seen := map[key]map[uint32]bool{}
	order := []key{}
	for _, asset := range assets {
		well := strings.TrimSpace(asset.WellID)
		tp := strings.TrimSpace(asset.Timepoint)
		if well == "" || asset.FOVIndex == 0 {
			continue
		}
		k := key{well, tp}
		set, ok := seen[k]
		if !ok {
			set = map[uint32]bool{}
			seen[k] = set
			order = append(order, k)
		}
		set[asset.FOVIndex] = true
	}
	// Deterministic well order: the plan first, then recorded extras.
	wanted := map[string]bool{}
	orderedWells := make([]string, 0, len(wells))
	for _, well := range wells {
		well = strings.TrimSpace(well)
		if well == "" || wanted[well] {
			continue
		}
		wanted[well] = true
		orderedWells = append(orderedWells, well)
	}
	extra := []string{}
	for _, k := range order {
		if k.timepoint != timepoint {
			continue
		}
		if !wanted[k.well] {
			wanted[k.well] = true
			extra = append(extra, k.well)
		}
	}
	sort.Strings(extra)
	orderedWells = append(orderedWells, extra...)

	times := map[string]bool{timepoint: true}
	for _, k := range order {
		times[k.timepoint] = true
	}
	timeList := make([]string, 0, len(times))
	for tp := range times {
		timeList = append(timeList, tp)
	}
	sort.Strings(timeList)

	cells := make([]CoverageCell, 0, len(orderedWells)*len(timeList))
	for _, well := range orderedWells {
		for _, tp := range timeList {
			set := seen[key{well, tp}]
			fovs := make([]uint32, 0, len(set))
			for fov := range set {
				fovs = append(fovs, fov)
			}
			sort.Slice(fovs, func(i, j int) bool { return fovs[i] < fovs[j] })
			slots := fovPerWell
			if slots <= 0 {
				// No plan: the widest recorded index bounds the row, like the
				// gallery matrix. Gaps then only flag holes, never a total.
				for _, fov := range fovs {
					if int(fov) > slots {
						slots = int(fov)
					}
				}
			}
			gaps := []uint32{}
			inSet := map[uint32]bool{}
			for _, fov := range fovs {
				inSet[fov] = true
			}
			for i := 1; i <= slots; i++ {
				if !inSet[uint32(i)] {
					gaps = append(gaps, uint32(i))
				}
			}
			cells = append(cells, CoverageCell{
				WellID: well, Timepoint: tp,
				FOVs: fovs, Gaps: gaps, Slots: slots, Filled: len(fovs),
			})
		}
	}
	return cells
}

// BatchDrop is one planned slot for one dropped file: the coordinate triple
// the host persists through asset.link.
type BatchDrop struct {
	WellID    string
	Timepoint string
	FOVIndex  uint32
}

// PlanBatchDrops assigns dropped files to capture slots in order: the first
// missing FOV of the first unfilled well, then on. A full well is skipped, so
// after six images the plan moves to the next unfilled well. It returns fewer
// drops than files when the plate has no empty slots left; it never invents
// wells or reuses a recorded slot.
func PlanBatchDrops(wells []string, timepoint string, fovPerWell int, coverage []CoverageCell, count int) []BatchDrop {
	if count <= 0 || fovPerWell <= 0 {
		return nil
	}
	byWell := map[string]*CoverageCell{}
	for i := range coverage {
		cell := &coverage[i]
		if cell.Timepoint != timepoint {
			continue
		}
		if _, ok := byWell[cell.WellID]; !ok {
			byWell[cell.WellID] = cell
		}
	}
	ordered := make([]string, 0, len(wells))
	for _, well := range wells {
		if well = strings.TrimSpace(well); well != "" {
			ordered = append(ordered, well)
		}
	}
	drops := []BatchDrop{}
	for _, well := range ordered {
		cell := byWell[well]
		var gaps []uint32
		if cell == nil {
			for i := 1; i <= fovPerWell; i++ {
				gaps = append(gaps, uint32(i))
			}
		} else {
			gaps = cell.Gaps
		}
		for _, gap := range gaps {
			if len(drops) >= count {
				return drops
			}
			drops = append(drops, BatchDrop{WellID: well, Timepoint: timepoint, FOVIndex: gap})
		}
	}
	return drops
}

// FOVPerWellForStep reads the planned FOV count from the frozen AsterDoc
// source: inline step blocks[], block_refs, or uses pointing to attachment@1
// (scope: container_grid, slots_per_cell) or legacy measurement_profile. Zero
// means the plan declares none and the surface falls back to recorded data.
func FOVPerWellForStep(source, stepID string) int {
	stepID = strings.TrimSpace(stepID)
	if stepID == "" || strings.TrimSpace(source) == "" {
		return 0
	}
	doc, err := asterdoc.Parse(source)
	if err != nil {
		return 0
	}
	byID := map[string]asterdoc.Module{}
	for _, m := range doc.Modules {
		byID[m.ID] = m
	}
	step, ok := byID[stepID]
	if !ok {
		return 0
	}
	if blocks, ok := step.Data["blocks"].([]any); ok {
		for _, raw := range blocks {
			if m, _ := raw.(map[string]any); m != nil {
				if n := fovFromModuleMap(m); n > 0 {
					return n
				}
			}
		}
	}
	if refs, ok := step.Data["block_refs"].([]any); ok {
		for _, raw := range refs {
			if ref, _ := raw.(string); ref != "" {
				if target, exists := byID[ref]; exists {
					if n := fovFromModuleMap(target.Data); n > 0 {
						return n
					}
				}
			}
		}
	}
	uses, _ := step.Data["uses"].([]any)
	for _, raw := range uses {
		item, _ := raw.(map[string]any)
		ref, _ := item["ref"].(string)
		target, exists := byID[ref]
		if !exists {
			continue
		}
		if n := fovFromModuleMap(target.Data); n > 0 {
			return n
		}
	}
	return 0
}

// FOVPerWellForDoc scans the entire frozen or draft AsterDoc source for the
// maximum planned container_grid slots_per_cell (across top-level attachments
// and step inline blocks).
func FOVPerWellForDoc(source string) int {
	if strings.TrimSpace(source) == "" {
		return 0
	}
	doc, err := asterdoc.Parse(source)
	if err != nil {
		return 0
	}
	best := 0
	for _, m := range doc.Modules {
		if n := fovFromModuleMap(m.Data); n > best {
			best = n
		}
		if blocks, ok := m.Data["blocks"].([]any); ok {
			for _, raw := range blocks {
				if bm, _ := raw.(map[string]any); bm != nil {
					if n := fovFromModuleMap(bm); n > best {
						best = n
					}
				}
			}
		}
	}
	return best
}

func fovFromModuleMap(data map[string]any) int {
	if data == nil {
		return 0
	}
	kind, _ := data["kind"].(string)
	switch kind {
	case "attachment":
		scope, _ := data["scope"].(string)
		if scope == "container_grid" {
			return toPositiveInt(data["slots_per_cell"])
		}
	case "measurement_profile":
		if schema, _ := data["readout_schema"].(map[string]any); schema != nil {
			return toPositiveInt(schema["per_well_fov_count"])
		}
	}
	return 0
}

func toPositiveInt(v any) int {
	var n int
	switch value := v.(type) {
	case int:
		n = value
	case int64:
		n = int(value)
	case float64:
		n = int(value)
	case string:
		var parsed int
		if _, err := fmt.Sscanf(strings.TrimSpace(value), "%d", &parsed); err == nil {
			n = parsed
		}
	}
	if n <= 0 {
		return 0
	}
	return n
}

// renderCaptureStrip draws the per-well FOV progress from coverage plus the
// batch-link entry. Wells with a full row read n/n; gaps name the missing
// slots. The strip only appears when the host supplies the planned wells.
func renderCaptureStrip(c *ui.Context, s *State, task *RunTaskViewModel, wells []string, timepoint string, fovPerWell int, t *ui.Theme, tokens theme.Tokens) {
	if len(wells) == 0 || fovPerWell <= 0 {
		return
	}
	bench := s.bench()
	assets := []LinkedAsset{}
	if s.Callbacks.TaskAssets != nil {
		assets = s.Callbacks.TaskAssets(task.TaskKey)
	}
	cov := CoverageByWell(assets, wells, timepoint, fovPerWell)

	ui.Column(c).FillWidth().Gap(t.Space(1)).Children(func() {
		ui.Row(c).FillWidth().Wrap().AlignItems(ui.Center).Gap(t.Space(1.5)).Children(func() {
			ui.Text(c, "采图进度").FontSize(float32(tokens.TextSection)).FontWeight(600).TextColor(t.Text)
			ui.Text(c, fmt.Sprintf("%s · 每孔 %d 视野", timepointLabel(timepoint), fovPerWell)).
				FontSize(t.Rem(.7)).TextColor(t.TextMuted)
			ui.Spacer(c)
			if s.Callbacks.RequestBatchLink != nil {
				if batch := components.Button(c, "批量关联", components.ButtonSecondary); batch.Clicked() {
					s.Callbacks.RequestBatchLink(task.TaskKey)
				}
			}
		})
		if bench.BatchErr != "" {
			ui.Text(c, "批量关联失败："+bench.BatchErr).FontSize(t.Rem(.72)).TextColor(tokens.DangerText)
		}
		if bench.BatchDone != "" {
			ui.Text(c, bench.BatchDone).FontSize(t.Rem(.74)).TextColor(tokens.SuccessText)
		}
		match := func(cell CoverageCell) bool {
			return timepoint == "" || cell.Timepoint == timepoint
		}
		wellLabel := func(cell CoverageCell) string {
			if timepoint != "" || strings.TrimSpace(cell.Timepoint) == "" {
				return cell.WellID
			}
			return cell.WellID + " · " + cell.Timepoint
		}
		full, total := 0, 0
		for _, cell := range cov {
			if !match(cell) {
				continue
			}
			total++
			if len(cell.Gaps) == 0 && cell.Filled > 0 {
				full++
			}
		}
		ui.Text(c, fmt.Sprintf("%d/%d 孔已传满", full, total)).FontSize(t.Rem(.74)).TextColor(t.TextMuted)
		ui.Row(c).FillWidth().Wrap().Gap(t.Space(1)).Children(func() {
			for _, cell := range cov {
				if !match(cell) {
					continue
				}
				cell := cell
				label := fmt.Sprintf("%s %d/%d", wellLabel(cell), cell.Filled, cell.Slots)
				bg, fg := tokens.Elevated, t.TextMuted
				if len(cell.Gaps) == 0 && cell.Filled > 0 {
					bg, fg = tokens.SuccessBg, tokens.SuccessText
				} else if cell.Filled > 0 {
					bg, fg = tokens.WarningBg, tokens.WarningText
				}
				ui.Row(c).Padding(3, 8).Radius(5).Background(bg).Border(1, tokens.Border).
					Children(func() {
						ui.Text(c, label).FontSize(t.Rem(.7)).Bold().TextColor(fg).FontFeatures("tnum")
					})
			}
		})
		// Gap detail stays one glance away: only wells that still miss slots
		// name them, so the strip never invents a missing file.
		for _, cell := range cov {
			if !match(cell) || len(cell.Gaps) == 0 {
				continue
			}
			ui.Text(c, fmt.Sprintf("%s 缺 FOV %v", wellLabel(cell), cell.Gaps)).
				FontSize(t.Rem(.68)).TextColor(tokens.WarningText)
		}
	})
}

func timepointLabel(timepoint string) string {
	if strings.TrimSpace(timepoint) == "" {
		return "全部时间点"
	}
	return timepoint
}
