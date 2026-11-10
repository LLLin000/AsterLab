package designer

import (
	"bytes"
	"encoding/json"

	"asterlab-mygo-prototype/internal/assays"
	"asterlab-mygo-prototype/internal/blocks"
	"asterlab-mygo-prototype/internal/workflow"
)

// draftState is the part of the designer that can be undone locally. Canvas
// pan/zoom, inspector docking and transient selections are deliberately
// excluded: they are view state, not a design draft.
type draftState struct {
	Title               string                       `json:"title"`
	Doc                 workflow.WorkflowDocument    `json:"doc"`
	Instances           map[string][]blocks.Instance `json:"instances"`
	Assays              map[string]assays.Design     `json:"assays"`
	StepAssays          map[string]string            `json:"stepAssays"`
	StepTimepoints      map[string][]string          `json:"stepTimepoints"`
	StepTaskPaths       map[string]string            `json:"stepTaskPaths"`
	AssayIDDraft        string                       `json:"assayIDDraft"`
	AssayIDInput        string                       `json:"assayIDInput"`
	Timepoints          string                       `json:"timepoints"`
	PlateInstance       string                       `json:"plateInstance"`
	Replicates          string                       `json:"replicates"`
	StepTimepointsDraft string                       `json:"stepTimepointsDraft"`
	ControlGroup        string                       `json:"controlGroup"`
	CalibratorGroup     string                       `json:"calibratorGroup"`
	Samples             string                       `json:"samples"`
	Genes               string                       `json:"genes"`
	ReferenceGenes      string                       `json:"referenceGenes"`
	SampleID            string                       `json:"sampleID"`
	GroupID             string                       `json:"groupID"`
	WellTimepoint       string                       `json:"wellTimepoint"`
	WellReplicates      string                       `json:"wellReplicates"`
	ComponentRefs       string                       `json:"componentRefs"`
	ParamKey            string                       `json:"paramKey"`
	ParamValue          string                       `json:"paramValue"`
}

func (d *Designer) snapshotDoc() []byte {
	if d.Editor == nil {
		return nil
	}
	raw, _ := json.Marshal(d.Editor.Doc)
	return raw
}

func (d *Designer) snapshotDraft() []byte {
	if d.Editor == nil {
		d.Editor = workflow.NewEditor()
	}
	state := draftState{
		Title: d.Title, Doc: d.Editor.Doc,
		Instances: d.Instances, Assays: d.Assays,
		StepAssays: d.StepAssays, StepTimepoints: d.StepTimepoints,
		StepTaskPaths: d.StepTaskPaths,
		AssayIDDraft:  d.AssayIDDraft, AssayIDInput: d.AssayIDInputDraft,
		Timepoints: d.TimepointsDraft, PlateInstance: d.PlateInstanceDraft,
		Replicates: d.ReplicatesDraft, StepTimepointsDraft: d.StepTimepointsDraft,
		ControlGroup: d.ControlGroupDraft, CalibratorGroup: d.CalibratorGroupDraft,
		Samples: d.SamplesDraft, Genes: d.GenesDraft, ReferenceGenes: d.ReferenceGenesDraft,
		SampleID: d.SampleIDDraft, GroupID: d.GroupIDDraft,
		WellTimepoint: d.WellTimepointDraft, WellReplicates: d.WellReplicatesDraft,
		ComponentRefs: d.ComponentRefsDraft, ParamKey: d.ParamKeyDraft,
		ParamValue: d.ParamValueDraft,
	}
	raw, _ := json.Marshal(state)
	return raw
}

// captureDraft records one local undo step when the draft changed since the
// previous frame. A node drag is held back until release, so a drag is one
// step rather than one per frame.
func (d *Designer) captureDraft() {
	if d.Editor != nil && d.Editor.MovingNode != "" {
		return
	}
	current := d.snapshotDraft()
	doc := d.snapshotDoc()
	if d.lastDraft == nil {
		d.lastDraft, d.lastDoc = current, doc
		d.historySuspend = false
		return
	}
	if d.historySuspend {
		d.historySuspend = false
		if bytes.Equal(d.lastDoc, doc) {
			// Selection reloaded the inspector drafts; not a draft edit.
			d.lastDraft, d.lastDoc = current, doc
			return
		}
	}
	if bytes.Equal(d.lastDraft, current) {
		return
	}
	d.draftUndo = append(d.draftUndo, d.lastDraft)
	if len(d.draftUndo) > 100 {
		d.draftUndo = d.draftUndo[len(d.draftUndo)-100:]
	}
	d.draftRedo = nil
	d.lastDraft, d.lastDoc = current, doc
}

func (d *Designer) resetDraftHistory() {
	d.draftUndo, d.draftRedo = nil, nil
	d.lastDraft, d.lastDoc = d.snapshotDraft(), d.snapshotDoc()
	d.historySuspend = false
}

func (d *Designer) applyDraft(raw []byte) bool {
	if len(raw) == 0 {
		return false
	}
	var state draftState
	if err := json.Unmarshal(raw, &state); err != nil {
		return false
	}
	if d.Editor == nil {
		d.Editor = workflow.NewEditor()
	}
	d.Title = state.Title
	d.Editor.Doc = state.Doc
	d.Instances = state.Instances
	d.Assays = state.Assays
	d.StepAssays = state.StepAssays
	d.StepTimepoints = state.StepTimepoints
	d.StepTaskPaths = state.StepTaskPaths
	d.AssayIDDraft, d.AssayIDInputDraft = state.AssayIDDraft, state.AssayIDInput
	d.TimepointsDraft, d.PlateInstanceDraft = state.Timepoints, state.PlateInstance
	d.ReplicatesDraft, d.StepTimepointsDraft = state.Replicates, state.StepTimepointsDraft
	d.ControlGroupDraft, d.CalibratorGroupDraft = state.ControlGroup, state.CalibratorGroup
	d.SamplesDraft, d.GenesDraft, d.ReferenceGenesDraft = state.Samples, state.Genes, state.ReferenceGenes
	d.SampleIDDraft, d.GroupIDDraft = state.SampleID, state.GroupID
	d.WellTimepointDraft, d.WellReplicatesDraft = state.WellTimepoint, state.WellReplicates
	d.ComponentRefsDraft, d.ParamKeyDraft = state.ComponentRefs, state.ParamKey
	d.ParamValueDraft = state.ParamValue
	d.SelectedWells = nil
	if d.SelectedNode != "" {
		found := false
		for _, node := range d.Editor.Doc.Nodes {
			if node.ID == d.SelectedNode {
				found = true
				break
			}
		}
		if !found {
			d.SelectedNode = ""
			d.Editor.SelectedNode = ""
			d.loadAssayDraft("")
		}
	}
	d.Empty = len(d.Editor.Doc.Nodes) == 0
	d.lastDraft = append([]byte(nil), raw...)
	d.lastDoc = d.snapshotDoc()
	d.historySuspend = false
	return true
}

// UndoDraft restores the previous local draft snapshot. It does not call Core;
// Save remains an explicit action.
func (d *Designer) UndoDraft() bool {
	if len(d.draftUndo) == 0 {
		return false
	}
	current := d.snapshotDraft()
	last := d.draftUndo[len(d.draftUndo)-1]
	d.draftUndo = d.draftUndo[:len(d.draftUndo)-1]
	if !d.applyDraft(last) {
		return false
	}
	d.draftRedo = append(d.draftRedo, current)
	d.Dirty = true
	d.SaveState = SaveDraft
	d.Error = ""
	return true
}

// RedoDraft restores a draft snapshot removed by UndoDraft.
func (d *Designer) RedoDraft() bool {
	if len(d.draftRedo) == 0 {
		return false
	}
	current := d.snapshotDraft()
	next := d.draftRedo[len(d.draftRedo)-1]
	d.draftRedo = d.draftRedo[:len(d.draftRedo)-1]
	if !d.applyDraft(next) {
		return false
	}
	d.draftUndo = append(d.draftUndo, current)
	d.Dirty = true
	d.SaveState = SaveDraft
	d.Error = ""
	return true
}
