# SCI-04 evidence — sample-level execution identity and `workflow.progress`

Task: `SCI-04 (Core) 样本级执行身份与进度查询`
Branch: `feature/sci-04-core-o21`
Base: `0c93b93`

## Acceptance commands (raw output in this directory)

| Command | Expectation | Result | File |
|---|---|---|---|
| `cargo test -p asterlab-core sample_progress` | exit 0, >= 5 passed | exit 0, 10 passed, 0 failed; includes D1/D3, null-timepoint and no-cross-product cases | `sample-progress-tests.out` |
| `cargo test --bin labworkbench` | exit 0 | exit 0, 17 passed, 0 failed | `labworkbench-tests.out` |
| `cargo run --bin labworkbench -- schema workflow.progress` | exit 0, output contains `sample_id` and `timepoint` semantics | exit 0, schema exposes both `sample_id` and `timepoint` | `schema-workflow-progress.out` |

Extra (work method L1, not required by the card):

| Command | Result | File |
|---|---|---|
| `cargo test -p asterlab-core` | exit 0, 315 + 3 + 6 + 2 passed | `core-suite.out` |
| `cargo test --manifest-path crates/asterlab-app/Cargo.toml` | exit 0, 23 passed | `app-suite.out` |

The only output normalization is the exact machine-specific Cargo target root
`/srv/build/asterlab-target` → `<CARGO_TARGET_DIR>`; test results and all other
output are unchanged.

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
4. `workflow-progress-partial.out` demonstrates the Run's actual unscoped `null`
   timepoint group and confirms the query does not invent a second sample from
   another step. Planned samples can be added only by a query scoped to a
   concrete `step_path + timepoint`; the Core test covers that partial matrix.

## Files

- `crates/asterlab-core/src/sample_progress.rs` — `SampleExecutionIdentity`,
  `SampleStatus`, `ProgressEvent`, the append-only attempt resolver, grouped
  projection and write-side identity planner. Ten tests cover retry separation,
  partial matrices, D1/D3 identity and retry isolation, null timepoint, no
  timepoint/sample Cartesian product, failures, skipped matrices, last-event
  time, explicit attempts, attempt gaps/zero, unknown steps and retry lineage.
- `crates/asterlab-core/src/run_workspace.rs` — `ExecutionEventView.timepoint`
  and `.attempt` carry explicit event identity; legacy attempt remains `None`.
- `crates/asterlab-core/src/lib.rs` — module registration.
- `src/cli.rs` — `execution_event.create` accepts `attempt`, validates identity
  per timepoint, stores it, and deduplicates by timepoint/sample/attempt;
  `workflow.progress` registers timepoint-aware input/output schemas.
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
3. **Sample universe.** Samples are grouped by exact `(step_path,timepoint)`.
   Each group's matrix uses samples from that group's events; `sample_ids[]`
   adds `not_started` cells only when both a step and concrete timepoint are
   selected. The fixture's shared-step events remain in the `null` group.
4. **Dedupe key change.** The old completion dedupe matched
   step/timepoint/assay/task only, which returned S1's record when S2 completed
   the same step. It now also compares the sorted `sample_ids` and `attempt`;
   identical repeats still return the existing event, different samples get
   their own fact.
5. **No attempt auto-increment on write.** `attempt` is explicit; the read
   model derives attempts from retry links when it is absent. The CLI rejects
   only claims that skip ahead (`attempt > current + 1`), keeping existing
   event writes without `attempt` compatible.
