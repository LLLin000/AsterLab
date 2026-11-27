package kit

import (
	"fmt"
	"math"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/onevra/source/icons/lucide"

	"asterlab-mygo-prototype/internal/theme"
)

const (
	lightboxDefaultDismissY = float32(110)
	lightboxMinZoom         = float32(1.0)
	lightboxDoubleZoom      = float32(2.0)
	lightboxMaxZoom         = float32(3.0)
)

// DarkroomLightboxAction describes one quiet footer action pill inside DarkroomLightbox.
type DarkroomLightboxAction struct {
	ID       string
	Label    string
	Icon     *ui.SVG
	Primary  bool
	Disabled bool
	OnClick  func()
}

// DarkroomLightboxState tracks interactive zoom, HUD chrome visibility, and swipe-to-dismiss state.
type DarkroomLightboxState struct {
	Zoom         float32
	ChromeHidden bool
	DragOffset   ui.Point
	draggingPrev bool
}

// DarkroomLightboxProps configures the darkroom specimen/image viewer.
type DarkroomLightboxProps struct {
	Key          any
	Title        string
	FormatBadge  string
	MetaSummary  string
	Caption      string
	HeroMatchKey any
	Image        ui.ImageSource
	RenderImage  func(c *ui.Context, zoom float32)
	Actions      []DarkroomLightboxAction
	OnClose      func()
}

func computeLightboxDismissMetrics(dragY, winH float32) (dismissThreshold, scrimAlpha, dragScale float32) {
	dismissThreshold = lightboxDefaultDismissY
	if winH > 0 {
		if cand := winH * 0.16; cand < dismissThreshold {
			dismissThreshold = max(72, cand)
		}
	}
	if dragY <= 0 {
		return dismissThreshold, 1.0, 1.0
	}
	progress := float32(math.Min(1.0, float64(dragY/(dismissThreshold*2.2))))
	scrimAlpha = 1.0 - progress*0.65
	if scrimAlpha < 0.30 {
		scrimAlpha = 0.30
	}
	scaleProgress := float32(math.Min(1.0, float64(dragY/(dismissThreshold*2.6))))
	dragScale = 1.0 - scaleProgress*0.12
	if dragScale < 0.88 {
		dragScale = 0.88
	}
	return dismissThreshold, scrimAlpha, dragScale
}

// DarkroomLightbox renders an immersive darkroom photo/specimen viewer inside ui.Overlay
// with vertical swipe-to-dismiss at 1.0x zoom, single-tap HUD chrome toggle, double-click
// 1.0x <-> 2.0x zoom, and a tabular (tnum) scientific footer strip.
func DarkroomLightbox(c *ui.Context, tok theme.Tokens, open *bool, st *DarkroomLightboxState, props DarkroomLightboxProps) {
	if c == nil || open == nil {
		return
	}
	if !*open {
		if st != nil {
			st.DragOffset = ui.Point{}
			st.draggingPrev = false
			st.ChromeHidden = false
			st.Zoom = lightboxMinZoom
		}
		return
	}

	rootKey := props.Key
	if rootKey == nil {
		rootKey = "kit-darkroom-lightbox"
	}

	ui.Overlay(c, func() {
		root := ui.Themed(c, theme.DarkTheme()).Key(rootKey).Absolute().Fill().Modal().Background(ui.Transparent)
		if st == nil {
			st = ui.Local(root, "lb-state", func() DarkroomLightboxState {
				return DarkroomLightboxState{Zoom: lightboxMinZoom}
			})
		}
		if st.Zoom < lightboxMinZoom {
			st.Zoom = lightboxMinZoom
		} else if st.Zoom > lightboxMaxZoom {
			st.Zoom = lightboxMaxZoom
		}

		closeLightbox := func() {
			*open = false
			st.DragOffset = ui.Point{}
			st.draggingPrev = false
			st.ChromeHidden = false
			st.Zoom = lightboxMinZoom
			if props.OnClose != nil {
				props.OnClose()
			}
		}

		if c.Shortcut(0, ui.KeyEscape) {
			closeLightbox()
			return
		}

		vw, vh := c.Size()
		compact := vw > 0 && vw < mobileContextBreakpoint
		topSafe, rightSafe, bottomSafe, leftSafe := c.SafeArea()
		dismissThreshold, scrimAlpha, dragScale := computeLightboxDismissMetrics(st.DragOffset.Y, vh)

		root.Children(func() {
			// Darkroom scrim backdrop fading smoothly during vertical swipe-down.
			ui.Box(c.Key("lb-scrim")).Absolute().Fill().
				Background(tok.DarkroomCanvas.Alpha(scrimAlpha)).
				Transition(theme.MobileOverlayFadeTransition())

			// Centered interactive image stage.
			stage := ui.ButtonBase(c.Key("lb-stage")).Absolute().Fill().Center().
				Label("暗房画布")
			if st.Zoom <= 1.01 {
				stage.Draggable(&st.DragOffset, ui.DraggableOptions{
					Axis: ui.Vertical,
					Bounds: &ui.Rect{
						Y: -float32(tok.Spacing48),
						H: max(float32(tok.Spacing48)*10, vh),
					},
					Snap: func(projected ui.Point) ui.Point {
						if projected.Y >= dismissThreshold {
							return ui.Point{Y: dismissThreshold + float32(tok.Spacing48)*3}
						}
						return ui.Point{}
					},
					Animation: theme.MotionLightbox,
				})
				curDrag := stage.DragOffset()
				dragging := stage.Dragging()
				_, scrimAlpha, dragScale = computeLightboxDismissMetrics(curDrag.Y, vh)
				if !dragging && (st.DragOffset.Y >= dismissThreshold || (st.draggingPrev && curDrag.Y >= dismissThreshold)) {
					closeLightbox()
					return
				}
				st.draggingPrev = dragging
			} else {
				st.DragOffset = ui.Point{}
				st.draggingPrev = false
			}

			if stage.DoubleClicked() {
				if st.Zoom > 1.05 {
					st.Zoom = lightboxMinZoom
				} else {
					st.Zoom = lightboxDoubleZoom
				}
			} else if stage.Clicked() && !stage.Dragging() {
				st.ChromeHidden = !st.ChromeHidden
			}

			// Visual image container with HeroFly or DarkroomLightbox transition and zoom/drag scale.
			stage.Children(func() {
				hero := ui.Column(c.Key("lb-hero")).Center().Clip()
				if props.HeroMatchKey != nil {
					hero.Transition(theme.HeroFlyTransition(props.HeroMatchKey))
				} else {
					hero.Transition(theme.DarkroomLightboxTransition())
				}
				if dragScale < 1.0 {
					hero.Scale(dragScale, dragScale)
				}
				hero.Children(func() {
					if props.RenderImage != nil {
						props.RenderImage(c, st.Zoom)
					} else if props.Image != nil {
						maxW := float32(840)
						maxH := float32(560)
						if vw > 0 {
							maxW = vw - float32(tok.Spacing32)
						}
						if vh > 0 {
							maxH = vh * 0.72
						}
						ui.Image(c, props.Image).
							Width(maxW * st.Zoom).
							Height(maxH * st.Zoom).
							Fit(ui.Contain).
							Radius(float32(tok.ControlRadius))
					} else {
						ui.Column(c).Width(float32(tok.Spacing48)*6).Height(float32(tok.Spacing48)*4).
							Center().Radius(float32(tok.PanelRadius)).
							Background(tok.DarkroomSurface).Border(1, tok.DarkroomBorder).
							Gap(float32(tok.Spacing8)).Children(func() {
							ui.Icon(c, lucide.Image()).Size(float32(tok.IconLg), float32(tok.IconLg)).TextColor(tok.DarkroomMuted)
							ui.Text(c, "暂无可预览图像").FontSize(float32(tok.TextMeta)).TextColor(tok.DarkroomMuted)
						})
					}
				})
			})

			// Immersive HUD Chrome (top title/zoom bar + bottom tabular scientific footer strip).
			// Rendered as direct siblings (never inside an IgnoreInput(true) parent) so buttons remain clickable when visible.
			chromeTarget := float32(1.0)
			if st.ChromeHidden {
				chromeTarget = 0.0
			}
			chromeAlpha := root.AnimateTo("lb-chrome-alpha", chromeTarget, theme.MotionMicroFade)
			if chromeAlpha > 0.01 {
				// Top floating header strip
				topBar := ui.Row(c.Key("lb-topbar")).Absolute().Top(0).Left(0).Right(0).
					Padding(topSafe+float32(tok.Spacing12), rightSafe+float32(tok.Spacing16), float32(tok.Spacing12), leftSafe+float32(tok.Spacing16)).
					AlignItems(ui.Center).Gap(float32(tok.Spacing8)).
					Background(tok.DarkroomCanvas.Alpha(0.76)).
					Opacity(chromeAlpha).
					IgnoreInput(st.ChromeHidden)
				topBar.Children(func() {
					ui.Row(c).Grow(1).MinWidth(0).AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
						title := props.Title
						if title == "" {
							title = "暗房检视"
						}
						ui.Text(c, title).FontSize(float32(tok.TextSection)).Bold().
							TextColor(tok.DarkroomText).SingleLine().Ellipsis("…").MinWidth(0)
						if props.FormatBadge != "" {
							renderDarkroomBadge(c, tok, props.FormatBadge, "")
						}
					})

					// Subtle zoom control pills (- / 100% / +)
					zoomTrack := ui.Row(c).AlignItems(ui.Center).
						Padding(float32(tok.Spacing4)*0.5).
						Radius(float32(tok.ControlRadius)).
						Background(tok.DarkroomSurface).
						Border(1, tok.DarkroomBorder).
						Gap(float32(tok.Spacing4)*0.5)
					zoomBtnRad := tok.InnerControlRadius(tok.Spacing4 / 2)
					zoomTrack.Children(func() {
						if renderDarkroomIconPill(c, "lb-zoom-out", tok, lucide.ZoomOut(), "缩小", zoomBtnRad).Clicked() {
							st.Zoom = max(lightboxMinZoom, st.Zoom-0.5)
						}
						zoomLbl := fmt.Sprintf("%.0f%%", st.Zoom*100)
						zoomBtn := ui.ButtonBase(c.Key("lb-zoom-reset")).
							Padding(float32(tok.Spacing4), float32(tok.Spacing8)).
							Radius(zoomBtnRad).
							Background(ui.Transparent).
							Label(zoomLbl)
						if zoomBtn.Hovered() {
							zoomBtn.Background(tok.DarkroomElevated)
						}
						zoomBtn.Children(func() {
							ui.Text(c, zoomLbl).FontSize(float32(tok.TextMeta)).FontFeatures("tnum").
								TextColor(tok.DarkroomText).SingleLine()
						})
						if zoomBtn.Clicked() {
							if st.Zoom > 1.05 {
								st.Zoom = lightboxMinZoom
							} else {
								st.Zoom = lightboxDoubleZoom
							}
						}
						if renderDarkroomIconPill(c, "lb-zoom-in", tok, lucide.ZoomIn(), "放大", zoomBtnRad).Clicked() {
							st.Zoom = min(lightboxMaxZoom, st.Zoom+0.5)
						}
					})

					if renderDarkroomIconPill(c, "lb-close", tok, lucide.X(), "关闭灯箱", float32(tok.ControlRadius)).Clicked() {
						mygo.Haptics.Selection()
						closeLightbox()
					}
				})

				// Bottom tabular scientific footer strip
				botWrap := ui.Column(c.Key("lb-botbar")).Absolute().Bottom(0).Left(0).Right(0).
					Padding(float32(tok.Spacing8), rightSafe+float32(tok.Spacing16), bottomSafe+float32(tok.Spacing12), leftSafe+float32(tok.Spacing16)).
					Opacity(chromeAlpha).
					IgnoreInput(st.ChromeHidden)
				botWrap.Children(func() {
					strip := ui.Column(c).FillWidth().
						Padding(float32(tok.Spacing12), float32(tok.Spacing16)).
						Radius(float32(tok.PanelRadius)).
						Background(tok.DarkroomSurface.Alpha(0.94)).
						Border(1, tok.DarkroomBorder).
						Gap(float32(tok.Spacing8))
					if tok.SpecularRim.A > 0 {
						rim := tok.SpecularRim
						rad := float32(tok.PanelRadius)
						strip.DrawOver(func(p *ui.Painter, r ui.Rect) {
							if r.W > 4 && r.H > 4 {
								p.Stroke(ui.Rect{X: r.X + 1, Y: r.Y + 1, W: r.W - 2, H: r.H - 2}, rim, theme.ConcentricRadius(rad, 1), 1)
							}
						})
					}
					actionRad := tok.InnerPanelRadius(tok.Spacing8)
					strip.Children(func() {
						buildMeta := func() {
							ui.Column(c).Grow(1).MinWidth(0).Gap(float32(tok.Spacing4) * 0.5).Children(func() {
								metaLine := props.MetaSummary
								zoomStr := fmt.Sprintf("%.0f%%", st.Zoom*100)
								if metaLine == "" {
									metaLine = zoomStr
								} else {
									metaLine = metaLine + " · " + zoomStr
								}
								ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
									if props.FormatBadge != "" {
										renderDarkroomBadge(c, tok, props.FormatBadge, "")
									}
									ui.Text(c, metaLine).FontSize(float32(tok.TextData)).FontFeatures("tnum").
										TextColor(tok.DarkroomText).SingleLine().Ellipsis("…").Grow(1).MinWidth(0)
								})
								if props.Caption != "" {
									ui.Text(c, props.Caption).FontSize(float32(tok.TextMeta)).FontFeatures("tnum").
										TextColor(tok.DarkroomMuted).SingleLine().Ellipsis("…")
								}
							})
						}
						buildActions := func() {
							if len(props.Actions) == 0 {
								return
							}
							limit := len(props.Actions)
							if limit > 3 {
								limit = 3
							}
							ui.Row(c).Wrap().AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
								for i := 0; i < limit; i++ {
									act := props.Actions[i]
									if renderDarkroomActionBtn(c, tok, i, act, actionRad).Clicked() && act.OnClick != nil {
										act.OnClick()
									}
								}
							})
						}

						if compact {
							buildMeta()
							if len(props.Actions) > 0 {
								ui.Box(c).FillWidth().Height(1).Background(tok.DarkroomBorder)
								buildActions()
							}
						} else {
							ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(float32(tok.Spacing12)).Children(func() {
								buildMeta()
								buildActions()
							})
						}
					})
				})
			}
		})
	})
}

func renderDarkroomBadge(c *ui.Context, tok theme.Tokens, format, role string) {
	lbl := format
	if role != "" && format != "" {
		lbl = role + " · " + format
	} else if role != "" {
		lbl = role
	}
	if lbl == "" {
		return
	}
	ui.Row(c).Shrink(0).AlignSelf(ui.Center).
		Padding(float32(tok.Spacing4)*0.5, float32(tok.Spacing8)).
		Radius(tok.InnerControlRadius(tok.Spacing4/2)).
		Background(tok.DarkroomBadgeBg).
		Border(1, tok.DarkroomBorder).Children(func() {
		ui.Text(c, lbl).FontSize(float32(tok.TextMeta)).FontFeatures("tnum").
			TextColor(tok.DarkroomText).SingleLine()
	})
}

func renderDarkroomIconPill(c *ui.Context, key string, tok theme.Tokens, icon *ui.SVG, label string, rad float32) ui.Element {
	btn := ui.ButtonBase(c.Key(key)).
		Size(float32(tok.ButtonHeight), float32(tok.ButtonHeight)).
		Radius(rad).
		Background(tok.DarkroomSurface).
		Border(1, tok.DarkroomBorder).
		Center().
		Label(label)
	if btn.Pressed() {
		btn.Background(tok.DarkroomElevated)
	} else if btn.Hovered() {
		btn.Background(tok.DarkroomElevated)
	}
	btn.Children(func() {
		ui.Icon(c, icon).Size(float32(tok.IconSm), float32(tok.IconSm)).TextColor(tok.DarkroomText)
	})
	return btn
}

func renderDarkroomActionBtn(c *ui.Context, tok theme.Tokens, idx int, act DarkroomLightboxAction, rad float32) ui.Element {
	bg := tok.DarkroomElevated
	fg := tok.DarkroomText
	if act.Primary {
		bg = tok.DarkroomText
		fg = tok.DarkroomCanvas
	}
	btn := ui.ButtonBase(c.Key(fmt.Sprintf("lb-act-%d-%s", idx, act.ID))).
		Disabled(act.Disabled).
		MinHeight(float32(tok.ButtonHeight)).
		Padding(float32(tok.Spacing4), float32(tok.Spacing12)).
		Radius(rad).
		Background(bg).
		Border(1, tok.DarkroomBorder).
		AlignItems(ui.Center).
		Gap(float32(tok.Spacing4)).
		Label(act.Label)
	if act.Disabled {
		fg = tok.DarkroomMuted
	} else if btn.Pressed() {
		btn.Opacity(0.82)
	}
	btn.Children(func() {
		if act.Icon != nil {
			ui.Icon(c, act.Icon).Size(float32(tok.IconSm), float32(tok.IconSm)).TextColor(fg).Shrink(0)
		}
		ui.Text(c, act.Label).FontSize(float32(tok.TextData)).Bold().TextColor(fg).SingleLine()
	})
	return btn
}
