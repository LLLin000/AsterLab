package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"asterlab-mygo-prototype/modules/runs"
)

// Core must reject an invalid plate assignment even when the GUI never saw it.
func TestCoreRejectsInvalidWellAtRunCreation(t *testing.T) {
	copyFixtureDB(t)
	ctx := context.Background()
	svc, err := runs.NewDesignService()
	if err != nil {
		t.Fatal(err)
	}
	defer svc.Close()
	designs, err := svc.ListDesigns(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var designID string
	for _, design := range designs {
		snap, err := svc.LoadDesign(ctx, design.DesignID)
		if err != nil {
			continue
		}
		var doc struct {
			Assays []struct {
				ID string `json:"id"`
			} `json:"assays"`
		}
		if json.Unmarshal([]byte(snap.Data), &doc) != nil || len(doc.Assays) == 0 {
			continue
		}
		designID = design.DesignID
		break
	}
	if designID == "" {
		t.Skip("fixture has no design")
	}
	cur, err := svc.LoadDesign(ctx, designID)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal([]byte(cur.Data), &doc); err != nil {
		t.Fatal(err)
	}
	mygo, _ := doc["mygo"].(map[string]any)
	if mygo == nil {
		mygo = map[string]any{}
	}
	mygo["assays"] = map[string]any{
		"cck8": map[string]any{
			"timepoints": []any{"D1"},
			"wells": []any{map[string]any{
				"wellId": "Z99", "sampleId": "Control-1", "groupId": "g1",
				"replicate": 1, "timepoint": "D1",
			}},
		},
	}
	doc["mygo"] = mygo
	raw, _ := json.Marshal(mygo)
	title, _ := json.Marshal(cur.Title)
	if _, err := svc.SaveDesign(ctx, runs.DesignSaveInput{
		DesignID: designID, ExpectedRevision: cur.Revision,
		Changes:     fmt.Sprintf(`{"title":%s,"mygo":%s}`, title, raw),
		OperationID: "audit-invalid-well",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.CreateRun(ctx, designID, "audit-invalid-run"); err == nil {
		t.Fatal("Core created a Run with an invalid well")
	} else if !strings.Contains(err.Error(), "孔位 Z99 无效") {
		t.Fatalf("unexpected rejection: %v", err)
	}
}
