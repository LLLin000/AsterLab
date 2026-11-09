package assays

import (
	"encoding/json"
	"testing"
)

func TestWellMappingPopulatesSampleGroupReplicate(t *testing.T) {
	design := map[string]any{
		"assays": []any{map[string]any{"id": "cck8", "title": "CCK-8", "timepoints": []string{"D3"}}},
		"mygo": map[string]any{"assays": map[string]any{"cck8": map[string]any{
			"timepoints": []string{"D3"},
			"wells": []any{
				map[string]any{"wellId": "A1", "sampleId": "Control-1", "groupId": "Control", "replicate": 1, "timepoint": "D3"},
				map[string]any{"wellId": "A2", "sampleId": "Control-1", "groupId": "Control", "replicate": 2, "timepoint": "D3"},
			},
		}}},
	}
	b, _ := json.Marshal(design)
	schema, cells, err := BuildSchema(b, "cck8", "D3")
	if err != nil {
		t.Fatal(err)
	}
	if schema.ID == "" || len(cells) != 96 {
		t.Fatalf("bad schema: %#v cells=%d", schema, len(cells))
	}
	if cells[0].SampleID != "Control-1" || cells[0].Group != "Control" || cells[0].Replicate != 1 {
		t.Fatalf("A1 mapping = %#v", cells[0])
	}
	if cells[1].Replicate != 2 {
		t.Fatalf("A2 replicate = %#v", cells[1])
	}
}
