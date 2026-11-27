package kit

import (
	"fmt"

	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/onevra/source/components"
	"github.com/oneo-me/onevra/source/icons/lucide"

	"asterlab-mygo-prototype/internal/theme"
)

// ContactSheetItem represents one square specimen/photo tile in a museum-style contact-sheet grid.
type ContactSheetItem struct {
	ID           string
	Title        string
	FormatBadge  string
	RoleBadge    string
	CapturedAt   string
	Image        ui.ImageSource
	HeroMatchKey any
	RenderThumb  func(c *ui.Context)
}

// ContactSheetBatch groups contact-sheet tiles under a tabular timeline/batch header.
type ContactSheetBatch struct {
	ID          string
	Title       string
	Subtitle    string
	ActionLabel string
	OnAction    func()
	Items       []ContactSheetItem
}

// ContactSheetOptions configures grid responsiveness, darkroom surface mode, and selection.
type ContactSheetOptions struct {
	Darkroom      bool
	Columns       int
	SelectedIDs   map[string]bool
	SelectionOn   bool
	ActiveHeroKey any
	EmptyTitle    string
	EmptySub      string
	OnItemClick   func(item ContactSheetItem)
}

// ContactSheetGrid renders a Halide / Apple Photos style 2px-gap square thumbnail grid
// grouped by timeline batch headers, with zero noisy tile borders, bottom-right translucent
// DNG/role badges, and optional DarkroomCanvas seamless background.
func ContactSheetGrid(c *ui.Context, tok theme.Tokens, batches []ContactSheetBatch, opts ContactSheetOptions) {
	if c == nil {
		return
	}

	canvasBg := tok.Canvas
	primaryFg := tok.Text
	mutedFg := tok.TextMuted
	tileBg := tok.Elevated
	dividerCol := tok.HairlineDivider
	selAccent := tok.Accent
	selCheckFg := tok.AccentText
	if opts.Darkroom {
		canvasBg = tok.DarkroomCanvas
		primaryFg = tok.DarkroomText
		mutedFg = tok.DarkroomMuted
		tileBg = tok.DarkroomElevated
		dividerCol = tok.DarkroomBorder
		selAccent = tok.DarkroomText
		selCheckFg = tok.DarkroomCanvas
	}

	root := ui.Column(c.Key("kit-contact-sheet")).FillWidth().Background(canvasBg).Gap(float32(tok.Spacing16))
	availW, _ := c.Size()
	if bw := root.Bounds().W; bw > 0 && (availW <= 0 || bw < availW) {
		availW = bw
	}
	cols := opts.Columns
	if cols <= 0 {
		switch {
		case availW > 0 && availW < mobileContextBreakpoint:
			cols = 3
		case availW >= 960:
			cols = 5
		default:
			cols = 4
		}
	}
	tileGap := float32(tok.Spacing4) * 0.5

	root.Children(func() {
		totalItems := 0
		for _, b := range batches {
			totalItems += len(b.Items)
		}
		if totalItems == 0 {
			title := opts.EmptyTitle
			if title == "" {
				title = "暂无实验图样"
			}
			sub := opts.EmptySub
			if sub == "" {
				sub = "拍摄或导入显微图像后将按批次时间线排列于此。"
			}
			ui.Column(c).FillWidth().Padding(float32(tok.Spacing24), float32(tok.Spacing16)).
				Center().Gap(float32(tok.Spacing8)).Children(func() {
				ui.Icon(c, lucide.Images()).Size(float32(tok.IconLg), float32(tok.IconLg)).TextColor(mutedFg)
				ui.Text(c, title).FontSize(float32(tok.TextBody)).Bold().TextColor(primaryFg)
				ui.Text(c, sub).FontSize(float32(tok.TextMeta)).TextColor(mutedFg)
			})
			return
		}

		for bi, batch := range batches {
			if len(batch.Items) == 0 {
				continue
			}
			batch := batch
			batchKey := batch.ID
			if batchKey == "" {
				batchKey = fmt.Sprintf("batch-%d", bi)
			}
			ui.Column(c.Key("cs-batch-" + batchKey)).FillWidth().Gap(float32(tok.Spacing8)).Children(func() {
				// Batch timeline header with tnum count + optional right-side action
				ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
					ui.Column(c).Grow(1).MinWidth(0).Gap(float32(tok.Spacing4) * 0.5).Children(func() {
						ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
							ui.Text(c, batch.Title).FontSize(float32(tok.TextData)).Bold().
								FontFeatures("tnum").TextColor(primaryFg).SingleLine().Ellipsis("…").MinWidth(0)
							ui.Text(c, fmt.Sprintf("%d 张", len(batch.Items))).
								FontSize(float32(tok.TextMeta)).FontFeatures("tnum").TextColor(mutedFg).SingleLine().Shrink(0)
						})
						if batch.Subtitle != "" {
							ui.Text(c, batch.Subtitle).FontSize(float32(tok.TextMeta)).FontFeatures("tnum").
								TextColor(mutedFg).SingleLine().Ellipsis("…")
						}
					})
					if batch.ActionLabel != "" && batch.OnAction != nil {
						if opts.Darkroom {
							actBtn := ui.ButtonBase(c.Key("cs-act-" + batchKey)).
								Padding(float32(tok.Spacing4), float32(tok.Spacing8)).
								Radius(float32(tok.ControlRadius)).
								Background(tok.DarkroomSurface).
								Border(1, tok.DarkroomBorder).
								Label(batch.ActionLabel)
							if actBtn.Hovered() || actBtn.Pressed() {
								actBtn.Background(tok.DarkroomElevated)
							}
							actBtn.Children(func() {
								ui.Text(c, batch.ActionLabel).FontSize(float32(tok.TextMeta)).Bold().TextColor(tok.DarkroomText).SingleLine()
							})
							if actBtn.Clicked() {
								batch.OnAction()
							}
						} else if components.Button(c.Key("cs-act-"+batchKey), batch.ActionLabel, components.ButtonGhost).Clicked() {
							batch.OnAction()
						}
					}
				})
				ui.Box(c).FillWidth().Height(1).Background(dividerCol)

				// Tight 2px gap square museum grid (zero outer tile borders/shadows)
				ui.Grid(c.Key("cs-grid-" + batchKey)).FillWidth().Columns(cols).Gap(tileGap).Children(func() {
					for ii, item := range batch.Items {
						item := item
						itemKey := item.ID
						if itemKey == "" {
							itemKey = fmt.Sprintf("%s-%d", batchKey, ii)
						}
						lbl := item.Title
						if lbl == "" {
							lbl = itemKey
						}
						selected := opts.SelectedIDs != nil && opts.SelectedIDs[item.ID]
						tile := ui.ButtonBase(c.Key("cs-tile-" + itemKey)).
							FillWidth().AspectRatio(1).Clip().
							Background(tileBg).
							Label(lbl)
						if item.HeroMatchKey != nil && item.HeroMatchKey != opts.ActiveHeroKey {
							tile.Transition(theme.HeroFlyTransition(item.HeroMatchKey))
						} else if item.HeroMatchKey == nil {
							tile.Transition(theme.ContactSheetCardTransition())
						}
						if tile.Pressed() {
							tile.Opacity(0.86)
						}
						tile.Children(func() {
							if item.RenderThumb != nil {
								ui.Box(c).Fill().Children(func() {
									item.RenderThumb(c)
								})
							} else if item.Image != nil {
								ui.Image(c, item.Image).Fill().Fit(ui.Cover)
							} else {
								ui.Box(c).Fill().Center().Children(func() {
									ui.Icon(c, lucide.Image()).Size(float32(tok.IconMd), float32(tok.IconMd)).TextColor(mutedFg)
								})
							}

							// Bottom-right translucent DNG / role badge
							if item.FormatBadge != "" || item.RoleBadge != "" {
								badgeText := item.FormatBadge
								if item.RoleBadge != "" && item.FormatBadge != "" {
									badgeText = item.RoleBadge + " · " + item.FormatBadge
								} else if item.RoleBadge != "" {
									badgeText = item.RoleBadge
								}
								ui.Row(c).Absolute().Bottom(float32(tok.Spacing4)).Right(float32(tok.Spacing4)).
									Padding(0, float32(tok.Spacing4)).
									Radius(tok.InnerControlRadius(tok.Spacing4 / 2)).
									Background(tok.DarkroomBadgeBg).
									IgnoreInput(true).Children(func() {
									ui.Text(c, badgeText).FontSize(float32(tok.TextMeta)).FontFeatures("tnum").
										TextColor(tok.DarkroomText).SingleLine()
								})
							}

							// Top-right crisp selection indicator when SelectionOn or SelectedIDs active
							if opts.SelectionOn || selected {
								dotSize := float32(tok.Spacing16) + float32(tok.Spacing4)
								dot := ui.Box(c).Absolute().Top(float32(tok.Spacing4)).Right(float32(tok.Spacing4)).
									Size(dotSize, dotSize).
									Radius(float32(tok.PillRadius)).
									Center().
									IgnoreInput(true)
								if selected {
									dot.Background(selAccent)
									dot.Children(func() {
										ui.Icon(c, lucide.Check()).Size(float32(tok.Spacing12), float32(tok.Spacing12)).TextColor(selCheckFg)
									})
								} else {
									dot.Background(tok.DarkroomBadgeBg).Border(1, tok.DarkroomText.Alpha(0.75))
								}
							}
						})

						if selected {
							col := selAccent
							tile.DrawOver(func(p *ui.Painter, r ui.Rect) {
								if r.W > 4 && r.H > 4 {
									p.Stroke(ui.Rect{X: r.X + 1, Y: r.Y + 1, W: r.W - 2, H: r.H - 2}, col, 0, 2)
								}
							})
						}

						if tile.Clicked() && opts.OnItemClick != nil {
							opts.OnItemClick(item)
						}
					}
				})
			})
		}
	})
}
