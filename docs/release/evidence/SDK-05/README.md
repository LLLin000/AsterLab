# SDK-05 evidence — readout.submit / readout.batch

Task: `SDK-05 (Core) 读数提交命令 readout.submit 与 readout.batch`
Branch: `feature/sdk-05-core-readout-bgy`
Base: `dd00f20` (SCI-03 merged)

## Acceptance commands (raw output in this directory)

| Command | Expectation | Result | File |
|---|---|---|---|
| `cargo test -p asterlab-core readout_submit` | exit 0, >= 6 passed | exit 0, 8 passed, 0 failed | `core-readout-submit.out` |
| `cargo test --bin labworkbench` | exit 0 | exit 0, 21 passed, 0 failed | `cli-tests.out` |
| `cargo run --bin labworkbench -- schema readout.batch` | exit 0 | exit 0 | `schema-readout-batch.out` |

L1 suite (work method for `(Core)`): `cargo test -p asterlab-core` → 295 + 3 + 6 passed, exit 0
(`core-suite.out`); `cargo test --manifest-path crates/asterlab-app/Cargo.toml` → 27 passed, exit 0
(`app-tests.out`). The repository is not a Cargo workspace, so the app crate is tested through its
manifest.

## Smoke run (real CLI, throwaway database)

`schema-readout-batch.stderr` holds the build stderr; `smoke.out` is the transcript of:

1. `readout.submit` on the `PureLink RNA run` of a **copy** of
   `prototypes/mygo-run/testdata/rna-case/labworkbench.sqlite` → `ok=true`, `replayed=false`;
2. the identical retry → `replayed=true`, same record id, one stored event;
3. `readout.batch` with CSV cells against one plate record inserted into that copy only →
   `accepted=2`, one event carrying two `result_cells` (one `raw`, one `missing` with `value=null`);
4. the same batch with a duplicated position → `INVALID_INPUT` + `duplicate_position`, execution-event
   count unchanged (no partial write).

The evidence SQLite file in the repository is untouched; the plate row exists only in the temp copy.

## Files

- `crates/asterlab-core/src/readout_submit.rs` — request types, validation, idempotency keys, CSV
  parser, `plan_submit` / `plan_batch`; 8 tests named `readout_submit_*`.
- `crates/asterlab-core/src/lib.rs` — `pub mod readout_submit;`.
- `src/cli.rs` — `readout.submit` / `readout.batch` dispatch, one-transaction write path,
  replay lookup, tool registry (input/output schema, description, side effects), 4 CLI tests,
  and `run_workspace_typed` extracted from `run_workspace` so submission planning validates against
  the typed frozen Run.
- `AGENT_API.md` — command table row and `## 读数提交（readout.*）` section.

No new record kind, no new table, no change to `execution_event` field semantics, no GUI/Windows work.

## Notes for review

1. **Idempotency scope.** `readout.submit` uses the card's tuple
   `(run_id, step_path, endpoint, sample_id, timepoint)`. `readout.batch` uses the same identity
   fields expanded per cell with `(plate_id, row, column)`: several wells of one plate legitimately
   share endpoint/sample/timepoint, so the bare tuple would collapse a plate into one key. Values are
   content, not identity; a replay returns the first record and reports `replayed=true` instead of
   overwriting. Corrections are out of scope for this card.
2. **Cross-Run isolation.** `run_id` is part of every key and the replay lookup is scoped to
   `parent_id`; covered by `readout_submit_key_is_stable_and_run_scoped` and
   `readout_submit_replays_the_first_record_and_isolates_runs`.
3. **Atomicity.** Planning happens before any insert; the insert itself is one
   `execution_event.create` (`create_execution_event_tx`) inside one immediate transaction, so both a
   validation failure and a rejected event reference roll back with zero events
   (`readout_batch_writes_one_event_or_nothing`).
4. `readout.batch` requires `plate_id` and rejects batches whose plate cannot be uniquely resolved
   from the Run (`unresolved_plate` / `ambiguous_plate` / `unknown_plate`) rather than guessing a
   well coordinate.
5. `docs/sdk/compatibility-and-gaps.md` still says there is no CLI batch readout tool; it is outside
   this card's write scope and needs a follow-up doc update.
