# MyGo Desktop v1 — Workstreams

## Orchestration

- Freeze this contract before spawning writers.
- One `ovnfs` checkout per workstream; never let two writers touch one file.
- Orchestrator owns `prototypes/mygo-run/main.go`, `internal/shell/**`, final
  integration and review.
- Each writer builds/tests its package independently; no mid-flight build of
  the whole app is required.
- Merge order: Core → Bridge → Shell/Run/Workflow/Blocks → Release.
- A failed contract check or duplicate business logic is rejected, not
  patched around.

## Workstreams

| Agent | Checkout | Owns | Definition of done |
|---|---|---|---|
| A Core | `core-v1` | `crates/asterlab-app/**`, minimal `src/cli.rs`, core tests | revision, idempotency, design CRUD, rollback, CLI tests pass |
| B Bridge | `bridge-v1` | `modules/asterlab/**`, `internal/state/**` | generated module, typed errors, stale response guard, fake service |
| C Run UI | `runui-v1` | `internal/runui/**` | real snapshot, save/complete states, keyboard, view tests |
| D Workflow | `workflow-v1` | `internal/workflow/**` | SVG nodes/edges, pan/zoom, drag/connect, undo/redo, selection |
| E Blocks | `blocks-v1` | `internal/blocks/**` | 7 typed blocks, plate selection, validation, serialization |
| F Release | `release-v1` | `e2e/**`, packaging scripts | Windows E2E, DLL dist, restart, performance report |

## Phase 0

Contract freeze (this file) and baseline inspection. No writer starts on
`main.go`.

## Phase 1

A, C, D, E, F run in parallel ovnfs checkouts. B starts from the frozen
interface and can run alongside A; B must not copy Core validation.

## Phase 2

Integration in order. Each merge requires:

- package build pass
- package tests pass
- no business logic in Go
- no duplicate store/validation
- no edits outside ownership
- a screenshot or headless view test

## Phase 3

Acceptance scenarios:

- CCK-8 design and Run
- two-step RT-qPCR
- CFU/Biofilm
- two readouts + quit/reopen
- CLI cross-read
- invalid transaction rollback
- duplicate operation ID
- stale async response
- clean Windows install/DLL run

## Phase 4

Screenshots at 100/150/200%, Dark; record PASS/FAIL/NOT TESTED and evidence.
Rune remains the fallback until this passes.
