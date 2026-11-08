# AsterLab Run — native Rune slice

Gate 2 native Run surface. It uses pinned Rune `a7f276c...` and `rune-kit` components; it does not open SQLite directly. Reads/writes go through `labworkbench.exe` CLI.

## Requirements

- `target/release/labworkbench.exe`
- a data dir with a frozen Run
- Windows release build

## Run

```powershell
$env:ASTERLAB_RUN_ID = "<run-id>"
$env:LABWORKBENCH_DATA_DIR = "<repo>/target/asterlab-ui-runtime3"
$env:ASTERLAB_CLI = "<repo>/target/release/labworkbench.exe"
cargo run --release --manifest-path native/asterlab-run/Cargo.toml
```

The UI shows project folders, design files, Run timepoints, current task, checks, measurements and results. `ASTERLAB_RUN_ID` is optional; otherwise the first Run is used.

## Verification

- Native mouse click on the segmented tab and sidebar row was verified in the real window.
- Fonts: local Rune styles 20/16/14.5/12.5, families `Inter, Segoe UI, Microsoft YaHei`.
- Synthetic `computer.type` could not reliably insert into the custom-drawn TextInput; use a real keyboard for the save/restart check.

## Not yet

- Installer/signing.
