package scicamera

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNextLabPhotoIDSequentialAndRoles(t *testing.T) {
	assets := t.TempDir()

	id1 := NextLabPhotoID(assets, "20261126", "GelMA Day3!", RoleSample)
	if id1 != "20261126_GELMADAY3_SMP_001" {
		t.Fatalf("unexpected first id: %q", id1)
	}
	// Create a file for 001 and a sidecar for 002 so next sample should become 003.
	if err := os.WriteFile(filepath.Join(assets, id1+".jpg"), []byte("jpg"), 0o644); err != nil {
		t.Fatal(err)
	}
	id2 := NextLabPhotoID(assets, "20261126", "GelMA Day3!", RoleSample)
	if id2 != "20261126_GELMADAY3_SMP_002" {
		t.Fatalf("unexpected second id: %q", id2)
	}
	if err := os.WriteFile(filepath.Join(assets, id2+".dng"), []byte("dng"), 0o644); err != nil {
		t.Fatal(err)
	}
	id3 := NextLabPhotoID(assets, "20261126", "GelMA Day3!", RoleSample)
	if id3 != "20261126_GELMADAY3_SMP_003" {
		t.Fatalf("unexpected third id: %q", id3)
	}

	ffID := NextLabPhotoID(assets, "20261126", "GelMA Day3!", RoleFlat)
	if ffID != "20261126_GELMADAY3_FLT_003" {
		t.Fatalf("unexpected flat-field id: %q", ffID)
	}
	wbID := NextLabPhotoID(assets, "20261126", "GelMA Day3!", RoleWBRef)
	if wbID != "20261126_GELMADAY3_REF_003" {
		t.Fatalf("unexpected wb-ref id: %q", wbID)
	}
	scID := NextLabPhotoID(assets, "20261126", "GelMA Day3!", RoleScaleRef)
	if scID != "20261126_GELMADAY3_SCL_003" {
		t.Fatalf("unexpected scale-ref id: %q", scID)
	}
}

func TestSaveCapturedTrioScanGalleryAndExportBatch(t *testing.T) {
	tmp := t.TempDir()
	assets := filepath.Join(tmp, "assets")
	if err := os.MkdirAll(assets, 0o755); err != nil {
		t.Fatal(err)
	}

	previewSrc := filepath.Join(tmp, "preview.jpg")
	rawSrc := filepath.Join(tmp, "raw.dng")
	if err := os.WriteFile(previewSrc, []byte("FAKE_SRGB_JPEG_BYTES"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(rawSrc, []byte("II*\x00LINEAR_BAYER_DNG_BYTES"), 0o644); err != nil {
		t.Fatal(err)
	}

	wantSHA, err := ComputeFileSHA256(rawSrc)
	if err != nil {
		t.Fatal(err)
	}

	meta := PhotoMetadata{
		Schema:        MetadataSchema,
		PhotosAssetID: "PH-LOCAL-001",
		Raw: RawInfo{
			Format:           "bayer_cfa_dng",
			BitDepth:         12,
			SmartHDRDisabled: true,
			MultiFrameFusion: false,
			AsShotNeutral:    [3]float64{0.46, 1.0, 0.62},
		},
		Exposure: ExposureInfo{
			ISO:                50,
			DeviceMinNativeISO: 34,
			ShutterS:           0.01, // 1/100s
			ApertureF:          1.78,
			FlickerGridHz:      50,
			ExposureMode:       "batch_locked",
			WBMode:             "kelvin",
			WBKelvinEst:        5200,
			WBRGBGains:         [3]float64{2.1, 1.0, 1.5},
		},
		Geometry: GeometryInfo{
			DeviceModel:       "iPhone 15 Pro",
			FocusMode:         "locked",
			FocusLensPosition: 0.35,
			IMUPitchDeg:       0.4,
			IMURollDeg:        -0.3,
		},
		Calibration: CalibrationInfo{
			ColorSpacePreview: "sRGB IEC61966-2.1",
		},
		Experiment: ExperimentInfo{
			ExperimentID: "Exp-GelMA",
			BatchID:      "BATCH-20261126-01",
			FrameRole:    RoleSample,
			CapturedAt:   "2026-11-26T14:00:00Z",
		},
	}

	id := NextLabPhotoID(assets, "20261126", "Exp-GelMA", RoleSample)
	arch, err := SaveCapturedTrio(assets, id, previewSrc, rawSrc, meta)
	if err != nil {
		t.Fatalf("SaveCapturedTrio: %v", err)
	}
	if arch.Meta.Raw.RawSHA256 != wantSHA {
		t.Fatalf("RawSHA256 mismatch: got %s want %s", arch.Meta.Raw.RawSHA256, wantSHA)
	}
	if arch.PreviewRel != "./assets/"+id+".jpg" || arch.RawRel != "./assets/"+id+".dng" {
		t.Fatalf("unexpected rel paths: preview=%q raw=%q", arch.PreviewRel, arch.RawRel)
	}

	items := ScanLabGallery(assets)
	if len(items) != 1 {
		t.Fatalf("expected 1 gallery item, got %d", len(items))
	}
	if items[0].LabPhotoID != id || !items[0].HasRaw || items[0].Meta.PhotosAssetID != "PH-LOCAL-001" {
		t.Fatalf("unexpected scanned gallery item: %+v", items[0])
	}

	exportDir := filepath.Join(tmp, "export")
	rep, err := ExportBatchBundle(assets, "BATCH-20261126-01", exportDir)
	if err != nil {
		t.Fatalf("ExportBatchBundle: %v", err)
	}
	if rep.ShotCount != 1 {
		t.Fatalf("expected 1 exported photo, got %d", rep.ShotCount)
	}
	csvBytes, err := os.ReadFile(rep.CSVPath)
	if err != nil {
		t.Fatalf("read CSV: %v", err)
	}
	csvText := string(csvBytes)
	if !strings.Contains(csvText, id) || !strings.Contains(csvText, wantSHA) || !strings.Contains(csvText, "5200") {
		t.Fatalf("CSV missing expected calibration columns:\n%s", csvText)
	}
	if len(rep.ExportedFiles) != 4 { // jpg + dng + asterphoto.json + csv
		t.Fatalf("expected 4 exported files (trio + csv), got %d: %v", len(rep.ExportedFiles), rep.ExportedFiles)
	}
}

func TestAntiFlickerShutterPresets50HzVs60Hz(t *testing.T) {
	p50 := AntiFlickerShutterPresets(50)
	p60 := AntiFlickerShutterPresets(60)
	if len(p50) < 3 || len(p60) < 3 {
		t.Fatalf("expected >= 3 presets each")
	}
	if p50[0].Label != "自动" || p50[1].Label != "1/50s" || p50[2].Label != "1/100s" ||
		p60[0].Label != "自动" || p60[1].Label != "1/60s" || p60[2].Label != "1/120s" {
		t.Fatalf("unexpected anti-flicker labels: 50Hz=%+v 60Hz=%+v", p50, p60)
	}
}
