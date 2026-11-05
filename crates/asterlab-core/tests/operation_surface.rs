use asterlab_core::operation_module::ValueSource;
use asterlab_core::operation_module::{
    OperationModule, ParameterConstraint, ParameterValueType, SampleEffect,
};
use asterlab_core::protocol_preview::StepTimeScope;

fn load(json: &str) -> OperationModule {
    serde_json::from_str(json).expect("operation JSON must match the contract")
}

#[test]
fn liquid_transfer_exposes_conditional_modes_without_a_family_range() {
    let operation = load(include_str!(
        "../../../catalog/operations/liquid-transfer.v1.json"
    ));
    operation.validate_contract().expect("liquid transfer");

    let modes = operation
        .semantics
        .sample_transitions
        .iter()
        .filter_map(|transition| transition.applies_when.as_ref())
        .map(|condition| condition.value.as_str())
        .collect::<Vec<_>>();
    assert!(modes.contains(&"partial"));
    assert!(modes.contains(&"full"));
    assert!(modes.contains(&"add_to_existing"));

    let volume = operation
        .parameters
        .iter()
        .find(|parameter| parameter.key == "volume")
        .expect("volume parameter");
    assert_eq!(volume.value_type, ParameterValueType::Number);
    assert!(volume
        .constraints
        .iter()
        .any(|constraint| matches!(constraint, ParameterConstraint::Positive)));
    assert!(!volume.constraints.iter().any(|constraint| matches!(
        constraint,
        ParameterConstraint::Min { .. } | ParameterConstraint::Max { .. }
    )));
    assert!(operation
        .execution
        .allowed_time_scopes
        .contains(&StepTimeScope::PerTimepoint));
}

#[test]
fn aliquot_keeps_parent_and_child_identity() {
    let operation = load(include_str!(
        "../../../catalog/operations/sample-aliquot.v1.json"
    ));
    operation.validate_contract().expect("aliquot");
    assert!(operation
        .semantics
        .sample_transitions
        .iter()
        .any(|transition| {
            transition.effect == SampleEffect::Aliquoted
                && transition.output_port_id.as_deref() == Some("aliquot")
        }));
    assert!(operation
        .semantics
        .sample_transitions
        .iter()
        .any(|transition| {
            transition.effect == SampleEffect::Preserved
                && transition.output_port_id.as_deref() == Some("parent_remaining")
        }));
    assert!(operation
        .execution
        .record_fields
        .iter()
        .any(|field| field.key == "parent_sample_id"));
}

#[test]
fn incubation_has_an_explicit_time_anchor_and_state_change() {
    let operation = load(include_str!(
        "../../../catalog/operations/incubation.v1.json"
    ));
    operation.validate_contract().expect("incubation");
    let anchor = operation
        .parameters
        .iter()
        .find(|parameter| parameter.key == "time_anchor")
        .expect("time anchor");
    assert_eq!(anchor.value_type, ParameterValueType::Text);
    assert!(anchor
        .constraints
        .iter()
        .any(|constraint| matches!(constraint, ParameterConstraint::NonEmpty)));
    assert!(operation
        .semantics
        .sample_transitions
        .iter()
        .any(|transition| transition.effect == SampleEffect::Unknown));
    assert_eq!(operation.execution.default_time_scope, StepTimeScope::Once);
}

#[test]
fn washing_separates_waste_from_sample_state_change() {
    let operation = load(include_str!("../../../catalog/operations/washing.v1.json"));
    operation.validate_contract().expect("washing");
    assert!(operation
        .semantics
        .sample_transitions
        .iter()
        .any(|transition| {
            transition.input_port_id == "wash_buffer"
                && transition.output_port_id.as_deref() == Some("waste")
                && transition.effect == SampleEffect::Consumed
        }));
    let preserving = operation
        .semantics
        .sample_transitions
        .iter()
        .find(|transition| transition.transition_id == "sample_washed_preserving")
        .expect("preserving transition");
    assert_eq!(preserving.effect, SampleEffect::Preserved);
    assert_eq!(
        preserving
            .applies_when
            .as_ref()
            .map(|condition| condition.value.as_str()),
        Some("preserving")
    );
    let lossy = operation
        .semantics
        .sample_transitions
        .iter()
        .find(|transition| transition.transition_id == "sample_washed_lossy")
        .expect("lossy transition");
    assert_eq!(lossy.effect, SampleEffect::StateChanged);
    assert_eq!(
        lossy
            .applies_when
            .as_ref()
            .map(|condition| condition.value.as_str()),
        Some("lossy_possible")
    );
    assert!(operation.execution.instrument_requirements.is_empty());
}

#[test]
fn absorbance_keeps_method_controls_out_of_the_generic_operation() {
    let operation = load(include_str!(
        "../../../catalog/operations/absorbance-measurement.v1.json"
    ));
    operation.validate_contract().expect("absorbance");
    assert!(operation.knowledge.controls.is_empty());
    assert!(!operation
        .parameters
        .iter()
        .any(|parameter| parameter.key == "blank_wells"));
    let readout = operation.knowledge.readout.as_ref().expect("readout");
    assert!(readout.not_implied.contains("Viability"));
    assert!(operation
        .parameters
        .iter()
        .find(|parameter| parameter.key == "wavelength")
        .expect("wavelength")
        .constraints
        .iter()
        .all(|constraint| !matches!(
            constraint,
            ParameterConstraint::Min { .. } | ParameterConstraint::Max { .. }
        )));
}

#[test]
fn all_five_drafts_are_research_drafts_with_evidence() {
    let drafts = [
        include_str!("../../../catalog/operations/liquid-transfer.v1.json"),
        include_str!("../../../catalog/operations/sample-aliquot.v1.json"),
        include_str!("../../../catalog/operations/incubation.v1.json"),
        include_str!("../../../catalog/operations/washing.v1.json"),
        include_str!("../../../catalog/operations/absorbance-measurement.v1.json"),
    ];
    for json in drafts {
        let operation = load(json);
        assert_eq!(
            operation.knowledge.review_status,
            asterlab_core::operation_module::ReviewStatus::ResearchDraft
        );
        assert!(!operation.knowledge.evidence.is_empty());
        assert!(operation
            .parameters
            .iter()
            .all(|parameter| parameter.value_source != ValueSource::Unknown
                || parameter.required_at != asterlab_core::operation_module::RequiredAt::Draft));
    }
}
