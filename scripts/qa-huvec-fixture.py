"""Build the QA-HUVEC-01 gate fixture: HUVEC tube-formation design + run.

Appends a HUVEC project/design/run into an existing isolated data dir
(RT-qPCR side is built first by scripts/rna-fixture.py into the same dir).
Never touches product code; CLI seeding only.

Usage (from the repository root):
    LABWORKBENCH_DATA_DIR=D:/.ci-cache/qa-huvec-01 LABWORKBENCH_CLI=<exe> python scripts/qa-huvec-fixture.py

Environment:
    LABWORKBENCH_DATA_DIR fixture data directory (must exist; never deleted here)
    LABWORKBENCH_CLI    labworkbench binary (fresh build from this checkout)
"""
import json, os, pathlib, subprocess, sys

ROOT = pathlib.Path(__file__).resolve().parent.parent
DATA = pathlib.Path(os.environ.get("LABWORKBENCH_DATA_DIR", ""))
CLI = os.environ.get("LABWORKBENCH_CLI", "")
if not DATA.is_dir():
    sys.exit(f"LABWORKBENCH_DATA_DIR must be an existing isolated dir, got {DATA!r}")
if not CLI or not os.path.exists(CLI):
    sys.exit(f"LABWORKBENCH_CLI missing: {CLI!r}")
env = dict(os.environ, LABWORKBENCH_DATA_DIR=str(DATA))


def run(tool, payload):
    proc = subprocess.run([CLI, tool, "--input", "-"],
                          input=json.dumps(payload).encode(),
                          capture_output=True, env=env)
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


def block(project, title, summary, steps):
    return run("protocol.block.create", {
        "project_id": project, "title": title, "summary": summary,
        "parameter_slots": [], "steps": steps,
    })["version"]["id"]


def text(use_id, title, instruction):
    return {"kind": "text", "use_id": use_id, "title": title, "instruction": instruction}


project = run("project.create", {"name": "HUVEC tube formation QA"})["record"]["id"]
print("project:", project)

gel = block(project, "GelMA 消毒与脱泡", "80C 5min / 冰上 10min x3, 必要时 2000rpm 离心脱泡", [
    text("gel-cycle", "冷热循环三轮", "(80C 水浴 5min → 冰上 10min) x3; 每轮记录完成时间"),
    text("gel-spin", "脱泡离心(条件)", "有可见气泡时 2000rpm 离心脱泡; 无气泡按规则跳过"),
])
cast = block(project, "铺胶与 405nm 固化", "0.5mL/孔铺匀, 405nm 照射至固化(~120s)", [
    text("cast-spread", "铺胶", "12 孔板每孔铺 0.5mL, 铺匀并确认"),
    text("cast-cure", "405nm 固化", "405nm 紫外照射至固化, 记录各组实际秒数"),
])
soak = block(project, "平衡浸泡", "1.0mL/孔完全培养基, 4C ≥2h", [
    text("soak-add", "加平衡培养基", "每孔 1.0mL 完全培养基(不加 VEGF)"),
    text("soak-wait", "4C 浸泡", "4C 放入并记录时刻; 取出记录时刻, 需 ≥2h"),
])
seed = block(project, "接种 HUVEC", "0.5mL/孔 2e5 cells/mL + 10ng/mL VEGF, 锁定 t0", [
    text("seed-susp", "接种细胞悬液", "复温吸去旧液; 每孔 0.5mL 悬液(2e5 cells/mL, 10ng/mL VEGF)"),
    text("seed-t0", "放回培养箱(t0)", "记录放回培养箱绝对时间戳 t0"),
])
observe = block(project, "明场观察", "每孔 6 视野明场成像", [
    text("obs-image", "明场采图", "9 孔 x 6 视野 = 54 张, 绑定 Well x Timepoint x FOV"),
])

root = block(project, "HUVEC protocol root", "成管全流程", [
    {"kind": "block_use", "use_id": "gel", "block_version_id": gel, "bindings": []},
    {"kind": "block_use", "use_id": "cast", "block_version_id": cast, "bindings": []},
    {"kind": "block_use", "use_id": "soak", "block_version_id": soak, "bindings": []},
    {"kind": "block_use", "use_id": "seed", "block_version_id": seed, "bindings": []},
    {"kind": "block_use", "use_id": "observe", "block_version_id": observe, "bindings": []},
])
protocol = run("protocol.create", {
    "project_id": project, "title": "HUVEC protocol", "root_block_version_id": root,
})["version"]["id"]
print("protocol:", protocol)

STEP_DEFS = [
    ("step-gel", "01 GelMA 消毒与脱泡", "root/gel", 1, "gel", []),
    ("step-cast", "02 铺胶与 405nm 固化", "root/cast", 2, "cast", []),
    ("step-soak", "03 平衡浸泡(4C≥2h)", "root/soak", 3, "soak", []),
    ("step-seed", "04 接种 HUVEC(t0)", "root/seed", 4, "seed", []),
    ("step-obs", "05 明场观察(t0+6/12/24h)", "root/observe/obs-image", 5, "observe", ["6 h", "12 h", "24 h"]),
]
BLOCKREF = {"gel": gel, "cast": cast, "soak": soak, "seed": seed, "observe": observe}

ref_blocks = "\n\n".join(
    "```asterlab\nkind: reference\nid: ref-%s\nschema: 1\nref: %s\nview: protocol_block\n```"
    % (name, ver) for name, ver in BLOCKREF.items())

samples = "\n\n".join(
    "```asterlab\nkind: sample\nid: huvec-s%02d\nschema: 1\nsample_id: HUVEC-%s-%d\ngroup_ref: group-%s\namount: {value: 0.5, unit: mL}\n```"
    % (i, conc, rep, conc) for i, (conc, rep) in enumerate(
        [(c, r) for c in ("3pct", "5pct", "7pct") for r in (1, 2, 3)], start=1))

steps_src = []
for sid, title, path, order, blk, tps in STEP_DEFS:
    uses = "  - {ref: ref-%s, role: other}" % blk
    if sid.startswith("step-obs"):
        uses += "\n  - {ref: mp-tube, role: other}"
    tp_line = "\ntimepoints: [%s]" % ", ".join(tps) if tps else "\ntimepoints: []"
    steps_src.append(
        "```asterlab\nkind: step\nid: %s\nschema: 3\ntitle: %s\nexecution: executable\nassay: assay-tube%s\npath: %s\norder: %d\nuses:\n%s\n```"
        % (sid, title, tp_line, path, order, uses))
steps_src = "\n\n".join(steps_src)

source = f"""# HUVEC 成管实验(QA 门禁)

```asterlab
kind: experiment
id: exp-huvec
schema: 1
title: HUVEC 成管实验
objective: 比较 3%、5%、7% GelMA 水凝胶对 HUVEC 成管的影响
research_question: 不同 GelMA 浓度是否改变 HUVEC 成管能力？
experimental_unit: 一个 12 孔板孔
factors:
  - name: GelMA 浓度
    levels: ["3%", "5%", "7%"]
timepoints: [6 h, 12 h, 24 h]
```

```asterlab
kind: group
id: group-3pct
schema: 1
name: 3% GelMA
role: treatment
condition: 3% GelMA
```

```asterlab
kind: group
id: group-5pct
schema: 1
name: 5% GelMA
role: treatment
condition: 5% GelMA
```

```asterlab
kind: group
id: group-7pct
schema: 1
name: 7% GelMA
role: treatment
condition: 7% GelMA
```

```asterlab
kind: assay
id: assay-tube
schema: 1
title: HUVEC 成管
endpoint: tube length
method: 光镜成像
unit: µm
group_refs: [group-3pct, group-5pct, group-7pct]
protocol_version_id: {protocol}
timepoints: [6 h, 12 h, 24 h]
```

```asterlab
kind: dilution
id: dilution-vegf
schema: 1
stock:
  value: 10
  unit: ug/mL
target:
  value: 0.01
  unit: ug/mL
final_volume:
  value: 5
  unit: mL
```

```asterlab
kind: measurement_profile
id: mp-tube
schema: 1
endpoint: tube length
unit: µm
readout_schema:
  columns: [well, length]
  per_well_fov_count: 6
calculator_refs: []
```

```asterlab
kind: plate
id: plate-huvec
schema: 1
format: 12
assay: assay-tube
groups:
  group-3pct: [A1, A2, A3]
  group-5pct: [B1, B2, B3]
  group-7pct: [C1, C2, C3]
mix_volume_per_well: {{value: 500, unit: uL}}
mix_excess_ratio: 0.1
mix_gelma_percent_by_group:
  group-3pct: 3
  group-5pct: 5
  group-7pct: 7
mix_lap_stock_percent: 0.5
mix_lap_final_percent: 0.05
mix_cell_density: {{value: 100000, unit: cells/well}}
mix_factor_stock_concentration: {{value: 10, unit: ug/mL}}
mix_factor_final_concentration: {{value: 10, unit: ng/mL}}
```

{samples}

{ref_blocks}

{steps_src}

```asterlab
kind: workflow
id: wf-huvec
schema: 1
steps: [step-gel, step-cast, step-soak, step-seed, step-obs]
edges:
  - {{from: step-gel, to: step-cast}}
  - {{from: step-cast, to: step-soak}}
  - {{from: step-soak, to: step-seed}}
  - {{from: step-seed, to: step-obs}}
business_order: [step-gel, step-cast, step-soak, step-seed, step-obs]
```
"""

design = run("design.create", {"project_id": project, "design": {"title": "HUVEC 成管 QA"}})
did = design["design"]["id"]
doc = run("asterdoc.save", {"design_id": did, "source": source, "expected_revision": 0})
rev = doc["revision"]
print("design:", did, "rev:", rev)

assess = run("asterdoc.assess", {"design_id": did, "revision": rev})
print("assess:", assess["state"], json.dumps(assess.get("errors"), ensure_ascii=False))
if assess["state"] == "incomplete":
    sys.exit(2)

created = run("asterdoc.create_run", {"design_id": did, "revision": rev, "title": "HUVEC 成管 QA 运行"})
rid = created["id"]
ws = run("run.workspace", {"id": rid})
print("run:", rid, "tasks:", len(ws["tasks"]))
for task in ws["tasks"]:
    print("  -", task.get("step_path"), "|", task.get("assay_id"), "|", task.get("timepoint"), "|", task.get("status"))

ids = {"data": str(DATA), "project": project, "design": did, "revision": rev,
       "run": rid, "protocol": protocol,
       "blocks": {"gel": gel, "cast": cast, "soak": soak, "seed": seed, "observe": observe}}
(DATA / "huvec-ids.json").write_text(json.dumps(ids, indent=2), encoding="utf-8")
print("ids:", DATA / "huvec-ids.json")
