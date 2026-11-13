"""QA-06 evidence: exported references carry no absolute path; escaping symlinks fail.

Runs the built CLI against an isolated data directory and rewrites
docs/release/evidence/QA-06/export-scan.txt. Needs `cargo build --bin
labworkbench` first; the data directory stays under CARGO_TARGET_DIR.

Usage (repository root): python3 docs/release/evidence/QA-06/scan.py
"""
import hashlib
import json
import os
import pathlib
import shutil
import sqlite3
import subprocess
import sys
import tempfile

ROOT = pathlib.Path(__file__).resolve().parents[4]
CLI = pathlib.Path(os.environ.get("CARGO_TARGET_DIR", ROOT / "target")) / "debug" / "labworkbench"
EVIDENCE = ROOT / "docs" / "release" / "evidence" / "QA-06"
EVIDENCE.mkdir(parents=True, exist_ok=True)
BASE = pathlib.Path(tempfile.mkdtemp(prefix="qa06-", dir=str(pathlib.Path(os.environ.get("CARGO_TARGET_DIR", ROOT / "target")))))
DATA = BASE / "data"
ASSET_ROOT = BASE / "root"
OUTSIDE = BASE / "outside"
EXPORTS = BASE / "export"
for p in (DATA, ASSET_ROOT / "d1", OUTSIDE, EXPORTS):
    p.mkdir(parents=True, exist_ok=True)

ENV = dict(os.environ, LABWORKBENCH_DATA_DIR=str(DATA), ASTERLAB_DATA_DIR=str(DATA))


def cli(tool, payload):
    p = subprocess.run([str(CLI), tool, "--input", "-"], input=json.dumps(payload).encode(),
                       capture_output=True, env=ENV)
    out = p.stdout.decode().strip()
    try:
        return json.loads(out)
    except Exception:
        print("NON-JSON", tool, out[:400], p.stderr.decode()[:400])
        raise


def ok(tool, payload):
    v = cli(tool, payload)
    if not v.get("ok"):
        print("FAIL", tool, json.dumps(v.get("error"), ensure_ascii=False))
        sys.exit(1)
    return v["result"]


def sha256(path):
    h = hashlib.sha256()
    with open(path, "rb") as f:
        for chunk in iter(lambda: f.read(1 << 20), b""):
            h.update(chunk)
    return h.hexdigest()


def write_export(name, value):
    path = EXPORTS / name
    path.write_text(json.dumps(value, ensure_ascii=False, indent=2) + "\n")
    return path


lines = []
def say(s=""):
    print(s)
    lines.append(s)

fixture = subprocess.run([sys.executable, str(ROOT / "scripts" / "rc3-fixture.py")],
                         capture_output=True, env=ENV, cwd=str(ROOT))
(EVIDENCE / "fixture.txt").write_text(fixture.stdout.decode() + fixture.stderr.decode())
if fixture.returncode != 0:
    print(fixture.stdout.decode(), fixture.stderr.decode())
    sys.exit(1)
ids = json.loads((DATA / "rc3-ids.json").read_text())

ws = ok("run.workspace", {"id": ids["run"]})
task_key = ws["tasks"][0]["task_key"]
say(f"run_id={ids['run']} task_key={task_key}")

# Files: one inside the root, one outside it.
(ASSET_ROOT / "d1" / "readout.csv").write_text("well,od450\nA1,0.412\n")
(OUTSIDE / "secret.csv").write_text("secret\n")
os.symlink(OUTSIDE / "secret.csv", ASSET_ROOT / "leak.csv")
os.symlink(OUTSIDE, ASSET_ROOT / "linked-dir")

base_link = {"root_id": "project-data", "root_dir": str(ASSET_ROOT),
             "run_id": ids["run"], "task_key": task_key}

positive = ok("asset.link", dict(base_link, relative_path="d1/readout.csv"))
say("positive relative_path link: ok")
by_path = ok("asset.link", dict(base_link, path=str(ASSET_ROOT / "d1" / "readout.csv")))
say("positive absolute path link (derived relative): ok")
assert by_path["record"]["data"]["relativePath"] == "d1/readout.csv"
assert by_path["record"]["data"]["rootId"] == "project-data"

exports = {
    "asset-list": ok("asset.list", {"run_id": ids["run"]}),
    "asset-events": ok("asset.events", {"run_id": ids["run"]}),
    "record-asset-link": ok("record.list", {"kind": "asset_link"}),
    "record-asset-link-event": ok("record.list", {"kind": "asset_link_event"}),
    "record-run": ok("record.list", {"kind": "run"}),
}
export_paths = {name: write_export(name + ".json", value) for name, value in exports.items()}

say("")
say("=== absolute-path scan of exported JSON ===")
needles = [str(ASSET_ROOT), str(OUTSIDE), str(DATA), str(BASE)]
generic = subprocess.run(
    ["grep", "-nEo",
     r"([A-Za-z]:\\\\|/srv/|/home/|/root/|/var/|/tmp/)[^\"[:space:]]*|\\\\\\\\[A-Za-z0-9._-]+\\\\\\\\"]
    + [str(p) for p in export_paths.values()], capture_output=True, text=True)
total = 0
for name, path in export_paths.items():
    text = path.read_text()
    hits = [n for n in needles if n in text]
    total += len(hits)
    say(f"{name}: bytes={len(text)} absolute-path hits={len(hits)} {hits}")
generic_hits = generic.stdout.strip().splitlines()
say(f"generic absolute-path regex hits: {len(generic_hits)}")
for h in generic_hits:
    say("  " + h)
if total or generic_hits:
    sys.exit("absolute path leaked into an export")

say("")
say("=== persisted data_json scan ===")
db = DATA / "labworkbench.sqlite"
con = sqlite3.connect(db)
rows = con.execute("select id,kind,data_json from records where kind in ('asset_link','asset_link_event')").fetchall()
say(f"asset rows: {len(rows)}")
leaks = [r[0] for r in rows if any(n in r[2] for n in needles)]
say(f"rows containing a local root/outside/data path: {leaks}")
for rid, kind, data in rows:
    d = json.loads(data)
    say(f"{kind} {rid} rootId={d.get('rootId')} relativePath={d.get('relativePath')}")
con.close()
if leaks:
    sys.exit("local path persisted in the database")
say(f"sha256 {sha256(db)}  {db}")

negative = [
    ("absolute_root_id", dict(base_link, root_id=str(ASSET_ROOT), relative_path="d1/readout.csv")),
    ("drive_absolute_root_id", dict(base_link, root_id="C:/data", relative_path="d1/readout.csv")),
    ("unc_root_id", dict(base_link, root_id="\\\\server\\share", relative_path="d1/readout.csv")),
    ("parent_root_id", dict(base_link, root_id="roots/../escape", relative_path="d1/readout.csv")),
    ("traversal", dict(base_link, relative_path="../outside/secret.csv")),
    ("traversal_inner", dict(base_link, relative_path="d1/../../outside/secret.csv")),
    ("absolute_relative", dict(base_link, relative_path="/etc/passwd")),
    ("drive_relative", dict(base_link, relative_path="C:/Windows/win.ini")),
    ("symlink_file", dict(base_link, relative_path="leak.csv")),
    ("symlink_dir", dict(base_link, relative_path="linked-dir/secret.csv")),
    ("absolute_path_outside", dict(base_link, path=str(OUTSIDE / "secret.csv"))),
]
say("")
say("=== negative cases (isolated database) ===")
before = sha256(db)
counts_before = con_counts = len(rows)
for name, payload in negative:
    v = cli("asset.link", payload)
    say(f"[{name}] {json.dumps(v, ensure_ascii=False, sort_keys=True)}")
    if v.get("ok"):
        sys.exit(f"{name} was accepted")
after = sha256(db)
con = sqlite3.connect(db)
rows_after = con.execute("select count(*) from records where kind in ('asset_link','asset_link_event')").fetchone()[0]
con.close()
say(f"sha256_before_negative_cases {before}")
say(f"sha256_after_negative_cases  {after}")
say(f"asset_rows_before={counts_before} asset_rows_after={rows_after}")
if before != after or rows_after != counts_before:
    sys.exit("a rejected link still wrote to the database")

# Escaping symlink replacing a valid reference: inspect must not reuse stale state.
(ASSET_ROOT / "d1" / "readout.csv").unlink()
os.symlink(OUTSIDE / "secret.csv", ASSET_ROOT / "d1" / "readout.csv")
say("")
say("=== inspect after the valid file is replaced by an escaping symlink ===")
say("re-read of the stored reference is covered by the Rust suite "
    "(assets::tests::symlinks_that_leave_the_root_are_rejected); the CLI has no "
    "read-only inspect tool that takes a root_dir.")

say("")
say(f"isolated base: {BASE}")
say(f"database: {db}")

(EVIDENCE / "export-scan.txt").write_text("\n".join(lines) + "\n")
print("wrote", EVIDENCE / "export-scan.txt")
