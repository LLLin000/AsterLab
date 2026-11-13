#!/usr/bin/env python3
"""Telegram decision callback service for Kandev.

Long-polls getUpdates for inline-keyboard presses on approval messages:
  merge:<task_id>   squash-merge the task's PR, move card toward Merged
  diff:<task_id>    send a diff summary (stat + first ~60 lines)
  reject:<task_id>  move the card back to Agent for rework

Safety: every merge press is re-verified before acting —
  1. task still in Review, no human-hold label
  2. latest review verdict still APPROVED
  3. PR still open and mergeable
  4. required checks (linux-tests, windows-build, scope-check) all green
Any failure aborts with a Telegram reply explaining why; nothing merges.

Environment: same file as the feeder (/etc/kandev-ready-feeder.env) plus
  KANDEV_REPO_DIR default /srv/projects/AsterLab (for gh commands)
"""

import json
import os
import subprocess
import sys
import time
import urllib.error
import urllib.request

BASE = os.environ.get("KANDEV_URL", "http://127.0.0.1:38429").rstrip("/")
TOKEN = os.environ.get("KANDEV_API_TOKEN", "")
WORKFLOW = os.environ.get("KANDEV_VPS_WORKFLOW_ID", "d1a08c74-42ab-4e8d-aee3-a67b459537f8")
AGENT = os.environ.get("KANDEV_VPS_AGENT_STEP", "47b1aac7-2bb9-4b26-b8e3-412c424b4f46")
REVIEW = os.environ.get("KANDEV_VPS_REVIEW_STEP", "2b6d9951-e9fd-4578-8bf4-6f3e7aa7ce31")
VERIFY = "a5f7c8e1-0c9e-4e3b-9f3a-2e4b7c6d8e9f"  # resolved at runtime, see below
DB = os.environ.get("KANDEV_DB", "/var/lib/kandev/data/kandev.db")
TG_TOKEN = os.environ.get("TELEGRAM_BOT_TOKEN", "")
TG_CHAT = os.environ.get("TELEGRAM_CHAT_ID", "")
REPO_DIR = os.environ.get("KANDEV_REPO_DIR", "/srv/projects/AsterLab")
REQUIRED_CHECKS = {"linux-tests", "windows-build", "scope-check"}


def kapi(method, path, body=None):
    data = json.dumps(body).encode() if body is not None else None
    req = urllib.request.Request(
        BASE + path,
        data=data,
        method=method,
        headers={"Authorization": f"Bearer {TOKEN}", "Content-Type": "application/json"},
    )
    try:
        with urllib.request.urlopen(req, timeout=20) as resp:
            return resp.status, json.loads(resp.read().decode() or "{}")
    except urllib.error.HTTPError as exc:
        return exc.code, {"error": exc.read().decode(errors="replace")}
    except Exception as exc:  # noqa: BLE001
        return 0, {"error": str(exc)}


def tg(method, payload):
    req = urllib.request.Request(
        f"https://api.telegram.org/bot{TG_TOKEN}/{method}",
        data=json.dumps(payload).encode(),
        headers={"Content-Type": "application/json"},
    )
    with urllib.request.urlopen(req, timeout=20) as resp:
        return json.loads(resp.read().decode())


def tg_answer(callback_id, text):
    try:
        tg("answerCallbackQuery", {"callback_query_id": callback_id, "text": text[:190]})
    except Exception as exc:  # noqa: BLE001
        print(f"answer failed: {exc}", file=sys.stderr)


def tg_say(text):
    tg("sendMessage", {"chat_id": TG_CHAT, "text": text})


def db():
    import sqlite3

    con = sqlite3.connect(f"file:{DB}?mode=ro", uri=True)
    con.row_factory = sqlite3.Row
    return con


def latest_verdict(task_id):
    con = db()
    rows = con.execute(
        """select m.content from task_session_messages m
            join task_sessions s on s.id=m.task_session_id
           where s.task_id=? and m.author_type='agent' and m.type='message'
             and (m.content like '%REVIEW: APPROVED%' or m.content like '%REVIEW: CHANGES REQUESTED%')
           order by m.created_at desc limit 5""",
        (task_id,),
    ).fetchall()
    con.close()
    for (content,) in rows:
        if "你是 AsterLab 的 Review agent" in content[:300]:
            continue
        if "REVIEW: APPROVED" in content:
            return "APPROVED"
        if "REVIEW: CHANGES REQUESTED" in content:
            return "CHANGES_REQUESTED"
    return ""


def task_pr(task_id):
    con = db()
    r = con.execute(
        "select pr_number,pr_url,state from github_task_prs where task_id=? order by pr_number desc limit 1",
        (task_id,),
    ).fetchone()
    con.close()
    return dict(r) if r else {}


def gh(*args):
    p = subprocess.run(
        ["gh", *args], cwd=REPO_DIR, capture_output=True, text=True, timeout=60
    )
    return p.returncode, (p.stdout + p.stderr).strip()


def verify_mergeable(task_id):
    """Re-check everything. Returns (ok, reason, pr_number)."""
    _, task = kapi("GET", f"/api/v1/tasks/{task_id}")
    if not task or "id" not in task:
        return False, "任务不存在或 API 不可达", 0
    if task.get("workflow_step_id") != REVIEW:
        return False, "任务已不在 Review（可能已移动）", 0
    if "human-hold" in (task.get("labels") or []):
        return False, "任务被熔断（human-hold），需人工在 Kandev 处理", 0
    if latest_verdict(task_id) != "APPROVED":
        return False, "最新 Review 结论不是 APPROVED（可能有新打回）", 0
    pr = task_pr(task_id)
    if not pr or pr.get("state") != "open":
        return False, "PR 已不在 open 状态", 0
    n = pr["pr_number"]
    rc, out = gh("pr", "view", str(n), "--json", "mergeable,mergeStateStatus,state")
    if rc != 0:
        return False, f"gh pr view 失败：{out[:200]}", n
    info = json.loads(out)
    if info.get("state") != "OPEN" or info.get("mergeable") != "MERGEABLE":
        return False, f"PR #{n} 不可合并（state={info.get('state')} mergeable={info.get('mergeable')})", n
    rc, out = gh("pr", "view", str(n), "--json", "statusCheckRollup")
    if rc != 0:
        return False, f"gh pr view checks 失败：{out[:200]}", n
    green = {
        c["name"]
        for c in json.loads(out).get("statusCheckRollup", [])
        if c.get("conclusion") == "SUCCESS"
    }
    missing = REQUIRED_CHECKS - green
    if missing:
        return False, f"必需检查未全绿：缺 {sorted(missing)}", n
    return True, "", n


def do_merge(task_id, title):
    ok, reason, n = verify_mergeable(task_id)
    if not ok:
        return f"⛔ 未合并 {title[:40]}：{reason}"
    rc, out = gh("pr", "merge", str(n), "--squash", "--delete-branch")
    if rc != 0:
        return f"⛔ 合并 PR #{n} 失败：{out[:300]}"
    # Move toward Verify/Merged: Verify listens on_pr_merged and completes.
    _, task = kapi("GET", f"/api/v1/tasks/{task_id}")
    st, _ = kapi("POST", f"/api/v1/tasks/{task_id}/move", {"workflow_id": WORKFLOW, "workflow_step_id": VERIFY_STEP})
    # Clear the approval-sent label so a future APPROVED can notify again.
    try:
        import sqlite3

        wcon = sqlite3.connect(DB)
        cur = json.loads(
            wcon.execute("select labels from tasks where id=?", (task_id,)).fetchone()[0] or "[]"
        )
        if "tg-approved-sent" in cur:
            cur.remove("tg-approved-sent")
            wcon.execute("update tasks set labels=? where id=?", (json.dumps(cur), task_id))
            wcon.commit()
        wcon.close()
    except Exception as exc:  # noqa: BLE001
        print(f"label clear failed: {exc}", file=sys.stderr)
    return f"✅ 已合并 PR #{n}（{title[:40]}），卡已移向 Verify（move={st}）"


def do_diff(task_id, title):
    pr = task_pr(task_id)
    if not pr:
        return f"{title[:40]}：无关联 PR"
    n = pr["pr_number"]
    rc, names = gh("pr", "diff", str(n), "--name-only")
    if rc != 0:
        return f"PR #{n} diff 读取失败：{names[:300]}"
    rc, body = gh("pr", "diff", str(n), "--patch")
    lines = body.splitlines()[:60]
    text = f"PR #{n} 文件：\n{names[:800]}\n\n前 60 行：\n" + "\n".join(lines)
    return text[:3500]


def do_reject(task_id, title):
    st, _ = kapi("POST", f"/api/v1/tasks/{task_id}/move", {"workflow_id": WORKFLOW, "workflow_step_id": AGENT})
    try:
        import sqlite3

        wcon = sqlite3.connect(DB)
        cur = json.loads(
            wcon.execute("select labels from tasks where id=?", (task_id,)).fetchone()[0] or "[]"
        )
        if "tg-approved-sent" in cur:
            cur.remove("tg-approved-sent")
            wcon.execute("update tasks set labels=? where id=?", (json.dumps(cur), task_id))
            wcon.commit()
        wcon.close()
    except Exception as exc:  # noqa: BLE001
        print(f"label clear failed: {exc}", file=sys.stderr)
    return f"↩️ {title[:40]} 已打回 Agent（move={st}），返工计数+1"


VERIFY_STEP = ""


def resolve_verify_step():
    global VERIFY_STEP
    import sqlite3

    con = sqlite3.connect(f"file:{DB}?mode=ro", uri=True)
    r = con.execute(
        "select id from workflow_steps where workflow_id=? and name='Verify'", (WORKFLOW,)
    ).fetchone()
    con.close()
    VERIFY_STEP = r[0] if r else ""
    if not VERIFY_STEP:
        print("FATAL: Verify step not found", file=sys.stderr)
        sys.exit(2)


def main():
    if not (TG_TOKEN and TG_CHAT and TOKEN):
        print("missing TELEGRAM_BOT_TOKEN / TELEGRAM_CHAT_ID / KANDEV_API_TOKEN", file=sys.stderr)
        return 2
    resolve_verify_step()
    print(f"callback service up, verify={VERIFY_STEP[:8]}", flush=True)
    offset = 0
    while True:
        try:
            data = tg("getUpdates", {"offset": offset, "timeout": 50, "allowed_updates": ["callback_query"]})
        except Exception as exc:  # noqa: BLE001
            print(f"getUpdates failed: {exc}", file=sys.stderr)
            time.sleep(5)
            continue
        for upd in data.get("result", []):
            offset = upd["update_id"] + 1
            cq = upd.get("callback_query") or {}
            chat = (cq.get("message") or {}).get("chat", {}).get("id")
            if str(chat) != str(TG_CHAT):
                continue
            raw = cq.get("data", "")
            action, _, tid = raw.partition(":")
            if action not in ("merge", "diff", "reject", "cardfix", "retro_go", "retro_hold", "retro_stop") or not tid:
                continue
            _, task = kapi("GET", f"/api/v1/tasks/{tid}")
            title = (task or {}).get("title", tid[:8])
            tg_answer(cq["id"], "收到，处理中…")
            try:
                if action == "merge":
                    res = do_merge(tid, title)
                elif action == "diff":
                    res = do_diff(tid, title)
                elif action == "cardfix":
                    res = (f"🔧 {title[:40]}：卡片问题已记录。\n"
                           "你改完任务描述后告诉我（回这条消息或发卡号），我来唤醒 review agent 重跑。")
                elif action == "retro_go":
                    res = ("✅ 收到，按报告建议开下一阶段。你把报告里认同的候选发我"
                           "（卡号或一句话），我来建卡；或者直接说“全按报告来”。")
                elif action == "retro_hold":
                    res = "📝 好的，你改完报告（PR 评论或直接说）再告诉我，我来推进。"
                elif action == "retro_stop":
                    res = "⏸ 收到，lane 保持清空不动。有需要再叫我。"
                else:
                    res = do_reject(tid, title)
            except Exception as exc:  # noqa: BLE001
                res = f"⛔ 处理异常：{exc}"
            try:
                tg_say(res)
            except Exception as exc:  # noqa: BLE001
                print(f"reply failed: {exc}", file=sys.stderr)


if __name__ == "__main__":
    raise SystemExit(main())
