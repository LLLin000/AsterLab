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

    /// Advances a completed countdown cycle to the next numbered round.
    pub fn advance_countdown(&mut self, now_utc: &str) -> Result<bool, String> {
        if self.mode != TimerMode::Countdown || self.status != TimerStatus::Running {
            return Ok(false);
        }
        let Some(planned) = self.planned_s else { return Err("countdown requires planned_s".into()) };
        if self.elapsed_s(now_utc)? < planned {
            return Ok(false);
        }
        if self.round_count.is_some_and(|total| self.round_index + 1 >= total) {
            self.accumulated_s = planned;
            self.started_at_utc = None;
            self.status = TimerStatus::Completed;
        } else {
            self.round_index += 1;
            self.accumulated_s = 0;
            self.started_at_utc = Some(now_utc.to_owned());
        }
        Ok(true)
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
    if !(1..=12).contains(&month) || !(1..=31).contains(&day) || hour > 23 || minute > 59 || second > 60 {
        return Err("invalid timestamp".into());
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

    #[test]
    fn countdown_advances_round_only_after_planned_duration() {
        let mut timer = timer(TimerMode::Countdown);
        timer.start("2026-11-15T10:00:00Z").unwrap();
        assert!(!timer.advance_countdown("2026-11-15T10:00:09Z").unwrap());
        assert!(timer.advance_countdown("2026-11-15T10:00:10Z").unwrap());
        assert_eq!(timer.round_index, 1);
        assert_eq!(timer.started_at_utc.as_deref(), Some("2026-11-15T10:00:10Z"));
    }

    #[test]
    fn window_timer_starts_once_when_its_t0_event_arrives() {
        let mut timer = timer(TimerMode::Window);
        timer.anchor_task_key = Some("t0-task".into());
        assert!(!timer.trigger_anchor("other-event", Some("other-task"), "2026-11-15T10:00:00Z").unwrap());
        assert!(timer.trigger_anchor("generated-event", Some("t0-task"), "2026-11-15T10:00:00Z").unwrap());
    }
}
