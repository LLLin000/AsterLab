package asterdoc

import (
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/onevra/source/components"
	"github.com/oneo-me/onevra/source/icons/lucide"

	"asterlab-mygo-prototype/internal/theme"
)

type svgTextLabel struct {
	X      float64
	Y      float64
	Anchor string
	Text   string
	Size   float64
}

var (
	reViewBox  = regexp.MustCompile(`viewBox\s*=\s*["']([^"']+)["']`)
	reTextElem = regexp.MustCompile(`(?s)<text\b([^>]*)>(.*?)</text>`)
	reAttrX    = regexp.MustCompile(`\bx\s*=\s*["']([^"']+)["']`)
	reAttrY    = regexp.MustCompile(`\by\s*=\s*["']([^"']+)["']`)
	reAttrSize = regexp.MustCompile(`\bfont-size\s*=\s*["']([^"']+)["']`)
	reAttrAnch = regexp.MustCompile(`\btext-anchor\s*=\s*["']([^"']+)["']`)
	reTagStrip = regexp.MustCompile(`<[^>]+>`)
)

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
	for _, match := range reTextElem.FindAllStringSubmatch(rawSVG, -1) {
		attrs, body := match[1], strings.TrimSpace(reTagStrip.ReplaceAllString(match[2], ""))
		if body == "" {
			continue
		}
		var x, y float64
		size := 12.0
		anchor := "start"
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
		labels = append(labels, svgTextLabel{X: x, Y: y, Anchor: anchor, Text: body, Size: size})
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

	ui.Column(c).FillWidth().Gap(float32(tok.Spacing8)).Padding(float32(tok.Spacing12)).
		Radius(float32(tok.PanelRadius)).Background(tok.Surface).Border(1, tok.Border).Children(func() {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
			ui.Icon(c, lucide.Image()).Size(float32(tok.Spacing16), float32(tok.Spacing16)).TextColor(tok.TextMuted)
			ui.Text(c, title).FontSize(float32(tok.TextSection)).Bold().TextColor(tok.Text)
			ui.Spacer(c)
			components.Badge(c, badge)
			zoomBtn := components.IconButton(c, lucide.Maximize2(), "放大查看", components.IconButtonCompact)
			components.Tooltip(c, zoomBtn, "放大查看示意图与文字标注")
			if zoomBtn.Clicked() {
				v.ExpandedFigureID = moduleID
				v.figureOpen = true
			}
		})

		v.drawFigureCanvas(c, moduleID, src, svgRaw, annotations, maxH, false)

		if caption != "" {
			ui.Text(c, caption).FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
		}

		if editableDesign {
			openEd := v.ensureWidgetBool("fig_ed:" + moduleID)
			components.Collapsible(c, "编辑 SVG / 图片路径与图注", openEd, components.CollapsibleOptions{}, func() {
				srcBuf := v.ensureWidgetDraft("fig:"+moduleID+":src", src)
				capBuf := v.ensureWidgetDraft("fig:"+moduleID+":cap", caption)
				svgBuf := v.ensureWidgetDraft("fig:"+moduleID+":svg", svgRaw)
				ui.Column(c).FillWidth().Gap(float32(tok.Spacing8)).Padding(float32(tok.Spacing8), 0, 0, 0).Children(func() {
					ui.Row(c).FillWidth().Gap(float32(tok.Spacing8)).AlignItems(ui.Center).Children(func() {
						ui.Box(c).FillWidth().Children(func() {
							components.TextBox(c, srcBuf, components.TextBoxOptions{}).Placeholder("图片相对路径或 URI（可选）")
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

		if v.figureOpen && v.ExpandedFigureID == moduleID {
			components.Dialog(c, &v.figureOpen, components.DialogOptions{Label: title}, func(panel ui.Element) {
				ui.Column(c).Width(float32(tok.Spacing48)*12).Padding(float32(tok.Spacing16)).Gap(float32(tok.Spacing12)).Children(func() {
					ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
						ui.Text(c, title).FontSize(float32(tok.TextSection)).Bold().TextColor(tok.Text)
						ui.Spacer(c)
						if components.Button(c, "关闭", components.ButtonOutline).Clicked() {
							v.figureOpen = false
						}
					})
					v.drawFigureCanvas(c, moduleID, src, svgRaw, annotations, 360, true)
					if caption != "" {
						ui.Text(c, caption).FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
					}
				})
			})
		}
	})
}

func (v *View) drawFigureCanvas(c *ui.Context, moduleID, src, svgRaw string, annotations []svgTextLabel, maxH float32, large bool) {
	t := c.Theme()
	tok := theme.GetTokens(t)
	if svgRaw != "" {
		vbX, vbY, vbW, vbH, labels := parseSVGMetricsAndLabels(svgRaw)
		allLabels := append(labels, annotations...)
		drawH := maxH
		drawW := float32(vbW / vbH * float64(drawH))
		if large && drawW > 560 {
			drawW = 560
			drawH = float32(vbH / vbW * float64(drawW))
		}
		if !large && drawW > 380 {
			drawW = 380
			drawH = float32(vbH / vbW * float64(drawW))
		}
		parsedSVG, err := v.cachedSVG(svgRaw)
		if err != nil {
			ui.Row(c).Padding(float32(tok.Spacing8), float32(tok.Spacing12)).Radius(float32(tok.ControlRadius)).
				Background(tok.Danger.Alpha(.08)).Border(1, tok.Danger).Children(func() {
				ui.Text(c, "SVG 解析失败："+err.Error()).FontSize(float32(tok.TextMeta)).TextColor(tok.Danger)
			})
			return
		}
		ui.Row(c).FillWidth().Justify(ui.Center).Padding(float32(tok.Spacing8)).
			Radius(float32(tok.ControlRadius)).Background(tok.Elevated).Border(1, tok.Border).Children(func() {
			ui.Box(c).Size(drawW, drawH).DrawOver(func(p *ui.Painter, r ui.Rect) {
				sx := float64(r.W) / vbW
				sy := float64(r.H) / vbH
				for _, lbl := range allLabels {
					px := r.X + float32((lbl.X-vbX)*sx)
					py := r.Y + float32((lbl.Y-vbY)*sy)
					fsize := float32(lbl.Size * sx)
					if fsize < float32(tok.TextMeta) {
						fsize = float32(tok.TextMeta)
					}
					if fsize > float32(tok.TextSection) {
						fsize = float32(tok.TextSection)
					}
					if lbl.Anchor == "middle" {
						// Offset left slightly based on rune length to visually center text cleanly
						px -= float32(len([]rune(lbl.Text))) * fsize * 0.28
					} else if lbl.Anchor == "end" {
						px -= float32(len([]rune(lbl.Text))) * fsize * 0.56
					}
					p.Text(px, py-fsize*0.35, lbl.Text, fsize, tok.Text)
				}
			}).Children(func() {
				ui.Image(c, parsedSVG).Size(drawW, drawH)
			})
		})
		return
	}

	if src != "" {
		imgSrc, loadErr := v.cachedImageFile(src)
		if loadErr != "" {
			ui.Row(c).FillWidth().Padding(float32(tok.Spacing8), float32(tok.Spacing12)).
				Radius(float32(tok.ControlRadius)).Background(tok.Elevated).Border(1, tok.Border).
				Gap(float32(tok.Spacing8)).AlignItems(ui.Center).Children(func() {
				ui.Icon(c, lucide.Image()).Size(float32(tok.Spacing16), float32(tok.Spacing16)).TextColor(tok.TextMuted)
				ui.Text(c, "图片引用："+src+"（"+loadErr+"）").FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
			})
			return
		}
		ui.Row(c).FillWidth().Justify(ui.Center).Padding(float32(tok.Spacing8)).
			Radius(float32(tok.ControlRadius)).Background(tok.Elevated).Border(1, tok.Border).Children(func() {
			ui.Image(c, imgSrc).Height(maxH)
		})
	}
}

func (v *View) cachedSVG(raw string) (*ui.SVG, error) {
	if v.svgCache == nil {
		v.svgCache = map[string]*ui.SVG{}
	}
	if s, ok := v.svgCache[raw]; ok {
		return s, nil
	}
	s, err := ui.ParseSVG([]byte(raw))
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
	if img, ok := v.imgCache[path]; ok {
		return img, ""
	}
	if errText, ok := v.imgErrCache[path]; ok {
		return nil, errText
	}
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
