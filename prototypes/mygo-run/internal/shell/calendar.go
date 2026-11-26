package shell

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/mobile"
	"github.com/oneo-me/onevra/source/components"
	"github.com/oneo-me/onevra/source/icons/lucide"

	"asterlab-mygo-prototype/internal/schedule"
	"asterlab-mygo-prototype/internal/theme"
	"asterlab-mygo-prototype/internal/uikit/kit"
)

// StageLabel returns the human-readable Chinese lifecycle label for an ExperimentStage.
func StageLabel(stage ExperimentStage) string {
	switch stage {
	case StageDraft:
		return "设计中"
	case StageReady:
		return "方案就绪"
	case StageInProgress:
		return "进行中"
	case StageCompleted:
		return "已完成"
	default:
		return "设计中"
	}
}

func stageTone(stage ExperimentStage) components.BadgeTone {
	switch stage {
	case StageDraft:
		return components.BadgeWarning
	case StageReady:
		return components.BadgeSuccess
	case StageInProgress:
		return components.BadgeAccent
	case StageCompleted:
		return components.BadgeNeutral
	default:
		return components.BadgeWarning
	}
}

func stageIcon(stage ExperimentStage) *ui.SVG {
	switch stage {
	case StageDraft:
		return lucide.PenLine()
	case StageReady:
		return lucide.FlaskConical()
	case StageInProgress:
		return lucide.Play()
	case StageCompleted:
		return lucide.Check()
	default:
		return lucide.FlaskConical()
	}
}

func stagePillColors(tok theme.Tokens, stage ExperimentStage) (bg, accent, fg ui.Color) {
	switch stage {
	case StageReady:
		return tok.Success.Alpha(.12), tok.Success, tok.SuccessText
	case StageInProgress:
		return tok.AccentSoftBg, tok.Accent, tok.Accent
	case StageCompleted:
		return tok.Elevated, tok.BorderStrong, tok.TextMuted
	default: // StageDraft
		return tok.Warning.Alpha(.14), tok.Warning, tok.WarningText
	}
}

func (s *Shell) resolvedToday() time.Time {
	if raw := strings.TrimSpace(s.TodayDate); len(raw) >= 10 {
		if t, err := time.Parse("2006-01-02", raw[:10]); err == nil {
			return t
		}
	}
	now := time.Now()
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)
}

func (s *Shell) ensureSelectedDate() string {
	if raw := strings.TrimSpace(s.SelectedDate); len(raw) >= 10 {
		if _, err := time.Parse("2006-01-02", raw[:10]); err == nil {
			return raw[:10]
		}
	}
	today := s.resolvedToday().Format("2006-01-02")
	s.SelectedDate = today
	return today
}

func (s *Shell) ensureCalendarCursor() time.Time {
	if raw := strings.TrimSpace(s.CalendarCursor); len(raw) >= 10 {
		if t, err := time.Parse("2006-01-02", raw[:10]); err == nil {
			return t
		}
	}
	sel, err := time.Parse("2006-01-02", s.ensureSelectedDate())
	if err != nil {
		sel = s.resolvedToday()
	}
	s.CalendarCursor = sel.Format("2006-01-02")
	return sel
}

func (s *Shell) resolvedExperiments() []ExperimentItem {
	if len(s.Experiments) > 0 {
		return s.Experiments
	}
	today := s.resolvedToday().Format("2006-01-02")
	out := make([]ExperimentItem, 0, len(s.Designs)+len(s.RecentRuns))
	for _, r := range s.RecentRuns {
		st := StageInProgress
		if r.Meta == "completed" {
			st = StageCompleted
		}
		out = append(out, ExperimentItem{
			ID:          "run:" + r.ID,
			ActiveRunID: r.ID,
			Title:       r.Title,
			Date:        today,
			Stage:       st,
			Summary:     statusLabel(r.Meta),
		})
	}
	for _, d := range s.Designs {
		st := StageReady
		gap := ""
		if strings.Contains(d.Meta, "设计中") {
			st = StageDraft
			gap = d.Meta
		}
		out = append(out, ExperimentItem{
			ID:       "design:" + d.ID,
			DesignID: d.ID,
			Title:    d.Title,
			Date:     today,
			Stage:    st,
			Summary:  d.Meta,
			GapHint:  gap,
		})
	}
	return out
}

func (s *Shell) triggerCreateForDate(dateYMD string) {
	s.DayPopoverOpen = false
	if s.OnCreateForDate != nil {
		s.OnCreateForDate(dateYMD)
		return
	}
	if s.OnCreate != nil {
		s.OnCreate()
	}
}

func (s *Shell) triggerCreateForDateTime(dateYMD, startTimeHHMM string) {
	s.DayPopoverOpen = false
	if s.OnCreateForDateTime != nil {
		s.OnCreateForDateTime(dateYMD, startTimeHHMM)
		return
	}
	s.triggerCreateForDate(dateYMD)
}

func (s *Shell) openExperiment(exp ExperimentItem) {
	s.DayPopoverOpen = false
	if exp.DesignID != "" {
		s.Selected = exp.DesignID
	} else {
		s.Selected = exp.ID
	}
	if s.OnOpenExperiment != nil {
		s.OnOpenExperiment(exp)
		return
	}
	s.OpenExperimentTab(exp, defaultStageForExperiment(exp))
}

func (s *Shell) openExperimentInSplit(exp ExperimentItem) {
	s.DayPopoverOpen = false
	path := strings.TrimSpace(exp.FilePath)
	if path == "" {
		path = exp.DesignID
	}
	s.SplitPaneWithTab(s.ActivePaneID, WorkspaceTab{
		Kind:      TabFile,
		Title:     firstNonEmptyStr(exp.Title, "未命名实验"),
		FilePath:  path,
		DesignID:  exp.DesignID,
		RunID:     exp.ActiveRunID,
		Mode:      ModeEdit,
		StageLock: exp.Stage,
	}, "right")
}

func weekdayMondayIndex(w time.Weekday) int {
	if w == time.Sunday {
		return 6
	}
	return int(w) - 1
}

func weekdayCNShort(w time.Weekday) string {
	switch w {
	case time.Monday:
		return "一"
	case time.Tuesday:
		return "二"
	case time.Wednesday:
		return "三"
	case time.Thursday:
		return "四"
	case time.Friday:
		return "五"
	case time.Saturday:
		return "六"
	default:
		return "日"
	}
}

func isWeekend(w time.Weekday) bool {
	return w == time.Saturday || w == time.Sunday
}

func (s *Shell) calendarHomeView(c *ui.Context) {
	t := c.Theme()
	tok := theme.GetTokens(t)
	today := s.resolvedToday()
	todayYMD := today.Format("2006-01-02")
	selDateYMD := s.ensureSelectedDate()
	cursor := s.ensureCalendarCursor()

	if !s.calFilterInited {
		s.calFilterInited = true
		s.CalendarDomainFilter = schedule.DefaultFilter()
	}
	domFlt := s.CalendarDomainFilter
	domFlt.Query = s.Query
	evByDate := schedule.IndexByDate(s.CalendarEvents, domFlt)

	allExps := s.resolvedExperiments()
	query := strings.ToLower(strings.TrimSpace(s.Query))
	byDate := map[string][]ExperimentItem{}
	if domFlt.ShowExperiment {
		for _, exp := range allExps {
			if query != "" && !strings.Contains(strings.ToLower(exp.Title), query) && !strings.Contains(strings.ToLower(exp.Summary), query) {
				continue
			}
			if strings.TrimSpace(exp.Date) == "" {
				exp.Date = todayYMD
			}
			byDate[exp.Date] = append(byDate[exp.Date], exp)
		}
	}

	_, winH := c.Size()
	compactV := winH > 0 && winH < 720
	mobileMode := s.IsMobileLayout(c)
	if s.CalendarMode < 0 || s.CalendarMode > 1 {
		s.CalendarMode = 0
	}

	if mobileMode {
		ui.Scroll(c.Key("cal-mobile-scroll")).Fill().
			TrackScroll(s.MobileContentScrollState()).
			ScrollbarInsets(s.MobileTopReserve(c), 0, s.MobileBottomReserve(c), 0).Children(func() {
			s.calendarHomeBody(c, tok, cursor, today, todayYMD, selDateYMD, compactV, mobileMode, allExps, byDate, evByDate)
		})
		return
	}
	s.calendarHomeBody(c, tok, cursor, today, todayYMD, selDateYMD, compactV, mobileMode, allExps, byDate, evByDate)
}

func (s *Shell) calendarHomeBody(
	c *ui.Context,
	tok theme.Tokens,
	cursor time.Time,
	today time.Time,
	todayYMD string,
	selDateYMD string,
	compactV bool,
	mobileMode bool,
	allExps []ExperimentItem,
	byDate map[string][]ExperimentItem,
	evByDate map[string][]schedule.Event,
) {
	// UI-15: Generous desktop breathing margins so the calendar grid sits comfortably in the canvas.
	padTop := float32(tok.Spacing24)
	padH := float32(tok.Spacing32)
	padB := float32(tok.Spacing24)
	if compactV {
		padTop = float32(tok.Spacing12)
		padH = float32(tok.Spacing24)
		padB = float32(tok.Spacing12)
	}
	if mobileMode {
		padTop = s.MobileTopReserve(c) + float32(tok.Spacing4)
		padH = float32(tok.Spacing8)
		padB = s.MobileBottomReserve(c) + float32(tok.Spacing8)
	}
	col := ui.Column(c).FillWidth().Padding(padTop, padH, padB, padH).Gap(float32(tok.Spacing16))
	if !mobileMode {
		col.FillHeight()
	}
	col.Children(func() {
		if len(allExps) == 0 {
			ui.Row(c).FillWidth().Padding(float32(tok.Spacing8), float32(tok.Spacing12)).
				Radius(float32(tok.ControlRadius)).Background(tok.Elevated).Border(1, tok.Border).
				AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
				ui.Icon(c, lucide.CalendarDays()).Size(float32(tok.Spacing16), float32(tok.Spacing16)).TextColor(tok.Accent)
				hint := "双击任意日期格新建实验，或从左侧文件树创建项目文件夹分类整理 .asterml 文件。"
				if mobileMode {
					hint = "点按「+ 新建」或「+ 安排」记录实验，从左边缘向右滑展开实验库。"
				}
				components.Text(c, hint).
					FontSize(float32(tok.TextData)).TextColor(tok.TextMuted).Grow(1).MinWidth(0)
			})
		}
		if mobileMode {
			s.renderMobileCalendarToolbar(c, tok, &cursor, today, todayYMD, &selDateYMD)
		} else {
			// 1. Balanced 3-Column Calendar Toolbar:
			// UI-16: Left & Right tracks share equal Grow(1) so the center "月 | 周" segmented control
			// stays mathematically fixed in position regardless of date title width.
			// UI-17: Week date range renders as a smaller muted subtitle next to the bold year-month title.
			ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(float32(tok.Spacing12)).Children(func() {
				// Left track (Grow 1): Period heading + prev/today/next controls
				ui.Row(c).Grow(1).MinWidth(0).Justify(ui.Start).AlignItems(ui.Center).Gap(float32(tok.Spacing12)).Children(func() {
					ui.Row(c).AlignItems(ui.Baseline).Gap(float32(tok.Spacing8)).Children(func() {
						components.Text(c, cursor.Format("2006 年 01 月")).FontSize(float32(tok.TextPage)).Bold().
							TextColor(tok.Text).FontFeatures("tnum").SingleLine()
						if s.CalendarMode == 1 {
							wStart := cursor.AddDate(0, 0, -weekdayMondayIndex(cursor.Weekday()))
							wEnd := wStart.AddDate(0, 0, 6)
							rangeLbl := fmt.Sprintf("%s – %s", wStart.Format("01/02"), wEnd.Format("01/02"))
							components.Text(c, rangeLbl).FontSize(float32(tok.TextData)).
								TextColor(tok.TextMuted).FontFeatures("tnum").SingleLine()
						}
					})

					ui.Row(c).AlignItems(ui.Center).Gap(float32(tok.Spacing4)).Shrink(0).Children(func() {
						if components.IconButton(c, lucide.ChevronLeft(), "上一时段", components.IconButtonCompact).Clicked() {
							s.calAnimDir = -1
							s.DayPopoverOpen = false
							if s.CalendarMode == 0 {
								cursor = cursor.AddDate(0, -1, 0)
							} else {
								cursor = cursor.AddDate(0, 0, -7)
							}
							s.CalendarCursor = cursor.Format("2006-01-02")
						}
						if components.Button(c, "今天", components.ButtonOutline).Clicked() {
							s.calAnimDir = 0
							s.SelectedDate = todayYMD
							s.CalendarCursor = todayYMD
							s.DayPopoverDate = todayYMD
							s.DayPopoverOpen = true
							selDateYMD = todayYMD
							cursor = today
						}
						if components.IconButton(c, lucide.ChevronRight(), "下一时段", components.IconButtonCompact).Clicked() {
							s.calAnimDir = 1
							s.DayPopoverOpen = false
							if s.CalendarMode == 0 {
								cursor = cursor.AddDate(0, 1, 0)
							} else {
								cursor = cursor.AddDate(0, 0, 7)
							}
							s.CalendarCursor = cursor.Format("2006-01-02")
						}
					})
				})

				// Center pinned track (Shrink 0): Month / Week segmented switcher
				ui.Row(c).Shrink(0).AlignItems(ui.Center).Children(func() {
					modeTab := s.CalendarMode
					components.Tabs(c, &modeTab, components.TabsSegmented, "月", "周")
					if modeTab != s.CalendarMode {
						s.DayPopoverOpen = false
						s.CalendarMode = modeTab
					}
				})

				// Right track (Grow 1): Search + Primary CTA + More menu
				ui.Row(c).Grow(1).MinWidth(0).Justify(ui.End).AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
					ui.TextInput(c, &s.Query).Placeholder("搜索实验…").Width(float32(tok.Spacing48) * 3)
					if s.OnCreateForDate != nil || s.OnCreate != nil {
						if components.Button(c, "＋ 加入新实验", components.ButtonPrimary).Clicked() {
							s.triggerCreateForDate(selDateYMD)
						}
					}
					if s.OnOpenTemplatePicker != nil || s.OnImport != nil || s.OnCreateReagentForDate != nil || s.OnCreateAnimalForDate != nil {
						moreBtn := components.IconButton(c, lucide.Ellipsis(), "更多创建方式", components.IconButtonCompact)
						components.MenuButton(c, moreBtn, func(m *components.Menu) {
							if s.OnOpenTemplatePicker != nil && m.Item("从模板创建实验…").Chosen() {
								s.OnOpenTemplatePicker(selDateYMD)
							}
							if s.OnCreateReagentForDate != nil && m.Item("登记试剂购入…").Chosen() {
								s.OnCreateReagentForDate(selDateYMD)
							}
							if s.OnCreateAnimalForDate != nil && m.Item("登记动物入栏 / 收样…").Chosen() {
								s.OnCreateAnimalForDate(selDateYMD)
							}
							if s.OnImport != nil && m.Item("导入 .asterml 文件…").Chosen() {
								s.OnImport()
							}
						})
					}
				})
			})
		}

		// 3-Domain Schedule Layer Bar: Experiment × Reagent × Vivarium
		reagentEvCnt, animalEvCnt := 0, 0
		for _, ev := range s.CalendarEvents {
			switch ev.Domain {
			case schedule.DomainReagent:
				reagentEvCnt++
			case schedule.DomainVivarium:
				animalEvCnt++
			}
		}
		if len(s.CalendarEvents) > 0 || len(allExps) > 0 {
			ui.Row(c).FillWidth().Wrap().AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
				ui.Text(c, "日历图层：").FontSize(float32(tok.TextMeta)).Bold().TextColor(tok.TextMuted).SingleLine()
				allOn := s.CalendarDomainFilter.ShowExperiment && s.CalendarDomainFilter.ShowReagent && s.CalendarDomainFilter.ShowVivarium
				allVar := components.ButtonGhost
				if allOn {
					allVar = components.ButtonToolbar
				}
				if components.Button(c, "全部三域", allVar).Clicked() {
					s.CalendarDomainFilter = schedule.DefaultFilter()
				}
				expVar := components.ButtonGhost
				if s.CalendarDomainFilter.ShowExperiment {
					expVar = components.ButtonToolbar
				}
				if components.Button(c, fmt.Sprintf("实验 (%d)", len(allExps)), expVar).FontFeatures("tnum").Clicked() {
					s.CalendarDomainFilter.ShowExperiment = !s.CalendarDomainFilter.ShowExperiment
				}
				rgVar := components.ButtonGhost
				if s.CalendarDomainFilter.ShowReagent {
					rgVar = components.ButtonToolbar
				}
				if components.Button(c, fmt.Sprintf("试剂效期与采购 (%d)", reagentEvCnt), rgVar).FontFeatures("tnum").Clicked() {
					s.CalendarDomainFilter.ShowReagent = !s.CalendarDomainFilter.ShowReagent
				}
				anVar := components.ButtonGhost
				if s.CalendarDomainFilter.ShowVivarium {
					anVar = components.ButtonToolbar
				}
				if components.Button(c, fmt.Sprintf("动物入栏与收样 (%d)", animalEvCnt), anVar).FontFeatures("tnum").Clicked() {
					s.CalendarDomainFilter.ShowVivarium = !s.CalendarDomainFilter.ShowVivarium
				}
			})
		}

		// 2. Apple Seamless Surface Grid (or Mobile 3-Day / Day Agenda with swipeable date strip)
		if s.CalendarMode == 0 {
			s.renderAppleMonthGrid(c, tok, cursor, todayYMD, selDateYMD, compactV, byDate, evByDate)
			if mobileMode {
				s.renderMobileDayAgenda(c, tok, selDateYMD, byDate[selDateYMD], evByDate[selDateYMD])
			}
		} else if mobileMode {
			s.renderMobileAgendaView(c, tok, cursor, today, todayYMD, selDateYMD, byDate, evByDate)
		} else {
			s.renderAppleWeekTimeGrid(c, tok, cursor, todayYMD, selDateYMD, byDate, evByDate)
		}
	})
}

func (s *Shell) renderMobileCalendarToolbar(c *ui.Context, tok theme.Tokens, cursor *time.Time, today time.Time, todayYMD string, selDateYMD *string) {
	ui.Column(c).FillWidth().Gap(float32(tok.Spacing8)).Children(func() {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
			components.Text(c, cursor.Format("2006 年 01 月")).FontSize(float32(tok.TextSection)).Bold().
				TextColor(tok.Text).FontFeatures("tnum").SingleLine().Grow(1).MinWidth(0)

			ui.Row(c).AlignItems(ui.Center).Gap(float32(tok.Spacing4)).Shrink(0).Children(func() {
				if components.IconButton(c, lucide.ChevronLeft(), "上一时段", components.IconButtonCompact).Clicked() {
					s.calAnimDir = -1
					s.DayPopoverOpen = false
					if s.CalendarMode == 0 {
						*cursor = cursor.AddDate(0, -1, 0)
					} else {
						*cursor = cursor.AddDate(0, 0, -7)
					}
					s.CalendarCursor = cursor.Format("2006-01-02")
				}
				if components.Button(c, "今天", components.ButtonOutline).Clicked() {
					s.calAnimDir = 0
					s.SelectedDate = todayYMD
					s.CalendarCursor = todayYMD
					s.DayPopoverOpen = false
					*selDateYMD = todayYMD
					*cursor = today
				}
				if components.IconButton(c, lucide.ChevronRight(), "下一时段", components.IconButtonCompact).Clicked() {
					s.calAnimDir = 1
					s.DayPopoverOpen = false
					if s.CalendarMode == 0 {
						*cursor = cursor.AddDate(0, 1, 0)
					} else {
						*cursor = cursor.AddDate(0, 0, 7)
					}
					s.CalendarCursor = cursor.Format("2006-01-02")
				}
			})
		})

		ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
			modeTab := s.CalendarMode
			components.Tabs(c, &modeTab, components.TabsSegmented, "月", "日程")
			if modeTab != s.CalendarMode {
				s.DayPopoverOpen = false
				s.CalendarMode = modeTab
			}
			ui.Spacer(c)
			if s.OnCreateForDate != nil || s.OnCreate != nil {
				if components.Button(c, "＋ 新建", components.ButtonPrimary).Clicked() {
					s.triggerCreateForDate(*selDateYMD)
				}
			}
			if s.OnOpenTemplatePicker != nil || s.OnImport != nil || s.OnCreateReagentForDate != nil || s.OnCreateAnimalForDate != nil {
				moreBtn := components.IconButton(c, lucide.Ellipsis(), "更多创建方式", components.IconButtonCompact)
				components.MenuButton(c, moreBtn, func(m *components.Menu) {
					if s.OnOpenTemplatePicker != nil && m.Item("从模板创建实验…").Chosen() {
						s.OnOpenTemplatePicker(*selDateYMD)
					}
					if s.OnCreateReagentForDate != nil && m.Item("登记试剂购入…").Chosen() {
						s.OnCreateReagentForDate(*selDateYMD)
					}
					if s.OnCreateAnimalForDate != nil && m.Item("登记动物入栏 / 收样…").Chosen() {
						s.OnCreateAnimalForDate(*selDateYMD)
					}
					if s.OnImport != nil && m.Item("导入 .asterml 文件…").Chosen() {
						s.OnImport()
					}
				})
			}
		})
	})
}

func (s *Shell) renderMobileAgendaView(
	c *ui.Context,
	tok theme.Tokens,
	cursor time.Time,
	today time.Time,
	todayYMD string,
	selDateYMD string,
	byDate map[string][]ExperimentItem,
	evByDate map[string][]schedule.Event,
) {
	extra := s.mobileExtra()
	if !extra.calAgendaInited {
		extra.calAgendaInited = true
		extra.calAgenda3Day = true
	}
	if !extra.calStripInited {
		extra.calStripInited = true
		extra.calStripPage = mobile.PageState{Page: 1}
	}

	selDate, err := time.Parse("2006-01-02", selDateYMD)
	if err != nil {
		selDate = cursor
		selDateYMD = selDate.Format("2006-01-02")
	}

	// 1. Swipeable Horizontal 7-Day Date Strip (Page 0 = prev week, Page 1 = current week, Page 2 = next week)
	stripH := float32(tok.Spacing48) + float32(tok.Spacing16)
	ui.Box(c.Key("mb-cal-strip-wrap")).FillWidth().Height(stripH).
		Radius(float32(tok.PanelRadius)).Background(tok.Surface).Border(1, tok.Border).Clip().Children(func() {
		mobile.PageView(c, &extra.calStripPage, 3, func(pageIdx int) {
			weekShift := (pageIdx - 1) * 7
			anchor := cursor.AddDate(0, 0, weekShift)
			wStart := anchor.AddDate(0, 0, -weekdayMondayIndex(anchor.Weekday()))
			ui.Row(c).Fill().AlignItems(ui.Center).Padding(float32(tok.Spacing4)).Gap(float32(tok.Spacing4)*0.5).Children(func() {
				for i := 0; i < 7; i++ {
					day := wStart.AddDate(0, 0, i)
					ymd := day.Format("2006-01-02")
					isToday := ymd == todayYMD
					isSel := ymd == selDateYMD
					hasItems := len(byDate[ymd])+len(evByDate[ymd]) > 0

					bg := ui.Transparent
					borderW := float32(0)
					borderCol := ui.Transparent
					numCol := tok.Text
					wdCol := tok.TextMuted
					if isToday && isSel {
						bg = tok.Accent
						numCol = tok.AccentText
						wdCol = tok.AccentText.Alpha(0.85)
					} else if isSel {
						bg = tok.AccentSoftBg
						borderW = 1
						borderCol = tok.Accent
						numCol = tok.Accent
						wdCol = tok.Accent
					} else if isToday {
						borderW = 1
						borderCol = tok.Accent
						numCol = tok.Accent
					}

					cell := ui.ButtonBase(c.Key("mb-strip-day-"+ymd)).Grow(1).MinWidth(0).FillHeight().
						Radius(float32(tok.ControlRadius)).Background(bg).Border(borderW, borderCol).
						Center().Label("日期 " + ymd)
					theme.ApplyTactilePress(cell)
					cell.Children(func() {
						ui.Column(c).Center().Gap(float32(tok.Spacing4)*0.5).Children(func() {
							components.Text(c, "周"+weekdayCNShort(day.Weekday())).
								FontSize(float32(tok.TextMeta)*0.85).TextColor(wdCol).SingleLine()
							components.Text(c, fmt.Sprintf("%d", day.Day())).
								FontSize(float32(tok.TextBody)).Bold().TextColor(numCol).
								FontFeatures("tnum").SingleLine()
							dotCol := ui.Transparent
							if hasItems {
								dotCol = tok.Accent
								if isToday && isSel {
									dotCol = tok.AccentText
								}
							}
							ui.Box(c).Size(float32(tok.Spacing4), float32(tok.Spacing4)).
								Radius(float32(tok.ControlRadius)).Background(dotCol)
						})
					})
					if cell.Clicked() {
						mygo.Haptics.Selection()
						s.SelectedDate = ymd
						s.CalendarCursor = ymd
					}
				}
			})
		}).Fill()
	})

	if extra.calStripPage.Page != 1 {
		delta := (extra.calStripPage.Page - 1) * 7
		nextCursor := cursor.AddDate(0, 0, delta)
		nextSel := selDate.AddDate(0, 0, delta)
		s.CalendarCursor = nextCursor.Format("2006-01-02")
		s.SelectedDate = nextSel.Format("2006-01-02")
		extra.calStripPage.Page = 1
		mygo.Haptics.Selection()
		selDate = nextSel
		selDateYMD = s.SelectedDate
	}

	// 2. 3-Day / Single-Day Mode Toggle & Range Header
	spanDays := 1
	rangeText := fmt.Sprintf("%d月%d日 · 周%s", selDate.Month(), selDate.Day(), weekdayCNShort(selDate.Weekday()))
	if extra.calAgenda3Day {
		spanDays = 3
		endD := selDate.AddDate(0, 0, 2)
		rangeText = fmt.Sprintf("%d月%d日 – %d月%d日 · 3日日程", selDate.Month(), selDate.Day(), endD.Month(), endD.Day())
	}
	agendaTab := 0
	if !extra.calAgenda3Day {
		agendaTab = 1
	}
	ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
		components.Text(c, rangeText).FontSize(float32(tok.TextData)).Bold().
			TextColor(tok.Text).FontFeatures("tnum").SingleLine().Ellipsis("…").Grow(1).MinWidth(0)
		components.Tabs(c, &agendaTab, components.TabsSegmented, "3日", "单日")
		if (agendaTab == 0) != extra.calAgenda3Day {
			extra.calAgenda3Day = (agendaTab == 0)
			mygo.Haptics.Selection()
		}
	})

	// 3. 3-Day / Day Agenda Flow Cards
	ui.Column(c.Key("mb-agenda-cards")).FillWidth().Gap(float32(tok.Spacing12)).Children(func() {
		for i := 0; i < spanDays; i++ {
			day := selDate.AddDate(0, 0, i)
			ymd := day.Format("2006-01-02")
			isToday := ymd == todayYMD
			dayExps := append([]ExperimentItem(nil), byDate[ymd]...)
			sort.SliceStable(dayExps, func(a, b int) bool {
				return dayExps[a].StartHour < dayExps[b].StartHour
			})
			dayEvs := evByDate[ymd]
			totalCnt := len(dayExps) + len(dayEvs)

			cardBorder := tok.Border
			if isToday {
				cardBorder = tok.Accent.Alpha(0.55)
			}
			ui.Column(c.Key("mb-ag-day-"+ymd)).FillWidth().
				Padding(float32(tok.Spacing12)).Radius(float32(tok.PanelRadius)).
				Background(tok.Surface).Border(1, cardBorder).Gap(float32(tok.Spacing8)).Children(func() {
				ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
					dayLbl := fmt.Sprintf("%d月%d日 · 周%s", day.Month(), day.Day(), weekdayCNShort(day.Weekday()))
					components.Text(c, dayLbl).FontSize(float32(tok.TextBody)).Bold().TextColor(tok.Text).FontFeatures("tnum")
					if isToday {
						kit.Badge(c, "今天", components.BadgeOptions{Tone: components.BadgeAccent})
					}
					if totalCnt > 0 {
						kit.Badge(c, fmt.Sprintf("%d 项", totalCnt), components.BadgeOptions{Tone: components.BadgeNeutral})
					}
					ui.Spacer(c)
					if components.Button(c, "+ 安排", components.ButtonGhost).Clicked() {
						s.SelectedDate = ymd
						s.triggerCreateForDate(ymd)
					}
				})

				if totalCnt == 0 {
					components.Text(c, "暂无排期事项").FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
					return
				}

				for _, exp := range dayExps {
					s.renderMobileAgendaExperimentCard(c, tok, exp)
				}
				for _, dev := range dayEvs {
					s.renderAgendaDomainEventRow(c, tok, dev)
				}
			})
		}
	})
}

func (s *Shell) renderMobileAgendaExperimentCard(c *ui.Context, tok theme.Tokens, exp ExperimentItem) {
	_, accentCol, _ := stagePillColors(tok, exp.Stage)
	timeText := "全天"
	if exp.StartHour > 0 {
		hr := int(exp.StartHour)
		mn := int(math.Round(float64(exp.StartHour-float32(hr)) * 60))
		if mn >= 60 {
			hr++
			mn -= 60
		}
		timeText = fmt.Sprintf("%02d:%02d", hr, mn)
	}
	rowH := float32(tok.Spacing48)
	if rowH < 48 {
		rowH = 48
	}
	row := ui.ButtonBase(c.Key("mb-ag-card-"+exp.ID)).FillWidth().MinHeight(rowH).
		Padding(float32(tok.Spacing8), float32(tok.Spacing12)).
		Radius(float32(tok.ControlRadius)).Background(tok.Elevated).Border(1, tok.BorderSubtle).
		Justify(ui.Start).Label(exp.Title)
	theme.ApplyTactilePress(row)
	row.Children(func() {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
			ui.Box(c).Width(float32(tok.Spacing4)*0.75).Height(float32(tok.Spacing24)).
				Radius(float32(tok.ControlRadius)).Background(accentCol).Shrink(0)
			ui.Text(c, timeText).FontSize(float32(tok.TextMeta)).Bold().
				TextColor(tok.TextMuted).FontFeatures("tnum").Shrink(0)
			ui.Column(c).Grow(1).MinWidth(0).Gap(float32(tok.Spacing4)*0.5).Children(func() {
				ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(float32(tok.Spacing4)).Children(func() {
					components.Text(c, exp.Title).FontSize(float32(tok.TextBody)).Bold().
						TextColor(tok.Text).SingleLine().Ellipsis("…").Grow(1).MinWidth(0)
					kit.Badge(c, StageLabel(exp.Stage), components.BadgeOptions{Tone: stageTone(exp.Stage)})
				})
				sub := exp.Folder
				if sub == "" {
					sub = exp.Summary
				}
				if exp.DurationMin > 0 {
					dur := formatMinutesCompact(exp.DurationMin)
					if sub != "" {
						sub += " · " + dur
					} else {
						sub = dur
					}
				}
				if sub != "" {
					components.Text(c, sub).FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted).
						SingleLine().Ellipsis("…")
				}
			})
			ui.Icon(c, lucide.ChevronRight()).Size(float32(tok.IconMd), float32(tok.IconMd)).TextColor(tok.TextMuted)
		})
	})
	if row.Clicked() {
		s.openExperiment(exp)
	}
	if exp.FilePath != "" {
		kit.ContextMenu(c, row, func(m *kit.Menu) {
			if m.Item("打开实验").Chosen() {
				s.openExperiment(exp)
			}
			if s.OnVaultDuplicate != nil && m.Item("创建副本到今天").Chosen() {
				s.OnVaultDuplicate(exp.FilePath, s.resolvedToday().Format("2006-01-02"))
			}
			if s.OnSetExperimentDate != nil && m.Item("清除排期日期").Chosen() {
				s.OnSetExperimentDate(exp.FilePath, "")
			}
			if s.OnVaultDelete != nil && m.Item("移入回收站").Chosen() {
				s.OnVaultDelete(exp.FilePath)
			}
		})
	}
}

func (s *Shell) renderMobileDayAgenda(c *ui.Context, tok theme.Tokens, ymd string, dayExps []ExperimentItem, dayDomainEvs []schedule.Event) {
	dayHeading := ymd
	if parsed, err := time.Parse("2006-01-02", ymd); err == nil {
		dayHeading = fmt.Sprintf("%d月%d日 · 周%s", parsed.Month(), parsed.Day(), weekdayCNShort(parsed.Weekday()))
	}
	ui.Column(c).FillWidth().Padding(float32(tok.Spacing8), 0).Gap(float32(tok.Spacing8)).Children(func() {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
			components.Text(c, dayHeading).FontSize(float32(tok.TextBody)).Bold().TextColor(tok.Text).Grow(1).MinWidth(0)
			if components.Button(c, "+ 当日实验", components.ButtonGhost).Clicked() {
				s.triggerCreateForDate(ymd)
			}
		})
		if len(dayExps) == 0 && len(dayDomainEvs) == 0 {
			components.Text(c, "当天暂无排期实验，点击右上角「+ 新建」安排实验。").
				FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
			return
		}
		for _, exp := range dayExps {
			s.renderMobileAgendaExperimentCard(c, tok, exp)
		}
		for _, dev := range dayDomainEvs {
			s.renderAgendaDomainEventRow(c, tok, dev)
		}
	})
}

func (s *Shell) renderAppleMonthGrid(c *ui.Context, tok theme.Tokens, cursor time.Time, todayYMD, selDateYMD string, compactV bool, byDate map[string][]ExperimentItem, evByDate map[string][]schedule.Event) {
	firstOfMonth := time.Date(cursor.Year(), cursor.Month(), 1, 0, 0, 0, 0, cursor.Location())
	lead := weekdayMondayIndex(firstOfMonth.Weekday())
	startDay := firstOfMonth.AddDate(0, 0, -lead)
	daysInMonth := time.Date(cursor.Year(), cursor.Month()+1, 0, 0, 0, 0, 0, cursor.Location()).Day()
	totalCells := 35
	if lead+daysInMonth > 35 {
		totalCells = 42
	}
	weeks := totalCells / 7
	var gridDays []time.Time
	for d := 0; d < totalCells; d++ {
		gridDays = append(gridDays, startDay.AddDate(0, 0, d))
	}

	gridKey := cursor.Format("2006-01")
	slideX := float32(s.calAnimDir) * float32(tok.Spacing24)

	gridBg := tok.Surface
	if tok.LiquidGlass {
		gridBg = tok.Surface.Alpha(0.84)
	}
	gridBox := ui.Column(c).FillWidth().Radius(float32(tok.PanelRadius)).
		Background(gridBg).Border(1, tok.Border.Alpha(0.8)).Clip()
	if !s.IsMobileLayout(c) {
		gridBox.Grow(1).MinHeight(0)
	}
	gridBox.Children(func() {
		// Apple right-aligned weekday column headers above a crisp hairline separator
		ui.Row(c).FillWidth().Gap(0).Padding(float32(tok.Spacing4), 0).Background(gridBg).Children(func() {
			for idx, wd := range []string{"周一", "周二", "周三", "周四", "周五", "周六", "周日"} {
				txtCol := tok.TextMuted
				if idx >= 5 {
					txtCol = tok.TextMuted.Alpha(.75)
				}
				ui.Row(c).Grow(1).MinWidth(0).Justify(ui.End).Padding(float32(tok.Spacing4), float32(tok.Spacing8)).Children(func() {
					components.Text(c, wd).FontSize(float32(tok.TextMeta)).Bold().TextColor(txtCol)
				})
			}
		})
		ui.Box(c).FillWidth().Height(1).Background(tok.Border)

		// Animated month grid body
		gridBody := ui.Column(c.Key("cal-month-"+gridKey)).FillWidth().Grow(1).MinHeight(0).Gap(0)
		if slideX != 0 {
			gridBody.Transition(ui.ElementTransition{
				Enter:     &ui.Motion{X: slideX, Opacity: 0},
				Animation: &theme.MotionFluid,
			})
		}
		gridBody.Children(func() {
			_, availH := c.Size()
			cellMinH := float32(tok.Spacing48) * 1.9
			if s.IsMobileLayout(c) {
				cellMinH = float32(tok.Spacing48) * 0.95
			} else if compactV || (availH > 0 && availH < 720) {
				cellMinH = float32(tok.Spacing48) * 1.45
			}
			maxShow := 3
			if weeks == 6 || compactV {
				maxShow = 2
			}
			for w := 0; w < weeks; w++ {
				week := gridDays[w*7 : (w+1)*7]
				if w > 0 {
					ui.Box(c).FillWidth().Height(1).Background(tok.Border)
				}
				ui.Row(c).FillWidth().Grow(1).MinHeight(cellMinH).Gap(0).Children(func() {
					for colIdx, day := range week {
						day := day
						if colIdx > 0 {
							ui.Box(c).FillHeight().Width(1).Background(tok.Border)
						}
						ymd := day.Format("2006-01-02")
						isToday := ymd == todayYMD
						isSelected := ymd == selDateYMD
						inMonth := day.Year() == cursor.Year() && day.Month() == cursor.Month()
						weekend := isWeekend(day.Weekday())
						dayExps := byDate[ymd]
						dayDomainEvs := evByDate[ymd]

						cellBg := tok.Surface
						if weekend {
							cellBg = tok.Elevated
						}
						if isSelected && !isToday {
							cellBg = tok.AccentSoftBg.Mix(cellBg, .5)
						}

						btn := ui.ButtonBase(c.Key("cal-"+ymd)).Grow(1).MinWidth(0).FillHeight().
							Label("日历格 " + ymd).
							Justify(ui.Start).AlignItems(ui.Start).
							Padding(float32(tok.Spacing4), float32(tok.Spacing4)).
							Background(cellBg)
						dayClicked := false
						dayDoubleClicked := false
						btn.Children(func() {
							ui.Column(c).FillWidth().Gap(float32(tok.Spacing4)).Children(func() {
								// Top bar inside day cell: optional month label + top-right Apple day badge
								mobileCell := s.IsMobileLayout(c)
								hdrBtn := ui.ButtonBase(c.Key("cal-hdr-"+ymd)).FillWidth().Label(ymd).
									Justify(ui.Start).AlignItems(ui.Center)
								hdrBtn.Children(func() {
									if mobileCell {
										ui.Row(c).FillWidth().Center().Children(func() {
											if day.Day() == 1 {
												txtColor := tok.Accent
												if isToday {
													txtColor = tok.AccentText
												}
												pill := ui.Box(c.Key("mob-m1-"+ymd)).Height(float32(tok.Spacing24)).
													Padding(0, float32(tok.Spacing4)).Radius(float32(tok.Spacing24) * 0.5).Center()
												if isToday {
													pill.Background(tok.Accent)
												} else if isSelected {
													pill.Background(tok.AccentSoftBg).Border(1, tok.Accent)
												}
												pill.Children(func() {
													components.Text(c, fmt.Sprintf("%d/%d", day.Month(), day.Day())).
														FontSize(float32(tok.TextMeta)).Bold().TextColor(txtColor).
														FontFeatures("tnum").SingleLine()
												})
											} else {
												s.renderAppleDayNumberBadge(c, tok, day.Day(), isToday, isSelected, inMonth)
											}
										})
									} else {
										ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(float32(tok.Spacing4)).Children(func() {
											if day.Day() == 1 {
												components.Text(c, fmt.Sprintf("%d月", day.Month())).
													FontSize(float32(tok.TextMeta)).Bold().TextColor(tok.Accent)
											}
											ui.Spacer(c)
											s.renderAppleDayNumberBadge(c, tok, day.Day(), isToday, isSelected, inMonth)
										})
									}
								})
								if hdrBtn.DoubleClicked() {
									dayDoubleClicked = true
								}
								if hdrBtn.Clicked() {
									dayClicked = true
								}
								if s.IsMobileLayout(c) {
									if len(dayExps) > 0 || len(dayDomainEvs) > 0 {
										ui.Row(c).FillWidth().Justify(ui.Center).Wrap().Gap(float32(tok.Spacing4)*0.75).Children(func() {
											shownDots := 0
											for _, exp := range dayExps {
												if shownDots >= 4 {
													break
												}
												_, accentCol, _ := stagePillColors(tok, exp.Stage)
												ui.Box(c).Size(float32(tok.Spacing4)*1.5, float32(tok.Spacing4)*1.5).
													Radius(float32(tok.ControlRadius)).Background(accentCol)
												shownDots++
											}
											for _, dev := range dayDomainEvs {
												if shownDots >= 4 {
													break
												}
												_, dotCol, _ := domainEventColors(tok, dev)
												ui.Box(c).Size(float32(tok.Spacing4)*1.5, float32(tok.Spacing4)*1.5).
													Radius(float32(tok.ControlRadius)).Background(dotCol)
												shownDots++
											}
										})
									}
								} else {
									shown := 0
									totalCellItems := len(dayExps) + len(dayDomainEvs)
									for _, exp := range dayExps {
										if shown >= maxShow {
											break
										}
										s.renderAppleEventPill(c, tok, exp, false)
										shown++
									}
									for _, dev := range dayDomainEvs {
										if shown >= maxShow {
											break
										}
										s.renderDomainEventPill(c, tok, dev)
										shown++
									}
									if totalCellItems > shown {
										ui.Text(c, fmt.Sprintf("还有 +%d 项…", totalCellItems-shown)).
											FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted).
											Padding(0, float32(tok.Spacing4))
									}
								}
							})
						})

						dbl := dayDoubleClicked || btn.DoubleClicked()
						if dayClicked || btn.Clicked() {
							now := time.Now()
							if dbl || (s.lastDayClickYMD == ymd && !s.lastDayClickAt.IsZero() && now.Sub(s.lastDayClickAt) <= 450*time.Millisecond) {
								s.lastDayClickYMD = ""
								s.lastDayClickAt = time.Time{}
								s.DayPopoverOpen = false
								s.SelectedDate = ymd
								s.triggerCreateForDate(ymd)
							} else {
								s.lastDayClickYMD = ymd
								s.lastDayClickAt = now
								s.SelectedDate = ymd
								if !s.IsMobileLayout(c) {
									s.DayPopoverDate = ymd
									s.DayPopoverOpen = true
								} else {
									s.DayPopoverOpen = false
								}
								if !inMonth {
									s.CalendarCursor = ymd
								}
							}
						}
						if ymd == s.DayPopoverDate {
							components.Popover(c, btn, &s.DayPopoverOpen, func(panel ui.Element) {
								s.renderDayPopover(c, panel, tok, ymd, dayExps, dayDomainEvs)
							})
						}
						if dragged, over := ui.DragOver[fileDragPayload](btn); over && !dragged.IsDir {
							btn.Border(2, tok.Accent)
						}
						if dragged, dropped := ui.Drop[fileDragPayload](btn); dropped && !dragged.IsDir && dragged.RelPath != "" {
							if s.OnSetExperimentDate != nil {
								s.OnSetExperimentDate(dragged.RelPath, ymd)
							}
						}
						if dragged, over := ui.DragOver[ExperimentItem](btn); over && dragged.FilePath != "" {
							btn.Border(2, tok.Accent)
						}
						if dragged, dropped := ui.Drop[ExperimentItem](btn); dropped && dragged.FilePath != "" {
							if s.OnSetExperimentDate != nil {
								s.OnSetExperimentDate(dragged.FilePath, ymd)
							}
						}
						kit.ContextMenu(c, btn, func(m *kit.Menu) {
							if m.Item("在此日新建实验").Chosen() {
								s.SelectedDate = ymd
								s.triggerCreateForDate(ymd)
							}
							if s.OnCreateReagentForDate != nil && m.Item("在此日登记试剂购入…").Chosen() {
								s.SelectedDate = ymd
								s.OnCreateReagentForDate(ymd)
							}
							if s.OnCreateAnimalForDate != nil && m.Item("在此日登记动物入栏 / 收样…").Chosen() {
								s.SelectedDate = ymd
								s.OnCreateAnimalForDate(ymd)
							}
							if s.OnOpenTemplatePicker != nil && m.Item("从模板新建…").Chosen() {
								s.SelectedDate = ymd
								s.OnOpenTemplatePicker(ymd)
							}
						})
					}
				})
			}
		})
	})
}

func (s *Shell) renderAppleDayNumberBadge(c *ui.Context, tok theme.Tokens, dayNum int, isToday, isSelected, inMonth bool) {
	numStr := fmt.Sprintf("%d", dayNum)
	badgeSize := float32(tok.Spacing24)
	circleRad := badgeSize * 0.5
	if isToday {
		ui.Box(c.Key("today-pill")).Size(badgeSize, badgeSize).Radius(circleRad).
			Background(tok.Accent).Center().
			Transition(ui.ElementTransition{Animation: &theme.MotionSnappy}).Children(func() {
			components.Text(c, numStr).FontSize(float32(tok.TextMeta)).Bold().
				TextColor(tok.AccentText).FontFeatures("tnum")
		})
		return
	}
	if isSelected {
		ui.Box(c.Key("sel-pill")).Size(badgeSize, badgeSize).Radius(circleRad).
			Background(tok.AccentSoftBg).Border(1, tok.Accent).Center().
			Transition(theme.SharedPillTransition("cal-selected-circle")).Children(func() {
			components.Text(c, numStr).FontSize(float32(tok.TextMeta)).Bold().
				TextColor(tok.Accent).FontFeatures("tnum")
		})
		return
	}
	numColor := tok.Text
	if !inMonth {
		numColor = tok.TextMuted.Alpha(.5)
	}
	ui.Box(c).Size(badgeSize, badgeSize).Center().Children(func() {
		components.Text(c, numStr).FontSize(float32(tok.TextMeta)).
			TextColor(numColor).FontFeatures("tnum")
	})
}

func (s *Shell) renderAppleEventPill(c *ui.Context, tok theme.Tokens, exp ExperimentItem, showDuration bool) {
	bg, bar, fg := stagePillColors(tok, exp.Stage)
	pill := ui.Row(c.Key("cal-pill-"+exp.ID)).FillWidth().Padding(float32(tok.Spacing4)*0.35, float32(tok.Spacing4)).
		Radius(float32(tok.ControlRadius)).Background(bg).AlignItems(ui.Center).
		Gap(float32(tok.Spacing4)).Children(func() {
		ui.Box(c).Width(float32(tok.Spacing4)*0.75).Height(float32(tok.Spacing12)).
			Radius(float32(tok.ControlRadius)*0.35).Background(bar).Shrink(0)
		components.Text(c, exp.Title).FontSize(float32(tok.TextMeta)).TextColor(fg).
			SingleLine().Ellipsis("…").Grow(1).MinWidth(0)
		if showDuration && exp.DurationMin > 0 {
			ui.Text(c, formatMinutesCompact(exp.DurationMin)).FontSize(float32(tok.TextMeta)).
				TextColor(fg).FontFeatures("tnum").Shrink(0)
		}
	})
	if exp.FilePath != "" {
		pill.Drag(exp)
		kit.ContextMenu(c, pill, func(m *kit.Menu) {
			if m.Item("打开").Chosen() {
				s.openExperiment(exp)
			}
			if m.Item("向右分屏打开").Chosen() {
				s.openExperimentInSplit(exp)
			}
			if s.OnVaultDuplicate != nil && m.Item("创建副本到今天").Chosen() {
				s.OnVaultDuplicate(exp.FilePath, s.resolvedToday().Format("2006-01-02"))
			}
			if s.OnSetExperimentDate != nil && m.Item("清除排期日期").Chosen() {
				s.OnSetExperimentDate(exp.FilePath, "")
			}
			if len(s.VaultFolders) > 0 && s.OnVaultMove != nil {
				m.Group("移动至项目", func(sub *kit.Menu) {
					if sub.Item("根目录 /").Chosen() {
						s.OnVaultMove(exp.FilePath, "")
					}
					for _, dir := range s.VaultFolders {
						dir := dir
						if sub.Item(dir).Chosen() {
							s.OnVaultMove(exp.FilePath, dir)
						}
					}
				})
			}
			m.Separator()
			if s.OnVaultDelete != nil && m.Item("删除").Chosen() {
				s.OnVaultDelete(exp.FilePath)
			}
		})
	}
}

func domainEventColors(tok theme.Tokens, ev schedule.Event) (bg, bar, fg ui.Color) {
	if ev.Done {
		return tok.Elevated, tok.BorderStrong, tok.TextMuted
	}
	switch ev.Severity {
	case schedule.SeverityDanger:
		return tok.DangerBg, tok.Danger, tok.DangerText
	case schedule.SeverityWarning:
		return tok.WarningBg, tok.Warning, tok.WarningText
	case schedule.SeveritySuccess:
		return tok.SuccessBg, tok.Success, tok.SuccessText
	default:
		if ev.Domain == schedule.DomainReagent {
			return tok.InfoBg, tok.Accent, tok.InfoText
		}
		return tok.AccentSoftBg, tok.Accent, tok.AccentText
	}
}

func domainEventIcon(ev schedule.Event) *ui.SVG {
	if ev.Domain == schedule.DomainVivarium {
		return lucide.PawPrint()
	}
	if ev.Domain == schedule.DomainReagent {
		return lucide.Package()
	}
	return lucide.FlaskConical()
}

func (s *Shell) renderDomainEventPill(c *ui.Context, tok theme.Tokens, ev schedule.Event) {
	bg, bar, fg := domainEventColors(tok, ev)
	btn := ui.ButtonBase(c.Key("cal-dom-"+ev.ID)).FillWidth().Justify(ui.Start).
		Padding(float32(tok.Spacing4)*0.35, float32(tok.Spacing4)).
		Radius(float32(tok.ControlRadius)).Background(bg).Label(ev.Title)
	btn.Children(func() {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(float32(tok.Spacing4)).Children(func() {
			ui.Box(c).Width(float32(tok.Spacing4)*0.75).Height(float32(tok.Spacing12)).
				Radius(float32(tok.ControlRadius)*0.35).Background(bar).Shrink(0)
			ui.Icon(c, domainEventIcon(ev)).Size(float32(tok.Spacing12), float32(tok.Spacing12)).TextColor(bar).Shrink(0)
			components.Text(c, ev.Title).FontSize(float32(tok.TextMeta)).TextColor(fg).
				SingleLine().Ellipsis("…").Grow(1).MinWidth(0)
			if ev.Done {
				ui.Icon(c, lucide.Check()).Size(float32(tok.Spacing12), float32(tok.Spacing12)).TextColor(tok.Success).Shrink(0)
			}
		})
	})
	if btn.Clicked() && s.OnOpenScheduleEvent != nil {
		s.DayPopoverOpen = false
		s.OnOpenScheduleEvent(ev)
	}
}

func (s *Shell) renderAppleWeekTimeGrid(c *ui.Context, tok theme.Tokens, cursor time.Time, todayYMD, selDateYMD string, byDate map[string][]ExperimentItem, evByDate map[string][]schedule.Event) {
	wStart := cursor.AddDate(0, 0, -weekdayMondayIndex(cursor.Weekday()))
	days := make([]time.Time, 7)
	for i := 0; i < 7; i++ {
		days[i] = wStart.AddDate(0, 0, i)
	}
	gutterW := float32(tok.Spacing48) + float32(tok.Spacing8)

	ui.Column(c).FillWidth().Grow(1).MinHeight(0).Radius(float32(tok.PanelRadius)).
		Background(tok.Surface).Border(1, tok.Border).Clip().Children(func() {
		// 1. Day Header Row
		ui.Row(c).FillWidth().Background(tok.Surface).Gap(0).Padding(float32(tok.Spacing4), 0).Children(func() {
			ui.Box(c).Width(gutterW).Shrink(0)
			for _, day := range days {
				day := day
				ymd := day.Format("2006-01-02")
				isToday := ymd == todayYMD
				isSel := ymd == selDateYMD
				dayExps := byDate[ymd]
				ui.Box(c).FillHeight().Width(1).Background(tok.Border)
				hdrBtn := ui.ButtonBase(c.Key("wk-hdr-"+ymd)).Grow(1).MinWidth(0).Label(ymd).
					Padding(float32(tok.Spacing4), float32(tok.Spacing8))
				hdrBtn.Children(func() {
					ui.Row(c).FillWidth().AlignItems(ui.Center).Justify(ui.Center).Gap(float32(tok.Spacing4)).Children(func() {
						components.Text(c, "周"+weekdayCNShort(day.Weekday())).
							FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
						s.renderAppleDayNumberBadge(c, tok, day.Day(), isToday, isSel, true)
					})
				})
				if hdrBtn.Clicked() {
					s.SelectedDate = ymd
					if !s.IsMobileLayout(c) {
						s.DayPopoverDate = ymd
						s.DayPopoverOpen = true
					}
				}
				if ymd == s.DayPopoverDate {
					components.Popover(c, hdrBtn, &s.DayPopoverOpen, func(panel ui.Element) {
						s.renderDayPopover(c, panel, tok, ymd, dayExps, evByDate[ymd])
					})
				}
			}
		})
		ui.Box(c).FillWidth().Height(1).Background(tok.Border)

		// 2. All-Day / Unscheduled Hour Strip
		ui.Row(c).FillWidth().Background(tok.Elevated).Gap(0).Children(func() {
			ui.Row(c).Width(gutterW).Shrink(0).Padding(float32(tok.Spacing4), float32(tok.Spacing8)).
				AlignItems(ui.Center).Justify(ui.End).Children(func() {
				ui.Text(c, "全天").FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
			})
			for _, day := range days {
				day := day
				ymd := day.Format("2006-01-02")
				ui.Box(c).FillHeight().Width(1).Background(tok.Border)
				ui.Column(c).Grow(1).MinWidth(0).Padding(float32(tok.Spacing4)).Gap(float32(tok.Spacing4)).Children(func() {
					allDay := filterAllDayExps(byDate[ymd])
					for _, exp := range allDay {
						exp := exp
						pillBtn := ui.ButtonBase(c.Key("wk-allday-"+exp.ID)).FillWidth().Justify(ui.Start).Label(exp.Title)
						pillBtn.Children(func() {
							s.renderAppleEventPill(c, tok, exp, true)
						})
						if pillBtn.Clicked() {
							s.openExperiment(exp)
						}
					}
					for _, dev := range evByDate[ymd] {
						s.renderDomainEventPill(c, tok, dev)
					}
				})
			}
		})
		ui.Box(c).FillWidth().Height(1).Background(tok.Border)

		// 3. Scrollable 07:00 – 22:00 Hourly Axis (UI-09 & UI-18: full-width continuous timeline
		// with gentle 30-minute magnetic snap across all 7 weekday columns).
		curHour := s.CurrentHour
		if curHour <= 0 {
			now := time.Now()
			curHour = float32(now.Hour()) + float32(now.Minute())/60.0
		}
		_, availH := c.Size()
		const startHr = 7
		const endHr = 22
		rowCount := float32(endHr - startHr + 1)
		rowH := float32(tok.Spacing48)
		if availH > 260 {
			fitH := (availH - 220) / rowCount
			if fitH > rowH {
				rowH = fitH
			}
		}
		totalGridH := rowH * rowCount

		ui.Scroll(c).FillWidth().Grow(1).MinHeight(0).Children(func() {
			// UI-04: Virtualized single-canvas week hourly grid (O(1) layout nodes instead of
			// 112 nested slot boxes) — eliminates hover lag completely and keeps the magnetic
			// 30-minute timeline butter-smooth across all 7 weekday columns.
			axisBox := ui.Box(c.Key("wk-axis-canvas")).FillWidth().Height(totalGridH)
			px, py, insideAxis := axisBox.PointerPosition()
			if s.AnyModalOpen() {
				s.weekHoverOn = false
			} else if insideAxis && py >= 0 && py <= totalGridH {
				s.weekHoverOn = true
				s.weekHoverY = py
			} else {
				s.weekHoverOn = false
			}

			// Single GPU Draw pass for all 7 day columns, hour lines, time labels, and current-time line.
			axisBox.Draw(func(p *ui.Painter, r ui.Rect) {
				gridW := r.W - gutterW
				if gridW <= 0 {
					return
				}
				colW := gridW / 7.0
				// 1. Column weekend shading + vertical separators
				for dIdx, day := range days {
					colX := r.X + gutterW + float32(dIdx)*colW
					if isWeekend(day.Weekday()) {
						p.Fill(ui.Rect{X: colX, Y: r.Y, W: colW, H: r.H}, tok.Elevated, 0)
					}
					p.Fill(ui.Rect{X: colX, Y: r.Y, W: 1, H: r.H}, tok.Border, 0)
				}
				// 2. Horizontal hour dividers (07:00 .. 22:00)
				for hr := startHr; hr <= endHr; hr++ {
					y := r.Y + float32(hr-startHr)*rowH
					if hr > startHr {
						p.Fill(ui.Rect{X: r.X, Y: y, W: r.W, H: 1}, tok.Border, 0)
					}
				}
				// Closing bottom hairline
				p.Fill(ui.Rect{X: r.X, Y: r.Y + r.H - 1, W: r.W, H: 1}, tok.Border.Alpha(0.6), 0)

				// 3. Today's live red current-time indicator
				if curHour >= float32(startHr) && curHour <= float32(endHr+1) {
					for dIdx, day := range days {
						if day.Format("2006-01-02") == todayYMD {
							cy := r.Y + (curHour-float32(startHr))*rowH
							colX := r.X + gutterW + float32(dIdx)*colW
							dotR := float32(tok.Spacing4)
							p.Fill(ui.Rect{X: colX - dotR, Y: cy - dotR, W: dotR * 2, H: dotR * 2}, tok.Danger, dotR)
							p.Fill(ui.Rect{X: colX, Y: cy - 0.75, W: colW, H: 1.5}, tok.Danger, 0)
							break
						}
					}
				}
			})

			// Mount accessible left hour gutter labels (16 static labels) + real buttons ONLY for scheduled hourly events.
			axisBox.Children(func() {
				ui.Row(c).Fill().Gap(0).Children(func() {
					ui.Column(c).Width(gutterW).FillHeight().Shrink(0).Gap(0).Children(func() {
						for hr := startHr; hr <= endHr; hr++ {
							ui.Row(c).FillWidth().Height(rowH).Padding(float32(tok.Spacing4), float32(tok.Spacing8)).
								AlignItems(ui.Start).Justify(ui.End).Children(func() {
								ui.Text(c, fmt.Sprintf("%02d:00", hr)).FontSize(float32(tok.TextMeta)).
									TextColor(tok.TextMuted).FontFeatures("tnum").SingleLine()
							})
						}
					})
					for _, day := range days {
						day := day
						ymd := day.Format("2006-01-02")
						ui.Box(c.Key("wk-col-"+ymd)).Grow(1).MinWidth(0).FillHeight().Children(func() {
							for _, exp := range byDate[ymd] {
								exp := exp
								if exp.StartHour < float32(startHr) || exp.StartHour > float32(endHr)+0.95 {
									continue
								}
								topY := (exp.StartHour - float32(startHr)) * rowH
								bg, bar, fg := stagePillColors(tok, exp.Stage)
								blk := ui.ButtonBase(c.Key("wk-hr-"+exp.ID)).Absolute().
									Left(float32(tok.Spacing4)).Right(float32(tok.Spacing4)).Top(topY + 2).
									Justify(ui.Start).
									Padding(float32(tok.Spacing4), float32(tok.Spacing8)).
									Radius(float32(tok.ControlRadius)).Background(bg).Label(exp.Title)
								blk.Children(func() {
									ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(float32(tok.Spacing4)).Children(func() {
										ui.Box(c).Width(float32(tok.Spacing4)*0.75).Height(float32(tok.Spacing16)).
											Radius(float32(tok.ControlRadius)*0.35).Background(bar).Shrink(0)
										components.Text(c, exp.Title).FontSize(float32(tok.TextMeta)).Bold().
											TextColor(fg).SingleLine().Ellipsis("…").Grow(1).MinWidth(0)
										if exp.DurationMin > 0 {
											ui.Text(c, formatMinutesCompact(exp.DurationMin)).
												FontSize(float32(tok.TextMeta)).TextColor(fg).FontFeatures("tnum").Shrink(0)
										}
									})
								})
								if blk.Clicked() {
									s.openExperiment(exp)
								}
							}
						})
					}
				})
			})

			if !s.AnyModalOpen() && axisBox.DoubleClicked() && px > gutterW {
				bounds := axisBox.LayoutBounds()
				gridW := bounds.W - gutterW
				if gridW > 0 {
					dIdx := int((px - gutterW) / (gridW / 7.0))
					if dIdx >= 0 && dIdx < 7 {
						ymd := days[dIdx].Format("2006-01-02")
						s.SelectedDate = ymd
						totalMins, _, _ := weekGridYToMinutes(py, rowH, startHr, endHr)
						startHHMM := fmt.Sprintf("%02d:%02d", totalMins/60, totalMins%60)
						s.weekHoverOn = false
						s.triggerCreateForDateTime(ymd, startHHMM)
					}
				}
			}

			// Smooth click-through hover timeline with gentle ±7px 30-min snap and 5-min fine precision (:05/:15...).
			axisBox.DrawOver(func(p *ui.Painter, r ui.Rect) {
				if s.AnyModalOpen() || !s.weekHoverOn || s.weekHoverY < 0 || s.weekHoverY > totalGridH {
					return
				}
				totalMins, snapped, lineY := weekGridYToMinutes(s.weekHoverY, rowH, startHr, endHr)
				timeLbl := fmt.Sprintf("%02d:%02d", totalMins/60, totalMins%60)
				yAbs := r.Y + lineY
				lineRect := ui.Rect{X: r.X + gutterW, Y: yAbs - 0.75, W: r.W - gutterW, H: 1.5}
				p.Fill(lineRect, tok.Accent.Alpha(0.75), 0)
				dotR := float32(3)
				if snapped {
					dotR = 4
				}
				p.Fill(ui.Rect{X: r.X + gutterW - dotR, Y: yAbs - dotR, W: dotR * 2, H: dotR * 2}, tok.Accent, dotR)
				pillW := gutterW - float32(tok.Spacing8)
				pillH := float32(tok.Spacing16) + 2
				pillX := r.X + float32(tok.Spacing4)
				pillY := yAbs - pillH*0.5
				if pillY < r.Y {
					pillY = r.Y
				}
				p.Fill(ui.Rect{X: pillX, Y: pillY, W: pillW, H: pillH}, tok.Accent, float32(tok.ControlRadius))
				fontPx := float32(tok.TextMeta) - 1
				p.Text(pillX+float32(tok.Spacing4), pillY+(pillH-fontPx)*0.42, timeLbl, fontPx, tok.AccentText)
			})
		})
	})
}

func filterAllDayExps(exps []ExperimentItem) []ExperimentItem {
	var out []ExperimentItem
	for _, exp := range exps {
		if exp.StartHour <= 0 {
			out = append(out, exp)
		}
	}
	return out
}

func formatMinutesCompact(min int) string {
	if min < 60 {
		return fmt.Sprintf("%dm", min)
	}
	h := min / 60
	m := min % 60
	if m == 0 {
		return fmt.Sprintf("%dh", h)
	}
	return fmt.Sprintf("%dh%dm", h, m)
}

func (s *Shell) renderDayPopover(c *ui.Context, panel ui.Element, tok theme.Tokens, ymd string, exps []ExperimentItem, domEvs []schedule.Event) {
	dateLabel := ymd
	if t, err := time.Parse("2006-01-02", ymd); err == nil {
		dateLabel = fmt.Sprintf("%d 月 %d 日（周%s）", t.Month(), t.Day(), weekdayCNShort(t.Weekday()))
	}
	totalItems := len(exps) + len(domEvs)

	popoverTitle := dateLabel + " · 实验安排"
	if len(domEvs) > 0 {
		popoverTitle = dateLabel + " · 综合日程"
	}

	panelW := float32(tok.Spacing48) * 9.6
	if vw, _ := c.Size(); vw > 0 && panelW > vw-float32(tok.Spacing24) {
		panelW = max(float32(tok.Spacing48)*5, vw-float32(tok.Spacing24))
	}
	panel.Width(panelW).MaxHeight(float32(tok.Spacing48)*10.5).
		Label(popoverTitle).Children(func() {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
			ui.Icon(c, lucide.CalendarDays()).Size(float32(tok.Spacing16), float32(tok.Spacing16)).TextColor(tok.Accent)
			components.Text(c, popoverTitle).FontSize(float32(tok.TextBody)).Bold().TextColor(tok.Text).
				SingleLine().Ellipsis("…").Grow(1).MinWidth(0)
			kit.Badge(c, fmt.Sprintf("%d 项", totalItems), components.BadgeOptions{Tone: components.BadgeNeutral})
		})

		if totalItems == 0 {
			ui.Column(c).FillWidth().Padding(float32(tok.Spacing12)).Radius(float32(tok.ControlRadius)).
				Background(tok.Elevated).Border(1, tok.Border).Gap(float32(tok.Spacing8)).Children(func() {
				components.Text(c, "该日期暂无已安排实验、试剂提醒或动物房节点。").FontSize(float32(tok.TextBody)).TextColor(tok.TextMuted)
				ui.Row(c).FillWidth().Wrap().AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
					if s.OnCreateForDate != nil || s.OnCreate != nil {
						if components.Button(c, "＋ 在此日新建实验", components.ButtonPrimary).Clicked() {
							s.triggerCreateForDate(ymd)
						}
					}
					if s.OnCreateReagentForDate != nil && components.Button(c, "+ 试剂购入", components.ButtonSecondary).Clicked() {
						s.DayPopoverOpen = false
						s.OnCreateReagentForDate(ymd)
					}
					if s.OnCreateAnimalForDate != nil && components.Button(c, "+ 动物入栏", components.ButtonSecondary).Clicked() {
						s.DayPopoverOpen = false
						s.OnCreateAnimalForDate(ymd)
					}
				})
			})
			return
		}

		ui.Scroll(c).FillWidth().MaxHeight(float32(tok.Spacing48) * 9.0).Children(func() {
			ui.Column(c).FillWidth().Gap(float32(tok.Spacing8)).Children(func() {
				if len(exps) > 0 {
					ui.Text(c, fmt.Sprintf("实验安排 (%d)", len(exps))).FontSize(float32(tok.TextMeta)).Bold().TextColor(tok.TextMuted)
					for _, exp := range exps {
						exp := exp
						s.renderAgendaExperimentRow(c, tok, exp)
					}
				}
				var reagentEvs, animalEvs []schedule.Event
				for _, ev := range domEvs {
					if ev.Domain == schedule.DomainReagent {
						reagentEvs = append(reagentEvs, ev)
					} else if ev.Domain == schedule.DomainVivarium {
						animalEvs = append(animalEvs, ev)
					}
				}
				if len(reagentEvs) > 0 {
					ui.Text(c, fmt.Sprintf("试剂购入与效期 (%d)", len(reagentEvs))).FontSize(float32(tok.TextMeta)).Bold().TextColor(tok.TextMuted)
					for _, ev := range reagentEvs {
						s.renderAgendaDomainEventRow(c, tok, ev)
					}
				}
				if len(animalEvs) > 0 {
					ui.Text(c, fmt.Sprintf("动物房节点 (%d)", len(animalEvs))).FontSize(float32(tok.TextMeta)).Bold().TextColor(tok.TextMuted)
					for _, ev := range animalEvs {
						s.renderAgendaDomainEventRow(c, tok, ev)
					}
				}
			})
		})

		if s.OnCreateForDate != nil || s.OnCreate != nil || s.OnCreateReagentForDate != nil || s.OnCreateAnimalForDate != nil {
			ui.Row(c).FillWidth().Wrap().AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
				if s.OnCreateForDate != nil || s.OnCreate != nil {
					if components.Button(c, "＋ 在此日新建实验", components.ButtonSecondary).Clicked() {
						s.triggerCreateForDate(ymd)
					}
				}
				if s.OnCreateReagentForDate != nil && components.Button(c, "+ 登记试剂购入", components.ButtonGhost).Clicked() {
					s.DayPopoverOpen = false
					s.OnCreateReagentForDate(ymd)
				}
				if s.OnCreateAnimalForDate != nil && components.Button(c, "+ 登记动物入栏", components.ButtonGhost).Clicked() {
					s.DayPopoverOpen = false
					s.OnCreateAnimalForDate(ymd)
				}
			})
		}
	})
}

func (s *Shell) renderAgendaDomainEventRow(c *ui.Context, tok theme.Tokens, ev schedule.Event) {
	bg, bar, fg := domainEventColors(tok, ev)
	tone := components.BadgeNeutral
	switch ev.Severity {
	case schedule.SeverityDanger:
		tone = components.BadgeDanger
	case schedule.SeverityWarning:
		tone = components.BadgeWarning
	case schedule.SeveritySuccess:
		tone = components.BadgeSuccess
	default:
		tone = components.BadgeAccent
	}
	ui.Row(c.Key("agenda-dom-"+ev.ID)).FillWidth().Padding(float32(tok.Spacing8)).
		Radius(float32(tok.ControlRadius)).Background(tok.Elevated).Border(1, tok.Border).
		AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
		mainBtn := ui.ButtonBase(c).Grow(1).MinWidth(0).Justify(ui.Start).Label(ev.Title)
		mainBtn.Children(func() {
			ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
				ui.Box(c).Size(float32(tok.Spacing24), float32(tok.Spacing24)).
					Radius(float32(tok.ControlRadius)).Background(bg).Center().Children(func() {
					ui.Icon(c, domainEventIcon(ev)).Size(float32(tok.Spacing12), float32(tok.Spacing12)).TextColor(bar)
				})
				ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
					ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(float32(tok.Spacing4)).Children(func() {
						components.Text(c, ev.Title).FontSize(float32(tok.TextBody)).Bold().TextColor(fg).SingleLine().Ellipsis("…").Grow(1).MinWidth(0)
						kit.Badge(c, schedule.KindLabel(ev.Kind), components.BadgeOptions{Tone: tone})
					})
					if ev.Subtitle != "" {
						components.Text(c, ev.Subtitle).FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted).SingleLine().Ellipsis("…")
					}
				})
			})
		})
		if mainBtn.Clicked() && s.OnOpenScheduleEvent != nil {
			s.DayPopoverOpen = false
			s.OnOpenScheduleEvent(ev)
		}
		if ev.MilestoneID != "" && s.OnToggleScheduleEventDone != nil {
			btnLbl := "✓ 完成"
			if ev.Done {
				btnLbl = "已完成"
			}
			if components.Button(c, btnLbl, components.ButtonSecondary).Clicked() {
				s.OnToggleScheduleEventDone(ev)
			}
		}
	})
}

func (s *Shell) renderAgendaExperimentRow(c *ui.Context, tok theme.Tokens, exp ExperimentItem) {
	detailLine := exp.Summary
	switch exp.Stage {
	case StageDraft:
		hint := exp.GapHint
		if hint == "" {
			hint = "方案尚不完整"
		}
		detailLine = "缺口：" + hint
	case StageReady:
		if detailLine == "" {
			detailLine = "方案已就绪，可直接开始实验"
		}
	case StageInProgress:
		if detailLine == "" {
			detailLine = "实验进行中，进入运行台录入实测"
		}
	case StageCompleted:
		if detailLine == "" {
			detailLine = "实验已完成，可查阅记录并导出 PDF"
		}
	}

	ui.Row(c.Key("agenda-"+exp.ID)).FillWidth().Padding(float32(tok.Spacing8)).
		Radius(float32(tok.ControlRadius)).Background(tok.Elevated).Border(1, tok.Border).
		AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
		mainBtn := ui.ButtonBase(c).Grow(1).MinWidth(0).Justify(ui.Start).Label("打开 " + exp.Title)
		mainBtn.Children(func() {
			ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
				ui.Box(c).Size(float32(tok.Spacing24), float32(tok.Spacing24)).
					Radius(float32(tok.ControlRadius)).Background(tok.AccentSoftBg).Center().Children(func() {
					ui.Icon(c, stageIcon(exp.Stage)).Size(float32(tok.Spacing12), float32(tok.Spacing12)).TextColor(tok.Accent)
				})
				ui.Column(c).Grow(1).MinWidth(0).Gap(float32(tok.Spacing4)*0.5).Children(func() {
					ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(float32(tok.Spacing4)).Children(func() {
						components.Text(c, exp.Title).FontSize(float32(tok.TextBody)).Bold().TextColor(tok.Text).
							SingleLine().Ellipsis("…").Grow(1).MinWidth(0)
						components.Badge(c, StageLabel(exp.Stage), components.BadgeOptions{Tone: stageTone(exp.Stage)})
					})
					components.Text(c, detailLine).FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted).
						SingleLine().Ellipsis("…")
				})
			})
		})
		if mainBtn.Clicked() {
			s.openExperiment(exp)
		}

		splitBtn := components.IconButton(c, lucide.Columns2(), "在右侧分屏中打开", components.IconButtonCompact)
		if splitBtn.Clicked() {
			s.openExperimentInSplit(exp)
		}

		switch exp.Stage {
		case StageDraft:
			if components.Button(c, "继续设计", components.ButtonSecondary).Clicked() {
				s.openExperiment(exp)
			}
		case StageReady:
			if exp.DesignID != "" && s.OnQuickStartRun != nil {
				if components.Button(c, "开始实验", components.ButtonSecondary).Clicked() {
					s.DayPopoverOpen = false
					s.OnQuickStartRun(exp.DesignID)
				}
			} else if components.Button(c, "打开", components.ButtonSecondary).Clicked() {
				s.openExperiment(exp)
			}
		case StageInProgress:
			if components.Button(c, "继续运行", components.ButtonSecondary).Clicked() {
				s.openExperiment(exp)
			}
		case StageCompleted:
			if components.Button(c, "查看记录", components.ButtonSecondary).Clicked() {
				s.openExperiment(exp)
			}
		}
	})
}

// weekGridYToMinutes maps a vertical pixel offset inside the week-grid time body
// to clock minutes, snapping within ±7 px of :00/:30 slots and resolving to
// 5-minute increments everywhere else. Hover preview and double-click creation
// share this helper so clicking always creates at the exact previewed time.
func weekGridYToMinutes(y, rowH float32, startHr, endHr int) (totalMins int, snapped bool, displayY float32) {
	if rowH <= 0 {
		return startHr * 60, true, 0
	}
	halfH := rowH * 0.5
	nearestStep := float32(math.Round(float64(y / halfH)))
	snapY := nearestStep * halfH
	if abs32Cal(y-snapY) <= 7 {
		totalMins = int(nearestStep)*30 + startHr*60
		snapped = true
		displayY = snapY
	} else {
		totalMins = int(math.Round(float64(y/rowH)*12.0))*5 + startHr*60
		displayY = y
	}
	minMins := startHr * 60
	maxMins := endHr*60 + 55
	if totalMins < minMins {
		totalMins = minMins
	}
	if totalMins > maxMins {
		totalMins = maxMins
	}
	return totalMins, snapped, displayY
}

func abs32Cal(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}
