#!/usr/bin/env python3
"""Reap omp processes Kandev already considers stopped.

Native ACP suspension marks executors stopped/suspended but sometimes fails
to kill the omp process (broken agentctl control connection). Those zombies
hold ~300MB each forever.

Safety: only kill a process when ALL hold:
  - an executors_running row exists for its session with status='stopped'
  - the session itself is not RUNNING/SCHEDULING
  - the process cwd matches the session's task worktree (never kill blind)

This touches no Kandev state; it only reaps processes Kandev gave up on.
Usage:
  kandev-zombie-reaper.py          report only
  kandev-zombie-reaper.py --kill   SIGTERM, then SIGKILL after 10s if needed
"""

import os
import signal
import sqlite3
import subprocess
import sys
import time

DB = os.environ.get("KANDEV_DB", "/var/lib/kandev/data/kandev.db")


def omp_procs():
    out = subprocess.run(
        ["pgrep", "-f", "omp --model"], capture_output=True, text=True
    ).stdout
    procs = []
    for line in out.splitlines():
        try:
            pid = int(line.strip())
            cwd = os.readlink(f"/proc/{pid}/cwd")
            procs.append((pid, cwd))
        except (ValueError, OSError):
            continue
    return procs


def stopped_sessions():
    """session_id -> task worktree path, for sessions safe to reap.

    A task with ANY live session is excluded entirely: we match processes
    by cwd, which cannot tell live apart from dead within one task.
    """
    con = sqlite3.connect(f"file:{DB}?mode=ro", uri=True)
    live_tasks = {
        r[0]
        for r in con.execute(
            "select distinct task_id from task_sessions where state in ('RUNNING','SCHEDULING')"
        ).fetchall()
    }
    rows = con.execute(
        """select e.session_id, t.id as task_id from executors_running e
            join task_sessions s on s.id=e.session_id
            join tasks t on t.id=s.task_id
           where e.status='stopped'
             and s.state not in ('RUNNING','SCHEDULING')"""
    ).fetchall()
    out = {}
    for sid, tid in rows:
        if tid in live_tasks:
            continue
        r = con.execute(
            "select workspace_path from task_environments where task_id=? order by created_at desc limit 1",
            (tid,),
        ).fetchone()
        if r and r[0]:
            out[sid] = r[0]
    con.close()
    return out


def main():
    kill = "--kill" in sys.argv
    safe = stopped_sessions()
    zombies = []
    for pid, cwd in omp_procs():
        # match process to a stopped session by worktree path
        for sid, wt in safe.items():
            if cwd == wt or cwd.startswith(wt + "/"):
                zombies.append((pid, cwd, sid[:8]))
                break
    print(f"omp total: {len(omp_procs())}, zombies: {len(zombies)}")
    for pid, cwd, sid in zombies:
        print(f"  PID {pid} {cwd} (session {sid} stopped)")
    if not kill:
        print("dry run: nothing killed (use --kill)")
        return 0
    for pid, _, _ in zombies:
        try:
            os.kill(pid, signal.SIGTERM)
        except ProcessLookupError:
            pass
    time.sleep(10)
    for pid, _, _ in zombies:
        try:
            os.kill(pid, 0)
            os.kill(pid, signal.SIGKILL)
            print(f"  PID {pid}: SIGTERM ignored, SIGKILL sent")
        except ProcessLookupError:
            print(f"  PID {pid}: reaped")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
