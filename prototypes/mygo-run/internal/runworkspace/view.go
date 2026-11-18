package runworkspace

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"asterlab-mygo-prototype/internal/asterdoc"
	"asterlab-mygo-prototype/internal/notebook/record"
	"asterlab-mygo-prototype/internal/theme"
	workflowtree "asterlab-mygo-prototype/internal/workflowtree"
	"asterlab-mygo-prototype/modules/runs"

	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/onevra/source/components"
	"github.com/oneo-me/onevra/source/icons/lucide"
)

// Callbacks are the write seams. Every write goes through Core; the UI never
// mutates a record itself.
type Callbacks struct {
	SelectTask func(taskKey string)
	// ToggleTree persists expansion changes after a parent row is clicked.
	ToggleTree func()
	SaveChecks func(taskKey string, checks []CheckAckView) error
	Complete   func(taskKey string, checks []CheckAckView) error
	OpenBatch  func(batchID string)
	RetryLoad  func()
	// Recording renders the recording module for a task. The host supplies it so
	// the same module serves Design, Run, Notebook and PDF.
	Recording func(c *ui.Context, task *RunTaskViewModel)
	// LoadTimers lists the run's persisted timers; the bench renders them
	// from these checkpoints only.
	LoadTimers func(runID string) ([]runs.TimerRecord, error)
	// SaveTimer persists one timer row for a task.
	SaveTimer func(timer runs.TimerRecord) (runs.TimerRecord, error)
	// TimerAction applies one lifecycle action at the caller's clock.
	TimerAction func(input runs.TimerActionInput) (runs.TimerUpdateResult, error)
	// TimerPhases supplies the relay plan an advance walks. Nil means the
	// timer's own single phase.
	TimerPhases func(taskKey string) []runs.TimerPhase
	// AppendEvent appends one execution event (skipped_by_rule, deviation,
	// observation, completed round) through CreateExecutionEvent. It returns
	// Core's pantry advisory for the composer to display ("" when none).
	AppendEvent func(taskKey string, input runs.ExecutionEventInput) (string, error)
	// LoadPantry lists the pantry ledger for the optional event reference.
	LoadPantry func() (runs.PantryLedger, error)
	// TaskAssets returns the task's linked assets for the capture strip.
	TaskAssets func(taskKey string) []LinkedAsset
	// CaptureWells returns the planned wells in fill order plus the frozen
	// per-well FOV count (0 when the plan declares none).
	CaptureWells func(taskKey string) (wells []string, fovPerWell int)
	// RequestBatchLink opens the host file picker and links the chosen files
	// across capture slots for the task.
	RequestBatchLink func(taskKey string)
	// ResolveAssetPath resolves one LinkedAsset's RootID + RelativePath to a local file path for thumbnail preview.
	ResolveAssetPath func(asset LinkedAsset) string
	// RequestStepAssetLink opens single-step / spot file picker when an attachment is not a multi-FOV plate grid.
	RequestStepAssetLink func(taskKey, wellID string)
	// OpenAsset opens an asset externally through the host OS launcher.
	OpenAsset func(taskKey string, asset LinkedAsset)
	// AsyncDecode enables non-blocking background bitmap decode for thumbnails when a UI window is running.
	AsyncDecode       bool
	OnAsyncInvalidate func()
	// HistoryInputs supplies the stored snapshot plus already-linked assets
	// for the shared record projection. It is a cheap getter the bench may
	// call every render; the projection is memoized by Run revision plus
	// every asset's identity and coordinates (link writes never bump the
	// Run revision). Nil hides the summary/matrix and keeps the
	// entry-level timeline only.
	// RecordKey is the host's opaque memo key for the Core record
	// projection (run, revision, asset coordinates, FOV plan, view).
	// History never parses it.
	// FetchRecord returns the host's memoized Core record projection plus
	// the opaque key it was memoized under. Nil hides the summary/matrix
	// and keeps the entry-level timeline only.
	FetchRecord func() (RecordKey, record.Projection, error)
	// QueryProjection reads Core through the Bridge v2 generic channel
	// (INT-01). The blind switch uses kind run.workspace with
	// view=blind|unblinded; the host decodes ResultJSON verbatim.
	QueryProjection func(input runs.QueryProjectionInput) (runs.QueryProjectionResult, error)
	// ExecuteCommand writes through the Bridge v2 generic channel. Sample
	// exclusion uses kind execution_event.create and categorical/table
	// readout entry uses kind readout.submit; Core owns all validation.
	ExecuteCommand func(input runs.ExecuteCommandInput) (runs.ExecuteCommandResult, error)
}

// RecordKey is the host's opaque memo key for the Core record projection
// (run, revision, asset coordinates, FOV plan, view). History never parses it.
type RecordKey string

// CheckAckView is one operator verdict sent back to Core.
type CheckAckView struct {
	CheckID string
	Status  string
}

// State holds the mutable UI state of the run workspace.
type State struct {
	VM        *RunWorkspaceViewModel
	Selected  string
	Callbacks Callbacks

	// Checks holds the operator's pending checkbox state per (task, check).
	Checks map[string]*bool

	// Collapsed tracks whether a legacy step group is collapsed in the rail.
	Collapsed map[string]bool

	// Expanded is the stable expansion state for the tree rail. Pointers are
	// kept across frames because components.TreeItem owns the toggle target.
	Expanded map[string]*bool
	// expansionFingerprint detects arrow/keyboard toggles for session saving.
	expansionFingerprint string
	expansionInitialized bool

	// WorkspaceTab selects task (0), history (1), or Run-wide timeline (2).
	WorkspaceTab int
	// SelectedTimelineTask highlights this task's entries after asset navigation.
	SelectedTimelineTask string

	// ShowHistory toggles the selected task's operation-history layer.
	ShowHistory bool

	// ShowProcedure keeps narrow-layout procedure metadata collapsed by default.
	ShowProcedure bool

	// ShowTechnical reveals the raw step/block/protocol identifiers. It stays
	// collapsed so the default view is identifier-free (UX-P2).
	ShowTechnical bool
	// ShowTreeIssues reveals the frozen-plan layer diagnostics. They stay
	// collapsed so the default rail is diagnostic-free (UX-P2).
	ShowTreeIssues bool
	// BlindErr carries the last blind-projection failure. The mask stays as
	// it was and the header shows cause + retry (UX-STATE, UX-P5).
	BlindErr string

	// railScrollKey remembers which selection was last scrolled into view.
	railScrollKey string
	// railInline reports that the flow rail shares the page scroll at narrow
	// width, where revealing its selection must not scroll the recording away.
	railInline bool

	Narrow bool

	// RecordingDirty tracks unsaved recording edits per task (UX-STATE, UX-P5).
	RecordingDirty map[string]bool
	// Bench holds the run bench (timers, event composer, capture, blinding).
	Bench               *BenchState
	// StepViews keeps per-task asterdoc.View instances for same-source ModeRun block rendering.
	StepViews           map[string]*asterdoc.View
	LoadError           string
	Completing          bool
	CompletionError     string
	CompletionErrorTask string
	CheckError          string
	CheckErrorTask      string
	pendingCheckAcks    []CheckAckView
}

// NewState constructs the runtime state from a view model.
func NewState(vm *RunWorkspaceViewModel, cb Callbacks) *State {
	s := &State{
		VM:             vm,
		Callbacks:      cb,
		Checks:         map[string]*bool{},
		Collapsed:      map[string]bool{},
		Expanded:       map[string]*bool{},
		RecordingDirty: map[string]bool{},
		StepViews:      map[string]*asterdoc.View{},
	}
	if vm != nil && vm.Selected != nil {
		s.Selected = vm.Selected.TaskKey
	}
	return s
}

func (s *State) SetRecordingDirty(taskKey string, dirty bool) {
	if s == nil || taskKey == "" {
		return
	}
	if s.RecordingDirty == nil {
		s.RecordingDirty = map[string]bool{}
	}
	s.RecordingDirty[taskKey] = dirty
}

// treeOpen returns the stable expansion target for a tree node. Roots and
// composites default to open so the real four-parent/fourteen-row case is
// visible immediately; the user can collapse any branch.
func (s *State) treeOpen(id string) *bool {
	if open, ok := s.Expanded[id]; ok {
		return open
	}
	open := true
	s.Expanded[id] = &open
	return &open
}

// TreeCollapsed returns the collapsed branch IDs for the session. It includes
// legacy rail groups so one session field serves both navigation modes.
func (s *State) TreeCollapsed() map[string]bool {
	out := map[string]bool{}
	for id, open := range s.Expanded {
		if open != nil && !*open {
			out[id] = true
		}
	}
	for id, collapsed := range s.Collapsed {
		if collapsed {
			out[id] = true
		}
	}
	return out
}

// SetTreeCollapsed restores expansion from the session. Unknown IDs are
// ignored; the default remains open.
func (s *State) SetTreeCollapsed(collapsed map[string]bool) {
	for id, value := range collapsed {
		open := s.treeOpen(id)
		*open = !value
	}
}

// treeExpansionFingerprint changes when any branch toggles, letting the host
// persist arrow/keyboard expansion without a custom click hook.
func (s *State) treeExpansionFingerprint() string {
	ids := make([]string, 0, len(s.Expanded))
	for id, open := range s.Expanded {
		if open != nil && !*open {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	return strings.Join(ids, "\x00")
}

// SyncFrom replaces the view model after a Core reload, preserving UI state
// (selection, collapsible groups) but never inventing facts.
func (s *State) SyncFrom(vm *RunWorkspaceViewModel) {
	if vm == nil {
		return
	}
	s.VM = vm
	if vm.Status != "error" {
		s.LoadError = ""
	}
	for taskKey, task := range vm.TaskByKey {
		for _, c := range task.Checks {
			key := taskKey + "\x00" + c.CheckID
			if v, ok := s.Checks[key]; ok && v != nil {
				*v = c.Status == "passed"
			}
		}
	}
	if s.Selected != "" {
		if _, ok := vm.TaskByKey[s.Selected]; ok {
			return
		}
		// Keep a step-node selection valid too.
		for _, step := range vm.Steps {
			if step.TaskKey == s.Selected {
				return
			}
		}
	}
	s.Selected = ""
	if vm.Selected != nil {
		s.Selected = vm.Selected.TaskKey
	}
}

// selectedTask returns the task view model for the current selection, falling
// back to the first task in the run.
func (s *State) selectedTask() *RunTaskViewModel {
	if s.VM == nil {
		return nil
	}
	if s.Selected != "" {
		if t, ok := s.VM.TaskByKey[s.Selected]; ok {
			return t
		}
		for _, step := range s.VM.Steps {
			if step.TaskKey == s.Selected {
				if len(step.Children) > 0 {
					return step.Children[0]
				}
				return step
			}
		}
	}
	if s.VM.Selected != nil {
		return s.VM.Selected
	}
	return nil
}

// selectedStep returns the step group that owns the selected task.
func (s *State) selectedStep() *RunTaskViewModel {
	if s.VM == nil {
		return nil
	}
	for _, step := range s.VM.Steps {
		if step.TaskKey == s.Selected {
			return step
		}
		for _, child := range step.Children {
			if child.TaskKey == s.Selected {
				return step
			}
		}
	}
	return nil
}

// checkOn returns the current checkbox value for a check, honouring the
// operator's pending edit first, then the stored status.
func (s *State) checkOn(taskKey, checkID string) *bool {
	key := taskKey + "\x00" + checkID
	if v, ok := s.Checks[key]; ok {
		return v
	}
	task := s.taskByKey(taskKey)
	if task != nil {
		for _, c := range task.Checks {
			if c.CheckID == checkID {
				on := c.Status == "passed"
				b := on
				s.Checks[key] = &b
				return &b
			}
		}
	}
	off := false
	s.Checks[key] = &off
	return &off
}

func (s *State) taskByKey(key string) *RunTaskViewModel {
	if s.VM == nil {
		return nil
	}
	return s.VM.TaskByKey[key]
}

// openBlocking returns the blocking checks that are still unconfirmed for a task.
func (s *State) openBlocking(t *RunTaskViewModel) []CheckViewModel {
	var out []CheckViewModel
	if t == nil {
		return out
	}
	for _, c := range t.Checks {
		if c.Blocking && !*s.checkOn(t.TaskKey, c.CheckID) {
			out = append(out, c)
		}
	}
	return out
}

// acks builds the Core payload from current checkbox state.
func (s *State) acks(t *RunTaskViewModel) []CheckAckView {
	var out []CheckAckView
	if t == nil {
		return out
	}
	for _, c := range t.Checks {
		status := "failed"
		if *s.checkOn(t.TaskKey, c.CheckID) {
			status = "passed"
		}
		out = append(out, CheckAckView{CheckID: c.CheckID, Status: status})
	}
	return out
}

// View renders the whole run workspace: left flow rail and right task workspace.
func View(c *ui.Context, s *State) {
	if s == nil || s.VM == nil {
		return
	}
	t := c.Theme()
	tokens := theme.GetTokens(t)

	ui.Column(c).Fill().Background(tokens.Canvas).Children(func() {
		runHeader(c, s, t, tokens)
		if (s.LoadError != "" || s.VM.Status == "error") && len(s.VM.Steps) == 0 {
			runLoadErrorState(c, s, t, tokens)
			return
		}
		if s.LoadError != "" {
			runLoadErrorBanner(c, s, t, tokens)
		}
		if s.VM.Status == "loading" {
			ui.Row(c).Fill().Center().Children(func() {
				ui.Text(c, "正在加载实验运行数据…").FontSize(float32(tokens.TextBody)).TextColor(t.TextMuted)
			})
			return
		}
		if len(s.VM.Steps) == 0 {
			ui.Row(c).Fill().Center().Children(func() {
				ui.Column(c).Gap(8).Center().Children(func() {
					ui.Text(c, "此运行没有任务").FontSize(float32(tokens.TextSection)).Bold().TextColor(t.Text)
					ui.Text(c, "冻结方案中没有可执行步骤，或步骤未绑定检测。").
						FontSize(float32(tokens.TextMeta)).TextColor(t.TextMuted)
				})
			})
			return
		}
		if s.shouldStack(c) {
			// UX-P1, UX-LAYOUT: at narrow widths put the recording action before
			// navigation and procedure details in the initial viewport. The rail
			// shares this scroll, so it must not reveal its selection here: that
			// scrolled the recording's first rows off screen (UX-03).
			s.railInline = true
			ui.Scroll(c).Grow(1).Children(func() {
				ui.Column(c).FillWidth().Gap(12).Padding(12).Children(func() {
					taskWorkspace(c, s, t, tokens)
					unassociatedRecords(c, s, t, tokens)
					flowRail(c, s, t, tokens, true)
				})
			})
			return
		}
		// ≥1100: left flow chain, middle record projection, right properties.
		if s.railInline {
			// The rail has its own scroll again after a wider layout: reveal the
			// selection there instead of keeping the stale inline key (UX-03).
			s.railInline = false
			s.railScrollKey = ""
		}
		ui.Row(c).Grow(1).MinHeight(0).Children(func() {
			ui.Box(c).Width(railWidth(c)).Shrink(0).FillHeight().
				Background(tokens.RailBackground).
				Border(1, tokens.Border).Children(func() {
				ui.Scroll(c).FillHeight().Children(func() {
					flowRail(c, s, t, tokens, false)
				})
			})
			ui.Column(c).Grow(1).MinWidth(0).FillHeight().Children(func() {
				ui.Scroll(c).Grow(1).Children(func() {
					ui.Column(c).FillWidth().Padding(16, 20).Children(func() {
						recordingWorkspace(c, s, t, tokens)
						unassociatedRecords(c, s, t, tokens)
					})
				})
			})
			ui.Box(c).Width(propertiesWidth(c)).Shrink(0).FillHeight().
				Background(tokens.Canvas).
				Border(1, tokens.Border).Children(func() {
				ui.Scroll(c).FillHeight().Children(func() {
					ui.Column(c).FillWidth().Padding(16).Children(func() {
						propertiesPanel(c, s, t, tokens)
					})
				})
			})
		})
	})
	if task := s.selectedTask(); task != nil && s.StepViews != nil {
		if sv := s.StepViews[task.TaskKey]; sv != nil {
			sv.RenderPlateOverlay(c)
		}
	}
}

func (s *State) shouldStack(c *ui.Context) bool {
	if s.Narrow {
		return true
	}
	if c != nil {
		if w, _ := c.Size(); w > 0 && w < 1100 {
			return true
		}
	}
	return false
}

func railWidth(c *ui.Context) float32 {
	if c != nil {
		if w, _ := c.Size(); w > 0 && w < 1200 {
			return 240
		}
	}
	return 272
}

// propertiesWidth keeps the right column compact so the middle recording panel
// stays the widest region (UX-LAYOUT).
func propertiesWidth(c *ui.Context) float32 {
	if c != nil {
		if w, _ := c.Size(); w > 0 && w < 1360 {
			return 300
		}
	}
	return 360
}

// runHeader is the compact title bar: run title, status, frozen revision, and
// the blind switch. The switch consumes run.workspace view=blind|unblinded
// through QueryProjection (Bridge v2): blind mode hides real group treatment
// names — pure display masking, wells stay visible as coordinates — and Core
// grants it only to designs that declare a blinding basis (UX-P2, UX-P3,
// UX-STATE, UX-DESIGN).
func runHeader(c *ui.Context, s *State, t *ui.Theme, tokens theme.Tokens) {
	vm := s.VM
	done, total := 0, 0
	for _, step := range vm.Steps {
		total++
		if step.Status == TaskStatusCompleted {
			done++
		}
	}
	ui.Row(c).FillWidth().Height(52).Padding(0, 16).
		Background(tokens.Surface).Border(1, tokens.Border).
		AlignItems(ui.Center).Children(func() {
		ui.Text(c, vm.Title).FontSize(float32(tokens.TextSection)).Bold().TextColor(t.Text).
			SingleLine().Ellipsis("…").Shrink(1).MaxWidth(420)
		ui.Spacer(c)
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			if vm.Status != "" {
				statusPill(c, vm.Status, tokens)
			}
			if planRev := vm.DesignRevision; planRev > 0 || vm.AsterDocRevision > 0 {
				if planRev == 0 {
					planRev = vm.AsterDocRevision
				}
				ui.Text(c, fmt.Sprintf("冻结方案第 %d 版", planRev)).FontSize(float32(tokens.TextMeta)).
					TextColor(t.TextMuted).FontFeatures("tnum")
			}
			ui.Text(c, fmt.Sprintf("%d/%d 步已完成", done, total)).FontSize(float32(tokens.TextMeta)).
				TextColor(t.TextMuted).FontFeatures("tnum")
			if b := components.Button(c, blindSwitchLabel(s.bench().Blind), components.ButtonGhost); b.Clicked() {
				s.toggleBlind()
			}
		})
	})
	// The banners below only exist when visible, so the default header stays
	// pixel-identical to main: same 52px row, same border, nothing above it.
	if s.bench().Blind {
		ui.Row(c).FillWidth().Padding(2, 16).Background(tokens.Surface).Border(1, tokens.Border).
			AlignItems(ui.Center).Gap(t.Space(1)).Children(func() {
			ui.Text(c, "已设盲 · 分组已隐藏，孔位坐标保持可见").FontSize(t.Rem(.68)).TextColor(t.TextMuted)
		})
	}
	// UX-STATE, UX-P5: a rejected blind projection keeps the current mask
	// and names cause + retry instead of failing silently.
	if s.BlindErr != "" {
		ui.Row(c).FillWidth().Padding(6, 16).Background(tokens.DangerBg).
			AlignItems(ui.Center).Gap(t.Space(1.5)).Children(func() {
			ui.Text(c, "设盲视图："+s.BlindErr).FontSize(t.Rem(.72)).TextColor(tokens.DangerText)
			ui.Spacer(c)
			if components.Button(c, "重试", components.ButtonGhost).Clicked() {
				s.BlindErr = ""
				s.toggleBlind()
			}
		})
	}
}

func statusPill(c *ui.Context, status string, tokens theme.Tokens) {
	label := treeStatusLabel(status)
	switch status {
	case "error":
		label = "阻塞"
	case "loading":
		label = "加载中"
	}
	if label == "" {
		label = "未开始"
	}
	bg, fg := tokens.NeutralBg, tokens.NeutralText
	switch label {
	case "进行中":
		bg, fg = tokens.WarningBg, tokens.WarningText
	case "已完成":
		bg, fg = tokens.SuccessBg, tokens.SuccessText
	case "阻塞":
		bg, fg = tokens.DangerBg, tokens.DangerText
	}
	ui.Row(c).Padding(3, 9).Radius(6).Background(bg).Children(func() {
		ui.Text(c, label).FontSize(float32(tokens.TextMeta)).Bold().TextColor(fg)
	})
}

// flowRail is the vertical execution navigation on the left (UX-STATE).
func flowRail(c *ui.Context, s *State, t *ui.Theme, tokens theme.Tokens, inline bool) {
	ui.Column(c).FillWidth().Gap(t.Space(1)).Padding(t.Space(3), t.Space(2)).Children(func() {
		ui.Text(c, "实验流程").FontSize(float32(tokens.TextMeta)).Bold().TextColor(t.TextMuted).
			Padding(0, 0, t.Space(2), 0)
		if len(s.VM.TreeIssues) > 0 {
			open := s.ShowTreeIssues
			label := fmt.Sprintf("层级诊断（%d）", len(s.VM.TreeIssues))
			components.Collapsible(c, label, &open, components.CollapsibleOptions{}, func() {
				ui.Column(c).FillWidth().Gap(2).Children(func() {
					for _, issue := range s.VM.TreeIssues {
						ui.Text(c, issue).FontSize(t.Rem(.66)).TextColor(tokens.WarningText)
					}
				})
			})
			s.ShowTreeIssues = open
		}
		if s.VM.Tree != nil {
			fingerprint := s.treeExpansionFingerprint()
			if s.expansionInitialized && fingerprint != s.expansionFingerprint && s.Callbacks.ToggleTree != nil {
				s.Callbacks.ToggleTree()
			}
			s.expansionInitialized = true
			s.expansionFingerprint = fingerprint
			components.Tree(c, func() {
				for _, root := range s.VM.Tree.Roots {
					treeNode(c, s, t, tokens, root)
				}
			})
			return
		}
		for i, step := range s.VM.Steps {
			stepNode(c, s, t, tokens, step)
			if i < len(s.VM.Steps)-1 {
				flowConnector(c, t, tokens)
			}
		}
	})
}

func treeNode(c *ui.Context, s *State, t *ui.Theme, tokens theme.Tokens, node *workflowtree.Node) {
	if node == nil {
		return
	}
	if node.Kind == workflowtree.KindTask {
		treeLeafItem(c, s, t, tokens, node.Title, node)
		return
	}
	if !node.Organizational && !node.Composite && len(node.Children) == 1 && node.Children[0].Kind == workflowtree.KindTask {
		treeLeafItem(c, s, t, tokens, node.Title, node.Children[0])
		return
	}
	open := s.treeOpen(node.ID)
	label := node.Title
	if node.Composite && !node.Organizational {
		label = fmt.Sprintf("%s   %d/%d", label, node.Completed, node.Total)
	}
	if node.Timepoint != "" {
		label += " · " + node.Timepoint
	}
	if node.Organizational {
		label += " · 组织节点"
	} else if status := treeStatusLabel(node.Status); status != "" {
		label += " · " + status
	}
	item := components.TreeItemWithLeading(c, label, open, s.Selected == node.ID, func(c *ui.Context, size float32) {
		if node.Organizational {
			return
		}
		if node.Composite {
			components.Icon(c, lucide.Folder()).Size(size, size).TextColor(treeStatusColor(node.Status, t, tokens))
			return
		}
		ui.Box(c).Size(7, 7).Radius(3.5).Background(treeStatusColor(node.Status, t, tokens))
	}, func() {
		for _, child := range node.Children {
			treeNode(c, s, t, tokens, child)
		}
	})
	if item.Clicked() {
		if node.Composite {
			*open = !*open
		} else if len(node.Children) > 0 {
			selectTreeTask(s, node.Children[0].ID)
		}
	}
}

func treeLeafItem(c *ui.Context, s *State, t *ui.Theme, tokens theme.Tokens, title string, task *workflowtree.Node) {
	selected := s.Selected == task.ID
	label := title
	if task.Timepoint != "" {
		label += " · " + task.Timepoint
	}
	if status := treeStatusLabel(task.Status); status != "" {
		label += " · " + status
	}
	item := components.TreeItemWithLeading(c, label, nil, selected, func(c *ui.Context, size float32) {
		ui.Box(c).Size(7, 7).Radius(3.5).Background(treeStatusColor(task.Status, t, tokens))
	}, nil)
	if !s.railInline && selected && s.railScrollKey != task.ID {
		item.ScrollIntoView(ui.ScrollIntoViewOptions{Block: ui.ScrollCenter})
		s.railScrollKey = task.ID
	}
	if item.Clicked() {
		selectTreeTask(s, task.ID)
	}
}

func selectTreeTask(s *State, taskKey string) {
	if taskKey == "" {
		return
	}
	s.Selected = taskKey
	if s.Callbacks.SelectTask != nil {
		s.Callbacks.SelectTask(taskKey)
	}
}

func treeStatusColor(status string, t *ui.Theme, tokens theme.Tokens) ui.Color {
	switch status {
	case "completed", "done":
		return t.Success
	case "current", "running", "in_progress":
		return t.Accent
	case "blocked":
		return t.Danger
	case "skipped", "skipped_by_rule":
		return t.Warning
	case "unresolved":
		return t.Warning
	}
	return tokens.NeutralText
}

func treeStatusLabel(status string) string {
	switch status {
	case "pending", "not_started", "not-started", "planned":
		return "未开始"
	case "current", "running", "in_progress":
		return "进行中"
	case "completed", "done":
		return "已完成"
	case "blocked":
		return "阻塞"
	case "skipped", "skipped_by_rule":
		return "已跳过"
	default:
		return ""
	}
}

// flowConnector draws the vertical link and arrow between two step nodes.
func flowConnector(c *ui.Context, t *ui.Theme, tokens theme.Tokens) {
	ui.Column(c).Width(32).Height(20).AlignItems(ui.Center).Children(func() {
		ui.Box(c).Width(2).Height(6).Background(tokens.BorderStrong)
		ui.Text(c, "↓").FontSize(t.Rem(1.0)).TextColor(tokens.BorderStrong)
	})
}

// stepNode renders one step group and its tasks.
func stepNode(c *ui.Context, s *State, t *ui.Theme, tokens theme.Tokens, step *RunTaskViewModel) {
	selected := s.Selected == step.TaskKey
	for _, child := range step.Children {
		if s.Selected == child.TaskKey {
			selected = true
		}
	}
	ui.Column(c).FillWidth().Children(func() {
		b := ui.Row(c).FillWidth().Padding(t.Space(2), t.Space(2)).Radius(8).
			Background(tokens.Surface).Border(1, tokens.Border).
			AlignItems(ui.Center).Gap(t.Space(2))
		if selected {
			b.Background(t.Accent.Alpha(.10)).Border(1, t.Accent.Alpha(.45))
		}
		b.Children(func() {
			statusDot(c, step.Status, t, tokens)
			ui.Column(c).Grow(1).MinWidth(0).Gap(1).Children(func() {
				ui.Text(c, step.StepTitle).FontSize(t.Rem(.9)).FontWeight(600).
					TextColor(t.Text).SingleLine().Ellipsis("…")
				meta := ""
				if step.Timepoint != "" {
					meta = step.Timepoint
				}
				if status := treeStatusLabel(string(step.Status)); status != "" {
					if meta != "" {
						meta += " · "
					}
					meta += status
				}
				if meta != "" {
					ui.Text(c, meta).FontSize(t.Rem(.72)).TextColor(t.TextMuted).
						SingleLine().Ellipsis("…")
				}
			})
			if len(step.Children) > 1 {
				if components.IconButton(c, lucide.ChevronDown(), "展开任务", components.IconButtonCompact).Clicked() {
					s.Collapsed[step.TaskKey] = !s.Collapsed[step.TaskKey]
				}
			}
		})
		if !s.railInline && selected && s.railScrollKey != step.TaskKey {
			b.ScrollIntoView(ui.ScrollIntoViewOptions{Block: ui.ScrollCenter})
			s.railScrollKey = step.TaskKey
		}
		if b.Clicked() {
			target := step.TaskKey
			if len(step.Children) > 0 {
				// A step group's TaskKey is a synthetic step id, not a Core
				// task key: selecting it cannot survive a session restore
				// (UX-03). Select the first real task instead.
				target = step.Children[0].TaskKey
			}
			s.Selected = target
			if s.Callbacks.SelectTask != nil {
				s.Callbacks.SelectTask(target)
			}
		}
		// Collapsed by default when multiple tasks; the selected task is always
		// reachable from the right panel anyway.
		if len(step.Children) > 1 && !s.Collapsed[step.TaskKey] {
			ui.Column(c).FillWidth().Padding(t.Space(2), 0, 0, t.Space(3)).Gap(t.Space(.5)).
				Children(func() {
					for _, child := range step.Children {
						childItem(c, s, t, tokens, child)
					}
				})
		}
	})
}

func childItem(c *ui.Context, s *State, t *ui.Theme, tokens theme.Tokens, task *RunTaskViewModel) {
	selected := s.Selected == task.TaskKey
	row := ui.ButtonBase(c).FillWidth().Radius(5).
		Padding(t.Space(1.25), t.Space(1.5)).Gap(t.Space(1.5))
	if selected {
		row.Background(t.Accent.Alpha(.08))
	}
	row.Children(func() {
		statusDot(c, task.Status, t, tokens)
		label := task.Timepoint
		if label == "" {
			label = task.StepTitle
		}
		if status := treeStatusLabel(string(task.Status)); status != "" {
			label += " · " + status
		}
		ui.Text(c, label).FontSize(t.Rem(.78)).TextColor(t.TextMuted).
			SingleLine().Ellipsis("…")
	})
	if !s.railInline && selected && s.railScrollKey != task.TaskKey {
		row.ScrollIntoView(ui.ScrollIntoViewOptions{Block: ui.ScrollCenter})
		s.railScrollKey = task.TaskKey
	}
	if row.Clicked() {
		s.Selected = task.TaskKey
		if s.Callbacks.SelectTask != nil {
			s.Callbacks.SelectTask(task.TaskKey)
		}
	}
}

func statusDot(c *ui.Context, status TaskStatus, t *ui.Theme, tokens theme.Tokens) {
	color := tokens.NeutralText
	switch status {
	case TaskStatusCompleted:
		color = t.Success
	case TaskStatusCurrent:
		color = t.Accent
	case TaskStatusBlocked:
		color = t.Danger
	}
	ui.Box(c).Size(8, 8).Radius(4).Shrink(0).Background(color)
}

// taskWorkspace is the narrow single column: recording first, then the
// procedure metadata collapsed below it (UX-P1, UX-LAYOUT).
func taskWorkspace(c *ui.Context, s *State, t *ui.Theme, tokens theme.Tokens) {
	task := s.selectedTask()
	if task == nil {
		ui.Text(c, "选择一个步骤查看操作说明").FontSize(float32(tokens.TextBody)).TextColor(t.TextMuted)
		return
	}
	ui.Column(c).FillWidth().Gap(float32(tokens.Spacing12)).Children(func() {
		tab := &s.WorkspaceTab
		components.Tabs(c, tab, components.TabsHighlight, "任务", "历史", "时间线")
		switch *tab {
		case 1:
			historyTab(c, s, t, tokens, task)
			return
		case 2:
			timelineTab(c, s, t, tokens)
			return
		}
		// UX-P1: the bench (timers, skip/deviation entry, capture) leads the
		// task column so the step's primary action stays in the first viewport.
		renderBench(c, s, task, t, tokens)
		renderFrozenStepBlocks(c, s, task, t, tokens)
		recordingLayer(c, s, t, tokens, task)
		open := s.ShowProcedure
		components.Collapsible(c, "操作说明与元数据", &open, components.CollapsibleOptions{}, func() {
			procedureLayer(c, s, t, tokens, task)
		})
		s.ShowProcedure = open
	})
}

// recordingWorkspace is the middle column at ≥1100: the same record projection
// as the narrow layout, reusing RecordingView (no second fact store).
func recordingWorkspace(c *ui.Context, s *State, t *ui.Theme, tokens theme.Tokens) {
	task := s.selectedTask()
	if task == nil {
		ui.Text(c, "选择一个步骤查看操作说明").FontSize(float32(tokens.TextBody)).TextColor(t.TextMuted)
		return
	}
	ui.Column(c).FillWidth().Gap(float32(tokens.Spacing12)).Children(func() {
		tab := &s.WorkspaceTab
		components.Tabs(c, tab, components.TabsHighlight, "任务", "历史", "时间线")
		switch *tab {
		case 1:
			historyTab(c, s, t, tokens, task)
			return
		case 2:
			timelineTab(c, s, t, tokens)
			return
		}
		renderBench(c, s, task, t, tokens)
		renderFrozenStepBlocks(c, s, task, t, tokens)
		recordingLayer(c, s, t, tokens, task)
	})
}

// propertiesPanel is the compact right column: the frozen procedure, its live
// checks, and the collapsed raw identifiers (UX-P2, UX-LAYOUT).
func propertiesPanel(c *ui.Context, s *State, t *ui.Theme, tokens theme.Tokens) {
	task := s.selectedTask()
	if task == nil {
		return
	}
	procedureLayer(c, s, t, tokens, task)
}

func procedureLayer(c *ui.Context, s *State, t *ui.Theme, tokens theme.Tokens, task *RunTaskViewModel) {
	passed, total := 0, len(task.Checks)
	for _, check := range task.Checks {
		if check.Status == "passed" {
			passed++
		}
	}
	recorded, files := 0, 0
	if task.Actual != nil {
		recorded = len(task.Actual.Measurements)
		files = len(task.Actual.Files)
	}
	instruction := strings.TrimSpace(task.Instruction)
	next := "录入读数并保存"
	switch {
	case total > passed:
		if instruction == "" {
			instruction = "先完成执行确认，再记录本步骤读数。"
		}
		next = fmt.Sprintf("完成 %d 项执行确认", total-passed)
	case recorded > 0:
		if instruction == "" {
			instruction = "读数已记录；核对后保存读数并完成任务。"
		}
		next = "核对读数，保存并完成任务"
	case instruction == "":
		instruction = "执行本步骤，并在下方实验记录中录入读数。"
	}


	ui.Column(c).FillWidth().Padding(16).Radius(float32(tokens.PanelRadius)).
		Background(tokens.Surface).Border(1, tokens.Border).Gap(t.Space(2)).Children(func() {
		ui.Row(c).FillWidth().Wrap().AlignItems(ui.Center).Gap(t.Space(2)).Children(func() {
			ui.Text(c, task.StepTitle).FontSize(float32(tokens.TextSection)).FontWeight(600).TextColor(t.Text)
			ui.Spacer(c)
			taskStatusPill(c, task.Status, t, tokens)
			ui.Text(c, "当前步骤").FontSize(t.Rem(.7)).TextColor(t.TextMuted)
		})
		// The procedure and planned parameters are frozen; UX-STATE/UX-P2.
		ui.Text(c, "操作说明 · 已冻结 · 只读").FontSize(t.Rem(.72)).Bold().TextColor(t.TextMuted)
		ui.Text(c, instruction).FontSize(t.Rem(.82)).TextColor(t.Text)
		// UX-P1, UX-P5, UX-STATE, UX-DONE: the no-readout step has one retryable primary action.
		if !s.shouldStack(c) {
			completionAction(c, s, t, tokens, task)
		}


		if len(task.Parameters) > 0 {
			ui.Divider(c)
			ui.Text(c, "操作参数 · 已冻结 · 只读").FontSize(t.Rem(.78)).Bold().TextColor(t.TextMuted)
			ui.Row(c).Wrap().Gap(t.Space(2)).Children(func() {
				for _, p := range task.Parameters {
					paramChip(c, p, t, tokens)
				}
			})
		}

		// Checks come before recording: the operator confirms the step first.
		if len(task.Checks) > 0 {
			ui.Divider(c)
			checkList(c, s, t, tokens, task)
		}

		ui.Divider(c)
		ui.Text(c, "要记录什么").FontSize(t.Rem(.78)).Bold().TextColor(t.TextMuted)
		ui.Row(c).FillWidth().Wrap().Gap(t.Space(2)).Children(func() {
			statChip(c, fmt.Sprintf("已记录读数 %d", recorded), t, tokens)
			statChip(c, fmt.Sprintf("执行确认 %d/%d", passed, total), t, tokens)
			if files > 0 {
				statChip(c, fmt.Sprintf("关联文件 %d", files), t, tokens)
			}
		})
		if task.Actual != nil && len(task.Actual.Measurements) > 0 {
			ui.Row(c).Wrap().Gap(t.Space(1.5)).Children(func() {
				limit := len(task.Actual.Measurements)
				if limit > 4 {
					limit = 4
				}
				for _, m := range task.Actual.Measurements[:limit] {
					ui.Row(c).Padding(4, 9).Radius(6).Background(tokens.Elevated).
						Border(1, tokens.Border).Gap(6).AlignItems(ui.Center).Children(func() {
						ui.Text(c, fallback(m.SampleID, "未分配")).FontSize(t.Rem(.72)).TextColor(t.TextMuted)
						ui.Text(c, strings.TrimSpace(m.Field+" "+m.Value+" "+m.Unit)).FontSize(t.Rem(.78)).
							FontWeight(600).TextColor(t.Text)
					})
				}
				if len(task.Actual.Measurements) > limit {
					ui.Text(c, fmt.Sprintf("+%d", len(task.Actual.Measurements)-limit)).
						FontSize(t.Rem(.72)).TextColor(t.TextMuted)
				}
			})
		}
		ui.Text(c, "在下方「实验记录」中录入读数；保存后写入 Run 记录。").
			FontSize(t.Rem(.72)).TextColor(t.TextMuted)

		ui.Row(c).FillWidth().Padding(6, 10).Radius(6).Background(tokens.Elevated).
			Border(1, tokens.Border).AlignItems(ui.Center).Gap(t.Space(1)).Children(func() {
			ui.Text(c, "下一步").FontSize(t.Rem(.7)).Bold().TextColor(t.TextMuted)
			ui.Text(c, next).FontSize(t.Rem(.76)).TextColor(t.Text)
		})

		if len(task.Modules) > 0 {
			ui.Divider(c)
			ui.Text(c, "步骤资源").FontSize(t.Rem(.78)).Bold().TextColor(t.TextMuted)
			ui.Row(c).Wrap().Gap(t.Space(1.5)).Children(func() {
				for _, m := range task.Modules {
					moduleChip(c, m, t, tokens)
				}
			})
		}

		if identity := frozenIdentity(task); len(identity) > 0 {
			if s.shouldStack(c) {
				// The narrow procedure panel is itself collapsed, so its body
				// may carry the identifiers directly: one click on
				// 操作说明与元数据 reveals them (UX-P2).
				ui.Text(c, "技术属性 · "+strings.Join(identity, " · ")).
					FontSize(t.Rem(.68)).TextColor(t.TextMuted)
			} else {
				open := s.ShowTechnical
				components.Collapsible(c, "技术属性（只读）", &open, components.CollapsibleOptions{}, func() {
					ui.Column(c).FillWidth().Gap(2).Children(func() {
						for _, line := range identity {
							ui.Text(c, line).FontSize(t.Rem(.68)).TextColor(t.TextMuted)
						}
					})
				})
				s.ShowTechnical = open
			}
		}
	})
}

// frozenIdentity lists the raw step/block/protocol identifiers. They are hidden
// behind the properties disclosure by default (UX-P2).
func frozenIdentity(task *RunTaskViewModel) []string {
	if task == nil {
		return nil
	}
	var out []string
	if v := strings.TrimSpace(task.StepPath); v != "" {
		out = append(out, "路径 "+v)
	}
	if v := strings.TrimSpace(task.AssayID); v != "" {
		out = append(out, "检测 "+v)
	}
	if v := strings.TrimSpace(task.StepID); v != "" {
		out = append(out, "步骤 ID "+v)
	}
	if v := strings.TrimSpace(task.ProtocolVersionID); v != "" {
		out = append(out, "协议版本 "+v)
	}
	for _, m := range task.Modules {
		if id := strings.TrimSpace(m.ID); id != "" {
			out = append(out, "资源 "+id)
		}
	}
	return out
}

// canMarkComplete keeps the no-readout completion path exclusive with checks
// and measurement plans (UX-P1, UX-STATE, UX-DONE).
func canMarkComplete(task *RunTaskViewModel) bool {
	if task == nil || task.Status == TaskStatusCompleted || task.Status == TaskStatusSkipped || len(task.Checks) != 0 {
		return false
	}
	for _, module := range task.Modules {
		if module.Kind == "data_table" || module.Kind == "readout" || module.Kind == "measurement_profile" {
			return false
		}
	}
	return true
}

func (s *State) complete(task *RunTaskViewModel) {
	if s == nil || task == nil || s.Completing || s.Callbacks.Complete == nil {
		return
	}
	s.Completing = true
	s.CompletionError = ""
	err := s.Callbacks.Complete(task.TaskKey, s.acks(task))
	s.Completing = false
	if err != nil {
		s.CompletionError = err.Error()
		s.CompletionErrorTask = task.TaskKey
	}
}
func completionAction(c *ui.Context, s *State, t *ui.Theme, tokens theme.Tokens, task *RunTaskViewModel) {
	if !canMarkComplete(task) {
		return
	}
	errorText := ""
	if s.CompletionErrorTask == task.TaskKey {
		errorText = s.CompletionError
	}
	ui.Divider(c)
	if errorText != "" {
		ui.Row(c).FillWidth().Padding(8, 10).Radius(6).
			Background(tokens.DangerBg).Border(1, tokens.DangerText).Children(func() {
			ui.Text(c, "完成失败："+errorText).FontSize(t.Rem(.74)).
				TextColor(tokens.DangerText)
		})
	}
	label := "标记完成"
	if errorText != "" {
		label = "重试完成"
	} else if s.Completing {
		label = "提交中…"
	}
	button := components.Button(c, label, components.ButtonPrimary)
	button.Disabled(s.Completing || s.Callbacks.Complete == nil)
	if button.Clicked() && !s.Completing && s.Callbacks.Complete != nil {
		s.complete(task)
	}
}


func statChip(c *ui.Context, label string, t *ui.Theme, tokens theme.Tokens) {
	ui.Row(c).Padding(2, 7).Radius(5).Background(tokens.Elevated).
		Border(1, tokens.Border).Children(func() {
		ui.Text(c, label).FontSize(t.Rem(.7)).TextColor(t.TextMuted)
	})
}

func taskStatusPill(c *ui.Context, status TaskStatus, t *ui.Theme, tokens theme.Tokens) {
	bg, fg := tokens.NeutralBg, tokens.NeutralText
	switch status {
	case TaskStatusCurrent:
		bg, fg = tokens.WarningBg, tokens.WarningText
	case TaskStatusCompleted:
		bg, fg = tokens.SuccessBg, tokens.SuccessText
	case TaskStatusBlocked:
		bg, fg = tokens.DangerBg, tokens.DangerText
	case TaskStatusSkipped:
		bg, fg = tokens.WarningBg, tokens.WarningText
	}
	ui.Row(c).Padding(2, 7).Radius(5).Background(bg).Children(func() {
		ui.Text(c, StatusLabel(status)).FontSize(t.Rem(.68)).Bold().TextColor(fg)
	})
}

func fallback(value string, alt ...string) string {
	if strings.TrimSpace(value) != "" {
		return value
	}
	if len(alt) > 0 {
		return alt[0]
	}
	return "—"
}

func paramChip(c *ui.Context, p PlannedParameter, t *ui.Theme, tokens theme.Tokens) {
	value := p.Value
	if p.Unit != "" {
		value += " " + p.Unit
	}
	ui.Row(c).Padding(4, 9).Radius(6).Background(tokens.Elevated).
		Border(1, tokens.Border).Gap(6).AlignItems(ui.Center).Children(func() {
		ui.Text(c, p.Label).FontSize(t.Rem(.72)).TextColor(t.TextMuted)
		ui.Text(c, value).FontSize(t.Rem(.78)).FontWeight(600).TextColor(t.Text)
	})
}

func moduleChip(c *ui.Context, m ModuleRef, t *ui.Theme, tokens theme.Tokens) {
	ui.Row(c).Padding(4, 9).Radius(6).Background(tokens.Elevated).
		Border(1, tokens.Border).Gap(6).AlignItems(ui.Center).Children(func() {
		ui.Text(c, moduleLabel(m.Kind, m.Title)).FontSize(t.Rem(.76)).TextColor(t.Text)
		// The raw module id is provenance: it stays in the properties
		// disclosure instead of the default view (UX-P2).
		if m.Role != "" {
			ui.Text(c, moduleRoleLabel(m.Role)).FontSize(t.Rem(.7)).TextColor(t.TextMuted)
		}
	})
}

func moduleRoleLabel(role string) string {
	switch strings.TrimSpace(role) {
	case "readout":
		return "读数"
	case "timer":
		return "计时"
	case "attachment":
		return "采图/附件"
	case "instrument":
		return "仪器"
	case "prose":
		return "说明"
	case "calculator":
		return "配液/换算"
	case "param_table", "parameters":
		return "参数"
	case "checklist", "check", "checks":
		return "核查"
	case "subflow":
		return "子步骤"
	case "plate":
		return "孔板"
	case "group_table", "group", "groups":
		return "分组"
	case "material":
		return "试剂"
	case "data_table":
		return "数据表"
	case "figure":
		return "示意图"
	case "procedure":
		return "操作说明"
	case "other":
		return "通用模块"
	default:
		return role
	}
}

func moduleLabel(kind, title string) string {
	if title != "" {
		return title
	}
	switch kind {
	case "prose":
		return "说明文本"
	case "group_table":
		return "实验分组表"
	case "param_table":
		return "参数表"
	case "figure":
		return "示意图 / SVG"
	case "data_table":
		return "数据记录表"
	case "checklist":
		return "检查清单"
	case "calculator":
		return "实验计算器"
	case "timer":
		return "计时器"
	case "attachment":
		return "附件与采集"
	case "subflow":
		return "子流程"
	case "instrument_profile":
		return "仪器能力档案"
	case "dilution":
		return "稀释计算"
	case "plate":
		return "孔板"
	case "readout", "measurement_profile":
		return "测量方案"
	case "material":
		return "材料"
	case "sample":
		return "样本"
	case "container":
		return "容器"
	}
	return kind
}

func checkList(c *ui.Context, s *State, t *ui.Theme, tokens theme.Tokens, task *RunTaskViewModel) {
	open := s.openBlocking(task)
	ui.Row(c).AlignItems(ui.Center).Gap(t.Space(1.5)).Children(func() {
		ui.Text(c, "执行确认").FontSize(t.Rem(.78)).Bold().TextColor(t.TextMuted)
		if len(open) > 0 {
			ui.Row(c).Padding(2, 7).Radius(5).Background(tokens.DangerBg).Children(func() {
				ui.Text(c, fmt.Sprintf("%d 项未确认", len(open))).FontSize(t.Rem(.68)).
					Bold().TextColor(tokens.DangerText)
			})
		}
	})
	for _, check := range task.Checks {
		check := check
		on := s.checkOn(task.TaskKey, check.CheckID)
		box := components.Checkbox(c, on, labelOr(check.Label, check.CheckID))
		if box.Changed() && s.Callbacks.SaveChecks != nil {
			s.saveChecks(task)
		}
		statusText, statusColor := "未知", t.TextMuted
		switch check.Status {
		case "passed":
			statusText, statusColor = "已确认", tokens.SuccessText
		case "failed":
			statusText, statusColor = "未通过", tokens.DangerText
		case "pending":
			statusText = "未确认"
		}
		ui.Text(c, statusText).FontSize(t.Rem(.66)).TextColor(statusColor)
		if check.Instruction != "" {
			ui.Text(c, check.Instruction).FontSize(t.Rem(.7)).TextColor(t.TextMuted).
				Padding(0, 0, 0, t.Space(1))
		}
	}
	if s.CheckErrorTask == task.TaskKey && s.CheckError != "" {
		ui.Column(c).FillWidth().Padding(8, 10).Radius(6).Gap(6).
			Background(tokens.DangerBg).Border(1, tokens.DangerText).Children(func() {
			ui.Text(c, "未保存 · "+s.CheckError).FontSize(t.Rem(.74)).
				TextColor(tokens.DangerText)
			retryBtn := components.Button(c, "重试", components.ButtonSecondary)
			retryBtn.Disabled(s.Callbacks.SaveChecks == nil)
			if retryBtn.Clicked() && s.Callbacks.SaveChecks != nil {
				s.retryChecks(task)
			}
		})
	}
}

func (s *State) saveChecks(task *RunTaskViewModel) {
	if s == nil || task == nil {
		return
	}
	s.persistChecks(task, s.acks(task), false)
}

func (s *State) retryChecks(task *RunTaskViewModel) {
	if s == nil || task == nil {
		return
	}
	acks := s.pendingCheckAcks
	if len(acks) == 0 {
		acks = s.acks(task)
	}
	s.persistChecks(task, acks, true)
}

func (s *State) persistChecks(task *RunTaskViewModel, acks []CheckAckView, applyOnSuccess bool) {
	if s.Callbacks.SaveChecks == nil {
		return
	}
	s.CheckError = ""
	s.CheckErrorTask = ""
	if err := s.Callbacks.SaveChecks(task.TaskKey, acks); err != nil {
		s.CheckError = err.Error()
		s.CheckErrorTask = task.TaskKey
		s.pendingCheckAcks = append([]CheckAckView(nil), acks...)
		s.revertChecks(task)
		return
	}
	s.pendingCheckAcks = nil
	if applyOnSuccess {
		s.applyCheckAcks(task.TaskKey, acks)
	}
}

func (s *State) revertChecks(task *RunTaskViewModel) {
	if s == nil || task == nil {
		return
	}
	for _, check := range task.Checks {
		*s.checkOn(task.TaskKey, check.CheckID) = check.Status == "passed"
	}
}

func (s *State) applyCheckAcks(taskKey string, acks []CheckAckView) {
	if s == nil {
		return
	}
	for _, ack := range acks {
		*s.checkOn(taskKey, ack.CheckID) = ack.Status == "passed"
	}
}

func runLoadErrorState(c *ui.Context, s *State, t *ui.Theme, tokens theme.Tokens) {
	reason := strings.TrimSpace(s.LoadError)
	if reason == "" {
		reason = "无法读取实验运行数据。"
	}
	ui.Row(c).Fill().Center().Padding(t.Space(4)).Children(func() {
		ui.Column(c).MaxWidth(520).Padding(16).Radius(float32(tokens.PanelRadius)).
			Background(tokens.DangerBg).Border(1, tokens.DangerText).Gap(t.Space(2)).Children(func() {
			ui.Text(c, "实验运行加载失败").FontSize(float32(tokens.TextSection)).Bold().TextColor(tokens.DangerText)
			ui.Text(c, reason).FontSize(float32(tokens.TextBody)).TextColor(t.Text)
			if s.Callbacks.RetryLoad != nil {
				if components.Button(c, "重试", components.ButtonPrimary).Clicked() {
					s.Callbacks.RetryLoad()
				}
			}
		})
	})
}

func runLoadErrorBanner(c *ui.Context, s *State, t *ui.Theme, tokens theme.Tokens) {
	reason := strings.TrimSpace(s.LoadError)
	if reason == "" {
		return
	}
	ui.Row(c).FillWidth().Padding(8, 16).Background(tokens.DangerBg).Border(1, tokens.DangerText).
		AlignItems(ui.Center).Gap(t.Space(2)).Children(func() {
		ui.Box(c).Grow(1).MinWidth(0).Children(func() {
			ui.Text(c, "实验运行加载失败："+reason).FontSize(t.Rem(.78)).TextColor(tokens.DangerText)
		})
		if s.Callbacks.RetryLoad != nil {
			if components.Button(c, "重试", components.ButtonSecondary).Clicked() {
				s.Callbacks.RetryLoad()
			}
		}
	})
}

func labelOr(value, fallback string) string {
	if strings.TrimSpace(value) != "" {
		return value
	}
	return fallback
}

func recordingLayer(c *ui.Context, s *State, t *ui.Theme, tokens theme.Tokens, task *RunTaskViewModel) {
	ui.Column(c).FillWidth().Padding(16).Radius(float32(tokens.PanelRadius)).
		Background(tokens.Surface).Border(1, tokens.Border).Gap(t.Space(2)).Children(func() {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
			ui.Column(c).Gap(t.Space(.5)).Children(func() {
				ui.Text(c, "实验记录").FontSize(float32(tokens.TextSection)).FontWeight(600).TextColor(t.Text)
				ui.Text(c, task.StepTitle).FontSize(t.Rem(.72)).TextColor(t.TextMuted)
			})
			ui.Spacer(c)
			recorded := len(task.Actual.Measurements)
			if recorded > 0 {
				ui.Text(c, fmt.Sprintf("已记录 %d 项", recorded)).FontSize(t.Rem(.72)).
					TextColor(t.TextMuted)
			}
		})
		if s.shouldStack(c) {
			completionAction(c, s, t, tokens, task)
		}
		if s.Callbacks.Recording != nil {
			s.Callbacks.Recording(c, task)
			return
		}
		ui.Text(c, "没有可用的记录界面。").FontSize(t.Rem(.78)).TextColor(t.TextMuted)
	})
}

func historyLayer(c *ui.Context, s *State, t *ui.Theme, tokens theme.Tokens, task *RunTaskViewModel) {
	open := s.ShowHistory
	components.Collapsible(c, "操作历史", &open, components.CollapsibleOptions{}, func() {
		if len(task.Events) == 0 {
			ui.Text(c, "尚无执行事件。").FontSize(t.Rem(.75)).TextColor(t.TextMuted)
			return
		}
		ui.Column(c).FillWidth().Gap(t.Space(1.5)).Children(func() {
			for _, event := range task.Events {
				event := event
				ui.Column(c).FillWidth().Gap(t.Space(.5)).Padding(t.Space(1.5), t.Space(2)).
					Radius(6).Background(tokens.Elevated).Border(1, tokens.Border).Children(func() {
					ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(t.Space(1)).Children(func() {
						ui.Text(c, eventTypeLabel(event.Type)).FontSize(t.Rem(.7)).Bold().TextColor(t.Text)
						if event.CheckAck {
							ui.Row(c).Padding(1, 5).Radius(4).Background(tokens.SuccessBg).Children(func() {
								ui.Text(c, "检查确认").FontSize(t.Rem(.62)).TextColor(tokens.SuccessText)
							})
						}
						ui.Spacer(c)
						when := event.RecordedAt
						if when == "" {
							when = event.OccurredAt
						}
						ui.Text(c, when).FontSize(t.Rem(.64)).TextColor(t.TextMuted)
					})
					ui.Text(c, event.Content).FontSize(t.Rem(.76)).TextColor(t.Text)
					if event.Deviation != "" {
						ui.Text(c, "偏差："+event.Deviation).FontSize(t.Rem(.72)).TextColor(tokens.WarningText)
					}
					if len(event.Checks) > 0 {
						ui.Row(c).Wrap().Gap(t.Space(1)).Children(func() {
							for _, check := range event.Checks {
								ui.Row(c).Padding(1, 5).Radius(4).Background(tokens.NeutralBg).Children(func() {
									ui.Text(c, fmt.Sprintf("%s · %s", check.CheckID, check.Status)).
										FontSize(t.Rem(.64)).TextColor(tokens.NeutralText)
								})
							}
						})
					}
					if len(event.Materials) > 0 {
						ui.Row(c).Wrap().Gap(t.Space(1)).Children(func() {
							for _, material := range event.Materials {
								ui.Text(c, fmt.Sprintf("%s %.3g %s", material.Material, material.Amount, material.Unit)).
									FontSize(t.Rem(.66)).TextColor(t.TextMuted)
							}
						})
					}
					if len(event.Parameters) > 0 {
						ui.Row(c).Wrap().Gap(t.Space(1)).Children(func() {
							for _, parameter := range event.Parameters {
								ui.Text(c, strings.TrimSpace(fmt.Sprintf("%s %s %s", parameter.Key, parameter.Value, parameter.Unit))).
									FontSize(t.Rem(.64)).TextColor(t.TextMuted)
							}
						})
					}
					if len(event.Lineage) > 0 {
						for _, edge := range event.Lineage {
							ui.Text(c, fmt.Sprintf("%s:%s → %s:%s (%s)", edge.FromKind, edge.FromID, edge.ToKind, edge.ToID, edge.Relation)).
								FontSize(t.Rem(.64)).TextColor(t.TextMuted)
						}
					}
					if len(event.Measurements) > 0 {
						for _, measurement := range event.Measurements {
							ui.Text(c, fmt.Sprintf("%s · %s %s %s", fallback(measurement.SampleID, "未分配"),
								measurement.Field, measurement.Value, measurement.Unit)).
								FontSize(t.Rem(.66)).TextColor(t.TextMuted)
						}
					}
				})
			}
		})
	})
	s.ShowHistory = open
}

func eventTypeLabel(kind string) string {
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
	case "skipped_by_rule":
		return "按规则跳过"
	}
	return kind
}

// unassociatedRecords renders run-level records that cannot be tied to a
// frozen task. They are never guessed into a task.
func unassociatedRecords(c *ui.Context, s *State, t *ui.Theme, tokens theme.Tokens) {
	if s.VM == nil || (len(s.VM.UnassociatedMeasurements) == 0 && len(s.VM.UnassociatedEvents) == 0) {
		return
	}
	ui.Column(c).FillWidth().Padding(t.Space(2)).Radius(float32(tokens.PanelRadius)).
		Background(tokens.WarningBg).Border(1, tokens.WarningText).Gap(t.Space(1.5)).Children(func() {
		ui.Text(c, "未关联记录").FontSize(t.Rem(.85)).Bold().TextColor(tokens.WarningText)
		ui.Text(c, "这些记录缺少可确认的 task key 或 protocol step path，未归入任何步骤。").
			FontSize(t.Rem(.7)).TextColor(tokens.WarningText)
		for _, measurement := range s.VM.UnassociatedMeasurements {
			ui.Text(c, fmt.Sprintf("%s · %s %s %s", fallback(measurement.SampleID, "未分配"),
				measurement.Field, measurement.Value, measurement.Unit)).
				FontSize(t.Rem(.72)).TextColor(t.Text)
		}
		for _, event := range s.VM.UnassociatedEvents {
			ui.Text(c, fmt.Sprintf("%s · %s", eventTypeLabel(event.Type), event.Content)).
				FontSize(t.Rem(.72)).TextColor(t.Text)
		}
	})
}

// historyTab is the read-only history tab: it lists VM.TaskByKey[selected].Events
// (same source as the task events) and opens the original plate for events that
// carry a BatchID.
func historyTab(c *ui.Context, s *State, t *ui.Theme, tokens theme.Tokens, task *RunTaskViewModel) {
	// The shared projection renders the audit head (planned-vs-actual and
	// capture reconciliation); the entry list below stays the notebook.Build
	// domain and is never re-derived here.
	proj, tasks := s.historyRecord()
	if len(task.Events) == 0 && proj == nil {
		ui.Text(c, "尚无执行事件。").FontSize(t.Rem(.75)).TextColor(t.TextMuted)
		return
	}
	ui.Column(c).FillWidth().Gap(t.Space(2)).Children(func() {
		if proj != nil {
			renderHistoryRecord(c, s, proj, tasks, t, tokens)
		}
		for _, event := range task.Events {
			ui.Column(c).FillWidth().Gap(t.Space(.5)).Padding(t.Space(1.5), t.Space(2)).
				Radius(6).Background(tokens.Elevated).Border(1, tokens.Border).Children(func() {
				ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(t.Space(1)).Children(func() {
					ui.Text(c, eventTypeLabel(event.Type)).FontSize(t.Rem(.7)).Bold().TextColor(t.Text)
					ui.Spacer(c)
					when := event.RecordedAt
					if when == "" {
						when = event.OccurredAt
					}
					ui.Text(c, when).FontSize(t.Rem(.64)).TextColor(t.TextMuted)
				})
				ui.Text(c, event.Content).FontSize(t.Rem(.76)).TextColor(t.Text)
				if event.BatchID != "" {
					ui.Text(c, "Batch "+event.BatchID).FontSize(t.Rem(.64)).TextColor(t.TextMuted)
					if s.Callbacks.OpenBatch != nil {
						btn := components.Button(c, "打开原始孔板", components.ButtonGhost)
						if btn.Clicked() {
							s.Callbacks.OpenBatch(event.BatchID)
						}
					}
				}
			})
		}
	})
}

// timelineTab renders the Run's immutable Core events in recorded chronology (UX-LAYOUT).
func timelineTab(c *ui.Context, s *State, t *ui.Theme, tokens theme.Tokens) {
	events := BuildTimeline(s.VM)
	if len(events) == 0 {
		ui.Text(c, "尚无执行事件。").FontSize(t.Rem(.75)).TextColor(t.TextMuted)
		return
	}
	ui.Column(c).FillWidth().Gap(t.Space(1.5)).Children(func() {
		for _, event := range events {
			event := event
			background := tokens.Elevated
			if event.TaskKey == s.SelectedTimelineTask {
				background = t.Selection
			}
			ui.Column(c).FillWidth().Gap(t.Space(.75)).Padding(t.Space(1.5), t.Space(2)).
				Background(background).Border(1, tokens.Border).Children(func() {
				ui.Row(c).FillWidth().Wrap().AlignItems(ui.Center).Gap(t.Space(1)).Children(func() {
					title := eventTypeLabel(event.Type)
					if title == "" {
						title = "执行事件"
					}
					ui.Text(c, title).FontSize(t.Rem(.72)).Bold().TextColor(t.Text)
					if task := s.VM.TaskByKey[event.TaskKey]; task != nil && task.StepTitle != "" {
						ui.Text(c, task.StepTitle).FontSize(t.Rem(.68)).TextColor(t.TextMuted)
					}
					if event.Badge != "" {
						bg, fg := tokens.NeutralBg, tokens.NeutralText
						switch event.Badge {
						case "已确认", "已完成", "测量":
							bg, fg = tokens.SuccessBg, tokens.SuccessText
						case "失败":
							bg, fg = tokens.DangerBg, tokens.DangerText
						case "重试":
							bg, fg = tokens.WarningBg, tokens.WarningText
						case "偏差":
							bg, fg = tokens.WarningBg, tokens.WarningText
						}
						ui.Row(c).Padding(1, 6).Radius(4).Background(bg).Children(func() {
							ui.Text(c, event.Badge).FontSize(t.Rem(.64)).Bold().TextColor(fg)
						})
					}
					if event.Attempt > 1 {
						ui.Text(c, fmt.Sprintf("第 %d 次", event.Attempt)).
							FontSize(t.Rem(.64)).TextColor(t.TextMuted)
					}
					ui.Spacer(c)
					when := timelineTime(event)
					ui.Text(c, when).FontSize(t.Rem(.64)).TextColor(t.TextMuted)
				})
				if event.Content != "" {
					ui.Text(c, event.Content).FontSize(t.Rem(.74)).TextColor(t.Text)
				}
				if len(event.Checks) > 0 {
					ui.Row(c).Wrap().Gap(t.Space(1)).Children(func() {
						for _, check := range event.Checks {
							label, fg := timelineCheckStatus(check.Status), tokens.NeutralText
							bg := tokens.NeutralBg
							switch check.Status {
							case "passed":
								bg, fg = tokens.SuccessBg, tokens.SuccessText
							case "failed", "blocked":
								bg, fg = tokens.DangerBg, tokens.DangerText
							case "skipped":
								bg, fg = tokens.WarningBg, tokens.WarningText
							}
							ui.Row(c).Padding(1, 6).Radius(4).Background(bg).Children(func() {
								ui.Text(c, "检查 · "+label).FontSize(t.Rem(.64)).TextColor(fg)
							})
						}
					})
				}
				for _, measurement := range event.Measurements {
					ui.Text(c, fmt.Sprintf("%s · %s %s %s", fallback(measurement.SampleID, "未分配"),
						measurement.Field, measurement.Value, measurement.Unit)).
						FontSize(t.Rem(.68)).TextColor(t.TextMuted)
				}
				if event.Deviation != "" {
					ui.Text(c, "偏差："+event.Deviation).FontSize(t.Rem(.68)).TextColor(tokens.WarningText)
				}
				if event.StepStatus == "completed" {
					ui.Text(c, "步骤已完成").FontSize(t.Rem(.68)).TextColor(tokens.SuccessText)
				} else if event.StepStatus == "blocked" {
					ui.Text(c, "步骤阻塞").FontSize(t.Rem(.68)).TextColor(tokens.DangerText)
				} else if event.StepStatus == "skipped" {
					ui.Text(c, "步骤已跳过").FontSize(t.Rem(.68)).TextColor(tokens.WarningText)
				}
			})
		}
	})
}

// HydrateReadOnlyAsterDocView builds a read-only asterdoc.View hydrated with
// Run actual parameters, sample exclusions, calculator substitutions, checklist
// ticks, persisted data_table readouts, and linked assets so Notebook surfaces
// reuse the exact same module widgets as Design/Run.
func HydrateReadOnlyAsterDocView(
	vm *RunWorkspaceViewModel,
	decodedReadouts []record.DecodedReadout,
	taskAssets func(taskKey string) []LinkedAsset,
	resolveAsset func(LinkedAsset) string,
) *asterdoc.View {
	if vm == nil || vm.FrozenDoc == nil {
		return nil
	}
	v := &asterdoc.View{
		Doc:            vm.FrozenDoc,
		Registry:       asterdoc.BuiltinRegistry(),
		Mode:           asterdoc.ModeRun,
		Density:        asterdoc.DensityCompact,
		RunID:          vm.RunID,
		ReadOnly:       true,
		HideStepHeader: true,
	}
	populateAsterViewState(v, vm, nil, decodedReadouts, taskAssets, resolveAsset)
	return v
}

func collectAllDataTableBlocks(doc *asterdoc.Document) []map[string]any {
	if doc == nil {
		return nil
	}
	var out []map[string]any
	for _, m := range doc.Modules {
		if m.Kind == "data_table" {
			out = append(out, m.Data)
		} else if m.Kind == "step" {
			if rawBlocks, ok := m.Data["blocks"].([]any); ok {
				for _, rb := range rawBlocks {
					if bm, _ := rb.(map[string]any); bm != nil && textValueFromAny(bm["kind"]) == "data_table" {
						out = append(out, bm)
					}
				}
			}
		}
	}
	return out
}

func allVMTasks(vm *RunWorkspaceViewModel) []*RunTaskViewModel {
	if vm == nil {
		return nil
	}
	var out []*RunTaskViewModel
	for _, st := range vm.Steps {
		if st == nil {
			continue
		}
		out = append(out, st)
		for _, ch := range st.Children {
			if ch != nil {
				out = append(out, ch)
			}
		}
	}
	return out
}

func matchTableForBlock(bid, schemaKey string, tables map[string]PersistedTable) (PersistedTable, bool) {
	if tbl, ok := tables[schemaKey]; ok {
		return tbl, true
	}
	if tbl, ok := tables[schemaKey+".table"]; ok {
		return tbl, true
	}
	for k, candidate := range tables {
		if strings.Contains(k, strings.TrimPrefix(strings.ToLower(bid), "dt-")) {
			return candidate, true
		}
	}
	return PersistedTable{}, false
}

func populateAsterViewState(
	v *asterdoc.View,
	vm *RunWorkspaceViewModel,
	targetTask *RunTaskViewModel,
	decodedReadouts []record.DecodedReadout,
	taskAssets func(string) []LinkedAsset,
	resolveAsset func(LinkedAsset) string,
) {
	if v == nil || vm == nil {
		return
	}
	actualParams := map[string]string{}
	for k, val := range vm.ActualParamNumbers {
		disp := strconv.FormatFloat(val, 'g', -1, 64)
		if u := vm.ActualParamUnits[k]; u != "" {
			disp += " " + u
		}
		actualParams[k] = disp
	}
	tasks := allVMTasks(vm)
	if targetTask != nil {
		tasks = []*RunTaskViewModel{targetTask}
	}
	for _, tk := range tasks {
		if tk.Actual != nil {
			for k, val := range tk.Actual.Parameters {
				actualParams[k] = val
			}
		}
	}
	v.ActualParams = actualParams

	if len(vm.SampleStatuses) > 0 {
		if v.SampleStatuses == nil {
			v.SampleStatuses = map[string]string{}
		}
		for sid, st := range vm.SampleStatuses {
			v.SampleStatuses[sid] = st
		}
	}

	calcCtx := &asterdoc.CalcActualContext{
		Params:     map[string]float64{},
		ParamUnits: map[string]string{},
	}
	for k, n := range vm.ActualParamNumbers {
		calcCtx.Params[k] = n
	}
	for k, u := range vm.ActualParamUnits {
		calcCtx.ParamUnits[k] = u
	}
	if len(vm.SampleMeasurements) > 0 {
		sids := make([]string, 0, len(vm.SampleMeasurements))
		for sid := range vm.SampleMeasurements {
			st := vm.SampleStatuses[sid]
			if st == "QC剔除" || st == "删失" {
				continue
			}
			sids = append(sids, sid)
		}
		sort.Strings(sids)
		for _, sid := range sids {
			eps := vm.SampleMeasurements[sid]
			epKeys := make([]string, 0, len(eps))
			for ep := range eps {
				epKeys = append(epKeys, ep)
			}
			sort.Strings(epKeys)
			for _, ep := range epKeys {
				mv := eps[ep]
				if num, err := strconv.ParseFloat(strings.TrimSpace(mv.Value), 64); err == nil {
					calcCtx.Samples = append(calcCtx.Samples, asterdoc.CalcSampleInput{
						SampleID: sid,
						Value:    num,
						Unit:     mv.Unit,
						Endpoint: mv.Field,
					})
				}
			}
		}
	}
	v.CalcActuals = calcCtx

	// Hydrate data_table cells from task actual tables and decoded readouts.
	dtBlocks := collectAllDataTableBlocks(vm.FrozenDoc)
	combinedTables := map[string]PersistedTable{}
	for _, tk := range tasks {
		if tk.Actual == nil {
			continue
		}
		for k, tbl := range tk.Actual.Tables {
			combinedTables[k] = tbl
		}
	}
	for _, dr := range decodedReadouts {
		if dr.Kind != "table" || len(dr.Columns) == 0 || len(dr.Rows) == 0 {
			continue
		}
		anyRows := make([][]any, 0, len(dr.Rows))
		for _, r := range dr.Rows {
			rowAny := make([]any, len(r))
			for i, cell := range r {
				rowAny[i] = cell
			}
			anyRows = append(anyRows, rowAny)
		}
		combinedTables[dr.SchemaID] = PersistedTable{SchemaID: dr.SchemaID, Columns: dr.Columns, Rows: anyRows}
	}
	if len(combinedTables) > 0 {
		for _, bm := range dtBlocks {
			bid := textValueFromAny(bm["id"])
			schemaKey := textValueFromAny(bm["schema_id"])
			if schemaKey == "" {
				schemaKey = "data_table." + strings.ToLower(strings.ReplaceAll(bid, "_", "-")) + ".v1"
			}
			tbl, ok := matchTableForBlock(bid, schemaKey, combinedTables)
			if !ok {
				continue
			}
			if v.TableCells == nil {
				v.TableCells = map[string]map[string]string{}
			}
			if v.TableCells[bid] == nil {
				v.TableCells[bid] = map[string]string{}
			}
			for rIdx, row := range tbl.Rows {
				for cIdx, ckey := range tbl.Columns {
					if cIdx >= len(row) || row[cIdx] == nil {
						continue
					}
					ck := fmt.Sprintf("dt:%s:%d:%s", bid, rIdx, ckey)
					if _, exists := v.TableCells[bid][ck]; !exists {
						v.TableCells[bid][ck] = fmt.Sprint(row[cIdx])
					}
				}
			}
		}
	}

	checked := map[string]bool{}
	skipped := map[string]string{}
	for _, tk := range tasks {
		for _, chk := range tk.Checks {
			if chk.Status == "passed" {
				checked[chk.CheckID] = true
				checked[tk.StepID+":"+chk.CheckID] = true
			} else if chk.Status == "skipped" {
				skipped[chk.CheckID] = "已跳过"
			}
		}
	}
	v.CheckedItems = checked
	v.SkippedItems = skipped

	if taskAssets != nil {
		grouped := map[string][]asterdoc.AssetThumb{}
		var allThumbs []asterdoc.AssetThumb
		for _, tk := range tasks {
			rawAssets := taskAssets(tk.TaskKey)
			thumbs := make([]asterdoc.AssetThumb, 0, len(rawAssets))
			for _, a := range rawAssets {
				lbl := a.RelativePath
				if lbl == "" {
					lbl = a.AssetID
				}
				filePath := a.RelativePath
				if resolveAsset != nil {
					if resolved := resolveAsset(a); resolved != "" {
						filePath = resolved
					}
				}
				th := asterdoc.AssetThumb{
					AssetID:  a.AssetID,
					Label:    lbl,
					FilePath: filePath,
					WellID:   a.WellID,
					FovIndex: int(a.FOVIndex),
				}
				thumbs = append(thumbs, th)
				allThumbs = append(allThumbs, th)
				if a.WellID != "" {
					grouped["well:"+a.WellID] = append(grouped["well:"+a.WellID], th)
				}
			}
			if len(thumbs) > 0 {
				grouped[tk.StepID] = append(grouped[tk.StepID], thumbs...)
			}
		}
		if len(allThumbs) > 0 {
			grouped[""] = allThumbs
			v.LinkedAssets = grouped
		}
	}
}

func renderFrozenStepBlocks(c *ui.Context, s *State, task *RunTaskViewModel, t *ui.Theme, tokens theme.Tokens) {
	if s == nil || s.VM == nil || s.VM.FrozenDoc == nil || task == nil || task.StepID == "" {
		return
	}
	var stepMod *asterdoc.Module
	for i := range s.VM.FrozenDoc.Modules {
		if s.VM.FrozenDoc.Modules[i].Kind == "step" && s.VM.FrozenDoc.Modules[i].ID == task.StepID {
			stepMod = &s.VM.FrozenDoc.Modules[i]
			break
		}
	}
	if stepMod == nil {
		return
	}
	blocks, _ := stepMod.Data["blocks"].([]any)
	refs, _ := stepMod.Data["block_refs"].([]any)
	if len(blocks) == 0 && len(refs) == 0 {
		return
	}
	if s.StepViews == nil {
		s.StepViews = map[string]*asterdoc.View{}
	}
	v := s.StepViews[task.TaskKey]
	if v == nil {
		v = &asterdoc.View{}
		s.StepViews[task.TaskKey] = v
	}
	v.Doc = s.VM.FrozenDoc
	v.Mode = asterdoc.ModeRun
	v.Density = asterdoc.DensityFull
	v.RunID = s.VM.RunID
	v.StepID = task.StepID
	v.HideStepHeader = true
	v.AsyncDecode = s.Callbacks.AsyncDecode
	v.OnAsyncInvalidate = s.Callbacks.OnAsyncInvalidate
	v.ReadOnly = task.Status == TaskStatusCompleted || task.Status == TaskStatusSkipped
	v.Allowed = map[string]bool{
		asterdoc.PermRunReadout: s.Callbacks.ExecuteCommand != nil,
	}

	actualParams := map[string]string{}
	for k, val := range s.VM.ActualParamNumbers {
		disp := strconv.FormatFloat(val, 'g', -1, 64)
		if u := s.VM.ActualParamUnits[k]; u != "" {
			disp += " " + u
		}
		actualParams[k] = disp
	}
	if task.Actual != nil {
		for k, val := range task.Actual.Parameters {
			actualParams[k] = val
		}
	}
	v.ActualParams = actualParams

	if len(s.VM.SampleStatuses) > 0 {
		if v.SampleStatuses == nil {
			v.SampleStatuses = map[string]string{}
		}
		for sid, st := range s.VM.SampleStatuses {
			v.SampleStatuses[sid] = st
		}
	}

	// Build CalcActualContext from upstream Run parameters and live non-excluded sample measurements
	calcCtx := &asterdoc.CalcActualContext{
		Params:     map[string]float64{},
		ParamUnits: map[string]string{},
	}
	for k, n := range s.VM.ActualParamNumbers {
		calcCtx.Params[k] = n
	}
	for k, u := range s.VM.ActualParamUnits {
		calcCtx.ParamUnits[k] = u
	}
	if len(s.VM.SampleMeasurements) > 0 {
		sids := make([]string, 0, len(s.VM.SampleMeasurements))
		for sid := range s.VM.SampleMeasurements {
			st := s.VM.SampleStatuses[sid]
			if st == "QC剔除" || st == "删失" {
				continue
			}
			sids = append(sids, sid)
		}
		sort.Strings(sids)
		for _, sid := range sids {
			eps := s.VM.SampleMeasurements[sid]
			epKeys := make([]string, 0, len(eps))
			for ep := range eps {
				epKeys = append(epKeys, ep)
			}
			sort.Strings(epKeys)
			for _, ep := range epKeys {
				mv := eps[ep]
				if num, err := strconv.ParseFloat(strings.TrimSpace(mv.Value), 64); err == nil {
					calcCtx.Samples = append(calcCtx.Samples, asterdoc.CalcSampleInput{
						SampleID: sid,
						Value:    num,
						Unit:     mv.Unit,
						Endpoint: mv.Field,
					})
				}
			}
		}
	}
	v.CalcActuals = calcCtx

	// Hydrate persisted Core data_table readouts into TableCells (non-destructive to unsubmitted edits)
	if task.Actual != nil && len(task.Actual.Tables) > 0 {
		for _, raw := range blocks {
			bm, _ := raw.(map[string]any)
			if textValueFromAny(bm["kind"]) != "data_table" {
				continue
			}
			bid := textValueFromAny(bm["id"])
			schemaKey := textValueFromAny(bm["schema_id"])
			if schemaKey == "" {
				schemaKey = "data_table." + strings.ToLower(strings.ReplaceAll(bid, "_", "-")) + ".v1"
			}
			tbl, ok := task.Actual.Tables[schemaKey]
			if !ok {
				tbl, ok = task.Actual.Tables[schemaKey+".table"]
			}
			if !ok && len(task.Actual.Tables) > 0 {
				// Also try hyphen-normalized schema id from widget_tables
				for k, candidate := range task.Actual.Tables {
					if strings.Contains(k, strings.TrimPrefix(strings.ToLower(bid), "dt-")) {
						tbl, ok = candidate, true
						break
					}
				}
			}
			if !ok {
				continue
			}
			if v.TableCells == nil {
				v.TableCells = map[string]map[string]string{}
			}
			if v.TableCells[bid] == nil {
				v.TableCells[bid] = map[string]string{}
			}
			for rIdx, row := range tbl.Rows {
				for cIdx, ckey := range tbl.Columns {
					if cIdx >= len(row) || row[cIdx] == nil {
						continue
					}
					ck := fmt.Sprintf("dt:%s:%d:%s", bid, rIdx, ckey)
					if _, exists := v.TableCells[bid][ck]; !exists {
						v.TableCells[bid][ck] = fmt.Sprint(row[cIdx])
					}
				}
			}
		}
	}

	checked := map[string]bool{}
	skipped := map[string]string{}
	for _, chk := range task.Checks {
		if chk.Status == "passed" || *s.checkOn(task.TaskKey, chk.CheckID) {
			checked[chk.CheckID] = true
			checked[task.StepID+":"+chk.CheckID] = true
		} else if chk.Status == "skipped" {
			skipped[chk.CheckID] = "已跳过"
		}
	}
	for fullKey, ptr := range s.Checks {
		if ptr != nil && *ptr && strings.HasPrefix(fullKey, task.TaskKey+"::") {
			cid := strings.TrimPrefix(fullKey, task.TaskKey+"::")
			checked[cid] = true
			checked[task.StepID+":"+cid] = true
		}
	}
	v.CheckedItems = checked
	v.SkippedItems = skipped

	timers := map[string]asterdoc.TimerRuntimeState{}
	for _, rec := range s.bench().Timers[task.TaskKey] {
		plannedS := 0
		if rec.PlannedS != nil {
			plannedS = int(*rec.PlannedS)
		}
		accumulatedS := int(rec.AccumulatedS)
		remainingS := 0
		if plannedS > accumulatedS {
			remainingS = plannedS - accumulatedS
		}
		status := rec.Status
		if status == "" || status == "ready" {
			status = "idle"
		}
		ts := asterdoc.TimerRuntimeState{
			TimerID:      rec.TimerID,
			Status:       status,
			PlannedS:     plannedS,
			RemainingS:   remainingS,
			AccumulatedS: accumulatedS,
			PhaseIndex:   int(rec.PhaseIndex),
			RoundIndex:   int(rec.RoundIndex),
		}
		timers[rec.TimerID] = ts
		if idx := strings.LastIndex(rec.TimerID, "@"); idx >= 0 {
			timers[rec.TimerID[idx+1:]] = ts
		}
	}
	v.TimerStates = timers

	if s.Callbacks.TaskAssets != nil {
		rawAssets := s.Callbacks.TaskAssets(task.TaskKey)
		thumbs := make([]asterdoc.AssetThumb, 0, len(rawAssets))
		grouped := map[string][]asterdoc.AssetThumb{}
		assetByID := map[string]LinkedAsset{}
		for _, a := range rawAssets {
			assetByID[a.AssetID] = a
			lbl := a.RelativePath
			if lbl == "" {
				lbl = a.AssetID
			}
			filePath := a.RelativePath
			if s.Callbacks.ResolveAssetPath != nil {
				if resolved := s.Callbacks.ResolveAssetPath(a); resolved != "" {
					filePath = resolved
				}
			}
			th := asterdoc.AssetThumb{
				AssetID:  a.AssetID,
				Label:    lbl,
				FilePath: filePath,
				WellID:   a.WellID,
				FovIndex: int(a.FOVIndex),
			}
			thumbs = append(thumbs, th)
			if a.WellID != "" {
				grouped["well:"+a.WellID] = append(grouped["well:"+a.WellID], th)
			}
		}
		grouped[task.StepID] = thumbs
		v.LinkedAssets = grouped
		if s.Callbacks.OpenAsset != nil {
			v.OnOpenExternalAsset = func(th asterdoc.AssetThumb) {
				if la, ok := assetByID[th.AssetID]; ok {
					s.Callbacks.OpenAsset(task.TaskKey, la)
				}
			}
		}
	} else {
		v.LinkedAssets = nil
		v.OnOpenExternalAsset = nil
	}

	v.OnToggleCheck = func(stepID, checkID string, on bool) {
		*s.checkOn(task.TaskKey, checkID) = on
		inTaskChecks := false
		for _, chk := range task.Checks {
			if chk.CheckID == checkID {
				inTaskChecks = true
				break
			}
		}
		if inTaskChecks && s.Callbacks.SaveChecks != nil {
			s.saveChecks(task)
			return
		}
		if s.Callbacks.AppendEvent != nil {
			stepPath, assayID, pvID, tp := eventScope(task)
			ev := runs.ExecutionEventInput{
				EventType: "observation",
				Content:   fmt.Sprintf("检查项 %s：%v", checkID, on),
				Timepoint: tp,
			}
			if assayID != "" {
				ev.StepPath, ev.AssayID, ev.ProtocolVersionID = stepPath, assayID, pvID
			}
			_, _ = s.Callbacks.AppendEvent(task.TaskKey, ev)
		}
	}
	v.OnRuleSkip = func(stepID, checkID, rule string) {
		if s.Callbacks.AppendEvent != nil {
			_, _ = s.Callbacks.AppendEvent(task.TaskKey, SkipEventInput(task, rule))
		}
	}
	v.OnRecordActualParam = func(stepID, blockID, key, value, unit string) {
		if task.Actual == nil {
			task.Actual = &ActualRecord{Parameters: map[string]string{}}
		}
		if task.Actual.Parameters == nil {
			task.Actual.Parameters = map[string]string{}
		}
		disp := value
		if unit != "" {
			disp += " " + unit
		}
		task.Actual.Parameters[stepID+":"+blockID+":"+key] = disp
		task.Actual.Parameters[key] = disp
		if n, err := strconv.ParseFloat(strings.TrimSpace(value), 64); err == nil {
			if s.VM.ActualParamNumbers == nil {
				s.VM.ActualParamNumbers = map[string]float64{}
			}
			if s.VM.ActualParamUnits == nil {
				s.VM.ActualParamUnits = map[string]string{}
			}
			s.VM.ActualParamNumbers[key] = n
			if unit != "" {
				s.VM.ActualParamUnits[key] = unit
			}
		}
		if s.Callbacks.AppendEvent != nil {
			valBytes, _ := json.Marshal(value)
			if n, err := strconv.ParseFloat(strings.TrimSpace(value), 64); err == nil {
				valBytes, _ = json.Marshal(n)
			}
			stepPath, assayID, pvID, tp := eventScope(task)
			ev := runs.ExecutionEventInput{
				EventType: "observation",
				Content:   fmt.Sprintf("实测参数 %s = %s", key, disp),
				ActualParameters: []runs.EventActualParameter{{
					Key:       key,
					ValueJSON: string(valBytes),
					Unit:      unit,
					Source:    "step_param_table",
				}},
				Timepoint: tp,
			}
			if assayID != "" {
				ev.StepPath, ev.AssayID, ev.ProtocolVersionID = stepPath, assayID, pvID
			}
			_, _ = s.Callbacks.AppendEvent(task.TaskKey, ev)
		}
	}
	v.OnTimerAction = func(stepID, blockID, action string) {
		recs := s.bench().Timers[task.TaskKey]
		var target *runs.TimerRecord
		for i := range recs {
			if recs[i].TimerID == blockID || strings.HasSuffix(recs[i].TimerID, "@"+blockID) {
				target = &recs[i]
				break
			}
		}
		if target == nil && len(recs) == 1 {
			target = &recs[0]
		}
		if target == nil && action == "start" && s.Callbacks.SaveTimer != nil {
			plannedS := uint64(300)
			mode := "countdown"
			for _, raw := range blocks {
				if bm, _ := raw.(map[string]any); bm != nil && textValueFromAny(bm["id"]) == blockID {
					if m := textValueFromAny(bm["mode"]); m == "countdown" || m == "stopwatch" || m == "window" {
						mode = m
					}
					if n := toPositiveInt(bm["duration_s"]); n > 0 {
						plannedS = uint64(n)
					} else if n := toPositiveInt(bm["planned_s"]); n > 0 {
						plannedS = uint64(n)
					}
				}
			}
			timerID := fmt.Sprintf("%s/%s/%d@%s", task.TaskKey, mode, len(recs)+1, strings.ReplaceAll(blockID, "/", "-"))
			rec := runs.TimerRecord{TimerID: timerID, TaskKey: task.TaskKey, Mode: mode, Status: "ready"}
			if mode != "stopwatch" {
				rec.PlannedS = &plannedS
			}
			saved, err := s.Callbacks.SaveTimer(rec)
			if err != nil {
				s.bench().TimersErr[task.TaskKey] = timerActionError(err)
				return
			}
			target = &saved
		}
		if target != nil && s.Callbacks.TimerAction != nil {
			var phases []runs.TimerPhase
			if s.Callbacks.TimerPhases != nil {
				phases = s.Callbacks.TimerPhases(task.TaskKey)
			}
			s.runTimerAction(task, *target, phases, action)
		}
	}
	v.OnSubmitTableReadout = func(stepID, blockID string, payload map[string]any) error {
		if s.Callbacks.ExecuteCommand == nil {
			return fmt.Errorf("记录保存服务暂未就绪")
		}
		if payload == nil {
			return fmt.Errorf("表格数据为空")
		}
		schemaID, _ := payload["schema_id"].(string)
		schemaDefJSON, _ := payload["schema_def_json"].(string)
		cols, _ := payload["columns"].([]string)
		matrix, _ := payload["rows"].([][]any)
		if len(cols) == 0 || len(matrix) == 0 {
			return fmt.Errorf("表格不能为空")
		}
		stepPath, assayID, pvID, tp := eventScope(task)
		cmd, err := buildTableCommand(s.VM.RunID, stepPath, pvID, assayID, tp, schemaID, schemaDefJSON, "", cols, matrix, newIdempotencyKey())
		if err != nil {
			return err
		}
		if _, err := s.Callbacks.ExecuteCommand(cmd); err != nil {
			return fmt.Errorf("%s", channelError(err))
		}
		if task.Actual != nil {
			if task.Actual.Tables == nil {
				task.Actual.Tables = map[string]PersistedTable{}
			}
			task.Actual.Tables[schemaID] = PersistedTable{SchemaID: schemaID, Columns: cols, Rows: matrix}
		}
		return nil
	}
	v.OnExcludeSample = func(sampleID, status string) {
		if s.Callbacks.ExecuteCommand == nil {
			return
		}
		cmd, err := buildExclusionCommand(s.VM.RunID, status, []string{sampleID}, "", task.StepPath, task.AssayID, task.ProtocolVersionID, task.Timepoint, s.bench().Blind, newIdempotencyKey())
		if err != nil {
			return
		}
		_, _ = s.Callbacks.ExecuteCommand(cmd)
	}
	v.OnLinkAsset = func(stepID, blockID, wellID string) {
		scope := ""
		for _, raw := range blocks {
			if bm, _ := raw.(map[string]any); bm != nil && textValueFromAny(bm["id"]) == blockID {
				scope = textValueFromAny(bm["scope"])
				break
			}
		}
		if scope == "container_grid" && s.Callbacks.CaptureWells != nil && s.Callbacks.RequestBatchLink != nil {
			if wells, fov := s.Callbacks.CaptureWells(task.TaskKey); len(wells) > 0 && fov > 0 {
				s.Callbacks.RequestBatchLink(task.TaskKey)
				return
			}
		}
		if s.Callbacks.RequestStepAssetLink != nil {
			s.Callbacks.RequestStepAssetLink(task.TaskKey, wellID)
			return
		}
		if s.Callbacks.RequestBatchLink != nil {
			s.Callbacks.RequestBatchLink(task.TaskKey)
		}
	}

	ui.Column(c).FillWidth().Padding(float32(tokens.Spacing16)).Radius(float32(tokens.PanelRadius)).
		Background(tokens.Surface).Border(1, tokens.Border).Gap(float32(tokens.Spacing12)).Children(func() {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(float32(tokens.Spacing8)).Children(func() {
			ui.Text(c, "本步实验操作与记录").FontSize(float32(tokens.TextSection)).Bold().TextColor(t.Text)
			ui.Spacer(c)
			components.Badge(c, fmt.Sprintf("%d 项模块", len(blocks)+len(refs)), components.BadgeOptions{Tone: components.BadgeAccent})
		})
		asterdoc.StepBlockComposer(v, c, stepMod)
	})
}

func textValueFromAny(v any) string {
	s, _ := v.(string)
	return strings.TrimSpace(s)
}
