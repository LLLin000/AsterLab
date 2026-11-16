// Package shell is the AsterLab natively drawn desktop frame: an Onevra
// sidebar window whose navigation, page header and pages are AsterLab's.
package shell

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/onevra/source/components"
	"github.com/oneo-me/onevra/source/icons/lucide"
	"github.com/oneo-me/onevra/source/shells/sidebar"

	"asterlab-mygo-prototype/internal/theme"
	"asterlab-mygo-prototype/modules/runs"
)

type Page int

const (
	Project Page = iota
	Design
	Protocol
	Run
	Notebook
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
	case Notebook:
		return "笔记"
	case Tools:
		return "工具"
	}
	return "项目"
}

// PageName is the persisted session name of the current page.
func (s *Shell) PageName() string {
	switch s.Page {
	case Design:
		return "design"
	case Protocol:
		return "protocol"
	case Run:
		return "run"
	case Notebook:
		return "notebook"
	case Tools:
		return "tools"
	}
	return "project"
}

func (p Page) Label() string {
	switch p {
	case Design:
		return "实验设计"
	case Protocol:
		return "实验方案"
	case Run:
		return "实验运行"
	case Notebook:
		return "Notebook"
	case Tools:
		return "资料与工具"
	}
	return "项目工作区"
}

// icon is the page's navigation icon.
func (p Page) icon() *ui.SVG {
	switch p {
	case Design:
		return lucide.FlaskConical()
	case Protocol:
		return lucide.BookOpen()
	case Run:
		return lucide.Play()
	case Notebook:
		return lucide.NotebookPen()
	case Tools:
		return lucide.Boxes()
	}
	return lucide.FolderKanban()
}

type DesignItem struct{ ID, Title, Meta string }
type RecentRun struct{ ID, Title, Meta string }

// Shell owns AsterLab's window-level state: the page, the selected project and
// design, and the window Onevra draws.
type Shell struct {
	Project    string
	Projects   []string
	Designs    []DesignItem
	RecentRuns []RecentRun
	Selected   string
	Page       Page
	Loading    bool
	Error      string
	// ActiveTimers feeds the persistent cross-page timer strip. Empty means
	// no strip at all: no placeholder, no reserved space. UX-LAYOUT, UX-DESIGN.
	ActiveTimers  []runs.ActiveTimer
	OnTimerSelect func(runID string)
	OnSelect      func(string)
	OnProject     func(string)
	OnRunSelect   func(string)
	OnPage        func(Page)
	OnSidebar     func(bool)

	Query      string
	OnContinue func()
	OnCreate   func()
	OnImport   func()
	Content    func(*ui.Context)

	window *sidebar.Shell
}

var pages = []Page{Project, Design, Protocol, Run, Notebook, Tools}

// NewWindow creates the AsterLab window. The Onevra shell owns the title bar,
// the floating header and the resizable navigation column.
func (s *Shell) NewWindow() *sidebar.Shell {
	s.window = sidebar.NewShell(sidebar.ShellOptions{
		Title:        "AsterLab",
		Width:        1360,
		Height:       880,
		MinWidth:     900,
		MinHeight:    600,
		StateKey:     "asterlab-desk",
		Theme:        theme.Window,
		LeftWidth:    232,
		Left:         s.navigation,
		Content:      s.page,
		ContentTitle: s.breadcrumb,
		LeftHeader:   s.navigationHeader,
		LeftToolbar:  s.navigationFooter,
	})
	return s.window
}

// Invalidate asks the window for a frame after state changed off the UI thread.
func (s *Shell) Invalidate() {
	if s.window != nil {
		s.window.Window().Invalidate()
	}
}

// breadcrumb is the trimmed top bar: the window title already says AsterLab,
// so only the location and live states render here. UX-P1, UX-LAYOUT, UX-DESIGN.
func (s *Shell) breadcrumb(c *ui.Context) {
	t := c.Theme()
	components.Text(c, s.location()).TextColor(t.TextMuted).SingleLine().Ellipsis("…")
	ui.Spacer(c)
	if s.Loading {
		components.Text(c, "加载中…").TextColor(t.TextMuted).FontSize(t.Rem(.85))
	}
	if s.Error != "" {
		components.Text(c, s.Error).TextColor(t.Danger).FontSize(t.Rem(.85)).SingleLine().Ellipsis("…").MaxWidth(420)
	}
}

// location names the project, or the design being edited, after AsterLab.
func (s *Shell) location() string {
	if s.Page == Design && s.Selected != "" {
		for _, d := range s.Designs {
			if d.ID == s.Selected {
				return d.Title
			}
		}
	}
	if s.Project != "" {
		return s.Project
	}
	return "未选择项目"
}

func (s *Shell) navigationHeader(c *ui.Context) {
	ui.Spacer(c)
	if s.OnCreate == nil {
		return
	}
	if components.IconButton(c, lucide.SquarePen(), "新建设计", components.IconButtonCompact).Clicked() {
		s.OnCreate()
	}
}

func (s *Shell) navigation(c *ui.Context) {
	inset := sidebar.HeaderHeight(c)
	ui.Column(c).Fill().Children(func() {
		ui.Scroll(c).Grow(1).Padding(inset, 0, 0, 0).Children(func() {
			components.Sidebar(c).Children(func() {
				components.SidebarSection(c, "工作区", func() {
					for _, p := range pages {
						p := p
						if components.SidebarItem(c, p.Label(), p.icon(), s.Page == p).Clicked() {
							s.setPage(p)
						}
					}
				})
				components.SidebarSectionWithActions(c, "最近设计", nil, func() {
					if len(s.Designs) == 0 {
						components.Text(c, "尚无已保存的设计").TextColor(c.Theme().TextMuted).
							FontSize(c.Theme().Rem(.85)).Padding(c.Theme().Space(1), c.Theme().Space(2))
						return
					}
					for _, d := range s.Designs {
						d := d
						item := components.SidebarItemWithSlots(c, d.Title, s.Selected == d.ID,
							func(c *ui.Context, size float32) {
								components.Icon(c, lucide.FileText()).Size(size, size).TextColor(c.Theme().Accent)
							},
							func(c *ui.Context, _ float32) {
								components.Text(c, d.Meta).TextColor(c.Theme().TextMuted).FontSize(c.Theme().Rem(.78)).Shrink(0)
							})
						if item.Clicked() {
							s.Selected = d.ID
							s.Page = Design
							if s.OnSelect != nil {
								s.OnSelect(d.ID)
							}
							s.setPage(Design)
						}
					}
				})
			})
		})
	})
}

func (s *Shell) navigationFooter(c *ui.Context) {
	t := c.Theme()
	components.Text(c, "AsterLab · 原生设计工作台").TextColor(t.TextMuted).FontSize(t.Rem(.78)).
		SingleLine().Ellipsis("…")
}

func (s *Shell) setPage(p Page) {
	changed := s.Page != p
	s.Page = p
	if s.OnPage != nil {
		s.OnPage(p)
	}
	if changed && s.window != nil {
		s.window.Window().Invalidate()
	}
}

func (s *Shell) page(c *ui.Context) {
	t := c.Theme()
	inset := sidebar.HeaderHeight(c)
	if c.Shortcut(ui.Cmd, ui.KeyB) {
		if s.window != nil {
			s.window.SetLeftExpanded(!s.window.LeftExpanded())
		}
	}
	for i, key := range []ui.Key{ui.Key1, ui.Key2, ui.Key3, ui.Key4, ui.Key5, ui.Key6} {
		if c.Shortcut(ui.Cmd, key) {
			s.setPage(pages[i])
		}
	}
	padding := t.Space(3)
	ui.Column(c).Fill().Padding(inset, 0, 0, 0).Background(t.Background).Children(func() {
		// UX-LAYOUT / UX-DESIGN: the global timer strip sits above every page
		// and renders nothing at all when no timer runs — no placeholder, no
		// reserved space.
		s.timerStrip(c)
		ui.Column(c).Grow(1).MinHeight(0).Padding(0, padding, padding, padding).Children(func() {
			if s.Error != "" {
				// UX-P5 / UX-STATE / UX-DONE: keep a failed write visible on every surface, with cause and recovery guidance.
				ui.Column(c).FillWidth().Padding(t.Space(1), t.Space(2)).Gap(t.Space(.5)).
					Background(t.Surface).Children(func() {
					components.Text(c, "未保存 · "+s.Error).TextColor(t.Danger)
					components.Text(c, "检查目标目录权限后，返回当前操作并重试保存。").
						TextColor(t.TextMuted).FontSize(t.Rem(.8))
				})
			}
			if s.Page == Project {
				s.projectView(c)
				return
			}
			if s.Content != nil {
				s.Content(c)
			}
		})
	})
}

// timerStrip is the persistent cross-page global timer bar. It shows running
// timers only — across experiments, countdowns and windows — and renders
// nothing when the list is empty. Actions use components.Button Ghost, the
// quiet one-action control; all values come from theme tokens. UX-IA-3,
// UX-P3, UX-DESIGN.
func (s *Shell) timerStrip(c *ui.Context) {
	if len(s.ActiveTimers) == 0 {
		return
	}
	t := c.Theme()
	ui.Row(c).FillWidth().Padding(t.Space(1), t.Space(2)).Gap(t.Space(2)).
		Background(t.Surface).Children(func() {
		components.Text(c, "计时中").TextColor(t.TextMuted).FontSize(t.Rem(.8))
		for _, timer := range s.ActiveTimers {
			timer := timer
			label := timer.RunTitle + " · " + formatTimerClock(timer)
			if components.Button(c, label, components.ButtonGhost).Clicked() && s.OnTimerSelect != nil {
				s.OnTimerSelect(timer.RunID)
			}
		}
	})
}

// formatTimerClock renders one running timer: remaining time for a countdown,
// elapsed time for a stopwatch with no planned duration.
func formatTimerClock(timer runs.ActiveTimer) string {
	if timer.RemainingS != nil {
		return "剩余 " + formatDuration(*timer.RemainingS)
	}
	return "已计时 " + formatDuration(timer.ElapsedS)
}

// formatDuration renders seconds as mm:ss, or h:mm:ss past the hour.
func formatDuration(total uint64) string {
	h, m, sec := total/3600, (total%3600)/60, total%60
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, sec)
	}
	return fmt.Sprintf("%d:%02d", m, sec)
}

func (s *Shell) projectView(c *ui.Context) {
	t := c.Theme()
	width, _ := c.Size()
	columns := 3
	if width < 980 {
		columns = 2
	}
	if width < 680 {
		columns = 1
	}
	query := strings.ToLower(strings.TrimSpace(s.Query))
	designs := make([]DesignItem, 0, len(s.Designs))
	for _, design := range s.Designs {
		if query == "" || strings.Contains(strings.ToLower(design.Title), query) {
			designs = append(designs, design)
		}
	}
	ui.Scroll(c).Grow(1).Children(func() {
		ui.Column(c).Padding(t.Space(5), t.Space(4)).Gap(t.Space(5)).MaxWidth(1040).
			MaxWidthPercent(100).Children(func() {
			ui.Row(c).FillWidth().AlignItems(ui.End).Gap(t.Space(3)).Children(func() {
				ui.Column(c).Gap(t.Space(.75)).Children(func() {
					components.Text(c, "项目工作区").FontSize(t.Rem(1.7)).FontWeight(600)
					components.Text(c, "继续一个设计，或从已有案例开始；所有文档共用同一份 AsterDoc。").
						TextColor(t.TextMuted).FontSize(t.Rem(.92))
				})
				ui.Spacer(c)
				ui.TextInput(c, &s.Query).Placeholder("搜索设计…").Width(t.Space(32))
			})

			ui.Row(c).Wrap().Gap(t.Space(2)).AlignItems(ui.Center).Children(func() {
				if s.OnCreate != nil && len(s.Designs) > 0 && components.Button(c, "新建设计", components.ButtonPrimary).Clicked() {
					s.OnCreate()
				}
				if s.OnImport != nil && components.Button(c, "导入设计", components.ButtonSecondary).Clicked() {
					s.OnImport()
				}
				if s.OnContinue != nil && len(s.RecentRuns) > 0 && components.Button(c, "继续上次工作", components.ButtonGhost).Clicked() {
					s.OnContinue()
				}
				ui.Spacer(c)
				count := fmt.Sprintf("%d 个设计 · %d 个运行", len(s.Designs), len(s.RecentRuns))
				if len(s.Projects) > 0 {
					count = fmt.Sprintf("%d 个项目 · %s", len(s.Projects), count)
				}
				components.Text(c, count).TextColor(t.TextMuted).FontSize(t.Rem(.78))
			})

			if len(s.Designs) == 0 {
				s.homeEmpty(c)
			} else {
				inProgress := []RecentRun{}
				for _, run := range s.RecentRuns {
					if run.Meta != "completed" {
						inProgress = append(inProgress, run)
					}
				}
				if len(inProgress) > 2 {
					inProgress = inProgress[:2]
				}
				// No hard item cap: hiding designs made older work unreachable
				// from the workspace (UX-IA-1). The grid scrolls instead.
				ui.Column(c).Gap(t.Space(1.5)).Children(func() {
					ui.Row(c).AlignItems(ui.Center).Children(func() {
						components.Text(c, "继续工作").FontSize(t.Rem(1.05)).FontWeight(600)
						ui.Spacer(c)
						components.Text(c, fmt.Sprintf("%d 个工作项", len(designs)+len(inProgress))).
							TextColor(t.TextMuted).FontSize(t.Rem(.78))
					})
					if len(designs) == 0 && len(inProgress) == 0 {
						components.Text(c, "没有匹配的设计。").
							TextColor(t.TextMuted).FontSize(t.Rem(.9))
						return
					}
					ui.Grid(c).Columns(columns).Gap(t.Space(1)).Children(func() {
						for _, run := range inProgress {
							s.runCard(c, run)
						}
						for _, design := range designs {
							s.designCard(c, design)
						}
					})
				})
				s.homeTemplates(c)
				s.homeRuns(c)
			}

			if len(s.Projects) > 0 {
				ui.Column(c).Gap(t.Space(1)).Children(func() {
					components.Text(c, "项目").FontSize(t.Rem(1.05)).FontWeight(600)
					ui.Row(c).Wrap().Gap(t.Space(1)).Children(func() {
						for _, project := range s.Projects {
							project := project
							b := components.Button(c, project, components.ButtonGhost)
							if project == s.Project {
								b = components.Button(c, project, components.ButtonSecondary)
							}
							if b.Clicked() && s.OnProject != nil {
								s.OnProject(project)
							}
						}
					})
				})
			}
			if s.Loading {
				components.Text(c, "正在加载工作区…").TextColor(t.TextMuted)
			}
		})
	})
}

func (s *Shell) homeEmpty(c *ui.Context) {
	t := c.Theme()
	title, detail := "未开始 · 还没有实验设计", "新建一个设计，或导入已有记录开始。"
	if s.Project == "" && len(s.Projects) == 0 {
		title, detail = "未开始 · 尚无项目", "新建设计开始工作；现有记录也可以导入。"
	}
	// UX-P5 / UX-STATE / UX-DONE: distinguish no-project from an empty project and offer a direct first action.
	ui.Column(c).FillWidth().Gap(t.Space(1.5)).Padding(t.Space(5)).Radius(t.Space(2)).
		Background(t.Surface).Border(1, t.Border).AlignItems(ui.Center).Children(func() {
		components.Icon(c, lucide.FlaskConical()).Size(t.Space(6), t.Space(6)).TextColor(t.Accent)
		components.Text(c, title).FontSize(t.Rem(1.15)).FontWeight(600)
		components.Text(c, detail).TextColor(t.TextMuted).FontSize(t.Rem(.9))
		if s.OnCreate != nil && components.Button(c, "新建设计", components.ButtonPrimary).Clicked() {
			s.OnCreate()
		}
	})
}

func (s *Shell) designCard(c *ui.Context, design DesignItem) {
	t := c.Theme()
	card := ui.ButtonBase(c).FillWidth().Padding(t.Space(2), t.Space(2)).Radius(t.Space(1.5)).
		Background(t.Surface).Border(1, t.Border).Label(design.Title)
	card.Children(func() {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(t.Space(1.5)).Children(func() {
			ui.Box(c).Size(t.Space(4.5), t.Space(4.5)).Radius(t.Space(1)).
				Background(t.Accent.Alpha(.10)).Center().Children(func() {
				components.Icon(c, lucide.FileText()).Size(t.Space(2.2), t.Space(2.2)).
					TextColor(t.Accent)
			})
			ui.Column(c).Grow(1).MinWidth(0).Gap(1).Children(func() {
				components.Text(c, design.Title).FontSize(t.Rem(.9)).FontWeight(600).
					SingleLine().Ellipsis("…")
				components.Text(c, design.Meta).TextColor(t.TextMuted).FontSize(t.Rem(.75))
			})
			components.Icon(c, lucide.ArrowUpRight()).Size(t.Space(2), t.Space(2)).
				TextColor(t.TextMuted)
		})
	})
	if card.Clicked() {
		s.Selected = design.ID
		s.Page = Design
		if s.OnSelect != nil {
			s.OnSelect(design.ID)
		}
		s.setPage(Design)
	}
}

func statusLabel(status string) string {
	switch status {
	case "in_progress":
		return "执行中"
	case "planned":
		return "计划中"
	case "completed":
		return "已完成"
	case "blocked":
		return "阻塞"
	case "cancelled":
		return "已取消"
	}
	return status
}

func (s *Shell) runCard(c *ui.Context, run RecentRun) {
	t := c.Theme()
	card := ui.ButtonBase(c).FillWidth().Padding(t.Space(2), t.Space(2)).Radius(t.Space(1.5)).
		Background(t.Surface).Border(1, t.Border).Label(run.Title)
	card.Children(func() {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(t.Space(1.5)).Children(func() {
			ui.Box(c).Size(t.Space(4.5), t.Space(4.5)).Radius(t.Space(1)).
				Background(t.Warning.Alpha(.12)).Center().Children(func() {
				components.Icon(c, lucide.Play()).Size(t.Space(2.2), t.Space(2.2)).
					TextColor(t.Warning)
			})
			ui.Column(c).Grow(1).MinWidth(0).Gap(1).Children(func() {
				components.Text(c, run.Title).FontSize(t.Rem(.9)).FontWeight(600).
					SingleLine().Ellipsis("…")
				components.Text(c, "Run · "+statusLabel(run.Meta)).TextColor(t.TextMuted).FontSize(t.Rem(.75))
			})
			components.Icon(c, lucide.ArrowUpRight()).Size(t.Space(2), t.Space(2)).
				TextColor(t.TextMuted)
		})
	})
	if card.Clicked() && s.OnRunSelect != nil {
		s.OnRunSelect(run.ID)
	}
}

func (s *Shell) homeTemplates(c *ui.Context) {
	t := c.Theme()
	type match struct {
		title  string
		design DesignItem
	}
	matches := []match{}
	for _, template := range []struct{ title, match string }{
		{"CCK-8 细胞活力", "CCK-8"},
		{"L929 生物相容性", "L929"},
		{"RT-qPCR 表达比较", "RT-qPCR"},
	} {
		for _, design := range s.Designs {
			if strings.Contains(design.Title, template.match) {
				matches = append(matches, match{title: template.title, design: design})
				break
			}
		}
	}
	if len(matches) == 0 {
		return
	}
	ui.Column(c).Gap(t.Space(1.5)).Children(func() {
		components.Text(c, "浏览实验设计").FontSize(t.Rem(1.05)).FontWeight(600)
		ui.Row(c).Wrap().Gap(t.Space(1)).Children(func() {
			for _, item := range matches {
				item := item
				b := ui.ButtonBase(c).Padding(t.Space(1.5), t.Space(2)).Radius(t.Space(1.5)).
					Background(t.Background).Border(1, t.Border).Label(item.design.Title)
				b.Children(func() {
					ui.Row(c).Gap(t.Space(1)).AlignItems(ui.Center).Children(func() {
						components.Icon(c, lucide.FlaskConical()).Size(t.Space(2.2), t.Space(2.2)).
							TextColor(t.Accent)
						ui.Column(c).Gap(0).Children(func() {
							components.Text(c, item.title).FontSize(t.Rem(.85))
							components.Text(c, item.design.Meta).TextColor(t.TextMuted).
								FontSize(t.Rem(.7))
						})
					})
				})
				if b.Clicked() {
					s.Selected = item.design.ID
					if s.OnSelect != nil {
						s.OnSelect(item.design.ID)
					}
					s.setPage(Design)
				}
			}
		})
	})
}

func (s *Shell) homeRuns(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Gap(t.Space(1.5)).Children(func() {
		ui.Row(c).AlignItems(ui.Center).Children(func() {
			components.Text(c, "最近的运行").FontSize(t.Rem(1.05)).FontWeight(600)
			ui.Spacer(c)
			components.Text(c, fmt.Sprintf("%d 个", len(s.RecentRuns))).
				TextColor(t.TextMuted).FontSize(t.Rem(.78))
		})
		if len(s.RecentRuns) == 0 {
			components.Text(c, "尚无运行。设计保存后可以创建冻结运行。").
				TextColor(t.TextMuted).FontSize(t.Rem(.88))
			return
		}
		shown := s.RecentRuns
		if len(shown) > 5 {
			shown = shown[:5]
		}
		for _, run := range shown {
			run := run
			b := ui.ButtonBase(c).FillWidth().Padding(t.Space(1.5), t.Space(2)).Radius(t.Space(1.5)).
				Background(t.Surface).Border(1, t.Border).Label(run.Title)
			b.Children(func() {
				ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(t.Space(1.5)).Children(func() {
					components.Icon(c, lucide.Play()).Size(t.Space(2.2), t.Space(2.2)).
						TextColor(t.TextMuted)
					components.Text(c, run.Title).FontSize(t.Rem(.88)).SingleLine().Ellipsis("…")
					ui.Spacer(c)
					components.Text(c, statusLabel(run.Meta)).TextColor(t.TextMuted).FontSize(t.Rem(.75))
					components.Icon(c, lucide.ArrowUpRight()).Size(t.Space(1.8), t.Space(1.8)).
						TextColor(t.TextMuted)
				})
			})
			if b.Clicked() && s.OnRunSelect != nil {
				s.OnRunSelect(run.ID)
			}
		}
	})
}

// Quit closes the application when the AsterLab window closes.
func (s *Shell) Quit() {
	if s.window != nil {
		s.window.Window().OnClosed(func() { mygo.App.Quit() })
	}
}
