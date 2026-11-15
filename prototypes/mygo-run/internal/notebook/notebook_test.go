package notebook

import (
	"reflect"
	"testing"

	"asterlab-mygo-prototype/modules/runs"
)

func fixture() runs.RunSnapshot {
	value := 0.82
	return runs.RunSnapshot{
		RunID:          "run-1",
		DesignID:       "design-1",
		DesignRevision: 3,
		Status:         "in_progress",
		Tasks: []runs.RunTask{
			{Key: "task-1", AssayID: "cck8", Timepoint: "D3", DesignStepID: "step-s1"},
		},
		Events: []runs.RunEvent{
			{
				ID: "evt-batch", EventType: "measurement", TaskKey: "task-1", Timepoint: "D3",
				BatchID: "batch-1", SchemaID: "cck8.plate96.od450.v1", SchemaVersion: 1,
				OccurredAt: "2026-11-10T10:42:00+08:00", RecordedAt: "2026-11-10T10:43:00+08:00",
				ResultCells: []runs.ResultCell{
					{Row: 0, Column: 0, SampleID: "S1", Field: "OD450", Value: &value},
					{Row: 0, Column: 1, SampleID: "S1", Field: "OD450", Value: &value},
					{Row: 0, Column: 2, SampleID: "S1", Field: "OD450", Value: &value},
				},
			},
			{
				ID: "evt-done", EventType: "operation", Content: "完成测量任务",
				TaskKey: "task-1", StepStatus: "completed",
				OccurredAt: "2026-11-10T10:50:00+08:00", RecordedAt: "2026-11-10T10:50:30+08:00",
			},
			{
				// A stored event without timestamps: they must stay unknown.
				ID: "evt-legacy", EventType: "observation", Content: "孔板边缘有气泡",
			},
			{
				// A duplicate load of the same event must not repeat the entry.
				ID: "evt-batch", EventType: "measurement", BatchID: "batch-1",
			},
		},
	}
}

// TestBuildAggregatesBatchesAndKeepsTimesApart pins the Notebook contract:
// one batch is one entry, times are not conflated, and duplicates collapse.
func TestBuildAggregatesBatchesAndKeepsTimesApart(t *testing.T) {
	nb := Build(fixture())
	if nb.RunID != "run-1" || nb.DesignRevision != 3 {
		t.Fatalf("header = %+v", nb)
	}
	if len(nb.Entries) != 4 { // run + batch + completion + legacy observation
		t.Fatalf("entries = %d: %+v", len(nb.Entries), nb.Entries)
	}
	batch := nb.Entries[1]
	if batch.Category != Measurement || batch.Title != "保存读数批次" {
		t.Fatalf("batch entry = %+v", batch)
	}
	if batch.DesignStepID != "step-s1" || batch.TaskID != "task-1" {
		t.Fatalf("batch lost its step link: %+v", batch)
	}
	if batch.OccurredAt != "2026-11-10T10:42:00+08:00" || batch.RecordedAt != "2026-11-10T10:43:00+08:00" {
		t.Fatalf("times were conflated: %+v", batch)
	}
	refs := map[string]string{}
	for _, ref := range batch.References {
		refs[ref.Kind] = ref.ID
	}
	if refs["readout_batch"] != "batch-1" || refs["execution_event"] != "evt-batch" || refs["run_task"] != "task-1" {
		t.Fatalf("batch references = %+v", batch.References)
	}
	details := map[string]string{}
	for _, field := range batch.Details {
		details[field.Label] = field.Value
	}
	if details["单元格"] != "3 个" || details["批次"] != "batch-1" {
		t.Fatalf("batch details = %+v", batch.Details)
	}
	last := nb.Entries[len(nb.Entries)-1]
	if last.SourceID != "evt-legacy" || last.OccurredAt != "" || last.RecordedAt != "" {
		t.Fatalf("unknown-time event was filled in or reordered: %+v", last)
	}
}

// TestBuildIsDeterministicAndReadOnly guards restart and reload: the same
// snapshot yields the same projection, and nothing in the snapshot changes.
func TestBuildIsDeterministicAndReadOnly(t *testing.T) {
	snap := fixture()
	before := fixture()
	first := Build(snap)
	second := Build(snap)
	if !reflect.DeepEqual(first, second) {
		t.Fatal("two projections of one snapshot differ")
	}
	if !reflect.DeepEqual(snap, before) {
		t.Fatal("projection modified the snapshot")
	}
}

// TestBuildMarksUntraceableRecords checks that missing source identity is
// reported, not papered over.
func TestBuildMarksUntraceableRecords(t *testing.T) {
	snap := fixture()
	snap.Events = append(snap.Events, runs.RunEvent{EventType: "measurement"})
	nb := Build(snap)
	var found *Entry
	for i := range nb.Entries {
		if nb.Entries[i].SourceID == "" {
			found = &nb.Entries[i]
		}
	}
	if found == nil {
		t.Fatalf("record without a source is missing: %+v", nb.Entries)
	}
	if found.Trace != TraceMissing {
		t.Fatalf("record without a source claimed traceability: %+v", *found)
	}
}

func TestTraceStateIsDerivedFromStoredSources(t *testing.T) {
	base := runs.RunSnapshot{
		RunID: "run-1", DesignID: "design-1", DesignRevision: 3,
		Tasks:  []runs.RunTask{{Key: "task-1", DesignStepID: "step-s1"}},
		Events: []runs.RunEvent{{ID: "evt-1", EventType: "operation", TaskKey: "task-1"}},
	}
	book := Build(base)
	if got := book.Entries[1].Trace; got != TraceRecorded {
		t.Fatalf("resolved entry trace = %q, want recorded", got)
	}

	danglingTask := base
	danglingTask.Tasks = nil
	book = Build(danglingTask)
	if got := book.Entries[1].Trace; got != TraceInvalid {
		t.Fatalf("dangling task trace = %q, want invalid", got)
	}

	noEventID := runs.RunSnapshot{
		RunID: "run-1", DesignRevision: 3,
		Tasks:  []runs.RunTask{{Key: "task-1"}},
		Events: []runs.RunEvent{{EventType: "operation", TaskKey: "task-1", Content: "执行"}},
	}
	book = Build(noEventID)
	if got := book.Entries[1].Trace; got != TraceMissing {
		t.Fatalf("entry without event id trace = %q, want missing", got)
	}
}

// TestTimelineProjectsEventsInRecordedOrder pins the timeline contract: the
// projection lists one entry per stored record, ordered by its stored time,
// and never reorders the records that have a time.
func TestTimelineProjectsEventsInRecordedOrder(t *testing.T) {
	snap := fixture()
	nb := Build(snap)
	var ids []string
	for _, entry := range nb.Entries[1:] {
		ids = append(ids, entry.SourceID)
	}
	// batch 10:42, completion 10:50, then the untimed observation by id.
	want := []string{"evt-batch", "evt-done", "evt-legacy"}
	if !reflect.DeepEqual(ids, want) {
		t.Fatalf("timeline order = %v, want %v", ids, want)
	}
}

// TestCitationResolvesToTimelineEntryId checks that a citation points at an
// timeline entry that exists, instead of inventing a place for it.
func TestCitationResolvesToTimelineEntryId(t *testing.T) {
	snap := fixture()
	eventCitation := NewEventCitation(snap.Events[0])
	target, ok := CitationTargetOf(eventCitation, snap, nil)
	if !ok {
		t.Fatal("event citation did not resolve")
	}
	if target.EntryID != "event:evt-batch" {
		t.Fatalf("event citation entry = %q", target.EntryID)
	}
	// The target must name the same entry Build produces.
	var found bool
	for _, entry := range Build(snap).Entries {
		if entry.ID == target.EntryID {
			found = true
		}
	}
	if !found {
		t.Fatalf("citation target %q is not a projected entry", target.EntryID)
	}

	batchCitation := Citation{Kind: "batch", ID: "batch-1"}
	if target, ok = CitationTargetOf(batchCitation, snap, nil); !ok || target.EntryID != "event:evt-batch" {
		t.Fatalf("batch citation target = %+v ok=%v", target, ok)
	}

	// A step citation resolves to the entry of the task it names.
	stepCitation := NewStepCitation(snap.Tasks[0])
	if target, ok = CitationTargetOf(stepCitation, snap, nil); !ok || target.EntryID != "event:evt-batch" {
		t.Fatalf("step citation target = %+v ok=%v", target, ok)
	}

	// An unknown citation must not resolve.
	if _, ok = CitationTargetOf(Citation{Kind: "event", ID: "evt-absent"}, snap, nil); ok {
		t.Fatal("absent event citation resolved")
	}
}

// TestCitationTargetCopiesNoContent guards the "citation, not a second copy"
// rule: the target carries only identity and a label, never stored content.
func TestCitationTargetCopiesNoContent(t *testing.T) {
	snap := fixture()
	citation := NewEventCitation(snap.Events[2]) // "孔板边缘有气泡"
	before := Build(snap)
	target, ok := CitationTargetOf(citation, snap, nil)
	if !ok {
		t.Fatal("observation citation did not resolve")
	}
	if target.SourceID != "evt-legacy" || target.EntryID != "event:evt-legacy" {
		t.Fatalf("target lost its identity: %+v", target)
	}
	// Resolution must not add a second store of the event: the projection is
	// deeply equal before and after, so the citation only points at it.
	if after := Build(snap); !reflect.DeepEqual(before, after) {
		t.Fatal("resolving a citation changed the timeline projection")
	}
	var matches int
	for _, entry := range before.Entries {
		if entry.SourceID == citation.ID {
			matches++
		}
	}
	if matches != 1 {
		t.Fatalf("event appears %d times in the timeline, want exactly one", matches)
	}
}
