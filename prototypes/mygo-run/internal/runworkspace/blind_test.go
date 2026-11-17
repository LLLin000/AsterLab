package runworkspace

import (
	"errors"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"asterlab-mygo-prototype/internal/resultgrid"
	"asterlab-mygo-prototype/internal/theme"
	"asterlab-mygo-prototype/modules/runs"
)

func TestBlindQueryParamsIsExactCLIJSON(t *testing.T) {
	if got := blindQueryParams("run-cck8"); got != `{"id":"run-cck8","view":"blind"}` {
		t.Fatalf("params = %q", got)
	}
}

// TestBlindToggleConsumesWorkspaceProjection wires the Run top-bar switch to
// run.workspace view=blind through QueryProjection (Bridge v2): entering
// blind confirms with Core, the mask notice names coordinates as visible, and
// no group treatment name leaks.
func TestBlindToggleConsumesWorkspaceProjection(t *testing.T) {
	s := buildTestState()
	var got runs.QueryProjectionInput
	calls := 0
	s.Callbacks.QueryProjection = func(input runs.QueryProjectionInput) (runs.QueryProjectionResult, error) {
		got, calls = input, calls+1
		return runs.QueryProjectionResult{ResultJSON: `{"ok":true}`}, nil
	}
	render := func(c *ui.Context) {
		c.SetTheme(theme.LightTheme())
		View(c, s)
	}
	before := ui.NewTester(render, 1360, 880)
	before.Frame()
	if !before.HasText("○ 设盲视图") {
		t.Fatalf("header blind switch missing: %q", before.Texts())
	}
	if err := before.Click("○ 设盲视图"); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("QueryProjection calls = %d, want 1", calls)
	}
	if got.Kind != "run.workspace" {
		t.Fatalf("query kind = %q, want run.workspace", got.Kind)
	}
	if got.ParamsJSON != `{"id":"run-cck8","view":"blind"}` {
		t.Fatalf("params = %q", got.ParamsJSON)
	}
	after := ui.NewTester(render, 1360, 880)
	after.Frame()
	if !after.HasText("● 设盲视图") {
		t.Fatalf("blind switch did not flip: %q", after.Texts())
	}
	if !after.HasText("已设盲 · 分组已隐藏，孔位坐标保持可见") {
		t.Fatalf("blind mask notice missing: %q", after.Texts())
	}
	if !s.bench().Blind {
		t.Fatal("bench blind flag not set")
	}
	// Leaving blind unmasks locally with no second Core round-trip.
	if err := after.Click("● 设盲视图"); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("unmask must not query Core, calls = %d", calls)
	}
	if s.bench().Blind {
		t.Fatal("bench blind flag not cleared")
	}
}

// TestBlindToggleSurfacesInvalidBasis keeps the current mask and names the
// cause when the frozen design declares no blinding basis (UX-STATE, UX-P5).
func TestBlindToggleSurfacesInvalidBasis(t *testing.T) {
	s := buildTestState()
	s.Callbacks.QueryProjection = func(runs.QueryProjectionInput) (runs.QueryProjectionResult, error) {
		return runs.QueryProjectionResult{}, errors.New("INVALID_INPUT: blinding basis missing")
	}
	render := func(c *ui.Context) {
		c.SetTheme(theme.LightTheme())
		View(c, s)
	}
	tt := ui.NewTester(render, 1360, 880)
	tt.Frame()
	if err := tt.Click("○ 设盲视图"); err != nil {
		t.Fatal(err)
	}
	if s.bench().Blind {
		t.Fatal("mask must stay off after Core rejection")
	}
	retry := ui.NewTester(render, 1360, 880)
	retry.Frame()
	if !retry.HasText("该设计未声明盲法依据，无法进入设盲视图") {
		t.Fatalf("cause missing: %q", retry.Texts())
	}
	if !retry.HasText("重试") {
		t.Fatalf("retry missing: %q", retry.Texts())
	}
}

// TestBlindToggleWithoutCallback masks locally: the switch is pure display
// masking and never requires the host to wire the channel.
func TestBlindToggleWithoutCallback(t *testing.T) {
	s := buildTestState()
	if s.Callbacks.QueryProjection != nil {
		t.Fatal("fixture must leave QueryProjection unwired")
	}
	s.toggleBlind()
	if !s.bench().Blind || s.BlindErr != "" {
		t.Fatalf("local mask failed: blind=%v err=%q", s.bench().Blind, s.BlindErr)
	}
	s.toggleBlind()
	if s.bench().Blind {
		t.Fatal("local unmask failed")
	}
}

// TestRecordingBlindMasksTableGroups keeps sample coordinates visible while
// group treatment names disappear from table rows (UX-P2).
func TestRecordingBlindMasksTableGroups(t *testing.T) {
	vm := BuildViewModel(makeSnapshot(), makeFrozenDoc())
	task := vm.TaskByKey["t-read"]
	grid := blindGridForTable()
	for _, blind := range []bool{false, true} {
		rv := &RecordingView{Kind: ViewTable, Grid: grid, Task: task, Blind: blind}
		tt := ui.NewTester(func(c *ui.Context) {
			c.SetTheme(theme.LightTheme())
			rv.Render(c)
		}, 1000, 700)
		tt.Frame()
		if !tt.HasText("A1") {
			t.Fatalf("blind=%v: coordinate missing: %q", blind, tt.Texts())
		}
		if has := tt.HasText("给药组"); has == blind {
			t.Fatalf("blind=%v: group visibility wrong: %q", blind, tt.Texts())
		}
		if !grid.Blind == blind {
			t.Fatalf("blind=%v: grid flag not propagated", blind)
		}
	}
	if !strings.Contains(blindSwitchLabel(true), "设盲") {
		t.Fatal("blind label must name the mask state, never a raw enum")
	}
}

func blindGridForTable() *resultgrid.Grid {
	value := 0.82
	return resultgrid.New(resultgrid.Schema{
		ID: "cck8.plate96.od450.v1", Version: 1,
		RowLabels: []string{"A"}, ColumnLabels: []string{"1"},
		Fields: []resultgrid.Field{{Key: "od450", Label: "OD450", Unit: "AU"}},
	}, []resultgrid.Cell{
		{Row: 0, Column: 0, SampleID: "M3-1", Group: "给药组", Replicate: 1, Field: "od450", Value: &value, Status: "raw"},
	})
}
