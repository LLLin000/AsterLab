package designer

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"

	"asterlab-mygo-prototype/internal/theme"
	"asterlab-mygo-prototype/internal/workflow"
	"asterlab-mygo-prototype/modules/runs"
	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/onevra/source/components"
	"github.com/oneo-me/onevra/source/icons/lucide"
)

// Canonical step-timing param keys. Only these keys feed the Core time
// budget: free-text durations elsewhere stay annotations and never mask a
// gap (CLOSE-03 §6, UX-P5).
const (
	TimingKindKey   = "计时类型"
	TimingDurKey    = "计时时长"
	TimingRepeatKey = "重复次数"
	TimingAnchorKey = "时间锚点"

	// Second-phase keys for alternating cycles (e.g. cold/hot). One step
	// carries at most two phases; a third timing is a second step.
	TimingKind2Key   = "计时类型2"
	TimingDur2Key    = "计时时长2"
	TimingRepeat2Key = "重复次数2"
)

// Timing kinds in operator words. They only affect the Core projection:
// 占手 counts hands-on, the others count span/parallel windows. All three
// relay as countdown phases in Run.
const (
	TimingHandsOn = "占手"
	TimingWait    = "等待"
	TimingTimed   = "定时终点"
)

// StepTiming is one parsed timing entry. HasTiming is false when the step
// declares no duration; Err names the cause when a declared value is
// unusable (UX-P5: never silently dropped).
type StepTiming struct {
	Kind      string
	DurationS float64
	Repeat    uint32
	Anchor    string
	HasTiming bool
	Err       string
}

// ParseStepTiming reads the primary (and optional second-phase) timing of
// one step. Missing keys mean "not declared", never zero.
func ParseStepTiming(params []workflow.NodeParam) (first, second StepTiming) {
	first = parseTimingEntry(paramVal(params, TimingKindKey), paramVal(params, TimingDurKey),
		paramVal(params, TimingRepeatKey), paramVal(params, TimingAnchorKey))
	second = parseTimingEntry(paramVal(params, TimingKind2Key), paramVal(params, TimingDur2Key),
		paramVal(params, TimingRepeat2Key), paramVal(params, TimingAnchorKey))
	return first, second
}

func parseTimingEntry(kind, dur, repeat, anchor string) StepTiming {
	kind = strings.TrimSpace(kind)
	dur = strings.TrimSpace(dur)
	repeat = strings.TrimSpace(repeat)
	anchor = strings.TrimSpace(anchor)
	if dur == "" {
		// A kind or repeat without a duration declares nothing.
		return StepTiming{}
	}
	out := StepTiming{HasTiming: true, Anchor: anchor}
	switch kind {
	case "", TimingWait:
		out.Kind = TimingWait
	case TimingHandsOn:
		out.Kind = TimingHandsOn
	case TimingTimed:
		out.Kind = TimingTimed
	default:
		out.Err = fmt.Sprintf("计时类型 %q 须为%s/%s/%s之一", kind, TimingHandsOn, TimingWait, TimingTimed)
		return out
	}
	seconds, ok := ParseDurationS(dur)
	if !ok {
		out.Err = fmt.Sprintf("计时时长 %q 无法解析（如 5m、10min、1.5h、30s）", dur)
		return out
	}
	out.DurationS = seconds
	if repeat == "" {
		out.Repeat = 1
		return out
	}
	n, err := strconv.Atoi(repeat)
	if err != nil || n <= 0 {
		out.Err = fmt.Sprintf("重复次数 %q 须为正整数", repeat)
		return out
	}
	out.Repeat = uint32(n)
	return out
}

// ParseDurationS reads a lab-style duration: number plus s/m/h/d unit.
// A bare number means minutes, matching the card annotation convention.
func ParseDurationS(raw string) (float64, bool) {
	s := strings.ToLower(strings.TrimSpace(raw))
	if s == "" {
		return 0, false
	}
	mult := 60.0
	unit := ""
	for _, u := range []string{"mins", "min", "secs", "sec", "hrs", "hour", "分钟", "小时"} {
		if strings.HasSuffix(s, u) {
			unit = u
			break
		}
	}
	if unit == "" {
		for _, u := range []string{"m", "s", "h", "d", "分", "秒", "时", "天"} {
			if strings.HasSuffix(s, u) {
				unit = u
				break
			}
		}
	}
	num := s
	if unit != "" {
		num = strings.TrimSpace(strings.TrimSuffix(s, unit))
	}
	v, err := strconv.ParseFloat(num, 64)
	if err != nil || math.IsNaN(v) || math.IsInf(v, 0) || v <= 0 {
		return 0, false
	}
	switch unit {
	case "", "m", "min", "mins", "分", "分钟":
		mult = 60
	case "s", "sec", "secs", "秒":
		mult = 1
	case "h", "hr", "hrs", "hour", "小时", "时":
		mult = 3600
	case "d", "day", "天":
		mult = 86400
	default:
		return 0, false
	}
	seconds := v * mult
	if math.IsNaN(seconds) || math.IsInf(seconds, 0) || seconds <= 0 || seconds > 1e12 {
		return 0, false
	}
	return seconds, true
}

func paramVal(params []workflow.NodeParam, key string) string {
	for _, p := range params {
		if strings.TrimSpace(p.Key) == key {
			return p.Val
		}
	}
	return ""
}

// TimingSummary renders the step's declared timing for the card line, e.g.
// "等待 5m ×3" or "占手 45m". Unknowns render as names, never numbers.
func TimingSummary(first, second StepTiming) string {
	one := func(t StepTiming) string {
		if !t.HasTiming || t.Err != "" {
			return ""
		}
		s := fmt.Sprintf("%s %s", t.Kind, FormatDurationS(t.DurationS))
		if t.Repeat > 1 {
			s += fmt.Sprintf(" ×%d", t.Repeat)
		}
		return s
	}
	parts := []string{}
	if s := one(first); s != "" {
		parts = append(parts, s)
	}
	if s := one(second); s != "" {
		parts = append(parts, s)
	}
	return strings.Join(parts, " ↔ ")
}

// FormatDurationS renders seconds compactly for operators.
func FormatDurationS(seconds float64) string {
	total := int64(seconds + 0.5)
	h, m, s := total/3600, (total%3600)/60, total%60
	switch {
	case h > 0 && m > 0:
		return fmt.Sprintf("%dh%dm", h, m)
	case h > 0:
		return fmt.Sprintf("%dh", h)
	case m > 0 && s > 0:
		return fmt.Sprintf("%dm%ds", m, s)
	case m > 0:
		return fmt.Sprintf("%dm", m)
	default:
		return fmt.Sprintf("%ds", s)
	}
}

// BudgetInput builds the OperationSpec v2 array (protocol order) for Core's
// TimeBudgetService. Steps without usable timing are omitted: Core counts
// them as unknown instead of receiving a guessed zero. Problems carry the
// step-scoped cause for the budget bar (UX-P5).
func BudgetInput(nodes []workflow.Node) (specsJSON string, known, unknown int, problems []string) {
	specs := []any{}
	for i, node := range nodes {
		first, second := ParseStepTiming(node.Params)
		made := false
		for phase, t := range []StepTiming{first, second} {
			if !t.HasTiming {
				continue
			}
			if t.Err != "" {
				problems = append(problems, fmt.Sprintf("步骤%02d（%s）%s，已按未知计", i+1, stepName(node), t.Err))
				continue
			}
			specs = append(specs, timingSpec(t))
			made = true
			_ = phase
		}
		if made {
			known++
		} else {
			unknown++
		}
	}
	raw, err := json.Marshal(specs)
	if err != nil {
		return "[]", 0, len(nodes), []string{"计时输入序列化失败，已按全部未知计"}
	}
	return string(raw), known, unknown, problems
}

func stepName(node workflow.Node) string {
	if strings.TrimSpace(node.Title) != "" {
		return strings.TrimSpace(node.Title)
	}
	return strings.TrimSpace(node.ID)
}

func timingCoreKind(kind string) string {
	switch kind {
	case TimingHandsOn:
		return "hands_on"
	case TimingTimed:
		return "timed_endpoint"
	default:
		return "wait"
	}
}

// timingSpec is the OperationSpec v2 sidecar for one timing entry. The
// operation carrier is neutral (all-None incubate): the budget only reads
// timing/repeat/anchor, and the step's real operation lives in its block.
func timingSpec(t StepTiming) any {
	timing := map[string]any{
		"kind":     timingCoreKind(t.Kind),
		"duration": map[string]any{"value": t.DurationS, "unit": "s"},
	}
	spec := map[string]any{
		"schema_version": 2,
		"operation":      map[string]any{"kind": "incubate"},
		"timing":         timing,
	}
	if t.Repeat > 1 {
		spec["repeat"] = map[string]any{"count": t.Repeat}
	}
	if t.Anchor != "" {
		spec["time_anchor_id"] = t.Anchor
	}
	return spec
}

// PhasesForStep derives the Run relay plan from the frozen step's declared
// timing. Nil means the step declares no usable timing: the timer keeps its
// own single phase (Core from_record), never an empty plan.
func PhasesForStep(params []workflow.NodeParam, title string) []runs.TimerPhase {
	first, second := ParseStepTiming(params)
	var out []runs.TimerPhase
	for i, t := range []StepTiming{first, second} {
		if !t.HasTiming || t.Err != "" {
			continue
		}
		name := strings.TrimSpace(title)
		if i == 1 {
			name += "（二段）"
		}
		planned := uint64(t.DurationS + 0.5)
		if planned == 0 {
			planned = 1
		}
		rounds := t.Repeat
		if rounds < 1 {
			rounds = 1
		}
		out = append(out, runs.TimerPhase{Name: name, PlannedS: &planned, Mode: "countdown", Rounds: &rounds})
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// PhasesForFrozenStep resolves one frozen workflow node's relay plan from
// the frozen design JSON. Unknown step or no timing returns nil.
func PhasesForFrozenStep(designData, stepID string) []runs.TimerPhase {
	if strings.TrimSpace(designData) == "" || strings.TrimSpace(stepID) == "" {
		return nil
	}
	var doc designDoc
	if err := json.Unmarshal([]byte(designData), &doc); err != nil {
		return nil
	}
	for _, node := range doc.MyGo.Workflow.Nodes {
		if node.ID == stepID {
			return PhasesForStep(node.Params, stepName(node))
		}
	}
	return nil
}

// setNodeParam upserts one canonical param on the selected step. An empty
// value removes the key so "unset" never persists as a blank estimate.
func (d *Designer) setNodeParam(nodeID, key, val string) {
	val = strings.TrimSpace(val)
	for i := range d.Editor.Doc.Nodes {
		if d.Editor.Doc.Nodes[i].ID != nodeID {
			continue
		}
		params := d.Editor.Doc.Nodes[i].Params[:0]
		kept := false
		for _, p := range d.Editor.Doc.Nodes[i].Params {
			if strings.TrimSpace(p.Key) == key {
				if val != "" {
					params = append(params, workflow.NodeParam{Key: key, Val: val})
					kept = true
				}
				continue
			}
			params = append(params, p)
		}
		if val != "" && !kept {
			params = append(params, workflow.NodeParam{Key: key, Val: val})
		}
		d.Editor.Doc.Nodes[i].Params = params
		d.Touch()
		return
	}
}

// timingInspector edits the step's OperationSpec timing inputs. It is the
// only place that writes 计时* params; the budget bar and Run relay only
// read them (UX-P3 reuses properties/textRow/selectRow, UX-P1 keeps the
// primary Create-Run action in the toolbar).
func timingInspector(c *ui.Context, d *Designer) {
	var node *workflow.Node
	for i := range d.Editor.Doc.Nodes {
		if d.Editor.Doc.Nodes[i].ID == d.SelectedNode {
			node = &d.Editor.Doc.Nodes[i]
			break
		}
	}
	if node == nil {
		return
	}
	kind, dur, repeat, anchor := paramVal(node.Params, TimingKindKey),
		paramVal(node.Params, TimingDurKey), paramVal(node.Params, TimingRepeatKey),
		paramVal(node.Params, TimingAnchorKey)
	kind2, dur2, repeat2 := paramVal(node.Params, TimingKind2Key),
		paramVal(node.Params, TimingDur2Key), paramVal(node.Params, TimingRepeat2Key)
	properties(c, "计时", func(list *propertyList) {
		changed := false
		selectRow(list, "计时类型", "占手计入净占手；等待/定时终点计入总跨度与并行空窗", &kind,
			[]string{TimingHandsOn, TimingWait, TimingTimed}, &changed)
		if changed {
			d.setNodeParam(node.ID, TimingKindKey, kind)
		}
		if textRow(list, "计时时长", "单次时长，如 5m、10min、1.5h、30s；留空即未知", &dur, "例如 5m", lucide.Timer()) {
			d.setNodeParam(node.ID, TimingDurKey, dur)
		}
		if textRow(list, "重复次数", "正整数；留空为 1", &repeat, "例如 3", lucide.Repeat()) {
			d.setNodeParam(node.ID, TimingRepeatKey, repeat)
		}
		if textRow(list, "时间锚点", "可选，如 t0；周期计划必填", &anchor, "例如 t0", lucide.Anchor()) {
			d.setNodeParam(node.ID, TimingAnchorKey, anchor)
		}
		changed2 := false
		selectRow(list, "计时类型2", "交替循环的后段；留空则只有一段", &kind2,
			[]string{TimingHandsOn, TimingWait, TimingTimed}, &changed2)
		if changed2 {
			d.setNodeParam(node.ID, TimingKind2Key, kind2)
		}
		if textRow(list, "计时时长2", "后段单次时长，如 10m", &dur2, "例如 10m", lucide.Timer()) {
			d.setNodeParam(node.ID, TimingDur2Key, dur2)
		}
		if textRow(list, "重复次数2", "后段重复次数；留空为 1", &repeat2, "例如 3", lucide.Repeat()) {
			d.setNodeParam(node.ID, TimingRepeat2Key, repeat2)
		}
	})
	first, second := ParseStepTiming(node.Params)
	for _, t := range []StepTiming{first, second} {
		if t.Err != "" {
			components.Text(c, t.Err).FontSize(c.Theme().Rem(.75)).TextColor(theme.GetTokens(c.Theme()).Danger)
		}
	}
	if sum := TimingSummary(first, second); sum != "" {
		components.Text(c, "本步计时："+sum).FontSize(c.Theme().Rem(.75)).TextColor(theme.GetTokens(c.Theme()).TextMuted)
	}
}
