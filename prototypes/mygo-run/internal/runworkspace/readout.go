package runworkspace

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/onevra/source/components"

	"asterlab-mygo-prototype/internal/theme"
	"asterlab-mygo-prototype/modules/runs"
)

// Typed readout kinds. The strings match Core's readout-schema v1 value_kind
// names (docs/contracts/readout-schema-v1.md); numeric entry stays in the
// result grid, so the composer only offers the two non-numeric cell kinds.
const (
	typedReadoutCategorical = "categorical"
	typedReadoutTable       = "table"
)

// typedReadoutLabel is the operator wording for a readout kind (UX-P2).
func typedReadoutLabel(kind string) string {
	switch kind {
	case typedReadoutCategorical:
		return "分类"
	case typedReadoutTable:
		return "表格"
	default:
		return kind
	}
}

// readoutValueJSON is one v1 Readout value: categorical carries a category,
// table carries columns plus rows. Numeric/file/plate kinds are not offered
// here (numeric lives in the grid; file/plate link through assets).
type readoutValueJSON struct {
	Kind     string   `json:"kind"`
	Category *string  `json:"category,omitempty"`
	Columns  []string `json:"columns,omitempty"`
	Rows     [][]any  `json:"rows,omitempty"`
}

// readoutSourceJSON marks the entry as a raw observation. Derived readouts
// must name a method and sources; the composer never files those.
type readoutSourceJSON struct {
	Kind        string   `json:"kind"`
	DerivedFrom []string `json:"derived_from"`
}

// readoutLinkJSON attaches the optional sample/timepoint link. It is omitted
// entirely when neither is set so Core applies the schema's own link policy.
type readoutLinkJSON struct {
	SampleID  *string `json:"sample_id,omitempty"`
	Timepoint *string `json:"timepoint,omitempty"`
}

// readoutJSON is one v1 Readout in a readout.submit call.
type readoutJSON struct {
	SchemaID      string            `json:"schema_id"`
	SchemaVersion int               `json:"schema_version"`
	Value         readoutValueJSON  `json:"value"`
	Source        readoutSourceJSON `json:"source"`
	Link          *readoutLinkJSON  `json:"link,omitempty"`
}

// readoutSubmitJSON is the exact readout.submit multi-value input Core
// validates (AGENT_API readout.submit): frozen step scope, one transaction,
// readouts[] in v1 shape, plus optional call-scoped schema definitions.
type readoutSubmitJSON struct {
	RunID             string            `json:"run_id"`
	StepPath          string            `json:"step_path"`
	ProtocolVersionID string            `json:"protocol_version_id"`
	AssayID           string            `json:"assay_id,omitempty"`
	Timepoint         string            `json:"timepoint,omitempty"`
	Readouts          []readoutJSON     `json:"readouts"`
	ReadoutSchemas    []json.RawMessage `json:"readout_schemas,omitempty"`
}

// readoutLink builds the optional link from operator input. Empty sample and
// empty scope timepoint mean no link; Core then applies the schema policy.
func readoutLink(sampleID, timepoint string) *readoutLinkJSON {
	sampleID = strings.TrimSpace(sampleID)
	timepoint = strings.TrimSpace(timepoint)
	if sampleID == "" && timepoint == "" {
		return nil
	}
	link := &readoutLinkJSON{}
	if sampleID != "" {
		link.SampleID = &sampleID
	}
	if timepoint != "" {
		link.Timepoint = &timepoint
	}
	return link
}

// checkSchemaDef passes a call-scoped schema definition through verbatim when
// the operator supplies one. Core validates the definition against the
// registry; the UI never edits or defaults it.
func checkSchemaDef(raw string) (json.RawMessage, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, nil
	}
	var v map[string]any
	if err := json.Unmarshal([]byte(trimmed), &v); err != nil {
		return nil, fmt.Errorf("schema 定义不是合法 JSON")
	}
	if len(v) == 0 {
		return nil, fmt.Errorf("schema 定义不能为空对象")
	}
	return json.RawMessage(trimmed), nil
}

// buildCategoricalCommand packs one categorical readout into the Bridge v2
// generic envelope for kind readout.submit. Validation here is only what the
// UI can check without Core (schema id, category, schema-def JSON shape);
// options, units and link policy stay Core's.
func buildCategoricalCommand(runID, stepPath, protocolVersionID, assayID, timepoint, schemaID, schemaDef, sampleID, category, idempotencyKey string) (runs.ExecuteCommandInput, error) {
	schemaID = strings.TrimSpace(schemaID)
	category = strings.TrimSpace(category)
	if schemaID == "" {
		return runs.ExecuteCommandInput{}, fmt.Errorf("请填写 schema ID")
	}
	if category == "" {
		return runs.ExecuteCommandInput{}, fmt.Errorf("请填写或选择分类结果")
	}
	def, err := checkSchemaDef(schemaDef)
	if err != nil {
		return runs.ExecuteCommandInput{}, err
	}
	req := readoutSubmitJSON{
		RunID:             runID,
		StepPath:          stepPath,
		ProtocolVersionID: protocolVersionID,
		AssayID:           assayID,
		Timepoint:         timepoint,
		Readouts: []readoutJSON{{
			SchemaID:      schemaID,
			SchemaVersion: 1,
			Value:         readoutValueJSON{Kind: typedReadoutCategorical, Category: &category},
			Source:        readoutSourceJSON{Kind: "raw", DerivedFrom: []string{}},
			Link:          readoutLink(sampleID, ""),
		}},
	}
	if len(def) > 0 {
		req.ReadoutSchemas = []json.RawMessage{def}
	}
	return packReadoutCommand(req, idempotencyKey)
}

// parseTableValue parses operator-typed table cells: columns are
// comma-separated, rows are separated by "|" (single-line input) or newlines,
// cells by commas. A cell that parses as a number stays a number so Core
// sees the same 1 vs "1" distinction as its own CSV path.
func parseTableValue(columnsText, rowsText string) ([]string, [][]any, error) {
	var columns []string
	for _, c := range strings.Split(columnsText, ",") {
		c = strings.TrimSpace(c)
		if c != "" {
			columns = append(columns, c)
		}
	}
	if len(columns) == 0 {
		return nil, nil, fmt.Errorf("请填写至少一列列名")
	}
	rowsText = strings.ReplaceAll(rowsText, "\n", "|")
	var rows [][]any
	for _, line := range strings.Split(rowsText, "|") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		cells := strings.Split(line, ",")
		if len(cells) != len(columns) {
			return nil, nil, fmt.Errorf("第 %d 行有 %d 个单元格，应与 %d 列一致", len(rows)+1, len(cells), len(columns))
		}
		row := make([]any, 0, len(cells))
		for _, cell := range cells {
			cell = strings.TrimSpace(cell)
			if f, err := strconv.ParseFloat(cell, 64); err == nil {
				row = append(row, f)
			} else {
				row = append(row, cell)
			}
		}
		rows = append(rows, row)
	}
	if len(rows) == 0 {
		return nil, nil, fmt.Errorf("请填写至少一行数据")
	}
	return columns, rows, nil
}

// buildTableCommand packs one table readout into the Bridge v2 generic
// envelope for kind readout.submit.
func buildTableCommand(runID, stepPath, protocolVersionID, assayID, timepoint, schemaID, schemaDef, sampleID string, columns []string, rows [][]any, idempotencyKey string) (runs.ExecuteCommandInput, error) {
	schemaID = strings.TrimSpace(schemaID)
	if schemaID == "" {
		return runs.ExecuteCommandInput{}, fmt.Errorf("请填写 schema ID")
	}
	if len(columns) == 0 || len(rows) == 0 {
		return runs.ExecuteCommandInput{}, fmt.Errorf("表格不能为空")
	}
	def, err := checkSchemaDef(schemaDef)
	if err != nil {
		return runs.ExecuteCommandInput{}, err
	}
	req := readoutSubmitJSON{
		RunID:             runID,
		StepPath:          stepPath,
		ProtocolVersionID: protocolVersionID,
		AssayID:           assayID,
		Timepoint:         timepoint,
		Readouts: []readoutJSON{{
			SchemaID:      schemaID,
			SchemaVersion: 1,
			Value:         readoutValueJSON{Kind: typedReadoutTable, Columns: columns, Rows: rows},
			Source:        readoutSourceJSON{Kind: "raw", DerivedFrom: []string{}},
			Link:          readoutLink(sampleID, ""),
		}},
	}
	if len(def) > 0 {
		req.ReadoutSchemas = []json.RawMessage{def}
	}
	return packReadoutCommand(req, idempotencyKey)
}

// packReadoutCommand seals a readout.submit payload into the generic
// envelope with one idempotency key per user action.
func packReadoutCommand(req readoutSubmitJSON, idempotencyKey string) (runs.ExecuteCommandInput, error) {
	payload, err := json.Marshal(req)
	if err != nil {
		return runs.ExecuteCommandInput{}, err
	}
	if strings.TrimSpace(idempotencyKey) == "" {
		idempotencyKey = newIdempotencyKey()
	}
	return runs.ExecuteCommandInput{
		Kind:           channelCommandReadout,
		IdempotencyKey: idempotencyKey,
		PayloadJSON:    string(payload),
	}, nil
}

// typedReadoutForm holds the categorical/table readout draft per task.
type typedReadoutForm struct {
	Kind     string
	SchemaID string
	Schema   string
	SampleID string
	Category string
	Options  string
	Columns  string
	Rows     string
	Busy     bool
	Err      string
	Done     string
}

func (s *State) typedReadoutForm(taskKey string) *typedReadoutForm {
	bench := s.bench()
	if bench.readoutForms == nil {
		bench.readoutForms = map[string]*typedReadoutForm{}
	}
	form, ok := bench.readoutForms[taskKey]
	if !ok {
		form = &typedReadoutForm{}
		bench.readoutForms[taskKey] = form
	}
	return form
}

// readoutOptions splits the operator's candidate-option shortcut list. The
// options are only chips; Core still validates the filed category.
func readoutOptions(text string) []string {
	var out []string
	for _, part := range strings.Split(text, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

// renderTypedReadoutSection draws the categorical/table readout entry: one
// kind, one schema, one sample, and the typed value, written through
// ExecuteCommand kind readout.submit. Core owns schema, unit and link
// validation (UX-P1, UX-P4, UX-STATE, UX-DESIGN).
func renderTypedReadoutSection(c *ui.Context, s *State, task *RunTaskViewModel, t *ui.Theme, tokens theme.Tokens) {
	if s.Callbacks.ExecuteCommand == nil || task == nil {
		return
	}
	form := s.typedReadoutForm(task.TaskKey)
	ui.Column(c).FillWidth().Gap(t.Space(1.5)).Children(func() {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(t.Space(1.5)).Children(func() {
			ui.Text(c, "分类与表格读数").FontSize(float32(tokens.TextSection)).FontWeight(600).TextColor(t.Text)
			ui.Text(c, "经 readout.submit 写入 Core，数值读数仍在结果网格录入").FontSize(t.Rem(.7)).TextColor(t.TextMuted)
		})
		ui.Row(c).FillWidth().Wrap().AlignItems(ui.Center).Gap(t.Space(1)).Children(func() {
			for _, kind := range []string{typedReadoutCategorical, typedReadoutTable} {
				kind := kind
				label := typedReadoutLabel(kind)
				if form.Kind == kind {
					label = "● " + label
				}
				if components.Button(c, label, components.ButtonSecondary).Clicked() {
					if form.Kind == kind {
						form.Kind = ""
					} else {
						form.Kind = kind
					}
				}
			}
		})
		if form.Kind == "" {
			return
		}
		ui.Column(c).FillWidth().Padding(t.Space(1.5), t.Space(2)).Radius(6).
			Background(tokens.Elevated).Border(1, tokens.Border).Gap(t.Space(1)).Children(func() {
			ui.Text(c, "schema ID（必填）").FontSize(t.Rem(.72)).TextColor(t.TextMuted)
			ui.TextInput(c, &form.SchemaID).FillWidth().Placeholder("例如：wb_grade（内置 schema 可直接使用）")
			ui.Text(c, "本次调用的 schema 定义（可选，仅自定义 schema 需要，粘贴 JSON 对象）").FontSize(t.Rem(.72)).TextColor(t.TextMuted)
			ui.TextInput(c, &form.Schema).FillWidth().Placeholder("留空表示使用 Core 内置定义")
			ui.Text(c, "样本 ID（可选，缺省按 schema 关联策略）").FontSize(t.Rem(.72)).TextColor(t.TextMuted)
			ui.TextInput(c, &form.SampleID).FillWidth().Placeholder("例如：M3-1")
			if form.Kind == typedReadoutCategorical {
				renderCategoryEntry(c, form, t)
			} else {
				renderTableEntry(c, form, t)
			}
			ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(t.Space(1)).Children(func() {
				if submit := components.Button(c, "提交读数", components.ButtonSecondary); submit.Clicked() && !form.Busy {
					s.submitTypedReadout(task, form)
				}
				if form.Busy {
					ui.Text(c, "提交中…").FontSize(t.Rem(.72)).TextColor(t.TextMuted)
				}
			})
		})
		if form.Done != "" {
			ui.Text(c, form.Done).FontSize(t.Rem(.74)).TextColor(tokens.SuccessText)
		}
		if form.Err != "" {
			ui.Text(c, "提交失败："+form.Err).FontSize(t.Rem(.74)).TextColor(tokens.DangerText)
		}
	})
}

// renderCategoryEntry draws the categorical value entry: an option-chip
// shortcut row plus free text. Chips never constrain; Core validates.
func renderCategoryEntry(c *ui.Context, form *typedReadoutForm, t *ui.Theme) {
	ui.Text(c, "候选项（可选，逗号分隔，仅作快捷选择）").FontSize(t.Rem(.72)).TextColor(t.TextMuted)
	ui.TextInput(c, &form.Options).FillWidth().Placeholder("例如：positive, negative")
	if options := readoutOptions(form.Options); len(options) > 0 {
		ui.Row(c).FillWidth().Wrap().AlignItems(ui.Center).Gap(t.Space(1)).Children(func() {
			for _, option := range options {
				option := option
				label := option
				if form.Category == option {
					label = "● " + option
				}
				if components.Button(c, label, components.ButtonSecondary).Clicked() {
					form.Category = option
				}
			}
		})
	}
	ui.Text(c, "分类结果（必填）").FontSize(t.Rem(.72)).TextColor(t.TextMuted)
	ui.TextInput(c, &form.Category).FillWidth().Placeholder("例如：positive")
}

// renderTableEntry draws the table value entry: comma-separated columns and
// rows separated by "|" (or newlines); numeric cells stay numeric.
func renderTableEntry(c *ui.Context, form *typedReadoutForm, t *ui.Theme) {
	ui.Text(c, "列（必填，逗号分隔）").FontSize(t.Rem(.72)).TextColor(t.TextMuted)
	ui.TextInput(c, &form.Columns).FillWidth().Placeholder("例如：lane, grade")
	ui.Text(c, "行（必填，行之间用 | 分隔，单元格用逗号分隔）").FontSize(t.Rem(.72)).TextColor(t.TextMuted)
	ui.TextInput(c, &form.Rows).FillWidth().Placeholder("例如：1, + | 2, +++")
}

// submitTypedReadout sends one categorical/table readout through the Bridge
// v2 generic channel. Success keeps schema and kind for the next sample;
// the fact itself only changes when Core projects it on reload.
func (s *State) submitTypedReadout(task *RunTaskViewModel, form *typedReadoutForm) {
	if s == nil || task == nil || form == nil || s.Callbacks.ExecuteCommand == nil {
		return
	}
	stepPath, assayID, protocolVersionID, timepoint := eventScope(task)
	var input runs.ExecuteCommandInput
	var err error
	switch form.Kind {
	case typedReadoutCategorical:
		input, err = buildCategoricalCommand(s.VM.RunID, stepPath, protocolVersionID, assayID, timepoint,
			form.SchemaID, form.Schema, form.SampleID, form.Category, newIdempotencyKey())
	case typedReadoutTable:
		var columns []string
		var rows [][]any
		columns, rows, err = parseTableValue(form.Columns, form.Rows)
		if err == nil {
			input, err = buildTableCommand(s.VM.RunID, stepPath, protocolVersionID, assayID, timepoint,
				form.SchemaID, form.Schema, form.SampleID, columns, rows, newIdempotencyKey())
		}
	default:
		err = fmt.Errorf("请选择读数类型")
	}
	if err != nil {
		form.Err = err.Error()
		return
	}
	form.Busy, form.Err, form.Done = true, "", ""
	_, err = s.Callbacks.ExecuteCommand(input)
	form.Busy = false
	if err != nil {
		form.Err = channelError(err)
		return
	}
	form.Category, form.SampleID, form.Rows = "", "", ""
	form.Err = ""
	form.Done = typedReadoutLabel(form.Kind) + "读数已提交（" + time.Now().Format("15:04") + "）"
}
