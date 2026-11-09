# AsterLab v1.2 — Design Review Checklist (Demo Rejection Rubric)

- **Purpose**: Strict automated/manual inspection rubric for AsterLab native desktop builds.
- **Enforcement**: Any build exhibiting even ONE violation from this checklist **MUST BE REJECTED**.
- **Target Comparison**: Explicitly evaluates the current Demo against the v1.2 production standard.

---

## 1. The 10 Core Inspection Criteria

| # | Inspection Criterion | Current Demo Status | v1.2 Production Requirement | Verdict |
| :- | :--- | :--- | :--- | :-: |
| **01** | **No Raw Button Piles** | **FAIL**: `projectView()` renders a flat vertical stack of raw buttons: `继续上次工作`, `新建设计`, `导入`, followed by unstyled project/design buttons. | Replace with structured `SectionCard`, metadata rows, status badges, and prioritized visual affordances. | **REJECT** |
| **02** | **Zero Developer Confessions in UI** | **FAIL**: Protocol displays `"Core does not currently store structured reagent or instrument details here"` and `"Protocol editing and saving are not supported by Core"`. Designer shows `"Rust RunService 不可用"`. | Never display backend architectural apologies or module names. Display clear, positive user guidance: `"ℹ 本方案由实验设计固化生成，修改实验条件请在设计中调整并发布新版本"`. | **REJECT** |
| **03** | **No Comma-Separated CSV Hacks** | **FAIL**: Designer inspector requires typing comma-separated strings into single text fields: `"时间点"`, `"对照组"`, `"样本 ID:组别"`, `"基因（参考基因加 *）"`. | Multi-value biological parameters must use structured token fields, dedicated form inputs, or table rows with explicit validation. | **REJECT** |
| **04** | **Authentic 96-Well Microplate Widget** | **FAIL**: Designer uses 96 raw tiny text buttons with `"●A1"` or `"✓A1"` text with no plate border, no column/row headers, and no group colors. | Dedicated `Plate96Widget` with A–H / 1–12 coordinates, rectangular box-drag selection, and group color fills (`Control`, `Treatment`, `Blank`). | **REJECT** |
| **05** | **Zero Rogue Hex Colors** | **FAIL**: Designer and Protocol hardcode `#b00020`, `#22863a`, `#0366d6` across Go files. | Single-source token consumption from `internal/theme`. Every color must resolve to `Canvas`, `Surface`, `Border`, `Accent`, `Success`, `Warning`, or `Danger`. | **REJECT** |
| **06** | **Two-Tier Compact Desktop Discipline** | **FAIL**: At 900×600, sidebar and tables clip, text buttons overlap, and content overflows without responsive reorganization. | Strict responsive contract: 900×600 automatically collapses sidebars to 0px, switches dual-surfaces to tabbed modes, and preserves 30px click targets. | **REJECT** |
| **07** | **Complete 8-State Handling** | **FAIL**: Designer status shows raw enum strings `"idle"`, `"draft"`, `"saving"`, `"saved"`, `"error"` directly to users. | Localized visual state badges: `● 就绪`, `● 草稿未保存`, `⏳ 保存中…`, `✓ 已保存`, `⚠ 格式错误`. | **REJECT** |
| **08** | **Strict Separation of Review Mode vs. Run** | **FAIL**: In Run, clicking a historical completed task keeps edit inputs active, risking accidental data overwrites. | Navigating to completed tasks switches to **"任务回看 (Review Mode)"** with read-only cells, banner notice, and locked save buttons. | **REJECT** |
| **09** | **Professional Chinese Scientific Terminology** | **FAIL**: Jargon mixing: `"CCK-8 read"`, `"Same experimental unit repeated"`, `"Plate"`, `"Table"`, `"Paste preview"`. | Natural Chinese wet-lab phrasing: `"CCK-8 读板"`, `"同一实验单位重复"`, `"孔板视图"`, `"表格视图"`, `"粘贴预览"`. | **REJECT** |
| **10** | **Keyboard Focus & Accessibility** | **FAIL**: Keyboard Tab navigation leaves invisible focus or default OS square outline; no reduced-motion check. | High-visibility 2px Accent focus ring (`FocusRingWidth: 2`, `FocusRingOffset: 1`); immediate transition bypass when `ReduceMotion` is enabled. | **REJECT** |

---

## 2. Review Sign-Off Protocol

A release build candidate is eligible for v1.2 sign-off **only if**:
1. All 10 checklist items score **PASS**.
2. Automated visual regression tests match baseline mockups within a 0.5% pixel diff threshold.
3. No console errors or panics occur when resizing the window rapidly between 1200×760 and 900×600.
4. Tab key can traverse every interactive control on all five pages without pointer interaction.
