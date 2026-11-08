# Rune Technical Feasibility Report

- 日期：2026-11-08
- 结论：**Gate 1 未通过（IME、表格、打包尚不可用）**；停止 Rune 扩张，保留 Web Beta。CJK 长文本在显式 `.wrap()` 下可换行，但 ICU4X 仍报告缺分段模型。
- 固定 revision：`a7f276c3630f85a06a0230295da4d2bd172d8d3b`（`anomalyco/rune` `dev`）
- 许可：MIT（仓库声明）
- 状态：Rune 官方仍是 pre-release；pin revision，不追浮动分支。

## What ran

| 项目 | 命令 / 环境 | 结果 |
|---|---|---|
| Rune 构建 | `cargo test -p hello` in pinned clone | 通过；`hello` 1/1 |
| Spike 编译 | `cargo check -p aster-spike` | 通过 |
| Headless PNG | `cargo run -p aster-spike -- --snap ... --scale 2 --theme light` | 通过；生成截图 |
| Accessibility tree | 同次 `--tree ...json` | 通过；中文 label、text-input、button 出现在树中 |
| 150% scale | `--scale 1.5 --theme dark` | 窗口/文字清晰；长中文仍溢出 |
| Scripted input | `--script`，tap 保存按钮 | 保存状态变化；字段键入未确认 |
| 表格 | 未做 | CJK 风险暴露后停止扩展；未证明可编辑表格 |
| 中文 IME | 未做 | 无法用 headless 替代真实 Windows IME |
| 打包 | **未完成** | `cargo build --release` 600s 超时；无 release 产物 |

Spike 源码：[rune-spike/](rune-spike/)（临时 clone：`target/rune-spike/examples/aster-spike/`）。  
证据截图：

- `docs/design/rune-spike-light.png`
- `docs/design/rune-spike-cjk-wrap.png`
- `docs/design/rune-spike-150.png`
- `docs/design/rune-spike-input.png`
- `docs/design/rune-spike-tree.json`

## Risk — CJK segmentation

`aster-spike` renders Chinese glyphs correctly, but every long Chinese paragraph emits:

```text
ICU4X data error: No segmentation model for complex script: Chinese/Japanese
```

Before adding `.wrap()`, the paragraph overflowed the right edge. With explicit `.wrap()` it wraps at character boundaries and stays inside the window at `--scale 2` and `--scale 1.5` (light and dark). The ICU4X errors remain.

Why it is still a risk:

- AsterLab's UI is Chinese-first; descriptions, errors, instructions and parameter notes are long.
- The error means Rune/ICU4X lacks a CJK segmentation dictionary in this build; wrapping may fall back to character boundaries and produce poor punctuation/word-break behavior.
- Rune's default text wrap is not automatic; every long text element must opt in. That is a component contract requirement, not a framework guarantee.
- This spike did not test line-break quality against Chinese typography rules.

**Gate result: not passed — CJK is conditionally usable; IME/table/packaging remain unproven.**

## What passed

- Rune compiles and runs on Windows with wgpu.
- Chinese glyph fallback works for short labels.
- Headless PNG and accessibility tree work; elements are inspectable.
- High-DPI 1.5x render is crisp.
- Basic text fields, focusable button and tap-save work; scripted tap reached the save action.
- AccessKit tree exposes roles and Chinese names.

## What remains unproven

| Gate item | Status | Why |
|---|---|---|
| Windows Chinese IME | **未测** | Headless cannot prove candidate window/composition; requires real window + IME |
| Table editing | **未测** | Rune/rune-kit has Table, but AsterLab editable table was not proven; stopped after CJK risk |
| 100+ row scrolling | **未测** | Same |
| Keyboard focus across table | **未测** | Basic button tap only |
| Packaging | **未完成** | Release build timed out at 600s; no artifact |
| Real app data | **未测** | Spike is synthetic; no database access |
| CJK line breaking | **有条件通过** | 显式 `.wrap()` 后长段落换行；ICU4X 仍报告缺分段模型，断行质量未验证 |

## Packaging

`cargo build --release -p aster-spike` exceeded 600s and produced no executable before the timeout. This is not a pass; it is an inconclusive packaging gate. Build time itself is a risk for a desktop release path.

## Environment

- OS: Windows 10/11 (win32 x64)
- Rust toolchain: repository default
- Rune clone: `target/rune-spike` (ignored, not committed)
- Render: headless, `--scale 2` / `--scale 1.5`, light/dark

## Decision

Per [MIGRATION.md](../../MIGRATION.md):

- **Do not start Run vertical slice.**
- **Do not migrate other pages.**
- **Keep Web Beta writable and default.**
- Open a focused Rune investigation: enable/load ICU4X CJK segmentation and re-run the same long-paragraph fixture.
- If fixed, repeat Gate 1 including real Windows IME and editable table; otherwise evaluate Slint/WebView2 or retain Web.

## Next experiment (bounded)

1. Find Rune/Parley feature or data package for CJK segmentation.
2. Re-run `aster-spike` with the same long Chinese string at 920×600 and 390 wide.
3. Only if wrapping passes, test:
   - 100-row editable table;
   - Chinese IME candidate/composition;
   - package size and startup;
   - accessibility tree for table cells.

No UI migration is approved by this report.
