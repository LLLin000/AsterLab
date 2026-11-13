# SDK-04 evidence — unified structured diagnostics

Task: `SDK-04 (Core) 统一结构化诊断输出`
Branch: `feature/sdk-04-core-jun`
Base commit: `ca69142` (SCI-02 merged)

The captured Cargo logs preserve command/test output; machine-local absolute
source and target paths are redacted as `[workspace]` and `[target]` to avoid
publishing host paths.
## Acceptance commands (raw output in this directory)

| Command | Expectation | Result | File |
|---|---|---|---|
| `cargo test -p asterlab-core diagnostic` | exit 0, >= 5 passed | exit 0, 9 passed, 0 failed | `diagnostic-tests.out` |
| `cargo test --bin labworkbench` | exit 0 | exit 0, 20 passed, 0 failed | `cli-tests.out` |
| `grep -c 'source' crates/asterlab-core/src/diagnostic.rs` | >= 1 | 19 | `source-grep.out` |

## L1 suites (category work method)

| Command | Result | File |
|---|---|---|
| `cargo test --manifest-path crates/asterlab-core/Cargo.toml` | exit 0; 314 lib + 3 + 6 + 2 doctests passed | `core-suite.out` |
| `cargo test --manifest-path crates/asterlab-app/Cargo.toml` | exit 0; 25 passed | `app-suite.out` |

`asterlab-app` is a path dependency but not a root workspace member, so
`cargo test -p asterlab-app` cannot match it; the manifest form above is the
equivalent CI command (`.github/workflows/linux-tests.yml`).

An earlier `cargo test -p asterlab-core --doc` run failed with
`extern location for asterlab_core does not exist` while another task rebuilt
the shared `CARGO_TARGET_DIR`; the same command passed afterwards
(`doctest.out`). No source file involved in that failure was modified by this
card.

## CLI smoke (`--diagnostics json`)

`cli-diagnostics-smoke.out` records five invocations: success envelope with an
empty array, failure envelope with a structured `NOT_FOUND` diagnostic, the
unchanged failure envelope without the flag, the rejected `--diagnostics yaml`
value, and `agent_edit.validate` returning `status = blocked` with the
`INVALID_PROPOSAL` finding in both `result.issues` and `diagnostics`.

## Files

- `crates/asterlab-core/src/diagnostic.rs` — `Diagnostic` with `path`, `code`,
  `severity`, `message`, `affects`, `next_action`, `source`;
  `DiagnosticSeverity` (`blocking` / `needs_confirmation` / `advisory`);
  `DiagnosticSource` (`user_confirmed` / `imported_record` /
  `referenced_protocol` / `derived` / `agent_suggestion` / `unknown`);
  conversions from `SpecIssue`, `ContractIssue`, `SemanticIssue`,
  `AgentEditIssue`, `TimelineIssue`; normalization of legacy prose and
  `error.issues` JSON.
- `crates/asterlab-core/src/labware.rs` — the existing string-error validation
  path (`ContainerPlan` and labware profile fields) now produces
  `Vec<Diagnostic>` from `validate_diagnostics`; the legacy `validate`
  signature and messages are preserved as a lossless projection.
- `crates/asterlab-core/src/agent_edit.rs` — additive
  `AgentEditErrorCode::as_str` (same strings as serde) so the code survives the
  conversion; no field or wire change.
- `crates/asterlab-core/src/lib.rs` — registers `pub mod diagnostic`.
- `src/cli.rs` — `--diagnostics json` on any business command; envelopes then
  carry a top-level `diagnostics` array. Failures derive it from the error and
  its `issues`; successful read-only validations derive it from
  `result.issues`. Failure envelopes keep `error.code`, `error.message`,
  `error.path` and `error.issues` unchanged. Output schemas document the
  optional `diagnostics` property.
- `AGENT_API.md` — documents the flag, the seven fields and the two closed
  vocabularies.

## Notes for review

1. No error code meaning changed: `error.code` still drives exit codes; the new
   array is an additional view.
2. `Diagnostic.source` is `derived` for Core checks and `unknown` for legacy
   prose where the origin was never recorded; it is never guessed.
3. UI, frozen Runs, `docs/contracts/*` and `spec/modules.json` are untouched.
