use super::*;

/// Column order read by [`pantry_read`]; keep in sync with its row mapper.
pub(crate) const PANTRY_COLUMNS: &str = "id,label_code,project_id,name,location,purchased_on,opened_on,\
shelf_life_days,opened_shelf_life_days,exhausted,revision,created_at,updated_at";

/// `pantry.upsert` is a whole-object write: omitted optional fields are
/// cleared, so a stale reader can never merge half of two edits.
#[derive(Debug, Deserialize)]
#[serde(deny_unknown_fields)]
pub(crate) struct PantryUpsertInput {
    #[serde(default)]
    pub(crate) id: Option<String>,
    #[serde(default)]
    pub(crate) project_id: Option<String>,
    pub(crate) name: String,
    #[serde(default)]
    pub(crate) location: String,
    #[serde(default)]
    pub(crate) purchased_on: Option<String>,
    #[serde(default)]
    pub(crate) opened_on: Option<String>,
    #[serde(default)]
    pub(crate) shelf_life_days: Option<u32>,
    #[serde(default)]
    pub(crate) opened_shelf_life_days: Option<u32>,
    #[serde(default)]
    pub(crate) exhausted: bool,
    #[serde(default)]
    pub(crate) expected_revision: Option<i64>,
}

/// Shared read input for `pantry.list` and `label.export`. `as_of` exists so a
/// caller (and a test) can pin the date the status is computed against.
#[derive(Debug, Default, Deserialize)]
#[serde(deny_unknown_fields)]
pub(crate) struct PantryQueryInput {
    #[serde(default)]
    pub(crate) project_id: Option<String>,
    #[serde(default)]
    pub(crate) include_exhausted: Option<bool>,
    #[serde(default)]
    pub(crate) as_of: Option<String>,
    #[serde(default)]
    pub(crate) near_expiry_days: Option<u32>,
}

/// `pantry.delete` retires a row instead of erasing it: its `label_code` may
/// already be printed on a package, so it must never be handed to another item.
#[derive(Debug, Deserialize)]
#[serde(deny_unknown_fields)]
pub(crate) struct PantryDeleteInput {
    pub(crate) id: String,
    #[serde(default)]
    pub(crate) expected_revision: Option<i64>,
}

pub(crate) fn pantry_has_deleted_column(db: &Connection) -> Result<bool, CliError> {
    let mut statement = db
        .prepare("PRAGMA table_info(pantry_items)")
        .map_err(|e| CliError::internal(e.to_string()))?;
    let columns = statement
        .query_map([], |row| row.get::<_, String>(1))
        .map_err(|e| CliError::internal(e.to_string()))?
        .collect::<rusqlite::Result<Vec<_>>>()
        .map_err(|e| CliError::internal(e.to_string()))?;
    Ok(columns.iter().any(|column| column == "deleted"))
}

pub(crate) fn pantry_read(
    db: &Connection,
    id: &str,
) -> Result<Option<(PantryItem, String, String)>, CliError> {
    db.query_row(
        &format!("SELECT {PANTRY_COLUMNS} FROM pantry_items WHERE id=?1 AND deleted=0"),
        [id],
        |row| {
            Ok((
                PantryItem {
                    id: row.get(0)?,
                    label_code: row.get(1)?,
                    project_id: row.get(2)?,
                    revision: row.get(10)?,
                    draft: PantryDraft {
                        name: row.get(3)?,
                        location: row.get(4)?,
                        purchased_on: row.get(5)?,
                        opened_on: row.get(6)?,
                        shelf_life_days: row.get::<_, Option<i64>>(7)?.map(|days| days as u32),
                        opened_shelf_life_days: row
                            .get::<_, Option<i64>>(8)?
                            .map(|days| days as u32),
                        exhausted: row.get::<_, i64>(9)? != 0,
                    },
                },
                row.get::<_, String>(11)?,
                row.get::<_, String>(12)?,
            ))
        },
    )
    .optional()
    .map_err(|e| CliError::internal(e.to_string()))
}

/// First free `PN-XXXXXXXX` variant for a new item. The suffix is persisted, so
/// an already printed label stays scannable for the life of the item.
pub(crate) fn pantry_label_code(db: &Connection, id: &str) -> Result<String, CliError> {
    let base = pantry::base_label_code(id);
    for sequence in 1..=1000u32 {
        let candidate = pantry::label_code_with_suffix(&base, sequence);
        let taken: bool = db
            .query_row(
                "SELECT EXISTS(SELECT 1 FROM pantry_items WHERE label_code=?1)",
                [&candidate],
                |row| row.get(0),
            )
            .map_err(|e| CliError::internal(e.to_string()))?;
        if !taken {
            return Ok(candidate);
        }
    }
    Err(CliError::internal("无法为储藏室条目分配唯一短 ID"))
}

pub(crate) fn pantry_as_of(input: &PantryQueryInput) -> Result<CalendarDate, CliError> {
    match input.as_of.as_deref().map(str::trim) {
        Some(raw) => CalendarDate::parse(raw)
            .ok_or_else(|| CliError::input("as_of 必须是 YYYY-MM-DD 的真实日期")),
        None => CalendarDate::parse(&Utc::now().date_naive().to_string())
            .ok_or_else(|| CliError::internal("无法解析当前日期")),
    }
}

pub(crate) fn pantry_near_expiry_days(input: &PantryQueryInput) -> Result<u32, CliError> {
    let days = input
        .near_expiry_days
        .unwrap_or(pantry::NEAR_EXPIRY_DAYS);
    if days > pantry::MAX_SHELF_LIFE_DAYS {
        return Err(CliError::input(format!(
            "near_expiry_days 不能超过 {} 天",
            pantry::MAX_SHELF_LIFE_DAYS
        )));
    }
    Ok(days)
}

/// Flat JSON shape shared by `pantry.list`, `pantry.upsert` and `label.export`.
pub(crate) fn pantry_item_json(
    item: &PantryItem,
    created_at: &str,
    updated_at: &str,
    as_of: CalendarDate,
    near_expiry_days: u32,
) -> Value {
    let evaluation = pantry::evaluate(item, as_of, near_expiry_days);
    json!({
        "id": item.id,
        "label_code": item.label_code,
        "project_id": item.project_id,
        "name": item.draft.name,
        "location": item.draft.location,
        "purchased_on": item.draft.purchased_on,
        "opened_on": item.draft.opened_on,
        "shelf_life_days": item.draft.shelf_life_days,
        "opened_shelf_life_days": item.draft.opened_shelf_life_days,
        "exhausted": item.draft.exhausted,
        "revision": item.revision,
        "status": evaluation.status,
        "expires_on": evaluation.expires_on,
        "days_remaining": evaluation.days_remaining,
        "expiry_basis": evaluation.basis,
        "created_at": created_at,
        "updated_at": updated_at,
    })
}

pub(crate) fn pantry_blanks_as_none(value: Option<&str>) -> Option<String> {
    value
        .map(str::trim)
        .filter(|text| !text.is_empty())
        .map(str::to_owned)
}

pub(crate) fn pantry_upsert(
    db: &mut Connection,
    input: &PantryUpsertInput,
) -> Result<Value, CliError> {
    let draft = PantryDraft {
        name: input.name.trim().to_owned(),
        location: input.location.trim().to_owned(),
        purchased_on: pantry_blanks_as_none(input.purchased_on.as_deref()),
        opened_on: pantry_blanks_as_none(input.opened_on.as_deref()),
        shelf_life_days: input.shelf_life_days,
        opened_shelf_life_days: input.opened_shelf_life_days,
        exhausted: input.exhausted,
    };
    draft.validate().map_err(CliError::input)?;
    let project_id = pantry_blanks_as_none(input.project_id.as_deref());
    if let Some(project_id) = project_id.as_deref() {
        ensure_project(db, project_id)?;
    }
    let tx = db
        .transaction_with_behavior(TransactionBehavior::Immediate)
        .map_err(|e| CliError::internal(e.to_string()))?;
    let now = stamp();
    let existing_id = pantry_blanks_as_none(input.id.as_deref());
    let (item, created_at) = match existing_id {
        Some(id) => {
            let (existing, created_at, _) = pantry_read(&tx, &id)?
                .ok_or_else(|| CliError::missing(format!("储藏室条目不存在：{id}")))?;
            if let Some(expected) = input.expected_revision {
                if existing.revision != expected {
                    return Err(CliError::conflict(format!(
                        "储藏室条目版本已从 v{expected} 更新为 v{}；请先重新读取。",
                        existing.revision
                    )));
                }
            }
            let revision = existing.revision + 1;
            tx.execute(
                "UPDATE pantry_items SET project_id=?2,name=?3,location=?4,purchased_on=?5,\
opened_on=?6,shelf_life_days=?7,opened_shelf_life_days=?8,exhausted=?9,revision=?10,\
updated_at=?11 WHERE id=?1",
                params![
                    id,
                    project_id,
                    draft.name,
                    draft.location,
                    draft.purchased_on,
                    draft.opened_on,
                    draft.shelf_life_days,
                    draft.opened_shelf_life_days,
                    i64::from(draft.exhausted),
                    revision,
                    now,
                ],
            )
            .map_err(|e| CliError::internal(e.to_string()))?;
            (
                PantryItem {
                    id,
                    label_code: existing.label_code,
                    project_id,
                    revision,
                    draft,
                },
                created_at,
            )
        }
        None => {
            let id = make_id();
            let label_code = pantry_label_code(&tx, &id)?;
            tx.execute(
                "INSERT INTO pantry_items(id,label_code,project_id,name,location,purchased_on,\
opened_on,shelf_life_days,opened_shelf_life_days,exhausted,revision,created_at,updated_at)\
 VALUES(?1,?2,?3,?4,?5,?6,?7,?8,?9,?10,1,?11,?11)",
                params![
                    id,
                    label_code,
                    project_id,
                    draft.name,
                    draft.location,
                    draft.purchased_on,
                    draft.opened_on,
                    draft.shelf_life_days,
                    draft.opened_shelf_life_days,
                    i64::from(draft.exhausted),
                    now,
                ],
            )
            .map_err(|e| CliError::internal(e.to_string()))?;
            (
                PantryItem {
                    id,
                    label_code,
                    project_id,
                    revision: 1,
                    draft,
                },
                now.clone(),
            )
        }
    };
    tx.commit().map_err(|e| CliError::internal(e.to_string()))?;
    let as_of = pantry_as_of(&PantryQueryInput::default())?;
    Ok(json!({
        "item": pantry_item_json(&item, &created_at, &now, as_of, pantry::NEAR_EXPIRY_DAYS)
    }))
}

pub(crate) fn pantry_rows(
    db: &Connection,
    input: &PantryQueryInput,
) -> Result<Vec<(PantryItem, String, String)>, CliError> {
    let project_id = pantry_blanks_as_none(input.project_id.as_deref());
    let include_exhausted = input.include_exhausted.unwrap_or(true);
    let mut statement = db
        .prepare(&format!(
            "SELECT {PANTRY_COLUMNS} FROM pantry_items \
WHERE deleted=0 AND (?1 IS NULL OR project_id=?1) AND (?2 OR exhausted=0) ORDER BY name, label_code"
        ))
        .map_err(|e| CliError::internal(e.to_string()))?;
    let rows = statement
        .query_map(params![project_id, include_exhausted], |row| {
            Ok((
                PantryItem {
                    id: row.get(0)?,
                    label_code: row.get(1)?,
                    project_id: row.get(2)?,
                    revision: row.get(10)?,
                    draft: PantryDraft {
                        name: row.get(3)?,
                        location: row.get(4)?,
                        purchased_on: row.get(5)?,
                        opened_on: row.get(6)?,
                        shelf_life_days: row.get::<_, Option<i64>>(7)?.map(|days| days as u32),
                        opened_shelf_life_days: row
                            .get::<_, Option<i64>>(8)?
                            .map(|days| days as u32),
                        exhausted: row.get::<_, i64>(9)? != 0,
                    },
                },
                row.get::<_, String>(11)?,
                row.get::<_, String>(12)?,
            ))
        })
        .map_err(|e| CliError::internal(e.to_string()))?
        .collect::<rusqlite::Result<Vec<_>>>()
        .map_err(|e| CliError::internal(e.to_string()))?;
    Ok(rows)
}

pub(crate) fn pantry_list(db: &Connection, input: &PantryQueryInput) -> Result<Value, CliError> {
    let as_of = pantry_as_of(input)?;
    let near_expiry_days = pantry_near_expiry_days(input)?;
    let items = pantry_rows(db, input)?
        .iter()
        .map(|(item, created_at, updated_at)| {
            pantry_item_json(item, created_at, updated_at, as_of, near_expiry_days)
        })
        .collect::<Vec<_>>();
    Ok(json!({
        "as_of": as_of.to_iso(),
        "near_expiry_days": near_expiry_days,
        "items": items,
    }))
}

pub(crate) fn pantry_export_labels(
    db: &Connection,
    input: &PantryQueryInput,
) -> Result<Value, CliError> {
    let as_of = pantry_as_of(input)?;
    let near_expiry_days = pantry_near_expiry_days(input)?;
    let mut rows = pantry_rows(db, input)?;
    rows.sort_by(|left, right| left.0.label_code.cmp(&right.0.label_code));
    let labels = rows
        .iter()
        .map(|(item, _, _)| {
            let evaluation = pantry::evaluate(item, as_of, near_expiry_days);
            json!({
                "label_code": item.label_code,
                "pantry_item_id": item.id,
                "name": item.draft.name,
                "location": item.draft.location,
                "status": evaluation.status,
                "expires_on": evaluation.expires_on,
                "days_remaining": evaluation.days_remaining,
                "revision": item.revision,
            })
        })
        .collect::<Vec<_>>();
    Ok(json!({
        "schema_version": 1,
        "generated_at": stamp(),
        "as_of": as_of.to_iso(),
        "near_expiry_days": near_expiry_days,
        "labels": labels,
    }))
}

/// Retire one item. The row stays as a tombstone so `label_code` remains
/// reserved; every read path filters it out, and the id cannot be revived.
pub(crate) fn pantry_delete(
    db: &mut Connection,
    input: &PantryDeleteInput,
) -> Result<Value, CliError> {
    let id = input.id.trim().to_owned();
    check_text(&id, "储藏室条目 ID", 120)?;
    let tx = db
        .transaction_with_behavior(TransactionBehavior::Immediate)
        .map_err(|e| CliError::internal(e.to_string()))?;
    let (existing, _, _) = pantry_read(&tx, &id)?
        .ok_or_else(|| CliError::missing(format!("储藏室条目不存在：{id}")))?;
    if let Some(expected) = input.expected_revision {
        if existing.revision != expected {
            return Err(CliError::conflict(format!(
                "储藏室条目版本已从 v{expected} 更新为 v{}；请先重新读取。",
                existing.revision
            )));
        }
    }
    let revision = existing.revision + 1;
    tx.execute(
        "UPDATE pantry_items SET deleted=1,revision=?2,updated_at=?3 WHERE id=?1",
        params![id, revision, stamp()],
    )
    .map_err(|e| CliError::internal(e.to_string()))?;
    tx.commit().map_err(|e| CliError::internal(e.to_string()))?;
    Ok(json!({
        "pantry_item_id": existing.id,
        "label_code": existing.label_code,
        "revision": revision,
        "deleted": true,
    }))
}

/// Advisory attached to an execution event that names a pantry item. Never
/// blocks the write; it only surfaces an item that is near expiry or overdue.
pub(crate) fn pantry_event_notice(db: &Connection, item_id: Option<&str>) -> Result<Option<Value>, CliError> {
    let Some(item_id) = item_id else {
        return Ok(None);
    };
    let Some((item, _, _)) = pantry_read(db, item_id)? else {
        return Ok(None);
    };
    let as_of = pantry_as_of(&PantryQueryInput::default())?;
    let evaluation = pantry::evaluate(&item, as_of, pantry::NEAR_EXPIRY_DAYS);
    if !evaluation.needs_notice() {
        return Ok(None);
    }
    Ok(Some(json!({
        "code": "pantry_item_expiry_notice",
        "severity": "advisory",
        "pantry_item_id": item.id,
        "label_code": item.label_code,
        "name": item.draft.name,
        "status": evaluation.status,
        "expires_on": evaluation.expires_on,
        "days_remaining": evaluation.days_remaining,
        "next_action": "核对或更换该储藏室条目；提示不阻断事件写入",
    })))
}

