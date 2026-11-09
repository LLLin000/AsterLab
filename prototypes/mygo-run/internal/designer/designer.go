package designer

import (
	"context"
	"encoding/json"
	"fmt"
	"sync/atomic"
	"time"

	"asterlab-mygo-prototype/internal/blocks"
	"asterlab-mygo-prototype/internal/workflow"
	"asterlab-mygo-prototype/modules/runs"

	"github.com/egoist/mygo/ui"
)

// SaveState tracks explicit design save lifecycle.
type SaveState string

const (
	SaveIdle   SaveState = "idle"
	SaveDraft  SaveState = "draft"
	SaveSaving SaveState = "saving"
	SaveSaved  SaveState = "saved"
	SaveError  SaveState = "error"
)

// designDoc is the serialized design payload stored in DesignSnapshot.Data/Changes.
type designDoc struct {
	DesignID  string                       `json:"designId"`
	Title     string                       `json:"title"`
	Workflow  workflow.WorkflowDocument    `json:"workflow"`
	Instances map[string][]blocks.Instance `json:"instances,omitempty"`
}

type Callbacks struct {
	LoadDesign func(ctx context.Context, designID string) (runs.DesignSnapshot, error)
	SaveDesign func(ctx context.Context, input runs.DesignSaveInput) (runs.DesignSnapshot, error)
	CreateRun  func(ctx context.Context, designID string, operationID string) (runs.RunSnapshot, error)
}

// Designer composes workflow + blocks into one design view state.
type Designer struct {
	DesignID string
	Title    string
	Revision uint64

	Editor    *workflow.Editor
	Instances map[string][]blocks.Instance

	SelectedNode string

	Dirty     bool
	SaveState SaveState
	Error     string
	Loading   bool
	Empty     bool

	Service   runs.DesignService
	Callbacks Callbacks

	OnCreateRun func(runs.RunSnapshot)

	Left  func(*ui.Context)
	Right func(*ui.Context)

	savedDoc designDoc
	opSeq    uint64
}

func New(designID, title string, revision uint64, svc runs.DesignService) *Designer {
	d := &Designer{
		DesignID:  designID,
		Title:     title,
		Revision:  revision,
		Editor:    workflow.NewEditor(),
		Instances: map[string][]blocks.Instance{},
		SaveState: SaveIdle,
		Service:   svc,
	}
	d.savedDoc = designDoc{DesignID: designID, Title: title, Workflow: d.Editor.Doc, Instances: d.Instances}
	return d
}

func (d *Designer) loadService() runs.DesignService {
	if d.Service != nil {
		return d.Service
	}
	return nil
}

func (d *Designer) loadDesign(ctx context.Context, designID string) (runs.DesignSnapshot, error) {
	if d.Callbacks.LoadDesign != nil {
		return d.Callbacks.LoadDesign(ctx, designID)
	}
	svc := d.loadService()
	if svc == nil {
		return runs.DesignSnapshot{}, fmt.Errorf("design service unavailable")
	}
	return svc.LoadDesign(ctx, designID)
}

func (d *Designer) saveDesign(ctx context.Context, input runs.DesignSaveInput) (runs.DesignSnapshot, error) {
	if d.Callbacks.SaveDesign != nil {
		return d.Callbacks.SaveDesign(ctx, input)
	}
	svc := d.loadService()
	if svc == nil {
		return runs.DesignSnapshot{}, fmt.Errorf("design service unavailable")
	}
	return svc.SaveDesign(ctx, input)
}

func (d *Designer) createRun(ctx context.Context, designID string, opID string) (runs.RunSnapshot, error) {
	if d.Callbacks.CreateRun != nil {
		return d.Callbacks.CreateRun(ctx, designID, opID)
	}
	svc := d.loadService()
	if svc == nil {
		return runs.RunSnapshot{}, fmt.Errorf("design service unavailable")
	}
	return svc.CreateRun(ctx, designID, opID)
}

// Load fetches a design from Core and applies it; preserves draft on error.
func (d *Designer) Load(ctx context.Context, designID string) {
	d.Loading = true
	d.Error = ""
	snap, err := d.loadDesign(ctx, designID)
	if err != nil {
		d.Loading = false
		d.Error = err.Error()
		d.SaveState = SaveError
		return
	}
	if designID != "" {
		d.DesignID = designID
	}
	d.applySnapshot(snap)
	d.Loading = false
	d.Empty = d.Editor.Doc.Nodes == nil || len(d.Editor.Doc.Nodes) == 0
}

func (d *Designer) applySnapshot(snap runs.DesignSnapshot) {
	d.Revision = snap.Revision
	if snap.Title != "" {
		d.Title = snap.Title
	}
	if snap.DesignID != "" {
		d.DesignID = snap.DesignID
	}
	var doc designDoc
	if snap.Data != "" {
		if err := json.Unmarshal([]byte(snap.Data), &doc); err == nil {
			d.Editor.Doc = doc.Workflow
			if doc.Instances != nil {
				d.Instances = doc.Instances
			}
			if doc.Title != "" {
				d.Title = doc.Title
			}
		}
	}
	d.Dirty = false
	d.SaveState = SaveIdle
	d.Error = ""
	d.savedDoc = designDoc{DesignID: d.DesignID, Title: d.Title, Workflow: d.Editor.Doc, Instances: d.Instances}
}

// Touch marks the design dirty.
func (d *Designer) Touch() {
	d.Dirty = true
	d.SaveState = SaveDraft
}

// Save performs an explicit save with expected revision and operation ID.
func (d *Designer) Save(ctx context.Context) {
	d.SaveState = SaveSaving
	d.Error = ""
	changes, err := json.Marshal(designDoc{
		DesignID:  d.DesignID,
		Title:     d.Title,
		Workflow:  d.Editor.Doc,
		Instances: d.Instances,
	})
	if err != nil {
		d.SaveState = SaveError
		d.Error = err.Error()
		return
	}
	atomic.AddUint64(&d.opSeq, 1)
	opID := fmt.Sprintf("design-save-%s-%d-%d", d.DesignID, time.Now().UnixNano(), d.opSeq)
	snap, err := d.saveDesign(ctx, runs.DesignSaveInput{
		DesignID:         d.DesignID,
		ExpectedRevision: d.Revision,
		Changes:          string(changes),
		OperationID:      opID,
	})
	if err != nil {
		d.SaveState = SaveError
		d.Error = err.Error()
		// preserve draft: do not overwrite
		return
	}
	d.Revision = snap.Revision
	if snap.Title != "" {
		d.Title = snap.Title
	}
	d.Dirty = false
	d.SaveState = SaveSaved
	d.savedDoc = designDoc{DesignID: d.DesignID, Title: d.Title, Workflow: d.Editor.Doc, Instances: d.Instances}
}

// CreateRun creates a Run from the current design.
func (d *Designer) CreateRun(ctx context.Context) {
	d.Error = ""
	atomic.AddUint64(&d.opSeq, 1)
	opID := fmt.Sprintf("design-create-run-%s-%d-%d", d.DesignID, time.Now().UnixNano(), d.opSeq)
	snap, err := d.createRun(ctx, d.DesignID, opID)
	if err != nil {
		d.SaveState = SaveError
		d.Error = err.Error()
		return
	}
	if d.OnCreateRun != nil {
		d.OnCreateRun(snap)
	}
}

// SelectedInstances returns block instances for the selected node.
func (d *Designer) SelectedInstances() []blocks.Instance {
	if d.SelectedNode == "" {
		return nil
	}
	return d.Instances[d.SelectedNode]
}

// View renders the designer: left/right hooks, top workflow editor, bottom inspector.
func View(c *ui.Context, d *Designer) {
	t := c.Theme()
	ui.Column(c).Fill().Background(t.Background).Children(func() {
		ui.Row(c).Grow(1).Children(func() {
			if d.Left != nil {
				ui.Column(c).Width(220).FillHeight().Padding(8).Gap(6).Background(t.Surface).Children(func() {
				})
			}
			ui.Column(c).Grow(1).FillHeight().Children(func() {
				// top workflow editor
				ui.Box(c).Grow(1).Fill().Children(func() {
					if d.Editor != nil {
						d.Editor.View(c)
					}
				})
				ui.Divider(c)
				// bottom inspector
				ui.Box(c).Height(220).Fill().Padding(8).Children(func() {
					inspector(c, d, t)
				})
			})
			if d.Right != nil {
				ui.Column(c).Width(260).FillHeight().Padding(8).Gap(6).Background(t.Surface).Children(func() {
					d.Right(c)
				})
			}
		})
		// status bar
		ui.Row(c).Height(32).Padding(0, 12).Gap(8).Children(func() {
			if ui.Button(c, "保存").Clicked() {
				d.Save(context.Background())
			}
			if ui.Button(c, "创建运行").Clicked() {
				d.CreateRun(context.Background())
			}
			if d.Loading {
				ui.Text(c, "加载中…").FontSize(12).TextColor(t.TextMuted)
			} else if d.Error != "" {
				ui.Text(c, d.Error).FontSize(12).TextColor(t.Danger)
			} else if d.Empty {
				ui.Text(c, "空设计").FontSize(12).TextColor(t.TextMuted)
			} else if d.Dirty {
				ui.Text(c, "Draft").FontSize(12).TextColor(t.TextMuted)
			} else {
				ui.Text(c, string(d.SaveState)).FontSize(12).TextColor(t.TextMuted)
			}
		})
	})
}

func inspector(c *ui.Context, d *Designer, t *ui.Theme) {
	ui.Column(c).Gap(6).Children(func() {
		ui.Text(c, "步骤检查器").Bold()
		if d.SelectedNode == "" {
			ui.Text(c, "未选中节点").FontSize(12).TextColor(t.TextMuted)
			return
		}
		ui.Text(c, "节点: "+d.SelectedNode).FontSize(12)
		insts := d.SelectedInstances()
		if len(insts) == 0 {
			ui.Text(c, "无组件预览").FontSize(12).TextColor(t.TextMuted)
			return
		}
		for i := range insts {
			inst := insts[i]
			ui.Box(c).Padding(6).Children(func() {
				blocks.View(c, inst)
			})
		}
	})
}
