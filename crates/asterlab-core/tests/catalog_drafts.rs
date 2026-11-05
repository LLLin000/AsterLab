use asterlab_core::instrument_matching::{match_requirement, InstrumentProfile, MatchStatus};
use asterlab_core::operation_module::OperationModule;

fn operation(json: &str) -> OperationModule {
    serde_json::from_str(json).expect("operation JSON must match the contract")
}

fn profile(json: &str) -> InstrumentProfile {
    serde_json::from_str(json).expect("instrument profile JSON must match the contract")
}

#[test]
fn operation_drafts_validate_structurally() {
    let liquid = operation(include_str!(
        "../../../catalog/operations/liquid-transfer.v1.json"
    ));
    liquid.validate_contract().expect("liquid-transfer draft");
    let absorbance = operation(include_str!(
        "../../../catalog/operations/absorbance-measurement.v1.json"
    ));
    absorbance.validate_contract().expect("absorbance draft");
}

#[test]
fn instrument_drafts_validate_but_remain_research_drafts() {
    let pipette = profile(include_str!(
        "../../../catalog/instruments/liquid-handler.eppendorf-research-plus.single-channel.100-1000.v1.json"
    ));
    pipette.validate().expect("pipette draft");
    assert!(!pipette.is_review_verified());

    let reader = profile(include_str!(
        "../../../catalog/instruments/plate-reader.multiskan-skyhigh.v1.json"
    ));
    reader.validate().expect("reader draft");
    assert!(!reader.is_review_verified());
}

#[test]
fn drafts_cannot_produce_compatible_matches() {
    let liquid = operation(include_str!(
        "../../../catalog/operations/liquid-transfer.v1.json"
    ));
    let pipette = profile(include_str!(
        "../../../catalog/instruments/liquid-handler.eppendorf-research-plus.single-channel.100-1000.v1.json"
    ));
    let result = match_requirement(
        &liquid.execution.instrument_requirements[0],
        Some(&pipette),
        None,
        None,
    );
    assert_eq!(result.status, MatchStatus::Unknown);

    let absorbance = operation(include_str!(
        "../../../catalog/operations/absorbance-measurement.v1.json"
    ));
    let reader = profile(include_str!(
        "../../../catalog/instruments/plate-reader.multiskan-skyhigh.v1.json"
    ));
    let result = match_requirement(
        &absorbance.execution.instrument_requirements[0],
        Some(&reader),
        None,
        None,
    );
    assert_eq!(result.status, MatchStatus::Unknown);
}
