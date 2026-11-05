//! Labware profiles and planned container relations (C3-1).
//!
//! These are Scientific Core planning objects, not ViewDocument content.
//! Profile fields carry their own knowledge state; empty values never imply a
//! plausible default.

use serde::{Deserialize, Serialize};
use serde_json::Value;
use std::collections::{BTreeMap, HashMap, HashSet};

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq, Hash)]
#[serde(deny_unknown_fields)]
pub struct ProfileRef {
    pub id: String,
    pub version: u32,
}

#[derive(Debug, Clone, Copy, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "snake_case")]
pub enum LabwareFamily {
    CellCulturePlate,
    AssayPlate,
    PcrPlate,
    DeepWellPlate,
    TranswellInsert,
    Container,
    Rack,
}

#[derive(Debug, Clone, Copy, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "snake_case")]
pub enum ProfileFieldState {
    Known,
    Unknown,
    NotApplicable,
    Conflicted,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(deny_unknown_fields)]
pub struct EvidenceRef {
    pub source: String,
    pub locator: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[serde(deny_unknown_fields)]
pub struct ProfileField {
    pub state: ProfileFieldState,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub value: Option<Value>,
    #[serde(default)]
    pub evidence: Vec<EvidenceRef>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub note: Option<String>,
}

impl ProfileField {
    pub fn known(value: Value) -> Self {
        Self {
            state: ProfileFieldState::Known,
            value: Some(value),
            evidence: vec![],
            note: None,
        }
    }

    pub fn unknown() -> Self {
        Self {
            state: ProfileFieldState::Unknown,
            value: None,
            evidence: vec![],
            note: None,
        }
    }

    pub fn not_applicable() -> Self {
        Self {
            state: ProfileFieldState::NotApplicable,
            value: None,
            evidence: vec![],
            note: None,
        }
    }
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[serde(deny_unknown_fields)]
pub struct LabwareProfile {
    pub profile: ProfileRef,
    pub family: LabwareFamily,
    pub title: String,
    /// Geometry, physical, use and compatibility fields are keyed by stable
    /// names such as rows, columns, nominal_volume, surface_treatment.
    pub fields: BTreeMap<String, ProfileField>,
    #[serde(default)]
    pub review_status: String,
}

#[derive(Debug, Clone, Default)]
pub struct LabwareRegistry {
    profiles: HashMap<(String, u32), LabwareProfile>,
}

impl LabwareRegistry {
    pub fn register(&mut self, profile: LabwareProfile) -> Result<(), String> {
        let key = (profile.profile.id.clone(), profile.profile.version);
        if let Some(existing) = self.profiles.get(&key) {
            if existing == &profile {
                return Ok(());
            }
            return Err(format!(
                "labware profile {}@{} is already registered with different content",
                profile.profile.id, profile.profile.version
            ));
        }
        if profile.profile.id.trim().is_empty()
            || profile.profile.version == 0
            || profile.title.trim().is_empty()
        {
            return Err("labware profile is missing required identity".into());
        }
        self.profiles.insert(key, profile);
        Ok(())
    }

    pub fn get(&self, reference: &ProfileRef) -> Option<&LabwareProfile> {
        self.profiles
            .get(&(reference.id.clone(), reference.version))
    }
}

pub fn default_labware_registry() -> LabwareRegistry {
    let mut registry = LabwareRegistry::default();
    let geometry = |rows: u64, columns: u64| {
        BTreeMap::from([
            ("rows".into(), ProfileField::known(rows.into())),
            ("columns".into(), ProfileField::known(columns.into())),
            (
                "coordinate_system".into(),
                ProfileField::known("A1 row-major".into()),
            ),
        ])
    };
    let base = |id: &str, family, title: &str, rows, columns| LabwareProfile {
        profile: ProfileRef {
            id: id.into(),
            version: 1,
        },
        family,
        title: title.into(),
        fields: {
            let mut fields = geometry(rows, columns);
            fields.insert("nominal_volume".into(), ProfileField::unknown());
            fields.insert("working_volume_range".into(), ProfileField::unknown());
            fields.insert("surface_treatment".into(), ProfileField::unknown());
            fields.insert("bottom_shape".into(), ProfileField::unknown());
            fields.insert("culture_area".into(), ProfileField::not_applicable());
            fields.insert("sterility".into(), ProfileField::unknown());
            fields.insert("optical_properties".into(), ProfileField::unknown());
            fields.insert("compatible_instruments".into(), ProfileField::unknown());
            fields
        },
        review_status: "research_draft".into(),
    };
    registry
        .register(base(
            "labware.cell_culture_96",
            LabwareFamily::CellCulturePlate,
            "96-well cell culture plate",
            8,
            12,
        ))
        .unwrap();
    registry
        .register(base(
            "labware.pcr_96",
            LabwareFamily::PcrPlate,
            "96-well PCR plate",
            8,
            12,
        ))
        .unwrap();
    registry
        .register(base(
            "labware.cell_culture_6",
            LabwareFamily::CellCulturePlate,
            "6-well cell culture plate",
            2,
            3,
        ))
        .unwrap();
    registry
        .register(base(
            "labware.transwell_insert_6",
            LabwareFamily::TranswellInsert,
            "Transwell insert for 6-well plate",
            1,
            1,
        ))
        .unwrap();
    registry
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(deny_unknown_fields)]
pub struct PlannedContainer {
    pub id: String,
    pub profile_ref: ProfileRef,
    pub label: String,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub source: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(tag = "kind", rename_all = "snake_case")]
pub enum PlanTargetRef {
    Sample { id: String },
    Reaction { id: String },
    Material { id: String },
    Data { id: String },
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(deny_unknown_fields)]
pub struct AssignedTarget {
    pub target: PlanTargetRef,
    pub role: String,
    pub relationship: String,
    #[serde(default)]
    pub counts_as_independent_unit: bool,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub source: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(deny_unknown_fields)]
pub struct PositionAssignment {
    pub container_id: String,
    pub position_id: String,
    pub targets: Vec<AssignedTarget>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(deny_unknown_fields)]
pub struct ContainerPlacement {
    pub parent_container_id: String,
    pub position_id: String,
    pub child_container_id: String,
    pub relationship: String,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub source: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(deny_unknown_fields)]
pub struct ContainerPlan {
    #[serde(default)]
    pub containers: Vec<PlannedContainer>,
    #[serde(default)]
    pub assignments: Vec<PositionAssignment>,
    #[serde(default)]
    pub placements: Vec<ContainerPlacement>,
}

impl Default for ContainerPlan {
    fn default() -> Self {
        Self {
            containers: vec![],
            assignments: vec![],
            placements: vec![],
        }
    }
}

impl ContainerPlan {
    pub fn validate(&self, registry: &LabwareRegistry) -> Result<(), Vec<String>> {
        let mut issues = Vec::new();
        let mut container_ids = HashSet::new();
        let mut profiles = HashMap::new();
        for container in &self.containers {
            if container.id.trim().is_empty() {
                issues.push("container id must not be empty".into());
            }
            if !container_ids.insert(container.id.as_str()) {
                issues.push(format!("duplicate container id {}", container.id));
            }
            let Some(profile) = registry.get(&container.profile_ref) else {
                issues.push(format!(
                    "container {} references unknown profile {}@{}",
                    container.id, container.profile_ref.id, container.profile_ref.version
                ));
                continue;
            };
            profiles.insert(container.id.as_str(), profile);
            if profile.review_status == "conflicted" {
                issues.push(format!(
                    "container {} uses a conflicted profile",
                    container.id
                ));
            }
        }

        for assignment in &self.assignments {
            let Some(profile) = profiles.get(assignment.container_id.as_str()) else {
                issues.push(format!(
                    "assignment references unknown container {}",
                    assignment.container_id
                ));
                continue;
            };
            if !position_is_valid(profile, &assignment.position_id) {
                issues.push(format!(
                    "assignment position {} is not valid for container {}",
                    assignment.position_id, assignment.container_id
                ));
            }
            if assignment.targets.is_empty() {
                issues.push(format!(
                    "assignment {}:{} has no targets",
                    assignment.container_id, assignment.position_id
                ));
            }
            let mut target_keys = HashSet::new();
            for target in &assignment.targets {
                if target.role.trim().is_empty() || target.relationship.trim().is_empty() {
                    issues.push(format!(
                        "assignment {}:{} target needs role and relationship",
                        assignment.container_id, assignment.position_id
                    ));
                }
                let key = format!("{}:{:?}", target.relationship, target.target);
                if !target_keys.insert(key) {
                    issues.push(format!(
                        "assignment {}:{} repeats a target relationship",
                        assignment.container_id, assignment.position_id
                    ));
                }
            }
        }

        for placement in &self.placements {
            let Some(parent) = profiles.get(placement.parent_container_id.as_str()) else {
                issues.push(format!(
                    "placement references unknown parent {}",
                    placement.parent_container_id
                ));
                continue;
            };
            if !container_ids.contains(placement.child_container_id.as_str()) {
                issues.push(format!(
                    "placement references unknown child {}",
                    placement.child_container_id
                ));
            }
            if placement.parent_container_id == placement.child_container_id {
                issues.push("container cannot be placed inside itself".into());
            }
            if !position_is_valid(parent, &placement.position_id) {
                issues.push(format!(
                    "placement position {} is not valid for container {}",
                    placement.position_id, placement.parent_container_id
                ));
            }
            if placement.relationship.trim().is_empty() {
                issues.push("placement relationship must not be empty".into());
            }
        }

        if issues.is_empty() {
            Ok(())
        } else {
            Err(issues)
        }
    }
}

fn position_is_valid(profile: &LabwareProfile, position: &str) -> bool {
    let rows = profile
        .fields
        .get("rows")
        .and_then(|field| field.value.as_ref())
        .and_then(Value::as_u64);
    let columns = profile
        .fields
        .get("columns")
        .and_then(|field| field.value.as_ref())
        .and_then(Value::as_u64);
    let (Some(rows), Some(columns)) = (rows, columns) else {
        return false;
    };
    let bytes = position.as_bytes();
    if bytes.is_empty() {
        return false;
    }
    let row = (bytes[0] as char).to_ascii_uppercase() as u64;
    if !(b'A' as u64..b'A' as u64 + rows).contains(&row) {
        return false;
    }
    let column = position[1..].parse::<u64>().ok();
    column.is_some_and(|column| (1..=columns).contains(&column))
}

#[cfg(test)]
mod tests {
    use super::*;

    fn container(id: &str, profile: &str) -> PlannedContainer {
        PlannedContainer {
            id: id.into(),
            profile_ref: ProfileRef {
                id: profile.into(),
                version: 1,
            },
            label: id.into(),
            source: None,
        }
    }

    #[test]
    fn profile_fields_keep_unknown_and_not_applicable() {
        let registry = default_labware_registry();
        let profile = registry
            .get(&ProfileRef {
                id: "labware.pcr_96".into(),
                version: 1,
            })
            .unwrap();
        assert_eq!(
            profile.fields["culture_area"].state,
            ProfileFieldState::NotApplicable
        );
        assert_eq!(
            profile.fields["surface_treatment"].state,
            ProfileFieldState::Unknown
        );
        assert!(profile.fields["surface_treatment"].value.is_none());
    }

    #[test]
    fn plan_validates_container_positions_and_placement() {
        let registry = default_labware_registry();
        let plan = ContainerPlan {
            containers: vec![
                container("plate-1", "labware.cell_culture_96"),
                container("insert-1", "labware.transwell_insert_6"),
            ],
            assignments: vec![PositionAssignment {
                container_id: "plate-1".into(),
                position_id: "A1".into(),
                targets: vec![AssignedTarget {
                    target: PlanTargetRef::Sample { id: "s1".into() },
                    role: "primary".into(),
                    relationship: "contains".into(),
                    counts_as_independent_unit: true,
                    source: None,
                }],
            }],
            placements: vec![ContainerPlacement {
                parent_container_id: "plate-1".into(),
                position_id: "A1".into(),
                child_container_id: "insert-1".into(),
                relationship: "inserted_into".into(),
                source: None,
            }],
        };
        plan.validate(&registry).unwrap();
    }

    #[test]
    fn invalid_position_and_unknown_profile_are_rejected() {
        let registry = default_labware_registry();
        let plan = ContainerPlan {
            containers: vec![container("plate-1", "labware.cell_culture_96")],
            assignments: vec![PositionAssignment {
                container_id: "plate-1".into(),
                position_id: "Z99".into(),
                targets: vec![AssignedTarget {
                    target: PlanTargetRef::Reaction { id: "r1".into() },
                    role: "primary".into(),
                    relationship: "reaction_in".into(),
                    counts_as_independent_unit: true,
                    source: None,
                }],
            }],
            placements: vec![],
        };
        assert!(plan.validate(&registry).is_err());

        let mut plan = plan;
        plan.containers[0].profile_ref.id = "missing".into();
        assert!(plan.validate(&registry).is_err());
    }

    #[test]
    fn multiple_targets_do_not_infer_independent_count() {
        let registry = default_labware_registry();
        let plan = ContainerPlan {
            containers: vec![container("plate-1", "labware.cell_culture_96")],
            assignments: vec![PositionAssignment {
                container_id: "plate-1".into(),
                position_id: "A1".into(),
                targets: vec![
                    AssignedTarget {
                        target: PlanTargetRef::Sample { id: "s1".into() },
                        role: "primary".into(),
                        relationship: "contains".into(),
                        counts_as_independent_unit: true,
                        source: None,
                    },
                    AssignedTarget {
                        target: PlanTargetRef::Material { id: "m1".into() },
                        role: "reagent".into(),
                        relationship: "uses".into(),
                        counts_as_independent_unit: false,
                        source: None,
                    },
                ],
            }],
            placements: vec![],
        };
        assert_eq!(
            plan.assignments[0]
                .targets
                .iter()
                .filter(|target| target.counts_as_independent_unit)
                .count(),
            1
        );
        plan.validate(&registry).unwrap();
    }
}
