package scicamera

import (
	"context"

	"github.com/egoist/mygo"
)

const pluginID = "scicamera"

// Options configures the embedded scientific viewfinder plugin view.
type Options struct {
	Front bool `json:"front"`
}

// ControlRequest updates hardware exposure, white balance, and focus parameters on mobile devices.
type ControlRequest struct {
	ExposureLocked *bool       `json:"exposureLocked,omitempty"`
	ISO            float64     `json:"iso,omitempty"`
	ShutterS       float64     `json:"shutterS,omitempty"`
	EVBias         *float64    `json:"evBias,omitempty"`
	WBMode         string      `json:"wbMode,omitempty"` // "" | "auto" | "lock_current" | "kelvin" | "gains"
	WBKelvin       float64     `json:"wbKelvin,omitempty"`
	WBTint         float64     `json:"wbTint,omitempty"`
	WBGains        *[3]float64 `json:"wbGains,omitempty"`
	FocusMode      string      `json:"focusMode,omitempty"` // "" | "auto" | "lock_current" | "point" | "lens_pos"
	FocusX         float64     `json:"focusX,omitempty"`
	FocusY         float64     `json:"focusY,omitempty"`
	FocusLensPos   float64     `json:"focusLensPos,omitempty"`
}

// Telemetry reflects live physical sensor & IMU state returned by iOS/Android camera plugins.
type Telemetry struct {
	Running        bool       `json:"running"`
	RawSupported   bool       `json:"rawSupported"`
	ISO            float64    `json:"iso"`
	MinISO         float64    `json:"minISO"`
	MaxISO         float64    `json:"maxISO"`
	ShutterS       float64    `json:"shutterS"`
	ApertureF      float64    `json:"apertureF"`
	EVBias         float64    `json:"evBias"`
	WBKelvin       float64    `json:"wbKelvin"`
	WBTint         float64    `json:"wbTint"`
	WBGains        [3]float64 `json:"wbGains"`
	MaxWBGain      float64    `json:"maxWBGain"`
	FocusLensPos   float64    `json:"focusLensPos"`
	PitchDeg       float64    `json:"pitchDeg"`
	RollDeg        float64    `json:"rollDeg"`
	YawDeg         float64    `json:"yawDeg"`
	ExposureLocked bool       `json:"exposureLocked"`
	WBLocked       bool       `json:"wbLocked"`
	FocusLocked    bool       `json:"focusLocked"`
	DeviceModel    string     `json:"deviceModel"`
	OSVersion      string     `json:"osVersion"`
	FocalLengthMM  float64    `json:"focalLengthMM"`
}

// CaptureOptions configures a single scientific capture invocation.
type CaptureOptions struct {
	CaptureRaw   bool   `json:"captureRaw"`
	SaveToPhotos bool   `json:"saveToPhotos"`
	AlbumName    string `json:"albumName,omitempty"`
	LabPhotoID   string `json:"labPhotoID,omitempty"`
}

// CaptureResult holds temporary file outputs returned by the native plugin before Vault archival.
type CaptureResult struct {
	PreviewPath   string     `json:"previewPath"`
	RawPath       string     `json:"rawPath,omitempty"`
	RawFormat     string     `json:"rawFormat"`
	BitDepth      int        `json:"bitDepth"`
	AsShotNeutral [3]float64 `json:"asShotNeutral"`
	PhotosAssetID string     `json:"photosAssetId,omitempty"`
	Telemetry     Telemetry  `json:"telemetry"`
}

// Camera wraps MyGo's native scicamera PluginView for embedding inside ui.WebView overlays.
type Camera struct {
	view *mygo.PluginView
}

// New creates the embedded physical camera preview surface on mobile OSes (errors.ErrUnsupported on desktop).
func New(w *mygo.Window, o Options) (*Camera, error) {
	v, err := mygo.NewPluginView(w, pluginID, map[string]any{"front": o.Front})
	if err != nil {
		return nil, err
	}
	return &Camera{view: v}, nil
}

// View returns the underlying *mygo.PluginView passed directly to ui.WebView(c, cam.View()).
func (c *Camera) View() *mygo.PluginView {
	if c == nil {
		return nil
	}
	return c.view
}

// Configure applies manual locks or parameter changes on the live capture device.
func (c *Camera) Configure(ctx context.Context, req ControlRequest) (Telemetry, error) {
	var out Telemetry
	if c == nil || c.view == nil {
		return out, nil
	}
	args := map[string]any{"id": c.view.ID}
	if req.ExposureLocked != nil {
		args["exposureLocked"] = *req.ExposureLocked
	}
	if req.ISO > 0 {
		args["iso"] = req.ISO
	}
	if req.ShutterS > 0 {
		args["shutterS"] = req.ShutterS
	}
	if req.EVBias != nil {
		args["evBias"] = *req.EVBias
	}
	if req.WBMode != "" {
		args["wbMode"] = req.WBMode
	}
	if req.WBKelvin > 0 {
		args["wbKelvin"] = req.WBKelvin
	}
	args["wbTint"] = req.WBTint
	if req.WBGains != nil {
		args["wbGains"] = req.WBGains[:]
	}
	if req.FocusMode != "" {
		args["focusMode"] = req.FocusMode
	}
	if req.FocusX > 0 || req.FocusY > 0 {
		args["focusX"] = req.FocusX
		args["focusY"] = req.FocusY
	}
	if req.FocusLensPos > 0 {
		args["focusLensPos"] = req.FocusLensPos
	}
	err := mygo.Native(pluginID).Call(ctx, "configure", args, &out)
	return out, err
}

// Telemetry reads the current hardware exposure, white balance, focus, and IMU orientation.
func (c *Camera) Telemetry(ctx context.Context) (Telemetry, error) {
	var out Telemetry
	if c == nil || c.view == nil {
		return out, nil
	}
	err := mygo.Native(pluginID).Call(ctx, "telemetry", map[string]any{"id": c.view.ID}, &out)
	return out, err
}

// Capture fires a single scientific capture (Bayer DNG + processed sRGB JPEG where supported).
func (c *Camera) Capture(ctx context.Context, opts CaptureOptions) (CaptureResult, error) {
	var out CaptureResult
	if c == nil || c.view == nil {
		return out, nil
	}
	args := map[string]any{
		"id":           c.view.ID,
		"captureRaw":   opts.CaptureRaw,
		"saveToPhotos": opts.SaveToPhotos,
		"albumName":    opts.AlbumName,
		"labPhotoID":   opts.LabPhotoID,
	}
	err := mygo.Native(pluginID).Call(ctx, "capture", args, &out)
	return out, err
}

// Destroy releases the camera hardware, stops IMU polling, and closes the plugin view.
func (c *Camera) Destroy() {
	if c == nil || c.view == nil {
		return
	}
	go mygo.Native(pluginID).Call(context.Background(), "dispose", map[string]any{"id": c.view.ID}, nil)
	c.view.Destroy()
}
