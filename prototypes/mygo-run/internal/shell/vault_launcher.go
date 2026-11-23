package shell

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/onevra/source/components"
	"github.com/oneo-me/onevra/source/icons/lucide"

	"asterlab-mygo-prototype/internal/theme"
)

// RecentVaultEntry represents one remembered local Vault directory shown in the
// Obsidian-style Vault Launcher & Switcher.
type RecentVaultEntry struct {
	Name    string
	Path    string
	Active  bool
	Sandbox bool
}

// renderVaultSwitcherFooter renders the Obsidian-style bottom-left Vault switcher bar
// at the foot of the file tree pane so users always know where their .asterml files
// and .asterlab/pantry.jsonl live and can switch or create Vaults in one click.
func (s *Shell) renderVaultSwitcherFooter(c *ui.Context, tok theme.Tokens) {
	ui.Divider(c)
	curPath := strings.TrimSpace(s.VaultPathInput)
	curName := filepath.Base(curPath)
	if curName == "" || curName == "." || curName == string(filepath.Separator) {
		curName = "实验库"
	}
	if s.IsTempSandboxVault {
		curName = "临时体验库"
	}

	rowBtn := ui.ButtonBase(c.Key("vault-switcher-footer")).
		Label("切换实验库 " + curName).
		FillWidth().
		Padding(float32(tok.Spacing8), float32(tok.Spacing8)).
		Justify(ui.Start).
		OnClick(func() {
			s.openVaultLauncher()
		})
	targetBg := ui.Transparent
	if rowBtn.Hovered() {
		targetBg = tok.Elevated
	}
	rowBtn.Background(rowBtn.AnimateColor("vsf-bg", targetBg, theme.MotionSnappy))
	rowBtn.Children(func() {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
			iconCol := tok.TextMuted
			if s.IsTempSandboxVault {
				iconCol = tok.Warning
			}
			ui.Icon(c, lucide.Database()).Size(float32(tok.Spacing16), float32(tok.Spacing16)).TextColor(iconCol).Shrink(0)
			ui.Column(c).Grow(1).MinWidth(0).Gap(1).Children(func() {
				ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(float32(tok.Spacing4)).Children(func() {
					components.Text(c, curName).FontSize(float32(tok.TextMeta)).Bold().TextColor(tok.Text).
						SingleLine().Ellipsis("…").Grow(1).MinWidth(0)
					if s.IsTempSandboxVault {
						components.Badge(c, "临时", components.BadgeOptions{Tone: components.BadgeWarning})
					}
				})
				if curPath != "" {
					components.Text(c, curPath).FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted).
						SingleLine().Ellipsis("…").MinWidth(0)
				}
			})
			ui.Icon(c, lucide.ChevronsUpDown()).Size(float32(tok.Spacing16), float32(tok.Spacing16)).TextColor(tok.TextMuted).Shrink(0)
		})
	})

	tip := "点击切换 / 新建本地实验库"
	if curPath != "" {
		tip = "当前实验库：" + curPath + "（点击切换或新建）"
	}
	components.Tooltip(c, rowBtn, tip)

	components.ContextMenu(c, rowBtn, func(m *components.Menu) {
		if m.Item("切换 / 管理本地实验库…").Chosen() {
			s.openVaultLauncher()
		}
		if s.OnRevealInExplorer != nil && m.Item("在资源管理器中打开当前库目录").Chosen() {
			s.OnRevealInExplorer("")
		}
	})
}

func (s *Shell) openVaultLauncher() {
	if strings.TrimSpace(s.VaultLauncherName) == "" {
		s.VaultLauncherName = "AsterLab"
	}
	if strings.TrimSpace(s.VaultLauncherParent) == "" {
		s.VaultLauncherParent = defaultSuggestedVaultParent()
	}
	s.VaultLauncherErr = ""
	s.VaultLauncherOpen = true
}

func defaultSuggestedVaultParent() string {
	// Prefer D:\ on Windows if it exists and is writable so user data stays off C:\AppData
	if info, err := os.Stat(`D:\`); err == nil && info.IsDir() {
		return `D:\`
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		docs := filepath.Join(home, "Documents")
		if info, err := os.Stat(docs); err == nil && info.IsDir() {
			return docs
		}
		return home
	}
	return ""
}

// renderVaultLauncherDialog renders the Obsidian-style Vault Launcher & Switcher modal.
// Always mounts its Dialog host cleanly across open/closed frames so exit transitions stay safe.
func (s *Shell) renderVaultLauncherDialog(c *ui.Context) {
	if !s.VaultLauncherOpen {
		ui.Box(c.Key("vault-launcher-dlg-host")).Children(func() {
			components.Dialog(c.Key("vault-launcher-dlg"), &s.VaultLauncherOpen, components.DialogOptions{
				Label: "管理本地实验库",
			}, nil)
		})
		return
	}
	if strings.TrimSpace(s.VaultLauncherName) == "" {
		s.VaultLauncherName = "AsterLab"
	}
	if strings.TrimSpace(s.VaultLauncherParent) == "" {
		s.VaultLauncherParent = defaultSuggestedVaultParent()
	}

	tok := theme.GetTokens(c.Theme())
	open := s.VaultLauncherOpen
	ui.Box(c.Key("vault-launcher-dlg-host")).Children(func() {
		components.Dialog(c.Key("vault-launcher-dlg"), &open, components.DialogOptions{
			Label: "选择或新建本地实验库",
		}, func(panel ui.Element) {
			panel.Width(float32(tok.Spacing48)*14.5).Padding(0).Gap(0).
				Radius(float32(tok.DialogRadius)).Background(tok.Surface).Border(1, tok.BorderStrong).Clip()

			ui.Row(c).FillWidth().Gap(0).Children(func() {
				// Left Pane: Brand + Recent Vaults list
				ui.Column(c).Width(float32(tok.Spacing48)*5.2).FillHeight().
					Padding(float32(tok.Spacing16)).Gap(float32(tok.Spacing12)).
					Background(tok.Elevated).Children(func() {
					ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
						ui.Icon(c, theme.LogoParsedSmall()).Size(float32(tok.Spacing24), float32(tok.Spacing24)).TextColor(tok.Accent)
						ui.Column(c).Grow(1).MinWidth(0).Gap(1).Children(func() {
							components.Text(c, "AsterLab").FontSize(float32(tok.TextSection)).Bold().TextColor(tok.Text)
							components.Text(c, "本地优先实验工作台").FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
						})
					})

					ui.Divider(c)
					components.Text(c, "最近打开的实验库").FontSize(float32(tok.TextMeta)).Bold().TextColor(tok.TextMuted)

					if len(s.RecentVaults) == 0 {
						ui.Column(c).FillWidth().Padding(float32(tok.Spacing12)).Radius(float32(tok.ControlRadius)).
							Background(tok.Surface).Border(1, tok.Border).Gap(float32(tok.Spacing8)).Children(func() {
							components.Text(c, "还没有记录的实验库").FontSize(float32(tok.TextBody)).Bold().TextColor(tok.Text)
							components.Text(c, "所有 .asterml 实验文档、图片与 .asterlab/pantry.jsonl 储藏室台账都直接保存在你选定的本地文件夹里，方便用网盘或 Git 备份同步。").
								FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
						})
					} else {
						ui.Scroll(c.Key("vault-recent-scroll")).FillWidth().MaxHeight(float32(tok.Spacing48)*7).Children(func() {
							ui.Column(c).FillWidth().Gap(float32(tok.Spacing4)).Children(func() {
								for _, rv := range s.RecentVaults {
									rv := rv
									bg := tok.Surface
									bdr := tok.Border
									if rv.Active {
										bg = tok.AccentSoftBg
										bdr = tok.Accent.Alpha(0.55)
									}
									ui.Row(c.Key("rv-"+rv.Path)).FillWidth().Padding(float32(tok.Spacing8)).
										Radius(float32(tok.ControlRadius)).Background(bg).Border(1, bdr).
										AlignItems(ui.Center).Gap(float32(tok.Spacing4)).Children(func() {
										pick := ui.ButtonBase(c.Key("rv-open-"+rv.Path)).Label("打开 "+rv.Name).
											Grow(1).MinWidth(0).Justify(ui.Start).OnClick(func() {
											if s.OnSwitchVaultPath != nil {
												s.OnSwitchVaultPath(rv.Path)
												open = false
											}
										})
										pick.Children(func() {
											ui.Column(c).FillWidth().Gap(2).Children(func() {
												ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(float32(tok.Spacing4)).Children(func() {
													components.Text(c, rv.Name).FontSize(float32(tok.TextBody)).Bold().TextColor(tok.Text).
														SingleLine().Ellipsis("…").Grow(1).MinWidth(0)
													if rv.Active {
														components.Badge(c, "当前", components.BadgeOptions{Tone: components.BadgeAccent})
													} else if rv.Sandbox {
														components.Badge(c, "临时", components.BadgeOptions{Tone: components.BadgeWarning})
													}
												})
												components.Text(c, rv.Path).FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted).
													SingleLine().Ellipsis("…").MinWidth(0)
											})
										})
										if !rv.Active && s.OnRemoveRecentVault != nil {
											if components.IconButton(c.Key("rv-rm-"+rv.Path), lucide.X(), "从列表移除", components.IconButtonCompact).Clicked() {
												s.OnRemoveRecentVault(rv.Path)
											}
										}
									})
								}
							})
						})
					}

					ui.Spacer(c)
					if s.VaultPathInput != "" {
						components.Text(c, "当前路径："+s.VaultPathInput).FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted).
							SingleLine().Ellipsis("…")
					}
				})

				ui.Box(c).Width(1).FillHeight().Background(tok.Border)

				// Right Pane: 3 Clear Actions (Create New / Open Existing / Quick Sandbox)
				ui.Column(c).Grow(1).MinWidth(0).Padding(float32(tok.Spacing16)).Gap(float32(tok.Spacing12)).Children(func() {
					ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
						components.Text(c, "选择你的实验库目录").FontSize(float32(tok.TextSection)).Bold().TextColor(tok.Text)
						ui.Spacer(c)
						if components.IconButton(c.Key("vault-launcher-x"), lucide.X(), "关闭", components.IconButtonCompact).Clicked() {
							open = false
						}
					})

					// 1. Create New Vault Card
					ui.Column(c).FillWidth().Padding(float32(tok.Spacing12)).Radius(float32(tok.PanelRadius)).
						Background(tok.Elevated).Border(1, tok.Accent.Alpha(0.45)).Gap(float32(tok.Spacing8)).Children(func() {
						ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
							ui.Icon(c, lucide.FolderPlus()).Size(float32(tok.Spacing16), float32(tok.Spacing16)).TextColor(tok.Accent)
							components.Text(c, "新建本地实验库").FontSize(float32(tok.TextBody)).Bold().TextColor(tok.Text)
							components.Badge(c, "推荐", components.BadgeOptions{Tone: components.BadgeAccent})
						})
						components.Text(c, "在你指定的磁盘目录创建一个全新的独立实验库文件夹，自动初始化模板、试剂储藏室与 AI 协作规则。").
							FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)

						ui.Row(c).FillWidth().Gap(float32(tok.Spacing8)).Children(func() {
							ui.Column(c).Width(float32(tok.Spacing48)*2.8).Gap(float32(tok.Spacing4)).Children(func() {
								components.Text(c, "实验库名称").FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
								components.TextBox(c.Key("vl-name"), &s.VaultLauncherName, components.TextBoxOptions{}).
									Placeholder("AsterLab")
							})
							ui.Column(c).Grow(1).MinWidth(0).Gap(float32(tok.Spacing4)).Children(func() {
								components.Text(c, "存放位置").FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
								ui.Row(c).FillWidth().Gap(float32(tok.Spacing4)).Children(func() {
									ui.Box(c).Grow(1).MinWidth(0).Children(func() {
										components.TextBox(c.Key("vl-parent"), &s.VaultLauncherParent, components.TextBoxOptions{}).
											Placeholder(`例如 D:\ 或文档目录`)
									})
									if s.OnPickVaultPath != nil && components.Button(c.Key("vl-browse"), "浏览…", components.ButtonSecondary).Clicked() {
										if picked := strings.TrimSpace(s.OnPickVaultPath()); picked != "" {
											s.VaultLauncherParent = picked
										}
									}
								})
							})
						})

						previewPath := filepath.Join(strings.TrimSpace(s.VaultLauncherParent), strings.TrimSpace(s.VaultLauncherName))
						if strings.TrimSpace(s.VaultLauncherParent) != "" && strings.TrimSpace(s.VaultLauncherName) != "" {
							components.Text(c, "将创建目录："+previewPath).FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted).
								SingleLine().Ellipsis("…")
						}

						ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
							components.Checkbox(c.Key("vl-seed-demo"), &s.VaultLauncherSeedDemo, "放入新手指南与示范实验 (CCK-8 / CFU)")
							ui.Spacer(c)
							if components.Button(c.Key("vl-create"), "创建并打开", components.ButtonPrimary).Clicked() {
								name := strings.TrimSpace(s.VaultLauncherName)
								parent := strings.TrimSpace(s.VaultLauncherParent)
								if name == "" || parent == "" {
									s.VaultLauncherErr = "请先填写实验库名称和存放位置"
								} else if s.OnCreateVault != nil {
									if err := s.OnCreateVault(filepath.Join(parent, name), s.VaultLauncherSeedDemo); err != nil {
										s.VaultLauncherErr = err.Error()
									} else {
										s.VaultLauncherErr = ""
										open = false
									}
								}
							}
						})
					})

					// 2. Open Existing Folder Card
					ui.Row(c).FillWidth().Padding(float32(tok.Spacing12)).Radius(float32(tok.PanelRadius)).
						Background(tok.Elevated).Border(1, tok.Border).AlignItems(ui.Center).Gap(float32(tok.Spacing12)).Children(func() {
						ui.Icon(c, lucide.FolderOpen()).Size(float32(tok.Spacing16), float32(tok.Spacing16)).TextColor(tok.TextMuted)
						ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
							components.Text(c, "打开电脑已有文件夹作为实验库").FontSize(float32(tok.TextBody)).Bold().TextColor(tok.Text)
							components.Text(c, "直接打开已有的 .asterml 实验库、OneDrive/坚果云同步文件夹或 Git 仓库。").
								FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
						})
						if s.OnPickVaultPath != nil && components.Button(c.Key("vl-open-existing"), "选择文件夹…", components.ButtonSecondary).Clicked() {
							if picked := strings.TrimSpace(s.OnPickVaultPath()); picked != "" {
								if s.OnSwitchVaultPath != nil {
									s.OnSwitchVaultPath(picked)
									s.VaultLauncherErr = ""
									open = false
								}
							}
						}
					})

					// 3. Quick Sandbox Fallback
					ui.Row(c).FillWidth().Padding(float32(tok.Spacing8), float32(tok.Spacing12)).Radius(float32(tok.ControlRadius)).
						Background(tok.Surface).Border(1, tok.Border).AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
						components.Text(c, "只想先看看功能？可进入内置临时体验库，随时在左下角迁移到正式文件夹。").
							FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted).Grow(1).MinWidth(0)
						if components.Button(c.Key("vl-sandbox"), "进入临时体验库", components.ButtonGhost).Clicked() {
							if s.OnOpenSandboxVault != nil {
								s.OnOpenSandboxVault()
							}
							open = false
						}
					})

					if s.VaultLauncherErr != "" {
						components.Text(c, s.VaultLauncherErr).FontSize(float32(tok.TextMeta)).TextColor(tok.Danger)
					}
				})
			})
		})
	})
	s.VaultLauncherOpen = open
}
