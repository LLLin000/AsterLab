# Rust Native Module spike

- MyGo pseudo-version: `v0.8.4-0.20261108220316-c4197b7fe562`
- Note: `v0.8.3` has no `mygo native` command; the native-module CLI and
  generated Rust bindings are on this newer revision.
- Module: `modules/runs`
- Generator: `go run github.com/egoist/mygo/cmd/mygo@v0.8.4-0.20261108220316-c4197b7fe562 native new -lang rust runs`
- Check: `mygo native check modules/runs` → built `Runs.dll`
- Persistence test: `go test ./...`

## Interface

```go
type RunService interface {
    LoadRun(ctx context.Context, runID string) (RunSnapshot, error)
    SaveReadout(ctx context.Context, input ReadoutInput) (RunSnapshot, error)
}
```

Generated Rust: `trait RunService`, `RunSnapshot`, `RunTask`,
`Measurement`, `ReadoutInput`; factory `make_run_service`.

Implementation: `modules/runs/rust/src/lib.rs`. It is a **file-backed
adapter** that writes a temp JSON and renames it atomically. It proves the
Go↔Rust typed call, error return, context parameter and state across calls.

The original AsterLab SQLite/Core is **not connected yet**; a Rust module
can depend on the core crate once the Run command layer is extracted into a
library. This spike deliberately does not bypass domain validation.

## Test result

`go test ./...` passes:

- construct `RunService` from Go
- `LoadRun` returns the seeded snapshot
- `SaveReadout` appends a measurement
- a second `LoadRun` sees the persisted value

The window prototype uses the generated service for title/status and keeps
its schedule/flow/plate synthetic.

## Next

1. Extract `RunWorkspace` + save/complete commands from `src/cli.rs` into a
   library crate.
2. Add it as a path dependency of the Rust module.
3. Replace the file-backed adapter with SQLite transactions.
4. Test Windows packaging: DLL beside the app, restart, and the same Run.
