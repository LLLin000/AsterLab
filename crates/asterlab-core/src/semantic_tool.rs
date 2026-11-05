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

#[derive(Debug, Clone, Copy, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "snake_case")]
pub enum ToolTrust {
    Trusted,
    Unregistered,
    Revoked,
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
    pub trust: ToolTrust,
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
        if self.trust != ToolTrust::Trusted {
            issues.push(ToolError::new(
                "untrusted_implementation",
                "pure ToolAction runner only accepts a trusted registry implementation",
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

/// Run one trusted pure computation once. Purity is enforced by the registry
/// trust flag and the declared side effect; a Rust trait cannot prove isolation.
/// Tests may repeat execution separately to check determinism.
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
    let mut issues = validate_schema_value(&spec.input_schema, &inputs, "inputs");
    if !issues.is_empty() {
        issues.insert(
            0,
            ToolError::new("invalid_inputs", "inputs do not satisfy input schema"),
        );
        return Err(issues);
    }

    let outputs = tool.execute(&inputs).map_err(|error| vec![error])?;
    let mut issues = validate_schema_value(&spec.output_schema, &outputs, "outputs");
    if !issues.is_empty() {
        issues.insert(
            0,
            ToolError::new("invalid_outputs", "outputs do not satisfy output schema"),
        );
        return Err(issues);
    }

    Ok(ToolInvocation {
        invocation_id,
        action_id: spec.action_id.clone(),
        action_version: spec.version.clone(),
        inputs,
        outputs,
        provenance: vec![ProvenanceRef {
            kind: "tool_action".into(),
            id: format!("{}@{}", spec.action_id, spec.version),
            path: None,
        }],
    })
}

/// Minimal JSON-Schema subset used by pure tools: object/array/string/number/
/// integer/boolean/null, required, properties, additionalProperties, items,
/// enum, const, minimum, maximum, minLength and maxLength. Schemas outside this
/// subset are not treated as proof of a valid input.
pub fn validate_schema_value(schema: &Value, value: &Value, path: &str) -> Vec<ToolError> {
    let mut issues = Vec::new();
    validate_schema_inner(schema, value, path, &mut issues);
    issues
}

fn validate_schema_inner(schema: &Value, value: &Value, path: &str, issues: &mut Vec<ToolError>) {
    let Some(schema_object) = schema.as_object() else {
        issues.push(ToolError::new(
            "invalid_schema",
            format!("{path}: schema must be an object"),
        ));
        return;
    };

    if let Some(const_value) = schema_object.get("const") {
        if value != const_value {
            issues.push(ToolError::new(
                "const",
                format!("{path}: value does not match const"),
            ));
        }
    }
    if let Some(values) = schema_object.get("enum").and_then(Value::as_array) {
        if !values.contains(value) {
            issues.push(ToolError::new(
                "enum",
                format!("{path}: value is not in enum"),
            ));
        }
    }
    if let Some(type_spec) = schema_object.get("type") {
        if !type_matches(type_spec, value) {
            issues.push(ToolError::new(
                "type",
                format!("{path}: value does not match type"),
            ));
            return;
        }
    }

    if let Some(min) = schema_object.get("minimum").and_then(Value::as_f64) {
        if value.as_f64().is_some_and(|value| value < min) {
            issues.push(ToolError::new(
                "minimum",
                format!("{path}: value is below minimum"),
            ));
        }
    }
    if let Some(max) = schema_object.get("maximum").and_then(Value::as_f64) {
        if value.as_f64().is_some_and(|value| value > max) {
            issues.push(ToolError::new(
                "maximum",
                format!("{path}: value is above maximum"),
            ));
        }
    }
    if let Some(text) = value.as_str() {
        if let Some(min) = schema_object.get("minLength").and_then(Value::as_u64) {
            if text.chars().count() < min as usize {
                issues.push(ToolError::new(
                    "min_length",
                    format!("{path}: string is too short"),
                ));
            }
        }
        if let Some(max) = schema_object.get("maxLength").and_then(Value::as_u64) {
            if text.chars().count() > max as usize {
                issues.push(ToolError::new(
                    "max_length",
                    format!("{path}: string is too long"),
                ));
            }
        }
    }

    match value {
        Value::Object(object) => {
            if let Some(required) = schema_object.get("required").and_then(Value::as_array) {
                for key in required.iter().filter_map(Value::as_str) {
                    if !object.contains_key(key) {
                        issues.push(ToolError::new(
                            "required",
                            format!("{path}.{key}: required property is missing"),
                        ));
                    }
                }
            }
            if let Some(properties) = schema_object.get("properties").and_then(Value::as_object) {
                for (key, child_schema) in properties {
                    if let Some(child) = object.get(key) {
                        validate_schema_inner(
                            child_schema,
                            child,
                            &format!("{path}.{key}"),
                            issues,
                        );
                    }
                }
            }
            if let Some(additional) = schema_object.get("additionalProperties") {
                if additional == &Value::Bool(false) {
                    if let Some(properties) =
                        schema_object.get("properties").and_then(Value::as_object)
                    {
                        for key in object.keys() {
                            if !properties.contains_key(key) {
                                issues.push(ToolError::new(
                                    "additional_property",
                                    format!("{path}.{key}: additional property is not allowed"),
                                ));
                            }
                        }
                    }
                }
            }
        }
        Value::Array(array) => {
            if let Some(item_schema) = schema_object.get("items") {
                for (index, item) in array.iter().enumerate() {
                    validate_schema_inner(item_schema, item, &format!("{path}[{index}]"), issues);
                }
            }
        }
        _ => {}
    }
}

fn type_matches(type_spec: &Value, value: &Value) -> bool {
    let matches = |name: &str| match name {
        "object" => value.is_object(),
        "array" => value.is_array(),
        "string" => value.is_string(),
        "number" => value.is_number(),
        "integer" => value.as_i64().is_some() || value.as_u64().is_some(),
        "boolean" => value.is_boolean(),
        "null" => value.is_null(),
        _ => false,
    };
    if let Some(name) = type_spec.as_str() {
        return matches(name);
    }
    type_spec
        .as_array()
        .is_some_and(|types| types.iter().filter_map(Value::as_str).any(matches))
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
            trust: ToolTrust::Trusted,
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
    fn production_runner_executes_once() {
        let tool = Unstable {
            spec: spec(ToolSideEffects::Pure),
            calls: Cell::new(0),
        };
        let invocation = run_pure(&tool, "calc-1", json!({})).unwrap();
        assert_eq!(invocation.outputs, json!({"value": 1}));
        assert_eq!(tool.calls.get(), 1);
    }

    #[test]
    fn determinism_is_checked_in_tests_not_the_production_runner() {
        let tool = Unstable {
            spec: spec(ToolSideEffects::Pure),
            calls: Cell::new(0),
        };
        let first = tool.execute(&json!({})).unwrap();
        let second = tool.execute(&json!({})).unwrap();
        assert_ne!(first, second);
    }

    #[test]
    fn input_schema_rejects_missing_or_wrong_values() {
        let mut tool = VolumeSum {
            spec: spec(ToolSideEffects::Pure),
        };
        tool.spec.input_schema = json!({
            "type": "object",
            "required": ["a", "b"],
            "properties": {
                "a": {"type": "number"},
                "b": {"type": "number"}
            },
            "additionalProperties": false
        });
        let errors = run_pure(&tool, "calc-1", json!({"a": 1.0})).unwrap_err();
        assert!(errors.iter().any(|error| error.code == "required"));
        let errors = run_pure(&tool, "calc-1", json!({"a": "one", "b": 2.0})).unwrap_err();
        assert!(errors.iter().any(|error| error.code == "type"));
    }

    #[test]
    fn output_schema_rejects_wrong_shape() {
        let mut tool = VolumeSum {
            spec: spec(ToolSideEffects::Pure),
        };
        tool.spec.output_schema = json!({
            "type": "object",
            "required": ["total"],
            "properties": {"total": {"type": "string"}}
        });
        let errors = run_pure(&tool, "calc-1", json!({"a": 1.0, "b": 2.0})).unwrap_err();
        assert!(errors.iter().any(|error| error.code == "invalid_outputs"));
    }

    #[test]
    fn untrusted_implementation_is_rejected() {
        let mut tool = VolumeSum {
            spec: spec(ToolSideEffects::Pure),
        };
        tool.spec.trust = ToolTrust::Unregistered;
        let errors = run_pure(&tool, "calc-1", json!({})).unwrap_err();
        assert!(errors
            .iter()
            .any(|error| error.code == "untrusted_implementation"));
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
