package shell

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/onevra/source/components"
	"github.com/oneo-me/onevra/source/icons/lucide"
	"github.com/oneo-me/onevra/source/shells/sidebar"

	"asterlab-mygo-prototype/internal/theme"
	"asterlab-mygo-prototype/internal/vault"
)

func (s *Shell) ensureRightDrawerDefaults() {
	if s.RightTab < 0 || s.RightTab > 2 {
		s.RightTab = 0
	}
	if s.RightCalc.Stock == "" {
		s.RightCalc = RightCalcState{
			Mode:       0,
			Stock:      "100",
			StockUnit:  "µg/mL",
			Target:     "50",
			TargetUnit: "ng/mL",
			FinalVol:   "4",
			VolUnit:    "mL",
			MolarMass:  "1449.25",
		}
	}
	// Normalize legacy ASCII micro symbols cleanly
	s.RightCalc.StockUnit = normalizeQuickUnitDisplay(s.RightCalc.StockUnit)
	s.RightCalc.TargetUnit = normalizeQuickUnitDisplay(s.RightCalc.TargetUnit)
	s.RightCalc.VolUnit = normalizeQuickVolUnit(s.RightCalc.VolUnit)
}

type drawerContextKind uint8

const (
	drawerCtxDocument drawerContextKind = iota
	drawerCtxCalendar
	drawerCtxPantry
)

func (s *Shell) activeDrawerContext() drawerContextKind {
	tab := s.ActiveTab()
	if tab != nil {
		switch tab.Kind {
		case TabCalendar:
			return drawerCtxCalendar
		case TabPantry:
			return drawerCtxPantry
		case TabFile:
			return drawerCtxDocument
		}
	}
	if s.Page == Tools {
		return drawerCtxPantry
	}
	if s.Page == Project {
		return drawerCtxCalendar
	}
	return drawerCtxDocument
}

func (s *Shell) drawerTabsForContext(ctx drawerContextKind) (shortLabels, fullTitles []string) {
	switch ctx {
	case drawerCtxCalendar:
		return []string{"日程概览", "速算计时"}, []string{"日程概览", "速算与计时"}
	case drawerCtxPantry:
		return []string{"库存概览", "速算计时"}, []string{"试剂库存概览", "速算与计时"}
	default:
		return []string{"大纲", "速算计时", "关联诊断"}, []string{"步骤大纲", "速算与计时", "关联与诊断"}
	}
}

func (s *Shell) rightDrawerHeader(c *ui.Context) {
	tok := theme.GetTokens(c.Theme())
	s.ensureRightDrawerDefaults()
	ctx := s.activeDrawerContext()
	_, titles := s.drawerTabsForContext(ctx)
	idx := s.RightTab
	if idx < 0 || idx >= len(titles) {
		idx = 0
	}
	components.Text(c, titles[idx]).FontSize(float32(tok.TextBody)).Bold().TextColor(tok.Text)
}

func (s *Shell) rightDrawer(c *ui.Context) {
	tok := theme.GetTokens(c.Theme())
	s.ensureRightDrawerDefaults()
	ctx := s.activeDrawerContext()
	shortLabels, ariaLabels := s.drawerTabsForContext(ctx)
	if s.RightTab >= len(shortLabels) {
		s.RightTab = 0
	}

	var topInset float32
	if s.window != nil {
		topInset = sidebar.HeaderHeight(c)
	}

	drawerCol := ui.Column(c).Fill().Padding(topInset, 0, 0, 0)
	if !c.Vibrancy() {
		drawerCol.Background(tok.Surface.Alpha(0.92))
	}
	drawerCol.Children(func() {
		ui.Row(c).FillWidth().Padding(float32(tok.Spacing8), float32(tok.Spacing12)).Children(func() {
			ui.Row(c).FillWidth().Padding(float32(tok.Spacing4)).
				Radius(float32(tok.ControlRadius)).Background(tok.Elevated).
				Gap(float32(tok.Spacing4)).AlignItems(ui.Center).Children(func() {
				for idx, lbl := range shortLabels {
					idx, lbl := idx, lbl
					active := s.RightTab == idx
					btn := ui.ButtonBase(c.Key(fmt.Sprintf("rd-tab-%d-%d", ctx, idx))).
						Grow(1).MinWidth(0).Center().
						Padding(float32(tok.Spacing4), float32(tok.Spacing4)).
						Radius(float32(tok.ControlRadius)).Label(ariaLabels[idx])
					fg := tok.TextMuted
					if active {
						btn.Background(tok.Surface).Border(1, tok.Border.Alpha(0.75))
						fg = tok.Text
					} else if btn.Hovered() {
						btn.Background(tok.Text.Alpha(0.04))
						fg = tok.Text
					}
					btn.Children(func() {
						ui.Text(c, lbl).FontSize(float32(tok.TextData)).Bold().TextColor(fg).SingleLine().Ellipsis("…")
					})
					if btn.Clicked() {
						s.RightTab = idx
					}
				}
			})
		})
		ui.Box(c).FillWidth().Height(1).Background(tok.Border.Alpha(0.5))

		ui.Scroll(c).FillWidth().Grow(1).MinHeight(0).Children(func() {
			ui.Column(c).FillWidth().Padding(float32(tok.Spacing12)).Gap(float32(tok.Spacing8)).Children(func() {
				switch ctx {
				case drawerCtxCalendar:
					if s.RightTab == 0 {
						s.renderDrawerCalendarSummary(c, tok)
					} else {
						s.renderDrawerTimerAndCalc(c, tok)
					}
				case drawerCtxPantry:
					if s.RightTab == 0 {
						s.renderDrawerPantrySummary(c, tok)
					} else {
						s.renderDrawerTimerAndCalc(c, tok)
					}
				default:
					switch s.RightTab {
					case 0:
						s.renderDrawerOutline(c, tok)
					case 1:
						s.renderDrawerTimerAndCalc(c, tok)
					case 2:
						s.renderDrawerLinksAndDiagnostics(c, tok)
					}
				}
			})
		})
	})
}

// UI-14: Calendar-specific context panel inside the right companion drawer.
func (s *Shell) renderDrawerCalendarSummary(c *ui.Context, tok theme.Tokens) {
	selDate := s.ensureSelectedDate()
	todayYMD := s.resolvedToday().Format("2006-01-02")

	var dayExps []ExperimentItem
	var upcoming []ExperimentItem
	var draftCnt, readyCnt, activeCnt, doneCnt int
	for _, exp := range s.Experiments {
		switch exp.Stage {
		case StageReady:
			readyCnt++
		case StageInProgress:
			activeCnt++
		case StageCompleted:
			doneCnt++
		default:
			draftCnt++
		}
		if exp.Date == selDate {
			dayExps = append(dayExps, exp)
		}
		if exp.Date >= todayYMD {
			upcoming = append(upcoming, exp)
		}
	}
	sort.Slice(upcoming, func(i, j int) bool {
		if upcoming[i].Date != upcoming[j].Date {
			return upcoming[i].Date < upcoming[j].Date
		}
		return upcoming[i].Title < upcoming[j].Title
	})

	// Stage overview card
	ui.Column(c).FillWidth().Padding(float32(tok.Spacing12)).Radius(float32(tok.PanelRadius)).
		Background(tok.Elevated).Border(1, tok.Border).Gap(float32(tok.Spacing8)).Children(func() {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
			components.Text(c, "实验排期统计").FontSize(float32(tok.TextData)).Bold().TextColor(tok.Text)
			ui.Spacer(c)
			components.Badge(c, fmt.Sprintf("共 %d 项", len(s.Experiments)))
		})
		ui.Row(c).FillWidth().Gap(float32(tok.Spacing8)).Children(func() {
			renderDrawerStatTile(c, tok, "进行中", fmt.Sprintf("%d", activeCnt), tok.Accent)
			renderDrawerStatTile(c, tok, "待执行", fmt.Sprintf("%d", readyCnt+draftCnt), tok.Warning)
			renderDrawerStatTile(c, tok, "已完成", fmt.Sprintf("%d", doneCnt), tok.Success)
		})
	})

	// Selected date agenda card
	ui.Column(c).FillWidth().Padding(float32(tok.Spacing12)).Radius(float32(tok.PanelRadius)).
		Background(tok.Elevated).Border(1, tok.Border).Gap(float32(tok.Spacing8)).Children(func() {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
			components.Text(c, "选中日期 · "+selDate).FontSize(float32(tok.TextData)).Bold().TextColor(tok.Text).FontFeatures("tnum")
			ui.Spacer(c)
			if s.OnCreateForDate != nil || s.OnCreate != nil {
				if components.Button(c, "+ 新建", components.ButtonGhost).Clicked() {
					s.triggerCreateForDate(selDate)
				}
			}
		})
		if len(dayExps) == 0 {
			components.Text(c, "当天暂无排期实验，可双击日历格或点右上角新建。").
				FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
		} else {
			for _, exp := range dayExps {
				exp := exp
				btn := ui.ButtonBase(c.Key("rd-cal-day-"+exp.ID)).FillWidth().Justify(ui.Start).
					Padding(float32(tok.Spacing4), float32(tok.Spacing8)).
					Radius(float32(tok.ControlRadius)).Background(tok.Surface).Border(1, tok.Border.Alpha(0.75)).Label(exp.Title)
				btn.Children(func() {
					components.Text(c, exp.Title).FontSize(float32(tok.TextData)).TextColor(tok.Text).SingleLine().Ellipsis("…")
				})
				if btn.Clicked() {
					s.openExperiment(exp)
				}
			}
		}
	})

	// Upcoming schedule list
	if len(upcoming) > 0 {
		ui.Column(c).FillWidth().Gap(float32(tok.Spacing4)).Children(func() {
			components.Text(c, "近期排期").FontSize(float32(tok.TextMeta)).Bold().TextColor(tok.TextMuted)
			limit := len(upcoming)
			if limit > 6 {
				limit = 6
			}
			for _, exp := range upcoming[:limit] {
				exp := exp
				row := ui.ButtonBase(c.Key("rd-cal-up-"+exp.ID)).FillWidth().Justify(ui.Start).
					Padding(float32(tok.Spacing4), float32(tok.Spacing8)).
					Radius(float32(tok.ControlRadius)).Label(exp.Title)
				if row.Hovered() {
					row.Background(tok.Text.Alpha(0.04))
				}
				row.Children(func() {
					ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
						ui.Text(c, exp.Date[5:]).FontSize(float32(tok.TextMeta)).TextColor(tok.Accent).FontFeatures("tnum").Shrink(0)
						components.Text(c, exp.Title).FontSize(float32(tok.TextData)).TextColor(tok.Text).
							SingleLine().Ellipsis("…").Grow(1).MinWidth(0)
					})
				})
				if row.Clicked() {
					s.openExperiment(exp)
				}
			}
		})
	}
}

// UI-14: Pantry-specific context panel inside the right companion drawer.
func (s *Shell) renderDrawerPantrySummary(c *ui.Context, tok theme.Tokens) {
	var lowItems []QuickPantryItem
	var expItems []QuickPantryItem
	var okCnt int
	for _, it := range s.PantryItems {
		switch strings.ToLower(strings.TrimSpace(it.Status)) {
		case "exhausted", "low", "empty", "out":
			lowItems = append(lowItems, it)
		case "expired", "near_expiry", "expiring":
			expItems = append(expItems, it)
		default:
			okCnt++
		}
	}

	ui.Column(c).FillWidth().Padding(float32(tok.Spacing12)).Radius(float32(tok.PanelRadius)).
		Background(tok.Elevated).Border(1, tok.Border).Gap(float32(tok.Spacing8)).Children(func() {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
			components.Text(c, "试剂耗材健康度").FontSize(float32(tok.TextData)).Bold().TextColor(tok.Text)
			ui.Spacer(c)
			components.Badge(c, fmt.Sprintf("%d 项", len(s.PantryItems)))
		})
		ui.Row(c).FillWidth().Gap(float32(tok.Spacing8)).Children(func() {
			renderDrawerStatTile(c, tok, "充足", fmt.Sprintf("%d", okCnt), tok.Success)
			renderDrawerStatTile(c, tok, "偏低", fmt.Sprintf("%d", len(lowItems)), tok.Warning)
			renderDrawerStatTile(c, tok, "效期预警", fmt.Sprintf("%d", len(expItems)), tok.Danger)
		})
	})

	if len(lowItems) == 0 && len(expItems) == 0 {
		ui.Row(c).FillWidth().Padding(float32(tok.Spacing12)).Radius(float32(tok.PanelRadius)).
			Background(tok.Elevated).Border(1, tok.Border).AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
			ui.Icon(c, lucide.CircleCheck()).Size(float32(tok.Spacing16), float32(tok.Spacing16)).TextColor(tok.Success)
			components.Text(c, "当前试剂库存充足，暂无低库存或过期预警。").
				FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted).Grow(1).MinWidth(0)
		})
		return
	}

	if len(expItems) > 0 {
		ui.Column(c).FillWidth().Gap(float32(tok.Spacing4)).Children(func() {
			components.Text(c, "效期预警").FontSize(float32(tok.TextMeta)).Bold().TextColor(tok.Danger)
			for i, it := range expItems {
				renderDrawerPantryAlertRow(c, tok, fmt.Sprintf("rd-pexp-%d", i), it, tok.Danger)
			}
		})
	}
	if len(lowItems) > 0 {
		ui.Column(c).FillWidth().Gap(float32(tok.Spacing4)).Children(func() {
			components.Text(c, "低库存补货清单").FontSize(float32(tok.TextMeta)).Bold().TextColor(tok.Warning)
			for i, it := range lowItems {
				renderDrawerPantryAlertRow(c, tok, fmt.Sprintf("rd-plow-%d", i), it, tok.Warning)
			}
		})
	}
}

func renderDrawerStatTile(c *ui.Context, tok theme.Tokens, label, val string, accent ui.Color) {
	ui.Column(c).Grow(1).MinWidth(0).Padding(float32(tok.Spacing8)).
		Radius(float32(tok.ControlRadius)).Background(tok.Surface).Border(1, tok.Border.Alpha(0.75)).
		AlignItems(ui.Center).Gap(2).Children(func() {
		ui.Text(c, val).FontSize(float32(tok.TextSection)).Bold().TextColor(accent).FontFeatures("tnum").SingleLine()
		ui.Text(c, label).FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted).SingleLine()
	})
}

func renderDrawerPantryAlertRow(c *ui.Context, tok theme.Tokens, key string, it QuickPantryItem, tone ui.Color) {
	ui.Row(c.Key(key)).FillWidth().Padding(float32(tok.Spacing8)).Radius(float32(tok.ControlRadius)).
		Background(tok.Elevated).Border(1, tone.Alpha(0.32)).AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
		ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
			components.Text(c, it.Name).FontSize(float32(tok.TextData)).Bold().TextColor(tok.Text).SingleLine().Ellipsis("…")
			meta := firstNonEmptyStr(it.Location, it.LotNumber)
			if it.ExpiresOn != "" {
				if meta != "" {
					meta += " · 效期 " + it.ExpiresOn
				} else {
					meta = "效期 " + it.ExpiresOn
				}
			}
			if meta != "" {
				components.Text(c, meta).FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted).SingleLine().Ellipsis("…")
			}
		})
	})
}

func (s *Shell) renderDrawerOutline(c *ui.Context, tok theme.Tokens) {
	if len(s.OutlineSteps) == 0 {
		components.Text(c, "打开实验文件后，此处显示文档大纲与步骤以便点击跳转。").
			FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
		return
	}
	// Compact segmented filter: [ 全部 | 仅步骤 ]
	hasSteps := false
	hasHeadings := false
	for _, it := range s.OutlineSteps {
		if it.IsStep {
			hasSteps = true
		} else {
			hasHeadings = true
		}
	}
	if hasSteps && hasHeadings {
		ui.Row(c).FillWidth().Padding(0, 0, float32(tok.Spacing8), 0).AlignItems(ui.Center).Gap(float32(tok.Spacing4)).Children(func() {
			for idx, lbl := range []string{"全部", "仅步骤"} {
				idx, lbl := idx, lbl
				active := (idx == 1) == s.OutlineOnlySteps
				pill := ui.ButtonBase(c.Key(fmt.Sprintf("rd-ol-flt-%d", idx))).
					Height(float32(tok.Spacing24)).Padding(0, float32(tok.Spacing8)).
					AlignItems(ui.Center).Radius(float32(tok.ControlRadius)).Label("大纲筛选：" + lbl)
				fg := tok.TextMuted
				if active {
					pill.Background(tok.AccentSoftBg).Border(1, tok.Accent.Alpha(0.32))
					fg = tok.Accent
				} else if pill.Hovered() {
					pill.Background(tok.Text.Alpha(0.04))
					fg = tok.Text
				}
				pill.Children(func() {
					ui.Text(c, lbl).FontSize(float32(tok.TextMeta)).Bold().TextColor(fg).SingleLine()
				})
				if pill.Clicked() {
					s.OutlineOnlySteps = idx == 1
				}
			}
		})
	}

	for _, item := range s.OutlineSteps {
		item := item
		if s.OutlineOnlySteps && !item.IsStep {
			continue
		}
		depth := item.Depth
		if s.OutlineOnlySteps {
			depth = item.StepDepth
		}
		fg := tok.TextMuted
		if item.Active {
			fg = tok.Accent
		} else if !item.IsStep && depth == 0 {
			fg = tok.Text
		} else if item.IsStep && item.StepDepth == 0 {
			fg = tok.Text
		}
		leftPad := float32(tok.Spacing8) + float32(min(depth, 4))*float32(tok.Spacing12)
		btn := ui.ButtonBase(c.Key("rd-step-"+item.ID)).FillWidth().Justify(ui.Start).
			Padding(float32(tok.Spacing4)*1.25, float32(tok.Spacing8), float32(tok.Spacing4)*1.25, leftPad).
			Radius(float32(tok.ControlRadius)).Label("跳至 " + item.Title)
		bg := ui.Transparent
		if item.Active {
			bg = tok.AccentSoftBg
		} else if btn.Hovered() {
			bg = tok.Text.Alpha(0.045)
			fg = tok.Text
		}
		btn.Background(btn.AnimateColor("rd-bg", bg, theme.MotionMicroFade))
		theme.ApplyTactilePress(btn)
		btn.Children(func() {
			ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
				if item.IsStep {
					if item.StepDepth == 0 {
						ui.Row(c).Height(float32(tok.Spacing16)+2).Padding(0, float32(tok.Spacing4)*1.2).
							AlignItems(ui.Center).Radius(float32(tok.ControlRadius)).
							Background(tok.AccentSoftBg).Border(1, tok.Accent.Alpha(0.28)).Shrink(0).Children(func() {
							ui.Text(c, item.Number).FontSize(float32(tok.TextMeta)-1).Bold().TextColor(tok.Accent).FontFeatures("tnum").SingleLine()
						})
					} else {
						ui.Text(c, item.Number).FontSize(float32(tok.TextMeta)).Bold().TextColor(tok.TextMuted).FontFeatures("tnum").Shrink(0)
					}
				} else if item.Active {
					ui.Box(c).Width(2).Height(float32(tok.Spacing12)).Radius(float32(tok.ControlRadius)).Background(tok.Accent)
				}
				titleTxt := components.Text(c, item.Title).FontSize(float32(tok.TextData)).TextColor(fg).
					SingleLine().Ellipsis("…").Grow(1).MinWidth(0)
				if !item.IsStep && depth == 0 {
					titleTxt.Bold()
				}
				if item.Meta != "" {
					ui.Text(c, item.Meta).FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted).FontFeatures("tnum").Shrink(0)
				}
			})
		})
		if btn.Clicked() && s.OnSelectOutlineStep != nil {
			s.OnSelectOutlineStep(item.ID)
		}
	}
}

func (s *Shell) renderDrawerTimerAndCalc(c *ui.Context, tok theme.Tokens) {
	if len(s.VaultTimers) > 0 {
		components.Text(c, "Vault 全局计时器").FontSize(float32(tok.TextBody)).Bold().TextColor(tok.Text)
		now := time.Now()
		for _, vt := range s.VaultTimers {
			vt := vt
			ui.Row(c.Key("rd-vt-"+vt.RelPath+"-"+vt.TimerID)).FillWidth().Padding(float32(tok.Spacing8)).
				Radius(float32(tok.ControlRadius)).Background(tok.AccentSoftBg).Border(1, tok.Accent.Alpha(.25)).
				AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
				ui.Icon(c, lucide.Timer()).Size(float32(tok.Spacing16), float32(tok.Spacing16)).TextColor(tok.Accent)
				ui.Column(c).Grow(1).MinWidth(0).Children(func() {
					components.Text(c, fmt.Sprintf("%s · %s", firstNonEmptyStr(vt.DocTitle, vt.RelPath), vt.Title)).
						FontSize(float32(tok.TextMeta)).Bold().TextColor(tok.Text).SingleLine().Ellipsis("…")
					components.Text(c, formatVaultTimerClock(vt, now)).
						FontSize(float32(tok.TextMeta)).TextColor(tok.Accent).FontFeatures("tnum")
				})
				if s.OnOpenSearchHit != nil && components.Button(c, "打开", components.ButtonGhost).Clicked() {
					s.OnOpenSearchHit(vt.RelPath, "")
				}
			})
		}
		ui.Divider(c)
	}
	if len(s.ActiveTimers) > 0 {
		components.Text(c, "运行中计时器").FontSize(float32(tok.TextBody)).Bold().TextColor(tok.Text)
		for _, tm := range s.ActiveTimers {
			tm := tm
			ui.Row(c.Key("rd-tm-"+tm.TimerID)).FillWidth().Padding(float32(tok.Spacing8)).
				Radius(float32(tok.ControlRadius)).Background(tok.AccentSoftBg).Border(1, tok.Accent.Alpha(.25)).
				AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
				ui.Icon(c, lucide.Timer()).Size(float32(tok.Spacing16), float32(tok.Spacing16)).TextColor(tok.Accent)
				ui.Column(c).Grow(1).MinWidth(0).Children(func() {
					components.Text(c, tm.RunTitle).FontSize(float32(tok.TextMeta)).Bold().TextColor(tok.Text).SingleLine().Ellipsis("…")
					components.Text(c, formatTimerClock(tm)).FontSize(float32(tok.TextMeta)).TextColor(tok.Accent).FontFeatures("tnum")
				})
				if s.OnTimerSelect != nil && components.Button(c, "跳转", components.ButtonGhost).Clicked() {
					s.OnTimerSelect(tm.RunID)
				}
			})
		}
		ui.Divider(c)
	}
	s.renderDrawerQuickCalc(c, tok)
}

func formatVaultTimerClock(vt vault.VaultTimerEntry, now time.Time) string {
	elapsed := vt.ElapsedSec
	if vt.Status == "running" && vt.StartedAt != "" {
		if started, err := time.Parse(time.RFC3339, vt.StartedAt); err == nil {
			if delta := int(now.Sub(started).Seconds()); delta > elapsed {
				elapsed = delta
			}
		}
	}
	if elapsed < 0 {
		elapsed = 0
	}
	formatSec := func(sec int) string {
		h := sec / 3600
		m := (sec % 3600) / 60
		s := sec % 60
		if h > 0 {
			return fmt.Sprintf("%02d:%02d:%02d", h, m, s)
		}
		return fmt.Sprintf("%02d:%02d", m, s)
	}
	stateLbl := "暂停"
	if vt.Status == "running" {
		stateLbl = "进行中"
	}
	if vt.Mode == "countdown" && vt.PlannedSec > 0 {
		rem := vt.PlannedSec - elapsed
		if rem < 0 {
			rem = 0
		}
		return fmt.Sprintf("%s · 剩余 %s", stateLbl, formatSec(rem))
	}
	return fmt.Sprintf("%s · 已用 %s", stateLbl, formatSec(elapsed))
}

func (s *Shell) renderDrawerLinksAndDiagnostics(c *ui.Context, tok theme.Tokens) {
	ui.Column(c).FillWidth().Gap(float32(tok.Spacing8)).Children(func() {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(float32(tok.Spacing4)).Children(func() {
			components.Text(c, "反向链接与复现谱系").FontSize(float32(tok.TextBody)).Bold().TextColor(tok.Text)
			ui.Spacer(c)
			components.Badge(c, fmt.Sprintf("%d", len(s.CurrentBacklinks)))
		})
		if len(s.CurrentBacklinks) == 0 {
			components.Text(c, "暂无其他实验通过 [[双链]] 或 repeat_of 引用本文档。").
				FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
		} else {
			for i, ref := range s.CurrentBacklinks {
				ref := ref
				kindLbl := "双链引用"
				if ref.Kind == "repeat_of" {
					kindLbl = "复现引用"
				}
				row := ui.Row(c.Key(fmt.Sprintf("rd-link-%d-%s", i, ref.SourceRel))).FillWidth().
					Padding(float32(tok.Spacing8)).Radius(float32(tok.ControlRadius)).
					Background(tok.Elevated).AlignItems(ui.Center).Gap(float32(tok.Spacing8))
				theme.ApplyTactilePress(row)
				row.Children(func() {
					ui.Icon(c, lucide.Link()).Size(float32(tok.Spacing12), float32(tok.Spacing12)).TextColor(tok.Accent)
					ui.Column(c).Grow(1).MinWidth(0).Children(func() {
						components.Text(c, firstNonEmptyStr(ref.SourceTitle, ref.SourceRel)).
							FontSize(float32(tok.TextData)).Bold().TextColor(tok.Text).SingleLine().Ellipsis("…")
						sub := kindLbl
						if ref.SourceDate != "" {
							sub += " · " + ref.SourceDate
						}
						components.Text(c, sub).FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted).SingleLine()
					})
				})
				if row.Clicked() && s.OnOpenSearchHit != nil {
					s.OnOpenSearchHit(ref.SourceRel, "")
				}
			}
		}

		ui.Divider(c)

		ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(float32(tok.Spacing4)).Children(func() {
			components.Text(c, "实验质检与对照检查").FontSize(float32(tok.TextBody)).Bold().TextColor(tok.Text)
			ui.Spacer(c)
			components.Badge(c, fmt.Sprintf("%d", len(s.CurrentDiagnostics)))
		})
		if len(s.CurrentDiagnostics) == 0 {
			ui.Row(c).FillWidth().Padding(float32(tok.Spacing8)).Radius(float32(tok.ControlRadius)).
				Background(tok.Elevated).AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
				ui.Icon(c, lucide.CircleCheck()).Size(float32(tok.Spacing16), float32(tok.Spacing16)).TextColor(tok.Success)
				components.Text(c, "结构完整，对照与样本量均已就绪。").FontSize(float32(tok.TextMeta)).TextColor(tok.Success)
			})
			return
		}
		for i, diag := range s.CurrentDiagnostics {
			diag := diag
			sevLbl := "建议"
			sevCol := tok.Accent
			switch diag.Severity {
			case "blocking":
				sevLbl = "阻断"
				sevCol = tok.Danger
			case "needs_confirmation":
				sevLbl = "待确认"
				sevCol = tok.Warning
			}
			ui.Column(c.Key(fmt.Sprintf("rd-diag-%d-%s", i, diag.Code))).FillWidth().
				Padding(float32(tok.Spacing8)).Radius(float32(tok.ControlRadius)).
				Background(tok.Elevated).Border(1, sevCol.Alpha(0.32)).Gap(float32(tok.Spacing4)).Children(func() {
				ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(float32(tok.Spacing4)).Children(func() {
					ui.Text(c, sevLbl).FontSize(float32(tok.TextMeta)).Bold().TextColor(sevCol)
				})
				components.Text(c, diag.Message).FontSize(float32(tok.TextData)).TextColor(tok.Text)
				if diag.NextAction != "" {
					components.Text(c, "建议："+diag.NextAction).FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
				}
			})
		}
	})
}

// Complete wet-lab concentration & volume unit lists (UI-07 & UI-08).
var (
	quickMolarUnits = []string{"M", "mM", "µM", "nM", "pM", "fM"}
	quickMassUnits  = []string{"g/mL", "mg/mL", "µg/mL", "ng/mL", "pg/mL", "g/L", "mg/L", "µg/L"}
	quickFoldUnits  = []string{"X"}
	quickPctUnits   = []string{"%"}
	quickActUnits   = []string{"U/mL", "mU/mL"}
	quickCellUnits  = []string{"CFU/mL", "cells/mL"}
	quickVolUnits   = []string{"L", "mL", "µL", "nL"}

	quickAllStockUnits = []string{
		"M", "mM", "µM", "nM", "pM",
		"g/mL", "mg/mL", "µg/mL", "ng/mL", "pg/mL", "g/L", "mg/L", "µg/L",
		"%", "X", "U/mL", "mU/mL", "CFU/mL", "cells/mL",
	}
)

func normalizeQuickUnitDisplay(u string) string {
	switch strings.TrimSpace(u) {
	case "uM":
		return "µM"
	case "ug/mL":
		return "µg/mL"
	case "ug/L":
		return "µg/L"
	case "uL":
		return "µL"
	case "":
		return "µg/mL"
	default:
		return strings.TrimSpace(u)
	}
}

func normalizeQuickVolUnit(u string) string {
	switch strings.TrimSpace(u) {
	case "uL", "µL":
		return "µL"
	case "L", "mL", "nL":
		return strings.TrimSpace(u)
	default:
		return "mL"
	}
}

func compatibleTargetUnitsFor(stockUnit string) []string {
	fam, _ := quickConcUnitBase(stockUnit)
	switch fam {
	case "molar":
		return quickMolarUnits
	case "mass":
		return quickMassUnits
	case "fold":
		return quickFoldUnits
	case "percent":
		return quickPctUnits
	case "activity":
		return quickActUnits
	case "count":
		return quickCellUnits
	default:
		return quickMassUnits
	}
}

func defaultCompatibleTargetUnit(stockUnit string) string {
	compat := compatibleTargetUnitsFor(stockUnit)
	for i, u := range compat {
		if u == stockUnit && i+1 < len(compat) {
			return compat[i+1]
		}
	}
	return compat[0]
}

// UI-07, UI-08, UI-09: Cohesive editorial card calculator with complete wet-lab units
// and strict compatible-unit filtering (no unusable options ever shown).
func (s *Shell) renderDrawerQuickCalc(c *ui.Context, tok theme.Tokens) {
	// Segmented mode switch
	ui.Row(c).FillWidth().Padding(float32(tok.Spacing4)).
		Radius(float32(tok.ControlRadius)).Background(tok.Elevated).
		Gap(float32(tok.Spacing4)).AlignItems(ui.Center).Children(func() {
		for idx, lbl := range []string{"C₁V₁ 稀释", "摩尔称量"} {
			idx, lbl := idx, lbl
			active := s.RightCalc.Mode == idx
			btn := ui.ButtonBase(c.Key(fmt.Sprintf("rd-calc-%d", idx))).
				Grow(1).MinWidth(0).Center().
				Padding(float32(tok.Spacing4), float32(tok.Spacing4)).
				Radius(float32(tok.ControlRadius)).Label(lbl)
			fg := tok.TextMuted
			if active {
				btn.Background(tok.Surface).Border(1, tok.Border.Alpha(0.75))
				fg = tok.Text
			} else if btn.Hovered() {
				btn.Background(tok.Text.Alpha(0.04))
				fg = tok.Text
			}
			btn.Children(func() {
				ui.Text(c, lbl).FontSize(float32(tok.TextData)).Bold().TextColor(fg).SingleLine().Ellipsis("…")
			})
			if btn.Clicked() {
				s.RightCalc.Mode = idx
			}
		}
	})

	if s.RightCalc.Mode == 0 {
		// UI-08: Auto-coerce TargetUnit so it always matches StockUnit's dimension family.
		compatTargets := compatibleTargetUnitsFor(s.RightCalc.StockUnit)
		fam1, _ := quickConcUnitBase(s.RightCalc.StockUnit)
		fam2, _ := quickConcUnitBase(s.RightCalc.TargetUnit)
		if fam1 == "" {
			s.RightCalc.StockUnit = "µg/mL"
			compatTargets = quickMassUnits
			fam1 = "mass"
		}
		if fam2 != fam1 {
			s.RightCalc.TargetUnit = defaultCompatibleTargetUnit(s.RightCalc.StockUnit)
		}

		ui.Column(c).FillWidth().Padding(float32(tok.Spacing12)).Radius(float32(tok.PanelRadius)).
			Background(tok.Elevated).Border(1, tok.Border).Gap(float32(tok.Spacing12)).Children(func() {
			drawerLabeledInput(c, tok, "母液浓度 (C₁)", &s.RightCalc.Stock, &s.RightCalc.StockUnit, quickAllStockUnits)
			// Re-check compatibility immediately if user just changed StockUnit
			if f1, _ := quickConcUnitBase(s.RightCalc.StockUnit); f1 != "" {
				compatTargets = compatibleTargetUnitsFor(s.RightCalc.StockUnit)
				if f2, _ := quickConcUnitBase(s.RightCalc.TargetUnit); f2 != f1 {
					s.RightCalc.TargetUnit = defaultCompatibleTargetUnit(s.RightCalc.StockUnit)
				}
			}
			drawerLabeledInput(c, tok, "目标浓度 (C₂)", &s.RightCalc.Target, &s.RightCalc.TargetUnit, compatTargets)
			drawerLabeledInput(c, tok, "配置总体积 (V₂)", &s.RightCalc.FinalVol, &s.RightCalc.VolUnit, quickVolUnits)

			ui.Box(c).FillWidth().Height(1).Background(tok.Border.Alpha(0.65))

			v1UL, diluentUL, msg := computeQuickDilution(s.RightCalc)
			ui.Column(c).FillWidth().Padding(float32(tok.Spacing12)).Radius(float32(tok.ControlRadius)).
				Background(tok.Surface).Border(1, tok.Accent.Alpha(0.28)).Gap(float32(tok.Spacing4)).Children(func() {
				if msg != "" {
					components.Text(c, msg).FontSize(float32(tok.TextMeta)).TextColor(tok.Danger)
					return
				}
				ui.Row(c).FillWidth().AlignItems(ui.Baseline).Children(func() {
					ui.Text(c, "取母液 V₁").FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
					ui.Spacer(c)
					ui.Text(c, formatQuickVolUL(v1UL)).FontSize(float32(tok.TextSection)).Bold().
						TextColor(tok.Accent).FontFeatures("tnum").ContentTransition(theme.NumericRoll(false))
				})
				ui.Row(c).FillWidth().AlignItems(ui.Baseline).Children(func() {
					ui.Text(c, "补加稀释液").FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
					ui.Spacer(c)
					ui.Text(c, formatQuickVolUL(diluentUL)).FontSize(float32(tok.TextData)).Bold().
						TextColor(tok.Text).FontFeatures("tnum").ContentTransition(theme.NumericRoll(false))
				})
			})
		})
		return
	}

	// Molar weighing mode: auto-coerce non-molar unit so incompatible units never appear (UI-08).
	if fam, _ := quickConcUnitBase(s.RightCalc.TargetUnit); fam != "molar" {
		s.RightCalc.TargetUnit = "mM"
	}
	ui.Column(c).FillWidth().Padding(float32(tok.Spacing12)).Radius(float32(tok.PanelRadius)).
		Background(tok.Elevated).Border(1, tok.Border).Gap(float32(tok.Spacing12)).Children(func() {
		drawerLabeledInput(c, tok, "目标摩尔浓度", &s.RightCalc.Target, &s.RightCalc.TargetUnit, quickMolarUnits)
		drawerLabeledInput(c, tok, "配置总体积", &s.RightCalc.FinalVol, &s.RightCalc.VolUnit, quickVolUnits)
		ui.Column(c).FillWidth().Gap(float32(tok.Spacing4)).Children(func() {
			ui.Text(c, "分子量 MW (g/mol)").FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
			components.TextBox(c, &s.RightCalc.MolarMass, components.TextBoxOptions{}).Placeholder("例如 1449.25")
		})

		ui.Box(c).FillWidth().Height(1).Background(tok.Border.Alpha(0.65))

		massMg, msg := computeQuickMolarMass(s.RightCalc)
		ui.Column(c).FillWidth().Padding(float32(tok.Spacing12)).Radius(float32(tok.ControlRadius)).
			Background(tok.Surface).Border(1, tok.Accent.Alpha(0.28)).Gap(float32(tok.Spacing4)).Children(func() {
			if msg != "" {
				components.Text(c, msg).FontSize(float32(tok.TextMeta)).TextColor(tok.Danger)
				return
			}
			ui.Row(c).FillWidth().AlignItems(ui.Baseline).Children(func() {
				ui.Text(c, "需称量溶质").FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
				ui.Spacer(c)
				ui.Text(c, formatQuickMassMg(massMg)).FontSize(float32(tok.TextSection)).Bold().
					TextColor(tok.Accent).FontFeatures("tnum").ContentTransition(theme.NumericRoll(false))
			})
		})
	})
}

func drawerLabeledInput(c *ui.Context, tok theme.Tokens, label string, val *string, unit *string, units []string) {
	ui.Column(c).FillWidth().Gap(float32(tok.Spacing4)).Children(func() {
		ui.Text(c, label).FontSize(float32(tok.TextMeta)).Bold().TextColor(tok.TextMuted)
		ui.Row(c).FillWidth().Gap(float32(tok.Spacing8)).AlignItems(ui.Center).Children(func() {
			ui.Box(c).Grow(1).MinWidth(0).Children(func() {
				components.TextBox(c, val, components.TextBoxOptions{}).FontFeatures("tnum")
			})
			if unit != nil && len(units) > 0 {
				ui.Box(c).Width(float32(tok.Spacing48) * 1.9).Shrink(0).Children(func() {
					sel := components.Select(c, unit)
					sel.Trigger.FillWidth().Children(func() { components.Text(c, *unit).FontSize(float32(tok.TextMeta)).SingleLine() })
					sel.Popup(func(panel ui.Element) {
						for _, opt := range units {
							opt := opt
							item := sel.Item(opt)
							item.Children(func() { components.Text(c, opt).FontSize(float32(tok.TextMeta)).SingleLine() })
						}
					})
				})
			}
		})
	})
}

func formatQuickVolUL(ul float64) string {
	switch {
	case ul >= 1e6:
		return fmt.Sprintf("%.3f L", ul/1e6)
	case ul >= 1000:
		return fmt.Sprintf("%.2f mL", ul/1000)
	case ul > 0 && ul < 1:
		return fmt.Sprintf("%.1f nL", ul*1000)
	default:
		return fmt.Sprintf("%.2f µL", ul)
	}
}

func formatQuickMassMg(mg float64) string {
	switch {
	case mg >= 1000:
		return fmt.Sprintf("%.3f g", mg/1000)
	case mg > 0 && mg < 1:
		return fmt.Sprintf("%.2f µg", mg*1000)
	default:
		return fmt.Sprintf("%.3f mg", mg)
	}
}

func quickConcUnitBase(unit string) (family string, factor float64) {
	switch strings.TrimSpace(unit) {
	case "M":
		return "molar", 1
	case "mM":
		return "molar", 1e-3
	case "uM", "µM":
		return "molar", 1e-6
	case "nM":
		return "molar", 1e-9
	case "pM":
		return "molar", 1e-12
	case "fM":
		return "molar", 1e-15
	case "g/mL":
		return "mass", 1e3
	case "mg/mL", "g/L":
		return "mass", 1
	case "ug/mL", "µg/mL", "mg/L":
		return "mass", 1e-3
	case "ng/mL", "ug/L", "µg/L":
		return "mass", 1e-6
	case "pg/mL":
		return "mass", 1e-9
	case "X", "x":
		return "fold", 1
	case "%":
		return "percent", 1
	case "U/mL":
		return "activity", 1
	case "mU/mL":
		return "activity", 1e-3
	case "CFU/mL", "cells/mL":
		return "count", 1
	default:
		return "", 0
	}
}

func quickVolToUL(val float64, unit string) float64 {
	switch strings.TrimSpace(unit) {
	case "L":
		return val * 1e6
	case "mL", "":
		return val * 1e3
	case "uL", "µL":
		return val
	case "nL":
		return val * 1e-3
	default:
		return val * 1e3
	}
}

func computeQuickDilution(st RightCalcState) (v1UL, dilUL float64, errMsg string) {
	c1, err1 := strconv.ParseFloat(strings.TrimSpace(st.Stock), 64)
	c2, err2 := strconv.ParseFloat(strings.TrimSpace(st.Target), 64)
	v2, err3 := strconv.ParseFloat(strings.TrimSpace(st.FinalVol), 64)
	if err1 != nil || err2 != nil || err3 != nil || c1 <= 0 || c2 <= 0 || v2 <= 0 {
		return 0, 0, "请输入大于 0 的有效数值"
	}
	fam1, f1 := quickConcUnitBase(st.StockUnit)
	fam2, f2 := quickConcUnitBase(st.TargetUnit)
	if fam1 == "" || fam1 != fam2 {
		return 0, 0, "C₁ 与 C₂ 浓度单位量纲不一致"
	}
	v2UL := quickVolToUL(v2, st.VolUnit)
	v1UL = v2UL * (c2 * f2) / (c1 * f1)
	if v1UL > v2UL || math.IsNaN(v1UL) || math.IsInf(v1UL, 0) {
		return 0, 0, "母液浓度低于目标浓度，无法稀释"
	}
	return v1UL, v2UL - v1UL, ""
}

func computeQuickMolarMass(st RightCalcState) (massMg float64, errMsg string) {
	c2, err1 := strconv.ParseFloat(strings.TrimSpace(st.Target), 64)
	v2, err2 := strconv.ParseFloat(strings.TrimSpace(st.FinalVol), 64)
	mw, err3 := strconv.ParseFloat(strings.TrimSpace(st.MolarMass), 64)
	if err1 != nil || err2 != nil || err3 != nil || c2 <= 0 || v2 <= 0 || mw <= 0 {
		return 0, "请输入有效浓度、体积和分子量"
	}
	fam, f := quickConcUnitBase(st.TargetUnit)
	if fam != "molar" {
		return 0, "称量模式请选择摩尔浓度单位 (M / mM / µM / nM)"
	}
	volL := quickVolToUL(v2, st.VolUnit) * 1e-6
	grams := (c2 * f) * volL * mw
	return grams * 1000.0, ""
}
