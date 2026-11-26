package main

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"asterlab-mygo-prototype/internal/scicamera"
	"asterlab-mygo-prototype/internal/shell"
	"asterlab-mygo-prototype/internal/theme"
)

func writeSamplePNG(t *testing.T, path string) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 64, 48))
	for y := 0; y < 48; y++ {
		for x := 0; x < 64; x++ {
			img.Set(x, y, color.RGBA{R: 70, G: 165, B: 210, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode sample png: %v", err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatalf("write sample png %s: %v", path, err)
	}
}

func TestScientificCameraHUDCaptureAndLabGallery402x874(t *testing.T) {
	vaultRoot := filepath.Join(t.TempDir(), "SciCameraVault")
	a, sh := newWiredFSMTestApp(t, vaultRoot)
	sh.ForceMobilePreview = true

	origOverlays := sh.RenderOverlays
	sh.RenderOverlays = func(c *ui.Context) {
		if origOverlays != nil {
			origOverlays(c)
		}
		a.renderLabGalleryOverlay(c)
		a.renderSciCameraOverlay(c)
	}
	sh.OnOpenLabGallery = func() {
		a.openLabGallery(sh.ActiveTab(), nil)
	}
	sh.IsExternalModalOpen = func() bool {
		return a.isExternalModalOrGalleryOpen()
	}

	expPath := filepath.Join(vaultRoot, "GelMA-Day3.asterml")
	curTab := shell.WorkspaceTab{Kind: shell.TabFile, FilePath: expPath, Title: "GelMA-Day3"}
	curLD, _ := a.docStore.Open(curTab.FilePath)

	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.DarkTheme())
		sh.RenderSplitView(c)
	}, 402, 874)
	tt.SetInsets(59, 0, 34, 0)
	settleIPhoneFrames(tt, 200)

	// Open Scientific Camera overlay and inject deterministic Bayer DNG + preview capture hook.
	sess := a.ensureSciCamera()
	tmpDir := t.TempDir()
	previewSrc := filepath.Join(tmpDir, "temp_preview.jpg")
	rawSrc := filepath.Join(tmpDir, "temp_raw.dng")
	writeSamplePNG(t, previewSrc)
	if err := os.WriteFile(rawSrc, []byte("II*\x00SIMULATED_LINEAR_BAYER_DNG_12BIT"), 0o644); err != nil {
		t.Fatal(err)
	}

	sess.captureOverride = func(_ context.Context, opts scicamera.CaptureOptions) (scicamera.CaptureResult, error) {
		return scicamera.CaptureResult{
			PreviewPath:   previewSrc,
			RawPath:       rawSrc,
			RawFormat:     "bayer_cfa_dng",
			BitDepth:      12,
			AsShotNeutral: [3]float64{0.46, 1.0, 0.62},
			PhotosAssetID: "PH-SIM-001",
			Telemetry: scicamera.Telemetry{
				Running:        true,
				RawSupported:   true,
				ISO:            50,
				MinISO:         34,
				MaxISO:         3200,
				ShutterS:       0.01,
				ApertureF:      1.78,
				WBKelvin:       5200,
				WBGains:        [3]float64{2.1, 1.0, 1.5},
				MaxWBGain:      8.0,
				FocusLensPos:   0.35,
				PitchDeg:       0.3,
				RollDeg:        -0.2,
				ExposureLocked: true,
				WBLocked:       true,
				FocusLocked:    true,
				DeviceModel:    "iPhone 16 Pro",
				OSVersion:      "iOS 18.0",
				FocalLengthMM:  6.765,
			},
		}, nil
	}

	a.openSciCamera(nil, curTab, curLD, nil)
	a.toggleSciBatchLock()
	settleIPhoneFrames(tt, 180)
	assertNoHorizontalOverflow402(t, tt, "SciCameraHUD")
	maybeSaveIPhoneSnapshot(t, tt, "sci_camera_hud_402x874.png")

	if !tt.HasText("科研取景器（连接物理主摄后实时显示）") || !tt.HasText("已锁定") {
		t.Fatalf("expected Scientific Camera HUD viewfinder and batch lock control")
	}

	// Trigger capture and drain async captureCh via frames.
	a.triggerSciCapture()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		tt.Advance(20 * time.Millisecond)
		tt.Frame()
		if sess.shotCount == 1 && !sess.busy {
			break
		}
	}
	if sess.shotCount != 1 || sess.lastRawAbs == "" || sess.lastPreviewAbs == "" || sess.lastMetaAbs == "" {
		t.Fatalf("expected archived trio after capture, got count=%d raw=%q preview=%q meta=%q",
			sess.shotCount, sess.lastRawAbs, sess.lastPreviewAbs, sess.lastMetaAbs)
	}
	if _, err := os.Stat(sess.lastRawAbs); err != nil {
		t.Fatalf("expected archived DNG on disk: %v", err)
	}
	if _, err := os.Stat(sess.lastMetaAbs); err != nil {
		t.Fatalf("expected archived sidecar JSON on disk: %v", err)
	}

	// Open In-App Lab Gallery and inspect metadata + role filter + ghost reuse.
	a.closeSciCamera()
	a.openLabGallery(&curTab, nil)
	settleIPhoneFrames(tt, 220)
	assertNoHorizontalOverflow402(t, tt, "LabGallery402")
	maybeSaveIPhoneSnapshot(t, tt, "lab_gallery_402x874.png")

	if !tt.HasText("科研图库") || !tt.HasText("Bayer DNG") {
		t.Fatalf("expected Lab Gallery heading and Bayer DNG badge")
	}
	g := a.ensureLabGallery()
	if len(g.items) != 1 {
		t.Fatalf("expected 1 item in Lab Gallery, got %d", len(g.items))
	}

	// Filter to flat_field -> list becomes empty, then back to 全部 -> restores item.
	tt.Click("筛选 平场")
	settleIPhoneFrames(tt, 120)
	if g.filterRole != scicamera.RoleFlat {
		t.Fatalf("expected filterRole=%q, got %q", scicamera.RoleFlat, g.filterRole)
	}
	tt.Click("筛选 全部")
	settleIPhoneFrames(tt, 120)
	if g.filterRole != "" {
		t.Fatalf("expected filterRole cleared, got %q", g.filterRole)
	}

	// Clicking 设为叠影并复用参数 re-opens Scientific Camera with ghost enabled and batch locked.
	tt.Click("设为叠影并复用参数")
	settleIPhoneFrames(tt, 180)
	if !sess.open || !sess.ghostEnabled || !sess.batchLocked || sess.ghostImagePath == "" {
		t.Fatalf("expected ghost reuse to reopen camera locked: open=%v ghost=%v locked=%v path=%q",
			sess.open, sess.ghostEnabled, sess.batchLocked, sess.ghostImagePath)
	}
}
