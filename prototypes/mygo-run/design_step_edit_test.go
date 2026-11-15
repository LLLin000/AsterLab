package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"asterlab-mygo-prototype/modules/runs"
)

type stepEditService struct {
	runs.DesignService
	doc       runs.AsterDocSnapshot
	applyErr  error
	commands  []runs.AsterDocCommandInput
	saveCalls []runs.AsterDocSaveInput
}

func (s *stepEditService) ApplyAsterDocCommand(_ context.Context, input runs.AsterDocCommandInput) (runs.AsterDocSnapshot, error) {
	s.commands = append(s.commands, input)
	if s.applyErr != nil {
		return runs.AsterDocSnapshot{}, s.applyErr
	}
	s.doc = runs.AsterDocSnapshot{
		DesignID: input.DesignID, Revision: s.doc.Revision + 1,
		Source: fmt.Sprintf("%s\nrevision-%d", s.doc.Source, s.doc.Revision+1),
	}
	return s.doc, nil
}

func (s *stepEditService) SaveAsterDoc(_ context.Context, input runs.AsterDocSaveInput) (runs.AsterDocSnapshot, error) {
	s.saveCalls = append(s.saveCalls, input)
	s.doc = runs.AsterDocSnapshot{DesignID: input.DesignID, Revision: input.ExpectedRevision + 1, Source: input.Source}
	return s.doc, nil
}

func (s *stepEditService) AssessAsterDoc(_ context.Context, designID string, revision uint64) (runs.AsterDocAssessment, error) {
	return runs.AsterDocAssessment{DesignID: designID, Revision: revision, State: "ready"}, nil
}

func stepEditApp() (*app, *stepEditService) {
	service := &stepEditService{doc: runs.AsterDocSnapshot{DesignID: "design-1", Revision: 4, Source: "source-0"}}
	return &app{designSvc: service, designDoc: &service.doc}, service
}

func TestStepCommandReloadsReturnedRevisionAndRecordsUndo(t *testing.T) {
	a, service := stepEditApp()
	if !a.applyDesignRaw(map[string]any{"type": "reorder_steps", "moduleId": "wf", "order": []string{"b", "a"}}) {
		t.Fatal("reorder command failed")
	}
	if len(service.commands) != 1 || service.commands[0].ExpectedRevision != 4 ||
		!strings.Contains(service.commands[0].Command, `"type":"reorder_steps"`) ||
		!strings.Contains(service.commands[0].Command, `"moduleId":"wf"`) {
		t.Fatalf("command calls = %+v", service.commands)
	}
	if a.designDoc.Revision != 5 || a.designDoc.Source != service.doc.Source || len(a.designHistory) != 1 {
		t.Fatalf("returned revision not applied: doc=%+v undo=%d", a.designDoc, len(a.designHistory))
	}
}

func TestRejectedStepDeleteKeepsDocumentAndShowsCoreReason(t *testing.T) {
	a, service := stepEditApp()
	service.applyErr = errors.New("STEP_REFERENCED: step step-a is referenced and cannot be removed")
	before := *a.designDoc
	if a.applyDesignRaw(map[string]any{"type": "remove_step", "moduleId": "step-a"}) {
		t.Fatal("referenced delete unexpectedly succeeded")
	}
	if a.designDoc.Source != before.Source || a.designDoc.Revision != before.Revision || len(a.designHistory) != 0 {
		t.Fatalf("rejected delete changed document/history: doc=%+v undo=%d", a.designDoc, len(a.designHistory))
	}
	if !strings.Contains(a.designDocErr, "STEP_REFERENCED") || !strings.Contains(a.designDocErr, "cannot be removed") {
		t.Fatalf("Core reason not surfaced: %q", a.designDocErr)
	}
}

func TestStepCommandUndoRedoAcrossDifferentOperations(t *testing.T) {
	a, service := stepEditApp()
	initial := a.designDoc.Source
	if !a.applyDesignRaw(map[string]any{"type": "reorder_steps", "moduleId": "wf", "order": []string{"b", "a"}}) {
		t.Fatal("reorder command failed")
	}
	reordered := a.designDoc.Source
	if !a.applyDesignRaw(map[string]any{"type": "nest_step", "moduleId": "step-b", "parentId": "step-a"}) {
		t.Fatal("nest command failed")
	}
	nested := a.designDoc.Source
	a.designUndo()
	if a.designDoc.Source != reordered {
		t.Fatalf("undo nest = %q, want %q", a.designDoc.Source, reordered)
	}
	a.designUndo()
	if a.designDoc.Source != initial {
		t.Fatalf("undo reorder = %q, want %q", a.designDoc.Source, initial)
	}
	a.designRedo()
	if a.designDoc.Source != reordered {
		t.Fatalf("redo reorder = %q, want %q", a.designDoc.Source, reordered)
	}
	a.designRedo()
	if a.designDoc.Source != nested {
		t.Fatalf("redo nest = %q, want %q", a.designDoc.Source, nested)
	}
	if len(service.saveCalls) != 4 {
		t.Fatalf("undo/redo save calls = %d, want 4", len(service.saveCalls))
	}
}
