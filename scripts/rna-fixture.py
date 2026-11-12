"""Build the RT-qPCR / RNA-extraction fixture for the G1 hierarchy gate.

Four executable design steps, each bound to its own ProtocolBlock version, and a
root protocol that contains all four. The RNA block carries the ten documented
sub-operations, so the Design drill-down and the Run tree both have real depth.

Usage (from the repository root):
    python scripts/rna-fixture.py
Environment overrides:
    ASTERLAB_DATA_DIR   fixture data directory (default target/rna-case-data)
    LABWORKBENCH_CLI    labworkbench binary
"""
import json, os, pathlib, shutil, subprocess, sys

ROOT = pathlib.Path(__file__).resolve().parent.parent
DATA = pathlib.Path(os.environ.get("ASTERLAB_DATA_DIR", ROOT / "target" / "rna-case-data"))
CLI = os.environ.get(
    "LABWORKBENCH_CLI",
    str(pathlib.Path(os.environ.get("CARGO_TARGET_DIR", ROOT / "target")) / "debug" / "labworkbench"),
)
if not os.path.exists(CLI):
    sys.exit(
        f"labworkbench not found at {CLI}; build it with "
        "`cargo build --bin labworkbench` or set LABWORKBENCH_CLI"
    )
if not os.path.exists(CLI) and os.path.exists(CLI + ".exe"):
    CLI += ".exe"

shutil.rmtree(DATA, ignore_errors=True)
DATA.mkdir(parents=True, exist_ok=True)
env = dict(os.environ, LABWORKBENCH_DATA_DIR=str(DATA))


def run(tool, payload):
    proc = subprocess.run([CLI, tool, "--input", "-"], input=json.dumps(payload).encode(), capture_output=True, env=env)
    out = proc.stdout.decode("utf-8", "replace").strip()
    try:
        value = json.loads(out)
    except Exception:
        print("NON-JSON", tool, out[:400], proc.stderr.decode("utf-8", "replace")[:400])
        sys.exit(1)
    if not value.get("ok"):
        print("FAIL", tool, json.dumps(value.get("error"), ensure_ascii=False))
        sys.exit(1)
    return value["result"]


RNA_STEPS = [
    ("step-01", "01 收集细胞", "收集约 5×10^6 细胞，弃上清。"),
    ("step-02", "02 裂解与匀浆", "加入裂解缓冲液，匀浆后离心。"),
    ("step-03", "03 加乙醇", "加入等体积 70% 乙醇，混匀。"),
    ("step-04", "04 上柱结合", "将混合液过柱，离心弃流出液。"),
    ("step-05", "05 Wash I", "加入 Wash Buffer I，离心弃流出液。"),
    ("step-06", "06 Wash II 第一次", "加入 Wash Buffer II，离心弃流出液。"),
    ("step-07", "07 Wash II 第二次", "再次加入 Wash Buffer II，离心弃流出液。"),
    ("step-08", "08 干柱离心", "空柱离心 2 min，去除残留乙醇。"),
    ("step-09", "09 RNA 洗脱", "加入无 RNase 水，离心收集 RNA。"),
    ("step-10", "10 RNA 质检与保存", "测定浓度与 A260/A280，-80 °C 保存。"),
]


def block(project, title, summary, steps):
    return run("protocol.block.create", {
        "project_id": project, "title": title, "summary": summary,
        "parameter_slots": [], "steps": steps,
    })["version"]["id"]


project = run("project.create", {"name": "G1 hierarchy gate"})
pid = project["record"]["id"]

rna = block(pid, "PureLink RNA extraction", "十步 RNA 提取块",
            [{"kind": "text", "use_id": use_id, "title": title, "instruction": instruction}
             for use_id, title, instruction in RNA_STEPS])
qc = block(pid, "RNA QC", "浓度与纯度", [
    {"kind": "text", "use_id": "qc", "title": "RNA 质检", "instruction": "记录 RNA 浓度与 A260/A280。"}])
rt = block(pid, "Reverse transcription", "逆转录", [
    {"kind": "text", "use_id": "rt", "title": "逆转录", "instruction": "RNA 逆转录为 cDNA。"}])
qpcr = block(pid, "qPCR", "扩增与检测", [
    {"kind": "text", "use_id": "qpcr", "title": "qPCR 扩增", "instruction": "配置反应并运行 qPCR，记录 Ct。"}])

root = block(pid, "RT-qPCR protocol root", "RNA 提取 + 质控 + 逆转录 + qPCR", [
    {"kind": "block_use", "use_id": "rna", "block_version_id": rna, "bindings": []},
    {"kind": "block_use", "use_id": "qc", "block_version_id": qc, "bindings": []},
    {"kind": "block_use", "use_id": "rt", "block_version_id": rt, "bindings": []},
    {"kind": "block_use", "use_id": "qpcr", "block_version_id": qpcr, "bindings": []},
])
protocol = run("protocol.create", {
    "project_id": pid, "title": "RT-qPCR protocol", "root_block_version_id": root,
})["version"]["id"]

source = f"""# RT-qPCR parent workflow — containment test

```asterlab
kind: experiment
id: exp-rtqpcr
schema: 1
title: RT-qPCR parent workflow
objective: Test whether a parent workflow can contain a reusable RNA extraction block.
experimental_unit: sample
primary_endpoint: Relative expression
endpoint_method: RT-qPCR
endpoint_unit: fold change
sample_size_rationale: Four virtual samples; UI validation only.
planned_run_count: 1
planned_run_relationship: independent
```

```asterlab
kind: group
id: g-rtqpcr
schema: 1
name: RT-qPCR
role: control
condition: RNA extraction block reused from the PureLink protocol block
independent_n: 4
technical_repeats: 1
```

```asterlab
kind: assay
id: assay-rtqpcr
schema: 1
title: RT-qPCR
endpoint: Relative expression
method: RT-qPCR
unit: fold change
group_refs: [g-rtqpcr]
protocol_version_id: {protocol}
```

```asterlab
kind: reference
id: ref-rna-block
schema: 1
ref: {rna}
view: protocol_block
```

```asterlab
kind: reference
id: ref-qc-block
schema: 1
ref: {qc}
view: protocol_block
```

```asterlab
kind: reference
id: ref-rt-block
schema: 1
ref: {rt}
view: protocol_block
```

```asterlab
kind: reference
id: ref-qpcr-block
schema: 1
ref: {qpcr}
view: protocol_block
```

```asterlab
kind: sample
id: rt-s01
schema: 1
sample_id: RNA-S01
group_ref: g-rtqpcr
amount: {{value: 1, unit: mL}}
```

```asterlab
kind: sample
id: rt-s02
schema: 1
sample_id: RNA-S02
group_ref: g-rtqpcr
amount: {{value: 1, unit: mL}}
```

```asterlab
kind: sample
id: rt-s03
schema: 1
sample_id: RNA-S03
group_ref: g-rtqpcr
amount: {{value: 1, unit: mL}}
```

```asterlab
kind: sample
id: rt-s04
schema: 1
sample_id: RNA-S04
group_ref: g-rtqpcr
amount: {{value: 1, unit: mL}}
```

```asterlab
kind: step
id: step-rna-extract
schema: 3
title: 01 RNA 提取
execution: executable
assay: assay-rtqpcr
path: root/rna-extraction
order: 1
uses:
  - {{ref: ref-rna-block, role: other}}
  - {{ref: rt-s01, role: sample}}
  - {{ref: rt-s02, role: sample}}
  - {{ref: rt-s03, role: sample}}
  - {{ref: rt-s04, role: sample}}
```

```asterlab
kind: step
id: step-rna-qc
schema: 3
title: 02 RNA 质控
execution: executable
assay: assay-rtqpcr
path: root/rna-qc
order: 2
uses:
  - {{ref: ref-qc-block, role: other}}
  - {{ref: rt-s01, role: sample}}
  - {{ref: rt-s02, role: sample}}
  - {{ref: rt-s03, role: sample}}
  - {{ref: rt-s04, role: sample}}
```

```asterlab
kind: step
id: step-rt
schema: 3
title: 03 逆转录
execution: executable
assay: assay-rtqpcr
path: root/reverse-transcription
order: 3
uses:
  - {{ref: ref-rt-block, role: other}}
  - {{ref: rt-s01, role: sample}}
  - {{ref: rt-s02, role: sample}}
  - {{ref: rt-s03, role: sample}}
  - {{ref: rt-s04, role: sample}}
```

```asterlab
kind: step
id: step-qpcr
schema: 3
title: 04 qPCR 扩增及检测
execution: executable
assay: assay-rtqpcr
path: root/qpcr
order: 4
uses:
  - {{ref: ref-qpcr-block, role: other}}
  - {{ref: rt-s01, role: sample}}
  - {{ref: rt-s02, role: sample}}
  - {{ref: rt-s03, role: sample}}
  - {{ref: rt-s04, role: sample}}
```

```asterlab
kind: workflow
id: wf-rtqpcr
schema: 1
steps: [step-rna-extract, step-rna-qc, step-rt, step-qpcr]
edges:
  - {{from: step-rna-extract, to: step-rna-qc}}
  - {{from: step-rna-qc, to: step-rt}}
  - {{from: step-rt, to: step-qpcr}}
business_order: [step-rna-extract, step-rna-qc, step-rt, step-qpcr]
```
"""

design = run("design.create", {"project_id": pid, "design": {"title": "RT-qPCR hierarchy"}})
did = design["design"]["id"]
doc = run("asterdoc.save", {"design_id": did, "source": source, "expected_revision": 0})
rev = doc["revision"]

assess = run("asterdoc.assess", {"design_id": did, "revision": rev})
print("assess:", assess["state"], json.dumps(assess["errors"], ensure_ascii=False))
if assess["state"] == "incomplete":
    sys.exit(2)

created = run("asterdoc.create_run", {"design_id": did, "revision": rev, "title": "RT-qPCR run"})
rid = created["id"]
ws = run("run.workspace", {"id": rid})
print("run:", rid, "tasks:", len(ws["tasks"]))
for task in ws["tasks"]:
    print("  -", task["step_path"])

ids = {"data": str(DATA), "project": pid, "design": did, "revision": rev, "run": rid,
       "protocol": protocol, "rna_block": rna}
(DATA.parent / "rna-ids.json").write_text(json.dumps(ids, indent=2), encoding="utf-8")
print("ids:", DATA.parent / "rna-ids.json")
