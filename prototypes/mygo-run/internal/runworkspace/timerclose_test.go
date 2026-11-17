package runworkspace

import (
	"strings"
	"testing"

	"asterlab-mygo-prototype/modules/runs"
)

// CLOSE-05: same-task same-mode timers carry explicit identity and scope;
// creation is never a silent no-op (QA-HUVEC F1).

func TestTimerIdentity(t *testing.T) {
	cases := []struct {
		id   string
		mode string
		want string
	}{
		{"root/cure/countdown/1", "countdown", "倒计时 #1"},
		{"root/cure/stopwatch/2", "stopwatch", "正计时 #2"},
		{"root/cure/stopwatch/2@5%", "stopwatch", "正计时 #2 · 5%"},
		{"root/cure/countdown/3@3%", "countdown", "倒计时 #3 · 3%"},
		{"legacy-id", "window", "时间窗"},
	}
	for _, tc := range cases {
		got := TimerIdentity(runs.TimerRecord{TimerID: tc.id, Mode: tc.mode})
		if got != tc.want {
			t.Fatalf("TimerIdentity(%q) = %q, want %q", tc.id, got, tc.want)
		}
		if strings.Contains(got, tc.id) && tc.id != "legacy-id" {
			t.Fatalf("identity must not leak the raw id: %q", got)
		}
	}
}

func TestSecondStopwatchKeepsDistinctIdentity(t *testing.T) {
	s, _, saved := benchHarness()
	task := s.VM.TaskByKey["t-read"]
	s.createTimer(task, &timerForm{Mode: "stopwatch"}, 0)
	if len(*saved) != 1 {
		t.Fatalf("first stopwatch not saved: %+v", *saved)
	}
	s.createTimer(task, &timerForm{Mode: "stopwatch", Group: "5%"}, len(*saved))
	if msg := s.bench().TimersErr[task.TaskKey]; msg != "" {
		t.Fatalf("second stopwatch must succeed visibly, got error %q", msg)
	}
	if len(*saved) != 2 {
		t.Fatalf("second stopwatch silently lost: %+v", *saved)
	}
	if (*saved)[0].TimerID == (*saved)[1].TimerID {
		t.Fatalf("same-task stopwatches share one id: %q", (*saved)[0].TimerID)
	}
	if got := TimerIdentity((*saved)[1]); got != "正计时 #2 · 5%" {
		t.Fatalf("second identity = %q", got)
	}
}

func TestCreateTimerWithoutModeFailsVisibly(t *testing.T) {
	s, _, saved := benchHarness()
	task := s.VM.TaskByKey["t-read"]
	s.createTimer(task, &timerForm{}, 0)
	if len(*saved) != 0 {
		t.Fatalf("modeless timer must not save: %+v", *saved)
	}
	if msg := s.bench().TimersErr[task.TaskKey]; !strings.Contains(msg, "计时类型") {
		t.Fatalf("modeless error = %q, want a visible cause", msg)
	}
}

func TestAdvanceBeforeElapsedFilesNoEvent(t *testing.T) {
	// A retried advance that Core reports as not-yet-complete must not file a
	// second fact: restart replays the checkpoint, never the round.
	s, appended, _ := benchHarness()
	calls := 0
	s.Callbacks.TimerAction = func(input runs.TimerActionInput) (runs.TimerUpdateResult, error) {
		return runs.TimerUpdateResult{Timer: input.Timer, Completed: false}, nil
	}
	s.Callbacks.AppendEvent = func(taskKey string, input runs.ExecutionEventInput) (string, error) {
		calls++
		*appended = &input
		return "", nil
	}
	task := s.VM.TaskByKey["t-read"]
	planned := uint64(300)
	record := runs.TimerRecord{TimerID: "t/1", TaskKey: task.TaskKey, Mode: "countdown",
		PlannedS: &planned, Status: "running"}
	s.runTimerAction(task, record, nil, "advance")
	if calls != 0 {
		t.Fatalf("incomplete advance filed %d events, want 0", calls)
	}
	if msg := s.bench().TimersErr[task.TaskKey]; msg != "" {
		t.Fatalf("no-event advance must stay quiet, got %q", msg)
	}
}

func TestRunTimerActionForwardsFrozenPhases(t *testing.T) {
	// Run consumes the frozen relay plan: UpdateTimer must receive the exact
	// phases the caller supplied, never an empty plan in their place.
	s, _, _ := benchHarness()
	var got []runs.TimerPhase
	planned := uint64(300)
	rounds := uint32(3)
	plannedHot := uint64(600)
	roundsHot := uint32(3)
	phases := []runs.TimerPhase{
		{Name: "冷", PlannedS: &planned, Mode: "countdown", Rounds: &rounds},
		{Name: "热", PlannedS: &plannedHot, Mode: "countdown", Rounds: &roundsHot},
	}
	s.Callbacks.TimerAction = func(input runs.TimerActionInput) (runs.TimerUpdateResult, error) {
		got = input.Phases
		return runs.TimerUpdateResult{Timer: input.Timer, Completed: false}, nil
	}
	task := s.VM.TaskByKey["t-read"]
	record := runs.TimerRecord{TimerID: "t/1", TaskKey: task.TaskKey, Mode: "countdown",
		PlannedS: &planned, Status: "running"}
	s.runTimerAction(task, record, phases, "pause")
	if len(got) != 2 || got[0].Name != "冷" || got[1].Name != "热" {
		t.Fatalf("forwarded phases = %+v, want the frozen plan", got)
	}
	if *got[0].Rounds != 3 || *got[1].PlannedS != 600 {
		t.Fatalf("forwarded plan corrupted: %+v", got)
	}
}
