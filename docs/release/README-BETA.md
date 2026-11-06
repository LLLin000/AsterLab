# AsterLab 0.1.0 Beta

AsterLab is a local, single-user experiment design and execution workbench. It runs on
Windows x64 without Rust or Node; data stays in a local SQLite database.

## Start

Double-click `START_HERE.bat`. The app starts at `http://127.0.0.1:8832` and stores data
under `%LOCALAPPDATA%\AsterLab`. To keep data beside the package, start `AsterLab.exe`
with `LABWORKBENCH_DATA_DIR` pointing to a folder.

## First workflow

Project → Design → Protocol → View/Workflow → Run → Actual Records → History/Export.

- Run creation is a human action in the UI.
- Synthetic examples are explicitly marked synthetic.
- `operator` / `checked_by` are recorded labels, not authenticated signatures.
- See `BACKUP_RESTORE.md` and `KNOWN_LIMITATIONS.md`.
