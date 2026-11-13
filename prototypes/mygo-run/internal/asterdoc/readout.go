package asterdoc

import (
	"fmt"
	"strings"
)

func readoutSpec() ModuleSpec {
	return ModuleSpec{
		Descriptor: descriptorFor("readout", 1),
		Validate: func(data map[string]any) error {
			if err := specValidatorFor("readout", 1)(data); err != nil {
				return err
			}
			return validateReadoutFields(data)
		},
	}
}

func validateReadoutFields(data map[string]any) error {
	for _, field := range []string{"schema_id", "endpoint"} {
		value, _ := data[field].(string)
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("field %q must be a non-empty string", field)
		}
	}

	valueKind, _ := data["value_kind"].(string)
	unit, hasUnit := data["unit"]
	if hasUnit {
		text, ok := unit.(string)
		if !ok {
			return fmt.Errorf("field %q must be a string", "unit")
		}
		if strings.TrimSpace(text) != "" && (valueKind == "categorical" || valueKind == "file") {
			return fmt.Errorf("field %q is not applicable to %s readouts", "unit", valueKind)
		}
	}

	if raw, ok := data["null_allowed"]; ok {
		if _, valid := raw.(bool); !valid {
			return fmt.Errorf("field %q must be a boolean", "null_allowed")
		}
	}

	if raw, ok := data["missing_reasons"]; ok {
		if _, valid := raw.([]any); !valid {
			return fmt.Errorf("field %q must be a list", "missing_reasons")
		}
	}

	if raw, ok := data["source_kinds"]; ok {
		if _, valid := raw.([]any); !valid {
			return fmt.Errorf("field %q must be a list", "source_kinds")
		}
	}

	if raw, ok := data["missing_reasons"]; ok {
		items, _ := raw.([]any)
		seen := make(map[string]bool, len(items))
		for _, item := range items {
			reason, _ := item.(string)
			switch reason {
			case "not_detected", "below_lod", "above_uloq", "invalid", "other":
			default:
				return fmt.Errorf("field %q contains unsupported reason %q", "missing_reasons", reason)
			}
			if seen[reason] {
				return fmt.Errorf("field %q repeats reason %q", "missing_reasons", reason)
			}
			seen[reason] = true
		}
	}

	if raw, ok := data["source_kinds"]; ok {
		items, _ := raw.([]any)
		seen := make(map[string]bool, len(items))
		for _, item := range items {
			kind, _ := item.(string)
			if kind != "raw" && kind != "derived" {
				return fmt.Errorf("field %q contains unsupported source kind %q", "source_kinds", kind)
			}
			seen[kind] = true
		}
		if seen["derived"] && !seen["raw"] {
			return fmt.Errorf("field %q allows derived without raw", "source_kinds")
		}
	}

	columns, hasColumns := data["table_columns"]
	if hasColumns {
		if _, valid := columns.([]any); !valid {
			return fmt.Errorf("field %q must be a list of objects", "table_columns")
		}
	}
	if valueKind == "table" && !hasColumns {
		return fmt.Errorf("table readout needs table_columns")
	}
	if valueKind != "table" && hasColumns {
		return fmt.Errorf("field %q is only valid for table readouts", "table_columns")
	}
	if hasColumns {
		items, _ := columns.([]any)
		if len(items) == 0 {
			return fmt.Errorf("field %q must not be empty", "table_columns")
		}
		seen := make(map[string]bool, len(items))
		for i, raw := range items {
			column, _ := raw.(map[string]any)
			prefix := fmt.Sprintf("table_columns[%d]", i)
			for key := range column {
				if key != "key" && key != "label" && key != "value_kind" && key != "unit" {
					return fmt.Errorf("%s has unknown field %q", prefix, key)
				}
			}
			key, keyOK := column["key"].(string)
			_, labelOK := column["label"].(string)
			kind, _ := column["value_kind"].(string)
			if !keyOK || !labelOK {
				return fmt.Errorf("%s needs string key and label", prefix)
			}
			if kind != "numeric" && kind != "categorical" {
				return fmt.Errorf("%s value_kind must be numeric or categorical", prefix)
			}
			if seen[key] {
				return fmt.Errorf("%s repeats column key %q", prefix, key)
			}
			seen[key] = true
			if rawUnit, ok := column["unit"]; ok && rawUnit != nil {
				if _, valid := rawUnit.(string); !valid {
					return fmt.Errorf("%s unit must be a string", prefix)
				}
			}
		}
	}

	plateFormat, hasPlateFormat := data["plate_format"]
	if hasPlateFormat {
		if _, valid := plateFormat.(string); !valid {
			return fmt.Errorf("field %q must be an enum string", "plate_format")
		}
	}
	if valueKind == "plate" && !hasPlateFormat {
		return fmt.Errorf("plate readout needs plate_format")
	}
	if valueKind != "plate" && hasPlateFormat {
		return fmt.Errorf("field %q is only valid for plate readouts", "plate_format")
	}
	return nil
}
