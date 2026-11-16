// Package record is the single shared read model for the record surfaces:
// Notebook's automatic board, the Run history audit rendering, and the
// protocol archive all project from the same RecordProjection, so the
// renderings can never disagree. It derives only from the stored RunSnapshot
// plus already-linked assets; it performs no writes and invents no
// coordinates. The package stays a leaf (stdlib plus modules/runs only) so
// every surface can import it without a cycle.
package record

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"asterlab-mygo-prototype/modules/runs"
)

// FOVSlots is the fixed number of field-of-view slots per Well x Timepoint
// cell. Micro-plate imaging captures up to six views per well; a slot with no
// attributable image stays empty and renders as a gap, never a placeholder.
const FOVSlots = 6

// Param is one planned or actually-recorded parameter, already human-readable.
type Param struct {
	Key   string
	Label string
	Value string
	Unit  string
}

// Measure is one recorded readout value with its source endpoint.
type Measure struct {
	SampleID string
	Endpoint string
	Value    string
	Unit     string
	Source   string
}

// TaskSummary is the planned-vs-actual row for one frozen task. Planned comes
// from the frozen task's ParametersJSON; Actual, Measurements and Deviations
// come only from stored events and snapshot measurements. Anything absent is
// absent here too.
type TaskSummary struct {
	TaskKey      string
	Title        string
	Timepoint    string
	Status       string
	Planned      []Param
	Actual       []Param
	Measurements []Measure
	Deviations   []string
	Skipped      bool
}

// MatrixImage is one attributable image asset placed in a FOV slot.
type MatrixImage struct {
	Filled  bool
	AssetID string
	// RootID and RelativePath locate the linked file for opening; Label is
	// the display name.
	RootID       string
	RelativePath string
	Label        string
	Description  string
}

// MatrixCell is one representative Well x Timepoint cell. Images holds the
// FOV 1..6 slots in order; an unfilled slot is a gap.
type MatrixCell struct {
	Well      string
	Samples   []string
	Timepoint string
	Values    []Measure
	Images    [FOVSlots]MatrixImage
	Overflow  int
	TaskKeys  []string
}

// Representative returns the image for the given 1-based FOV slot, or nil when
// that slot is a gap.
func (cell MatrixCell) Representative(fov int) *MatrixImage {
	if fov < 1 || fov > FOVSlots {
		return nil
	}
	img := cell.Images[fov-1]
	if !img.Filled {
		return nil
	}
	return &img
}

// FilledSlots counts the FOV slots that hold an image.
func (cell MatrixCell) FilledSlots() int {
	n := 0
	for _, img := range cell.Images {
		if img.Filled {
			n++
		}
	}
	return n
}

// RecordProjection is the shared read model for Notebook and Run history.
// Both surfaces render from the same value built by Project. The prose
// timeline stays owned by notebook.Build; the projection owns the summary and
// matrix derivation exactly once, so neither surface re-derives it.
type RecordProjection struct {
	RunID          string
	Title          string
	Status         string
	DesignID       string
	DesignRevision uint64
	Summary        []TaskSummary
	Timepoints     []string
	Wells          []string
	Matrix         []MatrixCell
	// Gaps counts empty FOV slots across the matrix.
	Gaps int
	// UnassignedImages counts linked images that name no resolvable
	// (task, sample): they are listed, never silently dropped or guessed
	// into a slot.
	UnassignedImages int
	UnassignedLabels []string
	// UnassignedMeasurements holds stored measurements no task claims: rows
	// with an unknown TaskKey, or keyless legacy rows whose ProtocolStepPath
	// matches no task or more than one. Listed explicitly, never dropped.
	UnassignedMeasurements []Measure
}

// Project builds the shared projection from one stored snapshot and its
// already-linked assets. It is deterministic for one input and performs no
// writes. Image attribution is deliberately strict: an image fills a FOV slot
// only when its link names a known task and a sample measured in that well.
// Anything else is reported as unassigned, never placed by filename or order
// guessing beyond a stable sort for slot assignment.
func Project(snap runs.RunSnapshot, assets []runs.Asset) RecordProjection {
	proj := RecordProjection{
		RunID:          snap.RunID,
		Title:          snap.Title,
		Status:         snap.Status,
		DesignID:       snap.DesignID,
		DesignRevision: snap.DesignRevision,
	}

	// Task summaries in frozen task order. Measurements come from one source
	// only — snap.Measurements — because the bridge copies every event's
	// measurements[] into the snapshot as well; reading DataJSON too would
	// list every stored fact twice. DataJSON still carries parameters,
	// deviations and skips.
	summaries := make([]TaskSummary, 0, len(snap.Tasks))
	actualByTask := collectActuals(snap.Events)
	byKey := make(map[string]int, len(snap.Tasks))
	byPath := make(map[string][]int)
	for i, task := range snap.Tasks {
		sum := TaskSummary{
			TaskKey:   task.Key,
			Title:     task.Title,
			Timepoint: task.Timepoint,
			Status:    task.Status,
			Planned:   parseParameters(task.ParametersJSON),
		}
		if actual, ok := actualByTask[task.Key]; ok {
			sum.Actual = actual.params
			sum.Deviations = actual.deviations
			sum.Skipped = actual.skipped
		}
		summaries = append(summaries, sum)
		byKey[task.Key] = i
		if path := taskEffectivePath(task); path != "" {
			byPath[path] = append(byPath[path], i)
		}
	}
	// Measurement routing is exact, never guessed: a TaskKey names its task;
	// a keyless legacy row resolves only through an unambiguous
	// ProtocolStepPath match. Anything else lands in the explicit unassigned
	// bucket instead of being dropped or misattributed.
	toMeasure := func(m runs.Measurement) Measure {
		return Measure{
			SampleID: m.SampleID, Endpoint: m.Endpoint,
			Value: formatNumber(m.Value), Unit: m.Unit, Source: "measurement",
		}
	}
	for _, m := range snap.Measurements {
		measure := toMeasure(m)
		if m.TaskKey != "" {
			if idx, ok := byKey[m.TaskKey]; ok {
				summaries[idx].Measurements = append(summaries[idx].Measurements, measure)
				continue
			}
			proj.UnassignedMeasurements = append(proj.UnassignedMeasurements, measure)
			continue
		}
		path := strings.TrimSpace(m.ProtocolStepPath)
		if path != "" {
			if idxs := byPath[path]; len(idxs) == 1 {
				summaries[idxs[0]].Measurements = append(summaries[idxs[0]].Measurements, measure)
				continue
			}
		}
		proj.UnassignedMeasurements = append(proj.UnassignedMeasurements, measure)
	}
	proj.Summary = summaries

	// Wells from result cells, row-major.
	wellSamples := map[string]map[string]bool{}
	wellOrder := []string{}
	cellEvents := map[string]map[string]bool{} // well -> timepoint -> true
	cellValues := map[string][]Measure{}       // well\x00timepoint -> values
	cellTasks := map[string]map[string]bool{}
	for _, ev := range snap.Events {
		if strings.TrimSpace(ev.Timepoint) == "" && len(ev.ResultCells) == 0 {
			continue
		}
		for _, rc := range ev.ResultCells {
			well := wellLabel(rc.Row, rc.Column)
			if wellSamples[well] == nil {
				wellSamples[well] = map[string]bool{}
				wellOrder = append(wellOrder, well)
			}
			if strings.TrimSpace(rc.SampleID) != "" {
				wellSamples[well][rc.SampleID] = true
			}
			tp := ev.Timepoint
			if cellEvents[well] == nil {
				cellEvents[well] = map[string]bool{}
			}
			cellEvents[well][tp] = true
			if strings.TrimSpace(rc.Field) != "" || rc.Value != nil {
				key := well + "\x00" + tp
				cellValues[key] = append(cellValues[key], Measure{
					SampleID: rc.SampleID, Endpoint: rc.Field,
					Value: formatValuePtr(rc.Value), Unit: "", Source: "result_cell",
				})
			}
			if strings.TrimSpace(ev.TaskKey) != "" {
				if cellTasks[well+"\x00"+tp] == nil {
					cellTasks[well+"\x00"+tp] = map[string]bool{}
				}
				cellTasks[well+"\x00"+tp][ev.TaskKey] = true
			}
		}
	}
	sort.SliceStable(wellOrder, func(i, j int) bool { return wellLess(wellOrder[i], wellOrder[j]) })
	proj.Wells = wellOrder

	// Timepoints: task timepoints in task order, then event-only ones first-seen.
	seenTP := map[string]bool{}
	for _, task := range snap.Tasks {
		if strings.TrimSpace(task.Timepoint) == "" || seenTP[task.Timepoint] {
			continue
		}
		seenTP[task.Timepoint] = true
		proj.Timepoints = append(proj.Timepoints, task.Timepoint)
	}
	for _, ev := range snap.Events {
		if strings.TrimSpace(ev.Timepoint) == "" || seenTP[ev.Timepoint] {
			continue
		}
		seenTP[ev.Timepoint] = true
		proj.Timepoints = append(proj.Timepoints, ev.Timepoint)
	}

	// Cells exist where a result cell was stored; coordinated images may add
	// their own cells (a well photographed but not otherwise measured still
	// shows, with its image and no values).
	for _, well := range proj.Wells {
		tps := []string{}
		for _, tp := range proj.Timepoints {
			if cellEvents[well][tp] {
				tps = append(tps, tp)
			}
		}
		for _, tp := range tps {
			key := well + "\x00" + tp
			samples := []string{}
			for s := range wellSamples[well] {
				samples = append(samples, s)
			}
			sort.Strings(samples)
			tasks := []string{}
			for k := range cellTasks[key] {
				tasks = append(tasks, k)
			}
			sort.Strings(tasks)
			proj.Matrix = append(proj.Matrix, MatrixCell{
				Well: well, Samples: samples, Timepoint: tp,
				Values:   cellValues[key],
				TaskKeys: tasks,
			})
		}
	}
	ensureImageCells(&proj, assets, seenTP)

	assignImages(&proj, assets)
	return proj
}

// taskActuals accumulates what stored events recorded for one task, minus
// measurements: those come from snap.Measurements alone (single source), so
// DataJSON contributes only parameters, deviations and skips here.
type taskActuals struct {
	params     []Param
	paramIndex map[string]int
	deviations []string
	skipped    bool
}

func collectActuals(events []runs.RunEvent) map[string]*taskActuals {
	out := map[string]*taskActuals{}
	acc := func(key string) *taskActuals {
		if out[key] == nil {
			out[key] = &taskActuals{paramIndex: map[string]int{}}
		}
		return out[key]
	}
	for _, ev := range events {
		if strings.TrimSpace(ev.TaskKey) == "" {
			continue
		}
		a := acc(ev.TaskKey)
		var data struct {
			ActualParameters []struct {
				Key   string `json:"key"`
				Value any    `json:"value"`
				Unit  string `json:"unit"`
			} `json:"actual_parameters"`
			Deviation     *string `json:"deviation"`
			SkippedByRule any     `json:"skipped_by_rule"`
		}
		if strings.TrimSpace(ev.DataJSON) == "" {
			continue
		}
		if json.Unmarshal([]byte(ev.DataJSON), &data) != nil {
			continue
		}
		for _, p := range data.ActualParameters {
			value := ""
			if p.Value != nil {
				value = fmt.Sprint(p.Value)
			}
			param := Param{Key: p.Key, Label: p.Key, Value: value, Unit: p.Unit}
			if idx, ok := a.paramIndex[p.Key]; ok {
				a.params[idx] = param
			} else {
				a.paramIndex[p.Key] = len(a.params)
				a.params = append(a.params, param)
			}
		}
		if data.Deviation != nil && strings.TrimSpace(*data.Deviation) != "" {
			a.deviations = append(a.deviations, *data.Deviation)
		}
		switch v := data.SkippedByRule.(type) {
		case bool:
			if v {
				a.skipped = true
			}
		case string:
			if strings.TrimSpace(v) != "" && !strings.EqualFold(v, "false") {
				a.skipped = true
			}
		}
	}
	return out
}

// taskEffectivePath is the stable frozen-step identity a legacy measurement
// row can name: the task Path, falling back to the path carried by legacy
// frozen task keys, exactly as the Run surface resolves them.
func taskEffectivePath(task runs.RunTask) string {
	if path := strings.TrimSpace(task.Path); path != "" {
		return path
	}
	var parsed struct {
		Path string `json:"path"`
	}
	if json.Unmarshal([]byte(task.Key), &parsed) != nil {
		return ""
	}
	return strings.TrimSpace(parsed.Path)
}

// ensureImageCells adds matrix cells for coordinated images whose (well,
// timepoint) has no measured cell, and extends the timepoint order with
// image-only timepoints first-seen. Wells sort with the measured wells.
func ensureImageCells(proj *RecordProjection, assets []runs.Asset, seenTP map[string]bool) {
	have := map[string]bool{}
	for _, cell := range proj.Matrix {
		have[cell.Well+"\x00"+cell.Timepoint] = true
	}
	wellSet := map[string]bool{}
	for _, well := range proj.Wells {
		wellSet[well] = true
	}
	for _, asset := range assets {
		if !isImageAsset(asset) {
			continue
		}
		well, tp := normalizeCoord(asset.Link.WellID, asset.Link.Timepoint)
		if well == "" || tp == "" {
			continue
		}
		if !seenTP[tp] {
			seenTP[tp] = true
			proj.Timepoints = append(proj.Timepoints, tp)
		}
		if !wellSet[well] {
			wellSet[well] = true
			proj.Wells = append(proj.Wells, well)
			sort.SliceStable(proj.Wells, func(i, j int) bool { return wellLess(proj.Wells[i], proj.Wells[j]) })
		}
		key := well + "\x00" + tp
		if !have[key] {
			have[key] = true
			proj.Matrix = append(proj.Matrix, MatrixCell{Well: well, Timepoint: tp})
		}
	}
	sort.SliceStable(proj.Matrix, func(i, j int) bool {
		if proj.Matrix[i].Well != proj.Matrix[j].Well {
			return wellLess(proj.Matrix[i].Well, proj.Matrix[j].Well)
		}
		return timepointOrder(proj.Timepoints, proj.Matrix[i].Timepoint, proj.Matrix[j].Timepoint)
	})
}

func timepointOrder(order []string, a, b string) bool {
	ia, ib := len(order), len(order)
	for i, tp := range order {
		if tp == a {
			ia = i
		}
		if tp == b {
			ib = i
		}
	}
	return ia < ib
}

func normalizeCoord(well, tp string) (string, string) {
	return strings.ToUpper(strings.TrimSpace(well)), strings.TrimSpace(tp)
}

// assignImages places images by Core coordinates only: a link with a well, a
// timepoint, and a 1-based FOV index fills exactly that slot. Missing any
// coordinate stays in the explicit unassigned bucket — the same rule the Run
// gallery uses — and coordinates are never parsed out of file paths. A
// second image claiming a filled slot, or an index past FOVSlots, overflows
// instead of displacing the first.
func assignImages(proj *RecordProjection, assets []runs.Asset) {
	claims := map[string][]runs.Asset{}
	for _, asset := range assets {
		if !isImageAsset(asset) {
			continue
		}
		well, tp := normalizeCoord(asset.Link.WellID, asset.Link.Timepoint)
		if well == "" || tp == "" || asset.Link.FOVIndex == 0 {
			proj.UnassignedImages++
			proj.UnassignedLabels = append(proj.UnassignedLabels, assetLabel(asset))
			continue
		}
		claims[well+"\x00"+tp] = append(claims[well+"\x00"+tp], asset)
	}
	for key, list := range claims {
		sort.SliceStable(list, func(i, j int) bool {
			if list[i].RelativePath != list[j].RelativePath {
				return list[i].RelativePath < list[j].RelativePath
			}
			return list[i].ID < list[j].ID
		})
		for i := range proj.Matrix {
			cell := &proj.Matrix[i]
			if cell.Well+"\x00"+cell.Timepoint != key {
				continue
			}
			seen := map[uint32]bool{}
			for _, asset := range list {
				fov := asset.Link.FOVIndex
				if fov < 1 || fov > FOVSlots || seen[fov] || cell.Images[fov-1].Filled {
					cell.Overflow++
					continue
				}
				seen[fov] = true
				cell.Images[fov-1] = MatrixImage{
					Filled: true, AssetID: asset.ID,
					RootID:       asset.RootID,
					RelativePath: asset.RelativePath,
					Label:        assetLabel(asset),
					Description:  fmt.Sprintf("FOV %d · %s", fov, assetLabel(asset)),
				}
			}
		}
	}
	gaps := 0
	for _, cell := range proj.Matrix {
		gaps += FOVSlots - cell.FilledSlots()
	}
	proj.Gaps = gaps
}

func isImageAsset(asset runs.Asset) bool {
	mt := strings.ToLower(strings.TrimSpace(asset.MediaType))
	if mt == "image" {
		return true
	}
	switch strings.ToLower(lastExt(asset.RelativePath)) {
	case ".png", ".jpg", ".jpeg", ".tif", ".tiff", ".bmp", ".gif", ".webp":
		return true
	}
	return false
}

func lastExt(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '.' {
			return path[i:]
		}
		if path[i] == '/' || path[i] == '\\' {
			return ""
		}
	}
	return ""
}

func assetLabel(asset runs.Asset) string {
	if strings.TrimSpace(asset.RelativePath) != "" {
		rel := strings.ReplaceAll(asset.RelativePath, "\\", "/")
		if idx := strings.LastIndex(rel, "/"); idx >= 0 {
			return rel[idx+1:]
		}
		return rel
	}
	if len(asset.ID) > 8 {
		return asset.ID[:8]
	}
	return asset.ID
}

func parseParameters(raw string) []Param {
	if strings.TrimSpace(raw) == "" || strings.TrimSpace(raw) == "[]" {
		return nil
	}
	var items []struct {
		Key   string `json:"key"`
		Label string `json:"label"`
		Name  string `json:"name"`
		Value any    `json:"value"`
		Unit  string `json:"unit"`
	}
	if json.Unmarshal([]byte(raw), &items) != nil {
		return nil
	}
	out := make([]Param, 0, len(items))
	for _, item := range items {
		label := item.Label
		if label == "" {
			label = item.Name
		}
		if label == "" {
			label = item.Key
		}
		value := ""
		if item.Value != nil {
			value = fmt.Sprint(item.Value)
		}
		out = append(out, Param{Key: item.Key, Label: label, Value: value, Unit: item.Unit})
	}
	return out
}


func formatNumber(value float64) string {
	return fmt.Sprintf("%g", value)
}

func formatValuePtr(value *float64) string {
	if value == nil {
		return "未记录"
	}
	return formatNumber(*value)
}

// wellLabel names a result-cell coordinate the way operators read plates:
// row 0 is A, column 0 is 1.
func wellLabel(row, col int) string {
	letter := string(rune('A' + row%26))
	if row >= 26 {
		letter = "A" + letter
	}
	return fmt.Sprintf("%s%d", letter, col+1)
}

func wellLess(a, b string) bool {
	ra, ca := splitWell(a)
	rb, cb := splitWell(b)
	if ra != rb {
		return ra < rb
	}
	return ca < cb
}

func splitWell(well string) (string, int) {
	i := 0
	for i < len(well) && (well[i] < '0' || well[i] > '9') {
		i++
	}
	n := 0
	for _, digit := range well[i:] {
		n = n*10 + int(digit-'0')
	}
	return well[:i], n
}

// CompletedTasks counts task summaries whose frozen status is completed.
func (proj RecordProjection) CompletedTasks() int {
	n := 0
	for _, sum := range proj.Summary {
		if strings.EqualFold(strings.TrimSpace(sum.Status), "completed") {
			n++
		}
	}
	return n
}

// MeasurementCount counts matrix-cell values (ResultCell readouts placed in
// Well x Timepoint cells). It deliberately does not roll up per-task
// measurements: the same fact can appear in both, and the two cannot be
// deduplicated safely, so consumers must label this count narrowly.
func (proj RecordProjection) MeasurementCount() int {
	n := 0
	for _, cell := range proj.Matrix {
		n += len(cell.Values)
	}
	return n
}
