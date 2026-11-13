# SCI-04 evidence — sample-level execution identity and `workflow.progress`

Task: `SCI-04 (Core) 样本级执行身份与进度查询`
Branch: `feature/sci-04-core-o21`
Base: `0c93b93`

## Acceptance commands (raw output in this directory)

| Command | Expectation | Result | File |
|---|---|---|---|
| `cargo test -p asterlab-core sample_progress` | exit 0, >= 5 passed | exit 0, 9 passed, 0 failed | `sample-progress-tests.out` |
| `cargo test --bin labworkbench` | exit 0 | exit 0, 17 passed, 0 failed | `labworkbench-tests.out` |
| `cargo run --bin labworkbench -- schema workflow.progress` | exit 0, output contains `sample_id` | exit 0, `ok:true`, `sample_id` in input and output schema | `schema-workflow-progress.out` |

Extra (work method L1, not required by the card):

| Command | Result | File |
|---|---|---|
| `cargo test -p asterlab-core` | exit 0, 314 + 3 + 6 + 2 passed | `core-suite.out` |
| `cargo test --manifest-path crates/asterlab-app/Cargo.toml` | exit 0, 23 passed | `app-suite.out` |

Reproduce the CLI scenario: `bash docs/release/evidence/SCI-04/run.sh`.

## What the scenario proves

Input run: the reviewed RNA extraction fixture
(`prototypes/mygo-run/testdata/rna-case/labworkbench.sqlite`,
run `ad9b452e-f8ea-404b-9a2a-7124f9403b02`). The fixture database is copied to a
temp directory; nothing in the fixture changes.

1. `event-identity.out` — attempt 1 records S1 completed and S2 blocked
   (deviation); attempt 2 records the S2 retry with an explicit `retry` lineage
   link. Re-submitting the same `step_path + sample_ids + attempt` completion
   returns the original event id, so no duplicate fact is created.
2. `attempt-gap.out` — attempt 4 for S1 on the same step is rejected with
   `INVALID_INPUT` (`样本 S1 的下一 attempt 只能是 2，收到 4`), exit 2.
3. `workflow-progress-retry.out` — `workflow.progress` for
   `root/rna-extraction/02-lyse` returns two sample cells: S1 attempt 1
   completed; S2 attempt 1 failed (`deviation: true`) and attempt 2 completed.
   Step `complete: true` because both samples completed; the failed attempt is
   still queryable.
4. `workflow-progress-partial.out` — two negative cases:
   - one sample in progress → step `in_progress`, `complete: false`, second
     sample `not_started`;
   - an event declaring `step_status = "completed"` for S1 only on
     `root/rna-extraction/04-bind` → step `in_progress`, `complete: false`,
     `not_started_samples: 1`. A step completion event is not a step-wide
     success.

## Files

- `crates/asterlab-core/src/sample_progress.rs` — `SampleExecutionIdentity`,
  `SampleStatus`, `ProgressEvent`, the append-only attempt resolver, the
  per-step sample matrix projection (`build`, `build_from_workspace`) and the
  write-side identity planner (`plan_event_identity`, `next_attempt`). Nine
  tests cover retry separation, partial matrices, failures, skipped matrices,
  last-event time, explicit attempts, attempt gaps/zero, unknown steps and
  stored retry lineage.
- `crates/asterlab-core/src/run_workspace.rs` — `ExecutionEventView.attempt`
  carries the explicit attempt from stored events (`None` for legacy events).
- `crates/asterlab-core/src/lib.rs` — module registration.
- `src/cli.rs` — `execution_event.create` accepts `attempt`, validates it
  through `plan_event_identity`, stores it, and extends the completion dedupe
  key from step scope to `sample_ids[] + attempt`; new read-only
  `workflow.progress` command with input/output schema, description, side
  effects and `writes_data=false`.
- `docs/architecture.md` §5.6 — identity, attempt rules, matrix aggregation.
- `AGENT_API.md` — command catalogue, `workflow.progress` contract,
  `attempt` on `execution_event.create`, sample-scoped `step_status`.

No change to `docs/contracts/*`, `spec/modules.json`, frozen Runs, GUI or
Windows-only code.

## Notes for review

1. **Time field.** `last_event_at` uses `recorded_at` and falls back to
   `occurred_at`, matching `reading_facts`/`execution_timeline`. In the
   evidence the server stamp is what appears even when `occurred_at` is given.
2. **Legacy events.** Events without `attempt` stay readable: the resolver
   follows explicit retry links (`retry_of_event_id` or
   `lineage.relation = "retry"`) and defaults to attempt 1. Nothing is
   rewritten retroactively.
3. **Sample universe.** The matrix is built from the samples that appear in
   this Run's events plus `sample_ids[]` passed to the query. The Run does not
   persist a separate planned-sample list, so a sample can only be
   `not_started` when the caller names it or another step already mentions it.
   The evidence names both `S1`/`S2` through events.
4. **Dedupe key change.** The old completion dedupe matched
   step/timepoint/assay/task only, which returned S1's record when S2 completed
   the same step. It now also compares the sorted `sample_ids` and `attempt`;
   identical repeats still return the existing event, different samples get
   their own fact.
5. **No attempt auto-increment on write.** `attempt` is explicit; the read
   model derives attempts from retry links when it is absent. The CLI rejects
   only claims that skip ahead (`attempt > current + 1`), keeping existing
   event writes without `attempt` compatible.
