#!/usr/bin/env python3
"""Fall back from DeepSeek to Luna on DeepSeek quota exhaustion.

This is intentionally one-way: Luna quota errors never trigger a switch to DS.
The script never auto-fails back to DS; selecting DS again is a manual action.
"""

import json
import os
import re
import sqlite3
import urllib.error
import urllib.request
from datetime import datetime, timedelta, timezone

DB = os.environ.get("KANDEV_DB", "/var/lib/kandev/data/kandev.db")
API_BASE = os.environ.get("KANDEV_URL", "http://127.0.0.1:38429").rstrip("/")
API_TOKEN = os.environ.get("KANDEV_API_TOKEN", "")
DS = "29ea4051-cbfb-46b3-9474-27331d8841e8"
LUNA = "e930e6a0-835f-4e84-834e-09dea519280c"
DS_MODEL = "opencode-go/deepseek-v4.1-flash"
LUNA_MODEL = "openai-codex/gpt-6-luna"

STEPS = {
    "Agent": "47b1aac7-2bb9-4b26-b8e3-412c424b4f46",
    "Review": "2b6d9951-e9fd-4578-8bf4-6f3e7aa7ce31",
}
ACTIVE_SESSION_STATES = ("IN_PROGRESS", "SCHEDULING", "WAITING_FOR_INPUT")
QUOTA_MARKER = "usage limit reached"
RESET_RE = re.compile(r"\bresets in\s+((?:\d+\s*[wdhms]\s*)+)", re.IGNORECASE)
DURATION_RE = re.compile(r"(\d+)\s*([wdhms])", re.IGNORECASE)


def _utc(value):
    if isinstance(value, datetime):
        result = value
    else:
        value = str(value)
        if value.endswith("Z"):
            value = value[:-1] + "+00:00"
        result = datetime.fromisoformat(value)
    if result.tzinfo is None:
        result = result.replace(tzinfo=timezone.utc)
    return result.astimezone(timezone.utc)


def _quota_error_is_active(model, content, created_at, now):
    if model != DS_MODEL or QUOTA_MARKER not in (content or "").lower():
        return False
    reset = RESET_RE.search(content)
    if not reset:
        return True
    try:
        duration = timedelta()
        for amount, unit in DURATION_RE.findall(reset.group(1)):
            amount = int(amount)
            duration += {
                "w": timedelta(weeks=amount),
                "d": timedelta(days=amount),
                "h": timedelta(hours=amount),
                "m": timedelta(minutes=amount),
                "s": timedelta(seconds=amount),
            }[unit.lower()]
        return _utc(created_at) + duration > _utc(now)
    except (TypeError, ValueError, OverflowError):
        return True


def ds_quota_exhausted(messages, now=None):
    now = _utc(now or datetime.now(timezone.utc))
    return any(
        _quota_error_is_active(model, content, created_at, now)
        for model, content, created_at in messages
    )


def fallback_target(current_profile, ds_quota_exhausted):
    return LUNA if ds_quota_exhausted else current_profile


def _snapshot_model(snapshot):
    try:
        value = json.loads(snapshot) if isinstance(snapshot, str) else snapshot
    except (TypeError, json.JSONDecodeError):
        return ""
    return value.get("model", "") if isinstance(value, dict) else ""


def _quota_messages(con):
    rows = con.execute(
        """SELECT s.agent_profile_snapshot, m.content, m.created_at
           FROM task_session_messages AS m
           JOIN task_sessions AS s ON s.id = m.task_session_id
           WHERE lower(m.content) LIKE ?
           ORDER BY m.created_at DESC""",
        (f"%{QUOTA_MARKER}%",),
    )
    return [
        (_snapshot_model(snapshot), content, created_at)
        for snapshot, content, created_at in rows
    ]


def _set_session_model(session_id):
    if not API_TOKEN:
        print(f"session {session_id[:8]}: cannot switch model; API token is unset")
        return False
    request = urllib.request.Request(
        f"{API_BASE}/api/v1/task-sessions/{session_id}/set-model",
        data=json.dumps({"model_id": LUNA_MODEL}).encode(),
        method="POST",
        headers={
            "Authorization": f"Bearer {API_TOKEN}",
            "Content-Type": "application/json",
        },
    )
    try:
        with urllib.request.urlopen(request, timeout=20) as response:
            if response.status == 200:
                print(f"session {session_id[:8]}: switched -> LUNA")
                return True
            print(f"session {session_id[:8]}: set-model returned {response.status}")
    except urllib.error.HTTPError as exc:
        detail = exc.read().decode(errors="replace")
        print(f"session {session_id[:8]}: set-model failed ({exc.code}): {detail[:200]}")
    except Exception as exc:
        print(f"session {session_id[:8]}: set-model failed: {exc}")
    return False

def reconcile(con, now=None, set_session_model=None):
    con.row_factory = sqlite3.Row
    now = _utc(now or datetime.now(timezone.utc))
    if set_session_model is None:
        set_session_model = _set_session_model

    if not ds_quota_exhausted(_quota_messages(con), now):
        return 0, 0

    changes = 0
    for name, step_id in STEPS.items():
        row = con.execute(
            "SELECT agent_profile_id FROM workflow_steps WHERE id=?", (step_id,)
        ).fetchone()
        if row is None:
            print(f"step {name} ({step_id[:8]}) not found; skipping")
            continue
        current = row["agent_profile_id"]
        target = fallback_target(current, ds_quota_exhausted=True)
        if current == target:
            continue
        con.execute(
            "UPDATE workflow_steps SET agent_profile_id=? WHERE id=?",
            (target, step_id),
        )
        changes += 1
        print(f"{name}: switched -> LUNA")

    sessions = con.execute(
        """SELECT id, agent_profile_snapshot
           FROM task_sessions
           WHERE state IN (?, ?, ?)""",
        ACTIVE_SESSION_STATES,
    ).fetchall()
    con.commit()

    session_changes = 0
    for session in sessions:
        if _snapshot_model(session["agent_profile_snapshot"]) != DS_MODEL:
            continue
        session_changes += int(set_session_model(session["id"]))
    if changes or session_changes:
        print(f"DeepSeek quota active; step changes={changes}, session changes={session_changes}")
    return changes, session_changes


def run_once():
    con = sqlite3.connect(DB)
    try:
        return reconcile(con)
    finally:
        con.close()


def main():
    run_once()


if __name__ == "__main__":
    main()
