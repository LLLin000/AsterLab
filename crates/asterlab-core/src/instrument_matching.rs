//! K2 draft instrument profile and matching contract.
//!
//! Pure Core logic: no profiles are imported, no Run values are written, and
//! no Slint or CLI contract is changed. Matching returns an explicit status
//! with reasons; it never returns a bare boolean.

use crate::operation_module::{
    CapabilityConstraint, EvidenceRef, InstrumentRequirement, ReviewStatus,
};
use serde::{Deserialize, Serialize};
use std::collections::HashSet;

pub const INSTRUMENT_PROFILE_SCHEMA_VERSION: u32 = 1;

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[serde(deny_unknown_fields)]
pub struct InstrumentProfile {
    pub schema_version: u32,
    pub profile_id: String,
    pub version: u32,
    pub class: String,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub model: Option<String>,
    #[serde(default)]
    pub capabilities: Vec<InstrumentCapability>,
    #[serde(default)]
    pub container_kinds: Vec<String>,
    pub source: ProfileSource,
    #[serde(default)]
    pub evidence: Vec<EvidenceRef>,
    pub review_status: ReviewStatus,
}

#[derive(Debug, Clone, Copy, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "snake_case")]
pub enum ProfileSource {
    BuiltIn,
    LabImported,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[serde(deny_unknown_fields)]
pub struct InstrumentCapability {
    pub key: String,
    pub value: CapabilityValue,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub unit: Option<String>,
    #[serde(default)]
    pub conditions: Vec<String>,
    #[serde(default)]
    pub evidence: Vec<EvidenceRef>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[serde(tag = "kind", rename_all = "snake_case")]
pub enum CapabilityValue {
    Exists,
    Unsupported,
    Number { value: f64 },
    Range { min: f64, max: f64 },
    Text { value: String },
    Set { values: Vec<String> },
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[serde(deny_unknown_fields)]
pub struct ProfileRef {
    pub profile_id: String,
    pub version: u32,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[serde(deny_unknown_fields)]
pub struct ContainerRequirement {
    pub kind: String,
}

#[derive(Debug, Clone, Copy, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "snake_case")]
pub enum MatchStatus {
    Compatible,
    Unknown,
    Incompatible,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[serde(deny_unknown_fields)]
pub struct MatchResult {
    pub status: MatchStatus,
    pub reasons: Vec<MatchReason>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[serde(deny_unknown_fields)]
pub struct MatchReason {
    pub code: String,
    pub path: String,
    pub message: String,
    pub source: MatchSource,
}

#[derive(Debug, Clone, Copy, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "snake_case")]
pub enum MatchSource {
    Requirement,
    Profile,
    ProfileAbsent,
    UnitUnknown,
    Evidence,
    Container,
}

impl InstrumentProfile {
    pub fn validate(&self) -> Result<(), Vec<ContractIssue>> {
        let mut issues = Vec::new();
        if self.schema_version != INSTRUMENT_PROFILE_SCHEMA_VERSION {
            issues.push(issue(
                "schema_version",
                "schema_version",
                "unsupported instrument-profile schema version",
            ));
        }
        validate_id(&self.profile_id, "profile_id", &mut issues);
        if self.version == 0 {
            issues.push(issue("version", "version", "version must be >= 1"));
        }
        if self.class.trim().is_empty() {
            issues.push(issue("class", "class", "class must not be empty"));
        }

        let mut capability_keys = HashSet::new();
        for (index, capability) in self.capabilities.iter().enumerate() {
            let path = format!("capabilities[{index}]");
            validate_id(&capability.key, &format!("{path}.key"), &mut issues);
            if !capability_keys.insert(capability.key.as_str()) {
                issues.push(issue(
                    "duplicate_capability",
                    format!("{path}.key"),
                    "capability keys must be unique",
                ));
            }
            validate_capability_value(&capability.value, &format!("{path}.value"), &mut issues);
            if capability
                .unit
                .as_deref()
                .is_some_and(|unit| unit.trim().is_empty())
            {
                issues.push(issue(
                    "unit_empty",
                    format!("{path}.unit"),
                    "unit must not be empty",
                ));
            }
            if capability.unit.is_some()
                && !matches!(
                    capability.value,
                    CapabilityValue::Number { .. } | CapabilityValue::Range { .. }
                )
            {
                issues.push(issue(
                    "unit_on_non_numeric_capability",
                    format!("{path}.unit"),
                    "only numeric capability values may declare a unit",
                ));
            }
            for (evidence_index, evidence) in capability.evidence.iter().enumerate() {
                if evidence.citation.trim().is_empty() {
                    issues.push(issue(
                        "evidence_citation",
                        format!("{path}.evidence[{evidence_index}].citation"),
                        "evidence citation must not be empty",
                    ));
                }
            }
        }

        let mut container_kinds = HashSet::new();
        for (index, kind) in self.container_kinds.iter().enumerate() {
            if kind.trim().is_empty() {
                issues.push(issue(
                    "container_kind",
                    format!("container_kinds[{index}]"),
                    "container kind must not be empty",
                ));
            }
            if !container_kinds.insert(kind.as_str()) {
                issues.push(issue(
                    "duplicate_container_kind",
                    format!("container_kinds[{index}]"),
                    "container kinds must be unique",
                ));
            }
        }

        if self.review_status != ReviewStatus::ResearchDraft && self.evidence.is_empty() {
            issues.push(issue(
                "evidence_required",
                "evidence",
                "source_checked or higher requires at least one evidence reference",
            ));
        }
        for (index, evidence) in self.evidence.iter().enumerate() {
            if evidence.citation.trim().is_empty() {
                issues.push(issue(
                    "evidence_citation",
                    format!("evidence[{index}].citation"),
                    "evidence citation must not be empty",
                ));
            }
        }

        finish(issues)
    }
}

/// Match one operation requirement against at most one profile.
///
/// `requested` is the exact profile referenced by the requirement; a version
/// mismatch is Unknown, not a guessed substitution.
pub fn match_requirement(
    requirement: &InstrumentRequirement,
    profile: Option<&InstrumentProfile>,
    requested: Option<&ProfileRef>,
    container: Option<&ContainerRequirement>,
) -> MatchResult {
    let mut reasons = Vec::new();
    let mut unknown = false;
    let mut incompatible = false;

    let Some(profile) = profile else {
        return MatchResult {
            status: MatchStatus::Unknown,
            reasons: vec![reason(
                "profile_missing",
                "profile",
                "no instrument profile is available",
                MatchSource::ProfileAbsent,
            )],
        };
    };

    if profile.validate().is_err() {
        return MatchResult {
            status: MatchStatus::Unknown,
            reasons: vec![reason(
                "profile_invalid",
                "profile",
                "instrument profile is structurally invalid",
                MatchSource::Profile,
            )],
        };
    }

    let source_checked =
        profile.review_status.at_least(ReviewStatus::SourceChecked) && !profile.evidence.is_empty();
    if !source_checked {
        unknown = true;
        reasons.push(reason(
            "profile_not_source_checked",
            "review_status",
            "profile is not source-checked; capability claims are not confirmed",
            MatchSource::Evidence,
        ));
    }

    // Check class before exact version: an explicit class conflict must not be
    // hidden behind a version mismatch. If the profile is not source-checked,
    // even that conflict is capped at Unknown.
    if profile.class != requirement.class {
        if source_checked {
            return MatchResult {
                status: MatchStatus::Incompatible,
                reasons: vec![reason(
                    "class_mismatch",
                    "class",
                    "instrument class does not satisfy the operation requirement",
                    MatchSource::Profile,
                )],
            };
        }
        reasons.push(reason(
            "class_mismatch_unverified",
            "class",
            "class differs, but the profile is not source-checked",
            MatchSource::Profile,
        ));
        return MatchResult {
            status: MatchStatus::Unknown,
            reasons,
        };
    }

    if requirement.profile_version.is_some() && requirement.profile_id.is_none() {
        return MatchResult {
            status: MatchStatus::Unknown,
            reasons: vec![reason(
                "profile_version_missing",
                "profile_id",
                "profile version was supplied without a profile id",
                MatchSource::Profile,
            )],
        };
    }

    if let Some(profile_id) = &requirement.profile_id {
        let Some(version) = requirement.profile_version else {
            return MatchResult {
                status: MatchStatus::Unknown,
                reasons: vec![reason(
                    "profile_version_missing",
                    "profile_version",
                    "exact profile reference is missing its version",
                    MatchSource::Profile,
                )],
            };
        };
        if profile.profile_id != *profile_id || profile.version != version {
            return MatchResult {
                status: MatchStatus::Unknown,
                reasons: vec![reason(
                    "profile_version_mismatch",
                    "profile",
                    "requirement profile does not equal the available profile",
                    MatchSource::Profile,
                )],
            };
        }
    }

    if let Some(requested) = requested {
        if requested.profile_id != profile.profile_id || requested.version != profile.version {
            return MatchResult {
                status: MatchStatus::Unknown,
                reasons: vec![reason(
                    "profile_version_mismatch",
                    "profile",
                    "requested profile version does not equal the available profile",
                    MatchSource::Profile,
                )],
            };
        }
    }

    if let Some(container) = container {
        if profile.container_kinds.is_empty() {
            unknown = true;
            reasons.push(reason(
                "container_unknown",
                "container_kinds",
                "profile does not record supported container kinds",
                MatchSource::Container,
            ));
        } else if !profile
            .container_kinds
            .iter()
            .any(|kind| kind == &container.kind)
        {
            incompatible = true;
            reasons.push(reason(
                "container_mismatch",
                "container_kinds",
                "profile explicitly does not list the requested container kind",
                MatchSource::Container,
            ));
        }
    }

    for (index, requirement_capability) in requirement.capabilities.iter().enumerate() {
        let path = format!("capabilities[{index}]");
        let Some(profile_capability) = profile
            .capabilities
            .iter()
            .find(|capability| capability.key == requirement_capability.key)
        else {
            unknown = true;
            reasons.push(reason(
                "capability_missing",
                &path,
                "profile does not record this capability",
                MatchSource::Profile,
            ));
            continue;
        };
        if profile_capability.evidence.is_empty() {
            unknown = true;
            reasons.push(reason(
                "capability_evidence_missing",
                &path,
                "matched capability lacks located evidence",
                MatchSource::Evidence,
            ));
            continue;
        }
        match match_capability(&requirement_capability.constraint, profile_capability) {
            CapabilityMatch::Pass => reasons.push(reason(
                "capability_matched",
                &path,
                "profile capability satisfies the requirement",
                MatchSource::Profile,
            )),
            CapabilityMatch::Unknown(code, message) => {
                unknown = true;
                let source = if code == "unit_conversion_unknown" {
                    MatchSource::UnitUnknown
                } else {
                    MatchSource::Profile
                };
                reasons.push(reason(code, &path, message, source));
            }
            CapabilityMatch::Incompatible(code, message) => {
                incompatible = true;
                reasons.push(reason(code, &path, message, MatchSource::Profile));
            }
        }
    }

    let status = if !source_checked {
        MatchStatus::Unknown
    } else if incompatible {
        MatchStatus::Incompatible
    } else if unknown {
        MatchStatus::Unknown
    } else {
        MatchStatus::Compatible
    };
    if reasons.is_empty() {
        reasons.push(reason(
            "profile_matched",
            "profile",
            "profile satisfies the known requirements",
            MatchSource::Profile,
        ));
    }
    MatchResult { status, reasons }
}

enum CapabilityMatch {
    Pass,
    Unknown(&'static str, &'static str),
    Incompatible(&'static str, &'static str),
}

fn match_capability(
    requirement: &CapabilityConstraint,
    profile: &InstrumentCapability,
) -> CapabilityMatch {
    if profile.value == CapabilityValue::Unsupported {
        return CapabilityMatch::Incompatible(
            "capability_unsupported",
            "profile explicitly declares this capability unsupported",
        );
    }
    match requirement {
        CapabilityConstraint::Exists => CapabilityMatch::Pass,
        CapabilityConstraint::Min { value, unit } => {
            match_numeric_min(*value, unit.as_deref(), profile)
        }
        CapabilityConstraint::Max { value, unit } => {
            match_numeric_max(*value, unit.as_deref(), profile)
        }
        CapabilityConstraint::Range { min, max, unit } => {
            match_numeric_range(*min, *max, unit.as_deref(), profile)
        }
        CapabilityConstraint::OneOf { values } => match_one_of(values, profile),
    }
}

fn match_numeric_min(
    value: f64,
    unit: Option<&str>,
    profile: &InstrumentCapability,
) -> CapabilityMatch {
    if !units_match(unit, profile.unit.as_deref()) {
        return CapabilityMatch::Unknown(
            "unit_conversion_unknown",
            "requirement and profile units differ; no automatic conversion is allowed",
        );
    }
    match profile.value {
        CapabilityValue::Number { value: actual } if actual >= value => CapabilityMatch::Pass,
        CapabilityValue::Number { .. } => CapabilityMatch::Incompatible(
            "capability_below_minimum",
            "profile numeric capability is below the requested minimum",
        ),
        CapabilityValue::Range { max, .. } if max >= value => CapabilityMatch::Pass,
        CapabilityValue::Range { .. } => CapabilityMatch::Incompatible(
            "capability_below_minimum",
            "profile range cannot reach the requested minimum",
        ),
        _ => CapabilityMatch::Unknown(
            "capability_value_unknown",
            "profile does not provide a comparable numeric capability",
        ),
    }
}

fn match_numeric_max(
    value: f64,
    unit: Option<&str>,
    profile: &InstrumentCapability,
) -> CapabilityMatch {
    if !units_match(unit, profile.unit.as_deref()) {
        return CapabilityMatch::Unknown(
            "unit_conversion_unknown",
            "requirement and profile units differ; no automatic conversion is allowed",
        );
    }
    match profile.value {
        CapabilityValue::Number { value: actual } if actual <= value => CapabilityMatch::Pass,
        CapabilityValue::Number { .. } => CapabilityMatch::Incompatible(
            "capability_above_maximum",
            "profile numeric capability is above the requested maximum",
        ),
        CapabilityValue::Range { min, .. } if min <= value => CapabilityMatch::Pass,
        CapabilityValue::Range { .. } => CapabilityMatch::Incompatible(
            "capability_above_maximum",
            "profile range starts above the requested maximum",
        ),
        _ => CapabilityMatch::Unknown(
            "capability_value_unknown",
            "profile does not provide a comparable numeric capability",
        ),
    }
}

fn match_numeric_range(
    min: f64,
    max: f64,
    unit: Option<&str>,
    profile: &InstrumentCapability,
) -> CapabilityMatch {
    if !units_match(unit, profile.unit.as_deref()) {
        return CapabilityMatch::Unknown(
            "unit_conversion_unknown",
            "requirement and profile units differ; no automatic conversion is allowed",
        );
    }
    match profile.value {
        CapabilityValue::Range {
            min: profile_min,
            max: profile_max,
        } if profile_min <= min && profile_max >= max => CapabilityMatch::Pass,
        CapabilityValue::Range { .. } => CapabilityMatch::Incompatible(
            "capability_range_mismatch",
            "profile range does not contain the requested range",
        ),
        CapabilityValue::Number { value } if value >= min && value <= max => CapabilityMatch::Pass,
        CapabilityValue::Number { .. } => CapabilityMatch::Incompatible(
            "capability_range_mismatch",
            "profile numeric value is outside the requested range",
        ),
        _ => CapabilityMatch::Unknown(
            "capability_value_unknown",
            "profile does not provide a comparable numeric capability",
        ),
    }
}

fn match_one_of(values: &[String], profile: &InstrumentCapability) -> CapabilityMatch {
    match &profile.value {
        CapabilityValue::Set {
            values: profile_values,
        } if profile_values
            .iter()
            .any(|profile_value| values.iter().any(|value| value == profile_value)) =>
        {
            CapabilityMatch::Pass
        }
        CapabilityValue::Set { .. } => CapabilityMatch::Incompatible(
            "capability_value_mismatch",
            "profile set has no intersection with the requested values",
        ),
        CapabilityValue::Text { value } if values.iter().any(|requested| requested == value) => {
            CapabilityMatch::Pass
        }
        CapabilityValue::Text { .. } => CapabilityMatch::Incompatible(
            "capability_value_mismatch",
            "profile text value is not one of the requested values",
        ),
        _ => CapabilityMatch::Unknown(
            "capability_value_unknown",
            "profile does not provide a comparable value set",
        ),
    }
}

fn units_match(requirement: Option<&str>, profile: Option<&str>) -> bool {
    requirement == profile
}

fn validate_capability_value(value: &CapabilityValue, path: &str, issues: &mut Vec<ContractIssue>) {
    match value {
        CapabilityValue::Number { value } => {
            if !value.is_finite() {
                issues.push(issue(
                    "non_finite_capability",
                    path,
                    "capability value must be finite",
                ));
            }
        }
        CapabilityValue::Range { min, max } => {
            if !min.is_finite() || !max.is_finite() {
                issues.push(issue(
                    "non_finite_capability",
                    path,
                    "capability bounds must be finite",
                ));
            }
            if min > max {
                issues.push(issue(
                    "capability_range",
                    path,
                    "capability minimum exceeds maximum",
                ));
            }
        }
        CapabilityValue::Set { values } => {
            if values.is_empty() {
                issues.push(issue(
                    "capability_set_empty",
                    path,
                    "capability set must not be empty",
                ));
            }
        }
        CapabilityValue::Text { value } => {
            if value.trim().is_empty() {
                issues.push(issue(
                    "capability_text_empty",
                    path,
                    "capability text must not be empty",
                ));
            }
        }
        CapabilityValue::Exists | CapabilityValue::Unsupported => {}
    }
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub struct ContractIssue {
    pub code: String,
    pub path: String,
    pub message: String,
}

fn validate_id(value: &str, path: &str, issues: &mut Vec<ContractIssue>) {
    let mut chars = value.chars();
    let first = chars.next();
    let ok = first.is_some_and(|c| c.is_ascii_lowercase())
        && chars.all(|c| {
            c.is_ascii_lowercase() || c.is_ascii_digit() || c == '_' || c == '.' || c == '-'
        })
        && value.len() <= 120;
    if !ok {
        issues.push(issue(
            "stable_id",
            path,
            "ID must start lowercase and contain only lowercase letters, digits, '.', '_' or '-'",
        ));
    }
}

fn reason(
    code: impl Into<String>,
    path: impl Into<String>,
    message: impl Into<String>,
    source: MatchSource,
) -> MatchReason {
    MatchReason {
        code: code.into(),
        path: path.into(),
        message: message.into(),
        source,
    }
}

fn issue(
    code: impl Into<String>,
    path: impl Into<String>,
    message: impl Into<String>,
) -> ContractIssue {
    ContractIssue {
        code: code.into(),
        path: path.into(),
        message: message.into(),
    }
}

fn finish(issues: Vec<ContractIssue>) -> Result<(), Vec<ContractIssue>> {
    if issues.is_empty() {
        Ok(())
    } else {
        Err(issues)
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::operation_module::{CapabilityRequirement, EvidenceKind};

    fn evidence() -> EvidenceRef {
        EvidenceRef {
            evidence_id: "spec".into(),
            kind: EvidenceKind::VendorManual,
            citation: "synthetic spec".into(),
            url: None,
            locator: None,
            version: None,
            retrieved_at: None,
        }
    }

    fn profile() -> InstrumentProfile {
        InstrumentProfile {
            schema_version: INSTRUMENT_PROFILE_SCHEMA_VERSION,
            profile_id: "plate_reader.example".into(),
            version: 1,
            class: "plate_reader".into(),
            model: Some("Example".into()),
            capabilities: vec![
                InstrumentCapability {
                    key: "wavelength".into(),
                    value: CapabilityValue::Range {
                        min: 400.0,
                        max: 700.0,
                    },
                    unit: Some("nm".into()),
                    conditions: vec![],
                    evidence: vec![evidence()],
                },
                InstrumentCapability {
                    key: "plate_format".into(),
                    value: CapabilityValue::Set {
                        values: vec!["96".into(), "384".into()],
                    },
                    unit: None,
                    conditions: vec![],
                    evidence: vec![evidence()],
                },
            ],
            container_kinds: vec!["96-well".into()],
            source: ProfileSource::BuiltIn,
            evidence: vec![evidence()],
            review_status: ReviewStatus::SourceChecked,
        }
    }

    fn requirement() -> InstrumentRequirement {
        InstrumentRequirement {
            requirement_id: "read_absorbance".into(),
            class: "plate_reader".into(),
            profile_id: None,
            profile_version: None,
            required: true,
            capabilities: vec![
                CapabilityRequirement {
                    key: "wavelength".into(),
                    constraint: CapabilityConstraint::Range {
                        min: 450.0,
                        max: 600.0,
                        unit: Some("nm".into()),
                    },
                },
                CapabilityRequirement {
                    key: "plate_format".into(),
                    constraint: CapabilityConstraint::OneOf {
                        values: vec!["96".into()],
                    },
                },
            ],
        }
    }

    #[test]
    fn matching_profile_is_compatible_with_reasons() {
        let result = match_requirement(&requirement(), Some(&profile()), None, None);
        assert_eq!(result.status, MatchStatus::Compatible);
        assert!(!result.reasons.is_empty());
    }

    #[test]
    fn compatible_result_always_has_a_reason() {
        let mut requirement = requirement();
        requirement.capabilities.clear();
        let result = match_requirement(&requirement, Some(&profile()), None, None);
        assert_eq!(result.status, MatchStatus::Compatible);
        assert!(!result.reasons.is_empty());
    }

    #[test]
    fn missing_profile_is_unknown_not_compatible() {
        let result = match_requirement(&requirement(), None, None, None);
        assert_eq!(result.status, MatchStatus::Unknown);
        assert_eq!(result.reasons[0].code, "profile_missing");
    }

    #[test]
    fn class_mismatch_wins_over_version_mismatch() {
        let mut requirement = requirement();
        requirement.profile_id = Some("plate_reader.example".into());
        requirement.profile_version = Some(2);
        let mut profile = profile();
        profile.class = "microscope".into();
        let result = match_requirement(&requirement, Some(&profile), None, None);
        assert_eq!(result.status, MatchStatus::Incompatible);
        assert_eq!(result.reasons[0].code, "class_mismatch");
    }

    #[test]
    fn capability_without_evidence_is_unknown() {
        let mut profile = profile();
        profile.capabilities[0].evidence.clear();
        let result = match_requirement(&requirement(), Some(&profile), None, None);
        assert_eq!(result.status, MatchStatus::Unknown);
        assert!(result
            .reasons
            .iter()
            .any(|reason| reason.code == "capability_evidence_missing"));
    }

    #[test]
    fn profile_version_without_id_is_unknown() {
        let mut requirement = requirement();
        requirement.profile_version = Some(1);
        let result = match_requirement(&requirement, Some(&profile()), None, None);
        assert_eq!(result.status, MatchStatus::Unknown);
        assert_eq!(result.reasons[0].code, "profile_version_missing");
    }

    #[test]
    fn class_mismatch_is_incompatible() {
        let mut profile = profile();
        profile.class = "microscope".into();
        let result = match_requirement(&requirement(), Some(&profile), None, None);
        assert_eq!(result.status, MatchStatus::Incompatible);
    }

    #[test]
    fn missing_capability_is_unknown() {
        let mut profile = profile();
        profile
            .capabilities
            .retain(|capability| capability.key != "wavelength");
        let result = match_requirement(&requirement(), Some(&profile), None, None);
        assert_eq!(result.status, MatchStatus::Unknown);
        assert!(result
            .reasons
            .iter()
            .any(|reason| reason.code == "capability_missing"));
    }

    #[test]
    fn unit_mismatch_is_unknown_not_incompatible() {
        let mut profile = profile();
        profile.capabilities[0].unit = Some("mL".into());
        let result = match_requirement(&requirement(), Some(&profile), None, None);
        assert_eq!(result.status, MatchStatus::Unknown);
        assert!(result
            .reasons
            .iter()
            .any(|reason| reason.code == "unit_conversion_unknown"));
    }

    #[test]
    fn disjoint_range_is_incompatible() {
        let mut profile = profile();
        profile.capabilities[0].value = CapabilityValue::Range {
            min: 500.0,
            max: 600.0,
        };
        let result = match_requirement(&requirement(), Some(&profile), None, None);
        assert_eq!(result.status, MatchStatus::Incompatible);
    }

    #[test]
    fn explicit_requirement_profile_is_enforced_without_request_argument() {
        let mut requirement = requirement();
        requirement.profile_id = Some("plate_reader.example".into());
        requirement.profile_version = Some(2);
        let result = match_requirement(&requirement, Some(&profile()), None, None);
        assert_eq!(result.status, MatchStatus::Unknown);
        assert_eq!(result.reasons[0].code, "profile_version_mismatch");
    }

    #[test]
    fn invalid_profile_is_unknown() {
        let mut profile = profile();
        profile.schema_version = 99;
        let result = match_requirement(&requirement(), Some(&profile), None, None);
        assert_eq!(result.status, MatchStatus::Unknown);
        assert_eq!(result.reasons[0].code, "profile_invalid");
    }

    #[test]
    fn profile_version_mismatch_is_unknown() {
        let requested = ProfileRef {
            profile_id: "plate_reader.example".into(),
            version: 2,
        };
        let result = match_requirement(&requirement(), Some(&profile()), Some(&requested), None);
        assert_eq!(result.status, MatchStatus::Unknown);
        assert_eq!(result.reasons[0].code, "profile_version_mismatch");
    }

    #[test]
    fn unverified_profile_caps_incompatible_at_unknown() {
        let mut profile = profile();
        profile.review_status = ReviewStatus::ResearchDraft;
        profile.capabilities[0].value = CapabilityValue::Unsupported;
        profile.capabilities[0].unit = None;
        let result = match_requirement(&requirement(), Some(&profile), None, None);
        assert_eq!(result.status, MatchStatus::Unknown);
        assert!(result
            .reasons
            .iter()
            .any(|reason| reason.code == "profile_not_source_checked"));
    }

    #[test]
    fn research_draft_profile_cannot_be_compatible() {
        let mut profile = profile();
        profile.review_status = ReviewStatus::ResearchDraft;
        let result = match_requirement(&requirement(), Some(&profile), None, None);
        assert_eq!(result.status, MatchStatus::Unknown);
        assert!(result
            .reasons
            .iter()
            .any(|reason| reason.code == "profile_not_source_checked"));
    }

    #[test]
    fn unsupported_capability_is_incompatible() {
        let mut profile = profile();
        profile.capabilities[0].value = CapabilityValue::Unsupported;
        profile.capabilities[0].unit = None;
        let result = match_requirement(&requirement(), Some(&profile), None, None);
        assert_eq!(result.status, MatchStatus::Incompatible);
    }

    #[test]
    fn unknown_container_is_unknown_and_conflict_is_incompatible() {
        let mut profile = profile();
        profile.container_kinds.clear();
        let unknown = match_requirement(
            &requirement(),
            Some(&profile),
            None,
            Some(&ContainerRequirement {
                kind: "96-well".into(),
            }),
        );
        assert_eq!(unknown.status, MatchStatus::Unknown);
        profile.container_kinds = vec!["24-well".into()];
        let conflict = match_requirement(
            &requirement(),
            Some(&profile),
            None,
            Some(&ContainerRequirement {
                kind: "96-well".into(),
            }),
        );
        assert_eq!(conflict.status, MatchStatus::Incompatible);
    }

    #[test]
    fn profile_validation_rejects_empty_unit() {
        let mut profile = profile();
        profile.capabilities[0].unit = Some("".into());
        let errors = profile.validate().unwrap_err();
        assert!(errors.iter().any(|issue| issue.code == "unit_empty"));
    }

    #[test]
    fn profile_validation_rejects_invalid_range_and_duplicate_capability() {
        let mut profile = profile();
        profile.capabilities[0].value = CapabilityValue::Range {
            min: 700.0,
            max: 400.0,
        };
        profile.capabilities.push(profile.capabilities[0].clone());
        let errors = profile.validate().unwrap_err();
        assert!(errors.iter().any(|issue| issue.code == "capability_range"));
        assert!(errors
            .iter()
            .any(|issue| issue.code == "duplicate_capability"));
    }
}
