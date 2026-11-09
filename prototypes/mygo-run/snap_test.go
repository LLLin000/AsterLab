package main

import (
	"context"
	"image/png"
	"os"
	"strconv"
	"testing"

	"asterlab-mygo-prototype/internal/designer"
	"asterlab-mygo-prototype/internal/runui"
	"asterlab-mygo-prototype/internal/shell"
	"asterlab-mygo-prototype/internal/workflow"
	"asterlab-mygo-prototype/modules/runs"

	"github.com/egoist/mygo/ui"
)

func TestSnap(t *testing.T) {
	page, step := "run", "design-l929"
	a := &app{page: &page, step: &step, samples: []sample{{"Control-1", ""}, {"E1-1", ""}}}
	a.snap = runs.RunSnapshot{Title: "L929 生物相容性", Status: "进行中", Tasks: []runs.RunTask{{Key: "a", Title: "Seed", Timepoint: "D1", Status: "completed"}, {Key: "b", Title: "CCK-8", Timepoint: "D3", Status: "not_started"}, {Key: "c", Title: "Image", Timepoint: "D7", Status: "not_started"}}, Measurements: []runs.Measurement{{SampleID: "Control-1", Endpoint: "OD450", Value: 0.82, Unit: "AU"}}}
	a.shell = &shell.Shell{Project: "L929", Page: shell.Run, SidebarOpen: true, Designs: []shell.DesignItem{{ID: "d1", Title: "L929 生物相容性"}}, Selected: "d1", Content: func(c *ui.Context) { contentPage(c, a) }}
	a.ws = runui.NewWorkspace(toRunView(a.snap), runui.Callbacks{})
	a.ws.State = runui.StateReady
	for _, sz := range []struct{ w, h int }{{1200, 760}, {900, 600}} {
		tt := ui.NewTester(a.view, sz.w, sz.h)
		tt.Frame()
		name := "snapshot-" + strconv.Itoa(sz.w) + "x" + strconv.Itoa(sz.h) + ".png"
		f, err := os.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(f, tt.Image()); err != nil {
			f.Close()
			t.Fatal(err)
		}
		f.Close()
	}
}

func TestDesignerRealCopiedDBSaveReopenCreateRun(t *testing.T) {
	copyFixtureDB(t)
	ctx := context.Background()
	dsvc, err := runs.NewDesignService()
	if err != nil {
		t.Fatal(err)
	}
	defer dsvc.Close()
	designs, err := dsvc.ListDesigns(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(designs) == 0 {
		t.Skip("no designs in fixture")
	}
	id := designs[0].DesignID
	a := &app{designSvc: dsvc, shell: &shell.Shell{}}
	a.loadDesign(id)
	d := a.designer
	d.Editor.AddNode(workflow.KindFlask, "EditedNode", workflow.Position{X: 10, Y: 10})
	if !d.Dirty || d.SaveState != designer.SaveDraft {
		t.Fatalf("editor change state = dirty:%v save:%s", d.Dirty, d.SaveState)
	}
	d.Save(ctx)
	if d.SaveState != designer.SaveSaved {
		t.Fatalf("save state = %s (%s)", d.SaveState, d.Error)
	}
	savedRev := d.Revision
	// reopen with a fresh service (GUI-like second connection)
	dsvc2, err := runs.NewDesignService()
	if err != nil {
		t.Fatal(err)
	}
	defer dsvc2.Close()
	a2 := &app{designSvc: dsvc2, shell: &shell.Shell{}}
	a2.loadDesign(id)
	d2 := a2.designer
	found := false
	for _, n := range d2.Editor.Doc.Nodes {
		if n.Title == "EditedNode" {
			found = true
		}
	}
	if !found {
		t.Fatal("edited node not found after reopen")
	}
	d2.CreateRun(ctx)
	if d2.Error != "" {
		t.Fatalf("create run error: %s", d2.Error)
	}
	if a2.snap.RunID == "" {
		t.Fatal("create run did not return snapshot")
	}
	if a2.snap.DesignRevision != savedRev {
		t.Fatalf("create run design revision = %d, want %d", a2.snap.DesignRevision, savedRev)
	}
}
