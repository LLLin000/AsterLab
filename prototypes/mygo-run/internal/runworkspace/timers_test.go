package runworkspace

import (
	"strings"
	"testing"
	"time"

	"asterlab-mygo-prototype/modules/runs"
)

func timerNow() time.Time {
	return time.Date(2026, 11, 16, 10, 0, 0, 0, time.UTC)
}

func TestDescribeTimerRunningCountdown(t *testing.T) {
	planned := uint64(600)
	started := timerNow().Add(-90 * time.Second).UTC().Format(time.RFC3339)
	display := DescribeTimer(runs.TimerRecord{
		TimerID: "t1", TaskKey: "k", Mode: "countdown",
		PlannedS: &planned, StartedAtUTC: &started,
		AccumulatedS: 30, Status: "running",
	}, timerNow())
	// Checkpoint math only: 30 stored + 90 running, 600 planned.
	if display.ElapsedS != 120 {
		t.Fatalf("elapsed = %d, want 120", display.ElapsedS)
	}
	if display.RemainingS == nil || *display.RemainingS != 480 {
		t.Fatalf("remaining = %+v, want 480", display.RemainingS)
	}
	if display.Over || display.ClockIssue {
		t.Fatalf("unexpected flags: %+v", display)
	}
}

func TestDescribeTimerPausedFreezesAtAccumulation(t *testing.T) {
	planned := uint64(600)
	started := timerNow().Add(-90 * time.Second).UTC().Format(time.RFC3339)
	display := DescribeTimer(runs.TimerRecord{
		Mode: "countdown", PlannedS: &planned, StartedAtUTC: &started,
		AccumulatedS: 30, Status: "paused",
	}, timerNow())
	if display.ElapsedS != 30 {
		t.Fatalf("paused elapsed = %d, want 30", display.ElapsedS)
	}
}

func TestDescribeTimerRunningWithoutAnchorIsFlagged(t *testing.T) {
	display := DescribeTimer(runs.TimerRecord{
		Mode: "countdown", AccumulatedS: 45, Status: "running",
	}, timerNow())
	// No anchor to measure from: fall back to the stored accumulation and
	// flag the gap instead of inventing elapsed time.
	if display.ElapsedS != 45 || !display.ClockIssue {
		t.Fatalf("got %+v, want elapsed=45 with clock issue", display)
	}
}

func TestDescribeTimerOverrunAndStopwatch(t *testing.T) {
	planned := uint64(60)
	display := DescribeTimer(runs.TimerRecord{
		Mode: "countdown", PlannedS: &planned, AccumulatedS: 61, Status: "paused",
	}, timerNow())
	if !display.Over || display.RemainingS == nil || *display.RemainingS != 0 {
		t.Fatalf("overrun not reported: %+v", display)
	}
	stopwatch := DescribeTimer(runs.TimerRecord{
		Mode: "stopwatch", AccumulatedS: 3721, Status: "running",
		StartedAtUTC: new(timerNow().Add(-4 * time.Second).UTC().Format(time.RFC3339)),
	}, timerNow())
	if stopwatch.RemainingS != nil {
		t.Fatalf("stopwatch must plan nothing: %+v", stopwatch)
	}
	if got := FormatHMS(3721); got != "1:02:01" {
		t.Fatalf("FormatHMS(3721) = %q", got)
	}
	if got := FormatHMS(65); got != "1:05" {
		t.Fatalf("FormatHMS(65) = %q", got)
	}
}

func TestWindowLineWaitsForT0(t *testing.T) {
	anchor := "step-04"
	display := DescribeTimer(runs.TimerRecord{
		Mode: "window", Status: "ready", AnchorTaskKey: &anchor, AccumulatedS: 0,
	}, timerNow())
	line := WindowLine(display)
	if !strings.Contains(line, "step-04") || !strings.Contains(line, "t0") {
		t.Fatalf("window line = %q, want t0 wait naming the anchor", line)
	}
	planned := uint64(6 * 3600)
	started := timerNow().Add(-3600 * time.Second).UTC().Format(time.RFC3339)
	running := DescribeTimer(runs.TimerRecord{
		Mode: "window", Status: "running", PlannedS: &planned,
		StartedAtUTC: &started, AccumulatedS: 0,
	}, timerNow())
	if line := WindowLine(running); !strings.Contains(line, "目标") {
		t.Fatalf("running window line = %q, want target clock", line)
	}
}

func TestRoundLabelRelay(t *testing.T) {
	count := uint32(3)
	record := runs.TimerRecord{Mode: "countdown", RoundIndex: 1, RoundCount: &count}
	if got := RoundLabel(record); got != "第 2/3 轮" {
		t.Fatalf("round label = %q", got)
	}
	if got := PhaseName([]runs.TimerPhase{{Name: "冰浴"}}, 0); got != "冰浴" {
		t.Fatalf("phase name = %q", got)
	}
	if got := PhaseName(nil, 0); got != "" {
		t.Fatalf("empty plan must name nothing, got %q", got)
	}
}

func TestParseMinutesStrict(t *testing.T) {
	for _, raw := range []string{"30", " 30 ", "1.5", "0.05", "1e2"} {
		if _, err := parseMinutes(raw); err != nil {
			t.Fatalf("parseMinutes(%q) = %v, want seconds", raw, err)
		}
	}
	if got, err := parseMinutes("1.5"); err != nil || got != 90 {
		t.Fatalf("parseMinutes(1.5) = %d, %v; want 90", got, err)
	}
	if got, err := parseMinutes("0.05"); err != nil || got != 3 {
		t.Fatalf("parseMinutes(0.05) = %d, %v; want 3", got, err)
	}
	// A large but clock-representable plan stays valid; anything beyond
	// what time.Duration can hold as seconds must fail before WindowLine
	// converts it.
	if got, err := parseMinutes("100000000"); err != nil || got != 6000000000 {
		t.Fatalf("parseMinutes(100000000) = %d, %v; want 6000000000", got, err)
	}
	if got, err := parseMinutes("200000000"); err == nil {
		t.Fatalf("parseMinutes(200000000) = %d, want an overflow error", got)
	}
	// Sscanf("%g") accepted a numeric prefix; strconv must reject it.
	for _, raw := range []string{"", "  ", "abc", "5mins", "30 分钟", "NaN", "Inf", "-Inf", "1,5", "0", "-3", "0.001", "1e400"} {
		if got, err := parseMinutes(raw); err == nil {
			t.Fatalf("parseMinutes(%q) = %d, want an error", raw, got)
		}
	}
}

func TestCreateTimerRefusesExistingID(t *testing.T) {
	vm := BuildViewModel(makeSnapshot(), makeFrozenDoc())
	s := NewState(vm, Callbacks{})
	task := vm.TaskByKey["t-read"]
	s.bench().Timers[task.TaskKey] = []runs.TimerRecord{
		{TimerID: task.TaskKey + "/countdown/1", TaskKey: task.TaskKey, Mode: "countdown", Status: "ready"},
	}
	called := false
	s.Callbacks.SaveTimer = func(timer runs.TimerRecord) (runs.TimerRecord, error) {
		called = true
		return timer, nil
	}
	// Core's store is an upsert: saving over the same id would silently reset
	// the timer, so the panel must refuse before the write.
	s.createTimer(task, &timerForm{Mode: "countdown", Minutes: "5"}, 0)
	if called {
		t.Fatal("SaveTimer must not run for a duplicate timer id")
	}
	if msg := s.bench().TimersErr[task.TaskKey]; !strings.Contains(msg, "已存在") {
		t.Fatalf("duplicate error = %q", msg)
	}
}

func TestRunTimerActionFilesCompletedRound(t *testing.T) {
	vm := BuildViewModel(makeSnapshot(), makeFrozenDoc())
	s := NewState(vm, Callbacks{})
	task := vm.TaskByKey["t-read"]
	planned := uint64(300)
	var appended runs.ExecutionEventInput
	appendCalls := 0
	s.Callbacks.TimerAction = func(input runs.TimerActionInput) (runs.TimerUpdateResult, error) {
		if input.Action != "advance" {
			t.Fatalf("action = %q", input.Action)
		}
		if input.NowUTC == "" {
			t.Fatal("the caller must supply the action clock")
		}
		round := input.Timer.RoundIndex
		return runs.TimerUpdateResult{
			Timer:     input.Timer,
			Completed: true,
			Round: runs.TimerRound{
				PhaseIndex: 0, RoundIndex: round, PhaseName: "离心",
				Mode: "countdown", PlannedS: &planned, ActualS: 305,
				StartedAtUTC: "2026-11-16T09:55:00Z", EndedAtUTC: "2026-11-16T10:00:05Z",
				PhaseCompleted: true, TimerCompleted: true,
			},
		}, nil
	}
	s.Callbacks.AppendEvent = func(taskKey string, input runs.ExecutionEventInput) (string, error) {
		appendCalls++
		appended = input
		return "", nil
	}
	s.Callbacks.LoadTimers = func(runID string) ([]runs.TimerRecord, error) { return nil, nil }
	record := runs.TimerRecord{
		TimerID: "t/1", TaskKey: task.TaskKey, Mode: "countdown",
		PlannedS: &planned, Status: "running", RoundIndex: 2,
		StartedAtUTC: new("2026-11-16T09:55:00Z"),
	}
	s.runTimerAction(task, record, nil, "advance")
	if appendCalls != 1 {
		t.Fatalf("completed round filed %d times, want 1", appendCalls)
	}
	if appended.EventType != "observation" || !strings.Contains(appended.Content, "第 3 轮") {
		t.Fatalf("round event = %+v", appended)
	}
	if appended.OccurredAt != "2026-11-16T10:00:05Z" {
		t.Fatalf("round occurred_at = %q", appended.OccurredAt)
	}
}

func TestSkippedTaskNeverCompletes(t *testing.T) {
	if got := buildTaskStatus(runs.RunTask{Status: "skipped"}); got != TaskStatusSkipped {
		t.Fatalf("Core skipped maps to %q, want skipped", got)
	}
	if got := StatusLabel(TaskStatusSkipped); got != "已跳过" {
		t.Fatalf("skipped label = %q", got)
	}
	if canMarkComplete(&RunTaskViewModel{Status: TaskStatusSkipped}) {
		t.Fatal("a skipped step must not offer completion")
	}
	if got := eventTypeLabel("skipped_by_rule"); got != "按规则跳过" {
		t.Fatalf("skip event label = %q", got)
	}
}
