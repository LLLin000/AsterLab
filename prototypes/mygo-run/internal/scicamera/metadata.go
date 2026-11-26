package scicamera

import (
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

const MetadataSchema = "asterphoto/1"

// Frame role constants for wet-lab scientific photography.
const (
	RoleSample   = "sample"
	RoleFlat     = "flat_field"
	RoleWBRef    = "wb_reference"
	RoleScaleRef = "scale_bar_ref"
)

// PhotoMetadata captures the 5-layer linear DNG & experimental provenance sidecar
// (`<LabPhotoID>.asterphoto.json`) co-archived with every captured `.dng` + `.jpg`.
type PhotoMetadata struct {
	Schema        string          `json:"schema"`
	LabPhotoID    string          `json:"lab_photo_id"`
	PhotosAssetID string          `json:"photos_asset_id,omitempty"`
	Raw           RawInfo         `json:"raw"`
	Exposure      ExposureInfo    `json:"exposure"`
	Geometry      GeometryInfo    `json:"geometry"`
	Calibration   CalibrationInfo `json:"calibration"`
	Experiment    ExperimentInfo  `json:"experiment"`
}

type RawInfo struct {
	Format           string     `json:"format"` // bayer_cfa_dng | processed_jpeg_sim | processed_jpeg_android
	BitDepth         int        `json:"bit_depth"`
	SmartHDRDisabled bool       `json:"smart_hdr_disabled"`
	MultiFrameFusion bool       `json:"multi_frame_fusion"`
	RawSHA256        string     `json:"raw_sha256,omitempty"`
	AsShotNeutral    [3]float64 `json:"as_shot_neutral,omitempty"`
}

type ExposureInfo struct {
	ISO                float64    `json:"iso"`
	DeviceMinNativeISO float64    `json:"device_min_native_iso"`
	ShutterS           float64    `json:"shutter_s"`
	ApertureF          float64    `json:"aperture_f"`
	EVBias             float64    `json:"ev_bias"`
	FlickerGridHz      int        `json:"flicker_grid_hz"`
	ExposureMode       string     `json:"exposure_mode"` // auto | locked | batch_locked
	WBMode             string     `json:"wb_mode"`       // auto | locked_gains | kelvin
	WBKelvinEst        float64    `json:"wb_kelvin_est"`
	WBTintEst          float64    `json:"wb_tint_est"`
	WBRGBGains         [3]float64 `json:"wb_rgb_gains"`
}

type GeometryInfo struct {
	DeviceModel       string   `json:"device_model,omitempty"`
	OSVersion         string   `json:"os_version,omitempty"`
	PhysicalCameraID  string   `json:"physical_camera_id,omitempty"`
	FocalLengthMM     float64  `json:"focal_length_mm,omitempty"`
	FocalLength35mmEq float64  `json:"focal_length_35mm_eq,omitempty"`
	FocusMode         string   `json:"focus_mode"`
	FocusLensPosition float64  `json:"focus_lens_position"`
	IMUPitchDeg       float64  `json:"imu_pitch_deg"`
	IMURollDeg        float64  `json:"imu_roll_deg"`
	IMUYawDeg         float64  `json:"imu_yaw_deg"`
	ScaleMMPerPx      *float64 `json:"scale_mm_per_px,omitempty"`
}

type CalibrationInfo struct {
	LightingPreset      string     `json:"lighting_preset,omitempty"`
	ReferenceTarget     string     `json:"reference_target,omitempty"` // none | neutral_gray_patch_18pct | colorchecker_24
	ReferenceROINorm    [4]float64 `json:"reference_roi_norm,omitempty"`
	ReferenceLinearRGB  [3]float64 `json:"reference_linear_rgb,omitempty"`
	FlatFieldAssetID    string     `json:"flat_field_asset_id,omitempty"`
	ColorSpacePreview   string     `json:"color_space_preview"`
	HighlightClippedPct float64    `json:"highlight_clipped_pct"`
	BlurLaplacianScore  float64    `json:"blur_laplacian_score"`
}

type ExperimentInfo struct {
	ExperimentID      string   `json:"experiment_id,omitempty"`
	StepID            string   `json:"step_id,omitempty"`
	BatchID           string   `json:"batch_id,omitempty"`
	SlotLabel         string   `json:"slot_label,omitempty"`
	Timepoint         string   `json:"timepoint,omitempty"`
	FrameRole         string   `json:"frame_role"` // sample | flat_field | wb_reference | scale_bar_ref
	BatchRefAssetID   string   `json:"batch_ref_asset_id,omitempty"`
	CapturedAt        string   `json:"captured_at"`
	NonDestructiveOps []string `json:"non_destructive_ops"`
}

// ArchivedPhoto holds resolved Vault paths and metadata after archiving a shot.
type ArchivedPhoto struct {
	LabPhotoID string
	PreviewRel string
	PreviewAbs string
	RawRel     string
	RawAbs     string
	MetaRel    string
	MetaAbs    string
	Meta       PhotoMetadata
}

// GalleryItem summarizes a vault lab photo for fast in-app Lab Gallery rendering.
type GalleryItem struct {
	LabPhotoID string
	PreviewRel string
	PreviewAbs string
	RawRel     string
	RawAbs     string
	HasRaw     bool
	Meta       PhotoMetadata
}

// ShutterPreset represents an anti-flicker shutter option aligned to local AC power frequency.
type ShutterPreset struct {
	Label    string
	Seconds  float64
	AutoMode bool
}

// AntiFlickerShutterPresets returns shutter speeds aligned with integer multiples of 50Hz or 60Hz AC lighting.
func AntiFlickerShutterPresets(gridHz int) []ShutterPreset {
	if gridHz == 60 {
		return []ShutterPreset{
			{Label: "自动", AutoMode: true},
			{Label: "1/60s", Seconds: 1.0 / 60.0},
			{Label: "1/120s", Seconds: 1.0 / 120.0},
			{Label: "1/240s", Seconds: 1.0 / 240.0},
			{Label: "1/500s", Seconds: 1.0 / 500.0},
		}
	}
	return []ShutterPreset{
		{Label: "自动", AutoMode: true},
		{Label: "1/50s", Seconds: 1.0 / 50.0},
		{Label: "1/100s", Seconds: 1.0 / 100.0},
		{Label: "1/200s", Seconds: 1.0 / 200.0},
		{Label: "1/400s", Seconds: 1.0 / 400.0},
	}
}

// WBPreset represents a reproducible laboratory lighting Kelvin/Tint anchor.
type WBPreset struct {
	Label    string
	Mode     string // auto | lock_current | kelvin
	Kelvin   float64
	Tint     float64
}

// DefaultWBPresets returns lab lighting presets (standard 5000K D50 benchtop, 5500K, 6500K hood).
func DefaultWBPresets() []WBPreset {
	return []WBPreset{
		{Label: "AWB", Mode: "auto"},
		{Label: "定白锁定", Mode: "lock_current"},
		{Label: "4500K", Mode: "kelvin", Kelvin: 4500, Tint: 0},
		{Label: "5000K", Mode: "kelvin", Kelvin: 5000, Tint: 0},
		{Label: "5500K", Mode: "kelvin", Kelvin: 5500, Tint: 0},
		{Label: "6500K", Mode: "kelvin", Kelvin: 6500, Tint: 0},
	}
}

// RoleCode maps a human frame role into its 3-letter canonical filename token.
func RoleCode(role string) string {
	switch strings.TrimSpace(role) {
	case RoleFlat:
		return "FLT"
	case RoleWBRef:
		return "REF"
	case RoleScaleRef:
		return "SCL"
	default:
		return "SMP"
	}
}

// RoleLabel returns the concise Chinese UI badge label for a frame role.
func RoleLabel(role string) string {
	switch strings.TrimSpace(role) {
	case RoleFlat:
		return "平场"
	case RoleWBRef:
		return "灰卡"
	case RoleScaleRef:
		return "标尺"
	default:
		return "样本"
	}
}

var nonTagRe = regexp.MustCompile(`[^A-Za-z0-9]+`)

// SanitizeExpTag extracts an ASCII uppercase experiment code safe for DNG/Lightroom/ImageJ filenames.
func SanitizeExpTag(raw string) string {
	cleaned := nonTagRe.ReplaceAllString(strings.ToUpper(strings.TrimSpace(raw)), "")
	if cleaned == "" {
		return "LAB"
	}
	if len(cleaned) > 12 {
		cleaned = cleaned[:12]
	}
	return cleaned
}

var seqTailRe = regexp.MustCompile(`_(\d{3,5})(?:\.[^.]+)?$`)

// NextLabPhotoID returns the next canonical lab photo ID (<YYYYMMDD>_<TAG>_<ROLE>_<NNN>)
// guaranteed not to collide with any existing `.jpg`, `.dng`, or `.asterphoto.json` in assetsDir.
func NextLabPhotoID(assetsDir, dateYMD, expTag, role string) string {
	compactDate := strings.ReplaceAll(strings.TrimSpace(dateYMD), "-", "")
	if len(compactDate) != 8 {
		compactDate = time.Now().Format("20060102")
	}
	tag := SanitizeExpTag(expTag)
	code := RoleCode(role)
	prefix := compactDate + "_" + tag + "_" + code + "_"

	maxSeq := 0
	if entries, err := os.ReadDir(assetsDir); err == nil {
		for _, ent := range entries {
			name := ent.Name()
			if !strings.HasPrefix(name, compactDate+"_"+tag+"_") {
				continue
			}
			stem := strings.TrimSuffix(name, ".asterphoto.json")
			stem = strings.TrimSuffix(stem, filepath.Ext(stem))
			if m := seqTailRe.FindStringSubmatch(stem); len(m) == 2 {
				if n, convErr := strconv.Atoi(m[1]); convErr == nil && n > maxSeq {
					maxSeq = n
				}
			}
		}
	}
	for seq := maxSeq + 1; ; seq++ {
		id := fmt.Sprintf("%s%03d", prefix, seq)
		if _, err := os.Stat(filepath.Join(assetsDir, id+".jpg")); os.IsNotExist(err) {
			if _, err2 := os.Stat(filepath.Join(assetsDir, id+".dng")); os.IsNotExist(err2) {
				if _, err3 := os.Stat(filepath.Join(assetsDir, id+".asterphoto.json")); os.IsNotExist(err3) {
					return id
				}
			}
		}
	}
}

// ComputeFileSHA256 computes lowercase hex SHA-256 of path via streaming I/O.
func ComputeFileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func copyFileAtomic(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	tmp := dst + ".tmp"
	out, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		_ = os.Remove(tmp)
		return err
	}
	if err := out.Close(); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, dst)
}

// WriteSidecarJSON writes meta atomically to path.
func WriteSidecarJSON(path string, meta PhotoMetadata) error {
	if meta.Schema == "" {
		meta.Schema = MetadataSchema
	}
	if meta.Experiment.NonDestructiveOps == nil {
		meta.Experiment.NonDestructiveOps = []string{}
	}
	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// ReadSidecarJSON reads a `.asterphoto.json` sidecar from path.
func ReadSidecarJSON(path string) (PhotoMetadata, error) {
	var meta PhotoMetadata
	data, err := os.ReadFile(path)
	if err != nil {
		return meta, err
	}
	err = json.Unmarshal(data, &meta)
	return meta, err
}

// SaveCapturedTrio stores `<labPhotoID>.jpg`, optional `<labPhotoID>.dng` (hashing raw bytes),
// and `<labPhotoID>.asterphoto.json` inside assetsDir.
func SaveCapturedTrio(assetsDir, labPhotoID, previewSrcPath, rawSrcPath string, meta PhotoMetadata) (ArchivedPhoto, error) {
	var out ArchivedPhoto
	if strings.TrimSpace(assetsDir) == "" {
		return out, fmt.Errorf("assetsDir is required")
	}
	if strings.TrimSpace(labPhotoID) == "" {
		labPhotoID = NextLabPhotoID(assetsDir, "", "", meta.Experiment.FrameRole)
	}
	if err := os.MkdirAll(assetsDir, 0o755); err != nil {
		return out, err
	}

	meta.Schema = MetadataSchema
	meta.LabPhotoID = labPhotoID
	if meta.Experiment.CapturedAt == "" {
		meta.Experiment.CapturedAt = time.Now().Format(time.RFC3339)
	}
	if meta.Calibration.ColorSpacePreview == "" {
		meta.Calibration.ColorSpacePreview = "sRGB"
	}

	out.LabPhotoID = labPhotoID
	if strings.TrimSpace(previewSrcPath) != "" {
		ext := strings.ToLower(filepath.Ext(previewSrcPath))
		if ext == "" {
			ext = ".jpg"
		}
		name := labPhotoID + ext
		dst := filepath.Join(assetsDir, name)
		if err := copyFileAtomic(previewSrcPath, dst); err != nil {
			return out, err
		}
		out.PreviewRel = "./assets/" + name
		out.PreviewAbs = dst
	}

	if strings.TrimSpace(rawSrcPath) != "" {
		if _, err := os.Stat(rawSrcPath); err == nil {
			rawName := labPhotoID + ".dng"
			rawDst := filepath.Join(assetsDir, rawName)
			if err := copyFileAtomic(rawSrcPath, rawDst); err != nil {
				return out, err
			}
			sum, err := ComputeFileSHA256(rawDst)
			if err == nil {
				meta.Raw.RawSHA256 = sum
			}
			out.RawRel = "./assets/" + rawName
			out.RawAbs = rawDst
		}
	}

	metaName := labPhotoID + ".asterphoto.json"
	metaDst := filepath.Join(assetsDir, metaName)
	if err := WriteSidecarJSON(metaDst, meta); err != nil {
		return out, err
	}
	out.MetaRel = "./assets/" + metaName
	out.MetaAbs = metaDst
	out.Meta = meta
	return out, nil
}

// ScanLabGallery enumerates all captured `.asterphoto.json` items in assetsDir newest-first.
func ScanLabGallery(assetsDir string) []GalleryItem {
	entries, err := os.ReadDir(assetsDir)
	if err != nil {
		return nil
	}
	items := make([]GalleryItem, 0, len(entries)/2)
	for _, ent := range entries {
		name := ent.Name()
		if ent.IsDir() || !strings.HasSuffix(name, ".asterphoto.json") {
			continue
		}
		metaAbs := filepath.Join(assetsDir, name)
		meta, err := ReadSidecarJSON(metaAbs)
		if err != nil {
			continue
		}
		id := strings.TrimSuffix(name, ".asterphoto.json")
		if meta.LabPhotoID != "" {
			id = meta.LabPhotoID
		}
		item := GalleryItem{
			LabPhotoID: id,
			Meta:       meta,
		}
		for _, ext := range []string{".jpg", ".jpeg", ".png"} {
			cand := filepath.Join(assetsDir, id+ext)
			if _, sErr := os.Stat(cand); sErr == nil {
				item.PreviewAbs = cand
				item.PreviewRel = "./assets/" + id + ext
				break
			}
		}
		rawCand := filepath.Join(assetsDir, id+".dng")
		if _, sErr := os.Stat(rawCand); sErr == nil {
			item.RawAbs = rawCand
			item.RawRel = "./assets/" + id + ".dng"
			item.HasRaw = true
		}
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool {
		ci, cj := items[i].Meta.Experiment.CapturedAt, items[j].Meta.Experiment.CapturedAt
		if ci != cj {
			return ci > cj
		}
		return items[i].LabPhotoID > items[j].LabPhotoID
	})
	return items
}

// ExportBundleReport summarizes files packaged for Lightroom / ImageJ / journal submission.
type ExportBundleReport struct {
	DestDir       string
	CSVPath       string
	ShotCount     int
	RawCount      int
	ExportedFiles []string
}

// ExportBatchBundle copies all photos matching batchID (or all lab photos if batchID == "")
// into destDir together with README_calibration.csv for external pro software ingestion.
func ExportBatchBundle(assetsDir, batchID, destDir string) (ExportBundleReport, error) {
	var rep ExportBundleReport
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return rep, err
	}
	rep.DestDir = destDir
	gallery := ScanLabGallery(assetsDir)
	var rows [][]string
	rows = append(rows, []string{
		"lab_photo_id", "batch_id", "role", "raw_format", "preview_file", "raw_file",
		"iso", "shutter_s", "wb_kelvin", "wb_gains_rgb", "focus_pos",
		"pitch_deg", "roll_deg", "captured_at", "raw_sha256",
	})
	for _, it := range gallery {
		if batchID != "" && it.Meta.Experiment.BatchID != batchID {
			continue
		}
		rep.ShotCount++
		prevName := ""
		if it.PreviewAbs != "" {
			prevName = filepath.Base(it.PreviewAbs)
			dst := filepath.Join(destDir, prevName)
			if err := copyFileAtomic(it.PreviewAbs, dst); err != nil {
				return rep, err
			}
			rep.ExportedFiles = append(rep.ExportedFiles, dst)
		}
		rawName := ""
		if it.RawAbs != "" {
			rawName = filepath.Base(it.RawAbs)
			dst := filepath.Join(destDir, rawName)
			if err := copyFileAtomic(it.RawAbs, dst); err != nil {
				return rep, err
			}
			rep.RawCount++
			rep.ExportedFiles = append(rep.ExportedFiles, dst)
		}
		metaPath := filepath.Join(assetsDir, it.LabPhotoID+".asterphoto.json")
		if _, err := os.Stat(metaPath); err == nil {
			dst := filepath.Join(destDir, it.LabPhotoID+".asterphoto.json")
			if err := copyFileAtomic(metaPath, dst); err != nil {
				return rep, err
			}
			rep.ExportedFiles = append(rep.ExportedFiles, dst)
		}
		rows = append(rows, []string{
			it.LabPhotoID,
			it.Meta.Experiment.BatchID,
			it.Meta.Experiment.FrameRole,
			it.Meta.Raw.Format,
			prevName,
			rawName,
			fmt.Sprintf("%.0f", it.Meta.Exposure.ISO),
			fmt.Sprintf("%.6f", it.Meta.Exposure.ShutterS),
			fmt.Sprintf("%.0f", it.Meta.Exposure.WBKelvinEst),
			fmt.Sprintf("[%.4f %.4f %.4f]", it.Meta.Exposure.WBRGBGains[0], it.Meta.Exposure.WBRGBGains[1], it.Meta.Exposure.WBRGBGains[2]),
			fmt.Sprintf("%.4f", it.Meta.Geometry.FocusLensPosition),
			fmt.Sprintf("%.2f", it.Meta.Geometry.IMUPitchDeg),
			fmt.Sprintf("%.2f", it.Meta.Geometry.IMURollDeg),
			it.Meta.Experiment.CapturedAt,
			it.Meta.Raw.RawSHA256,
		})
	}
	csvPath := filepath.Join(destDir, "README_calibration.csv")
	f, err := os.OpenFile(csvPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return rep, err
	}
	w := csv.NewWriter(f)
	if err := w.WriteAll(rows); err != nil {
		f.Close()
		return rep, err
	}
	w.Flush()
	if err := f.Close(); err != nil {
		return rep, err
	}
	rep.CSVPath = csvPath
	rep.ExportedFiles = append(rep.ExportedFiles, csvPath)
	return rep, nil
}
