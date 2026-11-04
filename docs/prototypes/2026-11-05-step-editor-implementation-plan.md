# Protocol 步骤编辑器：前后端实现方案

- 日期：2026-11-05
- 状态：实现方案，未写代码、未改数据库、未启动应用。
- 基线：仓库 HEAD `8e14130`。
- 关联：[协议知识与实验步骤](../research/2026-11-05-protocol-knowledge-and-related-projects.md)、[仪器步骤模块](instrument-step-modules.md)、[P2 工作流规格](p2-operator-workflow-spec.md)。

## 0. 结论

> 本方案已被 [实验地图与 Agent 契约](2026-11-05-experiment-map-and-agent-contract.md) 重新排序：
> 先做只读实验地图和执行台，再把 `StepMeta` 作为 Protocol 的语义补充。不要让字段扩展抢走
> “看懂实验、按步骤执行”的优先级。

不要新建“工具中心”，也不要重写工作流引擎。最小可行路径是：

1. **先做只读实验地图**：让 Agent 和研究者看懂研究问题、组别、检测、时间、样本关系和待确认项。
2. **再做执行台**：让一个真实冻结 Protocol 的步骤、参数、目标位置和记录入口可见。
3. 然后才给现有 `protocol_block_version` 的步骤增加可选 `meta`，记录目的、阶段、对照、判定、失败模式、读数边界、样本约束、证据和可选仪器。
4. 最后做多步骤编辑器和 Protocol 组合器；仪器模块只是带 `meta.instrument` 的步骤块。

先做地图和执行闭环，再做步骤语义和编辑器。不要先做“AI 生成整份 Protocol”。

## 1. 当前实现事实

### 1.1 后端

| 事实 | 位置 |
|---|---|
| 通用 `records` 表：`kind, parent_id, revision, data_json`；没有独立迁移框架 | `src/main.rs:650-675` |
| 步骤类型只有 `text` 与 `block_use` | `src/cli.rs:520-540` |
| 步骤块版本数据：`summary`、`parameter_slots`、`steps` | `src/cli.rs:580-600` |
| 参数槽：`key/label/value_type/unit/required` | `src/cli.rs:466-495` |
| 步骤块创建/修订是整块替换；版本不可变 | `src/cli.rs:1599-1690` |
| 参数槽校验、步骤引用校验、递归组合校验 | `src/cli.rs:653-840` |
| 展开步骤时只复制 `title/instruction/parameters/visualization` | `src/cli.rs:840-949` |
| Run 创建时把 Protocol 展开结果冻结进 `protocol_manifest` | `src/cli.rs:975-1145` |
| HTTP 只有 `/api/state`、`/api/records`、`/api/records/update`、`/api/protocol-blocks/preview` | `src/main.rs:675-682` |
| CLI schema 手写，`protocol.block.create/preview` 有严格 `additionalProperties:false` | `src/cli.rs:3738-3800` |

### 1.2 前端

`web/index.html` 是单文件、内联样式和脚本：

| 事实 | 位置 |
|---|---|
| Protocol 库显示步骤块卡片和 Protocol 卡片；没有阶段/用途/仪器筛选 | `web/index.html:414-418` |
| 新建步骤块只支持**一个 text 步骤**；组合块提示改用 CLI | `web/index.html:278` |
| 步骤块字段只读 `summary`、`parameter_slots`、单个 `steps[0]` | `web/index.html:302` |
| Protocol 创建只支持选一个根步骤块 + 根参数绑定 | `web/index.html:281` |
| 预览只显示标题/说明/待填参数 | `web/index.html:303` |
| 执行台已有步骤列表、参数、时间点、板图、记录入口 | `web/index.html:469-510` |
| 执行台没有显示 purpose/controls/decision/failure/readout 的固定位置 | `web/index.html:509` |

### 1.3 结论

后端已经能保存和展开多步骤、递归组合；缺口主要是**步骤语义字段**和**前端编辑器**。不需要新表，也不需要改 Run 的冻结模型。

## 2. 数据模型：加一个可选 `meta`

### 2.1 位置

把 `meta` 放在 `ProtocolStep::Text` 上，而不是新建 `step_definition` 表：

```json
{
  "kind": "text",
  "use_id": "read_od450",
  "title": "读取 OD450",
  "instruction": "按已确认的读板设置测量，保留原始读数。",
  "parameter_refs": ["wavelength"],
  "visualization": "measurement",
  "meta": {
    "purpose": "获得可比较的吸光度信号；不是直接得到活细胞数。",
    "stage": "measurement",
    "controls": [
      {
        "label": "无细胞空白",
        "purpose": "扣除试剂/培养基本底",
        "kind": "blank",
        "required": true
      }
    ],
    "decision_points": [
      {
        "question": "读数是否落在方法线性范围？",
        "evidence": "空白、标准曲线、已知稀释",
        "continue_if": "在线性范围内",
        "stop_if": "饱和或低于检测限；需要稀释/重做"
      }
    ],
    "failure_modes": [
      {
        "mode": "气泡",
        "symptom": "孔间读数异常或重复性差",
        "impact": "吸光度偏倚",
        "action": "去除气泡/重新读板；记录偏差"
      }
    ],
    "readout": {
      "raw": "OD450",
      "derived": "相对活力/标准曲线结果",
      "not_implied": "活细胞数、毒性或增殖本身"
    },
    "sample_constraints": ["repeatable_measurement"],
    "evidence": [
      {
        "kind": "vendor_manual",
        "citation": "Dojindo CCK-8 Technical Manual CK04e",
        "url": "https://dojindo.com/products/manual/CK04e.pdf"
      }
    ],
    "instrument": {
      "class": "plate_reader",
      "profile_id": "multiskan_skyhigh",
      "required": true
    }
  }
}
```

理由：

- 语义跟着步骤走；组合块展开时自动带出。
- `block_use` 不需要重复 `meta`；子块自己的 `meta` 就是定义。
- 旧记录没有 `meta`，`serde(default)` 后为空；不改数据库。
- 不需要新增“工具”实体；仪器只是步骤元数据 + 步骤块。
- 不需要 workflow 条件引擎；`decision_points` 只是人和执行台的检查提示。

### 2.2 `StepMeta` 字段

| 字段 | 类型 | 说明 |
|---|---|---|
| `purpose` | string | 为什么做、产生什么变化 |
| `stage` | enum | `inoculation/treatment/wash/stain/lysis/separation/culture/measurement/imaging/sampling/storage/other` |
| `controls` | array | 该步骤依赖的空白/阴性/阳性/标准/未染色等“要求”，不是实际组别 |
| `decision_points` | array | 问题、证据、继续/停止条件 |
| `failure_modes` | array | 现象、原因、影响、处理 |
| `readout` | object | `raw/derived/not_implied` |
| `sample_constraints` | array | `repeatable_measurement/terminal/aliquotable/requires_live_cells/requires_fixation/requires_permeabilization/recoverable` |
| `evidence` | array | 来源类型、引用、URL、检索日期 |
| `instrument` | object? | `class`、`profile_id`、`required`；未知型号时只留 class |

`controls` 与现有设计字段 `control_group_ids` 的分工：

- 步骤 `controls`：这个方法**需要哪类对照**，例如“无细胞空白”。
- 设计 `control_group_ids`：这次实验**实际把哪个组当对照**。
- 两者不能互相替代；软件最多提示“方法要求空白，但设计未绑定空白组”。

### 2.3 参数槽扩展

当前 `ParameterSlot` 只有 key/label/type/unit/required。建议只加两个可选字段：

```json
{
  "key": "wavelength",
  "label": "检测波长",
  "value_type": "integer",
  "unit": "nm",
  "required": true,
  "description": "按染料/底物和仪器确认；不是通用默认值。",
  "source": "vendor_manual"
}
```

- `description`：人类可读的约束/来源说明，不参与计算。
- `source`：枚举 `protocol_fixed/design_variable/lab_sop/vendor_manual/unknown`，只作提示，不改变现有绑定规则。
- 不把示例值写成默认值；不新增自动换算。

### 2.4 版本字段

在 `ProtocolBlockVersionData` 增加：

```json
{ "schema_version": 2 }
```

- 旧记录缺省为 `1`。
- 新建/修订写 `2`。
- 本轮不做数据迁移；读取时按字段存在性解释。
- 未来若需要真正的迁移，再用 `schema_version` 分流。

## 3. 后端改动

### 3.1 Rust 结构

在 `src/cli.rs` 增加：

```rust
#[derive(Debug, Clone, Default, Deserialize, Serialize)]
#[serde(deny_unknown_fields)]
struct StepMeta { ... }

#[derive(Debug, Clone, Deserialize, Serialize)]
#[serde(deny_unknown_fields)]
struct StepControl { ... }

#[derive(Debug, Clone, Deserialize, Serialize)]
#[serde(deny_unknown_fields)]
struct DecisionPoint { ... }

#[derive(Debug, Clone, Deserialize, Serialize)]
#[serde(deny_unknown_fields)]
struct FailureMode { ... }

#[derive(Debug, Clone, Deserialize, Serialize)]
#[serde(deny_unknown_fields)]
struct StepReadout { ... }

#[derive(Debug, Clone, Deserialize, Serialize)]
#[serde(deny_unknown_fields)]
struct StepEvidence { ... }

#[derive(Debug, Clone, Deserialize, Serialize)]
#[serde(deny_unknown_fields)]
struct StepInstrument { ... }
```

`ProtocolStep::Text` 增加：

```rust
#[serde(default, skip_serializing_if = "StepMeta::is_empty")]
meta: StepMeta,
```

注意：

- `StepMeta` 必须 `Default`，旧 JSON 才能反序列化。
- 空 `meta` 不序列化，避免旧记录被无意义膨胀。
- 不要给 `meta` 设一个“默认目的”；缺省就是未填写。

### 3.2 校验

新增 `validate_step_meta(meta, path)`：

| 字段 | 规则 |
|---|---|
| `purpose` | ≤ 1200 字符；可空（草案） |
| `stage` | serde 枚举；未知值拒绝 |
| `controls` | ≤ 20；label/purpose 必填；`kind` 枚举 |
| `decision_points` | ≤ 20；question 必填；continue/stop 至少一个 |
| `failure_modes` | ≤ 20；mode 必填；action 可空但显示为待补 |
| `readout` | raw/not_implied 各 ≤ 1200；derived 可空 |
| `sample_constraints` | 枚举去重；≤ 10 |
| `evidence` | ≤ 20；citation 必填；URL ≤ 2048；不自动抓取 |
| `instrument` | class ≤ 80；profile_id ≤ 120；未知 profile 只提示 |

在 `validate_block_content` 的 `ProtocolStep::Text` 分支调用。

不要因为 `meta` 缺失而拒绝保存草案；只在预览/执行台显示“步骤语义未补”。只有结构损坏/枚举非法/超长才拒绝写入。

### 3.3 展开与 Run 冻结

`expand_block` 的 `ProtocolStep::Text` 分支增加：

```rust
if !meta.is_empty() {
    rendered["meta"] = serde_json::to_value(meta)?;
}
```

`protocol_block_manifest` 和 `protocol_manifest_entry` 已经复制 `steps`，因此自动进入 Run 冻结快照。旧 Run 没有 `meta`，前端按空处理。

不需要改 `execution_event` 的结构；事件仍通过 `protocol_step_path` + `protocol_version_id` 找到冻结步骤，再从步骤读 `meta`。不要在每个事件里重复存一份完整 `meta`，避免漂移。

### 3.4 API / CLI schema

需要更新的 schema 位置：

- `parameter_slot_input_schema()`：加 `description`、`source`。
- `protocol_step_input_schema()`：text 分支加 `meta`。
- `record_data_schema("protocol_block_version")`：steps 加 `meta`。
- `protocol.block.preview` 输出 schema：steps 增加可选 `meta`。
- `record_data_schema("run")` 中 `protocol_manifest.steps` 如果是严格 schema，也同步。
- `tool_help` / `tool_effects`：说明 `meta` 只记录语义，不自动执行。

HTTP 不需要新路由：`/api/records` 已能写 `protocol_block` / `protocol_block_version`，`/api/protocol-blocks/preview` 已能预览。新增字段由 JSON 透传。

### 3.5 兼容性规则

- 旧记录无 `meta`：默认空，读取/预览/旧 Run 正常。
- 新记录有 `meta`：旧版本二进制不保证可读；本项目是单机应用，升级时整体替换，不做降级兼容承诺。
- `protocol.block.revise` 仍是整块替换：前端必须把已有 `meta` 回传；否则新版本会丢语义。测试必须覆盖。
- 不要新增 `step_meta` 表；不要 JSON Patch 子结构；不要做自动迁移。
- `deny_unknown_fields` 保留；新字段必须是显式字段，防止错字静默丢失。

## 4. 前端改动

### 4.1 页面结构

仍用现有 `protocols` 视图，不新增一级导航：

```text
Protocol
├─ 步骤库
│  ├─ 筛选：阶段 / 仪器 / 搜索
│  ├─ 步骤卡：标题、目的、阶段、对照数、版本
│  └─ 新建步骤块
└─ Protocol 库
   ├─ Protocol 卡：标题、步骤数、未解决参数、版本
   └─ 新建 Protocol
```

不使用全局“工具中心”。仪器步骤从步骤库/插入菜单进入。

### 4.2 步骤块编辑器

把 `createProtocolBlock` 从“单步骤表单”升级为：

```text
[基本信息]
标题 / 目的 / 阶段 / 摘要

[步骤 01]  [上移] [下移] [删除]
  说明
  参数槽（可添加）
  对照（可添加）
  判定点（可添加）
  失败模式（可添加）
  读数边界（raw / derived / not_implied）
  样本约束
  仪器（可选）
  证据（可添加）
[+ 添加步骤]
[预览] [保存为新版本]
```

实现建议：

- `stepDraft` 状态：`{use_id, title, instruction, parameter_refs, visualization, meta}` 数组。
- 复用现有 `modal`, `field`, `area`, `esc`, `read...` 函数。
- `renderStepEditor(step, index)` 生成可折叠块；默认只展开“说明+参数”，其余 `<details>`。
- `readProtocolBlockFields(form)` 改为读取所有 `.step-row`，返回 `steps[]`，每个含 `meta`。
- `add-step-row` / `remove-step-row` / `move-step` 只改 DOM 状态，保存时统一读取。
- 旧的单步骤版本仍能打开；没有 `meta` 时显示“尚未补充步骤语义”，不自动生成。

### 4.3 Protocol 组合器

当前 `createProtocol` 只能选一个根块。需要增加“组合”模式：

```text
Protocol 组合
01 [接种细胞]        [参数] [上移] [删除]
02 [处理]            [参数] [上移] [删除]
03 [读取 OD450]      [参数] [上移] [删除]
[+ 在此插入步骤]
预览：6 步 / 2 个参数待确认
[保存 Protocol]
```

实现方式（不新增后端类型）：

1. 用户保存组合时，前端创建一个 `protocol_block`（root），其 `steps` 为 `block_use` 数组，按顺序引用子块版本并绑定参数。
2. 再用这个 root block version 调 `protocol.create`。
3. 如果用户编辑已有 Protocol，创建新 root block version，再调 `protocol.revise`。
4. 插入菜单按阶段/容器/检测过滤；选中步骤块后立即显示参数绑定；未填参数保留为空。

这样仍然只有一种根步骤块模型，不引入 `protocol.steps[]` 第二套结构。

### 4.4 仪器/工具模块

- 在“插入步骤”菜单里增加“仪器/操作”分组：移液、离心、孵育、洗板、读板、成像、qPCR、流式等。
- 选中后创建一个子块，`meta.instrument.class` 写入；`profile_id` 未知就留空。
- 不弹“是否使用该仪器”；只在用户主动插入时出现。
- 不自动填转速、波长、体积、时间；参数槽为空，预览显示待确认。
- 仪器模块仍是步骤块；没有独立 `tools` 页面/表。

### 4.5 执行台

在 `renderOperationWorkbench` 的步骤详情中加一个 `op-meta` 区：

```text
为什么做：...
需要对照：无细胞空白（必需）...
判定：读数在线性范围才继续；饱和则稀释/重做
读数边界：OD450 ≠ 活细胞数
失败模式：气泡 / 蒸发 / 抑制...
来源：Dojindo CK04e
```

- `meta` 来自 Run 冻结步骤，不来自当前 Protocol 编辑状态。
- 完成状态仍只看 `execution_event.step_status`；`meta` 不自动标记完成。
- 没有 `meta` 时显示“该步骤未记录语义”，不要补写。

### 4.6 手机/窄屏

- 步骤卡纵向排列；编辑区在下方，不并排。
- 高级字段默认折叠；参数/对照先显示。
- 不使用宽表；对照/失败模式用定义列表或卡片。
- 390px 下检查无横向溢出；长 URL/来源可换行。

## 5. 起步步骤库

不要自动向用户数据库塞“系统步骤”。建议做显式导入：

- 先准备 8–12 个通用步骤模板（JSON）：细胞接种、离心、孵育、洗板、吸光度读板、荧光读板、显微成像、qPCR、流式、CV/CFU、IF、样本存储。
- 每个模板带 `meta`、参数槽、来源；不含具体实验剂量/时间默认值。
- UI 提供“查看起步步骤库 → 选择 → 导入到当前项目”；预览后再写入。
- 幂等：按 `title + source_key` 检查；重复导入提示，不创建重复块。
- 后端可先不加专用导入接口，前端可以逐条调用 `protocol.block.create`；但 10+ 条会多次请求，后续再做 `protocol.block.import` 批量事务。

起步库不是“所有实验”，只是让用户第一次打开时不必从空白开始；任何模板都可改参数、保存新版本。

## 6. 测试

### 6.1 Rust/CLI

1. 旧 `protocol_block_version` 无 `meta`：`protocol.block.get/preview` 正常，输出无 `meta`。
2. 新块带完整 `meta`：create → get → preview，字段原样返回。
3. 非法 `stage` / 非法 `control.kind` / 超长 `purpose` → `INVALID_INPUT`，错误路径指向步骤。
4. `meta` 缺 `purpose`：允许保存；preview 返回 advisory（如果实现）。
5. `protocol.block.revise` 带 `meta`：v2 保留，v1 不变；不带 `meta` 时明确产生空 `meta`，测试记录该行为。
6. 组合块引用子块：子块 `meta` 在展开步骤中出现；父块不重复存储。
7. `run.create` 冻结 `protocol_manifest.steps[].meta`；旧 Run 无 `meta` 仍可读。
8. `execution_event.create` 仍按 `protocol_step_path`/`protocol_version_id` 校验；不因 `meta` 改变。
9. HTTP `/api/protocol-blocks/preview` 返回 `meta`，schema 通过。
10. 不自动填值：只给 `purpose`/`stage`、不给参数，preview 仍 `ready=false`。

### 6.2 前端 smoke

11. 新建多步骤块：添加 2 步、各自参数/对照/读数，保存；`/api/state` 中 `steps.length===2` 且 `meta` 存在。
12. 编辑已有块：回传 `meta`，保存新版本；旧版本仍可读。
13. Protocol 组合器：插入两个块、绑定参数、保存；preview 顺序正确，未填参数显示“待确认”。
14. 执行台：打开含 `meta` 的 Run，显示 purpose/controls/readout；记录事件后状态不自动完成。
15. 390px 视口：步骤编辑器/执行台无横向溢出。
16. 没有 `meta` 的旧 Run：显示“步骤语义未记录”，不报错。

## 7. 分阶段交付

| 阶段 | 内容 | 验收 |
|---|---|---|
| P0 | **只读实验地图** `design.map` + CLI/HTTP + 设计卡片入口 | 地图回答七个问题；缺失项明确；不新增字段 |
| P1 | **执行台切片**：L929 冻结 Protocol/Run + 地图面包屑 + 完整步骤/参数/记录 | 打开 Run 能看到完整步骤；记录绑定正确；动画不算完成 |
| P2 | `StepMeta` 结构、校验、预览/Run 透传、schema 更新、测试 | 旧记录可读；新 `meta` 可往返；不自动填值 |
| P3 | 多步骤块编辑器 + 语义字段 | 能创建含对照/判定/读数的步骤块 |
| P4 | Protocol 组合器：插入、排序、绑定、预览 | 能组合两个以上块并保存 Protocol |
| P5 | 执行台 `meta` 面板 + 手机布局 + 起步步骤库 | 执行时看到目的/对照/读数边界；模板显式导入 |

P0/P1 不依赖 `StepMeta`；P2 之后才增加 Protocol 编辑复杂度。

## 8. 风险与决策

1. **`meta` 嵌套 vs 顶层**
   - 选嵌套：语义跟步骤走，组合展开自然；代价是 CLI schema 变深。
   - 不选新表：避免迁移、join、版本同步。

2. **`revise` 整块替换**
   - 旧客户端可能丢 `meta`。前端必须回传；版本历史保留旧值。
   - 若要更稳，可让 `revise` 接受可选 `meta_by_use_id` 并在缺省时保留；但这会增加合并规则，第一版不做。

3. **`controls` 的层级**
   - 步骤 `controls` 只表达“方法要求”；实际对照仍是设计字段。
   - 不做全局对照库；不自动把某个组标成空白。

4. **仪器 profile**
   - 未知型号只写 `class`，不编造转速/波长/通道。
   - `profile_id` 是可选引用；没有 profile registry 时 UI 显示“型号未指定”。

5. **步骤粒度**
   - 一个步骤块可以多步；不要强制每个动作单独建块。
   - 组合器按 `block_use` 插入；可把连续步骤保存为子块，但第一版不要求。

6. **旧记录/降级**
   - additive 字段 + serde default 保证旧数据读入。
   - 新数据被旧二进制读取不在支持范围；单机应用升级即可。

## 9. 非目标

- 不建 `tools` 表或工具中心。
- 不建通用工作流/条件/循环引擎。
- 不自动推断剂量、时间、温度、转速、波长、细胞密度。
- 不把 `readout` 自动翻译成生物结论。
- 不自动把技术重复当独立 n。
- 不把动物/临床研究塞进孔板 Protocol 模型。
- 不在第一版实现完整 ISA/OBI/RO-Crate 导出；只保留字段结构，后续映射。
