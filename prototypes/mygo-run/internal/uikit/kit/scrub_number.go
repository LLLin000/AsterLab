package kit

import (
	"fmt"
	"math"
	"runtime"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/onevra/source/components"
	"github.com/oneo-me/onevra/source/icons/lucide"

	"asterlab-mygo-prototype/internal/theme"
)

// ScrubOptions configures a Blender/Figma-style scrubbable numeric control.
type ScrubOptions struct {
	Label       string
	Unit        string
	Min         float64
	Max         float64
	Step        float64
	Decimals    int
	Sensitivity float32 // pixels per Step (default 6)
	Width       float32 // 0 uses responsive compact pill width
}

// ScrubResult describes whether a value changed live vs committed atomically.
type ScrubResult struct {
	Changed   bool
	Committed bool
}

type scrubMode uint8

const (
	scrubIdle scrubMode = iota
	scrubPointing
	scrubDragging
	scrubEditing
)

type scrubState struct {
	mode     scrubMode
	downX    float32
	startVal float64
	editBuf  string
}

func clampScrub(v, minV, maxV float64) float64 {
	if maxV > minV {
		if v < minV {
			return minV
		}
		if v > maxV {
			return maxV
		}
	}
	return v
}

func roundScrubDecimals(v float64, decimals int) float64 {
	if decimals <= 0 {
		return math.Round(v)
	}
	pow := math.Pow(10, float64(decimals))
	return math.Round(v*pow) / pow
}

func formatScrubValue(v float64, decimals int) string {
	if decimals <= 0 {
		return strconv.FormatInt(int64(math.Round(v)), 10)
	}
	return fmt.Sprintf("%.*f", decimals, v)
}

// applyScrubDelta applies a 3px deadzone pointer delta with Shift (0.1x) & Cmd (5x) modifiers.
func applyScrubDelta(st *scrubState, ev ui.InputEvent, val *float64, opts ScrubOptions) ScrubResult {
	return applyScrubDeltaDeadzone(st, ev, val, opts, 3)
}

func applyScrubDeltaDeadzone(st *scrubState, ev ui.InputEvent, val *float64, opts ScrubOptions, deadzonePx float32) ScrubResult {
	var res ScrubResult
	step := opts.Step
	if step <= 0 {
		step = 1
	}
	sens := opts.Sensitivity
	if sens <= 0 {
		sens = 6
	}
	if deadzonePx <= 0 {
		deadzonePx = 3
	}
	switch ev.Kind {
	case ui.InputPointerDown:
		if ev.Button != 0 {
			return res
		}
		st.mode = scrubPointing
		st.downX = ev.X
		st.startVal = *val
	case ui.InputPointerMove:
		if st.mode != scrubPointing && st.mode != scrubDragging {
			return res
		}
		dx := ev.X - st.downX
		if st.mode == scrubPointing {
			if math.Abs(float64(dx)) < float64(deadzonePx) {
				return res
			}
			st.mode = scrubDragging
		}
		mult := 1.0
		if ev.Mods&ui.Shift != 0 {
			mult = 0.1
		} else if ev.Mods&ui.Cmd != 0 {
			mult = 5.0
		}
		steps := float64(dx) / float64(sens) * mult
		next := clampScrub(roundScrubDecimals(st.startVal+steps*step, opts.Decimals), opts.Min, opts.Max)
		if next != *val {
			*val = next
			res.Changed = true
		}
	case ui.InputPointerUp:
		if ev.Button != 0 {
			return res
		}
		switch st.mode {
		case scrubDragging:
			st.mode = scrubIdle
			res.Committed = true
		case scrubPointing:
			st.mode = scrubEditing
			st.editBuf = formatScrubValue(*val, opts.Decimals)
		}
	}
	return res
}

// ScrubNumber renders a Blender/Figma-style scrubbable numeric field:
// - Drag horizontally (>3px deadzone) to continuously scrub transient values; release commits once.
// - Hold Shift while scrubbing for 0.1x precision, or Ctrl/Cmd for 5x steps.
// - Click (<3px) to enter exact inline text edit (Enter commits, Esc cancels).
func ScrubNumber(c *ui.Context, key any, tok theme.Tokens, val *float64, opts ScrubOptions) ScrubResult {
	var res ScrubResult
	if c == nil || val == nil {
		return res
	}
	step := opts.Step
	if step <= 0 {
		step = 1
	}
	decimals := opts.Decimals
	if decimals < 0 {
		decimals = 0
	}
	host := ui.Row(c.Key(key)).AlignItems(ui.Center)
	if opts.Width > 0 {
		host.Width(opts.Width)
	}
	st := ui.Local(host, "kit-scrub", func() scrubState { return scrubState{} })
	if st == nil {
		return res
	}

	if st.mode == scrubDragging && c.Shortcut(0, ui.KeyEscape) {
		if *val != st.startVal {
			*val = st.startVal
			res.Changed = true
		}
		st.mode = scrubIdle
		return res
	}

	if st.mode == scrubEditing {
		focPtr := ui.Local(host, "kit-scrub-foc", func() bool { return false })
		commitEdit := func() {
			if parsed, err := strconv.ParseFloat(strings.TrimSpace(st.editBuf), 64); err == nil {
				clamped := clampScrub(roundScrubDecimals(parsed, decimals), opts.Min, opts.Max)
				if clamped != *val {
					*val = clamped
					res.Changed = true
				}
				res.Committed = true
			}
			*focPtr = false
			st.mode = scrubIdle
		}
		var tb ui.Element
		okClicked := false
		host.Gap(float32(tok.Spacing4)).Children(func() {
			ui.Box(c).Grow(1).MinWidth(0).Children(func() {
				tb = components.TextBox(c, &st.editBuf, components.TextBoxOptions{
					LeadingText: opts.Label,
				}).Keyboard(ui.KeyboardDecimal)
			})
			if opts.Unit != "" {
				ui.Text(c, opts.Unit).FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted).SingleLine().Shrink(0)
			}
			okBtn := ui.ButtonBase(c.Key("kit-scrub-ok")).
				Size(float32(tok.Spacing32), float32(tok.Spacing32)).
				Radius(float32(tok.ControlRadius)).
				Background(tok.AccentSoftBg).
				Border(1, tok.Accent.Alpha(0.35)).
				Center().Label("确认")
			okBtn.Children(func() {
				ui.Icon(c, lucide.Check()).Size(float32(tok.Spacing16), float32(tok.Spacing16)).TextColor(tok.Accent)
			})
			okClicked = okBtn.Clicked()
		})
		focusedNow := tb.Focused()
		wasFocused := *focPtr
		*focPtr = focusedNow
		if c.Shortcut(0, ui.KeyEscape) {
			*focPtr = false
			st.mode = scrubIdle
			return res
		}
		if okClicked || c.Shortcut(0, ui.KeyEnter) || tb.Submitted() || (wasFocused && !focusedNow) {
			commitEdit()
		}
		return res
	}

	vw, _ := c.Size()
	deadzonePx := float32(3)
	if runtime.GOOS == "ios" || runtime.GOOS == "android" || (vw > 0 && vw < 680) {
		deadzonePx = 8
	}
	host.Children(func() {
		pill := ui.ButtonBase(c).
			FillWidth().
			Height(float32(tok.InputHeight)).
			Padding(0, float32(tok.Spacing8)).
			Radius(float32(tok.ControlRadius)).
			Background(tok.Surface).
			Border(1, tok.Border).
			Justify(ui.Start).
			AlignItems(ui.Center)
		lbl := opts.Label
		if lbl == "" {
			lbl = "数值"
		}
		pill.Label(fmt.Sprintf("%s %s %s", lbl, formatScrubValue(*val, decimals), opts.Unit))

		if st.mode == scrubDragging {
			pill.Background(tok.AccentSoftBg).Border(1.5, tok.BorderFocused)
		} else if pill.Hovered() {
			pill.Background(tok.ElementHover).Border(1, tok.BorderStrong)
		}

		pill.HandleInput(func(ev ui.InputEvent) bool {
			evRes := applyScrubDeltaDeadzone(st, ev, val, opts, deadzonePx)
			if evRes.Changed {
				res.Changed = true
			}
			if evRes.Committed {
				res.Committed = true
			}
			return ev.Kind == ui.InputPointerDown || (st.mode == scrubDragging && ev.Kind == ui.InputPointerMove)
		})

		pill.Children(func() {
			ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(float32(tok.Spacing4)).Children(func() {
				if opts.Label != "" {
					ui.Text(c, opts.Label).FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted).SingleLine().Shrink(0)
				}
				ui.Spacer(c)
				ui.Icon(c, lucide.MoveHorizontal()).Size(float32(tok.Spacing12), float32(tok.Spacing12)).TextColor(tok.TextPlaceholder).Shrink(0)
				valTxt := formatScrubValue(*val, decimals)
				ui.Text(c, valTxt).FontSize(float32(tok.TextData)).Bold().TextColor(tok.Text).FontFeatures("tnum").SingleLine().Shrink(0)
				if opts.Unit != "" {
					ui.Text(c, opts.Unit).FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted).SingleLine().Shrink(0)
				}
			})
		})
	})
	return res
}
