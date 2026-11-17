package main

import (
	"context"
	"os"
	"testing"

	"asterlab-mygo-prototype/modules/runs"
)

func TestCreateAsterDocDesignCreatesAuthoritativeDocument(t *testing.T) {
	copyFixtureDB(t)
	ctx := context.Background()
	svc, err := runs.NewDesignService()
	if err != nil {
		t.Fatal(err)
	}
	defer svc.Close()
	projects, err := svc.ListProjects(ctx)
	if err != nil || len(projects) == 0 {
		t.Fatalf("projects: %v %v", projects, err)
	}
	raw, err := os.ReadFile("starter.asterml")
	if err != nil {
		t.Fatal(err)
	}
	doc, err := svc.CreateAsterDocDesign(ctx, runs.AsterDocCreateInput{
		ProjectID: projects[0].ProjectID, Title: "新实验", Source: string(raw),
		OperationID: "create-asterdoc-design", ActorID: "tester",
	})
	if err != nil {
		t.Fatalf("CreateAsterDocDesign: %v", err)
	}
	if doc.DesignID == "" || doc.Revision != 1 || doc.Source != string(raw) {
		t.Fatalf("document = %+v", doc)
	}
	design, err := svc.LoadDesign(ctx, doc.DesignID)
	if err != nil {
		t.Fatal(err)
	}
	if design.Title != "新实验" {
		t.Fatalf("design title = %q", design.Title)
	}
	loaded, err := svc.LoadAsterDocAt(ctx, doc.DesignID, 1)
	if err != nil || loaded.Source != string(raw) {
		t.Fatalf("loaded doc = %+v, %v", loaded, err)
	}
}

func TestAssessAsterDocReportsIncompleteStarter(t *testing.T) {
	copyFixtureDB(t)
	ctx := context.Background()
	svc, err := runs.NewDesignService()
	if err != nil {
		t.Fatal(err)
	}
	defer svc.Close()
	projects, err := svc.ListProjects(ctx)
	if err != nil || len(projects) == 0 {
		t.Fatalf("projects: %v %v", projects, err)
	}
	stepless := "```asterlab\nkind: experiment\nid: exp-empty\nschema: 1\ntitle: 无步骤草稿\n```\n" +
		"```asterlab\nkind: group\nid: g-c\nschema: 1\nname: 对照\nrole: control\n```\n" +
		"```asterlab\nkind: assay\nid: assay-main\nschema: 1\ntitle: 检测\ngroup_refs: [g-c]\ntimepoints: [D1]\n```\n"
	doc, err := svc.CreateAsterDocDesign(ctx, runs.AsterDocCreateInput{
		ProjectID: projects[0].ProjectID, Title: "无步骤草稿", Source: stepless,
		OperationID: "assess-create",
	})
	if err != nil {
		t.Fatal(err)
	}
	assessment, err := svc.AssessAsterDoc(ctx, doc.DesignID, doc.Revision)
	if err != nil {
		t.Fatal(err)
	}
	if assessment.State != "incomplete" || len(assessment.Errors) == 0 {
		t.Fatalf("assessment = %+v", assessment)
	}
}

func TestProtocolBindingMakesStarterReadinessExecutable(t *testing.T) {
	copyFixtureDB(t)
	ctx := context.Background()
	svc, err := runs.NewDesignService()
	if err != nil {
		t.Fatal(err)
	}
	defer svc.Close()
	projects, err := svc.ListProjects(ctx)
	if err != nil || len(projects) == 0 {
		t.Fatalf("projects: %v %v", projects, err)
	}
	raw, err := os.ReadFile("starter.asterml")
	if err != nil {
		t.Fatal(err)
	}
	doc, err := svc.CreateAsterDocDesign(ctx, runs.AsterDocCreateInput{
		ProjectID: projects[0].ProjectID, Title: "新实验", Source: string(raw),
		OperationID: "assess-self-contained-create",
	})
	if err != nil {
		t.Fatal(err)
	}
	assessment, err := svc.AssessAsterDoc(ctx, doc.DesignID, doc.Revision)
	if err != nil {
		t.Fatal(err)
	}
	if len(assessment.Errors) != 0 {
		t.Fatalf("self-contained v2 starter readiness has errors: %+v", assessment)
	}
}
