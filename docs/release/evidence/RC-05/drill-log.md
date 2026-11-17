# RC-05 drill log — install → uninstall → reinstall, user data byte-identical

Base: `e2a4603` (checkout `rc05`). Isolated data dir
`D:/.ci-cache/rc05-data` (host `%LOCALAPPDATA%\AsterLab` never written;
all launches used `LABWORKBENCH_DATA_DIR`). Seed:
`docs/release/evidence/QA-05/legacy-v0.sqlite`
(SHA256 `9E21BC796DD…6586B99C`, `integrity_check` ok, 4 rows, `user_version` 0).
Scripts used (outside the repo, `D:/.ci-cache/rc05-scripts/`):
`01-install.ps1`, `02-launch.ps1`, `03-shot.ps1`, `04-close.ps1`,
`05-uninstall.ps1`, `06c-uninstalled-shot.ps1`. No app code touched.

## 0. Pre-install — clean

`reg query HKCU\...\Uninstall\AsterLabMyGo` → not found;
`%LOCALAPPDATA%\Programs\AsterLab MyGo` absent.

## 1. Install Beta (`setup.exe /S`)

Exit ok. Installed `mygo-run.exe` / `Runs.dll` / `BUILD_INFO.txt` hashes
match dist (`2B3A4636…`, `F0F12C9A…`, `E11A3732…`). HKCU key present
(`DisplayName` = `AsterLab MyGo (Unsigned Beta)`,
`DisplayVersion` = `0.1.0-beta-unsigned`, `NoModify/NoRepair` = 1);
start-menu folder + desktop shortcut present.
DB still seed hash `9E21BC79…` — install does not touch the data dir.

## 2. First launch on seeded (v0) data

`Start-Process` installed exe with
`LABWORKBENCH_DATA_DIR=D:/.ci-cache/rc05-data`; attached via
`mygo-automation open` (socket needed a few seconds after process start;
first `open` raced startup and was retried). Snapshot shows the `AsterLab`
window with all six surfaces (项目工作区 / 实验设计 / 实验方案 /
实验运行 / Notebook / 资料与工具) and
`Legacy design` / `Legacy project` / `1 个项目 · 1 个设计 · 0 个运行`.
Window foregrounded (`SetForegroundWindow`), capture:
`beta-open-data.png` (md5 `259ab23f…`, 2048×1848).
Graceful close (`CloseMainWindow`, exited ≤ 8 s, no process left).

DB after close: `33118C49…` at first read, then `1ABEE49A…` after the
storage checkpoint (same `33118C49 → 1ABEE49A` step RC-02 recorded);
`user_version` 0 → 1, 4 rows, `integrity_check` ok, all four legacy
payloads exact
(`project-legacy`, `design-legacy`, `design-version-legacy`,
`protocol-legacy`, each revision 1). Manifest `rc05-manifest-postopen.txt`:
`1ABEE49A… labworkbench.sqlite` (single file; WAL checkpointed away).

## 3. Uninstall (`uninstall.exe /S`) — zero residue, data intact

Exit ok. Verified absent: INSTDIR, HKCU `AsterLabMyGo` key
(`reg query` → not found), start-menu folder, desktop shortcut; no
`mygo-run` process. Data dir: `sha256sum -c rc05-manifest-postopen.txt`
→ `labworkbench.sqlite: OK`; `integrity_check` ok, 4 rows,
`user_version` 1. Capture `uninstalled-proof.png` (md5 `67cdd138…`):
foregrounded form showing the live verification (INSTDIR/HKCU/shortcuts
absent, manifest OK, `ok / 4 / 1`). Method note: this session is headless
for spawned consoles (no `MainWindowHandle` on child shells — the same
reason `02-launch.ps1`'s inline `open` raced), so the proof panel is an
in-process WinForms form rather than a console; content is the verbatim
output of the commands above, and the app screenshots use
`mygo-automation screenshot` of the real window as in RC-01/RC-02.

## 4. Reinstall same Beta (`setup.exe /S`) — old data opens again

Exit ok; installed exe hash matches dist; DB hash unchanged
(`1ABEE49A…`). Launch (same isolated dir): snapshot again shows
`Legacy design` / `Legacy project` / `1 个项目 · 1 个设计 · 0 个运行`;
capture `reinstalled-open-data.png` (md5 `f985f01a…`, 2048×1848 —
distinct bytes from the stage-2 shot). Graceful close. Final state:
manifest `OK`, `integrity_check` ok, 4 rows, `user_version` 1, row dump
identical to §2.

## 5. Cleanup — host left as found

Uninstalled again (exit ok; INSTDIR/HKCU/shortcuts gone, no process).
Host `%LOCALAPPDATA%\AsterLab` untouched throughout. Isolated data dir
retained at `D:/.ci-cache/rc05-data` (outside the repo); drill scripts at
`D:/.ci-cache/rc05-scripts/` (outside the repo, not committed).

## Result

Install → uninstall → reinstall preserves the user database
byte-identical (`1ABEE49A…` before and after the uninstall/reinstall
cycle) and the reinstalled app opens it (window + snapshot proof).
Rollback path "reinstall same Beta" VERIFIED at `e2a4603`.
