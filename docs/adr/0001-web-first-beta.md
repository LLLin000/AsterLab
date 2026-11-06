# ADR 0001: Web-first AsterLab Beta

- Status: accepted for v0.1.0 Beta
- Date: 2026-11-07
- Supersedes: the "Slint is the delivery frontend" assumption in `docs/architecture.md`

## Decision

The first distributable AsterLab is the existing Rust + Axum + SQLite service with the
HTML/CSS/Vanilla-JS workspace. The Web UI is the product shell; the native Slint desktop
prototype remains in `apps/desktop` as a research branch and is not maintained as a second
production frontend during the Beta.

## Consequences

- `target/release/labworkbench.exe` serves the embedded Web assets and starts the local
  service; no Rust/Node toolchain is required by end users.
- The Beta is a local desktop application even though its window is the user's browser.
- The Web UI must be validated with real browser E2E tests, not static prototype screenshots.
- A later native shell/WebView decision is deferred until the Beta workflow is stable.

## Boundaries

- No React/Vue/TypeScript migration in this release.
- No new scientific data model for UI convenience.
- Slint assets are retained for research; they are not a second shipping UI.
