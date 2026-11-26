package main

import (
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/onevra/source/components"
	"github.com/oneo-me/onevra/source/icons/lucide"

	"asterlab-mygo-prototype/internal/astercmd"
	"asterlab-mygo-prototype/internal/scicamera"
	"asterlab-mygo-prototype/internal/shell"
	"asterlab-mygo-prototype/internal/theme"
	"asterlab-mygo-prototype/internal/uikit/kit"
	"asterlab-mygo-prototype/internal/vault"
)

type sciCaptureEvent struct {
	res scicamera.CaptureResult
	err error
}

type sciTelemetryEvent struct {
	telem scicamera.Telemetry
	err   error
}

type sciCameraSession struct {
	open           bool
	cam            *scicamera.Camera
	camInitFailed  bool
	tabPath        string
	expTag         string
	dateYMD        string
	applyDoc       func(string)
	batchID        string
	shotCount      int
	lastPreviewRel string
	lastRawRel     string
	lastMetaRel    string
	lastPreviewAbs string
	lastRawAbs     string
	lastMetaAbs    string
	batchRawPaths  []string
	batchFiles     []string

	// Low-frequency body configuration
	captureRaw   bool
	saveToPhotos bool
	albumName    string
	flickerHz    int // 50 | 60
	gridEnabled  bool
	ghostEnabled bool
	ghostImagePath string
	settingsOpen bool

	// Optical manual controls (ISO / SS / EV / WB / MF + AE/AWB/AF lock)
	activeParam      string // "" | "iso" | "ss" | "ev" | "wb" | "mf"
	isoAuto          bool
	selectedISO      float64
	shutterAuto      bool
	selectedShutterS float64
	evBias           float64
	wbMode           string // "auto" | "lock_current" | "kelvin"
	selectedWBKelvin float64
	focusAuto        bool
	focusLocked      bool
	focusLensPos     float64
	allLocked        bool

	refPitchDeg      float64
	refRollDeg       float64
	hasRefPose       bool
	flatFieldAssetID string
	batchRefAssetID  string

	busy       bool
	toastText  string
	toastUntil time.Time
	flashUntil time.Time

	telemetry  scicamera.Telemetry
	lastPollAt time.Time
	polling    bool
	captureCh  chan sciCaptureEvent
	telemCh    chan sciTelemetryEvent

	// Hook for deterministic headless testing.
	captureOverride func(context.Context, scicamera.CaptureOptions) (scicamera.CaptureResult, error)
}

type labGalleryState struct {
	open       bool
	fromCamera bool
	filterRole string // "" | sample | flat_field | wb_reference | scale_bar_ref
	items      []scicamera.GalleryItem
	selectedID string
	dirty      bool
	tabPath    string
	applyDoc   func(string)

	imgMu    sync.Mutex
	imgCache map[string]ui.ImageSource
	imgErr   map[string]bool
}

func (a *app) ensureSciCamera() *sciCameraSession {
	if a.sciCam == nil {
		a.sciCam = &sciCameraSession{
			captureRaw:       true,
			saveToPhotos:     true,
			albumName:        "AsterLab",
			flickerHz:        50,
			gridEnabled:      true,
			isoAuto:          true,
			shutterAuto:      true,
			wbMode:           "auto",
			selectedWBKelvin: 5200,
			focusAuto:        true,
			focusLensPos:     0.50,
			captureCh:        make(chan sciCaptureEvent, 2),
			telemCh:          make(chan sciTelemetryEvent, 4),
		}
	}
	return a.sciCam
}

func (a *app) ensureLabGallery() *labGalleryState {
	if a.labGallery == nil {
		a.labGallery = &labGalleryState{
			dirty:    true,
			imgCache: map[string]ui.ImageSource{},
			imgErr:   map[string]bool{},
		}
	}
	return a.labGallery
}

func (g *labGalleryState) cachedThumb(path string) ui.ImageSource {
	if g == nil || strings.TrimSpace(path) == "" {
		return nil
	}
	g.imgMu.Lock()
	defer g.imgMu.Unlock()
	if g.imgCache == nil {
		g.imgCache = map[string]ui.ImageSource{}
	}
	if g.imgErr == nil {
		g.imgErr = map[string]bool{}
	}
	if img, ok := g.imgCache[path]; ok {
		return img
	}
	if g.imgErr[path] {
		return nil
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		g.imgErr[path] = true
		return nil
	}
	img, err := ui.DecodeImage(raw)
	if err != nil {
		g.imgErr[path] = true
		return nil
	}
	if len(g.imgCache) > 24 {
		g.imgCache = map[string]ui.ImageSource{}
	}
	g.imgCache[path] = img
	return img
}

func (a *app) openSciCamera(c *ui.Context, curTab shell.WorkspaceTab, _ *vault.LiveDocument, applyDoc func(string)) {
	sess := a.ensureSciCamera()
	if sess.cam != nil {
		sess.cam.Destroy()
		sess.cam = nil
	}
	sess.open = true
	sess.camInitFailed = false
	sess.tabPath = curTab.FilePath
	sess.applyDoc = applyDoc
	sess.dateYMD = time.Now().Format("2006-01-02")
	sess.expTag = scicamera.SanitizeExpTag(strings.TrimSuffix(filepath.Base(curTab.FilePath), filepath.Ext(curTab.FilePath)))
	if curTab.Title != "" {
		if t := scicamera.SanitizeExpTag(curTab.Title); t != "LAB" {
			sess.expTag = t
		}
	}
	sess.batchID = fmt.Sprintf("batch-%s-%s", strings.ReplaceAll(sess.dateYMD, "-", ""), time.Now().Format("150405"))
	sess.shotCount = 0
	sess.batchRawPaths = nil
	sess.batchFiles = nil
	sess.activeParam = ""
	sess.settingsOpen = false
	sess.busy = false
	sess.toastText = ""
	sess.lastPollAt = time.Time{}

	// Preload latest vault photo as default ghost candidate if none set yet.
	if sess.ghostImagePath == "" && a.vault != nil {
		g := a.ensureLabGallery()
		if g.dirty {
			a.refreshLabGallery()
		}
		if len(g.items) > 0 {
			sess.ghostImagePath = g.items[0].PreviewAbs
		}
	}

	if isMobileRuntime() && a.shell != nil && a.shell.MyGoWindow() != nil && sess.captureOverride == nil {
		cam, err := scicamera.New(a.shell.MyGoWindow(), scicamera.Options{})
		if err != nil {
			sess.camInitFailed = true
			sess.setToast("相机启动失败: " + err.Error())
		} else {
			sess.cam = cam
		}
	}
	if c != nil {
		c.Invalidate()
	}
}

func (s *sciCameraSession) setToast(msg string) {
	s.toastText = msg
	s.toastUntil = time.Now().Add(2200 * time.Millisecond)
}

func (a *app) closeSciCamera() {
	if a.sciCam == nil {
		return
	}
	a.sciCam.open = false
	if a.sciCam.cam != nil {
		a.sciCam.cam.Destroy()
		a.sciCam.cam = nil
	}
	a.sciCam.busy = false
	a.sciCam.polling = false
	a.sciCam.settingsOpen = false
	a.sciCam.activeParam = ""
}

func (a *app) openLabGallery(curTab *shell.WorkspaceTab, applyDoc func(string)) {
	g := a.ensureLabGallery()
	g.open = true
	g.fromCamera = a.sciCam != nil && a.sciCam.open
	g.dirty = true
	if curTab != nil && curTab.Kind == shell.TabFile {
		g.tabPath = curTab.FilePath
	} else if a.sciCam != nil && a.sciCam.tabPath != "" {
		g.tabPath = a.sciCam.tabPath
	}
	if applyDoc != nil {
		g.applyDoc = applyDoc
	} else if a.sciCam != nil && a.sciCam.applyDoc != nil {
		g.applyDoc = a.sciCam.applyDoc
	}
	a.refreshLabGallery()
}

func (a *app) refreshLabGallery() {
	g := a.ensureLabGallery()
	if a.vault == nil {
		g.items = nil
		g.dirty = false
		return
	}
	g.items = scicamera.ScanLabGallery(a.vault.AssetsDir())
	g.dirty = false
	if len(g.items) > 0 {
		found := false
		for _, it := range g.items {
			if it.LabPhotoID == g.selectedID {
				found = true
				break
			}
		}
		if !found {
			g.selectedID = g.items[0].LabPhotoID
		}
	}
}

func (a *app) pollSciCamera(c *ui.Context) {
	sess := a.sciCam
	if sess == nil || !sess.open {
		return
	}
	for {
		select {
		case ev := <-sess.captureCh:
			sess.busy = false
			if ev.err != nil {
				sess.setToast("拍摄失败: " + ev.err.Error())
				mygo.Haptics.Warning()
			} else {
				a.handleSciCaptureSuccess(c, ev.res)
			}
		case tv := <-sess.telemCh:
			sess.polling = false
			if tv.err == nil {
				sess.telemetry = tv.telem
			}
		default:
			goto doneDrain
		}
	}
doneDrain:
	if sess.cam != nil && !sess.polling && time.Since(sess.lastPollAt) >= 200*time.Millisecond {
		sess.polling = true
		sess.lastPollAt = time.Now()
		cam := sess.cam
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 400*time.Millisecond)
			defer cancel()
			t, err := cam.Telemetry(ctx)
			select {
			case sess.telemCh <- sciTelemetryEvent{telem: t, err: err}:
			default:
			}
		}()
	}
	if sess.cam != nil || (sess.toastText != "" && time.Now().Before(sess.toastUntil)) || time.Now().Before(sess.flashUntil) {
		c.After(220 * time.Millisecond)
	}
}

func (a *app) applySciControl(req scicamera.ControlRequest) {
	sess := a.sciCam
	if sess == nil || sess.cam == nil {
		return
	}
	cam := sess.cam
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		t, err := cam.Configure(ctx, req)
		if err == nil {
			select {
			case sess.telemCh <- sciTelemetryEvent{telem: t}:
			default:
			}
		}
	}()
}

func (a *app) syncManualExposureControl(sess *sciCameraSession) {
	if sess.isoAuto && sess.shutterAuto {
		lock := false
		a.applySciControl(scicamera.ControlRequest{ExposureLocked: &lock})
		return
	}
	lock := true
	req := scicamera.ControlRequest{ExposureLocked: &lock}
	if !sess.isoAuto && sess.selectedISO > 0 {
		req.ISO = sess.selectedISO
	}
	if !sess.shutterAuto && sess.selectedShutterS > 0 {
		req.ShutterS = sess.selectedShutterS
	}
	a.applySciControl(req)
}

func (a *app) toggleSciBatchLock() {
	sess := a.sciCam
	if sess == nil {
		return
	}
	sess.allLocked = !sess.allLocked
	if sess.allLocked {
		lock := true
		// Lock-what-you-see: capture actual metered ISO + shutter without jumping down to MinISO.
		if sess.isoAuto && sess.telemetry.ISO > 0 {
			sess.isoAuto = false
			sess.selectedISO = sess.telemetry.ISO
		}
		if sess.shutterAuto && sess.telemetry.ShutterS > 0 {
			sess.shutterAuto = false
			sess.selectedShutterS = sess.telemetry.ShutterS
		}
		req := scicamera.ControlRequest{
			ExposureLocked: &lock,
			ISO:            sess.selectedISO,
			ShutterS:       sess.selectedShutterS,
			FocusMode:      "lock_current",
		}
		sess.focusAuto = false
		sess.focusLocked = true
		if sess.telemetry.FocusLensPos > 0 {
			sess.focusLensPos = sess.telemetry.FocusLensPos
		}
		if sess.wbMode == "auto" {
			req.WBMode = "lock_current"
			sess.wbMode = "lock_current"
			if sess.telemetry.WBKelvin > 0 {
				sess.selectedWBKelvin = sess.telemetry.WBKelvin
			}
		}
		a.applySciControl(req)
		if sess.telemetry.Running {
			sess.refPitchDeg = sess.telemetry.PitchDeg
			sess.refRollDeg = sess.telemetry.RollDeg
			sess.hasRefPose = true
		}
		sess.setToast("已锁定 AE / AWB / AF")
		mygo.Haptics.Success()
		return
	}
	lock := false
	sess.isoAuto = true
	sess.shutterAuto = true
	sess.wbMode = "auto"
	sess.focusAuto = true
	sess.focusLocked = false
	sess.evBias = 0
	ev := 0.0
	a.applySciControl(scicamera.ControlRequest{
		ExposureLocked: &lock,
		EVBias:         &ev,
		WBMode:         "auto",
		FocusMode:      "auto",
	})
	sess.setToast("已恢复自动参数")
	mygo.Haptics.Selection()
}

func (a *app) triggerSciCapture() {
	sess := a.sciCam
	if sess == nil || sess.busy || a.vault == nil {
		return
	}
	if sess.cam == nil && sess.captureOverride == nil {
		sess.setToast("当前无物理相机")
		return
	}
	labID := scicamera.NextLabPhotoID(a.vault.AssetsDir(), sess.dateYMD, sess.expTag, scicamera.RoleSample)
	opts := scicamera.CaptureOptions{
		CaptureRaw:   sess.captureRaw,
		SaveToPhotos: sess.saveToPhotos,
		AlbumName:    sess.albumName,
		LabPhotoID:   labID,
	}
	sess.busy = true
	sess.flashUntil = time.Now().Add(140 * time.Millisecond)
	mygo.Haptics.Light()

	cam := sess.cam
	override := sess.captureOverride
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
		defer cancel()
		var res scicamera.CaptureResult
		var err error
		if override != nil {
			res, err = override(ctx, opts)
		} else {
			res, err = cam.Capture(ctx, opts)
		}
		sess.captureCh <- sciCaptureEvent{res: res, err: err}
	}()
}

func (a *app) handleSciCaptureSuccess(c *ui.Context, res scicamera.CaptureResult) {
	sess := a.sciCam
	if sess == nil || a.vault == nil {
		return
	}
	labID := scicamera.NextLabPhotoID(a.vault.AssetsDir(), sess.dateYMD, sess.expTag, scicamera.RoleSample)
	expMode := "auto"
	if res.Telemetry.ExposureLocked || !sess.isoAuto || !sess.shutterAuto || sess.allLocked {
		expMode = "locked"
	}
	wbMode := sess.wbMode
	if wbMode == "" {
		wbMode = "auto"
	}
	focusMode := "auto"
	if res.Telemetry.FocusLocked || !sess.focusAuto || sess.allLocked {
		focusMode = "locked"
	}
	bitDepth := res.BitDepth
	if bitDepth <= 0 {
		bitDepth = 8
	}
	rawFmt := res.RawFormat
	if rawFmt == "" {
		if res.RawPath != "" {
			rawFmt = "bayer_cfa_dng"
		} else {
			rawFmt = "processed_jpeg"
		}
	}
	if sess.shotCount == 0 && !sess.hasRefPose {
		sess.refPitchDeg = res.Telemetry.PitchDeg
		sess.refRollDeg = res.Telemetry.RollDeg
		sess.hasRefPose = true
	}

	flickerHz := sess.flickerHz
	if flickerHz <= 0 {
		flickerHz = 50
	}

	meta := scicamera.PhotoMetadata{
		Schema:        scicamera.MetadataSchema,
		LabPhotoID:    labID,
		PhotosAssetID: res.PhotosAssetID,
		Raw: scicamera.RawInfo{
			Format:           rawFmt,
			BitDepth:         bitDepth,
			SmartHDRDisabled: rawFmt == "bayer_cfa_dng",
			MultiFrameFusion: false,
			AsShotNeutral:    res.AsShotNeutral,
		},
		Exposure: scicamera.ExposureInfo{
			ISO:                res.Telemetry.ISO,
			DeviceMinNativeISO: res.Telemetry.MinISO,
			ShutterS:           res.Telemetry.ShutterS,
			ApertureF:          res.Telemetry.ApertureF,
			EVBias:             res.Telemetry.EVBias,
			FlickerGridHz:      flickerHz,
			ExposureMode:       expMode,
			WBMode:             wbMode,
			WBKelvinEst:        res.Telemetry.WBKelvin,
			WBTintEst:          res.Telemetry.WBTint,
			WBRGBGains:         res.Telemetry.WBGains,
		},
		Geometry: scicamera.GeometryInfo{
			DeviceModel:       res.Telemetry.DeviceModel,
			OSVersion:         res.Telemetry.OSVersion,
			PhysicalCameraID:  "builtInWideAngleCamera_back",
			FocalLengthMM:     res.Telemetry.FocalLengthMM,
			FocalLength35mmEq: 24,
			FocusMode:         focusMode,
			FocusLensPosition: res.Telemetry.FocusLensPos,
			IMUPitchDeg:       res.Telemetry.PitchDeg,
			IMURollDeg:        res.Telemetry.RollDeg,
			IMUYawDeg:         res.Telemetry.YawDeg,
		},
		Calibration: scicamera.CalibrationInfo{
			FlatFieldAssetID:  sess.flatFieldAssetID,
			ColorSpacePreview: "sRGB",
		},
		Experiment: scicamera.ExperimentInfo{
			ExperimentID:      sess.expTag,
			BatchID:           sess.batchID,
			FrameRole:         scicamera.RoleSample,
			BatchRefAssetID:   sess.batchRefAssetID,
			CapturedAt:        time.Now().Format(time.RFC3339),
			NonDestructiveOps: []string{},
		},
	}

	archived, err := scicamera.SaveCapturedTrio(a.vault.AssetsDir(), labID, res.PreviewPath, res.RawPath, meta)
	if err != nil {
		sess.setToast("归档原片失败: " + err.Error())
		return
	}
	_, _ = a.generateAssetPreviewSidecar(archived.PreviewRel)
	if a.labGallery != nil {
		a.labGallery.dirty = true
	}

	sess.shotCount++
	sess.lastPreviewRel = archived.PreviewRel
	sess.lastRawRel = archived.RawRel
	sess.lastMetaRel = archived.MetaRel
	sess.lastPreviewAbs = archived.PreviewAbs
	sess.lastRawAbs = archived.RawAbs
	sess.lastMetaAbs = archived.MetaAbs
	sess.ghostImagePath = archived.PreviewAbs
	if sess.batchRefAssetID == "" {
		sess.batchRefAssetID = archived.LabPhotoID
	}
	if archived.RawAbs != "" {
		sess.batchRawPaths = append(sess.batchRawPaths, archived.RawAbs)
	}
	if archived.PreviewAbs != "" {
		sess.batchFiles = append(sess.batchFiles, archived.PreviewAbs)
	}
	if archived.MetaAbs != "" {
		sess.batchFiles = append(sess.batchFiles, archived.MetaAbs)
	}

	tag := "已拍 JPG"
	if archived.RawAbs != "" {
		tag = "已存 DNG"
	}
	if res.PhotosAssetID != "" {
		tag += " · 相册"
	}
	sess.setToast(tag + " (" + archived.LabPhotoID + ")")
	mygo.Haptics.Success()
	if c != nil {
		c.Invalidate()
	}
}

func (a *app) shareLabFiles(c *ui.Context, files []string, label string) {
	var clean []string
	for _, f := range files {
		if f == "" {
			continue
		}
		if _, err := os.Stat(f); err == nil {
			clean = append(clean, f)
		}
	}
	if len(clean) == 0 {
		addPageToast(c, "暂无可导出的原片文件", components.ToastWarning)
		return
	}
	if runtime.GOOS == "ios" || runtime.GOOS == "android" {
		if err := mygo.Share(mygo.ShareOptions{
			Text:  label,
			Files: clean,
		}); err != nil {
			addPageToast(c, "分享原片失败："+err.Error(), components.ToastWarning)
		}
		return
	}
	if len(clean) == 1 {
		data, err := os.ReadFile(clean[0])
		if err != nil {
			addPageToast(c, "读取原片失败："+err.Error(), components.ToastWarning)
			return
		}
		ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(clean[0])), ".")
		_, _ = a.saveExportBytes(c, filepath.Base(clean[0]), "导出科研原片", mygo.FileFilter{Name: "科研原片", Extensions: []string{ext}}, data)
		return
	}
	if a.vault != nil {
		dest := filepath.Join(filepath.Dir(clean[0]), "export_"+time.Now().Format("150405"))
		rep, err := scicamera.ExportBatchBundle(a.vault.AssetsDir(), "", dest)
		if err == nil {
			addPageToast(c, fmt.Sprintf("已打包 %d 组科研原片至 %s", rep.ShotCount, dest), components.ToastSuccess)
			return
		}
	}
	addPageToast(c, fmt.Sprintf("共就绪 %d 个原片与元数据文件", len(clean)), components.ToastSuccess)
}

func (a *app) routeArchivedPhotoIntoDoc(c *ui.Context, relAsset, rawAbs, labID string) {
	sess := a.sciCam
	tabPath := ""
	var applyDoc func(string)
	if a.labGallery != nil && a.labGallery.tabPath != "" {
		tabPath = a.labGallery.tabPath
		applyDoc = a.labGallery.applyDoc
	} else if sess != nil && sess.tabPath != "" {
		tabPath = sess.tabPath
		applyDoc = sess.applyDoc
	} else if a.shell != nil {
		if tab := a.shell.ActiveTab(); tab != nil && tab.Kind == shell.TabFile {
			tabPath = tab.FilePath
		}
	}
	if tabPath == "" || a.docStore == nil || relAsset == "" {
		return
	}
	ld, err := a.docStore.Open(tabPath)
	if err != nil || ld == nil {
		return
	}
	if applyDoc == nil {
		applyDoc = func(next string) {
			if next == ld.Source {
				return
			}
			_ = ld.ApplyEdit(next)
			ld.NotifySaved(a.vault, false)
			if a.vaultIndex != nil {
				a.vaultIndex.UpsertSource(tabPath, ld.Source)
			}
			if a.liveEditorSynced != nil {
				a.liveEditorSynced[tabPath] = strings.ReplaceAll(ld.Source, "\r\n", "\n")
			}
			a.refreshVaultTreeAndCalendar()
		}
	}
	safeAsset := sanitizeWikiAssetPath(relAsset)
	var platFmt, excPct int
	if a.shell != nil {
		platFmt = a.shell.DefaultPlateFormat
		excPct = a.shell.DefaultExcessPct
	}
	opts := collectPhotoRouteOptions(ld.Parsed)
	if len(opts) > 0 {
		a.pendingPhotoRoute = &photoRouteRequest{
			open:       true,
			asset:      safeAsset,
			rawAbs:     rawAbs,
			labPhotoID: labID,
			tabPath:    tabPath,
			options:    opts,
			applyDoc:   applyDoc,
		}
		return
	}
	res := executePhotoRouteChoice(ld.Source, astercmd.ModeEdit, platFmt, excPct, safeAsset, photoRouteOption{Kind: "inline_embed"})
	if res.OK {
		applyDoc(res.NextSource)
		addPageToast(c, "已归档照片并插入 ![[...]] 图片卡片", components.ToastSuccess)
	}
}

func formatShutterShort(s float64) string {
	if s <= 0 {
		return "自动"
	}
	if s < 0.8 {
		denom := math.Round(1.0 / s)
		if denom < 1 {
			denom = 1
		}
		return fmt.Sprintf("1/%.0f", denom)
	}
	return fmt.Sprintf("%.1fs", s)
}

func (a *app) renderSciCameraOverlay(c *ui.Context) {
	sess := a.sciCam
	if sess == nil || !sess.open {
		return
	}
	if a.labGallery != nil && a.labGallery.open {
		return
	}
	a.pollSciCamera(c)

	tok := theme.DarkTokens()
	safeTop, safeRight, safeBottom, safeLeft := c.SafeArea()
	g := a.ensureLabGallery()

	ui.Overlay(c, func() {
		root := ui.Column(c.Key("sci-camera-overlay")).Absolute().Fill().Modal().Background(tok.Canvas)
		root.Children(func() {
			// ============================================================
			// ZONE 1: TOP UTILITY BAR (Pure low-frequency body configuration)
			// ============================================================
			ui.Row(c.Key("sci-cam-topbar")).FillWidth().
				Padding(safeTop+float32(tok.Spacing4), safeRight+float32(tok.Spacing12), float32(tok.Spacing8), safeLeft+float32(tok.Spacing12)).
				AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Background(tok.Canvas).Children(func() {
				closeBtn := ui.ButtonBase(c.Key("sci-cam-close")).Size(float32(tok.Spacing32)+float32(tok.Spacing4), float32(tok.Spacing32)+float32(tok.Spacing4)).
					Radius(float32(tok.PillRadius)).Background(tok.Elevated).Center().Label("关闭相机").
					OnClick(func() {
						mygo.Haptics.Selection()
						a.closeSciCamera()
					})
				theme.ApplyTactilePress(closeBtn)
				closeBtn.Children(func() {
					ui.Icon(c, lucide.X()).Size(float32(tok.IconMd), float32(tok.IconMd)).TextColor(tok.Text)
				})

				ui.Spacer(c)

				// Pure format toggle: DNG vs JPG (honest fallback if hardware does not support Bayer RAW)
				rawAvail := !sess.telemetry.Running || sess.telemetry.RawSupported
				fmtLbl := "DNG"
				fmtBg := tok.AccentSoftBg
				fmtFg := tok.Accent
				if !sess.captureRaw || !rawAvail {
					fmtLbl = "JPG"
					fmtBg = tok.Elevated
					fmtFg = tok.TextMuted
				}
				fmtBtn := ui.ButtonBase(c.Key("sci-cam-fmt")).Height(float32(tok.Spacing32)+float32(tok.Spacing4)).
					Padding(0, float32(tok.Spacing12)).Radius(float32(tok.PillRadius)).
					Background(fmtBg).Center().Label("切换格式").
					OnClick(func() {
						if !rawAvail {
							sess.captureRaw = false
							sess.setToast("当前设备仅支持 JPG 格式")
							mygo.Haptics.Warning()
							return
						}
						sess.captureRaw = !sess.captureRaw
						mygo.Haptics.Selection()
					})
				theme.ApplyTactilePress(fmtBtn)
				fmtBtn.Children(func() {
					ui.Text(c, fmtLbl).FontSize(float32(tok.TextMeta)).Bold().TextColor(fmtFg).SingleLine()
				})

				// Grid & Level toggle
				topIconBtn := func(key, label string, icon *ui.SVG, active bool, onTap func()) {
					bg := tok.Elevated
					fg := tok.TextMuted
					if active {
						bg = tok.AccentSoftBg
						fg = tok.Accent
					}
					b := ui.ButtonBase(c.Key(key)).Size(float32(tok.Spacing32)+float32(tok.Spacing4), float32(tok.Spacing32)+float32(tok.Spacing4)).
						Radius(float32(tok.PillRadius)).Background(bg).Center().Label(label).OnClick(onTap)
					theme.ApplyTactilePress(b)
					b.Children(func() {
						ui.Icon(c, icon).Size(float32(tok.IconMd), float32(tok.IconMd)).TextColor(fg)
					})
				}

				topIconBtn("sci-cam-grid", "网格与水平仪", lucide.Grid3x3(), sess.gridEnabled, func() {
					sess.gridEnabled = !sess.gridEnabled
					mygo.Haptics.Selection()
				})
				topIconBtn("sci-cam-ghost", "叠影参照", lucide.Layers(), sess.ghostEnabled && sess.ghostImagePath != "", func() {
					if !sess.ghostEnabled {
						if sess.ghostImagePath == "" && len(g.items) > 0 {
							sess.ghostImagePath = g.items[0].PreviewAbs
						}
						if sess.ghostImagePath == "" {
							sess.setToast("先拍 1 张照片后可开启叠影参照")
							mygo.Haptics.Warning()
							return
						}
					}
					sess.ghostEnabled = !sess.ghostEnabled
					mygo.Haptics.Selection()
				})
				topIconBtn("sci-cam-settings", "相机设置", lucide.Settings(), sess.settingsOpen, func() {
					sess.settingsOpen = !sess.settingsOpen
					if sess.settingsOpen {
						sess.activeParam = ""
					}
					mygo.Haptics.Selection()
				})
			})

			// ============================================================
			// ZONE 2: CLEAN 4:3 NATIVE SENSOR VIEWFINDER
			// ============================================================
			vfWrap := ui.Box(c.Key("sci-cam-vf-wrap")).FillWidth().Grow(1).Basis(0).MinHeight(0).
				Center().Clip().Background(tok.Canvas)
			vfWrap.Children(func() {
				vf := ui.Box(c.Key("sci-cam-vf")).FillWidth().AspectRatio(0.75).
					Background(tok.Surface.Alpha(0.25)).Border(1, tok.BorderSubtle).Clip()
				vf.Children(func() {
					// Live native viewfinder or headless placeholder
					if sess.cam != nil && sess.cam.View() != nil {
						ui.WebView(c, sess.cam.View()).Fill()
					} else {
						ui.Box(c.Key("sci-cam-sim-viewfinder")).Fill().Center().Children(func() {
							ui.Column(c).AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
								ui.Icon(c, lucide.Camera()).Size(float32(tok.Spacing32), float32(tok.Spacing32)).TextColor(tok.TextMuted)
								ui.Text(c, "1x 物理主摄取景器").FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
							})
						})
					}

					// Optional 22% Ghost Overlay
					if sess.ghostEnabled && sess.ghostImagePath != "" {
						if ghostImg := g.cachedThumb(sess.ghostImagePath); ghostImg != nil {
							ui.Box(c.Key("sci-cam-ghost-wrap")).Absolute().Fill().PassThrough().Center().Children(func() {
								ui.Image(c, ghostImg).Fill().Opacity(0.22)
							})
						}
					}

					// Optional 3x3 Rule-of-Thirds Grid & Center Level Horizon
					if sess.gridEnabled {
						gridOverlay := ui.Box(c.Key("sci-cam-grid-lines")).Absolute().Fill().PassThrough()
						gridOverlay.Children(func() {
							// Vertical thirds
							ui.Row(c).Absolute().Fill().PassThrough().Children(func() {
								ui.Box(c).Grow(1).FillHeight()
								ui.Box(c).Width(1).FillHeight().Background(tok.Text.Alpha(0.16))
								ui.Box(c).Grow(1).FillHeight()
								ui.Box(c).Width(1).FillHeight().Background(tok.Text.Alpha(0.16))
								ui.Box(c).Grow(1).FillHeight()
							})
							// Horizontal thirds
							ui.Column(c).Absolute().Fill().PassThrough().Children(func() {
								ui.Box(c).Grow(1).FillWidth()
								ui.Box(c).Height(1).FillWidth().Background(tok.Text.Alpha(0.16))
								ui.Box(c).Grow(1).FillWidth()
								ui.Box(c).Height(1).FillWidth().Background(tok.Text.Alpha(0.16))
								ui.Box(c).Grow(1).FillWidth()
							})
							// Subtle center crosshair + live tilt readout only when hardware telemetry runs
							ui.Box(c).Absolute().Fill().PassThrough().Center().Children(func() {
								ui.Column(c).AlignItems(ui.Center).Gap(float32(tok.Spacing4)).Children(func() {
									ui.Row(c).AlignItems(ui.Center).Gap(float32(tok.Spacing4)).Children(func() {
										ui.Box(c).Width(float32(tok.Spacing12)).Height(1).Background(tok.BorderStrong)
										ui.Box(c).Size(float32(tok.Spacing4), float32(tok.Spacing4)).Radius(float32(tok.PillRadius)).Background(tok.Accent)
										ui.Box(c).Width(float32(tok.Spacing12)).Height(1).Background(tok.BorderStrong)
									})
									if sess.telemetry.Running {
										dp, dr := sess.telemetry.PitchDeg, sess.telemetry.RollDeg
										if sess.hasRefPose {
											dp -= sess.refPitchDeg
											dr -= sess.refRollDeg
										}
										col := tok.TextMuted
										if math.Abs(dp) < 2.0 && math.Abs(dr) < 2.0 {
											col = tok.SuccessText
										}
										ui.Box(c).Padding(float32(tok.Spacing4)*0.5, float32(tok.Spacing8)).Radius(float32(tok.PillRadius)).Background(tok.Scrim).Children(func() {
											ui.Text(c, fmt.Sprintf("%+.1f° / %+.1f°", dp, dr)).FontSize(float32(tok.TextMeta)).TextColor(col).FontFeatures("tnum")
										})
									}
								})
							})
						})
					}

					// Minimal "1x" lens badge anchored at bottom center of viewfinder frame
					ui.Box(c).Absolute().Fill().PassThrough().
						Padding(0, 0, float32(tok.Spacing8), 0).Justify(ui.End).AlignItems(ui.Center).Children(func() {
						ui.Box(c).Padding(float32(tok.Spacing4)*0.5, float32(tok.Spacing8)).Radius(float32(tok.PillRadius)).
							Background(tok.Scrim).Border(1, tok.BorderSubtle).Children(func() {
							ui.Text(c, "1x").FontSize(float32(tok.TextMeta)).Bold().TextColor(tok.Text).FontFeatures("tnum")
						})
					})

					// Transient capture feedback toast (never permanently blocks center composition)
					if sess.toastText != "" && time.Now().Before(sess.toastUntil) {
						ui.Box(c).Absolute().Fill().PassThrough().
							Padding(float32(tok.Spacing12), 0, 0, 0).Justify(ui.Start).AlignItems(ui.Center).Children(func() {
							ui.Box(c).Padding(float32(tok.Spacing4), float32(tok.Spacing12)).
								Radius(float32(tok.PillRadius)).Background(tok.Scrim).Border(1, tok.Border).Children(func() {
								ui.Text(c, sess.toastText).FontSize(float32(tok.TextMeta)).TextColor(tok.Text).SingleLine().Ellipsis("…")
							})
						})
					}

					// Subtle shutter blink on capture
					if time.Now().Before(sess.flashUntil) {
						ui.Box(c).Absolute().Fill().PassThrough().Background(tok.Text.Alpha(0.24))
					}
				})

				// Compact Camera Settings drawer anchored at top of viewfinder when gear icon is open
				if sess.settingsOpen {
					ui.ButtonBase(c.Key("sci-cam-settings-backdrop")).Absolute().Fill().Label("关闭相机设置").
						OnClick(func() {
							sess.settingsOpen = false
						})
					ui.Box(c.Key("sci-cam-settings-panel")).Absolute().Fill().PassThrough().
						Padding(float32(tok.Spacing8), safeRight+float32(tok.Spacing12), 0, safeLeft+float32(tok.Spacing12)).
						AlignItems(ui.End).Justify(ui.Start).Children(func() {
						ui.Column(c).FillWidth().Padding(float32(tok.Spacing12)).
							Radius(float32(tok.PanelRadius)).Background(tok.Surface).Border(1, tok.Border).
							Gap(float32(tok.Spacing8)).Children(func() {
							ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
								ui.Text(c, "同步保存至系统相册").FontSize(float32(tok.TextMeta)).TextColor(tok.Text).Grow(1).MinWidth(0)
								lbl := "已开启"
								bg, fg := tok.Accent, tok.AccentText
								if !sess.saveToPhotos {
									lbl = "已关闭"
									bg, fg = tok.Elevated, tok.TextMuted
								}
								b := ui.ButtonBase(c.Key("sci-set-photos")).Height(float32(tok.Spacing24)+float32(tok.Spacing4)).
									Padding(0, float32(tok.Spacing12)).Radius(float32(tok.PillRadius)).
									Background(bg).Center().Label("切换系统相册同步").
									OnClick(func() {
										sess.saveToPhotos = !sess.saveToPhotos
										mygo.Haptics.Selection()
									})
								b.Children(func() {
									ui.Text(c, lbl).FontSize(float32(tok.TextMeta)).Bold().TextColor(fg).SingleLine()
								})
							})
							ui.Box(c).FillWidth().Height(1).Background(tok.BorderSubtle)
							ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
								ui.Text(c, "防频闪电网频率").FontSize(float32(tok.TextMeta)).TextColor(tok.Text).Grow(1).MinWidth(0)
								for _, hz := range []int{50, 60} {
									hz := hz
									bg, fg := tok.Elevated, tok.TextMuted
									if sess.flickerHz == hz {
										bg, fg = tok.Accent, tok.AccentText
									}
									b := ui.ButtonBase(c.Key(fmt.Sprintf("sci-set-hz-%d", hz))).Height(float32(tok.Spacing24)+float32(tok.Spacing4)).
										Padding(0, float32(tok.Spacing12)).Radius(float32(tok.ControlRadius)).
										Background(bg).Center().Label(fmt.Sprintf("%dHz", hz)).
										OnClick(func() {
											sess.flickerHz = hz
											mygo.Haptics.Selection()
										})
									b.Children(func() {
										ui.Text(c, fmt.Sprintf("%dHz", hz)).FontSize(float32(tok.TextMeta)).Bold().TextColor(fg).SingleLine()
									})
								}
							})
						})
					})
				}
			})

			// ============================================================
			// ZONE 3: OPTICAL PARAMETER DECK (Dial drawer + 5-column readouts)
			// ============================================================
			if sess.activeParam != "" {
				a.renderSciParamDrawer(c, tok, sess)
			}

			ui.Box(c).FillWidth().Height(1).Background(tok.BorderSubtle)
			ui.Row(c.Key("sci-cam-param-bar")).FillWidth().
				Padding(float32(tok.Spacing4), safeRight+float32(tok.Spacing8), float32(tok.Spacing4), safeLeft+float32(tok.Spacing8)).
				Background(tok.Canvas).Children(func() {
				paramCol := func(id, a11yTitle, title, val string, manual bool) {
					isOpen := sess.activeParam == id
					titleCol := tok.TextMuted
					valCol := tok.Text
					bg := tok.Canvas
					if manual {
						valCol = tok.Accent
					}
					if isOpen {
						bg = tok.Elevated
						titleCol = tok.Accent
						valCol = tok.Accent
					}
					b := ui.ButtonBase(c.Key("sci-param-"+id)).Grow(1).Basis(0).MinWidth(0).
						Height(float32(tok.Spacing48)).Radius(float32(tok.ControlRadius)).
						Background(bg).Center().Label("调节 " + a11yTitle).
						OnClick(func() {
							sess.settingsOpen = false
							if sess.activeParam == id {
								sess.activeParam = ""
							} else {
								sess.activeParam = id
							}
							mygo.Haptics.Selection()
						})
					theme.ApplyTactilePress(b)
					b.Children(func() {
						ui.Column(c).AlignItems(ui.Center).Gap(2).Children(func() {
							ui.Text(c, title).FontSize(float32(tok.TextMeta)).TextColor(titleCol).SingleLine()
							ui.Text(c, val).FontSize(float32(tok.TextData)).Bold().TextColor(valCol).FontFeatures("tnum").SingleLine().Ellipsis("…")
						})
					})
				}

				manualExp := !sess.isoAuto || !sess.shutterAuto || sess.allLocked

				isoTitle := "ISO"
				isoVal := "自动"
				if !sess.isoAuto && sess.selectedISO > 0 {
					isoVal = fmt.Sprintf("%.0f", sess.selectedISO)
				} else if sess.telemetry.ISO > 0 {
					if sess.allLocked {
						isoVal = fmt.Sprintf("%.0f", sess.telemetry.ISO)
					} else {
						isoTitle = "ISO·A"
						isoVal = fmt.Sprintf("%.0f", sess.telemetry.ISO)
					}
				}

				ssTitle := "SS"
				ssVal := "自动"
				if !sess.shutterAuto && sess.selectedShutterS > 0 {
					ssVal = formatShutterShort(sess.selectedShutterS)
				} else if sess.telemetry.ShutterS > 0 {
					if sess.allLocked {
						ssVal = formatShutterShort(sess.telemetry.ShutterS)
					} else {
						ssTitle = "SS·A"
						ssVal = formatShutterShort(sess.telemetry.ShutterS)
					}
				}

				evVal := "0.0"
				if manualExp {
					evVal = "—"
				} else if math.Abs(sess.evBias) >= 0.05 {
					evVal = fmt.Sprintf("%+.1f", sess.evBias)
				}

				wbTitle := "WB"
				wbVal := "AWB"
				switch sess.wbMode {
				case "lock_current":
					if sess.selectedWBKelvin > 0 {
						wbVal = fmt.Sprintf("%.0fK", sess.selectedWBKelvin)
					} else {
						wbVal = "锁定"
					}
				case "kelvin":
					wbVal = fmt.Sprintf("%.0fK", sess.selectedWBKelvin)
				default:
					if sess.telemetry.WBKelvin > 0 {
						wbTitle = "WB·A"
						wbVal = fmt.Sprintf("%.0fK", sess.telemetry.WBKelvin)
					}
				}

				mfVal := "AF"
				if !sess.focusAuto {
					mfVal = fmt.Sprintf("%.2f", sess.focusLensPos)
				} else if sess.focusLocked {
					if sess.telemetry.FocusLensPos > 0 {
						mfVal = fmt.Sprintf("%.2f", sess.telemetry.FocusLensPos)
					} else {
						mfVal = "锁定"
					}
				}

				paramCol("iso", "ISO", isoTitle, isoVal, !sess.isoAuto || sess.allLocked)
				paramCol("ss", "SS", ssTitle, ssVal, !sess.shutterAuto || sess.allLocked)
				paramCol("ev", "EV", "EV", evVal, !manualExp && math.Abs(sess.evBias) >= 0.05)
				paramCol("wb", "WB", wbTitle, wbVal, sess.wbMode != "auto")
				paramCol("mf", "MF", "MF", mfVal, !sess.focusAuto || sess.focusLocked)
			})

			// ============================================================
			// ZONE 4: BOTTOM SHUTTER DECK ([Thumbnail] | [Shutter] | [LOCK])
			// ============================================================
			ui.Row(c.Key("sci-cam-dock")).FillWidth().AlignItems(ui.Center).
				Padding(float32(tok.Spacing12), safeRight+float32(tok.Spacing16), safeBottom+float32(tok.Spacing12), safeLeft+float32(tok.Spacing16)).
				Background(tok.Canvas).Children(func() {
				// Left slot: Latest shot thumbnail -> opens Lab Gallery (Flow A)
				ui.Row(c).Grow(1).Basis(0).MinWidth(0).Justify(ui.Start).AlignItems(ui.Center).Children(func() {
					thumbPath := sess.lastPreviewAbs
					if thumbPath == "" && len(g.items) > 0 {
						thumbPath = g.items[0].PreviewAbs
					}
					galBtn := ui.ButtonBase(c.Key("sci-cam-gallery")).
						Size(float32(tok.Spacing48), float32(tok.Spacing48)).
						Radius(float32(tok.ControlRadius)).Background(tok.Elevated).
						Border(1, tok.Border).Clip().Center().Label("打开科研图库").
						OnClick(func() {
							mygo.Haptics.Selection()
							a.openLabGallery(&shell.WorkspaceTab{Kind: shell.TabFile, FilePath: sess.tabPath}, sess.applyDoc)
						})
					theme.ApplyTactilePress(galBtn)
					galBtn.Children(func() {
						if img := g.cachedThumb(thumbPath); img != nil {
							ui.Image(c, img).Fill()
						} else {
							ui.Icon(c, lucide.Images()).Size(float32(tok.IconLg), float32(tok.IconLg)).TextColor(tok.TextMuted)
						}
						if sess.shotCount > 0 {
							ui.Box(c).Absolute().Fill().PassThrough().Justify(ui.End).AlignItems(ui.End).
								Padding(float32(tok.Spacing4) * 0.5).Children(func() {
								ui.Box(c).Padding(0, float32(tok.Spacing4)).Radius(float32(tok.PillRadius)).
									Background(tok.Accent).Children(func() {
									ui.Text(c, fmt.Sprintf("%d", sess.shotCount)).FontSize(float32(tok.TextMeta)).Bold().TextColor(tok.AccentText).FontFeatures("tnum")
								})
							})
						}
					})
				})

				// Center slot: Classic double-ring shutter release
				shutterSize := float32(tok.Spacing48) + float32(tok.Spacing16)
				shutterRing := ui.ButtonBase(c.Key("sci-cam-shutter")).Size(shutterSize, shutterSize).
					Radius(float32(tok.PillRadius)).Background(tok.Canvas).Border(2, tok.Text).
					Padding(float32(tok.Spacing4)).Center().Label("拍摄").Disabled(sess.busy).
					OnClick(func() {
						a.triggerSciCapture()
					})
				theme.ApplyTactilePress(shutterRing)
				shutterRing.Children(func() {
					innerBg := tok.Text
					if sess.busy {
						innerBg = tok.TextMuted
					}
					ui.Box(c).Fill().Radius(float32(tok.PillRadius)).Background(innerBg)
				})

				// Right slot: Compact physical AE / AWB / AF Lock button
				ui.Row(c).Grow(1).Basis(0).MinWidth(0).Justify(ui.End).AlignItems(ui.Center).Children(func() {
					lockBg, lockFg := tok.Elevated, tok.Text
					lockLbl := "锁参"
					if sess.allLocked {
						lockBg, lockFg = tok.Accent, tok.AccentText
						lockLbl = "已锁定"
					}
					lockBtn := ui.ButtonBase(c.Key("sci-cam-lock")).Height(float32(tok.Spacing48)).
						Padding(0, float32(tok.Spacing12)).Radius(float32(tok.ControlRadius)).
						Background(lockBg).Center().Label("锁定曝光与白平衡").
						OnClick(func() { a.toggleSciBatchLock() })
					theme.ApplyTactilePress(lockBtn)
					lockBtn.Children(func() {
						ui.Row(c).AlignItems(ui.Center).Gap(float32(tok.Spacing4)).Children(func() {
							ui.Icon(c, lucide.Lock()).Size(float32(tok.IconSm), float32(tok.IconSm)).TextColor(lockFg)
							ui.Text(c, lockLbl).FontSize(float32(tok.TextMeta)).Bold().TextColor(lockFg).SingleLine()
						})
					})
				})
			})
		})
	})
}

func (a *app) renderSciParamDrawer(c *ui.Context, tok theme.Tokens, sess *sciCameraSession) {
	ui.Box(c).FillWidth().Height(1).Background(tok.BorderSubtle)
	ui.Column(c.Key("sci-cam-param-drawer")).FillWidth().
		Padding(float32(tok.Spacing8), float32(tok.Spacing12)).
		Background(tok.Surface).Gap(float32(tok.Spacing8)).Children(func() {
		chipBtn := func(key, lbl string, sel bool, onTap func()) {
			bg, fg := tok.Elevated, tok.Text
			if sel {
				bg, fg = tok.Accent, tok.AccentText
			}
			b := ui.ButtonBase(c.Key("sci-sub-"+key)).Height(float32(tok.Spacing32)).
				Padding(0, float32(tok.Spacing12)).Radius(float32(tok.ControlRadius)).
				Background(bg).Center().Label(lbl).OnClick(onTap)
			theme.ApplyTactilePress(b)
			b.Children(func() {
				ui.Text(c, lbl).FontSize(float32(tok.TextMeta)).Bold().TextColor(fg).FontFeatures("tnum").SingleLine()
			})
		}

		fitChipBtn := func(key, lbl string, sel bool, onTap func()) {
			bg, fg := tok.Elevated, tok.Text
			if sel {
				bg, fg = tok.Accent, tok.AccentText
			}
			b := ui.ButtonBase(c.Key("sci-sub-"+key)).Grow(1).Basis(0).MinWidth(0).
				Height(float32(tok.Spacing32)).Padding(0, float32(tok.Spacing4)).
				Radius(float32(tok.ControlRadius)).Background(bg).Center().Label(lbl).OnClick(onTap)
			theme.ApplyTactilePress(b)
			b.Children(func() {
				ui.Text(c, lbl).FontSize(float32(tok.TextMeta)).Bold().TextColor(fg).FontFeatures("tnum").SingleLine()
			})
		}

		switch sess.activeParam {
		case "iso":
			ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(float32(tok.Spacing4)).Children(func() {
				fitChipBtn("iso-auto", "自动", sess.isoAuto, func() {
					sess.isoAuto = true
					sess.selectedISO = 0
					a.syncManualExposureControl(sess)
					mygo.Haptics.Selection()
				})
				for _, v := range scicamera.DefaultISOPresets() {
					v := v
					sel := !sess.isoAuto && math.Abs(sess.selectedISO-v) < 1
					fitChipBtn(fmt.Sprintf("iso-%.0f", v), fmt.Sprintf("%.0f", v), sel, func() {
						sess.isoAuto = false
						sess.selectedISO = v
						a.syncManualExposureControl(sess)
						mygo.Haptics.Selection()
					})
				}
			})

		case "ss":
			ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(float32(tok.Spacing4)).Children(func() {
				fitChipBtn("ss-auto", "自动", sess.shutterAuto, func() {
					sess.shutterAuto = true
					sess.selectedShutterS = 0
					a.syncManualExposureControl(sess)
					mygo.Haptics.Selection()
				})
				for _, sp := range scicamera.AntiFlickerShutterPresets(sess.flickerHz) {
					if sp.AutoMode {
						continue
					}
					sp := sp
					sel := !sess.shutterAuto && math.Abs(sess.selectedShutterS-sp.Seconds) < 0.0002
					lbl := strings.TrimSuffix(sp.Label, "s")
					fitChipBtn("ss-"+sp.Label, lbl, sel, func() {
						sess.shutterAuto = false
						sess.selectedShutterS = sp.Seconds
						a.syncManualExposureControl(sess)
						mygo.Haptics.Selection()
					})
				}
			})

		case "ev":
			if !sess.isoAuto || !sess.shutterAuto || sess.allLocked {
				ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
					ui.Text(c, "手动曝光下 EV 不生效，请调 ISO / SS").
						FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted).Grow(1).MinWidth(0).SingleLine().Ellipsis("…")
					resetBtn := ui.ButtonBase(c.Key("sci-ev-to-auto")).Height(float32(tok.Spacing32)).
						Padding(0, float32(tok.Spacing12)).Radius(float32(tok.ControlRadius)).
						Background(tok.Accent).Center().Label("切回自动曝光").
						OnClick(func() {
							sess.isoAuto = true
							sess.shutterAuto = true
							sess.selectedISO = 0
							sess.selectedShutterS = 0
							sess.allLocked = false
							lock := false
							a.applySciControl(scicamera.ControlRequest{ExposureLocked: &lock})
							mygo.Haptics.Selection()
						})
					theme.ApplyTactilePress(resetBtn)
					resetBtn.Children(func() {
						ui.Text(c, "切回自动曝光").FontSize(float32(tok.TextMeta)).Bold().TextColor(tok.AccentText).SingleLine()
					})
				})
				break
			}
			ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
				chipBtn("ev-reset", "0.0", math.Abs(sess.evBias) < 0.05, func() {
					sess.evBias = 0
					ev := 0.0
					a.applySciControl(scicamera.ControlRequest{EVBias: &ev})
					mygo.Haptics.Selection()
				})
				ui.Box(c).Grow(1).MinWidth(0).Children(func() {
					s := components.Slider(c, &sess.evBias, -2.0, 2.0, components.SliderOptions{Step: 0.1})
					s.Label("曝光补偿 EV").FillWidth()
					if s.Changed() {
						ev := sess.evBias
						a.applySciControl(scicamera.ControlRequest{EVBias: &ev})
					}
				})
				ui.Text(c, fmt.Sprintf("%+.1f", sess.evBias)).FontSize(float32(tok.TextMeta)).Bold().TextColor(tok.Accent).FontFeatures("tnum").SingleLine()
			})

		case "wb":
			ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(float32(tok.Spacing4)).Children(func() {
				for _, wp := range scicamera.DefaultWBPresets() {
					wp := wp
					lbl := wp.Label
					if lbl == "定白锁定" {
						lbl = "锁定"
					}
					sel := sess.wbMode == wp.Mode && (wp.Mode != "kelvin" || math.Abs(sess.selectedWBKelvin-wp.Kelvin) < 50)
					fitChipBtn("wb-"+wp.Label, lbl, sel, func() {
						sess.wbMode = wp.Mode
						if wp.Kelvin > 0 {
							sess.selectedWBKelvin = wp.Kelvin
						}
						a.applySciControl(scicamera.ControlRequest{WBMode: wp.Mode, WBKelvin: wp.Kelvin, WBTint: wp.Tint})
						mygo.Haptics.Selection()
					})
				}
			})
			ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
				ui.Text(c, "2800K").FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted).FontFeatures("tnum").SingleLine()
				ui.Box(c).Grow(1).MinWidth(0).Children(func() {
					s := components.Slider(c, &sess.selectedWBKelvin, 2800, 7500, components.SliderOptions{Step: 100})
					s.Label("色温滑块").FillWidth()
					if s.Changed() {
						sess.wbMode = "kelvin"
						a.applySciControl(scicamera.ControlRequest{WBMode: "kelvin", WBKelvin: sess.selectedWBKelvin})
					}
				})
				ui.Text(c, fmt.Sprintf("%.0fK", sess.selectedWBKelvin)).FontSize(float32(tok.TextMeta)).Bold().TextColor(tok.Accent).FontFeatures("tnum").SingleLine()
			})

		case "mf":
			ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
				chipBtn("mf-auto", "AF", sess.focusAuto && !sess.focusLocked, func() {
					sess.focusAuto = true
					sess.focusLocked = false
					a.applySciControl(scicamera.ControlRequest{FocusMode: "auto"})
					mygo.Haptics.Selection()
				})
				chipBtn("mf-lock", "锁定", sess.focusLocked && sess.focusAuto, func() {
					sess.focusAuto = true
					sess.focusLocked = true
					a.applySciControl(scicamera.ControlRequest{FocusMode: "lock_current"})
					mygo.Haptics.Selection()
				})
				ui.Text(c, "近").FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted).SingleLine()
				ui.Box(c).Grow(1).MinWidth(0).Children(func() {
					s := components.Slider(c, &sess.focusLensPos, 0.0, 1.0, components.SliderOptions{Step: 0.02})
					s.Label("手动对焦 MF").FillWidth()
					if s.Changed() {
						sess.focusAuto = false
						sess.focusLocked = true
						a.applySciControl(scicamera.ControlRequest{FocusMode: "lens_pos", FocusLensPos: sess.focusLensPos})
					}
				})
				ui.Text(c, "∞").FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted).SingleLine()
			})
		}
	})
}

func (a *app) pickFromAlbumInLabGallery(c *ui.Context) {
	if a.vault == nil {
		return
	}
	paths, err := PickLabPhotos(context.Background(), 1)
	if err != nil || len(paths) == 0 {
		return
	}
	rel, iErr := a.vault.ImportAssetFile(paths[0])
	if iErr != nil {
		addPageToast(c, "导入照片失败："+iErr.Error(), components.ToastWarning)
		return
	}
	_, _ = a.generateAssetPreviewSidecar(rel)
	if a.labGallery != nil {
		a.labGallery.open = false
	}
	a.closeSciCamera()
	a.routeArchivedPhotoIntoDoc(c, rel, "", "")
}

func (a *app) renderLabGalleryOverlay(c *ui.Context) {
	g := a.labGallery
	if g == nil || !g.open {
		return
	}
	if g.dirty {
		a.refreshLabGallery()
	}

	tok := theme.GetTokens(c.Theme())
	safeTop, safeRight, safeBottom, safeLeft := c.SafeArea()

	filtered := make([]scicamera.GalleryItem, 0, len(g.items))
	var selectedItem *scicamera.GalleryItem
	for i := range g.items {
		it := &g.items[i]
		if g.filterRole != "" && it.Meta.Experiment.FrameRole != g.filterRole {
			continue
		}
		filtered = append(filtered, *it)
		if g.selectedID == it.LabPhotoID {
			selectedItem = it
		}
	}
	if selectedItem == nil && len(filtered) > 0 {
		selectedItem = &filtered[0]
		g.selectedID = selectedItem.LabPhotoID
	}

	ui.Overlay(c, func() {
		page := ui.Column(c.Key("lab-gallery-overlay")).Absolute().Fill().Modal().Background(tok.Canvas)
		page.Children(func() {
			// Header
			ui.Row(c).FillWidth().
				Padding(safeTop+float32(tok.Spacing8), safeRight+float32(tok.Spacing16), float32(tok.Spacing8), safeLeft+float32(tok.Spacing16)).
				AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Background(tok.Surface).Children(func() {
				if g.fromCamera {
					if components.Button(c.Key("gal-back-cam"), "返回拍摄", components.ButtonGhost).Clicked() {
						g.open = false
					}
				} else {
					kit.IconBox(c, tok, lucide.Images(), tok.Accent)
				}
				ui.Column(c).Grow(1).MinWidth(0).Children(func() {
					components.Text(c, "科研图库").FontSize(float32(tok.TextSection)).Bold().TextColor(tok.Text).SingleLine()
					components.Text(c, fmt.Sprintf("%d 组原片 · DNG", len(g.items))).
						FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted).FontFeatures("tnum").SingleLine().Ellipsis("…")
				})
				if !g.fromCamera {
					pickBtn := ui.ButtonBase(c.Key("gal-import-album")).Size(float32(tok.Spacing32)+float32(tok.Spacing4), float32(tok.Spacing32)+float32(tok.Spacing4)).
						Radius(float32(tok.ControlRadius)).Background(tok.Elevated).Center().Label("从相册导入").
						OnClick(func() { a.pickFromAlbumInLabGallery(c) })
					pickBtn.Children(func() {
						ui.Icon(c, lucide.ImagePlus()).Size(float32(tok.IconMd), float32(tok.IconMd)).TextColor(tok.Text)
					})
				}
				if len(g.items) > 0 {
					if components.Button(c.Key("gal-export-all"), "全部导出", components.ButtonSecondary).Clicked() {
						var allRaw []string
						for _, it := range g.items {
							if it.RawAbs != "" {
								allRaw = append(allRaw, it.RawAbs)
							} else if it.PreviewAbs != "" {
								allRaw = append(allRaw, it.PreviewAbs)
							}
						}
						a.shareLabFiles(c, allRaw, "AsterLab 科研原片批导出")
					}
				}
				closeLabel := "关闭"
				if g.fromCamera {
					closeLabel = "完成"
				}
				if components.Button(c.Key("gal-close"), closeLabel, components.ButtonGhost).Clicked() {
					g.open = false
					if g.fromCamera {
						a.closeSciCamera()
					}
				}
			})
			ui.Box(c).FillWidth().Height(1).Background(tok.BorderSubtle)

			// Filter bar
			ui.ScrollHorizontal(c.Key("gal-filter-scroll")).Scrollbars(false).FillWidth().Children(func() {
				ui.Row(c).Padding(float32(tok.Spacing8), safeRight+float32(tok.Spacing16), float32(tok.Spacing8), safeLeft+float32(tok.Spacing16)).
					Gap(float32(tok.Spacing8)).Children(func() {
					for _, f := range []struct{ id, lbl string }{
						{"", "全部"},
						{scicamera.RoleSample, "样本"},
						{scicamera.RoleWBRef, "灰卡"},
						{scicamera.RoleFlat, "平场"},
						{scicamera.RoleScaleRef, "标尺"},
					} {
						f := f
						bg, fg := tok.Elevated, tok.Text
						if g.filterRole == f.id {
							bg, fg = tok.Accent, tok.AccentText
						}
						b := ui.ButtonBase(c.Key("gal-flt-"+f.id)).Height(float32(tok.Spacing32)).
							Padding(0, float32(tok.Spacing12)).Radius(float32(tok.PillRadius)).
							Background(bg).Center().Label("筛选 " + f.lbl).OnClick(func() {
							g.filterRole = f.id
						})
						b.Children(func() {
							ui.Text(c, f.lbl).FontSize(float32(tok.TextMeta)).Bold().TextColor(fg).SingleLine()
						})
					}
				})
			})

			// Scrollable photo list + detail actions
			ui.Scroll(c.Key("gal-body-scroll")).FillWidth().Grow(1).Basis(0).MinHeight(0).Children(func() {
				ui.Column(c).FillWidth().
					Padding(float32(tok.Spacing8), safeRight+float32(tok.Spacing16), safeBottom+float32(tok.Spacing24), safeLeft+float32(tok.Spacing16)).
					Gap(float32(tok.Spacing12)).Children(func() {
					if len(filtered) == 0 {
						ui.Column(c).FillWidth().Padding(float32(tok.Spacing24)).Radius(float32(tok.PanelRadius)).
							Background(tok.Elevated).Center().Gap(float32(tok.Spacing8)).Children(func() {
							ui.Icon(c, lucide.Camera()).Size(float32(tok.Spacing32), float32(tok.Spacing32)).TextColor(tok.TextMuted)
							components.Text(c, "暂无带编号的科研原片").FontSize(float32(tok.TextBody)).Bold().TextColor(tok.Text)
							components.Text(c, "拍摄后会自动以规范编号保存 Bayer DNG 与 5 层光学元数据。").
								FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
						})
						return
					}

					if selectedItem != nil {
						it := *selectedItem
						ui.Column(c.Key("gal-detail-card")).FillWidth().Padding(float32(tok.Spacing12)).
							Radius(float32(tok.PanelRadius)).Background(tok.Elevated).Border(1, tok.Border).
							Gap(float32(tok.Spacing8)).Children(func() {
							ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
								components.Text(c, it.LabPhotoID).FontSize(float32(tok.TextBody)).Bold().TextColor(tok.Text).Grow(1).MinWidth(0).SingleLine().Ellipsis("…")
								if it.HasRaw {
									kit.Badge(c, "Bayer DNG", components.BadgeOptions{Tone: components.BadgeAccent})
								}
								kit.Badge(c, scicamera.RoleLabel(it.Meta.Experiment.FrameRole), components.BadgeOptions{Tone: components.BadgeNeutral})
							})
							if thumb := g.cachedThumb(it.PreviewAbs); thumb != nil {
								ui.Box(c).FillWidth().Height(float32(tok.Spacing48)*3.6).Radius(float32(tok.ControlRadius)).
									Background(tok.Canvas).Center().Clip().Children(func() {
									ui.Image(c, thumb).Height(float32(tok.Spacing48) * 3.6)
								})
							}
							expSpec := fmt.Sprintf("ISO %.0f · %s · %.0fK · 对焦 %.2f · 姿态 %.1f°/%.1f°",
								it.Meta.Exposure.ISO,
								formatShutterShort(it.Meta.Exposure.ShutterS),
								it.Meta.Exposure.WBKelvinEst,
								it.Meta.Geometry.FocusLensPosition,
								it.Meta.Geometry.IMUPitchDeg,
								it.Meta.Geometry.IMURollDeg,
							)
							components.Text(c, expSpec).FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted).FontFeatures("tnum")

							// Post-capture purpose/role annotation belongs here in Gallery, never in Camera HUD.
							ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(float32(tok.Spacing4)).Wrap().Children(func() {
								components.Text(c, "用途标注").FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
								for _, rp := range []struct{ id, lbl string }{
									{scicamera.RoleSample, "样本"},
									{scicamera.RoleWBRef, "灰卡"},
									{scicamera.RoleFlat, "平场"},
									{scicamera.RoleScaleRef, "标尺"},
								} {
									rp := rp
									bg, fg := tok.Surface, tok.TextMuted
									if it.Meta.Experiment.FrameRole == rp.id {
										bg, fg = tok.AccentSoftBg, tok.Accent
									}
									tb := ui.ButtonBase(c.Key("gal-tag-"+rp.id)).Height(float32(tok.Spacing24)).
										Padding(0, float32(tok.Spacing8)).Radius(float32(tok.PillRadius)).
										Background(bg).Center().Label("标注 " + rp.lbl).
										OnClick(func() {
											it.Meta.Experiment.FrameRole = rp.id
											metaPath := filepath.Join(filepath.Dir(it.PreviewAbs), it.LabPhotoID+".asterphoto.json")
											if err := scicamera.WriteSidecarJSON(metaPath, it.Meta); err == nil {
												g.dirty = true
											}
										})
									tb.Children(func() {
										ui.Text(c, rp.lbl).FontSize(float32(tok.TextMeta)).Bold().TextColor(fg).SingleLine()
									})
								}
							})

							ui.Row(c).FillWidth().Wrap().Gap(float32(tok.Spacing8)).Children(func() {
								if it.PreviewRel != "" && components.Button(c.Key("gal-act-insert"), "插入当前实验", components.ButtonPrimary).Clicked() {
									g.open = false
									a.closeSciCamera()
									a.routeArchivedPhotoIntoDoc(c, it.PreviewRel, it.RawAbs, it.LabPhotoID)
								}
								if components.Button(c.Key("gal-act-export"), "导出原片 (.DNG)", components.ButtonSecondary).Clicked() {
									var files []string
									if it.RawAbs != "" {
										files = append(files, it.RawAbs)
									}
									metaAbs := filepath.Join(filepath.Dir(it.PreviewAbs), it.LabPhotoID+".asterphoto.json")
									if _, err := os.Stat(metaAbs); err == nil {
										files = append(files, metaAbs)
									}
									if len(files) == 0 && it.PreviewAbs != "" {
										files = append(files, it.PreviewAbs)
									}
									a.shareLabFiles(c, files, it.LabPhotoID)
								}
								if components.Button(c.Key("gal-act-ghost"), "设为叠影并复用参数", components.ButtonGhost).Clicked() {
									sess := a.ensureSciCamera()
									g.open = false
									if a.shell != nil {
										if tab := a.shell.ActiveTab(); tab != nil && !sess.open {
											a.openSciCamera(c, *tab, nil, g.applyDoc)
										}
									}
									sess.ghostEnabled = true
									sess.ghostImagePath = it.PreviewAbs
									sess.refPitchDeg = it.Meta.Geometry.IMUPitchDeg
									sess.refRollDeg = it.Meta.Geometry.IMURollDeg
									sess.hasRefPose = true
									sess.isoAuto = false
									sess.selectedISO = it.Meta.Exposure.ISO
									sess.shutterAuto = false
									sess.selectedShutterS = it.Meta.Exposure.ShutterS
									sess.wbMode = "kelvin"
									sess.selectedWBKelvin = it.Meta.Exposure.WBKelvinEst
									sess.focusAuto = false
									sess.focusLocked = true
									sess.focusLensPos = it.Meta.Geometry.FocusLensPosition
									sess.allLocked = true
									lock := true
									a.applySciControl(scicamera.ControlRequest{
										ExposureLocked: &lock,
										ISO:            it.Meta.Exposure.ISO,
										ShutterS:       it.Meta.Exposure.ShutterS,
										WBMode:         "kelvin",
										WBKelvin:       it.Meta.Exposure.WBKelvinEst,
										FocusMode:      "lens_pos",
										FocusLensPos:   it.Meta.Geometry.FocusLensPosition,
									})
								}
							})
						})
					}

					// Compact grouped list of all matching lab shots (zero box-in-box)
					ui.Column(c.Key("gal-list-box")).FillWidth().Radius(float32(tok.PanelRadius)).
						Background(tok.Surface).Border(1, tok.Border).Clip().Children(func() {
						for i, it := range filtered {
							it := it
							if i > 0 {
								ui.Box(c).FillWidth().Height(1).Background(tok.BorderSubtle)
							}
							rowBg := tok.Surface
							if it.LabPhotoID == g.selectedID {
								rowBg = tok.AccentSoftBg
							}
							row := ui.ButtonBase(c.Key("gal-row-"+it.LabPhotoID)).FillWidth().
								MinHeight(float32(tok.Spacing48)).Padding(float32(tok.Spacing8), float32(tok.Spacing12)).
								Background(rowBg).Justify(ui.Start).Label(it.LabPhotoID).OnClick(func() {
								g.selectedID = it.LabPhotoID
							})
							row.Children(func() {
								ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
									kit.IconBox(c, tok, lucide.FileImage(), tok.Accent)
									ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
										ui.Text(c, it.LabPhotoID).FontSize(float32(tok.TextData)).Bold().TextColor(tok.Text).SingleLine().Ellipsis("…")
										ui.Text(c, fmt.Sprintf("%s · ISO %.0f · %s",
											scicamera.RoleLabel(it.Meta.Experiment.FrameRole),
											it.Meta.Exposure.ISO,
											formatShutterShort(it.Meta.Exposure.ShutterS))).
											FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted).FontFeatures("tnum").SingleLine()
									})
									if it.HasRaw {
										kit.Badge(c, "DNG", components.BadgeOptions{Tone: components.BadgeAccent})
									}
								})
							})
						}
					})
				})
			})
		})
	})
}

func (a *app) isExternalModalOrGalleryOpen() bool {
	return (a.sciCam != nil && a.sciCam.open) || (a.labGallery != nil && a.labGallery.open)
}
