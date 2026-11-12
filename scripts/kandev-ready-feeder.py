#!/usr/bin/env python3
"""Promote only eligible VPS Dev cards into Ready, then into Agent.

Kandev's pull_from_step_id feeder does not skip blocked tasks: putting a
blocked card into Ready can start an agent anyway. This feeder is the
dependency-aware replacement. It only touches lane:vps cards in the VPS Dev
workflow and never moves work in Review/Verify/Done.

Environment:
  KANDEV_API_TOKEN       required
  KANDEV_URL             default http://127.0.0.1:38429
  KANDEV_WORKSPACE_ID    default asterlab workspace
  KANDEV_VPS_WORKFLOW_ID default VPS Dev workflow
  KANDEV_VPS_INBOX_STEP  default Inbox step
  KANDEV_VPS_READY_STEP  default Ready step
  KANDEV_VPS_AGENT_STEP  default Agent step
  KANDEV_AGENT_WIP       default 2
"""

import argparse
import json
import os
import sys
import time
import urllib.error
import urllib.request

BASE = os.environ.get("KANDEV_URL", "http://127.0.0.1:38429").rstrip("/")
TOKEN = os.environ.get("KANDEV_API_TOKEN", "")
WORKSPACE = os.environ.get("KANDEV_WORKSPACE_ID", "b67b6785-db21-46de-bd68-c0c437823798")
WORKFLOW = os.environ.get("KANDEV_VPS_WORKFLOW_ID", "d1a08c74-42ab-4e8d-aee3-a67b459537f8")
INBOX = os.environ.get("KANDEV_VPS_INBOX_STEP", "1141b1f5-2ea7-44bb-b182-d7490b31a8e9")
READY = os.environ.get("KANDEV_VPS_READY_STEP", "9e7881ac-cda4-480c-8652-d3665f78645e")
AGENT = os.environ.get("KANDEV_VPS_AGENT_STEP", "47b1aac7-2bb9-4b26-b8e3-412c424b4f46")
WIP = int(os.environ.get("KANDEV_AGENT_WIP", "2"))
ACTIVE_STATES = {"IN_PROGRESS", "SCHEDULING", "WAITING_FOR_INPUT"}


def api(method, path, body=None):
    data = json.dumps(body).encode() if body is not None else None
    req = urllib.request.Request(
        BASE + path,
        data=data,
        method=method,
        headers={
            "Authorization": f"Bearer {TOKEN}",
            "Content-Type": "application/json",
        },
    )
    try:
        with urllib.request.urlopen(req, timeout=20) as resp:
            return resp.status, json.loads(resp.read().decode() or "{}")
    except urllib.error.HTTPError as exc:
        raw = exc.read().decode(errors="replace")
        try:
            payload = json.loads(raw)
        except json.JSONDecodeError:
            payload = {"error": raw}
        return exc.code, payload
    except Exception as exc:  # noqa: BLE001 - feeder must keep running
        return 0, {"error": str(exc)}


def list_tasks():
    status, data = api("GET", f"/api/v1/workspaces/{WORKSPACE}/tasks")
    if status != 200:
        raise RuntimeError(f"list tasks failed: {status} {data}")
    return data.get("tasks", [])


def get_task(task_id):
    status, data = api("GET", f"/api/v1/tasks/{task_id}")
    if status != 200:
        raise RuntimeError(f"get task failed: {status} {data}")
    return data


def lane_labels(task):
    try:
        return json.loads(task.get("labels") or "[]")
    except json.JSONDecodeError:
        return []


def is_vps(task):
    return "lane:vps" in lane_labels(task)


def is_eligible(task):
    return not task.get("blocked")


def move(task, step_id, reason):
    status, data = api(
        "POST",
        f"/api/v1/tasks/{task['id']}/move",
        {"workflow_id": WORKFLOW, "workflow_step_id": step_id},
    )
    if status == 200:
        print(f"move {task['title'][:52]} -> {reason}")
        return True
    print(f"move failed {task['title'][:52]} -> {reason}: {status} {data}", file=sys.stderr)
    return False


def run_once():
    tasks = list_tasks()
    vps = [t for t in tasks if is_vps(t) and t.get("workflow_id") == WORKFLOW]
    # Refresh each task: the list projection can lag behind blocked_by edits.
    detailed = []
    for task in vps:
        try:
            detailed.append(get_task(task["id"]))
        except Exception as exc:  # noqa: BLE001
            print(f"skip {task['title'][:40]}: {exc}", file=sys.stderr)

    agent_tasks = [t for t in detailed if t.get("workflow_step_id") == AGENT]
    active = [t for t in agent_tasks if t.get("state") in ACTIVE_STATES]
    slots = max(0, WIP - len(active))

    # A blocked card in Ready is a bug: put it back before it can start.
    for task in detailed:
        if task.get("workflow_step_id") == READY and not is_eligible(task):
            move(task, INBOX, "Ready blocked -> Inbox")

    # Fill Ready with every eligible Inbox card.
    for task in detailed:
        if task.get("workflow_step_id") == INBOX and is_eligible(task) and task.get("state") == "CREATED":
            move(task, READY, "Inbox eligible -> Ready")

    # Start eligible Ready cards while slots exist.
    if slots > 0:
        ready = [t for t in detailed if t.get("workflow_step_id") == READY and is_eligible(t)]
        ready.sort(key=lambda t: (t.get("position", 0), t.get("created_at", "")))
        for task in ready[:slots]:
            move(task, AGENT, "Ready eligible -> Agent")


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--once", action="store_true")
    parser.add_argument("--interval", type=int, default=30)
    args = parser.parse_args()
    if not TOKEN:
        print("KANDEV_API_TOKEN is required", file=sys.stderr)
        return 2
    while True:
        try:
            run_once()
        except Exception as exc:  # noqa: BLE001
            print(f"feeder error: {exc}", file=sys.stderr)
        if args.once:
            return 0
        time.sleep(max(5, args.interval))


if __name__ == "__main__":
    raise SystemExit(main())
