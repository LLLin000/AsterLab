# Environment gate — D: volume

Measured 2026-11-12. This gate blocks parallel worktree builds and large E2E.

## 1. State

| Moment | D: free | D: used |
|---|---|---|
| Before cleanup | **0 B** (builds failed with `There is not enough space on the disk`) | 452 / 452 GB |
| After deleting three redundant `mimi` trees | **7.8 GB** | 444 / 452 GB |

C: has ~13 GB free and is not the build volume.

## 2. What was deleted (reviewed, one-off)

| Path | Size | Why safe |
|---|---|---|
| `D:/L/AI/mimi` | 7.0 GB | Not a git repo; 7.0 GB of 7.0 GB was `src-tauri` build output |
| `D:/L/AI/mimi-upstream` | 778 MB | Not a git repo; 631 MB `src-tauri` build output + 101 MB `node_modules` |
| `D:/L/AI/mimi-build` | 8.4 MB | A single prebuilt `mimi.exe` |

Kept: `D:/L/AI/mimi-src` (225 MB) because it holds the only `FINDINGS.md` and
`_repro/`; it is the smallest of the four and can be deleted on request.
Loose `D:/L/AI/mimi-*.md|mjs|png|patch` files were not touched (small, and some
are issue/PR drafts).

Nothing under `C:/Users/Lin/AppData/Local/AsterLab` was touched.

## 3. Protected paths (never delete, never migrate blindly)

- `C:/Users/Lin/AppData/Local/AsterLab` — live app data (29 KB, keep as-is).
- Repository source, uncommitted changes, `docs/`, `scripts/`, `spec/`.
- `docs/sdk/evidence/`, `prototypes/mygo-run/internal/*/evidence-*.png` — audit
  screenshots referenced by the SDK docs.
- Any signing material and released binaries.
- `target/acc-data`, `target/rc3-data` — disposable fixtures, but keep while
  RC-3 acceptance is being reviewed.

## 4. What refills D:

| Path | Size | Regenerable | Recommendation |
|---|---|---|---|
| `target/` (repo root, Rust + CLI) | 490 MB | yes | relocate with `CARGO_TARGET_DIR` |
| `prototypes/mygo-run/.mygo` | 258 MB | yes (slow: native module rebuild) | relocate if a second volume is available |
| `target/debug` | ~450 MB after a CLI build | yes | covered by the relocation above |

Deleting `target/` alone is not a fix: the next build recreates it on the same
full volume. The durable fix is to point build output at a volume with headroom.

### Relocation (no source change)

```
CARGO_TARGET_DIR=C:/Users/Lin/AppData/Local/asterlab-build/cargo-target
```

Set it per shell before `cargo`/`vsdev.cmd`. The existing `target/vsdev.cmd`
wrapper can export it so every Rust command inherits it. Go already builds with
`GOTMPDIR`/`TMP` on C:, but `go build -o target/...` still writes the binary to
D: by design (that is the shipped artifact, keep it).

## 5. Rules until free space is ≥ 20 GB

1. No two agents build at the same time on D:.
2. No full `cargo clean` / cache wipe as a space strategy.
3. No recursive delete outside `target/`, `*.tmp`, and explicitly named trees.
4. Every cleanup names the exact paths and reports the recovered size.
5. A build failure caused by disk is reported as an environment failure, not a
   test failure.

## 6. Open item

The full per-directory scan of `D:/L` is still running; when it lands, append the
top consumers here before choosing the next migration target. The three `mimi`
trees were the largest known consumer and are already removed.
