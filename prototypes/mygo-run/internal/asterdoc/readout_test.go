package asterdoc

import (
	"strings"
	"testing"
)

func TestParseReadoutSchemaV1(t *testing.T) {
	valid := "kind: readout\nid: ro-1\nschema: 1\nschema_id: cck8_od450\nendpoint: od450\nvalue_kind: numeric\n"
	forKind := func(kind, extra string) string {
		return strings.Replace(valid, "value_kind: numeric", "value_kind: "+kind, 1) + extra
	}
	tests := []struct {
		name  string
		body  string
		valid bool
	}{
		{name: "minimal numeric", body: valid, valid: true},
		{name: "numeric with unit and source policy", body: valid + "unit: AU\nnull_allowed: true\nsource_kinds: [raw, derived]\n", valid: true},
		{name: "table columns", body: forKind("table", "table_columns:\n  - key: od450\n    label: OD450\n    value_kind: numeric\n    unit: AU\n"), valid: true},
		{name: "plate format", body: forKind("plate", "plate_format: \"96\"\n"), valid: true},
		{name: "categorical without unit", body: forKind("categorical", ""), valid: true},
		{name: "file without unit", body: forKind("file", ""), valid: true},
		{name: "missing schema id", body: strings.Replace(valid, "schema_id: cck8_od450\n", "", 1)},
		{name: "blank schema id", body: strings.Replace(valid, "schema_id: cck8_od450", "schema_id: '  '", 1)},
		{name: "schema id wrong type", body: strings.Replace(valid, "schema_id: cck8_od450", "schema_id: 12", 1)},
		{name: "missing endpoint", body: strings.Replace(valid, "endpoint: od450\n", "", 1)},
		{name: "endpoint wrong type", body: strings.Replace(valid, "endpoint: od450", "endpoint: false", 1)},
		{name: "missing value kind", body: strings.Replace(valid, "value_kind: numeric\n", "", 1)},
		{name: "value kind enum", body: strings.Replace(valid, "value_kind: numeric", "value_kind: image", 1)},
		{name: "unit wrong type", body: valid + "unit: 7\n"},
		{name: "blank unit means no canonical unit", body: valid + "unit: '  '\n", valid: true},
		{name: "plate format wrong type", body: forKind("plate", "plate_format: 96\n")},
		{name: "unit not applicable", body: forKind("categorical", "unit: AU\n")},
		{name: "null allowed wrong type", body: valid + "null_allowed: yes\n"},
		{name: "source kinds wrong type", body: valid + "source_kinds: raw\n"},
		{name: "source kind enum", body: valid + "source_kinds: [raw, estimated]\n"},
		{name: "derived requires raw", body: valid + "source_kinds: [derived]\n"},
		{name: "table requires columns", body: forKind("table", "")},
		{name: "table columns wrong type", body: forKind("table", "table_columns: od450\n")},
		{name: "table column missing key", body: forKind("table", "table_columns:\n  - label: OD450\n    value_kind: numeric\n")},
		{name: "table column kind enum", body: forKind("table", "table_columns:\n  - key: od450\n    label: OD450\n    value_kind: plate\n")},
		{name: "plate requires format", body: forKind("plate", "")},
		{name: "plate format enum", body: forKind("plate", "plate_format: \"1536\"\n")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			source := "```asterlab\n" + tt.body + "```\n"
			_, err := Parse(source)
			if (err == nil) != tt.valid {
				t.Fatalf("valid=%v, Parse() error = %v", tt.valid, err)
			}
		})
	}
}
