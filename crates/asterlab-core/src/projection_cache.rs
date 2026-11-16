//! Pure derived-only read-model cache for expensive projections.
//!
//! `projection_cache` stores serialized [`RunWorkspace`] and
//! [`RecordProjection`] payloads keyed by
//! `(entity_id, source_revision, projection_kind)`. Every entry is derived:
//! readers MUST recompute on a miss, on a revision mismatch, or when the
//! stored payload fails to parse, and writers MUST never persist a
//! non-derived truth. Dropping the table (or skipping the read) always falls
//! back to live computation.

use serde::{Deserialize, Serialize};
use serde_json::Value;
use sha2::{Digest, Sha256};

/// Storage schema version of the `projection_cache` table rows.
pub const PROJECTION_CACHE_SCHEMA_VERSION: u32 = 1;

/// Projection kinds stored in `projection_cache`.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum ProjectionKind {
    /// Serialized [`crate::run_workspace::RunWorkspace`].
    RunWorkspace,
    /// Serialized [`crate::record_projection::RecordProjection`].
    RecordProjection,
}

impl ProjectionKind {
    /// Stable storage string used as the `projection_kind` column value.
    pub fn as_str(self) -> &'static str {
        match self {
            Self::RunWorkspace => "run_workspace",
            Self::RecordProjection => "record_projection",
        }
    }

    /// Parse a stored `projection_kind` column value.
    pub fn parse(value: &str) -> Option<Self> {
        match value.trim() {
            "run_workspace" => Some(Self::RunWorkspace),
            "record_projection" => Some(Self::RecordProjection),
            _ => None,
        }
    }
}

/// Cache identity: which entity, at which source revision, for which kind.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct ProjectionCacheKey {
    pub entity_id: String,
    pub source_revision: i64,
    pub kind: ProjectionKind,
}

impl ProjectionCacheKey {
    pub fn new(entity_id: impl Into<String>, source_revision: i64, kind: ProjectionKind) -> Self {
        Self {
            entity_id: entity_id.into(),
            source_revision,
            kind,
        }
    }
}

/// One cached row: the key plus the serialized derived payload.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct ProjectionCacheEntry {
    pub key: ProjectionCacheKey,
    pub payload: Value,
    /// Content hash of the canonical payload bytes at write time.
    pub payload_hash: String,
    pub updated_at: String,
}

/// Stable content hash over canonical JSON bytes.
pub fn payload_hash(payload: &Value) -> String {
    let mut hasher = Sha256::new();
    hasher.update(serde_json::to_vec(payload).unwrap_or_default());
    hasher
        .finalize()
        .iter()
        .map(|byte| format!("{byte:02x}"))
        .collect()
}

/// Build an entry from a freshly computed projection payload.
pub fn cache_entry(key: ProjectionCacheKey, payload: Value, updated_at: impl Into<String>) -> ProjectionCacheEntry {
    let payload_hash = payload_hash(&payload);
    ProjectionCacheEntry {
        key,
        payload,
        payload_hash,
        updated_at: updated_at.into(),
    }
}

/// Validate a stored row against the requested key.
///
/// Returns the payload when the row matches the key exactly (entity,
/// revision, kind) and the payload hash still matches; otherwise `None` so
/// callers recompute. A revision mismatch is a normal invalidation, not an
/// error; a hash mismatch or unparseable payload means the cached copy is
/// corrupt and must be recomputed.
pub fn validate_cached(
    requested: &ProjectionCacheKey,
    stored_entity_id: &str,
    stored_revision: i64,
    stored_kind: &str,
    stored_hash: &str,
    payload: Value,
) -> Option<Value> {
    if stored_entity_id != requested.entity_id.as_str() {
        return None;
    }
    if stored_revision != requested.source_revision {
        return None;
    }
    if ProjectionKind::parse(stored_kind) != Some(requested.kind) {
        return None;
    }
    if payload_hash(&payload) != stored_hash {
        return None;
    }
    Some(payload)
}

#[cfg(test)]
mod tests {
    use super::*;
    use serde_json::json;

    fn key() -> ProjectionCacheKey {
        ProjectionCacheKey::new("run-1", 3, ProjectionKind::RunWorkspace)
    }

    #[test]
    fn cache_hit_returns_payload_for_exact_key() {
        let payload = json!({"run_id": "run-1", "steps": []});
        let entry = cache_entry(key(), payload.clone(), "2026-11-17T00:00:00Z");
        let hit = validate_cached(
            &key(),
            &entry.key.entity_id,
            entry.key.source_revision,
            entry.key.kind.as_str(),
            &entry.payload_hash,
            entry.payload.clone(),
        );
        assert_eq!(hit, Some(payload));
    }

    #[test]
    fn revision_change_invalidates_without_error() {
        let payload = json!({"run_id": "run-1"});
        let entry = cache_entry(key(), payload, "2026-11-17T00:00:00Z");
        let stale = ProjectionCacheKey::new("run-1", 4, ProjectionKind::RunWorkspace);
        assert_eq!(
            validate_cached(
                &stale,
                &entry.key.entity_id,
                entry.key.source_revision,
                entry.key.kind.as_str(),
                &entry.payload_hash,
                entry.payload,
            ),
            None
        );
    }

    #[test]
    fn corrupt_payload_hash_forces_recompute() {
        let payload = json!({"run_id": "run-1"});
        let entry = cache_entry(key(), payload, "2026-11-17T00:00:00Z");
        assert_eq!(
            validate_cached(
                &key(),
                &entry.key.entity_id,
                entry.key.source_revision,
                entry.key.kind.as_str(),
                "deadbeef",
                entry.payload,
            ),
            None
        );
    }

    #[test]
    fn wrong_entity_or_kind_misses() {
        let payload = json!({"run_id": "run-1"});
        let entry = cache_entry(key(), payload, "2026-11-17T00:00:00Z");
        let other_entity = ProjectionCacheKey::new("run-2", 3, ProjectionKind::RunWorkspace);
        assert_eq!(
            validate_cached(
                &other_entity,
                &entry.key.entity_id,
                entry.key.source_revision,
                entry.key.kind.as_str(),
                &entry.payload_hash,
                entry.payload.clone(),
            ),
            None
        );
        let other_kind = ProjectionCacheKey::new("run-1", 3, ProjectionKind::RecordProjection);
        assert_eq!(
            validate_cached(
                &other_kind,
                &entry.key.entity_id,
                entry.key.source_revision,
                entry.key.kind.as_str(),
                &entry.payload_hash,
                entry.payload,
            ),
            None
        );
    }

    #[test]
    fn unknown_kind_string_misses() {
        let payload = json!({"run_id": "run-1"});
        let entry = cache_entry(key(), payload, "2026-11-17T00:00:00Z");
        assert_eq!(
            validate_cached(
                &key(),
                &entry.key.entity_id,
                entry.key.source_revision,
                "not_a_kind",
                &entry.payload_hash,
                entry.payload,
            ),
            None
        );
    }
}
