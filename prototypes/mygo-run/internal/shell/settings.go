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
)

type hotkeySpec struct {
	ID      string
	Action  string
	Group   string
	Default string
}

var defaultHotkeySpecs = []hotkeySpec{
	{ID: "cmd.palette", Action: "打开命令面板 / 快速切换实验", Group: "导航", Default: "Ctrl+K / Ctrl+O"},
	{ID: "nav.global_search", Action: "全局全文搜索 Vault", Group: "导航", Default: "Ctrl+Shift+F"},
	{ID: "vault.new_exp", Action: "新建实验", Group: "导航", Default: "Ctrl+N"},
	{ID: "nav.calendar", Action: "实验日程主页", Group: "导航", Default: "Ctrl+1"},
	{ID: "nav.back", Action: "后退", Group: "导航", Default: "Alt+←"},
	{ID: "nav.forward", Action: "前进", Group: "导航", Default: "Alt+→"},
	{ID: "nav.left_sidebar", Action: "切换左侧栏", Group: "工作区", Default: "Ctrl+B"},
	{ID: "nav.right_drawer", Action: "切换右侧栏", Group: "工作区", Default: "Ctrl+Shift+R"},
	{ID: "pane.split_right", Action: "向右分屏", Group: "工作区", Default: "Ctrl+\\"},
	{ID: "tab.close", Action: "关闭标签页", Group: "工作区", Default: "Ctrl+W"},
	{ID: "doc.toggle_mode", Action: "切换编辑 / 阅读", Group: "编辑器", Default: "Ctrl+E"},
	{ID: "doc.undo", Action: "撤销", Group: "编辑器", Default: "Ctrl+Z"},
	{ID: "doc.redo", Action: "重做", Group: "编辑器", Default: "Ctrl+Y"},
	{ID: "tree.rename", Action: "重命名", Group: "文件树", Default: "F2"},
	{ID: "tree.delete", Action: "移入回收站", Group: "文件树", Default: "Del"},
	{ID: "app.settings", Action: "打开设置", Group: "系统", Default: "Ctrl+,"},
}

func (s *Shell) ensureSettingsDefaults() {
	if s.ThemeSource == "" {
		s.ThemeSource = "system"
	}
	s.SyncThemeLook(nil)
	if s.Density == "" {
		s.Density = theme.DensityDefault
	}
	if s.DefaultOpenMode == "" {
		s.DefaultOpenMode = "edit"
	}
	if s.DefaultPlateFormat == 0 {
		s.DefaultPlateFormat = 96
	}
	if s.DefaultExcessPct == 0 {
		s.DefaultExcessPct = 10
	}
	s.EnsureMobileDefaults()
	if s.Hotkeys == nil {
		s.Hotkeys = map[string]string{}
	}
	for _, spec := range defaultHotkeySpecs {
		if strings.TrimSpace(s.Hotkeys[spec.ID]) == "" {
			s.Hotkeys[spec.ID] = spec.Default
		}
	}
}

func (s *Shell) notifySettingsChanged() {
	if s.OnSettingsChanged != nil {
		s.OnSettingsChanged()
	}
}

// HotkeyDisplay returns the active shortcut badge text for a command ID.
func (s *Shell) HotkeyDisplay(id string) string {
	s.ensureSettingsDefaults()
	if v := strings.TrimSpace(s.Hotkeys[id]); v != "" {
		return v
	}
	for _, spec := range defaultHotkeySpecs {
		if spec.ID == id {
			return spec.Default
		}
	}
	return ""
}

func (s *Shell) renderObsidianSettingsDialog(c *ui.Context) {
	if !s.VaultSettingsOpen {
		return
	}
	s.ensureSettingsDefaults()
	t := c.Theme()
	w, h := c.Size()
	if w <= 0 {
		w = 1080
	}
	if h <= 0 {
		h = 720
	}
	var inset float32 = 20
	if s.window != nil {
		inset = sidebar.HeaderHeight(c)
	}
	width := min(t.Space(205), w*0.9, max(0, w-2*inset))
	height := min(t.Space(175), h*0.86)
	narrow := width < t.Space(170) || s.IsMobileLayout(c)

	open := s.VaultSettingsOpen
	tok := theme.GetTokens(t)
	ui.Box(c.Key("obsidian-settings-dlg")).Children(func() {
		components.Dialog(c, &open, components.DialogOptions{Label: "设置", Inset: inset}, func(panel ui.Element) {
			panel.Width(width).Height(height).Padding(0).Gap(0).Clip()
			if tok.Dark {
				panel.Background(tok.Surface).Border(1, tok.BorderStrong)
			}
			components.Settings(c, &s.SettingsView, s.obsidianSettingsCategories(c), components.SettingsOptions{
				Narrow:            narrow,
				Title:             "设置",
				SearchPlaceholder: "搜索设置…",
				CategoryLabel:     "设置",
				ResultsTitle:      "搜索结果",
				EmptyText:         "无匹配项",
				Close: func() {
					open = false
				},
			})
		})
	})
	if s.ComponentGalleryOpen {
		s.VaultSettingsOpen = false
	} else {
		s.VaultSettingsOpen = open
	}
}

func (s *Shell) obsidianSettingsCategories(c *ui.Context) []components.SettingsCategory {
	tok := theme.GetTokens(c.Theme())

	applyThemeChoice := func(choice string) {
		s.ThemeSource = choice
		if s.window != nil {
			switch choice {
			case "light":
				mygo.Theme.SetSource(mygo.ThemeLight)
			case "dark":
				mygo.Theme.SetSource(mygo.ThemeDark)
			default:
				mygo.Theme.SetSource(mygo.ThemeSystem)
			}
		}
		s.notifySettingsChanged()
		c.Invalidate()
	}

	vaultCat := components.SettingsCategory{
		ID:      "vault",
		Title:   "文件与库",
		Section: "工作区",
		Icon:    lucide.FolderOpen(),
		Groups: []components.SettingsGroup{
			{
				ID:    "vault-location",
				Title: "实验库",
				Items: []components.SettingsItem{
					{
						ID:       "vault-path",
						Title:    "当前库路径",
						Keywords: []string{"vault", "path", "目录", "路径", "切换"},
						Stacked:  true,
						Build: func(c *ui.Context) {
							ui.Row(c).FillWidth().Gap(float32(tok.Spacing8)).AlignItems(ui.Center).Children(func() {
								ui.Box(c).Grow(1).MinWidth(0).Children(func() {
									components.TextBox(c, &s.VaultPathInput, components.TextBoxOptions{}).
										Placeholder("选择本地实验库目录")
								})
								if s.OnPickVaultPath != nil && components.Button(c, "浏览…", components.ButtonOutline).Clicked() {
									if picked := strings.TrimSpace(s.OnPickVaultPath()); picked != "" {
										s.VaultPathInput = picked
										if s.OnSwitchVaultPath != nil {
											s.OnSwitchVaultPath(picked)
										}
									}
								}
								if components.Button(c, "切换", components.ButtonSecondary).Clicked() && s.OnSwitchVaultPath != nil {
									s.OnSwitchVaultPath(strings.TrimSpace(s.VaultPathInput))
									s.VaultSettingsOpen = false
								}
								if s.OnRevealInExplorer != nil && components.Button(c, "打开目录", components.ButtonOutline).Clicked() {
									s.OnRevealInExplorer("")
								}
							})
						},
					},
					{
						ID:       "hide-assets",
						Title:    "隐藏 assets 文件夹",
						Keywords: []string{"assets", "隐藏", "附件", "树"},
						Build: func(c *ui.Context) {
							if components.Switch(c, &s.HideAssetsInTree, "隐藏 assets", components.SwitchOptions{HideLabel: true}).Changed() {
								if s.OnToggleHideAssets != nil {
									s.OnToggleHideAssets(s.HideAssetsInTree)
								}
								s.notifySettingsChanged()
							}
						},
					},
					{
						ID:       "auto-archive-assets",
						Title:    "粘贴或拖入图片自动存至 ./assets/",
						Keywords: []string{"paste", "drag", "assets", "图片", "相对路径"},
						Build: func(c *ui.Context) {
							if components.Switch(c, &s.AutoArchiveAssets, "自动存至 assets", components.SwitchOptions{HideLabel: true}).Changed() {
								s.notifySettingsChanged()
							}
						},
					},
					{
						ID:       "default-proj",
						Title:    "新建实验默认位置",
						Keywords: []string{"project", "folder", "默认", "新建", "项目"},
						Build: func(c *ui.Context) {
							choices := append([]string{"", "/"}, s.VaultFolders...)
							sel := components.Select(c, &s.DefaultProjectDir)
							labelFor := func(v string) string {
								switch v {
								case "":
									return "上次所在项目"
								case "/":
									return "库根目录"
								default:
									return v
								}
							}
							sel.Trigger.MinWidth(c.Theme().Space(36)).Children(func() {
								components.Text(c, labelFor(s.DefaultProjectDir)).SingleLine()
							})
							sel.Popup(func(panel ui.Element) {
								for _, ch := range choices {
									ch := ch
									sel.Item(ch).Children(func() {
										components.Text(c, labelFor(ch)).SingleLine()
									})
								}
							})
							if sel.Trigger.Changed() {
								s.notifySettingsChanged()
							}
						},
					},
				},
			},
			{
				ID:    "vault-trash",
				Title: "文件恢复",
				Items: []components.SettingsItem{
					{
						ID:       "trash-restore-list",
						Title:    "回收站",
						Keywords: []string{"trash", "restore", "回收站", "恢复", "删除"},
						Stacked:  len(s.TrashItems) > 0,
						Build: func(c *ui.Context) {
							if len(s.TrashItems) == 0 {
								components.Text(c, "空").FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted).SingleLine().Shrink(0)
								return
							}
							ui.Column(c).FillWidth().Gap(float32(tok.Spacing4)).Children(func() {
								limit := len(s.TrashItems)
								if limit > 8 {
									limit = 8
								}
								for i := 0; i < limit; i++ {
									it := s.TrashItems[i]
									ui.Row(c.Key("trash-it-"+it.TrashName)).FillWidth().AlignItems(ui.Center).
										Padding(float32(tok.Spacing4), float32(tok.Spacing8)).
										Radius(float32(tok.ControlRadius)).Background(tok.Elevated).Gap(float32(tok.Spacing8)).Children(func() {
										ui.Column(c).Grow(1).MinWidth(0).Children(func() {
											components.Text(c, it.OrigRel).FontSize(float32(tok.TextBody)).TextColor(tok.Text).SingleLine().Ellipsis("…")
											components.Text(c, it.DeletedAt.Format("2006-01-02 15:04")).FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted).FontFeatures("tnum")
										})
										if components.Button(c, "还原", components.ButtonSecondary).Clicked() && s.OnVaultRestoreTrash != nil {
											s.OnVaultRestoreTrash(it.TrashName)
										}
									})
								}
							})
						},
					},
					{
						ID:          "snapshot-recovery-list",
						Title:       "历史版本快照",
						Description: "自动保留最近 20 个本地快照",
						Keywords:    []string{"snapshot", "recovery", "history", "文件恢复", "快照", "历史版本", "恢复"},
						Stacked:     len(s.CurrentSnapshots) > 0,
						Build: func(c *ui.Context) {
							if s.CurrentDocRelPath == "" {
								components.Text(c, "需打开实验").FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted).SingleLine().Shrink(0)
								return
							}
							if len(s.CurrentSnapshots) == 0 {
								components.Text(c, "暂无快照").FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted).SingleLine().Shrink(0)
								return
							}
							ui.Column(c).FillWidth().Gap(float32(tok.Spacing4)).Children(func() {
								limit := len(s.CurrentSnapshots)
								if limit > 8 {
									limit = 8
								}
								for i := 0; i < limit; i++ {
									sn := s.CurrentSnapshots[i]
									ui.Row(c.Key("snap-"+sn.ID)).FillWidth().AlignItems(ui.Center).
										Padding(float32(tok.Spacing4), float32(tok.Spacing8)).
										Radius(float32(tok.ControlRadius)).Background(tok.Elevated).Gap(float32(tok.Spacing8)).Children(func() {
										ui.Column(c).Grow(1).MinWidth(0).Children(func() {
											components.Text(c, sn.CreatedAt.Format("2006-01-02 15:04:05")).FontSize(float32(tok.TextBody)).TextColor(tok.Text).FontFeatures("tnum")
											components.Text(c, fmt.Sprintf("%d B · %s", sn.Size, sn.RelPath)).FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted).SingleLine().Ellipsis("…").FontFeatures("tnum")
										})
										if components.Button(c, "恢复此版本", components.ButtonSecondary).Clicked() && s.OnRestoreSnapshot != nil {
											s.OnRestoreSnapshot(s.CurrentDocRelPath, sn.ID)
										}
									})
								}
							})
						},
					},
				},
			},
		},
	}

	mobileItems := []components.SettingsItem{
		{
			ID:       "mobile-preview",
			Title:    "移动端单栏预览",
			Keywords: []string{"mobile", "android", "ios", "touch", "移动端", "自适应"},
			Build: func(c *ui.Context) {
				if components.Switch(c, &s.ForceMobilePreview, "移动预览模式", components.SwitchOptions{HideLabel: true}).Changed() {
					s.MobileDrawer = ""
					s.MobileTabSwitcherOpen = false
				}
			},
		},
	}
	if s.ForceMobilePreview || s.IsMobileLayout(c) {
		mobileItems = append(mobileItems,
			components.SettingsItem{
				ID:       "mobile-pull-action",
				Title:    "顶部下拉快捷动作",
				Keywords: []string{"pull", "quick action", "下拉", "快捷手势", "命令面板"},
				Build: func(c *ui.Context) {
					labels := map[string]string{
						"palette": "命令面板",
						"new_exp": "新建实验",
						"calc":    "速算与计时",
						"tree":    "实验库",
					}
					sel := components.Select(c, &s.MobilePullAction)
					sel.Trigger.Children(func() {
						components.Text(c, firstNonEmptyStr(labels[s.MobilePullAction], "命令面板")).SingleLine()
					})
					sel.Popup(func(ui.Element) {
						for _, opt := range []string{"palette", "new_exp", "calc", "tree"} {
							opt := opt
							sel.Item(opt).Children(func() {
								components.Text(c, labels[opt]).SingleLine()
							})
						}
					})
					if sel.Trigger.Changed() {
						s.notifySettingsChanged()
					}
				},
			},
			components.SettingsItem{
				ID:       "mobile-autohide-dock",
				Title:    "滚动时自动收起底栏",
				Keywords: []string{"autohide", "dock", "scroll", "悬浮", "底栏"},
				Build: func(c *ui.Context) {
					if components.Switch(c, &s.MobileAutoHideDock, "自动隐藏底栏", components.SwitchOptions{HideLabel: true}).Changed() {
						if !s.MobileAutoHideDock {
							s.MobileDockHidden = false
						}
						s.notifySettingsChanged()
					}
				},
			},
			components.SettingsItem{
				ID:       "mobile-edit-toolbar",
				Title:    "编辑态显示底部工具条",
				Keywords: []string{"toolbar", "edit", "mobile", "工具栏", "编辑"},
				Build: func(c *ui.Context) {
					if components.Switch(c, &s.MobileEditToolbar, "移动编辑工具条", components.SwitchOptions{HideLabel: true}).Changed() {
						s.MobileDockForceNav = false
						s.notifySettingsChanged()
					}
				},
			},
		)
	}

	appearanceCat := components.SettingsCategory{
		ID:      "appearance",
		Title:   "外观",
		Section: "工作区",
		Icon:    lucide.Palette(),
		Groups: []components.SettingsGroup{
			{
				ID:    "theme-group",
				Title: "界面风格",
				Items: []components.SettingsItem{
					{
						ID:       "theme-mode",
						Title:    "外观模式",
						Keywords: []string{"theme", "dark", "light", "system", "主题", "深色", "浅色"},
						Build: func(c *ui.Context) {
							options := []string{"system", "light", "dark"}
							names := map[string]string{"system": "跟随系统", "light": "浅色", "dark": "深色"}
							sel := components.Select(c, &s.ThemeSource)
							sel.Trigger.MinWidth(c.Theme().Space(32)).Children(func() {
								components.Text(c, names[s.ThemeSource]).SingleLine()
							})
							sel.Popup(func(panel ui.Element) {
								for _, opt := range options {
									opt := opt
									sel.Item(opt).Children(func() {
										components.Text(c, names[opt]).SingleLine()
									})
								}
							})
							if sel.Trigger.Changed() {
								applyThemeChoice(s.ThemeSource)
							}
						},
					},
					{
						ID:       "theme-preset",
						Title:    "配色方案",
						Keywords: []string{"preset", "obsidian", "zinc", "warm", "配色", "画布", "冷灰", "纯白"},
						Build: func(c *ui.Context) {
							options := []string{theme.PresetObsidian, theme.PresetZinc, theme.PresetWarm}
							names := map[string]string{
								theme.PresetObsidian: "纯净白纸",
								theme.PresetZinc:     "冷银灰",
								theme.PresetWarm:     "暖润纸面",
							}
							if s.ThemePreset == "" || s.ThemePreset == "aurora" {
								s.ThemePreset = theme.PresetObsidian
							}
							sel := components.Select(c, &s.ThemePreset)
							sel.Trigger.MinWidth(c.Theme().Space(32)).Children(func() {
								components.Text(c, firstNonEmptyStr(names[s.ThemePreset], names[theme.PresetObsidian])).SingleLine()
							})
							sel.Popup(func(panel ui.Element) {
								for _, opt := range options {
									opt := opt
									sel.Item(opt).Children(func() {
										components.Text(c, names[opt]).SingleLine()
									})
								}
							})
							if sel.Trigger.Changed() {
								s.SyncThemeLook(c)
								s.notifySettingsChanged()
								c.Invalidate()
							}
						},
					},
					{
						ID:       "liquid-glass",
						Title:    "液态玻璃",
						Keywords: []string{"glass", "liquid", "blur", "液态玻璃", "毛玻璃"},
						Build: func(c *ui.Context) {
							if components.Switch(c, &s.LiquidGlass, "液态玻璃", components.SwitchOptions{HideLabel: true}).Changed() {
								s.SyncThemeLook(c)
								s.notifySettingsChanged()
								c.Invalidate()
							}
						},
					},
					{
						ID:          "text-scale",
						Title:       "界面缩放",
						Description: "快捷键 Ctrl+= / Ctrl+- / Ctrl+0",
						Keywords:    []string{"font", "scale", "zoom", "字号", "图标", "缩放"},
						Build: func(c *ui.Context) {
							scaleStr := fmt.Sprintf("%.2f", s.TextScale)
							if s.TextScale <= 0 {
								scaleStr = "0.00"
							}
							options := []string{"0.00", "0.90", "1.00", "1.10", "1.15", "1.25", "1.35", "1.50"}
							names := map[string]string{
								"0.00": "跟随系统",
								"0.90": "90%",
								"1.00": "100%",
								"1.10": "110%",
								"1.15": "115%",
								"1.25": "125%",
								"1.35": "135%",
								"1.50": "150%",
							}
							curLabel := names[scaleStr]
							if curLabel == "" {
								curLabel = fmt.Sprintf("%.0f%%", s.TextScale*100)
							}
							sel := components.Select(c, &scaleStr)
							sel.Trigger.MinWidth(c.Theme().Space(32)).Children(func() {
								components.Text(c, curLabel).SingleLine()
							})
							sel.Popup(func(panel ui.Element) {
								for _, opt := range options {
									opt := opt
									sel.Item(opt).Children(func() {
										components.Text(c, names[opt]).SingleLine()
									})
								}
							})
							if sel.Trigger.Changed() {
								switch scaleStr {
								case "0.90":
									s.TextScale = 0.90
								case "1.00":
									s.TextScale = 1.00
								case "1.10":
									s.TextScale = 1.10
								case "1.15":
									s.TextScale = 1.15
								case "1.25":
									s.TextScale = 1.25
								case "1.35":
									s.TextScale = 1.35
								case "1.50":
									s.TextScale = 1.50
								default:
									s.TextScale = 0
								}
								s.SyncThemeLook(c)
								s.notifySettingsChanged()
								c.Invalidate()
							}
						},
					},
					{
						ID:       "ui-density",
						Title:    "界面密度",
						Keywords: []string{"density", "compact", "comfortable", "密度", "紧凑", "舒展"},
						Build: func(c *ui.Context) {
							s.Density = theme.NormalizeDensity(s.Density)
							options := []string{theme.DensityCompact, theme.DensityDefault, theme.DensityComfortable}
							names := map[string]string{
								theme.DensityCompact:     "紧凑",
								theme.DensityDefault:     "标准",
								theme.DensityComfortable: "舒展",
							}
							sel := components.Select(c, &s.Density)
							sel.Trigger.MinWidth(c.Theme().Space(32)).Children(func() {
								components.Text(c, firstNonEmptyStr(names[s.Density], names[theme.DensityDefault])).SingleLine()
							})
							sel.Popup(func(panel ui.Element) {
								for _, opt := range options {
									opt := opt
									sel.Item(opt).Children(func() {
										components.Text(c, names[opt]).SingleLine()
									})
								}
							})
							if sel.Trigger.Changed() {
								s.SyncThemeLook(c)
								s.notifySettingsChanged()
								c.Invalidate()
							}
						},
					},
					{
						ID:       "reduce-motion",
						Title:    "减弱动效",
						Keywords: []string{"motion", "animation", "reduce", "动效", "无障碍"},
						Build: func(c *ui.Context) {
							if components.Switch(c, &s.ReduceMotion, "减弱动效", components.SwitchOptions{HideLabel: true}).Changed() {
								s.SyncThemeLook(c)
								s.notifySettingsChanged()
								c.Invalidate()
							}
						},
					},
				},
			},
			{
				ID:    "mobile-layout",
				Title: "紧凑与触控",
				Items: mobileItems,
			},
		},
	}

	editorCat := components.SettingsCategory{
		ID:      "editor",
		Title:   "编辑器",
		Section: "工作区",
		Icon:    lucide.SlidersHorizontal(),
		Groups: []components.SettingsGroup{
			{
				ID:    "doc-defaults",
				Title: "实验默认",
				Items: []components.SettingsItem{
					{
						ID:       "default-open-mode",
						Title:    "默认打开视图",
						Keywords: []string{"mode", "edit", "read", "编辑", "阅读", "默认"},
						Build: func(c *ui.Context) {
							// Avoid components.Tabs inside Settings Dialog (Onevra Tabs.Draw panics on Dialog Exit transition).
							ui.Row(c).Padding(2).Radius(float32(tok.ControlRadius)).
								Background(tok.Elevated).Border(1, tok.Border).Gap(2).Children(func() {
								for _, item := range []struct{ key, lbl string }{{"edit", "编辑态"}, {"read", "阅读态"}} {
									item := item
									active := s.DefaultOpenMode == item.key || (s.DefaultOpenMode == "" && item.key == "edit")
									targetBg, fg := ui.Transparent, tok.TextMuted
									if active {
										targetBg, fg = tok.Surface, tok.Text
									}
									btn := ui.ButtonBase(c.Key("set-open-mode-"+item.key)).Label(item.lbl).
										Padding(float32(tok.Spacing4), float32(tok.Spacing12)).
										Radius(float32(tok.ControlRadius) - 1)
									theme.ApplyTactilePress(btn)
									btn.Background(btn.AnimateColor("sombg", targetBg, theme.MotionMicroFade)).
										OnClick(func() {
											if item.key != s.DefaultOpenMode {
												s.DefaultOpenMode = item.key
												s.notifySettingsChanged()
											}
										})
									btn.Children(func() {
										txt := ui.Text(c, item.lbl).FontSize(float32(tok.TextMeta)).TextColor(fg).SingleLine()
										if active {
											txt.Bold()
										}
									})
								}
							})
						},
					},
					{
						ID:       "default-plate-format",
						Title:    "新孔板默认规格",
						Keywords: []string{"plate", "96", "384", "24", "孔板", "默认"},
						Build: func(c *ui.Context) {
							opts := []int{6, 12, 24, 48, 96, 384}
							sel := components.Select(c, &s.DefaultPlateFormat)
							sel.Trigger.MinWidth(c.Theme().Space(28)).Children(func() {
								components.Text(c, fmt.Sprintf("%d 孔板", s.DefaultPlateFormat)).SingleLine()
							})
							sel.Popup(func(panel ui.Element) {
								for _, opt := range opts {
									opt := opt
									sel.Item(opt).Children(func() {
										components.Text(c, fmt.Sprintf("%d 孔板", opt)).SingleLine()
									})
								}
							})
							if sel.Trigger.Changed() {
								s.notifySettingsChanged()
							}
						},
					},
					{
						ID:       "default-excess",
						Title:    "配液计算默认余量",
						Keywords: []string{"excess", "calculator", "余量", "配液"},
						Build: func(c *ui.Context) {
							opts := []int{10, 15, 20}
							sel := components.Select(c, &s.DefaultExcessPct)
							sel.Trigger.MinWidth(c.Theme().Space(28)).Children(func() {
								components.Text(c, fmt.Sprintf("+%d%%", s.DefaultExcessPct)).SingleLine()
							})
							sel.Popup(func(panel ui.Element) {
								for _, opt := range opts {
									opt := opt
									sel.Item(opt).Children(func() {
										components.Text(c, fmt.Sprintf("+%d%%", opt)).SingleLine()
									})
								}
							})
							if sel.Trigger.Changed() {
								s.notifySettingsChanged()
							}
						},
					},
				},
			},
		},
	}

	hotkeyGroups := []components.SettingsGroup{
		{
			ID:    "hotkeys-search",
			Title: "键位绑定",
			Actions: func(c *ui.Context) {
				if components.Button(c, "恢复默认", components.ButtonGhost).Clicked() {
					for _, spec := range defaultHotkeySpecs {
						s.Hotkeys[spec.ID] = spec.Default
					}
					s.notifySettingsChanged()
				}
			},
			Items: func() []components.SettingsItem {
				var items []components.SettingsItem
				for _, spec := range defaultHotkeySpecs {
					spec := spec
					items = append(items, components.SettingsItem{
						ID:          "hk-" + spec.ID,
						Title:       spec.Action,
						Description: spec.Group,
						Keywords:    []string{"hotkey", "shortcut", spec.Action, spec.Group, spec.Default},
						Build: func(c *ui.Context) {
							cur := s.HotkeyDisplay(spec.ID)
							ui.Row(c).Shrink(0).AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
								ui.Box(c).Width(c.Theme().Space(32)).Children(func() {
									components.TextBox(c, &cur, components.TextBoxOptions{}).Placeholder(spec.Default)
								})
								if strings.TrimSpace(cur) != s.Hotkeys[spec.ID] && strings.TrimSpace(cur) != "" {
									s.Hotkeys[spec.ID] = strings.TrimSpace(cur)
									s.notifySettingsChanged()
								}
							})
						},
					})
				}
				return items
			}(),
		},
	}
	hotkeysCat := components.SettingsCategory{
		ID:      "hotkeys",
		Title:   "快捷键",
		Section: "交互",
		Icon:    lucide.Keyboard(),
		Groups:  hotkeyGroups,
	}

	aboutCat := components.SettingsCategory{
		ID:      "about",
		Title:   "关于",
		Section: "系统",
		Icon:    theme.LogoParsedSmall(),
		Groups: []components.SettingsGroup{
			{
				ID:    "app-update",
				Title: "软件版本与自动更新",
				Items: []components.SettingsItem{
					{
						ID:       "update-status",
						Title:    "当前版本",
						Keywords: []string{"update", "version", "release", "升级", "更新", "版本"},
						Stacked:  true,
						Build: func(c *ui.Context) {
							curVer := firstNonEmptyStr(s.AppVersion, "0.1.0-dev")
							commitStr := strings.TrimSpace(s.BuildCommit)
							ui.Column(c).FillWidth().Gap(float32(tok.Spacing8)).Children(func() {
								ui.Row(c).Wrap().Gap(float32(tok.Spacing8)).AlignItems(ui.Center).Children(func() {
									verLabel := "v" + strings.TrimPrefix(curVer, "v")
									if commitStr != "" && commitStr != "dev" {
										verLabel += " (" + commitStr + ")"
									}
									components.Badge(c, verLabel)
									switch s.UpdateStatus {
									case "checking":
										components.Text(c, "正在检查更新…").FontSize(float32(tok.TextData)).TextColor(tok.TextMuted)
									case "up_to_date":
										components.Text(c, "已是最新版本").FontSize(float32(tok.TextData)).TextColor(tok.Success)
									case "available":
										components.Badge(c, "发现新版本 "+s.UpdateLatestVersion)
									case "downloading":
										components.Text(c, "正在下载更新…").FontSize(float32(tok.TextData)).TextColor(tok.Accent)
									case "restarting":
										components.Text(c, "正在重启…").FontSize(float32(tok.TextData)).TextColor(tok.Accent)
									case "error":
										if s.UpdateErr != "" {
											components.Text(c, s.UpdateErr).FontSize(float32(tok.TextData)).TextColor(tok.Danger)
										}
									}
								})
								ui.Row(c).Wrap().Gap(float32(tok.Spacing8)).AlignItems(ui.Center).Children(func() {
									chkBtn := components.Button(c, "检查更新", components.ButtonSecondary)
									if s.UpdateStatus == "checking" || s.UpdateStatus == "downloading" || s.UpdateStatus == "restarting" {
										chkBtn.Disabled(true)
									}
									if chkBtn.Clicked() && s.OnCheckUpdate != nil {
										s.OnCheckUpdate()
									}
									if s.UpdateStatus == "available" && s.OnInstallUpdate != nil {
										if components.Button(c, "立即更新并重启", components.ButtonPrimary).Clicked() {
											s.OnInstallUpdate()
										}
									}
									if s.UpdateDownloadURL != "" && s.OnOpenUpdatePage != nil {
										if components.Button(c, "发布说明", components.ButtonGhost).Clicked() {
											s.OnOpenUpdatePage()
										}
									}
								})
								if strings.TrimSpace(s.UpdateNotes) != "" && s.UpdateStatus == "available" {
									ui.Box(c).FillWidth().Padding(float32(tok.Spacing8)).Radius(float32(tok.ControlRadius)).
										Background(tok.Elevated).Children(func() {
										ui.Text(c, s.UpdateNotes).FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
									})
								}
							})
						},
					},
					{
						ID:       "update-auto-check",
						Title:    "启动时检查更新",
						Keywords: []string{"auto update", "自动检查"},
						Build: func(c *ui.Context) {
							if components.Switch(c, &s.UpdateAutoCheck, "自动检查", components.SwitchOptions{HideLabel: true}).Changed() {
								s.notifySettingsChanged()
							}
						},
					},
					{
						ID:       "update-prerelease",
						Title:    "接收预发布版本",
						Keywords: []string{"prerelease", "beta", "rc", "预发布"},
						Build: func(c *ui.Context) {
							if components.Switch(c, &s.UpdateIncludePre, "预发布", components.SwitchOptions{HideLabel: true}).Changed() {
								s.notifySettingsChanged()
							}
						},
					},
				},
			},
			{
				ID:    "diag-feedback",
				Title: "诊断与反馈",
				Items: []components.SettingsItem{
					{
						ID:          "diag-actions",
						Title:       "诊断日志与问题反馈",
						Description: "仅含脱敏日志，不含实验内容",
						Keywords:    []string{"feedback", "bug", "log", "crash", "diagnostic", "反馈", "日志", "诊断"},
						Build: func(c *ui.Context) {
							ui.Row(c).Wrap().Gap(float32(tok.Spacing8)).AlignItems(ui.Center).Children(func() {
								if components.Button(c, "导出诊断包 (.zip)", components.ButtonSecondary).Clicked() && s.OnExportDiagBundle != nil {
									s.OnExportDiagBundle()
								}
								if components.Button(c, "反馈问题 / 建议", components.ButtonPrimary).Clicked() {
									s.FeedbackOpen = true
								}
								if components.Button(c, "UI 组件画廊", components.ButtonGhost).Clicked() {
									s.ComponentGalleryOpen = true
								}
							})
						},
					},
					{
						ID:       "stats-summary",
						Title:    "当前库统计",
						Keywords: []string{"stats", "count", "统计"},
						Build: func(c *ui.Context) {
							expCnt, tplCnt, assetCnt := 0, 0, 0
							for _, f := range s.VaultFiles {
								if f.IsTemplate {
									tplCnt++
								} else if f.IsAsset {
									assetCnt++
								} else {
									expCnt++
								}
							}
							components.Badge(c, fmt.Sprintf("%d 实验 · %d 模板 · %d 附件", expCnt, tplCnt, assetCnt))
						},
					},
				},
			},
		},
	}

	return []components.SettingsCategory{
		vaultCat,
		appearanceCat,
		editorCat,
		hotkeysCat,
		aboutCat,
	}
}
