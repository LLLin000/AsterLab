# Backup and restore

Single-user, local-only operation. Aligned with the QA-05 acceptance matrix
(`evidence/QA-05/README.md`); anything QA-05 marks NOT RUN is marked
unsupported below, not described as working.

## Backup

In the running UI, choose **备份数据库**. The service creates a consistent
SQLite copy under `<data-dir>/backups/labworkbench-YYYYMMDD-HHMMSS.sqlite`.

Verified semantics (`evidence/QA-05/README.md` § SQLite copy/restore
acceptance): back up with SQLite closed to new writes; the backup passes
`PRAGMA integrity_check`, carries the same user tables and row counts
(fixture: `records` = 4 rows), and is byte-identical after restore (`cmp`
identical, matching SHA-256).

## Restore

1. Finish current writes; do not run two restores at once (concurrent restore
   is NOT RUN per QA-05 and is unsupported).
2. In the UI choose **恢复备份** and enter the full path of a backup file.
3. The service verifies `PRAGMA integrity_check`, replaces the database, and
   reopens it.
4. Recheck the project list and recent records after restore. A deleted row is
   recoverable from the copy: QA-05 deleted one fixture row (4 → 3) and
   restored it to 4 rows with matching hashes.

A restore is a local single-user operation. It is not a signed audit event.

## Schema migration (automatic, safe to interrupt)

Opening an older (v0) database migrates it automatically: integrity `ok`,
schema `user_version` → 1, every legacy field preserved, and a reopen
re-checks clean (`evidence/QA-05/README.md` + `legacy-migration-output.txt`).
If migration itself fails, the database is left untouched — hash, schema, row
count, and `user_version` are identical before and after the failed open
(`failed-migration-output.txt`). You can simply retry or fall back to a backup.

## What is NOT covered

- Concurrent restores / restore during active writes: NOT RUN, unsupported —
  finish writes first (`evidence/QA-05/README.md` § QA test matrix).
- The UI backup/restore buttons were validated for closed-file copy semantics
  on fixtures, not exercised end-to-end on a live service from VPS
  (`evidence/QA-05/README.md` closing note).
