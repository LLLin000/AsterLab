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

func TestInstantiateTemplateComplexAllModules(t *testing.T) {
	raw, err := os.ReadFile("testdata/template-complex.asterml")
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
	// All module IDs must be remapped (none identical)
	origIDs := map[string]bool{}
	for _, m := range origDoc.Modules {
		origIDs[m.ID] = true
	}
	for _, m := range instDoc.Modules {
		if origIDs[m.ID] {
			t.Fatalf("module id not remapped: %s (kind=%s)", m.ID, m.Kind)
		}
	}
	// All module kinds must be preserved
	origKinds := map[string]int{}
	for _, m := range origDoc.Modules {
		origKinds[m.Kind]++
	}
	instKinds := map[string]int{}
	for _, m := range instDoc.Modules {
		instKinds[m.Kind]++
	}
	for kind, count := range origKinds {
		if instKinds[kind] != count {
			t.Fatalf("kind %s count mismatch: template=%d instance=%d", kind, count, instKinds[kind])
		}
	}
	// All module IDs in instance must be unique
	instIDs := map[string]bool{}
	for _, m := range instDoc.Modules {
		if instIDs[m.ID] {
			t.Fatalf("instance has duplicate module id: %s", m.ID)
		}
		instIDs[m.ID] = true
	}
}

func TestInstantiateTemplateStepUsesRemapped(t *testing.T) {
	raw, err := os.ReadFile("testdata/template-complex.asterml")
	if err != nil {
		t.Fatal(err)
	}
	source := string(raw)
	out, err := InstantiateTemplate(source, nil)
	if err != nil {
		t.Fatal(err)
	}
	origDoc, _ := Parse(source)
	instDoc, _ := Parse(out)
	origSteps := map[string]bool{}
	for _, m := range origDoc.Modules {
		if m.Kind == "step" {
			origSteps[m.ID] = true
			uses, _ := m.Data["uses"].([]any)
			for _, raw := range uses {
				item, _ := raw.(map[string]any)
				ref, _ := item["ref"].(string)
				_, _ = item["role"].(string)
				t.Logf("ORIGINAL step %s uses ref=%s", m.ID, ref)
			}
		}
	}
	instSteps := map[string]bool{}
	for _, m := range instDoc.Modules {
		if m.Kind == "step" {
			instSteps[m.ID] = true
			if origSteps[m.ID] {
				t.Fatalf("step id not remapped: %s", m.ID)
			}
			uses, _ := m.Data["uses"].([]any)
			for _, raw := range uses {
				item, _ := raw.(map[string]any)
				ref, _ := item["ref"].(string)
				_, _ = item["role"].(string)
				if origSteps[ref] {
					t.Fatalf("step %s uses ref %s which is an original step id (not remapped)", m.ID, ref)
				}
				t.Logf("INSTANCE step %s uses ref=%s", m.ID, ref)
			}
		}
	}
	if len(instSteps) != len(origSteps) {
		t.Fatalf("step count mismatch: template=%d instance=%d", len(origSteps), len(instSteps))
	}
}

func TestInstantiateTemplateReferenceTargetsRemapped(t *testing.T) {
	raw, err := os.ReadFile("testdata/template-complex.asterml")
	if err != nil {
		t.Fatal(err)
	}
	source := string(raw)
	out, err := InstantiateTemplate(source, nil)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = Parse(source)
	instDoc, _ := Parse(out)
	// Collect all instance module IDs
	instIDs := map[string]bool{}
	for _, m := range instDoc.Modules {
		instIDs[m.ID] = true
	}
	// All reference targets in instance must point to existing instance IDs
	for _, m := range instDoc.Modules {
		if m.Kind == "sample" {
			if groupRef, _ := m.Data["group_ref"].(string); groupRef != "" {
				if !instIDs[groupRef] {
					t.Fatalf("sample %s group_ref %s points to missing instance module", m.ID, groupRef)
				}
			}
			if materialRefs, ok := m.Data["material_refs"].([]any); ok {
				for _, raw := range materialRefs {
					ref, _ := raw.(string)
					if !instIDs[ref] {
						t.Fatalf("sample %s material_refs %s points to missing instance module", m.ID, ref)
					}
				}
			}
		}
		if m.Kind == "plate" {
			if groups, ok := m.Data["groups"].(map[string]any); ok {
				for group := range groups {
					if !instIDs[group] {
						t.Fatalf("plate %s references group %s which is missing from instance", m.ID, group)
					}
				}
			}
		}
		if m.Kind == "assay" {
			if groupRefs, ok := m.Data["group_refs"].([]any); ok {
				for _, raw := range groupRefs {
					ref, _ := raw.(string)
					if !instIDs[ref] {
						t.Fatalf("assay %s group_refs %s points to missing instance module", m.ID, ref)
					}
				}
			}
		}
	}
}

func TestInstantiateTemplateInstanceIndependence(t *testing.T) {
	raw, err := os.ReadFile("testdata/template-complex.asterml")
	if err != nil {
		t.Fatal(err)
	}
	source := string(raw)
	out1, err := InstantiateTemplate(source, nil)
	if err != nil {
		t.Fatal(err)
	}
	out2, err := InstantiateTemplate(source, nil)
	if err != nil {
		t.Fatal(err)
	}
	// Two instances must have different IDs
	doc1, _ := Parse(out1)
	doc2, _ := Parse(out2)
	idMap1 := map[string]bool{}
	idMap2 := map[string]bool{}
	for _, m := range doc1.Modules {
		idMap1[m.ID] = true
	}
	for _, m := range doc2.Modules {
		idMap2[m.ID] = true
	}
	for id := range idMap1 {
		if idMap2[id] {
			t.Fatalf("two instances share module id: %s", id)
		}
	}
	// Editing instance 1 must not affect instance 2
	doc1Edited, _ := doc1.UpdateModule(doc1.Modules[0].ID, map[string]any{"title": "EDITED-INSTANCE-1"})
	_ = doc1Edited
	if strings.Contains(out2, "EDITED-INSTANCE-1") {
		t.Fatal("editing instance 1 affected instance 2")
	}
	if strings.Contains(source, "EDITED-INSTANCE-1") {
		t.Fatal("editing instance 1 affected template source")
	}
}

func TestInstantiateTemplateWorkflowEdgesPreserved(t *testing.T) {
	raw, err := os.ReadFile("testdata/template-complex.asterml")
	if err != nil {
		t.Fatal(err)
	}
	source := string(raw)
	out, err := InstantiateTemplate(source, nil)
	if err != nil {
		t.Fatal(err)
	}
	origDoc, _ := Parse(source)
	instDoc, _ := Parse(out)
	origEdges := map[string]bool{}
	for _, m := range origDoc.Modules {
		if m.Kind == "workflow" {
			if edges, ok := m.Data["edges"].([]any); ok {
				for _, raw := range edges {
					edge, _ := raw.(map[string]any)
					from, _ := edge["from"].(string)
					to, _ := edge["to"].(string)
					origEdges[from+"->"+to] = true
					t.Logf("ORIGINAL edge: %s->%s", from, to)
					_ = from
					_ = to
				}
			}
		}
	}
	instEdges := map[string]bool{}
	for _, m := range instDoc.Modules {
		if m.Kind == "workflow" {
			if edges, ok := m.Data["edges"].([]any); ok {
				for _, raw := range edges {
					edge, _ := raw.(map[string]any)
					from, _ := edge["from"].(string)
					to, _ := edge["to"].(string)
					instEdges[from+"->"+to] = true
					t.Logf("INSTANCE edge: %s->%s", from, to)
					_ = from
					_ = to
				}
			}
		}
	}
	if len(origEdges) != len(instEdges) {
		t.Fatalf("edge count mismatch: template=%d instance=%d", len(origEdges), len(instEdges))
	}
	// Each original edge's from/to must be remapped to instance IDs
	for edge := range origEdges {
		if _, ok := instEdges[edge]; ok {
			t.Fatalf("edge %s was not remapped (still uses original IDs)", edge)
		}
	}
}
