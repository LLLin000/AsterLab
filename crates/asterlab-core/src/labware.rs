//! Labware profiles and planned container relations (C3-1).
//!
//! These are Scientific Core planning objects, not ViewDocument content.
//! Profile fields carry their own knowledge state; empty values never imply a
//! plausible default.

use serde::{Deserialize, Serialize};
use serde_json::Value;
use std::collections::{BTreeMap, HashMap, HashSet};

use crate::diagnostic::Diagnostic;

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
pub enum LabwareCompatibility {
    Compatible,
    Incompatible,
    Unknown,
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
pub struct Quantity {
    pub value: f64,
    pub unit: String,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[serde(deny_unknown_fields)]
pub struct QuantityRange {
    pub min: Quantity,
    pub max: Quantity,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[serde(deny_unknown_fields)]
pub struct ProfileField {
    pub state: ProfileFieldState,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub value: Option<Value>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub quantity: Option<Quantity>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub range: Option<QuantityRange>,
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
            quantity: None,
            range: None,
            evidence: vec![],
            note: None,
        }
    }

    pub fn known_quantity(value: f64, unit: impl Into<String>) -> Self {
        Self {
            state: ProfileFieldState::Known,
            value: None,
            quantity: Some(Quantity {
                value,
                unit: unit.into(),
            }),
            range: None,
            evidence: vec![],
            note: None,
        }
    }

    pub fn known_range(min: f64, max: f64, unit: impl Into<String>) -> Self {
        let unit = unit.into();
        Self {
            state: ProfileFieldState::Known,
            value: None,
            quantity: None,
            range: Some(QuantityRange {
                min: Quantity {
                    value: min,
                    unit: unit.clone(),
                },
                max: Quantity { value: max, unit },
            }),
            evidence: vec![],
            note: None,
        }
    }

    pub fn unknown() -> Self {
        Self {
            state: ProfileFieldState::Unknown,
            value: None,
            quantity: None,
            range: None,
            evidence: vec![],
            note: None,
        }
    }

    pub fn not_applicable() -> Self {
        Self {
            state: ProfileFieldState::NotApplicable,
            value: None,
            quantity: None,
            range: None,
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
        validate_profile_fields(&profile)?;
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
            fields.insert("culture_area".into(), ProfileField::unknown());
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
    let mut pcr = base(
        "labware.pcr_96",
        LabwareFamily::PcrPlate,
        "96-well PCR plate",
        8,
        12,
    );
    pcr.fields
        .insert("culture_area".into(), ProfileField::not_applicable());
    registry.register(pcr).unwrap();
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
            "labware.cell_culture_24",
            LabwareFamily::CellCulturePlate,
            "24-well cell culture plate",
            4,
            6,
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

#[derive(Debug, Clone, Copy, Default, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "snake_case")]
pub enum ContainerTopology {
    #[default]
    TwoD,
    #[serde(rename = "lane_1d")]
    Lane1d,
    CageCohort,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(deny_unknown_fields)]
pub struct PhysicalSplitChild {
    pub container_id: String,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub sample_id: Option<String>,
    pub aliquot_from: String,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub run_id: Option<String>,
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

/// Extended topology metadata kept separate so existing `PlannedContainer`
/// struct literals and frozen container plans remain source-compatible.
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(deny_unknown_fields)]
pub struct PhysicalContainer {
    pub container: PlannedContainer,
    #[serde(default)]
    pub topology: ContainerTopology,
    #[serde(default)]
    pub split_children: Vec<PhysicalSplitChild>,
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

/// One planned position and what is assigned to it.
///
/// `target`, `well_role` and `dilution_factor` are optional detection-layout
/// annotations. They default to `None`, so plans serialized before they existed
/// deserialize without migration and old records stay byte-compatible.
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[serde(deny_unknown_fields)]
pub struct PositionAssignment {
    pub container_id: String,
    pub position_id: String,
    pub targets: Vec<AssignedTarget>,
    /// Planned detection target label, for example a gene or analyte name.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub target: Option<String>,
    /// Planned role of this position in the detection layout.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub well_role: Option<WellRole>,
    /// Planned dilution factor at this position; unitless and positive.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub dilution_factor: Option<f64>,
}

/// Planned role of a well or 1D lane in a detection layout.
///
/// A superset of [`crate::readout_schema::WellRole`]: a planned layout also
/// marks positive controls, which a readout records only when the instrument
/// reports them.
#[derive(Debug, Clone, Copy, Serialize, Deserialize, PartialEq, Eq, Hash)]
#[serde(rename_all = "snake_case")]
pub enum WellRole {
    Unknown,
    Ntc,
    Standard,
    PositiveControl,
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

/// A plan holds `dilution_factor: Option<f64>`, so it is only [`PartialEq`].
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
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
    /// Legacy string view of [`Self::validate_diagnostics`], kept so existing
    /// callers and frozen output shapes stay unchanged.
    pub fn validate(&self, registry: &LabwareRegistry) -> Result<(), Vec<String>> {
        self.validate_diagnostics(registry)
            .map_err(|issues| issues.into_iter().map(|issue| issue.message).collect())
    }

    /// Canonical structured validation. Every finding carries a stable code
    /// and the path of the container, assignment or placement it anchors to.
    pub fn validate_diagnostics(&self, registry: &LabwareRegistry) -> Result<(), Vec<Diagnostic>> {
        let mut issues = Vec::new();
        let mut container_ids = HashSet::new();
        let mut profiles = HashMap::new();
        for (index, container) in self.containers.iter().enumerate() {
            let path = format!("containers[{index}]");
            if container.id.trim().is_empty() {
                issues.push(Diagnostic::blocking(
                    "container_id_empty",
                    format!("{path}.id"),
                    "container id must not be empty",
                ));
            }
            if !container_ids.insert(container.id.as_str()) {
                issues.push(Diagnostic::blocking(
                    "container_id_duplicate",
                    format!("{path}.id"),
                    format!("duplicate container id {}", container.id),
                ));
            }
            let Some(profile) = registry.get(&container.profile_ref) else {
                issues.push(Diagnostic::blocking(
                    "container_profile_unknown",
                    format!("{path}.profile_ref"),
                    format!(
                        "container {} references unknown profile {}@{}",
                        container.id, container.profile_ref.id, container.profile_ref.version
                    ),
                ));
                continue;
            };
            profiles.insert(container.id.as_str(), profile);
            if profile.review_status == "conflicted" {
                issues.push(Diagnostic::blocking(
                    "container_profile_conflicted",
                    format!("{path}.review_status"),
                    format!("container {} uses a conflicted profile", container.id),
                ));
            }
        }

        let mut assignment_positions = HashSet::new();
        for (index, assignment) in self.assignments.iter().enumerate() {
            let path = format!("assignments[{index}]");
            let key = format!("{}::{}", assignment.container_id, assignment.position_id);
            if !assignment_positions.insert(key) {
                issues.push(Diagnostic::blocking(
                    "assignment_duplicate",
                    path.clone(),
                    format!(
                        "duplicate assignment for {}:{}",
                        assignment.container_id, assignment.position_id
                    ),
                ));
            }
            let Some(profile) = profiles.get(assignment.container_id.as_str()) else {
                issues.push(Diagnostic::blocking(
                    "assignment_container_unknown",
                    format!("{path}.container_id"),
                    format!(
                        "assignment references unknown container {}",
                        assignment.container_id
                    ),
                ));
                continue;
            };
            if !position_is_valid(profile, &assignment.position_id) {
                issues.push(Diagnostic::blocking(
                    "assignment_position_invalid",
                    format!("{path}.position_id"),
                    format!(
                        "assignment position {} is not valid for container {}",
                        assignment.position_id, assignment.container_id
                    ),
                ));
            }
            if assignment.targets.is_empty() {
                issues.push(Diagnostic::blocking(
                    "assignment_targets_missing",
                    format!("{path}.targets"),
                    format!(
                        "assignment {}:{} has no targets",
                        assignment.container_id, assignment.position_id
                    ),
                ));
            }
            let mut target_keys = HashSet::new();
            let primaries = assignment
                .targets
                .iter()
                .filter(|target| target.role.eq_ignore_ascii_case("primary"))
                .count();
            if primaries > 1 {
                issues.push(Diagnostic::blocking(
                    "assignment_primary_conflict",
                    format!("{path}.targets"),
                    format!(
                        "assignment {}:{} has multiple primary targets",
                        assignment.container_id, assignment.position_id
                    ),
                ));
            }
            for (target_index, target) in assignment.targets.iter().enumerate() {
                let target_path = format!("{path}.targets[{target_index}]");
                if target.role.trim().is_empty() || target.relationship.trim().is_empty() {
                    issues.push(Diagnostic::blocking(
                        "assignment_target_incomplete",
                        target_path.clone(),
                        format!(
                            "assignment {}:{} target needs role and relationship",
                            assignment.container_id, assignment.position_id
                        ),
                    ));
                }
                let key = format!("{}:{:?}", target.relationship, target.target);
                if !target_keys.insert(key) {
                    issues.push(Diagnostic::blocking(
                        "assignment_target_duplicate",
                        target_path,
                        format!(
                            "assignment {}:{} repeats a target relationship",
                            assignment.container_id, assignment.position_id
                        ),
                    ));
                }
            }
        }

        let mut placement_children = HashSet::new();
        let mut placement_positions = HashSet::new();
        let mut placement_pairs = HashSet::new();
        for (index, placement) in self.placements.iter().enumerate() {
            let path = format!("placements[{index}]");
            let Some(parent) = profiles.get(placement.parent_container_id.as_str()) else {
                issues.push(Diagnostic::blocking(
                    "placement_parent_unknown",
                    format!("{path}.parent_container_id"),
                    format!(
                        "placement references unknown parent {}",
                        placement.parent_container_id
                    ),
                ));
                continue;
            };
            if !container_ids.contains(placement.child_container_id.as_str()) {
                issues.push(Diagnostic::blocking(
                    "placement_child_unknown",
                    format!("{path}.child_container_id"),
                    format!(
                        "placement references unknown child {}",
                        placement.child_container_id
                    ),
                ));
            }
            if placement.parent_container_id == placement.child_container_id {
                issues.push(Diagnostic::blocking(
                    "placement_self_reference",
                    path.clone(),
                    "container cannot be placed inside itself",
                ));
            }
            if !placement_children.insert(placement.child_container_id.as_str()) {
                issues.push(Diagnostic::blocking(
                    "placement_child_reused",
                    format!("{path}.child_container_id"),
                    format!(
                        "child container {} is placed in more than one parent",
                        placement.child_container_id
                    ),
                ));
            }
            let position_key = format!(
                "{}::{}",
                placement.parent_container_id, placement.position_id
            );
            if !placement_positions.insert(position_key) {
                issues.push(Diagnostic::blocking(
                    "placement_position_occupied",
                    format!("{path}.position_id"),
                    format!(
                        "position {} already has a placed child",
                        placement.position_id
                    ),
                ));
            }
            let pair = format!(
                "{}::{}::{}",
                placement.parent_container_id, placement.position_id, placement.child_container_id
            );
            if !placement_pairs.insert(pair) {
                issues.push(Diagnostic::blocking(
                    "placement_duplicate",
                    path.clone(),
                    "duplicate container placement",
                ));
            }
            if !position_is_valid(parent, &placement.position_id) {
                issues.push(Diagnostic::blocking(
                    "placement_position_invalid",
                    format!("{path}.position_id"),
                    format!(
                        "placement position {} is not valid for container {}",
                        placement.position_id, placement.parent_container_id
                    ),
                ));
            }
            if placement.relationship.trim().is_empty() {
                issues.push(Diagnostic::blocking(
                    "placement_relationship_missing",
                    format!("{path}.relationship"),
                    "placement relationship must not be empty",
                ));
            }
            if let Some(child) = profiles.get(placement.child_container_id.as_str()) {
                if placement_compatibility(parent, child) == LabwareCompatibility::Incompatible {
                    issues.push(Diagnostic::blocking(
                        "placement_incompatible",
                        path.clone(),
                        format!(
                            "container {} is incompatible with parent {}",
                            placement.child_container_id, placement.parent_container_id
                        ),
                    ));
                }
            }
        }
        let mut placement_edges: HashMap<String, Vec<String>> = HashMap::new();
        for placement in &self.placements {
            placement_edges
                .entry(placement.parent_container_id.clone())
                .or_default()
                .push(placement.child_container_id.clone());
        }
        if has_placement_cycle(&placement_edges) {
            issues.push(Diagnostic::blocking(
                "placement_cycle",
                "placements",
                "container placement graph contains a cycle",
            ));
        }

        if issues.is_empty() {
            Ok(())
        } else {
            Err(issues)
        }
    }
}

fn profile_field_diagnostics(profile: &LabwareProfile) -> Vec<Diagnostic> {
    let mut issues = Vec::new();
    for (key, field) in &profile.fields {
        let path = format!("fields.{key}");
        match field.state {
            ProfileFieldState::Known
                if field.value.is_none() && field.quantity.is_none() && field.range.is_none() =>
            {
                issues.push(Diagnostic::blocking(
                    "profile_field_value_missing",
                    path,
                    format!("profile field {key} is known but has no value"),
                ));
            }
            ProfileFieldState::Unknown | ProfileFieldState::NotApplicable
                if field.value.is_some() =>
            {
                issues.push(Diagnostic::blocking(
                    "profile_field_state_conflict",
                    path,
                    format!("profile field {key} carries a value but is not known"),
                ));
            }
            ProfileFieldState::Conflicted if field.value.is_some() => {
                issues.push(Diagnostic::blocking(
                    "profile_field_conflict_single_value",
                    path,
                    format!("profile field {key} is conflicted but carries one value"),
                ));
            }
            ProfileFieldState::Conflicted if field.evidence.is_empty() => {
                issues.push(Diagnostic::blocking(
                    "profile_field_conflict_evidence_missing",
                    path,
                    format!("profile field {key} is conflicted without evidence"),
                ));
            }
            _ => {}
        }
    }
    issues
}

fn validate_profile_fields(profile: &LabwareProfile) -> Result<(), String> {
    match profile_field_diagnostics(profile).into_iter().next() {
        Some(issue) => Err(issue.message),
        None => Ok(()),
    }
}

pub fn placement_compatibility(
    parent: &LabwareProfile,
    child: &LabwareProfile,
) -> LabwareCompatibility {
    let accepted = parent
        .fields
        .get("accepts_child_families")
        .and_then(|field| field.value.as_ref())
        .and_then(Value::as_array);
    let parents = child
        .fields
        .get("parent_families")
        .and_then(|field| field.value.as_ref())
        .and_then(Value::as_array);
    let (Some(accepted), Some(parents)) = (accepted, parents) else {
        return LabwareCompatibility::Unknown;
    };
    let parent_ok = parents.iter().any(|family| {
        serde_json::to_value(parent.family)
            .ok()
            .as_ref()
            .is_some_and(|value| value == family)
    });
    let child_ok = accepted.iter().any(|family| {
        serde_json::to_value(child.family)
            .ok()
            .as_ref()
            .is_some_and(|value| value == family)
    });
    if parent_ok && child_ok {
        LabwareCompatibility::Compatible
    } else {
        LabwareCompatibility::Incompatible
    }
}

fn has_placement_cycle(edges: &HashMap<String, Vec<String>>) -> bool {
    fn visit(
        node: &str,
        edges: &HashMap<String, Vec<String>>,
        state: &mut HashMap<String, u8>,
    ) -> bool {
        match state.get(node).copied() {
            Some(1) => return true,
            Some(2) => return false,
            _ => {}
        }
        state.insert(node.to_owned(), 1);
        if let Some(next) = edges.get(node) {
            for child in next {
                if visit(child, edges, state) {
                    return true;
                }
            }
        }
        state.insert(node.to_owned(), 2);
        false
    }
    let mut state = HashMap::new();
    edges.keys().any(|node| visit(node, edges, &mut state))
}

/// Canonical address of a planned container position.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash)]
pub enum ContainerPosition {
    /// 2D plate well; `row` and `column` are 1-based (`A1` is row 1, column 1).
    Well { row: u64, column: u64 },
    /// 1D slot of a single-row container (`Lane3`, `L3`, `Tube3`); 1-based.
    Lane { index: u64 },
}

/// Parse a canonical position for `profile`, or `None` when it is not valid.
///
/// Two-dimensional plates address wells as `<row><column>`, where the row is one
/// or two uppercase letters (`A1`, `AA12`). A single-row container
/// (`rows == 1`) additionally accepts 1D slot names `Lane<N>`, `L<N>` and
/// `Tube<N>`. Input must already be canonical: lowercase, zero-padded, unicode
/// or out-of-range addresses are rejected rather than normalized.
pub fn parse_container_position(
    profile: &LabwareProfile,
    position: &str,
) -> Option<ContainerPosition> {
    let rows = geometry_value(profile, "rows")?;
    let columns = geometry_value(profile, "columns")?;
    if rows == 0 || columns == 0 {
        return None;
    }
    if rows == 1 {
        if let Some(index) = parse_lane_index(position) {
            return (1..=columns)
                .contains(&index)
                .then_some(ContainerPosition::Lane { index });
        }
    }
    let (row, column) = parse_well_position(position)?;
    (row <= rows && (1..=columns).contains(&column))
        .then_some(ContainerPosition::Well { row, column })
}

fn position_is_valid(profile: &LabwareProfile, position: &str) -> bool {
    parse_container_position(profile, position).is_some()
}

fn geometry_value(profile: &LabwareProfile, key: &str) -> Option<u64> {
    profile.fields.get(key)?.value.as_ref()?.as_u64()
}

fn parse_lane_index(position: &str) -> Option<u64> {
    // `Lane` is checked before `L`, otherwise `Lane4` would parse as `ane4`.
    ["Lane", "Tube", "L"]
        .into_iter()
        .find_map(|prefix| position.strip_prefix(prefix))
        .and_then(parse_canonical_number)
}

fn parse_well_position(position: &str) -> Option<(u64, u64)> {
    let letters_end = position
        .char_indices()
        .find(|(_, ch)| !ch.is_ascii_uppercase())
        .map_or(position.len(), |(index, _)| index);
    let (row_letters, column_digits) = position.split_at(letters_end);
    if row_letters.is_empty() || row_letters.len() > 2 {
        return None;
    }
    if !column_digits.bytes().all(|byte| byte.is_ascii_digit()) {
        return None;
    }
    let row = row_letters
        .bytes()
        .fold(0u64, |row, byte| row * 26 + u64::from(byte - b'A') + 1);
    let column = parse_canonical_number(column_digits)?;
    Some((row, column))
}

fn parse_canonical_number(digits: &str) -> Option<u64> {
    if digits.is_empty() || digits.len() > 2 || digits.starts_with('0') {
        return None;
    }
    digits.parse().ok()
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
    fn culture_area_is_unknown_not_not_applicable_for_culture_plate() {
        let registry = default_labware_registry();
        let culture = registry
            .get(&ProfileRef {
                id: "labware.cell_culture_96".into(),
                version: 1,
            })
            .unwrap();
        assert_eq!(
            culture.fields["culture_area"].state,
            ProfileFieldState::Unknown
        );
        let pcr = registry
            .get(&ProfileRef {
                id: "labware.pcr_96".into(),
                version: 1,
            })
            .unwrap();
        assert_eq!(
            pcr.fields["culture_area"].state,
            ProfileFieldState::NotApplicable
        );
    }

    #[test]
    fn profile_registry_rejects_inconsistent_field_states() {
        let mut registry = LabwareRegistry::default();
        let mut fields = BTreeMap::new();
        fields.insert(
            "rows".into(),
            ProfileField {
                state: ProfileFieldState::Known,
                value: None,
                quantity: None,
                range: None,
                evidence: vec![],
                note: None,
            },
        );
        let profile = LabwareProfile {
            profile: ProfileRef {
                id: "bad".into(),
                version: 1,
            },
            family: LabwareFamily::CellCulturePlate,
            title: "bad".into(),
            fields,
            review_status: "research_draft".into(),
        };
        assert!(registry.register(profile).is_err());
    }

    #[test]
    fn placement_cycles_duplicate_assignments_and_unicode_positions_are_rejected() {
        let registry = default_labware_registry();
        let mut plan = ContainerPlan {
            containers: vec![
                container("plate-1", "labware.cell_culture_96"),
                container("insert-1", "labware.transwell_insert_6"),
            ],
            assignments: vec![PositionAssignment {
                container_id: "plate-1".into(),
                position_id: "A１".into(),
                targets: vec![AssignedTarget {
                    target: PlanTargetRef::Sample { id: "s1".into() },
                    role: "primary".into(),
                    relationship: "contains".into(),
                    counts_as_independent_unit: true,
                    source: None,
                }],
                target: None,
                well_role: None,
                dilution_factor: None,
            }],
            placements: vec![
                ContainerPlacement {
                    parent_container_id: "plate-1".into(),
                    position_id: "A1".into(),
                    child_container_id: "insert-1".into(),
                    relationship: "inserted_into".into(),
                    source: None,
                },
                ContainerPlacement {
                    parent_container_id: "insert-1".into(),
                    position_id: "A1".into(),
                    child_container_id: "plate-1".into(),
                    relationship: "inserted_into".into(),
                    source: None,
                },
            ],
        };
        assert!(plan.validate(&registry).is_err());

        plan.placements.clear();
        plan.assignments.push(plan.assignments[0].clone());
        assert!(plan.validate(&registry).is_err());
    }

    #[test]
    fn container_plan_diagnostics_carry_paths_and_codes() {
        use crate::diagnostic::{DiagnosticSeverity, DiagnosticSource};

        let registry = default_labware_registry();
        let plan = ContainerPlan {
            containers: vec![
                container("plate-1", "labware.cell_culture_96"),
                container("plate-1", "labware.cell_culture_96"),
            ],
            assignments: vec![],
            placements: vec![],
        };
        let issues = plan.validate_diagnostics(&registry).unwrap_err();
        let duplicate = issues
            .iter()
            .find(|issue| issue.code == "container_id_duplicate")
            .expect("duplicate container id diagnostic");
        assert_eq!(duplicate.path, "containers[1].id");
        assert_eq!(duplicate.severity, DiagnosticSeverity::Blocking);
        assert_eq!(duplicate.source, DiagnosticSource::Derived);
        // The legacy string view stays a lossless projection of the messages.
        let legacy = plan.validate(&registry).unwrap_err();
        let expected: Vec<String> = issues.into_iter().map(|issue| issue.message).collect();
        assert_eq!(legacy, expected);
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
                target: None,
                well_role: None,
                dilution_factor: None,
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
                target: None,
                well_role: None,
                dilution_factor: None,
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
                target: None,
                well_role: None,
                dilution_factor: None,
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

    #[test]
    fn planned_containers_represent_lanes_cages_and_split_lineage() {
        let lane: PhysicalContainer = serde_json::from_value(serde_json::json!({
            "container": {
                "id": "wb-lanes",
                "profile_ref": {"id": "labware.western_blot_membrane", "version": 1},
                "label": "WB membrane"
            },
            "topology": "lane_1d",
            "split_children": [
                {"container_id": "tumor-formalin", "sample_id": "aliquot-formalin", "aliquot_from": "tumor-1"},
                {"container_id": "tumor-rna", "sample_id": "aliquot-rna", "aliquot_from": "tumor-1"},
                {"container_id": "tumor-protein", "sample_id": "aliquot-protein", "aliquot_from": "tumor-1"}
            ]
        })).unwrap();
        assert_eq!(lane.topology, ContainerTopology::Lane1d);
        assert_eq!(lane.split_children[0].aliquot_from, "tumor-1");
        assert_eq!(lane.split_children.len(), 3);
        let cage: PhysicalContainer = serde_json::from_value(serde_json::json!({
            "container": {
                "id": "cage",
                "profile_ref": {"id": "labware.mouse_cage", "version": 1},
                "label": "Cage 2"
            },
            "topology": "cage_cohort"
        })).unwrap();
        assert_eq!(cage.topology, ContainerTopology::CageCohort);
    }

    fn lane_registry() -> LabwareRegistry {
        let mut registry = LabwareRegistry::default();
        registry
            .register(LabwareProfile {
                profile: ProfileRef {
                    id: "labware.lane_gel_10".into(),
                    version: 1,
                },
                family: LabwareFamily::Container,
                title: "10-lane gel".into(),
                fields: BTreeMap::from([
                    ("rows".into(), ProfileField::known(1.into())),
                    ("columns".into(), ProfileField::known(10.into())),
                ]),
                review_status: "research_draft".into(),
            })
            .unwrap();
        registry
    }

    fn lane_profile(registry: &LabwareRegistry) -> &LabwareProfile {
        registry
            .get(&ProfileRef {
                id: "labware.lane_gel_10".into(),
                version: 1,
            })
            .unwrap()
    }

    #[test]
    fn single_row_containers_accept_lane_and_tube_positions() {
        let registry = lane_registry();
        let profile = lane_profile(&registry);
        for index in 1..=10 {
            for position in [
                format!("Lane{index}"),
                format!("L{index}"),
                format!("Tube{index}"),
            ] {
                assert_eq!(
                    parse_container_position(profile, &position),
                    Some(ContainerPosition::Lane { index }),
                    "{position}"
                );
            }
        }
        assert_eq!(
            parse_container_position(profile, "A1"),
            Some(ContainerPosition::Well { row: 1, column: 1 })
        );
        for position in [
            "Lane11", "L11", "Tube11", "Lane0", "Lane01", "lane1", "Lane", "Lane-1", "B1", "A11",
        ] {
            assert_eq!(
                parse_container_position(profile, position),
                None,
                "{position}"
            );
        }
    }

    #[test]
    fn two_d_plate_positions_keep_their_contract() {
        let registry = default_labware_registry();
        let plate = registry
            .get(&ProfileRef {
                id: "labware.pcr_96".into(),
                version: 1,
            })
            .unwrap();
        assert_eq!(
            parse_container_position(plate, "A1"),
            Some(ContainerPosition::Well { row: 1, column: 1 })
        );
        assert_eq!(
            parse_container_position(plate, "H12"),
            Some(ContainerPosition::Well { row: 8, column: 12 })
        );
        for position in [
            "A0", "A01", "A13", "I1", "AA1", "a1", "A１", "A", "1A", "Lane1", "L1", "Tube1",
        ] {
            assert_eq!(parse_container_position(plate, position), None, "{position}");
        }

        let mut registry = default_labware_registry();
        registry
            .register(LabwareProfile {
                profile: ProfileRef {
                    id: "labware.wide_2d_27".into(),
                    version: 1,
                },
                family: LabwareFamily::DeepWellPlate,
                title: "27 x 12 deep well plate".into(),
                fields: BTreeMap::from([
                    ("rows".into(), ProfileField::known(27.into())),
                    ("columns".into(), ProfileField::known(12.into())),
                ]),
                review_status: "research_draft".into(),
            })
            .unwrap();
        let wide = registry
            .get(&ProfileRef {
                id: "labware.wide_2d_27".into(),
                version: 1,
            })
            .unwrap();
        assert_eq!(
            parse_container_position(wide, "AA1"),
            Some(ContainerPosition::Well { row: 27, column: 1 })
        );
        // On a multi-row container `L1` is the 2D well at row L, not a lane.
        assert_eq!(
            parse_container_position(wide, "L1"),
            Some(ContainerPosition::Well { row: 12, column: 1 })
        );
        assert_eq!(parse_container_position(wide, "AB1"), None);
    }

    #[test]
    fn well_metadata_fields_are_optional_and_backward_compatible() {
        let legacy: PositionAssignment = serde_json::from_value(serde_json::json!({
            "container_id": "plate-1",
            "position_id": "A1",
            "targets": [{
                "target": {"kind": "sample", "id": "s1"},
                "role": "primary",
                "relationship": "contains"
            }]
        }))
        .unwrap();
        assert_eq!(legacy.target, None);
        assert_eq!(legacy.well_role, None);
        assert_eq!(legacy.dilution_factor, None);
        let legacy_value = serde_json::to_value(&legacy).unwrap();
        for key in ["target", "well_role", "dilution_factor"] {
            assert!(legacy_value.get(key).is_none(), "{key}");
        }

        for (label, role) in [
            ("unknown", WellRole::Unknown),
            ("ntc", WellRole::Ntc),
            ("standard", WellRole::Standard),
            ("positive_control", WellRole::PositiveControl),
        ] {
            let annotated: PositionAssignment = serde_json::from_value(serde_json::json!({
                "container_id": "plate-1",
                "position_id": "A1",
                "targets": [],
                "target": "GAPDH",
                "well_role": label,
                "dilution_factor": 4.0
            }))
            .unwrap();
            assert_eq!(annotated.target.as_deref(), Some("GAPDH"));
            assert_eq!(annotated.well_role, Some(role), "{label}");
            assert_eq!(annotated.dilution_factor, Some(4.0));
            let round_trip = serde_json::to_value(&annotated).unwrap();
            assert_eq!(round_trip["well_role"], serde_json::json!(label));
        }

        assert!(serde_json::from_value::<PositionAssignment>(serde_json::json!({
            "container_id": "plate-1",
            "position_id": "A1",
            "targets": [],
            "well_role": "sample"
        }))
        .is_err());
    }

    fn lane_assignment(position_id: &str, role: Option<WellRole>) -> PositionAssignment {
        PositionAssignment {
            container_id: "lane-gel".into(),
            position_id: position_id.into(),
            targets: vec![AssignedTarget {
                target: PlanTargetRef::Sample {
                    id: format!("sample-{position_id}"),
                },
                role: "primary".into(),
                relationship: "contains".into(),
                counts_as_independent_unit: true,
                source: None,
            }],
            target: Some("GAPDH".into()),
            well_role: role,
            dilution_factor: Some(4.0),
        }
    }

    #[test]
    fn lane_positions_and_metadata_validate_end_to_end() {
        let registry = lane_registry();
        let mut plan = ContainerPlan {
            containers: vec![PlannedContainer {
                id: "lane-gel".into(),
                profile_ref: ProfileRef {
                    id: "labware.lane_gel_10".into(),
                    version: 1,
                },
                label: "Gel".into(),
                source: None,
            }],
            assignments: vec![
                lane_assignment("Lane1", Some(WellRole::PositiveControl)),
                lane_assignment("Tube10", Some(WellRole::Ntc)),
                lane_assignment("L5", Some(WellRole::Standard)),
            ],
            placements: vec![],
        };
        plan.validate(&registry).unwrap();
        assert_eq!(plan.assignments[1].well_role, Some(WellRole::Ntc));

        plan.assignments[0].position_id = "Lane11".into();
        let issues = plan.validate_diagnostics(&registry).unwrap_err();
        assert!(issues.iter().any(|issue| {
            issue.code == "assignment_position_invalid"
                && issue.path == "assignments[0].position_id"
        }));
    }
}
