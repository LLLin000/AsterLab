#!/usr/bin/env bash
# SCI-01 field-ownership audit checks. Read-only except for stdout.
set -euo pipefail
cd "$(dirname "$0")/../../../.."

DOC=docs/sdk/science-field-ownership.md

python3 - "$DOC" <<'PY'
import json
import sys

path = sys.argv[1]
lines = open(path, encoding="utf-8").read().splitlines()

def section(start, end):
    out, on = [], False
    for line in lines:
        if line.startswith(start):
            on = True
            continue
        if on and line.startswith(end):
            break
        if on:
            out.append(line)
    return out

field_rows = [l for l in section("## 1. 字段归属表", "## 2.")
              if l.startswith("| ") and not l.startswith("|---")]
body = field_rows[1:]
print(f"field table header + rows: {len(body)}")
assert len(body) >= 40, "field table must have >= 40 rows"
bad = []
for line in body:
    cells = [c.strip() for c in line.strip().strip("|").split("|")]
    if len(cells) != 7:
        bad.append(("column_count", line))
        continue
    for index, name in ((2, "authority"), (3, "writer"), (4, "reader")):
        if not cells[index] or cells[index] in {"TBD", "-", "—"}:
            bad.append((name, line))
    if cells[0] not in {"Experiment", "Group", "Assay", "Timepoint", "Step", "Block",
                        "Protocol", "Run", "Sample", "Readout", "Notebook"}:
        bad.append(("object", line))
print("field table violations:", len(bad))
for kind, line in bad:
    print(f"  {kind}: {line}")
assert not bad, "field table has empty authority/writer/reader cells"

objects = {l.split("|")[1].strip() for l in body}
required = {"Experiment", "Group", "Assay", "Timepoint", "Step", "Block",
            "Run", "Sample", "Readout", "Notebook"}
print("objects covered:", ",".join(sorted(objects)))
assert required <= objects, f"missing objects: {sorted(required - objects)}"

# Readers must not claim design.map for fields the typed map does not expose.
import re
map_src = open("crates/asterlab-core/src/design_map.rs", encoding="utf-8").read()
def struct_block(name):
    m = re.search(r"pub struct " + name + r"\s*\{(.*?)\n\}", map_src, re.S)
    assert m, f"struct {name} not found"
    return m.group(1)
map_content = struct_block("DesignMapContent")
assay = struct_block("DesignAssay")
output = struct_block("PlannedOutputSummary")
for field in ("primary_endpoint", "sample_size_rationale", "planned_run_count"):
    assert field not in map_content, f"{field} unexpectedly in DesignMapContent"
assert "control_group_ids" not in assay, "control_group_ids unexpectedly in DesignAssay"
assert "root_binding_count" in output
for field in ("primary_endpoint", "sample_size_rationale", "planned_run_count",
              "control_group_ids"):
    rows = [l for l in body if f"`{field}`" in l]
    assert rows, f"missing field row for {field}"
    for row in rows:
        assert "design.get" in row, f"reader for {field} must be design.get: {row}"
root_rows = [l for l in body if "root_bindings" in l and "planned_outputs" in l]
assert root_rows and all("design.get" in l and "root_binding_count" in l for l in root_rows)
print("design.map coverage: raw-only fields point at design.get")

for heading in ("## 2. 数据流表", "## 3. 冲突与重复清单",
                "## 4. 模块目录与契约陈旧事实（供 DOC-05）", "## 5. 未决问题"):
    assert heading in lines, f"missing section {heading}"
print("sections: data-flow / conflicts / stale-facts / open-questions present")

text = "\n".join(lines)
print("authority mentions:", text.count("权威"))
assert text.count("权威") >= 10

manifest = json.load(open("prototypes/mygo-run/internal/asterdoc/spec/modules.json"))
mods = manifest["modules"]
print("manifest modules:", len(mods))
print("manifest permissions:", sorted({p for m in mods for p in m.get("permissions", [])}))
print("manifest renderer surfaces:", sorted({k for m in mods for k in m.get("renderers", {})}))
assert len(mods) == 16
assert all("run" in m.get("renderers", {}) for m in mods)
assert {p for m in mods for p in m.get("permissions", [])} == {"document.edit"}
print("manifest facts: OK")
PY

echo "--- stale-fact greps"
grep -n "15 个已注册模块\|注册 15 个" docs/sdk/module-catalog-v1.md
grep -n "run.readout\|notebook.append" docs/sdk/module-contract-v1.md
grep -n "renderers.design.*document.*static" docs/sdk/module-contract-v1.md || true
grep -n "not implemented" docs/sdk/module-contract-v1.md
grep -n "ordered_steps\|parameters_ready\|不是已实现代码" docs/contracts/core-v1.md
grep -n "let occurrences = Vec::new()" crates/asterlab-core/src/run_workspace.rs
grep -n "rna_reagent_usage" docs/sdk/module-catalog-v1.md

echo "field-check: PASS"
