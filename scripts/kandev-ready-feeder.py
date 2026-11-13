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
  KANDEV_VPS_AGENT_STEP    default Agent step
  KANDEV_VPS_REVIEW_STEP   default Review step
  KANDEV_AGENT_WIP         default 2
  KANDEV_REVIEW_WIP        default 2
  KANDEV_DB                default /var/lib/kandev/data/kandev.db (read-only, for rework counting)
  KANDEV_REWORK_LIMIT      default 3 (Review->Agent trips before human hold)
  TELEGRAM_BOT_TOKEN       optional; when set, human-hold alerts go to Telegram
  TELEGRAM_CHAT_ID         optional; required with TELEGRAM_BOT_TOKEN
"""

import argparse
import json
import os
import re
import subprocess
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
REVIEW = os.environ.get("KANDEV_VPS_REVIEW_STEP", "2b6d9951-e9fd-4578-8bf4-6f3e7aa7ce31")
MERGED_STEP = "e22063df-1124-4836-8e5c-b0c0e387be68"
VERIFY_STEP = "bc096440-6879-45fd-a04a-3d52568efcea"
WIP = int(os.environ.get("KANDEV_AGENT_WIP", "2"))
REVIEW_WIP = int(os.environ.get("KANDEV_REVIEW_WIP", "2"))
DB = os.environ.get("KANDEV_DB", "/var/lib/kandev/data/kandev.db")
REWORK_LIMIT = int(os.environ.get("KANDEV_REWORK_LIMIT", "3"))
TG_TOKEN = os.environ.get("TELEGRAM_BOT_TOKEN", "")
TG_CHAT = os.environ.get("TELEGRAM_CHAT_ID", "")
ACTIVE_STATES = {"IN_PROGRESS", "SCHEDULING", "WAITING_FOR_INPUT"}
STUCK_MINUTES = int(os.environ.get("KANDEV_STUCK_MINUTES", "10"))
STUCK_FIX_LIMIT = int(os.environ.get("KANDEV_STUCK_FIX_LIMIT", "3"))


def stuck_fix_count(labels):
    n = 0
    for lb in labels:
        if lb.startswith("stuck-fix-"):
            try:
                n = max(n, int(lb.split("-")[-1]))
            except ValueError:
                pass
    return n


CI_WAIT_KEYWORDS = ("pending", "仍在跑", "等 CI", "等待 CI", "未全绿", "CI 仍")
REQUIRED_CHECKS = {"linux-tests", "windows-build", "scope-check"}


def ci_all_green(task_id):
    """True when Kandev's own PR poller says checks are success."""
    try:
        import sqlite3

        con = sqlite3.connect(f"file:{DB}?mode=ro", uri=True)
        r = con.execute(
            "select checks_state from github_task_prs where task_id=? order by pr_number desc limit 1",
            (task_id,),
        ).fetchone()
        con.close()
        return bool(r and r[0] == "success")
    except Exception:  # noqa: BLE001
        return False


def scan_ci_ready(review_tasks):
    """Rewake quiet reviews whose PR is now green and mergeable.

    Kandev's PR poller updates checks_state but never wakes the waiting
    session. If the card sits in Review with no live session, an open
    mergeable PR, green checks, and no agent message for 10+ minutes,
    bounce it to trigger a fresh review turn. No keyword matching:
    agent wording varies too much to rely on.
    """
    import datetime

    for task in review_tasks:
        labels = lane_labels(task)
        if "human-hold" in labels:
            continue
        try:
            import sqlite3

            con = sqlite3.connect(f"file:{DB}?mode=ro", uri=True)
            live = con.execute(
                "select count(*) from task_sessions where task_id=? and state in ('RUNNING','SCHEDULING')",
                (task["id"],),
            ).fetchone()[0]
            last = con.execute(
                """select max(m.created_at) from task_session_messages m
                    join task_sessions s on s.id=m.task_session_id
                   where s.task_id=? and m.author_type='agent'""",
                (task["id"],),
            ).fetchone()[0]
            con.close()
        except Exception:  # noqa: BLE001
            continue
        if live:
            continue
        if not last:
            continue
        try:
            dt = datetime.datetime.fromisoformat(last.replace("Z", "+00:00"))
            if dt.tzinfo is None:
                dt = dt.replace(tzinfo=datetime.timezone.utc)
            idle = (datetime.datetime.now(datetime.timezone.utc) - dt).total_seconds() / 60
        except Exception:  # noqa: BLE001
            continue
        if idle < 10:
            continue
        pr = task_pr(task["id"])
        if not pr or pr.get("state") != "open":
            continue
        if not ci_all_green(task["id"]):
            continue
        # Also require mergeable: gh is source of truth, poller lacks it.
        try:
            import subprocess

            out = subprocess.run(
                ["gh", "pr", "view", str(pr["pr_number"]), "--json", "mergeable",
                 "--repo", "LLLin000/AsterLab"],
                capture_output=True, text=True, timeout=30,
            )
            if json.loads(out.stdout or "{}").get("mergeable") != "MERGEABLE":
                continue
        except Exception:  # noqa: BLE001
            continue
        # Retire old sessions so reuse doesn't resurrect the stale one.
        try:
            import sqlite3

            wcon = sqlite3.connect(DB)
            for (sid,) in wcon.execute(
                "select id from task_sessions where task_id=? and state in ('RUNNING','WAITING_FOR_INPUT','SCHEDULING')",
                (task["id"],),
            ).fetchall():
                wcon.execute(
                    "update task_sessions set state='FAILED',completed_at=datetime('now') where id=?", (sid,)
                )
                wcon.execute(
                    "update executors_running set status='stopped',local_pid=0,pid=0 where session_id=?", (sid,)
                )
            wcon.commit()
            wcon.close()
        except Exception as exc:  # noqa: BLE001
            print(f"ci-ready retire failed: {exc}", file=sys.stderr)
            continue
        if move(task, INBOX, "ci-ready -> Inbox"):
            time.sleep(1)
            move(task, REVIEW, "ci-ready -> Review re-review")


def scan_no_pr(review_tasks):
    """Bounce Review cards with no associated PR back to Agent.

    Agent's on_turn_complete fires even when the turn produced nothing;
    without a PR there is nothing to review.
    """
    for task in review_tasks:
        labels = lane_labels(task)
        if "human-hold" in labels:
            continue
        pr = task_pr(task["id"])
        if pr:
            continue
        if merged_pr_by_card(task):
            continue  # already merged; the sweep will move it
        # Grace period: just entered Review, PR may be seconds away.
        try:
            import sqlite3
            from datetime import datetime, timezone

            con = sqlite3.connect(f"file:{DB}?mode=ro", uri=True)
            last = con.execute(
                "select max(occurred_at) from task_step_transitions where task_id=? and to_workflow_step_id=?",
                (task["id"], REVIEW),
            ).fetchone()[0]
            con.close()
        except Exception:  # noqa: BLE001
            continue
        if not last:
            continue
        try:
            dt = datetime.fromisoformat(last.replace("Z", "+00:00"))
            idle = (datetime.now(timezone.utc) - dt).total_seconds() / 60
        except Exception:  # noqa: BLE001
            continue
        if idle < 10:
            continue
        move(task, AGENT, "no PR in Review -> back to Agent")


def session_capacity():
    """Effective max concurrent sessions (env-locked on this host)."""
    try:
        status, data = api("GET", "/api/v1/system/session-capacity/settings")
        if status == 200:
            eff = data.get("effective") or {}
            return int(eff.get("max_sessions") or 0)
    except Exception:  # noqa: BLE001
        pass
    return int(os.environ.get("KANDEV_MAX_CONCURRENT_SESSIONS", "4"))


def live_session_ids():
    """Sessions that occupy a capacity slot (RUNNING/WAITING)."""
    import sqlite3

    try:
        con = sqlite3.connect(f"file:{DB}?mode=ro", uri=True)
        rows = con.execute(
            "select id from task_sessions where state in ('RUNNING','WAITING_FOR_INPUT')"
        ).fetchall()
        con.close()
        return {r[0] for r in rows}
    except Exception:  # noqa: BLE001
        return set()


def _agent_procs():
    """Live omp ACP processes: pid -> (cwd, start_epoch)."""
    try:
        btime = None
        for line in open("/proc/stat"):
            if line.startswith("btime "):
                btime = int(line.split()[1])
                break
        hz = os.sysconf("SC_CLK_TCK")
    except Exception:  # noqa: BLE001
        return {}
    procs = {}
    for pid in os.listdir("/proc"):
        if not pid.isdigit():
            continue
        try:
            cmd = open(f"/proc/{pid}/cmdline", "rb").read().replace(b"\0", b" ").decode(errors="replace")
            if "omp" not in cmd or "acp" not in cmd:
                continue
            fields = open(f"/proc/{pid}/stat").read().rsplit(") ", 1)[1].split()
            started = btime + int(fields[19]) / hz
            procs[int(pid)] = (os.readlink(f"/proc/{pid}/cwd"), started)
        except Exception:  # noqa: BLE001
            continue
    return procs


def _kill_proc(pid):
    import signal

    try:
        os.kill(pid, signal.SIGTERM)
    except OSError:
        return
    for _ in range(10):
        time.sleep(0.5)
        if not os.path.exists(f"/proc/{pid}"):
            return
    try:
        os.kill(pid, signal.SIGKILL)
    except OSError:
        pass


def scan_session_reaper(detailed):
    """Reconcile task_sessions with reality.

    Kandev leaves sessions in RUNNING/WAITING_FOR_INPUT after a card is
    merged, and its executors_running pid is not the omp pid, so teardown
    misses processes. Match omp processes by worktree path and start time;
    close sessions whose task has left Agent/Review and kill their process.
    """
    procs = _agent_procs()
    if not procs:
        return
    step_of = {t["id"]: t.get("workflow_step_id") for t in detailed}
    try:
        import sqlite3

        con = sqlite3.connect(f"file:{DB}?mode=ro", uri=True)
        con.row_factory = sqlite3.Row
        all_rows = [
            dict(r)
            for r in con.execute(
                """select id,task_id,state,started_at,workspace_path from task_sessions"""
            )
            if r["task_id"] in step_of
        ]
        con.close()
    except Exception as exc:  # noqa: BLE001
        print(f"session scan failed: {exc}", file=sys.stderr)
        return
    rows = [s for s in all_rows if s["state"] in ("RUNNING", "WAITING_FOR_INPUT")]

    def epoch(s):
        from datetime import datetime, timezone

        try:
            return datetime.fromisoformat(s.replace("Z", "+00:00")).timestamp()
        except Exception:  # noqa: BLE001
            return 0

    # Match each process to the session that launched it (same worktree,
    # closest start within a 15 min window).
    matched = {}
    for pid, (cwd, pstart) in procs.items():
        best = None
        for s in rows:
            if s["workspace_path"] != cwd:
                continue
            delta = abs(pstart - epoch(s["started_at"] or ""))
            if delta > 900:
                continue
            if best is None or delta < best[0]:
                best = (delta, s)
        if best:
            matched.setdefault(best[1]["id"], []).append(pid)

    stale = {}
    for s in rows:
        step = step_of.get(s["task_id"])
        if step in (MERGED_STEP, INBOX, READY) or not matched.get(s["id"]):
            stale[s["id"]] = s

    # Processes in a known task worktree that belong to no live session:
    # failed/retired sessions leave these behind.
    keep = {s["id"] for s in rows if s["id"] not in stale}
    keep_pids = {pid for sid in keep for pid in matched.get(sid, [])}
    stale_pids = {pid for sid in stale for pid in matched.get(sid, [])}
    known_cwds = {s["workspace_path"] for s in all_rows if s["workspace_path"]}
    orphan_pids = [
        pid
        for pid, (cwd, pstart) in procs.items()
        if pid not in keep_pids
        and pid not in stale_pids
        and cwd in known_cwds
        and time.time() - pstart > 60
    ]

    if not stale and not orphan_pids:
        return
    try:
        import sqlite3

        wcon = sqlite3.connect(DB)
        for sid, s in stale.items():
            for pid in matched.get(sid, []):
                _kill_proc(pid)
            wcon.execute(
                """update task_sessions set state='CANCELLED',completed_at=datetime('now'),
                   updated_at=datetime('now'),error_message='reaped: no live agent process'
                   where id=?""",
                (sid,),
            )
            wcon.execute(
                "update executors_running set status='stopped',local_pid=0,pid=0 where session_id=?",
                (sid,),
            )
        for pid in orphan_pids:
            _kill_proc(pid)
        wcon.commit()
        wcon.close()
        print(f"reaped {len(stale)} stale sessions, killed {sum(len(matched.get(k, [])) for k in stale) + len(orphan_pids)} procs")
    except Exception as exc:  # noqa: BLE001
        print(f"session reap failed: {exc}", file=sys.stderr)


def scan_stale_prompt(detailed):
    """Bounce sessions started before their step's prompt was updated.

    A session reads the step prompt once at start; prompt edits don't
    propagate to running sessions. Retire stale ones so the relaunch
    picks up the new prompt.
    """
    try:
        import sqlite3

        con = sqlite3.connect(f"file:{DB}?mode=ro", uri=True)
        prompts = dict(con.execute(
            "select id,updated_at from workflow_steps where workflow_id=?", (WORKFLOW,)
        ).fetchall())
        con.close()
    except Exception as exc:  # noqa: BLE001
        print(f"stale-prompt scan failed: {exc}", file=sys.stderr)
        return
    for task in detailed:
        step = task.get("workflow_step_id")
        if step not in (AGENT, REVIEW):
            continue
        if "human-hold" in lane_labels(task):
            continue
        pupd = prompts.get(step)
        if not pupd:
            continue
        try:
            import sqlite3
            from datetime import datetime, timezone

            con = sqlite3.connect(f"file:{DB}?mode=ro", uri=True)
            rows = con.execute(
                "select id,started_at from task_sessions where task_id=? and state in ('RUNNING','WAITING_FOR_INPUT','SCHEDULING')",
                (task["id"],),
            ).fetchall()
            con.close()
            pdt = datetime.fromisoformat(pupd.replace("Z", "+00:00"))
            if pdt.tzinfo is None:
                pdt = pdt.replace(tzinfo=timezone.utc)
        except Exception:  # noqa: BLE001
            continue
        stale = []
        for sid, started in rows:
            try:
                sdt = datetime.fromisoformat(started.replace("Z", "+00:00"))
                if sdt.tzinfo is None:
                    sdt = sdt.replace(tzinfo=timezone.utc)
                if sdt < pdt:
                    stale.append(sid)
            except Exception:  # noqa: BLE001
                continue
        if not stale:
            continue
        try:
            import sqlite3

            wcon = sqlite3.connect(DB)
            for sid in stale:
                wcon.execute(
                    "update task_sessions set state='FAILED',completed_at=datetime('now') where id=?", (sid,)
                )
                wcon.execute(
                    "update executors_running set status='stopped',local_pid=0,pid=0 where session_id=?", (sid,)
                )
            wcon.commit()
            wcon.close()
        except Exception as exc:  # noqa: BLE001
            print(f"stale-prompt retire failed: {exc}", file=sys.stderr)
            continue
        if move(task, INBOX, "stale prompt -> Inbox"):
            time.sleep(1)
            move(task, step, "stale prompt -> back with new prompt")


def validate_card(task_id):
    """Static admission gate. Returns list of problems (empty = pass).

    Checks (Stage A, deterministic, no LLM):
    1. scope block parses (write/read/forbid present)
    2. 证据 paths all covered by scope.write
    3. 验收 has executable commands (backticks) or is Docs (prose ok)
    4. labels include lane:*
    5. no unresolved {{...}} placeholders
    """
    import re
    import sqlite3

    problems = []
    try:
        con = sqlite3.connect(f"file:{DB}?mode=ro", uri=True)
        con.row_factory = sqlite3.Row
        r = con.execute(
            "select title,description,labels from tasks where id=?", (task_id,)
        ).fetchone()
        con.close()
    except Exception as exc:  # noqa: BLE001
        return [f"db read failed: {exc}"]
    if not r:
        return ["task not found"]
    title, desc, labels = r["title"], r["description"] or "", r["labels"] or "[]"
    cat_m = re.search(r"\((Docs|Audit|Contract|Core|UI|Integration|QA|Build|Release)\)", title)
    cat = cat_m.group(1) if cat_m else ""
    # 1. scope block
    m = re.search(r"<!-- kandev-scope(.*?)-->", desc, re.DOTALL)
    if not m:
        problems.append("missing <!-- kandev-scope --> block")
        return problems
    body = m.group(1)
    writes = []
    cur = None
    for line in body.splitlines():
        ms = re.match(r"^(write|read|forbid)\s*:\s*(\[\s*\])?$", line.strip())
        if ms:
            cur = ms.group(1)
            continue
        me = re.match(r"^-\s+(.*)$", line.strip())
        if me and cur == "write":
            writes.append(me.group(1).strip().rstrip("/"))
    if not writes:
        problems.append("scope.write is empty")
    # 2. evidence paths covered
    me = re.search(r"## 证据[^\n]*\n([^\n#]+(?:\n[^\n#]+)*)", desc)
    if me:
        for em in re.finditer(r"[\w./-]*evidence/[\w./-]*", me.group(1)):
            ev = em.group(0).rstrip("/")
            if ev and not any(ev == w or ev.startswith(w.rstrip("*").rstrip("/")) for w in writes):
                problems.append(f"evidence path not in scope.write: {ev}")
    # 3. acceptance executable (Docs prose exempt)
    ma = re.search(r"## 验收[^\n]*\n(.*?)(?=\n## |\Z)", desc, re.DOTALL)
    if not ma:
        problems.append("missing ## 验收 section")
    elif cat != "Docs" and "`" not in ma.group(1):
        problems.append("验收 has no executable commands (no backticks)")
    # 4. lane label
    if "lane:" not in labels:
        problems.append("missing lane:* label")
    # 5. placeholders
    if re.search(r"\{\{[^}]+\}\}", desc):
        problems.append("unresolved {{...}} placeholder in description")
    return problems


def scan_deadlock():
    """Detect blocked_by cycles; notify once per cycle (label on each member)."""
    try:
        import sqlite3

        con = sqlite3.connect(f"file:{DB}?mode=ro", uri=True)
        edges = con.execute("select task_id,blocker_task_id from task_blockers").fetchall()
        titles = dict(con.execute("select id,title from tasks where archived_at is null"))
        con.close()
    except Exception as exc:  # noqa: BLE001
        print(f"deadlock scan failed: {exc}", file=sys.stderr)
        return
    graph = {}
    for a, b in edges:
        graph.setdefault(a, []).append(b)
    cycles = []
    visited, stack = set(), []
    def dfs(n):
        if n in stack:
            cycles.append(stack[stack.index(n):] + [n])
            return
        if n in visited or n not in titles:
            return
        visited.add(n)
        stack.append(n)
        for m in graph.get(n, []):
            dfs(m)
        stack.pop()
    for n in list(graph):
        if n in titles:
            dfs(n)
    for cyc in cycles:
        names = [titles.get(t, t[:8])[:30] for t in cyc]
        try:
            import sqlite3

            con = sqlite3.connect(f"file:{DB}?mode=ro", uri=True)
            sent = any(
                "tg-deadlock-sent" in (con.execute("select labels from tasks where id=?", (t,)).fetchone()[0] or "")
                for t in cyc if t in titles
            )
            con.close()
        except Exception:  # noqa: BLE001
            sent = False
        if sent:
            continue
        tg_send(
            "Kandev 死锁：blocked_by 成环，谁也动不了。\n"
            + " → ".join(names)
            + "\n请人工断环（删掉反向的那条）。"
        )
        print(f"DEADLOCK: {' -> '.join(names)}", file=sys.stderr)
        for t in cyc:
            if t in titles:
                try:
                    add_label(t, "tg-deadlock-sent")
                except Exception:  # noqa: BLE001
                    pass


RETRO_LUNA = "e930e6a0-835f-4e84-834e-09dea519280c"

RETRO_PROMPT = """你是 AsterLab 的阶段回顾 agent。本次回顾范围只包括以下任务（不要写范围外的卡）：

{task_list}

## 范围
<!-- kandev-scope
write:
  - docs/release/retrospective-{stamp}.md
read:
  - AGENTS.md
  - docs/engineering/kandev-card-template.md
forbid:
  - crates/**
  - src/**
  - prototypes/**
  - docs/contracts/**
-->

## 验收
- 命令：`test -f docs/release/retrospective-{stamp}.md && grep -c '^## ' docs/release/retrospective-{stamp}.md` 期望：exit 0 且计数 >= 6（六节齐全）
- 命令：`gh pr view --json state,mergeable --jq '"\\(.state) \\(.mergeable)"'`（在 PR 分支上） 期望：`OPEN MERGEABLE`

数据源（只读，不许改）：
1. Kandev DB（/var/lib/kandev/data/kandev.db，只读连接）：上述任务的 verdict 历史、返工次数、熔断记录、session 消息
2. `gh pr view/diff/checks`：每个任务关联 PR 的 diff 摘要与 CI 记录
3. `git log`：本阶段提交（只看上述任务相关的 PR 合并提交）
4. 本次范围的时间窗口：{since} 之后进 Merged 的 VPS 卡

报告必须包含（写中文）：
1. 改动总结：按模块分（Core/CLI/CI/Docs），每模块一句话
2. 出现的问题：每个一句话 + 影响的卡 + 怎么解决的
3. 做的调整：流程/配置变更及原因
4. 可能的问题：从返工模式、CI 失败模式推断的隐患
5. 项目状况：Milestone 进度、技术债清单（只基于本次范围+已知的 Windows lane 状态，不要编造）
6. 下一步方向：3-5 个候选，每个一句话 + 理由 + 依赖。不做决定，等人工拍板。

输出：
2. 提交到分支 agent/RETRO-{stamp} 并开 PR（标题带 RETRO-{stamp}），PR 描述贴报告全文，PR 正文末尾附 `<!-- kandev-task: <本任务id> -->` 和上面的 scope 块原文
3. 开完 PR 后直接结束 turn。不要写任务评论（你没有评论工具），不要合并 PR。系统检测到 PR 后会自动通知人工来看报告。
不要合并 PR，等人工看完报告再决定。
"""


def scan_retro_trigger():
    """Create a retrospective card once per lane drain.

    Fires when VPS Inbox+Ready+Agent+Review are all empty AND new cards
    merged since the watermark. Watermark in kandev_meta; scope is
    post-watermark merges only, never re-covered.
    """
    import datetime

    try:
        import sqlite3

        con = sqlite3.connect(f"file:{DB}?mode=ro", uri=True)
        open_n = con.execute(
            """select count(*) from tasks where archived_at is null and workflow_id=?
               and workflow_step_id != ?""",
            (WORKFLOW, MERGED_STEP),
        ).fetchone()[0]
        if open_n > 0:
            con.close()
            return
        wm = con.execute("select value from kandev_meta where key='retro.vps.last_run_at'").fetchone()
        since = wm[0] if wm else "1970-01-01T00:00:00Z"
        rows = con.execute(
            """select t.id,t.title,max(tr.occurred_at) m from task_step_transitions tr
                join tasks t on t.id=tr.task_id
               where t.workflow_id=? and tr.to_workflow_step_id=? and tr.occurred_at > ?
               group by t.id order by m""",
            (WORKFLOW, MERGED_STEP, since),
        ).fetchall()
        con.close()
    except Exception as exc:  # noqa: BLE001
        print(f"retro scan failed: {exc}", file=sys.stderr)
        return
    if not rows:
        return
    # Skip RETRO cards themselves (avoid self-trigger loop).
    rows = [r for r in rows if not r[1].startswith("RETRO-")]
    if not rows:
        return
    task_list = "\n".join(f"- {r[1]} (merged {r[2]})" for r in rows)
    task_ids = [r[0] for r in rows]
    stamp = datetime.datetime.now().strftime("%Y%m%d")
    status, data = api(
        "POST",
        "/api/v1/tasks",
        {
            "workspace_id": WORKSPACE,
            "workflow_id": WORKFLOW,
            "title": f"RETRO-{stamp} (Audit) VPS lane 阶段回顾",
            "description": RETRO_PROMPT.format(task_list=task_list, since=since, stamp=stamp),
            "labels": ["retro", "roadmap", "lane:vps"],
            "priority": "high",
            "agent_profile_id": RETRO_LUNA,
            "repository_id": "a26e8981-6ae8-42ff-845e-1bd312072abd",
        },
    )
    if status not in (200, 201):
        print(f"retro create failed: {status} {data}", file=sys.stderr)
        return
    print(f"retro triggered for {len(rows)} cards: {data.get('id', '?')[:8]}")
    tid = data.get("id")
    try:
        import sqlite3
        import uuid

        wcon = sqlite3.connect(DB)
        # No API for repo binding; write task_repositories directly.
        wcon.execute(
            """insert into task_repositories(id,task_id,repository_id,workspace_relative_path,
               base_branch,checkout_branch,position,metadata,created_at,updated_at)
               values (?,?,?,?,?,?,?,datetime('now'),datetime('now'))""",
            (str(uuid.uuid4()), tid, "a26e8981-6ae8-42ff-845e-1bd312072abd",
             "", "main", "", 0,
             '{"base_branch_selection":{"version":1,"branch":"main","source":"explicit"}}'),
        )
        wcon.commit()
        wcon.close()
    except Exception as exc:  # noqa: BLE001
        print(f"retro repo bind failed: {exc}", file=sys.stderr)
        return
        import sqlite3

        wcon = sqlite3.connect(DB)
        now = datetime.datetime.now(datetime.timezone.utc).isoformat()
        wcon.execute(
            "insert into kandev_meta(key,value) values('retro.vps.last_run_at',?) "
            "on conflict(key) do update set value=excluded.value",
            (now,),
        )
        wcon.execute(
            "insert into kandev_meta(key,value) values('retro.vps.last_task_ids',?) "
            "on conflict(key) do update set value=excluded.value",
            (json.dumps(task_ids),),
        )
        wcon.commit()
        wcon.close()
    except Exception as exc:  # noqa: BLE001
        print(f"retro watermark failed: {exc}", file=sys.stderr)


def scan_stuck(detailed):
    """Self-heal cards with no live session; escalate after STUCK_FIX_LIMIT."""
    import datetime

    cap = session_capacity()
    if len(live_session_ids()) >= cap:
        return  # cards waiting for a session slot are not stuck
    for task in detailed:
        step = task.get("workflow_step_id")
        if step not in (AGENT, REVIEW):
            continue
        labels = lane_labels(task)
        if "human-hold" in labels or "stuck-escalated" in labels:
            continue
        try:
            import sqlite3

            con = sqlite3.connect(f"file:{DB}?mode=ro", uri=True)
            live = con.execute(
                """select id from task_sessions
                    where task_id=? and state in ('RUNNING','WAITING_FOR_INPUT','SCHEDULING')""",
                (task["id"],),
            ).fetchall()
            last = con.execute(
                "select max(occurred_at) from task_step_transitions where task_id=?",
                (task["id"],),
            ).fetchone()[0]
            con.close()
        except Exception as exc:  # noqa: BLE001
            print(f"stuck scan failed: {exc}", file=sys.stderr)
            continue
        if live or not last:
            continue
        try:
            dt = datetime.datetime.fromisoformat(last.replace("Z", "+00:00"))
            idle_min = (datetime.datetime.now(datetime.timezone.utc) - dt).total_seconds() / 60
        except Exception:  # noqa: BLE001
            continue
        if idle_min < STUCK_MINUTES:
            continue
        fixes = stuck_fix_count(labels)
        if fixes >= STUCK_FIX_LIMIT:
            try:
                add_label(task["id"], "stuck-escalated")
            except Exception as exc:  # noqa: BLE001
                print(f"escalate label failed: {exc}", file=sys.stderr)
            tg_send(
                f"Kandev 卡住升级：{task['title'][:60]}\n"
                f"无存活 session 已 {idle_min:.0f} 分钟，自修 {fixes} 次无效，等人工。\n"
                f"task_id={task['id']}"
            )
            print(f"STUCK ESCALATED: {task['title'][:40]}", file=sys.stderr)
            continue
        if move(task, INBOX, f"stuck self-heal #{fixes + 1} -> Inbox"):
            time.sleep(1)
            if move(task, step, f"stuck self-heal #{fixes + 1} -> back"):
                try:
                    add_label(task["id"], f"stuck-fix-{fixes + 1}")
                except Exception as exc:  # noqa: BLE001
                    print(f"stuck-fix label failed: {exc}", file=sys.stderr)


def rework_count(task_id):
    """How many CHANGES REQUESTED verdicts this task got in Review.

    Rework happens in place (same session, no step moves), so rounds are
    counted by verdicts, not by Review -> Agent transitions.
    """
    try:
        import sqlite3

        con = sqlite3.connect(f"file:{DB}?mode=ro", uri=True)
        rows = con.execute(
            """select m.content from task_session_messages m
                join task_sessions s on s.id=m.task_session_id
               where s.task_id=? and m.author_type='agent' and m.type='message'
                 and (m.content like '%VERDICT: CHANGES%' or m.content like '%REVIEW: CHANGES REQUESTED%')
               order by m.created_at""",
            (task_id,),
        ).fetchall()
        con.close()
        n = 0
        for (content,) in rows:
            if "你是 AsterLab 的 Review agent" in content[:300]:
                continue  # review prompt quoting the format, not a verdict
            first = content.strip().splitlines()[0].strip() if content.strip() else ""
            if first == "VERDICT: CHANGES":
                n += 1
            elif content.strip().startswith("**REVIEW: CHANGES REQUESTED") or content.strip().startswith("REVIEW: CHANGES REQUESTED"):
                n += 1  # legacy format
        return n
    except Exception as exc:  # noqa: BLE001 - never block feeding on stats
        print(f"rework count failed: {exc}", file=sys.stderr)
        return 0


def tg_send(text, reply_markup=None):
    """Send a Telegram message; no-op when token/chat unset."""
    if not (TG_TOKEN and TG_CHAT):
        return False
    try:
        body = {"chat_id": TG_CHAT, "text": text}
        if reply_markup:
            body["reply_markup"] = reply_markup
        req = urllib.request.Request(
            f"https://api.telegram.org/bot{TG_TOKEN}/sendMessage",
            data=json.dumps(body).encode(),
            headers={"Content-Type": "application/json"},
        )
        urllib.request.urlopen(req, timeout=15)
        return True
    except Exception as exc:  # noqa: BLE001
        print(f"telegram failed: {exc}", file=sys.stderr)
        return False


def notify_human_hold(task, rounds):
    text = (
        f"Kandev 熔断：{task['title'][:60]}\n"
        f"返工 {rounds} 次仍未过 Review，已留在 Review 等人工。\n"
        f"task_id={task['id']}"
    )
    tg_send(text)
    print(f"HUMAN HOLD: {text}", file=sys.stderr)



def task_category(task):
    import re

    m = re.search(r"\((Docs|Audit|Contract|Core|UI|Integration|QA|Build|Release)\)", task.get("title", ""))
    return m.group(1) if m else ""


def latest_review_verdict(task_id):
    """Latest verdict from agent messages. Returns (verdict, content).

    New format (strict): first line is exactly one of
      VERDICT: APPROVED | VERDICT: CHANGES | VERDICT: BLOCKED-NEEDS-HUMAN
    Legacy format (transitional): REVIEW: APPROVED / REVIEW: CHANGES REQUESTED
    anywhere in the message. Legacy support can be dropped once all open
    cards have cycled through the new prompt.
    """
    try:
        import sqlite3

        con = sqlite3.connect(f"file:{DB}?mode=ro", uri=True)
        rows = con.execute(
            """select m.content from task_session_messages m
                join task_sessions s on s.id=m.task_session_id
               where s.task_id=? and m.author_type='agent' and m.type='message'
               order by m.created_at desc limit 20""",
            (task_id,),
        ).fetchall()
        con.close()
        for (content,) in rows:
            if not content:
                continue
            if "你是 AsterLab 的 Review agent" in content[:300]:
                continue
            first = content.strip().splitlines()[0].strip() if content.strip() else ""
            if first == "VERDICT: APPROVED":
                return "APPROVED", content
            if first == "VERDICT: CHANGES":
                return "CHANGES_REQUESTED", content
            if first == "VERDICT: BLOCKED-NEEDS-HUMAN":
                return "BLOCKED", content
        # Legacy fallback: old REVIEW: format anywhere in message.
        for (content,) in rows:
            if not content or "你是 AsterLab 的 Review agent" in content[:300]:
                continue
            if "REVIEW: APPROVED" in content:
                return "APPROVED", content
            if "REVIEW: CHANGES REQUESTED" in content:
                return "CHANGES_REQUESTED", content
        return "", ""
    except Exception as exc:  # noqa: BLE001
        print(f"verdict read failed: {exc}", file=sys.stderr)
        return "", ""


def task_pr(task_id):
    try:
        import sqlite3

        con = sqlite3.connect(f"file:{DB}?mode=ro", uri=True)
        con.row_factory = sqlite3.Row
        r = con.execute(
            "select pr_number,pr_url,state from github_task_prs where task_id=? order by pr_number desc limit 1",
            (task_id,),
        ).fetchone()
        con.close()
        return dict(r) if r else {}
    except Exception:  # noqa: BLE001
        return {}


GH_REPO = "LLLin000/AsterLab"
_merged_pr_cache = {}


def merged_pr_by_card(task):
    """Find a merged PR by card id when Kandev did not link one.

    Linking can miss (for example when the review agent merges before the
    GitHub watcher associates the PR), leaving a merged card in Review.
    PR titles start with the card id, so a title search recovers it.
    """
    m = re.match(r"^([A-Z]+-\d+[a-z]?)", task.get("title", ""))
    if not m:
        return None
    cid = m.group(1)
    now = time.time()
    hit = _merged_pr_cache.get(cid)
    if hit and now - hit[0] < 300:
        return hit[1]
    found = None
    try:
        out = subprocess.run(
            [
                "gh", "pr", "list", "--repo", GH_REPO, "--state", "merged",
                "--search", f'"{cid}" in:title',
                "--json", "number,title,url,mergedAt", "--limit", "10",
            ],
            capture_output=True, text=True, timeout=25,
        )
        for pr in json.loads(out.stdout or "[]"):
            if cid not in pr.get("title", ""):
                continue
            found = {
                "pr_number": pr["number"],
                "pr_url": pr.get("url", ""),
                "state": "merged",
            }
            break
    except Exception as exc:  # noqa: BLE001
        print(f"merged PR lookup failed for {cid}: {exc}", file=sys.stderr)
    _merged_pr_cache[cid] = (now, found)
    return found


def pr_for_task(task):
    """Linked PR, else a merged PR recovered by card id."""
    pr = task_pr(task["id"])
    if pr:
        return pr
    return merged_pr_by_card(task)


def add_label(task_id, label):
    """Append a label via direct DB write (no write API exists)."""
    import sqlite3

    wcon = sqlite3.connect(DB)
    cur = json.loads(
        wcon.execute("select labels from tasks where id=?", (task_id,)).fetchone()[0] or "[]"
    )
    if label not in cur:
        cur.append(label)
        wcon.execute("update tasks set labels=? where id=?", (json.dumps(cur), task_id))
        wcon.commit()
    wcon.close()


def notify_approval(task, pr):
    text = (
        f"Kandev 待决策：{task['title'][:60]}\n"
        f"REVIEW: APPROVED · PR #{pr.get('pr_number')} {pr.get('state','')}\n"
        f"{pr.get('pr_url','')}"
    )
    markup = {
        "inline_keyboard": [
            [
                {"text": "✅ 合并", "callback_data": f"merge:{task['id']}"},
                {"text": "👀 看diff", "callback_data": f"diff:{task['id']}"},
                {"text": "❌ 打回", "callback_data": f"reject:{task['id']}"},
            ]
        ]
    }
    if tg_send(text, markup):
        print(f"approval sent for {task['title'][:40]}")


def scan_retro_ready(detailed):
    """Push when a RETRO card's report PR exists and is open.

    No keyword matching: the PR itself is the signal. The agent writes
    REVIEW-READY in a task comment, but comments share the message table
    with prompts that quote the format, so text matching misfires.
    """
    for task in detailed:
        if not task.get("title", "").startswith("RETRO-"):
            continue
        labels = lane_labels(task)
        if "tg-retro-sent" in labels:
            continue
        pr = task_pr(task["id"])
        if not pr or pr.get("state") != "open":
            continue

        text = (
            f"Kandev 阶段回顾已就绪：{task['title'][:50]}\n"
            f"PR #{pr.get('pr_number', '?')} {pr.get('pr_url', '')}\n"
            "看完报告后回复：按建议开卡 / 我改完再说 / 先停"
        )
        markup = {
            "inline_keyboard": [
                [
                    {"text": "✅ 按建议开卡", "callback_data": f"retro_go:{task['id']}"},
                    {"text": "📝 我改完再说", "callback_data": f"retro_hold:{task['id']}"},
                    {"text": "⏸ 先停", "callback_data": f"retro_stop:{task['id']}"},
                ]
            ]
        }
        if tg_send(text, markup):
            print(f"retro sent for {task['title'][:40]}")
        try:
            add_label(task["id"], "tg-retro-sent")
        except Exception as exc:  # noqa: BLE001
            print(f"retro-sent label failed: {exc}", file=sys.stderr)


def scan_approvals(review_tasks):
    """Push decision requests for APPROVED cards still sitting in Review.

    The review agent merges by itself when confident; a card still in Review
    with APPROVED means the agent was unsure and wants a human call.
    """
    for task in review_tasks:
        labels = lane_labels(task)
        if "human-hold" in labels or "tg-approved-sent" in labels:
            continue
        verdict, _ = latest_review_verdict(task["id"])
        if verdict != "APPROVED":
            continue
        pr = task_pr(task["id"])
        if not pr or pr.get("state") != "open":
            continue
        notify_approval(task, pr)
        try:
            add_label(task["id"], "tg-approved-sent")
        except Exception as exc:  # noqa: BLE001
            print(f"approved-sent label failed: {exc}", file=sys.stderr)


CARD_FIX_KEYWORDS = ("修卡", "人工修卡", "需要人工", "二选一", "改任务描述", "改卡", "CARD-BROKEN", "自矛盾", "澄清范围", "范围不一致", "scope 不一致", "先澄清")


def scan_cardfix(tasks):
    """Push when an agent reports the card itself is broken.

    Two sources: Review CHANGES verdicts mentioning a card fix, and Agent
    preflight CARD-BROKEN reports. The agent cannot edit task descriptions;
    these need a human card fix.
    """
    for task in tasks:
        if task.get("title", "").startswith("RETRO-"):
            continue  # retro cards report via scan_retro_ready, not cardfix
        labels = lane_labels(task)
        if "human-hold" in labels or "tg-cardfix-sent" in labels:
            continue
        verdict, content = latest_review_verdict(task["id"])
        hit = (verdict == "BLOCKED") or (
            verdict == "CHANGES_REQUESTED" and any(k in content for k in CARD_FIX_KEYWORDS)
        )
        if not hit:
            # Agent preflight: CARD-BROKEN in any recent agent message.
            try:
                import sqlite3

                con = sqlite3.connect(f"file:{DB}?mode=ro", uri=True)
                row = con.execute(
                    """select m.content from task_session_messages m
                        join task_sessions s on s.id=m.task_session_id
                       where s.task_id=? and m.author_type='agent'
                         and m.content like '%CARD-BROKEN%'
                       order by m.created_at desc limit 1""",
                    (task["id"],),
                ).fetchone()
                con.close()
            except Exception:  # noqa: BLE001
                row = None
            if not row:
                continue
            content = row[0]
        pr = task_pr(task["id"])
        if not pr and merged_pr_by_card(task):
            continue  # card is merged; the sweep moves it, not a card fix
        # Snippet around the keyword hit, not the head: the reason for
        # human attention is where the keyword is.
        snippet = content[:300]
        for k in CARD_FIX_KEYWORDS:
            i = content.find(k)
            if i >= 0:
                s = max(0, content.rfind("\n", 0, i - 100))
                snippet = content[s : i + 250].strip()
                break
        text = (
            f"Kandev 修卡请求：{task['title'][:60]}\n"
            f"卡片自矛盾，agent 修不动，已停工等你。\n"
            f"PR #{pr.get('pr_number', '?')} {pr.get('pr_url', '')}\n"
            f"原因：{snippet[:400]}"
        )
        markup = {
            "inline_keyboard": [
                [
                    {"text": "🔧 我来修卡", "callback_data": f"cardfix:{task['id']}"},
                    {"text": "👀 看diff", "callback_data": f"diff:{task['id']}"},
                ]
            ]
        }
        if tg_send(text, markup):
            print(f"cardfix sent for {task['title'][:40]}")
        try:
            add_label(task["id"], "tg-cardfix-sent")
        except Exception as exc:  # noqa: BLE001
            print(f"cardfix-sent label failed: {exc}", file=sys.stderr)


ARCHIVE_AFTER_DAYS = int(os.environ.get("KANDEV_ARCHIVE_AFTER_DAYS", "7"))


def scan_archive_merged():
    """Archive Merged cards older than ARCHIVE_AFTER_DAYS.

    Archiving keeps sessions/messages but lets Kandev reclaim the worktree.
    """
    import datetime

    try:
        import sqlite3

        con = sqlite3.connect(f"file:{DB}?mode=ro", uri=True)
        con.row_factory = sqlite3.Row
        rows = con.execute(
            """select t.id,t.title,max(tr.occurred_at) m from tasks t
                join task_step_transitions tr on tr.task_id=t.id
               where t.archived_at is null and t.workflow_step_id=?
                 and tr.to_workflow_step_id=?
               group by t.id""",
            (MERGED_STEP, MERGED_STEP),
        ).fetchall()
        con.close()
    except Exception as exc:  # noqa: BLE001
        print(f"archive scan failed: {exc}", file=sys.stderr)
        return
    now = datetime.datetime.now(datetime.timezone.utc)
    for r in rows:
        try:
            dt = datetime.datetime.fromisoformat(r["m"].replace("Z", "+00:00"))
            age = (now - dt).total_seconds() / 86400
        except Exception:  # noqa: BLE001
            continue
        if age < ARCHIVE_AFTER_DAYS:
            continue
        status, _ = api("POST", f"/api/v1/tasks/{r['id']}/archive", {})
        print(f"archive {r['title'][:40]} (merged {age:.0f}d ago): {status}")


def scan_ci_merges(detailed):
    """FYI when a card touching .github/** gets merged."""
    for task in detailed:
        if task.get("workflow_step_id") != MERGED_STEP:
            continue
        labels = lane_labels(task)
        if "tg-ci-merged-sent" in labels:
            continue
        try:
            import sqlite3

            con = sqlite3.connect(f"file:{DB}?mode=ro", uri=True)
            desc = con.execute(
                "select description from tasks where id=?", (task["id"],)
            ).fetchone()[0] or ""
            con.close()
        except Exception:  # noqa: BLE001
            continue
        scope = desc.split("<!-- kandev-scope", 1)
        if len(scope) < 2 or ".github/" not in scope[1].split("-->", 1)[0]:
            continue
        pr = task_pr(task["id"])
        tg_send(
            f"Kandev 知会：CI 配置已合并\n{task['title'][:60]}\n"
            f"PR #{pr.get('pr_number', '?')} {pr.get('pr_url', '')}"
        )
        try:
            add_label(task["id"], "tg-ci-merged-sent")
        except Exception as exc:  # noqa: BLE001
            print(f"ci-merged label failed: {exc}", file=sys.stderr)


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
    scan_session_reaper(detailed)
    active = [t for t in agent_tasks if t.get("state") in ACTIVE_STATES]
    cap = session_capacity()
    used = len(live_session_ids())
    slots = max(0, min(WIP - len(active), cap - used))

    # A blocked card in Ready is a bug: put it back before it can start.
    for task in detailed:
        if task.get("workflow_step_id") == READY and not is_eligible(task):
            move(task, INBOX, "Ready blocked -> Inbox")

    # Rework circuit breaker: a card that bounced Review -> Agent REWORK_LIMIT
    # times stays in Review for a human. Notify once (tracked by label).
    review_tasks = [t for t in detailed if t.get("workflow_step_id") == REVIEW]
    for task in review_tasks:
        labels = lane_labels(task)
        if "human-hold" in labels:
            continue
        rounds = rework_count(task["id"])
        if rounds >= REWORK_LIMIT:
            try:
                add_label(task["id"], "human-hold")
                notify_human_hold(task, rounds)
            except Exception as exc:  # noqa: BLE001
                print(f"human-hold label failed: {exc}", file=sys.stderr)

    # Merged sweep: PR merged but card still in Review/Verify (missed event).
    for task in detailed:
        if task.get("workflow_step_id") not in (REVIEW, VERIFY_STEP):
            continue
        pr = pr_for_task(task)
        if pr and pr.get("state") == "merged":
            move(task, MERGED_STEP, "PR merged -> Merged (sweep)")

    scan_approvals(review_tasks)
    scan_cardfix([t for t in detailed if t.get("workflow_step_id") in (AGENT, REVIEW)])
    scan_retro_ready(detailed)
    scan_ci_ready(review_tasks)
    scan_no_pr(review_tasks)

    # FYI: CI-config cards that got merged.
    scan_ci_merges(detailed)

    # Stuck self-heal before backpressure: a card with no live session
    # gets bounced to trigger a fresh one; escalates after 3 fixes.
    scan_stuck(detailed)
    scan_deadlock()
    scan_retro_trigger()
    scan_stale_prompt(detailed)

    # Backpressure: do not feed new work while Review is full.
    # human-hold cards still occupy Review slots: clear them by hand.
    if len(review_tasks) >= REVIEW_WIP:
        print(f"review full ({len(review_tasks)}/{REVIEW_WIP}); not feeding")
        return

    # Fill Ready with every eligible Inbox card (admission gate first).
    for task in detailed:
        if task.get("workflow_step_id") == INBOX and is_eligible(task) and task.get("state") == "CREATED":
            if "dor-fail" in lane_labels(task):
                continue
            problems = validate_card(task["id"])
            if problems:
                try:
                    add_label(task["id"], "dor-fail")
                except Exception:  # noqa: BLE001
                    pass
                tg_send(
                    f"Kandev 建卡校验失败：{task['title'][:60]}\n"
                    + "\n".join(f"· {p}" for p in problems[:8])
                    + f"\n已打标 dor-fail 留在 Inbox，修好卡后去掉 label 即可放行。"
                )
                print(f"DOR FAIL {task['title'][:40]}: {problems}", file=sys.stderr)
                continue
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
