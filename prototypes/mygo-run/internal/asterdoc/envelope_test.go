package asterdoc

import (
	"strings"
	"testing"
)

const sampleDesignForEnvelope = `# 抑菌圈测定方案

` + "```asterlab\nkind: experiment\nid: exp-iz\nschema: 1\ntitle: 四象限抑菌圈对比\nscheduled_date: 2026-11-19\nobjective: 对比万古霉素与 HAMA 水凝胶抑菌活性\nprimary_endpoint: 抑菌圈直径 (mm)\n```\n\n```asterlab\nkind: step\nid: step-1\nschema: 1\ntitle: 01 铺板贴片\nblocks:\n  - kind: param_table\n    id: pt-1\n    schema: 1\n    title: 孵育参数\n    rows:\n      - {key: temp, label: 培养温度, value: 37, unit: Cel, record_actual: true}\n```\n"

func TestEnvelopeRoundTripNestedRecordAndSealVerification(t *testing.T) {
	sealDigest := ComputeDesignSeal(sampleDesignForEnvelope)
	fm := EnvelopeFrontmatter{
		AsterML:       EnvelopeFormatVersion,
		ID:            "exp-20261119-iz",
		Title:         "四象限抑菌圈对比",
		ScheduledDate: "2026-11-19",
		Stage:         "in_progress",
		Seal: &EnvelopeSeal{
			DesignSHA256: sealDigest,
			FrozenAt:     "2026-11-19T14:00:00Z",
		},
	}
	exec := &ExecutionLedger{
		Schema:       1,
		RunID:        "run-iz-1",
		DesignSHA256: sealDigest,
		Status:       "in_progress",
		Checks:       map[string]string{"step-1:chk-1": "checked"},
		ActualParams: map[string]any{"temp": 36.8},
	}
	nestedRecordMD := "# 运行观察\n\n万古霉素阳性组清楚呈现抑菌透明圈。\n\n```asterlab\nkind: param_table\nid: note-nested-pt\nschema: 1\ntitle: 实测复核\nrows:\n  - {key: zone, label: 直径, value: 21.0, unit: mm}\n```"
	rec := &RecordSection{
		Schema:    1,
		RunID:     "run-iz-1",
		UpdatedAt: "2026-11-19T16:30:00Z",
		Markdown:  nestedRecordMD,
	}

	raw, err := ComposeEnvelope(fm, sampleDesignForEnvelope, exec, rec)
	if err != nil {
		t.Fatalf("ComposeEnvelope: %v", err)
	}
	if !strings.Contains(raw, "````asterlab-record") {
		t.Fatalf("expected 4-backtick asterlab-record wrapper:\n%s", raw)
	}

	// Parsing raw directly as a Design Document must skip frontmatter, asterlab-exec,
	// AND the nested ```asterlab``` block inside 4-backtick ````asterlab-record!
	doc, err := Parse(raw)
	if err != nil {
		t.Fatalf("Parse(envelope) failed (embedded fence collision?): %v", err)
	}
	if len(doc.Modules) != 2 {
		t.Fatalf("Parse(envelope) modules = %d, want 2 (must exclude nested record module)", len(doc.Modules))
	}

	gotFM, fmEnd, ok := ScanFrontmatter(raw)
	if !ok || fmEnd == 0 || gotFM.ID != "exp-20261119-iz" || gotFM.ScheduledDate != "2026-11-19" {
		t.Fatalf("ScanFrontmatter = %+v end=%d ok=%v", gotFM, fmEnd, ok)
	}

	env, err := ParseEnvelope(raw)
	if err != nil {
		t.Fatalf("ParseEnvelope: %v", err)
	}
	sealed, valid := env.VerifyDesignSeal()
	if !sealed || !valid {
		t.Fatalf("VerifyDesignSeal = (%v, %v), want (true, true)", sealed, valid)
	}
	if env.Execution == nil || env.Execution.RunID != "run-iz-1" || env.Execution.Checks["step-1:chk-1"] != "checked" {
		t.Fatalf("Execution mismatch: %+v", env.Execution)
	}
	if env.Record == nil || env.Record.RunID != "run-iz-1" || !strings.Contains(env.Record.Markdown, "note-nested-pt") {
		t.Fatalf("Record mismatch: %+v", env.Record)
	}

	// Tampering with Design after sealing must be detected immediately.
	tampered := strings.Replace(raw, "value: 37", "value: 42", 1)
	tamperedEnv, err := ParseEnvelope(tampered)
	if err != nil {
		t.Fatalf("ParseEnvelope(tampered): %v", err)
	}
	sealed, valid = tamperedEnv.VerifyDesignSeal()
	if !sealed || valid {
		t.Fatalf("tampered VerifyDesignSeal = (%v, %v), want (true, false)", sealed, valid)
	}
}

func TestEnvelopeSurgicalPatchPreservesDesignAndSeal(t *testing.T) {
	raw, err := ComposeEnvelope(EnvelopeFrontmatter{
		AsterML:       EnvelopeFormatVersion,
		ID:            "exp-orig",
		Title:         "四象限抑菌圈对比",
		ScheduledDate: "2026-11-19",
		Stage:         "ready",
	}, sampleDesignForEnvelope, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Patch execution ledger onto envelope; seal is stamped automatically and stage advances to in_progress.
	patchedExec, err := PatchEnvelopeExecution(raw, ExecutionLedger{
		Schema: 1,
		RunID:  "run-42",
		Status: "in_progress",
		Checks: map[string]string{"step-1:chk-1": "checked"},
	})
	if err != nil {
		t.Fatalf("PatchEnvelopeExecution: %v", err)
	}
	// Patch notebook record onto the already-sealed file without altering Design bytes or Execution ledger.
	patchedBoth, err := PatchEnvelopeRecord(patchedExec, RecordSection{
		Schema:    1,
		RunID:     "run-42",
		UpdatedAt: "2026-11-19T18:00:00Z",
		Markdown:  "# 实验结论\n\n结果稳定。\n\n```asterlab\nkind: prose\nid: p-note\nschema: 1\nbody: 正常\n```",
	})
	if err != nil {
		t.Fatalf("PatchEnvelopeRecord: %v", err)
	}
	env, err := ParseEnvelope(patchedBoth)
	if err != nil {
		t.Fatalf("ParseEnvelope(patchedBoth): %v", err)
	}
	if sealed, valid := env.VerifyDesignSeal(); !sealed || !valid {
		t.Fatalf("seal invalid after record patch: sealed=%v valid=%v", sealed, valid)
	}
	if env.Execution == nil || env.Execution.RunID != "run-42" {
		t.Fatalf("execution lost after record patch: %+v", env.Execution)
	}
	if env.Frontmatter.Stage != "in_progress" {
		t.Fatalf("stage = %q, want in_progress", env.Frontmatter.Stage)
	}

	// Forking creates a clean unsealed draft with repeat_of provenance and new date.
	forked, err := ForkEnvelopeAsDraft(patchedBoth, "exp-fork-2", "2026-11-26")
	if err != nil {
		t.Fatalf("ForkEnvelopeAsDraft: %v", err)
	}
	forkEnv, err := ParseEnvelope(forked)
	if err != nil {
		t.Fatal(err)
	}
	if forkEnv.Frontmatter.RepeatOf != "exp-orig" || forkEnv.Frontmatter.ScheduledDate != "2026-11-26" || forkEnv.Frontmatter.Stage != "draft" {
		t.Fatalf("fork frontmatter = %+v", forkEnv.Frontmatter)
	}
	if forkEnv.Frontmatter.Seal != nil || forkEnv.Execution != nil || forkEnv.Record != nil {
		t.Fatalf("fork should clear seal/exec/record: %+v", forkEnv)
	}
	if InspectExperiment(forkEnv.DesignSource, nil).ScheduledDate != "2026-11-26" {
		t.Fatalf("fork Design scheduled_date not updated:\n%s", forkEnv.DesignSource)
	}
}

func TestStartTimeFrontmatterRoundTrip(t *testing.T) {
	raw := "---\nasterml: \"2.1\"\ntitle: Timed Assay\nscheduled_date: \"2026-11-24\"\n---\n\n# Timed Assay\n"
	updated := SetFrontmatterStartTime(raw, "14:30")
	fm, _, ok := ScanFrontmatter(updated)
	if !ok {
		t.Fatalf("ScanFrontmatter failed on updated source:\n%s", updated)
	}
	if fm.StartTime != "14:30" {
		t.Fatalf("StartTime = %q, want 14:30", fm.StartTime)
	}
	if h := ParseStartHour(fm.StartTime); h != 14.5 {
		t.Fatalf("ParseStartHour = %v, want 14.5", h)
	}
}
