package runworkspace

import (
	"fmt"
	"sort"
	"strings"

	"asterlab-mygo-prototype/internal/theme"
	workflowtree "asterlab-mygo-prototype/internal/workflowtree"

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
	SaveChecks func(taskKey string, checks []CheckAckView)
	Complete   func(taskKey string, checks []CheckAckView)
	OpenBatch  func(batchID string)
	// Recording renders the recording module for a task. The host supplies it so
	// the same module serves Design, Run, Notebook and PDF.
	Recording func(c *ui.Context, task *RunTaskViewModel)
}

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

	// ShowHistory toggles the third layer (operation history) for the selected task.
	ShowHistory bool

	// ShowProcedure toggles procedure metadata when rendered below the recording
	// panel (narrow layout); defaults to collapsed.
	ShowProcedure bool
	// WorkspaceTab selects the task workspace (0) or the read-only history tab (1).
	WorkspaceTab int

	// railScrollKey remembers which selection was last scrolled into view.
	railScrollKey string

	Narrow bool

	// RecordingValid reports whether the recording module has unsaved edits.
	RecordingDirty bool
}

// NewState constructs the runtime state from a view model.
func NewState(vm *RunWorkspaceViewModel, cb Callbacks) *State {
	s := &State{VM: vm, Callbacks: cb, Checks: map[string]*bool{}, Collapsed: map[string]bool{}, Expanded: map[string]*bool{}}
	if vm != nil && vm.Selected != nil {
		s.Selected = vm.Selected.TaskKey
	}
	return s
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
		if s.VM.Status == "loading" {
			ui.Row(c).Fill().Center().Children(func() {
				ui.Text(c, "正在加载实验运行数据…").FontSize(14).TextColor(t.TextMuted)
			})
			return
		}
		if len(s.VM.Steps) == 0 {
			ui.Row(c).Fill().Center().Children(func() {
				ui.Column(c).Gap(8).Center().Children(func() {
					ui.Text(c, "此运行没有任务").FontSize(16).Bold().TextColor(t.Text)
					ui.Text(c, "冻结方案中没有可执行步骤，或步骤未绑定检测。").
						FontSize(12).TextColor(t.TextMuted)
				})
			})
			return
		}
		if s.shouldStack(c) {
			// Narrow: flow on top, task below.
			ui.Scroll(c).Grow(1).Children(func() {
				ui.Column(c).FillWidth().Gap(12).Padding(12).Children(func() {
					flowRail(c, s, t, tokens, true)
					taskWorkspace(c, s, t, tokens)
					unassociatedRecords(c, s, t, tokens)
				})
			})
			return
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
						taskWorkspace(c, s, t, tokens)
						unassociatedRecords(c, s, t, tokens)
					})
				})
			})
		})
	})
}

func (s *State) shouldStack(c *ui.Context) bool {
	if s.Narrow {
		return true
	}
	if c != nil {
		if w, _ := c.Size(); w > 0 && w <= 1000 {
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

// runHeader is the compact title bar: run title, status, frozen revision.
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
		ui.Text(c, vm.Title).FontSize(16).Bold().TextColor(t.Text).
			SingleLine().Ellipsis("…").Shrink(1).MaxWidth(420)
		ui.Spacer(c)
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			if vm.Status != "" {
				statusPill(c, vm.Status, tokens)
			}
			ui.Text(c, fmt.Sprintf("Run rev %d · Design rev %d · AsterDoc rev %d",
				vm.Revision, vm.DesignRevision, vm.AsterDocRevision)).FontSize(11).
				TextColor(t.TextMuted)
			ui.Text(c, fmt.Sprintf("%d/%d 步已完成", done, total)).FontSize(11).
				TextColor(t.TextMuted).FontFeatures("tnum")
		})
	})
}

func statusPill(c *ui.Context, status string, tokens theme.Tokens) {
	bg, fg := tokens.NeutralBg, tokens.NeutralText
	switch status {
	case "completed", "done":
		bg, fg = tokens.SuccessBg, tokens.SuccessText
	case "running", "in_progress", "active":
		bg, fg = tokens.WarningBg, tokens.WarningText
	case "blocked", "error":
		bg, fg = tokens.DangerBg, tokens.DangerText
	}
	ui.Row(c).Padding(3, 9).Radius(6).Background(bg).Children(func() {
		ui.Text(c, status).FontSize(11).Bold().TextColor(fg)
	})
}

// flowRail is the vertical execution navigation on the left.
func flowRail(c *ui.Context, s *State, t *ui.Theme, tokens theme.Tokens, inline bool) {
	ui.Column(c).FillWidth().Gap(t.Space(1)).Padding(t.Space(3), t.Space(2)).Children(func() {
		ui.Text(c, "实验流程").FontSize(11).Bold().TextColor(t.TextMuted).
			Padding(0, 0, t.Space(2), 0)
		if len(s.VM.TreeIssues) > 0 {
			ui.Row(c).FillWidth().Padding(6, 8).Radius(6).Background(tokens.WarningBg).
				Border(1, tokens.WarningText).Children(func() {
				ui.Text(c, "层级诊断："+strings.Join(s.VM.TreeIssues, "；")).
					FontSize(t.Rem(.66)).TextColor(tokens.WarningText)
			})
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
	if !node.Composite && len(node.Children) == 1 && node.Children[0].Kind == workflowtree.KindTask {
		treeLeafItem(c, s, t, tokens, node.Title, node.Children[0])
		return
	}
	open := s.treeOpen(node.ID)
	label := node.Title
	if node.Composite {
		label = fmt.Sprintf("%s   %d/%d", node.Title, node.Completed, node.Total)
	}
	item := components.TreeItemWithLeading(c, label, open, s.Selected == node.ID, func(c *ui.Context, size float32) {
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
	item := components.TreeItemWithLeading(c, title, nil, selected, func(c *ui.Context, size float32) {
		ui.Box(c).Size(7, 7).Radius(3.5).Background(treeStatusColor(task.Status, t, tokens))
	}, nil)
	if selected && s.railScrollKey != task.ID {
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
	case "completed":
		return t.Success
	case "current", "in_progress":
		return t.Accent
	case "blocked":
		return t.Danger
	case "unresolved":
		return t.Warning
	}
	return tokens.NeutralText
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
				if step.AssayID != "" {
					if meta != "" {
						meta += " · "
					}
					meta += step.AssayID
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
		if selected && s.railScrollKey != step.TaskKey {
			b.ScrollIntoView(ui.ScrollIntoViewOptions{Block: ui.ScrollCenter})
			s.railScrollKey = step.TaskKey
		}
		if b.Clicked() {
			s.Selected = step.TaskKey
			if s.Callbacks.SelectTask != nil {
				s.Callbacks.SelectTask(step.TaskKey)
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
			label = task.TaskKey
		}
		ui.Text(c, label).FontSize(t.Rem(.78)).TextColor(t.TextMuted).
			SingleLine().Ellipsis("…")
	})
	if selected && s.railScrollKey != task.TaskKey {
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

// taskWorkspace is the right column: task workspace and read-only history tab.
func taskWorkspace(c *ui.Context, s *State, t *ui.Theme, tokens theme.Tokens) {
	task := s.selectedTask()
	if task == nil {
		ui.Text(c, "选择一个步骤查看操作说明").FontSize(13).TextColor(t.TextMuted)
		return
	}
	ui.Column(c).FillWidth().Gap(12).Children(func() {
		tab := &s.WorkspaceTab
		components.Tabs(c, tab, components.TabsHighlight, "任务", "历史")
		if *tab == 1 {
			historyTab(c, s, t, tokens, task)
			return
		}
		if s.shouldStack(c) {
			recordingLayer(c, s, t, tokens, task)
			open := s.ShowProcedure
			components.Collapsible(c, "操作说明与元数据", &open, components.CollapsibleOptions{}, func() {
				procedureLayer(c, s, t, tokens, task)
			})
			s.ShowProcedure = open
		} else {
			ui.Row(c).FillWidth().Gap(16).Children(func() {
				ui.Column(c).Grow(1).MinWidth(0).Children(func() {
					procedureLayer(c, s, t, tokens, task)
				})
				ui.Column(c).Width(420).Shrink(0).Children(func() {
					recordingLayer(c, s, t, tokens, task)
				})
			})
		}
	})
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
		ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(t.Space(2)).Children(func() {
			ui.Text(c, task.StepTitle).FontSize(float32(tokens.TextSection)).FontWeight(600).TextColor(t.Text)
			taskStatusPill(c, task.Status, t, tokens)
			ui.Spacer(c)
			ui.Text(c, "当前步骤").FontSize(t.Rem(.7)).TextColor(t.TextMuted)
		})
		ui.Text(c, instruction).FontSize(t.Rem(.82)).TextColor(t.Text)

		if len(task.Parameters) > 0 {
			ui.Divider(c)
			ui.Text(c, "操作参数").FontSize(t.Rem(.78)).Bold().TextColor(t.TextMuted)
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

		ui.Divider(c)
		frozen := []string{fallback(task.StepPath), fallback(task.AssayID, "未绑定"), fallback(task.Timepoint, "未绑定")}
		if strings.TrimSpace(task.StepID) != "" {
			frozen = append(frozen, "步骤 ID "+task.StepID)
		}
		ui.Text(c, "冻结信息（只读） · "+strings.Join(frozen, " · ")).
			FontSize(t.Rem(.68)).TextColor(t.TextMuted).SingleLine().Ellipsis("…")
	})
}

func statChip(c *ui.Context, label string, t *ui.Theme, tokens theme.Tokens) {
	ui.Row(c).Padding(2, 7).Radius(5).Background(tokens.Elevated).
		Border(1, tokens.Border).Children(func() {
		ui.Text(c, label).FontSize(t.Rem(.7)).TextColor(t.TextMuted)
	})
}

func taskStatusPill(c *ui.Context, status TaskStatus, t *ui.Theme, tokens theme.Tokens) {
	label, bg, fg := "未开始", tokens.NeutralBg, tokens.NeutralText
	switch status {
	case TaskStatusCurrent:
		label, bg, fg = "进行中", tokens.WarningBg, tokens.WarningText
	case TaskStatusCompleted:
		label, bg, fg = "已完成", tokens.SuccessBg, tokens.SuccessText
	case TaskStatusBlocked:
		label, bg, fg = "已阻塞", tokens.DangerBg, tokens.DangerText
	}
	ui.Row(c).Padding(2, 7).Radius(5).Background(bg).Children(func() {
		ui.Text(c, label).FontSize(t.Rem(.68)).Bold().TextColor(fg)
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
		if m.ID != "" {
			ui.Text(c, m.ID).FontSize(t.Rem(.66)).TextColor(t.TextMuted)
		}
		if m.Role != "" {
			ui.Text(c, m.Role).FontSize(t.Rem(.7)).TextColor(t.TextMuted)
		}
	})
}

func moduleLabel(kind, title string) string {
	if title != "" {
		return title
	}
	switch kind {
	case "instrument_profile":
		return "仪器能力档案"
	case "dilution":
		return "稀释计算"
	case "plate":
		return "孔板"
	case "measurement_profile":
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
			s.Callbacks.SaveChecks(task.TaskKey, s.acks(task))
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
			ui.Text(c, "实验记录").FontSize(float32(tokens.TextSection)).FontWeight(600).TextColor(t.Text)
			ui.Spacer(c)
			recorded := len(task.Actual.Measurements)
			if recorded > 0 {
				ui.Text(c, fmt.Sprintf("已记录 %d 项", recorded)).FontSize(t.Rem(.72)).
					TextColor(t.TextMuted)
			}
		})
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
