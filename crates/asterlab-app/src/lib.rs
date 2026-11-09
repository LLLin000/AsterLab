#![recursion_limit = "256"]

//! Shared AsterLab application core.
//!
//! This crate includes the existing CLI command implementation as a module
//! and exposes the stable operations the desktop UI needs. The CLI binary
//! still uses the same source, so both entry points execute one implementation.

#[path = "../../../src/cli.rs"]
mod cli_impl;

use std::path::Path;
use serde_json::{json, Value};
use rusqlite::Connection;

pub struct AppCore {
    db: Connection,
}

impl AppCore {
    pub fn open(data_dir: &Path) -> Result<AppCore, String> {
        let db = cli_impl::open_db(data_dir).map_err(|e| e.message)?;
        Ok(AppCore { db })
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

    /// Append a measurement event and return the authoritative snapshot.
    /// The event is written in one transaction; on any validation error the
    /// transaction is rolled back and the old snapshot is returned unchanged.
    pub fn save_readout(&mut self, input: Value) -> Result<Value, String> {
        let run_id = input.get("run_id").and_then(Value::as_str).map(str::to_owned)
            .ok_or_else(|| "run_id is required".to_owned())?;
        let tx = self.db.transaction().map_err(|e| e.to_string())?;
        let mut event = input.clone();
        if event.get("event_type").is_none() {
            event["event_type"] = json!("measurement");
        }
        if event.get("content").is_none() {
            event["content"] = json!("结构化记录");
        }
        cli_impl::create_execution_event_tx(&tx, &run_id, &event).map_err(|e| e.message)?;
        tx.commit().map_err(|e| e.to_string())?;
        self.load_run(&run_id)
    }
}
