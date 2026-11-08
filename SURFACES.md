# AsterLab — Surface Contract

- 状态：**Native UI Direction Freeze / 2026-11-08**
- 本文定义页面职责、区域、状态和验收；不重复 [CONTEXT.md](CONTEXT.md) 的科学 Schema。
- 任何迁移页面必须先有本文件中的状态清单和 Web 基线截图。

## Surface map

| Surface | 用户问题 | 主对象 | 主要动作 | 不负责 |
|---|---|---|---|---|
| Shell / context | 我在哪个项目、设计或 Run？ | Project / Design / Run context | 切换上下文、返回、备份 | 科学设计或执行 |
| 实验设计 | 这次要比较什么、怎么做？ | Design / DesignVersion | 编辑草稿、保存版本、开始运行 | 实际读数、仪器操作 |
| Protocol | 这个方法怎么复用？ | Protocol / ProtocolVersion / BlockVersion | 查看/编辑步骤块、参数和版本 | 组别、独立 n、实际执行 |
| 实验运行 | 当前做哪一步、记什么、结果在哪？ | Run / PlannedTask | 检查、确认、测量、回看 | 改写冻结设计、伪造实际值 |
| 资料与工具 | 用哪个方法或资源完成一次动作？ | Material / LibraryTable / Formula / Instrument | 查看、计算、引用、应用 | 复制科学设计、伪造执行事实 |
| 项目 | 这些设计和运行属于谁？ | Project | 浏览、进入、归档 | 代替设计或执行 |

## Global shell

**区域**

- 顶栏：产品标记、项目 → 设计 → 当前 Run、状态、开发入口。
- 左侧 rail：实验设计 / Protocol / 实验运行 / 资料与工具，固定四项。
- 内容区：当前 Surface。
- 全局选择器：项目、设计、Run；不把选择器做成第五个标签。

**约束**

- 当前上下文始终可见；切换 Run 不丢失未保存的输入，先提示。
- 页面标题与上下文分离：标题是任务名，副标题才是版本/状态。
- 返回路径可预测；不出现“浏览器后退才回到上一层”的隐藏导航。
- 所有写操作有明确结果：保存、失败、冲突、未保存离开。

**验收**

- 1440、1920、390 宽度下无横向溢出。
- 键盘可从 rail 进入内容区，焦点环可见。
- 当前项目/设计/Run 可通过文本识别，不只靠颜色。

## 实验设计

**用户情境**：研究者只有一个模糊问题，需要把它变成组别、检测、时间和 Protocol 的组合；设计必须允许不完整保存。

**区域**

1. 设计入口：一句话描述、复制已有设计、导入记录、手动添加。
2. 四步草稿：问题与比较 → 组别与 n → 检测与时间 → 审查与保存。
3. 设计画布/摘要：组别 × 检测矩阵、样本关系摘要、未确认项。
4. Inspector：选中组别、检测或时间点的完整参数。

**状态**

- 空白草稿：显示入口与已有案例，不显示空矩阵。
- 草稿不完整：允许保存；未确认项逐条可见。
- 已保存版本：只读快照；修改产生新 revision。
- 被 Run 引用：提示冻结版本，不用当前设计覆盖 Run。
- 冲突/导入失败：保留原文和来源，不静默丢弃。

**主操作**：保存设计版本（或开始运行）；一次只出现一个主按钮。

**不负责**：实际孔位、实际读数、仪器设置。

**验收**

- 切换四步不丢输入。
- 无检测时不显示空矩阵；有检测后才显示覆盖矩阵。
- 独立 n、技术重复、容器位数、样本数分别显示。
- 手机上关系图默认收起，桌面默认展开；不因展开挤掉主操作。

## Protocol

**用户情境**：研究者要复用一套方法，不想在每次设计里重抄说明书。

**区域**

- Protocol 列表与版本。
- 根步骤块、子步骤块树、参数槽位、来源和版本。
- 与 Assay 的引用关系；被哪些设计/Run 使用。

**状态**

- 无 Protocol：空状态 + 创建入口。
- 草稿/已发布版本：版本号与来源可见。
- 被引用：删除/修改必须明确影响范围，不静默更新旧 Run。
- 参数未解析：标“待确认”，不填入默认值。

**主操作**：保存 ProtocolVersion 或复用已有版本。

**不负责**：组别、独立 n、实际执行。

## 实验运行

**用户情境**：执行者站在实验台前，需要知道当前一步、缺什么、记录什么，并在完成后回看证据。

**区域**

1. Run 标题：名称、设计版本、Protocol 版本、时间点、任务进度、状态。
2. 模式：执行安排 / 结果。
3. Schedule Strip：由结构化任务决定时间点、阶段或步骤进度；不硬编码 D1/D3/D7。
4. 当前任务：步骤标题、说明、参数、作用域、下一步。
5. 记录区（左侧主区）：本步结构化字段、检查、偏差、操作人（按需）、完成本步。
6. 就绪栏（右侧）：材料/试剂、仪器/程序、检查/接受；紧凑行，不三个大卡片。
7. 结果：事件、测量、读数、检查、未记录读数的已完成任务、批量录入。

**Schedule Strip 规则**

- 有多个时间点：显示时间点。
- Protocol 有明确命名阶段：显示阶段。
- 普通线性 Protocol：显示步骤进度。
- 同时存在重复层级时只显示一层主进度；详细步骤放在任务区。
- 不从标题关键词猜测阶段；由任务/步骤结构化字段决定。
- 没有时间点时不画空时间轴。

**状态**

| 状态 | 显示 | 动作 |
|---|---|---|
| 未开始 | 中性文字/空 | 可查看步骤；不可记录未来事件 |
| 当前 | 紫色标识 + 文本“当前” | 显示记录区与主操作 |
| 已完成 | 绿色中性完成标记 | 可回看；不重复追加完成事件 |
| 阻断 | 红色/警示 + 原因 | 必须处理阻断检查 |
| 待确认 | 琥珀文字 + 具体缺项 | 允许保留草稿；不伪造值 |
| 无已解析参数 | 文字明示 | 只记录本次实际值或偏差，不填默认值 |
| 无数据 | 句子说明缺什么 | 给出唯一下一步 |
| 错误 | 错误摘要 + 重试/返回 | 不丢已输入内容 |

**主操作**：完成本步。测量保存是次级主操作；两者不合并为一个不可分辨的动作。

**不负责**：改写冻结 Design/Protocol；把计划参数写成实际测量；自动推断样本/孔位/n。

**验收**

- 同一个冻结 Run 在 Web 与 Rune 中显示同一任务、同一状态、同一事件。
- 保存测量后刷新/重启仍可回看。
- 完成事件和测量事件分别存在。
- 当前任务判定只有一个来源（Rust 应用层）。
- 记录只在 Run 中出现；`?page=records` 不再是产品页面。

## 资料与工具

**用户情境**：执行前或执行中需要计算、找材料、看资料表、打开仪器方法。

**区域**

- 快速入口：稀释、总量、单位换算、板图。
- 资源卡：材料、资料表、公式、仪器。
- 计算抽屉/面板：输入 → 预览 → 应用/插入；不静默写入 Run。
- 仪器/采集入口：能力说明与外部程序，不伪造预约或读数。

**状态**

- 资源不存在：空状态 + 创建/导入。
- 计算结果未应用：预览状态明确。
- 应用到 Run：必须选择明确目标；否则只复制结果。
- 仪器不可用：显示原因，不显示假成功。

**主操作**：应用一次工具结果，或打开一个资源。

## 项目

**用户情境**：整理多个实验设计、Run 和资料。

**区域**：项目摘要、设计数、Run 数、最近活动；进入设计与运行。

**状态**：空项目、只有设计、只有 Run、归档/删除影响提示。

## Retired surfaces

- **实验记录（独立页）**：删除。记录属于 Run；`?page=records` 回退到实验运行。
- **旧 `/workspace` 交互页**：不再作为产品入口；保留仅为历史/测试时须标注 legacy。
- **第二套完整编辑 UI**：迁移期间只保留 Web 一套可写；Rune 通过验收后才切换默认。

## State inventory for baseline

Baseline must capture at least:

1. Design empty / draft incomplete / design with assays.
2. Protocol list / protocol detail with unresolved parameters.
3. Run execution: current step without measurement schema.
4. Run execution: current step with measurement schema (CCK-8).
5. Run execution: completed step / blocked check.
6. Run results: measurement present / no measurement.
7. Tools: empty resource state / calculator open.
8. Mobile 390: Run current task and design relation summary.

Each capture records: route, fixture/Run id, viewport, theme, state, expected primary action, and known gap. Store under `docs/design/baseline/`.

## Acceptance for this contract

- Every surface has one primary action and an explicit unknown state.
- No surface duplicates another surface's write responsibility.
- Run's measurement and completion are separate events.
- Schedule Strip is derived, not hardcoded.
- Web baseline and Rune slice can be compared using the same frozen Run.
