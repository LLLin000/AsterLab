//! Minimal scientific fact and constraint evaluator.
//!
//! Only explicit, testable rules live here. Missing facts stay unresolved; a
//! manufacturer recommendation is a warning boundary, not an automatic SOP.

use crate::labware::{LabwareProfile, ProfileFieldState, Quantity};
use serde::{Deserialize, Serialize};

#[derive(Debug, Clone, Copy, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "snake_case")]
pub enum AssessmentStatus {
    Valid,
    Warning,
    Blocked,
    Unresolved,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[serde(deny_unknown_fields)]
pub struct ScientificAssessment {
    pub status: AssessmentStatus,
    pub rule_id: String,
    pub subject_ref: String,
    pub observed: Option<Quantity>,
    pub limit: Option<String>,
    pub explanation: String,
    pub evidence: Vec<String>,
    pub affected_components: Vec<String>,
}

pub fn assess_working_volume(
    profile: &LabwareProfile,
    subject_ref: &str,
    volume: &Quantity,
) -> ScientificAssessment {
    let field = profile.fields.get("working_volume_range");
    let Some(field) = field else {
        return unresolved(
            subject_ref,
            "working_volume_missing",
            "profile has no working volume range",
        );
    };
    if field.state != ProfileFieldState::Known {
        return unresolved(
            subject_ref,
            "working_volume_unknown",
            "working volume range is not known",
        );
    }
    let Some(range) = &field.range else {
        return unresolved(
            subject_ref,
            "working_volume_unknown",
            "working volume range has no typed range",
        );
    };
    if range.min.unit != volume.unit || range.max.unit != volume.unit {
        return ScientificAssessment {
            status: AssessmentStatus::Blocked,
            rule_id: "working_volume_unit".into(),
            subject_ref: subject_ref.into(),
            observed: Some(volume.clone()),
            limit: Some(format!(
                "{}-{} {}",
                range.min.value, range.max.value, range.min.unit
            )),
            explanation: "working volume unit does not match the profile range".into(),
            evidence: vec![profile.profile.id.clone()],
            affected_components: vec![],
        };
    }
    if volume.value < range.min.value || volume.value > range.max.value {
        return ScientificAssessment {
            status: AssessmentStatus::Warning,
            rule_id: "working_volume_range".into(),
            subject_ref: subject_ref.into(),
            observed: Some(volume.clone()),
            limit: Some(format!(
                "{}-{} {}",
                range.min.value, range.max.value, range.min.unit
            )),
            explanation:
                "planned volume is outside the manufacturer reference range; review required".into(),
            evidence: vec![profile.profile.id.clone()],
            affected_components: vec![],
        };
    }
    ScientificAssessment {
        status: AssessmentStatus::Valid,
        rule_id: "working_volume_range".into(),
        subject_ref: subject_ref.into(),
        observed: Some(volume.clone()),
        limit: Some(format!(
            "{}-{} {}",
            range.min.value, range.max.value, range.min.unit
        )),
        explanation: "planned volume is within the recorded reference range".into(),
        evidence: vec![profile.profile.id.clone()],
        affected_components: vec![],
    }
}

pub fn assess_seeding_density(
    profile: &LabwareProfile,
    subject_ref: &str,
    density: &Quantity,
) -> ScientificAssessment {
    let Some(area) = profile.fields.get("growth_area_per_well") else {
        return unresolved(
            subject_ref,
            "growth_area_missing",
            "profile has no growth area per well",
        );
    };
    if area.state != ProfileFieldState::Known {
        return unresolved(
            subject_ref,
            "growth_area_unknown",
            "growth area is not known",
        );
    }
    let Some(area_value) = &area.quantity else {
        return unresolved(
            subject_ref,
            "growth_area_unknown",
            "growth area has no typed quantity",
        );
    };
    if density.unit != "cells/cm2" || area_value.unit != "cm2" {
        return ScientificAssessment {
            status: AssessmentStatus::Blocked,
            rule_id: "seeding_unit".into(),
            subject_ref: subject_ref.into(),
            observed: Some(density.clone()),
            limit: Some("cells/cm2 over cm2".into()),
            explanation: "seeding density or growth area has an incompatible unit".into(),
            evidence: vec![profile.profile.id.clone()],
            affected_components: vec![],
        };
    }
    let cells_per_well = area_value.value * density.value;
    ScientificAssessment {
        status: AssessmentStatus::Valid,
        rule_id: "seeding_cells_per_well".into(),
        subject_ref: subject_ref.into(),
        observed: Some(Quantity {
            value: cells_per_well,
            unit: "cells/well".into(),
        }),
        limit: None,
        explanation: "cells per well derived from recorded growth area and user density".into(),
        evidence: vec![profile.profile.id.clone()],
        affected_components: vec![],
    }
}

fn unresolved(subject_ref: &str, rule_id: &str, explanation: &str) -> ScientificAssessment {
    ScientificAssessment {
        status: AssessmentStatus::Unresolved,
        rule_id: rule_id.into(),
        subject_ref: subject_ref.into(),
        observed: None,
        limit: None,
        explanation: explanation.into(),
        evidence: vec![],
        affected_components: vec![],
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::labware::{LabwareFamily, LabwareProfile, ProfileField, ProfileRef};

    fn profile() -> LabwareProfile {
        LabwareProfile {
            profile: ProfileRef {
                id: "test".into(),
                version: 1,
            },
            family: LabwareFamily::CellCulturePlate,
            title: "test".into(),
            fields: std::collections::BTreeMap::from([
                (
                    "growth_area_per_well".into(),
                    ProfileField::known_quantity(0.32, "cm2"),
                ),
                (
                    "working_volume_range".into(),
                    ProfileField::known_range(100.0, 200.0, "uL"),
                ),
            ]),
            review_status: "research_draft".into(),
        }
    }

    #[test]
    fn volume_outside_reference_is_warning() {
        let result = assess_working_volume(
            &profile(),
            "plate-1",
            &Quantity {
                value: 250.0,
                unit: "uL".into(),
            },
        );
        assert_eq!(result.status, AssessmentStatus::Warning);
    }

    #[test]
    fn unknown_volume_is_unresolved() {
        let mut profile = profile();
        profile.fields.remove("working_volume_range");
        let result = assess_working_volume(
            &profile,
            "plate-1",
            &Quantity {
                value: 150.0,
                unit: "uL".into(),
            },
        );
        assert_eq!(result.status, AssessmentStatus::Unresolved);
    }

    #[test]
    fn seeding_uses_area_and_density() {
        let result = assess_seeding_density(
            &profile(),
            "plate-1",
            &Quantity {
                value: 20000.0,
                unit: "cells/cm2".into(),
            },
        );
        assert_eq!(result.status, AssessmentStatus::Valid);
        assert_eq!(result.observed.unwrap().value, 6400.0);
    }
}
