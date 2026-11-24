package asterdoc

import (
	"github.com/egoist/mygo/plugins/glass"
	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/onevra/source/icons/lucide"

	"asterlab-mygo-prototype/internal/theme"
)

type asterMenuState struct {
	open            bool
	below           bool
	dx, dy          float32
	focus, keyboard bool
	pointer         [2]float32
}

type asterMenuRow struct {
	e       ui.Element
	enabled bool
}

type asterMenu struct {
	c       *ui.Context
	st      *asterMenuState
	tok     theme.Tokens
	moved   bool
	pending *asterMenuItem
	divide  bool
	shown   bool
	items   []asterMenuRow
}

type asterMenuItem struct {
	m               *asterMenu
	label, shortcut string
	disabled        bool
	check, checked  bool
	built, chosen   bool
}

// asterContextMenu attaches a right-click / keyboard context menu to target using
// a cursor-positioned 1x1 invisible anchor rather than target itself, so left-clicks
// anywhere inside the same component card (outside the menu bounds) dismiss it cleanly.
func asterContextMenu(c *ui.Context, target ui.Element, build func(m *asterMenu)) {
	if c == nil || build == nil {
		return
	}
	st := ui.Local(target, "asterdoc-context-menu", func() asterMenuState { return asterMenuState{} })
	if st == nil {
		return
	}
	asked := false
	target.ContextMenu(func(*ui.Menu) { asked = true })
	if asked {
		if x, y, over := target.PointerPosition(); over {
			*st = asterMenuState{open: true, dx: x, dy: y, focus: true}
		} else {
			*st = asterMenuState{open: true, below: true, focus: true, keyboard: true}
		}
	}
	if !st.open {
		return
	}
	if target.IsDisabled() {
		st.open = false
		return
	}

	tok := theme.GetTokens(c.Theme())
	anchor := ui.Box(c.Key(st)).Size(1, 1).IgnoreInput(true)
	if st.below {
		anchor.AttachTo(target, ui.AnchorBottomStart, ui.AnchorTopStart)
	} else {
		anchor.AttachTo(target, ui.AnchorTopLeft, ui.AnchorTopLeft).Margin(st.dy, 0, 0, st.dx)
	}

	ui.PopoverBase(c, anchor, &st.open, func(panel ui.Element) {
		panel.Column().
			Role(ui.RoleMenu).Focusable().
			MinWidth(180).MaxWidth(360).
			Padding(float32(tok.Spacing4)).Gap(1).
			Radius(float32(tok.PanelRadius)).
			Material(glass.Glass{Tint: tok.Surface.Alpha(0.96)}).
			TextColor(tok.Text).
			Border(1, tok.Border).
			Shadow(0, float32(tok.Spacing8), float32(tok.Spacing24), 0, tok.ShadowAmbient).
			AttachTo(anchor, ui.AnchorTopLeft, ui.AnchorTopLeft)

		x, y, _ := panel.PointerPosition()
		moved := [2]float32{x, y} != st.pointer
		st.pointer = [2]float32{x, y}
		m := &asterMenu{c: c, st: st, tok: tok, moved: moved}
		panel.Children(func() {
			build(m)
			m.flush()
		})
		switch {
		case !st.open:
		case panel.Shortcut(0, ui.KeyDown):
			m.move(1)
		case panel.Shortcut(0, ui.KeyUp):
			m.move(-1)
		case panel.Shortcut(0, ui.KeyHome):
			m.focusAt(0, 1)
		case panel.Shortcut(0, ui.KeyEnd):
			m.focusAt(len(m.items)-1, -1)
		case st.focus && st.keyboard:
			m.focusAt(0, 1)
		case st.focus:
			panel.Focus()
		}
		st.focus = false
	})
}

func (m *asterMenu) Item(label string) *asterMenuItem {
	m.flush()
	it := &asterMenuItem{m: m, label: label}
	m.pending = it
	return it
}

func (m *asterMenu) Separator() {
	m.flush()
	m.divide = true
}

func (m *asterMenu) Group(label string, build func(m *asterMenu)) {
	m.flush()
	m.dividing()
	ui.Text(m.c, label).
		FontSize(float32(m.tok.TextMeta)).
		TextColor(m.tok.TextMuted).
		Padding(float32(m.tok.Spacing8), float32(m.tok.Spacing8), float32(m.tok.Spacing4), float32(m.tok.Spacing8)).
		SingleLine().Ellipsis("…")
	m.shown = true
	build(m)
	m.flush()
}

func (m *asterMenu) dividing() {
	if m.divide && m.shown {
		ui.Box(m.c).FillWidth().Height(1).
			Margin(float32(m.tok.Spacing4), float32(m.tok.Spacing8), float32(m.tok.Spacing4), float32(m.tok.Spacing8)).
			Background(m.tok.BorderSubtle).Shrink(0)
	}
	m.divide = false
}

func (it *asterMenuItem) Disabled(d bool) *asterMenuItem {
	it.disabled = d
	return it
}

func (it *asterMenuItem) Checked(on bool) *asterMenuItem {
	it.check, it.checked = true, on
	return it
}

func (it *asterMenuItem) Shortcut(hint string) *asterMenuItem {
	it.shortcut = hint
	return it
}

func (it *asterMenuItem) Chosen() bool {
	it.m.flush()
	return it.chosen
}

func (m *asterMenu) flush() {
	it := m.pending
	if it == nil || it.built {
		return
	}
	m.pending, it.built = nil, true
	m.dividing()
	m.shown = true
	c, tok := m.c, m.tok
	b := ui.ButtonBase(c).Key(len(m.items)).Label(it.label).Disabled(it.disabled).FillWidth().MinWidth(0).
		Padding(float32(tok.Spacing4)+2, float32(tok.Spacing8)).
		Radius(float32(tok.ControlRadius)).
		Justify(ui.Start).AlignItems(ui.Center).Gap(float32(tok.Spacing8)).
		TextColor(tok.Text)
	if it.check {
		b.Role(ui.RoleMenuItemCheckBox).Checked(it.checked)
	} else {
		b.Role(ui.RoleMenuItem)
	}
	m.items = append(m.items, asterMenuRow{e: b, enabled: !it.disabled})

	hoverBg := ui.Transparent
	switch {
	case it.disabled:
		b.TextColor(tok.TextMuted)
	case b.Hovered() && m.moved && !b.Focused():
		b.Focus()
		fallthrough
	case b.Hovered(), b.Focused():
		hoverBg = tok.Text.Alpha(0.06)
	}
	b.Background(b.AnimateColor("acm-bg", hoverBg, theme.MotionInstant))
	b.Children(func() {
		if it.check {
			mark := ui.Box(c).Size(float32(tok.Spacing12), float32(tok.Spacing12)).Shrink(0)
			if it.checked {
				mark.Children(func() {
					ui.Icon(c, lucide.Check()).Size(float32(tok.Spacing12), float32(tok.Spacing12)).TextColor(tok.Accent)
				})
			}
		}
		ui.Text(c, it.label).Grow(1).MinWidth(0).SingleLine().Ellipsis("…").FontSize(float32(tok.TextData))
		if it.shortcut != "" {
			ui.Text(c, it.shortcut).Shrink(0).FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted).MarginStart(float32(tok.Spacing16))
		}
	})
	if b.Clicked() {
		it.chosen, m.st.open = true, false
	}
}

func (m *asterMenu) move(step int) {
	at := -1
	for i, r := range m.items {
		if r.e.Focused() {
			at = i
		}
	}
	if at < 0 {
		if step > 0 {
			m.focusAt(0, 1)
		} else {
			m.focusAt(len(m.items)-1, -1)
		}
		return
	}
	n := len(m.items)
	for i := 1; i <= n; i++ {
		if r := m.items[((at+step*i)%n+n)%n]; r.enabled {
			r.e.Focus()
			return
		}
	}
}

func (m *asterMenu) focusAt(i, step int) {
	for ; i >= 0 && i < len(m.items); i += step {
		if m.items[i].enabled {
			m.items[i].e.Focus()
			return
		}
	}
}
