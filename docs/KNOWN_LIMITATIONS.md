# Known limitations

- `operator` and `checked_by` are recorded names, not authenticated electronic signatures.
- The local HTTP API is bound to `127.0.0.1` but has no user accounts or network authentication.
- Run creation requires an explicit human-confirmation header in the UI and a trusted-writer
  process gate in the CLI. Neither is a substitute for a signed audit trail.
- A backup restore replaces the live database while the service is running; restore should be
  performed when no other writes are in progress. The UI prompts for confirmation.
- The Beta is single-user and has no collaboration, cloud sync, automated scheduled backup,
  instrument control, file attachments, or full electronic-signature workflow.
- The Slint application is retained as a research prototype and is not the Beta frontend.
- CLI catalog coverage is checked by static call/reference scanning across Node and Rust tests; it confirms every tool is invoked by a test but is not a coverage percentage or proof that every branch is exercised.
- DS-08 is `[BLOCKED]` at RC-05 (`e2a4603`): one deterministic Windows-only cargo failure, `protocol_version_queries_return_expansion_and_stable_not_found` (open SQLite handle vs `remove_dir_all`, OS error 32); Linux lane PASS, Windows build + full Go suite PASS (`evidence/RC-05/notes-vs-scope.md`, `evidence/RC-04/README.md`).
- DS-12 / DS-13 are `[UNVERIFIED]` at RC-05: no ADR superseding `0001-web-first-beta` exists; `scripts/rc3-fixture.py` was not re-run at `e2a4603`. DS-05 GUI E2E step, DS-07 CLI `sdk.validate` probe, and the DS-04 GUI freeze-badge screenshot are `[UNVERIFIED]`; DS-09 is proven only for closed-file copy semantics (QA-05), not end-to-end through the in-app buttons. Unverified items must not be claimed as complete.
- CLOSE-02…CLOSE-07 merged scope is in: record projection + archive finality (`record-closure.md`), design OperationSpec/time-budget + multi-phase timers (`time-plan-closure.md`), CLOSE-04 record surfaces, CLOSE-05 budget/frozen-phase timers, CLOSE-06 frozen-plan capture/gap refill, CLOSE-07 unified entries/timer strip/`asterlab://` deep links. In-app `record.sign_archive` finality holds after restart (`run_archive_state`); the in-app backup/restore buttons (`BACKUP_RESTORE.md`) were not exercised end-to-end.
