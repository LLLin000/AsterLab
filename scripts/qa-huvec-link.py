"""Link the 162 HUVEC fixtures via CLI asset.link (same Core contract the GUI
batch dialog uses; the OS-native file dialog is not automation-drivable).

Maps huvec/<WELL>/<6h|12h|24h>/fov<N>.(png|tif) to the matching obs task key
with well_id/timepoint/fov_index coordinates, then seeds
workspace-session.json so the GUI resolves the same logical root.

Usage: LABWORKBENCH_DATA_DIR=... LABWORKBENCH_CLI=... ASSETS_DIR=... python scripts/qa-huvec-link.py
"""
import json, os, pathlib, subprocess, sys

DATA = pathlib.Path(os.environ.get("LABWORKBENCH_DATA_DIR", ""))
CLI = os.environ.get("LABWORKBENCH_CLI", "")
ASSETS = pathlib.Path(os.environ.get("ASSETS_DIR", "D:/.ci-cache/qa-huvec-01-assets"))
if not DATA.is_dir() or not CLI or not ASSETS.is_dir():
    sys.exit(f"bad env: DATA={DATA} CLI={CLI} ASSETS={ASSETS}")
env = dict(os.environ, LABWORKBENCH_DATA_DIR=str(DATA))

WELLS = ["A1", "A2", "A3", "B1", "B2", "B3", "C1", "C2", "C3"]
TPS = {"6h": "6 h", "12h": "12 h", "24h": "24 h"}
SAMPLE = {w: f"HUVEC-{c}-{r}" for w, c, r in [
    ("A1", "3%", 1), ("A2", "3%", 2), ("A3", "3%", 3),
    ("B1", "5%", 1), ("B2", "5%", 2), ("B3", "5%", 3),
    ("C1", "7%", 1), ("C2", "7%", 2), ("C3", "7%", 3)]}


def cli(tool, payload):
    proc = subprocess.run([CLI, tool, "--input", "-"],
                          input=json.dumps(payload).encode(),
                          capture_output=True, env=env)
    out = proc.stdout.decode("utf-8", "replace").strip()
    value = json.loads(out[out.index("{"):])
    if not value.get("ok"):
        print("FAIL", tool, json.dumps(value.get("error"), ensure_ascii=False)[:500])
        sys.exit(1)
    return value["result"]


ids = json.loads((DATA / "huvec-ids.json").read_text(encoding="utf-8"))
run_id, design_id = ids["run"], ids["design"]
root_id = "design-data-" + design_id

ws = cli("run.workspace", {"id": run_id})
key_of = {}
for t in ws["tasks"]:
    key_of[(t["step_path"], t.get("timepoint") or "")] = t["task_key"]
obs_keys = {tp: key_of[("root/observe/obs-image", tp)] for tp in ("6 h", "12 h", "24 h")}
print("obs task keys:", json.dumps(obs_keys, ensure_ascii=False)[:200], "...")

n = 0
for well in WELLS:
    for d, tp in TPS.items():
        for fov in range(1, 7):
            ext = "png" if fov % 2 == 1 else "tif"
            rel = f"huvec/{well}/{d}/fov{fov}.{ext}"
            assert (ASSETS / rel).is_file(), rel
            cli("asset.link", {
                "root_id": root_id, "root_dir": str(ASSETS),
                "relative_path": rel.replace("/", os.sep),
                "run_id": run_id, "task_key": obs_keys[tp],
                "sample_id": SAMPLE[well],
                "well_id": well, "timepoint": tp, "fov_index": fov,
            })
            n += 1
print(f"linked {n} assets")

sess_path = DATA / "workspace-session.json"
sess = {}
if sess_path.is_file():
    try:
        sess = json.loads(sess_path.read_text(encoding="utf-8"))
    except Exception:
        sess = {}
roots = sess.get("assetRoots", {})
roots[root_id] = str(ASSETS)
sess["assetRoots"] = roots
sess_path.write_text(json.dumps(sess, ensure_ascii=False, indent=1), encoding="utf-8")
print("session roots:", root_id, "->", str(ASSETS))
