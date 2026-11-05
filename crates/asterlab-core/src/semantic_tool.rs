//! S1-C1 pure ToolAction contract.
//!
//! A pure ToolAction is deterministic calculation/preview. It is not a
//! ProtocolStep, does not write the database, and does not drive instruments.
//! This module defines the contract and a runner that rejects impure and
//! non-deterministic tools. It is not wired into CLI or UI.

use crate::semantic_component::ProvenanceRef;
use serde::{Deserialize, Serialize};
use serde_json::Value;

pub const SEMANTIC_TOOL_SCHEMA_VERSION: u32 = 1;

#[derive(Debug, Clone, Copy, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "snake_case")]
pub enum ToolSideEffects {
    Pure,
    DatabaseWrite,
    ExternalEffect,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[serde(deny_unknown_fields)]
pub struct ToolActionSpec {
    pub action_id: String,
    pub version: String,
    pub input_schema: Value,
    pub output_schema: Value,
    pub side_effects: ToolSideEffects,
    pub implementation_source: String,
    pub requires_confirmation: bool,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub struct ToolError {
    pub code: String,
    pub message: String,
}

impl ToolError {
    pub fn new(code: impl Into<String>, message: impl Into<String>) -> Self {
        Self {
            code: code.into(),
            message: message.into(),
        }
    }
}

impl ToolActionSpec {
    pub fn validate_pure(&self) -> Result<(), Vec<ToolError>> {
        let mut issues = Vec::new();
        if self.action_id.trim().is_empty() {
            issues.push(ToolError::new(
                "empty_action_id",
                "action id must not be empty",
            ));
        }
        if self.version.trim().is_empty() {
            issues.push(ToolError::new(
                "empty_version",
                "action version must not be empty",
            ));
        }
        if self.implementation_source.trim().is_empty() {
            issues.push(ToolError::new(
                "empty_source",
                "implementation source must not be empty",
            ));
        }
        if !self.input_schema.is_object() {
            issues.push(ToolError::new(
                "invalid_input_schema",
                "input schema must be a JSON object",
            ));
        }
        if !self.output_schema.is_object() {
            issues.push(ToolError::new(
                "invalid_output_schema",
                "output schema must be a JSON object",
            ));
        }
        if self.side_effects != ToolSideEffects::Pure {
            issues.push(ToolError::new(
                "not_pure",
                "pure ToolAction runner only accepts Pure side effects",
            ));
        }
        if issues.is_empty() {
            Ok(())
        } else {
            Err(issues)
        }
    }
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[serde(deny_unknown_fields)]
pub struct ToolInvocation {
    pub invocation_id: String,
    pub action_id: String,
    pub action_version: String,
    pub inputs: Value,
    pub outputs: Value,
    pub provenance: Vec<ProvenanceRef>,
}

pub trait PureTool {
    fn spec(&self) -> &ToolActionSpec;
    fn execute(&self, inputs: &Value) -> Result<Value, ToolError>;
}

/// Run a pure tool twice on the same inputs and reject differing outputs. This
/// is a small deterministic check, not a proof of purity; the spec still must be
/// declared Pure and the implementation must not write or act externally.
pub fn run_pure(
    tool: &dyn PureTool,
    invocation_id: impl Into<String>,
    inputs: Value,
) -> Result<ToolInvocation, Vec<ToolError>> {
    let invocation_id = invocation_id.into();
    if invocation_id.trim().is_empty() {
        return Err(vec![ToolError::new(
            "empty_invocation_id",
            "invocation id must not be empty",
        )]);
    }
    let spec = tool.spec();
    spec.validate_pure()?;
    if !inputs.is_object() {
        return Err(vec![ToolError::new(
            "invalid_inputs",
            "inputs must be a JSON object",
        )]);
    }

    let first = tool.execute(&inputs).map_err(|error| vec![error])?;
    let second = tool.execute(&inputs).map_err(|error| vec![error])?;
    if first != second {
        return Err(vec![ToolError::new(
            "non_deterministic",
            "the same inputs produced different outputs",
        )]);
    }

    Ok(ToolInvocation {
        invocation_id,
        action_id: spec.action_id.clone(),
        action_version: spec.version.clone(),
        inputs,
        outputs: first,
        provenance: vec![ProvenanceRef {
            kind: "tool_action".into(),
            id: format!("{}@{}", spec.action_id, spec.version),
            path: None,
        }],
    })
}

#[cfg(test)]
mod tests {
    use super::*;
    use serde_json::json;
    use std::cell::Cell;

    fn spec(side_effects: ToolSideEffects) -> ToolActionSpec {
        ToolActionSpec {
            action_id: "tool.volume".into(),
            version: "1".into(),
            input_schema: json!({"type": "object"}),
            output_schema: json!({"type": "object"}),
            side_effects,
            implementation_source: "core:test".into(),
            requires_confirmation: false,
        }
    }

    struct VolumeSum {
        spec: ToolActionSpec,
    }

    impl PureTool for VolumeSum {
        fn spec(&self) -> &ToolActionSpec {
            &self.spec
        }

        fn execute(&self, inputs: &Value) -> Result<Value, ToolError> {
            let a = inputs["a"].as_f64().unwrap_or(0.0);
            let b = inputs["b"].as_f64().unwrap_or(0.0);
            Ok(json!({"total": a + b}))
        }
    }

    struct Unstable {
        spec: ToolActionSpec,
        calls: Cell<u32>,
    }

    impl PureTool for Unstable {
        fn spec(&self) -> &ToolActionSpec {
            &self.spec
        }

        fn execute(&self, _inputs: &Value) -> Result<Value, ToolError> {
            let next = self.calls.get() + 1;
            self.calls.set(next);
            Ok(json!({"value": next}))
        }
    }

    #[test]
    fn pure_tool_runs_deterministically_and_keeps_provenance() {
        let tool = VolumeSum {
            spec: spec(ToolSideEffects::Pure),
        };
        let invocation = run_pure(&tool, "calc-1", json!({"a": 1.0, "b": 2.0})).unwrap();
        assert_eq!(invocation.outputs, json!({"total": 3.0}));
        assert_eq!(invocation.action_id, "tool.volume");
        assert_eq!(invocation.action_version, "1");
        assert_eq!(invocation.provenance[0].kind, "tool_action");
        assert_eq!(invocation.provenance[0].id, "tool.volume@1");
    }

    #[test]
    fn database_write_and_external_effect_are_rejected() {
        for side_effects in [
            ToolSideEffects::DatabaseWrite,
            ToolSideEffects::ExternalEffect,
        ] {
            let tool = VolumeSum {
                spec: spec(side_effects),
            };
            let errors = run_pure(&tool, "calc-1", json!({})).unwrap_err();
            assert!(errors.iter().any(|error| error.code == "not_pure"));
        }
    }

    #[test]
    fn non_deterministic_tool_is_rejected() {
        let tool = Unstable {
            spec: spec(ToolSideEffects::Pure),
            calls: Cell::new(0),
        };
        let errors = run_pure(&tool, "calc-1", json!({})).unwrap_err();
        assert!(errors.iter().any(|error| error.code == "non_deterministic"));
    }

    #[test]
    fn invalid_schema_and_inputs_are_rejected() {
        let mut invalid = spec(ToolSideEffects::Pure);
        invalid.input_schema = Value::Null;
        let tool = VolumeSum { spec: invalid };
        let errors = run_pure(&tool, "calc-1", json!({})).unwrap_err();
        assert!(errors
            .iter()
            .any(|error| error.code == "invalid_input_schema"));

        let tool = VolumeSum {
            spec: spec(ToolSideEffects::Pure),
        };
        let errors = run_pure(&tool, "calc-1", json!([])).unwrap_err();
        assert!(errors.iter().any(|error| error.code == "invalid_inputs"));
    }

    #[test]
    fn empty_invocation_id_is_rejected() {
        let tool = VolumeSum {
            spec: spec(ToolSideEffects::Pure),
        };
        let errors = run_pure(&tool, "", json!({})).unwrap_err();
        assert!(errors
            .iter()
            .any(|error| error.code == "empty_invocation_id"));
    }
}
