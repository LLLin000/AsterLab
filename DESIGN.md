---
version: alpha
name: AsterLab
description: "A quiet, task-first native research workspace. Warm neutral canvas, one violet accent reserved for selection and primary action, hairline structure instead of card stacks, and a typography ladder where black body text always outranks gray labels."
colors:
  accent: "#5b45c9"
  accent-hover: "#4b37ae"
  accent-soft: "#efebfb"
  accent-tint: "#faf8ff"
  accent-line: "#cfc4f0"
  ink: "#1c1b18"
  ink-muted: "#5f5a52"
  ink-subtle: "#8f897f"
  canvas: "#f7f6f3"
  sidebar: "#f1efea"
  surface: "#ffffff"
  surface-subtle: "#faf9f7"
  surface-inset: "#f4f2ee"
  hairline: "#e7e3db"
  hairline-strong: "#d6d0c5"
  hover: "#efece6"
  warn: "#7a6233"
  warn-soft: "#f6eedd"
  plate-body: "#fbfaf7"
  plate-line: "#e3ded4"
  well-empty: "#f7f5f1"
  well-empty-line: "#ded9cf"
  well-mapped: "#d7d0c2"
  well-mapped-line: "#a49a88"
typography:
  display:
    fontFamily: system-ui
    fontSize: 26px
    fontWeight: 600
    lineHeight: 1.2
    letterSpacing: -0.2px
  title:
    fontFamily: system-ui
    fontSize: 20px
    fontWeight: 600
    lineHeight: 1.3
    letterSpacing: -0.1px
  headline:
    fontFamily: system-ui
    fontSize: 15px
    fontWeight: 600
    lineHeight: 1.35
    letterSpacing: 0
  body:
    fontFamily: system-ui
    fontSize: 14px
    fontWeight: 400
    lineHeight: 1.5
    letterSpacing: 0
  body-strong:
    fontFamily: system-ui
    fontSize: 14px
    fontWeight: 500
    lineHeight: 1.5
    letterSpacing: 0
  secondary:
    fontFamily: system-ui
    fontSize: 13px
    fontWeight: 400
    lineHeight: 1.45
    letterSpacing: 0
  meta:
    fontFamily: system-ui
    fontSize: 12px
    fontWeight: 400
    lineHeight: 1.4
    letterSpacing: 0
  caption:
    fontFamily: system-ui
    fontSize: 11px
    fontWeight: 500
    lineHeight: 1.3
    letterSpacing: 0.3px
spacing:
  xxs: 4px
  xs: 8px
  sm: 12px
  md: 16px
  lg: 24px
  xl: 32px
  xxl: 48px
rounded:
  xs: 4px
  sm: 6px
  md: 10px
  lg: 12px
  pill: 999px
components:
  page-shell:
    backgroundColor: "{colors.canvas}"
    textColor: "{colors.ink}"
  sidebar:
    backgroundColor: "{colors.sidebar}"
    textColor: "{colors.ink-muted}"
    width: 216px
  nav-row:
    backgroundColor: transparent
    textColor: "{colors.ink}"
    typography: "{typography.secondary}"
    rounded: "{rounded.sm}"
    height: 36px
    padding: "0 {spacing.sm}"
  nav-row-selected:
    backgroundColor: "{colors.accent-soft}"
    textColor: "{colors.accent}"
    typography: "{typography.secondary}"
  mode-tabs:
    backgroundColor: "{colors.surface-inset}"
    rounded: "{rounded.md}"
    padding: 3px
    height: 34px
  mode-tab:
    backgroundColor: transparent
    textColor: "{colors.ink-muted}"
    typography: "{typography.meta}"
    rounded: "{rounded.sm}"
    height: 28px
    padding: "0 {spacing.sm}"
  mode-tab-selected:
    backgroundColor: "{colors.surface}"
    textColor: "{colors.ink}"
    typography: "{typography.meta}"
    borderColor: "{colors.hairline}"
  button-primary:
    backgroundColor: "{colors.accent}"
    textColor: "{colors.surface}"
    typography: "{typography.meta}"
    rounded: "{rounded.sm}"
    height: 28px
    padding: "0 {spacing.sm}"
  button-secondary:
    backgroundColor: transparent
    textColor: "{colors.ink-muted}"
    typography: "{typography.meta}"
    rounded: "{rounded.sm}"
    height: 28px
    padding: "0 {spacing.sm}"
  section-label:
    textColor: "{colors.ink-subtle}"
    typography: "{typography.caption}"
  group-row:
    backgroundColor: transparent
    textColor: "{colors.ink}"
    typography: "{typography.body}"
    borderColor: "{colors.hairline}"
    height: 44px
    padding: "{spacing.xs} 0"
  group-row-selected:
    backgroundColor: "{colors.accent-soft}"
    textColor: "{colors.ink}"
  assay-row:
    backgroundColor: transparent
    textColor: "{colors.ink}"
    typography: "{typography.body}"
    borderColor: "{colors.hairline}"
    padding: "{spacing.sm} 0"
  assay-row-selected:
    backgroundColor: "{colors.accent-tint}"
  inspector:
    backgroundColor: "{colors.surface}"
    textColor: "{colors.ink}"
    width: 304px
    borderColor: "{colors.hairline}"
  field-label:
    textColor: "{colors.ink-subtle}"
    typography: "{typography.caption}"
  field-value:
    textColor: "{colors.ink}"
    typography: "{typography.body}"
  plate-card:
    backgroundColor: "{colors.surface}"
    textColor: "{colors.ink}"
    borderColor: "{colors.hairline}"
    rounded: "{rounded.md}"
    padding: "{spacing.md}"
  check-row:
    textColor: "{colors.ink}"
    typography: "{typography.meta}"
    borderColor: "{colors.hairline}"
---

## Overview

AsterLab is a **quiet, task-first native research workspace** for planning, preparing, and running cell experiments. It should feel like a well-made tool: calm, precise, and dense with real data—not a dashboard, not a form, not a marketing page.

The design follows three rules learned from Linear, Vercel Geist, Apple HIG, and GitHub Primer:

1. **Don’t compete for attention you haven’t earned.** Navigation, metadata, and supporting controls recede; the current work object stays in focus. (Linear’s 2026 refresh)
2. **Structure should be felt, not seen.** Use alignment, spacing, and hairline separators before boxes, borders, and shadows. (Linear; Apple layout)
3. **Black body text outranks gray labels.** Hierarchy comes from size, weight, and color—not from making every label bold or every card heavy. (Vercel Geist; Apple typography)
4. **Rhythm follows adjacency, not uniform padding.** List rows sit close together; sections get larger breathing room. (Notion’s page spacing refresh)
5. **Tokens are semantic, not raw colors.** Surface, border, text, and interaction roles stay stable across themes. (Obsidian CSS variables; GitHub Primer tokens)

The product is read-only in the current spike, but the visual system is designed for edit and execution states. A control that cannot write yet must not pretend to be a completed workflow.

The surface is a warm neutral canvas (`{colors.canvas}`), with pure white only where content is the object of attention (inspector, plate card, selected mode segment). There is **one accent** (`{colors.accent}`) and it means selection or primary action—never decoration, never “data exists.”

## Colors

The palette is a warm neutral ladder plus one violet accent. It deliberately avoids green as a data color: “mapped” is a neutral recorded state, not a success verdict.

- **Canvas** `{colors.canvas}` — page background.
- **Sidebar** `{colors.sidebar}` — one step quieter than the canvas.
- **Surface** `{colors.surface}` — inspector, plate card, selected segment.
- **Surface subtle / inset** `{colors.surface-subtle}` / `{colors.surface-inset}` — selected-mode track, nested metadata, quiet fills.
- **Hairline** `{colors.hairline}` — 1px separators and card outlines. Use it sparingly; prefer spacing first.
- **Ink / ink-muted / ink-subtle** `{colors.ink}` / `{colors.ink-muted}` / `{colors.ink-subtle}` — primary, secondary, and metadata text.
- **Accent / soft / tint / line** `{colors.accent}` / `{colors.accent-soft}` / `{colors.accent-tint}` / `{colors.accent-line}` — current selection and primary action only.
- **Warn / warn-soft** `{colors.warn}` / `{colors.warn-soft}` — unresolved conflicts and missing critical conditions. Do not use it for ordinary unknowns.
- **Well states** — `{colors.well-empty}` for unmapped, `{colors.well-mapped}` for recorded mapping. Selection adds `{colors.accent}`; the mapped state is never violet.

Rules:
- Never use accent for “mapped”, “complete”, or “valid.”
- Unknown, undecided, and not-applicable are text states first; color is secondary.
- A single screen may use at most one accented action and one accented selection.
- Avoid pure black and pure gray-blue; keep the warm-neutral cast consistent.

## Typography

The type ladder is seven levels. Black body text is always larger than gray labels in the same block.

| Token | Size/weight | Color | Use |
|---|---|---|---|
| `{typography.display}` | 26/600 | ink | page title |
| `{typography.title}` | 20/600 | ink | inspector title, run title |
| `{typography.headline}` | 15/600 | ink | group / assay / plate names |
| `{typography.body}` | 14/400 | ink | field values, reading text |
| `{typography.secondary}` | 13/400 | ink-muted | objectives, methods, metadata lines |
| `{typography.meta}` | 12/400 | ink-muted | mode tabs, counts, hints |
| `{typography.caption}` | 11/500 | ink-subtle | section labels, field labels, axis labels |

Rules:
- Section labels are **11/500 ink-subtle**, not 12/600. They organize; they do not compete with body values.
- Use 600 only for display, title, headline, and selected navigation.
- Body values wrap fully. Only list titles and metadata may elide.
- Do not set gray text larger than black body text in the same information block.
- Chinese text uses the system UI font; no custom display face in this spike.

## Layout

The shell has four persistent regions. Experiment design is **a state of the center workspace**, not a fifth panel or a full-screen overlay. See [frontend architecture](docs/research/2026-11-05-design-workspace-architecture.md) for the state machine, step flow, and vocabulary model.

| Region | Size | Responsibility |
|---|---|---|
| Top bar | 56px | project → design → run context, design/prepare/execute modes, draft state, save/cancel |
| Sidebar | 216px | project, design, run navigation; stays visible while editing; recedes visually |
| Center | fluid | browse state (map / structure / evidence) or draft state (4 steps: question, groups, assays, review) |
| Inspector | 304px | selected-object context in browse; live summary and unresolved items in draft |

Browsing and editing share the same shell. Entering a draft changes the center and the right panel, not the navigation or the current design context. The browse center uses three views—map, structure, evidence—rather than one long scroll. The draft center uses a four-step rail; the draft object is one, and switching steps must not lose input.

Center content:
- 32px side padding, 24px top/bottom.
- 24px between major sections; 8–12px within a section.
- Max readable measure around 760px; keep the design canvas centered rather than stretching text edge to edge.
- No card around every section. A section is a label, content, and whitespace. Use a 1px hairline only when two rows need separation.
- Adjacent list rows use 8–10px vertical padding and compact line height; major sections use 24–32px. Do not give a list row the same padding as a paragraph block (Notion adjacency).
- Semantic roles first: background, surface, border, text, accent, warning. Raw palette values never appear outside the theme file (Obsidian/Primer).
- Selected rows use a soft accent background and, at most, a 1px accent outline. Do not change row height or add heavy borders.

Density:
- Nav rows: 36px.
- Group rows: 44px.
- Assay rows: content height, 12px vertical padding.
- Inspector: 20/18 padding, 10px field spacing, 24px between sections.
- Plate card: 16px padding; plate board centered; no shadow.

## Elevation & Depth

This is a flat, surface-based system.

- **In-flow content:** no shadows. Use surface contrast and hairline borders.
- **Selected/floating:** a hairline border plus a very soft accent tint; no blur or gradient.
- **Only floating layers** (about window, future menus/popovers) may use a shadow; keep it soft and neutral.
- Never use shadow to imply hover or selection.

## Shapes

- Controls and list rows: 6px radius.
- Panels, plate card, selected mode track: 10–12px.
- Wells: 2px in thumbnail, 5px in top view.
- Pills are reserved for short status labels; do not turn every metadata item into a pill.
- Avoid fully rounded cards; they read as marketing UI, not a research tool.

## Components

### Navigation row
Height 36px; 13/500 title; 11/400 detail; selected = `{colors.accent-soft}` background + 2×18px accent indicator, text accent. No border. The sidebar should sit a few notches quieter than the canvas (Linear).

### Mode tabs
A single inset track (`{colors.surface-inset}`, radius 10, 3px padding) with three 28px segments. Selected segment = white surface + 1px hairline; unselected = transparent + ink-muted. No per-tab borders. This is one control, not three buttons.

### Buttons
- Primary: accent fill, white text, 28px, 6px radius. At most one per view.
- Secondary/quiet: transparent, ink-muted text, hairline only when needed. Hover = `{colors.hover}`.
- No drop shadows on buttons. Icon-only buttons require an accessible label.

### Section label
11/500, ink-subtle, letter-spacing 0.3px, 24px line box. Never 600 or accent.

### Group row
Flat row with a bottom hairline, not a card. Name 15/600; role and condition 13/400; `n` and technical repeats 12/400. Selected row uses `{colors.accent-tint}`; do not add a border. The number `n` is independent experimental units—never derived from wells.

### Assay row
Flat row, not a card. Title 15/600; endpoint 13/400; timepoints/relation 12/400; protocol state 12/500. Selected row uses `{colors.accent-tint}`. Full parameters live in the inspector. Do not repeat six label/value rows inside every assay.

### Inspector
A white contextual panel with grouped fields. Section heading 11/500 ink-subtle; field label 11/400 ink-subtle; value 14/400 ink; missing value 14/400 ink-subtle. Separators only between groups. No nested cards. The inspector answers “what is this object?”—not “run a form.”

### Plate
White card with a subtle hairline; the plate body is inset, not another card. Axis labels 10–11px. Mapped wells are neutral; selected wells use accent stroke and a 1px accent-line halo. Do not put explanatory text inside every empty well.

### Check row
A 6px dot (warn or neutral) plus a 12/600 title and 12/400 detail. Text carries the level; color is reinforcement only. Checks are data-completeness notes in this spike, not a scientific validation verdict.

## Data Semantics

Design language must not flatten scientific meaning:

- Design mode shows the current DesignVersion; prepare/execute show the frozen snapshot referenced by the Run.
- A Run is not a label; it freezes design and Protocol versions. If the snapshot or Protocol manifest is missing, say so and do not invent steps.
- Independent n, technical repeats, sample count, container slots, and Run count are separate values.
- Assay timepoints and relations are per assay; never render one global time line.
- Plan ≠ fact: planned outputs, planned slots, and planned layout are not actual materials, wells, or observations.
- Unknown ≠ no / zero / not applicable. “未记录”, “待确认”, and “未记录映射” remain visible.
- The relationship graph is optional and secondary. It only draws links explicitly listed by the current assay. No link means “not listed”, not “explicitly excluded.”

## Do's and Don'ts

**Do**
- Use whitespace and alignment before borders.
- Keep one accent per view.
- Make the current work object the largest, darkest element.
- Use flat rows with hairlines for repeated data.
- Put full details in the inspector; keep the canvas scannable.
- Show empty states as sentences that explain what is missing.
- Respect `prefers-reduced-motion` when the platform exposes it.

**Don't**
- Don’t wrap every row in a card.
- Don’t use accent for mapped wells, success, or “valid.”
- Don’t set gray labels at 12/600 while black values are 13/400.
- Don’t use six chips where one meta line would do.
- Don’t use shadows for hierarchy in flowing content.
- Don’t fabricate progress, steps, parameters, or completion from names or counts.
- Don’t present a read-only view as an execution workflow.

## Accessibility & Motion

- Text contrast: body and labels ≥ 4.5:1; large titles ≥ 3:1.
- Controls: 28px minimum desktop target; plate wells 26px in top view.
- Focus: 2px accent-line ring, never color alone.
- State: selection uses background + text/outline, not only color.
- Motion: 120ms hover, 200ms selection, 320ms plate expand; ease-out/in-out. No continuous animation when idle.
- Reduced motion: switch to final state; do not animate data meaning.

## Review Checklist

- [ ] One accent per view; mapped wells are neutral.
- [ ] No card stack for repeated rows; hairlines + spacing carry structure.
- [ ] Body 14 black > section 11 gray > field label 11 gray.
- [ ] Mode tabs read as one segmented control.
- [ ] Inspector fields are grouped, not a database dump.
- [ ] Design / prepare / execute each have a different central object.
- [ ] Frozen Run snapshot is visible; missing Protocol/manifest is stated, not filled.
- [ ] Unknown and not-applicable states are text, not guessed values.
- [ ] 96-well expansion and selection work; no idle animation.
- [ ] The screen is understandable without reading the implementation notes.

## References

- Linear, “A calmer interface for a product in motion” (2026-03-12): navigation recedes, fewer separators, compact tabs, warmer gray.
- Vercel Geist: Materials (6/12px radii; floating-only shadows), Typography (12/13/14/16 scale), Button variants.
- Apple Human Interface Guidelines: Layout (hierarchy, alignment, grouping, progressive disclosure), Typography (macOS 13pt body, 10pt minimum).
- GitHub Primer: semantic color roles, spacing, typography tokens.
- Obsidian developer docs: 4px spacing grid; radius 4/8/12/16px; UI type 12/13/15/20px; line-height 1.3/1.5; semantic surface/border variables.
- Notion, “Updating the design of Notion pages” (2026-03-18): standardized spacing, adjacency-based compact lists, paragraph breathing room.
- Slint examples (gallery, fancy_demo): small composable `.slint` components, `TouchArea` roots, focus scopes, accessibility roles, state-based hover/selection.
- Google Labs `design.md` format: YAML token front matter + prose rationale, canonical section order.
