# MyGo Desktop v1 — Frozen Contract

Status: frozen for parallel implementation, 2026-11-09.
Baseline: `2344ef2`; verify actual HEAD before branching.
MyGo pin: `v0.8.4-0.20261108220316-c4197b7fe562` (v0.8.3 has no `mygo native`).

## Authority

```
MyGo Native UI (Go)       presentation + transient edit draft only
MyGo Rust Native Module   typed adapter, no business rules
asterlab-app (Rust)       application commands, validation, transactions
src/cli.rs                same implementation used by the existing CLI
SQLite                    durable records / event log
```

- Rust Core is the only authority for validation, revision, idempotency and
  transaction commit.
- Go may own UI draft (`WorkflowDraft`, selected node, unsaved form values),
  never a second copy of business rules.
- CLI and MyGo must call the same Rust functions.
- No `os/exec` for normal business calls.
- No direct SQLite writes from Go.

## Go ↔ Rust services

The generated native module lives in `prototypes/mygo-run/modules/asterlab`
(one module, multiple entry-point interfaces). Keep generated files untouched.

### RunService

```go
//mygo:native
type RunService interface {
    LoadRun(ctx context.Context, runID string) (RunSnapshot, error)
    SaveReadout(ctx context.Context, input ReadoutInput) (RunSnapshot, error)
    CompleteTask(ctx context.Context, input CompleteTaskInput) (RunSnapshot, error)
    Close() error
}
```

`RunSnapshot` must include:

```go
RunID, DesignID string
DesignRevision, Revision uint64
Title, Status string
Tasks []RunTask
Measurements []Measurement
Events []RunEvent
```

`ReadoutInput` and `CompleteTaskInput` include `OperationID string`.
The same operation ID with the same payload is idempotent; the same ID with
different payload is a conflict. UI sends one operation ID per user action
(not per retry); Core stores it in the transaction.

Snapshot application rule in Go:

```go
if incoming.Revision < current.Revision { ignore }
if incoming.Revision == current.Revision && incoming != current { conflict / reload }
```

### DesignService

```go
//mygo:native
type DesignService interface {
    ListDesigns(ctx context.Context) ([]DesignSummary, error)
    LoadDesign(ctx context.Context, designID string) (DesignSnapshot, error)
    SaveDesign(ctx context.Context, input DesignSaveInput) (DesignSnapshot, error)
    CreateRun(ctx context.Context, designID string, operationID string) (RunSnapshot, error)
    Close() error
}
```

A design save carries `ExpectedRevision` (optimistic concurrency) and
`OperationID`; the Core rejects stale revisions. Design and Run revisions are
independent.

## Core schema (reuse existing records)

Do not introduce a competing experiment model. Reuse existing:
`design`, `design_version`, `run`, `execution_event`, `task`, `protocol*`,
`plate`, `well`, `note`, `formula`, `library_table`.

New fields required by this contract:

- `run_workspace.revision`: monotonic count of execution events for the Run
  (current implementation), returned by `AppCore::load_run`.
- `design revision`: existing record revision.
- operation idempotency: stored with the event/operation; same ID + same
  payload returns the existing result; different payload returns conflict.
- layout coordinates for workflow nodes are UI data, stored separately from
  business dependencies; they never alter Protocol order.

## Errors

Rust returns typed errors; Go maps:

| Core | Go |
|---|---|
| validation | `ErrInvalid` with message |
| stale revision | `ErrConflict` with current revision |
| duplicate operation ID + different payload | `ErrConflict` |
| not found | `ErrNotFound` |
| SQLite/io | `ErrInternal` |

UI must preserve the draft and show the message; never turn a failure into
success.

## File ownership for parallel work

| Workstream | Owns |
|---|---|
| Core | `crates/asterlab-app/**`, minimal `src/cli.rs`, core tests |
| Bridge | `prototypes/mygo-run/modules/asterlab/**`, Go adapter `internal/state/**` |
| Run UI | `prototypes/mygo-run/internal/runui/**` |
| Workflow | `prototypes/mygo-run/internal/workflow/**` |
| Blocks | `prototypes/mygo-run/internal/blocks/**` |
| Release | `scripts/mygo-*`, `prototypes/mygo-run/e2e/**`, packaging |
| Orchestrator | `prototypes/mygo-run/main.go`, `internal/shell/**`, final integration |

No two writers edit the same file. `main.go` is orchestrator-only.
