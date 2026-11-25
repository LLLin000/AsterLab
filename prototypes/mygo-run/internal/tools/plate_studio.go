package tools

import (
	"fmt"
	"runtime"
	"strconv"
	"strings"
	"time"

	"asterlab-mygo-prototype/internal/asterdoc"
	"asterlab-mygo-prototype/internal/theme"
	plateui "asterlab-mygo-prototype/internal/uikit/plate"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/onevra/source/components"
	"github.com/oneo-me/onevra/source/icons/lucide"
	"gopkg.in/yaml.v3"
)

const mobileBreakpointWidth = float32(680)

func isMobileToolsLayout(c *ui.Context) bool {
	if runtime.GOOS == "android" || runtime.GOOS == "ios" {
		return true
	}
	if c == nil {
		return false
	}
	w, _ := c.Size()
	return w > 0 && w < mobileBreakpointWidth
}

type mobilePlateSheetKey struct{ w *Workspace }

// PlatePreset describes a standard container geometry selectable before designing.
type PlatePreset struct {
	Key          string
	Label        string
	Format       int
	Rows         int
	Cols         int
	Topology     string
	DefaultTitle string
}

// StandardPlatePresets enumerates the supported plate and strip geometries
// available in the "先选孔板，再设计" step. Continuous agar plates (such as
// 90 mm Petri dishes) use agent-authored SVG diagrams rather than discrete grids.
var StandardPlatePresets = []PlatePreset{
	{Key: "plate-6", Label: "6孔板", Format: 6, Rows: 2, Cols: 3, Topology: "plate", DefaultTitle: "6 孔培养板"},
	{Key: "plate-12", Label: "12孔板", Format: 12, Rows: 3, Cols: 4, Topology: "plate", DefaultTitle: "12 孔培养板"},
	{Key: "plate-24", Label: "24孔板", Format: 24, Rows: 4, Cols: 6, Topology: "plate", DefaultTitle: "24 孔培养板"},
	{Key: "plate-48", Label: "48孔板", Format: 48, Rows: 6, Cols: 8, Topology: "plate", DefaultTitle: "48 孔培养板"},
	{Key: "plate-96", Label: "96孔板", Format: 96, Rows: 8, Cols: 12, Topology: "plate", DefaultTitle: "96 孔排板设计"},
	{Key: "plate-384", Label: "384孔板", Format: 384, Rows: 16, Cols: 24, Topology: "plate", DefaultTitle: "384 孔高通量排板"},
	{Key: "rack-8", Label: "8联管/泳道", Format: 8, Rows: 1, Cols: 8, Topology: "rack_1d", DefaultTitle: "8 联管 / 样本泳道"},
}

func default96Preset() PlatePreset {
	if p, ok := presetForKey("plate-96"); ok {
		return p
	}
	return StandardPlatePresets[0]
}

// PlateHistoryEntry persists a complete AsterML plate snapshot for quick reload
// and direct SVG/PNG export from the 2-column history wall.
type PlateHistoryEntry struct {
	ID            string `json:"id"`
	Title         string `json:"title"`
	Format        int    `json:"format"`
	Rows          int    `json:"rows"`
	Cols          int    `json:"cols"`
	Topology      string `json:"topology"`
	AssignedWells int    `json:"assignedWells"`
	GroupCount    int    `json:"groupCount"`
	UpdatedAt     string `json:"updatedAt"`
	Source        string `json:"source"`
}

const (
	studioPlateID     = "plate-studio"
	maxHistoryEntries = 24
)

func presetForKey(key string) (PlatePreset, bool) {
	for _, p := range StandardPlatePresets {
		if p.Key == key {
			return p, true
		}
	}
	return PlatePreset{}, false
}

func presetForPlate(p *asterdoc.Plate) PlatePreset {
	if p != nil {
		topo := strings.TrimSpace(p.Topology)
		if topo == "" {
			topo = "plate"
		}
		for _, candidate := range StandardPlatePresets {
			if candidate.Topology == topo && candidate.Format == p.Format {
				return candidate
			}
		}
		geom, ok := asterdoc.GeometryForPlate(p.Format, topo, p.Rows, p.Columns)
		if ok {
			return PlatePreset{
				Key:          fmt.Sprintf("%s-%d", topo, p.Format),
				Format:       geom.Format,
				Rows:         geom.Rows,
				Cols:         geom.Columns,
				Topology:     topo,
				DefaultTitle: p.Title,
			}
		}
	}
	return default96Preset()
}

func buildStudioPlateSource(preset PlatePreset, title string) string {
	t := strings.TrimSpace(title)
	if t == "" {
		t = preset.DefaultTitle
	}
	plateMap := map[string]any{
		"kind":   "plate",
		"id":     studioPlateID,
		"schema": 1,
		"title":  t,
		"format": preset.Format,
		"groups": map[string][]string{},
	}
	if preset.Topology != "" && preset.Topology != "plate" {
		plateMap["topology"] = preset.Topology
		plateMap["rows"] = preset.Rows
		plateMap["columns"] = preset.Cols
	}
	expMap := map[string]any{
		"kind":   "experiment",
		"id":     "exp-plate-studio",
		"schema": 1,
		"title":  t,
	}
	expRaw, _ := yaml.Marshal(expMap)
	plateRaw, _ := yaml.Marshal(plateMap)
	return "```asterlab\n" + string(expRaw) + "```\n\n```asterlab\n" + string(plateRaw) + "```\n"
}

func (w *Workspace) initPlateStudio() {
	if len(w.PlateHistory) == 0 && w.Callbacks.LoadPlateHistory != nil {
		if loaded, err := w.Callbacks.LoadPlateHistory(); err == nil && len(loaded) > 0 {
			filtered := make([]PlateHistoryEntry, 0, len(loaded))
			for _, entry := range loaded {
				if entry.Topology == "dish" || entry.Format == 4 {
					continue
				}
				filtered = append(filtered, entry)
			}
			w.PlateHistory = filtered
		}
	}
	defaultPreset := default96Preset()
	w.ActivePlatePreset = defaultPreset.Key
	w.PlateFormat = defaultPreset.Format
	w.PlateTopology = defaultPreset.Topology
	src := buildStudioPlateSource(defaultPreset, defaultPreset.DefaultTitle)
	w.loadPlateDocumentSource(src, "")
}

func (w *Workspace) loadPlateDocumentSource(source, historyID string) bool {
	doc, err := asterdoc.Parse(source)
	if err != nil || doc == nil {
		return false
	}
	plate := findPrimaryPlate(doc)
	if plate == nil {
		return false
	}
	preset := presetForPlate(plate)
	w.plateDoc = doc
	w.ActivePlatePreset = preset.Key
	w.PlateFormat = preset.Format
	w.PlateTopology = preset.Topology
	w.currentHistoryID = historyID
	if w.plateView == nil {
		w.plateView = &asterdoc.View{
			Mode: asterdoc.ModeDesign,
			Allowed: map[string]bool{
				asterdoc.PermDocumentEdit: true,
			},
			OnChange: func(nextSource string) {
				w.onPlateDocChange(nextSource)
			},
		}
	}
	w.plateView.Doc = doc
	w.plateView.ExpandedPlate = ""
	w.plateView.ExpandedWells = nil
	w.plateView.PlateSampleError = ""
	w.plateView.SetPlateSelection(plate.ID, nil)
	w.syncLegacyPlateFromDoc()
	return true
}

func findPrimaryPlate(doc *asterdoc.Document) *asterdoc.Plate {
	if doc == nil {
		return nil
	}
	for i := range doc.Modules {
		if doc.Modules[i].Plate != nil {
			return doc.Modules[i].Plate
		}
	}
	return nil
}

func (w *Workspace) onPlateDocChange(nextSource string) {
	if nextSource != "" {
		if parsed, err := asterdoc.Parse(nextSource); err == nil {
			w.plateDoc = parsed
			if w.plateView != nil {
				w.plateView.Doc = parsed
			}
		}
	}
	w.syncLegacyPlateFromDoc()
}

func (w *Workspace) syncLegacyPlateFromDoc() {
	plate := findPrimaryPlate(w.plateDoc)
	if plate == nil {
		return
	}
	geom, ok := asterdoc.GeometryForPlate(plate.Format, plate.Topology, plate.Rows, plate.Columns)
	if !ok {
		return
	}
	w.PlateFormat = plate.Format
	topo := strings.TrimSpace(plate.Topology)
	if topo == "" {
		topo = "plate"
	}
	w.PlateTopology = topo
	w.Plate.Props.Rows = geom.Rows
	w.Plate.Props.Cols = geom.Columns
	w.Plate.Props.Groups = map[int]string{}
	w.Plate.Props.Labels = map[int]string{}
	for well, group := range plate.WellGroups() {
		if idx, ok := legacyWellIdx(well, geom.Rows, geom.Columns); ok {
			w.Plate.Props.Groups[idx] = group
			if s := plate.Samples[well]; s != "" {
				w.Plate.Props.Labels[idx] = s
			}
		}
	}
}

func legacyWellIdx(well string, rows, cols int) (int, bool) {
	well = strings.ToUpper(strings.TrimSpace(well))
	if len(well) < 2 || well[0] < 'A' || well[0] > 'Z' {
		return 0, false
	}
	r := int(well[0] - 'A')
	col, err := strconv.Atoi(well[1:])
	if err != nil || col < 1 || col > cols || r < 0 || r >= rows {
		return 0, false
	}
	return r*12 + (col - 1), true
}

func (w *Workspace) activePlatePropsForExport() (plateui.Props, string, bool) {
	if w.plateDoc == nil {
		return plateui.Props{}, "", false
	}
	colorBy, factorKey := "group", ""
	plate := findPrimaryPlate(w.plateDoc)
	if plate != nil && w.plateView != nil {
		colorBy, factorKey = w.plateView.PlateColorLayer(plate.ID)
	}
	return docToPlateProps(w.plateDoc, colorBy, factorKey)
}

func docToPlateProps(doc *asterdoc.Document, colorBy, factorKey string) (plateui.Props, string, bool) {
	plate := findPrimaryPlate(doc)
	if plate == nil {
		return plateui.Props{}, "", false
	}
	geom, ok := asterdoc.GeometryForPlate(plate.Format, plate.Topology, plate.Rows, plate.Columns)
	if !ok {
		return plateui.Props{}, "", false
	}
	wells := make(map[string]plateui.WellState)
	for well, group := range plate.WellGroups() {
		wells[well] = plateui.WellState{
			Group:     group,
			SampleID:  plate.Samples[well],
			Replicate: plate.Replicates[well],
			Role:      plate.Roles[well],
			Factors:   plate.WellFactors(well),
		}
	}
	if colorBy == "" {
		colorBy = "group"
	}
	title := strings.TrimSpace(plate.Title)
	if title == "" {
		title = presetForPlate(plate).DefaultTitle
	}
	return plateui.Props{
		Wells:       wells,
		Rows:        geom.Rows,
		Cols:        geom.Columns,
		Topology:    plate.Topology,
		Roles:       plate.Roles,
		ColorBy:     colorBy,
		FactorKey:   factorKey,
		GroupIndex:  asterdoc.PlateGroupPaletteMap(doc, plate),
		GroupLabels: asterdoc.PlateGroupLabelMap(doc),
	}, title, true
}

func (w *Workspace) activePlateAssignedCount() int {
	plate := findPrimaryPlate(w.plateDoc)
	if plate == nil {
		return 0
	}
	return len(plate.WellGroups())
}

// SelectPlatePreset switches the active plate format ("先选孔板，再设计"),
// automatically archiving any currently designed wells into PlateHistory first.
func (w *Workspace) SelectPlatePreset(presetKey string) {
	preset, ok := presetForKey(presetKey)
	if !ok {
		return
	}
	if w.activePlateAssignedCount() > 0 {
		w.SaveCurrentPlateToHistory()
	}
	w.loadPlateDocumentSource(buildStudioPlateSource(preset, preset.DefaultTitle), "")
	w.Error, w.Status = "", fmt.Sprintf("已切换为 %s，可直接在下方框选孔位标注分组、样本与复孔", preset.Label)
}

// ResetBlankPlate clears the top designer to a blank plate of the active format
// after archiving the current non-empty layout to history.
func (w *Workspace) ResetBlankPlate() {
	preset, ok := presetForKey(w.ActivePlatePreset)
	if !ok {
		preset = default96Preset()
	}
	if w.activePlateAssignedCount() > 0 {
		w.SaveCurrentPlateToHistory()
	}
	w.loadPlateDocumentSource(buildStudioPlateSource(preset, preset.DefaultTitle), "")
	w.Error, w.Status = "", fmt.Sprintf("已新建空白 %s", preset.Label)
}

// SelectPlateWells sets the currently selected well IDs in the top studio plate.
func (w *Workspace) SelectPlateWells(wells []string) {
	if w.plateView == nil {
		return
	}
	plate := findPrimaryPlate(w.plateDoc)
	if plate == nil {
		return
	}
	w.plateView.SetPlateSelection(plate.ID, wells)
}

// AnnotateSelectedWells applies unified grouping/sample/replicate/factor/role syntax
// to currently selected wells using the exact asterdoc floating-bar engine.
func (w *Workspace) AnnotateSelectedWells(rawInput string) error {
	if w.plateDoc == nil || w.plateView == nil {
		return fmt.Errorf("孔板设计器尚未就绪")
	}
	plate := findPrimaryPlate(w.plateDoc)
	if plate == nil {
		return fmt.Errorf("未找到当前孔板")
	}
	selected := w.plateView.PlateSelection(plate.ID)
	res, err := asterdoc.ApplyUnifiedPlateAnnotation(w.plateDoc, plate, selected, rawInput)
	if err != nil {
		return err
	}
	if res.UpdateGroupTableID != "" {
		_, err = w.plateDoc.UpdateModule(res.UpdateGroupTableID, map[string]any{"rows": res.UpdateGroupTableRows})
		if err != nil {
			return err
		}
	} else if res.NewGroupModule != nil {
		raw, err := yaml.Marshal(res.NewGroupModule)
		if err != nil {
			return err
		}
		fence := "\n```asterlab\n" + string(raw) + "```\n"
		nextDoc, err := asterdoc.Parse(w.plateDoc.Source + fence)
		if err != nil {
			return err
		}
		w.plateDoc = nextDoc
		w.plateView.Doc = nextDoc
	}
	if _, err := w.plateDoc.UpdateModule(plate.ID, res.PlateProps); err != nil {
		return err
	}
	w.plateView.SetPlateSelection(plate.ID, nil)
	w.onPlateDocChange(w.plateDoc.Source)
	w.Error, w.Status = "", res.AppliedSummary
	return nil
}

// SaveCurrentPlateToHistory snapshots the current designer plate into PlateHistory
// and invokes SavePlateHistory persistence when wired.
func (w *Workspace) SaveCurrentPlateToHistory() (PlateHistoryEntry, bool) {
	if w.plateDoc == nil {
		return PlateHistoryEntry{}, false
	}
	plate := findPrimaryPlate(w.plateDoc)
	if plate == nil {
		return PlateHistoryEntry{}, false
	}
	geom, ok := asterdoc.GeometryForPlate(plate.Format, plate.Topology, plate.Rows, plate.Columns)
	if !ok {
		return PlateHistoryEntry{}, false
	}
	wells := plate.WellGroups()
	if len(wells) == 0 {
		w.Error = "当前孔板尚无标注内容，请先框选孔位并标注分组或样本"
		return PlateHistoryEntry{}, false
	}
	preset := presetForPlate(plate)
	title := strings.TrimSpace(plate.Title)
	if title == "" {
		title = preset.DefaultTitle
	}
	topo := strings.TrimSpace(plate.Topology)
	if topo == "" {
		topo = "plate"
	}

	// Skip duplicate consecutive snapshot if source matches an existing history entry at top.
	if w.currentHistoryID != "" {
		for i := range w.PlateHistory {
			if w.PlateHistory[i].ID == w.currentHistoryID && w.PlateHistory[i].Source == w.plateDoc.Source {
				return w.PlateHistory[i], true
			}
		}
	}
	if len(w.PlateHistory) > 0 && w.PlateHistory[0].Source == w.plateDoc.Source {
		w.currentHistoryID = w.PlateHistory[0].ID
		return w.PlateHistory[0], true
	}

	entryID := w.currentHistoryID
	if entryID == "" {
		w.NextDraftID++
		entryID = fmt.Sprintf("hist-%d-%d", time.Now().Unix()%100000, w.NextDraftID)
	}
	entry := PlateHistoryEntry{
		ID:            entryID,
		Title:         title,
		Format:        geom.Format,
		Rows:          geom.Rows,
		Cols:          geom.Columns,
		Topology:      topo,
		AssignedWells: len(wells),
		GroupCount:    len(plate.GroupNames()),
		UpdatedAt:     time.Now().Format("01-02 15:04"),
		Source:        w.plateDoc.Source,
	}

	next := make([]PlateHistoryEntry, 0, len(w.PlateHistory)+1)
	next = append(next, entry)
	for _, existing := range w.PlateHistory {
		if existing.ID == entry.ID {
			continue
		}
		next = append(next, existing)
	}
	if len(next) > maxHistoryEntries {
		next = next[:maxHistoryEntries]
	}
	w.PlateHistory = next
	w.currentHistoryID = entry.ID
	w.persistPlateHistory()
	w.Error, w.Status = "", fmt.Sprintf("已保存「%s」至孔板历史", entry.Title)
	return entry, true
}

// LoadHistoryEntry promotes a historical plate card into the top editable designer.
func (w *Workspace) LoadHistoryEntry(id string) bool {
	for _, entry := range w.PlateHistory {
		if entry.ID != id {
			continue
		}
		// Auto-archive current unsaved design before replacing the top canvas.
		if w.activePlateAssignedCount() > 0 && w.currentHistoryID != entry.ID {
			w.SaveCurrentPlateToHistory()
		}
		if !w.loadPlateDocumentSource(entry.Source, entry.ID) {
			w.Error = "历史记录解析失败，无法载入"
			return false
		}
		w.Error, w.Status = "", fmt.Sprintf("已载入历史孔板「%s」，可继续框选编辑或导出", entry.Title)
		return true
	}
	return false
}

// DeleteHistoryEntry removes a saved entry from the 2-column history grid.
func (w *Workspace) DeleteHistoryEntry(id string) {
	next := make([]PlateHistoryEntry, 0, len(w.PlateHistory))
	for _, entry := range w.PlateHistory {
		if entry.ID == id {
			continue
		}
		next = append(next, entry)
	}
	if w.currentHistoryID == id {
		w.currentHistoryID = ""
	}
	w.PlateHistory = next
	w.persistPlateHistory()
}

func (w *Workspace) persistPlateHistory() {
	if w.Callbacks.SavePlateHistory != nil {
		if err := w.Callbacks.SavePlateHistory(w.PlateHistory); err != nil {
			w.Error = "保存孔板历史失败：" + err.Error()
		}
	}
}

func sanitizeExportFileName(title, ext string) string {
	name := strings.TrimSpace(title)
	if name == "" {
		name = "plate-layout"
	}
	replacer := strings.NewReplacer("/", "-", "\\", "-", ":", "-", "*", "-", "?", "", "\"", "", "<", "", ">", "", "|", "")
	name = replacer.Replace(name)
	return name + "." + ext
}

// ExportActivePlate renders the top designer plate to SVG or PNG and triggers SavePlateFile.
func (w *Workspace) ExportActivePlate(format string, dark bool) ([]byte, string, error) {
	props, title, ok := w.activePlatePropsForExport()
	if !ok {
		return nil, "", fmt.Errorf("未找到可导出的孔板")
	}
	return w.exportPlateProps(props, title, format, dark)
}

// ExportHistoryPlate renders a history entry directly to SVG or PNG.
func (w *Workspace) ExportHistoryPlate(id, format string, dark bool) ([]byte, string, error) {
	for _, entry := range w.PlateHistory {
		if entry.ID != id {
			continue
		}
		doc, err := asterdoc.Parse(entry.Source)
		if err != nil {
			return nil, "", fmt.Errorf("解析历史孔板失败：%w", err)
		}
		props, title, ok := docToPlateProps(doc, "group", "")
		if !ok {
			return nil, "", fmt.Errorf("历史记录不含有效孔板")
		}
		return w.exportPlateProps(props, title, format, dark)
	}
	return nil, "", fmt.Errorf("未找到指定历史记录")
}

func (w *Workspace) exportPlateProps(props plateui.Props, title, format string, dark bool) ([]byte, string, error) {
	ext := strings.ToLower(strings.TrimPrefix(strings.TrimSpace(format), "."))
	var data []byte
	switch ext {
	case "svg":
		data = []byte(plateui.RenderSVG(props, title, dark))
	case "png":
		raw, err := plateui.RenderPNG(props, title, dark)
		if err != nil {
			w.Error, w.Status = err.Error(), ""
			return nil, "", err
		}
		data = raw
	default:
		err := fmt.Errorf("不支持的导出格式 %q", format)
		w.Error, w.Status = err.Error(), ""
		return nil, "", err
	}
	suggestedName := sanitizeExportFileName(title, ext)
	savedPath := suggestedName
	if w.Callbacks.SavePlateFile != nil {
		p, err := w.Callbacks.SavePlateFile(suggestedName, ext, data)
		if err != nil {
			w.Error, w.Status = "导出失败："+err.Error(), ""
			return nil, "", err
		}
		if p == "" {
			// User canceled save dialog cleanly.
			return data, "", nil
		}
		savedPath = p
	}
	w.Error, w.Status = "", fmt.Sprintf("已导出 %s：%s", strings.ToUpper(ext), savedPath)
	return data, savedPath, nil
}

func formatBadgeText(format int, rows, cols int, topology string) string {
	switch strings.TrimSpace(topology) {
	case "dish":
		return fmt.Sprintf("%d区培养皿", rows*cols)
	case "rack_1d":
		return fmt.Sprintf("%d联管", rows*cols)
	case "cohort":
		return fmt.Sprintf("%d×%d 队列", rows, cols)
	default:
		if format > 0 {
			return fmt.Sprintf("%d孔板", format)
		}
		return fmt.Sprintf("%d×%d", rows, cols)
	}
}

func miniZoomForGrid(rows, cols int, topology string) float32 {
	if topology == "rack_1d" {
		return 0.54
	}
	switch {
	case cols <= 2:
		return 0.54
	case cols <= 4:
		return 0.56
	case cols <= 6:
		return 0.54
	case cols <= 8:
		return 0.52
	case cols <= 12:
		return 0.55
	default:
		return 0.48
	}
}

func plateView(c *ui.Context, w *Workspace, tok theme.Tokens) {
	if w.plateDoc == nil || w.plateView == nil {
		w.initPlateStudio()
	}
	dark := c.Theme() != nil && c.Theme().Dark
	mobileLayout := isMobileToolsLayout(c)
	if w.plateView != nil {
		w.plateView.MobileCompact = mobileLayout
	}
	studioSheetOpen := ui.Local(c.Root(), mobilePlateSheetKey{w}, func() bool { return false })

	// Header row: title + quick actions (Save to History, Export SVG, Export PNG, Apply to Design)
	ui.Row(c).FillWidth().Wrap().AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
		ui.Column(c).Gap(float32(tok.Spacing4) * 0.5).Children(func() {
			ui.Text(c, "孔板速排").FontSize(float32(tok.TextSection)).Bold().TextColor(tok.Text)
			ui.Text(c, "先选择容器规格，再在画布框选孔位标注分组、样本与复孔；支持直接导出矢量 SVG 或高清 PNG。").
				FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
		})
		if !mobileLayout {
			ui.Spacer(c)
		}
		ui.Row(c).Wrap().AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
			saveHistBtn := components.Button(c, "保存至历史", components.ButtonGhost)
			saveHistBtn.Disabled(w.activePlateAssignedCount() == 0)
			if saveHistBtn.Clicked() {
				w.SaveCurrentPlateToHistory()
			}
			if components.Button(c, "导出 SVG", components.ButtonSecondary).Clicked() {
				_, _, _ = w.ExportActivePlate("svg", dark)
			}
			if components.Button(c, "导出 PNG", components.ButtonSecondary).Clicked() {
				_, _, _ = w.ExportActivePlate("png", dark)
			}
			if components.Button(c, "应用到当前设计", components.ButtonPrimary).Clicked() {
				w.handoff()
			}
		})
	})

	// Step ①: Select plate format before designing
	ui.Column(c).FillWidth().Padding(float32(tok.Spacing12)).Radius(float32(tok.PanelRadius)).
		Background(tok.Elevated).Border(1, tok.Border).Gap(float32(tok.Spacing8)).Children(func() {
		ui.Row(c).FillWidth().Wrap().AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
			components.Badge(c, "① 选择孔板", components.BadgeOptions{Tone: components.BadgeAccent})
			ui.Text(c, "切换容器或新建空白板时，当前已标注内容会自动存入下方历史").
				FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
			ui.Spacer(c)
			if components.Button(c, "新建空白板", components.ButtonGhost).Clicked() {
				w.ResetBlankPlate()
			}
		})
		renderPresetChips := func() {
			for _, preset := range StandardPlatePresets {
				preset := preset
				active := w.ActivePlatePreset == preset.Key
				variant := components.ButtonGhost
				if active {
					variant = components.ButtonSecondary
				}
				btn := components.Button(c, preset.Label, variant)
				if mobileLayout {
					btn.MinHeight(float32(tok.Spacing48))
				}
				if btn.Clicked() {
					if mobileLayout {
						mygo.Haptics.Selection()
					}
					w.SelectPlatePreset(preset.Key)
				}
			}
		}
		if mobileLayout {
			ui.ScrollHorizontal(c).FillWidth().Children(func() {
				ui.Row(c).AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(renderPresetChips)
			})
		} else {
			ui.Row(c).FillWidth().Wrap().AlignItems(ui.Center).Gap(float32(tok.Spacing4)).Children(renderPresetChips)
		}
	})

	// Step ②: Active shared asterdoc Plate Designer (or mobile compact card + fullscreen Zoomable Sheet)
	plateMod := primaryPlateModule(w.plateDoc)
	if plateMod != nil {
		if mobileLayout {
			if plate := findPrimaryPlate(w.plateDoc); plate != nil {
				w.plateView.RenderMobilePlateStudio(c, plate, "", studioSheetOpen)
			}
		} else {
			w.plateView.RenderModule(c, "plate", 1, plateMod.Data)
		}
	}

	// Lower Section: 2-Column (or 1-Column on mobile) 孔板历史 Component Wall
	ui.Column(c).FillWidth().Gap(float32(tok.Spacing8)).Padding(float32(tok.Spacing4), 0).Children(func() {
		ui.Row(c).FillWidth().Wrap().AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
			ui.Text(c, "孔板历史").FontSize(float32(tok.TextSection)).Bold().TextColor(tok.Text)
			components.Badge(c, fmt.Sprintf("%d 份记录", len(w.PlateHistory)))
			ui.Text(c, "点击任意历史缩略板即可载入上方继续编辑，或直接导出 SVG / PNG").
				FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
		})

		if len(w.PlateHistory) == 0 {
			ui.Box(c).FillWidth().Padding(float32(tok.Spacing16)).Radius(float32(tok.PanelRadius)).
				Background(tok.Elevated).Border(1, tok.Border).Children(func() {
				ui.Text(c, "尚无历史孔板记录：在上方完成标注后点击「保存至历史」，或切换新建下一块孔板时会自动归档至此。").
					FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
			})
			return
		}

		if mobileLayout {
			ui.Column(c).FillWidth().Gap(float32(tok.Spacing12)).Children(func() {
				for _, entry := range w.PlateHistory {
					renderPlateHistoryCard(c, w, tok, dark, entry, true)
				}
			})
		} else {
			// Two-column visual grid of reusable plate history cards
			for i := 0; i < len(w.PlateHistory); i += 2 {
				left := w.PlateHistory[i]
				hasRight := i+1 < len(w.PlateHistory)
				var right PlateHistoryEntry
				if hasRight {
					right = w.PlateHistory[i+1]
				}
				ui.Row(c.Key(fmt.Sprintf("plate-hist-row-%d", i/2))).FillWidth().Gap(float32(tok.Spacing12)).AlignItems(ui.Stretch).Children(func() {
					renderPlateHistoryCard(c, w, tok, dark, left, false)
					if hasRight {
						renderPlateHistoryCard(c, w, tok, dark, right, false)
					} else {
						ui.Box(c).Grow(1).Basis(0)
					}
				})
			}
		}
	})
}

func primaryPlateModule(doc *asterdoc.Document) *asterdoc.Module {
	if doc == nil {
		return nil
	}
	for i := range doc.Modules {
		if doc.Modules[i].Kind == "plate" {
			return &doc.Modules[i]
		}
	}
	return nil
}

func renderPlateHistoryCard(c *ui.Context, w *Workspace, tok theme.Tokens, dark bool, entry PlateHistoryEntry, mobileLayout bool) {
	isEditing := w.currentHistoryID == entry.ID
	borderCol := tok.Border
	if isEditing {
		borderCol = tok.Accent
	}
	card := ui.Column(c.Key("plate-hist:"+entry.ID)).MinWidth(0).Padding(float32(tok.Spacing12)).
		Radius(float32(tok.PanelRadius)).Background(tok.Elevated).Border(1, borderCol).Gap(float32(tok.Spacing8))
	if mobileLayout {
		card.FillWidth()
	} else {
		card.Grow(1).Basis(0)
	}
	card.Children(func() {
		ui.Row(c).FillWidth().Wrap().AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
			title := strings.TrimSpace(entry.Title)
			if title == "" {
				title = formatBadgeText(entry.Format, entry.Rows, entry.Cols, entry.Topology)
			}
			ui.Text(c, title).FontSize(float32(tok.TextBody)).Bold().TextColor(tok.Text).Grow(1).MinWidth(0)
			components.Badge(c, formatBadgeText(entry.Format, entry.Rows, entry.Cols, entry.Topology))
			if isEditing {
				components.Badge(c, "编辑中", components.BadgeOptions{Tone: components.BadgeAccent})
			}
		})

		meta := fmt.Sprintf("已标 %d 孔 · %d 组", entry.AssignedWells, entry.GroupCount)
		if entry.UpdatedAt != "" {
			meta += " · " + entry.UpdatedAt
		}
		ui.Text(c, meta).FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted).FontFeatures("tnum")

		if mobileLayout {
			if doc, err := asterdoc.Parse(entry.Source); err == nil {
				if props, _, ok := docToPlateProps(doc, "group", ""); ok {
					legend := plateui.ComputeLegend(props, dark)
					totalW := max(1, props.Rows*props.Cols)
					ui.Row(c).FillWidth().Height(float32(tok.Spacing12)).Radius(float32(tok.ControlRadius)).
						Clip().Background(tok.Surface).Border(1, tok.Border).Gap(0).Children(func() {
						sum := 0
						for _, item := range legend {
							if item.Count > 0 {
								sum += item.Count
								ui.Box(c).FillHeight().Grow(float32(item.Count)).Basis(0).Background(item.Color)
							}
						}
						if rem := totalW - sum; rem > 0 {
							ui.Box(c).FillHeight().Grow(float32(rem)).Basis(0).Background(tok.Canvas)
						}
					})
				}
			}
		} else {
			// Clickable miniature plate component preview (reusing plateui.View)
			preview := ui.Box(c).FillWidth().Grow(1).MinHeight(float32(tok.Spacing48)*3).Padding(float32(tok.Spacing8)).Radius(float32(tok.ControlRadius)).
				Background(tok.Surface).Border(1, tok.Border).AlignItems(ui.Center).Justify(ui.Center).Cursor(ui.CursorPointer).Label("载入历史孔板 " + entry.Title)
			preview.Children(func() {
				if doc, err := asterdoc.Parse(entry.Source); err == nil {
					if props, _, ok := docToPlateProps(doc, "group", ""); ok {
						props.Interactive = false
						props.Zoom = miniZoomForGrid(entry.Rows, entry.Cols, entry.Topology)
						plateui.View(c, props)
					}
				}
			})
			if preview.Clicked() {
				w.LoadHistoryEntry(entry.ID)
			}
		}

		// Card footer: Load to Top / Export SVG / Export PNG / Delete
		ui.Row(c).FillWidth().Wrap().AlignItems(ui.Center).Gap(float32(tok.Spacing4)).Children(func() {
			loadLabel := "载入编辑"
			if isEditing {
				loadLabel = "重新载入"
			}
			loadBtn := components.Button(c, loadLabel, components.ButtonSecondary)
			if mobileLayout {
				loadBtn.MinHeight(float32(tok.Spacing48))
			}
			if loadBtn.Clicked() {
				w.LoadHistoryEntry(entry.ID)
			}
			if components.Button(c, "导出 SVG", components.ButtonGhost).Clicked() {
				_, _, _ = w.ExportHistoryPlate(entry.ID, "svg", dark)
			}
			if components.Button(c, "导出 PNG", components.ButtonGhost).Clicked() {
				_, _, _ = w.ExportHistoryPlate(entry.ID, "png", dark)
			}
			ui.Spacer(c)
			if components.IconButton(c, lucide.Trash(), "删除历史记录 "+entry.Title, components.IconButtonCompact).Clicked() {
				w.DeleteHistoryEntry(entry.ID)
			}
		})
	})
}
