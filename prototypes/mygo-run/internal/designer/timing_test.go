package designer

import (
	"encoding/json"
	"strings"
	"testing"

	"asterlab-mygo-prototype/internal/workflow"
)

func timingParams(pairs ...string) []workflow.NodeParam {
	out := []workflow.NodeParam{}
	for i := 0; i+1 < len(pairs); i += 2 {
		out = append(out, workflow.NodeParam{Key: pairs[i], Val: pairs[i+1]})
	}
	return out
}

func TestParseDurationS(t *testing.T) {
	cases := []struct {
		raw  string
		want float64
		ok   bool
	}{
		{"5m", 300, true},
		{"10min", 600, true},
		{"1.5h", 5400, true},
		{"30s", 30, true},
		{"30", 1800, true},
		{"2h", 7200, true},
		{"1d", 86400, true},
		{"45分钟", 2700, true},
		{"", 0, false},
		{"abc", 0, false},
		{"-5m", 0, false},
		{"0", 0, false},
		{"5x", 0, false},
	}
	for _, tc := range cases {
		got, ok := ParseDurationS(tc.raw)
		if ok != tc.ok || (ok && got != tc.want) {
			t.Fatalf("ParseDurationS(%q) = %v,%v want %v,%v", tc.raw, got, ok, tc.want, tc.ok)
		}
	}
}

func TestParseStepTiming(t *testing.T) {
	first, second := ParseStepTiming(nil)
	if first.HasTiming || second.HasTiming {
		t.Fatal("empty params must declare no timing")
	}
	first, _ = ParseStepTiming(timingParams(TimingKindKey, TimingHandsOn, TimingDurKey, "45m"))
	if !first.HasTiming || first.Kind != TimingHandsOn || first.DurationS != 2700 || first.Repeat != 1 {
		t.Fatalf("primary timing = %+v", first)
	}
	first, second = ParseStepTiming(timingParams(
		TimingKindKey, TimingWait, TimingDurKey, "5m", TimingRepeatKey, "3",
		TimingKind2Key, TimingWait, TimingDur2Key, "10m", TimingRepeat2Key, "3",
	))
	if !first.HasTiming || first.Repeat != 3 || !second.HasTiming || second.DurationS != 600 || second.Repeat != 3 {
		t.Fatalf("alternating timing = %+v / %+v", first, second)
	}
	bad, _ := ParseStepTiming(timingParams(TimingKindKey, "微波", TimingDurKey, "5m"))
	if bad.Err == "" {
		t.Fatalf("unknown kind must carry a cause, got %+v", bad)
	}
	bad, _ = ParseStepTiming(timingParams(TimingDurKey, "soon"))
	if bad.Err == "" {
		t.Fatalf("unparseable duration must carry a cause, got %+v", bad)
	}
	// A kind alone declares nothing: no duration, no gap-fill.
	kindOnly, _ := ParseStepTiming(timingParams(TimingKindKey, TimingWait))
	if kindOnly.HasTiming {
		t.Fatalf("kind without duration must not count, got %+v", kindOnly)
	}
}

func TestBudgetInputFeedsCoreSpecs(t *testing.T) {
	nodes := []workflow.Node{
		{ID: "n1", Title: "冷热交替", Params: timingParams(
			TimingKindKey, TimingHandsOn, TimingDurKey, "5m", TimingRepeatKey, "3",
			TimingKind2Key, TimingWait, TimingDur2Key, "10m", TimingRepeat2Key, "3",
		)},
		{ID: "n2", Title: "未定量", Params: timingParams("温度", "80 °C")},
		{ID: "n3", Title: "坏时长", Params: timingParams(TimingDurKey, "soon")},
	}
	raw, known, unknown, problems := BudgetInput(nodes)
	if known != 1 || unknown != 2 {
		t.Fatalf("known=%d unknown=%d, want 1/2", known, unknown)
	}
	if len(problems) != 1 || !strings.Contains(problems[0], "步骤03") {
		t.Fatalf("problems = %q, want the step-scoped cause", problems)
	}
	var specs []map[string]any
	if err := json.Unmarshal([]byte(raw), &specs); err != nil {
		t.Fatalf("SpecsJSON must be valid JSON: %v", err)
	}
	if len(specs) != 2 {
		t.Fatalf("specs = %d, want 2 (cold hands-on + hot wait)", len(specs))
	}
	timing, _ := specs[0]["timing"].(map[string]any)
	if timing["kind"] != "hands_on" {
		t.Fatalf("first timing kind = %v, want hands_on", timing["kind"])
	}
	dur, _ := timing["duration"].(map[string]any)
	if dur["value"] != 300.0 || dur["unit"] != "s" {
		t.Fatalf("first duration = %v", timing["duration"])
	}
	rep, _ := specs[0]["repeat"].(map[string]any)
	if rep["count"] != 3.0 {
		t.Fatalf("first repeat = %v, want 3", specs[0]["repeat"])
	}
	timing1, _ := specs[1]["timing"].(map[string]any)
	if timing1["kind"] != "wait" {
		t.Fatalf("second timing kind = %v, want wait", timing1["kind"])
	}
}

func TestPhasesForStepRelay(t *testing.T) {
	if got := PhasesForStep(nil, "空"); got != nil {
		t.Fatalf("no timing must yield nil phases, never an empty plan: %+v", got)
	}
	phases := PhasesForStep(timingParams(
		TimingKindKey, TimingWait, TimingDurKey, "5m", TimingRepeatKey, "3",
		TimingKind2Key, TimingWait, TimingDur2Key, "10m", TimingRepeat2Key, "3",
	), "冷热交替")
	if len(phases) != 2 {
		t.Fatalf("phases = %+v, want cold+hot", phases)
	}
	if phases[0].Name != "冷热交替" || phases[1].Name != "冷热交替（二段）" {
		t.Fatalf("phase names = %q/%q", phases[0].Name, phases[1].Name)
	}
	if *phases[0].PlannedS != 300 || *phases[0].Rounds != 3 || *phases[1].PlannedS != 600 {
		t.Fatalf("phase plan = %+v", phases)
	}
	for _, p := range phases {
		if p.Mode != "countdown" {
			t.Fatalf("design timing relays as countdown, got %q", p.Mode)
		}
	}
}

func TestPhasesForFrozenStep(t *testing.T) {
	doc := `{"mygo":{"workflow":{"nodes":[` +
		`{"id":"n1","title":"冷热交替","params":[` +
		`{"Key":"计时类型","Val":"等待"},{"Key":"计时时长","Val":"5m"},{"Key":"重复次数","Val":"3"},` +
		`{"Key":"计时类型2","Val":"等待"},{"Key":"计时时长2","Val":"10m"},{"Key":"计时重复2","Val":"3"}]}` +
		`]}}}`
	phases := PhasesForFrozenStep(doc, "n1")
	if len(phases) != 2 || *phases[0].PlannedS != 300 || *phases[1].PlannedS != 600 {
		t.Fatalf("frozen phases = %+v", phases)
	}
	if got := PhasesForFrozenStep(doc, "missing"); got != nil {
		t.Fatalf("unknown step must yield nil, got %+v", got)
	}
	if got := PhasesForFrozenStep("not json", "n1"); got != nil {
		t.Fatalf("bad design data must yield nil, got %+v", got)
	}
}

func TestTimingSummary(t *testing.T) {
	first, second := ParseStepTiming(timingParams(TimingKindKey, TimingWait, TimingDurKey, "5m", TimingRepeatKey, "3"))
	if got := TimingSummary(first, second); got != "等待 5m ×3" {
		t.Fatalf("summary = %q", got)
	}
	if got := TimingSummary(StepTiming{}, StepTiming{}); got != "" {
		t.Fatalf("empty summary = %q, want empty", got)
	}
}
