package runworkspace

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"asterlab-mygo-prototype/internal/theme"
	"asterlab-mygo-prototype/modules/runs"

	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/onevra/source/components"
)

// TimerDisplay is the checkpoint rendering of one persisted TimerRecord. Every
// number derives from the record Core stored (accumulated_s, started_at_utc,
// planned_s) measured at now; the UI keeps no ticking state of its own.
type TimerDisplay struct {
	Record runs.TimerRecord
	// ElapsedS is accumulated_s plus the running interval, saturating at zero
	// when the clock moved backwards. It mirrors Core timer.elapsed_s.
	ElapsedS uint64
	// RemainingS is planned_s minus elapsed, saturating at zero. Nil for a
	// stopwatch, which plans nothing. It mirrors Core timer.remaining_s.
	RemainingS *uint64
	// Over reports a countdown/window that ran past its plan.
	Over bool
	// ClockIssue reports a running timer without a usable anchor: Core would
	// reject its next transition, so the surface says so instead of guessing.
	ClockIssue bool
}

// DescribeTimer renders one persisted timer at now. Unparseable timestamps
// fall back to the stored accumulation; they are flagged, never invented.
func DescribeTimer(record runs.TimerRecord, now time.Time) TimerDisplay {
	out := TimerDisplay{Record: record, ElapsedS: record.AccumulatedS}
	elapsed := record.AccumulatedS
	if record.Status == "running" {
		if record.StartedAtUTC == nil || strings.TrimSpace(*record.StartedAtUTC) == "" {
			out.ClockIssue = true
		} else if started, err := time.Parse(time.RFC3339, strings.TrimSpace(*record.StartedAtUTC)); err != nil {
			out.ClockIssue = true
		} else {
			if delta := now.Sub(started); delta > 0 {
				elapsed += uint64(delta / time.Second)
			}
			out.ElapsedS = elapsed
		}
	}
	if record.PlannedS != nil {
		remaining := uint64(0)
		if *record.PlannedS > elapsed {
			remaining = *record.PlannedS - elapsed
		}
		out.RemainingS = &remaining
		out.Over = elapsed > *record.PlannedS
	}
	return out
}

// FormatHMS renders seconds for an operator: H:MM:SS past an hour, M:SS
// below. One formatting helper serves countdowns, stopwatches and windows.
func FormatHMS(totalS uint64) string {
	h, m, s := totalS/3600, (totalS%3600)/60, totalS%60
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%d:%02d", m, s)
}

// TimerStatusLabel prints the stored lifecycle status, never a raw enum
// (UX-STATE, UX-P2).
func TimerStatusLabel(status string) string {
	switch status {
	case "running":
		return "进行中"
	case "paused":
		return "已暂停"
	case "completed":
		return "已完成"
	default:
		return "就绪"
	}
}

// TimerModeLabel prints the stored timer mode (UX-STATE, UX-P2).
func TimerModeLabel(mode string) string {
	switch mode {
	case "stopwatch":
		return "正计时"
	case "window":
		return "时间窗"
	default:
		return "倒计时"
	}
}

// TimerIdentity renders one timer's explicit identity and scope without raw
// identifiers (UX-P2): mode plus sequence plus the operator group when the
// id carries one (task/mode/seq or task/mode/seq@group). Unparseable ids
// fall back to the mode label, never the raw id (QA-HUVEC F1, UX-STATE).
func TimerIdentity(record runs.TimerRecord) string {
	mode := TimerModeLabel(record.Mode)
	tail := record.TimerID
	if i := strings.LastIndex(tail, "/"); i >= 0 {
		tail = tail[i+1:]
	}
	group := ""
	if i := strings.Index(tail, "@"); i >= 0 {
		group = strings.TrimSpace(tail[i+1:])
		tail = tail[:i]
	}
	seq := strings.TrimSpace(tail)
	if seq == "" {
		return mode
	}
	for _, r := range seq {
		if r < '0' || r > '9' {
			return mode
		}
	}
	out := mode + " #" + seq
	if group != "" {
		out += " · " + group
	}
	return out
}

// RoundLabel prints the relay cursor: rounds are 1-based for the operator
// while the record keeps the 0-based index.
func RoundLabel(record runs.TimerRecord) string {
	current := record.RoundIndex + 1
	if record.RoundCount != nil && *record.RoundCount > 0 {
		return fmt.Sprintf("第 %d/%d 轮", current, *record.RoundCount)
	}
	return fmt.Sprintf("第 %d 轮", current)
}

// PhaseName resolves the relay phase name from the caller-supplied plan. An
// empty plan means the timer runs its own single phase (Core
// TimerSequence.from_record semantics); the name is then empty, never guessed.
func PhaseName(phases []runs.TimerPhase, phaseIndex uint32) string {
	if int(phaseIndex) < len(phases) {
		return phases[phaseIndex].Name
	}
	return ""
}

// WindowLine describes a t0 window from its checkpoint: before the anchor
// fires it waits; after, it counts down to started_at + planned.
func WindowLine(display TimerDisplay) string {
	record := display.Record
	if record.Status != "running" || record.StartedAtUTC == nil {
		anchor := record.AnchorTaskKey
		if anchor == nil || strings.TrimSpace(*anchor) == "" {
			anchor = record.AnchorEventID
		}
		if anchor != nil && strings.TrimSpace(*anchor) != "" {
			return fmt.Sprintf("等待 t0（%s 完成后自动起表）", strings.TrimSpace(*anchor))
		}
		return "等待 t0 触发"
	}
	if record.PlannedS == nil {
		return fmt.Sprintf("已用 %s", FormatHMS(display.ElapsedS))
	}
	started, err := time.Parse(time.RFC3339, strings.TrimSpace(*record.StartedAtUTC))
	if err != nil {
		return fmt.Sprintf("剩余 %s", FormatHMS(*display.RemainingS))
	}
	target := started.Add(time.Duration(*record.PlannedS) * time.Second)
	return fmt.Sprintf("目标 %s · 剩余 %s", target.Format("15:04"), FormatHMS(*display.RemainingS))
}

// timerHeadline is the one-line checkpoint summary per timer.
func timerHeadline(display TimerDisplay, phases []runs.TimerPhase) string {
	record := display.Record
	if record.Mode == "window" {
		return WindowLine(display)
	}
	if record.Mode == "stopwatch" {
		return fmt.Sprintf("已用 %s", FormatHMS(display.ElapsedS))
	}
	parts := []string{RoundLabel(record)}
	if name := PhaseName(phases, record.PhaseIndex); name != "" {
		parts = append([]string{name}, parts...)
	}
	if display.Over {
		return strings.Join(parts, " · ") + fmt.Sprintf(" · 超时 %s", FormatHMS(display.ElapsedS-*record.PlannedS))
	}
	return strings.Join(parts, " · ") + fmt.Sprintf(" · 剩余 %s", FormatHMS(*display.RemainingS))
}

// timerActionError maps a Core timer rejection to operator wording (UX-P5).
func timerActionError(err error) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	switch {
	case strings.Contains(msg, "NOT_FOUND"):
		return "计时器不存在，请刷新后重试"
	case strings.Contains(msg, "FROZEN") || strings.Contains(msg, "frozen"):
		return "Run 已冻结，计时器不可再改"
	case strings.Contains(msg, "INVALID"):
		return "计时器状态不允许该操作（" + msg + "）"
	default:
		return msg
	}
}

// renderTimerSection draws the selected task's persisted timers plus the
// create row. Every number comes from ListTimers checkpoints; actions go
// through SaveTimer/UpdateTimer via callbacks.
func renderTimerSection(c *ui.Context, s *State, task *RunTaskViewModel, t *ui.Theme, tokens theme.Tokens) {
	bench := s.bench()
	now := time.Now()
	timers := bench.Timers[task.TaskKey]
	phases := s.phasesFor(task.TaskKey)

	ui.Column(c).FillWidth().Gap(t.Space(1.5)).Children(func() {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(t.Space(1.5)).Children(func() {
			ui.Text(c, "计时").FontSize(float32(tokens.TextSection)).FontWeight(600).TextColor(t.Text)
			ui.Text(c, "持久化计时器 · 按检查点渲染").FontSize(t.Rem(.7)).TextColor(t.TextMuted)
			ui.Spacer(c)
			if s.Callbacks.LoadTimers != nil {
				if components.Button(c, "刷新", components.ButtonGhost).Clicked() {
					s.refreshTimers(task.TaskKey)
				}
			}
		})
		if msg := bench.TimersErr[task.TaskKey]; msg != "" {
			ui.Row(c).FillWidth().Padding(8, 10).Radius(6).
				Background(tokens.DangerBg).Border(1, tokens.DangerText).Children(func() {
				ui.Text(c, "计时器加载失败："+msg).FontSize(t.Rem(.74)).TextColor(tokens.DangerText)
			})
		}
		if s.Callbacks.LoadTimers == nil {
			ui.Text(c, "计时服务未接线，显示上次检查点。").FontSize(t.Rem(.72)).TextColor(t.TextMuted)
		}
		if len(timers) == 0 {
			ui.Text(c, "本步骤还没有计时器。").FontSize(t.Rem(.76)).TextColor(t.TextMuted)
		}
		for i := range timers {
			renderTimerRow(c, s, task, timers[i], phases, now, t, tokens)
		}
		renderTimerCreate(c, s, task, len(timers), t, tokens)
	})
}

func renderTimerRow(c *ui.Context, s *State, task *RunTaskViewModel, record runs.TimerRecord, phases []runs.TimerPhase, now time.Time, t *ui.Theme, tokens theme.Tokens) {
	bench := s.bench()
	display := DescribeTimer(record, now)
	busy := bench.TimerBusy == record.TimerID

	ui.Column(c).FillWidth().Padding(t.Space(1.5), t.Space(2)).Radius(6).
		Background(tokens.Elevated).Border(1, tokens.Border).Gap(t.Space(1)).Children(func() {
		ui.Row(c).FillWidth().Wrap().AlignItems(ui.Center).Gap(t.Space(1.5)).Children(func() {
			ui.Text(c, TimerIdentity(record)).FontSize(t.Rem(.72)).Bold().TextColor(t.TextMuted)
			ui.Text(c, TimerStatusLabel(record.Status)).FontSize(t.Rem(.72)).TextColor(t.TextMuted)
			if task.Timepoint != "" {
				ui.Text(c, task.Timepoint).FontSize(t.Rem(.72)).TextColor(t.TextMuted)
			}
			headline := timerHeadline(display, phases)
			ui.Text(c, headline).FontSize(t.Rem(.85)).FontWeight(600).TextColor(t.Text).FontFeatures("tnum")
			if display.Over {
				ui.Text(c, "超时").FontSize(t.Rem(.68)).Bold().TextColor(tokens.DangerText)
			}
			if display.ClockIssue {
				ui.Text(c, "时钟锚点缺失").FontSize(t.Rem(.68)).Bold().TextColor(tokens.WarningText)
			}
		})
		if record.Status == "completed" {
			return
		}
		if s.Callbacks.TimerAction == nil {
			return
		}
		ui.Row(c).FillWidth().Wrap().AlignItems(ui.Center).Gap(t.Space(1)).Children(func() {
			// Countdowns relay rounds through advance; windows and stopwatches
			// only start and pause. The action set follows the stored mode so
			// an invalid transition is never offered.
			if record.Status != "running" {
				startLabel := "开始"
				if record.Status == "paused" {
					startLabel = "继续"
				}
				if start := components.Button(c, startLabel, components.ButtonSecondary); start.Clicked() && !busy {
					s.runTimerAction(task, record, phases, "start")
				}
			} else {
				if pause := components.Button(c, "暂停", components.ButtonSecondary); pause.Clicked() && !busy {
					s.runTimerAction(task, record, phases, "pause")
				}
				if record.Mode == "countdown" {
					if advance := components.Button(c, "推进一轮", components.ButtonSecondary); advance.Clicked() && !busy {
						s.runTimerAction(task, record, phases, "advance")
					}
				}
			}
			if busy {
				ui.Text(c, "提交中…").FontSize(t.Rem(.72)).TextColor(t.TextMuted)
			}
		})
		if msg := bench.TimersErr[task.TaskKey]; msg != "" && busy {
			ui.Text(c, "提交失败："+msg).FontSize(t.Rem(.72)).TextColor(tokens.DangerText)
		}
	})
}

func renderTimerCreate(c *ui.Context, s *State, task *RunTaskViewModel, existing int, t *ui.Theme, tokens theme.Tokens) {
	if s.Callbacks.SaveTimer == nil {
		return
	}
	bench := s.bench()
	form := bench.timerForm(task.TaskKey)
	modes := []struct{ value, label string }{
		{"countdown", "倒计时"},
		{"stopwatch", "正计时"},
		{"window", "时间窗"},
	}

	ui.Row(c).FillWidth().Wrap().AlignItems(ui.Center).Gap(t.Space(1)).Children(func() {
		ui.Text(c, "新建").FontSize(t.Rem(.72)).Bold().TextColor(t.TextMuted)
		for _, mode := range modes {
			mode := mode
			label := mode.label
			if form.Mode == mode.value {
				label = "● " + label
			}
			if components.Button(c, label, components.ButtonGhost).Clicked() {
				form.Mode = mode.value
			}
		}
		if form.Mode != "stopwatch" {
			ui.Text(c, "分钟").FontSize(t.Rem(.72)).TextColor(t.TextMuted)
			ui.TextInput(c, &form.Minutes).Width(64).Placeholder("如 30")
		} else {
			ui.Text(c, "分组秒表不设时长").FontSize(t.Rem(.72)).TextColor(t.TextMuted)
		}
		ui.Text(c, "分组").FontSize(t.Rem(.72)).TextColor(t.TextMuted)
		ui.TextInput(c, &form.Group).Width(64).Placeholder("可选，如 5%")
		if create := components.Button(c, "创建计时器", components.ButtonSecondary); create.Clicked() && !bench.CreateBusy {
			s.createTimer(task, form, existing)
		}
		if bench.CreateBusy {
			ui.Text(c, "创建中…").FontSize(t.Rem(.72)).TextColor(t.TextMuted)
		}
	})
}

// maxPlannedSeconds is the largest whole-second span a time.Duration can
// hold. WindowLine renders the t0 target as started_at + planned, so a
// larger plan would overflow the clock math; nothing on the bench needs
// more than ~292 years.
const maxPlannedSeconds = float64(math.MaxInt64 / int64(time.Second))

// parseMinutes parses the operator's duration input strictly: the whole
// string must be one finite number (strconv rejects "5mins" where Sscanf
// accepted a numeric prefix). Fractional minutes truncate toward zero, as
// before; a sub-second result and anything outside the representable clock
// range is an error the surface reports. No duration is ever defaulted.
func parseMinutes(raw string) (uint64, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return 0, fmt.Errorf("请填写分钟数")
	}
	minutes, err := strconv.ParseFloat(trimmed, 64)
	if err != nil || math.IsNaN(minutes) || math.IsInf(minutes, 0) {
		return 0, fmt.Errorf("分钟数必须是纯数字（如 30 或 1.5）")
	}
	seconds := minutes * 60
	if math.IsNaN(seconds) || math.IsInf(seconds, 0) || seconds < 1 || seconds > maxPlannedSeconds {
		return 0, fmt.Errorf("分钟数超出可计时范围（至少 1 秒）")
	}
	return uint64(seconds), nil
}
