"""Build the RC-3 CCK-8 acceptance fixture through the typed CLI.

Every executable design step is bound to an exact ProtocolBlock version, so the
Run must materialize both steps. Outputs ids to $DATA/../rc3-ids.json.

Usage (from the repository root):
    python scripts/rc3-fixture.py
Environment overrides:
    ASTERLAB_DATA_DIR   fixture data directory (default target/rc3-data)
    LABWORKBENCH_CLI    labworkbench binary
"""
import json, os, pathlib, shutil, subprocess, sys

ROOT = pathlib.Path(__file__).resolve().parent.parent
DATA = pathlib.Path(os.environ.get("ASTERLAB_DATA_DIR", ROOT / "target" / "rc3-data"))
CLI = os.environ.get(
    "LABWORKBENCH_CLI",
    str(pathlib.Path(os.environ.get("CARGO_TARGET_DIR", ROOT / "target")) / "debug" / "labworkbench"),
)
shutil.rmtree(DATA, ignore_errors=True)
DATA.mkdir(parents=True, exist_ok=True)
if not os.path.exists(CLI):
    sys.exit(
        f"labworkbench not found at {CLI}; build it with "
        "`cargo build --bin labworkbench` or set LABWORKBENCH_CLI"
    )
if not os.path.exists(CLI) and os.path.exists(CLI + ".exe"):
    CLI += ".exe"

env = dict(os.environ, LABWORKBENCH_DATA_DIR=str(DATA))

def run(tool, payload):
    p = subprocess.run([CLI, tool, "--input", "-"], input=json.dumps(payload).encode(),
                       capture_output=True, env=env)
    out = p.stdout.decode("utf-8", "replace").strip()
    try:
        v = json.loads(out)
    except Exception:
        print("NON-JSON", tool, out[:400], p.stderr.decode("utf-8","replace")[:400]); sys.exit(1)
    if not v.get("ok"):
        print("FAIL", tool, json.dumps(v.get("error"), ensure_ascii=False)); sys.exit(1)
    return v["result"]

project = run("project.create", {"name": "RC-3 acceptance"})
pid = project["record"]["id"]

seed = run("protocol.block.create", {
    "project_id": pid, "title": "CCK-8 seed", "summary": "单步接种块",
    "parameter_slots": [],
    "steps": [{"kind": "text", "use_id": "seed", "title": "接种细胞",
               "instruction": "每孔接种 100 µL 细胞悬液，37 °C 过夜。"}],
})["version"]["id"]

read = run("protocol.block.create", {
    "project_id": pid, "title": "CCK-8 readout", "summary": "读数块",
    "parameter_slots": [],
    "steps": [
        {"kind": "text", "use_id": "incubate", "title": "孵育",
         "instruction": "加入 CCK-8 试剂，37 °C 孵育 2 h。"},
        {"kind": "text", "use_id": "read", "title": "读取 OD450",
         "instruction": "读取 450 nm 吸光度。",
         "checks": [{"check_id": "blank", "label": "确认空白对照", "blocking": True}]},
    ],
})["version"]["id"]

root = run("protocol.block.create", {
    "project_id": pid, "title": "CCK-8 protocol root", "summary": "接种 + 读数",
    "parameter_slots": [],
    "steps": [
        {"kind": "block_use", "use_id": "seed", "block_version_id": seed, "bindings": []},
        {"kind": "block_use", "use_id": "readout", "block_version_id": read, "bindings": []},
    ],
})["version"]["id"]

protocol = run("protocol.create", {
    "project_id": pid, "title": "CCK-8 protocol", "root_block_version_id": root,
})["version"]["id"]

source = f"""# CCK-8 验收

```asterlab
kind: experiment
id: exp-cck8
schema: 1
title: CCK-8 viability acceptance
objective: verify two executable steps materialize
experimental_unit: well
```

```asterlab
kind: group
id: g-control
schema: 1
name: Control
role: control
```

```asterlab
kind: group
id: g-material
schema: 1
name: Material
role: treatment
```

```asterlab
kind: assay
id: assay-cck8
schema: 1
title: CCK-8
endpoint: OD450
unit: AU
group_refs: [g-control, g-material]
timepoints: [D1]
protocol_version_id: {protocol}
```

```asterlab
kind: plate
id: plate-96
schema: 1
format: 96
assay: assay-cck8
groups:
  g-control: [A1, A2]
  g-material: [B1, B2]
```

```asterlab
kind: reference
id: ref-seed
schema: 1
ref: {seed}
view: protocol_block
```

```asterlab
kind: reference
id: ref-readout
schema: 1
ref: {read}
view: protocol_block
```

```asterlab
kind: step
id: step-seed
schema: 3
title: 接种细胞
execution: executable
assay: assay-cck8
path: cck8/seed
order: 1
timepoints: [D1]
uses:
  - {{ref: ref-seed, role: other}}
```

```asterlab
kind: step
id: step-readout
schema: 3
title: CCK-8 检测
execution: executable
assay: assay-cck8
path: cck8/readout
order: 2
timepoints: [D1]
uses:
  - {{ref: ref-readout, role: other}}
```

```asterlab
kind: workflow
id: wf-cck8
schema: 1
steps: [step-seed, step-readout]
edges:
  - {{from: step-seed, to: step-readout}}
business_order: [step-seed, step-readout]
```
"""

design = run("design.create", {"project_id": pid, "design": {"title": "CCK-8 acceptance"}})
did = design["design"]["id"]
doc = run("asterdoc.save", {"design_id": did, "source": source, "expected_revision": 0})
rev = doc["revision"]

assess = run("asterdoc.assess", {"design_id": did, "revision": rev})
print("assess:", assess["state"], json.dumps(assess["errors"], ensure_ascii=False))
if assess["state"] == "incomplete":
    sys.exit(2)

created = run("asterdoc.create_run", {"design_id": did, "revision": rev, "title": "CCK-8 RC-3 run"})
rid = created["id"]

ws = run("run.workspace", {"id": rid})
print("run:", rid)
print("protocol tasks:", len(ws["tasks"]))
for t in ws["tasks"]:
    print("  -", t["step_path"], "| timepoint:", t.get("timepoint"))

json.dump({"data": str(DATA), "project": pid, "design": did, "revision": rev, "run": rid,
           "protocol": protocol, "seed_block": seed, "read_block": read},
          open(DATA.parent / "rc3-ids.json", "w"), indent=2)
