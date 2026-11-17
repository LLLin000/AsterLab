package runworkspace

import (
	"encoding/json"
	"testing"
)

// TestBuildCategoricalCommandMatchesCoreInput pins the exact readout.submit
// multi-value JSON Core validates (BE-09): v1 Readout with a categorical
// value, raw source, and no invented link.
func TestBuildCategoricalCommandMatchesCoreInput(t *testing.T) {
	input, err := buildCategoricalCommand("run-1", "run/read", "pv1", "assay-1", "",
		"wb_grade", "", "M3-1", "positive", "key-1")
	if err != nil {
		t.Fatal(err)
	}
	if input.Kind != "readout.submit" {
		t.Fatalf("kind = %q", input.Kind)
	}
	var payload struct {
		RunID             string `json:"run_id"`
		StepPath          string `json:"step_path"`
		ProtocolVersionID string `json:"protocol_version_id"`
		AssayID           string `json:"assay_id"`
		Readouts          []struct {
			SchemaID      string `json:"schema_id"`
			SchemaVersion int    `json:"schema_version"`
			Value         struct {
				Kind     string `json:"kind"`
				Category string `json:"category"`
			} `json:"value"`
			Source struct {
				Kind        string   `json:"kind"`
				DerivedFrom []string `json:"derived_from"`
			} `json:"source"`
			Link *struct {
				SampleID string `json:"sample_id"`
			} `json:"link"`
		} `json:"readouts"`
	}
	if err := json.Unmarshal([]byte(input.PayloadJSON), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.RunID != "run-1" || payload.StepPath != "run/read" ||
		payload.ProtocolVersionID != "pv1" || payload.AssayID != "assay-1" {
		t.Fatalf("scope wrong: %+v", payload)
	}
	if len(payload.Readouts) != 1 {
		t.Fatalf("readouts = %+v", payload.Readouts)
	}
	r := payload.Readouts[0]
	if r.SchemaID != "wb_grade" || r.SchemaVersion != 1 {
		t.Fatalf("schema ref wrong: %+v", r)
	}
	if r.Value.Kind != "categorical" || r.Value.Category != "positive" {
		t.Fatalf("value wrong: %+v", r.Value)
	}
	if r.Source.Kind != "raw" || r.Source.DerivedFrom == nil {
		t.Fatalf("source wrong: %+v", r.Source)
	}
	if r.Link == nil || r.Link.SampleID != "M3-1" {
		t.Fatalf("link wrong: %+v", r.Link)
	}
}

func TestBuildCategoricalCommandIncludesSchemaDef(t *testing.T) {
	def := `{"schema_id":"wb_grade","schema_version":1,"endpoint":"wb_grade","value_kind":"categorical"}`
	input, err := buildCategoricalCommand("run-1", "run/read", "pv1", "", "",
		"wb_grade", def, "", "negative", "")
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(input.PayloadJSON), &payload); err != nil {
		t.Fatal(err)
	}
	schemas, _ := payload["readout_schemas"].([]any)
	if len(schemas) != 1 {
		t.Fatalf("readout_schemas = %v", payload["readout_schemas"])
	}
	if input.IdempotencyKey == "" {
		t.Fatal("idempotency key must be minted per action")
	}
	if _, err := buildCategoricalCommand("run-1", "run/read", "pv1", "", "", "", "", "", "positive", ""); err == nil {
		t.Fatal("missing schema id must fail before Core")
	}
	if _, err := buildCategoricalCommand("run-1", "run/read", "pv1", "", "", "wb_grade", "{bad", "", "positive", ""); err == nil {
		t.Fatal("malformed schema def must fail before Core")
	}
}

func TestParseTableValue(t *testing.T) {
	columns, rows, err := parseTableValue("lane, grade", "1, + | 2, +++")
	if err != nil {
		t.Fatal(err)
	}
	if len(columns) != 2 || columns[0] != "lane" || columns[1] != "grade" {
		t.Fatalf("columns = %v", columns)
	}
	if len(rows) != 2 {
		t.Fatalf("rows = %v", rows)
	}
	// Numeric cells stay numeric, matching Core's own CSV path.
	if rows[0][0] != 1.0 || rows[0][1] != "+" {
		t.Fatalf("row 0 = %v", rows[0])
	}
	if _, _, err := parseTableValue("a, b", "1, 2, 3"); err == nil {
		t.Fatal("ragged row must fail before Core")
	}
	if _, _, err := parseTableValue("", "1, 2"); err == nil {
		t.Fatal("missing columns must fail before Core")
	}
}

// TestBuildTableCommandMatchesCoreInput pins the table value shape Core's own
// CLI test files: kind table with columns plus rows of mixed cells.
func TestBuildTableCommandMatchesCoreInput(t *testing.T) {
	columns, rows, err := parseTableValue("lane, grade", "1, + | 2, +++")
	if err != nil {
		t.Fatal(err)
	}
	input, err := buildTableCommand("run-1", "run/read", "pv1", "", "",
		"wb_table", "", "", columns, rows, "key-2")
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(input.PayloadJSON), &payload); err != nil {
		t.Fatal(err)
	}
	readouts, _ := payload["readouts"].([]any)
	if len(readouts) != 1 {
		t.Fatalf("readouts = %v", payload["readouts"])
	}
	value, _ := readouts[0].(map[string]any)["value"].(map[string]any)
	if value["kind"] != "table" {
		t.Fatalf("value = %v", value)
	}
	gotRows, _ := json.Marshal(value["rows"])
	if string(gotRows) != `[[1,"+"],[2,"+++"]]` {
		t.Fatalf("rows = %s", gotRows)
	}
}
