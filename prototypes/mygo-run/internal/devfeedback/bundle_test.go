package devfeedback

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func makeSyntheticPNG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	bg := color.RGBA{R: 245, G: 247, B: 250, A: 255}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetRGBA(x, y, bg)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode synthetic png: %v", err)
	}
	return buf.Bytes()
}

func TestAnnotateAndCrop_DPIScalingAndZoom(t *testing.T) {
	// Simulate 150% DPI window: 1000x600 DIP captured at 1500x900 px.
	raw := makeSyntheticPNG(t, 1500, 900)
	fullBytes, cropBytes, imgW, imgH, err := AnnotateAndCrop(
		raw,
		Rect{X: 200, Y: 100, W: 120, H: 40},
		1000, 600,
	)
	if err != nil {
		t.Fatalf("AnnotateAndCrop failed: %v", err)
	}
	if imgW != 1500 || imgH != 900 {
		t.Fatalf("expected 1500x900 image dims, got %dx%d", imgW, imgH)
	}
	fullImg, err := png.Decode(bytes.NewReader(fullBytes))
	if err != nil {
		t.Fatalf("decode full annotated png: %v", err)
	}
	// At 1.5x scale, DIP (200, 100) maps to px (300, 150) which should be painted crisp red (#EF4444).
	r, g, b, _ := fullImg.At(300, 150).RGBA()
	if r>>8 < 220 || g>>8 > 90 || b>>8 > 90 {
		t.Fatalf("expected red highlight pixel at scaled corner (300,150), got RGB=(%d,%d,%d)", r>>8, g>>8, b>>8)
	}
	cropImg, err := png.Decode(bytes.NewReader(cropBytes))
	if err != nil {
		t.Fatalf("decode crop png: %v", err)
	}
	if cropImg.Bounds().Dx() < 200 || cropImg.Bounds().Dy() < 100 {
		t.Fatalf("expected padded 2x zoomed crop, got %dx%d", cropImg.Bounds().Dx(), cropImg.Bounds().Dy())
	}
}

func TestAnnotateAndCrop_ClickPointNearEdgeClampsCleanly(t *testing.T) {
	raw := makeSyntheticPNG(t, 800, 600)
	fullBytes, cropBytes, _, _, err := AnnotateAndCrop(raw, Rect{X: 2, Y: 2, W: 0, H: 0}, 800, 600)
	if err != nil {
		t.Fatalf("click near edge failed: %v", err)
	}
	if len(fullBytes) == 0 || len(cropBytes) == 0 {
		t.Fatalf("expected non-empty annotated images")
	}
}

func TestDetectRegion_ClassifiesWorkbenchZones(t *testing.T) {
	cases := []struct {
		name     string
		x, y     float32
		leftOpen bool
		right    bool
		tabKind  string
		wantKey  string
	}{
		{"left tree", 120, 240, true, false, "file", "left-file-tree"},
		{"top tab strip", 520, 20, true, false, "file", "top-tab-strip"},
		{"doc header", 520, 70, true, false, "file", "doc-header-bar"},
		{"live editor", 520, 320, true, false, "file", "live-asterml-editor"},
		{"right drawer", 1280, 320, true, true, "file", "right-companion-drawer"},
		{"calendar grid", 520, 320, true, false, "calendar", "calendar-grid"},
		{"pantry view", 520, 320, true, false, "pantry", "pantry-workbench"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := DetectRegion(tc.x, tc.y, 1400, 900, 284, 340, tc.leftOpen, tc.right, "design", tc.tabKind)
			if got.Key != tc.wantKey {
				t.Fatalf("got key=%q (%s), want %q", got.Key, got.Label, tc.wantKey)
			}
			if got.SourceHint == "" {
				t.Fatalf("expected non-empty SourceHint for %q", got.Key)
			}
		})
	}
}

func TestExportSession_WritesFolderManifestMarkdownAndZip(t *testing.T) {
	tmp := t.TempDir()
	raw := makeSyntheticPNG(t, 1200, 800)
	fullPNG, cropPNG, w, h, err := AnnotateAndCrop(raw, Rect{X: 300, Y: 160, W: 180, H: 44}, 1200, 800)
	if err != nil {
		t.Fatalf("AnnotateAndCrop: %v", err)
	}

	sess := SessionMeta{
		SessionID:  "20261123-210000",
		AppVersion: "0.1.9-dev",
		Commit:     "bc6ced6",
		OS:         "windows",
		Arch:       "amd64",
		WindowW:    1200,
		WindowH:    800,
		UIScale:    1.0,
		DarkTheme:  false,
		ActiveTab:  "GelMA 水凝胶溶胀实验",
		ActiveFile: "GelMA-Swelling.asterml",
		ActiveMode: "live",
	}
	issues := []Issue{
		{
			ID:               "UI-01",
			CreatedAt:        "2026-11-23T21:00:00+08:00",
			Category:         "文字裁切",
			Severity:         "P2",
			PickKind:         "drag_box",
			RectDIP:          Rect{X: 300, Y: 160, W: 180, H: 44},
			WindowW:          1200,
			WindowH:          800,
			ImageW:           w,
			ImageH:           h,
			Region:           DetectRegion(390, 182, 1200, 800, 284, 0, true, false, "design", "file"),
			PageName:         "design",
			ActiveTab:        sess.ActiveTab,
			ActiveFile:       sess.ActiveFile,
			ActiveMode:       sess.ActiveMode,
			InspectorSnippet: `<Row key="step-pill-1" label="STEP 1"> (internal/asterdoc/live_markdown.go:310)`,
			Suggestion:       "按钮文字右侧略挤，建议左右内边距从 8px 增加到 12px。",
			FullPNG:          fullPNG,
			CropPNG:          cropPNG,
		},
	}

	folder, zipPath, err := ExportSession(tmp, sess, issues, []string{"21:00:00 INFO boot ok"})
	if err != nil {
		t.Fatalf("ExportSession: %v", err)
	}

	mdBytes, err := os.ReadFile(filepath.Join(folder, "feedback.md"))
	if err != nil {
		t.Fatalf("read feedback.md: %v", err)
	}
	md := string(mdBytes)
	for _, want := range []string{"UI-01", "文字裁切", "live_markdown.go", "shots/ui-01-crop.png", "按钮文字右侧略挤"} {
		if !strings.Contains(md, want) {
			t.Fatalf("feedback.md missing %q", want)
		}
	}

	manBytes, err := os.ReadFile(filepath.Join(folder, "manifest.json"))
	if err != nil {
		t.Fatalf("read manifest.json: %v", err)
	}
	var man Manifest
	if err := json.Unmarshal(manBytes, &man); err != nil {
		t.Fatalf("unmarshal manifest.json: %v", err)
	}
	if len(man.Issues) != 1 || man.Issues[0].FullShotFile != "shots/ui-01-full.png" {
		t.Fatalf("unexpected manifest issues: %+v", man.Issues)
	}

	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		t.Fatalf("open exported zip: %v", err)
	}
	defer zr.Close()
	names := map[string]bool{}
	for _, f := range zr.File {
		names[filepath.Base(f.Name)] = true
	}
	for _, want := range []string{"feedback.md", "manifest.json", "ui-01-full.png", "ui-01-crop.png", "recent.log"} {
		if !names[want] {
			t.Fatalf("zip missing %q; got %v", want, names)
		}
	}
}
