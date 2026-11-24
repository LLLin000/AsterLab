package kit

import (
	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/onevra/source/icons/lucide"

	"asterlab-mygo-prototype/internal/theme"
)

type menuState struct {
	open            bool
	below           bool
	dx, dy          float32
	focus, keyboard bool
	pointer         [2]float32
}

type menuRow struct {
	e       ui.Element
	enabled bool
}

// Menu builds items inside a ContextMenu panel.
type Menu struct {
	c       *ui.Context
	st      *menuState
	tok     theme.Tokens
	moved   bool
	pending *MenuItem
	divide  bool
	shown   bool
	items   []menuRow
}

// MenuItem configures a menu entry and reports whether it was activated.
type MenuItem struct {
	m               *Menu
	label, shortcut string
	disabled        bool
	check, checked  bool
	built, chosen   bool
}

// ContextMenu attaches a right-click / keyboard context menu to target using
// a cursor-positioned 1x1 IgnoreInput anchor rather than target itself, so left-clicks
// anywhere inside the same card/row (outside the menu bounds) dismiss it cleanly.
func ContextMenu(c *ui.Context, target ui.Element, build func(m *Menu)) {
	if c == nil || build == nil {
		return
	}
	st := ui.Local(target, "kit-context-menu", func() menuState { return menuState{} })
	if st == nil {
		return
	}
	asked := false
	target.ContextMenu(func(*ui.Menu) { asked = true })
	if asked {
		if x, y, over := target.PointerPosition(); over {
			*st = menuState{open: true, dx: x, dy: y, focus: true}
		} else {
			*st = menuState{open: true, below: true, focus: true, keyboard: true}
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
			TextColor(tok.Text).
			AttachTo(anchor, ui.AnchorTopLeft, ui.AnchorTopLeft)
		ApplyElevation(panel, tok, ElevationFloating)

		x, y, _ := panel.PointerPosition()
		moved := [2]float32{x, y} != st.pointer
		st.pointer = [2]float32{x, y}
		m := &Menu{c: c, st: st, tok: tok, moved: moved}
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

func (m *Menu) Item(label string) *MenuItem {
	m.flush()
	it := &MenuItem{m: m, label: label}
	m.pending = it
	return it
}

func (m *Menu) Separator() {
	m.flush()
	m.divide = true
}

func (m *Menu) Group(label string, build func(m *Menu)) {
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

func (m *Menu) dividing() {
	if m.divide && m.shown {
		ui.Box(m.c).FillWidth().Height(1).
			Margin(float32(m.tok.Spacing4), float32(m.tok.Spacing8), float32(m.tok.Spacing4), float32(m.tok.Spacing8)).
			Background(m.tok.BorderSubtle).Shrink(0)
	}
	m.divide = false
}

func (it *MenuItem) Disabled(d bool) *MenuItem {
	it.disabled = d
	return it
}

func (it *MenuItem) Checked(on bool) *MenuItem {
	it.check, it.checked = true, on
	return it
}

func (it *MenuItem) Shortcut(hint string) *MenuItem {
	it.shortcut = hint
	return it
}

func (it *MenuItem) Chosen() bool {
	it.m.flush()
	return it.chosen
}

func (m *Menu) flush() {
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
	m.items = append(m.items, menuRow{e: b, enabled: !it.disabled})

	hoverBg := ui.Transparent
	switch {
	case it.disabled:
		b.TextColor(tok.TextDisabled)
	case b.Hovered() && m.moved && !b.Focused():
		b.Focus()
		fallthrough
	case b.Hovered(), b.Focused():
		hoverBg = tok.GhostHover
	}
	b.Background(b.AnimateColor("kit-cm-bg", hoverBg, theme.MotionInstant))
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
			Keycap(c, tok, it.shortcut)
		}
	})
	if b.Clicked() {
		it.chosen, m.st.open = true, false
	}
}

func (m *Menu) move(step int) {
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

func (m *Menu) focusAt(i, step int) {
	for ; i >= 0 && i < len(m.items); i += step {
		if m.items[i].enabled {
			m.items[i].e.Focus()
			return
		}
	}
}
