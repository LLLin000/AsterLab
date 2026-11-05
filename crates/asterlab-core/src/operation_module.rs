//! K1.1 draft contract for reusable operation modules.
//!
//! This module is intentionally not wired into the CLI, Protocol preview or
//! Run runtime. It is a reviewable schema plus structural checks; a later
//! catalog compiler must map it onto `ProtocolBlockVersion` and `BlockUse`.
//!
//! Scientific values are never inferred here. A missing source or an unknown
//! capability stays explicit.

use crate::protocol_preview::StepTimeScope;
use serde::{Deserialize, Serialize};
use serde_json::Value;
use std::collections::HashSet;

pub const OPERATION_MODULE_SCHEMA_VERSION: u32 = 1;

/// Stable identity of a definition in the built-in or imported knowledge
/// catalog. This is not a project database ID.
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[serde(deny_unknown_fields)]
pub struct CatalogModuleRef {
    pub module_id: String,
    pub version: u32,
}

impl CatalogModuleRef {
    /// Version ranges and "latest" are deliberately not supported.
    pub fn matches_exact(&self, other: &CatalogModuleRef) -> bool {
        self.module_id == other.module_id && self.version == other.version
    }
}

/// Identity of a project-local, immutable block version. The catalog compiler
/// establishes this mapping; a catalog ID must never replace it.
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[serde(deny_unknown_fields)]
pub struct ProtocolBlockVersionRef {
    pub block_id: String,
    pub version_id: String,
}

#[derive(Debug, Clone, Copy, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "snake_case")]
pub enum ModuleOrigin {
    Imported,
    Derived,
    LocalCustom,
}

/// A project-local instance of a module. Two projects can import the same
/// catalog version while owning different block versions.
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[serde(deny_unknown_fields)]
pub struct ModuleBinding {
    /// Present for imported definitions; derived/custom definitions use
    /// `based_on` or no catalog source.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub catalog: Option<CatalogModuleRef>,
    pub block_version: ProtocolBlockVersionRef,
    pub origin: ModuleOrigin,
    /// The catalog version this local definition was derived from.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub based_on: Option<CatalogModuleRef>,
}

impl ModuleBinding {
    pub fn validate(&self) -> Result<(), Vec<ContractIssue>> {
        let mut issues = Vec::new();
        validate_stable_id(
            &self.block_version.block_id,
            "block_version.block_id",
            "block_id",
            &mut issues,
        );
        validate_stable_id(
            &self.block_version.version_id,
            "block_version.version_id",
            "version_id",
            &mut issues,
        );
        if let Some(catalog) = &self.catalog {
            validate_catalog_ref(catalog, "catalog", &mut issues);
        }
        if let Some(based_on) = &self.based_on {
            validate_catalog_ref(based_on, "based_on", &mut issues);
        }
        match self.origin {
            ModuleOrigin::Imported => {
                if self.catalog.is_none() {
                    issues.push(issue(
                        "imported_without_catalog",
                        "catalog",
                        "imported binding must name its catalog source",
                    ));
                }
                if self.based_on.is_some() {
                    issues.push(issue(
                        "imported_with_based_on",
                        "based_on",
                        "imported binding is not a local derivation",
                    ));
                }
            }
            ModuleOrigin::Derived => {
                if self.based_on.is_none() {
                    issues.push(issue(
                        "derived_without_based_on",
                        "based_on",
                        "derived binding must name the catalog version it derives from",
                    ));
                }
                if self.catalog.is_some() {
                    issues.push(issue(
                        "derived_with_catalog",
                        "catalog",
                        "derived binding is local, not a catalog definition",
                    ));
                }
            }
            ModuleOrigin::LocalCustom => {
                if self.catalog.is_some() || self.based_on.is_some() {
                    issues.push(issue(
                        "custom_with_catalog_source",
                        "origin",
                        "local custom binding cannot claim a catalog source",
                    ));
                }
            }
        }
        finish(issues)
    }
}

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
    /// Catalog identity; project-local block identity lives in `ModuleBinding`.
    pub catalog: CatalogModuleRef,
    pub title: String,
    pub action_type: ActionType,
    pub applicability: Applicability,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub variant_of: Option<CatalogModuleRef>,
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

impl ActionType {
    /// Actions that must produce a material output to be meaningful.
    pub fn requires_material_output(self) -> bool {
        matches!(
            self,
            Self::LiquidTransfer | Self::Inoculation | Self::Sampling
        )
    }
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
pub struct ModuleSemantics {
    pub purpose: String,
    #[serde(default)]
    pub preconditions: Vec<String>,
    #[serde(default)]
    pub inputs: Vec<PortSpec>,
    #[serde(default)]
    pub outputs: Vec<PortSpec>,
    #[serde(default)]
    pub sample_transitions: Vec<SampleTransition>,
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
    DestinationSample,
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

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[serde(deny_unknown_fields)]
pub struct SampleTransition {
    pub transition_id: String,
    pub input_port_id: String,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub output_port_id: Option<String>,
    pub effect: SampleEffect,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub note: Option<String>,
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

impl SampleEffect {
    pub fn requires_output(self) -> bool {
        matches!(self, Self::Aliquoted | Self::StateChanged)
    }
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
    /// Allowed sources for this parameter; a binding still needs its own proof.
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
    /// Classification only. Source category alone never authorizes a binding;
    /// use `ParameterBinding::is_plan_ready` for that decision.
    pub fn is_planned_source(self) -> bool {
        matches!(self, Self::ProtocolFixed | Self::LabSop)
    }

    pub fn is_suggestion_only(self) -> bool {
        matches!(self, Self::VendorRecommended | Self::ExampleOnly)
    }
}

/// A concrete value chosen for a parameter. The source category is separate
/// from the evidence that this particular value came from a specific source
/// version and was confirmed.
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[serde(deny_unknown_fields)]
pub struct ParameterBinding {
    pub parameter_key: String,
    pub value: Value,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub unit: Option<String>,
    pub source: ValueSource,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub source_reference: Option<String>,
    pub confirmed: bool,
}

impl ParameterBinding {
    pub fn validate_against(
        &self,
        parameter: &OperationParameter,
    ) -> Result<(), Vec<ContractIssue>> {
        let mut issues = Vec::new();
        if self.parameter_key != parameter.key {
            issues.push(issue(
                "binding_key",
                "parameter_key",
                "binding key does not match the parameter definition",
            ));
        }
        if self.unit != parameter.unit {
            issues.push(issue(
                "binding_unit",
                "unit",
                "binding unit must match the parameter unit exactly; no implicit conversion",
            ));
        }
        if self.source == ValueSource::Unknown && !self.value.is_null() {
            issues.push(issue(
                "unknown_with_value",
                "value",
                "an unknown binding must not carry a fabricated value",
            ));
        }
        if matches!(
            self.source,
            ValueSource::ProtocolFixed | ValueSource::LabSop
        ) && self
            .source_reference
            .as_deref()
            .is_none_or(|value| value.trim().is_empty())
        {
            issues.push(issue(
                "binding_source_reference",
                "source_reference",
                "protocol_fixed and lab_sop bindings require a specific source reference",
            ));
        }
        validate_binding_value(parameter, &self.value, &mut issues);
        finish(issues)
    }

    /// A planning binding is ready only when the value is valid, confirmed,
    /// and tied to a specific protocol/SOP source; the enum alone is not proof.
    pub fn is_plan_ready(&self, parameter: &OperationParameter) -> bool {
        self.confirmed
            && self.source.is_planned_source()
            && self
                .source_reference
                .as_deref()
                .is_some_and(|value| !value.trim().is_empty())
            && self.validate_against(parameter).is_ok()
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
    /// Default for a Protocol step. The compiled step stores the one effective
    /// `time_scope`; this field is not a second live value.
    pub default_time_scope: StepTimeScope,
    /// Additional time scopes the compiler may choose. Empty means only the
    /// default is allowed.
    #[serde(default)]
    pub allowed_time_scopes: Vec<StepTimeScope>,
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

    /// Status comparison only; this is not an audit approval.
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
        validate_catalog_ref(&self.identity.catalog, "identity.catalog", &mut issues);
        if self.identity.catalog.version == 0 {
            issues.push(issue(
                "version",
                "identity.catalog.version",
                "version must be >= 1",
            ));
        }
        if let Some(variant_of) = &self.identity.variant_of {
            validate_catalog_ref(variant_of, "identity.variant_of", &mut issues);
            if variant_of.matches_exact(&self.identity.catalog) {
                issues.push(issue(
                    "variant_self_reference",
                    "identity.variant_of",
                    "a module cannot be its own variant",
                ));
            }
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

        validate_ports(
            &self.semantics.inputs,
            "semantics.inputs",
            &mut HashSet::new(),
            &mut issues,
        );
        let mut output_ids = HashSet::new();
        validate_ports(
            &self.semantics.outputs,
            "semantics.outputs",
            &mut output_ids,
            &mut issues,
        );
        let input_ids = self
            .semantics
            .inputs
            .iter()
            .map(|port| port.port_id.as_str())
            .collect::<HashSet<_>>();
        validate_transitions(self, &input_ids, &output_ids, &mut issues);

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

        validate_time_scopes(self, &mut issues);

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
                validate_capability(
                    &capability.constraint,
                    &format!("{path}.capabilities[{cap_index}].constraint"),
                    &mut issues,
                );
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

        finish(issues)
    }
}

fn validate_time_scopes(module: &OperationModule, issues: &mut Vec<ContractIssue>) {
    let mut seen = Vec::new();
    for (index, scope) in module.execution.allowed_time_scopes.iter().enumerate() {
        if seen.contains(scope) {
            issues.push(issue(
                "duplicate_time_scope",
                format!("execution.allowed_time_scopes[{index}]"),
                "allowed time scopes must not repeat",
            ));
        } else {
            seen.push(*scope);
        }
    }
    if !module.execution.allowed_time_scopes.is_empty()
        && !seen.contains(&module.execution.default_time_scope)
    {
        issues.push(issue(
            "time_scope_not_allowed",
            "execution.default_time_scope",
            "default time scope must be one of the allowed scopes",
        ));
    }
}

fn validate_transitions(
    module: &OperationModule,
    input_ids: &HashSet<&str>,
    output_ids: &HashSet<String>,
    issues: &mut Vec<ContractIssue>,
) {
    let mut transition_ids = HashSet::new();
    let mut referenced_inputs = HashSet::new();
    let mut referenced_outputs = HashSet::new();
    let mut has_unknown = false;

    for (index, transition) in module.semantics.sample_transitions.iter().enumerate() {
        let path = format!("semantics.sample_transitions[{index}]");
        validate_stable_id(
            &transition.transition_id,
            &format!("{path}.transition_id"),
            "transition_id",
            issues,
        );
        if !transition_ids.insert(transition.transition_id.as_str()) {
            issues.push(issue(
                "duplicate_transition",
                format!("{path}.transition_id"),
                "transition IDs must be unique",
            ));
        }
        if !input_ids.contains(transition.input_port_id.as_str()) {
            issues.push(issue(
                "transition_input",
                format!("{path}.input_port_id"),
                "transition input must reference an input port",
            ));
        } else {
            referenced_inputs.insert(transition.input_port_id.as_str());
        }
        if let Some(output_port_id) = &transition.output_port_id {
            if !output_ids.contains(output_port_id) {
                issues.push(issue(
                    "transition_output",
                    format!("{path}.output_port_id"),
                    "transition output must reference an output port",
                ));
            } else {
                referenced_outputs.insert(output_port_id.as_str());
            }
        }
        if transition.effect.requires_output() && transition.output_port_id.is_none() {
            issues.push(issue(
                "transition_output_required",
                format!("{path}.effect"),
                "aliquoted and state_changed effects require an output port",
            ));
        }
        if transition.effect == SampleEffect::Unknown {
            has_unknown = true;
        }
    }

    if module.semantics.sample_transitions.is_empty() {
        issues.push(issue(
            "sample_transition",
            "semantics.sample_transitions",
            "at least one sample transition is required",
        ));
    }
    if has_unknown && module.semantics.sample_transitions.len() > 1 {
        issues.push(issue(
            "sample_effect_ambiguous",
            "semantics.sample_transitions",
            "unknown cannot be combined with a known effect",
        ));
    }

    for port in &module.semantics.inputs {
        if matches!(
            port.object_type,
            PortObjectType::Sample | PortObjectType::Reagent | PortObjectType::Material
        ) && !referenced_inputs.contains(port.port_id.as_str())
        {
            issues.push(issue(
                "unreferenced_input",
                format!("semantics.inputs.{}", port.port_id),
                "sample/reagent/material inputs need a sample transition",
            ));
        }
    }
    for port in &module.semantics.outputs {
        if matches!(
            port.object_type,
            PortObjectType::Sample | PortObjectType::Material
        ) && !referenced_outputs.contains(port.port_id.as_str())
        {
            issues.push(issue(
                "unreferenced_output",
                format!("semantics.outputs.{}", port.port_id),
                "sample/material outputs need a sample transition",
            ));
        }
    }

    let has_material_output = module.semantics.outputs.iter().any(|port| {
        matches!(
            port.object_type,
            PortObjectType::Sample | PortObjectType::Material
        )
    });
    if module.identity.action_type.requires_material_output() && !has_material_output {
        issues.push(issue(
            "material_output_required",
            "semantics.outputs",
            "this action requires at least one material output",
        ));
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

fn validate_catalog_ref(catalog: &CatalogModuleRef, path: &str, issues: &mut Vec<ContractIssue>) {
    validate_stable_id(
        &catalog.module_id,
        &format!("{path}.module_id"),
        "module_id",
        issues,
    );
    if catalog.version == 0 {
        issues.push(issue(
            "version",
            format!("{path}.version"),
            "version must be >= 1",
        ));
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

    let mut min_values = Vec::new();
    let mut max_values = Vec::new();
    for (index, constraint) in parameter.constraints.iter().enumerate() {
        let constraint_path = format!("{path}.constraints[{index}]");
        match constraint {
            ParameterConstraint::Min { value } | ParameterConstraint::Max { value } => {
                if !parameter.value_type.is_numeric() {
                    issues.push(issue(
                        "numeric_constraint",
                        &constraint_path,
                        "min/max constraints require a numeric parameter",
                    ));
                }
                if !value.is_finite() {
                    issues.push(issue(
                        "non_finite_constraint",
                        &constraint_path,
                        "constraint values must be finite",
                    ));
                }
                if matches!(constraint, ParameterConstraint::Min { .. }) {
                    min_values.push(*value);
                } else {
                    max_values.push(*value);
                }
            }
            ParameterConstraint::MaxLength { value } => {
                if !matches!(parameter.value_type, ParameterValueType::Text) {
                    issues.push(issue(
                        "max_length_constraint",
                        &constraint_path,
                        "max_length applies only to text parameters",
                    ));
                }
                if *value == 0 {
                    issues.push(issue(
                        "max_length_zero",
                        &constraint_path,
                        "max_length must be greater than zero",
                    ));
                }
            }
            ParameterConstraint::OneOf { values } => {
                if values.is_empty() {
                    issues.push(issue(
                        "one_of_empty",
                        &constraint_path,
                        "one_of must contain at least one value",
                    ));
                }
                for value in values {
                    if !allowed_value_parses(parameter.value_type, value) {
                        issues.push(issue(
                            "one_of_type",
                            &constraint_path,
                            "one_of values must match the parameter type",
                        ));
                        break;
                    }
                }
            }
            ParameterConstraint::NonEmpty => {
                if !matches!(parameter.value_type, ParameterValueType::Text) {
                    issues.push(issue(
                        "non_empty_constraint",
                        &constraint_path,
                        "non_empty applies to text parameters",
                    ));
                }
            }
        }
    }
    if min_values
        .iter()
        .any(|min| max_values.iter().any(|max| min > max))
    {
        issues.push(issue(
            "parameter_range",
            format!("{path}.constraints"),
            "minimum constraint is greater than maximum constraint",
        ));
    }
}

fn validate_capability(
    constraint: &CapabilityConstraint,
    path: &str,
    issues: &mut Vec<ContractIssue>,
) {
    match constraint {
        CapabilityConstraint::Exists => {}
        CapabilityConstraint::Min { value, .. } | CapabilityConstraint::Max { value, .. } => {
            if !value.is_finite() {
                issues.push(issue(
                    "non_finite_capability",
                    path,
                    "capability bounds must be finite",
                ));
            }
        }
        CapabilityConstraint::Range { min, max, .. } => {
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
                    "capability minimum is greater than maximum",
                ));
            }
        }
        CapabilityConstraint::OneOf { values } => {
            if values.is_empty() {
                issues.push(issue(
                    "capability_one_of_empty",
                    path,
                    "capability one_of must contain at least one value",
                ));
            }
        }
    }
}

fn validate_binding_value(
    parameter: &OperationParameter,
    value: &Value,
    issues: &mut Vec<ContractIssue>,
) {
    let valid_type = match parameter.value_type {
        ParameterValueType::Text => value.as_str().is_some(),
        ParameterValueType::Number => value.as_f64().is_some_and(f64::is_finite),
        ParameterValueType::Integer => value.as_i64().is_some() || value.as_u64().is_some(),
        ParameterValueType::Boolean => value.as_bool().is_some(),
    };
    if !valid_type {
        issues.push(issue(
            "binding_type",
            "value",
            "binding value does not match the parameter type",
        ));
        return;
    }

    for constraint in &parameter.constraints {
        match constraint {
            ParameterConstraint::Min { value: minimum } => {
                if numeric_value(parameter.value_type, value).is_none_or(|actual| actual < *minimum)
                {
                    issues.push(issue(
                        "binding_min",
                        "value",
                        "binding value is below the minimum",
                    ));
                }
            }
            ParameterConstraint::Max { value: maximum } => {
                if numeric_value(parameter.value_type, value).is_none_or(|actual| actual > *maximum)
                {
                    issues.push(issue(
                        "binding_max",
                        "value",
                        "binding value is above the maximum",
                    ));
                }
            }
            ParameterConstraint::OneOf { values } => {
                if !values
                    .iter()
                    .any(|allowed| value_matches_allowed(parameter.value_type, value, allowed))
                {
                    issues.push(issue(
                        "binding_one_of",
                        "value",
                        "binding value is not one of the allowed values",
                    ));
                }
            }
            ParameterConstraint::MaxLength { value: maximum } => {
                if value
                    .as_str()
                    .is_none_or(|text| text.chars().count() > *maximum)
                {
                    issues.push(issue(
                        "binding_max_length",
                        "value",
                        "binding text is longer than the maximum",
                    ));
                }
            }
            ParameterConstraint::NonEmpty => {
                if value.as_str().is_none_or(|text| text.is_empty()) {
                    issues.push(issue(
                        "binding_non_empty",
                        "value",
                        "binding text must not be empty",
                    ));
                }
            }
        }
    }
}

fn allowed_value_parses(value_type: ParameterValueType, value: &str) -> bool {
    match value_type {
        ParameterValueType::Text => !value.is_empty(),
        ParameterValueType::Number => value.parse::<f64>().is_ok_and(f64::is_finite),
        ParameterValueType::Integer => value.parse::<i64>().is_ok(),
        ParameterValueType::Boolean => matches!(value, "true" | "false"),
    }
}

fn value_matches_allowed(value_type: ParameterValueType, value: &Value, allowed: &str) -> bool {
    match value_type {
        ParameterValueType::Text => value.as_str() == Some(allowed),
        ParameterValueType::Number => {
            let Ok(allowed) = allowed.parse::<f64>() else {
                return false;
            };
            value.as_f64().is_some_and(|actual| actual == allowed)
        }
        ParameterValueType::Integer => {
            let Ok(allowed) = allowed.parse::<i64>() else {
                return false;
            };
            value.as_i64().is_some_and(|actual| actual == allowed)
                || value
                    .as_u64()
                    .is_some_and(|actual| actual == allowed as u64)
        }
        ParameterValueType::Boolean => {
            let allowed = matches!(allowed, "true");
            value.as_bool() == Some(allowed)
        }
    }
}

fn numeric_value(value_type: ParameterValueType, value: &Value) -> Option<f64> {
    match value_type {
        ParameterValueType::Number => value.as_f64().filter(|value| value.is_finite()),
        ParameterValueType::Integer => value
            .as_i64()
            .map(|value| value as f64)
            .or_else(|| value.as_u64().map(|value| value as f64)),
        _ => None,
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
    use serde_json::json;

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

    fn catalog() -> CatalogModuleRef {
        CatalogModuleRef {
            module_id: "op.liquid_transfer".into(),
            version: 1,
        }
    }

    fn module() -> OperationModule {
        OperationModule {
            schema_version: OPERATION_MODULE_SCHEMA_VERSION,
            identity: ModuleIdentity {
                catalog: catalog(),
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
                outputs: vec![PortSpec {
                    port_id: "transferred".into(),
                    role: PortRole::DestinationSample,
                    object_type: PortObjectType::Sample,
                    required: true,
                    cardinality: Cardinality::One,
                    description: "transferred material".into(),
                }],
                sample_transitions: vec![SampleTransition {
                    transition_id: "source_to_transferred".into(),
                    input_port_id: "source".into(),
                    output_port_id: Some("transferred".into()),
                    effect: SampleEffect::Aliquoted,
                    note: None,
                }],
            },
            parameters: vec![parameter("volume", ParameterValueType::Number, Some("uL"))],
            execution: ExecutionSpec {
                default_time_scope: StepTimeScope::Once,
                allowed_time_scopes: vec![StepTimeScope::Once],
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

    fn binding() -> ParameterBinding {
        ParameterBinding {
            parameter_key: "volume".into(),
            value: json!(10.0),
            unit: Some("uL".into()),
            source: ValueSource::ProtocolFixed,
            source_reference: Some("protocol.version.1".into()),
            confirmed: true,
        }
    }

    #[test]
    fn minimal_contract_is_structurally_valid() {
        assert!(module().validate_contract().is_ok());
    }

    #[test]
    fn catalog_references_are_exact_version_only() {
        let first = CatalogModuleRef {
            module_id: "op.liquid_transfer".into(),
            version: 1,
        };
        let second = CatalogModuleRef {
            module_id: "op.liquid_transfer".into(),
            version: 2,
        };
        assert!(first.matches_exact(&first));
        assert!(!first.matches_exact(&second));
    }

    #[test]
    fn module_binding_keeps_catalog_and_project_ids_separate() {
        let first = ModuleBinding {
            catalog: Some(catalog()),
            block_version: ProtocolBlockVersionRef {
                block_id: "block-a173".into(),
                version_id: "version-a1".into(),
            },
            origin: ModuleOrigin::Imported,
            based_on: None,
        };
        let second = ModuleBinding {
            catalog: Some(catalog()),
            block_version: ProtocolBlockVersionRef {
                block_id: "block-b682".into(),
                version_id: "version-b1".into(),
            },
            origin: ModuleOrigin::Imported,
            based_on: None,
        };
        assert!(first.validate().is_ok());
        assert!(second.validate().is_ok());
        assert_ne!(first.block_version.block_id, second.block_version.block_id);
        assert_eq!(first.catalog, second.catalog);
    }

    #[test]
    fn imported_binding_requires_catalog_source() {
        let value = ModuleBinding {
            catalog: None,
            block_version: ProtocolBlockVersionRef {
                block_id: "block-a173".into(),
                version_id: "version-a1".into(),
            },
            origin: ModuleOrigin::Imported,
            based_on: None,
        };
        let errors = value.validate().unwrap_err();
        assert!(errors
            .iter()
            .any(|issue| issue.code == "imported_without_catalog"));
    }

    #[test]
    fn derived_binding_requires_source_catalog_version() {
        let value = ModuleBinding {
            catalog: None,
            block_version: ProtocolBlockVersionRef {
                block_id: "block-local".into(),
                version_id: "version-local".into(),
            },
            origin: ModuleOrigin::Derived,
            based_on: None,
        };
        let errors = value.validate().unwrap_err();
        assert!(errors
            .iter()
            .any(|issue| issue.code == "derived_without_based_on"));
    }

    #[test]
    fn unknown_transition_cannot_be_mixed_with_known_effect() {
        let mut value = module();
        value.semantics.sample_transitions[0].effect = SampleEffect::Unknown;
        value.semantics.sample_transitions.push(SampleTransition {
            transition_id: "second".into(),
            input_port_id: "source".into(),
            output_port_id: Some("transferred".into()),
            effect: SampleEffect::Consumed,
            note: None,
        });
        let errors = value.validate_contract().unwrap_err();
        assert!(errors
            .iter()
            .any(|issue| issue.code == "sample_effect_ambiguous"));
    }

    #[test]
    fn liquid_transfer_without_material_output_is_rejected() {
        let mut value = module();
        value.semantics.outputs.clear();
        value.semantics.sample_transitions[0].effect = SampleEffect::Consumed;
        value.semantics.sample_transitions[0].output_port_id = None;
        let errors = value.validate_contract().unwrap_err();
        assert!(errors
            .iter()
            .any(|issue| issue.code == "material_output_required"));
    }

    #[test]
    fn transition_must_reference_existing_ports() {
        let mut value = module();
        value.semantics.sample_transitions[0].output_port_id = Some("missing".into());
        let errors = value.validate_contract().unwrap_err();
        assert!(errors.iter().any(|issue| issue.code == "transition_output"));
    }

    #[test]
    fn time_scope_must_be_in_allowed_set() {
        let mut value = module();
        value.execution.allowed_time_scopes = vec![StepTimeScope::PerAssay];
        let errors = value.validate_contract().unwrap_err();
        assert!(errors
            .iter()
            .any(|issue| issue.code == "time_scope_not_allowed"));
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
    fn parameter_min_must_not_exceed_max() {
        let mut value = module();
        value.parameters[0].constraints = vec![
            ParameterConstraint::Min { value: 10.0 },
            ParameterConstraint::Max { value: 5.0 },
        ];
        let errors = value.validate_contract().unwrap_err();
        assert!(errors.iter().any(|issue| issue.code == "parameter_range"));
    }

    #[test]
    fn one_of_values_must_match_parameter_type() {
        let mut value = module();
        value.parameters[0].constraints = vec![ParameterConstraint::OneOf {
            values: vec!["not-a-number".into()],
        }];
        let errors = value.validate_contract().unwrap_err();
        assert!(errors.iter().any(|issue| issue.code == "one_of_type"));
    }

    #[test]
    fn capability_range_must_be_ordered() {
        let mut value = module();
        value.execution.instrument_requirements[0].capabilities[0].constraint =
            CapabilityConstraint::Range {
                min: 10.0,
                max: 1.0,
                unit: Some("uL".into()),
            };
        let errors = value.validate_contract().unwrap_err();
        assert!(errors.iter().any(|issue| issue.code == "capability_range"));
    }

    #[test]
    fn profile_id_requires_exact_profile_version() {
        let mut value = module();
        value.execution.instrument_requirements[0].profile_id = Some("multiskan_skyhigh".into());
        let errors = value.validate_contract().unwrap_err();
        assert!(errors.iter().any(|issue| issue.code == "profile_version"));
    }

    #[test]
    fn suggestions_are_not_planned_bindings() {
        assert!(ValueSource::ProtocolFixed.is_planned_source());
        assert!(ValueSource::LabSop.is_planned_source());
        assert!(!ValueSource::VendorRecommended.is_planned_source());
        assert!(!ValueSource::ExampleOnly.is_planned_source());
        assert!(ValueSource::VendorRecommended.is_suggestion_only());
    }

    #[test]
    fn lab_sop_category_alone_does_not_authorize_a_binding() {
        let parameter = module().parameters.remove(0);
        let mut value = binding();
        value.source = ValueSource::LabSop;
        value.source_reference = None;
        assert!(!value.is_plan_ready(&parameter));
        assert!(value.validate_against(&parameter).is_err());
    }

    #[test]
    fn unknown_binding_cannot_carry_a_value() {
        let parameter = module().parameters.remove(0);
        let mut value = binding();
        value.source = ValueSource::Unknown;
        value.source_reference = None;
        value.confirmed = false;
        let errors = value.validate_against(&parameter).unwrap_err();
        assert!(errors
            .iter()
            .any(|issue| issue.code == "unknown_with_value"));
    }

    #[test]
    fn confirmed_protocol_binding_is_plan_ready() {
        let parameter = module().parameters.remove(0);
        assert!(binding().is_plan_ready(&parameter));
    }

    #[test]
    fn binding_rejects_mismatched_unit_and_range() {
        let mut parameter = module().parameters.remove(0);
        parameter.constraints = vec![ParameterConstraint::Min { value: 1.0 }];
        let mut value = binding();
        value.unit = Some("mL".into());
        assert!(value.validate_against(&parameter).is_err());
        value.unit = Some("uL".into());
        value.value = json!(0.0);
        let errors = value.validate_against(&parameter).unwrap_err();
        assert!(errors
            .iter()
            .any(|issue| issue.code == "parameter_range" || issue.code == "binding_min"));
    }

    #[test]
    fn serde_round_trip_preserves_source_and_time_scope() {
        let value = module();
        let json = serde_json::to_value(&value).unwrap();
        assert_eq!(json["parameters"][0]["value_source"], "protocol_fixed");
        assert_eq!(json["execution"]["default_time_scope"], "once");
        let decoded: OperationModule = serde_json::from_value(json).unwrap();
        assert_eq!(decoded, value);
    }

    #[test]
    fn review_status_is_monotonic() {
        assert!(ReviewStatus::LabValidated.at_least(ReviewStatus::SourceChecked));
        assert!(!ReviewStatus::ResearchDraft.at_least(ReviewStatus::SourceChecked));
    }
}
