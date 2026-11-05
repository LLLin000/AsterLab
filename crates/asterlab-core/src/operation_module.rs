//! K1 draft contract for reusable operation modules.
//!
//! This module is intentionally not wired into the CLI, Protocol preview or
//! Run runtime. It is a reviewable schema plus structural checks; a later
//! catalog compiler must map it onto `ProtocolBlockVersion` and `BlockUse`.
//!
//! Scientific values are never inferred here. A missing source or an unknown
//! capability stays explicit.

use crate::protocol_preview::StepTimeScope;
use serde::{Deserialize, Serialize};
use std::collections::HashSet;

pub const OPERATION_MODULE_SCHEMA_VERSION: u32 = 1;

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[serde(deny_unknown_fields)]
pub struct OperationModule {
    pub schema_version: u32,
    pub identity: ModuleIdentity,
    pub semantics: ModuleSemantics,
    pub parameters: Vec<OperationParameter>,
    pub execution: ExecutionSpec,
    pub knowledge: ModuleKnowledge,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[serde(deny_unknown_fields)]
pub struct ModuleIdentity {
    pub module_id: String,
    pub version: u32,
    pub title: String,
    pub action_type: ActionType,
    pub applicability: Applicability,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub variant_of: Option<ModuleRef>,
}

#[derive(Debug, Clone, Copy, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "snake_case")]
pub enum ActionType {
    LiquidTransfer,
    Inoculation,
    Incubation,
    Washing,
    Sampling,
    Centrifugation,
    AbsorbanceMeasurement,
    FluorescenceMeasurement,
    FluorescenceImaging,
    PlateWash,
    Qpcr,
    FlowCytometry,
    Spectrophotometry,
    SampleStorage,
    Other,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Default)]
#[serde(deny_unknown_fields)]
pub struct Applicability {
    #[serde(default)]
    pub assay_families: Vec<String>,
    #[serde(default)]
    pub sample_kinds: Vec<String>,
    #[serde(default)]
    pub container_kinds: Vec<String>,
    #[serde(default)]
    pub variants: Vec<String>,
    #[serde(default)]
    pub exclusions: Vec<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[serde(deny_unknown_fields)]
pub struct ModuleRef {
    pub module_id: String,
    pub version: u32,
}

impl ModuleRef {
    /// Version ranges and "latest" are deliberately not supported.
    pub fn matches_exact(&self, other: &ModuleRef) -> bool {
        self.module_id == other.module_id && self.version == other.version
    }
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[serde(deny_unknown_fields)]
pub struct ModuleSemantics {
    pub purpose: String,
    #[serde(default)]
    pub preconditions: Vec<String>,
    #[serde(default)]
    pub inputs: Vec<PortSpec>,
    #[serde(default)]
    pub outputs: Vec<PortSpec>,
    #[serde(default)]
    pub sample_effects: Vec<SampleEffect>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[serde(deny_unknown_fields)]
pub struct PortSpec {
    pub port_id: String,
    pub role: PortRole,
    pub object_type: PortObjectType,
    pub required: bool,
    pub cardinality: Cardinality,
    pub description: String,
}

#[derive(Debug, Clone, Copy, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "snake_case")]
pub enum PortRole {
    SourceSample,
    Reagent,
    DestinationContainer,
    Consumable,
    Instrument,
    RawMeasurement,
    DerivedResult,
    ReferenceData,
}

#[derive(Debug, Clone, Copy, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "snake_case")]
pub enum PortObjectType {
    Sample,
    Reagent,
    Container,
    Consumable,
    Instrument,
    Measurement,
    Data,
    Material,
}

#[derive(Debug, Clone, Copy, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "snake_case")]
pub enum Cardinality {
    One,
    Many,
}

#[derive(Debug, Clone, Copy, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "snake_case")]
pub enum SampleEffect {
    Preserved,
    Aliquoted,
    Consumed,
    Terminal,
    StateChanged,
    Unknown,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[serde(deny_unknown_fields)]
pub struct OperationParameter {
    pub key: String,
    pub label: String,
    pub value_type: ParameterValueType,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub unit: Option<String>,
    pub required: bool,
    pub required_at: RequiredAt,
    pub value_source: ValueSource,
    #[serde(default)]
    pub constraints: Vec<ParameterConstraint>,
    pub description: String,
    #[serde(default)]
    pub evidence: Vec<EvidenceRef>,
}

#[derive(Debug, Clone, Copy, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "snake_case")]
pub enum ParameterValueType {
    Text,
    Number,
    Integer,
    Boolean,
}

impl ParameterValueType {
    pub fn is_numeric(self) -> bool {
        matches!(self, Self::Number | Self::Integer)
    }
}

#[derive(Debug, Clone, Copy, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "snake_case")]
pub enum RequiredAt {
    Draft,
    ProtocolFreeze,
    RunFreeze,
    Execution,
}

#[derive(Debug, Clone, Copy, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "snake_case")]
pub enum ValueSource {
    ProtocolFixed,
    DesignVariable,
    LabSop,
    VendorRecommended,
    ExampleOnly,
    RunActual,
    Unknown,
}

impl ValueSource {
    /// Only these sources may be treated as an exact method requirement.
    pub fn may_bind_as_planned(self) -> bool {
        matches!(self, Self::ProtocolFixed | Self::LabSop)
    }

    /// These sources may be shown as suggestions, never as confirmed values.
    pub fn is_suggestion_only(self) -> bool {
        matches!(self, Self::VendorRecommended | Self::ExampleOnly)
    }
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[serde(tag = "kind", rename_all = "snake_case")]
pub enum ParameterConstraint {
    Min { value: f64 },
    Max { value: f64 },
    OneOf { values: Vec<String> },
    MaxLength { value: usize },
    NonEmpty,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[serde(deny_unknown_fields)]
pub struct ExecutionSpec {
    pub time_scope: StepTimeScope,
    #[serde(default)]
    pub instrument_requirements: Vec<InstrumentRequirement>,
    #[serde(default)]
    pub completion_checks: Vec<CompletionCheck>,
    #[serde(default)]
    pub record_fields: Vec<RecordField>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[serde(deny_unknown_fields)]
pub struct InstrumentRequirement {
    pub requirement_id: String,
    pub class: String,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub profile_id: Option<String>,
    pub profile_version: Option<u32>,
    pub required: bool,
    #[serde(default)]
    pub capabilities: Vec<CapabilityRequirement>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[serde(deny_unknown_fields)]
pub struct CapabilityRequirement {
    pub key: String,
    pub constraint: CapabilityConstraint,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[serde(tag = "kind", rename_all = "snake_case")]
pub enum CapabilityConstraint {
    Exists,
    Min {
        value: f64,
        unit: Option<String>,
    },
    Max {
        value: f64,
        unit: Option<String>,
    },
    Range {
        min: f64,
        max: f64,
        unit: Option<String>,
    },
    OneOf {
        values: Vec<String>,
    },
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[serde(deny_unknown_fields)]
pub struct CompletionCheck {
    pub check_id: String,
    pub kind: CheckKind,
    pub description: String,
    pub blocking: bool,
}

#[derive(Debug, Clone, Copy, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "snake_case")]
pub enum CheckKind {
    Deterministic,
    Operational,
    Advisory,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[serde(deny_unknown_fields)]
pub struct RecordField {
    pub key: String,
    pub label: String,
    pub value_type: ParameterValueType,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub unit: Option<String>,
    pub required: bool,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[serde(deny_unknown_fields)]
pub struct ModuleKnowledge {
    #[serde(default)]
    pub controls: Vec<ControlSpec>,
    #[serde(default)]
    pub decision_points: Vec<DecisionPoint>,
    #[serde(default)]
    pub failure_modes: Vec<FailureMode>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub readout: Option<ReadoutMeaning>,
    #[serde(default)]
    pub evidence: Vec<EvidenceRef>,
    pub review_status: ReviewStatus,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[serde(deny_unknown_fields)]
pub struct ControlSpec {
    pub control_id: String,
    pub label: String,
    pub kind: ControlKind,
    pub purpose: String,
    pub required: bool,
}

#[derive(Debug, Clone, Copy, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "snake_case")]
pub enum ControlKind {
    Blank,
    Negative,
    Positive,
    Standard,
    Vehicle,
    Untreated,
    Unstained,
    Other,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[serde(deny_unknown_fields)]
pub struct DecisionPoint {
    pub question: String,
    pub evidence: String,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub continue_if: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub stop_if: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[serde(deny_unknown_fields)]
pub struct FailureMode {
    pub mode: String,
    pub symptom: String,
    pub impact: String,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub action: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[serde(deny_unknown_fields)]
pub struct ReadoutMeaning {
    pub raw: String,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub derived: Option<String>,
    pub not_implied: String,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[serde(deny_unknown_fields)]
pub struct EvidenceRef {
    pub evidence_id: String,
    pub kind: EvidenceKind,
    pub citation: String,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub url: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub locator: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub version: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub retrieved_at: Option<String>,
}

#[derive(Debug, Clone, Copy, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "snake_case")]
pub enum EvidenceKind {
    VendorManual,
    PeerReviewed,
    LabSop,
    Standard,
    InternalValidation,
    Other,
}

#[derive(Debug, Clone, Copy, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "snake_case")]
pub enum ReviewStatus {
    ResearchDraft,
    SourceChecked,
    ExpertReviewed,
    LabValidated,
}

impl ReviewStatus {
    pub fn rank(self) -> u8 {
        match self {
            Self::ResearchDraft => 0,
            Self::SourceChecked => 1,
            Self::ExpertReviewed => 2,
            Self::LabValidated => 3,
        }
    }

    /// Status is monotonic; `at_least` never upgrades a template by itself.
    pub fn at_least(self, required: ReviewStatus) -> bool {
        self.rank() >= required.rank()
    }
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub struct ContractIssue {
    pub code: String,
    pub path: String,
    pub message: String,
}

impl OperationModule {
    /// Structural checks only. Scientific correctness and instrument
    /// compatibility are deliberately out of scope for K1.
    pub fn validate_contract(&self) -> Result<(), Vec<ContractIssue>> {
        let mut issues = Vec::new();

        if self.schema_version != OPERATION_MODULE_SCHEMA_VERSION {
            issues.push(issue(
                "schema_version",
                "schema_version",
                "unsupported operation-module schema version",
            ));
        }
        validate_stable_id(
            &self.identity.module_id,
            "identity.module_id",
            "module_id",
            &mut issues,
        );
        if self.identity.version == 0 {
            issues.push(issue("version", "identity.version", "version must be >= 1"));
        }
        if self.identity.title.trim().is_empty() {
            issues.push(issue("title", "identity.title", "title must not be empty"));
        }
        if self.semantics.purpose.trim().is_empty() {
            issues.push(issue(
                "purpose",
                "semantics.purpose",
                "purpose must not be empty",
            ));
        }
        if self.semantics.sample_effects.is_empty() {
            issues.push(issue(
                "sample_effect",
                "semantics.sample_effects",
                "at least one sample effect is required",
            ));
        }
        if self
            .semantics
            .sample_effects
            .iter()
            .any(|effect| *effect == SampleEffect::Unknown)
            && self.semantics.sample_effects.len() > 1
        {
            issues.push(issue(
                "sample_effect_ambiguous",
                "semantics.sample_effects",
                "unknown cannot be combined with a known effect",
            ));
        }

        let mut port_ids = HashSet::new();
        validate_ports(
            &self.semantics.inputs,
            "semantics.inputs",
            &mut port_ids,
            &mut issues,
        );
        validate_ports(
            &self.semantics.outputs,
            "semantics.outputs",
            &mut port_ids,
            &mut issues,
        );

        let mut parameter_keys = HashSet::new();
        for (index, parameter) in self.parameters.iter().enumerate() {
            let path = format!("parameters[{index}]");
            validate_parameter(parameter, &path, &mut issues);
            if !parameter_keys.insert(parameter.key.as_str()) {
                issues.push(issue(
                    "duplicate_parameter",
                    format!("{path}.key"),
                    "parameter keys must be unique within a module",
                ));
            }
        }

        let mut requirement_ids = HashSet::new();
        for (index, requirement) in self.execution.instrument_requirements.iter().enumerate() {
            let path = format!("execution.instrument_requirements[{index}]");
            validate_stable_id(
                &requirement.requirement_id,
                &format!("{path}.requirement_id"),
                "requirement_id",
                &mut issues,
            );
            if requirement.class.trim().is_empty() {
                issues.push(issue(
                    "instrument_class",
                    format!("{path}.class"),
                    "instrument class must not be empty",
                ));
            }
            if requirement.profile_id.is_some() != requirement.profile_version.is_some() {
                issues.push(issue(
                    "profile_version",
                    format!("{path}.profile_version"),
                    "profile_id and profile_version must be supplied together",
                ));
            }
            if !requirement_ids.insert(requirement.requirement_id.as_str()) {
                issues.push(issue(
                    "duplicate_instrument_requirement",
                    format!("{path}.requirement_id"),
                    "instrument requirement IDs must be unique",
                ));
            }
            for (cap_index, capability) in requirement.capabilities.iter().enumerate() {
                if capability.key.trim().is_empty() {
                    issues.push(issue(
                        "capability_key",
                        format!("{path}.capabilities[{cap_index}].key"),
                        "capability key must not be empty",
                    ));
                }
            }
        }

        let mut check_ids = HashSet::new();
        for (index, check) in self.execution.completion_checks.iter().enumerate() {
            if check.description.trim().is_empty() {
                issues.push(issue(
                    "completion_check",
                    format!("execution.completion_checks[{index}].description"),
                    "completion check description must not be empty",
                ));
            }
            if !check_ids.insert(check.check_id.as_str()) {
                issues.push(issue(
                    "duplicate_completion_check",
                    format!("execution.completion_checks[{index}].check_id"),
                    "completion check IDs must be unique",
                ));
            }
        }

        let mut control_ids = HashSet::new();
        for (index, control) in self.knowledge.controls.iter().enumerate() {
            if control.label.trim().is_empty() || control.purpose.trim().is_empty() {
                issues.push(issue(
                    "control",
                    format!("knowledge.controls[{index}]"),
                    "control label and purpose must not be empty",
                ));
            }
            if !control_ids.insert(control.control_id.as_str()) {
                issues.push(issue(
                    "duplicate_control",
                    format!("knowledge.controls[{index}].control_id"),
                    "control IDs must be unique",
                ));
            }
        }

        if self.knowledge.review_status != ReviewStatus::ResearchDraft
            && self.knowledge.evidence.is_empty()
        {
            issues.push(issue(
                "evidence_required",
                "knowledge.evidence",
                "source_checked or higher requires at least one evidence reference",
            ));
        }
        for (index, evidence) in self.knowledge.evidence.iter().enumerate() {
            if evidence.citation.trim().is_empty() {
                issues.push(issue(
                    "evidence_citation",
                    format!("knowledge.evidence[{index}].citation"),
                    "evidence citation must not be empty",
                ));
            }
        }

        if issues.is_empty() {
            Ok(())
        } else {
            Err(issues)
        }
    }
}

fn validate_ports(
    ports: &[PortSpec],
    path: &str,
    ids: &mut HashSet<String>,
    issues: &mut Vec<ContractIssue>,
) {
    for (index, port) in ports.iter().enumerate() {
        validate_stable_id(
            &port.port_id,
            &format!("{path}[{index}].port_id"),
            "port_id",
            issues,
        );
        if port.description.trim().is_empty() {
            issues.push(issue(
                "port_description",
                format!("{path}[{index}].description"),
                "port description must not be empty",
            ));
        }
        if !ids.insert(port.port_id.clone()) {
            issues.push(issue(
                "duplicate_port",
                format!("{path}[{index}].port_id"),
                "port IDs must be unique within the input/output list",
            ));
        }
    }
}

fn validate_parameter(parameter: &OperationParameter, path: &str, issues: &mut Vec<ContractIssue>) {
    let mut chars = parameter.key.chars();
    let key_ok = chars.next().is_some_and(|c| c.is_ascii_lowercase())
        && chars.all(|c| c.is_ascii_alphanumeric() || c == '_')
        && parameter.key.len() <= 80;
    if !key_ok {
        issues.push(issue(
            "parameter_key",
            format!("{path}.key"),
            "parameter key must start lowercase and contain only lowercase letters, digits or underscore",
        ));
    }
    if parameter.label.trim().is_empty() {
        issues.push(issue(
            "parameter_label",
            format!("{path}.label"),
            "parameter label must not be empty",
        ));
    }
    if parameter.unit.is_some() && !parameter.value_type.is_numeric() {
        issues.push(issue(
            "unit_on_non_numeric",
            format!("{path}.unit"),
            "only number/integer parameters may declare a unit",
        ));
    }
    for (index, constraint) in parameter.constraints.iter().enumerate() {
        match constraint {
            ParameterConstraint::Min { value } | ParameterConstraint::Max { value } => {
                if !parameter.value_type.is_numeric() {
                    issues.push(issue(
                        "numeric_constraint",
                        format!("{path}.constraints[{index}]"),
                        "min/max constraints require a numeric parameter",
                    ));
                }
                if !value.is_finite() {
                    issues.push(issue(
                        "non_finite_constraint",
                        format!("{path}.constraints[{index}]"),
                        "constraint values must be finite",
                    ));
                }
            }
            ParameterConstraint::MaxLength { value } => {
                if !matches!(
                    parameter.value_type,
                    ParameterValueType::Text | ParameterValueType::Integer
                ) {
                    issues.push(issue(
                        "max_length_constraint",
                        format!("{path}.constraints[{index}]"),
                        "max_length applies to text or integer parameters",
                    ));
                }
                if *value == 0 {
                    issues.push(issue(
                        "max_length_zero",
                        format!("{path}.constraints[{index}]"),
                        "max_length must be greater than zero",
                    ));
                }
            }
            ParameterConstraint::OneOf { values } => {
                if values.is_empty() {
                    issues.push(issue(
                        "one_of_empty",
                        format!("{path}.constraints[{index}]"),
                        "one_of must contain at least one value",
                    ));
                }
            }
            ParameterConstraint::NonEmpty => {
                if !matches!(parameter.value_type, ParameterValueType::Text) {
                    issues.push(issue(
                        "non_empty_constraint",
                        format!("{path}.constraints[{index}]"),
                        "non_empty applies to text parameters",
                    ));
                }
            }
        }
    }
}

fn validate_stable_id(value: &str, path: &str, name: &str, issues: &mut Vec<ContractIssue>) {
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
            format!("{name} must be a lowercase stable ID (letters, digits, '.', '_' or '-')"),
        ));
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

#[cfg(test)]
mod tests {
    use super::*;

    fn evidence() -> EvidenceRef {
        EvidenceRef {
            evidence_id: "dojindo.ck04e".into(),
            kind: EvidenceKind::VendorManual,
            citation: "Dojindo CK04e".into(),
            url: Some("https://dojindo.com/products/manual/CK04e.pdf".into()),
            locator: Some("manual".into()),
            version: None,
            retrieved_at: None,
        }
    }

    fn parameter(
        key: &str,
        value_type: ParameterValueType,
        unit: Option<&str>,
    ) -> OperationParameter {
        OperationParameter {
            key: key.into(),
            label: key.into(),
            value_type,
            unit: unit.map(str::to_owned),
            required: true,
            required_at: RequiredAt::ProtocolFreeze,
            value_source: ValueSource::ProtocolFixed,
            constraints: vec![],
            description: "test".into(),
            evidence: vec![evidence()],
        }
    }

    fn module() -> OperationModule {
        OperationModule {
            schema_version: OPERATION_MODULE_SCHEMA_VERSION,
            identity: ModuleIdentity {
                module_id: "op.liquid_transfer".into(),
                version: 1,
                title: "Liquid transfer".into(),
                action_type: ActionType::LiquidTransfer,
                applicability: Applicability {
                    assay_families: vec!["cck8".into()],
                    ..Default::default()
                },
                variant_of: None,
            },
            semantics: ModuleSemantics {
                purpose: "Move a defined volume from a source to a destination.".into(),
                preconditions: vec!["source exists".into()],
                inputs: vec![
                    PortSpec {
                        port_id: "source".into(),
                        role: PortRole::SourceSample,
                        object_type: PortObjectType::Sample,
                        required: true,
                        cardinality: Cardinality::One,
                        description: "source sample".into(),
                    },
                    PortSpec {
                        port_id: "destination".into(),
                        role: PortRole::DestinationContainer,
                        object_type: PortObjectType::Container,
                        required: true,
                        cardinality: Cardinality::One,
                        description: "destination container".into(),
                    },
                ],
                outputs: vec![],
                sample_effects: vec![SampleEffect::Consumed],
            },
            parameters: vec![parameter("volume", ParameterValueType::Number, Some("uL"))],
            execution: ExecutionSpec {
                time_scope: StepTimeScope::Once,
                instrument_requirements: vec![InstrumentRequirement {
                    requirement_id: "pipette".into(),
                    class: "pipette".into(),
                    profile_id: None,
                    profile_version: None,
                    required: true,
                    capabilities: vec![CapabilityRequirement {
                        key: "volume_range".into(),
                        constraint: CapabilityConstraint::Range {
                            min: 1.0,
                            max: 1000.0,
                            unit: Some("uL".into()),
                        },
                    }],
                }],
                completion_checks: vec![],
                record_fields: vec![],
            },
            knowledge: ModuleKnowledge {
                controls: vec![],
                decision_points: vec![],
                failure_modes: vec![],
                readout: None,
                evidence: vec![evidence()],
                review_status: ReviewStatus::SourceChecked,
            },
        }
    }

    #[test]
    fn minimal_contract_is_structurally_valid() {
        assert!(module().validate_contract().is_ok());
    }

    #[test]
    fn module_references_are_exact_version_only() {
        let first = ModuleRef {
            module_id: "op.liquid_transfer".into(),
            version: 1,
        };
        let second = ModuleRef {
            module_id: "op.liquid_transfer".into(),
            version: 2,
        };
        assert!(first.matches_exact(&first));
        assert!(!first.matches_exact(&second));
    }

    #[test]
    fn unknown_sample_effect_cannot_be_mixed_with_known_effect() {
        let mut value = module();
        value.semantics.sample_effects = vec![SampleEffect::Unknown, SampleEffect::Consumed];
        let errors = value.validate_contract().unwrap_err();
        assert!(errors
            .iter()
            .any(|issue| issue.code == "sample_effect_ambiguous"));
    }

    #[test]
    fn source_checked_requires_evidence() {
        let mut value = module();
        value.knowledge.evidence.clear();
        let errors = value.validate_contract().unwrap_err();
        assert!(errors.iter().any(|issue| issue.code == "evidence_required"));
    }

    #[test]
    fn unit_is_rejected_on_text_parameter() {
        let mut value = module();
        value.parameters = vec![parameter("label", ParameterValueType::Text, Some("uL"))];
        let errors = value.validate_contract().unwrap_err();
        assert!(errors
            .iter()
            .any(|issue| issue.code == "unit_on_non_numeric"));
    }

    #[test]
    fn duplicate_parameter_keys_are_rejected() {
        let mut value = module();
        value
            .parameters
            .push(parameter("volume", ParameterValueType::Number, Some("uL")));
        let errors = value.validate_contract().unwrap_err();
        assert!(errors
            .iter()
            .any(|issue| issue.code == "duplicate_parameter"));
    }

    #[test]
    fn profile_id_requires_exact_profile_version() {
        let mut value = module();
        value.execution.instrument_requirements[0].profile_id = Some("multiskan_skyhigh".into());
        let errors = value.validate_contract().unwrap_err();
        assert!(errors.iter().any(|issue| issue.code == "profile_version"));
    }

    #[test]
    fn serde_round_trip_preserves_source_and_time_scope() {
        let value = module();
        let json = serde_json::to_value(&value).unwrap();
        assert_eq!(json["parameters"][0]["value_source"], "protocol_fixed");
        assert_eq!(json["execution"]["time_scope"], "once");
        let decoded: OperationModule = serde_json::from_value(json).unwrap();
        assert_eq!(decoded, value);
    }

    #[test]
    fn suggestions_are_not_planned_bindings() {
        assert!(ValueSource::ProtocolFixed.may_bind_as_planned());
        assert!(ValueSource::LabSop.may_bind_as_planned());
        assert!(!ValueSource::VendorRecommended.may_bind_as_planned());
        assert!(!ValueSource::ExampleOnly.may_bind_as_planned());
        assert!(ValueSource::VendorRecommended.is_suggestion_only());
    }

    #[test]
    fn review_status_is_monotonic() {
        assert!(ReviewStatus::LabValidated.at_least(ReviewStatus::SourceChecked));
        assert!(!ReviewStatus::ResearchDraft.at_least(ReviewStatus::SourceChecked));
    }
}
