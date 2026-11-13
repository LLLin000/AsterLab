-- Rebuild the checked-in version-0 fixture used by the CLI migration tests.
PRAGMA user_version = 0;
CREATE TABLE records(
    id TEXT PRIMARY KEY,
    kind TEXT NOT NULL,
    parent_id TEXT REFERENCES records(id) ON DELETE RESTRICT,
    revision INTEGER NOT NULL DEFAULT 1,
    data_json TEXT NOT NULL,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);
INSERT INTO records (id, kind, parent_id, revision, data_json, created_at, updated_at) VALUES
    ('project-legacy', 'project', NULL, 1,
     '{"name":"Legacy project","description":"kept"}',
     '2026-11-13T00:00:00Z', '2026-11-13T00:00:00Z'),
    ('design-legacy', 'design', 'project-legacy', 1,
     '{"title":"Legacy design","objective":"retained"}',
     '2026-11-13T00:00:01Z', '2026-11-13T00:00:01Z'),
    ('design-version-legacy', 'design_version', 'design-legacy', 1,
     '{"design_id":"design-legacy","revision":1,"snapshot":{"title":"Legacy design"}}',
     '2026-11-13T00:00:02Z', '2026-11-13T00:00:02Z'),
    ('protocol-legacy', 'protocol', 'project-legacy', 1,
     '{"title":"Legacy protocol","body":"retained"}',
     '2026-11-13T00:00:03Z', '2026-11-13T00:00:03Z');
