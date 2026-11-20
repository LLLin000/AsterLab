package shell

import (
	"strings"

	"github.com/egoist/mygo/ui"
)

type parsedHotkey struct {
	mods ui.Modifiers
	key  ui.Key
}

// parseHotkeySpec parses one or more slash-separated shortcut specs like
// "Ctrl+K / Ctrl+O", "Ctrl+Shift+R", "Alt+←", "Ctrl+\\", "F2", "Del".
func parseHotkeySpec(spec string) []parsedHotkey {
	parts := strings.Split(spec, "/")
	var out []parsedHotkey
	for _, raw := range parts {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		if hk, ok := parseSingleHotkey(raw); ok {
			out = append(out, hk)
		}
	}
	return out
}

func parseSingleHotkey(raw string) (parsedHotkey, bool) {
	tokens := strings.Split(raw, "+")
	var mods ui.Modifiers
	var keyToken string
	for _, tok := range tokens {
		tok = strings.TrimSpace(tok)
		if tok == "" {
			continue
		}
		switch strings.ToLower(tok) {
		case "ctrl", "cmd", "mod", "command", "control":
			mods |= ui.Cmd
		case "shift":
			mods |= ui.Shift
		case "alt", "opt", "option":
			mods |= ui.Alt
		case "super", "meta", "win":
			mods |= ui.Super
		default:
			keyToken = tok
		}
	}
	if keyToken == "" {
		return parsedHotkey{}, false
	}
	k, ok := lookupUIKey(keyToken)
	if !ok {
		return parsedHotkey{}, false
	}
	return parsedHotkey{mods: mods, key: k}, true
}

func lookupUIKey(tok string) (ui.Key, bool) {
	if len(tok) == 1 {
		ch := strings.ToUpper(tok)[0]
		if ch >= 'A' && ch <= 'Z' {
			return ui.KeyA + ui.Key(ch-'A'), true
		}
		if ch >= '0' && ch <= '9' {
			return ui.Key0 + ui.Key(ch-'0'), true
		}
		switch ch {
		case '\\':
			return ui.KeyBackslash, true
		case ',':
			return ui.KeyComma, true
		case '.':
			return ui.KeyPeriod, true
		case '/':
			return ui.KeySlash, true
		case ';':
			return ui.KeySemicolon, true
		case '-':
			return ui.KeyMinus, true
		case '=':
			return ui.KeyEqual, true
		case '[':
			return ui.KeyBracketLeft, true
		case ']':
			return ui.KeyBracketRight, true
		}
	}
	switch strings.ToLower(tok) {
	case "←", "left", "arrowleft":
		return ui.KeyLeft, true
	case "→", "right", "arrowright":
		return ui.KeyRight, true
	case "↑", "up", "arrowup":
		return ui.KeyUp, true
	case "↓", "down", "arrowdown":
		return ui.KeyDown, true
	case "esc", "escape":
		return ui.KeyEscape, true
	case "enter", "return":
		return ui.KeyEnter, true
	case "tab":
		return ui.KeyTab, true
	case "space":
		return ui.KeySpace, true
	case "backspace":
		return ui.KeyBackspace, true
	case "del", "delete":
		return ui.KeyDelete, true
	case "home":
		return ui.KeyHome, true
	case "end":
		return ui.KeyEnd, true
	case "pageup", "pgup":
		return ui.KeyPageUp, true
	case "pagedown", "pgdn":
		return ui.KeyPageDown, true
	case "f1":
		return ui.KeyF1, true
	case "f2":
		return ui.KeyF2, true
	case "f3":
		return ui.KeyF3, true
	case "f4":
		return ui.KeyF4, true
	case "f5":
		return ui.KeyF5, true
	case "f6":
		return ui.KeyF6, true
	case "f7":
		return ui.KeyF7, true
	case "f8":
		return ui.KeyF8, true
	case "f9":
		return ui.KeyF9, true
	case "f10":
		return ui.KeyF10, true
	case "f11":
		return ui.KeyF11, true
	case "f12":
		return ui.KeyF12, true
	}
	return 0, false
}

// matchHotkey checks whether any shortcut bound to commandID (customized in Settings
// or default fallback) fired on c this frame.
func (s *Shell) matchHotkey(c *ui.Context, commandID string, fallback ...parsedHotkey) bool {
	spec := s.HotkeyDisplay(commandID)
	parsed := parseHotkeySpec(spec)
	if len(parsed) == 0 {
		parsed = fallback
	}
	for _, hk := range parsed {
		if c.Shortcut(hk.mods, hk.key) {
			return true
		}
	}
	return false
}
