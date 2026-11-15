# LB-07 evidence

The Design step list provides move, indent, outdent, delete, and property-edit controls. The properties drawer edits the existing AsterDoc fields; successful commands adopt Core's returned revision. UX-P1, UX-P2, UX-P5, UX-STATE, UX-LAYOUT.

Build: `prototypes/mygo-run/dist/mygo-run.exe`; `BUILD_INFO.txt` reports
`commit bdb9520`, matching `git rev-parse --short HEAD` before capture.

## Captures

Both PNGs are real `mygo-run.exe` desktop-window captures at 1360×880, light
theme, from an isolated copy of the RNA extraction fixture database.

- [Step row actions, including the disabled path action and reason](lb07-row-actions-1360x880-light.png)
- [Rejected referenced-step delete with the Core message](lb07-rejected-delete-1360x880-light.png)

## Core 缺口

`src/cli.rs` `step_path()` returns `""` for a step without `path`, and `nest_step`/`promote_step` reject it with `step path must have a final segment`; the HUVEC sample has empty `path` on all steps. Indent/outdent remain disabled until the step has a non-empty path and a valid parent; the UI shows `需要先设置步骤路径`.

已由 LB-09 (#126, `5e8e52b`) 在 Core 侧补全；UI 仍不合成路径。
