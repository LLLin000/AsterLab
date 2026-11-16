use super::*;

/// Read-only record view shared by the validation and commit adapters.
pub(crate) struct CliEditStore<'a> {
    pub(crate) conn: &'a Connection,
}

pub(crate) fn edit_protocol_head(
    conn: &Connection,
    protocol_id: &str,
) -> Result<Option<StoredProtocolVersion>, String> {
    let row = conn
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

pub(crate) fn edit_block_version(
    conn: &Connection,
    version_id: &str,
) -> Result<Option<StoredBlockVersion>, String> {
    let row = conn
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
    let data: ProtocolBlockVersionData = serde_json::from_str(&raw).map_err(|error| {
        format!("protocol_block_version {version_id} data_json 无效：{error}")
    })?;
    Ok(Some(StoredBlockVersion {
        version_id: version_id.to_owned(),
        revision,
        data,
    }))
}

impl EditTargetStore for CliEditStore<'_> {
    fn protocol_head(
        &self,
        protocol_id: &str,
    ) -> Result<Option<StoredProtocolVersion>, String> {
        edit_protocol_head(self.conn, protocol_id)
    }

    fn block_version(&self, version_id: &str) -> Result<Option<StoredBlockVersion>, String> {
        edit_block_version(self.conn, version_id)
    }
}

/// Transactional commit view: every read and the append go through one
/// `BEGIN IMMEDIATE` transaction so a concurrent proposal cannot slip between
/// the head re-read and the insert.
pub(crate) struct CliCommitStore<'a, 'conn> {
    pub(crate) tx: &'a Transaction<'conn>,
}

impl EditTargetStore for CliCommitStore<'_, '_> {
    fn protocol_head(
        &self,
        protocol_id: &str,
    ) -> Result<Option<StoredProtocolVersion>, String> {
        edit_protocol_head(self.tx, protocol_id)
    }

    fn block_version(&self, version_id: &str) -> Result<Option<StoredBlockVersion>, String> {
        edit_block_version(self.tx, version_id)
    }
}

impl CommitStore for CliCommitStore<'_, '_> {
    fn edit_store(&self) -> &dyn EditTargetStore {
        self
    }

    fn protocol_version_data(&self, version_id: &str) -> Result<Option<Value>, String> {
        let row = self
            .tx
            .query_row(
                "SELECT data_json FROM records WHERE kind='protocol_version' AND id=?1",
                [version_id],
                |row| row.get::<_, String>(0),
            )
            .optional()
            .map_err(|error| error.to_string())?;
        row.map(|raw| serde_json::from_str(&raw).map_err(|error| error.to_string()))
            .transpose()
    }

    fn find_commit(&self, idempotency_key: &str) -> Result<Option<CommittedVersion>, String> {
        let row = self
            .tx
            .query_row(
                "SELECT id,revision,data_json FROM records \
                 WHERE kind IN ('protocol_version','protocol_block_version') \
                   AND json_extract(data_json,'$.agent_edit.idempotency_key')=?1 \
                 ORDER BY created_at DESC, rowid DESC LIMIT 1",
                [idempotency_key],
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
        let data: Value = serde_json::from_str(&raw).map_err(|error| error.to_string())?;
        Ok(Some(CommittedVersion {
            version_id,
            revision,
            provenance: data["agent_edit"].clone(),
        }))
    }

    fn timestamp(&self) -> String {
        stamp()
    }

    fn append_protocol_version(
        &mut self,
        protocol_id: &str,
        revision: i64,
        data: Value,
    ) -> Result<(String, i64), String> {
        let (kind, project, _, _, _) = kind_id(self.tx, protocol_id)
            .map_err(|error| error.message)?
            .ok_or_else(|| format!("Protocol 不存在：{protocol_id}"))?;
        if kind != "protocol" {
            return Err(format!("{protocol_id} 不是 protocol 记录"));
        }
        let record = insert(
            self.tx,
            "protocol_version",
            project.as_deref(),
            Some(protocol_id),
            revision,
            &data,
        )
        .map_err(|error| error.message)?;
        Ok((
            record["id"].as_str().unwrap_or_default().to_owned(),
            record["revision"].as_i64().unwrap_or(revision),
        ))
    }

    fn append_block_version(
        &mut self,
        block_id: &str,
        revision: i64,
        data: Value,
    ) -> Result<(String, i64), String> {
        let (kind, project, _, _, _) = kind_id(self.tx, block_id)
            .map_err(|error| error.message)?
            .ok_or_else(|| format!("步骤块不存在：{block_id}"))?;
        if kind != "protocol_block" {
            return Err(format!("{block_id} 不是 protocol_block 记录"));
        }
        let record = insert(
            self.tx,
            "protocol_block_version",
            project.as_deref(),
            Some(block_id),
            revision,
            &data,
        )
        .map_err(|error| error.message)?;
        Ok((
            record["id"].as_str().unwrap_or_default().to_owned(),
            record["revision"].as_i64().unwrap_or(revision),
        ))
    }
}

/// Read-only `agent_edit.validate`: resolve one proposal against the current
/// head. Semantic rejections stay in the result (`status = blocked`).
pub(crate) fn validate_agent_edit(db: &Connection, input: Value) -> Result<Value, CliError> {
    let proposal: Proposal = serde_json::from_value(input).map_err(|error| {
        CliError::input(format!(
            "proposal 不是合法的 agent edit v1 载荷：{error}"
        ))
    })?;
    let result = validate_agent_edit_request(&proposal, &CliEditStore { conn: db })
        .map_err(CliError::internal)?;
    serde_json::to_value(result).map_err(|error| CliError::internal(error.to_string()))
}

/// `agent_edit.commit`: append one confirmed version in a single write
/// transaction. A stale or concurrent proposal returns `VERSION_CONFLICT` and
/// writes nothing.
pub(crate) fn commit_agent_edit(db: &mut Connection, input: Value) -> Result<Value, CliError> {
    let request: ConfirmRequest = serde_json::from_value(input).map_err(|error| {
        CliError::input(format!(
            "commit 载荷不符合 agent edit v1 ConfirmRequest：{error}"
        ))
    })?;
    let tx = db
        .transaction_with_behavior(rusqlite::TransactionBehavior::Immediate)
        .map_err(|error| CliError::internal(error.to_string()))?;
    let outcome = {
        let mut store = CliCommitStore { tx: &tx };
        commit_agent_edit_request(&request, &mut store)
    };
    match outcome {
        Ok(result) => {
            tx.commit()
                .map_err(|error| CliError::internal(error.to_string()))?;
            serde_json::to_value(result).map_err(|error| CliError::internal(error.to_string()))
        }
        Err(CommitError::Rejected(error)) => {
            tx.rollback().ok();
            Err(agent_edit_cli_error(error))
        }
        Err(CommitError::Store(message)) => {
            tx.rollback().ok();
            Err(CliError::internal(message))
        }
    }
}

pub(crate) fn agent_edit_cli_error(error: AgentEditError) -> CliError {
    let code = match error.code {
        AgentEditErrorCode::InvalidProposal => "INVALID_PROPOSAL",
        AgentEditErrorCode::InvalidBinding => "INVALID_BINDING",
        AgentEditErrorCode::UnitMismatch => "UNIT_MISMATCH",
        AgentEditErrorCode::StaleReference => "STALE_REFERENCE",
        AgentEditErrorCode::ForbiddenField => "FORBIDDEN_FIELD",
        AgentEditErrorCode::VersionConflict => "VERSION_CONFLICT",
        AgentEditErrorCode::ConfirmationRequired => "CONFIRMATION_REQUIRED",
        AgentEditErrorCode::ConfirmationMismatch => "CONFIRMATION_MISMATCH",
        AgentEditErrorCode::IdempotencyMismatch => "IDEMPOTENCY_MISMATCH",
    };
    let exit = match error.code {
        AgentEditErrorCode::VersionConflict => 4,
        AgentEditErrorCode::StaleReference => 3,
        _ => 2,
    };
    let issues = error
        .issues
        .into_iter()
        .filter_map(|issue| serde_json::to_value(issue).ok())
        .collect();
    CliError {
        code,
        message: error.message,
        exit,
        path: error.path,
        current_version: error.current_version,
        current_version_id: error.current_version_id,
        issues,
    }
}

/// Read-only view for `step.edit` validation.
pub(crate) struct CliStepEditStore<'a> {
    pub(crate) conn: &'a Connection,
}

pub(crate) fn step_edit_block_version(
    conn: &Connection,
    version_id: &str,
) -> Result<Option<StoredEditBlock>, String> {
    let row = conn
        .query_row(
            "SELECT revision,parent_id,project_id,data_json FROM records \
             WHERE kind='protocol_block_version' AND id=?1",
            [version_id],
            |row| {
                Ok((
                    row.get::<_, i64>(0)?,
                    row.get::<_, Option<String>>(1)?,
                    row.get::<_, Option<String>>(2)?,
                    row.get::<_, String>(3)?,
                ))
            },
        )
        .optional()
        .map_err(|error| error.to_string())?;
    let Some((revision, parent, project, raw)) = row else {
        return Ok(None);
    };
    let data: ProtocolBlockVersionData = serde_json::from_str(&raw).map_err(|error| {
        format!("protocol_block_version {version_id} data_json 无效：{error}")
    })?;
    Ok(Some(StoredEditBlock {
        version_id: version_id.to_owned(),
        block_id: parent.unwrap_or_else(|| data.block_id.clone()),
        revision,
        project_id: project,
        data,
    }))
}

pub(crate) fn step_edit_block_head(
    conn: &Connection,
    block_id: &str,
) -> Result<Option<StoredBlockHead>, String> {
    conn.query_row(
        "SELECT id,revision FROM records \
         WHERE kind='protocol_block_version' AND parent_id=?1 \
         ORDER BY revision DESC LIMIT 1",
        [block_id],
        |row| {
            Ok(StoredBlockHead {
                block_id: block_id.to_owned(),
                version_id: row.get(0)?,
                revision: row.get(1)?,
            })
        },
    )
    .optional()
    .map_err(|error| error.to_string())
}

impl StepEditStore for CliStepEditStore<'_> {
    fn step_block_version(&self, version_id: &str) -> Result<Option<StoredEditBlock>, String> {
        step_edit_block_version(self.conn, version_id)
    }

    fn step_block_head(&self, block_id: &str) -> Result<Option<StoredBlockHead>, String> {
        step_edit_block_head(self.conn, block_id)
    }
}

/// Transactional view for `step.edit` commit: the head re-read, the
/// idempotency lookup and every append share one `BEGIN IMMEDIATE` transaction.
pub(crate) struct CliStepEditCommitStore<'a, 'conn> {
    pub(crate) tx: &'a Transaction<'conn>,
}

impl StepEditStore for CliStepEditCommitStore<'_, '_> {
    fn step_block_version(&self, version_id: &str) -> Result<Option<StoredEditBlock>, String> {
        step_edit_block_version(self.tx, version_id)
    }

    fn step_block_head(&self, block_id: &str) -> Result<Option<StoredBlockHead>, String> {
        step_edit_block_head(self.tx, block_id)
    }
}

impl StepEditCommitStore for CliStepEditCommitStore<'_, '_> {
    fn step_store(&self) -> &dyn StepEditStore {
        self
    }

    fn find_step_commit(
        &self,
        idempotency_key: &str,
    ) -> Result<Vec<CommittedStepEdit>, String> {
        let mut statement = self
            .tx
            .prepare(
                "SELECT id,revision,data_json FROM records \
                 WHERE kind='protocol_block_version' \
                   AND json_extract(data_json,'$.step_edit.idempotency_key')=?1 \
                 ORDER BY revision ASC, rowid ASC",
            )
            .map_err(|error| error.to_string())?;
        let rows = statement
            .query_map([idempotency_key], |row| {
                Ok((
                    row.get::<_, String>(0)?,
                    row.get::<_, i64>(1)?,
                    row.get::<_, String>(2)?,
                ))
            })
            .map_err(|error| error.to_string())?;
        let mut out = Vec::new();
        for row in rows {
            let (version_id, revision, raw) = row.map_err(|error| error.to_string())?;
            let data: Value = serde_json::from_str(&raw).map_err(|error| error.to_string())?;
            out.push(CommittedStepEdit {
                version_id,
                revision,
                provenance: data["step_edit"].clone(),
            });
        }
        Ok(out)
    }

    fn timestamp(&self) -> String {
        stamp()
    }

    fn append_block_version(
        &mut self,
        block_id: &str,
        revision: i64,
        data: Value,
    ) -> Result<(String, i64), String> {
        let (kind, project, _, _, _) = kind_id(self.tx, block_id)
            .map_err(|error| error.message)?
            .ok_or_else(|| format!("步骤块不存在：{block_id}"))?;
        if kind != "protocol_block" {
            return Err(format!("{block_id} 不是 protocol_block 记录"));
        }
        let record = insert(
            self.tx,
            "protocol_block_version",
            project.as_deref(),
            Some(block_id),
            revision,
            &data,
        )
        .map_err(|error| error.message)?;
        Ok((
            record["id"].as_str().unwrap_or_default().to_owned(),
            record["revision"].as_i64().unwrap_or(revision),
        ))
    }
}

/// `step.edit`: validate (no `confirmation`) or commit (with `confirmation`)
/// block-level structural edits in one command.
pub(crate) fn step_edit_command(db: &mut Connection, input: Value) -> Result<Value, CliError> {
    let request: StepEditRequest = serde_json::from_value(input)
        .map_err(|error| CliError::input(format!("step.edit 载荷无效：{error}")))?;
    if request.confirmation.is_none() {
        let result = step_edit_validate_request(&request, &CliStepEditStore { conn: db })
            .map_err(CliError::internal)?;
        return serde_json::to_value(result).map_err(|error| CliError::internal(error.to_string()));
    }
    let tx = db
        .transaction_with_behavior(TransactionBehavior::Immediate)
        .map_err(|error| CliError::internal(error.to_string()))?;
    let outcome = {
        let mut store = CliStepEditCommitStore { tx: &tx };
        step_edit_commit_request(&request, &mut store)
    };
    match outcome {
        Ok(result) => {
            tx.commit()
                .map_err(|error| CliError::internal(error.to_string()))?;
            serde_json::to_value(result).map_err(|error| CliError::internal(error.to_string()))
        }
        Err(StepEditCommitError::Rejected(error)) => {
            tx.rollback().ok();
            Err(agent_edit_cli_error(error))
        }
        Err(StepEditCommitError::Store(message)) => {
            tx.rollback().ok();
            Err(CliError::internal(message))
        }
    }
}

// ---------------------------------------------------------------------------
// Pantry ledger (储藏室). Optional and non-blocking: Design never requires an
// item, and a Run event only may reference one.
// ---------------------------------------------------------------------------

