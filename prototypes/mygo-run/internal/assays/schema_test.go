package assays

import (
	"encoding/json"
	"os"
	"testing"

	"asterlab-mygo-prototype/internal/blocks"
)

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// These snapshots are from real Design records 234c5228-15b3-4407-b4c9-2fd75b17ee8d
// and dfe3dbd7-cb48-4fa4-976e-b842d837eba8 in labworkbench.sqlite.
func TestBuildSchemaCCK8Fixture(t *testing.T) {
	schema, cells, err := BuildSchema(fixture(t, "cck8-design.json"), "cck8", "24 h")
	if err != nil {
		t.Fatal(err)
	}
	if schema.ID != "cck8.plate96.od450.v1" || schema.Layout != "plate96" || len(schema.RowLabels) != 8 || len(schema.ColumnLabels) != 12 || len(cells) != 96 {
		t.Fatalf("unexpected CCK-8 schema: %+v cells=%d", schema, len(cells))
	}
	if schema.Metadata["controlGroup"] != "Control" || schema.Metadata["groups"] != "Control,Treatment" {
		t.Fatalf("missing design metadata: %#v", schema.Metadata)
	}
	for _, cell := range cells {
		if cell.Status != Missing || cell.SampleID != "" || cell.Group != "" {
			t.Fatalf("invented CCK-8 layout: %+v", cell)
		}
	}
}

func TestBuildSchemaQPCRFixture(t *testing.T) {
	schema, cells, err := BuildSchema(fixture(t, "qpcr-design.json"), "qpcr", "24 h")
	if err != nil {
		t.Fatal(err)
	}
	if schema.ID != "qpcr.sample_gene.cq.v1" || schema.Layout != "sample_gene" || schema.Metadata["groups"] != "Control,Treatment" || schema.Metadata["unresolved"] == "" {
		t.Fatalf("unexpected qPCR schema: %+v", schema)
	}
	if len(schema.RowLabels) != 0 || len(schema.ColumnLabels) != 0 || len(cells) != 0 {
		t.Fatalf("sample/gene allocation must remain unresolved, got rows=%v cols=%v cells=%d", schema.RowLabels, schema.ColumnLabels, len(cells))
	}
}

func TestBuildSchemaCCK8MapsOnlySelectedWells(t *testing.T) {
	var doc map[string]any
	if err := json.Unmarshal(fixture(t, "cck8-design.json"), &doc); err != nil {
		t.Fatal(err)
	}
	plate, _ := json.Marshal(blocks.Plate96Props{Rows: 8, Cols: 12, Selected: []int{0, 1, 13}, Groups: map[int]string{0: "Control", 1: "Control", 13: "Treatment"}, Labels: map[int]string{0: "S1", 1: "S1", 13: "S2"}})
	doc["mygo"] = map[string]any{"assays": map[string]any{"cck8": map[string]any{"plateInstanceId": "plate-1", "timepoints": []string{"24 h"}}}, "instances": map[string]any{"step-1": []any{map[string]any{"id": "plate-1", "type": string(blocks.TypePlate96), "props": json.RawMessage(plate)}}}}
	data, _ := json.Marshal(doc)
	_, cells, err := BuildSchema(data, "cck8", "24 h")
	if err != nil {
		t.Fatal(err)
	}
	if cells[0].SampleID != "S1" || cells[0].Group != "Control" || cells[0].Replicate != 1 || cells[1].Replicate != 2 || cells[13].SampleID != "S2" || cells[2].SampleID != "" {
		t.Fatalf("unexpected well mapping: %+v %+v %+v", cells[0], cells[1], cells[13])
	}
}

func TestBuildSchemaQPCRMapping(t *testing.T) {
	var doc map[string]any
	if err := json.Unmarshal(fixture(t, "qpcr-design.json"), &doc); err != nil {
		t.Fatal(err)
	}
	doc["mygo"] = map[string]any{"assays": map[string]any{"qpcr": map[string]any{
		"timepoints": []string{"0 h", "24 h"}, "technicalReplicates": 2, "referenceGenes": []string{"GAPDH"}, "calibratorGroup": "Control",
		"samples": []any{map[string]any{"id": "S1", "group": "Control"}}, "genes": []any{map[string]any{"id": "GAPDH", "reference": true}, map[string]any{"id": "IL6"}},
	}}}
	data, _ := json.Marshal(doc)
	schema, cells, err := BuildSchema(data, "qpcr", "24 h")
	if err != nil {
		t.Fatal(err)
	}
	if len(schema.RowLabels) != 1 || len(schema.ColumnLabels) != 4 || len(cells) != 4 || schema.Metadata["referenceGenes"] != "GAPDH" || schema.Metadata["calibratorGroup"] != "Control" {
		t.Fatalf("unexpected qPCR mapping: schema=%+v cells=%+v", schema, cells)
	}
	if cells[3].Replicate != 2 || cells[3].Column != 3 || cells[3].Field != Cq || cells[3].Status != Missing {
		t.Fatalf("wrong replicate mapping: %+v", cells[3])
	}
}
