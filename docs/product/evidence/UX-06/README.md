# UX-06 evidence — write-error state + Notebook state word

Build provenance (screenshots must come from this build):

- Build commit (HEAD): `ebe1d1d`
- mygo pin: `v0.8.4-0.20261108220316-c4197b7fe562`
- `Runs.dll` SHA256: `615C49225F3308EA449EB7256E2D9C59F8C8EDCA719C4BFD13FBB9C309140927`
- `mygo-run.exe` SHA256: `E88FD3E5A8FA2A9D61FA13A4CDD2A33E056A058CA340EEB04554808ECA97D5D3`

Target states (1360x880, light):

- (a) Run where Core rejects the asset link: the recording gallery now shows the
  Core reason (`关联失败：…`) with a `重新关联` retry affordance (UX-P5).
- (b) Notebook in saved state: the toolbar shows `已保存` (and `未保存` while a
  change is pending), replacing the unsaved-only dot (UX-STATE), and no font
  size outside the theme scale (UX-P3).

Unit/regression evidence (from commit `ebe1d1d`, `go test ./... -count=1`, exit 0):

- `TestRecordingLinkErrorShowsCauseAndRetry` (internal/runworkspace) — a failing
  `OnLinkFile` surfaces the Core message + `重新关联`, and retry re-invokes it.
- `TestNotebookShowsSavedAndUnsavedState` — both `已保存` and `未保存` render in
  the right states.

Note: real-GUI screenshots were not captured in this run; the checks above are
the automated verification attached to this card. Capture (a)/(b) from the build
above before merge.
