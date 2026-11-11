# Workflow hierarchy UI proposal — Track C

Design only. No nested Canvas, Run tree, or Agent write command is implemented
in this round.

## 1. Design Canvas

- Keep the root canvas flat by default. A composite step card shows
  `RNA 提取 · 10 个操作` and an `展开` control.
- `展开` shows a bounded local preview (for example the first 3–4 child
  operations) inside the root canvas; it does not expand the entire subtree.
- `进入` focuses the canvas on `RT-qPCR / RNA 提取`; the breadcrumb returns to
  the root with the previous zoom and pan preserved.
- Expanded state, focus path, zoom, and pan live in the workspace session, not
  in AsterML or the frozen Run.
- Editing a child in the focused view edits the referenced ProtocolBlock
  definition through the existing versioned protocol commands, not a copied
  child list.

## 2. Run workspace

- The left rail becomes a tree: parent step rows show `3/8`, state, and an
  anomaly count; clicking a parent expands its child operations.
- The right side always focuses one executable child operation: instruction,
  resolved parameters, checks, recording, next action.
- Parent completion is derived from children. A parent cannot be marked
  complete while a child is pending or blocked.
- The same right-side detail rules as Track A apply; hierarchy changes
  navigation only, not the execution surface.

## 3. Agent permissions

- Read: inspect the whole tree, definitions, uses, occurrences, and status.
- Edit a shared definition: create a new ProtocolBlock version and a new
  parent block/protocol/design revision; never mutate a frozen Run or another
  instance.
- Edit a single use: only if a future instance-level binding exists; v1 has no
  such command.
- The Agent must show the target `definitionRef`, `use.path`, and affected
  occurrences before writing.

## 4. Out of scope

- Arbitrary nested DAGs, conditional branches, loops, and a second workflow
  engine.
- A new event store or a copied child-step tree in AsterDoc.
- Cross-version write commands before the Track B identity contract is stable.
