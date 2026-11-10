package asterdoc

import (
	"os"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"asterlab-mygo-prototype/internal/theme"
)

const dilutionSource = "# Doc\n\n```asterlab\nkind: dilution\nid: dil-1\nschema: 1\nstock:\n  value: 20\n  unit: mg/mL\ntarget:\n  value: 2\n  unit: mg/mL\nfinal_volume:\n  value: 5\n  unit: mL\n```\n\n尾注。\n"

func TestUpdateModulePreservesProseAndOtherFences(t *testing.T) {
	doc, err := Parse(dilutionSource)
	if err != nil {
		t.Fatal(err)
	}
	source, err := doc.UpdateModule("dil-1", map[string]any{"target": map[string]any{"value": 4.0, "unit": "mg/mL"}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(source, "尾注。") {
		t.Fatalf("prose not preserved:\n%s", source)
	}
	if !strings.Contains(source, "value: 4") {
		t.Fatalf("quantity not updated:\n%s", source)
	}
}

func TestUpdateModuleRejectsInvalidQuantityUnit(t *testing.T) {
	doc, err := Parse(dilutionSource)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := doc.UpdateModule("dil-1", map[string]any{"stock": map[string]any{"value": 20.0, "unit": "not-a-unit"}}); err == nil {
		t.Fatal("invalid unit accepted")
	}
}

func TestGenericEditorRendersDilutionFieldsAndSave(t *testing.T) {
	doc, err := Parse(dilutionSource)
	if err != nil {
		t.Fatal(err)
	}
	var saved map[string]any
	v := &View{Doc: doc, Registry: BuiltinRegistry(), Allowed: map[string]bool{PermDocumentEdit: true},
		OnUpdateModule: func(id string, props map[string]any) { saved = props }}
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.LightTheme())
		v.Render(c)
	}, 1000, 900)
	tt.Frame()
	if !tt.HasText("stock") || !tt.HasText("target") || !tt.HasText("final_volume") {
		t.Fatalf("fields missing: %q", tt.Texts())
	}
	if !tt.HasText("保存") {
		t.Fatalf("save missing: %q", tt.Texts())
	}
	if err := tt.Click("保存"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if saved == nil {
		t.Fatal("OnUpdateModule not called")
	}
	_ = os.Stdout
}

func TestGenericEditorReadOnlyNoSave(t *testing.T) {
	doc, err := Parse(dilutionSource)
	if err != nil {
		t.Fatal(err)
	}
	v := &View{Doc: doc, Registry: BuiltinRegistry(), ReadOnly: true, Allowed: map[string]bool{PermDocumentEdit: true}}
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.LightTheme())
		v.Render(c)
	}, 1000, 900)
	tt.Frame()
	if tt.HasText("保存") {
		t.Fatalf("save present in read-only mode: %q", tt.Texts())
	}
}

func TestGenericEditorSavesChangedInstrumentModel(t *testing.T) {
	var saved map[string]any
	v := &View{Registry: BuiltinRegistry(), Allowed: map[string]bool{PermDocumentEdit: true},
		OnUpdateModule: func(id string, props map[string]any) { saved = props }}
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.LightTheme())
		v.GenericModuleEditor(c, "uv-1", map[string]any{
			"kind": "instrument_profile", "id": "uv-1", "schema": 1,
			"class": "uv_curing", "model": "UV-1", "source": "built_in",
			"capabilities": []any{map[string]any{"name": "365 nm"}},
		})
	}, 900, 700)
	tt.Frame()
	rect, ok := tt.Find("model")
	if !ok {
		t.Fatal("model row not found")
	}
	tt.ClickAt(rect.X+240, rect.Y+rect.H/2)
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Type("UV-2")
	tt.Frame()
	if err := tt.Click("保存"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if saved == nil || saved["model"] != "UV-2" {
		t.Fatalf("saved props = %+v", saved)
	}
}
