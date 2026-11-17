package plate

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestLanePositionIsCanonical(t *testing.T) {
	if LanePosition(0) != "Lane1" || LanePosition(7) != "Lane8" {
		t.Fatalf("positions = %q %q", LanePosition(0), LanePosition(7))
	}
}

func rackLanes() []LaneState {
	return []LaneState{
		{Position: "Lane1", SampleID: "M3-1", Group: "给药组", Status: "raw", HasValue: true},
		{Position: "Lane2", SampleID: "M3-2", Group: "给药组", Status: "missing"},
		{Position: "Lane3", SampleID: "M3-3", Group: "对照组", Status: "excluded"},
	}
}

// TestRackRendersSequenceNotMatrix checks the 1D contract: coordinates and
// samples visible, shared palette, no plate-matrix wording (UX-P4, UX-DESIGN).
func TestRackRendersSequenceNotMatrix(t *testing.T) {
	tt := ui.NewTester(func(c *ui.Context) {
		Rack(c, RackProps{Lanes: rackLanes(), Interactive: true})
	}, 900, 600)
	tt.Frame()
	for _, want := range []string{"3 管架", "Lane1", "Lane3", "M3-1", "给药组", "已记录 1 · 缺失 2", "单击选择"} {
		if !tt.HasText(want) {
			t.Fatalf("rack missing %q: %q", want, tt.Texts())
		}
	}
}

// TestRackBlindMasksGroups keeps coordinates while group treatment names
// disappear (UX-P2).
func TestRackBlindMasksGroups(t *testing.T) {
	tt := ui.NewTester(func(c *ui.Context) {
		Rack(c, RackProps{Lanes: rackLanes(), Blind: true})
	}, 900, 600)
	tt.Frame()
	if !tt.HasText("已设盲 · 分组已隐藏") {
		t.Fatalf("blind notice missing: %q", tt.Texts())
	}
	for _, leak := range []string{"给药组", "对照组"} {
		if tt.HasText(leak) {
			t.Fatalf("blind rack leaks group %q: %q", leak, tt.Texts())
		}
	}
	for _, want := range []string{"Lane1", "M3-1", "3 管架"} {
		if !tt.HasText(want) {
			t.Fatalf("blind rack must keep coordinates, missing %q: %q", want, tt.Texts())
		}
	}
}

// TestRackSelectsLane checks single selection round-trips the position.
func TestRackSelectsLane(t *testing.T) {
	var got string
	tt := ui.NewTester(func(c *ui.Context) {
		Rack(c, RackProps{Lanes: rackLanes(), Interactive: true, Selected: got,
			OnSelect: func(position string) { got = position }})
	}, 900, 600)
	tt.Frame()
	if err := tt.Click("Lane2"); err != nil {
		t.Fatal(err)
	}
	if got != "Lane2" {
		t.Fatalf("selected = %q, want Lane2", got)
	}
}
