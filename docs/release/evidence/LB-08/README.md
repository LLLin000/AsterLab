# LB-08 evidence — `asset.link.update` coordinate update

Card: `LB-08 (Contract) 资产坐标更新命令`
Base commit: `c7f4d5b` (`origin/main`)

## Environment

- `cargo 1.100.0 (3acc79c0c 2026-11-05)`, `rustc 1.100.0 (0ef1bc386 2026-11-09)`
- Shared build cache used as-is (`CARGO_TARGET_DIR=/srv/build/asterlab-target`, `RUSTC_WRAPPER=sccache`); no target-dir override.
- Repository paths in the captured output are rewritten to `<repo>`; no task-worktree absolute path appears in any artifact.

## Acceptance commands

| Command | Exit | Result |
|---|---:|---|
| `cargo test --bin labworkbench` | 0 | 46 passed, 0 failed (4 new `asset_link_update_*` tests) |
| `cargo test --manifest-path crates/asterlab-core/Cargo.toml` | 0 | 413 + 3 + 6 + 2 passed, 0 failed |
| `cargo run --bin labworkbench -- schema asset.link.update` | 0 | success envelope; `input_schema.required` = `id/well_id/timepoint/fov_index`, `output_schema` = `{record, event}` |
| real update run (temp data dir) | 0 | coordinates changed, `sha256` and file identity unchanged, `updated` event carries `previous` |

Raw files:

- `cargo-test-bin-labworkbench.txt`
- `cargo-test-asterlab-core.txt`
- `schema-asset-link-update.json` (stdout of the schema command; build diagnostics went to stderr)
- `asset-link-update-run.txt` (real CLI transcript: fixture, link, update, list, events, workspace, error paths)
- `checks.txt` (command → exit code → result)

## The real update run

`scripts/rc3-fixture.py` creates a project, a design and a frozen Run through the CLI in a temporary
data dir. The transcript then:

1. `run.workspace` → derives a real `task_key` and `step_path`.
2. `asset.link` → links `scan_A1.tif` at `A1 / t6h / fov 1` (sha256 computed from the file).
3. `asset.link.update` → `{id, well_id: "B2", timepoint: "t12h", fov_index: 2}`.
4. `asset.list`, `asset.events(updated)`, `run.workspace` → read the new coordinates back.
5. Four rejected calls (`fov_index=0`, malformed well, omitted `timepoint`, unknown id) → stable
   `INVALID_INPUT` (exit 2) / `NOT_FOUND` (exit 3), and the record keeps its previous coordinates.

Observed: record id unchanged, `sha256` unchanged (`4a48ee83…72f5e8`), root/relative path/size/modTime/
mediaType/availability unchanged, coordinates `A1_t6h_fov01 → B2_t12h_fov02`, event
`action = "updated"` with `previous = {wellId: A1, timepoint: t6h, fovIndex: 1, coordinateId: A1_t6h_fov01}`.

## What changed

- `src/cli.rs`: `asset.link.update` command, input/output schemas, tool registry entries,
  `asset.events` `updated` filter, optional `previous` in the event payload schema,
  one transaction for the record update and the event.
- `docs/contracts/asset-coordinates-v1.md` §5: input table, error codes, transaction and event
  rules, record-id semantics; §7/§8 updated for compatibility and scope.
- `AGENT_API.md`: command table row and the `asset.link.update` contract.

Not in this card: Go native bridge (`RunService`) and generator hashes — split to LB-10; no UI.
