# DES-00 evidence

Raw acceptance outputs for the AsterDoc step-tree commands. Every file is the
verbatim stdout+stderr of the command below, with two machine-specific
prefixes rewritten so the evidence does not leak host paths:

| placeholder | replaced prefix |
|---|---|
| `<worktree>` | the task worktree path |
| `<tmp>` | the temporary data directory used for the run |

Commands (run 2026-11-14 in the task worktree, `CARGO_TARGET_DIR=target`,
`LABWORKBENCH_DATA_DIR=/tmp/des00-evidence/data`):

| file | command |
|---|---|
| `cargo-test-labworkbench.txt` | `cargo test --locked --bin labworkbench` |
| `cargo-test-core.txt` | `cargo test --locked --manifest-path crates/asterlab-core/Cargo.toml` |
| `cargo-test-app.txt` | `cargo test --locked --manifest-path crates/asterlab-app/Cargo.toml` |
| `asterdoc-command-schema.txt` | `cargo run --locked --bin labworkbench -- schema asterdoc.command` |

Results: 33 passed (bin), core suite green, app suite green, schema command
exit 0 and prints the `asterdoc.command` schema.

Deletion contract (card section 实施 step 3): `remove_step` returns
`STEP_REFERENCED` and leaves the document, revision and database untouched when
the step is used by a Run, referenced by another design object or step
(`uses:`), has nested steps, or is marked `protect_references: true`. A delete
with none of those blockers still removes the step from `steps`,
`business_order` and the workflow `edges` atomically — the card requires that
cleanup rather than an edge-based rejection.
