-- Legacy records schema used by the existing CLI compatibility test setup.
-- This recreates a version-0 SQLite database with representative project data.
PRAGMA user_version = 0;
CREATE TABLE records(
    id TEXT PRIMARY KEY,
    kind TEXT NOT NULL,
    project_id TEXT,
    parent_id TEXT,
    revision INTEGER NOT NULL,
    data_json TEXT NOT NULL,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);
INSERT INTO records VALUES
    ('qa05-project', 'project', NULL, NULL, 1,
     '{"name":"QA-05 legacy fixture","description":"backup/restore verification"}',
     '2026-11-13T00:00:00Z', '2026-11-13T00:00:00Z'),
    ('qa05-design', 'design', 'qa05-project', NULL, 1,
     '{"title":"Legacy design","objective":"preserve this record"}',
     '2026-11-13T00:00:00Z', '2026-11-13T00:00:00Z');
