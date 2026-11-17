package record

import "strings"

// ParamRow is one display-ready planned-vs-actual row. Values and the status
// word come from one Core comparison; Go only formats them.
type ParamRow struct {
	Label   string
	Planned string
	Actual  string
	Status  string
}

// ParamGroup is one scope's planned-vs-actual rows, in first-seen order.
type ParamGroup struct {
	Title string
	Rows  []ParamRow
}

// GroupParameters groups Core parameter comparisons by scope for the record
// surfaces. No re-derivation: the comparison (planned, actuals, status) is
// Core's; this only shapes it for tables.
func GroupParameters(params []ParamCompare) []ParamGroup {
	var out []ParamGroup
	index := map[string]int{}
	for _, param := range params {
		title := strings.TrimSpace(param.ScopeID)
		if param.StepPath != nil && strings.TrimSpace(*param.StepPath) != "" {
			title += " · " + strings.TrimSpace(*param.StepPath)
		}
		if title == "" {
			title = "未命名范围"
		}
		i, ok := index[title]
		if !ok {
			i = len(out)
			index[title] = i
			out = append(out, ParamGroup{Title: title})
		}
		out[i].Rows = append(out[i].Rows, CompareRow(param))
	}
	return out
}

// CompareRow renders one Core comparison: the planned value, every recorded
// actual value, and the comparison status word.
func CompareRow(param ParamCompare) ParamRow {
	label := strings.TrimSpace(param.Slot)
	if param.Label != nil && strings.TrimSpace(*param.Label) != "" {
		label = strings.TrimSpace(*param.Label)
	}
	actuals := make([]string, 0, len(param.Actual))
	for _, actual := range param.Actual {
		text := displayJSON(actual.Value)
		if actual.Unit != nil && strings.TrimSpace(*actual.Unit) != "" {
			text += " " + strings.TrimSpace(*actual.Unit)
		}
		actuals = append(actuals, text)
	}
	actual := "—"
	if len(actuals) > 0 {
		actual = strings.Join(actuals, "；")
	}
	return ParamRow{
		Label:   label,
		Planned: displayJSON(param.Planned),
		Actual:  actual,
		Status:  ParamStatusWord(param.Status),
	}
}

// displayJSON renders one Core JSON value. Missing stays —, never zero.
func displayJSON(raw []byte) string {
	text := strings.TrimSpace(string(raw))
	if text == "" || text == "null" {
		return "—"
	}
	return text
}
