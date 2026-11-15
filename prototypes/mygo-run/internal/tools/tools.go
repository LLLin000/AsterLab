package tools

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"asterlab-mygo-prototype/internal/assays"
	"asterlab-mygo-prototype/internal/blocks"
	"asterlab-mygo-prototype/internal/theme"
	"github.com/egoist/mygo/ui"
	"asterlab-mygo-prototype/modules/runs"
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
	Group                string
	Stock, Final, Volume string
	Rows                 []blocks.ParamRow
	SelectedType         blocks.BlockType
	ImportJSON           string
	Error                string
	Status               string
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
		w.Error = "Choose a draft to import"
		return
	}
	if err != nil {
		w.Error = err.Error()
		return
	}
	w.NextDraftID++
	inst.ID = fmt.Sprintf("tool-draft-%d", w.NextDraftID)
	w.Error, w.Status = "", "Draft ready"
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
		return fmt.Errorf("paste a block instance or assay design JSON")
	}
	var inst blocks.Instance
	if err := json.Unmarshal([]byte(trim), &inst); err == nil && inst.ID != "" {
		if err := inst.Validate(); err != nil {
			return fmt.Errorf("invalid block instance: %w", err)
		}
		if w.Callbacks.UseInDesigner != nil {
			w.Callbacks.UseInDesigner(inst)
		}
		if w.Callbacks.CreateDraft != nil {
			w.Callbacks.CreateDraft(Draft{Block: &inst})
		}
		w.Error, w.Status = "", "Import accepted"
		return nil
	}
	var design assays.Design
	if err := json.Unmarshal([]byte(trim), &design); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	if err := assays.ValidateWellAssignments(design.Wells); err != nil {
		return err
	}
	if len(design.Wells) == 0 {
		return fmt.Errorf("assay design needs at least one well assignment")
	}
	if w.Callbacks.CreateDraft != nil {
		w.Callbacks.CreateDraft(Draft{Assay: &design})
	}
	w.Error, w.Status = "", "Import accepted"
	return nil
}

func View(c *ui.Context, w *Workspace) {
	if w == nil {
		return
	}
	t := c.Theme()
	tok := theme.GetTokens(t)
	ui.Column(c).Fill().Padding(float32(tok.Spacing16)).Gap(float32(tok.Spacing12)).Children(func() {
		ui.Row(c).Gap(float32(tok.Spacing8)).Children(func() {
			ui.Text(c, "Tools & Components").FontSize(float32(tok.TextPage)).Bold().TextColor(t.Text)
		})
		ui.Row(c).Grow(1).Gap(float32(tok.Spacing12)).Children(func() {
			ui.Column(c).Width(190).FillHeight().Padding(float32(tok.Spacing8)).Gap(float32(tok.Spacing4)).Background(tok.SidebarBackground).Children(func() {
				ui.Text(c, "WORKSPACE").FontSize(float32(tok.TextMeta)).TextColor(t.TextMuted)
				for _, item := range []struct {
					tool  Tool
					label string
				}{{ToolPantry, "储藏室"}, {ToolPlate, "96-well plate"}, {ToolDilution, "Dilution calculator"}, {ToolParameters, "Parameter table"}, {ToolLibrary, "Component library"}, {ToolImport, "Structured import"}} {
					item := item
					if ui.Button(c, item.label).Clicked() {
						w.Active = item.tool
						if w.PantryRetry == nil {
							w.Error, w.Status = "", ""
						}
					}
				}
			})
			ui.Column(c).Grow(1).FillHeight().Padding(float32(tok.Spacing12)).Gap(float32(tok.Spacing8)).Background(t.Surface).Children(func() {
				switch w.Active {
				case ToolPantry:
					pantryView(c, w, t, tok)
				case ToolPlate:
					plateView(c, w, t)
				case ToolDilution:
					dilutionView(c, w, t)
				case ToolParameters:
					parameterView(c, w, t)
				case ToolLibrary:
					libraryView(c, w, t)
				case ToolImport:
					importView(c, w, t)
				}
				if w.Error != "" {
					ui.Row(c).Gap(float32(tok.Spacing8)).Children(func() {
						ui.Text(c, w.Error).TextColor(t.Danger)
						if w.PantryRetry != nil && ui.Button(c, "重试").Clicked() {
							w.PantryRetry()
						}
					})
				} else if w.Status != "" {
					ui.Text(c, w.Status).TextColor(t.Success)
				}
			})
		})
	})
}

func plateView(c *ui.Context, w *Workspace, t *ui.Theme) {
	ui.Text(c, "Plate layout").Bold()
	ui.Text(c, fmt.Sprintf("96 wells · %d selected", w.Plate.SelectedCount())).FontSize(12).TextColor(t.TextMuted)
	for row := range 8 {
		r := row
		ui.Row(c).Gap(3).Children(func() {
			ui.Text(c, string(rune('A'+r))).FontSize(12)
			for col := range 12 {
				idx := r*12 + col
				label := fmt.Sprintf("%d", col+1)
				if w.Plate.IsSelected(idx) {
					label = "●"
				}
				if ui.Button(c, label).Clicked() {
					if err := w.Plate.ToggleWell(idx); err != nil {
						w.Error = err.Error()
					} else {
						w.Error, w.Status = "", ""
					}
				}
			}
		})
	}
	ui.Text(c, "Group label for selected wells").FontSize(12)
	if ui.TextInput(c, &w.Group).Placeholder("e.g. Control").Changed() {
		for _, well := range w.Plate.Props.Selected {
			_ = w.Plate.SetGroup(well, w.Group)
		}
	}
	if ui.Button(c, "Use plate in Designer / create draft").Clicked() {
		w.handoff()
	}
}
func dilutionView(c *ui.Context, w *Workspace, t *ui.Theme) {
	ui.Text(c, "Dilution calculator").Bold()
	ui.Text(c, "C₁V₁ = C₂V₂ · use consistent units").FontSize(12).TextColor(t.TextMuted)
	ui.Text(c, "Stock concentration")
	ui.TextInput(c, &w.Stock).Placeholder("e.g. 100")
	ui.Text(c, "Target concentration")
	ui.TextInput(c, &w.Final).Placeholder("e.g. 10")
	ui.Text(c, "Final volume")
	ui.TextInput(c, &w.Volume).Placeholder("e.g. 100")
	if inst, err := w.dilutionDraft(); err == nil {
		var p blocks.DilutionProps
		_ = inst.DecodeProps(&p)
		ui.Text(c, fmt.Sprintf("Stock: %.4g · Diluent: %.4g", p.Result.StockVol, p.Result.DiluentVol)).Bold()
	} else {
		ui.Text(c, err.Error()).TextColor(t.TextMuted)
	}
	if ui.Button(c, "Use calculation in Designer / create draft").Clicked() {
		w.handoff()
	}
}
func parameterView(c *ui.Context, w *Workspace, t *ui.Theme) {
	ui.Text(c, "Parameter table").Bold()
	ui.Text(c, "Edit the rows, then hand off as a typed component.").FontSize(12).TextColor(t.TextMuted)
	for i := range w.Rows {
		r := &w.Rows[i]
		ui.Row(c).Gap(6).Children(func() {
			ui.TextInput(c, &r.Key).Placeholder("Parameter")
			ui.TextInput(c, &r.Value).Placeholder("Value")
			ui.TextInput(c, &r.Unit).Placeholder("Unit")
		})
	}
	ui.Row(c).Gap(8).Children(func() {
		if ui.Button(c, "Add row").Clicked() {
			w.Rows = append(w.Rows, blocks.ParamRow{})
		}
		if ui.Button(c, "Use table in Designer / create draft").Clicked() {
			w.handoff()
		}
	})
}
func libraryView(c *ui.Context, w *Workspace, t *ui.Theme) {
	ui.Text(c, "Component library").Bold()
	ui.Text(c, "Built-in typed block definitions").FontSize(12).TextColor(t.TextMuted)
	for _, kind := range blocks.AllTypes() {
		k := kind
		if ui.Button(c, string(k)).Clicked() {
			w.SelectedType = k
		}
	}
	ui.Text(c, "Selected: "+string(w.SelectedType)).Bold()
	if inst, err := w.template(w.SelectedType); err == nil {
		blocks.View(c, inst)
	} else {
		ui.Text(c, err.Error()).TextColor(t.Danger)
	}
	if ui.Button(c, "Add selected component to Designer / create draft").Clicked() {
		w.handoff()
	}
}
func importView(c *ui.Context, w *Workspace, t *ui.Theme) {
	ui.Text(c, "Structured import").Bold()
	ui.Text(c, "Paste a typed block instance or assays.Design JSON. Imported values are validated before handoff.").FontSize(12).TextColor(t.TextMuted)
	ui.TextInput(c, &w.ImportJSON).Placeholder(`{"id":"...","definitionId":"...","definitionVersion":1,"type":"parameter_table","props":{"rows":[]}}`)
	if ui.Button(c, "Validate and create draft").Clicked() {
		if err := w.Import(); err != nil {
			w.Error, w.Status = err.Error(), ""
		}
	}
}
func pantryView(c *ui.Context, w *Workspace, t *ui.Theme, tok theme.Tokens) {
	ui.Row(c).Gap(float32(tok.Spacing8)).Children(func() {
		ui.Text(c, "储藏室台账").FontSize(float32(tok.TextSection)).Bold().TextColor(t.Text)
		ui.Spacer(c)
		if ui.Button(c, "导出贴标清单").Clicked() {
			w.ExportPantryLabels()
		}
	})
	ui.Text(c, "名称 · 位置 · 购入/开封日 · 保质期").FontSize(float32(tok.TextMeta)).TextColor(t.TextMuted)
	ui.Row(c).Gap(float32(tok.Spacing4)).Children(func() {
		ui.TextInput(c, &w.PantryName).Placeholder("名称")
		ui.TextInput(c, &w.PantryLocation).Placeholder("位置")
	})
	ui.Row(c).Gap(float32(tok.Spacing4)).Children(func() {
		ui.TextInput(c, &w.PantryPurchasedOn).Placeholder("购入日 YYYY-MM-DD")
		ui.TextInput(c, &w.PantryOpenedOn).Placeholder("开封日 YYYY-MM-DD")
		ui.TextInput(c, &w.PantryShelfLifeDays).Placeholder("保质期（天）")
	})
	ui.Row(c).Gap(float32(tok.Spacing8)).Children(func() {
		if ui.Button(c, "状态："+pantryStatusText(w.PantryExhausted, w.PantryOpenedOn != "")+"（点击切换）").Clicked() {
			w.PantryExhausted = !w.PantryExhausted
		}
		if ui.Button(c, "保存物品").Clicked() {
			w.SavePantry()
		}
		if w.PantryID != "" && ui.Button(c, "取消编辑").Clicked() {
			w.clearPantryForm()
		}
	})
	if w.PantryLoaded {
		ui.Row(c).Gap(float32(tok.Spacing8)).Children(func() {
			ui.Text(c, "物品").Bold()
			ui.Spacer(c)
			if ui.Button(c, "刷新").Clicked() {
				w.ReloadPantry()
			}
		})
		if len(w.Pantry.Items) == 0 {
			ui.Text(c, "台账为空").TextColor(t.TextMuted)
		}
		near, other := groupPantryItems(w.Pantry.Items)
		for _, group := range []struct {
			title string
			items []runs.PantryItem
		}{{"临期", near}, {"其他", other}} {
			if len(group.items) == 0 {
				continue
			}
			ui.Text(c, group.title).Bold()
			for _, item := range group.items {
				item := item
				tint := t.Text
				if item.Status == "near_expiry" {
					tint = tok.Warning
				} else if item.Status == "expired" || item.Status == "exhausted" {
					tint = tok.Danger
				}
				ui.Row(c).Gap(float32(tok.Spacing8)).Children(func() {
					ui.Column(c).Grow(1).Children(func() {
						ui.Text(c, item.Name+" · "+item.Location+" · "+pantryItemStatus(item)).TextColor(tint)
						details := "购入 "+pantryDate(item.PurchasedOn)+" · 开封 "+pantryDate(item.OpenedOn)+" · 保质期 "+pantryDays(item.ShelfLifeDays)
						if item.OpenedShelfLifeDays != nil {
							details += " · 开封后 "+pantryDays(item.OpenedShelfLifeDays)
						}
						if item.ExpiresOn != "" {
							details += " · 到期 "+item.ExpiresOn
						}
						ui.Text(c, details).FontSize(float32(tok.TextMeta)).TextColor(t.TextMuted)
					})
					if ui.Button(c, "编辑").Clicked() {
						w.EditPantry(item)
					}
					if ui.Button(c, "删除").Clicked() {
						w.DeletePantry(item)
					}
				})
			}
		}
	}
	if w.PantryExportJSON != "" {
		ui.Text(c, "插件清单 JSON").FontSize(float32(tok.TextMeta)).Bold()
		ui.TextInput(c, &w.PantryExportJSON)
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
