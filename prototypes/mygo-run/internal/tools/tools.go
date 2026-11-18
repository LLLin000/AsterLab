package tools

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"asterlab-mygo-prototype/internal/assays"
	"asterlab-mygo-prototype/internal/blocks"
	"asterlab-mygo-prototype/internal/theme"
	plateui "asterlab-mygo-prototype/internal/uikit/plate"
	"asterlab-mygo-prototype/modules/runs"
	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/onevra/source/components"
	"github.com/oneo-me/onevra/source/icons/lucide"
)

type Tool string

const (
	ToolPantry     Tool = "pantry"
	ToolPlate      Tool = "plate"
	ToolDilution   Tool = "dilution"
	ToolParameters Tool = "parameters"
	ToolLibrary    Tool = "library"
	ToolImport     Tool = "import"
)

type Draft struct {
	Block *blocks.Instance
	Assay *assays.Design
}

type Callbacks struct {
	UseInDesigner func(blocks.Instance)
	CreateDraft   func(Draft)
	ListPantry    func() (runs.PantryLedger, error)
	UpsertPantry  func(runs.PantryUpsert) (runs.PantryItem, error)
	DeletePantry  func(runs.PantryDelete) error
	ExportPantry  func() (runs.PantryLabels, error)
}

// Workspace holds editable tool drafts until an explicit handoff action.
type Workspace struct {
	Active               Tool
	Plate                blocks.Plate
	PlateFormat          int
	PlateTopology        string
	plateUIState         plateui.InteractionState
	Group                string
	Stock, Final, Volume string
	Rows                 []blocks.ParamRow
	SelectedType         blocks.BlockType
	ImportJSON           string
	Error                string
	Status               string
	// Retry re-runs the last failed non-pantry action (import validation).
	// UX-P5: every failure offers cause + retry. Pantry keeps PantryRetry.
	Retry                func()
	NextDraftID          uint64
	Callbacks            Callbacks

	Pantry                      runs.PantryLedger
	PantryID                    string
	PantryName                  string
	PantryLocation              string
	PantryPurchasedOn           string
	PantryOpenedOn              string
	PantryShelfLifeDays         string
	PantryOpenedShelfLifeDays   *uint32
	PantryExhausted             bool
	PantryLoaded                bool
	PantryRetry                 func()
	PantryExportJSON            string
}

func New(cb Callbacks) *Workspace {
	w := &Workspace{Active: ToolPantry, Plate: blocks.NewPlate(), Stock: "100", Final: "10", Volume: "100", Rows: []blocks.ParamRow{{}}, SelectedType: blocks.TypeParameterTable, Callbacks: cb}
	w.ReloadPantry()
	return w
}

func (w *Workspace) ReloadPantry() {
	if w.Callbacks.ListPantry == nil {
		return
	}
	ledger, err := w.Callbacks.ListPantry()
	if err != nil {
		w.Error, w.Status = err.Error(), ""
		w.PantryRetry = w.ReloadPantry
		return
	}
	w.Pantry, w.PantryLoaded = ledger, true
	w.Error, w.Status, w.PantryRetry = "", "", nil
}

func (w *Workspace) SavePantry() {
	w.PantryRetry = nil
	if w.Callbacks.UpsertPantry == nil {
		return
	}
	name := strings.TrimSpace(w.PantryName)
	if name == "" {
		w.Error, w.Status = "请输入名称", ""
		return
	}
	var days *uint32
	if value := strings.TrimSpace(w.PantryShelfLifeDays); value != "" {
		n, err := strconv.ParseUint(value, 10, 32)
		if err != nil || n == 0 {
			w.Error, w.Status = "保质期请输入正整数天数", ""
			return
		}
		d := uint32(n)
		days = &d
	}
	input := runs.PantryUpsert{
		ID: w.PantryID, Name: name, Location: strings.TrimSpace(w.PantryLocation),
		PurchasedOn: strings.TrimSpace(w.PantryPurchasedOn), OpenedOn: strings.TrimSpace(w.PantryOpenedOn),
		ShelfLifeDays: days, OpenedShelfLifeDays: w.PantryOpenedShelfLifeDays,
		Exhausted: w.PantryExhausted,
	}
	if w.PantryID != "" {
		for _, item := range w.Pantry.Items {
			if item.ID == w.PantryID {
				revision := item.Revision
				input.ExpectedRevision = &revision
				break
			}
		}
	}
	if _, err := w.Callbacks.UpsertPantry(input); err != nil {
		w.Error, w.Status = err.Error(), ""
		w.PantryRetry = w.SavePantry
		return
	}
	w.clearPantryForm()
	w.ReloadPantry()
	if w.Error == "" {
		w.Status = "已保存"
	}
}

func (w *Workspace) EditPantry(item runs.PantryItem) {
	w.PantryID, w.PantryName, w.PantryLocation = item.ID, item.Name, item.Location
	w.PantryPurchasedOn, w.PantryOpenedOn, w.PantryExhausted = item.PurchasedOn, item.OpenedOn, item.Exhausted
	w.PantryOpenedShelfLifeDays = item.OpenedShelfLifeDays
	w.PantryShelfLifeDays = ""
	if item.ShelfLifeDays != nil {
		w.PantryShelfLifeDays = strconv.FormatUint(uint64(*item.ShelfLifeDays), 10)
	}
	w.Error, w.Status, w.PantryRetry = "", "", nil
}

func (w *Workspace) DeletePantry(item runs.PantryItem) {
	w.PantryRetry = nil
	if w.Callbacks.DeletePantry == nil {
		return
	}
	if err := w.Callbacks.DeletePantry(runs.PantryDelete{ID: item.ID, ExpectedRevision: &item.Revision}); err != nil {
		w.Error, w.Status = err.Error(), ""
		w.PantryRetry = func() { w.DeletePantry(item) }
		return
	}
	if w.PantryID == item.ID {
		w.clearPantryForm()
	}
	w.ReloadPantry()
}

func (w *Workspace) clearPantryForm() {
	w.PantryID, w.PantryName, w.PantryLocation = "", "", ""
	w.PantryPurchasedOn, w.PantryOpenedOn, w.PantryShelfLifeDays = "", "", ""
	w.PantryOpenedShelfLifeDays = nil
	w.PantryExhausted = false

}
func (w *Workspace) ExportPantryLabels() {
	w.PantryRetry, w.PantryExportJSON = nil, ""
	w.Error, w.Status = "", ""
	if w.Callbacks.ExportPantry == nil {
		return
	}
	labels, err := w.Callbacks.ExportPantry()
	if err != nil {
		w.Error, w.Status = err.Error(), ""
		w.PantryRetry = w.ExportPantryLabels
		return
	}
	data, err := json.MarshalIndent(labels, "", "  ")
	if err != nil {
		w.Error, w.Status = err.Error(), ""
		w.PantryRetry = w.ExportPantryLabels
		return
	}
	w.PantryExportJSON = string(data)
	w.Error, w.Status, w.PantryRetry = "", fmt.Sprintf("贴标清单已就绪 · %d 项", len(labels.Labels)), nil
}

func blockInstance(kind blocks.BlockType, props any) (blocks.Instance, error) {
	return blocks.NewInstance("tool-draft", blocks.Definition{ID: "tool." + string(kind), Version: 1, Type: kind, Props: props})
}

func (w *Workspace) plateDraft() (blocks.Instance, error) {
	return blockInstance(blocks.TypePlate96, w.Plate.Props)
}
func (w *Workspace) dilutionDraft() (blocks.Instance, error) {
	stock, e1 := strconv.ParseFloat(strings.TrimSpace(w.Stock), 64)
	final, e2 := strconv.ParseFloat(strings.TrimSpace(w.Final), 64)
	volume, e3 := strconv.ParseFloat(strings.TrimSpace(w.Volume), 64)
	if e1 != nil || e2 != nil || e3 != nil {
		return blocks.Instance{}, fmt.Errorf("enter numeric concentrations and volume")
	}
	p := blocks.DilutionPropsFromInput(blocks.DilutionInput{StockConc: stock, FinalConc: final, FinalVol: volume})
	if !p.Valid {
		return blocks.Instance{}, fmt.Errorf("%s", p.Error)
	}
	return blockInstance(blocks.TypeDilutionCalculator, p)
}
func (w *Workspace) parameterDraft() (blocks.Instance, error) {
	rows := make([]blocks.ParamRow, 0, len(w.Rows))
	for _, r := range w.Rows {
		if strings.TrimSpace(r.Key) != "" {
			rows = append(rows, r)
		}
	}
	if len(rows) == 0 {
		return blocks.Instance{}, fmt.Errorf("add at least one named parameter")
	}
	return blockInstance(blocks.TypeParameterTable, blocks.ParameterTableProps{Rows: rows})
}

func (w *Workspace) handoff() {
	var inst blocks.Instance
	var err error
	switch w.Active {
	case ToolPlate:
		inst, err = w.plateDraft()
	case ToolDilution:
		inst, err = w.dilutionDraft()
	case ToolParameters:
		inst, err = w.parameterDraft()
	case ToolLibrary:
		inst, err = w.template(w.SelectedType)
	default:
		w.Error = "请先选择一个草稿再导入" // UX-P5: §3 state vocabulary (was English).
		return
	}
	if err != nil {
		w.Error = err.Error()
		return
	}
	w.NextDraftID++
	inst.ID = fmt.Sprintf("tool-draft-%d", w.NextDraftID)
	w.Error, w.Status = "", "草稿已就绪" // UX-P5: §3 state vocabulary (was English).
	if w.Callbacks.UseInDesigner != nil {
		w.Callbacks.UseInDesigner(inst)
	}
	if w.Callbacks.CreateDraft != nil {
		w.Callbacks.CreateDraft(Draft{Block: &inst})
	}
}

func (w *Workspace) template(kind blocks.BlockType) (blocks.Instance, error) {
	var props any
	switch kind {
	case blocks.TypeNote:
		props = blocks.NoteProps{}
	case blocks.TypeParameterTable:
		props = blocks.ParameterTableProps{Rows: []blocks.ParamRow{}}
	case blocks.TypeMeasurementTable:
		props = blocks.MeasurementTableProps{Columns: []string{}, Rows: []blocks.MeasurementRow{}}
	case blocks.TypePlate96:
		props = blocks.NewPlate().Props
	case blocks.TypeDilutionCalculator:
		props = blocks.DilutionProps{Input: blocks.DilutionInput{StockConc: 1, FinalConc: 1, FinalVol: 1}, Result: blocks.DilutionResult{StockVol: 1}, Valid: true}
	case blocks.TypeChecklist:
		props = blocks.ChecklistProps{Items: []blocks.ChecklistItem{}}
	case blocks.TypeNestedWorkflow:
		props = blocks.NestedWorkflowProps{}
	default:
		return blocks.Instance{}, fmt.Errorf("unsupported component type %q", kind)
	}
	return blockInstance(kind, props)
}

func (w *Workspace) Import() error {
	trim := strings.TrimSpace(w.ImportJSON)
	if trim == "" {
		return fmt.Errorf("请粘贴 Typed block 实例或 assay 设计 JSON") // UX-P5: was English.
	}
	var inst blocks.Instance
	if err := json.Unmarshal([]byte(trim), &inst); err == nil && inst.ID != "" {
		if err := inst.Validate(); err != nil {
			return fmt.Errorf("Typed block 实例无效：%w", err) // UX-P5: cause in Chinese; detail from blocks.
		}
		if w.Callbacks.UseInDesigner != nil {
			w.Callbacks.UseInDesigner(inst)
		}
		if w.Callbacks.CreateDraft != nil {
			w.Callbacks.CreateDraft(Draft{Block: &inst})
		}
	w.Error, w.Status = "", "导入成功" // UX-P5: §3 state vocabulary (was English).
		return nil
	}
	var design assays.Design
	if err := json.Unmarshal([]byte(trim), &design); err != nil {
		return fmt.Errorf("JSON 无效：%w", err) // UX-P5: was English.
	}
	if err := assays.ValidateWellAssignments(design.Wells); err != nil {
		return err
	}
	if len(design.Wells) == 0 {
		return fmt.Errorf("assay 设计至少需要一个孔位分配") // UX-P5: was English.
	}
	if w.Callbacks.CreateDraft != nil {
		w.Callbacks.CreateDraft(Draft{Assay: &design})
	}
	w.Error, w.Status = "", "导入成功" // UX-P5: §3 state vocabulary (was English).
	return nil
}

func View(c *ui.Context, w *Workspace) {
	if w == nil {
		return
	}
	t := c.Theme()
	tok := theme.GetTokens(t)
	ui.Column(c).Fill().Background(tok.Canvas).Padding(float32(tok.Spacing16)).Gap(float32(tok.Spacing12)).Children(func() {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
			ui.Text(c, "资料与工具").FontSize(float32(tok.TextPage)).Bold().TextColor(tok.Text)
			ui.Spacer(c)
			components.Badge(c, "储藏室台账与实验通用工具")
		})
		// UX-P1/UX-LAYOUT: bounded scrollable main pane so 900x600 stays reachable.
		ui.Row(c).Grow(1).MinHeight(0).Gap(float32(tok.Spacing12)).AlignItems(ui.Stretch).Children(func() {
			ui.Column(c).Width(212).FillHeight().Padding(float32(tok.Spacing8)).Radius(float32(tok.PanelRadius)).
				Background(tok.Surface).Border(1, tok.Border).Gap(float32(tok.Spacing4)).Children(func() {
				components.Sidebar(c).Children(func() {
					components.SidebarSection(c, "工具导航", func() {
						navItems := []struct {
							tool  Tool
							label string
							icon  *ui.SVG
						}{
							{ToolPantry, "储藏室台账", lucide.FlaskConical()},
							{ToolPlate, "孔板与容器速排", lucide.Grid2x2()},
							{ToolDilution, "稀释配液换算", lucide.Calculator()},
							{ToolParameters, "实验参数表", lucide.SlidersHorizontal()},
							{ToolLibrary, "通用模块模板", lucide.Blocks()},
							{ToolImport, "结构化数据导入", lucide.FileInput()},
						}
						for _, item := range navItems {
							item := item
							if components.SidebarItem(c, item.label, item.icon, w.Active == item.tool).Clicked() {
								w.Active = item.tool
								if w.PantryRetry == nil && w.Retry == nil {
									w.Error, w.Status = "", ""
								}
							}
						}
					})
				})
			})
			ui.ScrollBoth(c).Grow(1).Children(func() {
				ui.Column(c).FillWidth().Padding(float32(tok.Spacing16)).Radius(float32(tok.PanelRadius)).
					Gap(float32(tok.Spacing12)).Background(tok.Surface).Border(1, tok.Border).Children(func() {
					switch w.Active {
					case ToolPantry:
						pantryView(c, w, tok)
					case ToolPlate:
						plateView(c, w, tok)
					case ToolDilution:
						dilutionView(c, w, tok)
					case ToolParameters:
						parameterView(c, w, tok)
					case ToolLibrary:
						libraryView(c, w, tok)
					case ToolImport:
						importView(c, w, tok)
					}
					if w.Error != "" {
						ui.Row(c).FillWidth().Padding(float32(tok.Spacing8), float32(tok.Spacing12)).
							Radius(float32(tok.ControlRadius)).Background(tok.DangerBg).Border(1, tok.Danger).
							AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
							ui.Text(c, w.Error).FontSize(float32(tok.TextMeta)).TextColor(tok.DangerText).Grow(1).MinWidth(0)
							if w.PantryRetry != nil && components.Button(c, "重试", components.ButtonSecondary).Clicked() {
								w.PantryRetry()
							} else if w.Retry != nil && components.Button(c, "重试", components.ButtonSecondary).Clicked() {
								w.Retry()
							}
						})
					} else if w.Status != "" {
						ui.Row(c).FillWidth().Padding(float32(tok.Spacing8), float32(tok.Spacing12)).
							Radius(float32(tok.ControlRadius)).Background(tok.SuccessBg).Border(1, tok.Success.Alpha(.4)).
							AlignItems(ui.Center).Children(func() {
							ui.Text(c, w.Status).FontSize(float32(tok.TextMeta)).Bold().TextColor(tok.SuccessText)
						})
					}
				})
			})
		})
	})
}

func plateView(c *ui.Context, w *Workspace, tok theme.Tokens) {
	if w.PlateFormat == 0 {
		w.PlateFormat = 96
	}
	if w.PlateTopology == "" {
		w.PlateTopology = "plate"
	}
	rows, cols := 8, 12
	switch {
	case w.PlateTopology == "rack_1d":
		rows, cols = 1, 8
	case w.PlateFormat == 4:
		rows, cols = 2, 2
	case w.PlateFormat == 6:
		rows, cols = 2, 3
	case w.PlateFormat == 12:
		rows, cols = 3, 4
	case w.PlateFormat == 24:
		rows, cols = 4, 6
	case w.PlateFormat == 48:
		rows, cols = 6, 8
	}

	ui.Row(c).FillWidth().Wrap().AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
		ui.Text(c, "通用孔板与容器速排").FontSize(float32(tok.TextSection)).Bold().TextColor(tok.Text)
		ui.Spacer(c)
		presets := []struct {
			label, topo string
			format      int
		}{
			{"4区皿", "dish", 4},
			{"6孔", "plate", 6},
			{"12孔", "plate", 12},
			{"24孔", "plate", 24},
			{"96孔", "plate", 96},
			{"8联管", "rack_1d", 8},
		}
		for _, p := range presets {
			p := p
			active := w.PlateFormat == p.format && w.PlateTopology == p.topo
			v := components.ButtonGhost
			if active {
				v = components.ButtonSecondary
			}
			if components.Button(c, p.label, v).Clicked() {
				w.PlateFormat = p.format
				w.PlateTopology = p.topo
				w.Plate.Props.Selected = nil
			}
		}
	})

	wells := map[string]plateui.WellState{}
	groupIdxMap := map[string]int{}
	nextGIdx := 0
	for r := 0; r < rows; r++ {
		for col := 0; col < cols; col++ {
			id := fmt.Sprintf("%c%d", 'A'+r, col+1)
			idx := r*12 + col
			if idx < len(w.Plate.Props.Groups) && strings.TrimSpace(w.Plate.Props.Groups[idx]) != "" {
				g := strings.TrimSpace(w.Plate.Props.Groups[idx])
				if _, seen := groupIdxMap[g]; !seen {
					groupIdxMap[g] = nextGIdx
					nextGIdx++
				}
				wells[id] = plateui.WellState{Group: g, SampleID: g}
			}
		}
	}
	selectedIDs := make([]string, 0, len(w.Plate.Props.Selected))
	for _, idx := range w.Plate.Props.Selected {
		r, col := idx/12, idx%12
		if r >= 0 && r < rows && col >= 0 && col < cols {
			selectedIDs = append(selectedIDs, fmt.Sprintf("%c%d", 'A'+r, col+1))
		}
	}

	plateui.View(c, plateui.Props{
		Wells:       wells,
		Rows:        rows,
		Cols:        cols,
		Topology:    w.PlateTopology,
		Interactive: true,
		SelectedIDs: selectedIDs,
		GroupIndex:  groupIdxMap,
		State:       &w.plateUIState,
		OnSelect: func(ids []string) {
			var next []int
			for _, id := range ids {
				if len(id) >= 2 {
					r := int(id[0] - 'A')
					col, err := strconv.Atoi(id[1:])
					if err == nil && r >= 0 && r < 8 && col >= 1 && col <= 12 {
						next = append(next, r*12+(col-1))
					}
				}
			}
			w.Plate.Props.Selected = next
			w.Error, w.Status = "", ""
		},
	})

	applyGroupText := func(lbl string) {
		lbl = strings.TrimSpace(lbl)
		for _, well := range w.Plate.Props.Selected {
			_ = w.Plate.SetGroup(well, lbl)
		}
		if lbl != "" && len(w.Plate.Props.Selected) > 0 {
			w.Status = fmt.Sprintf("已将 %d 孔标注为 %s", len(w.Plate.Props.Selected), lbl)
			w.Plate.Props.Selected = nil
		}
	}

	ui.Row(c).FillWidth().Wrap().Padding(float32(tok.Spacing8), float32(tok.Spacing12)).
		Radius(float32(tok.PanelRadius)).Background(tok.Elevated).Border(1, tok.BorderStrong).
		AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
		if len(w.Plate.Props.Selected) > 0 {
			components.Badge(c, fmt.Sprintf("%d 孔", len(w.Plate.Props.Selected)), components.BadgeOptions{Tone: components.BadgeAccent})
		} else {
			components.Badge(c, fmt.Sprintf("%d 孔", rows*cols))
		}
		ui.Box(c).Grow(1).MinWidth(200).Children(func() {
			tb := components.TextBox(c, &w.Group, components.TextBoxOptions{}).
				Placeholder("框选孔位后输入分组、处理或样本名（如 VEGF / 对照组）")
			tb.OnTextKey(func(k ui.TextKey) bool {
				if k.Key == ui.KeyEnter && k.Mods == 0 && len(w.Plate.Props.Selected) > 0 && strings.TrimSpace(w.Group) != "" {
					applyGroupText(w.Group)
					return true
				}
				return false
			})
		})
		applyBtn := components.Button(c, "标注", components.ButtonSecondary)
		applyBtn.Disabled(len(w.Plate.Props.Selected) == 0 || strings.TrimSpace(w.Group) == "")
		if applyBtn.Clicked() {
			applyGroupText(w.Group)
		}
		clearBtn := components.Button(c, "清除", components.ButtonGhost)
		clearBtn.Disabled(len(w.Plate.Props.Selected) == 0)
		if clearBtn.Clicked() {
			applyGroupText("")
			w.Plate.Props.Selected = nil
		}
		if components.Button(c, "应用孔板到当前设计", components.ButtonPrimary).Clicked() {
			w.handoff()
		}
	})
}

func dilutionView(c *ui.Context, w *Workspace, tok theme.Tokens) {
	ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
		ui.Text(c, "稀释配液速算 (C₁V₁ = C₂V₂)").FontSize(float32(tok.TextSection)).Bold().TextColor(tok.Text)
		ui.Spacer(c)
		components.Badge(c, "浓度单位需保持一致")
	})
	ui.Row(c).FillWidth().Wrap().Gap(float32(tok.Spacing8)).Children(func() {
		ui.Column(c).Grow(1).MinWidth(140).Gap(float32(tok.Spacing4)).Children(func() {
			ui.Text(c, "母液浓度 (C₁)").FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
			components.TextBox(c, &w.Stock, components.TextBoxOptions{}).Placeholder("如 100")
		})
		ui.Column(c).Grow(1).MinWidth(140).Gap(float32(tok.Spacing4)).Children(func() {
			ui.Text(c, "目标终浓度 (C₂)").FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
			components.TextBox(c, &w.Final, components.TextBoxOptions{}).Placeholder("如 10")
		})
		ui.Column(c).Grow(1).MinWidth(140).Gap(float32(tok.Spacing4)).Children(func() {
			ui.Text(c, "配制终体积 (V₂)").FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
			components.TextBox(c, &w.Volume, components.TextBoxOptions{}).Placeholder("如 100")
		})
	})
	ui.Row(c).FillWidth().Padding(float32(tok.Spacing12)).Radius(float32(tok.ControlRadius)).
		Background(tok.Elevated).Border(1, tok.Border).AlignItems(ui.Center).Gap(float32(tok.Spacing12)).Children(func() {
		if inst, err := w.dilutionDraft(); err == nil {
			var p blocks.DilutionProps
			_ = inst.DecodeProps(&p)
			ui.Text(c, fmt.Sprintf("需吸取母液：%.4g · 补加稀释液：%.4g", p.Result.StockVol, p.Result.DiluentVol)).
				FontSize(float32(tok.TextBody)).Bold().TextColor(tok.Accent).FontFeatures("tnum")
		} else {
			ui.Text(c, err.Error()).FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
		}
		ui.Spacer(c)
		if components.Button(c, "插入计算器到当前设计", components.ButtonPrimary).Clicked() {
			w.handoff()
		}
	})
}

func parameterView(c *ui.Context, w *Workspace, tok theme.Tokens) {
	ui.Text(c, "实验参数表草稿").FontSize(float32(tok.TextSection)).Bold().TextColor(tok.Text)
	ui.Text(c, "编辑常用条件参数行后，可直接加入当前实验步骤。").FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
	for i := range w.Rows {
		r := &w.Rows[i]
		ui.Row(c).FillWidth().Gap(float32(tok.Spacing8)).Children(func() {
			ui.Box(c).Grow(2).MinWidth(0).Children(func() { components.TextBox(c, &r.Key, components.TextBoxOptions{}).Placeholder("参数名称（如孵育时间）") })
			ui.Box(c).Grow(1).MinWidth(0).Children(func() { components.TextBox(c, &r.Value, components.TextBoxOptions{}).Placeholder("数值") })
			ui.Box(c).Width(float32(tok.Spacing48)*2).Children(func() { components.TextBox(c, &r.Unit, components.TextBoxOptions{}).Placeholder("单位") })
		})
	}
	ui.Row(c).Gap(float32(tok.Spacing8)).Children(func() {
		if components.Button(c, "添加参数行", components.ButtonSecondary).Clicked() {
			w.Rows = append(w.Rows, blocks.ParamRow{})
		}
		ui.Spacer(c)
		if components.Button(c, "插入参数表到当前设计", components.ButtonPrimary).Clicked() {
			w.handoff()
		}
	})
}

func libraryView(c *ui.Context, w *Workspace, tok theme.Tokens) {
	ui.Text(c, "通用模块模板").FontSize(float32(tok.TextSection)).Bold().TextColor(tok.Text)
	ui.Text(c, "内置标准化模块结构，可直接插入到实验设计步骤中。").FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
	ui.Row(c).FillWidth().Wrap().Gap(float32(tok.Spacing4)).Children(func() {
		for _, kind := range blocks.AllTypes() {
			k := kind
			variant := components.ButtonGhost
			if w.SelectedType == k {
				variant = components.ButtonSecondary
			}
			if components.Button(c, blockTypeLabel(k), variant).Clicked() {
				w.SelectedType = k
			}
		}
	})
	ui.Column(c).FillWidth().Padding(float32(tok.Spacing12)).Radius(float32(tok.ControlRadius)).
		Background(tok.Elevated).Border(1, tok.Border).Gap(float32(tok.Spacing8)).Children(func() {
		components.Badge(c, "预览 · "+blockTypeLabel(w.SelectedType), components.BadgeOptions{Tone: components.BadgeAccent})
		if inst, err := w.template(w.SelectedType); err == nil {
			blocks.View(c, inst)
		} else {
			ui.Text(c, err.Error()).FontSize(float32(tok.TextMeta)).TextColor(tok.DangerText)
		}
	})
	ui.Row(c).FillWidth().Justify(ui.End).Children(func() {
		if components.Button(c, "添加模块到当前设计", components.ButtonPrimary).Clicked() {
			w.handoff()
		}
	})
}

func blockTypeLabel(k blocks.BlockType) string {
	switch k {
	case blocks.TypeNote:
		return "操作文本 / 注意"
	case blocks.TypeParameterTable:
		return "参数表"
	case blocks.TypeMeasurementTable:
		return "数据记录表"
	case blocks.TypePlate96:
		return "96 孔板"
	case blocks.TypePlate12:
		return "12 孔板"
	case blocks.TypeDilutionCalculator:
		return "稀释计算器"
	case blocks.TypeChecklist:
		return "检查清单"
	case blocks.TypeNestedWorkflow:
		return "嵌套子流程"
	default:
		return string(k)
	}
}

func importView(c *ui.Context, w *Workspace, tok theme.Tokens) {
	ui.Text(c, "结构化数据与排板导入").FontSize(float32(tok.TextSection)).Bold().TextColor(tok.Text)
	ui.Text(c, "粘贴标准结构化模块或孔板布局内容，通过格式核验后自动生成草稿或加入设计。").
		FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
	components.TextBox(c, &w.ImportJSON, components.TextBoxOptions{}).
		Placeholder(`{"id":"...","definitionId":"...","definitionVersion":1,"type":"parameter_table","props":{"rows":[]}}`)
	ui.Row(c).FillWidth().Justify(ui.End).Children(func() {
		if components.Button(c, "核验并导入草稿", components.ButtonPrimary).Clicked() {
			if err := w.Import(); err != nil {
				w.Error, w.Status = err.Error(), ""
				w.Retry = func() {
					if err := w.Import(); err != nil {
						w.Error, w.Status = err.Error(), ""
						return
					}
					w.Retry = nil
				}
			} else {
				w.Retry = nil
			}
		}
	})
}

func pantryView(c *ui.Context, w *Workspace, tok theme.Tokens) {
	ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
		ui.Text(c, "储藏室台账").FontSize(float32(tok.TextSection)).Bold().TextColor(tok.Text)
		ui.Spacer(c)
		if components.Button(c, "导出贴标清单", components.ButtonSecondary).Clicked() {
			w.ExportPantryLabels()
		}
	})
	ui.Text(c, "统一管理常用试剂、缓冲液与耗材的存放位置、开封日期与效期提醒。").
		FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
	ui.Column(c).FillWidth().Padding(float32(tok.Spacing12)).Radius(float32(tok.ControlRadius)).
		Background(tok.Elevated).Border(1, tok.Border).Gap(float32(tok.Spacing8)).Children(func() {
		ui.Row(c).FillWidth().Gap(float32(tok.Spacing8)).Children(func() {
			ui.Box(c).Grow(1).MinWidth(0).Children(func() { components.TextBox(c, &w.PantryName, components.TextBoxOptions{}).Placeholder("试剂 / 耗材名称") })
			ui.Box(c).Grow(1).MinWidth(0).Children(func() { components.TextBox(c, &w.PantryLocation, components.TextBoxOptions{}).Placeholder("存放位置（如 4°C 冷藏柜）") })
		})
		ui.Row(c).FillWidth().Gap(float32(tok.Spacing8)).Children(func() {
			ui.Box(c).Grow(1).MinWidth(0).Children(func() { components.TextBox(c, &w.PantryPurchasedOn, components.TextBoxOptions{}).Placeholder("购入日 YYYY-MM-DD") })
			ui.Box(c).Grow(1).MinWidth(0).Children(func() { components.TextBox(c, &w.PantryOpenedOn, components.TextBoxOptions{}).Placeholder("开封日 YYYY-MM-DD") })
			ui.Box(c).Width(float32(tok.Spacing48)*2.6).Children(func() { components.TextBox(c, &w.PantryShelfLifeDays, components.TextBoxOptions{}).Placeholder("保质期（天）") })
		})
		ui.Row(c).FillWidth().Wrap().Gap(float32(tok.Spacing8)).AlignItems(ui.Center).Children(func() {
			if components.Button(c, "状态："+pantryStatusText(w.PantryExhausted, w.PantryOpenedOn != "")+"（切换）", components.ButtonGhost).Clicked() {
				w.PantryExhausted = !w.PantryExhausted
			}
			ui.Spacer(c)
			if w.PantryID != "" && components.Button(c, "取消编辑", components.ButtonGhost).Clicked() {
				w.clearPantryForm()
			}
			if components.Button(c, "保存物品", components.ButtonPrimary).Clicked() {
				w.SavePantry()
			}
		})
	})
	if w.PantryLoaded {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
			ui.Text(c, fmt.Sprintf("库存清单 (%d)", len(w.Pantry.Items))).FontSize(float32(tok.TextBody)).Bold().TextColor(tok.Text).FontFeatures("tnum")
			ui.Spacer(c)
			if components.Button(c, "刷新", components.ButtonGhost).Clicked() {
				w.ReloadPantry()
			}
		})
		if len(w.Pantry.Items) == 0 {
			ui.Text(c, "台账为空，可在上方录入常备试剂或耗材。").FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
		}
		near, other := groupPantryItems(w.Pantry.Items)
		for _, group := range []struct {
			title string
			items []runs.PantryItem
			tone  components.BadgeTone
		}{{"临期提醒", near, components.BadgeWarning}, {"在库清单", other, components.BadgeNeutral}} {
			if len(group.items) == 0 {
				continue
			}
			ui.Row(c).AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
				ui.Text(c, group.title).FontSize(float32(tok.TextBody)).Bold().TextColor(tok.Text)
				components.Badge(c, fmt.Sprintf("%d 项", len(group.items)), components.BadgeOptions{Tone: group.tone})
			})
			for _, item := range group.items {
				item := item
				tint := tok.Text
				statusTone := components.BadgeNeutral
				switch item.Status {
				case "near_expiry":
					tint, statusTone = tok.WarningText, components.BadgeWarning
				case "expired", "exhausted":
					tint, statusTone = tok.DangerText, components.BadgeDanger
				case "opened":
					statusTone = components.BadgeAccent
				}
				ui.Row(c).FillWidth().Padding(float32(tok.Spacing8), float32(tok.Spacing12)).
					Radius(float32(tok.ControlRadius)).Background(tok.Elevated).Border(1, tok.Border).
					AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
					ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
						ui.Row(c).Wrap().AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
							ui.Text(c, item.Name).FontSize(float32(tok.TextBody)).Bold().TextColor(tint)
							if item.Location != "" {
								ui.Text(c, "· "+item.Location).FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
							}
							components.Badge(c, pantryItemStatus(item), components.BadgeOptions{Tone: statusTone})
						})
						details := "购入 " + pantryDate(item.PurchasedOn) + " · 开封 " + pantryDate(item.OpenedOn) + " · 保质期 " + pantryDays(item.ShelfLifeDays)
						if item.OpenedShelfLifeDays != nil {
							details += " · 开封后 " + pantryDays(item.OpenedShelfLifeDays)
						}
						if item.ExpiresOn != "" {
							details += " · 到期 " + item.ExpiresOn
						}
						ui.Text(c, details).FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
					})
					if components.Button(c, "编辑", components.ButtonGhost).Clicked() {
						w.EditPantry(item)
					}
					if components.Button(c, "删除", components.ButtonGhost).Clicked() {
						w.DeletePantry(item)
					}
				})
			}
		}
	}
	if w.PantryExportJSON != "" {
		ui.Text(c, "贴标清单明细").FontSize(float32(tok.TextMeta)).Bold().TextColor(tok.Text)
		components.TextBox(c, &w.PantryExportJSON, components.TextBoxOptions{})
	}
}

func pantryDate(value string) string {
	if value == "" {
		return "—"
	}
	return value
}

func pantryDays(days *uint32) string {
	if days == nil {
		return "—"
	}
	return strconv.FormatUint(uint64(*days), 10) + "天"
}

func groupPantryItems(items []runs.PantryItem) (near, other []runs.PantryItem) {
	for _, item := range items {
		if item.Status == "near_expiry" {
			near = append(near, item)
		} else {
			other = append(other, item)
		}
	}
	return near, other
}

func pantryStatusText(exhausted, opened bool) string {
	if exhausted {
		return "已用尽"
	}
	if opened {
		return "已开封"
	}
	return "未开封"
}

func pantryItemStatus(item runs.PantryItem) string {
	switch item.Status {
	case "near_expiry":
		return "临期"
	case "expired":
		return "已过期"
	case "exhausted":
		return "已用尽"
	case "opened":
		return "已开封"
	default:
		return "未开封"
	}
}
