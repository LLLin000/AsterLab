package asterdoc

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"math"
	"strings"
)

// Foundation v1 validates the named unit subset in spec/units.json, including
// the additional OperationSpec v2 laboratory units. Conversions are explicit
// and never rewrite the authored value or unit.
//
//go:embed spec/units.json
var unitSpecJSON []byte

type unitSpecFile struct {
	SchemaVersion int        `json:"schemaVersion"`
	Units         []unitSpec `json:"units"`
}

type unitSpec struct {
	Symbol    string  `json:"symbol"`
	Dimension string  `json:"dimension"`
	Factor    float64 `json:"factor"`
}

var (
	unitBySymbol map[string]unitSpec
	unitSpecErr  error
)

func init() { unitSpecErr = loadUnits() }

func loadUnits() error {
	var file unitSpecFile
	if err := json.Unmarshal(unitSpecJSON, &file); err != nil {
		return fmt.Errorf("asterdoc unit registry: %w", err)
	}
	if file.SchemaVersion != 1 {
		return fmt.Errorf("asterdoc unit registry: unsupported schema %d", file.SchemaVersion)
	}
	unitBySymbol = make(map[string]unitSpec, len(file.Units))
	for _, unit := range file.Units {
		if unit.Symbol == "" || unit.Dimension == "" || unit.Factor <= 0 {
			return fmt.Errorf("asterdoc unit registry: invalid unit %q", unit.Symbol)
		}
		if _, exists := unitBySymbol[unit.Symbol]; exists {
			return fmt.Errorf("asterdoc unit registry repeats %q", unit.Symbol)
		}
		unitBySymbol[unit.Symbol] = unit
	}
	return nil
}

// quantityParts reads the Foundation v1 quantity shape {value, unit}. The
// value is a JSON/YAML number; the unit is a UCUM code in the shared subset.
func quantityParts(value any) (float64, string, error) {
	mapping, ok := value.(map[string]any)
	if !ok {
		return 0, "", fmt.Errorf("quantity must be a mapping with value and unit")
	}
	raw, ok := mapping["value"]
	if !ok {
		return 0, "", fmt.Errorf("quantity needs value")
	}
	number, ok := numericValue(raw)
	if !ok {
		return 0, "", fmt.Errorf("quantity value must be a number")
	}
	if math.IsNaN(number) || math.IsInf(number, 0) {
		return 0, "", fmt.Errorf("quantity value must be finite")
	}
	unit, _ := mapping["unit"].(string)
	unit = strings.TrimSpace(unit)
	if unit == "" {
		return 0, "", fmt.Errorf("quantity needs a UCUM unit")
	}
	return number, unit, nil
}

func numericValue(value any) (float64, bool) {
	switch n := value.(type) {
	case int:
		return float64(n), true
	case int8:
		return float64(n), true
	case int16:
		return float64(n), true
	case int32:
		return float64(n), true
	case int64:
		return float64(n), true
	case uint:
		return float64(n), true
	case uint8:
		return float64(n), true
	case uint16:
		return float64(n), true
	case uint32:
		return float64(n), true
	case uint64:
		return float64(n), true
	case float32:
		return float64(n), true
	case float64:
		return n, true
	}
	return 0, false
}

func unitFor(symbol string) (unitSpec, bool) {
	if unitSpecErr != nil {
		return unitSpec{}, false
	}
	unit, ok := unitBySymbol[symbol]
	return unit, ok
}

// validateQuantity checks the shape and, when dimension is set, that the unit
// belongs to that dimension. No implicit conversion happens here.
func validateQuantity(value any, dimension string) error {
	_, unit, err := quantityParts(value)
	if err != nil {
		return err
	}
	def, ok := unitFor(unit)
	if !ok {
		return fmt.Errorf("unit %q is not registered", unit)
	}
	if dimension != "" && def.Dimension != dimension {
		return fmt.Errorf("unit %q is %s, not %s", unit, def.Dimension, dimension)
	}
	return nil
}

// quantityIn converts a quantity to targetUnit only when both units are in the
// same registered dimension. Authored input is never mutated.
func quantityIn(value any, targetUnit string) (float64, error) {
	number, unit, err := quantityParts(value)
	if err != nil {
		return 0, err
	}
	from, ok := unitFor(unit)
	if !ok {
		return 0, fmt.Errorf("unit %q is not registered", unit)
	}
	to, ok := unitFor(targetUnit)
	if !ok {
		return 0, fmt.Errorf("unit %q is not registered", targetUnit)
	}
	if from.Dimension != to.Dimension {
		return 0, fmt.Errorf("cannot convert %s to %s", from.Dimension, to.Dimension)
	}
	return number * from.Factor / to.Factor, nil
}
