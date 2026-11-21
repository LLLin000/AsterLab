package shell

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/onevra/source/components"
	"github.com/oneo-me/onevra/source/icons/lucide"

	"asterlab-mygo-prototype/internal/theme"
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
					components.Badge(c, "v"+strings.TrimPrefix(ver, "v"))
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
	open := s.WelcomeOpen
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
						ui.Text(c, "本地单真源 .asterml 实验工作台 · 数据完全保存在您自己的电脑目录").
							FontSize(float32(tok.TextData)).TextColor(tok.TextMuted)
					})
				})

				ui.Column(c).FillWidth().Padding(float32(tok.Spacing12)).Radius(float32(tok.ControlRadius)).
					Background(tok.Elevated).Gap(float32(tok.Spacing4)).Children(func() {
					ui.Text(c, "3 步快速理解工作方式：").FontSize(float32(tok.TextData)).Bold().TextColor(tok.Text)
					ui.Text(c, "1. 实验库（Vault）：左侧树展示所有 .asterml 实验文档，每份实验都是独立的明文科研档案。").
						FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
					ui.Text(c, "2. 实验结构：上方设定目的与分组，下方每个「步骤」内自由组合文本、参数表、孔板、计时器与计算器。").
						FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
					ui.Text(c, "3. 储藏室与工具：随时登记试剂批号/冻存盒位置，右下角可随时检测更新与导出诊断反馈。").
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
							if components.Button(c, "选择", btnStyle).Clicked() {
								open = false
								if s.OnOnboardingChoice != nil {
									s.OnOnboardingChoice(choice)
								}
							}
						})
					}
					card("new", "新建空白实验（推荐）", "打开极简双步骤空白画布，点击虚线框即可自由添加文本、表格、孔板与计时器。", "快速开始", true, "new")
					card("demo", "体验「CCK-8 细胞活力示范实验库」", "一键生成含分组、步骤卡片、96孔板与读数统计的示范实验，直观感受完整用法。", "", false, "demo")
					card("existing", "打开本地已有实验库目录", fmt.Sprintf("当前默认路径：%s", firstNonEmptyStr(s.VaultPathInput, "默认实验库目录")), "", false, "existing")
				})

				ui.Row(c).FillWidth().Justify(ui.End).Children(func() {
					if components.Button(c, "稍后探索（跳过引导）", components.ButtonGhost).Clicked() {
						open = false
						if s.OnOnboardingChoice != nil {
							s.OnOnboardingChoice("skip")
						}
					}
				})
			})
		})
	})
	if !open && s.WelcomeOpen && s.OnOnboardingChoice != nil {
		s.OnOnboardingChoice("skip")
	}
	s.WelcomeOpen = open
}
