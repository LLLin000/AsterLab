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
- `tests/cli-coverage.cjs` still reports 13 catalogued CLI tools without a real positive/negative test (view.data_preview, semantic/catalog/labware/component tools). The functional smoke does not fail early on this gap.
- Synthetic demo records are explicitly synthetic; they must not be presented as real data.
