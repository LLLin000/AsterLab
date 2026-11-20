package asterdoc

import (
	"os"
	"strings"
	"testing"
)

func TestInspectExperimentCompletenessAndDate(t *testing.T) {
	blank := "```asterlab\nkind: experiment\nid: exp-1\nschema: 1\ntitle: 新实验\nscheduled_date: 2026-11-20\n```\n"
	meta := InspectExperiment(blank, nil)
	if meta.Complete || meta.IncompleteReason != "待填写实验标题" {
		t.Fatalf("blank title check failed: %+v", meta)
	}
	if meta.ScheduledDate != "2026-11-20" {
		t.Fatalf("scheduled_date = %q, want 2026-11-20", meta.ScheduledDate)
	}

	noSteps := "```asterlab\nkind: experiment\nid: exp-1\nschema: 1\ntitle: CCK-8 测试\n```\n"
	meta = InspectExperiment(noSteps, nil)
	if meta.Complete || meta.IncompleteReason != "待添加实验步骤" {
		t.Fatalf("no steps check failed: %+v", meta)
	}

	emptyStep := "```asterlab\nkind: experiment\nid: exp-1\nschema: 1\ntitle: CCK-8 测试\n```\n```asterlab\nkind: step\nid: step-1\nschema: 1\ntitle: 01 细胞接种\nblocks: []\n```\n"
	meta = InspectExperiment(emptyStep, nil)
	if meta.Complete || meta.FirstGapStepID != "step-1" || !strings.Contains(meta.IncompleteReason, "待补充实验模块") {
		t.Fatalf("empty step check failed: %+v", meta)
	}

	emptyPlate := emptyStep[:strings.Index(emptyStep, "blocks:")] + "blocks:\n  - kind: plate\n    id: pl-1\n    schema: 1\n    format: 96\n    groups: {}\n```\n"
	meta = InspectExperiment(emptyPlate, nil)
	if meta.Complete || meta.IncompleteReason != "孔板尚未分配样本孔位" {
		t.Fatalf("empty plate check failed: %+v", meta)
	}

	raw, err := os.ReadFile("testdata/inhibition-zone.asterml")
	if err != nil {
		t.Fatal(err)
	}
	stamped := StampScheduledDate(string(raw), "2026-11-21T08:00:00Z")
	meta = InspectExperiment(stamped, nil)
	if !meta.Complete {
		t.Fatalf("flagship inhibition-zone should be complete, got %+v", meta)
	}
	if meta.ScheduledDate != "2026-11-21" {
		t.Fatalf("stamped scheduled_date = %q, want 2026-11-21", meta.ScheduledDate)
	}
}

func TestUnknownKeyAndCrossModuleWarningsParity(t *testing.T) {
	badYAML := "```asterlab\nkind: assay\nid: a1\nschema: 1\ntitle: 测试检测\nbogus_field: 1\n```\n"
	if _, err := Parse(badYAML); err == nil || !strings.Contains(err.Error(), "unknown field \"bogus_field\"") {
		t.Fatalf("expected unknown field error, got %v", err)
	}

	mismatchYAML := `
` + "```asterlab" + `
kind: experiment
id: exp-m
schema: 1
title: 数量校验测试
` + "```" + `
` + "```asterlab" + `
kind: group_table
id: gt-m
schema: 1
rows:
  - {id: g1, name: 对照组, role: blank, independent_n: 99}
` + "```" + `
` + "```asterlab" + `
kind: plate
id: p1
schema: 1
format: 96
groups:
  g1: [B2, B3, B4]
` + "```" + `
` + "```asterlab" + `
kind: calculator
id: c1
schema: 1
mode: group_mastermix
plate_ref: p1
sample_count: 999
volume_per_unit: {value: 100, unit: uL}
` + "```" + `
`
	doc, err := Parse(mismatchYAML)
	if err != nil {
		t.Fatalf("parse mismatchYAML: %v", err)
	}
	warns := CrossModuleWarnings(doc)
	if len(warns) != 2 {
		t.Fatalf("expected 2 cross-module warnings, got %d: %v", len(warns), warns)
	}
}
