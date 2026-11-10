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

func TestAsterDocCLIReadsSameRevision(t *testing.T) {
	dir := copyFixtureDB(t)
	ctx := context.Background()
	svc, err := runs.NewDesignService()
	if err != nil {
		t.Fatal(err)
	}
	defer svc.Close()
	designs, err := svc.ListDesigns(ctx)
	if err != nil || len(designs) == 0 {
		t.Fatalf("designs: %v %v", designs, err)
	}
	designID := designs[0].DesignID
	if _, err := svc.SaveAsterDoc(ctx, runs.AsterDocSaveInput{
		DesignID: designID, Source: storeDoc, ExpectedRevision: 0,
		OperationID: "cli-parity-doc", ActorID: "tester",
	}); err != nil {
		t.Fatal(err)
	}
	cli := "D:/L/AI/Cindy/CellPlanner-ProductStudy/cellplanner-replica/target/release/labworkbench.exe"
	if _, err := os.Stat(cli); err != nil {
		t.Skip("CLI binary unavailable")
	}
	input := filepath.Join(dir, "asterdoc-load.json")
	if err := os.WriteFile(input, []byte(`{"design_id":"`+designID+`"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(cli, "asterdoc.load", "--input", input)
	cmd.Env = append(os.Environ(), "LABWORKBENCH_DATA_DIR="+dir)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("asterdoc.load: %v\n%s", err, out)
	}
	var envelope struct {
		OK     bool `json:"ok"`
		Result struct {
			Revision uint64 `json:"revision"`
			Source   string `json:"source"`
		} `json:"result"`
	}
	if err := json.Unmarshal(out, &envelope); err != nil {
		t.Fatalf("decode: %v\n%s", err, out)
	}
	if !envelope.OK || envelope.Result.Revision != 1 || envelope.Result.Source != storeDoc {
		t.Fatalf("CLI read = %+v", envelope)
	}
}
