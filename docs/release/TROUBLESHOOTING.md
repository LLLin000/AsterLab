# Troubleshooting (Beta 0.1.0)

Every item cites the evidence it was verified against. Anything not verified
is labeled as such — do not treat it as a tested recovery.

## Install

| Symptom | Recovery | Evidence |
|---|---|---|
| Windows SmartScreen warns the app is from an unknown publisher | Expected: the Beta is unsigned. Click through only if you trust this build; never claim it is signed | `beta-scope.md` §1.2; `MYGO_0.1.0_BETA_RC_VERIFICATION.md` (Authenticode `NotSigned`) |
| Installer seems to have done nothing | Check `%LOCALAPPDATA%\Programs\AsterLab MyGo\` contains `mygo-run.exe`, `Runs.dll`, `BUILD_INFO.txt`, `uninstall.exe`, and HKCU `...\Uninstall\AsterLabMyGo` exists; reinstall with `setup.exe /S` | `evidence/RC-01/README.md` §4 |
| Want a clean reinstall | Uninstall (`uninstall.exe /S`) removes program folder, HKCU key, and shortcuts with zero residue; the user database is preserved, so reinstalling restores the app without losing data | `evidence/RC-01/README.md` §6 |
| App window never appears | Launch the installed `mygo-run.exe` directly; RC-01 reached a usable main window with workspace nav and `新建设计` (see `evidence/RC-01/first-launch.png`). If it still fails, the install layout check above comes first | `evidence/RC-01/README.md` §5 |

## Backup / restore

| Symptom | Recovery | Evidence |
|---|---|---|
| Deleted or lost records | Restore a backup copy over the closed database, then verify `PRAGMA integrity_check` → `ok` and row counts (fixture: `records` back to 4 rows, matching SHA-256) | `evidence/QA-05/README.md` § SQLite copy/restore acceptance |
| Old database won't open / migration interrupted | Retry the open: successful migration yields integrity `ok`, `user_version` 1, all legacy fields preserved. A failed migration leaves hash, schema, row count, and `user_version` untouched, so retrying or falling back to a backup is safe | `evidence/QA-05/` `legacy-migration-output.txt`, `failed-migration-output.txt` |
| Restore while the app is writing, or two restores at once | Stop: this is NOT RUN and unsupported. Finish current writes first; restores are single-user only | `evidence/QA-05/README.md` § QA test matrix (Concurrency: NOT RUN) |

## Common errors

| Symptom | Recovery | Evidence |
|---|---|---|
| `integrity_check` does not return `ok` | Do not keep writing. Restore the newest backup whose own `integrity_check` is `ok`, then re-verify row counts | `evidence/QA-05/README.md` (backup requires `ok` on both sides) |
| Database file locked / cannot copy | Close the app (graceful `WM_CLOSE` exits within 2 s) so no SQLite process holds the file, then copy | `MYGO_0.1.0_BETA_RC_VERIFICATION.md` § Close; `evidence/QA-05/README.md` (closed-file copies) |
| Out-of-date frozen document rejected at PDF export | Expected: the exporter rejects a frozen source whose content no longer matches its pinned source hash. Re-freeze from the current content and export again; the export never modifies the source | `pdf-export.md` |
| Untrusted file path rejected (`INVALID_INPUT` on link/open) | Expected: `asset.link` stores logical root ID + relative path only; absolute paths, drive/UNC prefixes, and escaping symlinks are rejected by design. Keep files under the linked root | `evidence/QA-06/README.md` |

## Not verified — do not rely on these

- Concurrent restore during active writes (NOT RUN — see above).
- UI backup/restore buttons exercised end-to-end on a live service from VPS
  (`evidence/QA-05/README.md` closing note).
- Signed-installer behavior (signing NOT TESTED).
