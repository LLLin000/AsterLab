package main

import (
	"context"
	"log"
	"math"
	"os"
	"strconv"

	"asterlab-mygo-prototype/modules/runs"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

type sample struct{ id, value string }
type app struct {
	page      *string
	step      *string
	samples   []sample
	checked   bool
	plateWell *int
	svc       runs.RunService
	snap      runs.RunSnapshot
	err       string
}

func (a *app) save() {
	if a.svc == nil {
		a.err = "Rust RunService 不可用"
		return
	}
	ms := make([]runs.Measurement, 0, len(a.samples))
	for _, r := range a.samples {
		v, err := strconv.ParseFloat(r.value, 64)
		if err != nil || !isFinite(v) {
			continue
		}
		if r.id == "" {
			a.err = "样本 / 孔位不能为空"
			return
		}
		ms = append(ms, runs.Measurement{SampleID: r.id, Endpoint: "OD450", Value: v, Unit: "AU"})
	}
	if len(ms) == 0 {
		a.err = "至少填写一条有效数值"
		return
	}
	snap, err := a.svc.SaveReadout(context.Background(), runs.ReadoutInput{RunID: a.snap.RunID, Measurements: ms})
	if err != nil {
		a.err = err.Error()
		return
	}
	a.snap = snap
	a.err = "已保存"
}

func isFinite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }

func (a *app) view(c *ui.Context) {
	t := c.Theme()
	bar := c.TitleBar()
	ui.Column(c).Fill().Background(t.Background).Children(func() {
		// Custom title bar: no Windows chrome.
		ui.Row(c).Height(max(bar.Height, 40)).Padding(0, bar.Right+12, 0, bar.Left+14).
			DragWindow().Children(func() {
			ui.Text(c, "AsterLab").Bold()
			ui.Text(c, "/").TextColor(t.TextMuted)
			ui.Text(c, "实验运行").TextColor(t.TextMuted)
			ui.Spacer(c)
			ui.Text(c, a.snap.Title+" · 第 2 次运行").TextColor(t.TextMuted)
		})
		ui.Divider(c)
		ui.Row(c).Grow(1).Children(func() {
			a.sidebar(c)
			a.workspace(c)
		})
	})
}

func (a *app) sidebar(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Width(240).FillHeight().Padding(10).Gap(8).Background(ui.Hex("#f1efea")).Children(func() {
		ui.Text(c, "项目 / 实验设计").FontSize(12).TextColor(t.TextMuted)
		ui.Sidebar(c, a.step, func() {
			ui.SidebarSection(c, "L929 GelMA/ZnO biocompatibility", nil, func() {
				ui.SidebarItem(c, "design-l929", nil, "GelMA/ZnO L929 生物相容性")
				ui.SidebarItem(c, "design-timed", nil, "Timed design")
				ui.SidebarItem(c, "design-wb", nil, "p-AKT Western blot")
			})
			ui.SidebarSection(c, "Second project", nil, func() {
				ui.SidebarItem(c, "design-elisa", nil, "ELISA design")
				ui.SidebarItem(c, "design-cck", nil, "CCK-8 设计工作区")
			})
		}).Grow(1)
		ui.Text(c, "2 次执行 · 0 项待确认").FontSize(12).TextColor(t.TextMuted)
	})
}

func (a *app) workspace(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Grow(1).FillHeight().Padding(18).Gap(14).Children(func() {
		ui.Row(c).Children(func() {
			ui.Text(c, a.snap.Title).FontSize(22).Bold()
			ui.Spacer(c)
			ui.Text(c, a.snap.Status+" · D3").TextColor(t.TextMuted)
		})
		a.schedule(c)
		ui.Grid(c).Grow(1).ColumnTracks(ui.Fr(2), ui.Fixed(280)).Gap(16).Children(func() {
			a.record(c)
			a.ready(c)
		})
	})
}

func (a *app) schedule(c *ui.Context) {
	t := c.Theme()
	ui.Row(c).Gap(8).Children(func() {
		for _, d := range []string{"D1", "D3", "D7"} {
			d := d
			bg := t.Surface
			if d == "D3" {
				bg = ui.Hex("#5b45c9")
			}
			ui.Button(c, d+"  2/3").Radius(7).Padding(8, 12).Background(bg)
		}
	})
}

func (a *app) record(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Gap(12).Padding(16).Radius(10).Background(t.Surface).Border(1, t.Border).Children(func() {
		ui.Text(c, "当前任务 · CCK-8 读数").FontSize(17).Bold()
		ui.Text(c, "具体操作条件继承已选 Protocol").TextColor(t.TextMuted)
		ui.Row(c).Gap(10).Children(func() {
			if ui.Checkbox(c, &a.checked, "确认空白孔设置").Changed() {
			}
			ui.Text(c, "必需").FontSize(11).TextColor(t.TextMuted)
		})
		ui.Divider(c)
		ui.Text(c, "记录 OD450（AU）").FontSize(15).Bold()
		ui.Grid(c).Columns(2).Gap(8).Children(func() {
			ui.Text(c, "样本 / 孔位").FontSize(11).TextColor(t.TextMuted)
			ui.Text(c, "测量值").FontSize(11).TextColor(t.TextMuted)
			for i := range a.samples {
				i := i
				ui.TextInput(c, &a.samples[i].id)
				ui.TextInput(c, &a.samples[i].value)
			}
		})
		ui.Row(c).Gap(8).Children(func() {
			if ui.PrimaryButton(c, "保存读数").Clicked() {
			}
			if ui.Button(c, "完成本步").Clicked() {
			}
		})
	})
}

func (a *app) ready(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Gap(10).Padding(16).Radius(10).Background(t.Surface).Border(1, t.Border).Children(func() {
		ui.Text(c, "就绪信息").FontSize(15).Bold()
		ui.Text(c, "材料 / 试剂").FontSize(11).TextColor(t.TextMuted)
		ui.Text(c, "尚未绑定")
		ui.Text(c, a.err)
		ui.Text(c, "仪器 / 程序").FontSize(11).TextColor(t.TextMuted)
		ui.Text(c, "尚未绑定")
		ui.Text(c, "孔板").FontSize(11).TextColor(t.TextMuted)
		ui.Text(c, "尚未绑定")
		ui.Divider(c)
		ui.Text(c, "流程预览").FontSize(15).Bold()
		a.flow(c)
		ui.Text(c, "96 孔板").FontSize(15).Bold()
		a.plate(c)
	})
}

func (a *app) flow(c *ui.Context) {
	t := c.Theme()
	ui.Box(c).Height(145).Draw(func(p *ui.Painter, r ui.Rect) {
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

func (a *app) plate(c *ui.Context) {
	t := c.Theme()
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

func main() {
	page, step := "run", "design-l929"
	svc, err := runs.NewRunService()
	if err != nil {
		log.Fatal(err)
	}
	runID := os.Getenv("ASTERLAB_RUN_ID")
	if runID == "" {
		runID = "1c0c8989-0513-483a-851b-bdc7d6c67483"
	}
	a := &app{page: &page, step: &step, samples: []sample{{"Control-1", ""}, {"E1-1", ""}}, svc: svc, snap: runs.RunSnapshot{Title: "L929 生物相容性", Status: "加载中"}}
	mygo.App.WhenReady(func() {
		go func() {
			if snap, err := svc.LoadRun(context.Background(), runID); err == nil {
				a.snap = snap
			}
		}()
		mygo.NewWindow(mygo.WindowOptions{
			Title: "AsterLab MyGo Prototype", Width: 1200, Height: 760,
			TitleBarStyle: mygo.TitleBarHidden,
			Content:       ui.View(a.view),
		})
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
