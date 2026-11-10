# MyGo Rust Native Module — shared Core integration

- MyGo pseudo-version: `v0.8.4-0.20261108220316-c4197b7fe562`
- `v0.8.3` has no `mygo native` command; use the pinned pseudo-version.
- Module: `modules/runs`
- Shared Rust library: `crates/asterlab-app`
- Core: existing `src/cli.rs` command implementation, included once by both CLI and desktop

## Architecture

```text
Go MyGo UI
  └─ generated runs.RunService (Go)
       └─ generated Rust RunService trait
            └─ modules/runs/rust (cdylib)
                 └─ asterlab-app::AppCore
                      └─ existing CLI command implementation
                           └─ SQLite / transactions / validation
```

`asterlab-app` includes `src/cli.rs` as its implementation module and exposes:

```rust
AppCore::open(data_dir)
AppCore::load_run(run_id)
AppCore::save_readout(event)
```

`save_readout` runs `create_execution_event_tx` inside one SQLite transaction,
then returns the authoritative `RunWorkspace`; errors roll back.

## Verified

- `mygo native check modules/runs` → `Runs.dll` built
- `go test ./...` → Go loads the Rust DLL, calls `LoadRun` and
  `SaveReadout` against a **copy of a real AsterLab SQLite database**
- saved `Control-1 / OD450 / 0.82` is read back in the next `LoadRun`
- generated Go/Rust types, context parameter, error return and Close work

Test command:

```sh
cd prototypes/mygo-run
go test ./...
```

Runtime:

```powershell
$env:LABWORKBENCH_DATA_DIR="<repo>/target/asterlab-ui-runtime3"
$env:ASTERLAB_RUN_ID="1c0c8989-0513-483a-851b-bdc7d6c67483"
go run .
```

## Regenerating after a contract change

Run both, in order, whenever `modules/runs/runs.go` gains or changes a field:

```sh
go run github.com/egoist/mygo/cmd/mygo native check modules/runs
go run github.com/egoist/mygo/cmd/mygo native build -target windows-amd64 modules/runs
```

`check` regenerates the Go/Rust glue and builds the dev library; `build` refreshes the packaged `Runs.dll`
(`.mygo/native/windows-amd64`). A `go build` alone keeps the stale DLL, and the GUI then fails with
"a value from the implementation ends early".

## Still open

- UI schedule/flow/plate are still synthetic; only title/status and save use
  the real Core.
- CompleteTask is intentionally not added.
- Revision/out-of-order response guard is not implemented yet.
- Two-save concurrency is serialized by the module mutex; no UI request
  ordering test.
- Windows installer and independent DLL loading after install are not tested.
