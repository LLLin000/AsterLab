package main

import (
	"asterlab-mygo-prototype/internal/assays"
	"asterlab-mygo-prototype/internal/assets"
	"asterlab-mygo-prototype/internal/asterdoc"
	"asterlab-mygo-prototype/internal/blocks"
	"asterlab-mygo-prototype/internal/blockui"
	"asterlab-mygo-prototype/internal/designer"
	"asterlab-mygo-prototype/internal/notebook"
	"asterlab-mygo-prototype/internal/protocol"
	"asterlab-mygo-prototype/internal/resultgrid"
	"asterlab-mygo-prototype/internal/runui"
	"asterlab-mygo-prototype/internal/runworkspace"
	"asterlab-mygo-prototype/internal/shell"
	"asterlab-mygo-prototype/internal/state"
	"asterlab-mygo-prototype/internal/theme"
	"asterlab-mygo-prototype/internal/tools"
	"asterlab-mygo-prototype/internal/workflow"
	"asterlab-mygo-prototype/internal/workflowtree"
	"asterlab-mygo-prototype/modules/runs"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	mygo "github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/richtext"
	"github.com/oneo-me/onevra/source/components"
)

type sample struct{ id, value string }

type stepPropDraft struct {
	Title      string
	Assay      string
	Timepoints string
	Tasks      []canvasTaskDraft
}

type pdfExportResult struct {
	path     string
	err      error
	canceled bool
}

type app struct {
	page    *string
	step    *string
	samples []sample

	runSvc                     runs.RunService
	designSvc                  runs.DesignService
	timeBudgetSvc              runs.TimeBudgetService
	timeBudgetErr              error
	snap                       runs.RunSnapshot
	syncErr                    string
	batchView                  string
	asterDoc                   *asterdoc.FrozenAsterDoc
	asterErr                   string
	pdfExporting               bool
	pdfExportResults           chan pdfExportResult
	showFrozenDoc              bool
	designDoc                  *runs.AsterDocSnapshot
	designView                 *asterdoc.View
	assetRoots                 map[string]string
	runAssets                  []runs.Asset
	runViewKinds               map[string]runworkspace.ViewKind
	taskGrids                  map[string]*resultgrid.Grid
	taskDrafts                 map[string]runui.Draft
	designDocErr               string
	designLoadErr              string
	designLoadID               string
	designDocLoadFailed        bool
	retryDesignDoc             func()
	pendingRunID               string
	projects                   []runs.ProjectSummary
	protocols                  []runs.ProtocolSummary
	readiness                  *runs.AsterDocAssessment
	readinessLoading           bool
	readinessErr               string
	preflightOpen              bool
	preflightRisk              bool
	newDesignOpen              bool
	addModuleOpen              bool
	importOpen                 bool
	importPath                 string
	importTitle                string
	importErr                  string
	designHistory              []runs.AsterDocSnapshot
	designFuture               []runs.AsterDocSnapshot
	asterDraftUndo             [][]byte
	asterDraftRedo             [][]byte
	asterDraftLast             []byte
	asterDraftSuspend          bool
	asterDraftSelection        string
	designTab                  int
	designCanvas               *workflow.Editor
	designCanvasSource         string
	designCanvasFitPending     bool
	designCanvasBoundsW        float32
	designCanvasBoundsH        float32
	designInspectorOpen        bool
	designInspectorUserToggled bool
	designLayouts              map[string]map[string]workflow.Position
	designHierarchy            *workflowtree.Tree
	designHierarchySource      string
	designHierarchyErr         string
	blockPreviewCache          map[string]workflowtree.BlockDefinition
	designPreviewExpanded      bool
	designSubflow              *workflow.Editor
	designSubflowParent        string
	designSubflowTitle         string
	designSubflowSelected      string
	designSubflowFitted        bool
	designSubflowBoundsW       float32
	designSubflowBoundsH       float32
	designSelected             string
	canvasTitle                string
	canvasAssay                string
	canvasPath                 string
	canvasTimepoints           string
	canvasSelLoaded            string
	canvasSyncing              bool
	confirmDeleteStep          string
	canvasTasks                []canvasTaskDraft
	canvasChecks               []canvasCheckDraft
	canvasUses                 []canvasUseDraft
	canvasUsePick              string
	stepDrafts                 map[string]stepPropDraft
	stepPropSaved              string
	noteDraft                  string
	noteRunID                  string
	noteDirty                  bool
	noteRecovered              bool
	noteDrafts                 map[string]notebookDraft
	noteState                  string
	noteErr                    string
	createRunErr               string
	linkErr                    string
	linkErrTask                string
	assetErr                   string
	assetErrTask               string
	assetErrRetry              func()
	linkDialogOpen             bool
	linkDialogTask             string
	linkDialogAssetID          string
	linkDialogWell             string
	linkDialogTimepoint        string
	linkDialogFOV              string
	linkDialogWells            []string
	linkDialogTimepoints       []string
	linkDialogErr              string
	noteMode                   int
	noteEditMode               int
	noteDoc                    *richtext.Document
	noteAster                  *asterdoc.View

	shell               *shell.Shell
	ws                  *runui.Workspace
	runWS               *runworkspace.State
	runWSVM             *runworkspace.RunWorkspaceViewModel
	runWSKey            string
	designer            *designer.Designer
	pendingDesignID     string
	confirmDesignSwitch bool
	windowReady         bool
	guard               state.StaleGuard

	designSnap *runs.DesignSnapshot

	wf     *workflow.Editor
	blocks []blocks.Instance

	opCounter atomic.Uint64

	selectedTask string
	tools        *tools.Workspace
	// timersPoll throttles the global timer-strip refresh; see pollTimers.
	timersPoll time.Time
}

func (a *app) nextOp() string {
	n := a.opCounter.Add(1)
	return fmt.Sprintf("op-%d-%d", time.Now().UnixNano(), n)
}

func stepModuleRefs(designData, stepID string) map[string]bool {
	refs := map[string]bool{}
	if stepID == "" {
		return refs
	}
	var design struct {
		MyGo struct {
			StepUses map[string][]struct {
				Ref  string `json:"ref"`
				Role string `json:"role"`
			} `json:"stepUses"`
		} `json:"mygo"`
	}
	if json.Unmarshal([]byte(designData), &design) != nil {
		return refs
	}
	for _, use := range design.MyGo.StepUses[stepID] {
		if use.Ref != "" {
			refs[use.Ref] = true
		}
	}
	return refs
}

func sdkModulesFromDesign(designData string) []map[string]any {
	var design struct {
		AsterLab struct {
			Modules []map[string]any `json:"modules"`
		} `json:"asterlab"`
	}
	if json.Unmarshal([]byte(designData), &design) != nil {
		return nil
	}
	return design.AsterLab.Modules
}

func frozenAssayEndpoints(designData string) map[string][2]string {
	out := map[string][2]string{}
	var design struct {
		Assays []struct {
			ID       string `json:"id"`
			Endpoint string `json:"endpoint"`
			Unit     string `json:"unit"`
		} `json:"assays"`
	}
	if json.Unmarshal([]byte(designData), &design) != nil {
		return out
	}
	for _, assay := range design.Assays {
		out[assay.ID] = [2]string{assay.Endpoint, assay.Unit}
	}
	return out
}

func toRunView(s runs.RunSnapshot) runui.RunView {
	endpoints := frozenAssayEndpoints(s.DesignData)
	v := runui.RunView{
		Title: s.Title, Status: s.Status, Revision: s.Revision,
		DesignRevision: s.DesignRevision,
		Measurements: make([]runui.Measurement, 0, len(s.Measurements)),
		Events:       make([]runui.Event, 0, len(s.Events)),
	}
	if s.AsterDoc != nil {
		v.AsterDocRevision = s.AsterDoc.Revision
	}
	for _, t := range s.Tasks {
		endpoint, unit := "", ""
		if pair, ok := endpoints[t.AssayID]; ok {
			endpoint, unit = pair[0], pair[1]
		}
		task := runui.Task{
			Key: t.Key, Title: t.Title, Timepoint: t.Timepoint, Status: t.Status,
			AssayID: t.AssayID, DesignStepID: t.DesignStepID,
			ProtocolVersionID: t.ProtocolVersionID, Instruction: t.Instruction,
			Endpoint: endpoint, Unit: unit,
		}
		var params []struct {
			Key string `json:"key"`
			Label string `json:"label"`
			Name string `json:"name"`
			Value any `json:"value"`
			Unit string `json:"unit"`
		}
		if json.Unmarshal([]byte(t.ParametersJSON), &params) == nil {
			for _, p := range params {
				label := p.Label
				if label == "" { label = p.Name }
				if label == "" { label = p.Key }
				value := ""
				if p.Value != nil { value = fmt.Sprint(p.Value) }
				task.Parameters = append(task.Parameters, runui.FrozenParameter{
					Key: p.Key, Label: label, Value: value, Unit: p.Unit,
				})
			}
		}
		for _, check := range t.Checks {
			task.Checks = append(task.Checks, runui.TaskCheck{
				CheckID: check.CheckID, Label: check.Label,
				Instruction: check.Instruction, Blocking: check.Blocking,
			})
		}
		v.Tasks = append(v.Tasks, task)
	}
	for _, m := range s.Measurements {
		v.Measurements = append(v.Measurements, runui.Measurement{SampleID: m.SampleID, Endpoint: m.Endpoint, Value: m.Value, Unit: m.Unit})
	}
	if audit := s.CreationAudit; audit != nil {
		v.Audit = &runui.AuditInfo{ActorID: audit.ActorID, RecordedAt: audit.RecordedAt,
			Decision: audit.Decision, Warnings: audit.Warnings}
	}
	for _, e := range s.Events {
		event := runui.Event{Kind: e.EventType, Message: e.Content,
			TaskKey: e.TaskKey, StepPath: e.ProtocolStepPath, AssayID: e.AssayID,
			Timepoint: e.Timepoint}
		for _, check := range e.Checks {
			event.Checks = append(event.Checks, runui.CheckAck{CheckID: check.CheckID, Status: check.Status})
		}
		v.Events = append(v.Events, event)
	}
	return v
}

func (a *app) applySnap(snap runs.RunSnapshot) {
	switchingRun := a.snap.RunID != "" && snap.RunID != "" && a.snap.RunID != snap.RunID
	if switchingRun {
		// StaleGuard tracks revisions within a single Run; switching Runs resets it so
		// returning to a lower-revision Run is not dropped as stale.
		a.guard = state.StaleGuard{}
	}
	if !a.guard.Accept(snap.Revision) {
		a.syncErr = fmt.Sprintf("忽略过期快照（收到 rev %d，当前已应用 rev %d）", snap.Revision, a.guard.Latest())
		return
	}
	var newSnap runs.RunSnapshot
	if switchingRun {
		newSnap = snap
		a.syncErr = ""
	} else {
		var err error
		newSnap, err = state.ApplySnapshot(a.snap, snap)
		if err != nil {
			a.syncErr = fmt.Sprintf("快照应用失败：%v", err)
			return
		}
		if a.snap.RunID != "" && a.snap.RunID == snap.RunID {
			if err := state.ConflictIfDifferent(a.snap, snap); err != nil {
				a.syncErr = fmt.Sprintf("检测到同修订快照冲突（rev %d）：%v", snap.Revision, err)
			} else {
				a.syncErr = ""
			}
		} else {
			a.syncErr = ""
		}
	}
	previousRunID := a.snap.RunID
	if previousRunID != "" && previousRunID != snap.RunID {
		a.clearTaskRecording()
		a.clearAssetError()
		a.linkErr = ""
		a.linkErrTask = ""
		a.selectedTask = ""
		if a.ws != nil {
			a.ws.Selected = ""
		}
	}
	a.snap = newSnap
	a.loadFrozenAsterDoc()
	if previousRunID != newSnap.RunID {
		a.loadRunAssets()
	}
	a.refreshRunWorkspace()
	a.rememberRun(newSnap)
	if a.ws != nil {
		view := toRunView(a.snap)
		a.ws.View = view
		a.ws.SyncChecks()
		a.ws.Notebook = notebook.Build(a.snap)
		a.ws.Draft = runui.NewDraft(view)
		if a.ws.Selected == "" {
			a.ws.Selected = currentRunTask(a.snap.Tasks).Key
		}
		a.selectedTask = a.ws.Selected
		if len(a.snap.Tasks) == 0 && len(a.snap.Measurements) == 0 {
			a.ws.State = runui.StateEmpty
		} else {
			a.ws.State = runui.StateReady
		}
		a.prepareGrid()
		a.selectTaskFromEnv()
	}
	if a.noteRunID != a.snap.RunID {
		a.loadNotebook()
	}
}

func assetRootID(designID string) string {
	return "design-data-" + strings.TrimSpace(designID)
}

func localAssetFromRun(asset runs.Asset) assets.Asset {
	return assets.Asset{
		ID: asset.ID, RootID: asset.RootID, RelativePath: asset.RelativePath,
		MediaType: assets.MediaType(asset.MediaType), Size: asset.Size,
		ModTime: asset.ModTime, SHA256: asset.SHA256,
		Availability: assets.Availability(asset.Availability),
		Link:         assets.Link{RunID: asset.Link.RunID, TaskKey: asset.Link.TaskKey, SampleID: asset.Link.SampleID},
		LinkedAt:     asset.LinkedAt, ActorID: asset.ActorID,
	}
}

func runAssetFromLocal(asset assets.Asset) runs.Asset {
	return runs.Asset{
		ID: asset.ID, RootID: asset.RootID, RelativePath: asset.RelativePath,
		MediaType: string(asset.MediaType), Size: asset.Size, ModTime: asset.ModTime,
		SHA256: asset.SHA256, Availability: string(asset.Availability),
		Link:     runs.AssetLink{RunID: asset.Link.RunID, TaskKey: asset.Link.TaskKey, SampleID: asset.Link.SampleID},
		LinkedAt: asset.LinkedAt, ActorID: asset.ActorID,
	}
}

func localAssetFromView(asset runworkspace.LinkedAsset) assets.Asset {
	availability := assets.Unavailable
	if asset.Available {
		availability = assets.Available
	}
	return assets.Asset{
		ID: asset.AssetID, RootID: asset.RootID, RelativePath: asset.RelativePath,
		MediaType: assets.MediaType(asset.MediaType), Size: asset.Size,
		ModTime: asset.ModTime, SHA256: asset.SHA256, Availability: availability,
		Link: assets.Link{RunID: asset.LinkedTo.RunID, TaskKey: asset.LinkedTo.TaskKey, SampleID: asset.LinkedTo.SampleID},
	}
}

func (a *app) setAssetError(taskKey, msg string, retry func()) {
	a.assetErr = msg
	a.assetErrTask = taskKey
	a.assetErrRetry = retry
}

func (a *app) clearAssetError() {
	a.assetErr = ""
	a.assetErrTask = ""
	a.assetErrRetry = nil
}

func (a *app) loadRunAssets() {
	if a.snap.RunID == "" || a.runSvc == nil {
		a.runAssets = nil
		return
	}
	linked, err := a.runSvc.ListAssetLinks(context.Background(), a.snap.RunID)
	if err != nil {
		a.setAssetError("", "加载关联文件列表失败："+state.MapError(err).Error(), a.loadRunAssets)
		return
	}
	if a.assetErrTask == "" {
		a.clearAssetError()
	}
	a.runAssets = linked
}

func (a *app) runAssetsForTask(taskKey string) []runworkspace.LinkedAsset {
	out := []runworkspace.LinkedAsset{}
	roots := assets.Roots(a.assetRoots)
	for _, linked := range a.runAssets {
		if linked.Link.TaskKey != taskKey {
			continue
		}
		local := localAssetFromRun(linked)
		path, err := roots.Resolve(local)
		available := false
		if err == nil {
			available, _ = assets.Verify(local, path, false)
		}
		out = append(out, runworkspace.LinkedAsset{
			AssetID: linked.ID, RootID: linked.RootID, RelativePath: linked.RelativePath,
			MediaType: linked.MediaType, Size: linked.Size, ModTime: linked.ModTime,
			SHA256: linked.SHA256, Available: available,
			WellID: linked.Link.WellID, Timepoint: linked.Link.Timepoint, FOVIndex: linked.Link.FOVIndex,
			LinkedTo: runworkspace.AssetLink{
				RunID: linked.Link.RunID, TaskKey: linked.Link.TaskKey, SampleID: linked.Link.SampleID,
			},
		})
	}
	return out
}

func (a *app) openRunAsset(asset runworkspace.LinkedAsset) error {
	local := localAssetFromView(asset)
	path, err := assets.Roots(a.assetRoots).Resolve(local)
	if err != nil {
		return err
	}
	if ok, reason := assets.Verify(local, path, true); !ok {
		return fmt.Errorf("文件校验失败：%s", reason)
	}
	go mygo.Shell.OpenPath(path)
	return nil
}

func (a *app) tryOpenRunAsset(taskKey string, asset runworkspace.LinkedAsset) error {
	if err := a.openRunAsset(asset); err != nil {
		a.setAssetError(taskKey, "无法打开文件："+err.Error(), func() {
			_ = a.tryOpenRunAsset(taskKey, asset)
		})
		if a.shell != nil {
			a.shell.Invalidate()
		}
		return err
	}
	a.clearAssetError()
	if a.shell != nil {
		a.shell.Invalidate()
	}
	return nil
}

func (a *app) tryRelocateRunAsset(taskKey string, asset runworkspace.LinkedAsset) error {
	if err := a.relocateRunAsset(asset); err != nil {
		a.setAssetError(taskKey, "文件重新定位失败："+err.Error(), func() {
			_ = a.tryRelocateRunAsset(taskKey, asset)
		})
		if a.shell != nil {
			a.shell.Invalidate()
		}
		return err
	}
	a.clearAssetError()
	if a.shell != nil {
		a.shell.Invalidate()
	}
	return nil
}

func (a *app) relocateRunAsset(asset runworkspace.LinkedAsset) error {
	local := localAssetFromView(asset)
	paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
		Parent: mygo.FocusedWindow(), Title: "重新定位本地数据根目录",
		DefaultPath: a.assetRoots[asset.RootID], Directory: true,
	})
	if err != nil || len(paths) == 0 {
		return err
	}
	roots := assets.Roots(a.assetRoots)
	roots[asset.RootID] = paths[0]
	path, err := roots.Resolve(local)
	if err != nil {
		return err
	}
	if ok, reason := assets.Verify(local, path, true); !ok {
		return fmt.Errorf("文件校验失败：%s", reason)
	}
	if a.assetRoots == nil {
		a.assetRoots = map[string]string{}
	}
	a.assetRoots[asset.RootID] = paths[0]
	a.saveSession()
	return nil
}

func validateAssetCoordinates(well, timepoint, fov string, wells, timepoints []string) (runs.AssetLink, error) {
	valid := func(value string, allowed []string) bool {
		for _, option := range allowed {
			if value == option {
				return true
			}
		}
		return false
	}
	if !valid(well, wells) {
		return runs.AssetLink{}, fmt.Errorf("未知孔位：%s", well)
	}
	if !valid(timepoint, timepoints) {
		return runs.AssetLink{}, fmt.Errorf("未知时间点：%s", timepoint)
	}
	index, err := strconv.ParseUint(strings.TrimSpace(fov), 10, 32)
	if err != nil || index < 1 {
		return runs.AssetLink{}, fmt.Errorf("FOV 必须是大于等于 1 的整数")
	}
	return runs.AssetLink{WellID: well, Timepoint: timepoint, FOVIndex: uint32(index)}, nil
}

func linkPickedAssets(paths []string, coordinate runs.AssetLink, link func(string, runs.AssetLink) error) error {
	for _, path := range paths {
		if err := link(path, coordinate); err != nil {
			return err
		}
	}
	return nil
}

func (a *app) openAssetLinkForm(taskKey string, prefill runworkspace.LinkedAsset) {
	a.linkDialogTask = taskKey
	a.linkDialogAssetID = strings.TrimSpace(prefill.AssetID)
	a.linkDialogWells, a.linkDialogTimepoints = a.assetCoordinateOptions(taskKey)
	a.linkDialogWell = strings.TrimSpace(prefill.WellID)
	a.linkDialogTimepoint = strings.TrimSpace(prefill.Timepoint)
	a.linkDialogFOV = strconv.FormatUint(uint64(prefill.FOVIndex), 10)
	if a.linkDialogWell == "" && len(a.linkDialogWells) > 0 {
		a.linkDialogWell = a.linkDialogWells[0]
	}
	if a.linkDialogTimepoint == "" {
		for _, task := range a.snap.Tasks {
			if task.Key == taskKey {
				a.linkDialogTimepoint = strings.TrimSpace(task.Timepoint)
				break
			}
		}
	}
	if a.linkDialogTimepoint == "" && len(a.linkDialogTimepoints) > 0 {
		a.linkDialogTimepoint = a.linkDialogTimepoints[0]
	}
	if a.linkDialogFOV == "0" || a.linkDialogFOV == "" {
		a.linkDialogFOV = "1"
	}
	a.linkDialogErr = ""
	a.linkDialogOpen = true
}

func designPlateWells(raw, assayID string) []string {
	var doc struct {
		MyGo struct {
			Instances map[string][]blocks.Instance `json:"instances"`
			Assays    map[string]struct {
				PlateInstanceID string `json:"plateInstanceId"`
			} `json:"assays"`
		} `json:"mygo"`
	}
	if json.Unmarshal([]byte(raw), &doc) != nil {
		return nil
	}
	plateID := doc.MyGo.Assays[assayID].PlateInstanceID
	keys := make([]string, 0, len(doc.MyGo.Instances))
	for key := range doc.MyGo.Instances {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var plate *blocks.Instance
	for _, key := range keys {
		for i := range doc.MyGo.Instances[key] {
			instance := &doc.MyGo.Instances[key][i]
			if instance.Type != blocks.TypePlate96 && instance.Type != blocks.TypePlate12 {
				continue
			}
			if plateID == "" || instance.ID == plateID {
				plate = instance
				break
			}
		}
		if plate != nil {
			break
		}
	}
	if plate == nil {
		return nil
	}
	var dimensions struct {
		Rows int `json:"rows"`
		Cols int `json:"cols"`
	}
	if json.Unmarshal(plate.RawProps, &dimensions) != nil {
		return nil
	}
	if dimensions.Rows == 0 || dimensions.Cols == 0 {
		if plate.Type == blocks.TypePlate12 {
			dimensions.Rows, dimensions.Cols = 3, 4
		} else {
			dimensions.Rows, dimensions.Cols = 8, 12
		}
	}
	wells := make([]string, 0, dimensions.Rows*dimensions.Cols)
	for row := 0; row < dimensions.Rows && row < 26; row++ {
		for column := 1; column <= dimensions.Cols; column++ {
			wells = append(wells, string(rune('A'+row))+strconv.Itoa(column))
		}
	}
	return wells
}

func frozenAssayTimepoints(raw, assayID string) []string {
	var doc struct {
		Assays []struct {
			ID         string   `json:"id"`
			Timepoints []string `json:"timepoints"`
		} `json:"assays"`
		MyGo struct {
			Assays map[string]assays.Design `json:"assays"`
		} `json:"mygo"`
	}
	if json.Unmarshal([]byte(raw), &doc) != nil {
		return nil
	}
	if tps := doc.MyGo.Assays[assayID].Timepoints; len(tps) > 0 {
		return tps
	}
	for _, assay := range doc.Assays {
		if assay.ID == assayID && len(assay.Timepoints) > 0 {
			return assay.Timepoints
		}
	}
	return nil
}

func (a *app) assetCoordinateOptions(taskKey string) ([]string, []string) {
	selectedAssay := ""
	for _, task := range a.snap.Tasks {
		if task.Key == taskKey {
			selectedAssay = task.AssayID
			break
		}
	}

	timepoints := make([]string, 0, len(a.snap.Tasks))
	seenTimes := map[string]bool{}
	addTimepoint := func(raw string) {
		value := strings.TrimSpace(raw)
		if value != "" && !seenTimes[value] {
			seenTimes[value] = true
			timepoints = append(timepoints, value)
		}
	}
	for _, task := range a.snap.Tasks {
		if task.AssayID != selectedAssay {
			continue
		}
		addTimepoint(task.Timepoint)
	}

	designData := a.snap.DesignData
	if designData == "" && a.designSnap != nil {
		designData = a.designSnap.Data
	}
	if selectedAssay != "" {
		for _, value := range frozenAssayTimepoints(designData, selectedAssay) {
			addTimepoint(value)
		}
	}
	wells := designPlateWells(designData, selectedAssay)
	if len(wells) == 0 && selectedAssay != "" {
		if schema, _, err := assays.BuildSchema([]byte(designData), selectedAssay, ""); err == nil &&
			(schema.Layout == "plate96" || schema.Layout == "plate12") {
			for _, row := range schema.RowLabels {
				for _, column := range schema.ColumnLabels {
					wells = append(wells, row+column)
				}
			}
		}
	}
	return wells, timepoints
}

func (a *app) linkRunAsset(taskKey string, coordinate runs.AssetLink) error {
	if a.runSvc == nil || a.snap.RunID == "" || taskKey == "" {
		return fmt.Errorf("当前没有可关联文件的 Run 任务")
	}
	if strings.TrimSpace(a.snap.DesignID) == "" {
		return fmt.Errorf("当前 Run 没有关联设计")
	}
	rootID := assetRootID(a.snap.DesignID)
	rootDir := a.assetRoots[rootID]
	info, statErr := os.Stat(rootDir)
	if rootDir == "" || statErr != nil || !info.IsDir() {
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
			Parent: mygo.FocusedWindow(), Title: "选择实验数据根目录", Directory: true,
		})
		if err != nil {
			return err
		}
		if len(paths) == 0 {
			return nil
		}
		rootDir = paths[0]
		if a.assetRoots == nil {
			a.assetRoots = map[string]string{}
		}
		a.assetRoots[rootID] = rootDir
		a.saveSession()
	}
	paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
		Parent: mygo.FocusedWindow(), Title: "关联实验文件", DefaultPath: rootDir, Multiple: true,
	})
	if err != nil || len(paths) == 0 {
		return err
	}
	coordinate.RunID, coordinate.TaskKey = a.snap.RunID, taskKey
	return a.linkAssetPaths(rootID, rootDir, paths, coordinate)
}

func (a *app) upsertRunAsset(saved runs.Asset) {
	for i, old := range a.runAssets {
		if (saved.ID != "" && old.ID == saved.ID) ||
			(old.RootID == saved.RootID && old.RelativePath == saved.RelativePath &&
				old.Link.RunID == saved.Link.RunID && old.Link.TaskKey == saved.Link.TaskKey &&
				old.Link.SampleID == saved.Link.SampleID) {
			a.runAssets[i] = saved
			return
		}
	}
	a.runAssets = append(a.runAssets, saved)
}

func (a *app) relabelRunAsset(_ string, coordinate runs.AssetLink) error {
	if a.runSvc == nil || a.snap.RunID == "" || strings.TrimSpace(a.linkDialogAssetID) == "" {
		return fmt.Errorf("当前没有可重设坐标的 Run 资产")
	}
	updated, err := a.runSvc.UpdateAssetLinkCoordinates(
		context.Background(),
		a.snap.RunID,
		a.linkDialogAssetID,
		coordinate.WellID,
		coordinate.Timepoint,
		coordinate.FOVIndex,
	)
	if err != nil {
		return state.MapError(err)
	}
	a.upsertRunAsset(updated)
	if a.runWS != nil {
		a.refreshRunWorkspace()
	}
	return nil
}

// linkAssetPaths validates every selected path against rootDir before saving
// any link so an out-of-bounds or unreadable file leaves Core untouched.
func (a *app) linkAssetPaths(rootID, rootDir string, paths []string, coordinate runs.AssetLink) error {
	staged := make([]runs.Asset, 0, len(paths))
	for _, path := range paths {
		asset, err := assets.NewLink(rootID, rootDir, path, assets.Link{
			RunID: coordinate.RunID, TaskKey: coordinate.TaskKey,
		}, "local-user", true)
		if err != nil {
			return err
		}
		savedAsset := runAssetFromLocal(asset)
		savedAsset.Link.WellID = coordinate.WellID
		savedAsset.Link.Timepoint = coordinate.Timepoint
		savedAsset.Link.FOVIndex = coordinate.FOVIndex
		staged = append(staged, savedAsset)
	}
	for _, savedAsset := range staged {
		saved, err := a.runSvc.SaveAssetLink(context.Background(), runs.AssetLinkInput{Asset: savedAsset})
		if err != nil {
			return state.MapError(err)
		}
		a.upsertRunAsset(saved)
	}
	return nil
}

// buildRunWorkspaceVM asks Core for the frozen Run hierarchy. Go only decodes
// the contract and renders it; no containment or task identity is rebuilt here.
func (a *app) buildRunWorkspaceVM() *runworkspace.RunWorkspaceViewModel {
	if a.runSvc == nil || a.snap.RunID == "" {
		return runworkspace.BuildViewModel(a.snap, a.asterDoc, runworkspace.Projection{})
	}
	projection := runworkspace.LoadProjection(a.snap, func() (*workflowtree.Tree, error) {
		raw, err := a.runSvc.WorkflowTree(context.Background(), a.snap.RunID)
		if err != nil {
			return nil, state.MapError(err)
		}
		return workflowtree.ParseJSON(raw)
	})
	return runworkspace.BuildViewModel(a.snap, a.asterDoc, projection)
}

// loadFrozenAsterDoc loads the exact document revision frozen on the Run. The
// latest live document is never substituted for a missing or broken revision.
func (a *app) refreshRunWorkspace() {
	vm := a.buildRunWorkspaceVM()
	key := a.snap.RunID + "|" + fmt.Sprintf("%d", a.snap.Revision)
	if a.asterDoc != nil {
		key += "|" + a.asterDoc.SourceHash
	}
	a.runWSVM = vm
	a.runWSKey = key
	if a.runWS != nil {
		a.runWS.SyncFrom(vm)
	}
}

func (a *app) loadFrozenAsterDoc() {
	a.asterDoc = nil
	a.asterErr = ""
	if a.snap.AsterDoc == nil {
		return
	}
	if a.designSvc == nil {
		a.asterErr = "设计服务不可用，无法读取冻结文档"
		return
	}
	ref := a.snap.AsterDoc
	if ref.SourceHash == "" {
		a.asterErr = "Run 未记录冻结文档源码哈希"
		return
	}
	frozen, err := asterdoc.LoadFrozenAsterDocAt(
		context.Background(), a.designSvc, ref.DesignID, ref.Revision, ref.SourceHash,
	)
	if err != nil {
		a.asterErr = state.MapError(err).Error()
		return
	}
	a.asterDoc = &frozen
}

func (a *app) startPDFExport(c *ui.Context) {
	if a.pdfExporting {
		return
	}
	if a.snap.AsterDoc == nil || a.designSvc == nil {
		c.AddToast(ui.Toast{Title: "无法导出 PDF", Description: "当前 Run 未关联可读取的冻结 AsterDoc。", Type: components.ToastError})
		return
	}
	ref := *a.snap.AsterDoc
	loader := a.designSvc
	runID, title := a.snap.RunID, a.snap.Title
	designData, designID, designRevision := a.snap.DesignData, a.snap.DesignID, a.snap.DesignRevision
	if ref.SourceHash == "" {
		c.AddToast(ui.Toast{Title: "无法导出 PDF", Description: "Run 未记录冻结文档源码哈希。", Type: components.ToastError})
		return
	}

	frozenAt := ""
	if audit := a.snap.CreationAudit; audit != nil {
		frozenAt = audit.RecordedAt
	}
	runRecords := renderRunRecords(a.runWSVM, a.snap.Revision, frozenAt)
	a.pdfExporting = true
	a.pdfExportResults = make(chan pdfExportResult, 1)
	c.AddToast(ui.Toast{
		ID: "asterlab-pdf-export", Title: "正在导出 PDF…", Timeout: -1,
		Data: components.ToastContent{Leading: func() { components.Spinner(c, components.SpinnerRing) }},
	})
	results := a.pdfExportResults
	go func() {
		defer func() {
			if a.shell != nil {
				a.shell.Invalidate()
			}
		}()
		frozen, err := asterdoc.LoadFrozenAsterDocAt(
			context.Background(), loader, ref.DesignID, ref.Revision, ref.SourceHash,
		)
		if err != nil {
			results <- pdfExportResult{err: err}
			return
		}
		plan := ""
		if designData != "" {
			doc, err := protocol.Build(designData, designID, designRevision)
			if err != nil {
				results <- pdfExportResult{err: fmt.Errorf("冻结实验方案无法生成: %w", err)}
				return
			}
			plan = doc.Markdown()
		}
		home, _ := os.UserHomeDir()
		path, err := mygo.Dialog.Save(mygo.SaveDialogOptions{
			Parent: mygo.FocusedWindow(), Title: "导出 PDF",
			DefaultPath: filepath.Join(home, pdfFilename(title)), ButtonLabel: "保存",
			Filters: []mygo.FileFilter{{Name: "PDF", Extensions: []string{"pdf"}}},
		})
		if err != nil {
			results <- pdfExportResult{err: err}
			return
		}
		if path == "" {
			results <- pdfExportResult{canceled: true}
			return
		}
		if !strings.EqualFold(filepath.Ext(path), ".pdf") {
			path += ".pdf"
		}
		dir := filepath.Dir(path)
		tmp, err := os.CreateTemp(dir, ".asterlab-export-*.pdf")
		if err != nil {
			results <- pdfExportResult{err: err}
			return
		}
		tmpPath := tmp.Name()
		err = asterdoc.ExportFrozenPDFWithRunRecords(tmp, title, frozen, runID, plan, runRecords)
		if err == nil {
			err = tmp.Sync()
		}
		if closeErr := tmp.Close(); err == nil {
			err = closeErr
		}
		if err == nil {
			err = os.Rename(tmpPath, path)
		}
		if err != nil {
			_ = os.Remove(tmpPath)
			results <- pdfExportResult{err: err}
			return
		}
		results <- pdfExportResult{path: path}
	}()
}

func renderRunRecords(vm *runworkspace.RunWorkspaceViewModel, revision uint64, frozenAt string) string {
	var output strings.Builder
	fmt.Fprintf(&output, "Run Revision: %d\n", revision)
	if frozenAt == "" {
		output.WriteString("冻结时间：未记录（旧版 Run）\n")
	} else {
		fmt.Fprintf(&output, "冻结时间：%s\n", frozenAt)
	}
	output.WriteString("\n## Run 记录\n")
	if vm != nil {
		for _, task := range vm.Steps {
			appendRunTaskRecords(&output, task)
		}
		if len(vm.UnassociatedMeasurements) > 0 || len(vm.UnassociatedEvents) > 0 {
			output.WriteString("\n## 未关联记录\n这些记录缺少可确认的 task key 或 protocol step path，未归入任何步骤。\n")
			for _, measurement := range vm.UnassociatedMeasurements {
				sampleID := measurement.SampleID
				if sampleID == "" {
					sampleID = "未分配"
				}
				fmt.Fprintf(&output, "- %s · %s %s %s\n",
					sampleID, measurement.Field, measurement.Value, measurement.Unit)
			}
			for _, event := range vm.UnassociatedEvents {
				fmt.Fprintf(&output, "- %s · %s\n", runEventLabel(event.Type), event.Content)
			}
		}
	}
	return output.String()
}

func appendRunTaskRecords(output *strings.Builder, task *runworkspace.RunTaskViewModel) {
	if task == nil {
		return
	}
	if len(task.Children) > 0 {
		for _, child := range task.Children {
			appendRunTaskRecords(output, child)
		}
		return
	}
	fmt.Fprintf(output, "\n### %s", task.StepTitle)
	if task.Timepoint != "" {
		fmt.Fprintf(output, " · %s", task.Timepoint)
	}
	output.WriteByte('\n')
	if len(task.Parameters) > 0 {
		output.WriteString("计划与实际参数\n")
		for _, parameter := range task.Parameters {
			planned := parameter.Value
			if parameter.Unit != "" {
				planned += " " + parameter.Unit
			}
			fmt.Fprintf(output, "- %s: %s", parameter.Label, planned)
			if task.Actual != nil {
				if actual, ok := task.Actual.Parameters[parameter.Key]; ok && actual != planned {
					fmt.Fprintf(output, "；实际 %s", actual)
				}
			}
			output.WriteByte('\n')
		}
	}
	if task.Actual != nil && len(task.Actual.Measurements) > 0 {
		output.WriteString("实际读数\n")
		for _, measurement := range task.Actual.Measurements {
			sampleID := measurement.SampleID
			if sampleID == "" {
				sampleID = "未分配"
			}
			value := strings.TrimSpace(measurement.Field + " " + measurement.Value + " " + measurement.Unit)
			fmt.Fprintf(output, "- %s · %s\n", sampleID, value)
		}
	}
	for _, event := range task.Events {
		when := event.RecordedAt
		if when == "" {
			when = event.OccurredAt
		}
		if when == "" {
			fmt.Fprintf(output, "\n#### %s\n%s\n", runEventLabel(event.Type), event.Content)
		} else {
			fmt.Fprintf(output, "\n#### %s · %s\n%s\n", runEventLabel(event.Type), when, event.Content)
		}
		if event.BatchID != "" {
			fmt.Fprintf(output, "- Batch %s\n", event.BatchID)
		}
		if event.CheckAck {
			output.WriteString("- 检查确认\n")
		}
		if event.Deviation != "" {
			fmt.Fprintf(output, "- 偏差：%s\n", event.Deviation)
		}
		for _, check := range event.Checks {
			fmt.Fprintf(output, "- %s · %s\n", check.CheckID, check.Status)
		}
		for _, material := range event.Materials {
			fmt.Fprintf(output, "- %s %.3g %s\n", material.Material, material.Amount, material.Unit)
		}
		for _, parameter := range event.Parameters {
			value := parameter.Value
			if parameter.Unit != "" {
				value += " " + parameter.Unit
			}
			fmt.Fprintf(output, "- %s %s\n", parameter.Key, value)
		}
		for _, edge := range event.Lineage {
			fmt.Fprintf(output, "- %s:%s → %s:%s (%s)\n",
				edge.FromKind, edge.FromID, edge.ToKind, edge.ToID, edge.Relation)
		}
		for _, measurement := range event.Measurements {
			sampleID := measurement.SampleID
			if sampleID == "" {
				sampleID = "未分配"
			}
			fmt.Fprintf(output, "- %s · %s %s %s\n",
				sampleID, measurement.Field, measurement.Value, measurement.Unit)
		}
	}
}

func runEventLabel(kind string) string {
	switch kind {
	case "operation":
		return "操作"
	case "material_use":
		return "材料使用"
	case "measurement":
		return "测量"
	case "deviation":
		return "偏差"
	case "observation":
		return "观察"
	case "check":
		return "检查"
	default:
		return kind
	}
}

func pdfFilename(title string) string {
	name := strings.Map(func(r rune) rune {
		if strings.ContainsRune(`<>:"/\\|?*`, r) || r < 32 {
			return '_'
		}
		return r
	}, strings.TrimSpace(title))
	if name == "" || name == "." || name == ".." {
		name = "AsterLab"
	}
	return name + ".pdf"
}

func (a *app) pollPDFExport(c *ui.Context) {
	if a.pdfExportResults == nil {
		return
	}
	select {
	case result := <-a.pdfExportResults:
		a.pdfExportResults = nil
		a.pdfExporting = false
		c.CloseToast("asterlab-pdf-export")
		if result.canceled {
			return
		}
		if result.err != nil {
			c.AddToast(ui.Toast{Title: "PDF 导出失败", Description: result.err.Error(), Type: components.ToastError})
			return
		}
		c.AddToast(ui.Toast{
			Title: "PDF 已保存", Description: result.path, Type: components.ToastSuccess,
			Action: "打开 PDF", OnAction: func() { go mygo.Shell.OpenPath(result.path) },
		})
	default:
	}
}

func (a *app) documentExportButton(c *ui.Context) {
	if a.snap.AsterDoc == nil {
		return
	}
	button := components.Button(c, "导出 PDF", components.ButtonSecondary)
	button.Disabled(a.pdfExporting || a.asterErr != "")
	if button.Clicked() && !a.pdfExporting && a.asterErr == "" {
		a.startPDFExport(c)
	}
}

func (a *app) documentToolbar(c *ui.Context) {
	ui.Row(c).FillWidth().Padding(6, 12).AlignItems(ui.Center).Gap(8).Children(func() {
		if a.snap.AsterDoc != nil {
			ui.Text(c, fmt.Sprintf("冻结文档 Revision: %d", a.snap.AsterDoc.Revision)).
				FontSize(11).TextColor(c.Theme().TextMuted)
		} else if a.snap.DesignRevision > 0 {
			ui.Text(c, fmt.Sprintf("Document Revision: %d", a.snap.DesignRevision)).
				FontSize(11).TextColor(c.Theme().TextMuted)
		} else {
			ui.Text(c, "尚未关联文档 Revision").FontSize(11).TextColor(c.Theme().TextMuted)
		}
		if a.snap.AsterDoc != nil {
			label := "查看冻结文档"
			if a.showFrozenDoc {
				label = "隐藏冻结文档"
			}
			if components.Button(c, label, components.ButtonGhost).Clicked() {
				a.showFrozenDoc = !a.showFrozenDoc
			}
		}
		a.documentExportButton(c)
	})
}

func (a *app) renderFrozenAsterDoc(c *ui.Context, mode asterdoc.Mode) {
	t := c.Theme()
	if a.asterErr != "" {
		ui.Text(c, "冻结文档无法读取："+a.asterErr).FontSize(t.Rem(.85)).TextColor(t.Danger)
		return
	}
	if a.asterDoc == nil {
		ui.Text(c, "正在读取冻结文档…").FontSize(t.Rem(.85)).TextColor(t.TextMuted)
		return
	}
	doc, err := asterdoc.Parse(a.asterDoc.Source)
	if err != nil {
		ui.Text(c, "冻结文档语法无法解析："+err.Error()).FontSize(t.Rem(.85)).TextColor(t.Danger)
		return
	}
	view := &asterdoc.View{
		Doc: doc, Registry: asterdoc.BuiltinRegistry(), Mode: mode,
		ReadOnly: true, DocumentRevision: fmt.Sprint(a.asterDoc.Revision),
	}
	view.Render(c)
}

//go:embed starter.asterml
var starterAsterDoc string

//go:embed blank.asterml
var blankAsterDoc string

func (a *app) createAsterDocDesign(source, title string) {
	if a.designSvc == nil {
		msg := "设计服务不可用"
		a.syncErr = msg
		if a.shell != nil {
			a.shell.Error = msg
		}
		return
	}
	if len(a.projects) == 0 {
		msg := "没有可用项目；请先从 Core 导入项目"
		a.syncErr = msg
		if a.shell != nil {
			a.shell.Error = msg
		}
		return
	}
	project := a.projects[0]
	doc, err := a.designSvc.CreateAsterDocDesign(context.Background(), runs.AsterDocCreateInput{
		ProjectID: project.ProjectID, Title: title, Source: source,
		OperationID: a.nextOp(), ActorID: os.Getenv("USERNAME"),
	})
	if err != nil {
		msg := state.MapError(err).Error()
		a.syncErr = msg
		if a.shell != nil {
			a.shell.Error = msg
		}
		return
	}
	a.syncErr = ""
	if a.shell != nil {
		a.shell.Designs = append([]shell.DesignItem{{
			ID: doc.DesignID, Title: title, Meta: fmt.Sprintf("rev %d", doc.Revision),
		}}, a.shell.Designs...)
		a.shell.Selected = doc.DesignID
		a.shell.Page = shell.Design
		a.shell.Error = ""
	}
	a.loadDesign(doc.DesignID)
	a.saveSession()
}

func (a *app) selectRun(runID string) {
	if runID == "" {
		return
	}
	a.pendingRunID = runID
	if a.runSvc == nil {
		err := fmt.Errorf("Rust RunService 不可用")
		a.applyRun(nil, err)
		if a.shell != nil {
			a.shell.Error = err.Error()
		}
		return
	}
	snap, err := a.runSvc.LoadRun(context.Background(), runID)
	if err != nil {
		mapped := state.MapError(err)
		a.applyRun(nil, mapped)
		if a.shell != nil {
			a.shell.Error = mapped.Error()
		}
		return
	}
	a.applyRun(&snap, nil)
	if a.shell != nil {
		a.shell.Error = ""
		a.shell.Page = shell.Run
		a.shell.Selected = snap.DesignID
		a.saveSession()
	}
}

func (a *app) retryLoadRun() {
	runID := a.pendingRunID
	if runID == "" {
		runID = os.Getenv("ASTERLAB_RUN_ID")
	}
	if runID == "" {
		runID = a.snap.RunID
	}
	if runID == "" {
		snap, err := a.fetchRun()
		a.applyRun(snap, err)
		return
	}
	a.selectRun(runID)
}

type workspaceSession struct {
	Project       string                                  `json:"project"`
	AssetRoots    map[string]string                       `json:"asset_roots,omitempty"`
	CanvasLayouts map[string]map[string]workflow.Position `json:"canvas_layouts,omitempty"`
	DesignID      string                                  `json:"design_id"`
	RunID         string                                  `json:"run_id"`
	TaskKey       string                                  `json:"task_key,omitempty"`
	TaskPath      string                                  `json:"task_path,omitempty"`
	DesignFocus   string                                  `json:"design_focus,omitempty"`
	DesignPreview bool                                    `json:"design_preview,omitempty"`
	DesignSubflow string                                  `json:"design_subflow,omitempty"`
	Collapsed     map[string]bool                         `json:"collapsed,omitempty"`
	Page          string                                  `json:"page"`
	DesignTab     int                                     `json:"design_tab"`
	UpdatedAt     string                                  `json:"updated_at"`
}

func workspaceDataDir() string {
	if dir := os.Getenv("LABWORKBENCH_DATA_DIR"); dir != "" {
		return dir
	}
	if dir, err := os.UserConfigDir(); err == nil {
		return filepath.Join(dir, "AsterLab", "data")
	}
	return ""
}

func (a *app) sessionPath() string {
	dir := workspaceDataDir()
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, "workspace-session.json")
}

func (a *app) saveSession() {
	if a.shell == nil || a.designDoc == nil {
		if a.shell == nil {
			return
		}
	}
	path := a.sessionPath()
	if path == "" {
		return
	}
	session := workspaceSession{
		Project: a.shell.Project, DesignID: a.shell.Selected,
		Page: a.shell.PageName(), DesignTab: a.designTab,
		CanvasLayouts: a.designLayouts,
		AssetRoots:    a.assetRoots,
		UpdatedAt:     time.Now().Format(time.RFC3339),
	}
	if a.snap.RunID != "" {
		// The selected Run belongs to the workspace, not only to the Run page.
		// Keeping it lets Design/Notebook/Tools restore with their run context.
		session.RunID = a.snap.RunID
	}
	if a.designDoc != nil {
		session.DesignFocus = a.designSelected
		session.DesignPreview = a.designPreviewExpanded
		if a.designSubflow != nil {
			session.DesignSubflow = a.designSubflowParent
		}
	}
	if a.runWS != nil {
		collapsed := a.runWS.TreeCollapsed()
		if len(collapsed) > 0 {
			session.Collapsed = collapsed
		}
		if a.runWS.Selected != "" {
			for _, task := range a.snap.Tasks {
				if task.Key == a.runWS.Selected {
					session.TaskKey = task.Key
					session.TaskPath = taskStepPath(task)
					break
				}
			}
		}
	}
	raw, err := json.Marshal(session)
	if err != nil {
		return
	}
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	_ = os.WriteFile(path, raw, 0o644)
}

func (a *app) restoreSession() {
	path := a.sessionPath()
	if path == "" {
		return
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var session workspaceSession
	if json.Unmarshal(raw, &session) != nil {
		return
	}
	if session.AssetRoots != nil {
		a.assetRoots = session.AssetRoots
	}
	if session.CanvasLayouts != nil {
		a.designLayouts = session.CanvasLayouts
	}
	if session.Project != "" {
		a.shell.Project = session.Project
	}
	loadedRun := false
	if session.RunID != "" {
		a.selectRun(session.RunID)
		loadedRun = a.snap.RunID != ""
		if loadedRun && a.runWS != nil {
			if session.Collapsed != nil {
				a.runWS.Collapsed = session.Collapsed
				a.runWS.SetTreeCollapsed(session.Collapsed)
			}
			if key := restoreTaskKey(a.snap.Tasks, session.TaskKey, session.TaskPath); key != "" {
				a.ws.Selected = key
				a.runWS.Selected = key
				a.prepareGrid()
			}
		}
	}
	if session.Page == "design" && session.DesignID != "" {
		a.selectDesign(session.DesignID)
		a.shell.Page = shell.Design
		a.designTab = session.DesignTab
		if a.designDoc != nil {
			a.designSelected = session.DesignFocus
			a.designPreviewExpanded = session.DesignPreview
			if session.DesignSubflow != "" {
				a.enterDesignSubflow(session.DesignSubflow)
			}
		}
		a.saveSession()
		return
	}
	switch session.Page {
	case "project":
		a.shell.Page = shell.Project
	case "tools":
		a.shell.Page = shell.Tools
	case "notebook":
		a.shell.Page = shell.Notebook
	case "run":
		if loadedRun {
			a.shell.Page = shell.Run
		}
	default:
		if loadedRun {
			a.shell.Page = shell.Run
		}
	}
	a.saveSession()
}

func (a *app) rememberRun(snap runs.RunSnapshot) {
	if a.shell == nil || snap.RunID == "" {
		return
	}
	meta := snap.Status
	if meta == "" {
		meta = fmt.Sprintf("rev %d", snap.Revision)
	}
	next := []shell.RecentRun{{ID: snap.RunID, Title: snap.Title, Meta: meta}}
	for _, run := range a.shell.RecentRuns {
		if run.ID != snap.RunID {
			next = append(next, run)
		}
	}
	if len(next) > 5 {
		next = next[:5]
	}
	a.shell.RecentRuns = next
}

// selectTaskFromEnv applies an explicit automation deep link without changing
// normal task selection: ASTERLAB_TASK_PATH names a frozen step path.
func (a *app) selectTaskFromEnv() {
	path := os.Getenv("ASTERLAB_TASK_PATH")
	if path == "" || a.ws == nil {
		return
	}
	for _, task := range a.snap.Tasks {
		if task.Path == path || (task.Path == "" && task.Title == path) {
			a.selectRunTask(task.Key)
			return
		}
	}
}

// openRunLinkFromEnv applies an explicit automation deep link from
// ASTERLAB_RUN_LINK, alongside ASTERLAB_TASK_PATH. Invalid links become a
// visible shell error, never a panic. UX-P5, UX-IA-3, UX-DESIGN.
func (a *app) openRunLinkFromEnv() {
	raw := os.Getenv("ASTERLAB_RUN_LINK")
	if raw == "" {
		return
	}
	a.openRunWellLink(raw)
}

// openRunWellLink resolves asterlab://run/<runID>/well/<well> into a Run-page
// navigation intent with the well selected on the recording grid.
func (a *app) openRunWellLink(raw string) {
	link, err := state.ParseRunWellLink(raw)
	if err != nil {
		if a.shell != nil {
			a.shell.Error = err.Error()
		}
		return
	}
	a.selectRun(link.RunID)
	if a.shell == nil || a.shell.Page != shell.Run {
		return
	}
	if a.ws == nil || a.ws.Grid == nil {
		a.shell.Error = "实验已打开，但当前任务没有可定位的孔板"
		return
	}
	schema := a.ws.Grid.Schema
	row, col, ok := state.FindWellCell(link.Well, schema.RowLabels, schema.ColumnLabels)
	if !ok {
		a.shell.Error = "孔位 " + link.Well + " 不在当前任务的孔板上"
		return
	}
	a.ws.Grid.SetSelection(resultgrid.Selection{StartRow: row, StartColumn: col, EndRow: row, EndColumn: col})
	a.shell.Error = ""
}

// pollTimers refreshes the global timer strip at most every five seconds, so
// countdowns stay live without a Core call on every frame. Only successful
// reads replace the strip; a failed poll keeps the last known timers, and a
// failing service must never crash the frame loop. UX-P5, UX-DESIGN.
func (a *app) pollTimers() {
	if a.runSvc == nil || a.shell == nil {
		return
	}
	now := time.Now()
	if !a.timersPoll.IsZero() && now.Sub(a.timersPoll) < 5*time.Second {
		return
	}
	a.timersPoll = now
	timers, err := listActiveTimers(a.runSvc, now)
	if err != nil {
		return
	}
	if !equalTimers(a.shell.ActiveTimers, timers) {
		a.shell.ActiveTimers = timers
		a.shell.Invalidate()
	}
}

// listActiveTimers calls the service with a panic guard: the poller runs on
// the frame loop, so a torn implementation must degrade to stale timers.
func listActiveTimers(svc runs.RunService, now time.Time) (timers []runs.ActiveTimer, err error) {
	defer func() {
		if recover() != nil {
			timers, err = nil, fmt.Errorf("计时服务不可用")
		}
	}()
	return svc.ListActiveTimers(context.Background(), now.UTC().Format(time.RFC3339))
}

func equalTimers(a, b []runs.ActiveTimer) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].TimerID != b[i].TimerID || a[i].RunID != b[i].RunID ||
			a[i].ElapsedS != b[i].ElapsedS || remainingS(a[i]) != remainingS(b[i]) {
			return false
		}
	}
	return true
}

func remainingS(t runs.ActiveTimer) uint64 {
	if t.RemainingS == nil {
		return 0
	}
	return *t.RemainingS
}

func taskStepPath(task runs.RunTask) string {
	if task.Path != "" {
		return task.Path
	}
	return task.Title
}

// restoreTaskKey prefers the complete TaskKey. A path is only a fallback when
// exactly one frozen task carries it; duplicate paths across assays or
// timepoints are ambiguous and must not select an arbitrary occurrence.
func restoreTaskKey(tasks []runs.RunTask, taskKey, taskPath string) string {
	if taskKey != "" {
		for _, task := range tasks {
			if task.Key == taskKey {
				return task.Key
			}
		}
	}
	if taskPath == "" {
		return ""
	}
	key := ""
	for _, task := range tasks {
		if taskStepPath(task) != taskPath {
			continue
		}
		if key != "" {
			return ""
		}
		key = task.Key
	}
	return key
}

func currentRunTask(tasks []runs.RunTask) runs.RunTask {
	for _, task := range tasks {
		if task.Status != "completed" {
			return task
		}
	}
	if len(tasks) > 0 {
		return tasks[len(tasks)-1]
	}
	return runs.RunTask{}
}

func runTaskCacheKey(runID, taskKey string) string { return runID + "\x00" + taskKey }

func (a *app) clearTaskRecording() {
	a.taskGrids = nil
	a.taskDrafts = nil
}

func (a *app) clearTaskRecordingFor(runID, taskKey string) {
	key := runTaskCacheKey(runID, taskKey)
	delete(a.taskGrids, key)
	delete(a.taskDrafts, key)
}

// selectRunTask keeps the current task's in-memory edits before changing the
// selection. UX-P5: task navigation must never discard an unsaved draft.
func (a *app) selectRunTask(taskKey string) {
	if a.ws == nil {
		return
	}
	if a.selectedTask != "" && a.selectedTask != taskKey {
		key := runTaskCacheKey(a.snap.RunID, a.selectedTask)
		if a.taskGrids == nil {
			a.taskGrids = map[string]*resultgrid.Grid{}
		}
		if a.taskDrafts == nil {
			a.taskDrafts = map[string]runui.Draft{}
		}
		if a.ws.Grid != nil {
			a.taskGrids[key] = a.ws.Grid
		}
		a.taskDrafts[key] = a.ws.Draft
	}
	a.selectedTask = taskKey
	a.ws.Selected = taskKey
	key := runTaskCacheKey(a.snap.RunID, taskKey)
	if draft, ok := a.taskDrafts[key]; ok {
		a.ws.Draft = draft
	} else {
		a.ws.Draft = runui.NewDraft(a.ws.View)
	}
	if grid, ok := a.taskGrids[key]; ok {
		a.ws.Grid = grid
	} else {
		a.prepareGrid()
	}
	if a.runWS != nil {
		a.runWS.Selected = taskKey
		a.runWS.SetRecordingDirty(taskKey, a.ws.Grid != nil && a.ws.Grid.State != resultgrid.StateSaved)
	}
}

func (a *app) prepareGrid() {
	if a.ws == nil {
		return
	}
	if grid, ok := a.taskGrids[runTaskCacheKey(a.snap.RunID, a.ws.Selected)]; ok {
		a.ws.Grid = grid
		return
	}
	var task *runs.RunTask
	for i := range a.snap.Tasks {
		if a.snap.Tasks[i].Key == a.ws.Selected {
			task = &a.snap.Tasks[i]
			break
		}
	}
	if task == nil || task.AssayID == "" {
		a.ws.Grid = nil
		return
	}
	designData := a.snap.DesignData
	if designData == "" && a.designSnap != nil {
		designData = a.designSnap.Data
	}
	if designData == "" {
		a.ws.Grid = nil
		a.ws.Error = "运行缺少冻结设计数据"
		return
	}
	schema, cells, err := assays.BuildSchema([]byte(designData), task.AssayID, task.Timepoint)
	if err != nil {
		a.ws.Grid = nil
		a.ws.Error = state.MapError(err).Error()
		return
	}
	a.ws.Error = ""
	key := runTaskCacheKey(a.snap.RunID, task.Key)
	grid := resultgrid.New(schema, cells)
	// Batch events are scoped to one frozen task. Unscoped legacy batches
	// cannot be assigned safely when a Run has multiple tasks.
	hasSavedValues := false
	for _, event := range a.snap.Events {
		if event.TaskKey != task.Key {
			continue
		}
		for _, c := range event.ResultCells {
			cell, ok := grid.CellAt(c.Row, c.Column)
			if !ok {
				continue
			}
			if c.SampleID != "" {
				cell.SampleID = c.SampleID
			}
			if c.Group != "" {
				cell.Group = c.Group
			}
			if c.Replicate != 0 {
				cell.Replicate = c.Replicate
			}
			cell.Field, cell.Value, cell.Status = c.Field, c.Value, c.Status
			_ = grid.SetCell(cell)
			hasSavedValues = true
		}
	}
	// Old measurements do not carry task/timepoint IDs, so only replay them
	// when the Run has one task and the target is unambiguous.
	if len(a.snap.Tasks) == 1 {
		for _, measurement := range a.snap.Measurements {
			for _, cell := range grid.Cells {
				if cell.SampleID == measurement.SampleID && strings.EqualFold(cell.Field, measurement.Endpoint) {
					value := measurement.Value
					cell.Value, cell.Status = &value, "raw"
					_ = grid.SetCell(cell)
					hasSavedValues = true
					break
				}
			}
		}
	}
	if hasSavedValues {
		grid.State = resultgrid.StateSaved
	}
	if a.taskGrids != nil {
		a.taskGrids[key] = grid
	}
	a.ws.Grid = grid
}

func (a *app) loadDesign(id string) {
	if id != "" {
		a.designLoadID = id
	}
	if a.designSvc == nil {
		a.designLoadErr = "设计服务不可用"
		return
	}
	snap, err := a.designSvc.LoadDesign(context.Background(), id)
	if err != nil {
		a.designLoadErr = state.MapError(err).Error()
		return
	}
	a.designLoadErr = ""
	a.designer = designer.New(id, "", 0, a.designSvc)
	a.designer.TimeBudgetSvc = a.timeBudgetSvc
	if a.timeBudgetErr != nil {
		a.designer.TimeBudgetError = a.timeBudgetErr.Error()
	}
	a.designer.OnCreateRun = func(snap runs.RunSnapshot) {
		// The new run replaces the previous run's revision/task state.
		a.snap = snap
		a.loadRunAssets()
		a.guard = state.StaleGuard{}
		if a.ws != nil {
			view := toRunView(snap)
			a.ws.View = view
			a.ws.Notebook = notebook.Build(snap)
			a.ws.Draft = runui.NewDraft(view)
			a.ws.Selected = ""
			a.ws.Grid = nil
			if len(snap.Tasks) == 0 && len(snap.Measurements) == 0 {
				a.ws.State = runui.StateEmpty
			} else {
				a.ws.State = runui.StateReady
			}
		}
		a.prepareGrid()
		a.shell.Page = shell.Run
	}
	a.designer.Load(context.Background(), id)
	prevDoc := a.designDoc
	a.designSnap = &snap
	a.designDoc = nil
	a.designView = nil
	a.designDocErr = ""
	a.designDocLoadFailed = false
	a.retryDesignDoc = nil
	a.createRunErr = ""
	a.readiness = nil
	a.readinessErr = ""
	a.readinessLoading = false
	a.designHistory = nil
	a.designFuture = nil
	a.resetAsterDraftHistory()
	a.designCanvas = nil
	a.designCanvasSource = ""
	a.designCanvasFitPending = false
	a.designCanvasBoundsW = 0
	a.designCanvasBoundsH = 0
	a.designInspectorOpen = false
	a.designInspectorUserToggled = false
	a.designHierarchy = nil
	a.designHierarchySource = ""
	a.designHierarchyErr = ""
	a.designSubflow = nil
	a.designSubflowParent = ""
	a.designSubflowTitle = ""
	a.designSubflowSelected = ""
	// Keep per-design layouts loaded from the session; selecting a design uses
	// its own saved coordinates. (no-op)
	a.designSelected = ""
	a.canvasSelLoaded = ""
	a.stepPropSaved = ""
	if prevDoc == nil || prevDoc.DesignID != id {
		a.stepDrafts = nil
	}
	if doc, docErr := a.designSvc.LoadAsterDoc(context.Background(), id); docErr != nil {
		a.designDocErr = state.MapError(docErr).Error()
		a.designDocLoadFailed = true
		if prevDoc != nil && prevDoc.DesignID == id {
			a.designDoc = prevDoc
		}
	} else if doc.Revision > 0 {
		a.designDoc = &doc
		a.stepDrafts = nil
		a.refreshReadiness()
	}
	a.preflightOpen = false
	a.preflightRisk = false
}

func (a *app) retryDesignID() string {
	if a.designLoadID != "" {
		return a.designLoadID
	}
	return a.currentDesignID()
}

func (a *app) currentDesignID() string {
	if a.designDoc != nil && a.designDoc.DesignID != "" {
		return a.designDoc.DesignID
	}
	if a.designer != nil && a.designer.DesignID != "" {
		return a.designer.DesignID
	}
	if a.shell != nil {
		return a.shell.Selected
	}
	return ""
}

// preload fetches the design list and the Run named by ASTERLAB_RUN_ID before
// the first meaningful frame. Native services must not be called on the UI
// thread, so the fetch runs on its own goroutine and the result is applied on
// the main thread.
func (a *app) preload() {
	go func() {
		items, designErr := a.fetchDesigns()
		projects, projectErr := a.fetchProjects()
		protocols, protocolErr := a.fetchProtocols()
		runItems, runListErr := a.fetchRuns()
		snap, runErr := a.fetchRun()
		mygo.RunOnMain(func() {
			a.applyDesigns(items, designErr)
			a.applyProjects(projects, projectErr)
			a.applyProtocols(protocols, protocolErr)
			a.applyRuns(runItems, runListErr)
			a.applyRun(snap, runErr)
			if os.Getenv("ASTERLAB_RUN_ID") == "" && os.Getenv("ASTERLAB_DESIGN_ID") == "" {
				a.restoreSession()
			}
			// An explicit deep link wins over the restored session.
			a.openRunLinkFromEnv()
			a.shell.Invalidate()
		})
	}()
}

func (a *app) fetchProjects() ([]runs.ProjectSummary, error) {
	if a.designSvc == nil {
		return nil, fmt.Errorf("设计服务不可用")
	}
	items, err := a.designSvc.ListProjects(context.Background())
	if err != nil {
		return nil, state.MapError(err)
	}
	return items, nil
}

func (a *app) applyProjects(items []runs.ProjectSummary, err error) {
	if err != nil || a.shell == nil {
		return
	}
	names := make([]string, 0, len(items))
	for _, project := range items {
		names = append(names, project.Name)
	}
	a.projects = items
	a.shell.Projects = names
	if a.shell.Project == "" && len(names) > 0 {
		a.shell.Project = names[0]
	}
}

func (a *app) fetchProtocols() ([]runs.ProtocolSummary, error) {
	if a.designSvc == nil {
		return nil, fmt.Errorf("设计服务不可用")
	}
	items, err := a.designSvc.ListProtocols(context.Background())
	if err != nil {
		return nil, state.MapError(err)
	}
	return items, nil
}

func (a *app) applyProtocols(items []runs.ProtocolSummary, err error) {
	if err != nil {
		return
	}
	a.protocols = items
}

func (a *app) fetchRuns() ([]runs.RunSummary, error) {
	if a.runSvc == nil {
		return nil, fmt.Errorf("运行服务不可用")
	}
	items, err := a.runSvc.ListRuns(context.Background())
	if err != nil {
		return nil, state.MapError(err)
	}
	return items, nil
}

func (a *app) applyRuns(items []runs.RunSummary, err error) {
	if err != nil || a.shell == nil {
		return
	}
	recent := make([]shell.RecentRun, 0, len(items))
	for _, run := range items {
		meta := run.Status
		if meta == "" {
			meta = run.UpdatedAt
		}
		recent = append(recent, shell.RecentRun{ID: run.RunID, Title: run.Title, Meta: meta})
	}
	a.shell.RecentRuns = recent
}

func (a *app) fetchDesigns() ([]shell.DesignItem, error) {
	if a.designSvc == nil {
		return nil, fmt.Errorf("设计服务不可用")
	}
	ds, err := a.designSvc.ListDesigns(context.Background())
	if err != nil {
		return nil, state.MapError(err)
	}
	items := make([]shell.DesignItem, 0, len(ds))
	for _, d := range ds {
		items = append(items, shell.DesignItem{ID: d.DesignID, Title: d.Title, Meta: fmt.Sprintf("rev %d", d.Revision)})
	}
	return items, nil
}

func (a *app) applyDesigns(items []shell.DesignItem, err error) {
	if err != nil {
		a.shell.Error = err.Error()
		return
	}
	a.shell.Designs = items
	if id := os.Getenv("ASTERLAB_DESIGN_ID"); id != "" {
		for _, item := range items {
			if item.ID == id {
				a.selectDesign(id)
				break
			}
		}
	}
}

func (a *app) selectDesign(id string) {
	if id == "" {
		return
	}
	if a.designer != nil && id == a.designer.DesignID && a.designLoadErr == "" && !a.designDocLoadFailed {
		a.shell.Selected = id
		return
	}
	if a.designer != nil && a.designer.Dirty {
		a.pendingDesignID = id
		a.confirmDesignSwitch = true
		a.shell.Selected = a.designer.DesignID
		return
	}
	a.shell.Selected = id
	a.loadDesign(id)
	if a.designLoadErr != "" && a.designer != nil && a.designer.DesignID != "" {
		a.shell.Selected = a.designer.DesignID
	}
	a.saveSession()
}

func (a *app) renderDesignSwitchConfirmation(c *ui.Context) {
	if !a.confirmDesignSwitch {
		return
	}
	t := c.Theme()
	title := "所选设计"
	for _, item := range a.shell.Designs {
		if item.ID == a.pendingDesignID {
			title = item.Title
			break
		}
	}
	components.Dialog(c, &a.confirmDesignSwitch, components.DialogOptions{Label: "切换设计"}, func(panel ui.Element) {
		panel.Width(460).Children(func() {
			components.Text(c, "当前设计有未保存修改").FontSize(t.Rem(1.1)).FontWeight(600)
			components.Text(c, fmt.Sprintf("切换到“%s”前，请先保存、明确放弃修改，或继续编辑。", title)).
				TextColor(t.TextMuted)
			ui.Row(c).FillWidth().Wrap().Justify(ui.End).Gap(t.Space(2)).Children(func() {
				if components.Button(c, "继续编辑", components.ButtonSecondary).Clicked() {
					a.confirmDesignSwitch = false
					a.pendingDesignID = ""
					a.shell.Selected = a.designer.DesignID
				}
				if components.Button(c, "放弃并切换", components.ButtonOutline).Clicked() {
					id := a.pendingDesignID
					a.confirmDesignSwitch = false
					a.pendingDesignID = ""
					a.shell.Selected = id
					a.loadDesign(id)
				}
				if components.Button(c, "保存并切换", components.ButtonPrimary).Clicked() {
					id := a.pendingDesignID
					a.designer.Save(context.Background())
					if a.designer.SaveState == designer.SaveSaved {
						a.confirmDesignSwitch = false
						a.pendingDesignID = ""
						a.shell.Selected = id
						a.loadDesign(id)
					}
				}
			})
		})
	})
	if !a.confirmDesignSwitch {
		a.pendingDesignID = ""
	}
}

func (a *app) fetchRun() (*runs.RunSnapshot, error) {
	runID := os.Getenv("ASTERLAB_RUN_ID")
	if runID == "" {
		return nil, nil
	}
	if a.runSvc == nil {
		return nil, fmt.Errorf("Rust RunService 不可用")
	}
	snap, err := a.runSvc.LoadRun(context.Background(), runID)
	if err != nil {
		return nil, state.MapError(err)
	}
	return &snap, nil
}

func (a *app) applyRun(snap *runs.RunSnapshot, err error) {
	if err != nil {
		if a.ws != nil {
			a.ws.State = runui.StateBlocked
			a.ws.Error = err.Error()
		}
		if a.snap.RunID == "" || a.snap.Status == "loading" {
			a.snap = runs.RunSnapshot{Title: "实验运行加载失败", Status: "error"}
			a.runWSVM = &runworkspace.RunWorkspaceViewModel{
				Title:  "实验运行加载失败",
				Status: "error",
			}
			a.runWSKey = ""
			if a.runWS != nil {
				a.runWS.SyncFrom(a.runWSVM)
			}
		}
		if a.runWS != nil {
			a.runWS.LoadError = err.Error()
		}
		return
	}
	a.syncErr = ""
	if a.shell != nil {
		a.shell.Error = ""
	}
	if snap == nil {
		if a.noteRunID != "" && (a.noteDirty || a.noteErr != "") {
			a.persistNotebookDraft()
		}
		a.snap = runs.RunSnapshot{}
		if a.ws != nil {
			a.ws.View = toRunView(a.snap)
			a.ws.Draft = runui.NewDraft(a.ws.View)
			a.ws.Selected = ""
			a.ws.Grid = nil
			a.ws.State = runui.StateEmpty
			a.ws.Error = ""
		}
		a.runAssets = nil
		a.runWSVM = a.buildRunWorkspaceVM()
		if a.runWS != nil {
			a.runWS.SyncFrom(a.runWSVM)
		}
		return
	}
	a.applySnap(*snap)
	if a.ws != nil && a.ws.State == runui.StateLoading {
		if len(a.snap.Tasks) == 0 && len(a.snap.Measurements) == 0 {
			a.ws.State = runui.StateEmpty
		} else {
			a.ws.State = runui.StateReady
		}
	}
}

func isFinite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }

// applyDesignCommand routes a professional editor action through Core, then
// reloads the new immutable revision.
func (a *app) applyDesignCommand(kind, moduleID, group string, wells []string) {
	command := map[string]any{"type": kind, "moduleId": moduleID, "wells": wells}
	if group != "" {
		command["group"] = group
	}
	_ = a.applyDesignRaw(command)
}

// applyDesignRaw sends one semantic command and reloads the document on
// success. It is the only write seam for professional editors.
func (a *app) applyDesignRaw(command map[string]any) bool {
	if a.designSvc == nil {
		a.designDocErr = "设计服务不可用，无法保存修改"
		a.designDocLoadFailed = false
		a.retryDesignDoc = func() { _ = a.applyDesignRaw(command) }
		return false
	}
	if a.designDoc == nil {
		a.designDocErr = "尚未加载 AsterDoc 设计文档，无法应用修改"
		a.designDocLoadFailed = false
		a.retryDesignDoc = func() { _ = a.applyDesignRaw(command) }
		return false
	}
	previous := *a.designDoc
	raw, err := json.Marshal(command)
	if err != nil {
		a.designDocErr = err.Error()
		a.designDocLoadFailed = false
		a.retryDesignDoc = func() { _ = a.applyDesignRaw(command) }
		return false
	}
	updated, err := a.designSvc.ApplyAsterDocCommand(context.Background(), runs.AsterDocCommandInput{
		DesignID: a.designDoc.DesignID, ExpectedRevision: a.designDoc.Revision,
		OperationID: a.nextOp(), ActorID: os.Getenv("USERNAME"), Command: string(raw),
	})
	if err != nil {
		a.designDocErr = state.MapError(err).Error()
		a.designDocLoadFailed = false
		a.retryDesignDoc = func() { _ = a.applyDesignRaw(command) }
		return false
	}
	a.designDocErr = ""
	a.designDocLoadFailed = false
	a.retryDesignDoc = nil
	a.designHistory = append(a.designHistory, previous)
	a.designFuture = nil
	a.resetAsterDraftHistory()
	a.designDoc = &updated
	a.designView = nil
	a.designCanvas = nil
	a.designCanvasSource = ""
	a.designCanvasFitPending = false
	a.designCanvasBoundsW = 0
	a.designCanvasBoundsH = 0
	a.designInspectorOpen = false
	a.designInspectorUserToggled = false
	a.designHierarchy = nil
	a.designHierarchySource = ""
	a.designSubflow = nil
	a.designSubflowParent = ""
	a.refreshReadiness()
	return true
}

func (a *app) designUndo() {
	if a.asterDraftUndoStep() {
		return
	}
	if a.designSvc == nil || a.designDoc == nil || len(a.designHistory) == 0 {
		return
	}
	target := a.designHistory[len(a.designHistory)-1]
	current := *a.designDoc
	updated, err := a.designSvc.SaveAsterDoc(context.Background(), runs.AsterDocSaveInput{
		DesignID: a.designDoc.DesignID, Source: target.Source,
		ExpectedRevision: a.designDoc.Revision, OperationID: a.nextOp(),
		ActorID: os.Getenv("USERNAME"),
	})
	if err != nil {
		a.designDocErr = state.MapError(err).Error()
		a.designDocLoadFailed = false
		a.retryDesignDoc = func() { a.designUndo() }
		return
	}
	a.designHistory = a.designHistory[:len(a.designHistory)-1]
	a.designFuture = append(a.designFuture, current)
	a.designDocErr = ""
	a.designDocLoadFailed = false
	a.retryDesignDoc = nil
	a.designDoc = &updated
	a.designView = nil
	a.designCanvas = nil
	a.designCanvasSource = ""
	a.designCanvasFitPending = false
	a.designCanvasBoundsW = 0
	a.designCanvasBoundsH = 0
	a.designInspectorOpen = false
	a.designInspectorUserToggled = false
	a.designHierarchy = nil
	a.designHierarchySource = ""
	a.designSubflow = nil
	a.designSubflowParent = ""
	a.refreshReadiness()
}

func (a *app) designRedo() {
	if a.asterDraftRedoStep() {
		return
	}
	if a.designSvc == nil || a.designDoc == nil || len(a.designFuture) == 0 {
		return
	}
	target := a.designFuture[len(a.designFuture)-1]
	current := *a.designDoc
	updated, err := a.designSvc.SaveAsterDoc(context.Background(), runs.AsterDocSaveInput{
		DesignID: a.designDoc.DesignID, Source: target.Source,
		ExpectedRevision: a.designDoc.Revision, OperationID: a.nextOp(),
		ActorID: os.Getenv("USERNAME"),
	})
	if err != nil {
		a.designDocErr = state.MapError(err).Error()
		a.designDocLoadFailed = false
		a.retryDesignDoc = func() { a.designRedo() }
		return
	}
	a.designFuture = a.designFuture[:len(a.designFuture)-1]
	a.designHistory = append(a.designHistory, current)
	a.designDocErr = ""
	a.designDocLoadFailed = false
	a.retryDesignDoc = nil
	a.designDoc = &updated
	a.designView = nil
	a.designCanvas = nil
	a.designCanvasSource = ""
	a.designCanvasFitPending = false
	a.designCanvasBoundsW = 0
	a.designCanvasBoundsH = 0
	a.designInspectorOpen = false
	a.designInspectorUserToggled = false
	a.designHierarchy = nil
	a.designHierarchySource = ""
	a.designSubflow = nil
	a.designSubflowParent = ""
	a.refreshReadiness()
}

func (a *app) createRunFromDesignDoc() {
	if a.designSvc == nil {
		a.createRunErr = "设计服务不可用，无法创建 Run"
		return
	}
	if a.designDoc == nil {
		a.createRunErr = "尚未加载 AsterDoc 设计文档，无法创建 Run"
		return
	}
	a.createRunErr = ""
	acknowledged := []string{}
	if a.preflightRisk && a.readiness != nil {
		acknowledged = append(acknowledged, a.readiness.Warnings...)
	}
	run, err := a.designSvc.CreateRunFromAsterDoc(context.Background(), runs.AsterDocRunInput{
		DesignID: a.designDoc.DesignID, DocumentRevision: a.designDoc.Revision,
		OperationID: a.nextOp(), AcknowledgedWarnings: acknowledged,
	})
	if err != nil {
		a.createRunErr = state.MapError(err).Error()
		return
	}
	a.createRunErr = ""
	a.snap = run
	a.loadRunAssets()
	a.guard = state.StaleGuard{}
	if a.ws != nil {
		view := toRunView(run)
		a.ws.View = view
		a.ws.Notebook = notebook.Build(run)
		a.ws.Draft = runui.NewDraft(view)
		a.ws.Selected = ""
		a.ws.Grid = nil
		a.ws.State = runui.StateReady
	}
	a.loadFrozenAsterDoc()
	a.prepareGrid()
	a.shell.Page = shell.Run
}

func (a *app) newToolsWorkspace() *tools.Workspace {
	return tools.New(tools.Callbacks{
		ListPantry: func() (runs.PantryLedger, error) {
			if a.runSvc == nil {
				return runs.PantryLedger{}, fmt.Errorf("运行服务不可用")
			}
			return a.runSvc.ListPantry(context.Background(), runs.PantryQuery{})
		},
		UpsertPantry: func(input runs.PantryUpsert) (runs.PantryItem, error) {
			if a.runSvc == nil {
				return runs.PantryItem{}, fmt.Errorf("运行服务不可用")
			}
			return a.runSvc.UpsertPantry(context.Background(), input)
		},
		DeletePantry: func(input runs.PantryDelete) error {
			if a.runSvc == nil {
				return fmt.Errorf("运行服务不可用")
			}
			_, err := a.runSvc.DeletePantry(context.Background(), input)
			return err
		},
		ExportPantry: func() (runs.PantryLabels, error) {
			if a.runSvc == nil {
				return runs.PantryLabels{}, fmt.Errorf("运行服务不可用")
			}
			return a.runSvc.ExportPantryLabels(context.Background(), runs.PantryQuery{})
		},
	})
}

func toolsPage(c *ui.Context, a *app) {
	t := c.Theme()
	tokens := theme.GetTokens(t)
	ui.Column(c).Fill().Children(func() {
		ui.Row(c).FillWidth().Padding(t.Space(2), t.Space(4)).Border(1, t.Border).
			Background(t.Surface).Children(func() {
			ui.Text(c, "资料与工具").FontSize(float32(tokens.TextSection)).Bold().TextColor(t.Text)
			ui.Spacer(c)
			ui.Text(c, "储藏室台账与方法工具").FontSize(float32(tokens.TextMeta)).TextColor(t.TextMuted)
		})
		if a.tools == nil {
			a.tools = a.newToolsWorkspace()
		}
		tools.View(c, a.tools)
	})
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "--inspect-tree" {
		if err := runInspectTree(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	if strings.EqualFold(os.Getenv("ASTERLAB_THEME"), "dark") {
		mygo.Theme.SetSource(mygo.ThemeDark)
	}
	page, step := "project", "design-l929"
	_ = step

	runSvc, runErr := runs.NewRunService()
	designSvc, designErr := runs.NewDesignService()
	timeBudgetSvc, timeBudgetErr := runs.NewTimeBudgetService()

	a := &app{
		page:       &page,
		step:       &step,
		runSvc:     runSvc,
		designSvc:  designSvc,
		timeBudgetSvc: timeBudgetSvc,
		timeBudgetErr: timeBudgetErr,
		snap:       runs.RunSnapshot{Title: "加载中", Status: "loading"},
		assetRoots: map[string]string{}, runViewKinds: map[string]runworkspace.ViewKind{},
	}
	initErr := ""
	if designErr != nil {
		initErr = "设计服务：" + designErr.Error()
	} else if runErr != nil {
		initErr = "运行服务：" + runErr.Error()
	} else if designSvc == nil {
		initErr = "设计服务返回空实现"
	}
	a.syncErr = initErr
	log.Printf("native services: run=%v(%v) design=%v(%v) time-budget=%v(%v)", runSvc != nil, runErr, designSvc != nil, designErr, timeBudgetSvc != nil, timeBudgetErr)
	a.shell = &shell.Shell{
		Project:       "",
		Page:          shell.Project,
		Error:         initErr,
		OnSelect:      func(id string) { a.selectDesign(id) },
		OnRunSelect:   func(id string) { a.selectRun(id) },
		OnTimerSelect: func(id string) { a.selectRun(id) },
		OnCreate:      func() { a.newDesignOpen = true },
		OnImport:      func() { a.importOpen = true },
		OnPage:        func(p shell.Page) { a.shell.Page = p; a.saveSession() },
		Content:       func(c *ui.Context) { contentPage(c, a) },
	}

	// runui workspace.
	cb := runui.Callbacks{
		SaveReadout: func(ms []runui.Measurement) {
			if a.ws == nil {
				return
			}
			rMs := make([]runs.Measurement, 0, len(ms))
			for _, m := range ms {
				rMs = append(rMs, runs.Measurement{SampleID: m.SampleID, Endpoint: m.Endpoint, Value: m.Value, Unit: m.Unit})
			}
			taskKey := a.ws.Selected
			timepoint, assayID, stepPath, protocolVersion := "", "", "", ""
			for _, t := range a.snap.Tasks {
				if t.Key == taskKey {
					timepoint, assayID, stepPath, protocolVersion = t.Timepoint, t.AssayID, taskStepPath(t), t.ProtocolVersionID
				}
			}
			if taskKey == "" && len(a.snap.Tasks) > 0 {
				t := a.snap.Tasks[0]
				taskKey, timepoint, assayID, stepPath, protocolVersion = t.Key, t.Timepoint, t.AssayID, taskStepPath(t), t.ProtocolVersionID
			}
			opID := a.nextOp()
			snap, err := a.runSvc.SaveReadout(context.Background(), runs.ReadoutInput{
				RunID:             a.snap.RunID,
				TaskKey:           taskKey,
				Timepoint:         timepoint,
				AssayID:           assayID,
				ProtocolStepPath:  stepPath,
				ProtocolVersionID: protocolVersion,
				Measurements:      rMs,
				OperationID:       opID,
			})
			if err != nil {
				a.ws.State = runui.StateBlocked
				a.ws.Error = state.MapError(err).Error()
				a.ws.Draft.Error = state.MapError(err).Error()
				return
			}
			a.clearTaskRecordingFor(a.snap.RunID, taskKey)
			a.applySnap(snap)
			if a.ws != nil {
				a.ws.State = runui.StateSaved
				if a.ws.Grid != nil {
					a.ws.Grid.State = resultgrid.StateSaved
				}
			}
			if a.runWS != nil {
				a.runWS.SetRecordingDirty(taskKey, false)
			}
		},
		SaveBatch: func(schema resultgrid.Schema, cells []resultgrid.Cell) {
			a.saveBatch(schema, cells)
		},
		SaveChecks: func(taskKey string, checks []runui.CheckAck) {
			if a.ws == nil {
				return
			}
			timepoint, assayID, stepPath, protocolVersion := "", "", "", ""
			for _, t := range a.snap.Tasks {
				if t.Key == taskKey {
					timepoint, assayID, stepPath, protocolVersion = t.Timepoint, t.AssayID, taskStepPath(t), t.ProtocolVersionID
				}
			}
			acks := make([]runs.CheckAck, 0, len(checks))
			for _, check := range checks {
				acks = append(acks, runs.CheckAck{CheckID: check.CheckID, Status: check.Status})
			}
			snap, err := a.runSvc.SaveTaskChecks(context.Background(), runs.TaskCheckInput{
				RunID: a.snap.RunID, TaskKey: taskKey, OperationID: a.nextOp(),
				AssayID: assayID, Timepoint: timepoint,
				ProtocolStepPath: stepPath, ProtocolVersionID: protocolVersion,
				Checks: acks,
			})
			if err != nil {
				log.Printf("save task checks: %v", err)
				a.ws.CheckSaving = false
				a.ws.SyncChecks()
				a.ws.State = runui.StateError
				a.ws.Error = state.MapError(err).Error()
				return
			}
			a.applySnap(snap)
		},
		CompleteTask: func(taskKey string, checks []runui.CheckAck) {
			if a.ws == nil {
				return
			}
			if taskKey == "" {
				taskKey = a.ws.Selected
			}
			opID := a.nextOp()
			timepoint, assayID, stepPath, protocolVersion := "", "", "", ""
			for _, t := range a.snap.Tasks {
				if t.Key == taskKey {
					timepoint, assayID, stepPath, protocolVersion = t.Timepoint, t.AssayID, taskStepPath(t), t.ProtocolVersionID
				}
			}
			acks := make([]runs.CheckAck, 0, len(checks))
			for _, check := range checks {
				acks = append(acks, runs.CheckAck{CheckID: check.CheckID, Status: check.Status})
			}
			snap, err := a.runSvc.CompleteTask(context.Background(), runs.CompleteTaskInput{
				RunID:             a.snap.RunID,
				TaskKey:           taskKey,
				OperationID:       opID,
				AssayID:           assayID,
				Timepoint:         timepoint,
				ProtocolStepPath:  stepPath,
				ProtocolVersionID: protocolVersion,
				Checks:            acks,
			})
			if err != nil {
				a.ws.State = runui.StateError
				a.ws.Error = state.MapError(err).Error()
				return
			}
			a.applySnap(snap)
			if a.ws != nil {
				a.ws.State = runui.StateReady
			}
		},
		SelectTask: func(taskKey string) {
			a.selectRunTask(taskKey)
		},
		OpenBatch: func(batchID string) {
			a.batchView = batchID
		},
	}

	// runworkspace callbacks reuse the same Core seams as legacy runui.
	rwcb := runworkspace.Callbacks{
		SelectTask: func(taskKey string) {
			a.selectRunTask(taskKey)
			a.saveSession()
		},
		ToggleTree: func() {
			a.saveSession()
		},
		SaveChecks: func(taskKey string, checks []runworkspace.CheckAckView) error {
			return a.saveRunChecks(taskKey, checks)
		},
		RetryLoad: func() {
			a.retryLoadRun()
		},
		Complete: func(taskKey string, checks []runworkspace.CheckAckView) error {
			if taskKey == "" && a.runWS != nil {
				taskKey = a.runWS.Selected
			}
			opID := a.nextOp()
			timepoint, assayID, stepPath, protocolVersion := "", "", "", ""
			for _, t := range a.snap.Tasks {
				if t.Key == taskKey {
					timepoint, assayID, stepPath, protocolVersion = t.Timepoint, t.AssayID, taskStepPath(t), t.ProtocolVersionID
				}
			}
			acks := make([]runs.CheckAck, 0, len(checks))
			for _, check := range checks {
				acks = append(acks, runs.CheckAck{CheckID: check.CheckID, Status: check.Status})
			}
			snap, err := a.runSvc.CompleteTask(context.Background(), runs.CompleteTaskInput{
				RunID:             a.snap.RunID,
				TaskKey:           taskKey,
				OperationID:       opID,
				AssayID:           assayID,
				Timepoint:         timepoint,
				ProtocolStepPath:  stepPath,
				ProtocolVersionID: protocolVersion,
				Checks:            acks,
			})
			if err != nil {
				mappedErr := state.MapError(err)
				return mappedErr
			}
			a.applySnap(snap)
			return nil
		},
		OpenBatch: func(batchID string) {
			a.batchView = batchID
		},
		Recording: func(c *ui.Context, task *runworkspace.RunTaskViewModel) {
			key := runTaskCacheKey(a.snap.RunID, task.TaskKey)
			grid := (*resultgrid.Grid)(nil)
			if a.ws != nil {
				if a.ws.Selected == task.TaskKey {
					grid = a.ws.Grid
				} else {
					grid = a.taskGrids[key]
				}
			}
			if a.runWS != nil {
				a.runWS.SetRecordingDirty(task.TaskKey, grid != nil && grid.State != resultgrid.StateSaved)
			}
			linkedAssets := a.runAssetsForTask(task.TaskKey)
			kindKey := a.snap.RunID + "\x00" + task.TaskKey
			if a.runViewKinds == nil {
				a.runViewKinds = map[string]runworkspace.ViewKind{}
			}
			kind, ok := a.runViewKinds[kindKey]
			if !ok {
				kind = runworkspace.SelectKind(task, grid, linkedAssets)
			}
			linkErr := ""
			if a.linkErr != "" && (a.linkErrTask == "" || a.linkErrTask == task.TaskKey) {
				linkErr = a.linkErr
			}
			assetErr := ""
			var onRetryAsset func()
			if a.assetErr != "" && (a.assetErrTask == "" || a.assetErrTask == task.TaskKey) {
				assetErr = a.assetErr
				onRetryAsset = a.assetErrRetry
			}
			rv := runworkspace.RecordingView{
				RunID: a.snap.RunID, Kind: kind, Grid: grid, Assets: linkedAssets, Task: task,
				LinkErr:         linkErr,
				AssetErr:        assetErr,
				OnRetryAssetErr: onRetryAsset,
				OnKindChange: func(kind runworkspace.ViewKind) {
					a.runViewKinds[kindKey] = kind
				},
				OnTimeline: func() {
					a.runWS.WorkspaceTab = 2
					a.runWS.SelectedTimelineTask = task.TaskKey
				},
				OnOpenAsset: func(asset runworkspace.LinkedAsset) {
					if err := a.tryOpenRunAsset(task.TaskKey, asset); err != nil {
						c.AddToast(ui.Toast{Title: "无法打开文件", Description: err.Error(), Type: components.ToastError})
					}
				},
				OnRelocate: func(asset runworkspace.LinkedAsset) {
					if err := a.tryRelocateRunAsset(task.TaskKey, asset); err != nil {
						c.AddToast(ui.Toast{Title: "文件重新定位失败", Description: err.Error(), Type: components.ToastError})
					}
				},
				OnRelabelAsset: func(asset runworkspace.LinkedAsset) {
					a.openAssetLinkForm(task.TaskKey, asset)
					if a.shell != nil {
						a.shell.Invalidate()
					}
				},
				OnLinkFile: func(prefill runworkspace.LinkedAsset) {
					a.openAssetLinkForm(task.TaskKey, prefill)
					if a.shell != nil {
						a.shell.Invalidate()
					}
				},
				OnSaveBatch: func(cells []resultgrid.Cell) {
					if grid == nil || grid.State == resultgrid.StateSaving {
						return
					}
					a.startSaveBatch(grid, task.TaskKey, cells)
				},
			}
			rv.Render(c)
		},
	}
	a.tools = a.newToolsWorkspace()
	a.ws = runui.NewWorkspace(toRunView(a.snap), cb)
	a.selectedTask = a.ws.Selected
	a.runWSVM = a.buildRunWorkspaceVM()

	a.runWS = runworkspace.NewState(a.runWSVM, rwcb)
	// The Run surface renders the frozen design's modules through the same
	// blockui the Designer uses: Run for open tasks, Review once completed.
	a.ws.Modules = func(c *ui.Context, task runui.Task) {
		t := c.Theme()
		if a.snap.DesignData == "" {
			ui.Text(c, "没有冻结设计数据").FontSize(12).TextColor(t.TextMuted)
			return
		}
		design, err := designer.DecodeBlocks(a.snap.DesignData)
		if err != nil {
			ui.Text(c, "冻结设计无法读取："+err.Error()).FontSize(12).TextColor(t.Danger)
			return
		}
		insts := design.Instances[task.DesignStepID]
		if len(insts) == 0 {
			ui.Text(c, "该步骤在设计里没有模块").FontSize(12).TextColor(t.TextMuted)
		}
		mode := blockui.Run
		if task.Status == "completed" {
			mode = blockui.Review
		}
		for i := range insts {
			if blockui.View(c, &insts[i], mode) {
				// The frozen design is read-only here by contract; blockui only
				// reports edits in Design mode.
			}
		}
		allModules := sdkModulesFromDesign(a.snap.DesignData)
		refs := stepModuleRefs(a.snap.DesignData, task.DesignStepID)
		modules := make([]map[string]any, 0, len(allModules))
		for _, module := range allModules {
			if id, _ := module["id"].(string); refs[id] {
				modules = append(modules, module)
			}
		}
		if len(modules) == 0 {
			if len(allModules) > 0 {
				ui.Text(c, "该任务未绑定 SDK 模块").FontSize(t.Rem(.75)).TextColor(t.TextMuted)
			}
			return
		}
		ui.Text(c, "SDK 模块 · 本任务绑定").FontSize(t.Rem(.8)).Bold().TextColor(t.TextMuted)
		frozen := &asterdoc.View{Mode: asterdoc.ModeRun, ReadOnly: true}
		for _, module := range modules {
			kind, _ := module["kind"].(string)
			schema := 1
			switch value := module["schema"].(type) {
			case float64:
				schema = int(value)
			case int:
				schema = value
			}
			frozen.RenderModule(c, kind, schema, module)
		}
	}
	a.ws.State = runui.StateLoading
	a.ws.Width = 1200

	mygo.App.WhenReady(func() {
		a.shell.NewWindow()
		a.windowReady = true
		a.shell.Quit()
		a.preload()
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
func (a *app) saveRunChecks(taskKey string, checks []runworkspace.CheckAckView) error {
	if a.runWS == nil {
		return nil
	}
	if a.runSvc == nil {
		return fmt.Errorf("运行服务不可用")
	}
	timepoint, assayID, stepPath, protocolVersion := "", "", "", ""
	for _, t := range a.snap.Tasks {
		if t.Key == taskKey {
			timepoint, assayID, stepPath, protocolVersion = t.Timepoint, t.AssayID, taskStepPath(t), t.ProtocolVersionID
		}
	}
	acks := make([]runs.CheckAck, 0, len(checks))
	for _, check := range checks {
		acks = append(acks, runs.CheckAck{CheckID: check.CheckID, Status: check.Status})
	}
	snap, err := a.runSvc.SaveTaskChecks(context.Background(), runs.TaskCheckInput{
		RunID: a.snap.RunID, TaskKey: taskKey, OperationID: a.nextOp(),
		AssayID: assayID, Timepoint: timepoint,
		ProtocolStepPath: stepPath, ProtocolVersionID: protocolVersion,
		Checks: acks,
	})
	if err != nil {
		log.Printf("save task checks: %v", err)
		return state.MapError(err)
	}
	a.applySnap(snap)
	return nil
}

type batchSaveRequest struct {
	grid       *resultgrid.Grid
	runID      string
	taskKey    string
	input      runs.BatchReadoutInput
	totalCells int
}

type batchSaveOutcome struct {
	result  runs.BatchReadoutResult
	err     error
	snap    runs.RunSnapshot
	loadErr error
}

func (a *app) prepareBatchSaveForTask(grid *resultgrid.Grid, taskKey string, schema resultgrid.Schema, cells []resultgrid.Cell) (batchSaveRequest, bool) {
	if grid == nil && a.ws != nil {
		grid = a.ws.Grid
	}
	if a.runSvc == nil {
		msg := "运行服务不可用"
		if grid != nil {
			grid.Error = msg
			grid.State = resultgrid.StateError
		}
		if a.ws != nil {
			a.ws.State = runui.StateError
			a.ws.Error = msg
		}
		return batchSaveRequest{}, false
	}
	if grid == nil {
		return batchSaveRequest{}, false
	}
	if taskKey == "" && a.ws != nil {
		taskKey = a.ws.Selected
	}
	task := currentRunTask(a.snap.Tasks)
	for _, candidate := range a.snap.Tasks {
		if candidate.Key == taskKey {
			task = candidate
			break
		}
	}
	batchCells := make([]runs.ResultCell, 0, len(cells))
	for _, cell := range cells {
		batchCells = append(batchCells, runs.ResultCell{
			Row: cell.Row, Column: cell.Column, SampleID: cell.SampleID,
			Group: cell.Group, Replicate: cell.Replicate, Field: cell.Field,
			Value: cell.Value, Status: cell.Status,
		})
	}
	grid.State = resultgrid.StateSaving
	grid.Error = ""
	if a.ws != nil {
		a.ws.State = runui.StateSaving
		a.ws.Error = ""
	}
	cellsDigest := grid.CellsDigest()
	if grid.PendingOpID == "" || grid.PendingOpHash != cellsDigest {
		grid.PendingOpID = a.nextOp()
		grid.PendingOpHash = cellsDigest
	}
	_ = runworkspace.SaveGridDraft(a.snap.RunID, task.Key, grid)
	return batchSaveRequest{
		grid:       grid,
		runID:      a.snap.RunID,
		taskKey:    task.Key,
		totalCells: len(batchCells),
		input: runs.BatchReadoutInput{
			RunID: a.snap.RunID, OperationID: grid.PendingOpID, TaskKey: task.Key,
			AssayID: task.AssayID, Timepoint: task.Timepoint,
			ProtocolStepPath: taskStepPath(task), ProtocolVersionID: task.ProtocolVersionID,
			SchemaID: schema.ID, SchemaVersion: schema.Version, Cells: batchCells,
			ExpectedRevision: a.snap.Revision,
		},
	}, true
}

func executeBatchSave(svc runs.RunService, req batchSaveRequest) batchSaveOutcome {
	batchResult, err := svc.SaveReadoutsBatch(context.Background(), req.input)
	if err != nil {
		return batchSaveOutcome{err: err}
	}
	if batchResult.Accepted != req.totalCells {
		return batchSaveOutcome{result: batchResult}
	}
	snap, loadErr := svc.LoadRun(context.Background(), req.runID)
	return batchSaveOutcome{result: batchResult, snap: snap, loadErr: loadErr}
}

func (a *app) applyBatchSaveOutcome(req batchSaveRequest, out batchSaveOutcome) {
	setErr := func(message string) {
		if req.grid != nil {
			req.grid.Error = message
			req.grid.State = resultgrid.StateError
		}
		if a.ws != nil {
			a.ws.State = runui.StateError
			a.ws.Error = message
		}
	}
	if out.err != nil {
		setErr(state.MapError(out.err).Error())
		return
	}
	if out.result.Accepted != req.totalCells {
		setErr(fmt.Sprintf("批量保存未接受全部单元格：%d/%d", out.result.Accepted, req.totalCells))
		return
	}
	if out.loadErr != nil {
		if out.result.Revision > a.snap.Revision {
			a.snap.Revision = out.result.Revision
		}
		setErr("已写入 Core，但刷新 Run 失败：" + state.MapError(out.loadErr).Error())
		return
	}
	savedGrid := req.grid
	savedGrid.PendingOpID = ""
	savedGrid.PendingOpHash = ""
	savedGrid.Error = ""
	savedGrid.State = resultgrid.StateSaved
	runworkspace.ClearGridDraft(req.runID, req.taskKey)
	a.clearTaskRecordingFor(req.runID, req.taskKey)
	a.applySnap(out.snap)
	if a.ws != nil {
		a.ws.State = runui.StateSaved
		a.ws.Error = ""
		if a.ws.Selected == req.taskKey || a.ws.Selected == "" {
			a.ws.Grid = savedGrid
		}
	}
	if a.taskGrids == nil {
		a.taskGrids = map[string]*resultgrid.Grid{}
	}
	a.taskGrids[runTaskCacheKey(req.runID, req.taskKey)] = savedGrid
	if a.runWS != nil {
		a.runWS.SetRecordingDirty(req.taskKey, false)
	}
}

func (a *app) saveBatch(schema resultgrid.Schema, cells []resultgrid.Cell) {
	if a.ws == nil {
		return
	}
	req, ok := a.prepareBatchSaveForTask(a.ws.Grid, a.ws.Selected, schema, cells)
	if !ok {
		return
	}
	out := executeBatchSave(a.runSvc, req)
	a.applyBatchSaveOutcome(req, out)
}

func (a *app) startSaveBatch(grid *resultgrid.Grid, taskKey string, cells []resultgrid.Cell) {
	if grid == nil {
		return
	}
	req, ok := a.prepareBatchSaveForTask(grid, taskKey, grid.Schema, cells)
	if !ok {
		return
	}
	if !a.windowReady {
		out := executeBatchSave(a.runSvc, req)
		a.applyBatchSaveOutcome(req, out)
		return
	}
	if a.shell != nil {
		a.shell.Invalidate()
	}
	svc := a.runSvc
	go func() {
		out := executeBatchSave(svc, req)
		mygo.RunOnMain(func() {
			a.applyBatchSaveOutcome(req, out)
			if a.shell != nil {
				a.shell.Invalidate()
			}
		})
	}()
}

func (a *app) renderSyncErrorBanner(c *ui.Context) {
	if a.syncErr == "" {
		return
	}
	t := c.Theme()
	tokens := theme.GetTokens(t)
	ui.Row(c).FillWidth().Padding(t.Space(1), t.Space(2)).
		Background(tokens.DangerBg).Border(1, tokens.DangerText).
		AlignItems(ui.Center).Gap(t.Space(1.5)).Children(func() {
		ui.Box(c).Grow(1).MinWidth(0).Children(func() {
			ui.Text(c, a.syncErr).FontSize(t.Rem(.78)).TextColor(tokens.DangerText)
		})
		if a.runSvc != nil && (a.pendingRunID != "" || a.snap.RunID != "") {
			if components.Button(c, "重新加载 Run", components.ButtonSecondary).Clicked() {
				a.retryLoadRun()
			}
		}
		if components.Button(c, "忽略", components.ButtonGhost).Clicked() {
			a.syncErr = ""
		}
	})
}

func contentPage(c *ui.Context, a *app) {
	a.pollPDFExport(c)
	a.pollTimers()
	ui.Column(c).Fill().Children(func() {
		a.renderSyncErrorBanner(c)
		ui.Box(c).Grow(1).MinHeight(0).FillWidth().Children(func() {
			switch a.shell.Page {
			case shell.Design:
				designPage(c, a)
			case shell.Run:
				runPage(c, a)
			case shell.Protocol:
				protocolPage(c, a)
			case shell.Notebook:
				notebookPage(c, a)
			case shell.Tools:
				toolsPage(c, a)
			}
		})
	})
	a.renderNewDesignDialog(c)
	a.renderImportDialog(c)
	a.renderDesignSwitchConfirmation(c)
	a.renderAssetLinkDialog(c)
}

func (a *app) renderAssetLinkDialog(c *ui.Context) {
	if !a.linkDialogOpen {
		return
	}
	t := c.Theme()
	tokens := theme.GetTokens(t)
	open := a.linkDialogOpen
	relabelMode := a.linkDialogAssetID != ""
	title := "关联本地文件"
	subtitle := "先标注孔位、时间点和视野；所选坐标应用于本次选择的每个文件。"
	submitLabel := "选择文件"
	if relabelMode {
		title = "重设资产坐标"
		subtitle = "为未标注资产补齐孔位、时间点和视野坐标。"
		submitLabel = "保存坐标"
	}
	components.Dialog(c, &open, components.DialogOptions{Label: title}, func(panel ui.Element) {
		panel.Width(520).Children(func() {
			ui.Text(c, title).FontSize(float32(tokens.TextBody)).Bold().TextColor(t.Text)
			ui.Text(c, subtitle).
				FontSize(float32(tokens.TextMeta)).TextColor(t.TextMuted)
			ui.Row(c).FillWidth().Gap(t.Space(1)).AlignItems(ui.Center).Children(func() {
				ui.Text(c, "孔位").FontSize(float32(tokens.TextMeta)).TextColor(t.TextMuted).Width(68)
				ui.Select(c, &a.linkDialogWell, a.linkDialogWells).FillWidth()
			})
			ui.Row(c).FillWidth().Gap(t.Space(1)).AlignItems(ui.Center).Children(func() {
				ui.Text(c, "时间点").FontSize(float32(tokens.TextMeta)).TextColor(t.TextMuted).Width(68)
				ui.Select(c, &a.linkDialogTimepoint, a.linkDialogTimepoints).FillWidth()
			})
			ui.Row(c).FillWidth().Gap(t.Space(1)).AlignItems(ui.Center).Children(func() {
				ui.Text(c, "FOV").FontSize(float32(tokens.TextMeta)).TextColor(t.TextMuted).Width(68)
				ui.TextInput(c, &a.linkDialogFOV).Placeholder("≥ 1").FillWidth()
			})
			if a.linkDialogErr != "" {
				ui.Text(c, a.linkDialogErr).FontSize(float32(tokens.TextMeta)).TextColor(t.Danger)
			}
			ui.Row(c).FillWidth().Wrap().Justify(ui.End).Gap(t.Space(1)).Children(func() {
				if components.Button(c, "取消", components.ButtonSecondary).Clicked() {
					a.linkDialogOpen = false
					a.linkDialogAssetID = ""
				}
				submit := components.Button(c, submitLabel, components.ButtonPrimary)
				submit.Disabled(len(a.linkDialogWells) == 0 || len(a.linkDialogTimepoints) == 0)
				if submit.Clicked() {
					if relabelMode {
						a.submitAssetLinkForm(a.relabelRunAsset)
					} else {
						a.submitAssetLinkForm(a.linkRunAsset)
					}
				}
			})
		})
	})
	if !open {
		a.linkDialogOpen = false
		a.linkDialogAssetID = ""
	}
}

func (a *app) submitAssetLinkForm(linker func(string, runs.AssetLink) error) {
	coordinate, err := validateAssetCoordinates(
		a.linkDialogWell, a.linkDialogTimepoint, a.linkDialogFOV,
		a.linkDialogWells, a.linkDialogTimepoints,
	)
	if err != nil {
		a.linkDialogErr = err.Error()
		return
	}
	a.linkDialogErr = ""
	a.linkErr = ""
	a.linkErrTask = ""
	if err := linker(a.linkDialogTask, coordinate); err != nil {
		a.linkDialogErr = err.Error()
		a.linkErr = err.Error()
		a.linkErrTask = a.linkDialogTask
		if a.shell != nil {
			a.shell.Invalidate()
		}
		return
	}
	a.linkDialogOpen = false
	a.linkDialogAssetID = ""
	if a.shell != nil {
		a.shell.Invalidate()
	}
}

// batchPage shows one stored readout batch as a read-only plate. Values come
// from the execution event; nothing here can edit a cell.
func batchPage(c *ui.Context, a *app) {
	t := c.Theme()
	tokens := theme.GetTokens(t)
	var event *runs.RunEvent
	for i := range a.snap.Events {
		if a.snap.Events[i].BatchID == a.batchView {
			event = &a.snap.Events[i]
			break
		}
	}
	header := func() {
		ui.Row(c).FillWidth().Padding(12, 16).AlignItems(ui.Center).Gap(12).
			Background(tokens.CardBackground).Border(1, tokens.CardBorder).Children(func() {
			if components.Button(c, "← 返回 Notebook", components.ButtonGhost).Clicked() {
				a.batchView = ""
			}
			ui.Text(c, "原始读数").FontSize(18).Bold().TextColor(t.Text)
			if event != nil {
				ui.Text(c, event.BatchID).FontSize(12).TextColor(t.TextMuted)
			}
			ui.Spacer(c)
			ui.Text(c, "只读 · 来自保存时的执行事件").FontSize(11).TextColor(t.TextMuted)
		})
	}
	if event == nil {
		header()
		ui.Text(c, "找不到这条批次记录；它可能已失效。").Padding(t.Space(6)).TextColor(t.Danger)
		return
	}
	schema, plan, err := assays.BuildSchema([]byte(a.snap.DesignData), event.AssayID, event.Timepoint)
	if err != nil || schema.Layout != "plate96" {
		rows := []string{"A", "B", "C", "D", "E", "F", "G", "H"}
		cols := make([]string, 12)
		for i := range cols {
			cols[i] = fmt.Sprint(i + 1)
		}
		schema = resultgrid.Schema{ID: "cck8.plate96.od450.v1", Version: 1,
			Layout: "plate96", RowLabels: rows, ColumnLabels: cols,
			Fields: []resultgrid.Field{{Key: "od450", Label: "OD450", Unit: "AU", DataType: "number"}}}
		plan = nil
	}
	_ = plan
	grid := resultgrid.New(schema, batchGridCells(event.ResultCells))
	recorded, missing, excluded := 0, 0, 0
	for _, cell := range grid.Cells {
		switch {
		case cell.Status == "excluded":
			excluded++
		case cell.Value == nil:
			missing++
		default:
			recorded++
		}
	}
	header()
	ui.Scroll(c).Grow(1).Children(func() {
		ui.Column(c).FillWidth().Padding(16).Gap(12).Children(func() {
			ui.Row(c).Wrap().Gap(16).AlignItems(ui.Center).Children(func() {
				meta := func(label, value string) {
					ui.Row(c).Gap(4).Children(func() {
						ui.Text(c, label).FontSize(11).TextColor(t.TextMuted)
						ui.Text(c, value).FontSize(12).TextColor(t.Text)
					})
				}
				meta("检测", event.AssayID)
				if event.Timepoint != "" {
					meta("时间点", event.Timepoint)
				}
				if event.RecordedAt != "" {
					meta("记录时间", event.RecordedAt)
				}
				meta("有效读数", fmt.Sprintf("%d", recorded))
				meta("缺失", fmt.Sprintf("%d", missing))
				if excluded > 0 {
					meta("排除", fmt.Sprintf("%d", excluded))
				}
			})
			ui.Grid(c).Columns(len(schema.ColumnLabels)).Gap(4).Children(func() {
				for _, col := range schema.ColumnLabels {
					ui.Text(c, col).FontSize(10).TextColor(t.TextMuted).Center()
				}
				for row := range schema.RowLabels {
					for col := range schema.ColumnLabels {
						cell := grid.Cells[row*len(schema.ColumnLabels)+col]
						value := "—"
						color := t.TextMuted
						if cell.Value != nil {
							value = fmt.Sprintf("%.4g", *cell.Value)
							color = t.Text
						}
						if cell.Status == "excluded" {
							value = "排除"
							color = t.Warning
						}
						ui.Column(c).Padding(4).Gap(1).Radius(4).
							Background(tokens.CardBackground).Border(1, tokens.CardBorder).Children(func() {
							ui.Text(c, schema.RowLabels[row]+schema.ColumnLabels[col]).
								FontSize(9).TextColor(t.TextMuted)
							ui.Text(c, value).FontSize(12).FontFeatures("tnum").TextColor(color)
							sample := cell.SampleID
							if sample == "" {
								sample = "未分配"
							}
							ui.Text(c, sample).FontSize(9).TextColor(t.TextMuted).
								SingleLine().Ellipsis("…")
						})
					}
				}
			})
			ui.Text(c, "孔位、样本与数值均读取自该批次事件；此页不能修改原始读数。").
				FontSize(11).TextColor(t.TextMuted)
		})
	})
}

func batchGridCells(cells []runs.ResultCell) []resultgrid.Cell {
	out := make([]resultgrid.Cell, len(cells))
	for i, cell := range cells {
		out[i] = resultgrid.Cell{Row: cell.Row, Column: cell.Column, SampleID: cell.SampleID,
			Group: cell.Group, Replicate: cell.Replicate, Field: cell.Field,
			Value: cell.Value, Status: cell.Status}
	}
	return out
}

func (a *app) refreshReadiness() {
	a.readinessErr = ""
	if a.designDoc == nil {
		a.readinessLoading = false
		a.readiness = nil
		return
	}
	if a.designSvc == nil {
		a.readinessLoading = false
		a.readiness = nil
		a.readinessErr = "设计服务不可用，无法获取 Core 校验结果"
		return
	}
	a.readinessLoading = true
	assessment, err := a.designSvc.AssessAsterDoc(context.Background(), a.designDoc.DesignID, a.designDoc.Revision)
	a.readinessLoading = false
	if err != nil {
		a.readiness = nil
		a.readinessErr = state.MapError(err).Error()
		return
	}
	a.readiness = &assessment
}

func (a *app) renderPreflight(c *ui.Context) {
	if !a.preflightOpen || a.designDoc == nil {
		return
	}
	t := c.Theme()
	open := a.preflightOpen
	components.Dialog(c, &open, components.DialogOptions{Label: "创建运行前检查"}, func(panel ui.Element) {
		panel.Width(560).Children(func() {
			ui.Text(c, "创建运行前检查").FontSize(15).Bold()
			ui.Text(c, fmt.Sprintf("文档 rev %d · 校验由 Rust Core 生成", a.designDoc.Revision)).
				FontSize(11).TextColor(t.TextMuted)
			if a.readinessLoading {
				ui.Text(c, "校验中 · 正在等待 Core 校验结果…").FontSize(12).TextColor(t.TextMuted)
				ui.Row(c).FillWidth().Wrap().Justify(ui.End).Gap(8).Children(func() {
					if components.Button(c, "返回修改", components.ButtonSecondary).Clicked() {
						a.preflightOpen = false
					}
					create := components.Button(c, "创建 Run", components.ButtonPrimary)
					create.Disabled(true)
				})
				return
			}
			assessment := a.readiness
			if assessment == nil {
				ui.Text(c, "未解析 · 无法获取 Core 校验结果").FontSize(12).Bold().TextColor(t.Danger)
				if a.readinessErr != "" {
					ui.Box(c).FillWidth().Padding(6, 8).Radius(t.Space(1)).Background(t.Danger.Alpha(.06)).Children(func() {
						ui.Text(c, a.readinessErr).FontSize(12).TextColor(t.Danger)
					})
				}
				ui.Row(c).FillWidth().Wrap().Justify(ui.End).Gap(8).Children(func() {
					if components.Button(c, "返回修改", components.ButtonSecondary).Clicked() {
						a.preflightOpen = false
					}
					if components.Button(c, "重试", components.ButtonPrimary).Clicked() {
						a.refreshReadiness()
					}
				})
				return
			}
			if len(assessment.Errors) > 0 {
				ui.Text(c, "必须修复").FontSize(12).Bold().TextColor(t.Danger)
				for _, issue := range assessment.Errors {
					ui.Row(c).Padding(6, 8).Radius(t.Space(1)).Background(t.Danger.Alpha(.06)).Children(func() {
						ui.Text(c, issue).FontSize(12).TextColor(t.Danger)
					})
				}
			}
			if len(assessment.Warnings) > 0 {
				ui.Text(c, "可以确认的风险").FontSize(12).Bold().TextColor(t.Warning)
				for _, warning := range assessment.Warnings {
					ui.Text(c, warning).FontSize(12).TextColor(t.TextMuted)
				}
				components.Checkbox(c, &a.preflightRisk, "我已了解并确认这些风险")
			}
			canCreate := len(assessment.Errors) == 0 && (len(assessment.Warnings) == 0 || a.preflightRisk)
			ui.Row(c).FillWidth().Wrap().Justify(ui.End).Gap(8).Children(func() {
				if components.Button(c, "返回修改", components.ButtonSecondary).Clicked() {
					a.preflightOpen = false
				}
				create := components.Button(c, "创建 Run", components.ButtonPrimary)
				create.Disabled(!canCreate)
				if create.Clicked() && canCreate {
					a.preflightOpen = false
					a.createRunFromDesignDoc()
				}
			})
		})
	})
	if !open {
		a.preflightOpen = false
	}
}

func (a *app) renderNewDesignDialog(c *ui.Context) {
	if !a.newDesignOpen {
		return
	}
	open := a.newDesignOpen
	components.Dialog(c, &open, components.DialogOptions{Label: "新建设计"}, func(panel ui.Element) {
		panel.Width(520).Children(func() {
			ui.Text(c, "新建设计").FontSize(15).Bold()
			ui.Text(c, "从空白实验开始，或使用 CCK-8 骨架快速验证流程。").
				FontSize(12).TextColor(c.Theme().TextMuted)
			ui.Row(c).FillWidth().Gap(8).Children(func() {
				if components.Button(c, "空白实验", components.ButtonPrimary).Clicked() {
					a.newDesignOpen = false
					a.createAsterDocDesign(blankAsterDoc, "新实验")
				}
				if components.Button(c, "CCK-8 骨架", components.ButtonSecondary).Clicked() {
					a.newDesignOpen = false
					a.createAsterDocDesign(starterAsterDoc, "新实验")
				}
				if components.Button(c, "取消", components.ButtonGhost).Clicked() {
					a.newDesignOpen = false
				}
			})
		})
	})
	if !open {
		a.newDesignOpen = false
	}
}

func (a *app) renderAddModuleDialog(c *ui.Context, doc *asterdoc.Document) {
	if !a.addModuleOpen {
		return
	}
	open := a.addModuleOpen
	groups := []string{}
	assays := []string{}
	for _, module := range doc.Modules {
		if module.Kind == "group" {
			groups = append(groups, module.ID)
		}
		if module.Kind == "assay" {
			assays = append(assays, module.ID)
		}
	}
	components.Dialog(c, &open, components.DialogOptions{Label: "添加模块"}, func(panel ui.Element) {
		panel.Width(420).Children(func() {
			ui.Text(c, "添加模块").FontSize(15).Bold()
			ui.Row(c).Wrap().Gap(8).Children(func() {
				if components.Button(c, "分组", components.ButtonSecondary).Clicked() {
					id := fmt.Sprintf("group-%d", len(groups)+1)
					if a.applyDesignRaw(map[string]any{"type": "add_module", "module": map[string]any{
						"kind": "group", "id": id, "schema": 1, "name": id, "role": "treatment",
					}}) {
						a.addModuleOpen = false
					}
				}
				assayBtn := components.Button(c, "检测", components.ButtonSecondary)
				assayBtn.Disabled(len(groups) == 0)
				if assayBtn.Clicked() && len(groups) > 0 {
					id := fmt.Sprintf("assay-%d", len(assays)+1)
					if a.applyDesignRaw(map[string]any{"type": "add_module", "module": map[string]any{
						"kind": "assay", "id": id, "schema": 1, "title": "新检测",
						"group_refs": groups, "timepoints": []any{},
					}}) {
						a.addModuleOpen = false
					}
				}
				plateBtn := components.Button(c, "孔板", components.ButtonSecondary)
				plateBtn.Disabled(len(assays) == 0 || len(groups) == 0)
				if plateBtn.Clicked() && len(assays) > 0 && len(groups) > 0 {
					groupMap := map[string]any{}
					for _, group := range groups {
						groupMap[group] = []any{}
					}
					id := fmt.Sprintf("plate-%d", len(doc.Modules)+1)
					if a.applyDesignRaw(map[string]any{"type": "add_module", "module": map[string]any{
						"kind": "plate", "id": id, "schema": 1, "format": 96,
						"assay": assays[0], "groups": groupMap,
					}}) {
						a.addModuleOpen = false
					}
				}
			})
			if len(groups) == 0 {
				ui.Text(c, "先添加一个分组，才能添加检测。").
					FontSize(11).TextColor(c.Theme().TextMuted)
			}
		})
	})
	if !open {
		a.addModuleOpen = false
	}
}

func (a *app) renderImportDialog(c *ui.Context) {
	if !a.importOpen {
		return
	}
	open := a.importOpen
	components.Dialog(c, &open, components.DialogOptions{Label: "导入 AsterML"}, func(panel ui.Element) {
		panel.Width(560).Children(func() {
			ui.Text(c, "导入 AsterML 设计").FontSize(15).Bold()
			ui.Text(c, "选择 .asterml / .md 源文件；导入后仍由 Core 校验并生成文档版本。").
				FontSize(12).TextColor(c.Theme().TextMuted)
			ui.Text(c, "文件路径").FontSize(11).TextColor(c.Theme().TextMuted)
			ui.TextInput(c, &a.importPath).Placeholder("D:\\...\\experiment.asterml").FillWidth()
			ui.Text(c, "设计名称").FontSize(11).TextColor(c.Theme().TextMuted)
			ui.TextInput(c, &a.importTitle).Placeholder("例如 HUVEC 成管实验").FillWidth()
			if a.importErr != "" {
				ui.Text(c, a.importErr).FontSize(11).TextColor(c.Theme().Danger)
			}
			ui.Row(c).FillWidth().Wrap().Justify(ui.End).Gap(8).Children(func() {
				if components.Button(c, "取消", components.ButtonGhost).Clicked() {
					a.importOpen = false
				}
				if components.Button(c, "导入", components.ButtonPrimary).Clicked() {
					a.importAsterDocFile()
				}
			})
		})
	})
	if !open {
		a.importOpen = false
	}
}

func (a *app) importAsterDocFile() {
	if a.designSvc == nil || len(a.projects) == 0 {
		a.importErr = "设计服务或项目不可用"
		return
	}
	path := strings.TrimSpace(a.importPath)
	if path == "" {
		a.importErr = "请填写文件路径"
		return
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		a.importErr = "无法读取文件：" + err.Error()
		return
	}
	title := strings.TrimSpace(a.importTitle)
	if title == "" {
		title = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	}
	doc, err := a.designSvc.CreateAsterDocDesign(context.Background(), runs.AsterDocCreateInput{
		ProjectID: a.projects[0].ProjectID, Title: title, Source: string(raw),
		OperationID: a.nextOp(), ActorID: os.Getenv("USERNAME"),
	})
	if err != nil {
		a.importErr = state.MapError(err).Error()
		return
	}
	a.importOpen = false
	a.importErr = ""
	if a.shell != nil {
		a.shell.Designs = append([]shell.DesignItem{{
			ID: doc.DesignID, Title: title, Meta: fmt.Sprintf("rev %d", doc.Revision),
		}}, a.shell.Designs...)
		a.shell.Selected = doc.DesignID
		a.shell.Page = shell.Design
	}
	a.loadDesign(doc.DesignID)
	a.saveSession()
}
