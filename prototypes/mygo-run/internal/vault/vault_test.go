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

func TestNormalizeFolderPathAndNestedCreation(t *testing.T) {
	cases := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{in: "", want: ""},
		{in: "  /  ", want: ""},
		{in: "Fig4-动物实验", want: "Fig4-动物实验"},
		{in: "Fig4/子目录/第3轮", want: "Fig4/子目录/第3轮"},
		{in: `Fig4\WinSub\Run1`, want: "Fig4/WinSub/Run1"},
		{in: "../escape", wantErr: true},
		{in: "Fig4/..", wantErr: true},
		{in: "Fig4//Sub", wantErr: true},
		{in: ".asterlab/bad", wantErr: true},
		{in: "templates/custom", wantErr: true},
	}
	for _, tc := range cases {
		got, err := NormalizeFolderPath(tc.in)
		if tc.wantErr {
			if err == nil {
				t.Fatalf("NormalizeFolderPath(%q): expected error, got %q", tc.in, got)
			}
			continue
		}
		if err != nil {
			t.Fatalf("NormalizeFolderPath(%q): unexpected error: %v", tc.in, err)
		}
		if got != tc.want {
			t.Fatalf("NormalizeFolderPath(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}

	root := t.TempDir()
	v, err := Open(root)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	nested, _ := NormalizeFolderPath("Fig4-动物实验/第1批/Day7")
	entry, err := v.CreateExperiment(nested, "嵌套实验", "2026-11-25", "title: 嵌套实验\n")
	if err != nil {
		t.Fatalf("CreateExperiment nested: %v", err)
	}
	if entry.Folder != "Fig4-动物实验/第1批/Day7" {
		t.Fatalf("expected nested folder Fig4-动物实验/第1批/Day7, got %q", entry.Folder)
	}
	if _, err := os.Stat(entry.AbsPath); err != nil {
		t.Fatalf("expected nested file on disk: %v", err)
	}
}

func TestVaultRelatedDocsAndHybridSearch(t *testing.T) {
	root := t.TempDir()
	v, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	doc1 := "---\ntitle: 万古霉素抑菌圈第2轮\nscheduled_date: 2026-11-20\ntags: [抗菌, 水凝胶]\n---\n\n#@ 贴片抑菌\n- 用 GelMA 水凝胶装载万古霉素并测量抑菌圈直径\n"
	doc2 := "---\ntitle: 万古霉素液体共培养抑菌测试\nscheduled_date: 2026-11-22\ntags: [抗菌, 水凝胶]\n---\n\n#@ 共培养计数\n- 使用 GelMA 水凝胶对比革兰氏阳性菌抑制率\n"
	doc3 := "---\ntitle: 小鼠免疫组化染色切片\nscheduled_date: 2026-11-23\n---\n\n#@ DAPI 封片\n- 荧光显微镜拍摄血管切片\n"
	_, err = v.CreateExperiment("抗菌", "万古霉素抑菌圈第2轮", "2026-11-20", doc1)
	if err != nil {
		t.Fatal(err)
	}
	_, err = v.CreateExperiment("抗菌", "万古霉素液体共培养抑菌测试", "2026-11-22", doc2)
	if err != nil {
		t.Fatal(err)
	}
	_, err = v.CreateExperiment("免疫组化", "小鼠免疫组化染色切片", "2026-11-23", doc3)
	if err != nil {
		t.Fatal(err)
	}

	idx := NewVaultIndex(v)
	if err := idx.Refresh(); err != nil {
		t.Fatal(err)
	}

	related := idx.RelatedDocs("抗菌/万古霉素抑菌圈第2轮.asterml", 3)
	if len(related) == 0 || related[0].RelPath != "抗菌/万古霉素液体共培养抑菌测试.asterml" {
		t.Fatalf("expected 万古霉素液体共培养抑菌测试 as top related doc, got %+v", related)
	}

	// Query phrasing doesn't exist verbatim as substring, yet HybridSearch recalls via semantic similarity
	hits := idx.HybridSearch("水凝胶抑菌测试", 10)
	if len(hits) == 0 {
		t.Fatalf("expected HybridSearch to recall related hydrogel antibacterial docs")
	}
}
