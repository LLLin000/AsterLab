# AsterLab Beta acceptance matrix

A checked item requires executable evidence, not a static prototype.

| # | Gate | State | Evidence |
|---|---|---|---|
| 1 | Unzip and launch without Cargo/Node | PASS | package extracted; health + ASTERLAB assets |
| 2 | Create a complete design in the UI | PARTIAL | design form smoke passes; three E2E designs were seeded via CLI |
| 3 | Edit Protocol and inspect versions/instructions | PARTIAL | Protocol/View smoke and Execution brief; no full three-protocol UI authoring run |
| 4 | Operate Workflow components | PASS | `smoke.cjs` View/component UI |
| 5 | Human review and trusted Run creation | PASS | UI run form header + CLI trusted-writer + HTTP 403 negative |
| 6 | Record and reread raw/derived results | PASS | `execution-acceptance.cjs` + UI check/actual parameter |
| 7 | CCK-8, RT-qPCR, biofilm browser E2E | PASS | `e2e-three-protocols.cjs`, 3 runs, 3 checks, restart |
| 8 | Backup and restore after restart | PASS | `backup-restore.cjs` |
| 9 | Real 1440/1920 screenshots, both themes | PASS | `docs/release/screenshots/` |
| 10 | No unexplained failing test | OPEN | CLI catalog coverage has 13 unexercised tools |

## Honest scope note

The three browser E2E scenarios use synthetic Protocol/Design fixtures created through the
existing CLI, then human Run creation, UI recording, restart, and readback. They do not claim
that all three Designs and Protocols were authored end-to-end in the UI. That is the main
remaining UI-authoring gate.

## Current test commands

```text
cargo test --release
node tests/smoke.cjs
node tests/execution-acceptance.cjs
node tests/backup-restore.cjs
node tests/e2e-three-protocols.cjs
node tests/cli-coverage.cjs   # open gap
```
