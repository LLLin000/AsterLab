package main

import (
	"context"
	"fmt"
	"log"
	"math"
	"os"
	"sync/atomic"

	"asterlab-mygo-prototype/internal/blocks"
	"asterlab-mygo-prototype/internal/designer"
	"asterlab-mygo-prototype/internal/runui"
	"asterlab-mygo-prototype/internal/shell"
	"asterlab-mygo-prototype/internal/state"
	"asterlab-mygo-prototype/internal/workflow"
	"asterlab-mygo-prototype/modules/runs"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

type sample struct{ id, value string }

type app struct {
	page    *string
	step    *string
	samples []sample

	runSvc    runs.RunService
	designSvc runs.DesignService
	snap      runs.RunSnapshot
	err       string

	shell    *shell.Shell
	ws       *runui.Workspace
	designer *designer.Designer
	guard    state.StaleGuard

	designSnap *runs.DesignSnapshot

	wf     *workflow.Editor
	blocks []blocks.Instance

	opCounter atomic.Uint64

	selectedTask string
}

func (a *app) nextOp() string {
	n := a.opCounter.Add(1)
	return fmt.Sprintf("op-%d", n)
}

func toRunView(s runs.RunSnapshot) runui.RunView {
	v := runui.RunView{
		Title:        s.Title,
		Status:       s.Status,
		Revision:     s.Revision,
		Measurements: make([]runui.Measurement, 0, len(s.Measurements)),
		Events:       make([]runui.Event, 0, len(s.Events)),
	}
	for _, t := range s.Tasks {
		v.Tasks = append(v.Tasks, runui.Task{Key: t.Key, Title: t.Title, Timepoint: t.Timepoint, Status: t.Status, AssayID: t.AssayID})
	}
	for _, m := range s.Measurements {
		v.Measurements = append(v.Measurements, runui.Measurement{SampleID: m.SampleID, Endpoint: m.Endpoint, Value: m.Value, Unit: m.Unit})
	}
	for _, e := range s.Events {
		v.Events = append(v.Events, runui.Event{Kind: e.EventType, Message: e.Content})
	}
	return v
}

func (a *app) applySnap(snap runs.RunSnapshot) {
	if !a.guard.Accept(snap.Revision) {
		a.err = "忽略过期快照"
		return
	}
	newSnap, err := state.ApplySnapshot(a.snap, snap)
	if err != nil {
		a.err = err.Error()
		return
	}
	if err := state.ConflictIfDifferent(a.snap, snap); err != nil {
		a.err = err.Error()
	}
	a.snap = newSnap
	if a.ws != nil {
		view := toRunView(a.snap)
		a.ws.View = view
		// A loaded/saved snapshot is authoritative for the rows shown.
		a.ws.Draft = runui.NewDraft(view)
		if len(a.snap.Tasks) == 0 && len(a.snap.Measurements) == 0 {
			a.ws.State = runui.StateEmpty
		} else {
			a.ws.State = runui.StateReady
		}
	}
}

func (a *app) loadDesign(id string) {
	if a.designSvc == nil {
		return
	}
	a.designer = designer.New(id, "", 0, a.designSvc)
	a.designer.OnCreateRun = func(snap runs.RunSnapshot) {
		a.applySnap(snap)
		a.shell.Page = shell.Run
	}
	a.designer.Load(context.Background(), id)
	snap, err := a.designSvc.LoadDesign(context.Background(), id)
	if err != nil {
		a.err = state.MapError(err).Error()
		return
	}
	a.designSnap = &snap
}

func (a *app) loadDesigns() {
	if a.designSvc == nil {
		return
	}
	ds, err := a.designSvc.ListDesigns(context.Background())
	if err != nil {
		a.err = state.MapError(err).Error()
		return
	}
	a.shell.Designs = make([]shell.DesignItem, 0, len(ds))
	for _, d := range ds {
		a.shell.Designs = append(a.shell.Designs, shell.DesignItem{ID: d.DesignID, Title: d.Title, Meta: fmt.Sprintf("rev %d", d.Revision)})
	}
	if a.shell.Selected == "" && len(ds) > 0 {
		a.shell.Selected = ds[0].DesignID
		a.loadDesign(ds[0].DesignID)
	}
}

func (a *app) loadRun() {
	if a.runSvc == nil {
		if a.ws != nil {
			a.ws.State = runui.StateBlocked
			a.ws.Error = "Rust RunService 不可用"
		}
		return
	}
	runID := os.Getenv("ASTERLAB_RUN_ID")
	if runID == "" {
		runID = "1c0c8989-0513-483a-851b-bdc7d6c67483"
	}
	snap, err := a.runSvc.LoadRun(context.Background(), runID)
	if err != nil {
		if a.ws != nil {
			a.ws.State = runui.StateBlocked
			a.ws.Error = state.MapError(err).Error()
		}
		a.err = state.MapError(err).Error()
		return
	}
	a.applySnap(snap)
	if a.ws != nil && a.ws.State == runui.StateLoading {
		if len(a.snap.Tasks) == 0 && len(a.snap.Measurements) == 0 {
			a.ws.State = runui.StateEmpty
		} else {
			a.ws.State = runui.StateReady
		}
	}
}

func isFinite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }

func (a *app) view(c *ui.Context) {
	shell.View(c, a.shell)
}

func designPage(c *ui.Context, a *app) {
	ui.Column(c).Fill().Padding(16).Gap(10).Children(func() {
		ui.Text(c, "实验设计").FontSize(20).Bold()
		if a.designer != nil {
			designer.View(c, a.designer)
			return
		}
		ui.Text(c, "DesignService 不可用").TextColor(ui.Hex("#b00020"))
	})
}

func protocolPage(c *ui.Context, a *app) {
	t := c.Theme()
	ui.Column(c).Fill().Padding(16).Gap(10).Children(func() {
		ui.Text(c, "Protocol（只读）").FontSize(20).Bold()
		if len(a.snap.Tasks) == 0 {
			ui.Text(c, "无 Protocol 任务").TextColor(t.TextMuted)
			return
		}
		for _, task := range a.snap.Tasks {
			ui.Text(c, fmt.Sprintf("%s · %s · %s", task.Timepoint, task.Title, task.Status))
		}
	})
}

func toolsPage(c *ui.Context, a *app) {
	t := c.Theme()
	ui.Column(c).Fill().Padding(16).Gap(10).Children(func() {
		ui.Text(c, "资料与工具（只读）").FontSize(20).Bold()
		for _, inst := range a.blocks {
			ui.Column(c).Padding(8).Radius(6).Border(1, t.Border).Children(func() {
				blocks.View(c, inst)
			})
		}
	})
}

func main() {
	page, step := "run", "design-l929"
	_ = step

	runSvc, _ := runs.NewRunService()
	designSvc, _ := runs.NewDesignService()

	a := &app{
		page:      &page,
		step:      &step,
		runSvc:    runSvc,
		designSvc: designSvc,
		snap:      runs.RunSnapshot{Title: "加载中", Status: "loading"},
	}
	a.shell = &shell.Shell{
		Project:     "L929",
		Page:        shell.Run,
		SidebarOpen: true,
		OnSelect:    func(id string) { a.shell.Selected = id; a.loadDesign(id) },
		OnPage:      func(p shell.Page) { a.shell.Page = p },
		Content:     func(c *ui.Context) { contentPage(c, a) },
	}

	// runui workspace.
	cb := runui.Callbacks{
		SaveReadout: func(ms []runui.Measurement) {
			if a.ws == nil {
				return
			}
			rMs := make([]runs.Measurement, 0, len(ms))
			for _, m := range ms {
				rMs = append(rMs, runs.Measurement{SampleID: m.SampleID, Endpoint: m.Endpoint, Value: m.Value, Unit: m.Unit})
			}
			taskKey := a.ws.Selected
			timepoint, assayID, stepPath, protocolVersion := "", "", "", ""
			for _, t := range a.snap.Tasks {
				if t.Key == taskKey {
					timepoint, assayID, stepPath, protocolVersion = t.Timepoint, t.AssayID, t.Title, t.ProtocolVersionID
				}
			}
			if taskKey == "" && len(a.snap.Tasks) > 0 {
				t := a.snap.Tasks[0]
				taskKey, timepoint, assayID, stepPath, protocolVersion = t.Key, t.Timepoint, t.AssayID, t.Title, t.ProtocolVersionID
			}
			opID := a.nextOp()
			snap, err := a.runSvc.SaveReadout(context.Background(), runs.ReadoutInput{
				RunID:             a.snap.RunID,
				TaskKey:           taskKey,
				Timepoint:         timepoint,
				AssayID:           assayID,
				ProtocolStepPath:  stepPath,
				ProtocolVersionID: protocolVersion,
				Measurements:      rMs,
				OperationID:       opID,
			})
			if err != nil {
				a.ws.State = runui.StateBlocked
				a.ws.Error = state.MapError(err).Error()
				a.ws.Draft.Error = state.MapError(err).Error()
				a.err = state.MapError(err).Error()
				return
			}
			a.applySnap(snap)
			if a.ws != nil {
				a.ws.State = runui.StateSaved
			}
		},
		CompleteTask: func(taskKey string) {
			if a.ws == nil {
				return
			}
			if taskKey == "" {
				taskKey = a.ws.Selected
			}
			opID := a.nextOp()
			timepoint, assayID, stepPath, protocolVersion := "", "", "", ""
			for _, t := range a.snap.Tasks {
				if t.Key == taskKey {
					timepoint, assayID, stepPath, protocolVersion = t.Timepoint, t.AssayID, t.Title, t.ProtocolVersionID
				}
			}
			snap, err := a.runSvc.CompleteTask(context.Background(), runs.CompleteTaskInput{
				RunID:             a.snap.RunID,
				TaskKey:           taskKey,
				OperationID:       opID,
				AssayID:           assayID,
				Timepoint:         timepoint,
				ProtocolStepPath:  stepPath,
				ProtocolVersionID: protocolVersion,
			})
			if err != nil {
				a.ws.State = runui.StateBlocked
				a.ws.Error = state.MapError(err).Error()
				a.err = state.MapError(err).Error()
				return
			}
			a.applySnap(snap)
			if a.ws != nil {
				a.ws.State = runui.StateReady
			}
		},
		SelectTask: func(taskKey string) { a.selectedTask = taskKey },
	}
	a.ws = runui.NewWorkspace(toRunView(a.snap), cb)
	a.ws.State = runui.StateLoading
	a.ws.Width = 1200

	mygo.App.WhenReady(func() {
		go func() {
			a.loadDesigns()
			a.loadRun()
		}()
		mygo.NewWindow(mygo.WindowOptions{
			Title:         "AsterLab MyGo Prototype",
			Width:         1200,
			Height:        760,
			TitleBarStyle: mygo.TitleBarHidden,
			Content:       ui.View(a.view),
		})
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}

func contentPage(c *ui.Context, a *app) {
	switch a.shell.Page {
	case shell.Design:
		designPage(c, a)
	case shell.Protocol:
		protocolPage(c, a)
	case shell.Run:
		if a.ws != nil {
			a.ws.Width = 1200
			runui.View(c, a.ws)
		} else {
			ui.Text(c, "加载中…")
		}
	case shell.Tools:
		toolsPage(c, a)
	}
}
