package main

import (
	"context"
	"errors"
	"log"
	"runtime"
	"runtime/debug"
	"strings"

	"asterlab-mygo-prototype/internal/shell"
	mygo "github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/imagepicker"
)

// mobileLifecycleState holds the flat, value-only navigation state persisted
// across mygo dev restarts and mobile OS background termination via mygo.KeepState.
type mobileLifecycleState struct {
	Restored         bool
	ActiveTabKind    uint8
	ActiveFilePath   string
	ActiveTabMode    uint8
	Page             int
	SelectedDate     string
	ScrollToStepID   string
	SelectedDesignID string
	SelectedRunID    string
	SelectedTask     string
	MobileScrollY    float32
}

// installMobileLifecycle wires mygo.KeepState and App.OnMemoryWarning before
// mygo.App.WhenReady opens windows. Desktop builds treat both as safe no-ops
// (or dev-only state restoration under mygo dev).
func (a *app) installMobileLifecycle() {
	if a == nil {
		return
	}
	mygo.KeepState(&a.mobileState)
	a.mobileMemWarnOff = mygo.App.OnMemoryWarning(func() {
		a.releaseMemoryCaches()
	})
}

// syncMobileLifecycleState snapshots the active mobile/desktop workspace cursor
// into a.mobileState on the main thread so KeepState encodes an up-to-date view.
func (a *app) syncMobileLifecycleState() {
	if a == nil || a.shell == nil {
		return
	}
	a.mobileState.Restored = true
	a.mobileState.Page = int(a.shell.Page)
	a.mobileState.SelectedDate = a.shell.SelectedDate
	a.mobileState.ScrollToStepID = a.scrollToStepID
	a.mobileState.SelectedDesignID = a.snap.DesignID
	a.mobileState.SelectedRunID = a.snap.RunID
	a.mobileState.SelectedTask = a.selectedTask
	if st := a.shell.MobileContentScrollState(); st != nil {
		a.mobileState.MobileScrollY = st.Y
	}
	if tab := a.shell.ActiveTab(); tab != nil {
		a.mobileState.ActiveTabKind = uint8(tab.Kind)
		a.mobileState.ActiveFilePath = tab.FilePath
		a.mobileState.ActiveTabMode = uint8(tab.Mode)
	} else {
		a.mobileState.ActiveTabKind = uint8(shell.TabCalendar)
		a.mobileState.ActiveFilePath = ""
		a.mobileState.ActiveTabMode = uint8(shell.ModeEdit)
	}
}

// applyRestoredMobileState reapplies state recovered by mygo.KeepState after
// initVaultWorkspace and the initial window are ready.
func (a *app) applyRestoredMobileState() {
	if a == nil || a.shell == nil || !a.mobileState.Restored {
		return
	}
	if d := strings.TrimSpace(a.mobileState.SelectedDate); d != "" {
		a.shell.SelectedDate = d
	}
	if a.mobileState.ActiveTabKind == uint8(shell.TabFile) && strings.TrimSpace(a.mobileState.ActiveFilePath) != "" {
		target := a.mobileState.ActiveFilePath
		if a.vaultHasFile(target) {
			a.openVaultFileInActivePane(target, "")
			a.shell.SetActiveTabMode(shell.ViewMode(a.mobileState.ActiveTabMode))
		}
	}
	if a.mobileState.ScrollToStepID != "" {
		a.scrollToStepID = a.mobileState.ScrollToStepID
	}
	if a.mobileState.SelectedTask != "" {
		a.selectedTask = a.mobileState.SelectedTask
	}
	if a.mobileState.MobileScrollY > 0 {
		if st := a.shell.MobileContentScrollState(); st != nil {
			st.Y = a.mobileState.MobileScrollY
		}
	}
	a.shell.Invalidate()
}

func (a *app) vaultHasFile(relPath string) bool {
	if a == nil || a.shell == nil || relPath == "" {
		return false
	}
	for _, f := range a.shell.VaultFiles {
		if f.RelPath == relPath {
			return true
		}
	}
	return false
}

// releaseMemoryCaches drops re-creatable parsed views, block previews, design
// snapshots, and assay caches when iOS/Android signals low memory via
// App.OnMemoryWarning, helping prevent background termination / OOM kills.
func (a *app) releaseMemoryCaches() {
	if a == nil {
		return
	}
	activePath := ""
	if a.shell != nil {
		if tab := a.shell.ActiveTab(); tab != nil && tab.Kind == shell.TabFile {
			activePath = tab.FilePath
		}
	}
	if a.designCache != nil {
		clear(a.designCache)
	}
	if a.blockPreviewCache != nil {
		clear(a.blockPreviewCache)
	}
	if a.cachedAssays != nil {
		clear(a.cachedAssays)
	}
	if a.secondaryViews != nil {
		clear(a.secondaryViews)
	}
	for k := range a.liveViews {
		if k != activePath {
			delete(a.liveViews, k)
		}
	}
	for k := range a.liveEditors {
		if k != activePath {
			delete(a.liveEditors, k)
		}
	}
	runtime.GC()
	debug.FreeOSMemory()
	if a.shell != nil {
		a.shell.Invalidate()
	}
	log.Printf("mobile memory warning: released inactive caches (kept=%q)", activePath)
}

func isMobileRuntime() bool {
	return runtime.GOOS == "android" || runtime.GOOS == "ios"
}

// ShareFile opens the OS native share sheet (UIActivityViewController on iOS,
// ACTION_SEND chooser on Android) for an exported file and optional caption.
// Returns errors.ErrUnsupported on desktop platforms.
func ShareFile(path, text string) error {
	clean := strings.TrimSpace(path)
	if clean == "" {
		return errors.New("share file path is required")
	}
	if !isMobileRuntime() {
		return errors.ErrUnsupported
	}
	return mygo.Share(mygo.ShareOptions{
		Text:  strings.TrimSpace(text),
		Files: []string{clean},
	})
}

// ShareText opens the OS native share sheet for plain text and/or a URL.
// Returns errors.ErrUnsupported on desktop platforms.
func ShareText(text, link string) error {
	t := strings.TrimSpace(text)
	u := strings.TrimSpace(link)
	if t == "" && u == "" {
		return errors.New("share text or url is required")
	}
	if !isMobileRuntime() {
		return errors.ErrUnsupported
	}
	return mygo.Share(mygo.ShareOptions{
		Text: t,
		URL:  u,
	})
}

// CaptureLabPhoto opens the mobile camera via MyGo plugins/imagepicker and
// returns the temporary JPEG file path (or "" if canceled). On desktop builds
// this cleanly returns errors.ErrUnsupported.
func CaptureLabPhoto(ctx context.Context) (string, error) {
	if !isMobileRuntime() {
		return "", errors.ErrUnsupported
	}
	if ctx == nil {
		ctx = context.Background()
	}
	return imagepicker.Capture(ctx)
}

// PickLabPhotos opens the mobile photo library picker via MyGo
// plugins/imagepicker and returns copied temporary file paths. On desktop
// builds this cleanly returns errors.ErrUnsupported.
func PickLabPhotos(ctx context.Context, limit int) ([]string, error) {
	if !isMobileRuntime() {
		return nil, errors.ErrUnsupported
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if limit == 0 {
		limit = 1
	}
	return imagepicker.Pick(ctx, imagepicker.Options{
		Kind:  imagepicker.Images,
		Limit: limit,
	})
}
