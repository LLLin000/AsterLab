# DOC-05 evidence — SDK catalog and agent-edit status facts

- Base: `6ca46c5e60b54000cb6900b7bdde1d4016b7a39d` (`origin/main`)
- Raw command output: [`commands.txt`](commands.txt)
- Reproducible table check: [`compare_manifest.py`](compare_manifest.py)

## What changed

1. `docs/sdk/module-catalog-v1.md`
   - Header count: 15 → **16 `kind@schema`** (14 distinct `kind`).
   - Added the rule that `step@1` / `step@2` / `step@3` are three schema versions of one `kind`.
   - Go registration line: `BuiltinRegistry()` registers 16 `kind@schema`, matching `spec/modules.json` and the 16 specs in `registry.go`.
2. `docs/sdk/compatibility-and-gaps.md`
   - Manifest count: 15 → **16 `kind@schema`** (14 distinct `kind`; `step` has schema 1/2/3).
   - The remaining `asterdoc.batch` gap now cites its evidence (`src/cli.rs` has no `asterdoc.batch` entry).
3. `docs/contracts/agent-edit-v1.md`
   - Status changed from “validate/commit 未实现” to implemented:
     `agent_edit.validate` = AG-02, PR #10, merge `8363d1c`;
     `agent_edit.commit` = AG-03, PR #17, merge `2423959`.
   - Provenance item marked implemented: `records.data_json.agent_edit` stores
     `proposal_id`, `proposal_digest`, `confirmed_by`, `idempotency_key`, `committed_at`, `new_revision`.
   - Still-unimplemented items kept with evidence: context validation, `needs_confirmation` / `advisory`
     production, `AGENT_API.md` registration, and index registration in `docs/README.md` / `core-v1.md`.

## Verification

- `len(modules[])` = `16` (acceptance command, exit 0).
- Catalog table rows = 16 module rows + 1 header; `grep -c '^| '` = 41 because other tables also start with `| `.
- `compare_manifest.py`: `mismatches: 0` across category, capabilities, permissions, renderers, rules.
- `grep -n '尚未实现'`: one remaining hit, `asterdoc.batch`, now carrying its evidence.
- `grep -n 'agent_edit' AGENT_API.md` and `grep -rn 'agent_edit' docs/README.md docs/contracts/core-v1.md`
  return no matches, so those registrations are still gaps.

## Out of scope (not edited)

- `docs/sdk/README.md` still says “All 15 registered modules”. That is outside DOC-05's `write` scope;
  it is ambiguous (14 distinct kinds vs 16 `kind@schema`) and should be aligned by a separate docs card.
