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

	captureRaw       bool
	saveToPhotos     bool
	albumName        string
	ghostEnabled     bool
	ghostImagePath   string
	frameRole        string // sample | flat_field | wb_reference | scale_bar_ref
	batchLocked      bool
	refPitchDeg      float64
	refRollDeg       float64
	hasRefPose       bool
	flatFieldAssetID string
	batchRefAssetID  string

	activeDrawer     string // "" | "shutter" | "wb" | "role"
	selectedShutterS float64
	wbMode           string
	selectedWBKelvin float64
	busy             bool
	statusNote       string

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
			captureRaw:   true,
			saveToPhotos: true,
			albumName:    "AsterLab",
			frameRole:    scicamera.RoleSample,
			wbMode:       "auto",
			captureCh:    make(chan sciCaptureEvent, 2),
			telemCh:      make(chan sciTelemetryEvent, 4),
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

func (a *app) openSciCamera(c *ui.Context, curTab shell.WorkspaceTab, curLD *vault.LiveDocument, applyDoc func(string)) {
	sess := a.ensureSciCamera()
	if sess.cam != nil {
		sess.cam.Destroy()
		sess.cam = nil
	}
	sess.open = true
	sess.camInitFailed = false
	sess.tabPath = curTab.FilePath
	sess.applyDoc = applyDoc
	sess.dateYMD = ""
	sess.expTag = scicamera.SanitizeExpTag(strings.TrimSuffix(filepath.Base(curTab.FilePath), filepath.Ext(curTab.FilePath)))
	if curTab.Title != "" {
		if t := scicamera.SanitizeExpTag(curTab.Title); t != "LAB" {
			sess.expTag = t
		}
	}
	if sess.dateYMD == "" {
		sess.dateYMD = time.Now().Format("2006-01-02")
	}
	sess.batchID = fmt.Sprintf("batch-%s-%s", strings.ReplaceAll(sess.dateYMD, "-", ""), time.Now().Format("150405"))
	sess.shotCount = 0
	sess.batchRawPaths = nil
	sess.batchFiles = nil
	sess.activeDrawer = ""
	sess.busy = false
	sess.statusNote = "1x 物理主摄就绪 · 建议先用灰卡定白后锁参"
	sess.lastPollAt = time.Time{}

	if isMobileRuntime() && a.shell != nil && a.shell.MyGoWindow() != nil && sess.captureOverride == nil {
		cam, err := scicamera.New(a.shell.MyGoWindow(), scicamera.Options{})
		if err != nil {
			sess.camInitFailed = true
			sess.statusNote = "原生取景器启动失败：" + err.Error()
		} else {
			sess.cam = cam
		}
	}
	if c != nil {
		c.Invalidate()
	}
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
}

func (a *app) openLabGallery(curTab *shell.WorkspaceTab, applyDoc func(string)) {
	g := a.ensureLabGallery()
	g.open = true
	g.dirty = true
	if curTab != nil && curTab.Kind == shell.TabFile {
		g.tabPath = curTab.FilePath
	}
	if applyDoc != nil {
		g.applyDoc = applyDoc
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
	if g.selectedID == "" && len(g.items) > 0 {
		g.selectedID = g.items[0].LabPhotoID
	}
}

func (a *app) pollSciCamera(c *ui.Context) {
	sess := a.sciCam
	if sess == nil || !sess.open {
		return
	}
	// Drain completed captures.
	for {
		select {
		case ev := <-sess.captureCh:
			sess.busy = false
			if ev.err != nil {
				sess.statusNote = "拍摄失败：" + ev.err.Error()
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
	if sess.cam != nil {
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

func (a *app) toggleSciBatchLock() {
	sess := a.sciCam
	if sess == nil {
		return
	}
	sess.batchLocked = !sess.batchLocked
	if sess.batchLocked {
		lock := true
		req := scicamera.ControlRequest{
			ExposureLocked: &lock,
			FocusMode:      "lock_current",
		}
		if sess.telemetry.MinISO > 0 {
			req.ISO = sess.telemetry.MinISO
		}
		if sess.selectedShutterS > 0 {
			req.ShutterS = sess.selectedShutterS
		}
		if sess.wbMode == "auto" {
			req.WBMode = "lock_current"
			sess.wbMode = "lock_current"
		}
		a.applySciControl(req)
		if sess.telemetry.Running {
			sess.refPitchDeg = sess.telemetry.PitchDeg
			sess.refRollDeg = sess.telemetry.RollDeg
			sess.hasRefPose = true
		}
		sess.statusNote = "已锁死本批光学参数与机位倾角基准"
		mygo.Haptics.Success()
		return
	}
	lock := false
	sess.wbMode = "auto"
	sess.selectedShutterS = 0
	a.applySciControl(scicamera.ControlRequest{
		ExposureLocked: &lock,
		WBMode:         "auto",
		FocusMode:      "auto",
	})
	sess.statusNote = "已解锁参数，恢复自动测光与对白"
	mygo.Haptics.Selection()
}

func (a *app) triggerSciCapture() {
	sess := a.sciCam
	if sess == nil || sess.busy || a.vault == nil {
		return
	}
	if sess.cam == nil && sess.captureOverride == nil {
		sess.statusNote = "当前环境无物理相机，可使用右上角相册导入"
		return
	}
	labID := scicamera.NextLabPhotoID(a.vault.AssetsDir(), sess.dateYMD, sess.expTag, sess.frameRole)
	opts := scicamera.CaptureOptions{
		CaptureRaw:   sess.captureRaw,
		SaveToPhotos: sess.saveToPhotos,
		AlbumName:    sess.albumName,
		LabPhotoID:   labID,
	}
	sess.busy = true
	sess.statusNote = "正在捕获 " + labID + "…"
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
	labID := scicamera.NextLabPhotoID(a.vault.AssetsDir(), sess.dateYMD, sess.expTag, sess.frameRole)
	expMode := "auto"
	if res.Telemetry.ExposureLocked || sess.batchLocked {
		expMode = "locked"
	}
	wbMode := sess.wbMode
	if wbMode == "" {
		wbMode = "auto"
	}
	focusMode := "auto"
	if res.Telemetry.FocusLocked || sess.batchLocked {
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
			FlickerGridHz:      50,
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
			FrameRole:         sess.frameRole,
			BatchRefAssetID:   sess.batchRefAssetID,
			CapturedAt:        time.Now().Format(time.RFC3339),
			NonDestructiveOps: []string{},
		},
	}

	archived, err := scicamera.SaveCapturedTrio(a.vault.AssetsDir(), labID, res.PreviewPath, res.RawPath, meta)
	if err != nil {
		sess.statusNote = "归档原片失败：" + err.Error()
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
	if archived.RawAbs != "" {
		sess.batchRawPaths = append(sess.batchRawPaths, archived.RawAbs)
	}
	if archived.PreviewAbs != "" {
		sess.batchFiles = append(sess.batchFiles, archived.PreviewAbs)
	}
	if archived.MetaAbs != "" {
		sess.batchFiles = append(sess.batchFiles, archived.MetaAbs)
	}
	if sess.frameRole == scicamera.RoleFlat {
		sess.flatFieldAssetID = archived.LabPhotoID
	} else if sess.batchRefAssetID == "" {
		sess.batchRefAssetID = archived.LabPhotoID
		sess.ghostImagePath = archived.PreviewAbs
	}
	if sess.ghostImagePath == "" {
		sess.ghostImagePath = archived.PreviewAbs
	}

	rawTag := "JPG"
	if archived.RawAbs != "" {
		rawTag = "Bayer DNG + JPG"
	}
	photosNote := ""
	if res.PhotosAssetID != "" {
		photosNote = " · 已同步系统相册"
	}
	sess.statusNote = fmt.Sprintf("已编号归档 %s（%s%s）", archived.LabPhotoID, rawTag, photosNote)
	mygo.Haptics.Success()
	if c != nil {
		c.Invalidate()
	}
}

func (a *app) shareLabFiles(c *ui.Context, files []string, label string) {
	var clean []string
	for _, f := range files {
		if strings.TrimSpace(f) == "" {
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
	if sess != nil && sess.tabPath != "" {
		tabPath = sess.tabPath
		applyDoc = sess.applyDoc
	} else if a.labGallery != nil && a.labGallery.tabPath != "" {
		tabPath = a.labGallery.tabPath
		applyDoc = a.labGallery.applyDoc
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
	den := int(math.Round(1.0 / s))
	if den > 0 {
		return fmt.Sprintf("1/%d", den)
	}
	return fmt.Sprintf("%.2fs", s)
}

func (a *app) renderSciCameraOverlay(c *ui.Context) {
	sess := a.sciCam
	if sess == nil || !sess.open {
		return
	}
	a.pollSciCamera(c)

	tok := theme.DarkTokens()
	safeTop, safeRight, safeBottom, safeLeft := c.SafeArea()
	g := a.ensureLabGallery()

	ui.Overlay(c, func() {
		root := ui.Box(c.Key("sci-camera-overlay")).Absolute().Fill().Modal().Background(tok.Canvas)
		root.Children(func() {
			// 1. Live camera viewfinder layer underneath UI HUD.
			if sess.cam != nil && sess.cam.View() != nil {
				ui.WebView(c, sess.cam.View()).Fill()
			} else {
				ui.Box(c.Key("sci-cam-sim-viewfinder")).Fill().Center().Background(tok.Canvas).Children(func() {
					ui.Column(c).AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
						ui.Icon(c, lucide.Camera()).Size(float32(tok.Spacing32), float32(tok.Spacing32)).TextColor(tok.TextMuted)
						ui.Text(c, "科研取景器（连接物理主摄后实时显示）").FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
					})
				})
			}

			// 2. Non-destructive 22% Ghost Overlay from batch reference shot.
			if sess.ghostEnabled && sess.ghostImagePath != "" {
				if ghostImg := g.cachedThumb(sess.ghostImagePath); ghostImg != nil {
					ui.Box(c.Key("sci-cam-ghost-wrap")).Absolute().Fill().PassThrough().Center().Children(func() {
						ui.Image(c, ghostImg).Fill().Opacity(0.22)
					})
				}
			}

			// 3. Center subtle pose & alignment crosshair HUD (PassThrough so finger taps reach controls cleanly).
			ui.Box(c.Key("sci-cam-guide")).Absolute().Fill().PassThrough().Center().Children(func() {
				ui.Column(c).AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
					ui.Row(c).AlignItems(ui.Center).Gap(float32(tok.Spacing4)).Children(func() {
						ui.Box(c).Width(float32(tok.Spacing16)).Height(1).Background(tok.BorderStrong)
						ui.Box(c).Size(float32(tok.Spacing4), float32(tok.Spacing4)).Radius(float32(tok.PillRadius)).Background(tok.Accent)
						ui.Box(c).Width(float32(tok.Spacing16)).Height(1).Background(tok.BorderStrong)
					})
					if sess.hasRefPose && sess.telemetry.Running {
						dp := sess.telemetry.PitchDeg - sess.refPitchDeg
						dr := sess.telemetry.RollDeg - sess.refRollDeg
						poseText := fmt.Sprintf("姿态 Δ %.1f° / %.1f°", dp, dr)
						poseCol := tok.WarningText
						if math.Abs(dp) < 2.5 && math.Abs(dr) < 2.5 {
							poseText = "机位已对齐基准 ✓"
							poseCol = tok.SuccessText
						}
						ui.Box(c).Padding(float32(tok.Spacing4), float32(tok.Spacing8)).
							Radius(float32(tok.PillRadius)).Background(tok.Scrim).Children(func() {
							ui.Text(c, poseText).FontSize(float32(tok.TextMeta)).TextColor(poseCol).FontFeatures("tnum")
						})
					}
				})
			})

			// 4. Foreground Halide-inspired minimalist HUD (top narrow pill + bottom tactile dock).
			ui.Column(c.Key("sci-cam-hud")).Absolute().Fill().
				Padding(safeTop+float32(tok.Spacing8), safeRight+float32(tok.Spacing12), safeBottom+float32(tok.Spacing12), safeLeft+float32(tok.Spacing12)).
				Children(func() {
				// Top Control Capsule + optional compact 1-row dial strip
				ui.Column(c).FillWidth().Gap(float32(tok.Spacing8)).Children(func() {
					topBar := ui.Row(c.Key("sci-cam-top-bar")).FillWidth().AlignItems(ui.Center).Gap(float32(tok.Spacing8)).
						Padding(float32(tok.Spacing4), float32(tok.Spacing8)).
						Radius(float32(tok.PillRadius)).
						Background(tok.Surface.Alpha(0.92)).Border(1, tok.Border)
					topBar.Children(func() {
						closeBtn := ui.ButtonBase(c.Key("sci-cam-close")).Size(float32(tok.Spacing32), float32(tok.Spacing32)).
							Radius(float32(tok.PillRadius)).Center().Label("关闭科研相机").
							OnClick(func() {
								mygo.Haptics.Selection()
								a.closeSciCamera()
							})
						theme.ApplyTactilePress(closeBtn)
						closeBtn.Children(func() {
							ui.Icon(c, lucide.X()).Size(float32(tok.IconMd), float32(tok.IconMd)).TextColor(tok.Text)
						})

						// Scrollable compact parameter readout pills.
						ui.ScrollHorizontal(c.Key("sci-cam-pills-scroll")).Scrollbars(false).Grow(1).MinWidth(0).Children(func() {
							ui.Row(c).AlignItems(ui.Center).Gap(float32(tok.Spacing4)).Children(func() {
								hudChip := func(key, lbl string, active bool, onTap func()) {
									bg := tok.Elevated
									fg := tok.Text
									if active {
										bg = tok.AccentSoftBg
										fg = tok.Accent
									}
									b := ui.ButtonBase(c.Key("sci-chip-"+key)).
										Height(float32(tok.Spacing24)+float32(tok.Spacing4)).
										Padding(0, float32(tok.Spacing8)).Radius(float32(tok.PillRadius)).
										Background(bg).Center().Label(lbl).OnClick(onTap)
									theme.ApplyTactilePress(b)
									b.Children(func() {
										ui.Text(c, lbl).FontSize(float32(tok.TextMeta)).Bold().TextColor(fg).FontFeatures("tnum").SingleLine()
									})
								}

								rawLbl := "DNG"
								if !sess.captureRaw {
									rawLbl = "JPG"
								}
								hudChip("raw", rawLbl, sess.captureRaw, func() {
									sess.captureRaw = !sess.captureRaw
									mygo.Haptics.Selection()
								})

								shutterLbl := "快门 " + formatShutterShort(sess.selectedShutterS)
								if sess.selectedShutterS == 0 && sess.telemetry.ShutterS > 0 {
									shutterLbl = "快门 " + formatShutterShort(sess.telemetry.ShutterS)
								}
								hudChip("shutter", shutterLbl, sess.activeDrawer == "shutter", func() {
									if sess.activeDrawer == "shutter" {
										sess.activeDrawer = ""
									} else {
										sess.activeDrawer = "shutter"
									}
								})

								wbLbl := "WB 自动"
								switch sess.wbMode {
								case "lock_current":
									wbLbl = "WB 已锁定"
								case "kelvin":
									wbLbl = fmt.Sprintf("%.0fK", sess.selectedWBKelvin)
								}
								hudChip("wb", wbLbl, sess.activeDrawer == "wb", func() {
									if sess.activeDrawer == "wb" {
										sess.activeDrawer = ""
									} else {
										sess.activeDrawer = "wb"
									}
								})

								hudChip("role", "角色 "+scicamera.RoleLabel(sess.frameRole), sess.activeDrawer == "role", func() {
									if sess.activeDrawer == "role" {
										sess.activeDrawer = ""
									} else {
										sess.activeDrawer = "role"
									}
								})

								photosLbl := "同步相册"
								if !sess.saveToPhotos {
									photosLbl = "仅存库内"
								}
								hudChip("photos", photosLbl, sess.saveToPhotos, func() {
									sess.saveToPhotos = !sess.saveToPhotos
									mygo.Haptics.Selection()
								})

								hudChip("ghost", "叠影", sess.ghostEnabled, func() {
									sess.ghostEnabled = !sess.ghostEnabled
									mygo.Haptics.Selection()
								})
							})
						})

						// Fallback system album picker button
						pickBtn := ui.ButtonBase(c.Key("sci-cam-pick")).Size(float32(tok.Spacing32), float32(tok.Spacing32)).
							Radius(float32(tok.PillRadius)).Center().Label("从相册导入").
							OnClick(func() {
								mygo.Haptics.Selection()
								a.pickFromAlbumInSciCamera(c)
							})
						theme.ApplyTactilePress(pickBtn)
						pickBtn.Children(func() {
							ui.Icon(c, lucide.ImagePlus()).Size(float32(tok.IconMd), float32(tok.IconMd)).TextColor(tok.Text)
						})
					})

					// Single-row compact drawer under top capsule (opens only for the clicked parameter)
					if sess.activeDrawer != "" {
						strip := ui.ScrollHorizontal(c.Key("sci-cam-substrip")).Scrollbars(false).FillWidth()
						strip.Children(func() {
							ui.Row(c).Padding(float32(tok.Spacing4), float32(tok.Spacing8)).
								Radius(float32(tok.PanelRadius)).Background(tok.Surface.Alpha(0.95)).
								Border(1, tok.Border).Gap(float32(tok.Spacing4)).Children(func() {
								subBtn := func(key, lbl string, sel bool, onTap func()) {
									bg := tok.Elevated
									fg := tok.Text
									if sel {
										bg = tok.Accent
										fg = tok.AccentText
									}
									b := ui.ButtonBase(c.Key("sci-sub-"+key)).Height(float32(tok.Spacing32)).
										Padding(0, float32(tok.Spacing12)).Radius(float32(tok.ControlRadius)).
										Background(bg).Center().Label(lbl).OnClick(onTap)
									b.Children(func() {
										ui.Text(c, lbl).FontSize(float32(tok.TextMeta)).Bold().TextColor(fg).SingleLine()
									})
								}

								switch sess.activeDrawer {
								case "shutter":
									for _, sp := range scicamera.AntiFlickerShutterPresets(50) {
										sp := sp
										selected := (sp.AutoMode && sess.selectedShutterS == 0) || (!sp.AutoMode && math.Abs(sess.selectedShutterS-sp.Seconds) < 0.0002)
										subBtn("sh-"+sp.Label, sp.Label, selected, func() {
											sess.activeDrawer = ""
											if sp.AutoMode {
												sess.selectedShutterS = 0
												lock := false
												a.applySciControl(scicamera.ControlRequest{ExposureLocked: &lock})
											} else {
												sess.selectedShutterS = sp.Seconds
												lock := true
												iso := sess.telemetry.MinISO
												if iso <= 0 {
													iso = 50
												}
												a.applySciControl(scicamera.ControlRequest{ExposureLocked: &lock, ISO: iso, ShutterS: sp.Seconds})
											}
										})
									}
								case "wb":
									for _, wp := range scicamera.DefaultWBPresets() {
										wp := wp
										selected := sess.wbMode == wp.Mode && (wp.Mode != "kelvin" || math.Abs(sess.selectedWBKelvin-wp.Kelvin) < 1)
										subBtn("wb-"+wp.Label, wp.Label, selected, func() {
											sess.activeDrawer = ""
											sess.wbMode = wp.Mode
											sess.selectedWBKelvin = wp.Kelvin
											a.applySciControl(scicamera.ControlRequest{WBMode: wp.Mode, WBKelvin: wp.Kelvin, WBTint: wp.Tint})
										})
									}
								case "role":
									for _, rp := range []struct{ id, lbl string }{
										{scicamera.RoleSample, "样本 (SMP)"},
										{scicamera.RoleWBRef, "灰卡定白 (REF)"},
										{scicamera.RoleFlat, "空拍平场 (FLT)"},
										{scicamera.RoleScaleRef, "比例尺 (SCL)"},
									} {
										rp := rp
										subBtn("role-"+rp.id, rp.lbl, sess.frameRole == rp.id, func() {
											sess.frameRole = rp.id
											sess.activeDrawer = ""
										})
									}
								}
							})
						})
					}
				})

				ui.Spacer(c)

				// Status telemetry pill just above bottom dock
				if sess.statusNote != "" {
					ui.Row(c).FillWidth().Justify(ui.Center).Padding(0, 0, float32(tok.Spacing8), 0).Children(func() {
						ui.Box(c).Padding(float32(tok.Spacing4), float32(tok.Spacing12)).
							Radius(float32(tok.PillRadius)).Background(tok.Surface.Alpha(0.92)).Border(1, tok.Border).Children(func() {
							ui.Text(c, sess.statusNote).FontSize(float32(tok.TextMeta)).TextColor(tok.Text).SingleLine().Ellipsis("…")
						})
					})
				}

				// Bottom Tactile Dock: [科研图库 / 导出] | [快门] | [锁参 / 归档]
				dock := ui.Row(c.Key("sci-cam-dock")).FillWidth().AlignItems(ui.Center).
					Padding(float32(tok.Spacing12), float32(tok.Spacing16)).
					Radius(float32(tok.PanelRadius)).Background(tok.Surface.Alpha(0.94)).Border(1, tok.Border).
					Gap(float32(tok.Spacing8))
				dock.Children(func() {
					ui.Row(c).Grow(1).MinWidth(0).AlignItems(ui.Center).Gap(float32(tok.Spacing4)).Children(func() {
						galBtn := ui.ButtonBase(c.Key("sci-cam-gallery")).Height(float32(tok.Spacing32)+float32(tok.Spacing8)).
							Padding(0, float32(tok.Spacing8)).Radius(float32(tok.ControlRadius)).
							Background(tok.Elevated).Center().Label("打开科研图库").
							OnClick(func() {
								a.openLabGallery(&shell.WorkspaceTab{Kind: shell.TabFile, FilePath: sess.tabPath}, sess.applyDoc)
							})
						theme.ApplyTactilePress(galBtn)
						galBtn.Children(func() {
							ui.Row(c).AlignItems(ui.Center).Gap(float32(tok.Spacing4)).Children(func() {
								ui.Icon(c, lucide.Images()).Size(float32(tok.IconSm), float32(tok.IconSm)).TextColor(tok.Text)
								lbl := "图库"
								if sess.shotCount > 0 {
									lbl = fmt.Sprintf("图库 %d", sess.shotCount)
								}
								ui.Text(c, lbl).FontSize(float32(tok.TextMeta)).Bold().TextColor(tok.Text).FontFeatures("tnum").SingleLine()
							})
						})

						if len(sess.batchFiles) > 0 {
							expBtn := ui.ButtonBase(c.Key("sci-cam-export-raw")).Height(float32(tok.Spacing32)+float32(tok.Spacing8)).
								Padding(0, float32(tok.Spacing8)).Radius(float32(tok.ControlRadius)).
								Background(tok.Elevated).Center().Label("导出原片").
								OnClick(func() {
									files := append([]string(nil), sess.batchRawPaths...)
									if len(files) == 0 {
										files = append([]string(nil), sess.batchFiles...)
									}
									a.shareLabFiles(c, files, "AsterLab 科研原片 "+sess.batchID)
								})
							theme.ApplyTactilePress(expBtn)
							expBtn.Children(func() {
								ui.Icon(c, lucide.Share2()).Size(float32(tok.IconSm), float32(tok.IconSm)).TextColor(tok.Accent)
							})
						}
					})

					// Center Primary Shutter Button
					shutterSize := float32(tok.Spacing48) + float32(tok.Spacing12)
					shutterBtn := ui.ButtonBase(c.Key("sci-cam-shutter")).Size(shutterSize, shutterSize).
						Radius(float32(tok.PillRadius)).Background(tok.Text).Border(3, tok.Accent).
						Center().Label("拍摄科研照片").Disabled(sess.busy).
						OnClick(func() {
							a.triggerSciCapture()
						})
					theme.ApplyTactilePress(shutterBtn)
					shutterBtn.Children(func() {
						ui.Icon(c, lucide.Camera()).Size(float32(tok.IconLg), float32(tok.IconLg)).TextColor(tok.Canvas)
					})

					ui.Row(c).Grow(1).MinWidth(0).Justify(ui.End).AlignItems(ui.Center).Gap(float32(tok.Spacing4)).Children(func() {
						lockBg := tok.Elevated
						lockFg := tok.Text
						lockLbl := "锁参"
						if sess.batchLocked {
							lockBg = tok.Accent
							lockFg = tok.AccentText
							lockLbl = "已锁定"
						}
						lockBtn := ui.ButtonBase(c.Key("sci-cam-lock")).Height(float32(tok.Spacing32)+float32(tok.Spacing8)).
							Padding(0, float32(tok.Spacing8)).Radius(float32(tok.ControlRadius)).
							Background(lockBg).Center().Label("锁定本批拍摄参数").
							OnClick(func() { a.toggleSciBatchLock() })
						theme.ApplyTactilePress(lockBtn)
						lockBtn.Children(func() {
							ui.Row(c).AlignItems(ui.Center).Gap(float32(tok.Spacing4)).Children(func() {
								ui.Icon(c, lucide.Lock()).Size(float32(tok.IconSm), float32(tok.IconSm)).TextColor(lockFg)
								ui.Text(c, lockLbl).FontSize(float32(tok.TextMeta)).Bold().TextColor(lockFg).SingleLine()
							})
						})

						if sess.lastPreviewRel != "" {
							doneBtn := ui.ButtonBase(c.Key("sci-cam-route-done")).Height(float32(tok.Spacing32)+float32(tok.Spacing8)).
								Padding(0, float32(tok.Spacing12)).Radius(float32(tok.ControlRadius)).
								Background(tok.Success).Center().Label("归档入实验").
								OnClick(func() {
									rel := sess.lastPreviewRel
									rawAbs := sess.lastRawAbs
									labID := ""
									if sess.batchRefAssetID != "" {
										labID = sess.batchRefAssetID
									}
									a.closeSciCamera()
									a.routeArchivedPhotoIntoDoc(c, rel, rawAbs, labID)
								})
							theme.ApplyTactilePress(doneBtn)
							doneBtn.Children(func() {
								ui.Text(c, "归档").FontSize(float32(tok.TextMeta)).Bold().TextColor(tok.AccentText).SingleLine()
							})
						}
					})
				})
			})
		})
	})
}

func (a *app) pickFromAlbumInSciCamera(c *ui.Context) {
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
				kit.IconBox(c, tok, lucide.Images(), tok.Accent)
				ui.Column(c).Grow(1).MinWidth(0).Children(func() {
					components.Text(c, "科研图库").FontSize(float32(tok.TextSection)).Bold().TextColor(tok.Text).SingleLine()
					components.Text(c, fmt.Sprintf("%d 组原片记录 · DNG", len(g.items))).
						FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted).FontFeatures("tnum").SingleLine().Ellipsis("…")
				})
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
				if components.Button(c.Key("gal-close"), "关闭", components.ButtonGhost).Clicked() {
					g.open = false
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
						bg := tok.Elevated
						fg := tok.Text
						if g.filterRole == f.id {
							bg = tok.Accent
							fg = tok.AccentText
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
							components.Text(c, "使用科研相机拍摄后，会自动以规范编号保存 DNG 原片与 5 层光学元数据。").
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
							ui.Row(c).FillWidth().Wrap().Gap(float32(tok.Spacing8)).Children(func() {
								if it.PreviewRel != "" && components.Button(c.Key("gal-act-insert"), "插入当前实验", components.ButtonPrimary).Clicked() {
									g.open = false
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
									sess.ghostEnabled = true
									sess.ghostImagePath = it.PreviewAbs
									sess.refPitchDeg = it.Meta.Geometry.IMUPitchDeg
									sess.refRollDeg = it.Meta.Geometry.IMURollDeg
									sess.hasRefPose = true
									sess.selectedShutterS = it.Meta.Exposure.ShutterS
									sess.selectedWBKelvin = it.Meta.Exposure.WBKelvinEst
									g.open = false
									if a.shell != nil {
										if tab := a.shell.ActiveTab(); tab != nil {
											a.openSciCamera(c, *tab, nil, g.applyDoc)
											sess.ghostEnabled = true
											sess.ghostImagePath = it.PreviewAbs
											sess.selectedShutterS = it.Meta.Exposure.ShutterS
											sess.selectedWBKelvin = it.Meta.Exposure.WBKelvinEst
											sess.batchLocked = true
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
									}
								}
							})
						})
					}

					// Compact grouped list of all matching lab shots (zero box-in-box!)
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
