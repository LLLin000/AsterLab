-- Fault-injection database: an earlier records layout without project_id.
-- The current open_db initializer's second index references project_id.
PRAGMA user_version = 0;
CREATE TABLE records(
    id TEXT PRIMARY KEY,
    kind TEXT NOT NULL,
    parent_id TEXT,
    revision INTEGER NOT NULL,
    data_json TEXT NOT NULL,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);
INSERT INTO records VALUES
    ('qa05-old-project', 'project', NULL, 1,
     '{"name":"QA-05 rollback fixture"}',
     '2026-11-13T00:00:00Z', '2026-11-13T00:00:00Z');
