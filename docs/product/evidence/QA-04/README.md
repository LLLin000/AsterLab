# QA-04 Evidence — RNA fixture parameter slots and bindings

## Baseline
- Checkout `O:\qa04`, base `18567c2` (= `origin/main` at run time).
- Card scope is the fixture only: `write: scripts/rna-fixture.py, docs/product/evidence/QA-04/`. Nothing else changed.

## Bug it fixes
`QA-02` measured `protocol.block.preview` failing with
`INVALID_INPUT: 步骤块不存在根参数槽位：lysis_vol`.
Cause: the fixture created every block with `parameter_slots: []`, while the RNA flow's
documented parameters (`lysis_vol`, `lysis_time` — see `docs/product/evidence/AG-05/README.md`)
were referenced by callers. Core rejects a preview binding whose slot the **root** block does
not declare (`src/cli.rs`, `validate_root_bindings`).

## Change
1. `RNA_SLOTS` declares `lysis_vol` (number, `uL`, required) and `lysis_time` (number, `min`, required).
2. The RNA child block carries them; the root block repeats them and maps the child's slots with
   `source_slot`, which Core requires because both are required children slots.
3. `rna-ids.json` now also records `root_block` so a preview can target the root directly.
4. The fixture stays single-shot: it clears and recreates its own data directory, so repeated runs
   do not duplicate records.

## Verification
- `python scripts/rna-fixture.py` → exit 0, 13 task paths created; verbatim output in
  `fixture-output.txt`.
- `labworkbench protocol.block.preview` with
  `{block_version_id: <root_block>, bindings: [{lysis_vol: 200 uL}, {lysis_time: 10 min}]}`
  → **exit 0**, `ready: true`, and the values resolve down the tree:
  `root/lysis_vol`, `root/lysis_time`, `root/rna/lysis_vol`, `root/rna/lysis_time`.
  Verbatim output in `preview-output.txt`.

## What this unblocks
`QA-02` (RNA multi-step regression) can now exercise parameter binding instead of stopping at
the preview error. Its remaining blocker is RUN-07 (steps with no checks have no completion action).
