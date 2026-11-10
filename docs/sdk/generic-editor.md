# Generic Manifest-Driven Module Editor

`(*Document).UpdateModule(id string, properties map[string]any) (string, error)` rewrites
only the named module's fenced YAML body, preserves prose and other modules, re-parses
the source, and rejects invalid edits. It is the single source-rewrite path; no second
parser/writer exists.

View reports edits through `OnUpdateModule func(moduleID string, props map[string]any)` and
never writes Core or the document. The host applies the edit and reloads a revision.

The generic editor renders labeled fields from `Descriptor.Inputs`, hides `kind`, `id`,
and `schema`, and exposes one Save action. It renders only when the host grants
`document.edit` and `ReadOnly` is false. Document mode remains read-only.

Dilution and instrument_profile use this path when editing is allowed; otherwise their
read-only viewers render. Plate renderers are untouched.

Intentionally not generic: plate well assignment, derived calculations, reference
resolution, and any module needing bespoke UI remain module-specific.
