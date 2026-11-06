# AsterLab Component Module Contract

Each component is a self-contained visual module. Do not edit `web/app.js`,
`web/view.html`, or another component's files. Do not create a second global
state store.

## Files

```text
web/components/<name>.css
web/components/<name>.js
```

CSS must be scoped under `.cmp-<name>` and must use the shared tokens from
`web/components/tokens.css`. Do not override global `body`, `.card`, or `.rail`.

JS registers one renderer:

```js
window.AsterUI.register("definition.id@version", {
  renderDetail(instance, ctx) { return "<div class=\"card cmp-wellplate\">…</div>"; },
  mount(root, instance, ctx) { return () => { /* cleanup */ }; },
  renderCompact(instance, ctx) { return "…"; } // optional
});
```

`renderDetail` returns a full card whose root has:

```html
data-instance="<instance_id>"
data-component="<definition.id>"
```

The host inserts it into the grid and calls `mount`. `mount` returns a cleanup
function. The host calls cleanup before replacing the detail view.

## Context

```ts
ctx = {
  esc(value): string,
  title(instance): string,
  definition(instance): string,
  section: string,
  plates: instance[],
  root: ComponentInstance,
  find(id): ComponentInstance | null,
  commit(operations, label): Promise<void>,
  loadPlate(instanceId): Promise<PlateProjection>,
  preview(operations): Promise<ProposalPreview>,
  drafts: {
    get(id): any | undefined,
    set(id, value): void,
    clear(id): void,
    markDirty(id): void
  }
}
```

Components own their local editing UI and call `ctx.commit` only for explicit
Save actions. No component may write SQLite directly or call `fetch` for
ViewDocument mutations. Read-only fetches should use `ctx` helpers.

## Visual rules

- Follow `tokens.css`; no arbitrary colors, radii, or font sizes.
- Card header: title 15/20 semibold; metadata 12/16; body 13/18.
- Controls: minimum 32px high; focus ring visible.
- Use text + icon for unresolved/error; never color alone.
- Cards fill their grid cell. Content may scroll inside the card.
- Use icons from `/icons/*.svg` or inline simple geometry; do not add a new icon library.
- No decorative illustrations, no gradients, no large shadows.

## Product semantics

- A visual selection is not a scientific fact.
- WellPlate selection is local to the card until explicitly submitted.
- Table edits are drafts until Save.
- Calculator inputs must show value + unit + unresolved state.
- Never infer a value from a table cell or a plate position.

## Deliverable

Return: files written, component API used, visual states covered, and any host
integration changes needed. Do not run a full repo rewrite.
