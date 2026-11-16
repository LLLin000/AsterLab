use super::*;

#[derive(Debug, Deserialize)]
#[serde(deny_unknown_fields)]
pub(crate) struct AssetLinkCreate {
    pub(crate) root_id: String,
    pub(crate) root_dir: String,
    #[serde(default)]
    pub(crate) relative_path: Option<String>,
    #[serde(default)]
    pub(crate) path: Option<String>,
    pub(crate) run_id: String,
    pub(crate) task_key: String,
    #[serde(default)]
    pub(crate) sample_id: Option<String>,
    #[serde(default)]
    pub(crate) well_id: Option<String>,
    #[serde(default)]
    pub(crate) timepoint: Option<String>,
    #[serde(default)]
    pub(crate) fov_index: Option<u32>,
    #[serde(default)]
    pub(crate) actor_id: Option<String>,
}

#[derive(Debug, Deserialize)]
#[serde(deny_unknown_fields)]
pub(crate) struct AssetUnlinkInput {
    pub(crate) id: String,
    #[serde(default)]
    pub(crate) actor_id: Option<String>,
}

/// `asset.link.update` carries the complete coordinate triple. An omitted field
/// would silently clear a recorded coordinate, so `timepoint` / `fov_index`
/// use the double-option idiom: `None` = key absent, `Some(None)` = explicit
/// `null`. Only explicit `null` clears a field.
///
/// The fields are crate-visible because the MyGo native bridge calls
/// `update_asset_link` with the same validation instead of re-deriving Core's
/// coordinate rules; the bridge has no absent/`null` distinction, so it always
/// passes `Some(..)`.
#[derive(Debug, Deserialize)]
#[serde(deny_unknown_fields)]
pub(crate) struct AssetLinkUpdateInput {
    pub(crate) id: String,
    pub(crate) well_id: String,
    #[serde(default, deserialize_with = "deserialize_some")]
    pub(crate) timepoint: Option<Option<String>>,
    #[serde(default, deserialize_with = "deserialize_some")]
    pub(crate) fov_index: Option<Option<u32>>,
    #[serde(default)]
    pub(crate) actor_id: Option<String>,
}

/// Wraps a present JSON value (including `null`) in `Some` so the caller can
/// tell "absent" apart from "explicitly null" with `#[serde(default)]`.
pub(crate) fn deserialize_some<'de, D, T>(deserializer: D) -> Result<Option<Option<T>>, D::Error>
where
    D: serde::Deserializer<'de>,
    T: Deserialize<'de>,
{
    Option::<T>::deserialize(deserializer).map(Some)
}

#[derive(Debug, Deserialize)]
#[serde(deny_unknown_fields)]
pub(crate) struct AssetListInput {
    pub(crate) run_id: String,
}

#[derive(Debug, Deserialize)]
#[serde(deny_unknown_fields)]
pub(crate) struct AssetEventsInput {
    pub(crate) run_id: String,
    #[serde(default)]
    pub(crate) action: Option<String>,
}

pub(crate) fn check_asset_root_id(root_id: &str) -> Result<(), CliError> {
    if root_id.is_empty() {
        return Err(CliError::input("root_id is required"));
    }
    if root_id.len() > 200 || root_id.chars().any(char::is_control) {
        return Err(CliError::input("root_id is invalid"));
    }
    Ok(())
}

pub(crate) fn check_asset_sample_id(sample_id: &str) -> Result<(), CliError> {
    if sample_id.len() > 200 || sample_id.chars().any(char::is_control) {
        return Err(CliError::input("sample_id is invalid"));
    }
    Ok(())
}

/// Validates the Run/task association and returns the Run's project id.
pub(crate) fn asset_link_scope(db: &Connection, run_id: &str, task_key: &str) -> Result<String, CliError> {
    let (kind, project, _, _, _) =
        kind_id(db, run_id)?.ok_or_else(|| CliError::missing("实验运行不存在"))?;
    if kind != "run" {
        return Err(CliError::input("run_id must point to a run"));
    }
    if task_key.is_empty() || task_key.len() > 1000 || task_key.chars().any(char::is_control) {
        return Err(CliError::input("task_key is invalid"));
    }
    let workspace = run_workspace(db, run_id)?;
    let task_exists = workspace["tasks"]
        .as_array()
        .is_some_and(|tasks| tasks.iter().any(|task| task["task_key"].as_str() == Some(task_key)));
    if !task_exists {
        return Err(CliError::input("task_key does not belong to this Run"));
    }
    project.ok_or_else(|| CliError::internal("实验运行缺少所属项目"))
}

pub(crate) fn asset_link_record_id(
    asset: &asset_refs::AssetRef,
    run_id: &str,
    task_key: &str,
    sample_id: &str,
    coordinates: &Value,
) -> String {
    let mut hasher = Sha256::new();
    for part in [
        asset.root_id.as_str(),
        asset.relative_path.as_str(),
        run_id,
        task_key,
        sample_id,
        coordinates["coordinateId"].as_str().unwrap_or(""),
    ] {
        hasher.update(part.as_bytes());
        hasher.update(b"\0");
    }
    let digest = hasher.finalize();
    let mut record_id = String::from("asset-link-");
    for byte in digest.iter().take(16) {
        record_id.push_str(&format!("{byte:02x}"));
    }
    record_id
}

pub(crate) fn validate_asset_coordinates(
    well_id: Option<&str>,
    timepoint: Option<&str>,
    fov_index: Option<u32>,
) -> Result<(), CliError> {
    for (field, value, limit) in [("well_id", well_id, 40), ("timepoint", timepoint, 100)] {
        if value.is_some_and(|v| v.chars().count() > limit || v.chars().any(char::is_control)) {
            return Err(CliError::input(format!("{field} is invalid")));
        }
    }
    if let Some(well) = well_id {
        split_well(well)?;
    }
    if fov_index == Some(0) {
        return Err(CliError::input("fov_index must start at 1"));
    }
    if well_id.is_none() && (timepoint.is_some() || fov_index.is_some()) {
        return Err(CliError::input("timepoint and fov_index require well_id"));
    }
    Ok(())
}

pub(crate) fn asset_coordinate_id(well_id: Option<&str>, timepoint: Option<&str>, fov_index: Option<u32>) -> Option<String> {
    let well = well_id.filter(|value| !value.trim().is_empty())?;
    if fov_index == Some(0) {
        return None;
    }
    let mut id = well.to_ascii_uppercase();
    if let Some(timepoint) = timepoint.filter(|value| !value.is_empty()) {
        id.push('_');
        id.push_str(timepoint);
    }
    if let Some(fov) = fov_index {
        id.push_str(&format!("_fov{fov:02}"));
    }
    Some(id)
}

pub(crate) fn asset_coordinates(well_id: Option<&str>, timepoint: Option<&str>, fov_index: Option<u32>) -> Value {
    json!({"wellId":well_id,"timepoint":timepoint,"fovIndex":fov_index,
        "coordinateId":asset_coordinate_id(well_id,timepoint,fov_index)})
}

pub(crate) fn asset_link_data(
    asset: &asset_refs::AssetRef,
    run_id: &str,
    task_key: &str,
    sample_id: &str,
    linked_at: &str,
    actor_id: &str,
    coordinates: &Value,
) -> Value {
    json!({
        "id": asset.id,
        "rootId": asset.root_id,
        "relativePath": asset.relative_path,
        "mediaType": asset.media_type,
        "size": asset.size,
        "modTime": asset.mod_time,
        "sha256": asset.sha256,
        "availability": asset.availability,
        "link": {
            "runId": run_id,
            "taskKey": task_key,
            "sampleId": if sample_id.is_empty() { Value::Null } else { Value::String(sample_id.to_owned()) },
            "wellId": coordinates["wellId"],
            "timepoint": coordinates["timepoint"],
            "fovIndex": coordinates["fovIndex"],
            "coordinateId": coordinates["coordinateId"]
        },
        "linkedAt": linked_at,
        "actorId": actor_id
    })
}

pub(crate) fn asset_link_event_data(
    event_id: &str,
    action: &str,
    asset: &asset_refs::AssetRef,
    run_id: &str,
    task_key: &str,
    sample_id: &str,
    at: &str,
    actor_id: &str,
    coordinates: &Value,
) -> Value {
    json!({
        "id": event_id,
        "action": action,
        "assetId": asset.id,
        "rootId": asset.root_id,
        "relativePath": asset.relative_path,
        "mediaType": asset.media_type,
        "size": asset.size,
        "modTime": asset.mod_time,
        "sha256": asset.sha256,
        "availability": asset.availability,
        "link": {
            "runId": run_id,
            "taskKey": task_key,
            "sampleId": if sample_id.is_empty() { Value::Null } else { Value::String(sample_id.to_owned()) },
            "wellId": coordinates["wellId"],
            "timepoint": coordinates["timepoint"],
            "fovIndex": coordinates["fovIndex"],
            "coordinateId": coordinates["coordinateId"]
        },
        "at": at,
        "actorId": actor_id
    })
}

/// Writes the durable link and its append-only event in one transaction.
pub(crate) fn store_asset_link(
    db: &mut Connection,
    asset: &asset_refs::AssetRef,
    project: &str,
    run_id: &str,
    task_key: &str,
    sample_id: &str,
    linked_at: &str,
    actor_id: &str,
    coordinates: &Value,
) -> Result<(Value, Value), CliError> {
    let record_id = asset_link_record_id(asset, run_id, task_key, sample_id, coordinates);
    let data = asset_link_data(asset, run_id, task_key, sample_id, linked_at, actor_id, coordinates);
    let event_id = format!("asset-event-{}", Uuid::new_v4());
    let event = asset_link_event_data(
        &event_id, "linked", asset, run_id, task_key, sample_id, linked_at, actor_id, coordinates,
    );
    let tx = db
        .transaction_with_behavior(rusqlite::TransactionBehavior::Immediate)
        .map_err(|e| CliError::internal(e.to_string()))?;
    let exists: bool = tx
        .query_row("SELECT 1 FROM records WHERE id=?1", [&record_id], |_| Ok(()))
        .optional()
        .map_err(|e| CliError::internal(e.to_string()))?
        .is_some();
    if exists {
        tx.execute(
            "UPDATE records SET data_json=?1, updated_at=?2 WHERE id=?3",
            params![
                serde_json::to_string(&data).map_err(|e| CliError::internal(e.to_string()))?,
                stamp(),
                record_id
            ],
        )
        .map_err(|e| CliError::internal(e.to_string()))?;
    } else {
        tx.execute(
            "INSERT INTO records(id,kind,project_id,parent_id,revision,data_json,created_at,updated_at) VALUES(?1,?2,?3,?4,?5,?6,?7,?7)",
            params![
                record_id,
                "asset_link",
                project,
                run_id,
                1i64,
                serde_json::to_string(&data).map_err(|e| CliError::internal(e.to_string()))?,
                stamp()
            ],
        )
        .map_err(|e| CliError::internal(e.to_string()))?;
    }
    tx.execute(
        "INSERT INTO records(id,kind,project_id,parent_id,revision,data_json,created_at,updated_at) VALUES(?1,?2,?3,?4,?5,?6,?7,?7)",
        params![
            event_id,
            "asset_link_event",
            project,
            run_id,
            1i64,
            serde_json::to_string(&event).map_err(|e| CliError::internal(e.to_string()))?,
            stamp()
        ],
    )
    .map_err(|e| CliError::internal(e.to_string()))?;
    tx.commit()
        .map_err(|e| CliError::internal(e.to_string()))?;
    Ok((get(db, &record_id)?, get(db, &event_id)?))
}

/// `asset.link`: computes the file's observed metadata and SHA-256, then stores
/// only the logical root, relative path and metadata. The file itself is never
/// copied; `root_dir` is used for this call only and is not persisted.
pub(crate) fn link_local_asset(
    db: &mut Connection,
    request: &AssetLinkCreate,
) -> Result<(Value, Value), CliError> {
    let root_id = request.root_id.trim();
    check_asset_root_id(root_id)?;
    let run_id = request.run_id.trim();
    let task_key = request.task_key.trim();
    let sample_id = request.sample_id.as_deref().unwrap_or("").trim();
    check_asset_sample_id(sample_id)?;
    let coordinates = asset_coordinates(
        request.well_id.as_deref(),
        request.timepoint.as_deref(),
        request.fov_index,
    );
    validate_asset_coordinates(
        request.well_id.as_deref(),
        request.timepoint.as_deref(),
        request.fov_index,
    )?;
    let actor_id = request.actor_id.as_deref().unwrap_or("").trim();
    if actor_id.len() > 200 || actor_id.chars().any(char::is_control) {
        return Err(CliError::input("actor_id is invalid"));
    }
    let root_dir = request.root_dir.as_str();
    if root_dir.is_empty() {
        return Err(CliError::input("root_dir is required"));
    }
    // Validate the Run/task before reading a potentially large file.
    let project = asset_link_scope(db, run_id, task_key)?;
    let asset = asset_refs::link(
        root_id,
        Path::new(root_dir),
        request.path.as_deref(),
        request.relative_path.as_deref(),
    )
    .map_err(|error| CliError::input(error.to_string()))?;
    store_asset_link(
        db, &asset, &project, run_id, task_key, sample_id,
        &stamp(), actor_id, &coordinates,
    )
}

/// Legacy `save_asset_link` input used by the Run UI module: the caller has
/// already observed the metadata. The stored link and a `linked` event are
/// written together.
pub(crate) fn save_asset_link(db: &mut Connection, input: &Value) -> Result<Value, CliError> {
    let asset = input
        .get("asset")
        .ok_or_else(|| CliError::input("asset is required"))?;
    let root_id = asset
        .get("rootId")
        .and_then(Value::as_str)
        .unwrap_or("")
        .trim()
        .to_owned();
    check_asset_root_id(&root_id)?;
    let relative_path = asset_refs::normalize_relative_path(
        asset
            .get("relativePath")
            .and_then(Value::as_str)
            .unwrap_or(""),
    )
    .map_err(|error| CliError::input(error.to_string()))?;

    let link = asset
        .get("link")
        .ok_or_else(|| CliError::input("asset link is required"))?;
    let run_id = link
        .get("runId")
        .and_then(Value::as_str)
        .unwrap_or("")
        .trim()
        .to_owned();
    if run_id.is_empty() {
        return Err(CliError::input("runId is required"));
    }
    let task_key = link
        .get("taskKey")
        .and_then(Value::as_str)
        .unwrap_or("")
        .trim()
        .to_owned();
    let sample_id = link
        .get("sampleId")
        .and_then(Value::as_str)
        .unwrap_or("")
        .trim()
        .to_owned();
    check_asset_sample_id(&sample_id)?;
    let project = asset_link_scope(db, &run_id, &task_key)?;

    let media_type: asset_refs::MediaType = serde_json::from_value(json!(
        asset
            .get("mediaType")
            .and_then(Value::as_str)
            .unwrap_or("unknown")
    ))
    .map_err(|_| CliError::input("mediaType is invalid"))?;
    let size = asset.get("size").and_then(Value::as_i64).unwrap_or(0);
    if size < 0 {
        return Err(CliError::input("size must not be negative"));
    }
    let mod_time = asset
        .get("modTime")
        .and_then(Value::as_str)
        .unwrap_or("")
        .to_owned();
    let sha256 = asset
        .get("sha256")
        .and_then(Value::as_str)
        .unwrap_or("")
        .to_owned();
    if !sha256.is_empty()
        && (sha256.len() != 64 || !sha256.chars().all(|c| c.is_ascii_hexdigit()))
    {
        return Err(CliError::input("sha256 must be 64 hexadecimal characters"));
    }
    let availability: asset_refs::Availability = serde_json::from_value(json!(
        asset
            .get("availability")
            .and_then(Value::as_str)
            .unwrap_or("available")
    ))
    .map_err(|_| CliError::input("availability is invalid"))?;
    let linked_at = asset
        .get("linkedAt")
        .and_then(Value::as_str)
        .unwrap_or("")
        .to_owned();
    let linked_at = if linked_at.is_empty() { stamp() } else { linked_at };
    let actor_id = asset
        .get("actorId")
        .and_then(Value::as_str)
        .unwrap_or("")
        .to_owned();
    let coordinates = asset_coordinates(
        link.get("wellId").and_then(Value::as_str),
        link.get("timepoint").and_then(Value::as_str),
        link.get("fovIndex").and_then(Value::as_u64).map(|v| v as u32),
    );
    validate_asset_coordinates(
        link.get("wellId").and_then(Value::as_str),
        link.get("timepoint").and_then(Value::as_str),
        link.get("fovIndex").and_then(Value::as_u64).map(|v| v as u32),
    )?;

    let reference = asset_refs::AssetRef {
        id: asset_refs::stable_id(&root_id, &relative_path),
        root_id,
        relative_path,
        media_type,
        size: size as u64,
        mod_time,
        sha256,
        availability,
    };
    let (record, _) = store_asset_link(
        db, &reference, &project, &run_id, &task_key, &sample_id, &linked_at, &actor_id, &coordinates,
    )?;
    Ok(record["data"].clone())
}

pub(crate) fn list_asset_links(db: &Connection, run_id: &str) -> Result<Value, CliError> {
    let (kind, _, _, _, _) =
        kind_id(db, run_id)?.ok_or_else(|| CliError::missing("实验运行不存在"))?;
    if kind != "run" {
        return Err(CliError::input("run_id must point to a run"));
    }
    let mut stmt = db
        .prepare("SELECT data_json FROM records WHERE kind='asset_link' AND parent_id=?1 ORDER BY created_at ASC, rowid ASC")
        .map_err(|e| CliError::internal(e.to_string()))?;
    let rows = stmt
        .query_map([run_id], |row| row.get::<_, String>(0))
        .map_err(|e| CliError::internal(e.to_string()))?;
    let mut out = Vec::new();
    for row in rows.flatten() {
        if let Ok(v) = serde_json::from_str::<Value>(&row) {
            out.push(v);
        }
    }
    Ok(Value::Array(out))
}

/// Removes only the reference; the referenced file is never touched.
pub(crate) fn unlink_asset_link(
    db: &mut Connection,
    id: &str,
    actor_id: &str,
) -> Result<(Value, Value), CliError> {
    if actor_id.len() > 200 || actor_id.chars().any(char::is_control) {
        return Err(CliError::input("actor_id is invalid"));
    }
    let record = get(db, id)?;
    if record["kind"] != "asset_link" {
        return Err(CliError::input("id must point to an asset_link"));
    }
    let data = record["data"].clone();
    let run_id = record["parent_id"]
        .as_str()
        .ok_or_else(|| CliError::internal("asset_link 缺少所属 Run"))?;
    if !has_kind(db, run_id, "run")? {
        return Err(CliError::internal("asset_link 所属 Run 不存在"));
    }
    let project = record["project_id"].as_str().unwrap_or("");
    let at = stamp();
    let actor_id = if actor_id.is_empty() {
        data["actorId"].as_str().unwrap_or("")
    } else {
        actor_id
    };
    let event_id = format!("asset-event-{}", Uuid::new_v4());
    let event = asset_link_event_from_data(&event_id, "unlinked", &data, &at, actor_id, None);
    let tx = db
        .transaction_with_behavior(rusqlite::TransactionBehavior::Immediate)
        .map_err(|e| CliError::internal(e.to_string()))?;
    tx.execute("DELETE FROM records WHERE id=?1", [id])
        .map_err(|e| CliError::internal(e.to_string()))?;
    tx.execute(
        "INSERT INTO records(id,kind,project_id,parent_id,revision,data_json,created_at,updated_at) VALUES(?1,?2,?3,?4,?5,?6,?7,?7)",
        params![
            event_id,
            "asset_link_event",
            project,
            run_id,
            1i64,
            serde_json::to_string(&event).map_err(|e| CliError::internal(e.to_string()))?,
            at
        ],
    )
    .map_err(|e| CliError::internal(e.to_string()))?;
    tx.commit()
        .map_err(|e| CliError::internal(e.to_string()))?;
    Ok((record, get(db, &event_id)?))
}

/// Builds one `asset_link_event` payload from the link's durable data so the
/// event always mirrors the record it describes. `previous` is present only on
/// `updated` events and carries the replaced coordinate values.
pub(crate) fn asset_link_event_from_data(
    event_id: &str,
    action: &str,
    data: &Value,
    at: &str,
    actor_id: &str,
    previous: Option<Value>,
) -> Value {
    let mut event = json!({
        "id": event_id,
        "action": action,
        "assetId": data["id"],
        "rootId": data["rootId"],
        "relativePath": data["relativePath"],
        "mediaType": data["mediaType"],
        "size": data["size"],
        "modTime": data["modTime"],
        "sha256": data["sha256"],
        "availability": data["availability"],
        "link": data["link"],
        "at": at,
        "actorId": actor_id
    });
    if let Some(previous) = previous {
        event["previous"] = previous;
    }
    event
}

/// `asset.link.update`: replaces the coordinate triple of one existing link.
///
/// Only `link.wellId/timepoint/fovIndex/coordinateId` change. The file identity
/// (`id`, `rootId`, `relativePath`, `sha256`, size, modTime, mediaType,
/// availability) and the association (`runId`, `taskKey`, `sampleId`) are
/// copied verbatim. The record id is the link's creation handle and is not
/// recomputed. The update and the `updated` event land in one transaction; a
/// validation failure leaves both untouched.
///
/// Crate-visible because the MyGo native bridge exposes the same command as
/// `RunService.UpdateAssetLinkCoordinates`; there is exactly one implementation
/// of the coordinate rules.
pub(crate) fn update_asset_link(
    db: &mut Connection,
    request: &AssetLinkUpdateInput,
) -> Result<(Value, Value), CliError> {
    let id = request.id.trim();
    let actor_id = request.actor_id.as_deref().unwrap_or("").trim();
    if actor_id.len() > 200 || actor_id.chars().any(char::is_control) {
        return Err(CliError::input("actor_id is invalid"));
    }
    let well_id = request.well_id.as_str();
    let timepoint = request
        .timepoint
        .clone()
        .ok_or_else(|| CliError::input("timepoint is required; pass null to clear it"))?;
    let fov_index = request
        .fov_index
        .ok_or_else(|| CliError::input("fov_index is required; pass null to clear it"))?;
    validate_asset_coordinates(Some(well_id), timepoint.as_deref(), fov_index)?;

    let record = get(db, id)?;
    if record["kind"] != "asset_link" {
        return Err(CliError::input("id must point to an asset_link"));
    }
    let run_id = record["parent_id"]
        .as_str()
        .ok_or_else(|| CliError::internal("asset_link 缺少所属 Run"))?;
    if !has_kind(db, run_id, "run")? {
        return Err(CliError::internal("asset_link 所属 Run 不存在"));
    }
    let project = record["project_id"].as_str().unwrap_or("");
    let coordinates = asset_coordinates(Some(well_id), timepoint.as_deref(), fov_index);
    let at = stamp();
    let actor_id = if actor_id.is_empty() {
        record["data"]["actorId"].as_str().unwrap_or("")
    } else {
        actor_id
    };

    let mut data = record["data"].clone();
    let link = data
        .get_mut("link")
        .and_then(Value::as_object_mut)
        .ok_or_else(|| CliError::internal("asset_link 缺少 link"))?;
    let previous = json!({
        "wellId": link.get("wellId").cloned().unwrap_or(Value::Null),
        "timepoint": link.get("timepoint").cloned().unwrap_or(Value::Null),
        "fovIndex": link.get("fovIndex").cloned().unwrap_or(Value::Null),
        "coordinateId": link.get("coordinateId").cloned().unwrap_or(Value::Null)
    });
    for field in ["wellId", "timepoint", "fovIndex", "coordinateId"] {
        link.insert(field.to_owned(), coordinates[field].clone());
    }
    let data_json =
        serde_json::to_string(&data).map_err(|e| CliError::internal(e.to_string()))?;
    let event_id = format!("asset-event-{}", Uuid::new_v4());
    let event = asset_link_event_from_data(&event_id, "updated", &data, &at, actor_id, Some(previous));
    let event_json =
        serde_json::to_string(&event).map_err(|e| CliError::internal(e.to_string()))?;

    let tx = db
        .transaction_with_behavior(rusqlite::TransactionBehavior::Immediate)
        .map_err(|e| CliError::internal(e.to_string()))?;
    let updated = tx
        .execute(
            "UPDATE records SET data_json=?1, updated_at=?2 WHERE id=?3",
            params![data_json, at, id],
        )
        .map_err(|e| CliError::internal(e.to_string()))?;
    if updated != 1 {
        return Err(CliError::internal("asset_link 在更新时消失"));
    }
    tx.execute(
        "INSERT INTO records(id,kind,project_id,parent_id,revision,data_json,created_at,updated_at) VALUES(?1,?2,?3,?4,?5,?6,?7,?7)",
        params![event_id, "asset_link_event", project, run_id, 1i64, event_json, at],
    )
    .map_err(|e| CliError::internal(e.to_string()))?;
    tx.commit()
        .map_err(|e| CliError::internal(e.to_string()))?;
    Ok((get(db, id)?, get(db, &event_id)?))
}

pub(crate) fn list_asset_events(
    db: &Connection,
    run_id: &str,
    action: Option<&str>,
) -> Result<Value, CliError> {
    if let Some(action) = action {
        if !matches!(action, "linked" | "unlinked" | "updated") {
            return Err(CliError::input(
                "action must be linked, unlinked or updated",
            ));
        }
    }
    let (kind, _, _, _, _) =
        kind_id(db, run_id)?.ok_or_else(|| CliError::missing("实验运行不存在"))?;
    if kind != "run" {
        return Err(CliError::input("run_id must point to a run"));
    }
    let mut stmt = db
        .prepare("SELECT id,kind,project_id,parent_id,revision,data_json,created_at,updated_at FROM records WHERE kind='asset_link_event' AND parent_id=?1 ORDER BY created_at ASC, rowid ASC")
        .map_err(|e| CliError::internal(e.to_string()))?;
    let rows = stmt
        .query_map([run_id], |row| {
            let raw: String = row.get(5)?;
            let data: Value = serde_json::from_str(&raw).unwrap_or(Value::Null);
            Ok(json!({
                "id": row.get::<_, String>(0)?,
                "kind": row.get::<_, String>(1)?,
                "project_id": row.get::<_, Option<String>>(2)?,
                "parent_id": row.get::<_, Option<String>>(3)?,
                "revision": row.get::<_, i64>(4)?,
                "data": data,
                "created_at": row.get::<_, String>(6)?,
                "updated_at": row.get::<_, String>(7)?
            }))
        })
        .map_err(|e| CliError::internal(e.to_string()))?;
    let mut out = Vec::new();
    for row in rows {
        let record = row.map_err(|e| CliError::internal(e.to_string()))?;
        if action.is_some_and(|action| record["data"]["action"] != action) {
            continue;
        }
        out.push(record);
    }
    Ok(Value::Array(out))
}

