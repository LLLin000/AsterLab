//! Lightweight pantry (储藏室) ledger model.
//!
//! The ledger is deliberately optional and non-blocking: Design never requires
//! a pantry item, and Run only *may* reference one. This module owns the parts
//! that must be identical for the CLI, the desktop app and any future bridge:
//! field validation, calendar arithmetic, expiry status and the stable short
//! label code handed to external label/printer plugins.
//!
//! It performs no I/O and holds no database handle.

use serde::{Deserialize, Serialize};

/// Default window (in days) inside which an item is reported as `near_expiry`.
pub const NEAR_EXPIRY_DAYS: u32 = 14;
/// Longest accepted shelf-life window, in days (ten years).
pub const MAX_SHELF_LIFE_DAYS: u32 = 3650;
/// Longest accepted name/location length, in characters.
pub const MAX_TEXT_CHARS: usize = 200;
/// Shortest accepted calendar year; keeps label/date typos out of the ledger.
pub const MIN_YEAR: i64 = 1900;
/// Longest accepted calendar year.
pub const MAX_YEAR: i64 = 2999;

/// A calendar date without time or timezone. Pantry dates are human labels on a
/// package, never instants, so the ledger never stores a time of day.
#[derive(Debug, Clone, Copy, PartialEq, Eq, PartialOrd, Ord, Serialize, Deserialize)]
pub struct CalendarDate {
    pub year: i64,
    pub month: u32,
    pub day: u32,
}

impl CalendarDate {
    /// Parse strict `YYYY-MM-DD`. Anything looser is rejected: a silently
    /// accepted `2026-1-1` or `2026-02-30` would change an expiry status.
    pub fn parse(raw: &str) -> Option<CalendarDate> {
        let bytes = raw.as_bytes();
        if bytes.len() != 10 || bytes[4] != b'-' || bytes[7] != b'-' {
            return None;
        }
        let digits = |range: std::ops::Range<usize>| -> Option<i64> {
            let mut value = 0i64;
            for index in range {
                let byte = bytes[index];
                if !byte.is_ascii_digit() {
                    return None;
                }
                value = value * 10 + i64::from(byte - b'0');
            }
            Some(value)
        };
        let year = digits(0..4)?;
        let month = digits(5..7)?;
        let day = digits(8..10)?;
        if !(MIN_YEAR..=MAX_YEAR).contains(&year) {
            return None;
        }
        if !(1..=12).contains(&month) {
            return None;
        }
        let month = month as u32;
        if day < 1 || day > i64::from(days_in_month(year, month)) {
            return None;
        }
        Some(CalendarDate {
            year,
            month,
            day: day as u32,
        })
    }

    pub fn to_iso(self) -> String {
        format!("{:04}-{:02}-{:02}", self.year, self.month, self.day)
    }

    /// Days since 1970-01-01 (Howard Hinnant's civil calendar algorithm), so
    /// day windows never depend on a timezone database or a `chrono` version.
    pub fn day_number(self) -> i64 {
        let year = if self.month <= 2 { self.year - 1 } else { self.year };
        let era = if year >= 0 { year } else { year - 399 } / 400;
        let year_of_era = year - era * 400;
        let shifted_month = (i64::from(self.month) + 9) % 12;
        let day_of_year = (153 * shifted_month + 2) / 5 + i64::from(self.day) - 1;
        let day_of_era =
            year_of_era * 365 + year_of_era / 4 - year_of_era / 100 + day_of_year;
        era * 146_097 + day_of_era - 719_468
    }

    pub fn from_day_number(days: i64) -> CalendarDate {
        let shifted = days + 719_468;
        let era = if shifted >= 0 { shifted } else { shifted - 146_096 } / 146_097;
        let day_of_era = shifted - era * 146_097;
        let year_of_era = (day_of_era - day_of_era / 1460 + day_of_era / 36_524
            - day_of_era / 146_096)
            / 365;
        let year = year_of_era + era * 400;
        let day_of_year = day_of_era - (365 * year_of_era + year_of_era / 4 - year_of_era / 100);
        let shifted_month = (5 * day_of_year + 2) / 153;
        let day = day_of_year - (153 * shifted_month + 2) / 5 + 1;
        let month = if shifted_month < 10 {
            shifted_month + 3
        } else {
            shifted_month - 9
        };
        let year = if month <= 2 { year + 1 } else { year };
        CalendarDate {
            year,
            month: month as u32,
            day: day as u32,
        }
    }

    pub fn checked_add_days(self, days: i64) -> Option<CalendarDate> {
        let result = CalendarDate::from_day_number(self.day_number().checked_add(days)?);
        if (MIN_YEAR..=MAX_YEAR).contains(&result.year) {
            Some(result)
        } else {
            None
        }
    }
}

fn days_in_month(year: i64, month: u32) -> u32 {
    match month {
        1 | 3 | 5 | 7 | 8 | 10 | 12 => 31,
        4 | 6 | 9 | 11 => 30,
        2 if is_leap_year(year) => 29,
        2 => 28,
        _ => 0,
    }
}

fn is_leap_year(year: i64) -> bool {
    (year % 4 == 0 && year % 100 != 0) || year % 400 == 0
}

/// Everything the operator may change about an item. Kept separate from
/// [`PantryItem`] so validation can run before an id or revision exists.
#[derive(Debug, Clone, Default, PartialEq, Eq, Serialize, Deserialize)]
pub struct PantryDraft {
    pub name: String,
    #[serde(default)]
    pub location: String,
    #[serde(default)]
    pub purchased_on: Option<String>,
    #[serde(default)]
    pub opened_on: Option<String>,
    #[serde(default)]
    pub shelf_life_days: Option<u32>,
    #[serde(default)]
    pub opened_shelf_life_days: Option<u32>,
    #[serde(default)]
    pub exhausted: bool,
}

impl PantryDraft {
    /// Reject values that would make the expiry status a guess. Every message
    /// names the offending field so the CLI can return it as `INVALID_INPUT`.
    pub fn validate(&self) -> Result<(), String> {
        let name = self.name.trim();
        if name.is_empty() || name.chars().count() > MAX_TEXT_CHARS {
            return Err(format!(
                "储藏室条目名称不能为空且不能超过 {MAX_TEXT_CHARS} 个字符"
            ));
        }
        if self.location.trim().chars().count() > MAX_TEXT_CHARS {
            return Err(format!(
                "储藏室条目位置不能超过 {MAX_TEXT_CHARS} 个字符"
            ));
        }
        let purchased = match self.purchased_on.as_deref() {
            Some(raw) => Some(
                CalendarDate::parse(raw)
                    .ok_or_else(|| "购入日必须是 YYYY-MM-DD 的真实日期".to_owned())?,
            ),
            None => None,
        };
        let opened = match self.opened_on.as_deref() {
            Some(raw) => Some(
                CalendarDate::parse(raw)
                    .ok_or_else(|| "开封日必须是 YYYY-MM-DD 的真实日期".to_owned())?,
            ),
            None => None,
        };
        if let (Some(purchased), Some(opened)) = (purchased, opened) {
            if opened < purchased {
                return Err("开封日不能早于购入日".to_owned());
            }
        }
        for (value, label) in [
            (self.shelf_life_days, "未开封保质期"),
            (self.opened_shelf_life_days, "开封后保质期"),
        ] {
            if let Some(days) = value {
                if days == 0 || days > MAX_SHELF_LIFE_DAYS {
                    return Err(format!(
                        "{label}必须是 1–{MAX_SHELF_LIFE_DAYS} 天"
                    ));
                }
            }
        }
        Ok(())
    }
}

/// One ledger row. `draft` is flattened so the JSON shape stays flat.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct PantryItem {
    pub id: String,
    pub label_code: String,
    pub project_id: Option<String>,
    pub revision: i64,
    #[serde(flatten)]
    pub draft: PantryDraft,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum PantryStatus {
    /// Nothing recorded as opened yet.
    Unopened,
    /// Opened, and inside its usable window.
    Opened,
    /// At or inside the near-expiry window; still usable, but flagged.
    NearExpiry,
    /// Past the computed expiry date.
    Expired,
    /// Consumed; dates no longer drive the status.
    Exhausted,
}

/// Which date rule produced `expires_on`.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum ExpiryBasis {
    /// `purchased_on + shelf_life_days`.
    ShelfLife,
    /// `opened_on + opened_shelf_life_days` — preferred once opened.
    OpenedShelfLife,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct PantryEvaluation {
    pub status: PantryStatus,
    pub expires_on: Option<String>,
    /// Days from `as_of` to `expires_on`; negative means overdue.
    pub days_remaining: Option<i64>,
    pub basis: Option<ExpiryBasis>,
}

impl PantryEvaluation {
    /// True when the item deserves an operator-visible expiry notice.
    pub fn needs_notice(&self) -> bool {
        matches!(self.status, PantryStatus::NearExpiry | PantryStatus::Expired)
    }
}

/// Compute status for one item as of a caller-supplied date. `as_of` is an
/// argument, never `now`, so every caller and test sees the same answer.
pub fn evaluate(item: &PantryItem, as_of: CalendarDate, near_expiry_days: u32) -> PantryEvaluation {
    let expiry = expiry_date(&item.draft);
    let (expires_on, days_remaining) = match expiry {
        Some((date, _)) => (
            Some(date.to_iso()),
            Some(date.day_number() - as_of.day_number()),
        ),
        None => (None, None),
    };
    let basis = expiry.map(|(_, basis)| basis);
    let status = if item.draft.exhausted {
        PantryStatus::Exhausted
    } else {
        match days_remaining {
            Some(remaining) if remaining < 0 => PantryStatus::Expired,
            Some(remaining) if remaining <= i64::from(near_expiry_days) => PantryStatus::NearExpiry,
            _ => {
                if item.draft.opened_on.is_some() {
                    PantryStatus::Opened
                } else {
                    PantryStatus::Unopened
                }
            }
        }
    };
    PantryEvaluation {
        status,
        expires_on,
        days_remaining,
        basis,
    }
}

fn expiry_date(draft: &PantryDraft) -> Option<(CalendarDate, ExpiryBasis)> {
    if let (Some(opened_on), Some(window)) = (draft.opened_on.as_deref(), draft.opened_shelf_life_days) {
        if let (Some(opened), Ok(window)) = (
            CalendarDate::parse(opened_on),
            i64::try_from(window),
        ) {
            if let Some(date) = opened.checked_add_days(window) {
                return Some((date, ExpiryBasis::OpenedShelfLife));
            }
        }
    }
    let purchased_on = draft.purchased_on.as_deref()?;
    let window = draft.shelf_life_days?;
    let purchased = CalendarDate::parse(purchased_on)?;
    let date = purchased.checked_add_days(i64::try_from(window).ok()?)?;
    Some((date, ExpiryBasis::ShelfLife))
}

/// Stable short code for labels and scanners. Derived only from the immutable
/// item id, so reprinting a label never changes what a scanner reads.
/// Collisions are resolved by the caller appending a numeric suffix once.
pub fn base_label_code(id: &str) -> String {
    let mut chars: Vec<char> = id
        .chars()
        .filter(|c| c.is_ascii_alphanumeric())
        .map(|c| c.to_ascii_uppercase())
        .collect();
    chars.resize(8, '0');
    chars.truncate(8);
    format!("PN-{}", chars.into_iter().collect::<String>())
}

/// `base` for the first holder, then `base-2`, `base-3`, … for later ones.
pub fn label_code_with_suffix(base: &str, sequence: u32) -> String {
    if sequence <= 1 {
        base.to_owned()
    } else {
        format!("{base}-{sequence}")
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn date(raw: &str) -> CalendarDate {
        CalendarDate::parse(raw).unwrap_or_else(|| panic!("{raw} should parse"))
    }

    fn draft(purchased: Option<&str>, opened: Option<&str>) -> PantryDraft {
        PantryDraft {
            name: "Trypsin".to_owned(),
            location: "4C door".to_owned(),
            purchased_on: purchased.map(str::to_owned),
            opened_on: opened.map(str::to_owned),
            shelf_life_days: Some(365),
            opened_shelf_life_days: Some(30),
            exhausted: false,
        }
    }

    fn item(d: PantryDraft) -> PantryItem {
        PantryItem {
            id: "6f1c2d3e-4a5b-6c7d-8e9f-001122334455".to_owned(),
            label_code: base_label_code("6f1c2d3e-4a5b-6c7d-8e9f-001122334455"),
            project_id: None,
            revision: 1,
            draft: d,
        }
    }

    #[test]
    fn rejects_malformed_and_impossible_dates() {
        assert!(CalendarDate::parse("2026-02-30").is_none());
        assert!(CalendarDate::parse("2026-13-01").is_none());
        assert!(CalendarDate::parse("2026-1-1").is_none());
        assert!(CalendarDate::parse("2026-01-01T00:00:00Z").is_none());
        assert!(CalendarDate::parse("").is_none());
        assert_eq!(date("2024-02-29").to_iso(), "2024-02-29");
        assert!(CalendarDate::parse("2023-02-29").is_none());
    }

    #[test]
    fn calendar_arithmetic_crosses_month_and_leap_boundaries() {
        assert_eq!(date("1970-01-01").day_number(), 0);
        assert_eq!(date("1970-01-02").day_number(), 1);
        assert_eq!(date("2024-02-28").checked_add_days(1), Some(date("2024-02-29")));
        assert_eq!(date("2024-02-29").checked_add_days(1), Some(date("2024-03-01")));
        assert_eq!(date("2023-02-28").checked_add_days(1), Some(date("2023-03-01")));
        assert_eq!(date("2026-11-15").checked_add_days(-15), Some(date("2026-10-31")));
        assert_eq!(date("2026-12-31").checked_add_days(1), Some(date("2027-01-01")));
        let roundtrip = CalendarDate::from_day_number(date("2026-11-15").day_number());
        assert_eq!(roundtrip.to_iso(), "2026-11-15");
    }

    #[test]
    fn unopened_items_report_purchase_based_expiry() {
        let item = item(draft(Some("2026-01-01"), None));
        let evaluation = evaluate(&item, date("2026-06-01"), NEAR_EXPIRY_DAYS);
        assert_eq!(evaluation.status, PantryStatus::Unopened);
        assert_eq!(evaluation.expires_on.as_deref(), Some("2027-01-01"));
        assert_eq!(evaluation.days_remaining, Some(214));
        assert_eq!(evaluation.basis, Some(ExpiryBasis::ShelfLife));
    }

    #[test]
    fn opened_items_use_the_opened_window_over_the_purchase_window() {
        let item = item(draft(Some("2026-01-01"), Some("2026-06-01")));
        let evaluation = evaluate(&item, date("2026-06-10"), NEAR_EXPIRY_DAYS);
        assert_eq!(evaluation.status, PantryStatus::Opened);
        assert_eq!(evaluation.expires_on.as_deref(), Some("2026-07-01"));
        assert_eq!(evaluation.days_remaining, Some(21));
        assert_eq!(evaluation.basis, Some(ExpiryBasis::OpenedShelfLife));
    }

    #[test]
    fn near_expiry_threshold_is_inclusive_and_expiry_has_its_own_status() {
        let item = item(draft(Some("2026-01-01"), Some("2026-06-01")));
        // 30 day opened window from 2026-06-01 → expires 2026-07-01.
        let at_threshold = evaluate(&item, date("2026-06-01"), 30);
        assert_eq!(at_threshold.days_remaining, Some(30));
        assert_eq!(at_threshold.status, PantryStatus::NearExpiry);
        let just_outside = evaluate(&item, date("2026-06-01"), 29);
        assert_eq!(just_outside.status, PantryStatus::Opened);
        let last_day = evaluate(&item, date("2026-07-01"), 5);
        assert_eq!(last_day.days_remaining, Some(0));
        assert_eq!(last_day.status, PantryStatus::NearExpiry);
        let overdue = evaluate(&item, date("2026-07-02"), 5);
        assert_eq!(overdue.days_remaining, Some(-1));
        assert_eq!(overdue.status, PantryStatus::Expired);
    }

    #[test]
    fn exhausted_wins_over_every_date_rule() {
        let mut d = draft(Some("2026-01-01"), Some("2026-06-01"));
        d.exhausted = true;
        let evaluation = evaluate(&item(d), date("2026-07-02"), 5);
        assert_eq!(evaluation.status, PantryStatus::Exhausted);
        assert_eq!(evaluation.expires_on.as_deref(), Some("2026-07-01"));
    }

    #[test]
    fn items_without_dates_stay_usable_without_an_expiry_claim() {
        let mut d = draft(None, None);
        d.shelf_life_days = None;
        d.opened_shelf_life_days = None;
        let evaluation = evaluate(&item(d.clone()), date("2026-06-01"), 5);
        assert_eq!(evaluation.status, PantryStatus::Unopened);
        assert_eq!(evaluation.expires_on, None);
        assert_eq!(evaluation.days_remaining, None);
        assert_eq!(evaluation.basis, None);

        d.opened_on = Some("2026-06-01".to_owned());
        let opened = evaluate(&item(d), date("2026-06-01"), 5);
        assert_eq!(opened.status, PantryStatus::Opened);
        assert_eq!(opened.expires_on, None);
    }

    #[test]
    fn validation_rejects_dates_and_windows_that_cannot_be_true() {
        let mut d = draft(Some("2026-06-01"), Some("2026-05-01"));
        assert!(d.validate().unwrap_err().contains("开封日"));

        d = draft(Some("2026-13-01"), None);
        assert!(d.validate().unwrap_err().contains("购入日"));

        d = draft(Some("2026-06-01"), Some("2026-06-02"));
        d.shelf_life_days = Some(0);
        assert!(d.validate().unwrap_err().contains("保质期"));

        d = draft(Some("2026-06-01"), None);
        d.opened_shelf_life_days = Some(9999);
        assert!(d.validate().unwrap_err().contains("开封后"));

        d = draft(None, None);
        d.name = "  ".to_owned();
        assert!(d.validate().unwrap_err().contains("名称"));

        d = draft(None, None);
        d.location = "x".repeat(MAX_TEXT_CHARS + 1);
        assert!(d.validate().unwrap_err().contains("位置"));

        d = draft(Some("2026-06-01"), Some("2026-06-01"));
        assert!(d.validate().is_ok(), "same-day purchase and opening is valid");
    }

    #[test]
    fn label_code_is_derived_from_the_immutable_id_only() {
        let id = "6f1c2d3e-4a5b-6c7d-8e9f-001122334455";
        assert_eq!(base_label_code(id), "PN-6F1C2D3E");
        assert_eq!(base_label_code(id), base_label_code(id), "deterministic");
        assert_eq!(base_label_code("short"), "PN-SHORT000");
        // A re-read of the same item must not depend on mutable fields.
        let a = item(draft(Some("2026-01-01"), None));
        let mut b = a.clone();
        b.draft.location = "bench".to_owned();
        assert_eq!(a.label_code, b.label_code);
    }
}
