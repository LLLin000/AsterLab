package main

// Run recording bench: production seams between the run workspace bench
// (timers, skip/deviation/observation composer, capture strip) and Core.
// Owned by the Run Owner after Wave 1 extraction; routing stays in main.go.
//
// Every write goes through the existing bridge entry points — SaveTimer,
// UpdateTimer, ListTimers, CreateExecutionEvent, SaveAssetLink, ListPantry —
// and every read renders Core's persisted rows. This file adds no second
// store and no new Core command; it only fills the callbacks the bench
// declares.

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"asterlab-mygo-prototype/internal/assets"
	"asterlab-mygo-prototype/internal/runworkspace"
	"asterlab-mygo-prototype/internal/state"
	"asterlab-mygo-prototype/modules/runs"

	mygo "github.com/egoist/mygo"
)

// syncRunBenchCallbacks wires the bench seams every frame. The callbacks live
// on the workspace State so they survive snapshot reloads; assigning them
// here keeps main.go's construction site untouched.
func (a *app) syncRunBenchCallbacks() {
	if a.runWS == nil {
		return
	}
	cb := &a.runWS.Callbacks
	cb.LoadTimers = a.loadRunTimers
	cb.SaveTimer = a.saveRunTimer
	cb.TimerAction = a.applyTimerAction
	cb.AppendEvent = a.appendRunEvent
	cb.LoadPantry = a.loadRunPantry
	cb.TaskAssets = a.runAssetsForTask
	cb.CaptureWells = a.captureWellsForTask
	cb.RequestBatchLink = a.requestBatchLink
	// The shared record projection reads the stored snapshot plus
	// already-linked assets; the bench memoizes it by Run revision.
	cb.HistoryInputs = func() (runs.RunSnapshot, []runs.Asset) {
		return a.snap, a.runAssets
	}
	// The blind flag is view state on the bench; grids are Core-backed data
	// the flag only masks at render time.
	blind := a.runWS.Bench != nil && a.runWS.Bench.Blind
	if a.ws != nil && a.ws.Grid != nil {
		a.ws.Grid.Blind = blind
	}
	for _, grid := range a.taskGrids {
		if grid != nil {
			grid.Blind = blind
		}
	}
}

// pollRunBench reloads the run's timers when the run changed, the revision
// moved, or five seconds passed since the last load — the same throttle the
// global timer strip uses. It repaints only when a poll actually moved time.
func (a *app) pollRunBench() {
	if a.runWS == nil || a.snap.RunID == "" {
		return
	}
	before := a.runWS.TimersFingerprint()
	if !a.runWS.RefreshTimers(false, 5*time.Second) {
		return
	}
	if a.runWS.TimersFingerprint() != before && a.shell != nil {
		a.shell.Invalidate()
	}
}

// loadRunTimers lists the run's persisted timers for checkpoint rendering.
func (a *app) loadRunTimers(runID string) ([]runs.TimerRecord, error) {
	if a.runSvc == nil {
		return nil, fmt.Errorf("运行服务不可用")
	}
	if strings.TrimSpace(runID) == "" {
		runID = a.snap.RunID
	}
	if strings.TrimSpace(runID) == "" {
		return nil, fmt.Errorf("当前没有选中的 Run")
	}
	timers, err := a.runSvc.ListTimers(context.Background(), runID)
	if err != nil {
		return nil, state.MapError(err)
	}
	return timers, nil
}

// saveRunTimer persists one timer row. The run identity comes from the open
// Run; the panel supplies the task-scoped row.
func (a *app) saveRunTimer(timer runs.TimerRecord) (runs.TimerRecord, error) {
	if a.runSvc == nil || a.snap.RunID == "" {
		return runs.TimerRecord{}, fmt.Errorf("运行服务不可用")
	}
	timer.RunID = a.snap.RunID
	saved, err := a.runSvc.SaveTimer(context.Background(), timer)
	if err != nil {
		return runs.TimerRecord{}, state.MapError(err)
	}
	return saved, nil
}

// applyTimerAction applies one lifecycle action at the caller's clock. Timer
// writes keep their own table and do not move the Run revision, so no
// snapshot reload is needed; the panel refreshes its ListTimers cache.
func (a *app) applyTimerAction(input runs.TimerActionInput) (runs.TimerUpdateResult, error) {
	if a.runSvc == nil || a.snap.RunID == "" {
		return runs.TimerUpdateResult{}, fmt.Errorf("运行服务不可用")
	}
	input.Timer.RunID = a.snap.RunID
	result, err := a.runSvc.UpdateTimer(context.Background(), input)
	if err != nil {
		return runs.TimerUpdateResult{}, state.MapError(err)
	}
	return result, nil
}

// appendRunEvent appends one execution event and applies Core's authoritative
// snapshot. It never completes the step: skips stay skips. Core's pantry
// advisory is returned as display text ("" when there is none).
func (a *app) appendRunEvent(taskKey string, input runs.ExecutionEventInput) (string, error) {
	if a.runSvc == nil || a.snap.RunID == "" {
		return "", fmt.Errorf("运行服务不可用")
	}
	input.RunID = a.snap.RunID
	result, err := a.runSvc.CreateExecutionEvent(context.Background(), input)
	if err != nil {
		return "", state.MapError(err)
	}
	a.applySnap(result.Snapshot)
	return pantryAdvisory(result.PantryNotices), nil
}

// pantryAdvisory renders Core's near-expiry/expired notices for the composer.
// An empty notice list means nothing to report, never a failure.
func pantryAdvisory(notices []runs.PantryNotice) string {
	parts := make([]string, 0, len(notices))
	for _, notice := range notices {
		name := notice.Name
		if notice.LabelCode != "" {
			name = notice.LabelCode + " · " + notice.Name
		}
		status := "近效期"
		if notice.Status == "expired" {
			status = "已过期"
		}
		part := fmt.Sprintf("耗材%s%s", name, status)
		if notice.NextAction != "" {
			part += "，请" + notice.NextAction
		}
		parts = append(parts, part)
	}
	return strings.Join(parts, "；")
}

// loadRunPantry lists the pantry ledger for the composer's optional
// reference. It mirrors the Tools wiring: today, Core's default horizon.
func (a *app) loadRunPantry() (runs.PantryLedger, error) {
	if a.runSvc == nil {
		return runs.PantryLedger{}, fmt.Errorf("运行服务不可用")
	}
	ledger, err := a.runSvc.ListPantry(context.Background(), runs.PantryQuery{})
	if err != nil {
		return runs.PantryLedger{}, state.MapError(err)
	}
	return ledger, nil
}

// captureWellsForTask resolves the planned wells in fill order plus the
// frozen per-well FOV count. Wells come from the frozen plate coordinate
// options; the FOV count comes from the frozen measurement profile bound to
// the design step. Zero FOV means the plan declares none.
func (a *app) captureWellsForTask(taskKey string) ([]string, int) {
	if strings.TrimSpace(taskKey) == "" {
		return nil, 0
	}
	wells, _ := a.assetCoordinateOptions(taskKey)
	if len(wells) == 0 {
		return nil, 0
	}
	fov := 0
	if a.asterDoc != nil && a.runWSVM != nil {
		if task, ok := a.runWSVM.TaskByKey[taskKey]; ok && task != nil {
			fov = runworkspace.FOVPerWellForStep(a.asterDoc.Source, task.StepID)
		}
	}
	return wells, fov
}

// requestBatchLink opens the host file picker and links the chosen files
// across the task's capture slots: after six images the plan moves to the
// next unfilled well. Each file persists through the existing asset.link
// entry with its own well_id/timepoint/fov_index triple.
func (a *app) requestBatchLink(taskKey string) {
	if a.runSvc == nil || a.runWS == nil || a.snap.RunID == "" || strings.TrimSpace(taskKey) == "" {
		return
	}
	fail := func(msg string) {
		a.runWS.NoteBatchResult("", msg)
		if a.shell != nil {
			a.shell.Invalidate()
		}
	}
	wells, fov := a.captureWellsForTask(taskKey)
	if len(wells) == 0 || fov <= 0 {
		fail("该步骤没有采集槽位（无计划孔位或每孔视野数未定）")
		return
	}
	timepoint := ""
	if a.runWSVM != nil {
		if task, ok := a.runWSVM.TaskByKey[taskKey]; ok && task != nil {
			timepoint = strings.TrimSpace(task.Timepoint)
		}
	}
	coverage := runworkspace.CoverageByWell(a.runAssetsForTask(taskKey), wells, timepoint, fov)
	if a.snap.DesignID == "" {
		fail("当前 Run 没有关联设计")
		return
	}
	rootID := assetRootID(a.snap.DesignID)
	rootDir := a.assetRoots[rootID]
	if info, statErr := os.Stat(rootDir); rootDir == "" || statErr != nil || !info.IsDir() {
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
			Parent: mygo.FocusedWindow(), Title: "选择实验数据根目录", Directory: true,
		})
		if err != nil {
			fail(state.MapError(err).Error())
			return
		}
		if len(paths) == 0 {
			return
		}
		rootDir = paths[0]
		if a.assetRoots == nil {
			a.assetRoots = map[string]string{}
		}
		a.assetRoots[rootID] = rootDir
		a.saveSession()
	}
	paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
		Parent: mygo.FocusedWindow(), Title: "批量关联采图", DefaultPath: rootDir, Multiple: true,
	})
	if err != nil {
		fail(state.MapError(err).Error())
		return
	}
	if len(paths) == 0 {
		return
	}
	drops := runworkspace.PlanBatchDrops(wells, timepoint, fov, coverage, len(paths))
	linked, skipped := 0, 0
	var firstErr string
	for i, path := range paths {
		if i >= len(drops) {
			skipped++
			continue
		}
		drop := drops[i]
		asset, err := assets.NewLink(rootID, rootDir, path, assets.Link{
			RunID: a.snap.RunID, TaskKey: taskKey,
		}, "local-user", true)
		if err != nil {
			skipped++
			if firstErr == "" {
				firstErr = err.Error()
			}
			continue
		}
		savedAsset := runAssetFromLocal(asset)
		savedAsset.Link.WellID = drop.WellID
		savedAsset.Link.Timepoint = drop.Timepoint
		savedAsset.Link.FOVIndex = drop.FOVIndex
		saved, err := a.runSvc.SaveAssetLink(context.Background(), runs.AssetLinkInput{Asset: savedAsset})
		if err != nil {
			skipped++
			if firstErr == "" {
				firstErr = state.MapError(err).Error()
			}
			continue
		}
		a.upsertRunAsset(saved)
		linked++
	}
	a.refreshRunWorkspace()
	done := fmt.Sprintf("已关联 %d 个文件", linked)
	if skipped > 0 {
		done += fmt.Sprintf("，%d 个未关联（空槽不足或文件不可读）", skipped)
		if firstErr != "" {
			done += "：" + firstErr
		}
	}
	// Partial success stays an outcome, not a failure: the linked files are
	// persisted and only the remainder needs attention.
	a.runWS.NoteBatchResult(done, "")
	if a.shell != nil {
		a.shell.Invalidate()
	}
}
