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
	"asterlab-mygo-prototype/internal/uikit/kit"
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
	s.ensureSyncDefaults()
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

func (s *Shell) ensureSyncDefaults() {
	if !s.syncPrefsInited {
		s.syncPrefsInited = true
		if s.SyncSettings.ImagePolicy == "" && s.SyncSettings.ImageThresholdMB == 0 && s.SyncSettings.Endpoint == "" && !s.SyncSettings.Enabled {
			s.SyncSettings.WiFiOnlyOriginals = true
		}
		if s.SyncSettings.AutoIntervalMin == 0 && s.SyncSettings.Endpoint == "" && !s.SyncSettings.Enabled {
			s.SyncSettings.AutoIntervalMin = 5
		}
		if strings.TrimSpace(s.SyncSettings.RemotePath) == "" {
			s.SyncSettings.RemotePath = "/AsterLab"
		}
	}
	s.SyncSettings.ImagePolicy = NormalizeImagePolicy(s.SyncSettings.ImagePolicy)
	switch s.SyncSettings.ImageThresholdMB {
	case 1, 2, 5, 10:
	default:
		s.SyncSettings.ImageThresholdMB = 2
	}
}

func (s *Shell) saveSyncSettings() {
	s.ensureSyncDefaults()
	s.notifySettingsChanged()
	if s.OnSaveSyncSettings != nil {
		s.OnSaveSyncSettings()
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
	if s.IsMobileLayout(c) {
		extra := s.mobileExtra()
		if !s.VaultSettingsOpen {
			extra.mobileSettingsLastOpen = false
			extra.mobileSettingsDetail = false
			return
		}
		s.ensureSettingsDefaults()
		if !extra.mobileSettingsLastOpen {
			extra.mobileSettingsLastOpen = true
			extra.mobileSettingsDetail = strings.TrimSpace(s.SettingsView.Category) != ""
		}
		tok := theme.GetTokens(c.Theme())
		ui.Overlay(c, func() {
			s.renderMobileSettingsPage(c, tok, extra)
		})
		return
	}
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
	s.VaultSettingsOpen = open
}

func (s *Shell) renderMobileSettingsPage(c *ui.Context, tok theme.Tokens, extra *mobileExtraState) {
	ui.Overlay(c, func() {
		t := c.Theme()
		topSafe, rightSafe, bottomSafe, leftSafe := c.SafeArea()
	cats := s.obsidianSettingsCategories(c)
	query := strings.TrimSpace(s.SettingsView.Query)
	searching := query != ""

	selected := -1
	for i, cat := range cats {
		if cat.ID == s.SettingsView.Category && cat.Placeholder == nil {
			selected = i
			break
		}
	}
	if selected < 0 && len(cats) > 0 && extra.mobileSettingsDetail {
		for i, cat := range cats {
			if cat.Placeholder == nil {
				selected = i
				s.SettingsView.Category = cat.ID
				break
			}
		}
	}

	pageTitle := "设置"
	if extra.mobileSettingsDetail && !searching && selected >= 0 && selected < len(cats) {
		pageTitle = cats[selected].Title
	}

	onBack := func() {
		if extra.mobileSettingsDetail || searching {
			extra.mobileSettingsDetail = false
			s.SettingsView.Category = ""
			s.SettingsView.Query = ""
			return
		}
		s.VaultSettingsOpen = false
		extra.mobileSettingsLastOpen = false
		extra.mobileSettingsDetail = false
	}

	swipeKey := "mb-settings-root"
	if extra.mobileSettingsDetail || searching {
		swipeKey = "mb-settings-detail"
	}
	s.renderMobileSwipeBackContainer(c, swipeKey, tok, onBack, func() {
		// Obsidian Mobile Top Navigation Bar (edge-to-edge full-screen page)
		ui.Row(c).FillWidth().
			Padding(t.Space(2)+topSafe, t.Space(4)+rightSafe, t.Space(2), t.Space(4)+leftSafe).
			MinHeight(t.Space(12)+topSafe).
			AlignItems(ui.Center).Gap(t.Space(2)).Background(tok.Surface).Children(func() {
			if extra.mobileSettingsDetail || searching {
				backBtn := ui.ButtonBase(c.Key("mb-set-back")).Label("返回设置").
					Padding(t.Space(1), t.Space(2)).Radius(float32(tok.ControlRadius)).
					Background(ui.Transparent).MinHeight(t.Space(9)).Center().
					OnClick(func() {
						mygo.Haptics.Selection()
						extra.mobileSettingsDetail = false
						s.SettingsView.Category = ""
						s.SettingsView.Query = ""
					})
				backBtn.Children(func() {
					ui.Row(c).AlignItems(ui.Center).Gap(t.Space(1)).Children(func() {
						ui.Icon(c, lucide.ChevronLeft()).Size(float32(tok.IconMd), float32(tok.IconMd)).TextColor(tok.Accent)
						ui.Text(c, "设置").FontSize(float32(tok.TextBody)).TextColor(tok.Accent).SingleLine()
					})
				})
			} else {
				ui.Icon(c, lucide.Settings()).Size(float32(tok.IconMd), float32(tok.IconMd)).TextColor(tok.Accent)
			}

			components.Text(c, pageTitle).FontSize(float32(tok.TextSection)).Bold().TextColor(tok.Text).
				Grow(1).MinWidth(0).SingleLine().Ellipsis("…")

			doneBtn := components.Button(c.Key("mb-set-close"), "完成", components.ButtonGhost).Label("关闭设置").Shrink(0)
			if doneBtn.Clicked() {
				s.VaultSettingsOpen = false
				extra.mobileSettingsLastOpen = false
				extra.mobileSettingsDetail = false
			}
		})

		// Search field on Level 1 Home or while actively filtering
		if !extra.mobileSettingsDetail || searching {
			ui.Column(c).FillWidth().
				Padding(t.Space(2), t.Space(4)+rightSafe, t.Space(3), t.Space(4)+leftSafe).
				Background(tok.Surface).Children(func() {
				components.TextBox(c.Key("mb-set-search"), &s.SettingsView.Query, components.TextBoxOptions{
					LeadingIcon: lucide.Search(),
				}).Placeholder("搜索设置…").FillWidth()
			})
		}
		ui.Box(c).FillWidth().Height(1).Background(tok.BorderSubtle)

		if searching {
			s.renderMobileSettingsResultsList(c, tok, cats, selected, true, topSafe, rightSafe, bottomSafe, leftSafe)
			return
		}

		if extra.mobileSettingsDetail && selected >= 0 && selected < len(cats) {
			s.renderMobileSettingsResultsList(c, tok, cats, selected, false, topSafe, rightSafe, bottomSafe, leftSafe)
			return
		}

		// Level 1: Obsidian Mobile Grouped Settings Home
		ui.Scroll(c.Key("mb-set-home-scroll")).FillWidth().Grow(1).Basis(0).MinHeight(0).Clip().Children(func() {
			ui.Column(c).FillWidth().
				Padding(t.Space(4), t.Space(4)+rightSafe, t.Space(8)+bottomSafe, t.Space(4)+leftSafe).
				Gap(t.Space(5)).Children(func() {
				var workspaceCats, systemCats []components.SettingsCategory
				for _, cat := range cats {
					if cat.Placeholder != nil {
						continue
					}
					if cat.ID == "about" {
						systemCats = append(systemCats, cat)
					} else {
						workspaceCats = append(workspaceCats, cat)
					}
				}

				renderGroup := func(groupTitle string, list []components.SettingsCategory) {
					if len(list) == 0 {
						return
					}
					ui.Column(c).FillWidth().Gap(t.Space(2)).Children(func() {
						components.Text(c, groupTitle).FontSize(float32(tok.TextMeta)).Bold().TextColor(tok.TextMuted).
							Padding(0, t.Space(1))
						ui.Column(c).FillWidth().Radius(float32(tok.PanelRadius)).
							Background(tok.Surface).Border(1, tok.Border).Clip().Children(func() {
							for i, cat := range list {
								cat := cat
								if i > 0 {
									ui.Box(c).FillWidth().Height(1).Background(tok.BorderSubtle)
								}
								row := ui.ButtonBase(c.Key("mb-set-cat-"+cat.ID)).Label(cat.Title).
									FillWidth().MinHeight(float32(tok.Spacing48)+t.Space(1)).
									Padding(t.Space(3), t.Space(4)).
									Background(tok.Surface).Justify(ui.Start).
									OnClick(func() {
										mygo.Haptics.Selection()
										s.SettingsView.Category = cat.ID
										s.SettingsView.Query = ""
										extra.mobileSettingsDetail = true
									})
								theme.ApplyTactilePress(row)
								row.Children(func() {
									ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(t.Space(3)).Children(func() {
										if cat.Icon != nil {
											kit.IconBox(c, tok, cat.Icon, tok.Accent)
										}
										ui.Text(c, cat.Title).FontSize(float32(tok.TextBody)).Bold().TextColor(tok.Text).
											Grow(1).MinWidth(0).SingleLine().Ellipsis("…")
										if hint := s.mobileSettingsCategoryHint(cat.ID); hint != "" {
											ui.Text(c, hint).FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted).
												SingleLine().Ellipsis("…")
										}
										ui.Icon(c, lucide.ChevronRight()).Size(float32(tok.IconSm), float32(tok.IconSm)).TextColor(tok.TextMuted)
									})
								})
							}
						})
					})
				}

				renderGroup("工作区", workspaceCats)
				renderGroup("系统", systemCats)
			})
		})
	})
	})
}

func (s *Shell) mobileSettingsCategoryHint(catID string) string {
	switch catID {
	case "files":
		p := strings.TrimRight(strings.TrimSpace(s.VaultPathInput), `/\`)
		if p == "" {
			return ""
		}
		if idx := strings.LastIndexAny(p, `/\`); idx >= 0 && idx+1 < len(p) {
			return p[idx+1:]
		}
		return p
	case "sync":
		if strings.TrimSpace(s.SyncSettings.Endpoint) != "" {
			if st := strings.TrimSpace(s.SyncState.LastSyncText); st != "" {
				return st
			}
			return "WebDAV"
		}
		return "未配置"
	case "appearance":
		switch s.ThemeSource {
		case "dark":
			return "深色"
		case "light":
			return "浅色"
		default:
			return "跟随系统"
		}
	case "about":
		return "AsterLab"
	default:
		return ""
	}
}

func (s *Shell) renderMobileSettingsResultsList(
	c *ui.Context,
	tok theme.Tokens,
	cats []components.SettingsCategory,
	selected int,
	searching bool,
	topSafe, rightSafe, bottomSafe, leftSafe float32,
) {
	t := c.Theme()
	_ = topSafe
	results := components.SearchSettings(cats, s.SettingsView.Query)
	scrollKey := "mb-set-scroll-search"
	if !searching && selected >= 0 && selected < len(cats) {
		scrollKey = "mb-set-scroll-" + cats[selected].ID
	}
	ui.Scroll(c.Key(scrollKey)).FillWidth().Grow(1).Basis(0).MinHeight(0).Clip().Children(func() {
		ui.Column(c).FillWidth().
			Padding(t.Space(3), t.Space(4)+rightSafe, t.Space(8)+bottomSafe, t.Space(4)+leftSafe).
			Gap(t.Space(3)).Children(func() {
			count, lastCat, lastGroup := 0, -1, -1
			for _, res := range results {
				if !searching && res.Category != selected {
					continue
				}
				cat := cats[res.Category]
				grp := cat.Groups[res.Group]
				it := grp.Items[res.Item]
				if lastCat != res.Category && searching {
					components.Text(c, cat.Title).FontSize(float32(tok.TextSection)).Bold().TextColor(tok.Text)
				}
				if (lastCat != res.Category || lastGroup != res.Group) && (grp.Title != "" || grp.Actions != nil) {
					ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(t.Space(2)).Padding(t.Space(1), 0, 0, 0).Children(func() {
						components.Text(c, grp.Title).FontSize(float32(tok.TextMeta)).Bold().TextColor(tok.TextMuted).Grow(1).MinWidth(0)
						if grp.Actions != nil {
							grp.Actions(c)
						}
					})
				}
				lastCat, lastGroup = res.Category, res.Group
				count++
				itemCard := ui.Column(c.Key("mb-set-it-"+cat.ID+"-"+grp.ID+"-"+it.ID)).FillWidth().Gap(t.Space(2))
				if it.ID != "mobile-toolbar-customize" {
					itemCard.Padding(t.Space(3)).Radius(float32(tok.PanelRadius)).
						Background(tok.Elevated).Border(1, tok.Border)
				}
				itemCard.Children(func() {
					if it.Content != nil {
						it.Content(c)
						return
					}
					if it.Title != "" || it.Description != "" || it.DescriptionContent != nil {
						ui.Column(c).FillWidth().Gap(t.Space(1)).Children(func() {
							if it.Title != "" {
								components.Text(c, it.Title).FontSize(float32(tok.TextBody)).Bold().TextColor(tok.Text)
							}
							switch {
							case it.DescriptionContent != nil:
								ui.RichText(c).FillWidth().TextColor(tok.TextMuted).FontSize(float32(tok.TextMeta)).Wrap().Children(func() {
									it.DescriptionContent(c)
								})
							case it.Description != "":
								components.Text(c, it.Description).FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
							}
						})
					}
					if it.Build != nil {
						it.Build(c)
					}
				})
			}
			if count == 0 {
				kit.EmptyState(c, tok, lucide.Search(), "无匹配设置项", "尝试更换关键词或清空搜索词查看全部设置。", "清空搜索", func() {
					s.SettingsView.Query = ""
				})
			}
		})
	})
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
							if s.IsMobileLayout(c) {
								t := c.Theme()
								ui.Column(c).FillWidth().Gap(t.Space(2)).Children(func() {
									components.TextBox(c, &s.VaultPathInput, components.TextBoxOptions{}).
										Placeholder("选择本地实验库目录").FillWidth()
									ui.Row(c).FillWidth().Wrap().Gap(t.Space(2)).AlignItems(ui.Center).Children(func() {
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
								})
								return
							}
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
							if s.IsMobileLayout(c) {
								sel.Trigger.FillWidth().Children(func() {
									components.Text(c, labelFor(s.DefaultProjectDir)).SingleLine().Ellipsis("…")
								})
							} else {
								sel.Trigger.MinWidth(c.Theme().Space(36)).Shrink(0).Children(func() {
									components.Text(c, labelFor(s.DefaultProjectDir)).SingleLine().Ellipsis("…")
								})
							}
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
							isMobile := s.IsMobileLayout(c)
							rowPad := float32(tok.Spacing4)
							rowBg := tok.Elevated
							if isMobile {
								rowPad = float32(tok.Spacing8)
								rowBg = tok.Surface
							}
							ui.Column(c).FillWidth().Gap(float32(tok.Spacing4)).Children(func() {
								limit := len(s.TrashItems)
								if limit > 8 {
									limit = 8
								}
								for i := 0; i < limit; i++ {
									it := s.TrashItems[i]
									ui.Row(c.Key("trash-it-"+it.TrashName)).FillWidth().AlignItems(ui.Center).
										Padding(rowPad, float32(tok.Spacing8)).
										Radius(float32(tok.ControlRadius)).Background(rowBg).Gap(float32(tok.Spacing8)).Children(func() {
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
							isMobile := s.IsMobileLayout(c)
							rowPad := float32(tok.Spacing4)
							rowBg := tok.Elevated
							if isMobile {
								rowPad = float32(tok.Spacing8)
								rowBg = tok.Surface
							}
							ui.Column(c).FillWidth().Gap(float32(tok.Spacing4)).Children(func() {
								limit := len(s.CurrentSnapshots)
								if limit > 8 {
									limit = 8
								}
								for i := 0; i < limit; i++ {
									sn := s.CurrentSnapshots[i]
									ui.Row(c.Key("snap-"+sn.ID)).FillWidth().AlignItems(ui.Center).
										Padding(rowPad, float32(tok.Spacing8)).
										Radius(float32(tok.ControlRadius)).Background(rowBg).Gap(float32(tok.Spacing8)).Children(func() {
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
					{
						ID:          "vault-backup-zip",
						Title:       "整库备份与恢复",
						Description: "导出包含全部实验、附件与库存台账的 .zip 备份，或从备份压缩包恢复整库",
						Keywords:    []string{"backup", "restore", "zip", "备份", "恢复", "整库", "导出"},
						Stacked:     true,
						Build: func(c *ui.Context) {
							ui.Row(c).Wrap().AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
								if components.Button(c, "导出整库 .zip 备份", components.ButtonSecondary).Clicked() && s.OnVaultBackupZip != nil {
									s.OnVaultBackupZip()
								}
								if components.Button(c, "从 .zip 恢复整库", components.ButtonOutline).Clicked() && s.OnVaultRestoreZip != nil {
									s.OnVaultRestoreZip()
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
						mygo.Haptics.Selection()
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
			components.SettingsItem{
				ID:       "mobile-toolbar-customize",
				Title:    "自定义移动端编辑工具栏",
				Keywords: []string{"toolbar", "customize", "order", "visible", "工具栏", "排序", "显隐", "移动端"},
				Stacked:  true,
				Build: func(c *ui.Context) {
					specs := s.MobileToolbarOrderedSpecs()
					ui.Column(c).FillWidth().Gap(float32(tok.Spacing8)).Children(func() {
						ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
							components.Text(c, fmt.Sprintf("共 %d 项", len(specs))).
								FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted).FontFeatures("tnum").Grow(1).MinWidth(0)
							if components.Button(c, "恢复默认工具栏", components.ButtonGhost).Clicked() {
								mygo.Haptics.Selection()
								s.ResetMobileToolbarConfig()
							}
						})
						ui.Column(c).FillWidth().Radius(float32(tok.PanelRadius)).
							Background(tok.Surface).Border(1, tok.Border).Clip().Children(func() {
							isMobile := s.IsMobileLayout(c)
							for idx, spec := range specs {
								idx := idx
								spec := spec
								vis := s.IsMobileToolbarActionVisible(spec.ID)
								if idx > 0 {
									ui.Box(c).FillWidth().Height(1).Background(tok.BorderSubtle)
								}
								ui.Row(c.Key("mb-tb-cfg-"+spec.ID)).FillWidth().AlignItems(ui.Center).
									Padding(float32(tok.Spacing8), float32(tok.Spacing12)).
									Background(tok.Surface).
									Gap(float32(tok.Spacing8)).Children(func() {
									kit.IconBox(c, tok, spec.Icon, tok.Text)
									if isMobile {
										ui.Column(c).Grow(1).MinWidth(0).Gap(1).Children(func() {
											components.Text(c, spec.Label).FontSize(float32(tok.TextData)).Bold().TextColor(tok.Text).
												SingleLine().Ellipsis("…")
											components.Text(c, spec.Group).FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted).
												SingleLine()
										})
									} else {
										components.Text(c, spec.Label).FontSize(float32(tok.TextData)).TextColor(tok.Text).
											SingleLine().Ellipsis("…").Grow(1).MinWidth(0)
										components.Badge(c, spec.Group, components.BadgeOptions{Tone: components.BadgeNeutral})
									}
									upBtn := components.IconButton(c.Key("mb-tb-up-"+spec.ID), lucide.ChevronUp(), "上移 "+spec.Label, components.IconButtonCompact).
										Disabled(idx == 0)
									if upBtn.Clicked() {
										mygo.Haptics.Selection()
										s.MoveMobileToolbarAction(spec.ID, -1)
									}
									dnBtn := components.IconButton(c.Key("mb-tb-dn-"+spec.ID), lucide.ChevronDown(), "下移 "+spec.Label, components.IconButtonCompact).
										Disabled(idx == len(specs)-1)
									if dnBtn.Clicked() {
										mygo.Haptics.Selection()
										s.MoveMobileToolbarAction(spec.ID, 1)
									}
									if components.Switch(c.Key("mb-tb-vis-"+spec.ID), &vis, "显示 "+spec.Label, components.SwitchOptions{HideLabel: true}).Changed() {
										mygo.Haptics.Selection()
										s.SetMobileToolbarActionVisible(spec.ID, vis)
									}
								})
							}
						})
					})
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
							sel.Trigger.MinWidth(c.Theme().Space(32)).Shrink(0).Children(func() {
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
							sel.Trigger.MinWidth(c.Theme().Space(32)).Shrink(0).Children(func() {
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
							sel.Trigger.MinWidth(c.Theme().Space(32)).Shrink(0).Children(func() {
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
							sel.Trigger.MinWidth(c.Theme().Space(32)).Shrink(0).Children(func() {
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
							// Keep Shrink(0) on container and buttons so right-hand segmented pills never clip text (FB-103901).
							ui.Row(c).Shrink(0).AlignItems(ui.Center).Padding(float32(tok.Spacing4)*0.5).Radius(float32(tok.ControlRadius)).
								Background(tok.Elevated).Border(1, tok.Border).Gap(2).Children(func() {
								for _, item := range []struct{ key, lbl string }{{"edit", "编辑态"}, {"read", "阅读态"}} {
									item := item
									active := s.DefaultOpenMode == item.key || (s.DefaultOpenMode == "" && item.key == "edit")
									targetBg, fg := ui.Transparent, tok.TextMuted
									if active {
										targetBg, fg = tok.Surface, tok.Text
									}
									btn := ui.ButtonBase(c.Key("set-open-mode-"+item.key)).Label(item.lbl).
										Shrink(0).
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
							sel.Trigger.MinWidth(c.Theme().Space(28)).Shrink(0).Children(func() {
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
							sel.Trigger.MinWidth(c.Theme().Space(28)).Shrink(0).Children(func() {
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
							if s.IsMobileLayout(c) {
								ui.Box(c).FillWidth().Children(func() {
									components.TextBox(c, &cur, components.TextBoxOptions{}).Placeholder(spec.Default).FillWidth()
								})
								if strings.TrimSpace(cur) != s.Hotkeys[spec.ID] && strings.TrimSpace(cur) != "" {
									s.Hotkeys[spec.ID] = strings.TrimSpace(cur)
									s.notifySettingsChanged()
								}
								return
							}
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
						ID:          "feedback-mode",
						Title:       "可视化截图标注反馈模式",
						Description: "开启右下角悬浮反馈条，随时红框截图批注并一键直传开发者（Ctrl+Shift+D）",
						Keywords:    []string{"feedback", "beta", "反馈模式", "截图标注", "红框"},
						Build: func(c *ui.Context) {
							if components.Switch(c, &s.FeedbackMode, "反馈模式", components.SwitchOptions{HideLabel: true}).Changed() {
								s.notifySettingsChanged()
							}
						},
					},
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

	syncCat := s.buildSyncSettingsCategory(c, tok)

	return []components.SettingsCategory{
		vaultCat,
		syncCat,
		appearanceCat,
		editorCat,
		hotkeysCat,
		aboutCat,
	}
}

func (s *Shell) buildSyncSettingsCategory(c *ui.Context, tok theme.Tokens) components.SettingsCategory {
	intervalLabels := map[int]string{
		0:  "仅手动",
		1:  "1 分钟",
		5:  "5 分钟",
		15: "15 分钟",
		30: "30 分钟",
	}
	intervalLabel := intervalLabels[s.SyncSettings.AutoIntervalMin]
	if intervalLabel == "" {
		intervalLabel = fmt.Sprintf("%d 分钟", s.SyncSettings.AutoIntervalMin)
	}

	policyMode := NormalizeImagePolicy(s.SyncSettings.ImagePolicy)
	renderPillChoice := func(keyID, label string, active bool, onClick func()) {
		bg := tok.Elevated
		fg := tok.TextMuted
		bd := tok.Border
		if active {
			bg = tok.AccentSoftBg
			fg = tok.Text
			bd = tok.BorderSelected
		}
		btn := ui.ButtonBase(c.Key(keyID)).
			Label(label).
			Shrink(0).
			Padding(float32(tok.Spacing4), float32(tok.Spacing12)).
			Radius(float32(tok.ControlRadius)).
			Background(bg).
			Border(1, bd)
		theme.ApplyTactilePress(btn)
		btn.Children(func() {
			txt := ui.Text(c, label).FontSize(float32(tok.TextMeta)).TextColor(fg).SingleLine()
			if active {
				txt.Bold()
			}
		})
		if btn.Clicked() {
			onClick()
		}
	}

	groups := []components.SettingsGroup{
		{
			ID:    "sync-webdav",
			Title: "WebDAV 多端同步",
			Items: []components.SettingsItem{
				{
					ID:          "sync-enabled",
					Title:       "启用多端同步",
					Description: "基于单文件清单索引增量同步，避免目录遍历触发限流",
					Keywords:    []string{"sync", "webdav", "同步", "启用", "多端"},
					Build: func(c *ui.Context) {
						if components.Switch(c, &s.SyncSettings.Enabled, "启用多端同步", components.SwitchOptions{HideLabel: true}).Changed() {
							s.saveSyncSettings()
						}
					},
				},
				{
					ID:       "sync-status-summary",
					Title:    "同步状态",
					Keywords: []string{"sync", "status", "状态", "同步", "待同步"},
					Stacked:  true,
					Build: func(c *ui.Context) {
						ui.Column(c).FillWidth().Gap(float32(tok.Spacing8)).Children(func() {
							ui.Row(c).Wrap().AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
								badgeText := "未启用"
								tone := components.BadgeNeutral
								switch {
								case len(s.SyncConflicts) > 0:
									badgeText = fmt.Sprintf("存在 %d 项冲突", len(s.SyncConflicts))
									tone = components.BadgeWarning
								case s.SyncState.Status == "error":
									badgeText = "同步异常"
									tone = components.BadgeDanger
								case s.SyncState.Status == "syncing":
									badgeText = "同步中…"
									tone = components.BadgeAccent
								case s.SyncState.Status == "synced":
									badgeText = "已同步"
									tone = components.BadgeSuccess
								case s.SyncSettings.Enabled:
									badgeText = "已就绪"
									tone = components.BadgeSuccess
								}
								kit.Badge(c, badgeText, components.BadgeOptions{Tone: tone})
								if strings.TrimSpace(s.SyncState.LastSyncText) != "" {
									components.Text(c, "上次同步: "+s.SyncState.LastSyncText).
										FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted).FontFeatures("tnum")
								}
								if s.SyncState.PendingCount > 0 {
									kit.Badge(c, fmt.Sprintf("待同步 %d 项", s.SyncState.PendingCount), components.BadgeOptions{Tone: components.BadgeWarning})
								}
							})
							if strings.TrimSpace(s.SyncState.ErrorText) != "" {
								kit.InlineBanner(c, tok, lucide.CircleAlert(), kit.BannerDanger, s.SyncState.ErrorText, "重试", func() {
									s.saveSyncSettings()
									if s.OnTriggerSyncNow != nil {
										s.OnTriggerSyncNow()
									}
								})
							}
						})
					},
				},
				{
					ID:          "sync-endpoint",
					Title:       "服务器地址 (WebDAV)",
					Description: "支持坚果云、群晖 NAS、Nextcloud 与 Alist；识别坚果云后自动启用防限流保护",
					Keywords:    []string{"webdav", "url", "endpoint", "nutstore", "坚果云", "群晖", "nextcloud"},
					Stacked:     true,
					Build: func(c *ui.Context) {
						ui.Column(c).FillWidth().Gap(float32(tok.Spacing8)).Children(func() {
							prev := s.SyncSettings.Endpoint
							ui.Box(c.Key("sync-endpoint-box")).FillWidth().Children(func() {
								components.TextBox(c, &s.SyncSettings.Endpoint, components.TextBoxOptions{LeadingIcon: lucide.Cloud()}).
									Label("WebDAV 服务器地址").
									Placeholder("https://dav.jianguoyun.com/dav/")
							})
							if s.SyncSettings.Endpoint != prev {
								s.saveSyncSettings()
							}
							if s.SyncSettings.IsNutstore() {
								ui.Row(c).Wrap().AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
									kit.Badge(c, "坚果云自动限流保护 · 单线程并发", components.BadgeOptions{
										Tone: components.BadgeAccent,
										Icon: lucide.ShieldCheck(),
									})
								})
							}
						})
					},
				},
				{
					ID:       "sync-username",
					Title:    "用户名",
					Keywords: []string{"username", "account", "账号", "用户名"},
					Stacked:  true,
					Build: func(c *ui.Context) {
						prev := s.SyncSettings.Username
						ui.Box(c.Key("sync-username-box")).FillWidth().Children(func() {
							components.TextBox(c, &s.SyncSettings.Username, components.TextBoxOptions{}).
								Label("WebDAV 用户名").
								Placeholder("账号邮箱或用户名")
						})
						if s.SyncSettings.Username != prev {
							s.saveSyncSettings()
						}
					},
				},
				{
					ID:       "sync-password",
					Title:    "应用密码",
					Keywords: []string{"password", "secret", "token", "密码", "应用密码"},
					Stacked:  true,
					Build: func(c *ui.Context) {
						prev := s.SyncSettings.Password
						ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
							ui.Box(c.Key("sync-password-box")).Grow(1).MinWidth(0).Children(func() {
								tb := components.TextBox(c, &s.SyncSettings.Password, components.TextBoxOptions{LeadingIcon: lucide.Lock()}).
									Label("WebDAV 应用密码").
									Placeholder("输入 WebDAV 专用应用密码")
								if !s.syncPasswordVisible {
									tb.Password()
								}
							})
							eyeIcon := lucide.Eye()
							eyeLabel := "显示密码"
							if s.syncPasswordVisible {
								eyeIcon = lucide.EyeOff()
								eyeLabel = "隐藏密码"
							}
							if components.IconButton(c.Key("sync-password-eye"), eyeIcon, eyeLabel, components.IconButtonCompact).Clicked() {
								s.syncPasswordVisible = !s.syncPasswordVisible
							}
						})
						if s.SyncSettings.Password != prev {
							s.saveSyncSettings()
						}
					},
				},
				{
					ID:       "sync-remote-path",
					Title:    "远端目录路径",
					Keywords: []string{"remote", "path", "目录", "远端"},
					Stacked:  true,
					Build: func(c *ui.Context) {
						prev := s.SyncSettings.RemotePath
						ui.Box(c.Key("sync-remote-path-box")).FillWidth().Children(func() {
							components.TextBox(c, &s.SyncSettings.RemotePath, components.TextBoxOptions{}).
								Label("远端同步目录").
								Placeholder("/AsterLab")
						})
						if s.SyncSettings.RemotePath != prev {
							s.saveSyncSettings()
						}
					},
				},
				{
					ID:       "sync-interval",
					Title:    "自动同步间隔",
					Keywords: []string{"interval", "auto", "同步间隔", "频率"},
					Build: func(c *ui.Context) {
						opts := []int{0, 1, 5, 15, 30}
						sel := components.Select(c, &s.SyncSettings.AutoIntervalMin)
						sel.Trigger.MinWidth(c.Theme().Space(32)).Shrink(0).Children(func() {
							components.Text(c, intervalLabel).SingleLine()
						})
						sel.Popup(func(ui.Element) {
							for _, opt := range opts {
								opt := opt
								sel.Item(opt).Children(func() {
									components.Text(c, intervalLabels[opt]).SingleLine()
								})
							}
						})
						if sel.Trigger.Changed() {
							s.saveSyncSettings()
						}
					},
				},
				{
					ID:       "sync-actions",
					Title:    "连接测试与手动同步",
					Keywords: []string{"test", "sync", "测试连接", "立即同步"},
					Stacked:  true,
					Build: func(c *ui.Context) {
						ui.Row(c).Wrap().AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
							if components.Button(c, "测试连接", components.ButtonOutline).Clicked() {
								s.saveSyncSettings()
								if s.OnTestSyncConnection != nil {
									s.OnTestSyncConnection()
								}
							}
							syncNowBtn := components.Button(c, "立即同步", components.ButtonPrimary)
							if s.SyncState.Status == "syncing" {
								syncNowBtn.Disabled(true)
							}
							if syncNowBtn.Clicked() {
								s.saveSyncSettings()
								if s.OnTriggerSyncNow != nil {
									s.OnTriggerSyncNow()
								}
							}
							if components.Button(c, "保存设置", components.ButtonSecondary).Clicked() {
								s.saveSyncSettings()
							}
						})
					},
				},
			},
		},
		{
			ID:    "sync-image-policy",
			Title: "科学图像双轨同步策略",
			Items: []components.SettingsItem{
				{
					ID:       "sync-image-policy-banner",
					Title:    "科学原图零损原则",
					Keywords: []string{"image", "original", "preview", "科学原图", "预览图", "双轨"},
					Content: func(c *ui.Context) {
						kit.InlineBanner(c, tok, lucide.Images(), kit.BannerInfo, "上传端始终保留 100% 科学原图，手机端按阈值拉取预览图", "", nil)
					},
				},
				{
					ID:          "sync-image-policy-mode",
					Title:       "手机端图像拉取策略",
					Description: "控制远端高清显微/电泳原图同步到移动端时的本地拉取方式",
					Keywords:    []string{"preview_only", "threshold", "always_original", "仅同步预览图", "阈值内拉原图", "始终拉原图"},
					Stacked:     true,
					Build: func(c *ui.Context) {
						ui.Column(c).FillWidth().Gap(float32(tok.Spacing8)).Children(func() {
							ui.Row(c).Wrap().AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
								for _, opt := range []struct {
									key   string
									label string
								}{
									{SyncImagePolicyPreviewOnly, "仅同步预览图"},
									{SyncImagePolicyThreshold, "阈值内拉原图 (1/2/5/10 MB)"},
									{SyncImagePolicyAlwaysOriginal, "始终拉原图"},
								} {
									opt := opt
									renderPillChoice("sync-pol-"+opt.key, opt.label, policyMode == opt.key, func() {
										s.SyncSettings.ImagePolicy = opt.key
										s.saveSyncSettings()
									})
								}
							})
							if policyMode == SyncImagePolicyThreshold {
								ui.Row(c).Wrap().AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
									components.Text(c, "自动拉取原图阈值:").
										FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted).Shrink(0)
									for _, mb := range []int{1, 2, 5, 10} {
										mb := mb
										lbl := fmt.Sprintf("%d MB", mb)
										renderPillChoice(fmt.Sprintf("sync-thresh-%d", mb), lbl, s.SyncSettings.ImageThresholdMB == mb, func() {
											s.SyncSettings.ImageThresholdMB = mb
											s.saveSyncSettings()
										})
									}
								})
							}
						})
					},
				},
				{
					ID:          "sync-wifi-only-originals",
					Title:       "仅 Wi-Fi 自动拉原图",
					Description: "蜂窝移动网络下仅拉取轻量预览图，避免流量消耗",
					Keywords:    []string{"wifi", "cellular", "流量", "原图"},
					Build: func(c *ui.Context) {
						if components.Switch(c, &s.SyncSettings.WiFiOnlyOriginals, "切换仅 Wi-Fi 自动拉原图", components.SwitchOptions{HideLabel: true}).Changed() {
							s.saveSyncSettings()
						}
					},
				},
				{
					ID:          "sync-originals-cache",
					Title:       "本地原图缓存空间",
					Description: "释放仅删除本机缓存的高清原图，远端与上传端 100% 原图不受影响",
					Keywords:    []string{"cache", "clear", "释放", "缓存", "原图"},
					Stacked:     true,
					Build: func(c *ui.Context) {
						label := fmt.Sprintf("本地原图缓存 %.1f MB · 一键释放", s.SyncState.LocalOriginalsCacheMB)
						if components.Button(c, label, components.ButtonSecondary).Clicked() && s.OnClearOriginalsCache != nil {
							s.OnClearOriginalsCache()
						}
					},
				},
			},
		},
	}

	if len(s.SyncConflicts) > 0 {
		conflictItems := make([]components.SettingsItem, 0, len(s.SyncConflicts))
		for idx, cf := range s.SyncConflicts {
			idx := idx
			cf := cf
			conflictItems = append(conflictItems, components.SettingsItem{
				ID:       fmt.Sprintf("sync-conflict-%d", idx),
				Title:    cf.RelPath,
				Keywords: []string{"conflict", "冲突", cf.RelPath, cf.ConflictPath},
				Stacked:  true,
				Build: func(c *ui.Context) {
					ui.Column(c.Key(fmt.Sprintf("sync-conflict-row-%d", idx))).FillWidth().
						Padding(float32(tok.Spacing8), float32(tok.Spacing12)).
						Radius(float32(tok.ControlRadius)).
						Background(tok.WarningBg).
						Border(1, tok.Warning).
						Gap(float32(tok.Spacing8)).Children(func() {
						ui.Column(c).FillWidth().Gap(float32(tok.Spacing4)).Children(func() {
							components.Text(c, "冲突副本: "+cf.ConflictPath).
								FontSize(float32(tok.TextMeta)).TextColor(tok.WarningText).SingleLine().Ellipsis("…")
							if strings.TrimSpace(cf.DetectedAt) != "" {
								components.Text(c, "检测时间: "+cf.DetectedAt).
									FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted).FontFeatures("tnum")
							}
						})
						ui.Row(c).Wrap().AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
							if components.Button(c, "保留本地", components.ButtonSecondary).Clicked() && s.OnResolveSyncConflict != nil {
								s.OnResolveSyncConflict(cf.RelPath, cf.ConflictPath, "keep_local")
							}
							if components.Button(c, "采用远端", components.ButtonOutline).Clicked() && s.OnResolveSyncConflict != nil {
								s.OnResolveSyncConflict(cf.RelPath, cf.ConflictPath, "use_remote")
							}
							if components.Button(c, "打开副本", components.ButtonGhost).Clicked() && s.OnResolveSyncConflict != nil {
								s.OnResolveSyncConflict(cf.RelPath, cf.ConflictPath, "open_copy")
							}
						})
					})
				},
			})
		}
		groups = append(groups, components.SettingsGroup{
			ID:    "sync-conflicts",
			Title: "冲突副本处理",
			Items: conflictItems,
		})
	}

	return components.SettingsCategory{
		ID:      "sync",
		Title:   "多端同步",
		Section: "工作区",
		Icon:    lucide.CloudSync(),
		Groups:  groups,
	}
}
