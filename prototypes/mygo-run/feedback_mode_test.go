package main

import (
	"path/filepath"
	"testing"

	"asterlab-mygo-prototype/internal/shell"
	"asterlab-mygo-prototype/internal/vault"

	"github.com/egoist/mygo/ui"
)

func TestBuiltinFeedbackModeToggleAndVaultPersistence(t *testing.T) {
	t.Setenv("LABWORKBENCH_DATA_DIR", t.TempDir())
	root := filepath.Join(t.TempDir(), "FeedbackVault")
	sh := &shell.Shell{TodayDate: "2026-11-24", SelectedDate: "2026-11-24"}
	a := &app{shell: sh}
	if err := a.initVaultWorkspace(root); err != nil {
		t.Fatalf("initVaultWorkspace: %v", err)
	}

	// Default off: zero floating pill rendered and zero side effects.
	if a.isFeedbackModeActive() || sh.FeedbackMode {
		t.Fatalf("expected FeedbackMode default false")
	}
	ttOff := ui.NewTester(func(c *ui.Context) {
		a.renderDevInspectorOverlay(c)
	}, 1280, 800)
	ttOff.Frame()
	if ttOff.HasText("反馈模式") || ttOff.HasText("拾取细节 (Ctrl+Shift+D)") {
		t.Fatalf("expected no floating feedback pill when FeedbackMode is off, texts=%v", ttOff.Texts())
	}

	// Turn on: pill renders and closes cleanly via exit button, persisting to vault Config.
	a.setFeedbackMode(true)
	if !a.isFeedbackModeActive() || !sh.FeedbackMode {
		t.Fatalf("expected FeedbackMode active after setFeedbackMode(true)")
	}
	if !a.vault.Config.FeedbackMode {
		t.Fatalf("expected vault.Config.FeedbackMode persisted as true")
	}

	reopened, err := vault.Open(root)
	if err != nil {
		t.Fatalf("vault.Open: %v", err)
	}
	if !reopened.Config.FeedbackMode {
		t.Fatalf("expected reopened vault Config.FeedbackMode true")
	}

	ttOn := ui.NewTester(func(c *ui.Context) {
		a.renderDevInspectorOverlay(c)
	}, 1280, 800)
	ttOn.Frame()
	for _, want := range []string{"反馈模式", "拾取细节 (Ctrl+Shift+D)", "队列 (0)"} {
		if !ttOn.HasText(want) {
			t.Fatalf("expected floating feedback pill text %q, got %v", want, ttOn.Texts())
		}
	}

	if err := ttOn.Click("退出反馈模式"); err != nil {
		t.Fatalf("click exit feedback mode: %v", err)
	}
	ttOn.Frame()
	if a.isFeedbackModeActive() || sh.FeedbackMode {
		t.Fatalf("expected FeedbackMode turned off after clicking exit button")
	}
	if ttOn.HasText("拾取细节 (Ctrl+Shift+D)") {
		t.Fatalf("expected floating pill hidden after exit, texts=%v", ttOn.Texts())
	}
}
