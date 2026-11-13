# SDK-03 evidence — block-level step editing through `step.edit`

## What was built

- `crates/asterlab-core/src/step_edit.rs` (new): `validate` / `commit` for the four
  block operations `insert`, `reorder`, `reuse`, `bind`, hanging off the existing
  agent edit v1 flow (`propose → validate → confirm → commit`).
  - Paths reuse the Protocol expansion addressing (`root`, `root/<use_id>`,
    `root/<use_id>/<use_id>`), so a deep sub-step edit is the same command.
  - Ops that leave the root tree are `FORBIDDEN_FIELD`; a well-formed path that
    does not resolve is `STALE_REFERENCE`; a container op whose path names a text
    step is `INVALID_PROPOSAL` (it never falls back to editing the parent block).
    No new error codes: the closed agent-edit vocabulary is reused unchanged.
  - Version numbers are produced by Core: every changed block gets
    `previous_version + 1`, the plan is ordered child-first, and each parent's
    block use is rewritten to the child version created in the same commit.
    Unchanged blocks keep their exact version (reuse), so frozen versions, Protocol
    versions and frozen Runs are never rewritten.
  - `edit_digest` covers the ops and the version preconditions (not the
    confirmation block); `confirmation.idempotency_key` replays the original result
    and writes no second version.
- `crates/asterlab-core/src/protocol_block.rs`: additive
  `ProtocolBlockVersionData.based_on_version_id` and `.step_edit` (typed
  provenance). Old block payloads deserialize unchanged (`default` + `skip`).
- `src/cli.rs`: `step.edit` registered in the dispatch, input/result schemas, tool
  catalogue (`tools --json`), effects and the agent-edit error envelope;
  `protocol_block_version` record schema accepts the two new fields.
- `AGENT_API.md`: command-table row plus the `step.edit` section (ops, path rules,
  version rules, digest/idempotency, error codes, and the follow-up
  `protocol.revise` call a Protocol needs to adopt the new root block).

## Acceptance (raw output)

| Command | Expected | Observed | Output |
|---|---|---|---|
| `cargo test -p asterlab-core step_edit` | exit 0, ≥ 6 passed | exit 0, **13 passed** | [core-step-edit.txt](core-step-edit.txt) |
| `cargo test --bin labworkbench` | exit 0 | exit 0, **20 passed** | [labworkbench-tests.txt](labworkbench-tests.txt) |
| `cargo run --bin labworkbench -- schema step.edit` | exit 0 | exit 0, `ok: true` plus the full input/output schema | [schema-step-edit.txt](schema-step-edit.txt) |

L1 suites from the card's work method:

| Command | Observed | Output |
|---|---|---|
| `cargo test -p asterlab-core` | exited 0, 316 lib + 3 + 6 + 2 passed | [core-tests.txt](core-tests.txt) |
| `cargo test --manifest-path crates/asterlab-app/Cargo.toml` | exited 0, 26 passed | [app-tests.txt](app-tests.txt) |

The `step_edit` filter matches 12 tests in `step_edit::tests` plus the
pre-existing `semantic_changeset::tests::protocol_step_edit_requires_location`
(13 total). Tests cover: insert positioning, deep reuse with child-before-parent
version ordering, reorder permutation rejection, unit mismatch vs the slot unit,
out-of-tree and unresolvable paths, container ops on a text-step path, stale
`expected_version` / base version, missing or mismatched confirmation digest,
idempotent replay, no-op rejection, cycle rejection, a concurrently moved child
block, and the closed wire shape.

## End-to-end CLI proof

[`step-edit-cli.txt`](step-edit-cli.txt) drives the real binary against a fresh
`LABWORKBENCH_DATA_DIR`:

1. `protocol.block.create` builds a child block, a reusable wash block and a root
   block that references the child.
2. `step.edit` without `confirmation` returns `needs_confirmation`, a plan with the
   child block first, and an `edit_digest`; nothing is written.
3. `step.edit` with `confirmation` returns two new versions (child, then root, both
   `version: 2` with `based_on_version_id`) and `replayed: false`.
4. Repeating the same `idempotency_key` returns the identical result with
   `replayed: true` and `created = 2`; no third version appears.
5. `protocol.block.preview` on the old root version still expands to
   `["root/prep", "root/lyse/iris"]`, while the new root expands to
   `["root/prep", "root/lyse/iris", "root/lyse/wait", "root/wash/rinse"]`.
6. `protocol.block.get` shows `latest_version.data.based_on_version_id` and the
   `step_edit` provenance (`edit_id`, `edit_digest`, `confirmed_by`,
   `committed_at`, `idempotency_key`, `invalidated_refs`).

## Notes and limits

- `cargo test -p asterlab-app` is not a valid invocation (the crate is not a root
  workspace member); the manifest-path form above is the one from `HANDOFF-VPS.md`.
- The VPS build cache is shared across worktrees (`CARGO_TARGET_DIR` plus the
  `sccache` Rust wrapper). During this card a sibling worktree's `asterlab-core`
  artifact was selected by one later build, which made
  `cargo test --bin labworkbench` fail with `unresolved import
  asterlab_core::step_edit`. Each recorded run above was produced after touching
  the crate sources in the same command, so the artifact came from this worktree;
  the failing contaminated run is not included.
- `step.edit` only produces `protocol_block_version` records. A Protocol keeps
  pointing at its old root until `protocol.revise` is called with the returned
  `new_root_block_version_id`; frozen Runs keep their exact versions by design.
- Host paths in the captured output are replaced with
  `[workspace path redacted]` / `[build output path redacted]`, and the schema run
  uses a temporary `LABWORKBENCH_DATA_DIR` so no user data directory is printed.
