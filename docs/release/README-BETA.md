# AsterLab 0.1.0 Beta

AsterLab is a local, single-user experiment design and execution workbench:
Windows x64 MyGo native desktop app + local Rust Core. No Rust/Go/Node needed
on the target machine; data stays in a local SQLite database. The app is
currently **unsigned** — expect a Windows SmartScreen prompt; this is the known
state, not a defect (`beta-scope.md` §1.2).

> Release-cut values (exact installer file name, SHA-256, source commit) are
> filled at release time and marked `[RELEASE-CUT]` below. Until then, the
> verified RC-01 build is quoted as the reference example.

## System requirements

- Windows 10/11 x64. No other platform ships a Beta GUI (`beta-scope.md` §1.3).
- No toolchain required. No admin rights required (user-level install).

## Install

1. Download `[RELEASE-CUT: installer file name, e.g. AsterLab-MyGo-0.1.0-beta-unsigned-win-x64-setup.exe]`.
2. (Optional) Verify SHA-256: `[RELEASE-CUT: SHA-256]`.
   Reference: RC-01 built `AsterLab-MyGo-0.1.0-beta-unsigned-win-x64-setup.exe`
   (16,667,781 bytes, SHA-256 `66059cfe…9e0b7899`) from `origin/main`
   @ `45d7ee8` — see `evidence/RC-01/README.md` §2.
3. Run the installer (silent install also works: `setup.exe /S`). It installs
   under `%LOCALAPPDATA%\Programs\AsterLab MyGo` (`mygo-run.exe`, `Runs.dll`,
   `BUILD_INFO.txt`, `uninstall.exe`), registers HKCU
   `...\Uninstall\AsterLabMyGo` (`DisplayVersion 0.1.0-beta-unsigned`), and
   creates Start Menu + Desktop shortcuts (`evidence/RC-01/README.md` §4).

## First launch

1. Launch **AsterLab MyGo** from the Start Menu or Desktop shortcut.
2. A real `AsterLab` main window opens (workspace nav, `新建设计` available;
   captured in `evidence/RC-01/first-launch.png`).
3. First launch creates `labworkbench.sqlite*` under the data directory: default `%LOCALAPPDATA%\AsterLab\data` (data file `%LOCALAPPDATA%\AsterLab\data\labworkbench.sqlite`; `store.rs` falls back to `~/.local/share/AsterLab/data` only when `LOCALAPPDATA` is unset, i.e. non-Windows dev runs — not a Beta path). Override with `LABWORKBENCH_DATA_DIR` (RC-01 verified first launch against an isolated data dir).

## First workflow

Project → Design → Protocol → View/Workflow → Run → Actual Records → History/Export.

- Run creation is a human action in the UI.
- Synthetic examples are explicitly marked synthetic.
- Backup/restore: see `BACKUP_RESTORE.md` (in-app buttons) and the file semantics behind them. Proven: QA-05 closed-file copy/restore (4/4 rows, `integrity_check` ok). **Not proven:** the in-app button path end-to-end (RC-05 `notes-vs-scope.md`, DS-09 partial); concurrent restore / restore during writes is unsupported — finish writes first. Problems: see `TROUBLESHOOTING.md`.
- What changed in this Beta: see `CHANGELOG.md`.

## Uninstall

Run `uninstall.exe /S` (or Apps → Uninstall). The program folder, HKCU
uninstall key, and shortcuts are removed with zero residue; the user database
under `%LOCALAPPDATA%\AsterLab` is left untouched and stays readable
(`evidence/RC-01/README.md` §6).
