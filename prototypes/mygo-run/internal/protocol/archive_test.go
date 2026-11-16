package protocol

import (
	"strings"
	"testing"
	"time"

	"asterlab-mygo-prototype/internal/notebook/record"
	"asterlab-mygo-prototype/modules/runs"
)

func archiveFixture() (record.RecordProjection, string) {
	snap := runs.RunSnapshot{
		RunID: "run-1", Title: "HUVEC 成管",
		DesignID: "design-1", DesignRevision: 3, Status: "completed",
		Tasks: []runs.RunTask{
			{
				Key: "task-cure", Title: "405nm 固化", Timepoint: "0h",
				Status: "completed", DesignStepID: "step-2",
				ParametersJSON: `[{"key":"cure_s","label":"固化秒数","value":60,"unit":"s"}]`,
			},
		},
		Events: []runs.RunEvent{
			{
				ID: "evt-cure", EventType: "operation", TaskKey: "task-cure",
				Timepoint: "0h", OccurredAt: "2026-11-10T10:00:00+08:00",
				RecordedAt: "2026-11-10T10:01:00+08:00",
				DataJSON:   `{"actual_parameters":[{"key":"cure_s","value":75,"unit":"s"}]}`,
			},
			{
				ID: "evt-read", EventType: "measurement", TaskKey: "task-cure",
				Timepoint: "6h", BatchID: "batch-1",
				OccurredAt: "2026-11-10T16:00:00+08:00", RecordedAt: "2026-11-10T16:01:00+08:00",
				ResultCells: []runs.ResultCell{
					{Row: 0, Column: 0, SampleID: "S1", Field: "OD450", Value: new(0.82)},
				},
			},
		},
	}
	assets := []runs.Asset{
		{ID: "img-1", RelativePath: "run-1/6h/S1-fov1.png", MediaType: "image",
			Link: runs.AssetLink{RunID: "run-1", TaskKey: "task-cure", SampleID: "S1"}},
	}
	proj := record.Project(snap, assets)
	return proj, "# 研判\n\n结论见对比 @matrix(6h)\n"
}

func TestBuildArchiveFlattensRefs(t *testing.T) {
	proj, prose := archiveFixture()
	arch := BuildArchive(proj, prose, time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC))
	if arch.Sealed {
		t.Fatal("unsigned prose must not read sealed")
	}
	if len(arch.SourceRefs) != 1 || arch.SourceRefs[0].Raw != "@matrix(6h)" {
		t.Fatalf("source refs = %+v", arch.SourceRefs)
	}
	doc := arch.Markdown()
	for _, want := range []string{
		"# 实验记录归档", "run-1", "设计：design-1 · 冻结 rev 3",
		"计划 vs 实际", "405nm 固化", "代表视野矩阵", "研判记录",
		"派生设计参数", "cure_s", "未签核",
	} {
		if !strings.Contains(doc, want) {
			t.Fatalf("archive missing %q:\n%s", want, doc)
		}
	}
	if strings.Contains(doc, "@matrix") {
		t.Fatal("dynamic reference leaked into the archive")
	}
}

func TestBuildArchiveKeepsSignature(t *testing.T) {
	proj, prose := archiveFixture()
	signed, sig := record.Sign(prose, "run-1", proj.DesignRevision, "lin",
		time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC))
	arch := BuildArchive(proj, signed, time.Date(2026, 11, 16, 13, 0, 0, 0, time.UTC))
	if !arch.Sealed || arch.Signature == nil || arch.Signature.Hash != sig.Hash {
		t.Fatalf("sealed = %v sig = %+v", arch.Sealed, arch.Signature)
	}
	doc := arch.Markdown()
	if !strings.Contains(doc, "lin") || strings.Contains(doc, "record-sign") {
		t.Fatal("signature block leaked or missing from the archive")
	}
}
