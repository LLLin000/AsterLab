# Worktree classification (post RC commits)

- Date: 2026-11-11
- State: `bffb8ae` pushed. 72 worktree entries remain uncommitted by design; nothing was deleted or overwritten.
- Purpose: keep release acceptance from accidentally including generated or unrelated files.

| Category | Count | Examples | Handling |
| --- | ---: | --- | --- |
| Screenshots/evidence images | 56 | `internal/runui/evidence-*.png`, `internal/shell/evidence-*.png`, `docs/prototypes/.../*.webp` | Review against the release evidence set; commit only the intended final captures. Do not bulk-delete. |
| Dependency/lock metadata | 5 | root `Cargo.lock`, `crates/asterlab-app/Cargo.lock`, `prototypes/mygo-run/go.mod`, `go.sum` | Audit with the next dependency change; not part of the SDK contract commit. |
| Unrelated docs/archive | 5 | `docs/AsterLab_Overnight_...`, `docs/asterdoc-phase0-audit.md`, `docs/design/v1.2/...` | Keep as research/archive; decide separately whether to publish. |
| Generated/build artifacts | 5 | `crates/asterlab-core/target/`, `prototypes/mygo-run/target/`, `dist2/`, `*.exe~` | Do not commit. Clean only with explicit approval. |
| Visual design doc | 1 | `DESIGN.md` | Prior visual-system work; not part of SDK RC. |

## Release rule

The RC executable must be built from committed source plus the committed `spec/modules.json` and `spec/foundation-v1.schema.json`. Uncommitted images, build caches and lockfile changes must not be used as evidence that the committed source passes acceptance.

## Current evidence committed

- `docs/sdk/evidence/cck8-900x600-light.png`
- `docs/sdk/evidence/cck8-900x600-dark-150pct.png`
- `docs/sdk/evidence/cck8-run-1080x640.png`

Scale-200 and 100% real-window captures remain a release-quality gap; existing `runui`/`shell` scale-200 PNGs are headless evidence, not real-window proof.
