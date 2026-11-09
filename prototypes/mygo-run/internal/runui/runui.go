package runui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"
)

// State is the real UI state; never fake success.
type State string

const (
	StateLoading          State = "loading"
	StateReady            State = "ready"
	StateSaving           State = "saving"
	StateSaved            State = "saved"
	StateValidationError  State = "validation_error"
	StateBlocked          State = "blocked"
	StateEmpty            State = "empty"
)

type Task struct {
	Key       string
	Title     string
	Timepoint string
	Status    string
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
	CompleteTask  func(taskKey string)
	SelectTask   func(taskKey string)
}

type Workspace struct {
	View       RunView
	State      State
	Draft      Draft
	Callbacks  Callbacks
	Selected   string
	Width      int
	Narrow     bool
	Error      string
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
	if w.Width > 0 && w.Width < 900 {
		return true
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
	ui.Column(c).Fill().Background(t.Background).Children(func() {
		ui.Row(c).Padding(8, 12).Children(func() {
			ui.Text(c, w.View.Title).FontSize(20).Bold()
			ui.Spacer(c)
			ui.Text(c, fmt.Sprintf("%s · rev %d", w.View.Status, w.View.Revision)).TextColor(t.TextMuted)
			ui.Text(c, " · ").TextColor(t.TextMuted)
			ui.Text(c, string(w.State)).TextColor(t.TextMuted)
		})
		ui.Divider(c)
		if w.State == StateLoading {
			ui.Text(c, "加载中…").Padding(16)
			return
		}
		if w.State == StateEmpty {
			ui.Text(c, "空运行").Padding(16)
			return
		}
		if w.State == StateBlocked {
			ui.Text(c, "阻塞: "+w.Error).Padding(16)
			return
		}
		if w.isNarrow() {
			ui.Column(c).Grow(1).Gap(12).Padding(12).Children(func() {
				recordPanel(c, w, t)
				readinessPanel(c, w, t)
			})
		} else {
			ui.Grid(c).Grow(1).ColumnTracks(ui.Fr(2), ui.Fixed(280)).Gap(16).Padding(12).Children(func() {
				recordPanel(c, w, t)
				readinessPanel(c, w, t)
			})
		}
	})
}

func recordPanel(c *ui.Context, w *Workspace, t *ui.Theme) {
	ui.Column(c).Gap(12).Padding(16).Radius(10).Background(t.Surface).Border(1, t.Border).Children(func() {
		ct := w.currentTask()
		title := "当前任务"
		if ct != nil && ct.Title != "" {
			title = "当前任务 · " + ct.Title
		}
		ui.Text(c, title).FontSize(17).Bold()
		ui.Text(c, "具体操作条件继承已选 Protocol").TextColor(t.TextMuted)
		done, total := w.taskCounts()
		ui.Text(c, fmt.Sprintf("任务进度 %d/%d", done, total)).FontSize(12).TextColor(t.TextMuted)
		// task chips
		ui.Row(c).Gap(6).Children(func() {
			for i := range w.View.Tasks {
				task := w.View.Tasks[i]
				bg := t.Surface
				if task.Key == w.Selected {
					bg = ui.Hex("#5b45c9")
				}
				label := task.Title
				if task.Timepoint != "" {
					label = task.Timepoint + " " + label
				}
				if task.Done() {
					label += " ✓"
				}
				key := task.Key
				if ui.Button(c, label).Radius(7).Padding(6, 10).Background(bg).Clicked() {
					w.Selected = key
					if w.Callbacks.SelectTask != nil {
						w.Callbacks.SelectTask(key)
					}
				}
			}
		})
		ui.Divider(c)
		// measurement editor
		ui.Text(c, "记录 OD450（AU）").FontSize(15).Bold()
		ui.Grid(c).Columns(2).Gap(8).Children(func() {
			ui.Text(c, "样本 / 孔位").FontSize(11).TextColor(t.TextMuted)
			ui.Text(c, "测量值").FontSize(11).TextColor(t.TextMuted)
			for i := range w.Draft.Rows {
				i := i
				ui.TextInput(c, &w.Draft.Rows[i].SampleID)
				ui.TextInput(c, &w.Draft.Rows[i].Value)
			}
		})
		ui.Row(c).Gap(8).Children(func() {
			if ui.Button(c, "新增一行").Clicked() {
				w.Draft.Rows = append(w.Draft.Rows, DraftRow{})
			}
		})
		if w.State == StateValidationError || w.Draft.Error != "" {
			msg := w.Draft.Error
			if msg == "" {
				msg = w.Error
			}
			ui.Text(c, msg).TextColor(ui.Hex("#b00020"))
		}
		if w.State == StateSaved {
			ui.Text(c, "已保存").TextColor(ui.Hex("#2e7d32"))
		}
		ui.Row(c).Gap(8).Children(func() {
			saveLabel := "保存读数"
			if w.State == StateSaving {
				saveLabel = "保存中…"
			}
			if ui.PrimaryButton(c, saveLabel).Clicked() {
				w.save()
			}
			if ui.Button(c, "完成本步").Clicked() {
				w.complete()
			}
		})
	})
}

func readinessPanel(c *ui.Context, w *Workspace, t *ui.Theme) {
	ui.Column(c).Gap(10).Padding(16).Radius(10).Background(t.Surface).Border(1, t.Border).Children(func() {
		ui.Text(c, "就绪信息").FontSize(15).Bold()
		ui.Text(c, "材料 / 试剂").FontSize(11).TextColor(t.TextMuted)
		if len(w.View.Checks) == 0 {
			ui.Text(c, "未绑定")
		}
		for _, ch := range w.View.Checks {
			label := ch.Title
			if ch.Binding != "" {
				label += ": " + ch.Binding
			} else {
				label += ": 未绑定"
			}
			ui.Text(c, label)
		}
		ui.Text(c, "仪器 / 程序").FontSize(11).TextColor(t.TextMuted)
		ui.Text(c, "未绑定")
		ui.Text(c, "孔板").FontSize(11).TextColor(t.TextMuted)
		ui.Text(c, "未绑定")
		ui.Divider(c)
		ui.Text(c, "流程预览 · 示意").FontSize(15).Bold()
		flow(c, t)
		ui.Text(c, "96 孔板 · 示意").FontSize(15).Bold()
		plate(c, t)
		if len(w.View.Events) > 0 {
			ui.Divider(c)
			ui.Text(c, "事件").FontSize(12).Bold()
			for _, e := range w.View.Events {
				ui.Text(c, e.Kind+": "+e.Message).FontSize(11).TextColor(t.TextMuted)
			}
		}
	})
}

func flow(c *ui.Context, t *ui.Theme) {
	ui.Box(c).Height(120).Draw(func(p *ui.Painter, r ui.Rect) {
		nodes := []struct {
			x, y  float32
			label string
		}{
			{0.12, 0.35, "Seed"}, {0.38, 0.35, "Treat"}, {0.64, 0.35, "Read"}, {0.64, 0.75, "Stain"}, {0.88, 0.75, "Image"},
		}
		for i := 0; i < 4; i++ {
			x1 := r.X + r.W*nodes[i].x
			y1 := r.Y + r.H*nodes[i].y
			x2 := r.X + r.W*nodes[i+1].x
			y2 := r.Y + r.H*nodes[i+1].y
			p.Line(x1+12, y1, x2-12, y2, 1, t.Border)
		}
		for i, n := range nodes {
			x := r.X + r.W*n.x
			y := r.Y + r.H*n.y
			fill := t.Surface
			if i == 2 {
				fill = ui.Hex("#5b45c9")
			}
			p.Fill(ui.Rect{X: x - 32, Y: y - 16, W: 64, H: 32}, fill, 8)
			p.Stroke(ui.Rect{X: x - 32, Y: y - 16, W: 64, H: 32}, t.Border, 8, 1)
			p.Text(x-22, y-5, n.label, 11, t.Text)
		}
	})
}

func plate(c *ui.Context, t *ui.Theme) {
	ui.Grid(c).Columns(12).Gap(2).Children(func() {
		for i := 0; i < 96; i++ {
			fill := t.Background
			if i == 13 || i == 14 || i == 25 {
				fill = ui.Hex("#d7d0c2")
			}
			ui.Box(c).Height(10).Radius(2).Background(fill).Border(1, t.Border)
		}
	})
}
