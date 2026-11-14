# QA-01 Evidence — CCK-8 design → run → 96-well record → restart → cross-read

## Baseline & Scope
- Checkout: `O:\qa03`, HEAD `29e230c` (= `origin/main` at run time, includes RUN-06).
- Card scope is evidence only: `write: docs/product/evidence/QA-01/`. No fixture, code, frozen Run or Protocol changed.
- The harness lives outside the repo (`%TEMP%\qa-harness.py`); its data directory is `%TEMP%\qa-qa01\data`, never the user database.

## What ran
1. Seeded a CCK-8 project through the CLI only: Protocol block/version, then an AsterDoc design with two groups, a 96-well plate (`A1/A2` control, `B1/B2` treatment) and one executable read step; `asterdoc.create_run` froze that revision.
2. Real Windows GUI, 1360×880 light: recorded `A1 = 0.62` and `B1 = 0.71` through the plate view, `Ctrl+S` saved (`已保存`).
3. Relaunched at 900×600 light: plate view still shows `0.62` in `A1` and `0.71` in `B1`.
4. Relaunched at 1360×880 dark and 900×600 dark: switched to the **table** projection of the same run and read the same two values back — a cross-view read of one stored record, not a second copy.
5. Re-read Core with `labworkbench run.workspace`: exactly one `measurement` event, `batch-op-1794664645813243400-1`, schema `cck8.plate96.od450.v1`, cells `(0,0)=0.62` and `(1,0)=0.71`.

## Restart consistency
`cell_values` before and after the two restarts are equal (`QA01_RESTART_EQUAL_OK`), and the measurement
event count stays 1 — no duplication, no loss.

## Screenshots
- `qa01-1-plate-1360x880-light.png` — plate entry before save.
- `qa01-2-plate-900x600-light.png` — same record after relaunch, `已保存`.
- `qa01-3-table-1360x880-dark.png` — table cross-read: `A1 0.62`, `B1 0.71`.
- `qa01-4-table-900x600-dark.png` — table cross-read at the narrow breakpoint (`A1 0.62`; `B1` is below the fold in a 900×600 viewport).

## Files
- `run-output.txt` — verbatim harness output.
- `commands.txt` — checkout identity and the Core read-back.
- `core-readback.txt` — the `run.workspace` summary the harness asserted against.

## Observed defects (not fixed here)
- The flow rail prints `层级诊断: NOT_FOUND: AsterDoc revision v2 does not exist` for a run whose design has no AsterDoc revision (both screenshots show it). Evidence only; needs its own card.
- At 900×600 the table keeps its scroll offset across view switches, so the first rows can start off screen; the harness scrolls up before asserting. Worth a look as a separate UI card.
