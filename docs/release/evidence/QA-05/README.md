# QA-05 SQLite schema migration fixtures

`legacy-v0.sqlite` is an immutable SQLite v0 database. Its `records` table has the legacy columns `id`, `kind`, `parent_id`, `revision`, `data_json`, `created_at`, and `updated_at`; it intentionally has no `project_id` column and no secondary indexes. It contains a project, a design, a design version, and a protocol record with fixed payloads and timestamps. `PRAGMA user_version` is 0.

The companion `legacy-v0.sql` rebuilds the same v0 schema and rows; replaying it reports `user_version` 0 and integrity `ok`.

## QA test matrix

| Case | Scenario | Result |
|---|---|---|
| Normal | Check integrity, copy a closed database, compare the user-table row count, restore a deleted row from the copy, compare hashes | PASS: `records` has 4 rows before backup and after restore; backup and restored file hashes match |
| Boundary | Open a v0 fixture through the CLI database opener and check migrated data and schema version | PASS: current migration tests assert integrity `ok`, schema version 1, and exact legacy-row preservation; see [legacy migration output](legacy-migration-output.txt) |
| Failure | Abort migration after its first schema change and compare database state | PASS: [failure test output](failed-migration-output.txt) records the test pass; test compares hash, schema, row count, and user_version |
| Restart | Reopen the migrated database and recheck integrity and data | PASS: covered by the legacy migration test |
| Concurrency | Restore during active writes or run two restores concurrently | NOT RUN: the documented restore is single-user and requires writes to finish; no concurrent restore surface was exercised |


The migration test copies this fixture into a temporary data directory, opens it through the CLI database opener, and checks integrity, current `user_version`, and exact preservation of every legacy field. It closes and reopens the migrated database, then repeats the checks. The checked-in fixture itself remains v0 and is copied afresh on every test run.

`failure-v0.sql` installs a deterministic `BEFORE UPDATE` abort trigger on a disposable copy of the fixture. The migration adds `project_id` as its first persistent schema change and then backfills project ownership. The trigger aborts that backfill. The failure test compares the database SHA-256, schema, row count, and `user_version` before and after the failed open; the original fixture is not modified.

Run the focused checks from the repository root:

```text
cargo test --bin labworkbench schema_migration_legacy_v0
cargo test --bin labworkbench failed_schema_migration_preserves_database
```
The card's literal `cargo test --bin labworkbench schema_migration_legacy_v0 -- --exact` filter matches zero tests because the harness name is `cli::tests::schema_migration_legacy_v0`. The qualified exact command was run twice; both outputs are retained.

Cargo test output files retain the command, compiler diagnostics, test names, and results; only environment-specific absolute workspace/build paths are replaced with `[workspace path redacted]` or `[build output path redacted]` to avoid disclosing host filesystem layout.
Raw command outputs: [legacy migration](legacy-migration-output.txt), [failed migration](failed-migration-output.txt), [literal exact filter, run 1](exact-filter-run-1.txt), [literal exact filter, run 2](exact-filter-run-2.txt), [qualified exact test, run 1](qualified-exact-run-1.txt), [qualified exact test, run 2](qualified-exact-run-2.txt), [full CLI suite](labworkbench-tests.txt), [Core suite](asterlab-core-tests.txt), and [App suite](asterlab-app-tests.txt).

## SQLite copy/restore acceptance

All operations used a fresh temporary directory and closed SQLite processes between file copies. The fixture has one user table, `records`.

```sh
ROOT=$(mktemp -d)
cp docs/release/evidence/QA-05/legacy-v0.sqlite "$ROOT/labworkbench.sqlite"
cd "$ROOT"
echo 'SOURCE INTEGRITY'
sqlite3 labworkbench.sqlite 'PRAGMA integrity_check;'
echo 'SOURCE USER TABLES AND ROW COUNTS'
sqlite3 labworkbench.sqlite "SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name; SELECT count(*) FROM records;"
cp labworkbench.sqlite backup.sqlite
echo 'BACKUP INTEGRITY'
sqlite3 backup.sqlite 'PRAGMA integrity_check;'
echo 'BACKUP USER TABLES AND ROW COUNTS'
sqlite3 backup.sqlite "SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name; SELECT count(*) FROM records;"
sqlite3 labworkbench.sqlite "DELETE FROM records WHERE id='protocol-legacy';"
echo 'SOURCE ROW COUNT AFTER SIMULATED LOSS'
sqlite3 labworkbench.sqlite 'SELECT count(*) FROM records;'
cp backup.sqlite labworkbench.sqlite
echo 'RESTORED INTEGRITY AND ROW COUNT'
sqlite3 labworkbench.sqlite 'PRAGMA integrity_check; SELECT count(*) FROM records;'
echo 'RESTORE FILE COMPARISON'
if cmp -s labworkbench.sqlite backup.sqlite; then echo 'identical'; else echo 'different'; exit 1; fi
echo 'BACKUP AND RESTORED SOURCE SHA256'
sha256sum backup.sqlite labworkbench.sqlite
```

```text
SOURCE INTEGRITY
ok
SOURCE USER TABLES AND ROW COUNTS
records
4
BACKUP INTEGRITY
ok
BACKUP USER TABLES AND ROW COUNTS
records
4
SOURCE ROW COUNT AFTER SIMULATED LOSS
3
RESTORED INTEGRITY AND ROW COUNT
ok
4
RESTORE FILE COMPARISON
identical
BACKUP AND RESTORED SOURCE SHA256
9e21bc796ddb02222549d988ccf2d2dc7ae6586b99c416a8dbd8a6579dc8dd12  backup.sqlite
9e21bc796ddb02222549d988ccf2d2dc7ae6586b99c416a8dbd8a6579dc8dd12  labworkbench.sqlite
```

The database backup/restore above validates closed-file copy semantics, not the UI service implementation. The UI entry is documented in [BACKUP_RESTORE.md](../../BACKUP_RESTORE.md); it was not exercised on VPS.
