package blocks

import (
	"image/png"
	"os"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestPlateSelection(t *testing.T) {
	p := NewPlate()
	if err := p.ToggleWell(0); err != nil {
		t.Fatal(err)
	}
	if err := p.ToggleWell(5); err != nil {
		t.Fatal(err)
	}
	if !p.IsSelected(0) || !p.IsSelected(5) {
		t.Fatal("selection missing")
	}
	if p.SelectedCount() != 2 {
		t.Fatalf("count=%d", p.SelectedCount())
	}
	if err := p.SetGroup(0, "A"); err != nil {
		t.Fatal(err)
	}
	if err := p.SetColor(0, "#ff0000"); err != nil {
		t.Fatal(err)
	}
	if err := p.SetLabel(0, "ctrl"); err != nil {
		t.Fatal(err)
	}
	if p.Props.Groups[0] != "A" || p.Props.Colors[0] != "#ff0000" || p.Props.Labels[0] != "ctrl" {
		t.Fatal("metadata not set")
	}
	p.DeselectWell(5)
	if p.IsSelected(5) {
		t.Fatal("not deselected")
	}
	if err := p.ToggleWell(96); err == nil {
		t.Fatal("expected error")
	}
}

func TestTableEdit(t *testing.T) {
	props := ParameterTableProps{Rows: []ParamRow{{Key: "temp", Value: "37", Unit: "C"}}}
	props.Rows[0].Value = "42"
	if props.Rows[0].Value != "42" {
		t.Fatal("edit failed")
	}
	mp := MeasurementTableProps{Columns: []string{"sample", "value"}, Rows: []MeasurementRow{{SampleID: "s1", Value: 1.2, Unit: "AU"}}}
	if len(mp.Rows) != 1 {
		t.Fatal("rows")
	}
	mp.Rows[0].Value = 2.4
	if mp.Rows[0].Value != 2.4 {
		t.Fatal("measurement edit failed")
	}
}

func TestCalculatorValidation(t *testing.T) {
	res, err := CalculateDilution(DilutionInput{StockConc: 10, FinalConc: 2, FinalVol: 100})
	if err != nil {
		t.Fatal(err)
	}
	if res.StockVol != 20 || res.DiluentVol != 80 {
		t.Fatalf("got %+v", res)
	}
	if _, err := CalculateDilution(DilutionInput{StockConc: 2, FinalConc: 10, FinalVol: 100}); err == nil {
		t.Fatal("expected error")
	}
	if _, err := CalculateDilution(DilutionInput{StockConc: 0, FinalConc: 1, FinalVol: 1}); err == nil {
		t.Fatal("expected error")
	}
	p := DilutionPropsFromInput(DilutionInput{StockConc: 10, FinalConc: 2, FinalVol: 100})
	if !p.Valid {
		t.Fatal("should be valid")
	}
	p2 := DilutionPropsFromInput(DilutionInput{StockConc: 1, FinalConc: 2, FinalVol: 100})
	if p2.Valid {
		t.Fatal("should be invalid")
	}
}

func TestSerializationRoundtrip(t *testing.T) {
	reg := NewRegistry()
	def := Definition{ID: "note-1", Version: 1, Type: TypeNote, Props: NoteProps{Title: "t", Text: "hello"}}
	if err := reg.Register(def); err != nil {
		t.Fatal(err)
	}
	inst, err := reg.FreezeInstance("inst-1", "note-1", 1)
	if err != nil {
		t.Fatal(err)
	}
	def2 := Definition{ID: "note-1", Version: 2, Type: TypeNote, Props: NoteProps{Title: "changed", Text: "new"}}
	if err := reg.Register(def2); err != nil {
		t.Fatal(err)
	}
	var p NoteProps
	if err := inst.DecodeProps(&p); err != nil {
		t.Fatal(err)
	}
	if p.Text != "hello" {
		t.Fatalf("frozen instance mutated: %v", p)
	}
	b, err := MarshalInstance(inst)
	if err != nil {
		t.Fatal(err)
	}
	inst2, err := UnmarshalInstance(b)
	if err != nil {
		t.Fatal(err)
	}
	if inst2.ID != inst.ID || inst2.DefinitionVersion != inst.DefinitionVersion {
		t.Fatal("roundtrip mismatch")
	}
	plate := NewPlate()
	_ = plate.ToggleWell(3)
	plate.Props.Selected = []int{3}
	defP := Definition{ID: "plate-1", Version: 1, Type: TypePlate96, Props: plate.Props}
	if err := reg.Register(defP); err != nil {
		t.Fatal(err)
	}
	instP, _ := reg.FreezeInstance("inst-p", "plate-1", 1)
	bp, _ := MarshalInstance(instP)
	if _, err := UnmarshalInstance(bp); err != nil {
		t.Fatal(err)
	}
}

func TestBlocksPNG(t *testing.T) {
	reg := NewRegistry()
	defs := []Definition{
		{ID: "n", Version: 1, Type: TypeNote, Props: NoteProps{Title: "Note", Text: "hello"}},
		{ID: "pt", Version: 1, Type: TypeParameterTable, Props: ParameterTableProps{Rows: []ParamRow{{Key: "k", Value: "v", Unit: "u"}}}},
		{ID: "mt", Version: 1, Type: TypeMeasurementTable, Props: MeasurementTableProps{Rows: []MeasurementRow{{SampleID: "s1", Value: 1.2, Unit: "AU"}}}},
	}
	pl := NewPlate()
	_ = pl.ToggleWell(0)
	_ = pl.ToggleWell(1)
	defs = append(defs, Definition{ID: "pl", Version: 1, Type: TypePlate96, Props: pl.Props})
	defs = append(defs, Definition{ID: "dc", Version: 1, Type: TypeDilutionCalculator, Props: DilutionPropsFromInput(DilutionInput{StockConc: 10, FinalConc: 2, FinalVol: 100})})
	defs = append(defs, Definition{ID: "cl", Version: 1, Type: TypeChecklist, Props: ChecklistProps{Items: []ChecklistItem{{ID: "1", Text: "step", Checked: true}}}})
	defs = append(defs, Definition{ID: "nw", Version: 1, Type: TypeNestedWorkflow, Props: NestedWorkflowProps{WorkflowID: "wf", StepRef: "s1"}})
	for _, d := range defs {
		if err := reg.Register(d); err != nil {
			t.Fatal(err)
		}
	}
	insts := make([]Instance, 0, len(defs))
	for _, d := range defs {
		inst, err := reg.FreezeInstance("i-"+string(d.ID), d.ID, d.Version)
		if err != nil {
			t.Fatal(err)
		}
		insts = append(insts, inst)
	}
	app := &galleryApp{insts: insts}
	tt := ui.NewTester(app.view, 1200, 760)
	tt.Frame()
	img := tt.Image()
	f, err := os.Create("../../../../docs/design/mygo-blocks.png")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
}

type galleryApp struct {
	insts []Instance
}

func (g *galleryApp) view(c *ui.Context) {
	Gallery(c, g.insts)
}
