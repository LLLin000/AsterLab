use serde::{Deserialize, Serialize};

#[derive(Debug, Clone, Copy, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "snake_case")]
pub enum TimerMode {
    Countdown,
    Stopwatch,
    Window,
}

#[derive(Debug, Clone, Copy, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "snake_case")]
pub enum TimerStatus {
    Ready,
    Running,
    Paused,
    Completed,
}

/// One stage of a timer's plan. `name` labels the stage, `planned_s` is its
/// countdown duration, `mode` is the stage's own mode, and `rounds` is how many
/// countdown rounds the stage runs; `rounds: None` means the stage repeats
/// until the caller stops the timer.
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "camelCase")]
pub struct TimerPhase {
    pub name: String,
    pub planned_s: Option<u64>,
    pub mode: TimerMode,
    #[serde(default)]
    pub rounds: Option<u32>,
}

/// The ordered phases a timer walks through.
///
/// [`TimerRecord`] stores only the cursor (`phase_index`, `round_index`); the
/// plan itself stays with the caller, like the clock does. A timer created
/// before phase sequences existed uses [`TimerSequence::from_record`], which
/// reproduces its single `planned_s`/`mode`/`round_count` exactly.
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq, Default)]
#[serde(rename_all = "camelCase")]
pub struct TimerSequence {
    pub phases: Vec<TimerPhase>,
}

impl TimerSequence {
    /// The one-phase plan an existing timer already implies.
    pub fn from_record(timer: &TimerRecord) -> Self {
        Self {
            phases: vec![TimerPhase {
                name: timer.task_key.clone(),
                planned_s: timer.planned_s,
                mode: timer.mode,
                rounds: timer.round_count,
            }],
        }
    }

    /// Validates an explicit plan: named phases, a countdown duration for every
    /// countdown phase, and at least one round when a phase bounds its rounds.
    pub fn new(phases: Vec<TimerPhase>) -> Result<Self, String> {
        if phases.is_empty() {
            return Err("a timer sequence needs at least one phase".into());
        }
        for (index, phase) in phases.iter().enumerate() {
            if phase.name.trim().is_empty() {
                return Err(format!("phase {index} needs a name"));
            }
            if phase.rounds == Some(0) {
                return Err(format!("phase {index} needs at least one round"));
            }
            if phase.mode == TimerMode::Countdown && phase.planned_s.is_none() {
                return Err(format!("phase {index} is a countdown and needs planned_s"));
            }
        }
        Ok(Self { phases })
    }

    pub fn phase(&self, index: u32) -> Result<&TimerPhase, String> {
        self.phases.get(index as usize).ok_or_else(|| {
            format!("phase_index {index} is outside the timer's {} phase(s)", self.phases.len())
        })
    }

    pub fn len(&self) -> usize {
        self.phases.len()
    }

    pub fn is_empty(&self) -> bool {
        self.phases.is_empty()
    }
}

/// One completed countdown round: what the timer planned, how long it actually
/// ran, and when. It is a returned fact, not a stored table — the caller
/// appends it through Core's execution-event entry, which is where the round
/// history is read back from.
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "camelCase")]
pub struct TimerRound {
    pub phase_index: u32,
    pub round_index: u32,
    pub phase_name: String,
    pub mode: TimerMode,
    pub planned_s: u64,
    pub actual_s: u64,
    /// The completed round's running anchor. Paused time is included in
    /// `actual_s`, never in the interval between this instant and
    /// `ended_at_utc`.
    pub started_at_utc: String,
    pub ended_at_utc: String,
    pub phase_completed: bool,
    pub timer_completed: bool,
}

/// Durable timer state. `started_at_utc` is the running interval's UTC anchor;
/// paused time is held in `accumulated_s` and is never inferred from process uptime.
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "camelCase")]
pub struct TimerRecord {
    pub timer_id: String,
    pub run_id: String,
    pub task_key: String,
    pub phase_index: u32,
    pub mode: TimerMode,
    pub planned_s: Option<u64>,
    pub started_at_utc: Option<String>,
    pub accumulated_s: u64,
    pub status: TimerStatus,
    #[serde(default)]
    pub round_index: u32,
    #[serde(default)]
    pub round_count: Option<u32>,
    #[serde(default)]
    pub anchor_event_id: Option<String>,
    #[serde(default)]
    pub anchor_task_key: Option<String>,
}

impl TimerRecord {
    pub fn elapsed_s(&self, now_utc: &str) -> Result<u64, String> {
        let running = if self.status == TimerStatus::Running {
            match self.started_at_utc.as_deref() {
                Some(started) => parse_utc_seconds(now_utc)?
                    .saturating_sub(parse_utc_seconds(started)?).max(0) as u64,
                None => return Err("running timer is missing started_at_utc".into()),
            }
        } else {
            0
        };
        Ok(self.accumulated_s.saturating_add(running))
    }

    pub fn remaining_s(&self, now_utc: &str) -> Result<Option<u64>, String> {
        self.planned_s
            .map(|planned| self.elapsed_s(now_utc).map(|elapsed| planned.saturating_sub(elapsed)))
            .transpose()
    }

    pub fn start(&mut self, now_utc: &str) -> Result<(), String> {
        parse_utc_seconds(now_utc)?;
        if matches!(self.status, TimerStatus::Running | TimerStatus::Completed) {
            return Err("timer cannot be started in its current status".into());
        }
        self.started_at_utc = Some(now_utc.to_owned());
        self.status = TimerStatus::Running;
        Ok(())
    }

    pub fn pause(&mut self, now_utc: &str) -> Result<(), String> {
        if self.status != TimerStatus::Running {
            return Err("only a running timer can be paused".into());
        }
        self.accumulated_s = self.elapsed_s(now_utc)?;
        self.started_at_utc = None;
        self.status = TimerStatus::Paused;
        Ok(())
    }

    /// Starts a window timer after its t0 event or task is recorded.
    pub fn trigger_anchor(&mut self, event_id: &str, task_key: Option<&str>, occurred_at_utc: &str) -> Result<bool, String> {
        let matches_anchor = self.anchor_event_id.as_deref() == Some(event_id)
            || task_key.is_some_and(|key| self.anchor_task_key.as_deref() == Some(key));
        if !matches_anchor || self.status != TimerStatus::Ready {
            return Ok(false);
        }
        parse_utc_seconds(occurred_at_utc)?;
        self.started_at_utc = Some(occurred_at_utc.to_owned());
        self.status = TimerStatus::Running;
        Ok(true)
    }

    /// Advances a countdown once its current round has run its planned time.
    ///
    /// `sequence` is the plan being run; a single-phase timer passes
    /// [`TimerSequence::from_record`] and keeps the exact previous behaviour.
    /// Every completed round is returned as a [`TimerRound`] fact, so the
    /// previous round's accumulation is never dropped silently. The record
    /// keeps `accumulated_s` as the *current* round's accumulation and resets
    /// only that round's runtime state; crossing into the next phase also moves
    /// `mode` and `planned_s` to that phase's values, which the read models
    /// report for the current cycle.
    pub fn advance_countdown(&mut self, sequence: &TimerSequence, now_utc: &str) -> Result<Option<TimerRound>, String> {
        if self.status != TimerStatus::Running {
            return Ok(None);
        }
        let phase = sequence.phase(self.phase_index)?.clone();
        if phase.mode != TimerMode::Countdown {
            return Ok(None);
        }
        let planned = phase.planned_s.or(self.planned_s).ok_or("countdown requires planned_s")?;
        parse_utc_seconds(now_utc)?;
        let actual = self.elapsed_s(now_utc)?;
        if actual < planned {
            return Ok(None);
        }
        let started_at_utc = self
            .started_at_utc
            .clone()
            .ok_or("running timer is missing started_at_utc")?;
        let phase_completed = phase.rounds.is_some_and(|total| self.round_index + 1 >= total);
        let timer_completed = phase_completed && self.phase_index as usize + 1 >= sequence.len();
        let round = TimerRound {
            phase_index: self.phase_index,
            round_index: self.round_index,
            phase_name: phase.name,
            mode: TimerMode::Countdown,
            planned_s: planned,
            actual_s: actual,
            started_at_utc,
            ended_at_utc: now_utc.to_owned(),
            phase_completed,
            timer_completed,
        };
        if timer_completed {
            self.accumulated_s = planned;
            self.started_at_utc = None;
            self.status = TimerStatus::Completed;
        } else if phase_completed {
            let next = sequence.phase(self.phase_index + 1)?.clone();
            self.phase_index += 1;
            self.round_index = 0;
            self.mode = next.mode;
            self.planned_s = next.planned_s;
            self.accumulated_s = 0;
            self.started_at_utc = Some(now_utc.to_owned());
        } else {
            self.round_index += 1;
            self.accumulated_s = 0;
            self.started_at_utc = Some(now_utc.to_owned());
        }
        Ok(Some(round))
    }
}

// Parse RFC3339 timestamps into Unix seconds without adding a dependency to Core.
fn parse_utc_seconds(value: &str) -> Result<i64, String> {
    let bytes = value.as_bytes();
    if bytes.len() < 20 || bytes.get(4) != Some(&b'-') || bytes.get(7) != Some(&b'-')
        || bytes.get(10) != Some(&b'T') || bytes.get(13) != Some(&b':') || bytes.get(16) != Some(&b':') {
        return Err("timestamp must be RFC 3339".into());
    }
    let number = |start: usize, end: usize| -> Result<i64, String> {
        value.get(start..end).ok_or("invalid timestamp")?.parse().map_err(|_| "invalid timestamp".into())
    };
    let (year, month, day) = (number(0,4)?, number(5,7)?, number(8,10)?);
    let (hour, minute, second) = (number(11,13)?, number(14,16)?, number(17,19)?);
    if !(1..=12).contains(&month) || day < 1 || hour > 23 || hour < 0
        || !(0..=59).contains(&minute) || !(0..=60).contains(&second)
    {
        return Err("invalid timestamp".into());
    }
    let leap = (year % 4 == 0 && year % 100 != 0) || year % 400 == 0;
    let month_days = match month {
        1 | 3 | 5 | 7 | 8 | 10 | 12 => 31,
        4 | 6 | 9 | 11 => 30,
        _ => if leap { 29 } else { 28 },
    };
    if day > month_days {
        return Err("invalid timestamp: day does not exist in that month".into());
    }
    let days = days_from_civil(year, month, day);
    let mut offset = 0i64;
    let rest = &value[19..];
    let zone_index = rest.find(|c| c == 'Z' || c == '+' || c == '-')
        .map(|index| index + 19)
        .ok_or("timestamp must have a UTC offset")?;
    let fraction = &value[19..zone_index];
    if !fraction.is_empty()
        && !(fraction.starts_with('.') && fraction[1..].bytes().all(|byte| byte.is_ascii_digit()))
    {
        return Err("invalid timestamp fraction".into());
    }
    let zone = &value[zone_index..];
    if zone != "Z" {
        let sign = match zone.as_bytes().first() {
            Some(b'+') => 1,
            Some(b'-') => -1,
            _ => return Err("timestamp must have a UTC offset".into()),
        };
        if zone.len() != 6 || zone.as_bytes().get(3) != Some(&b':') {
            return Err("invalid timestamp offset".into());
        }
        let oh: i64 = zone[1..3].parse().map_err(|_| "invalid timestamp offset")?;
        let om: i64 = zone[4..6].parse().map_err(|_| "invalid timestamp offset")?;
        if oh > 23 || om > 59 {
            return Err("invalid timestamp offset".into());
        }
        offset = sign * (oh * 3600 + om * 60);
    }
    Ok(days * 86_400 + hour * 3600 + minute * 60 + second - offset)
}

fn days_from_civil(year: i64, month: i64, day: i64) -> i64 {
    let year = year - i64::from(month <= 2);
    let era = year.div_euclid(400);
    let yoe = year - era * 400;
    let doy = (153 * (month + if month > 2 { -3 } else { 9 }) + 2) / 5 + day - 1;
    let doe = yoe * 365 + yoe / 4 - yoe / 100 + doy;
    era * 146097 + doe - 719468
}

#[cfg(test)]
mod tests {
    use super::*;

    fn timer(mode: TimerMode) -> TimerRecord {
        TimerRecord {
            timer_id: "timer-1".into(), run_id: "run-1".into(), task_key: "task-1".into(),
            phase_index: 0, mode, planned_s: Some(10), started_at_utc: None,
            accumulated_s: 0, status: TimerStatus::Ready, round_index: 0,
            round_count: Some(3), anchor_event_id: None, anchor_task_key: None,
        }
    }

    #[test]
    fn restores_running_time_after_process_restart_and_keeps_pause_accumulation() {
        let mut timer = timer(TimerMode::Stopwatch);
        timer.start("2026-11-15T10:00:00Z").unwrap();
        assert_eq!(timer.elapsed_s("2026-11-15T10:00:37Z").unwrap(), 37);
        timer.pause("2026-11-15T10:00:37Z").unwrap();
        assert_eq!(timer.elapsed_s("2026-11-16T10:00:00Z").unwrap(), 37);
        timer.start("2026-11-16T10:00:00Z").unwrap();
        assert_eq!(timer.elapsed_s("2026-11-16T10:00:05Z").unwrap(), 42);
    }

    fn at(seconds: u64) -> String {
        format!("2026-11-15T10:{:02}:{:02}Z", seconds / 60, seconds % 60)
    }

    fn phase(name: &str, planned_s: Option<u64>, mode: TimerMode, rounds: Option<u32>) -> TimerPhase {
        TimerPhase { name: name.into(), planned_s, mode, rounds }
    }

    #[test]
    fn countdown_advances_round_only_after_planned_duration() {
        let mut timer = timer(TimerMode::Countdown);
        let plan = TimerSequence::from_record(&timer);
        timer.start("2026-11-15T10:00:00Z").unwrap();
        assert!(timer.advance_countdown(&plan, "2026-11-15T10:00:09Z").unwrap().is_none());
        let round = timer
            .advance_countdown(&plan, "2026-11-15T10:00:10Z")
            .unwrap()
            .expect("the round completes at its planned duration");
        assert_eq!(round.phase_name, "task-1");
        assert_eq!((round.phase_index, round.round_index), (0, 0));
        assert_eq!((round.planned_s, round.actual_s), (10, 10));
        assert_eq!((round.phase_completed, round.timer_completed), (false, false));
        assert_eq!(timer.round_index, 1);
        assert_eq!(timer.started_at_utc.as_deref(), Some("2026-11-15T10:00:10Z"));
    }

    #[test]
    fn single_phase_timer_completes_after_its_round_count() {
        let mut timer = timer(TimerMode::Countdown);
        let plan = TimerSequence::from_record(&timer);
        timer.start("2026-11-15T10:00:00Z").unwrap();
        let mut rounds = Vec::new();
        for second in [10, 20, 30] {
            rounds.push(timer.advance_countdown(&plan, &at(second)).unwrap().expect("round"));
        }
        assert_eq!(
            rounds.iter().map(|round| (round.round_index, round.actual_s)).collect::<Vec<_>>(),
            vec![(0, 10), (1, 10), (2, 10)]
        );
        assert!(rounds[2].phase_completed && rounds[2].timer_completed);
        assert_eq!(timer.status, TimerStatus::Completed);
        assert_eq!(timer.accumulated_s, 10);
        assert!(timer.started_at_utc.is_none());
    }

    #[test]
    fn two_phases_of_three_rounds_report_each_round_in_order() {
        let mut timer = timer(TimerMode::Countdown);
        timer.round_count = None;
        timer.planned_s = Some(5);
        let plan = TimerSequence::new(vec![
            phase("stain", Some(5), TimerMode::Countdown, Some(3)),
            phase("wash", Some(20), TimerMode::Countdown, Some(3)),
        ])
        .expect("two named countdown phases");
        timer.start("2026-11-15T10:00:00Z").unwrap();

        let mut history = Vec::new();
        let mut second = 0;
        for _ in 0..3 {
            second += 5;
            history.push(timer.advance_countdown(&plan, &at(second)).unwrap().expect("stain round"));
        }
        assert_eq!(timer.phase_index, 1, "the stain phase is done");
        assert_eq!(timer.planned_s, Some(20), "the record carries the current phase's plan");
        for _ in 0..3 {
            second += 20;
            history.push(timer.advance_countdown(&plan, &at(second)).unwrap().expect("wash round"));
        }

        let order: Vec<(u32, u32, &str, u64, bool, bool)> = history
            .iter()
            .map(|round| {
                (
                    round.phase_index,
                    round.round_index,
                    round.phase_name.as_str(),
                    round.actual_s,
                    round.phase_completed,
                    round.timer_completed,
                )
            })
            .collect();
        assert_eq!(
            order,
            vec![
                (0, 0, "stain", 5, false, false),
                (0, 1, "stain", 5, false, false),
                (0, 2, "stain", 5, true, false),
                (1, 0, "wash", 20, false, false),
                (1, 1, "wash", 20, false, false),
                (1, 2, "wash", 20, true, true),
            ]
        );
        assert_eq!(timer.status, TimerStatus::Completed);
        assert_eq!(timer.accumulated_s, 20);
    }

    #[test]
    fn pause_and_resume_keeps_the_round_position_and_reports_the_paused_seconds() {
        let mut timer = timer(TimerMode::Countdown);
        let plan = TimerSequence::from_record(&timer);
        timer.start("2026-11-15T10:00:00Z").unwrap();
        timer.pause("2026-11-15T10:00:04Z").unwrap();
        assert_eq!(timer.accumulated_s, 4);
        assert!(timer.advance_countdown(&plan, "2026-11-15T10:00:04Z").unwrap().is_none());
        timer.start("2026-11-15T10:00:10Z").unwrap();
        let round = timer
            .advance_countdown(&plan, "2026-11-15T10:00:16Z")
            .unwrap()
            .expect("six running seconds complete the paused round");
        assert_eq!(round.actual_s, 10, "paused time stays in the round's actual seconds");
        assert_eq!(round.started_at_utc, "2026-11-15T10:00:10Z");
        assert_eq!(round.ended_at_utc, "2026-11-15T10:00:16Z");
        assert_eq!(timer.round_index, 1);
        assert_eq!(timer.status, TimerStatus::Running);
        assert_eq!(timer.accumulated_s, 0, "only the new round's runtime is reset");
    }

    #[test]
    fn a_sequence_rejects_unnamed_or_undated_countdown_phases() {
        assert!(TimerSequence::new(vec![]).is_err());
        assert!(TimerSequence::new(vec![phase(" ", Some(5), TimerMode::Countdown, Some(1))]).is_err());
        assert!(TimerSequence::new(vec![phase("stain", None, TimerMode::Countdown, Some(1))]).is_err());
        assert!(TimerSequence::new(vec![phase("stain", Some(5), TimerMode::Countdown, Some(0))]).is_err());
        assert!(TimerSequence::new(vec![phase("observe", None, TimerMode::Stopwatch, None)]).is_ok());
    }

    #[test]
    fn advance_rejects_a_cursor_outside_the_supplied_sequence() {
        let mut timer = timer(TimerMode::Countdown);
        let plan = TimerSequence::from_record(&timer);
        timer.start("2026-11-15T10:00:00Z").unwrap();
        timer.phase_index = 1;
        let error = timer.advance_countdown(&plan, "2026-11-15T10:00:10Z").unwrap_err();
        assert!(error.contains("phase_index 1"), "{error}");
    }

    #[test]
    fn non_countdown_phases_do_not_advance() {
        let mut timer = timer(TimerMode::Stopwatch);
        timer.planned_s = None;
        let plan = TimerSequence::new(vec![phase("observe", None, TimerMode::Stopwatch, None)]).unwrap();
        timer.start("2026-11-15T10:00:00Z").unwrap();
        assert!(timer.advance_countdown(&plan, "2026-11-15T11:00:00Z").unwrap().is_none());
        assert_eq!(timer.status, TimerStatus::Running);
    }

    #[test]
    fn rejects_calendar_dates_that_do_not_exist() {
        let mut timer = timer(TimerMode::Stopwatch);
        assert!(timer.start("2026-02-31T10:00:00Z").is_err());
        assert!(timer.start("2026-04-31T10:00:00Z").is_err());
        assert!(timer.start("2025-02-29T10:00:00Z").is_err());
        assert!(timer.start("2026-13-01T10:00:00Z").is_err());
        assert!(timer.start("2026-01-01T24:00:00Z").is_err());
        assert!(timer.start("2026-01-01T10:60:00Z").is_err());
        timer.start("2024-02-29T10:00:00Z").unwrap();
        assert_eq!(timer.elapsed_s("2024-02-29T10:00:01Z").unwrap(), 1);
    }

    #[test]
    fn window_timer_starts_once_when_its_t0_event_arrives() {
        let mut timer = timer(TimerMode::Window);
        timer.anchor_task_key = Some("t0-task".into());
        assert!(!timer.trigger_anchor("other-event", Some("other-task"), "2026-11-15T10:00:00Z").unwrap());
        assert!(timer.trigger_anchor("generated-event", Some("t0-task"), "2026-11-15T10:00:00Z").unwrap());
    }
}
