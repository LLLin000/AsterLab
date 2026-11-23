//go:build devdebug

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"asterlab-mygo-prototype/internal/devfeedback"
	"asterlab-mygo-prototype/internal/diag"
	"asterlab-mygo-prototype/internal/shell"
	"asterlab-mygo-prototype/internal/theme"
	"asterlab-mygo-prototype/internal/updater"

	mygo "github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/onevra/source/components"
)

// devInspectorState holds developer-only UI pick/annotate/export session state.
// Compiled ONLY with `-tags devdebug`; completely stripped from public releases.
type devInspectorState struct {
	pickActive      bool
	dragging        bool
	capturePending  bool
	hoverX          float32
	hoverY          float32
	startX          float32
	startY          float32
	curX            float32
	curY            float32
	delayUntil      time.Time
	delayScheduled  bool
	draftHydrated   bool
	resolvedOutBase string
	liveMDPath      string

	// Draft annotation popup after picking a UI element or dragging a pixel box.
	draftOpen       bool
	draftRect       devfeedback.Rect
	draftPickKind   string
	draftRegion     devfeedback.RegionHint
	draftWinW       float32
	draftWinH       float32
	draftImgW       int
	draftImgH       int
	draftFullPNG    []byte
	draftCropPNG    []byte
	draftCategory   string
	draftSeverity   string
	draftSuggestion string
	draftInspector  string
	draftCaptureErr string

	// Recorded session queue & export / remote upload status.
	issues              []devfeedback.Issue
	nextID              int
	queueOpen           bool
	testerName          string
	uploading           bool
	uploadID            string
	pendingCrashLog     string
	pendingCrashSummary string
	exportBanner        string
	exportZip           string
	exportErr           string
}

var devCategories = []string{"样式/间距", "文字裁切", "交互逻辑", "功能Bug"}
var devSeverities = []string{"P1", "P2", "P3"}

func (a *app) devInspectorPaletteCommands() []shell.PaletteCommand {
	return []shell.PaletteCommand{
		{
			ID:       "dev-ui-pick",
			Title:    "[DEV] 切换 UI 细节拾取标注模式 (Ctrl+Shift+D)",
			Subtitle: "点击或框选任意像素/组件细节，自动红框截图并实时落盘",
			OnRun: func() {
				a.toggleDevPickMode()
			},
		},
		{
			ID:       "dev-ui-delay-pick",
			Title:    "[DEV] 延迟 2 秒拾取（用于截取设置弹窗 / 下拉菜单 / 面板）",
			Subtitle: "2 秒后在所有打开的弹窗最顶层自动进入红框拾取模式",
			OnRun: func() {
				a.scheduleDelayedDevPick(2 * time.Second)
			},
		},
		{
			ID:       "dev-ui-queue",
			Title:    "[DEV] 打开 UI 调试反馈队列与导出面板",
			Subtitle: "查看实时落盘的反馈记录并一键打包 .zip",
			OnRun: func() {
				a.devInsp.pickActive = false
				a.devInsp.draftOpen = false
				a.devInsp.queueOpen = true
			},
		},
	}
}

func (a *app) toggleDevPickMode() {
	if a.devInsp.capturePending {
		return
	}
	if a.devInsp.pickActive {
		a.devInsp.pickActive = false
		a.devInsp.dragging = false
		return
	}
	a.devInsp.queueOpen = false
	a.devInsp.draftOpen = false
	a.devInsp.exportErr = ""
	a.devInsp.pickActive = true
	a.devInsp.dragging = false
}

// scheduleDelayedDevPick lets the developer open any modal dialog, dropdown menu,
// or hover popover (UI-13) and automatically activates the top-most modal pick overlay
// right over the open surface after delay.
func (a *app) scheduleDelayedDevPick(delay time.Duration) {
	st := &a.devInsp
	st.queueOpen = false
	st.draftOpen = false
	st.pickActive = false
	st.exportErr = ""
	st.delayUntil = time.Now().Add(delay)
	st.delayScheduled = true
	st.exportBanner = fmt.Sprintf("%.0f 秒后自动拾取：请立刻打开目标弹窗 / 面板…", delay.Seconds())
	time.AfterFunc(delay, func() {
		defer diag.RecoverAndLog("dev-delay-pick")
		mygo.RunOnMain(func() {
			defer diag.RecoverAndLog("dev-delay-pick-main")
			if !a.devInsp.delayScheduled {
				return
			}
			a.devInsp.delayScheduled = false
			a.devInsp.delayUntil = time.Time{}
			a.devInsp.exportBanner = ""
			a.devInsp.pickActive = true
			a.devInsp.dragging = false
			if a.shell != nil {
				a.shell.Invalidate()
			}
		})
	})
}

func (a *app) resolveDevFeedbackDir() (string, error) {
	if a.devInsp.resolvedOutBase != "" {
		if dir, err := devfeedback.ResolveWritableOutputDir(a.devInsp.resolvedOutBase); err == nil {
			return dir, nil
		}
	}
	var candidates []string
	// 1. Right beside asterlab-dev executable (always visible, writable, immune to Desktop/OneDrive DLP)
	if exePath, err := os.Executable(); err == nil && strings.TrimSpace(exePath) != "" {
		candidates = append(candidates, filepath.Join(filepath.Dir(exePath), "feedback"))
	}
	// 2. Inside active Vault `.asterlab/dev-feedback`
	if a.vault != nil && strings.TrimSpace(a.vault.Root) != "" {
		candidates = append(candidates, filepath.Join(a.vault.Root, ".asterlab", "dev-feedback"))
	}
	// 3. App data dir
	if dataDir := workspaceDataDir(); strings.TrimSpace(dataDir) != "" {
		candidates = append(candidates, filepath.Join(dataDir, "dev-feedback"))
	}
	if localApp := strings.TrimSpace(os.Getenv("LOCALAPPDATA")); localApp != "" {
		candidates = append(candidates, filepath.Join(localApp, "AsterLab", "dev-feedback"))
	}
	// 4. Temp fallback
	candidates = append(candidates, filepath.Join(os.TempDir(), "AsterLab-DevFeedback"))

	dir, err := devfeedback.ResolveWritableOutputDir(candidates...)
	if err == nil {
		a.devInsp.resolvedOutBase = dir
	}
	return dir, err
}

func (a *app) hydrateDevDraftOnce() {
	if a.devInsp.draftHydrated {
		return
	}
	a.devInsp.draftHydrated = true
	baseDir, err := a.resolveDevFeedbackDir()
	if err == nil {
		if rawName, rErr := os.ReadFile(filepath.Join(baseDir, "tester.txt")); rErr == nil {
			a.devInsp.testerName = strings.TrimSpace(string(rawName))
		}
		liveDir := devfeedback.LiveSessionDir(baseDir)
		restored, lErr := devfeedback.LoadRealtimeSession(liveDir)
		if lErr != nil || len(restored) == 0 {
			restored, _ = devfeedback.LoadDraft(baseDir)
		}
		if len(restored) > 0 {
			a.devInsp.issues = restored
			a.devInsp.liveMDPath = filepath.Join(liveDir, "feedback.md")
			maxNum := len(restored)
			for _, it := range restored {
				if strings.HasPrefix(it.ID, "UI-") {
					if n, convErr := strconv.Atoi(strings.TrimPrefix(it.ID, "UI-")); convErr == nil && n > maxNum {
						maxNum = n
					}
				}
			}
			a.devInsp.nextID = maxNum
			a.devInsp.exportBanner = fmt.Sprintf("已自动恢复上次落盘的 %d 条记录", len(restored))
		}
	}
	if rec := diag.PendingCrash(""); rec != nil {
		a.devInsp.pendingCrashSummary = rec.Summary
		a.devInsp.pendingCrashLog = diag.ReadLatestCrashLog("")
		if a.devInsp.pendingCrashLog != "" {
			if len(a.devInsp.issues) > 0 {
				a.devInsp.exportBanner = fmt.Sprintf("⚠ 检测到上次异常退出，已保留 %d 条批注与崩溃堆栈 → 点「队列」一键上报", len(a.devInsp.issues))
			} else {
				a.devInsp.exportBanner = "⚠ 检测到上次异常退出，已捕获崩溃堆栈 → 点「队列」一键上报开发者"
			}
		}
	}
}

func (a *app) buildDevSessionMeta(winW, winH float32, tok theme.Tokens) devfeedback.SessionMeta {
	pageName, _, tabTitle, activeFile, activeMode, _, _, _, _, uiScale := a.devContextSnapshot(winW, winH, tok)
	vaultRoot := ""
	if a.vault != nil {
		vaultRoot = a.vault.Root
	}
	return devfeedback.SessionMeta{
		SessionID:   time.Now().Format("20060102-150405"),
		ExportedAt:  time.Now().Format(time.RFC3339),
		TesterName:  strings.TrimSpace(a.devInsp.testerName),
		AppVersion:  updater.Version,
		Commit:      updater.Commit,
		OS:          runtime.GOOS,
		Arch:        runtime.GOARCH,
		WindowW:     winW,
		WindowH:     winH,
		UIScale:     uiScale,
		DarkTheme:   tok.Dark,
		VaultRoot:   diag.SanitizeText(vaultRoot),
		ActiveTab:   tabTitle,
		ActiveFile:  activeFile,
		ActiveMode:  activeMode,
		PageName:    pageName,
		HasCrashLog: strings.TrimSpace(a.devInsp.pendingCrashLog) != "",
		CrashLog:    a.devInsp.pendingCrashLog,
	}
}

func tabKindString(k shell.TabKind) string {
	switch k {
	case shell.TabCalendar:
		return "calendar"
	case shell.TabFile:
		return "file"
	case shell.TabPantry:
		return "pantry"
	default:
		return ""
	}
}

func viewModeString(m shell.ViewMode) string {
	switch m {
	case shell.ModeEdit:
		return "live"
	case shell.ModeLab:
		return "lab"
	case shell.ModeSource:
		return "source"
	default:
		return "live"
	}
}

func (a *app) devContextSnapshot(winW, winH float32, tok theme.Tokens) (pageName, tabKind, tabTitle, activeFile, activeMode string, leftW, rightW float32, leftOpen, rightOpen bool, uiScale float32) {
	uiScale = 1.0
	leftW = 284
	rightW = float32(tok.Spacing48) * 6.2
	if winW > 0 && winW < 1100 {
		leftW = 220
		rightW = 220
	}
	if a.shell == nil {
		return
	}
	pageName = a.shell.PageName()
	leftOpen = a.shell.LeftExpanded()
	rightOpen = a.shell.RightOpen
	if s := theme.EffectiveScale(); s > 0 {
		uiScale = s
	}
	if tab := a.shell.ActiveTab(); tab != nil {
		tabKind = tabKindString(tab.Kind)
		tabTitle = tab.Title
		activeFile = tab.FilePath
		activeMode = viewModeString(tab.Mode)
		if a.vault != nil && activeFile != "" {
			if rel, err := filepath.Rel(a.vault.Root, activeFile); err == nil {
				activeFile = filepath.ToSlash(rel)
			}
		}
	}
	return
}

// beginDraftFromPickAsync exits pick mode cleanly FIRST, waits one frame so MyGo finishes
// HandleInput and removes the crosshair overlay (preventing CapturePage() re-entrancy crashes!),
// captures the unobstructed window screenshot safely off-event, annotates off the UI thread,
// and opens the draft annotation card on the main thread.
func (a *app) beginDraftFromPickAsync(rect devfeedback.Rect, pickKind string, winW, winH float32, tok theme.Tokens) {
	st := &a.devInsp
	st.pickActive = false
	st.dragging = false
	st.capturePending = true
	st.draftRect = rect
	st.draftPickKind = pickKind
	st.draftWinW = winW
	st.draftWinH = winH
	st.draftCategory = "样式/间距"
	st.draftSeverity = "P2"
	st.draftSuggestion = ""
	st.draftInspector = ""
	st.draftCaptureErr = ""
	st.draftFullPNG = nil
	st.draftCropPNG = nil
	st.draftImgW = 0
	st.draftImgH = 0

	pageName, tabKind, _, _, _, leftW, rightW, leftOpen, rightOpen, _ := a.devContextSnapshot(winW, winH, tok)
	cx := rect.X + rect.W*0.5
	cy := rect.Y + rect.H*0.5
	st.draftRegion = devfeedback.DetectRegion(cx, cy, winW, winH, leftW, rightW, leftOpen, rightOpen, pageName, tabKind)

	var win *mygo.Window
	if a.shell != nil {
		win = a.shell.MyGoWindow()
	}
	if win == nil {
		st.capturePending = false
		st.draftCaptureErr = "无头窗口模式（未捕获像素截图，仍可记录坐标与建议）"
		st.draftOpen = true
		return
	}

	// Capture safely AFTER HandleInput returns and overlay unmounts (fixes MyGo re-entrancy crash!).
	go func() {
		defer diag.RecoverAndLog("dev-async-capture")
		time.Sleep(60 * time.Millisecond)

		var (
			fullPNG, cropPNG []byte
			imgW, imgH       int
			capErrStr        string
		)
		rawPNG, err := win.CapturePage()
		if err != nil {
			capErrStr = fmt.Sprintf("窗口截图失败: %v", err)
		} else if len(rawPNG) > 0 {
			fPNG, cPNG, w, h, annErr := devfeedback.AnnotateAndCrop(rawPNG, rect, winW, winH)
			if annErr != nil {
				capErrStr = fmt.Sprintf("红框标注失败: %v", annErr)
			} else {
				fullPNG, cropPNG, imgW, imgH = fPNG, cPNG, w, h
			}
		}

		mygo.RunOnMain(func() {
			defer diag.RecoverAndLog("dev-async-capture-main")
			a.devInsp.capturePending = false
			a.devInsp.draftFullPNG = fullPNG
			a.devInsp.draftCropPNG = cropPNG
			a.devInsp.draftImgW = imgW
			a.devInsp.draftImgH = imgH
			a.devInsp.draftCaptureErr = capErrStr
			a.devInsp.draftOpen = true
			if a.shell != nil {
				a.shell.Invalidate()
			}
		})
	}()
}

func (a *app) commitDraftIssue(tok theme.Tokens) {
	st := &a.devInsp
	st.nextID++
	pageName, _, tabTitle, activeFile, activeMode, _, _, _, rightOpen, uiScale := a.devContextSnapshot(st.draftWinW, st.draftWinH, tok)
	it := devfeedback.Issue{
		ID:               fmt.Sprintf("UI-%02d", st.nextID),
		CreatedAt:        time.Now().Format(time.RFC3339),
		Category:         st.draftCategory,
		Severity:         st.draftSeverity,
		PickKind:         st.draftPickKind,
		RectDIP:          st.draftRect,
		WindowW:          st.draftWinW,
		WindowH:          st.draftWinH,
		ImageW:           st.draftImgW,
		ImageH:           st.draftImgH,
		Region:           st.draftRegion,
		PageName:         pageName,
		ActiveTab:        tabTitle,
		ActiveFile:       activeFile,
		ActiveMode:       activeMode,
		RightDrawerOpen:  rightOpen,
		DarkTheme:        tok.Dark,
		UIScale:          uiScale,
		InspectorSnippet: strings.TrimSpace(st.draftInspector),
		Suggestion:       strings.TrimSpace(st.draftSuggestion),
		FullPNG:          st.draftFullPNG,
		CropPNG:          st.draftCropPNG,
	}
	st.issues = append(st.issues, it)
	st.draftOpen = false

	// Immediately persist this issue + PNGs + feedback.md to disk (never silently swallow errors!).
	baseDir, dirErr := a.resolveDevFeedbackDir()
	if dirErr != nil {
		st.exportErr = fmt.Sprintf("实时落盘失败（仅存内存）: %v", dirErr)
		st.exportBanner = ""
		return
	}
	liveDir := devfeedback.LiveSessionDir(baseDir)
	sess := a.buildDevSessionMeta(st.draftWinW, st.draftWinH, tok)
	mdPath, saveErr := devfeedback.AppendRealtimeIssue(liveDir, sess, st.issues, it)
	if saveErr != nil {
		st.exportErr = fmt.Sprintf("实时落盘失败（仅存内存）: %v", saveErr)
		st.exportBanner = ""
		return
	}
	st.liveMDPath = mdPath
	st.exportErr = ""
	st.exportBanner = fmt.Sprintf("✓ 已实时落盘 %s（共 %d 条）→ %s", it.ID, len(st.issues), liveDir)
}

func (a *app) syncLiveSessionAfterMutation(c *ui.Context, tok theme.Tokens) {
	baseDir, err := a.resolveDevFeedbackDir()
	if err != nil {
		a.devInsp.exportErr = fmt.Sprintf("同步落盘失败: %v", err)
		return
	}
	winW, winH := c.Size()
	liveDir := devfeedback.LiveSessionDir(baseDir)
	if _, err := devfeedback.SyncRealtimeSession(liveDir, a.buildDevSessionMeta(winW, winH, tok), a.devInsp.issues); err != nil {
		a.devInsp.exportErr = fmt.Sprintf("同步落盘失败: %v", err)
	}
}

func (a *app) cleanupDevTempFiles(tempFolder, tempZip string, clearLiveSession bool) {
	if strings.TrimSpace(tempFolder) != "" {
		_ = os.RemoveAll(tempFolder)
	}
	if strings.TrimSpace(tempZip) != "" {
		_ = os.Remove(tempZip)
	}
	if !clearLiveSession {
		return
	}
	if baseDir, err := a.resolveDevFeedbackDir(); err == nil && baseDir != "" {
		_ = os.RemoveAll(devfeedback.LiveSessionDir(baseDir))
		_ = devfeedback.ClearDraft(baseDir)
	}
	a.devInsp.issues = nil
	a.devInsp.nextID = 0
	a.devInsp.liveMDPath = ""
	a.devInsp.exportZip = ""
}

func (a *app) exportDevFeedbackBundle(c *ui.Context, tok theme.Tokens) {
	st := &a.devInsp
	if len(st.issues) == 0 && strings.TrimSpace(st.pendingCrashLog) == "" {
		st.exportErr = "当前尚未记录任何 UI 问题，请先按 Ctrl+Shift+D 拾取页面元素"
		return
	}
	defaultName := "AsterLab-Feedback-" + time.Now().Format("20060102-150405") + ".zip"
	startDir := ""
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		startDir = filepath.Join(home, "Downloads", defaultName)
	}
	// Let the user choose where to save the exported .zip via native OS Save dialog.
	destZip, saveErr := mygo.Dialog.Save(mygo.SaveDialogOptions{
		Parent:      mygo.FocusedWindow(),
		Title:       "导出 UI 反馈包 (.zip)",
		DefaultPath: startDir,
		Filters:     []mygo.FileFilter{{Name: "ZIP 压缩包", Extensions: []string{"zip"}}},
	})
	if saveErr != nil {
		st.exportErr = fmt.Sprintf("打开保存对话框失败: %v", saveErr)
		return
	}
	destZip = strings.TrimSpace(destZip)
	if destZip == "" {
		return // user cancelled
	}
	if !strings.HasSuffix(strings.ToLower(destZip), ".zip") {
		destZip += ".zip"
	}

	tmpFolder, tmpZip, err := a.buildExportZipOnDisk(c, tok)
	if err != nil {
		st.exportErr = fmt.Sprintf("打包失败: %v", err)
		return
	}
	rawZip, rErr := os.ReadFile(tmpZip)
	if rErr != nil {
		a.cleanupDevTempFiles(tmpFolder, tmpZip, false)
		st.exportErr = fmt.Sprintf("读取临时包失败: %v", rErr)
		return
	}
	if wErr := os.WriteFile(destZip, rawZip, 0o644); wErr != nil {
		a.cleanupDevTempFiles(tmpFolder, tmpZip, false)
		st.exportErr = fmt.Sprintf("写入目标路径失败: %v", wErr)
		return
	}

	exportedCount := len(st.issues)
	hasCrash := strings.TrimSpace(st.pendingCrashLog) != ""
	// Clean temporary scratch once exported safely to user's chosen location.
	a.cleanupDevTempFiles(tmpFolder, tmpZip, true)
	if hasCrash {
		diag.DismissPendingCrash("")
		st.pendingCrashLog = ""
		st.pendingCrashSummary = ""
	}
	st.exportErr = ""
	clipErr := devfeedback.CopyFileToSystemClipboard(destZip)
	if clipErr == nil && (runtime.GOOS == "windows" || runtime.GOOS == "darwin") {
		st.exportBanner = fmt.Sprintf("✓ 已导出 %d 条至 %s 并复制文件（已清理临时缓存）", exportedCount, filepath.Base(destZip))
	} else {
		c.WriteClipboard(destZip)
		st.exportBanner = fmt.Sprintf("✓ 已导出 %d 条至 %s（已清理临时缓存）", exportedCount, filepath.Base(destZip))
	}
	revealInFileManager(destZip, filepath.Dir(destZip))
}

func (a *app) buildExportZipOnDisk(c *ui.Context, tok theme.Tokens) (string, string, error) {
	st := &a.devInsp
	winW, winH := c.Size()
	sess := a.buildDevSessionMeta(winW, winH, tok)
	outBase, dirErr := a.resolveDevFeedbackDir()
	if dirErr != nil {
		return "", "", fmt.Errorf("无法找到可写导出目录: %w", dirErr)
	}
	if strings.TrimSpace(st.testerName) != "" {
		_ = os.WriteFile(filepath.Join(outBase, "tester.txt"), []byte(strings.TrimSpace(st.testerName)+"\n"), 0o644)
	}
	rawLogs := diag.RecentLines(180)
	cleanLogs := make([]string, 0, len(rawLogs))
	for _, line := range rawLogs {
		cleanLogs = append(cleanLogs, diag.SanitizeText(line))
	}
	return devfeedback.ExportSession(outBase, sess, st.issues, cleanLogs)
}

func (a *app) devUploadConfigured() bool {
	endpoint := strings.TrimSpace(devfeedback.DefaultUploadEndpoint)
	if env := strings.TrimSpace(os.Getenv("ASTERLAB_FEEDBACK_ENDPOINT")); env != "" {
		endpoint = env
	}
	key := strings.TrimSpace(devfeedback.DefaultUploadKey)
	if env := strings.TrimSpace(os.Getenv("ASTERLAB_FEEDBACK_KEY")); env != "" {
		key = env
	}
	return endpoint != "" && key != ""
}

func (a *app) uploadDevFeedbackBundleAsync(c *ui.Context, tok theme.Tokens) {
	st := &a.devInsp
	if st.uploading {
		return
	}
	if len(st.issues) == 0 && strings.TrimSpace(st.pendingCrashLog) == "" {
		st.exportErr = "当前尚未记录任何 UI 问题或崩溃日志"
		return
	}
	endpoint := strings.TrimSpace(devfeedback.DefaultUploadEndpoint)
	if env := strings.TrimSpace(os.Getenv("ASTERLAB_FEEDBACK_ENDPOINT")); env != "" {
		endpoint = env
	}
	key := strings.TrimSpace(devfeedback.DefaultUploadKey)
	if env := strings.TrimSpace(os.Getenv("ASTERLAB_FEEDBACK_KEY")); env != "" {
		key = env
	}
	if endpoint == "" || key == "" {
		st.exportErr = "当前本地编译包未内置上报口令，请改用旁边的「导出 .zip…」发给开发者"
		return
	}

	tmpFolder, tmpZip, err := a.buildExportZipOnDisk(c, tok)
	if err != nil {
		st.exportErr = fmt.Sprintf("打包失败: %v", err)
		return
	}
	tester := strings.TrimSpace(st.testerName)
	if tester == "" {
		tester = "内测用户"
	}
	issueCount := len(st.issues)
	hasCrash := strings.TrimSpace(st.pendingCrashLog) != ""
	var sumLines []string
	for i, it := range st.issues {
		if i >= 5 {
			sumLines = append(sumLines, fmt.Sprintf("…共 %d 条", len(st.issues)))
			break
		}
		txt := strings.TrimSpace(it.Suggestion)
		if txt == "" {
			txt = it.Region.Label
		}
		sumLines = append(sumLines, fmt.Sprintf("- %s: %s", it.ID, txt))
	}
	summary := strings.Join(sumLines, "\n")

	st.uploading = true
	st.exportErr = ""
	st.exportBanner = "正在提交反馈包给开发者…"

	go func() {
		defer diag.RecoverAndLog("dev-feedback-upload")
		resp, upErr := devfeedback.UploadBundle(endpoint, key, tmpZip, tester, updater.Version, issueCount, hasCrash, summary)
		mygo.RunOnMain(func() {
			defer diag.RecoverAndLog("dev-feedback-upload-main")
			a.devInsp.uploading = false
			if upErr != nil {
				// Keep live-session intact as fallback; remove only the transient export folder/zip.
				a.cleanupDevTempFiles(tmpFolder, tmpZip, false)
				a.devInsp.exportErr = fmt.Sprintf("直传失败（本地暂存已保留，请改点「导出 .zip…」）: %v", upErr)
				a.devInsp.exportBanner = ""
			} else {
				// Upload confirmed -> purge temporary local scratch so disk never accumulates stale bundles.
				a.cleanupDevTempFiles(tmpFolder, tmpZip, true)
				a.devInsp.exportErr = ""
				a.devInsp.uploadID = resp.ID
				a.devInsp.exportBanner = fmt.Sprintf("✓ 已提交给开发者！编号 %s（共 %d 条，已清理本地缓存）", resp.ID, issueCount)
				if hasCrash {
					diag.DismissPendingCrash("")
					a.devInsp.pendingCrashLog = ""
					a.devInsp.pendingCrashSummary = ""
				}
			}
			if a.shell != nil {
				a.shell.Invalidate()
			}
		})
	}()
}

func revealInFileManager(zipPath, folderPath string) {
	switch runtime.GOOS {
	case "windows":
		if zipPath != "" {
			_ = exec.Command("explorer.exe", "/select,"+zipPath).Start()
		} else if folderPath != "" {
			_ = exec.Command("explorer.exe", folderPath).Start()
		}
	case "darwin":
		if zipPath != "" {
			_ = exec.Command("open", "-R", zipPath).Start()
		}
	default:
		if folderPath != "" {
			_ = exec.Command("xdg-open", folderPath).Start()
		}
	}
}

func (a *app) renderDevInspectorOverlay(c *ui.Context) {
	defer diag.RecoverAndLog("dev-overlay")
	a.hydrateDevDraftOnce()
	tok := theme.GetTokens(c.Theme())
	st := &a.devInsp

	// Global developer shortcuts: Ctrl+Shift+D toggles immediate UI pick mode;
	// Ctrl+Shift+Y triggers 2-second delayed pick for open modals/menus (UI-13).
	c.OnWindowShortcut(ui.Cmd|ui.Shift, ui.KeyD, func() {
		a.toggleDevPickMode()
	})
	c.OnWindowShortcut(ui.Cmd|ui.Shift, ui.KeyY, func() {
		a.scheduleDelayedDevPick(2 * time.Second)
	})
	if st.pickActive {
		c.OnWindowShortcut(0, ui.KeyEscape, func() {
			st.pickActive = false
			st.dragging = false
		})
	}

	// 1. Live Full-Window Pick / Drag-Box Capture Overlay (.Modal() sits ABOVE any open app Dialog!)
	if st.pickActive {
		a.renderDevPickOverlay(c, tok)
		return
	}

	// While async CapturePage() is grabbing the unobstructed window frame, hide DEV chrome completely!
	if st.capturePending {
		return
	}

	// 2. Floating Compact Dev Pill (bottom-right, devdebug builds ONLY)
	a.renderDevFloatingPill(c, tok)

	// 3 & 4. Always keep Dialog wrappers mounted (nil body when closed) so closing/canceling
	// completes MyGo's modal exit lifecycle cleanly without corrupting modal state.
	a.renderDevDraftDialog(c, tok)
	a.renderDevQueueDialog(c, tok)
}

func (a *app) renderDevPickOverlay(c *ui.Context, tok theme.Tokens) {
	st := &a.devInsp
	winW, winH := c.Size()
	pageName, tabKind, _, _, _, leftW, rightW, leftOpen, rightOpen, _ := a.devContextSnapshot(winW, winH, tok)

	ui.Overlay(c, func() {
		// .Modal() ensures this capture layer sits ABOVE open Settings / Pantry / Dialog panels (UI-13).
		capture := ui.Box(c.Key("dev-pick-surface")).Absolute().Left(0).Top(0).Right(0).Bottom(0).
			Modal().
			Background(tok.Accent.Alpha(0.03)).
			HandleInput(func(ev ui.InputEvent) bool {
				defer diag.RecoverAndLog("dev-pick-input")
				switch ev.Kind {
				case ui.InputPointerMove:
					st.hoverX, st.hoverY = ev.X, ev.Y
					if st.dragging {
						st.curX, st.curY = ev.X, ev.Y
					}
					return true
				case ui.InputPointerDown:
					if ev.Button != 0 {
						st.pickActive = false
						st.dragging = false
						return true
					}
					st.dragging = true
					st.startX, st.startY = ev.X, ev.Y
					st.curX, st.curY = ev.X, ev.Y
					st.hoverX, st.hoverY = ev.X, ev.Y
					return true
				case ui.InputPointerUp:
					if !st.dragging {
						return true
					}
					st.dragging = false
					st.curX, st.curY = ev.X, ev.Y
					dx := abs32Dev(st.curX - st.startX)
					dy := abs32Dev(st.curY - st.startY)
					pickKind := "click"
					rx, ry, rw, rh := st.startX, st.startY, float32(0), float32(0)
					if dx >= 6 || dy >= 6 {
						pickKind = "drag_box"
						rx = min32Dev(st.startX, st.curX)
						ry = min32Dev(st.startY, st.curY)
						rw = dx
						rh = dy
					}
					a.beginDraftFromPickAsync(devfeedback.Rect{X: rx, Y: ry, W: rw, H: rh}, pickKind, winW, winH, tok)
					return true
				}
				return false
			})

		capture.Children(func() {
			// Crisp crosshair guides following cursor.
			if st.hoverX > 0 && st.hoverY > 0 {
				ui.Box(c).Absolute().Left(0).Top(st.hoverY).FillWidth().Height(1).
					Background(tok.Danger.Alpha(0.35))
				ui.Box(c).Absolute().Left(st.hoverX).Top(0).Width(1).FillHeight().
					Background(tok.Danger.Alpha(0.35))
			}

			// Live red drag marquee rectangle.
			if st.dragging {
				rx := min32Dev(st.startX, st.curX)
				ry := min32Dev(st.startY, st.curY)
				rw := abs32Dev(st.curX - st.startX)
				rh := abs32Dev(st.curY - st.startY)
				if rw >= 4 || rh >= 4 {
					ui.Box(c).Absolute().Left(rx).Top(ry).Width(rw).Height(rh).
						Background(tok.Danger.Alpha(0.14)).
						Border(2, tok.Danger)
				}
			}

			// Top HUD banner showing live region & instructions.
			reg := devfeedback.DetectRegion(st.hoverX, st.hoverY, winW, winH, leftW, rightW, leftOpen, rightOpen, pageName, tabKind)
			hudText := fmt.Sprintf("DEV 拾取模式 · 点击或拖拽框选任意 UI 细节 · 当前: %s (%.0f, %.0f) · Esc / 右键退出",
				reg.Label, st.hoverX, st.hoverY)
			ui.Row(c).Absolute().Top(float32(tok.Spacing12)).Left(float32(tok.Spacing16)).
				Padding(float32(tok.Spacing8), float32(tok.Spacing12)).
				Radius(float32(tok.ControlRadius)).
				Background(tok.Danger).
				Children(func() {
					components.Text(c, hudText).FontSize(float32(tok.TextMeta)).Bold().TextColor(tok.Surface)
				})
		})
	})
}

func (a *app) renderDevFloatingPill(c *ui.Context, tok theme.Tokens) {
	st := &a.devInsp
	ui.Overlay(c, func() {
		ui.Row(c.Key("dev-floating-pill")).Absolute().
			Right(float32(tok.Spacing16)).Bottom(float32(tok.Spacing12)).
			Padding(float32(tok.Spacing4), float32(tok.Spacing8)).
			Radius(float32(tok.ControlRadius)).
			Background(tok.Surface).
			Border(1, tok.Accent.Alpha(0.75)).
			AlignItems(ui.Center).
			Gap(float32(tok.Spacing8)).
			Children(func() {
				components.Badge(c, "DEV", components.BadgeOptions{Tone: components.BadgeAccent})
				if components.Button(c, "拾取细节 (Ctrl+Shift+D)", components.ButtonGhost).Clicked() {
					a.toggleDevPickMode()
				}
				delayLbl := "延迟2s(截弹窗)"
				if st.delayScheduled {
					rem := time.Until(st.delayUntil).Seconds()
					if rem < 0 {
						rem = 0
					}
					delayLbl = fmt.Sprintf("%.1fs后拾取…", rem)
				}
				if components.Button(c, delayLbl, components.ButtonGhost).Clicked() {
					a.scheduleDelayedDevPick(2 * time.Second)
				}
				queueLabel := fmt.Sprintf("队列 (%d)", len(st.issues))
				if strings.TrimSpace(st.pendingCrashLog) != "" {
					queueLabel = fmt.Sprintf("⚠ 队列 (%d+崩溃)", len(st.issues))
				}
				if components.Button(c, queueLabel, components.ButtonSecondary).Clicked() {
					st.queueOpen = !st.queueOpen
				}
				if len(st.issues) > 0 || strings.TrimSpace(st.pendingCrashLog) != "" {
					if components.Button(c, "导出 .zip…", components.ButtonGhost).Clicked() && !st.uploading {
						a.exportDevFeedbackBundle(c, tok)
					}
					if a.devUploadConfigured() {
						btnLbl := "提交给开发者"
						if st.uploading {
							btnLbl = "上传中…"
						}
						if components.Button(c, btnLbl, components.ButtonPrimary).Clicked() && !st.uploading {
							a.uploadDevFeedbackBundleAsync(c, tok)
						}
					}
				}
				if st.exportBanner != "" {
					components.Text(c, st.exportBanner).FontSize(float32(tok.TextMeta)).TextColor(tok.Success)
				}
				if st.exportErr != "" {
					components.Text(c, st.exportErr).FontSize(float32(tok.TextMeta)).TextColor(tok.Danger)
				}
			})
	})
}

func (a *app) renderDevDraftDialog(c *ui.Context, tok theme.Tokens) {
	st := &a.devInsp
	if !st.draftOpen {
		ui.Box(c.Key("dev-draft-dlg-host")).Children(func() {
			components.Dialog(c.Key("dev-draft-dlg"), &st.draftOpen, components.DialogOptions{
				Label: "记录 UI 细节问题",
			}, nil)
		})
		return
	}
	open := st.draftOpen
	ui.Box(c.Key("dev-draft-dlg-host")).Children(func() {
		components.Dialog(c.Key("dev-draft-dlg"), &open, components.DialogOptions{
			Label: "记录 UI 细节问题",
		}, func(panel ui.Element) {
		panel.Width(540).Padding(float32(tok.Spacing16)).Gap(float32(tok.Spacing12)).
			Background(tok.Surface).Border(1, tok.BorderStrong)
		ui.Column(c).FillWidth().Gap(float32(tok.Spacing12)).Children(func() {
			ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
				components.Badge(c, "DEV UI 拾取", components.BadgeOptions{Tone: components.BadgeAccent})
				components.Text(c, st.draftRegion.Label).FontSize(float32(tok.TextSection)).Bold().TextColor(tok.Text)
				ui.Spacer(c)
				coord := fmt.Sprintf("(%.0f, %.0f, %.0f×%.0f DIP)", st.draftRect.X, st.draftRect.Y, st.draftRect.W, st.draftRect.H)
				components.Text(c, coord).FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
			})

			components.Text(c, "参考源码: "+st.draftRegion.SourceHint).FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
			if st.draftCaptureErr != "" {
				components.Text(c, st.draftCaptureErr).FontSize(float32(tok.TextMeta)).TextColor(tok.Warning)
			} else if len(st.draftFullPNG) > 0 {
				components.Text(c, fmt.Sprintf("✓ 已捕获无遮挡红框全图 + 2x 局部放大图 (%d×%d px) · 保存即实时落盘", st.draftImgW, st.draftImgH)).
					FontSize(float32(tok.TextMeta)).TextColor(tok.Success)
			}

			// Category chips
			ui.Row(c).FillWidth().Wrap().AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
				components.Text(c, "分类:").FontSize(float32(tok.TextMeta)).Bold().TextColor(tok.TextMuted)
				for _, cat := range devCategories {
					style := components.ButtonSecondary
					if st.draftCategory == cat {
						style = components.ButtonPrimary
					}
					if components.Button(c.Key("cat-"+cat), cat, style).Clicked() {
						st.draftCategory = cat
					}
				}
			})

			// Severity chips
			ui.Row(c).FillWidth().Wrap().AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
				components.Text(c, "严重度:").FontSize(float32(tok.TextMeta)).Bold().TextColor(tok.TextMuted)
				for _, sev := range devSeverities {
					style := components.ButtonSecondary
					if st.draftSeverity == sev {
						style = components.ButtonPrimary
					}
					if components.Button(c.Key("sev-"+sev), sev, style).Clicked() {
						st.draftSeverity = sev
					}
				}
			})

			// Suggestion input
			ui.Column(c).FillWidth().Gap(float32(tok.Spacing4)).Children(func() {
				components.Text(c, "修改建议（具体说明你看到的 UI 问题与期望效果）").
					FontSize(float32(tok.TextMeta)).Bold().TextColor(tok.Text)
				ui.TextInput(c.Key("dev-suggestion-input"), &st.draftSuggestion).
					FillWidth().
					Placeholder("例如：此处按钮右边文字被裁切，建议加宽内边距或缩短文案…")
			})

			// Optional MyGo native F12 Inspector snippet bridge
			ui.Column(c).FillWidth().Gap(float32(tok.Spacing4)).Children(func() {
				ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
					components.Text(c, "可选：MyGo F12 / Ctrl+Shift+C 精确组件源码定位").
						FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
					ui.Spacer(c)
					if components.Button(c, "粘贴剪贴板定位", components.ButtonGhost).Clicked() {
						if clip := strings.TrimSpace(c.ReadClipboard()); clip != "" {
							st.draftInspector = clip
						}
					}
				})
				ui.TextInput(c.Key("dev-inspector-input"), &st.draftInspector).
					FillWidth().
					Placeholder("如需组件级 Go 行号，可按 F12/Ctrl+Shift+C 复制后点右侧粘贴")
			})

			ui.Row(c).FillWidth().Justify(ui.End).Gap(float32(tok.Spacing8)).Children(func() {
				if components.Button(c.Key("dev-draft-cancel"), "取消", components.ButtonGhost).Clicked() {
					open = false
				}
				if components.Button(c.Key("dev-draft-save-cont"), "保存并继续拾取", components.ButtonSecondary).Clicked() {
					a.commitDraftIssue(tok)
					open = false
					st.pickActive = true
				}
				if components.Button(c.Key("dev-draft-save"), "保存此条", components.ButtonPrimary).Clicked() {
					a.commitDraftIssue(tok)
					open = false
				}
			})
		})
		})
	})
	st.draftOpen = open
}

func (a *app) renderDevQueueDialog(c *ui.Context, tok theme.Tokens) {
	st := &a.devInsp
	if !st.queueOpen {
		ui.Box(c.Key("dev-queue-dlg-host")).Children(func() {
			components.Dialog(c.Key("dev-queue-dlg"), &st.queueOpen, components.DialogOptions{
				Label: "UI 调试反馈队列",
			}, nil)
		})
		return
	}
	open := st.queueOpen
	ui.Box(c.Key("dev-queue-dlg-host")).Children(func() {
		components.Dialog(c.Key("dev-queue-dlg"), &open, components.DialogOptions{
			Label: "UI 调试反馈队列",
		}, func(panel ui.Element) {
		panel.Width(640).Padding(float32(tok.Spacing16)).Gap(float32(tok.Spacing12)).
			Background(tok.Surface).Border(1, tok.BorderStrong)
		ui.Column(c).FillWidth().Gap(float32(tok.Spacing12)).Children(func() {
			ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
				components.Text(c, fmt.Sprintf("已记录 %d 条 UI 反馈（每条实时落盘）", len(st.issues))).
					FontSize(float32(tok.TextSection)).Bold().TextColor(tok.Text)
				ui.Spacer(c)
				if st.liveMDPath != "" {
					if components.Button(c, "打开实时目录", components.ButtonGhost).Clicked() {
						revealInFileManager(st.liveMDPath, filepath.Dir(st.liveMDPath))
					}
				}
				if components.Button(c, "延迟2s截弹窗", components.ButtonGhost).Clicked() {
					a.scheduleDelayedDevPick(2 * time.Second)
				}
				if components.Button(c, "+ 继续拾取", components.ButtonSecondary).Clicked() {
					st.queueOpen = false
					st.pickActive = true
				}
			})

			if strings.TrimSpace(st.pendingCrashLog) != "" {
				ui.Row(c).FillWidth().Padding(float32(tok.Spacing8), float32(tok.Spacing12)).
					Radius(float32(tok.ControlRadius)).Background(tok.DangerBg).Border(1, tok.Danger).
					AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
					msg := "检测到上次会话异常退出，已自动捕获完整脱敏崩溃堆栈，提交或导出时将一并附带。"
					if st.pendingCrashSummary != "" {
						msg = fmt.Sprintf("检测到上次异常退出（%s）— 已保留批注并自动附带崩溃日志。", st.pendingCrashSummary)
					}
					components.Text(c, msg).FontSize(float32(tok.TextMeta)).TextColor(tok.DangerText).Grow(1).MinWidth(0)
				})
			}

			// Tester name row so incoming bundles in the private feedback repo are easy to identify
			ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
				components.Text(c, "测试者昵称:").FontSize(float32(tok.TextMeta)).Bold().TextColor(tok.TextMuted).Shrink(0)
				ui.Box(c).Width(float32(tok.Spacing48) * 4).Children(func() {
					components.TextBox(c.Key("dev-tester-name"), &st.testerName, components.TextBoxOptions{}).
						Placeholder("如：小王 / Alice（方便开发者归类回复）")
				})
				ui.Spacer(c)
				if st.liveMDPath != "" {
					components.Text(c, "实时落盘已开启").FontSize(float32(tok.TextMeta)).TextColor(tok.Success)
				}
			})

			if len(st.issues) == 0 {
				ui.Box(c).FillWidth().Padding(float32(tok.Spacing16)).Radius(float32(tok.ControlRadius)).
					Background(tok.Elevated).Children(func() {
					components.Text(c, "暂无记录。点击「+ 继续拾取」或按 Ctrl+Shift+D 框选页面细节即可开始记录。").
						FontSize(float32(tok.TextBody)).TextColor(tok.TextMuted)
				})
			} else {
				ui.Scroll(c.Key("dev-queue-scroll")).FillWidth().MaxHeight(320).Children(func() {
					ui.Column(c).FillWidth().Gap(float32(tok.Spacing8)).Children(func() {
						deleteIdx := -1
						for idx, it := range st.issues {
							i := idx
							item := it
							ui.Column(c.Key("dev-issue-"+item.ID)).FillWidth().
								Padding(float32(tok.Spacing8), float32(tok.Spacing12)).
								Radius(float32(tok.ControlRadius)).
								Background(tok.Elevated).
								Border(1, tok.Border).
								Gap(float32(tok.Spacing4)).
								Children(func() {
									ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
										components.Badge(c, item.ID, components.BadgeOptions{Tone: components.BadgeAccent})
										components.Badge(c, item.Severity, components.BadgeOptions{Tone: components.BadgeWarning})
										components.Badge(c, item.Category, components.BadgeOptions{Tone: components.BadgeNeutral})
										components.Text(c, item.Region.Label).FontSize(float32(tok.TextMeta)).Bold().TextColor(tok.Text)
										ui.Spacer(c)
										if components.Button(c.Key("del-"+item.ID), "删除", components.ButtonGhost).Clicked() {
											deleteIdx = i
										}
									})
									sugg := item.Suggestion
									if sugg == "" {
										sugg = "（未填写建议）"
									}
									components.Text(c, sugg).FontSize(float32(tok.TextBody)).TextColor(tok.Text)
								})
						}
						if deleteIdx >= 0 && deleteIdx < len(st.issues) {
							st.issues = append(st.issues[:deleteIdx], st.issues[deleteIdx+1:]...)
							a.syncLiveSessionAfterMutation(c, tok)
						}
					})
				})
			}

			if st.exportErr != "" {
				components.Text(c, st.exportErr).FontSize(float32(tok.TextMeta)).TextColor(tok.Danger)
			}
			if st.exportBanner != "" {
				components.Text(c, st.exportBanner).FontSize(float32(tok.TextMeta)).TextColor(tok.Success)
			}

			ui.Row(c).FillWidth().Justify(ui.End).Gap(float32(tok.Spacing8)).Children(func() {
				if len(st.issues) > 0 {
					if components.Button(c, "清空队列", components.ButtonGhost).Clicked() {
						st.issues = nil
						st.exportBanner = ""
						st.exportErr = ""
						a.syncLiveSessionAfterMutation(c, tok)
					}
				}
				if components.Button(c.Key("dev-queue-close"), "关闭", components.ButtonSecondary).Clicked() {
					open = false
				}
				if len(st.issues) > 0 || strings.TrimSpace(st.pendingCrashLog) != "" {
					if components.Button(c.Key("dev-queue-export"), "导出 .zip…", components.ButtonSecondary).Clicked() && !st.uploading {
						a.exportDevFeedbackBundle(c, tok)
					}
					if a.devUploadConfigured() {
						upLbl := "一键提交给开发者"
						if st.uploading {
							upLbl = "正在上传…"
						}
						if components.Button(c.Key("dev-queue-upload"), upLbl, components.ButtonPrimary).Clicked() && !st.uploading {
							a.uploadDevFeedbackBundleAsync(c, tok)
						}
					}
				}
			})
		})
		})
	})
	st.queueOpen = open
}

func min32Dev(a, b float32) float32 {
	if a < b {
		return a
	}
	return b
}

func abs32Dev(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}
