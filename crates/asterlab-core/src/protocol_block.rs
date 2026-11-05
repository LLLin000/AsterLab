//! Formal ProtocolBlockVersion data shape.
//!
//! This is the persisted/local block shape used by the CLI. The optional
//! `module_contract` sidecar is additive: old blocks without it remain readable.

use crate::operation_module::{OperationModule, ParameterValueType};
use crate::protocol_preview::StepTimeScope;
use serde::{Deserialize, Serialize};
use serde_json::Value;

#[derive(Debug, Clone, Deserialize, Serialize, PartialEq)]
#[serde(deny_unknown_fields)]
pub struct ParameterSlot {
    pub key: String,
    pub label: String,
    pub value_type: ParameterValueType,
    #[serde(default)]
    pub unit: Option<String>,
    #[serde(default = "required_parameter")]
    pub required: bool,
}

fn required_parameter() -> bool {
    true
}

#[derive(Debug, Clone, Deserialize, Serialize, PartialEq)]
#[serde(deny_unknown_fields)]
pub struct StepBinding {
    pub slot: String,
    #[serde(default)]
    pub value: Option<Value>,
    #[serde(default)]
    pub source_slot: Option<String>,
    #[serde(default)]
    pub unit: Option<String>,
}

#[derive(Debug, Clone, Copy, Deserialize, Serialize, PartialEq, Eq)]
#[serde(rename_all = "snake_case")]
pub enum StepVisualization {
    Transfer,
    Incubation,
    PlateLayout,
    SampleSplit,
    Measurement,
}

#[derive(Debug, Clone, Deserialize, Serialize, PartialEq)]
#[serde(tag = "kind", rename_all = "snake_case", deny_unknown_fields)]
pub enum ProtocolStep {
    Text {
        use_id: String,
        title: String,
        instruction: String,
        #[serde(default)]
        parameter_refs: Vec<String>,
        #[serde(default, skip_serializing_if = "Option::is_none")]
        visualization: Option<StepVisualization>,
        #[serde(default, skip_serializing_if = "Option::is_none")]
        time_scope: Option<StepTimeScope>,
    },
    BlockUse {
        use_id: String,
        block_version_id: String,
        #[serde(default)]
        bindings: Vec<StepBinding>,
    },
}

#[derive(Debug, Clone, Deserialize, Serialize, PartialEq)]
#[serde(deny_unknown_fields)]
pub struct ProtocolBlockVersionData {
    pub block_id: String,
    pub version: i64,
    pub summary: String,
    pub parameter_slots: Vec<ParameterSlot>,
    pub steps: Vec<ProtocolStep>,
    /// Additive sidecar for compiled catalog modules. Old blocks omit it.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub module_contract: Option<OperationModule>,
}
