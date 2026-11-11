package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

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
	"asterlab-mygo-prototype/modules/runs"

	mygo "github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/richtext"
	"github.com/oneo-me/onevra/source/components"
	"github.com/oneo-me/onevra/source/icons/lucide"
)

type sample struct{ id, value string }

type pdfExportResult struct {
	path     string
	err      error
	canceled bool
}

type app struct {
	page    *string
	step    *string
	samples []sample

	runSvc              runs.RunService
	designSvc           runs.DesignService
	snap                runs.RunSnapshot
	err                 string
	batchView           string
	asterDoc            *asterdoc.FrozenAsterDoc
	asterErr            string
	pdfExporting        bool
	pdfExportResults    chan pdfExportResult
	showFrozenDoc       bool
	designDoc           *runs.AsterDocSnapshot
	designView          *asterdoc.View
	assetRoots          map[string]string
	runAssets           []runs.Asset
	runViewKinds        map[string]runworkspace.ViewKind
	designDocErr        string
	projects            []runs.ProjectSummary
	protocols           []runs.ProtocolSummary
	readiness           *runs.AsterDocAssessment
	preflightOpen       bool
	preflightRisk       bool
	newDesignOpen       bool
	addModuleOpen       bool
	importOpen          bool
	importPath          string
	importTitle         string
	importErr           string
	designHistory       []runs.AsterDocSnapshot
	designFuture        []runs.AsterDocSnapshot
	asterDraftUndo      [][]byte
	asterDraftRedo      [][]byte
	asterDraftLast      []byte
	asterDraftSuspend   bool
	asterDraftSelection string
	designTab           int
	designCanvas        *workflow.Editor
	designCanvasSource  string
	designLayouts       map[string]map[string]workflow.Position
	designSelected      string
	canvasTitle         string
	canvasAssay         string
	canvasPath          string
	canvasTimepoints    string
	canvasSelLoaded     string
	canvasSyncing       bool
	confirmDeleteStep   string
	canvasTasks         []canvasTaskDraft
	canvasChecks        []canvasCheckDraft
	canvasUses          []canvasUseDraft
	canvasUsePick       string
	toolsTab            int
	noteDraft           string
	noteRunID           string
	noteDirty           bool
	noteState           string
	noteErr             string
	createRunErr        string
	noteMode            int
	noteEditMode        int
	noteDoc             *richtext.Document
	noteAster           *asterdoc.View

	shell               *shell.Shell
	ws                  *runui.Workspace
	runWS               *runworkspace.State
	runWSVM             *runworkspace.RunWorkspaceViewModel
	runWSKey            string
	designer            *designer.Designer
	pendingDesignID     string
	confirmDesignSwitch bool
	guard               state.StaleGuard

	designSnap *runs.DesignSnapshot

	wf     *workflow.Editor
	blocks []blocks.Instance

	opCounter atomic.Uint64

	selectedTask string
	tools        *tools.Workspace
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
		Title:        s.Title,
		Status:       s.Status,
		Revision:     s.Revision,
		Measurements: make([]runui.Measurement, 0, len(s.Measurements)),
		Events:       make([]runui.Event, 0, len(s.Events)),
	}
	for _, t := range s.Tasks {
		endpoint, unit := "", ""
		if pair, ok := endpoints[t.AssayID]; ok {
			endpoint, unit = pair[0], pair[1]
		}
		task := runui.Task{
			Key: t.Key, Title: t.Title, Timepoint: t.Timepoint, Status: t.Status,
			AssayID: t.AssayID, DesignStepID: t.DesignStepID,
			Endpoint: endpoint, Unit: unit,
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
	if !a.guard.Accept(snap.Revision) {
		a.err = "忽略过期快照"
		return
	}
	newSnap, err := state.ApplySnapshot(a.snap, snap)
	if err != nil {
		a.err = err.Error()
		return
	}
	if err := state.ConflictIfDifferent(a.snap, snap); err != nil {
		a.err = err.Error()
	}
	previousRunID := a.snap.RunID
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
		if len(a.snap.Tasks) == 0 && len(a.snap.Measurements) == 0 {
			a.ws.State = runui.StateEmpty
		} else {
			a.ws.State = runui.StateReady
		}
		a.prepareGrid()
		a.selectTaskFromEnv()
		if a.noteRunID != a.snap.RunID {
			a.loadNotebook()
		}
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

func (a *app) loadRunAssets() {
	if a.snap.RunID == "" || a.runSvc == nil {
		a.runAssets = nil
		return
	}
	linked, err := a.runSvc.ListAssetLinks(context.Background(), a.snap.RunID)
	if err != nil {
		a.err = state.MapError(err).Error()
		return
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

func (a *app) linkRunAsset(taskKey string) error {
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
		Parent: mygo.FocusedWindow(), Title: "关联实验文件", DefaultPath: rootDir,
	})
	if err != nil || len(paths) == 0 {
		return err
	}
	linked, err := assets.NewLink(rootID, rootDir, paths[0], assets.Link{
		RunID: a.snap.RunID, TaskKey: taskKey,
	}, "local-user", true)
	if err != nil {
		return err
	}
	saved, err := a.runSvc.SaveAssetLink(context.Background(), runs.AssetLinkInput{Asset: runAssetFromLocal(linked)})
	if err != nil {
		return state.MapError(err)
	}
	for i, old := range a.runAssets {
		if old.RootID == saved.RootID && old.RelativePath == saved.RelativePath &&
			old.Link.RunID == saved.Link.RunID && old.Link.TaskKey == saved.Link.TaskKey &&
			old.Link.SampleID == saved.Link.SampleID {
			a.runAssets[i] = saved
			return nil
		}
	}
	a.runAssets = append(a.runAssets, saved)
	return nil
}

// loadFrozenAsterDoc loads the exact document revision frozen on the Run. The
// latest live document is never substituted for a missing or broken revision.
func (a *app) refreshRunWorkspace() {
	vm := runworkspace.BuildViewModel(a.snap, a.asterDoc)
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
		err = asterdoc.ExportFrozenPDF(tmp, title, frozen, runID, plan)
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
		a.err = "设计服务不可用"
		return
	}
	if len(a.projects) == 0 {
		a.err = "没有可用项目；请先从 Core 导入项目"
		if a.shell != nil {
			a.shell.Error = a.err
		}
		return
	}
	project := a.projects[0]
	doc, err := a.designSvc.CreateAsterDocDesign(context.Background(), runs.AsterDocCreateInput{
		ProjectID: project.ProjectID, Title: title, Source: source,
		OperationID: a.nextOp(), ActorID: os.Getenv("USERNAME"),
	})
	if err != nil {
		a.err = state.MapError(err).Error()
		if a.shell != nil {
			a.shell.Error = a.err
		}
		return
	}
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
	if a.runSvc == nil || runID == "" {
		return
	}
	snap, err := a.runSvc.LoadRun(context.Background(), runID)
	if err != nil {
		a.err = state.MapError(err).Error()
		if a.shell != nil {
			a.shell.Error = a.err
		}
		return
	}
	a.applyRun(&snap, nil)
	if a.shell != nil {
		a.shell.Page = shell.Run
		a.shell.Selected = snap.DesignID
		a.saveSession()
	}
}

type workspaceSession struct {
	Project       string                                  `json:"project"`
	AssetRoots    map[string]string                       `json:"asset_roots,omitempty"`
	CanvasLayouts map[string]map[string]workflow.Position `json:"canvas_layouts,omitempty"`
	DesignID      string                                  `json:"design_id"`
	RunID         string                                  `json:"run_id"`
	TaskPath      string                                  `json:"task_path,omitempty"`
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
	if a.runWS != nil {
		collapsed := a.runWS.TreeCollapsed()
		if len(collapsed) > 0 {
			session.Collapsed = collapsed
		}
		if a.runWS.Selected != "" {
			for _, task := range a.snap.Tasks {
				if task.Key == a.runWS.Selected {
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
			if session.TaskPath != "" {
				for _, task := range a.snap.Tasks {
					if taskStepPath(task) == session.TaskPath {
						a.ws.Selected = task.Key
						a.runWS.Selected = task.Key
						a.prepareGrid()
						break
					}
				}
			}
		}
	}
	if session.Page == "design" && session.DesignID != "" {
		a.selectDesign(session.DesignID)
		a.shell.Page = shell.Design
		a.designTab = session.DesignTab
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
			a.ws.Selected = task.Key
			if a.runWS != nil {
				a.runWS.Selected = task.Key
			}
			a.prepareGrid()
			return
		}
	}
}

func taskStepPath(task runs.RunTask) string {
	if task.Path != "" {
		return task.Path
	}
	return task.Title
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

func (a *app) prepareGrid() {
	if a.ws == nil {
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
	a.ws.Grid = grid
}

func (a *app) loadDesign(id string) {
	if a.designSvc == nil {
		return
	}
	a.designer = designer.New(id, "", 0, a.designSvc)
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
	snap, err := a.designSvc.LoadDesign(context.Background(), id)
	if err != nil {
		a.err = state.MapError(err).Error()
		return
	}
	a.designSnap = &snap
	a.designDoc = nil
	a.designView = nil
	a.designDocErr = ""
	a.designHistory = nil
	a.designFuture = nil
	a.resetAsterDraftHistory()
	a.designCanvas = nil
	a.designCanvasSource = ""
	// Keep per-design layouts loaded from the session; selecting a design uses
	// its own saved coordinates. (no-op)
	a.designSelected = ""
	if doc, docErr := a.designSvc.LoadAsterDoc(context.Background(), id); docErr != nil {
		a.designDocErr = state.MapError(docErr).Error()
	} else if doc.Revision > 0 {
		a.designDoc = &doc
		a.refreshReadiness()
	}
	a.preflightOpen = false
	a.preflightRisk = false
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
		a.err = err.Error()
		a.shell.Error = a.err
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
	if a.designer != nil && id == a.designer.DesignID {
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
		a.err = err.Error()
		return
	}
	if snap == nil {
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
		a.runWSVM = runworkspace.BuildViewModel(a.snap, a.asterDoc)
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

func designPage(c *ui.Context, a *app) {
	if a.designDoc != nil {
		asterDesignPage(c, a)
		return
	}
	if a.designDocErr != "" {
		ui.Column(c).Fill().Center().Gap(c.Theme().Space(2)).Children(func() {
			ui.Text(c, "AsterDoc 设计无法读取").FontSize(16).Bold().TextColor(c.Theme().Danger)
			ui.Text(c, a.designDocErr).FontSize(13).TextColor(c.Theme().TextMuted)
		})
		return
	}
	if a.designer != nil {
		designer.View(c, a.designer)
		return
	}
	t := c.Theme()
	message := "请从项目页选择一个已保存的设计。"
	if a.designSvc == nil {
		message = "设计服务不可用。"
	}
	ui.Column(c).Fill().Center().Gap(t.Space(2)).Children(func() {
		components.Text(c, "还没有打开的设计").FontSize(t.Rem(1.15)).FontWeight(600)
		components.Text(c, message).TextColor(t.TextMuted)
	})
}

func protocolPage(c *ui.Context, a *app) {
	// A document-backed Run reads its exact frozen AsterDoc revision.
	if a.snap.RunID != "" && a.snap.AsterDoc != nil {
		ui.Column(c).Fill().Children(func() {
			ui.Row(c).FillWidth().Padding(c.Theme().Space(2), c.Theme().Space(4)).
				AlignItems(ui.Center).Children(func() {
				ui.Text(c, fmt.Sprintf("冻结文档 · rev %d", a.snap.AsterDoc.Revision)).
					FontSize(11).Bold().TextColor(c.Theme().TextMuted)
				ui.Spacer(c)
				a.documentExportButton(c)
			})
			if a.asterErr != "" {
				ui.Column(c).Grow(1).Padding(c.Theme().Space(8)).Gap(c.Theme().Space(2)).Children(func() {
					ui.Text(c, "冻结文档无法读取。").FontSize(16).Bold().TextColor(c.Theme().Danger)
					ui.Text(c, a.asterErr).FontSize(13).TextColor(c.Theme().TextMuted)
				})
				return
			}
			ui.Scroll(c).Fill().Children(func() {
				ui.Column(c).FillWidth().Padding(c.Theme().Space(6), c.Theme().Space(6)).
					AlignItems(ui.Center).Children(func() {
					ui.Box(c).FillWidth().MaxWidth(900).MaxWidthPercent(100).
						Radius(float32(18)).Border(1, c.Theme().Border).Background(c.Theme().Surface).
						Padding(c.Theme().Space(7), c.Theme().Space(8)).Children(func() {
						a.renderFrozenAsterDoc(c, asterdoc.ModeProtocol)
					})
				})
			})
		})
		return
	}
	// A loaded Run must read its frozen design. If that revision is missing or
	// corrupt the page reports it; showing the live draft would pass a later
	// edit off as the plan executed on the day.
	if a.snap.RunID != "" && a.snap.AsterDoc == nil {
		ui.Row(c).FillWidth().Padding(c.Theme().Space(2), c.Theme().Space(4)).Children(func() {
			ui.Text(c, fmt.Sprintf("Document Revision: %d", a.snap.DesignRevision)).
				FontSize(11).TextColor(c.Theme().TextMuted)
		})
	}
	if a.snap.RunID != "" {
		if a.snap.DesignData == "" {
			ui.Column(c).Grow(1).Padding(c.Theme().Space(8)).Gap(c.Theme().Space(2)).Children(func() {
				ui.Text(c, "已选择 Run，但它的冻结设计缺失。").FontSize(16).Bold().
					TextColor(c.Theme().Danger)
				ui.Text(c, "不会改用实时设计；请检查该运行的设计快照。").
					FontSize(13).TextColor(c.Theme().TextMuted)
			})
			return
		}
		doc, err := protocol.Build(a.snap.DesignData, a.snap.DesignID, a.snap.DesignRevision)
		if err != nil {
			ui.Text(c, "冻结设计无法生成实验方案："+err.Error()).TextColor(c.Theme().Danger)
			return
		}
		protocol.ViewDocument(c, doc)
		return
	}
	state := &protocol.State{DesignID: a.shell.Selected, Revision: a.snap.DesignRevision}
	if a.designer != nil {
		state.Document = a.designer.Editor.Doc
		state.Instances = a.designer.Instances
		state.Revision = a.designer.Revision
	}
	protocol.View(c, state)
}

// asterDesignPage is the document-backed Design surface. Professional editors
// issue semantic commands; this slice supports plate editing and Run creation.
func asterDesignPage(c *ui.Context, a *app) {
	t := c.Theme()
	c.OnShortcut(ui.Cmd, ui.KeyZ, func() { a.designUndo() })
	c.OnShortcut(ui.Cmd|ui.Shift, ui.KeyZ, func() { a.designRedo() })
	c.OnShortcut(ui.Cmd, ui.KeyY, func() { a.designRedo() })
	doc, err := asterdoc.Parse(a.designDoc.Source)
	if err != nil {
		ui.Text(c, "AsterDoc 语法错误："+err.Error()).Padding(t.Space(6)).TextColor(t.Danger)
		return
	}
	ui.Column(c).Fill().Children(func() {
		ui.Row(c).FillWidth().Padding(t.Space(2), t.Space(4)).Border(1, t.Border).
			Background(t.Surface).AlignItems(ui.Center).Gap(t.Space(2)).Children(func() {
			ui.Text(c, "AsterDoc 设计").FontSize(16).Bold().TextColor(t.Text)
			ui.Text(c, fmt.Sprintf("rev %d", a.designDoc.Revision)).FontSize(11).
				TextColor(t.TextMuted)
			if a.readiness != nil {
				label, color := "草稿", t.TextMuted
				switch a.readiness.State {
				case "incomplete":
					label = "待完善"
					color = t.Warning
				case "warning":
					label = "可创建（有警告）"
					color = t.Warning
				case "ready":
					label = "可创建运行"
					color = t.Success
				}
				ui.Text(c, label).FontSize(10).Bold().TextColor(color)
			}
			tab := a.designTab
			components.Tabs(c, &tab, components.TabsSegmented, "流程", "文档")
			a.designTab = tab
			ui.Spacer(c)
			undo := components.IconButton(c, lucide.Undo2(), "撤销", components.IconButtonCompact)
			undo.Disabled(len(a.designHistory) == 0 && len(a.asterDraftUndo) == 0)
			if undo.Clicked() {
				a.designUndo()
			}
			redo := components.IconButton(c, lucide.Redo2(), "重做", components.IconButtonCompact)
			redo.Disabled(len(a.designFuture) == 0 && len(a.asterDraftRedo) == 0)
			if redo.Clicked() {
				a.designRedo()
			}
			if a.createRunErr != "" {
				ui.Text(c, a.createRunErr).FontSize(11).TextColor(t.Danger).
					SingleLine().Ellipsis("…").MaxWidth(360)
			}
			if components.Button(c, "创建 Run", components.ButtonPrimary).Clicked() {
				a.preflightRisk = false
				a.preflightOpen = true
			}
		})
		if a.designDocErr != "" {
			ui.Row(c).FillWidth().Padding(6, 12).Background(theme.GetTokens(t).DangerBg).Children(func() {
				ui.Text(c, a.designDocErr).FontSize(11).TextColor(theme.GetTokens(t).DangerText)
			})
		}
		if a.designTab == 0 {
			a.renderWorkflowCanvas(c)
			a.renderPreflight(c)
			return
		}
		ui.Scroll(c).Grow(1).Children(func() {
			ui.Column(c).FillWidth().Padding(t.Space(4), t.Space(5)).
				AlignItems(ui.Center).Children(func() {
				ui.Box(c).FillWidth().MaxWidth(980).MaxWidthPercent(100).
					Background(t.Surface).Border(1, t.Border).Radius(t.Space(2)).
					Padding(t.Space(5), t.Space(6)).Children(func() {
					if a.designView == nil || a.designView.Doc == nil || a.designView.Doc.Source != a.designDoc.Source {
						a.designView = &asterdoc.View{
							Doc: doc, Mode: asterdoc.ModeDesign, DocumentRevision: fmt.Sprint(a.designDoc.Revision),
							Allowed: map[string]bool{asterdoc.PermDocumentEdit: true},
							OnAssignWells: func(moduleID, group string, wells []string) {
								a.applyDesignCommand("assign_wells", moduleID, group, wells)
							},
							OnClearWells: func(moduleID string, wells []string) {
								a.applyDesignCommand("clear_wells", moduleID, "", wells)
							},
							OnUpdateModule: func(moduleID string, props map[string]any) {
								_ = a.applyDesignRaw(map[string]any{
									"type": "update_module", "moduleId": moduleID, "properties": props,
								})
							},
						}
					}
					a.designView.Render(c)
				})
			})
		})
	})
	a.renderPreflight(c)
	a.captureAsterDraft()
}

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
	if a.designSvc == nil || a.designDoc == nil {
		return false
	}
	previous := *a.designDoc
	raw, err := json.Marshal(command)
	if err != nil {
		a.designDocErr = err.Error()
		return false
	}
	updated, err := a.designSvc.ApplyAsterDocCommand(context.Background(), runs.AsterDocCommandInput{
		DesignID: a.designDoc.DesignID, ExpectedRevision: a.designDoc.Revision,
		OperationID: a.nextOp(), ActorID: os.Getenv("USERNAME"), Command: string(raw),
	})
	if err != nil {
		a.designDocErr = state.MapError(err).Error()
		return false
	}
	a.designDocErr = ""
	a.designHistory = append(a.designHistory, previous)
	a.designFuture = nil
	a.resetAsterDraftHistory()
	a.designDoc = &updated
	a.designView = nil
	a.designCanvas = nil
	a.designCanvasSource = ""
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
		return
	}
	a.designHistory = a.designHistory[:len(a.designHistory)-1]
	a.designFuture = append(a.designFuture, current)
	a.designDocErr = ""
	a.designDoc = &updated
	a.designView = nil
	a.designCanvas = nil
	a.designCanvasSource = ""
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
		return
	}
	a.designFuture = a.designFuture[:len(a.designFuture)-1]
	a.designHistory = append(a.designHistory, current)
	a.designDocErr = ""
	a.designDoc = &updated
	a.designView = nil
	a.designCanvas = nil
	a.designCanvasSource = ""
	a.refreshReadiness()
}

func (a *app) createRunFromDesignDoc() {
	if a.designSvc == nil || a.designDoc == nil {
		return
	}
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

func toolsPage(c *ui.Context, a *app) {
	t := c.Theme()
	ui.Column(c).Fill().Children(func() {
		ui.Row(c).FillWidth().Padding(t.Space(2), t.Space(4)).Border(1, t.Border).
			Background(t.Surface).Children(func() {
			components.Tabs(c, &a.toolsTab, components.TabsUnderline, "工具", "Protocol")
			ui.Spacer(c)
			ui.Text(c, "方法与参考材料").FontSize(11).TextColor(t.TextMuted)
		})
		if a.toolsTab == 1 {
			protocolPage(c, a)
			return
		}
		if a.tools == nil {
			a.tools = tools.New(tools.Callbacks{})
		}
		tools.View(c, a.tools)
	})
}

// notebookPage follows Obsidian's shape: one paper sheet, a floating toolbar,
// and three modes over the same stored Markdown — live rich rendering, source
// editing, and reading. Each save appends one notebook-stage event.
func notebookPage(c *ui.Context, a *app) {
	t := c.Theme()
	if a.snap.RunID == "" {
		ui.Column(c).Fill().Center().Gap(t.Space(2)).Children(func() {
			ui.Text(c, "Notebook").FontSize(20).Bold().TextColor(t.Text)
			ui.Text(c, "先选择或加载一个 Run，笔记会记录在它的执行历史中。").
				FontSize(13).TextColor(t.TextMuted)
		})
		return
	}
	if a.noteRunID != a.snap.RunID {
		a.loadNotebook()
	}
	ui.Column(c).Fill().Padding(t.Space(2), t.Space(3)).Gap(t.Space(2)).Children(func() {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Padding(0, t.Space(1)).Children(func() {
			ui.Text(c, "Notebook").FontSize(11).Bold().TextColor(t.TextMuted)
			ui.Text(c, "/").FontSize(11).TextColor(t.Border)
			ui.Text(c, a.snap.Title).FontSize(11).TextColor(t.TextMuted).
				SingleLine().Ellipsis("…").MaxWidth(420)
			if a.snap.AsterDoc == nil && a.snap.DesignRevision > 0 {
				ui.Text(c, fmt.Sprintf("Document Revision: %d", a.snap.DesignRevision)).
					FontSize(10).TextColor(t.TextMuted)
			}
			ui.Spacer(c)
			a.documentExportButton(c)
			ui.Text(c, fmt.Sprintf("%d / 5000", len([]rune(a.noteSource())))).
				FontSize(10).TextColor(t.TextMuted).FontFeatures("tnum")
		})
		// Floating toolbar above the sheet, following Obsidian's editing
		// toolbar: mode switch, editing commands, then one save action.
		if c.Shortcut(ui.Cmd, ui.KeyE) {
			if a.noteMode == 2 {
				a.setNoteMode(a.noteEditMode)
			} else {
				a.setNoteMode(2)
			}
		}
		ui.Row(c).FillWidth().Justify(ui.Center).Children(func() {
			ui.Row(c).Padding(t.Space(1), t.Space(1.25)).Radius(t.Space(2.5)).
				Background(t.Surface).Border(1, t.Border).
				Shadow(0, 2, 16, 0, t.Text.Alpha(.10)).Gap(t.Space(.75)).
				AlignItems(ui.Center).Children(func() {
				live := a.noteMode == 0
				doc := a.noteDoc
				if live && doc != nil {
					formats := doc.Formatting()
					a.iconCmd(c, lucide.Undo2(), "撤销", false, doc.CanUndo(), doc.Undo)
					a.iconCmd(c, lucide.Redo2(), "重做", false, doc.CanRedo(), doc.Redo)
					ui.Box(c).Width(1).Height(18).Background(t.Border)
					a.iconCmd(c, lucide.Bold(), "粗体", formats.Bold == richtext.FormatOn, true, doc.ToggleBold)
					a.iconCmd(c, lucide.Italic(), "斜体", formats.Italic == richtext.FormatOn, true, doc.ToggleItalic)
					a.iconCmd(c, lucide.Strikethrough(), "删除线", formats.Strikethrough == richtext.FormatOn, true, doc.ToggleStrikethrough)
					a.iconCmd(c, lucide.Code(), "行内代码", formats.Code == richtext.FormatOn, true, doc.ToggleCode)
					ui.Box(c).Width(1).Height(18).Background(t.Border)
					a.iconCmd(c, lucide.Heading1(), "一级标题", formats.Paragraph.Heading == 1, true, func() { doc.SetHeading(1) })
					a.iconCmd(c, lucide.Heading2(), "二级标题", formats.Paragraph.Heading == 2, true, func() { doc.SetHeading(2) })
					a.iconCmd(c, lucide.List(), "项目列表", formats.Paragraph.List == richtext.Bullets, true, func() { doc.ToggleList(richtext.Bullets) })
					a.iconCmd(c, lucide.ListOrdered(), "编号列表", formats.Paragraph.List == richtext.Numbers, true, func() { doc.ToggleList(richtext.Numbers) })
					a.iconCmd(c, lucide.Quote(), "引用", formats.Paragraph.Quote, true, doc.ToggleQuote)
				} else {
					a.iconCmd(c, lucide.Bold(), "实时渲染中可格式化", false, false, func() {})
				}
				if a.noteMode == 3 {
					add := components.IconButton(c, lucide.Plus(), "插入孔板模块", components.IconButtonCompact)
					if add.Clicked() {
						a.insertPlateModule()
					}
				}
				ui.Spacer(c)
				old := a.noteMode
				parts := components.TabsWithLeading(c, &a.noteMode, components.TabsSegmented,
					func(c *ui.Context, i int, size float32) {
						icon := []*ui.SVG{lucide.Sparkles(), lucide.PenLine(), lucide.BookOpen(), lucide.Blocks(), lucide.ListClock()}[i]
						components.Icon(c, icon).Size(size, size)
					}, "实时", "编辑", "阅读", "文档", "时间线")
				if parts.List.Changed() {
					a.applyNoteModeChange(old)
				}
				if a.noteDirty {
					ui.Box(c).Size(6, 6).Radius(3).Background(t.Warning)
				}
				save := components.IconButton(c, lucide.Save(), "保存笔记", components.IconButtonCompact)
				save.Disabled(a.noteMode == 2 || a.noteMode == 4)
				if save.Clicked() && a.noteMode != 2 && a.noteMode != 4 {
					a.saveNotebook()
				}
			})
		})
		if a.noteErr != "" {
			ui.Row(c).FillWidth().Padding(6, 10).Radius(6).
				Background(theme.GetTokens(t).DangerBg).Children(func() {
				ui.Text(c, a.noteErr).FontSize(12).TextColor(theme.GetTokens(t).DangerText)
			})
		}
		paper := func(children func()) {
			ui.Column(c).Grow(1).FillWidth().AlignItems(ui.Center).Children(func() {
				ui.Box(c).Grow(1).FillWidth().MaxWidth(820).MaxWidthPercent(100).
					Background(t.Surface).Border(1, t.Border).
					Radius(t.Space(2.5)).Shadow(0, 1, 3, 0, t.Text.Alpha(.04)).
					Padding(t.Space(5), t.Space(7)).Children(children)
			})
		}
		switch a.noteMode {
		case 4: // frozen Run timeline, read-only
			a.renderNotebookTimeline(c)
		case 3: // AsterML document
			paper(func() {
				if a.noteAster == nil || a.noteAster.Doc == nil || a.noteAster.Doc.Source != a.noteDraft {
					doc, err := asterdoc.Parse(a.noteDraft)
					if err != nil {
						ui.Text(c, "文档语法错误："+err.Error()).FontSize(t.Rem(.85)).TextColor(t.Danger)
						return
					}
					a.noteAster = &asterdoc.View{Doc: doc, Mode: asterdoc.ModeNotebook,
						Allowed: map[string]bool{asterdoc.PermDocumentEdit: true},
						OnChange: func(source string) {
							a.noteDraft = source
							a.noteDirty = true
							a.noteState = ""
						}}
				}
				a.noteAster.Render(c)
			})
		case 2: // reading
			ui.Scroll(c).Grow(1).Children(func() {
				ui.Column(c).FillWidth().Padding(0, t.Space(2), t.Space(4), t.Space(2)).
					AlignItems(ui.Center).Children(func() {
					ui.Box(c).FillWidth().MaxWidth(820).MaxWidthPercent(100).
						Background(t.Surface).Border(1, t.Border).Radius(t.Space(2.5)).
						Padding(t.Space(7), t.Space(8)).Children(func() {
						if strings.TrimSpace(a.noteDraft) == "" {
							ui.Text(c, "还没有内容。切换到“编辑”开始写。").
								FontSize(13).TextColor(t.TextMuted)
							return
						}
						protocol.RenderMarkdown(c, a.noteDraft)
					})
				})
			})
		case 1: // source
			paper(func() {
				area := ui.TextAreaBase(c, &a.noteDraft)
				area.Fill().Grow(1).MinWidth(0).MinHeight(0)
				if area.Changed() {
					a.noteDirty = true
					a.noteState = ""
				}
			})
		default: // live rich rendering
			paper(func() {
				if a.noteDoc == nil {
					a.noteDoc = richtext.NewDocument(richtext.ParseMarkdown(a.noteDraft))
				}
				editor := richtext.EditorBase(c, a.noteDoc, richtext.EditorOptions{
					Label: "Notebook", Placeholder: "开始写实验笔记…",
				})
				editor.Fill().Grow(1).MinWidth(0).MinHeight(0)
				if editor.Changed() {
					a.noteDirty = true
					a.noteState = ""
				}
			})
		}
	})
}

// renderNotebookTimeline is a read-only view over the authoritative Run
// events. It never copies events into the editable Markdown draft.
func (a *app) renderNotebookTimeline(c *ui.Context) {
	t := c.Theme()
	ui.Scroll(c).Grow(1).Children(func() {
		ui.Column(c).FillWidth().Padding(t.Space(3), t.Space(5)).Gap(t.Space(2)).
			AlignItems(ui.Center).Children(func() {
			if a.snap.AsterDoc != nil {
				ui.Box(c).FillWidth().MaxWidth(980).MaxWidthPercent(100).
					Background(t.Surface).Border(1, t.Border).Radius(t.Space(2)).
					Padding(t.Space(4), t.Space(5)).Children(func() {
					ui.Text(c, fmt.Sprintf("冻结 AsterDoc · Revision %d", a.snap.AsterDoc.Revision)).
						FontSize(t.Rem(1)).Bold().TextColor(t.Text)
					a.renderFrozenAsterDoc(c, asterdoc.ModeNotebook)
				})
			}
			ui.Box(c).FillWidth().MaxWidth(980).MaxWidthPercent(100).
				Background(t.Surface).Border(1, t.Border).Radius(t.Space(2)).
				Padding(t.Space(4), t.Space(5)).Children(func() {
				ui.Text(c, "运行时间线 · 只读").FontSize(t.Rem(1.15)).Bold().TextColor(t.Text)
				ui.Text(c, "读取 Core Event Store；这里不重新保存事件事实。").
					FontSize(t.Rem(.75)).TextColor(t.TextMuted)
				if len(a.snap.Events) == 0 {
					ui.Text(c, "本次运行还没有事件。").FontSize(t.Rem(.85)).TextColor(t.TextMuted)
					return
				}
				for _, event := range a.snap.Events {
					event := event
					ui.Column(c).FillWidth().Gap(t.Space(.5)).Padding(t.Space(2)).
						Border(1, t.Border).Radius(t.Space(1)).Children(func() {
						ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(t.Space(1)).Children(func() {
							ui.Text(c, event.EventType).FontSize(t.Rem(.78)).Bold().TextColor(t.Text)
							ui.Text(c, event.Content).FontSize(t.Rem(.78)).TextColor(t.TextMuted)
							ui.Spacer(c)
							if event.BatchID != "" {
								if components.Button(c, "打开原始孔板", components.ButtonGhost).Clicked() {
									a.batchView = event.BatchID
									if a.shell != nil {
										a.shell.Page = shell.Run
									}
								}
							}
						})
						label := func(value string) string {
							if strings.TrimSpace(value) == "" {
								return "未记录"
							}
							return value
						}
						ui.Text(c, fmt.Sprintf("event %s · batch %s · task %s · %s",
							event.ID, label(event.BatchID), label(event.TaskKey), label(event.StepStatus))).
							FontSize(t.Rem(.68)).TextColor(t.TextMuted)
						if len(event.Checks) > 0 || len(event.ResultCells) > 0 {
							ui.Text(c, fmt.Sprintf("checks %d · cells %d", len(event.Checks), len(event.ResultCells))).
								FontSize(t.Rem(.68)).TextColor(t.TextMuted)
						}
					})
				}
			})
		})
	})
}

// applyNoteModeChange finishes a switch the Tabs control already made.
func (a *app) applyNoteModeChange(old int) {
	if old == a.noteMode {
		return
	}
	if old == 0 {
		a.noteDraft = a.noteSource()
	}
	if a.noteMode == 0 {
		a.noteDoc = richtext.NewDocument(richtext.ParseMarkdown(a.noteDraft))
	}
	if a.noteMode != 2 {
		a.noteEditMode = a.noteMode
	}
}

func (a *app) insertPlateModule() {
	a.noteAster = nil
	stamp := time.Now().UnixNano()
	groupID := fmt.Sprintf("group-%d", stamp)
	block := fmt.Sprintf("\n```asterlab\nkind: group\nid: %s\nschema: 1\nname: Control\nrole: control\n```\n\n```asterlab\nkind: plate\nid: plate-%d\nschema: 1\nformat: 96\ngroups:\n  %s: [A1]\n```\n", groupID, stamp, groupID)
	if strings.TrimSpace(a.noteDraft) != "" {
		a.noteDraft = strings.TrimRight(a.noteDraft, "\n") + "\n" + block
	} else {
		a.noteDraft = strings.TrimLeft(block, "\n")
	}
	a.noteDirty = true
	a.noteState = ""
}

func (a *app) setNoteMode(mode int) {
	old := a.noteMode
	if old == mode {
		return
	}
	if old == 0 {
		a.noteDraft = a.noteSource()
	}
	if mode == 0 {
		a.noteDoc = richtext.NewDocument(richtext.ParseMarkdown(a.noteDraft))
	}
	if mode != 2 {
		a.noteEditMode = mode
	}
	a.noteMode = mode
}

// iconCmd renders one compact toolbar command. Formatting commands act on the
// live rich document and mark the note dirty.
func (a *app) iconCmd(c *ui.Context, icon *ui.SVG, label string, active, enabled bool, act func()) {
	t := c.Theme()
	b := components.IconButton(c, icon, label, components.IconButtonCompact)
	if active {
		b.Background(t.Accent.Alpha(.14))
		b.TextColor(t.Accent)
	}
	b.Disabled(!enabled)
	if b.Clicked() && enabled {
		act()
		a.noteDirty = true
		a.noteState = ""
	}
}

// noteSource returns the canonical Markdown for the current mode.
func (a *app) noteSource() string {
	if a.noteMode == 0 && a.noteDoc != nil {
		return a.noteDoc.Text().Markdown()
	}
	return a.noteDraft
}

func (a *app) loadNotebook() {
	a.noteRunID = a.snap.RunID
	a.noteDraft = ""
	a.noteDirty = false
	a.noteState = ""
	a.noteErr = ""
	var latest *runs.RunEvent
	for i := range a.snap.Events {
		event := &a.snap.Events[i]
		if event.Stage != "notebook" {
			continue
		}
		if latest == nil || event.RecordedAt >= latest.RecordedAt {
			latest = event
		}
	}
	if latest != nil {
		a.noteDraft = latest.Content
	}
	a.noteMode = 0
	if os.Getenv("ASTERLAB_NOTE_MODE") == "doc" {
		a.noteMode = 3
	}
	a.noteEditMode = 0
	a.noteDoc = richtext.NewDocument(richtext.ParseMarkdown(a.noteDraft))
}

func (a *app) saveNotebook() {
	source := a.noteSource()
	a.noteDraft = source
	if a.runSvc == nil {
		a.noteErr = "运行服务不可用"
		a.noteState = "error"
		return
	}
	if strings.TrimSpace(source) == "" {
		a.noteErr = "Notebook 不能为空"
		a.noteState = "error"
		return
	}
	if len([]rune(source)) > 5000 {
		a.noteErr = "Notebook 超过 5000 个字符；请拆分。"
		a.noteState = "error"
		return
	}
	snap, err := a.runSvc.SaveNotebook(context.Background(), runs.NotebookInput{
		RunID: a.snap.RunID, Markdown: source, OperationID: a.nextOp(),
	})
	if err != nil {
		a.noteErr = state.MapError(err).Error()
		a.noteState = "error"
		return
	}
	a.noteErr = ""
	a.noteState = "saved"
	a.noteDirty = false
	a.applySnap(snap)
}

func main() {
	if strings.EqualFold(os.Getenv("ASTERLAB_THEME"), "dark") {
		mygo.Theme.SetSource(mygo.ThemeDark)
	}
	page, step := "project", "design-l929"
	_ = step

	runSvc, runErr := runs.NewRunService()
	designSvc, designErr := runs.NewDesignService()

	a := &app{
		page:       &page,
		step:       &step,
		runSvc:     runSvc,
		designSvc:  designSvc,
		snap:       runs.RunSnapshot{Title: "加载中", Status: "loading"},
		assetRoots: map[string]string{}, runViewKinds: map[string]runworkspace.ViewKind{},
	}
	if designErr != nil {
		a.err = "设计服务：" + designErr.Error()
	} else if runErr != nil {
		a.err = "运行服务：" + runErr.Error()
	} else if designSvc == nil {
		a.err = "设计服务返回空实现"
	}
	log.Printf("native services: run=%v(%v) design=%v(%v)", runSvc != nil, runErr, designSvc != nil, designErr)
	a.shell = &shell.Shell{
		Project:     "",
		Page:        shell.Project,
		OnSelect:    func(id string) { a.selectDesign(id) },
		OnRunSelect: func(id string) { a.selectRun(id) },
		OnCreate:    func() { a.newDesignOpen = true },
		OnImport:    func() { a.importOpen = true },
		OnPage:      func(p shell.Page) { a.shell.Page = p; a.saveSession() },
		Content:     func(c *ui.Context) { contentPage(c, a) },
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
				a.err = state.MapError(err).Error()
				return
			}
			a.applySnap(snap)
			if a.ws != nil {
				a.ws.State = runui.StateSaved
				if a.ws.Grid != nil {
					a.ws.Grid.State = resultgrid.StateSaved
				}
			}
		},
		SaveBatch: func(schema resultgrid.Schema, cells []resultgrid.Cell) {
			if a.ws == nil || a.runSvc == nil {
				return
			}
			taskKey := a.ws.Selected
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
			batchResult, err := a.runSvc.SaveReadoutsBatch(context.Background(), runs.BatchReadoutInput{
				RunID: a.snap.RunID, OperationID: a.nextOp(), TaskKey: task.Key,
				AssayID: task.AssayID, Timepoint: task.Timepoint,
				ProtocolStepPath: taskStepPath(task), ProtocolVersionID: task.ProtocolVersionID,
				SchemaID: schema.ID, SchemaVersion: schema.Version, Cells: batchCells,
				ExpectedRevision: a.snap.Revision,
			})
			if err != nil {
				message := state.MapError(err).Error()
				a.ws.State = runui.StateError
				a.ws.Error, a.ws.Grid.Error = message, message
				a.ws.Grid.State = resultgrid.StateError
				a.err = message
				return
			}
			if batchResult.Accepted != len(batchCells) {
				message := fmt.Sprintf("批量保存未接受全部单元格：%d/%d", batchResult.Accepted, len(batchCells))
				a.ws.State = runui.StateError
				a.ws.Error, a.ws.Grid.Error = message, message
				a.ws.Grid.State = resultgrid.StateError
				a.err = message
				return
			}
			snap, err := a.runSvc.LoadRun(context.Background(), a.snap.RunID)
			if err != nil {
				message := state.MapError(err).Error()
				a.ws.State = runui.StateError
				a.ws.Error, a.ws.Grid.Error = message, message
				a.ws.Grid.State = resultgrid.StateError
				a.err = message
				return
			}
			if batchResult.Revision > a.snap.Revision {
				a.snap.Revision = batchResult.Revision
			}
			savedGrid := a.ws.Grid
			savedGrid.State = resultgrid.StateSaved
			a.applySnap(snap)
			a.ws.State = runui.StateSaved
			a.ws.Grid = savedGrid
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
				a.err = a.ws.Error
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
				a.err = state.MapError(err).Error()
				return
			}
			a.applySnap(snap)
			if a.ws != nil {
				a.ws.State = runui.StateReady
			}
		},
		SelectTask: func(taskKey string) {
			a.selectedTask = taskKey
			if a.ws != nil {
				a.ws.Selected = taskKey
				a.prepareGrid()
			}
		},
		OpenBatch: func(batchID string) {
			a.batchView = batchID
		},
	}

	// runworkspace callbacks reuse the same Core seams as legacy runui.
	rwcb := runworkspace.Callbacks{
		SelectTask: func(taskKey string) {
			a.selectedTask = taskKey
			if a.ws != nil {
				a.ws.Selected = taskKey
				a.prepareGrid()
			}
			if a.runWS != nil {
				a.runWS.Selected = taskKey
			}
			a.saveSession()
		},
		ToggleTree: func() {
			a.saveSession()
		},
		SaveChecks: func(taskKey string, checks []runworkspace.CheckAckView) {
			if a.runWS == nil {
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
				if a.runWS != nil {
					a.runWS.RecordingDirty = false
				}
				a.err = state.MapError(err).Error()
				return
			}
			a.applySnap(snap)
		},
		Complete: func(taskKey string, checks []runworkspace.CheckAckView) {
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
				a.err = state.MapError(err).Error()
				return
			}
			a.applySnap(snap)
		},
		OpenBatch: func(batchID string) {
			a.batchView = batchID
		},
		Recording: func(c *ui.Context, task *runworkspace.RunTaskViewModel) {
			grid := (*resultgrid.Grid)(nil)
			if a.ws != nil && a.ws.Selected == task.TaskKey {
				grid = a.ws.Grid
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
			rv := runworkspace.RecordingView{
				Kind: kind, Grid: grid, Assets: linkedAssets, Task: task,
				OnKindChange: func(kind runworkspace.ViewKind) {
					a.runViewKinds[kindKey] = kind
				},
				OnOpenAsset: func(asset runworkspace.LinkedAsset) {
					if err := a.openRunAsset(asset); err != nil {
						c.AddToast(ui.Toast{Title: "无法打开文件", Description: err.Error(), Type: components.ToastError})
					}
				},
				OnRelocate: func(asset runworkspace.LinkedAsset) {
					if err := a.relocateRunAsset(asset); err != nil {
						c.AddToast(ui.Toast{Title: "文件重新定位失败", Description: err.Error(), Type: components.ToastError})
					} else if a.shell != nil {
						a.shell.Invalidate()
					}
				},
				OnLinkFile: func(_ runworkspace.LinkedAsset) {
					if err := a.linkRunAsset(task.TaskKey); err != nil {
						c.AddToast(ui.Toast{Title: "文件关联失败", Description: err.Error(), Type: components.ToastError})
					} else if a.shell != nil {
						a.shell.Invalidate()
					}
				},
				OnSaveBatch: func(cells []resultgrid.Cell) {
					if grid != nil {
						cb.SaveBatch(grid.Schema, cells)
					}
				},
			}
			rv.Render(c)
		},
	}
	a.tools = tools.New(tools.Callbacks{})
	a.ws = runui.NewWorkspace(toRunView(a.snap), cb)
	a.runWSVM = runworkspace.BuildViewModel(a.snap, a.asterDoc)
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
		a.shell.Quit()
		a.preload()
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}

func contentPage(c *ui.Context, a *app) {
	a.pollPDFExport(c)
	switch a.shell.Page {
	case shell.Design:
		designPage(c, a)
	case shell.Run:
		ui.Column(c).Fill().Children(func() {
			a.documentToolbar(c)
			if a.showFrozenDoc && a.snap.AsterDoc != nil {
				ui.Scroll(c).Height(260).Children(func() {
					ui.Box(c).FillWidth().Background(c.Theme().Surface).
						Padding(c.Theme().Space(3), c.Theme().Space(4)).Children(func() {
						a.renderFrozenAsterDoc(c, asterdoc.ModeRun)
					})
				})
			}
			if a.batchView != "" {
				batchPage(c, a)
			} else if a.runWS != nil {
				runworkspace.View(c, a.runWS)
			} else if a.ws != nil {
				a.ws.Width = 1200
				runui.View(c, a.ws)
			} else {
				ui.Text(c, "加载中…")
			}
		})
	case shell.Notebook:
		notebookPage(c, a)
	case shell.Tools:
		toolsPage(c, a)
	}
	a.renderNewDesignDialog(c)
	a.renderImportDialog(c)
	a.renderDesignSwitchConfirmation(c)
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
	if a.designSvc == nil || a.designDoc == nil {
		a.readiness = nil
		return
	}
	assessment, err := a.designSvc.AssessAsterDoc(context.Background(), a.designDoc.DesignID, a.designDoc.Revision)
	if err != nil {
		a.readiness = nil
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
			assessment := a.readiness
			if assessment == nil {
				ui.Text(c, "无法获取 Core 校验结果").FontSize(12).TextColor(t.Danger)
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
