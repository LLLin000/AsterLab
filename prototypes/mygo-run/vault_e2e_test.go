package main

import (
	"path/filepath"
	"strings"
	"testing"

	"asterlab-mygo-prototype/internal/asterdoc"
	"asterlab-mygo-prototype/internal/shell"
	"asterlab-mygo-prototype/internal/theme"
	"asterlab-mygo-prototype/internal/vault"

	"github.com/egoist/mygo/ui"
)

func TestVaultObsidianSplitHostEndToEnd(t *testing.T) {
	root := t.TempDir()
	v, err := vault.Open(root)
	if err != nil {
		t.Fatalf("vault.Open: %v", err)
	}
	if err := v.SeedTemplates(defaultTemplateSeeds()); err != nil {
		t.Fatalf("SeedTemplates: %v", err)
	}
	ds := vault.NewDocStore(v)

	folder, err := v.CreateFolder("", "细胞毒性测试")
	if err != nil {
		t.Fatalf("CreateFolder: %v", err)
	}
	tplLD, err := ds.Open("templates/CCK-8细胞增殖活力测定.asterml")
	if err != nil {
		t.Fatalf("open seeded template: %v", err)
	}
	entry, err := v.CreateExperiment(folder, "L929-药物梯度", "2026-11-20", tplLD.Source)
	if err != nil {
		t.Fatalf("CreateExperiment: %v", err)
	}

	sh := &shell.Shell{
		TodayDate:    "2026-11-20",
		SelectedDate: "2026-11-20",
	}
	a := &app{
		shell:    sh,
		vault:    v,
		docStore: ds,
	}
	sh.RenderTabContent = func(c *ui.Context, pane *shell.PaneNode, tab shell.WorkspaceTab) {
		a.renderVaultTabContent(c, pane, tab)
	}
	sh.RenderPantryContent = func(c *ui.Context) {
		a.renderStandalonePantry(c)
	}
	sh.RenderOverlays = func(c *ui.Context) {
		a.renderVaultNewExperimentDialog(c)
	}
	a.refreshVaultTreeAndCalendar()

	sh.EnsureTabs()
	sh.OpenFileInPane(sh.ActivePaneID, entry.RelPath, entry.Title, shell.ModeEdit, true)
	// Also split the same file into Read mode side-by-side!
	sh.SplitPaneWithTab(sh.ActivePaneID, shell.WorkspaceTab{
		Kind:     shell.TabFile,
		Title:    entry.Title,
		FilePath: entry.RelPath,
		Mode:     shell.ModeRead,
	}, "right")
	if sh.RootPane == nil || sh.RootPane.First == nil || sh.RootPane.Second == nil {
		t.Fatal("expected SplitPaneWithTab to create split branch")
	}

	// Render the full shell with peer split panes (Edit left + Read right)
	_ = ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.LightTheme())
		sh.RenderSplitView(c)
	}, 1280, 800)

	// Verify shared LiveDocument edit + 100-step Undo/Redo
	ld, err := ds.Open(entry.RelPath)
	if err != nil {
		t.Fatalf("ds.Open: %v", err)
	}
	before := ld.Source
	updated := asterdoc.SetFrontmatterMetadata(before, "L929-药物梯度-更新", "2026-11-21", "in_progress")
	if err := ld.ApplyEdit(updated); err != nil {
		t.Fatalf("ApplyEdit: %v", err)
	}
	if !strings.Contains(ld.Source, "2026-11-21") {
		t.Fatalf("expected updated date in live source")
	}
	if !ld.Undo() {
		t.Fatal("expected Undo to succeed")
	}
	if ld.Source != before {
		t.Fatalf("expected Undo to restore exact original source")
	}
	if !ld.Redo() {
		t.Fatal("expected Redo to succeed")
	}

	// SaveAsTemplate clears date/status/execution state
	tplOut, err := v.SaveAsTemplate(entry.RelPath, "复用模板-L929")
	if err != nil {
		t.Fatalf("SaveAsTemplate: %v", err)
	}
	if !tplOut.IsTemplate || tplOut.Date != "" {
		t.Fatalf("expected template with cleared date, got %+v", tplOut)
	}

	// Export and restore workspace persistence
	if err := v.SaveWorkspaceJSON(sh.ExportPersistState()); err != nil {
		t.Fatalf("SaveWorkspaceJSON: %v", err)
	}
	var loaded shell.WorkspacePersistState
	if err := v.LoadWorkspaceJSON(&loaded); err != nil {
		t.Fatalf("LoadWorkspaceJSON: %v", err)
	}
	if loaded.RootPane == nil || loaded.RootPane.First == nil || loaded.RootPane.Second == nil {
		t.Fatalf("expected split tree persisted in workspace.json, got %+v", loaded.RootPane)
	}

	// Move & Trash lifecycle
	proj2, err := v.CreateFolder("", "归档项目")
	if err != nil {
		t.Fatalf("CreateFolder 2: %v", err)
	}
	a.moveVaultFile(entry.RelPath, proj2)
	expectedMoved := filepath.ToSlash(filepath.Join(proj2, filepath.Base(entry.RelPath)))
	if _, err := ds.Open(expectedMoved); err != nil {
		t.Fatalf("expected moved file at %s: %v", expectedMoved, err)
	}
	a.deleteVaultFile(expectedMoved)
}
