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

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
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
    pub step: QualifiedStepRef,
    pub port_id: String,
    pub kind: DataKind,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[serde(deny_unknown_fields)]
pub struct ResourceTarget {
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
    pub provenance: Vec<ProvenanceRef>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub struct SemanticIssue {
    pub code: String,
    pub path: String,
    pub message: String,
}

impl TypedBinding {
    pub fn validate(&self) -> Result<(), Vec<SemanticIssue>> {
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

        if matches!(self.owner, BindingOwner::Definition { .. }) {
            issues.push(issue(
                "definition_binding",
                "owner",
                "definition edits must create a new version; typed bindings only target plan/run instances",
            ));
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
            (BindingTarget::Sample(_), _) => issues.push(issue(
                "sample_source",
                "source",
                "sample target/source expectations do not match",
            )),
            (BindingTarget::Data(target), BindingSource::RawData { .. })
                if target.kind == DataKind::Raw => {}
            (BindingTarget::Data(target), BindingSource::DerivedData { .. })
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
                | BindingSource::Legacy {
                    source: ValueSource::RunActual,
                    ..
                }
        ) {
            issues.push(issue(
                "run_ref_in_plan",
                "source",
                "protocol plans cannot reference Run results",
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
    let type_ok = match target.expected_type {
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
    if target.expected_unit.as_deref() != unit {
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
            provenance: vec![],
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
        assert!(errors.iter().any(|issue| issue.code == "run_ref_in_plan"));
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
}
