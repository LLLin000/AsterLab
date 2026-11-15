#![recursion_limit = "256"]
mod mygo;
use mygo::*;
use std::path::PathBuf;
use std::sync::{Arc, Mutex};

#[path = "../../../../../../src/cli.rs"]
mod cli_impl;

use asterlab_app::AppCore;
use asterlab_core::assets::{AssetLink as CoreAssetLink, AssetRef};
use asterlab_core::calculators::{runtime_normalize, RuntimeNormalizeInput as CoreNormalizeInput};
use asterlab_core::design_commands::{derive_design_from_run, DeriveDesignCommand};
use asterlab_core::operation_spec::OperationSpec;
use asterlab_core::protocol_preview::project_time_budget as core_time_budget;
use asterlab_core::record_projection::{
    build_record_projection, sign_and_archive, summarize_record, ArchiveSignature, ProjectionView,
    RecordArchiveState, RecordCommandError, RecordNote, RecordProjectionInput as CoreProjectionInput,
    RecordStatus,
};
use asterlab_core::timer::{TimerMode, TimerRecord as CoreTimerRecord, TimerStatus};
use rusqlite::OptionalExtension;

pub fn make_run_service() -> Result<Arc<dyn RunService>, Error> {
    let dir = cli_impl::data_dir().map_err(map_cli)?;
    let core = AppCore::open(&dir).map_err(|e| Error::Go(format!("INTERNAL: {e}")))?;
    ensure_module_tables(core.raw_db())?;
    Ok(Arc::new(CoreRunService { core: Mutex::new(Some(core)) }))
}

pub fn make_design_service() -> Result<Arc<dyn DesignService>, Error> {
    let dir = cli_impl::data_dir().map_err(map_cli)?;
    let db = cli_impl::open_db(&dir).map_err(|e| map_cli(e))?;
    ensure_module_tables(&db)?;
    Ok(Arc::new(CoreDesignService { db: Mutex::new(Some(db)) }))
}

/// The budget projection is Core's pure function: no data directory is opened,
/// nothing is locked and nothing is stored.
pub fn make_time_budget_service() -> Result<Arc<dyn TimeBudgetService>, Error> {
    Ok(Arc::new(CoreTimeBudgetService))
}

struct CoreTimeBudgetService;

impl TimeBudgetService for CoreTimeBudgetService {
    fn project_time_budget(&self, _ctx: &Context, input: ProjectTimeBudgetInput) -> Result<TimeBudget, Error> {
        project_time_budget_json(&input.specs_json)
    }
}

/// Creates the two tables this module owns on the shared database. `timer_records`
/// is the same schema `asterlab-app` creates; `run_archive_state` persists the
/// terminal record lifecycle, which Core exposes as a pure transition but does
/// not store. Creating them is idempotent, so either service may open first.
fn ensure_module_tables(db: &rusqlite::Connection) -> Result<(), Error> {
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
    )
    .map_err(|e| Error::Go(format!("INTERNAL: {e}")))
}

struct CoreRunService { core: Mutex<Option<AppCore>> }

impl RunService for CoreRunService {
    fn list_runs(&self, _ctx: &Context) -> Result<Vec<RunSummary>, Error> {
        let mut guard = self.core.lock().map_err(|_| Error::Go("INTERNAL: core lock poisoned".into()))?;
        let core = guard.as_mut().ok_or_else(|| Error::Go("INTERNAL: closed".into()))?;
        let db = core.raw_db();
        let mut stmt = db.prepare("SELECT id,project_id,data_json,updated_at FROM records WHERE kind='run' ORDER BY updated_at DESC, rowid DESC LIMIT 50")
            .map_err(|e| Error::Go(format!("INTERNAL: {e}")))?;
        let rows = stmt.query_map([], |row| Ok((row.get::<_,String>(0)?, row.get::<_,Option<String>>(1)?, row.get::<_,String>(2)?, row.get::<_,String>(3)?)))
            .map_err(|e| Error::Go(format!("INTERNAL: {e}")))?;
        let mut out = Vec::new();
        for row in rows.flatten() {
            let (id, _project, raw, updated) = row;
            let data: serde_json::Value = serde_json::from_str(&raw).unwrap_or(serde_json::json!({}));
            out.push(RunSummary {
                run_id: id,
                title: data["title"].as_str().unwrap_or("").into(),
                status: data["status"].as_str().unwrap_or("").into(),
                design_id: data["design_id"].as_str().unwrap_or("").into(),
                updated_at: updated,
            });
        }
        Ok(out)
    }

    fn load_run(&self, _ctx: &Context, run_id: String) -> Result<RunSnapshot, Error> {
        let mut guard = self.core.lock().map_err(|_| Error::Go("INTERNAL: core lock poisoned".into()))?;
        let core = guard.as_mut().ok_or_else(|| Error::Go("INTERNAL: closed".into()))?;
        let v = core.load_run(&run_id).map_err(|e| Error::Go(format!("INTERNAL: {e}")))?;
        Ok(snapshot(v))
    }

    fn save_readouts_batch(&self, _ctx: &Context, input: BatchReadoutInput) -> Result<BatchReadoutResult, Error> {
        let mut guard = self.core.lock().map_err(|_| Error::Go("INTERNAL: core lock poisoned".into()))?;
        let core = guard.as_mut().ok_or_else(|| Error::Go("INTERNAL: closed".into()))?;
        let cells: Vec<serde_json::Value> = input.cells.iter().map(|c| serde_json::json!({
            "row":c.row,"column":c.column,"sample_id":c.sample_id,"group":c.group,
            "replicate":c.replicate,"field":c.field,"value":c.value,"status":c.status
        })).collect();
        let mut request = serde_json::json!({
            "run_id":input.run_id,"operation_id":input.operation_id,"task_key":input.task_key,
            "schema_id":input.schema_id,"schema_version":input.schema_version,
            "expected_revision":input.expected_revision,"cells":cells
        });
        if !input.assay_id.is_empty() { request["assay_id"] = serde_json::json!(input.assay_id); }
        if !input.timepoint.is_empty() { request["timepoint"] = serde_json::json!(input.timepoint); }
        if !input.protocol_step_path.is_empty() { request["protocol_step_path"] = serde_json::json!(input.protocol_step_path); }
        if !input.protocol_version_id.is_empty() { request["protocol_version_id"] = serde_json::json!(input.protocol_version_id); }
        if !input.plate_id.is_empty() { request["plate_id"] = serde_json::json!(input.plate_id); }
        let result = core.save_readouts_batch(request).map_err(|e| Error::Go(format!("INTERNAL: {e}")))?;
        Ok(BatchReadoutResult {
            run_id: result["run_id"].as_str().unwrap_or_default().to_owned(),
            revision: result["revision"].as_u64().unwrap_or_default(),
            batch_id: result["batch_id"].as_str().unwrap_or_default().to_owned(),
            event_id: result["event_id"].as_str().unwrap_or_default().to_owned(),
            accepted: result["accepted"].as_i64().unwrap_or_default(),
            rejected: vec![],
        })
    }

    fn save_readout(&self, _ctx: &Context, input: ReadoutInput) -> Result<RunSnapshot, Error> {
        let mut guard = self.core.lock().map_err(|_| Error::Go("INTERNAL: core lock poisoned".into()))?;
        let core = guard.as_mut().ok_or_else(|| Error::Go("INTERNAL: closed".into()))?;
        let run_id = input.run_id.clone();
        let measurements: Vec<serde_json::Value> = input.measurements.iter().map(|m| serde_json::json!({
            "endpoint": m.endpoint, "value": m.value, "unit": m.unit, "sample_id": m.sample_id
        })).collect();
        let mut event = serde_json::json!({
            "run_id": run_id,
            "event_type": "measurement",
            "content": "结构化记录",
            "measurements": measurements
        });
        if !input.task_key.is_empty() { event["task_key"] = serde_json::json!(input.task_key); }
        if !input.timepoint.is_empty() { event["timepoint"] = serde_json::json!(input.timepoint); }
        if !input.assay_id.is_empty() { event["assay_id"] = serde_json::json!(input.assay_id); }
        if !input.protocol_step_path.is_empty() { event["protocol_step_path"] = serde_json::json!(input.protocol_step_path); }
        if !input.protocol_version_id.is_empty() { event["protocol_version_id"] = serde_json::json!(input.protocol_version_id); }
        if !input.operation_id.is_empty() { event["operation_id"] = serde_json::json!(input.operation_id); }
        let v = core.save_readout(event).map_err(|e| Error::Go(format!("INTERNAL: {e}")))?;
        Ok(snapshot(v))
    }

    fn save_notebook(&self, _ctx: &Context, input: NotebookInput) -> Result<RunSnapshot, Error> {
        let mut guard = self.core.lock().map_err(|_| Error::Go("INTERNAL: core lock poisoned".into()))?;
        let core = guard.as_mut().ok_or_else(|| Error::Go("INTERNAL: closed".into()))?;
        let event = serde_json::json!({
            "run_id": input.run_id,
            "event_type": "observation",
            "stage": "notebook",
            "content": input.markdown,
            "operation_id": input.operation_id
        });
        let v = core.save_readout(event).map_err(|e| Error::Go(format!("INTERNAL: {e}")))?;
        Ok(snapshot(v))
    }

    fn save_task_checks(&self, _ctx: &Context, input: TaskCheckInput) -> Result<RunSnapshot, Error> {
        let mut guard = self.core.lock().map_err(|_| Error::Go("INTERNAL: core lock poisoned".into()))?;
        let core = guard.as_mut().ok_or_else(|| Error::Go("INTERNAL: closed".into()))?;
        let checks: Vec<serde_json::Value> = input.checks.iter().map(|check| serde_json::json!({
            "check_id": check.check_id, "status": check.status
        })).collect();
        let mut event = serde_json::json!({
            "run_id": input.run_id,
            "event_type": "operation",
            "content": "核对执行检查项",
            "task_key": input.task_key,
            "operation_id": input.operation_id,
            "check_ack": true,
            "checks": checks
        });
        if !input.assay_id.is_empty() { event["assay_id"] = serde_json::json!(input.assay_id); }
        if !input.timepoint.is_empty() { event["timepoint"] = serde_json::json!(input.timepoint); }
        if !input.protocol_step_path.is_empty() { event["protocol_step_path"] = serde_json::json!(input.protocol_step_path); }
        if !input.protocol_version_id.is_empty() { event["protocol_version_id"] = serde_json::json!(input.protocol_version_id); }
        if input.checks.is_empty() { event["checks"] = serde_json::json!([]); }
        let v = core.save_readout(event).map_err(|e| Error::Go(format!("INTERNAL: {e}")))?;
        Ok(snapshot(v))
    }

    fn complete_task(&self, _ctx: &Context, input: CompleteTaskInput) -> Result<RunSnapshot, Error> {
        let mut guard = self.core.lock().map_err(|_| Error::Go("INTERNAL: core lock poisoned".into()))?;
        let core = guard.as_mut().ok_or_else(|| Error::Go("INTERNAL: closed".into()))?;
        let checks: Vec<serde_json::Value> = input.checks.iter().map(|check| serde_json::json!({
            "check_id": check.check_id, "status": check.status
        })).collect();
        let mut event = serde_json::json!({
            "run_id": input.run_id,
            "event_type": "operation",
            "content": "完成任务",
            "task_key": input.task_key,
            "step_status": "completed",
            "operation_id": input.operation_id,
            "check_ack": true,
            "checks": checks
        });
        if !input.assay_id.is_empty() { event["assay_id"] = serde_json::json!(input.assay_id); }
        if !input.timepoint.is_empty() { event["timepoint"] = serde_json::json!(input.timepoint); }
        if !input.protocol_step_path.is_empty() { event["protocol_step_path"] = serde_json::json!(input.protocol_step_path); }
        if !input.protocol_version_id.is_empty() { event["protocol_version_id"] = serde_json::json!(input.protocol_version_id); }
        if input.checks.is_empty() { event["checks"] = serde_json::json!([]); }
        let v = core.save_readout(event).map_err(|e| Error::Go(format!("INTERNAL: {e}")))?;
        Ok(snapshot(v))
    }

    fn save_asset_link(&self, _ctx: &Context, input: AssetLinkInput) -> Result<Asset, Error> {
        let mut guard = self.core.lock().map_err(|_| Error::Go("INTERNAL: core lock poisoned".into()))?;
        let core = guard.as_mut().ok_or_else(|| Error::Go("INTERNAL: closed".into()))?;
        let asset_json = serde_json::json!({
            "asset": {
                "id": input.asset.id,
                "rootId": input.asset.root_id,
                "relativePath": input.asset.relative_path,
                "mediaType": input.asset.media_type,
                "size": input.asset.size,
                "modTime": input.asset.mod_time,
                "sha256": input.asset.sha256,
                "availability": input.asset.availability,
                "link": {
                    "runId": input.asset.link.run_id,
                    "taskKey": input.asset.link.task_key,
                    "sampleId": if input.asset.link.sample_id.is_empty() { serde_json::Value::Null } else { serde_json::json!(input.asset.link.sample_id) }
                },
                "linkedAt": input.asset.linked_at,
                "actorId": input.asset.actor_id
            }
        });
        let result = core.save_asset_link(asset_json).map_err(|e| Error::Go(format!("INTERNAL: {e}")))?;
        Ok(asset_from_value(&result))
    }

    fn list_asset_links(&self, _ctx: &Context, run_id: String) -> Result<Vec<Asset>, Error> {
        let mut guard = self.core.lock().map_err(|_| Error::Go("INTERNAL: core lock poisoned".into()))?;
        let core = guard.as_mut().ok_or_else(|| Error::Go("INTERNAL: closed".into()))?;
        let v = core.list_asset_links(&run_id).map_err(|e| Error::Go(format!("INTERNAL: {e}")))?;
        let arr = v.as_array().cloned().unwrap_or_default();
        Ok(arr.iter().map(asset_from_value).collect())
    }

    fn workflow_tree(&self, _ctx: &Context, run_id: String) -> Result<String, Error> {
        let mut guard = self.core.lock().map_err(|_| Error::Go("INTERNAL: core lock poisoned".into()))?;
        let core = guard.as_mut().ok_or_else(|| Error::Go("INTERNAL: closed".into()))?;
        let value = cli_impl::workflow_tree_for_db(core.raw_db(), None, None, Some(&run_id))
            .map_err(map_cli)?;
        Ok(value.to_string())
    }

    fn save_timer(&self, _ctx: &Context, timer: TimerRecord) -> Result<TimerRecord, Error> {
        self.save_timer_impl(timer)
    }

    fn update_timer(&self, _ctx: &Context, input: TimerActionInput) -> Result<TimerRecord, Error> {
        self.update_timer_impl(input)
    }

    fn list_timers(&self, _ctx: &Context, run_id: String) -> Result<Vec<TimerRecord>, Error> {
        self.list_timers_impl(&run_id)
    }

    fn list_pantry(&self, _ctx: &Context, query: PantryQuery) -> Result<PantryLedger, Error> {
        self.list_pantry_impl(&query)
    }

    fn upsert_pantry(&self, _ctx: &Context, input: PantryUpsert) -> Result<PantryItem, Error> {
        self.upsert_pantry_impl(input)
    }

    fn delete_pantry(&self, _ctx: &Context, input: PantryDelete) -> Result<PantryDeleteResult, Error> {
        self.delete_pantry_impl(input)
    }

    fn export_pantry_labels(&self, _ctx: &Context, query: PantryQuery) -> Result<PantryLabels, Error> {
        self.export_pantry_labels_impl(&query)
    }

    fn record_projection(&self, _ctx: &Context, input: RecordProjectionInput) -> Result<String, Error> {
        self.record_projection_impl(&input)
    }

    fn normalize_runtime(&self, _ctx: &Context, input: RuntimeNormalizeInput) -> Result<RuntimeNormalizeOutput, Error> {
        self.normalize_runtime_impl(&input)
    }

    fn sign_and_archive_run(&self, _ctx: &Context, input: SignAndArchiveInput) -> Result<ArchiveState, Error> {
        self.sign_and_archive_run_impl(&input)
    }

    fn close(&self) -> Result<(), Error> {
        if let Ok(mut g) = self.core.lock() { g.take(); }
        Ok(())
    }
}

/// The bodies behind the RunService methods. They take no context so the tests
/// can exercise the bridge without the generated Go-side context.
impl CoreRunService {
    fn save_timer_impl(&self, timer: TimerRecord) -> Result<TimerRecord, Error> {
        let mut guard = self.core.lock().map_err(|_| Error::Go("INTERNAL: core lock poisoned".into()))?;
        let core = guard.as_mut().ok_or_else(|| Error::Go("INTERNAL: closed".into()))?;
        let record = timer_to_core(&timer)?;
        core.save_timer_record(&record).map_err(Error::Go)?;
        Ok(timer)
    }

    fn update_timer_impl(&self, input: TimerActionInput) -> Result<TimerRecord, Error> {
        let mut guard = self.core.lock().map_err(|_| Error::Go("INTERNAL: core lock poisoned".into()))?;
        let core = guard.as_mut().ok_or_else(|| Error::Go("INTERNAL: closed".into()))?;
        let mut timer = timer_to_core(&input.timer)?;
        match input.action.as_str() {
            "start" => timer.start(&input.now_utc),
            "pause" => timer.pause(&input.now_utc),
            "advance" => timer.advance_countdown(&input.now_utc).map(|_| ()),
            "" => Err("action is required: start, pause or advance".to_owned()),
            other => Err(format!("unsupported timer action {other}; use start, pause or advance")),
        }
        .map_err(|e| Error::Go(format!("INVALID: {e}")))?;
        core.save_timer_record(&timer).map_err(Error::Go)?;
        Ok(timer_from_core(&timer))
    }

    fn list_timers_impl(&self, run_id: &str) -> Result<Vec<TimerRecord>, Error> {
        let guard = self.core.lock().map_err(|_| Error::Go("INTERNAL: core lock poisoned".into()))?;
        let core = guard.as_ref().ok_or_else(|| Error::Go("INTERNAL: closed".into()))?;
        let timers = core.load_timer_records(run_id).map_err(Error::Go)?;
        Ok(timers.iter().map(timer_from_core).collect())
    }

    fn list_pantry_impl(&self, query: &PantryQuery) -> Result<PantryLedger, Error> {
        let guard = self.core.lock().map_err(|_| Error::Go("INTERNAL: core lock poisoned".into()))?;
        let core = guard.as_ref().ok_or_else(|| Error::Go("INTERNAL: closed".into()))?;
        let value = core.pantry_list(pantry_query_json(query)).map_err(Error::Go)?;
        Ok(PantryLedger {
            as_of: text_field(&value, "as_of"),
            near_expiry_days: value["near_expiry_days"].as_u64().unwrap_or_default() as u32,
            items: value["items"]
                .as_array()
                .into_iter()
                .flatten()
                .map(pantry_item_from_value)
                .collect(),
        })
    }

    fn upsert_pantry_impl(&self, input: PantryUpsert) -> Result<PantryItem, Error> {
        let mut guard = self.core.lock().map_err(|_| Error::Go("INTERNAL: core lock poisoned".into()))?;
        let core = guard.as_mut().ok_or_else(|| Error::Go("INTERNAL: closed".into()))?;
        let mut request = serde_json::json!({
            "name": input.name,
            "location": input.location,
            "exhausted": input.exhausted
        });
        if !input.id.is_empty() { request["id"] = serde_json::json!(input.id); }
        if !input.project_id.is_empty() { request["project_id"] = serde_json::json!(input.project_id); }
        if !input.purchased_on.is_empty() { request["purchased_on"] = serde_json::json!(input.purchased_on); }
        if !input.opened_on.is_empty() { request["opened_on"] = serde_json::json!(input.opened_on); }
        if let Some(days) = input.shelf_life_days { request["shelf_life_days"] = serde_json::json!(days); }
        if let Some(days) = input.opened_shelf_life_days { request["opened_shelf_life_days"] = serde_json::json!(days); }
        if let Some(revision) = input.expected_revision { request["expected_revision"] = serde_json::json!(revision); }
        let value = core.pantry_upsert(request).map_err(Error::Go)?;
        Ok(pantry_item_from_value(&value["item"]))
    }

    fn delete_pantry_impl(&self, input: PantryDelete) -> Result<PantryDeleteResult, Error> {
        let mut guard = self.core.lock().map_err(|_| Error::Go("INTERNAL: core lock poisoned".into()))?;
        let core = guard.as_mut().ok_or_else(|| Error::Go("INTERNAL: closed".into()))?;
        let mut request = serde_json::json!({"id": input.id});
        if let Some(revision) = input.expected_revision { request["expected_revision"] = serde_json::json!(revision); }
        let value = core.pantry_delete(request).map_err(Error::Go)?;
        Ok(PantryDeleteResult {
            id: text_field(&value, "pantry_item_id"),
            label_code: text_field(&value, "label_code"),
            revision: value["revision"].as_i64().unwrap_or_default(),
            deleted: value["deleted"].as_bool().unwrap_or(false),
        })
    }

    fn export_pantry_labels_impl(&self, query: &PantryQuery) -> Result<PantryLabels, Error> {
        let guard = self.core.lock().map_err(|_| Error::Go("INTERNAL: core lock poisoned".into()))?;
        let core = guard.as_ref().ok_or_else(|| Error::Go("INTERNAL: closed".into()))?;
        let value = core.pantry_export_labels(pantry_query_json(query)).map_err(Error::Go)?;
        Ok(PantryLabels {
            schema_version: value["schema_version"].as_i64().unwrap_or_default(),
            generated_at: text_field(&value, "generated_at"),
            as_of: text_field(&value, "as_of"),
            near_expiry_days: value["near_expiry_days"].as_u64().unwrap_or_default() as u32,
            labels: value["labels"]
                .as_array()
                .into_iter()
                .flatten()
                .map(|label| PantryLabel {
                    label_code: text_field(label, "label_code"),
                    pantry_item_id: text_field(label, "pantry_item_id"),
                    name: text_field(label, "name"),
                    location: text_field(label, "location"),
                    status: text_field(label, "status"),
                    expires_on: text_field(label, "expires_on"),
                    days_remaining: label["days_remaining"].as_i64(),
                    revision: label["revision"].as_i64().unwrap_or_default(),
                })
                .collect(),
        })
    }

    fn record_projection_impl(&self, input: &RecordProjectionInput) -> Result<String, Error> {
        let guard = self.core.lock().map_err(|_| Error::Go("INTERNAL: core lock poisoned".into()))?;
        let core = guard.as_ref().ok_or_else(|| Error::Go("INTERNAL: closed".into()))?;
        let db = core.raw_db();
        let workspace = cli_impl::run_workspace_typed(db, &input.run_id).map_err(map_cli)?;
        let timers = core.load_timer_records(&input.run_id).map_err(Error::Go)?;
        let (links, _) = load_asset_links(db, &input.run_id)?;
        let projection_input = CoreProjectionInput {
            view: projection_view(&input.view)?,
            fov_per_well: if input.fov_per_well == 0 { 1 } else { input.fov_per_well },
            notes: load_notes(db, &input.run_id)?,
            archive: load_archive_state(db, &input.run_id)?,
            now_utc: opt_string(&input.now_utc),
        };
        let projection = build_record_projection(&workspace, &links, &timers, &projection_input);
        serde_json::to_string(&projection).map_err(|e| Error::Go(format!("INTERNAL: {e}")))
    }

    fn normalize_runtime_impl(&self, input: &RuntimeNormalizeInput) -> Result<RuntimeNormalizeOutput, Error> {
        let output = runtime_normalize(&CoreNormalizeInput {
            planned_wells: input.planned_wells,
            actual_wells: input.actual_wells,
            volume_per_well_ul: input.volume_per_well_ul,
            excess_ratio: input.excess_ratio,
            dead_volume_ul: input.dead_volume_ul,
            prepared_volume_ul: input.prepared_volume_ul,
        })
        .map_err(|e| Error::Go(format!("INVALID: {e}")))?;
        Ok(RuntimeNormalizeOutput {
            planned_volume_ul: output.planned_volume_ul,
            required_volume_ul: output.required_volume_ul,
            dead_volume_ul: output.dead_volume_ul,
            additional_volume_ul: output.additional_volume_ul,
            unused_volume_ul: output.unused_volume_ul,
        })
    }

    fn sign_and_archive_run_impl(&self, input: &SignAndArchiveInput) -> Result<ArchiveState, Error> {
        let guard = self.core.lock().map_err(|_| Error::Go("INTERNAL: core lock poisoned".into()))?;
        let core = guard.as_ref().ok_or_else(|| Error::Go("INTERNAL: closed".into()))?;
        let db = core.raw_db();
        let workspace = cli_impl::run_workspace_typed(db, &input.run_id).map_err(map_cli)?;
        let timers = core.load_timer_records(&input.run_id).map_err(Error::Go)?;
        let notes = load_notes(db, &input.run_id)?;
        let (links, assets) = load_asset_links(db, &input.run_id)?;
        let mut state = load_archive_state(db, &input.run_id)?;
        if state.status == RecordStatus::Recording {
            state = summarize_record(&state).map_err(record_error)?;
        }
        let projection = build_record_projection(
            &workspace,
            &links,
            &timers,
            &CoreProjectionInput {
                view: projection_view(&input.view)?,
                fov_per_well: if input.fov_per_well == 0 { 1 } else { input.fov_per_well },
                notes,
                archive: state.clone(),
                now_utc: opt_string(&input.now_utc),
            },
        );
        let signature = ArchiveSignature {
            signed_by: input.signed_by.clone(),
            signed_at_utc: input.signed_at_utc.clone(),
            statement: opt_string(&input.statement),
        };
        let archived = sign_and_archive(&state, &signature, &projection, &links, &assets)
            .map_err(record_error)?;
        save_archive_state(db, &input.run_id, &archived)?;
        Ok(archive_state_out(&input.run_id, &archived))
    }
}

fn asset_from_value(v: &serde_json::Value) -> Asset {
    Asset {
        id: v.get("id").and_then(|x| x.as_str()).unwrap_or("").to_owned(),
        root_id: v.get("rootId").and_then(|x| x.as_str()).unwrap_or("").to_owned(),
        relative_path: v.get("relativePath").and_then(|x| x.as_str()).unwrap_or("").to_owned(),
        media_type: v.get("mediaType").and_then(|x| x.as_str()).unwrap_or("").to_owned(),
        size: v.get("size").and_then(|x| x.as_i64()).unwrap_or(0),
        mod_time: v.get("modTime").and_then(|x| x.as_str()).unwrap_or("").to_owned(),
        sha256: v.get("sha256").and_then(|x| x.as_str()).unwrap_or("").to_owned(),
        availability: v.get("availability").and_then(|x| x.as_str()).unwrap_or("").to_owned(),
        link: AssetLink {
            run_id: v.get("link").and_then(|x| x.get("runId")).and_then(|x| x.as_str()).unwrap_or("").to_owned(),
            task_key: v.get("link").and_then(|x| x.get("taskKey")).and_then(|x| x.as_str()).unwrap_or("").to_owned(),
            sample_id: v.get("link").and_then(|x| x.get("sampleId")).and_then(|x| x.as_str()).unwrap_or("").to_owned(),
            well_id: v.get("link").and_then(|x| x.get("wellId")).and_then(|x| x.as_str()).unwrap_or("").to_owned(),
            timepoint: v.get("link").and_then(|x| x.get("timepoint")).and_then(|x| x.as_str()).unwrap_or("").to_owned(),
            fov_index: v.get("link").and_then(|x| x.get("fovIndex")).and_then(|x| x.as_u64()).unwrap_or(0) as u32,
        },
        linked_at: v.get("linkedAt").and_then(|x| x.as_str()).unwrap_or("").to_owned(),
        actor_id: v.get("actorId").and_then(|x| x.as_str()).unwrap_or("").to_owned(),
    }
}

fn parameter_text(value: &serde_json::Value) -> String {
    match value {
        serde_json::Value::String(text) => text.clone(),
        serde_json::Value::Number(number) => number.to_string(),
        serde_json::Value::Bool(flag) => flag.to_string(),
        serde_json::Value::Null => String::new(),
        other => other.to_string(),
    }
}

fn text_field(value: &serde_json::Value, key: &str) -> String {
    value.get(key).and_then(|field| field.as_str()).unwrap_or_default().to_owned()
}

fn opt_string(value: &str) -> Option<String> {
    if value.trim().is_empty() { None } else { Some(value.to_owned()) }
}

fn projection_view(view: &str) -> Result<ProjectionView, Error> {
    match view.trim() {
        "" | "unblinded" => Ok(ProjectionView::Unblinded),
        "blind" => Ok(ProjectionView::Blind),
        other => Err(Error::Go(format!(
            "INVALID: unknown projection view {other}; use blind or unblinded"
        ))),
    }
}

fn timer_mode(text: &str) -> Result<TimerMode, Error> {
    serde_json::from_value(serde_json::json!(text))
        .map_err(|_| Error::Go(format!("INVALID: unknown timer mode {text}")))
}

fn timer_status(text: &str) -> Result<TimerStatus, Error> {
    serde_json::from_value(serde_json::json!(text))
        .map_err(|_| Error::Go(format!("INVALID: unknown timer status {text}")))
}

fn timer_to_core(timer: &TimerRecord) -> Result<CoreTimerRecord, Error> {
    Ok(CoreTimerRecord {
        timer_id: timer.timer_id.clone(),
        run_id: timer.run_id.clone(),
        task_key: timer.task_key.clone(),
        phase_index: timer.phase_index,
        mode: timer_mode(&timer.mode)?,
        planned_s: timer.planned_s,
        started_at_utc: timer.started_at_utc.clone(),
        accumulated_s: timer.accumulated_s,
        status: timer_status(&timer.status)?,
        round_index: timer.round_index,
        round_count: timer.round_count,
        anchor_event_id: timer.anchor_event_id.clone(),
        anchor_task_key: timer.anchor_task_key.clone(),
    })
}

fn timer_from_core(timer: &CoreTimerRecord) -> TimerRecord {
    let mode = serde_json::to_value(timer.mode).ok();
    let status = serde_json::to_value(timer.status).ok();
    TimerRecord {
        timer_id: timer.timer_id.clone(),
        run_id: timer.run_id.clone(),
        task_key: timer.task_key.clone(),
        phase_index: timer.phase_index,
        mode: mode.and_then(|value| value.as_str().map(str::to_owned)).unwrap_or_default(),
        planned_s: timer.planned_s,
        started_at_utc: timer.started_at_utc.clone(),
        accumulated_s: timer.accumulated_s,
        status: status.and_then(|value| value.as_str().map(str::to_owned)).unwrap_or_default(),
        round_index: timer.round_index,
        round_count: timer.round_count,
        anchor_event_id: timer.anchor_event_id.clone(),
        anchor_task_key: timer.anchor_task_key.clone(),
    }
}

fn pantry_query_json(query: &PantryQuery) -> serde_json::Value {
    let mut value = serde_json::json!({});
    if !query.project_id.is_empty() { value["project_id"] = serde_json::json!(query.project_id); }
    if !query.as_of.is_empty() { value["as_of"] = serde_json::json!(query.as_of); }
    if let Some(days) = query.near_expiry_days { value["near_expiry_days"] = serde_json::json!(days); }
    if let Some(flag) = query.include_exhausted { value["include_exhausted"] = serde_json::json!(flag); }
    value
}

fn pantry_item_from_value(value: &serde_json::Value) -> PantryItem {
    PantryItem {
        id: text_field(value, "id"),
        label_code: text_field(value, "label_code"),
        project_id: text_field(value, "project_id"),
        name: text_field(value, "name"),
        location: text_field(value, "location"),
        purchased_on: text_field(value, "purchased_on"),
        opened_on: text_field(value, "opened_on"),
        shelf_life_days: value["shelf_life_days"].as_u64().map(|days| days as u32),
        opened_shelf_life_days: value["opened_shelf_life_days"].as_u64().map(|days| days as u32),
        exhausted: value["exhausted"].as_bool().unwrap_or(false),
        revision: value["revision"].as_i64().unwrap_or_default(),
        status: text_field(value, "status"),
        expires_on: text_field(value, "expires_on"),
        days_remaining: value["days_remaining"].as_i64(),
        expiry_basis: text_field(value, "expiry_basis"),
        created_at: text_field(value, "created_at"),
        updated_at: text_field(value, "updated_at"),
    }
}

fn record_error(error: RecordCommandError) -> Error {
    let prefix = match error.code.as_str() {
        "invalid_transition" | "already_archived" | "asset_checksum_missing" => "CONFLICT",
        "missing_signer" | "invalid_timestamp" | "unsupported_manifest" | "invalid_checksum" => "INVALID",
        _ => "INTERNAL",
    };
    Error::Go(format!("{prefix}: {}", error.message))
}

/// Prose notes of a Run: `note` records plus the notebook-stage observations
/// this module's SaveNotebook writes. Both are prose, never facts; a note's
/// markdown citations are resolved by the projection, not here.
fn load_notes(db: &rusqlite::Connection, run_id: &str) -> Result<Vec<RecordNote>, Error> {
    let mut statement = db
        .prepare(
            "SELECT id,kind,data_json,updated_at FROM records \
             WHERE parent_id=?1 AND (kind='note' OR kind='execution_event') \
             ORDER BY created_at ASC, rowid ASC",
        )
        .map_err(|e| Error::Go(format!("INTERNAL: {e}")))?;
    let rows = statement
        .query_map([run_id], |row| {
            Ok((
                row.get::<_, String>(0)?,
                row.get::<_, String>(1)?,
                row.get::<_, String>(2)?,
                row.get::<_, String>(3)?,
            ))
        })
        .map_err(|e| Error::Go(format!("INTERNAL: {e}")))?;
    let mut notes = Vec::new();
    for row in rows.flatten() {
        let (id, kind, raw, updated_at) = row;
        let data: serde_json::Value = serde_json::from_str(&raw).unwrap_or(serde_json::Value::Null);
        if kind == "execution_event" && data["stage"].as_str() != Some("notebook") {
            continue;
        }
        let title = if kind == "note" {
            data.get("title").and_then(|value| value.as_str()).map(str::to_owned)
        } else {
            None
        };
        notes.push(RecordNote {
            note_id: id,
            title,
            markdown: text_field(&data, "content"),
            updated_at_utc: updated_at,
        });
    }
    Ok(notes)
}

/// Stored asset links, in the two shapes Core needs: the Run association and
/// the observed file evidence used for the archive checksums.
fn load_asset_links(
    db: &rusqlite::Connection,
    run_id: &str,
) -> Result<(Vec<CoreAssetLink>, Vec<AssetRef>), Error> {
    let value = cli_impl::list_asset_links(db, run_id).map_err(map_cli)?;
    let mut links = Vec::new();
    let mut assets = Vec::new();
    for item in value.as_array().into_iter().flatten() {
        let asset: AssetRef = serde_json::from_value(item.clone())
            .map_err(|e| Error::Go(format!("INTERNAL: invalid asset link: {e}")))?;
        let link = &item["link"];
        links.push(CoreAssetLink {
            asset_id: asset.id.clone(),
            run_id: run_id.to_owned(),
            task_key: text_field(link, "taskKey"),
            sample_id: opt_string(&text_field(link, "sampleId")),
            well_id: opt_string(&text_field(link, "wellId")),
            timepoint: opt_string(&text_field(link, "timepoint")),
            fov_index: link["fovIndex"].as_u64().map(|index| index as u32),
        });
        assets.push(asset);
    }
    Ok((links, assets))
}

fn load_timers(db: &rusqlite::Connection, run_id: &str) -> Result<Vec<CoreTimerRecord>, Error> {
    let mut statement = db
        .prepare("SELECT data_json FROM timer_records WHERE run_id=?1 ORDER BY task_key,timer_id")
        .map_err(|e| Error::Go(format!("INTERNAL: {e}")))?;
    let rows = statement
        .query_map([run_id], |row| row.get::<_, String>(0))
        .map_err(|e| Error::Go(format!("INTERNAL: {e}")))?;
    let mut timers = Vec::new();
    for raw in rows.flatten() {
        timers.push(serde_json::from_str(&raw).map_err(|e| Error::Go(format!("INTERNAL: {e}")))?);
    }
    Ok(timers)
}

fn load_archive_state(db: &rusqlite::Connection, run_id: &str) -> Result<RecordArchiveState, Error> {
    let raw: Option<String> = db
        .query_row(
            "SELECT data_json FROM run_archive_state WHERE run_id=?1",
            [run_id],
            |row| row.get(0),
        )
        .optional()
        .map_err(|e| Error::Go(format!("INTERNAL: {e}")))?;
    match raw {
        Some(raw) => serde_json::from_str(&raw).map_err(|e| Error::Go(format!("INTERNAL: {e}"))),
        None => Ok(RecordArchiveState::recording()),
    }
}

fn save_archive_state(
    db: &rusqlite::Connection,
    run_id: &str,
    state: &RecordArchiveState,
) -> Result<(), Error> {
    let payload = serde_json::to_string(state).map_err(|e| Error::Go(format!("INTERNAL: {e}")))?;
    db.execute(
        "INSERT INTO run_archive_state(run_id,data_json,updated_at) VALUES(?1,?2,datetime('now')) \
         ON CONFLICT(run_id) DO UPDATE SET data_json=excluded.data_json,updated_at=excluded.updated_at",
        rusqlite::params![run_id, payload],
    )
    .map_err(|e| Error::Go(format!("INTERNAL: {e}")))?;
    Ok(())
}

fn archive_state_out(run_id: &str, state: &RecordArchiveState) -> ArchiveState {
    let (signed_by, signed_at_utc, statement) = match &state.signature {
        Some(signature) => (
            signature.signed_by.clone(),
            signature.signed_at_utc.clone(),
            signature.statement.clone().unwrap_or_default(),
        ),
        None => (String::new(), String::new(), String::new()),
    };
    ArchiveState {
        run_id: run_id.to_owned(),
        status: state.status.as_str().to_owned(),
        signed_by,
        signed_at_utc,
        statement,
        manifest_json: state
            .manifest
            .as_ref()
            .and_then(|manifest| serde_json::to_string(manifest).ok())
            .unwrap_or_default(),
        notes_locked: state.notes_locked(),
    }
}

struct CoreDesignService { db: Mutex<Option<rusqlite::Connection>> }

impl DesignService for CoreDesignService {
    fn list_projects(&self, _ctx: &Context) -> Result<Vec<ProjectSummary>, Error> {
        let guard = self.db.lock().map_err(|_| Error::Go("INTERNAL: db lock poisoned".into()))?;
        let db = guard.as_ref().ok_or_else(|| Error::Go("INTERNAL: closed".into()))?;
        let mut stmt = db.prepare("SELECT id,data_json FROM records WHERE kind='project' ORDER BY created_at ASC")
            .map_err(|e| Error::Go(format!("INTERNAL: {e}")))?;
        let rows = stmt.query_map([], |row| Ok((row.get::<_,String>(0)?, row.get::<_,String>(1)?)))
            .map_err(|e| Error::Go(format!("INTERNAL: {e}")))?;
        let mut out = Vec::new();
        for row in rows.flatten() {
            let (id, raw) = row;
            let data: serde_json::Value = serde_json::from_str(&raw).unwrap_or(serde_json::json!({}));
            out.push(ProjectSummary {
                project_id: id,
                name: data["name"].as_str().unwrap_or("未命名项目").into(),
            });
        }
        Ok(out)
    }

    fn list_protocols(&self, _ctx: &Context) -> Result<Vec<ProtocolSummary>, Error> {
        let guard = self.db.lock().map_err(|_| Error::Go("INTERNAL: db lock poisoned".into()))?;
        let db = guard.as_ref().ok_or_else(|| Error::Go("INTERNAL: closed".into()))?;
        let mut stmt = db.prepare(
            "SELECT v.id, v.parent_id, v.data_json, p.data_json FROM records v LEFT JOIN records p ON p.id=v.parent_id WHERE v.kind='protocol_version' ORDER BY v.created_at DESC"
        ).map_err(|e| Error::Go(format!("INTERNAL: {e}")))?;
        let rows = stmt.query_map([], |row| Ok((
            row.get::<_,String>(0)?, row.get::<_,Option<String>>(1)?,
            row.get::<_,String>(2)?, row.get::<_,Option<String>>(3)?,
        ))).map_err(|e| Error::Go(format!("INTERNAL: {e}")))?;
        let mut out = Vec::new();
        for row in rows.flatten() {
            let (id, protocol_id, raw, protocol_raw) = row;
            let version: serde_json::Value = serde_json::from_str(&raw).unwrap_or(serde_json::json!({}));
            let protocol: serde_json::Value = serde_json::from_str(protocol_raw.as_deref().unwrap_or("{}")).unwrap_or(serde_json::json!({}));
            out.push(ProtocolSummary {
                protocol_version_id: id,
                protocol_id: protocol_id.unwrap_or_default(),
                title: protocol["title"].as_str().or_else(|| version["title"].as_str()).unwrap_or("Protocol").into(),
                version: version["version"].as_i64().unwrap_or(1) as i64,
            });
        }
        Ok(out)
    }

    fn get_protocol_version(&self, _ctx: &Context, protocol_version_id: String) -> Result<ProtocolVersionContent, Error> {
        let guard = self.db.lock().map_err(|_| Error::Go("INTERNAL: db lock poisoned".into()))?;
        let db = guard.as_ref().ok_or_else(|| Error::Go("INTERNAL: closed".into()))?;
        let view = cli_impl::protocol_version_get(db, &protocol_version_id).map_err(map_cli)?;
        protocol_version_content(&view)
    }

    fn list_protocol_versions(&self, _ctx: &Context, protocol_id: String) -> Result<Vec<ProtocolVersionContent>, Error> {
        let guard = self.db.lock().map_err(|_| Error::Go("INTERNAL: db lock poisoned".into()))?;
        let db = guard.as_ref().ok_or_else(|| Error::Go("INTERNAL: closed".into()))?;
        let versions = cli_impl::protocol_version_list(db, &protocol_id).map_err(map_cli)?;
        versions.iter().map(protocol_version_content).collect()
    }

    fn preview_protocol_block(&self, _ctx: &Context, block_version_id: String) -> Result<ProtocolBlockPreview, Error> {
        let guard = self.db.lock().map_err(|_| Error::Go("INTERNAL: db lock poisoned".into()))?;
        let db = guard.as_ref().ok_or_else(|| Error::Go("INTERNAL: closed".into()))?;
        let manifest = cli_impl::protocol_block_manifest(db, Some(&block_version_id), &serde_json::json!([]))
            .map_err(|e| Error::Go(format!("INVALID: {e}")))?;
        let mut steps = Vec::new();
        for step in manifest["steps"].as_array().into_iter().flatten() {
            let path = step["path"].as_str().unwrap_or("").to_owned();
            if path.is_empty() {
                continue;
            }
            let mut parameters = Vec::new();
            for parameter in step["parameters"].as_array().into_iter().flatten() {
                parameters.push(ProtocolBlockParameter {
                    key: parameter["slot"].as_str().unwrap_or("").to_owned(),
                    label: parameter["label"].as_str().unwrap_or("").to_owned(),
                    value: parameter_text(&parameter["value"]),
                    unit: parameter["unit"].as_str().unwrap_or("").to_owned(),
                });
            }
            let unresolved_parameter_refs = step["unresolved_parameter_refs"]
                .as_array()
                .into_iter()
                .flatten()
                .filter_map(|value| value.as_str().map(str::to_owned))
                .collect();
            steps.push(ProtocolBlockStep {
                use_id: step["use_id"].as_str().unwrap_or("").to_owned(),
                path,
                title: step["title"].as_str().unwrap_or("").to_owned(),
                instruction: step["instruction"].as_str().unwrap_or("").to_owned(),
                source_block_version_id: step["source_block_version_id"].as_str().unwrap_or("").to_owned(),
                parameters,
                unresolved_parameter_refs,
            });
        }
        Ok(ProtocolBlockPreview {
            block_version_id,
            ready: manifest["protocol_ready"].as_bool().unwrap_or(false),
            steps,
        })
    }

    fn list_designs(&self, _ctx: &Context) -> Result<Vec<DesignSummary>, Error> {
        let guard = self.db.lock().map_err(|_| Error::Go("INTERNAL: db lock poisoned".into()))?;
        let db = guard.as_ref().ok_or_else(|| Error::Go("INTERNAL: closed".into()))?;
        let mut stmt = db.prepare("SELECT id,revision,data_json FROM records WHERE kind='design' ORDER BY created_at DESC").map_err(|e| Error::Go(format!("INTERNAL: {e}")))?;
        let rows = stmt.query_map([], |r| Ok((r.get::<_,String>(0)?, r.get::<_,i64>(1)?, r.get::<_,String>(2)?))).map_err(|e| Error::Go(format!("INTERNAL: {e}")))?;
        let mut out = Vec::new();
        for row in rows {
            let (id, rev, raw) = row.map_err(|e| Error::Go(format!("INTERNAL: {e}")))?;
            let data: serde_json::Value = serde_json::from_str(&raw).unwrap_or(serde_json::json!({}));
            out.push(DesignSummary { design_id: id, title: data["title"].as_str().unwrap_or("").into(), revision: rev as u64 });
        }
        Ok(out)
    }

    fn load_design(&self, _ctx: &Context, design_id: String) -> Result<DesignSnapshot, Error> {
        let guard = self.db.lock().map_err(|_| Error::Go("INTERNAL: db lock poisoned".into()))?;
        let db = guard.as_ref().ok_or_else(|| Error::Go("INTERNAL: closed".into()))?;
        let row = db.query_row("SELECT kind,revision,data_json FROM records WHERE id=?1", rusqlite::params![design_id], |r| Ok((r.get::<_,String>(0)?, r.get::<_,i64>(1)?, r.get::<_,String>(2)?)));
        let (kind, revision, raw) = match row { Ok(x) => x, Err(_) => return Err(Error::Go("NOT_FOUND: design not found".into())) };
        if kind != "design" { return Err(Error::Go("INVALID: not a design".into())); }
        let data: serde_json::Value = serde_json::from_str(&raw).unwrap_or(serde_json::json!({}));
        let title = data["title"].as_str().unwrap_or("").to_string();
        Ok(DesignSnapshot { design_id: design_id.clone(), revision: revision as u64, title, data: data.to_string() })
    }

    fn create_aster_doc_design(&self, _ctx: &Context, input: AsterDocCreateInput) -> Result<AsterDocSnapshot, Error> {
        let mut guard = self.db.lock().map_err(|_| Error::Go("INTERNAL: db lock poisoned".into()))?;
        let db = guard.as_mut().ok_or_else(|| Error::Go("INTERNAL: closed".into()))?;
        let request = serde_json::json!({
            "project_id": input.project_id,
            "title": input.title,
            "source": input.source,
            "operation_id": input.operation_id,
            "actor_id": input.actor_id
        });
        let value = cli_impl::create_asterdoc_design(db, &request).map_err(map_cli)?;
        Ok(asterdoc_snapshot(value))
    }

    fn apply_aster_doc_command(&self, _ctx: &Context, input: AsterDocCommandInput) -> Result<AsterDocSnapshot, Error> {
        let mut guard = self.db.lock().map_err(|_| Error::Go("INTERNAL: db lock poisoned".into()))?;
        let db = guard.as_mut().ok_or_else(|| Error::Go("INTERNAL: closed".into()))?;
        let command: serde_json::Value = serde_json::from_str(&input.command)
            .map_err(|e| Error::Go(format!("INVALID: command is not JSON: {e}")))?;
        let request = serde_json::json!({
            "design_id": input.design_id,
            "expected_revision": input.expected_revision,
            "operation_id": input.operation_id,
            "actor_id": input.actor_id,
            "command": command
        });
        let value = cli_impl::apply_asterdoc_command(db, &request).map_err(map_cli)?;
        Ok(asterdoc_snapshot(value))
    }

    fn save_aster_doc(&self, _ctx: &Context, input: AsterDocSaveInput) -> Result<AsterDocSnapshot, Error> {
        let mut guard = self.db.lock().map_err(|_| Error::Go("INTERNAL: db lock poisoned".into()))?;
        let db = guard.as_mut().ok_or_else(|| Error::Go("INTERNAL: closed".into()))?;
        let request = serde_json::json!({
            "design_id": input.design_id,
            "source": input.source,
            "expected_revision": input.expected_revision,
            "operation_id": input.operation_id,
            "actor_id": input.actor_id
        });
        let value = cli_impl::save_asterdoc_version(db, &request).map_err(map_cli)?;
        Ok(asterdoc_snapshot(value))
    }

    fn load_aster_doc(&self, _ctx: &Context, design_id: String) -> Result<AsterDocSnapshot, Error> {
        let guard = self.db.lock().map_err(|_| Error::Go("INTERNAL: db lock poisoned".into()))?;
        let db = guard.as_ref().ok_or_else(|| Error::Go("INTERNAL: closed".into()))?;
        let value = cli_impl::load_asterdoc(db, &design_id).map_err(map_cli)?;
        Ok(asterdoc_snapshot(value))
    }

    fn assess_aster_doc(&self, _ctx: &Context, design_id: String, revision: u64) -> Result<AsterDocAssessment, Error> {
        let guard = self.db.lock().map_err(|_| Error::Go("INTERNAL: db lock poisoned".into()))?;
        let db = guard.as_ref().ok_or_else(|| Error::Go("INTERNAL: closed".into()))?;
        let value = cli_impl::assess_asterdoc(db, &serde_json::json!({
            "design_id": design_id, "revision": revision
        })).map_err(map_cli)?;
        Ok(AsterDocAssessment {
            design_id: value["design_id"].as_str().unwrap_or("").into(),
            revision: value["revision"].as_u64().unwrap_or_default(),
            source_hash: value["source_hash"].as_str().unwrap_or("").into(),
            state: value["state"].as_str().unwrap_or("").into(),
            errors: value["errors"].as_array().into_iter().flatten().filter_map(|v| v.as_str().map(str::to_owned)).collect(),
            warnings: value["warnings"].as_array().into_iter().flatten().filter_map(|v| v.as_str().map(str::to_owned)).collect(),
        })
    }

    fn create_run_from_aster_doc(&self, _ctx: &Context, input: AsterDocRunInput) -> Result<RunSnapshot, Error> {
        let mut guard = self.db.lock().map_err(|_| Error::Go("INTERNAL: db lock poisoned".into()))?;
        let db = guard.as_mut().ok_or_else(|| Error::Go("INTERNAL: closed".into()))?;
        let title = if input.title.is_empty() { None } else { Some(input.title.as_str()) };
        let op = if input.operation_id.is_empty() { None } else { Some(input.operation_id.as_str()) };
        let v = cli_impl::create_run_from_asterdoc(db, &input.design_id, input.document_revision, op, title, &input.acknowledged_warnings)
            .map_err(map_cli)?;
        let run_id = v["id"].as_str().unwrap_or_default();
        let workspace = cli_impl::run_workspace(db, run_id).map_err(map_cli)?;
        let mut value = workspace;
        value["revision"] = v["revision"].clone();
        Ok(snapshot(value))
    }

    fn compile_aster_doc(&self, _ctx: &Context, design_id: String, revision: u64) -> Result<AsterDocProjection, Error> {
        let guard = self.db.lock().map_err(|_| Error::Go("INTERNAL: db lock poisoned".into()))?;
        let db = guard.as_ref().ok_or_else(|| Error::Go("INTERNAL: closed".into()))?;
        let doc = cli_impl::load_asterdoc_at(db, &design_id, revision).map_err(map_cli)?;
        let source = doc["source"].as_str().unwrap_or("");
        let design = cli_impl::compile_asterdoc_to_design(source).map_err(map_cli)?;
        Ok(AsterDocProjection {
            design: serde_json::to_string(&design).map_err(|e| Error::Go(format!("INTERNAL: {e}")))?,
            document: asterdoc_snapshot(doc),
        })
    }

    fn load_aster_doc_at(&self, _ctx: &Context, design_id: String, revision: u64) -> Result<AsterDocSnapshot, Error> {
        let guard = self.db.lock().map_err(|_| Error::Go("INTERNAL: db lock poisoned".into()))?;
        let db = guard.as_ref().ok_or_else(|| Error::Go("INTERNAL: closed".into()))?;
        let value = cli_impl::load_asterdoc_at(db, &design_id, revision).map_err(map_cli)?;
        Ok(asterdoc_snapshot(value))
    }

    fn save_design(&self, _ctx: &Context, input: DesignSaveInput) -> Result<DesignSnapshot, Error> {
        let mut guard = self.db.lock().map_err(|_| Error::Go("INTERNAL: db lock poisoned".into()))?;
        let db = guard.as_mut().ok_or_else(|| Error::Go("INTERNAL: closed".into()))?;
        let changes: serde_json::Value = serde_json::from_str(&input.changes)
            .unwrap_or(serde_json::Value::Null);
        let result = cli_impl::revise_design_record(
            db,
            &input.design_id,
            input.expected_revision as i64,
            &changes,
        ).map_err(map_cli)?;
        let data = result["design"]["data"].clone();
        Ok(DesignSnapshot {
            design_id: input.design_id,
            revision: result["design"]["revision"].as_u64().unwrap_or_default(),
            title: data["title"].as_str().unwrap_or("").to_string(),
            data: serde_json::to_string(&data).unwrap_or_default(),
        })
    }

    fn create_run(&self, _ctx: &Context, design_id: String, operation_id: String) -> Result<RunSnapshot, Error> {
        let mut guard = self.db.lock().map_err(|_| Error::Go("INTERNAL: db lock poisoned".into()))?;
        let db = guard.as_mut().ok_or_else(|| Error::Go("INTERNAL: closed".into()))?;
        let op = if operation_id.trim().is_empty() { None } else { Some(operation_id.as_str()) };
        let record = cli_impl::create_run_for_design(db, &design_id, None, op).map_err(map_cli)?;
        let run_id = record["id"].as_str().unwrap_or("").to_string();
        let v = cli_impl::run_workspace(db, &run_id).map_err(map_cli)?;
        let mut s = snapshot(v);
        s.run_id = run_id;
        Ok(s)
    }

    fn workflow_tree(&self, _ctx: &Context, design_id: String, revision: u64) -> Result<String, Error> {
        let guard = self.db.lock().map_err(|_| Error::Go("INTERNAL: db lock poisoned".into()))?;
        let db = guard.as_ref().ok_or_else(|| Error::Go("INTERNAL: closed".into()))?;
        let value = cli_impl::workflow_tree_for_db(
            db,
            Some(&design_id),
            (revision > 0).then_some(revision),
            None,
        )
        .map_err(map_cli)?;
        Ok(value.to_string())
    }

    fn derive_design_from_run(&self, _ctx: &Context, input: DeriveDesignInput) -> Result<DerivedDesign, Error> {
        self.derive_design_from_run_impl(&input)
    }

    fn close(&self) -> Result<(), Error> {
        if let Ok(mut g) = self.db.lock() { g.take(); }
        Ok(())
    }
}

/// The bodies behind the DesignService methods; see [`CoreRunService`].
impl CoreDesignService {
    fn derive_design_from_run_impl(&self, input: &DeriveDesignInput) -> Result<DerivedDesign, Error> {
        let guard = self.db.lock().map_err(|_| Error::Go("INTERNAL: db lock poisoned".into()))?;
        let db = guard.as_ref().ok_or_else(|| Error::Go("INTERNAL: closed".into()))?;
        let workspace = cli_impl::run_workspace_typed(db, &input.run_id).map_err(map_cli)?;
        let timers = load_timers(db, &input.run_id)?;
        let command = DeriveDesignCommand {
            run_id: input.run_id.clone(),
            title: opt_string(&input.title),
            objective: opt_string(&input.objective),
            note: opt_string(&input.note),
        };
        let derived = derive_design_from_run(&workspace, &timers, &command)
            .map_err(|e| Error::Go(format!("INVALID: {}", e.message)))?;
        let design_json = serde_json::to_string(&derived.to_design_json())
            .map_err(|e| Error::Go(format!("INTERNAL: {e}")))?;
        Ok(DerivedDesign {
            design_id: derived.design_id,
            source_design_version_id: derived.source_design_version_id,
            source_design_revision: derived.source_design_revision,
            expected_revision: derived.expected_revision,
            source_run_id: derived.source_run_id,
            design_json,
            note: derived.note.unwrap_or_default(),
        })
    }
}

fn map_cli(e: cli_impl::CliError) -> Error {
    match e.code {
        "INVALID_INPUT" => Error::Go(format!("INVALID: {}", e.message)),
        "NOT_FOUND" => Error::Go(format!("NOT_FOUND: {}", e.message)),
        "REVISION_CONFLICT" => Error::Go(format!("CONFLICT: {}", e.message)),
        _ => Error::Go(format!("INTERNAL: {}", e.message)),
    }
}

fn protocol_version_content(view: &serde_json::Value) -> Result<ProtocolVersionContent, Error> {
    let blob = |key: &str| -> Result<String, Error> {
        serde_json::to_string(&view[key]).map_err(|e| Error::Go(format!("INTERNAL: {e}")))
    };
    Ok(ProtocolVersionContent {
        protocol_version_id: view["protocol_version_id"].as_str().unwrap_or_default().to_owned(),
        protocol_id: view["protocol_id"].as_str().unwrap_or_default().to_owned(),
        version: view["version"].as_i64().unwrap_or_default(),
        root_block_version_id: view["root_block_version_id"].as_str().map(str::to_owned),
        root_bindings: blob("root_bindings")?,
        parameter_slots: blob("parameter_slots")?,
        steps: blob("steps")?,
        resolved_parameters: blob("resolved_parameters")?,
        unresolved_parameters: blob("unresolved_parameters")?,
        block_versions: view["block_version_ids"]
            .as_array()
            .into_iter()
            .flatten()
            .filter_map(|id| id.as_str().map(str::to_owned))
            .collect(),
        frozen: view["frozen"].as_bool().unwrap_or(false),
    })
}

/// Projects Core's single time budget from the exact OperationSpec JSON array.
/// No database is read and no duration is summed here: Core owns the arithmetic,
/// so the bridge cannot drift from `project_time_budget`.
fn project_time_budget_json(specs_json: &str) -> Result<TimeBudget, Error> {
    let specs: Vec<OperationSpec> = serde_json::from_str(specs_json)
        .map_err(|e| Error::Go(format!("INVALID: specs_json: {e}")))?;
    let budget = core_time_budget(&specs).map_err(|e| Error::Go(format!("INVALID: {e}")))?;
    Ok(TimeBudget {
        total_span_s: budget.total_span_s,
        hands_on_s: budget.hands_on_s,
        parallel_windows: budget
            .parallel_windows
            .iter()
            .map(|window| TimeBudgetWindow {
                step_index: window.step_index as u32,
                start_s: window.start_s,
                end_s: window.end_s,
                duration_s: window.duration_s,
                time_anchor_id: window.time_anchor_id.clone().unwrap_or_default(),
            })
            .collect(),
    })
}

fn asterdoc_snapshot(v: serde_json::Value) -> AsterDocSnapshot {
    AsterDocSnapshot {
        design_id: v["design_id"].as_str().unwrap_or("").into(),
        revision: v["revision"].as_u64().unwrap_or_default(),
        design_revision: v["design_revision"].as_u64().unwrap_or_default(),
        source: v["source"].as_str().unwrap_or("").into(),
        source_hash: v["source_hash"].as_str().unwrap_or("").into(),
        schema_version: v["schema_version"].as_i64().unwrap_or_default(),
        compiler_version: v["compiler_version"].as_str().unwrap_or("").into(),
        created_at: v["created_at"].as_str().unwrap_or("").into(),
        actor_id: v["actor_id"].as_str().unwrap_or("").into(),
    }
}

fn snapshot(v: serde_json::Value) -> RunSnapshot {
    let mut design_content = v["frozen_design"]["content"].clone();
    if let Some(mygo) = v["frozen_design"].get("mygo") {
        if !mygo.is_null() { design_content["mygo"] = mygo.clone(); }
    }
    if let Some(asterlab) = v["frozen_design"].get("asterlab") {
        if !asterlab.is_null() { design_content["asterlab"] = asterlab.clone(); }
    }
    let design_data = serde_json::to_string(&design_content).unwrap_or_default();
    let design_id = v["design_id"].as_str().unwrap_or("").into();
    let design_revision = v["design_revision"].as_u64().or_else(|| v["revision"].as_u64()).unwrap_or(0);
    let mut s = RunSnapshot {
        run_id: v["run_id"].as_str().unwrap_or("").into(),
        design_id,
        design_revision,
        design_data,
        revision: v["revision"].as_u64().unwrap_or(0),
        title: v["frozen_design"]["content"]["title"].as_str().unwrap_or("").into(),
        status: v["status"].as_str().unwrap_or("").into(),
        tasks: vec![],
        measurements: vec![],
        events: vec![],
        aster_doc: v["asterdoc"].as_object().map(|doc| AsterDocRef {
            design_id: doc["design_id"].as_str().unwrap_or("").into(),
            revision: doc["revision"].as_u64().unwrap_or_default(),
            source_hash: doc["source_hash"].as_str().unwrap_or("").into(),
            schema_version: doc["schema_version"].as_i64().unwrap_or_default() as i64,
            compiler_version: doc["compiler_version"].as_str().unwrap_or("").into(),
        }),
        creation_audit: v["creation_audit"].as_object().map(|audit| CreationAudit {
            rules_version: audit["rules_version"].as_i64().unwrap_or_default() as i64,
            design_revision: audit["design_revision"].as_u64().unwrap_or_default(),
            design_version_id: audit["design_version_id"].as_str().unwrap_or("").into(),
            actor_id: audit["actor_id"].as_str().unwrap_or("").into(),
            recorded_at: audit["recorded_at"].as_str().unwrap_or("").into(),
            decision: audit["decision"].as_str().unwrap_or("").into(),
            acknowledged_warnings: audit["acknowledged_warnings"].as_array().into_iter().flatten()
                .filter_map(|value| value.as_str().map(str::to_owned)).collect(),
            warnings: audit["warnings"].as_array().into_iter().flatten()
                .filter_map(|value| value.as_str().map(str::to_owned)).collect(),
            errors: audit["errors"].as_array().into_iter().flatten()
                .filter_map(|value| value.as_str().map(str::to_owned)).collect(),
        }),
    };
    // A task names the design step it executes through the design's explicit
    // stepTasks entries, matched on assay and timepoint; several matches stay
    // unresolved. Only designs written before stepTasks existed fall back to
    // the old unique-assay rule, and a title never decides either way.
    let step_tasks = v["frozen_design"]["mygo"]["stepTasks"]
        .as_array()
        .cloned()
        .unwrap_or_default();
    // A present table, even empty, means the design authored its bindings:
    // tasks without an entry stay unresolved instead of falling back.
    let explicit_links = v["frozen_design"]["mygo"]
        .as_object()
        .is_some_and(|mygo| mygo.contains_key("stepTasks"));
    // How many protocol paths each (assay, timepoint) pair appears under: an
    // entry without a path may only claim a pair that occurs once.
    let mut paths_per_pair: std::collections::HashMap<(String, String), std::collections::HashSet<String>> =
        std::collections::HashMap::new();
    for task in v["tasks"].as_array().into_iter().flatten() {
        let assay = task["scope"]
            .get("assay_id")
            .or_else(|| task["scope"].get("Assay").and_then(|a| a.get("assay_id")))
            .and_then(|x| x.as_str())
            .unwrap_or("");
        let timepoint = task["timepoint"].as_str().unwrap_or("");
        if assay.is_empty() {
            continue;
        }
        paths_per_pair
            .entry((assay.to_string(), timepoint.to_string()))
            .or_default()
            .insert(task["step_path"].as_str().unwrap_or("").to_string());
    }
    // Checks belong to the frozen protocol step. Match by its path; a path
    // names one step, and a task stores exactly that path.
    let mut checks_by_path: std::collections::HashMap<String, Vec<TaskCheck>> =
        std::collections::HashMap::new();
    for step in v["steps"].as_array().into_iter().flatten() {
        let path = step["step"]["path"].as_str().unwrap_or("").to_owned();
        let checks = step["step"]["checks"].as_array().into_iter().flatten().map(|check| TaskCheck {
            check_id: check["check_id"].as_str().unwrap_or("").into(),
            label: check["label"].as_str().unwrap_or("").into(),
            instruction: check["instruction"].as_str().unwrap_or("").into(),
            blocking: check["blocking"].as_bool().unwrap_or(false),
            status: String::new(),
        }).collect();
        checks_by_path.insert(path, checks);
    }
    let mut steps_by_path: std::collections::HashMap<String, (String, String, String, String)> =
        std::collections::HashMap::new();
    for step in v["steps"].as_array().into_iter().flatten() {
        let path = step["step"]["path"].as_str().unwrap_or("").to_owned();
        if path.is_empty() { continue; }
        let title = step["step"]["title"].as_str().unwrap_or("").to_owned();
        let instruction = step["step"]["instruction"].as_str().unwrap_or("").to_owned();
        let parameters_json = serde_json::to_string(&step["step"]["parameters"]).unwrap_or_else(|_| "[]".into());
        let source_block_version_id = step["step"]["source_block_version_id"].as_str().unwrap_or("").to_owned();
        steps_by_path.insert(path, (title, instruction, parameters_json, source_block_version_id));
    }
    let mut steps_by_assay: std::collections::HashMap<String, Vec<String>> = std::collections::HashMap::new();
    if let Some(step_assays) = v["frozen_design"]["mygo"]["stepAssays"].as_object() {
        for (node, assay) in step_assays {
            if let Some(assay) = assay.as_str().filter(|a| !a.is_empty()) {
                steps_by_assay.entry(assay.to_string()).or_default().push(node.clone());
            }
        }
    }
    // The compiler records which ProtocolBlock version each design step
    // reuses, and a frozen task records the block version that produced it.
    // That identity joins the two layers when the author-chosen design step
    // path and the block-internal protocol path differ, which they normally
    // do: paths alone can never join them.
    let mut step_blocks: std::collections::HashMap<String, Vec<String>> =
        std::collections::HashMap::new();
    if let Some(blocks) = v["frozen_design"]["mygo"]["stepBlocks"].as_object() {
        for (step_id, value) in blocks {
            let list: Vec<String> = value
                .as_array()
                .into_iter()
                .flatten()
                .filter_map(|block| block.as_str())
                .map(str::to_owned)
                .collect();
            if !list.is_empty() {
                step_blocks.insert(step_id.clone(), list);
            }
        }
    }
    for t in v["tasks"].as_array().into_iter().flatten() {
        let assay_id = t
            .get("scope")
            .and_then(|sc| {
                sc.get("assay_id")
                    .or_else(|| sc.get("Assay").and_then(|a| a.get("assay_id")))
            })
            .and_then(|x| x.as_str())
            .unwrap_or("")
            .to_string();
        let protocol_version_id = v["scopes"]
            .as_array()
            .into_iter()
            .flatten()
            .find(|sc| sc["kind"].as_str() == Some("assay") && sc["id"].as_str() == Some(assay_id.as_str()))
            .and_then(|sc| sc["protocol_version_id"].as_str())
            .unwrap_or("")
            .to_string();
        let timepoint = t["timepoint"].as_str().unwrap_or("").to_string();
        let path: String = t["step_path"].as_str().unwrap_or("").into();
        let source_block_version_id = steps_by_path
            .get(&path)
            .map(|(_, _, _, block)| block.clone())
            .unwrap_or_default();
        let design_step_id = if explicit_links {
            let step_path = path.as_str();
            let pair: Vec<&serde_json::Value> = step_tasks
                .iter()
                .filter(|link| {
                    link["assayId"].as_str() == Some(assay_id.as_str())
                        && link["timepoint"].as_str() == Some(timepoint.as_str())
                })
                .collect();
            // A path-qualified entry is exact. Otherwise the pair must occur
            // under exactly one protocol path and name exactly one step.
            let by_path: Vec<&serde_json::Value> = pair
                .iter()
                .copied()
                .filter(|link| link["path"].as_str() == Some(step_path))
                .collect();
            let mut chosen: Option<String> = if by_path.len() == 1 {
                by_path[0]["stepId"].as_str().map(str::to_owned)
            } else if by_path.is_empty() {
                let pair_paths = paths_per_pair
                    .get(&(assay_id.clone(), timepoint.clone()))
                    .map(|paths| paths.len())
                    .unwrap_or(0);
                let unqualified: Vec<&serde_json::Value> = pair
                    .iter()
                    .copied()
                    .filter(|link| {
                        link.get("path")
                            .and_then(|p| p.as_str())
                            .map(str::is_empty)
                            .unwrap_or(true)
                    })
                    .collect();
                if pair_paths == 1 && unqualified.len() == 1 {
                    unqualified[0]["stepId"].as_str().map(str::to_owned)
                } else {
                    None
                }
            } else {
                None
            };
            // Paths never join a block-internal protocol path to the design's
            // own step path. The producing block version does, as long as
            // exactly one design step reuses it.
            if chosen.is_none() && !source_block_version_id.is_empty() {
                let mut candidates: Vec<&str> = step_blocks
                    .iter()
                    .filter(|(_, blocks)| blocks.iter().any(|b| b == &source_block_version_id))
                    .map(|(id, _)| id.as_str())
                    .collect();
                candidates.sort_unstable();
                if candidates.len() == 1 {
                    chosen = Some(candidates[0].to_owned());
                }
            }
            chosen.unwrap_or_default()
        } else {
            steps_by_assay
                .get(&assay_id)
                .filter(|nodes| nodes.len() == 1)
                .map(|nodes| nodes[0].clone())
                .unwrap_or_default()
        };
        let (title, instruction, parameters_json, source_block_version_id) = steps_by_path
            .get(&path)
            .cloned()
            .unwrap_or_else(|| (path.clone(), String::new(), "[]".into(), source_block_version_id));
        let mut checks = checks_by_path.get(&path).cloned().unwrap_or_default();
        for entry in v["design_checks"].as_array().into_iter().flatten() {
            if entry["path"].as_str() != Some(path.as_str()) {
                continue;
            }
            for check in entry["checks"].as_array().into_iter().flatten() {
                let check_id = check["check_id"].as_str().unwrap_or("");
                if check_id.is_empty()
                    || checks.iter().any(|existing| existing.check_id == check_id)
                {
                    continue;
                }
                checks.push(TaskCheck {
                    check_id: check_id.into(),
                    label: check["label"].as_str().unwrap_or("").into(),
                    instruction: "".into(),
                    blocking: check["blocking"].as_bool().unwrap_or(false),
                    status: String::new(),
                });
            }
        }
        s.tasks.push(RunTask {
            key: t["task_key"].as_str().unwrap_or("").into(),
            title,
            path,
            timepoint,
            status: t["status"].as_str().unwrap_or("").into(),
            assay_id,
            protocol_version_id,
            design_step_id,
            instruction,
            parameters_json,
            source_block_version_id,
            checks,
        });
    }
    // Apply only formal CheckAck events. Free-form execution_event checks are
    // observations, not authoritative acknowledgements.
    for task in &mut s.tasks {
        for check in &mut task.checks {
            let mut best: Option<(String, String)> = None;
            for e in v["events"].as_array().into_iter().flatten() {
                if !e["check_ack"].as_bool().unwrap_or(false) {
                    continue;
                }
                let matches = e["task_key"].as_str() == Some(task.key.as_str())
                    || e["protocol_step_path"].as_str() == Some(task.path.as_str());
                if !matches {
                    continue;
                }
                let at = e["recorded_at"].as_str()
                    .or_else(|| e["occurred_at"].as_str())
                    .unwrap_or("")
                    .to_owned();
                for ack in e["checks"].as_array().into_iter().flatten() {
                    if ack["check_id"].as_str() != Some(check.check_id.as_str()) {
                        continue;
                    }
                    if best.as_ref().map_or(true, |(prev, _)| at.as_str() >= prev.as_str()) {
                        best = Some((at.clone(), ack["status"].as_str().unwrap_or("").to_owned()));
                    }
                }
            }
            if let Some((_, status)) = best {
                check.status = status;
            }
        }
    }
    for e in v["events"].as_array().into_iter().flatten() {
        s.events.push(RunEvent {
            id: e["event_id"].as_str().or_else(||e["id"].as_str()).unwrap_or("").into(),
            event_type: e["event_type"].as_str().unwrap_or("").into(),
            content: e["content"].as_str().unwrap_or("").into(),
            occurred_at: e["occurred_at"].as_str().unwrap_or("").into(),
            recorded_at: e["recorded_at"].as_str().unwrap_or("").into(),
            task_key: e["task_key"].as_str().unwrap_or("").into(),
            step_status: e["step_status"].as_str().unwrap_or("").into(),
            timepoint: e["timepoint"].as_str().unwrap_or("").into(),
            stage: e["stage"].as_str().unwrap_or("").into(),
            protocol_step_path: e["protocol_step_path"].as_str().unwrap_or("").into(),
            assay_id: e["assay_id"].as_str().unwrap_or("").into(),
            batch_id: e["batch_id"].as_str().unwrap_or("").into(),
            schema_id: e["schema_id"].as_str().unwrap_or("").into(),
            schema_version: e["schema_version"].as_u64().unwrap_or_default() as i64,
            result_cells: e["result_cells"].as_array().into_iter().flatten().map(|cell| ResultCell {
                row: cell["row"].as_u64().unwrap_or_default() as i64,
                column: cell["column"].as_u64().unwrap_or_default() as i64,
                sample_id: cell["sample_id"].as_str().unwrap_or("").into(),
                group: cell["group"].as_str().unwrap_or("").into(),
                replicate: cell["replicate"].as_u64().unwrap_or_default() as i64,
                field: cell["field"].as_str().unwrap_or("").into(),
                value: cell["value"].as_f64(),
                status: cell["status"].as_str().unwrap_or("").into(),
            }).collect(),
            checks: e["checks"].as_array().into_iter().flatten().map(|check| CheckAck {
                check_id: check["check_id"].as_str().unwrap_or("").into(),
                status: check["status"].as_str().unwrap_or("").into(),
            }).collect(),
            data_json: serde_json::to_string(e).unwrap_or_default(),
        });
        for m in e["measurements"].as_array().into_iter().flatten() {
            s.measurements.push(Measurement {
                sample_id: m["sample_id"].as_str().unwrap_or("").into(),
                endpoint: m["endpoint"].as_str().unwrap_or("").into(),
                value: m["value"].as_f64().unwrap_or(0.0),
                unit: m["unit"].as_str().unwrap_or("").into(),
                task_key: e["task_key"].as_str().unwrap_or("").into(),
                protocol_step_path: e["protocol_step_path"].as_str().unwrap_or("").into(),
            });
        }
    }
    if s.title.is_empty() {
        s.title = v["title"].as_str().unwrap_or("").into();
    }
    s
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::path::PathBuf;
    use std::sync::OnceLock;

    /// One data directory for the whole binary: LABWORKBENCH_DATA_DIR is read
    /// when a service opens, so tests must not race on it. Every test seeds its
    /// own ids, so they can share the database.
    fn data_dir() -> PathBuf {
        static DIR: OnceLock<PathBuf> = OnceLock::new();
        DIR.get_or_init(|| {
            let dir = std::env::temp_dir()
                .join(format!("asterlab-runs-module-{}", std::process::id()));
            std::fs::create_dir_all(&dir).expect("create data dir");
            std::env::set_var("LABWORKBENCH_DATA_DIR", &dir);
            dir
        })
        .clone()
    }

    struct Seeded {
        design_id: String,
        design_version_id: String,
        run_id: String,
        note_id: String,
    }

    /// Writes the minimum records `run_workspace_typed` needs: a project, a
    /// design, one immutable design version and a frozen Run. Optional note and
    /// asset link rows make the projection and archive non-empty.
    fn seed_run(with_note: bool, with_asset: bool) -> Seeded {
        let project_id = format!("proj-{}", uuid::Uuid::new_v4());
        let design_id = format!("design-{}", uuid::Uuid::new_v4());
        let design_version_id = format!("dv-{}", uuid::Uuid::new_v4());
        let run_id = format!("run-{}", uuid::Uuid::new_v4());
        let note_id = format!("note-{}", uuid::Uuid::new_v4());
        let db = cli_impl::open_db(&data_dir()).expect("open seeded db");
        ensure_module_tables(&db).expect("module tables");
        let at = "2026-11-15T09:00:00Z";
        let insert = |id: &str, kind: &str, project: Option<&str>, parent: Option<&str>, data: &serde_json::Value| {
            db.execute(
                "INSERT INTO records(id,kind,project_id,parent_id,revision,data_json,created_at,updated_at) \
                 VALUES(?1,?2,?3,?4,1,?5,?6,?6)",
                rusqlite::params![id, kind, project, parent, data.to_string(), at],
            )
            .expect("insert record");
        };
        insert(&project_id, "project", None, None, &serde_json::json!({"title": "Module tests"}));
        insert(&design_id, "design", Some(&project_id), None, &serde_json::json!({"title": "Design"}));
        insert(
            &design_version_id,
            "design_version",
            Some(&project_id),
            Some(&design_id),
            &serde_json::json!({
                "design_revision": 1,
                "snapshot": {"title": "Derived design", "objective": "compare groups"}
            }),
        );
        insert(
            &run_id,
            "run",
            Some(&project_id),
            Some(&design_id),
            &serde_json::json!({
                "design_version_id": design_version_id,
                "title": "Frozen run",
                "status": "in_progress",
                "protocol_manifest": [],
                "material_manifest": []
            }),
        );
        if with_note {
            insert(
                &note_id,
                "note",
                Some(&project_id),
                Some(&run_id),
                &serde_json::json!({"content": "Observed confluency at 80%."}),
            );
        }
        if with_asset {
            insert(
                &format!("asset-link-{}", uuid::Uuid::new_v4()),
                "asset_link",
                Some(&project_id),
                Some(&run_id),
                &serde_json::json!({
                    "id": format!("asset-{}", uuid::Uuid::new_v4()),
                    "rootId": "project-data",
                    "relativePath": "huvec/d1/image_01.tif",
                    "mediaType": "image",
                    "size": 12,
                    "modTime": "2026-01-01T00:00:00Z",
                    "sha256": "a".repeat(64),
                    "availability": "available",
                    "link": {
                        "runId": run_id,
                        "taskKey": "task-1",
                        "sampleId": null,
                        "wellId": "A1",
                        "timepoint": null,
                        "fovIndex": null,
                        "coordinateId": "A1"
                    },
                    "linkedAt": "2026-01-01T00:00:00Z",
                    "actorId": "tester"
                }),
            );
        }
        Seeded { design_id, design_version_id, run_id, note_id }
    }

    fn service() -> CoreRunService {
        let core = AppCore::open(&data_dir()).expect("open core");
        ensure_module_tables(core.raw_db()).expect("module tables");
        CoreRunService { core: Mutex::new(Some(core)) }
    }

    fn design_service() -> CoreDesignService {
        let db = cli_impl::open_db(&data_dir()).expect("open design db");
        ensure_module_tables(&db).expect("module tables");
        CoreDesignService { db: Mutex::new(Some(db)) }
    }

    fn projection_json(run_id: &str) -> serde_json::Value {
        let raw = service()
            .record_projection_impl(&RecordProjectionInput {
                run_id: run_id.to_owned(),
                view: String::new(),
                fov_per_well: 1,
                now_utc: String::new(),
            })
            .expect("record projection");
        serde_json::from_str(&raw).expect("projection JSON")
    }

    #[test]
    fn runtime_normalize_bridges_the_live_mix() {
        let output = service()
            .normalize_runtime_impl(&RuntimeNormalizeInput {
                planned_wells: 9,
                actual_wells: 10,
                volume_per_well_ul: 200.0,
                excess_ratio: 0.1,
                dead_volume_ul: 50.0,
                prepared_volume_ul: 2030.0,
            })
            .expect("normalize");
        assert!((output.planned_volume_ul - 2030.0).abs() < 1e-9);
        assert!((output.required_volume_ul - 2250.0).abs() < 1e-9);
        assert!((output.additional_volume_ul - 220.0).abs() < 1e-9);
        assert!(service()
            .normalize_runtime_impl(&RuntimeNormalizeInput {
                planned_wells: 9,
                actual_wells: 9,
                volume_per_well_ul: f64::NAN,
                ..RuntimeNormalizeInput::default()
            })
            .is_err());
    }

    #[test]
    fn pantry_round_trip_keeps_label_codes_stable() {
        let svc = service();
        let item = svc
            .upsert_pantry_impl(PantryUpsert {
                name: "Trypsin-EDTA".into(),
                location: "fridge-2".into(),
                purchased_on: "2026-01-01".into(),
                opened_on: "2026-06-01".into(),
                shelf_life_days: Some(365),
                ..PantryUpsert::default()
            })
            .expect("upsert pantry");
        assert!(item.label_code.starts_with("PN-"), "label code {}", item.label_code);
        assert_eq!(item.revision, 1);

        let ledger = svc
            .list_pantry_impl(&PantryQuery {
                as_of: "2026-06-25".into(),
                ..PantryQuery::default()
            })
            .expect("list pantry");
        assert_eq!(ledger.as_of, "2026-06-25");
        let listed = ledger.items.iter().find(|row| row.id == item.id).expect("item listed");
        assert_eq!(listed.status, "opened");
        assert_eq!(listed.expires_on, "2027-01-01");

        let labels = svc
            .export_pantry_labels_impl(&PantryQuery {
                as_of: "2026-06-25".into(),
                ..PantryQuery::default()
            })
            .expect("export labels");
        assert_eq!(labels.schema_version, 1);
        assert!(labels.labels.iter().any(|label| label.pantry_item_id == item.id));

        let deleted = svc
            .delete_pantry_impl(PantryDelete {
                id: item.id.clone(),
                expected_revision: Some(item.revision),
            })
            .expect("delete pantry");
        assert!(deleted.deleted);
        assert_eq!(deleted.revision, 2);
        assert!(svc
            .upsert_pantry_impl(PantryUpsert {
                id: item.id,
                name: "Trypsin-EDTA".into(),
                ..PantryUpsert::default()
            })
            .is_err(), "a retired item must not be revived");
    }

    #[test]
    fn timer_lifecycle_rejects_unknown_actions_and_records_elapsed_time() {
        let seeded = seed_run(false, false);
        let svc = service();
        let timer = svc
            .save_timer_impl(TimerRecord {
                timer_id: "timer-1".into(),
                run_id: seeded.run_id.clone(),
                task_key: "readout".into(),
                mode: "countdown".into(),
                status: "ready".into(),
                planned_s: Some(600),
                ..TimerRecord::default()
            })
            .expect("save timer");
        assert_eq!(timer.status, "ready");

        let started = svc
            .update_timer_impl(TimerActionInput {
                timer: timer.clone(),
                action: "start".into(),
                now_utc: "2026-11-15T09:00:00Z".into(),
            })
            .expect("start timer");
        assert_eq!(started.status, "running");
        assert_eq!(started.started_at_utc.as_deref(), Some("2026-11-15T09:00:00Z"));

        let paused = svc
            .update_timer_impl(TimerActionInput {
                timer: started,
                action: "pause".into(),
                now_utc: "2026-11-15T09:10:00Z".into(),
            })
            .expect("pause timer");
        assert_eq!(paused.status, "paused");
        assert_eq!(paused.accumulated_s, 600);

        let listed = svc.list_timers_impl(&seeded.run_id).expect("list timers");
        assert_eq!(listed.len(), 1);
        assert_eq!(listed[0].accumulated_s, 600);

        assert!(svc
            .update_timer_impl(TimerActionInput {
                timer: paused,
                action: "warp".into(),
                now_utc: "2026-11-15T09:10:00Z".into(),
            })
            .is_err());
    }

    #[test]
    fn record_projection_exposes_notes_and_the_recording_state() {
        let seeded = seed_run(true, true);
        let projection = projection_json(&seeded.run_id);
        assert_eq!(projection["run_id"], seeded.run_id);
        assert_eq!(projection["schema_version"], 1);
        assert_eq!(projection["archive"]["status"], "recording");
        assert_eq!(projection["notes_locked"], false);
        let notes = projection["notes"].as_array().expect("notes array");
        assert_eq!(notes.len(), 1);
        assert_eq!(notes[0]["note_id"], seeded.note_id);
        assert_eq!(notes[0]["markdown"], "Observed confluency at 80%.");
        assert_eq!(projection["assets"].as_array().map(Vec::len), Some(1));
    }

    #[test]
    fn sign_and_archive_is_terminal_and_fingerprints_every_member() {
        let seeded = seed_run(true, true);
        let svc = service();
        let archived = svc
            .sign_and_archive_run_impl(&SignAndArchiveInput {
                run_id: seeded.run_id.clone(),
                view: String::new(),
                fov_per_well: 1,
                now_utc: String::new(),
                signed_by: "operator".into(),
                signed_at_utc: "2026-11-15T10:00:00Z".into(),
                statement: "Run complete".into(),
            })
            .expect("sign and archive");
        assert_eq!(archived.status, "archived");
        assert_eq!(archived.signed_by, "operator");
        assert!(archived.notes_locked);
        let manifest: serde_json::Value =
            serde_json::from_str(&archived.manifest_json).expect("manifest JSON");
        assert_eq!(manifest["run_id"], seeded.run_id);
        assert_eq!(manifest["design_version_id"], seeded.design_version_id);
        let paths: Vec<&str> = manifest["entries"]
            .as_array()
            .expect("entries")
            .iter()
            .filter_map(|entry| entry["logical_path"].as_str())
            .collect();
        assert!(paths.contains(&"projection.json"));
        assert!(paths.contains(&"project-data/huvec/d1/image_01.tif"));
        assert!(paths.contains(&format!("notes/{}.md", seeded.note_id).as_str()));

        let projection = projection_json(&seeded.run_id);
        assert_eq!(projection["archive"]["status"], "archived");
        assert_eq!(projection["notes_locked"], true);

        assert!(svc
            .sign_and_archive_run_impl(&SignAndArchiveInput {
                run_id: seeded.run_id,
                signed_by: "operator".into(),
                signed_at_utc: "2026-11-15T10:05:00Z".into(),
                ..SignAndArchiveInput::default()
            })
            .is_err(), "an archived Run is terminal");
    }

    #[test]
    fn derive_design_carries_measured_drift_without_writing() {
        let seeded = seed_run(false, false);
        let run_service = service();
        run_service
            .save_timer_impl(TimerRecord {
                timer_id: "timer-derive".into(),
                run_id: seeded.run_id.clone(),
                task_key: "readout".into(),
                mode: "countdown".into(),
                status: "paused".into(),
                planned_s: Some(300),
                accumulated_s: 600,
                ..TimerRecord::default()
            })
            .expect("save timer");

        let design_service = design_service();
        let derived = design_service
            .derive_design_from_run_impl(&DeriveDesignInput {
                run_id: seeded.run_id.clone(),
                title: String::new(),
                objective: String::new(),
                note: "measured overrun".into(),
            })
            .expect("derive design");
        assert_eq!(derived.design_id, seeded.design_id);
        assert_eq!(derived.source_design_version_id, seeded.design_version_id);
        assert_eq!(derived.source_run_id, seeded.run_id);
        assert_eq!(derived.expected_revision, 1);
        assert_eq!(derived.note, "measured overrun");
        let design: serde_json::Value = serde_json::from_str(&derived.design_json).expect("design JSON");
        assert_eq!(design["title"], "Derived design");
        assert_eq!(design["derived_from"]["run_id"], seeded.run_id);
        assert_eq!(design["derived_from"]["duration_updates"][0]["actual_s"], 600);
        // Pure derivation: the frozen Run and Design stay untouched.
        let db = cli_impl::open_db(&data_dir()).expect("open db");
        let revision: i64 = db
            .query_row(
                "SELECT revision FROM records WHERE id=?1",
                [&seeded.run_id],
                |row| row.get(0),
            )
            .expect("run revision");
        assert_eq!(revision, 1);
    }

    fn time_budget_of(specs: serde_json::Value) -> TimeBudget {
        project_time_budget_json(&specs.to_string()).expect("time budget")
    }

    /// One hands-on step is both span and hands-on time; nothing is a window.
    #[test]
    fn time_budget_single_step_is_hands_on() {
        let budget = time_budget_of(serde_json::json!([
            {
                "schema_version": 2,
                "operation": { "kind": "measure", "endpoint_id": "od450", "unit": "AU" },
                "timing": { "kind": "hands_on", "duration": { "value": 5.0, "unit": "min" } }
            }
        ]));
        assert_eq!(budget.total_span_s, 300.0);
        assert_eq!(budget.hands_on_s, 300.0);
        assert!(budget.parallel_windows.is_empty());
    }

    /// The card's case: `(80°C 5m -> 冰 10m) × 3` is 45 minutes of span with 15
    /// hands-on. Repeat multiplies one occurrence, so the waits stay one window.
    #[test]
    fn time_budget_repeat_count_expands_each_occurrence() {
        let budget = time_budget_of(serde_json::json!([
            {
                "schema_version": 2,
                "operation": { "kind": "incubate",
                    "temperature": { "value": 80.0, "unit": "C" },
                    "duration": { "value": 5.0, "unit": "min" } },
                "timing": { "kind": "hands_on", "duration": { "value": 5.0, "unit": "min" } },
                "repeat": { "count": 3 }
            },
            {
                "schema_version": 2,
                "operation": { "kind": "incubate",
                    "temperature": { "value": -20.0, "unit": "C" },
                    "duration": { "value": 10.0, "unit": "min" } },
                "timing": { "kind": "wait", "duration": { "value": 10.0, "unit": "min" } },
                "repeat": { "count": 3 }
            }
        ]));
        assert_eq!(budget.total_span_s, 2_700.0);
        assert_eq!(budget.hands_on_s, 900.0);
        assert_eq!(
            budget.parallel_windows,
            vec![TimeBudgetWindow {
                step_index: 1,
                start_s: 900.0,
                end_s: 2_700.0,
                duration_s: 1_800.0,
                time_anchor_id: String::new(),
            }]
        );
    }

    /// A `wait >= 2h` occurrence is a window with its anchor, not hands-on time.
    #[test]
    fn time_budget_wait_creates_a_parallel_window() {
        let budget = time_budget_of(serde_json::json!([
            {
                "schema_version": 2,
                "operation": { "kind": "measure", "endpoint_id": "capture", "unit": "count" },
                "timing": { "kind": "hands_on", "duration": { "value": 30.0, "unit": "s" } }
            },
            {
                "schema_version": 2,
                "operation": { "kind": "incubate",
                    "temperature": { "value": 4.0, "unit": "C" },
                    "duration": { "value": 2.0, "unit": "h" } },
                "timing": { "kind": "wait", "duration": { "value": 2.0, "unit": "h" } },
                "time_anchor_id": "cold_start"
            }
        ]));
        assert_eq!(budget.total_span_s, 7_230.0);
        assert_eq!(budget.hands_on_s, 30.0);
        assert_eq!(
            budget.parallel_windows,
            vec![TimeBudgetWindow {
                step_index: 1,
                start_s: 30.0,
                end_s: 7_230.0,
                duration_s: 7_200.0,
                time_anchor_id: "cold_start".into(),
            }]
        );
    }

    /// A step without `timing` adds no duration: the projection never invents
    /// one, and the remaining hands-on step still measures.
    #[test]
    fn time_budget_missing_timing_adds_no_duration() {
        let budget = time_budget_of(serde_json::json!([
            {
                "schema_version": 2,
                "operation": { "kind": "measure", "endpoint_id": "od450", "unit": "AU" },
                "timing": { "kind": "hands_on", "duration": { "value": 1.0, "unit": "min" } }
            },
            {
                "schema_version": 2,
                "operation": { "kind": "incubate", "temperature": { "value": 37.0, "unit": "C" } }
            }
        ]));
        assert_eq!(budget.total_span_s, 60.0);
        assert_eq!(budget.hands_on_s, 60.0);
        assert!(budget.parallel_windows.is_empty());
    }

    /// The input is the OperationSpec array, so a malformed document is an
    /// INVALID error and never a silently empty budget.
    #[test]
    fn time_budget_rejects_documents_that_are_not_operation_specs() {
        let error = project_time_budget_json(r#"[{"schema_version":2,"operation":{"kind":"measure"}}]"#)
            .expect_err("missing endpoint_id");
        match error {
            Error::Go(message) => {
                assert!(message.starts_with("INVALID: "), "{message}");
                assert!(message.contains("endpoint_id"), "{message}");
            }
            other => panic!("unexpected error {other:?}"),
        }
    }
}
