# QA-05 SQLite schema migration fixtures

`legacy-v0.sqlite` is an immutable SQLite v0 database. Its `records` table has the legacy columns `id`, `kind`, `parent_id`, `revision`, `data_json`, `created_at`, and `updated_at`; it intentionally has no `project_id` column and no secondary indexes. It contains a project, a design, a design version, and a protocol record with fixed payloads and timestamps. `PRAGMA user_version` is 0.

The migration test copies this fixture into a temporary data directory, opens it through the CLI database opener, and checks integrity, current `user_version`, and exact preservation of every legacy field. It closes and reopens the migrated database, then repeats the checks. The checked-in fixture itself remains v0 and is copied afresh on every test run.

`failure-v0.sql` installs a deterministic `BEFORE UPDATE` abort trigger on a disposable copy of the fixture. The migration adds `project_id` as its first persistent schema change and then backfills project ownership. The trigger aborts that backfill. The failure test compares the database SHA-256, schema, row count, and `user_version` before and after the failed open; the original fixture is not modified.

Run the focused checks from the repository root:

```text
cargo test --bin labworkbench schema_migration_legacy_v0
cargo test --bin labworkbench failed_schema_migration_preserves_database
```
The card's literal `cargo test --bin labworkbench schema_migration_legacy_v0 -- --exact` filter matches zero tests because the harness name is `cli::tests::schema_migration_legacy_v0`. The qualified exact command was run twice; both outputs are retained.

Raw command outputs: [legacy migration](legacy-migration-output.txt), [failed migration](failed-migration-output.txt), [literal exact filter, run 1](exact-filter-run-1.txt), [literal exact filter, run 2](exact-filter-run-2.txt), [qualified exact test, run 1](qualified-exact-run-1.txt), [qualified exact test, run 2](qualified-exact-run-2.txt), [full CLI suite](labworkbench-tests.txt), [Core suite](asterlab-core-tests.txt), and [App suite](asterlab-app-tests.txt).
