# AsterLab v1.2 — Design Review Checklist & Layered Acceptance Model

- **Version**: 1.2.1 (Revised per Conditional Review)
- **Purpose**: Multi-tier inspection and sign-off rubric for AsterLab native desktop builds.
- **Enforcement**: Any build exhibiting even ONE violation from this checklist **MUST BE REJECTED**.
- **Evaluation Method**: Replaces mechanical pixel-diff tests with a **4-Layer Acceptance Model**: Structure, Visual, Interaction, and Business/Science.

---

## 1. The 4-Layer Acceptance Model

```
+---------------------------------------------------------------------------------------------------------+
| Layer 4: Business & Science Layer (业务科学层)                                                          |
| - Zero developer confessions, natural Chinese wet-lab terminology, pure math scientific calculators     |
+---------------------------------------------------------------------------------------------------------+
| Layer 3: Interaction & State Layer (交互与状态层)                                                       |
| - Honest transactional save lifecycle, paste preview, 8-state coverage, review mode lock, focus ring    |
+---------------------------------------------------------------------------------------------------------+
| Layer 2: Visual & Token Layer (视觉与令牌层)                                                            |
| - Single-source theme tokens, typography scale, 1px border discipline, hero SVG icon primacy in nodes   |
+---------------------------------------------------------------------------------------------------------+
| Layer 1: Structural Layer (信息架构与结构层)                                                            |
| - Strict 900×600 responsive contract, Inspector tabs, single-column Run tabs, zero horizontal clipping  |
+---------------------------------------------------------------------------------------------------------+
```

---

## 2. Layer 1: Structural Layer Acceptance (信息架构与结构层验收)

| # | Inspection Criterion | Current Demo Status | v1.2 Production Requirement | Verdict |
| :- | :--- | :--- | :--- | :-: |
| **1.1** | **No Raw Button Piles** | **FAIL**: `projectView()` renders a flat vertical stack of raw buttons: `继续上次工作`, `新建设计`, `导入`, followed by unstyled project buttons. | Replace with structured `SectionCard`, metadata rows, status badges, and prioritized visual affordances. | **REJECT** |
| **1.2** | **900×600 Inspector Responsive Contract** | **FAIL**: At 900×600, Designer inspector columns squish horizontally, cutting off well coordinates and form labels. | In 900×600, Inspector converts from 3 horizontal columns into a top 3-tab segmented switcher (`[ 步骤属性 ] [ 96孔板映射 ] [ 检测合约 ]`). | **REJECT** |
| **1.3** | **900×600 Run Responsive Contract** | **FAIL**: At 900×600, dual 50/50 surface crushes table width below minimum readable bounds. | In 900×600, Run converts to a single-column tabbed interface (`[ 操作指引与质控 ] [ 读数录入表格 ]`) with sticky bottom save bar. | **REJECT** |
| **1.4** | **Zero Canvas Horizontal Overflow** | **FAIL**: Workflow nodes overflow off the right window edge when resizing. | Canvas auto-fits within viewport and enables smooth horizontal pan/scroll with visible indicators. No node clips. | **REJECT** |

---

## 3. Layer 2: Visual & Token Layer Acceptance (视觉与令牌层验收)

| # | Inspection Criterion | Current Demo Status | v1.2 Production Requirement | Verdict |
| :- | :--- | :--- | :--- | :-: |
| **2.1** | **Hero Scientific SVG Icons on DAG Nodes** | **FAIL**: Designer nodes are text-only rounded rectangles without scientific illustrations. | Every node must feature a prominent **36×36px Hero SVG Illustration (`<path>`)** for Material, Operation, and Instrument, with in/out ports. | **REJECT** |
| **2.2** | **Authentic 96-Well Microplate Widget** | **FAIL**: Designer uses 96 raw tiny text buttons with `"●A1"` or `"✓A1"` labels and no coordinates. | Dedicated `Plate96Widget` with A–H / 1–12 coordinates, rectangular box-drag selection, and semantic group color fills. | **REJECT** |
| **2.3** | **Zero Rogue Hex Colors** | **FAIL**: Hardcoded rogue hex colors (`#b00020`, `#22863a`, `#0366d6`) scattered across Go files. | Single-source token consumption from `internal/theme`. Every color resolves to semantic design tokens. | **REJECT** |
| **2.4** | **OpenType Tabular Numbers** | **FAIL**: Proportional font causing cell text jitter during live updates. | Tabular numbers enabled (`font-variant-numeric: tabular-nums` / `tnum`) for all numerical cells, coordinates, and units. | **REJECT** |

---

## 4. Layer 3: Interaction & State Layer Acceptance (交互与状态层验收)

| # | Inspection Criterion | Current Demo Status | v1.2 Production Requirement | Verdict |
| :- | :--- | :--- | :--- | :-: |
| **3.1** | **Honest Transactional Save Lifecycle** | **FAIL**: UI claims "已自动同步" or "已保存" before Core confirms save transaction. | Strict state machine: `就绪 (rev X)` -> `粘贴预览` -> `草稿 (N项未保存修改)` -> `提交中 (锁定)` -> `已保存 (rev X+1 · Core已确认)`. | **REJECT** |
| **3.2** | **Non-Destructive Paste Preview** | **FAIL**: Pasting TSV immediately mutates cells without dimensions or overwrite warnings. | Explicit `PastePreviewDialog` modal reporting row/col dimensions and overwrite counts before user commits. | **REJECT** |
| **3.3** | **Strict Separation of Review Mode vs. Run** | **FAIL**: Navigating to a completed step in Run keeps inputs editable, risking accidental biological data loss. | Navigating to completed steps activates **"任务回看 (Review Mode)"** with read-only cells, banner notice, and locked save buttons. | **REJECT** |
| **3.4** | **Keyboard Focus & Reduced Motion** | **FAIL**: Tab navigation leaves invisible focus or default OS square outline; animations ignore system settings. | High-visibility 2px Accent focus ring (`FocusRingWidth: 2`, `FocusRingOffset: 1`); immediate transition bypass when `ReduceMotion` is enabled. | **REJECT** |

---

## 5. Layer 4: Business & Scientific Layer Acceptance (业务科学层验收)

| # | Inspection Criterion | Current Demo Status | v1.2 Production Requirement | Verdict |
| :- | :--- | :--- | :--- | :-: |
| **4.1** | **Zero Developer Confessions in UI** | **FAIL**: Displays `"Core does not currently store..."`, `"Protocol editing and saving are not supported by Core"`, `"Rust RunService 不可用"`. | Never display backend architectural apologies or module names. Display clear, positive user guidance. | **REJECT** |
| **4.2** | **Scientific Calculator Purity (Math Only)** | **FAIL**: Dilution calculator invents speculative wet-lab protocols ("涡旋 10 秒", "使用 P10 移液器", "避光保存"). | Calculators compute pure physical-chemical quantities ($V_1$, $V_{\text{diluent}}$, $D$) and include explicit scientific boundary disclaimers. | **REJECT** |
| **4.3** | **Separation of Design vs. Execution Checks** | **FAIL**: Design pre-conditions and bench checks are lumped together into an undifferentiated list. | Protocol cleanly separates **设计前置校验 (Design Pre-conditions)** from **现场执行质控 (Execution Bench Checks: Blocking vs Advisory)**. | **REJECT** |
| **4.4** | **Professional Chinese Scientific Terminology** | **FAIL**: Jargon mixing: `"CCK-8 read"`, `"Same experimental unit repeated"`, `"Plate"`, `"Table"`. | Natural Chinese wet-lab phrasing: `"CCK-8 读板"`, `"同一实验单位重复"`, `"孔板视图"`, `"表格视图"`. | **REJECT** |

---

## 6. Review Sign-Off Signatures

A release build candidate is eligible for v1.2 sign-off **only if**:
1. All 16 checklist items across all 4 layers score **PASS**.
2. No console errors or panics occur when resizing the window rapidly between 1200×760 and 900×600.
3. Tab key can traverse every interactive control on all five pages without pointer interaction.
