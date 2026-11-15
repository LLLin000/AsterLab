# Web Run Technical Audit

- 日期：2026-11-08
- 范围：现有 Web Run 执行/结果页面
- 方法：Impeccable `detect` + 源码检查 + Chromium 1440/390 走查
- 限制：不是完整 WCAG 人工审计；未做屏幕阅读器、完整键盘路径和真实 Windows 高 DPI 测试。

## Health summary

| Dimension | Score | Finding |
|---|---|---|
| Accessibility | 2/4 | 10px functional text；主按钮对比 3.1:1；placeholder 2.4:1 |
| Performance | 3/4 | 无测量证据；未发现明显的每帧布局问题；静态单页 |
| Theming | 1/4 | token 与 DESIGN.md 漂移；默认 dark 与暖灰 light 不一致 |
| Responsive | 3/4 | 390px 无横向溢出；移动记录表可用性未验证 |
| Implementation integrity | 2/4 | 结构与 token 有重复来源；legacy CSS 仍在扫描范围 |
| **Provisional total** | **11/20** | **Acceptable — 需要先修 token/可读性再迁移** |

## Evidence commands

```bash
"C:/Users/Lin/.agents/skills/impeccable/scripts/impeccable.cmd" detect --json web
"C:/Users/Lin/.agents/skills/impeccable/scripts/impeccable.cmd" detect --json \
  'http://127.0.0.1:19000/?page=runs&run=61b8961c-e2b7-43e1-bcd9-84aa6c427c57'
```

Live result: **10 findings — 8 warning, 2 advisory**.

## P1 — Theme/token drift

**Evidence**

- `web/view.html` line 2 hardcodes `data-bs-theme="dark"`.
- `web/components/tokens.css` light: `--al-bg:#f7f8fa`, `--al-accent:#2563eb`.
- DESIGN.md: `canvas:#f7f6f3`, `accent:#5b45c9`.
- Dark background rendered `rgb(16,19,24)`.
- Forcing `data-bs-theme=light` gives `rgb(247,248,250)`, still not DESIGN.md's warm canvas.

**Impact**

Rune theme generated from DESIGN.md and Web using `tokens.css` will disagree in canvas, accent, and contrast. The “warm neutral + violet” decision becomes decorative prose rather than an implemented system.

**Fix**

- Pick one token source; generate both Web CSS and Rune theme.
- Align light/dark values with DESIGN.md or explicitly amend DESIGN.md.
- Make theme selection explicit; do not force dark by default while documenting only light.
- Add a test that compares generated tokens to `DESIGN.md`.

## P1 — Contrast

**Evidence**

| Element | Ratio | Requirement |
|---|---|---|
| Primary button white on `#6d8cff` | 3.1:1 | 4.5:1 body |
| Brand white on `#7873ff` | 3.7:1 | 4.5:1 body |
| Textarea placeholder `#757575` on `#3b3b3b` | 2.4:1 | 4.5:1 body |

**Impact**

Primary action and form hints fail AA in the current dark theme. Rune must not inherit these values.

**Fix**

- Deepen accent for text-bearing fills, or use ink text on a light accent.
- Placeholder remains secondary text but ≥ 4.5:1; don't rely on opacity.
- Add contrast checks to the Rune component acceptance suite.

## P1 — Functional text below 11px

**Evidence** (live detector, all 10px)

- `通用准备`
- `D7 任务`
- `已完成` ×3
- `当前`
- `未开始`

**Impact**

These are task states, not legal smallprint. At 125%/150% Windows scaling and on 390px, 10px becomes a legibility failure. It also makes Rune's text layout harder to compare because Web is already below the floor.

**Fix**

- Functional text floor: 11px; design target 12px.
- Do not solve by adding a 10px token to DESIGN.md. Change the component.
- Add a fixture with long Chinese labels and three-digit numbers.

## P2 — Legacy CSS in scan scope

**Evidence**

Static `web` scan: 145 findings / 12 warnings / 133 advisories. Many come from `web/vendor/tabler`, `web/components/markdown.css`, `proposal.css`, and legacy `view.html` rules, not the current Run.

**Impact**

A naive “fix all 145” pass will churn unrelated code and may break legacy `/workspace`. A naive ignore will hide real Run drift.

**Fix**

- Define the migration audit scope as current `pages.js`/`run.js`/`tokens.css` and their rendered surfaces.
- Quarantine legacy CSS explicitly; do not let it define new tokens.
- Keep the live URL scan as the Run gate; static scan as advisory.

## P2 — Responsive / table behavior

**Evidence**

- 390px `scrollWidth === innerWidth` (no horizontal overflow).
- Run record table is wide; mobile screenshot shows page stacked, but full table interaction was not exercised.

**Risk**

Inline inputs in a horizontally constrained table may become unusable under touch; `RecordTable` needs a defined mobile strategy (scroll container with sticky first column, or per-row cards).

**Fix**

- Define `RecordTable` mobile mode in COMPONENTS.
- Test 390px scroll, input focus, keyboard, and touch target.
- Do not simply shrink cells below 11px.

## P2 — No single source for task/result state

**Evidence**

- Run page derives current task, completed chips, and results rows from separate client-side filters.
- `RECORDING.md` identifies the known `measurements[]`/current-task consistency gap.
- The UI currently renders both `task.status` and event-derived state.

**Risk**

Rune could reproduce a different “completed/current” interpretation than Web. That would be a factual mismatch, not a visual one.

**Fix**

- Rust app layer returns `RunWorkspace` with task status and measurement projection.
- UI renders only that projection.
- Add a cross-surface fixture test before Rune slice.

## P3 — Confirmation copy and nested headings

**Evidence**

- “确认 / 记录” contains “本步读数 · …” and separate “本任务记录”.
- Copy says completion and measurement are separate, but the hierarchy does not show it.

**Fix**

- One “本步记录” section with measurement sub-block; “本步事件” below.
- Put the completion bar at the end of the record block.

## Pass/fail for Rune migration gate

**Not passed yet.**

Reasons:

1. Token source is inconsistent; Rune would inherit a different visual language.
2. Contrast and 10px text fail before migration; reproducing them would not be acceptable.
3. `RunWorkspace`/measurement single-source contract is not yet fixed.
4. Mobile `RecordTable` behavior is undefined.
5. Rune technical spike has not run.

These are **pre-migration fixes**, not reasons to cancel Rune.
