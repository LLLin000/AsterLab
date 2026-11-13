#!/usr/bin/env python3
"""Trigger a lane retrospective when the VPS lane drains.

Fires once per drain: VPS Inbox+Ready+Agent+Review all empty AND at least one
new Merged card since the last retrospective watermark. Creates an (Audit)
card with a fixed Luna profile; the agent writes the report, opens a PR, and
the feeder's approval push notifies the human.

Watermark keys in kandev_meta (never in the repo):
  retro.vps.last_run_at   ISO timestamp of last trigger
  retro.vps.last_task_ids JSON list of task ids covered last time

Scope of each run: VPS tasks whose Merged transition happened after the
watermark. Never re-covers old cards.
"""

import json
import os
import sqlite3
import sys
import urllib.request

BASE = os.environ.get("KANDEV_URL", "http://127.0.0.1:38429").rstrip("/")
TOKEN = os.environ.get("KANDEV_API_TOKEN", "")
WORKSPACE = os.environ.get("KANDEV_WORKSPACE_ID", "b67b6785-db21-46de-bd68-c0c437823798")
WORKFLOW = os.environ.get("KANDEV_VPS_WORKFLOW_ID", "d1a08c74-42ab-4e8d-aee3-a67b459537f8")
DB = os.environ.get("KANDEV_DB", "/var/lib/kandev/data/kandev.db")
LUNA = "e930e6a0-835f-4e84-834e-09dea519280c"
MERGED = "e22063df-1124-4836-8e5c-b0c0e387be68"

RETRO_PROMPT = """你是 AsterLab 的阶段回顾 agent。本次回顾范围只包括以下任务（不要写范围外的卡）：

{task_list}

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
1. 写到 docs/release/retrospective-<YYYYMMDD>.md
2. 提交到分支 agent/RETRO-<YYYYMMDD> 并开 PR（标题带 RETRO-<YYYYMMDD>），PR 描述贴报告全文
3. 在任务评论写 REVIEW-READY
不要合并 PR，等人工看完报告再决定。
"""


def api(method, path, body=None):
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
    except Exception as exc:  # noqa: BLE001
        return 0, {"error": str(exc)}


def main():
    if not TOKEN:
        print("KANDEV_API_TOKEN required", file=sys.stderr)
        return 2
    con = sqlite3.connect(f"file:{DB}?mode=ro", uri=True)
    con.row_factory = sqlite3.Row
    # 1. lane drained?
    open_n = con.execute(
        """select count(*) from tasks where archived_at is null and workflow_id=?
           and workflow_step_id != ?""",
        (WORKFLOW, MERGED),
    ).fetchone()[0]
    if open_n > 0:
        print(f"lane not drained ({open_n} open), skip")
        return 0
    # 2. new merges since watermark?
    wm = con.execute("select value from kandev_meta where key='retro.vps.last_run_at'").fetchone()
    since = wm[0] if wm else "1970-01-01T00:00:00Z"
    rows = con.execute(
        """select t.id,t.title,max(tr.occurred_at) m from task_step_transitions tr
            join tasks t on t.id=tr.task_id
           where t.workflow_id=? and tr.to_workflow_step_id=? and tr.occurred_at > ?
           group by t.id order by m""",
        (WORKFLOW, MERGED, since),
    ).fetchall()
    con.close()
    if not rows:
        print("no new merges since watermark, skip")
        return 0
    task_list = "\n".join(f"- {r['title']} (merged {r['m']})" for r in rows)
    task_ids = [r["id"] for r in rows]
    print(f"triggering retrospective for {len(rows)} cards since {since}")

    import datetime

    stamp = datetime.datetime.now().strftime("%Y%m%d")
    title = f"RETRO-{stamp} (Audit) VPS lane 阶段回顾"
    desc = RETRO_PROMPT.format(task_list=task_list, since=since)
    status, data = api(
        "POST",
        "/api/v1/tasks",
        {
            "workspace_id": WORKSPACE,
            "workflow_id": WORKFLOW,
            "title": title,
            "description": desc,
            "labels": ["retro", "roadmap", "lane:vps"],
            "priority": "high",
            "agent_profile_id": LUNA,
        },
    )
    if status not in (200, 201):
        print(f"create card failed: {status} {data}", file=sys.stderr)
        return 1
    tid = data.get("id", "?")
    print(f"created {tid}")
    # 3. advance watermark (only after successful creation)
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
    print(f"watermark advanced to {now}, {len(task_ids)} tasks")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
