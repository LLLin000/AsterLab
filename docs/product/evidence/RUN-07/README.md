# RUN-07 completion-control evidence

The QA-01 AsterDoc seed was used in an isolated `qa-run07` data directory: one plate, one assay and one executable step with `checks: []`. UX basis: **UX-P1** (primary action visible), **UX-P5** (failure cause and retry), **UX-STATE** (completion is persisted through Core), and **UX-DONE** (screenshots and focused behavior tests).

## Before

- `run07-before-1360x880-light.png` — real Run surface before the change; no step-completion control.
- `run07-before-900x600-light.png` — same defect at the narrow breakpoint.

## After

- `run07-after-action-1360x880-light.png` — 「标记完成」 visible before submission.
- `run07-after-completed-1360x880-light.png` — Core-confirmed `已完成` status after clicking the action.
- `core-readback.txt` — `labworkbench run.workspace` readback showing the step status `completed`.

The requested post-change 900×600 and dark-theme screenshots were skipped to prioritize the single real-GUI completion path and Core readback. Screenshots use the real Windows app in the light theme; the Run and data directory are isolated from the user's normal application data.
