# QA-07 dependency restore audit

Date: 2026-11-14 (VPS local), 2026-11-13T17:24Z
Operator: OMP under user instruction
Task: QA-07 `0760cd15-3e5e-4999-92e7-5cec28794232`
Blocker: OPS-01 `1d956a99-e266-450c-966d-f4d7ddbf0e69`

## Reason

The earlier recovery work removed the incoming dependency so OPS-01 could
start. After OPS-01 was running, the dependency was restored. The public
Kandev API had no working blocker-write route (collection POST and workspace
variants returned 404), so this one repair used a direct `task_blockers` insert
with `created_at`.

## Verification

- Kandev task context lists blockers: RDO-01 (COMPLETED), OPS-01 (IN_PROGRESS).
- `blocked=true` for QA-07.
- `human-hold` remains on QA-07.
- No feeder, workflow, session, or merge state was changed by the repair.

## Boundary

This is an audited maintenance exception, not a Home or feeder implementation
pattern. A supported Kandev blocker-write API, or a Home-owned effect adapter,
is required before automating this class of repair.

## Follow-up

OPS-01 / PR #38 later reached Merged/COMPLETED. At that point QA-07's
`blocked` flag became false. The `human-hold` was intentionally retained for a
separate reason: the QA-07 card's acceptance references a missing
`tests/fixtures/run_qa07.py` and its scope does not include that path. The
dependency condition is satisfied. The card's `tests/fixtures/**` write scope
does cover the missing runner, so the earlier hold reason was a misread of the
scope, not a card-design blocker. After the dependency cleared, Home
`task check` passed and the `human-hold` label was removed; QA-07 was moved to
Agent to implement `tests/fixtures/run_qa07.py` within the approved scope.
