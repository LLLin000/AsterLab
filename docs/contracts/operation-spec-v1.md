# OperationSpec Contract v1（类型化操作契约）

- 日期：2026-11-13
- 状态：契约 + 已编译类型；**未接入执行器、CLI、Protocol 预览、Run 或 GUI**。
- 代码：[`crates/asterlab-core/src/operation_spec.rs`](../../crates/asterlab-core/src/operation_spec.rs)
- 关联：[`operation-module-v1.md`](operation-module-v1.md)（目录级操作知识）、[`core-v1.md`](core-v1.md)（Core 边界）、[`../../CONTEXT.md`](../../CONTEXT.md)（术语）、[`../sdk/science-field-ownership.md`](../sdk/science-field-ownership.md)（字段归属）
- 机器校验：`cargo test -p asterlab-core operation_spec`；文档示例由测试从本文件反向读取并断言（见 §11）。

## 0. 一句话

`OperationSpec` 把液体转移、离心、孵育、洗涤、分样、测量六类操作的参数、输入输出和检查条件表达为**封闭类型**；`ProtocolBlock` 仍是操作的权威来源，spec 只是它的类型化描述，不产生第二套执行语义。

## 1. 边界

| 是 | 不是 |
|---|---|
| 六类操作的参数、输入输出、检查条件的类型定义 + serde | 执行器、任务展开或仪器控制 |
| 结构化校验：单位族、缺参、越界 | 科学合理性判断或参数推荐 |
| 与 `ProtocolBlockVersionData` 的挂载说明（字段名另卡决定） | 新增数据库表、第二条写路径 |
| 版本、并发、回滚边界 | SDK 模块 kind 或 `spec/modules.json` 规则 |

本轮不改 `ProtocolBlockVersionData` 现有字段，不改 `AGENT_API.md` 命令，不改任何 `.slint` / MyGo 包。

## 2. 与 ProtocolBlock 的关系

### 2.1 权威顺序

1. `ProtocolBlockVersion`（`steps`、`parameter_slots`、`StepBinding`、`ProtocolVersion` 根绑定）决定**执行什么、用什么值**。
2. `OperationSpec` 是同一操作的类型化描述：读它不能得到第二个参数来源，也不能绕过 `protocol.block.revise` 的校验。
3. 冲突时不静默选边：block 保持不变，spec 判为不一致，由 block 校验或后续编译卡产出 `blocking` 级 [`ValidationIssue`](../../CONTEXT.md)。禁止把 spec 的值自动写回 block 或反向覆盖绑定。

### 2.2 挂载形态与字段归属

| 形态 | 说明 | 权威 |
|---|---|---|
| 内联 | `ProtocolStep::Text.operation_spec: Option<OperationSpec>`；与文本步骤一同保存在其 block version | 同一 block version |
| 引用 | `BlockUse` 不复制 spec；通过其 `block_version_id` 读取确切子 block version 中各文本步骤的 spec | 精确子 block version |

- `operation_spec` 使用 serde 默认值；缺省等价于 `None`，序列化时省略 `None`，旧 block 无需迁移且往返不变。
- spec 随 block version 不可变。组合块校验/展开按 `BlockUse` 引用的精确版本读取，错误路径包含展开步骤位置。
- Core 在 block revision 校验和 ProtocolBlock 展开校验时调用 `OperationSpec::validate()`；不支持的 schema version 在加载/校验时拒绝。校验只报告闭合的 spec 错误码，不改写步骤、参数、绑定或 checks。
- 同一 spec 被引用两次是两个使用位置，值语义共享、状态不共享。spec 只描述参数与检查项，不产生第二条执行路径。

### 2.3 v1 兼容规则

1. 没有 spec 的 block 仍然可读、可预览、可冻结、可执行；缺失 spec 不产生任何推断或默认值。
2. 旧 Run 冻结的内容不重新解析 spec；新增 spec 不回写已冻结 Run。
3. `deny_unknown_fields` 全开：拼错的键是加载错误，不静默丢弃。
4. 不承诺旧二进制读新 spec 版本；单机升级整体替换。

## 3. 类型总览

### 3.1 顶层结构

```rust
pub const OPERATION_SPEC_SCHEMA_VERSION: u32 = 1;

pub struct OperationSpec {
    pub schema_version: u32,
    pub operation: Operation,   // tag: kind
    pub checks: Vec<StepCheck>, // 复用 protocol_preview::StepCheck
}

pub enum Operation {
    Transfer(TransferSpec),
    Centrifuge(CentrifugeSpec),
    Incubate(IncubateSpec),
    Wash(WashSpec),
    Aliquot(AliquotSpec),
    Measure(MeasureSpec),
}

pub struct Quantity { pub value: f64, pub unit: Unit }
pub struct ContainerRef { pub container_id: String }
```

- 枚举外形：`{"kind": "transfer", ...}`。未知 `kind` 是加载错误。
- `Quantity` 是数 + 声明单位；v1 **不做换算**，值按声明单位原样保留。
- `ContainerRef` 只接受稳定容器 ID；裸孔位坐标（`A1`）不是容器 ID，孔位仍是 `plate_id + coordinate`。

### 3.2 单位与量纲

封闭单位集（serde 词元 = 下表字符串）：

| 单位 | 词元 | 量纲 `UnitFamily` |
|---|---|---|
| 微升 / 毫升 / 升 | `uL` / `mL` / `L` | volume |
| 秒 / 分 / 小时 | `s` / `min` / `h` | time |
| 摄氏度 | `C` | temperature |
| 相对离心力 | `xg` | force |
| 转速 | `rpm` | rotation |
| 吸光度 / 相对荧光 / 计数 | `AU` / `RFU` / `count` | measurement |

每个字段声明它接受的量纲；词元合法但量纲不符 → `unit_mismatch`。词元不在集合内 → serde 加载错误（不属于三码）。

### 3.3 检查条件

- 复用 `protocol_preview::StepCheck`（`check_id` / `label` / `instruction` / `blocking`），不新增第二种 check 类型。
- spec 的 `checks` 是操作级检查条件；执行阻断仍以步骤定义的 checks 为准。
- `check_id` 必须非空；重复 ID 与阻断级别由 block 校验负责，本契约不重复实现。

## 4. 六类操作

### 4.1 液体转移 transfer

| 字段 | 类型 | 必填 | 量纲 |
|---|---|---|---|
| `volume` | `Quantity` | 是（`null` 表示草案未定） | volume |
| `source` | `ContainerRef` | 是 | — |
| `destination` | `ContainerRef` | 是 | — |

- 输入：源容器、体积；输出：目标容器中的材料。
- 拒绝：体积缺失、容器 ID 空白 → `missing_parameter`；体积单位非 volume 或值 ≤ 0 → `unit_mismatch` / `value_out_of_range`。
- 不检查：源与目标是否同一容器、容器是否真实存在（block 与 Core 负责）。

```json
{
  "schema_version": 1,
  "operation": {
    "kind": "transfer",
    "volume": { "value": 200.0, "unit": "uL" },
    "source": { "container_id": "tube-source-1" },
    "destination": { "container_id": "plate-96-1" }
  }
}
```

### 4.2 离心 centrifuge

| 字段 | 类型 | 必填 | 量纲 |
|---|---|---|---|
| `drive` | `{"kind":"rcf","force":Quantity}` 或 `{"kind":"speed","speed":Quantity}` | 是 | force 或 rotation |
| `duration` | `Quantity` | 是 | time |

- 输入：离心力或转速、时长；输出：分离后的同一容器（本契约不声明 pellet/supernatant 新对象）。
- 力与转速**二选一**，类型上不可同时出现；没有转子半径，`rpm` 不换算为 `xg`。
- 拒绝：`drive` 或 `duration` 缺失 → `missing_parameter`；`rcf` 声明 `rpm` → `unit_mismatch`；≤ 0 → `value_out_of_range`。

```json
{
  "schema_version": 1,
  "operation": {
    "kind": "centrifuge",
    "drive": { "kind": "rcf", "force": { "value": 300.0, "unit": "xg" } },
    "duration": { "value": 5.0, "unit": "min" }
  }
}
```

### 4.3 孵育 incubate

| 字段 | 类型 | 必填 | 量纲 |
|---|---|---|---|
| `temperature` | `Quantity` | 是 | temperature |
| `duration` | `Quantity` | 是 | time |

- 输入：温度、时长；输出：状态变化后的同一材料（不新建对象）。
- 拒绝：缺参 → `missing_parameter`；温度非 Celsius → `unit_mismatch`；温度 ≤ −273.15 C 或非有限、时长 ≤ 0 → `value_out_of_range`。

```json
{
  "schema_version": 1,
  "operation": {
    "kind": "incubate",
    "temperature": { "value": 37.0, "unit": "C" },
    "duration": { "value": 24.0, "unit": "h" }
  }
}
```

### 4.4 洗涤 wash

| 字段 | 类型 | 必填 | 量纲 |
|---|---|---|---|
| `cycles` | `u32` | 是 | — |
| `volume` | `Quantity` | 是 | volume |

- 输入：次数、每次体积；输出：洗涤后的同一材料（废液是执行事实，不在 spec）。
- 拒绝：缺参 → `missing_parameter`；`cycles` = 0 → `value_out_of_range`；单位非 volume → `unit_mismatch`。

```json
{
  "schema_version": 1,
  "operation": {
    "kind": "wash",
    "cycles": 3,
    "volume": { "value": 200.0, "unit": "uL" }
  }
}
```

### 4.5 分样 aliquot

| 字段 | 类型 | 必填 | 量纲 |
|---|---|---|---|
| `parts` | `u32` | 是 | — |
| `per_part_volume` | `Quantity` | 是 | volume |

- 输入：份数、每份体积；输出：`parts` 份子样本（`Aliquot` 身份由 Run 记录，spec 只描述参数）。
- 拒绝：缺参 → `missing_parameter`；`parts` = 0 → `value_out_of_range`；单位非 volume → `unit_mismatch`。

```json
{
  "schema_version": 1,
  "operation": {
    "kind": "aliquot",
    "parts": 4,
    "per_part_volume": { "value": 50.0, "unit": "uL" }
  }
}
```

### 4.6 测量 measure

| 字段 | 类型 | 必填 | 量纲 |
|---|---|---|---|
| `endpoint_id` | `String` | 是（非空白） | — |
| `unit` | `Unit` | 是 | measurement |

- 输入：设计 `Endpoint` 的稳定 ID 与读数单位；输出：原始观测值（执行事件写入，**不写进 spec**）。
- `endpoint_id` 是引用，不是自由文本标签；spec 不重新定义 Endpoint 的含义或时间点。
- 拒绝：`endpoint_id` 空白或 `unit` 缺失 → `missing_parameter`；`unit` 非 measurement（如 `uL`）→ `unit_mismatch`。
- 不检查：Endpoint 是否属于本次设计的 Assay（Core 查询负责）。

```json
{
  "schema_version": 1,
  "operation": {
    "kind": "measure",
    "endpoint_id": "endpoint-cck8-od450",
    "unit": "AU"
  }
}
```

## 5. 错误码（封闭）

`OperationSpec::validate()` 只可能返回下列三个 `code`；路径以 `operation.` 或 `checks[` 开头，可直接锚定字段。`OPERATION_SPEC_ERROR_CODES` 与本节由测试互校。

### 5.1 `unit_mismatch`

- 触发：字段声明的单位词元合法，但量纲与该字段不符（体积写成 `min`、`rcf` 写成 `rpm`、测量单位写成 `uL`）。
- 不触发：未知词元（serde 加载错误）。
- 处置：拒绝该 spec；不改写单位，不做隐式换算。

### 5.2 `missing_parameter`

- 触发：必填数值字段为 `null`/缺省（草案未定）、引用型字段为空串、`check_id` 为空。
- 语义：草案可以保存；只阻止依赖该值的冻结或执行，不是数据损坏。
- 处置：保留为待确认，不填入“常见值”。

### 5.3 `value_out_of_range`

- 触发：数值非有限（NaN/∞）；体积、时长、离心力、转速 ≤ 0；温度 ≤ −273.15 C；`cycles`/`parts` = 0。
- 不触发：科学上合理但未在本契约声明的范围。本契约只做结构性边界，科学范围仍由 block `parameter_slots` 的约束决定。
- 处置：拒绝；不夹取（clamp）到边界。

## 6. 完整示例

两个示例都是可直接 `serde_json::from_str::<OperationSpec>` 的完整对象，并已被测试断言 `validate()` 通过。

### 示例 1：CCK-8 加样（transfer）

```json
{
  "schema_version": 1,
  "operation": {
    "kind": "transfer",
    "volume": { "value": 100.0, "unit": "uL" },
    "source": { "container_id": "tube-cck8-working" },
    "destination": { "container_id": "plate-96-cck8" }
  },
  "checks": [
    {
      "check_id": "no_bubbles",
      "label": "孔内无气泡",
      "instruction": "加样后目视确认无气泡，必要时轻敲板边。",
      "blocking": true
    }
  ]
}
```

### 示例 2：CCK-8 读数（measure）

```json
{
  "schema_version": 1,
  "operation": {
    "kind": "measure",
    "endpoint_id": "endpoint-cck8-od450",
    "unit": "AU"
  },
  "checks": [
    {
      "check_id": "blank_well",
      "label": "空白孔已包含",
      "blocking": true
    }
  ]
}
```

## 7. 拒绝示例

每个示例都能被 serde 解析（字段形状正确），由 `validate()` 拒绝；小标题里的错误码就是首个 issue 的 `code`。

### 7.1 `unit_mismatch`：孵育温度写成时间单位

```json
{
  "schema_version": 1,
  "operation": {
    "kind": "incubate",
    "temperature": { "value": 37.0, "unit": "min" },
    "duration": { "value": 2.0, "unit": "h" }
  }
}
```

首个 issue：`operation.temperature`，`unit min is time, expected temperature`。

### 7.2 `missing_parameter`：洗涤次数未定

```json
{
  "schema_version": 1,
  "operation": {
    "kind": "wash",
    "volume": { "value": 200.0, "unit": "uL" }
  }
}
```

首个 issue：`operation.cycles`，`count is not set`。

### 7.3 `value_out_of_range`：分样份数为 0

```json
{
  "schema_version": 1,
  "operation": {
    "kind": "aliquot",
    "parts": 0,
    "per_part_volume": { "value": 50.0, "unit": "uL" }
  }
}
```

首个 issue：`operation.parts`，`count must be at least 1`。

## 8. 版本、并发与回滚

- **版本**：`schema_version` 必须为 1，由 `OperationSpec::is_supported_schema_version()` 在**加载时**判定。拒绝版本不是三码之一：它是加载错误，调用方不得把不支持的 spec 交给 `validate()` 后当成“通过”。字段语义变化升到 2，旧 spec 保留，不隐式迁移。
- **并发**：spec 不是独立记录，随 block version 存储；写入复用 `protocol.block.revise` 的 `expected_version`，没有第二条写路径。同一 spec 值可在多个引用中出现，但不存在可变的共享状态。
- **回滚**：删除 spec 字段即回到 v1 纯文本 block，执行语义不变，无需数据迁移；已冻结 Run 不重新解析 spec。`validate()` 是纯函数，无副作用，失败不留下部分写入。
- **幂等**：同一 spec 反复校验得到同一 issue 列表（路径与顺序稳定）。

## 9. dry-run 示例

dry-run = 加载 spec + `validate()`，不写库、不改 block、不生成 Run。下例让洗涤次数保持未定：

```json
{
  "schema_version": 1,
  "operation": {
    "kind": "wash",
    "volume": { "value": 200.0, "unit": "uL" }
  }
}
```

输出（`Vec<SpecIssue>`，即本 crate 的序列化结果）：

```json
[
  {
    "code": "missing_parameter",
    "path": "operation.cycles",
    "message": "count is not set"
  }
]
```

调用方据此显示“洗涤次数待定”，允许保存草案，只阻止冻结/执行；不自动填 3 次。

## 10. 校验边界

`validate()` 通过不等于可执行。以下明确不在本契约：

| 不做 | 归属 |
|---|---|
| spec 与 block 参数/绑定是否一致 | block 校验与编译卡（§2） |
| 单位换算（`mL` ↔ `uL`，`rpm` → `xg`） | 后续卡；v1 只判量纲 |
| 科学合理性（37 C 是否适合该细胞） | 研究者 / 专家审核 |
| 仪器能力与真实可用性 | `InstrumentProfile` / `instrument-matching-v1.md` |
| 孔位坐标、容器存在性 | Core 查询与 Run |
| 读数数值、实际参数、偏差 | `ExecutionEvent` |
| 参数来源证明 | `ParameterBinding` / `operation-module-v1.md` |

另：`missing_parameter` 表示“草案未定”，不是“不适用”；不得用 `null` 表达“该操作确实没有这一步”。

## 11. 测试与验收

```bash
cargo test -p asterlab-core operation_spec   # exit 0，>= 6 个测试通过
cargo check -p asterlab-core                 # exit 0
grep -c '^### ' docs/contracts/operation-spec-v1.md
```

测试覆盖：六类操作的序列化往返；力/转速二选一；三个错误码各自的触发路径与 `path`；非有限与 ≤ 0 边界；`deny_unknown_fields`；版本门槛不进入三码；三个错误码与本文 §5 互校；**本文 §6 的两个示例必须校验通过、§7 的三个示例必须按小标题的错误码被拒绝**。

## 12. 未决项

1. 字段与校验时机已确定：spec 是 `ProtocolStep::Text.operation_spec` 可选 sidecar；block version revision 校验及 ProtocolBlock 展开校验调用 `OperationSpec::validate()`，字段路径前缀带步骤位置。`BlockUse` 只引用确切子 block version，不复制 spec。
2. 单位换算与科学范围：是否引入 block `parameter_slots.constraints` 之外的边界。
3. 测量类 spec 是否需要 `time_scope`（本契约只写端点与单位，时间点仍由设计决定）。
4. `checks` 与步骤 checks 的一致性规则（当前只有非空 `check_id` 校验）。
5. 离心 `rpm → xg` 所需转子半径的来源（`InstrumentProfile` 还是步骤参数）。

## 13. 来源

- 术语与对象边界：[`CONTEXT.md`](../../CONTEXT.md)
- Core 契约：[`core-v1.md`](core-v1.md)
- 目录级操作知识：[`operation-module-v1.md`](operation-module-v1.md)
- 字段归属：[`../sdk/science-field-ownership.md`](../sdk/science-field-ownership.md)
- 代码：[`operation_spec.rs`](../../crates/asterlab-core/src/operation_spec.rs)
