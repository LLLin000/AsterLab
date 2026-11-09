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
	d.Editor.SelectedNode, d.Editor.SelectedEdge = "", ""
	d.Editor.UndoStack, d.Editor.RedoStack = nil, nil
	d.SelectedNode, d.SelectedWells = "", nil
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
	labels := []string{"步骤属性", "96 孔映射", "检测参数"}
	ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
		components.SectionTitle(c, "检查器")
		ui.Spacer(c)
		if ui.Button(c, "−").Clicked() && d.InspectorHeight > 200 {
			d.InspectorHeight -= 24
		}
		if ui.Button(c, "+").Clicked() && d.InspectorHeight < 480 {
			d.InspectorHeight += 24
		}
	})
	components.Tabs(c, labels, &d.InspectorTab)
	ui.Scroll(c).Fill().Children(func() {
		ui.Column(c).Gap(8).Padding(8).Children(func() {
			if d.SelectedNode == "" {
				components.SectionTitle(c, "步骤检查器")
				components.Body(c, "选择画布中的节点以查看或编辑其属性。")
				return
			}
			switch d.InspectorTab {
			case 1:
				plateInspector(c, d, t)
			case 2:
				assayInspector(c, d, t)
			default:
				stepInspector(c, d)
			}
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
