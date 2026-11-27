package asterdoc

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/transfer"
	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/onevra/source/components"
	"github.com/oneo-me/onevra/source/icons/lucide"
	_ "golang.org/x/image/tiff"

	"asterlab-mygo-prototype/internal/theme"
	"asterlab-mygo-prototype/internal/uikit/kit"
)

type svgTextLabel struct {
	X        float64
	Y        float64
	Anchor   string
	Baseline string
	Fill     string
	Text     string
	Size     float64
}

var (
	reViewBox      = regexp.MustCompile(`viewBox\s*=\s*["']([^"']+)["']`)
	reSVGToken     = regexp.MustCompile(`(?s)<g\b([^>]*)>|</g>|<text\b([^>]*)>(.*?)</text>`)
	reTranslate    = regexp.MustCompile(`translate\(\s*([-+]?[0-9]*\.?[0-9]+)(?:[\s,]+([-+]?[0-9]*\.?[0-9]+))?\s*\)`)
	reAttrX        = regexp.MustCompile(`\bx\s*=\s*["']([^"']+)["']`)
	reAttrY        = regexp.MustCompile(`\by\s*=\s*["']([^"']+)["']`)
	reAttrSize     = regexp.MustCompile(`\bfont-size\s*=\s*["']([^"']+)["']`)
	reAttrAnch     = regexp.MustCompile(`\btext-anchor\s*=\s*["']([^"']+)["']`)
	reAttrFill     = regexp.MustCompile(`\bfill\s*=\s*["']([^"']+)["']`)
	reAttrBaseline = regexp.MustCompile(`\bdominant-baseline\s*=\s*["']([^"']+)["']`)
	reTagStrip     = regexp.MustCompile(`<[^>]+>`)
	reStripSVGText = regexp.MustCompile(`(?is)<text\b[^>]*>.*?</text>`)
)

func parseTranslateAttr(attrs string) (float64, float64) {
	m := reTranslate.FindStringSubmatch(attrs)
	if len(m) < 2 {
		return 0, 0
	}
	tx, _ := strconv.ParseFloat(m[1], 64)
	var ty float64
	if len(m) > 2 && strings.TrimSpace(m[2]) != "" {
		ty, _ = strconv.ParseFloat(m[2], 64)
	}
	return tx, ty
}

func estimateSVGTextWidth(text string, fsize float32) float32 {
	var units float32
	for _, r := range text {
		if r > 0x7F {
			units += 0.96
		} else {
			units += 0.54
		}
	}
	return units * fsize
}

func parseSVGMetricsAndLabels(rawSVG string) (vbX, vbY, vbW, vbH float64, labels []svgTextLabel) {
	vbW, vbH = 360, 260
	if m := reViewBox.FindStringSubmatch(rawSVG); len(m) == 2 {
		parts := strings.Fields(m[1])
		if len(parts) == 4 {
			if x, err := strconv.ParseFloat(parts[0], 64); err == nil {
				vbX = x
			}
			if y, err := strconv.ParseFloat(parts[1], 64); err == nil {
				vbY = y
			}
			if w, err := strconv.ParseFloat(parts[2], 64); err == nil && w > 0 {
				vbW = w
			}
			if h, err := strconv.ParseFloat(parts[3], 64); err == nil && h > 0 {
				vbH = h
			}
		}
	}
	var stack [][2]float64
	var curX, curY float64
	for _, match := range reSVGToken.FindAllStringSubmatch(rawSVG, -1) {
		tok := match[0]
		switch {
		case strings.HasPrefix(tok, "</g"):
			if n := len(stack); n > 0 {
				curX -= stack[n-1][0]
				curY -= stack[n-1][1]
				stack = stack[:n-1]
			}
		case strings.HasPrefix(tok, "<g"):
			gAttrs := match[1]
			if strings.HasSuffix(strings.TrimSpace(gAttrs), "/") {
				continue
			}
			tx, ty := parseTranslateAttr(gAttrs)
			stack = append(stack, [2]float64{tx, ty})
			curX += tx
			curY += ty
		default:
			attrs, body := match[2], strings.TrimSpace(reTagStrip.ReplaceAllString(match[3], ""))
			if body == "" {
				continue
			}
			var x, y float64
			size := 12.0
			anchor := "start"
			baseline := ""
			fillAttr := ""
			if xm := reAttrX.FindStringSubmatch(attrs); len(xm) == 2 {
				x, _ = strconv.ParseFloat(xm[1], 64)
			}
			if ym := reAttrY.FindStringSubmatch(attrs); len(ym) == 2 {
				y, _ = strconv.ParseFloat(ym[1], 64)
			}
			if sm := reAttrSize.FindStringSubmatch(attrs); len(sm) == 2 {
				size, _ = strconv.ParseFloat(strings.TrimSuffix(sm[1], "px"), 64)
			}
			if am := reAttrAnch.FindStringSubmatch(attrs); len(am) == 2 {
				anchor = am[1]
			}
			if bm := reAttrBaseline.FindStringSubmatch(attrs); len(bm) == 2 {
				baseline = strings.ToLower(strings.TrimSpace(bm[1]))
			}
			if fm := reAttrFill.FindStringSubmatch(attrs); len(fm) == 2 {
				fillAttr = strings.TrimSpace(fm[1])
			}
			labels = append(labels, svgTextLabel{
				X:        x + curX,
				Y:        y + curY,
				Anchor:   anchor,
				Baseline: baseline,
				Fill:     fillAttr,
				Text:     body,
				Size:     size,
			})
		}
	}
	return vbX, vbY, vbW, vbH, labels
}

func (v *View) renderFigure(c *ui.Context, data map[string]any) {
	t := c.Theme()
	tok := theme.GetTokens(t)
	moduleID := textValue(data, "id")
	title := firstNonEmpty(textValue(data, "title"), "示意图 / 实验照片")
	caption := textValue(data, "caption")
	src := textValue(data, "src")
	svgRaw := textValue(data, "svg")
	maxH := float32(220)
	if mh, ok := numericValue(data["max_height"]); ok && mh >= 60 {
		maxH = float32(mh)
	}
	editableDesign := v.Mode == ModeDesign && !v.ReadOnly && v.Can(PermDocumentEdit)

	var annotations []svgTextLabel
	for _, am := range mapSlice(data["annotations"]) {
		ax, _ := numericValue(am["x"])
		ay, _ := numericValue(am["y"])
		lbl := textValue(am, "label")
		if lbl != "" {
			annotations = append(annotations, svgTextLabel{X: ax, Y: ay, Anchor: "middle", Text: lbl, Size: 11})
		}
	}

	badge := "补充视觉表达"
	if svgRaw != "" {
		badge = "矢量 SVG"
	} else if src != "" {
		badge = "本地图片"
	}

	if v.FigureZoom <= 0 {
		v.FigureZoom = 1
	}

	if v.UnifiedLive {
		v.renderUnifiedFigure(c, tok, moduleID, title, caption, src, svgRaw, annotations, maxH, editableDesign)
		return
	}

	card := ui.Column(c.Key("fig-card:"+moduleID)).FillWidth().Gap(float32(tok.Spacing8)).
		Padding(float32(tok.Spacing12), float32(tok.Spacing16))
	kit.ApplyElevation(card, tok, kit.ElevationCard)
	openFig := v.figureOpen && v.ExpandedFigureID == moduleID
	openLightbox := func() {
		v.ExpandedFigureID = moduleID
		v.FigureZoom = 1
		v.figureOpen = true
	}
	card.Children(func() {
		ui.Column(c).FillWidth().Gap(float32(tok.Spacing8)).Children(func() {
			ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
				ui.Icon(c, lucide.Image()).Size(float32(tok.Spacing16), float32(tok.Spacing16)).TextColor(tok.Accent).Shrink(0)
				ui.Row(c).Grow(1).MinWidth(0).AlignItems(ui.Center).Children(func() {
					renderInlineEditableTitle(v, c, moduleID, title, editableDesign)
				})
				inlineBadge(c, badge)
				zoomBtn := components.IconButton(c, lucide.Maximize2(), "放大查看", components.IconButtonCompact)
				components.Tooltip(c, zoomBtn, "放大查看示意图与文字标注")
				if zoomBtn.Clicked() {
					openLightbox()
				}
			})
			ui.Box(c).FillWidth().Height(1).Background(tok.BorderSubtle)
		})

		heroBox := ui.Column(c.Key("fig-inline-hero:" + moduleID)).FillWidth()
		if !openFig {
			heroBox.Transition(theme.HeroFlyTransition(figureHeroMatch{ID: moduleID}))
		}
		heroBox.Children(func() {
			v.drawFigureCanvasInteractive(c, moduleID, src, svgRaw, annotations, maxH, 1, openLightbox)
		})

		if caption != "" {
			ui.Text(c, caption).FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted).FontFeatures("tnum")
		}

		if editableDesign {
			openEd := v.ensureWidgetBool("fig_ed:" + moduleID)
			components.Collapsible(c, "编辑 SVG / 图片路径与图注（支持直接拖入本地文件）", openEd, components.CollapsibleOptions{}, func() {
				srcBuf := v.ensureWidgetDraft("fig:"+moduleID+":src", src)
				capBuf := v.ensureWidgetDraft("fig:"+moduleID+":cap", caption)
				svgBuf := v.ensureWidgetDraft("fig:"+moduleID+":svg", svgRaw)
				ui.Column(c).FillWidth().Gap(float32(tok.Spacing8)).Padding(float32(tok.Spacing8), 0, 0, 0).Children(func() {
					ui.Row(c).FillWidth().Gap(float32(tok.Spacing8)).AlignItems(ui.Center).Children(func() {
						ui.Box(c).FillWidth().Children(func() {
							components.TextBox(c, srcBuf, components.TextBoxOptions{}).Placeholder("本地图片路径（可选，支持直接拖入文件）")
						})
						if v.OnPickFigureFile != nil {
							if components.Button(c, "选择文件", components.ButtonOutline).Clicked() {
								v.OnPickFigureFile(v.StepID, moduleID)
							}
						}
					})
					components.TextBox(c, capBuf, components.TextBoxOptions{}).Placeholder("图注 caption")
					components.TextBox(c, svgBuf, components.TextBoxOptions{Multiline: true}).
						Placeholder("内嵌 <svg viewBox=\"0 0 360 260\">…</svg> 源码").Lines(4, 10)
					if components.Button(c, "应用示意图", components.ButtonSecondary).Clicked() {
						props := map[string]any{"caption": strings.TrimSpace(*capBuf)}
						if strings.TrimSpace(*srcBuf) != "" {
							props["src"] = strings.TrimSpace(*srcBuf)
						}
						if strings.TrimSpace(*svgBuf) != "" {
							props["svg"] = strings.TrimSpace(*svgBuf)
						}
						v.emitAnyModuleEdit(moduleID, props)
					}
				})
			})
		}
	})

	var heroKey any = figureHeroMatch{ID: moduleID}
	if strings.TrimSpace(src) == "" && strings.TrimSpace(svgRaw) == "" {
		heroKey = nil
	}
	v.renderDarkroomFigureLightbox(c, tok, figureLightboxOptions{
		Key:          "fig-darkroom-lb:" + moduleID,
		ModuleID:     moduleID,
		Title:        title,
		Src:          src,
		SVGRaw:       svgRaw,
		Annotations:  annotations,
		MetaSummary:  figureDefaultMetaSummary(src, svgRaw, annotations),
		Caption:      caption,
		HeroMatchKey: heroKey,
	})
	if editableDesign {
		if card.FileDragOver() {
			card.Border(2, tok.Accent)
		}
		if dropped := card.DroppedFiles(); len(dropped) > 0 {
			picked := dropped[0]
			srcVal := portableFigureAssetSrc(v.BaseDir, picked)
			props := map[string]any{"src": srcVal}
			if strings.EqualFold(filepath.Ext(picked), ".svg") {
				if rawBytes, readErr := os.ReadFile(picked); readErr == nil && strings.Contains(string(rawBytes), "<svg") {
					props["svg"] = string(rawBytes)
				}
			}
			v.emitAnyModuleEdit(moduleID, props)
		}
	}
}

func (v *View) renderUnifiedFigure(c *ui.Context, tok theme.Tokens, moduleID, title, caption, src, svgRaw string, annotations []svgTextLabel, maxH float32, editableDesign bool) {
	pasteFromClipboard := func() {
		if v.ReadOnly {
			return
		}
		data := c.Services().ReadClipboardData(transfer.PNG, transfer.FileList)
		if b, err := data.Read(transfer.PNG); err == nil && len(b) > 0 {
			fname := fmt.Sprintf("paste_%d.png", time.Now().UnixMilli())
			if v.OnImportAssetBytes != nil {
				if srcPath, impErr := v.OnImportAssetBytes(b, fname); impErr == nil && srcPath != "" {
					v.emitAnyModuleEdit(moduleID, map[string]any{"src": srcPath, "svg": ""})
					return
				}
			}
			if v.BaseDir != "" {
				assetsDir := filepath.Join(v.BaseDir, "assets")
				if mkErr := os.MkdirAll(assetsDir, 0o755); mkErr == nil {
					dest := filepath.Join(assetsDir, fname)
					if wErr := os.WriteFile(dest, b, 0o644); wErr == nil {
						v.emitAnyModuleEdit(moduleID, map[string]any{"src": "./assets/" + fname, "svg": ""})
						return
					}
				}
			}
		}
		if files, err := data.Files(); err == nil && len(files) > 0 {
			picked := files[0]
			srcVal := portableFigureAssetSrc(v.BaseDir, picked)
			if v.OnImportAssetBytes != nil {
				if raw, rErr := os.ReadFile(picked); rErr == nil && len(raw) > 0 {
					if srcPath, impErr := v.OnImportAssetBytes(raw, filepath.Base(picked)); impErr == nil && srcPath != "" {
						srcVal = srcPath
					}
				}
			}
			props := map[string]any{"src": srcVal}
			if strings.EqualFold(filepath.Ext(picked), ".svg") {
				if rawBytes, readErr := os.ReadFile(picked); readErr == nil && strings.Contains(string(rawBytes), "<svg") {
					props["svg"] = string(rawBytes)
				}
			}
			v.emitAnyModuleEdit(moduleID, props)
		}
	}

	if v.FigureZoom <= 0 {
		v.FigureZoom = 1
	}

	trimSVG := strings.TrimSpace(svgRaw)
	isPlaceholderSVG := trimSVG == "" || (strings.HasPrefix(trimSVG, "<svg") && strings.HasSuffix(trimSVG, "></svg>") && strings.Count(trimSVG, "<") == 2)
	hasGraphic := strings.TrimSpace(src) != "" || !isPlaceholderSVG
	card := ui.Column(c.Key("live-fig:"+moduleID)).Focusable().FillWidth().Gap(float32(tok.Spacing8)).
		Padding(float32(tok.Spacing12), float32(tok.Spacing16))
	kit.ApplyElevation(card, tok, kit.ElevationCard)

	cardHover := card.Hovered()
	if editableDesign && card.FileDragOver() {
		card.Border(2, tok.Accent)
	}
	openFig := v.figureOpen && v.ExpandedFigureID == moduleID
	openLightbox := func() {
		v.ExpandedFigureID = moduleID
		v.FigureZoom = 1
		v.figureOpen = true
	}
	card.Children(func() {
		if hasGraphic {
			heroBox := ui.Column(c.Key("fig-inline-hero:" + moduleID)).FillWidth()
			if !openFig {
				heroBox.Transition(theme.HeroFlyTransition(figureHeroMatch{ID: moduleID}))
			}
			heroBox.Children(func() {
				v.drawFigureCanvasInteractive(c, moduleID, src, svgRaw, annotations, maxH, 1, openLightbox)
			})
		} else {
			dropBox := ui.Row(c.Key("live-fig-empty:"+moduleID)).FillWidth().
				Padding(float32(tok.Spacing12), float32(tok.Spacing16)).
				Radius(tok.InnerPanelRadius(tok.Spacing8)).Background(tok.Elevated.Alpha(0.7)).Border(1, tok.BorderSubtle).
				AlignItems(ui.Center).Gap(float32(tok.Spacing12))
			dropBox.Children(func() {
				ui.Icon(c, lucide.Image()).Size(float32(tok.Spacing16), float32(tok.Spacing16)).TextColor(tok.TextMuted).Shrink(0)
				ui.Text(c, "拖入或点击添加实验图 / SVG").
					FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted).Grow(1).MinWidth(0).SingleLine().Ellipsis("…")
				ui.Row(c).Shrink(0).Gap(float32(tok.Spacing4)).Children(func() {
					if v.OnPickFigureFile != nil && editableDesign {
						if components.Button(c, "选择文件", components.ButtonGhost).Clicked() {
							v.OnPickFigureFile(v.StepID, moduleID)
						}
					}
					if editableDesign {
						if components.Button(c, "粘贴", components.ButtonGhost).Clicked() {
							pasteFromClipboard()
						}
					}
				})
			})
		}

		// Clean inline caption below figure (wraps naturally at rest; single click enters inline edit)
		capBuf := v.ensureWidgetDraft("fig:"+moduleID+":cap", caption)
		capFocusKey := "figcap:" + moduleID
		editingCap := editableDesign && (v.FocusedTableCell == capFocusKey || strings.TrimSpace(*capBuf) == "")
		showCapRow := hasGraphic || strings.TrimSpace(*capBuf) != "" || cardHover
		if showCapRow && (editableDesign || caption != "" || hasGraphic) {
			ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
				if editingCap {
					ui.Box(c).Grow(1).MinWidth(0).Children(func() {
						capPh := ""
						if cardHover || v.FocusedTableCell == capFocusKey || strings.TrimSpace(*capBuf) != "" {
							capPh = "添加图注…"
						}
						ti := BorderlessTextInput(c, tok, capBuf, float32(tok.TextMeta), false, capPh).
							TextColor(tok.TextMuted).FontFeatures("tnum")
						ti.FocusBind(&v.FocusedTableCell, capFocusKey)
						if ti.Submitted() {
							v.FocusedTableCell = ""
						}
						if ti.Submitted() || (!ti.Focused() && strings.TrimSpace(*capBuf) != strings.TrimSpace(caption)) {
							v.emitAnyModuleEdit(moduleID, map[string]any{"caption": strings.TrimSpace(*capBuf)})
						}
					})
				} else if strings.TrimSpace(*capBuf) != "" {
					if editableDesign {
						capBtn := ui.ButtonBase(c.Key("fig-cap-btn:" + moduleID)).
							Grow(1).MinWidth(0).Padding(float32(tok.Spacing4)*0.5, float32(tok.Spacing4)).
							Radius(float32(tok.ControlRadius)).Label("编辑图注")
						if capBtn.Hovered() {
							capBtn.Background(tok.Text.Alpha(0.04))
						}
						capBtn.Children(func() {
							ui.Text(c, *capBuf).FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted).FontFeatures("tnum").FillWidth()
						})
						if capBtn.Clicked() {
							v.FocusedTableCell = capFocusKey
						}
					} else {
						ui.Text(c, *capBuf).FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted).FontFeatures("tnum").Grow(1).MinWidth(0)
					}
				} else {
					ui.Spacer(c)
				}
				if hasGraphic {
					zoomOp := float32(0)
					if cardHover || v.MobileCompact {
						zoomOp = 1.0
					}
					zoomWrap := ui.Box(c.Key("fig-zoom-wrap:" + moduleID)).Shrink(0)
					zoomWrap.Opacity(zoomWrap.AnimateTo("fz-op", zoomOp, theme.MotionMicroFade)).Children(func() {
						if components.IconButton(c, lucide.Maximize2(), "放大查看", components.IconButtonCompact).Clicked() {
							openLightbox()
						}
					})
				}
			})
		}
	})

	if editableDesign {
		if card.Clicked() {
			card.Focus()
		}
		if card.Shortcut(ui.Cmd, ui.KeyV) {
			pasteFromClipboard()
		}
		if dropped := card.DroppedFiles(); len(dropped) > 0 {
			picked := dropped[0]
			srcVal := portableFigureAssetSrc(v.BaseDir, picked)
			if v.OnImportAssetBytes != nil {
				if raw, rErr := os.ReadFile(picked); rErr == nil && len(raw) > 0 {
					if srcPath, impErr := v.OnImportAssetBytes(raw, filepath.Base(picked)); impErr == nil && srcPath != "" {
						srcVal = srcPath
					}
				}
			}
			props := map[string]any{"src": srcVal}
			if strings.EqualFold(filepath.Ext(picked), ".svg") {
				if rawBytes, readErr := os.ReadFile(picked); readErr == nil && strings.Contains(string(rawBytes), "<svg") {
					props["svg"] = string(rawBytes)
				}
			}
			v.emitAnyModuleEdit(moduleID, props)
		}
	}

	svgEdOpen := v.ensureWidgetBool("fig_svg_dlg:" + moduleID)
	asterContextMenu(c, card, func(m *asterMenu) {
		if editableDesign && v.OnPickFigureFile != nil {
			if m.Item("从本地选择图片…").Chosen() {
				v.OnPickFigureFile(v.StepID, moduleID)
			}
		}
		if editableDesign {
			if m.Item("从剪贴板粘贴图片").Shortcut("Ctrl+V").Chosen() {
				pasteFromClipboard()
			}
		}
		if hasGraphic {
			if m.Item("放大查看").Chosen() {
				v.ExpandedFigureID = moduleID
				v.FigureZoom = 1
				v.figureOpen = true
			}
		}
		if editableDesign {
			m.Separator()
			if m.Item("编辑 SVG 源码…").Chosen() {
				*svgEdOpen = true
			}
			if hasGraphic && m.Item("清除当前图像").Chosen() {
				v.emitAnyModuleEdit(moduleID, map[string]any{"src": "", "svg": ""})
			}
		}
	})

	var heroKey any
	if hasGraphic {
		heroKey = figureHeroMatch{ID: moduleID}
	}
	v.renderDarkroomFigureLightbox(c, tok, figureLightboxOptions{
		Key:          "fig-darkroom-lb:" + moduleID,
		ModuleID:     moduleID,
		Title:        title,
		Src:          src,
		SVGRaw:       svgRaw,
		Annotations:  annotations,
		MetaSummary:  figureDefaultMetaSummary(src, svgRaw, annotations),
		Caption:      strings.TrimSpace(*v.ensureWidgetDraft("fig:"+moduleID+":cap", caption)),
		HeroMatchKey: heroKey,
	})

	if editableDesign {
		svgBuf := v.ensureWidgetDraft("fig:"+moduleID+":svg", svgRaw)
		if *svgEdOpen && isMobileAsterdocViewport(c, v) {
			renderMobileSVGEditorPage(c, tok, "mb-fig-svg-ed:"+moduleID, svgEdOpen, svgBuf, func(val string) {
				v.emitAnyModuleEdit(moduleID, map[string]any{"svg": val})
			})
		} else {
			ui.Box(c.Key("fig-svg-ed:"+moduleID)).Children(func() {
				components.Dialog(c, svgEdOpen, components.DialogOptions{Label: "编辑 SVG 矢量源码"}, func(panel ui.Element) {
					panel.Width(float32(tok.Spacing48)*11).Gap(float32(tok.Spacing12)).Children(func() {
						components.TextBox(c, svgBuf, components.TextBoxOptions{Multiline: true}).
							Placeholder("<svg viewBox=\"0 0 360 220\">…</svg>").Lines(8, 16)
						ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
							ui.Spacer(c)
							if components.Button(c, "取消", components.ButtonOutline).Clicked() {
								*svgEdOpen = false
							}
							if components.Button(c, "应用 SVG", components.ButtonPrimary).Clicked() {
								v.emitAnyModuleEdit(moduleID, map[string]any{"svg": strings.TrimSpace(*svgBuf)})
								*svgEdOpen = false
							}
						})
					})
				})
			})
		}
	}
}

func isMobileAsterdocViewport(c *ui.Context, v *View) bool {
	if (v != nil && v.MobileCompact) || runtime.GOOS == "ios" || runtime.GOOS == "android" {
		return true
	}
	if c != nil {
		if w, _ := c.Size(); w > 0 && w < 680 {
			return true
		}
	}
	return false
}

func renderResponsiveLightboxPage(
	c *ui.Context,
	key any,
	tok theme.Tokens,
	title string,
	onClose func(),
	toolbar func(),
	body func(maxCanvasH float32),
	footer func(),
) {
	ui.Overlay(c, func() {
		t := c.Theme()
		topSafe, rightSafe, bottomSafe, leftSafe := c.SafeArea()
		_, vh := c.Size()
		maxH := float32(tok.Spacing48) * 9
		if vh > 0 {
			maxH = max(float32(tok.Spacing48)*5, vh-topSafe-bottomSafe-float32(tok.Spacing48)*4)
		}
		ui.Column(c.Key(key)).Absolute().Fill().Modal().Background(tok.Canvas).Children(func() {
			ui.Row(c).FillWidth().
				Padding(t.Space(2)+topSafe, t.Space(4)+rightSafe, t.Space(2), t.Space(4)+leftSafe).
				MinHeight(t.Space(12)+topSafe).
				AlignItems(ui.Center).Gap(t.Space(2)).Background(tok.Surface).Children(func() {
				if components.Button(c.Key("mb-lb-close"), "关闭", components.ButtonGhost).Shrink(0).Clicked() && onClose != nil {
					onClose()
				}
				ui.Text(c, title).FontSize(float32(tok.TextSection)).Bold().TextColor(tok.Text).
					Grow(1).MinWidth(0).SingleLine().Ellipsis("…")
			})
			ui.Box(c).FillWidth().Height(1).Background(tok.BorderSubtle)
			if toolbar != nil {
				ui.Row(c).FillWidth().Wrap().
					Padding(t.Space(2), t.Space(4)+rightSafe, t.Space(2), t.Space(4)+leftSafe).
					AlignItems(ui.Center).Gap(t.Space(2)).Background(tok.Elevated).Children(toolbar)
				ui.Box(c).FillWidth().Height(1).Background(tok.BorderSubtle)
			}
			ui.Scroll(c.Key("mb-lb-scroll")).FillWidth().Grow(1).Basis(0).MinHeight(0).Clip().Children(func() {
				ui.Column(c).FillWidth().
					Padding(t.Space(3), t.Space(4)+rightSafe, t.Space(6)+bottomSafe, t.Space(4)+leftSafe).
					Gap(t.Space(3)).Children(func() {
					if body != nil {
						body(maxH)
					}
					if footer != nil {
						footer()
					}
				})
			})
		})
	})
}

type figureHeroMatch struct{ ID string }

type figureLightboxOptions struct {
	Key          string
	ModuleID     string
	Title        string
	Src          string
	SVGRaw       string
	Annotations  []svgTextLabel
	MetaSummary  string
	Caption      string
	HeroMatchKey any
	ExtraActions []kit.DarkroomLightboxAction
}

func figureFormatBadge(src, svgRaw string) string {
	trimSVG := strings.TrimSpace(svgRaw)
	if trimSVG != "" && !(strings.HasPrefix(trimSVG, "<svg") && strings.HasSuffix(trimSVG, "></svg>") && strings.Count(trimSVG, "<") == 2) {
		return "SVG"
	}
	ext := strings.TrimPrefix(strings.ToUpper(filepath.Ext(strings.TrimSpace(src))), ".")
	switch ext {
	case "JPEG":
		return "JPG"
	case "TIFF":
		return "TIF"
	default:
		return ext
	}
}

func figureDefaultMetaSummary(src, svgRaw string, annotations []svgTextLabel) string {
	var parts []string
	if trimSrc := strings.TrimSpace(src); trimSrc != "" {
		parts = append(parts, filepath.Base(trimSrc))
	} else if strings.TrimSpace(svgRaw) != "" {
		_, _, vbW, vbH, _ := CachedSVGMetrics(svgRaw)
		parts = append(parts, fmt.Sprintf("矢量 %.0f×%.0f", vbW, vbH))
	}
	if len(annotations) > 0 {
		parts = append(parts, fmt.Sprintf("%d 处标注", len(annotations)))
	}
	return strings.Join(parts, " · ")
}

func applyDarkSpecularRim(el ui.Element, tok theme.Tokens, rad float32) {
	if tok.Dark && tok.SpecularRim.A > 0 {
		rim := tok.SpecularRim
		el.DrawOver(func(p *ui.Painter, r ui.Rect) {
			if r.W > 4 && r.H > 4 {
				p.Stroke(ui.Rect{X: r.X + 1, Y: r.Y + 1, W: r.W - 2, H: r.H - 2}, rim, theme.ConcentricRadius(rad, 1), 1)
			}
		})
	}
}

func (v *View) renderDarkroomFigureLightbox(c *ui.Context, tok theme.Tokens, opts figureLightboxOptions) {
	openFig := v.figureOpen && v.ExpandedFigureID == opts.ModuleID
	if !openFig {
		return
	}

	trimSVG := strings.TrimSpace(opts.SVGRaw)
	hasSVG := trimSVG != "" && !(strings.HasPrefix(trimSVG, "<svg") && strings.HasSuffix(trimSVG, "></svg>") && strings.Count(trimSVG, "<") == 2)

	var imgSrc ui.ImageSource
	var loadErr string
	if !hasSVG && strings.TrimSpace(opts.Src) != "" {
		resolved := resolveFigurePath(v.BaseDir, opts.Src)
		imgSrc, loadErr = v.cachedImageFile(resolved)
	}

	title := firstNonEmpty(strings.TrimSpace(opts.Title), filepath.Base(strings.TrimSpace(opts.Src)), "示意图 / 实验照片")
	formatBadge := figureFormatBadge(opts.Src, opts.SVGRaw)

	actions := make([]kit.DarkroomLightboxAction, 0, len(opts.ExtraActions)+2)
	actions = append(actions, opts.ExtraActions...)
	if strings.TrimSpace(opts.Src) != "" && v.OnOpenExternalAsset != nil {
		srcPath := opts.Src
		modID := opts.ModuleID
		actions = append(actions, kit.DarkroomLightboxAction{
			ID:    "open-external",
			Label: "系统打开",
			Icon:  lucide.ExternalLink(),
			OnClick: func() {
				v.OnOpenExternalAsset(AssetThumb{
					AssetID:  modID,
					Label:    title,
					FilePath: resolveFigurePath(v.BaseDir, srcPath),
				})
			},
		})
	}
	if strings.TrimSpace(opts.Src) != "" && v.OnOpenVaultLink != nil {
		srcPath := opts.Src
		actions = append(actions, kit.DarkroomLightboxAction{
			ID:    "locate-vault",
			Label: "定位文件",
			Icon:  lucide.FolderOpen(),
			OnClick: func() {
				v.OnOpenVaultLink(srcPath)
			},
		})
	}

	props := kit.DarkroomLightboxProps{
		Key:          opts.Key,
		Title:        title,
		FormatBadge:  formatBadge,
		MetaSummary:  opts.MetaSummary,
		Caption:      opts.Caption,
		HeroMatchKey: opts.HeroMatchKey,
		Actions:      actions,
		OnClose: func() {
			v.figureOpen = false
			v.FigureZoom = 1
		},
	}

	if !hasSVG && len(opts.Annotations) == 0 && imgSrc != nil {
		props.Image = imgSrc
	} else if hasSVG || strings.TrimSpace(opts.Src) != "" {
		props.RenderImage = func(lc *ui.Context, zoom float32) {
			v.FigureZoom = zoom
			v.drawDarkroomFigureStage(lc, opts.Src, opts.SVGRaw, opts.Annotations, imgSrc, loadErr, zoom)
		}
	}

	kit.DarkroomLightbox(c, tok, &openFig, nil, props)
	if !openFig && v.ExpandedFigureID == opts.ModuleID {
		v.figureOpen = false
		v.FigureZoom = 1
	}
}

func (v *View) drawDarkroomFigureStage(c *ui.Context, src, svgRaw string, annotations []svgTextLabel, imgSrc ui.ImageSource, loadErr string, zoom float32) {
	dtok := theme.GetTokens(c.Theme())
	if zoom <= 0 {
		zoom = 1
	}
	vw, vh := c.Size()
	baseH := float32(320)
	if vh > 0 {
		baseH = min(float32(460), max(float32(220), vh*0.56))
	}
	maxW := float32(820)
	if vw > 0 {
		maxW = max(float32(dtok.Spacing48)*4, vw-float32(dtok.Spacing32))
	}

	if strings.TrimSpace(svgRaw) != "" {
		vbX, vbY, vbW, vbH, labels := CachedSVGMetrics(svgRaw)
		allLabels := append(labels, annotations...)
		drawH := baseH * zoom
		drawW := float32(vbW/vbH) * drawH
		if zoom <= 1 && drawW > maxW {
			drawW = maxW
			drawH = float32(vbH/vbW) * drawW
		} else if zoom > 1 && drawW > maxW*zoom {
			drawW = maxW * zoom
			drawH = float32(vbH/vbW) * drawW
		}
		parsedSVG, err := v.cachedSVG(svgRaw)
		if err != nil {
			ui.Row(c).Padding(float32(dtok.Spacing8), float32(dtok.Spacing12)).
				Radius(float32(dtok.ControlRadius)).
				Background(dtok.DarkroomSurface).Border(1, dtok.Danger).Children(func() {
				ui.Text(c, "SVG 解析失败："+err.Error()).FontSize(float32(dtok.TextMeta)).TextColor(dtok.DangerText)
			})
			return
		}
		frameRad := float32(dtok.PanelRadius)
		frame := ui.Row(c).Justify(ui.Center).
			Padding(float32(dtok.Spacing16)).
			Radius(frameRad).Background(dtok.DarkroomSurface).Border(1, dtok.DarkroomBorder).Clip()
		applyDarkSpecularRim(frame, dtok, frameRad)
		frame.Children(func() {
			ui.Box(c).Size(drawW, drawH).DrawOver(func(p *ui.Painter, r ui.Rect) {
				v.paintSVGLabels(p, r, dtok, vbX, vbY, vbW, vbH, allLabels, zoom)
			}).Children(func() {
				ui.Image(c, parsedSVG).Size(drawW, drawH)
			})
		})
		return
	}

	if imgSrc != nil {
		frameRad := float32(dtok.ControlRadius)
		ui.Box(c).Radius(frameRad).Clip().Children(func() {
			ui.Image(c, imgSrc).Width(maxW * zoom).Height(baseH * zoom).Fit(ui.Contain).Radius(frameRad)
		})
		return
	}

	ui.Column(c).Width(min(maxW, float32(dtok.Spacing48)*7)).
		Padding(float32(dtok.Spacing16)).
		Radius(float32(dtok.PanelRadius)).
		Background(dtok.DarkroomSurface).
		Border(1, dtok.DarkroomBorder).
		Center().Gap(float32(dtok.Spacing8)).Children(func() {
		ui.Icon(c, lucide.Image()).Size(float32(dtok.IconLg), float32(dtok.IconLg)).TextColor(dtok.DarkroomMuted)
		ui.Text(c, "图片引用："+src+"（"+firstNonEmpty(loadErr, "暂无可预览图像")+"）").
			FontSize(float32(dtok.TextMeta)).FontFeatures("tnum").TextColor(dtok.DarkroomMuted)
	})
}

func renderMobileSVGEditorPage(c *ui.Context, tok theme.Tokens, key any, open *bool, svgBuf *string, onApply func(string)) {
	ui.Overlay(c, func() {
		t := c.Theme()
		topSafe, rightSafe, bottomSafe, leftSafe := c.SafeArea()
		ui.Column(c.Key(key)).Absolute().Fill().Modal().Background(tok.Canvas).Children(func() {
			ui.Row(c).FillWidth().
				Padding(t.Space(2)+topSafe, t.Space(4)+rightSafe, t.Space(2), t.Space(4)+leftSafe).
				MinHeight(t.Space(12)+topSafe).
				AlignItems(ui.Center).Gap(t.Space(2)).Background(tok.Surface).Children(func() {
				if components.Button(c.Key("mb-svg-cancel"), "取消", components.ButtonGhost).Shrink(0).Clicked() {
					*open = false
				}
				ui.Text(c, "编辑 SVG 矢量源码").FontSize(float32(tok.TextSection)).Bold().TextColor(tok.Text).
					Grow(1).MinWidth(0).SingleLine().Ellipsis("…")
				if components.Button(c.Key("mb-svg-apply"), "应用 SVG", components.ButtonPrimary).Shrink(0).Clicked() {
					if onApply != nil {
						onApply(strings.TrimSpace(*svgBuf))
					}
					*open = false
				}
			})
			ui.Box(c).FillWidth().Height(1).Background(tok.BorderSubtle)
			ui.Column(c).FillWidth().Grow(1).MinHeight(0).
				Padding(t.Space(4), t.Space(4)+rightSafe, t.Space(6)+bottomSafe, t.Space(4)+leftSafe).
				Gap(t.Space(2)).Children(func() {
				components.TextBox(c, svgBuf, components.TextBoxOptions{Multiline: true}).
					Placeholder("<svg viewBox=\"0 0 360 220\">…</svg>").Lines(10, 20).FillWidth()
			})
		})
	})
}

// PortableFigureAssetPath resolves or copies a user-selected image into
// `<baseDir>/assets/<filename>` when baseDir is set, returning `./assets/<filename>`.
func PortableFigureAssetPath(baseDir, picked string) string {
	return portableFigureAssetSrc(baseDir, picked)
}

func resolveFigurePath(baseDir, src string) string {
	trimmed := strings.TrimSpace(src)
	if trimmed == "" || filepath.IsAbs(trimmed) || baseDir == "" {
		return trimmed
	}
	return filepath.Join(baseDir, filepath.FromSlash(trimmed))
}

func portableFigureAssetSrc(baseDir, picked string) string {
	trimmed := strings.TrimSpace(picked)
	if baseDir == "" || trimmed == "" {
		return trimmed
	}
	if rawBytes, err := os.ReadFile(trimmed); err == nil {
		assetsDir := filepath.Join(baseDir, "assets")
		if mkErr := os.MkdirAll(assetsDir, 0o755); mkErr == nil {
			base := filepath.Base(trimmed)
			dest := filepath.Join(assetsDir, base)
			if writeErr := os.WriteFile(dest, rawBytes, 0o644); writeErr == nil {
				return "./assets/" + base
			}
		}
	}
	if rel, err := filepath.Rel(baseDir, trimmed); err == nil && !strings.HasPrefix(rel, "..") {
		slash := filepath.ToSlash(rel)
		if !strings.HasPrefix(slash, "./") {
			slash = "./" + slash
		}
		return slash
	}
	return trimmed
}

func (v *View) paintSVGLabels(p *ui.Painter, r ui.Rect, tok theme.Tokens, vbX, vbY, vbW, vbH float64, allLabels []svgTextLabel, zoom float32) {
	sx := float64(r.W) / vbW
	sy := float64(r.H) / vbH
	for _, lbl := range allLabels {
		px := r.X + float32((lbl.X-vbX)*sx)
		py := r.Y + float32((lbl.Y-vbY)*sy)
		fsize := float32(lbl.Size * sx)
		if fsize < 9.5 {
			fsize = 9.5
		}
		if fsize > float32(tok.TextSection)*zoom {
			fsize = float32(tok.TextSection) * zoom
		}
		textW := estimateSVGTextWidth(lbl.Text, fsize)
		if lbl.Anchor == "middle" {
			px -= textW * 0.5
		} else if lbl.Anchor == "end" {
			px -= textW
		}
		// SVG y denotes the alphabetic baseline; MyGo Painter draws from top-left Y.
		topOffset := fsize * 0.84
		switch lbl.Baseline {
		case "middle", "central":
			topOffset = fsize * 0.50
		case "hanging", "text-before-edge":
			topOffset = 0
		}
		col := tok.Text
		if lbl.Fill != "" {
			if parsedCol, ok := theme.ParseHexColor(lbl.Fill); ok {
				luma := 0.299*float64(parsedCol.R) + 0.587*float64(parsedCol.G) + 0.114*float64(parsedCol.B)
				if !(tok.Dark && luma < 90) {
					col = parsedCol
				}
			}
		}
		p.Text(px, py-topOffset, lbl.Text, fsize, col)
	}
}

func (v *View) drawFigureCanvas(c *ui.Context, moduleID, src, svgRaw string, annotations []svgTextLabel, maxH float32, zoom float32) {
	v.drawFigureCanvasInteractive(c, moduleID, src, svgRaw, annotations, maxH, zoom, nil)
}

func (v *View) drawFigureCanvasInteractive(c *ui.Context, moduleID, src, svgRaw string, annotations []svgTextLabel, maxH float32, zoom float32, onTap func()) {
	t := c.Theme()
	tok := theme.GetTokens(t)
	if zoom <= 0 {
		zoom = 1
	}
	maxInlineW := float32(480)
	if v.inPairedRow {
		maxInlineW = 360
	}
	if vw, _ := c.Size(); vw > 0 && (v.MobileCompact || vw < 680) {
		mobileMax := max(float32(tok.Spacing48)*4, vw-float32(tok.Spacing48)*2.0)
		if mobileMax < maxInlineW {
			maxInlineW = mobileMax
		}
	}
	frameRad := tok.InnerPanelRadius(tok.Spacing8)
	imgRad := tok.InnerRadius(int(frameRad), tok.Spacing4)

	if svgRaw != "" {
		vbX, vbY, vbW, vbH, labels := CachedSVGMetrics(svgRaw)
		allLabels := append(labels, annotations...)
		drawH := maxH * zoom
		drawW := float32(vbW/vbH) * drawH
		if zoom <= 1 && drawW > maxInlineW {
			drawW = maxInlineW
			drawH = float32(vbH/vbW) * drawW
		} else if zoom > 1 && drawW > 960 {
			drawW = 960
			drawH = float32(vbH/vbW) * drawW
		}
		parsedSVG, err := v.cachedSVG(svgRaw)
		if err != nil {
			ui.Row(c).Padding(float32(tok.Spacing8), float32(tok.Spacing12)).Radius(frameRad).
				Background(tok.Danger.Alpha(.08)).Border(1, tok.Danger).Children(func() {
				ui.Text(c, "SVG 解析失败："+err.Error()).FontSize(float32(tok.TextMeta)).TextColor(tok.Danger)
			})
			return
		}
		buildSVGContent := func() {
			ui.Box(c).Size(drawW, drawH).DrawOver(func(p *ui.Painter, r ui.Rect) {
				v.paintSVGLabels(p, r, tok, vbX, vbY, vbW, vbH, allLabels, zoom)
			}).Children(func() {
				ui.Image(c, parsedSVG).Size(drawW, drawH)
			})
		}
		if onTap != nil && zoom <= 1 {
			svgFrame := ui.ButtonBase(c.Key("fig-svg-btn:"+moduleID)).FillWidth().Justify(ui.Center).
				Padding(float32(tok.Spacing12)).
				Radius(frameRad).Background(tok.Elevated).Border(1, tok.BorderSubtle).Clip().
				Label("预览大图")
			if svgFrame.Hovered() {
				svgFrame.Border(1, tok.Border)
			}
			applyDarkSpecularRim(svgFrame, tok, frameRad)
			svgFrame.Children(buildSVGContent)
			if svgFrame.Clicked() {
				onTap()
			}
		} else {
			svgFrame := ui.Row(c).FillWidth().Justify(ui.Center).
				Padding(float32(tok.Spacing12)).
				Radius(frameRad).Background(tok.Elevated).Border(1, tok.BorderSubtle).Clip()
			applyDarkSpecularRim(svgFrame, tok, frameRad)
			svgFrame.Children(buildSVGContent)
		}
		return
	}

	if src != "" {
		resolved := resolveFigurePath(v.BaseDir, src)
		imgSrc, loadErr := v.cachedImageFile(resolved)
		if loadErr != "" {
			errRow := ui.Row(c).FillWidth().Padding(float32(tok.Spacing8), float32(tok.Spacing12)).
				Radius(frameRad).Background(tok.Elevated).Border(1, tok.BorderSubtle).
				Gap(float32(tok.Spacing8)).AlignItems(ui.Center)
			applyDarkSpecularRim(errRow, tok, frameRad)
			errRow.Children(func() {
				ui.Icon(c, lucide.Image()).Size(float32(tok.Spacing16), float32(tok.Spacing16)).TextColor(tok.TextMuted)
				ui.Text(c, "图片引用："+src+"（"+loadErr+"）").FontSize(float32(tok.TextMeta)).FontFeatures("tnum").TextColor(tok.TextMuted)
			})
			return
		}
		renderImg := func(frameEl ui.Element, hovered, pressed bool) {
			imgEl := ui.Image(c, imgSrc).Height(maxH * zoom).Radius(imgRad)
			if zoom <= 1 {
				imgEl.MaxWidth(maxInlineW)
			}
			if tok.Dark && zoom <= 1 {
				op := float32(0.86)
				if hovered || pressed {
					op = 1.0
				}
				imgEl.Opacity(frameEl.AnimateTo("fig-dark-op", op, theme.MotionMicroFade))
			}
		}
		if onTap != nil && zoom <= 1 {
			imgBtn := ui.ButtonBase(c.Key("fig-img-btn:"+moduleID)).FillWidth().Justify(ui.Center).
				Padding(float32(tok.Spacing12)).
				Radius(frameRad).Background(tok.Elevated).Border(1, tok.BorderSubtle).Clip().
				Label("预览大图")
			if imgBtn.Hovered() {
				imgBtn.Border(1, tok.Border)
			}
			applyDarkSpecularRim(imgBtn, tok, frameRad)
			hov, prs := imgBtn.Hovered(), imgBtn.Pressed()
			imgBtn.Children(func() {
				renderImg(imgBtn, hov, prs)
			})
			if imgBtn.Clicked() {
				onTap()
			}
		} else {
			imgRow := ui.Row(c.Key("fig-img-row:"+moduleID)).FillWidth().Justify(ui.Center).
				Padding(float32(tok.Spacing12)).
				Radius(frameRad).Background(tok.Elevated).Border(1, tok.BorderSubtle).Clip()
			applyDarkSpecularRim(imgRow, tok, frameRad)
			hov, prs := imgRow.Hovered(), imgRow.Pressed()
			imgRow.Children(func() {
				renderImg(imgRow, hov, prs)
			})
		}
	}
}

func (v *View) cachedSVG(raw string) (*ui.SVG, error) {
	if v.svgCache == nil {
		v.svgCache = map[string]*ui.SVG{}
	}
	if s, ok := v.svgCache[raw]; ok {
		return s, nil
	}
	// UI-01: MyGo v0.10+ natively renders SVG <text>, which collided with our
	// CJK-safe DrawOver text pass and created double labels. Strip <text> from
	// the vector geometry before parsing so each label renders cleanly once.
	geomOnly := reStripSVGText.ReplaceAllString(raw, "")
	s, err := ui.ParseSVG([]byte(geomOnly))
	if err != nil {
		return nil, err
	}
	v.svgCache[raw] = s
	return s, nil
}

func (v *View) cachedImageFile(path string) (ui.ImageSource, string) {
	if v.imgCache == nil {
		v.imgCache = map[string]ui.ImageSource{}
	}
	if v.imgErrCache == nil {
		v.imgErrCache = map[string]string{}
	}
	if v.imgLoading == nil {
		v.imgLoading = map[string]bool{}
	}
	if img, ok := v.imgCache[path]; ok {
		return img, ""
	}
	if errText, ok := v.imgErrCache[path]; ok {
		return nil, errText
	}
	if v.imgLoading[path] {
		return nil, "加载缩略图中…"
	}
	if !v.AsyncDecode {
		bytes, err := os.ReadFile(path)
		if err != nil {
			v.imgErrCache[path] = "未找到本地文件"
			return nil, v.imgErrCache[path]
		}
		img, err := ui.DecodeImage(bytes)
		if err != nil {
			v.imgErrCache[path] = "图像解码失败"
			return nil, v.imgErrCache[path]
		}
		v.imgCache[path] = img
		return img, ""
	}
	v.imgLoading[path] = true
	go func(p string) {
		rawBytes, readErr := os.ReadFile(p)
		var decoded ui.ImageSource
		var errText string
		if readErr != nil {
			errText = "未找到本地文件"
		} else {
			img, decErr := ui.DecodeImage(rawBytes)
			if decErr != nil {
				errText = "图像解码失败"
			} else {
				decoded = img
			}
		}
		mygo.RunOnMain(func() {
			delete(v.imgLoading, p)
			if decoded != nil {
				v.imgCache[p] = decoded
			} else {
				v.imgErrCache[p] = errText
			}
			if v.OnAsyncInvalidate != nil {
				v.OnAsyncInvalidate()
			}
		})
	}(path)
	return nil, "加载缩略图中…"
}
