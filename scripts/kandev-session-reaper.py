#!/usr/bin/env python3
"""Stop executor sessions Kandev left behind for finished tasks.

Merged (COMPLETED) tasks keep their executor `ready` forever, one omp ACP
process (~200MB) each. This reaper stops them through Kandev's own
`session.stop` WebSocket action -- never `kill`. Stopping an idle session
ends its process, flips its executor row `ready` -> `stopped`, and keeps the
conversation history, so completed cards stay reviewable.

Safety rules (a session is only stopped when ALL hold):
  - its task is COMPLETED (Merged) or archived
  - its executor row is `ready` (not stopped/prepared/other)
  - its session state is WAITING_FOR_INPUT (never RUNNING/SCHEDULING)

Usage:
  kandev-session-reaper.py            report candidates, change nothing
  kandev-session-reaper.py --stop     stop candidates
  kandev-session-reaper.py --stop --limit 1   stop oldest candidate only
"""

import argparse
import base64
import json
import os
import socket
import sqlite3
import sys
import time

DB = os.environ.get("KANDEV_DB", "/var/lib/kandev/data/kandev.db")
URL = os.environ.get("KANDEV_URL", "http://127.0.0.1:38429").rstrip("/")
TOKEN = os.environ.get("KANDEV_API_TOKEN", "")


def ws_call(messages, timeout=6):
    """Send actions to /ws; return received text frames (best effort)."""
    if URL.startswith("http://127.0.0.1:") or URL.startswith("http://localhost:"):
        host, port = URL.rsplit("://", 1)[1].split(":", 1)[0], int(
            URL.rsplit(":", 1)[1].split("/")[0]
        )
    else:
        print(f"refusing non-local KANDEV_URL for ws: {URL}", file=sys.stderr)
        return []
    key = base64.b64encode(os.urandom(16)).decode()
    sock = socket.create_connection((host, port), timeout=timeout)
    req = (
        f"GET /ws HTTP/1.1\r\nHost: {host}:{port}\r\n"
        "Upgrade: websocket\r\nConnection: Upgrade\r\n"
        f"Sec-WebSocket-Key: {key}\r\nSec-WebSocket-Version: 13\r\n"
        f"Authorization: Bearer {TOKEN}\r\n\r\n"
    )
    sock.sendall(req.encode())
    header = b""
    while b"\r\n\r\n" not in header:
        header += sock.recv(4096)
    if b"101" not in header.split(b"\r\n")[0]:
        print(f"ws handshake failed: {header[:120]}", file=sys.stderr)
        sock.close()
        return []
    out = []
    for m in messages:
        payload = json.dumps(m).encode()
        mask = os.urandom(4)
        frame = bytearray([0x81, 0x80 | len(payload)]) + mask + bytes(
            b ^ mask[i % 4] for i, b in enumerate(payload)
        )
        sock.sendall(bytes(frame))
    sock.settimeout(2.0)
    try:
        while True:
            d = sock.recv(65536)
            if not d:
                break
            ln = d[1] & 0x7F
            off = 2
            if ln == 126:
                off = 4
                ln = int.from_bytes(d[2:4], "big")
            elif ln == 127:
                off = 10
                ln = int.from_bytes(d[2:10], "big")
            out.append(d[off : off + ln].decode("utf-8", errors="replace"))
    except socket.timeout:
        pass
    sock.close()
    return out


def candidates(con):
    return list(
        con.execute(
            """select t.title, s.id, s.agent_profile_id, e.agentctl_port
                 from executors_running e
                 join task_sessions s on s.id = e.session_id
                 join tasks t on t.id = s.task_id
                where e.status = 'ready'
                  and s.state = 'WAITING_FOR_INPUT'
                  and (t.state = 'COMPLETED' or t.archived_at is not null)
                order by e.last_seen_at"""
        )
    )


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--stop", action="store_true")
    ap.add_argument("--limit", type=int, default=0)
    args = ap.parse_args()
    if args.stop and not TOKEN:
        print("KANDEV_API_TOKEN is required for --stop", file=sys.stderr)
        return 1
    con = sqlite3.connect(f"file:{DB}?mode=ro", uri=True)
    con.row_factory = sqlite3.Row
    rows = candidates(con)
    if args.limit:
        rows = rows[: args.limit]
    print(f"candidates: {len(rows)}")
    stopped, failed = 0, 0
    for r in rows:
        ws_call([{"action": "session.stop", "payload": {"session_id": r["id"]}}])
        time.sleep(0.5)
        # The stop response is often drowned by notification frames; the
        # executor row is the source of truth.
        ok = False
        for _ in range(10):
            time.sleep(1.0)
            st = con.execute(
                "select status from executors_running where session_id=?",
                (r["id"],),
            ).fetchone()
            if st and st["status"] == "stopped":
                ok = True
                break
        if ok:
            stopped += 1
            print(f"stop {r['id'][:8]} ({r['title'][:30]}): ok")
        else:
            failed += 1
            print(f"stop {r['id'][:8]} ({r['title'][:30]}): FAILED (still ready)")


if __name__ == "__main__":
    raise SystemExit(main())
