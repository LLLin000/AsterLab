# LB-04 — Run asset matrix real-window evidence

## Capture source and build

All images below are screenshots from the launched `prototypes/mygo-run/dist/mygo-run.exe` on real Windows windows (not `ui.Tester`). The recorded window size is the outer window size; the PNG dimensions are the captured client area.

- Build commit: `f087ba1`
- `Runs.dll` SHA-256: `79B8A35520EDDE2EBF34660EEF7413C0BCE50D4606FFC986556F12785DC2A348`
- `mygo-run.exe` SHA-256: `3AEEDA6F6E8E0DA4A9AEDCFED8AB85B1CC5CFB556C41DD8D219B55F9E7B47142`
- Run: QA-01 CCK-8 AsterDoc Run `7f0e6a5d-a7da-41af-976a-732c95b18f60`; one task with saved readouts 0.62 and 0.71.
- Asset links were created with the checkout-built `labworkbench asset.link` tool. The local source `prototypes/mygo-run/internal/runui/evidence-notebook.png` is linked at the coordinates below and once without coordinates to exercise the matrix and `未标注` bucket.

## Real-window captures

| Image | Window size | PNG size | Theme | Content |
|---|---:|---:|---|---|
| `lb04-real-assets-1360x880-light.png` | 1360×880 | 1344×871 | Light | Full 2-well × 2-timepoint matrix, `未标注`, and the A1 / 6 h / FOV 1 coordinate preview with `返回 Run 时间线`. |
| `lb04-real-assets-1360x880-dark.png` | 1360×880 | 1344×871 | Dark | Same matrix and coordinate preview in dark theme. |
| `lb04-real-assets-900x600-light.png` | 900×600 | 884×591 | Light | Narrow-layout matrix with both wells, both timepoints, and `未标注`. |

## Matrix data and reported gaps

- `A1 × 6 h`: FOV 1, 2, 3 (complete).
- `A1 × 24 h`: FOV 1 and 3; the matrix explicitly reports **FOV 2** missing (`1 个缺口 · FOV [2]`).
- `B1 × 6 h`: FOV 1.
- `B1 × 24 h`: no linked asset; the empty matrix cell reports **1 unlinked view** (`1 个缺口 · 未关联视野`) and does not name an FOV index.
- `未标注`: one linked file with no well, timepoint, or FOV.

Thus the exact missing coordinates the matrix reports are `A1 × 24 h × FOV 2`, and the empty cell `B1 × 24 h` (one unlinked view, no FOV index supplied). It does not report additional missing FOV indices after the highest linked index.

## UX rules evidenced

- `UX-IA-3` — Run owns execution facts and linked assets.
- `UX-P4` — the asset grid is rendered as a well × timepoint matrix.
- `UX-LAYOUT` — evidence at 1360×880 and 900×600.
- `UI-REDESIGN.md` §2, Run: `LinkedAsset` coordinates include `WellID`, `Timepoint`, and `FOVIndex`.
