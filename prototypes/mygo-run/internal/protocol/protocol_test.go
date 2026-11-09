package protocol

import (
	"encoding/json"
	"image/png"
	"os"
	"reflect"
	"testing"

	"asterlab-mygo-prototype/internal/blocks"
	"asterlab-mygo-prototype/internal/workflow"

	"github.com/egoist/mygo/ui"
)

func TestStepIDsUsesBusinessOrderAndIncludesRemainingNodes(t *testing.T) {
	s := &State{Document: workflow.WorkflowDocument{
		Nodes: []workflow.Node{{ID: "step-a"}, {ID: "step-b"}, {ID: "step-c"}},
		BusinessOrder: []string{"step-b", "missing", "step-b", "step-a"},
	}}
	if got, want := s.StepIDs(), []string{"step-b", "step-a", "step-c"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("StepIDs() = %v, want %v", got, want)
	}
}

func TestViewScreenshot(t *testing.T) {
	newInst := func(id string, kind blocks.BlockType, props any) blocks.Instance {
		raw, err := json.Marshal(props)
		if err != nil {
			t.Fatal(err)
		}
		return blocks.Instance{ID: id, Type: kind, RawProps: raw}
	}
	s := &State{
		Document: workflow.WorkflowDocument{
			Nodes: []workflow.Node{
				{ID: "mix", Kind: workflow.KindFlask, Title: "Prepare lysate", ComponentRefs: []string{"buffer-a", "tube-15ml"}},
				{ID: "read", Kind: workflow.KindPlate, Title: "Measure absorbance"},
			},
			BusinessOrder: []string{"mix", "read"},
		},
		Instances: map[string][]blocks.Instance{
			"mix": {
				newInst("purpose", blocks.TypeNote, blocks.NoteProps{Title: "Purpose", Text: "Homogenize sample in lysis buffer."}),
				newInst("params", blocks.TypeParameterTable, blocks.ParameterTableProps{Rows: []blocks.ParamRow{{Key: "Incubation", Value: "10", Unit: "min"}}}),
				newInst("check", blocks.TypeChecklist, blocks.ChecklistProps{Items: []blocks.ChecklistItem{{ID: "sealed", Text: "Confirm tube is sealed", Checked: true}}}),
			},
			"read": {
				newInst("output", blocks.TypeMeasurementTable, blocks.MeasurementTableProps{Rows: []blocks.MeasurementRow{{SampleID: "sample-01", Value: 0.82, Unit: "AU"}}}),
			},
		},
		DesignID: "design-42", Revision: 3, SelectedStepID: "mix",
	}
	tt := ui.NewTester(func(c *ui.Context) { View(c, s) }, 1200, 760)
	tt.Frame()
	f, err := os.Create("protocol.png")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, tt.Image()); err != nil {
		t.Fatal(err)
	}
}
