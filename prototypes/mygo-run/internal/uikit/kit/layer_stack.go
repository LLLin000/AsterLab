package kit

import "github.com/egoist/mygo/ui"

// LayerEntry identifies one active modal/popover/drawer layer and how to dismiss it.
type LayerEntry struct {
	ID        string
	OnDismiss func()
}

// LayerStack tracks open overlay layers in paint order (bottom -> top) so global
// Escape handling strictly dismisses only the topmost active layer (Ariakit/React-Aria pattern).
type LayerStack struct {
	entries []LayerEntry
}

// BeginFrame resets the frame's open-layer stack before overlays register themselves.
func (s *LayerStack) BeginFrame() {
	if s == nil {
		return
	}
	s.entries = s.entries[:0]
}

// Register appends an overlay layer when open is true; safe when onDismiss is nil.
func (s *LayerStack) Register(id string, open bool, onDismiss func()) {
	if s == nil || !open || id == "" {
		return
	}
	for i := range s.entries {
		if s.entries[i].ID == id {
			s.entries[i].OnDismiss = onDismiss
			return
		}
	}
	s.entries = append(s.entries, LayerEntry{ID: id, OnDismiss: onDismiss})
}

// HasOpen reports whether at least one registered overlay layer is open.
func (s *LayerStack) HasOpen() bool {
	return s != nil && len(s.entries) > 0
}

// TopID returns the ID of the topmost open overlay layer, or "" when empty.
func (s *LayerStack) TopID() string {
	if s == nil || len(s.entries) == 0 {
		return ""
	}
	return s.entries[len(s.entries)-1].ID
}

// DismissTop pops and dismisses the topmost registered overlay layer.
func (s *LayerStack) DismissTop() bool {
	if s == nil || len(s.entries) == 0 {
		return false
	}
	top := s.entries[len(s.entries)-1]
	s.entries = s.entries[:len(s.entries)-1]
	if top.OnDismiss != nil {
		top.OnDismiss()
	}
	return true
}

// HandleEscape dismisses the topmost open overlay when Escape is pressed and returns true if consumed.
func (s *LayerStack) HandleEscape(c *ui.Context) bool {
	if s == nil || len(s.entries) == 0 || c == nil {
		return false
	}
	if !c.Shortcut(0, ui.KeyEscape) {
		return false
	}
	return s.DismissTop()
}
