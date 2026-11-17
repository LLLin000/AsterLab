# RC-02 evidence — upgrade install preserves user data, uninstall leaves zero residue

Card: RC-02. Base: `origin/main` @ `4fc6c54` (fresh ovnfs checkout `rc02`, `git reset --hard && git clean -fdx`, `git fetch origin main && git merge origin/main`).
Host: Windows x64, user-level install (HKCU + `%LOCALAPPDATA%\Programs\AsterLab MyGo`). No app code touched. Reads: RC-01 + QA-05 evidence.

**Retention policy: the installer and uninstaller only ever write `INSTDIR`, the two shortcuts, and the `HKCU\...\Uninstall\AsterLabMyGo` key — the user data dir is never created, modified, or deleted by setup.**

## 0. `.nsi` analysis — unmodified, no fix needed

`scripts/mygo-installer.nsi` (untouched): `File` overwrites `mygo-run.exe`/`Runs.dll`/`BUILD_INFO.txt` on reinstall (upgrade = overwrite, no version gate); `WriteUninstaller`/`WriteRegStr` are idempotent; the `Uninstall` section deletes only `INSTDIR` files + `RMDir INSTDIR` + the two shortcuts + `DeleteRegKey HKCU\...\AsterLabMyGo`. No data-dir path appears anywhere in the script, so preservation is structural. `RequestExecutionLevel user` + `SetShellVarContext current` keep everything per-user.

## 1. Build (dist + installer from checkout)

`powershell -File scripts/mygo-build.ps1` (with `GOCACHE`/`TEMP` on `D:/.ci-cache`), then `makensis scripts/mygo-installer.nsi` (v3.13; only benign 6001 MUI warnings, same as RC-01).

`dist/BUILD_INFO.txt` (commit `4fc6c54`, mygo pin `v0.8.4-0.20261108220316-c4197b7fe562`):

```text
Runs.dll SHA256 B5EEA2122B92F7BD894D5529D24883DD2D37DBE52F1FB24160266D96B4E5ED9C
mygo-run.exe SHA256 0798C9D302B9D3821BD221E7DC82015127E7F5998B9A97555F6A6B96298734C
```

Independent re-hash: `BUILD_INFO.txt` `60EB1A8D…C18FF`, `Runs.dll` `B5EEA212…B4E5ED9C`, `mygo-run.exe` `0798C9D3…96298734C` (match).
Installer `release/AsterLab-MyGo-0.1.0-beta-unsigned-win-x64-setup.exe`: SHA256 `7F6735BE7EAF16C3398C50C4B0A92F9B7EA3275B06CC8D2C67F32E73A7397BF8` (16670647 bytes). Setup exe is git-ignored, rebuildable from the above.

## 2. Seed isolated data dir with old-version database

```text
cp docs/release/evidence/QA-05/legacy-v0.sqlite D:/.ci-cache/rc02-data/labworkbench.sqlite
```

Seed state: SHA256 `9E21BC796DDB02222549D988CCF2D2DC7AE6586B9C416A8DBD8A6579DC8DD12` (== QA-05 fixture hash), `integrity_check` ok, `records` = 4, `user_version` = 0. Host `%LOCALAPPDATA%\AsterLab` untouched (all launches used this dir via `LABWORKBENCH_DATA_DIR`).

## 3. Install v1 → open old data

`setup.exe /S` → exit 0. Installed `mygo-run.exe`/`Runs.dll`/`BUILD_INFO.txt` hashes match dist; `BUILD_INFO.txt` match. HKCU key present (`DisplayName`, `UninstallString`, `InstallLocation`, `DisplayVersion 0.1.0-beta-unsigned`, `NoModify/NoRepair`=1); start-menu + desktop shortcuts present. DB hash still seed hash.

First launch (`LABWORKBENCH_DATA_DIR=D:/.ci-cache/rc02-data`, `mygo-automation open/wait/snapshot`): window `AsterLab` focused; snapshot shows `Legacy design`, `Legacy project`, `1 个项目 · 1 个设计 · 0 个运行`. Screenshot `v1-open-old-data.png` (window foregrounded first). Graceful close (`CloseMainWindow`, exited ≤8 s).

First open migrates the v0 DB in place: `user_version` 0 → 1, schema gains `project_id` + `records_kind_parent`/`records_project` indexes; all 4 legacy rows preserved exactly (ids/kinds/parents/revisions/payloads/timestamps byte-identical, `integrity_check` ok). Screenshot + snapshot are the "opens old data" proof.

## 4. Upgrade: install v2 over v1

Same installer run again (`/S`, exit 0) over the existing install. Installed binaries re-verified against dist hashes (overwrite confirmed). DB hash identical before and after the overwrite install (`1ABEE49A…9254F4` both sides). Post-upgrade launch shows the same legacy data (`Legacy design`, `1 个项目 · 1 个设计 · 0 个运行`); screenshot `v2-upgraded-open-old-data.png`; graceful close; DB hash unchanged, `integrity_check` ok, 4 rows.

DB hash log across the whole run (`labworkbench.sqlite`):

| phase | SHA256 (prefix) | `user_version` | rows | integrity |
|---|---|---|---|---|
| seed (QA-05 v0) | `9E21BC79…` | 0 | 4 | ok |
| after v1 first launch + graceful close | `33118C49…` | 1 | 4, legacy payloads exact | ok |
| v2 overwrite pre/post + all later launches/closes | `1ABEE49A…` | 1 | 4, legacy payloads exact | ok |

The `33118C49 → 1ABEE49A` step happened while the app was closed (shutdown checkpoint at storage level); row dump before and after is identical (see §6), so no user data changed — only page layout.

## 5. Uninstall — zero residue, data dir byte-identical

`uninstall.exe /S` → exit 0. Verified absent: `%LOCALAPPDATA%\Programs\AsterLab MyGo`, `HKCU\...\Uninstall\AsterLabMyGo` (`reg query` → not found), start-menu folder, desktop `.lnk`. Screenshot `uninstalled-no-instdir.png` shows the foregrounded verification terminal (INSTDIR ABSENT, HKCU not found, both shortcut `Test-Path` False).
Data dir: `sha256sum -c` against the pre-uninstall manifest → `labworkbench.sqlite: OK`; `integrity_check` ok, 4 rows, `user_version` 1.

## 6. Reinstall — old data opens again

`setup.exe /S` → exit 0; installed exe hash matches dist; DB hash unchanged (`1ABEE49A…`). Launch: snapshot again shows `Legacy design` / `Legacy project` / `1 个项目 · 1 个设计 · 0 个运行`; screenshot `v3-reinstalled-open-old-data.png`; graceful close; DB hash unchanged, integrity ok, 4 rows. Final row dump (all four legacy payloads byte-exact vs `legacy-v0.sql`):
```text
project-legacy|project||1|{"name":"Legacy project","description":"kept"}|2026-11-13T00:00:00Z|2026-11-13T00:00:00Z
design-legacy|design|project-legacy|1|{"title":"Legacy design","objective":"retained"}|2026-11-13T00:00:01Z|2026-11-13T00:00:01Z
design-version-legacy|design_version|design-legacy|1|{"design_id":"design-legacy","revision":1,"snapshot":{"title":"Legacy design"}}|2026-11-13T00:00:02Z|2026-11-13T00:00:02Z
protocol-legacy|protocol|project-legacy|1|{"title":"Legacy protocol","body":"retained"}|2026-11-13T00:00:03Z|2026-11-13T00:00:03Z
```

Post-verification cleanup: uninstalled again (exit 0, INSTDIR/HKCU/shortcuts gone, no `mygo-run` process); host left as found. Isolated data dir retained at `D:/.ci-cache/rc02-data` (outside the repo, host `%LOCALAPPDATA%\AsterLab` never written).

## Screenshots

- `v1-open-old-data.png` — installed v1 window open on legacy data (mygo-automation capture, window foregrounded first).
- `v2-upgraded-open-old-data.png` — same after overwrite upgrade install.
- `v3-reinstalled-open-old-data.png` — same after uninstall + reinstall.
- `uninstalled-no-instdir.png` — foregrounded terminal proving INSTDIR/HKCU/shortcuts gone, data dir intact.
