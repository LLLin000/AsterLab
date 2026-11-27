package kit

import (
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/onevra/source/components"
	"github.com/oneo-me/onevra/source/icons/lucide"

	"asterlab-mygo-prototype/internal/theme"
)

// MiddleTruncate shortens s to at most maxRunes Unicode runes while preserving both
// the leading identifier prefix and trailing well/FOV/extension suffix around '…'.
func MiddleTruncate(s string, maxRunes int) string {
	if maxRunes <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= maxRunes {
		return s
	}
	if maxRunes == 1 {
		return "…"
	}
	if maxRunes == 2 {
		return string(runes[:1]) + "…"
	}
	avail := maxRunes - 1
	prefixLen := (avail + 1) / 2
	suffixLen := avail - prefixLen

	// Keep file extension + immediate identifier token intact when a short extension exists.
	extLen := 0
	for i := len(runes) - 1; i >= 0 && len(runes)-i <= 10; i-- {
		if runes[i] == '.' {
			extLen = len(runes) - i
			break
		}
	}
	if extLen > 0 && suffixLen < extLen+4 && avail > extLen+8 {
		suffixLen = min(avail-4, extLen+5)
		prefixLen = avail - suffixLen
	}
	return string(runes[:prefixLen]) + "…" + string(runes[len(runes)-suffixLen:])
}

// ShortcutKeycaps renders tactile recessed keyboard shortcut keycaps using
// theme.ApplyRecessedWell, tabular numerals (tnum), and concentric inner radii.
func ShortcutKeycaps(c *ui.Context, tok theme.Tokens, keys ...string) ui.Element {
	row := ui.Row(c).AlignItems(ui.Center).Gap(float32(tok.Spacing4)).Shrink(0)
	row.Children(func() {
		pad := tok.Spacing4 / 2
		capRad := tok.InnerControlRadius(pad)
		for _, raw := range keys {
			k := strings.TrimSpace(raw)
			if k == "" {
				continue
			}
			capBox := ui.Row(c).
				Padding(float32(tok.Spacing4)*0.5, float32(tok.Spacing8)).
				Radius(capRad).
				Background(tok.Elevated).
				Border(1, tok.Border).
				Center()
			theme.ApplyRecessedWell(capBox, tok)
			capBox.Children(func() {
				ui.Text(c, k).
					FontSize(float32(tok.TextMeta)).
					FontFeatures("tnum").
					TextColor(tok.TextMuted).
					SingleLine()
			})
		}
	})
	return row
}

// MorphButtonPhase represents the lifecycle state of a width-stable MorphActionButton.
type MorphButtonPhase uint8

const (
	MorphIdle MorphButtonPhase = iota
	MorphBusy
	MorphDone
)

// MorphActionState tracks the active phase and locked minimum width across transitions.
type MorphActionState struct {
	Phase      MorphButtonPhase
	PhaseSince time.Time
	lockedW    float32
}

// MorphActionOptions configures a width-stable MorphActionButton.
type MorphActionOptions struct {
	Key       any
	IdleLabel string
	BusyLabel string
	DoneLabel string
	IdleIcon  *ui.SVG
	BusyIcon  *ui.SVG
	DoneIcon  *ui.SVG
	Primary   bool
	MinWidth  float32
}

func estimateLabelWidth(s string, fontSize float32) float32 {
	var w float32
	for _, r := range s {
		if r > unicode.MaxASCII {
			w += fontSize
		} else {
			w += fontSize * 0.6
		}
	}
	return w
}

// MorphActionButton renders a width-stable action button using IconMorph and tactile press feedback
// so Idle -> Busy -> Done transitions never jitter surrounding layout width.
func MorphActionButton(c *ui.Context, tok theme.Tokens, st *MorphActionState, opts MorphActionOptions) bool {
	if c == nil {
		return false
	}
	key := opts.Key
	if key == nil {
		key = "kit-morph-btn"
	}
	idleLbl := opts.IdleLabel
	if idleLbl == "" {
		idleLbl = "保存更改"
	}
	busyLbl := opts.BusyLabel
	if busyLbl == "" {
		busyLbl = "处理中…"
	}
	doneLbl := opts.DoneLabel
	if doneLbl == "" {
		doneLbl = "已完成"
	}

	btn := ui.ButtonBase(c.Key(key))
	if st == nil {
		st = ui.Local(btn, "morph-state", func() MorphActionState { return MorphActionState{} })
	}

	lbl := idleLbl
	ico := opts.IdleIcon
	bg := tok.Elevated
	fg := tok.Text
	bd := tok.Border
	if opts.Primary {
		bg = tok.Accent
		fg = tok.AccentText
		bd = tok.Accent
	}
	switch st.Phase {
	case MorphBusy:
		lbl = busyLbl
		if opts.BusyIcon != nil {
			ico = opts.BusyIcon
		} else {
			ico = lucide.LoaderCircle()
		}
		bg = tok.SelectedRow
		fg = tok.TextMuted
		bd = tok.BorderStrong
	case MorphDone:
		lbl = doneLbl
		if opts.DoneIcon != nil {
			ico = opts.DoneIcon
		} else {
			ico = lucide.Check()
		}
		bg = tok.SuccessBg
		fg = tok.SuccessText
		bd = tok.Success
	}

	maxLblW := max(
		estimateLabelWidth(idleLbl, float32(tok.TextData)),
		max(
			estimateLabelWidth(busyLbl, float32(tok.TextData)),
			estimateLabelWidth(doneLbl, float32(tok.TextData)),
		),
	)
	minW := maxLblW + float32(tok.Spacing12)*2 + float32(tok.IconSm) + float32(tok.Spacing8)
	if opts.MinWidth > minW {
		minW = opts.MinWidth
	}
	if st.lockedW > minW {
		minW = st.lockedW
	}

	btn.MinWidth(minW).
		MinHeight(float32(tok.ButtonHeight)).
		Padding(float32(tok.Spacing4), float32(tok.Spacing12)).
		Radius(float32(tok.ControlRadius)).
		Background(bg).
		Border(1, bd).
		Center().
		Disabled(st.Phase == MorphBusy).
		Label(lbl)
	if btn.Hovered() && st.Phase == MorphIdle {
		if opts.Primary {
			btn.Background(tok.AccentHover)
		} else {
			btn.Background(tok.ElementHover)
		}
	}
	theme.ApplyTactilePress(btn)
	btn.Children(func() {
		content := ui.Row(c.Key("morph-content")).
			AlignItems(ui.Center).
			Center().
			Gap(float32(tok.Spacing4)).
			ContentTransition(theme.IconMorph())
		content.Children(func() {
			if ico != nil {
				ui.Icon(c, ico).Size(float32(tok.IconSm), float32(tok.IconSm)).TextColor(fg).Shrink(0)
			}
			ui.Text(c, lbl).
				FontSize(float32(tok.TextData)).
				Bold().
				FontFeatures("tnum").
				TextColor(fg).
				SingleLine().
				ContentTransition(theme.IconMorph())
		})
	})
	if b := btn.Bounds(); b.W > st.lockedW {
		st.lockedW = b.W
	}
	return btn.Clicked() && st.Phase == MorphIdle
}

// SlideConfirmState holds drag progress and confirmed lock state for SlideConfirm.
type SlideConfirmState struct {
	Confirmed   bool
	DragOffset  ui.Point
	Progress    float32 // 0..1 normalized progress along track
	dragging    bool
	pointerDown bool
	nudgeSeq    int
}

// Reset clears the confirmed lock and snaps the handle back to 0.
func (s *SlideConfirmState) Reset() {
	if s == nil {
		return
	}
	s.Confirmed = false
	s.DragOffset = ui.Point{}
	s.Progress = 0
	s.dragging = false
}

// SlideConfirmOptions configures an irreversible horizontal drag-to-confirm bar.
type SlideConfirmOptions struct {
	Key            any
	Label          string
	ConfirmedLabel string
	Danger         bool
	Threshold      float32 // default 0.82
	Width          float32 // 0 fills available width cleanly
	Disabled       bool
}

const slideConfirmDefaultThreshold = float32(0.82)

// SlideConfirm renders a Topinspo-inspired recessed horizontal drag-to-confirm track.
// Releasing below threshold (82%) snaps back via spring; releasing >= threshold locks confirmed
// and returns true on the transition frame. Keyboard Enter/Space when focused also confirms accessibly.
func SlideConfirm(c *ui.Context, tok theme.Tokens, st *SlideConfirmState, opts SlideConfirmOptions) bool {
	if c == nil {
		return false
	}
	key := opts.Key
	if key == nil {
		key = "kit-slide-confirm"
	}
	track := ui.Row(c.Key(key))
	if st == nil {
		st = ui.Local(track, "slide-state", func() SlideConfirmState { return SlideConfirmState{} })
	}
	threshold := opts.Threshold
	if threshold <= 0 || threshold > 0.98 {
		threshold = slideConfirmDefaultThreshold
	}
	label := opts.Label
	if label == "" {
		label = "向右滑动确认操作"
	}
	confirmedLabel := opts.ConfirmedLabel
	if confirmedLabel == "" {
		confirmedLabel = "已锁定确认"
	}

	tone := tok.Warning
	if opts.Danger {
		tone = tok.Danger
	}

	trackH := float32(tok.ButtonHeight) + float32(tok.Spacing8)
	pad := float32(tok.Spacing4)
	handleSz := trackH - pad*2
	track.Height(trackH).
		Padding(pad).
		Radius(float32(tok.ControlRadius)).
		Background(tok.Elevated).
		Border(1, tok.Border).
		AlignItems(ui.Center).
		Clip()
	if opts.Width > 0 {
		track.Width(opts.Width)
	} else {
		track.FillWidth()
	}
	theme.ApplyRecessedWell(track, tok)
	theme.ApplyNudge(track, st.nudgeSeq)

	trackW := track.Bounds().W
	if trackW <= 0 {
		if opts.Width > 0 {
			trackW = opts.Width
		} else {
			trackW = float32(tok.Spacing48) * 6
		}
	}
	maxTravel := trackW - pad*2 - handleSz
	if maxTravel < handleSz {
		maxTravel = handleSz
	}

	confirmedNow := false
	if st.Confirmed {
		st.DragOffset = ui.Point{X: maxTravel}
		st.Progress = 1
	}

	track.Children(func() {
		// Progress fill along the recessed well.
		pct := st.Progress
		if pct < 0 {
			pct = 0
		} else if pct > 1 {
			pct = 1
		}
		ui.Box(c.Key("slide-fill")).
			Absolute().Top(0).Bottom(0).Left(0).FillWidth().
			Background(tone.Alpha(0.20)).
			ClipInsetPercent(0, (1-pct)*100, 0, 0, 0).
			IgnoreInput(true)

		// Center prompt text.
		ui.Row(c).Absolute().Fill().Center().Padding(0, handleSz+float32(tok.Spacing8)).IgnoreInput(true).Children(func() {
			txt := label
			txtCol := tok.TextMuted
			if st.Confirmed {
				txt = confirmedLabel
				txtCol = tone
			}
			ui.Text(c, txt).
				FontSize(float32(tok.TextMeta)).
				Bold().
				TextColor(txtCol).
				SingleLine().
				Ellipsis("…").
				MinWidth(0)
		})

		handleRad := tok.InnerControlRadius(tok.Spacing4)
		handleBg := tok.Surface
		handleFg := tok.Text
		handleIcon := lucide.ChevronsRight()
		if st.Confirmed {
			handleBg = tone
			handleFg = tok.AccentText
			handleIcon = lucide.Check()
		}
		handle := ui.ButtonBase(c.Key("slide-handle")).
			Size(handleSz, handleSz).
			Radius(handleRad).
			Background(handleBg).
			Border(1, tok.BorderStrong).
			Center().
			Focusable().
			Disabled(opts.Disabled).
			Label(label)
		ApplyFocusRing(handle, tok)

		if !st.Confirmed && !opts.Disabled {
			handle.Draggable(&st.DragOffset, ui.DraggableOptions{
				Axis:  ui.Horizontal,
				Rigid: true,
				Bounds: &ui.Rect{
					X: 0,
					W: maxTravel,
				},
				Snap: func(projected ui.Point) ui.Point {
					if maxTravel > 0 && projected.X/maxTravel >= threshold {
						return ui.Point{X: maxTravel}
					}
					return ui.Point{}
				},
				Animation: theme.MotionSnappy,
			})
			curDrag := handle.DragOffset()
			dragging := handle.Dragging()
			ratio := curDrag.X / maxTravel
			if ratio < 0 {
				ratio = 0
			} else if ratio > 1 {
				ratio = 1
			}
			st.Progress = ratio
			if !dragging && (st.DragOffset.X/maxTravel >= threshold || (st.dragging && ratio >= threshold)) {
				st.Confirmed = true
				st.DragOffset = ui.Point{X: maxTravel}
				st.Progress = 1
				confirmedNow = true
				mygo.Haptics.Selection()
			}
			st.dragging = dragging

			// Keyboard accessible confirmation when handle is focused, or nudge on plain pointer click.
			handle.ObserveInput(func(ev ui.InputEvent) {
				switch ev.Kind {
				case ui.InputPointerDown:
					st.pointerDown = true
				case ui.InputKeyDown:
					if ev.Key == ui.KeyEnter || ev.Key == ui.KeySpace {
						st.pointerDown = false
					}
				}
			})
			if handle.Focused() && (c.Shortcut(0, ui.KeyEnter) || c.Shortcut(0, ui.KeySpace)) {
				st.Confirmed = true
				st.DragOffset = ui.Point{X: maxTravel}
				st.Progress = 1
				confirmedNow = true
				mygo.Haptics.Selection()
			} else if handle.Clicked() && !dragging {
				if !st.pointerDown {
					st.Confirmed = true
					st.DragOffset = ui.Point{X: maxTravel}
					st.Progress = 1
					confirmedNow = true
					mygo.Haptics.Selection()
				} else if ratio < 0.1 {
					st.nudgeSeq++
				}
			}
		} else {
			handle.Translate(maxTravel, 0)
		}

		handle.Children(func() {
			ui.Icon(c, handleIcon).Size(float32(tok.IconSm), float32(tok.IconSm)).TextColor(handleFg)
		})
	})

	return confirmedNow
}

// FloatingSelectionAction describes one action button inside FloatingSelectionBar.
type FloatingSelectionAction struct {
	ID      string
	Label   string
	Icon    *ui.SVG
	Primary bool
	Danger  bool
	OnClick func()
}

// FloatingSelectionBarProps configures FloatingSelectionBar.
type FloatingSelectionBarProps struct {
	Key        any
	Count      int
	UnitLabel  string
	Actions    []FloatingSelectionAction
	ClearLabel string
	OnClear    func()
}

// FloatingSelectionBar renders an elevated contextual selection toolbar with a NumericRoll
// selected count badge, action buttons, and a clear action. Returns zero Element when Count <= 0.
func FloatingSelectionBar(c *ui.Context, tok theme.Tokens, props FloatingSelectionBarProps) ui.Element {
	if c == nil || props.Count <= 0 {
		return ui.Element{}
	}
	key := props.Key
	if key == nil {
		key = "kit-floating-selection-bar"
	}
	unitLbl := props.UnitLabel
	if unitLbl == "" {
		unitLbl = "项已选"
	}
	clearLbl := props.ClearLabel
	if clearLbl == "" {
		clearLbl = "取消选择"
	}

	bar := ui.Row(c.Key(key)).
		Wrap().
		Padding(float32(tok.Spacing8), float32(tok.Spacing12)).
		AlignItems(ui.Center).
		Gap(float32(tok.Spacing8)).
		Transition(theme.FloatingBarTransition())
	ApplyElevation(bar, tok, ElevationFloating)

	innerRad := tok.InnerPanelRadius(tok.Spacing8)
	bar.Children(func() {
		// Selected count badge with mechanical NumericRoll.
		ui.Row(c).
			Padding(float32(tok.Spacing4), float32(tok.Spacing8)).
			Radius(float32(tok.PillRadius)).
			Background(tok.AccentSoftBg).
			Border(1, tok.BorderSelected).
			AlignItems(ui.Center).
			Gap(float32(tok.Spacing4)).
			Shrink(0).Children(func() {
			ui.Text(c, fmt.Sprintf("%d", props.Count)).
				FontSize(float32(tok.TextData)).
				Bold().
				FontFeatures("tnum").
				TextColor(tok.Accent).
				ContentTransition(theme.NumericRoll(false)).
				SingleLine()
			ui.Text(c, unitLbl).
				FontSize(float32(tok.TextMeta)).
				TextColor(tok.Text).
				SingleLine()
		})

		ui.Box(c).Width(1).Height(float32(tok.Spacing16)).Background(tok.HairlineDivider)

		for idx, act := range props.Actions {
			act := act
			variant := components.ButtonSecondary
			fg := tok.Text
			if act.Primary {
				variant = components.ButtonPrimary
				fg = tok.AccentText
			} else if act.Danger {
				variant = components.ButtonGhost
				fg = tok.DangerText
			}
			ui.Box(c.Key(fmt.Sprintf("fsb-act-%d-%s", idx, act.ID))).Children(func() {
				var btn ui.Element
				if act.Icon != nil {
					btn = components.Button(c, "", variant).Label(act.Label)
					btn.Children(func() {
						ui.Row(c).AlignItems(ui.Center).Gap(float32(tok.Spacing4)).Children(func() {
							ui.Icon(c, act.Icon).Size(float32(tok.IconSm), float32(tok.IconSm)).TextColor(fg).Shrink(0)
							ui.Text(c, act.Label).FontSize(float32(tok.TextData)).Bold().TextColor(fg).SingleLine()
						})
					})
				} else {
					btn = components.Button(c, act.Label, variant)
					if act.Danger {
						btn.TextColor(tok.DangerText)
					}
				}
				btn.Radius(innerRad)
				theme.ApplyTactilePress(btn)
				if btn.Clicked() && act.OnClick != nil {
					act.OnClick()
				}
			})
		}

		if props.OnClear != nil {
			ui.Box(c.Key("fsb-clear")).Children(func() {
				clr := components.Button(c, clearLbl, components.ButtonGhost).Radius(innerRad)
				theme.ApplyTactilePress(clr)
				if clr.Clicked() {
					props.OnClear()
				}
			})
		}
	})
	return bar
}

// SkeletonBlock renders a single token-backed placeholder block with rounded corners.
func SkeletonBlock(c *ui.Context, tok theme.Tokens, w, h, radius float32) ui.Element {
	if c == nil {
		return ui.Element{}
	}
	if radius <= 0 {
		radius = float32(tok.ControlRadius)
	}
	if h <= 0 {
		h = float32(tok.Spacing12)
	}
	el := ui.Box(c).
		Height(h).
		Radius(radius).
		Background(tok.Elevated).
		Border(1, tok.BorderSubtle)
	if w > 0 {
		el.Width(w)
	} else {
		el.FillWidth()
	}
	return el
}

// SkeletonCard renders a structured loading placeholder card composed of SkeletonBlocks.
func SkeletonCard(c *ui.Context, tok theme.Tokens) ui.Element {
	if c == nil {
		return ui.Element{}
	}
	card := ui.Column(c).FillWidth().
		Padding(float32(tok.Spacing12), float32(tok.Spacing16)).
		Gap(float32(tok.Spacing12))
	ApplyElevation(card, tok, ElevationCard)
	card.Children(func() {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
			SkeletonBlock(c, tok, float32(tok.IconBoxSize), float32(tok.IconBoxSize), float32(tok.ControlRadius))
			SkeletonBlock(c, tok, float32(tok.Spacing48)*3.0, float32(tok.Spacing16), float32(tok.ControlRadius))
			ui.Spacer(c)
			SkeletonBlock(c, tok, float32(tok.Spacing48), float32(tok.Spacing16), float32(tok.PillRadius))
		})
		ui.Box(c).FillWidth().Height(1).Background(tok.HairlineDivider)
		SkeletonBlock(c, tok, 0, float32(tok.Spacing12), float32(tok.ControlRadius))
		SkeletonBlock(c, tok, float32(tok.Spacing48)*4.5, float32(tok.Spacing12), float32(tok.ControlRadius))
		SkeletonBlock(c, tok, float32(tok.Spacing48)*3.2, float32(tok.Spacing12), float32(tok.ControlRadius))
	})
	return card
}

// EmptyStateProps configures EmptyStateCard with optional keyboard shortcut hints.
type EmptyStateProps struct {
	Icon        *ui.SVG
	Title       string
	Hint        string
	ActionLabel string
	Shortcuts   []string
	OnAction    func()
}

// EmptyStateCard renders a calm Topinspo-crafted empty state with a recessed icon well,
// clear title/hint, primary action button, and optional tactile ShortcutKeycaps.
func EmptyStateCard(c *ui.Context, tok theme.Tokens, props EmptyStateProps) ui.Element {
	if c == nil {
		return ui.Element{}
	}
	card := ui.Column(c).FillWidth().
		Padding(float32(tok.Spacing24), float32(tok.Spacing16)).
		Center().
		Gap(float32(tok.Spacing8))
	ApplyElevation(card, tok, ElevationCard)
	card.Children(func() {
		if props.Icon != nil {
			well := ui.Box(c).
				Size(float32(tok.Spacing32)+float32(tok.Spacing8), float32(tok.Spacing32)+float32(tok.Spacing8)).
				Radius(float32(tok.PanelRadius)).
				Background(tok.Elevated).
				Border(1, tok.Border).
				Center()
			theme.ApplyRecessedWell(well, tok)
			well.Children(func() {
				ui.Icon(c, props.Icon).Size(float32(tok.IconLg), float32(tok.IconLg)).TextColor(tok.TextMuted)
			})
		}
		if props.Title != "" {
			ui.Text(c, props.Title).FontSize(float32(tok.TextBody)).Bold().TextColor(tok.Text)
		}
		if props.Hint != "" {
			ui.Text(c, props.Hint).FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
		}
		if props.ActionLabel != "" && props.OnAction != nil {
			btn := components.Button(c, props.ActionLabel, components.ButtonPrimary)
			theme.ApplyTactilePress(btn)
			if btn.Clicked() {
				props.OnAction()
			}
		}
		if len(props.Shortcuts) > 0 {
			ShortcutKeycaps(c, tok, props.Shortcuts...)
		}
	})
	return card
}
