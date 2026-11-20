package vault

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const sampleExpSource = `---
asterml: "2.1"
title: CCK-8 药敏测定
scheduled_date: "2026-11-20"
stage: planned
---

` + "```asterlab\nkind: experiment\nid: exp-1\nschema: 1\ntitle: CCK-8 药敏测定\nscheduled_date: 2026-11-20\n```\n\n```asterlab\nkind: step\nid: step-01\nschema: 1\ntitle: 01 细胞接种\nblocks:\n  - kind: checklist\n    id: chk-1\n    schema: 1\n    items:\n      - {id: c1, label: 胰酶消化}\n```\n"

func TestVaultLifecycleFoldersDuplicateTrashUndoAndAssets(t *testing.T) {
	root := t.TempDir()
	v, err := Open(root)
	if err != nil {
		t.Fatalf("Open vault: %v", err)
	}
	for _, dir := range []string{SystemDirName, AssetsDirName, TemplatesDirName} {
		if _, err := os.Stat(filepath.Join(root, dir)); err != nil {
			t.Fatalf("missing vault dir %s: %v", dir, err)
		}
	}

	projRel, err := v.CreateFolder("", "肿瘤耐药课题")
	if err != nil {
		t.Fatalf("CreateFolder: %v", err)
	}
	exp1, err := v.CreateExperiment(projRel, "阿霉素药敏", "2026-11-20", sampleExpSource)
	if err != nil {
		t.Fatalf("CreateExperiment: %v", err)
	}
	if exp1.Date != "2026-11-20" || exp1.Folder != "肿瘤耐药课题" {
		t.Fatalf("unexpected entry: %+v", exp1)
	}

	// Duplicate into same project folder for biological repeat #2
	exp2, err := v.DuplicateFile(exp1.RelPath, "2026-11-22")
	if err != nil {
		t.Fatalf("DuplicateFile: %v", err)
	}
	if exp2.Name != "阿霉素药敏 2" || exp2.Date != "2026-11-22" {
		t.Fatalf("unexpected duplicate entry: %+v", exp2)
	}

	// Save as template clears date & strips execution status
	tmpl, err := v.SaveAsTemplate(exp1.RelPath, "CCK8 标准方案")
	if err != nil {
		t.Fatalf("SaveAsTemplate: %v", err)
	}
	if !tmpl.IsTemplate || tmpl.Date != "" {
		t.Fatalf("unexpected template entry: %+v", tmpl)
	}

	// Scan tree verifies .asterlab hidden and project folder visible
	tree, files, err := v.ScanTree()
	if err != nil {
		t.Fatalf("ScanTree: %v", err)
	}
	if len(files) != 3 {
		t.Fatalf("expected 3 asterml files, got %d", len(files))
	}
	for _, ch := range tree.Children {
		if ch.Name == SystemDirName {
			t.Fatalf(".asterlab should be hidden from file tree")
		}
	}

	// Shared LiveDocument + Undo/Redo
	store := NewDocStore(v)
	ldA, err := store.Open(exp1.RelPath)
	if err != nil {
		t.Fatal(err)
	}
	ldB, err := store.Open(exp1.RelPath)
	if err != nil {
		t.Fatal(err)
	}
	if ldA != ldB {
		t.Fatalf("expected shared LiveDocument pointer across panes")
	}
	orig := ldA.Source
	modified := strings.Replace(orig, "胰酶消化", "Accutase 消化", 1)
	if err := ldA.ApplyEdit(modified); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(ldB.Source, "Accutase 消化") {
		t.Fatalf("pane B did not observe live shared edit")
	}
	if !ldB.Undo() || !strings.Contains(ldA.Source, "胰酶消化") {
		t.Fatalf("Undo did not revert shared LiveDocument")
	}
	if !ldA.Redo() || !strings.Contains(ldB.Source, "Accutase 消化") {
		t.Fatalf("Redo did not restore shared LiveDocument")
	}

	// Content-addressed assets
	assetRef, err := v.ImportAssetBytes([]byte("fake-png-bytes"), "image.png")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(assetRef, "./assets/image_") {
		t.Fatalf("unexpected asset ref: %q", assetRef)
	}

	// Soft trash & restore
	trashItem, err := v.Trash(exp2.RelPath)
	if err != nil {
		t.Fatalf("Trash: %v", err)
	}
	if _, err := os.Stat(exp2.AbsPath); !os.IsNotExist(err) {
		t.Fatalf("trashed file still exists at original path")
	}
	if _, err := v.RestoreFromTrash(trashItem.TrashName); err != nil {
		t.Fatalf("RestoreFromTrash: %v", err)
	}
	if _, err := os.Stat(exp2.AbsPath); err != nil {
		t.Fatalf("restored file missing: %v", err)
	}
}
