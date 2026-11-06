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
- Synthetic demo records are explicitly synthetic; they must not be presented as real data.
