# AsterLab Native UI — Requirements and Acceptance

This file preserves the product brief and the P1 spike boundary. A technically working window is not accepted as a successful UI unless it also meets the interaction and visual criteria below.

## Product intent

- AsterLab is a lightweight, quiet, refined native research workstation, inspired by Things 3—not a direct HTML-to-Slint port.
- Priorities: low idle CPU and memory, responsive direct manipulation, no unnecessary resident processes or continuous redraw, and cross-platform support. Windows first; macOS/Linux later.
- Stack decision: Rust + Slint, Winit, FemtoVG as the primary renderer with software rendering as a measured alternative, SQLite/rusqlite retained, existing CLI and Web preserved during migration.
- Commercial use is allowed under Slint’s applicable terms; attribution/licensing must be recorded and surfaced appropriately. Mobile targets are possible, but require dedicated narrow-screen, touch, safe-area, keyboard, and packaging work. Cross-device data synchronization is a separate feature, not supplied by Slint.

## Workspace and interaction model

- Use one workspace with a sparse left navigation, an elastic central work surface, a contextual right inspector, and a compact top context bar. Desktop reference widths: navigation 200–220 px; inspector 280–320 px; top bar 48–56 px.
- The central experiment canvas is the primary surface. It visualizes groups, assays, timepoints, and sample relationships as separate scientific concepts. It is not a stack of generic information cards.
- The inspector follows the selected object (assay, material, plate, or well); it must distinguish recorded values from missing or unresolved values.
- Selection, expansion, and preview transitions change presentation only. They must not create execution events or mutate scientific data.
- Full product navigation may include Today, Projects, Schedule, Method Library, and Tools. Today is a task list, not a dashboard of metric cards. The method library separates reusable step modules from complete experimental protocols. Execution is step-centered. These are product direction, not P1 implementation scope.

## Scientific integrity

- Group is not assay; assay is not method; an experiment/protocol is not a reusable step module. Keep the terms and their UI placement distinct.
- Draw links only when the stored model explicitly supports them. Do not infer shared physical samples, well mappings, source relationships, completion, or experimental parameters.
- Unknown, absent, or unresolved fields remain visibly unknown; do not fill them with plausible defaults.
- Plate coordinates come from the stored plate dimensions. Only explicit `Well` records count as mapped; every other coordinate is an unmapped/ unrecorded position, not a sample assignment.
- P1 opens an explicitly supplied isolated database copy read-only. It does not create/migrate the production database, change the schema, rewrite old snapshots/events, or duplicate core validation.

## Visual language and motion

- Use restrained warm/neutral surfaces, readable system typography, subtle selection treatment, consistent spacing, and a small number of accents. Purple may be reserved for selection/primary action after contrast review.
- Type scale reference: body 14–15 px; key headings 18–22 px; secondary labels 12–13 px. Maintain clear hierarchy; avoid dense concatenated labels and large empty list/card regions.
- Custom visual controls must feel deliberate and consistent; default platform buttons are not a substitute for the intended visual system.
- Animations are brief, state-driven, and idle-static: page changes 180–240 ms; panel changes 220–300 ms; plate expansion 300–420 ms. Respect reduced-motion preferences where available.
- P1 includes `PlateView`: compact thumbnail → expanding → expanded top view → selected well. Expanded positions and labels must match real coordinates. No persistent 3D or continuous particle animation.
- `PipetteView` and `ExperimentFlow` are later visual components, not requirements for the P1 spike.

## P1 scope and acceptance

1. Preserve the existing CLI, Web, SQLite schema, and domain semantics.
2. Add a separate Slint desktop prototype; do not replace the Web UI.
3. Load an isolated L929 database copy through a read-only path.
4. Implement project/design navigation, a real experiment canvas, assay selection, and a contextual inspector.
5. Implement the 96-well thumbnail-to-top-view interaction and accurate coordinate selection.
6. Show only confirmed mappings and preserve unknown values.
7. Keep all animation UI-only; it must not write database facts or execution events.
8. Test FemtoVG and software rendering on the same machine, including Chinese text.
9. Record startup, idle CPU, process-tree memory/working set, and interaction performance against the Web baseline; treat targets as budgets, not framework guarantees.
10. Deliver real screenshots, an interaction recording, performance data, and technical risks.

Initial performance budgets (to measure, not assume): cold start to interactive ≤1.5 s; idle CPU near 0%; private memory target ≤100 MB; common selection response P95 ≤100 ms excluding disk I/O; 96-well interaction target 60 FPS. Keep accuracy and reliability ahead of the budget.

## Explicit non-goals for P1

- No broad application migration, new mobile client, cloud sync, database migration, schema rewrite, or CLI-as-GUI subprocess.
- No removal of the existing Web UI before a later migration acceptance.
- No invented plate map, sample source, assay/group relationship, or execution state.

## Review gate for this spike

Reject a build that merely compiles or checks off controls while the central canvas, visual hierarchy, or plate interaction remains a generic scaffold. Review a real window screenshot and exercise the selection path before accepting the UI.