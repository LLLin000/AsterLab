package asterdoc

import (
	"testing"

	"asterlab-mygo-prototype/internal/theme"
	"github.com/egoist/mygo/ui"
)

func TestBuiltInRenderersMatchDeclaredSurfaces(t *testing.T) {
	if err := BuiltinError(); err != nil {
		t.Fatalf("shipped registry rejected its own modules: %v", err)
	}
	registry := BuiltinRegistry()
	if got := len(registry.Descriptors()); got != 21 {
		t.Fatalf("registered module count = %d, want 21", got)
	}
	for _, spec := range registry.specs {
		for surface, rendererKey := range spec.declaredRenderers() {
			if !registry.HasRenderer(rendererKey) {
				t.Fatalf("%s@%d declares %s renderer %q without a callback",
					spec.Kind, spec.Schema, surface, rendererKey)
			}
		}
	}
	if _, ok := registry.Lookup("plate96", 1); ok {
		t.Fatal("duplicate plate96 module remains registered")
	}
}

func TestRegisterRejectsUnimplementedDeclaredRenderer(t *testing.T) {
	registry := NewRegistry()
	err := registry.Register(ModuleSpec{
		Descriptor: Descriptor{Kind: "ghost", Schema: 1, Title: "Ghost",
			Renderers: &Renderers{Design: "ghost.editor"}},
		Validate: func(map[string]any) error { return nil },
	})
	if err == nil {
		t.Fatal("unimplemented renderer accepted")
	}
	if registry.HasRenderer("ghost.editor") {
		t.Fatal("rejected module left a renderer registered")
	}
	if _, ok := registry.Lookup("ghost", 1); ok {
		t.Fatal("rejected module left a module registered")
	}
}

func TestRendererKeysAreReusableAcrossModuleKindsAndModes(t *testing.T) {
	registry := NewRegistry()
	shared := func(_ *View, c *ui.Context, _ map[string]any) { ui.Text(c, "shared renderer") }
	run := func(_ *View, c *ui.Context, _ map[string]any) { ui.Text(c, "run renderer") }
	if err := registry.RegisterRenderer("generic.editor", shared); err != nil {
		t.Fatal(err)
	}
	if err := registry.RegisterRenderer("generic.run", run); err != nil {
		t.Fatal(err)
	}
	if err := registry.RegisterRenderer("alpha.document", func(_ *View, c *ui.Context, _ map[string]any) {
		ui.Text(c, "document renderer")
	}); err != nil {
		t.Fatal(err)
	}
	validate := func(map[string]any) error { return nil }
	for _, kind := range []string{"alpha", "beta"} {
		if err := registry.Register(ModuleSpec{
			Descriptor: Descriptor{Kind: kind, Schema: 1, Title: kind,
				Renderers: &Renderers{Design: "generic.editor", Run: "generic.run"}},
			Validate: validate,
		}); err != nil {
			t.Fatalf("%s could not reuse shared keys: %v", kind, err)
		}
	}
	if err := registry.Register(ModuleSpec{
		Descriptor: Descriptor{Kind: "alpha", Schema: 2, Title: "Alpha v2",
			Renderers: &Renderers{Document: "alpha.document"}},
		Validate: validate,
	}); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		kind   string
		schema int
		mode   Mode
		want   string
	}{{"beta", 1, ModeDesign, "shared renderer"}, {"beta", 1, ModeRun, "run renderer"}, {"alpha", 2, ModeProtocol, "document renderer"}} {
		view := &View{Registry: registry, Mode: test.mode, ReadOnly: test.mode == ModeRun}
		tt := ui.NewTester(func(c *ui.Context) {
			c.SetTheme(theme.LightTheme())
			view.RenderModule(c, test.kind, test.schema, map[string]any{"id": "m1"})
		}, 500, 300)
		tt.Frame()
		if !tt.HasText(test.want) {
			t.Fatalf("%s@%d %s view did not dispatch %q: %q", test.kind, test.schema, test.mode, test.want, tt.Texts())
		}
	}
	if err := registry.RegisterRenderer("generic.run", run); err == nil {
		t.Fatal("duplicate renderer key was silently replaced")
	}
}

func TestMissingRunSurfaceDoesNotFallBackToDocumentRenderer(t *testing.T) {
	registry := NewRegistry()
	if err := registry.RegisterRenderer("document.only", func(_ *View, c *ui.Context, _ map[string]any) {
		ui.Text(c, "document only")
	}); err != nil {
		t.Fatal(err)
	}
	if err := registry.Register(ModuleSpec{
		Descriptor: Descriptor{Kind: "readme", Schema: 1, Title: "Readme",
			Renderers: &Renderers{Document: "document.only"}},
		Validate: func(map[string]any) error { return nil },
	}); err != nil {
		t.Fatal(err)
	}
	view := &View{Registry: registry, Mode: ModeRun, ReadOnly: true}
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.LightTheme())
		view.RenderModule(c, "readme", 1, map[string]any{"id": "r1"})
	}, 500, 300)
	tt.Frame()
	if tt.HasText("document only") || !tt.HasText("readme@1 · 该模块未声明此视图的渲染能力") {
		t.Fatalf("Run silently reused a document renderer: %q", tt.Texts())
	}
}

func TestGenericRunSummaryHonorsDensityAndIsReadOnly(t *testing.T) {
	registry := NewRegistry()
	if err := registry.RegisterRenderer("generic.run", func(v *View, c *ui.Context, data map[string]any) {
		v.GenericModuleSummary(c, data)
	}); err != nil {
		t.Fatal(err)
	}
	if err := registry.Register(ModuleSpec{
		Descriptor: Descriptor{Kind: "planned", Schema: 1, Title: "计划参数",
			Renderers: &Renderers{Run: "generic.run"},
			Inputs:    []Port{{Name: "a"}, {Name: "b"}, {Name: "c"}, {Name: "d"}}},
		Validate: func(map[string]any) error { return nil },
	}); err != nil {
		t.Fatal(err)
	}
	data := map[string]any{"kind": "planned", "id": "p1", "schema": 1,
		"a": "A", "b": "B", "c": "C", "d": "D"}
	for _, test := range []struct {
		density Density
		showD   bool
	}{{DensityCompact, false}, {DensityFull, true}} {
		view := &View{Registry: registry, Mode: ModeRun, Density: test.density, ReadOnly: true}
		tt := ui.NewTester(func(c *ui.Context) {
			c.SetTheme(theme.LightTheme())
			view.RenderModule(c, "planned", 1, data)
		}, 500, 400)
		tt.Frame()
		if tt.HasText("D") != test.showD {
			t.Fatalf("density %s displayed field D=%v; texts=%q", test.density, tt.HasText("D"), tt.Texts())
		}
		if tt.HasText("保存") {
			t.Fatalf("Run summary exposed an edit action: %q", tt.Texts())
		}
	}
}
