# AsterML Foundation v1

- Date: 2026-11-10
- Status: foundation contract candidate, frozen for implementation. Module-specific fields and UI layouts are not frozen.
- Baseline: repository working tree at commit `271eeff`; no user study or real-instrument validation is claimed.
- Evidence: source paths and tests are named inline. External standards are cited at the claim.

## 1. What is frozen, and what is not

Frozen now:

1. the `asterlab` Markdown fence and the `kind` / `id` / `schema` envelope;
2. the fifteen canonical Foundation port types plus the legacy compatibility aliases;
3. the named UCUM subset and the rule that quantities preserve the authored value and unit;
4. typed references with a target kind and document-local scope;
5. duration, timepoint and relative-time shapes;
6. external artifact references;
7. schema-version and unknown-field policy;
8. the diagnostic shape;
9. the unified module-contract dimensions in [module-contract-v1.md](module-contract-v1.md).

Not frozen:

- fields specific to a scientific domain (`step@1`, `instrument_profile@1`, and future modules);
- professional editor layouts and static/PDF layout;
- device commands and hardware control;
- template instantiation;
- dynamic third-party loading.

This follows the layered-freeze proposal: freeze the language and extension mechanism, then let professional modules evolve behind versioned contracts.

## 2. Document syntax

An AsterDoc is ordinary Markdown. Exactly one fenced block declares one module:

````markdown
```asterlab
kind: plate
id: plate-d1
schema: 1
format: 96
groups:
  g-control: [A1, A2, A3]
```
````

Rules enforced by the shared manifest and both validators:

- the fence language is exactly `asterlab`;
- the YAML body must be a mapping;
- `kind` is non-empty and selects a registered module;
- `id` is non-empty and unique within the document;
- `schema` is a positive integer;
- an unknown kind or schema is rejected; there is no generic editable fallback;
- parsing never rewrites prose or unrelated fences;
- a semantic edit replaces only the targeted fence body and reparses the whole document.

`kind` is stable and may use a dot namespace for future modules (`lab.dilution`). Existing modules use short names for compatibility.

### 2.1 Unknown fields

Foundation v1 preserves unknown fields in the authored YAML; it does not silently drop them. They are not interpreted unless the matching module manifest declares them. A future strict mode may report them as advisory or blocking, but v1 does not invent that rule retroactively.

Unknown **modules** and unknown **module versions** are different: those are rejected, because rendering or compiling them could misrepresent a scientific object.

### 2.2 Duplicate fields

The YAML parser is the authority for duplicate mapping keys. Implementations must not merge duplicate scientific fields silently. Current Go and Rust validation paths share the same module manifest, but a formal duplicate-key diagnostic is still a compatibility gap (see [compatibility-and-gaps.md](compatibility-and-gaps.md)).

## 3. Foundation data types

The manifest declares ports with the following types. JSON Schema 2020-12 is the structural reference; scientific cross-module rules stay in the validators.

| Type | Shape | Validation and boundary |
| --- | --- | --- |
| `string` | text | UTF-8 text; empty allowed unless `required` |
| `enum` | text from `values` | value must be listed |
| `boolean` | `true` / `false` | no truthy strings |
| `integer` | integer | no fractional values |
| `decimal` | number | finite numeric value |
| `quantity` | `{value, unit}` | unit must be in the Foundation v1 registry; `dimension` constrains it |
| `reference` | non-empty string | `target` names the required module kind (or `module`) |
| `list` | array | `item_type` constrains items; reference lists are validated by target kind |
| `object` | mapping | structure is declared by the module's own rules |
| `table` | array of mappings | rows are objects; column semantics stay module-specific |
| `geometry` | mapping | spatial layout payload; plate formats use the existing fixed geometry rules |
| `artifact` | `{uri, media_type?, sha256?, label?}` | external reference only; inline `data:` is rejected |
| `duration` | `{value, unit}` | unit dimension must be `time` |
| `timepoint` | string or `{label, ...}` | planned label, not an actual timestamp |
| `relative_time` | `{anchor, offset}` | offset is a duration; anchor names the event |

Compatibility aliases remain accepted for existing documents: `int`→integer, `number`→decimal, `bool`→boolean, `string[]`→list of strings, `object-list`→list of objects, `well-map` is a plate-specific mapping. New modules should use the canonical names.

`quantity` never means "the number 120". It means a value with a unit. The authored value and unit stay in the document; a converted value is a derived value and must retain its calculator/version.

### 3.1 Named UCUM subset

`spec/units.json` is the shared registry. It covers:

- time: `s`, `min`, `h`, `d`;
- volume: `L`, `mL`, `uL`, `nL`;
- mass: `g`, `mg`, `ug`, `ng`, `kg`;
- amount: `mol`, `mmol`, `umol`, `nmol`;
- temperature: `Cel`;
- mass concentration: `g/L`, `mg/mL`, `mg/L`, `ug/mL`, `ug/L`, `ng/mL`, `ng/L`;
- amount concentration: `M`, `mol/L`, `mM`, `uM`, `nM`;
- length: `mm`, `cm`, `m`, `um`, `nm`.

This is a **named subset**, not all of UCUM. UCUM defines units by a numeric factor and a vector of base-unit exponents for unambiguous machine communication ([UCUM: About](https://ucum.org/about)). Expanding this registry is a deliberate versioned change. There is no implicit unit conversion in the document: conversion happens only in a calculator or derived output and produces a separate value.

### 3.2 Typed references

A reference is a string ID in the document, but its meaning is declared in the manifest:

```json
{"name": "group_refs", "type": "list", "item_type": "reference", "target": "group"}
```

Core validates that every non-empty reference resolves to an existing module of that kind in the same document. It does not guess from an ID prefix. The reference module's free `ref` field is intentionally **not** a typed reference in v1 because it can point at a record-store object outside the document; resolving record-store references is a separate adapter.

Future resolvers may add cross-document, resource-library and frozen-version references without changing each module's business logic.

### 3.3 Time

The four planned time meanings are kept distinct:

| Meaning | Foundation shape | Example |
| --- | --- | --- |
| Duration | `quantity` with time dimension | `{value: 120, unit: s}` |
| Timepoint | `timepoint` label | `D3`, `{label: D3}` |
| Relative offset | `relative_time` | `{anchor: seeding, offset: {value: 6, unit: h}}` |
| Planned window | module-specific fields, not yet a frozen type | start window |
| Actual timestamp | **not in the document** | Run execution event only |

Existing `timepoint: D3` string fields remain valid. A planned timepoint is not an actual execution time, and a relative window is not proof that the operation happened.

### 3.4 Artifacts

An artifact references external bytes:

```yaml
evidence:
  - uri: file:///lab/records/reader-manual.pdf
    media_type: application/pdf
    sha256: 0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef
```

Inline `data:` URIs are rejected. An artifact reference does not copy the file into the document and does not make it a measured result. Actual files belong to the record store or an external store with provenance.

## 4. Schema versions and compatibility

- A module's `schema` selects its structure version.
- A new `schema` version must not silently change the meaning of an old version.
- Old documents remain readable as long as their module kind and schema are still registered.
- Adding a module does not change existing module semantics.
- A breaking field change creates a new schema version and a migration rule; it never mutates an already-frozen Run.
- Existing fields such as `step@1` are not rewritten to express HUVEC material amounts; a future `step@2` or composable parameter module must carry that meaning.

There is no automatic migration in Foundation v1. The current compiler ignores modules it does not project; adding a renderable module therefore does not force a Core projection change.

## 5. Diagnostics

The shared diagnostic shape is defined in [foundation-v1.schema.json](../../prototypes/mygo-run/internal/asterdoc/spec/foundation-v1.schema.json):

```json
{
  "code": "REFERENCE_MISSING",
  "path": "assay-1.group_refs",
  "message": "references missing group \"g-control\"",
  "severity": "blocking",
  "affects": ["assay-1"],
  "next_action": "create the referenced group or remove the reference"
}
```

Severity levels:

- `blocking`: malformed structure, unknown module/schema, type or unit mismatch, missing required reference; the write is refused.
- `needs_confirmation`: a scientific choice or missing key value; a draft may save, but the dependent freeze/calculation/Run is blocked.
- `advisory`: method suggestion; never silently converted into a blocking rule.

Current implementation boundary: Go and Rust reject invalid documents with a stable CLI error code and a readable message. The structured diagnostic shape is defined and machine-checked, but the CLI error envelope still exposes `code` + `message` rather than the full diagnostic array. This is recorded as a gap, not presented as completed.

## 6. External standards: what is borrowed, what is not

| Standard | Borrowed here | Not claimed |
| --- | --- | --- |
| [JSON Schema 2020-12](https://json-schema.org/draft/2020-12) | `$defs`, object/array/number constraints, explicit schema dialect | JSON Schema does not perform cross-module scientific or reference validation. |
| [UCUM](https://ucum.org/about) | Unit codes and dimensional semantics | Foundation v1 validates a named subset; it is not a complete UCUM implementation. |
| [ISA](https://isa-specs.readthedocs.io/en/latest/isamodel.html) | Investigation/Study/Assay, Material/Process/Data separation | No full ISA-Tab/ISA-JSON import or export in this phase. |
| [W3C PROV-O](https://www.w3.org/TR/prov-o/) | Entity/Activity/Agent provenance framing | No RDF graph or complete PROV serialization. |
| [SiLA 2](https://sila-standard.com/standards/) | Feature/Command/Property model for a future Device SDK | No hardware control, discovery or SiLA server in Foundation v1. |
| [AnIML](https://www.animl.org/) | Analytical instrument data and audit-trail direction | No AnIML import/export in Foundation v1. |

The design rule is the same for all of them: borrow the semantic pattern in the appropriate SDK, and keep the Core document format lightweight. Do not force SiLA 2, AnIML or ISA into every AsterML document.

## 7. Implemented references

- Go: `prototypes/mygo-run/internal/asterdoc/{spec.go,units.go,asterdoc.go,registry.go,modules.go}`.
- Rust: `src/cli.rs` AsterDoc validation and compiler section.
- Shared: `spec/modules.json`, `spec/units.json`, `spec/foundation-v1.schema.json`.
- Tests: `prototypes/mygo-run/internal/asterdoc/asterdoc_test.go`, `prototypes/mygo-run/asterdoc_store_test.go` (Go/Rust parity).
