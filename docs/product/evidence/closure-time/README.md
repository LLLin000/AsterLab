# CLOSE-05 Evidence — Design budget + Run multi-phase timer closure

## Baseline & Scope
- Repository: `LLLin000/AsterLab`, isolated `ovnfs` checkout `O:/close05`.
- Baseline: `62fa199` (CLOSE-04 merge); no rebase/force-push.
- Scope writes (all inside the card):
  - `prototypes/mygo-run/internal/designer/timing.go` (new)
  - `prototypes/mygo-run/internal/designer/timing_test.go` (new)
  - `prototypes/mygo-run/internal/designer/designer.go`
  - `prototypes/mygo-run/internal/designer/step_list.go`
  - `prototypes/mygo-run/internal/runworkspace/bench.go`
  - `prototypes/mygo-run/internal/runworkspace/timers.go`
  - `prototypes/mygo-run/internal/runworkspace/timerclose_test.go` (new)
  - `prototypes/mygo-run/run_recording_page.go`
  - `prototypes/mygo-run/close05_test.go` (new)
- Zero Core changes (`crates/`, `src/` untouched).

## Implementation (per-item verdicts)
1. **Design OperationSpec inputs + Core budget (`UX-P3`, `UX-STATE`, `UX-P5`, `UX-DESIGN`)** — DONE in code, unit-tested.
   Canonical per-step `计时*` params (`计时类型/计时时长/重复次数/时间锚点`, optional second phase `*2`)
   edited in the step inspector (reused `properties`/`selectRow`/`textRow`, `lucide.Timer/Repeat/Anchor`).
   `BudgetInput` builds the OperationSpec v2 array (protocol order, neutral `incubate` carrier, seconds);
   steps without usable timing are counted `未知`, malformed values surface step-scoped causes —
   never estimated. Budget cache invalidates on `Touch`/`Save`/`Load` (previously loaded once and stale).
2. **Run consumes frozen phases (`UX-STATE`, `UX-P5`)** — DONE in code, unit-tested.
   `runTimerPhases` wired into bench callbacks reads the frozen `DesignData` revision the Run pinned
   (`designer.PhasesForFrozenStep`), never the live draft, never an empty slice
   (nil = single-phase `from_record`). Phases flow into `UpdateTimer` advance and the relay labels;
   completed rounds file exactly one observation event (incomplete advance files none — restart-safe).
   Pause/resume explicit (`继续` for paused), timeout (`超时`) and t0 wait (`等待 t0…`) unchanged and covered.
3. **QA-HUVEC F1 fix (`UX-P5`, `UX-STATE`, `UX-P2`)** — DONE in code, unit-tested.
   Same-task timers get explicit identity + scope (`TimerIdentity`: `正计时 #2 · 5%`, group encoded as
   `@group` id suffix, Core-opaque); task timepoint shown; unknown ids fall back to mode label, never raw id.
   Every creation refusal sets a visible cause (no mode / duplicate / service); success clears the error.
   Second same-mode stopwatch gets a distinct id — pinned by `TestSecondStopwatchKeepsDistinctIdentity`.
4. **Real-GUI proof — PARTIAL (honest caveat, see below).**
   App boots with the change (`native services: run/design/time-budget` all true), project `CLOSE-05 定时闭环`
   created, CCK-8 skeleton design `新实验` rev 1 opens with the pre-change budget bar
   (`设计标注` + `尚无 OperationSpec v2 时间数据`). Timing-edit → budget → Freeze → relay screenshots
   NOT captured in this run (automation input investigation consumed the budget; see caveats).

## Verification
- `go test -count=1 ./internal/designer/ ./internal/runworkspace/ .` — all `ok`
  (incl. new `TestParseDurationS`, `TestParseStepTiming`, `TestBudgetInputFeedsCoreSpecs`,
  `TestPhasesForStepRelay`, `TestPhasesForFrozenStep`, `TestSecondStopwatchKeepsDistinctIdentity`,
  `TestCreateTimerWithoutModeFailsVisibly`, `TestAdvanceBeforeElapsedFilesNoEvent`,
  `TestRunTimerActionForwardsFrozenPhases`, `TestClose05TimerPhasesFromFrozenDesign`).
- `go vet` on the three scope packages — clean; `gofmt -l` on touched files — clean.
- `internal/asterdoc` passes with the freshly built native module
  (`mygo native build -target host ./modules/runs` → `Runs.dll` for windows-amd64).
- Native `Runs.dll` + `labworkbench.exe` rebuilt from `62fa199` into `D:/.ci-cache/close05/bin/`.

## DESIGN CONSISTENCY (`UX-DESIGN`)
- Tokens: `tokens.TextMeta/TextBody/Canvas/Elevated/Border/Text/TextMuted/Danger/DangerBg/WarningText`,
  `t.Space`/`t.Rem`, `tokens.PanelRadius`-adjacent `Radius(6)` — no literals added.
- Reused: `components.Text/Button/TextInput/Select/TagInput/Badge`, `propertyList` rows,
  `plate.GroupColor`-safe palette (no new palette), onevra icons.
- New components: none. New visual vocabulary: none.
- P1: primary `创建 Run` stays in the toolbar; P2: timer identity never shows raw ids;
  P4: budget bar keeps chip row; P5: every failure carries cause + retry.

## Honest caveats
- No real-window screenshots of the new UI in this run: `click <ref>` only focuses buttons in this
  harness (`press @ref --settle` is the working verb); page-nav-to-Design auto-opens the create dialog,
  but the full timing-edit → Freeze → relay → restart → stopwatch screenshot chain was not completed.
  The GUI proof in item 4 must be completed on the Verify lane before merge.
- Window sizes 1360x880 / 900x600 not yet exercised; evidence dir currently holds this README only.
- Second timing phase is unanchored by default (window phases stay manual in Run); schedules omitted (caveat by design).
- `internal/protocol/archive_test.go` + `internal/runworkspace/history_test.go` show CRLF-only diffs from the
  Windows checkout (content identical, `git diff --ignore-cr-at-eol` empty); left untouched, not committed.
