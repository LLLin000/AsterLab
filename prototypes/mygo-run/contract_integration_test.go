package main

import (
	"context"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"asterlab-mygo-prototype/modules/runs"
)

const contractRunID = "1c0c8989-0513-483a-851b-bdc7d6c67483"

func copyFixtureDB(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "asterlab-mygo-contract-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	src := "D:/L/AI/Cindy/CellPlanner-ProductStudy/cellplanner-replica/target/asterlab-ui-runtime3/labworkbench.sqlite"
	db, err := os.ReadFile(src)
	if err != nil {
		t.Skipf("fixture DB unavailable: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "labworkbench.sqlite"), db, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("LABWORKBENCH_DATA_DIR", dir)
	return dir
}

func TestContractSameSQLiteCopy(t *testing.T) {
	dir := copyFixtureDB(t)
	ctx := context.Background()
	svc, err := runs.NewRunService()
	if err != nil {
		t.Fatal(err)
	}
	defer svc.Close()
	before, err := svc.LoadRun(ctx, contractRunID)
	if err != nil {
		t.Fatal(err)
	}
	for _, sid := range []string{"Control-1", "E1-1"} {
		_, err = svc.SaveReadout(ctx, runs.ReadoutInput{
			RunID:        contractRunID,
			Measurements: []runs.Measurement{{SampleID: sid, Endpoint: "OD450", Value: 0.82, Unit: "AU"}},
			OperationID:  "op-" + sid,
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	after, err := svc.LoadRun(ctx, contractRunID)
	if err != nil {
		t.Fatal(err)
	}
	if after.Revision <= before.Revision {
		t.Fatalf("revision did not advance: %d -> %d", before.Revision, after.Revision)
	}
	svc2, err := runs.NewRunService()
	if err != nil {
		t.Fatal(err)
	}
	defer svc2.Close()
	reopened, err := svc2.LoadRun(ctx, contractRunID)
	if err != nil {
		t.Fatal(err)
	}
	if reopened.Revision != after.Revision {
		t.Fatalf("reopen revision mismatch: %d vs %d", reopened.Revision, after.Revision)
	}
	cli := "D:/L/AI/Cindy/CellPlanner-ProductStudy/cellplanner-replica/target/release/labworkbench.exe"
	if _, err := os.Stat(cli); err == nil {
		input := filepath.Join(dir, "workspace-input.json")
		_ = os.WriteFile(input, []byte(`{"id":"`+contractRunID+`"}`), 0o644)
		cmd := exec.Command(cli, "run.workspace", "--input", input)
		cmd.Env = append(os.Environ(), "LABWORKBENCH_DATA_DIR="+dir)
		out, err := cmd.Output()
		if err != nil {
			t.Fatalf("CLI cross-check: %v\n%s", err, out)
		}
		if !strings.Contains(string(out), "OD450") {
			t.Fatalf("CLI cross-read missing OD450: %s", string(out))
		}
	}
}

func TestContractOperationIDIdempotency(t *testing.T) {
	copyFixtureDB(t)
	ctx := context.Background()
	svc, err := runs.NewRunService()
	if err != nil {
		t.Fatal(err)
	}
	defer svc.Close()
	input := runs.ReadoutInput{RunID: contractRunID,
		Measurements: []runs.Measurement{{SampleID: "Control-1", Endpoint: "OD450", Value: 0.82, Unit: "AU"}},
		OperationID:  "op-idem"}
	r1, err := svc.SaveReadout(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	r2, err := svc.SaveReadout(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	if r2.Revision != r1.Revision {
		t.Fatalf("duplicate op should not advance revision: %d -> %d", r1.Revision, r2.Revision)
	}
	bad := runs.ReadoutInput{RunID: contractRunID,
		Measurements: []runs.Measurement{{SampleID: "Control-1", Endpoint: "OD450", Value: 0.41, Unit: "AU"}},
		OperationID:  "op-idem"}
	if _, err := svc.SaveReadout(ctx, bad); err == nil {
		t.Fatal("expected conflict for different payload")
	}
}

func TestContractRollbackRevisionUnchanged(t *testing.T) {
	copyFixtureDB(t)
	ctx := context.Background()
	svc, err := runs.NewRunService()
	if err != nil {
		t.Fatal(err)
	}
	defer svc.Close()
	before, _ := svc.LoadRun(ctx, contractRunID)
	_, err = svc.SaveReadout(ctx, runs.ReadoutInput{
		RunID: contractRunID, Measurements: []runs.Measurement{{SampleID: "Control-1", Endpoint: "OD450", Value: math.NaN(), Unit: "AU"}},
	})
	if err == nil {
		t.Fatal("expected invalid readout error")
	}
	after, _ := svc.LoadRun(ctx, contractRunID)
	if after.Revision != before.Revision {
		t.Fatalf("rollback failed: %d -> %d", before.Revision, after.Revision)
	}
}

func TestContractDesignCRUD(t *testing.T) {
	copyFixtureDB(t)
	ctx := context.Background()
	dsvc, err := runs.NewDesignService()
	if err != nil {
		t.Fatal(err)
	}
	defer dsvc.Close()
	designs, err := dsvc.ListDesigns(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(designs) == 0 {
		t.Skip("no designs in fixture")
	}
	d := designs[0]
	cur, err := dsvc.LoadDesign(ctx, d.DesignID)
	if err != nil {
		t.Fatal(err)
	}
	saved, err := dsvc.SaveDesign(ctx, runs.DesignSaveInput{
		DesignID: d.DesignID, ExpectedRevision: cur.Revision, Changes: `{"title":"renamed"}`, OperationID: "op-design",
	})
	if err != nil {
		t.Fatal(err)
	}
	if saved.Revision <= cur.Revision {
		t.Fatalf("design revision did not advance: %d -> %d", cur.Revision, saved.Revision)
	}
	_, err = dsvc.SaveDesign(ctx, runs.DesignSaveInput{
		DesignID: d.DesignID, ExpectedRevision: cur.Revision, Changes: `{"title":"stale"}`, OperationID: "op-design-stale",
	})
	if err == nil {
		t.Fatal("expected stale design save rejected")
	}
}
