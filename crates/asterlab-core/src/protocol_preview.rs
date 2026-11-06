//! Typed projection of a recursively expanded Protocol preview.
//!
//! This module does not load blocks or validate parameter bindings; the CLI
//! expansion produces these types. Keeping the shape here prevents adapters
//! from inventing their own step/parameter JSON.

use serde::{Deserialize, Serialize};
use serde_json::Value;

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct ProtocolPreview {
    pub block_version_id: String,
    pub block_id: String,
    pub title: String,
    pub version: i64,
    pub ready: bool,
    pub steps: Vec<OperationStep>,
    pub resolved_parameters: Vec<ResolvedParameter>,
    pub unresolved_parameters: Vec<UnresolvedParameter>,
    pub block_version_ids: Vec<String>,
    #[serde(default)]
    pub module_contracts: Vec<Value>,
}

#[derive(Debug, Clone, Copy, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "snake_case")]
pub enum StepTimeScope {
    Once,
    PerTimepoint,
    PerAssay,
    Unresolved,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct OperationStep {
    pub path: String,
    pub use_id: String,
    pub title: String,
    pub instruction: String,
    pub parameters: Vec<ResolvedParameter>,
    pub unresolved_parameter_refs: Vec<String>,
    pub source_block_version_id: String,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub visualization: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub time_scope: Option<StepTimeScope>,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub checks: Vec<StepCheck>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[serde(deny_unknown_fields)]
pub struct StepCheck {
    pub check_id: String,
    pub label: String,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub instruction: Option<String>,
    #[serde(default)]
    pub blocking: bool,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct ResolvedParameter {
    /// Present for global resolved parameters; omitted for step parameters.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub path: Option<String>,
    pub slot: String,
    pub label: String,
    pub value: Value,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub unit: Option<String>,
    pub provenance: Vec<Value>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct UnresolvedParameter {
    pub path: String,
    pub slot: String,
    pub label: String,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub unit: Option<String>,
    pub required: bool,
}

#[cfg(test)]
mod tests {
    use super::*;
    use serde_json::json;

    #[test]
    fn step_parameter_omits_path_but_global_keeps_it() {
        let step_param = ResolvedParameter {
            path: None,
            slot: "wavelength".into(),
            label: "波长".into(),
            value: json!(450),
            unit: Some("nm".into()),
            provenance: vec![],
        };
        let global = ResolvedParameter {
            path: Some("root/read_od/wavelength".into()),
            ..step_param.clone()
        };
        let step_json = serde_json::to_value(&step_param).unwrap();
        let global_json = serde_json::to_value(&global).unwrap();
        assert!(!step_json.as_object().unwrap().contains_key("path"));
        assert_eq!(global_json["path"], "root/read_od/wavelength");
    }

    #[test]
    fn visualization_is_omitted_when_absent() {
        let step = OperationStep {
            path: "root/a".into(),
            use_id: "a".into(),
            title: "A".into(),
            instruction: "do".into(),
            parameters: vec![],
            unresolved_parameter_refs: vec![],
            source_block_version_id: "v".into(),
            visualization: None,
            time_scope: None,
            checks: vec![],
        };
        assert!(!serde_json::to_value(step)
            .unwrap()
            .as_object()
            .unwrap()
            .contains_key("visualization"));
    }
}
