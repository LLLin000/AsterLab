//! S2-A/B minimal Catalog Compiler and formal block mapping.
//!
//! Compile an `OperationModule` into a project-local block version and map it
//! onto the formal `ProtocolBlockVersionData` shape. The catalog identity,
//! module-contract sidecar, ports, instrument requirements and sample
//! transitions are preserved. This is pure Core: no SQLite, no second
//! Protocol/Run, no mutation of frozen records.

use crate::operation_module::{
    CatalogModuleRef, ContractIssue, InstrumentRequirement, ModuleBinding, ModuleOrigin,
    OperationModule, PortSpec, ReviewStatus, SampleTransition,
};
use crate::protocol_block::{ParameterSlot, ProtocolBlockVersionData, ProtocolStep, ReviewRecord};
use crate::protocol_preview::StepTimeScope;
use crate::semantic_component::{ProtocolScope, SampleExpectation, SemanticIssue};
use crate::semantic_composition::{
    BlockUseSource, PortDirection, PortKind, PortSpec as CompositionPortSpec,
};
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
        let expected_slots = self
            .module_contract
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
        if self.parameter_slots != expected_slots {
            issues.push(issue(
                "parameters_mismatch",
                "parameter_slots",
                "compiled parameter slots differ from the module contract",
            ));
        }
        if issues.is_empty() {
            Ok(())
        } else {
            Err(issues)
        }
    }
}

impl CompiledBlock {
    /// Map the intermediate representation onto the formal persisted block
    /// shape. Old blocks omit the sidecar; new compiled blocks keep it.
    pub fn to_block_version_data(
        &self,
        version: i64,
    ) -> Result<ProtocolBlockVersionData, Vec<SemanticIssue>> {
        let mut issues = self.validate_consistency().err().unwrap_or_default();
        if version < 1 {
            issues.push(issue(
                "invalid_version",
                "version",
                "ProtocolBlockVersion version must be >= 1",
            ));
        }
        if !issues.is_empty() {
            return Err(issues);
        }
        let steps = self
            .steps
            .iter()
            .map(|step| match step {
                CompiledStep::Text {
                    use_id,
                    title,
                    instruction,
                    parameter_refs,
                    time_scope,
                } => ProtocolStep::Text {
                    use_id: use_id.clone(),
                    title: title.clone(),
                    instruction: instruction.clone(),
                    parameter_refs: parameter_refs.clone(),
                    visualization: None,
                    time_scope: *time_scope,
                },
            })
            .collect();
        Ok(ProtocolBlockVersionData {
            block_id: self.block_id.clone(),
            version,
            summary: self.summary.clone(),
            parameter_slots: self
                .parameter_slots
                .iter()
                .map(|slot| ParameterSlot {
                    key: slot.key.clone(),
                    label: slot.label.clone(),
                    value_type: slot.value_type,
                    unit: slot.unit.clone(),
                    required: slot.required,
                })
                .collect(),
            steps,
            module_contract: Some(self.module_contract.clone()),
            module_origin: Some(self.binding.origin),
            catalog_review: None,
        })
    }

    /// Build the formal `ProtocolStep::BlockUse` node that a method template
    /// uses to reference this compiled block. Bindings stay local to the use.
    pub fn to_block_use_step(
        &self,
        use_id: impl Into<String>,
        bindings: Vec<crate::protocol_block::StepBinding>,
    ) -> ProtocolStep {
        ProtocolStep::BlockUse {
            use_id: use_id.into(),
            block_version_id: self.block_version_id.clone(),
            bindings,
        }
    }

    /// Read-only adapter for the existing recursive BlockUse shape. A catalog
    /// module becomes one local block use; it does not create a second Protocol.
    pub fn to_block_use_source(&self, scope: ProtocolScope, use_id: &str) -> BlockUseSource {
        let mut ports = Vec::new();
        for port in &self.inputs {
            ports.push(map_port(port, PortDirection::Input));
        }
        for port in &self.outputs {
            ports.push(map_port(port, PortDirection::Output));
        }
        BlockUseSource {
            use_id: use_id.into(),
            block_version_id: self.block_version_id.clone(),
            scope,
            ports,
            children: Vec::new(),
        }
    }
}

fn map_port(port: &PortSpec, direction: PortDirection) -> CompositionPortSpec {
    use crate::operation_module::PortRole;
    let kind = match port.role {
        PortRole::SourceSample | PortRole::DestinationSample => PortKind::Sample,
        PortRole::RawMeasurement | PortRole::DerivedResult | PortRole::ReferenceData => {
            PortKind::Data
        }
        PortRole::Instrument => PortKind::Resource,
        PortRole::Reagent | PortRole::Consumable | PortRole::DestinationContainer => {
            PortKind::Material
        }
    };
    let sample_expectation = matches!(
        port.role,
        PortRole::SourceSample | PortRole::DestinationSample
    )
    .then_some(SampleExpectation::Planned);
    CompositionPortSpec {
        id: port.port_id.clone(),
        direction,
        kind,
        unit: None,
        sample_expectation,
    }
}

/// Compile one catalog module into a local block version. The module is never
/// inferred from the binding; both must name the same exact catalog version.
pub fn compile_operation_module(
    module: &OperationModule,
    binding: ModuleBinding,
    policy: CompilePolicy,
    review: Option<&ReviewRecord>,
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
    if !policy.allow_research_draft {
        match review {
            Some(review) => issues.extend(
                review
                    .validate_for(module)
                    .into_iter()
                    .map(|issue| {
                        crate::semantic_component::SemanticIssue {
                            code: issue.code,
                            path: "review".into(),
                            message: issue.message,
                        }
                    }),
            ),
            None => issues.push(issue(
                "review_record_required",
                "review",
                "formal compilation requires an immutable review record for the exact catalog version",
            )),
        }
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
    use crate::protocol_block::module_content_digest;
    use crate::protocol_preview::StepTimeScope;
    use crate::semantic_composition::composition_from_block_uses;

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

    fn review_for(module: &OperationModule) -> ReviewRecord {
        ReviewRecord {
            review_id: "review-1".into(),
            subject: module.identity.catalog.clone(),
            status: ReviewStatus::SourceChecked,
            reviewer: "reviewer-1".into(),
            reviewed_at: "2026-11-05T00:00:00Z".into(),
            evidence_id: "module-source-1".into(),
            evidence_locator: "fixture:1".into(),
            content_digest: module_content_digest(module),
        }
    }

    #[test]
    fn compiles_catalog_identity_contract_and_semantics() {
        let module = module(ReviewStatus::SourceChecked);
        let compiled = compile_operation_module(
            &module,
            binding("block-1", "block-v1"),
            CompilePolicy::default(),
            Some(&review_for(&module)),
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

    fn specialized_module(action: ActionType) -> OperationModule {
        let mut module = module(ReviewStatus::SourceChecked);
        module.identity.action_type = action;
        match action {
            ActionType::LiquidTransfer => {}
            ActionType::Incubation => {
                module.identity.catalog.module_id = "op.incubation".into();
                module.identity.title = "Incubation".into();
                module.semantics.purpose = "Hold the sample in controlled conditions".into();
                let parameter = &mut module.parameters[0];
                parameter.key = "duration".into();
                parameter.label = "Duration".into();
                parameter.unit = Some("min".into());
            }
            ActionType::AbsorbanceMeasurement => {
                module.identity.catalog.module_id = "op.absorbance_measurement".into();
                module.identity.title = "Absorbance measurement".into();
                module.semantics.purpose = "Read absorbance at a configured wavelength".into();
                let parameter = &mut module.parameters[0];
                parameter.key = "wavelength".into();
                parameter.label = "Wavelength".into();
                parameter.unit = Some("nm".into());
                module.semantics.outputs = vec![PortSpec {
                    port_id: "measurement".into(),
                    role: PortRole::RawMeasurement,
                    object_type: PortObjectType::Measurement,
                    required: true,
                    cardinality: Cardinality::One,
                    description: "raw absorbance".into(),
                }];
                module.semantics.sample_transitions = vec![SampleTransition {
                    transition_id: "t1".into(),
                    input_port_id: "sample_in".into(),
                    output_port_id: None,
                    effect: SampleEffect::Terminal,
                    applies_when: None,
                    note: None,
                }];
            }
            _ => panic!("unsupported fixture action"),
        }
        module
    }

    #[test]
    fn liquid_transfer_incubation_and_absorbance_fixtures_compile() {
        for (action, module_id, parameter, unit) in [
            (
                ActionType::LiquidTransfer,
                "op.liquid_transfer",
                "volume",
                "uL",
            ),
            (ActionType::Incubation, "op.incubation", "duration", "min"),
            (
                ActionType::AbsorbanceMeasurement,
                "op.absorbance_measurement",
                "wavelength",
                "nm",
            ),
        ] {
            let module = specialized_module(action);
            let compiled = compile_operation_module(
                &module,
                binding_for(module.identity.catalog.clone(), "block-1", "v1"),
                CompilePolicy::default(),
                Some(&review_for(&module)),
            )
            .unwrap();
            assert_eq!(compiled.catalog.module_id, module_id);
            assert_eq!(compiled.module_contract.identity.action_type, action);
            assert_eq!(compiled.parameter_slots[0].key, parameter);
            assert_eq!(compiled.parameter_slots[0].unit.as_deref(), Some(unit));
            assert_eq!(compiled.module_contract, module);
            assert!(compiled.validate_consistency().is_ok());
        }
    }

    #[test]
    fn formal_compilation_requires_matching_review_record() {
        let module = module(ReviewStatus::SourceChecked);
        let errors = compile_operation_module(
            &module,
            binding("block-1", "block-v1"),
            CompilePolicy::default(),
            None,
        )
        .unwrap_err();
        assert!(errors
            .iter()
            .any(|issue| issue.code == "review_record_required"));

        let mut wrong = review_for(&module);
        wrong.content_digest = "not-the-digest".into();
        let errors = compile_operation_module(
            &module,
            binding("block-1", "block-v1"),
            CompilePolicy::default(),
            Some(&wrong),
        )
        .unwrap_err();
        assert!(errors
            .iter()
            .any(|issue| issue.code == "review_content_digest"));

        let mut wrong = review_for(&module);
        wrong.subject.version = 2;
        let errors = compile_operation_module(
            &module,
            binding("block-1", "block-v1"),
            CompilePolicy::default(),
            Some(&wrong),
        )
        .unwrap_err();
        assert!(errors
            .iter()
            .any(|issue| issue.code == "review_subject_mismatch"));
    }

    #[test]
    fn formal_block_version_keeps_sidecar_and_old_blocks_stay_readable() {
        let module = module(ReviewStatus::SourceChecked);
        let compiled = compile_operation_module(
            &module,
            binding("block-1", "block-v1"),
            CompilePolicy::default(),
            Some(&review_for(&module)),
        )
        .unwrap();
        let data = compiled.to_block_version_data(1).unwrap();
        assert_eq!(data.block_id, "block-1");
        assert_eq!(data.version, 1);
        assert_eq!(data.module_contract.as_ref(), Some(&module));
        assert_eq!(data.module_origin, Some(ModuleOrigin::Imported));
        let serialized = serde_json::to_value(&data).unwrap();
        assert!(serialized.get("module_contract").is_some());
        let round_trip: ProtocolBlockVersionData =
            serde_json::from_value(serialized.clone()).unwrap();
        assert_eq!(round_trip, data);

        let mut old = serialized;
        old.as_object_mut().unwrap().remove("module_contract");
        let old_block: ProtocolBlockVersionData = serde_json::from_value(old).unwrap();
        assert!(old_block.module_contract.is_none());
    }

    #[test]
    fn consistency_catches_changed_parameter_metadata() {
        let module = module(ReviewStatus::SourceChecked);
        let mut compiled = compile_operation_module(
            &module,
            binding("block-1", "block-v1"),
            CompilePolicy::default(),
            Some(&review_for(&module)),
        )
        .unwrap();
        compiled.parameter_slots[0].unit = Some("mL".into());
        assert!(compiled
            .validate_consistency()
            .unwrap_err()
            .iter()
            .any(|issue| issue.code == "parameters_mismatch"));
    }

    #[test]
    fn compiled_blocks_feed_recursive_composition_and_scope_identity() {
        let liquid = module(ReviewStatus::SourceChecked);
        let compiled = compile_operation_module(
            &liquid,
            binding("block-liquid", "v1"),
            CompilePolicy::default(),
            Some(&review_for(&liquid)),
        )
        .unwrap();
        let roots = vec![
            compiled.to_block_use_source(
                ProtocolScope::Assay {
                    assay_id: "cck8".into(),
                },
                "seed",
            ),
            compiled.to_block_use_source(
                ProtocolScope::Assay {
                    assay_id: "live_dead".into(),
                },
                "seed",
            ),
            compiled.to_block_use_source(
                ProtocolScope::Assay {
                    assay_id: "cck8".into(),
                },
                "transfer",
            ),
        ];
        let protocol = composition_from_block_uses("protocol-v1", roots);
        assert!(protocol.validate().is_ok());
    }

    #[test]
    fn cck8_composition_reuses_liquid_transfer_and_isolates_assay_scope() {
        let liquid = specialized_module(ActionType::LiquidTransfer);
        let incubation = specialized_module(ActionType::Incubation);
        let absorbance = specialized_module(ActionType::AbsorbanceMeasurement);
        let compile = |module: &OperationModule, block: &str| {
            compile_operation_module(
                module,
                binding_for(module.identity.catalog.clone(), block, "v1"),
                CompilePolicy::default(),
                Some(&review_for(module)),
            )
            .unwrap()
        };
        let liquid = compile(&liquid, "block-liquid");
        let incubation = compile(&incubation, "block-incubation");
        let absorbance = compile(&absorbance, "block-absorbance");
        let cck8 = ProtocolScope::Assay {
            assay_id: "cck8".into(),
        };
        let live_dead = ProtocolScope::Assay {
            assay_id: "live_dead".into(),
        };
        let roots = vec![
            liquid.to_block_use_source(cck8.clone(), "seed"),
            liquid.to_block_use_source(cck8.clone(), "treat"),
            incubation.to_block_use_source(cck8.clone(), "incubate"),
            absorbance.to_block_use_source(cck8.clone(), "read"),
            liquid.to_block_use_source(live_dead.clone(), "seed"),
            absorbance.to_block_use_source(live_dead, "read"),
        ];
        let protocol = composition_from_block_uses("protocol-v1", roots);
        assert!(protocol.validate().is_ok());
        let paths = protocol
            .roots
            .iter()
            .map(|root| (root.scope.key(), root.step_path.as_str()))
            .collect::<Vec<_>>();
        assert!(paths.contains(&(cck8.key(), "seed")));
        assert!(paths.contains(&(cck8.key(), "treat")));
        assert!(paths.contains(&(
            ProtocolScope::Assay {
                assay_id: "live_dead".into()
            }
            .key(),
            "seed"
        )));
        let block_data = absorbance.to_block_version_data(1).unwrap();
        assert!(block_data.module_contract.is_some());
        assert_eq!(block_data.parameter_slots[0].unit.as_deref(), Some("nm"));

        let method_step = liquid.to_block_use_step(
            "seed",
            vec![crate::protocol_block::StepBinding {
                slot: "volume".into(),
                value: Some(serde_json::json!(100.0)),
                source_slot: None,
                unit: Some("uL".into()),
            }],
        );
        match method_step {
            ProtocolStep::BlockUse {
                use_id,
                block_version_id,
                bindings,
            } => {
                assert_eq!(use_id, "seed");
                assert_eq!(block_version_id, "v1");
                assert_eq!(bindings.len(), 1);
            }
            ProtocolStep::Text { .. } => panic!("expected BlockUse"),
        }
    }

    #[test]
    fn research_draft_is_gated_but_isolated_test_policy_can_compile() {
        let module = module(ReviewStatus::ResearchDraft);
        let errors = compile_operation_module(
            &module,
            binding("block-1", "block-v1"),
            CompilePolicy::default(),
            Some(&review_for(&module)),
        )
        .unwrap_err();
        assert!(errors.iter().any(|issue| issue.code == "review_gate"));
        assert!(compile_operation_module(
            &module,
            binding("block-1", "block-v1"),
            CompilePolicy::isolated_test(),
            None
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
        let errors = compile_operation_module(
            &module,
            local,
            CompilePolicy::default(),
            Some(&review_for(&module)),
        )
        .unwrap_err();
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
            Some(&review_for(&module)),
        )
        .unwrap();
        let second = compile_operation_module(
            &module,
            binding("block-elisa", "v1"),
            CompilePolicy::default(),
            Some(&review_for(&module)),
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
        let errors = compile_operation_module(
            &module,
            local,
            CompilePolicy::default(),
            Some(&review_for(&module)),
        )
        .unwrap_err();
        assert!(errors
            .iter()
            .any(|issue| issue.code == "catalog_binding_mismatch"));
    }
}
