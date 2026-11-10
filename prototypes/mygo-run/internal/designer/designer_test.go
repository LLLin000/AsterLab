package designer

import (
	"context"
	"encoding/json"
	"image/png"
	"os"
	"strings"
	"testing"

	"asterlab-mygo-prototype/internal/assays"
	"asterlab-mygo-prototype/internal/blocks"
	"asterlab-mygo-prototype/internal/blockui"
	"asterlab-mygo-prototype/internal/state"
	"asterlab-mygo-prototype/internal/theme"
	"asterlab-mygo-prototype/internal/workflow"
	"asterlab-mygo-prototype/modules/runs"

	"github.com/egoist/mygo/ui"
)

func sampleDesigner(t *testing.T) *Designer {
	reg := blocks.NewRegistry()
	def := blocks.Definition{ID: "note1", Type: blocks.TypeNote, Version: 1, Props: map[string]any{"text": "hello"}}
	if err := reg.Register(def); err != nil {
		t.Fatal(err)
	}
	inst, err := reg.FreezeInstance("i1", "note1", 1)
	if err != nil {
		t.Fatal(err)
	}
	d := New("design-1", "Test Design", 3, &state.FakeDesignService{
		Snapshot: runs.DesignSnapshot{DesignID: "design-1", Revision: 4, Title: "Test Design", Data: ""},
	})
	d.Instances = map[string][]blocks.Instance{"n1": {inst}}
	d.Editor.Doc.Nodes = append(d.Editor.Doc.Nodes, workflow.Node{ID: "n1", Kind: workflow.NodeKind("step"), Title: "Step 1"})
	d.SelectedNode = "n1"
	return d
}

func TestSaveCallback(t *testing.T) {
	d := sampleDesigner(t)
	d.Assays["cck8"] = assays.Design{Timepoints: []string{"24 h"}, ControlGroup: "Control"}
	d.StepAssays["n1"] = "cck8"
	fake := &state.FakeDesignService{
		Snapshot: runs.DesignSnapshot{DesignID: "design-1", Revision: 5, Title: "Test Design"},
	}
	d.Service = nil
	var got runs.DesignSaveInput
	d.Callbacks.SaveDesign = func(ctx context.Context, input runs.DesignSaveInput) (runs.DesignSnapshot, error) {
		got = input
		return fake.SaveDesign(ctx, input)
	}
	d.Dirty = true
	d.Save(context.Background())
	var saved designDoc
	if err := json.Unmarshal([]byte(got.Changes), &saved); err != nil {
		t.Fatal(err)
	}
	if saved.MyGo.Assays["cck8"].ControlGroup != "Control" || saved.MyGo.StepAssays["n1"] != "cck8" {
		t.Fatalf("assay mapping not persisted in mygo extension: %+v", saved.MyGo)
	}
	if got.ExpectedRevision != 3 {
		t.Fatalf("expected revision = %d", got.ExpectedRevision)
	}
	if got.OperationID == "" {
		t.Fatalf("operation id empty")
	}
	if d.Revision != 5 {
		t.Fatalf("revision = %d", d.Revision)
	}
	if d.Dirty {
		t.Fatalf("dirty not cleared")
	}
	if d.SaveState != SaveSaved {
		t.Fatalf("save state = %s", d.SaveState)
	}
}

func TestErrorDraftRetention(t *testing.T) {
	d := sampleDesigner(t)
	d.Service = nil
	d.Callbacks.SaveDesign = func(ctx context.Context, input runs.DesignSaveInput) (runs.DesignSnapshot, error) {
		return runs.DesignSnapshot{}, &typedErr{msg: "CONFLICT: stale"}
	}
	before := len(d.Editor.Doc.Nodes)
	d.Save(context.Background())
	if d.SaveState != SaveError {
		t.Fatalf("state = %s", d.SaveState)
	}
	if len(d.Editor.Doc.Nodes) != before {
		t.Fatalf("draft mutated")
	}
	if d.Error == "" {
		t.Fatalf("no error")
	}
}

type typedErr struct{ msg string }

func (e *typedErr) Error() string { return e.msg }

func TestLoadApply(t *testing.T) {
	doc := map[string]any{
		"designId": "design-1",
		"title":    "Loaded",
		"workflow": map[string]any{"nodes": []any{}, "edges": []any{}},
	}
	b, _ := json.Marshal(doc)
	fake := &state.FakeDesignService{
		Snapshot: runs.DesignSnapshot{DesignID: "design-1", Revision: 7, Title: "Loaded", Data: string(b)},
	}
	d := New("design-1", "X", 1, fake)
	d.Load(context.Background(), "design-1")
	if d.Revision != 7 {
		t.Fatalf("rev=%d", d.Revision)
	}
	if d.Title != "Loaded" {
		t.Fatalf("title=%s", d.Title)
	}
	if d.Loading {
		t.Fatalf("loading true")
	}
}

func TestEmptyState(t *testing.T) {
	fake := &state.FakeDesignService{
		Snapshot: runs.DesignSnapshot{DesignID: "design-empty", Revision: 1, Title: "Empty", Data: ""},
	}
	d := New("design-empty", "Empty", 0, fake)
	d.Load(context.Background(), "design-empty")
	if !d.Empty {
		t.Fatalf("not empty")
	}
	if len(d.Editor.Doc.Nodes) != 0 || len(d.Assays) != 0 {
		t.Fatalf("empty design received prefilled data: nodes=%d assays=%d", len(d.Editor.Doc.Nodes), len(d.Assays))
	}
	tt := ui.NewTester(func(c *ui.Context) { View(c, d) }, 1200, 760)
	tt.Frame()
	_ = tt.Image()
}

func TestScreenshot(t *testing.T) {
	d := sampleDesigner(t)
	tt := ui.NewTester(func(c *ui.Context) { View(c, d) }, 1200, 760)
	tt.Frame()
	f, err := os.Create("designer.png")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, tt.Image()); err != nil {
		t.Fatal(err)
	}
}

type persistedDesignService struct {
	runs.DesignService
	snapshot runs.DesignSnapshot
	created  bool
}

func (s *persistedDesignService) ListDesigns(context.Context) ([]runs.DesignSummary, error) {
	return nil, nil
}
func (s *persistedDesignService) LoadDesign(context.Context, string) (runs.DesignSnapshot, error) {
	return s.snapshot, nil
}
func (s *persistedDesignService) SaveDesign(_ context.Context, in runs.DesignSaveInput) (runs.DesignSnapshot, error) {
	s.snapshot = runs.DesignSnapshot{DesignID: in.DesignID, Revision: in.ExpectedRevision + 1, Title: "Saved design", Data: in.Changes}
	return s.snapshot, nil
}
func (s *persistedDesignService) CreateRun(_ context.Context, designID, _ string) (runs.RunSnapshot, error) {
	s.created = true
	return runs.RunSnapshot{DesignID: designID, RunID: "run-created"}, nil
}
func (s *persistedDesignService) Close() error { return nil }

func TestDesignerUIMapSaveReopenCreateRun(t *testing.T) {
	svc := &persistedDesignService{snapshot: runs.DesignSnapshot{DesignID: "design-1", Revision: 2, Title: "Test"}}
	d := New("design-1", "Test", 2, svc)
	tt := ui.NewTester(func(c *ui.Context) { View(c, d) }, 1200, 760)
	d.Editor.AddNode(workflow.KindFlask, "Flask", workflow.Position{X: 80, Y: 80})
	tt.Frame()
	if d.SelectedNode == "" {
		t.Fatal("workflow editor did not select the new step")
	}
	d.setNodeTitle(d.SelectedNode, "Incubate cells")
	d.AssayIDDraft = assays.CCK8
	d.StepAssays[d.SelectedNode] = assays.CCK8
	d.Assays[assays.CCK8] = assays.Design{Timepoints: []string{"24 h"}}
	d.selectWells([]string{"A1"})
	d.SampleIDDraft, d.GroupIDDraft, d.WellTimepointDraft = "sample-1", "control", "24 h"
	d.WellReplicatesDraft = "1"
	tt.Frame()
	if !containsString(d.SelectedWells, "A1") {
		t.Fatal("well selection did not update")
	}
	if err := d.AssignSelectedWells(); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if got := len(d.Assays[assays.CCK8].Wells); got != 1 {
		t.Fatalf("assigned wells=%d", got)
	}
	d.Save(context.Background())
	if d.SaveState != SaveSaved {
		t.Fatalf("save state=%s error=%s", d.SaveState, d.Error)
	}

	reopened := New("design-1", "Test", 0, svc)
	reopened.Load(context.Background(), "design-1")
	if got := reopened.nodeTitle(reopened.Editor.Doc.Nodes[0].ID); got != "Incubate cells" {
		t.Fatalf("reopened title=%q", got)
	}
	if got := reopened.Assays[assays.CCK8].Wells; len(got) != 1 || got[0].WellID != "A1" {
		t.Fatalf("reopened mapping=%+v", got)
	}
	reopened.Editor.SelectNode(reopened.Editor.Doc.Nodes[0].ID)
	tt = ui.NewTester(func(c *ui.Context) { View(c, reopened) }, 1200, 760)
	tt.Frame()
	f, err := os.Create("designer-mapping-roundtrip.png")
	if err != nil {
		t.Fatal(err)
	}
	if err = png.Encode(f, tt.Image()); err != nil {
		f.Close()
		t.Fatal(err)
	}
	if err = f.Close(); err != nil {
		t.Fatal(err)
	}
	reopened.setNodeTitle(reopened.Editor.Doc.Nodes[0].ID, "Updated step")
	var created runs.RunSnapshot
	reopened.OnCreateRun = func(s runs.RunSnapshot) { created = s }
	reopened.CreateRun(context.Background())
	if !svc.created || created.RunID != "run-created" {
		t.Fatalf("CreateRun not reached: %+v", created)
	}
	if reopened.Dirty || reopened.SaveState != SaveSaved {
		t.Fatalf("run creation failed to save draft: %s", reopened.SaveState)
	}
}
func TestAssignSelectedWellsPreservesOtherTimepoints(t *testing.T) {
	d := New("design-1", "Test Design", 0, nil)
	nodeID := d.Editor.AddNode(workflow.KindFlask, "Step", workflow.Position{})
	assayID := "assay-1"
	d.AssayIDDraft = assayID
	d.StepAssays[nodeID] = assayID
	d.Assays[assayID] = assays.Design{Wells: []assays.WellAssignment{
		{WellID: "A1", SampleID: "sample-old", GroupID: "old", Replicate: 1, Timepoint: "24 h"},
		{WellID: "A1", SampleID: "sample-old", GroupID: "old", Replicate: 1, Timepoint: "48 h"},
	}}
	d.selectWells([]string{"A1"})
	d.SampleIDDraft, d.GroupIDDraft = "sample-new", "new"
	d.WellTimepointDraft, d.WellReplicatesDraft = "24 h", "2"

	if err := d.AssignSelectedWells(); err != nil {
		t.Fatal(err)
	}
	got := d.Assays[assayID].Wells
	if len(got) != 2 || got[0].SampleID != "sample-new" || got[0].GroupID != "new" || got[0].Replicate != 2 {
		t.Fatalf("matching timepoint was not replaced: %+v", got)
	}
	if got[1].Timepoint != "48 h" || got[1].SampleID != "sample-old" || got[1].GroupID != "old" {
		t.Fatalf("other timepoint was changed: %+v", got)
	}
}
func TestCreateRunRejectsEmptyDesign(t *testing.T) {
	svc := &persistedDesignService{}
	d := New("design-1", "Empty", 0, svc)

	d.CreateRun(context.Background())
	if svc.created {
		t.Fatal("created a Run from an empty design")
	}
	if d.Error == "" {
		t.Fatal("empty design was not rejected")
	}
}

// TestOnevraDesignerScreenshot renders the real Designer with a synthetic
// in-memory design: three workflow steps, one measured endpoint plan, and six
// mapped wells. It is visual evidence only; nothing is written to a database.
func TestOnevraDesignerScreenshot(t *testing.T) {
	d := New("design-1", "CCK-8 设计工作区", 2, &state.FakeDesignService{})
	d.Editor.Doc.Nodes = []workflow.Node{
		{ID: "n1", Kind: workflow.KindFlask, Title: "GelMA 水凝胶", Tag: "[材料 · 样本]", Order: "01", Pos: workflow.Position{X: 60, Y: 90}},
		{ID: "n2", Kind: workflow.KindPipette, Title: "铺板 96 孔", Order: "02", Params: []workflow.NodeParam{{Key: "体积", Val: "100 µL"}, {Key: "重复", Val: "3"}}, Pos: workflow.Position{X: 340, Y: 90}},
		{ID: "n3", Kind: workflow.KindPlate, Title: "酶标仪读数", Order: "03", Params: []workflow.NodeParam{{Key: "波长", Val: "450 nm"}}, Pos: workflow.Position{X: 620, Y: 90}},
	}
	d.Editor.Doc.Edges = []workflow.Edge{{ID: "e1", From: "n1", To: "n2"}, {ID: "e2", From: "n2", To: "n3"}}
	d.Editor.SelectedNode, d.SelectedNode = "n2", "n2"
	d.Assays["cck8"] = assays.Design{
		Timepoints: []string{"24 h"}, ControlGroup: "Control",
		Wells: []assays.WellAssignment{
			{WellID: "A1", SampleID: "C1", GroupID: "Control", Replicate: 1, Timepoint: "24 h"},
			{WellID: "A2", SampleID: "C2", GroupID: "Control", Replicate: 2, Timepoint: "24 h"},
			{WellID: "B1", SampleID: "T1", GroupID: "C1", Replicate: 1, Timepoint: "24 h"},
			{WellID: "B2", SampleID: "T2", GroupID: "C1", Replicate: 2, Timepoint: "24 h"},
			{WellID: "C1", SampleID: "T3", GroupID: "C2", Replicate: 1, Timepoint: "24 h"},
			{WellID: "C2", SampleID: "T4", GroupID: "C2", Replicate: 2, Timepoint: "24 h"},
		},
	}
	d.AssayIDDraft, d.StepAssays["n2"] = "cck8", "cck8"
	// Reusable modules for the selected step, built through the same block
	// definitions the designer's add menu uses.
	for _, typ := range []blocks.BlockType{blocks.TypeNote, blocks.TypeDilutionCalculator, blocks.TypeParameterTable} {
		def := blockui.NewDefinition(typ)
		inst, err := blocks.NewInstance(string(typ), def)
		if err != nil {
			t.Fatal(err)
		}
		if typ == blocks.TypeNote {
			inst.RawProps = []byte(`{"title":"铺板条件","text":"每孔 100 µL，37 °C 过夜。"}`)
		}
		if typ == blocks.TypeParameterTable {
			inst.RawProps = []byte(`{"rows":[{"key":"接种密度","value":"5×10^3","unit":"cells/well"}]}`)
		}
		d.Instances["n2"] = append(d.Instances["n2"], inst)
	}
	d.SelectedWells = []string{"A1", "A2"}
	d.Touch()
	for dock, name := range []string{"designer-onevra-canvas.png", "designer-onevra-dock-bottom.png"} {
		d.InspectorDock = Dock(dock)
		tt := ui.NewTester(func(c *ui.Context) {
			c.SetTheme(theme.LightTheme())
			View(c, d)
		}, 1100, 820)
		tt.Frame()
		f, err := os.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(f, tt.Image()); err != nil {
			f.Close()
			t.Fatal(err)
		}
		if err := f.Close(); err != nil {
			t.Fatal(err)
		}
	}
}

// TestToolbarAddsSteps drives the one command row: each kind adds a step and
// the canvas shows it.
func TestToolbarAddsSteps(t *testing.T) {
	d := New("design-1", "Test Design", 1, &state.FakeDesignService{})
	tt := ui.NewTester(func(c *ui.Context) { View(c, d) }, 1200, 760)
	tt.Frame()
	for _, label := range []string{"新建材料步骤", "新建操作步骤", "新建仪器步骤"} {
		if err := tt.Click(label); err != nil {
			t.Fatal(err)
		}
		tt.Frame()
	}
	if len(d.Editor.Doc.Nodes) != 3 {
		t.Fatalf("toolbar added %d steps, want 3: %+v", len(d.Editor.Doc.Nodes), d.Editor.Doc.Nodes)
	}
	if !tt.HasText("新材料") || !tt.HasText("新操作") || !tt.HasText("新仪器") {
		t.Fatalf("canvas missing new steps: %q", tt.Texts())
	}
	if f, err := os.Create("evidence-canvas-toolbar.png"); err == nil {
		png.Encode(f, tt.Image())
		f.Close()
	}
}

// TestInspectorPanelControls drives the floating panel itself: close, reopen
// from the toolbar, and both dock switches. Drag is the one gesture a headless
// tester cannot synthesize, so the dock buttons are the automated proof and
// drag remains covered by the shared dock decision.
func TestInspectorPanelControls(t *testing.T) {
	d := New("design-1", "Test Design", 1, &state.FakeDesignService{})
	d.Editor.AddNode(workflow.KindFlask, "细胞悬液", workflow.Position{X: 80, Y: 80})
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.LightTheme())
		View(c, d)
	}, 1100, 820)
	tt.Frame()
	if !d.InspectorOpen || !tt.HasText("步骤属性") {
		t.Fatalf("panel did not open: open=%v texts=%q", d.InspectorOpen, tt.Texts())
	}
	if d.InspectorDock != DockRight {
		t.Fatalf("default dock = %v, want right", d.InspectorDock)
	}

	if err := tt.Click("停靠到底部"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if d.InspectorDock != DockBottom {
		t.Fatalf("dock = %v after 停靠到底部", d.InspectorDock)
	}
	if err := tt.Click("停靠到右侧"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if d.InspectorDock != DockRight {
		t.Fatalf("dock = %v after 停靠到右侧", d.InspectorDock)
	}

	if err := tt.Click("收起检查器"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if d.InspectorOpen || tt.HasText("步骤属性") {
		t.Fatalf("panel did not close: open=%v texts=%q", d.InspectorOpen, tt.Texts())
	}
	if err := tt.Click("展开检查器"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !d.InspectorOpen || !tt.HasText("步骤属性") {
		t.Fatalf("panel did not reopen: open=%v texts=%q", d.InspectorOpen, tt.Texts())
	}
}

// TestDesignerAuthorsStepTasks pins the explicit execution plan: the step's own
// timepoints materialize as StepTasks on save, and a step with none contributes
// nothing rather than claiming its assay wholesale.
func TestDesignerAuthorsStepTasks(t *testing.T) {
	d := New("design-1", "Test Design", 1, nil)
	node := d.Editor.AddNode(workflow.KindFlask, "铺板", workflow.Position{X: 80, Y: 80})
	d.StepAssays[node] = "cck8"
	d.StepTimepoints[node] = []string{"D1", "D3"}
	d.StepAssays["orphan"] = "cck8" // no timepoints: must not be claimed
	var got string
	d.Callbacks.SaveDesign = func(_ context.Context, in runs.DesignSaveInput) (runs.DesignSnapshot, error) {
		got = in.Changes
		return runs.DesignSnapshot{DesignID: in.DesignID, Revision: in.ExpectedRevision + 1}, nil
	}
	d.Dirty = true
	d.Save(context.Background())
	var doc designDoc
	if err := json.Unmarshal([]byte(got), &doc); err != nil {
		t.Fatal(err)
	}
	if len(doc.MyGo.StepTasks) != 2 {
		t.Fatalf("stepTasks = %+v, want the two authored timepoints", doc.MyGo.StepTasks)
	}
	for _, task := range doc.MyGo.StepTasks {
		if task.StepID != node || task.AssayID != "cck8" || (task.Timepoint != "D1" && task.Timepoint != "D3") {
			t.Fatalf("unexpected stepTask %+v", task)
		}
	}
}

// TestDesignerLoadsStepTasks checks the round trip: a saved design restores the
// step's own timepoints and shows them for the selected step.
func TestDesignerLoadsStepTasks(t *testing.T) {
	data := `{"mygo":{"workflow":{"Nodes":[{"id":"n1","kind":"flask","title":"铺板"}]},` +
		`"stepAssays":{"n1":"cck8"},"stepTasks":[{"stepId":"n1","assayId":"cck8","timepoint":"D1","path":"root/read"},` +
		`{"stepId":"n1","assayId":"cck8","timepoint":"D3"}]}}`
	fake := &state.FakeDesignService{
		Snapshot: runs.DesignSnapshot{DesignID: "design-1", Revision: 2, Title: "T", Data: data},
	}
	d := New("design-1", "T", 0, fake)
	d.Load(context.Background(), "design-1")
	if got := d.StepTimepoints["n1"]; len(got) != 2 || got[0] != "D1" || got[1] != "D3" {
		t.Fatalf("step timepoints = %v", got)
	}
	if d.StepTimepointsDraft != "D1, D3" {
		t.Fatalf("draft = %q", d.StepTimepointsDraft)
	}
	tasks := d.stepTasks()
	if len(tasks) != 2 {
		t.Fatalf("rebuilt stepTasks = %+v", tasks)
	}
	paths := map[string]string{}
	for _, task := range tasks {
		paths[task.Timepoint] = task.Path
	}
	if paths["D1"] != "root/read" || paths["D3"] != "" {
		t.Fatalf("saved paths were not preserved: %+v", tasks)
	}
	// Re-saving must not drop the path-qualified entry.
	var saved string
	d.Callbacks.SaveDesign = func(_ context.Context, in runs.DesignSaveInput) (runs.DesignSnapshot, error) {
		saved = in.Changes
		return runs.DesignSnapshot{DesignID: in.DesignID, Revision: in.ExpectedRevision + 1}, nil
	}
	d.Dirty = true
	d.Save(context.Background())
	var doc designDoc
	if err := json.Unmarshal([]byte(saved), &doc); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, task := range doc.MyGo.StepTasks {
		if task.Timepoint == "D1" && task.Path == "root/read" {
			found = true
		}
	}
	if !found {
		t.Fatalf("save dropped the path: %+v", doc.MyGo.StepTasks)
	}
}

// TestCreateRunPreflightWarnsOnUnlinkedTasks pins the preflight split: an
// unresolved execution plan is surfaced for a decision, never silently frozen
// and never blocked outright.
func TestCreateRunPreflightWarnsOnUnlinkedTasks(t *testing.T) {
	svc := &persistedDesignService{snapshot: runs.DesignSnapshot{DesignID: "design-1", Revision: 1, Title: "T"}}
	d := New("design-1", "T", 1, svc)
	node := d.Editor.AddNode(workflow.KindFlask, "铺板", workflow.Position{X: 80, Y: 80})
	d.StepAssays[node] = "cck8"
	d.Assays["cck8"] = assays.Design{Timepoints: []string{"24 h"}}

	d.RequestCreateRun(context.Background())
	if svc.created {
		t.Fatal("run was created before the unresolved plan was confirmed")
	}
	if !d.ConfirmCreateRun || len(d.CreateRunWarnings) == 0 {
		t.Fatalf("no preflight dialog: confirm=%v warnings=%v", d.ConfirmCreateRun, d.CreateRunWarnings)
	}
	if !strings.Contains(d.CreateRunWarnings[0], "没有填写本步骤时间点") {
		t.Fatalf("warning does not name the missing timepoints: %q", d.CreateRunWarnings)
	}

	// The researcher may still freeze the design as it is.
	d.ConfirmCreateRun = false
	d.CreateRunWarnings = nil
	d.CreateRun(context.Background())
	if !svc.created {
		t.Fatal("confirmed design did not create a run")
	}
}

func TestCreateRunPreflightDetectsDuplicateClaimsAndUndeclaredTimepoints(t *testing.T) {
	d := New("design-1", "T", 1, nil)
	a := d.Editor.AddNode(workflow.KindFlask, "铺板 A", workflow.Position{})
	b := d.Editor.AddNode(workflow.KindPipette, "铺板 B", workflow.Position{X: 240})
	d.StepAssays[a], d.StepAssays[b] = "cck8", "cck8"
	d.StepTimepoints[a], d.StepTimepoints[b] = []string{"24 h"}, []string{"24 h"}
	d.Assays["cck8"] = assays.Design{
		Timepoints: []string{"24 h"},
		Wells: []assays.WellAssignment{{
			WellID: "A1", SampleID: "S1", GroupID: "Control", Replicate: 1, Timepoint: "48 h",
		}},
	}
	warnings := d.PreflightWarnings()
	joined := strings.Join(warnings, "\n")
	if !strings.Contains(joined, "同时被 2 个步骤声明") {
		t.Fatalf("duplicate claim not reported: %v", warnings)
	}
	if !strings.Contains(joined, "未在检测时间点中声明的时间点 48 h") {
		t.Fatalf("undeclared well timepoint not reported: %v", warnings)
	}
	if errs := d.PreflightErrors(); len(errs) != 0 {
		t.Fatalf("valid wells reported as errors: %v", errs)
	}
}

// TestCreateRunDialogFlow drives the preflight dialog: create opens it, and
// 仍要创建 freezes the design.
func TestCreateRunDialogFlow(t *testing.T) {
	svc := &persistedDesignService{snapshot: runs.DesignSnapshot{DesignID: "design-1", Revision: 1, Title: "T"}}
	d := New("design-1", "T", 1, svc)
	node := d.Editor.AddNode(workflow.KindFlask, "铺板", workflow.Position{X: 80, Y: 80})
	d.StepAssays[node] = "cck8"
	tt := ui.NewTester(func(c *ui.Context) { View(c, d) }, 1200, 760)
	tt.Frame()
	if err := tt.Click("创建运行"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("创建运行前检查") || !tt.HasText("仍要创建") {
		t.Fatalf("preflight dialog missing: %q", tt.Texts())
	}
	if svc.created {
		t.Fatal("dialog opened after creating the run")
	}
	if err := tt.Click("返回修改"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if d.ConfirmCreateRun || svc.created {
		t.Fatalf("返回修改 did not cancel: confirm=%v created=%v", d.ConfirmCreateRun, svc.created)
	}
}
