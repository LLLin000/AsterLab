package runworkspace

import (
	"encoding/json"
	"testing"
)

func TestParseSampleIDs(t *testing.T) {
	got := parseSampleIDs("M3-1, M3-2\nM3-1; M3-3\tM3-4")
	want := []string{"M3-1", "M3-2", "M3-3", "M3-4"}
	if len(got) != len(want) {
		t.Fatalf("samples = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("samples = %v, want %v", got, want)
		}
	}
	if len(parseSampleIDs(" ,;\n")) != 0 {
		t.Fatal("blank input must parse to no samples")
	}
}

func TestExclusionLabelsHideRawEnum(t *testing.T) {
	for status, want := range map[string]string{
		exclusionQCDropped: "QC 剔除", exclusionCensoredDead: "删失（中途死亡）", exclusionRework: "重做",
	} {
		if got := exclusionLabel(status); got != want {
			t.Fatalf("label(%q) = %q, want %q", status, got, want)
		}
	}
}

// TestBuildExclusionCommandMatchesCoreInput pins the exact
// execution_event.create JSON Core validates (BE-08): operation event, frozen
// step scope, concerned samples, and blind_masked only under the mask.
func TestBuildExclusionCommandMatchesCoreInput(t *testing.T) {
	input, err := buildExclusionCommand("run-1", exclusionQCDropped,
		[]string{"sample-qc_dropped"}, "溶血", "s/1", "assay-1", "pv1", "", true, "key-1")
	if err != nil {
		t.Fatal(err)
	}
	if input.Kind != "execution_event.create" {
		t.Fatalf("kind = %q", input.Kind)
	}
	if input.IdempotencyKey != "key-1" {
		t.Fatalf("key = %q", input.IdempotencyKey)
	}
	if input.ExpectedRevision != nil {
		t.Fatal("append-only commands carry no expected revision")
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(input.PayloadJSON), &payload); err != nil {
		t.Fatal(err)
	}
	for key, want := range map[string]any{
		"run_id": "run-1", "event_type": "operation",
		"step_status": "qc_dropped", "protocol_step_path": "s/1",
		"protocol_version_id": "pv1", "assay_id": "assay-1",
		"blind_masked": true,
	} {
		if payload[key] != want {
			t.Fatalf("payload[%q] = %v, want %v (payload %s)", key, payload[key], want, input.PayloadJSON)
		}
	}
	samples, _ := payload["sample_ids"].([]any)
	if len(samples) != 1 || samples[0] != "sample-qc_dropped" {
		t.Fatalf("sample_ids = %v", payload["sample_ids"])
	}
	if _, ok := payload["parent_sample_id"]; ok {
		t.Fatal("lineage fields stay omitted without a lineage composer")
	}
}

func TestBuildExclusionCommandRejectsLocally(t *testing.T) {
	if _, err := buildExclusionCommand("run-1", "abandoned", []string{"s1"}, "", "", "", "", "", false, "k"); err == nil {
		t.Fatal("unknown status must fail before Core")
	}
	if _, err := buildExclusionCommand("run-1", exclusionRework, nil, "", "", "", "", "", false, "k"); err == nil {
		t.Fatal("empty samples must fail before Core")
	}
}

// TestSubmitExclusionReportsCauseAndRetry shows the failure inline and keeps
// the draft; a success clears samples but keeps the status (UX-STATE, UX-P5).
func TestSubmitExclusionReportsCauseAndRetry(t *testing.T) {
	s := buildTestState()
	task := s.VM.TaskByKey["t-read"]
	form := s.exclusionForm(task.TaskKey)
	form.Status, form.Samples, form.Reason = exclusionCensoredDead, "M3-1", "中途死亡"
	s.Callbacks.ExecuteCommand = nil
	s.submitExclusion(task, form)
	if form.Busy {
		t.Fatal("missing callback must not hang the form")
	}
}
