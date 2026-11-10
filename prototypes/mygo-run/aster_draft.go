package main

import (
	"bytes"
	"encoding/json"
)

// asterDraftState is the unsaved state of the AsterDoc workflow editor. It is
// separate from designHistory, which stores committed Core revisions.
type asterDraftState struct {
	Selected   string             `json:"selected"`
	Title      string             `json:"title"`
	Assay      string             `json:"assay"`
	Path       string             `json:"path"`
	Timepoints string             `json:"timepoints"`
	Tasks      []canvasTaskDraft  `json:"tasks"`
	Checks     []canvasCheckDraft `json:"checks"`
}

func (a *app) asterDraftBytes() []byte {
	state := asterDraftState{
		Selected: a.designSelected, Title: a.canvasTitle, Assay: a.canvasAssay,
		Path: a.canvasPath, Timepoints: a.canvasTimepoints,
		Tasks:  append([]canvasTaskDraft(nil), a.canvasTasks...),
		Checks: append([]canvasCheckDraft(nil), a.canvasChecks...),
	}
	raw, _ := json.Marshal(state)
	return raw
}

// captureAsterDraft records one local undo step when an unsaved editor field
// changed. A selection change reloads the inspector and resets the baseline
// instead of creating a fake edit.
func (a *app) captureAsterDraft() {
	if a.asterDraftSelection != a.designSelected {
		// Switching steps reloads the inspector; it is navigation, not an edit.
		a.asterDraftSelection = a.designSelected
		a.asterDraftUndo, a.asterDraftRedo = nil, nil
		a.asterDraftLast = a.asterDraftBytes()
		a.asterDraftSuspend = false
		return
	}
	current := a.asterDraftBytes()
	if a.asterDraftLast == nil || a.asterDraftSuspend {
		a.asterDraftLast = current
		a.asterDraftSuspend = false
		return
	}
	if bytes.Equal(a.asterDraftLast, current) {
		return
	}
	a.asterDraftUndo = append(a.asterDraftUndo, a.asterDraftLast)
	if len(a.asterDraftUndo) > 100 {
		a.asterDraftUndo = a.asterDraftUndo[len(a.asterDraftUndo)-100:]
	}
	a.asterDraftRedo = nil
	a.asterDraftLast = current
}

func (a *app) resetAsterDraftHistory() {
	a.asterDraftUndo, a.asterDraftRedo = nil, nil
	a.asterDraftLast = nil
	a.asterDraftSuspend = false
	a.asterDraftSelection = a.designSelected
}

func (a *app) applyAsterDraft(raw []byte) bool {
	var state asterDraftState
	if err := json.Unmarshal(raw, &state); err != nil {
		return false
	}
	a.designSelected = state.Selected
	a.asterDraftSelection = state.Selected
	a.canvasSelLoaded = state.Selected
	if a.designCanvas != nil {
		a.designCanvas.SelectedNode = state.Selected
	}
	a.canvasTitle, a.canvasAssay = state.Title, state.Assay
	a.canvasPath, a.canvasTimepoints = state.Path, state.Timepoints
	a.canvasTasks = append([]canvasTaskDraft(nil), state.Tasks...)
	a.canvasChecks = append([]canvasCheckDraft(nil), state.Checks...)
	a.asterDraftLast = append([]byte(nil), raw...)
	a.asterDraftSuspend = false
	return true
}

func (a *app) asterDraftUndoStep() bool {
	if len(a.asterDraftUndo) == 0 {
		return false
	}
	current := a.asterDraftBytes()
	last := a.asterDraftUndo[len(a.asterDraftUndo)-1]
	a.asterDraftUndo = a.asterDraftUndo[:len(a.asterDraftUndo)-1]
	if !a.applyAsterDraft(last) {
		return false
	}
	a.asterDraftRedo = append(a.asterDraftRedo, current)
	return true
}

func (a *app) asterDraftRedoStep() bool {
	if len(a.asterDraftRedo) == 0 {
		return false
	}
	current := a.asterDraftBytes()
	next := a.asterDraftRedo[len(a.asterDraftRedo)-1]
	a.asterDraftRedo = a.asterDraftRedo[:len(a.asterDraftRedo)-1]
	if !a.applyAsterDraft(next) {
		return false
	}
	a.asterDraftUndo = append(a.asterDraftUndo, current)
	return true
}
