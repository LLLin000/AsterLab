# LB-00 evidence — asset coordinates in the contract and the Run read model

Card: `LB-00 (Contract) V2 资产坐标与读模型字段`
Base commit: `0ab3f2a` (origin/main)

## Environment

- `cargo 1.100.0 (3acc79c0c 2026-11-05)`, `rustc 1.100.0 (0ef1bc386 2026-11-09)`
- `go version go1.27.3 linux/amd64`
- Shared build cache used as-is (`CARGO_TARGET_DIR=/srv/build/asterlab-target`, `RUSTC_WRAPPER=sccache`); no target-dir override.
- Repository paths in the captured output are rewritten to `<repo>`; no `/var/lib/kandev/...` absolute path is stored.

## Acceptance commands

| Command | Exit | Result |
|---|---:|---|
| `cargo test --bin labworkbench` | 0 | 42 passed, 0 failed — includes `cli::tests::run_workspace_carries_asset_coordinates_and_the_gap_grouping` |
| `cargo test --manifest-path crates/asterlab-core/Cargo.toml` | 0 | 409 + 3 + 6 + 2 passed, 0 failed — includes the three `run_workspace::tests` coordinate tests |
| `cargo test --manifest-path crates/asterlab-app/Cargo.toml` | 0 | 50 + 0 passed, 0 failed |
| `cargo run --bin labworkbench -- schema run.workspace` | 0 | output schema contains `linked_assets`, `asset_coverage`, `fov_index`, `coordinate_id` |

Raw files:

- `cargo-test-bin-labworkbench.txt`
- `cargo-test-asterlab-core.txt`
- `cargo-test-asterlab-app.txt`
- `schema-run-workspace.json` (stdout of the schema command)
- `checks.txt` (command → exit code → test summary / matched schema fields)

## Go type sync

`prototypes/mygo-run/modules/runs/runs.go` renames nothing and only adds three fields with
matching JSON tags (`wellId`, `timepoint`, `fovIndex`). Compile check, run from
`prototypes/mygo-run`:

```text
$ go build ./modules/runs
exit=0
```

No `runs_native.go` regeneration was needed: the generated bridge decodes into `Asset` / `AssetLink`,
whose field set only grew with optional values.

## What the read model now carries

`run.workspace` adds two fields next to `plates`:

- `linked_assets[]`: `asset_id`, `task_key`, `sample_id`, `well_id`, `timepoint`, `fov_index`, `coordinate_id`.
- `asset_coverage[]`: `(well_id, timepoint)` groups with `fov_indices[]` and `asset_ids[]`.

Old links stay `null`; the read model never fills a well, timepoint or FOV from the task, the sample or
the file name. Contract and default rules: `docs/contracts/asset-coordinates-v1.md`.
