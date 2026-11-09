package runui

import (
	"fmt"
	"strconv"
	"strings"

	"asterlab-mygo-prototype/internal/resultgrid"
	"asterlab-mygo-prototype/internal/theme"

	"github.com/egoist/mygo/ui"
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
}

func (t Task) Done() bool { return t.Status == "completed" }

type Check struct {
	Key     string
	Title   string
	Status  string
	Binding string
}

type Measurement struct {
	SampleID string
	Endpoint string
	Value    float64
	Unit     string
}

type Event struct {
	Kind    string
	Message string
}

type RunView struct {
	Title        string
	Status       string
	Revision     uint64
	Tasks        []Task
	Checks       []Check
	Measurements []Measurement
	Events       []Event
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
	CompleteTask func(taskKey string)
	SelectTask   func(taskKey string)
}

type Workspace struct {
	View      RunView
	State     State
	Draft     Draft
	Grid      *resultgrid.Grid
	PlateView bool
	FullWidth bool
	Callbacks Callbacks
	Selected  string
	Width     int
	Narrow    bool
	Error     string
}

func NewWorkspace(v RunView, cb Callbacks) *Workspace {
	w := &Workspace{View: v, State: StateLoading, Callbacks: cb, Width: 1200}
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
		if len(w.View.Measurements) > 0 {
			ep = w.View.Measurements[0].Endpoint
			unit = w.View.Measurements[0].Unit
		}
		ms = append(ms, Measurement{SampleID: id, Endpoint: ep, Value: v, Unit: unit})
	}
	if !hasValid {
		return nil, "至少填写一条有效数值"
	}
	return ms, ""
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
	w.State = StateSaving
	if w.Callbacks.CompleteTask != nil {
		w.Callbacks.CompleteTask(t.Key)
	}
}

func View(c *ui.Context, w *Workspace) {
	if w == nil {
		return
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
			ui.Text(c, w.View.Title).FontSize(18).Bold().TextColor(t.Text)
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
			})
			if w.PlateView {
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
		ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
			saveLabel := "保存读数 (Ctrl+S)"
			if w.State == StateSaving {
				saveLabel = "保存中…"
			}
			btnSave := ui.ButtonBase(c).Radius(8).Padding(8, 20).Background(t.Accent).OnClick(func() { w.save() })
			btnSave.Children(func() {
				ui.Text(c, saveLabel).FontSize(13).Bold().TextColor(t.AccentText)
			})

			btnComplete := ui.Button(c, "完成本步").Radius(8).Padding(8, 16)
			if btnComplete.Clicked() {
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
		if ct.AssayID != "" {
			ui.Text(c, "Assay "+ct.AssayID).FontSize(11).TextColor(t.TextMuted)
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

		// Checklist Items
		ui.Column(c).Gap(8).Children(func() {
			ui.Text(c, "检查项").FontSize(11).TextColor(t.TextMuted).Bold()
			if len(w.View.Checks) == 0 {
				ui.Text(c, "尚无检查项").FontSize(12).TextColor(t.TextMuted)
			} else {
				for _, ch := range w.View.Checks {
					label := ch.Title + ": "
					switch ch.Status {
					case "ready", "completed", "passed":
						label += "就绪"
					case "blocked", "failed", "invalid":
						label += "未通过"
					default:
						label += "待检查"
					}
					if ch.Binding != "" {
						label += " · " + ch.Binding
					}
					ui.Text(c, label).FontSize(12).TextColor(t.TextMuted)
				}
			}
		})

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
