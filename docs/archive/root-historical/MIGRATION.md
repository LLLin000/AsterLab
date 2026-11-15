# AsterLab — Native UI Migration Contract

- 状态：**Native UI Direction Freeze / 2026-11-08**
- 批准：设计定义阶段、Rune 技术可行性 Spike。
- 未批准：全应用迁移。
- 本文定义迁移的工程边界、验收门槛和停止条件。

## Decision

- **原生范围**：先做 Run 单屏完整纵向切片；不是静态原型。
- **Web**：迁移期间保持可写；Rune 达到全应用功能对等并通过真实科研任务验收后，Web 收敛为只读/Agent 参考。
- **视觉**：AsterLab 暖灰 + 紫色，紧凑、细线、少卡片；不引入 Rune/OpenCode 主题。
- **记录**：测量与完成分离；非测量步骤轻量确认。
- **Rune**：验证驱动的技术选择，不是已确定成功的终点。

## Architecture

目标分层，不在当前阶段一次性重构全部 crate：

```text
asterlab-core    领域对象、SQLite、校验、版本、事务、命令
asterlab-app     用例命令、view model、任务/结果投影
asterlab-theme   语义 token → Web CSS / Rune theme
asterlab-kit     AsterLab 组件（包装 rune / rune-kit）
asterlab-ui      Rune 页面与导航
labworkbench     CLI / HTTP / 原生 shell
```

规则：

1. **同一命令层**：Web HTTP handler 与 Rune 原生调用同一 Rust 服务函数，不复制业务逻辑。
2. **UI 只发意图**：Rune 按钮不自行判断阻断、完成或事务。
3. **同一读契约**：Run 与 Results 使用同一 `RunWorkspace` 投影；不能各拼一套 `measurements[]` / task status。
4. **数据库兼容**：迁移不得破坏现有 SQLite；不改写冻结引用。
5. **单写路径**：同一 Run 同一任务不得被 Web 与 Rune 并发写入；命令层加锁/事务。
6. **Rune pin**：固定具体 Git revision；不使用浮动 `main`。升级另开验证任务。

## Rune gate 1 — Technical feasibility Spike

**目的**：在正式迁移前证明关键桌面体验可用。

必须实测：

| # | 项目 | 通过条件 |
|---|---|---|
| 1 | Windows 中文 IME | 候选词、组合输入、提交/取消、失焦、复制粘贴正常 |
| 2 | CJK 字体 | 中文标签、长标题、标点、数字/单位混排不截断、不糊 |
| 3 | 表格输入 | 100+ 行滚动；单元格编辑；Tab/方向键；失焦提交；错误态 |
| 4 | 高 DPI | 125% / 150% 清晰；窗口缩放不破布局 |
| 5 | 键盘焦点 | Tab 顺序、Enter/Escape、焦点环、模态/抽屉 |
| 6 | Headless | 截 PNG、读元素树、脚本输入；可做回归 |
| 7 | 打包 | Windows release 可启动；安装/单文件路径可接受；签名方案明确 |
| 8 | 数据接入 | 能读一个冻结 Run；不改现有数据库 |

**失败处理**：任一关键项失败 → 停止 Rune 扩张，保留 Web Beta，记录原因。不得用“后续再修”跳过 IME/表格。

**报告必须包含**：固定 revision、构建环境、实际命令、截图/日志、通过/失败/未测、人工步骤。

## Rune gate 2 — Run vertical slice

**目的**：证明一个真实科研任务可以闭环。

流程：

```text
冻结 Run
→ 显示 Schedule Strip 与当前任务
→ 查看材料/仪器/检查
→ 通过/阻断检查
→ 完成本步
→ 录入结构化读数
→ 查看 Results
→ 重启应用
→ 回看同一 Run 的同一状态与读数
```

通过条件：

- 完成与测量事件分别存在。
- 阻断检查无法被 UI 绕过。
- 刷新/重启后任务状态、读数、结果一致。
- Web 与 Rune 使用同一冻结 Run 时显示一致；允许视觉差异，不允许事实差异。
- 数据库兼容；不产生第二套记录入口。
- 通过 Impeccable critique/audit；无 P0/P1 未解决项。

**失败处理**：不迁移其他页面；修复后重跑同一 fixture。

## Phases

### Phase 0 — Definition and baseline（当前）

- 六份契约：PRODUCT / DESIGN / SURFACES / COMPONENTS / RECORDING / MIGRATION。
- Web Run 设计 critique、技术 audit、首屏/交互截图、状态清单。
- 不写 Rune 页面。

### Phase 1 — Rune technical spike

- 固定 revision。
- 最小窗口：中文、表格、输入、焦点、高 DPI、headless、打包。
- 输出技术评估报告。
- 未通过即停止。

### Phase 2 — Run slice

- 只实现 Run 一屏。
- 使用冻结 Run fixture。
- 同一数据与 Web 对照。
- 通过 gate 2 后，才讨论其他页面。

### Phase 3 — Surface-by-surface

顺序建议：Run → 资料与工具 → Protocol → 实验设计。

每屏必须：

1. 设计契约与状态清单。
2. Web 基线截图。
3. Impeccable critique + audit。
4. Rune 实现与对照。
5. 真实/仿真任务走查。
6. 数据持久化重启验证。

### Phase 4 — Cutover

- 全应用功能对等 + 验收通过。
- Rune 成为默认入口。
- Web 转为只读/Agent 参考。
- 删除重复编辑逻辑；不保留第二套写路径。

## Parity matrix

Before migrating each surface, fill:

| 项目 | Web | Rune | 证据 |
|---|---|---|---|
| 页面区域 |  |  | 截图 |
| 主要动作 |  |  | 操作录像/步骤 |
| 空/未知/错误态 |  |  | 状态清单 |
| 键盘/焦点 |  |  | 测试记录 |
| 数据写入 |  |  | 同 fixture 结果 |
| 重启回看 |  |  | 同 Run id |
| 性能 |  |  | 行数/延迟 |
| 可访问性 |  |  | 角色/名称/焦点 |

事实差异 = 不通过；视觉差异需要设计确认。

## What not to do

- 不提前重构所有 Rust crate。
- 不一次性重写四个页面。
- 不借迁移扩充科学功能。
- 不让 Rune 组件直接判定实验事实。
- 不为了“看起来原生”牺牲记录能力。
- 不在未通过 IME/表格门槛时宣布 Rune 可用。
- 不长期维护两份完整可写 UI。

## Definition of done for this phase

1. 六份设计契约已写并互相链接。
2. 现有 Web Run 有 critique、audit、截图、状态清单。
3. Rune 技术评估报告有固定 revision 与真实结果。
4. 未通过技术门槛时明确停止；通过则只批准 Run 切片。
5. 不声称全应用迁移已获批准。
