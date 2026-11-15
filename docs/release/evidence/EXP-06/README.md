# EXP-06 evidence — pantry ledger and label export

Commands were run from the task worktree on 2026-11-15, in the order below.

| Command | Exit | Log |
|---|---|---|
| `cargo test --bin labworkbench` | 0 | [`cargo-test-bin-labworkbench.log`](cargo-test-bin-labworkbench.log) |
| `cargo test --manifest-path crates/asterlab-core/Cargo.toml` | 0 | [`cargo-test-asterlab-core.log`](cargo-test-asterlab-core.log) |
| `cargo test --manifest-path crates/asterlab-app/Cargo.toml` | 0 | [`cargo-test-asterlab-app.log`](cargo-test-asterlab-app.log) |

Test totals:

- `labworkbench` bin: 41 passed, 0 failed.
- `asterlab-core`: 406 + 3 + 6 + 2 passed, 0 failed.
- `asterlab-app`: 49 passed, 0 failed.

The logs are the raw command output. Only absolute build paths were redacted:
the checkout path became `<repo>`, the shared target directory became
`<target-dir>` and the temp data directories became `<tmp-dir>`, so no
machine-specific absolute path is stored here. Test names, counts and failures
are unchanged.

## What these tests cover

- `crates/asterlab-core/src/pantry.rs` — strict `YYYY-MM-DD` parsing, civil-date
  arithmetic across leap boundaries, status derivation (`unopened`, `opened`,
  `near_expiry`, `expired`, `exhausted`), inclusive near-expiry threshold,
  field validation, and the stable `PN-XXXXXXXX` label code derived from the
  immutable item id.
- `crates/asterlab-app/src/lib.rs` — pantry rows survive an app reopen, revision
  increments, `include_exhausted` filtering, label codes are not rederived on
  update, and a retired (deleted) item stays retired after reopen.
- `src/cli.rs` — `pantry.upsert` / `pantry.list` / `pantry.delete` /
  `label.export` through the real dispatch path (expiry projection with pinned
  `as_of`, `REVISION_CONFLICT`, `INVALID_INPUT`, byte-identical label exports,
  tombstone delete that keeps the retired label code reserved), plus
  `execution_event.create` with `pantry_item_id`: `NOT_FOUND` for an unknown or
  retired item, an `advisory` notice for an expired item, and a `null` reference
  with an empty notice list when the field is omitted.
