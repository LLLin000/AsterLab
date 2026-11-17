package runworkspace

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"asterlab-mygo-prototype/internal/notebook/record"
	"asterlab-mygo-prototype/internal/theme"
	"asterlab-mygo-prototype/modules/runs"

	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/onevra/source/components"
)

// timerForm holds the create-timer draft per task. Group is the optional
// operator scope (e.g. 5%): same-task same-mode timers are told apart by it.
type timerForm struct {
	Mode    string
	Minutes string
	Group   string
}

// eventForm holds the composer draft per task.
type eventForm struct {
	Kind       string
	Text       string
	Pantry     string
	PantryOpen bool
	Busy       bool
	Err        string
	Done       string
}

// BenchState is the run bench: persisted timers, the skip/deviation/
// observation composer, the capture strip and the blind flag. It lives on
// State so it survives view-model reloads; Core stays the only fact store.
type BenchState struct {
	// Timers caches the last ListTimers checkpoint per task.
	Timers map[string][]runs.TimerRecord
	// TimersErr carries the last load/action failure per task (UX-P5).
	TimersErr map[string]string
	// TimerBusy names the timer with an in-flight action.
	TimerBusy string
	// CreateBusy guards the create row.
	CreateBusy bool
	// timerForms holds the create drafts per task.
	timerForms map[string]*timerForm
	// eventForms holds the composer drafts per task.
	eventForms map[string]*eventForm
	// exclusionForms holds the sample-exclusion drafts per task.
	exclusionForms map[string]*exclusionForm
	// readoutForms holds the categorical/table readout drafts per task.
	readoutForms map[string]*typedReadoutForm
	// Pantry caches the last ListPantry ledger for the composer.
	Pantry       *runs.PantryLedger
	PantryLoaded bool
	PantryBusy   bool
	PantryErr    string
	// Blind masks group identity in the run capture panels.
	Blind bool
	// BatchErr/BatchDone report the last batch-link outcome (UX-P5).
	BatchErr  string
	BatchDone string
	// timersKey remembers which run+revision the cache belongs to.
	timersKey string
	// timersAt is the last successful load instant.
	timersAt time.Time
	// historyKey/historyProj/historyErr memoize the shared Core record
	// projection by the host's opaque key; the projection itself lives in
	// the notebook/record leaf package, never here.
	historyKey  string
	historyProj *record.Projection
	historyErr  string
}

func (s *State) bench() *BenchState {
	if s == nil {
		return &BenchState{}
	}
	if s.Bench == nil {
		s.Bench = &BenchState{}
	}
	if s.Bench.Timers == nil {
		s.Bench.Timers = map[string][]runs.TimerRecord{}
	}
	if s.Bench.TimersErr == nil {
		s.Bench.TimersErr = map[string]string{}
	}
	if s.Bench.timerForms == nil {
		s.Bench.timerForms = map[string]*timerForm{}
	}
	if s.Bench.eventForms == nil {
		s.Bench.eventForms = map[string]*eventForm{}
	}
	return s.Bench
}

func (b *BenchState) timerForm(taskKey string) *timerForm {
	form, ok := b.timerForms[taskKey]
	if !ok {
		form = &timerForm{Mode: "countdown"}
		b.timerForms[taskKey] = form
	}
	return form
}

func (b *BenchState) eventForm(taskKey string) *eventForm {
	form, ok := b.eventForms[taskKey]
	if !ok {
		form = &eventForm{}
		b.eventForms[taskKey] = form
	}
	return form
}

// phasesFor resolves the relay plan for a task. Nil means the timer runs its
// own single phase (Core TimerSequence.from_record semantics).
func (s *State) phasesFor(taskKey string) []runs.TimerPhase {
	if s == nil || s.Callbacks.TimerPhases == nil {
		return nil
	}
	return s.Callbacks.TimerPhases(taskKey)
}

// benchKey identifies the timer cache: run plus Core revision.
func (s *State) benchKey() string {
	if s == nil || s.VM == nil {
		return ""
	}
	return fmt.Sprintf("%s|%d", s.VM.RunID, s.VM.Revision)
}

// RefreshTimers reloads the current run's timers when the run changed, the
// revision moved, or the callback asks. It returns true when a load ran.
func (s *State) RefreshTimers(force bool, maxAge time.Duration) bool {
	if s == nil || s.Callbacks.LoadTimers == nil || s.VM == nil {
		return false
	}
	bench := s.bench()
	key := s.benchKey()
	stale := bench.timersKey != key || force
	if !stale && maxAge > 0 && time.Since(bench.timersAt) > maxAge {
		stale = true
	}
	if !stale {
		return false
	}
	timers, err := s.Callbacks.LoadTimers(s.VM.RunID)
	// A failed attempt still records the key and instant: the run page
	// polls every frame, and a persistent Core failure must back off to
	// maxAge instead of retrying ListTimers on every frame. Forcing or a
	// new key/expired age retries.
	bench.timersKey = key
	bench.timersAt = time.Now()
	if err != nil {
		bench.TimersErr[s.Selected] = err.Error()
		return true
	}
	byTask := map[string][]runs.TimerRecord{}
	for _, timer := range timers {
		byTask[timer.TaskKey] = append(byTask[timer.TaskKey], timer)
	}
	bench.Timers = byTask
	bench.TimersErr = map[string]string{}
	bench.timersKey = key
	bench.timersAt = time.Now()
	return true
}

// refreshTimers reloads after a local action on one task.
func (s *State) refreshTimers(taskKey string) {
	if s == nil || s.Callbacks.LoadTimers == nil || s.VM == nil {
		return
	}
	timers, err := s.Callbacks.LoadTimers(s.VM.RunID)
	bench := s.bench()
	if err != nil {
		bench.TimersErr[taskKey] = timerActionError(err)
		return
	}
	byTask := map[string][]runs.TimerRecord{}
	for _, timer := range timers {
		byTask[timer.TaskKey] = append(byTask[timer.TaskKey], timer)
	}
	bench.Timers = byTask
	delete(bench.TimersErr, taskKey)
	bench.timersKey = s.benchKey()
	bench.timersAt = time.Now()
}

// runTimerAction applies one lifecycle action at the caller's clock, stores
// the result, and files a completed round as an observation event. The caller
// supplies the instant so the transition is deterministic.
func (s *State) runTimerAction(task *RunTaskViewModel, record runs.TimerRecord, phases []runs.TimerPhase, action string) {
	if s == nil || task == nil || s.Callbacks.TimerAction == nil {
		return
	}
	bench := s.bench()
	bench.TimerBusy = record.TimerID
	bench.TimersErr[task.TaskKey] = ""
	result, err := s.Callbacks.TimerAction(runs.TimerActionInput{
		Timer:  record,
		Action: action,
		NowUTC: time.Now().UTC().Format(time.RFC3339),
		Phases: phases,
	})
	bench.TimerBusy = ""
	if err != nil {
		bench.TimersErr[task.TaskKey] = timerActionError(err)
		return
	}
	if result.Completed && s.Callbacks.AppendEvent != nil {
		if _, err := s.Callbacks.AppendEvent(task.TaskKey, RoundEventInput(task, result.Round)); err != nil {
			bench.TimersErr[task.TaskKey] = eventSubmitError(err)
		}
	}
	s.refreshTimers(task.TaskKey)
}

// createTimer persists one timer row through SaveTimer. The id is
// deterministic per task/mode/sequence with the optional operator group as
// scope (task/mode/seq or task/mode/seq@group). Core's store is an upsert,
// so the panel refuses to save over an existing id instead of silently
// resetting its accumulated progress, and every refusal names its cause:
// creation never fails silently (QA-HUVEC F1, UX-P5).
func (s *State) createTimer(task *RunTaskViewModel, form *timerForm, existing int) {
	if s == nil || task == nil || form == nil {
		return
	}
	bench := s.bench()
	if s.Callbacks.SaveTimer == nil {
		bench.TimersErr[task.TaskKey] = "计时服务未接线，无法创建计时器"
		return
	}
	bench.CreateBusy = true
	defer func() { bench.CreateBusy = false }()
	key := task.TaskKey
	if key == "" {
		bench.TimersErr[""] = "未选中步骤"
		return
	}
	mode := strings.TrimSpace(form.Mode)
	if mode != "countdown" && mode != "stopwatch" && mode != "window" {
		bench.TimersErr[key] = "请先选择计时类型（倒计时/正计时/时间窗）"
		return
	}
	group := strings.TrimSpace(form.Group)
	group = strings.ReplaceAll(group, "/", "-")
	group = strings.ReplaceAll(group, "@", "-")
	timerID := fmt.Sprintf("%s/%s/%d", key, mode, existing+1)
	if group != "" {
		timerID += "@" + group
	}
	for _, known := range bench.Timers[key] {
		if known.TimerID == timerID {
			bench.TimersErr[key] = "同名计时器已存在（" + TimerIdentity(known) + ")，请刷新后使用"
			return
		}
	}
	record := runs.TimerRecord{
		TimerID: timerID,
		TaskKey: key,
		Mode:    mode,
		Status:  "ready",
	}
	if mode != "stopwatch" {
		planned, err := parseMinutes(form.Minutes)
		if err != nil {
			bench.TimersErr[key] = err.Error()
			return
		}
		record.PlannedS = &planned
		if mode == "window" {
			record.AnchorTaskKey = &key
		}
	}
	if _, err := s.Callbacks.SaveTimer(record); err != nil {
		bench.TimersErr[key] = timerActionError(err)
		return
	}
	form.Minutes = ""
	form.Group = ""
	bench.TimersErr[key] = ""
	s.refreshTimers(key)
}

// renderBench draws the run bench above the recording panel: timers, the
// skip/deviation/observation composer, and the capture strip. It stays out of
// the way when no callback is wired: every section is optional.
func renderBench(c *ui.Context, s *State, task *RunTaskViewModel, t *ui.Theme, tokens theme.Tokens) {
	if task == nil || task.Status == TaskStatusCompleted {
		return
	}
	hasTimers := s.Callbacks.LoadTimers != nil
	hasEvents := s.Callbacks.AppendEvent != nil
	// hasChannel gates the Bridge v2 composers (sample exclusion and
	// categorical/table readout entry); without it both sections stay
	// hidden and the bench keeps its old shape.
	hasChannel := s.Callbacks.ExecuteCommand != nil
	wells, timepoint, fov := capturePlan(s, task)
	hasCapture := len(wells) > 0 && fov > 0
	if !hasTimers && !hasEvents && !hasCapture && !hasChannel {
		return
	}
	ui.Column(c).FillWidth().Padding(float32(tokens.Spacing16)).Radius(float32(tokens.PanelRadius)).
		Background(tokens.Surface).Border(1, tokens.Border).Gap(t.Space(2.5)).Children(func() {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(t.Space(1.5)).Children(func() {
			ui.Text(c, "执行台面").FontSize(float32(tokens.TextSection)).FontWeight(600).TextColor(t.Text)
			ui.Spacer(c)
			renderBlindToggle(c, s, t, tokens)
		})
		if hasTimers {
			renderTimerSection(c, s, task, t, tokens)
		}
		if hasCapture {
			renderCaptureStrip(c, s, task, wells, timepoint, fov, t, tokens)
		}
		if hasEvents {
			renderEventComposer(c, s, task, t, tokens)
		}
		// UX-P1, UX-P4, UX-STATE: exclusion and typed readout entry live in
		// the bench as their own sections with one write action each; Core
		// owns all validation and every failure names cause + retry.
		if hasChannel {
			renderExclusionSection(c, s, task, t, tokens)
			renderTypedReadoutSection(c, s, task, t, tokens)
		}
	})
}

// capturePlan resolves the planned wells, timepoint and FOV count for the
// capture strip. Wells come from the host's coordinate options (the frozen
// plate), the expected FOV count from the frozen measurement profile.
func capturePlan(s *State, task *RunTaskViewModel) (wells []string, timepoint string, fovPerWell int) {
	if s == nil || task == nil || s.Callbacks.CaptureWells == nil {
		return nil, "", 0
	}
	wells, fovPerWell = s.Callbacks.CaptureWells(task.TaskKey)
	return wells, strings.TrimSpace(task.Timepoint), fovPerWell
}

// renderBlindToggle flips the run blinding mask through the same Core
// confirmation as the Run top-bar switch: entering blind consumes the
// run.workspace blind projection, leaving blind unmasks locally. Masked
// capture panels hide group identity; the authoritative blind projection
// stays in Record. UX-P3, UX-STATE, UX-DESIGN.
func renderBlindToggle(c *ui.Context, s *State, t *ui.Theme, tokens theme.Tokens) {
	bench := s.bench()
	if components.Button(c, blindSwitchLabel(bench.Blind), components.ButtonGhost).Clicked() {
		s.toggleBlind()
	}
	if bench.Blind {
		ui.Text(c, "分组已隐藏").FontSize(t.Rem(.68)).TextColor(t.TextMuted)
	}
}

// NoteBatchResult records the last batch-link outcome for the capture strip
// (UX-P5). The host calls it after linking; empty strings clear the notice.
func (s *State) NoteBatchResult(done, errMsg string) {
	bench := s.bench()
	bench.BatchDone, bench.BatchErr = done, errMsg
}

// TimersFingerprint hashes the cached timer checkpoints so the host can
// repaint only when a poll actually moved time.
func (s *State) TimersFingerprint() string {
	bench := s.bench()
	keys := make([]string, 0, len(bench.Timers))
	for key := range bench.Timers {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var out strings.Builder
	for _, key := range keys {
		for _, timer := range bench.Timers[key] {
			started := ""
			if timer.StartedAtUTC != nil {
				started = *timer.StartedAtUTC
			}
			planned := ""
			if timer.PlannedS != nil {
				planned = strconv.FormatUint(*timer.PlannedS, 10)
			}
			fmt.Fprintf(&out, "%s|%s|%s|%d|%s|%d|%d;", timer.TimerID, timer.Status,
				started, timer.AccumulatedS, planned, timer.PhaseIndex, timer.RoundIndex)
		}
	}
	return out.String()
}
