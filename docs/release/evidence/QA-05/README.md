# QA-05 SQLite backup/restore and upgrade evidence

## Scope and isolation

Runtime database tests use isolated copies in a fresh temporary directory; no user data directory was used. `legacy-v0.sql` builds a version-0 SQLite database with the legacy `records` layout used by existing CLI compatibility test setup. `legacy-v0.sqlite` is the reproducible fixture stored in this evidence directory.

## Test matrix

| Case | Scenario | Result |
|---|---|---|
| Normal | Integrity-check, copy to backup, compare every user-table row count, replace a modified source from backup, compare data and hashes | PASS for the closed-file `cp` workflow; one user table (`records`), 2 rows restored |
| Boundary | Apply current `open_db` table/index SQL to legacy layout; check data and `PRAGMA user_version` | Data preserved, integrity `ok`; `user_version` remains `0`, so no current version/upgrade contract exists |
| Failure | Inject an error in current schema-initialization SQL and compare source SHA-256 | FAIL: SQLite exits `1`; hash changes and the first index remains; no rollback |
| Restart | Reopen restored database in a new `sqlite3` process | PASS: integrity `ok`, `user_version` `0`, 2 rows |
| Concurrency | Restore during active writes / two simultaneous restores | NOT RUN: the documented operation requires writes to finish and forbids concurrent restores; no scoped CLI restore entry exists |

## Findings

- `src/cli.rs::open_db` issues `CREATE TABLE/INDEX IF NOT EXISTS`; it neither sets `PRAGMA user_version` nor contains a column migration. Applying its current schema SQL leaves the legacy fixture at version `0`.
- Injecting a missing `project_id` column makes the second index creation fail after WAL mode and the first index have been applied. The source hash changes; this DDL sequence is not transactional.
- `AGENT_API.md` lists no backup/restore CLI tool. `docs/release/BACKUP_RESTORE.md` documents UI backup and restore only; UI was intentionally not exercised.
- Rust CLI execution was unavailable: neither `cargo` nor `rustc` is installed. Schema initialization/failure checks below replay the exact DDL from `open_db` through `sqlite3`; they do not claim an application-level run. UI service backup/restore remains unverified.
- This result requires a Core repair card; no application code was changed in QA-05.

## Raw commands and output

SQLite version and tool availability:

```text
sqlite3 version: 3.45.1 2024-01-30 16:01:20 e876e51a0ed5c5b3126f52e532044363a014bc594cfefa87ffb5b82257ccalt1 (64-bit)
cargo: not installed
rustc: not installed
```

Fixture creation and baseline:

```sh
FIXTURE_ROOT=$(mktemp -d)
sqlite3 "$FIXTURE_ROOT/legacy-v0.sqlite" < docs/release/evidence/QA-05/legacy-v0.sql
sqlite3 "$FIXTURE_ROOT/legacy-v0.sqlite" 'PRAGMA integrity_check;'
sqlite3 "$FIXTURE_ROOT/legacy-v0.sqlite" 'PRAGMA user_version;'
sqlite3 "$FIXTURE_ROOT/legacy-v0.sqlite" "SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name; SELECT count(*) FROM records;"
```

```text
ok
0
records
2
```

Closed-file backup, table comparison, restore and restart checks (set `ROOT` to a fresh temporary directory; all output paths below are relative):

```sh
ROOT=$(mktemp -d)
mkdir -p "$ROOT/normal" "$ROOT/upgrade" "$ROOT/failure"
cp docs/release/evidence/QA-05/legacy-v0.sqlite "$ROOT/normal/labworkbench.sqlite"
sqlite3 "$ROOT/normal/labworkbench.sqlite" 'PRAGMA integrity_check;'
sqlite3 "$ROOT/normal/labworkbench.sqlite" "SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name;"
sqlite3 "$ROOT/normal/labworkbench.sqlite" 'SELECT count(*) FROM records;'
cp "$ROOT/normal/labworkbench.sqlite" "$ROOT/normal/backup.sqlite"
sqlite3 "$ROOT/normal/backup.sqlite" 'PRAGMA integrity_check;'
sqlite3 "$ROOT/normal/backup.sqlite" 'SELECT count(*) FROM records;'
sqlite3 "$ROOT/normal/labworkbench.sqlite" "DELETE FROM records WHERE id='qa05-design';"
sqlite3 "$ROOT/normal/labworkbench.sqlite" 'SELECT count(*) FROM records;'
cp "$ROOT/normal/backup.sqlite" "$ROOT/normal/labworkbench.sqlite"
sqlite3 "$ROOT/normal/labworkbench.sqlite" 'PRAGMA integrity_check;'
sqlite3 "$ROOT/normal/labworkbench.sqlite" 'SELECT count(*) FROM records;'
sqlite3 "$ROOT/normal/labworkbench.sqlite" 'SELECT id,kind,revision,data_json FROM records ORDER BY id;'
sqlite3 "$ROOT/normal/backup.sqlite" 'SELECT id,kind,revision,data_json FROM records ORDER BY id;'
sqlite3 "$ROOT/normal/labworkbench.sqlite" 'PRAGMA integrity_check; PRAGMA user_version; SELECT count(*) FROM records;'
(cd "$ROOT/normal" && sha256sum backup.sqlite labworkbench.sqlite)
```

```text
SOURCE INTEGRITY
ok
SOURCE USER TABLES
records
SOURCE RECORDS ROW COUNT
2
BACKUP INTEGRITY
ok
BACKUP RECORDS ROW COUNT
2
MODIFIED SOURCE ROW COUNT BEFORE RESTORE
1
RESTORED SOURCE INTEGRITY
ok
RESTORED SOURCE RECORDS ROW COUNT
2
ROW-BY-ROW SOURCE
qa05-design|design|1|{"title":"Legacy design","objective":"preserve this record"}
qa05-project|project|1|{"name":"QA-05 legacy fixture","description":"backup/restore verification"}
ROW-BY-ROW BACKUP
qa05-design|design|1|{"title":"Legacy design","objective":"preserve this record"}
qa05-project|project|1|{"name":"QA-05 legacy fixture","description":"backup/restore verification"}
REOPENED AFTER RESTORE: INTEGRITY / USER VERSION / RECORD COUNT
ok
0
2
FIXTURE SHA256
0e7cd11dfba48e8f7968b4c62ce91553e3ac5efeab8a0482af9fb1899cda8481  docs/release/evidence/QA-05/legacy-v0.sqlite
BACKUP AND RESTORED SOURCE SHA256
0e7cd11dfba48e8f7968b4c62ce91553e3ac5efeab8a0482af9fb1899cda8481  backup.sqlite
0e7cd11dfba48e8f7968b4c62ce91553e3ac5efeab8a0482af9fb1899cda8481  labworkbench.sqlite
```

Schema initialization uses the `open_db` DDL from `src/cli.rs` on a separate fixture copy. The exact SQL is recorded in the test command:
```sh

ROOT=$(mktemp -d)
mkdir -p "$ROOT/upgrade"
cp docs/release/evidence/QA-05/legacy-v0.sqlite "$ROOT/upgrade/labworkbench.sqlite"
sqlite3 "$ROOT/upgrade/labworkbench.sqlite" 'PRAGMA journal_mode=WAL; CREATE TABLE IF NOT EXISTS records(id TEXT PRIMARY KEY,kind TEXT NOT NULL,project_id TEXT REFERENCES records(id),parent_id TEXT REFERENCES records(id) ON DELETE RESTRICT,revision INTEGER NOT NULL DEFAULT 1,data_json TEXT NOT NULL,created_at TEXT NOT NULL,updated_at TEXT NOT NULL); CREATE INDEX IF NOT EXISTS records_kind_parent ON records(kind,parent_id); CREATE INDEX IF NOT EXISTS records_project ON records(project_id);'
sqlite3 "$ROOT/upgrade/labworkbench.sqlite" 'PRAGMA integrity_check; PRAGMA user_version; SELECT count(*) FROM records;'
```

```text
wal
ok
0
2
```

Failure injection against `failure-v0.sql` replays the same DDL. The `records` layout intentionally lacks `project_id`; the second index fails after the first index is created.
Failure injection commands (each run creates an isolated database and temporary directory):

```sh
ROOT=$(mktemp -d)
mkdir -p "$ROOT/failure"
sqlite3 "$ROOT/failure/labworkbench.sqlite" < docs/release/evidence/QA-05/failure-v0.sql
echo 'FAILURE INJECTION BEFORE SHA256'; (cd "$ROOT/failure" && sha256sum labworkbench.sqlite)
if sqlite3 "$ROOT/failure/labworkbench.sqlite" 'PRAGMA journal_mode=WAL; CREATE TABLE IF NOT EXISTS records(id TEXT PRIMARY KEY,kind TEXT NOT NULL,project_id TEXT REFERENCES records(id),parent_id TEXT REFERENCES records(id) ON DELETE RESTRICT,revision INTEGER NOT NULL DEFAULT 1,data_json TEXT NOT NULL,created_at TEXT NOT NULL,updated_at TEXT NOT NULL); CREATE INDEX IF NOT EXISTS records_kind_parent ON records(kind,parent_id); CREATE INDEX IF NOT EXISTS records_project ON records(project_id);'; then status=0; else status=$?; fi
echo "FAILURE INJECTION SQLITE EXIT: $status"
echo 'FAILURE INJECTION AFTER SHA256'; (cd "$ROOT/failure" && sha256sum labworkbench.sqlite)
echo 'FAILED INITIALIZATION DATABASE INTEGRITY / USER VERSION / RECORD COUNT'; sqlite3 "$ROOT/failure/labworkbench.sqlite" 'PRAGMA integrity_check; PRAGMA user_version; SELECT count(*) FROM records;'
echo 'FAILED INITIALIZATION INDEXES'; sqlite3 "$ROOT/failure/labworkbench.sqlite" "SELECT name FROM sqlite_master WHERE type='index' AND name NOT LIKE 'sqlite_%' ORDER BY name;"
```


```text
FAILURE INJECTION BEFORE SHA256: 276e654ee5ea4af288e4b50e354ccd956547f5cf9d9f2046e3c1b71a4d35b8e0
Error: in prepare, no such column: project_id
  TE INDEX IF NOT EXISTS records_project ON records(project_id);
                                      error here ---^
wal
FAILURE INJECTION SQLITE EXIT: 1
FAILURE INJECTION AFTER SHA256: 168a584b50dbb088969943b0874b27fcc0d8d4fd8d497e9fa7f36e24b943d388
FAILED INITIALIZATION DATABASE INTEGRITY / USER VERSION / RECORD COUNT
ok
0
1
FAILED INITIALIZATION INDEXES
records_kind_parent
```

`failure-v0.sql` and `legacy-v0.sql` are the reproducible fixture inputs. The captured failure hashes are taken immediately before and after the injected DDL failure; they differ, and `records_kind_parent` persists after the failed second index.
