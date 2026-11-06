# W6 Proposal Review 交付说明

- 日期：2026-11-06
- 分支：`feature/slint-native-spike`
- 范围：Selection Context、Proposal Preview、结构化 Diff、接受/拒绝、CAS 冲突

## 启动

```powershell
./run-local.ps1
```

然后打开 `http://127.0.0.1:8832/view`。当前开发服务也可能运行在 `http://127.0.0.1:19000/view`。

## 用户流程

1. 打开一个 View。
2. 点击 Step Detail 中的卡片选中组件。
3. Proposal 面板选择确定性提案：
   - 表格移动到最上面
   - 追加说明
   - 添加 Markdown 卡片
   - 删除此卡片
4. 服务端 `POST /api/views/{id}/propose` 只验证并返回 Diff，不写入。
5. 用户接受后，才通过 `POST /api/views/{id}/revise` 写入。
6. 若 revision 已变化，提案标记过期，必须重新生成。

## 只读接口

```text
GET /api/views/{view_id}
GET /api/views/{view_id}/selection/{instance_id}
GET /api/views/{view_id}/steps/{instance_id}
```

`selection` 返回：

```json
{
  "view_id": "...",
  "document_id": "...",
  "revision": 35,
  "instance_id": "concentration_table",
  "renderer_key": "layout.table@1",
  "component": {},
  "step_detail": {
    "status": "resolved | not_applicable | error",
    "detail": null,
    "error": null
  }
}
```

## Proposal

```text
POST /api/views/{view_id}/propose
Content-Type: application/json

{ "changeset": { ...ViewChangeSet... } }
```

返回：

```json
{
  "valid": true,
  "document_id": "...",
  "expected_revision": 35,
  "new_revision": 36,
  "changed_instances": ["concentration_table"],
  "affected_bindings": [],
  "requires_confirmation": true,
  "operation_changes": [],
  "changes": []
}
```

- `new_revision` 是预览目标号，不代表已保存。
- `operation_changes` 是 `ViewPatch` 级别的结构 Diff。
- 写入仍使用现有 `view.revise` 和 CAS。

## 确定性 Agent 演示

```bash
node scripts/agent-proposal-demo.cjs <view-id>
node scripts/agent-proposal-demo.cjs <view-id> --accept
node scripts/w6-proposal-tests.cjs <view-id>
```

脚本走公开 HTTP API：

```text
Selection Context
  → Proposal Preview
  → Accept / no-write
  → Revision conflict check
```

## 边界

- 当前 UI 的提案按钮是确定性操作，不是 LLM 生成。
- 外部 Agent 可以使用同一 Selection + Proposal + revise API，不应直接写 SQLite。
- `WorkflowEdge` 仍是视觉关系；连接点不是 `ComponentPort`，不表示样本转移或 Protocol 顺序。
- 科学修改（新增洗涤步骤、改 Protocol 参数）仍需正式 Scientific Command；本交付只提供 View 层提案审阅。

## 验证

```bash
node tests/w5-layout.test.cjs
cargo test -p asterlab-core
cargo test
cargo clippy -p asterlab-core --all-targets
```

浏览器证据：

```text
target/workspace-final.png
target/w5-branch.png
```
