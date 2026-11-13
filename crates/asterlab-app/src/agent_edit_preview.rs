//! Store-backed, read-only agent edit preview.
//!
//! Wires [`asterlab_core::agent_edit_validate`] to the record store. Every
//! query here is a `SELECT`; the module has no write path, no transaction and
//! no cache, so validating a proposal cannot change the database.

use asterlab_core::agent_edit::Proposal;
use asterlab_core::agent_edit_validate::{
    validate, EditTargetStore, StoredBlockVersion, StoredProtocolVersion,
};
use asterlab_core::protocol_block::{ProtocolBlockVersionData, StepBinding};
use rusqlite::{Connection, OptionalExtension};
use serde_json::Value;
use sha2::{Digest, Sha256};

/// Read-only record store view used by the validator.
pub(crate) struct SqliteEditStore<'a> {
    pub conn: &'a Connection,
}

impl EditTargetStore for SqliteEditStore<'_> {
    fn protocol_head(&self, protocol_id: &str) -> Result<Option<StoredProtocolVersion>, String> {
        let row = self
            .conn
            .query_row(
                "SELECT id,revision,data_json FROM records \
                 WHERE kind='protocol_version' AND parent_id=?1 \
                 ORDER BY revision DESC LIMIT 1",
                [protocol_id],
                |row| {
                    Ok((
                        row.get::<_, String>(0)?,
                        row.get::<_, i64>(1)?,
                        row.get::<_, String>(2)?,
                    ))
                },
            )
            .optional()
            .map_err(|error| error.to_string())?;
        let Some((version_id, revision, raw)) = row else {
            return Ok(None);
        };
        let data: Value = serde_json::from_str(&raw)
            .map_err(|error| format!("protocol_version {version_id} data_json 无效：{error}"))?;
        let root_block_version_id = data
            .get("root_block_version_id")
            .and_then(Value::as_str)
            .map(str::to_owned);
        let root_bindings = match data.get("root_bindings") {
            None | Some(Value::Null) => Vec::new(),
            Some(value) => serde_json::from_value::<Vec<StepBinding>>(value.clone())
                .map_err(|error| format!("protocol_version {version_id} root_bindings 无效：{error}"))?,
        };
        Ok(Some(StoredProtocolVersion {
            version_id,
            protocol_id: protocol_id.to_owned(),
            revision,
            root_block_version_id,
            root_bindings,
        }))
    }

    fn block_version(&self, version_id: &str) -> Result<Option<StoredBlockVersion>, String> {
        let row = self
            .conn
            .query_row(
                "SELECT revision,data_json FROM records \
                 WHERE kind='protocol_block_version' AND id=?1",
                [version_id],
                |row| Ok((row.get::<_, i64>(0)?, row.get::<_, String>(1)?)),
            )
            .optional()
            .map_err(|error| error.to_string())?;
        let Some((revision, raw)) = row else {
            return Ok(None);
        };
        let data: ProtocolBlockVersionData = serde_json::from_str(&raw)
            .map_err(|error| format!("protocol_block_version {version_id} data_json 无效：{error}"))?;
        Ok(Some(StoredBlockVersion {
            version_id: version_id.to_owned(),
            revision,
            data,
        }))
    }
}

/// Read-only validate/preview of one proposal payload.
///
/// Returns the `ValidationResult` wire shape. A malformed payload is a caller
/// error (`Err`); a semantic rejection is `status: "blocked"` with `issues`.
pub fn validate_proposal_value(conn: &Connection, proposal: Value) -> Result<Value, String> {
    let proposal: Proposal = serde_json::from_value(proposal)
        .map_err(|error| format!("proposal 不是合法的 agent edit v1 载荷：{error}"))?;
    let store = SqliteEditStore { conn };
    let result = validate(&proposal, &store)?;
    serde_json::to_value(&result).map_err(|error| error.to_string())
}

/// Stable digest over every record row.
///
/// Evidence helper: a command that leaves this digest unchanged changed no
/// record. Rows are read in id order and hashed field by field so the value
/// does not depend on the SQLite file layout or WAL state.
pub fn records_digest(conn: &Connection) -> Result<String, String> {
    let mut statement = conn
        .prepare(
            "SELECT id,kind,COALESCE(parent_id,''),revision,data_json,created_at,updated_at \
             FROM records ORDER BY id",
        )
        .map_err(|error| error.to_string())?;
    let rows = statement
        .query_map([], |row| {
            Ok((
                row.get::<_, String>(0)?,
                row.get::<_, String>(1)?,
                row.get::<_, String>(2)?,
                row.get::<_, i64>(3)?,
                row.get::<_, String>(4)?,
                row.get::<_, String>(5)?,
                row.get::<_, String>(6)?,
            ))
        })
        .map_err(|error| error.to_string())?;
    let mut hasher = Sha256::new();
    let mut count = 0usize;
    for row in rows {
        let (id, kind, parent, revision, data, created_at, updated_at) =
            row.map_err(|error| error.to_string())?;
        for field in [id, kind, parent, revision.to_string(), data, created_at, updated_at] {
            hasher.update(field.as_bytes());
            hasher.update([0x1f]);
        }
        hasher.update([0x1e]);
        count += 1;
    }
    let hash = hasher.finalize();
    Ok(format!(
        "records={count} sha256:{}",
        hash.iter().map(|byte| format!("{byte:02x}")).collect::<String>()
    ))
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::AppCore;
    use serde_json::json;
    use std::fs;

    const PROTOCOL: &str = "protocol-rna-lyse";
    const PROTOCOL_VERSION: &str = "protocol-rna-lyse-v3";
    const STEP: &str = "root/lyse/lyse";

    fn fixture() -> (std::path::PathBuf, AppCore) {
        let dir = std::env::temp_dir().join(format!("asterlab-agent-edit-{}", uuid::Uuid::new_v4()));
        fs::create_dir_all(&dir).unwrap();
        let core = AppCore::open(&dir).unwrap();
        let now = "2026-11-13T00:00:00Z";
        let lyse = json!({
            "block_id": "block-rna-lyse",
            "version": 1,
            "summary": "lysis",
            "parameter_slots": [
                {"key":"volume_per_sample","label":"每样本体积","value_type":"number","unit":"uL","required":true},
                {"key":"sample_count","label":"样本数","value_type":"integer","required":true},
                {"key":"target_concentration","label":"目标浓度","value_type":"number","unit":"ng/uL","required":true},
                {"key":"extra_ratio","label":"额外比例","value_type":"number","required":false}
            ],
            "steps": [
                {"kind":"text","use_id":"lyse","title":"裂解","instruction":"加入裂解液"},
                {"kind":"text","use_id":"incubate","title":"孵育","instruction":"室温孵育"}
            ]
        })
        .to_string();
        let root = json!({
            "block_id": "block-rna-root",
            "version": 1,
            "summary": "root",
            "parameter_slots": [],
            "steps": [{
                "kind": "block_use",
                "use_id": "lyse",
                "block_version_id": "block-rna-lyse-v1",
                "bindings": [
                    {"slot":"volume_per_sample","value":200.0,"unit":"uL"},
                    {"slot":"target_concentration","value":50.0,"unit":"ng/uL"},
                    {"slot":"extra_ratio","value":0.1}
                ]
            }]
        })
        .to_string();
        let version = json!({
            "protocol_id": PROTOCOL,
            "version": 3,
            "body": "",
            "root_block_version_id": "block-rna-root-v1",
            "root_bindings": []
        })
        .to_string();
        {
            let db = core.raw_db();
            db.execute_batch(
                "BEGIN;
                 INSERT INTO records VALUES('project-1','project',NULL,NULL,1,'{}','2026-11-13T00:00:00Z','2026-11-13T00:00:00Z');
                 INSERT INTO records VALUES('block-rna-root','protocol_block','project-1',NULL,1,'{}','2026-11-13T00:00:00Z','2026-11-13T00:00:00Z');
                 INSERT INTO records VALUES('block-rna-lyse','protocol_block','project-1',NULL,1,'{}','2026-11-13T00:00:00Z','2026-11-13T00:00:00Z');
                 INSERT INTO records VALUES('protocol-rna-lyse','protocol','project-1',NULL,1,'{\"title\":\"RNA lysis\"}','2026-11-13T00:00:00Z','2026-11-13T00:00:00Z');
                 COMMIT;",
            )
            .unwrap();
            let rows = [
                (
                    "protocol-rna-lyse-v3",
                    "protocol_version",
                    "project-1",
                    Some(PROTOCOL),
                    3,
                    version.as_str(),
                ),
                (
                    "block-rna-root-v1",
                    "protocol_block_version",
                    "project-1",
                    Some("block-rna-root"),
                    1,
                    root.as_str(),
                ),
                (
                    "block-rna-lyse-v1",
                    "protocol_block_version",
                    "project-1",
                    Some("block-rna-lyse"),
                    1,
                    lyse.as_str(),
                ),
            ];
            for (id, kind, project, parent, revision, data) in rows {
                db.execute(
                    "INSERT INTO records VALUES(?1,?2,?3,?4,?5,?6,?7,?8)",
                    rusqlite::params![id, kind, project, parent, revision, data, now, now],
                )
                .unwrap();
            }
        }
        (dir, core)
    }

    fn binding(slot: &str, value: Value, unit: Option<&str>) -> Value {
        let mut encoded = json!({
            "parameter_key": slot,
            "value": value,
            "source": "protocol_fixed",
            "source_reference": "rna-kit-ifu@2026-03#step-2",
            "confirmed": false
        });
        if let Some(unit) = unit {
            encoded["unit"] = json!(unit);
        }
        encoded
    }

    fn proposal(diffs: Value) -> Value {
        json!({
            "proposal_id": "ae-rna-lyse-001",
            "target": {
                "kind": "protocol",
                "object_id": PROTOCOL,
                "location": {
                    "protocol_version_id": PROTOCOL_VERSION,
                    "scope": {"kind": "assay", "assay_id": "assay-rna-extraction"},
                    "step_path": STEP
                }
            },
            "base_version_id": PROTOCOL_VERSION,
            "expected_version": 3,
            "origin": {"source": "agent", "rationale": "kit instruction update"},
            "diffs": diffs
        })
    }

    fn codes(result: &Value) -> Vec<(String, String)> {
        result["issues"]
            .as_array()
            .unwrap()
            .iter()
            .map(|issue| {
                (
                    issue["code"].as_str().unwrap_or_default().to_owned(),
                    issue["path"].as_str().unwrap_or_default().to_owned(),
                )
            })
            .collect()
    }

    #[test]
    fn preview_rejects_the_three_illegal_input_classes() {
        let (dir, core) = fixture();
        let conn = core.raw_db();

        let illegal_binding = validate_proposal_value(
            conn,
            proposal(json!([{
                "path": "params.volume_per_well",
                "operation": "set",
                "after": binding("volume_per_well", json!(100.0), Some("uL"))
            }])),
        )
        .unwrap();
        assert_eq!(illegal_binding["status"], "blocked");
        assert_eq!(
            codes(&illegal_binding),
            vec![(
                "INVALID_BINDING".to_owned(),
                "params.volume_per_well".to_owned()
            )]
        );

        let wrong_unit = validate_proposal_value(
            conn,
            proposal(json!([{
                "path": "params.volume_per_sample",
                "operation": "set",
                "after": binding("volume_per_sample", json!(0.25), Some("mL"))
            }])),
        )
        .unwrap();
        assert_eq!(wrong_unit["status"], "blocked");
        assert_eq!(
            codes(&wrong_unit),
            vec![(
                "UNIT_MISMATCH".to_owned(),
                "params.volume_per_sample.unit".to_owned()
            )]
        );

        let mut stale = proposal(json!([{
            "path": "params.volume_per_sample",
            "operation": "set",
            "after": binding("volume_per_sample", json!(250.0), Some("uL"))
        }]));
        stale["target"]["location"]["step_path"] = json!("root/lyse/wash");
        let stale = validate_proposal_value(conn, stale).unwrap();
        assert_eq!(stale["status"], "blocked");
        assert_eq!(
            codes(&stale),
            vec![(
                "STALE_REFERENCE".to_owned(),
                "target.location.step_path".to_owned()
            )]
        );

        // Semantic rejection is a result, not a transport error.
        assert!(validate_proposal_value(conn, json!({"proposal_id": "x"})).is_err());
        let _ = fs::remove_dir_all(dir);
    }

    #[test]
    fn preview_resolves_diffs_from_the_store_and_leaves_it_untouched() {
        let (dir, core) = fixture();
        let conn = core.raw_db();
        let before = records_digest(conn).unwrap();
        let rows_before: i64 = conn
            .query_row("SELECT COUNT(*) FROM records", [], |row| row.get(0))
            .unwrap();

        let valid = validate_proposal_value(
            conn,
            proposal(json!([
                {
                    "path": "params.volume_per_sample",
                    "operation": "set",
                    "after": binding("volume_per_sample", json!(250.0), Some("uL"))
                },
                {
                    "path": "params.sample_count",
                    "operation": "set",
                    "after": binding("sample_count", json!(8), None)
                },
                {
                    "path": "params.extra_ratio",
                    "operation": "unset"
                }
            ])),
        )
        .unwrap();
        assert_eq!(valid["status"], "needs_confirmation");
        assert_eq!(valid["current_version_id"], PROTOCOL_VERSION);
        assert_eq!(valid["current_revision"], 3);
        assert_eq!(valid["requires_confirmation"], true);
        assert!(valid["issues"].as_array().unwrap().is_empty());
        assert_eq!(valid["diffs"][0]["current"]["value"], 200.0);
        assert_eq!(valid["diffs"][0]["current"]["unit"], "uL");
        assert_eq!(valid["diffs"][1]["current"], Value::Null);
        assert_eq!(
            valid["invalidated_refs"],
            json!([
                "core:binding:assay:assay-rna-extraction:root/lyse/lyse/volume_per_sample",
                "core:binding:assay:assay-rna-extraction:root/lyse/lyse/sample_count",
                "core:binding:assay:assay-rna-extraction:root/lyse/lyse/extra_ratio"
            ])
        );
        assert!(valid["proposal_digest"]
            .as_str()
            .unwrap()
            .starts_with("sha256:"));

        // Rejected payloads are part of the same read-only path.
        let _ = validate_proposal_value(
            conn,
            proposal(json!([{
                "path": "params.volume_per_well",
                "operation": "set",
                "after": binding("volume_per_well", json!(100.0), Some("uL"))
            }])),
        )
        .unwrap();

        let after = records_digest(conn).unwrap();
        let rows_after: i64 = conn
            .query_row("SELECT COUNT(*) FROM records", [], |row| row.get(0))
            .unwrap();
        assert_eq!(before, after);
        assert_eq!(rows_before, rows_after);

        // The digest must be sensitive to writes, otherwise the equality above
        // would prove nothing.
        conn.execute(
            "INSERT INTO records VALUES('probe','note',NULL,NULL,1,'{}','2026-11-13T00:00:00Z','2026-11-13T00:00:00Z')",
            [],
        )
        .unwrap();
        assert_ne!(records_digest(conn).unwrap(), before);
        conn.execute("DELETE FROM records WHERE id='probe'", []).unwrap();
        assert_eq!(records_digest(conn).unwrap(), before);

        let _ = fs::remove_dir_all(dir);
    }
}
