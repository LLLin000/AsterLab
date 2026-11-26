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
		Running:        true,
		RawSupported:   true,
		ISO:            200,
		MinISO:         34,
		MaxISO:         3200,
		ShutterS:       0.01,
		ApertureF:      1.78,
		WBKelvin:       4900,
		WBGains:        [3]float64{2.1, 1.0, 1.5},
		MaxWBGain:      8.0,
		FocusLensPos:   0.35,
		PitchDeg:       -6.0,
		RollDeg:        8.5,
		ZoomFactor:     1.0,
		MinZoom:        1.0,
		MaxZoom:        6.0,
		TorchAvailable: true,
		DeviceModel:    "iPhone 16 Pro",
		OSVersion:      "iOS 18.0",
		FocalLengthMM:  6.765,
	}

	var lastCapturedOpts scicamera.CaptureOptions
	sess.captureOverride = func(_ context.Context, opts scicamera.CaptureOptions) (scicamera.CaptureResult, error) {
		lastCapturedOpts = opts
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

	// 1. Minimalist 4-zone Pro Camera HUD (default DNG+JPG, side-view visual level, zoom stops, live Auto readouts)
	a.openSciCamera(nil, curTab, curLD, nil)
	sess.telemetry = liveTelem
	settleIPhoneFrames(tt, 180)
	assertNoHorizontalOverflow402(t, tt, "SciCameraHUDv5")
	maybeSaveIPhoneSnapshot(t, tt, "sci_camera_hud_v5_402x874.png")

	for _, want := range []string{"DNG+JPG", "1.0×", "2×", "3×", "ISO·A", "200", "SS·A", "1/100", "WB·A", "4900K", "LOCK"} {
		if !tt.HasText(want) {
			t.Fatalf("expected %q in Pro Camera HUD", want)
		}
	}

	// Verify DNG+JPG -> DNG -> JPG -> DNG+JPG explicit format cycle
	tt.Click("切换格式")
	settleIPhoneFrames(tt, 100)
	if sess.captureFormat != "dng" || !tt.HasText("DNG") {
		t.Fatalf("expected DNG format after 1st click, got %q", sess.captureFormat)
	}
	tt.Click("切换格式")
	settleIPhoneFrames(tt, 100)
	if sess.captureFormat != "jpg" || !tt.HasText("JPG") {
		t.Fatalf("expected JPG format after 2nd click, got %q", sess.captureFormat)
	}
	tt.Click("切换格式")
	settleIPhoneFrames(tt, 100)
	if sess.captureFormat != "dng+jpg" || !tt.HasText("DNG+JPG") {
		t.Fatalf("expected DNG+JPG format after 3rd click, got %q", sess.captureFormat)
	}

	// Verify Apple-style Zoom Wheel Dial & 2x zoom stop
	tt.Click("缩放 2×")
	settleIPhoneFrames(tt, 120)
	if sess.zoomFactor != 2.0 {
		t.Fatalf("expected 2.0x zoom factor after clicking 2× stop, got %.2f", sess.zoomFactor)
	}
	tt.Click("展开缩放滚轮")
	settleIPhoneFrames(tt, 140)
	assertNoHorizontalOverflow402(t, tt, "SciCameraZoomWheelv5")
	maybeSaveIPhoneSnapshot(t, tt, "sci_camera_zoom_wheel_v5_402x874.png")
	if !tt.HasText("2.0×") || !tt.HasText("收起") {
		t.Fatalf("expected expanded Zoom Wheel Dial with 2.0× readout")
	}
	tt.Click("收起缩放滚轮")
	settleIPhoneFrames(tt, 100)

	// Verify overhead dual-crosshair Apple level when pointing straight down (-86 deg)
	sess.telemetry.PitchDeg = -86.0
	sess.telemetry.RollDeg = 3.2
	settleIPhoneFrames(tt, 120)
	maybeSaveIPhoneSnapshot(t, tt, "sci_camera_overhead_level_v5_402x874.png")

	// Verify practical lab utilities: Torch & Anti-shake Timer (0 -> 2 -> 5 -> 0)
	tt.Click("常亮补光灯")
	settleIPhoneFrames(tt, 100)
	if !sess.torchEnabled {
		t.Fatalf("expected torchEnabled=true after click")
	}
	tt.Click("定时防抖快门")
	settleIPhoneFrames(tt, 100)
	if sess.timerSeconds != 2 || !tt.HasText("2s") {
		t.Fatalf("expected 2s timer preset, got %d", sess.timerSeconds)
	}
	tt.Click("定时防抖快门")
	settleIPhoneFrames(tt, 100)
	if sess.timerSeconds != 5 || !tt.HasText("5s") {
		t.Fatalf("expected 5s timer preset, got %d", sess.timerSeconds)
	}
	tt.Click("定时防抖快门")
	settleIPhoneFrames(tt, 100)
	if sess.timerSeconds != 0 {
		t.Fatalf("expected timer reset to 0s, got %d", sess.timerSeconds)
	}

	// Verify Lock-What-You-See + EV mutual exclusion + capture with CaptureFormat="dng+jpg" & DigitalZoomFactor=2.0
	tt.Click("锁定曝光与白平衡")
	settleIPhoneFrames(tt, 120)
	if !sess.allLocked || sess.selectedISO != 200 || sess.isoAuto || sess.shutterAuto {
		t.Fatalf("expected lock-what-you-see ISO=200, got locked=%v iso=%.0f", sess.allLocked, sess.selectedISO)
	}

	a.triggerSciCapture()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		tt.Advance(20 * time.Millisecond)
		tt.Frame()
		if sess.shotCount == 1 && !sess.busy {
			break
		}
	}
	if sess.shotCount != 1 || lastCapturedOpts.CaptureFormat != "dng+jpg" || !lastCapturedOpts.CaptureRaw {
		t.Fatalf("expected CaptureFormat=dng+jpg archive, got count=%d opts=%+v", sess.shotCount, lastCapturedOpts)
	}
	metaRead, err := scicamera.ReadSidecarJSON(sess.lastMetaAbs)
	if err != nil || metaRead.Geometry.DigitalZoomFactor != 2.0 {
		t.Fatalf("expected DigitalZoomFactor=2.0 in sidecar, got %.2f (err=%v)", metaRead.Geometry.DigitalZoomFactor, err)
	}

	// 2. Flow A: Lab Gallery
	tt.Click("打开科研图库")
	settleIPhoneFrames(tt, 220)
	assertNoHorizontalOverflow402(t, tt, "LabGallery402v5")
	maybeSaveIPhoneSnapshot(t, tt, "lab_gallery_v5_402x874.png")

	for _, want := range []string{"返回拍摄", "科研图库", "Bayer DNG", "用途标注"} {
		if !tt.HasText(want) {
			t.Fatalf("expected %q in Lab Gallery Flow A", want)
		}
	}
}
