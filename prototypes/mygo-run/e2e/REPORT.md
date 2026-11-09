# MyGo Production Gate Report (unsigned)

| Check | Result | Evidence |
|---|---|---|
| Native module build | PASS (go run pinned mygo native check) | `.mygo/native/windows-amd64/Runs.dll` |
| Clean dist build | PASS | `scripts/mygo-build.ps1` (BUILD_INFO.txt) |
| GUI launch | NOT TESTED | manual |
| Two readouts save/reopen | NOT TESTED (GUI) | `go test ./e2e/...` covers service-level |
| CLI cross-read | PASS (module test) | `module_test.go` |
| Failure injection (missing sample) | PASS | `module_test.go` |
| Stale response guard | NOT TESTED (GUI) | `app.apply` in main.go |
| Startup/memory | PASS (observed) | MEASURE.txt: 3.67s, 93.8MB peak |
| Signed installer | FAIL (not provided) | unsigned only |

Limitations:
- No GUI automation available; manual verification required.
- No code signing/installer.
- Operation idempotency and cross-process concurrency not implemented.
