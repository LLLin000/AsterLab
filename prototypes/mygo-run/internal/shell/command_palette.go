package shell

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/onevra/source/components"
	"github.com/oneo-me/onevra/source/icons/lucide"

	"asterlab-mygo-prototype/internal/theme"
	"asterlab-mygo-prototype/internal/uikit/kit"
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
			kit.ApplyElevation(panel, tok, kit.ElevationModal)
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
				ph := "搜索实验 (.asterml)、试剂/样本或快捷命令…"
				if s.PaletteMode == "files" {
					ph = "快速切换：搜索实验文档 (.asterml) 或储藏室试剂 / 样本…"
				} else if s.PaletteMode == "commands" {
					ph = "命令面板：输入操作名称（如 新建、模板、冻存盒、导出、切库）…"
				}
				inp := ui.TextInputBase(c, &s.PaletteQuery).FillWidth().FontSize(float32(tok.TextBody)).TextColor(tok.Text).
					Padding(float32(tok.Spacing4)*0.5, float32(tok.Spacing4)).Placeholder(ph).
					Grow(1).MinWidth(0)
				// UI-15: Handle Enter / Up / Down directly on the focused TextInput so pressing Enter
				// always executes the highlighted command or file immediately!
				inp.OnTextKey(func(k ui.TextKey) bool {
					if len(entries) == 0 {
						return false
					}
					switch k.Key {
					case ui.KeyDown:
						s.PaletteCursor = (s.PaletteCursor + 1) % len(entries)
						return true
					case ui.KeyUp:
						s.PaletteCursor = (s.PaletteCursor - 1 + len(entries)) % len(entries)
						return true
					case ui.KeyEnter:
						chosen := entries[s.PaletteCursor]
						s.executePaletteEntry(chosen, k.Mods&ui.Cmd != 0)
						open = false
						return true
					}
					return false
				})
				inp.AutoFocus()

				// Compact mode switcher chips (Obsidian Ctrl+O vs Ctrl+P separation — UI-14)
				for _, m := range []struct{ key, lbl string }{
					{"", "全部"},
					{"files", "文件/储藏室"},
					{"commands", "命令"},
				} {
					m := m
					active := s.PaletteMode == m.key
					bg, fg := ui.Transparent, tok.TextMuted
					if active {
						bg, fg = tok.AccentSoftBg, tok.Accent
					}
					chip := ui.ButtonBase(c.Key("pal-mode-"+m.key)).Label(m.lbl).
						Padding(2, float32(tok.Spacing8)).Radius(float32(tok.ControlRadius)).
						Background(bg).Shrink(0).OnClick(func() {
						s.PaletteMode = m.key
						s.PaletteCursor = 0
					})
					chip.Children(func() {
						txt := ui.Text(c, m.lbl).FontSize(float32(tok.TextMeta)).TextColor(fg).SingleLine()
						if active {
							txt.Bold()
						}
					})
				}
			})

			ui.Divider(c)

			if len(entries) == 0 {
				kit.EmptyState(c, tok, lucide.Search(), "未找到匹配实验、试剂或命令", "尝试输入实验名称、试剂位置或直接清空搜索。", "", nil)
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
										kit.Keycap(c, tok, entry.shortcut)
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
			ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
				kit.Keycap(c, tok, "↑↓")
				components.Text(c, "移动").FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted).SingleLine()
				kit.Keycap(c, tok, "Enter")
				components.Text(c, "打开").FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted).SingleLine()
				kit.Keycap(c, tok, "Ctrl+Enter")
				components.Text(c, "分屏").FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted).SingleLine()
				ui.Spacer(c)
				kit.Keycap(c, tok, "Esc")
				components.Text(c, "关闭").FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted).SingleLine()
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
	includeFiles := s.PaletteMode != "commands"
	includeCommands := s.PaletteMode != "files"

	if includeFiles {
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

		// 2. Pantry reagents & biosamples
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
	}

	if !includeCommands {
		return out
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
			kind: paletteAction, key: "cmd-apply-template", title: "从模板新建 / 套用模板…",
			subtitle: "选择 templates/ 文件夹中的任意实验模板新建文档",
			runAct: func() {
				if s.OnOpenTemplatePicker != nil {
					s.OnOpenTemplatePicker(s.ensureSelectedDate())
				}
			},
		},
		paletteEntry{
			kind: paletteAction, key: "cmd-vault-launcher", title: "切换 / 管理本地实验库 (Vault)…",
			subtitle: "新建实验库、切换已有文件夹或管理最近打开的实验库",
			runAct: func() { s.openVaultLauncher() },
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
		paletteEntry{
			kind: paletteAction, key: "cmd-check-update", title: "检查软件更新",
			subtitle: "查询 AsterLab 最新发布版本并下载更新",
			runAct: func() {
				s.SettingsView.Category = "about"
				s.VaultSettingsOpen = true
				if s.OnCheckUpdate != nil {
					s.OnCheckUpdate()
				}
			},
		},
		paletteEntry{
			kind: paletteAction, key: "cmd-feedback-diag", title: "反馈问题 / 导出脱敏诊断包",
			subtitle: "收集脱敏运行日志、异常快照并在 GitHub 快速提交反馈",
			runAct: func() {
				s.FeedbackOpen = true
			},
		},
		paletteEntry{
			kind: paletteAction, key: "cmd-welcome-guide", title: "打开首次使用引导 / 示范实验库",
			subtitle: "创建 CCK-8 示范实验与预置试剂，快速熟悉 AsterLab 结构",
			runAct: func() {
				s.WelcomeOpen = true
			},
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
