package asterdoc

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"
	"github.com/oneo-me/onevra/source/components"
	"github.com/oneo-me/onevra/source/icons/lucide"

	"asterlab-mygo-prototype/internal/protocol"
	"asterlab-mygo-prototype/internal/theme"
	plateui "asterlab-mygo-prototype/internal/uikit/plate"
)

// View renders an AsterML document with native modules. It never writes a
// record: a change replaces the module's fence body and reports the new source
// to OnChange, leaving validation and persistence to the caller.
type View struct {
	Doc              *Document
	Registry         *Registry
	Mode             Mode
	Density          Density
	RunID            string
	DocumentRevision string
	ReadOnly         bool
	SelectedGroup    string
	SelectedWells    []string
	// Allowed is the explicit business-command grant for this render. Mode and
	// ReadOnly never imply write access on their own.
	Allowed  map[string]bool
	OnChange func(source string)
	// OnAssignWells/OnClearWells are the Core command seam. When set, the
	// renderer reports the semantic command and never mutates the source
	// locally; the caller applies it and reloads a new revision.
	OnAssignWells func(moduleID, group string, wells []string)
	OnClearWells  func(moduleID string, wells []string)
	// OnUpdateModule reports a generic module edit intent. The renderer never
	// writes Core or the document; the host applies it and reloads a revision.
	OnUpdateModule func(moduleID string, props map[string]any)
	// Step-scoped block editing callbacks (Design mode).
	OnInsertStepBlock    func(stepID string, index int, block map[string]any)
	OnUpdateStepBlock    func(stepID, blockID string, props map[string]any)
	OnMoveStepBlock      func(stepID, blockID, direction string)
	OnRemoveStepBlock    func(stepID, blockID string)
	OnPickFigureFile     func(stepID, blockID string)
	StepID               string
	// Run-mode interactive execution callbacks & state (nil-safe in read-only views).
	OnRecordActualParam  func(stepID, blockID, key, value, unit string)
	OnToggleCheck        func(stepID, checkID string, checked bool)
	OnRuleSkip           func(stepID, checkID, rule string)
	OnTimerAction        func(stepID, blockID, action string)
	OnSubmitTableReadout func(stepID, blockID string, payload map[string]any)
	OnExcludeSample      func(sampleID, status string)
	OnLinkAsset          func(stepID, blockID, wellID string)
	ActualParams         map[string]string
	CheckedItems         map[string]bool
	SkippedItems         map[string]string
	TimerStates          map[string]TimerRuntimeState
	TableCells           map[string]map[string]string
	TableQualifiers      map[string]map[string]string
	SampleStatuses       map[string]string
	LinkedAssets         map[string][]AssetThumb
	ExpandedFigureID     string
	figureOpen           bool
	HideStepHeader       bool
	widgetTextDrafts     map[string]*string
	widgetBoolDrafts     map[string]*bool
	svgCache             map[string]*ui.SVG
	imgCache             map[string]ui.ImageSource
	imgErrCache          map[string]string
	calcScratch          map[string]map[string]*string
	// ExpandedPlate is the plate rendered in the full-screen overlay; it is
	// editor state, not document state.
	ExpandedPlate string
	// moduleDrafts stores editable text per module/field across frames.
	moduleDrafts        map[string]map[string]*string
	moduleDraftBool     map[string]map[string]*bool
	moduleLists         map[string]map[string][]string
	moduleOriginals     map[string]map[string]any
	ExpandedWells       []string
	PlateSampleID       string
	PlateReplicateStart string
	PlateSampleError    string
	PlateZoom           float32
	gridPointerDown     bool
	gridPointerStart    ui.Point
	gridDragging        bool
	gridAdditive        bool
	gridSuppressClick   bool
	gridStart           ui.Point
	gridLast            ui.Point
	gridBase            []string
}

// Can reports whether the host granted one business command.
func (v *View) Can(permission string) bool {
	return v.Allowed[permission]
}

// Render draws the whole document, replacing every asterlab fence with the
// registered module renderer.
func (v *View) Render(c *ui.Context) {
	if v.Doc == nil {
		return
	}
	protocol.RenderMarkdownWith(c, v.Doc.Source, v.codeBlock)
	v.plateOverlay(c)
}

func (v *View) codeBlock(c *ui.Context, block *markdown.CodeBlockView) {
	if block.Language != "asterlab" {
		markdown.DefaultCodeBlock(c, block)
		return
	}
	registry := v.Registry
	if registry == nil {
		registry = BuiltinRegistry()
	}
	module, err := parseModuleWith(block.Code(), 0, 0, registry)
	if err != nil {
		v.moduleError(c, err)
		return
	}
	v.renderRegisteredModule(c, registry, module.Kind, module.Schema, module.ID, module.Data)
}

// renderRegisteredModule checks the surface declared for this mode and then
// renders the module through the registered implementation.
func (v *View) renderRegisteredModule(c *ui.Context, registry *Registry, kind string, schema int, moduleID string, data map[string]any) {
	spec, ok := registry.Lookup(kind, schema)
	if !ok {
		v.rendererGap(c, kind, schema, "")
		return
	}
	key, declared := spec.Descriptor.RendererKey(v.Mode)
	if !declared {
		v.rendererGap(c, kind, schema, "")
		return
	}
	if !registry.HasRenderer(key) {
		v.rendererGap(c, kind, schema, key)
		return
	}
	// Generic editor is a registered renderer capability; dispatch by key only.
	fn, _ := registry.Renderer(key)
	if fn == nil {
		v.rendererGap(c, kind, schema, key)
		return
	}
	fn(v, c, data)
}

// rendererGap reports a declared-but-unavailable or undeclared renderer as a
// visible capability gap. The host must never draw a guess.
func (v *View) rendererGap(c *ui.Context, kind string, schema int, key string) {
	t := c.Theme()
	tok := theme.GetTokens(t)
	reason := "该模块未声明此视图的渲染能力"
	if key != "" {
		reason = fmt.Sprintf("渲染能力 %q 未被实现", key)
	}
	ui.Row(c).Padding(float32(tok.Spacing8), float32(tok.Spacing12)).Radius(float32(tok.PanelRadius)).
		Background(tok.Warning.Alpha(.08)).Border(1, tok.Warning).Children(func() {
		ui.Text(c, fmt.Sprintf("%s@%d · %s", kind, schema, reason)).
			FontSize(float32(tok.TextMeta)).TextColor(tok.Warning)
	})
}

func (v *View) moduleError(c *ui.Context, err error) {
	t := c.Theme()
	tok := theme.GetTokens(t)
	ui.Row(c).Padding(float32(tok.Spacing8), float32(tok.Spacing12)).Radius(float32(tok.PanelRadius)).
		Background(tok.Danger.Alpha(.08)).Border(1, tok.Danger).Children(func() {
		ui.Text(c, "asterlab 模块无法解析："+err.Error()).FontSize(float32(tok.TextMeta)).
			TextColor(tok.Danger)
	})
}

// RenderModule renders one already-parsed module through the same surface
// checks used for fenced AsterML blocks.
func (v *View) RenderModule(c *ui.Context, kind string, schema int, data map[string]any) {
	registry := v.Registry
	if registry == nil {
		registry = BuiltinRegistry()
	}
	moduleID, _ := data["id"].(string)
	v.renderRegisteredModule(c, registry, kind, schema, moduleID, data)
}

func (v *View) plate(c *ui.Context, p *Plate) {
	t := c.Theme()
	tok := theme.GetTokens(t)
	groups := p.GroupNames()
	counts := map[string]int{}
	geometry, _ := geometryForPlate(p.Format, p.Topology, p.Rows, p.Columns)
	wells := map[string]plateui.WellState{}
	for well, group := range p.WellGroups() {
		counts[group]++
		wells[well] = plateui.WellState{
			Group:     group,
			SampleID:  p.Samples[well],
			Replicate: p.Replicates[well],
			Role:      p.Roles[well],
		}
	}
	title := p.Title
	if strings.TrimSpace(title) == "" {
		if p.Topology == "dish" {
			title = fmt.Sprintf("培养皿分区（%d 区）", p.Format)
		} else {
			title = fmt.Sprintf("%d 孔板", p.Format)
		}
	}
	ui.Row(c).FillWidth().Children(func() {
		ui.Column(c).FillWidth().Gap(float32(tok.Spacing8)).Padding(float32(tok.Spacing12)).
			Radius(float32(tok.PanelRadius)).Background(tok.Surface).Border(1, tok.Border).Children(func() {
			ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
				ui.Column(c).Gap(float32(tok.Spacing4)).Children(func() {
					ui.Text(c, title).FontSize(float32(tok.TextSection)).Bold().TextColor(tok.Text)
					meta := p.ID
					if p.Assay != "" {
						meta += " · " + p.Assay
					}
					if p.Timepoint != "" {
						meta += " · " + p.Timepoint
					}
					ui.Text(c, meta).FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
				})
				ui.Spacer(c)
				if v.ReadOnly {
					components.Badge(c, "冻结视图")
				}
				if components.IconButton(c, lucide.Maximize2(), "放大孔板", components.IconButtonCompact).Clicked() {
					v.ExpandedPlate = p.ID
					v.ExpandedWells = nil
					v.PlateZoom = 1
				}
			})
			ui.Row(c).Wrap().Gap(float32(tok.Spacing12)).Children(func() {
				for _, group := range groups {
					ui.Row(c).Gap(float32(tok.Spacing4)).AlignItems(ui.Center).Children(func() {
						ui.Box(c).Size(float32(tok.Spacing8), float32(tok.Spacing8)).Radius(float32(tok.ControlRadius)).Background(groupColorForCtx(c, group))
						ui.Text(c, fmt.Sprintf("%s · %d 孔", group, counts[group])).
							FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted).FontFeatures("tnum")
					})
				}
			})
			plateui.View(c, plateui.Props{
				Wells: wells, Rows: geometry.Rows, Cols: geometry.Columns,
				Topology: p.Topology, Roles: p.Roles, Interactive: false,
			})
			ui.Text(c, "点击右上角放大查看与编辑孔位分配。").
				FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
		})
	})
}

const (
	plateGridLabelWidth   = float32(24)
	plateGridHeaderHeight = float32(18)
	plateGridGap          = float32(4)
	plateGridWellSize     = float32(18)
)

// ponytail: the canvas is sized for the largest supported 384-well plate; grow it if larger formats are added.
func plateGridLayout(geometry Geometry, zoom float32) (viewportW, viewportH, cell float32) {
	maxGeometry, _ := geometryFor(384)
	viewportW = plateGridLabelWidth + plateGridGap +
		float32(maxGeometry.Columns)*plateGridWellSize*zoom +
		float32(maxGeometry.Columns-1)*plateGridGap
	viewportH = plateGridHeaderHeight + plateGridGap +
		float32(maxGeometry.Rows)*plateGridWellSize*zoom +
		float32(maxGeometry.Rows-1)*plateGridGap

	cellW := (viewportW - plateGridLabelWidth - plateGridGap -
		float32(geometry.Columns-1)*plateGridGap) / float32(geometry.Columns)
	cellH := (viewportH - plateGridHeaderHeight - plateGridGap -
		float32(geometry.Rows-1)*plateGridGap) / float32(geometry.Rows)
	cell = cellW
	if cellH < cell {
		cell = cellH
	}
	return viewportW, viewportH, cell
}

// plateOverlay floats the enlarged plate over a dimmed canvas. There is no
// modal panel or inspector: the plate itself is the surface, with one floating
// toolbar for zoom, group choice and batch assignment.
func (v *View) plateOverlay(c *ui.Context) {
	suppressGridClick := v.gridSuppressClick
	v.gridSuppressClick = false
	if v.ExpandedPlate == "" {
		return
	}
	var plate *Plate
	for i := range v.Doc.Modules {
		if v.Doc.Modules[i].ID == v.ExpandedPlate && v.Doc.Modules[i].Plate != nil {
			plate = v.Doc.Modules[i].Plate
		}
	}
	if plate == nil {
		v.ExpandedPlate = ""
		return
	}
	t := c.Theme()
	geometry, _ := geometryFor(plate.Format)
	wells := plate.WellGroups()
	groups := plate.GroupNames()
	if v.SelectedGroup == "" && len(groups) > 0 {
		v.SelectedGroup = groups[0]
	}
	canEdit := v.Can(PermDocumentEdit) && !v.ReadOnly
	zoom := v.PlateZoom
	if zoom <= 0 {
		zoom = 1
	}
	closeOverlay := func() {
		v.ExpandedPlate = ""
		v.ExpandedWells = nil
	}
	assign := func() {
		if len(v.ExpandedWells) == 0 || v.SelectedGroup == "" {
			return
		}
		if v.OnAssignWells != nil {
			v.OnAssignWells(plate.ID, v.SelectedGroup, append([]string(nil), v.ExpandedWells...))
		} else {
			_ = v.assign(v.ExpandedWells, v.SelectedGroup)
		}
		v.ExpandedWells = nil
	}
	clear := func() {
		if len(v.ExpandedWells) == 0 {
			return
		}
		if v.OnClearWells != nil {
			v.OnClearWells(plate.ID, append([]string(nil), v.ExpandedWells...))
		} else {
			_ = v.assign(v.ExpandedWells, "")
		}
		v.ExpandedWells = nil
	}

	assignSample := func() {
		if len(v.ExpandedWells) == 0 {
			v.PlateSampleError = "请先选择孔位"
			return
		}
		start := 0
		rawStart := strings.TrimSpace(v.PlateReplicateStart)
		if rawStart != "" {
			n, err := strconv.Atoi(rawStart)
			if err != nil || n <= 0 {
				v.PlateSampleError = "技术重复编号必须为正整数"
				return
			}
			start = n
		}
		samples, replicates, err := ApplyPlateSamples(geometry, plate.Groups, plate.Samples, plate.Replicates, v.ExpandedWells, v.PlateSampleID, start)
		if err != nil {
			v.PlateSampleError = err.Error()
			return
		}
		props := map[string]any{"samples": samples, "replicates": replicates}
		if v.OnUpdateModule != nil {
			v.OnUpdateModule(plate.ID, props)
		} else if v.Doc != nil {
			if _, err := v.Doc.UpdateModule(plate.ID, props); err != nil {
				v.PlateSampleError = err.Error()
				return
			}
			if v.OnChange != nil {
				v.OnChange(v.Doc.Source)
			}
		}
		v.PlateSampleError = ""
		v.ExpandedWells = nil
	}
	ui.Overlay(c, func() {
		backdrop := ui.Box(c).Absolute().Left(0).Top(0).Right(0).Bottom(0).
			Background(ui.RGBA(0, 0, 0, .62)).Modal()
		if backdrop.Clicked() {
			closeOverlay()
		}
		layer := ui.Column(c).Absolute().Left(0).Top(0).Right(0).Bottom(0).
			AlignItems(ui.Center).Justify(ui.Center).Padding(t.Space(6)).Modal()
		if layer.OverlayShortcut(0, ui.KeyEscape) {
			closeOverlay()
		}
		layer.Children(func() {
			ui.Column(c).Gap(t.Space(2)).AlignItems(ui.Center).Children(func() {
				// The plate itself floats on the dimmed canvas: a quiet card,
				// circular wells, and a legend instead of cell text.
				ui.Box(c).Background(t.Surface).Border(1, t.Border).
					Radius(t.Space(2.5)).Shadow(0, t.Space(3), t.Space(9), 0, ui.RGBA(0, 0, 0, .38)).
					Padding(t.Space(4), t.Space(4.5)).Children(func() {
					ui.Column(c).Gap(t.Space(1.5)).Children(func() {
						counts := map[string]int{}
						for _, group := range wells {
							if group != "" {
								counts[group]++
							}
						}
						ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(t.Space(1)).Children(func() {
							ui.Column(c).Gap(1).Children(func() {
								ui.Text(c, fmt.Sprintf("%d 孔板", plate.Format)).FontSize(t.Rem(1.1)).
									Bold().TextColor(t.Text)
								meta := plate.ID
								if plate.Assay != "" {
									meta += " · " + plate.Assay
								}
								ui.Text(c, meta).FontSize(t.Rem(.72)).TextColor(t.TextMuted)
							})
							ui.Spacer(c)
							ui.Row(c).Wrap().Gap(t.Space(1.25)).Children(func() {
								for _, group := range groups {
									ui.Row(c).Gap(t.Space(1)).AlignItems(ui.Center).Children(func() {
										ui.Box(c).Size(t.Space(2), t.Space(2)).Radius(t.Space(1)).Background(groupColorForCtx(c, group))
										ui.Text(c, fmt.Sprintf("%s %d", group, counts[group])).
											FontSize(t.Rem(.7)).TextColor(t.TextMuted).FontFeatures("tnum")
									})
								}
							})
						})
						viewportW, viewportH, cell := plateGridLayout(geometry, zoom)
						selectMany := func(ids []string, additive bool) {
							if !additive {
								v.ExpandedWells = nil
							}
							for _, id := range ids {
								if !containsString(v.ExpandedWells, id) {
									v.ExpandedWells = append(v.ExpandedWells, id)
								}
							}
						}
						applyMarquee := func() {
							x0, y0 := v.gridStart.X, v.gridStart.Y
							x1, y1 := v.gridLast.X, v.gridLast.Y
							if x0 > x1 {
								x0, x1 = x1, x0
							}
							if y0 > y1 {
								y0, y1 = y1, y0
							}
							selected := WellsInRect(geometry, cell, plateGridGap,
								ui.Rect{X: x0, Y: y0, W: x1 - x0, H: y1 - y0})
							v.ExpandedWells = v.ExpandedWells[:0]
							if v.gridAdditive {
								v.ExpandedWells = append(v.ExpandedWells, v.gridBase...)
							}
							for _, id := range selected {
								if !containsString(v.ExpandedWells, id) {
									v.ExpandedWells = append(v.ExpandedWells, id)
								}
							}
						}
						ui.Box(c).Key("plate-grid").Size(viewportW, viewportH).Children(func() {
							ui.Column(c).Fill().Gap(plateGridGap).AlignItems(ui.Center).Justify(ui.Center).Children(func() {
								ui.Row(c).Gap(plateGridGap).Children(func() {
									ui.Box(c).Width(plateGridLabelWidth).Height(plateGridHeaderHeight)
									for col := 1; col <= geometry.Columns; col++ {
										col := col
										btn := ui.ButtonBase(c).Width(cell).Height(plateGridHeaderHeight).
											Label(fmt.Sprintf("选择第 %d 列", col))
										btn.Children(func() {
											ui.Text(c, fmt.Sprintf("%d", col)).FontSize(t.Rem(.62)).
												TextColor(t.TextMuted).Center()
										})
										if btn.Clicked() && !v.gridDragging && !suppressGridClick {
											ids := make([]string, 0, geometry.Rows)
											for row := 0; row < geometry.Rows; row++ {
												ids = append(ids, fmt.Sprintf("%s%d", rowLabel(row), col))
											}
											selectMany(ids, btn.ClickModifiers()&(ui.Shift|ui.Cmd) != 0)
										}
									}
								})
								for row := 0; row < geometry.Rows; row++ {
									row := row
									ui.Row(c).Gap(plateGridGap).Children(func() {
										btn := ui.ButtonBase(c).Width(plateGridLabelWidth).Height(cell).
											Label("选择第 " + rowLabel(row) + " 行")
										btn.Children(func() {
											ui.Text(c, rowLabel(row)).FontSize(t.Rem(.62)).TextColor(t.TextMuted).Center()
										})
										if btn.Clicked() && !v.gridDragging && !suppressGridClick {
											ids := make([]string, 0, geometry.Columns)
											for col := 1; col <= geometry.Columns; col++ {
												ids = append(ids, fmt.Sprintf("%s%d", rowLabel(row), col))
											}
											selectMany(ids, btn.ClickModifiers()&(ui.Shift|ui.Cmd) != 0)
										}
										for col := 1; col <= geometry.Columns; col++ {
											col := col
											well := fmt.Sprintf("%s%d", rowLabel(row), col)
											group := wells[well]
											selected := containsString(v.ExpandedWells, well)
											bg := t.Background
											if group != "" {
												bg = groupColorForCtx(c, group)
											}
											border := t.Border
											width := float32(1)
											if selected {
												border, width = t.Accent, 2
											}
											btn := ui.ButtonBase(c).Key(well).Size(cell, cell).Radius(cell/2).
												Background(bg).Border(width, border).Label("孔 " + well)
											cellPoint := func(x, y float32) ui.Point {
												return ui.Point{
													X: float32(col-1)*(cell+plateGridGap) + x,
													Y: float32(row)*(cell+plateGridGap) + y,
												}
											}
											btn = btn.HandleInput(func(event ui.InputEvent) bool {
												if (event.Kind == ui.InputPointerDown || event.Kind == ui.InputPointerUp) &&
													event.Button != 0 {
													return false
												}
												switch event.Kind {
												case ui.InputPointerDown:
													point := cellPoint(event.X, event.Y)
													v.gridPointerDown = true
													v.gridPointerStart = ui.Point{X: event.X, Y: event.Y}
													v.gridStart, v.gridLast = point, point
													v.gridDragging = false
													v.gridAdditive = event.Mods&(ui.Shift|ui.Cmd) != 0
													v.gridBase = append(v.gridBase[:0], v.ExpandedWells...)
													return false
												case ui.InputPointerMove:
													if !v.gridPointerDown {
														return false
													}
													dx := event.X - v.gridPointerStart.X
													dy := event.Y - v.gridPointerStart.Y
													if !v.gridDragging {
														if dx > -3 && dx < 3 && dy > -3 && dy < 3 {
															return false
														}
														v.gridDragging = true
													}
													v.gridLast = cellPoint(event.X, event.Y)
													applyMarquee()
													return true
												case ui.InputPointerUp:
													if !v.gridPointerDown {
														return false
													}
													v.gridPointerDown = false
													if !v.gridDragging {
														v.gridBase = nil
														return false
													}
													if event.Canceled {
														v.ExpandedWells = append(v.ExpandedWells[:0], v.gridBase...)
													} else {
														v.gridLast = cellPoint(event.X, event.Y)
														applyMarquee()
													}
													v.gridDragging = false
													v.gridSuppressClick = true
													v.gridBase = nil
													return true
												}
												return false
											})
											if btn.Clicked() && !v.gridDragging && !suppressGridClick {
												mods := btn.ClickModifiers()
												if mods&(ui.Shift|ui.Cmd) != 0 {
													if selected {
														v.ExpandedWells = removeString(v.ExpandedWells, well)
													} else {
														v.ExpandedWells = append(v.ExpandedWells, well)
													}
												} else {
													v.ExpandedWells = []string{well}
												}
											}
										}
									})
								}
							})
						})
					})
				})
				// Floating toolbar: directly above the plate, not a modal panel.
				ui.Row(c).Padding(t.Space(.75), t.Space(1)).Radius(t.Space(2)).
					Background(t.Surface).Border(1, t.Border).
					Shadow(0, t.Space(2), t.Space(6), 0, ui.RGBA(0, 0, 0, .30)).
					Gap(t.Space(.5)).AlignItems(ui.Center).Children(func() {
					ui.Text(c, fmt.Sprintf("%d 孔板", plate.Format)).FontSize(t.Rem(.8)).
						Bold().TextColor(t.Text).Padding(0, t.Space(.75))
					ui.Box(c).Width(1).Height(18).Background(t.Border)
					if components.IconButton(c, lucide.ZoomOut(), "缩小孔板", components.IconButtonPlain).Clicked() && zoom > 0.6 {
						v.PlateZoom = zoom - 0.1
					}
					ui.Text(c, fmt.Sprintf("%.0f%%", zoom*100)).FontSize(t.Rem(.72)).
						TextColor(t.TextMuted).FontFeatures("tnum")
					if components.IconButton(c, lucide.ZoomIn(), "放大孔板", components.IconButtonPlain).Clicked() && zoom < 1.6 {
						v.PlateZoom = zoom + 0.1
					}
					if components.IconButton(c, lucide.RotateCcw(), "重置缩放", components.IconButtonPlain).Clicked() {
						v.PlateZoom = 1
					}
					ui.Box(c).Width(1).Height(18).Background(t.Border)
					for _, group := range groups {
						group := group
						active := group == v.SelectedGroup
						btn := ui.ButtonBase(c).Padding(t.Space(1), t.Space(2)).Radius(t.Space(1)).
							Background(map[bool]ui.Color{true: t.Accent.Alpha(.16), false: ui.Color{}}[active]).
							OnClick(func() { v.SelectedGroup = group })
						btn.Children(func() {
							ui.Row(c).Gap(t.Space(1)).AlignItems(ui.Center).Children(func() {
								ui.Box(c).Size(t.Space(2), t.Space(2)).Radius(t.Space(.75)).Background(groupColorForCtx(c, group))
								ui.Text(c, group).FontSize(t.Rem(.75)).TextColor(t.Text)
							})
						})
					}
					if len(v.ExpandedWells) > 0 {
						ui.Text(c, fmt.Sprintf("%d 孔", len(v.ExpandedWells))).
							FontSize(t.Rem(.72)).TextColor(t.TextMuted)
					}
					if canEdit {
						ui.Box(c).Width(1).Height(18).Background(t.Border)
						ui.TextInput(c, &v.PlateSampleID).Placeholder("样本 ID")
						ui.TextInput(c, &v.PlateReplicateStart).Placeholder("起始技术重复 (可选)")
						applySample := components.IconButton(c, lucide.CheckCheck(), "分配样本", components.IconButtonPlain)
						applySample.Disabled(len(v.ExpandedWells) == 0)
						if applySample.Clicked() {
							assignSample()
						}
						if v.PlateSampleError != "" {
							ui.Text(c, v.PlateSampleError).FontSize(t.Rem(.7)).
								TextColor(t.Danger)
						}
						ui.Box(c).Width(1).Height(18).Background(t.Border)
						assignBtn := components.IconButton(c, lucide.CheckCheck(), "分配到 "+v.SelectedGroup, components.IconButtonPlain)
						assignBtn.Disabled(len(v.ExpandedWells) == 0)
						if assignBtn.Clicked() {
							assign()
						}
						erase := components.IconButton(c, lucide.Eraser(), "清除所选孔位", components.IconButtonPlain)
						erase.Disabled(len(v.ExpandedWells) == 0)
						if erase.Clicked() {
							clear()
						}
					}
					ui.Box(c).Width(1).Height(18).Background(t.Border)
					if components.IconButton(c, lucide.X(), "关闭放大孔板", components.IconButtonPlain).Clicked() {
						closeOverlay()
					}
				})
			})
		})
	})
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

// ApplyPlateSamples maps one sample ID onto the selected wells. A positive
// startReplicate assigns successive replicate numbers in deterministic
// well order; zero leaves replicate labels absent. Existing mappings for
// other wells are preserved.
func ApplyPlateSamples(geometry Geometry, groups map[string][]string, samples map[string]string, replicates map[string]int, wells []string, sampleID string, startReplicate int) (map[string]string, map[string]int, error) {
	sampleID = strings.TrimSpace(sampleID)
	if sampleID == "" {
		return nil, nil, fmt.Errorf("sample ID cannot be blank")
	}
	if startReplicate < 0 {
		return nil, nil, fmt.Errorf("replicate start must be positive")
	}
	groupByWell := map[string]string{}
	for group, assigned := range groups {
		for _, well := range assigned {
			groupByWell[strings.ToUpper(strings.TrimSpace(well))] = strings.TrimSpace(group)
		}
	}
	outSamples := make(map[string]string, len(samples)+len(wells))
	for well, id := range samples {
		outSamples[strings.ToUpper(strings.TrimSpace(well))] = strings.TrimSpace(id)
	}
	outReplicates := make(map[string]int, len(replicates)+len(wells))
	for well, n := range replicates {
		outReplicates[strings.ToUpper(strings.TrimSpace(well))] = n
	}
	selected := make([]string, 0, len(wells))
	seen := map[string]bool{}
	for _, well := range wells {
		well = strings.ToUpper(strings.TrimSpace(well))
		if well == "" {
			return nil, nil, fmt.Errorf("well ID cannot be blank")
		}
		if seen[well] {
			continue
		}
		seen[well] = true
		selected = append(selected, well)
	}
	sort.Slice(selected, func(i, j int) bool {
		return wellLess(geometry, selected[i], selected[j])
	})
	if startReplicate > 0 && len(selected) > 0 {
		maxInt := int(^uint(0) >> 1)
		if startReplicate > maxInt-(len(selected)-1) {
			return nil, nil, fmt.Errorf("replicate range overflows integer")
		}
	}
	selectedSet := make(map[string]bool, len(selected))
	for _, well := range selected {
		selectedSet[well] = true
	}
	replicateOwner := map[string]string{}
	for well, replicate := range outReplicates {
		if selectedSet[well] || replicate < 1 || outSamples[well] == "" {
			continue
		}
		replicateOwner[outSamples[well]+"\x00"+strconv.Itoa(replicate)] = well
	}
	for i, well := range selected {
		if !validWell(well, geometry) {
			return nil, nil, fmt.Errorf("invalid well %q", well)
		}
		if _, ok := groupByWell[well]; !ok {
			return nil, nil, fmt.Errorf("well %s has no group assignment", well)
		}
		outSamples[well] = sampleID
		if startReplicate > 0 {
			replicate := startReplicate + i
			key := sampleID + "\x00" + strconv.Itoa(replicate)
			if previous, exists := replicateOwner[key]; exists {
				return nil, nil, fmt.Errorf("sample %s replicate %d is already assigned to %s", sampleID, replicate, previous)
			}
			replicateOwner[key] = well
			outReplicates[well] = replicate
		} else {
			delete(outReplicates, well)
		}
	}
	return outSamples, outReplicates, nil
}

func wellLess(geometry Geometry, a, b string) bool {
	ra, ca := wellRowCol(a)
	rb, cb := wellRowCol(b)
	if ra != rb {
		return ra < rb
	}
	return ca < cb
}

func wellRowCol(well string) (int, int) {
	if len(well) < 2 {
		return 0, 0
	}
	row := int(well[0] - 'A')
	col := 0
	for _, r := range well[1:] {
		col = col*10 + int(r-'0')
	}
	return row, col
}

func removeString(values []string, target string) []string {
	out := values[:0]
	for _, value := range values {
		if value != target {
			out = append(out, value)
		}
	}
	return out
}

func groupColorForCtx(c *ui.Context, group string) ui.Color {
	t := c.Theme()
	return plateui.GroupColor(group, t != nil && t.Dark)
}

// Assign moves wells into group in the current document and reports the new
// source. An empty group removes the wells. It is exported for tests and for a
// future source editor to call outside a render pass.
func (v *View) Assign(wells []string, group string) error {
	return v.assign(wells, group)
}

func (v *View) assign(wells []string, group string) error {
	if !v.Can(PermDocumentEdit) {
		return fmt.Errorf("permission %s is not granted", PermDocumentEdit)
	}
	if v.Doc == nil {
		return fmt.Errorf("document has no plate module")
	}
	var module *Module
	for i := range v.Doc.Modules {
		candidate := &v.Doc.Modules[i]
		if candidate.Plate == nil || (v.ExpandedPlate != "" && candidate.ID != v.ExpandedPlate) {
			continue
		}
		module = candidate
		break
	}
	if module == nil {
		return fmt.Errorf("document has no plate module")
	}
	groups := map[string][]string{}
	for name, existing := range module.Plate.Groups {
		groups[name] = append([]string(nil), existing...)
	}
	target := strings.TrimSpace(group)
	for _, well := range wells {
		well = strings.ToUpper(strings.TrimSpace(well))
		for name, existing := range groups {
			kept := existing[:0]
			for _, candidate := range existing {
				if candidate != well {
					kept = append(kept, candidate)
				}
			}
			groups[name] = kept
		}
		if target != "" {
			groups[target] = append(groups[target], well)
		}
	}
	// Deterministic order inside each group.
	for name := range groups {
		sort.Strings(groups[name])
		groups[name] = dedupe(groups[name])
	}
	source, err := v.Doc.UpdatePlate(module.ID, groups)
	if err != nil {
		return err
	}
	if v.OnChange != nil {
		v.OnChange(source)
	}
	return nil
}

func dedupe(values []string) []string {
	if len(values) < 2 {
		return values
	}
	out := values[:1]
	for _, value := range values[1:] {
		if value != out[len(out)-1] {
			out = append(out, value)
		}
	}
	return out
}

func (v *View) ensureDraft(moduleID string, data map[string]any) (map[string]*string, map[string]*bool) {
	if v.moduleDrafts == nil {
		v.moduleDrafts = map[string]map[string]*string{}
	}
	if v.moduleDraftBool == nil {
		v.moduleDraftBool = map[string]map[string]*bool{}
	}
	if v.moduleOriginals == nil {
		v.moduleOriginals = map[string]map[string]any{}
	}
	draft, ok := v.moduleDrafts[moduleID]
	orig := v.moduleOriginals[moduleID]
	changed := !ok
	if ok {
		// reset if original data changed
		for k, val := range data {
			if !sameValue(orig[k], val) {
				changed = true
				break
			}
		}
	}
	if changed {
		draft = map[string]*string{}
		bools := map[string]*bool{}
		lists := map[string][]string{}
		for key, val := range data {
			if key == "kind" || key == "id" || key == "schema" {
				continue
			}
			if items, ok := val.([]any); ok {
				values := make([]string, 0, len(items))
				allStrings := true
				for _, item := range items {
					text, isText := item.(string)
					if !isText {
						allStrings = false
						break
					}
					values = append(values, text)
				}
				if allStrings {
					lists[key] = values
					continue
				}
			}
			switch typed := val.(type) {
			case bool:
				b := typed
				bools[key] = &b
				draft[key] = nil
			case map[string]any:
				if _, ok := typed["value"]; ok {
					vs := fmt.Sprint(typed["value"])
					draft[key] = &vs
					if u, ok := typed["unit"].(string); ok {
						us := u
						draft[key+"__unit"] = &us
					}
				} else {
					vs := fmt.Sprint(typed)
					draft[key] = &vs
				}
			default:
				vs := fmt.Sprint(val)
				draft[key] = &vs
			}
		}
		v.moduleDrafts[moduleID] = draft
		v.moduleDraftBool[moduleID] = bools
		if v.moduleLists == nil {
			v.moduleLists = map[string]map[string][]string{}
		}
		v.moduleLists[moduleID] = lists
		clone := map[string]any{}
		for k, val := range data {
			clone[k] = val
		}
		v.moduleOriginals[moduleID] = clone
	}
	if v.moduleDraftBool[moduleID] == nil {
		v.moduleDraftBool[moduleID] = map[string]*bool{}
	}
	return v.moduleDrafts[moduleID], v.moduleDraftBool[moduleID]
}

func sameValue(a, b any) bool {
	return fmt.Sprint(a) == fmt.Sprint(b)
}

func (v *View) genericModuleEditor(c *ui.Context, moduleID string, data map[string]any) {
	v.GenericModuleEditor(c, moduleID, data)
}

func (v *View) GenericModuleEditor(c *ui.Context, moduleID string, data map[string]any) {
	t := c.Theme()
	kind, _ := data["kind"].(string)
	schema := 1
	switch value := data["schema"].(type) {
	case int:
		schema = value
	case int64:
		schema = int(value)
	case float64:
		schema = int(value)
	}
	registry := v.Registry
	if registry == nil {
		registry = BuiltinRegistry()
	}
	spec, ok := registry.Lookup(kind, schema)
	if !ok {
		v.moduleError(c, fmt.Errorf("no editor for %s@%d", kind, schema))
		return
	}
	draft, bools := v.ensureDraft(moduleID, data)
	canEdit := v.Can(PermDocumentEdit) && !v.ReadOnly
	ui.Column(c).FillWidth().Gap(t.Space(1)).Padding(t.Space(3)).Radius(t.Space(1.5)).
		Background(t.Surface).Border(1, t.Border).Children(func() {
		ui.Text(c, spec.Descriptor.Title).FontSize(t.Rem(1.0)).Bold().TextColor(t.Text)
		for _, input := range spec.Descriptor.Inputs {
			if input.Name == "kind" || input.Name == "id" || input.Name == "schema" {
				continue
			}
			// Composite ports are never edited through the scalar draft. Giving
			// them an empty string draft made a plain save write "" over the
			// stored value (a plate's groups, a step's tasks).
			switch input.Type {
			case "string[]", "list", "object", "table", "object-list", "well-map", "geometry", "artifact":
			default:
				if draft[input.Name] == nil {
					empty := ""
					draft[input.Name] = &empty
				}
			}
			if input.Type == "quantity" && draft[input.Name+"__unit"] == nil {
				empty := ""
				draft[input.Name+"__unit"] = &empty
			}
			ui.Row(c).FillWidth().Gap(t.Space(1)).AlignItems(ui.Center).Children(func() {
				ui.Text(c, input.Name).FontSize(t.Rem(.8)).Bold().TextColor(t.TextMuted).Width(172)
				switch input.Type {
				case "string", "reference", "artifact":
					if input.Type == "artifact" {
						ui.Text(c, "read-only").FontSize(t.Rem(.75)).TextColor(t.TextMuted)
					} else {
						ui.Box(c).Grow(1).MinWidth(0).Children(func() {
							ui.TextInput(c, draft[input.Name]).Label(input.Name).FillWidth()
						})
					}
				case "enum":
					ui.Select(c, draft[input.Name], input.Values)
				case "bool", "boolean":
					if bools[input.Name] == nil {
						b := false
						bools[input.Name] = &b
					}
					ui.Checkbox(c, bools[input.Name], "")
				case "int", "integer", "number", "decimal":
					ui.Box(c).Grow(1).MinWidth(0).Children(func() {
						ui.DecimalInput(c, draft[input.Name], "", "", "1")
					})
				case "quantity":
					ui.DecimalInput(c, draft[input.Name], "", "", "1").Label(input.Name)
					ui.Box(c).Width(96).Children(func() {
						ui.TextInput(c, draft[input.Name+"__unit"]).Label(input.Name + " unit").FillWidth()
					})
				case "string[]":
					lists := v.moduleLists[moduleID]
					if lists == nil {
						lists = map[string][]string{}
						v.moduleLists[moduleID] = lists
					}
					items := lists[input.Name]
					ui.Column(c).Gap(4).Grow(1).Children(func() {
						remove := -1
						for i := range items {
							i := i
							ui.Row(c).FillWidth().Gap(4).AlignItems(ui.Center).Children(func() {
								ui.TextInput(c, &items[i]).Label(input.Name)
								if components.IconButton(c, lucide.Trash(), "删除 "+input.Name, components.IconButtonCompact).Clicked() {
									remove = i
								}
							})
						}
						if remove >= 0 {
							items = append(items[:remove], items[remove+1:]...)
						}
						if components.Button(c, "+ 添加", components.ButtonGhost).Clicked() {
							items = append(items, "")
						}
						lists[input.Name] = items
					})
				case "list", "object", "table", "object-list", "well-map", "geometry":
					ui.Text(c, "read-only").FontSize(t.Rem(.75)).TextColor(t.TextMuted)
				default:
					ui.TextInput(c, draft[input.Name])
				}
				if input.Description != "" {
					ui.Text(c, "· "+input.Description).FontSize(t.Rem(.7)).TextColor(t.TextMuted)
				}
			})
		}
		if canEdit {
			btn := ui.Button(c, "保存")
			if btn.Clicked() && v.OnUpdateModule != nil {
				props := map[string]any{}
				orig := v.moduleOriginals[moduleID]
				for key, valptr := range draft {
					if strings.HasSuffix(key, "__unit") {
						continue
					}
					if bools[key] != nil {
						continue
					}
					val := ""
					if valptr != nil {
						val = *valptr
					}
					origVal, origExists := orig[key]
					if !origExists && val == "" {
						continue
					}
					var originalStr string
					qm, isQ := origVal.(map[string]any)
					if isQ {
						originalStr = fmt.Sprint(qm["value"])
					} else {
						originalStr = fmt.Sprint(origVal)
					}
					unitChanged := false
					if isQ {
						u, _ := qm["unit"].(string)
						if draft[key+"__unit"] != nil && *draft[key+"__unit"] != u {
							unitChanged = true
						}
					}
					if val == originalStr && !unitChanged {
						continue
					}
					if isQ {
						num := parseNumber(val)
						unit := ""
						if draft[key+"__unit"] != nil {
							unit = *draft[key+"__unit"]
						}
						props[key] = map[string]any{"value": num, "unit": unit}
					} else if bptr := bools[key]; bptr != nil {
						props[key] = *bptr
					} else {
						if inputTypeIsInt(spec, key) {
							props[key] = parseInt(val)
						} else {
							props[key] = val
						}
					}
				}
				// bool changes: bools not in draft text; handle separately
				for key, ptr := range bools {
					if ptr == nil {
						continue
					}
					origVal, origExists := orig[key].(bool)
					if !origExists && !*ptr {
						continue
					}
					if *ptr != origVal {
						props[key] = *ptr
					}
				}
				if lists := v.moduleLists[moduleID]; lists != nil {
					for name, items := range lists {
						if sameStringList(items, orig[name]) {
							continue
						}
						out := make([]any, 0, len(items))
						for _, item := range items {
							if strings.TrimSpace(item) == "" {
								continue
							}
							out = append(out, item)
						}
						props[name] = out
					}
				}
				v.OnUpdateModule(moduleID, props)
			}
		}
	})
}

// sameStringList compares an editable string list with the stored value, which
// arrives as []any from YAML/JSON.
func sameStringList(items []string, original any) bool {
	stored, ok := original.([]any)
	if !ok {
		return len(items) == 0 && original == nil
	}
	trimmed := make([]string, 0, len(items))
	for _, item := range items {
		if strings.TrimSpace(item) == "" {
			continue
		}
		trimmed = append(trimmed, item)
	}
	if len(trimmed) != len(stored) {
		return false
	}
	for i := range trimmed {
		if fmt.Sprint(stored[i]) != trimmed[i] {
			return false
		}
	}
	return true
}

// GenericModuleSummary is the shared, read-only renderer for Run surfaces.
// Hosts can request compact, inspector, or full density without changing data.
func (v *View) GenericModuleSummary(c *ui.Context, data map[string]any) {
	kind, _ := data["kind"].(string)
	schema := 1
	switch value := data["schema"].(type) {
	case int:
		schema = value
	case int64:
		schema = int(value)
	case float64:
		schema = int(value)
	}
	registry := v.Registry
	if registry == nil {
		registry = BuiltinRegistry()
	}
	spec, ok := registry.Lookup(kind, schema)
	if !ok {
		v.moduleError(c, fmt.Errorf("no Run summary for %s@%d", kind, schema))
		return
	}
	density := v.Density
	if density == "" {
		density = DensityCompact
	}
	fields := make([]Port, 0, len(spec.Inputs))
	for _, input := range spec.Inputs {
		if input.Name == "kind" || input.Name == "id" || input.Name == "schema" {
			continue
		}
		if value, exists := data[input.Name]; exists && value != nil {
			fields = append(fields, input)
		}
	}
	t := c.Theme()
	ui.Column(c).FillWidth().Gap(t.Space(1)).Padding(t.Space(2)).Radius(t.Space(1)).
		Background(t.Surface).Border(1, t.Border).Children(func() {
		ui.Text(c, spec.Title).FontSize(t.Rem(.9)).Bold().TextColor(t.Text)
		if len(fields) == 0 {
			ui.Text(c, "暂无可预览的参数").FontSize(t.Rem(.75)).TextColor(t.TextMuted)
			return
		}
		limit := len(fields)
		if density == DensityCompact && limit > 3 {
			limit = 3
		}
		for _, input := range fields[:limit] {
			value := summaryFieldValue(data[input.Name])
			if value == "" {
				continue
			}
			ui.Row(c).FillWidth().Gap(t.Space(1)).AlignItems(ui.Center).Children(func() {
				ui.Text(c, input.Name).FontSize(t.Rem(.72)).TextColor(t.TextMuted).Width(112)
				ui.Text(c, value).FontSize(t.Rem(.78)).TextColor(t.Text)
			})
			if density == DensityFull && input.Description != "" {
				ui.Text(c, input.Description).FontSize(t.Rem(.68)).TextColor(t.TextMuted)
			}
		}
		if limit < len(fields) {
			ui.Text(c, fmt.Sprintf("另有 %d 项参数", len(fields)-limit)).
				FontSize(t.Rem(.68)).TextColor(t.TextMuted)
		}
	})
}

func summaryFieldValue(value any) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case map[string]any:
		if quantity, ok := typed["value"]; ok {
			unit, _ := typed["unit"].(string)
			return strings.TrimSpace(fmt.Sprint(quantity) + " " + unit)
		}
		return "结构化数据"
	case []any:
		return fmt.Sprintf("%d 项", len(typed))
	default:
		return fmt.Sprint(value)
	}
}

func (v *View) GenericEditor(c *ui.Context, moduleID string, data map[string]any) {
	v.GenericModuleEditor(c, moduleID, data)
}
func (v *View) RenderGenericEditor(c *ui.Context, moduleID string, data map[string]any) {
	v.GenericModuleEditor(c, moduleID, data)
}

func inputTypeIsInt(spec ModuleSpec, key string) bool {
	for _, in := range spec.Descriptor.Inputs {
		if in.Name == key && (in.Type == "int" || in.Type == "integer") {
			return true
		}
	}
	return false
}

func parseNumber(s string) any {
	// preserve integer-like? use float64
	var f float64
	fmt.Sscan(s, &f)
	return f
}
func parseInt(s string) int {
	var n int
	fmt.Sscan(s, &n)
	return n
}

func (v *View) ensureWidgetDraft(key, initial string) *string {
	if v.widgetTextDrafts == nil {
		v.widgetTextDrafts = map[string]*string{}
	}
	if ptr, ok := v.widgetTextDrafts[key]; ok {
		return ptr
	}
	val := initial
	v.widgetTextDrafts[key] = &val
	return v.widgetTextDrafts[key]
}

func (v *View) ensureWidgetBool(key string) *bool {
	return v.ensureWidgetBoolInit(key, false)
}

func (v *View) ensureWidgetBoolInit(key string, initial bool) *bool {
	if v.widgetBoolDrafts == nil {
		v.widgetBoolDrafts = map[string]*bool{}
	}
	if ptr, ok := v.widgetBoolDrafts[key]; ok {
		return ptr
	}
	val := initial
	v.widgetBoolDrafts[key] = &val
	return v.widgetBoolDrafts[key]
}

func (v *View) emitAnyModuleEdit(moduleID string, props map[string]any) {
	if v.StepID != "" && v.OnUpdateStepBlock != nil {
		v.OnUpdateStepBlock(v.StepID, moduleID, props)
		return
	}
	if v.OnUpdateModule != nil {
		v.OnUpdateModule(moduleID, props)
		return
	}
	if v.Doc == nil || !v.Can(PermDocumentEdit) {
		return
	}
	source, err := v.Doc.UpdateModule(moduleID, props)
	if err != nil {
		return
	}
	if v.OnChange != nil {
		v.OnChange(source)
	}
}

func iconButton(c *ui.Context, icon *ui.SVG, label string, variant components.ButtonVariant) ui.Element {
	tok := theme.GetTokens(c.Theme())
	btn := components.Button(c, "", variant).Label(label)
	fg := tok.Text
	if variant == components.ButtonPrimary {
		fg = c.Theme().AccentText
	}
	btn.Children(func() {
		ui.Row(c).Gap(float32(tok.Spacing4)).AlignItems(ui.Center).Children(func() {
			if icon != nil {
				ui.Icon(c, icon).Size(float32(tok.Spacing12), float32(tok.Spacing12)).TextColor(fg)
			}
			components.Text(c, label).SingleLine().TextColor(fg)
		})
	})
	return btn
}
