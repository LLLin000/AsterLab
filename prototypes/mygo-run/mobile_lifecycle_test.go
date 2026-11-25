package main

import (
	"context"
	"encoding/json"
	"errors"
	"image/png"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"asterlab-mygo-prototype/internal/asterdoc"
	"asterlab-mygo-prototype/internal/shell"
	"asterlab-mygo-prototype/internal/vault"
	"asterlab-mygo-prototype/internal/workflowtree"
	"asterlab-mygo-prototype/modules/runs"
	"github.com/egoist/mygo/ui/editor"
)

func TestMobileReleaseMemoryCachesDropsInactiveEntries(t *testing.T) {
	s := &shell.Shell{}
	s.EnsureTabs()
	s.OpenFileInPane(s.ActivePaneID, "proj/active.asterml", "Active", shell.ModeLab, false)

	a := &app{
		shell: s,
		designCache: map[string]runs.AsterDocSnapshot{
			"d1": {DesignID: "d1"},
		},
		blockPreviewCache: map[string]workflowtree.BlockDefinition{
			"b1": {},
		},
		cachedAssays: map[string]cachedAssayResult{
			"a1": {},
		},
		secondaryViews: map[string]*asterdoc.View{
			"sec1": {},
		},
		liveViews: map[string]*asterdoc.View{
			"proj/active.asterml":   {},
			"proj/inactive.asterml": {},
		},
		liveEditors: map[string]*editor.State{
			"proj/active.asterml":   {},
			"proj/inactive.asterml": {},
		},
	}

	a.releaseMemoryCaches()

	if len(a.designCache) != 0 {
		t.Fatalf("expected designCache cleared, got %d", len(a.designCache))
	}
	if len(a.blockPreviewCache) != 0 {
		t.Fatalf("expected blockPreviewCache cleared, got %d", len(a.blockPreviewCache))
	}
	if len(a.cachedAssays) != 0 {
		t.Fatalf("expected cachedAssays cleared, got %d", len(a.cachedAssays))
	}
	if len(a.secondaryViews) != 0 {
		t.Fatalf("expected secondaryViews cleared, got %d", len(a.secondaryViews))
	}
	if _, ok := a.liveViews["proj/active.asterml"]; !ok {
		t.Fatal("expected active liveView to be retained")
	}
	if _, ok := a.liveViews["proj/inactive.asterml"]; ok {
		t.Fatal("expected inactive liveView to be evicted")
	}
	if _, ok := a.liveEditors["proj/active.asterml"]; !ok {
		t.Fatal("expected active liveEditor to be retained")
	}
	if _, ok := a.liveEditors["proj/inactive.asterml"]; ok {
		t.Fatal("expected inactive liveEditor to be evicted")
	}
}

func TestMobileLifecycleSyncAndRestoreRoundTrip(t *testing.T) {
	s := &shell.Shell{
		Page:         shell.Design,
		SelectedDate: "2026-11-26",
		VaultFiles: []vault.FileEntry{
			{RelPath: "proj/exp1.asterml", Title: "Exp 1"},
		},
	}
	s.EnsureTabs()
	s.OpenFileInPane(s.ActivePaneID, "proj/exp1.asterml", "Exp 1", shell.ModeLab, false)
	s.MobileContentScrollState().Y = 148.5

	src := &app{
		shell:          s,
		scrollToStepID: "step-3",
		selectedTask:   "task-2",
		snap: runs.RunSnapshot{
			DesignID: "des-1",
			RunID:    "run-1",
		},
	}
	src.syncMobileLifecycleState()
	saved := src.mobileState

	// Simulate fresh restart with restored state before opening the file.
	s2 := &shell.Shell{
		VaultFiles: []vault.FileEntry{
			{RelPath: "proj/exp1.asterml", Title: "Exp 1"},
		},
	}
	s2.EnsureTabs()
	dst := &app{
		shell:       s2,
		mobileState: saved,
	}
	dst.applyRestoredMobileState()

	if got := s2.SelectedDate; got != "2026-11-26" {
		t.Fatalf("SelectedDate = %q, want 2026-11-26", got)
	}
	tab := s2.ActiveTab()
	if tab == nil || tab.FilePath != "proj/exp1.asterml" || tab.Mode != shell.ModeLab {
		t.Fatalf("restored active tab = %+v, want proj/exp1.asterml ModeLab", tab)
	}
	if dst.scrollToStepID != "step-3" || dst.selectedTask != "task-2" {
		t.Fatalf("restored step/task = (%q, %q)", dst.scrollToStepID, dst.selectedTask)
	}
	if got := s2.MobileContentScrollState().Y; got != 148.5 {
		t.Fatalf("restored MobileScrollY = %v, want 148.5", got)
	}
}

func TestShareValidationAndDesktopImagePickerUnsupported(t *testing.T) {
	if err := ShareFile("  ", "caption"); err == nil {
		t.Fatal("expected ShareFile to reject empty path")
	}
	if err := ShareText("", ""); err == nil {
		t.Fatal("expected ShareText to reject empty text and url")
	}
	if err := ShareFile("sample.pdf", "caption"); !errors.Is(err, errors.ErrUnsupported) {
		t.Fatalf("desktop ShareFile = %v, want ErrUnsupported", err)
	}
	if _, err := CaptureLabPhoto(context.Background()); !errors.Is(err, errors.ErrUnsupported) {
		t.Fatalf("desktop CaptureLabPhoto = %v, want ErrUnsupported", err)
	}
	if _, err := PickLabPhotos(context.Background(), 2); !errors.Is(err, errors.ErrUnsupported) {
		t.Fatalf("desktop PickLabPhotos = %v, want ErrUnsupported", err)
	}
}

func TestMyGoJSONConfigAndMobileAssets(t *testing.T) {
	raw, err := os.ReadFile("mygo.json")
	if err != nil {
		t.Fatalf("read mygo.json: %v", err)
	}
	var cfg struct {
		Name       string `json:"name"`
		Identifier string `json:"identifier"`
		Icon       string `json:"icon"`
		Android    struct {
			Permissions  []string `json:"permissions"`
			Splash       string   `json:"splash"`
			AdaptiveIcon struct {
				Foreground string `json:"foreground"`
				Background string `json:"background"`
			} `json:"adaptiveIcon"`
		} `json:"android"`
		IOS struct {
			LaunchScreen struct {
				Background string `json:"background"`
				Image      string `json:"image"`
			} `json:"launchScreen"`
			InfoPlist map[string]any `json:"infoPlist"`
		} `json:"ios"`
	}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		t.Fatalf("parse mygo.json: %v", err)
	}
	if cfg.Name != "AsterLab" {
		t.Fatalf("name = %q, want AsterLab", cfg.Name)
	}
	if cfg.Identifier != "party.asterlab.app" {
		t.Fatalf("identifier = %q, want party.asterlab.app", cfg.Identifier)
	}
	for _, forbidden := range []string{"MANAGE_EXTERNAL_STORAGE", "android.permission.MANAGE_EXTERNAL_STORAGE"} {
		if slices.Contains(cfg.Android.Permissions, forbidden) {
			t.Fatalf("forbidden permission %q present in android.permissions", forbidden)
		}
	}
	for _, req := range []string{"CAMERA", "VIBRATE", "INTERNET"} {
		if !slices.Contains(cfg.Android.Permissions, req) {
			t.Fatalf("missing required permission %q in android.permissions", req)
		}
	}
	for _, key := range []string{
		"NSCameraUsageDescription",
		"NSPhotoLibraryUsageDescription",
		"UIFileSharingEnabled",
		"LSSupportsOpeningDocumentsInPlace",
		"NSAppTransportSecurity",
		"ITSAppUsesNonExemptEncryption",
	} {
		if _, ok := cfg.IOS.InfoPlist[key]; !ok {
			t.Fatalf("missing ios.infoPlist key %q", key)
		}
	}
	for _, imgPath := range []string{
		cfg.Icon,
		cfg.Android.AdaptiveIcon.Foreground,
		cfg.IOS.LaunchScreen.Image,
	} {
		f, err := os.Open(filepath.FromSlash(imgPath))
		if err != nil {
			t.Fatalf("open asset %q: %v", imgPath, err)
		}
		meta, err := png.DecodeConfig(f)
		_ = f.Close()
		if err != nil {
			t.Fatalf("decode PNG %q: %v", imgPath, err)
		}
		if meta.Width != meta.Height || meta.Width < 400 {
			t.Fatalf("asset %q unexpected dimensions %dx%d", imgPath, meta.Width, meta.Height)
		}
	}
}
