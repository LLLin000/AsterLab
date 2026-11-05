# Gate 0 仓库追踪(2026-11-06)

- 基线:`b3e99a0`;检查时工作树另有任务包目录(未跟踪),无其他未提交业务文件。
- 审查方式:源码阅读 + 本地 `cargo test` / `cargo check`;没有执行浏览器视觉走查。

## 代码—研究—测试追踪

| 研究/ADR | 代码位置 | 本地证据 | 状态 |
|---|---|---|---|
| 原子操作/组合/方法模板 | `operation_module.rs`、`protocol_block.rs`、`semantic_composition.rs`、`src/cli.rs::block_preview` | Core tests + `cck8_multistep...` | 保留 |
| OperationUse 真实作用域 | `src/cli.rs::operation_use_exists`、`project_view_records` | `view_document...` bad-scope 反例 | Gate 1 修复 |
| PlannedContainer/Profile | `labware.rs`、`DesignChanges`、`container_plan`、Run `labware_profiles` | `labware` tests、`design_container_plan...` | Gate 1/2 |
| Component capability | `ComponentRegistry::implemented`、`component.list` | `view_document...` capability assertion | Gate 2 |
| WellPlate → ToolAction → Table | `component_data.rs`、`view.data_preview`、`scientific_facts.rs` | `view.data_preview` 成功/缺值 | Gate 3 部分 |
| View 持久化/Patch | `component_framework.rs`、`view.*` | `view_document...` | 保留 |
| 旧兼容 | 旧 `layout.table`/note/dilution、旧 View JSON | Core/integration | 保留 |

## 当前测试

```text
cargo test -p asterlab-core  155 passed
cargo test                   3 core integration + 6 operation + 5 catalog integration
cargo clippy -p asterlab-core --all-targets  passed
cargo fmt --all -- --check  only pre-existing design_commands.rs diff
```

## 未闭合 / 阻断

- Gate 3 已支持 `calculator_instance_id` + DataBinding 解析计算器输入;显式 volume 仍作为回退。
- 已用 Design v1→v2 验证新 View 重算 A1→A2,旧 View 仍为 A1。
- `view.data_preview` 只支持一个绑定到 `PlannedContainer` 的 WellPlate 实例;尚未验证多板/多实例选择。
- Gate 4 API/HTML 数据预览入口已加入;已用本地浏览器打开 seeded View,选中 operation 后点击数据预览,显示 1 孔 × 10 µL × 1.1 = 11;截图 `target/view-ui-data-preview-fixed.png`。
- `OperationUse.scope` 已按 Design Assay/PlannedOutput 与 Protocol 的关联验证;仍需要更多多 Assay 回归。
- Profile 兼容性/来源仍是 research_draft;不能声称专家审核。
