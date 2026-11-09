// Package shell is the AsterLab natively drawn desktop frame.
package shell

import (
	"asterlab-mygo-prototype/internal/theme"
	"github.com/egoist/mygo/ui"
)

type Page int

const (
	Project Page = iota
	Design
	Protocol
	Run
	Tools
)

func (p Page) ShortLabel() string {
	switch p {
	case Project:
		return "项目"
	case Design:
		return "设计"
	case Protocol:
		return "方案"
	case Run:
		return "运行"
	default:
		return "工具"
	}
}

func (p Page) Label() string {
	switch p {
	case Project:
		return "项目工作区"
	case Design:
		return "实验设计"
	case Protocol:
		return "实验方案"
	case Run:
		return "实验运行"
	default:
		return "资料与工具"
	}
}

type DesignItem struct{ ID, Title, Meta string }
type RecentRun struct{ ID, Title, Meta string }

type Shell struct {
	Project     string
	Projects    []string
	Designs     []DesignItem
	RecentRuns  []RecentRun
	Selected    string
	Page        Page
	SidebarOpen bool
	Loading     bool
	Error       string
	OnSelect    func(string)
	OnProject   func(string)
	OnRunSelect func(string)
	OnPage      func(Page)
	OnSidebar   func(bool)
	OnContinue  func()
	OnCreate    func()
	OnImport    func()
	Content     func(*ui.Context)
}

func View(c *ui.Context, s *Shell) {
	t := c.Theme()
	tok := theme.GetTokens(t)
	width, _ := c.Size()
	if c.Shortcut(ui.Cmd, ui.KeyB) {
		s.SidebarOpen = !s.SidebarOpen
		if s.OnSidebar != nil {
			s.OnSidebar(s.SidebarOpen)
		}
	}
	pages := []Page{Project, Design, Protocol, Run, Tools}
	for i, key := range []ui.Key{ui.Key0, ui.Key1, ui.Key2, ui.Key3, ui.Key4} {
		if c.Shortcut(ui.Cmd, key) {
			s.Page = pages[i]
			if s.OnPage != nil {
				s.OnPage(pages[i])
			}
		}
	}
	ui.Column(c).Fill().Background(tok.Canvas).Children(func() {
		ui.Row(c).Height(48).Padding(0, 16).AlignItems(ui.Center).Gap(10).Background(tok.Surface).Border(1, tok.Border).DragWindow().Children(func() {
			ui.Text(c, "AsterLab").Bold().FontSize(13).TextColor(tok.Text)
			ui.Text(c, "/").TextColor(tok.TextMuted)
			ui.Text(c, s.Project).FontSize(13).TextColor(tok.Text)
			ui.Text(c, "/").TextColor(tok.TextMuted)
			ui.Text(c, s.Page.Label()).FontSize(13).TextColor(tok.Text).Bold()
			ui.Spacer(c)
			if s.Loading {
				ui.Text(c, "加载中…").FontSize(12).TextColor(tok.TextMuted)
			}
			if s.Error != "" {
				ui.Text(c, s.Error).FontSize(12).TextColor(tok.Danger)
			}
			if ui.Button(c, "侧栏").Radius(float32(tok.ControlRadius)).Padding(4, 10).Clicked() {
				s.SidebarOpen = !s.SidebarOpen
				if s.OnSidebar != nil {
					s.OnSidebar(s.SidebarOpen)
				}
			}
		})
		ui.Row(c).Grow(1).Children(func() {
			ui.Column(c).Width(56).FillHeight().Padding(8, 4).Gap(8).Background(tok.RailBackground).Border(1, tok.Border).AlignItems(ui.Center).Children(func() {
				ui.Row(c).Size(38, 38).Radius(float32(tok.PanelRadius)).Background(tok.Accent).Center().Children(func() { ui.Text(c, "AL").FontSize(14).Bold().TextColor(tok.AccentText) })
				ui.Divider(c)
				for _, p := range pages {
					p := p
					bg, color := ui.Color{}, tok.TextMuted
					if p == s.Page {
						bg, color = tok.Accent, tok.AccentText
					}
					b := ui.ButtonBase(c).Width(48).Height(40).Radius(float32(tok.PanelRadius)).Background(bg)
					b.Children(func() {
						ui.Row(c).Fill().Center().Children(func() { ui.Text(c, p.ShortLabel()).FontSize(11).TextColor(color).Bold() })
					})
					if b.Clicked() {
						s.Page = p
						if s.OnPage != nil {
							s.OnPage(p)
						}
					}
				}
			})
			if s.SidebarOpen && s.Page != Project && width > 900 {
				ui.Column(c).Width(220).FillHeight().Padding(12, 10).Gap(10).Background(tok.SidebarBackground).Border(1, tok.Border).Children(func() {
					ui.Text(c, "最近设计").FontSize(11).TextColor(tok.TextMuted).Bold()
					ui.Scroll(c).Grow(1).Children(func() {
						for _, d := range s.Designs {
							d := d
							if ui.Button(c, d.Title).Clicked() {
								s.Selected = d.ID
								if s.OnSelect != nil {
									s.OnSelect(d.ID)
								}
							}
						}
					})
				})
			}
			ui.Column(c).Grow(1).FillHeight().Background(tok.Canvas).Children(func() {
				if s.Page == Project {
					projectView(c, s, tok)
				} else if s.Content != nil {
					s.Content(c)
				}
			})
		})
	})
}

func projectView(c *ui.Context, s *Shell, tok theme.Tokens) {
	ui.Scroll(c).Grow(1).Children(func() {
		ui.Column(c).Padding(24).Gap(16).Children(func() {
			ui.Text(c, "项目工作区").FontSize(20).Bold().TextColor(tok.Text)
			if s.Project != "" {
				ui.Text(c, s.Project).FontSize(15).TextColor(tok.Text)
			} else {
				ui.Text(c, "选择或创建项目").FontSize(13).TextColor(tok.TextMuted)
			}
			ui.Row(c).Gap(8).Children(func() {
				if s.OnContinue != nil && ui.Button(c, "继续上次工作").Clicked() {
					s.OnContinue()
				}
				if s.OnCreate != nil && ui.Button(c, "新建设计").Clicked() {
					s.OnCreate()
				}
				if s.OnImport != nil && ui.Button(c, "导入").Clicked() {
					s.OnImport()
				}
			})
			ui.Text(c, "项目").FontSize(15).Bold().TextColor(tok.Text)
			if len(s.Projects) == 0 {
				ui.Text(c, "暂无项目").FontSize(13).TextColor(tok.TextMuted)
			}
			for _, p := range s.Projects {
				p := p
				if ui.Button(c, p).Clicked() && s.OnProject != nil {
					s.Project = p
					s.OnProject(p)
				}
			}
			ui.Text(c, "最近设计").FontSize(15).Bold().TextColor(tok.Text)
			if len(s.Designs) == 0 {
				ui.Text(c, "暂无设计").FontSize(13).TextColor(tok.TextMuted)
			}
			for _, d := range s.Designs {
				d := d
				if ui.Button(c, d.Title+"  "+d.Meta).Clicked() {
					s.Selected = d.ID
					s.Page = Design
					if s.OnSelect != nil {
						s.OnSelect(d.ID)
					}
					if s.OnPage != nil {
						s.OnPage(Design)
					}
				}
			}
			ui.Text(c, "最近运行").FontSize(15).Bold().TextColor(tok.Text)
			if len(s.RecentRuns) == 0 {
				ui.Text(c, "暂无运行").FontSize(13).TextColor(tok.TextMuted)
			}
			for _, r := range s.RecentRuns {
				r := r
				if ui.Button(c, r.Title+"  "+r.Meta).Clicked() && s.OnRunSelect != nil {
					s.OnRunSelect(r.ID)
				}
			}
			if s.Loading {
				ui.Text(c, "正在加载项目…").TextColor(tok.TextMuted)
			}
			if s.Error != "" {
				ui.Text(c, s.Error).TextColor(tok.Danger)
			}
		})
	})
}
