# SCI-03 evidence — ReadoutSchema v1

Task: `SCI-03 (Contract) 通用 ReadoutSchema 与记录契约`
Branch: `feature/sci-03-contract-read-epx`
Commit base: `04200ba` (SCI-01 merged)

## Acceptance commands (raw output in this directory)

| Command | Expectation | Result | File |
|---|---|---|---|
| `cargo test -p asterlab-core readout_schema` | exit 0, >= 6 passed | exit 0, 12 passed, 0 failed | `readout-schema-tests.out` |
| `cargo check -p asterlab-core` | exit 0 | exit 0, no warnings | `cargo-check.out` |
| `grep -c '^### ' docs/contracts/readout-schema-v1.md` | >= 5 | 12 | `heading-count.out` |

Extra (not required by the card, guards neighbouring tests): `cargo test -p asterlab-core` → 287 + 3 + 6 passed, exit 0 (`core-suite.out`).

## Files

- `docs/contracts/readout-schema-v1.md` — contract: five value kinds, unit, missing semantics (`null` vs reason), source (`raw`/`derived` + `derived_from`), timepoint/sample/target association, mapping onto `execution_event.create` `readouts[]`, error-code table, examples and rejection examples.
- `crates/asterlab-core/src/readout_schema.rs` — types, serde, `validate_schema`, `validate_readout`, `check_derived_sources`, pure `to_legacy_readouts` projection, 12 tests.
- `crates/asterlab-core/src/lib.rs` — `pub mod readout_schema;`.

No event contract, CLI, GUI or storage change: the legacy `readouts[]` shape is untouched, and existing events are neither migrated nor rewritten.

## Notes for review

1. Card body writes `above_ulog`; `AGENT_API.md` and existing `readouts[].qualifier` use `above_uloq`. The contract freezes `above_uloq` (serde rejects `above_ulog`) and records the discrepancy in §8.1 instead of adding an alias.
2. `table` readouts carry no per-cell missing reason; §8.2 states the limitation and points to `plate`/`numeric` readouts.
3. `to_legacy_readouts` is a pure projection used for the dry-run example; it performs no write and is not a new command.
