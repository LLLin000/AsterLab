# AG-05 Evidence — RNA End-to-End Gate

## Baseline and Scope
- Repository: `LLLin000/AsterLab`, `origin/main` at `a757f28` (includes `AG-03` + `AG-04`).
- Isolated `ovnfs` checkout: `O:\ag05`, isolated data dir `C:\Users\Lin\AppData\Local\Temp\ag05-verify\data`.
- Scope: `docs/product/evidence/AG-05/` only (zero code modifications).

## End-to-End Steps & Raw Outputs
1. **Seed baseline RNA fixture + frozen Run v1**:
   - Design `aeba3d9c-fead-4e9f-be76-2bf778f0a011` rev `1`, Protocol `b22d4922-4c40-455c-b4f2-7124854ce216` v1 (`9f37af03-ace9-4c3d-910f-587098623517`, `lysis_vol = 200 uL`, `lysis_time = 10 min`).
   - Froze `AG05-Old-Run-v1` (`6679a982-e024-4895-92ca-53b4035c3e5c`) with measurement `RNA-S1 · Conc = 385.2 ng/uL`; captured full pre-edit snapshot (`run.workspace` + `record.get`).
2. **Propose & Validate via GUI (`Agent 提案`)**:
   - Loaded `proposal-rna.json` (`lysis_vol: 200 uL -> 250 uL`, `lysis_time: 10 min -> 15 min`) and clicked `校验提案` → `STEP1_VALIDATE_OK` (`ag05-01-validate-1360x880-light.png`).
3. **Confirm & Commit (`接受并提交`) + New DesignVersion**:
   - Clicked `接受并提交` → `STEP2_COMMIT_OK proto_v2=4bd844f7-9ba2-4771-a874-db85cb536554` (`new_revision: 2`, `ag05-02-commit-1360x880-light.png`).
   - Saved `asterdoc.save` referencing `proto_v2` → `DESIGN_V2_SAVED rev= 2 design_rev= 2`.
4. **Reopen Design & Verify New Version**:
   - Reopened GUI → `实验设计` → `RNA 参数绑定案例` showed `rev 2` (`STEP3_REOPEN_DESIGN_OK`, `ag05-03-reopen-design-new-ver-1360x880-light.png`).
5. **Reopen Old Run & Verify Frozen Values Unchanged**:
   - Reopened GUI with `ASTERLAB_RUN_ID=6679a982-e024-4895-92ca-53b4035c3e5c` → `实验运行` & `实验方案` showed `已冻结` and `RNA-S1 · 385.2 ng/uL` (`STEP4_OLD_RUN_GUI_OK`, `ag05-04-old-run-frozen-1360x880-light.png`, `ag05-05-old-run-frozen-900x600-dark.png`).
   - Deep JSON equality check (`run.workspace` + `record.get` before vs after) → `OLD_RUN_FROZEN_EXACT_MATCH_OK` (every field identical).
