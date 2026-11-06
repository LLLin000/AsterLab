# Backup and restore

## Backup

In the running UI, choose **备份数据库**. The service creates a consistent SQLite copy under
`<data-dir>/backups/labworkbench-YYYYMMDD-HHMMSS.sqlite`.

## Restore

1. Finish current writes; do not run two restores at once.
2. In the UI choose **恢复备份** and enter the full path of a backup file.
3. The service verifies `PRAGMA integrity_check`, replaces the database, and reopens it.
4. Recheck the project list and recent records after restore.

A restore is a local single-user operation. It is not a signed audit event.
