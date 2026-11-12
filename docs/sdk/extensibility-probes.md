# Extensibility probes

Probe date: 2026-11-12. Binary: `target/debug/labworkbench.exe` (Rust Core CLI). Command: `asterdoc.validate --input <json>`.

## 1. RNA reagent usage calculator

Source: `target/probe/rna_reagent.md`

```markdown
# RNA reagent usage calculator probe

```asterlab
kind: experiment
id: probe-exp
schema: 1
title: probe
```

```asterlab
kind: dilution
id: rna-reagent-usage
schema: 1
stock:
  value: 100
  unit: ug/mL
target:
  value: 2
  unit: ug/mL
final_volume:
  value: 50
  unit: uL
```

```asterlab
kind: material
id: rna-reagent
schema: 1
name: RNA reagent
role: control
condition: working
concentration:
  value: 2
  unit: ug/mL
source: lab_imported
```
```

CLI transcript (trimmed):

```text
{"ok": true, "result": {"valid": true, "compiled": true, "error": null}, "tool": "asterdoc.validate"}
```

Rejection seen while probing:

- `material.role must be one of control, treatment` — `src/cli.rs:2720-2733` (enum port validation). Fix: manifest-only (use allowed enum value).

## 2. RT-qPCR Ct record + display

Source: `target/probe/rtqpcr.md`

```markdown
# RT-qPCR Ct record + display probe

```asterlab
kind: experiment
id: probe-exp
schema: 1
title: probe
```

```asterlab
kind: group
id: g-control
schema: 1
name: control
```

```asterlab
kind: group
id: g-treatment
schema: 1
name: treatment
```

```asterlab
kind: assay
id: rtqpcr-assay
schema: 1
title: RT-qPCR Ct
endpoint: Ct
method: qPCR
unit: Ct
group_refs: [g-control, g-treatment]
timepoints: [T0]
technical_replicates: 3
```

```asterlab
kind: measurement_profile
id: rtqpcr-profile
schema: 1
endpoint: Ct
unit: Ct
readout_schema:
  raw: Ct
  derived: dCt
calculator_refs: [dct.v1]
```

```asterlab
kind: sample
id: sample-1
schema: 1
sample_id: S1
group_ref: g-control
amount:
  value: 1
  unit: uL
```
```

CLI transcript (trimmed):

```text
{"ok": true, "result": {"valid": true, "compiled": true, "error": null}, "tool": "asterdoc.validate"}
```

Rejections seen while probing:

- `group module needs name` — `src/cli.rs:2898-2904` (required-field validation). Fix: manifest-only.
- (earlier) `assay ... references missing group ...` — `src/cli.rs:2458` (reference resolution). Fix: manifest-only.

## 3. Image / local-file association

Source: `target/probe/image_assoc.md`

```markdown
# Image / local-file association probe

```asterlab
kind: experiment
id: probe-exp
schema: 1
title: probe
```

```asterlab
kind: instrument_profile
id: imaging-profile
schema: 1
class: microscope
capabilities:
  - name: brightfield
source: unknown
evidence:
  - uri: file:///C:/data/image.png
    media_type: image/png
    label: sample image
```

```asterlab
kind: reference
id: image-ref
schema: 1
ref: imaging-profile
view: image
```
```

CLI transcript (trimmed):

```text
{"ok": true, "result": {"valid": true, "compiled": true, "error": null}, "tool": "asterdoc.validate"}
```

Rejection seen while probing:

- `a design document needs exactly one experiment module` — `src/cli.rs:3268` (compile/design validation). Fix: manifest-only (add experiment module).

## Change-impact table

| Module probe | Files that must change | Class |
| --- | --- | --- |
| RNA reagent usage calculator | `prototypes/mygo-run/internal/asterdoc/spec/modules.json` (new kind or fields); `internal/asterdoc/spec.go`, `modules.go`; `src/cli.rs` validator + calculator | new schema + Go renderer + Rust validator |
| RT-qPCR Ct record + display | record store transaction in `src/cli.rs`; Go renderers for tabular/raw-vs-derived; `modules.json` if new kind | Core store-transaction + Go renderer |
| Image / local-file association | Go renderer for artifact preview; `modules.json` evidence reuse; no Core change if using `instrument_profile` | Go renderer only |

## Verdict

All three probes pass `asterdoc.validate` and compile today with zero Core changes, because they are compositions of already-shipped module kinds. None of them is a new module, and none delivers domain-specific rendering, calculation or record capture without additional Go renderer, Rust validator or Core store-transaction work.
