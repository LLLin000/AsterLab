// Package shell is the AsterLab natively drawn desktop frame.
package shell

import (
	"github.com/egoist/mygo/ui"
)

type Page int

const (
	Design Page = iota
	Protocol
	Run
	Tools
)

func (p Page) Label() string {
	switch p {
	case Design:
		return "实验设计"
	case Protocol:
		return "Protocol"
	case Run:
		return "实验运行"
	default:
		return "资料与工具"
	}
}

type DesignItem struct {
	ID    string
	Title string
	Meta  string
}

type Shell struct {
	Project     string
	Designs     []DesignItem
	Selected    string
	Page        Page
	SidebarOpen bool
	OnSelect    func(designID string)
	OnPage      func(Page)
	OnSidebar   func(bool)
	Content     func(*ui.Context)
}

func View(c *ui.Context, s *Shell) {
	t := c.Theme()
	bar := c.TitleBar()
	ui.Column(c).Fill().Background(t.Background).Children(func() {
		// The native frame is hidden; this band is the window drag region.
		ui.Row(c).Height(max(bar.Height, 40)).Padding(0, bar.Right+12, 0, bar.Left+12).
			AlignItems(ui.Center).Gap(10).DragWindow().Children(func() {
			ui.Text(c, "AsterLab").Bold()
			ui.Text(c, "/").TextColor(t.TextMuted)
			ui.Text(c, s.Page.Label()).TextColor(t.TextMuted)
			ui.Spacer(c)
			if ui.Button(c, "侧栏").Clicked() {
				s.SidebarOpen = !s.SidebarOpen
				if s.OnSidebar != nil {
					s.OnSidebar(s.SidebarOpen)
				}
			}
			ui.Text(c, s.Project).TextColor(t.TextMuted)
		})
		ui.Divider(c)
		ui.Row(c).Grow(1).Children(func() {
			ui.Column(c).Width(52).FillHeight().Padding(6).Gap(4).Background(t.Background).Children(func() {
				ui.Text(c, "AL").FontSize(18).Bold()
				for _, p := range []Page{Design, Protocol, Run, Tools} {
					p := p
					label := p.Label()
					if ui.Button(c, label).Radius(7).Clicked() {
						s.Page = p
						if s.OnPage != nil {
							s.OnPage(p)
						}
					}
				}
			})
			if s.SidebarOpen {
				ui.Column(c).Width(230).FillHeight().Padding(10).Gap(8).Background(t.Surface).Children(func() {
					ui.Text(c, "项目 / 实验设计").FontSize(12).TextColor(t.TextMuted)
					sel := s.Selected
					if ui.Sidebar(c, &sel, func() {
						ui.SidebarSection(c, s.Project, nil, func() {
							for _, d := range s.Designs {
								ui.SidebarItem(c, d.ID, nil, d.Title)
							}
						})
					}).Grow(1).Changed() {
						s.Selected = sel
						if s.OnSelect != nil {
							s.OnSelect(sel)
						}
					}
					ui.Text(c, "2 次执行 · 0 项待确认").FontSize(11).TextColor(t.TextMuted)
				})
			}
			ui.Column(c).Grow(1).FillHeight().Children(func() {
				if s.Content != nil {
					s.Content(c)
				}
			})
		})
	})
}
