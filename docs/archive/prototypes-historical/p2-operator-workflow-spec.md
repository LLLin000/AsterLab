# P2 · Operator Workflow Prototype

状态：下一轮规格。`b556a9d` 的 P1 分支保留为 Slint 技术 Spike，不按最终产品 UI 合并。

`DESIGN.md` 继续规定“界面怎么画”；本文件规定“用户如何完成实验”。两者分开评审。

## 实施进度（2026-11-05）

- P0 上下文：已完成。显式 Run；孔板按 Run + Assay 过滤；切换检测不残留无关板。
- P1 读取模型：部分完成。桌面端已读取 `design_version.snapshot`、`sample_allocations`、
  `planned_outputs`、`protocol_manifest`、`material_manifest`、`execution_event`、组别 n 与技术重复。
  当前 fixture 的 Protocol/材料清单为空，因此执行步骤仍无法生成；UI 明确显示缺失，不伪造步骤。
- P2 工作区：已完成初版。设计 / 准备 / 执行三种模式共用同一外壳；设计模式显示设计摘要、
  扁平组别/检测表、样本分配、证据/冲突和数据完整性检查；准备/执行读取冻结快照。
- P3 操作可视化、P4 实际记录闭环、P5 性能复测：未实现。

## 1. P1 的真实定位

P1 验证了这些技术能力：

- Slint + Winit + FemtoVG/软件渲染可启动；
- 显式 SQLite 文件只读加载，不创建/迁移；
- 组别 × 检测关系画布、上下文检查器；
- 96 孔缩略 → 俯视 → 选孔；
- 中文排版、辅助功能 Invoke、截图与性能测量。

P1 没有验证产品主线。它把 **Design Overview** 当成了整个应用：

| 已有对象 | P1 中的位置 | 缺失的环节 |
|---|---|---|
| Design | 画布与检查器主体 | 设计版本冻结 |
| Protocol | 几乎未进入界面 | 当前步骤、操作参数 |
| Run | 侧栏一行/检查器一行 | Run 任务、批次、时间点 |
| Tools | 无 | 铺板、计算、设备入口 |
| Record | 只读孔位属性 | 实际事件、偏差、完成状态 |

因此 P1 可以作为“设计总览”，不能作为“实验操作台”。

> 工具/仪器的正确形态不是独立“工具中心”，而是 Protocol 步骤中的可插拔模块：在步骤列表的
> “+”处按上下文插入，不弹出“是否使用某仪器”的全局询问。模块目录、参数契约与插入规则见
> [`instrument-step-modules.md`](./instrument-step-modules.md)。

## 2. 目标对象模型

```
Project
└─ Design (revision, frozen when a Run is created)
   └─ Run (batch / schedule)
      └─ RunTask (what to do next, at a timepoint)
         └─ OperationStepInstance (this occurrence of a protocol step)
            ├─ Target (plate / well / sample / container)
            ├─ Parameters (amount, time, temperature, instrument)
            └─ Record (actual event, deviation, completion)
```

关键区分：

- **Design** 回答“研究怎么安排”。
- **Protocol** 回答“步骤怎么做”。
- **Run** 回答“这一次实际执行什么”。
- **RunTask / StepInstance** 回答“现在做哪一步、对哪个目标”。
- **Record** 回答“实际发生了什么”。

UI 不得从 Assay 的排列顺序推导步骤顺序；顺序只能来自冻结的 Protocol 或明确的计划依赖。

## 3. 三种工作模式

三种模式共用外壳（左导航、顶栏、右侧上下文），但中央主对象不同。

| 模式 | 左侧 | 中央主对象 | 右侧只回答 |
|---|---|---|---|
| A 设计 | 项目 / 设计修订 | 组别、检测、时间点、样本关系 | 当前设计对象的参数与来源 |
| B 准备 | 项目 / Run / 任务目录 | 材料、样本、计划板位、待确认条件 | 本任务的准备要求与工具入口 |
| C 执行 | 项目 / Run / 当前任务 | 当前步骤 + 目标容器/孔板 | 本步骤参数、目标、记录入口 |

模式切换不改变数据关系；只改变当前主对象和操作集合。

中央不应永远固定为“组别 × 检测关系图”：

- 设计模式：关系图是主角；
- 准备模式：准备清单和计划板位是主角；
- 执行模式：当前步骤与目标孔位是主角；关系图折叠为上下文。

## 4. 桌面端 Read Model

Slint 不解析完整 DesignVersion/Protocol JSON，也不自行推导关系。Rust 提供：

### RunTaskVM

| 字段 | 含义 |
|---|---|
| `task_id` | 任务实例 |
| `run_id` | 所属 Run |
| `design_version_id` | Run 创建时冻结的设计版本 |
| `protocol_version_id` | 冻结的 Protocol 版本 |
| `timepoint` | D1 / D3 / D7 或未记录 |
| `title` | 当前任务名称 |
| `state` | planned / active / blocked / done / cancelled |
| `blocking_reasons` | 缺失参数、未确认材料、未解决冲突 |
| `step_ids` | 本任务步骤实例顺序（来自冻结 Protocol） |

### OperationStepVM

| 字段 | 含义 |
|---|---|
| `step_instance_id` | 步骤实例；跨时间点唯一 |
| `step_path` | Protocol 中的步骤路径 |
| `title` | 操作名称 |
| `kind` | 核对 / 加样 / 孵育 / 测量 / 记录 |
| `parameters` | 结构化参数及来源；未知项为 `待确认` |
| `targets` | 孔位/容器/样本引用 |
| `preview_asset` | 操作示意资源标识 |
| `record_state` | 未开始 / 已记录 / 有偏差 |

### PlateContextVM

| 字段 | 含义 |
|---|---|
| `plate_id` / `run_id` | 所属 Run |
| `assay_id` | 关联检测；无关联时为空 |
| `rows` / `columns` | 真实板尺寸 |
| `wells` | 坐标、已记录映射、当前步骤目标标记 |
| `target_wells` | 本步骤明确关联的孔位；不由 UI 推断 |
| `legend` | 映射/目标/选中三种语义 |

所有只读模型由 Rust 计算；Slint 只渲染。后台读取/写入在工作线程完成后送回 UI 事件循环。

## 5. 冻结快照与血缘

Run 创建时必须绑定：

- `design_version_id`；
- `protocol_version_id`；
- 材料/样本分配快照。

执行模式读取冻结版本，不能读取当前 Design。若冻结版本缺失或无法解析：

- 显示“版本待确认”；
- 禁止把当前 Design 的组别/时间点套到旧 Run 上；
- 禁止写入执行事件，直到来源明确。

P1 的 `data.rs` 只读取当前 Design 记录；这是 P2 必须补齐的第一项数据能力。

## 6. 记录与动画的边界

- 操作动画只解释“将要做什么”，不表示已完成。
- 完成状态只能由明确的执行事件产生，并绑定 `run_id`、`step_instance_id`、目标、时间和操作者。
- D1 的记录不得改变 D3 的完成状态；每个时间点使用独立步骤实例。
- 未知参数显示“待确认”；不得填入常用加样量、孵育时间或孔位。
- 预览结束不自动推进任务；需要用户明确确认或记录。

## 7. P0 上下文修复（已落地）

在 `b556a9d` 之后已修复：

1. 显式 Run 选择：导航增加“实验运行”，顶栏显示 Run；选择 Run 保留当前检测上下文。
2. 检测与孔板同步：孔板列表只包含当前 Run 且 `assay_id` 匹配的板；切换检测不再显示无关板。
3. 默认选择：优先选择有孔板的检测（L929 下为 CCK-8），避免“活死染色 + CCK-8 板”的错配。
4. 无孔板状态：显示“该检测暂无培养板记录”，而不是保留上一块板。
5. 上下文校验：孔位选择要求当前板属于当前 Run/Assay；不满足则不响应。

这些是正确性修复，不改变 P1 的整体定位。

## 8. 下一轮阶段与验收

| 阶段 | 工作 | 验收 |
|---|---|---|
| P0 | Run 选择、检测/板同步、空状态 | 切换检测不出现无关板；Run 选择不丢失上下文 |
| P1 | 读取 `design_version`、Protocol、执行事件，产出 RunTaskVM/OperationStepVM/PlateContextVM | 能显示冻结步骤、参数来源、未解决条件 |
| P2 | 设计 / 准备 / 执行三种中央视图 | 首次打开即知道当前模式与下一步 |
| P3 | 当前步骤的操作示意、目标孔位高亮 | 图示与文本使用同一份结构化参数 |
| P4 | 通过共享 Rust 服务写入执行事件 | 记录绑定 Run/步骤实例/目标；D1 不影响 D3 |
| P5 | 性能复测与渲染器决策 | FemtoVG 空闲重绘定位；按预算选择默认后端 |

不建议在 P1 上继续堆页面；先完成 P1 数据模型与 P2 模式切换。

## 9. 三个真实验收任务

### 任务 1：检查 L929 实验设计

必须能：

- 识别组别、各检测时间点、独立 n 与样本关系；
- 定位待确认项（如划痕组别未明确、时间点关系待确认）；
- 区分 Design、Protocol、Run 三个层次。

### 任务 2：准备一次 CCK-8 Run

必须能：

- 找到本次需要的材料、已确认的 Protocol 参数、目标容器；
- 看到缺失条件并知道先补什么；
- 不把“演示板位”当成已确认铺板。

### 任务 3：按步骤执行并记录 D1

必须能：

- 选择 D1 的当前步骤；
- 查看对应操作示意与目标孔位；
- 记录实际事件；
- D1 的记录不错误标记 D3。

第二、第三项未完成前，即使孔板动画完整，也不能视为面向实验人员的主工作台完成。

## 10. 性能门槛

P1 实测（单机，提交记录）：

- FemtoVG：窗口显示 384ms；私有内存 87–102MB；稳定空闲约 4.7% 单核；最小化 0%。
- 软件渲染：63ms；私有内存 6.9MB；空闲 0%。

正式默认后端不应因“有 GPU”自动选 FemtoVG。P5 必须先定位空闲重绘来源，再按空闲 CPU、内存和复杂场景帧率共同决定。

## 11. 仪器与操作模块（待实现）

完整契约与目录见 [`instrument-step-modules.md`](./instrument-step-modules.md)。
后端 schema、前端编辑器与分阶段实现见
[`2026-11-05-step-editor-implementation-plan.md`](./2026-11-05-step-editor-implementation-plan.md)。
下一轮优先切片（只读实验地图 + 执行台）见
[`2026-11-05-experiment-map-and-agent-contract.md`](./2026-11-05-experiment-map-and-agent-contract.md)。

P3/P4 的前置条件：

- 步骤列表支持在任意位置插入/删除仪器模块；插入不改动组别、n、检测和样本分配。
- 插入菜单按当前步骤上下文过滤；不显示“是否使用某仪器”的全局弹窗。
- 每个模块带参数、单位、约束、未知状态和 Run 记录字段；型号未知时不套用具体规格。
- 仪器模块的原始观测与派生结果分开；读板/成像/流式等不自动生成科学结论。
- 同一模块两次使用是两个独立 `BlockUse`；修改一处参数不影响另一处。
