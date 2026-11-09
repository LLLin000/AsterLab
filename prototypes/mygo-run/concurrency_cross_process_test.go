package main

import (
	"asterlab-mygo-prototype/modules/runs"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

const concurrencyFixtureDB = "D:/L/AI/Cindy/CellPlanner-ProductStudy/cellplanner-replica/target/asterlab-ui-runtime3/labworkbench.sqlite"
const concurrencyContractRunID = "1c0c8989-0513-483a-851b-bdc7d6c67483"

type workerResult struct {
	OK        bool   `json:"ok"`
	Error     string `json:"error"`
	RunID     string `json:"runId"`
	Revision  uint64 `json:"revision"`
	BeforeRev uint64 `json:"beforeRev"`
}

func writeWorkerResult(path string, r workerResult) {
	b, _ := json.Marshal(r)
	_ = os.WriteFile(path, b, 0o644)
}

func TestConcurrencyWorker(t *testing.T) {
	if os.Getenv("ASTERLAB_CONCURRENCY_WORKER") != "1" {
		t.Skip("worker entrypoint only")
	}
	dir := os.Getenv("LABWORKBENCH_DATA_DIR")
	if dir == "" {
		t.Fatal("missing LABWORKBENCH_DATA_DIR")
	}
	resultPath := os.Getenv("ASTERLAB_RESULT")
	r := workerResult{}
	defer func() { writeWorkerResult(resultPath, r) }()

	action := os.Getenv("ASTERLAB_ACTION")
	switch action {
	case "save_readout":
		runID := os.Getenv("ASTERLAB_RUN_ID")
		op := os.Getenv("ASTERLAB_OP")
		sample := os.Getenv("ASTERLAB_SAMPLE")
		value, _ := strconv.ParseFloat(os.Getenv("ASTERLAB_VALUE"), 64)
		delay, _ := strconv.Atoi(os.Getenv("ASTERLAB_DELAY_MS"))
		if delay > 0 {
			time.Sleep(time.Duration(delay) * time.Millisecond)
		}
		svc, err := runs.NewRunService()
		if err != nil {
			r.Error = err.Error()
			return
		}
		defer svc.Close()
		ctx := context.Background()
		before, err := svc.LoadRun(ctx, runID)
		if err != nil {
			r.Error = err.Error()
			return
		}
		r.BeforeRev = before.Revision
		snap, err := svc.SaveReadout(ctx, runs.ReadoutInput{
			RunID:        runID,
			Measurements: []runs.Measurement{{SampleID: sample, Endpoint: "OD450", Value: value, Unit: "AU"}},
			OperationID:  op,
		})
		if err != nil {
			r.Error = err.Error()
			return
		}
		r.OK = true
		r.RunID = snap.RunID
		r.Revision = snap.Revision
	case "create_run":
		op := os.Getenv("ASTERLAB_OP")
		svc, err := runs.NewDesignService()
		if err != nil {
			r.Error = err.Error()
			return
		}
		defer svc.Close()
		ctx := context.Background()
		designs, err := svc.ListDesigns(ctx)
		if err != nil || len(designs) == 0 {
			if err != nil {
				r.Error = err.Error()
			} else {
				r.Error = "no designs"
			}
			return
		}
		delay, _ := strconv.Atoi(os.Getenv("ASTERLAB_DELAY_MS"))
		if delay > 0 {
			time.Sleep(time.Duration(delay) * time.Millisecond)
		}
		snap, err := svc.CreateRun(ctx, designs[0].DesignID, op)
		if err != nil {
			r.Error = err.Error()
			return
		}
		r.OK = true
		r.RunID = snap.RunID
		r.Revision = snap.Revision
	case "invalid_readout":
		runID := os.Getenv("ASTERLAB_RUN_ID")
		svc, err := runs.NewRunService()
		if err != nil {
			r.Error = err.Error()
			return
		}
		defer svc.Close()
		ctx := context.Background()
		before, err := svc.LoadRun(ctx, runID)
		if err != nil {
			r.Error = err.Error()
			return
		}
		r.BeforeRev = before.Revision
		_, err = svc.SaveReadout(ctx, runs.ReadoutInput{
			RunID: runID,
		})
		if err == nil {
			r.Error = "expected error"
			return
		}
		r.OK = true
		r.Error = err.Error()
		after, lerr := svc.LoadRun(ctx, runID)
		if lerr != nil {
			r.Error = lerr.Error()
			return
		}
		r.Revision = after.Revision
		return
	}
}

func runWorker(t *testing.T, bin string, env map[string]string) (workerResult, error) {
	t.Helper()
	resPath := filepath.Join(t.TempDir(), "result.json")
	e := os.Environ()
	for k, v := range env {
		e = append(e, k+"="+v)
	}
	e = append(e, "ASTERLAB_CONCURRENCY_WORKER=1")
	e = append(e, "ASTERLAB_RESULT="+resPath)
	cmd := exec.Command("go", "test", "-run=TestConcurrencyWorker", "-v", ".")
	cmd.Dir = "."
	cmd.Env = e
	out, err := cmd.CombinedOutput()
	b, rerr := os.ReadFile(resPath)
	if rerr != nil {
		return workerResult{}, fmt.Errorf("worker result missing: %v\noutput: %s", rerr, string(out))
	}
	var r workerResult
	if uerr := json.Unmarshal(b, &r); uerr != nil {
		return workerResult{}, fmt.Errorf("bad worker result: %v\n%s", uerr, string(out))
	}
	return r, err
}
func buildWorkerBinary(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "concurrency-worker.exe")
	cmd := exec.Command("go", "test", "-c", "-o", bin, ".")
	cmd.Dir = "."
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build worker: %v\n%s", err, out)
	}
	return bin
}

func copyConcurrencyFixture(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "asterlab-mygo-concurrency-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	db, err := os.ReadFile(concurrencyFixtureDB)
	if err != nil {
		t.Skipf("fixture DB unavailable: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "labworkbench.sqlite"), db, 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestCrossProcessConcurrentReadouts(t *testing.T) {
	dir := copyConcurrencyFixture(t)
	bin := buildWorkerBinary(t)
	runID := concurrencyContractRunID
	// baseline revision
	t.Setenv("LABWORKBENCH_DATA_DIR", dir)
	svc, err := runs.NewRunService()
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	before, err := svc.LoadRun(ctx, runID)
	if err != nil {
		svc.Close()
		t.Fatal(err)
	}
	svc.Close()

	envA := map[string]string{
		"LABWORKBENCH_DATA_DIR": dir,
		"ASTERLAB_ACTION":       "save_readout",
		"ASTERLAB_RUN_ID":       runID,
		"ASTERLAB_OP":           "op-conc-a",
		"ASTERLAB_SAMPLE":       "Control-1",
		"ASTERLAB_VALUE":        "0.82",
		"ASTERLAB_DELAY_MS":     "100",
	}
	envB := map[string]string{
		"LABWORKBENCH_DATA_DIR": dir,
		"ASTERLAB_ACTION":       "save_readout",
		"ASTERLAB_RUN_ID":       runID,
		"ASTERLAB_OP":           "op-conc-b",
		"ASTERLAB_SAMPLE":       "E1-1",
		"ASTERLAB_VALUE":        "0.41",
		"ASTERLAB_DELAY_MS":     "100",
	}

	type res struct {
		name string
		r    workerResult
		err  error
	}
	results := make(chan res, 2)
	go func() { r, err := runWorker(t, bin, envA); results <- res{"A", r, err} }()
	go func() { r, err := runWorker(t, bin, envB); results <- res{"B", r, err} }()
	r1 := <-results
	r2 := <-results
	if r1.err != nil {
		t.Fatalf("worker %s exec: %v", r1.name, r1.err)
	}
	if r2.err != nil {
		t.Fatalf("worker %s exec: %v", r2.name, r2.err)
	}
	if !r1.r.OK {
		t.Fatalf("worker %s failed: %s", r1.name, r1.r.Error)
	}
	if !r2.r.OK {
		t.Fatalf("worker %s failed: %s", r2.name, r2.r.Error)
	}

	svc4, err := runs.NewRunService()
	if err != nil {
		t.Fatal(err)
	}
	defer svc4.Close()
	after, err := svc4.LoadRun(ctx, runID)
	if err != nil {
		t.Fatal(err)
	}
	if after.Revision <= before.Revision {
		t.Fatalf("revision did not advance: %d -> %d", before.Revision, after.Revision)
	}
	has := map[string]bool{}
	for _, m := range after.Measurements {
		has[m.SampleID] = true
	}
	if !has["Control-1"] || !has["E1-1"] {
		t.Fatalf("lost update: measurements=%v", after.Measurements)
	}
	seen := map[string]bool{}
	for _, e := range after.Events {
		if e.ID == "" {
			continue
		}
		if seen[e.ID] {
			t.Fatalf("duplicate event id %s", e.ID)
		}
		seen[e.ID] = true
	}
}

func TestCrossProcessDuplicateOperation(t *testing.T) {
	dir := copyConcurrencyFixture(t)
	bin := buildWorkerBinary(t)
	runID := concurrencyContractRunID
	t.Setenv("LABWORKBENCH_DATA_DIR", dir)
	baseSvc, err := runs.NewRunService()
	if err != nil {
		t.Fatal(err)
	}
	before, err := baseSvc.LoadRun(context.Background(), runID)
	if err != nil {
		baseSvc.Close()
		t.Fatal(err)
	}
	baseSvc.Close()
	envA := map[string]string{
		"LABWORKBENCH_DATA_DIR": dir,
		"ASTERLAB_ACTION":       "save_readout",
		"ASTERLAB_RUN_ID":       runID,
		"ASTERLAB_OP":           "op-dup-cross",
		"ASTERLAB_SAMPLE":       "Control-1",
		"ASTERLAB_VALUE":        "0.82",
		"ASTERLAB_DELAY_MS":     "100",
	}
	envB := map[string]string{
		"LABWORKBENCH_DATA_DIR": dir,
		"ASTERLAB_ACTION":       "save_readout",
		"ASTERLAB_RUN_ID":       runID,
		"ASTERLAB_OP":           "op-dup-cross",
		"ASTERLAB_SAMPLE":       "Control-1",
		"ASTERLAB_VALUE":        "0.82",
		"ASTERLAB_DELAY_MS":     "100",
	}
	type res struct {
		name string
		r    workerResult
		err  error
	}
	results := make(chan res, 2)
	go func() { r, err := runWorker(t, bin, envA); results <- res{"A", r, err} }()
	go func() { r, err := runWorker(t, bin, envB); results <- res{"B", r, err} }()
	r1 := <-results
	r2 := <-results
	if r1.err != nil {
		t.Fatalf("worker %s exec: %v", r1.name, r1.err)
	}
	if r2.err != nil {
		t.Fatalf("worker %s exec: %v", r2.name, r2.err)
	}
	if !r1.r.OK || !r2.r.OK {
		t.Fatalf("duplicate op workers failed: A=%s B=%s", r1.r.Error, r2.r.Error)
	}
	// final state
	t.Setenv("LABWORKBENCH_DATA_DIR", dir)
	svc, err := runs.NewRunService()
	if err != nil {
		t.Fatal(err)
	}
	defer svc.Close()
	ctx := context.Background()
	after, err := svc.LoadRun(ctx, runID)
	if err != nil {
		t.Fatal(err)
	}
	if after.Revision != before.Revision+1 {
		t.Fatalf("duplicate operation advanced revision more than once: %d -> %d", before.Revision, after.Revision)
	}
	if len(after.Events) != len(before.Events)+1 {
		t.Fatalf("duplicate operation event count: %d -> %d", len(before.Events), len(after.Events))
	}
}

func TestCrossProcessConflictOperation(t *testing.T) {
	dir := copyConcurrencyFixture(t)
	bin := buildWorkerBinary(t)
	runID := concurrencyContractRunID
	t.Setenv("LABWORKBENCH_DATA_DIR", dir)
	baseSvc, err := runs.NewRunService()
	if err != nil {
		t.Fatal(err)
	}
	before, err := baseSvc.LoadRun(context.Background(), runID)
	if err != nil {
		baseSvc.Close()
		t.Fatal(err)
	}
	baseSvc.Close()
	envA := map[string]string{
		"LABWORKBENCH_DATA_DIR": dir,
		"ASTERLAB_ACTION":       "save_readout",
		"ASTERLAB_RUN_ID":       runID,
		"ASTERLAB_OP":           "op-conflict-cross",
		"ASTERLAB_SAMPLE":       "Control-1",
		"ASTERLAB_VALUE":        "0.82",
		"ASTERLAB_DELAY_MS":     "0",
	}
	envB := map[string]string{
		"LABWORKBENCH_DATA_DIR": dir,
		"ASTERLAB_ACTION":       "save_readout",
		"ASTERLAB_RUN_ID":       runID,
		"ASTERLAB_OP":           "op-conflict-cross",
		"ASTERLAB_SAMPLE":       "Control-1",
		"ASTERLAB_VALUE":        "0.41",
		"ASTERLAB_DELAY_MS":     "0",
	}
	rA, errA := runWorker(t, bin, envA)
	rB, errB := runWorker(t, bin, envB)
	if errA != nil {
		t.Fatalf("worker A failed to run: %v", errA)
	}
	if errB != nil {
		t.Fatalf("worker B failed to run: %v", errB)
	}
	if !rA.OK && !rB.OK {
		t.Fatalf("both workers failed: A=%s B=%s", rA.Error, rB.Error)
	}
	if rA.OK && rB.OK {
		t.Fatal("different payloads with same operation ID both succeeded")
	}
	finalSvc, err := runs.NewRunService()
	if err != nil {
		t.Fatal(err)
	}
	defer finalSvc.Close()
	after, err := finalSvc.LoadRun(context.Background(), runID)
	if err != nil {
		t.Fatal(err)
	}
	if after.Revision != before.Revision+1 || len(after.Events) != len(before.Events)+1 {
		t.Fatalf("conflict left multiple commits: revision %d -> %d, events %d -> %d", before.Revision, after.Revision, len(before.Events), len(after.Events))
	}
	if rA.OK == (rA.Error != "") || rB.OK == (rB.Error != "") {
		t.Fatalf("worker success/error mismatch: A=%+v B=%+v", rA, rB)
	}
}

func TestCrossProcessInvalidRollback(t *testing.T) {
	dir := copyConcurrencyFixture(t)
	bin := buildWorkerBinary(t)
	runID := concurrencyContractRunID
	// valid writer
	envValid := map[string]string{
		"LABWORKBENCH_DATA_DIR": dir,
		"ASTERLAB_ACTION":       "save_readout",
		"ASTERLAB_RUN_ID":       runID,
		"ASTERLAB_OP":           "op-valid-cross",
		"ASTERLAB_SAMPLE":       "Control-1",
		"ASTERLAB_VALUE":        "0.82",
		"ASTERLAB_DELAY_MS":     "100",
	}
	envInvalid := map[string]string{
		"LABWORKBENCH_DATA_DIR": dir,
		"ASTERLAB_ACTION":       "invalid_readout",
		"ASTERLAB_RUN_ID":       runID,
		"ASTERLAB_DELAY_MS":     "100",
	}
	type res struct {
		name string
		r    workerResult
		err  error
	}
	results := make(chan res, 2)
	go func() { r, err := runWorker(t, bin, envValid); results <- res{"valid", r, err} }()
	go func() { r, err := runWorker(t, bin, envInvalid); results <- res{"invalid", r, err} }()
	rV := <-results
	rI := <-results
	if rV.err != nil {
		t.Fatalf("worker %s exec: %v", rV.name, rV.err)
	}
	if rI.err != nil {
		t.Fatalf("worker %s exec: %v", rI.name, rI.err)
	}
	// identify by OK/error? valid should OK true
	var validRes, invalidRes workerResult
	if rV.name == "valid" {
		validRes, invalidRes = rV.r, rI.r
	} else {
		validRes, invalidRes = rI.r, rV.r
	}
	if !validRes.OK {
		t.Fatalf("valid worker failed: %s", validRes.Error)
	}
	if !invalidRes.OK {
		t.Fatalf("invalid worker did not observe rejection: %s", invalidRes.Error)
	}
	if invalidRes.Revision != invalidRes.BeforeRev {
		t.Fatalf("invalid transaction partially committed: %d -> %d", invalidRes.BeforeRev, invalidRes.Revision)
	}
	t.Setenv("LABWORKBENCH_DATA_DIR", dir)
	svc, err := runs.NewRunService()
	if err != nil {
		t.Fatal(err)
	}
	defer svc.Close()
	ctx := context.Background()
	after, err := svc.LoadRun(ctx, runID)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, m := range after.Measurements {
		if m.SampleID == "Control-1" && m.Value == 0.82 {
			found = true
		}
	}
	if !found {
		t.Fatalf("valid write lost after invalid failure: %v", after.Measurements)
	}
}

func TestCrossProcessCreateRunDuplicate(t *testing.T) {
	dir := copyConcurrencyFixture(t)
	bin := buildWorkerBinary(t)
	envA := map[string]string{
		"LABWORKBENCH_DATA_DIR": dir,
		"ASTERLAB_ACTION":       "create_run",
		"ASTERLAB_OP":           "op-create-run-cross",
		"ASTERLAB_DELAY_MS":     "100",
	}
	envB := map[string]string{
		"LABWORKBENCH_DATA_DIR": dir,
		"ASTERLAB_ACTION":       "create_run",
		"ASTERLAB_OP":           "op-create-run-cross",
		"ASTERLAB_DELAY_MS":     "100",
	}
	type res struct {
		name string
		r    workerResult
		err  error
	}
	results := make(chan res, 2)
	go func() { r, err := runWorker(t, bin, envA); results <- res{"A", r, err} }()
	go func() { r, err := runWorker(t, bin, envB); results <- res{"B", r, err} }()
	r1 := <-results
	r2 := <-results
	if r1.err != nil {
		t.Fatalf("worker %s exec: %v", r1.name, r1.err)
	}
	if r2.err != nil {
		t.Fatalf("worker %s exec: %v", r2.name, r2.err)
	}
	if !r1.r.OK || !r2.r.OK {
		t.Fatalf("create run workers failed: A=%s B=%s", r1.r.Error, r2.r.Error)
	}
	if r1.r.RunID == "" || r2.r.RunID != r1.r.RunID {
		t.Fatalf("same operation created different Runs: %s vs %s", r1.r.RunID, r2.r.RunID)
	}
}
