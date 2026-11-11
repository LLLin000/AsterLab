package runworkspace

import (
	"fmt"
	"sort"
	"strings"

	"asterlab-mygo-prototype/internal/asterdoc"
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
	Kind    string
	ID      string
	Schema  int
	Title   string
	Role    string
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

	Parameters []PlannedParameter
	Checks     []PlannedCheck
	Modules    []ModuleRef
}

// RunTaskViewModel is the view model for one execution task.
type RunTaskViewModel struct {
	TaskKey   string
	StepID    string
	StepTitle string
	Timepoint string
	AssayID   string

	Status TaskStatus

	Planned *PlannedStep
	Actual  *ActualRecord

	Checks  []CheckViewModel
	Modules []ModuleRef

	Children []*RunTaskViewModel
}

// RunWorkspaceViewModel is the top-level view model for a run execution surface.
type RunWorkspaceViewModel struct {
	RunID     string
	Title     string
	Revision  uint64
	Status    string

	Steps    []*RunTaskViewModel
	TaskByKey map[string]*RunTaskViewModel

	Selected *RunTaskViewModel
}

// moduleUse is a parsed step@2.uses entry.
type moduleUse struct {
	Kind  string
	ID    string
	Schema int
	Role  string
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
					Kind:  textValue(refDoc, "kind"),
					ID:    ref,
					Schema: intVal(refDoc["schema"]),
					Role:  role,
				})
			} else {
				usesList = append(usesList, moduleUse{
					Kind:  "",
					ID:    ref,
					Role:  role,
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

// BuildViewModel constructs the run workspace view model from a frozen run
// snapshot and its frozen AsterDoc.
func BuildViewModel(snap runs.RunSnapshot, doc *asterdoc.FrozenAsterDoc) *RunWorkspaceViewModel {
	vm := &RunWorkspaceViewModel{
		RunID:     snap.RunID,
		Title:     snap.Title,
		Revision:  snap.Revision,
		Status:    snap.Status,
		TaskByKey: map[string]*RunTaskViewModel{},
	}

	docModules := map[string]map[string]any{}
	stepUses     := map[string][]moduleUse{}
	if doc != nil && doc.Source != "" {
		docModules, stepUses = parseAsterDocSource(doc.Source)
	}

	taskOrder := make([]string, 0, len(snap.Tasks))
	for _, t := range snap.Tasks {
		taskOrder = append(taskOrder, t.Key)

		vmTask := &RunTaskViewModel{
			TaskKey:   t.Key,
			StepID:    t.DesignStepID,
			StepTitle: t.Title,
			Timepoint: t.Timepoint,
			AssayID:   t.AssayID,
			Status:    buildTaskStatus(t),
			Actual: &ActualRecord{
				Parameters:   map[string]string{},
				Measurements: []MeasurementView{},
			},
		}

		if uses, ok := stepUses[t.DesignStepID]; ok {
			vmTask.Planned = &PlannedStep{
				StepID:   t.DesignStepID,
				StepTitle: t.Title,
				AssayID:   t.AssayID,
				Timepoint: t.Timepoint,
				Path:      "",
			}
			for _, u := range uses {
				if modData, ok := docModules[u.ID]; ok {
					vmTask.Modules = append(vmTask.Modules, ModuleRef{
						Kind:    u.Kind,
						ID:      u.ID,
						Schema:  u.Schema,
						Title:   textValue(modData, "title"),
						Role:    u.Role,
					})
					for _, p := range plannedParameters(modData) {
						vmTask.Planned.Parameters = append(vmTask.Planned.Parameters, p)
					}
				}
			}
		}

		for _, c := range t.Checks {
			vmTask.Checks = append(vmTask.Checks, CheckViewModel{
				CheckID:     c.CheckID,
				Label:       c.Label,
				Instruction: c.Instruction,
				Blocking:    c.Blocking,
				Status:      "pending",
			})
		}

		for _, m := range snap.Measurements {
			if m.SampleID != "" {
				vmTask.Actual.Measurements = append(vmTask.Actual.Measurements, MeasurementView{
					SampleID: m.SampleID,
					Field:    m.Endpoint,
					Value:    fmt.Sprintf("%g", m.Value),
					Unit:     m.Unit,
					Status:   "raw",
				})
			}
		}

		vm.TaskByKey[t.Key] = vmTask
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
