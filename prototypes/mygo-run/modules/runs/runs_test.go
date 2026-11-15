package runs

import (
	"context"
	"math"
	"strings"
	"testing"
)

// newServices opens both native services on an isolated data directory. The
// module reads LABWORKBENCH_DATA_DIR when it opens, so every test pins its own.
func newServices(t *testing.T) (RunService, DesignService) {
	t.Helper()
	t.Setenv("LABWORKBENCH_DATA_DIR", t.TempDir())
	runService, err := NewRunService()
	if err != nil {
		t.Fatalf("NewRunService: %v", err)
	}
	t.Cleanup(func() { _ = runService.Close() })
	designService, err := NewDesignService()
	if err != nil {
		t.Fatalf("NewDesignService: %v", err)
	}
	t.Cleanup(func() { _ = designService.Close() })
	return runService, designService
}

func TestNormalizeRuntimeRecomputesLiveMix(t *testing.T) {
	svc, _ := newServices(t)
	ctx := context.Background()

	over, err := svc.NormalizeRuntime(ctx, RuntimeNormalizeInput{
		PlannedWells:     9,
		ActualWells:      10,
		VolumePerWellUL:  200,
		ExcessRatio:      0.1,
		DeadVolumeUL:     50,
		PreparedVolumeUL: 2030,
	})
	if err != nil {
		t.Fatalf("NormalizeRuntime: %v", err)
	}
	closeEnough(t, "planned volume", over.PlannedVolumeUL, 2030)
	closeEnough(t, "required volume", over.RequiredVolumeUL, 2250)
	closeEnough(t, "additional volume", over.AdditionalVolumeUL, 220)
	closeEnough(t, "unused volume", over.UnusedVolumeUL, 0)

	under, err := svc.NormalizeRuntime(ctx, RuntimeNormalizeInput{
		PlannedWells:     9,
		ActualWells:      7,
		VolumePerWellUL:  200,
		ExcessRatio:      0.1,
		DeadVolumeUL:     50,
		PreparedVolumeUL: 2000,
	})
	if err != nil {
		t.Fatalf("NormalizeRuntime: %v", err)
	}
	closeEnough(t, "required volume", under.RequiredVolumeUL, 1590)
	closeEnough(t, "unused volume", under.UnusedVolumeUL, 410)

	if _, err := svc.NormalizeRuntime(ctx, RuntimeNormalizeInput{
		PlannedWells:    9,
		ActualWells:     9,
		VolumePerWellUL: math.NaN(),
	}); err == nil {
		t.Fatal("expected a non-finite volume to be rejected")
	}
}

func TestPantryLedgerRoundTripAcrossReload(t *testing.T) {
	svc, _ := newServices(t)
	ctx := context.Background()

	shelfLife := uint32(365)
	item, err := svc.UpsertPantry(ctx, PantryUpsert{
		Name:          "Trypsin-EDTA",
		Location:      "fridge-2",
		PurchasedOn:   "2026-01-01",
		OpenedOn:      "2026-06-01",
		ShelfLifeDays: &shelfLife,
	})
	if err != nil {
		t.Fatalf("UpsertPantry: %v", err)
	}
	if item.ID == "" || !strings.HasPrefix(item.LabelCode, "PN-") {
		t.Fatalf("created item identity = %+v", item)
	}
	if item.Revision != 1 {
		t.Fatalf("created revision = %d", item.Revision)
	}

	ledger, err := svc.ListPantry(ctx, PantryQuery{AsOf: "2026-06-25"})
	if err != nil {
		t.Fatalf("ListPantry: %v", err)
	}
	if ledger.AsOf != "2026-06-25" {
		t.Fatalf("ledger as_of = %q", ledger.AsOf)
	}
	if len(ledger.Items) != 1 {
		t.Fatalf("ledger items = %d", len(ledger.Items))
	}
	listed := ledger.Items[0]
	if listed.ID != item.ID || listed.LabelCode != item.LabelCode || listed.Name != "Trypsin-EDTA" {
		t.Fatalf("listed item = %+v", listed)
	}
	if listed.Status != "opened" || listed.ExpiresOn != "2027-01-01" {
		t.Fatalf("evaluation = status %q expires %q", listed.Status, listed.ExpiresOn)
	}

	labels, err := svc.ExportPantryLabels(ctx, PantryQuery{AsOf: "2026-06-25"})
	if err != nil {
		t.Fatalf("ExportPantryLabels: %v", err)
	}
	if labels.SchemaVersion != 1 || len(labels.Labels) != 1 {
		t.Fatalf("label manifest = %+v", labels)
	}
	if labels.Labels[0].PantryItemID != item.ID || labels.Labels[0].LabelCode != item.LabelCode {
		t.Fatalf("label identity = %+v", labels.Labels[0])
	}

	deleted, err := svc.DeletePantry(ctx, PantryDelete{ID: item.ID, ExpectedRevision: &item.Revision})
	if err != nil {
		t.Fatalf("DeletePantry: %v", err)
	}
	if !deleted.Deleted || deleted.LabelCode != item.LabelCode || deleted.Revision != item.Revision+1 {
		t.Fatalf("delete result = %+v", deleted)
	}

	_ = svc.Close()
	reopened, err := NewRunService()
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	t.Cleanup(func() { _ = reopened.Close() })
	after, err := reopened.ListPantry(ctx, PantryQuery{AsOf: "2026-06-25"})
	if err != nil {
		t.Fatalf("ListPantry after reload: %v", err)
	}
	if len(after.Items) != 0 {
		t.Fatalf("retired item is still listed: %+v", after.Items)
	}
	if _, err := reopened.UpsertPantry(ctx, PantryUpsert{ID: item.ID, Name: "Trypsin-EDTA"}); err == nil {
		t.Fatal("a retired item must not be revived")
	}
}

func TestTimerAndRecordProjectionRequireAFrozenRun(t *testing.T) {
	svc, _ := newServices(t)
	ctx := context.Background()
	unknown := "no-such-run"

	planned := uint64(600)
	if _, err := svc.SaveTimer(ctx, TimerRecord{
		TimerID: "timer-1", RunID: unknown, TaskKey: "readout", Mode: "countdown",
		Status: "ready", PlannedS: &planned,
	}); err == nil {
		t.Fatal("a timer must not attach to a missing Run")
	}

	if _, err := svc.RecordProjection(ctx, RecordProjectionInput{RunID: unknown}); err == nil {
		t.Fatal("a projection must not be produced for a missing Run")
	}

	if _, err := svc.SignAndArchiveRun(ctx, SignAndArchiveInput{
		RunID: unknown, SignedBy: "operator", SignedAtUTC: "2026-11-15T10:00:00Z",
	}); err == nil {
		t.Fatal("a missing Run must not be signed and archived")
	}
}

func TestDeriveDesignFromRunRequiresAFrozenRun(t *testing.T) {
	_, designService := newServices(t)
	ctx := context.Background()
	if _, err := designService.DeriveDesignFromRun(ctx, DeriveDesignInput{RunID: "no-such-run"}); err == nil {
		t.Fatal("a missing Run must not derive a Design")
	}
}

func closeEnough(t *testing.T, label string, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 1e-9 {
		t.Fatalf("%s = %v, want %v", label, got, want)
	}
}
