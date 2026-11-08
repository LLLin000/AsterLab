#![recursion_limit = "256"]

use axum::{
    extract::{Path, Query, State},
    http::{HeaderMap, StatusCode},
    response::{Html, IntoResponse, Response},
    routing::{get, post},
    Json, Router,
};
use chrono::Utc;
use rusqlite::{params, Connection, OptionalExtension, Transaction};
use serde::Deserialize;
use serde_json::{json, Value};
use std::{
    env, fs,
    net::SocketAddr,
    path::PathBuf,
    sync::{Arc, Mutex},
};
use uuid::Uuid;

mod cli;
const INDEX_HTML: &str = include_str!("../web/index.html");
const DESIGN_PREVIEW_HTML: &str = include_str!("../docs/prototypes/tabler-vs-dify-2026-11-06/index.html");
const VIEW_HTML: &str = include_str!("../web/view.html");
const ICON_PLATE: &str = include_str!("../web/icons/plate.svg");
const ICON_PIPETTE: &str = include_str!("../web/icons/pipette.svg");
const ICON_INCUBATOR: &str = include_str!("../web/icons/incubator.svg");
const ICON_READER: &str = include_str!("../web/icons/reader.svg");
const ICON_FLASK: &str = include_str!("../web/icons/flask.svg");
const APP_JS: &str = include_str!("../web/app.js");
const PAGES_JS: &str = include_str!("../web/pages.js");
const RESOURCES_JS: &str = include_str!("../web/resources.js");
const PROTOCOL_PAGE_JS: &str = include_str!("../web/pages/protocol.js");
const RUN_PAGE_JS: &str = include_str!("../web/pages/run.js");
const MARKED_JS: &str = include_str!("../web/vendor/marked.min.js");
const PURIFY_JS: &str = include_str!("../web/vendor/purify.min.js");
const DAGRE_JS: &str = include_str!("../web/vendor/dagre.min.js");
const LAYOUT_JS: &str = include_str!("../web/layout.js");
const ICON_INLINE_JS: &str = include_str!("../web/icons/inline.js");
const TABLER_CSS: &str = include_str!("../web/vendor/tabler/tabler.min.css");
const TABLER_JS: &str = include_str!("../web/vendor/tabler/tabler.min.js");
const COMPONENT_FLOW_JS: &str = include_str!("../web/components/flow.js");
const COMPONENT_PROPOSAL_JS: &str = include_str!("../web/components/proposal.js");
const COMPONENT_PROPOSAL_CSS: &str = include_str!("../web/components/proposal.css");
const COMPONENT_MARKDOWN_JS: &str = include_str!("../web/components/markdown.js");
const COMPONENT_MARKDOWN_CSS: &str = include_str!("../web/components/markdown.css");
const COMPONENT_FLOW_CSS: &str = include_str!("../web/components/flow.css");
const COMPONENT_CALCULATOR_JS: &str = include_str!("../web/components/calculator.js");
const COMPONENT_CALCULATOR_CSS: &str = include_str!("../web/components/calculator.css");
const COMPONENT_TABLE_JS: &str = include_str!("../web/components/table.js");
const COMPONENT_TABLE_CSS: &str = include_str!("../web/components/table.css");
const COMPONENT_WELLPLATE_JS: &str = include_str!("../web/components/wellplate.js");
const COMPONENT_WELLPLATE_CSS: &str = include_str!("../web/components/wellplate.css");
const COMPONENT_TOKENS: &str = include_str!("../web/components/tokens.css");
#[derive(Clone)]
struct AppState {
    db: Arc<Mutex<Connection>>,
    data_dir: PathBuf,
    db_path: PathBuf,
}
#[derive(Debug)]
struct ApiError {
    status: StatusCode,
    message: String,
}
impl ApiError {
    fn bad(message: impl Into<String>) -> Self {
        Self {
            status: StatusCode::BAD_REQUEST,
            message: message.into(),
        }
    }
    fn missing(message: impl Into<String>) -> Self {
        Self {
            status: StatusCode::NOT_FOUND,
            message: message.into(),
        }
    }
    fn internal(message: impl Into<String>) -> Self {
        Self {
            status: StatusCode::INTERNAL_SERVER_ERROR,
            message: message.into(),
        }
    }
    fn from_cli(error: cli::CliError) -> Self {
        let status = match error.code {
            "NOT_FOUND" => StatusCode::NOT_FOUND,
            "REVISION_CONFLICT" => StatusCode::CONFLICT,
            "INTERNAL" => StatusCode::INTERNAL_SERVER_ERROR,
            _ => StatusCode::BAD_REQUEST,
        };
        Self {
            status,
            message: error.message,
        }
    }
}
impl From<rusqlite::Error> for ApiError {
    fn from(e: rusqlite::Error) -> Self {
        Self::internal(format!("数据库操作失败：{e}"))
    }
}
impl From<serde_json::Error> for ApiError {
    fn from(e: serde_json::Error) -> Self {
        Self::bad(format!("数据格式无效：{e}"))
    }
}
impl IntoResponse for ApiError {
    fn into_response(self) -> Response {
        (self.status, Json(json!({"error":self.message}))).into_response()
    }
}
type ApiResult<T> = Result<Json<T>, ApiError>;
fn now() -> String {
    Utc::now().to_rfc3339()
}
fn id() -> String {
    Uuid::new_v4().to_string()
}
fn lock(s: &AppState) -> Result<std::sync::MutexGuard<'_, Connection>, ApiError> {
    s.db.lock()
        .map_err(|_| ApiError::internal("数据库锁不可用"))
}
fn text<'a>(v: &'a Value, key: &str) -> &'a str {
    v.get(key).and_then(Value::as_str).unwrap_or("").trim()
}
fn required(v: &Value, key: &str, label: &str, max: usize) -> Result<String, ApiError> {
    let s = text(v, key);
    if s.is_empty() {
        return Err(ApiError::bad(format!("{label}不能为空")));
    }
    if s.chars().count() > max {
        return Err(ApiError::bad(format!("{label}不能超过 {max} 个字符")));
    }
    Ok(s.to_owned())
}
fn record(
    db: &Connection,
    record_id: &str,
) -> Result<Option<(String, Option<String>, Option<String>, i64, String)>, ApiError> {
    Ok(db
        .query_row(
            "SELECT kind,project_id,parent_id,revision,data_json FROM records WHERE id=?1",
            [record_id],
            |r| Ok((r.get(0)?, r.get(1)?, r.get(2)?, r.get(3)?, r.get(4)?)),
        )
        .optional()?)
}
fn insert(
    tx: &Transaction<'_>,
    kind: &str,
    project: Option<&str>,
    parent: Option<&str>,
    revision: i64,
    data: &Value,
) -> Result<Value, ApiError> {
    let record_id = id();
    let stamp = now();
    tx.execute("INSERT INTO records(id,kind,project_id,parent_id,revision,data_json,created_at,updated_at) VALUES(?1,?2,?3,?4,?5,?6,?7,?7)",
       params![record_id,kind,project,parent,revision,serde_json::to_string(data)?,stamp])?;
    Ok(
        json!({"id":record_id,"kind":kind,"project_id":project,"parent_id":parent,"revision":revision,"data":data,"created_at":stamp,"updated_at":stamp}),
    )
}
fn list(db: &Connection) -> Result<Vec<Value>, ApiError> {
    let mut s=db.prepare("SELECT id,kind,project_id,parent_id,revision,data_json,created_at,updated_at FROM records ORDER BY created_at DESC,rowid DESC")?;
    let rows=s.query_map([],|r| {
        let raw:String=r.get(5)?;
        let data:Value=serde_json::from_str(&raw).unwrap_or(Value::Null);
        Ok(json!({"id":r.get::<_,String>(0)?,"kind":r.get::<_,String>(1)?,"project_id":r.get::<_,Option<String>>(2)?,"parent_id":r.get::<_,Option<String>>(3)?,"revision":r.get::<_,i64>(4)?,"data":data,"created_at":r.get::<_,String>(6)?,"updated_at":r.get::<_,String>(7)?}))
    })?;
    Ok(rows.filter_map(Result::ok).collect())
}
async fn index() -> Html<&'static str> {
    Html(INDEX_HTML)
}
async fn view_page() -> Html<&'static str> {
    Html(VIEW_HTML)
}
async fn design_preview_page() -> Html<&'static str> {
    Html(DESIGN_PREVIEW_HTML)
}

async fn marked_js() -> Response {
    (
        [(axum::http::header::CONTENT_TYPE, "text/javascript")],
        MARKED_JS,
    )
        .into_response()
}

async fn tabler_css() -> Response {
    ([(axum::http::header::CONTENT_TYPE, "text/css")], TABLER_CSS).into_response()
}

async fn tabler_js() -> Response {
    (
        [(axum::http::header::CONTENT_TYPE, "text/javascript")],
        TABLER_JS,
    )
        .into_response()
}

async fn icon_inline_js() -> Response {
    (
        [(axum::http::header::CONTENT_TYPE, "text/javascript")],
        ICON_INLINE_JS,
    )
        .into_response()
}

async fn component_asset(Path(name): Path<String>) -> Response {
    let body = match name.as_str() {
        "tokens.css" => COMPONENT_TOKENS,
        "wellplate.css" => COMPONENT_WELLPLATE_CSS,
        "wellplate.js" => COMPONENT_WELLPLATE_JS,
        "table.css" => COMPONENT_TABLE_CSS,
        "table.js" => COMPONENT_TABLE_JS,
        "calculator.css" => COMPONENT_CALCULATOR_CSS,
        "calculator.js" => COMPONENT_CALCULATOR_JS,
        "flow.css" => COMPONENT_FLOW_CSS,
        "flow.js" => COMPONENT_FLOW_JS,
        "markdown.css" => COMPONENT_MARKDOWN_CSS,
        "markdown.js" => COMPONENT_MARKDOWN_JS,
        "proposal.css" => COMPONENT_PROPOSAL_CSS,
        "proposal.js" => COMPONENT_PROPOSAL_JS,
        _ => return StatusCode::NOT_FOUND.into_response(),
    };
    let content_type = if name.ends_with(".css") {
        "text/css"
    } else {
        "text/javascript"
    };
    ([(axum::http::header::CONTENT_TYPE, content_type)], body).into_response()
}
async fn page_asset(Path(name): Path<String>) -> Response {
    let body = match name.as_str() {
        "protocol.js" => PROTOCOL_PAGE_JS,
        "run.js" => RUN_PAGE_JS,
        _ => return StatusCode::NOT_FOUND.into_response(),
    };
    ([(axum::http::header::CONTENT_TYPE, "text/javascript")], body).into_response()
}


async fn layout_js() -> Response {
    (
        [(axum::http::header::CONTENT_TYPE, "text/javascript")],
        LAYOUT_JS,
    )
        .into_response()
}

async fn dagre_js() -> Response {
    (
        [(axum::http::header::CONTENT_TYPE, "text/javascript")],
        DAGRE_JS,
    )
        .into_response()
}

async fn purify_js() -> Response {
    (
        [(axum::http::header::CONTENT_TYPE, "text/javascript")],
        PURIFY_JS,
    )
        .into_response()
}

async fn app_js() -> Response {
    (
        [(axum::http::header::CONTENT_TYPE, "text/javascript")],
        APP_JS,
    )
        .into_response()
}
async fn pages_js() -> Response {
    (
        [(axum::http::header::CONTENT_TYPE, "text/javascript")],
        PAGES_JS,
    )
        .into_response()
}
async fn resources_js() -> Response {
    (
        [(axum::http::header::CONTENT_TYPE, "text/javascript")],
        RESOURCES_JS,
    )
        .into_response()
}

async fn icon(Path(name): Path<String>) -> Response {
    let name = name.trim_end_matches(".svg");
    let svg = match name {
        "plate" => ICON_PLATE,
        "pipette" => ICON_PIPETTE,
        "incubator" => ICON_INCUBATOR,
        "reader" => ICON_READER,
        "flask" => ICON_FLASK,
        _ => return StatusCode::NOT_FOUND.into_response(),
    };
    (
        [(axum::http::header::CONTENT_TYPE, "image/svg+xml")],
        svg.as_bytes().to_vec(),
    )
        .into_response()
}
async fn get_view(State(s): State<AppState>, Path(id): Path<String>) -> ApiResult<Value> {
    let db = lock(&s)?;
    let result = cli::get_view(&db, &id).map_err(ApiError::from_cli)?;
    Ok(Json(result))
}
#[derive(Deserialize)]
struct ViewDataQuery {
    instance_id: String,
    #[serde(default)]
    position_ids: Option<String>,
    #[serde(default)]
    calculator_instance_id: Option<String>,
    #[serde(default)]
    volume_per_well: Option<f64>,
    #[serde(default)]
    volume_unit: Option<String>,
    #[serde(default)]
    extra_ratio: Option<f64>,
}
async fn get_view_selection(
    State(s): State<AppState>,
    Path((id, instance_id)): Path<(String, String)>,
) -> ApiResult<Value> {
    let db = lock(&s)?;
    let result = cli::view_selection_context(&db, &id, &instance_id).map_err(ApiError::from_cli)?;
    Ok(Json(result))
}

#[derive(Deserialize)]
struct InboxProposalInput {
    changeset: Value,
    #[serde(default)]
    source: String,
}

async fn list_view_proposals(
    State(s): State<AppState>,
    Path(id): Path<String>,
) -> ApiResult<Value> {
    let db = lock(&s)?;
    let result = cli::list_view_proposals(&db, &id).map_err(ApiError::from_cli)?;
    Ok(Json(result))
}

async fn create_view_proposal(
    State(s): State<AppState>,
    Path(id): Path<String>,
    Json(input): Json<InboxProposalInput>,
) -> ApiResult<Value> {
    let db = lock(&s)?;
    let tx = db
        .unchecked_transaction()
        .map_err(|error| ApiError::internal(error.to_string()))?;
    let result = cli::create_view_proposal_tx(&tx, &id, &input.changeset, &input.source)
        .map_err(ApiError::from_cli)?;
    tx.commit()
        .map_err(|error| ApiError::internal(error.to_string()))?;
    Ok(Json(result))
}

async fn accept_view_proposal(
    State(s): State<AppState>,
    Path((id, proposal_id)): Path<(String, String)>,
) -> ApiResult<Value> {
    let db = lock(&s)?;
    let tx = db
        .unchecked_transaction()
        .map_err(|error| ApiError::internal(error.to_string()))?;
    let result =
        cli::accept_view_proposal_tx(&tx, &id, &proposal_id).map_err(ApiError::from_cli)?;
    tx.commit()
        .map_err(|error| ApiError::internal(error.to_string()))?;
    Ok(Json(result))
}

async fn reject_view_proposal(
    State(s): State<AppState>,
    Path((id, proposal_id)): Path<(String, String)>,
) -> ApiResult<Value> {
    let db = lock(&s)?;
    let tx = db
        .unchecked_transaction()
        .map_err(|error| ApiError::internal(error.to_string()))?;
    let result =
        cli::reject_view_proposal_tx(&tx, &id, &proposal_id).map_err(ApiError::from_cli)?;
    tx.commit()
        .map_err(|error| ApiError::internal(error.to_string()))?;
    Ok(Json(result))
}

#[derive(Deserialize)]
struct ProposalInput {
    changeset: asterlab_core::component_framework::ViewChangeSet,
}

async fn propose_view(
    State(s): State<AppState>,
    Path(id): Path<String>,
    Json(input): Json<ProposalInput>,
) -> ApiResult<Value> {
    let db = lock(&s)?;
    let result = cli::view_proposal(&db, &id, &input.changeset).map_err(ApiError::from_cli)?;
    Ok(Json(result))
}

async fn get_view_step_detail(
    State(s): State<AppState>,
    Path((id, instance_id)): Path<(String, String)>,
) -> ApiResult<Value> {
    let db = lock(&s)?;
    let result = cli::view_step_detail(&db, &id, &instance_id).map_err(ApiError::from_cli)?;
    Ok(Json(result))
}

async fn get_view_data(
    State(s): State<AppState>,
    Path(id): Path<String>,
    Query(query): Query<ViewDataQuery>,
) -> ApiResult<Value> {
    let db = lock(&s)?;
    let position_ids = query
        .position_ids
        .as_deref()
        .unwrap_or_default()
        .split(',')
        .map(str::trim)
        .filter(|value| !value.is_empty())
        .collect::<Vec<_>>();
    let result = cli::view_data_preview(
        &db,
        &json!({
            "view_id": id,
            "instance_id": query.instance_id,
            "position_ids": position_ids,
            "calculator_instance_id": query.calculator_instance_id,
            "volume_per_well": query.volume_per_well,
            "volume_unit": query.volume_unit,
            "extra_ratio": query.extra_ratio
        }),
    )
    .map_err(ApiError::from_cli)?;
    Ok(Json(result))
}
#[derive(Deserialize)]
struct ViewReviseInput {
    changeset: Value,
}
async fn revise_view(
    State(s): State<AppState>,
    Path(id): Path<String>,
    Json(input): Json<ViewReviseInput>,
) -> ApiResult<Value> {
    let mut db = lock(&s)?;
    let tx = db
        .transaction()
        .map_err(|e| ApiError::internal(e.to_string()))?;
    let result = cli::revise_view_tx(&tx, &id, &input.changeset).map_err(ApiError::from_cli)?;
    tx.commit().map_err(|e| ApiError::internal(e.to_string()))?;
    Ok(Json(result))
}
async fn health() -> Json<Value> {
    Json(json!({"ok":true,"app":"AsterLab","storage":"SQLite","agent":"typed-cli-v1"}))
}
async fn get_state(State(s): State<AppState>) -> ApiResult<Value> {
    let rows = {
        let db = lock(&s)?;
        list(&db)?
    };
    Ok(Json(json!({"records":rows})))
}
#[derive(Deserialize)]
struct BlockPreviewInput {
    block_version_id: String,
    #[serde(default = "empty_array")]
    bindings: Value,
}
fn empty_array() -> Value {
    json!([])
}
async fn preview_block(
    State(s): State<AppState>,
    Json(input): Json<BlockPreviewInput>,
) -> ApiResult<Value> {
    let db = lock(&s)?;
    let result = cli::preview_protocol_block(&db, &input.block_version_id, &input.bindings)
        .map_err(ApiError::from_cli)?;
    Ok(Json(result))
}

async fn get_design_map(State(s): State<AppState>, Path(id): Path<String>) -> ApiResult<Value> {
    let db = lock(&s)?;
    let result = cli::design_map(&db, &id).map_err(ApiError::from_cli)?;
    Ok(Json(result))
}

async fn ensure_design_workflow(State(s): State<AppState>, Path(id): Path<String>) -> ApiResult<Value> {
    let mut db = lock(&s)?;
    let result = cli::ensure_design_workflow(&mut db, &id).map_err(ApiError::from_cli)?;
    Ok(Json(result))
}

async fn get_run_workspace(State(s): State<AppState>, Path(id): Path<String>) -> ApiResult<Value> {
    let db = lock(&s)?;
    let result = cli::run_workspace(&db, &id).map_err(ApiError::from_cli)?;
    Ok(Json(result))
}

#[derive(Deserialize)]
struct CreateInput {
    kind: String,
    project_id: Option<String>,
    parent_id: Option<String>,
    #[serde(default)]
    data: Value,
}
async fn create_record(
    State(s): State<AppState>,
    headers: HeaderMap,
    Json(input): Json<CreateInput>,
) -> ApiResult<Value> {
    let mut db = lock(&s)?;
    let tx = db.transaction()?;
    let kind = input.kind.as_str();
    let mut project = input.project_id.clone();
    let parent = input.parent_id.clone();
    let mut data = input.data;
    let result = match kind {
        "project" => {
            required(&data, "name", "项目名称", 120)?;
            data["description"] = json!(text(&data, "description"));
            insert(&tx, "project", None, None, 1, &data)?
        }
        "design" => {
            let pid = parent
                .as_deref()
                .ok_or_else(|| ApiError::bad("实验设计需要归属项目"))?;
            if record(&tx, pid)?.map(|x| x.0) != Some("project".into()) {
                return Err(ApiError::bad("项目不存在"));
            }
            data = cli::normalize_validate_design_data(data, None).map_err(ApiError::from_cli)?;
            project = Some(pid.to_owned());
            let design = insert(&tx, "design", project.as_deref(), Some(pid), 1, &data)?;
            let snapshot = json!({"design_id":design["id"],"design_revision":1,"snapshot":data});
            insert(
                &tx,
                "design_version",
                project.as_deref(),
                design["id"].as_str(),
                1,
                &snapshot,
            )?;
            design
        }
        "run" => {
            if headers
                .get("x-asterlab-human-confirmed")
                .and_then(|value| value.to_str().ok())
                != Some("1")
            {
                return Err(ApiError {
                    status: StatusCode::FORBIDDEN,
                    message: "创建实验运行需要人类确认；请从 AsterLab 运行创建入口提交".into(),
                });
            }
            let did = parent
                .as_deref()
                .ok_or_else(|| ApiError::bad("实验运行需要关联实验设计"))?;
            let title = required(&data, "title", "运行名称", 160)?;
            let scheduled_date = data
                .get("scheduled_date")
                .and_then(Value::as_str)
                .filter(|date| !date.trim().is_empty());
            let global_protocol = data
                .get("protocol_version_id")
                .and_then(Value::as_str)
                .filter(|id| !id.is_empty());
            let repeat_of_run_id = data
                .get("repeat_of_run_id")
                .and_then(Value::as_str)
                .filter(|id| !id.is_empty());
            let view_id = data
                .get("view_id")
                .and_then(Value::as_str)
                .filter(|id| !id.is_empty());
            let repeat_relationship = data
                .get("repeat_relationship")
                .and_then(Value::as_str)
                .unwrap_or("unknown");
            cli::create_run_in_transaction(
                &tx,
                did,
                &title,
                scheduled_date,
                global_protocol,
                view_id,
                repeat_of_run_id,
                repeat_relationship,
            )
            .map_err(ApiError::from_cli)?
        }
        "task" => {
            if let Some(pid) = parent.as_deref() {
                if record(&tx, pid)?.map(|x| x.0) != Some("run".into()) {
                    return Err(ApiError::bad("关联的实验运行不存在"));
                }
            }
            required(&data, "title", "任务名称", 160)?;
            required(&data, "scheduled_date", "计划日期", 32)?;
            data["status"] = json!(data
                .get("status")
                .and_then(Value::as_str)
                .unwrap_or("planned"));
            insert(&tx, "task", project.as_deref(), parent.as_deref(), 1, &data)?
        }
        "protocol" => {
            if let Some(pid) = parent.as_deref() {
                if record(&tx, pid)?.map(|x| x.0) != Some("project".into()) {
                    return Err(ApiError::bad("归属项目不存在"));
                }
                if project.as_deref().is_some_and(|incoming| incoming != pid) {
                    return Err(ApiError::bad("Protocol parent_id 与 project_id 必须一致"));
                }
                project = Some(pid.to_owned());
            }
            required(&data, "title", "协议名称", 160)?;
            let body = text(&data, "body").to_owned();
            let root_id = data.get("root_block_version_id").and_then(Value::as_str);
            let bindings = data
                .get("root_bindings")
                .cloned()
                .unwrap_or_else(empty_array);
            if let Some(block_version_id) = root_id {
                let (_, block_project, _, _, _) = record(&tx, block_version_id)?
                    .ok_or_else(|| ApiError::bad("根步骤块版本不存在"))?;
                if block_project.as_deref() != project.as_deref() {
                    return Err(ApiError::bad("Protocol 与根步骤块必须属于同一项目"));
                }
                cli::protocol_block_manifest(&tx, Some(block_version_id), &bindings)
                    .map_err(ApiError::bad)?;
            } else if !bindings.as_array().is_some_and(Vec::is_empty) {
                return Err(ApiError::bad(
                    "未指定 root_block_version_id 时不能提供 root_bindings",
                ));
            }
            let protocol = insert(
                &tx,
                "protocol",
                project.as_deref(),
                parent.as_deref(),
                1,
                &json!({"title":text(&data,"title")}),
            )?;
            let version = insert(
                &tx,
                "protocol_version",
                project.as_deref(),
                protocol["id"].as_str(),
                1,
                &json!({"protocol_id":protocol["id"],"version":1,"body":body,"root_block_version_id":root_id,"root_bindings":bindings}),
            )?;
            json!({"protocol":protocol,"version":version})
        }
        "protocol_version" => {
            let pid = parent
                .as_deref()
                .ok_or_else(|| ApiError::bad("协议版本需要关联协议"))?;
            let (_, protocol_project, _, _, _) = record(&tx, pid)?
                .filter(|r| r.0 == "protocol")
                .ok_or_else(|| ApiError::bad("协议不存在"))?;
            if project
                .as_deref()
                .is_some_and(|incoming| Some(incoming) != protocol_project.as_deref())
            {
                return Err(ApiError::bad("协议版本必须属于 Protocol 所属项目"));
            }
            project = protocol_project;
            let latest:i64=tx.query_row("SELECT COALESCE(MAX(revision),0) FROM records WHERE kind='protocol_version' AND parent_id=?1",[pid],|r|r.get(0))?;
            let previous_raw: String = tx.query_row("SELECT data_json FROM records WHERE kind='protocol_version' AND parent_id=?1 AND revision=?2", params![pid, latest], |r| r.get(0))?;
            let previous: Value = serde_json::from_str(&previous_raw)?;
            let root_id = match data.get("root_block_version_id") {
                None => previous
                    .get("root_block_version_id")
                    .and_then(Value::as_str)
                    .map(str::to_owned),
                Some(Value::Null) => None,
                Some(Value::String(value)) => Some(value.clone()),
                Some(_) => return Err(ApiError::bad("root_block_version_id 必须是字符串或 null")),
            };
            let bindings = if data
                .get("root_block_version_id")
                .is_some_and(Value::is_null)
            {
                data.get("root_bindings")
                    .cloned()
                    .unwrap_or_else(empty_array)
            } else {
                data.get("root_bindings")
                    .cloned()
                    .or_else(|| previous.get("root_bindings").cloned())
                    .unwrap_or_else(empty_array)
            };
            if let Some(block_version_id) = root_id.as_deref() {
                let (_, block_project, _, _, _) = record(&tx, block_version_id)?
                    .ok_or_else(|| ApiError::bad("根步骤块版本不存在"))?;
                if block_project.as_deref() != project.as_deref() {
                    return Err(ApiError::bad("Protocol 与根步骤块必须属于同一项目"));
                }
                cli::protocol_block_manifest(&tx, Some(block_version_id), &bindings)
                    .map_err(ApiError::bad)?;
            } else if !bindings.as_array().is_some_and(Vec::is_empty) {
                return Err(ApiError::bad(
                    "清除 root_block_version_id 时必须清空 root_bindings",
                ));
            }
            data = json!({"protocol_id":pid,"version":latest+1,"body":data.get("body").and_then(Value::as_str).unwrap_or_else(|| previous["body"].as_str().unwrap_or_default()),"root_block_version_id":root_id,"root_bindings":bindings});
            insert(
                &tx,
                "protocol_version",
                project.as_deref(),
                Some(pid),
                latest + 1,
                &data,
            )?
        }
        "plate" => {
            let rid = parent
                .as_deref()
                .ok_or_else(|| ApiError::bad("板布局需要关联实验运行"))?;
            if record(&tx, rid)?.map(|x| x.0) != Some("run".into()) {
                return Err(ApiError::bad("实验运行不存在"));
            }
            let assay_id = match data.get("assay_id") {
                Some(Value::String(assay_id)) if !assay_id.trim().is_empty() => Some(assay_id),
                Some(Value::Null) | None => None,
                Some(_) => return Err(ApiError::bad("assay_id 必须是字符串或 null")),
            };
            let timepoint = match data.get("timepoint") {
                Some(Value::String(value)) if !value.trim().is_empty() => Some(value),
                Some(Value::Null) | None => None,
                Some(_) => return Err(ApiError::bad("timepoint 必须是字符串或 null")),
            };
            if let Some(assay_id) = assay_id {
                cli::validate_run_assay_reference(
                    &tx,
                    rid,
                    assay_id,
                    timepoint.map(String::as_str),
                )
                .map_err(ApiError::from_cli)?;
            } else if timepoint.is_some() {
                return Err(ApiError::bad("填写板图 timepoint 时必须同时关联 assay_id"));
            }
            let rows = data["rows"].as_i64().unwrap_or(0);
            let cols = data["columns"].as_i64().unwrap_or(0);
            if !(1..=32).contains(&rows) || !(1..=48).contains(&cols) {
                return Err(ApiError::bad("板型行列范围无效"));
            }
            required(&data, "name", "板名", 100)?;
            insert(&tx, "plate", project.as_deref(), Some(rid), 1, &data)?
        }
        "protocol_block" => {
            let pid = project
                .as_deref()
                .or(parent.as_deref())
                .ok_or_else(|| ApiError::bad("步骤块需要归属项目"))?
                .to_owned();
            if parent.as_deref().is_some_and(|parent_id| parent_id != pid) {
                return Err(ApiError::bad("步骤块 parent_id 与 project_id 必须一致"));
            }
            if record(&tx, &pid)?.map(|x| x.0) != Some("project".into()) {
                return Err(ApiError::bad("归属项目不存在"));
            }
            cli::create_protocol_block_records(&tx, &pid, &data).map_err(ApiError::from_cli)?
        }
        "protocol_block_version" => {
            let block_id = parent
                .as_deref()
                .ok_or_else(|| ApiError::bad("步骤块版本需要关联步骤块"))?;
            let (_, block_project, _, _, _) = record(&tx, block_id)?
                .filter(|r| r.0 == "protocol_block")
                .ok_or_else(|| ApiError::bad("步骤块不存在"))?;
            if project
                .as_deref()
                .is_some_and(|incoming| Some(incoming) != block_project.as_deref())
            {
                return Err(ApiError::bad("步骤块版本必须属于步骤块所属项目"));
            }
            cli::revise_protocol_block_records(&tx, block_id, &data).map_err(ApiError::from_cli)?
                ["version"]
                .clone()
        }
        "well" => {
            let plate_id = parent
                .as_deref()
                .ok_or_else(|| ApiError::bad("孔位需要关联板布局"))?;
            let (_, _, _, _, plate_raw) = record(&tx, plate_id)?
                .filter(|x| x.0 == "plate")
                .ok_or_else(|| ApiError::bad("板布局不存在"))?;
            let plate: Value = serde_json::from_str(&plate_raw)?;
            let well = required(&data, "well_id", "孔位", 12)?;
            let letters = well
                .chars()
                .take_while(|c| c.is_ascii_alphabetic())
                .collect::<String>();
            let number = well
                .chars()
                .skip_while(|c| c.is_ascii_alphabetic())
                .collect::<String>()
                .parse::<i64>()
                .unwrap_or(0);
            let row = letters.chars().fold(0i64, |n, c| {
                n * 26 + (c.to_ascii_uppercase() as i64 - 'A' as i64 + 1)
            });
            if row < 1
                || row > plate["rows"].as_i64().unwrap_or(0)
                || number < 1
                || number > plate["columns"].as_i64().unwrap_or(0)
            {
                return Err(ApiError::bad("孔位超出板型范围"));
            }
            let existing:Option<String>=tx.query_row("SELECT id FROM records WHERE kind='well' AND parent_id=?1 AND json_extract(data_json,'$.well_id')=?2",[plate_id,&well],|r|r.get(0)).optional()?;
            if let Some(existing) = existing {
                tx.execute(
                    "UPDATE records SET data_json=?2,updated_at=?3 WHERE id=?1",
                    params![existing, serde_json::to_string(&data)?, now()],
                )?;
                json!({"id":existing,"kind":"well","project_id":project,"parent_id":plate_id,"data":data})
            } else {
                insert(&tx, "well", project.as_deref(), Some(plate_id), 1, &data)?
            }
        }
        "note" => {
            let rid = parent
                .as_deref()
                .ok_or_else(|| ApiError::bad("实验记录需要关联实验运行"))?;
            if record(&tx, rid)?.map(|x| x.0) != Some("run".into()) {
                return Err(ApiError::bad("实验运行不存在"));
            }
            required(&data, "content", "记录内容", 10000)?;
            data["event_time"] = if text(&data, "event_time").is_empty() {
                Value::Null
            } else {
                json!(text(&data, "event_time"))
            };
            insert(&tx, "note", project.as_deref(), Some(rid), 1, &data)?
        }
        "execution_event" => {
            let run_id = parent
                .as_deref()
                .ok_or_else(|| ApiError::bad("执行事件需要关联实验运行"))?;
            cli::create_execution_event_tx(&tx, run_id, &data).map_err(ApiError::from_cli)?
        }
        "formula" => {
            let pid = parent
                .as_deref()
                .ok_or_else(|| ApiError::bad("公式需要归属项目"))?;
            if record(&tx, pid)?.map(|x| x.0) != Some("project".into()) {
                return Err(ApiError::bad("归属项目不存在"));
            }
            required(&data, "title", "公式名称", 160)?;
            required(&data, "expression", "公式表达式", 2000)?;
            project = Some(pid.to_owned());
            insert(&tx, "formula", project.as_deref(), Some(pid), 1, &data)?
        }
        "library_table" => {
            if let Some(pid) = parent.as_deref() {
                if record(&tx, pid)?.map(|x| x.0) != Some("project".into()) {
                    return Err(ApiError::bad("归属项目不存在"));
                }
                project = Some(pid.to_owned());
            }
            required(&data, "name", "资料表名称", 120)?;
            if data["columns"].as_array().map_or(0, Vec::len) > 40
                || data["rows"].as_array().map_or(0, Vec::len) > 5000
            {
                return Err(ApiError::bad("资料表行列超出上限"));
            }
            insert(
                &tx,
                "library_table",
                project.as_deref(),
                parent.as_deref(),
                1,
                &data,
            )?
        }
        _ => return Err(ApiError::bad("未知的数据类型")),
    };
    tx.commit()?;
    Ok(Json(result))
}

#[derive(Deserialize)]
struct UpdateInput {
    id: String,
    #[serde(default)]
    expected_revision: Option<i64>,
    data: Value,
}
async fn update_record(
    State(s): State<AppState>,
    Json(input): Json<UpdateInput>,
) -> ApiResult<Value> {
    let mut db = lock(&s)?;
    let (kind, project, parent, revision, old) =
        record(&db, &input.id)?.ok_or_else(|| ApiError::missing("找不到要更新的记录"))?;
    if matches!(kind.as_str(), "design" | "run") && input.expected_revision.is_none() {
        return Err(ApiError::bad(
            "修订实验设计或运行必须提供 expected_revision；请先读取当前版本",
        ));
    }
    if let Some(expected) = input.expected_revision {
        if expected != revision {
            return Err(ApiError {
                status: StatusCode::CONFLICT,
                message: format!(
                    "记录版本已从 v{expected} 更新为 v{revision}；请先重新读取后再保存"
                ),
            });
        }
    }
    if kind == "design" {
        let previous: Value = serde_json::from_str(&old)?;
        let data = cli::normalize_validate_design_data(input.data, Some(&previous))
            .map_err(ApiError::from_cli)?;
        let next = revision + 1;
        let tx = db.transaction()?;
        let changed = tx.execute(
            "UPDATE records SET revision=?2,data_json=?3,updated_at=?4 WHERE id=?1 AND revision=?5",
            params![
                input.id,
                next,
                serde_json::to_string(&data)?,
                now(),
                revision
            ],
        )?;
        if changed != 1 {
            return Err(ApiError {
                status: StatusCode::CONFLICT,
                message: "记录已被其他操作更新；请刷新后重试".into(),
            });
        }
        let snapshot = json!({"design_id":input.id,"design_revision":next,"snapshot":data});
        let version = insert(
            &tx,
            "design_version",
            project.as_deref(),
            Some(&input.id),
            next,
            &snapshot,
        )?;
        tx.commit()?;
        return Ok(Json(
            json!({"id":input.id,"kind":"design","project_id":project,"parent_id":parent,"revision":next,"data":data,"new_version":version}),
        ));
    }
    if kind == "run" {
        let expected = input.expected_revision.ok_or_else(|| {
            ApiError::bad("修订实验运行必须提供 expected_revision；请先读取当前版本")
        })?;
        let updated =
            cli::patch_run(&db, &input.id, expected, &input.data).map_err(ApiError::from_cli)?;
        return Ok(Json(updated));
    }
    if kind == "protocol" {
        return Err(ApiError::bad(
            "协议正文必须通过新建 protocol_version 保存，避免覆盖历史版本",
        ));
    }
    if kind == "protocol_version" {
        return Err(ApiError::bad("协议版本不可覆盖，请创建新版本"));
    }
    if kind == "execution_event" {
        return Err(ApiError::bad(
            "执行事件为追加记录，不可覆盖；请追加更正事件并说明依据",
        ));
    }
    let next = revision + 1;
    let changed = db.execute(
        "UPDATE records SET revision=?2,data_json=?3,updated_at=?4 WHERE id=?1 AND revision=?5",
        params![
            input.id,
            next,
            serde_json::to_string(&input.data)?,
            now(),
            revision
        ],
    )?;
    if changed != 1 {
        return Err(ApiError {
            status: StatusCode::CONFLICT,
            message: "记录已被其他操作更新；请刷新后重试".into(),
        });
    }
    Ok(Json(
        json!({"id":input.id,"kind":kind,"project_id":project,"parent_id":parent,"revision":next,"data":input.data,"previous_data":serde_json::from_str::<Value>(&old)?}),
    ))
}
#[derive(Debug, Deserialize)]
struct RestoreInput {
    path: String,
}
fn require_human_confirmation(headers: &HeaderMap) -> Result<(), ApiError> {
    if headers
        .get("x-asterlab-human-confirmed")
        .and_then(|value| value.to_str().ok())
        == Some("1")
    {
        Ok(())
    } else {
        Err(ApiError {
            status: StatusCode::FORBIDDEN,
            message: "此操作需要从 AsterLab 本地界面确认".into(),
        })
    }
}
async fn create_backup(
    State(s): State<AppState>,
    headers: HeaderMap,
) -> ApiResult<Value> {
    require_human_confirmation(&headers)?;
    let db = lock(&s)?;
    let dir = s.data_dir.join("backups");
    fs::create_dir_all(&dir).map_err(|e| ApiError::internal(format!("无法创建备份目录：{e}")))?;
    let stamp = Utc::now().format("%Y%m%d-%H%M%S").to_string();
    let path = dir.join(format!("labworkbench-{stamp}.sqlite"));
    db.execute("VACUUM INTO ?1", [path.to_string_lossy().as_ref()])
        .map_err(|e| ApiError::internal(format!("备份失败：{e}")))?;
    Ok(Json(json!({"path":path,"created_at":now()})))
}
async fn restore_backup(
    State(s): State<AppState>,
    headers: HeaderMap,
    Json(input): Json<RestoreInput>,
) -> ApiResult<Value> {
    require_human_confirmation(&headers)?;
    let source = fs::canonicalize(&input.path)
        .map_err(|e| ApiError::bad(format!("备份文件不可读：{e}")))?;
    let check = Connection::open(&source).map_err(|e| ApiError::bad(format!("备份无法打开：{e}")))?;
    let integrity: String = check
        .query_row("PRAGMA integrity_check", [], |row| row.get(0))
        .map_err(|e| ApiError::bad(format!("备份完整性检查失败：{e}")))?;
    if integrity != "ok" {
        return Err(ApiError::bad(format!("备份完整性检查未通过：{integrity}")));
    }
    drop(check);
    let mut guard = lock(&s)?;
    let placeholder = Connection::open_in_memory().map_err(|e| ApiError::internal(e.to_string()))?;
    let old = std::mem::replace(&mut *guard, placeholder);
    drop(old);
    let temp = s.db_path.with_extension("sqlite.restore");
    let restore = (|| -> Result<(), std::io::Error> {
        fs::copy(&source, &temp)?;
        for suffix in ["-wal", "-shm"] {
            let side = s.db_path.with_extension(format!("sqlite{suffix}"));
            let _ = fs::remove_file(side);
        }
        fs::rename(&temp, &s.db_path)?;
        Ok(())
    })();
    if let Err(error) = restore {
        let _ = fs::remove_file(&temp);
        *guard = Connection::open(&s.db_path).map_err(|e| ApiError::internal(format!("恢复失败且无法重新打开数据库：{e}")))?;
        return Err(ApiError::internal(format!("恢复失败：{error}")));
    }
    let restored = Connection::open(&s.db_path).map_err(|e| ApiError::internal(format!("恢复后无法打开数据库：{e}")))?;
    restored.pragma_update(None, "foreign_keys", "ON")?;
    *guard = restored;
    Ok(Json(json!({"restored_from":source,"restored_at":now()})))
}
#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    let folder = env::var_os("LABWORKBENCH_DATA_DIR")
        .map(PathBuf::from)
        .unwrap_or_else(|| PathBuf::from("data"));
    let args: Vec<String> = env::args().skip(1).collect();
    if !args.is_empty() {
        std::process::exit(cli::run(&args, &folder));
    }
    fs::create_dir_all(&folder)?;
    let db_path = folder.join("labworkbench.sqlite");
    let db = Connection::open(&db_path)?;
    db.pragma_update(None, "foreign_keys", "ON")?;
    db.execute_batch("CREATE TABLE IF NOT EXISTS records(id TEXT PRIMARY KEY,kind TEXT NOT NULL,project_id TEXT REFERENCES records(id),parent_id TEXT REFERENCES records(id) ON DELETE RESTRICT,revision INTEGER NOT NULL DEFAULT 1,data_json TEXT NOT NULL,created_at TEXT NOT NULL,updated_at TEXT NOT NULL); CREATE INDEX IF NOT EXISTS records_kind_parent ON records(kind,parent_id); CREATE INDEX IF NOT EXISTS records_project ON records(project_id);")?;
    let state = AppState {
        db: Arc::new(Mutex::new(db)),
        data_dir: folder.clone(),
        db_path: db_path.clone(),
    };
    let port = env::var("LABWORKBENCH_PORT")
        .ok()
        .and_then(|x| x.parse::<u16>().ok())
        .unwrap_or(8832);
    let addr = SocketAddr::from(([127, 0, 0, 1], port));
    let app = Router::new()
        .route("/", get(view_page))
        .route("/workspace", get(index))
        .route("/view", get(view_page))
        .route("/design-preview", get(design_preview_page))
        .route("/icons/{name}", get(icon))
        .route("/app.js", get(app_js))
        .route("/pages.js", get(pages_js))
        .route("/resources.js", get(resources_js))
        .route("/vendor/marked.js", get(marked_js))
        .route("/vendor/purify.js", get(purify_js))
        .route("/vendor/dagre.js", get(dagre_js))
        .route("/layout.js", get(layout_js))
        .route("/components/{name}", get(component_asset))
        .route("/pages/{name}", get(page_asset))
        .route("/icons/inline.js", get(icon_inline_js))
        .route("/vendor/tabler/tabler.min.css", get(tabler_css))
        .route("/vendor/tabler/tabler.min.js", get(tabler_js))
        .route("/api/views/{id}", get(get_view))
        .route("/api/views/{id}/data", get(get_view_data))
        .route(
            "/api/views/{id}/steps/{instance_id}",
            get(get_view_step_detail),
        )
        .route(
            "/api/views/{id}/selection/{instance_id}",
            get(get_view_selection),
        )
        .route("/api/views/{id}/propose", post(propose_view))
        .route(
            "/api/views/{id}/proposals",
            get(list_view_proposals).post(create_view_proposal),
        )
        .route(
            "/api/views/{id}/proposals/{proposal_id}/accept",
            post(accept_view_proposal),
        )
        .route(
            "/api/views/{id}/proposals/{proposal_id}/reject",
            post(reject_view_proposal),
        )
        .route("/api/views/{id}/revise", post(revise_view))
        .route("/api/health", get(health))
        .route("/api/state", get(get_state))
        .route("/api/designs/{id}/map", get(get_design_map))
        .route("/api/designs/{id}/workflow", post(ensure_design_workflow))
        .route("/api/runs/{id}/workspace", get(get_run_workspace))
        .route("/api/protocol-blocks/preview", post(preview_block))
        .route("/api/records", post(create_record))
        .route("/api/backup", post(create_backup))
        .route("/api/restore", post(restore_backup))
        .route("/api/records/update", post(update_record))
        .with_state(state);
    let listener = tokio::net::TcpListener::bind(addr).await?;
    println!("AsterLab：http://{addr}");
    println!("本地 SQLite：{}", db_path.display());
    println!("仅监听本机回环地址。Ctrl+C 可停止。");
    axum::serve(listener, app).await?;
    Ok(())
}
