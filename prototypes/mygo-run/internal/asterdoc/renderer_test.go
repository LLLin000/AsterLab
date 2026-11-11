package asterdoc

import (
	"strings"
	"testing"

	"asterlab-mygo-prototype/internal/theme"
	"github.com/egoist/mygo/ui"
)

// The shipped registry must satisfy its own declared contract: every renderer
// key a module declares is actually implemented, and every registered module
// resolves for the surfaces the app uses.
func TestShippedRenderersAreImplemented(t *testing.T) {
	if err := BuiltinError(); err != nil {
		t.Fatalf("shipped registry rejected its own modules: %v", err)
	}
	registry := BuiltinRegistry()
	for _, spec := range registry.specs {
		if spec.Renderers == nil {
			continue
		}
		for surface, key := range spec.declaredRenderers() {
			if key == "" {
				continue
			}
			if !registry.HasRenderer(key) {
				t.Fatalf("%s@%d declares %s renderer %q but the registry does not implement it",
					spec.Kind, spec.Schema, surface, key)
			}
		}
	}
}

// A module that declares a design renderer but has no implementation must be
// rejected at registration; the host may never draw a guess.
func TestRegisterRejectsUnimplementedDeclaredRenderer(t *testing.T) {
	r := NewRegistry()
	err := r.Register(ModuleSpec{
		Descriptor: Descriptor{
			Kind: "ghost", Schema: 1, Title: "Ghost",
			Renderers: &Renderers{Design: "ghost.editor"},
		},
		Validate: func(map[string]any) error { return nil },
		Renders:  []string{"ghost.viewer"},
	})
	if err == nil {
		t.Fatal("unimplemented renderer accepted")
	}
	// Either guard may fire first (no Render, or declared-but-unimplemented);
	// what matters is that registration is refused and nothing is left behind.
	if !strings.Contains(err.Error(), "does not implement") &&
		!strings.Contains(err.Error(), "no Render implementation") {
		t.Fatalf("rejection reason is not about the implementation gap: %v", err)
	}
	if r.HasRenderer("ghost.editor") {
		t.Fatal("rejected module left renderer ownership behind")
	}
}

// Two different kinds cannot claim the same renderer key, while two schema
// versions of one kind may share a renderer family (step@1 and step@2).
func TestRegisterRendererOwnershipRules(t *testing.T) {
	r := NewRegistry()
	noop := func(*View, *ui.Context, map[string]any) {}
	validate := func(map[string]any) error { return nil }

	first := ModuleSpec{
		Descriptor: Descriptor{Kind: "alpha", Schema: 1, Title: "Alpha",
			Renderers: &Renderers{Design: "shared.editor", Document: "alpha.viewer"}},
		Validate: validate, Render: noop,
		Renders: []string{"shared.editor", "alpha.viewer"},
	}
	if err := r.Register(first); err != nil {
		t.Fatalf("first module rejected: %v", err)
	}

	// Another kind claiming shared.editor must be refused.
	if err := r.Register(ModuleSpec{
		Descriptor: Descriptor{Kind: "beta", Schema: 1, Title: "Beta",
			Renderers: &Renderers{Design: "shared.editor"}},
		Validate: validate, Render: noop,
		Renders: []string{"shared.editor"},
	}); err == nil {
		t.Fatal("cross-kind renderer conflict accepted")
	}
	if r.HasRenderer("beta.viewer") {
		t.Fatal("rejected module claimed a renderer")
	}

	// Same kind, new schema version, same family: allowed.
	if err := r.Register(ModuleSpec{
		Descriptor: Descriptor{Kind: "alpha", Schema: 2, Title: "Alpha v2",
			Renderers: &Renderers{Design: "shared.editor"}},
		Validate: validate, Render: noop,
		Renders: []string{"shared.editor"},
	}); err != nil {
		t.Fatalf("same-kind schema upgrade rejected: %v", err)
	}
	if _, ok := r.Lookup("alpha", 2); !ok {
		t.Fatal("alpha@2 not registered")
	}
}

// A headless module (no interactive renderer) is valid and reports a gap.
func TestHeadlessModuleIsAllowed(t *testing.T) {
	r := NewRegistry()
	if err := r.Register(ModuleSpec{
		Descriptor: Descriptor{Kind: "headless", Schema: 1, Title: "Headless"},
		Validate:   func(map[string]any) error { return nil },
	}); err != nil {
		t.Fatalf("headless module rejected: %v", err)
	}
	spec, ok := r.Lookup("headless", 1)
	if !ok {
		t.Fatal("headless module not registered")
	}
	if key, declared := spec.Descriptor.RendererKey(ModeDesign); declared {
		t.Fatalf("headless module declared %q", key)
	}
}

func TestRendererKeyPerMode(t *testing.T) {
	d := Descriptor{Renderers: &Renderers{Design: "x.editor", Document: "x.viewer"}}
	if k, ok := d.RendererKey(ModeDesign); !ok || k != "x.editor" {
		t.Fatalf("design key = %q %v", k, ok)
	}
	for _, mode := range []Mode{ModeRun, ModeNotebook, ModeReview, ModeProtocol} {
		if k, ok := d.RendererKey(mode); !ok || k != "x.viewer" {
			t.Fatalf("%s key = %q %v", mode, k, ok)
		}
	}
	none := Descriptor{}
	if _, ok := none.RendererKey(ModeDesign); ok {
		t.Fatal("descriptor without renderers reported a key")
	}
}

// The generic editor is opt-in by capability; spatial modules keep their own.
func TestGenericEditorCapabilityGate(t *testing.T) {
	registry := BuiltinRegistry()
	plate, _ := registry.Lookup("plate", 1)
	if hasCapability(plate.Descriptor, "generic-editor") {
		t.Fatal("plate must keep its own editor, not the generic one")
	}
	dil, _ := registry.Lookup("dilution", 1)
	if !hasCapability(dil.Descriptor, "generic-editor") {
		t.Fatal("dilution should opt into the generic editor")
	}
}

// Run's parsed-module path must enforce the same surface declaration as the
// Markdown path. A document-only viewer is a gap in Design and callable in Run.
func TestRenderModuleHonorsDeclaredSurface(t *testing.T) {
	registry := NewRegistry()
	if err := registry.Register(ModuleSpec{
		Descriptor: Descriptor{
			Kind: "viewer_only", Schema: 1, Title: "Viewer only",
			Renderers: &Renderers{Document: "viewer_only.viewer"},
		},
		Validate: func(map[string]any) error { return nil },
		Render: func(_ *View, c *ui.Context, _ map[string]any) {
			ui.Text(c, "document renderer called")
		},
		Renders: []string{"viewer_only.viewer"},
	}); err != nil {
		t.Fatal(err)
	}

	design := &View{Registry: registry, Mode: ModeDesign}
	designTest := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.LightTheme())
		design.RenderModule(c, "viewer_only", 1, map[string]any{"id": "v1"})
	}, 800, 500)
	designTest.Frame()
	if designTest.HasText("document renderer called") ||
		!designTest.HasText("viewer_only@1 · 该模块未声明此视图的渲染能力") {
		t.Fatalf("Design did not report the missing surface: %q", designTest.Texts())
	}

	run := &View{Registry: registry, Mode: ModeRun, ReadOnly: true}
	runTest := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.LightTheme())
		run.RenderModule(c, "viewer_only", 1, map[string]any{"id": "v1"})
	}, 800, 500)
	runTest.Frame()
	if !runTest.HasText("document renderer called") {
		t.Fatalf("Run viewer was not dispatched: %q", runTest.Texts())
	}
}
