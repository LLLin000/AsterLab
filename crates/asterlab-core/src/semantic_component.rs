//! S1-A typed reference and binding contract.
//!
//! Pure Core types and validation. This module does not read SQLite, does not
//! create a component table, and is not wired into CLI or UI. `BlockUse`
//! remains the authoritative operation/composition instance; ToolAction and
//! Resource use their own invocation/binding shapes.

use crate::operation_module::{ParameterBinding, ParameterValueType, ValueSource};
use serde::{Deserialize, Serialize};
use serde_json::Value;

pub const SEMANTIC_COMPONENT_SCHEMA_VERSION: u32 = 1;

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq, Hash)]
#[serde(tag = "kind", rename_all = "snake_case")]
pub enum ProtocolScope {
    Assay { assay_id: String },
    PlannedOutput { planned_output_id: String },
    Unscoped,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(deny_unknown_fields)]
pub struct QualifiedStepRef {
    pub protocol_version_id: String,
    pub scope: ProtocolScope,
    pub step_path: String,
}

impl ProtocolScope {
    pub fn key(&self) -> String {
        scope_key(self)
    }
}

impl QualifiedStepRef {
    pub fn target_key(&self) -> String {
        format!(
            "{}::{}::{}",
            self.protocol_version_id,
            scope_key(&self.scope),
            self.step_path
        )
    }
}

fn scope_key(scope: &ProtocolScope) -> String {
    match scope {
        ProtocolScope::Assay { assay_id } => format!("assay:{assay_id}"),
        ProtocolScope::PlannedOutput { planned_output_id } => {
            format!("planned_output:{planned_output_id}")
        }
        ProtocolScope::Unscoped => "unscoped".to_owned(),
    }
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(tag = "kind", rename_all = "snake_case")]
pub enum BindingOwner {
    Definition {
        block_version_id: String,
    },
    ProtocolPlan {
        protocol_version_id: String,
    },
    RunPlan {
        run_id: String,
    },
    RunExecution {
        run_id: String,
        occurrence_id: String,
    },
    RunSnapshot {
        run_id: String,
    },
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[serde(deny_unknown_fields)]
pub struct ParameterTarget {
    pub block_version_id: String,
    pub step: QualifiedStepRef,
    pub parameter_key: String,
    pub expected_type: ParameterValueType,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub expected_unit: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "snake_case")]
pub enum SampleExpectation {
    Planned,
    Actual,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[serde(deny_unknown_fields)]
pub struct SampleTarget {
    pub block_version_id: String,
    pub step: QualifiedStepRef,
    pub port_id: String,
    pub expectation: SampleExpectation,
}

#[derive(Debug, Clone, Copy, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "snake_case")]
pub enum DataKind {
    Raw,
    Derived,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[serde(deny_unknown_fields)]
pub struct DataTarget {
    pub block_version_id: String,
    pub step: QualifiedStepRef,
    pub port_id: String,
    pub kind: DataKind,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[serde(deny_unknown_fields)]
pub struct ResourceTarget {
    pub block_version_id: String,
    pub step: QualifiedStepRef,
    pub slot_id: String,
    pub class: String,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub profile_id: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub profile_version: Option<u32>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[serde(tag = "kind", rename_all = "snake_case")]
pub enum BindingTarget {
    Parameter(ParameterTarget),
    Sample(SampleTarget),
    Data(DataTarget),
    Resource(ResourceTarget),
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[serde(deny_unknown_fields)]
pub struct PlannedSampleRef {
    pub design_version_id: String,
    pub sample_key: String,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[serde(deny_unknown_fields)]
pub struct SampleRef {
    pub sample_id: String,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[serde(deny_unknown_fields)]
pub struct RawDataRef {
    pub data_id: String,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[serde(deny_unknown_fields)]
pub struct DerivedDataRef {
    pub data_id: String,
    #[serde(default)]
    pub derived_from: Vec<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[serde(deny_unknown_fields)]
pub struct ResourceRef {
    pub class: String,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub profile_id: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub profile_version: Option<u32>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[serde(tag = "kind", rename_all = "snake_case")]
pub enum BindingSource {
    Literal {
        value: Value,
    },
    Legacy {
        value: Value,
        source: ValueSource,
        reference: Option<String>,
    },
    DesignRef {
        design_version_id: String,
        path: String,
    },
    ProtocolRef {
        protocol_version_id: String,
        path: String,
    },
    RunRef {
        run_id: String,
        path: String,
    },
    ToolOutput {
        invocation_id: String,
    },
    PlannedSample {
        reference: PlannedSampleRef,
    },
    ActualSample {
        reference: SampleRef,
    },
    HistoricalSample {
        reference: SampleRef,
        source_run_id: String,
    },
    HistoricalRawData {
        reference: RawDataRef,
        source_run_id: String,
    },
    HistoricalDerivedData {
        reference: DerivedDataRef,
        source_run_id: String,
    },
    RawData {
        reference: RawDataRef,
    },
    DerivedData {
        reference: DerivedDataRef,
    },
    Resource {
        reference: ResourceRef,
    },
}

#[derive(Debug, Clone, Copy, Serialize, Deserialize, PartialEq, Eq, Hash)]
#[serde(rename_all = "snake_case")]
pub enum BindingSourceKind {
    Literal,
    Legacy,
    DesignRef,
    ProtocolRef,
    RunRef,
    ToolOutput,
    PlannedSample,
    ActualSample,
    HistoricalSample,
    HistoricalRawData,
    HistoricalDerivedData,
    RawData,
    DerivedData,
    Resource,
}

impl BindingSource {
    pub fn kind(&self) -> BindingSourceKind {
        match self {
            Self::Literal { .. } => BindingSourceKind::Literal,
            Self::Legacy { .. } => BindingSourceKind::Legacy,
            Self::DesignRef { .. } => BindingSourceKind::DesignRef,
            Self::ProtocolRef { .. } => BindingSourceKind::ProtocolRef,
            Self::RunRef { .. } => BindingSourceKind::RunRef,
            Self::ToolOutput { .. } => BindingSourceKind::ToolOutput,
            Self::PlannedSample { .. } => BindingSourceKind::PlannedSample,
            Self::ActualSample { .. } => BindingSourceKind::ActualSample,
            Self::HistoricalSample { .. } => BindingSourceKind::HistoricalSample,
            Self::HistoricalRawData { .. } => BindingSourceKind::HistoricalRawData,
            Self::HistoricalDerivedData { .. } => BindingSourceKind::HistoricalDerivedData,
            Self::RawData { .. } => BindingSourceKind::RawData,
            Self::DerivedData { .. } => BindingSourceKind::DerivedData,
            Self::Resource { .. } => BindingSourceKind::Resource,
        }
    }
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[serde(deny_unknown_fields)]
pub struct ParameterSpec {
    pub value_type: ParameterValueType,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub unit: Option<String>,
    #[serde(default)]
    pub allowed_sources: Vec<BindingSourceKind>,
}

#[derive(Debug, Clone, Copy, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "snake_case")]
pub enum SourceStatus {
    Available,
    Missing,
    NotYetProduced,
    VersionMismatch,
    Denied,
}

pub trait BindingContext {
    fn parameter_spec(&self, block_version_id: &str, parameter_key: &str) -> Option<ParameterSpec>;
    fn source_status(&self, source: &BindingSource) -> SourceStatus;
    fn run_protocol_version(&self, run_id: &str) -> Option<String>;
}

#[derive(Debug, Clone, Copy, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "snake_case")]
pub enum BindingResolution {
    Resolved,
    Unresolved,
    NotApplicable,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(deny_unknown_fields)]
pub struct ProvenanceRef {
    pub kind: String,
    pub id: String,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub path: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[serde(deny_unknown_fields)]
pub struct TypedBinding {
    pub binding_id: String,
    pub owner: BindingOwner,
    pub target: BindingTarget,
    pub source: BindingSource,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub unit: Option<String>,
    pub resolution: BindingResolution,
    #[serde(default)]
    pub confirmed: bool,
    #[serde(default)]
    pub provenance: Vec<ProvenanceRef>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub struct SemanticIssue {
    pub code: String,
    pub path: String,
    pub message: String,
}

impl TypedBinding {
    /// Structural validation only. It cannot prove that referenced objects
    /// exist; use [`TypedBinding::validate_with_context`] for that.
    pub fn validate(&self) -> Result<(), Vec<SemanticIssue>> {
        self.validate_structural()
    }

    pub fn validate_structural(&self) -> Result<(), Vec<SemanticIssue>> {
        let mut issues = Vec::new();
        if self.binding_id.trim().is_empty() {
            issues.push(issue(
                "binding_id",
                "binding_id",
                "binding id must not be empty",
            ));
        }
        validate_owner(&self.owner, &mut issues);
        validate_target_step(&self.target, &mut issues);
        validate_target_fields(&self.target, &mut issues);
        validate_source_structure(&self.source, &mut issues);
        validate_resource_pairs(self.target.resource(), self.source.resource(), &mut issues);

        if matches!(self.owner, BindingOwner::Definition { .. }) {
            issues.push(issue(
                "definition_binding",
                "owner",
                "definition edits must create a new version; typed bindings only target plan/run instances",
            ));
        }

        if let BindingOwner::ProtocolPlan {
            protocol_version_id,
        } = &self.owner
        {
            if target_protocol(&self.target) != Some(protocol_version_id.as_str()) {
                issues.push(issue(
                    "owner_target_protocol",
                    "target.step.protocol_version_id",
                    "ProtocolPlan owner and target step must name the same Protocol version",
                ));
            }
        }

        match (&self.target, &self.source) {
            (BindingTarget::Parameter(target), BindingSource::Literal { value }) => {
                if self.resolution == BindingResolution::Resolved {
                    validate_literal(target, value, self.unit.as_deref(), &mut issues);
                }
            }
            (BindingTarget::Parameter(target), BindingSource::Legacy { value, .. }) => {
                if self.resolution == BindingResolution::Resolved {
                    validate_literal(target, value, self.unit.as_deref(), &mut issues);
                }
            }
            (BindingTarget::Parameter(_), BindingSource::DesignRef { .. })
            | (BindingTarget::Parameter(_), BindingSource::ProtocolRef { .. })
            | (BindingTarget::Parameter(_), BindingSource::ToolOutput { .. }) => {}
            (BindingTarget::Parameter(_), _) => issues.push(issue(
                "parameter_source",
                "source",
                "parameter bindings require a literal or a design/protocol/tool source",
            )),
            (BindingTarget::Sample(target), BindingSource::PlannedSample { .. })
                if target.expectation == SampleExpectation::Actual =>
            {
                issues.push(issue(
                    "planned_actual_sample",
                    "source",
                    "a planned sample cannot satisfy an actual sample target",
                ));
            }
            (BindingTarget::Sample(target), BindingSource::PlannedSample { .. })
                if target.expectation == SampleExpectation::Planned => {}
            (BindingTarget::Sample(target), BindingSource::ActualSample { .. })
                if target.expectation == SampleExpectation::Actual => {}
            (BindingTarget::Sample(target), BindingSource::HistoricalSample { .. })
                if target.expectation == SampleExpectation::Actual => {}
            (BindingTarget::Sample(_), _) => issues.push(issue(
                "sample_source",
                "source",
                "sample target/source expectations do not match",
            )),
            (BindingTarget::Data(target), BindingSource::RawData { .. })
                if target.kind == DataKind::Raw => {}
            (BindingTarget::Data(target), BindingSource::DerivedData { .. })
                if target.kind == DataKind::Derived => {}
            (BindingTarget::Data(target), BindingSource::HistoricalRawData { .. })
                if target.kind == DataKind::Raw => {}
            (BindingTarget::Data(target), BindingSource::HistoricalDerivedData { .. })
                if target.kind == DataKind::Derived => {}
            (BindingTarget::Data(_), _) => issues.push(issue(
                "data_source",
                "source",
                "data target/source kind does not match",
            )),
            (BindingTarget::Resource(target), BindingSource::Resource { reference }) => {
                if target.class != reference.class {
                    issues.push(issue(
                        "resource_class",
                        "source",
                        "resource class does not match the target slot",
                    ));
                }
                if target.profile_id != reference.profile_id
                    || target.profile_version != reference.profile_version
                {
                    issues.push(issue(
                        "resource_version",
                        "source",
                        "resource profile id/version must match exactly",
                    ));
                }
            }
            (BindingTarget::Resource(_), _) => issues.push(issue(
                "resource_source",
                "source",
                "resource targets require a resource reference",
            )),
        }

        if matches!(
            &self.owner,
            BindingOwner::ProtocolPlan { .. } | BindingOwner::RunPlan { .. }
        ) && matches!(
            &self.source,
            BindingSource::RunRef { .. }
                | BindingSource::ActualSample { .. }
                | BindingSource::RawData { .. }
                | BindingSource::DerivedData { .. }
                | BindingSource::Legacy {
                    source: ValueSource::RunActual,
                    ..
                }
        ) {
            issues.push(issue(
                "execution_source_in_plan",
                "source",
                "plans may not use current-run execution facts; use an explicit historical reference for prior data",
            ));
        }

        if matches!(&self.owner, BindingOwner::RunExecution { .. })
            && matches!(
                &self.target,
                BindingTarget::Sample(SampleTarget {
                    expectation: SampleExpectation::Planned,
                    ..
                })
            )
        {
            issues.push(issue(
                "execution_planned_target",
                "target",
                "execution facts must target an actual sample, not a planned sample slot",
            ));
        }

        finish(issues)
    }

    /// Structural checks plus read-only contextual resolution. A `Resolved`
    /// label alone is not trusted: the referenced object must exist, match its
    /// version/lifecycle, and fit the authoritative parameter spec.
    pub fn validate_with_context(
        &self,
        context: &dyn BindingContext,
    ) -> Result<(), Vec<SemanticIssue>> {
        let mut issues = self.validate_structural().err().unwrap_or_default();

        if let Some(run_id) = owner_run_id(&self.owner) {
            match context.run_protocol_version(run_id) {
                Some(protocol_version_id)
                    if target_protocol(&self.target) == Some(protocol_version_id.as_str()) => {}
                Some(_) => issues.push(issue(
                    "owner_target_protocol",
                    "target.step.protocol_version_id",
                    "Run owner and target step must share the frozen Run Protocol version",
                )),
                None => issues.push(issue(
                    "context_run_missing",
                    "owner.run_id",
                    "Run context is unavailable; binding cannot be contextual validated",
                )),
            }
        }

        if let BindingTarget::Parameter(target) = &self.target {
            match context.parameter_spec(&target.block_version_id, &target.parameter_key) {
                Some(spec) => {
                    if spec.value_type != target.expected_type {
                        issues.push(issue(
                            "target_schema_type",
                            "target.expected_type",
                            "target type does not match the authoritative parameter definition",
                        ));
                    }
                    if spec.unit != target.expected_unit {
                        issues.push(issue(
                            "target_schema_unit",
                            "target.expected_unit",
                            "target unit does not match the authoritative parameter definition",
                        ));
                    }
                    if !spec.allowed_sources.is_empty()
                        && !spec.allowed_sources.contains(&self.source.kind())
                    {
                        issues.push(issue(
                            "parameter_source_not_allowed",
                            "source",
                            "source kind is not allowed by the authoritative parameter definition",
                        ));
                    }
                    if self.resolution == BindingResolution::Resolved {
                        match &self.source {
                            BindingSource::Literal { value }
                            | BindingSource::Legacy { value, .. } => {
                                validate_parameter_value(
                                    spec.value_type,
                                    spec.unit.as_deref(),
                                    value,
                                    self.unit.as_deref(),
                                    &mut issues,
                                );
                            }
                            _ => {}
                        }
                    }
                }
                None => issues.push(issue(
                    "parameter_spec_missing",
                    "target.parameter_key",
                    "authoritative parameter definition is unavailable",
                )),
            }
        }

        if !matches!(
            self.source,
            BindingSource::Literal { .. } | BindingSource::Legacy { .. }
        ) {
            match context.source_status(&self.source) {
                SourceStatus::Available => {}
                SourceStatus::Missing => issues.push(issue(
                    "source_missing",
                    "source",
                    "referenced source does not exist in the supplied context",
                )),
                SourceStatus::NotYetProduced => issues.push(issue(
                    "source_not_yet_produced",
                    "source",
                    "referenced source has not been produced or is future execution data",
                )),
                SourceStatus::VersionMismatch => issues.push(issue(
                    "source_version",
                    "source",
                    "referenced source version does not match the binding",
                )),
                SourceStatus::Denied => issues.push(issue(
                    "source_denied",
                    "source",
                    "referenced source crosses a scope/Run boundary without authorization",
                )),
            }
        }

        finish(issues)
    }

    pub fn is_plan_ready(&self) -> bool {
        matches!(
            self.owner,
            BindingOwner::ProtocolPlan { .. } | BindingOwner::RunPlan { .. }
        ) && self.resolution == BindingResolution::Resolved
            && self.confirmed
            && self.validate_structural().is_ok()
    }

    pub fn is_plan_ready_with_context(&self, context: &dyn BindingContext) -> bool {
        self.is_plan_ready() && self.validate_with_context(context).is_ok()
    }
}

impl BindingTarget {
    fn resource(&self) -> Option<&ResourceTarget> {
        match self {
            Self::Resource(target) => Some(target),
            _ => None,
        }
    }
}

impl BindingSource {
    fn resource(&self) -> Option<&ResourceRef> {
        match self {
            Self::Resource { reference } => Some(reference),
            _ => None,
        }
    }
}

fn target_protocol(target: &BindingTarget) -> Option<&str> {
    Some(match target {
        BindingTarget::Parameter(target) => &target.step.protocol_version_id,
        BindingTarget::Sample(target) => &target.step.protocol_version_id,
        BindingTarget::Data(target) => &target.step.protocol_version_id,
        BindingTarget::Resource(target) => &target.step.protocol_version_id,
    })
}

fn owner_run_id(owner: &BindingOwner) -> Option<&str> {
    match owner {
        BindingOwner::RunPlan { run_id }
        | BindingOwner::RunExecution { run_id, .. }
        | BindingOwner::RunSnapshot { run_id } => Some(run_id),
        _ => None,
    }
}

fn validate_target_fields(target: &BindingTarget, issues: &mut Vec<SemanticIssue>) {
    match target {
        BindingTarget::Parameter(target) => {
            validate_id(&target.block_version_id, "target.block_version_id", issues);
            validate_id(&target.parameter_key, "target.parameter_key", issues);
        }
        BindingTarget::Sample(target) => {
            validate_id(&target.block_version_id, "target.block_version_id", issues);
            validate_id(&target.port_id, "target.port_id", issues);
        }
        BindingTarget::Data(target) => {
            validate_id(&target.block_version_id, "target.block_version_id", issues);
            validate_id(&target.port_id, "target.port_id", issues);
        }
        BindingTarget::Resource(target) => {
            validate_id(&target.block_version_id, "target.block_version_id", issues);
            validate_id(&target.slot_id, "target.slot_id", issues);
            validate_id(&target.class, "target.class", issues);
            validate_resource_pair(
                target.profile_id.as_deref(),
                target.profile_version,
                "target",
                issues,
            );
        }
    }
}

fn validate_source_structure(source: &BindingSource, issues: &mut Vec<SemanticIssue>) {
    match source {
        BindingSource::Literal { .. } => {}
        BindingSource::Legacy { reference, .. } => {
            if let Some(reference) = reference {
                validate_id(reference, "source.reference", issues);
            }
        }
        BindingSource::DesignRef {
            design_version_id,
            path,
        } => {
            validate_id(design_version_id, "source.design_version_id", issues);
            validate_id(path, "source.path", issues);
        }
        BindingSource::ProtocolRef {
            protocol_version_id,
            path,
        } => {
            validate_id(protocol_version_id, "source.protocol_version_id", issues);
            validate_id(path, "source.path", issues);
        }
        BindingSource::RunRef { run_id, path } => {
            validate_id(run_id, "source.run_id", issues);
            validate_id(path, "source.path", issues);
        }
        BindingSource::ToolOutput { invocation_id } => {
            validate_id(invocation_id, "source.invocation_id", issues)
        }
        BindingSource::PlannedSample { reference } => {
            validate_id(
                &reference.design_version_id,
                "source.reference.design_version_id",
                issues,
            );
            validate_id(&reference.sample_key, "source.reference.sample_key", issues);
        }
        BindingSource::ActualSample { reference } => {
            validate_id(&reference.sample_id, "source.reference.sample_id", issues)
        }
        BindingSource::HistoricalSample {
            reference,
            source_run_id,
        } => {
            validate_id(&reference.sample_id, "source.reference.sample_id", issues);
            validate_id(source_run_id, "source.source_run_id", issues);
        }
        BindingSource::HistoricalRawData {
            reference,
            source_run_id,
        } => {
            validate_id(&reference.data_id, "source.reference.data_id", issues);
            validate_id(source_run_id, "source.source_run_id", issues);
        }
        BindingSource::HistoricalDerivedData {
            reference,
            source_run_id,
        } => {
            validate_id(&reference.data_id, "source.reference.data_id", issues);
            validate_id(source_run_id, "source.source_run_id", issues);
        }
        BindingSource::RawData { reference } => {
            validate_id(&reference.data_id, "source.reference.data_id", issues)
        }
        BindingSource::DerivedData { reference } => {
            validate_id(&reference.data_id, "source.reference.data_id", issues)
        }
        BindingSource::Resource { reference } => {
            validate_id(&reference.class, "source.reference.class", issues);
            validate_resource_pair(
                reference.profile_id.as_deref(),
                reference.profile_version,
                "source",
                issues,
            );
        }
    }
}

fn validate_resource_pairs(
    target: Option<&ResourceTarget>,
    source: Option<&ResourceRef>,
    issues: &mut Vec<SemanticIssue>,
) {
    if let Some(target) = target {
        validate_resource_pair(
            target.profile_id.as_deref(),
            target.profile_version,
            "target",
            issues,
        );
    }
    if let Some(source) = source {
        validate_resource_pair(
            source.profile_id.as_deref(),
            source.profile_version,
            "source",
            issues,
        );
    }
}

fn validate_resource_pair(
    profile_id: Option<&str>,
    profile_version: Option<u32>,
    path: &str,
    issues: &mut Vec<SemanticIssue>,
) {
    if profile_id.is_some() != profile_version.is_some() {
        issues.push(issue(
            "resource_profile_pair",
            format!("{path}.profile"),
            "profile id and version must be supplied together",
        ));
    }
}

fn validate_owner(owner: &BindingOwner, issues: &mut Vec<SemanticIssue>) {
    match owner {
        BindingOwner::Definition { block_version_id } => {
            validate_id(block_version_id, "owner.block_version_id", issues)
        }
        BindingOwner::ProtocolPlan {
            protocol_version_id,
        } => validate_id(protocol_version_id, "owner.protocol_version_id", issues),
        BindingOwner::RunPlan { run_id } | BindingOwner::RunSnapshot { run_id } => {
            validate_id(run_id, "owner.run_id", issues)
        }
        BindingOwner::RunExecution {
            run_id,
            occurrence_id,
        } => {
            validate_id(run_id, "owner.run_id", issues);
            validate_id(occurrence_id, "owner.occurrence_id", issues);
        }
    }
}

fn validate_target_step(target: &BindingTarget, issues: &mut Vec<SemanticIssue>) {
    let step = match target {
        BindingTarget::Parameter(target) => &target.step,
        BindingTarget::Sample(target) => &target.step,
        BindingTarget::Data(target) => &target.step,
        BindingTarget::Resource(target) => &target.step,
    };
    validate_id(
        &step.protocol_version_id,
        "target.step.protocol_version_id",
        issues,
    );
    validate_id(&step.step_path, "target.step.step_path", issues);
    match &step.scope {
        ProtocolScope::Assay { assay_id } => {
            validate_id(assay_id, "target.step.scope.assay_id", issues)
        }
        ProtocolScope::PlannedOutput { planned_output_id } => validate_id(
            planned_output_id,
            "target.step.scope.planned_output_id",
            issues,
        ),
        ProtocolScope::Unscoped => {}
    }
}

fn validate_literal(
    target: &ParameterTarget,
    value: &Value,
    unit: Option<&str>,
    issues: &mut Vec<SemanticIssue>,
) {
    validate_parameter_value(
        target.expected_type,
        target.expected_unit.as_deref(),
        value,
        unit,
        issues,
    );
}

fn validate_parameter_value(
    expected_type: ParameterValueType,
    expected_unit: Option<&str>,
    value: &Value,
    unit: Option<&str>,
    issues: &mut Vec<SemanticIssue>,
) {
    let type_ok = match expected_type {
        ParameterValueType::Text => value.as_str().is_some(),
        ParameterValueType::Number => value.as_f64().is_some_and(f64::is_finite),
        ParameterValueType::Integer => value.as_i64().is_some() || value.as_u64().is_some(),
        ParameterValueType::Boolean => value.as_bool().is_some(),
    };
    if !type_ok {
        issues.push(issue(
            "parameter_type",
            "source.value",
            "literal value does not match the target parameter type",
        ));
    }
    if expected_unit != unit {
        issues.push(issue(
            "parameter_unit",
            "unit",
            "literal unit must exactly match the target parameter unit; no implicit conversion",
        ));
    }
}

/// Adapt the existing `operation_module::ParameterBinding` without changing it.
/// An unknown/null legacy binding stays unresolved; no default value is invented.
pub fn from_legacy_parameter_binding(
    legacy: &ParameterBinding,
    owner: BindingOwner,
    target: ParameterTarget,
    binding_id: impl Into<String>,
) -> TypedBinding {
    let unresolved = legacy.source == ValueSource::Unknown || legacy.value.is_null();
    let mut provenance = Vec::new();
    if let Some(reference) = &legacy.source_reference {
        provenance.push(ProvenanceRef {
            kind: "legacy_source_reference".to_owned(),
            id: reference.clone(),
            path: None,
        });
    }
    TypedBinding {
        binding_id: binding_id.into(),
        owner,
        target: BindingTarget::Parameter(target),
        source: BindingSource::Legacy {
            value: legacy.value.clone(),
            source: legacy.source,
            reference: legacy.source_reference.clone(),
        },
        unit: legacy.unit.clone(),
        resolution: if unresolved {
            BindingResolution::Unresolved
        } else {
            BindingResolution::Resolved
        },
        confirmed: legacy.confirmed,
        provenance,
    }
}

fn validate_id(value: &str, path: &str, issues: &mut Vec<SemanticIssue>) {
    if value.trim().is_empty() {
        issues.push(issue("empty_id", path, "id/path must not be empty"));
    }
}

fn issue(
    code: impl Into<String>,
    path: impl Into<String>,
    message: impl Into<String>,
) -> SemanticIssue {
    SemanticIssue {
        code: code.into(),
        path: path.into(),
        message: message.into(),
    }
}

fn finish(issues: Vec<SemanticIssue>) -> Result<(), Vec<SemanticIssue>> {
    if issues.is_empty() {
        Ok(())
    } else {
        Err(issues)
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::operation_module::ParameterBinding;
    use serde_json::json;

    fn step(protocol: &str, path: &str) -> QualifiedStepRef {
        QualifiedStepRef {
            protocol_version_id: protocol.into(),
            scope: ProtocolScope::Assay {
                assay_id: "assay-1".into(),
            },
            step_path: path.into(),
        }
    }

    fn parameter_target(protocol: &str, path: &str, key: &str) -> ParameterTarget {
        ParameterTarget {
            block_version_id: "block-v1".into(),
            step: step(protocol, path),
            parameter_key: key.into(),
            expected_type: ParameterValueType::Number,
            expected_unit: Some("uL".into()),
        }
    }

    fn owner(protocol: &str) -> BindingOwner {
        BindingOwner::ProtocolPlan {
            protocol_version_id: protocol.into(),
        }
    }

    fn binding(protocol: &str, path: &str, key: &str, value: Value, unit: &str) -> TypedBinding {
        TypedBinding {
            binding_id: format!("{protocol}:{path}:{key}"),
            owner: owner(protocol),
            target: BindingTarget::Parameter(parameter_target(protocol, path, key)),
            source: BindingSource::Literal { value },
            unit: Some(unit.into()),
            resolution: BindingResolution::Resolved,
            confirmed: true,
            provenance: vec![],
        }
    }

    #[derive(Default)]
    struct TestContext {
        specs: std::collections::HashMap<(String, String), ParameterSpec>,
        sources: std::collections::HashMap<String, SourceStatus>,
        runs: std::collections::HashMap<String, String>,
    }

    impl TestContext {
        fn with_volume() -> Self {
            let mut context = Self::default();
            context.specs.insert(
                ("block-v1".into(), "volume".into()),
                ParameterSpec {
                    value_type: ParameterValueType::Number,
                    unit: Some("uL".into()),
                    allowed_sources: vec![],
                },
            );
            context
        }

        fn source(mut self, source: BindingSource, status: SourceStatus) -> Self {
            self.sources
                .insert(serde_json::to_string(&source).unwrap(), status);
            self
        }

        fn run(mut self, run_id: &str, protocol_version_id: &str) -> Self {
            self.runs.insert(run_id.into(), protocol_version_id.into());
            self
        }
    }

    impl BindingContext for TestContext {
        fn parameter_spec(
            &self,
            block_version_id: &str,
            parameter_key: &str,
        ) -> Option<ParameterSpec> {
            self.specs
                .get(&(block_version_id.into(), parameter_key.into()))
                .cloned()
        }

        fn source_status(&self, source: &BindingSource) -> SourceStatus {
            self.sources
                .get(&serde_json::to_string(source).unwrap())
                .copied()
                .unwrap_or(SourceStatus::Missing)
        }

        fn run_protocol_version(&self, run_id: &str) -> Option<String> {
            self.runs.get(run_id).cloned()
        }
    }

    fn context() -> TestContext {
        TestContext::with_volume()
    }

    fn data_target(protocol: &str, kind: DataKind) -> DataTarget {
        DataTarget {
            block_version_id: "block-v1".into(),
            step: step(protocol, "root/read"),
            port_id: "data_out".into(),
            kind,
        }
    }

    #[test]
    fn same_module_in_two_protocols_does_not_cross_targets() {
        let first = binding("protocol-a", "root/transfer", "volume", json!(10.0), "uL");
        let second = binding("protocol-b", "root/transfer", "volume", json!(20.0), "uL");
        assert!(first.validate().is_ok());
        assert!(second.validate().is_ok());
        let keys = match (&first.target, &second.target) {
            (BindingTarget::Parameter(a), BindingTarget::Parameter(b)) => {
                (a.step.target_key(), b.step.target_key())
            }
            _ => panic!("parameter targets"),
        };
        assert_ne!(keys.0, keys.1);
    }

    #[test]
    fn same_protocol_duplicate_use_requires_distinct_step_paths() {
        let first = binding("protocol-a", "root/transfer-1", "volume", json!(10.0), "uL");
        let second = binding("protocol-a", "root/transfer-2", "volume", json!(10.0), "uL");
        assert!(first.validate().is_ok());
        assert!(second.validate().is_ok());
        let keys = match (&first.target, &second.target) {
            (BindingTarget::Parameter(a), BindingTarget::Parameter(b)) => {
                (a.step.target_key(), b.step.target_key())
            }
            _ => panic!("parameter targets"),
        };
        assert_ne!(keys.0, keys.1);
    }

    #[test]
    fn planned_sample_cannot_satisfy_actual_sample_target() {
        let mut value = binding("protocol-a", "root/transfer", "volume", json!(10.0), "uL");
        value.target = BindingTarget::Sample(SampleTarget {
            block_version_id: "block-v1".into(),
            step: step("protocol-a", "root/sample"),
            port_id: "sample_in".into(),
            expectation: SampleExpectation::Actual,
        });
        value.source = BindingSource::PlannedSample {
            reference: PlannedSampleRef {
                design_version_id: "design-v1".into(),
                sample_key: "g1-d3".into(),
            },
        };
        let errors = value.validate().unwrap_err();
        assert!(errors
            .iter()
            .any(|issue| issue.code == "planned_actual_sample"));
    }

    #[test]
    fn run_results_are_not_available_to_protocol_plans() {
        let mut value = binding("protocol-a", "root/read", "wavelength", json!(450.0), "uL");
        value.source = BindingSource::RunRef {
            run_id: "run-1".into(),
            path: "read/od".into(),
        };
        let errors = value.validate().unwrap_err();
        assert!(errors
            .iter()
            .any(|issue| issue.code == "execution_source_in_plan"));
    }

    #[test]
    fn unit_and_type_conflicts_are_rejected() {
        let wrong_unit = binding("protocol-a", "root/transfer", "volume", json!(10.0), "mL");
        assert!(wrong_unit.validate().is_err());
        let wrong_type = binding("protocol-a", "root/transfer", "volume", json!("ten"), "uL");
        assert!(wrong_type.validate().is_err());
    }

    #[test]
    fn unresolved_is_not_fabricated() {
        let mut value = binding("protocol-a", "root/transfer", "volume", json!(null), "uL");
        value.resolution = BindingResolution::Unresolved;
        value.source = BindingSource::DesignRef {
            design_version_id: "design-v1".into(),
            path: "sample.volume".into(),
        };
        assert!(value.validate().is_ok());
        assert_eq!(value.resolution, BindingResolution::Unresolved);
    }

    #[test]
    fn definition_bindings_are_rejected() {
        let mut value = binding("protocol-a", "root/transfer", "volume", json!(10.0), "uL");
        value.owner = BindingOwner::Definition {
            block_version_id: "block-v1".into(),
        };
        let errors = value.validate().unwrap_err();
        assert!(errors
            .iter()
            .any(|issue| issue.code == "definition_binding"));
    }

    #[test]
    fn legacy_binding_keeps_unknown_unresolved() {
        let legacy = ParameterBinding {
            parameter_key: "volume".into(),
            value: json!(null),
            unit: Some("uL".into()),
            source: ValueSource::Unknown,
            source_reference: None,
            confirmed: false,
        };
        let adapted = from_legacy_parameter_binding(
            &legacy,
            owner("protocol-a"),
            parameter_target("protocol-a", "root/transfer", "volume"),
            "legacy:volume",
        );
        assert_eq!(adapted.resolution, BindingResolution::Unresolved);
        assert!(matches!(
            adapted.source,
            BindingSource::Legacy {
                source: ValueSource::Unknown,
                ..
            }
        ));
        assert!(adapted.validate().is_ok());
    }

    #[test]
    fn empty_step_reference_is_rejected() {
        let mut value = binding("protocol-a", "root/transfer", "volume", json!(10.0), "uL");
        if let BindingTarget::Parameter(target) = &mut value.target {
            target.step.step_path.clear();
        }
        let errors = value.validate().unwrap_err();
        assert!(errors.iter().any(|issue| issue.code == "empty_id"));
    }

    #[test]
    fn execution_cannot_target_a_planned_sample() {
        let mut value = binding("protocol-a", "root/transfer", "volume", json!(10.0), "uL");
        value.owner = BindingOwner::RunExecution {
            run_id: "run-1".into(),
            occurrence_id: "occ-1".into(),
        };
        value.target = BindingTarget::Sample(SampleTarget {
            block_version_id: "block-v1".into(),
            step: step("protocol-a", "root/sample"),
            port_id: "sample_in".into(),
            expectation: SampleExpectation::Planned,
        });
        value.source = BindingSource::PlannedSample {
            reference: PlannedSampleRef {
                design_version_id: "design-v1".into(),
                sample_key: "g1-d3".into(),
            },
        };
        let errors = value.validate().unwrap_err();
        assert!(errors
            .iter()
            .any(|issue| issue.code == "execution_planned_target"));
    }

    #[test]
    fn resource_version_must_match() {
        let mut value = binding("protocol-a", "root/read", "volume", json!(10.0), "uL");
        value.target = BindingTarget::Resource(ResourceTarget {
            block_version_id: "block-v1".into(),
            step: step("protocol-a", "root/read"),
            slot_id: "reader".into(),
            class: "plate_reader".into(),
            profile_id: Some("reader-1".into()),
            profile_version: Some(1),
        });
        value.source = BindingSource::Resource {
            reference: ResourceRef {
                class: "plate_reader".into(),
                profile_id: Some("reader-1".into()),
                profile_version: Some(2),
            },
        };
        let errors = value.validate().unwrap_err();
        assert!(errors.iter().any(|issue| issue.code == "resource_version"));
    }
    #[test]
    fn owner_and_target_protocol_must_match() {
        let mut value = binding("protocol-a", "root/transfer", "volume", json!(10.0), "uL");
        value.owner = owner("protocol-b");
        let errors = value.validate().unwrap_err();
        assert!(errors
            .iter()
            .any(|issue| issue.code == "owner_target_protocol"));
    }

    #[test]
    fn resolved_reference_must_exist_in_context() {
        let mut value = binding("protocol-a", "root/transfer", "volume", json!(10.0), "uL");
        value.source = BindingSource::DesignRef {
            design_version_id: "design-v1".into(),
            path: "sample.volume".into(),
        };
        let errors = value.validate_with_context(&context()).unwrap_err();
        assert!(errors.iter().any(|issue| issue.code == "source_missing"));
    }

    #[test]
    fn available_reference_passes_context_validation() {
        let mut value = binding("protocol-a", "root/transfer", "volume", json!(10.0), "uL");
        value.source = BindingSource::DesignRef {
            design_version_id: "design-v1".into(),
            path: "sample.volume".into(),
        };
        let context = context().source(value.source.clone(), SourceStatus::Available);
        assert!(value.validate_with_context(&context).is_ok());
    }

    #[test]
    fn unconfirmed_legacy_is_resolved_but_not_plan_ready() {
        let legacy = ParameterBinding {
            parameter_key: "volume".into(),
            value: json!(10.0),
            unit: Some("uL".into()),
            source: ValueSource::ProtocolFixed,
            source_reference: None,
            confirmed: false,
        };
        let adapted = from_legacy_parameter_binding(
            &legacy,
            owner("protocol-a"),
            parameter_target("protocol-a", "root/transfer", "volume"),
            "legacy:volume",
        );
        assert_eq!(adapted.resolution, BindingResolution::Resolved);
        assert!(!adapted.is_plan_ready());
        assert!(!adapted.is_plan_ready_with_context(&context()));
    }

    #[test]
    fn plan_rejects_current_run_raw_data() {
        let mut value = binding("protocol-a", "root/read", "volume", json!(10.0), "uL");
        value.target = BindingTarget::Data(data_target("protocol-a", DataKind::Raw));
        value.source = BindingSource::RawData {
            reference: RawDataRef {
                data_id: "data-1".into(),
            },
        };
        let errors = value.validate().unwrap_err();
        assert!(errors
            .iter()
            .any(|issue| issue.code == "execution_source_in_plan"));
    }

    #[test]
    fn historical_data_in_plan_is_allowed() {
        let mut value = binding("protocol-a", "root/read", "volume", json!(10.0), "uL");
        value.target = BindingTarget::Data(data_target("protocol-a", DataKind::Raw));
        value.source = BindingSource::HistoricalRawData {
            reference: RawDataRef {
                data_id: "data-old".into(),
            },
            source_run_id: "run-old".into(),
        };
        let context = context().source(value.source.clone(), SourceStatus::Available);
        assert!(value.validate_with_context(&context).is_ok());
    }

    #[test]
    fn forged_parameter_type_is_rejected_by_context() {
        let mut context = context();
        context.specs.insert(
            ("block-v1".into(), "volume".into()),
            ParameterSpec {
                value_type: ParameterValueType::Text,
                unit: Some("uL".into()),
                allowed_sources: vec![],
            },
        );
        let value = binding("protocol-a", "root/transfer", "volume", json!(10.0), "uL");
        let errors = value.validate_with_context(&context).unwrap_err();
        assert!(errors
            .iter()
            .any(|issue| issue.code == "target_schema_type"));
    }

    #[test]
    fn resource_profile_id_requires_version() {
        let mut value = binding("protocol-a", "root/read", "volume", json!(10.0), "uL");
        value.target = BindingTarget::Resource(ResourceTarget {
            block_version_id: "block-v1".into(),
            step: step("protocol-a", "root/read"),
            slot_id: "reader".into(),
            class: "plate_reader".into(),
            profile_id: Some("reader-1".into()),
            profile_version: None,
        });
        value.source = BindingSource::Resource {
            reference: ResourceRef {
                class: "plate_reader".into(),
                profile_id: None,
                profile_version: None,
            },
        };
        let errors = value.validate().unwrap_err();
        assert!(errors
            .iter()
            .any(|issue| issue.code == "resource_profile_pair"));
    }

    #[test]
    fn empty_parameter_key_and_source_path_are_rejected() {
        let mut value = binding("protocol-a", "root/transfer", "volume", json!(10.0), "uL");
        if let BindingTarget::Parameter(target) = &mut value.target {
            target.parameter_key.clear();
        }
        value.source = BindingSource::DesignRef {
            design_version_id: "design-v1".into(),
            path: String::new(),
        };
        let errors = value.validate().unwrap_err();
        assert!(errors.iter().any(|issue| issue.code == "empty_id"));
    }

    #[test]
    fn run_owner_must_match_frozen_protocol() {
        let mut value = binding("protocol-b", "root/transfer", "volume", json!(10.0), "uL");
        value.owner = BindingOwner::RunPlan {
            run_id: "run-1".into(),
        };
        let context = context().run("run-1", "protocol-a");
        let errors = value.validate_with_context(&context).unwrap_err();
        assert!(errors
            .iter()
            .any(|issue| issue.code == "owner_target_protocol"));
    }

    #[test]
    fn plan_rejects_current_actual_sample() {
        let mut value = binding("protocol-a", "root/read", "volume", json!(10.0), "uL");
        value.target = BindingTarget::Sample(SampleTarget {
            block_version_id: "block-v1".into(),
            step: step("protocol-a", "root/sample"),
            port_id: "sample_in".into(),
            expectation: SampleExpectation::Actual,
        });
        value.source = BindingSource::ActualSample {
            reference: SampleRef {
                sample_id: "sample-1".into(),
            },
        };
        let errors = value.validate().unwrap_err();
        assert!(errors
            .iter()
            .any(|issue| issue.code == "execution_source_in_plan"));
    }

    #[test]
    fn empty_port_and_block_version_are_rejected() {
        let mut value = binding("protocol-a", "root/read", "volume", json!(10.0), "uL");
        value.target = BindingTarget::Sample(SampleTarget {
            block_version_id: String::new(),
            step: step("protocol-a", "root/sample"),
            port_id: String::new(),
            expectation: SampleExpectation::Planned,
        });
        value.source = BindingSource::PlannedSample {
            reference: PlannedSampleRef {
                design_version_id: "design-v1".into(),
                sample_key: "g1-d3".into(),
            },
        };
        let errors = value.validate().unwrap_err();
        assert!(errors.iter().any(|issue| issue.code == "empty_id"));
    }

    #[test]
    fn future_execution_data_is_rejected() {
        let mut value = binding("protocol-a", "root/read", "volume", json!(10.0), "uL");
        value.owner = BindingOwner::RunExecution {
            run_id: "run-1".into(),
            occurrence_id: "occ-1".into(),
        };
        value.target = BindingTarget::Data(data_target("protocol-a", DataKind::Raw));
        value.source = BindingSource::RawData {
            reference: RawDataRef {
                data_id: "data-1".into(),
            },
        };
        let context = context()
            .run("run-1", "protocol-a")
            .source(value.source.clone(), SourceStatus::NotYetProduced);
        let errors = value.validate_with_context(&context).unwrap_err();
        assert!(errors
            .iter()
            .any(|issue| issue.code == "source_not_yet_produced"));
    }
}
