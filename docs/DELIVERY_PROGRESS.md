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
| Browser design/Protocol authoring E2E | partial | existing smoke creates designs; protocol authoring still seeded |
| Real screenshots | done | `docs/release/screenshots/*.png` |
| Windows package | done | `release/AsterLab-0.1.0-beta-win-x64.zip` |
| Isolated package startup | done | no Cargo/Node; health + assets verified |
| CLI catalog coverage | open | `tests/cli-coverage.cjs`, 14 catalogued tools not yet exercised |
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
