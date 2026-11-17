# Rollback — AsterLab v0.1.0-beta (unsigned MyGo build)

Scope: the Windows x64 per-user install
(`%LOCALAPPDATA%\Programs\AsterLab MyGo`, HKCU uninstall key
`AsterLabMyGo`, start-menu + desktop shortcuts). Verified by the RC-05
drill (`docs/release/evidence/RC-05/drill-log.md`, base `e2a4603`) and
RC-02 (`docs/release/evidence/RC-02/README.md`).

Structural guarantee (unmodified `scripts/mygo-installer.nsi`): setup
and uninstall only ever write INSTDIR, the two shortcuts, and the HKCU
uninstall key. The user data dir is never created, modified, or deleted
by either. Rollback safety is structural, not procedural.

## Supported paths

1. **Reinstall the same Beta** (verified RC-05): run
   `AsterLab-MyGo-0.1.0-beta-unsigned-win-x64-setup.exe` again
   (silent: `setup.exe /S`), or uninstall first via
   `uninstall.exe /S` (or Add/Remove Programs) and then reinstall.
   Installed binaries overwrite cleanly; the database is byte-identical
   before and after (RC-05: `1ABEE49A…` across the full
   install → uninstall → reinstall cycle, `integrity_check` ok, 4/4
   rows, app reopens on the same data).
2. **Overwrite-upgrade with a newer Beta build** (verified RC-02
   pattern): run the newer setup exe over the existing install — same
   overwrite semantics, data dir untouched. Re-verify with a manifest
   (`sha256sum` before/after) as the drill log shows.
3. **Uninstall and walk away** (verified RC-01/RC-02/RC-05): silent
   uninstall removes INSTDIR, both shortcuts, and the HKCU key, and
   leaves the data dir fully intact for a later reinstall.

## Limits — read before rolling back

- **Downgrade to an older build is NOT verified.** The installer has no
  version gate (it blindly overwrites), and the data dir only migrates
  forward (`user_version` 0 → 1 on first open; there is no
  down-migration). Do not install an older Beta over data a newer Beta
  has opened.
- **Always back up first.** Copy `labworkbench.sqlite` (app closed)
  and confirm `PRAGMA integrity_check = ok` before any rollback; see
  `docs/release/BACKUP_RESTORE.md`. The in-app restore path replaces
  the live DB — finish all writes first; concurrent restore is
  unsupported (QA-05 NOT RUN).
- **Same machine only.** The data dir is single-machine SQLite (WAL);
  do not copy it across platforms or share it between installs
  (`beta-scope.md` §1.1, §3 item 9).
- **One install location.** The per-user installer owns exactly one
  INSTDIR; do not run two Beta versions side by side from the same
  data dir.
- Signing: all artifacts are unsigned (`NotSigned`); Windows SmartScreen
  will warn on download/launch. That is the known Beta state.

## If the app does not start after a rollback

1. Confirm no `mygo-run` process is stuck (`tasklist`), then retry.
2. Confirm the DB still passes `PRAGMA integrity_check`; if not,
   restore the pre-rollback copy (file copy, app closed) and reopen.
3. Reinstall the same Beta setup exe once more (overwrite is idempotent)
   and re-check installed hashes against the release inventory.
