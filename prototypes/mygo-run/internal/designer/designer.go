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
	InspectorHeight    float32
	opSeq              uint64
}

func New(designID, title string, revision uint64, svc runs.DesignService) *Designer {
	d := &Designer{
		DesignID:   designID,
		Title:      title,
		Revision:   revision,
		Editor:     workflow.NewEditor(),
		Instances:  map[string][]blocks.Instance{},
		Assays:     map[string]assays.Design{},
		StepAssays: map[string]string{},
		Service:    svc,
	}
	d.Editor.OnSelect = func(id string) {
		d.SelectedNode = id
		d.SelectedWells = nil
		d.ComponentRefsDraft = strings.Join(d.componentRefs(id), ", ")
		d.loadAssayDraft(id)
	}
	d.Editor.OnChange = func() {
		d.Empty = len(d.Editor.Doc.Nodes) == 0
		d.Touch()
	}
	d.SelectedNode = "n5"
	d.SelectedWells = []string{"A3", "B3", "C3", "D3", "E3", "F3", "G3", "H3"}
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
	if len(d.Editor.Doc.Nodes) == 0 && (strings.Contains(d.Title, "CCK") || strings.Contains(d.Title, "L929")) {
		d.Editor.Doc.SeedStandardCCK8()
		d.SelectedNode = "n5"
		d.SelectedWells = []string{"A3", "B3", "C3", "D3", "E3", "F3", "G3", "H3"}
		d.Empty = false
	} else {
		d.Empty = len(d.Editor.Doc.Nodes) == 0
	}
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
	width, height := c.Size()
	narrow := width > 0 && width <= 1000
	ui.Column(c).Fill().Background(t.Background).Children(func() {
		ui.Row(c).Grow(1).Children(func() {
			if d.Left != nil && !narrow {
				ui.Column(c).Width(188).FillHeight().Padding(8).Gap(6).Background(t.Surface).Children(func() { d.Left(c) })
			}
			ui.Column(c).Grow(1).FillHeight().Children(func() {
				inspectorHeight := float32(320)
				if narrow {
					inspectorHeight = 280
				}
				if d.InspectorHeight == 0 {
					d.InspectorHeight = inspectorHeight
				}
				inspectorHeight = d.InspectorHeight
				canvasHeight := height - 36 - inspectorHeight
				if canvasHeight < 180 {
					canvasHeight = 180
					inspectorHeight = height - 216
				}
				if narrow && canvasHeight > 260 {
					canvasHeight = 260
				}
				if d.InspectorCollapsed {
					canvasHeight = height - 36
					inspectorHeight = 0
				}
				ui.Box(c).Height(canvasHeight).Fill().Children(func() {
					if d.Editor != nil {
						d.Editor.View(c)
					}
				})
				if !d.InspectorCollapsed {
					ui.Divider(c)
					ui.Box(c).Height(inspectorHeight).Fill().Padding(8).Children(func() { inspector(c, d, t, narrow) })
				}
			})
			if d.Right != nil && !narrow {
				ui.Column(c).Width(220).FillHeight().Padding(8).Gap(6).Background(t.Surface).Children(func() { d.Right(c) })
			}
		})
		ui.Row(c).Height(36).Padding(0, 12).Gap(8).AlignItems(ui.Center).Background(t.Surface).Children(func() {
			if d.Dirty {
				ui.Text(c, "● 草稿未保存").FontSize(12).TextColor(t.Warning)
			} else if d.SaveState == SaveSaved {
				ui.Text(c, fmt.Sprintf("✓ 已保存 · rev %d", d.Revision)).FontSize(12).TextColor(t.Success)
			} else {
				ui.Text(c, fmt.Sprintf("就绪 · rev %d", d.Revision)).FontSize(12).TextColor(t.TextMuted)
			}
			ui.Spacer(c)
			if d.Loading {
				ui.Text(c, "加载中…").FontSize(12).TextColor(t.TextMuted)
			}
			if d.Error != "" {
				ui.Text(c, d.Error).FontSize(12).TextColor(t.Danger)
			}
			if ui.Button(c, "保存设计").Clicked() {
				d.Save(context.Background())
			}
			if ui.Button(c, "▶ 创建运行").Clicked() {
				d.CreateRun(context.Background())
			}
			if ui.Button(c, map[bool]string{true: "展开检查器", false: "收起检查器"}[d.InspectorCollapsed]).Clicked() {
				d.InspectorCollapsed = !d.InspectorCollapsed
			}
		})
	})
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

		tabBtn("步骤属性与仪器参数", 0, "")
		tabBtn("96 孔板分组映射 (Visual Plate Mapping)", 1, "80 孔已分配")
		tabBtn("科学合约与终点校验", 2, "")

		ui.Spacer(c)
		ui.Text(c, "节点 #05 · Multiskan SkyHigh").FontSize(10.5).TextColor(ui.Hex("#5A606F"))
		if ui.Button(c, "⤢ 展开检查器").Radius(4).Clicked() {
			if d.InspectorHeight < 400 {
				d.InspectorHeight = 440
			} else {
				d.InspectorHeight = 320
			}
		}
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
	ui.Column(c).Grow(1).FillHeight().Background(ui.Hex("#111317")).Border(1, ui.Hex("#1E222B")).Padding(6, 12).Gap(4).AlignItems(ui.Center).Children(func() {
		ui.Row(c).Fill().AlignItems(ui.Center).Children(func() {
			ui.Column(c).Gap(1).Children(func() {
				ui.Text(c, "96 孔板布局映射 (L929 CCK-8 Plate #1)").FontSize(11.5).Bold().TextColor(ui.Hex("#F0F2F5"))
				ui.Text(c, "8 排 × 12 列 · 80 铺板孔 · 16 边缘孔").FontSize(9.5).TextColor(ui.Hex("#7E8494"))
			})
			ui.Spacer(c)
			if ui.Button(c, "快速载入梯度模板").Radius(4).Clicked() {
				d.SelectedWells = []string{"A3", "B3", "C3", "D3", "E3", "F3", "G3", "H3"}
				d.Touch()
			}
			if ui.Button(c, "清除选中").Radius(4).Clicked() {
				d.SelectedWells = nil
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
						colIdx := col + 1
						wellColor, isEdge := wellPlateColor(rowChar, colIdx)
						selected := containsString(d.SelectedWells, wellID)
						wellBox := ui.Box(c).Size(18, 18).Draw(func(p *ui.Painter, rect ui.Rect) {
							p.Fill(rect, wellColor, 9)
							if selected {
								p.Stroke(rect, ui.Hex("#FFFFFF"), 9, 1.5)
								p.Stroke(ui.Rect{X: rect.X - 1, Y: rect.Y - 1, W: rect.W + 2, H: rect.H + 2}, ui.Hex("#4B88FF"), 10, 1.5)
							} else if isEdge {
								p.Stroke(rect, ui.Hex("#262D3A"), 9, 1)
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
			renderChip("空白 Blank (Col 1)", ui.Hex("#64748B"))
			renderChip("对照 NC (Col 2)", ui.Hex("#2DD4BF"))
			renderChip("C1 6.25 µg/mL", ui.Hex("#3B82F6"))
			renderChip("C2 12.5 µg/mL", ui.Hex("#6366F1"))
			renderChip("C3 25.0 µg/mL", ui.Hex("#8B5CF6"))
			renderChip("C4 50.0 µg/mL", ui.Hex("#A855F7"))
			renderChip("C5 100 µg/mL", ui.Hex("#C084FC"))
			renderChip("阳性 PC (Col 12)", ui.Hex("#F43F5E"))
		})
	})
}

func renderCol3GroupCalc(c *ui.Context, d *Designer, t *ui.Theme) {
	ui.Column(c).Width(290).FillHeight().Background(ui.Hex("#111317")).Border(1, ui.Hex("#1E222B")).Padding(8, 12).Gap(6).Children(func() {
		ui.Row(c).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "孔位选择与绑定").FontSize(11).Bold().TextColor(ui.Hex("#7E8494"))
			ui.Spacer(c)
			wellCount := len(d.SelectedWells)
			if wellCount == 0 {
				wellCount = 8
			}
			ui.Text(c, fmt.Sprintf("已选 %d 孔", wellCount)).FontSize(9.5).Bold().TextColor(ui.Hex("#4B88FF"))
		})

		ui.Column(c).Radius(6).Background(ui.Hex("#16191F")).Border(1, ui.Hex("#262D3A")).Padding(8, 10).Gap(4).Children(func() {
			ui.Row(c).AlignItems(ui.Center).Children(func() {
				ui.Text(c, "梯度组 C1 · 6.25 µg/mL").FontSize(11.5).Bold().TextColor(ui.Hex("#F0F2F5"))
				ui.Spacer(c)
				ui.Text(c, "A3 - H3").FontSize(10).TextColor(ui.Hex("#5A606F"))
			})
			ui.Text(c, "受试样本: ZnO-NP 浸提原液稀释 1:16 · 4 复孔重复").FontSize(10).TextColor(ui.Hex("#7E8494"))

			ui.Row(c).Gap(6).Children(func() {
				statBox := func(k, v string) {
					ui.Column(c).Grow(1).Radius(4).Background(ui.Hex("#0B0C0E")).Border(1, ui.Hex("#1E222B")).Padding(4, 6).Children(func() {
						ui.Text(c, k).FontSize(8.5).TextColor(ui.Hex("#5A606F"))
						ui.Text(c, v).FontSize(11.5).Bold().TextColor(ui.Hex("#F0F2F5"))
					})
				}
				statBox("CCK-8 试剂消耗", "80.0 µL")
				statBox("总体系体积", "880.0 µL")
			})
			ui.Row(c).Gap(6).Children(func() {
				statBox := func(k, v string) {
					ui.Column(c).Grow(1).Radius(4).Background(ui.Hex("#0B0C0E")).Border(1, ui.Hex("#1E222B")).Padding(4, 6).Children(func() {
						ui.Text(c, k).FontSize(8.5).TextColor(ui.Hex("#5A606F"))
						ui.Text(c, v).FontSize(11.5).Bold().TextColor(ui.Hex("#F0F2F5"))
					})
				}
				statBox("接种细胞总数", "40,000 cells")
				statBox("预期读数端点", "OD450 (AU)")
			})
		})

		groupBinding := "梯度 C1 (6.25 µg/mL · 4 复孔)"
		ui.Text(c, "分配至实验分组  group_binding").FontSize(10).TextColor(ui.Hex("#9DA3AF"))
		components.Field(c, "", &groupBinding, "")

		if ui.Button(c, "快速应用至 4 复孔").Radius(5).Clicked() {
			d.Touch()
		}
		if ui.Button(c, "载入梯度稀释预设 (1:2 梯度)").Radius(5).Clicked() {
			d.Touch()
		}
	})
}

func wellPlateColor(r rune, col int) (ui.Color, bool) {
	switch col {
	case 1:
		return ui.Hex("#334155"), false
	case 2:
		return ui.Hex("#134E4A"), false
	case 3, 4:
		return ui.Hex("#1E40AF"), false
	case 5, 6:
		return ui.Hex("#3730A3"), false
	case 7, 8:
		return ui.Hex("#5B21B6"), false
	case 9, 10:
		return ui.Hex("#701A75"), false
	case 11:
		return ui.Hex("#581C87"), false
	case 12:
		return ui.Hex("#881337"), false
	default:
		return ui.Hex("#14171E"), true
	}
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
