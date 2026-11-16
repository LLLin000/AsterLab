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

## Bridge v2 generic command/query channel (INT-01, additive)

Status: additive section, 2026-11-16. All sections above stay frozen.
New BE-08/09 capabilities cross this boundary as data (`kind` strings +
JSON payloads), never as new C ABI symbols. The existing typed methods in
`prototypes/mygo-run/modules/runs/runs.go` (`RunService`, `DesignService`)
remain backward compatible: adding a `kind` never renames, removes, or
re-signatures a typed method.

### ExecuteCommand

Single write entry point. The Go side passes one envelope; Core dispatches
on `kind` to the same implementation the CLI tool of that name uses, in one
transaction, with the same validation.

```json
{
  "kind": "execution_event.create",
  "idempotency_key": "<one per user action, never per retry>",
  "expected_revision": 3,
  "payload_json": "<exact JSON text the CLI --input file would carry>"
}
```

| Field | Rule |
|---|---|
| `kind` | Command name, e.g. `execution_event.create`. Unknown `kind` is rejected with `INVALID_INPUT`; no new symbol is minted for it. |
| `idempotency_key` | Required. Same key + same payload replays the first result (`replayed: true`, no second write); same key + different payload is `IDEMPOTENCY_MISMATCH` and writes nothing. Mirrors the `OperationID` rule in `RunService` above. |
| `expected_revision` | Required exactly when the underlying command revises a versioned record (`design.revise`, `run.revise`, `step.edit`, `asterdoc.command`, `pantry.upsert`, `pantry.delete`). Omit for append-only commands (`execution_event.create`, `readout.submit`, `readout.batch`, `note.create`). Stale revision returns `REVISION_CONFLICT` / `VERSION_CONFLICT` with the current revision; the caller re-reads and retries, never force-writes. |
| `payload_json` | Exact CLI input JSON, decoded by Core, never rebuilt in Go. Unknown fields are rejected; the caller never invents defaults. |

There is no dry-run flag on writes. Preview before sending with the
matching read path (`CompileAsterDoc` / `AssessAsterDoc` for design edits,
`WorkflowTree` / `workflow.progress` for Run state).

### QueryProjection

Single read entry point. Side-effect-free; it never writes records, never
creates timers, never invents defaults.

```json
{ "kind": "run.workspace", "params_json": "{\"id\":\"<run-id>\",\"view\":\"blind\"}" }
```

| Field | Rule |
|---|---|
| `kind` | Query name, e.g. `run.workspace`. Unknown `kind` is `INVALID_INPUT`. |
| `params_json` | Exact CLI input JSON for the matching read command, decoded by Core, never rebuilt in Go. |

The result is the CLI result JSON verbatim (same shape as `--json` stdout).
Go decodes it; it never re-derives numbers, states, or groupings from it.

### BE-08/BE-09 frozen exposure list

These capabilities are exposed through the generic channel with the exact
semantics of `AGENT_API.md`; the table cites the authoritative section for
each. Field names below are the CLI/JSON names; Go transliterations (if any)
keep the same names and meanings.

| # | Channel call | Frozen semantics |
|---|---|---|
| 1 | `QueryProjection` `run.workspace` | Input `{id, view?}`. `view` omitted or `unblinded` keeps frozen group identity; `view=blind` clears group id/name/role/condition, group sample counts, well group tags, and substitutes identity strings in free text — and only when the frozen design declares a non-empty `blinding` basis, otherwise `INVALID_INPUT`. Missing frozen snapshot is an error, never a fallback to the current Design. |
| 2 | `ExecuteCommand` `execution_event.create` | Append-only actuals: `event_type` `operation` / `material_use` / `deviation` / `observation` / `skipped_by_rule`. `step_status` is exactly `in_progress` / `completed` / `blocked` / `skipped` / `qc_dropped` / `censored_dead` / `rework`; `qc_dropped` / `censored_dead` count in `workflow.progress` but never count as step completion. Sample lineage travels in `parent_sample_id` / `aliquot_from` (text, ≤120 chars); blinded writes set `blind_masked: true`. `pantry_item_id` links a ledger row: unknown/retired is `NOT_FOUND`, near-expiry/expired only adds an `advisory` notice and never blocks. `attempt` (≥1) requires step path + samples and must be the current max or max+1; same step+sample+attempt replays. Unknown step paths are rejected unless the input sets `ad_hoc_step: true`, which still does not bypass sample/attempt/check validation. |
| 3 | `ExecuteCommand` `readout.submit` (multi-value) | `readouts[]` items carry `value` of `numeric` / `table` / `plate` / `categorical` / `file` per `docs/contracts/readout-schema-v1.md`; `readout_schemas[]` declares call-scoped schema definitions, checked against the registry (built-ins `cck8_od450`, `rna_concentration`, `rna_purity`, `cfu_count` always available) and never persisted. Non-numeric readouts never enter the numeric fact table and are never projected as raw `readouts[]`. |
| 4 | `ExecuteCommand` `readout.batch` + `well.set` 1D positions | Batch is `cells[]` or `csv`, never both; one transaction, one `measurement` event, any row failure rejects the batch with `INVALID_INPUT` + `issues[]`. The target plate must resolve to exactly one plate of this Run (`unresolved_plate` / `ambiguous_plate` / `unknown_plate` otherwise — no guessing). 1D single-row containers accept `Lane<N>` / `L<N>` / `Tube<N>` ≡ `A<N>`, stored as `Lane<N>`; 2D plates accept `<row><col>`. `target` / `well_role` / `dilution_factor` are plan annotations, not facts. |
| 5 | Cross-Design sample lineage | `run.create` accepts `source_run_id`: the upstream harvest Run may belong to a **different** Design in the same project (same-project check only; cross-project is `INVALID_INPUT`, unknown is `NOT_FOUND`). It is independent of `repeat_of_run_id`, which stays the same-Design re-execution relation. Per-event links (`parent_sample_id` / `aliquot_from`) are row 2. |
| 6 | `QueryProjection` `workflow.progress` (+ `workflow.inspect_tree`) | Progress groups by actual event `(step_path, timepoint)` only; no Run-global sample × timepoint cartesian product. Sample states are exactly `not_started` / `active` / `in_progress` / `completed` / `failed` / `skipped` / `skipped_by_rule` / `qc_dropped` / `censored_dead` / `rework`. Identity and hierarchy come from `workflow.inspect_tree`; classic-design Runs without an AsterDoc revision return empty roots with an explanatory issue, not `NOT_FOUND`. |

Existing typed methods covering the same ground (`SaveReadout`,
`SaveReadoutsBatch`, `CreateExecutionEvent`, `RecordProjection`,
`WorkflowTree`, `GetProtocolVersion`, `ListProtocolVersions`,
`PreviewProtocolBlock`) keep working unchanged; new callers prefer the
generic channel so no new per-field symbol is needed.

### Error mapping (Core → envelope → Go → UI)

The envelope error is `{ "code": "<stable Core code>", "message": "<human
text>", "current_revision?": <n>, "issues?": [...] }`. Codes are stable
machine tokens; `message` is display-only.

| Core `error.code` | Envelope | Go | UI rule |
|---|---|---|---|
| `INVALID_INPUT` (exit 2), incl. `STEP_REFERENCED`, `INVALID_PROPOSAL`, `INVALID_BINDING`, `UNIT_MISMATCH`, `FORBIDDEN_FIELD`, `CONFIRMATION_REQUIRED`, `CONFIRMATION_MISMATCH`, plate/readout `issues[]` codes (`missing_step`, `unresolved_plate`, …) | `ok: false`, code verbatim, `issues[]` verbatim | `ErrInvalid` (`INVALID: …`) | Show message + offending path; keep draft; offer retry. Never auto-fix the payload. |
| `NOT_FOUND` (exit 3) | `ok: false`, code verbatim | `ErrNotFound` (`NOT_FOUND: …`) | Show "object no longer exists"; reload, never synthesize an empty object. |
| `REVISION_CONFLICT` / `VERSION_CONFLICT` / `STALE_REFERENCE` (exit 4) | `ok: false`, code verbatim, `current_revision` | `ErrConflict` with current revision | Re-read the projection, rebase the draft onto it, ask for confirmation; never overwrite. |
| `IDEMPOTENCY_MISMATCH` | `ok: false`, code verbatim | `ErrConflict` | Same key + different payload means a real user-level conflict: surface it, mint a new key only for a new action. |
| Internal / SQLite / io (exit 5) | `ok: false`, code `INTERNAL` | `ErrInternal` (`INTERNAL: …`) | Preserve draft, show cause + retry. Never turn a failure into success. |

Prohibitions (binding on UI):

- Branch on `error.code` / diagnostic `code` / `severity` only; never parse `message` text.
- Render Core state enums verbatim (`step_status`, progress states, `ValidationIssue` `blocking` / `needs_confirmation` / `advisory`, `status` `raw` / `missing`). UI never derives completion, n, blinding, or expiry from raw fields — `record_projection`, `workflow.progress`, and `pantry` notices already carry the decided values.
- `unknown` (not recorded) vs `not_applicable` vs explicit values stay distinct; empty input is never coerced.
