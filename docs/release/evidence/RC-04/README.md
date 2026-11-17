# RC-04 — Full Linux + Windows gates (release commit)

Release SHA: `d7d25e8e5587b633d79e00c0e073502f8e755dcb`
(`RC-02: upgrade/uninstall/reinstall DB retention evidence (#179)`, 2026-11-17;
`origin/main` at dispatch time; checkout pinned to this SHA for the whole run.)

Toolchain (Windows workstation):
- `go version go1.27.3 windows/amd64`
- `rustc 1.100.0 (0ef1bc386 2026-11-09)` / `cargo 1.100.0 (3acc79c0c 2026-11-05)`
- `gcc 15.2.0` (MSYS2, MyGo cgo)

Note: the `$env:` prefix for `CARGO_TARGET_DIR`/`GOCACHE`/`TEMP` did not apply
(bash tool, not PowerShell), so the build/tests used default cache locations.
Recorded here so the deviation is visible; results are unaffected.

## Linux gate — PASS

- Workflow: `linux-tests`, dispatch: `gh workflow run linux-tests --ref main`
  (raw-SHA `--ref` is rejected with HTTP 422; `main` equalled the release SHA,
  and the run's `headSha` confirms it tested `d7d25e8`).
- Run: https://github.com/LLLin000/AsterLab/actions/runs/44025076085
- `headSha: d7d25e8e5587b633d79e00c0e073502f8e755dcb`, conclusion `success`
  (job `linux-tests: success`, 2026-11-17 ~05:34–05:39 UTC).
- Workflow-level skip (mirrored in `windows-build.yml`, owned by QA-10):
  `TestAssetOpenFailureShowsPageBannerWithRetryAndLinkErrReachable`
  (`-skip` by name; hosted-only Go GC fatal, never reproduces locally).

## Windows gate — BLOCKED (one deterministic cargo failure, not fixed per scope)

### Build — PASS

`scripts/mygo-build.ps1` → `prototypes/mygo-run/dist/`:

- `BUILD_INFO.txt` commit `d7d25e8` == `git rev-parse --short HEAD` (match).
- MyGo pin `v0.8.4-0.20261108220316-c4197b7fe562` (UNSIGNED).
- `Runs.dll` SHA256 `3D0CCF62DD6DC973AC794612D76F88238DB4DACD109163CF9443A591B57D7F05`
- `mygo-run.exe` SHA256 `7A1F560D42B10BE778D676C3D32DF66329A8D27C121863ABC0B1781EB5B13BF7`
- Full build output tail in `logs/build-tail.txt`; BUILD_INFO copy in `build-info.txt`.

### Go tests — PASS (full suite, unskipped)

`go test -tags=e2e -count=1 ./...` in `prototypes/mygo-run` (the local-ci
Windows Gate command; a superset of plain `go test ./...`): **EXIT=0**,
26 packages `ok`, 509 `--- PASS`, 0 `--- FAIL`.
Full log: `logs/go-test-full-unskipped.log`; verbose log: `logs/go-test-verbose.log`.

Side effect (reverted, not committed): running the suite rewrote 22 tracked
screenshot PNGs under `prototypes/mygo-run/` and generated 3 untracked ones;
all were restored to pre-run state so this evidence contains only RC-04 files.

Both workflow-quarantined names were run explicitly and PASS locally:

- `TestAssetOpenFailureShowsPageBannerWithRetryAndLinkErrReachable` — PASS
- `TestRootsResolveAndRejectTraversal` (`internal/assets`) — PASS

### Cargo tests — 1 DETERMINISTIC FAILURE (BLOCK)

| crate | result |
|---|---|
| `labworkbench` (bin) | 67 passed, **1 failed** |
| `crates/asterlab-core` | 483 passed (472+3+6+2 across binaries), 0 failed |
| `crates/asterlab-app` (lib) | 77 passed, **1 failed** (same test) |

Failing test (identical in both binaries, fails solo too — deterministic,
reproduced 3/3): `protocol_version_queries_return_expansion_and_stable_not_found`
(from DES-06 #98, `src/cli.rs:206`):

```text
std::fs::remove_dir_all(dir).unwrap();
Os { code: 32, kind: Uncategorized, message: "另一个程序正在使用此文件，进程无法访问。" }
```

Root cause (read-only observation, not fixed — `src/**` is out of scope):
the test keeps the SQLite `db` connection open while calling
`remove_dir_all(dir)`. On Windows the open handle locks the files (error 32);
on Linux unlinking open files succeeds, so the Linux lane never sees it —
and the Windows lane never runs `cargo test` (local-ci.md: the Windows Gate
runs no Rust targets), so no lane catches it. Fix belongs to the owning
card (drop/close `db` before cleanup, or retry cleanup); RC-04 records it
as BLOCKED.

Logs: `logs/cargo-test-labworkbench.log`, `logs/cargo-test-core.log`,
`logs/cargo-test-app.log`.

## Skip / quarantine inventory (no silent skips)

Workflow-level `-skip` (both documented in `.github/workflows/*.yml`):
1. `TestAssetOpenFailureShowsPageBannerWithRetryAndLinkErrReachable` —
   owner QA-10 (`c40bf7e1-ad9f-4c1d-8573-aab98cdbb2b1`); skipped on both lanes,
   PASS when run locally (see above).
2. `TestRootsResolveAndRejectTraversal` — `windows-build.yml` only
   (comment: "needs its own Windows fix card"); covered by linux-tests;
   PASS when run locally on this workstation.

In-code conditional skips observed in this run (36, all by-design guards,
no owning card; full list with reasons in `logs/go-skip-inventory.txt`):
- 33× `fixture DB unavailable` — tests resolve a hardcoded developer-machine
  fixture path
  (`.../cellplanner-replica/target/asterlab-ui-runtime3/labworkbench.sqlite`)
  that does not exist on this workstation, so they skip. Affected:
  AsterDoc store/compile/command/create/freeze suites, contract integration
  (×4), concurrency cross-process (×5), CCK-8 loop, create-run, readout,
  creation-audit, snap, stale-revision, Rust-module SQLite parity,
  asset-link persistence, two-readouts e2e.
- `TestConcurrencyWorker` — "worker entrypoint only" (subprocess re-exec guard).
- `TestSeedGUIRun` — opt-in (`ASTERLAB_SEED_GUI=1`).
- `TestLB02Evidence` — opt-in (`LB02_EVIDENCE=1` regeneration).

Rust: no `#[ignore]` tests in `crates/` or `src/`.

## Verdict

- Linux gate: **PASS** (run 44025076085, `success` on release SHA).
- Windows build + Go suite: **PASS**.
- Windows cargo suite: **BLOCKED** — deterministic Windows-only failure
  `protocol_version_queries_return_expansion_and_stable_not_found`
  (open-DB-handle vs `remove_dir_all`, error 32). No code touched; needs an
  owning fix card. RC-04 cannot close until that is fixed and re-gated.
