# RC-01 evidence — MyGo NSIS installer: build, install, first-launch, uninstall

Card: RC-01 (Windows Work) MyGo NSIS installer, fresh user-level install + first launch + uninstall cleanliness.
Base: `origin/main` @ `45d7ee8dc16835222b8962c45dd2e6adcf26c161` (fresh ovnfs checkout `rc01`, `git reset --hard && git clean -fdx`, merged `origin/main`).
Host: Windows x64, user-level install (HKCU + `%LOCALAPPDATA%\Programs\AsterLab MyGo`). No app code touched.

## 1. dist artifacts (built from checkout via `scripts/mygo-build.ps1`)

`BUILD_INFO.txt` (commit `45d7ee8`, mygo pin `v0.8.4-0.20261108220316-c4197b7fe562`):

```text
AsterLab MyGo Prototype (UNSIGNED)
commit 45d7ee8
mygo pin v0.8.4-0.20261108220316-c4197b7fe562
Runs.dll SHA256 F1157436737B2FBEC0540C1DB1A3291FDF670D9AFB3582FA7585044A2AEC266C
mygo-run.exe SHA256 350AAD07B53EE65A91CB623E91265E504D945DF6453AF87F46115320E43D71AF
```

SHA256 (independent re-hash):

```text
245b3cac6046f9669de5f68b211b296e2031e3377f9502133f7489761da43503  BUILD_INFO.txt
f1157436737b2fbec0540c1db1a3291fdf670d9afb3582fa7585044a2aec266c  Runs.dll
350aad07b53ee65a91cb623e91265e504d945df6453af87f46115320e43d71af  mygo-run.exe
```

## 2. Installer build (`scripts/mygo-installer.nsi`, unmodified)

```text
makensis.exe mygo-installer.nsi
Total size: 16667781 / 42533911 bytes (39.1%)
12 warnings: 6001 mui.Header/mui.Button vars unused (benign — SilentInstall, no MUI pages used)
```

- `release/AsterLab-MyGo-0.1.0-beta-unsigned-win-x64-setup.exe`
  SHA256 `66059cfe85b32cd2a708b4c5842034214aca74c01b7f0c28e4cde7299e0b7899` (16667781 bytes)
- The `.nsi` needed **no fix**: `File` paths (`..\prototypes\mygo-run\dist\...`) match the current dist layout; user-level semantics kept (`RequestExecutionLevel user`, `SetShellVarContext current`, HKCU uninstall key).
- The built setup exe is git-ignored (`/release/` in `.gitignore`) and not committed; rebuildable from the inventory above.

## 3. Pre-install state: clean

- `HKCU\...\Uninstall\AsterLabMyGo`: absent (`reg query` → not found).
- `%LOCALAPPDATA%\Programs\AsterLab MyGo`: absent.

## 4. Install (`setup.exe /S`)

Verified after install:

- `%LOCALAPPDATA%\Programs\AsterLab MyGo\` contains `mygo-run.exe`, `Runs.dll`, `BUILD_INFO.txt`, `uninstall.exe`.
- `HKCU\Software\Microsoft\Windows\CurrentVersion\Uninstall\AsterLabMyGo`: `DisplayName` = `AsterLab MyGo (Unsigned Beta)`, `UninstallString` = `"<INSTDIR>\uninstall.exe"`, `InstallLocation` = INSTDIR, `DisplayVersion` = `0.1.0-beta-unsigned`, `NoModify/NoRepair` = 1.
- Shortcuts present: `%SMPROGRAMS%\AsterLab MyGo\AsterLab MyGo.lnk`, `%DESKTOP%\AsterLab MyGo.lnk`.

## 5. First launch (isolated data dir, installed exe)

```text
$env:LABWORKBENCH_DATA_DIR='D:/.ci-cache/rc01-data'  # host's %LOCALAPPDATA%\AsterLab untouched
Start-Process 'C:\Users\Lin\AppData\Local\Programs\AsterLab MyGo\mygo-run.exe'
```

- App reached a usable main window: `mygo-automation` snapshot shows the `AsterLab` window with workspace nav (项目工作区 / 实验设计 / 实验方案 / 实验运行 / Notebook / 资料与工具), `0 个设计 · 0 个运行`, and `新建设计` action available. No missing-dependency / load failure.
- `first-launch.png`: real-window screenshot (window foregrounded first). Sidebar renders dark in the capture; nav items confirmed present via automation snapshot.
- First launch created `labworkbench.sqlite*` only inside the isolated `D:/.ci-cache/rc01-data`.

## 6. Uninstall (`uninstall.exe /S`) — zero residue

- `%LOCALAPPDATA%\Programs\AsterLab MyGo`: gone.
- `HKCU\...\Uninstall\AsterLabMyGo`: gone (`reg query` → not found).
- Start-menu folder and desktop shortcut: gone.
- `%LOCALAPPDATA%\AsterLab` (user's live data): still present, never written by this verification (all launches used the isolated data dir). Host left as found.
