#!/usr/bin/env python3
"""Daily 08:00 progress report to Telegram: last 24h of the VPS lane.

Sections:
  1. Merged (transitions into Merged in 24h)
  2. Review verdicts (APPROVED / CHANGES REQUESTED, agent messages only)
  3. Circuit breakers + stuck self-heals (labels)
  4. Failed sessions in 24h
  5. Current board snapshot (Inbox/Ready/Agent/Review counts)

Environment: /etc/kandev-ready-feeder.env (DB read-only + Telegram).
"""

import json
import os
import sqlite3
import sys
import urllib.request

DB = os.environ.get("KANDEV_DB", "/var/lib/kandev/data/kandev.db")
TG_TOKEN = os.environ.get("TELEGRAM_BOT_TOKEN", "")
TG_CHAT = os.environ.get("TELEGRAM_CHAT_ID", "")
WORKFLOW = os.environ.get("KANDEV_VPS_WORKFLOW_ID", "d1a08c74-42ab-4e8d-aee3-a67b459537f8")
INBOX = os.environ.get("KANDEV_VPS_INBOX_STEP", "1141b1f5-2ea7-44bb-b182-d7490b31a8e9")
READY = os.environ.get("KANDEV_VPS_READY_STEP", "9e7881ac-cda4-480c-8652-d3665f78645e")
AGENT = os.environ.get("KANDEV_VPS_AGENT_STEP", "47b1aac7-2bb9-4b26-b8e3-412c424b4f46")
REVIEW = os.environ.get("KANDEV_VPS_REVIEW_STEP", "2b6d9951-e9fd-4578-8bf4-6f3e7aa7ce31")
MERGED = "e22063df-1124-4836-8e5c-b0c0e387be68"


def short(title, n=34):
    return title[:n] + ("…" if len(title) > n else "")


def main():
    con = sqlite3.connect(f"file:{DB}?mode=ro", uri=True)
    con.row_factory = sqlite3.Row
    L = ["Kandev 日报（过去 24h）", ""]

    merged = con.execute(
        """select t.title from task_step_transitions tr join tasks t on t.id=tr.task_id
            where tr.to_workflow_step_id=? and tr.occurred_at > datetime('now','-1 day')
            order by tr.occurred_at""",
        (MERGED,),
    ).fetchall()
    L.append(f"✅ 合并 {len(merged)} 张：")
    L += [f"  · {short(r['title'])}" for r in merged] or ["  · 无"]
    L.append("")

    verdicts = con.execute(
        """select t.title,m.content,max(m.created_at) c from task_session_messages m
            join task_sessions s on s.id=m.task_session_id join tasks t on t.id=s.task_id
           where m.created_at > datetime('now','-1 day') and m.author_type='agent' and m.type='message'
             and (m.content like '%REVIEW: APPROVED%' or m.content like '%REVIEW: CHANGES REQUESTED%')
           group by t.id order by c desc"""
    ).fetchall()
    L.append(f"📝 Review 结论 {len(verdicts)} 条：")
    for r in verdicts:
        c = r["content"]
        v = "APPROVED" if "REVIEW: APPROVED" in c and "你是 AsterLab" not in c[:300] else (
            "CHANGES" if "REVIEW: CHANGES REQUESTED" in c and "你是 AsterLab" not in c[:300] else "?"
        )
        extra = ""
        if v == "CHANGES":
            # first finding line after the verdict
            for line in c.splitlines()[1:6]:
                line = line.strip(" -*")
                if len(line) > 10:
                    extra = f" — {line[:60]}"
                    break
        L.append(f"  · [{v}] {short(r['title'])}{extra}")
    if not verdicts:
        L.append("  · 无")
    L.append("")

    holds = con.execute(
        "select title,labels from tasks where archived_at is null and labels like '%human-hold%'"
    ).fetchall()
    stuck = con.execute(
        "select title,labels from tasks where archived_at is null and labels like '%stuck-%'"
    ).fetchall()
    L.append(f"⛔ 熔断 {len(holds)} 张，自修 {len(stuck)} 张：")
    for r in holds:
        L.append(f"  · 熔断 {short(r['title'])}")
    for r in stuck:
        L.append(f"  · 自修 {short(r['title'])} {r['labels']}")
    if not holds and not stuck:
        L.append("  · 无")
    L.append("")

    failed = con.execute(
        """select t.title,s.id,s.error_message from task_sessions s join tasks t on t.id=s.task_id
            where s.state='FAILED' and s.completed_at > datetime('now','-1 day')"""
    ).fetchall()
    real = [r for r in failed if (r["error_message"] or "").strip()]
    zombies = [r for r in failed if not (r["error_message"] or "").strip()]
    L.append(f"💥 真失败 session {len(real)} 个（僵尸清理 {len(zombies)} 个）：")
    for r in real:
        L.append(f"  · {short(r['title'])} ({r['id'][:8]}) {(r['error_message'] or '')[:60]}")
    for r in zombies:
        L.append(f"  · 僵尸 {short(r['title'])} ({r['id'][:8]})")
    if not failed:
        L.append("  · 无")
    L.append("")

    L.append("📋 当前看板：")
    for sid, name in ((INBOX, "Inbox"), (READY, "Ready"), (AGENT, "Agent"), (REVIEW, "Review")):
        rows = con.execute(
            "select title from tasks where archived_at is null and workflow_id=? and workflow_step_id=? order by title",
            (WORKFLOW, sid),
        ).fetchall()
        L.append(f"  {name}({len(rows)})：" + ("、".join(short(r["title"], 18) for r in rows) or "—"))
    con.close()

    text = "\n".join(L)[:3800]
    print(text)
    if TG_TOKEN and TG_CHAT:
        req = urllib.request.Request(
            f"https://api.telegram.org/bot{TG_TOKEN}/sendMessage",
            data=json.dumps({"chat_id": TG_CHAT, "text": text}).encode(),
            headers={"Content-Type": "application/json"},
        )
        urllib.request.urlopen(req, timeout=20)
        print("\nsent to telegram", file=sys.stderr)
    else:
        print("\nno telegram config; printed only", file=sys.stderr)


if __name__ == "__main__":
    main()
