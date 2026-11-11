package main

import (
	"context"
	"testing"

	"asterlab-mygo-prototype/modules/runs"
)

func TestAssetLinkSurvivesServiceReload(t *testing.T) {
	copyFixtureDB(t)
	ctx := context.Background()
	svc, err := runs.NewRunService()
	if err != nil {
		t.Fatal(err)
	}
	run, err := svc.LoadRun(ctx, contractRunID)
	if err != nil {
		t.Fatal(err)
	}
	if len(run.Tasks) == 0 {
		t.Fatal("run has no tasks")
	}
	taskKey := run.Tasks[0].Key
	rootID := "project-data"
	relPath := "huvec/d1/image_01.tif"
	asset := runs.Asset{
		RootID:       rootID,
		RelativePath: relPath,
		MediaType:    "image",
		Size:         12,
		ModTime:      "2026-01-01T00:00:00Z",
		SHA256:       "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Availability: "available",
		Link:         runs.AssetLink{RunID: run.RunID, TaskKey: taskKey, SampleID: "sample-1"},
		LinkedAt:     "2026-01-01T00:00:00Z",
		ActorID:      "tester",
	}
	saved, err := svc.SaveAssetLink(ctx, runs.AssetLinkInput{Asset: asset})
	if err != nil {
		t.Fatal(err)
	}
	if saved.RootID != rootID || saved.RelativePath != relPath {
		t.Fatalf("saved identity = %s/%s", saved.RootID, saved.RelativePath)
	}
	if saved.Link.TaskKey != taskKey || saved.Link.SampleID != "sample-1" {
		t.Fatalf("saved association = %+v", saved.Link)
	}
	svc.Close()

	reopened, err := runs.NewRunService()
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	links, err := reopened.ListAssetLinks(ctx, run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, l := range links {
		if l.RootID == rootID && l.RelativePath == relPath {
			found = true
			if l.Link.RunID != run.RunID || l.Link.TaskKey != taskKey || l.Link.SampleID != "sample-1" {
				t.Fatalf("reloaded association = %+v", l.Link)
			}
			if l.ID != saved.ID {
				t.Fatalf("id changed: %s vs %s", l.ID, saved.ID)
			}
		}
	}
	if !found {
		t.Fatal("asset link did not survive reload")
	}
}
