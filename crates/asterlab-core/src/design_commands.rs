//! Design command slice for creating and revising a versioned experiment design.
//!
//! This module defines the shared command intent, draft shape and deterministic
//! validation. Persistence is an adapter concern; CLI, HTTP and the desktop
//! application must call the same Application Service.

use serde::{Deserialize, Serialize};
use std::collections::HashSet;

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct DesignDraft {
    pub title: String,
    #[serde(default)]
    pub objective: String,
    #[serde(default)]
    pub experimental_unit: Option<String>,
    #[serde(default)]
    pub groups: Vec<DesignGroupDraft>,
    #[serde(default)]
    pub assays: Vec<DesignAssayDraft>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct DesignGroupDraft {
    pub id: String,
    pub name: String,
    #[serde(default)]
    pub role: Option<String>,
    #[serde(default)]
    pub condition: Option<String>,
    #[serde(default)]
    pub independent_n: Option<u32>,
    #[serde(default)]
    pub technical_repeats: Option<u32>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct DesignAssayDraft {
    pub id: String,
    pub title: String,
    #[serde(default)]
    pub endpoint: String,
    #[serde(default)]
    pub method: String,
    #[serde(default)]
    pub unit: String,
    #[serde(default)]
    pub group_ids: Vec<String>,
    #[serde(default)]
    pub timepoints: Vec<String>,
    #[serde(default)]
    pub timepoint_relation: Option<String>,
    #[serde(default)]
    pub protocol_version_id: Option<String>,
    #[serde(default)]
    pub container_type: Option<String>,
    #[serde(default)]
    pub container_format: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct CreateDesign {
    pub project_id: String,
    pub draft: DesignDraft,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct ReviseDesign {
    pub design_id: String,
    pub expected_revision: i64,
    pub draft: DesignDraft,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct DesignSaved {
    pub design_id: String,
    pub revision: i64,
    pub design_version_id: String,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct DesignCommandError {
    pub code: String,
    pub path: Option<String>,
    pub message: String,
}

impl DesignCommandError {
    fn invalid(path: impl Into<String>, message: impl Into<String>) -> Self {
        Self {
            code: "INVALID_DESIGN".to_owned(),
            path: Some(path.into()),
            message: message.into(),
        }
    }
}

pub trait DesignCommands {
    fn create_design(&self, command: CreateDesign) -> Result<DesignSaved, DesignCommandError>;
    fn revise_design(&self, command: ReviseDesign) -> Result<DesignSaved, DesignCommandError>;
}

/// Validate only structural/scientific-reference constraints. Missing n,
/// sample relations, doses or timepoints remain valid draft unknowns.
impl DesignDraft {
    /// Map the editor draft to the current CLI/HTTP design payload.
    /// Unknown fields are intentionally not invented.
    pub fn to_design_json(&self) -> serde_json::Value {
        let group_names = self
            .groups
            .iter()
            .map(|group| (group.id.as_str(), group.name.as_str()))
            .collect::<std::collections::HashMap<_, _>>();
        serde_json::json!({
            "title": self.title,
            "objective": self.objective,
            "experimental_unit": self.experimental_unit,
            "group_specs": self.groups.iter().map(|group| serde_json::json!({
                "id": group.id,
                "name": group.name,
                "role": group.role,
                "condition": group.condition,
                "independent_n": group.independent_n,
                "technical_repeats": group.technical_repeats
            })).collect::<Vec<_>>(),
            "assays": self.assays.iter().map(|assay| serde_json::json!({
                "id": assay.id,
                "title": assay.title,
                "endpoint": assay.endpoint,
                "method": assay.method,
                "unit": assay.unit,
                "group_ids": assay.group_ids,
                "group_names": assay.group_ids.iter().filter_map(|id| group_names.get(id.as_str()).copied()).collect::<Vec<_>>(),
                "timepoints": assay.timepoints,
                "timepoint_relation": assay.timepoint_relation,
                "protocol_version_id": assay.protocol_version_id,
                "container_type": assay.container_type,
                "container_format": assay.container_format
            })).collect::<Vec<_>>()
        })
    }

    pub fn from_design_json(data: &serde_json::Value) -> Self {
        let groups = data["group_specs"]
            .as_array()
            .map(|items| {
                items
                    .iter()
                    .map(|group| DesignGroupDraft {
                        id: group["id"].as_str().unwrap_or_default().to_owned(),
                        name: group["name"].as_str().unwrap_or_default().to_owned(),
                        role: group["role"].as_str().map(str::to_owned),
                        condition: group["condition"].as_str().map(str::to_owned),
                        independent_n: group["independent_n"]
                            .as_u64()
                            .and_then(|value| u32::try_from(value).ok()),
                        technical_repeats: group["technical_repeats"]
                            .as_u64()
                            .and_then(|value| u32::try_from(value).ok()),
                    })
                    .collect()
            })
            .unwrap_or_default();
        let assays = data["assays"]
            .as_array()
            .map(|items| {
                items
                    .iter()
                    .map(|assay| DesignAssayDraft {
                        id: assay["id"].as_str().unwrap_or_default().to_owned(),
                        title: assay["title"].as_str().unwrap_or_default().to_owned(),
                        endpoint: assay["endpoint"].as_str().unwrap_or_default().to_owned(),
                        method: assay["method"].as_str().unwrap_or_default().to_owned(),
                        unit: assay["unit"].as_str().unwrap_or_default().to_owned(),
                        group_ids: assay["group_ids"]
                            .as_array()
                            .map(|ids| ids.iter().filter_map(|id| id.as_str().map(str::to_owned)).collect())
                            .unwrap_or_default(),
                        timepoints: assay["timepoints"]
                            .as_array()
                            .map(|points| points.iter().filter_map(|point| point.as_str().map(str::to_owned)).collect())
                            .unwrap_or_default(),
                        timepoint_relation: assay["timepoint_relation"].as_str().map(str::to_owned),
                        protocol_version_id: assay["protocol_version_id"].as_str().map(str::to_owned),
                        container_type: assay["container_type"].as_str().map(str::to_owned),
                        container_format: assay["container_format"].as_str().map(str::to_owned),
                    })
                    .collect()
            })
            .unwrap_or_default();
        Self {
            title: data["title"].as_str().unwrap_or_default().to_owned(),
            objective: data["objective"].as_str().unwrap_or_default().to_owned(),
            experimental_unit: data["experimental_unit"].as_str().map(str::to_owned),
            groups,
            assays,
        }
    }
}

pub fn validate_draft(draft: &DesignDraft) -> Result<(), DesignCommandError> {
    if draft.title.trim().is_empty() {
        return Err(DesignCommandError::invalid("title", "设计名称不能为空"));
    }
    if draft.title.chars().count() > 160 {
        return Err(DesignCommandError::invalid("title", "设计名称不能超过 160 个字符"));
    }
    if draft.objective.chars().count() > 5000 {
        return Err(DesignCommandError::invalid("objective", "研究问题不能超过 5000 个字符"));
    }

    let mut group_ids = HashSet::new();
    for (index, group) in draft.groups.iter().enumerate() {
        if group.id.trim().is_empty() {
            return Err(DesignCommandError::invalid(
                format!("groups[{}].id", index),
                "组别必须有稳定 ID",
            ));
        }
        if !group_ids.insert(group.id.as_str()) {
            return Err(DesignCommandError::invalid(
                format!("groups[{}].id", index),
                "组别 ID 重复",
            ));
        }
        if group.name.trim().is_empty() {
            return Err(DesignCommandError::invalid(
                format!("groups[{}].name", index),
                "组别名称不能为空",
            ));
        }
    }

    let mut assay_ids = HashSet::new();
    for (index, assay) in draft.assays.iter().enumerate() {
        if assay.id.trim().is_empty() {
            return Err(DesignCommandError::invalid(
                format!("assays[{}].id", index),
                "检测必须有稳定 ID",
            ));
        }
        if !assay_ids.insert(assay.id.as_str()) {
            return Err(DesignCommandError::invalid(
                format!("assays[{}].id", index),
                "检测 ID 重复",
            ));
        }
        if assay.title.trim().is_empty() {
            return Err(DesignCommandError::invalid(
                format!("assays[{}].title", index),
                "检测名称不能为空",
            ));
        }
        for group_id in &assay.group_ids {
            if !group_ids.contains(group_id.as_str()) {
                return Err(DesignCommandError::invalid(
                    format!("assays[{}].group_ids", index),
                    format!("引用了未定义的组别 ID：{group_id}"),
                ));
            }
        }
        let mut timepoints = HashSet::new();
        for timepoint in &assay.timepoints {
            if timepoint.trim().is_empty() {
                return Err(DesignCommandError::invalid(
                    format!("assays[{}].timepoints", index),
                    "时间点不能为空",
                ));
            }
            if !timepoints.insert(timepoint.as_str()) {
                return Err(DesignCommandError::invalid(
                    format!("assays[{}].timepoints", index),
                    format!("时间点重复：{timepoint}"),
                ));
            }
        }
    }
    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;

    fn blank() -> DesignDraft {
        DesignDraft {
            title: "New design".into(),
            objective: "Compare conditions".into(),
            experimental_unit: None,
            groups: vec![],
            assays: vec![],
        }
    }

    #[test]
    fn empty_groups_and_assays_are_valid_draft_unknowns() {
        assert!(validate_draft(&blank()).is_ok());
    }

    #[test]
    fn draft_round_trips_through_design_json() {
        let mut draft = blank();
        draft.groups = vec![DesignGroupDraft {
            id: "g1".into(), name: "Control".into(), role: Some("control".into()),
            condition: None, independent_n: None, technical_repeats: Some(2),
        }];
        draft.assays = vec![DesignAssayDraft {
            id: "a1".into(), title: "CCK-8".into(), endpoint: "OD450".into(),
            method: "CCK-8".into(), unit: "absorbance".into(),
            group_ids: vec!["g1".into()], timepoints: vec!["D1".into()],
            timepoint_relation: None, protocol_version_id: None,
            container_type: None, container_format: None,
        }];
        let json = draft.to_design_json();
        assert_eq!(json["assays"][0]["group_names"][0], "Control");
        assert_eq!(DesignDraft::from_design_json(&json), draft);
    }

    #[test]
    fn duplicate_group_ids_are_rejected() {
        let mut draft = blank();
        draft.groups = vec![
            DesignGroupDraft { id: "g1".into(), name: "A".into(), role: None, condition: None, independent_n: None, technical_repeats: None },
            DesignGroupDraft { id: "g1".into(), name: "B".into(), role: None, condition: None, independent_n: None, technical_repeats: None },
        ];
        assert!(validate_draft(&draft).is_err());
    }

    #[test]
    fn assay_cannot_reference_unknown_group() {
        let mut draft = blank();
        draft.assays = vec![DesignAssayDraft {
            id: "a1".into(),
            title: "CCK-8".into(),
            endpoint: "OD450".into(),
            method: "CCK-8".into(),
            unit: "absorbance".into(),
            group_ids: vec!["missing".into()],
            timepoints: vec!["D1".into()],
            timepoint_relation: None,
            protocol_version_id: None,
            container_type: None,
            container_format: None,
        }];
        assert!(validate_draft(&draft).is_err());
    }

    #[test]
    fn duplicate_timepoints_are_rejected() {
        let mut draft = blank();
        draft.groups = vec![DesignGroupDraft { id: "g1".into(), name: "Control".into(), role: None, condition: None, independent_n: None, technical_repeats: None }];
        draft.assays = vec![DesignAssayDraft {
            id: "a1".into(), title: "CCK-8".into(), endpoint: "".into(), method: "".into(), unit: "".into(),
            group_ids: vec!["g1".into()], timepoints: vec!["D1".into(), "D1".into()],
            timepoint_relation: None, protocol_version_id: None, container_type: None, container_format: None,
        }];
        assert!(validate_draft(&draft).is_err());
    }
}
