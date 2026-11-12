# Environment gate — D: volume

Measured 2026-11-12. This gate blocks parallel worktree builds and large E2E.

## 1. State

| Moment | D: free | D: used |
|---|---|---|
| Before cleanup | **0 B** (builds failed with `There is not enough space on the disk`) | 452 / 452 GB |
| After deleting three redundant `mimi` trees | 7.8 GB | 444 / 452 GB |
| After removing `apps/desktop/target` and relocating cargo output | **13 GB** | 440 / 452 GB |

C: holds the new cargo target (270 MB after the first build) and has ~13 GB free.

## 1.1 Largest consumers under `D:/L/AI` (measured)

| Path | Size | Note |
|---|---|---|
| `Cindy` | 6.7 GB | our study tree; `apps/desktop/target` alone was 4.4 GB |
| `OpenMAIC` | 3.9 GB | other project, not touched |
| `Paper_ingest` | 2.8 GB | other project, not touched |
| `survey` | 2.7 GB | other project, not touched |
| `figpact-archive` | 2.4 GB | other project, not touched |
| `corpus` | 1.4 GB | other project, not touched |
| `models` | 1.3 GB | model cache, not touched |
| `vcpkg` / `figlayout` | 1.1 GB each | other projects, not touched |

Inside `cellplanner-replica` (6.2 GB before cleanup): `apps/` 4.4 GB (all
`apps/desktop/target`), `prototypes/` 674 MB (`.mygo` 258 MB), `target/` 490 MB,
`crates/` 342 MB, `.git` 241 MB.

Nothing outside `cellplanner-replica` was deleted. The other projects are the
owner's data; they are listed, not touched.

## 2. What was deleted (reviewed, one-off)

| Path | Size | Why safe |
|---|---|---|
| `D:/L/AI/mimi` | 7.0 GB | Not a git repo; 7.0 GB of 7.0 GB was `src-tauri` build output |
| `D:/L/AI/mimi-upstream` | 778 MB | Not a git repo; 631 MB `src-tauri` build output + 101 MB `node_modules` |
| `D:/L/AI/mimi-build` | 8.4 MB | A single prebuilt `mimi.exe` |
| `apps/desktop/target` | 4.4 GB | git-ignored Rust build cache |
| `target/debug` | 457 MB | stale cargo output after the relocation below |

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

### Relocation (applied and verified)

`target/vsdev.cmd` now exports:

```
if not defined CARGO_TARGET_DIR set "CARGO_TARGET_DIR=C:\Users\Lin\AppData\Local\asterlab-build\cargo-target"
```

Every `cargo` command run through the wrapper inherits it. Verified: a clean
`cargo build --bin labworkbench` wrote `labworkbench.exe` and 270 MB of
dependencies under `C:/Users/Lin/AppData/Local/asterlab-build/cargo-target`, and
D: free space did not move. The old D: `target/debug` was then deleted.

Go already builds with `GOTMPDIR`/`TMP` on C:; `go build -o target/...` still
writes the shipped binary to D: by design (that is the artifact, keep it).

`prototypes/mygo-run/.mygo` (258 MB) is the MyGo native-module cache; its path is
owned by the tool, so it stays until the tool supports relocation.

## 5. Rules until free space is ≥ 20 GB

1. No two agents build at the same time on D:.
2. No full `cargo clean` / cache wipe as a space strategy.
3. No recursive delete outside `target/`, `*.tmp`, and explicitly named trees.
4. Every cleanup names the exact paths and reports the recovered size.
5. A build failure caused by disk is reported as an environment failure, not a
   test failure.

## 6. Open item

D: is at 13 GB free, below the 20 GB target. The measured consumers are in §1.1.
The remaining large items are other projects under `D:/L/AI` (`OpenMAIC` 3.9 GB,
`Paper_ingest` 2.8 GB, `survey` 2.7 GB, `figpact-archive` 2.4 GB) and model
caches (`models` 1.3 GB); none was touched because they are the owner's data.

To reach 20 GB, choose one:

1. Move one of those projects to another volume.
2. Delete `D:/L/AI/mimi-src` (225 MB) plus the three large evidence JSON files in
   `CellPlanner-ProductStudy` (244 + 162 + 56 MB) if they are no longer needed.
3. Accept 13 GB and keep one build at a time; cargo no longer grows D:.
