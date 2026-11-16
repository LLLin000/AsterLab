#![recursion_limit = "256"]
mod mygo;
use mygo::*;
use std::path::PathBuf;
use std::sync::{Arc, Mutex};

#[path = "../../../../../../src/cli.rs"]
mod cli_impl;

use asterlab_app::AppCore;
use asterlab_core::assets::{AssetLink as CoreAssetLink, AssetRef};
use asterlab_core::calculators::{
    plate_mix_plan, runtime_normalize, PlateMixCellPlan as CorePlateMixCellPlan,
    PlateMixGelPercent as CorePlateMixGelPercent, PlateMixGelPlan as CorePlateMixGelPlan,
    PlateMixGroup as CorePlateMixGroup, PlateMixInput as CorePlateMixInput,
    RuntimeNormalizeInput as CoreNormalizeInput,
};
use asterlab_core::design_commands::{derive_design_from_run, DeriveDesignCommand};
use asterlab_core::operation_spec::OperationSpec;
use asterlab_core::protocol_preview::project_time_budget as core_time_budget;
use asterlab_core::record_projection::{
    build_record_projection, sign_and_archive, summarize_record, ArchiveSignature, ProjectionView,
    RecordArchiveState, RecordCommandError, RecordNote, RecordProjectionInput as CoreProjectionInput,
    RecordStatus,
};
use asterlab_core::timer::{
    TimerMode, TimerPhase as CoreTimerPhase, TimerRecord as CoreTimerRecord,
    TimerRound as CoreTimerRound, TimerSequence, TimerStatus,
};
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

/// The plate mix plan is Core's pure function: no data directory is opened,
/// nothing is locked and nothing is stored.
pub fn make_mix_plan_service() -> Result<Arc<dyn MixPlanService>, Error> {
    Ok(Arc::new(CoreMixPlanService))
}

struct CoreMixPlanService;

impl MixPlanService for CoreMixPlanService {
    fn plate_mix_plan(
        &self,
        _ctx: &Context,
        input: PlateMixInput,
    ) -> Result<PlateMixResult, Error> {
        plate_mix_plan_impl(&input)
    }
}

/// Map the generated bridge input onto Core's calculator and back. Core owns
/// every formula, default and rejection; nothing is filled in here.
fn plate_mix_plan_impl(input: &PlateMixInput) -> Result<PlateMixResult, Error> {
    let output = plate_mix_plan(&CorePlateMixInput {
        volume_per_well_ul: input.volume_per_well_ul,
        excess_ratio: input.excess_ratio,
        gel: input.gel.as_ref().map(|plan| CorePlateMixGelPlan {
            lap_stock_percent_wv: plan.lap_stock_percent_wv,
            lap_final_percent_wv: plan.lap_final_percent_wv,
            percent_by_group: plan
                .percent_by_group
                .iter()
                .map(|entry| CorePlateMixGelPercent {
                    name: entry.name.clone(),
                    percent: entry.percent,
                })
                .collect(),
        }),
        cells: input.cells.as_ref().map(|plan| CorePlateMixCellPlan {
            density_per_well: plan.density_per_well,
            factor_stock_mg_per_ml: plan.factor_stock_mg_per_ml,
            factor_final_mg_per_ml: plan.factor_final_mg_per_ml,
        }),
        groups: input
            .groups
            .iter()
            .map(|group| CorePlateMixGroup {
                name: group.name.clone(),
                wells: group.wells,
            })
            .collect(),
    })
    .map_err(|e| Error::Go(format!("INVALID: {e}")))?;
    Ok(PlateMixResult {
        gel: output.gel.map(|gel| PlateMixGelResult {
            groups: gel
                .groups
                .into_iter()
                .map(|group| PlateMixGelGroup {
                    name: group.name,
                    wells: group.wells,
                    gelma_percent: group.gelma_percent_wv,
                    volume_ul: group.volume_ul,
                    gelma_mass_mg: group.gelma_mass_mg,
                    lap_stock_volume_ul: group.lap_stock_volume_ul,
                    solvent_volume_ul: group.solvent_volume_ul,
                })
                .collect(),
            volume_ul: gel.volume_ul,
            gelma_mass_mg: gel.gelma_mass_mg,
            lap_stock_volume_ul: gel.lap_stock_volume_ul,
            solvent_volume_ul: gel.solvent_volume_ul,
        }),
        cells: output.cells.map(|cells| PlateMixCellResult {
            groups: cells
                .groups
                .into_iter()
                .map(|group| PlateMixCellGroup {
                    name: group.name,
                    wells: group.wells,
                    total_cells: group.total_cells,
                    suspension_volume_ul: group.suspension_volume_ul,
                    factor_stock_volume_ul: group.factor_stock_volume_ul,
                })
                .collect(),
            total_cells: cells.total_cells,
            suspension_volume_ul: cells.suspension_volume_ul,
            factor_stock_volume_ul: cells.factor_stock_volume_ul,
        }),
    })
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

    fn create_execution_event(&self, _ctx: &Context, input: ExecutionEventInput) -> Result<ExecutionEventResult, Error> {
        self.create_execution_event_impl(input)
    }

    fn execute_command(&self, _ctx: &Context, input: ExecuteCommandInput) -> Result<ExecuteCommandResult, Error> {
        let mut guard = self.core.lock().map_err(|_| Error::Go("INTERNAL: core lock poisoned".into()))?;
        let core = guard.as_mut().ok_or_else(|| Error::Go("INTERNAL: closed".into()))?;
        let payload: serde_json::Value = serde_json::from_str(&input.payload_json)
            .map_err(|e| Error::Go(format!("INVALID: payload_json 不是合法 JSON：{e}")))?;
        let expected = input.expected_revision;
        // expected_revision is enforced exactly when the underlying command
        // revises a versioned record; append-only commands ignore it.
        let mut payload = payload;
        if matches!(input.kind.as_str(), "run.revise" | "run.set_status") {
            let Some(expected) = expected else {
                return Err(Error::Go("INVALID: expected_revision is required for this command".into()));
            };
            payload["expected_revision"] = serde_json::json!(expected as i64);
        }
        if input.kind == "run.create" && payload.get("operation_id").is_none() {
            payload["operation_id"] = serde_json::json!(input.idempotency_key);
        }
        let out = core.execute_command(&input.kind, payload, &input.idempotency_key).map_err(map_envelope_error)?;
        Ok(ExecuteCommandResult {
            result_json: out["record"].to_string(),
            replayed: out.get("replayed").and_then(|v| v.as_bool()).unwrap_or(false),
        })
    }

    fn query_projection(&self, _ctx: &Context, input: QueryProjectionInput) -> Result<QueryProjectionResult, Error> {
        let mut guard = self.core.lock().map_err(|_| Error::Go("INTERNAL: core lock poisoned".into()))?;
        let core = guard.as_mut().ok_or_else(|| Error::Go("INTERNAL: closed".into()))?;
        let params: serde_json::Value = serde_json::from_str(&input.params_json)
            .map_err(|e| Error::Go(format!("INVALID: params_json 不是合法 JSON：{e}")))?;
        let out = core.query_projection(&input.kind, params).map_err(map_envelope_error)?;
        Ok(QueryProjectionResult { result_json: out.to_string() })
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

    fn unlink_asset_link(&self, _ctx: &Context, run_id: String, asset_id: String) -> Result<Asset, Error> {
        self.unlink_asset_link_impl(&run_id, &asset_id)
    }

    fn update_asset_link_coordinates(
        &self,
        _ctx: &Context,
        run_id: String,
        asset_id: String,
        well_id: String,
        timepoint: String,
        fov_index: u32,
    ) -> Result<Asset, Error> {
        self.update_asset_link_coordinates_impl(&run_id, &asset_id, &well_id, &timepoint, fov_index)
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

    fn update_timer(&self, _ctx: &Context, input: TimerActionInput) -> Result<TimerUpdateResult, Error> {
        self.update_timer_impl(input)
    }

    fn list_timers(&self, _ctx: &Context, run_id: String) -> Result<Vec<TimerRecord>, Error> {
        self.list_timers_impl(&run_id)
    }

    fn list_active_timers(&self, _ctx: &Context, now_utc: String) -> Result<Vec<ActiveTimer>, Error> {
        self.list_active_timers_impl(&now_utc)
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
    /// Removes one Run's link for an asset. `assetID` is Core's durable asset
    /// id, so the link record is resolved inside the Run first; the CLI's
    /// `asset.unlink` then does the delete and the `unlinked` event in one
    /// transaction, and the file is never touched.
    fn unlink_asset_link_impl(&self, run_id: &str, asset_id: &str) -> Result<Asset, Error> {
        let mut guard = self.core.lock().map_err(|_| Error::Go("INTERNAL: core lock poisoned".into()))?;
        let core = guard.as_mut().ok_or_else(|| Error::Go("INTERNAL: closed".into()))?;
        let record_id = asset_link_record_id(core.raw_db(), run_id, asset_id)?;
        let (removed, _event) =
            cli_impl::unlink_asset_link(core.raw_db_mut(), &record_id, "").map_err(map_cli)?;
        Ok(asset_from_value(&removed["data"]))
    }

    /// Replaces one link's coordinate triple through `asset.link.update`, so
    /// Core keeps the single copy of the coordinate rules, the record identity
    /// and the `updated` event. Go has no absent value: an empty timepoint and
    /// a FOV index of 0 are "not recorded" and become an explicit null, while
    /// both keys are always supplied.
    fn update_asset_link_coordinates_impl(
        &self,
        run_id: &str,
        asset_id: &str,
        well_id: &str,
        timepoint: &str,
        fov_index: u32,
    ) -> Result<Asset, Error> {
        let mut guard = self.core.lock().map_err(|_| Error::Go("INTERNAL: core lock poisoned".into()))?;
        let core = guard.as_mut().ok_or_else(|| Error::Go("INTERNAL: closed".into()))?;
        let record_id = asset_link_record_id(core.raw_db(), run_id, asset_id)?;
        let request = cli_impl::AssetLinkUpdateInput {
            id: record_id,
            well_id: well_id.to_owned(),
            timepoint: Some(opt_string(timepoint)),
            fov_index: Some(if fov_index == 0 { None } else { Some(fov_index) }),
            actor_id: None,
        };
        let (record, _event) =
            cli_impl::update_asset_link(core.raw_db_mut(), &request).map_err(map_cli)?;
        Ok(asset_from_value(&record["data"]))
    }

    /// Append one event through Core's own `execution_event.create` writer and
    /// read the Run back inside the same transaction, so the event, the Run
    /// status refresh and the projection commit together or not at all. Core
    /// keeps every freeze rule (step, sample, attempt, pantry reference) and
    /// its completed-step idempotency; no second fact table is involved.
    fn create_execution_event_impl(&self, input: ExecutionEventInput) -> Result<ExecutionEventResult, Error> {
        let mut guard = self.core.lock().map_err(|_| Error::Go("INTERNAL: core lock poisoned".into()))?;
        let core = guard.as_mut().ok_or_else(|| Error::Go("INTERNAL: closed".into()))?;
        let event = execution_event_payload(&input)?;
        let db = core.raw_db();
        let tx = db.unchecked_transaction().map_err(|e| Error::Go(format!("INTERNAL: {e}")))?;
        // A pantry reference is resolved before the write, as the CLI does: an
        // unknown or retired item is NOT_FOUND and nothing is appended.
        let notice = pantry_event_notice(&tx, &input.pantry_item_id)?;
        cli_impl::create_execution_event_tx(&tx, &input.run_id, &event).map_err(map_cli)?;
        let mut value = cli_impl::run_workspace(&tx, &input.run_id).map_err(map_cli)?;
        let revision: i64 = tx
            .query_row(
                "SELECT COUNT(*) FROM records WHERE kind='execution_event' AND parent_id=?1",
                [&input.run_id],
                |row| row.get(0),
            )
            .map_err(|e| Error::Go(format!("INTERNAL: {e}")))?;
        value["revision"] = serde_json::json!(revision);
        tx.commit().map_err(|e| Error::Go(format!("INTERNAL: {e}")))?;
        Ok(ExecutionEventResult {
            snapshot: snapshot(value),
            pantry_notices: notice.into_iter().collect(),
        })
    }

    fn save_timer_impl(&self, timer: TimerRecord) -> Result<TimerRecord, Error> {
        let mut guard = self.core.lock().map_err(|_| Error::Go("INTERNAL: core lock poisoned".into()))?;
        let core = guard.as_mut().ok_or_else(|| Error::Go("INTERNAL: closed".into()))?;
        let record = timer_to_core(&timer)?;
        core.save_timer_record(&record).map_err(Error::Go)?;
        Ok(timer)
    }

    /// Applies one lifecycle action and, for an advance, returns the completed
    /// round as a fact. The phase plan is supplied by the caller (the record
    /// keeps only the `phase_index`/`round_index` cursor), so a timer created
    /// before sequences existed passes no phases and keeps its single phase.
    fn update_timer_impl(&self, input: TimerActionInput) -> Result<TimerUpdateResult, Error> {
        let mut guard = self.core.lock().map_err(|_| Error::Go("INTERNAL: core lock poisoned".into()))?;
        let core = guard.as_mut().ok_or_else(|| Error::Go("INTERNAL: closed".into()))?;
        let mut timer = timer_to_core(&input.timer)?;
        let round = match input.action.as_str() {
            "start" => timer.start(&input.now_utc).map(|()| None),
            "pause" => timer.pause(&input.now_utc).map(|()| None),
            "advance" => {
                let sequence = timer_sequence(&input.phases, &timer)?;
                timer.advance_countdown(&sequence, &input.now_utc)
            }
            "" => Err("action is required: start, pause or advance".to_owned()),
            other => Err(format!("unsupported timer action {other}; use start, pause or advance")),
        }
        .map_err(|message| Error::Go(format!("INVALID: {message}")))?;
        core.save_timer_record(&timer).map_err(Error::Go)?;
        Ok(TimerUpdateResult {
            timer: timer_from_core(&timer),
            completed: round.is_some(),
            round: round.as_ref().map(round_from_core).unwrap_or_default(),
        })
    }

    fn list_timers_impl(&self, run_id: &str) -> Result<Vec<TimerRecord>, Error> {
        let guard = self.core.lock().map_err(|_| Error::Go("INTERNAL: core lock poisoned".into()))?;
        let core = guard.as_ref().ok_or_else(|| Error::Go("INTERNAL: closed".into()))?;
        let timers = core.load_timer_records(run_id).map_err(Error::Go)?;
        Ok(timers.iter().map(timer_from_core).collect())
    }

    /// Every running timer in one query. The rows are the ones SaveTimer and
    /// UpdateTimer write, decoded with the same Core type, so the global bar
    /// never sees a second serialization. Remaining time is measured at
    /// `now_utc`; a timer with no planned duration sorts after the countdowns.
    fn list_active_timers_impl(&self, now_utc: &str) -> Result<Vec<ActiveTimer>, Error> {
        let guard = self.core.lock().map_err(|_| Error::Go("INTERNAL: core lock poisoned".into()))?;
        let core = guard.as_ref().ok_or_else(|| Error::Go("INTERNAL: closed".into()))?;
        let db = core.raw_db();
        let mut statement = db
            .prepare(
                "SELECT t.data_json, r.data_json FROM timer_records t \
                 JOIN records r ON r.id=t.run_id AND r.kind='run'",
            )
            .map_err(|e| Error::Go(format!("INTERNAL: {e}")))?;
        let rows = statement
            .query_map([], |row| {
                Ok((row.get::<_, String>(0)?, row.get::<_, String>(1)?))
            })
            .map_err(|e| Error::Go(format!("INTERNAL: {e}")))?;
        let mut out = Vec::new();
        for row in rows {
            let (timer_raw, run_raw) = row.map_err(|e| Error::Go(format!("INTERNAL: {e}")))?;
            let timer: CoreTimerRecord = serde_json::from_str(&timer_raw)
                .map_err(|e| Error::Go(format!("INTERNAL: {e}")))?;
            if timer.status != TimerStatus::Running {
                continue;
            }
            let run: serde_json::Value =
                serde_json::from_str(&run_raw).unwrap_or_else(|_| serde_json::json!({}));
            let elapsed_s = timer
                .elapsed_s(now_utc)
                .map_err(|e| Error::Go(format!("INVALID: {e}")))?;
            let remaining_s = timer
                .remaining_s(now_utc)
                .map_err(|e| Error::Go(format!("INVALID: {e}")))?;
            out.push(ActiveTimer {
                run_id: timer.run_id.clone(),
                run_title: run["title"].as_str().unwrap_or("").to_owned(),
                timer_id: timer.timer_id.clone(),
                task_key: timer.task_key.clone(),
                mode: timer_mode_text(timer.mode),
                status: timer_status_text(timer.status),
                elapsed_s,
                remaining_s,
                time_anchor_id: timer
                    .anchor_event_id
                    .clone()
                    .or_else(|| timer.anchor_task_key.clone())
                    .unwrap_or_default(),
            });
        }
        out.sort_by(|a, b| {
            (a.remaining_s.is_none(), a.remaining_s.unwrap_or_default(), &a.run_id, &a.timer_id)
                .cmp(&(b.remaining_s.is_none(), b.remaining_s.unwrap_or_default(), &b.run_id, &b.timer_id))
        });
        Ok(out)
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

/// The stored `asset_link` record id for one Run-scoped asset id.
///
/// Core derives the record id from the file, the association and the
/// coordinates, so it is read from storage instead of recomputed here. A file
/// linked at two coordinates in the same Run has two records; that is ambiguous
/// for a call addressed by asset id alone, and guessing would update or delete
/// the wrong link.
fn asset_link_record_id(
    db: &rusqlite::Connection,
    run_id: &str,
    asset_id: &str,
) -> Result<String, Error> {
    let mut statement = db
        .prepare(
            "SELECT id, data_json FROM records \
             WHERE kind='asset_link' AND parent_id=?1 ORDER BY created_at ASC, rowid ASC",
        )
        .map_err(|e| Error::Go(format!("INTERNAL: {e}")))?;
    let rows = statement
        .query_map([run_id], |row| Ok((row.get::<_, String>(0)?, row.get::<_, String>(1)?)))
        .map_err(|e| Error::Go(format!("INTERNAL: {e}")))?;
    let mut found = Vec::new();
    for row in rows.flatten() {
        let data: serde_json::Value =
            serde_json::from_str(&row.1).unwrap_or(serde_json::Value::Null);
        if data["id"].as_str() == Some(asset_id) {
            found.push(row.0);
        }
    }
    match found.len() {
        0 => Err(Error::Go(format!(
            "NOT_FOUND: asset {asset_id} is not linked to this run"
        ))),
        1 => Ok(found.remove(0)),
        _ => Err(Error::Go(format!(
            "INVALID: asset {asset_id} is linked at more than one coordinate in this run"
        ))),
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
    TimerRecord {
        timer_id: timer.timer_id.clone(),
        run_id: timer.run_id.clone(),
        task_key: timer.task_key.clone(),
        phase_index: timer.phase_index,
        mode: timer_mode_text(timer.mode),
        planned_s: timer.planned_s,
        started_at_utc: timer.started_at_utc.clone(),
        accumulated_s: timer.accumulated_s,
        status: timer_status_text(timer.status),
        round_index: timer.round_index,
        round_count: timer.round_count,
        anchor_event_id: timer.anchor_event_id.clone(),
        anchor_task_key: timer.anchor_task_key.clone(),
    }
}

/// The phase plan for one action: the caller's explicit sequence when it sent
/// one, otherwise the single phase the record itself describes. Nothing extra is
/// stored on the timer row, so an old timer and a one-phase plan behave alike.
fn timer_sequence(phases: &[TimerPhase], timer: &CoreTimerRecord) -> Result<TimerSequence, Error> {
    if phases.is_empty() {
        return Ok(TimerSequence::from_record(timer));
    }
    let mut converted = Vec::with_capacity(phases.len());
    for phase in phases {
        converted.push(CoreTimerPhase {
            name: phase.name.clone(),
            planned_s: phase.planned_s,
            mode: timer_mode(&phase.mode)?,
            rounds: phase.rounds,
        });
    }
    TimerSequence::new(converted).map_err(|message| Error::Go(format!("INVALID: {message}")))
}

/// The completed round as the bridge reports it. Core owns the numbers; the
/// caller only forwards the fact to `execution_event.create`.
fn round_from_core(round: &CoreTimerRound) -> TimerRound {
    TimerRound {
        phase_index: round.phase_index,
        round_index: round.round_index,
        phase_name: round.phase_name.clone(),
        mode: timer_mode_text(round.mode),
        planned_s: Some(round.planned_s),
        actual_s: round.actual_s,
        started_at_utc: round.started_at_utc.clone(),
        ended_at_utc: round.ended_at_utc.clone(),
        phase_completed: round.phase_completed,
        timer_completed: round.timer_completed,
    }
}

fn timer_mode_text(mode: TimerMode) -> String {
    serde_json::to_value(mode)
        .ok()
        .and_then(|value| value.as_str().map(str::to_owned))
        .unwrap_or_default()
}

fn timer_status_text(status: TimerStatus) -> String {
    serde_json::to_value(status)
        .ok()
        .and_then(|value| value.as_str().map(str::to_owned))
        .unwrap_or_default()
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

/// The exact `execution_event.create` payload for one bridge input. Empty
/// optional fields are omitted instead of stored as empty strings, so the
/// caller never has to distinguish "not recorded" from "recorded as blank".
/// `skipped_by_rule` carries Core's mandatory `step_status=skipped` itself: the
/// event type is the datum, and the caller must not repeat it.
fn execution_event_payload(input: &ExecutionEventInput) -> Result<serde_json::Value, Error> {
    let mut event = serde_json::json!({
        "run_id": input.run_id,
        "event_type": input.event_type,
        "content": input.content,
    });
    if !input.occurred_at.is_empty() {
        event["occurred_at"] = serde_json::json!(input.occurred_at);
    }
    if !input.operator.is_empty() {
        event["operator"] = serde_json::json!(input.operator);
    }
    if !input.samples.is_empty() {
        event["sample_ids"] = serde_json::json!(input.samples);
    }
    if let Some(attempt) = input.attempt {
        event["attempt"] = serde_json::json!(attempt);
    }
    if !input.actual_parameters.is_empty() {
        let mut parameters = Vec::with_capacity(input.actual_parameters.len());
        for parameter in &input.actual_parameters {
            let value: serde_json::Value = serde_json::from_str(&parameter.value_json)
                .map_err(|e| Error::Go(format!("INVALID: 实际参数 {} 的值不是合法 JSON：{e}", parameter.key)))?;
            if value.is_null() {
                return Err(Error::Go(format!("INVALID: 实际参数 {} 的值不能为 null", parameter.key)));
            }
            let mut entry = serde_json::json!({"key": parameter.key, "value": value});
            if !parameter.unit.is_empty() {
                entry["unit"] = serde_json::json!(parameter.unit);
            }
            if !parameter.source.is_empty() {
                entry["source"] = serde_json::json!(parameter.source);
            }
            parameters.push(entry);
        }
        event["actual_parameters"] = serde_json::json!(parameters);
    }
    if !input.materials.is_empty() {
        let materials: Vec<serde_json::Value> = input
            .materials
            .iter()
            .map(|material| {
                let mut entry = serde_json::json!({"material": material.material});
                if !material.planned_output_id.is_empty() {
                    entry["planned_output_id"] = serde_json::json!(material.planned_output_id);
                }
                if let Some(amount) = material.amount {
                    entry["amount"] = serde_json::json!(amount);
                }
                if !material.unit.is_empty() {
                    entry["unit"] = serde_json::json!(material.unit);
                }
                entry
            })
            .collect();
        event["materials"] = serde_json::json!(materials);
    }
    if !input.deviation.is_empty() {
        event["deviation"] = serde_json::json!(input.deviation);
    }
    if !input.pantry_item_id.is_empty() {
        event["pantry_item_id"] = serde_json::json!(input.pantry_item_id);
    }
    if !input.timepoint.is_empty() {
        event["timepoint"] = serde_json::json!(input.timepoint);
    }
    if !input.step_path.is_empty() {
        event["protocol_step_path"] = serde_json::json!(input.step_path);
    }
    if !input.protocol_version_id.is_empty() {
        event["protocol_version_id"] = serde_json::json!(input.protocol_version_id);
    }
    if !input.assay_id.is_empty() {
        event["assay_id"] = serde_json::json!(input.assay_id);
    }
    if !input.planned_output_id.is_empty() {
        event["planned_output_id"] = serde_json::json!(input.planned_output_id);
    }
    if input.event_type == "skipped_by_rule" {
        event["step_status"] = serde_json::json!("skipped");
    }
    Ok(event)
}

/// Core's advisory for an event that named a pantry item, resolved through the
/// same ledger read `pantry.list` uses: an unknown or retired item is
/// `NOT_FOUND`, and a near-expiry or expired one adds one advisory notice.
/// The notice never blocks the event write.
fn pantry_event_notice(db: &rusqlite::Connection, item_id: &str) -> Result<Option<PantryNotice>, Error> {
    let item_id = item_id.trim();
    if item_id.is_empty() {
        return Ok(None);
    }
    let ledger = cli_impl::pantry_list(db, &cli_impl::PantryQueryInput::default()).map_err(map_cli)?;
    let item = ledger["items"]
        .as_array()
        .into_iter()
        .flatten()
        .find(|item| item["id"].as_str() == Some(item_id))
        .ok_or_else(|| Error::Go(format!("NOT_FOUND: 储藏室条目不存在：{item_id}")))?;
    let status = item["status"].as_str().unwrap_or_default();
    if !matches!(status, "near_expiry" | "expired") {
        return Ok(None);
    }
    Ok(Some(PantryNotice {
        code: "pantry_item_expiry_notice".into(),
        severity: "advisory".into(),
        pantry_item_id: item_id.to_owned(),
        label_code: text_field(item, "label_code"),
        name: text_field(item, "name"),
        status: status.to_owned(),
        expires_on: text_field(item, "expires_on"),
        days_remaining: item["days_remaining"].as_i64(),
        next_action: "核对或更换该储藏室条目；提示不阻断事件写入".into(),
    }))
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
/// Map the Bridge v2 envelope error string to a Go-typed error. The Core
/// prefix decides the Go class (`INVALID:` / `NOT_FOUND:` / `CONFLICT:` via
/// `internal/state.MapError`); the message is display-only and the caller
/// never parses it for control flow.
fn map_envelope_error(e: String) -> Error {
    let (prefix, rest) = e.split_once(':').map(|(p, r)| (p.trim(), r.trim())).unwrap_or(("INTERNAL", e.as_str()));
    match prefix {
        "INVALID_INPUT" => Error::Go(format!("INVALID: {rest}")),
        "NOT_FOUND" => Error::Go(format!("NOT_FOUND: {rest}")),
        "REVISION_CONFLICT" | "VERSION_CONFLICT" | "IDEMPOTENCY_MISMATCH" => Error::Go(format!("CONFLICT: {rest}")),
        _ => Error::Go(format!("INTERNAL: {rest}")),
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

    /// A frozen Run whose protocol manifest declares one assay step, so step
    /// references can be accepted or rejected against a real frozen list.
    fn seed_frozen_step_run(protocol_version_id: &str, assay_id: &str, step_path: &str) -> Seeded {
        let seeded = seed_run(false, false);
        let db = cli_impl::open_db(&data_dir()).expect("open db");
        let raw: String = db
            .query_row(
                "SELECT data_json FROM records WHERE id=?1",
                [&seeded.run_id],
                |row| row.get(0),
            )
            .expect("run data");
        let mut data: serde_json::Value = serde_json::from_str(&raw).expect("run JSON");
        data["protocol_manifest"] = serde_json::json!([{
            "assay_id": assay_id,
            "assay_title": "Assay",
            "protocol_version_id": protocol_version_id,
            "steps": [{
                "path": step_path,
                "use_id": "use-1",
                "title": "Incubate",
                "instruction": "Incubate the plate",
                "parameters": [],
                "unresolved_parameter_refs": [],
                "source_block_version_id": "block-1"
            }]
        }]);
        db.execute(
            "UPDATE records SET data_json=?2 WHERE id=?1",
            rusqlite::params![&seeded.run_id, data.to_string()],
        )
        .expect("update run manifest");
        seeded
    }

    fn event_count(run_id: &str) -> i64 {
        cli_impl::open_db(&data_dir())
            .expect("open db")
            .query_row(
                "SELECT COUNT(*) FROM records WHERE kind='execution_event' AND parent_id=?1",
                [run_id],
                |row| row.get(0),
            )
            .expect("count events")
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

    fn huvec_plate_mix_input() -> PlateMixInput {
        PlateMixInput {
            volume_per_well_ul: Some(200.0),
            excess_ratio: Some(0.1),
            gel: Some(PlateMixGelPlan {
                lap_stock_percent_wv: Some(0.5),
                lap_final_percent_wv: Some(0.05),
                percent_by_group: vec![
                    PlateMixGelPercent {
                        name: "3%".into(),
                        percent: Some(3.0),
                    },
                    PlateMixGelPercent {
                        name: "5%".into(),
                        percent: Some(5.0),
                    },
                    PlateMixGelPercent {
                        name: "7%".into(),
                        percent: Some(7.0),
                    },
                ],
            }),
            cells: Some(PlateMixCellPlan {
                density_per_well: Some(20_000.0),
                factor_stock_mg_per_ml: Some(0.01),
                factor_final_mg_per_ml: Some(0.00005),
            }),
            groups: vec![
                PlateMixGroup {
                    name: "3%".into(),
                    wells: 3,
                },
                PlateMixGroup {
                    name: "5%".into(),
                    wells: 3,
                },
                PlateMixGroup {
                    name: "7%".into(),
                    wells: 3,
                },
            ],
        }
    }

    #[test]
    fn plate_mix_plan_bridges_the_huvec_plate() {
        let result = plate_mix_plan_impl(&huvec_plate_mix_input()).expect("plate mix plan");
        let gel = result.gel.expect("gel plan");
        assert_eq!(gel.groups.len(), 3);
        assert!((gel.groups[0].volume_ul - 660.0).abs() < 1e-9);
        assert!((gel.groups[0].gelma_mass_mg - 19.8).abs() < 1e-9);
        assert!((gel.groups[0].lap_stock_volume_ul - 66.0).abs() < 1e-9);
        assert!((gel.groups[0].solvent_volume_ul - 594.0).abs() < 1e-9);
        assert!((gel.volume_ul - 1980.0).abs() < 1e-9);
        assert!((gel.gelma_mass_mg - 99.0).abs() < 1e-9);
        assert!((gel.lap_stock_volume_ul - 198.0).abs() < 1e-9);
        assert!((gel.solvent_volume_ul - 1782.0).abs() < 1e-9);

        let cells = result.cells.expect("cell plan");
        assert_eq!(cells.groups.len(), 3);
        assert_eq!(cells.groups[0].total_cells, 66_000);
        assert_eq!(cells.total_cells, 198_000);
        assert!((cells.suspension_volume_ul - 1980.0).abs() < 1e-9);
        assert!((cells.factor_stock_volume_ul - 9.9).abs() < 1e-9);
    }

    #[test]
    fn plate_mix_plan_bridges_the_half_millilitre_plate() {
        let result = plate_mix_plan_impl(&PlateMixInput {
            volume_per_well_ul: Some(500.0),
            ..huvec_plate_mix_input()
        })
        .expect("0.5 mL plate mix plan");
        let gel = result.gel.expect("gel plan");
        assert!((gel.groups[0].volume_ul - 1650.0).abs() < 1e-9);
        assert!((gel.groups[2].gelma_mass_mg - 115.5).abs() < 1e-9);
        assert!((gel.volume_ul - 4950.0).abs() < 1e-9);
        assert!((gel.lap_stock_volume_ul - 495.0).abs() < 1e-9);
        let cells = result.cells.expect("cell plan");
        assert_eq!(cells.total_cells, 198_000);
        assert!((cells.suspension_volume_ul - 4950.0).abs() < 1e-9);
    }

    #[test]
    fn plate_mix_plan_returns_core_errors() {
        let lap_above_stock = plate_mix_plan_impl(&PlateMixInput {
            gel: Some(PlateMixGelPlan {
                lap_final_percent_wv: Some(0.6),
                ..huvec_plate_mix_input().gel.expect("gel plan")
            }),
            ..huvec_plate_mix_input()
        })
        .expect_err("lap final above stock");
        assert!(
            format!("{lap_above_stock:?}")
                .contains("mix_lap_final_percent cannot exceed mix_lap_stock_percent"),
            "unexpected error: {lap_above_stock:?}"
        );

        let volume_without_plan = plate_mix_plan_impl(&PlateMixInput {
            gel: None,
            cells: None,
            ..huvec_plate_mix_input()
        })
        .expect_err("volume without a plan");
        assert!(
            format!("{volume_without_plan:?}").contains("need a gel or cell plan"),
            "unexpected error: {volume_without_plan:?}"
        );
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

    /// A skipped step is one appended event, not a second status column: the
    /// frozen step, the samples, the attempt and the actual parameters all come
    /// back from the same write.
    #[test]
    fn skipped_by_rule_event_is_appended_and_projects_a_skipped_step() {
        let seeded = seed_frozen_step_run("pv-1", "assay-1", "01-incubate");
        let svc = service();
        let result = svc
            .create_execution_event_impl(ExecutionEventInput {
                run_id: seeded.run_id.clone(),
                event_type: "skipped_by_rule".into(),
                content: "Confluency below 70%, rule R3 skips the passage".into(),
                samples: vec!["S1".into()],
                attempt: Some(1),
                actual_parameters: vec![EventActualParameter {
                    key: "confluency_pct".into(),
                    value_json: "62".into(),
                    unit: "%".into(),
                    source: "microscope".into(),
                }],
                step_path: "01-incubate".into(),
                protocol_version_id: "pv-1".into(),
                assay_id: "assay-1".into(),
                operator: "operator-1".into(),
                ..ExecutionEventInput::default()
            })
            .expect("append skipped_by_rule event");

        assert!(result.pantry_notices.is_empty(), "no pantry reference, no advisory");
        let event = result
            .snapshot
            .events
            .iter()
            .find(|event| event.event_type == "skipped_by_rule")
            .expect("the skip is in the read model");
        assert_eq!(event.step_status, "skipped", "Core's mandatory skip status");
        assert_eq!(event.protocol_step_path, "01-incubate");
        assert_eq!(event.task_key, "");
        let data: serde_json::Value = serde_json::from_str(&event.data_json).expect("event JSON");
        assert_eq!(data["sample_ids"], serde_json::json!(["S1"]));
        assert_eq!(data["attempt"], 1);
        assert_eq!(data["actual_parameters"][0]["value"], 62);
        assert_eq!(data["actual_parameters"][0]["unit"], "%");
        assert_eq!(data["actual_parameters"][0]["source"], "microscope");
        let task = result
            .snapshot
            .tasks
            .iter()
            .find(|task| task.path == "01-incubate")
            .expect("the frozen step is a task");
        assert_eq!(task.status, "skipped");
        assert_eq!(event_count(&seeded.run_id), 1, "one append-only fact");
    }

    #[test]
    fn unknown_step_is_rejected_and_writes_nothing() {
        let seeded = seed_frozen_step_run("pv-1", "assay-1", "01-incubate");
        let svc = service();
        let error = svc
            .create_execution_event_impl(ExecutionEventInput {
                run_id: seeded.run_id.clone(),
                event_type: "skipped_by_rule".into(),
                content: "Skip the wash".into(),
                step_path: "02-wash".into(),
                protocol_version_id: "pv-1".into(),
                assay_id: "assay-1".into(),
                ..ExecutionEventInput::default()
            })
            .expect_err("a step outside the frozen list must be rejected");
        let message = format!("{error:?}");
        assert!(message.contains("INVALID"), "{message}");
        assert!(message.contains("Protocol 步骤引用"), "{message}");
        assert_eq!(event_count(&seeded.run_id), 0, "nothing is appended");
    }

    #[test]
    fn expired_pantry_item_only_adds_an_advisory_notice() {
        let seeded = seed_frozen_step_run("pv-1", "assay-1", "01-incubate");
        let svc = service();
        let item = svc
            .upsert_pantry_impl(PantryUpsert {
                name: "Anti-CD3 antibody".into(),
                location: "fridge-1".into(),
                purchased_on: "2020-01-01".into(),
                shelf_life_days: Some(1),
                ..PantryUpsert::default()
            })
            .expect("upsert pantry");

        let result = svc
            .create_execution_event_impl(ExecutionEventInput {
                run_id: seeded.run_id.clone(),
                event_type: "deviation".into(),
                content: "Incubated 12 minutes instead of 10".into(),
                deviation: "The plate stayed in the incubator two minutes longer".into(),
                pantry_item_id: item.id.clone(),
                step_path: "01-incubate".into(),
                protocol_version_id: "pv-1".into(),
                assay_id: "assay-1".into(),
                ..ExecutionEventInput::default()
            })
            .expect("an expired pantry item must not block the event");

        assert_eq!(result.pantry_notices.len(), 1);
        let notice = &result.pantry_notices[0];
        assert_eq!(notice.code, "pantry_item_expiry_notice");
        assert_eq!(notice.severity, "advisory");
        assert_eq!(notice.pantry_item_id, item.id);
        assert_eq!(notice.label_code, item.label_code);
        assert_eq!(notice.status, "expired");
        assert!(notice.days_remaining.is_some_and(|days| days < 0));
        let event = result
            .snapshot
            .events
            .iter()
            .find(|event| event.event_type == "deviation")
            .expect("the deviation is in the read model");
        assert_eq!(event.protocol_step_path, "01-incubate");
        let data: serde_json::Value = serde_json::from_str(&event.data_json).expect("event JSON");
        assert_eq!(data["deviation"], "The plate stayed in the incubator two minutes longer");
        // The pantry reference is stored on the append-only record; the Run read
        // model reports it as the advisory above.
        let stored: String = cli_impl::open_db(&data_dir())
            .expect("open db")
            .query_row(
                "SELECT data_json FROM records WHERE kind='execution_event' AND parent_id=?1",
                [&seeded.run_id],
                |row| row.get(0),
            )
            .expect("stored event");
        let stored: serde_json::Value = serde_json::from_str(&stored).expect("stored JSON");
        assert_eq!(stored["pantry_item_id"], item.id);

        let error = svc
            .create_execution_event_impl(ExecutionEventInput {
                run_id: seeded.run_id.clone(),
                event_type: "observation".into(),
                content: "Looks confluent".into(),
                pantry_item_id: "missing-item".into(),
                ..ExecutionEventInput::default()
            })
            .expect_err("an unknown pantry item is NOT_FOUND");
        assert!(format!("{error:?}").contains("NOT_FOUND"), "{error:?}");
        assert_eq!(event_count(&seeded.run_id), 1, "the rejected reference wrote nothing");
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
                phases: vec![],
            })
            .expect("start timer");
        assert_eq!(started.timer.status, "running");
        assert_eq!(started.timer.started_at_utc.as_deref(), Some("2026-11-15T09:00:00Z"));
        assert!(!started.completed);

        let paused = svc
            .update_timer_impl(TimerActionInput {
                timer: started.timer,
                action: "pause".into(),
                now_utc: "2026-11-15T09:10:00Z".into(),
                phases: vec![],
            })
            .expect("pause timer");
        assert_eq!(paused.timer.status, "paused");
        assert_eq!(paused.timer.accumulated_s, 600);

        let listed = svc.list_timers_impl(&seeded.run_id).expect("list timers");
        assert_eq!(listed.len(), 1);
        assert_eq!(listed[0].accumulated_s, 600);

        assert!(svc
            .update_timer_impl(TimerActionInput {
                timer: paused.timer,
                action: "warp".into(),
                now_utc: "2026-11-15T09:10:00Z".into(),
                phases: vec![],
            })
            .is_err());
    }

    #[test]
    fn advance_walks_the_supplied_phases_and_reports_each_completed_round() {
        let seeded = seed_run(false, false);
        let svc = service();
        let timer = svc
            .save_timer_impl(TimerRecord {
                timer_id: "timer-seq".into(),
                run_id: seeded.run_id.clone(),
                task_key: "stain".into(),
                mode: "countdown".into(),
                status: "running".into(),
                planned_s: Some(5),
                started_at_utc: Some("2026-11-15T09:00:00Z".into()),
                ..TimerRecord::default()
            })
            .expect("save timer");
        let phases = vec![
            TimerPhase { name: "stain".into(), planned_s: Some(5), mode: "countdown".into(), rounds: Some(1) },
            TimerPhase { name: "wash".into(), planned_s: Some(20), mode: "countdown".into(), rounds: Some(1) },
        ];
        let first = svc
            .update_timer_impl(TimerActionInput {
                timer: timer.clone(),
                action: "advance".into(),
                now_utc: "2026-11-15T09:00:05Z".into(),
                phases: phases.clone(),
            })
            .expect("advance the stain phase");
        assert!(first.completed);
        assert_eq!(first.round.phase_name, "stain");
        assert_eq!((first.round.planned_s, first.round.actual_s), (Some(5), 5));
        assert!(first.round.phase_completed && !first.round.timer_completed);
        assert_eq!((first.timer.phase_index, first.timer.round_index), (1, 0));
        assert_eq!(first.timer.planned_s, Some(20), "the wash phase becomes the current plan");
        assert_eq!(first.timer.started_at_utc.as_deref(), Some("2026-11-15T09:00:05Z"));

        let second = svc
            .update_timer_impl(TimerActionInput {
                timer: first.timer,
                action: "advance".into(),
                now_utc: "2026-11-15T09:00:25Z".into(),
                phases,
            })
            .expect("advance the wash phase");
        assert_eq!(second.round.phase_name, "wash");
        assert_eq!(second.round.actual_s, 20);
        assert!(second.round.timer_completed);
        assert_eq!(second.timer.status, "completed");

        let early = svc
            .update_timer_impl(TimerActionInput {
                timer: second.timer.clone(),
                action: "advance".into(),
                now_utc: "2026-11-15T09:00:26Z".into(),
                phases: vec![],
            })
            .expect("a completed timer is not an error");
        assert!(!early.completed, "a completed timer never advances");

        let invalid = svc.update_timer_impl(TimerActionInput {
            timer,
            action: "advance".into(),
            now_utc: "2026-11-15T09:00:05Z".into(),
            phases: vec![TimerPhase {
                name: String::new(),
                planned_s: Some(5),
                mode: "countdown".into(),
                rounds: Some(1),
            }],
        });
        assert!(
            format!("{:?}", invalid.expect_err("an unnamed phase is rejected")).contains("INVALID"),
            "a malformed plan is INVALID, not a silent single-phase fallback"
        );
    }

    /// A service over its own data directory, so a query across every Run sees
    /// only the rows the test writes.
    fn isolated_service(label: &str) -> (CoreRunService, PathBuf) {
        let dir = std::env::temp_dir().join(format!(
            "asterlab-runs-active-{label}-{}-{}",
            std::process::id(),
            uuid::Uuid::new_v4()
        ));
        std::fs::create_dir_all(&dir).expect("create isolated data dir");
        let core = AppCore::open(&dir).expect("open isolated core");
        ensure_module_tables(core.raw_db()).expect("module tables");
        let service = CoreRunService { core: Mutex::new(Some(core)) };
        (service, dir)
    }

    fn insert_run(db: &rusqlite::Connection, run_id: &str, title: &str) {
        db.execute(
            "INSERT INTO records(id,kind,project_id,parent_id,revision,data_json,created_at,updated_at) \
             VALUES(?1,'run',NULL,NULL,1,?2,'2026-11-15T08:00:00Z','2026-11-15T08:00:00Z')",
            rusqlite::params![run_id, serde_json::json!({"title": title, "status": "in_progress"}).to_string()],
        )
        .expect("insert run");
    }

    #[test]
    fn list_active_timers_returns_only_running_timers_across_runs_in_remaining_order() {
        let (svc, dir) = isolated_service("cross-run");
        let db = cli_impl::open_db(&dir).expect("open isolated db");
        insert_run(&db, "run-a", "Alpha");
        insert_run(&db, "run-b", "Beta");

        let save = |timer_id: &str, run_id: &str, task_key: &str, mode: &str, status: &str,
                    started: Option<&str>, accumulated_s: u64, anchor_event: Option<&str>,
                    anchor_task: Option<&str>| {
            svc.save_timer_impl(TimerRecord {
                timer_id: timer_id.into(),
                run_id: run_id.into(),
                task_key: task_key.into(),
                mode: mode.into(),
                status: status.into(),
                planned_s: if mode == "stopwatch" { None } else { Some(600) },
                started_at_utc: started.map(str::to_owned),
                accumulated_s,
                anchor_event_id: anchor_event.map(str::to_owned),
                anchor_task_key: anchor_task.map(str::to_owned),
                ..TimerRecord::default()
            })
            .expect("save timer");
        };
        // Both Run have a running countdown; run-a has two with the same
        // remaining time so the run id and timer id decide.
        save("timer-b", "run-a", "wash", "countdown", "running", Some("2026-11-15T09:08:00Z"), 0, None, None);
        save("timer-a", "run-a", "spin", "countdown", "running", Some("2026-11-15T09:08:00Z"), 0, None, Some("task-anchor"));
        save("timer-c", "run-b", "incubate", "countdown", "running", Some("2026-11-15T09:05:00Z"), 0, Some("event-t0"), None);
        // Neither paused, completed nor a stopwatch may enter the global bar;
        // the stopwatch has no remaining time, so it sorts after the countdowns.
        save("timer-d", "run-a", "done", "countdown", "completed", None, 600, None, None);
        save("timer-e", "run-b", "held", "countdown", "paused", None, 120, None, None);
        save("timer-f", "run-b", "observe", "stopwatch", "running", Some("2026-11-15T09:00:00Z"), 0, None, None);

        let active = svc.list_active_timers_impl("2026-11-15T09:10:00Z").expect("list active timers");
        let order: Vec<(&str, &str)> = active.iter().map(|timer| (timer.run_id.as_str(), timer.timer_id.as_str())).collect();
        assert_eq!(
            order,
            vec![("run-b", "timer-c"), ("run-a", "timer-a"), ("run-a", "timer-b"), ("run-b", "timer-f")],
            "countdowns by remaining time, ties by run id then timer id, stopwatch last"
        );

        let first = &active[0];
        assert_eq!(first.run_title, "Beta");
        assert_eq!(first.task_key, "incubate");
        assert_eq!(first.mode, "countdown");
        assert_eq!(first.status, "running");
        assert_eq!(first.elapsed_s, 300);
        assert_eq!(first.remaining_s, Some(300));
        assert_eq!(first.time_anchor_id, "event-t0");
        assert_eq!(active[1].time_anchor_id, "task-anchor");
        assert_eq!(active[2].time_anchor_id, "");

        let stopwatch = &active[3];
        assert_eq!(stopwatch.elapsed_s, 600);
        assert_eq!(stopwatch.remaining_s, None);

        assert!(
            svc.list_active_timers_impl("not-a-timestamp").is_err(),
            "a running timer needs the caller's clock, never an invented one"
        );
    }

    #[test]
    fn list_active_timers_is_an_empty_list_without_timers() {
        let (svc, _dir) = isolated_service("empty");
        let active = svc.list_active_timers_impl("2026-11-15T09:10:00Z").expect("list active timers");
        assert!(active.is_empty(), "an empty database is not an error");
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

    /// Inserts one asset_link row for a seeded Run. The record id keeps Core's
    /// creation-handle shape but is otherwise arbitrary: the bridge resolves it
    /// from the asset id, so the test only has to make the row addressable.
    fn seed_asset_link(
        run_id: &str,
        asset_id: &str,
        well_id: Option<&str>,
        timepoint: Option<&str>,
        fov_index: Option<u32>,
    ) -> String {
        let db = cli_impl::open_db(&data_dir()).expect("open db");
        let project: String = db
            .query_row("SELECT project_id FROM records WHERE id=?1", [run_id], |row| row.get(0))
            .expect("run project");
        let coordinate_id = well_id.map(|well| {
            let mut id = well.to_ascii_uppercase();
            if let Some(timepoint) = timepoint {
                id.push('_');
                id.push_str(timepoint);
            }
            if let Some(fov) = fov_index {
                id.push_str(&format!("_fov{fov:02}"));
            }
            id
        });
        let record_id = format!("asset-link-{}", uuid::Uuid::new_v4());
        let data = serde_json::json!({
            "id": asset_id,
            "rootId": "project-data",
            "relativePath": format!("huvec/d1/{asset_id}.tif"),
            "mediaType": "image",
            "size": 12,
            "modTime": "2026-01-01T00:00:00Z",
            "sha256": "b".repeat(64),
            "availability": "available",
            "link": {
                "runId": run_id,
                "taskKey": "task-1",
                "sampleId": null,
                "wellId": well_id,
                "timepoint": timepoint,
                "fovIndex": fov_index,
                "coordinateId": coordinate_id
            },
            "linkedAt": "2026-01-01T00:00:00Z",
            "actorId": "tester"
        });
        db.execute(
            "INSERT INTO records(id,kind,project_id,parent_id,revision,data_json,created_at,updated_at) \
             VALUES(?1,'asset_link',?2,?3,1,?4,'2026-02-02T00:00:00Z','2026-02-02T00:00:00Z')",
            rusqlite::params![record_id, project, run_id, data.to_string()],
        )
        .expect("insert asset link");
        record_id
    }

    /// Stored asset_link rows of a Run, newest last: (record id, data).
    fn asset_link_rows(run_id: &str) -> Vec<(String, serde_json::Value)> {
        let db = cli_impl::open_db(&data_dir()).expect("open db");
        let mut statement = db
            .prepare(
                "SELECT id, data_json FROM records WHERE kind='asset_link' AND parent_id=?1 \
                 ORDER BY created_at ASC, rowid ASC",
            )
            .expect("prepare");
        let rows = statement
            .query_map([run_id], |row| {
                Ok((row.get::<_, String>(0)?, row.get::<_, String>(1)?))
            })
            .expect("query")
            .flatten()
            .map(|(id, raw)| (id, serde_json::from_str(&raw).expect("link JSON")))
            .collect();
        rows
    }

    /// Asset link events of a Run with one action, newest last.
    fn asset_events(run_id: &str, action: &str) -> Vec<serde_json::Value> {
        let db = cli_impl::open_db(&data_dir()).expect("open db");
        let mut statement = db
            .prepare(
                "SELECT data_json FROM records WHERE kind='asset_link_event' AND parent_id=?1 \
                 ORDER BY created_at ASC, rowid ASC",
            )
            .expect("prepare");
        statement
            .query_map([run_id], |row| row.get::<_, String>(0))
            .expect("query")
            .flatten()
            .map(|raw| serde_json::from_str::<serde_json::Value>(&raw).expect("event JSON"))
            .filter(|event| event["action"].as_str() == Some(action))
            .collect()
    }

    #[test]
    fn update_asset_link_coordinates_keeps_file_identity() {
        let seeded = seed_run(false, false);
        let record_id = seed_asset_link(&seeded.run_id, "asset-scan-1", None, None, None);
        let svc = service();

        let updated = svc
            .update_asset_link_coordinates_impl(&seeded.run_id, "asset-scan-1", "B2", "t12h", 2)
            .expect("update coordinates");
        assert_eq!(updated.id, "asset-scan-1");
        assert_eq!(updated.link.well_id, "B2");
        assert_eq!(updated.link.timepoint, "t12h");
        assert_eq!(updated.link.fov_index, 2);
        // File identity and the association are copied verbatim.
        assert_eq!(updated.sha256, "b".repeat(64));
        assert_eq!(updated.relative_path, "huvec/d1/asset-scan-1.tif");
        assert_eq!(updated.link.task_key, "task-1");

        let rows = asset_link_rows(&seeded.run_id);
        assert_eq!(rows.len(), 1, "a coordinate update re-links nothing");
        assert_eq!(rows[0].0, record_id, "the record id is the creation handle");
        assert_eq!(rows[0].1["link"]["coordinateId"], "B2_t12h_fov02");

        let events = asset_events(&seeded.run_id, "updated");
        assert_eq!(events.len(), 1, "one updated event");
        assert_eq!(events[0]["previous"]["wellId"], serde_json::Value::Null);
        assert_eq!(events[0]["previous"]["coordinateId"], serde_json::Value::Null);
        assert_eq!(events[0]["link"]["wellId"], "B2");
    }

    #[test]
    fn update_asset_link_coordinates_clears_zero_values() {
        let seeded = seed_run(false, false);
        seed_asset_link(&seeded.run_id, "asset-scan-2", Some("A1"), Some("t6h"), Some(1));
        let svc = service();

        // Go's zero values are Core's "not recorded", not an illegal FOV 0.
        let updated = svc
            .update_asset_link_coordinates_impl(&seeded.run_id, "asset-scan-2", "C3", "", 0)
            .expect("clear coordinates");
        assert_eq!(updated.link.well_id, "C3");
        assert_eq!(updated.link.timepoint, "");
        assert_eq!(updated.link.fov_index, 0);
        assert_eq!(
            asset_link_rows(&seeded.run_id)[0].1["link"]["coordinateId"],
            "C3"
        );
    }

    #[test]
    fn update_asset_link_coordinates_rejects_bad_input_without_writing() {
        let seeded = seed_run(false, false);
        seed_asset_link(&seeded.run_id, "asset-scan-3", Some("A1"), None, Some(1));
        let svc = service();

        let malformed = svc
            .update_asset_link_coordinates_impl(&seeded.run_id, "asset-scan-3", "A", "t6h", 1)
            .expect_err("a well without a column is invalid");
        assert!(
            matches!(&malformed, Error::Go(message) if message.starts_with("INVALID: ")),
            "{malformed:?}"
        );
        let unknown = svc
            .update_asset_link_coordinates_impl(&seeded.run_id, "asset-missing", "A1", "t6h", 1)
            .expect_err("an unlinked asset cannot be updated");
        assert!(
            matches!(&unknown, Error::Go(message) if message.starts_with("NOT_FOUND: ")),
            "{unknown:?}"
        );

        // Rejected calls leave the stored coordinates and the event stream alone.
        let rows = asset_link_rows(&seeded.run_id);
        assert_eq!(rows.len(), 1);
        assert_eq!(rows[0].1["link"]["wellId"], "A1");
        assert!(asset_events(&seeded.run_id, "updated").is_empty());
    }

    #[test]
    fn unlink_asset_link_removes_the_row_and_appends_the_event() {
        let seeded = seed_run(false, false);
        let record_id = seed_asset_link(&seeded.run_id, "asset-scan-4", Some("A1"), Some("t6h"), Some(1));
        let svc = service();

        let removed = svc
            .unlink_asset_link_impl(&seeded.run_id, "asset-scan-4")
            .expect("unlink asset");
        assert_eq!(removed.id, "asset-scan-4");
        assert_eq!(removed.link.well_id, "A1", "the removed link is returned as stored");
        assert!(asset_link_rows(&seeded.run_id).is_empty());

        let events = asset_events(&seeded.run_id, "unlinked");
        assert_eq!(events.len(), 1, "one unlinked event");
        assert_eq!(events[0]["assetId"], "asset-scan-4");
        assert_eq!(events[0]["link"]["coordinateId"], "A1_t6h_fov01");

        // A second unlink finds nothing: the row is gone, and a link of another
        // Run is out of reach too.
        let again = svc
            .unlink_asset_link_impl(&seeded.run_id, "asset-scan-4")
            .expect_err("already unlinked");
        assert!(
            matches!(&again, Error::Go(message) if message.starts_with("NOT_FOUND: ")),
            "{again:?}"
        );
        let other_run = seed_run(false, false);
        let refused = svc
            .unlink_asset_link_impl(&other_run.run_id, "asset-scan-4")
            .expect_err("another Run cannot see this link");
        assert!(
            matches!(&refused, Error::Go(message) if message.starts_with("NOT_FOUND: ")),
            "{refused:?}"
        );
        assert_eq!(asset_link_rows(&seeded.run_id).len(), 0);
        assert_eq!(asset_events(&seeded.run_id, "unlinked").len(), 1);
        let _ = record_id;
    }
}
