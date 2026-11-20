package labsticker

import (
	"fmt"
	"image"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/onevra/source/components"
	"github.com/oneo-me/onevra/source/icons/lucide"

	"asterlab-mygo-prototype/internal/theme"
)

// ItemStatus describes stock/expiry urgency displayed beneath each shelf sticker.
type ItemStatus string

const (
	StatusOK         ItemStatus = "ok"
	StatusOpened     ItemStatus = "opened"
	StatusNearExpiry ItemStatus = "near_expiry"
	StatusExpired    ItemStatus = "expired"
	StatusExhausted  ItemStatus = "exhausted"
)

// ShelfItem represents one reagent or storage box placed as a physical sticker on a shelf.
type ShelfItem struct {
	ID             string
	Name           string
	Location       string
	Vessel         VesselID
	CustomImage    image.Image // optional custom user bitmap auto-wrapped in die-cut white border
	Status         ItemStatus
	MetaLine       string // concise subtitle e.g. "500 mL · 4°C 门架"
	BadgeText      string // optional urgency or occupancy badge above sticker
	TooltipText    string // optional full detail string on hover
	HasManual      bool   // shows compact datasheet indicator on shelf label
	HasURL         bool   // shows compact vendor link indicator on shelf label
	IsBox          bool   // true for cryoboxes / tube racks that drill down into slot grids
	OccupancyText  string // e.g. "4/81 孔"
	MatchHighlight bool   // highlights box card when search matches a reagent inside
	HeightScale    float32
}

// ShelfTier groups stickers belonging to one physical storage shelf level or temperature cabinet.
type ShelfTier struct {
	Title    string
	Subtitle string
	ZoneKey  string
	Items    []ShelfItem
}

// ShelfRenderOptions configures interactive selection, quick-add slots, and compact cubby layout.
type ShelfRenderOptions struct {
	SelectedID     string
	CompactCubby   bool
	OnSelect       func(ShelfItem)
	OnAddInTier    func(ShelfTier)
	OnAddBoxInTier func(ShelfTier)
}

// SlotWidthDIP returns the natural horizontal footprint of a shelf slot so small tubes
// cluster comfortably instead of stretching awkwardly across wide monitors.
func SlotWidthDIP(item ShelfItem, metrics StickerMetrics) float32 {
	if item.IsBox || item.Vessel == VesselPCRStrip || item.Vessel == VesselErlenmeyer250 || metrics.WidthDIP > 88 {
		return 156
	}
	return 142
}

// RenderSticker renders a single white-bordered vessel sticker at its natural aspect.
func RenderSticker(c *ui.Context, id VesselID, customImg image.Image, opts StickerOptions) ui.Element {
	bmp, metrics := resolveStickerBitmap(id, customImg, opts)
	if bmp == nil {
		return ui.Box(c)
	}
	return ui.Image(c, bmp).Size(metrics.WidthDIP, metrics.HeightDIP)
}

func resolveStickerBitmap(id VesselID, customImg image.Image, opts StickerOptions) (*ui.Bitmap, StickerMetrics) {
	if customImg != nil {
		return StickerFromImage(customImg, opts)
	}
	bmp, metrics, err := Sticker(id, opts)
	if err != nil || bmp == nil {
		return nil, StickerMetrics{}
	}
	return bmp, metrics
}

func tierZoneIcon(title string) *ui.SVG {
	lower := strings.ToLower(title)
	switch {
	case strings.Contains(lower, "-80") || strings.Contains(lower, "超低温") || strings.Contains(lower, "液氮"):
		return lucide.Snowflake()
	case strings.Contains(lower, "-20") || strings.Contains(lower, "4°c") || strings.Contains(lower, "冷藏") || strings.Contains(lower, "冻"):
		return lucide.ThermometerSnowflake()
	case strings.Contains(lower, "避光") || strings.Contains(lower, "常温") || strings.Contains(lower, "干燥"):
		return lucide.SunDim()
	default:
		return lucide.Archive()
	}
}

func itemHeightDIP(item ShelfItem) float32 {
	spec, _ := Lookup(item.Vessel)
	h := spec.DefaultHeight
	if h <= 0 {
		h = 84
	}
	if item.HeightScale > 0 {
		h *= item.HeightScale
	}
	return h
}

// RenderShelfGrid arranges storage zones ergonomically: adjacent compact zones (<= 3 items)
// pair into balanced side-by-side cabinet bays, while fuller zones span the cabinet width.
func RenderShelfGrid(c *ui.Context, tiers []ShelfTier, opts ShelfRenderOptions) {
	t := c.Theme()
	ui.Column(c).FillWidth().Gap(t.Space(4)).Children(func() {
		for i := 0; i < len(tiers); {
			cur := tiers[i]
			canPair := len(cur.Items) <= 3 && i+1 < len(tiers) && len(tiers[i+1].Items) <= 3
			if canPair {
				next := tiers[i+1]
				ui.Row(c).FillWidth().Wrap().Gap(t.Space(4)).AlignItems(ui.Stretch).Children(func() {
					pairOpts := opts
					pairOpts.CompactCubby = true
					ui.Column(c).Grow(1).Basis(440).MinWidth(340).Children(func() {
						RenderShelfTierWithOptions(c, cur, pairOpts)
					})
					ui.Column(c).Grow(1).Basis(440).MinWidth(340).Children(func() {
						RenderShelfTierWithOptions(c, next, pairOpts)
					})
				})
				i += 2
				continue
			}
			singleOpts := opts
			singleOpts.CompactCubby = false
			RenderShelfTierWithOptions(c, cur, singleOpts)
			i++
		}
	})
}

// RenderShelfTier renders one visual laboratory shelf tier with natural vessel clustering,
// recessed cabinet shadow, beveled 3D shelf deck, and integrated shelf-clip label rail.
func RenderShelfTier(c *ui.Context, tier ShelfTier, onSelect func(ShelfItem)) {
	RenderShelfTierWithOptions(c, tier, ShelfRenderOptions{OnSelect: onSelect})
}

// RenderShelfTierWithOptions renders a laboratory cabinet tier configured by ShelfRenderOptions.
func RenderShelfTierWithOptions(c *ui.Context, tier ShelfTier, opts ShelfRenderOptions) {
	t := c.Theme()
	tok := theme.GetTokens(t)

	// Top-lit recessed cabinet interior & beveled deck colors derived strictly from tokens.
	nicheTop := tok.Canvas.Mix(tok.BorderStrong, 0.24)
	nicheBottom := tok.Surface.Mix(tok.Canvas, 0.42)
	plankTop := tok.Surface.Mix(tok.Elevated, 0.35)
	plankBottom := tok.Elevated.Mix(tok.BorderStrong, 0.48)
	plankEdge := tok.BorderStrong.Mix(tok.TextMuted, 0.30)
	railBg := tok.Elevated.Mix(tok.Canvas, 0.55)

	maxPerShelf := 6
	if opts.CompactCubby {
		maxPerShelf = 3
	}

	ui.Column(c).FillWidth().Padding(t.Space(3)).Radius(float32(tok.PanelRadius)).
		Background(tok.Surface).Border(1, tok.Border).
		Shadow(0, 1, 3, 0, tok.Text.Alpha(0.05)).Gap(t.Space(2)).Children(func() {
		// Cabinet zone header
		ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(t.Space(2)).Children(func() {
			ui.Box(c).Padding(t.Space(1)).Radius(float32(tok.ControlRadius)).
				Background(tok.AccentSoftBg).Children(func() {
				ui.Icon(c, tierZoneIcon(tier.Title)).Size(t.Space(4), t.Space(4)).TextColor(tok.Accent)
			})
			ui.Column(c).Grow(1).MinWidth(0).Gap(t.Space(0)).Children(func() {
				ui.Text(c, tier.Title).FontSize(float32(tok.TextSection)).Bold().TextColor(tok.Text).
					SingleLine().Ellipsis("…")
				if tier.Subtitle != "" {
					ui.Text(c, tier.Subtitle).FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted).
						SingleLine().Ellipsis("…")
				}
			})
			if opts.OnAddBoxInTier != nil {
				if components.Button(c, "+ 放盒", components.ButtonGhost).Clicked() {
					opts.OnAddBoxInTier(tier)
				}
			}
			if opts.OnAddInTier != nil {
				if components.Button(c, "+ 上架", components.ButtonGhost).Clicked() {
					opts.OnAddInTier(tier)
				}
			}
			components.Badge(c, fmt.Sprintf("%d 件在架", len(tier.Items)), components.BadgeOptions{Tone: components.BadgeNeutral})
		})

		// Recessed shelf cubby stack
		if len(tier.Items) == 0 {
			renderEmptyShelfBay(c, tok, nicheTop, nicheBottom, plankTop, plankBottom, plankEdge, railBg, tier, opts)
			return
		}

		for start := 0; start < len(tier.Items); start += maxPerShelf {
			end := min(start+maxPerShelf, len(tier.Items))
			rowItems := tier.Items[start:end]

			type renderedSlot struct {
				item    ShelfItem
				bmp     *ui.Bitmap
				metrics StickerMetrics
				slotW   float32
			}
			slots := make([]renderedSlot, 0, len(rowItems))
			for _, it := range rowItems {
				bmp, metrics := resolveStickerBitmap(it.Vessel, it.CustomImage, StickerOptions{
					TargetHeightDIP: itemHeightDIP(it),
					BorderDIP:       4.0,
					Exhausted:       it.Status == StatusExhausted,
				})
				slots = append(slots, renderedSlot{
					item:    it,
					bmp:     bmp,
					metrics: metrics,
					slotW:   SlotWidthDIP(it, metrics),
				})
			}

			ui.Column(c).FillWidth().Radius(float32(tok.ControlRadius)).
				Border(1, tok.Border).
				LinearGradient(ui.LinearGradient{Angle: 180, Oklab: true, From: nicheTop, To: nicheBottom}).
				InsetShadow(0, 2, 5, 0, tok.Text.Alpha(0.06)).
				Gap(t.Space(0)).Children(func() {
				// 1. Vessels clustered naturally from the left edge with uniform stage height so paired cabinet shelves align
				ui.Row(c).FillWidth().MinHeight(float32(tok.Spacing48)*2.6).AlignItems(ui.End).Gap(t.Space(2)).
					Padding(t.Space(3), t.Space(3), t.Space(0), t.Space(3)).Children(func() {
					for _, s := range slots {
						s := s
						slotBtn := ui.ButtonBase(c.Key("shelf-item-"+s.item.ID)).Label(s.item.Name).
							Width(s.slotW).Justify(ui.End).
							Padding(t.Space(1), t.Space(0), t.Space(0), t.Space(0)).
							Radius(float32(tok.ControlRadius)).
							OnClick(func() {
								if opts.OnSelect != nil {
									opts.OnSelect(s.item)
								}
							})
						if slotBtn.Pressed() {
							slotBtn.Translate(0, 1)
						} else if slotBtn.Hovered() {
							slotBtn.Translate(0, -2)
							slotBtn.Background(tok.AccentSoftBg)
						}
						slotBtn.Children(func() {
							ui.Column(c).FillWidth().AlignItems(ui.Center).Gap(t.Space(0)).Children(func() {
								topBadge := s.item.BadgeText
								topTone := shelfStatusTone(s.item.Status)
								if s.item.IsBox && topBadge == "" && s.item.OccupancyText != "" {
									topBadge = s.item.OccupancyText
									topTone = components.BadgeAccent
								}
								if topBadge != "" {
									ui.Row(c).FillWidth().Justify(ui.Center).Padding(t.Space(0), t.Space(0), t.Space(1), t.Space(0)).Children(func() {
										components.Badge(c, topBadge, components.BadgeOptions{Tone: topTone})
									})
								}
								if s.bmp != nil {
									ui.Image(c, s.bmp).Size(s.metrics.WidthDIP, s.metrics.HeightDIP)
								}
								// Contact shadow directly beneath the bottle or box foot
								footW := clampFloat32(s.metrics.WidthDIP*0.72, 26, 84)
								ui.Box(c).Width(footW).Height(t.Space(1)).Radius(float32(tok.ControlRadius)).
									Background(tok.Text.Alpha(0.14))
							})
						})
						tip := strings.TrimSpace(s.item.TooltipText)
						if tip == "" && s.item.MetaLine != "" {
							tip = s.item.Name + " · " + s.item.MetaLine + "（点击查看档案与说明书）"
						}
						if tip != "" {
							components.Tooltip(c, slotBtn, tip)
						}
					}
				})

				// 2. Dimensional 3D beveled shelf ledge + soft under-ledge drop shadow
				ui.Column(c).FillWidth().Gap(t.Space(0)).Children(func() {
					ui.Box(c).FillWidth().Height(t.Space(2)).
						LinearGradient(ui.LinearGradient{Angle: 180, Oklab: true, From: plankTop, To: plankBottom}).
						Border(1, plankEdge).
						InsetShadow(0, 1, 0, 0, tok.Surface.Alpha(0.85))
					ui.Box(c).FillWidth().Height(t.Space(1)).
						LinearGradient(ui.LinearGradient{Angle: 180, Oklab: true, From: tok.Text.Alpha(0.09), To: tok.Text.Alpha(0.0)})
				})

				// 3. Integrated shelf-edge label strip aligned 1:1 with the vessels above
				ui.Row(c).FillWidth().AlignItems(ui.Start).Gap(t.Space(2)).
					Background(railBg).
					Padding(t.Space(2), t.Space(3), t.Space(2), t.Space(3)).Children(func() {
					for _, s := range slots {
						s := s
						spec, _ := Lookup(s.item.Vessel)
						selected := opts.SelectedID != "" && opts.SelectedID == s.item.ID
						metaColor := shelfMetaTextColor(tok, s.item.Status)
						tagBg := tok.Surface
						tagBorder := tok.Border
						if selected || s.item.MatchHighlight {
							tagBg = tok.AccentSoftBg
							tagBorder = tok.Accent
						}
						lblPrefix := "打开 "
						lblSuffix := " 详情"
						if s.item.IsBox {
							lblPrefix = "打开盒子 "
							lblSuffix = ""
						}
						card := ui.ButtonBase(c.Key("shelf-lbl-"+s.item.ID)).Label(lblPrefix + s.item.Name + lblSuffix).
							Width(s.slotW).
							Padding(t.Space(1), t.Space(1)).
							Radius(float32(tok.ControlRadius)).
							Background(tagBg).Border(1, tagBorder).
							Shadow(0, 1, 2, 0, tok.Text.Alpha(0.05)).
							OnClick(func() {
								if opts.OnSelect != nil {
									opts.OnSelect(s.item)
								}
							})
						if card.Hovered() && !selected && !s.item.MatchHighlight {
							card.Border(1, tok.BorderStrong)
						}
						card.Children(func() {
							ui.Column(c).FillWidth().AlignItems(ui.Center).Gap(t.Space(0)).Children(func() {
								ui.Text(c, s.item.Name).FontSize(float32(tok.TextMeta)).Bold().
									TextColor(tok.Text).SingleLine().Ellipsis("…").MinWidth(0)
								meta := strings.TrimSpace(s.item.MetaLine)
								if meta == "" {
									meta = spec.VolumeLabel
								}
								ui.Row(c).FillWidth().Justify(ui.Center).AlignItems(ui.Center).Gap(t.Space(1)).Children(func() {
									if s.item.IsBox {
										ui.Icon(c, lucide.Grid3x3()).Size(t.Space(3), t.Space(3)).TextColor(tok.Accent)
									}
									ui.Text(c, meta).FontSize(t.Rem(.82)).FontFeatures("tnum").
										TextColor(metaColor).SingleLine().Ellipsis("…").MinWidth(0)
									if s.item.HasManual {
										ui.Icon(c, lucide.FileText()).Size(t.Space(3), t.Space(3)).TextColor(tok.Accent)
									}
									if s.item.HasURL {
										ui.Icon(c, lucide.Link()).Size(t.Space(3), t.Space(3)).TextColor(tok.TextMuted)
									}
								})
							})
						})
					}
				})
			})
		}
	})
}

func renderEmptyShelfBay(
	c *ui.Context,
	tok theme.Tokens,
	nicheTop, nicheBottom, plankTop, plankBottom, plankEdge, railBg ui.Color,
	tier ShelfTier,
	opts ShelfRenderOptions,
) {
	t := c.Theme()
	ui.Column(c).FillWidth().Radius(float32(tok.ControlRadius)).Border(1, tok.Border).
		LinearGradient(ui.LinearGradient{Angle: 180, Oklab: true, From: nicheTop, To: nicheBottom}).
		InsetShadow(0, 2, 5, 0, tok.Text.Alpha(0.06)).Gap(t.Space(0)).Children(func() {
		ui.Row(c).FillWidth().Height(84).AlignItems(ui.Center).Justify(ui.Center).Gap(t.Space(2)).Children(func() {
			ui.Text(c, "本层暂无在架试剂").FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
			if opts.OnAddInTier != nil {
				if components.Button(c, "上架试剂", components.ButtonSecondary).Clicked() {
					opts.OnAddInTier(tier)
				}
			}
		})
		ui.Box(c).FillWidth().Height(t.Space(2)).
			LinearGradient(ui.LinearGradient{Angle: 180, Oklab: true, From: plankTop, To: plankBottom}).
			Border(1, plankEdge)
		ui.Box(c).FillWidth().Height(t.Space(4)).Background(railBg)
	})
}

func shelfStatusTone(status ItemStatus) components.BadgeTone {
	switch status {
	case StatusNearExpiry:
		return components.BadgeWarning
	case StatusExpired, StatusExhausted:
		return components.BadgeDanger
	case StatusOpened:
		return components.BadgeAccent
	default:
		return components.BadgeNeutral
	}
}

func shelfMetaTextColor(tok theme.Tokens, status ItemStatus) ui.Color {
	switch status {
	case StatusNearExpiry:
		return tok.WarningText
	case StatusExpired, StatusExhausted:
		return tok.DangerText
	default:
		return tok.TextMuted
	}
}

func clampFloat32(v, lo, hi float32) float32 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// RenderStickerCatalogSheet renders the complete 12-vessel BioRender-style die-cut
// sticker showcase grid for documentation and visual inspection.
func RenderStickerCatalogSheet(c *ui.Context) {
	t := c.Theme()
	tok := theme.GetTokens(t)
	specs := AllVessels()
	stageTop := tok.Canvas.Mix(tok.BorderStrong, 0.22)
	stageBottom := tok.Surface.Mix(tok.Canvas, 0.42)
	plankTop := tok.Surface.Mix(tok.Elevated, 0.35)
	plankBottom := tok.Elevated.Mix(tok.BorderStrong, 0.45)
	plankEdge := tok.BorderStrong.Mix(tok.TextMuted, 0.28)

	ui.Column(c).Fill().Background(tok.Canvas).Padding(t.Space(6)).Gap(t.Space(3)).Children(func() {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(t.Space(2)).Children(func() {
			ui.Column(c).Gap(t.Space(0)).Children(func() {
				ui.Text(c, "BioRender 风实验室容器白边贴纸图鉴（12 款标准器皿 · 2D SDF 刀模白边）").
					FontSize(float32(tok.TextSection)).Bold().TextColor(tok.Text)
				ui.Text(c, "基于 NIH BioArt Source (Public Domain 2024–2025) 精准器皿骨架，经清透渐变调色、闭孔洪水填充与欧氏距离场白边刀模合成。").
					FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
			})
			ui.Spacer(c)
			components.Badge(c, "uikit/labsticker", components.BadgeOptions{Tone: components.BadgeAccent})
		})

		const cols = 4
		for i := 0; i < len(specs); i += cols {
			end := min(i+cols, len(specs))
			ui.Row(c).FillWidth().Gap(t.Space(3)).AlignItems(ui.Stretch).Children(func() {
				for _, spec := range specs[i:end] {
					spec := spec
					ui.Column(c).Grow(1).Basis(0).MinWidth(0).
						Padding(t.Space(3)).Radius(float32(tok.PanelRadius)).
						Background(tok.Surface).Border(1, tok.Border).
						Shadow(0, 1, 2, 0, tok.Text.Alpha(0.05)).
						Gap(t.Space(2)).Children(func() {
						ui.Column(c).FillWidth().Height(124).Justify(ui.End).AlignItems(ui.Center).
							Radius(float32(tok.ControlRadius)).
							LinearGradient(ui.LinearGradient{Angle: 180, Oklab: true, From: stageTop, To: stageBottom}).
							InsetShadow(0, 2, 4, 0, tok.Text.Alpha(0.05)).
							Border(1, tok.Border).
							Padding(t.Space(2), t.Space(2), t.Space(0), t.Space(2)).Children(func() {
							RenderSticker(c, spec.ID, nil, StickerOptions{
								TargetHeightDIP: spec.DefaultHeight,
								BorderDIP:       4.0,
							})
							ui.Box(c).FillWidth().Height(t.Space(2)).Radius(float32(tok.ControlRadius)).
								LinearGradient(ui.LinearGradient{Angle: 180, Oklab: true, From: plankTop, To: plankBottom}).
								Border(1, plankEdge)
							ui.Box(c).FillWidth().Height(t.Space(1))
						})

						ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(t.Space(1)).Children(func() {
							ui.Text(c, spec.Title).FontSize(float32(tok.TextBody)).Bold().TextColor(tok.Text).
								Grow(1).MinWidth(0).SingleLine().Ellipsis("…")
							components.Badge(c, spec.VolumeLabel, components.BadgeOptions{Tone: components.BadgeNeutral})
						})
						ui.Text(c, spec.ExampleUsage).FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted).
							SingleLine().Ellipsis("…")
					})
				}
			})
		}
	})
}

// RenderBoxCatalogSheet renders the 5 BioRender-style storage box & tube rack die-cut stickers.
func RenderBoxCatalogSheet(c *ui.Context) {
	t := c.Theme()
	tok := theme.GetTokens(t)
	specs := AllBoxes()
	stageTop := tok.Canvas.Mix(tok.BorderStrong, 0.22)
	stageBottom := tok.Surface.Mix(tok.Canvas, 0.42)
	plankTop := tok.Surface.Mix(tok.Elevated, 0.35)
	plankBottom := tok.Elevated.Mix(tok.BorderStrong, 0.45)
	plankEdge := tok.BorderStrong.Mix(tok.TextMuted, 0.28)

	ui.Column(c).Fill().Background(tok.Canvas).Padding(t.Space(6)).Gap(t.Space(3)).Children(func() {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(t.Space(2)).Children(func() {
			ui.Column(c).Gap(t.Space(0)).Children(func() {
				ui.Text(c, "储藏室二级收纳盒与离心管架贴纸（5 类冻存盒 / EP 盒 / 离心管架 / PCR 盒）").
					FontSize(float32(tok.TextSection)).Bold().TextColor(tok.Text)
				ui.Text(c, "用于在温区货架陈列收纳盒，点击任意收纳盒贴纸即可下钻查看盒内孔位坐标与试剂排布。").
					FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
			})
			ui.Spacer(c)
			components.Badge(c, "uikit/labsticker", components.BadgeOptions{Tone: components.BadgeAccent})
		})

		ui.Row(c).FillWidth().Wrap().Gap(t.Space(3)).AlignItems(ui.Stretch).Children(func() {
			for _, spec := range specs {
				spec := spec
				ui.Column(c).Grow(1).Basis(float32(tok.Spacing48)*4.5).MinWidth(float32(tok.Spacing48)*3.8).
					Padding(t.Space(3)).Radius(float32(tok.PanelRadius)).
					Background(tok.Surface).Border(1, tok.Border).
					Shadow(0, 1, 2, 0, tok.Text.Alpha(0.05)).
					Gap(t.Space(2)).Children(func() {
					ui.Column(c).FillWidth().Height(float32(tok.Spacing48)*2.7).Justify(ui.End).AlignItems(ui.Center).
						Radius(float32(tok.ControlRadius)).
						LinearGradient(ui.LinearGradient{Angle: 180, Oklab: true, From: stageTop, To: stageBottom}).
						InsetShadow(0, 2, 4, 0, tok.Text.Alpha(0.05)).
						Border(1, tok.Border).
						Padding(t.Space(2), t.Space(2), t.Space(0), t.Space(2)).Children(func() {
						RenderSticker(c, spec.ID, nil, StickerOptions{
							TargetHeightDIP: spec.DefaultHeight,
							BorderDIP:       4.0,
						})
						ui.Box(c).FillWidth().Height(t.Space(2)).Radius(float32(tok.ControlRadius)).
							LinearGradient(ui.LinearGradient{Angle: 180, Oklab: true, From: plankTop, To: plankBottom}).
							Border(1, plankEdge)
						ui.Box(c).FillWidth().Height(t.Space(1))
					})

					ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(t.Space(1)).Children(func() {
						ui.Text(c, spec.Title).FontSize(float32(tok.TextBody)).Bold().TextColor(tok.Text).
							Grow(1).MinWidth(0).SingleLine().Ellipsis("…")
						components.Badge(c, spec.VolumeLabel, components.BadgeOptions{Tone: components.BadgeAccent})
					})
					ui.Text(c, spec.ExampleUsage).FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted).
						SingleLine().Ellipsis("…")
				})
			}
		})
	})
}
