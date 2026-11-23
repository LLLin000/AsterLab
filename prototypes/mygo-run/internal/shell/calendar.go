package shell

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/onevra/source/components"
	"github.com/oneo-me/onevra/source/icons/lucide"

	"asterlab-mygo-prototype/internal/theme"
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

	allExps := s.resolvedExperiments()
	query := strings.ToLower(strings.TrimSpace(s.Query))
	byDate := map[string][]ExperimentItem{}
	for _, exp := range allExps {
		if query != "" && !strings.Contains(strings.ToLower(exp.Title), query) && !strings.Contains(strings.ToLower(exp.Summary), query) {
			continue
		}
		if strings.TrimSpace(exp.Date) == "" {
			exp.Date = todayYMD
		}
		byDate[exp.Date] = append(byDate[exp.Date], exp)
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
			s.calendarHomeBody(c, tok, cursor, today, todayYMD, selDateYMD, compactV, mobileMode, allExps, byDate)
		})
		return
	}
	s.calendarHomeBody(c, tok, cursor, today, todayYMD, selDateYMD, compactV, mobileMode, allExps, byDate)
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
		padTop = s.MobileTopReserve(c) + float32(tok.Spacing8)
		padH = float32(tok.Spacing8)
		padB = s.MobileBottomReserve(c) + float32(tok.Spacing16)
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
				components.Text(c, "双击任意日期格新建实验，或从左侧文件树创建项目文件夹分类整理 .asterml 文件。").
					FontSize(float32(tok.TextData)).TextColor(tok.TextMuted).Grow(1).MinWidth(0)
			})
		}
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
				if !mobileMode {
					ui.TextInput(c, &s.Query).Placeholder("搜索实验…").Width(float32(tok.Spacing48) * 3)
				}
				if s.OnCreateForDate != nil || s.OnCreate != nil {
					addLbl := "＋ 加入新实验"
					if mobileMode {
						addLbl = "＋ 新建"
					}
					if components.Button(c, addLbl, components.ButtonPrimary).Clicked() {
						s.triggerCreateForDate(selDateYMD)
					}
				}
				if s.OnOpenTemplatePicker != nil || s.OnImport != nil {
					moreBtn := components.IconButton(c, lucide.Ellipsis(), "更多创建方式", components.IconButtonCompact)
					components.MenuButton(c, moreBtn, func(m *components.Menu) {
						if s.OnOpenTemplatePicker != nil && m.Item("从模板创建实验…").Chosen() {
							s.OnOpenTemplatePicker(selDateYMD)
						}
						if s.OnImport != nil && m.Item("导入 .asterml 文件…").Chosen() {
							s.OnImport()
						}
					})
				}
			})
		})

		// 2. Apple Seamless Surface Grid
		if s.CalendarMode == 0 {
			s.renderAppleMonthGrid(c, tok, cursor, todayYMD, selDateYMD, compactV, byDate)
			if mobileMode {
				s.renderMobileDayAgenda(c, tok, selDateYMD, byDate[selDateYMD])
			}
		} else {
			s.renderAppleWeekTimeGrid(c, tok, cursor, todayYMD, selDateYMD, byDate)
		}
	})
}

func (s *Shell) renderMobileDayAgenda(c *ui.Context, tok theme.Tokens, ymd string, dayExps []ExperimentItem) {
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
		if len(dayExps) == 0 {
			components.Text(c, "当天暂无排期实验，点击右上角「+ 新建」安排实验。").
				FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
			return
		}
		for _, exp := range dayExps {
			exp := exp
			row := ui.ButtonBase(c.Key("mb-ag-"+exp.ID)).FillWidth().
				Padding(float32(tok.Spacing12), float32(tok.Spacing16)).
				Radius(float32(tok.PanelRadius)).Background(tok.Elevated).
				Justify(ui.Start).Label(exp.Title)
			row.Children(func() {
				ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
					_, accentCol, _ := stagePillColors(tok, exp.Stage)
					ui.Box(c).Width(float32(tok.Spacing4)*0.75).Height(float32(tok.Spacing16)).Radius(float32(tok.ControlRadius)).Background(accentCol)
					ui.Column(c).Grow(1).MinWidth(0).Gap(float32(tok.Spacing4)*0.5).Children(func() {
						components.Text(c, exp.Title).FontSize(float32(tok.TextBody)).Bold().TextColor(tok.Text).SingleLine().Ellipsis("…")
						if exp.Folder != "" {
							components.Text(c, exp.Folder+" · "+StageLabel(exp.Stage)).FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted).SingleLine().Ellipsis("…")
						}
					})
					ui.Icon(c, lucide.ChevronRight()).Size(float32(tok.Spacing16), float32(tok.Spacing16)).TextColor(tok.TextMuted)
				})
			})
			if row.Clicked() {
				s.openExperiment(exp)
			}
		}
	})
}

func (s *Shell) renderAppleMonthGrid(c *ui.Context, tok theme.Tokens, cursor time.Time, todayYMD, selDateYMD string, compactV bool, byDate map[string][]ExperimentItem) {
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
									if len(dayExps) > 0 {
										ui.Row(c).FillWidth().Justify(ui.Center).Wrap().Gap(float32(tok.Spacing4)*0.75).Children(func() {
											for i, exp := range dayExps {
												if i >= 4 {
													break
												}
												_, accentCol, _ := stagePillColors(tok, exp.Stage)
												ui.Box(c).Size(float32(tok.Spacing4)*1.5, float32(tok.Spacing4)*1.5).
													Radius(float32(tok.ControlRadius)).Background(accentCol)
											}
										})
									}
								} else {
									for i, exp := range dayExps {
										if i >= maxShow {
											ui.Text(c, fmt.Sprintf("还有 +%d 项…", len(dayExps)-maxShow)).
												FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted).
												Padding(0, float32(tok.Spacing4))
											break
										}
										s.renderAppleEventPill(c, tok, exp, false)
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
								s.renderDayPopover(c, panel, tok, ymd, dayExps)
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
						components.ContextMenu(c, btn, func(m *components.Menu) {
							if m.Item("在此日新建实验").Chosen() {
								s.SelectedDate = ymd
								s.triggerCreateForDate(ymd)
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
		components.ContextMenu(c, pill, func(m *components.Menu) {
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
				m.Group("移动至项目", func(sub *components.Menu) {
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

func (s *Shell) renderAppleWeekTimeGrid(c *ui.Context, tok theme.Tokens, cursor time.Time, todayYMD, selDateYMD string, byDate map[string][]ExperimentItem) {
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
					s.DayPopoverDate = ymd
					s.DayPopoverOpen = true
				}
				if ymd == s.DayPopoverDate {
					components.Popover(c, hdrBtn, &s.DayPopoverOpen, func(panel ui.Element) {
						s.renderDayPopover(c, panel, tok, ymd, dayExps)
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
			axisBox := ui.Box(c.Key("wk-axis-canvas")).FillWidth().HandleInput(func(ev ui.InputEvent) bool {
				if ev.Kind == ui.InputPointerMove {
					if ev.Y >= 0 && ev.Y <= totalGridH {
						s.weekHoverOn = true
						s.weekHoverY = ev.Y
					} else {
						s.weekHoverOn = false
					}
				}
				return false
			})
			if !axisBox.Hovered() {
				s.weekHoverOn = false
			}
			axisBox.Children(func() {
				ui.Column(c).FillWidth().Gap(0).Children(func() {
					for hr := startHr; hr <= endHr; hr++ {
						hr := hr
						if hr > startHr {
							ui.Box(c).FillWidth().Height(1).Background(tok.Border)
						}
						ui.Row(c.Key(fmt.Sprintf("wk-row-%d", hr))).FillWidth().Height(rowH).Gap(0).Children(func() {
							// Hour gutter
							ui.Row(c).Width(gutterW).FillHeight().Shrink(0).Padding(float32(tok.Spacing4), float32(tok.Spacing8)).
								AlignItems(ui.Start).Justify(ui.End).Children(func() {
								ui.Text(c, fmt.Sprintf("%02d:00", hr)).FontSize(float32(tok.TextMeta)).
									TextColor(tok.TextMuted).FontFeatures("tnum")
							})
							for dIdx, day := range days {
								dIdx, day := dIdx, day
								ymd := day.Format("2006-01-02")
								isToday := ymd == todayYMD
								ui.Box(c).FillHeight().Width(1).Background(tok.Border)
								cellBg := tok.Surface
								if isWeekend(day.Weekday()) {
									cellBg = tok.Elevated
								}
								slot := ui.Box(c.Key(fmt.Sprintf("wk-slot-%s-%d", ymd, hr))).Grow(1).MinWidth(0).FillHeight().
									Background(cellBg).Padding(float32(tok.Spacing4))
								if slot.Hovered() {
									s.weekHoverDay = dIdx
								}
								slot.Children(func() {
									for _, exp := range byDate[ymd] {
										exp := exp
										if exp.StartHour > 0 && int(exp.StartHour) == hr {
											bg, bar, fg := stagePillColors(tok, exp.Stage)
											blk := ui.ButtonBase(c.Key("wk-hr-"+exp.ID)).FillWidth().Justify(ui.Start).
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
									}
									if isToday && int(curHour) == hr {
										frac := curHour - float32(hr)
										if frac < 0.08 {
											frac = 0.08
										}
										if frac > 0.9 {
											frac = 0.9
										}
										ui.Row(c).Absolute().Left(0).Right(0).Top(rowH*frac).AlignItems(ui.Center).Children(func() {
											dotSize := float32(tok.Spacing8)
											ui.Box(c).Size(dotSize, dotSize).Radius(dotSize * 0.5).Background(tok.Danger)
											ui.Box(c).Grow(1).Height(1.5).Background(tok.Danger)
										})
									}
								})
								if slot.DoubleClicked() {
									s.SelectedDate = ymd
									s.triggerCreateForDate(ymd)
								}
							}
						})
					}
					// Crisp closing bottom hairline so the week grid never looks abruptly cut off
					ui.Box(c).FillWidth().Height(1).Background(tok.Border.Alpha(0.6))
				})
			})
			// UI-09: Full-width click-through hover timeline spanning from left time gutter across all 7 columns,
			// with gentle magnetic snap to 30-minute ticks (:00 and :30).
			axisBox.DrawOver(func(p *ui.Painter, r ui.Rect) {
				if !s.weekHoverOn || s.weekHoverY < 0 || s.weekHoverY > totalGridH {
					return
				}
				halfH := rowH * 0.5
				nearestStep := float32(math.Round(float64(s.weekHoverY / halfH)))
				snapY := nearestStep * halfH
				lineY := s.weekHoverY
				snapped := false
				if abs32Cal(s.weekHoverY-snapY) <= 7 {
					lineY = snapY
					snapped = true
				}
				totalMins := int(math.Round(float64(lineY/rowH)*60.0)) + startHr*60
				if snapped {
					totalMins = int(nearestStep)*30 + startHr*60
				}
				if totalMins < startHr*60 {
					totalMins = startHr * 60
				}
				if totalMins > (endHr+1)*60 {
					totalMins = (endHr + 1) * 60
				}
				timeLbl := fmt.Sprintf("%02d:%02d", totalMins/60, totalMins%60)
				yAbs := r.Y + lineY
				// Continuous horizontal line across all 7 weekday columns
				lineRect := ui.Rect{X: r.X + gutterW, Y: yAbs - 0.75, W: r.W - gutterW, H: 1.5}
				p.Fill(lineRect, tok.Accent.Alpha(0.75), 0)
				// Anchor dot at gutter boundary (slightly larger when magnetically snapped to :00/:30)
				dotR := float32(3)
				if snapped {
					dotR = 4
				}
				p.Fill(ui.Rect{X: r.X + gutterW - dotR, Y: yAbs - dotR, W: dotR * 2, H: dotR * 2}, tok.Accent, dotR)
				// Accent HH:MM pill in the left time gutter
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

func (s *Shell) renderDayPopover(c *ui.Context, panel ui.Element, tok theme.Tokens, ymd string, exps []ExperimentItem) {
	dateLabel := ymd
	if t, err := time.Parse("2006-01-02", ymd); err == nil {
		dateLabel = fmt.Sprintf("%d 月 %d 日（周%s）", t.Month(), t.Day(), weekdayCNShort(t.Weekday()))
	}

	panel.Width(float32(tok.Spacing48)*8.6).MaxHeight(float32(tok.Spacing48)*8).
		Label(dateLabel+" 实验安排").Children(func() {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
			ui.Icon(c, lucide.CalendarDays()).Size(float32(tok.Spacing16), float32(tok.Spacing16)).TextColor(tok.Accent)
			components.Text(c, dateLabel+" · 实验安排").FontSize(float32(tok.TextBody)).Bold().TextColor(tok.Text).
				SingleLine().Ellipsis("…").Grow(1).MinWidth(0)
			components.Badge(c, fmt.Sprintf("%d 项", len(exps)), components.BadgeOptions{Tone: components.BadgeNeutral})
			if s.OnCreateForDate != nil || s.OnCreate != nil {
				if components.Button(c, "＋ 新建", components.ButtonSecondary).Clicked() {
					s.triggerCreateForDate(ymd)
				}
			}
		})

		if len(exps) == 0 {
			ui.Column(c).FillWidth().Padding(float32(tok.Spacing12)).Radius(float32(tok.ControlRadius)).
				Background(tok.Elevated).Border(1, tok.Border).Gap(float32(tok.Spacing8)).Children(func() {
				components.Text(c, "该日期暂无已安排实验。").FontSize(float32(tok.TextBody)).TextColor(tok.TextMuted)
				ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
					if s.OnCreateForDate != nil || s.OnCreate != nil {
						if components.Button(c, "＋ 在此日新建实验", components.ButtonPrimary).Clicked() {
							s.triggerCreateForDate(ymd)
						}
					}
					ui.Text(c, "双击日期格也可新建").FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
				})
			})
			return
		}

		ui.Scroll(c).FillWidth().MaxHeight(float32(tok.Spacing48) * 6).Children(func() {
			ui.Column(c).FillWidth().Gap(float32(tok.Spacing8)).Children(func() {
				for _, exp := range exps {
					exp := exp
					s.renderAgendaExperimentRow(c, tok, exp)
				}
			})
		})
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

func abs32Cal(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}
