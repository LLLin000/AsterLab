package record

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"

	"asterlab-mygo-prototype/modules/runs"
)

func recordFixture() (runs.RunSnapshot, []runs.Asset) {
	snap := runs.RunSnapshot{
		RunID: "run-1", Title: "HUVEC 成管",
		DesignID: "design-1", DesignRevision: 3, Status: "in_progress",
		Tasks: []runs.RunTask{
			{
				Key: "task-cure", Title: "405nm 固化", Timepoint: "0h",
				Status: "completed", DesignStepID: "step-2",
				ParametersJSON: `[{"key":"cure_s","label":"固化秒数","value":60,"unit":"s"},{"key":"gel_pct","label":"GelMA 浓度","value":5,"unit":"%"}]`,
			},
			{
				Key: "task-image", Title: "采图", Timepoint: "6h",
				Status: "in_progress", DesignStepID: "step-5",
				ParametersJSON: `[{"key":"fov","label":"视野数","value":6}]`,
			},
		},
		Events: []runs.RunEvent{
			{
				ID: "evt-cure", EventType: "operation", TaskKey: "task-cure",
				Timepoint: "0h", Content: "按组固化",
				OccurredAt: "2026-11-10T10:00:00+08:00", RecordedAt: "2026-11-10T10:01:00+08:00",
				DataJSON: `{"actual_parameters":[{"key":"cure_s","value":75,"unit":"s"}],"operator":"lin"}`,
			},
			{
				ID: "evt-skip", EventType: "operation", TaskKey: "task-cure",
				Timepoint: "0h", Content: "离心跳过",
				OccurredAt: "2026-11-10T10:05:00+08:00", RecordedAt: "2026-11-10T10:05:00+08:00",
				DataJSON: `{"skipped_by_rule":"no-bubble","deviation":"流出液未完全弃置"}`,
			},
			{
				ID: "evt-read", EventType: "measurement", TaskKey: "task-image",
				Timepoint: "6h", BatchID: "batch-1",
				SchemaID: "huvec.tube.v1", SchemaVersion: 1,
				OccurredAt: "2026-11-10T16:00:00+08:00", RecordedAt: "2026-11-10T16:01:00+08:00",
				ResultCells: []runs.ResultCell{
					{Row: 0, Column: 0, SampleID: "S1", Field: "OD450", Value: new(0.82)},
					{Row: 0, Column: 1, SampleID: "S2", Field: "OD450", Value: new(0.91)},
				},
			},
		},
		Measurements: []runs.Measurement{
			{SampleID: "S1", Endpoint: "RNA 浓度", Value: 63.8, Unit: "ng/uL", TaskKey: "task-image"},
		},
	}
	assets := []runs.Asset{
		{ID: "img-1", RelativePath: "run-1/6h/S1-fov1.png", MediaType: "image",
			Link: runs.AssetLink{RunID: "run-1", TaskKey: "task-image", SampleID: "S1", WellID: "A1", Timepoint: "6h", FOVIndex: 1}},
		{ID: "img-2", RelativePath: "run-1/6h/S1-fov2.png", MediaType: "image",
			Link: runs.AssetLink{RunID: "run-1", TaskKey: "task-image", SampleID: "S1", WellID: "A1", Timepoint: "6h", FOVIndex: 2}},
		{ID: "img-dup", RelativePath: "run-1/6h/S1-fov1b.png", MediaType: "image",
			Link: runs.AssetLink{RunID: "run-1", TaskKey: "task-image", SampleID: "S1", WellID: "A1", Timepoint: "6h", FOVIndex: 1}},
		// No Core coordinates: stays unassigned even though the sample
		// matches a measured well. Coordinates are never guessed.
		{ID: "img-sample-only", RelativePath: "run-1/sample.png", MediaType: "image",
			Link: runs.AssetLink{RunID: "run-1", TaskKey: "task-image", SampleID: "S1"}},
		{ID: "img-orphan", RelativePath: "run-1/stray.png", MediaType: "image",
			Link: runs.AssetLink{RunID: "run-1", TaskKey: "task-unknown", SampleID: "SX"}},
		{ID: "doc-1", RelativePath: "run-1/notes.pdf", MediaType: "document",
			Link: runs.AssetLink{RunID: "run-1", TaskKey: "task-image"}},
	}
	return snap, assets
}

func TestProjectPlannedVsActual(t *testing.T) {
	snap, assets := recordFixture()
	proj := Project(snap, assets)
	if proj.RunID != "run-1" || proj.DesignRevision != 3 {
		t.Fatalf("header = %+v", proj)
	}
	if len(proj.Summary) != 2 {
		t.Fatalf("summaries = %d", len(proj.Summary))
	}
	cure := proj.Summary[0]
	if len(cure.Planned) != 2 || cure.Planned[0].Value != "60" {
		t.Fatalf("planned = %+v", cure.Planned)
	}
	actual := map[string]Param{}
	for _, p := range cure.Actual {
		actual[p.Key] = p
	}
	if actual["cure_s"].Value != "75" {
		t.Fatalf("actual cure_s = %+v", cure.Actual)
	}
	if !cure.Skipped {
		t.Fatal("skipped_by_rule was not detected")
	}
	if len(cure.Deviations) != 1 || !strings.Contains(cure.Deviations[0], "流出液") {
		t.Fatalf("deviations = %+v", cure.Deviations)
	}
	image := proj.Summary[1]
	if len(image.Measurements) == 0 {
		t.Fatalf("image measurements = %+v", image.Measurements)
	}
	found := false
	for _, m := range image.Measurements {
		if m.Endpoint == "RNA 浓度" && m.Value == "63.8" {
			found = true
		}
	}
	if !found {
		t.Fatalf("task-level snapshot measurement missing: %+v", image.Measurements)
	}
}

func TestProjectMatrixFOVAndGaps(t *testing.T) {
	snap, assets := recordFixture()
	proj := Project(snap, assets)
	if !reflect.DeepEqual(proj.Wells, []string{"A1", "A2"}) {
		t.Fatalf("wells = %+v", proj.Wells)
	}
	if !reflect.DeepEqual(proj.Timepoints, []string{"0h", "6h"}) {
		t.Fatalf("timepoints = %+v", proj.Timepoints)
	}
	// Only the 6h task recorded result cells: two cells, each with 6 FOV slots.
	if len(proj.Matrix) != 2 {
		t.Fatalf("matrix cells = %+v", proj.Matrix)
	}
	var a1 *MatrixCell
	for i := range proj.Matrix {
		if proj.Matrix[i].Well == "A1" {
			a1 = &proj.Matrix[i]
		}
	}
	if a1 == nil || a1.FilledSlots() != 2 {
		t.Fatalf("A1 cell = %+v", a1)
	}
	if got := a1.Representative(1); got == nil || got.AssetID != "img-1" {
		t.Fatalf("FOV 1 = %+v", got)
	}
	if got := a1.Representative(2); got == nil || got.AssetID != "img-2" {
		t.Fatalf("FOV 2 = %+v", got)
	}
	if got := a1.Representative(3); got != nil {
		t.Fatalf("FOV 3 should be a gap, got %+v", got)
	}
	// 2 cells x 6 slots - 2 filled = 10 gaps. The duplicate FOV 1 claim
	// overflows instead of displacing the first image.
	if proj.Gaps != 10 {
		t.Fatalf("gaps = %d", proj.Gaps)
	}
	if a1.Overflow != 1 {
		t.Fatalf("A1 overflow = %d", a1.Overflow)
	}
	// The sample-only image stays unassigned: coordinates are never guessed.
	if proj.UnassignedImages != 2 || len(proj.UnassignedLabels) != 2 {
		t.Fatalf("unassigned = %d %v", proj.UnassignedImages, proj.UnassignedLabels)
	}
	if len(proj.Matrix[0].Values) == 0 {
		t.Fatal("matrix cell lost its measured values")
	}
}

func TestProjectIsDeterministicAndReadOnly(t *testing.T) {
	snap, assets := recordFixture()
	before, _ := json.Marshal(snap)
	first := Project(snap, assets)
	second := Project(snap, assets)
	a, _ := json.Marshal(first)
	b, _ := json.Marshal(second)
	if string(a) != string(b) {
		t.Fatal("projection is not deterministic")
	}
	after, _ := json.Marshal(snap)
	if string(before) != string(after) {
		t.Fatal("projection mutated the snapshot")
	}
}

func TestFindAndResolveMatrixRefs(t *testing.T) {
	snap, assets := recordFixture()
	proj := Project(snap, assets)
	refs := FindMatrixRefs("研判 @matrix(6h,12h) 与 @matrix(*) 结束")
	if len(refs) != 2 || refs[0].Raw != "@matrix(6h,12h)" || !refs[1].All {
		t.Fatalf("refs = %+v", refs)
	}
	cells, unknown := ResolveMatrixRef(proj, refs[0])
	if len(cells) != 2 || len(unknown) != 1 || unknown[0] != "12h" {
		t.Fatalf("cells = %d unknown = %v", len(cells), unknown)
	}
	all, _ := ResolveMatrixRef(proj, refs[1])
	if len(all) != len(proj.Matrix) {
		t.Fatalf("all = %d, matrix = %d", len(all), len(proj.Matrix))
	}
}

func TestExpandMatrixRefs(t *testing.T) {
	snap, assets := recordFixture()
	proj := Project(snap, assets)
	out := ExpandMatrixRefs("结论：@matrix(6h)", proj)
	if strings.Contains(out, "@matrix") {
		t.Fatalf("token not expanded: %q", out)
	}
	if !strings.Contains(out, "A1") || !strings.Contains(out, "0.82") {
		t.Fatalf("table missing well or value: %q", out)
	}
	plain := ExpandMatrixRefs("无引用正文", proj)
	if plain != "无引用正文" {
		t.Fatalf("plain prose changed: %q", plain)
	}
}

func TestSignVerifyRoundTrip(t *testing.T) {
	signed, sig := Sign("# 研判\n\n贴壁良好\n", "run-1", 7, "lin", time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC))
	if sig.RunID != "run-1" || sig.Revision != 7 || len(sig.Hash) != 64 {
		t.Fatalf("sig = %+v", sig)
	}
	if got, ok := Verify(signed); !ok || got.Hash != sig.Hash {
		t.Fatalf("verify = %+v %v", got, ok)
	}
	if _, ok := Verify(signed + "\n多加一句\n"); ok {
		t.Fatal("tampered prose verified")
	}
	if _, ok := Verify("# 无签名正文\n"); ok {
		t.Fatal("unsigned prose verified")
	}
	if got := StripSignature(signed); strings.Contains(got, "record-sign") {
		t.Fatalf("trailer not stripped: %q", got)
	}
	// Re-signing covers the prose, never the old trailer.
	resigned, _ := Sign(signed, "run-1", 8, "lin", time.Now())
	if strings.Count(resigned, "record-sign") != 1 {
		t.Fatalf("double trailer: %q", resigned)
	}
	if _, ok := Verify(resigned); !ok {
		t.Fatal("resigned prose does not verify")
	}
}

func TestClassifyRecordState(t *testing.T) {
	if ClassifyRecordState(true, true) != StateUnsaved {
		t.Fatal("dirty prose must read 未保存 even with a stale signature")
	}
	if ClassifyRecordState(false, true) != StateSealed {
		t.Fatal("verified prose must read 已签核归档")
	}
	if ClassifyRecordState(false, false) != StateSaved {
		t.Fatal("clean unsigned prose must read 已保存")
	}
	if ClassifyRecordState(true, false) != StateUnsaved {
		t.Fatal("dirty prose must read 未保存")
	}
	if string(StateSealed) != "已签核归档" || string(StateSaved) != "已保存" || string(StateUnsaved) != "未保存" {
		t.Fatal("status vocabulary drifted")
	}
}

func TestDeriveDesign(t *testing.T) {
	snap, assets := recordFixture()
	derived := DeriveDesign(Project(snap, assets))
	if derived.SourceRunID != "run-1" || derived.SourceDesignRevision != 3 {
		t.Fatalf("provenance = %+v", derived)
	}
	byKey := map[string]DerivedParameter{}
	for _, p := range derived.Parameters {
		byKey[p.TaskKey+"\x00"+p.Key] = p
	}
	cure := byKey["task-cure\x00cure_s"]
	if cure.Measured != "75" || cure.Planned != "60" || !strings.Contains(cure.Source, "actual_parameters") {
		t.Fatalf("cure_s = %+v", cure)
	}
	gel := byKey["task-cure\x00gel_pct"]
	if gel.Measured != "" || !strings.Contains(gel.Source, "unmeasured") {
		t.Fatalf("gel_pct = %+v", gel)
	}
	rna, ok := byKey["task-image\x00measure:RNA 浓度"]
	if !ok || rna.Measured != "63.8" {
		t.Fatalf("rna = %+v %v", rna, ok)
	}
	if derived.JSON() == "{}" || !strings.Contains(derived.JSON(), "sourceRunId") {
		t.Fatal("derived JSON is not a handoff payload")
	}
}
