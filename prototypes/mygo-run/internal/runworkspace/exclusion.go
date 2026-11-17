package runworkspace

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/onevra/source/components"

	"asterlab-mygo-prototype/internal/theme"
	"asterlab-mygo-prototype/modules/runs"
)

// Sample exclusion statuses. The strings are Core's frozen step_status enum
// (BE-08): qc_dropped/censored_dead count in workflow.progress but never
// count as step completion. Go never invents a fourth state.
const (
	exclusionQCDropped    = "qc_dropped"
	exclusionCensoredDead = "censored_dead"
	exclusionRework       = "rework"
)

// exclusionLabel is the operator wording for an exclusion status. Raw enum
// text never reaches the default view (UX-P2).
func exclusionLabel(status string) string {
	switch status {
	case exclusionQCDropped:
		return "QC 剔除"
	case exclusionCensoredDead:
		return "删失（中途死亡）"
	case exclusionRework:
		return "重做"
	default:
		return status
	}
}

// parseSampleIDs splits operator-typed sample ids on commas, whitespace,
// semicolons and newlines, dropping empties and duplicates. Sample ids are
// operator labels, never raw record identifiers (UX-P2).
func parseSampleIDs(text string) []string {
	seen := map[string]bool{}
	var out []string
	for _, part := range strings.FieldsFunc(text, func(r rune) bool {
		return r == ',' || r == ';' || r == '\n' || r == '\r' || r == ' ' || r == '\t'
	}) {
		id := strings.TrimSpace(part)
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return out
}

// exclusionCommandJSON is the exact execution_event.create input Core
// validates (BE-08): event_type operation with a step_status, the frozen
// step scope, and the concerned samples. blind_masked travels only when the
// write happens under the blind mask; parent_sample_id/aliquot_from have no
// composer yet and stay omitted.
type exclusionCommandJSON struct {
	RunID             string   `json:"run_id"`
	EventType         string   `json:"event_type"`
	Content           string   `json:"content"`
	StepStatus        string   `json:"step_status"`
	ProtocolStepPath  string   `json:"protocol_step_path,omitempty"`
	ProtocolVersionID string   `json:"protocol_version_id,omitempty"`
	AssayID           string   `json:"assay_id,omitempty"`
	Timepoint         string   `json:"timepoint,omitempty"`
	SampleIDs         []string `json:"sample_ids"`
	BlindMasked       bool     `json:"blind_masked,omitempty"`
}

// buildExclusionCommand packs one sample-exclusion write into the Bridge v2
// generic envelope. Validation here is only what the UI can check without
// Core (known status, at least one sample); every frozen-scope and
// sample-identity rule stays Core's.
func buildExclusionCommand(runID, status string, samples []string, reason string, stepPath, assayID, protocolVersionID, timepoint string, blindMasked bool, idempotencyKey string) (runs.ExecuteCommandInput, error) {
	switch status {
	case exclusionQCDropped, exclusionCensoredDead, exclusionRework:
	default:
		return runs.ExecuteCommandInput{}, fmt.Errorf("unknown exclusion status %q", status)
	}
	if len(samples) == 0 {
		return runs.ExecuteCommandInput{}, fmt.Errorf("请填写至少一个样本 ID")
	}
	content := exclusionLabel(status)
	if trimmed := strings.TrimSpace(reason); trimmed != "" {
		content += "：" + trimmed
	}
	payload, err := json.Marshal(exclusionCommandJSON{
		RunID:             runID,
		EventType:         "operation",
		Content:           content,
		StepStatus:        status,
		ProtocolStepPath:  stepPath,
		ProtocolVersionID: protocolVersionID,
		AssayID:           assayID,
		Timepoint:         timepoint,
		SampleIDs:         samples,
		BlindMasked:       blindMasked,
	})
	if err != nil {
		return runs.ExecuteCommandInput{}, err
	}
	if strings.TrimSpace(idempotencyKey) == "" {
		idempotencyKey = newIdempotencyKey()
	}
	return runs.ExecuteCommandInput{
		Kind:           channelCommandExecEvent,
		IdempotencyKey: idempotencyKey,
		PayloadJSON:    string(payload),
	}, nil
}

// exclusionForm holds the sample-exclusion draft per task.
type exclusionForm struct {
	Status  string
	Samples string
	Reason  string
	Busy    bool
	Err     string
	Done    string
}

func (s *State) exclusionForm(taskKey string) *exclusionForm {
	bench := s.bench()
	if bench.exclusionForms == nil {
		bench.exclusionForms = map[string]*exclusionForm{}
	}
	form, ok := bench.exclusionForms[taskKey]
	if !ok {
		form = &exclusionForm{}
		bench.exclusionForms[taskKey] = form
	}
	return form
}

// renderExclusionSection draws the sample-exclusion composer: one of the
// three Core exclusion states plus the concerned samples, written through
// ExecuteCommand. It never marks the step complete (UX-P1, UX-STATE,
// UX-DESIGN).
func renderExclusionSection(c *ui.Context, s *State, task *RunTaskViewModel, t *ui.Theme, tokens theme.Tokens) {
	if s.Callbacks.ExecuteCommand == nil || task == nil {
		return
	}
	form := s.exclusionForm(task.TaskKey)
	ui.Column(c).FillWidth().Gap(t.Space(1.5)).Children(func() {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(t.Space(1.5)).Children(func() {
			ui.Text(c, "样本排除").FontSize(float32(tokens.TextSection)).FontWeight(600).TextColor(t.Text)
			ui.Text(c, "经由执行事件写入 Core，不计入步骤完成").FontSize(t.Rem(.7)).TextColor(t.TextMuted)
		})
		ui.Row(c).FillWidth().Wrap().AlignItems(ui.Center).Gap(t.Space(1)).Children(func() {
			for _, status := range []string{exclusionQCDropped, exclusionCensoredDead, exclusionRework} {
				status := status
				label := exclusionLabel(status)
				if form.Status == status {
					label = "● " + label
				}
				if components.Button(c, label, components.ButtonSecondary).Clicked() {
					if form.Status == status {
						form.Status = ""
					} else {
						form.Status = status
					}
				}
			}
		})
		if form.Status == "" {
			return
		}
		ui.Column(c).FillWidth().Padding(t.Space(1.5), t.Space(2)).Radius(6).
			Background(tokens.WarningBg).Border(1, tokens.WarningText).Gap(t.Space(1)).Children(func() {
			ui.Text(c, exclusionLabel(form.Status)+"只记录样本状态，workflow.progress 按该状态计数但不把步骤标为完成。").
				FontSize(t.Rem(.74)).TextColor(tokens.WarningText)
			ui.Text(c, "样本 ID（必填，逗号或换行分隔）").FontSize(t.Rem(.72)).TextColor(t.TextMuted)
			ui.TextInput(c, &form.Samples).FillWidth().Placeholder("例如：M3-1, M3-2")
			ui.Text(c, "原因（可选，随事件写入）").FontSize(t.Rem(.72)).TextColor(t.TextMuted)
			ui.TextInput(c, &form.Reason).FillWidth().Placeholder("例如：溶血，OD 异常偏高")
			ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(t.Space(1)).Children(func() {
				if submit := components.Button(c, "记录排除", components.ButtonSecondary); submit.Clicked() && !form.Busy {
					s.submitExclusion(task, form)
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

// submitExclusion sends one exclusion write through the Bridge v2 generic
// channel. Success clears the samples but keeps the status so the operator
// can exclude more samples; the step status itself only changes when Core
// projects it on reload.
func (s *State) submitExclusion(task *RunTaskViewModel, form *exclusionForm) {
	if s == nil || task == nil || form == nil || s.Callbacks.ExecuteCommand == nil {
		return
	}
	samples := parseSampleIDs(form.Samples)
	stepPath, assayID, protocolVersionID, timepoint := eventScope(task)
	input, err := buildExclusionCommand(s.VM.RunID, form.Status, samples, form.Reason,
		stepPath, assayID, protocolVersionID, timepoint, s.bench().Blind, newIdempotencyKey())
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
	form.Samples, form.Reason = "", ""
	form.Err = ""
	form.Done = exclusionLabel(form.Status) + "已记录（" + time.Now().Format("15:04") + "）"
}
