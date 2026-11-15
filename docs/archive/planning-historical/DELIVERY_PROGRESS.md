# AsterLab Beta delivery progress

- Target: `v0.1.0-beta` Windows x64, local single-user, offline
- Baseline: `d2da5e9` plus current delivery changes
- Last updated: 2026-11-07

## Status

| Area | State | Evidence |
|---|---|---|
| Web-first decision | done | `docs/adr/0001-web-first-beta.md` |
| CLI Run authorization | done | trusted-writer gate + unauthorized acceptance |
| HTTP Run authorization | done | human-confirmed header + 403 smoke |
| Backup / restore | done | `/api/backup`, `/api/restore`, `tests/backup-restore.cjs` |
| Three-protocol execution acceptance | done | `tests/execution-acceptance.cjs`, 8/5/8 steps |
| Three-protocol browser E2E | done with seeded Protocol/Design | `tests/e2e-three-protocols.cjs` |
| Unified page shell | done | 4 primary rail tabs; Projects/Design/Records via Tools; `pages-smoke.cjs` |
| Design page | Workflow only | compact design summary in Workflow; no separate Design Map page |
| Protocol page | done (read) | selectors, step navigation, instruction-first, collapsed history |
| Run page | done (read + record form) | `view_id` identity, scope-correct events, in-page record form, execution-only |
| Browser design/Protocol authoring E2E | partial | Workflow is primary design; project/design/View linkage and inline edit still in progress |
| Common resources catalog | done (static) | instruments/tools/materials in Tools; not yet typed-bound to steps |
| RT-qPCR walkthrough | done | `docs/research/2026-11-07-rt-qpcr-walkthrough.md` |
| Real screenshots | done | `docs/release/screenshots/*.png` |
| Windows package | done | `release/AsterLab-0.1.0-beta-win-x64.zip` |
| Isolated package startup | done | no Cargo/Node; health + assets verified |
| CLI catalog coverage | done | `tests/cli-coverage.cjs`: 55/55 referenced by Node + Rust tests |
| Documentation/handoff | done | README, START_HERE, backup, limitations, acceptance |

## Evidence commands

```text
cargo test --release
node tests/execution-acceptance.cjs
node tests/backup-restore.cjs
node tests/smoke.cjs
node tests/e2e-three-protocols.cjs
node tests/cli-coverage.cjs
powershell -File scripts/package-beta.ps1
```
