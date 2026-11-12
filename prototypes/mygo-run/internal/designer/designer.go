package designer

import (
	"asterlab-mygo-prototype/internal/assays"
	"asterlab-mygo-prototype/internal/blocks"
	"asterlab-mygo-prototype/internal/blockui"
	plateui "asterlab-mygo-prototype/internal/uikit/plate"
	"asterlab-mygo-prototype/internal/workflow"
	"asterlab-mygo-prototype/modules/runs"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/onevra/source/components"
	"github.com/oneo-me/onevra/source/icons/lucide"
	"github.com/oneo-me/onevra/source/shells/sidebar"
)

// Dock is where the floating inspector rests: against the right edge or along
// the bottom of the canvas.
type Dock uint8

const (
	DockRight Dock = iota
	DockBottom
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
	Workflow     workflow.WorkflowDocument    `json:"workflow"`
	Instances    map[string][]blocks.Instance `json:"instances,omitempty"`
	Assays       map[string]assays.Design     `json:"assays,omitempty"`
	StepAssays   map[string]string            `json:"stepAssays,omitempty"`
	StepTasks    []StepTask                   `json:"stepTasks"`
	PreflightAck *PreflightAck                `json:"preflightAck,omitempty"`
}

// PreflightAck is the researcher's recorded decision to create a Run despite
// the warnings shown for this design revision. Core matches it against the
// warnings it recomputes while freezing the same revision.
type PreflightAck struct {
	Warnings       []string `json:"warnings"`
	ActorID        string   `json:"actorId,omitempty"`
	AcknowledgedAt string   `json:"acknowledgedAt,omitempty"`
}

// StepTask is the design's explicit execution template: this step owns this
// assay at this timepoint. Run tasks are bound from these entries, so one assay
// can run at D1, D3 and D7 under three different steps. A task with no entry
// stays unresolved instead of being inferred from titles or assay uniqueness.
type StepTask struct {
	StepID    string `json:"stepId"`
	AssayID   string `json:"assayId"`
	Timepoint string `json:"timepoint"`
	// Path narrows the entry to one protocol step path when the same assay and
	// timepoint occur under several paths. Empty claims the pair only while it
	// occurs under a single path; otherwise the tasks stay unresolved.
	Path string `json:"path,omitempty"`
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
	// StepTimepoints is the design's explicit step→timepoints authoring; Save
	// materializes it as StepTasks for the frozen run.
	StepTimepoints map[string][]string
	// StepTaskPaths keeps an existing entry's protocol path, keyed by step and
	// timepoint, so re-saving never drops path-qualified bindings.
	StepTaskPaths map[string]string

	AssayIDDraft, AssayIDInputDraft, TimepointsDraft, PlateInstanceDraft, ReplicatesDraft  string
	StepTimepointsDraft                                                                    string
	ControlGroupDraft, CalibratorGroupDraft, SamplesDraft, GenesDraft, ReferenceGenesDraft string
	SampleIDDraft, GroupIDDraft, WellTimepointDraft, WellReplicatesDraft                   string
	SelectedWells                                                                          []string
	SelectedNode                                                                           string
	ComponentRefsDraft                                                                     string
	ParamKeyDraft, ParamValueDraft                                                         string

	Dirty     bool
	SaveState SaveState
	Error     string
	Loading   bool
	Empty     bool

	Service   runs.DesignService
	Callbacks Callbacks

	OnCreateRun func(runs.RunSnapshot)

	InspectorOpen         bool
	InspectorUserToggled bool
	InspectorDock         Dock
	CreateRunWarnings     []string
	ConfirmCreateRun      bool
	PreflightAck      *PreflightAck
	panelDrag         ui.Point
	panelDragging     bool
	ConfirmClearWells bool
	CmdPaletteOpen    bool
	CmdQuery          string
	opSeq             uint64

	draftUndo      [][]byte
	draftRedo      [][]byte
	lastDraft      []byte
	lastDoc        []byte
	historySuspend bool
}

func New(designID, title string, revision uint64, svc runs.DesignService) *Designer {
	d := &Designer{
		DesignID:       designID,
		Title:          title,
		Revision:       revision,
		Editor:         workflow.NewEditor(),
		Instances:      map[string][]blocks.Instance{},
		Assays:         map[string]assays.Design{},
		StepAssays:     map[string]string{},
		StepTimepoints: map[string][]string{},
		StepTaskPaths:  map[string]string{},
		Service:        svc,
		InspectorOpen:  true,
		Empty:          true,
	}
	d.Editor.OnSelect = func(id string) {
		d.historySuspend = true
		d.SelectedNode = id
		d.SelectedWells = nil
		d.ComponentRefsDraft = strings.Join(d.componentRefs(id), ", ")
		d.ParamKeyDraft, d.ParamValueDraft = "", ""
		d.StepTimepointsDraft = strings.Join(d.StepTimepoints[id], ", ")
		d.loadAssayDraft(id)
	}
	d.Editor.BeforeDelete = func(id string) bool {
		if len(d.Instances[id]) > 0 || d.StepAssays[id] != "" {
			d.Error = "此步骤仍关联组件或检测方案；先移除关联再删除。"
			return false
		}
		d.Error = ""
		return true
	}
	d.Editor.OnChange = func() {
		d.Empty = len(d.Editor.Doc.Nodes) == 0
		if d.SelectedNode != "" && d.nodeTitle(d.SelectedNode) == "" {
			d.SelectedNode = ""
		}
		if d.SelectedNode == "" && len(d.Editor.Doc.Nodes) > 0 {
			d.SelectedNode = d.Editor.Doc.Nodes[len(d.Editor.Doc.Nodes)-1].ID
			d.Editor.SelectedNode = d.SelectedNode
			d.loadAssayDraft(d.SelectedNode)
		} else if d.SelectedNode == "" {
			d.loadAssayDraft("")
		}
		d.Touch()
	}
	d.Editor.OnCommandPalette = func() {
		d.CmdPaletteOpen = !d.CmdPaletteOpen
	}
	return d
}

// FrozenBlocks is the MyGo extension of a frozen design, decoded for surfaces
// that only read it: the Run and Review modules.
type FrozenBlocks struct {
	Workflow   workflow.WorkflowDocument
	Instances  map[string][]blocks.Instance
	StepAssays map[string]string
	StepTasks  []StepTask
	Assays     map[string]assays.Design
}

// DecodeBlocks reads the MyGo extension from a design snapshot's Data. A
// snapshot without the extension decodes to empty maps, never an error.
func DecodeBlocks(data string) (FrozenBlocks, error) {
	out := FrozenBlocks{
		Instances:  map[string][]blocks.Instance{},
		StepAssays: map[string]string{},
		Assays:     map[string]assays.Design{},
	}
	if data == "" {
		return out, nil
	}
	var doc designDoc
	if err := json.Unmarshal([]byte(data), &doc); err != nil {
		return out, fmt.Errorf("无法读取设计数据: %w", err)
	}
	out.Workflow = doc.MyGo.Workflow
	if doc.MyGo.Instances != nil {
		out.Instances = doc.MyGo.Instances
	}
	if doc.MyGo.StepAssays != nil {
		out.StepAssays = doc.MyGo.StepAssays
	}
	if doc.MyGo.Assays != nil {
		out.Assays = doc.MyGo.Assays
	}
	out.StepTasks = doc.MyGo.StepTasks
	return out, nil
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

// Load fetches a design from Core and applies it; preserves the draft on error.
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
	if err := d.applySnapshot(snap); err != nil {
		d.Loading = false
		d.Error = err.Error()
		d.SaveState = SaveError
		return
	}
	if designID != "" {
		d.DesignID = designID
	}
	d.Loading = false
	d.Empty = len(d.Editor.Doc.Nodes) == 0
}

func (d *Designer) applySnapshot(snap runs.DesignSnapshot) error {
	var doc designDoc
	if snap.Data != "" {
		if err := json.Unmarshal([]byte(snap.Data), &doc); err != nil {
			return fmt.Errorf("无法读取设计数据: %w", err)
		}
		for assayID, params := range doc.MyGo.Assays {
			if err := assays.ValidateWellAssignments(params.Wells); err != nil {
				return fmt.Errorf("检测 %s 的孔位映射无效: %w", assayID, err)
			}
		}
	}
	d.Assays = map[string]assays.Design{}
	d.StepAssays = map[string]string{}
	d.StepTimepoints = map[string][]string{}
	d.StepTaskPaths = map[string]string{}
	d.Instances = map[string][]blocks.Instance{}
	d.Revision = snap.Revision
	if snap.Title != "" {
		d.Title = snap.Title
	}
	if snap.DesignID != "" {
		d.DesignID = snap.DesignID
	}
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
	for _, task := range doc.MyGo.StepTasks {
		if task.StepID == "" || task.Timepoint == "" {
			continue
		}
		d.StepTimepoints[task.StepID] = appendTimepointSlice(d.StepTimepoints[task.StepID], task.Timepoint)
		if task.Path != "" {
			d.StepTaskPaths[stepTaskKey(task.StepID, task.Timepoint)] = task.Path
		}
	}
	d.StepTimepointsDraft = strings.Join(d.StepTimepoints[d.SelectedNode], ", ")
	if doc.Title != "" {
		d.Title = doc.Title
	}
	d.Empty = len(d.Editor.Doc.Nodes) == 0
	if len(d.Editor.Doc.Nodes) > 0 {
		d.SelectedNode = d.Editor.Doc.Nodes[0].ID
		d.Editor.SelectedNode = d.SelectedNode
		d.loadAssayDraft(d.SelectedNode)
	} else {
		d.SelectedNode = ""
		d.Editor.SelectedNode = ""
		d.loadAssayDraft("")
	}
	d.StepTimepointsDraft = strings.Join(d.StepTimepoints[d.SelectedNode], ", ")
	d.SelectedWells = nil
	d.Editor.UndoStack, d.Editor.RedoStack = nil, nil
	d.Dirty = false
	d.SaveState = SaveIdle
	d.Error = ""
	d.resetDraftHistory()
	return nil
}
func (d *Designer) loadAssayDraft(nodeID string) {
	d.AssayIDDraft = d.StepAssays[nodeID]
	d.AssayIDInputDraft = d.AssayIDDraft
	p := d.Assays[d.AssayIDDraft]
	d.TimepointsDraft = strings.Join(p.Timepoints, ", ")
	d.PlateInstanceDraft = p.PlateInstanceID
	d.ReplicatesDraft = ""
	if p.TechnicalReplicates > 0 {
		d.ReplicatesDraft = fmt.Sprint(p.TechnicalReplicates)
	}
	d.ControlGroupDraft, d.CalibratorGroupDraft = p.ControlGroup, p.CalibratorGroup
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
	d.SampleIDDraft, d.GroupIDDraft, d.WellTimepointDraft, d.WellReplicatesDraft = "", "", "", ""
}

// AssignSelectedWells explicitly adds or replaces planned well/timepoint mappings.
func (d *Designer) AssignSelectedWells() error {
	if d.AssayIDDraft == "" || d.SelectedNode == "" {
		return fmt.Errorf("先为当前步骤关联检测方案")
	}
	p := d.Assays[d.AssayIDDraft]
	wells := append([]string(nil), d.SelectedWells...)
	sort.Strings(wells)
	sampleID := strings.TrimSpace(d.SampleIDDraft)
	groupID := strings.TrimSpace(d.GroupIDDraft)
	timepoint := strings.TrimSpace(d.WellTimepointDraft)
	replicateValues := splitCSV(d.WellReplicatesDraft)
	if len(wells) == 0 || sampleID == "" || groupID == "" || timepoint == "" {
		return fmt.Errorf("请选择孔位，并填写样本、组别和时间点")
	}
	if len(replicateValues) != len(wells) {
		return fmt.Errorf("需要按孔位顺序为 %d 个孔填写 %d 个重复编号", len(wells), len(wells))
	}

	assignments := append([]assays.WellAssignment(nil), p.Wells...)
	for i, wellID := range wells {
		wellID = strings.ToUpper(strings.TrimSpace(wellID))
		replicate, err := strconv.Atoi(replicateValues[i])
		if err != nil || replicate < 1 {
			return fmt.Errorf("孔位 %s 的重复编号必须为正整数", wellID)
		}
		replacement := assays.WellAssignment{
			WellID: wellID, SampleID: sampleID, GroupID: groupID,
			Replicate: replicate, Timepoint: timepoint,
		}
		replaced := false
		for j := range assignments {
			if strings.EqualFold(assignments[j].WellID, wellID) && assignments[j].Timepoint == timepoint {
				assignments[j] = replacement
				replaced = true
				break
			}
		}
		if !replaced {
			assignments = append(assignments, replacement)
		}
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

func (d *Designer) saveAssayDraft() error {
	if d.AssayIDDraft == "" || d.SelectedNode == "" {
		return nil
	}
	replicates := 0
	replicateText := strings.TrimSpace(d.ReplicatesDraft)
	if replicateText != "" {
		value, err := strconv.Atoi(replicateText)
		if err != nil || value < 1 {
			return fmt.Errorf("技术重复数必须为正整数；未知时请留空")
		}
		replicates = value
	}
	d.StepAssays[d.SelectedNode] = d.AssayIDDraft
	p := d.Assays[d.AssayIDDraft]
	p.Timepoints = splitCSV(d.TimepointsDraft)
	p.PlateInstanceID = strings.TrimSpace(d.PlateInstanceDraft)
	p.TechnicalReplicates = replicates
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
	return nil
}

func (d *Designer) setAssayID(nodeID, assayID string) error {
	if nodeID == "" {
		return fmt.Errorf("先选择一个流程步骤")
	}
	assayID = strings.TrimSpace(assayID)
	currentID := d.StepAssays[nodeID]
	if assayID == currentID {
		d.AssayIDInputDraft = assayID
		return nil
	}
	if currentID != "" {
		if len(d.Assays[currentID].Wells) > 0 {
			return fmt.Errorf("检测 %s 仍有孔位映射；请先清空计划后再更改关联", currentID)
		}
		if nodeID == d.SelectedNode && d.AssayIDDraft == currentID {
			if err := d.saveAssayDraft(); err != nil {
				return err
			}
		}
	}
	if assayID == "" {
		delete(d.StepAssays, nodeID)
	} else {
		d.StepAssays[nodeID] = assayID
		if _, ok := d.Assays[assayID]; !ok {
			d.Assays[assayID] = assays.Design{}
		}
	}
	if nodeID == d.SelectedNode {
		d.loadAssayDraft(nodeID)
	}
	d.Touch()
	return nil
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

// stepTasks materializes the explicit step→(assay, timepoint) authoring. Steps
// without recorded timepoints contribute nothing: their tasks stay unresolved
// rather than being guessed from an assay that several steps may share.
func (d *Designer) stepTasks() []StepTask {
	out := make([]StepTask, 0, len(d.StepAssays))
	for stepID, assayID := range d.StepAssays {
		if assayID == "" {
			continue
		}
		for _, timepoint := range d.StepTimepoints[stepID] {
			if timepoint == "" {
				continue
			}
			out = append(out, StepTask{
				StepID: stepID, AssayID: assayID, Timepoint: timepoint,
				Path: d.StepTaskPaths[stepTaskKey(stepID, timepoint)],
			})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].StepID != out[j].StepID {
			return out[i].StepID < out[j].StepID
		}
		return out[i].Timepoint < out[j].Timepoint
	})
	return out
}

// stepTaskKey identifies one entry of the explicit step task table.
func stepTaskKey(stepID, timepoint string) string {
	return stepID + "\x00" + timepoint
}

func appendTimepointSlice(existing []string, next string) []string {
	for _, item := range existing {
		if item == next {
			return existing
		}
	}
	return append(existing, next)
}

// Save performs an explicit save with expected revision and operation ID.
func (d *Designer) Save(ctx context.Context) {
	if err := d.saveAssayDraft(); err != nil {
		d.SaveState = SaveError
		d.Error = err.Error()
		return
	}
	d.SaveState = SaveSaving
	d.Error = ""
	changes, err := json.Marshal(designDoc{
		Title: d.Title,
		MyGo: designExtension{
			Workflow: d.Editor.Doc, Instances: d.Instances, Assays: d.Assays,
			StepAssays: d.StepAssays, StepTasks: d.stepTasks(), PreflightAck: d.PreflightAck,
		},
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
// RequestCreateRun checks the design before freezing it. Deterministic errors
// stop the request; unresolved task links are shown and the researcher decides,
// so an unusual design is never blocked by an incomplete rule set.
func (d *Designer) RequestCreateRun(ctx context.Context) {
	d.Error = ""
	if len(d.Editor.Doc.Nodes) == 0 {
		d.Error = "请先添加实验步骤，再创建运行。"
		return
	}
	if issues := d.PreflightErrors(); len(issues) > 0 {
		d.Error = strings.Join(issues, "；")
		return
	}
	if issues := d.PreflightWarnings(); len(issues) > 0 {
		d.CreateRunWarnings = issues
		d.ConfirmCreateRun = true
		return
	}
	d.CreateRun(ctx)
}

// PreflightErrors lists conditions that make the design itself invalid.
func (d *Designer) PreflightErrors() []string {
	var out []string
	for assayID, params := range d.Assays {
		if err := assays.ValidateWellAssignments(params.Wells); err != nil {
			out = append(out, fmt.Sprintf("检测 %s 的孔位映射无效：%v", assayID, err))
		}
	}
	sort.Strings(out)
	return out
}

// PreflightWarnings lists designs that would freeze with executable tasks
// whose modules cannot be linked. They never block creation.
func (d *Designer) PreflightWarnings() []string {
	var out []string
	claims := map[string][]string{}
	for stepID, assayID := range d.StepAssays {
		if assayID == "" {
			continue
		}
		points := d.StepTimepoints[stepID]
		if len(points) == 0 {
			out = append(out, fmt.Sprintf("步骤「%s」关联了检测 %s，但没有填写本步骤时间点；该检测的任务不会自动关联模块。",
				d.nodeTitle(stepID), assayID))
			continue
		}
		for _, timepoint := range points {
			key := assayID + "\x00" + timepoint
			claims[key] = append(claims[key], stepID)
		}
	}
	for key, steps := range claims {
		if len(steps) < 2 {
			continue
		}
		parts := strings.SplitN(key, "\x00", 2)
		out = append(out, fmt.Sprintf("检测 %s 的 %s 被 %d 个步骤共享；任务按 path / Step ID 区分，不按时间点合并。",
			parts[0], parts[1], len(steps)))
	}
	for assayID, params := range d.Assays {
		declared := map[string]bool{}
		for _, timepoint := range params.Timepoints {
			declared[timepoint] = true
		}
		seen := map[string]bool{}
		for _, well := range params.Wells {
			if well.Timepoint == "" || declared[well.Timepoint] || seen[well.Timepoint] {
				continue
			}
			seen[well.Timepoint] = true
			out = append(out, fmt.Sprintf("检测 %s 的孔位使用了未在检测时间点中声明的时间点 %s。", assayID, well.Timepoint))
		}
	}
	sort.Strings(out)
	return out
}

func (d *Designer) CreateRun(ctx context.Context) {
	d.Error = ""
	if len(d.Editor.Doc.Nodes) == 0 {
		d.Error = "请先添加实验步骤，再创建运行。"
		return
	}
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

// View renders the workflow canvas under one command row. The inspector is a
// floating, non-modal panel over the canvas's trailing edge, so selecting a
// step never reflows the design.
func View(c *ui.Context, d *Designer) {
	t := c.Theme()
	narrow := false
	if w, _ := c.Size(); w > 0 && w < 1100 {
		narrow = true
	}
	if narrow && !d.InspectorUserToggled {
		d.InspectorOpen = false
	}

	c.OnShortcut(ui.Cmd, ui.KeyK, func() {
		d.CmdPaletteOpen = !d.CmdPaletteOpen
	})
	c.OnShortcut(ui.Cmd, ui.KeyS, func() {
		d.Save(context.Background())
	})
	c.OnShortcut(ui.Cmd|ui.Alt, ui.KeyI, func() {
		d.InspectorOpen = !d.InspectorOpen
		d.InspectorUserToggled = true
	})
	c.OnShortcut(ui.Cmd, ui.KeyZ, func() { d.UndoDraft() })
	c.OnShortcut(ui.Cmd|ui.Shift, ui.KeyZ, func() { d.RedoDraft() })
	c.OnShortcut(ui.Cmd, ui.KeyY, func() { d.RedoDraft() })

	ui.Column(c).Fill().Background(t.Background).Children(func() {
		toolbar(c, d)
		if narrow {
			ui.Box(c).Fill().Grow(1).MinHeight(300).Children(func() {
				if d.Editor != nil {
					d.Editor.View(c)
				}
			})
			if d.InspectorOpen {
				ui.Box(c).FillWidth().Height(320).Shrink(0).Children(func() {
					inspectorColumn(c, d)
				})
			}
		} else {
			ui.Row(c).Fill().Grow(1).Children(func() {
				ui.Box(c).Grow(1).MinWidth(0).Children(func() {
					if d.Editor != nil {
						d.Editor.View(c)
					}
				})
				if d.InspectorOpen {
					ui.Box(c).Width(360).Shrink(0).FillHeight().Children(func() {
						inspectorColumn(c, d)
					})
				}
			})
		}
		statusBar(c, d)
	})
	if narrow && d.InspectorOpen {
		// bottom sheet: keep overlay for sheet feel but non-occluding of Run button (header above)
	}
	renderCommandPalette(c, d)
	confirmCreateRun(c, d)
	d.captureDraft()
}

func inspectorColumn(c *ui.Context, d *Designer) {
	t := c.Theme()
	ui.Column(c).Fill().Background(t.Background.Alpha(.97)).Border(1, t.Border).Children(func() {
		panelHeader(c, d)
		ui.Divider(c)
		panelBody(c, d)
	})
}
// inspectorPanel floats above the canvas. It restates Onevra's dialog panel:
// the same material, doubled radius, hairline edge and soft lift, but
// non-modal so the canvas keeps working behind it. A handle at its top center
// drags it between the right edge and the bottom of the canvas, and its modules
// lay themselves out for whichever dock it takes.
//
// ponytail: docks and a fixed size, no free-floating windows; add dragging to
// arbitrary coordinates only if a real task needs it.
func inspectorPanel(c *ui.Context, d *Designer) {
	t := c.Theme()
	winW, winH := c.Size()
	header := sidebar.HeaderHeight(c)
	top := header + t.Space(12)
	bottom := t.Space(8) + t.Space(3)
	left := float32(winW) - t.Space(3) - 360
	width := float32(360)
	height := float32(winH) - top - bottom
	if d.InspectorDock == DockBottom {
		left = 232 + t.Space(3)
		top = float32(winH) - bottom - 320
		width = float32(winW) - left - t.Space(3)
		height = 320
	}
	ui.Overlay(c, func() {
		ui.Box(c).Absolute().Left(left).Top(top).Width(width).Height(height).
			Translate(d.panelDrag.X, d.panelDrag.Y).Clip().
			Radius(t.Radius*2).Border(1, t.Background.Mix(t.Text, .25)).
			Background(t.Background.Alpha(.97)).
			Shadow(0, t.Space(2), t.Space(6), 0, ui.RGBA(0, 0, 0, .14)).
			Children(func() {
				ui.Column(c).Fill().Children(func() {
					panelHandle(c, d)
					panelHeader(c, d)
					ui.Divider(c)
					panelBody(c, d)
				})
			})
	})
}

// panelHandle is the top-center grip. A sideways drag leaves the panel on the
// right edge; a downward drag parks it along the bottom.
func panelHandle(c *ui.Context, d *Designer) {
	t := c.Theme()
	strip := ui.Box(c).FillWidth().Height(t.Space(4)).Center().
		Draggable(&d.panelDrag, ui.DraggableOptions{}).Children(func() {
		ui.Box(c).Size(t.Space(10), t.Space(1)).Radius(t.Space(.5)).Background(t.Text.Alpha(.22))
	})
	if d.panelDragging && !strip.Dragging() {
		d.InspectorDock = DockRight
		if d.panelDrag.Y > 0 && abs(d.panelDrag.Y) > abs(d.panelDrag.X) {
			d.InspectorDock = DockBottom
		}
		d.panelDrag = ui.Point{}
	}
	d.panelDragging = strip.Dragging()
}

func panelHeader(c *ui.Context, d *Designer) {
	t := c.Theme()
	ui.Row(c).FillWidth().Padding(0, t.Space(2), t.Space(2), t.Space(3)).
		Gap(t.Space(1)).AlignItems(ui.Center).Children(func() {
		components.Text(c, "检查器").FontWeight(500)
		if d.SelectedNode != "" {
			components.Text(c, "· "+d.nodeTitle(d.SelectedNode)).TextColor(t.TextMuted).
				FontSize(t.Rem(.78)).SingleLine().Ellipsis("…")
		}
		ui.Spacer(c)
		if d.InspectorDock == DockBottom {
			if components.IconButton(c, lucide.PanelRight(), "停靠到右侧", components.IconButtonCompact).Clicked() {
				d.InspectorDock = DockRight
			}
		} else if components.IconButton(c, lucide.PanelBottom(), "停靠到底部", components.IconButtonCompact).Clicked() {
			d.InspectorDock = DockBottom
		}
		if components.IconButton(c, lucide.X(), "收起检查器", components.IconButtonCompact).Clicked() {
			d.InspectorOpen = false
		}
	})
}

// panelBody renders the inspector as modules. Each concern of the selected
// step is its own card, and the dock decides whether modules stack or sit side
// by side: 步骤, 检测参数, 孔板映射, 写入所选孔位, and the dilution calculator.
func panelBody(c *ui.Context, d *Designer) {
	t := c.Theme()
	modules := func() {
		stepInspector(c, d)
		if d.SelectedNode == "" {
			return
		}
		assayInspector(c, d)
		modulesSection(c, d)
		if d.AssayIDDraft != "" {
			plateModule(c, d)
			assignmentModule(c, d)
		}
	}
	if d.InspectorDock == DockBottom {
		view := components.ScrollView(c, components.ScrollViewOptions{Axes: components.ScrollBoth}).
			Grow(1).MinHeight(0).ScrollbarInsets(t.Space(2), t.Space(2), t.Space(2), t.Space(3))
		view.Children(func() {
			ui.Row(c).Padding(t.Space(3)).Gap(t.Space(3)).AlignItems(ui.Start).Children(func() {
				module(c, 340, func() { stepInspector(c, d) })
				if d.SelectedNode == "" {
					return
				}
				module(c, 340, func() { assayInspector(c, d) })
				module(c, 340, func() { modulesSection(c, d) })
				if d.AssayIDDraft != "" {
					module(c, 360, func() { plateModule(c, d) })
					module(c, 360, func() { assignmentModule(c, d) })
				}
			})
		})
		return
	}
	ui.Scroll(c).Grow(1).Padding(t.Space(3), t.Space(3), t.Space(4), t.Space(3)).Children(func() {
		ui.Column(c).Gap(t.Space(3)).Children(modules)
	})
	if d.ConfirmClearWells {
		confirmClearWells(c, d)
	}
}

// module is one card's worth of inspector content at a fixed width.
func module(c *ui.Context, width float32, build func()) {
	ui.Box(c).Width(width).Shrink(0).Children(build)
}

func abs(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}

// plateModule is the 96-well plan as one module: the mapping visual, its
// legend and the two edits that act on the whole plate.
func plateModule(c *ui.Context, d *Designer) {
	t := c.Theme()
	params := d.Assays[d.AssayIDDraft]
	assignedCount := uniqueWellCount(params.Wells)
	wells := make(map[string]plateui.WellState, assignedCount)
	for _, assignment := range params.Wells {
		id := strings.ToUpper(strings.TrimSpace(assignment.WellID))
		current, exists := wells[id]
		if !exists {
			wells[id] = plateui.WellState{
				Group: assignment.GroupID, SampleID: assignment.SampleID,
				Replicate: assignment.Replicate, Timepoint: assignment.Timepoint,
			}
			continue
		}
		current.Timepoint = appendTimepoint(current.Timepoint, assignment.Timepoint)
		if current.Group != assignment.GroupID || current.SampleID != assignment.SampleID || current.Replicate != assignment.Replicate {
			current.Ambiguous = true
			current.Group = ""
		}
		wells[id] = current
	}
	properties(c, "96 孔板计划映射", func(list *propertyList) {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
			components.Text(c, fmt.Sprintf("%d 个孔位 · %d 条时间点分配", assignedCount, len(params.Wells))).
				TextColor(t.TextMuted).FontSize(t.Rem(.78))
			ui.Spacer(c)
			if len(d.SelectedWells) > 0 && components.Button(c, "清除选择", components.ButtonGhost).Clicked() {
				d.selectWells(nil)
			}
			if assignedCount > 0 && components.Button(c, "清空", components.ButtonGhost).Clicked() {
				d.ConfirmClearWells = true
			}
		})
		ui.Box(c).FillWidth().AlignItems(ui.Center).Children(func() {
			plateui.View(c, plateui.Props{
				Wells: wells, SelectedIDs: d.SelectedWells,
				Interactive: d.SelectedNode != "" && d.AssayIDDraft != "",
				OnSelect:    func(ids []string) { d.selectWells(ids) },
			})
		})
		components.Text(c, "孔位颜色表示计划组别；映射属于设计，不是实际执行记录。").
			TextColor(t.TextMuted).FontSize(t.Rem(.75))
	})
}

func assayInspector(c *ui.Context, d *Designer) {
	t := c.Theme()
	if d.AssayIDDraft == "" {
		properties(c, "检测参数", func(list *propertyList) {
			list.row("未关联检测", "在步骤属性中填写检测 ID 后，这里出现该检测的参数。", func() {})
		})
		return
	}
	changed := false
	properties(c, "检测参数", func(list *propertyList) {
		tagRow(list, "检测时间点", "该检测自己的时间点，用逗号分隔", &d.TimepointsDraft, "例如 24 h", &changed)
		if textRow(list, "孔板实例 ID", "引用已有孔板实例；未知时留空", &d.PlateInstanceDraft,
			"未引用孔板", lucide.Table2()) {
			changed = true
		}
		if textRow(list, "技术重复数", "正整数；未知时留空", &d.ReplicatesDraft,
			"未记录", lucide.Hash()) {
			changed = true
		}
		selectRow(list, "对照组", "用于比较的组别", &d.ControlGroupDraft, groupIDs(d, d.ControlGroupDraft), &changed)
		selectRow(list, "校准组", "用于校准的组别", &d.CalibratorGroupDraft, groupIDs(d, d.CalibratorGroupDraft), &changed)
	})
	if changed {
		if err := d.saveAssayDraft(); err != nil {
			d.Error = err.Error()
		} else {
			d.Error = ""
		}
	}
	components.Text(c, "这些是设计参数；实际仪器设置和执行记录属于 Run。").
		TextColor(t.TextMuted).FontSize(t.Rem(.75))
}

// confirmCreateRun shows the preflight warnings before freezing the design.
func confirmCreateRun(c *ui.Context, d *Designer) {
	if !d.ConfirmCreateRun {
		return
	}
	t := c.Theme()
	components.Dialog(c, &d.ConfirmCreateRun, components.DialogOptions{Label: "创建运行前检查"}, func(panel ui.Element) {
		panel.Width(540).Children(func() {
			components.Text(c, "创建运行前检查").FontSize(t.Rem(1.1)).FontWeight(600)
			components.Text(c, "以下任务在运行中不会自动关联设计模块。可以返回修改，或按当前设计创建。").
				TextColor(t.TextMuted).FontSize(t.Rem(.85))
			ui.Column(c).Gap(t.Space(1.5)).Children(func() {
				for _, warning := range d.CreateRunWarnings {
					ui.Row(c).Gap(t.Space(2)).AlignItems(ui.Start).Children(func() {
						ui.Box(c).Size(t.Space(1.5), t.Space(1.5)).Radius(t.Space(1)).Background(t.Warning).
							Margin(t.Space(1.5), 0, 0, 0).Shrink(0)
						components.Text(c, warning).FontSize(t.Rem(.85))
					})
				}
			})
			ui.Row(c).FillWidth().Wrap().Justify(ui.End).Gap(t.Space(2)).Children(func() {
				if components.Button(c, "返回修改", components.ButtonSecondary).Clicked() {
					d.CreateRunWarnings = nil
					d.ConfirmCreateRun = false
				}
				if components.Button(c, "仍要创建", components.ButtonPrimary).Clicked() {
					warnings := append([]string(nil), d.CreateRunWarnings...)
					d.PreflightAck = &PreflightAck{
						Warnings:       warnings,
						ActorID:        os.Getenv("USERNAME"),
						AcknowledgedAt: time.Now().Format(time.RFC3339),
					}
					d.CreateRunWarnings = nil
					d.ConfirmCreateRun = false
					d.Save(context.Background())
					if d.SaveState == SaveSaved && !d.Dirty {
						d.CreateRun(context.Background())
					}
				}
			})
		})
	})
	if !d.ConfirmCreateRun {
		d.CreateRunWarnings = nil
	}
}

// toolbar is a floating island over the canvas: creation and navigation tools
// on the left, document actions on the right. Text lives in tooltips; the
// canvas stays the visual focus.
func toolbar(c *ui.Context, d *Designer) {
	t := c.Theme()
	add := func(kind workflow.NodeKind, title string) {
		d.Editor.AddNode(kind, title, workflow.Position{
			X: 80 + float32(len(d.Editor.Doc.Nodes)%4)*220,
			Y: 80,
		})
	}
	island := func(children func()) {
		ui.Row(c).Padding(t.Space(.75), t.Space(1)).Radius(t.Space(1.5)).
			Background(t.Surface).Border(1, t.Border).
			Shadow(0, 2, 14, 0, t.Text.Alpha(.08)).Gap(t.Space(.5)).
			AlignItems(ui.Center).Children(children)
	}
	divider := func() { ui.Box(c).Width(1).Height(18).Background(t.Border) }
	ui.Row(c).FillWidth().Padding(t.Space(2), t.Space(3)).AlignItems(ui.Center).
		Gap(t.Space(2)).Children(func() {
		island(func() {
			if components.IconButton(c, lucide.FlaskConical(), "新建材料步骤", components.IconButtonPlain).Clicked() {
				add(workflow.KindFlask, "新材料")
			}
			if components.IconButton(c, lucide.Pipette(), "新建操作步骤", components.IconButtonPlain).Clicked() {
				add(workflow.KindPipette, "新操作")
			}
			if components.IconButton(c, lucide.Microscope(), "新建仪器步骤", components.IconButtonPlain).Clicked() {
				add(workflow.KindPlate, "新仪器")
			}
			divider()
			if components.IconButton(c, lucide.Undo2(), "撤销 (Ctrl+Z)", components.IconButtonPlain).Clicked() {
				d.UndoDraft()
			}
			if components.IconButton(c, lucide.Redo2(), "重做 (Ctrl+Shift+Z)", components.IconButtonPlain).Clicked() {
				d.RedoDraft()
			}
			divider()
			connect := components.IconButton(c, lucide.Link2(), "连接步骤", components.IconButtonPlain)
			if d.Editor.ConnectMode {
				connect.Background(t.Accent.Alpha(.14)).TextColor(t.Accent)
			}
			if connect.Clicked() {
				d.Editor.ConnectMode = !d.Editor.ConnectMode
				if !d.Editor.ConnectMode {
					d.Editor.ConnectFrom = ""
				}
			}
			if components.IconButton(c, lucide.Maximize2(), "适应画布", components.IconButtonPlain).Clicked() {
				d.Editor.PanX, d.Editor.PanY, d.Editor.Zoom = 0, 0, 1
			}
			zoom := d.Editor.Zoom
			if zoom <= 0 {
				zoom = 1
			}
			components.Text(c, fmt.Sprintf("%.0f%%", zoom*100)).
				TextColor(t.TextMuted).FontSize(t.Rem(.78)).FontFeatures("tnum").
				Padding(0, t.Space(.5))
		})
		ui.Spacer(c)
		if d.Loading {
			components.Text(c, "加载中…").TextColor(t.TextMuted).FontSize(t.Rem(.85))
		}
		if d.Error != "" {
			components.Text(c, d.Error).TextColor(t.Danger).FontSize(t.Rem(.85)).
				SingleLine().Ellipsis("…").MaxWidth(360)
		}
		saveBadge(c, d)
		island(func() {
			if components.IconButton(c, lucide.Play(), "创建运行", components.IconButtonPlain).Clicked() {
				d.RequestCreateRun(context.Background())
			}
			if components.IconButton(c, lucide.Save(), "保存", components.IconButtonPlain).Clicked() {
				d.Save(context.Background())
			}
			divider()
			if components.IconButton(c, lucide.PanelRight(), map[bool]string{true: "收起检查器", false: "展开检查器"}[d.InspectorOpen], components.IconButtonPlain).Clicked() {
				d.InspectorOpen = !d.InspectorOpen
				d.InspectorUserToggled = true
			}
		})
	})
}

func saveBadge(c *ui.Context, d *Designer) {
	switch {
	case d.SaveState == SaveSaving:
		components.Badge(c, "正在保存…", components.BadgeOptions{Tone: components.BadgeNeutral})
	case d.SaveState == SaveError:
		components.Badge(c, "保存失败", components.BadgeOptions{Tone: components.BadgeDanger})
	case d.Dirty:
		components.Badge(c, fmt.Sprintf("未保存 · rev %d", d.Revision), components.BadgeOptions{Tone: components.BadgeWarning})
	case d.SaveState == SaveSaved:
		components.Badge(c, fmt.Sprintf("已保存 · rev %d", d.Revision), components.BadgeOptions{Tone: components.BadgeSuccess})
	default:
		components.Badge(c, fmt.Sprintf("就绪 · rev %d", d.Revision))
	}
}

// statusBar closes the surface with the design's own counts and the shortcuts
// that act on it. It never restates a value the toolbar already shows.
func statusBar(c *ui.Context, d *Designer) {
	t := c.Theme()
	wells := uniqueWellCount(d.Assays[d.AssayIDDraft].Wells)
	counts := fmt.Sprintf("%d 个步骤 · %d 条检测 · %d 个计划孔位",
		len(d.Editor.Doc.Nodes), len(d.Assays), wells)
	ui.Divider(c)
	ui.Row(c).Height(t.Space(8)).Padding(0, t.Space(3)).Gap(t.Space(2)).AlignItems(ui.Center).Children(func() {
		components.Text(c, counts).FontSize(t.Rem(.75)).TextColor(t.TextMuted)
		ui.Spacer(c)
		if d.SelectedNode != "" {
			components.Text(c, "已选："+d.nodeTitle(d.SelectedNode)).FontSize(t.Rem(.75)).
				TextColor(t.TextMuted).SingleLine().Ellipsis("…").MaxWidth(280)
		}
		components.Text(c, "⌘I 检查器 · ⌘K 命令 · ⌘S 保存").FontSize(t.Rem(.75)).TextColor(t.TextMuted)
	})
}

func renderCommandPalette(c *ui.Context, d *Designer) {
	if !d.CmdPaletteOpen {
		return
	}
	t := c.Theme()
	components.Dialog(c, &d.CmdPaletteOpen, components.DialogOptions{Label: "命令"}, func(panel ui.Element) {
		panel.Width(520).Children(func() {
			components.TextBox(c, &d.CmdQuery, components.TextBoxOptions{LeadingIcon: lucide.Search()}).
				Label("搜索操作").Placeholder("搜索操作或步骤…").AutoFocus().FillWidth()
			ui.Divider(c)
			ui.Scroll(c).MaxHeight(320).Children(func() {
				cmdItem := func(title, desc string, act func()) {
					if d.CmdQuery != "" && !strings.Contains(strings.ToLower(title), strings.ToLower(d.CmdQuery)) && !strings.Contains(strings.ToLower(desc), strings.ToLower(d.CmdQuery)) {
						return
					}
					b := ui.ButtonBase(c).Key(title).Label(title).FillWidth().Justify(ui.Start).
						Padding(t.Space(2), t.Space(2.5)).Radius(t.Radius)
					b.Children(func() {
						ui.Column(c).Grow(1).MinWidth(0).Gap(t.Space(.5)).Children(func() {
							components.Text(c, title).FontWeight(500)
							components.Text(c, desc).FontSize(t.Rem(.8)).TextColor(t.TextMuted).SingleLine().Ellipsis("…")
						})
					})
					if b.Clicked() {
						act()
						d.CmdPaletteOpen = false
						d.CmdQuery = ""
					}
				}

				cmdItem("添加材料节点", "在画布中添加一个空白材料节点", func() {
					d.Editor.AddNode(workflow.KindFlask, "新材料", workflow.Position{X: 60 + float32(len(d.Editor.Doc.Nodes)*220), Y: 70})
				})
				cmdItem("添加操作步骤", "在画布中添加一个空白操作节点", func() {
					d.Editor.AddNode(workflow.KindPipette, "新操作", workflow.Position{X: 60 + float32(len(d.Editor.Doc.Nodes)*220), Y: 70})
				})
				cmdItem("添加仪器节点", "在画布中添加一个空白仪器节点", func() {
					d.Editor.AddNode(workflow.KindPlate, "新仪器", workflow.Position{X: 60 + float32(len(d.Editor.Doc.Nodes)*220), Y: 70})
				})
				cmdItem("水平对齐所有节点", "将所有工作流节点按 220px 间距排列在 Y=70 轴线", func() {
					for i := range d.Editor.Doc.Nodes {
						d.Editor.Doc.Nodes[i].Pos = workflow.Position{X: 60 + float32(i*220), Y: 70}
					}
					d.Editor.PanX, d.Editor.PanY, d.Editor.Zoom = 0, 0, 1
				})
				cmdItem("保存当前设计", "保存工作流结构与孔位计划映射", func() {
					d.Save(context.Background())
				})
				cmdItem("创建实验运行", "冻结当前实验设计版本并创建可追溯执行批次", func() {
					d.RequestCreateRun(context.Background())
				})
			})
		})
	})
}

// propertyList draws Onevra-style setting rows inside one bordered surface: a
// label and an optional description above its control.
type propertyList struct {
	c     *ui.Context
	first bool
}

func (p *propertyList) separator() {
	if !p.first {
		ui.Divider(p.c)
	}
	p.first = false
}

func (p *propertyList) row(label, desc string, control func()) {
	t := p.c.Theme()
	p.separator()
	ui.Column(p.c).FillWidth().Padding(t.Space(2.5), t.Space(3)).Gap(t.Space(1.5)).Children(func() {
		ui.Column(p.c).Gap(t.Space(.5)).Children(func() {
			components.Text(p.c, label).FontSize(t.Rem(.82)).FontWeight(500)
			if desc != "" {
				components.Text(p.c, desc).FontSize(t.Rem(.72)).TextColor(t.TextMuted)
			}
		})
		control()
	})
}

func properties(c *ui.Context, title string, build func(*propertyList)) {
	t := c.Theme()
	ui.Column(c).Gap(t.Space(1.5)).Children(func() {
		components.Text(c, title).FontSize(t.Rem(.75)).Bold().TextColor(t.TextMuted)
		ui.Column(c).FillWidth().Radius(float32(14)).Border(1, t.Border).
			Background(t.Surface).Children(func() {
			build(&propertyList{c: c, first: true})
		})
	})
}

func textRow(list *propertyList, label, desc string, value *string, placeholder string, icon *ui.SVG) bool {
	changed := false
	list.row(label, desc, func() {
		ui.Box(list.c).Key(label).Children(func() {
			if components.TextBox(list.c, value, components.TextBoxOptions{LeadingIcon: icon}).
				Label(label).Placeholder(placeholder).FillWidth().Changed() {
				changed = true
			}
		})
	})
	return changed
}

func tagRow(list *propertyList, label, desc string, value *string, placeholder string, changed *bool) {
	list.row(label, desc, func() {
		tags := splitCSV(*value)
		ui.Box(list.c).Key(label).Children(func() {
			if components.TagInput(list.c, &tags).Label(label).Placeholder(placeholder).
				FillWidth().Changed() {
				*value = strings.Join(tags, ", ")
				*changed = true
			}
		})
	})
}

func selectRow(list *propertyList, label, desc string, value *string, options []string, changed *bool) {
	list.row(label, desc, func() {
		ui.Box(list.c).Key(label).Children(func() {
			sel := components.Select(list.c, value)
			sel.Trigger.Children(func() {
				text := *value
				if text == "" {
					components.Text(list.c, "未设置").TextColor(list.c.Theme().TextMuted).SingleLine()
					return
				}
				components.Text(list.c, text).SingleLine().Ellipsis("…")
			})
			sel.Popup(func(panel ui.Element) {
				for _, opt := range append([]string{""}, options...) {
					item := sel.Item(opt)
					item.Children(func() {
						label := opt
						if label == "" {
							label = "未设置"
						}
						components.Text(list.c, label).SingleLine()
					})
				}
			})
			if sel.Trigger.Changed() {
				*changed = true
			}
		})
	})
}

// groupIDs lists the groups already mapped in the current assay, plus value.
func groupIDs(d *Designer, value string) []string {
	seen := map[string]bool{}
	var out []string
	for _, a := range d.Assays[d.AssayIDDraft].Wells {
		if a.GroupID != "" && !seen[a.GroupID] {
			seen[a.GroupID] = true
			out = append(out, a.GroupID)
		}
	}
	if value != "" && !seen[value] {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

// field is a labelled Onevra text box outside the module cards; it reports an

func stepInspector(c *ui.Context, d *Designer) {
	if d.SelectedNode == "" {
		properties(c, "步骤", func(list *propertyList) {
			list.row("未选择步骤", "在画布中选择一个步骤，或从上方添加材料、操作或仪器节点。", func() {})
		})
		return
	}
	title := d.nodeTitle(d.SelectedNode)
	properties(c, "步骤属性", func(list *propertyList) {
		if textRow(list, "步骤名称", "", &title, "为该步骤命名", lucide.Type()) {
			d.setNodeTitle(d.SelectedNode, title)
		}
		if textRow(list, "关联组件", "组件定义 ID，多个以逗号分隔", &d.ComponentRefsDraft,
			"未关联组件", lucide.Boxes()) {
			d.setComponentRefs(d.SelectedNode, d.ComponentRefsDraft)
		}
	})
	properties(c, "检测关联", func(list *propertyList) {
		if textRow(list, "检测 ID", "这一步引用的检测方案", &d.AssayIDInputDraft,
			"例如 cck8", lucide.FlaskConical()) {
			if err := d.setAssayID(d.SelectedNode, d.AssayIDInputDraft); err != nil {
				d.AssayIDInputDraft = d.AssayIDDraft
				d.Error = err.Error()
			} else {
				d.Error = ""
			}
		}
		list.row("本步骤时间点", "该步骤负责的执行时间点；留空的任务保持未关联", func() {
			tags := splitCSV(d.StepTimepointsDraft)
			placeholder := "例如 24 h"
			if points := d.Assays[d.AssayIDDraft].Timepoints; len(points) > 0 {
				placeholder = strings.Join(points, ", ")
			}
			ui.Box(c).Key("step-timepoints").Children(func() {
				if components.TagInput(c, &tags).Label("本步骤时间点").
					Placeholder(placeholder).FillWidth().Changed() {
					d.StepTimepoints[d.SelectedNode] = tags
					d.StepTimepointsDraft = strings.Join(tags, ", ")
					d.Touch()
				}
			})
		})
	})
}

func assignmentModule(c *ui.Context, d *Designer) {
	assignmentSection(c, d)
}

func assignmentSection(c *ui.Context, d *Designer) {
	t := c.Theme()
	wells := append([]string(nil), d.SelectedWells...)
	sort.Strings(wells)
	wellCount := len(wells)
	params := d.Assays[d.AssayIDDraft]
	timepoint := strings.TrimSpace(d.WellTimepointDraft)
	replacements, otherTimepoints := 0, 0
	for _, wellID := range wells {
		for _, assignment := range params.Wells {
			if !strings.EqualFold(assignment.WellID, wellID) {
				continue
			}
			if timepoint != "" && assignment.Timepoint == timepoint {
				replacements++
			} else {
				otherTimepoints++
			}
		}
	}
	if wellCount == 0 {
		properties(c, "写入所选孔位", func(list *propertyList) {
			list.row("尚未选择孔位", "在孔板上单击选择；Shift+单击可追加。", func() {})
		})
		return
	}
	components.Text(c, "孔位顺序："+strings.Join(wells, ", ")).
		TextColor(t.TextMuted).FontSize(t.Rem(.75))
	touched := false
	properties(c, "写入所选孔位", func(list *propertyList) {
		selectRow(list, "组别", "这批孔位所属的组", &d.GroupIDDraft, groupIDs(d, d.GroupIDDraft), &touched)
		if textRow(list, "样本 ID", "计划样本标识，不推断独立 n", &d.SampleIDDraft,
			"未记录样本", lucide.CircleDot()) {
			touched = true
		}
		selectRow(list, "时间点", "这批孔位对应的时间点", &d.WellTimepointDraft,
			params.Timepoints, &touched)
		tagRow(list, "重复编号", fmt.Sprintf("按孔位顺序的 %d 个正整数", wellCount),
			&d.WellReplicatesDraft, "例如 1, 2", &touched)
	})
	if touched {
		d.Touch()
	}
	if replacements > 0 {
		components.Text(c, fmt.Sprintf("保存时将替换当前时间点的 %d 条分配。", replacements)).
			TextColor(t.TextMuted).FontSize(t.Rem(.75))
	}
	if otherTimepoints > 0 {
		components.Text(c, fmt.Sprintf("所选孔位另有 %d 条其他时间点分配；本次不会移除。", otherTimepoints)).
			TextColor(t.TextMuted).FontSize(t.Rem(.75))
	}
	ui.Row(c).Children(func() {
		if components.Button(c, fmt.Sprintf("保存到所选 %d 孔", wellCount), components.ButtonPrimary).Clicked() {
			if err := d.AssignSelectedWells(); err != nil {
				d.Error = err.Error()
			} else {
				d.Error = ""
			}
		}
	})
}

func confirmClearWells(c *ui.Context, d *Designer) {
	count := len(d.Assays[d.AssayIDDraft].Wells)
	result := components.AlertDialog(c, &d.ConfirmClearWells, components.AlertDialogOptions{
		Title:        "清空当前检测的孔位计划？",
		Message:      fmt.Sprintf("将移除 %d 条时间点分配。操作只修改设计草稿；保存后生效，不会创建或删除实际运行记录。", count),
		ConfirmLabel: "清空映射",
		CancelLabel:  "取消",
	})
	if result != components.DialogConfirmed {
		return
	}
	params := d.Assays[d.AssayIDDraft]
	params.Wells = nil
	d.Assays[d.AssayIDDraft] = params
	d.SelectedWells = nil
	d.loadAssayDraft(d.SelectedNode)
	d.Touch()
	d.Error = ""
}

func (d *Designer) selectWells(ids []string) {
	d.SelectedWells = append(d.SelectedWells[:0], ids...)
	sort.Strings(d.SelectedWells)
	d.SampleIDDraft, d.GroupIDDraft, d.WellTimepointDraft, d.WellReplicatesDraft = "", "", "", ""
	if len(d.SelectedWells) == 0 {
		return
	}
	params := d.Assays[d.AssayIDDraft]
	samples, groups, timepoints := make(map[string]bool), make(map[string]bool), make(map[string]bool)
	replicates := make(map[string]int, len(d.SelectedWells))
	for _, wellID := range d.SelectedWells {
		matches := 0
		for _, assignment := range params.Wells {
			if !strings.EqualFold(assignment.WellID, wellID) {
				continue
			}
			matches++
			samples[assignment.SampleID] = true
			groups[assignment.GroupID] = true
			timepoints[assignment.Timepoint] = true
			replicates[wellID] = assignment.Replicate
		}
		if matches != 1 {
			return
		}
	}
	if len(samples) != 1 || len(groups) != 1 || len(timepoints) != 1 {
		return
	}
	for sample := range samples {
		d.SampleIDDraft = sample
	}
	for group := range groups {
		d.GroupIDDraft = group
	}
	for timepoint := range timepoints {
		d.WellTimepointDraft = timepoint
	}
	values := make([]string, len(d.SelectedWells))
	for i, wellID := range d.SelectedWells {
		values[i] = strconv.Itoa(replicates[wellID])
	}
	d.WellReplicatesDraft = strings.Join(values, ", ")
}

func uniqueWellCount(assignments []assays.WellAssignment) int {
	seen := make(map[string]struct{}, len(assignments))
	for _, assignment := range assignments {
		seen[strings.ToUpper(strings.TrimSpace(assignment.WellID))] = struct{}{}
	}
	return len(seen)
}

func appendTimepoint(existing, next string) string {
	next = strings.TrimSpace(next)
	if next == "" {
		return existing
	}
	for _, item := range strings.Split(existing, ", ") {
		if item == next {
			return existing
		}
	}
	if existing == "" {
		return next
	}
	return existing + ", " + next
}

func containsString(values []string, value string) bool {
	for _, item := range values {
		if item == value {
			return true
		}
	}
	return false
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

// modulesSection lists the selected step's block modules. Each module is the
// reusable blockui renderer over the design's own instance, the same one Run
// shows read-only.
func modulesSection(c *ui.Context, d *Designer) {
	t := c.Theme()
	ui.Column(c).Gap(t.Space(2)).Children(func() {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
			components.Text(c, "模块").FontSize(t.Rem(.75)).Bold().TextColor(t.TextMuted)
			ui.Spacer(c)
			menu := components.Button(c, "添加模块", components.ButtonGhost)
			menu.Menu(func(m *ui.Menu) {
				for _, typ := range blockui.Addable() {
					typ := typ
					if m.Item(blockui.TypeTitle(typ)).Chosen() {
						d.addModule(d.SelectedNode, typ)
					}
				}
			})
		})
		if d.SelectedNode == "" {
			components.Text(c, "选择一个步骤后，这里列出它的模块。").TextColor(t.TextMuted).FontSize(t.Rem(.8))
			return
		}
		insts := d.Instances[d.SelectedNode]
		if len(insts) == 0 {
			components.Text(c, "这一步还没有模块。添加计算器、孔板或备注来承载它需要的参数。").
				TextColor(t.TextMuted).FontSize(t.Rem(.8))
		}
		changed := false
		for i := range insts {
			if blockui.View(c, &insts[i], blockui.Design) {
				changed = true
			}
		}
		d.Instances[d.SelectedNode] = insts
		if changed {
			d.Touch()
		}
	})
}

// addModule appends one reusable module instance to the selected step.
func (d *Designer) addModule(nodeID string, typ blocks.BlockType) {
	def := blockui.NewDefinition(typ)
	inst, err := blocks.NewInstance(fmt.Sprintf("%s-%d", typ, len(d.Instances[nodeID])+1), def)
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
