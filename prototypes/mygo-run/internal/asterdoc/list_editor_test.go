package asterdoc

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"asterlab-mygo-prototype/internal/theme"
)

// A string list is the most common composite field (timepoints, group refs,
// gene lists). The generic editor must let an author add, edit and remove
// entries and report the changed list to the host command seam.
func TestGenericEditorEditsStringList(t *testing.T) {
	var saved map[string]any
	v := &View{
		Mode:           ModeDesign,
		Allowed:        map[string]bool{PermDocumentEdit: true},
		OnUpdateModule: func(_ string, props map[string]any) { saved = props },
	}
	data := map[string]any{
		"kind": "step", "id": "s1", "schema": 1, "title": "Seed",
		"assay": "a1", "timepoints": []any{"D1"},
	}
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.LightTheme())
		v.GenericModuleEditor(c, "s1", data)
	}, 900, 700)
	tt.Frame()

	if err := tt.Click("+ 添加"); err != nil {
		t.Fatalf("add control missing: %v", err)
	}
	tt.Frame()
	items := v.moduleLists["s1"]["timepoints"]
	if len(items) != 2 {
		t.Fatalf("add did not extend the list: %#v", items)
	}
	v.moduleLists["s1"]["timepoints"][1] = "D3"
	tt.Frame()

	if err := tt.Click("保存"); err != nil {
		t.Fatalf("save control missing: %v", err)
	}
	tt.Frame()
	got, ok := saved["timepoints"].([]any)
	if !ok || len(got) != 2 || got[0] != "D1" || got[1] != "D3" {
		t.Fatalf("saved timepoints = %#v (props %#v)", got, saved)
	}
}

func TestGenericEditorLeavesUnchangedListOutOfSave(t *testing.T) {
	var saved map[string]any
	v := &View{
		Mode:           ModeDesign,
		Allowed:        map[string]bool{PermDocumentEdit: true},
		OnUpdateModule: func(_ string, props map[string]any) { saved = props },
	}
	data := map[string]any{
		"kind": "step", "id": "s1", "schema": 1, "title": "Seed",
		"assay": "a1", "timepoints": []any{"D1", "D3"},
	}
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.LightTheme())
		v.GenericModuleEditor(c, "s1", data)
	}, 900, 700)
	tt.Frame()
	if err := tt.Click("保存"); err != nil {
		t.Fatalf("save control missing: %v", err)
	}
	tt.Frame()
	if _, present := saved["timepoints"]; present {
		t.Fatalf("unchanged list was written: %#v", saved)
	}
}
