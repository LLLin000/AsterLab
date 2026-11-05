//! S2-A minimal Catalog Compiler.
//!
//! Compile an `OperationModule` into a project-local block version while keeping
//! the catalog identity, module-contract sidecar, ports, instrument
//! requirements and sample transitions. This is pure Core: no SQLite, no
//! second Protocol/Run, no mutation of frozen records.

use crate::operation_module::{
    CatalogModuleRef, ContractIssue, InstrumentRequirement, ModuleBinding, ModuleOrigin,
    OperationModule, PortSpec, ReviewStatus, SampleTransition,
};
use crate::protocol_preview::StepTimeScope;
use crate::semantic_component::SemanticIssue;
use serde::{Deserialize, Serialize};
use std::collections::HashSet;

pub const SEMANTIC_CATALOG_SCHEMA_VERSION: u32 = 1;

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(deny_unknown_fields)]
pub struct CompiledParameterSlot {
    pub key: String,
    pub label: String,
    pub value_type: crate::operation_module::ParameterValueType,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub unit: Option<String>,
    pub required: bool,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[serde(tag = "kind", rename_all = "snake_case")]
pub enum CompiledStep {
    Text {
        use_id: String,
        title: String,
        instruction: String,
        parameter_refs: Vec<String>,
        #[serde(default, skip_serializing_if = "Option::is_none")]
        time_scope: Option<StepTimeScope>,
    },
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[serde(deny_unknown_fields)]
pub struct CompiledBlock {
    pub block_id: String,
    pub block_version_id: String,
    pub binding: ModuleBinding,
    pub catalog: CatalogModuleRef,
    pub summary: String,
    pub parameter_slots: Vec<CompiledParameterSlot>,
    pub steps: Vec<CompiledStep>,
    pub inputs: Vec<PortSpec>,
    pub outputs: Vec<PortSpec>,
    pub sample_transitions: Vec<SampleTransition>,
    pub instrument_requirements: Vec<InstrumentRequirement>,
    /// Original module contract. This is a sidecar, not a second live block.
    pub module_contract: OperationModule,
}

#[derive(Debug, Clone, Copy, Serialize, Deserialize, PartialEq, Eq)]
#[serde(deny_unknown_fields)]
pub struct CompilePolicy {
    /// Isolated test/fixture compilation only. Formal use must require review.
    pub allow_research_draft: bool,
}

impl Default for CompilePolicy {
    fn default() -> Self {
        Self {
            allow_research_draft: false,
        }
    }
}

impl CompilePolicy {
    pub fn isolated_test() -> Self {
        Self {
            allow_research_draft: true,
        }
    }
}

impl CompiledBlock {
    /// Verify that the sidecar and the explicit fields still agree. This does
    /// not re-validate scientific correctness; it catches silent loss during
    /// serialization or handoff.
    pub fn validate_consistency(&self) -> Result<(), Vec<SemanticIssue>> {
        let mut issues = Vec::new();
        if self.block_id != self.binding.block_version.block_id {
            issues.push(issue(
                "block_id_mismatch",
                "block_id",
                "compiled block id does not match the local binding",
            ));
        }
        if self.block_version_id != self.binding.block_version.version_id {
            issues.push(issue(
                "block_version_id_mismatch",
                "block_version_id",
                "compiled block version id does not match the local binding",
            ));
        }
        if self.catalog != self.module_contract.identity.catalog {
            issues.push(issue(
                "catalog_mismatch",
                "catalog",
                "compiled catalog id does not match the module contract",
            ));
        }
        if self.inputs != self.module_contract.semantics.inputs {
            issues.push(issue(
                "inputs_mismatch",
                "inputs",
                "compiled inputs changed from the module contract",
            ));
        }
        if self.outputs != self.module_contract.semantics.outputs {
            issues.push(issue(
                "outputs_mismatch",
                "outputs",
                "compiled outputs changed from the module contract",
            ));
        }
        if self.sample_transitions != self.module_contract.semantics.sample_transitions {
            issues.push(issue(
                "transitions_mismatch",
                "sample_transitions",
                "compiled sample transitions changed from the module contract",
            ));
        }
        if self.instrument_requirements != self.module_contract.execution.instrument_requirements {
            issues.push(issue(
                "instruments_mismatch",
                "instrument_requirements",
                "compiled instrument requirements changed from the module contract",
            ));
        }
        let mut slot_keys = HashSet::new();
        for slot in &self.parameter_slots {
            if !slot_keys.insert(slot.key.as_str()) {
                issues.push(issue(
                    "duplicate_slot",
                    format!("parameter_slots.{}", slot.key),
                    "compiled parameter slots must be unique",
                ));
            }
        }
        let module_keys = self
            .module_contract
            .parameters
            .iter()
            .map(|parameter| parameter.key.as_str())
            .collect::<HashSet<_>>();
        if slot_keys != module_keys {
            issues.push(issue(
                "parameters_mismatch",
                "parameter_slots",
                "compiled parameter slots do not match the module contract",
            ));
        }
        if issues.is_empty() {
            Ok(())
        } else {
            Err(issues)
        }
    }
}

/// Compile one catalog module into a local block version. The module is never
/// inferred from the binding; both must name the same exact catalog version.
pub fn compile_operation_module(
    module: &OperationModule,
    binding: ModuleBinding,
    policy: CompilePolicy,
) -> Result<CompiledBlock, Vec<SemanticIssue>> {
    let mut issues = Vec::new();
    issues.extend(
        module
            .validate_contract()
            .err()
            .unwrap_or_default()
            .into_iter()
            .map(contract_issue),
    );
    issues.extend(
        binding
            .validate()
            .err()
            .unwrap_or_default()
            .into_iter()
            .map(contract_issue),
    );

    if module.knowledge.review_status == ReviewStatus::ResearchDraft && !policy.allow_research_draft
    {
        issues.push(issue(
            "review_gate",
            "knowledge.review_status",
            "research_draft modules may only compile in an isolated test policy",
        ));
    }
    if !binding_source_matches(&binding, &module.identity.catalog) {
        issues.push(issue(
            "catalog_binding_mismatch",
            "binding",
            "local binding does not name the module's exact catalog version",
        ));
    }
    if !issues.is_empty() {
        return Err(issues);
    }

    let parameter_slots = module
        .parameters
        .iter()
        .map(|parameter| CompiledParameterSlot {
            key: parameter.key.clone(),
            label: parameter.label.clone(),
            value_type: parameter.value_type,
            unit: parameter.unit.clone(),
            required: parameter.required,
        })
        .collect::<Vec<_>>();
    let parameter_refs = module
        .parameters
        .iter()
        .map(|parameter| parameter.key.clone())
        .collect::<Vec<_>>();
    let step = CompiledStep::Text {
        use_id: "execute".into(),
        title: module.identity.title.clone(),
        instruction: module.semantics.purpose.clone(),
        parameter_refs,
        time_scope: Some(module.execution.default_time_scope),
    };

    let compiled = CompiledBlock {
        block_id: binding.block_version.block_id.clone(),
        block_version_id: binding.block_version.version_id.clone(),
        binding,
        catalog: module.identity.catalog.clone(),
        summary: module.semantics.purpose.clone(),
        parameter_slots,
        steps: vec![step],
        inputs: module.semantics.inputs.clone(),
        outputs: module.semantics.outputs.clone(),
        sample_transitions: module.semantics.sample_transitions.clone(),
        instrument_requirements: module.execution.instrument_requirements.clone(),
        module_contract: module.clone(),
    };
    compiled.validate_consistency()?;
    Ok(compiled)
}

fn binding_source_matches(binding: &ModuleBinding, catalog: &CatalogModuleRef) -> bool {
    match binding.origin {
        ModuleOrigin::Imported => binding.catalog.as_ref() == Some(catalog),
        ModuleOrigin::Derived => binding.based_on.as_ref() == Some(catalog),
        ModuleOrigin::LocalCustom => false,
    }
}

fn contract_issue(issue: ContractIssue) -> SemanticIssue {
    SemanticIssue {
        code: issue.code,
        path: issue.path,
        message: issue.message,
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

#[cfg(test)]
mod tests {
    use super::*;
    use crate::operation_module::{
        ActionType, Applicability, CapabilityConstraint, CapabilityRequirement, Cardinality,
        EvidenceKind, EvidenceRef, ExecutionSpec, InstrumentRequirement, ModuleIdentity,
        ModuleKnowledge, ModuleSemantics, OperationParameter, ParameterValueType, PortObjectType,
        PortRole, ReviewStatus, SampleEffect, SampleTransition, ValueSource,
    };
    use crate::protocol_preview::StepTimeScope;

    fn catalog(version: u32) -> CatalogModuleRef {
        CatalogModuleRef {
            module_id: "op.liquid_transfer".into(),
            version,
        }
    }

    fn module(review: ReviewStatus) -> OperationModule {
        OperationModule {
            schema_version: crate::operation_module::OPERATION_MODULE_SCHEMA_VERSION,
            identity: ModuleIdentity {
                catalog: catalog(1),
                title: "Liquid transfer".into(),
                action_type: ActionType::LiquidTransfer,
                applicability: Applicability::default(),
                variant_of: None,
            },
            semantics: ModuleSemantics {
                purpose: "Move a measured volume".into(),
                preconditions: vec![],
                inputs: vec![PortSpec {
                    port_id: "sample_in".into(),
                    role: PortRole::SourceSample,
                    object_type: PortObjectType::Sample,
                    required: true,
                    cardinality: Cardinality::One,
                    description: "source".into(),
                }],
                outputs: vec![PortSpec {
                    port_id: "sample_out".into(),
                    role: PortRole::DestinationSample,
                    object_type: PortObjectType::Sample,
                    required: true,
                    cardinality: Cardinality::One,
                    description: "destination".into(),
                }],
                sample_transitions: vec![SampleTransition {
                    transition_id: "t1".into(),
                    input_port_id: "sample_in".into(),
                    output_port_id: Some("sample_out".into()),
                    effect: SampleEffect::Moved,
                    applies_when: None,
                    note: None,
                }],
            },
            parameters: vec![OperationParameter {
                key: "volume".into(),
                label: "Volume".into(),
                value_type: ParameterValueType::Number,
                unit: Some("uL".into()),
                required: true,
                required_at: crate::operation_module::RequiredAt::ProtocolFreeze,
                value_source: ValueSource::ProtocolFixed,
                constraints: vec![],
                description: "transfer volume".into(),
                evidence: vec![EvidenceRef {
                    evidence_id: "source-1".into(),
                    kind: EvidenceKind::LabSop,
                    citation: "fixture".into(),
                    url: None,
                    locator: None,
                    version: None,
                    retrieved_at: None,
                }],
            }],
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
                        key: "volume".into(),
                        constraint: CapabilityConstraint::Min {
                            value: 1.0,
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
                evidence: vec![EvidenceRef {
                    evidence_id: "module-source-1".into(),
                    kind: EvidenceKind::LabSop,
                    citation: "fixture".into(),
                    url: None,
                    locator: None,
                    version: None,
                    retrieved_at: None,
                }],
                review_status: review,
            },
        }
    }

    fn binding_for(catalog: CatalogModuleRef, block_id: &str, version_id: &str) -> ModuleBinding {
        ModuleBinding {
            catalog: Some(catalog),
            block_version: crate::operation_module::ProtocolBlockVersionRef {
                block_id: block_id.into(),
                version_id: version_id.into(),
            },
            origin: ModuleOrigin::Imported,
            based_on: None,
        }
    }

    fn binding(block_id: &str, version_id: &str) -> ModuleBinding {
        binding_for(catalog(1), block_id, version_id)
    }

    #[test]
    fn compiles_catalog_identity_contract_and_semantics() {
        let module = module(ReviewStatus::SourceChecked);
        let compiled = compile_operation_module(
            &module,
            binding("block-1", "block-v1"),
            CompilePolicy::default(),
        )
        .unwrap();
        assert_eq!(compiled.catalog, catalog(1));
        assert_eq!(compiled.block_id, "block-1");
        assert_eq!(compiled.block_version_id, "block-v1");
        assert_eq!(compiled.parameter_slots[0].key, "volume");
        assert_eq!(compiled.parameter_slots[0].unit.as_deref(), Some("uL"));
        assert_eq!(compiled.inputs, module.semantics.inputs);
        assert_eq!(compiled.outputs, module.semantics.outputs);
        assert_eq!(
            compiled.instrument_requirements,
            module.execution.instrument_requirements
        );
        assert_eq!(
            compiled.sample_transitions,
            module.semantics.sample_transitions
        );
        assert_eq!(compiled.module_contract, module);
        assert!(compiled.validate_consistency().is_ok());
    }

    #[test]
    fn liquid_transfer_incubation_and_absorbance_fixtures_compile() {
        for (action, module_id) in [
            (ActionType::LiquidTransfer, "op.liquid_transfer"),
            (ActionType::Incubation, "op.incubation"),
            (
                ActionType::AbsorbanceMeasurement,
                "op.absorbance_measurement",
            ),
        ] {
            let mut module = module(ReviewStatus::SourceChecked);
            module.identity.action_type = action;
            module.identity.catalog.module_id = module_id.into();
            let compiled = compile_operation_module(
                &module,
                binding_for(module.identity.catalog.clone(), "block-1", "v1"),
                CompilePolicy::default(),
            )
            .unwrap();
            assert_eq!(compiled.catalog.module_id, module_id);
            assert_eq!(compiled.module_contract.identity.action_type, action);
            assert_eq!(compiled.module_contract, module);
            assert!(compiled.validate_consistency().is_ok());
        }
    }

    #[test]
    fn research_draft_is_gated_but_isolated_test_policy_can_compile() {
        let module = module(ReviewStatus::ResearchDraft);
        let errors = compile_operation_module(
            &module,
            binding("block-1", "block-v1"),
            CompilePolicy::default(),
        )
        .unwrap_err();
        assert!(errors.iter().any(|issue| issue.code == "review_gate"));
        assert!(compile_operation_module(
            &module,
            binding("block-1", "block-v1"),
            CompilePolicy::isolated_test()
        )
        .is_ok());
    }

    #[test]
    fn mismatched_catalog_binding_is_rejected() {
        let module = module(ReviewStatus::SourceChecked);
        let mut local = binding("block-1", "block-v1");
        local.catalog = Some(CatalogModuleRef {
            module_id: "op.liquid_transfer".into(),
            version: 2,
        });
        let errors =
            compile_operation_module(&module, local, CompilePolicy::default()).unwrap_err();
        assert!(errors
            .iter()
            .any(|issue| issue.code == "catalog_binding_mismatch"));
    }

    #[test]
    fn same_module_compiles_to_independent_local_blocks() {
        let module = module(ReviewStatus::SourceChecked);
        let first = compile_operation_module(
            &module,
            binding("block-cck8", "v1"),
            CompilePolicy::default(),
        )
        .unwrap();
        let second = compile_operation_module(
            &module,
            binding("block-elisa", "v1"),
            CompilePolicy::default(),
        )
        .unwrap();
        assert_ne!(first.block_id, second.block_id);
        assert_eq!(first.catalog, second.catalog);
        assert_eq!(first.parameter_slots, second.parameter_slots);
    }

    #[test]
    fn local_custom_binding_cannot_claim_catalog_module() {
        let module = module(ReviewStatus::SourceChecked);
        let mut local = binding("block-1", "block-v1");
        local.origin = ModuleOrigin::LocalCustom;
        local.catalog = None;
        let errors =
            compile_operation_module(&module, local, CompilePolicy::default()).unwrap_err();
        assert!(errors
            .iter()
            .any(|issue| issue.code == "catalog_binding_mismatch"));
    }
}
