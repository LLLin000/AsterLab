package runui

import (
	"fmt"
	"strconv"
	"strings"

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
	CompleteTask func(taskKey string)
	SelectTask   func(taskKey string)
}

type Workspace struct {
	View      RunView
	State     State
	Draft     Draft
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
		ep := "OD450"
		unit := "AU"
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
	if c.Shortcut(ui.Cmd, ui.KeyN) {
		w.Draft.Rows = append(w.Draft.Rows, DraftRow{})
	}

	ui.Column(c).Fill().Background(t.Background).Children(func() {
		// Header bar with status and metadata
		ui.Row(c).Height(52).Padding(0, 16).AlignItems(ui.Center).Background(tokens.CardBackground).
			Border(1, tokens.CardBorder).Children(func() {
			ui.Text(c, w.View.Title).FontSize(18).Bold().TextColor(t.Text)
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
			if w.shouldStack(c) {
				ui.Column(c).FillWidth().Gap(16).Padding(16).Children(func() {
					recordPanel(c, w, t, tokens)
					plateCard(c, w, t, tokens)
					flowCard(c, t, tokens)
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
						flowCard(c, t, tokens)
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
	case StateValidationError, StateBlocked:
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

		// Measurement editor
		ui.Column(c).Gap(6).Children(func() {
			ui.Text(c, "记录 OD450（AU）").FontSize(15).Bold().TextColor(t.Text)
			ui.Text(c, "请填入待测样本标识与对应的光密度读数").FontSize(12).TextColor(t.TextMuted)
		})

		// Measurement Grid
		ui.Grid(c).Columns(3).ColumnTracks(ui.Fr(1), ui.Fr(1), ui.Fixed(36)).Gap(8).Children(func() {
			ui.Text(c, "样本 / 孔位").FontSize(11).TextColor(t.TextMuted).Bold()
			ui.Text(c, "测量值 (AU)").FontSize(11).TextColor(t.TextMuted).Bold()
			ui.Text(c, "").FontSize(11) // action header placeholder

			for i := range w.Draft.Rows {
				i := i
				inp1 := ui.TextInput(c, &w.Draft.Rows[i].SampleID).Placeholder("样本编号 (如 Control-1)")
				if inp1.Changed() && (w.State == StateSaved || w.State == StateReady) {
					w.State = StateDraft
				}
				inp2 := ui.TextInput(c, &w.Draft.Rows[i].Value).Placeholder("吸光度数值 (如 0.82)")
				if inp2.Changed() && (w.State == StateSaved || w.State == StateReady) {
					w.State = StateDraft
				}
				if len(w.Draft.Rows) > 1 {
					if ui.Button(c, "✕").Radius(6).Padding(4, 8).Clicked() {
						w.Draft.Rows = append(w.Draft.Rows[:i], w.Draft.Rows[i+1:]...)
						w.State = StateDraft
					}
				} else {
					ui.Spacer(c)
				}
			}
		})

		ui.Row(c).Gap(8).Children(func() {
			if ui.Button(c, "+ 新增一行 (Ctrl+N)").Radius(6).Padding(6, 12).Clicked() {
				w.Draft.Rows = append(w.Draft.Rows, DraftRow{})
				w.State = StateDraft
			}
		})

		// Notifications
		if w.State == StateValidationError || w.Draft.Error != "" {
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
				ui.Text(c, "✓ 读数已成功持久化至实验记录").FontSize(12).TextColor(tokens.SuccessText).Bold()
			})
		}

		// Action buttons
		ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
			saveLabel := "保存读数 (Ctrl+S)"
			if w.State == StateSaving {
				saveLabel = "保存中…"
			}
			btnSave := ui.ButtonBase(c).Radius(8).Padding(8, 20).Background(t.Accent)
			btnSave.Children(func() {
				ui.Text(c, saveLabel).FontSize(13).Bold().TextColor(t.AccentText)
			})
			if btnSave.Clicked() {
				w.save()
			}

			btnComplete := ui.Button(c, "完成本步").Radius(8).Padding(8, 16)
			if btnComplete.Clicked() {
				w.complete()
			}
		})
	})
}

func plateCard(c *ui.Context, w *Workspace, t *ui.Theme, tokens theme.Tokens) {
	ui.Column(c).Gap(12).Padding(20).Radius(12).Background(tokens.CardBackground).Border(1, tokens.CardBorder).Children(func() {
		ui.Row(c).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "96 孔板布局").FontSize(15).Bold().TextColor(t.Text)
			ui.Spacer(c)
			ui.Text(c, "8 × 12 标准微孔阵列").FontSize(11).TextColor(t.TextMuted)
		})

		// Column headers 1-12
		ui.Grid(c).Columns(12).Gap(4).Children(func() {
			for col := range 12 {
				ui.Row(c).Center().Children(func() {
					ui.Text(c, strconv.Itoa(col+1)).FontSize(9).TextColor(t.TextMuted)
				})
			}
		})

		// 96-well grid with 18px minimum cell size
		ui.Grid(c).Columns(12).Gap(4).Children(func() {
			for i := range 96 {
				fill := tokens.NeutralBg
				borderCol := t.Border
				if i == 13 || i == 14 || i == 25 {
					fill = t.Accent
					borderCol = t.Accent
				}
				ui.Box(c).Height(18).Radius(4).Background(fill).Border(1, borderCol)
			}
		})

		// Legend
		ui.Row(c).Padding(4, 0).AlignItems(ui.Center).Gap(14).Children(func() {
			ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
				ui.Box(c).Size(10, 10).Radius(2).Background(t.Accent)
				ui.Text(c, "待测 / 已加样 (3)").FontSize(11).TextColor(t.Accent).Bold()
			})
			ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
				ui.Box(c).Size(10, 10).Radius(2).Background(tokens.NeutralBg).Border(1, t.Border)
				ui.Text(c, "空白微孔 (93)").FontSize(11).TextColor(t.TextMuted)
			})
		})
	})
}

func flowCard(c *ui.Context, t *ui.Theme, tokens theme.Tokens) {
	ui.Column(c).Gap(12).Padding(20).Radius(12).Background(tokens.CardBackground).Border(1, tokens.CardBorder).Children(func() {
		ui.Text(c, "实验流程预览").FontSize(15).Bold().TextColor(t.Text)
		ui.Box(c).Height(120).Draw(func(p *ui.Painter, r ui.Rect) {
			nodes := []struct {
				x, y  float32
				label string
			}{
				{0.12, 0.35, "Seed"}, {0.38, 0.35, "Treat"}, {0.64, 0.35, "Read"}, {0.64, 0.75, "Stain"}, {0.88, 0.75, "Image"},
			}
			for i := range 4 {
				x1 := r.X + r.W*nodes[i].x
				y1 := r.Y + r.H*nodes[i].y
				x2 := r.X + r.W*nodes[i+1].x
				y2 := r.Y + r.H*nodes[i+1].y
				p.Line(x1+16, y1, x2-16, y2, 1, t.Border)
			}
			for i, n := range nodes {
				x := r.X + r.W*n.x
				y := r.Y + r.H*n.y
				fill := tokens.CardBackground
				txtCol := t.Text
				if i == 2 {
					fill = t.Accent
					txtCol = t.AccentText
				}
				boxRect := ui.Rect{X: x - 32, Y: y - 16, W: 64, H: 32}
				p.Fill(boxRect, fill, 8)
				p.Stroke(boxRect, t.Border, 8, 1)
				p.Text(x-20, y-5, n.label, 11, txtCol)
			}
		})
	})
}

func readinessCard(c *ui.Context, w *Workspace, t *ui.Theme, tokens theme.Tokens) {
	ui.Column(c).Gap(14).Padding(20).Radius(12).Background(tokens.CardBackground).Border(1, tokens.CardBorder).Children(func() {
		ui.Text(c, "实验就绪检查").FontSize(15).Bold().TextColor(t.Text)

		// Checklist Items
		ui.Column(c).Gap(8).Children(func() {
			ui.Text(c, "材料 / 试剂").FontSize(11).TextColor(t.TextMuted).Bold()
			if len(w.View.Checks) == 0 {
				ui.Row(c).Padding(4, 8).Radius(6).Background(tokens.NeutralBg).Children(func() {
					ui.Text(c, "✓ GelMA · 水凝胶基质 (就绪)").FontSize(12).TextColor(tokens.NeutralText)
				})
			} else {
				for _, ch := range w.View.Checks {
					label := ch.Title
					if ch.Binding != "" {
						label += ": " + ch.Binding + " ✓"
					} else {
						label += ": 待配置"
					}
					ui.Row(c).Padding(4, 8).Radius(6).Background(tokens.NeutralBg).Children(func() {
						ui.Text(c, label).FontSize(12).TextColor(tokens.NeutralText)
					})
				}
			}

			ui.Text(c, "仪器 / 检测").FontSize(11).TextColor(t.TextMuted).Bold()
			ui.Row(c).Padding(4, 8).Radius(6).Background(tokens.NeutralBg).Children(func() {
				ui.Text(c, "✓ 多功能微孔板检测仪 (450 nm)").FontSize(12).TextColor(tokens.NeutralText)
			})

			ui.Text(c, "耗材规格").FontSize(11).TextColor(t.TextMuted).Bold()
			ui.Row(c).Padding(4, 8).Radius(6).Background(tokens.NeutralBg).Children(func() {
				ui.Text(c, "✓ 96 孔透明微孔培养板").FontSize(12).TextColor(tokens.NeutralText)
			})
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
