# Readout Schema v1（通用读数结构契约）

- 日期：2026-11-13
- 状态：v1 契约冻结；本卡只落类型与文档，不写库、不改 `execution_event.create`。
- 代码：[`crates/asterlab-core/src/readout_schema.rs`](../../crates/asterlab-core/src/readout_schema.rs)
- 关联：[Core Contract v1](core-v1.md) §2.3、[architecture](../architecture.md) §5.4/§5.5、[AGENT_API](../../AGENT_API.md) `execution_event.create`、[CONTEXT](../../CONTEXT.md) 读模型与“值、来源与事实状态”。

## 0. 目标与边界

一次 Run 的读数不再按实验分别定义记录结构：数值、表格、孔板、分类结果、文件引用五类读数共用一套 `ReadoutSchema` + `Readout`。记录组件按 `value_kind` 选择渲染方式，按 `unit` / `missing` / `source` / `link` 显示单位、缺失原因、来源和样本/时间点关联。

范围边界：

- 本契约是**读数的形状与校验**，不是新的存储表。读数仍只来自 `execution_event` 追加记录（[architecture §5.4](../architecture.md)）。
- 旧事件不改写、不迁移、不补字段；本类型只约束新写入。
- 不新增 CLI 命令、SDK kind 或 GUI 行为。

## 1. 类型

### 1.1 ReadoutSchema

```rust
pub const READOUT_SCHEMA_VERSION: u32 = 1;

pub struct ReadoutSchema {
    pub schema_id: String,
    pub schema_version: u32,       // 必须等于 READOUT_SCHEMA_VERSION
    pub endpoint: String,          // 测量指标字段，如 od450 / cfu / a260_a280
    pub value_kind: ValueKind,
    pub unit: UnitSpec,            // { canonical, required }
    pub missing: MissingSpec,      // { allowed[], null_allowed }
    pub source: SourcePolicy,      // { raw, derived }
    pub link: LinkPolicy,          // { timepoint_required, sample_required }
    pub table: Option<TableSpec>,
    pub plate: Option<PlateSpec>,
    pub categorical: Option<CategoricalSpec>,
    pub file: Option<FileSpec>,
}
```

规则：

- `schema_id + schema_version` 是稳定引用；`Readout` 引用它，不复制形状。
- 只有与 `value_kind` 对应的形状字段可以出现：缺 `kind_spec_missing`，多 `kind_spec_unexpected`。
- `endpoint` 属于 schema，不在 `Readout` 里重复；旧 `readouts[]` 的 `endpoint` 由 schema 提供。
- 所有结构 `deny_unknown_fields`；未知字段是反序列化错误，不是可忽略的附加信息。

### 1.2 value_kind 与形状

| `value_kind` | 形状字段 | 值形态 | 适用 |
|---|---|---|---|
| `numeric` | 无 | `{ "kind": "numeric", "number": 1.0 }` | 单点标量：OD、浓度、计数、比值 |
| `table` | `TableSpec { columns[] }` | `{ "kind": "table", "columns": [...], "rows": [[...]] }` | 一行一样本的多指标读数 |
| `plate` | `PlateSpec { format, rows, columns }` | `{ "kind": "plate", "plate_id", "rows", "columns", "cells": [{coordinate, number|missing}] }` | 孔板整板读数 |
| `categorical` | `CategoricalSpec { options[], allow_other }` | `{ "kind": "categorical", "category": "positive" }` | 染色判定、阴性/阳性 |
| `file` | `FileSpec { media_types[], require_checksum }` | `{ "kind": "file", "asset_id", "media_type", "bytes", "sha256?" }` | 仪器原始导出、图像 |

`PlateFormat` 是封闭值：`"6"`、`"12"`、`"24"`、`"48"`、`"96"`、`"384"`、`"custom"`。标准板型的 `rows × columns` 必须等于 2×3、3×4、4×6、6×8、8×12、16×24；`custom` 无固定尺寸，但行列不得为 0。

### 1.3 单位

```rust
pub struct UnitSpec { pub canonical: Option<String>, pub required: bool }
```

- `canonical` 是本文本记录的单位（`AU`、`ng/µL`、`CFU`、`ratio`）；不按 endpoint 名推断。
- `required = true` 时 `Readout.unit` 必须存在且等于 `canonical`（`missing_unit` / `unit_mismatch`）。
- `required = false` 且 `canonical = null` 表示无标量单位或单位未确认；未知保持 `null`，不补默认值。
- `file` / `categorical` 不携带标量单位，声明即 `unit_not_applicable`。
- 单位换算不属于本契约：不允许隐式换算，单位不一致即 `unit_mismatch`。

### 1.4 缺失值语义：null ≠ 原因

```rust
pub enum MissingReason { NotDetected, BelowLod, AboveUloq, Invalid, Other }
pub struct MissingSpec { pub allowed: Vec<MissingReason>, pub null_allowed: bool }
```

三种状态必须分开，且互不替代：

| 状态 | 表达 | 含义 |
|---|---|---|
| 已记录 | `number` 有值，`missing = null` | 真实测量值 |
| 明确缺失 | `number = null`，`missing = "<reason>"` | 测了但低于/高于范围、未检出、无效 |
| 未记录 | `number = null`，`missing = null` | 该格没有录入；不是 0，不是“未检出” |

- `null_allowed = false` 时“未记录”被拒绝（`missing_without_reason`）：写入端必须给值或给原因。
- `missing` 不在 `allowed` 内 → `missing_reason_not_allowed`；同时有数值和原因 → `reason_with_value`。
- `plate` 的缺失原因按孔记录在 `cells[].missing`；顶层 `missing` 对 `plate`/`table`/`file` 不适用（`top_level_missing_not_applicable`）。
- 枚举值与 [AGENT_API](../../AGENT_API.md) 的 `readouts[].qualifier` 一致。卡面写作 `above_ulog`，该拼写不存在于 `AGENT_API.md`；本契约按 `above_uloq` 冻结，**没有**加别名（见 §8）。

### 1.5 来源

```rust
pub enum SourceKind { Raw, Derived }
pub struct ReadoutSource {
    pub kind: SourceKind,
    pub method: Option<String>,      // tool@version，派生时必填
    pub derived_from: Vec<String>,   // 派生时非空，只引用原始 readout_id
}
```

- `raw`：直接观测；不得带 `method` 或 `derived_from`（`raw_with_derivation`）。
- `derived`：必须同时有 `method` 与 `derived_from`（`derived_without_method` / `derived_without_sources`）；`derived_from` 只能指向同 Run 已存在的**原始**读数（`derived_source_unknown` / `derived_source_not_raw`）。
- schema 的 `source` 是准入策略：`derived = true` 必须 `raw = true`（`derived_requires_raw`）；未允许的来源被拒绝（`raw_not_allowed` / `derived_not_allowed`）。
- 派生结果不得写成原始读数。

### 1.6 timepoint / sample / target 关联

```rust
pub struct ReadoutLink {
    pub timepoint: Option<String>,          // 冻结设计里的时间点标签
    pub sample_id: Option<String>,
    pub target: Option<ReadingTarget>,      // well { plate_id, coordinate } | unit { unit_id }
}
```

- `timepoint` 必须是 Run 冻结设计中的时间点；不是自由文本。
- `sample_id` 是实际样本；样本、容器、孔位不互相替代。孔位必须同时有 `plate_id` 和坐标。
- `LinkPolicy` 声明必填项，缺则 `timepoint_missing` / `sample_missing`。
- `plate` 读数由 `cells[].coordinate` 承载目标，不再带单一 `target`（`target_not_applicable`）；`numeric` / `categorical` 可用 `target` 精确到孔或实验单位。

## 2. 与 `execution_event.create` 的 `readouts[]` 映射

### 2.1 旧事件不改写

- 已存储事件的 `readouts[]` 保持原字段与语义，本契约不迁移、不回填 `schema_id`，也不重算 `result_kind`。
- 读取旧读数时仍按 [AGENT_API](../../AGENT_API.md)：`value` XOR `qualifier`、`result_kind = raw | derived`、派生带 `method` 与 `derived_from`。
- 新 schema 是旧形式的超集：旧事件能表达的读数，新类型必须能无损表达；旧形式表达不了的（整板、表格、分类、文件）不塞进已知字段。

### 2.2 各 value_kind 的映射

| `value_kind` | `ReadoutSchema::legacy_form()` | 投影结果 |
|---|---|---|
| `numeric` | `single` | 1 条 `readouts[]`：`endpoint` 来自 schema，`value` XOR `qualifier`，带 `unit`、`sample_id`、`result_kind`，派生时带 `method` + `derived_from` |
| `plate` | `per_cell` | 每个 cell 1 条；`readout_id` 为 `<readout_id>-<coordinate>`，`qualifier` 取 `cells[].missing` |
| `table` | `unsupported` | 不投影；`readouts[]` 保持空，数据在表格记录 |
| `categorical` | `unsupported` | 不投影；`qualifier` 是固定缺失词汇，业务类别不等于缺失原因 |
| `file` | `unsupported` | 不投影；数据在资产记录，事件保留引用 |

`to_legacy_readouts(schema, readout)` 是**纯投影**，用于写入前 dry-run 预览；不写库、不追加事件。`unsupported` 返回 `legacy_form_unsupported`，调用方不得把非数值读数硬塞进 `readouts[]`。

dry-run 示例（CCK-8，节选）：

```json
{
  "readouts": [
    {"readout_id": "ro-cck8-1-A1", "endpoint": "od450", "value": 1.234, "unit": "AU",
     "result_kind": "raw", "derived_from": []},
    {"readout_id": "ro-cck8-1-A2", "endpoint": "od450", "qualifier": "above_uloq", "unit": "AU",
     "result_kind": "raw", "derived_from": []},
    {"readout_id": "ro-cck8-1-A3", "endpoint": "od450", "unit": "AU",
     "result_kind": "raw", "derived_from": []}
  ]
}
```

第 3 条即“未记录”：没有 `value`、没有 `qualifier`，与第 2 条的“明确高于上限”不同。

## 3. 错误码

`ReadoutIssue { code, path, message }`；`code` 稳定、可被后续卡直接引用。

| 阶段 | code | 触发 |
|---|---|---|
| schema | `invalid_schema_version` | `schema_version != 1` |
| schema | `empty_identifier` | `schema_id` 或 `endpoint` 为空 |
| schema | `unit_required_without_canonical` | `required = true` 但无 `canonical` |
| schema | `unit_not_applicable` | `file` / `categorical` 声明了单位 |
| schema | `duplicate_missing_reason` | `missing.allowed` 重复 |
| schema | `kind_spec_missing` | 缺少与 `value_kind` 对应的形状 |
| schema | `kind_spec_unexpected` | 出现其他形状字段 |
| schema | `empty_table_columns` | 表格无列 |
| schema | `duplicate_table_column` | 表格列 key 重复 |
| schema | `invalid_table_column_kind` | 列 kind 不是 `numeric` / `categorical` |
| schema | `invalid_plate_shape` | 板型与行列不符，或行列含 0 |
| schema | `empty_categorical_options` | 无类别 |
| schema | `duplicate_categorical_option` | 类别重复 |
| schema | `empty_file_media_types` | 无允许 media type |
| schema | `derived_requires_raw` | 只允许 `derived` 而禁止 `raw` |
| readout | `schema_id_mismatch` / `schema_version_mismatch` | 读数引用与 schema 不符 |
| readout | `value_kind_mismatch` | 值形态与 `value_kind` 不符 |
| readout | `unit_mismatch` / `missing_unit` | 单位不一致 / 必填单位缺失 |
| readout | `non_finite_number` | NaN / 无穷 |
| readout | `reason_with_value` | 有值又标缺失原因 |
| readout | `missing_without_reason` | 既无值也无原因，且 `null_allowed = false` |
| readout | `missing_reason_not_allowed` | 原因不在 `allowed` |
| readout | `top_level_missing_not_applicable` | `plate` / `table` / `file` 用顶层 `missing` |
| readout | `duplicate_plate_coordinate` | 同一孔位出现两次 |
| readout | `table_columns_mismatch` / `table_row_width_mismatch` | 列与 schema 不符 / 行宽不符 |
| readout | `category_not_allowed` | 类别不在 `options` 且 `allow_other = false` |
| readout | `file_media_type_not_allowed` / `file_checksum_required` | media type 未声明 / 缺校验和 |
| source | `derived_without_method` / `derived_without_sources` | 派生缺方法 / 缺来源 |
| source | `raw_with_derivation` | 原始读数带 `method` 或 `derived_from` |
| source | `raw_not_allowed` / `derived_not_allowed` | schema 未允许该来源 |
| source | `derived_source_unknown` / `derived_source_not_raw` | 来源不存在 / 来源本身是派生 |
| link | `timepoint_missing` / `sample_missing` | 必填时间点 / 样本缺失 |
| link | `target_not_applicable` | `plate` 读数带顶层 `target` |
| legacy | `legacy_form_unsupported` | 该读数没有无损 `readouts[]` 形式 |

处理规则：校验返回 issue 列表，不抛异常、不静默修正；`validate_schema` 先于 `validate_readout`，schema 不合法时不得写入。未知枚举值（如 `above_ulog`、未知字段）是 serde 反序列化错误，没有 issue 码。

## 4. 示例

### 4.1 CCK-8 OD450 孔板

```json
{
  "schema_id": "cck8_od450",
  "schema_version": 1,
  "endpoint": "od450",
  "value_kind": "plate",
  "unit": {"canonical": "AU", "required": true},
  "missing": {"allowed": ["not_detected", "above_uloq"], "null_allowed": true},
  "source": {"raw": true, "derived": false},
  "link": {"timepoint_required": true, "sample_required": false},
  "plate": {"format": "96", "rows": 8, "columns": 12}
}
```

```json
{
  "readout_id": "ro-cck8-1",
  "schema_id": "cck8_od450",
  "schema_version": 1,
  "value": {
    "kind": "plate",
    "plate_id": "plate-1",
    "rows": 8,
    "columns": 12,
    "cells": [
      {"coordinate": "A1", "number": 1.234},
      {"coordinate": "A2", "missing": "above_uloq"},
      {"coordinate": "A3"}
    ]
  },
  "unit": "AU",
  "source": {"kind": "raw", "derived_from": []},
  "link": {"timepoint": "处理后 24 h"}
}
```

A2 是“明确高于上限”，A3 是“未记录”；两者都不写成 0。

### 4.2 RNA 浓度与纯度

浓度是原始读数，纯度比例是派生读数：

```json
{
  "schema_id": "rna_concentration",
  "schema_version": 1,
  "endpoint": "concentration",
  "value_kind": "numeric",
  "unit": {"canonical": "ng/µL", "required": true},
  "missing": {"allowed": ["below_lod"], "null_allowed": false},
  "source": {"raw": true, "derived": false},
  "link": {"timepoint_required": true, "sample_required": true}
}
```

```json
{
  "schema_id": "rna_purity",
  "schema_version": 1,
  "endpoint": "a260_a280",
  "value_kind": "numeric",
  "unit": {"canonical": "ratio", "required": true},
  "missing": {"allowed": [], "null_allowed": true},
  "source": {"raw": true, "derived": true},
  "link": {"timepoint_required": true, "sample_required": true}
}
```

```json
{
  "readout_id": "ro-rna-conc-1",
  "schema_id": "rna_concentration",
  "schema_version": 1,
  "value": {"kind": "numeric", "number": 214.5},
  "unit": "ng/µL",
  "source": {"kind": "raw", "derived_from": []},
  "link": {"timepoint": "裂解后立即", "sample_id": "S-1"}
}
```

```json
{
  "readout_id": "ro-rna-ratio-1",
  "schema_id": "rna_purity",
  "schema_version": 1,
  "value": {"kind": "numeric", "number": 2.06},
  "unit": "ratio",
  "source": {
    "kind": "derived",
    "method": "a260_a280_ratio@1",
    "derived_from": ["ro-rna-a260-1", "ro-rna-a280-1"]
  },
  "link": {"timepoint": "裂解后立即", "sample_id": "S-1"}
}
```

`ro-rna-a260-1` / `ro-rna-a280-1` 必须是同 Run 已存在的**原始**吸光度读数；把比例写成 `raw`，或让派生引用另一个派生读数，都会被拒。

### 4.3 CFU 菌落计数

```json
{
  "schema_id": "cfu_count",
  "schema_version": 1,
  "endpoint": "cfu",
  "value_kind": "numeric",
  "unit": {"canonical": "CFU", "required": true},
  "missing": {"allowed": ["not_detected", "above_uloq"], "null_allowed": false},
  "source": {"raw": true, "derived": false},
  "link": {"timepoint_required": true, "sample_required": true}
}
```

```json
{
  "readout_id": "ro-cfu-1",
  "schema_id": "cfu_count",
  "schema_version": 1,
  "value": {"kind": "numeric", "number": 143.0},
  "unit": "CFU",
  "source": {"kind": "raw", "derived_from": []},
  "link": {
    "timepoint": "处理后 48 h",
    "sample_id": "S-3",
    "target": {"kind": "well", "plate_id": "plate-cfu", "coordinate": "B2"}
  }
}
```

计数是实测值，不是“183 个孔”或 CFU/mL 推算；稀释倍数与换算属于派生读数，需另给 `method` 与原始计数。

### 4.4 拒绝示例

```json
{
  "schema_id": "cck8_od450", "schema_version": 1, "endpoint": "od450",
  "value_kind": "plate",
  "unit": {"canonical": "AU", "required": true},
  "missing": {"allowed": [], "null_allowed": true},
  "source": {"raw": true, "derived": false},
  "link": {"timepoint_required": false, "sample_required": false}
}
```

→ `kind_spec_missing`：`plate` 必须声明 `plate: {format, rows, columns}`。

```json
{
  "schema_id": "rna_purity", "schema_version": 1,
  "value": {"kind": "numeric", "number": 2.06},
  "unit": "ratio",
  "source": {"kind": "derived", "derived_from": []}
}
```

→ `derived_without_method`、`derived_without_sources`：派生读数必须能复算。

```json
{
  "schema_id": "cfu_count", "schema_version": 1,
  "value": {"kind": "numeric"},
  "unit": "CFU",
  "source": {"kind": "raw", "derived_from": []}
}
```

→ `missing_without_reason`（该 schema `null_allowed = false`）：不能既无计数也无缺失原因。

```json
{"kind": "file", "asset_id": "asset-1", "media_type": "text/csv", "bytes": 2048}
```

→ `legacy_form_unsupported`：文件读数不能投影成 `readouts[]`，`readouts[]` 保持空。

## 5. 版本、并发、dry-run 与回滚

- **版本**：`schema_version` 只接受 `1`。破坏性改动升 v2 并新增 `schema_id`，不改本文；同一 `schema_id` 不得静默改语义。
- **dry-run**：写入前用 `validate_schema` → `validate_readout` → `to_legacy_readouts` 预览；三者都是纯函数，无 I/O，可在提交前给用户看投影结果。
- **并发**：本类型不持有状态，线程间只读共享；实际写入的幂等与事务由 `execution_event.create` 的 `idempotency_key` 与 Run 事务负责（本卡不实现，见 SDK-05）。
- **回滚**：类型与文档可整体回退，已存事件不受影响；本卡不产生数据迁移，因此没有数据回滚步骤。
- **兼容**：新增可选字段必须带默认值，旧记录仍可读；不承诺旧二进制读新数据。

## 6. 测试

`cargo test -p asterlab-core readout_schema`（12 个用例，全部纯内存）：

| 用例 | 证明 |
|---|---|
| `schema_round_trips_all_five_value_kinds` | 五类值都能序列化/反序列化并保持 kind |
| `cck8_plate_example_is_valid_and_maps_per_cell` | 孔板示例合法；每孔一条 legacy，缺失原因随孔 |
| `rna_purity_is_marked_derived_from_raw_readouts` | 派生读数带 method 与原始来源；来源解析只接受 raw |
| `cfu_count_keeps_unit_and_well_target` | 计数保留单位与孔位目标 |
| `null_and_explicit_missing_reason_stay_distinct` | null、明确原因、矛盾状态三态分开；`above_ulog` 反序列化失败 |
| `derived_readout_requires_method_and_sources` | 缺方法/缺来源/引用派生都被拒 |
| `schema_requires_the_matching_kind_spec` | 形状缺失与多余形状都被拒 |
| `standard_plate_format_fixes_its_dimensions` | 标准板型尺寸固定，`custom` 无固定尺寸 |
| `table_and_file_and_categorical_have_no_legacy_form` | 三类非标量读数不投影，返回 `legacy_form_unsupported` |
| `categorical_and_file_rules_are_enforced` | 类别与 media type/校验和规则 |
| `required_unit_timepoint_and_sample_are_reported` | 必填单位、时间点、样本 |
| `closed_structs_reject_unknown_fields` | 未知字段被拒绝 |

## 7. 明确不做

- 不实现 GUI、记录组件或板图渲染。
- 不改 `execution_event.create` 的输入/输出契约，不新增事件字段。
- 不新增 SDK kind、`spec/modules.json` 规则或 ASTERDOC 规则。
- 不建表、不迁移旧事件、不回填 `schema_id`。
- 不实现写入命令、批量提交或幂等（属 SDK-05 / RDO-01）。
- 不做单位换算、统计汇总或缺失值插补。

## 8. 未决项

1. **`above_ulog` 拼写**：本卡正文写作 `above_ulog`，`AGENT_API.md` 与旧 `readouts[].qualifier` 是 `above_uloq`。本契约按 `AGENT_API.md` 冻结为 `above_uloq`，未加别名；若要以卡面拼写为准，先改 `AGENT_API.md` 与本文再改代码。
2. **表格逐格原因**：`table` 单元格没有缺失原因字段；需要逐格原因时用 `plate` 或 `numeric` 读数。是否补 `table` 的 cell reason 留给 v2。
3. **文件读数关联**：`asset_id` 的归属与事件到资产的引用方式未冻结，属资产模块与 SDK-05。
4. **`readout_id` 生成规则**：现有 app 用 `readout-{operation}-{row}-{column}-{field}`；契约只规定 `per_cell` 投影后缀 `-<coordinate>`，正式生成规则待写入命令卡冻结。
5. **混合坐标系**：`plate` 是否允许一板多坐标系（如自定义板）未定；当前按单一 `rows × columns` 网格。
