package main

import (
	"encoding/json"
	"testing"

	"asterlab-mygo-prototype/internal/workflow"
)

func TestWorkspaceSessionCanvasLayoutRoundTrip(t *testing.T) {
	session := workspaceSession{
		DesignID: "design-a",
		CanvasLayouts: map[string]map[string]workflow.Position{
			"design-a": {"step-1": {X: 12, Y: 34}},
			"design-b": {"step-2": {X: 56, Y: 78}},
		},
	}
	raw, err := json.Marshal(session)
	if err != nil {
		t.Fatal(err)
	}
	var got workspaceSession
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if pos := got.CanvasLayouts["design-a"]["step-1"]; pos != (workflow.Position{X: 12, Y: 34}) {
		t.Fatalf("design-a position = %+v", pos)
	}
	if pos := got.CanvasLayouts["design-b"]["step-2"]; pos != (workflow.Position{X: 56, Y: 78}) {
		t.Fatalf("design-b position = %+v", pos)
	}
	if _, ok := got.CanvasLayouts["design-a"]["step-2"]; ok {
		t.Fatal("layouts leaked across designs")
	}
}