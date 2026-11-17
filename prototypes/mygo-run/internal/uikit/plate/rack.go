package plate

import (
	"fmt"
	"strconv"

	"github.com/egoist/mygo/ui"

	"asterlab-mygo-prototype/internal/theme"
)

// LaneState is one 1D container position: a tube-rack slot or an
// animal-number sequence entry (Core canonical form Lane<N>). Status names
// share the 2D plate palette (recorded/missing/excluded/invalid) so the strip
// never invents its own state colours (UX-P3, UX-DESIGN).
type LaneState struct {
	Position string
	SampleID string
	Group    string
	Status   string
	HasValue bool
}

// RackProps renders a 1D lane strip instead of a 2D matrix. Position labels
// are coordinates and stay visible under the blind mask; only group
// treatment identity is hidden (UX-P2, UX-P4).
type RackProps struct {
	Lanes       []LaneState
	Blind       bool
	Selected    string
	OnSelect    func(position string)
	Interactive bool
}

// LanePosition returns the Core canonical 1D coordinate for a zero-based
// slot: Lane1, Lane2, … (BE-09: Lane<N>/L<N>/Tube<N> ≡ A<N>, stored Lane<N>).
func LanePosition(i int) string {
	return "Lane" + strconv.Itoa(i+1)
}

// laneFill maps a lane status to the shared 2D plate palette: recorded tints
// accent, excluded warns, invalid dangers, missing stays canvas.
func laneFill(lane LaneState, t *ui.Theme, tokens theme.Tokens) ui.Color {
	switch lane.Status {
	case "excluded":
		return tokens.WarningBg
	case "invalid":
		return tokens.DangerBg
	default:
		if lane.HasValue {
			return tokens.Accent.Alpha(.10)
		}
		return tokens.Canvas
	}
}

// Rack renders the tube-rack / animal-number sequence as a wrapped strip of
// lane chips (UX-P4: a sequence reads as a sequence, not a plate matrix).
// Click selects one lane; clicking it again clears it.
func Rack(c *ui.Context, props RackProps) {
	th := c.Theme()
	t := theme.GetTokens(th)
	recorded := 0
	for _, lane := range props.Lanes {
		if lane.HasValue {
			recorded++
		}
	}
	ui.Column(c).FillWidth().Gap(float32(t.Spacing8)).Children(func() {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(float32(t.Spacing8)).Children(func() {
			ui.Text(c, fmt.Sprintf("%d 管架", len(props.Lanes))).FontSize(float32(t.TextMeta)).FontWeight(600).TextColor(th.Text)
			ui.Text(c, fmt.Sprintf("已记录 %d · 缺失 %d", recorded, len(props.Lanes)-recorded)).
				FontSize(float32(t.TextMeta)).TextColor(th.TextMuted)
			ui.Spacer(c)
			if props.Blind {
				ui.Text(c, "已设盲 · 分组已隐藏").FontSize(float32(t.TextMeta)).TextColor(th.TextMuted)
			}
		})
		ui.Row(c).FillWidth().Wrap().Gap(float32(t.Spacing8)).Children(func() {
			for _, lane := range props.Lanes {
				lane := lane
				selected := props.Selected == lane.Position && lane.Position != ""
				border := t.Border
				if selected {
					border = t.Accent
				}
				chip := ui.ButtonBase(c).Padding(6, 10).Radius(6).
					Background(laneFill(lane, th, t)).Border(1, border)
				chip.Children(func() {
					ui.Column(c).Gap(2).Children(func() {
						ui.Text(c, lane.Position).FontSize(11).FontWeight(600).TextColor(th.Text).FontFeatures("tnum")
						label := lane.SampleID
						if label == "" {
							label = "未分配"
						}
						ui.Text(c, label).FontSize(10).TextColor(th.TextMuted).SingleLine().Ellipsis("…")
						if !props.Blind && lane.Group != "" {
							ui.Row(c).AlignItems(ui.Center).Gap(4).Children(func() {
								ui.Box(c).Size(8, 8).Radius(4).Background(GroupColor(lane.Group, th.Dark))
								ui.Text(c, lane.Group).FontSize(10).TextColor(th.TextMuted).SingleLine().Ellipsis("…")
							})
						}
					})
				})
				if chip.Clicked() && props.Interactive && lane.Position != "" {
					next := lane.Position
					if props.Selected == lane.Position && props.OnSelect != nil {
						next = ""
					}
					if props.OnSelect != nil {
						props.OnSelect(next)
					}
				}
			}
		})
		ui.Text(c, fmt.Sprintf("%d 个位置 · 单击选择", len(props.Lanes))).FontSize(float32(t.TextMeta)).TextColor(th.TextMuted)
	})
}
