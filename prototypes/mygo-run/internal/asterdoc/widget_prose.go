package asterdoc

import (
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/onevra/source/components"
	"github.com/oneo-me/onevra/source/icons/lucide"

	"asterlab-mygo-prototype/internal/theme"
)

func moduleCardFrame(v *View, c *ui.Context, icon *ui.SVG, title, badgeText string, accentBar *ui.Color, content func()) {
	t := c.Theme()
	tok := theme.GetTokens(t)
	editableTitle := v != nil && v.Mode == ModeDesign && !v.ReadOnly && v.Can(PermDocumentEdit) && v.currentModuleID != ""
	if v != nil && v.UnifiedLive {
		// In UnifiedLive canvas, render an open Obsidian-style hairline section without box cages
		ui.Column(c).FillWidth().Padding(float32(tok.Spacing4), 0).Gap(float32(tok.Spacing8)).Children(func() {
			if title != "" || badgeText != "" || icon != nil {
				hdrRow := ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(float32(tok.Spacing8))
				hdrRow.Children(func() {
					if icon != nil {
						ui.Icon(c, icon).Size(float32(tok.Spacing16), float32(tok.Spacing16)).TextColor(tok.TextMuted)
					}
					if title != "" {
						renderInlineEditableTitle(v, c, v.currentModuleID, title, editableTitle)
					}
					ui.Spacer(c)
					if badgeText != "" {
						ui.Text(c, badgeText).FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted).FontFeatures("tnum")
					}
				})
				if v.currentModuleData != nil && v.currentModuleKind != "" {
					components.ContextMenu(c, hdrRow, func(m *components.Menu) {
						v.buildComponentShareMenu(c, m, v.currentModuleKind, v.currentModuleData)
					})
				}
			}
			content()
		})
		return
	}
	ui.Row(c).FillWidth().Radius(float32(tok.PanelRadius)).Background(tok.Surface).
		Border(1, tok.Border.Alpha(0.7)).Children(func() {
		if accentBar != nil {
			ui.Box(c).Width(t.Space(0.75)).FillHeight().Background(*accentBar)
		}
		ui.Column(c).FillWidth().Gap(float32(tok.Spacing8)).Padding(float32(tok.Spacing12)).Children(func() {
			if title != "" || badgeText != "" || icon != nil {
				ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
					if icon != nil {
						ui.Icon(c, icon).Size(float32(tok.Spacing16), float32(tok.Spacing16)).TextColor(tok.TextMuted)
					}
					if title != "" {
						modID := ""
						if v != nil {
							modID = v.currentModuleID
						}
						renderInlineEditableTitle(v, c, modID, title, editableTitle)
					}
					ui.Spacer(c)
					if badgeText != "" {
						components.Badge(c, badgeText)
					}
				})
			}
			content()
		})
	})
}

func renderInlineEditableTitle(v *View, c *ui.Context, moduleID, title string, editable bool) {
	tok := theme.GetTokens(c.Theme())
	if v == nil || !editable || moduleID == "" {
		ui.Text(c, title).FontSize(float32(tok.TextSection)).Bold().TextColor(tok.Text)
		return
	}
	if v.titleEditID == moduleID {
		ui.Row(c).AlignItems(ui.Center).Gap(float32(tok.Spacing4)).Children(func() {
			ui.Box(c).Width(float32(tok.Spacing48) * 4).Children(func() {
				tb := components.TextBox(c, &v.titleEditBuf, components.TextBoxOptions{}).Placeholder("模块标题")
				tb.OnTextKey(func(k ui.TextKey) bool {
					if k.Key == ui.KeyEnter && k.Mods == 0 {
						next := strings.TrimSpace(v.titleEditBuf)
						if next != "" && next != title {
							v.emitAnyModuleEdit(moduleID, map[string]any{"title": next})
						}
						v.titleEditID = ""
						return true
					}
					return false
				})
			})
			if components.Button(c, "确定", components.ButtonSecondary).Clicked() {
				next := strings.TrimSpace(v.titleEditBuf)
				if next != "" && next != title {
					v.emitAnyModuleEdit(moduleID, map[string]any{"title": next})
				}
				v.titleEditID = ""
			}
			if components.IconButton(c, lucide.X(), "取消改名", components.IconButtonCompact).Clicked() {
				v.titleEditID = ""
			}
		})
		return
	}
	ui.Row(c).AlignItems(ui.Center).Gap(float32(tok.Spacing4)).Children(func() {
		ui.Text(c, title).FontSize(float32(tok.TextSection)).Bold().TextColor(tok.Text)
		editBtn := components.IconButton(c, lucide.Pencil(), "修改标题", components.IconButtonCompact)
		components.Tooltip(c, editBtn, "自定义模块标题")
		if editBtn.Clicked() {
			v.titleEditID = moduleID
			v.titleEditBuf = title
		}
	})
}

func (v *View) renderProse(c *ui.Context, data map[string]any) {
	t := c.Theme()
	tok := theme.GetTokens(t)
	moduleID := textValue(data, "id")
	title := textValue(data, "title")
	tone := textValue(data, "tone")
	if tone == "" {
		tone = "plain"
	}
	body := textValue(data, "body")
	editable := v.Mode == ModeDesign && !v.ReadOnly && v.Can(PermDocumentEdit)

	var icon *ui.SVG
	var badgeLabel string
	var barColor *ui.Color
	bgColor := tok.Surface
	switch tone {
	case "warning":
		icon = lucide.TriangleAlert()
		badgeLabel = "关键警示"
		col := tok.Warning
		barColor = &col
		bgColor = tok.Warning.Alpha(.07)
	case "tip":
		icon = lucide.Lightbulb()
		badgeLabel = "操作提示"
		col := tok.Accent
		barColor = &col
		bgColor = tok.AccentSoftBg
	case "note":
		icon = lucide.Info()
		badgeLabel = "备注"
		col := tok.InfoText
		barColor = &col
		bgColor = tok.Elevated
	default:
		icon = lucide.FileText()
		badgeLabel = "说明"
	}

	if v.UnifiedLive {
		if editable {
			bodyBuf := v.ensureWidgetDraft("prose:"+moduleID+":body", body)
			var wrapper ui.Element
			if barColor != nil {
				wrapper = ui.Row(c.Key("prose-ed:"+moduleID)).FillWidth().Radius(float32(tok.ControlRadius)).Background(bgColor)
			} else {
				wrapper = ui.Column(c.Key("prose-ed:"+moduleID)).FillWidth().Gap(float32(tok.Spacing4))
			}
			hovered := wrapper.Hovered()
			var tb ui.Element
			hasTB := false
			renderToneBar := func() {
				toneOp := float32(0)
				if hovered || tone != "plain" || v.MobileCompact {
					toneOp = 1
				}
				bar := ui.Row(c.Key("prose-tone:"+moduleID)).FillWidth().AlignItems(ui.Center).Gap(float32(tok.Spacing4))
				if bar.AnimateTo("pt-op", toneOp, theme.MotionMicroFade) > 0.05 {
					bar.Opacity(bar.AnimateTo("pt-op", toneOp, theme.MotionMicroFade)).Children(func() {
						toneChoice := tone
						sel := components.Select(c, &toneChoice)
						StyleSelectQuiet(c, tok, sel)
						sel.Trigger.Children(func() {
							ui.Text(c, proseToneLabel(toneChoice)).FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
						})
						sel.Popup(func(panel ui.Element) {
							for _, opt := range []string{"plain", "note", "tip", "warning"} {
								opt := opt
								sel.Item(opt).Children(func() {
									ui.Text(c, proseToneLabel(opt)).FontSize(float32(tok.TextMeta)).TextColor(tok.Text)
								})
							}
						})
						if sel.Trigger.Changed() && toneChoice != tone {
							v.emitAnyModuleEdit(moduleID, map[string]any{"tone": toneChoice})
						}
					})
				}
			}
			if barColor != nil {
				wrapper.Children(func() {
					ui.Box(c).Width(t.Space(0.75)).FillHeight().Background(*barColor)
					ui.Column(c).Grow(1).MinWidth(0).Padding(float32(tok.Spacing8)).Gap(float32(tok.Spacing4)).Children(func() {
						renderToneBar()
						tb = ui.TextAreaBase(c, bodyBuf).FillWidth().Lines(1, 10).
							Placeholder("输入操作步骤说明或注意事项（新行输入 / 可插入组件）…").TextColor(tok.Text)
						hasTB = true
					})
				})
			} else {
				wrapper.Children(func() {
					renderToneBar()
					tb = ui.TextAreaBase(c, bodyBuf).FillWidth().Lines(1, 12).
						Placeholder("输入文字说明（新行输入 / 可插入组件）…").TextColor(tok.Text)
					hasTB = true
				})
			}
			// Slash trigger when typing "/" on an empty prose block or a standalone "/" line
			if hasTB && v.OnInsertStepBlock != nil {
				if nextBody, triggered := stripStandaloneSlashLine(*bodyBuf); triggered {
					*bodyBuf = nextBody
					v.openSlashMenuForBlock(c, tb, moduleID)
				}
			}
			if hasTB && !tb.Focused() && *bodyBuf != body {
				v.emitAnyModuleEdit(moduleID, map[string]any{"body": *bodyBuf})
			}
			return
		}
		// Clean paper reading prose without form borders (supports basic bold/list formatting)
		renderCleanProseBody := func() {
			for _, para := range strings.Split(body, "\n") {
				line := strings.TrimSpace(para)
				if line == "" {
					continue
				}
				isHeading := false
				if strings.HasPrefix(line, "### ") {
					line = strings.TrimSpace(strings.TrimPrefix(line, "### "))
					isHeading = true
				} else if strings.HasPrefix(line, "- ") {
					line = "• " + strings.TrimSpace(strings.TrimPrefix(line, "- "))
				}
				txt := ui.Text(c, line).TextColor(tok.Text)
				if isHeading {
					txt.FontSize(float32(tok.TextSection)).Bold()
				} else {
					txt.FontSize(float32(tok.TextBody))
				}
			}
		}
		if barColor != nil {
			ui.Row(c).FillWidth().Radius(float32(tok.ControlRadius)).Background(bgColor).Children(func() {
				ui.Box(c).Width(t.Space(0.75)).FillHeight().Background(*barColor)
				ui.Column(c).FillWidth().Gap(float32(tok.Spacing4)).Padding(float32(tok.Spacing8), float32(tok.Spacing12)).Children(renderCleanProseBody)
			})
		} else {
			ui.Column(c).FillWidth().Gap(float32(tok.Spacing4)).Children(renderCleanProseBody)
		}
		return
	}

	if editable {
		moduleCardFrame(v, c, icon, firstNonEmpty(title, "文本 / 注意事项"), badgeLabel, barColor, func() {
			titleBuf := v.ensureWidgetDraft("prose:"+moduleID+":title", title)
			bodyBuf := v.ensureWidgetDraft("prose:"+moduleID+":body", body)
			toneChoice := tone
			ui.Row(c).FillWidth().Gap(float32(tok.Spacing8)).AlignItems(ui.Center).Children(func() {
				ui.Box(c).FillWidth().Children(func() {
					components.TextBox(c, titleBuf, components.TextBoxOptions{}).Placeholder("标题（可选）")
				})
				sel := components.Select(c, &toneChoice)
				sel.Trigger.Children(func() {
					ui.Text(c, proseToneLabel(toneChoice)).FontSize(float32(tok.TextMeta)).TextColor(tok.Text)
				})
				sel.Popup(func(panel ui.Element) {
					for _, opt := range []string{"plain", "note", "tip", "warning"} {
						opt := opt
						sel.Item(opt).Children(func() {
							ui.Text(c, proseToneLabel(opt)).FontSize(float32(tok.TextMeta)).TextColor(tok.Text)
						})
					}
				})
				if sel.Trigger.Changed() {
					v.emitAnyModuleEdit(moduleID, map[string]any{"tone": toneChoice})
				}
			})
			components.TextBox(c, bodyBuf, components.TextBoxOptions{Multiline: true}).
				Placeholder("输入操作说明、注意事项或 SOP 片段…").Lines(3, 8)
			if strings.TrimSpace(*bodyBuf) != "" && (*bodyBuf != body || *titleBuf != title) {
				if components.Button(c, "应用文本", components.ButtonSecondary).Clicked() {
					props := map[string]any{"body": *bodyBuf}
					if strings.TrimSpace(*titleBuf) != "" {
						props["title"] = strings.TrimSpace(*titleBuf)
					}
					v.emitAnyModuleEdit(moduleID, props)
				}
			}
		})
		return
	}

	ui.Row(c).FillWidth().Radius(float32(tok.PanelRadius)).Background(bgColor).
		Border(1, tok.Border).Children(func() {
		if barColor != nil {
			ui.Box(c).Width(t.Space(0.75)).FillHeight().Background(*barColor)
		}
		ui.Column(c).FillWidth().Gap(float32(tok.Spacing4)).Padding(float32(tok.Spacing12)).Children(func() {
			if title != "" || tone != "plain" {
				ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
					ui.Icon(c, icon).Size(float32(tok.Spacing16), float32(tok.Spacing16)).TextColor(tok.TextMuted)
					if title != "" {
						ui.Text(c, title).FontSize(float32(tok.TextSection)).Bold().TextColor(tok.Text)
					} else {
						ui.Text(c, badgeLabel).FontSize(float32(tok.TextMeta)).Bold().TextColor(tok.TextMuted)
					}
					ui.Spacer(c)
					if tone != "plain" {
						toneBadge(c, tone, badgeLabel)
					}
				})
			}
			for _, para := range strings.Split(body, "\n") {
				line := strings.TrimSpace(para)
				if line == "" {
					continue
				}
				if strings.HasPrefix(line, "- ") {
					line = "• " + strings.TrimSpace(strings.TrimPrefix(line, "- "))
				}
				ui.Text(c, line).FontSize(float32(tok.TextBody)).TextColor(tok.Text)
			}
		})
	})
}

func (v *View) openSlashMenuForBlock(c *ui.Context, anchor ui.Element, moduleID string) {
	st := ui.Local(c.Root(), "slash-menu-step:"+v.StepID+":"+moduleID, func() bool { return false })
	*st = true
}

func stripStandaloneSlashLine(raw string) (string, bool) {
	if strings.TrimSpace(raw) == "/" {
		return "", true
	}
	lines := strings.Split(raw, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if strings.TrimSpace(lines[i]) == "/" {
			kept := append(lines[:i], lines[i+1:]...)
			return strings.Join(kept, "\n"), true
		}
	}
	return raw, false
}

func proseToneLabel(tone string) string {
	switch tone {
	case "note":
		return "备注"
	case "tip":
		return "操作提示"
	case "warning":
		return "关键警示"
	default:
		return "普通段落"
	}
}

func toneBadge(c *ui.Context, tone, label string) {
	switch tone {
	case "warning":
		components.Badge(c, label, components.BadgeOptions{Tone: components.BadgeWarning})
	case "tip":
		components.Badge(c, label, components.BadgeOptions{Tone: components.BadgeAccent})
	default:
		components.Badge(c, label)
	}
}

func firstNonEmpty(values ...string) string {
	for _, s := range values {
		if strings.TrimSpace(s) != "" {
			return s
		}
	}
	return ""
}
