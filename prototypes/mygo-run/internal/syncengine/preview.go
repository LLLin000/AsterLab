package syncengine

import (
	"bytes"
	"image"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"
	"math"
	"path/filepath"
	"strings"

	_ "golang.org/x/image/bmp"
	"golang.org/x/image/draw"
	_ "golang.org/x/image/tiff"
	_ "golang.org/x/image/webp"
)

const (
	DefaultPreviewMaxDim  = 1200
	DefaultPreviewQuality = 82

	Threshold1MB  int64 = 1 << 20
	Threshold2MB  int64 = 2 << 20
	Threshold5MB  int64 = 5 << 20
	Threshold10MB int64 = 10 << 20
)

// DownloadPolicyMode defines when full lossless originals are pulled automatically
// versus pulling only lightweight preview sidecars.
type DownloadPolicyMode string

const (
	PolicyPreviewOnly    DownloadPolicyMode = "preview_only"
	PolicyThresholdBytes DownloadPolicyMode = "threshold_bytes"
	PolicyAlwaysOriginal DownloadPolicyMode = "always_original"
)

// DownloadPolicy controls the scientific original-upload + mobile preview-download
// behavior during synchronization.
type DownloadPolicy struct {
	Mode                            DownloadPolicyMode `json:"mode"`
	ThresholdBytes                  int64              `json:"thresholdBytes"`
	WiFiOnlyForOriginals            bool               `json:"wifiOnlyForOriginals"`
	IsMeteredNetwork                bool               `json:"isMeteredNetwork,omitempty"`
	IsMobile                        bool               `json:"isMobile"`
	AutoPruneMobileOriginalOnUpload bool               `json:"autoPruneMobileOriginalOnUpload,omitempty"`
}

// DefaultMobileDownloadPolicy returns the recommended mobile policy:
// pull originals automatically up to 2 MB on Wi-Fi, and preview-only above 2 MB.
func DefaultMobileDownloadPolicy() DownloadPolicy {
	return DownloadPolicy{
		Mode:                            PolicyThresholdBytes,
		ThresholdBytes:                  Threshold2MB,
		WiFiOnlyForOriginals:            true,
		IsMobile:                        true,
		AutoPruneMobileOriginalOnUpload: false,
	}
}

// DefaultDesktopDownloadPolicy returns the desktop policy: always pull full lossless originals.
func DefaultDesktopDownloadPolicy() DownloadPolicy {
	return DownloadPolicy{
		Mode:           PolicyAlwaysOriginal,
		ThresholdBytes: Threshold2MB,
		IsMobile:       false,
	}
}

// ThresholdPresets returns the UI-selectable byte threshold presets (1MB, 2MB, 5MB, 10MB).
func ThresholdPresets() []int64 {
	return []int64{Threshold1MB, Threshold2MB, Threshold5MB, Threshold10MB}
}

// ShouldDownloadOriginal decides whether an asset of originalSize bytes should
// download the full lossless original automatically under p.
// Non-asset files (.asterml, templates, config, pantry.jsonl) always return true.
func (p DownloadPolicy) ShouldDownloadOriginal(originalSize int64, isAsset bool) bool {
	if !isAsset {
		return true
	}
	if p.WiFiOnlyForOriginals && p.IsMeteredNetwork {
		return false
	}
	switch p.Mode {
	case PolicyPreviewOnly:
		return false
	case PolicyAlwaysOriginal:
		return true
	case PolicyThresholdBytes, "":
		limit := p.ThresholdBytes
		if limit <= 0 {
			limit = Threshold2MB
		}
		return originalSize <= limit
	default:
		return true
	}
}

// CanonicalOriginalPath returns the content-addressed remote storage path
// `assets/originals/<sha256>.<ext>` for an uploaded lossless scientific asset.
func CanonicalOriginalPath(sha256Hex, relPath string) string {
	ext := strings.ToLower(filepath.Ext(strings.TrimSpace(relPath)))
	if ext == "" {
		ext = ".bin"
	}
	return OriginalsDirRel + "/" + sha256Hex + ext
}

// CanonicalPreviewPath returns `.asterlab/previews/<sha256>.jpg`.
func CanonicalPreviewPath(sha256Hex string) string {
	return PreviewsDirRel + "/" + sha256Hex + ".jpg"
}

// GeneratePreviewJPEG decodes a raster image (PNG, JPEG, GIF, TIFF, BMP, WebP),
// resizes it proportionally so max(width, height) <= maxDim (default 1200px) using
// Catmull-Rom interpolation, and encodes a compact JPEG (default quality 82).
// If srcData is not a decodable image (e.g. PDF, CSV), it returns (nil, zero, false, nil).
func GeneratePreviewJPEG(srcData []byte, maxDim, quality int) ([]byte, image.Point, bool, error) {
	if len(srcData) == 0 {
		return nil, image.Point{}, false, nil
	}
	if maxDim <= 0 {
		maxDim = DefaultPreviewMaxDim
	}
	if quality <= 0 || quality > 100 {
		quality = DefaultPreviewQuality
	}

	srcImg, _, err := image.Decode(bytes.NewReader(srcData))
	if err != nil {
		// Non-image attachments (PDF/CSV/raw data) legitimately skip raster preview generation.
		return nil, image.Point{}, false, nil
	}
	bounds := srcImg.Bounds()
	srcW := bounds.Dx()
	srcH := bounds.Dy()
	if srcW <= 0 || srcH <= 0 {
		return nil, image.Point{}, false, nil
	}

	dstW, dstH := fitWithinMaxDim(srcW, srcH, maxDim)
	var outImg image.Image = srcImg
	if dstW != srcW || dstH != srcH {
		dst := image.NewRGBA(image.Rect(0, 0, dstW, dstH))
		draw.CatmullRom.Scale(dst, dst.Bounds(), srcImg, bounds, draw.Over, nil)
		outImg = dst
	}

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, outImg, &jpeg.Options{Quality: quality}); err != nil {
		return nil, image.Point{}, false, err
	}
	return buf.Bytes(), image.Point{X: dstW, Y: dstH}, true, nil
}

func fitWithinMaxDim(w, h, maxDim int) (int, int) {
	if w <= maxDim && h <= maxDim {
		return w, h
	}
	if w >= h {
		scale := float64(maxDim) / float64(w)
		dstH := int(math.Round(float64(h) * scale))
		if dstH < 1 {
			dstH = 1
		}
		return maxDim, dstH
	}
	scale := float64(maxDim) / float64(h)
	dstW := int(math.Round(float64(w) * scale))
	if dstW < 1 {
		dstW = 1
	}
	return dstW, maxDim
}
