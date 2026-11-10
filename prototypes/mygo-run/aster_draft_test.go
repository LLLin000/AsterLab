package main

import "testing"

func TestAsterDraftUndoRedoRestoresFields(t *testing.T) {
	a := &app{}
	a.captureAsterDraft()
	a.canvasTitle = "edited"
	a.canvasTasks = []canvasTaskDraft{{ID: "t1", Title: "run"}}
	a.captureAsterDraft()
	if !a.asterDraftUndoStep() {
		t.Fatal("undo failed")
	}
	if a.canvasTitle != "" || len(a.canvasTasks) != 0 {
		t.Fatalf("undo = %q %+v", a.canvasTitle, a.canvasTasks)
	}
	if !a.asterDraftRedoStep() {
		t.Fatal("redo failed")
	}
	if a.canvasTitle != "edited" || len(a.canvasTasks) != 1 {
		t.Fatalf("redo = %q %+v", a.canvasTitle, a.canvasTasks)
	}
}

func TestAsterDraftSelectionReloadDoesNotCreateHistory(t *testing.T) {
	a := &app{}
	a.designSelected = "n1"
	a.canvasSelLoaded = "n1"
	a.canvasTitle = "one"
	a.captureAsterDraft()
	a.designSelected = "n2"
	a.canvasSelLoaded = "n2"
	a.asterDraftSuspend = true
	a.canvasTitle = "two"
	a.captureAsterDraft()
	if len(a.asterDraftUndo) != 0 {
		t.Fatalf("selection created %d draft entries", len(a.asterDraftUndo))
	}
}
