#![recursion_limit = "256"]

//! Shared AsterLab application core.

#[path = "../../../src/cli.rs"]
mod cli_impl;

mod agent_edit_preview;

use std::path::Path;
use serde_json::{json, Value};
use rusqlite::{Connection, OptionalExtension};

pub struct AppCore {
    db: Connection,
}

impl AppCore {
    pub fn open(data_dir: &Path) -> Result<AppCore, String> {
        let db = cli_impl::open_db(data_dir).map_err(|e| e.message)?;
        db.execute_batch(
            "CREATE TABLE IF NOT EXISTS timer_records(\
                timer_id TEXT PRIMARY KEY,\
                run_id TEXT NOT NULL REFERENCES records(id) ON DELETE CASCADE,\
                task_key TEXT NOT NULL,\
                data_json TEXT NOT NULL,\
                updated_at TEXT NOT NULL\
            );\
            CREATE INDEX IF NOT EXISTS timer_records_run ON timer_records(run_id, task_key);\
            CREATE TABLE IF NOT EXISTS run_archive_state(\
                run_id TEXT PRIMARY KEY,\
                data_json TEXT NOT NULL,\
                updated_at TEXT NOT NULL\
            );",
        ).map_err(|e| e.to_string())?;
        Ok(AppCore { db })
    }

    pub fn raw_db(&self) -> &Connection {
        &self.db
    }

    /// Mutable handle for Core writers that own their transaction, as the CLI
    /// does. The MyGo native bridge uses it to call the same `cli_impl`
    /// functions instead of keeping a second copy of their SQL.
    pub fn raw_db_mut(&mut self) -> &mut Connection {
        &mut self.db
    }

    /// Create (no `id`) or replace (existing `id`) one pantry ledger row.
    /// Whole-object write; omitted optional fields are cleared.
    pub fn pantry_upsert(&mut self, input: Value) -> Result<Value, String> {
        let parsed: cli_impl::PantryUpsertInput =
            serde_json::from_value(input).map_err(|e| format!("INVALID_INPUT: {e}"))?;
        cli_impl::pantry_upsert(&mut self.db, &parsed).map_err(|e| e.message)
    }

    /// Read-only pantry projection. `as_of` pins the date expiry is computed
    /// against; omitting it uses the current UTC date.
    pub fn pantry_list(&self, input: Value) -> Result<Value, String> {
        let parsed: cli_impl::PantryQueryInput =
            serde_json::from_value(input).map_err(|e| format!("INVALID_INPUT: {e}"))?;
        cli_impl::pantry_list(&self.db, &parsed).map_err(|e| e.message)
    }

    /// Stable short-id manifest for external label/printer plugins. Read-only.
    pub fn pantry_export_labels(&self, input: Value) -> Result<Value, String> {
        let parsed: cli_impl::PantryQueryInput =
            serde_json::from_value(input).map_err(|e| format!("INVALID_INPUT: {e}"))?;
        cli_impl::pantry_export_labels(&self.db, &parsed).map_err(|e| e.message)
    }

    /// Retire one pantry item. The row stays as a tombstone so its printed
    /// short id is never reissued; reads stop returning it.
    pub fn pantry_delete(&mut self, input: Value) -> Result<Value, String> {
        let parsed: cli_impl::PantryDeleteInput =
            serde_json::from_value(input).map_err(|e| format!("INVALID_INPUT: {e}"))?;
        cli_impl::pantry_delete(&mut self.db, &parsed).map_err(|e| e.message)
    }

    pub fn load_run(&self, run_id: &str) -> Result<Value, String> {
        let mut v = cli_impl::run_workspace(&self.db, run_id).map_err(|e| e.message)?;
        v["revision"] = json!(self.revision(run_id)?);
        Ok(v)
    }

    fn revision(&self, run_id: &str) -> Result<i64, String> {
        self.db.query_row(
            "SELECT COUNT(*) FROM records WHERE kind='execution_event' AND parent_id=?1",
            rusqlite::params![run_id],
            |row| row.get(0),
        ).map_err(|e| e.to_string())
    }

    fn find_operation_event(&self, run_id: &str, op_id: &str) -> Option<Value> {
        let mut stmt = self.db.prepare(
            "SELECT data_json FROM records WHERE kind='execution_event' AND parent_id=?1",
        ).ok()?;
        let rows = stmt.query_map([run_id], |row| row.get::<_, String>(0)).ok()?;
        for row in rows.flatten() {
            if let Ok(v) = serde_json::from_str::<Value>(&row) {
                if v.get("operation_id").and_then(Value::as_str) == Some(op_id) {
                    return Some(v);
                }
            }
        }
        None
    }

    fn ensure_writable(&self, run_id: &str) -> Result<(), String> {
        let raw: Option<String> = self
            .db
            .query_row(
                "SELECT data_json FROM run_archive_state WHERE run_id=?1",
                [run_id],
                |row| row.get(0),
            )
            .optional()
            .map_err(|e| e.to_string())?;
        if let Some(raw) = raw {
            let state: asterlab_core::record_projection::RecordArchiveState =
                serde_json::from_str(&raw).map_err(|e| e.to_string())?;
            if state.notes_locked() {
                return Err("CONFLICT: Run 已签核归档，记录只读".into());
            }
        }
        Ok(())
    }

    fn find_operation_event_tx(tx: &rusqlite::Transaction<'_>, run_id: &str, op_id: &str) -> Option<Value> {
        let mut stmt = tx.prepare(
            "SELECT id,data_json FROM records WHERE kind='execution_event' AND parent_id=?1",
        ).ok()?;
        let rows = stmt.query_map([run_id], |row| Ok((row.get::<_, String>(0)?, row.get::<_, String>(1)?))).ok()?;
        for row in rows.flatten() {
            if let Ok(mut v) = serde_json::from_str::<Value>(&row.1) {
                if v.get("operation_id").and_then(Value::as_str) == Some(op_id) {
                    v["id"] = json!(row.0);
                    return Some(v);
                }
            }
        }
        None
    }

    pub fn save_readout(&mut self, input: Value) -> Result<Value, String> {
        let run_id = input.get("run_id").and_then(Value::as_str).map(str::to_owned)
            .ok_or_else(|| "run_id is required".to_owned())?;
        self.ensure_writable(&run_id)?;
        let op_id = input.get("operation_id").and_then(Value::as_str).map(str::to_owned);
        let tx = self.db.transaction_with_behavior(rusqlite::TransactionBehavior::Immediate).map_err(|e| e.to_string())?;
        if let Some(op) = &op_id {
            if let Some(existing) = Self::find_operation_event_tx(&tx, &run_id, op) {
                let existing_payload = existing.get("operation_payload").cloned().unwrap_or(Value::Null);
                if existing_payload == input {
                    tx.commit().map_err(|e| e.to_string())?;
                    return self.load_run(&run_id);
                }
                return Err("operation id conflict: same operation_id with different payload".to_owned());
            }
        }
        let mut event = input.clone();
        if event.get("event_type").is_none() {
            event["event_type"] = json!("measurement");
        }
        if event.get("content").is_none() {
            event["content"] = json!("结构化记录");
        }
        if let Some(op) = &op_id {
            event["operation_id"] = json!(op);
        }
        event["operation_payload"] = input.clone();
        let saved = cli_impl::create_execution_event_tx(&tx, &run_id, &event).map_err(|e| e.message)?;
        Self::trigger_timers_for_anchor_tx(
            &tx,
            &run_id,
            saved["id"].as_str().unwrap_or_default(),
            saved["data"]["task_key"].as_str(),
            saved["data"]["occurred_at"].as_str().unwrap_or_default(),
        )?;
        tx.commit().map_err(|e| e.to_string())?;
        self.load_run(&run_id)
    }

    pub fn save_readouts_batch(&mut self, input: Value) -> Result<Value, String> {
        let run_id = input["run_id"].as_str().ok_or("run_id is required")?.to_owned();
        let operation_id = input["operation_id"].as_str().filter(|s| !s.is_empty()).ok_or("operation_id is required")?.to_owned();
        self.ensure_writable(&run_id)?;
        let cells = input["cells"].as_array().ok_or("cells is required")?;
        let schema = input["schema_id"].as_str().filter(|s| !s.is_empty()).ok_or("schema_id is required")?;
        if input["schema_version"].as_u64() != Some(1) {
            return Err("unsupported result schema".into());
        }
        if cells.is_empty() || cells.len() > 1536 { return Err("batch must contain 1..=1536 cells".into()); }
        let tx = self.db.transaction_with_behavior(rusqlite::TransactionBehavior::Immediate).map_err(|e| e.to_string())?;
        eprintln!("run.save_batch layer=core transaction=start run_id={} operation_id={} schema_id={} cell_count={}", run_id, operation_id, schema, cells.len());
        if let Some(existing) = Self::find_operation_event_tx(&tx, &run_id, &operation_id) {
            if existing.get("operation_payload") == Some(&input) {
                let event_id = existing["id"].as_str().unwrap_or_default().to_owned();
                let revision = tx.query_row("SELECT COUNT(*) FROM records WHERE kind='execution_event' AND parent_id=?1", [&run_id], |r| r.get::<_, i64>(0)).map_err(|e|e.to_string())? as u64;
                tx.commit().map_err(|e|e.to_string())?;
                return Ok(json!({"run_id":run_id,"revision":revision,"batch_id":existing["batch_id"],"event_id":event_id,"accepted":cells.len(),"rejected":[]}));
            }
            return Err("operation id conflict: same operation_id with different payload".into());
        }
        let revision = tx.query_row("SELECT COUNT(*) FROM records WHERE kind='execution_event' AND parent_id=?1", [&run_id], |r| r.get::<_, i64>(0)).map_err(|e|e.to_string())? as u64;
        if let Some(expected) = input["expected_revision"].as_u64() {
            if expected != revision { return Err(format!("stale revision: expected {expected}, current {revision}")); }
        }
        let mut seen = std::collections::HashSet::new();
        let mut measurements = Vec::with_capacity(cells.len());
        let mut readouts = Vec::with_capacity(cells.len());
        for (index, cell) in cells.iter().enumerate() {
            let row = cell["row"].as_u64().ok_or_else(||format!("cell {index}: invalid row"))?;
            let column = cell["column"].as_u64().ok_or_else(||format!("cell {index}: invalid column"))?;
            if (schema == "cck8.plate96.od450.v1" && (row > 7 || column > 11)) || !seen.insert((row,column)) {
                return Err(format!("cell {index}: out of range or duplicate position"));
            }
            let field = cell["field"].as_str().filter(|s| !s.is_empty()).ok_or_else(||format!("cell {index}: field is required"))?;
            let status = cell["status"].as_str().unwrap_or("");
            let value = cell["value"].as_f64();
            if !matches!(status, "raw" | "missing") || (status == "raw" && value.is_none()) || (status == "missing" && value.is_some()) {
                return Err(format!("cell {index}: invalid status/value"));
            }
            if status == "raw" {
                let sample_id = cell["sample_id"].as_str().filter(|id|!id.trim().is_empty()).map(Value::from).unwrap_or(Value::Null);
                measurements.push(json!({"endpoint":field,"value":value.unwrap(),"unit":"","sample_id":sample_id,"row":row,"column":column,"group":cell["group"],"replicate":cell["replicate"],"status":status,"field":field}));
                let readout_id = format!("readout-{operation_id}-{row}-{column}-{field}");
                readouts.push(json!({"readout_id":readout_id,"endpoint":field,"value":value.unwrap(),"unit":"","sample_id":sample_id,"result_kind":"raw"}));
            }
        }
        let batch_id = format!("batch-{operation_id}");
        let event = json!({"run_id":run_id,"event_type":"measurement","content":"Structured readout batch","task_key":input["task_key"],"assay_id":input["assay_id"],"timepoint":input["timepoint"],"protocol_step_path":input["protocol_step_path"],"protocol_version_id":input["protocol_version_id"],"measurements":measurements,"result_cells":cells,"readouts":readouts,"operation_id":operation_id,"operation_payload":input,"batch_id":batch_id,"schema_id":schema,"schema_version":1});
        let saved = cli_impl::create_execution_event_tx(&tx, &run_id, &event).map_err(|e| e.message)?;
        let event_id = saved["id"].as_str().unwrap_or_default().to_owned();
        let occurred_at = saved["data"]["occurred_at"].as_str().unwrap_or_default().to_owned();
        let task_key = saved["data"]["task_key"].as_str().map(str::to_owned);
        let revision = revision + 1;
        Self::trigger_timers_for_anchor_tx(&tx, &run_id, &event_id, task_key.as_deref(), &occurred_at)?;
        tx.commit().map_err(|e|e.to_string())?;
        eprintln!("run.save_batch layer=core transaction=commit run_id={} operation_id={} batch_id={} event_id={} cell_count={}", run_id, operation_id, batch_id, event_id, cells.len());
        Ok(json!({"run_id":run_id,"revision":revision,"batch_id":batch_id,"event_id":event_id,"accepted":cells.len(),"rejected":[]}))
    }

    pub fn complete_task(&mut self, input: Value) -> Result<Value, String> {
        let run_id = input.get("run_id").and_then(Value::as_str).map(str::to_owned)
            .ok_or_else(|| "run_id is required".to_owned())?;
        self.ensure_writable(&run_id)?;
        let op_id = input.get("operation_id").and_then(Value::as_str).map(str::to_owned);
        let tx = self.db.transaction_with_behavior(rusqlite::TransactionBehavior::Immediate).map_err(|e| e.to_string())?;
        if let Some(op) = &op_id {
            if let Some(existing) = Self::find_operation_event_tx(&tx, &run_id, op) {
                let existing_payload = existing.get("operation_payload").cloned().unwrap_or(Value::Null);
                if existing_payload == input {
                    tx.commit().map_err(|e| e.to_string())?;
                    return self.load_run(&run_id);
                }
                return Err("operation id conflict: same operation_id with different payload".to_owned());
            }
        }
        let mut event = input.clone();
        if event.get("event_type").is_none() {
            event["event_type"] = json!("operation");
        }
        if event.get("content").is_none() {
            event["content"] = json!("完成任务");
        }
        if event.get("step_status").is_none() {
            event["step_status"] = json!("completed");
        }
        if let Some(op) = &op_id {
            event["operation_id"] = json!(op);
        }
        event["operation_payload"] = input.clone();
        let saved = cli_impl::create_execution_event_tx(&tx, &run_id, &event).map_err(|e| e.message)?;
        Self::trigger_timers_for_anchor_tx(
            &tx,
            &run_id,
            saved["id"].as_str().unwrap_or_default(),
            saved["data"]["task_key"].as_str(),
            saved["data"]["occurred_at"].as_str().unwrap_or_default(),
        )?;
        tx.commit().map_err(|e| e.to_string())?;
        self.load_run(&run_id)
    }

    pub fn list_designs(&self, project_id: Option<&str>) -> Result<Value, String> {
        cli_impl::list_designs(&self.db, project_id).map_err(|e| e.message)
    }

    pub fn load_design(&self, design_id: &str) -> Result<Value, String> {
        cli_impl::load_design(&self.db, design_id).map_err(|e| e.message)
    }

    pub fn get_protocol_version(&self, protocol_version_id: &str) -> Result<Value, String> {
        cli_impl::protocol_version_get(&self.db, protocol_version_id).map_err(|e| e.message)
    }

    pub fn list_protocol_versions(&self, protocol_id: &str) -> Result<Value, String> {
        let versions = cli_impl::protocol_version_list(&self.db, protocol_id).map_err(|e| e.message)?;
        Ok(json!({"versions": versions}))
    }
    pub fn save_design(&mut self, input: Value) -> Result<Value, String> {
        // create path
        if let (Some(project_id), Some(design)) = (
            input.get("project_id").and_then(Value::as_str),
            input.get("design"),
        ) {
            return cli_impl::create_design_record(&self.db, project_id, design).map_err(|e| e.message);
        }
        // revise path
        let id = input.get("id").or_else(|| input.get("design_id")).and_then(Value::as_str)
            .ok_or_else(|| "design id is required".to_owned())?;
        let expected = input.get("expected_revision").and_then(Value::as_i64)
            .ok_or_else(|| "expected_revision is required".to_owned())?;
        let changes = input.get("changes").cloned().unwrap_or_else(|| input.get("design").cloned().unwrap_or(Value::Null));
        cli_impl::revise_design_record(&self.db, id, expected, &changes).map_err(|e| e.message)
    }

    pub fn apply_asterdoc_command(&mut self, input: Value) -> Result<Value, String> {
        cli_impl::apply_asterdoc_command(&mut self.db, &input).map_err(|e| e.message)
    }

    pub fn create_asterdoc_design(&mut self, input: Value) -> Result<Value, String> {
        cli_impl::create_asterdoc_design(&mut self.db, &input).map_err(|e| e.message)
    }

    pub fn save_asterdoc(&mut self, input: Value) -> Result<Value, String> {
        cli_impl::save_asterdoc_version(&mut self.db, &input).map_err(|e| e.message)
    }

    pub fn load_asterdoc(&self, design_id: &str) -> Result<Value, String> {
        cli_impl::load_asterdoc(&self.db, design_id).map_err(|e| e.message)
    }

    pub fn load_asterdoc_at(&self, design_id: &str, revision: u64) -> Result<Value, String> {
        cli_impl::load_asterdoc_at(&self.db, design_id, revision).map_err(|e| e.message)
    }

    pub fn assess_asterdoc(&self, input: Value) -> Result<Value, String> {
        cli_impl::assess_asterdoc(&self.db, &input).map_err(|e| e.message)
    }

    pub fn create_run_from_asterdoc(&mut self, design_id: &str, revision: u64, operation_id: Option<&str>, title: Option<&str>, acknowledged: &[String]) -> Result<Value, String> {
        cli_impl::create_run_from_asterdoc(&mut self.db, design_id, revision, operation_id, title, acknowledged)
            .map_err(|e| e.message)
    }

    pub fn compile_asterdoc(&self, design_id: &str, revision: u64) -> Result<Value, String> {
        let doc = cli_impl::load_asterdoc_at(&self.db, design_id, revision).map_err(|e| e.message)?;
        let source = doc["source"].as_str().unwrap_or("");
        let design = cli_impl::compile_asterdoc_to_design(source).map_err(|e| e.message)?;
        Ok(json!({"design": design, "document": doc}))
    }

    pub fn create_run(&mut self, design_id: &str, title: Option<&str>, operation_id: Option<&str>) -> Result<Value, String> {
        cli_impl::create_run_for_design(&mut self.db, design_id, title, operation_id).map_err(|e| e.message)
    }

    pub fn save_asset_link(&mut self, input: Value) -> Result<Value, String> {
        cli_impl::save_asset_link(&mut self.db, &input).map_err(|e| e.message)
    }

    pub fn list_asset_links(&self, run_id: &str) -> Result<Value, String> {
        cli_impl::list_asset_links(&self.db, run_id).map_err(|e| e.message)
    }

    pub fn save_timer_record(&mut self, timer: &asterlab_core::timer::TimerRecord) -> Result<(), String> {
        self.ensure_writable(&timer.run_id)?;
        let run_kind: Option<String> = self.db.query_row(
            "SELECT kind FROM records WHERE id=?1", [&timer.run_id], |row| row.get(0)
        ).optional().map_err(|e| e.to_string())?;
        if run_kind.as_deref() != Some("run") {
            return Err("timer run_id must reference a Run".into());
        }
        let payload = serde_json::to_string(timer).map_err(|e| e.to_string())?;
        self.db.execute(
            "INSERT INTO timer_records(timer_id,run_id,task_key,data_json,updated_at) VALUES(?1,?2,?3,?4,datetime('now')) \
             ON CONFLICT(timer_id) DO UPDATE SET run_id=excluded.run_id,task_key=excluded.task_key,data_json=excluded.data_json,updated_at=excluded.updated_at",
            rusqlite::params![timer.timer_id, timer.run_id, timer.task_key, payload],
        ).map_err(|e| e.to_string())?;
        Ok(())
    }

    pub fn load_timer_records(&self, run_id: &str) -> Result<Vec<asterlab_core::timer::TimerRecord>, String> {
        let mut statement = self.db.prepare(
            "SELECT data_json FROM timer_records WHERE run_id=?1 ORDER BY task_key,timer_id"
        ).map_err(|e| e.to_string())?;
        let rows = statement.query_map([run_id], |row| row.get::<_, String>(0)).map_err(|e| e.to_string())?;
        rows.map(|row| {
            let json = row.map_err(|e| e.to_string())?;
            serde_json::from_str(&json).map_err(|e| e.to_string())
        }).collect()
    }

    pub fn trigger_timers_for_anchor(&mut self, run_id: &str, event_id: &str, task_key: Option<&str>, occurred_at_utc: &str) -> Result<usize, String> {
        let tx = self.db.transaction().map_err(|e| e.to_string())?;
        let triggered = Self::trigger_timers_for_anchor_tx(&tx, run_id, event_id, task_key, occurred_at_utc)?;
        tx.commit().map_err(|e| e.to_string())?;
        Ok(triggered)
    }

    /// Apply anchor-triggered timers inside the caller's transaction so a saved
    /// event and its t0 timer start commit atomically. If this fails, the event
    /// is rolled back and a retry cannot observe an event without its timer.
    pub fn trigger_timers_for_anchor_tx(
        tx: &rusqlite::Transaction<'_>, run_id: &str, event_id: &str,
        task_key: Option<&str>, occurred_at_utc: &str,
    ) -> Result<usize, String> {
        let mut statement = tx.prepare(
            "SELECT timer_id,data_json FROM timer_records WHERE run_id=?1 ORDER BY timer_id"
        ).map_err(|e| e.to_string())?;
        let rows = statement.query_map([run_id], |row| Ok((row.get::<_, String>(0)?, row.get::<_, String>(1)?)))
            .map_err(|e| e.to_string())?;
        let mut updates = Vec::new();
        for row in rows {
            let (id, json) = row.map_err(|e| e.to_string())?;
            let mut timer: asterlab_core::timer::TimerRecord = serde_json::from_str(&json).map_err(|e| e.to_string())?;
            if timer.trigger_anchor(event_id, task_key, occurred_at_utc)? {
                updates.push((id, serde_json::to_string(&timer).map_err(|e| e.to_string())?));
            }
        }
        drop(statement);
        for (id, payload) in &updates {
            tx.execute("UPDATE timer_records SET data_json=?1,updated_at=datetime('now') WHERE timer_id=?2", rusqlite::params![payload,id])
                .map_err(|e| e.to_string())?;
        }
        Ok(updates.len())
    }
    /// Read-only `agent_edit.validate`: resolve a proposal against the current
    /// head and return the preview. Writes nothing; see
    /// `docs/contracts/agent-edit-v1.md`.
    pub fn agent_edit_validate(&self, proposal: Value) -> Result<Value, String> {
        agent_edit_preview::validate_proposal_value(&self.db, proposal)
    }

    /// Bridge v2 generic write channel (INT-01 §ExecuteCommand): one envelope,
    /// routed to the same CLI-side functions the named tools use, in one
    /// transaction, with the same validation. Go passes the envelope; Core
    /// decodes `payload_json` (the exact CLI `--input` JSON text) and never
    /// lets Go rebuild or default it.
    ///
    /// Supported `kind` values (frozen exposure list, `mygo-v1-contract.md` §BE-08/BE-09):
    /// `execution_event.create` (append-only actuals, incl. extended lineage
    /// fields and `well.set` 1D lanes through the same Core validators),
    /// `readout.submit` (scalar + multi-value), `readout.batch` (`cells` or
    /// `csv`, one transaction), `run.create` (incl. cross-Design
    /// `source_run_id`), plus `note.create`, `run.revise`, `run.set_status`.
    pub fn execute_command(&mut self, kind: &str, payload: Value, idempotency_key: &str) -> Result<Value, String> {
        if idempotency_key.trim().is_empty() {
            return Err("INVALID_INPUT: idempotency_key is required (one per user action, never per retry)".into());
        }
        match kind {
            "execution_event.create" => {
                let run_id = payload.get("run_id").and_then(Value::as_str).unwrap_or("").to_owned();
                let tx = self.db.unchecked_transaction().map_err(|e| e.to_string())?;
                if let Some(existing) = find_command_event_tx(&tx, &run_id, idempotency_key).map_err(|e| e.to_string())? {
                    let same = existing.get("operation_payload").cloned().unwrap_or(Value::Null) == payload;
                    let pantry_item = payload.get("pantry_item_id").and_then(Value::as_str).map(str::to_owned);
                    let notice = pantry_notice_for(&tx, pantry_item.as_deref())?;
                    tx.commit().map_err(|e| e.to_string())?;
                    if same {
                        let mut out = serde_json::json!({"record": existing, "replayed": true});
                        if let Some(notice) = notice {
                            out["pantry_notices"] = serde_json::json!([notice]);
                        } else {
                            out["pantry_notices"] = serde_json::json!([]);
                        }
                        return Ok(out);
                    }
                    return Err("IDEMPOTENCY_MISMATCH: same idempotency_key with different payload; mint a new key for a new action".into());
                }
                let mut event = payload.clone();
                event["operation_id"] = serde_json::json!(idempotency_key);
                event["operation_payload"] = payload.clone();
                let (item_id, notice) = {
                    let item = payload.get("pantry_item_id").and_then(Value::as_str).map(str::to_owned);
                    let notice = pantry_notice_for(&tx, item.as_deref()).map_err(|e| e.to_string())?;
                    (item, notice)
                };
                if let Some(item_id) = item_id.as_deref() {
                    if item_id.trim().is_empty() {
                        return Err("INVALID_INPUT: 储藏室条目 ID不能为空".into());
                    }
                    if pantry_lookup(&tx, item_id).map_err(|e| e.to_string())?.is_none() {
                        return Err(format!("NOT_FOUND: 储藏室条目不存在：{item_id}"));
                    }
                }
                let record = cli_impl::create_execution_event_tx(&tx, &run_id, &event).map_err(map_command_error)?;
                Self::trigger_timers_for_anchor_tx(
                    &tx,
                    &run_id,
                    record["id"].as_str().unwrap_or_default(),
                    record["data"]["task_key"].as_str(),
                    record["data"]["occurred_at"].as_str().unwrap_or_default(),
                )?;
                tx.commit().map_err(|e| format!("INTERNAL: {e}"))?;
                let mut out = serde_json::json!({"record": record, "replayed": false});
                if let Some(notice) = notice {
                    out["pantry_notices"] = serde_json::json!([notice]);
                } else {
                    out["pantry_notices"] = serde_json::json!([]);
                }
                Ok(out)
            }
            "readout.submit" | "readout.batch" => {
                let request = payload.clone();
                let planned = {
                    let workspace = load_typed_workspace(&self.db, request.get("run_id").and_then(Value::as_str).unwrap_or(""))?;
                    plan_readout_request(kind, request.clone(), &workspace)?
                };
                let run_id = request.get("run_id").and_then(Value::as_str).unwrap_or("").to_owned();
                let tx = self.db.unchecked_transaction().map_err(|e| format!("INTERNAL: {e}"))?;
                if let Some(existing) = find_readout_tx(&tx, &run_id, &planned.key).map_err(|e| e.to_string())? {
                    let same = existing.get("operation_payload").cloned().unwrap_or(Value::Null) == planned.payload;
                    tx.commit().map_err(|e| e.to_string())?;
                    if same {
                        return Ok(serde_json::json!({"record": existing, "idempotency_key": planned.key, "replayed": true, "accepted": planned.count}));
                    }
                    return Err("IDEMPOTENCY_MISMATCH: same idempotency_key with different payload; mint a new key for a new action".into());
                }
                let submitted = if kind == "readout.submit" {
                    submit_scalar_or_typed_tx(&tx, &run_id, request, idempotency_key)?
                } else {
                    submit_batch_tx(&tx, &run_id, request, idempotency_key)?
                };
                if submitted.get("replayed").and_then(Value::as_bool) != Some(true) {
                    if let Some(record) = submitted.get("record") {
                        Self::trigger_timers_for_anchor_tx(
                            &tx,
                            &run_id,
                            record["id"].as_str().unwrap_or_default(),
                            record["data"]["task_key"].as_str(),
                            record["data"]["occurred_at"].as_str().unwrap_or_default(),
                        )?;
                    }
                }
                tx.commit().map_err(|e| format!("INTERNAL: {e}"))?;
                Ok(submitted)
            }
            "run.create" => {
                let tx = self.db.unchecked_transaction().map_err(|e| format!("INTERNAL: {e}"))?;
                if let Some(existing) = find_run_tx(&tx, idempotency_key).map_err(|e| e.to_string())? {
                    let same = existing.get("data").and_then(|d| d.get("operation_payload")).cloned().unwrap_or(Value::Null) == payload;
                    tx.commit().map_err(|e| e.to_string())?;
                    if same {
                        return Ok(serde_json::json!({"record": existing, "replayed": true}));
                    }
                    return Err("IDEMPOTENCY_MISMATCH: same idempotency_key with different payload; mint a new key for a new action".into());
                }
                let record = create_run_tx(&tx, &payload, Some(idempotency_key)).map_err(|e| e)?;
                tx.commit().map_err(|e| format!("INTERNAL: {e}"))?;
                Ok(serde_json::json!({"record": record, "replayed": false}))
            }
            "note.create" => {
                let record = create_note_tx(&mut self.db, &payload, idempotency_key)?;
                Ok(record)
            }
            "run.revise" | "run.set_status" => {
                if kind == "run.set_status"
                    && payload.get("status").and_then(Value::as_str) == Some("in_progress")
                {
                    let id = payload.get("id").and_then(Value::as_str).unwrap_or("");
                    cli_impl::ensure_run_protocol_ready(&self.db, id).map_err(map_command_error)?;
                }
                let record = apply_simple_command(&mut self.db, kind, &payload).map_err(|e: String| e)?;
                Ok(serde_json::json!({"record": record, "replayed": false}))
            }
            "record.sign_archive" => {
                let run_id = payload.get("run_id").and_then(Value::as_str).unwrap_or("");
                let signed_by = payload.get("signed_by").and_then(Value::as_str).unwrap_or("").trim();
                let signed_at_utc = payload.get("signed_at_utc").and_then(Value::as_str).unwrap_or("");
                if run_id.trim().is_empty() || signed_by.is_empty() || signed_at_utc.trim().is_empty() {
                    return Err("INVALID_INPUT: record.sign_archive requires run_id, signed_by and signed_at_utc".into());
                }
                let view = match payload.get("view").and_then(Value::as_str).map(str::trim).unwrap_or("") {
                    "" | "unblinded" => asterlab_core::record_projection::ProjectionView::Unblinded,
                    "blind" => asterlab_core::record_projection::ProjectionView::Blind,
                    other => return Err(format!("INVALID_INPUT: view 只能是 blind 或 unblinded，收到 {other}")),
                };
                let state = cli_impl::sign_and_archive_record(
                    &self.db,
                    run_id,
                    view,
                    payload.get("fov_per_well").and_then(Value::as_u64).unwrap_or(1) as u32,
                    payload.get("now_utc").and_then(Value::as_str),
                    asterlab_core::record_projection::ArchiveSignature {
                        signed_by: signed_by.to_owned(),
                        signed_at_utc: signed_at_utc.to_owned(),
                        statement: payload.get("statement").and_then(Value::as_str).filter(|text| !text.trim().is_empty()).map(str::to_owned),
                    },
                ).map_err(map_command_error)?;
                Ok(serde_json::json!({"record": state, "replayed": false}))
            }
            other => Err(format!("INVALID_INPUT: unknown command kind: {other}")),
        }
    }

    /// Bridge v2 generic read channel (INT-01 §QueryProjection): side-effect
    /// free; never writes records, creates timers, or invents defaults. Go
    /// decodes the returned CLI-result JSON; it never re-derives numbers,
    /// states, or groupings from it.
    ///
    /// Supported `kind` values: `run.workspace` (input `{id, view?}`;
    /// `view=blind` masks frozen group identity only when the frozen design
    /// declared a non-empty `blinding` basis), `workflow.progress` (groups by
    /// actual event `(step_path, timepoint)`), `workflow.inspect_tree`
    /// (identity/hierarchy; classic-Design Runs return empty roots with an
    /// explanatory issue, never `NOT_FOUND`), `protocol.time_budget`
    /// (Core serial projection over one block version's persisted
    /// OperationSpec v2 sidecars; read-only).
    pub fn query_projection(&self, kind: &str, params: Value) -> Result<Value, String> {
        match kind {
            "run.workspace" => {
                let id = params.get("id").and_then(Value::as_str).unwrap_or("");
                if id.trim().is_empty() {
                    return Err("INVALID_INPUT: run.workspace requires id".into());
                }
                let view = match params.get("view").and_then(Value::as_str).map(str::trim).unwrap_or("") {
                    "" | "unblinded" => asterlab_core::record_projection::ProjectionView::Unblinded,
                    "blind" => asterlab_core::record_projection::ProjectionView::Blind,
                    other => return Err(format!("INVALID_INPUT: view 只能是 blind 或 unblinded，收到 {other}")),
                };
                cli_impl::run_workspace_for_view(&self.db, id, view).map_err(map_command_error)
            }
            "record.projection" => {
                let run_id = params.get("run_id").and_then(Value::as_str).unwrap_or("");
                if run_id.trim().is_empty() {
                    return Err("INVALID_INPUT: record.projection requires run_id".into());
                }
                let view = match params.get("view").and_then(Value::as_str).map(str::trim).unwrap_or("") {
                    "" | "unblinded" => asterlab_core::record_projection::ProjectionView::Unblinded,
                    "blind" => asterlab_core::record_projection::ProjectionView::Blind,
                    other => return Err(format!("INVALID_INPUT: view 只能是 blind 或 unblinded，收到 {other}")),
                };
                cli_impl::record_projection(
                    &self.db,
                    run_id,
                    view,
                    params.get("fov_per_well").and_then(Value::as_u64).unwrap_or(1) as u32,
                    params.get("now_utc").and_then(Value::as_str),
                ).map_err(map_command_error)
            }
            "record.archive_state" => {
                let run_id = params.get("run_id").and_then(Value::as_str).unwrap_or("");
                if run_id.trim().is_empty() {
                    return Err("INVALID_INPUT: record.archive_state requires run_id".into());
                }
                cli_impl::load_archive_state(&self.db, run_id)
                    .and_then(|state| serde_json::to_value(&state).map_err(|e| cli_impl::CliError::internal(e.to_string())))
                    .map_err(map_command_error)
            }
            "protocol.time_budget" => {
                let block_version_id = params.get("block_version_id").and_then(Value::as_str).unwrap_or("");
                if block_version_id.trim().is_empty() {
                    return Err("INVALID_INPUT: protocol.time_budget requires block_version_id".into());
                }
                cli_impl::protocol_time_budget(&self.db, block_version_id).map_err(map_command_error)
            }
            "workflow.progress" => {
                let run_id = params.get("run_id").and_then(Value::as_str).unwrap_or("");
                if run_id.trim().is_empty() {
                    return Err("INVALID_INPUT: run_id 不能为空".into());
                }
                let workspace: asterlab_core::run_workspace::RunWorkspace =
                    serde_json::from_value(cli_impl::run_workspace(&self.db, run_id).map_err(map_command_error)?)
                        .map_err(|e| format!("INTERNAL: {e}"))?;
                let query = asterlab_core::sample_progress::SampleProgressQuery {
                    step_path: params.get("step_path").and_then(Value::as_str).map(str::to_owned),
                    timepoint: params.get("timepoint").and_then(Value::as_str).map(str::to_owned),
                    sample_id: params.get("sample_id").and_then(Value::as_str).map(str::to_owned),
                    sample_ids: params.get("sample_ids").and_then(Value::as_array).map(|a| {
                        a.iter().filter_map(Value::as_str).map(str::to_owned).collect()
                    }).unwrap_or_default(),
                };
                serde_json::to_value(asterlab_core::sample_progress::build_from_workspace(&workspace, &query))
                    .map_err(|e| format!("INTERNAL: {e}"))
            }
            "workflow.inspect_tree" => {
                let design_id = params.get("design_id").and_then(Value::as_str).map(str::to_owned);
                let run_id = params.get("run_id").and_then(Value::as_str).map(str::to_owned);
                if design_id.as_deref().is_none_or(str::is_empty) && run_id.as_deref().is_none_or(str::is_empty) {
                    return Err("INVALID_INPUT: workflow.inspect_tree requires design_id or run_id".into());
                }
                cli_impl::workflow_tree_for_db(
                    &self.db,
                    design_id.as_deref(),
                    params.get("revision").and_then(Value::as_u64),
                    run_id.as_deref(),
                ).map_err(map_command_error)
            }
            other => Err(format!("INVALID_INPUT: unknown query kind: {other}")),
        }
    }
}
/// Map the shared CLI error into the Bridge v2 envelope prefix. Stable Core
/// `code` values travel verbatim in the `ok: false` envelope; `message` is
/// display-only and Go maps the prefix (`INVALID:` / `NOT_FOUND:` /
/// `CONFLICT:`) to its typed errors without parsing message text.
fn map_command_error(e: cli_impl::CliError) -> String {
    match e.code {
        "INVALID_INPUT" | "INVALID_PROPOSAL" | "INVALID_BINDING" | "UNIT_MISMATCH"
        | "FORBIDDEN_FIELD" | "CONFIRMATION_REQUIRED" | "CONFIRMATION_MISMATCH"
        | "STEP_REFERENCED" => format!("INVALID_INPUT: {}", e.message),
        "NOT_FOUND" | "STALE_REFERENCE" => format!("NOT_FOUND: {}", e.message),
        "REVISION_CONFLICT" | "VERSION_CONFLICT" => format!("REVISION_CONFLICT: {}", e.message),
        "IDEMPOTENCY_MISMATCH" => format!("IDEMPOTENCY_MISMATCH: {}", e.message),
        _ => format!("INTERNAL: {}", e.message),
    }
}

/// Envelope-keyed replay lookup for `execution_event.create`: the generic
/// channel stores the caller-supplied `idempotency_key` as `operation_id` and
/// the exact `payload_json` as `operation_payload`, so the same key + same
/// payload replays the first result and a different payload is a mismatch.
fn find_command_event_tx(tx: &rusqlite::Transaction<'_>, run_id: &str, key: &str) -> Result<Option<Value>, String> {
    let id: Option<String> = tx.query_row(
        "SELECT id FROM records WHERE kind='execution_event' AND parent_id=?1 AND json_extract(data_json,'$.operation_id')=?2 ORDER BY created_at LIMIT 1",
        rusqlite::params![run_id, key],
        |row| row.get(0),
    ).optional().map_err(|e| e.to_string())?;
    match id {
        Some(id) => {
            let (raw, created, updated): (String, String, String) = tx.query_row(
                "SELECT data_json,created_at,updated_at FROM records WHERE id=?1",
                [&id],
                |row| Ok((row.get(0)?, row.get(1)?, row.get(2)?)),
            ).map_err(|e| e.to_string())?;
            let mut data: Value = serde_json::from_str(&raw).map_err(|e| e.to_string())?;
            data["id"] = json!(id.clone());
            data["kind"] = json!("execution_event");
            data["parent_id"] = json!(run_id);
            data["created_at"] = json!(created);
            data["updated_at"] = json!(updated);
            Ok(Some(data))
        }
        None => Ok(None),
    }
}

/// Readout replay lookup keyed by the planner-derived content key (same rule
/// as the named `readout.*` tools); the envelope key only gates the
/// mismatch check for `run.create`-style envelope idempotency.
fn find_readout_tx(tx: &rusqlite::Transaction<'_>, run_id: &str, key: &str) -> Result<Option<Value>, String> {
    let id: Option<String> = tx.query_row(
        "SELECT id FROM records WHERE kind='execution_event' AND parent_id=?1 AND json_extract(data_json,'$.operation_id')=?2 ORDER BY created_at LIMIT 1",
        rusqlite::params![run_id, key],
        |row| row.get(0),
    ).optional().map_err(|e| e.to_string())?;
    match id {
        Some(id) => {
            let (raw, created, updated): (String, String, String) = tx.query_row(
                "SELECT data_json,created_at,updated_at FROM records WHERE id=?1",
                [&id],
                |row| Ok((row.get(0)?, row.get(1)?, row.get(2)?)),
            ).map_err(|e| e.to_string())?;
            let mut data: Value = serde_json::from_str(&raw).map_err(|e| e.to_string())?;
            data["id"] = json!(id.clone());
            data["kind"] = json!("execution_event");
            data["parent_id"] = json!(run_id);
            data["created_at"] = json!(created);
            data["updated_at"] = json!(updated);
            Ok(Some(data))
        }
        None => Ok(None),
    }
}

/// Envelope-keyed replay lookup for `run.create`: Runs store the envelope
/// `idempotency_key` as `data.operation_id` and the exact payload as
/// `data.operation_payload`.
fn find_run_tx(tx: &rusqlite::Transaction<'_>, key: &str) -> Result<Option<Value>, String> {
    let id: Option<String> = tx.query_row(
        "SELECT id FROM records WHERE kind='run' AND json_extract(data_json,'$.operation_id')=?1 ORDER BY created_at LIMIT 1",
        [key],
        |row| row.get(0),
    ).optional().map_err(|e| e.to_string())?;
    match id {
        Some(id) => {
            let (raw, created, updated): (String, String, String) = tx.query_row(
                "SELECT data_json,created_at,updated_at FROM records WHERE id=?1",
                [&id],
                |row| Ok((row.get(0)?, row.get(1)?, row.get(2)?)),
            ).map_err(|e| e.to_string())?;
            let data: Value = serde_json::from_str(&raw).map_err(|e| e.to_string())?;
            Ok(Some(json!({"id":id,"kind":"run","data":data,"created_at":created,"updated_at":updated})))
        }
        None => Ok(None),
    }
}

/// Pantry advisory for the generic channel, resolved through the same ledger
/// read `pantry.list` uses: unknown/retired is `NOT_FOUND`, near-expiry or
/// expired adds one advisory notice and never blocks the write.
fn pantry_notice_for(tx: &rusqlite::Transaction<'_>, item_id: Option<&str>) -> Result<Option<Value>, String> {
    use asterlab_core::pantry::{self, CalendarDate, PantryDraft, PantryItem};
    let Some(item_id) = item_id else { return Ok(None) };
    let item_id = item_id.trim();
    if item_id.is_empty() { return Ok(None) }
    let row: Option<(String, String, Option<String>, String, String, Option<String>, Option<String>, Option<i64>, Option<i64>, i64, i64)> = tx.query_row(
        "SELECT id,label_code,project_id,name,location,purchased_on,opened_on,shelf_life_days,opened_shelf_life_days,exhausted,revision FROM pantry_items WHERE id=?1",
        [item_id],
        |row| Ok((row.get(0)?, row.get(1)?, row.get(2)?, row.get(3)?, row.get(4)?, row.get(5)?, row.get(6)?, row.get(7)?, row.get(8)?, row.get(9)?, row.get(10)?)),
    ).optional().map_err(|e| e.to_string())?;
    let Some((id, label_code, project_id, name, location, purchased_on, opened_on, shelf, opened_shelf, exhausted, revision)) = row else {
        return Err(format!("NOT_FOUND: 储藏室条目不存在：{item_id}"));
    };
    // Tombstoned rows stay reserved; every read path filters them out.
    let deleted: bool = tx.query_row(
        "SELECT deleted FROM pantry_items WHERE id=?1",
        [item_id],
        |row| row.get(0),
    ).optional().map_err(|e| e.to_string())?.unwrap_or(false);
    if deleted {
        return Err(format!("NOT_FOUND: 储藏室条目不存在：{item_id}"));
    }
    let item = PantryItem {
        id, label_code: label_code.clone(), project_id, revision,
        draft: PantryDraft {
            name: name.clone(), location,
            purchased_on, opened_on,
            shelf_life_days: shelf.map(|d| d as u32),
            opened_shelf_life_days: opened_shelf.map(|d| d as u32),
            exhausted: exhausted != 0,
        },
    };
    let today = chrono::Utc::now().format("%Y-%m-%d").to_string();
    let as_of = CalendarDate::parse(&today).ok_or_else(|| "INTERNAL: 系统日期无效".to_string())?;
    let evaluation = pantry::evaluate(&item, as_of, pantry::NEAR_EXPIRY_DAYS);
    if !evaluation.needs_notice() {
        return Ok(None);
    }
    Ok(Some(json!({
        "code": "pantry_item_expiry_notice",
        "severity": "advisory",
        "pantry_item_id": item_id,
        "label_code": label_code,
        "name": name,
        "status": evaluation.status,
        "expires_on": evaluation.expires_on,
        "days_remaining": evaluation.days_remaining,
        "next_action": "核对或更换该储藏室条目；提示不阻断事件写入",
    })))
}

/// Pantry existence check through the same ledger table the CLI reads.
fn pantry_lookup(tx: &rusqlite::Transaction<'_>, item_id: &str) -> Result<Option<Value>, String> {
    let row: Option<(String,)> = tx.query_row(
        "SELECT data_json FROM pantry_items WHERE id=?1",
        [item_id],
        |row| Ok((row.get::<_, String>(0)?,)),
    ).optional().map_err(|e| e.to_string())?;
    match row {
        Some((raw,)) => {
            let data: Value = serde_json::from_str(&raw).map_err(|e| e.to_string())?;
            if data.get("deleted").and_then(Value::as_bool).unwrap_or(false) {
                return Ok(None);
            }
            Ok(Some(data))
        }
        None => Ok(None),
    }
}

/// Typed workspace loader the readout planners validate against: the same
/// frozen Run the named `readout.*` tools check scope with.
fn load_typed_workspace(db: &Connection, run_id: &str) -> Result<asterlab_core::run_workspace::RunWorkspace, String> {
    cli_impl::run_workspace_typed(db, run_id).map_err(map_command_error)
}

/// Planned readout identity: scalar `readout.submit` (numeric or `qualifier`),
/// multi-value `readout.submit` (`readouts[]` + call-scoped
/// `readout_schemas[]`), and `readout.batch` (`cells[]` or `csv`). The plan
/// carries the content-derived idempotency key, the stored
/// `operation_payload` for mismatch comparison, and the accepted count.
struct PlannedReadout {
    key: String,
    payload: Value,
    count: usize,
}

/// Derive the readout plan without writing: same content-key rule as the named
/// tools, so the generic channel and the typed entry points replay each
/// other.
fn plan_readout_request(kind: &str, request: Value, workspace: &asterlab_core::run_workspace::RunWorkspace) -> Result<PlannedReadout, String> {
    use asterlab_core::readout_records::{plan_readout_submission as plan_typed, ReadoutRegistry, ReadoutSubmission};
    use asterlab_core::readout_submit::{plan_batch, plan_submit, ReadoutBatchRequest, ReadoutSubmitRequest};
    let run_id = request.get("run_id").and_then(Value::as_str).unwrap_or("").to_owned();
    if kind == "readout.batch" {
        let parsed: ReadoutBatchRequest = serde_json::from_value(request.clone())
            .map_err(|e| format!("INVALID_INPUT: 读数批次字段无效：{e}"))?;
        let plan = plan_batch(&parsed, workspace).map_err(|issues| {
            let first = issues.first().map(|i| i.message.as_str()).unwrap_or("校验失败");
            format!("INVALID_INPUT: 读数批次校验失败：{first}")
        })?;
        return Ok(PlannedReadout { key: plan.idempotency_key.clone(), payload: plan.event.get("operation_payload").cloned().unwrap_or(Value::Null), count: plan.readout_count });
    }
    if request.get("readouts").is_some() {
        let mut registry = ReadoutRegistry::builtin();
        for schema in request.get("readout_schemas").and_then(Value::as_array).into_iter().flatten() {
            let parsed: asterlab_core::readout_schema::ReadoutSchema = serde_json::from_value(schema.clone())
                .map_err(|e| format!("INVALID_INPUT: 读数 schema 无效：{e}"))?;
            registry.insert(parsed).map_err(|issues| {
                let first = issues.first().map(|i| i.message.as_str()).unwrap_or("校验失败");
                format!("INVALID_INPUT: 读数 schema 校验失败：{first}")
            })?;
        }
        let step_path = request.get("step_path").and_then(Value::as_str).unwrap_or("").trim().to_owned();
        if step_path.is_empty() {
            return Err("INVALID_INPUT: 缺少 Protocol 步骤路径".into());
        }
        let submission: ReadoutSubmission = serde_json::from_value(json!({
            "run_id": run_id,
            "step_path": step_path,
            "batch_id": request.get("batch_id").cloned().unwrap_or(Value::Null),
            "assay_id": request.get("assay_id").cloned().unwrap_or(Value::Null),
            "timepoint": request.get("timepoint").cloned().unwrap_or(Value::Null),
            "readouts": request.get("readouts").cloned().unwrap_or(json!([])),
        })).map_err(|e| format!("INVALID_INPUT: 读数提交字段无效：{e}"))?;
        let plan = plan_typed(&registry, &submission, workspace).map_err(|issues| {
            let first = issues.first().map(|i| i.message.as_str()).unwrap_or("校验失败");
            format!("INVALID_INPUT: 读数提交校验失败：{first}")
        })?;
        let key = format!("readout:typed:{}", sha256_hex(&serde_json::to_vec(&json!([
            submission.run_id, submission.step_path, submission.assay_id,
            submission.timepoint, submission.readouts,
        ])).unwrap_or_default()));
        let payload = request.get("readouts").cloned().unwrap_or(Value::Null);
        return Ok(PlannedReadout { key, payload, count: plan.readouts.len().max(submission.readouts.len()) });
    }
    let parsed: ReadoutSubmitRequest = serde_json::from_value(request.clone())
        .map_err(|e| format!("INVALID_INPUT: 读数提交字段无效：{e}"))?;
    let plan = plan_submit(&parsed, workspace).map_err(|issues| {
        let first = issues.first().map(|i| i.message.as_str()).unwrap_or("校验失败");
        format!("INVALID_INPUT: 读数提交校验失败：{first}")
    })?;
    Ok(PlannedReadout { key: plan.idempotency_key.clone(), payload: plan.event.get("operation_payload").cloned().unwrap_or(Value::Null), count: plan.readout_count })
}

/// Hex SHA-256 used for the typed multi-value readout key (same rule as
/// `src/cli.rs::typed_readout_key`).
fn sha256_hex(bytes: &[u8]) -> String {
    use sha2::{Digest, Sha256};
    let mut hasher = Sha256::new();
    hasher.update(bytes);
    hasher.finalize().iter().map(|b| format!("{b:02x}")).collect()
}

/// Submit one scalar or multi-value `readout.submit` inside the caller's
/// transaction. Replays the content key; the envelope key only gates the
/// `run.create`-style mismatch path, never the readout identity.
fn submit_scalar_or_typed_tx(tx: &rusqlite::Transaction<'_>, run_id: &str, request: Value, _envelope_key: &str) -> Result<Value, String> {
    let workspace: asterlab_core::run_workspace::RunWorkspace =
        serde_json::from_value(cli_impl::run_workspace(tx, run_id).map_err(map_command_error)?)
            .map_err(|e| format!("INTERNAL: {e}"))?;
    let planned = plan_readout_request("readout.submit", request.clone(), &workspace)?;
    if let Some(existing) = find_readout_tx(tx, run_id, &planned.key).map_err(|e| e.to_string())? {
        if existing.get("operation_payload").cloned().unwrap_or(Value::Null) == planned.payload {
            return Ok(json!({"record": existing, "idempotency_key": planned.key, "replayed": true, "accepted": planned.count}));
        }
        return Err("IDEMPOTENCY_MISMATCH: same readout identity with different payload".into());
    }
    let event = if request.get("readouts").is_some() {
        build_typed_event(&request, &planned, &workspace)?
    } else {
        use asterlab_core::readout_submit::{plan_submit, ReadoutSubmitRequest};
        let parsed: ReadoutSubmitRequest = serde_json::from_value(request.clone())
            .map_err(|e| format!("INVALID_INPUT: 读数提交字段无效：{e}"))?;
        plan_submit(&parsed, &workspace).map_err(|issues| {
            let first = issues.first().map(|i| i.message.as_str()).unwrap_or("校验失败");
            format!("INVALID_INPUT: 读数提交校验失败：{first}")
        })?.event
    };
    let record = cli_impl::create_execution_event_tx(tx, run_id, &event).map_err(map_command_error)?;
    Ok(json!({"record": record, "idempotency_key": planned.key, "replayed": false, "accepted": planned.count}))
}

/// Submit one `readout.batch` (`cells[]` or `csv`) inside the caller's
/// transaction. Atomic: any row failure rejects the batch with
/// `INVALID_INPUT` + `issues[]` and writes nothing.
fn submit_batch_tx(tx: &rusqlite::Transaction<'_>, run_id: &str, request: Value, _envelope_key: &str) -> Result<Value, String> {
    use asterlab_core::readout_submit::{plan_batch, ReadoutBatchRequest};
    let workspace: asterlab_core::run_workspace::RunWorkspace =
        serde_json::from_value(cli_impl::run_workspace(tx, run_id).map_err(map_command_error)?)
            .map_err(|e| format!("INTERNAL: {e}"))?;
    let mut normalized = request.clone();
    if normalized.get("csv").is_none_or(Value::is_null)
        && normalized.get("cells").and_then(Value::as_array).is_some_and(|cells| {
            cells.iter().any(|c| c.get("qualifier").is_some_and(|q| !q.is_null()))
        }) {
        use asterlab_core::readout_submit::parse_csv_cells;
        let cells = normalized.get("cells").and_then(Value::as_array).cloned().unwrap_or_default();
        let header = "row,column,field,value,unit,sample_id,group,replicate,status,qualifier";
        let mut csv = String::from(header);
        for cell in &cells {
            let text = |key: &str| match cell.get(key) {
                None | Some(Value::Null) => String::new(),
                Some(Value::String(s)) => format!("\"{}\"", s.replace('"', "\"\"")),
                Some(v) => v.to_string(),
            };
            csv.push_str(&format!("\n\"{}\",\"{}\",\"{}\",\"{}\",\"{}\",\"{}\",\"{}\",\"{}\",\"{}\",\"{}\"",
                text("row"), text("column"), text("field"), text("value"), text("unit"),
                text("sample_id"), text("group"), text("replicate"), text("status"), text("qualifier")));
        }
        normalized.as_object_mut().map(|o| { o.remove("cells"); o.insert("csv".into(), json!(csv)); });
    }
    for cell in normalized.get_mut("cells").and_then(Value::as_array_mut).into_iter().flatten() {
        if let Some(o) = cell.as_object_mut() {
            if o.get("qualifier").is_some_and(Value::is_null) { o.remove("qualifier"); }
        }
    }
    let parsed: ReadoutBatchRequest = serde_json::from_value(normalized.clone())
        .map_err(|e| format!("INVALID_INPUT: 读数批次字段无效：{e}"))?;
    let planned = plan_batch(&parsed, &workspace).map_err(|issues| {
        let detail = issues.first().map(|i| format!("{}：{}", i.code, i.message)).unwrap_or_else(|| "校验失败".into());
        format!("INVALID_INPUT: 读数批次校验失败：{detail}")
    })?;
    let record = cli_impl::create_execution_event_tx(tx, run_id, &planned.event).map_err(map_command_error)?;
    Ok(json!({"record": record, "idempotency_key": planned.idempotency_key, "replayed": false, "accepted": planned.readout_count}))
}

/// Build the stored multi-value event for the generic channel: the same
/// schema pipeline the named typed tool runs, plus the frozen Protocol scope
/// the other readout tools require, with the content-derived key stored as
/// `operation_id` so both channels replay each other.
fn build_typed_event(request: &Value, planned: &PlannedReadout, workspace: &asterlab_core::run_workspace::RunWorkspace) -> Result<Value, String> {
    use asterlab_core::readout_records::{plan_readout_submission as plan_typed, ReadoutRegistry, ReadoutSubmission};
    let mut registry = ReadoutRegistry::builtin();
    for schema in request.get("readout_schemas").and_then(Value::as_array).into_iter().flatten() {
        let parsed: asterlab_core::readout_schema::ReadoutSchema = serde_json::from_value(schema.clone())
            .map_err(|e| format!("INVALID_INPUT: 读数 schema 无效：{e}"))?;
        registry.insert(parsed).map_err(|issues| {
            let first = issues.first().map(|i| i.message.as_str()).unwrap_or("校验失败");
            format!("INVALID_INPUT: 读数 schema 校验失败：{first}")
        })?;
    }
    let submission: ReadoutSubmission = serde_json::from_value(json!({
        "run_id": request.get("run_id").cloned().unwrap_or(Value::Null),
        "step_path": request.get("step_path").cloned().unwrap_or(Value::Null),
        "batch_id": request.get("batch_id").cloned().unwrap_or(Value::Null),
        "assay_id": request.get("assay_id").cloned().unwrap_or(Value::Null),
        "timepoint": request.get("timepoint").cloned().unwrap_or(Value::Null),
        "readouts": request.get("readouts").cloned().unwrap_or(json!([])),
    })).map_err(|e| format!("INVALID_INPUT: 读数提交字段无效：{e}"))?;
    let plan = plan_typed(&registry, &submission, workspace).map_err(|issues| {
        let first = issues.first().map(|i| i.message.as_str()).unwrap_or("校验失败");
        format!("INVALID_INPUT: 读数提交校验失败：{first}")
    })?;
    let protocol_version_id = request.get("protocol_version_id").and_then(Value::as_str).unwrap_or("").trim();
    if protocol_version_id.is_empty() {
        return Err("INVALID_INPUT: 关联 Protocol 步骤时必须提供 protocol_version_id".into());
    }
    let mut event = plan.event.clone();
    event["protocol_version_id"] = json!(protocol_version_id);
    if let Some(at) = request.get("occurred_at").and_then(Value::as_str).map(str::trim).filter(|v| !v.is_empty()) {
        event["occurred_at"] = json!(at);
    }
    if let Some(op) = request.get("operator").and_then(Value::as_str).map(str::trim).filter(|v| !v.is_empty()) {
        event["operator"] = json!(op);
    }
    event["operation_id"] = json!(planned.key);
    event["operation_payload"] = request.get("readouts").cloned().unwrap_or(Value::Null);
    Ok(event)
}

/// Create one Run inside the caller's transaction: same project scope as the
/// CLI `run.create` (cross-Design `source_run_id` allowed, cross-project
/// rejected), with the envelope key stored as `data.operation_id` for replay.
fn create_run_tx(tx: &rusqlite::Transaction<'_>, payload: &Value, operation_id: Option<&str>) -> Result<Value, String> {
    use rusqlite::params;
    let design_id = payload.get("design_id").and_then(Value::as_str).unwrap_or("");
    if design_id.trim().is_empty() {
        return Err("INVALID_INPUT: design_id 不能为空".into());
    }
    let title = payload.get("title").and_then(Value::as_str).unwrap_or("").to_owned();
    if title.trim().is_empty() {
        return Err("INVALID_INPUT: 运行名称不能为空".into());
    }
    let scheduled = payload.get("scheduled_date").and_then(Value::as_str).map(str::to_owned);
    let global_pv = payload.get("protocol_version_id").and_then(Value::as_str).map(str::to_owned);
    let view = payload.get("view_id").and_then(Value::as_str).map(str::to_owned);
    let repeat_of = payload.get("repeat_of_run_id").and_then(Value::as_str).map(str::to_owned);
    let relationship = payload.get("repeat_relationship").and_then(Value::as_str).unwrap_or("independent").to_owned();
    let source = payload.get("source_run_id").and_then(Value::as_str).map(str::to_owned);
    let mut record = cli_impl::create_run_with_source_in_transaction(
        tx, design_id, &title,
        scheduled.as_deref(), global_pv.as_deref(), view.as_deref(),
        repeat_of.as_deref(), &relationship, source.as_deref(), operation_id,
    ).map_err(map_command_error)?;
    // Stamp the exact envelope payload for mismatch comparison on replay.
    let id = record.get("id").and_then(Value::as_str).unwrap_or("").to_owned();
    if !id.is_empty() {
        let mut data: Value = record.get("data").cloned().unwrap_or(json!({}));
        data["operation_payload"] = payload.clone();
        let raw = serde_json::to_string(&data).map_err(|e| format!("INTERNAL: {e}"))?;
        tx.execute(
            "UPDATE records SET data_json=?1,updated_at=?2 WHERE id=?3",
            params![raw, chrono::Utc::now().to_rfc3339(), id],
        ).map_err(|e| format!("INTERNAL: {e}"))?;
        record["data"] = data;
    }
    Ok(record)
}

/// Idempotent `note.create` for the Bridge generic channel: the envelope
/// `idempotency_key` is stored as `data.operation_id` with the exact payload
/// as `data.operation_payload`. Same key + same payload replays the first
/// record; a different payload is a mismatch. The named CLI `note.create`
/// keeps its append-only shape (no envelope fields) and is untouched.
fn create_note_tx(db: &mut Connection, payload: &Value, idempotency_key: &str) -> Result<Value, String> {
    use chrono::DateTime;
    use rusqlite::params;
    use uuid::Uuid;
    let run_id = payload.get("run_id").and_then(Value::as_str).unwrap_or("");
    if run_id.trim().is_empty() {
        return Err("INVALID_INPUT: run_id 不能为空".into());
    }
    let tx = db.unchecked_transaction().map_err(|e| format!("INTERNAL: {e}"))?;
    let existing: Option<(String, String)> = tx.query_row(
        "SELECT id,data_json FROM records WHERE kind='note' AND parent_id=?1 AND json_extract(data_json,'$.operation_id')=?2 ORDER BY created_at LIMIT 1",
        params![run_id, idempotency_key],
        |row| Ok((row.get(0)?, row.get(1)?)),
    ).optional().map_err(|e| format!("INTERNAL: {e}"))?;
    if let Some((id, raw)) = existing {
        let data: Value = serde_json::from_str(&raw).map_err(|e| format!("INTERNAL: {e}"))?;
        let same = data.get("operation_payload").cloned().unwrap_or(Value::Null) == *payload;
        let (project, parent, created, updated): (Option<String>, Option<String>, String, String) = tx.query_row(
            "SELECT project_id,parent_id,created_at,updated_at FROM records WHERE id=?1",
            [&id],
            |row| Ok((row.get(0)?, row.get(1)?, row.get(2)?, row.get(3)?)),
        ).map_err(|e| format!("INTERNAL: {e}"))?;
        tx.commit().map_err(|e| format!("INTERNAL: {e}"))?;
        if same {
            let record = json!({"id":id,"kind":"note","project_id":project,"parent_id":parent,"revision":1,"data":data,"created_at":created,"updated_at":updated});
            return Ok(json!({"record": record, "replayed": true}));
        }
        return Err("IDEMPOTENCY_MISMATCH: same idempotency_key with different payload; mint a new key for a new action".into());
    }
    let content = payload.get("content").and_then(Value::as_str).unwrap_or("");
    if content.trim().is_empty() || content.chars().count() > 10000 {
        return Err("INVALID_INPUT: 记录内容不能为空且不能超过 10000 个字符".into());
    }
    if let Some(time) = payload.get("event_time").and_then(Value::as_str) {
        if DateTime::parse_from_rfc3339(time).is_err() {
            return Err("INVALID_INPUT: event_time 必须是带时区的 RFC 3339 时间".into());
        }
    }
    let (kind_name, project): (String, Option<String>) = tx.query_row(
        "SELECT kind,project_id FROM records WHERE id=?1",
        [run_id],
        |row| Ok((row.get(0)?, row.get(1)?)),
    ).optional().map_err(|e| format!("INTERNAL: {e}"))?
        .ok_or_else(|| "NOT_FOUND: 实验运行不存在".to_string())?;
    if kind_name != "run" {
        return Err("INVALID_INPUT: run_id 必须指向实验运行".into());
    }
    let locked: Option<String> = tx.query_row(
        "SELECT data_json FROM run_archive_state WHERE run_id=?1",
        [run_id],
        |row| row.get(0),
    ).optional().map_err(|e| format!("INTERNAL: {e}"))?;
    if let Some(raw) = locked {
        let state: asterlab_core::record_projection::RecordArchiveState =
            serde_json::from_str(&raw).map_err(|e| format!("INTERNAL: {e}"))?;
        if state.notes_locked() {
            return Err("CONFLICT: Run 已签核归档，记录只读".into());
        }
    }
    let id = Uuid::new_v4().to_string();
    let at = chrono::Utc::now().to_rfc3339();
    let data = json!({
        "content": content,
        "event_time": payload.get("event_time").cloned().unwrap_or(Value::Null),
        "observation": payload.get("observation").cloned().unwrap_or(Value::Null),
        "result": payload.get("result").cloned().unwrap_or(Value::Null),
        "deviation": payload.get("deviation").cloned().unwrap_or(Value::Null),
        "operation_id": idempotency_key,
        "operation_payload": payload.clone(),
    });
    tx.execute(
        "INSERT INTO records(id,kind,project_id,parent_id,revision,data_json,created_at,updated_at) VALUES(?1,'note',?2,?3,1,?4,?5,?5)",
        params![id, project, run_id, serde_json::to_string(&data).map_err(|e| format!("INTERNAL: {e}"))?, at],
    ).map_err(|e| format!("INTERNAL: {e}"))?;
    tx.commit().map_err(|e| format!("INTERNAL: {e}"))?;
    Ok(json!({"record": json!({"id":id,"kind":"note","project_id":project,"parent_id":run_id,"revision":1,"data":data,"created_at":at,"updated_at":at}), "replayed": false}))
}
/// CLI-shaped append-only simples: `note.create` (Run prose),
/// `run.revise` (title/scheduled_date only, expected_revision guarded),
/// `run.set_status` (planned/in_progress/completed/blocked/cancelled).
/// The Bridge routes `note.create` through `create_note_tx` (envelope
/// idempotency) and re-checks the protocol-ready guard before `run.set_status`;
/// this function holds the shared field validation and writes.
fn apply_simple_command(db: &mut Connection, kind: &str, payload: &Value) -> Result<Value, String> {
    use chrono::DateTime;
    use rusqlite::params;
    use uuid::Uuid;
    match kind {
        "note.create" => {
            let run_id = payload.get("run_id").and_then(Value::as_str).unwrap_or("");
            let content = payload.get("content").and_then(Value::as_str).unwrap_or("");
            if run_id.trim().is_empty() {
                return Err("INVALID_INPUT: run_id 不能为空".into());
            }
            if content.trim().is_empty() || content.chars().count() > 10000 {
                return Err("INVALID_INPUT: 记录内容不能为空且不能超过 10000 个字符".into());
            }
            let (kind_name, project, _, _, _) = db.query_row(
                "SELECT kind,project_id,parent_id,revision,data_json FROM records WHERE id=?1",
                [run_id],
                |row| Ok((row.get::<_, String>(0)?, row.get::<_, Option<String>>(1)?, row.get::<_, Option<String>>(2)?, row.get::<_, i64>(3)?, row.get::<_, String>(4)?)),
            ).optional().map_err(|e| format!("INTERNAL: {e}"))?
                .ok_or_else(|| "NOT_FOUND: 实验运行不存在".to_string())?;
            if kind_name != "run" {
                return Err("INVALID_INPUT: run_id 必须指向实验运行".into());
            }
            let locked: Option<String> = db.query_row(
                "SELECT data_json FROM run_archive_state WHERE run_id=?1",
                [run_id],
                |row| row.get(0),
            ).optional().map_err(|e| format!("INTERNAL: {e}"))?;
            if let Some(raw) = locked {
                let state: asterlab_core::record_projection::RecordArchiveState =
                    serde_json::from_str(&raw).map_err(|e| format!("INTERNAL: {e}"))?;
                if state.notes_locked() {
                    return Err("CONFLICT: Run 已签核归档，记录只读".into());
                }
            }
            if let Some(time) = payload.get("event_time").and_then(Value::as_str) {
                if DateTime::parse_from_rfc3339(time).is_err() {
                    return Err("INVALID_INPUT: event_time 必须是带时区的 RFC 3339 时间".into());
                }
            }
            let id = Uuid::new_v4().to_string();
            let at = chrono::Utc::now().to_rfc3339();
            let data = json!({
                "content": content,
                "event_time": payload.get("event_time").cloned().unwrap_or(Value::Null),
                "observation": payload.get("observation").cloned().unwrap_or(Value::Null),
                "result": payload.get("result").cloned().unwrap_or(Value::Null),
                "deviation": payload.get("deviation").cloned().unwrap_or(Value::Null),
            });
            db.execute(
                "INSERT INTO records(id,kind,project_id,parent_id,revision,data_json,created_at,updated_at) VALUES(?1,'note',?2,?3,1,?4,?5,?5)",
                params![id, project, run_id, serde_json::to_string(&data).map_err(|e| format!("INTERNAL: {e}"))?, at],
            ).map_err(|e| format!("INTERNAL: {e}"))?;
            Ok(json!({"id":id,"kind":"note","project_id":project,"parent_id":run_id,"revision":1,"data":data,"created_at":at,"updated_at":at}))
        }
        "run.revise" => {
            let id = payload.get("id").and_then(Value::as_str).unwrap_or("");
            let expected = payload.get("expected_revision").and_then(Value::as_i64).ok_or_else(|| "INVALID_INPUT: expected_revision is required".to_string())?;
            if id.trim().is_empty() {
                return Err("INVALID_INPUT: id 不能为空".into());
            }
            cli_impl::patch_run(db, id, expected, &json!({
                "title": payload.get("title").cloned().unwrap_or(Value::Null),
                "scheduled_date": payload.get("scheduled_date").cloned().unwrap_or(Value::Null),
            })).map_err(map_command_error)
        }
        "run.set_status" => {
            let id = payload.get("id").and_then(Value::as_str).unwrap_or("");
            let expected = payload.get("expected_revision").and_then(Value::as_i64).ok_or_else(|| "INVALID_INPUT: expected_revision is required".to_string())?;
            let status = payload.get("status").and_then(Value::as_str).unwrap_or("").to_owned();
            if id.trim().is_empty() || status.is_empty() {
                return Err("INVALID_INPUT: id 与 status 不能为空".into());
            }
            if !["planned","in_progress","completed","blocked","cancelled"].contains(&status.as_str()) {
                return Err("INVALID_INPUT: 运行状态必须是 planned、in_progress、completed、blocked 或 cancelled".into());
            }
            let (kind_name, _, _, revision, raw) = db.query_row(
                "SELECT kind,project_id,parent_id,revision,data_json FROM records WHERE id=?1",
                [id],
                |row| Ok((row.get::<_, String>(0)?, row.get::<_, Option<String>>(1)?, row.get::<_, Option<String>>(2)?, row.get::<_, i64>(3)?, row.get::<_, String>(4)?)),
            ).optional().map_err(|e| format!("INTERNAL: {e}"))?
                .ok_or_else(|| "NOT_FOUND: 实验运行不存在".to_string())?;
            if kind_name != "run" {
                return Err("INVALID_INPUT: id 必须指向实验运行".into());
            }
            if revision != expected {
                return Err(format!("REVISION_CONFLICT: 运行版本已从 v{expected} 更新为 v{revision}；请先重新读取。"));
            }
            let mut data: Value = serde_json::from_str(&raw).map_err(|e| format!("INTERNAL: {e}"))?;
            data["status"] = json!(status);
            let tx = db.unchecked_transaction().map_err(|e| format!("INTERNAL: {e}"))?;
            let changed = tx.execute(
                "UPDATE records SET revision=?2,data_json=?3,updated_at=?4 WHERE id=?1 AND revision=?5",
                params![id, revision + 1, serde_json::to_string(&data).map_err(|e| format!("INTERNAL: {e}"))?, chrono::Utc::now().to_rfc3339(), revision],
            ).map_err(|e| format!("INTERNAL: {e}"))?;
            if changed != 1 {
                return Err("REVISION_CONFLICT: 记录已被其他操作更新；请重新读取。".into());
            }
            tx.commit().map_err(|e| format!("INTERNAL: {e}"))?;
            let raw2: (String, String, String) = db.query_row(
                "SELECT data_json,created_at,updated_at FROM records WHERE id=?1",
                [id],
                |row| Ok((row.get(0)?, row.get(1)?, row.get(2)?)),
            ).map_err(|e| format!("INTERNAL: {e}"))?;
            Ok(json!({"id":id,"kind":"run","revision":revision+1,"data":raw2.0,"created_at":raw2.1,"updated_at":raw2.2}))
        }
        other => Err(format!("INVALID_INPUT: 未知命令：{other}")),
    }
}

impl AppCore {
    pub fn records_digest(&self) -> Result<String, String> {
        agent_edit_preview::records_digest(&self.db)
    }
}


#[cfg(test)]
mod tests {
    use super::*;
    use std::fs;

    fn fixture() -> (std::path::PathBuf, String) {
        let d = std::env::temp_dir().join(format!("asterlab-batch-{}", uuid::Uuid::new_v4()));
        fs::create_dir_all(&d).unwrap();
        let mut core = AppCore::open(&d).unwrap();
        let project = cli_impl::create_project(&core.db, "p").unwrap();
        let design = core.save_design(json!({"project_id":project["id"],"design":{"title":"d"}})).unwrap();
        let run = core.create_run(design["design"]["id"].as_str().unwrap(), None, None).unwrap();
        (d, run["id"].as_str().unwrap().to_owned())
    }

    fn batch(run_id: &str, op: &str, expected_revision: u64) -> Value {
        let cells: Vec<Value> = (0..8).flat_map(|r| (0..12).map(move |c| json!({
            "row":r,"column":c,"sample_id":"","group":"control","replicate":1,
            "field":"od450","value":(r*12+c) as f64,"status":"raw"
        }))).collect();
        json!({"run_id":run_id,"operation_id":op,"schema_id":"cck8.plate96.od450.v1",
            "schema_version":1,"expected_revision":expected_revision,"cells":cells})
    }
    #[test]
    fn batch_is_atomic_idempotent_and_preserves_missing_cells() {
        let (dir, run_id) = fixture();
        let mut core = AppCore::open(&dir).unwrap();
        let input = batch(&run_id, "batch-op", 0);
        let result = core.save_readouts_batch(input.clone()).unwrap();
        assert_eq!(result["accepted"], 96);
        let revision = result["revision"].as_u64().unwrap();
        let event_id = result["event_id"].as_str().unwrap().to_owned();
        assert_eq!(core.save_readouts_batch(input.clone()).unwrap()["event_id"], event_id);
        let first = core.load_run(&run_id).unwrap();
        assert_eq!(first["revision"], revision);
        let first_event = first["events"].as_array().unwrap().iter().find(|e| e["event_id"] == event_id).unwrap();
        assert_eq!(first_event["readouts"].as_array().unwrap().len(), 96);
        assert!(first_event["readouts"][0]["readout_id"].as_str().is_some_and(|id| !id.is_empty()));
        let mut changed = input.clone();
        changed["cells"][0]["value"] = json!(999.0);
        assert!(core.save_readouts_batch(changed).unwrap_err().contains("conflict"));
        let mut invalid = batch(&run_id, "bad-op", revision);
        invalid["cells"][95]["value"] = Value::Null;
        assert!(core.save_readouts_batch(invalid).is_err());
        assert_eq!(core.load_run(&run_id).unwrap()["revision"], revision);
        let mut missing = batch(&run_id, "missing-op", revision);
        missing["cells"][0]["value"] = Value::Null;
        missing["cells"][0]["status"] = json!("missing");
        let missing_result = core.save_readouts_batch(missing).unwrap();
        let snapshot = core.load_run(&run_id).unwrap();
        let event = snapshot["events"].as_array().unwrap().iter().find(|e|e["event_id"] == missing_result["event_id"]).unwrap();
        assert_eq!(event["result_cells"][0]["status"], "missing");
        let _ = fs::remove_dir_all(dir);
    }

    #[test]
    fn concurrent_batch_writers_use_authoritative_revision() {
        let (dir, run_id) = fixture();
        let mut threads = Vec::new();
        for n in 0..2 {
            let dir = dir.clone();
            let run_id = run_id.clone();
            threads.push(std::thread::spawn(move || {
                let mut core = AppCore::open(&dir).unwrap();
                core.save_readouts_batch(batch(&run_id, &format!("parallel-{n}"), 0))
            }));
        }
        let results: Vec<_> = threads.into_iter().map(|t|t.join().unwrap()).collect();
        assert_eq!(results.iter().filter(|r|r.is_ok()).count(), 1);
        assert_eq!(results.iter().filter(|r|r.as_ref().is_err_and(|e|e.contains("stale revision"))).count(), 1);
        let core = AppCore::open(&dir).unwrap();
        assert_eq!(core.load_run(&run_id).unwrap()["revision"], 1);
        let _ = fs::remove_dir_all(dir);
    }
    #[test]
    fn idempotent_readout_and_rollback() {
        let d = std::env::temp_dir().join(format!("asterlab-app-test-{}", uuid::Uuid::new_v4()));
        fs::create_dir_all(&d).unwrap();
        let mut core = AppCore::open(&d).unwrap();
        let proj = cli_impl::create_project(&core.db, "p").unwrap();
        let pid = proj["id"].as_str().unwrap();
        let design = core.save_design(serde_json::json!({"project_id": pid, "design": {"title": "d"}})).unwrap();
        let did = design["design"]["id"].as_str().unwrap();
        let run = core.create_run(did, None, None).unwrap();
        let rid = run["id"].as_str().unwrap();
        let rev0 = core.load_run(rid).unwrap()["revision"].as_i64().unwrap();

        let input = serde_json::json!({"run_id": rid, "operation_id": "op1", "measurements": [{"endpoint":"e","value":1.0}]});
        let r1 = core.save_readout(input.clone()).unwrap();
        let rev1 = r1["revision"].as_i64().unwrap();
        assert!(rev1 > rev0);

        // same op+payload returns existing, no new event
        let r2 = core.save_readout(input.clone()).unwrap();
        assert_eq!(r2["revision"].as_i64().unwrap(), rev1);

        // different payload conflict
        let bad = serde_json::json!({"run_id": rid, "operation_id": "op1", "measurements": [{"endpoint":"e","value":2.0}]});
        assert!(core.save_readout(bad).is_err());

        // invalid readout rolls back: event count unchanged
        let before = core.load_run(rid).unwrap()["revision"].as_i64().unwrap();
        let invalid = serde_json::json!({"run_id": rid, "measurements": [{"endpoint":"e","value": f64::NAN}]});
        assert!(core.save_readout(invalid).is_err());
        let after = core.load_run(rid).unwrap()["revision"].as_i64().unwrap();
        assert_eq!(before, after);

        let _ = fs::remove_dir_all(&d);
    }
    #[test]
    fn readout_persists_across_reopen() {
        let d = std::env::temp_dir().join(format!("asterlab-reopen-{}", uuid::Uuid::new_v4()));
        std::fs::create_dir_all(&d).unwrap();
        std::env::set_var("LABWORKBENCH_DATA_DIR", &d);
        let resolved = cli_impl::data_dir().unwrap();
        assert_eq!(resolved, d);
        let rid = {
            let mut core = AppCore::open(&resolved).unwrap();
            assert_eq!(core.db.query_row("PRAGMA journal_mode", [], |row| row.get::<_, String>(0)).unwrap(), "wal");
            let proj = cli_impl::create_project(&core.db, "p").unwrap();
            let pid = proj["id"].as_str().unwrap();
            let design = core.save_design(serde_json::json!({"project_id": pid, "design": {"title": "d", "mygo": {"workflow": {"id":"wf"}, "instances": {}}}})).unwrap();
            let did = design["design"]["id"].as_str().unwrap();
            assert_eq!(core.load_design(did).unwrap()["design"]["data"]["mygo"]["workflow"]["id"], "wf");
            let run = core.create_run(did, None, None).unwrap();
            let rid = run["id"].as_str().unwrap().to_owned();
            let input = serde_json::json!({"run_id": rid, "operation_id": "op1", "measurements": [{"endpoint":"e","value":1.0}]});
            core.save_readout(input).unwrap();
            rid
        };
        let core2 = AppCore::open(&cli_impl::data_dir().unwrap()).unwrap();
        let v = core2.load_run(&rid).unwrap();
        assert!(v["revision"].as_i64().unwrap() > 0);
        let _ = std::fs::remove_dir_all(&d);
        std::env::remove_var("LABWORKBENCH_DATA_DIR");
    }
    #[test]
    fn freeze_timer_phases_persist_and_advance_through_the_supplied_plan() {
        use asterlab_core::timer::{TimerMode, TimerRecord, TimerSequence, TimerStatus};
        let (dir, run_id) = fixture();
        // Cold-hot alternation plan: 5min x3 then 10min x3. The plan stays
        // with the caller; the durable row keeps only the cursor.
        let plan = TimerSequence::new(vec![
            asterlab_core::timer::TimerPhase { name: "cold".into(), planned_s: Some(300), mode: TimerMode::Countdown, rounds: Some(3) },
            asterlab_core::timer::TimerPhase { name: "hot".into(), planned_s: Some(600), mode: TimerMode::Countdown, rounds: Some(3) },
        ])
        .unwrap();
        let timer = TimerRecord {
            timer_id: "freeze-alt".into(), run_id: run_id.clone(), task_key: "incubate".into(),
            phase_index: 0, mode: TimerMode::Countdown, planned_s: Some(300),
            started_at_utc: Some("2026-11-17T06:00:00Z".into()), accumulated_s: 0, status: TimerStatus::Running,
            round_index: 0, round_count: None, anchor_event_id: None, anchor_task_key: None,
        };
        let mut app = AppCore::open(&dir).unwrap();
        app.save_timer_record(&timer).unwrap();
        drop(app);
        // Restart: the row survives reopen with the same cursor.
        let app = AppCore::open(&dir).unwrap();
        let mut restored = app.load_timer_records(&run_id).unwrap().into_iter().next().unwrap();
        assert_eq!((restored.phase_index, restored.round_index), (0, 0));
        // Rounds 1-2 stay in the cold phase; each completed round is a fact
        // the caller appends through execution_event.create.
        let first = restored.advance_countdown(&plan, "2026-11-17T06:05:00Z").unwrap().unwrap();
        assert_eq!((first.phase_name.as_str(), first.timer_completed), ("cold", false));
        assert!(!first.phase_completed);
        let second = restored.advance_countdown(&plan, "2026-11-17T06:10:00Z").unwrap().unwrap();
        assert!(!second.phase_completed);
        let third = restored.advance_countdown(&plan, "2026-11-17T06:15:00Z").unwrap().unwrap();
        assert!(third.phase_completed);
        assert!(!third.timer_completed);
        // Crossing into the hot phase moves the durable mode/plan cursor.
        assert_eq!((restored.phase_index, restored.round_index), (1, 0));
        assert_eq!((restored.mode, restored.planned_s), (TimerMode::Countdown, Some(600)));
        let _ = fs::remove_dir_all(dir);
    }

    #[test]
    fn timer_records_persist_across_app_reopen_and_t0_triggers_window() {
        let (dir, run_id) = fixture();
        let timer = asterlab_core::timer::TimerRecord {
            timer_id: "window-6h".into(), run_id: run_id.clone(), task_key: "readout".into(),
            phase_index: 1, mode: asterlab_core::timer::TimerMode::Window, planned_s: Some(21_600),
            started_at_utc: None, accumulated_s: 0, status: asterlab_core::timer::TimerStatus::Ready,
            round_index: 0, round_count: None, anchor_event_id: None,
            anchor_task_key: Some("t0-task".into()),
        };
        {
            let mut app = AppCore::open(&dir).unwrap();
            app.save_timer_record(&timer).unwrap();
            app.save_readout(json!({
                "run_id":run_id,
                "task_key":"t0-task",
                "event_type":"measurement",
                "content":"t0 readout",
                "measurements":[{"endpoint":"od450","value":1.0}]
            })).unwrap();
        }
        let app = AppCore::open(&dir).unwrap();
        let restored = app.load_timer_records(&run_id).unwrap();
        assert_eq!(restored.len(), 1);
        assert!(restored[0].started_at_utc.is_some());
        assert_eq!(restored[0].status, asterlab_core::timer::TimerStatus::Running);
        let _ = fs::remove_dir_all(dir);
    }
    #[test]
    fn pantry_ledger_persists_across_reopen_and_keeps_label_codes_stable() {
        let dir = std::env::temp_dir().join(format!("asterlab-pantry-{}", uuid::Uuid::new_v4()));
        fs::create_dir_all(&dir).unwrap();
        let (item_id, label_code) = {
            let mut core = AppCore::open(&dir).unwrap();
            let created = core
                .pantry_upsert(json!({
                    "name":"Trypsin-EDTA",
                    "location":"4C door",
                    "purchased_on":"2026-01-01",
                    "opened_on":"2026-06-01",
                    "shelf_life_days":365,
                    "opened_shelf_life_days":30
                }))
                .unwrap();
            assert_eq!(created["item"]["expires_on"], "2026-07-01");
            assert_eq!(created["item"]["revision"], 1);
            (
                created["item"]["id"].as_str().unwrap().to_owned(),
                created["item"]["label_code"].as_str().unwrap().to_owned(),
            )
        };
        assert!(label_code.starts_with("PN-"));

        let mut core = AppCore::open(&dir).unwrap();
        let listed = core.pantry_list(json!({"as_of":"2026-06-25"})).unwrap();
        assert_eq!(listed["items"].as_array().unwrap().len(), 1);
        assert_eq!(listed["items"][0]["status"], "near_expiry");
        assert_eq!(listed["items"][0]["days_remaining"], 6);
        assert_eq!(listed["items"][0]["label_code"], label_code);

        let updated = core
            .pantry_upsert(json!({
                "id":item_id,
                "name":"Trypsin-EDTA",
                "exhausted":true,
                "expected_revision":1
            }))
            .unwrap();
        assert_eq!(updated["item"]["revision"], 2);
        assert_eq!(updated["item"]["status"], "exhausted");
        assert_eq!(updated["item"]["label_code"], label_code, "label code is not rederived");

        let with_exhausted = core.pantry_export_labels(json!({"as_of":"2026-06-25"})).unwrap();
        assert_eq!(with_exhausted["schema_version"], 1);
        assert_eq!(with_exhausted["labels"][0]["label_code"], label_code);
        assert_eq!(with_exhausted["labels"][0]["pantry_item_id"], item_id);
        let without = core
            .pantry_export_labels(json!({"as_of":"2026-06-25","include_exhausted":false}))
            .unwrap();
        assert!(without["labels"].as_array().unwrap().is_empty());

        let retired = core
            .pantry_delete(json!({"id":item_id,"expected_revision":2}))
            .unwrap();
        assert_eq!(retired["deleted"], true);
        assert_eq!(retired["label_code"], label_code);
        assert_eq!(retired["revision"], 3);
        drop(core);

        let mut core = AppCore::open(&dir).unwrap();
        assert!(core
            .pantry_list(json!({"as_of":"2026-06-25"}))
            .unwrap()["items"]
            .as_array()
            .unwrap()
            .is_empty());
        assert!(core
            .pantry_upsert(json!({"id":item_id,"name":"Trypsin-EDTA"}))
            .is_err());
        let _ = fs::remove_dir_all(dir);
    }
    #[test]
    fn generic_channel_replays_execution_event_and_maps_errors() {
        let (dir, run_id) = fixture();
        let mut core = AppCore::open(&dir).unwrap();
        let payload = json!({"run_id": run_id, "event_type": "observation", "content": "generic channel"});
        let first = core.execute_command("execution_event.create", payload.clone(), "generic-1").unwrap();
        assert_eq!(first["replayed"], false);
        let replay = core.execute_command("execution_event.create", payload.clone(), "generic-1").unwrap();
        assert_eq!(replay["replayed"], true);
        assert_eq!(replay["record"]["id"], first["record"]["id"]);
        let mismatch = core.execute_command("execution_event.create", json!({"run_id": run_id, "event_type": "observation", "content": "different"}), "generic-1").unwrap_err();
        assert!(mismatch.starts_with("IDEMPOTENCY_MISMATCH"), "unexpected: {mismatch}");
        assert!(core.execute_command("nope.create", payload.clone(), "generic-2").unwrap_err().starts_with("INVALID_INPUT"));
        assert!(core.execute_command("execution_event.create", payload.clone(), "").unwrap_err().starts_with("INVALID_INPUT"));
        assert!(core.query_projection("nope.workspace", json!({})).unwrap_err().starts_with("INVALID_INPUT"));
        assert!(core.query_projection("run.workspace", json!({"id": "missing"})).unwrap_err().starts_with("NOT_FOUND"));
        let _ = fs::remove_dir_all(dir);
    }

    #[test]
    fn generic_channel_reads_workspace_and_progress() {
        let (dir, run_id) = fixture();
        let mut core = AppCore::open(&dir).unwrap();
        let workspace = core.query_projection("run.workspace", json!({"id": run_id})).unwrap();
        assert_eq!(workspace["design_id"].as_str().is_some(), true);
        let progress = core.query_projection("workflow.progress", json!({"run_id": run_id})).unwrap();
        assert_eq!(progress["run_id"], run_id);
        let tree = core.query_projection("workflow.inspect_tree", json!({"run_id": run_id})).unwrap();
        assert!(tree.get("roots").is_some() || tree.get("issues").is_some());
        let _ = fs::remove_dir_all(dir);
    }
    fn run_record_revision(core: &AppCore, run_id: &str) -> i64 {
        core.db.query_row(
            "SELECT revision FROM records WHERE id=?1",
            [run_id],
            |row| row.get(0),
        ).unwrap()
    }

    fn note_count(core: &AppCore, run_id: &str) -> i64 {
        core.db.query_row(
            "SELECT COUNT(*) FROM records WHERE kind='note' AND parent_id=?1",
            [run_id],
            |row| row.get::<_, i64>(0),
        ).unwrap()
    }

    fn mark_run_protocol_unready(core: &AppCore, run_id: &str) {
        let raw: String = core.db.query_row(
            "SELECT data_json FROM records WHERE id=?1",
            [run_id],
            |row| row.get(0),
        ).unwrap();
        let mut data: Value = serde_json::from_str(&raw).unwrap();
        data["protocol_manifest"] = json!([{
            "protocol_ready": false,
            "unresolved_parameters": [{"path": "root/dose", "required": true}],
        }]);
        core.db.execute(
            "UPDATE records SET data_json=?1 WHERE id=?2",
            rusqlite::params![serde_json::to_string(&data).unwrap(), run_id],
        ).unwrap();
    }

    #[test]
    fn bridge_set_status_reuses_protocol_ready_guard() {
        let (dir, run_id) = fixture();
        let core = AppCore::open(&dir).unwrap();
        mark_run_protocol_unready(&core, &run_id);
        let revision = run_record_revision(&core, &run_id);
        drop(core);
        let mut core = AppCore::open(&dir).unwrap();
        let err = core.execute_command(
            "run.set_status",
            json!({"id": run_id, "expected_revision": revision, "status": "in_progress"}),
            "status-guard-1",
        ).unwrap_err();
        assert!(err.contains("Protocol"), "bridge skipped protocol-ready guard: {err}");
        assert_eq!(run_record_revision(&core, &run_id), revision);
        let _ = fs::remove_dir_all(dir);
    }

    #[test]
    fn bridge_note_create_is_idempotent() {
        let (dir, run_id) = fixture();
        let mut core = AppCore::open(&dir).unwrap();
        let payload = json!({"run_id": run_id, "content": "bridge note"});
        let first = core.execute_command("note.create", payload.clone(), "note-key-1").unwrap();
        assert_eq!(first["replayed"], false);
        assert_eq!(note_count(&core, &run_id), 1);
        let replay = core.execute_command("note.create", payload.clone(), "note-key-1").unwrap();
        assert_eq!(replay["replayed"], true);
        assert_eq!(replay["record"]["id"], first["record"]["id"]);
        assert_eq!(note_count(&core, &run_id), 1);
        let mismatch = core.execute_command(
            "note.create",
            json!({"run_id": run_id, "content": "different content"}),
            "note-key-1",
        ).unwrap_err();
        assert!(mismatch.starts_with("IDEMPOTENCY_MISMATCH"), "unexpected: {mismatch}");
        assert_eq!(note_count(&core, &run_id), 1);
        drop(core);
        let mut reopened = AppCore::open(&dir).unwrap();
        let after_reopen = reopened.execute_command("note.create", payload.clone(), "note-key-1").unwrap();
        assert_eq!(after_reopen["replayed"], true);
        assert_eq!(after_reopen["record"]["id"], first["record"]["id"]);
        assert_eq!(note_count(&reopened, &run_id), 1);
        let _ = fs::remove_dir_all(dir);
    }
    #[test]
    fn bridge_archive_state_persists_and_locks_all_write_entries() {
        let (dir, run_id) = fixture();
        let mut core = AppCore::open(&dir).unwrap();
        // Recording by default.
        let state = core
            .query_projection("record.archive_state", json!({"run_id": run_id}))
            .unwrap();
        assert_eq!(state["status"], "recording");
        // The shared CLI helper signs an empty run; Core derives the rest.
        let signed = core
            .execute_command(
                "record.sign_archive",
                json!({
                    "run_id": run_id,
                    "signed_by": "operator",
                    "signed_at_utc": "2026-11-15T10:00:00Z",
                }),
                "archive-key-1",
            )
            .unwrap();
        assert_eq!(signed["record"]["status"], "archived");
        let projection = core
            .query_projection("record.projection", json!({"run_id": run_id}))
            .unwrap();
        assert_eq!(projection["archive"]["status"], "archived");
        assert_eq!(projection["notes_locked"], true);
        // Restart keeps the lock: this is the persisted row, not memory.
        drop(core);
        let mut reopened = AppCore::open(&dir).unwrap();
        let state = reopened
            .query_projection("record.archive_state", json!({"run_id": run_id}))
            .unwrap();
        assert_eq!(state["status"], "archived");
        // Every write entry shares the guard: notes, events, revisions, timers.
        let note = reopened
            .execute_command(
                "note.create",
                json!({"run_id": run_id, "content": "late note"}),
                "note-after-archive",
            )
            .unwrap_err();
        assert!(note.starts_with("CONFLICT") || note.starts_with("REVISION_CONFLICT"), "unexpected: {note}");
        let event = reopened
            .execute_command(
                "execution_event.create",
                json!({"run_id": run_id, "event_type": "observation", "content": "late"}),
                "event-after-archive",
            )
            .unwrap_err();
        assert!(event.starts_with("CONFLICT") || event.starts_with("REVISION_CONFLICT"), "unexpected: {event}");
        let _ = fs::remove_dir_all(dir);
    }
}
