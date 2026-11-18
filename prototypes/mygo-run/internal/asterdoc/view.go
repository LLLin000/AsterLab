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
	OnAddModule          func(module map[string]any)
	OnInsertStepBlock    func(stepID string, index int, block map[string]any)
	OnUpdateStepBlock    func(stepID, blockID string, props map[string]any)
	OnMoveStepBlock      func(stepID, blockID, direction string)
	OnMoveStepBlockIndex func(stepID, blockID string, toIndex int)
	OnRemoveStepBlock    func(stepID, blockID string)
	OnPickFigureFile     func(stepID, blockID string)
	StepID               string
	BaseDir              string
	currentModuleID      string
	titleEditID          string
	titleEditBuf         string
	// Run-mode interactive execution callbacks & state (nil-safe in read-only views).
	OnRecordActualParam  func(stepID, blockID, key, value, unit string)
	OnToggleCheck        func(stepID, checkID string, checked bool)
	OnRuleSkip           func(stepID, checkID, rule string)
	OnTimerAction        func(stepID, blockID, action string)
	OnSubmitTableReadout func(stepID, blockID string, payload map[string]any) error
	OnExcludeSample      func(sampleID, status string)
	OnLinkAsset          func(stepID, blockID, wellID string)
	OnOpenExternalAsset  func(asset AssetThumb)
	AsyncDecode          bool
	OnAsyncInvalidate    func()
	ActualParams         map[string]string
	CalcActuals          *CalcActualContext
	CheckedItems         map[string]bool
	SkippedItems         map[string]string
	TimerStates          map[string]TimerRuntimeState
	TableCells           map[string]map[string]string
	TableQualifiers      map[string]map[string]string
	TableSubmitStatus    map[string]string
	TableSubmitErrors    map[string]string
	SampleStatuses       map[string]string
	LinkedAssets         map[string][]AssetThumb
	FocusedTableCell     string
	ExpandedFigureID     string
	FigureZoom           float32
	figureOpen           bool
	ExpandedAsset        *AssetThumb
	AssetZoom            float32
	assetOpen            bool
	HideStepHeader       bool
	widgetTextDrafts     map[string]*string
	widgetBoolDrafts     map[string]*bool
	svgCache             map[string]*ui.SVG
	imgCache             map[string]ui.ImageSource
	imgLoading           map[string]bool
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
	PlateBatchRepCount  int
	PlateFactorDim      string
	PlateFactorVal      string
	PlateRoleChoice     string
	PlateSampleError    string
	PlateZoom           float32
	plateColorBy        map[string]string
	plateFactorKey      map[string]string
	plateStates         map[string]*plateui.InteractionState
	plateSelections     map[string][]string
	plateInputDrafts    map[string]*string
	overlayPlateState   plateui.InteractionState
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

// RenderPlateOverlay draws the zoomed plate overlay when ExpandedPlate is active.
// Hosts calling StepBlockComposer (Design/Run/Notebook) should call this at page root.
func (v *View) RenderPlateOverlay(c *ui.Context) {
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
	prevID := v.currentModuleID
	v.currentModuleID = moduleID
	defer func() { v.currentModuleID = prevID }()
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
	dark := t != nil && t.Dark
	factorDims := p.FactorDimensions()
	geometry, _ := geometryForPlate(p.Format, p.Topology, p.Rows, p.Columns)
	wells := map[string]plateui.WellState{}
	for well, group := range p.WellGroups() {
		wells[well] = plateui.WellState{
			Group:     group,
			SampleID:  p.Samples[well],
			Replicate: p.Replicates[well],
			Role:      p.Roles[well],
			Factors:   p.WellFactors(well),
		}
	}
	if v.plateColorBy == nil {
		v.plateColorBy = map[string]string{}
	}
	if v.plateFactorKey == nil {
		v.plateFactorKey = map[string]string{}
	}
	if v.plateStates == nil {
		v.plateStates = map[string]*plateui.InteractionState{}
	}
	if v.plateSelections == nil {
		v.plateSelections = map[string][]string{}
	}
	st := v.plateStates[p.ID]
	if st == nil {
		st = &plateui.InteractionState{}
		v.plateStates[p.ID] = st
	}
	colorBy := v.plateColorBy[p.ID]
	if colorBy == "" {
		colorBy = "group"
	}
	factorKey := v.plateFactorKey[p.ID]
	if factorKey == "" && len(factorDims) > 0 {
		factorKey = factorDims[0]
		v.plateFactorKey[p.ID] = factorKey
	}

	title := p.Title
	if strings.TrimSpace(title) == "" {
		if p.Topology == "dish" {
			title = fmt.Sprintf("培养皿分区（%d 区）", geometry.Rows*geometry.Columns)
		} else if p.Topology == "rack_1d" {
			title = fmt.Sprintf("%d 位样本架 / 泳道", geometry.Rows*geometry.Columns)
		} else {
			title = fmt.Sprintf("%d 孔板", p.Format)
		}
	}
	editable := v.Mode == ModeDesign && !v.ReadOnly && v.Can(PermDocumentEdit)
	ownerStepID := v.ownerStepForPlate(p.ID)
	selected := v.plateSelections[p.ID]
	idxMap := plateGroupPaletteMap(v.Doc, p)

	ui.Row(c).FillWidth().Children(func() {
		ui.Column(c).FillWidth().Gap(float32(tok.Spacing8)).Padding(float32(tok.Spacing12)).
			Radius(float32(tok.PanelRadius)).Background(tok.Surface).Border(1, tok.Border).Children(func() {
			ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
				ui.Column(c).Gap(float32(tok.Spacing4)).Children(func() {
					renderInlineEditableTitle(v, c, p.ID, title, editable)
					meta := p.ID
					if p.Timepoint != "" {
						meta += " · " + p.Timepoint
					}
					if len(p.Samples) > 0 || len(factorDims) > 0 {
						meta += fmt.Sprintf(" · 样本 %d · 维度 %d", len(p.Samples), len(factorDims))
					}
					ui.Text(c, meta).FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
				})
				ui.Spacer(c)
				if v.ReadOnly {
					components.Badge(c, "冻结视图")
				}
				if components.IconButton(c, lucide.Maximize2(), "放大孔板", components.IconButtonCompact).Clicked() {
					v.ExpandedPlate = p.ID
					v.ExpandedWells = append([]string(nil), selected...)
					v.PlateZoom = 1
				}
			})

			plateui.View(c, plateui.Props{
				Wells: wells, Rows: geometry.Rows, Cols: geometry.Columns,
				Topology: p.Topology, Roles: p.Roles, Interactive: editable,
				SelectedIDs: selected,
				OnSelect: func(ids []string) {
					v.plateSelections[p.ID] = ids
				},
				ColorBy: colorBy, FactorKey: factorKey, GroupIndex: idxMap, State: st,
			})

			hasMultiLayer := len(factorDims) > 0 || len(p.Roles) > 0 || len(p.Samples) > 0
			if hasMultiLayer && len(p.WellGroups()) > 0 {
				ui.Row(c).FillWidth().Wrap().Gap(float32(tok.Spacing4)).AlignItems(ui.Center).Justify(ui.Center).Children(func() {
					type layerChip struct {
						mode, key, label string
					}
					var chips []layerChip
					chips = append(chips, layerChip{"group", "", "分组"})
					if len(p.Samples) > 0 {
						chips = append(chips, layerChip{"sample", "", "样本"})
					}
					for _, dim := range factorDims {
						chips = append(chips, layerChip{"factor", dim, dim})
					}
					if len(p.Roles) > 0 {
						chips = append(chips, layerChip{"role", "", "孔角色"})
					}
					for _, ch := range chips {
						ch := ch
						active := colorBy == ch.mode && (ch.mode != "factor" || factorKey == ch.key)
						variant := components.ButtonGhost
						if active {
							variant = components.ButtonSecondary
						}
						if components.Button(c, ch.label, variant).Clicked() {
							v.plateColorBy[p.ID] = ch.mode
							if ch.mode == "factor" {
								v.plateFactorKey[p.ID] = ch.key
							}
						}
					}
				})
			}

			if editable {
				if len(selected) > 0 {
					v.renderPlateFloatingBar(c, p, ownerStepID, selected, func(next []string) {
						v.plateSelections[p.ID] = next
					}, false, 1, nil)
				} else {
					ui.Text(c, "框选孔位后，在下方悬浮条直接输入处理、基因或样本名即可标注并自动生成复孔。").
						FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
				}
			}
			_ = dark
		})
	})
}

func (v *View) ownerStepForPlate(plateID string) string {
	if v.Doc == nil || plateID == "" {
		return v.StepID
	}
	for i := range v.Doc.Modules {
		if v.Doc.Modules[i].Kind == "step" {
			if blocks, ok := v.Doc.Modules[i].Data["blocks"].([]any); ok {
				for _, raw := range blocks {
					bm, _ := raw.(map[string]any)
					if bm != nil && textValue(bm, "kind") == "plate" && textValue(bm, "id") == plateID {
						return v.Doc.Modules[i].ID
					}
				}
			}
		}
	}
	return ""
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

func (v *View) findPlate(plateID string) (plate *Plate, ownerStepID string) {
	if v.Doc == nil || plateID == "" {
		return nil, ""
	}
	for i := range v.Doc.Modules {
		if v.Doc.Modules[i].ID == plateID && v.Doc.Modules[i].Plate != nil {
			return v.Doc.Modules[i].Plate, ""
		}
		if v.Doc.Modules[i].Kind == "step" {
			if blocks, ok := v.Doc.Modules[i].Data["blocks"].([]any); ok {
				for _, raw := range blocks {
					bm, _ := raw.(map[string]any)
					if bm != nil && textValue(bm, "kind") == "plate" && textValue(bm, "id") == plateID {
						if p, err := plateFromData(bm); err == nil {
							return &p, v.Doc.Modules[i].ID
						}
					}
				}
			}
		}
	}
	return nil, ""
}

func (v *View) applyPlateProps(plateID, ownerStepID string, props map[string]any) error {
	if ownerStepID != "" {
		if v.OnUpdateStepBlock != nil {
			v.OnUpdateStepBlock(ownerStepID, plateID, props)
			return nil
		}
		if v.Doc != nil && v.Can(PermDocumentEdit) {
			if err := v.updateStepBlockInDoc(ownerStepID, plateID, props); err != nil {
				return err
			}
			if v.OnChange != nil {
				v.OnChange(v.Doc.Source)
			}
		}
		return nil
	}
	if v.OnUpdateModule != nil {
		v.OnUpdateModule(plateID, props)
		return nil
	}
	if v.Doc != nil && v.Can(PermDocumentEdit) {
		if _, err := v.Doc.UpdateModule(plateID, props); err != nil {
			return err
		}
		if v.OnChange != nil {
			v.OnChange(v.Doc.Source)
		}
	}
	return nil
}

// plateOverlay floats the enlarged plate over a dimmed canvas using the exact
// same unified plateui.View component and floating single-input annotation bar.
func (v *View) plateOverlay(c *ui.Context) {
	if v.ExpandedPlate == "" {
		return
	}
	plate, ownerStepID := v.findPlate(v.ExpandedPlate)
	if plate == nil {
		v.ExpandedPlate = ""
		return
	}
	t := c.Theme()
	tok := theme.GetTokens(t)
	geometry, _ := geometryForPlate(plate.Format, plate.Topology, plate.Rows, plate.Columns)
	factorDims := plate.FactorDimensions()
	if v.plateColorBy == nil {
		v.plateColorBy = map[string]string{}
	}
	if v.plateFactorKey == nil {
		v.plateFactorKey = map[string]string{}
	}
	colorBy := v.plateColorBy[plate.ID]
	if colorBy == "" {
		colorBy = "group"
	}
	factorKey := v.plateFactorKey[plate.ID]
	if factorKey == "" && len(factorDims) > 0 {
		factorKey = factorDims[0]
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
	wells := map[string]plateui.WellState{}
	for well, group := range plate.WellGroups() {
		wells[well] = plateui.WellState{
			Group:     group,
			SampleID:  plate.Samples[well],
			Replicate: plate.Replicates[well],
			Role:      plate.Roles[well],
			Factors:   plate.WellFactors(well),
		}
	}
	idxMap := plateGroupPaletteMap(v.Doc, plate)

	ui.Overlay(c, func() {
		backdrop := ui.Box(c).Absolute().Left(0).Top(0).Right(0).Bottom(0).
			Background(ui.RGBA(0, 0, 0, .62)).Modal()
		if backdrop.Clicked() {
			closeOverlay()
		}
		layer := ui.Column(c).Absolute().Left(0).Top(0).Right(0).Bottom(0).
			AlignItems(ui.Center).Justify(ui.Center).Padding(float32(tok.Spacing24)).Modal()
		if layer.OverlayShortcut(0, ui.KeyEscape) {
			closeOverlay()
		}
		layer.Children(func() {
			ui.Column(c).MaxWidth(float32(tok.Spacing48)*20).Gap(float32(tok.Spacing8)).AlignItems(ui.Center).Children(func() {
				ui.Box(c).Background(tok.Surface).Border(1, tok.Border).
					Radius(float32(tok.DialogRadius)).Shadow(0, float32(tok.Spacing12), float32(tok.Spacing32), 0, ui.RGBA(0, 0, 0, .38)).
					Padding(float32(tok.Spacing16), float32(tok.Spacing16)).Children(func() {
					ui.Column(c).Gap(float32(tok.Spacing8)).AlignItems(ui.Center).Children(func() {
						plateTitle := plate.Title
						if strings.TrimSpace(plateTitle) == "" {
							if plate.Topology == "dish" {
								plateTitle = fmt.Sprintf("培养皿分区（%d 区）", geometry.Rows*geometry.Columns)
							} else {
								plateTitle = fmt.Sprintf("%d 孔板", plate.Format)
							}
						}
						ui.Row(c).FillWidth().AlignItems(ui.Center).Wrap().Gap(float32(tok.Spacing8)).Children(func() {
							ui.Column(c).Gap(2).Children(func() {
								ui.Text(c, plateTitle).FontSize(float32(tok.TextSection)).Bold().TextColor(tok.Text)
								meta := fmt.Sprintf("%s · %d 孔 · 已选 %d 孔", plate.ID, plate.Format, len(v.ExpandedWells))
								if len(factorDims) > 0 {
									meta += " · 维度：" + strings.Join(factorDims, " / ")
								}
								ui.Text(c, meta).FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
							})
							ui.Spacer(c)
							if len(factorDims) > 0 || len(plate.Roles) > 0 || len(plate.Samples) > 0 {
								ui.Row(c).Wrap().Gap(float32(tok.Spacing4)).AlignItems(ui.Center).Children(func() {
									if components.Button(c, "分组", map[bool]components.ButtonVariant{true: components.ButtonSecondary, false: components.ButtonGhost}[colorBy == "group"]).Clicked() {
										v.plateColorBy[plate.ID] = "group"
										colorBy = "group"
									}
									if len(plate.Samples) > 0 {
										if components.Button(c, "样本", map[bool]components.ButtonVariant{true: components.ButtonSecondary, false: components.ButtonGhost}[colorBy == "sample"]).Clicked() {
											v.plateColorBy[plate.ID] = "sample"
											colorBy = "sample"
										}
									}
									for _, dim := range factorDims {
										dim := dim
										if components.Button(c, dim, map[bool]components.ButtonVariant{true: components.ButtonSecondary, false: components.ButtonGhost}[colorBy == "factor" && factorKey == dim]).Clicked() {
											v.plateColorBy[plate.ID] = "factor"
											v.plateFactorKey[plate.ID] = dim
											colorBy = "factor"
											factorKey = dim
										}
									}
									if len(plate.Roles) > 0 {
										if components.Button(c, "孔角色", map[bool]components.ButtonVariant{true: components.ButtonSecondary, false: components.ButtonGhost}[colorBy == "role"]).Clicked() {
											v.plateColorBy[plate.ID] = "role"
											colorBy = "role"
										}
									}
								})
							}
						})
						overlayZoom := zoom * 1.2
						if geometry.Rows*geometry.Columns > 96 {
							overlayZoom = zoom
						}
						ui.ScrollBoth(c).MaxWidth(float32(tok.Spacing48)*19).MaxHeight(580).Children(func() {
							plateui.View(c, plateui.Props{
								Wells: wells, Rows: geometry.Rows, Cols: geometry.Columns,
								Topology: plate.Topology, Roles: plate.Roles,
								Interactive: canEdit || true,
								SelectedIDs: v.ExpandedWells,
								OnSelect: func(ids []string) {
									v.ExpandedWells = ids
								},
								ColorBy: colorBy, FactorKey: factorKey, GroupIndex: idxMap,
								Zoom: overlayZoom, State: &v.overlayPlateState,
							})
						})
					})
				})

				v.renderPlateFloatingBar(c, plate, ownerStepID, v.ExpandedWells, func(next []string) {
					v.ExpandedWells = next
				}, true, zoom, closeOverlay)
			})
		})
	})
}

func overlayWellTag(props plateui.Props, ws plateui.WellState, role string) string {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "ntc":
		return "NTC"
	case "blank":
		return "BLK"
	case "standard":
		return "STD"
	case "positive_control":
		return "+"
	case "negative_control":
		return "-"
	case "vehicle":
		return "V"
	}
	if props.ColorBy == "replicate" && ws.Replicate > 0 {
		return fmt.Sprintf("#%d", ws.Replicate)
	}
	if props.ColorBy == "factor" && props.FactorKey != "" && ws.Factors[props.FactorKey] != "" {
		rs := []rune(ws.Factors[props.FactorKey])
		if len(rs) > 3 {
			return string(rs[:3])
		}
		return ws.Factors[props.FactorKey]
	}
	if ws.Replicate > 0 {
		return fmt.Sprintf("%d", ws.Replicate)
	}
	return ""
}

func overlayContrastInk(bg ui.Color, tok theme.Tokens) ui.Color {
	if bg.A == 0 {
		return tok.Text
	}
	lum := (0.299*float64(bg.R) + 0.587*float64(bg.G) + 0.114*float64(bg.B)) / 255.0
	if lum > 0.54 {
		return theme.LightTokens().Text
	}
	return theme.DarkTokens().Text
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
	return ApplyPlateSamplesWithFactors(geometry, groups, samples, replicates, nil, wells, sampleID, startReplicate)
}

// ApplyPlateSamplesWithFactors maps one sample ID onto selected wells while
// scoping technical replicate uniqueness by (sampleID, factorSignature, replicate).
func ApplyPlateSamplesWithFactors(geometry Geometry, groups map[string][]string, samples map[string]string, replicates map[string]int, factors map[string]map[string]string, wells []string, sampleID string, startReplicate int) (map[string]string, map[string]int, error) {
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
	factorSigs := factorSignatureMap(factors)
	replicateOwner := map[string]string{}
	for well, replicate := range outReplicates {
		if selectedSet[well] || replicate < 1 || outSamples[well] == "" {
			continue
		}
		replicateOwner[outSamples[well]+"\x00"+factorSigs[well]+"\x00"+strconv.Itoa(replicate)] = well
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
			key := sampleID + "\x00" + factorSigs[well] + "\x00" + strconv.Itoa(replicate)
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

// ApplyPlateBatchSamples splits comma/newline-separated sample IDs evenly across selected wells
// with automatic per-sample replicate numbering (#1..#repCount).
func ApplyPlateBatchSamples(geometry Geometry, groups map[string][]string, samples map[string]string, replicates map[string]int, factors map[string]map[string]string, wells []string, sampleSpec string, repCount int) (map[string]string, map[string]int, error) {
	if repCount < 1 {
		repCount = 1
	}
	rawTokens := strings.FieldsFunc(sampleSpec, func(r rune) bool {
		return r == ',' || r == '，' || r == '\n' || r == '\r' || r == ';' || r == '；'
	})
	var sampleList []string
	for _, tok := range rawTokens {
		if t := strings.TrimSpace(tok); t != "" {
			sampleList = append(sampleList, t)
		}
	}
	if len(sampleList) == 0 {
		return nil, nil, fmt.Errorf("请填写至少一个样本 ID")
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
		w := strings.ToUpper(strings.TrimSpace(well))
		if w == "" || seen[w] {
			continue
		}
		if !validWell(w, geometry) {
			return nil, nil, fmt.Errorf("无效孔位 %q", w)
		}
		if _, ok := groupByWell[w]; !ok {
			return nil, nil, fmt.Errorf("孔位 %s 尚未分配实验分组", w)
		}
		seen[w] = true
		selected = append(selected, w)
	}
	sort.Slice(selected, func(i, j int) bool {
		return wellLess(geometry, selected[i], selected[j])
	})
	selectedSet := map[string]bool{}
	for _, w := range selected {
		selectedSet[w] = true
	}
	factorSigs := factorSignatureMap(factors)
	replicateOwner := map[string]string{}
	for well, rep := range outReplicates {
		if selectedSet[well] || rep < 1 || outSamples[well] == "" {
			continue
		}
		replicateOwner[outSamples[well]+"\x00"+factorSigs[well]+"\x00"+strconv.Itoa(rep)] = well
	}
	for idx, w := range selected {
		sIdx := (idx / repCount) % len(sampleList)
		sid := sampleList[sIdx]
		rep := (idx % repCount) + 1
		// Advance rep if same sample wraps or repeats in a non-orthogonal block
		for {
			key := sid + "\x00" + factorSigs[w] + "\x00" + strconv.Itoa(rep)
			if _, taken := replicateOwner[key]; !taken {
				replicateOwner[key] = w
				break
			}
			rep++
		}
		outSamples[w] = sid
		outReplicates[w] = rep
	}
	return outSamples, outReplicates, nil
}

// ApplyPlateFactor assigns (or clears when factorVal=="") an orthogonal factor value across selected wells.
func ApplyPlateFactor(geometry Geometry, groups map[string][]string, factors map[string]map[string]string, wells []string, factorDim, factorVal string) (map[string]map[string]string, error) {
	dim := strings.TrimSpace(factorDim)
	if dim == "" {
		return nil, fmt.Errorf("请填写因子维度名称（如 靶点基因、加药浓度）")
	}
	val := strings.TrimSpace(factorVal)
	groupByWell := map[string]string{}
	for group, assigned := range groups {
		for _, well := range assigned {
			groupByWell[strings.ToUpper(strings.TrimSpace(well))] = strings.TrimSpace(group)
		}
	}
	out := make(map[string]map[string]string, len(factors)+1)
	for d, m := range factors {
		sub := make(map[string]string, len(m))
		for k, v := range m {
			sub[k] = v
		}
		out[d] = sub
	}
	if out[dim] == nil {
		out[dim] = map[string]string{}
	}
	for _, well := range wells {
		w := strings.ToUpper(strings.TrimSpace(well))
		if !validWell(w, geometry) {
			return nil, fmt.Errorf("无效孔位 %q", w)
		}
		if _, ok := groupByWell[w]; !ok {
			return nil, fmt.Errorf("孔位 %s 尚未分配实验分组", w)
		}
		if val == "" {
			delete(out[dim], w)
		} else {
			out[dim][w] = val
		}
	}
	if len(out[dim]) == 0 {
		delete(out, dim)
	}
	return out, nil
}

// ApplyPlateRole assigns a control/sample role across selected wells.
func ApplyPlateRole(geometry Geometry, roles map[string]string, wells []string, role string) (map[string]string, error) {
	r := strings.TrimSpace(role)
	out := make(map[string]string, len(roles)+len(wells))
	for w, v := range roles {
		out[strings.ToUpper(strings.TrimSpace(w))] = v
	}
	for _, well := range wells {
		w := strings.ToUpper(strings.TrimSpace(well))
		if !validWell(w, geometry) {
			return nil, fmt.Errorf("无效孔位 %q", w)
		}
		if r == "" {
			delete(out, w)
		} else {
			out[w] = r
		}
	}
	return out, nil
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
				ui.Text(c, genericPortLabel(input.Name)).FontSize(t.Rem(.8)).Bold().TextColor(t.TextMuted).Width(172)
				switch input.Type {
				case "string", "reference", "artifact":
					if input.Type == "artifact" {
						ui.Text(c, "结构化列表（在步骤工作台或源码中编辑）").FontSize(t.Rem(.75)).TextColor(t.TextMuted)
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
					ui.Text(c, "结构化列表（在步骤工作台或源码中编辑）").FontSize(t.Rem(.75)).TextColor(t.TextMuted)
				default:
					ui.TextInput(c, draft[input.Name])
				}
				if input.Description != "" {
					ui.Text(c, "· "+input.Description).FontSize(t.Rem(.7)).TextColor(t.TextMuted)
				}
			})
		}
		if canEdit {
			btn := components.Button(c, "保存", components.ButtonSecondary)
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

var genericPortLabelMap = map[string]string{
	"title":               "标题",
	"name":                "名称",
	"objective":           "实验目的",
	"role":                "组别角色",
	"condition":           "处理条件",
	"independent_n":       "生物学重复 n",
	"technical_repeats":   "技术重复",
	"schema_id":           "结构标识",
	"endpoint":            "观测终点",
	"value_kind":          "数据类型",
	"unit":                "单位",
	"plate_format":        "容器规格",
	"timepoints":          "采样时间点",
	"format":              "孔数规格",
	"topology":            "容器拓扑",
	"stock_concentration": "母液浓度",
	"storage":             "储存条件",
	"lot":                 "批号",
	"catalog":             "货号",
	"cas":                 "CAS 号",
	"mode":                "模式",
	"groups":              "孔位分组",
	"samples":             "样本映射",
	"replicates":          "复孔编号",
	"roles":               "孔位角色",
	"factors":             "多维因子",
	"rows":                "明细行",
	"columns":             "数据列",
	"layout":              "视图布局",
	"container_ref":       "关联容器",
	"plate_ref":           "关联孔板",
	"body":                "正文内容",
	"src":                 "图片路径",
	"svg":                 "矢量源码",
	"caption":             "图注",
	"duration":            "持续时长",
	"offset":              "时间窗偏移",
	"rounds":              "循环轮数",
	"phases":              "循环阶段",
	"items":               "条目列表",
	"media_hint":          "采集说明",
	"scope":               "附件范围",
	"slots_per_cell":      "每孔采图槽数",
	"target_mass":         "目标上样量",
	"volume_per_unit":     "单份体积",
	"excess_ratio":        "余量比例",
	"dilution_factor":     "稀释倍数",
	"steps_count":         "梯度级数",
	"stock":               "母液浓度",
	"target":              "目标浓度",
	"final_volume":        "终体积",
	"formula":             "计算公式",
	"timepoint":           "时间点",
	"missing_reasons":     "缺失原因枚举",
	"null_allowed":        "允许空值",
	"source_kinds":        "数据来源类型",
	"table_columns":       "表格列定义",
	"method":              "检测方法",
	"group_refs":          "关联分组",
	"protocol_version_id": "方案版本标识",
	"controls":            "对照设置",
	"timepoint_relation":  "时间点关联",
	"technical_replicates":"技术重复数",
	"control_group_ref":   "对照分组引用",
	"sample_id":           "样本编号",
	"group_ref":           "所属分组",
	"material_refs":       "关联试剂/材料",
	"amount":              "用量/体积",
	"class":               "仪器类别",
	"model":               "型号",
	"capabilities":        "能力清单",
	"constraints":         "使用约束",
	"source":              "来源渠道",
	"pantry_code":         "储藏室短码",
	"steps":               "步骤序列",
	"edges":               "依赖连线",
	"business_order":      "业务顺序",
	"execution":           "执行属性",
	"assay":               "关联检测",
	"path":                "步骤路径",
	"order":               "排序序号",
	"block_refs":          "共享模块引用",
	"blocks":              "步骤内模块",
	"checks":              "检查项",
	"tasks":               "任务列表",
	"uses":                "关联资源",
	"experimental_unit":   "实验单元",
	"primary_endpoint":    "主要终点",
	"endpoint_method":     "测定方法",
	"endpoint_unit":       "终点单位",
}

func genericPortLabel(name string) string {
	if lbl, ok := genericPortLabelMap[name]; ok {
		return lbl
	}
	return name
}
