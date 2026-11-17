package main

// CLOSE-04 integration: the record board and the protocol archive project
// from Core through record.Fetch, and the derived-design review persists
// through SaveDesign. Both run against the real Core on the committed
// RNA-case fixture (copied to a temp dir, never mutated): Fetch decodes the
// corrected-design RNA run, and derive carries the legacy RNA run's measured
// params into reviewable evidence. Every fixture design is AsterDoc-managed,
// so persisting the derived draft is refused naming AsterDoc — the exact
// refusal the board surfaces instead of failing silently.

import (
	"context"
	"strings"
	"testing"
	"time"

	"asterlab-mygo-prototype/internal/notebook/record"
	"asterlab-mygo-prototype/modules/runs"
)

func TestClose04RecordFetchDecodesRNAFixture(t *testing.T) {
	dir := rnaFixtureDataDir(t)
	t.Setenv("LABWORKBENCH_DATA_DIR", dir)
	ctx := context.Background()
	rs, err := runs.NewRunService()
	if err != nil {
		t.Fatal(err)
	}
	defer rs.Close()

	proj, err := record.Fetch(ctx, rs, record.FetchInput{
		RunID:  "357c2f94-d5bf-4002-951e-f84c73e2d0b7",
		NowUTC: "2026-11-16T12:00:00Z",
	})
	if err != nil {
		t.Fatalf("fetch decode: %v", err)
	}
	if proj.SchemaVersion != 1 || proj.RunID != "357c2f94-d5bf-4002-951e-f84c73e2d0b7" {
		t.Fatalf("projection identity = v%d %q", proj.SchemaVersion, proj.RunID)
	}
	if proj.Design.DesignID != "d2d393a8-9af4-452c-beba-c3687503ffee" || proj.Design.DesignRevision != 3 {
		t.Fatalf("frozen design = %s rev %d", proj.Design.DesignID, proj.Design.DesignRevision)
	}
	if len(proj.Parameters) != 16 {
		t.Fatalf("parameters = %d, want 16", len(proj.Parameters))
	}
	if len(proj.Readings) != 8 {
		t.Fatalf("readings = %d, want 8", len(proj.Readings))
	}
	if proj.Archive.Status != "recording" {
		t.Fatalf("archive = %q, want recording", proj.Archive.Status)
	}
	counts := record.Summarize(proj, record.SnapshotTasks([]runs.RunTask{}))
	if !strings.Contains(record.Headline(counts), "实测 8 个") {
		t.Fatalf("headline = %q", record.Headline(counts))
	}
}

func TestClose04LegacyDeriveEvidenceAndAsterDocRefusal(t *testing.T) {
	dir := rnaFixtureDataDir(t)
	t.Setenv("LABWORKBENCH_DATA_DIR", dir)
	ctx := context.Background()
	ds, err := runs.NewDesignService()
	if err != nil {
		t.Fatal(err)
	}
	defer ds.Close()

	derived, err := ds.DeriveDesignFromRun(ctx, runs.DeriveDesignInput{
		RunID: "ee49d439-b82a-4e99-8edf-c7f39208330d",
	})
	if err != nil {
		t.Fatalf("derive: %v", err)
	}
	if derived.DesignID != "d2d393a8-9af4-452c-beba-c3687503ffee" || derived.SourceDesignRevision != 2 {
		t.Fatalf("derived = %s srcRev %d", derived.DesignID, derived.SourceDesignRevision)
	}
	ev, err := record.DecodeDerived(derived)
	if err != nil {
		t.Fatalf("decode derived: %v", err)
	}
	if len(ev.Params) != 16 {
		t.Fatalf("derived params = %d, want 16", len(ev.Params))
	}
	snap, err := ds.LoadDesign(ctx, derived.DesignID)
	if err != nil {
		t.Fatal(err)
	}
	changes, err := record.ChangesForDerived(snap.Data, derived, ev, time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("shape changes: %v", err)
	}
	_, err = ds.SaveDesign(ctx, runs.DesignSaveInput{
		DesignID: derived.DesignID, ExpectedRevision: uint64(derived.ExpectedRevision),
		Changes: changes, OperationID: "close04-derive-probe",
	})
	if err == nil {
		t.Fatal("SaveDesign on an AsterDoc-managed design succeeded")
	}
	if !strings.Contains(err.Error(), "AsterDoc") {
		t.Fatalf("refusal does not name AsterDoc: %v", err)
	}
}
