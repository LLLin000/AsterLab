//! Read-only data adapter for the Slint native UI spike.
//!
//! Loads an explicitly supplied SQLite file with `SQLITE_OPEN_READ_ONLY`:
//! a missing path fails instead of creating/migrating anything. Only
//! explicit `well` rows populate `Plate.wells`; empty positions stay absent.

use rusqlite::{Connection, OpenFlags};
use serde_json::Value;
use std::collections::HashMap;
use std::path::Path;

#[derive(Clone, Debug)]
pub struct ReadModel {
    pub projects: Vec<Project>,
    pub designs: Vec<Design>,
    pub runs: Vec<Run>,
    pub plates: Vec<Plate>,
}

#[derive(Clone, Debug)]
pub struct Project {
    pub id: String,
    pub name: String,
}

#[derive(Clone, Debug)]
pub struct Design {
    pub id: String,
    pub project_id: String,
    pub title: String,
    pub objective: Option<String>,
    pub experimental_unit: Option<String>,
    pub groups: Vec<Group>,
    pub assays: Vec<Assay>,
    pub sources: Vec<Source>,
    pub conflicts: Vec<Conflict>,
}

#[derive(Clone, Debug)]
pub struct Group {
    pub id: String,
    pub name: String,
    pub role: String,
    pub condition: Option<String>,
}

#[derive(Clone, Debug)]
pub struct Assay {
    pub id: String,
    pub title: String,
    pub endpoint: Option<String>,
    pub method: Option<String>,
    pub unit: Option<String>,
    pub timepoints: Vec<String>,
    pub group_ids: Vec<String>,
    pub group_names: Vec<String>,
    pub timepoint_relation: Option<String>,
    pub time_anchor_event: Option<String>,
    pub container_format: Option<String>,
    pub controls: Vec<String>,
    pub allocations: Vec<Allocation>,
    pub evidence_id: Option<String>,
}

/// One stored `sample_allocations` entry: an explicit group ↔ assay plan.
#[derive(Clone, Debug)]
pub struct Allocation {
    pub group_id: String,
    pub group_name: String,
    pub slots_per_group: Option<u32>,
    pub slots_scope: Option<String>,
}

#[derive(Clone, Debug)]
pub struct Source {
    pub id: String,
    pub source_document: Option<String>,
    pub source_locator: Option<String>,
    pub claim: Option<String>,
}

#[derive(Clone, Debug)]
pub struct Conflict {
    pub topic: String,
    pub status: Option<String>,
    pub evidence_ids: Vec<String>,
}

#[derive(Clone, Debug)]
pub struct Run {
    pub id: String,
    pub project_id: String,
    pub design_id: String,
    pub title: String,
    pub status: Option<String>,
}

#[derive(Clone, Debug)]
pub struct Plate {
    pub id: String,
    pub run_id: String,
    pub name: String,
    pub format: String,
    pub rows: u32,
    pub columns: u32,
    pub assay_id: Option<String>,
    pub timepoint: Option<String>,
    pub wells: Vec<Well>,
}

#[derive(Clone, Debug)]
pub struct Well {
    pub coordinate: String,
    pub group_label: Option<String>,
    pub sample_id: Option<String>,
    pub note: Option<String>,
}

/// Optional text: missing, null, blank, or the stored `unknown` marker all
/// stay `None` instead of inventing a value.
fn opt(value: &Value) -> Option<String> {
    match value {
        Value::String(s) => {
            let text = s.trim();
            if text.is_empty() || text.eq_ignore_ascii_case("unknown") {
                None
            } else {
                Some(text.to_owned())
            }
        }
        _ => None,
    }
}

fn str_list(value: &Value) -> Vec<String> {
    value
        .as_array()
        .map(|items| {
            items
                .iter()
                .filter_map(|item| item.as_str().map(str::to_owned))
                .collect()
        })
        .unwrap_or_default()
}

fn as_u32(value: &Value) -> Option<u32> {
    value.as_u64().and_then(|n| u32::try_from(n).ok())
}

fn parse_design(id: String, project_id: Option<String>, data: &Value) -> Design {
    let groups = data["group_specs"]
        .as_array()
        .map(|specs| {
            specs
                .iter()
                .filter_map(|spec| {
                    spec["name"].as_str().map(|name| Group {
                        id: opt(&spec["id"]).unwrap_or_default(),
                        name: name.to_owned(),
                        role: opt(&spec["role"]).unwrap_or_default(),
                        condition: opt(&spec["condition"]),
                    })
                })
                .collect()
        })
        .unwrap_or_default();

    // Per-assay slot/evidence detail lives on sample_allocations rows that
    // reference the assay; keep every explicit row so the canvas shows only
    // recorded group links and never invents one.
    let allocations: Vec<&Value> = data["sample_allocations"]
        .as_array()
        .map(|items| items.iter().collect())
        .unwrap_or_default();
    let allocation_field = |assay_id: &str, key: &str| -> Option<&Value> {
        allocations.iter().find_map(|allocation| {
            if allocation["assay_id"].as_str() == Some(assay_id) && !allocation[key].is_null() {
                Some(&allocation[key])
            } else {
                None
            }
        })
    };

    let assays = data["assays"]
        .as_array()
        .map(|items| {
            items
                .iter()
                .filter_map(|assay| {
                    let assay_id = assay["id"].as_str()?;
                    let title = assay["title"].as_str()?;
                    let assay_allocations = allocations
                        .iter()
                        .filter(|allocation| allocation["assay_id"].as_str() == Some(assay_id))
                        .map(|allocation| Allocation {
                            group_id: opt(&allocation["group_id"]).unwrap_or_default(),
                            group_name: opt(&allocation["group_name"]).unwrap_or_default(),
                            slots_per_group: as_u32(&allocation["physical_slots_per_group"]),
                            slots_scope: opt(&allocation["physical_slots_scope"]),
                        })
                        .collect();
                    Some(Assay {
                        id: assay_id.to_owned(),
                        title: title.to_owned(),
                        endpoint: opt(&assay["endpoint"]),
                        method: opt(&assay["method"]),
                        unit: opt(&assay["unit"]),
                        timepoints: str_list(&assay["timepoints"]),
                        group_ids: str_list(&assay["group_ids"]),
                        group_names: str_list(&assay["group_names"]),
                        timepoint_relation: opt(&assay["timepoint_relation"]),
                        time_anchor_event: opt(&assay["time_anchor_event"]),
                        container_format: opt(&assay["container_format"])
                            .or_else(|| opt(&assay["container_type"])),
                        controls: str_list(&assay["controls"]),
                        allocations: assay_allocations,
                        evidence_id: opt(&assay["evidence_id"])
                            .or_else(|| allocation_field(assay_id, "evidence_id").and_then(opt)),
                    })
                })
                .collect()
        })
        .unwrap_or_default();

    let sources = data["source_evidence"]
        .as_array()
        .map(|items| {
            items
                .iter()
                .filter_map(|evidence| {
                    evidence["id"].as_str().map(|id| Source {
                        id: id.to_owned(),
                        source_document: opt(&evidence["source_document"]),
                        source_locator: opt(&evidence["source_locator"]),
                        claim: opt(&evidence["claim"]),
                    })
                })
                .collect()
        })
        .unwrap_or_default();

    let conflicts = data["evidence_conflicts"]
        .as_array()
        .map(|items| {
            items
                .iter()
                .filter_map(|conflict| {
                    Some(Conflict {
                        topic: conflict["topic"].as_str().unwrap_or_default().to_owned(),
                        status: opt(&conflict["status"]),
                        evidence_ids: str_list(&conflict["evidence_ids"]),
                    })
                })
                .collect()
        })
        .unwrap_or_default();

    Design {
        id,
        project_id: project_id.unwrap_or_default(),
        title: data["title"].as_str().unwrap_or_default().to_owned(),
        objective: opt(&data["objective"]),
        experimental_unit: opt(&data["experimental_unit"]),
        groups,
        assays,
        sources,
        conflicts,
    }
}

pub fn load_read_model(db_path: &Path) -> Result<ReadModel, String> {
    let conn = Connection::open_with_flags(db_path, OpenFlags::SQLITE_OPEN_READ_ONLY)
        .map_err(|e| format!("打不开数据库（只读）：{}：{e}", db_path.display()))?;
    let mut stmt = conn
        .prepare(
            "SELECT id, kind, project_id, parent_id, data_json \
             FROM records ORDER BY id",
        )
        .map_err(|e| format!("查询 records 失败：{e}"))?;
    let rows: Vec<(String, String, Option<String>, Option<String>, String)> = stmt
        .query_map([], |row| {
            Ok((
                row.get(0)?,
                row.get(1)?,
                row.get(2)?,
                row.get(3)?,
                row.get(4)?,
            ))
        })
        .map_err(|e| format!("查询 records 失败：{e}"))?
        .collect::<Result<_, _>>()
        .map_err(|e| format!("读取 records 失败：{e}"))?;

    let mut model = ReadModel {
        projects: Vec::new(),
        designs: Vec::new(),
        runs: Vec::new(),
        plates: Vec::new(),
    };
    let mut plate_index: HashMap<String, usize> = HashMap::new();
    let mut pending_wells: Vec<(Option<String>, Well)> = Vec::new();

    for (id, kind, project_id, parent_id, raw) in rows {
        let data: Value = serde_json::from_str(&raw).unwrap_or(Value::Null);
        match kind.as_str() {
            "project" => model.projects.push(Project {
                id: id.clone(),
                name: data["name"].as_str().unwrap_or_default().to_owned(),
            }),
            "design" => model.designs.push(parse_design(id, project_id, &data)),
            "run" => model.runs.push(Run {
                id: id.clone(),
                project_id: project_id.unwrap_or_default(),
                design_id: parent_id.unwrap_or_default(),
                title: data["title"].as_str().unwrap_or_default().to_owned(),
                status: opt(&data["status"]),
            }),
            "plate" => {
                plate_index.insert(id.clone(), model.plates.len());
                model.plates.push(Plate {
                    id: id.clone(),
                    run_id: parent_id.unwrap_or_default(),
                    name: data["name"].as_str().unwrap_or_default().to_owned(),
                    format: data["format"].as_str().unwrap_or_default().to_owned(),
                    rows: as_u32(&data["rows"]).unwrap_or(0),
                    columns: as_u32(&data["columns"]).unwrap_or(0),
                    assay_id: opt(&data["assay_id"]),
                    timepoint: opt(&data["timepoint"]),
                    wells: Vec::new(),
                });
            }
            // well_id stays exactly as stored; no case rewrite.
            "well" => pending_wells.push((
                parent_id,
                Well {
                    coordinate: data["well_id"].as_str().unwrap_or_default().to_owned(),
                    group_label: opt(&data["group_label"]),
                    sample_id: opt(&data["sample_id"]),
                    note: opt(&data["note"]),
                },
            )),
            _ => {}
        }
    }

    // Only explicit well rows appear; plates without wells stay empty.
    for (parent_id, well) in pending_wells {
        if let Some(parent) = parent_id.as_deref().and_then(|id| plate_index.get(id)) {
            model.plates[*parent].wells.push(well);
        }
    }

    Ok(model)
}

#[cfg(test)]
mod tests {
    use super::*;
    use rusqlite::params;
    use serde_json::json;
    use std::sync::atomic::{AtomicU64, Ordering};

    fn tmp_path(tag: &str) -> std::path::PathBuf {
        static NEXT: AtomicU64 = AtomicU64::new(0);
        let n = NEXT.fetch_add(1, Ordering::SeqCst);
        let dir = Path::new(env!("CARGO_MANIFEST_DIR")).join("target/test-data");
        std::fs::create_dir_all(&dir).unwrap();
        dir.join(format!(
            "slint-spike-data-test-{}-{n}-{tag}.sqlite",
            std::process::id()
        ))
    }

    fn insert(
        conn: &Connection,
        id: &str,
        kind: &str,
        project: Option<&str>,
        parent: Option<&str>,
        data: &Value,
    ) {
        conn.execute(
            "INSERT INTO records(id, kind, project_id, parent_id, revision, data_json, created_at, updated_at) \
             VALUES(?1, ?2, ?3, ?4, 1, ?5, 't', 't')",
            params![id, kind, project, parent, data.to_string()],
        )
        .unwrap();
    }

    #[test]
    fn missing_db_path_does_not_create_file() {
        let path = tmp_path("missing");
        assert!(!path.exists());
        let result = load_read_model(&path);
        assert!(result.is_err());
        assert!(!path.exists());
    }

    #[test]
    fn unknown_assay_values_become_none_and_wells_are_explicit_only() {
        let path = tmp_path("unknowns");
        let _ = std::fs::remove_file(&path);
        {
            let conn = Connection::open(&path).unwrap();
            conn.execute_batch(
                "CREATE TABLE records(id TEXT PRIMARY KEY, kind TEXT NOT NULL, \
                 project_id TEXT, parent_id TEXT, revision INTEGER NOT NULL DEFAULT 1, \
                 data_json TEXT NOT NULL, created_at TEXT NOT NULL, updated_at TEXT NOT NULL)",
            )
            .unwrap();
            insert(
                &conn,
                "p1",
                "project",
                None,
                None,
                &json!({"name": "生物膜"}),
            );
            insert(
                &conn,
                "d1",
                "design",
                Some("p1"),
                Some("p1"),
                &json!({
                    "title": "设计",
                    "objective": "",
                    "experimental_unit": "unknown",
                    "group_specs": [{"name": "对照", "role": "control", "condition": ""}],
                    "assays": [{
                        "id": "a1", "title": "检测",
                        "endpoint": "unknown", "method": "", "unit": null,
                        "timepoints": [], "group_names": [],
                        "timepoint_relation": "unknown", "container_format": "unknown"
                    }],
                    "sample_allocations": [{
                        "assay_id": "a1", "group_name": "对照",
                        "timepoint_relation": "unknown",
                        "physical_slots_per_group": null,
                        "physical_slots_scope": "unknown",
                        "evidence_id": ""
                    }],
                    "source_evidence": [{
                        "id": "e1", "source_document": "doc", "source_locator": "", "claim": "c"
                    }],
                    "evidence_conflicts": [{
                        "id": "x1", "topic": "t", "evidence_ids": ["e1", "e2"], "status": "unresolved"
                    }],
                }),
            );
            insert(
                &conn,
                "r1",
                "run",
                Some("p1"),
                Some("d1"),
                &json!({"title": "运行", "status": "planned"}),
            );
            insert(
                &conn,
                "pl1",
                "plate",
                Some("p1"),
                Some("r1"),
                &json!({"name": "板", "format": "96", "rows": 8, "columns": 12}),
            );
            insert(
                &conn,
                "w1",
                "well",
                Some("p1"),
                Some("pl1"),
                &json!({"well_id": "A1", "group_label": "E1", "sample_id": "SYN-CCK8-A1", "note": ""}),
            );
            insert(
                &conn,
                "w2",
                "well",
                Some("p1"),
                Some("pl1"),
                &json!({"well_id": "H12", "group_label": "", "sample_id": "", "note": "x"}),
            );
        }

        let model = load_read_model(&path).unwrap();
        let assay = &model.designs[0].assays[0];
        assert!(assay.endpoint.is_none());
        assert!(assay.method.is_none());
        assert!(assay.unit.is_none());
        assert!(assay.timepoint_relation.is_none());
        assert!(assay.container_format.is_none());
        assert!(assay.evidence_id.is_none());
        assert_eq!(assay.allocations.len(), 1);
        assert_eq!(assay.allocations[0].group_name, "对照");
        assert!(assay.allocations[0].slots_per_group.is_none());
        assert!(model.designs[0].objective.is_none());
        assert!(model.designs[0].experimental_unit.is_none());

        let plate = &model.plates[0];
        assert_eq!(plate.wells.len(), 2);
        assert_eq!(plate.wells[0].coordinate, "A1");
        assert_eq!(plate.wells[0].group_label.as_deref(), Some("E1"));
        assert_eq!(plate.wells[0].sample_id.as_deref(), Some("SYN-CCK8-A1"));
        assert!(plate.wells[0].note.is_none());
        assert_eq!(plate.wells[1].coordinate, "H12");
        assert!(plate.wells[1].group_label.is_none());

        let _ = std::fs::remove_file(&path);
    }
}
