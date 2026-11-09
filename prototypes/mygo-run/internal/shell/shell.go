// Package shell is the AsterLab natively drawn desktop frame.
package shell

import (
	"fmt"

	"asterlab-mygo-prototype/internal/theme"

	"github.com/egoist/mygo/ui"
)

type Page int

const (
	Design Page = iota
	Protocol
	Run
	Tools
)

func (p Page) ShortLabel() string {
	switch p {
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
	tokens := theme.GetTokens(t)
	bar := c.TitleBar()

	// Keyboard shortcuts
	if c.Shortcut(ui.Cmd, ui.KeyB) {
		s.SidebarOpen = !s.SidebarOpen
		if s.OnSidebar != nil {
			s.OnSidebar(s.SidebarOpen)
		}
	}
	pages := []Page{Design, Protocol, Run, Tools}
	pageKeys := []ui.Key{ui.Key1, ui.Key2, ui.Key3, ui.Key4}
	for i, k := range pageKeys {
		if c.Shortcut(ui.Cmd, k) {
			s.Page = pages[i]
			if s.OnPage != nil {
				s.OnPage(pages[i])
			}
		}
	}

	ui.Column(c).Fill().Background(t.Background).Children(func() {
		// Top Titlebar and navigation strip
		ui.Row(c).Height(max(bar.Height, 44)).Padding(0, bar.Right+16, 0, bar.Left+16).
			AlignItems(ui.Center).Gap(10).Background(tokens.RailBackground).Border(1, t.Border).DragWindow().Children(func() {
			ui.Text(c, "AsterLab").Bold().FontSize(13).TextColor(t.Text)
			ui.Text(c, "/").TextColor(t.TextMuted)
			ui.Text(c, s.Page.Label()).FontSize(13).TextColor(t.Text).Bold()
			ui.Spacer(c)
			if ui.Button(c, "侧栏").Radius(6).Padding(4, 10).Clicked() {
				s.SidebarOpen = !s.SidebarOpen
				if s.OnSidebar != nil {
					s.OnSidebar(s.SidebarOpen)
				}
			}
			if s.Project != "" {
				ui.Row(c).Padding(3, 8).Radius(6).Background(tokens.NeutralBg).Border(1, t.Border).Children(func() {
					ui.Text(c, s.Project).FontSize(11).TextColor(tokens.NeutralText).Bold()
				})
			}
		})
		ui.Row(c).Grow(1).Children(func() {
			// Left navigation rail
			ui.Column(c).Width(64).FillHeight().Padding(8, 6).Gap(8).Background(tokens.RailBackground).
				Border(1, t.Border).AlignItems(ui.Center).Children(func() {
				// App Logo Icon
				ui.Row(c).Size(38, 38).Radius(10).Background(t.Accent).Center().Children(func() {
					ui.Text(c, "AL").FontSize(14).Bold().TextColor(t.AccentText)
				})
				ui.Divider(c)
				for _, p := range pages {
					p := p
					active := p == s.Page
					bg := ui.Color{}
					txtColor := t.TextMuted
					if active {
						bg = t.Accent
						txtColor = t.AccentText
					}
					b := ui.ButtonBase(c).Width(48).Height(40).Radius(8).Background(bg)
					b.Children(func() {
						ui.Row(c).Fill().Center().Children(func() {
							ui.Text(c, p.ShortLabel()).FontSize(12).TextColor(txtColor).Bold()
						})
					})
					if b.Clicked() {
						s.Page = p
						if s.OnPage != nil {
							s.OnPage(p)
						}
					}
				}
			})
			// Sidebar
			if s.SidebarOpen {
				ui.Column(c).Width(230).FillHeight().Padding(12, 10).Gap(10).Background(tokens.SidebarBackground).
					Border(1, t.Border).Children(func() {
					ui.Text(c, "项目 / 实验设计").FontSize(11).TextColor(t.TextMuted).Bold()
					sel := s.Selected
					ui.Scroll(c).Grow(1).Children(func() {
						if ui.Sidebar(c, &sel, func() {
							ui.SidebarSection(c, s.Project, nil, func() {
								for _, d := range s.Designs {
									ui.SidebarItem(c, d.ID, nil, d.Title)
								}
							})
						}).Changed() {
							s.Selected = sel
							if s.OnSelect != nil {
								s.OnSelect(sel)
							}
						}
					})
					if len(s.Designs) > 0 {
						ui.Text(c, fmt.Sprintf("%d 个实验设计", len(s.Designs))).FontSize(11).TextColor(t.TextMuted)
					}
				})
			}
			// Main content area
			ui.Column(c).Grow(1).FillHeight().Background(t.Background).Children(func() {
				if s.Content != nil {
					s.Content(c)
				}
			})
		})
	})
}
