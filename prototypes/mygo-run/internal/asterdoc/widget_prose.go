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
	ui.Row(c).FillWidth().Radius(float32(tok.PanelRadius)).Background(tok.Surface).
		Border(1, tok.Border).Children(func() {
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
