package record

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"asterlab-mygo-prototype/modules/runs"
)

func projFixture() Projection {
	str := func(s string) *string { return &s }
	u64 := func(v uint64) *uint64 { return &v }
	f64 := func(v float64) *float64 { return &v }
	eventType := func(s string) *string { return &s }
	return Projection{
		SchemaVersion: 1,
		RunID:         "run-1",
		Design: DesignSummary{
			DesignID: "design-1", DesignRevision: 3, Title: "HUVEC 成管",
			Groups: []Group{{Name: "3%"}, {Name: "5%"}},
			Assays: []Assay{{AssayID: "assay-huvec", Title: "成管", Endpoint: "OD450", Timepoints: []string{"0h", "6h"}}},
		},
		Parameters: []ParamCompare{
			{ScopeID: "root/cure", StepPath: str("root/cure"), Slot: "cure_s", Label: str("固化秒数"), Unit: str("s"),
				Planned: json.RawMessage(`60`), Actual: []ActualParam{{Value: json.RawMessage(`75`), Unit: str("s"), EventID: "e1"}}, Status: "differs"},
			{ScopeID: "root/cure", StepPath: str("root/cure"), Slot: "gel_pct", Label: str("GelMA 浓度"), Unit: str("%"),
				Planned: json.RawMessage(`5`), Status: "missing_actual"},
			{ScopeID: "root/extra", Slot: "stray", Planned: json.RawMessage(`1`),
				Actual: []ActualParam{{Value: json.RawMessage(`2`), EventID: "e2"}}, Status: "unplanned"},
		},
		Timers: []TimerObserve{
			{TimerID: "t1", TaskKey: "task-cure", Mode: "countdown", Status: "completed", PlannedS: u64(60), ElapsedS: u64(75)},
			{TimerID: "t2", TaskKey: "task-image", Mode: "countdown", Status: "running", PlannedS: u64(30)},
		},
		Images: ImageMatrix{
			FOVPerWell: 1, PlannedSlots: 2, FilledSlots: 1, MissingSlots: 1,
			UnlocatedAssetIDs: []string{"img-stray"}, UnplannedAssetIDs: []string{"img-extra"},
			Plates: []ImagePlateRow{{
				PlateID: "p1", Timepoints: []*string{str("0h"), str("6h")},
				PlannedSlots: 2, FilledSlots: 1,
				Wells: []ImageWellRow{
					{Coordinate: "A1", GroupLabel: str("3%"), PlannedSlots: 1, FilledSlots: 1,
						Slots: []ImageSlot{{Timepoint: str("6h"), FOVIndex: 1, AssetID: str("img-1")}}},
					{Coordinate: "A2", PlannedSlots: 1, Slots: []ImageSlot{{Timepoint: str("6h"), FOVIndex: 1}}},
				},
			}},
		},
		Assets: []RecordAsset{{AssetID: "img-1", TaskKey: "task-image", WellID: str("A1"), Timepoint: str("6h")}},
		Readings: []Reading{
			{RunID: "run-1", StepPath: "root/read", Endpoint: "OD450", Value: f64(0.82), Unit: str("AU"), SourceEventID: "e1"},
			{RunID: "run-1", StepPath: "root/read", Endpoint: "OD450", Unit: str("AU"), SourceEventID: "e2"},
		},
		Readouts: []ReadoutRecord{{Readout: json.RawMessage(`{"field":"OD450"}`), EventID: "e1", StepPath: "root/read"}},
		Timeline: TimelineSummary{
			SkippedByRule: 1,
			Events: []TimelineEvent{
				{EventID: "e1", EventType: eventType("operation"), TaskKey: str("task-cure"), Content: str("固化"), Deviation: str("流出液未完全弃置")},
				{EventID: "e2", EventType: eventType("skipped_by_rule"), TaskKey: str("task-cure"), Content: str("离心跳过")},
			},
		},
		Notes:   []NoteView{{NoteID: "n1", Markdown: "# 研判\n\n结论见对比 @matrix(6h)\n"}},
		Archive: ArchiveState{Status: "recording"},
		Issues:  []Issue{{Code: "timer_watermark_missing", Message: "缺水印", Path: str("timers/t2")}},
	}
}

func TestSummarizeOneCounting(t *testing.T) {
	proj := projFixture()
	tasks := SnapshotTasks([]runs.RunTask{
		{Key: "task-cure", Title: "固化", Timepoint: "0h", Status: "completed"},
		{Key: "task-image", Title: "采图", Timepoint: "6h", Status: "in_progress"},
	})
	if len(tasks) != 2 || tasks[0].Title != "固化" || tasks[1].Timepoint != "6h" {
		t.Fatalf("tasks = %+v", tasks)
	}
	c := Summarize(proj, tasks)
	if c.TasksTotal != 2 || c.TasksCompleted != 1 {
		t.Fatalf("tasks = %+v", c)
	}
	// Measurements: 1 valued reading + 1 readout; missing: 1 valueless
	// reading + 1 missing slot.
	if c.Measurements != 2 {
		t.Fatalf("measurements = %d", c.Measurements)
	}
	if c.Missing != 2 {
		t.Fatalf("missing = %d", c.Missing)
	}
	// Excluded: 1 unplanned param + 1 unlocated + 1 unplanned asset.
	if c.Excluded != 3 {
		t.Fatalf("excluded = %d", c.Excluded)
	}
	if c.SkippedByRule != 1 || c.Deviations != 1 {
		t.Fatalf("skips/deviations = %+v", c)
	}
	if c.Timers != 2 || c.TimersRunning != 1 {
		t.Fatalf("timers = %+v", c)
	}
	if c.Groups != 2 || c.Assets != 1 {
		t.Fatalf("groups/assets = %+v", c)
	}
	if c.ImagesPlanned != 2 || c.ImagesFilled != 1 || c.ImagesMissing != 1 {
		t.Fatalf("images = %+v", c)
	}
	if c.Issues != 1 {
		t.Fatalf("issues = %+v", c)
	}
	head := Headline(c)
	for _, want := range []string{"任务完成 1/2", "实测 2 个", "缺 2 个", "排除 3 个", "跳过 1 次", "计时 2 个"} {
		if !strings.Contains(head, want) {
			t.Fatalf("headline %q missing %q", head, want)
		}
	}
}

func TestGroupParameters(t *testing.T) {
	proj := projFixture()
	groups := GroupParameters(proj.Parameters)
	if len(groups) != 2 {
		t.Fatalf("groups = %+v", groups)
	}
	if !strings.Contains(groups[0].Title, "root/cure") {
		t.Fatalf("group title = %q", groups[0].Title)
	}
	byLabel := map[string]ParamRow{}
	for _, row := range groups[0].Rows {
		byLabel[row.Label] = row
	}
	if byLabel["固化秒数"].Planned != "60" || byLabel["固化秒数"].Actual != "75 s" {
		t.Fatalf("cure row = %+v", byLabel["固化秒数"])
	}
	if !strings.Contains(byLabel["GelMA 浓度"].Status, "缺") {
		t.Fatalf("missing status = %+v", byLabel["GelMA 浓度"])
	}
	if groups[1].Rows[0].Status != ParamStatusWord("unplanned") {
		t.Fatalf("unplanned status = %q", groups[1].Rows[0].Status)
	}
}

func TestStatusWords(t *testing.T) {
	if TaskStatusWord("completed") == "" || TaskStatusWord("mystery") != "mystery" {
		t.Fatal("task status word must pass unknown statuses through")
	}
	if !strings.Contains(ParamStatusWord("missing_actual"), "缺") {
		t.Fatal("missing_actual must read 缺")
	}
	if ArchiveStatusWord("archived") == "" || ArchiveStatusWord("recording") == "" {
		t.Fatal("archive status words must name the lifecycle")
	}
}

func TestStripLegacySignature(t *testing.T) {
	trailer := "<!-- record-sign run=run-1 rev=3 hash=" + strings.Repeat("a", 64) + " at=2026-11-16T12:00:00Z actor=lin -->"
	got := StripLegacySignature("# 研判\n\n贴壁良好\n" + trailer)
	if strings.Contains(got, "record-sign") {
		t.Fatalf("trailer not stripped: %q", got)
	}
	plain := "# 无签名正文\n"
	if StripLegacySignature(plain) != strings.TrimSpace(plain) {
		t.Fatalf("plain prose changed: %q", StripLegacySignature(plain))
	}
}

func TestMatrixRefsCellsAndImages(t *testing.T) {
	proj := projFixture()
	refs := FindMatrixRefs("研判 @matrix(6h,12h) 与 @matrix(*) 结束")
	if len(refs) != 2 || refs[0].Raw != "@matrix(6h,12h)" || !refs[1].All {
		t.Fatalf("refs = %+v", refs)
	}
	cells, unknown := ResolveMatrixRef(proj, refs[0])
	if len(cells) != 2 || len(unknown) != 1 || unknown[0] != "12h" {
		t.Fatalf("cells = %d unknown = %v", len(cells), unknown)
	}
	all, _ := ResolveMatrixRef(proj, refs[1])
	if len(all) != 2 {
		t.Fatalf("all = %d", len(all))
	}
	if got := MatrixCellText(MatrixCell{Well: "A1", Timepoint: "6h", Filled: 1, Planned: 1}); got != "图1/1" {
		t.Fatalf("cell text = %q", got)
	}
	if got := MatrixCellText(MatrixCell{Well: "A2", Timepoint: "6h"}); got != "缺" {
		t.Fatalf("gap text = %q", got)
	}
	if img := SlotImage(proj, "a1", "6h", 1); img == nil || img.AssetID != "img-1" {
		t.Fatalf("slot = %+v", img)
	}
	if img := SlotImage(proj, "A2", "6h", 1); img != nil {
		t.Fatalf("gap slot must be nil: %+v", img)
	}
	well, img, ok := RepresentativeGroup(proj, "3%", "6h", 1)
	if !ok || well != "A1" || img == nil || img.AssetID != "img-1" {
		t.Fatalf("representative = %q %+v %v", well, img, ok)
	}
	if _, _, ok := RepresentativeGroup(proj, "5%", "6h", 1); ok {
		t.Fatal("unknown group must select nothing")
	}
	if groups := MatrixGroups(proj); len(groups) != 2 || groups[0] != "3%" || groups[1] != "" {
		t.Fatalf("groups = %+v", groups)
	}
	if got := GroupDisplayName(proj, ""); got != "未分组" {
		t.Fatalf("ungrouped label = %q", got)
	}
	if out := ExpandMatrixRefs("结论：@matrix(6h)", proj); strings.Contains(out, "@matrix") {
		t.Fatalf("token not expanded: %q", out)
	}
	if plain := ExpandMatrixRefs("无引用正文", proj); plain != "无引用正文" {
		t.Fatalf("plain prose changed: %q", plain)
	}
}

func derivedFixture() runs.DerivedDesign {
	return runs.DerivedDesign{
		DesignID: "design-1", SourceDesignVersionID: "dv-3", SourceDesignRevision: 3,
		ExpectedRevision: 4, SourceRunID: "run-1",
		DesignJSON: `{"title":"HUVEC 成管 v2","timepoints":["0h","6h"],` +
			`"derived_from":{"design_version_id":"dv-3","design_revision":3,"run_id":"run-1",` +
			`"parameter_updates":[{"scope_id":"root/cure","parameter_key":"cure_s","planned":60,"actual":75,"event_id":"e1"}],` +
			`"duration_updates":[{"timer_id":"t1","task_key":"task-cure","phase_index":0,"planned_s":60,"actual_s":75}]}}`,
	}
}

func TestDecodeDerivedEvidence(t *testing.T) {
	ev, err := DecodeDerived(derivedFixture())
	if err != nil {
		t.Fatal(err)
	}
	if ev.SourceRunID != "run-1" || ev.SourceDesignRevision != 3 || ev.ExpectedRevision != 4 {
		t.Fatalf("provenance = %+v", ev)
	}
	if len(ev.Params) != 1 || ev.Params[0].ParameterKey != "cure_s" || ev.Params[0].EventID != "e1" {
		t.Fatalf("params = %+v", ev.Params)
	}
	if len(ev.Durations) != 1 || ev.Durations[0].ActualS != 75 {
		t.Fatalf("durations = %+v", ev.Durations)
	}
}

func TestChangesForDerived(t *testing.T) {
	derived := derivedFixture()
	ev, err := DecodeDerived(derived)
	if err != nil {
		t.Fatal(err)
	}
	out, err := ChangesForDerived(`{"title":"HUVEC 成管"}`, derived, ev, time.Date(2026, 11, 17, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	var shaped map[string]json.RawMessage
	if err := json.Unmarshal([]byte(out), &shaped); err != nil {
		t.Fatal(err)
	}
	if _, ok := shaped["derived_from"]; ok {
		t.Fatal("derived_from must be stripped: design.revise rejects it")
	}
	if _, ok := shaped["source_evidence"]; !ok {
		t.Fatal("source_evidence entry must be appended")
	}
	var existing struct {
		SourceEvidence []struct {
			ID string `json:"id"`
		} `json:"source_evidence"`
	}
	if err := json.Unmarshal([]byte(`{"source_evidence":[{"id":"manual-1"}]}`), &existing); err != nil {
		t.Fatal(err)
	}
	// Existing evidence is preserved: append, never replace.
	out2, err := ChangesForDerived(`{"title":"HUVEC 成管","source_evidence":[{"id":"manual-1"}]}`, derived, ev, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	var merged struct {
		SourceEvidence []map[string]any `json:"source_evidence"`
	}
	if err := json.Unmarshal([]byte(out2), &merged); err != nil {
		t.Fatal(err)
	}
	if len(merged.SourceEvidence) != 2 || merged.SourceEvidence[0]["id"] != "manual-1" {
		t.Fatalf("evidence = %+v", merged.SourceEvidence)
	}
	// Unknown draft keys fail loudly: persisting a partial draft would
	// silently lose design data.
	bad := derived
	bad.DesignJSON = `{"title":"x","mystery_field":1,"derived_from":{"run_id":"run-1"}}`
	if _, err := ChangesForDerived(`{}`, bad, ev, time.Now()); err == nil {
		t.Fatal("unknown draft key must fail, not silently drop")
	}
}
