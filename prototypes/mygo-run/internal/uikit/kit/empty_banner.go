package kit

import (
	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/onevra/source/components"

	"asterlab-mygo-prototype/internal/theme"
)

// BannerTone classifies inline feedback banners.
type BannerTone uint8

const (
	BannerInfo BannerTone = iota
	BannerSuccess
	BannerWarning
	BannerDanger
)

// EmptyState renders a calm single-action empty state placeholder adhering to L3 minimalism.
func EmptyState(c *ui.Context, tok theme.Tokens, icon *ui.SVG, title, subtitle, actionLabel string, onAction func()) {
	box := ui.Column(c).FillWidth().
		Padding(float32(tok.Spacing24), float32(tok.Spacing16)).
		Center().Gap(float32(tok.Spacing8))
	ApplyElevation(box, tok, ElevationCard)
	box.Children(func() {
		if icon != nil {
			ui.Icon(c, icon).Size(float32(tok.IconLg), float32(tok.IconLg)).TextColor(tok.TextMuted)
		}
		if title != "" {
			ui.Text(c, title).FontSize(float32(tok.TextBody)).Bold().TextColor(tok.Text)
		}
		if subtitle != "" {
			ui.Text(c, subtitle).FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
		}
		if actionLabel != "" && onAction != nil {
			if components.Button(c, actionLabel, components.ButtonPrimary).Clicked() {
				onAction()
			}
		}
	})
}

// InlineBanner renders a token-backed inline status/error strip with optional recovery action.
func InlineBanner(c *ui.Context, tok theme.Tokens, icon *ui.SVG, tone BannerTone, message, actionLabel string, onAction func()) {
	bg, fg, bd := tok.InfoBg, tok.InfoText, tok.Border
	switch tone {
	case BannerSuccess:
		bg, fg, bd = tok.SuccessBg, tok.SuccessText, tok.Success
	case BannerWarning:
		bg, fg, bd = tok.WarningBg, tok.WarningText, tok.Warning
	case BannerDanger:
		bg, fg, bd = tok.DangerBg, tok.DangerText, tok.Danger
	}
	ui.Row(c).FillWidth().
		Padding(float32(tok.Spacing8), float32(tok.Spacing12)).
		Radius(float32(tok.ControlRadius)).
		Background(bg).Border(1, bd).
		AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
		if icon != nil {
			ui.Icon(c, icon).Size(float32(tok.IconSm), float32(tok.IconSm)).TextColor(fg).Shrink(0)
		}
		ui.Text(c, message).FontSize(float32(tok.TextMeta)).TextColor(fg).Grow(1).MinWidth(0)
		if actionLabel != "" && onAction != nil {
			if components.Button(c, actionLabel, components.ButtonGhost).Clicked() {
				onAction()
			}
		}
	})
}

// SectionHeader renders a uniform icon + bold title + optional count meta + trailing slot header.
func SectionHeader(c *ui.Context, tok theme.Tokens, icon *ui.SVG, title, meta string, rightSlot func()) {
	ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
		if icon != nil {
			ui.Icon(c, icon).Size(float32(tok.IconMd), float32(tok.IconMd)).TextColor(tok.Accent).Shrink(0)
		}
		if title != "" {
			ui.Text(c, title).FontSize(float32(tok.TextSection)).Bold().TextColor(tok.Text).SingleLine().Ellipsis("…")
		}
		if meta != "" {
			ui.Text(c, meta).FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted).FontFeatures("tnum").SingleLine().Shrink(0)
		}
		ui.Spacer(c)
		if rightSlot != nil {
			ui.Row(c).Shrink(0).AlignItems(ui.Center).Gap(float32(tok.Spacing4)).Children(rightSlot)
		}
	})
}
