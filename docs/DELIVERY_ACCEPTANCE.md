# AsterLab Beta acceptance matrix

A checked item requires executable evidence, not a static prototype.

| # | Gate | State | Evidence |
|---|---|---|---|
| 1 | Unzip and launch without Cargo/Node | PASS | package extracted; health + ASTERLAB assets |
| 2 | Create a complete design in the UI | PARTIAL | read-only Design Map now exists; editing still routes to the old design modal |
| 3 | Edit Protocol and inspect versions/instructions | PARTIAL | read-only Protocol page exists; editing still routes to old workspace |
| 4 | Operate Workflow components | PASS | `smoke.cjs` View/component UI |
| 5 | Human review and trusted Run creation | PASS | UI run form header + CLI trusted-writer + HTTP 403 negative |
| 6 | Record and reread raw/derived results | PASS | `execution-acceptance.cjs` + UI check/actual parameter |
| 7 | CCK-8, RT-qPCR, biofilm browser E2E | PASS | `e2e-three-protocols.cjs`, 3 runs, 3 checks, restart |
| 8 | Backup and restore after restart | PASS | `backup-restore.cjs` |
| 9 | Real 1440/1920 screenshots, both themes | PASS | `docs/release/screenshots/` |
| 10 | No unexplained failing test | PASS | `cli-coverage.cjs` now includes Rust integration-test calls; 55/55 covered |

## Honest scope note

The three browser E2E scenarios use synthetic Protocol/Design fixtures created through the
existing CLI, then human Run creation, UI recording, restart, and readback. UI authoring of
designs/Protocols is covered separately by `smoke.cjs`; it is not yet combined into the same
three-scenario script.

## Current test commands

```text
cargo test --release
node tests/smoke.cjs
node tests/execution-acceptance.cjs
node tests/backup-restore.cjs
node tests/e2e-three-protocols.cjs
node tests/cli-coverage.cjs
```
