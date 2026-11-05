//! Pure read-model projection for the current experiment design.
//!
//! This module intentionally knows nothing about SQLite, HTTP or Slint.
//! Adapters pass the stored design JSON and receive a typed projection.

use serde::{Deserialize, Serialize};
use serde_json::{json, Value};

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct DesignMapContent {
    pub title: String,
    pub objective: String,
    pub experimental_unit: Option<String>,
    pub groups: Vec<DesignGroup>,
    pub assays: Vec<DesignAssay>,
    pub sample_links: Vec<SampleLink>,
    pub planned_outputs: Vec<PlannedOutputSummary>,
    pub conflicts: Vec<ConflictSummary>,
    pub unresolved: Vec<ValidationIssueSummary>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct CurrentDesignMap {
    pub design_id: String,
    pub revision: i64,
    pub content: DesignMapContent,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct FrozenDesignView {
    pub design_id: String,
    pub design_version_id: String,
    pub design_revision: i64,
    pub content: DesignMapContent,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct DesignGroup {
    pub id: String,
    pub name: String,
    pub role: Option<String>,
    pub condition: Option<String>,
    pub independent_n: Option<u32>,
    pub technical_repeats: Option<u32>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct DesignAssay {
    pub id: String,
    pub title: String,
    pub endpoint: String,
    pub method: String,
    pub unit: String,
    pub group_ids: Vec<String>,
    pub group_names: Vec<String>,
    pub timepoints: Vec<String>,
    pub timepoint_timing: Value,
    pub time_anchor_event: Option<String>,
    pub timepoint_relation: Option<String>,
    pub protocol_version_id: Option<String>,
    pub controls: Vec<String>,
    pub container_type: Option<String>,
    pub container_format: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct SampleLink {
    pub assay_id: String,
    pub group_id: Option<String>,
    pub group_name: String,
    pub timepoint_relation: Option<String>,
    pub source_relation: Option<String>,
    pub source_assay_id: Option<String>,
    pub physical_slots_per_group: Option<u32>,
    pub physical_slots_scope: Option<String>,
    pub planned_output_id: Option<String>,
    pub evidence_id: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct PlannedOutputSummary {
    pub id: String,
    pub title: String,
    pub protocol_version_id: String,
    pub root_binding_count: usize,
    pub evidence_id: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct ConflictSummary {
    pub id: String,
    pub topic: String,
    pub affected_assay_ids: Vec<String>,
    pub affects_container_estimate: bool,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct ValidationIssueSummary {
    pub key: String,
    pub question: String,
    pub affects: Vec<String>,
    pub source: String,
}

#[derive(Debug, Clone, PartialEq)]
pub struct DesignMapError(pub String);

impl std::fmt::Display for DesignMapError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.write_str(&self.0)
    }
}

impl std::error::Error for DesignMapError {}

fn text(value: &Value) -> String {
    value.as_str().unwrap_or_default().to_owned()
}

fn opt_text(value: &Value) -> Option<String> {
    value
        .as_str()
        .map(str::trim)
        .filter(|value| !value.is_empty())
        .map(str::to_owned)
}

fn u32_value(value: &Value) -> Option<u32> {
    value.as_u64().and_then(|value| u32::try_from(value).ok())
}

fn string_list(value: &Value) -> Vec<String> {
    value
        .as_array()
        .map(|items| {
            items
                .iter()
                .filter_map(Value::as_str)
                .map(str::to_owned)
                .collect()
        })
        .unwrap_or_default()
}

/// Project the stored `design` record data into the current design map.
/// Missing values stay missing; no scientific defaults are added.
pub fn build_current(
    design_id: &str,
    revision: i64,
    data: &Value,
) -> Result<CurrentDesignMap, DesignMapError> {
    if revision < 1 {
        return Err(DesignMapError("设计版本号无效".to_owned()));
    }
    let mut unresolved = Vec::new();
    let experimental_unit = opt_text(&data["experimental_unit"]);
    if experimental_unit.is_none() {
        unresolved.push(ValidationIssueSummary {
            key: "experimental_unit".to_owned(),
            question: "独立实验单位和生物学重复数是什么？".to_owned(),
            affects: vec![
                "组别 n".to_owned(),
                "时间点比较".to_owned(),
                "统计".to_owned(),
                "Run 计划".to_owned(),
            ],
            source: "missing".to_owned(),
        });
    }

    let groups = data["group_specs"]
        .as_array()
        .map(|items| {
            items
                .iter()
                .enumerate()
                .map(|(index, group)| {
                    let name = text(&group["name"]);
                    let independent_n = u32_value(&group["independent_n"]);
                    if independent_n.is_none() {
                        unresolved.push(ValidationIssueSummary {
                            key: format!("groups[{}].independent_n", index),
                            question: format!("组别「{}」的独立实验单位数是多少？", name),
                            affects: vec![
                                "该组 n".to_owned(),
                                "统计".to_owned(),
                                "Run 计划".to_owned(),
                            ],
                            source: "missing".to_owned(),
                        });
                    }
                    DesignGroup {
                        id: text(&group["id"]),
                        name,
                        role: opt_text(&group["role"]),
                        condition: opt_text(&group["condition"]),
                        independent_n,
                        technical_repeats: u32_value(&group["technical_repeats"]),
                    }
                })
                .collect::<Vec<_>>()
        })
        .unwrap_or_default();

    let assays = data["assays"]
        .as_array()
        .map(|items| {
            items
                .iter()
                .map(|assay| {
                    let id = text(&assay["id"]);
                    let title = text(&assay["title"]);
                    let timepoints = string_list(&assay["timepoints"]);
                    let protocol_version_id = opt_text(&assay["protocol_version_id"]);
                    let timepoint_relation = opt_text(&assay["timepoint_relation"]);
                    let endpoint = text(&assay["endpoint"]);
                    let method = text(&assay["method"]);
                    if timepoints.is_empty() {
                        unresolved.push(ValidationIssueSummary {
                            key: format!("assays.{}.timepoints", id),
                            question: format!("检测「{}」在哪些时间点进行？", title),
                            affects: vec![
                                "检测安排".to_owned(),
                                "样本关系".to_owned(),
                                "执行日程".to_owned(),
                            ],
                            source: "missing".to_owned(),
                        });
                    }
                    if protocol_version_id.is_none() {
                        unresolved.push(ValidationIssueSummary {
                            key: format!("assays.{}.protocol_version_id", id),
                            question: format!("检测「{}」使用哪个已确认的 Protocol 版本？", title),
                            affects: vec![
                                "操作步骤".to_owned(),
                                "Run 冻结".to_owned(),
                                "执行记录".to_owned(),
                            ],
                            source: "missing".to_owned(),
                        });
                    }
                    if timepoint_relation.is_none()
                        || timepoint_relation.as_deref() == Some("unknown")
                    {
                        unresolved.push(ValidationIssueSummary {
                            key: format!("assays.{}.timepoint_relation", id),
                            question: format!("检测「{}」各时间点是否使用同一对象或独立样本？", title),
                            affects: vec![
                                "样本流向".to_owned(),
                                "板图".to_owned(),
                                "独立 n".to_owned(),
                                "统计".to_owned(),
                            ],
                            source: "missing".to_owned(),
                        });
                    }
                    if endpoint.trim().is_empty() && method.trim().is_empty() {
                        unresolved.push(ValidationIssueSummary {
                            key: format!("assays.{}.endpoint", id),
                            question: format!("检测「{}」测量什么、用什么方法？", title),
                            affects: vec![
                                "检测安排".to_owned(),
                                "Protocol 选择".to_owned(),
                            ],
                            source: "missing".to_owned(),
                        });
                    }
                    DesignAssay {
                        id,
                        title,
                        endpoint,
                        method,
                        unit: text(&assay["unit"]),
                        group_ids: string_list(&assay["group_ids"]),
                        group_names: string_list(&assay["group_names"]),
                        timepoints,
                        timepoint_timing: assay["timepoint_timing"].clone(),
                        time_anchor_event: opt_text(&assay["time_anchor_event"]),
                        timepoint_relation,
                        protocol_version_id,
                        controls: string_list(&assay["controls"]),
                        container_type: opt_text(&assay["container_type"]),
                        container_format: opt_text(&assay["container_format"]),
                    }
                })
                .collect::<Vec<_>>()
        })
        .unwrap_or_default();

    let sample_links = data["sample_allocations"]
        .as_array()
        .map(|items| {
            items
                .iter()
                .map(|link| SampleLink {
                    assay_id: text(&link["assay_id"]),
                    group_id: opt_text(&link["group_id"]),
                    group_name: text(&link["group_name"]),
                    timepoint_relation: opt_text(&link["timepoint_relation"]),
                    source_relation: opt_text(&link["source_relation"]),
                    source_assay_id: opt_text(&link["source_assay_id"]),
                    physical_slots_per_group: u32_value(&link["physical_slots_per_group"]),
                    physical_slots_scope: opt_text(&link["physical_slots_scope"]),
                    planned_output_id: opt_text(&link["planned_output_id"]),
                    evidence_id: opt_text(&link["evidence_id"]),
                })
                .collect::<Vec<_>>()
        })
        .unwrap_or_default();

    let planned_outputs = data["planned_outputs"]
        .as_array()
        .map(|items| {
            items
                .iter()
                .map(|output| PlannedOutputSummary {
                    id: text(&output["id"]),
                    title: text(&output["title"]),
                    protocol_version_id: text(&output["protocol_version_id"]),
                    root_binding_count: output["root_bindings"]
                        .as_array()
                        .map_or(0, Vec::len),
                    evidence_id: opt_text(&output["evidence_id"]),
                })
                .collect::<Vec<_>>()
        })
        .unwrap_or_default();

    let mut conflicts = Vec::new();
    if let Some(items) = data["evidence_conflicts"].as_array() {
        for conflict in items {
            if conflict["status"].as_str() == Some("resolved") {
                continue;
            }
            let id = text(&conflict["id"]);
            let topic = text(&conflict["topic"]);
            let affected_assay_ids = string_list(&conflict["affected_assay_ids"]);
            unresolved.push(ValidationIssueSummary {
                key: format!("conflicts.{}", id),
                question: topic.clone(),
                affects: affected_assay_ids.clone(),
                source: "evidence_conflict".to_owned(),
            });
            conflicts.push(ConflictSummary {
                id,
                topic,
                affected_assay_ids,
                affects_container_estimate: conflict["affects_container_estimate"]
                    .as_bool()
                    .unwrap_or(false),
            });
        }
    }

    unresolved.sort_by(|left, right| left.key.cmp(&right.key));

    Ok(CurrentDesignMap {
        design_id: design_id.to_owned(),
        revision,
        content: DesignMapContent {
            title: text(&data["title"]),
            objective: text(&data["objective"]),
            experimental_unit,
            groups,
            assays,
            sample_links,
            planned_outputs,
            conflicts,
            unresolved,
        },
    })
}

impl CurrentDesignMap {
    /// Compatibility shape used by the existing `design.map` CLI/HTTP API.
    pub fn to_flat_json(&self) -> Value {
        let c = &self.content;
        json!({
            "design_id": self.design_id,
            "revision": self.revision,
            "title": c.title,
            "objective": c.objective,
            "experimental_unit": c.experimental_unit,
            "groups": c.groups,
            "assays": c.assays,
            "sample_links": c.sample_links,
            "planned_outputs": c.planned_outputs,
            "conflicts": c.conflicts,
            "unresolved": c.unresolved
        })
    }
}

impl FrozenDesignView {
    pub fn from_version(
        design_id: &str,
        design_version_id: &str,
        design_revision: i64,
        data: &Value,
    ) -> Result<Self, DesignMapError> {
        let current = build_current(design_id, design_revision, data)?;
        Ok(Self {
            design_id: design_id.to_owned(),
            design_version_id: design_version_id.to_owned(),
            design_revision,
            content: current.content,
        })
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn sample() -> Value {
        json!({
            "title": "Map test",
            "objective": "compare",
            "experimental_unit": null,
            "group_specs": [
                {"id":"g1","name":"Control","role":"control","technical_repeats":6},
                {"id":"g2","name":"Treatment","role":"treatment","independent_n":3}
            ],
            "assays": [{
                "id":"cck8","title":"CCK-8","endpoint":"OD450","method":"CCK-8",
                "group_ids":["g1","g2"],"timepoints":["D1","D3"],
                "timepoint_relation":"same_physical_sample_repeated"
            }],
            "sample_allocations": [],
            "planned_outputs": [],
            "evidence_conflicts": []
        })
    }

    #[test]
    fn missing_values_stay_unresolved() {
        let map = build_current("d1", 1, &sample()).unwrap();
        assert_eq!(map.content.groups.len(), 2);
        assert_eq!(map.content.groups[0].independent_n, None);
        assert!(map.content.unresolved.iter().any(|x| x.key == "experimental_unit"));
        assert!(map.content.unresolved.iter().any(|x| x.key == "groups[0].independent_n"));
        assert!(map.content.unresolved.iter().any(|x| x.key == "assays.cck8.protocol_version_id"));
        assert!(!map.content.unresolved.iter().any(|x| x.key == "assays.cck8.timepoints"));
    }

    #[test]
    fn frozen_view_reuses_projection_without_fallback() {
        let map = build_current("d1", 3, &sample()).unwrap();
        let frozen = FrozenDesignView::from_version("d1", "v1", 3, &sample()).unwrap();
        assert_eq!(frozen.design_version_id, "v1");
        assert_eq!(frozen.content, map.content);
    }

    #[test]
    fn unresolved_conflict_is_surfaced() {
        let mut data = sample();
        data["evidence_conflicts"] = json!([{
            "id":"c1","topic":"count mismatch","status":"unresolved",
            "affected_assay_ids":["cck8"],"evidence_ids":["e1","e2"]
        }]);
        let map = build_current("d1", 1, &data).unwrap();
        assert!(map.content.unresolved.iter().any(|x| x.key == "conflicts.c1"));
        assert_eq!(map.content.conflicts.len(), 1);
    }

    #[test]
    fn flat_shape_is_stable() {
        let map = build_current("d1", 2, &sample()).unwrap();
        let flat = map.to_flat_json();
        assert_eq!(flat["design_id"], "d1");
        assert_eq!(flat["revision"], 2);
        assert!(flat["groups"].is_array());
        assert!(flat["unresolved"].is_array());
    }
}
