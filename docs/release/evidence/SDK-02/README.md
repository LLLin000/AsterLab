# SDK-02 Go/Rust parity evidence

## Matrix

`fixtures.json` is generated from every module entry in `prototypes/mygo-run/internal/asterdoc/spec/modules.json`: 17 `kind@schema` entries, 59 fixture cases. Each entry has a valid case and two malformed cases (required field omitted and wrong type). Additional cases cover out-of-range plate coordinates, incompatible quantity units, and unresolved group/assay references. `step@1` and `step@2` are marked historical; `step@3` is current.

QA execution is isolated and stateless with respect to user data. Go `Parse` runs through a Go test overlay (no production Go files changed); subtests run concurrently. Rust `asterdoc.validate` runs one process per fixture, up to four concurrently, with a unique `LABWORKBENCH_DATA_DIR` under a temporary directory per case. The two full acceptance runs also exercise clean process restarts. Rust isolated data-tree SHA-256 aggregate: `59f16ffc15abe43349b48701feb88e4351e80fb0750181414310597f5d9d1e4e`.

## Difference disposition

| Difference | Evidence | Disposition |
|---|---|---|
| Go parser does not register `readout@1`; Rust accepts the valid sample and validates malformed readout fields. | Three cases: `valid-readout-schema1`, `invalid-readout-schema1-missing-schema_id`, `invalid-readout-schema1-wrong-type-schema_id`. | Known compatibility gap. SDK-02 does not alter module implementation. Repair task `3553701c-89a8-4077-9887-5ef49d9250d5` was opened; SDK-02 is blocked by that task. |
| Go exposes parser error text, while Rust exposes `INVALID_INPUT` plus a message. | Rust rejection results use code `INVALID_INPUT`; Go `Parse` has no stable code API. | Compare normalized categories (`missing_field`, `invalid_type`, `unit_or_quantity`, `invalid_reference`, `rule_rejection`); retain both raw errors in `parity.json`. |

## Acceptance output

`python3 tests/sdk-parity/run.py` — exit 0:

```text
cases=59 pass=59 fail=0 differences=3 isolated_data_sha256=59f16ffc15abe43349b48701feb88e4351e80fb0750181414310597f5d9d1e4e
```

The command also prints the three known `readout@1` differences and their dispositions. Full output: [`parity.txt`](parity.txt).

`python3 tests/sdk-parity/run.py --json` — exit 0; JSON includes `total`, `pass`, `fail`, `difference_count`, `differences`, and `known_differences`. Full output: [`parity.json`](parity.json).

`git status --short`:

```text
?? docs/release/evidence/SDK-02/
?? tests/sdk-parity/
```

## Reproduction

From the repository root, run the two commands above. The runner uses the workspace cargo target directory from `cargo metadata`; it does not override `CARGO_TARGET_DIR`, `RUSTC_WRAPPER`, or `SCCACHE_DIR`. Every Rust input and isolated data directory is temporary; no user database is selected.
