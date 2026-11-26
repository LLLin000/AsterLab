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

func (s *Shell) renderCrashOrWatchdogBanner(c *ui.Context, tok theme.Tokens) {
	if strings.TrimSpace(s.CrashNotice) == "" {
		return
	}
	ui.Row(c.Key("crash-watchdog-banner")).FillWidth().
		Padding(float32(tok.Spacing8), float32(tok.Spacing12)).
		Background(tok.Warning.Alpha(0.10)).
		AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
		ui.Icon(c, lucide.TriangleAlert()).Size(float32(tok.Spacing16), float32(tok.Spacing16)).TextColor(tok.Warning).Shrink(0)
		components.Text(c, s.CrashNotice).FontSize(float32(tok.TextData)).TextColor(tok.Text).Grow(1).MinWidth(0)
		if components.Button(c, "导出诊断包", components.ButtonSecondary).Clicked() && s.OnExportDiagBundle != nil {
			s.OnExportDiagBundle()
		}
		if components.Button(c, "反馈问题", components.ButtonPrimary).Clicked() {
			s.FeedbackCategory = "程序异常"
			s.FeedbackOpen = true
		}
		if components.Button(c, "忽略", components.ButtonGhost).Clicked() {
			s.CrashNotice = ""
			if s.OnDismissCrashNotice != nil {
				s.OnDismissCrashNotice()
			}
		}
	})
	ui.Box(c).FillWidth().Height(1).Background(tok.Warning.Alpha(0.25))
}

func (s *Shell) renderFeedbackDialog(c *ui.Context) {
	if !s.FeedbackOpen {
		ui.Box(c.Key("feedback-dlg")).Children(func() {
			components.Dialog(c, &s.FeedbackOpen, components.DialogOptions{Label: "反馈与诊断中心"}, nil)
		})
		return
	}
	tok := theme.GetTokens(c.Theme())
	if strings.TrimSpace(s.FeedbackCategory) == "" {
		s.FeedbackCategory = "程序异常"
	}
	if s.IsMobileLayout(c) {
		s.renderMobileFeedbackPage(c, tok)
		return
	}
	open := s.FeedbackOpen
	ui.Box(c.Key("feedback-dlg")).Children(func() {
		components.Dialog(c, &open, components.DialogOptions{Label: "反馈与诊断中心"}, func(panel ui.Element) {
			panel.Width(float32(tok.Spacing48)*11.5).Padding(float32(tok.Spacing24)).Gap(float32(tok.Spacing12))
			if tok.Dark {
				panel.Background(tok.Surface).Border(1, tok.BorderStrong)
			} else if tok.LiquidGlass {
				theme.ApplyPanelGlass(panel, tok, 0.36)
			}
			panel.Children(func() {
				ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
					ui.Icon(c, lucide.LifeBuoy()).Size(float32(tok.Spacing24), float32(tok.Spacing24)).TextColor(tok.Accent)
					ui.Text(c, "反馈与诊断中心").FontSize(float32(tok.TextSection)).Bold().TextColor(tok.Text)
					ui.Spacer(c)
					ver := firstNonEmptyStr(s.AppVersion, "0.1.0-dev")
					kit.Badge(c, "v"+strings.TrimPrefix(ver, "v"), components.BadgeOptions{Tone: components.BadgeNeutral})
				})

				ui.Column(c).FillWidth().Padding(float32(tok.Spacing12)).Radius(float32(tok.PanelRadius)).
					Background(tok.Elevated).Border(1, tok.Border).Gap(float32(tok.Spacing8)).Children(func() {
					ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
						ui.Icon(c, lucide.Camera()).Size(float32(tok.Spacing16), float32(tok.Spacing16)).TextColor(tok.Accent).Shrink(0)
						ui.Text(c, "可视化截图标注反馈模式").FontSize(float32(tok.TextData)).Bold().TextColor(tok.Text).Grow(1).MinWidth(0)
						if s.FeedbackMode {
							kit.Badge(c, "已开启", components.BadgeOptions{Tone: components.BadgeAccent})
						} else {
							kit.Badge(c, "默认关闭", components.BadgeOptions{Tone: components.BadgeNeutral})
						}
					})
					ui.Text(c, "支持随时开启并在界面红框截图批注、一键直传开发者（Ctrl+Shift+D），无需单独下载内测版。").
						FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
					ui.Row(c).FillWidth().Wrap().AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
						if components.Switch(c, &s.FeedbackMode, "显示右下角悬浮反馈条", components.SwitchOptions{}).Changed() {
							s.notifySettingsChanged()
						}
						ui.Spacer(c)
						if s.OnOpenFeedbackQueue != nil {
							if components.Button(c, "反馈队列", components.ButtonSecondary).Clicked() {
								s.OnOpenFeedbackQueue()
							}
						}
						if s.OnStartFeedbackPick != nil {
							if components.Button(c, "立即框选截图", components.ButtonPrimary).Clicked() {
								s.OnStartFeedbackPick()
							}
						}
					})
				})

				ui.Column(c).FillWidth().Gap(float32(tok.Spacing4)).Children(func() {
					ui.Text(c, "反馈类别").FontSize(float32(tok.TextData)).Bold().TextColor(tok.TextMuted)
					ui.Row(c).FillWidth().Wrap().Gap(float32(tok.Spacing8)).Children(func() {
						for _, cat := range []string{"程序异常", "安装与更新", "界面排版与显示", "功能建议"} {
							cat := cat
							variant := components.ButtonOutline
							if s.FeedbackCategory == cat {
								variant = components.ButtonPrimary
							}
							if components.Button(c, cat, variant).Clicked() {
								s.FeedbackCategory = cat
							}
						}
					})
				})

				ui.Column(c).FillWidth().Gap(float32(tok.Spacing4)).Children(func() {
					ui.Text(c, "现象描述与复现步骤").FontSize(float32(tok.TextData)).Bold().TextColor(tok.TextMuted)
					components.TextBox(c, &s.FeedbackText, components.TextBoxOptions{Multiline: true}).
						Placeholder("请简要描述您做了什么操作、遇到了什么报错或希望改进的体验…").
						Height(float32(tok.Spacing48) * 2.2)
				})

				ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
					components.Switch(c, &s.FeedbackIncludeLogs, "附加最近脱敏运行日志与环境摘要", components.SwitchOptions{})
				})

				ui.Row(c).FillWidth().Padding(float32(tok.Spacing8)).Radius(float32(tok.ControlRadius)).
					Background(tok.Elevated).AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
					ui.Icon(c, lucide.ShieldCheck()).Size(float32(tok.Spacing16), float32(tok.Spacing16)).TextColor(tok.Success).Shrink(0)
					ui.Text(c, "隐私保护：用户名、本机用户目录与令牌均自动脱敏为 <USER_HOME> / <REDACTED_TOKEN>，诊断包绝不包含您的 .asterml 实验内容与附件。").
						FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted).Grow(1).MinWidth(0)
				})

				if s.FeedbackStatus != "" {
					ui.Text(c, s.FeedbackStatus).FontSize(float32(tok.TextData)).TextColor(tok.Accent)
				}

				ui.Row(c).FillWidth().Wrap().AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
					if components.Button(c, "导出脱敏诊断包 (.zip)", components.ButtonSecondary).Clicked() && s.OnExportDiagBundle != nil {
						s.OnExportDiagBundle()
					}
					if components.Button(c, "复制报告摘要", components.ButtonGhost).Clicked() && s.OnCopyFeedbackReport != nil {
						s.OnCopyFeedbackReport()
					}
					ui.Spacer(c)
					if components.Button(c, "关闭", components.ButtonGhost).Clicked() {
						open = false
					}
					if components.Button(c, "前往 GitHub 提交", components.ButtonPrimary).Clicked() && s.OnSubmitFeedback != nil {
						s.OnSubmitFeedback()
					}
				})
			})
		})
	})
	s.FeedbackOpen = open
}

func (s *Shell) renderWelcomeDialog(c *ui.Context) {
	if !s.WelcomeOpen {
		ui.Box(c.Key("welcome-dlg")).Children(func() {
			components.Dialog(c, &s.WelcomeOpen, components.DialogOptions{Label: "欢迎使用 AsterLab"}, nil)
		})
		return
	}
	tok := theme.GetTokens(c.Theme())
	if s.IsMobileLayout(c) {
		s.renderMobileWelcomePage(c, tok)
		return
	}
	open := s.WelcomeOpen
	choiceHandled := false
	ui.Box(c.Key("welcome-dlg")).Children(func() {
		components.Dialog(c, &open, components.DialogOptions{Label: "欢迎使用 AsterLab"}, func(panel ui.Element) {
			panel.Width(float32(tok.Spacing48)*12).Padding(float32(tok.Spacing24)).Gap(float32(tok.Spacing16))
			if tok.Dark {
				panel.Background(tok.Surface).Border(1, tok.BorderStrong)
			} else if tok.LiquidGlass {
				theme.ApplyPanelGlass(panel, tok, 0.36)
			}
			panel.Children(func() {
				ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(float32(tok.Spacing12)).Children(func() {
					ui.Image(c, theme.LogoParsedTriStream(tok.Dark)).Width(float32(tok.Spacing48)).Height(float32(tok.Spacing48)*0.85)
					ui.Column(c).Grow(1).MinWidth(0).Gap(float32(tok.Spacing4)).Children(func() {
						ui.Text(c, "欢迎使用 AsterLab").FontSize(float32(tok.TextPage)).Bold().TextColor(tok.Text)
						ui.Text(c, "本地科研实验工作台 · 所有实验档案与照片安全保存在你自己的电脑中").
							FontSize(float32(tok.TextData)).TextColor(tok.TextMuted)
					})
				})

				ui.Column(c).FillWidth().Padding(float32(tok.Spacing12)).Radius(float32(tok.ControlRadius)).
					Background(tok.Elevated).Gap(float32(tok.Spacing4)).Children(func() {
					ui.Text(c, "3 步快速上手（无需懂任何代码语法）：").FontSize(float32(tok.TextData)).Bold().TextColor(tok.Text)
					ui.Text(c, "1. 写方案：左侧管理实验档案与模板，画布像普通文档一样打字，点 + 或右键即可插入分组、孔板、计算器和计时器。").
						FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
					ui.Text(c, "2. 文字变工具：写完稀释或孵育句子后，点行尾 ✦（或按住 Alt 悬停）即可一键转为交互组件。").
						FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
					ui.Text(c, "3. 台面执行：走进实验室点右上角「实验」开启防误触，直接计时、勾待办、从 Excel 粘贴读数并自动汇总。").
						FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
				})

				ui.Column(c).FillWidth().Gap(float32(tok.Spacing8)).Children(func() {
					card := func(key, title, desc string, badge string, primary bool, choice string) {
						row := ui.Row(c.Key("welcome-opt-"+key)).FillWidth().Padding(float32(tok.Spacing12)).
							Radius(float32(tok.ControlRadius)).Background(tok.Surface).Border(1, tok.Border).
							AlignItems(ui.Center).Gap(float32(tok.Spacing12))
						if primary {
							row.Border(1, tok.Accent.Alpha(0.55)).Background(tok.AccentSoftBg.Alpha(0.45))
						}
						theme.ApplyTactilePress(row)
						row.Children(func() {
							ui.Column(c).Grow(1).MinWidth(0).Gap(float32(tok.Spacing4)).Children(func() {
								ui.Row(c).AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
									ui.Text(c, title).FontSize(float32(tok.TextBody)).Bold().TextColor(tok.Text)
									if badge != "" {
										components.Badge(c, badge)
									}
								})
								ui.Text(c, desc).FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
							})
							btnStyle := components.ButtonSecondary
							if primary {
								btnStyle = components.ButtonPrimary
							}
							if components.Button(c, "开始", btnStyle).Clicked() {
								open = false
								choiceHandled = true
								if s.OnOnboardingChoice != nil {
									s.OnOnboardingChoice(choice)
								}
							}
						})
					}
					card("guide", "打开「新手入门指南」互动文档（推荐）", "边看边动手：内含可试玩的 12 孔板、配液计算器、计时器、快捷键与全部右键功能速查。", "5 分钟上手", true, "guide")
					card("new", "新建空白实验文档", "打开干净白纸，直接输入笔记，随时点击 + 插入步骤、孔板、计算器与数据表。", "", false, "new")
					card("demo", "体验「CCK-8 细胞活力」完整示范实验", "一键生成含分组、96 孔板、显色计时器与 OD450 汇总统计的真实示范实验。", "", false, "demo")
					card("existing", "打开电脑已有实验库文件夹", fmt.Sprintf("当前目录：%s", firstNonEmptyStr(s.VaultPathInput, "默认实验库目录")), "", false, "existing")
				})

				ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
					if components.Button(c, "1 分钟界面区域导览", components.ButtonOutline).Clicked() {
						open = false
						choiceHandled = true
						if s.OnOnboardingChoice != nil {
							s.OnOnboardingChoice("tour")
						}
					}
					ui.Spacer(c)
					if components.Button(c, "稍后探索", components.ButtonGhost).Clicked() {
						open = false
						choiceHandled = true
						if s.OnOnboardingChoice != nil {
							s.OnOnboardingChoice("skip")
						}
					}
				})
			})
		})
	})
	if !open && !choiceHandled && s.WelcomeOpen && s.OnOnboardingChoice != nil {
		s.OnOnboardingChoice("skip")
	}
	s.WelcomeOpen = open
}

func (s *Shell) renderMobileFeedbackPage(c *ui.Context, tok theme.Tokens) {
	ui.Overlay(c, func() {
		t := c.Theme()
		topSafe, rightSafe, bottomSafe, leftSafe := c.SafeArea()
	ui.Column(c.Key("mb-feedback-root")).Absolute().Fill().Modal().Background(tok.Canvas).Children(func() {
		ui.Row(c).FillWidth().
			Padding(t.Space(2)+topSafe, t.Space(4)+rightSafe, t.Space(2), t.Space(4)+leftSafe).
			MinHeight(t.Space(12)+topSafe).
			AlignItems(ui.Center).Gap(t.Space(2)).Background(tok.Surface).Children(func() {
			if components.Button(c.Key("mb-fb-cancel"), "取消", components.ButtonGhost).Shrink(0).Clicked() {
				s.FeedbackOpen = false
			}
			ui.Text(c, "反馈与诊断").FontSize(float32(tok.TextSection)).Bold().TextColor(tok.Text).
				Grow(1).MinWidth(0).SingleLine().Ellipsis("…")
			if components.Button(c.Key("mb-fb-submit"), "提交", components.ButtonPrimary).Shrink(0).Clicked() && s.OnSubmitFeedback != nil {
				s.OnSubmitFeedback()
			}
		})
		ui.Box(c).FillWidth().Height(1).Background(tok.BorderSubtle)

		ui.Scroll(c.Key("mb-fb-scroll")).FillWidth().Grow(1).Basis(0).MinHeight(0).Clip().Children(func() {
			ui.Column(c).FillWidth().
				Padding(t.Space(4), t.Space(4)+rightSafe, t.Space(8)+bottomSafe, t.Space(4)+leftSafe).
				Gap(t.Space(4)).Children(func() {
				ui.Column(c).FillWidth().Gap(t.Space(2)).Children(func() {
					ui.Text(c, "反馈类别").FontSize(float32(tok.TextMeta)).Bold().TextColor(tok.TextMuted)
					ui.Row(c).FillWidth().Wrap().Gap(t.Space(2)).Children(func() {
						for _, cat := range []string{"程序异常", "安装与更新", "界面排版与显示", "功能建议"} {
							cat := cat
							variant := components.ButtonOutline
							if s.FeedbackCategory == cat {
								variant = components.ButtonPrimary
							}
							if components.Button(c, cat, variant).Clicked() {
								s.FeedbackCategory = cat
							}
						}
					})
				})

				ui.Column(c).FillWidth().Gap(t.Space(1)).Children(func() {
					ui.Text(c, "现象描述与复现步骤").FontSize(float32(tok.TextMeta)).Bold().TextColor(tok.TextMuted)
					components.TextBox(c, &s.FeedbackText, components.TextBoxOptions{Multiline: true}).
						Placeholder("请简述操作路径、报错信息或建议…").
						FillWidth().Height(float32(tok.Spacing48) * 2.4)
				})

				ui.Column(c).FillWidth().Padding(t.Space(3)).Radius(float32(tok.PanelRadius)).
					Background(tok.Surface).Border(1, tok.Border).Gap(t.Space(2)).Children(func() {
					components.Switch(c, &s.FeedbackIncludeLogs, "附加脱敏运行日志与环境摘要", components.SwitchOptions{})
					ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(t.Space(2)).Children(func() {
						kit.IconBox(c, tok, lucide.ShieldCheck(), tok.Success)
						ui.Text(c, "目录与令牌已脱敏，不含实验内容与附件。").
							FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted).Grow(1).MinWidth(0)
					})
				})

				if s.FeedbackStatus != "" {
					ui.Text(c, s.FeedbackStatus).FontSize(float32(tok.TextData)).TextColor(tok.Accent)
				}

				ui.Row(c).FillWidth().Wrap().AlignItems(ui.Center).Gap(t.Space(2)).Children(func() {
					if components.Button(c, "导出诊断包 (.zip)", components.ButtonSecondary).Clicked() && s.OnExportDiagBundle != nil {
						s.OnExportDiagBundle()
					}
					if components.Button(c, "复制报告摘要", components.ButtonGhost).Clicked() && s.OnCopyFeedbackReport != nil {
						s.OnCopyFeedbackReport()
					}
				})
			})
		})
	})
	})
}

func (s *Shell) renderMobileWelcomePage(c *ui.Context, tok theme.Tokens) {
	ui.Overlay(c, func() {
		t := c.Theme()
		topSafe, rightSafe, bottomSafe, leftSafe := c.SafeArea()
	choose := func(choice string) {
		s.WelcomeOpen = false
		if s.OnOnboardingChoice != nil {
			s.OnOnboardingChoice(choice)
		}
	}
	ui.Column(c.Key("mb-welcome-root")).Absolute().Fill().Modal().Background(tok.Canvas).Children(func() {
		ui.Row(c).FillWidth().
			Padding(t.Space(2)+topSafe, t.Space(4)+rightSafe, t.Space(2), t.Space(4)+leftSafe).
			MinHeight(t.Space(12)+topSafe).
			AlignItems(ui.Center).Gap(t.Space(2)).Background(tok.Surface).Children(func() {
			ui.Text(c, "欢迎使用 AsterLab").FontSize(float32(tok.TextSection)).Bold().TextColor(tok.Text).
				Grow(1).MinWidth(0).SingleLine().Ellipsis("…")
			if components.Button(c.Key("mb-welcome-skip"), "稍后探索", components.ButtonGhost).Shrink(0).Clicked() {
				choose("skip")
			}
		})
		ui.Box(c).FillWidth().Height(1).Background(tok.BorderSubtle)

		ui.Scroll(c.Key("mb-welcome-scroll")).FillWidth().Grow(1).Basis(0).MinHeight(0).Clip().Children(func() {
			ui.Column(c).FillWidth().
				Padding(t.Space(4), t.Space(4)+rightSafe, t.Space(8)+bottomSafe, t.Space(4)+leftSafe).
				Gap(t.Space(4)).Children(func() {
				ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(t.Space(3)).Children(func() {
					ui.Image(c, theme.LogoParsedTriStream(tok.Dark)).Width(float32(tok.Spacing48)).Height(float32(tok.Spacing48) * 0.85)
					ui.Column(c).Grow(1).MinWidth(0).Gap(t.Space(1)).Children(func() {
						ui.Text(c, "本地科研实验工作台").FontSize(float32(tok.TextBody)).Bold().TextColor(tok.Text)
						ui.Text(c, "所有实验档案与照片保存在本地目录").FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
					})
				})

				ui.Column(c).FillWidth().Radius(float32(tok.PanelRadius)).
					Background(tok.Surface).Border(1, tok.Border).Clip().Children(func() {
					choices := []struct {
						key, title, desc string
						ico              *ui.SVG
						primary          bool
						choice           string
					}{
						{"guide", "新手入门指南（推荐）", "内含可交互 12 孔板、配液计算器与计时器", lucide.BookOpen(), true, "guide"},
						{"new", "新建空白实验文档", "直接开始记录步骤、试剂与数据", lucide.FilePlus(), false, "new"},
						{"demo", "体验「CCK-8」完整示范", "生成分组、96 孔板与 OD450 汇总实例", lucide.FlaskConical(), false, "demo"},
						{"existing", "打开已有实验库目录", firstNonEmptyStr(s.VaultPathInput, "默认实验库目录"), lucide.FolderOpen(), false, "existing"},
					}
					for i, ch := range choices {
						ch := ch
						if i > 0 {
							ui.Box(c).FillWidth().Height(1).Background(tok.BorderSubtle)
						}
						bg := tok.Surface
						if ch.primary {
							bg = tok.AccentSoftBg.Alpha(0.35)
						}
						row := ui.ButtonBase(c.Key("mb-welcome-opt-"+ch.key)).Label(ch.title).
							FillWidth().MinHeight(float32(tok.Spacing48)+t.Space(2)).
							Padding(t.Space(3), t.Space(4)).
							Background(bg).Justify(ui.Start).
							OnClick(func() { choose(ch.choice) })
						theme.ApplyTactilePress(row)
						row.Children(func() {
							ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(t.Space(3)).Children(func() {
								kit.IconBox(c, tok, ch.ico, tok.Accent)
								ui.Column(c).Grow(1).MinWidth(0).Gap(t.Space(1)).Children(func() {
									ui.Text(c, ch.title).FontSize(float32(tok.TextBody)).Bold().TextColor(tok.Text).SingleLine().Ellipsis("…")
									ui.Text(c, ch.desc).FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted).SingleLine().Ellipsis("…")
								})
								ui.Icon(c, lucide.ChevronRight()).Size(float32(tok.IconSm), float32(tok.IconSm)).TextColor(tok.TextMuted)
							})
						})
					}
				})

				ui.Row(c).FillWidth().Justify(ui.Center).Children(func() {
					if components.Button(c, "1 分钟界面区域导览", components.ButtonOutline).Clicked() {
						choose("tour")
					}
				})
			})
		})
	})
	})
}
