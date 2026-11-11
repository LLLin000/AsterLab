package main

import (
	"encoding/json"
	"strings"
	"testing"

	"asterlab-mygo-prototype/internal/workflowtree"
)

func TestInspectTreeContractJSON(t *testing.T) {
	tree := &workflowtree.Tree{
		Design: true,
		Roots: []*workflowtree.Node{{
			ID: "step-rna-extract", Path: "root/rna-extraction", Kind: workflowtree.KindStep,
			Identity: workflowtree.IdentityUse, UseID: "root/rna-extraction",
			BlockVersionID: "v1", DefinitionRef: "v1", Composite: true, Total: 1,
			Children: []*workflowtree.Node{{
				ID: "root/rna-extraction/01", Path: "root/rna-extraction/01",
				Identity: workflowtree.IdentityDefinition, Instruction: "加 500 µL",
				Parameters: []workflowtree.BlockParameter{{Key: "volume", Label: "体积", Value: "500", Unit: "µL"}},
			}},
		}},
		Order: []string{"step-rna-extract"}, Precedes: map[string][]string{},
	}
	raw, err := json.Marshal(tree)
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, want := range []string{`"useId":"root/rna-extraction"`, `"instruction":"加 500 µL"`, `"parameters"`, `"design":true`} {
		if !strings.Contains(text, want) {
			t.Fatalf("contract missing %s: %s", want, text)
		}
	}
}

func TestEnrichRunTreeKeepsOccurrenceIdentity(t *testing.T) {
	design := &workflowtree.Tree{Roots: []*workflowtree.Node{{
		Path: "root/rna-extraction", UseID: "root/rna-extraction",
		Children: []*workflowtree.Node{{
			Path: "root/rna-extraction/01", UseID: "root/rna-extraction/01",
			Instruction: "加 500 µL", Parameters: []workflowtree.BlockParameter{{Key: "volume", Value: "500"}},
		}},
	}}}
	run := &workflowtree.Tree{Roots: []*workflowtree.Node{{
		Path: "root/rna-extraction", UseID: "root/rna-extraction",
		Children: []*workflowtree.Node{{
			Path: "root/rna-extraction/01", Identity: workflowtree.IdentityOccurrence,
			TaskKey: "task-1", Status: "completed",
		}},
	}}}
	enrichRunTree(run, design)
	child := run.Roots[0].Children[0]
	if child.Identity != workflowtree.IdentityOccurrence || child.TaskKey != "task-1" || child.Status != "completed" {
		t.Fatalf("occurrence identity changed: %+v", child)
	}
	if child.Instruction != "加 500 µL" || len(child.Parameters) != 1 || child.UseID != "root/rna-extraction/01" {
		t.Fatalf("definition metadata missing: %+v", child)
	}
}
