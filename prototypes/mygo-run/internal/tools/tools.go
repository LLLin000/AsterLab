package tools

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"asterlab-mygo-prototype/internal/assays"
	"asterlab-mygo-prototype/internal/blocks"
	"asterlab-mygo-prototype/internal/components"
	"asterlab-mygo-prototype/internal/theme"
	"github.com/egoist/mygo/ui"
)

type Tool string

const (
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
}

func New(cb Callbacks) *Workspace {
	return &Workspace{Active: ToolPlate, Plate: blocks.NewPlate(), Stock: "100", Final: "10", Volume: "100", Rows: []blocks.ParamRow{{}}, SelectedType: blocks.TypeParameterTable, Callbacks: cb}
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
	ui.Column(c).Fill().Padding(16).Gap(12).Children(func() {
		ui.Row(c).Gap(8).Children(func() {
			components.PageTitle(c, "Tools & Components")
		})
		ui.Row(c).Grow(1).Gap(12).Children(func() {
			ui.Column(c).Width(190).FillHeight().Padding(8).Gap(4).Background(tok.SidebarBackground).Children(func() {
				ui.Text(c, "WORKSPACE").FontSize(11).TextColor(t.TextMuted)
				for _, item := range []struct {
					tool  Tool
					label string
				}{{ToolPlate, "96-well plate"}, {ToolDilution, "Dilution calculator"}, {ToolParameters, "Parameter table"}, {ToolLibrary, "Component library"}, {ToolImport, "Structured import"}} {
					item := item
					if ui.Button(c, item.label).Clicked() {
						w.Active = item.tool
						w.Error, w.Status = "", ""
					}
				}
			})
			ui.Column(c).Grow(1).FillHeight().Padding(12).Gap(8).Background(t.Surface).Children(func() {
				switch w.Active {
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
					ui.Text(c, w.Error).TextColor(t.Danger)
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
