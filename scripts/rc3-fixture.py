"""Build the RC-3 CCK-8 acceptance fixture through the typed CLI.

Every executable design step is bound to an exact ProtocolBlock version, so the
Run must materialize both steps. Outputs ids to $DATA/rc3-ids.json.

Usage (from the repository root):
    python scripts/rc3-fixture.py
Environment overrides:
    ASTERLAB_DATA_DIR   fixture data directory (default target/rc3-data)
    LABWORKBENCH_CLI    labworkbench binary

The fixture only deletes a directory under the system temp directory, the
repository's target directory, or CARGO_TARGET_DIR. A non-empty directory must
contain a `.rc3-fixture` marker from a previous fixture run; otherwise the
script refuses to delete it.
"""
import json, os, pathlib, shutil, subprocess, sys, tempfile

ROOT = pathlib.Path(__file__).resolve().parent.parent
FIXTURE_MARKER = ".rc3-fixture"


def prepare_data_dir(raw: str) -> pathlib.Path:
    path = pathlib.Path(raw).expanduser()
    if path.is_symlink():
        sys.exit(f"refusing ASTERLAB_DATA_DIR {path}: symlinks are not allowed")
    resolved = path.resolve()
    temp_root = pathlib.Path(tempfile.gettempdir()).resolve()
    target_root = (ROOT / "target").resolve()
    cargo_root = None
    if os.environ.get("CARGO_TARGET_DIR"):
        cargo_root = pathlib.Path(os.environ["CARGO_TARGET_DIR"]).expanduser().resolve()
    allowed_roots = [temp_root, target_root] + ([cargo_root] if cargo_root else [])
    if not any(resolved.is_relative_to(root) for root in allowed_roots):
        sys.exit(
            f"refusing ASTERLAB_DATA_DIR {resolved}: use a path under {temp_root}, "
            f"{target_root}, or CARGO_TARGET_DIR"
        )
    forbidden = {pathlib.Path("/"), temp_root, ROOT, ROOT.parent, target_root}
    if cargo_root:
        forbidden.add(cargo_root)
    if resolved in forbidden:
        sys.exit(f"refusing ASTERLAB_DATA_DIR {resolved}: choose a fixture subdirectory")
    if resolved.exists():
        if not resolved.is_dir():
            sys.exit(f"refusing ASTERLAB_DATA_DIR {resolved}: not a directory")
        if any(resolved.iterdir()) and not (resolved / FIXTURE_MARKER).is_file():
            sys.exit(
                f"refusing to delete non-empty directory {resolved}: it has no "
                f"{FIXTURE_MARKER} marker"
            )
        shutil.rmtree(resolved)
    resolved.mkdir(parents=True, exist_ok=True)
    (resolved / FIXTURE_MARKER).write_text("rc3-fixture\n", encoding="utf-8")
    return resolved


DATA = prepare_data_dir(os.environ.get("ASTERLAB_DATA_DIR", str(ROOT / "target" / "rc3-data")))
CLI = pathlib.Path(
    os.environ.get(
        "LABWORKBENCH_CLI",
        str(pathlib.Path(os.environ.get("CARGO_TARGET_DIR", ROOT / "target")) / "debug" / "labworkbench"),
    )
)
if not CLI.is_file() and pathlib.Path(str(CLI) + ".exe").is_file():
    CLI = pathlib.Path(str(CLI) + ".exe")
if not CLI.is_file():
    sys.exit(
        f"labworkbench not found at {CLI}; build it with "
        "`cargo build --bin labworkbench` or set LABWORKBENCH_CLI"
    )
CLI = str(CLI)

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

ids_path = DATA / "rc3-ids.json"
with ids_path.open("w", encoding="utf-8") as f:
    json.dump({"data": str(DATA), "project": pid, "design": did, "revision": rev, "run": rid,
               "protocol": protocol, "seed_block": seed, "read_block": read}, f, indent=2)
print("ids:", ids_path)
