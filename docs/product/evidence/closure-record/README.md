# CLOSE-04 Evidence — Record closure (third leg)

Checkout `O:/close04`, branch `agent/close04-closure` (to be pushed), base
`origin/main` @ `2f2ce20`. Writes stay inside the card scope
(`prototypes/mygo-run/notebook_page.go`, `protocol_page.go`,
`design_page.go`, `main.go`, `main_test.go`, `notebook_page_test.go`,
`protocol_page_test.go`, `close04_record_test.go`,
`internal/notebook/`, `internal/protocol/`, `internal/runworkspace/`,
`internal/asterdoc/pdf.go`, this dir).

Prior legs in this checkout: package layer rewritten
(record/protocol/runworkspace split, package tests green) and pages rewired
(non-test tree compiles). This leg: page-test rewrite, real-Core integration
test, evidence. No rebase, no merge; PR opened via `engctl pr open` for
coordinator Gate/merge.

## Build, seed, capture

- GUI: `go build -tags e2e -o dist/mygo-run.exe .` (the `e2e` tag links the
  `mygo-automation` listener; `dist/` is gitignored). No Rust changes; the
  native Core library is the one the test suite already loads.
- Seed: copied the committed fixture
  `prototypes/mygo-run/testdata/rna-case/labworkbench.sqlite` into isolated
  `D:/.ci-cache/close04-data`; `LABWORKBENCH_DATA_DIR` pointed there, so the
  session could not touch the fixture or any user tree.
- Run: `ASTERLAB_RUN_ID=357c2f94-d5bf-4002-951e-f84c73e2d0b7` (PureLink RNA
  run, corrected design, rev 3). Theme via `ASTERLAB_THEME=dark` for the dark
  set, unset for light.
- Capture: real Windows windows via `mygo-automation`; outer size set with
  `user32.MoveWindow` to 1360×880 / 900×600. Shots are device pixels
  (≈1.48×: 2018×1308 / 1328×888); the layout breakpoints (≥1360 / <1100)
  are what the code branches on, so verdicts hold at the right breakpoints.
  Capture scripts live outside the repo (`D:/.ci-cache/`).

## Item verdicts

| # | Item | Verdict |
|---|---|---|
| 1 | Record packages own their layer; pages project from Core, never from the snapshot | DONE. `record.Fetch` is the single decode of Core's `RecordProjection` JSON; `notebook_page.go` (`fetchRecord` memo, `recordProjection`, `recordSealed`, `recordArchived`, `invalidateRecord`), the board, and the protocol archive all read that memo. Package tests pass: `go test ./internal/notebook/... ./internal/protocol/... ./internal/runworkspace/...` |
| 2 | Board/summary/matrix/refs/derived/sign read Core types | DONE. Summary from `record.Summarize` + `SnapshotTasks`; matrix from `MatrixCells`/`SlotImage`/`RepresentativeGroup`; refs resolved by `ResolveMatrixRef` and flattened by `ExpandMatrixRefs` at export; derived draft reviewed (`DecodeDerived`) then persisted (`ChangesForDerived` → `SaveDesign` → `loadDesign`). Sign is one Core command behind an explicit count-naming confirmation (`SignAndArchiveRun` → `InvalidateRecord` → reading mode); sealed state is Core's `Archive.Status`, never prose. |
| 3 | Page tests against the new APIs | DONE. `notebook_page_test.go` (seal stub serves a canned `record.Projection`; sign flow asserts two-click confirm, `signed` Core state + signature, no unlock, sealed save refusal; sealed-open asserts reading mode + lock notice), `protocol_page_test.go` (sealed archive shows `已签核归档` + signer), `main_test.go` (save-fail spy stubs `RecordProjection` as out-of-scope). `go test .` passes. |
| 4 | Real-Core integration: Fetch decode + legacy derive persist | DONE. `close04_record_test.go`: Fetch of `357c2f94…` decodes (params 16, readings 8, `recording`); derive of the legacy RNA run `ee49d439…` yields reviewable evidence (params 16, srcRev 2); persisting is refused naming AsterDoc — the exact refusal the board surfaces. Temp-dir fixture copy; scratch probes deleted. |
| 5 | Real-window screenshots 1360×880 + 900×600, light/dark | DONE. Board: [1360 light](close04-board-1360x880-light.png), [1360 dark](close04-board-1360x880-dark.png), [900 light](close04-board-900x600-light.png), [900 dark](close04-board-900x600-dark.png). Archive: [1360 light](close04-archive-1360x880-light.png), [1360 dark](close04-archive-1360x880-dark.png), [900 light](close04-archive-900x600-light.png), [900 dark](close04-archive-900x600-dark.png). Board shows `记录图板 · 未签核` + `任务完成 10/10 · 实测 8 个 · … · 排除 16 个` from live Core data; archive shows `终态归档 · 未签核` + `实验记录归档` with Run/design/state and `导出归档包` / `导出 PDF`. |

Contract notes: one shared read model across board/archive/history (UX-STATE);
states use the §3 words (`未签核`, `已签核归档`) with explicit counts, and the
AsterDoc refusal surfaces cause-first (UX-P5). The archive names the Run by id
for traceability (pre-existing export format, out of scope for P2).

## Honest caveats

- The fixture has no image assets, so the board's Well×Timepoint matrix
  renders gaps on every real window. Preview decode (incl. TIFF) is covered
  by unit tests with synthetic assets, not by these shots.
- No signed run exists in the fixture and signing writes Core state, so the
  shots show the `recording`/`未签核` state only. The sealed visuals (lock
  notice, signer line, save refusal) are covered by stub-driven unit tests,
  not by real windows.
- Every fixture design is AsterDoc-managed, so `SaveDesign` of a derived
  draft is always refused on this seed. The integration test pins that
  refusal; no real-window derive-save exists.
- The 900×600 shots show the initial viewport; board/archive content below
  the fold scrolls (same component behavior as the rest of the app).
