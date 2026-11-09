package designer

import (
	"asterlab-mygo-prototype/internal/assays"
	"asterlab-mygo-prototype/internal/blocks"
	"asterlab-mygo-prototype/internal/components"
	"asterlab-mygo-prototype/internal/workflow"
	"asterlab-mygo-prototype/modules/runs"
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

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

type designExtension struct {
	Workflow   workflow.WorkflowDocument    `json:"workflow"`
	Instances  map[string][]blocks.Instance `json:"instances,omitempty"`
	Assays     map[string]assays.Design     `json:"assays,omitempty"`
	StepAssays map[string]string            `json:"stepAssays,omitempty"`
}

type designDoc struct {
	Title string          `json:"title,omitempty"`
	MyGo  designExtension `json:"mygo"`
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

	Editor     *workflow.Editor
	Instances  map[string][]blocks.Instance
	Assays     map[string]assays.Design
	StepAssays map[string]string

	AssayIDDraft, TimepointsDraft, PlateInstanceDraft, ReplicatesDraft                     string
	ControlGroupDraft, CalibratorGroupDraft, SamplesDraft, GenesDraft, ReferenceGenesDraft string
	SampleIDDraft, GroupIDDraft, WellTimepointDraft, WellAssignmentsDraft                  string
	SelectedWells                                                                          []string
	HoveredWell                                                                            string
	SelectedNode                                                                           string
	ComponentRefsDraft                                                                     string

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

	InspectorTab       int
	InspectorCollapsed bool
	CanvasHeight       float32
	InspectorHeight    float32
	CmdPaletteOpen     bool
	CmdQuery           string
	opSeq              uint64
}

func New(designID, title string, revision uint64, svc runs.DesignService) *Designer {
	d := &Designer{
		DesignID:     designID,
		Title:        title,
		Revision:     revision,
		Editor:       workflow.NewEditor(),
		Instances:    map[string][]blocks.Instance{},
		Assays:       map[string]assays.Design{},
		StepAssays:   map[string]string{},
		Service:      svc,
		CanvasHeight: 380,
	}
	d.Editor.OnSelect = func(id string) {
		d.SelectedNode = id
		d.SelectedWells = nil
		d.ComponentRefsDraft = strings.Join(d.componentRefs(id), ", ")
		d.loadAssayDraft(id)
	}
	d.Editor.OnChange = func() {
		d.Empty = len(d.Editor.Doc.Nodes) == 0
		if d.SelectedNode == "" && len(d.Editor.Doc.Nodes) > 0 {
			d.SelectedNode = d.Editor.Doc.Nodes[len(d.Editor.Doc.Nodes)-1].ID
			d.StepAssays[d.SelectedNode] = "cck8"
			d.loadAssayDraft(d.SelectedNode)
		}
		d.Touch()
	}
	d.Editor.OnCommandPalette = func() {
		d.CmdPaletteOpen = !d.CmdPaletteOpen
	}
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
	d.Assays = map[string]assays.Design{}
	d.StepAssays = map[string]string{}
	d.Instances = map[string][]blocks.Instance{}
	d.Revision = snap.Revision
	if snap.Title != "" {
		d.Title = snap.Title
	}
	if snap.DesignID != "" {
		d.DesignID = snap.DesignID
	}
	var doc designDoc
	d.Editor.Doc = workflow.WorkflowDocument{}
	if snap.Data != "" {
		if err := json.Unmarshal([]byte(snap.Data), &doc); err == nil {
			d.Editor.Doc = doc.MyGo.Workflow
			if doc.MyGo.Instances != nil {
				d.Instances = doc.MyGo.Instances
			}
			if doc.MyGo.Assays != nil {
				d.Assays = doc.MyGo.Assays
			}
			if doc.MyGo.StepAssays != nil {
				d.StepAssays = doc.MyGo.StepAssays
			}
			if doc.Title != "" {
				d.Title = doc.Title
			}
		}
	}
	d.Empty = len(d.Editor.Doc.Nodes) == 0
	if len(d.Editor.Doc.Nodes) > 0 {
		d.SelectedNode = d.Editor.Doc.Nodes[0].ID
		d.Editor.SelectedNode = d.SelectedNode
		d.loadAssayDraft(d.SelectedNode)
	} else {
		d.SelectedNode = ""
		d.Editor.SelectedNode = ""
	}
	d.SelectedWells = nil
	d.Editor.UndoStack, d.Editor.RedoStack = nil, nil
	d.Dirty = false
	d.SaveState = SaveIdle
	d.Error = ""
}
func (d *Designer) loadAssayDraft(nodeID string) {
	d.AssayIDDraft = d.StepAssays[nodeID]
	p := d.Assays[d.AssayIDDraft]
	d.TimepointsDraft = strings.Join(p.Timepoints, ", ")
	d.PlateInstanceDraft = p.PlateInstanceID
	d.ReplicatesDraft = fmt.Sprint(p.TechnicalReplicates)
	d.ControlGroupDraft = p.ControlGroup
	d.CalibratorGroupDraft = p.CalibratorGroup
	samples, genes := make([]string, 0, len(p.Samples)), make([]string, 0, len(p.Genes))
	for _, sample := range p.Samples {
		samples = append(samples, sample.ID+":"+sample.Group)
	}
	for _, gene := range p.Genes {
		if gene.Reference {
			genes = append(genes, gene.ID+"*")
		} else {
			genes = append(genes, gene.ID)
		}
	}
	d.SamplesDraft, d.GenesDraft = strings.Join(samples, ", "), strings.Join(genes, ", ")
	d.ReferenceGenesDraft = strings.Join(p.ReferenceGenes, ", ")
	assignments := make([]string, 0, len(p.Wells))
	for _, well := range p.Wells {
		assignments = append(assignments, fmt.Sprintf("%s=%s/%s#%d@%s", well.WellID, well.SampleID, well.GroupID, well.Replicate, well.Timepoint))
	}
	d.WellAssignmentsDraft = strings.Join(assignments, ", ")
	d.SampleIDDraft, d.GroupIDDraft, d.WellTimepointDraft = "", "", ""
}

// AssignSelectedWells commits visually selected 96-well plate positions.
func (d *Designer) AssignSelectedWells() error {
	if d.AssayIDDraft == "" || d.SelectedNode == "" {
		return fmt.Errorf("select an assay step first")
	}
	p := d.Assays[d.AssayIDDraft]
	wells := append([]string(nil), d.SelectedWells...)
	sampleID, groupID, timepoint := strings.TrimSpace(d.SampleIDDraft), strings.TrimSpace(d.GroupIDDraft), strings.TrimSpace(d.WellTimepointDraft)
	if len(wells) == 0 || sampleID == "" || groupID == "" || timepoint == "" {
		return fmt.Errorf("select wells and provide sample, group and timepoint")
	}
	assignments := append([]assays.WellAssignment(nil), p.Wells...)
	for i, wellID := range wells {
		wellID = strings.ToUpper(strings.TrimSpace(wellID))
		for _, existing := range assignments {
			if strings.EqualFold(existing.WellID, wellID) && existing.Timepoint == timepoint {
				return fmt.Errorf("well %s already assigned at %s", wellID, timepoint)
			}
		}
		assignments = append(assignments, assays.WellAssignment{WellID: wellID, SampleID: sampleID, GroupID: groupID, Replicate: i + 1, Timepoint: timepoint})
	}
	if err := assays.ValidateWellAssignments(assignments); err != nil {
		return err
	}
	p.Wells = assignments
	d.Assays[d.AssayIDDraft] = p
	d.SelectedWells = nil
	d.loadAssayDraft(d.SelectedNode)
	d.Touch()
	return nil
}

func (d *Designer) saveAssayDraft() {
	if d.AssayIDDraft == "" || d.SelectedNode == "" {
		return
	}
	d.StepAssays[d.SelectedNode] = d.AssayIDDraft
	p := d.Assays[d.AssayIDDraft]
	p.Timepoints = splitCSV(d.TimepointsDraft)
	p.PlateInstanceID = strings.TrimSpace(d.PlateInstanceDraft)
	p.TechnicalReplicates, _ = strconv.Atoi(strings.TrimSpace(d.ReplicatesDraft))
	p.ControlGroup, p.CalibratorGroup = strings.TrimSpace(d.ControlGroupDraft), strings.TrimSpace(d.CalibratorGroupDraft)
	p.ReferenceGenes = splitCSV(d.ReferenceGenesDraft)
	p.Samples, p.Genes = nil, nil
	for _, value := range splitCSV(d.SamplesDraft) {
		parts := strings.SplitN(value, ":", 2)
		sample := assays.Sample{ID: strings.TrimSpace(parts[0])}
		if len(parts) == 2 {
			sample.Group = strings.TrimSpace(parts[1])
		}
		p.Samples = append(p.Samples, sample)
	}
	for _, value := range splitCSV(d.GenesDraft) {
		value = strings.TrimSpace(value)
		gene := assays.Gene{ID: strings.TrimSuffix(value, "*"), Reference: strings.HasSuffix(value, "*")}
		if gene.ID != "" {
			p.Genes = append(p.Genes, gene)
		}
	}
	d.Assays[d.AssayIDDraft] = p
	d.Touch()
}

func splitCSV(value string) []string {
	var values []string
	for _, item := range strings.Split(value, ",") {
		if item = strings.TrimSpace(item); item != "" {
			values = append(values, item)
		}
	}
	return values
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
		Title: d.Title,
		MyGo:  designExtension{Workflow: d.Editor.Doc, Instances: d.Instances, Assays: d.Assays, StepAssays: d.StepAssays},
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
}

// CreateRun creates a Run from the saved current design.
func (d *Designer) CreateRun(ctx context.Context) {
	d.Error = ""
	if d.Dirty {
		d.Save(ctx)
		if d.Dirty || d.SaveState == SaveError {
			return
		}
	}
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

func (d *Designer) componentRefs(nodeID string) []string {
	for i := range d.Editor.Doc.Nodes {
		if d.Editor.Doc.Nodes[i].ID == nodeID {
			return d.Editor.Doc.Nodes[i].ComponentRefs
		}
	}
	return nil
}

func (d *Designer) setComponentRefs(nodeID, value string) {
	var refs []string
	for _, ref := range strings.Split(value, ",") {
		if ref = strings.TrimSpace(ref); ref != "" {
			refs = append(refs, ref)
		}
	}
	for i := range d.Editor.Doc.Nodes {
		node := &d.Editor.Doc.Nodes[i]
		if node.ID == nodeID {
			if strings.Join(node.ComponentRefs, "\x00") != strings.Join(refs, "\x00") {
				node.ComponentRefs = refs
				d.Touch()
			}
			return
		}
	}
}

// View renders the workflow and its structured inspector in one shared light/dark layout.
func View(c *ui.Context, d *Designer) {
	t := c.Theme()
	width, _ := c.Size()
	narrow := width > 0 && width <= 1000

	c.OnShortcut(ui.Cmd, ui.KeyK, func() {
		d.CmdPaletteOpen = !d.CmdPaletteOpen
	})
	c.OnShortcut(ui.Cmd, ui.KeyS, func() {
		d.Save(context.Background())
	})

	ui.Column(c).Fill().Background(t.Background).Children(func() {
		ui.Row(c).Grow(1).Children(func() {
			if d.Left != nil && !narrow {
				ui.Column(c).Width(188).FillHeight().Padding(8).Gap(6).Background(t.Surface).Children(func() { d.Left(c) })
			}
			ui.Column(c).Grow(1).FillHeight().Children(func() {
				if d.CanvasHeight <= 0 {
					d.CanvasHeight = 380
				}
				if d.InspectorCollapsed {
					ui.Box(c).Grow(1).Fill().Children(func() {
						if d.Editor != nil {
							d.Editor.View(c)
						}
					})
				} else {
					ui.SplitVertical(c, &d.CanvasHeight, func() {
						if d.Editor != nil {
							d.Editor.View(c)
						}
					}, func() {
						inspector(c, d, t, narrow)
					}).SplitReset(380).Fill()
				}
			})
			if d.Right != nil && !narrow {
				ui.Column(c).Width(220).FillHeight().Padding(8).Gap(6).Background(t.Surface).Children(func() { d.Right(c) })
			}
		})
		ui.Row(c).Height(34).Padding(0, 14).Gap(10).AlignItems(ui.Center).Background(ui.Hex("#111317")).Border(1, ui.Hex("#1E222B")).Children(func() {
			if d.Dirty {
				ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
					ui.Box(c).Size(6, 6).Radius(3).Background(ui.Hex("#F59E0B"))
					ui.Text(c, fmt.Sprintf("● 草稿未保存 (rev %d)", d.Revision)).FontSize(11.5).TextColor(ui.Hex("#F59E0B"))
				})
			} else if d.SaveState == SaveSaving {
				ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
					ui.Box(c).Size(6, 6).Radius(3).Background(ui.Hex("#60A5FA"))
					ui.Text(c, "⟳ 正在同步写入 SQLite…").FontSize(11.5).TextColor(ui.Hex("#60A5FA"))
				})
			} else if d.SaveState == SaveSaved {
				ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
					ui.Box(c).Size(6, 6).Radius(3).Background(ui.Hex("#34D399"))
					ui.Text(c, fmt.Sprintf("✓ 已保存 · rev %d · SQLite", d.Revision)).FontSize(11.5).TextColor(ui.Hex("#34D399"))
				})
			} else {
				ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
					ui.Box(c).Size(6, 6).Radius(3).Background(ui.Hex("#5A606F"))
					ui.Text(c, fmt.Sprintf("就绪 · rev %d · SQLite", d.Revision)).FontSize(11.5).TextColor(ui.Hex("#9DA3AF"))
				})
			}
			ui.Spacer(c)
			if d.Loading {
				ui.Text(c, "加载中…").FontSize(11.5).TextColor(t.TextMuted)
			}
			if d.Error != "" {
				ui.Text(c, d.Error).FontSize(11.5).TextColor(t.Danger)
			}
			if ui.Button(c, "保存设计 (Cmd+S)").Radius(5).Clicked() {
				d.Save(context.Background())
			}
			if ui.Button(c, "▶ 创建运行").Radius(5).Clicked() {
				d.CreateRun(context.Background())
			}
			if ui.Button(c, map[bool]string{true: "展开检查器", false: "收起检查器"}[d.InspectorCollapsed]).Radius(5).Clicked() {
				d.InspectorCollapsed = !d.InspectorCollapsed
			}
		})
	})
	renderCommandPalette(c, d)
}

func inspector(c *ui.Context, d *Designer, t *ui.Theme, compact bool) {
	ui.Row(c).Height(34).Padding(0, 12).AlignItems(ui.Center).Background(ui.Hex("#16191F")).Border(1, ui.Hex("#1E222B")).Children(func() {
		tabBtn := func(label string, idx int, badge string) {
			active := d.InspectorTab == idx
			bg := ui.Color{}
			color := ui.Hex("#7E8494")
			if active {
				bg = ui.Hex("#111317")
				color = ui.Hex("#F0F2F5")
			}
			b := ui.ButtonBase(c).Height(26).Padding(0, 10).Radius(4).Background(bg)
			b.Children(func() {
				ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
					ui.Text(c, label).FontSize(11).Bold().TextColor(color)
					if badge != "" {
						ui.Row(c).Padding(1, 4).Radius(3).Background(ui.Hex("#182236")).Children(func() {
							ui.Text(c, badge).FontSize(9).TextColor(ui.Hex("#4B88FF"))
						})
					}
				})
			})
			if b.Clicked() {
				d.InspectorTab = idx
			}
		}

		plateBadge := fmt.Sprintf("%d 孔已分配", len(d.Assays[d.AssayIDDraft].Wells))
		tabBtn("步骤属性与仪器参数", 0, "")
		tabBtn("96 孔板分组映射 (Visual Plate Mapping)", 1, plateBadge)
		tabBtn("科学合约与终点校验", 2, "")

		ui.Spacer(c)
		nodeLabel := "未选择节点"
		if d.SelectedNode != "" {
			nodeLabel = fmt.Sprintf("节点 %s · %s", d.SelectedNode, d.nodeTitle(d.SelectedNode))
		}
		ui.Text(c, nodeLabel).FontSize(10.5).TextColor(ui.Hex("#5A606F"))
	})

	if !compact {
		ui.Row(c).Grow(1).FillHeight().Children(func() {
			renderCol1Props(c, d, t)
			renderCol2Plate(c, d, t)
			renderCol3GroupCalc(c, d, t)
		})
	} else {
		ui.Scroll(c).Fill().Children(func() {
			switch d.InspectorTab {
			case 1:
				renderCol2Plate(c, d, t)
			case 2:
				renderCol3GroupCalc(c, d, t)
			default:
				renderCol1Props(c, d, t)
			}
		})
	}
}

func renderCol1Props(c *ui.Context, d *Designer, t *ui.Theme) {
	ui.Column(c).Width(270).FillHeight().Background(ui.Hex("#111317")).Border(1, ui.Hex("#1E222B")).Padding(8, 12).Gap(5).Children(func() {
		ui.Row(c).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "操作参数配置").FontSize(11).Bold().TextColor(ui.Hex("#7E8494"))
			ui.Spacer(c)
			ui.Text(c, "CCK-8 SOP").FontSize(10).TextColor(ui.Hex("#5A606F"))
		})

		title := d.nodeTitle(d.SelectedNode)
		if title == "" {
			title = "酶标仪吸光度读板"
		}
		ui.Text(c, "步骤名称  step_id: read_od450").FontSize(10).TextColor(ui.Hex("#9DA3AF"))
		if components.Field(c, "", &title, "为该步骤命名") {
			d.setNodeTitle(d.SelectedNode, title)
		}

		device := "Thermo Multiskan SkyHigh"
		ui.Text(c, "绑定分析仪器  device.catalog").FontSize(10).TextColor(ui.Hex("#9DA3AF"))
		components.Field(c, "", &device, "选择仪器")

		ui.Row(c).Gap(6).Children(func() {
			w1 := "450 nm"
			ui.Column(c).Grow(1).Children(func() {
				ui.Text(c, "测定波长 λ1").FontSize(9.5).TextColor(ui.Hex("#9DA3AF"))
				components.Field(c, "", &w1, "")
			})
			w2 := "650 nm"
			ui.Column(c).Grow(1).Children(func() {
				ui.Text(c, "参比波长 λref").FontSize(9.5).TextColor(ui.Hex("#9DA3AF"))
				components.Field(c, "", &w2, "")
			})
		})

		ui.Row(c).Gap(6).Children(func() {
			shake := "5s (中速)"
			ui.Column(c).Grow(1).Children(func() {
				ui.Text(c, "震板程序").FontSize(9.5).TextColor(ui.Hex("#9DA3AF"))
				components.Field(c, "", &shake, "")
			})
			maxOD := "2.500 AU"
			ui.Column(c).Grow(1).Children(func() {
				ui.Text(c, "量程上限").FontSize(9.5).TextColor(ui.Hex("#9DA3AF"))
				components.Field(c, "", &maxOD, "")
			})
		})

		blankPolicy := "扣除同板培养基空白 (Blank A1-H1 平均值)"
		ui.Text(c, "空白校正策略  blank_policy").FontSize(10).TextColor(ui.Hex("#9DA3AF"))
		components.Field(c, "", &blankPolicy, "")
	})
}

func renderCol2Plate(c *ui.Context, d *Designer, t *ui.Theme) {
	assignedCount := len(d.Assays[d.AssayIDDraft].Wells)
	ui.Column(c).Grow(1).FillHeight().Background(ui.Hex("#111317")).Border(1, ui.Hex("#1E222B")).Padding(6, 12).Gap(4).AlignItems(ui.Center).Children(func() {
		ui.Row(c).Fill().AlignItems(ui.Center).Children(func() {
			ui.Column(c).Gap(1).Children(func() {
				ui.Text(c, "96 孔板布局映射").FontSize(11.5).Bold().TextColor(ui.Hex("#F0F2F5"))
				ui.Text(c, fmt.Sprintf("8 排 × 12 列 · %d 孔已分配真实样本", assignedCount)).FontSize(9.5).TextColor(ui.Hex("#7E8494"))
			})
			ui.Spacer(c)
			if ui.Button(c, "载入 80 孔标准梯度模板").Radius(4).Clicked() {
				d.LoadStandardPlateTemplate()
			}
			if ui.Button(c, "清除已选").Radius(4).Clicked() {
				d.SelectedWells = nil
			}
			if assignedCount > 0 && ui.Button(c, "清空孔板").Radius(4).Clicked() {
				p := d.Assays[d.AssayIDDraft]
				p.Wells = nil
				d.Assays[d.AssayIDDraft] = p
				d.SelectedWells = nil
				d.loadAssayDraft(d.SelectedNode)
				d.Touch()
			}
		})

		ui.Box(c).Radius(6).Background(ui.Hex("#0E1014")).Border(1, ui.Hex("#1E222B")).Padding(4, 6).Children(func() {
			ui.Row(c).Gap(2).AlignItems(ui.Center).Children(func() {
				ui.Text(c, "").Width(16)
				for col := range 12 {
					ui.Row(c).Width(18).Center().Children(func() {
						ui.Text(c, strconv.Itoa(col+1)).FontSize(8.5).TextColor(ui.Hex("#5A606F"))
					})
				}
			})
			for r := range 8 {
				rowChar := rune('A' + r)
				ui.Row(c).Gap(2).AlignItems(ui.Center).Children(func() {
					ui.Text(c, string(rowChar)).Width(16).FontSize(8.5).TextColor(ui.Hex("#5A606F"))
					for col := range 12 {
						wellID := fmt.Sprintf("%c%d", rowChar, col+1)
						assigned := d.wellAssigned(wellID)
						group := d.wellGroup(wellID)
						selected := containsString(d.SelectedWells, wellID)

						wellColor := ui.Hex("#14171E")
						if assigned {
							wellColor = getScientificGroupColor(group)
						}

						wellBox := ui.Box(c).Size(18, 18).Draw(func(p *ui.Painter, rect ui.Rect) {
							p.Fill(rect, wellColor, 9)
							if selected {
								p.Stroke(rect, ui.Hex("#FFFFFF"), 9, 1.5)
								p.Stroke(ui.Rect{X: rect.X - 1, Y: rect.Y - 1, W: rect.W + 2, H: rect.H + 2}, ui.Hex("#4B88FF"), 10, 1.5)
							} else if assigned {
								p.Stroke(rect, ui.Hex("#2A313D"), 9, 1)
							} else {
								p.Stroke(rect, ui.Hex("#1E222B"), 9, 0.5)
							}
						}).OnClick(func() {
							d.toggleWell(wellID)
						}).Key(wellID)

						if wellBox.Hovered() {
							d.HoveredWell = wellID
						}
					}
				})
			}
		})

		ui.Row(c).Height(24).Radius(5).Background(ui.Hex("#16191F")).Border(1, ui.Hex("#1E222B")).Padding(0, 8).Gap(10).AlignItems(ui.Center).Children(func() {
			renderChip := func(label string, color ui.Color) {
				ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
					ui.Box(c).Size(6, 6).Radius(3).Background(color)
					ui.Text(c, label).FontSize(9).TextColor(ui.Hex("#9DA3AF"))
				})
			}
			groupsMap := make(map[string]int)
			for _, w := range d.Assays[d.AssayIDDraft].Wells {
				groupsMap[w.GroupID]++
			}
			if len(groupsMap) == 0 {
				ui.Text(c, "未分配孔位（点击孔板孔位并在右侧分配）").FontSize(9).TextColor(ui.Hex("#5A606F"))
			} else {
				var sortedGroups []string
				for g := range groupsMap {
					sortedGroups = append(sortedGroups, g)
				}
				sort.Strings(sortedGroups)
				for _, g := range sortedGroups {
					renderChip(fmt.Sprintf("%s (%d孔)", g, groupsMap[g]), getScientificGroupColor(g))
				}
			}
		})
	})
}

func renderCol3GroupCalc(c *ui.Context, d *Designer, t *ui.Theme) {
	wellCount := len(d.SelectedWells)

	ui.Column(c).Width(290).FillHeight().Background(ui.Hex("#111317")).Border(1, ui.Hex("#1E222B")).Padding(8, 12).Gap(6).Children(func() {
		ui.Row(c).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "孔位选择与绑定").FontSize(11).Bold().TextColor(ui.Hex("#7E8494"))
			ui.Spacer(c)
			if wellCount == 0 {
				ui.Text(c, "未选择孔位").FontSize(9.5).TextColor(ui.Hex("#5A606F"))
			} else {
				ui.Text(c, fmt.Sprintf("已选 %d 孔", wellCount)).FontSize(9.5).Bold().TextColor(ui.Hex("#4B88FF"))
			}
		})

		if wellCount == 0 {
			ui.Column(c).Radius(6).Background(ui.Hex("#16191F")).Border(1, ui.Hex("#262D3A")).Padding(12).Gap(6).AlignItems(ui.Center).Children(func() {
				ui.Text(c, "未选中孔位").FontSize(12).Bold().TextColor(ui.Hex("#7E8494"))
				ui.Text(c, "在左侧孔板点击单个或多个孔位，或点击上方载入模板").FontSize(10.5).TextColor(ui.Hex("#5A606F"))
			})
		} else {
			coordsStr := strings.Join(d.SelectedWells, ", ")
			if len(coordsStr) > 28 {
				coordsStr = fmt.Sprintf("%s 等 %d 孔", d.SelectedWells[0], wellCount)
			}
			ui.Column(c).Radius(6).Background(ui.Hex("#16191F")).Border(1, ui.Hex("#262D3A")).Padding(8, 10).Gap(4).Children(func() {
				ui.Row(c).AlignItems(ui.Center).Children(func() {
					grp := d.GroupIDDraft
					if grp == "" {
						grp = "待分配组"
					}
					ui.Text(c, grp).FontSize(11.5).Bold().TextColor(ui.Hex("#F0F2F5"))
					ui.Spacer(c)
					ui.Text(c, coordsStr).FontSize(10).TextColor(ui.Hex("#5A606F"))
				})
				ui.Text(c, "基于真实孔数的试剂与实验单位用量计算：").FontSize(9.5).TextColor(ui.Hex("#7E8494"))

				reagentVol := float64(wellCount) * 10.0
				totalVol := float64(wellCount) * 110.0
				cells := wellCount * 5000

				ui.Row(c).Gap(6).Children(func() {
					statBox := func(k, v string) {
						ui.Column(c).Grow(1).Radius(4).Background(ui.Hex("#0B0C0E")).Border(1, ui.Hex("#1E222B")).Padding(4, 6).Children(func() {
							ui.Text(c, k).FontSize(8.5).TextColor(ui.Hex("#5A606F"))
							ui.Text(c, v).FontSize(11.5).Bold().TextColor(ui.Hex("#F0F2F5"))
						})
					}
					statBox("试剂消耗量", fmt.Sprintf("%.1f µL", reagentVol))
					statBox("总培养体系", fmt.Sprintf("%.1f µL", totalVol))
				})
				ui.Row(c).Gap(6).Children(func() {
					statBox := func(k, v string) {
						ui.Column(c).Grow(1).Radius(4).Background(ui.Hex("#0B0C0E")).Border(1, ui.Hex("#1E222B")).Padding(4, 6).Children(func() {
							ui.Text(c, k).FontSize(8.5).TextColor(ui.Hex("#5A606F"))
							ui.Text(c, v).FontSize(11.5).Bold().TextColor(ui.Hex("#F0F2F5"))
						})
					}
					statBox("接种细胞总数", fmt.Sprintf("%d cells", cells))
					statBox("预期端点", "OD450")
				})
			})

			if d.GroupIDDraft == "" {
				d.GroupIDDraft = "C1"
			}
			if d.SampleIDDraft == "" {
				d.SampleIDDraft = "Sample-01"
			}
			if d.WellTimepointDraft == "" {
				d.WellTimepointDraft = "24 h"
			}

			ui.Text(c, "实验组别 (group_id)").FontSize(10).TextColor(ui.Hex("#9DA3AF"))
			components.Field(c, "", &d.GroupIDDraft, "例如 C1 / 对照组")

			ui.Text(c, "样本标识 (sample_id)").FontSize(10).TextColor(ui.Hex("#9DA3AF"))
			components.Field(c, "", &d.SampleIDDraft, "例如 Sample-01")

			if ui.PrimaryButton(c, fmt.Sprintf("应用绑定到已选 %d 孔", wellCount)).Radius(5).Clicked() {
				if err := d.AssignSelectedWells(); err != nil {
					d.Error = err.Error()
				} else {
					d.Error = ""
				}
			}
		}
	})
}

func (d *Designer) LoadStandardPlateTemplate() {
	if d.AssayIDDraft == "" {
		d.AssayIDDraft = "cck8"
	}
	if d.SelectedNode != "" {
		d.StepAssays[d.SelectedNode] = d.AssayIDDraft
	}
	p := d.Assays[d.AssayIDDraft]
	var assignments []assays.WellAssignment
	for r := range 8 {
		rowChar := rune('A' + r)
		assignments = append(assignments, assays.WellAssignment{WellID: fmt.Sprintf("%c1", rowChar), SampleID: "Blank", GroupID: "Blank", Replicate: r + 1, Timepoint: "24 h"})
		assignments = append(assignments, assays.WellAssignment{WellID: fmt.Sprintf("%c2", rowChar), SampleID: "NC", GroupID: "NC", Replicate: r + 1, Timepoint: "24 h"})
		for col := 3; col <= 4; col++ {
			assignments = append(assignments, assays.WellAssignment{WellID: fmt.Sprintf("%c%d", rowChar, col), SampleID: "Sample-C1", GroupID: "C1", Replicate: (col-3)*8 + r + 1, Timepoint: "24 h"})
		}
		for col := 5; col <= 6; col++ {
			assignments = append(assignments, assays.WellAssignment{WellID: fmt.Sprintf("%c%d", rowChar, col), SampleID: "Sample-C2", GroupID: "C2", Replicate: (col-5)*8 + r + 1, Timepoint: "24 h"})
		}
		for col := 7; col <= 8; col++ {
			assignments = append(assignments, assays.WellAssignment{WellID: fmt.Sprintf("%c%d", rowChar, col), SampleID: "Sample-C3", GroupID: "C3", Replicate: (col-7)*8 + r + 1, Timepoint: "24 h"})
		}
		for col := 9; col <= 10; col++ {
			assignments = append(assignments, assays.WellAssignment{WellID: fmt.Sprintf("%c%d", rowChar, col), SampleID: "Sample-C4", GroupID: "C4", Replicate: (col-9)*8 + r + 1, Timepoint: "24 h"})
		}
		assignments = append(assignments, assays.WellAssignment{WellID: fmt.Sprintf("%c11", rowChar), SampleID: "Sample-C5", GroupID: "C5", Replicate: r + 1, Timepoint: "24 h"})
		assignments = append(assignments, assays.WellAssignment{WellID: fmt.Sprintf("%c12", rowChar), SampleID: "PC", GroupID: "PC", Replicate: r + 1, Timepoint: "24 h"})
	}
	p.Wells = assignments
	d.Assays[d.AssayIDDraft] = p
	d.SelectedWells = []string{"A3", "B3", "C3", "D3", "E3", "F3", "G3", "H3"}
	d.loadAssayDraft(d.SelectedNode)
	d.Touch()
}

func getScientificGroupColor(group string) ui.Color {
	g := strings.ToLower(strings.TrimSpace(group))
	switch {
	case strings.Contains(g, "blank") || strings.Contains(g, "空白"):
		return ui.Hex("#334155")
	case strings.Contains(g, "nc") || strings.Contains(g, "对照") || strings.Contains(g, "control"):
		return ui.Hex("#134E4A")
	case strings.Contains(g, "pc") || strings.Contains(g, "阳性") || strings.Contains(g, "positive"):
		return ui.Hex("#881337")
	case strings.EqualFold(g, "c1"):
		return ui.Hex("#1E40AF")
	case strings.EqualFold(g, "c2"):
		return ui.Hex("#3730A3")
	case strings.EqualFold(g, "c3"):
		return ui.Hex("#5B21B6")
	case strings.EqualFold(g, "c4"):
		return ui.Hex("#701A75")
	case strings.EqualFold(g, "c5"):
		return ui.Hex("#581C87")
	default:
		return wellGroupTint(group, true)
	}
}

func renderCommandPalette(c *ui.Context, d *Designer) {
	if !d.CmdPaletteOpen {
		return
	}
	ui.Modal(c, &d.CmdPaletteOpen, func() {
		ui.Column(c).Width(520).Radius(10).Background(ui.Hex("#16191F")).Border(1, ui.Hex("#2A313D")).Padding(14).Gap(10).Children(func() {
			ui.Row(c).AlignItems(ui.Center).Gap(8).Children(func() {
				ui.Row(c).Padding(2, 6).Radius(4).Background(ui.Hex("#182236")).Children(func() {
					ui.Text(c, "⌘K").FontSize(11).Bold().TextColor(ui.Hex("#4B88FF"))
				})
				ui.TextInput(c, &d.CmdQuery).AutoFocus().Placeholder("搜索操作、步骤或模板… (Esc 关闭)").Grow(1)
			})
			ui.Divider(c)
			ui.Scroll(c).MaxHeight(260).Children(func() {
				cmdItem := func(title, desc string, act func()) {
					if d.CmdQuery != "" && !strings.Contains(strings.ToLower(title), strings.ToLower(d.CmdQuery)) && !strings.Contains(strings.ToLower(desc), strings.ToLower(d.CmdQuery)) {
						return
					}
					row := ui.Row(c.Key(title)).Padding(8, 10).Radius(6).AlignItems(ui.Center).Children(func() {
						ui.Column(c).Grow(1).Gap(2).Children(func() {
							ui.Text(c, title).FontSize(12).Bold().TextColor(ui.Hex("#F0F2F5"))
							ui.Text(c, desc).FontSize(10).TextColor(ui.Hex("#7E8494"))
						})
					})
					if row.Clicked() {
						act()
						d.CmdPaletteOpen = false
						d.CmdQuery = ""
					}
				}

				cmdItem("载入标准 CCK-8 流程模板", "初始化 5 节点科学实验设计流程（细胞悬液准备到吸光度读板）", func() {
					d.Editor.Doc.SeedStandardCCK8()
					d.SelectedNode = "n5"
					d.Editor.SelectNode("n5")
					d.StepAssays["n5"] = "cck8"
					d.loadAssayDraft("n5")
					d.Touch()
				})
				cmdItem("载入 80 孔标准梯度模板", "为当前设计分配 Blank / NC / C1~C5 / PC 80 孔布局", func() {
					d.LoadStandardPlateTemplate()
				})
				cmdItem("添加材料节点 (细胞悬液)", "在画布中追加一个细胞/原材料步骤节点", func() {
					d.Editor.AddNode(workflow.KindFlask, "细胞悬液", workflow.Position{X: 60 + float32(len(d.Editor.Doc.Nodes)*220), Y: 70})
				})
				cmdItem("添加操作步骤 (加样 / 孵育)", "在画布中追加一个加样或反应处理操作步骤", func() {
					d.Editor.AddNode(workflow.KindPipette, "加样操作", workflow.Position{X: 60 + float32(len(d.Editor.Doc.Nodes)*220), Y: 70})
				})
				cmdItem("添加分析仪器 (酶标仪读板)", "在画布中追加一个读板或检测采集节点", func() {
					d.Editor.AddNode(workflow.KindPlate, "酶标仪读板", workflow.Position{X: 60 + float32(len(d.Editor.Doc.Nodes)*220), Y: 70})
				})
				cmdItem("水平对齐所有节点", "将所有工作流节点按 220px 间距排列在 Y=70 轴线", func() {
					for i := range d.Editor.Doc.Nodes {
						d.Editor.Doc.Nodes[i].Pos = workflow.Position{X: 60 + float32(i*220), Y: 70}
					}
					d.Editor.PanX, d.Editor.PanY, d.Editor.Zoom = 0, 0, 1
				})
				cmdItem("保存当前设计 (Cmd+S)", "执行确定性保存，将工作流结构与孔位映射写入 SQLite", func() {
					d.Save(context.Background())
				})
				cmdItem("创建实验运行 (Run)", "冻结当前实验设计版本并创建可追溯执行批次", func() {
					d.CreateRun(context.Background())
				})
			})
		})
	})
}

func stepInspector(c *ui.Context, d *Designer) {
	components.SectionTitle(c, "步骤属性")
	title := d.nodeTitle(d.SelectedNode)
	if components.Field(c, "步骤名称", &title, "为该步骤命名") {
		d.setNodeTitle(d.SelectedNode, title)
	}
	if components.Field(c, "关联组件", &d.ComponentRefsDraft, "组件定义 ID") {
		d.setComponentRefs(d.SelectedNode, d.ComponentRefsDraft)
	}
	if components.Field(c, "检测 ID", &d.AssayIDDraft, "例如 cck8") {
		d.saveAssayDraft()
	}
	if ui.Button(c, "添加备注组件").Clicked() {
		d.addNoteComponent(d.SelectedNode)
	}
	for i := range d.SelectedInstances() {
		d.editInstance(c, d.SelectedNode, i, c.Theme())
	}
}

func plateInspector(c *ui.Context, d *Designer, t *ui.Theme) {
	components.SectionTitle(c, "96 孔板映射")
	d.HoveredWell = ""
	components.Meta(c, "选择孔位，再填写样本与组别并应用。")
	ui.Row(c).Gap(3).Children(func() {
		ui.Text(c, "").Width(20)
		for col := range 12 {
			ui.Text(c, strconv.Itoa(col+1)).Width(22).FontSize(10).TextColor(t.TextMuted)
		}
	})
	for row := range 8 {
		ui.Row(c).Gap(3).AlignItems(ui.Center).Children(func() {
			ui.Text(c, string(rune('A'+row))).Width(20).FontSize(10).TextColor(t.TextMuted)
			for col := range 12 {
				well := fmt.Sprintf("%c%d", 'A'+row, col+1)
				selected, assigned := containsString(d.SelectedWells, well), d.wellAssigned(well)
				group := d.wellGroup(well)
				wellElement := ui.Box(c).Size(22, 22).Draw(func(p *ui.Painter, r ui.Rect) {
					fill := t.Surface
					if assigned {
						fill = wellGroupTint(group, t.Dark)
					}
					p.Fill(ui.Rect{X: r.X + 2, Y: r.Y + 2, W: 18, H: 18}, fill, 9)
					border := t.Border
					if selected {
						border = t.Accent
					}
					p.Stroke(ui.Rect{X: r.X + 2, Y: r.Y + 2, W: 18, H: 18}, border, 9, 2)
				}).OnClick(func() { d.toggleWell(well) }).Key(well)
				if wellElement.Hovered() {
					d.HoveredWell = well
				}
			}
		})
	}
	components.Meta(c, fmt.Sprintf("已选 %d 孔", len(d.SelectedWells)))
	if d.HoveredWell != "" {
		message := d.HoveredWell + " · 未分配"
		for _, assignment := range d.Assays[d.AssayIDDraft].Wells {
			if strings.EqualFold(assignment.WellID, d.HoveredWell) {
				message = fmt.Sprintf("%s · 样本 %s · 组别 %s · 重复 %d", d.HoveredWell, assignment.SampleID, assignment.GroupID, assignment.Replicate)
				break
			}
		}
		components.Meta(c, message)
	}
	components.Field(c, "样本 ID", &d.SampleIDDraft, "例如 Control-01")
	components.Field(c, "组别", &d.GroupIDDraft, "例如 对照组")
	components.Field(c, "时间点", &d.WellTimepointDraft, "例如 24 h")
	if ui.Button(c, "应用到已选孔").Clicked() {
		if err := d.AssignSelectedWells(); err != nil {
			d.Error = err.Error()
		} else {
			d.Error = ""
		}
	}
	if d.WellAssignmentsDraft != "" {
		components.Meta(c, "已保存映射: "+d.WellAssignmentsDraft)
	}
}

func assayInspector(c *ui.Context, d *Designer, t *ui.Theme) {
	components.SectionTitle(c, "检测合约与参数")
	if d.AssayIDDraft == "" {
		components.Body(c, "先在步骤属性中关联一个检测方案。")
		return
	}
	fields := []*string{&d.TimepointsDraft, &d.PlateInstanceDraft, &d.ReplicatesDraft, &d.ControlGroupDraft, &d.CalibratorGroupDraft}
	labels := []string{"时间点（h）", "孔板组件 ID", "技术重复数（个）", "对照组", "校准组"}
	for i := range fields {
		if components.Field(c, labels[i], fields[i], "") {
			d.saveAssayDraft()
		}
	}
	components.Meta(c, "参数按当前设计修订保存；保存失败时草稿会保留。")
}

func (d *Designer) wellAssigned(id string) bool {
	for _, well := range d.Assays[d.AssayIDDraft].Wells {
		if strings.EqualFold(well.WellID, id) {
			return true
		}
	}
	return false
}
func (d *Designer) wellGroup(id string) string {
	for _, well := range d.Assays[d.AssayIDDraft].Wells {
		if strings.EqualFold(well.WellID, id) {
			return well.GroupID
		}
	}
	return ""
}

func wellGroupTint(group string, dark bool) ui.Color {
	palette := [...]string{"#DCEBFA", "#E5E1FA", "#DDF1E8", "#F8E8D4"}
	if dark {
		palette = [...]string{"#23354D", "#352E4F", "#233E35", "#493725"}
	}
	var index uint32
	for _, r := range group {
		index += uint32(r)
	}
	return ui.Hex(palette[index%uint32(len(palette))])
}

func containsString(values []string, value string) bool {
	for _, item := range values {
		if item == value {
			return true
		}
	}
	return false
}

func (d *Designer) toggleWell(well string) {
	for i, selected := range d.SelectedWells {
		if selected == well {
			d.SelectedWells = append(d.SelectedWells[:i], d.SelectedWells[i+1:]...)
			return
		}
	}
	d.SelectedWells = append(d.SelectedWells, well)
}

func (d *Designer) nodeTitle(id string) string {
	for i := range d.Editor.Doc.Nodes {
		if d.Editor.Doc.Nodes[i].ID == id {
			return d.Editor.Doc.Nodes[i].Title
		}
	}
	return ""
}

func (d *Designer) setNodeTitle(id, title string) {
	for i := range d.Editor.Doc.Nodes {
		if d.Editor.Doc.Nodes[i].ID == id && d.Editor.Doc.Nodes[i].Title != title {
			d.Editor.Doc.Nodes[i].Title = title
			d.Touch()
			return
		}
	}
}

func (d *Designer) addNoteComponent(nodeID string) {
	inst, err := blocks.NewInstance(fmt.Sprintf("note-%d", len(d.Instances[nodeID])+1), blocks.Definition{ID: "designer-note", Version: 1, Type: blocks.TypeNote, Props: blocks.NoteProps{Title: "Note"}})
	if err != nil {
		d.Error = err.Error()
		return
	}
	d.Instances[nodeID] = append(d.Instances[nodeID], inst)
	refs := d.componentRefs(nodeID)
	if !containsString(refs, inst.DefinitionID) {
		refs = append(refs, inst.DefinitionID)
	}
	for i := range d.Editor.Doc.Nodes {
		if d.Editor.Doc.Nodes[i].ID == nodeID {
			d.Editor.Doc.Nodes[i].ComponentRefs = refs
			break
		}
	}
	d.ComponentRefsDraft = strings.Join(refs, ", ")
	d.Touch()
}

func (d *Designer) editInstance(c *ui.Context, nodeID string, index int, t *ui.Theme) {
	instances := d.Instances[nodeID]
	if index >= len(instances) {
		return
	}
	inst := instances[index]
	if inst.Type != blocks.TypeNote {
		blocks.View(c, inst)
		return
	}
	var props blocks.NoteProps
	if inst.DecodeProps(&props) != nil {
		return
	}
	ui.Text(c, "备注组件 · "+inst.ID).FontSize(11).TextColor(t.TextMuted)
	changed := ui.TextInput(c, &props.Title).Placeholder("备注标题").Changed()
	if ui.TextInput(c, &props.Text).Placeholder("备注内容").Changed() {
		changed = true
	}
	if changed {
		raw, err := json.Marshal(props)
		if err == nil {
			inst.RawProps = raw
			instances[index] = inst
			d.Instances[nodeID] = instances
			d.Touch()
		}
	}
}
