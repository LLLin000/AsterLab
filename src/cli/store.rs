use super::*;

pub fn data_dir() -> Result<PathBuf, CliError> {
    let path = if let Ok(v) = std::env::var("LABWORKBENCH_DATA_DIR") {
        if v.trim().is_empty() {
            return Err(CliError::internal("LABWORKBENCH_DATA_DIR is empty"));
        }
        PathBuf::from(v.trim())
    } else if let Ok(base) = std::env::var("LOCALAPPDATA") {
        PathBuf::from(base).join("AsterLab").join("data")
    } else if cfg!(not(windows)) {
        PathBuf::from(std::env::var("HOME").map_err(|e| CliError::internal(e.to_string()))?)
            .join(".local/share/AsterLab/data")
    } else {
        return Err(CliError::internal("LOCALAPPDATA is unavailable"));
    };
    if !path.is_absolute() {
        return Err(CliError::internal("data directory must be an absolute path"));
    }
    ensure_data_dirs(&path).map_err(|e| CliError::internal(e.to_string()))?;
    log_data_dir_once(&path);
    Ok(path)
}

pub(crate) fn ensure_data_dirs(data_dir: &Path) -> std::io::Result<()> {
    fs::create_dir_all(data_dir)?;
    if let Some(parent) = data_dir.parent() {
        fs::create_dir_all(parent.join("logs"))?;
        fs::create_dir_all(parent.join("cache"))?;
        fs::create_dir_all(parent.join("backups"))?;
    }
    Ok(())
}

pub(crate) fn log_data_dir_once(data_dir: &Path) {
    use std::sync::atomic::{AtomicBool, Ordering};
    static LOGGED: AtomicBool = AtomicBool::new(false);
    if !LOGGED.swap(true, Ordering::Relaxed) {
        eprintln!("asterlab data dir: {}", data_dir.display());
    }
}

pub fn db_path(data_dir: &Path) -> PathBuf {
    data_dir.join("labworkbench.sqlite")
}


/// The pantry ledger is an auxiliary table, not a versioned `records` kind: it
/// never participates in Design/Protocol/Run freezing, and existing databases
/// gain it without a `records` schema migration or a `user_version` bump.
pub(crate) const PANTRY_TABLE: &str = "\
CREATE TABLE IF NOT EXISTS pantry_items(\
    id TEXT PRIMARY KEY,\
    label_code TEXT NOT NULL UNIQUE,\
    project_id TEXT,\
    name TEXT NOT NULL,\
    location TEXT NOT NULL,\
    purchased_on TEXT,\
    opened_on TEXT,\
    shelf_life_days INTEGER,\
    opened_shelf_life_days INTEGER,\
    exhausted INTEGER NOT NULL DEFAULT 0,\
    revision INTEGER NOT NULL DEFAULT 1,\
    deleted INTEGER NOT NULL DEFAULT 0,\
    created_at TEXT NOT NULL,\
    updated_at TEXT NOT NULL\
);\
CREATE INDEX IF NOT EXISTS pantry_items_project ON pantry_items(project_id);";

pub(crate) fn migrate_records_schema(db: &mut Connection) -> Result<(), CliError> {
    let tx = db
        .transaction_with_behavior(TransactionBehavior::Immediate)
        .map_err(|error| CliError::internal(error.to_string()))?;
    let user_version: i64 = tx
        .query_row("PRAGMA user_version", [], |row| row.get(0))
        .map_err(|error| CliError::internal(error.to_string()))?;
    if user_version > RECORDS_SCHEMA_VERSION {
        return Err(CliError::internal(format!(
            "database schema version {user_version} is newer than supported version {RECORDS_SCHEMA_VERSION}"
        )));
    }
    if user_version < 0 {
        return Err(CliError::internal(format!(
            "unsupported database schema version {user_version}"
        )));
    }
    if user_version == RECORDS_SCHEMA_VERSION {
        tx.commit()
            .map_err(|error| CliError::internal(error.to_string()))?;
        return Ok(());
    }

    let records_exists: bool = tx
        .query_row(
            "SELECT EXISTS(SELECT 1 FROM sqlite_master WHERE type='table' AND name='records')",
            [],
            |row| row.get(0),
        )
        .map_err(|error| CliError::internal(error.to_string()))?;
    if !records_exists {
        tx.execute_batch(
            "CREATE TABLE records(\
                id TEXT PRIMARY KEY,\
                kind TEXT NOT NULL,\
                project_id TEXT REFERENCES records(id),\
                parent_id TEXT REFERENCES records(id) ON DELETE RESTRICT,\
                revision INTEGER NOT NULL DEFAULT 1,\
                data_json TEXT NOT NULL,\
                created_at TEXT NOT NULL,\
                updated_at TEXT NOT NULL\
            );",
        )
        .map_err(|error| CliError::internal(error.to_string()))?;
    } else {
        let columns = {
            let mut statement = tx
                .prepare("PRAGMA table_info(records)")
                .map_err(|error| CliError::internal(error.to_string()))?;
            let columns = statement
                .query_map([], |row| row.get::<_, String>(1))
                .map_err(|error| CliError::internal(error.to_string()))?
                .collect::<rusqlite::Result<Vec<_>>>()
                .map_err(|error| CliError::internal(error.to_string()))?;
            columns
        };
        for required in [
            "id",
            "kind",
            "parent_id",
            "revision",
            "data_json",
            "created_at",
            "updated_at",
        ] {
            if !columns.iter().any(|column| column == required) {
                return Err(CliError::internal(format!(
                    "unsupported legacy records schema: missing {required} column"
                )));
            }
        }
        if !columns.iter().any(|column| column == "project_id") {
            tx.execute_batch(
                r#"ALTER TABLE records ADD COLUMN project_id TEXT REFERENCES records(id);
                 WITH RECURSIVE project_ancestors(record_id, ancestor_id, kind, parent_id) AS (
                     SELECT id, id, kind, parent_id FROM records
                     UNION
                     SELECT ancestors.record_id, parent.id, parent.kind, parent.parent_id
                     FROM project_ancestors AS ancestors
                     JOIN records AS parent ON parent.id=ancestors.parent_id
                     WHERE ancestors.kind <> 'project'
                 )
                 UPDATE records
                 SET project_id=(
                     SELECT ancestor_id FROM project_ancestors
                     WHERE record_id=records.id AND kind='project' LIMIT 1
                 )
                 WHERE kind <> 'project';"#,
            )
            .map_err(|error| CliError::internal(error.to_string()))?;
        }
    }

    tx.execute_batch(
        "CREATE INDEX IF NOT EXISTS records_kind_parent ON records(kind,parent_id);\
         CREATE INDEX IF NOT EXISTS records_project ON records(project_id);",
    )
    .map_err(|error| CliError::internal(error.to_string()))?;
    tx.pragma_update(None, "user_version", RECORDS_SCHEMA_VERSION)
        .map_err(|error| CliError::internal(error.to_string()))?;
    tx.commit()
        .map_err(|error| CliError::internal(error.to_string()))?;
    Ok(())
}

pub(crate) fn open_db(path: &Path) -> Result<Connection, CliError> {
    let data_dir = if path.extension().map(|e| e == "sqlite").unwrap_or(false) {
        path.parent().unwrap_or(path).to_path_buf()
    } else {
        path.to_path_buf()
    };
    ensure_data_dirs(&data_dir).map_err(|e| CliError::internal(e.to_string()))?;
    let db_file = data_dir.join("labworkbench.sqlite");
    let mut db = Connection::open(&db_file).map_err(|e| CliError::internal(e.to_string()))?;
    db.busy_timeout(std::time::Duration::from_secs(10))
        .map_err(|e| CliError::internal(e.to_string()))?;
    db.pragma_update(None, "foreign_keys", "ON")
        .map_err(|e| CliError::internal(e.to_string()))?;
    migrate_records_schema(&mut db)?;
    db.execute_batch(PANTRY_TABLE)
        .map_err(|e| CliError::internal(e.to_string()))?;
    // Databases created before the tombstone column existed cannot be fixed by
    // `CREATE TABLE IF NOT EXISTS`; adding the column is the only migration.
    if !pantry_has_deleted_column(&db)? {
        if let Err(error) =
            db.execute_batch("ALTER TABLE pantry_items ADD COLUMN deleted INTEGER NOT NULL DEFAULT 0")
        {
            // A concurrent opener may have added the column between the check
            // and the ALTER; that outcome is the one we wanted anyway.
            if !error.to_string().contains("duplicate column name") {
                return Err(CliError::internal(error.to_string()));
            }
        }
    }

    // WAL is a persistent database property. A concurrent opener may hold the
    // exclusive lock needed to change it; keep the existing journal mode rather
    // than failing the whole connection. Writes remain transactional.
    if let Err(error) = db.pragma_update(None, "journal_mode", "WAL") {
        eprintln!(
            "asterlab: WAL not enabled for {} ({error}); continuing with existing journal mode",
            db_file.display()
        );
    }
    log_data_dir_once(&data_dir);
    Ok(db)
}
pub(crate) fn kind_id(
    db: &Connection,
    id: &str,
) -> Result<Option<(String, Option<String>, Option<String>, i64, String)>, CliError> {
    db.query_row(
        "SELECT kind,project_id,parent_id,revision,data_json FROM records WHERE id=?1",
        [id],
        |r| Ok((r.get(0)?, r.get(1)?, r.get(2)?, r.get(3)?, r.get(4)?)),
    )
    .optional()
    .map_err(|e| CliError::internal(e.to_string()))
}
pub(crate) fn has_kind(db: &Connection, id: &str, kind: &str) -> Result<bool, CliError> {
    Ok(kind_id(db, id)?.is_some_and(|r| r.0 == kind))
}
pub(crate) fn insert(
    tx: &Transaction<'_>,
    kind: &str,
    project: Option<&str>,
    parent: Option<&str>,
    revision: i64,
    data: &Value,
) -> Result<Value, CliError> {
    let id = make_id();
    let at = stamp();
    let raw = serde_json::to_string(data).map_err(|e| CliError::internal(e.to_string()))?;
    tx.execute("INSERT INTO records(id,kind,project_id,parent_id,revision,data_json,created_at,updated_at) VALUES(?1,?2,?3,?4,?5,?6,?7,?7)",params![id,kind,project,parent,revision,raw,at]).map_err(|e|CliError::internal(e.to_string()))?;
    Ok(
        json!({"id":id,"kind":kind,"project_id":project,"parent_id":parent,"revision":revision,"data":data,"created_at":at,"updated_at":at}),
    )
}
pub(crate) fn get(db: &Connection, id: &str) -> Result<Value, CliError> {
    let (kind, project, parent, revision, raw) =
        kind_id(db, id)?.ok_or_else(|| CliError::missing(format!("记录不存在：{id}")))?;
    let data: Value = serde_json::from_str(&raw).map_err(|e| CliError::internal(e.to_string()))?;
    let (created, updated): (String, String) = db
        .query_row(
            "SELECT created_at,updated_at FROM records WHERE id=?1",
            [id],
            |r| Ok((r.get(0)?, r.get(1)?)),
        )
        .map_err(|e| CliError::internal(e.to_string()))?;
    Ok(
        json!({"id":id,"kind":kind,"project_id":project,"parent_id":parent,"revision":revision,"data":data,"created_at":created,"updated_at":updated}),
    )
}
pub(crate) fn list(
    db: &Connection,
    kind: Option<&str>,
    project: Option<&str>,
) -> Result<Vec<Value>, CliError> {
    let mut stmt=db.prepare("SELECT id,kind,project_id,parent_id,revision,data_json,created_at,updated_at FROM records ORDER BY created_at DESC,rowid DESC").map_err(|e|CliError::internal(e.to_string()))?;
    let mapped=stmt.query_map([],|r|{let raw:String=r.get(5)?;let data:Value=serde_json::from_str(&raw).unwrap_or(Value::Null);Ok(json!({"id":r.get::<_,String>(0)?,"kind":r.get::<_,String>(1)?,"project_id":r.get::<_,Option<String>>(2)?,"parent_id":r.get::<_,Option<String>>(3)?,"revision":r.get::<_,i64>(4)?,"data":data,"created_at":r.get::<_,String>(6)?,"updated_at":r.get::<_,String>(7)?}))}).map_err(|e|CliError::internal(e.to_string()))?;
    let mut out = Vec::new();
    for row in mapped {
        let r = row.map_err(|e| CliError::internal(e.to_string()))?;
        if r["kind"] == "agent_draft" {
            continue;
        }
        if kind.is_some_and(|k| r["kind"] != k) {
            continue;
        }
        if project.is_some_and(|p| r["project_id"] != p) {
            continue;
        }
        out.push(r)
    }
    Ok(out)
}
