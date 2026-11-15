// Package blocks is the typed component library for MyGo Desktop v1.
// It has no dependency on Core or DB; it owns presentation-level block
// definitions, instances, validation and serialization only.
package blocks

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
)

// BlockType is a stable identifier for a block kind.
type BlockType string

const (
	TypeNote               BlockType = "note"
	TypeParameterTable     BlockType = "parameter_table"
	TypeMeasurementTable   BlockType = "measurement_table"
	TypePlate96            BlockType = "plate_96"
	TypePlate12            BlockType = "plate_12"
	TypeDilutionCalculator BlockType = "dilution_calculator"
	TypeChecklist          BlockType = "checklist"
	TypeNestedWorkflow     BlockType = "nested_workflow"
)

// AllTypes lists every supported block type.
func AllTypes() []BlockType {
	return []BlockType{
		TypeNote, TypeParameterTable, TypeMeasurementTable, TypePlate96, TypePlate12,
		TypeDilutionCalculator, TypeChecklist, TypeNestedWorkflow,
	}
}

// --- typed props ---

type NoteProps struct {
	Title string `json:"title"`
	Text  string `json:"text"`
}

type ParamRow struct {
	Key   string `json:"key"`
	Value string `json:"value"`
	Unit  string `json:"unit"`
}

type ParameterTableProps struct {
	Rows []ParamRow `json:"rows"`
}

type MeasurementRow struct {
	SampleID string  `json:"sampleId"`
	Value    float64 `json:"value"`
	Unit     string  `json:"unit"`
}

type MeasurementTableProps struct {
	Columns []string         `json:"columns"`
	Rows    []MeasurementRow `json:"rows"`
}

type Plate96Props struct {
	Rows     int            `json:"rows"` // 8
	Cols     int            `json:"cols"` // 12
	Selected []int          `json:"selected"`
	Groups   map[int]string `json:"groups"`
	Colors   map[int]string `json:"colors"`
	Labels   map[int]string `json:"labels"`
}

type Plate12Props Plate96Props

type DilutionInput struct {
	StockConc float64 `json:"stockConc"`
	FinalConc float64 `json:"finalConc"`
	FinalVol  float64 `json:"finalVol"`
}

type DilutionResult struct {
	StockVol   float64 `json:"stockVol"`
	DiluentVol float64 `json:"diluentVol"`
}

type DilutionProps struct {
	Input  DilutionInput  `json:"input"`
	Result DilutionResult `json:"result"`
	Valid  bool           `json:"valid"`
	Error  string         `json:"error"`
}

type ChecklistItem struct {
	ID      string `json:"id"`
	Text    string `json:"text"`
	Checked bool   `json:"checked"`
}

type ChecklistProps struct {
	Items []ChecklistItem `json:"items"`
}

type NestedWorkflowProps struct {
	WorkflowID string `json:"workflowId"`
	StepRef    string `json:"stepRef"`
}

// --- Definition (shared, versioned) ---

type Definition struct {
	ID      string    `json:"id"`
	Version uint64    `json:"version"`
	Type    BlockType `json:"type"`
	Props   any       `json:"props"`
}

func (d Definition) Validate() error {
	if d.ID == "" {
		return errors.New("definition id required")
	}
	if d.Version == 0 {
		return errors.New("definition version must be > 0")
	}
	if !isKnownType(d.Type) {
		return fmt.Errorf("unknown block type %s", d.Type)
	}
	return validateProps(d.Type, d.Props)
}

// --- Instance (step-level frozen copy) ---

type Instance struct {
	ID                string          `json:"id"`
	DefinitionID      string          `json:"definitionId"`
	DefinitionVersion uint64          `json:"definitionVersion"`
	Type              BlockType       `json:"type"`
	RawProps          json.RawMessage `json:"props"`
}

func NewInstance(id string, def Definition) (Instance, error) {
	if err := def.Validate(); err != nil {
		return Instance{}, err
	}
	b, err := json.Marshal(def.Props)
	if err != nil {
		return Instance{}, err
	}
	return Instance{
		ID:                id,
		DefinitionID:      def.ID,
		DefinitionVersion: def.Version,
		Type:              def.Type,
		RawProps:          b,
	}, nil
}

func (i Instance) DecodeProps(v any) error {
	return json.Unmarshal(i.RawProps, v)
}

func (i Instance) Validate() error {
	if i.ID == "" {
		return errors.New("instance id required")
	}
	if i.DefinitionID == "" {
		return errors.New("definition id required")
	}
	if i.DefinitionVersion == 0 {
		return errors.New("definition version required")
	}
	if !isKnownType(i.Type) {
		return fmt.Errorf("unknown block type %s", i.Type)
	}
	var raw any
	if err := json.Unmarshal(i.RawProps, &raw); err != nil {
		return err
	}
	return validateRaw(i.Type, i.RawProps)
}

// MarshalJSON/UnmarshalJSON already via struct tags.

func (i Instance) PropsJSON() json.RawMessage { return i.RawProps }

// --- Registry ---

type Registry struct {
	// history keyed by definition id -> versions (ascending)
	history map[string][]Definition
}

func NewRegistry() *Registry { return &Registry{history: map[string][]Definition{}} }

func (r *Registry) Register(def Definition) error {
	if err := def.Validate(); err != nil {
		return err
	}
	versions := r.history[def.ID]
	for _, v := range versions {
		if v.Version == def.Version {
			return fmt.Errorf("definition %s version %d already registered", def.ID, def.Version)
		}
	}
	r.history[def.ID] = append(versions, def)
	// keep sorted
	sort.Slice(r.history[def.ID], func(a, b int) bool {
		return r.history[def.ID][a].Version < r.history[def.ID][b].Version
	})
	return nil
}

func (r *Registry) Get(id string) (Definition, bool) {
	versions := r.history[id]
	if len(versions) == 0 {
		return Definition{}, false
	}
	return versions[len(versions)-1], true
}

func (r *Registry) GetVersion(id string, version uint64) (Definition, bool) {
	for _, v := range r.history[id] {
		if v.Version == version {
			return v, true
		}
	}
	return Definition{}, false
}

func (r *Registry) Versions(id string) []uint64 {
	out := make([]uint64, 0, len(r.history[id]))
	for _, v := range r.history[id] {
		out = append(out, v.Version)
	}
	return out
}

func (r *Registry) IDs() []string {
	out := make([]string, 0, len(r.history))
	for k := range r.history {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// FreezeInstance resolves a definition from registry and creates a frozen
// instance. Editing the shared definition later does not mutate this instance.
func (r *Registry) FreezeInstance(id, defID string, version uint64) (Instance, error) {
	def, ok := r.GetVersion(defID, version)
	if !ok {
		// fall back to latest
		d, ok2 := r.Get(defID)
		if !ok2 {
			return Instance{}, fmt.Errorf("definition %s not found", defID)
		}
		def = d
	}
	return NewInstance(id, def)
}

// --- validation helpers ---

func isKnownType(t BlockType) bool {
	for _, x := range AllTypes() {
		if x == t {
			return true
		}
	}
	return false
}

func validateProps(t BlockType, props any) error {
	b, err := json.Marshal(props)
	if err != nil {
		return err
	}
	return validateRaw(t, b)
}

func validateRaw(t BlockType, raw json.RawMessage) error {
	switch t {
	case TypeNote:
		var p NoteProps
		if err := json.Unmarshal(raw, &p); err != nil {
			return err
		}
	case TypeParameterTable:
		var p ParameterTableProps
		if err := json.Unmarshal(raw, &p); err != nil {
			return err
		}
		for _, r := range p.Rows {
			if r.Key == "" {
				return errors.New("parameter key required")
			}
		}
	case TypeMeasurementTable:
		var p MeasurementTableProps
		if err := json.Unmarshal(raw, &p); err != nil {
			return err
		}
	case TypePlate96:
		var p Plate96Props
		if err := json.Unmarshal(raw, &p); err != nil {
			return err
		}
		if p.Rows == 0 {
			p.Rows = 8
		}
		if p.Cols == 0 {
			p.Cols = 12
		}
		if p.Rows*p.Cols != 96 {
			return fmt.Errorf("plate must be 96 wells, got %d", p.Rows*p.Cols)
		}
		for _, w := range p.Selected {
			if w < 0 || w >= 96 {
				return fmt.Errorf("well index out of range: %d", w)
			}
		}
		for _, wells := range []map[int]string{p.Groups, p.Colors, p.Labels} {
			for w := range wells {
				if w < 0 || w >= 96 {
					return fmt.Errorf("well index out of range: %d", w)
				}
			}
		}
	case TypePlate12:
		var p Plate12Props
		if err := json.Unmarshal(raw, &p); err != nil {
			return err
		}
		if (p.Rows != 0 && p.Rows != 3) || (p.Cols != 0 && p.Cols != 4) {
			return fmt.Errorf("plate must be 3x4, got %dx%d", p.Rows, p.Cols)
		}
		for _, w := range p.Selected {
			if w < 0 || w >= 12 {
				return fmt.Errorf("well index out of range: %d", w)
			}
		}
		for _, wells := range []map[int]string{p.Groups, p.Colors, p.Labels} {
			for w := range wells {
				if w < 0 || w >= 12 {
					return fmt.Errorf("well index out of range: %d", w)
				}
			}
		}
	case TypeDilutionCalculator:
		var p DilutionProps
		if err := json.Unmarshal(raw, &p); err != nil {
			return err
		}
		// validate input if present
		if p.Input.StockConc != 0 || p.Input.FinalConc != 0 || p.Input.FinalVol != 0 {
			if err := validateDilutionInput(p.Input); err != nil {
				return err
			}
		}
	case TypeChecklist:
		var p ChecklistProps
		if err := json.Unmarshal(raw, &p); err != nil {
			return err
		}
	case TypeNestedWorkflow:
		var p NestedWorkflowProps
		if err := json.Unmarshal(raw, &p); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unknown type %s", t)
	}
	return nil
}

func validateDilutionInput(in DilutionInput) error {
	if in.StockConc <= 0 {
		return errors.New("stock concentration must be > 0")
	}
	if in.FinalConc <= 0 {
		return errors.New("final concentration must be > 0")
	}
	if in.FinalVol <= 0 {
		return errors.New("final volume must be > 0")
	}
	if in.FinalConc > in.StockConc {
		return errors.New("final concentration cannot exceed stock")
	}
	return nil
}

// --- Plate helpers ---

type Plate struct {
	Props Plate96Props
}

func NewPlate() Plate {
	return Plate{Props: Plate96Props{Rows: 8, Cols: 12, Selected: []int{}, Groups: map[int]string{}, Colors: map[int]string{}, Labels: map[int]string{}}}
}

func (p *Plate) SelectWell(i int) error {
	if i < 0 || i >= 96 {
		return fmt.Errorf("well index out of range: %d", i)
	}
	for _, w := range p.Props.Selected {
		if w == i {
			return nil
		}
	}
	p.Props.Selected = append(p.Props.Selected, i)
	return nil
}

func (p *Plate) DeselectWell(i int) {
	out := p.Props.Selected[:0]
	for _, w := range p.Props.Selected {
		if w != i {
			out = append(out, w)
		}
	}
	p.Props.Selected = out
}

func (p *Plate) ToggleWell(i int) error {
	if i < 0 || i >= 96 {
		return fmt.Errorf("well index out of range: %d", i)
	}
	for j, w := range p.Props.Selected {
		if w == i {
			p.Props.Selected = append(p.Props.Selected[:j], p.Props.Selected[j+1:]...)
			return nil
		}
	}
	p.Props.Selected = append(p.Props.Selected, i)
	return nil
}

func (p *Plate) IsSelected(i int) bool {
	for _, w := range p.Props.Selected {
		if w == i {
			return true
		}
	}
	return false
}

func (p *Plate) SetGroup(well int, group string) error {
	if well < 0 || well >= 96 {
		return fmt.Errorf("well index out of range: %d", well)
	}
	if p.Props.Groups == nil {
		p.Props.Groups = map[int]string{}
	}
	if group == "" {
		delete(p.Props.Groups, well)
	} else {
		p.Props.Groups[well] = group
	}
	return nil
}

func (p *Plate) SetColor(well int, color string) error {
	if well < 0 || well >= 96 {
		return fmt.Errorf("well index out of range: %d", well)
	}
	if p.Props.Colors == nil {
		p.Props.Colors = map[int]string{}
	}
	if color == "" {
		delete(p.Props.Colors, well)
	} else {
		p.Props.Colors[well] = color
	}
	return nil
}

func (p *Plate) SetLabel(well int, label string) error {
	if well < 0 || well >= 96 {
		return fmt.Errorf("well index out of range: %d", well)
	}
	if p.Props.Labels == nil {
		p.Props.Labels = map[int]string{}
	}
	if label == "" {
		delete(p.Props.Labels, well)
	} else {
		p.Props.Labels[well] = label
	}
	return nil
}

func (p Plate) SelectedCount() int { return len(p.Props.Selected) }

// --- Calculator pure logic ---

func CalculateDilution(in DilutionInput) (DilutionResult, error) {
	if err := validateDilutionInput(in); err != nil {
		return DilutionResult{}, err
	}
	stockVol := in.FinalConc * in.FinalVol / in.StockConc
	diluentVol := in.FinalVol - stockVol
	if diluentVol < 0 {
		diluentVol = 0
	}
	return DilutionResult{StockVol: stockVol, DiluentVol: diluentVol}, nil
}

func DilutionPropsFromInput(in DilutionInput) DilutionProps {
	res, err := CalculateDilution(in)
	p := DilutionProps{Input: in}
	if err != nil {
		p.Valid = false
		p.Error = err.Error()
		return p
	}
	p.Result = res
	p.Valid = true
	return p
}

// --- Checklist helpers ---

type Checklist struct {
	Props ChecklistProps
}

func (c *Checklist) Toggle(id string) {
	for i := range c.Props.Items {
		if c.Props.Items[i].ID == id {
			c.Props.Items[i].Checked = !c.Props.Items[i].Checked
		}
	}
}

func (c Checklist) CheckedCount() int {
	n := 0
	for _, it := range c.Props.Items {
		if it.Checked {
			n++
		}
	}
	return n
}

// --- Serialization roundtrip helpers ---

// BlockEnvelope is a type-tagged serialized instance.
type BlockEnvelope struct {
	Type BlockType       `json:"type"`
	Data json.RawMessage `json:"data"`
}

func MarshalInstance(inst Instance) ([]byte, error) {
	return json.Marshal(inst)
}

func UnmarshalInstance(b []byte) (Instance, error) {
	var inst Instance
	if err := json.Unmarshal(b, &inst); err != nil {
		return Instance{}, err
	}
	if err := inst.Validate(); err != nil {
		return Instance{}, err
	}
	return inst, nil
}
