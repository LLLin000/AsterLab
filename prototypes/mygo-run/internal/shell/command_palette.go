package shell

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/onevra/source/components"
	"github.com/oneo-me/onevra/source/icons/lucide"

	"asterlab-mygo-prototype/internal/theme"
)

type paletteEntryKind uint8

const (
	paletteExperiment paletteEntryKind = iota
	palettePantry
	paletteAction
)

type paletteEntry struct {
	kind     paletteEntryKind
	key      string
	title    string
	subtitle string
	shortcut string
	pantryID string
	exp      *ExperimentItem
	runAct   func()
}

func (s *Shell) renderCommandPalette(c *ui.Context) {
	if !s.PaletteOpen {
		ui.Box(c.Key("command-palette-dlg")).Children(func() {
			components.Dialog(c, &s.PaletteOpen, components.DialogOptions{Label: "命令面板"}, nil)
		})
		return
	}
	tok := theme.GetTokens(c.Theme())
	open := s.PaletteOpen
	entries := s.buildPaletteEntries()
	if len(entries) == 0 {
		s.PaletteCursor = 0
	} else {
		if s.PaletteCursor < 0 {
			s.PaletteCursor = 0
		}
		if s.PaletteCursor >= len(entries) {
			s.PaletteCursor = len(entries) - 1
		}
	}

	ui.Box(c.Key("command-palette-dlg")).Children(func() {
		components.Dialog(c, &open, components.DialogOptions{Label: "命令面板"}, func(panel ui.Element) {
			panel.Width(float32(tok.Spacing48)*11.5).Padding(float32(tok.Spacing16)).Gap(float32(tok.Spacing8)).
				MaxHeight(float32(tok.Spacing48) * 11.5)
			if tok.Dark {
				panel.Background(tok.Surface).Border(1, tok.BorderStrong)
			} else if tok.LiquidGlass {
				theme.ApplyPanelGlass(panel, tok, 0.36)
			}
		panel.Children(func() {
			if len(entries) > 0 {
				if panel.Shortcut(0, ui.KeyDown) {
					s.PaletteCursor = (s.PaletteCursor + 1) % len(entries)
				}
				if panel.Shortcut(0, ui.KeyUp) {
					s.PaletteCursor = (s.PaletteCursor - 1 + len(entries)) % len(entries)
				}
				if panel.Shortcut(0, ui.KeyEnter) {
					chosen := entries[s.PaletteCursor]
					s.executePaletteEntry(chosen, false)
					open = false
					return
				}
				if panel.Shortcut(ui.Cmd, ui.KeyEnter) {
					chosen := entries[s.PaletteCursor]
					s.executePaletteEntry(chosen, true)
					open = false
					return
				}
			}

			ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
				ui.Icon(c, lucide.Search()).Size(float32(tok.Spacing16), float32(tok.Spacing16)).TextColor(tok.Accent)
				inp := ui.TextInputBase(c, &s.PaletteQuery).FillWidth().FontSize(float32(tok.TextBody)).TextColor(tok.Text).
					Padding(float32(tok.Spacing4)*0.5, float32(tok.Spacing4)).Placeholder("搜索实验 (.asterml)、试剂批号或快捷命令…").
					Grow(1).MinWidth(0)
				inp.AutoFocus()
			})

			ui.Divider(c)

			if len(entries) == 0 {
				ui.Column(c).FillWidth().Padding(float32(tok.Spacing24)).Center().Gap(float32(tok.Spacing4)).Children(func() {
					components.Text(c, "未找到匹配实验、试剂或命令").FontSize(float32(tok.TextBody)).Bold().TextColor(tok.TextMuted)
					components.Text(c, "尝试输入实验名称、试剂位置或直接清空搜索。").FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
				})
				return
			}

			ui.Scroll(c).FillWidth().MaxHeight(float32(tok.Spacing48) * 9.0).Children(func() {
				ui.Column(c).FillWidth().Gap(float32(tok.Spacing4)).Children(func() {
					var lastKind paletteEntryKind = 255
					for i, entry := range entries {
						entry := entry
						if entry.kind != lastKind {
							lastKind = entry.kind
							ui.Text(c, paletteGroupLabel(entry.kind)).
								FontSize(float32(tok.TextMeta)).Bold().TextColor(tok.TextMuted).
								Padding(float32(tok.Spacing4), float32(tok.Spacing4))
						}
						selected := i == s.PaletteCursor
						targetBg := ui.Transparent
						if selected {
							targetBg = tok.AccentSoftBg
						}
						palRow := ui.Row(c.Key("pal-"+entry.key)).FillWidth().Padding(float32(tok.Spacing4), float32(tok.Spacing8)).
							Radius(float32(tok.ControlRadius)).
							AlignItems(ui.Center).Gap(float32(tok.Spacing8))
						palRow.Background(palRow.AnimateColor("pal-bg", targetBg, theme.MotionSnappy))
						palRow.Children(func() {
							rowBtn := ui.ButtonBase(c).Grow(1).MinWidth(0).Justify(ui.Start).Label(entry.title)
							theme.ApplyTactilePress(rowBtn)
							rowBtn.Children(func() {
								ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
									ui.Icon(c, paletteEntryIcon(entry)).Size(float32(tok.Spacing16), float32(tok.Spacing16)).TextColor(tok.Accent)
									ui.Column(c).Grow(1).MinWidth(0).Children(func() {
										components.Text(c, entry.title).FontSize(float32(tok.TextBody)).Bold().TextColor(tok.Text).SingleLine().Ellipsis("…")
										if entry.subtitle != "" {
											components.Text(c, entry.subtitle).FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted).SingleLine().Ellipsis("…")
										}
									})
									if entry.shortcut != "" {
										components.Badge(c, entry.shortcut, components.BadgeOptions{Tone: components.BadgeNeutral})
									}
								})
							})
							if rowBtn.Clicked() {
								s.executePaletteEntry(entry, false)
								open = false
							}
							if entry.kind == paletteExperiment && entry.exp != nil {
								sp := components.IconButton(c, lucide.Columns2(), "在分屏中打开", components.IconButtonCompact)
								if sp.Clicked() {
									s.executePaletteEntry(entry, true)
									open = false
								}
							}
						})
					}
				})
			})
			ui.Divider(c)
			ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(float32(tok.Spacing12)).Children(func() {
				components.Text(c, "↑↓ 移动 · Enter 打开 · Ctrl+Enter 分屏打开 · Esc 关闭").
					FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted).SingleLine()
			})
		})
	})
	})
	s.PaletteOpen = open
}

func pantryQuickStatusBadge(st string) (string, components.BadgeTone) {
	switch st {
	case "expired":
		return "已过期", components.BadgeDanger
	case "near_expiry":
		return "临期", components.BadgeWarning
	case "exhausted":
		return "已用尽", components.BadgeNeutral
	case "opened":
		return "已开封", components.BadgeAccent
	default:
		return "未开封", components.BadgeSuccess
	}
}

func paletteGroupLabel(k paletteEntryKind) string {
	switch k {
	case paletteExperiment:
		return "实验 (.asterml)"
	case palettePantry:
		return "储藏室试剂"
	default:
		return "快捷命令"
	}
}

func paletteEntryIcon(e paletteEntry) *ui.SVG {
	switch e.kind {
	case paletteExperiment:
		if e.exp != nil {
			return stageIcon(e.exp.Stage)
		}
		return lucide.FlaskConical()
	case palettePantry:
		return lucide.Package()
	default:
		return lucide.Command()
	}
}

func (s *Shell) executePaletteEntry(entry paletteEntry, split bool) {
	s.PaletteOpen = false
	switch entry.kind {
	case paletteExperiment:
		if entry.exp != nil {
			if split {
				s.openExperimentInSplit(*entry.exp)
			} else {
				s.openExperiment(*entry.exp)
			}
		}
	case palettePantry:
		if s.OnLocatePantry != nil && entry.pantryID != "" {
			s.OnLocatePantry(entry.pantryID)
			return
		}
		s.OpenPantryTab()
	case paletteAction:
		if entry.runAct != nil {
			entry.runAct()
		}
	}
}

func (s *Shell) buildPaletteEntries() []paletteEntry {
	q := strings.ToLower(strings.TrimSpace(s.PaletteQuery))
	var out []paletteEntry

	// 1. Experiments
	for _, exp := range s.resolvedExperiments() {
		exp := exp
		sub := fmt.Sprintf("%s · %s", firstNonEmptyStr(exp.Date, s.resolvedToday().Format("2006-01-02")), StageLabel(exp.Stage))
		if exp.Summary != "" {
			sub += " · " + exp.Summary
		}
		if q != "" && !strings.Contains(strings.ToLower(exp.Title+" "+sub), q) {
			continue
		}
		out = append(out, paletteEntry{
			kind:     paletteExperiment,
			key:      "exp-" + exp.ID,
			title:    exp.Title,
			subtitle: sub,
			exp:      &exp,
		})
	}

	// 2. Pantry reagents
	for _, p := range s.PantryItems {
		stLabel, _ := pantryQuickStatusBadge(p.Status)
		sub := fmt.Sprintf("%s · %s", p.Location, stLabel)
		if p.ExpiresOn != "" {
			sub += " · 效期 " + p.ExpiresOn
		}
		if q != "" && !strings.Contains(strings.ToLower(p.Name+" "+sub+" "+p.LotNumber), q) {
			continue
		}
		out = append(out, paletteEntry{
			kind:     palettePantry,
			key:      "pan-" + p.ID,
			title:    p.Name,
			subtitle: sub,
			pantryID: p.ID,
		})
	}

	// 3. Host context actions + Built-in workspace commands
	var actions []paletteEntry
	for _, cmd := range s.PaletteCommands {
		cmd := cmd
		actions = append(actions, paletteEntry{
			kind:     paletteAction,
			key:      "host-" + cmd.ID,
			title:    cmd.Title,
			subtitle: cmd.Subtitle,
			shortcut: cmd.Shortcut,
			runAct:   cmd.OnRun,
		})
	}
	actions = append(actions,
		paletteEntry{
			kind: paletteAction, key: "cmd-new", title: "新建实验",
			subtitle: "在当前项目与日期新建 .asterml 实验文档", shortcut: s.HotkeyDisplay("vault.new_exp"),
			runAct: func() { s.triggerCreateForDate(s.ensureSelectedDate()) },
		},
		paletteEntry{
			kind: paletteAction, key: "cmd-global-search", title: "Vault 全局全文搜索",
			subtitle: "跨所有 .asterml 实验步骤、配方、试剂编号与笔记全文检索", shortcut: s.HotkeyDisplay("nav.global_search"),
			runAct: func() {
				s.GlobalSearchOpen = true
				s.refreshGlobalSearch()
			},
		},
		paletteEntry{
			kind: paletteAction, key: "cmd-drawer-outline", title: "打开 / 收起右侧步骤大纲与速算抽屉",
			subtitle: "查看当前实验步骤大纲或使用稀释/摩尔速算器", shortcut: s.HotkeyDisplay("nav.right_drawer"),
			runAct: func() { s.ToggleRightDrawer() },
		},
		paletteEntry{
			kind: paletteAction, key: "cmd-split", title: "向右分屏打开当前标签页",
			subtitle: "并排对照两份实验、同一实验的不同区域或储藏室", shortcut: s.HotkeyDisplay("pane.split_right"),
			runAct: func() { s.ToggleSplit() },
		},
		paletteEntry{
			kind: paletteAction, key: "cmd-cal", title: "切换到实验日程",
			subtitle: "查看月/周实验安排与排期", shortcut: s.HotkeyDisplay("nav.calendar"),
			runAct: func() { s.OpenCalendarTab() },
		},
		paletteEntry{
			kind: paletteAction, key: "cmd-pantry", title: "打开储藏室",
			subtitle: "管理冰箱货架、冻存盒孔位与试剂库存",
			runAct: func() { s.OpenPantryTab() },
		},
		paletteEntry{
			kind: paletteAction, key: "cmd-preset-obsidian", title: "切换配色：纯净白纸 (Obsidian 默认)",
			subtitle: "100% 纯净白纸底搭配 #f5f6f8 无边框冷灰层次与极克制淡紫标签",
			runAct: func() { s.ThemePreset = theme.PresetObsidian; s.notifySettingsChanged(); s.Invalidate() },
		},
		paletteEntry{
			kind: paletteAction, key: "cmd-preset-zinc", title: "切换配色：冷银极简 (Cool Zinc)",
			subtitle: "无彩色中性冷灰底配纯白卡片，干净克制",
			runAct: func() { s.ThemePreset = theme.PresetZinc; s.notifySettingsChanged(); s.Invalidate() },
		},
		paletteEntry{
			kind: paletteAction, key: "cmd-preset-warm", title: "切换配色：经典暖润实验纸 (Warm Paper)",
			subtitle: "保留原始温润米白护眼纸面 (#f7f6f3)",
			runAct: func() { s.ThemePreset = theme.PresetWarm; s.notifySettingsChanged(); s.Invalidate() },
		},
		paletteEntry{
			kind: paletteAction, key: "cmd-toggle-glass", title: "切换 Liquid Glass 液态玻璃材质",
			subtitle: "在 MyGo 折射液态玻璃与经典平铺磨砂之间实时切换",
			runAct: func() { s.LiquidGlass = !s.LiquidGlass; s.notifySettingsChanged(); s.Invalidate() },
		},
	)
	for _, b := range actions {
		if q != "" && !strings.Contains(strings.ToLower(b.title+" "+b.subtitle), q) {
			continue
		}
		out = append(out, b)
	}
	return out
}
