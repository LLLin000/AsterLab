# QA-10 — Hosted-only GC fatal in `TestAssetOpenFailureShowsPageBannerWithRetryAndLinkErrReachable`

- Task card: `QA-10` (`c40bf7e1-ad9f-4d1d-8573-a64cdbb2b1`), Windows Gate lane.
- Test: `prototypes/mygo-run/main_test.go:1829`
  (`TestAssetOpenFailureShowsPageBannerWithRetryAndLinkErrReachable`).
- Symptom (hosted only): the test binary dies with a Go runtime GC fatal —
  `found pointer to free object` / `bad pointer in Go heap` — inside the
  pinned MyGo Tester's render chain (`ui.NewTester` / `Frame` / `Click`),
  killing the whole `go test` binary instead of failing the test.
  MyGo pin: `v0.8.4-0.20261108220316-c4197b7fe562` (do NOT upgrade on this card).

## Repro matrix (local Windows, checkout at `a31e04c`, `GOMAXPROCS=2`)

All runs from `prototypes/mygo-run` with
`CARGO_TARGET_DIR=D:/.ci-cache/cargo-target GOCACHE=D:/.ci-cache/go
TEMP=D:/.ci-cache/tmp TMP=D:/.ci-cache/tmp`:

| # | Command | Result |
|---|---------|--------|
| 1 | `go test -run TestAssetOpenFailureShowsPageBannerWithRetryAndLinkErrReachable -count=1 -v .` | PASS (0.05 s) |
| 2 | `go test -run TestAssetOpenFailure… -count=20 .` | ok (no crash) |
| 3 | `GOGC=10 go test -run TestAssetOpenFailure… -count=20 -v .` | all 20 PASS |
| 4 | `GOGC=5 go test -run TestAssetOpenFailure… -count=30 .` | ok (no crash) |
| 5 | `GOGC=1 go test -run TestAssetOpenFailure… -count=5 .` | ok (extreme GC pressure, no crash) |
| 6 | `go test -tags=e2e -run TestAssetOpenFailure… -count=10 .` (exact CI tag shape) | ok (no crash) |
| 7 | `GOGC=5 go test -count=3 -run '<all 32 main_test.go tests>' .` (order/state interaction) | ok (no crash) |
| 8 | `go test -count=1 .` (full root package) | FAIL only on pre-existing missing native build (`ffi: Runs has no build for windows-amd64` — fresh checkout without `.mygo/native`, unrelated to this card); no GC fatal |

Conclusion: **does not reproduce locally** under repetition, GC pressure
(`GOGC=1/5/10`), the CI `-tags=e2e` shape, or suite-order interaction.
No deterministic trigger exists, so there is no test-side fix that
preserves assertion semantics (no teardown/GC-pacing/serialization change
in `main_test.go` — the crash is inside pinned MyGo/pinned-dep code, and
`main.go` is out of scope).

## Quarantine (this change)

- `.github/workflows/windows-build.yml`: extended the existing
  `-skip '^TestRootsResolveAndRejectTraversal$'` (REL-06 precedent) with
  `|^TestAssetOpenFailureShowsPageBannerWithRetryAndLinkErrReachable$`,
  plus a QA-10 comment block in the established format.
- `.github/workflows/linux-tests.yml`: added
  `-skip '^TestAssetOpenFailureShowsPageBannerWithRetryAndLinkErrReachable$'`
  with a matching QA-10 comment block (this file previously had no skip
  mechanism; style mirrors windows-build.yml).
- What stays covered: the page-banner / `重试` / `linkErr` assertions still
  run in the local gate (`scripts/ci/local-gate.sh` runs the unskipped
  `go test ./...` suite) and on demand via `-run TestAssetOpenFailure`.
- Real fix (out of scope here): MyGo upgrade past the GC bug, then drop both
  `-skip` alternatives together with that fix. QA-10 owns it.

## Hosted-CI note (2026-11-17 scan, last 100 runs per workflow)

- `linux-tests` run `43994675165` failed on this test, but positionally
  (`ui: "打开" is at (896, 884), outside the window`) — the separate
  layout issue already fixed via the 1360x1080 window height; not the GC fatal.
- `windows-build` runs `43994043561` / `43992797094` failed on
  `TestArchivePDF*` (unrelated, not this card's scope).
- `43893039768` (win) / `43893039770` (linux) show a nil-pointer
  panic after `TestApplyRunLoadErrorUpdatesRunWorkspaceAndRetries` — a
  different signature from this card's GC fatal, noted here only to avoid
  confusion.
- No fresh GC-fatal instance surfaced in the scan window, consistent with a
  rare nondeterministic hosted-only crash. The fatal itself is coordinator-
  verified ground truth for this card and was not re-proven.
