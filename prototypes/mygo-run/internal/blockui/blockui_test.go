package blockui

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"asterlab-mygo-prototype/internal/blocks"
)

// TestDefinitionsProduceValidInstances guards the add menu: every module a
// design step can add must pass the block registry's own validation.
func TestDefinitionsProduceValidInstances(t *testing.T) {
	for _, typ := range Addable() {
		def := NewDefinition(typ)
		if def.Type != typ || TypeTitle(typ) == "" {
			t.Fatalf("definition for %s incomplete: %+v", typ, def)
		}
		if _, err := blocks.NewInstance("m-"+string(typ), def); err != nil {
			t.Fatalf("addable module %s is not a valid instance: %v", typ, err)
		}
	}
}

// TestWellIndexRoundTrip guards the plate module's mapping between the block's
// 0..95 well indexes and the plate widget's A1..H12 ids.
func TestWellIndexRoundTrip(t *testing.T) {
	for idx := 0; idx < 96; idx++ {
		id := wellID(idx, 12)
		back, ok := wellIndex(id, 12)
		if !ok || back != idx {
			t.Fatalf("well %d -> %q -> %d, ok=%v", idx, id, back, ok)
		}
	}
	if id := wellID(0, 12); id != "A1" {
		t.Fatalf("first well = %q, want A1", id)
	}
	if id := wellID(95, 12); id != "H12" {
		t.Fatalf("last well = %q, want H12", id)
	}
	if _, ok := wellIndex("Z1", 12); ok {
		t.Fatal("out-of-range row accepted")
	}
}

// TestWriteStoresProps checks the one persistence seam: a module edit updates
// the instance the design saves, and an identical edit reports no change.
func TestWriteStoresProps(t *testing.T) {
	inst, err := blocks.NewInstance("note-1", NewDefinition(blocks.TypeNote))
	if err != nil {
		t.Fatal(err)
	}
	if changed := write(&inst, blocks.NoteProps{Title: "A"}); !changed {
		t.Fatal("first write reported no change")
	}
	var p blocks.NoteProps
	if err := inst.DecodeProps(&p); err != nil || p.Title != "A" {
		t.Fatalf("stored props = %+v err=%v", p, err)
	}
	if changed := write(&inst, blocks.NoteProps{Title: "A"}); changed {
		t.Fatal("identical write reported a change")
	}
}

// TestModulesRenderInBothModes renders every module type in Design and Run:
// the same instance must describe itself in both without inventing values.
func TestModulesRenderInBothModes(t *testing.T) {
	for _, typ := range []blocks.BlockType{
		blocks.TypeNote, blocks.TypeDilutionCalculator, blocks.TypePlate96,
		blocks.TypeParameterTable, blocks.TypeChecklist, blocks.TypeMeasurementTable,
	} {
		inst, err := blocks.NewInstance("m-"+string(typ), NewDefinition(typ))
		if err != nil {
			t.Fatal(err)
		}
		for _, mode := range []Mode{Design, Run} {
			inst := inst
			tt := ui.NewTester(func(c *ui.Context) { View(c, &inst, mode) }, 380, 600)
			tt.Frame()
			if !tt.HasText(TypeTitle(typ)) {
				t.Fatalf("%s mode=%d missing module title: %q", typ, mode, tt.Texts())
			}
		}
	}
}

// TestReviewSummarisesRecordedRows pins the Review derivation: recorded rows
// produce a labelled count, mean and sample SD, and Design never shows them.
func TestReviewSummarisesRecordedRows(t *testing.T) {
	inst, err := blocks.NewInstance("measure-1", NewDefinition(blocks.TypeMeasurementTable))
	if err != nil {
		t.Fatal(err)
	}
	inst.RawProps = []byte(`{"columns":["OD450"],"rows":[
		{"sampleId":"C1","value":0.8,"unit":"AU"},
		{"sampleId":"C2","value":0.9,"unit":"AU"},
		{"sampleId":"C3","value":1.0,"unit":"AU"}]}`)
	review := ui.NewTester(func(c *ui.Context) { View(c, &inst, Review) }, 380, 600)
	review.Frame()
	if !review.HasText("全部读数 3 条") || !review.HasText("均值 0.900") {
		t.Fatalf("review missing derivation: %q", review.Texts())
	}
	design := ui.NewTester(func(c *ui.Context) { View(c, &inst, Design) }, 380, 600)
	design.Frame()
	if design.HasText("全部读数") {
		t.Fatalf("design must not show recorded results: %q", design.Texts())
	}
}
