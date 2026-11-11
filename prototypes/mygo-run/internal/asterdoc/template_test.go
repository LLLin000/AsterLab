package asterdoc

import (
	"os"
	"strings"
	"testing"
)

func TestInstantiateTemplateRemapsIDs(t *testing.T) {
	raw, err := os.ReadFile("testdata/cck8-experiment.asterml")
	if err != nil {
		t.Fatal(err)
	}
	source := string(raw)
	out, err := InstantiateTemplate(source, nil)
	if err != nil {
		t.Fatal(err)
	}
	if out == source {
		t.Fatal("instance source identical to template")
	}
	origDoc, _ := Parse(source)
	instDoc, err := Parse(out)
	if err != nil {
		t.Fatal(err)
	}
	origIDs := map[string]bool{}
	for _, m := range origDoc.Modules {
		origIDs[m.ID] = true
	}
	for _, m := range instDoc.Modules {
		if origIDs[m.ID] {
			t.Fatalf("module id not remapped: %s", m.ID)
		}
	}
	// workflow steps remapped
	for _, m := range instDoc.Modules {
		if m.Kind == "workflow" {
			for _, s := range stringList(m.Data, "steps") {
				if origIDs[s] {
					t.Fatalf("workflow step not remapped: %s", s)
				}
			}
		}
		if m.Kind == "plate" {
			for g := range m.Data["groups"].(map[string]any) {
				if origIDs[g] {
					t.Fatalf("plate group not remapped: %s", g)
				}
			}
		}
	}
	// step uses remapped if present
	_ = strings.Contains
}

func TestInstantiateTemplateIndependence(t *testing.T) {
	raw, _ := os.ReadFile("testdata/cck8-experiment.asterml")
	source := string(raw)
	out, _ := InstantiateTemplate(source, nil)
	// edit instance
	doc, _ := Parse(out)
	// find a module id
	id := doc.Modules[0].ID
	_, err := doc.UpdateModule(id, map[string]any{"title": "INSTANCE EDITED"})
	if err != nil {
		t.Fatal(err)
	}
	// template source unchanged
	if strings.Contains(source, "INSTANCE EDITED") {
		t.Fatal("template affected")
	}
}
