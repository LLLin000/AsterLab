package assays

import (
	"encoding/json"
	"testing"
)

func TestWellMappingPreservesExplicitSampleAndReplicate(t *testing.T) {
	design := map[string]any{
		"assays": []any{map[string]any{"id": "cck8", "title": "CCK-8", "timepoints": []string{"D3"}}},
		"mygo": map[string]any{"assays": map[string]any{"cck8": map[string]any{
			"timepoints": []string{"D3"},
			"wells": []any{
				map[string]any{"wellId": "A1", "sampleId": "S1", "groupId": "Control", "replicate": 1, "timepoint": "D3"},
				map[string]any{"wellId": "A2", "sampleId": "S1", "groupId": "Control", "replicate": 2, "timepoint": "D3"},
			},
		}}},
	}
	b, _ := json.Marshal(design)
	_, cells, err := BuildSchema(b, "cck8", "D3")
	if err != nil {
		t.Fatal(err)
	}
	if cells[0].SampleID != "S1" || cells[0].Group != "Control" || cells[0].Replicate != 1 {
		t.Fatalf("A1 mapping = %#v", cells[0])
	}
	if cells[1].SampleID != "S1" || cells[1].Replicate != 2 {
		t.Fatalf("A2 mapping = %#v", cells[1])
	}
}
func TestWellMappingDoesNotInferOptionalIdentity(t *testing.T) {
	design := map[string]any{
		"assays": []any{map[string]any{"id": "cck8", "title": "CCK-8", "timepoints": []string{"D3"}}},
		"mygo": map[string]any{"assays": map[string]any{"cck8": map[string]any{
			"timepoints": []string{"D3"},
			"wells": []any{
				map[string]any{"wellId": "A1", "groupId": "Control", "replicate": 0, "timepoint": "D3"},
			},
		}}},
	}
	b, _ := json.Marshal(design)
	schema, cells, err := BuildSchema(b, "cck8", "D3")
	if err != nil {
		t.Fatal(err)
	}
	if cells[0].SampleID != "" || cells[0].Replicate != 0 || cells[0].Group != "Control" {
		t.Fatalf("missing identity was inferred: %#v", cells[0])
	}
	if schema.Metadata["unresolved"] == "" {
		t.Fatal("incomplete explicit identity was reported as resolved")
	}
}

func TestWellMappingScopesTechnicalReplicateByTimepoint(t *testing.T) {
	wells := []WellAssignment{
		{WellID: "A1", SampleID: "S1", GroupID: "Control", Replicate: 1, Timepoint: "D1"},
		{WellID: "A1", SampleID: "S1", GroupID: "Control", Replicate: 1, Timepoint: "D3"},
	}
	if err := ValidateWellAssignments(wells); err != nil {
		t.Fatalf("same technical replicate at separate timepoints rejected: %v", err)
	}
	wells = append(wells, WellAssignment{
		WellID: "A2", SampleID: "S1", GroupID: "Control", Replicate: 1, Timepoint: "D1",
	})
	if err := ValidateWellAssignments(wells); err == nil {
		t.Fatal("duplicate technical replicate at the same timepoint accepted")
	}
}
