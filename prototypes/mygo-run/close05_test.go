package main

import (
	"testing"

	"asterlab-mygo-prototype/modules/runs"
)

// CLOSE-05: the Run relay plan comes from the frozen design revision the Run
// pinned, never the live draft and never an empty plan.
const close05FrozenDesign = `{"mygo":{"workflow":{"nodes":[` +
	`{"id":"n1","title":"冷热交替","params":[` +
	`{"Key":"计时类型","Val":"等待"},{"Key":"计时时长","Val":"5m"},{"Key":"重复次数","Val":"3"},` +
	`{"Key":"计时类型2","Val":"等待"},{"Key":"计时时长2","Val":"10m"},{"Key":"重复次数2","Val":"3"}]},` +
	`{"id":"n2","title":"未定量"}` +
	`]}}}`

func TestClose05TimerPhasesFromFrozenDesign(t *testing.T) {
	a := &app{snap: runs.RunSnapshot{
		RunID:      "run-1",
		DesignData: close05FrozenDesign,
		Tasks: []runs.RunTask{
			{Key: "root/gel/gel-cycle", DesignStepID: "n1"},
			{Key: "root/other", DesignStepID: "n2"},
		},
	}}
	phases := a.runTimerPhases("root/gel/gel-cycle")
	if len(phases) != 2 {
		t.Fatalf("frozen phases = %+v, want cold+hot", phases)
	}
	if *phases[0].PlannedS != 300 || *phases[0].Rounds != 3 {
		t.Fatalf("cold phase = %+v", phases[0])
	}
	if *phases[1].PlannedS != 600 || *phases[1].Rounds != 3 || phases[1].Name != "冷热交替（二段）" {
		t.Fatalf("hot phase = %+v", phases[1])
	}
	if got := a.runTimerPhases("root/other"); got != nil {
		t.Fatalf("step without timing must yield nil, got %+v", got)
	}
	if got := a.runTimerPhases("missing"); got != nil {
		t.Fatalf("unknown task must yield nil, got %+v", got)
	}
	// The live draft is never consulted: empty DesignData yields nothing even
	// for a known task key.
	b := &app{snap: runs.RunSnapshot{RunID: "run-1", Tasks: []runs.RunTask{{Key: "k", DesignStepID: "n1"}}}}
	if got := b.runTimerPhases("k"); got != nil {
		t.Fatalf("unfrozen run must yield nil, got %+v", got)
	}
}
