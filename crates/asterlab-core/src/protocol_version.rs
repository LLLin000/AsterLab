//! Immutable ProtocolVersion query projection shared by CLI and application adapters.

use crate::protocol_block::ParameterSlot;
use crate::protocol_preview::{OperationStep, ResolvedParameter, UnresolvedParameter};
use serde::{Deserialize, Serialize};
use serde_json::Value;

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct ProtocolVersionView {
    pub protocol_version_id: String,
    pub protocol_id: String,
    pub version: i64,
    pub root_block_version_id: Option<String>,
    pub root_bindings: Vec<Value>,
    pub parameter_slots: Vec<ParameterSlot>,
    pub steps: Vec<OperationStep>,
    pub resolved_parameters: Vec<ResolvedParameter>,
    pub unresolved_parameters: Vec<UnresolvedParameter>,
    pub block_version_ids: Vec<String>,
    pub frozen: bool,
}
