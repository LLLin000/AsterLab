package e2e

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"asterlab-mygo-prototype/modules/runs"
)

func TestTwoReadoutsPersistAndCrossRead(t *testing.T) {
	dir, _ := os.MkdirTemp("", "asterlab-mygo-e2e-")
	defer os.RemoveAll(dir)
	src := "D:/L/AI/Cindy/CellPlanner-ProductStudy/cellplanner-replica/target/asterlab-ui-runtime3/labworkbench.sqlite"
	db, err := os.ReadFile(src)
	if err != nil {
		t.Skipf("fixture DB unavailable: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "labworkbench.sqlite"), db, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("LABWORKBENCH_DATA_DIR", dir)
	const runID = "1c0c8989-0513-483a-851b-bdc7d6c67483"
	svc, err := runs.NewRunService()
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	before, err := svc.LoadRun(ctx, runID)
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.SaveReadout(ctx, runs.ReadoutInput{
		RunID: runID,
		Measurements: []runs.Measurement{
			{SampleID: "Control-1", Endpoint: "OD450", Value: 0.82, Unit: "AU"},
			{SampleID: "E1-1", Endpoint: "OD450", Value: 0.41, Unit: "AU"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	after, err := svc.LoadRun(ctx, runID)
	if err != nil {
		t.Fatal(err)
	}
	if after.Revision <= before.Revision {
		t.Fatalf("revision did not advance: %d -> %d", before.Revision, after.Revision)
	}
	for _, sid := range []string{"Control-1", "E1-1"} {
		found := false
		for _, m := range after.Measurements {
			if m.SampleID == sid {
				found = true
			}
		}
		if !found {
			t.Fatalf("missing %s in snapshot", sid)
		}
	}
	// reopen
	svc2, err := runs.NewRunService()
	if err != nil {
		t.Fatal(err)
	}
	defer svc2.Close()
	reopened, err := svc2.LoadRun(ctx, runID)
	if err != nil {
		t.Fatal(err)
	}
	for _, sid := range []string{"Control-1", "E1-1"} {
		found := false
		for _, m := range reopened.Measurements {
			if m.SampleID == sid {
				found = true
			}
		}
		if !found {
			t.Fatalf("reopened missing %s", sid)
		}
	}
	// CLI cross-read
	cli := "D:/L/AI/Cindy/CellPlanner-ProductStudy/cellplanner-replica/target/release/labworkbench.exe"
	if _, err := os.Stat(cli); err == nil {
		input := filepath.Join(dir, "workspace-input.json")
		_ = os.WriteFile(input, []byte(`{"id":"`+runID+`"}`), 0o644)
		cmd := exec.Command(cli, "run.workspace", "--input", input)
		cmd.Env = append(os.Environ(), "LABWORKBENCH_DATA_DIR="+dir)
		out, err := cmd.Output()
		if err != nil {
			t.Fatalf("CLI cross-check: %v", err)
		}
		var got struct {
			OK bool `json:"ok"`
		}
		if err := json.Unmarshal(out, &got); err != nil || !got.OK {
			t.Fatalf("CLI failed: %s", out)
		}
	}
}
