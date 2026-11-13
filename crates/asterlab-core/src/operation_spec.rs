//! Typed operation contract (OperationSpec v1).
//!
//! `ProtocolBlock` stays the authority for what a protocol executes. An
//! [`OperationSpec`] is a typed description of one operation inside a block; it
//! never carries a second execution path. Blocks without a spec keep running
//! (v1 compatibility). The normative contract lives in
//! `docs/contracts/operation-spec-v1.md`.
//!
//! This module is contract-only: types, serde and structural validation. It
//! has no executor, no store access and no CLI/GUI wiring.
//!
//! ```
//! use asterlab_core::operation_spec::OperationSpec;
//!
//! let json = r#"{
//!   "schema_version": 1,
//!   "operation": {
//!     "kind": "transfer",
//!     "volume": { "value": 100.0, "unit": "uL" },
//!     "source": { "container_id": "tube-1" },
//!     "destination": { "container_id": "plate-1" }
//!   }
//! }"#;
//! let spec: OperationSpec = serde_json::from_str(json).expect("spec parses");
//! assert!(spec.is_supported_schema_version());
//! assert!(spec.validate().is_ok());
//! ```
//!
//! A wrong unit family is a semantic rejection, not a parse error:
//!
//! ```
//! use asterlab_core::operation_spec::{OperationSpec, SpecErrorCode};
//!
//! let json = r#"{
//!   "schema_version": 1,
//!   "operation": {
//!     "kind": "incubate",
//!     "temperature": { "value": 37.0, "unit": "min" },
//!     "duration": { "value": 30.0, "unit": "min" }
//!   }
//! }"#;
//! let spec: OperationSpec = serde_json::from_str(json).expect("spec parses");
//! let issues = spec.validate().expect_err("minutes are not a temperature");
//! assert_eq!(issues[0].code, SpecErrorCode::UnitMismatch);
//! assert_eq!(issues[0].path, "operation.temperature");
//! ```

use crate::protocol_preview::StepCheck;
use serde::{Deserialize, Serialize};

/// Schema version written and accepted by this crate.
pub const OPERATION_SPEC_SCHEMA_VERSION: u32 = 1;

/// The three semantic rejection classes. This set is closed: no other code is
/// produced by [`OperationSpec::validate`].
pub const OPERATION_SPEC_ERROR_CODES: [SpecErrorCode; 3] = [
    SpecErrorCode::UnitMismatch,
    SpecErrorCode::MissingParameter,
    SpecErrorCode::ValueOutOfRange,
];

/// Lowest physically meaningful Celsius value; nothing can be colder.
const ABSOLUTE_ZERO_CELSIUS: f64 = -273.15;

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum OperationKind {
    Transfer,
    Centrifuge,
    Incubate,
    Wash,
    Aliquot,
    Measure,
}

/// Declared unit. Values are never converted in v1: the value and the unit stay
/// exactly as declared. Unknown unit tokens are rejected by serde, which is a
/// parse error and therefore outside the closed semantic error set.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum Unit {
    #[serde(rename = "uL")]
    Microliter,
    #[serde(rename = "mL")]
    Milliliter,
    #[serde(rename = "L")]
    Liter,
    #[serde(rename = "s")]
    Second,
    #[serde(rename = "min")]
    Minute,
    #[serde(rename = "h")]
    Hour,
    #[serde(rename = "C")]
    Celsius,
    /// Relative centrifugal force, relative to standard gravity.
    #[serde(rename = "xg")]
    RcfG,
    #[serde(rename = "rpm")]
    Rpm,
    #[serde(rename = "AU")]
    AbsorbanceUnit,
    #[serde(rename = "RFU")]
    RelativeFluorescenceUnit,
    #[serde(rename = "count")]
    Count,
}

/// Dimension of a [`Unit`]. A field declares the family it accepts, so a wrong
/// unit is a semantic rejection instead of a silently accepted number.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum UnitFamily {
    Volume,
    Time,
    Temperature,
    Force,
    Rotation,
    Measurement,
}

impl UnitFamily {
    pub fn as_str(self) -> &'static str {
        match self {
            UnitFamily::Volume => "volume",
            UnitFamily::Time => "time",
            UnitFamily::Temperature => "temperature",
            UnitFamily::Force => "force",
            UnitFamily::Rotation => "rotation",
            UnitFamily::Measurement => "measurement",
        }
    }
}

impl Unit {
    pub fn family(self) -> UnitFamily {
        match self {
            Unit::Microliter | Unit::Milliliter | Unit::Liter => UnitFamily::Volume,
            Unit::Second | Unit::Minute | Unit::Hour => UnitFamily::Time,
            Unit::Celsius => UnitFamily::Temperature,
            Unit::RcfG => UnitFamily::Force,
            Unit::Rpm => UnitFamily::Rotation,
            Unit::AbsorbanceUnit | Unit::RelativeFluorescenceUnit | Unit::Count => {
                UnitFamily::Measurement
            }
        }
    }

    pub fn as_str(self) -> &'static str {
        match self {
            Unit::Microliter => "uL",
            Unit::Milliliter => "mL",
            Unit::Liter => "L",
            Unit::Second => "s",
            Unit::Minute => "min",
            Unit::Hour => "h",
            Unit::Celsius => "C",
            Unit::RcfG => "xg",
            Unit::Rpm => "rpm",
            Unit::AbsorbanceUnit => "AU",
            Unit::RelativeFluorescenceUnit => "RFU",
            Unit::Count => "count",
        }
    }
}

/// A number plus its declared unit. No conversion, no default unit.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct Quantity {
    pub value: f64,
    pub unit: Unit,
}

/// A stable project container reference. A bare well coordinate (`A1`) is not a
/// container id; wells stay `plate_id + coordinate` in their own type.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct ContainerRef {
    pub container_id: String,
}

#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct TransferSpec {
    /// `None` means the draft has not fixed a volume yet.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub volume: Option<Quantity>,
    pub source: ContainerRef,
    pub destination: ContainerRef,
}

#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct RcfDrive {
    /// Must be declared in `xg`.
    pub force: Quantity,
}

#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct SpeedDrive {
    /// Must be declared in `rpm`. No rotor radius is assumed, so the value is
    /// never converted to RCF.
    pub speed: Quantity,
}

/// Centrifuge drive is force or speed, never both.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(tag = "kind", rename_all = "snake_case")]
pub enum CentrifugeDrive {
    Rcf(RcfDrive),
    Speed(SpeedDrive),
}

#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct CentrifugeSpec {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub drive: Option<CentrifugeDrive>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub duration: Option<Quantity>,
}

#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct IncubateSpec {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub temperature: Option<Quantity>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub duration: Option<Quantity>,
}

#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct WashSpec {
    /// Number of wash cycles.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub cycles: Option<u32>,
    /// Volume per wash cycle.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub volume: Option<Quantity>,
}

#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct AliquotSpec {
    /// Number of parts produced by one split.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub parts: Option<u32>,
    /// Volume of each part.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub per_part_volume: Option<Quantity>,
}

#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct MeasureSpec {
    /// Stable reference to a design `Endpoint`. A free-text label is not an
    /// endpoint id.
    pub endpoint_id: String,
    /// Reading unit; must belong to the measurement family. The numeric reading
    /// itself is an execution fact and is never stored in a spec.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub unit: Option<Unit>,
}

/// One of the six typed operations, tagged by `kind`.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(tag = "kind", rename_all = "snake_case")]
pub enum Operation {
    Transfer(TransferSpec),
    Centrifuge(CentrifugeSpec),
    Incubate(IncubateSpec),
    Wash(WashSpec),
    Aliquot(AliquotSpec),
    Measure(MeasureSpec),
}

impl Operation {
    pub fn kind(&self) -> OperationKind {
        match self {
            Operation::Transfer(_) => OperationKind::Transfer,
            Operation::Centrifuge(_) => OperationKind::Centrifuge,
            Operation::Incubate(_) => OperationKind::Incubate,
            Operation::Wash(_) => OperationKind::Wash,
            Operation::Aliquot(_) => OperationKind::Aliquot,
            Operation::Measure(_) => OperationKind::Measure,
        }
    }
}

/// Typed description of one operation. Execution semantics stay in the owning
/// `ProtocolBlockVersionData`; this is a sidecar-shaped description, not a
/// runtime.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct OperationSpec {
    pub schema_version: u32,
    pub operation: Operation,
    /// Operation-level completion checks. Same type as the step checks in
    /// `protocol_preview`; the spec never defines a second check vocabulary.
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub checks: Vec<StepCheck>,
}

impl OperationSpec {
    pub fn v1(operation: Operation) -> Self {
        OperationSpec {
            schema_version: OPERATION_SPEC_SCHEMA_VERSION,
            operation,
            checks: Vec::new(),
        }
    }

    /// Version refusal is a load-time decision, not one of the three semantic
    /// codes. Consumers must check this before trusting a loaded spec.
    pub fn is_supported_schema_version(&self) -> bool {
        self.schema_version == OPERATION_SPEC_SCHEMA_VERSION
    }

    /// Structural checks over the spec's own fields.
    ///
    /// Not checked here: whether the owning block declares the same values,
    /// scientific appropriateness, instrument compatibility, and execution
    /// facts. Those stay with block validation and the Run.
    pub fn validate(&self) -> Result<(), Vec<SpecIssue>> {
        let mut issues = Vec::new();

        match &self.operation {
            Operation::Transfer(spec) => {
                match &spec.volume {
                    Some(volume) => validate_quantity(
                        volume,
                        "operation.volume",
                        UnitFamily::Volume,
                        &mut issues,
                    ),
                    None => issues.push(missing("operation.volume", "transfer volume is not set")),
                }
                validate_container_id(
                    &spec.source.container_id,
                    "operation.source.container_id",
                    &mut issues,
                );
                validate_container_id(
                    &spec.destination.container_id,
                    "operation.destination.container_id",
                    &mut issues,
                );
            }
            Operation::Centrifuge(spec) => {
                match &spec.drive {
                    Some(CentrifugeDrive::Rcf(drive)) => validate_quantity(
                        &drive.force,
                        "operation.drive.force",
                        UnitFamily::Force,
                        &mut issues,
                    ),
                    Some(CentrifugeDrive::Speed(drive)) => validate_quantity(
                        &drive.speed,
                        "operation.drive.speed",
                        UnitFamily::Rotation,
                        &mut issues,
                    ),
                    None => issues.push(missing(
                        "operation.drive",
                        "centrifuge force or speed is not set",
                    )),
                }
                match &spec.duration {
                    Some(duration) => {
                        validate_quantity(duration, "operation.duration", UnitFamily::Time, &mut issues)
                    }
                    None => {
                        issues.push(missing("operation.duration", "centrifuge duration is not set"))
                    }
                }
            }
            Operation::Incubate(spec) => {
                match &spec.temperature {
                    Some(temperature) => validate_quantity(
                        temperature,
                        "operation.temperature",
                        UnitFamily::Temperature,
                        &mut issues,
                    ),
                    None => issues.push(missing(
                        "operation.temperature",
                        "incubation temperature is not set",
                    )),
                }
                match &spec.duration {
                    Some(duration) => {
                        validate_quantity(duration, "operation.duration", UnitFamily::Time, &mut issues)
                    }
                    None => {
                        issues.push(missing("operation.duration", "incubation duration is not set"))
                    }
                }
            }
            Operation::Wash(spec) => {
                validate_positive_count(spec.cycles, "operation.cycles", &mut issues);
                match &spec.volume {
                    Some(volume) => validate_quantity(
                        volume,
                        "operation.volume",
                        UnitFamily::Volume,
                        &mut issues,
                    ),
                    None => issues.push(missing("operation.volume", "wash volume is not set")),
                }
            }
            Operation::Aliquot(spec) => {
                validate_positive_count(spec.parts, "operation.parts", &mut issues);
                match &spec.per_part_volume {
                    Some(volume) => validate_quantity(
                        volume,
                        "operation.per_part_volume",
                        UnitFamily::Volume,
                        &mut issues,
                    ),
                    None => issues.push(missing(
                        "operation.per_part_volume",
                        "per-part volume is not set",
                    )),
                }
            }
            Operation::Measure(spec) => {
                if spec.endpoint_id.trim().is_empty() {
                    issues.push(missing("operation.endpoint_id", "endpoint id is not set"));
                }
                match spec.unit {
                    Some(unit) => validate_unit(
                        unit,
                        "operation.unit",
                        UnitFamily::Measurement,
                        &mut issues,
                    ),
                    None => issues.push(missing("operation.unit", "measurement unit is not set")),
                }
            }
        }

        for (index, check) in self.checks.iter().enumerate() {
            if check.check_id.trim().is_empty() {
                issues.push(missing(
                    format!("checks[{index}].check_id"),
                    "check id is not set",
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

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum SpecErrorCode {
    UnitMismatch,
    MissingParameter,
    ValueOutOfRange,
}

impl SpecErrorCode {
    pub fn as_str(self) -> &'static str {
        match self {
            SpecErrorCode::UnitMismatch => "unit_mismatch",
            SpecErrorCode::MissingParameter => "missing_parameter",
            SpecErrorCode::ValueOutOfRange => "value_out_of_range",
        }
    }
}

/// One semantic rejection. `path` is stable and always starts at `operation.`
/// or `checks[`, so downstream diagnostics can anchor a field.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct SpecIssue {
    pub code: SpecErrorCode,
    pub path: String,
    pub message: String,
}

fn validate_quantity(
    quantity: &Quantity,
    path: &str,
    expected: UnitFamily,
    issues: &mut Vec<SpecIssue>,
) {
    if quantity.unit.family() != expected {
        validate_unit(quantity.unit, path, expected, issues);
        return;
    }
    if !quantity.value.is_finite() {
        issues.push(SpecIssue {
            code: SpecErrorCode::ValueOutOfRange,
            path: path.into(),
            message: "value must be a finite number".into(),
        });
        return;
    }
    if expected == UnitFamily::Temperature {
        if quantity.value <= ABSOLUTE_ZERO_CELSIUS {
            issues.push(SpecIssue {
                code: SpecErrorCode::ValueOutOfRange,
                path: path.into(),
                message: format!(
                    "temperature must be above absolute zero ({ABSOLUTE_ZERO_CELSIUS} C)"
                ),
            });
        }
        return;
    }
    if quantity.value <= 0.0 {
        issues.push(SpecIssue {
            code: SpecErrorCode::ValueOutOfRange,
            path: path.into(),
            message: "value must be greater than zero".into(),
        });
    }
}

fn validate_unit(unit: Unit, path: &str, expected: UnitFamily, issues: &mut Vec<SpecIssue>) {
    if unit.family() == expected {
        return;
    }
    issues.push(SpecIssue {
        code: SpecErrorCode::UnitMismatch,
        path: path.into(),
        message: format!(
            "unit {} is {}, expected {}",
            unit.as_str(),
            unit.family().as_str(),
            expected.as_str()
        ),
    });
}

fn validate_container_id(value: &str, path: &str, issues: &mut Vec<SpecIssue>) {
    if value.trim().is_empty() {
        issues.push(missing(path, "container id is not set"));
    }
}

fn validate_positive_count(value: Option<u32>, path: &str, issues: &mut Vec<SpecIssue>) {
    match value {
        None => issues.push(missing(path, "count is not set")),
        Some(0) => issues.push(SpecIssue {
            code: SpecErrorCode::ValueOutOfRange,
            path: path.into(),
            message: "count must be at least 1".into(),
        }),
        Some(_) => {}
    }
}

fn missing(path: impl Into<String>, message: impl Into<String>) -> SpecIssue {
    SpecIssue {
        code: SpecErrorCode::MissingParameter,
        path: path.into(),
        message: message.into(),
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use serde_json::json;

    fn spec_from(value: serde_json::Value) -> OperationSpec {
        serde_json::from_value(value).expect("spec parses")
    }

    fn codes(spec: &OperationSpec) -> Vec<&'static str> {
        spec.validate()
            .expect_err("spec is rejected")
            .iter()
            .map(|issue| issue.code.as_str())
            .collect()
    }

    #[test]
    fn transfer_roundtrips_volume_and_containers() {
        let spec = OperationSpec::v1(Operation::Transfer(TransferSpec {
            volume: Some(Quantity {
                value: 100.0,
                unit: Unit::Microliter,
            }),
            source: ContainerRef {
                container_id: "tube-1".into(),
            },
            destination: ContainerRef {
                container_id: "plate-1".into(),
            },
        }));
        assert_eq!(spec.operation.kind(), OperationKind::Transfer);
        assert!(spec.validate().is_ok());

        let json = serde_json::to_value(&spec).unwrap();
        assert_eq!(json["operation"]["kind"], "transfer");
        assert_eq!(json["operation"]["volume"]["unit"], "uL");
        let back: OperationSpec = serde_json::from_value(json).unwrap();
        assert_eq!(back, spec);
    }

    #[test]
    fn centrifuge_accepts_force_or_speed_and_requires_duration() {
        let rcf = spec_from(json!({
            "schema_version": 1,
            "operation": {
                "kind": "centrifuge",
                "drive": { "kind": "rcf", "force": { "value": 500.0, "unit": "xg" } },
                "duration": { "value": 5.0, "unit": "min" }
            }
        }));
        assert!(rcf.validate().is_ok());

        let speed = spec_from(json!({
            "schema_version": 1,
            "operation": {
                "kind": "centrifuge",
                "drive": { "kind": "speed", "speed": { "value": 12000.0, "unit": "rpm" } },
                "duration": { "value": 2.0, "unit": "min" }
            }
        }));
        assert!(speed.validate().is_ok());

        let incomplete = spec_from(json!({
            "schema_version": 1,
            "operation": { "kind": "centrifuge" }
        }));
        assert_eq!(codes(&incomplete), ["missing_parameter", "missing_parameter"]);
    }

    #[test]
    fn incubate_requires_temperature_and_duration() {
        let spec = spec_from(json!({
            "schema_version": 1,
            "operation": {
                "kind": "incubate",
                "temperature": { "value": 37.0, "unit": "C" },
                "duration": { "value": 2.0, "unit": "h" }
            }
        }));
        assert!(spec.validate().is_ok());

        let missing = spec_from(json!({
            "schema_version": 1,
            "operation": {
                "kind": "incubate",
                "duration": { "value": 2.0, "unit": "h" }
            }
        }));
        assert_eq!(missing.validate().unwrap_err()[0].path, "operation.temperature");
    }

    #[test]
    fn wash_requires_cycles_and_volume() {
        let spec = spec_from(json!({
            "schema_version": 1,
            "operation": {
                "kind": "wash",
                "cycles": 3,
                "volume": { "value": 200.0, "unit": "uL" }
            }
        }));
        assert!(spec.validate().is_ok());

        let no_cycles = spec_from(json!({
            "schema_version": 1,
            "operation": {
                "kind": "wash",
                "cycles": null,
                "volume": { "value": 200.0, "unit": "uL" }
            }
        }));
        assert_eq!(codes(&no_cycles), ["missing_parameter"]);
        assert_eq!(no_cycles.validate().unwrap_err()[0].path, "operation.cycles");
    }

    #[test]
    fn aliquot_requires_parts_and_per_part_volume() {
        let spec = spec_from(json!({
            "schema_version": 1,
            "operation": {
                "kind": "aliquot",
                "parts": 4,
                "per_part_volume": { "value": 50.0, "unit": "uL" }
            }
        }));
        assert!(spec.validate().is_ok());

        let missing = spec_from(json!({
            "schema_version": 1,
            "operation": { "kind": "aliquot", "parts": 4 }
        }));
        assert_eq!(missing.validate().unwrap_err()[0].path, "operation.per_part_volume");
    }

    #[test]
    fn measure_requires_endpoint_id_and_measurement_unit() {
        let spec = spec_from(json!({
            "schema_version": 1,
            "operation": { "kind": "measure", "endpoint_id": "endpoint-cck8-od", "unit": "AU" }
        }));
        assert!(spec.validate().is_ok());

        let blank_endpoint = spec_from(json!({
            "schema_version": 1,
            "operation": { "kind": "measure", "endpoint_id": "  ", "unit": "AU" }
        }));
        assert_eq!(
            blank_endpoint.validate().unwrap_err()[0].path,
            "operation.endpoint_id"
        );

        let wrong_family = spec_from(json!({
            "schema_version": 1,
            "operation": { "kind": "measure", "endpoint_id": "endpoint-cck8-od", "unit": "uL" }
        }));
        assert_eq!(codes(&wrong_family), ["unit_mismatch"]);
    }

    #[test]
    fn unit_mismatch_is_semantic_not_a_parse_error() {
        let spec = spec_from(json!({
            "schema_version": 1,
            "operation": {
                "kind": "transfer",
                "volume": { "value": 100.0, "unit": "min" },
                "source": { "container_id": "tube-1" },
                "destination": { "container_id": "plate-1" }
            }
        }));
        let issues = spec.validate().unwrap_err();
        assert_eq!(issues[0].code, SpecErrorCode::UnitMismatch);
        assert_eq!(issues[0].message, "unit min is time, expected volume");

        // The token is a valid unit, so serde accepted it; only the family check rejects.
        assert!(serde_json::from_str::<Quantity>(r#"{"value":1.0,"unit":"furlong"}"#).is_err());
    }

    #[test]
    fn centrifuge_rejects_rcf_declared_in_rpm() {
        let spec = spec_from(json!({
            "schema_version": 1,
            "operation": {
                "kind": "centrifuge",
                "drive": { "kind": "rcf", "force": { "value": 500.0, "unit": "rpm" } },
                "duration": { "value": 5.0, "unit": "min" }
            }
        }));
        assert_eq!(codes(&spec), ["unit_mismatch"]);
    }

    #[test]
    fn out_of_range_values_are_rejected() {
        let zero_volume = spec_from(json!({
            "schema_version": 1,
            "operation": {
                "kind": "transfer",
                "volume": { "value": 0.0, "unit": "uL" },
                "source": { "container_id": "tube-1" },
                "destination": { "container_id": "plate-1" }
            }
        }));
        assert_eq!(codes(&zero_volume), ["value_out_of_range"]);

        let cold = spec_from(json!({
            "schema_version": 1,
            "operation": {
                "kind": "incubate",
                "temperature": { "value": -300.0, "unit": "C" },
                "duration": { "value": 1.0, "unit": "h" }
            }
        }));
        assert_eq!(codes(&cold), ["value_out_of_range"]);

        let zero_parts = spec_from(json!({
            "schema_version": 1,
            "operation": {
                "kind": "aliquot",
                "parts": 0,
                "per_part_volume": { "value": 50.0, "unit": "uL" }
            }
        }));
        assert_eq!(codes(&zero_parts), ["value_out_of_range"]);

        let zero_duration = spec_from(json!({
            "schema_version": 1,
            "operation": {
                "kind": "incubate",
                "temperature": { "value": 37.0, "unit": "C" },
                "duration": { "value": 0.0, "unit": "min" }
            }
        }));
        assert_eq!(codes(&zero_duration), ["value_out_of_range"]);
    }

    #[test]
    fn blank_container_ids_are_missing_parameters() {
        let spec = spec_from(json!({
            "schema_version": 1,
            "operation": {
                "kind": "transfer",
                "volume": { "value": 100.0, "unit": "uL" },
                "source": { "container_id": "" },
                "destination": { "container_id": "plate-1" }
            }
        }));
        let issues = spec.validate().unwrap_err();
        assert_eq!(issues.len(), 1);
        assert_eq!(issues[0].code, SpecErrorCode::MissingParameter);
        assert_eq!(issues[0].path, "operation.source.container_id");
    }

    #[test]
    fn blank_check_ids_are_missing_parameters() {
        let mut spec = OperationSpec::v1(Operation::Measure(MeasureSpec {
            endpoint_id: "endpoint-cck8-od".into(),
            unit: Some(Unit::AbsorbanceUnit),
        }));
        spec.checks.push(StepCheck {
            check_id: " ".into(),
            label: "no bubbles".into(),
            instruction: None,
            blocking: true,
        });
        let issues = spec.validate().unwrap_err();
        assert_eq!(issues[0].path, "checks[0].check_id");
        assert_eq!(issues[0].code, SpecErrorCode::MissingParameter);
    }

    #[test]
    fn unsupported_schema_version_is_a_load_concern() {
        let mut spec = OperationSpec::v1(Operation::Measure(MeasureSpec {
            endpoint_id: "endpoint-cck8-od".into(),
            unit: Some(Unit::AbsorbanceUnit),
        }));
        assert!(spec.is_supported_schema_version());
        spec.schema_version = 2;
        assert!(!spec.is_supported_schema_version());
        // Version refusal never leaks into the closed semantic code set.
        assert!(spec.validate().is_ok());
    }

    #[test]
    fn unknown_fields_are_rejected() {
        let typo = json!({
            "schema_version": 1,
            "operation": {
                "kind": "transfer",
                "volume": { "value": 100.0, "unit": "uL" },
                "source": { "container_id": "tube-1" },
                "destination": { "container_id": "plate-1" },
                "volumes": { "value": 1.0, "unit": "uL" }
            }
        });
        assert!(serde_json::from_value::<OperationSpec>(typo).is_err());

        let unknown_kind = json!({
            "schema_version": 1,
            "operation": { "kind": "centrifugate" }
        });
        assert!(serde_json::from_value::<OperationSpec>(unknown_kind).is_err());
    }

    #[test]
    fn documented_error_codes_match_the_closed_set() {
        let doc = include_str!("../../../docs/contracts/operation-spec-v1.md");
        for code in OPERATION_SPEC_ERROR_CODES {
            assert!(
                doc.contains(&format!("`{}`", code.as_str())),
                "contract doc must list {}",
                code.as_str()
            );
        }
    }

    #[test]
    fn documented_examples_validate_as_documented() {
        let doc = include_str!("../../../docs/contracts/operation-spec-v1.md");

        // Every spec-shaped block in the contract parses.
        for block in json_blocks(doc).iter().filter(|b| b.contains("\"schema_version\"")) {
            serde_json::from_str::<OperationSpec>(block)
                .unwrap_or_else(|error| panic!("contract block does not parse: {error}\n{block}"));
        }

        let accepted = section(doc, "## 6.", "## 7.");
        let accepted_blocks = json_blocks(accepted);
        assert_eq!(accepted_blocks.len(), 2, "contract needs two complete examples");
        for block in accepted_blocks {
            let spec: OperationSpec = serde_json::from_str(block).expect("example parses");
            assert!(
                spec.validate().is_ok(),
                "complete example must validate: {:?}\n{block}",
                spec.validate()
            );
        }

        let rejected = section(doc, "## 7.", "## 8.");
        let mut seen = 0;
        for (heading, body) in subsections(rejected) {
            let code = backticked(heading).expect("rejection heading names its error code");
            let block = json_blocks(body)
                .into_iter()
                .next()
                .expect("rejection example has a json block");
            let spec: OperationSpec = serde_json::from_str(block).expect("rejection example parses");
            let issues = spec
                .validate()
                .expect_err("rejection example must be rejected");
            assert_eq!(issues[0].code.as_str(), code, "heading: {heading}");
            seen += 1;
        }
        assert_eq!(seen, 3, "contract needs three rejection examples");
    }

    #[test]
    fn all_six_operations_roundtrip_through_json() {
        let specimens = [
            OperationSpec::v1(Operation::Transfer(TransferSpec {
                volume: Some(Quantity {
                    value: 100.0,
                    unit: Unit::Microliter,
                }),
                source: ContainerRef {
                    container_id: "tube-1".into(),
                },
                destination: ContainerRef {
                    container_id: "plate-1".into(),
                },
            })),
            OperationSpec::v1(Operation::Centrifuge(CentrifugeSpec {
                drive: Some(CentrifugeDrive::Speed(SpeedDrive {
                    speed: Quantity {
                        value: 12000.0,
                        unit: Unit::Rpm,
                    },
                })),
                duration: Some(Quantity {
                    value: 2.0,
                    unit: Unit::Minute,
                }),
            })),
            OperationSpec::v1(Operation::Incubate(IncubateSpec {
                temperature: Some(Quantity {
                    value: 37.0,
                    unit: Unit::Celsius,
                }),
                duration: Some(Quantity {
                    value: 24.0,
                    unit: Unit::Hour,
                }),
            })),
            OperationSpec::v1(Operation::Wash(WashSpec {
                cycles: Some(3),
                volume: Some(Quantity {
                    value: 200.0,
                    unit: Unit::Microliter,
                }),
            })),
            OperationSpec::v1(Operation::Aliquot(AliquotSpec {
                parts: Some(4),
                per_part_volume: Some(Quantity {
                    value: 50.0,
                    unit: Unit::Microliter,
                }),
            })),
            OperationSpec::v1(Operation::Measure(MeasureSpec {
                endpoint_id: "endpoint-cck8-od450".into(),
                unit: Some(Unit::AbsorbanceUnit),
            })),
        ];
        assert_eq!(specimens.len(), 6);
        for spec in specimens {
            assert!(spec.validate().is_ok(), "{spec:?}");
            let json = serde_json::to_value(&spec).expect("spec serializes");
            let back: OperationSpec = serde_json::from_value(json).expect("spec parses");
            assert_eq!(back, spec);
        }
    }

    #[test]
    fn non_finite_values_are_out_of_range() {
        for value in [f64::NAN, f64::INFINITY] {
            let spec = OperationSpec::v1(Operation::Incubate(IncubateSpec {
                temperature: Some(Quantity {
                    value,
                    unit: Unit::Celsius,
                }),
                duration: Some(Quantity {
                    value: 1.0,
                    unit: Unit::Hour,
                }),
            }));
            let issues = spec.validate().unwrap_err();
            assert_eq!(issues[0].code, SpecErrorCode::ValueOutOfRange);
            assert_eq!(issues[0].path, "operation.temperature");
            assert_eq!(issues[0].message, "value must be a finite number");
        }
    }

    #[test]
    fn dry_run_example_matches_validation_output() {
        let doc = include_str!("../../../docs/contracts/operation-spec-v1.md");
        let dry_run = section(doc, "## 9.", "## 10.");
        let blocks = json_blocks(dry_run);
        assert_eq!(blocks.len(), 2, "dry-run section has one input and one output");

        let spec: OperationSpec = serde_json::from_str(blocks[0]).expect("dry-run input parses");
        let issues = spec.validate().expect_err("draft wash is incomplete");
        let actual = serde_json::to_value(&issues).unwrap();
        let expected: serde_json::Value =
            serde_json::from_str(blocks[1]).expect("dry-run output is json");
        assert_eq!(actual, expected);
    }

    fn section<'a>(doc: &'a str, start: &str, end: &str) -> &'a str {
        let mut begin = None;
        let mut cursor = 0;
        for line in doc.split_inclusive('\n') {
            let trimmed = line.trim_end();
            if begin.is_none() {
                if trimmed.starts_with(start) {
                    begin = Some(cursor);
                }
            } else if trimmed.starts_with(end) {
                return &doc[begin.unwrap()..cursor];
            }
            cursor += line.len();
        }
        begin.map(|begin| &doc[begin..]).unwrap_or("")
    }

    fn subsections(section: &str) -> Vec<(&str, &str)> {
        let mut out = Vec::new();
        let mut current: Option<(&str, usize)> = None;
        let mut cursor = 0;
        for line in section.split_inclusive('\n') {
            let trimmed = line.trim_end();
            if let Some(heading) = trimmed.strip_prefix("### ") {
                if let Some((previous, start)) = current {
                    out.push((previous, &section[start..cursor]));
                }
                current = Some((heading, cursor + line.len()));
            }
            cursor += line.len();
        }
        if let Some((heading, start)) = current {
            out.push((heading, &section[start..]));
        }
        out
    }

    fn json_blocks(text: &str) -> Vec<&str> {
        let mut blocks = Vec::new();
        let mut rest = text;
        while let Some(open) = rest.find("```json") {
            let body = &rest[open + "```json".len()..];
            let body = body.strip_prefix('\n').unwrap_or(body);
            match body.find("\n```") {
                Some(close) => {
                    blocks.push(body[..close].trim());
                    rest = &body[close + 4..];
                }
                None => break,
            }
        }
        blocks
    }

    fn backticked(text: &str) -> Option<&str> {
        let start = text.find('`')?;
        let rest = &text[start + 1..];
        let end = rest.find('`')?;
        Some(&rest[..end])
    }
}
