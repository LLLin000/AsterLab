package runui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"asterlab-mygo-prototype/internal/notebook"
	"asterlab-mygo-prototype/internal/resultgrid"
	"asterlab-mygo-prototype/internal/theme"

	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/onevra/source/components"
	"github.com/oneo-me/onevra/source/icons/lucide"
)

// State is the real UI state; never fake success.
type State string

const (
	StateLoading         State = "loading"
	StateReady           State = "ready"
	StateDraft           State = "draft"
	StateSaving          State = "saving"
	StateSaved           State = "saved"
	StateValidationError State = "validation_error"
	StateError           State = "error"
	StateBlocked         State = "blocked"
	StateEmpty           State = "empty"
)

type Task struct {
	Key       string
	Title     string
	Timepoint string
	Status    string
	AssayID   string
	// Endpoint/Unit come from the frozen assay design, not from an earlier
	// measurement. A task with neither cannot produce a valid readout.
	Endpoint string
	Unit     string
	// DesignStepID is the frozen design's workflow node this task executes.
	// Empty means unresolved; the surface says so instead of guessing.
	DesignStepID string
	ProtocolVersionID string
	Instruction string
	Parameters []FrozenParameter
	// Checks are the frozen protocol step's real checks, never placeholders.
	Checks []TaskCheck
}

type FrozenParameter struct {
	Key string
	Label string
	Value string
	Unit string
}

func (t Task) Done() bool { return t.Status == "completed" }

// taskSkipped reports the terminal rule-skip status Core projects. A skipped
// step never offers completion in any Run surface.
func taskSkipped(t *Task) bool {
	return t != nil && (t.Status == "skipped" || t.Status == "skipped_by_rule")
}

type TaskCheck struct {
	CheckID     string
	Label       string
	Instruction string
	Blocking    bool
}

type CheckAck struct {
	CheckID string
	Status  string
}

type CheckRow struct {
	Key  string
	Task string
	Def  TaskCheck
}

type Measurement struct {
	SampleID string
	Endpoint string
	Value    float64
	Unit     string
}

type AuditInfo struct {
	ActorID    string
	RecordedAt string
	Decision   string
	Warnings   []string
}

type Event struct {
	Kind      string
	Message   string
	TaskKey   string
	StepPath  string
	AssayID   string
	Timepoint string
	Checks    []CheckAck
}

type RunView struct {
	Title        string
	Status       string
	Revision     uint64
	DesignRevision uint64
	AsterDocRevision uint64
	Tasks        []Task
	Measurements []Measurement
	Events       []Event
	Audit        *AuditInfo
	// Modules renders the frozen design's reus able modules for a task. The
	// application supplies it; Run never edits the frozen plan through it.
	Modules func(c *ui.Context, task Task)
}

type DraftRow struct {
	SampleID string
	Value    string
}

type Draft struct {
	Rows  []DraftRow
	Error string
}

type Callbacks struct {
	SaveReadout  func(measurements []Measurement)
	SaveBatch    func(schema resultgrid.Schema, cells []resultgrid.Cell)
	CompleteTask func(taskKey string, checks []CheckAck)
	SaveChecks   func(taskKey string, checks []CheckAck)
	SelectTask   func(taskKey string)
	OpenBatch    func(batchID string)
}

type Workspace struct {
	View       RunView
	State      State
	Draft      Draft
	Grid       *resultgrid.Grid
	PlateView  bool
	ReviewView bool
	FullWidth  bool
	Callbacks  Callbacks
	Selected   string
	Width      int
	// Modules renders the frozen design's reusable modules for a task; the
	// application supplies it. Run never edits the frozen plan through it.
	Modules func(c *ui.Context, task Task)
	// Notebook is this run's read-only timeline; NotebookView switches the
	// surface between execution and the timeline.
	Notebook     notebook.Notebook
	NotebookView bool
	NotebookList components.ListState
	NotebookOpen map[string]bool
	Narrow       bool
	Error        string
	// CheckState holds one persistent bool per frozen check so the checkbox
	// keeps its value across frames. SavedChecks is what storage last returned;
	// a difference queues one SaveChecks call on the next frame.
	CheckState   map[string]*bool
	SavedChecks  map[string]bool
	CheckSaving  bool
	pendingCheck string
}

func NewWorkspace(v RunView, cb Callbacks) *Workspace {
	w := &Workspace{View: v, State: StateLoading, Callbacks: cb, Width: 1200}
	w.SyncChecks()
	w.Draft = NewDraft(v)
	if len(v.Tasks) == 0 && len(v.Measurements) == 0 {
		w.State = StateEmpty
	} else {
		w.State = StateReady
	}
	if w.Selected == "" {
		w.Selected = currentTaskKey(v.Tasks)
	}
	return w
}

func NewDraft(v RunView) Draft {
	rows := make([]DraftRow, 0, len(v.Measurements))
	for _, m := range v.Measurements {
		rows = append(rows, DraftRow{SampleID: m.SampleID, Value: formatValue(m.Value)})
	}
	if len(rows) == 0 {
		rows = []DraftRow{{}, {}}
	}
	return Draft{Rows: rows}
}

func currentTaskKey(tasks []Task) string {
	for _, t := range tasks {
		if !t.Done() {
			return t.Key
		}
	}
	if len(tasks) > 0 {
		return tasks[len(tasks)-1].Key
	}
	return ""
}

func formatValue(v float64) string {
	if v == 0 {
		return ""
	}
	return strconv.FormatFloat(v, 'g', -1, 64)
}

func (w *Workspace) isNarrow() bool {
	if w.Narrow {
		return true
	}
	if w.Width > 0 && w.Width <= 1000 {
		return true
	}
	return false
}

func (w *Workspace) shouldStack(c *ui.Context) bool {
	if w.isNarrow() {
		return true
	}
	if c != nil {
		cW, _ := c.Size()
		if cW > 0 && cW <= 1000 {
			return true
		}
	}
	return false
}

func (w *Workspace) currentTask() *Task {
	for i := range w.View.Tasks {
		if w.View.Tasks[i].Key == w.Selected {
			return &w.View.Tasks[i]
		}
	}
	if len(w.View.Tasks) > 0 {
		return &w.View.Tasks[0]
	}
	return nil
}

func (w *Workspace) taskCounts() (done, total int) {
	for _, t := range w.View.Tasks {
		total++
		if t.Done() {
			done++
		}
	}
	return
}

func (w *Workspace) parseDraft() ([]Measurement, string) {
	var ms []Measurement
	hasValid := false
	for _, r := range w.Draft.Rows {
		id := strings.TrimSpace(r.SampleID)
		valStr := strings.TrimSpace(r.Value)
		if id == "" && valStr == "" {
			continue
		}
		if id == "" {
			return nil, "样本 / 孔位不能为空"
		}
		if valStr == "" {
			continue
		}
		v, err := strconv.ParseFloat(valStr, 64)
		if err != nil {
			return nil, "测量值格式错误: " + valStr
		}
		hasValid = true
		ep, unit := "", ""
		if task := w.currentTask(); task != nil {
			ep, unit = task.Endpoint, task.Unit
		}
		if ep == "" && len(w.View.Measurements) > 0 {
			ep = w.View.Measurements[0].Endpoint
			unit = w.View.Measurements[0].Unit
		}
		if ep == "" {
			return nil, "当前任务没有可用的测量终点"
		}
		ms = append(ms, Measurement{SampleID: id, Endpoint: ep, Value: v, Unit: unit})
	}
	if !hasValid {
		return nil, "至少填写一条有效数值"
	}
	return ms, ""
}

func (w *Workspace) saveChecks(taskKey string) {
	if w.Callbacks.SaveChecks == nil {
		return
	}
	w.CheckSaving = true
	w.Callbacks.SaveChecks(taskKey, w.acks(taskKey))
}

func (w *Workspace) save() {
	if w.Grid != nil && w.Callbacks.SaveBatch != nil {
		if w.Grid.Preview != nil {
			w.Error = "请先确认或取消粘贴预览"
			w.Grid.Error = w.Error
			w.Grid.State = resultgrid.StateError
			return
		}
		w.State = StateSaving
		w.Grid.State = resultgrid.StateSaving
		w.Grid.Error = ""
		w.Error = ""
		w.Callbacks.SaveBatch(w.Grid.Schema, append([]resultgrid.Cell(nil), w.Grid.Cells...))
		return
	}
	ms, err := w.parseDraft()
	if err != "" {
		w.State = StateValidationError
		w.Draft.Error = err
		w.Error = err
		return
	}
	w.State = StateSaving
	w.Draft.Error = ""
	w.Error = ""
	if w.Callbacks.SaveReadout != nil {
		w.Callbacks.SaveReadout(ms)
	}
}

func (w *Workspace) complete() {
	t := w.currentTask()
	if t == nil {
		return
	}
	if taskSkipped(t) {
		// A rule-skipped step is terminal: completing it would contradict
		// Core's projected status. The button is disabled too; this guards
		// programmatic callers.
		return
	}
	if len(w.openBlocking(t)) > 0 {
		// The panel already explains which checks block completion; clicking
		// complete must not invent an error state or hide the checklist.
		return
	}
	w.State = StateSaving
	if w.Callbacks.CompleteTask != nil {
		w.Callbacks.CompleteTask(t.Key, w.acks(t.Key))
	}
}

func valueOrUnrecorded(value string) string {
	if strings.TrimSpace(value) == "" {
		return "未记录"
	}
	return value
}

func labelOr(label, fallback string) string {
	if strings.TrimSpace(label) != "" {
		return label
	}
	return fallback
}

func checkKey(taskKey, checkID string) string { return taskKey + "\x00" + checkID }

// SyncChecks rebuilds checkbox state from the loaded snapshot. A check counts
// as confirmed only when a stored event says "passed"; anything else, including
// an old event with no checks, stays unconfirmed.
func (w *Workspace) SyncChecks() {
	states := map[string]*bool{}
	saved := map[string]bool{}
	for _, task := range w.View.Tasks {
		for _, check := range task.Checks {
			key := checkKey(task.Key, check.CheckID)
			on := false
			if states[key] == nil {
				states[key] = &on
				saved[key] = false
			}
		}
	}
	for _, event := range w.View.Events {
		for _, ack := range event.Checks {
			for _, task := range w.View.Tasks {
				if !eventMatchesTask(event, task) {
					continue
				}
				key := checkKey(task.Key, ack.CheckID)
				on := ack.Status == "passed"
				states[key] = &on
				saved[key] = on
			}
		}
	}
	w.CheckState = states
	w.SavedChecks = saved
	w.CheckSaving = false
	w.pendingCheck = ""
}

func eventMatchesTask(event Event, task Task) bool {
	if event.TaskKey != "" {
		return event.TaskKey == task.Key
	}
	if event.StepPath != task.Title {
		return false
	}
	if event.AssayID != "" && task.AssayID != "" && event.AssayID != task.AssayID {
		return false
	}
	return event.Timepoint == task.Timepoint
}

func (w *Workspace) checkOn(taskKey, checkID string) *bool {
	key := checkKey(taskKey, checkID)
	if w.CheckState == nil {
		w.CheckState = map[string]*bool{}
	}
	on := w.CheckState[key]
	if on == nil {
		value := false
		on = &value
		w.CheckState[key] = on
	}
	if w.SavedChecks == nil {
		w.SavedChecks = map[string]bool{}
	}
	return on
}

func (w *Workspace) openBlocking(task *Task) []TaskCheck {
	open := []TaskCheck{}
	for _, check := range task.Checks {
		if check.Blocking && !*w.checkOn(task.Key, check.CheckID) {
			open = append(open, check)
		}
	}
	return open
}

func (w *Workspace) acks(taskKey string) []CheckAck {
	acks := []CheckAck{}
	for _, task := range w.View.Tasks {
		if task.Key != taskKey {
			continue
		}
		for _, check := range task.Checks {
			// Core accepts passed/failed/blocked/skipped. An unchecked box is
			// persisted as failed, never as an invented "pending" status.
			status := "failed"
			if *w.checkOn(taskKey, check.CheckID) {
				status = "passed"
			}
			acks = append(acks, CheckAck{CheckID: check.CheckID, Status: status})
		}
	}
	return acks
}

func View(c *ui.Context, w *Workspace) {
	if w == nil {
		return
	}
	if w.pendingCheck != "" && !w.CheckSaving {
		task := w.pendingCheck
		w.pendingCheck = ""
		w.saveChecks(task)
	}
	t := c.Theme()
	tokens := theme.GetTokens(t)

	// Keyboard shortcuts
	if c.Shortcut(ui.Cmd, ui.KeyS) {
		w.save()
	}
	if c.Shortcut(ui.Cmd, ui.KeyN) && w.Grid == nil {
		w.Draft.Rows = append(w.Draft.Rows, DraftRow{})
	}

	ui.Column(c).Fill().Background(t.Background).Children(func() {
		// Header bar with status and metadata
		ui.Row(c).Height(52).Padding(0, 16).AlignItems(ui.Center).Background(tokens.CardBackground).
			Border(1, tokens.CardBorder).Children(func() {
			ui.Text(c, w.View.Title).FontSize(18).Bold().TextColor(t.Text).
				SingleLine().Ellipsis("…").Shrink(1).MaxWidth(520)
			tab := 0
			if w.NotebookView {
				tab = 1
			}
			components.Tabs(c, &tab, components.TabsHighlight, "执行", "运行日志")
			w.NotebookView = tab == 1
			if w.Grid != nil {
				ui.Button(c, "保存").OnClick(func() { w.save() })
			}
			ui.Spacer(c)
			// Metadata Badges
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				if w.View.Status != "" {
					ui.Row(c).Padding(3, 8).Radius(6).Background(tokens.NeutralBg).Children(func() {
						ui.Text(c, w.View.Status).FontSize(11).TextColor(tokens.NeutralText).Bold()
					})
				}
				ui.Row(c).Padding(3, 8).Radius(6).Background(tokens.NeutralBg).Children(func() {
					ui.Text(c, fmt.Sprintf("rev %d", w.View.Revision)).FontSize(11).TextColor(tokens.NeutralText).Bold()
				})
				// Save State Badge
				renderStateBadge(c, w.State, tokens)
			})
		})

		if w.State == StateLoading {
			ui.Row(c).Fill().Center().Children(func() {
				ui.Text(c, "正在加载实验运行数据…").FontSize(14).TextColor(t.TextMuted)
			})
			return
		}
		if w.NotebookView {
			notebookPanel(c, w, t)
			return
		}
		if w.State == StateEmpty {
			ui.Row(c).Fill().Center().Children(func() {
				ui.Text(c, "当前运行暂无任务与测量记录").FontSize(14).TextColor(t.TextMuted)
			})
			return
		}
		if w.State == StateBlocked {
			ui.Row(c).Fill().Center().Children(func() {
				ui.Column(c).Gap(8).Center().Children(func() {
					ui.Text(c, "运行阻塞").FontSize(16).Bold().TextColor(t.Danger)
					ui.Text(c, w.Error).FontSize(13).TextColor(t.TextMuted)
				})
			})
			return
		}

		// Scrollable main workspace body
		ui.Scroll(c).Grow(1).Children(func() {
			if w.FullWidth || w.shouldStack(c) {
				ui.Column(c).FillWidth().Gap(16).Padding(16).Children(func() {
					recordPanel(c, w, t, tokens)
					plateCard(c, w, t, tokens)
					taskPanel(c, w, t, tokens)
					readinessCard(c, w, t, tokens)
				})
			} else {
				// 2 balanced proportional columns: Left (Execution & Readiness), Right (Plate & Flow)
				ui.Grid(c).FillWidth().ColumnTracks(ui.Fr(1), ui.Fr(1)).Gap(16).Padding(16).Children(func() {
					ui.Column(c).Gap(16).Children(func() {
						recordPanel(c, w, t, tokens)
						readinessCard(c, w, t, tokens)
					})
					ui.Column(c).Gap(16).Children(func() {
						plateCard(c, w, t, tokens)
						taskPanel(c, w, t, tokens)
					})
				})
			}
		})
	})
}

func renderStateBadge(c *ui.Context, state State, tokens theme.Tokens) {
	switch state {
	case StateSaving:
		ui.Row(c).Padding(3, 8).Radius(6).Background(tokens.WarningBg).Children(func() {
			ui.Text(c, "⏳ 保存中…").FontSize(11).TextColor(tokens.WarningText).Bold()
		})
	case StateSaved:
		ui.Row(c).Padding(3, 8).Radius(6).Background(tokens.SuccessBg).Children(func() {
			ui.Text(c, "✓ 已保存").FontSize(11).TextColor(tokens.SuccessText).Bold()
		})
	case StateValidationError, StateError, StateBlocked:
		ui.Row(c).Padding(3, 8).Radius(6).Background(tokens.DangerBg).Children(func() {
			ui.Text(c, "⚠ 错误").FontSize(11).TextColor(tokens.DangerText).Bold()
		})
	case StateDraft:
		ui.Row(c).Padding(3, 8).Radius(6).Background(tokens.NeutralBg).Children(func() {
			ui.Text(c, "● 草稿 (未保存)").FontSize(11).TextColor(tokens.NeutralText)
		})
	default:
		ui.Row(c).Padding(3, 8).Radius(6).Background(tokens.NeutralBg).Children(func() {
			ui.Text(c, "● 就绪").FontSize(11).TextColor(tokens.NeutralText)
		})
	}
}

func recordPanel(c *ui.Context, w *Workspace, t *ui.Theme, tokens theme.Tokens) {
	ui.Column(c).Gap(16).Padding(20).Radius(12).Background(tokens.CardBackground).Border(1, tokens.CardBorder).Children(func() {
		ct := w.currentTask()
		taskTitle := "当前任务"
		if ct != nil && ct.Title != "" {
			taskTitle = "当前任务 · " + ct.Title
		}
		ui.Row(c).AlignItems(ui.Center).Children(func() {
			ui.Text(c, taskTitle).FontSize(16).Bold().TextColor(t.Text)
			ui.Spacer(c)
			done, total := w.taskCounts()
			ui.Row(c).Padding(2, 8).Radius(6).Background(tokens.NeutralBg).Children(func() {
				ui.Text(c, fmt.Sprintf("任务进度 %d/%d", done, total)).FontSize(11).TextColor(tokens.NeutralText).Bold()
			})
		})
		ui.Text(c, "具体操作条件继承已选 Protocol").FontSize(12).TextColor(t.TextMuted)

		// Task chips stepper
		// Task chips stepper with wrapping to avoid clipping
		ui.Row(c).Wrap().Gap(6).Children(func() {
			for i := range w.View.Tasks {
				task := w.View.Tasks[i]
				selected := task.Key == w.Selected
				bg := tokens.NeutralBg
				txtColor := tokens.NeutralText
				if selected {
					bg = t.Accent
					txtColor = t.AccentText
				}
				label := task.Title
				if task.Timepoint != "" {
					label = task.Timepoint + " " + label
				}
				if task.AssayID != "" {
					label += " · " + task.AssayID
				}
				if task.Done() {
					label += " ✓"
				}
				key := task.Key
				b := ui.ButtonBase(c).Radius(8).Padding(6, 12).Background(bg)
				b.Children(func() {
					ui.Text(c, label).FontSize(12).TextColor(txtColor).Bold()
				})
				if b.Clicked() {
					w.Selected = key
					if w.Callbacks.SelectTask != nil {
						w.Callbacks.SelectTask(key)
					}
				}
			}
		})
		ui.Divider(c)

		if w.Grid != nil {
			ui.Row(c).Gap(8).Children(func() {
				if ui.Button(c, "Plate").Clicked() {
					w.PlateView = true
				}
				if ui.Button(c, "Table").Clicked() {
					w.PlateView = false
				}
				if ui.Button(c, "全宽编辑").Clicked() {
					w.PlateView = false
					w.FullWidth = !w.FullWidth
				}
				if ui.Button(c, "汇总").Clicked() {
					w.ReviewView = true
				}
				if ui.Button(c, "原始表格").Clicked() {
					w.ReviewView = false
				}
			})
			if w.ReviewView {
				reviewPanel(c, w, t, tokens)
			} else if w.PlateView {
				plateGrid(c, w.Grid)
			} else {
				resultgrid.View(c, w.Grid, resultgrid.Options{FullWidth: w.FullWidth})
				switch w.Grid.State {
				case resultgrid.StateDraft:
					w.State = StateDraft
				case resultgrid.StateError:
					w.State = StateError
					w.Error = w.Grid.Error
				}
			}
		} else {
			label := "原始测量记录"
			if len(w.View.Measurements) > 0 {
				m := w.View.Measurements[0]
				label = m.Endpoint
				if m.Unit != "" {
					label += " (" + m.Unit + ")"
				}
			}
			ui.Column(c).Gap(6).Children(func() {
				ui.Text(c, label).FontSize(15).Bold().TextColor(t.Text)
				ui.Text(c, "仅记录原始读数；派生结果由 assay 结果表独立呈现").FontSize(12).TextColor(t.TextMuted)
			})
			ui.Grid(c).Columns(3).ColumnTracks(ui.Fr(1), ui.Fr(1), ui.Fixed(36)).Gap(8).Children(func() {
				ui.Text(c, "样本 / 孔位").FontSize(11).TextColor(t.TextMuted).Bold()
				ui.Text(c, "原始测量值").FontSize(11).TextColor(t.TextMuted).Bold()
				ui.Text(c, "").FontSize(11)
				for i := range w.Draft.Rows {
					i := i
					inp1 := ui.TextInput(c, &w.Draft.Rows[i].SampleID).Placeholder("样本编号 (如 Control-1)")
					if inp1.Changed() {
						w.State = StateDraft
					}
					inp2 := ui.TextInput(c, &w.Draft.Rows[i].Value).Placeholder("原始测量值")
					if inp2.Changed() {
						w.State = StateDraft
					}
					if len(w.Draft.Rows) > 1 {
						if ui.Button(c, "✕").Clicked() {
							w.Draft.Rows = append(w.Draft.Rows[:i], w.Draft.Rows[i+1:]...)
							w.State = StateDraft
						}
					} else {
						ui.Spacer(c)
					}
				}
			})
			if ui.Button(c, "+ 新增一行 (Ctrl+N)").Clicked() {
				w.Draft.Rows = append(w.Draft.Rows, DraftRow{})
				w.State = StateDraft
			}
		}
		// Notifications
		if w.State == StateValidationError || w.State == StateError || w.Draft.Error != "" {
			msg := w.Draft.Error
			if msg == "" {
				msg = w.Error
			}
			ui.Row(c).Padding(10, 14).Radius(8).Background(tokens.DangerBg).Border(1, tokens.DangerText).Children(func() {
				ui.Text(c, "⚠ "+msg).FontSize(12).TextColor(tokens.DangerText).Bold()
			})
		}
		if w.State == StateSaved {
			ui.Row(c).Padding(10, 14).Radius(8).Background(tokens.SuccessBg).Border(1, tokens.SuccessText).Children(func() {
				ui.Text(c, "✓ 记录已成功持久化至实验记录").FontSize(12).TextColor(tokens.SuccessText).Bold()
			})
		}

		// Action buttons
		open := []TaskCheck{}
		if ct != nil {
			open = w.openBlocking(ct)
		}
		if len(open) > 0 {
			names := make([]string, 0, len(open))
			for _, check := range open {
				names = append(names, labelOr(check.Label, check.CheckID))
			}
			ui.Row(c).Padding(8, 12).Radius(6).Background(tokens.WarningBg).Children(func() {
				ui.Text(c, "完成前需确认："+strings.Join(names, "、")).FontSize(12).
					TextColor(tokens.WarningText)
			})
		}
		ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
			saveLabel := "保存读数 (Ctrl+S)"
			if w.State == StateSaving {
				saveLabel = "保存中…"
			}
			btnSave := ui.ButtonBase(c).Radius(8).Padding(8, 20).Background(t.Accent).OnClick(func() { w.save() })
			btnSave.Children(func() {
				ui.Text(c, saveLabel).FontSize(13).Bold().TextColor(t.AccentText)
			})

			btnComplete := ui.Button(c, "完成本步").Radius(8).Padding(8, 16).
				Disabled(len(open) > 0 || taskSkipped(ct))
			if btnComplete.Clicked() && len(open) == 0 && !taskSkipped(ct) {
				w.complete()
			}
		})
	})
}

func plateCard(c *ui.Context, w *Workspace, t *ui.Theme, tokens theme.Tokens) {
	ui.Column(c).Gap(12).Padding(20).Radius(12).Background(tokens.CardBackground).Border(1, tokens.CardBorder).Children(func() {
		ui.Text(c, "空间板视图 · 只读").FontSize(15).Bold().TextColor(t.Text)
		if w.Grid == nil {
			ui.Text(c, "当前任务尚无 assay schema").FontSize(12).TextColor(t.TextMuted)
			return
		}
		plateGrid(c, w.Grid)
	})
}

func plateGrid(c *ui.Context, grid *resultgrid.Grid) {
	if grid == nil || grid.Rows() == 0 || grid.Columns() == 0 {
		return
	}
	ui.Column(c).Gap(3).Children(func() {
		ui.Grid(c).Columns(grid.Columns() + 1).Gap(3).Children(func() {
			ui.Text(c, "")
			for _, label := range grid.Schema.ColumnLabels {
				ui.Text(c, label).FontSize(9).Bold()
			}
			for row, label := range grid.Schema.RowLabels {
				ui.Text(c, label).FontSize(9).Bold()
				for col := range grid.Columns() {
					cell, _ := grid.CellAt(row, col)
					value := "—"
					if cell.Value != nil {
						value = strconv.FormatFloat(*cell.Value, 'g', -1, 64)
					}
					cellLabel := value
					if cell.SampleID != "" {
						cellLabel = cell.SampleID + " · " + value
					}
					rr, cc := row, col
					if ui.Button(c, cellLabel).Clicked() {
						grid.SetSelection(resultgrid.Selection{StartRow: rr, StartColumn: cc, EndRow: rr, EndColumn: cc})
					}
				}
			}
		})
	})
}

// notebookRow is one line of the timeline: a day heading or a stored record.
type notebookRow struct {
	key   string
	day   string
	entry *notebook.Entry
}

// notebookPanel renders the stored timeline with Onevra's List, following the
// activity-feed pattern: day headings, a category icon per record, the action
// first, its time right-aligned, and details folded behind one disclosure.
func notebookPanel(c *ui.Context, w *Workspace, t *ui.Theme) {
	entries := w.Notebook.Entries
	rows := notebookRows(entries)
	w.NotebookList.Key = func(i int) any { return rows[i].key }
	if w.NotebookOpen == nil {
		w.NotebookOpen = map[string]bool{}
	}
	ui.Column(c).Grow(1).FillWidth().Padding(t.Space(6)).Gap(t.Space(3)).Children(func() {
		ui.Row(c).AlignItems(ui.Center).Gap(t.Space(2)).Children(func() {
			ui.Text(c, "运行日志").FontSize(20).Bold().TextColor(t.Text)
			ui.Text(c, "只读 · 系统记录").FontSize(11).TextColor(t.TextMuted)
			ui.Text(c, fmt.Sprintf("Design rev %d · %d 条记录", w.Notebook.DesignRevision, len(entries))).
				FontSize(12).TextColor(t.TextMuted)
		})
		if len(entries) == 0 {
			ui.Text(c, "此运行还没有可追溯的记录。").FontSize(13).TextColor(t.TextMuted)
			return
		}
		components.List(c, &w.NotebookList, len(rows), func(i int) {
			row := rows[i]
			if row.entry == nil {
				ui.Row(c).FillWidth().Padding(t.Space(3), 0).Gap(t.Space(2)).AlignItems(ui.Center).
					Children(func() {
						ui.Text(c, row.day).FontSize(11).Bold().TextColor(t.TextMuted).Shrink(0)
						ui.Box(c).Grow(1).Height(1).Background(t.Border)
					})
				return
			}
			entry := *row.entry
			open := w.NotebookOpen[entry.ID]
			ui.Column(c).FillWidth().Children(func() {
				ui.Row(c).FillWidth().Padding(t.Space(2.5), 0).Gap(t.Space(3)).
					AlignItems(ui.Start).Children(func() {
					ui.Box(c).Size(t.Space(7), t.Space(7)).Shrink(0).Radius(t.Space(2)).
						Border(1, t.Border).Background(t.Background).Center().Children(func() {
						components.Icon(c, notebookIcon(entry.Category)).
							Size(t.Space(3.5), t.Space(3.5)).TextColor(t.TextMuted)
					})
					ui.Column(c).Grow(1).MinWidth(0).Gap(t.Space(.75)).Children(func() {
						ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(t.Space(2)).Children(func() {
							ui.Text(c, entry.Title).FontSize(t.Rem(1.0)).FontWeight(600).
								SingleLine().Ellipsis("…")
							ui.Spacer(c)
							main, secondary := entryTime(entry)
							ui.Column(c).AlignItems(ui.End).Children(func() {
								ui.Text(c, main).FontSize(t.Rem(.78)).TextColor(t.TextMuted).
									FontFeatures("tnum")
								if secondary != "" {
									ui.Text(c, secondary).FontSize(t.Rem(.7)).TextColor(t.TextMuted).
										FontFeatures("tnum")
								}
							})
						})
						if summary := entrySummary(entry); summary != "" {
							ui.Text(c, summary).FontSize(t.Rem(.78)).TextColor(t.TextMuted)
						}
						if entry.Note != "" {
							ui.Text(c, entry.Note).FontSize(t.Rem(.72)).
								TextColor(t.TextMuted).SingleLine().Ellipsis("…")
						}
						if len(entry.References) > 0 {
							first := entry.References[0]
							ui.Text(c, "来源："+first.Label+" "+first.ID).FontSize(t.Rem(.72)).
								TextColor(t.TextMuted).SingleLine().Ellipsis("…")
						}
						switch entry.Trace {
						case notebook.TraceInvalid:
							ui.Text(c, "来源失效：引用的记录在此运行中不存在。").
								FontSize(t.Rem(.72)).TextColor(t.Warning)
						case notebook.TraceMissing:
							ui.Text(c, "来源未记录：这条记录无法追溯到原始数据。").
								FontSize(t.Rem(.72)).TextColor(t.Warning)
						}
						ui.Row(c).Gap(t.Space(2)).AlignItems(ui.Center).Children(func() {
							if w.Callbacks.OpenBatch != nil {
								for _, ref := range entry.References {
									if ref.Kind != "readout_batch" {
										continue
									}
									batchID := ref.ID
									if components.Button(c, "打开原始孔板", components.ButtonGhost).Clicked() {
										w.Callbacks.OpenBatch(batchID)
									}
									break
								}
							}
							if entry.TaskID != "" {
								if components.Button(c, "查看任务", components.ButtonGhost).Clicked() {
									w.Selected = entry.TaskID
									w.NotebookView = false
									if w.Callbacks.SelectTask != nil {
										w.Callbacks.SelectTask(entry.TaskID)
									}
								}
							}
							components.Collapsible(c, "详情", &open, components.CollapsibleOptions{}, func() {
								for _, detail := range entry.Details {
									ui.Text(c, detail.Label+"："+detail.Value).FontSize(t.Rem(.78)).
										TextColor(t.TextMuted)
								}
								for _, ref := range entry.References {
									ui.Text(c, "来源："+ref.Label+" "+ref.ID).FontSize(t.Rem(.72)).
										TextColor(t.TextMuted)
								}
							})
							w.NotebookOpen[entry.ID] = open
						})
					})
				})
				ui.Divider(c)
			})
		}).Fill()
	})
}

func notebookRows(entries []notebook.Entry) []notebookRow {
	rows := make([]notebookRow, 0, len(entries)+4)
	last := "\x00"
	for i := range entries {
		day := notebookDay(entries[i])
		if day != last {
			// The same heading may reappear when unknown-time records sit at
			// both ends of the stored order; the index keeps row keys unique.
			rows = append(rows, notebookRow{key: fmt.Sprintf("day:%s:%d", day, i), day: day})
			last = day
		}
		rows = append(rows, notebookRow{key: "entry:" + entries[i].ID, entry: &entries[i]})
	}
	return rows
}

// notebookDay groups by the stored time's local calendar day. A record without
// a stored time gets its own honest group instead of an invented date.
func notebookDay(entry notebook.Entry) string {
	raw := entry.RecordedAt
	if raw == "" {
		raw = entry.OccurredAt
	}
	if raw == "" {
		return "时间未记录"
	}
	at, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return "时间未记录"
	}
	local := at.Local()
	y, m, d := local.Date()
	now := time.Now()
	ny, nm, nd := now.Date()
	today := time.Date(ny, nm, nd, 0, 0, 0, 0, time.Local)
	date := time.Date(y, m, d, 0, 0, 0, 0, time.Local)
	switch {
	case date.Equal(today):
		return "今天"
	case date.Equal(today.AddDate(0, 0, -1)):
		return "昨天"
	}
	return local.Format("2006 年 1 月 2 日")
}

func entryTime(entry notebook.Entry) (main, secondary string) {
	raw := entry.RecordedAt
	if raw == "" {
		raw = entry.OccurredAt
	}
	if raw == "" {
		return "未记录", ""
	}
	main = timeOnly(raw)
	if entry.OccurredAt != "" && entry.RecordedAt != "" && entry.OccurredAt != entry.RecordedAt {
		secondary = "发生 " + timeOnly(entry.OccurredAt)
	}
	return main, secondary
}

func entrySummary(entry notebook.Entry) string {
	parts := []string{}
	if entry.TaskID != "" {
		parts = append(parts, "任务 "+entry.TaskID)
	}
	if entry.DesignStepID != "" {
		parts = append(parts, "步骤 "+entry.DesignStepID)
	}
	for _, detail := range entry.Details {
		if detail.Label == "时间点" {
			parts = append(parts, detail.Value)
		}
	}
	return strings.Join(parts, " · ")
}

func notebookIcon(category notebook.Category) *ui.SVG {
	switch category {
	case notebook.Execution:
		return lucide.Play()
	case notebook.Checklist:
		return lucide.ListChecks()
	case notebook.Measurement:
		return lucide.Table2()
	case notebook.Observation:
		return lucide.Eye()
	case notebook.Deviation:
		return lucide.TriangleAlert()
	}
	return lucide.Settings()
}

func recordedLabel(entry notebook.Entry) string {
	raw := entry.RecordedAt
	if raw == "" {
		raw = entry.OccurredAt
	}
	if raw == "" {
		return "时间未记录"
	}
	if entry.RecordedAt == "" {
		return timeOnly(raw)
	}
	return timeOnly(raw)
}

func timeOnly(raw string) string {
	if len(raw) >= 16 {
		return raw[11:16]
	}
	return raw
}

// reviewPanel is the read-only derivation view: per-sample technical
// replicates with recorded, missing and excluded counts. It never edits cells
// and never turns a count of wells into an independent biological n.
func reviewPanel(c *ui.Context, w *Workspace, t *ui.Theme, tokens theme.Tokens) {
	review := resultgrid.Summarize(w.Grid)
	ui.Column(c).Gap(8).Children(func() {
		ui.Text(c, "结果审阅").FontSize(15).Bold().TextColor(t.Text)
		ui.Text(c, fmt.Sprintf("%s · v%d · 冻结设计", review.SchemaID, review.SchemaVersion)).
			FontSize(11).TextColor(t.TextMuted)
		if len(review.Rows) == 0 {
			ui.Text(c, "尚无已记录的读数。").FontSize(12).TextColor(t.TextMuted)
			return
		}
		ui.Row(c).Gap(12).Children(func() {
			for _, header := range []string{"样本", "组别", "字段", "记录", "缺失", "排除", "均值", "SD"} {
				ui.Text(c, header).FontSize(11).Bold().TextColor(t.TextMuted)
			}
		})
		for _, row := range review.Rows {
			ui.Row(c).Gap(12).Children(func() {
				ui.Text(c, row.SampleID).FontSize(12).TextColor(t.Text)
				ui.Text(c, row.Group).FontSize(12).TextColor(t.TextMuted)
				ui.Text(c, row.Field).FontSize(12).TextColor(t.TextMuted)
				ui.Text(c, fmt.Sprintf("%d", row.Recorded)).FontSize(12).FontFeatures("tnum")
				ui.Text(c, fmt.Sprintf("%d", row.Missing)).FontSize(12).TextColor(t.Warning).FontFeatures("tnum")
				ui.Text(c, fmt.Sprintf("%d", row.Excluded)).FontSize(12).TextColor(t.TextMuted).FontFeatures("tnum")
				if row.Recorded == 0 {
					ui.Text(c, "—").FontSize(12).TextColor(t.TextMuted)
					ui.Text(c, "—").FontSize(12).TextColor(t.TextMuted)
					return
				}
				ui.Text(c, fmt.Sprintf("%.4g", row.Mean)).FontSize(12).FontFeatures("tnum")
				if row.Recorded < 2 {
					ui.Text(c, "—").FontSize(12).TextColor(t.TextMuted)
				} else {
					ui.Text(c, fmt.Sprintf("%.4g", row.SD)).FontSize(12).FontFeatures("tnum")
				}
			})
		}
		ui.Text(c, "按样本汇总技术重复（算术均值、样本 SD，n−1）；生物学重复 n 不由孔位或行数推导。").
			FontSize(11).TextColor(t.TextMuted)
	})
}

func taskPanel(c *ui.Context, w *Workspace, t *ui.Theme, tokens theme.Tokens) {
	ui.Column(c).Gap(8).Padding(16).Radius(8).Background(tokens.CardBackground).Border(1, tokens.CardBorder).Children(func() {
		ct := w.currentTask()
		ui.Text(c, "任务与结果").FontSize(15).Bold().TextColor(t.Text)
		if ct == nil {
			ui.Text(c, "当前运行没有任务").FontSize(13).TextColor(t.TextMuted)
			return
		}
		title := ct.Title
		if ct.Timepoint != "" {
			title = ct.Timepoint + " · " + title
		}
		ui.Text(c, title).FontSize(13).Bold().TextColor(t.Text)
		ui.Text(c, "任务 "+ct.Key+" · "+ct.Status).FontSize(11).TextColor(t.TextMuted)
		// Right-side inspector keeps its existing primary recording flow visible: UX-P1/UX-LAYOUT.
		// Frozen protocol details are an inspector section; UX-P2/P3 and UX-STATE.
		ui.Divider(c)
		ui.Text(c, "协议与参数 · 只读 · 已冻结").FontSize(12).Bold().TextColor(t.Text)
		ui.Text(c, "已冻结 · 无编辑入口").FontSize(11).TextColor(t.TextMuted)
		if ct.ProtocolVersionID != "" {
			ui.Text(c, "协议版本 "+ct.ProtocolVersionID).FontSize(11).TextColor(t.TextMuted)
		}
		ui.Text(c, fmt.Sprintf("Run rev %d · Design rev %d · AsterDoc rev %d",
			w.View.Revision, w.View.DesignRevision, w.View.AsterDocRevision)).
			FontSize(11).TextColor(t.TextMuted)
		if ct.Instruction != "" {
			ui.Text(c, "操作说明").FontSize(11).Bold().TextColor(t.Text)
			ui.Text(c, ct.Instruction).FontSize(12).TextColor(t.Text)
		}
		if len(ct.Parameters) > 0 {
			ui.Text(c, "冻结参数").FontSize(11).Bold().TextColor(t.Text)
			for _, p := range ct.Parameters {
				value := strings.TrimSpace(strings.Join([]string{p.Value, p.Unit}, " "))
				ui.Text(c, labelOr(p.Label, p.Key)+" · "+value).FontSize(11).TextColor(t.TextMuted)
			}
		}
		if ct.AssayID != "" {
			ui.Text(c, "Assay "+ct.AssayID).FontSize(11).TextColor(t.TextMuted)
		}
		if len(ct.Checks) > 0 {
			ui.Divider(c)
			ui.Row(c).AlignItems(ui.Center).Gap(8).Children(func() {
				ui.Text(c, "执行检查").FontSize(12).Bold().TextColor(t.Text)
				open := w.openBlocking(ct)
				if len(open) > 0 {
					ui.Row(c).Padding(2, 6).Radius(5).Background(tokens.DangerBg).Children(func() {
						ui.Text(c, fmt.Sprintf("%d 项阻塞", len(open))).FontSize(10).
							TextColor(tokens.DangerText).Bold()
					})
				} else {
					ui.Text(c, fmt.Sprintf("%d/%d 已确认", len(ct.Checks)-len(open), len(ct.Checks))).
						FontSize(11).TextColor(t.TextMuted)
				}
			})
			for _, check := range ct.Checks {
				check := check
				on := w.checkOn(ct.Key, check.CheckID)
				box := components.Checkbox(c, on, labelOr(check.Label, check.CheckID))
				if box.Changed() {
					w.pendingCheck = ct.Key
				}
				if check.Instruction != "" {
					ui.Text(c, check.Instruction).FontSize(11).TextColor(t.TextMuted)
				}
				if check.Blocking && !*on {
					ui.Text(c, "阻塞：未确认").FontSize(11).TextColor(tokens.DangerText)
				}
			}
		}
		if w.Modules != nil {
			ui.Divider(c)
			if ct.DesignStepID == "" {
				ui.Text(c, "未关联设计步骤").FontSize(12).Bold().TextColor(t.TextMuted)
				ui.Text(c, "该任务没有 design_step_id：可能来自旧运行，或设计把同一检测关联到了多个步骤。不按标题猜测对应模块。").
					FontSize(11).TextColor(t.TextMuted)
			} else {
				ui.Text(c, "设计模块 · "+ct.DesignStepID).FontSize(11).TextColor(t.TextMuted)
				w.Modules(c, *ct)
			}
		}
		if w.Grid != nil {
			ui.Text(c, fmt.Sprintf("结果表 %s · %d × %d", w.Grid.Schema.ID, w.Grid.Rows(), w.Grid.Columns())).FontSize(11).TextColor(t.TextMuted)
		} else if len(w.View.Measurements) > 0 {
			ui.Text(c, fmt.Sprintf("已记录 %d 条原始测量", len(w.View.Measurements))).FontSize(11).TextColor(t.TextMuted)
		} else {
			ui.Text(c, "尚无结果记录").FontSize(11).TextColor(t.TextMuted)
		}
	})
}

func readinessCard(c *ui.Context, w *Workspace, t *ui.Theme, tokens theme.Tokens) {
	ui.Column(c).Gap(14).Padding(20).Radius(12).Background(tokens.CardBackground).Border(1, tokens.CardBorder).Children(func() {
		ui.Text(c, "实验就绪检查").FontSize(15).Bold().TextColor(t.Text)

		if w.View.Audit != nil {
			ui.Divider(c)
			ui.Text(c, "创建记录").FontSize(11).Bold().TextColor(t.TextMuted)
			actor := w.View.Audit.ActorID
			if actor == "" {
				actor = "未记录"
			}
			ui.Text(c, "操作者 "+actor+" · "+valueOrUnrecorded(w.View.Audit.RecordedAt)).
				FontSize(11).TextColor(t.TextMuted)
			for _, warning := range w.View.Audit.Warnings {
				ui.Text(c, "警告："+warning).FontSize(11).TextColor(tokens.WarningText)
			}
		}
		if len(w.View.Events) > 0 {
			ui.Divider(c)
			ui.Text(c, "运行事件").FontSize(13).Bold().TextColor(t.Text)
			for _, e := range w.View.Events {
				ui.Row(c).Padding(3, 6).Children(func() {
					ui.Text(c, e.Kind+": "+e.Message).FontSize(11).TextColor(t.TextMuted)
				})
			}
		}
	})
}
