package main

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"asterlab-mygo-prototype/modules/runs"
)

func TestRustModuleReadsAndWritesSQLite(t *testing.T) {
	dir, err := os.MkdirTemp("", "asterlab-mygo-")
	src := "D:/L/AI/Cindy/CellPlanner-ProductStudy/cellplanner-replica/target/asterlab-ui-runtime3/labworkbench.sqlite"
	db, err := os.ReadFile(src)
	if err != nil {
		t.Skipf("fixture DB unavailable: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "labworkbench.sqlite"), db, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("LABWORKBENCH_DATA_DIR", dir)
	svc, err := runs.NewRunService()
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	snap, err := svc.LoadRun(ctx, "1c0c8989-0513-483a-851b-bdc7d6c67483")
	if err != nil {
		t.Fatal(err)
	}
	if snap.RunID == "" || len(snap.Tasks) == 0 {
		t.Fatalf("empty snapshot: %+v", snap)
	}
	_, err = svc.SaveReadout(ctx, runs.ReadoutInput{
		RunID:        "1c0c8989-0513-483a-851b-bdc7d6c67483",
		Measurements: []runs.Measurement{{SampleID: "Control-1", Endpoint: "OD450", Value: 0.82, Unit: "AU"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	again, err := svc.LoadRun(ctx, "1c0c8989-0513-483a-851b-bdc7d6c67483")
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, m := range again.Measurements {
		if m.SampleID == "Control-1" && m.Value == 0.82 {
			found = true
		}
	}
	if !found {
		t.Fatalf("saved measurement not in snapshot: %+v", again.Measurements)
	}

	// The CLI reads the same copied SQLite file through the same command layer.
	cli := "D:/L/AI/Cindy/CellPlanner-ProductStudy/cellplanner-replica/target/release/labworkbench.exe"
	if _, err := os.Stat(cli); err == nil {
		input := filepath.Join(dir, "workspace-input.json")
		_ = os.WriteFile(input, []byte(`{"id":"1c0c8989-0513-483a-851b-bdc7d6c67483"}`), 0o644)
		cmd := exec.Command(cli, "run.workspace", "--input", input)
		cmd.Env = append(os.Environ(), "LABWORKBENCH_DATA_DIR="+dir)
		out, err := cmd.Output()
		if err != nil {
			t.Fatalf("CLI cross-check: %v", err)
		}
		var got struct {
			OK     bool `json:"ok"`
			Result struct {
				Events []struct {
					Measurements []struct {
						SampleID string  `json:"sample_id"`
						Value    float64 `json:"value"`
					} `json:"measurements"`
				} `json:"events"`
			} `json:"result"`
		}
		if err := json.Unmarshal(out, &got); err != nil {
			t.Fatal(err)
		}
		cross := false
		for _, e := range got.Result.Events {
			for _, m := range e.Measurements {
				if m.SampleID == "Control-1" && m.Value == 0.82 {
					cross = true
				}
			}
		}
		if !got.OK || !cross {
			t.Fatalf("CLI did not see MyGo write: %s", out)
		}
	}
}
