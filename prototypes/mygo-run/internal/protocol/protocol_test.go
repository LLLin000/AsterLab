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
		Nodes:         []workflow.Node{{ID: "step-a"}, {ID: "step-b"}, {ID: "step-c"}},
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

func TestRenderVersionsListsSummariesWithoutRawIDs(t *testing.T) {
	versions := []VersionSummary{
		{ID: "id-1", Title: "CCK-8", Version: 1},
		{ID: "id-2", Title: "CCK-8", Version: 2},
	}
	tt := ui.NewTester(func(c *ui.Context) {
		RenderVersions(c, versions, "id-1", nil)
	}, 900, 600)
	tt.Frame()
	for _, want := range []string{"CCK-8 · v1", "CCK-8 · v2"} {
		if !tt.HasText(want) {
			t.Fatalf("version list missing %q", want)
		}
	}
	if tt.HasText("id-1") || tt.HasText("id-2") {
		t.Fatal("version list exposed raw protocol identifiers")
	}
}

func TestRenderProtocolStateVocabulary(t *testing.T) {
	tests := []struct {
		state string
		runID string
		rev   uint64
		want  string
		avoid string
	}{
		{state: "unresolved", want: "未解析"},
		{state: "preview", rev: 7, want: "预览 · Design revision 7", avoid: "冻结"},
		{state: "frozen", runID: "run-abc", rev: 4, want: "已冻结 · Run run-abc · revision 4"},
	}
	for _, test := range tests {
		tt := ui.NewTester(func(c *ui.Context) {
			RenderVersionState(c, test.state, test.runID, test.rev)
		}, 900, 600)
		tt.Frame()
		if !tt.HasText(test.want) {
			t.Errorf("%s state missing %q", test.state, test.want)
		}
		if test.avoid != "" && tt.HasText(test.avoid) {
			t.Errorf("%s state incorrectly contained %q", test.state, test.avoid)
		}
	}
}
