// Package workflownode draws one step of an experiment design as an Onevra
// card: the same soft surface, hairline border, 14-point corners and type
// scale as every other card in AsterLab.
package workflownode

import (
	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/onevra/source/components"
	"github.com/oneo-me/onevra/source/icons/lucide"
)

type NodeKind string

const (
	KindMaterial   NodeKind = "material"
	KindOperation  NodeKind = "operation"
	KindInstrument NodeKind = "instrument"
)

type NodeState string

const (
	StateDefault  NodeState = "default"
	StateHover    NodeState = "hover"
	StateSelected NodeState = "selected"
	StateError    NodeState = "error"
	StateDisabled NodeState = "disabled"
)

type ParamItem struct {
	Key string
	Val string
}

type NodeProps struct {
	ID        string
	Kind      NodeKind
	Order     string
	Tag       string
	Title     string
	Params    []ParamItem
	State     NodeState
	ErrorText string
}

const (
	// CardWidth and CardHeight are the card's unscaled size in DIPs. The
	// canvas positions nodes by these, so it must know them before layout.
	CardWidth  float32 = 224
	CardHeight float32 = 128
)

// View builds the card at the given canvas scale. The caller positions it; the
// card owns only its face and its click.
func View(c *ui.Context, props NodeProps, scale float32, onClick func()) ui.Element {
	if scale <= 0 {
		scale = 1
	}
	t := c.Theme()
	border := t.Border
	tagColor := t.TextMuted
	iconColor := t.Text
	switch props.State {
	case StateHover:
		border = t.Border.Mix(t.Text, .35)
	case StateSelected:
		border, tagColor, iconColor = t.Accent, t.Accent, t.Accent
	case StateError:
		border, tagColor = t.Danger, t.Danger
	case StateDisabled:
		tagColor, iconColor = t.TextMuted, t.TextMuted
	}
	radius := float32(14) * scale
	card := ui.Box(c).Size(CardWidth*scale, CardHeight*scale).Children(func() {
		ui.Column(c).Fill().Padding(t.Space(3)*scale, t.Space(3)*scale).
			Gap(t.Space(1.5)*scale).Radius(radius).Background(t.Surface).
			Border(1.5, border).
			Shadow(0, t.Space(.5), t.Space(3), 0, ui.RGBA(0, 0, 0, .05)).
			Children(func() {
				ui.Row(c).FillWidth().MinWidth(0).AlignItems(ui.Center).Children(func() {
					components.Text(c, tagText(props)).FontSize(t.Rem(.76) * scale).
						TextColor(tagColor).SingleLine()
					ui.Spacer(c)
					components.Text(c, orderText(props)).FontSize(t.Rem(.76) * scale).
						TextColor(t.TextMuted).Shrink(0)
				})
				ui.Row(c).Gap(t.Space(2) * scale).AlignItems(ui.Center).Children(func() {
					iconTile(c, props.Kind, scale, border, iconColor)
					ui.Column(c).Grow(1).MinWidth(0).Gap(t.Space(.5) * scale).Children(func() {
						components.Text(c, props.Title).FontSize(t.Rem(.95) * scale).
							FontWeight(500).SingleLine().Ellipsis("…")
					})
				})
				ui.Divider(c)
				params(c, props, scale)
			})
	})
	if props.State != StateDisabled && onClick != nil {
		card.OnClick(onClick)
	}
	return card
}

// iconTile is the small rounded tile before the title, matching Onevra's
// leading slots.
func iconTile(c *ui.Context, kind NodeKind, scale float32, border, color ui.Color) {
	t := c.Theme()
	size := t.Space(7) * scale
	ui.Box(c).Size(size, size).Shrink(0).Radius(t.Space(2)*scale).
		Background(t.Background).Border(1, border).Center().Children(func() {
		components.Icon(c, kindIcon(kind)).Size(size*.55, size*.55).Shrink(0).TextColor(color)
	})
}

// params lists up to two recorded parameters, the same muted key and plain
// value pairing the inspector uses.
func params(c *ui.Context, props NodeProps, scale float32) {
	t := c.Theme()
	if props.ErrorText != "" {
		components.Text(c, "⚠ "+props.ErrorText).FontSize(t.Rem(.78) * scale).
			TextColor(t.Danger).SingleLine().Ellipsis("…")
		return
	}
	if len(props.Params) == 0 {
		components.Text(c, "未填写参数").FontSize(t.Rem(.78) * scale).TextColor(t.TextMuted)
		return
	}
	for i, param := range props.Params {
		if i == 2 {
			break
		}
		ui.Row(c).FillWidth().MinWidth(0).Gap(t.Space(2) * scale).Children(func() {
			components.Text(c, param.Key).FontSize(t.Rem(.78) * scale).
				TextColor(t.TextMuted).SingleLine()
			ui.Spacer(c)
			components.Text(c, param.Val).FontSize(t.Rem(.78) * scale).
				SingleLine().Ellipsis("…").Shrink(0)
		})
	}
}

func tagText(props NodeProps) string {
	if props.Tag != "" {
		return props.Tag
	}
	return defaultKindTag(props.Kind)
}

func orderText(props NodeProps) string {
	if props.Order != "" {
		return props.Order
	}
	return "01"
}

func defaultKindTag(k NodeKind) string {
	switch k {
	case KindMaterial:
		return "材料 · 样本"
	case KindInstrument:
		return "分析仪器"
	default:
		return "操作步骤"
	}
}

func kindIcon(k NodeKind) *ui.SVG {
	switch k {
	case KindMaterial:
		return lucide.FlaskConical()
	case KindInstrument:
		return lucide.Microscope()
	}
	return lucide.Pipette()
}
