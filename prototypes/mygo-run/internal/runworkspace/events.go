package runworkspace

import (
	"fmt"
	"strings"
	"time"

	"asterlab-mygo-prototype/internal/theme"
	"asterlab-mygo-prototype/modules/runs"

	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/onevra/source/components"
)

// Composer kinds. Empty means the composer is folded away.
const (
	eventKindSkip       = "skip"
	eventKindDeviation  = "deviation"
	eventKindObservation = "observation"
)

// eventScope carries the frozen step scope both the UI and Core use to attach
// an event. A task without an assay stays unscoped: Core rejects a step
// reference outside its assay, so the surface never invents one.
func eventScope(task *RunTaskViewModel) (stepPath, assayID, protocolVersionID, timepoint string) {
	if task == nil {
		return "", "", "", ""
	}
	if strings.TrimSpace(task.AssayID) == "" {
		return "", "", "", strings.TrimSpace(task.Timepoint)
	}
	return strings.TrimSpace(task.StepPath),
		strings.TrimSpace(task.AssayID),
		strings.TrimSpace(task.ProtocolVersionID),
		strings.TrimSpace(task.Timepoint)
}

// SkipEventInput builds the conditional-skip payload. Core stamps
// step_status=skipped itself; the caller never marks the step complete — the
// skip path must not call CompleteTask.
func SkipEventInput(task *RunTaskViewModel, reason string) runs.ExecutionEventInput {
	content := "按规则跳过"
	if trimmed := strings.TrimSpace(reason); trimmed != "" {
		content += "：" + trimmed
	}
	stepPath, assayID, protocolVersionID, timepoint := eventScope(task)
	input := runs.ExecutionEventInput{
		EventType: "skipped_by_rule",
		Content:   content,
	}
	if assayID != "" {
		input.StepPath, input.AssayID, input.ProtocolVersionID = stepPath, assayID, protocolVersionID
	}
	input.Timepoint = timepoint
	return input
}

// DeviationEventInput builds the deviation payload through the same entry
// point as skips. Core requires a non-empty deviation explanation.
func DeviationEventInput(task *RunTaskViewModel, deviation string) runs.ExecutionEventInput {
	stepPath, assayID, protocolVersionID, timepoint := eventScope(task)
	input := runs.ExecutionEventInput{
		EventType: "deviation",
		Content:   strings.TrimSpace(deviation),
		Deviation: strings.TrimSpace(deviation),
	}
	if assayID != "" {
		input.StepPath, input.AssayID, input.ProtocolVersionID = stepPath, assayID, protocolVersionID
	}
	input.Timepoint = timepoint
	return input
}

// ObservationEventInput builds the free observation payload, optionally
// naming one pantry ledger row. The pantry item is a reference, never a copy.
func ObservationEventInput(task *RunTaskViewModel, content, pantryItemID string) runs.ExecutionEventInput {
	stepPath, assayID, protocolVersionID, timepoint := eventScope(task)
	input := runs.ExecutionEventInput{
		EventType:    "observation",
		Content:      strings.TrimSpace(content),
		PantryItemID: strings.TrimSpace(pantryItemID),
	}
	if assayID != "" {
		input.StepPath, input.AssayID, input.ProtocolVersionID = stepPath, assayID, protocolVersionID
	}
	input.Timepoint = timepoint
	return input
}

// RoundEventInput appends a completed countdown round as an observation. A
// completed round is a returned fact, not stored timer state: Core reads the
// round history back from events, so the timer panel files it here instead of
// keeping a second table.
func RoundEventInput(task *RunTaskViewModel, round runs.TimerRound) runs.ExecutionEventInput {
	planned := "未设"
	if round.PlannedS != nil {
		planned = FormatHMS(*round.PlannedS)
	}
	content := fmt.Sprintf("计时轮次完成：%s 第 %d 轮，用时 %s（计划 %s）",
		round.PhaseName, round.RoundIndex+1, FormatHMS(round.ActualS), planned)
	if round.PhaseCompleted {
		content += "，阶段完成"
	}
	if round.TimerCompleted {
		content += "，计时完成"
	}
	stepPath, assayID, protocolVersionID, timepoint := eventScope(task)
	input := runs.ExecutionEventInput{
		EventType:  "observation",
		Content:    content,
		OccurredAt: round.EndedAtUTC,
	}
	if assayID != "" {
		input.StepPath, input.AssayID, input.ProtocolVersionID = stepPath, assayID, protocolVersionID
	}
	input.Timepoint = timepoint
	return input
}

// renderEventComposer draws the rule-skip / deviation / observation entry.
// All three go through CreateExecutionEvent via AppendEvent; none of them
// completes the step.
func renderEventComposer(c *ui.Context, s *State, task *RunTaskViewModel, t *ui.Theme, tokens theme.Tokens) {
	if s.Callbacks.AppendEvent == nil || task == nil {
		return
	}
	if task.Status == TaskStatusCompleted || task.Status == TaskStatusSkipped {
		return
	}
	bench := s.bench()
	form := bench.eventForm(task.TaskKey)

	ui.Column(c).FillWidth().Gap(t.Space(1.5)).Children(func() {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(t.Space(1.5)).Children(func() {
			ui.Text(c, "偏差与跳过").FontSize(float32(tokens.TextSection)).FontWeight(600).TextColor(t.Text)
			ui.Text(c, "经由执行事件写入").FontSize(t.Rem(.7)).TextColor(t.TextMuted)
		})
		ui.Row(c).FillWidth().Wrap().AlignItems(ui.Center).Gap(t.Space(1)).Children(func() {
			for _, kind := range []struct{ value, label string }{
				{eventKindSkip, "按规则跳过"},
				{eventKindDeviation, "记偏差"},
				{eventKindObservation, "记观察"},
			} {
				kind := kind
				label := kind.label
				if form.Kind == kind.value {
					label = "● " + label
				}
				if components.Button(c, label, components.ButtonSecondary).Clicked() {
					if form.Kind == kind.value {
						form.Kind = ""
					} else {
						form.Kind = kind.value
					}
				}
			}
		})
		switch form.Kind {
		case eventKindSkip:
			renderSkipForm(c, s, task, form, t, tokens)
		case eventKindDeviation:
			renderDeviationForm(c, s, task, form, t, tokens)
		case eventKindObservation:
			renderObservationForm(c, s, task, form, t, tokens)
		}
		if form.Done != "" {
			ui.Text(c, form.Done).FontSize(t.Rem(.74)).TextColor(tokens.SuccessText)
		}
		if form.Err != "" {
			ui.Text(c, "提交失败："+form.Err).FontSize(t.Rem(.74)).TextColor(tokens.DangerText)
		}
	})
}

func renderSkipForm(c *ui.Context, s *State, task *RunTaskViewModel, form *eventForm, t *ui.Theme, tokens theme.Tokens) {
	ui.Column(c).FillWidth().Padding(t.Space(1.5), t.Space(2)).Radius(6).
		Background(tokens.WarningBg).Border(1, tokens.WarningText).Gap(t.Space(1)).Children(func() {
		ui.Text(c, "按规则跳过只记录事件，不会把本步骤标为完成。").FontSize(t.Rem(.74)).TextColor(tokens.WarningText)
		ui.Text(c, "跳过原因（可选）").FontSize(t.Rem(.72)).TextColor(t.TextMuted)
		ui.TextInput(c, &form.Text).FillWidth().Placeholder("例如：无气泡，符合跳过条件")
		ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(t.Space(1)).Children(func() {
			if submit := components.Button(c, "确认按规则跳过", components.ButtonPrimary); submit.Clicked() && !form.Busy {
				s.submitEvent(task, form, SkipEventInput(task, form.Text))
			}
			if form.Busy {
				ui.Text(c, "提交中…").FontSize(t.Rem(.72)).TextColor(t.TextMuted)
			}
		})
	})
}

func renderDeviationForm(c *ui.Context, s *State, task *RunTaskViewModel, form *eventForm, t *ui.Theme, tokens theme.Tokens) {
	ui.Column(c).FillWidth().Gap(t.Space(1)).Children(func() {
		ui.Text(c, "偏差说明（必填，写入 Core）").FontSize(t.Rem(.72)).TextColor(t.TextMuted)
		ui.TextInput(c, &form.Text).FillWidth().Placeholder("例如：实际离心 5 分钟，与方案 3 分钟不符")
		renderPantryRow(c, s, form, t, tokens)
		ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(t.Space(1)).Children(func() {
			if submit := components.Button(c, "记录偏差", components.ButtonPrimary); submit.Clicked() && !form.Busy {
				if strings.TrimSpace(form.Text) == "" {
					form.Err = "请填写偏差说明"
					return
				}
				s.submitEvent(task, form, DeviationEventInput(task, form.Text))
			}
			if form.Busy {
				ui.Text(c, "提交中…").FontSize(t.Rem(.72)).TextColor(t.TextMuted)
			}
		})
	})
}

func renderObservationForm(c *ui.Context, s *State, task *RunTaskViewModel, form *eventForm, t *ui.Theme, tokens theme.Tokens) {
	ui.Column(c).FillWidth().Gap(t.Space(1)).Children(func() {
		ui.Text(c, "观察记录").FontSize(t.Rem(.72)).TextColor(t.TextMuted)
		ui.TextInput(c, &form.Text).FillWidth().Placeholder("例如：3% 组 405nm 固化 42 秒，表面平整")
		renderPantryRow(c, s, form, t, tokens)
		ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(t.Space(1)).Children(func() {
			if submit := components.Button(c, "记录观察", components.ButtonPrimary); submit.Clicked() && !form.Busy {
				if strings.TrimSpace(form.Text) == "" {
					form.Err = "请填写观察内容"
					return
				}
				s.submitEvent(task, form, ObservationEventInput(task, form.Text, form.Pantry))
			}
			if form.Busy {
				ui.Text(c, "提交中…").FontSize(t.Rem(.72)).TextColor(t.TextMuted)
			}
		})
	})
}

// renderPantryRow offers the optional pantry reference. The ledger loads on
// first expand through LoadPantry; without that callback the row stays hidden
// and events carry no pantry reference.
func renderPantryRow(c *ui.Context, s *State, form *eventForm, t *ui.Theme, tokens theme.Tokens) {
	if s.Callbacks.LoadPantry == nil {
		return
	}
	bench := s.bench()
	open := form.PantryOpen
	components.Collapsible(c, "关联耗材（可选）", &open, components.CollapsibleOptions{}, func() {
		if !bench.PantryLoaded && !bench.PantryBusy {
			bench.PantryBusy = true
			ledger, err := s.Callbacks.LoadPantry()
			bench.PantryBusy = false
			if err != nil {
				bench.PantryErr = err.Error()
			} else {
				bench.Pantry = &ledger
				bench.PantryErr = ""
			}
			bench.PantryLoaded = true
		}
		switch {
		case bench.PantryBusy:
			ui.Text(c, "台账加载中…").FontSize(t.Rem(.72)).TextColor(t.TextMuted)
		case bench.PantryErr != "":
			ui.Text(c, "台账加载失败："+bench.PantryErr).FontSize(t.Rem(.72)).TextColor(tokens.DangerText)
		case bench.Pantry == nil || len(bench.Pantry.Items) == 0:
			ui.Text(c, "台账为空。").FontSize(t.Rem(.72)).TextColor(t.TextMuted)
		default:
			chosen := form.Pantry
			ui.Column(c).FillWidth().Gap(t.Space(.75)).Children(func() {
				for _, item := range bench.Pantry.Items {
					item := item
					label := item.Name
					if item.LabelCode != "" {
						label = item.LabelCode + " · " + item.Name
					}
					if item.Status != "" && item.Status != "unopened" {
						label += "（" + pantryStatusLabel(item.Status) + "）"
					}
					prefix := "○ "
					if chosen == item.ID {
						prefix = "● "
					}
					if components.Button(c, prefix+label, components.ButtonGhost).Clicked() {
						if form.Pantry == item.ID {
							form.Pantry = ""
						} else {
							form.Pantry = item.ID
						}
					}
				}
			})
		}
	})
	form.PantryOpen = open
}

// pantryStatusLabel prints the stored pantry status, never the raw enum.
func pantryStatusLabel(status string) string {
	switch status {
	case "opened":
		return "已开封"
	case "near_expiry":
		return "近效期"
	case "expired":
		return "已过期"
	case "exhausted":
		return "已耗尽"
	default:
		return "未开封"
	}
}

// submitEvent appends one event through the host. Success clears the form and
// reports where the fact went; the step status itself only changes when Core
// projects it on reload.
func (s *State) submitEvent(task *RunTaskViewModel, form *eventForm, input runs.ExecutionEventInput) {
	if s == nil || task == nil || form == nil || s.Callbacks.AppendEvent == nil {
		return
	}
	form.Busy, form.Err, form.Done = true, "", ""
	notice, err := s.Callbacks.AppendEvent(task.TaskKey, input)
	form.Busy = false
	if err != nil {
		form.Err = eventSubmitError(err)
		return
	}
	form.Text, form.Pantry, form.Kind = "", "", ""
	form.Err = ""
	form.Done = "已追加到 Run 记录（" + time.Now().Format("15:04") + "）"
	if strings.TrimSpace(notice) != "" {
		form.Done += " · " + strings.TrimSpace(notice)
	}
}

// eventSubmitError maps a Core event rejection to operator wording (UX-P5).
func eventSubmitError(err error) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	switch {
	case strings.Contains(msg, "NOT_FOUND"):
		return "引用的耗材或步骤不存在，请刷新后重试"
	case strings.Contains(msg, "FROZEN") || strings.Contains(msg, "frozen"):
		return "Run 已冻结，事件不可再记"
	case strings.Contains(msg, "INVALID"):
		return "事件不合法（" + msg + "）"
	default:
		return msg
	}
}
