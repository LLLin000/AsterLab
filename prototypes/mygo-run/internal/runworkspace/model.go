package runworkspace

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"asterlab-mygo-prototype/internal/asterdoc"
	"asterlab-mygo-prototype/internal/workflowtree"
	"asterlab-mygo-prototype/modules/runs"
)

// TaskStatus is the lifecycle status of a task.
type TaskStatus string

const (
	TaskStatusPending   TaskStatus = "pending"   // not started
	TaskStatusCurrent   TaskStatus = "current"   // in progress
	TaskStatusCompleted TaskStatus = "completed" // done
	TaskStatusBlocked   TaskStatus = "blocked"   // blocked by unmet checks
)

// PlannedParameter is a planned input parameter from the frozen design.
type PlannedParameter struct {
	Key       string
	Label     string
	Value     string
	Unit      string
	Dimension string
}

// PlannedCheck is a protocol check definition from the frozen design.
type PlannedCheck struct {
	CheckID     string
	Label       string
	Instruction string
	Blocking    bool
}

// ModuleRef is an SDK module bound to this step/task.
type ModuleRef struct {
	Kind   string
	ID     string
	Schema int
	Title  string
	Role   string
}

// MeasurementView is a single actual measurement.
type MeasurementView struct {
	SampleID string
	Field    string
	Value    string
	Unit     string
	Status   string // "raw", "missing", "excluded"
}

// Deviation is a deviation of actual from planned.
type Deviation struct {
	Field   string
	Planned string
	Actual  string
	Reason  string
}

// ActualRecord is the actual execution record for a task (from Core record store).
type ActualRecord struct {
	Parameters   map[string]string
	Measurements []MeasurementView
	Files        []string
	Deviations   []Deviation
}

// CheckViewModel is a check with its current verification state.
type CheckViewModel struct {
	CheckID     string
	Label       string
	Instruction string
	Blocking    bool
	Status      string // "passed", "failed", "pending", "needs_confirmation"
}

// PlannedStep is the frozen plan for a step/task (read-only, from AsterDoc).
type PlannedStep struct {
	StepID      string
	StepTitle   string
	AssayID     string
	Timepoint   string
	Path        string
	Instruction string

	Parameters []PlannedParameter
	Checks     []PlannedCheck
	Modules    []ModuleRef
}

// RunTaskViewModel is the view model for one execution task.
type RunTaskViewModel struct {
	TaskKey   string
	StepID    string
	StepPath  string
	StepTitle string
	Timepoint string
	AssayID   string

	Status TaskStatus

	Planned *PlannedStep
	Actual  *ActualRecord

	Checks  []CheckViewModel
	Modules []ModuleRef

	// Instruction and Parameters come from the frozen Protocol step, not from
	// raw module fields.
	Instruction string
	Parameters  []PlannedParameter

	Events []EventViewModel

	Children []*RunTaskViewModel
}

// MaterialUseView is one material consumption recorded in an event.
type MaterialUseView struct {
	Material string
	Amount   float64
	Unit     string
}

// LineageView is one structured object-transfer edge recorded in an event.
type LineageView struct {
	FromKind string
	FromID   string
	ToKind   string
	ToID     string
	Relation string
	Note     string
}

// EventViewModel is the read-only audit entry for a task.
type EventViewModel struct {
	ID         string
	Type       string
	Content    string
	OccurredAt string
	RecordedAt string
	Operator   string
	TaskKey    string
	StepPath   string

	Checks       []CheckAckView
	Materials    []MaterialUseView
	Deviation    string
	Lineage      []LineageView
	Measurements []MeasurementView
	Parameters   []PlannedParameter
	SampleIDs    []string
	CheckAck     bool
}

// RunWorkspaceViewModel is the top-level view model for a run execution surface.
type RunWorkspaceViewModel struct {
	RunID            string
	Title            string
	Revision         uint64
	DesignRevision   uint64
	AsterDocRevision uint64
	Status           string

	Steps     []*RunTaskViewModel
	TaskByKey map[string]*RunTaskViewModel

	UnassociatedMeasurements []MeasurementView
	UnassociatedEvents       []EventViewModel

	// Tree is the read-only containment projection used by the Run rail when
	// a real composite exists. TreeIssues holds diagnostics when the hierarchy
	// cannot be trusted; the legacy rail is then used.
	Tree       *workflowtree.Tree
	TreeIssues []string

	Selected *RunTaskViewModel
}

// moduleUse is a parsed step@2.uses entry.
type moduleUse struct {
	Kind   string
	ID     string
	Schema int
	Role   string
}

// parseAsterDocSource parses the frozen AsterDoc source YAML to extract
// module definitions and step@2.uses bindings.
func parseAsterDocSource(source string) (map[string]map[string]any, map[string][]moduleUse) {
	docModules := map[string]map[string]any{}
	stepUses := map[string][]moduleUse{}

	doc, err := asterdoc.Parse(source)
	if err != nil {
		return docModules, stepUses
	}

	for _, m := range doc.Modules {
		docModules[m.ID] = m.Data
	}

	for _, m := range doc.Modules {
		if m.Kind != "step" {
			continue
		}
		uses, ok := m.Data["uses"].([]any)
		if !ok {
			continue
		}
		var usesList []moduleUse
		for _, raw := range uses {
			item, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			ref, _ := item["ref"].(string)
			role, _ := item["role"].(string)
			if ref == "" {
				continue
			}
			if refDoc, ok := docModules[ref]; ok {
				usesList = append(usesList, moduleUse{
					Kind:   textValue(refDoc, "kind"),
					ID:     ref,
					Schema: intVal(refDoc["schema"]),
					Role:   role,
				})
			} else {
				usesList = append(usesList, moduleUse{
					Kind: "",
					ID:   ref,
					Role: role,
				})
			}
		}
		if len(usesList) > 0 {
			stepUses[m.ID] = usesList
		}
	}

	return docModules, stepUses
}

func textValue(data map[string]any, key string) string {
	v, _ := data[key].(string)
	return strings.TrimSpace(v)
}

func intVal(v any) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	case int64:
		return int(n)
	}
	return 0
}

// plannedParameters extracts planned parameter values from module data.
func plannedParameters(data map[string]any) []PlannedParameter {
	var params []PlannedParameter
	for k, v := range data {
		if k == "kind" || k == "id" || k == "schema" {
			continue
		}
		label := strings.Title(k)
		val := fmt.Sprintf("%v", v)
		unit := ""
		dim := ""
		if m, ok := v.(map[string]any); ok {
			if valStr, ok := m["value"].(string); ok {
				val = valStr
			} else if valNum, ok := m["value"]; ok {
				val = fmt.Sprintf("%v", valNum)
			}
			if u, ok := m["unit"].(string); ok {
				unit = u
			}
		}
		params = append(params, PlannedParameter{
			Key:       k,
			Label:     label,
			Value:     val,
			Unit:      unit,
			Dimension: dim,
		})
	}
	sort.Slice(params, func(i, j int) bool {
		return params[i].Key < params[j].Key
	})
	return params
}

// aggregateStatus computes the aggregate status of a step from its tasks.
func aggregateStatus(tasks []*RunTaskViewModel) TaskStatus {
	if len(tasks) == 0 {
		return TaskStatusPending
	}
	allCompleted := true
	anyCurrent := false
	anyBlocked := false
	for _, t := range tasks {
		switch t.Status {
		case TaskStatusCompleted:
		case TaskStatusCurrent:
			anyCurrent = true
			allCompleted = false
		case TaskStatusBlocked:
			anyBlocked = true
			allCompleted = false
		default:
			allCompleted = false
		}
	}
	if anyBlocked {
		return TaskStatusBlocked
	}
	if anyCurrent {
		return TaskStatusCurrent
	}
	if allCompleted {
		return TaskStatusCompleted
	}
	return TaskStatusPending
}

// taskPath extracts the protocol step path carried by a frozen task key. The
// key is a JSON object; legacy keys without a path stay empty.
func taskPath(key string) string {
	var parsed struct {
		Path string `json:"path"`
	}
	if json.Unmarshal([]byte(key), &parsed) != nil {
		return ""
	}
	return parsed.Path
}

func findTaskForRecord(vm *RunWorkspaceViewModel, taskKey, path string) *RunTaskViewModel {
	if taskKey != "" {
		if task := vm.TaskByKey[taskKey]; task != nil {
			return task
		}
	}
	if path == "" {
		return nil
	}
	var found *RunTaskViewModel
	for _, task := range vm.TaskByKey {
		if task.StepPath != path {
			continue
		}
		if found != nil {
			// The same path names more than one frozen task; without a
			// task key the record is ambiguous and stays unassociated.
			return nil
		}
		found = task
	}
	return found
}

func parseRunParameters(raw string) []PlannedParameter {
	if strings.TrimSpace(raw) == "" || strings.TrimSpace(raw) == "[]" {
		return nil
	}
	var items []struct {
		Key   string `json:"key"`
		Label string `json:"label"`
		Name  string `json:"name"`
		Value any    `json:"value"`
		Unit  string `json:"unit"`
	}
	if json.Unmarshal([]byte(raw), &items) != nil {
		return nil
	}
	out := make([]PlannedParameter, 0, len(items))
	for _, item := range items {
		label := item.Label
		if label == "" {
			label = item.Name
		}
		if label == "" {
			label = item.Key
		}
		value := ""
		if item.Value != nil {
			value = fmt.Sprint(item.Value)
		}
		out = append(out, PlannedParameter{Key: item.Key, Label: label, Value: value, Unit: item.Unit})
	}
	return out
}

func eventViewModel(e runs.RunEvent) EventViewModel {
	event := EventViewModel{
		ID: e.ID, Type: e.EventType, Content: e.Content,
		OccurredAt: e.OccurredAt, RecordedAt: e.RecordedAt,
		TaskKey: e.TaskKey, StepPath: e.ProtocolStepPath,
	}
	for _, check := range e.Checks {
		event.Checks = append(event.Checks, CheckAckView{CheckID: check.CheckID, Status: check.Status})
	}
	if strings.TrimSpace(e.DataJSON) == "" {
		return event
	}
	var data struct {
		Operator  string   `json:"operator"`
		CheckAck  bool     `json:"check_ack"`
		SampleIDs []string `json:"sample_ids"`
		Checks    []struct {
			CheckID string `json:"check_id"`
			Status  string `json:"status"`
		} `json:"checks"`
		Materials []struct {
			Material string   `json:"material"`
			Amount   *float64 `json:"amount"`
			Unit     string   `json:"unit"`
		} `json:"materials"`
		Deviation *string `json:"deviation"`
		Lineage   []struct {
			FromKind string `json:"from_kind"`
			FromID   string `json:"from_id"`
			ToKind   string `json:"to_kind"`
			ToID     string `json:"to_id"`
			Relation string `json:"relation"`
			Note     string `json:"note"`
		} `json:"lineage"`
		Measurements []struct {
			SampleID string   `json:"sample_id"`
			Endpoint string   `json:"endpoint"`
			Value    *float64 `json:"value"`
			Unit     string   `json:"unit"`
		} `json:"measurements"`
		ActualParameters []struct {
			Key   string `json:"key"`
			Value any    `json:"value"`
			Unit  string `json:"unit"`
		} `json:"actual_parameters"`
	}
	if json.Unmarshal([]byte(e.DataJSON), &data) != nil {
		return event
	}
	event.Operator = data.Operator
	event.CheckAck = data.CheckAck
	event.SampleIDs = data.SampleIDs
	if len(data.Checks) > 0 {
		event.Checks = nil
		for _, check := range data.Checks {
			event.Checks = append(event.Checks, CheckAckView{CheckID: check.CheckID, Status: check.Status})
		}
	}
	for _, material := range data.Materials {
		amount := 0.0
		if material.Amount != nil {
			amount = *material.Amount
		}
		event.Materials = append(event.Materials, MaterialUseView{Material: material.Material, Amount: amount, Unit: material.Unit})
	}
	if data.Deviation != nil {
		event.Deviation = *data.Deviation
	}
	for _, edge := range data.Lineage {
		event.Lineage = append(event.Lineage, LineageView{
			FromKind: edge.FromKind, FromID: edge.FromID,
			ToKind: edge.ToKind, ToID: edge.ToID,
			Relation: edge.Relation, Note: edge.Note,
		})
	}
	for _, measurement := range data.Measurements {
		value := ""
		if measurement.Value != nil {
			value = fmt.Sprintf("%g", *measurement.Value)
		}
		event.Measurements = append(event.Measurements, MeasurementView{
			SampleID: measurement.SampleID, Field: measurement.Endpoint,
			Value: value, Unit: measurement.Unit, Status: "raw",
		})
	}
	for _, parameter := range data.ActualParameters {
		value := ""
		if parameter.Value != nil {
			value = fmt.Sprint(parameter.Value)
		}
		event.Parameters = append(event.Parameters, PlannedParameter{Key: parameter.Key, Label: parameter.Key, Value: value, Unit: parameter.Unit})
	}
	return event
}

// BuildViewModel constructs the run workspace view model from a frozen run
// snapshot and its frozen AsterDoc.
func BuildViewModel(snap runs.RunSnapshot, doc *asterdoc.FrozenAsterDoc) *RunWorkspaceViewModel {
	vm := &RunWorkspaceViewModel{
		RunID:          snap.RunID,
		Title:          snap.Title,
		Revision:       snap.Revision,
		DesignRevision: snap.DesignRevision,
		Status:         snap.Status,
		TaskByKey:      map[string]*RunTaskViewModel{},
	}
	if snap.AsterDoc != nil {
		vm.AsterDocRevision = snap.AsterDoc.Revision
	}

	docModules := map[string]map[string]any{}
	stepUses := map[string][]moduleUse{}
	if doc != nil && doc.Source != "" {
		docModules, stepUses = parseAsterDocSource(doc.Source)
	}

	taskOrder := make([]string, 0, len(snap.Tasks))
	for _, t := range snap.Tasks {
		taskOrder = append(taskOrder, t.Key)
		path := t.Path
		if path == "" {
			path = taskPath(t.Key)
		}
		params := parseRunParameters(t.ParametersJSON)
		vmTask := &RunTaskViewModel{
			TaskKey:     t.Key,
			StepID:      t.DesignStepID,
			StepPath:    path,
			StepTitle:   t.Title,
			Timepoint:   t.Timepoint,
			AssayID:     t.AssayID,
			Status:      buildTaskStatus(t),
			Instruction: t.Instruction,
			Parameters:  params,
			Actual: &ActualRecord{
				Parameters:   map[string]string{},
				Measurements: []MeasurementView{},
			},
		}
		vmTask.Planned = &PlannedStep{
			StepID:      t.DesignStepID,
			StepTitle:   t.Title,
			AssayID:     t.AssayID,
			Timepoint:   t.Timepoint,
			Path:        path,
			Instruction: t.Instruction,
			Parameters:  params,
		}
		if uses, ok := stepUses[t.DesignStepID]; ok {
			for _, u := range uses {
				if modData, ok := docModules[u.ID]; ok {
					vmTask.Modules = append(vmTask.Modules, ModuleRef{
						Kind:   u.Kind,
						ID:     u.ID,
						Schema: u.Schema,
						Title:  textValue(modData, "title"),
						Role:   u.Role,
					})
				}
			}
		}
		for _, c := range t.Checks {
			status := c.Status
			if status == "" {
				status = "unknown"
			}
			vmTask.Checks = append(vmTask.Checks, CheckViewModel{
				CheckID:     c.CheckID,
				Label:       c.Label,
				Instruction: c.Instruction,
				Blocking:    c.Blocking,
				Status:      status,
			})
		}
		vm.TaskByKey[t.Key] = vmTask
	}

	// Measurements carry their authoritative task/path association. A record
	// without either is unassociated; never attach it to every task.
	for _, m := range snap.Measurements {
		if m.SampleID == "" {
			continue
		}
		view := MeasurementView{
			SampleID: m.SampleID,
			Field:    m.Endpoint,
			Value:    fmt.Sprintf("%g", m.Value),
			Unit:     m.Unit,
			Status:   "raw",
		}
		if target := findTaskForRecord(vm, m.TaskKey, m.ProtocolStepPath); target != nil {
			target.Actual.Measurements = append(target.Actual.Measurements, view)
		} else {
			vm.UnassociatedMeasurements = append(vm.UnassociatedMeasurements, view)
		}
	}

	for _, e := range snap.Events {
		event := eventViewModel(e)
		if target := findTaskForRecord(vm, e.TaskKey, e.ProtocolStepPath); target != nil {
			target.Events = append(target.Events, event)
		} else {
			vm.UnassociatedEvents = append(vm.UnassociatedEvents, event)
		}
	}

	stepOrder := make([]string, 0)
	stepTasks := make(map[string][]*RunTaskViewModel)
	for _, key := range taskOrder {
		t := vm.TaskByKey[key]
		if t.StepID == "" {
			vm.Steps = append(vm.Steps, t)
			continue
		}
		if _, exists := stepTasks[t.StepID]; !exists {
			stepOrder = append(stepOrder, t.StepID)
		}
		stepTasks[t.StepID] = append(stepTasks[t.StepID], t)
	}

	for _, stepID := range stepOrder {
		tasks := stepTasks[stepID]
		if len(tasks) == 0 {
			continue
		}
		first := tasks[0]
		parent := &RunTaskViewModel{
			TaskKey:   stepID,
			StepID:    stepID,
			StepTitle: first.StepTitle,
			Timepoint: first.Timepoint,
			AssayID:   first.AssayID,
			Status:    aggregateStatus(tasks),
			Planned:   first.Planned,
			Modules:   first.Modules,
			Children:  tasks,
		}
		vm.Steps = append(vm.Steps, parent)
	}

	if doc != nil {
		if tree, err := workflowtree.Build(snap.Tasks, doc); err != nil {
			vm.TreeIssues = []string{err.Error()}
		} else {
			// Per-node issues (a design step with no frozen task) must not
			// hide a real containment level: they are diagnostics, and the
			// rail falls back to raw task keys without the tree.
			if len(tree.Issues) > 0 {
				vm.TreeIssues = tree.Issues
			}
			if tree.HasComposite() || tree.HasOrganizational() {
				vm.Tree = tree
			}
		}
	}

	if vm.Selected == nil {
		for _, s := range vm.Steps {
			if s.Status == TaskStatusCurrent || s.Status == TaskStatusPending {
				vm.Selected = s
				if len(s.Children) > 0 {
					vm.Selected = s.Children[0]
				}
				break
			}
		}
		if vm.Selected == nil && len(vm.Steps) > 0 {
			vm.Selected = vm.Steps[0]
			if len(vm.Steps[0].Children) > 0 {
				vm.Selected = vm.Steps[0].Children[0]
			}
		}
	}

	return vm
}

// buildTaskStatus maps the authoritative Core task status to the presentation
// status. Core is the single authority: the UI never infers completion, nor
// promotes an unconfirmed blocking check to a "blocked" task on its own. Blocked
// checks are shown inside the execution panel where they can be confirmed.
func buildTaskStatus(task runs.RunTask) TaskStatus {
	switch task.Status {
	case "completed":
		return TaskStatusCompleted
	case "in_progress", "started", "active":
		return TaskStatusCurrent
	case "blocked":
		return TaskStatusBlocked
	}
	return TaskStatusPending
}
