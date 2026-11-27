package main

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"asterlab-mygo-prototype/internal/scicamera"
	"asterlab-mygo-prototype/internal/shell"
	"asterlab-mygo-prototype/internal/theme"
)

func imageMeanLuma(img image.Image) float64 {
	if img == nil {
		return 255
	}
	b := img.Bounds()
	var sum float64
	var count int
	for y := b.Min.Y; y < b.Max.Y; y += 2 {
		for x := b.Min.X; x < b.Max.X; x += 2 {
			r, g, bl, _ := img.At(x, y).RGBA()
			lum := 0.2126*float64(r>>8) + 0.7152*float64(g>>8) + 0.0722*float64(bl>>8)
			sum += lum
			count++
		}
	}
	if count == 0 {
		return 255
	}
	return sum / float64(count)
}

func writeSpecimenPNG(t *testing.T, path string, rBase, gBase, bBase uint8) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 128, 128))
	for y := 0; y < 128; y++ {
		for x := 0; x < 128; x++ {
			dx := float64(x-64) / 48.0
			dy := float64(y-64) / 48.0
			dist := dx*dx + dy*dy
			glow := 0.25
			if dist < 0.7 {
				glow = 1.0 - dist*0.8
			}
			r := uint8(float64(rBase) * glow)
			g := uint8(float64(gBase) * glow)
			b := uint8(float64(bBase) * glow)
			img.Set(x, y, color.RGBA{R: r, G: g, B: b, A: 255})
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

func writeSamplePNG(t *testing.T, path string) {
	writeSpecimenPNG(t, path, 70, 165, 210)
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

	// 2. Flow A: Contact-Sheet Lab Gallery + DarkroomLightbox
	tt.Click("打开科研图库")
	settleIPhoneFrames(tt, 260)
	assertNoHorizontalOverflow402(t, tt, "LabGallery402v5")
	maybeSaveIPhoneSnapshot(t, tt, "lab_gallery_v5_402x874.png")

	for _, want := range []string{"返回拍摄", "科研图库", "RAW+JPG", "1 张"} {
		if !tt.HasText(want) {
			t.Fatalf("expected %q in Contact-Sheet Lab Gallery, texts=%v", want, tt.Texts())
		}
	}

	// Tap captured tile -> opens DarkroomLightbox directly over Lab Gallery
	g := a.ensureLabGallery()
	if len(g.items) == 0 {
		t.Fatalf("expected captured item in lab gallery")
	}
	capturedID := g.items[0].LabPhotoID
	if err := tt.Click(capturedID); err != nil {
		t.Fatalf("click tile %s: %v", capturedID, err)
	}
	settleIPhoneFrames(tt, 260)
	assertNoHorizontalOverflow402(t, tt, "LabGalleryLightbox402")
	for _, want := range []string{"100%", "ISO 200", "4900K", "用途标注", "插入当前实验", "导出原片 (.DNG)", "设为叠影参考"} {
		if !tt.HasText(want) {
			t.Fatalf("expected %q in DarkroomLightbox, texts=%v", want, tt.Texts())
		}
	}
	if err := tt.Click("标注 灰卡"); err != nil {
		t.Fatalf("click role pill 灰卡: %v", err)
	}
	settleIPhoneFrames(tt, 120)
	if !tt.HasText("灰卡") {
		t.Fatalf("expected updated role 灰卡 in lightbox, texts=%v", tt.Texts())
	}
	if err := tt.Click("设为叠影参考"); err != nil {
		t.Fatalf("click 设为叠影参考: %v", err)
	}
	settleIPhoneFrames(tt, 140)
	if !sess.ghostEnabled || sess.ghostImagePath == "" || g.open {
		t.Fatalf("expected ghost reference applied and returned to camera, ghost=%v path=%q galOpen=%v", sess.ghostEnabled, sess.ghostImagePath, g.open)
	}
}

func TestScientificCameraLabGalleryContactSheetAndLightbox402x874(t *testing.T) {
	vaultRoot := filepath.Join(t.TempDir(), "LabGalleryDarkroomVault")
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

	expPath := "HUVEC-Tube.asterml"
	if err := os.WriteFile(filepath.Join(vaultRoot, expPath), []byte("---\ntitle: HUVEC-Tube\n---\n\n# HUVEC-Tube\n\n## 步骤 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	curTab := shell.WorkspaceTab{Kind: shell.TabFile, FilePath: expPath, Title: "HUVEC-Tube"}
	curLD, _ := a.docStore.Open(curTab.FilePath)
	sh.OpenFileInPane("", curTab.FilePath, curTab.Title, shell.ModeEdit, false)

	// Seed multi-batch lab photos with real PNG thumbnails + DNG/JPG variants.
	tmpDir := t.TempDir()
	assetsDir := a.vault.AssetsDir()
	imgCyan := filepath.Join(tmpDir, "cyan.png")
	imgAmber := filepath.Join(tmpDir, "amber.png")
	imgGreen := filepath.Join(tmpDir, "green.png")
	rawDummy := filepath.Join(tmpDir, "dummy.dng")
	writeSpecimenPNG(t, imgCyan, 60, 210, 195)
	writeSpecimenPNG(t, imgAmber, 225, 155, 45)
	writeSpecimenPNG(t, imgGreen, 75, 220, 120)
	if err := os.WriteFile(rawDummy, []byte("II*\x00SIMULATED_BAYER_DNG"), 0o644); err != nil {
		t.Fatal(err)
	}

	seedPhoto := func(id, batchID, role, capturedAt, previewPath, rawPath string, iso, shutter, kelvin, focus, pitch, roll float64) {
		t.Helper()
		meta := scicamera.PhotoMetadata{
			Schema:     scicamera.MetadataSchema,
			LabPhotoID: id,
			Raw: scicamera.RawInfo{
				Format:   "bayer_cfa_dng",
				BitDepth: 12,
			},
			Exposure: scicamera.ExposureInfo{
				ISO:         iso,
				ShutterS:    shutter,
				WBKelvinEst: kelvin,
			},
			Geometry: scicamera.GeometryInfo{
				FocusLensPosition: focus,
				IMUPitchDeg:       pitch,
				IMURollDeg:        roll,
			},
			Experiment: scicamera.ExperimentInfo{
				ExperimentID: "HUVECTUBE",
				BatchID:      batchID,
				FrameRole:    role,
				CapturedAt:   capturedAt,
			},
		}
		if _, err := scicamera.SaveCapturedTrio(assetsDir, id, previewPath, rawPath, meta); err != nil {
			t.Fatalf("seed photo %s: %v", id, err)
		}
	}

	seedPhoto("20261127_HUVECTUBE_SMP_001", "B20261127-01", scicamera.RoleSample, "2026-11-27T14:22:00+08:00", imgCyan, rawDummy, 400, 1.0/125.0, 5600, 0.42, -1.2, 0.4)
	seedPhoto("20261127_HUVECTUBE_REF_002", "B20261127-01", scicamera.RoleWBRef, "2026-11-27T14:21:00+08:00", imgAmber, rawDummy, 200, 1.0/100.0, 5200, 0.42, -0.5, 0.2)
	seedPhoto("20261127_HUVECTUBE_SMP_003", "B20261127-01", scicamera.RoleSample, "2026-11-27T14:20:00+08:00", "", rawDummy, 100, 1.0/250.0, 5400, 0.40, 0.0, 0.0)
	seedPhoto("20261126_HUVECTUBE_SMP_001", "B20261126-02", scicamera.RoleSample, "2026-11-26T18:05:00+08:00", imgGreen, "", 800, 1.0/60.0, 4800, 0.38, -2.0, 1.1)

	// Render inside LightTheme to verify Camera -> Lab Gallery has ZERO white flash.
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.LightTheme())
		sh.RenderSplitView(c)
	}, 402, 874)
	tt.SetInsets(59, 0, 34, 0)
	settleIPhoneFrames(tt, 180)

	a.openSciCamera(nil, curTab, curLD, nil)
	settleIPhoneFrames(tt, 160)
	if luma := imageMeanLuma(tt.Image()); luma >= 60 {
		t.Fatalf("expected darkroom camera canvas in LightTheme (mean_luma < 60), got %.1f", luma)
	}

	if err := tt.Click("打开科研图库"); err != nil {
		t.Fatalf("click 打开科研图库: %v", err)
	}
	// Check immediate frame AND settled frames for zero white flash
	tt.Frame()
	if luma0 := imageMeanLuma(tt.Image()); luma0 >= 60 {
		t.Fatalf("white flash detected on Camera -> Lab Gallery frame 0: mean_luma=%.1f (want < 60)", luma0)
	}
	settleIPhoneFrames(tt, 260)
	if lumaSettled := imageMeanLuma(tt.Image()); lumaSettled >= 60 {
		t.Fatalf("expected settled Lab Gallery darkroom canvas mean_luma < 60 in LightTheme, got %.1f", lumaSettled)
	}
	assertNoHorizontalOverflow402(t, tt, "LabGalleryContactSheet402")
	maybeSaveIPhoneSnapshot(t, tt, "lab_gallery_contact_sheet_402x874.png")

	for _, want := range []string{
		"科研图库", "4 组原片 · DNG 3", "全部", "DNG 原片",
		"2026-11-27 · B20261127-01", "3 张",
		"2026-11-26 · B20261126-02", "1 张",
		"RAW+JPG", "灰卡 · RAW+JPG", "DNG", "JPG",
	} {
		if !tt.HasText(want) {
			t.Fatalf("expected %q in contact-sheet gallery, texts=%v", want, tt.Texts())
		}
	}

	// Verify filter pill: DNG 原片 hides JPG-only item
	if err := tt.Click("筛选 DNG 原片"); err != nil {
		t.Fatalf("click DNG filter: %v", err)
	}
	settleIPhoneFrames(tt, 120)
	if tt.HasText("2026-11-26 · B20261126-02") {
		t.Fatalf("expected JPG-only batch hidden when filtering DNG 原片, texts=%v", tt.Texts())
	}
	if err := tt.Click("筛选 全部"); err != nil {
		t.Fatalf("click 全部 filter: %v", err)
	}
	settleIPhoneFrames(tt, 120)

	// Tap first tile -> opens DarkroomLightbox directly over Lab Gallery
	if err := tt.Click("20261127_HUVECTUBE_SMP_001"); err != nil {
		t.Fatalf("click contact sheet tile: %v", err)
	}
	settleIPhoneFrames(tt, 260)
	if lumaLB := imageMeanLuma(tt.Image()); lumaLB >= 60 {
		t.Fatalf("expected DarkroomLightbox mean_luma < 60, got %.1f", lumaLB)
	}
	assertNoHorizontalOverflow402(t, tt, "LabGalleryDarkroomLightbox402")
	maybeSaveIPhoneSnapshot(t, tt, "lab_gallery_darkroom_lightbox_402x874.png")

	for _, want := range []string{
		"20261127_HUVECTUBE_SMP_001", "100%",
		"ISO 400 · 1/125 · 5600K · F0.42 · -1.2°/0.4°",
		"用途标注", "插入当前实验", "导出原片 (.DNG)", "设为叠影参考",
	} {
		if !tt.HasText(want) {
			t.Fatalf("expected %q in DarkroomLightbox, texts=%v", want, tt.Texts())
		}
	}

	// Single-tap stage toggles chrome hidden, tap again restores chrome
	g := a.ensureLabGallery()
	tt.ClickAt(201, 420)
	tt.Advance(200 * time.Millisecond)
	tt.Frame()
	if !g.lightboxState.ChromeHidden {
		t.Fatalf("expected single tap on lightbox stage to hide chrome")
	}
	tt.ClickAt(201, 420)
	tt.Advance(200 * time.Millisecond)
	tt.Frame()
	if g.lightboxState.ChromeHidden {
		t.Fatalf("expected second tap on lightbox stage to restore chrome")
	}

	// Double-click toggles 1.0x <-> 2.0x zoom
	tt.ClickAt(201, 420)
	tt.Advance(40 * time.Millisecond)
	tt.ClickAt(201, 420)
	tt.Frame()
	if g.lightboxState.Zoom != 2.0 || !tt.HasText("200%") {
		t.Fatalf("expected double-click 2.0x zoom, got %.2f texts=%v", g.lightboxState.Zoom, tt.Texts())
	}
	tt.Advance(600 * time.Millisecond)
	tt.ClickAt(201, 420)
	tt.Advance(40 * time.Millisecond)
	tt.ClickAt(201, 420)
	tt.Frame()
	if g.lightboxState.Zoom != 1.0 {
		t.Fatalf("expected second double-click back to 1.0x, got %.2f", g.lightboxState.Zoom)
	}

	// Role tagging updates metadata & badge
	if err := tt.Click("标注 平场"); err != nil {
		t.Fatalf("click role pill 平场: %v", err)
	}
	settleIPhoneFrames(tt, 120)
	if !tt.HasText("平场") {
		t.Fatalf("expected role updated to 平场")
	}

	// Insert into active experiment document
	if err := tt.Click("插入当前实验"); err != nil {
		t.Fatalf("click 插入当前实验: %v", err)
	}
	settleIPhoneFrames(tt, 160)
	if g.open {
		t.Fatalf("expected gallery closed after inserting photo into document")
	}
	updatedLD, err := a.docStore.Open(expPath)
	if err != nil {
		t.Fatalf("reopen doc: %v", err)
	}
	if !strings.Contains(updatedLD.Source, "20261127_HUVECTUBE_SMP_001") {
		t.Fatalf("expected inserted photo reference in doc, got:\n%s", updatedLD.Source)
	}
}
