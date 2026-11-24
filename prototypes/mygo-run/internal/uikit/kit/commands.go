package kit

import (
	"strings"

	"github.com/egoist/mygo/ui"
)

// CommandContext snapshots focus & workspace state for VS Code / Zed style `when` clause gating.
type CommandContext struct {
	ModalOpen        bool
	TextInputFocused bool
	ActivePage       string // "file" | "calendar" | "pantry" | ""
	IsFileTab        bool
	ModeLab          bool
	ModeSource       bool
	HasSelection     bool
}

// CommandSpec is the single source of truth for one workspace/tool command across
// global hotkeys, the Command Palette, toolbar tooltips, and settings.
type CommandSpec struct {
	ID              string
	Title           string
	Subtitle        string
	Category        string
	DefaultShortcut string
	Icon            *ui.SVG
	When            func(CommandContext) bool
	Run             func()
}

// CommandRegistry holds deduplicated workspace commands and optional user shortcut overrides.
type CommandRegistry struct {
	specs     []CommandSpec
	byID      map[string]int
	overrides map[string]string
}

// NewCommandRegistry allocates an empty CommandRegistry.
func NewCommandRegistry() *CommandRegistry {
	return &CommandRegistry{
		byID:      map[string]int{},
		overrides: map[string]string{},
	}
}

// Register adds or updates a command by ID while preserving registration order.
func (r *CommandRegistry) Register(spec CommandSpec) {
	if r == nil || spec.ID == "" {
		return
	}
	if r.byID == nil {
		r.byID = map[string]int{}
	}
	if idx, ok := r.byID[spec.ID]; ok {
		r.specs[idx] = spec
		return
	}
	r.byID[spec.ID] = len(r.specs)
	r.specs = append(r.specs, spec)
}

// SetOverrides stores user-configured shortcut strings keyed by command ID.
func (r *CommandRegistry) SetOverrides(m map[string]string) {
	if r == nil {
		return
	}
	r.overrides = map[string]string{}
	for k, v := range m {
		if trimmed := strings.TrimSpace(v); trimmed != "" {
			r.overrides[k] = trimmed
		}
	}
}

// ShortcutFor returns the effective shortcut display string (override first, else DefaultShortcut).
func (r *CommandRegistry) ShortcutFor(id string) string {
	if r == nil {
		return ""
	}
	if ov, ok := r.overrides[id]; ok && ov != "" {
		return ov
	}
	if idx, ok := r.byID[id]; ok {
		return r.specs[idx].DefaultShortcut
	}
	return ""
}

// CanRun evaluates the command's `when` predicate against ctx.
func (r *CommandRegistry) CanRun(spec CommandSpec, ctx CommandContext) bool {
	if spec.When == nil {
		return true
	}
	return spec.When(ctx)
}

// Search filters registered commands by context availability and case-insensitive substring query.
func (r *CommandRegistry) Search(query string, ctx CommandContext) []CommandSpec {
	if r == nil {
		return nil
	}
	q := strings.ToLower(strings.TrimSpace(query))
	out := make([]CommandSpec, 0, len(r.specs))
	for _, spec := range r.specs {
		if !r.CanRun(spec, ctx) {
			continue
		}
		if q == "" {
			out = append(out, spec)
			continue
		}
		hay := strings.ToLower(spec.ID + " " + spec.Title + " " + spec.Subtitle + " " + spec.Category + " " + r.ShortcutFor(spec.ID))
		if strings.Contains(hay, q) {
			out = append(out, spec)
		}
	}
	return out
}

// All returns a shallow slice copy of all registered commands.
func (r *CommandRegistry) All() []CommandSpec {
	if r == nil {
		return nil
	}
	out := make([]CommandSpec, len(r.specs))
	copy(out, r.specs)
	return out
}

// Find looks up a command specification by ID.
func (r *CommandRegistry) Find(id string) (CommandSpec, bool) {
	if r == nil {
		return CommandSpec{}, false
	}
	idx, ok := r.byID[id]
	if !ok {
		return CommandSpec{}, false
	}
	return r.specs[idx], true
}
