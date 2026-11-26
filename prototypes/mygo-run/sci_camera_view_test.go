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
		a.renderSciCameraOverlay(c)
		a.renderLabGalleryOverlay(c)
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

	// Inject deterministic Bayer DNG + preview capture hook.
	sess := a.ensureSciCamera()
	tmpDir := t.TempDir()
	previewSrc := filepath.Join(tmpDir, "temp_preview.jpg")
	rawSrc := filepath.Join(tmpDir, "temp_raw.dng")
	writeSamplePNG(t, previewSrc)
	if err := os.WriteFile(rawSrc, []byte("II*\x00SIMULATED_LINEAR_BAYER_DNG_12BIT"), 0o644); err != nil {
		t.Fatal(err)
	}

	liveTelem := scicamera.Telemetry{
		Running:       true,
		RawSupported:  true,
		ISO:           200,
		MinISO:        34,
		MaxISO:        3200,
		ShutterS:      0.01,
		ApertureF:     1.78,
		WBKelvin:      4900,
		WBGains:       [3]float64{2.1, 1.0, 1.5},
		MaxWBGain:     8.0,
		FocusLensPos:  0.35,
		PitchDeg:      0.3,
		RollDeg:       -0.2,
		DeviceModel:   "iPhone 16 Pro",
		OSVersion:     "iOS 18.0",
		FocalLengthMM: 6.765,
	}

	sess.captureOverride = func(_ context.Context, opts scicamera.CaptureOptions) (scicamera.CaptureResult, error) {
		tOut := liveTelem
		tOut.ExposureLocked = true
		tOut.WBLocked = true
		tOut.FocusLocked = true
		return scicamera.CaptureResult{
			PreviewPath:   previewSrc,
			RawPath:       rawSrc,
			RawFormat:     "bayer_cfa_dng",
			BitDepth:      12,
			AsShotNeutral: [3]float64{0.46, 1.0, 0.62},
			PhotosAssetID: "PH-SIM-001",
			Telemetry:     tOut,
		}, nil
	}

	// 1. Minimalist 4-zone Pro Camera HUD (matching reference dark aesthetic & live Auto readouts)
	a.openSciCamera(nil, curTab, curLD, nil)
	sess.telemetry = liveTelem
	settleIPhoneFrames(tt, 180)
	assertNoHorizontalOverflow402(t, tt, "SciCameraHUDv4")
	maybeSaveIPhoneSnapshot(t, tt, "sci_camera_hud_v4_402x874.png")

	for _, want := range []string{"DNG", "1.0×", "ISO·A", "200", "SS·A", "1/100", "WB·A", "4900K", "LOCK"} {
		if !tt.HasText(want) {
			t.Fatalf("expected %q in minimalist Pro Camera HUD", want)
		}
	}
	if tt.HasText("角色") || tt.HasText("灰卡定白") {
		t.Fatalf("experiment role tags must not appear inside pure Camera HUD")
	}

	// 2. Open "•••" overflow menu -> Switch controls & honest empty-vault Ghost guard
	tt.Click("相机菜单")
	settleIPhoneFrames(tt, 160)
	assertNoHorizontalOverflow402(t, tt, "SciCameraMenuv4")
	maybeSaveIPhoneSnapshot(t, tt, "sci_camera_menu_v4_402x874.png")
	for _, want := range []string{"网格线", "水平仪", "叠影参照", "同步至系统相册 (AsterLab)", "防频闪电网频率"} {
		if !tt.HasText(want) {
			t.Fatalf("expected %q inside camera overflow menu", want)
		}
	}
	// Toggle 叠影参照 on empty vault -> stays false + shows toast cleanly
	tt.Click("叠影参照")
	settleIPhoneFrames(tt, 120)
	if sess.ghostEnabled {
		t.Fatalf("expected ghostEnabled=false when vault has no reference photo yet")
	}
	// Dismiss overflow menu cleanly via tap-outside backdrop
	tt.Click("关闭相机菜单")
	settleIPhoneFrames(tt, 120)
	if sess.settingsOpen {
		t.Fatalf("expected backdrop tap to close camera menu")
	}

	// 3. Open WB drawer (aligned capsule) & verify Lock-What-You-See + EV mutual exclusion
	tt.Click("调节 WB")
	settleIPhoneFrames(tt, 160)
	assertNoHorizontalOverflow402(t, tt, "SciCameraWBv4")
	maybeSaveIPhoneSnapshot(t, tt, "sci_camera_wb_v4_402x874.png")
	tt.Click("5500K")
	settleIPhoneFrames(tt, 120)
	tt.Click("调节 WB")
	settleIPhoneFrames(tt, 100)

	tt.Click("锁定曝光与白平衡")
	settleIPhoneFrames(tt, 120)
	if !sess.allLocked || sess.selectedISO != 200 || sess.isoAuto || sess.shutterAuto {
		t.Fatalf("expected lock-what-you-see ISO=200 (not MinISO=34), got locked=%v iso=%.0f isoAuto=%v",
			sess.allLocked, sess.selectedISO, sess.isoAuto)
	}

	tt.Click("调节 EV")
	settleIPhoneFrames(tt, 120)
	if !tt.HasText("切回自动曝光") {
		t.Fatalf("expected EV drawer under manual exposure to offer 切回自动曝光")
	}
	tt.Click("切回自动曝光")
	settleIPhoneFrames(tt, 120)
	if !sess.isoAuto || !sess.shutterAuto || sess.allLocked {
		t.Fatalf("expected 切回自动曝光 to unlock auto exposure")
	}
	tt.Click("调节 EV")
	settleIPhoneFrames(tt, 100)
	tt.Click("锁定曝光与白平衡")
	settleIPhoneFrames(tt, 120)

	// Capture shot
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

	// 4. Flow A: Tap bottom-left thumbnail -> Lab Gallery floats cleanly on top of Camera
	tt.Click("打开科研图库")
	settleIPhoneFrames(tt, 220)
	assertNoHorizontalOverflow402(t, tt, "LabGallery402v4")
	maybeSaveIPhoneSnapshot(t, tt, "lab_gallery_v4_402x874.png")

	for _, want := range []string{"返回拍摄", "科研图库", "Bayer DNG", "用途标注"} {
		if !tt.HasText(want) {
			t.Fatalf("expected %q in Lab Gallery Flow A", want)
		}
	}

	tt.Click("标注 灰卡")
	settleIPhoneFrames(tt, 160)
	metaRead, err := scicamera.ReadSidecarJSON(sess.lastMetaAbs)
	if err != nil || metaRead.Experiment.FrameRole != scicamera.RoleWBRef {
		t.Fatalf("expected post-capture role persisted as %q, got %q (err=%v)",
			scicamera.RoleWBRef, metaRead.Experiment.FrameRole, err)
	}

	tt.Click("设为叠影并复用参数")
	settleIPhoneFrames(tt, 180)
	if !sess.open || !sess.ghostEnabled || !sess.allLocked || sess.ghostImagePath == "" {
		t.Fatalf("expected ghost reuse to return to locked camera: open=%v ghost=%v locked=%v path=%q",
			sess.open, sess.ghostEnabled, sess.allLocked, sess.ghostImagePath)
	}
}
