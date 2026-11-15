# Asset Coordinates v1（资产坐标与读模型字段契约）

- 日期：2026-11-15
- 状态：v1 契约；只增字段，不改旧链接语义。
- 代码：`crates/asterlab-core/src/assets.rs`（`AssetLink`）、`crates/asterlab-core/src/run_workspace.rs`（`LinkedAssetView`、`AssetCoverageGroup`）、`src/cli.rs`（`asset.link`、`asset.link.update`、`asset.unlink`、`run.workspace`）。
- 关联：[UI-REDESIGN](../sdk/huvec-tube-formation-case/UI-REDESIGN.md) §2.2、[AGENT_API](../../AGENT_API.md)、[CONTEXT](../../CONTEXT.md) 读模型与“载体 / 孔位”。

## 0. 目标与边界

一张显微图要能按 **孔位 × 时间点 × 视野序号** 精确定位，并在 12 孔板上做缺漏检查。本契约固定三件事：

1. `asset_link` 的链接坐标字段（`wellId` / `timepoint` / `fovIndex`）的语义与校验；
2. `run.workspace` 读模型如何带出这些字段（`linked_assets`），以及缺漏检查的分组依据（`asset_coverage`）；
3. 旧链接的缺省规则：**缺失就是缺失，不补默认值、不推断**。

范围边界：

- 只增字段。`{runId, taskKey, sampleId}` 的身份与语义不变；`asset.link` 的输入/输出只加可选字段。
- 不在 Core 里计算“应为几张图”。计划视野数来自冻结设计与 `ReadoutSchema`/`RecordProjection` 的 `ImageSlotMatrix`；本契约只提供分组依据。
- 不做 UI。Go 侧在 LB-08 只镜像类型字段与 JSON tag；LB-10 起 `RunService` 也暴露解绑与坐标更新（§6.1），实现仍是 CLI 的同一份 Core 代码。
- 不改冻结 Run 的历史字段语义；旧链接回读保持空值。

## 1. 字段

`asset_link` 记录里的 `link` 对象（与 `AssetLink` 结构一致，camelCase）：

| JSON | Rust | 类型 | 语义 |
|---|---|---|---|
| `runId` | `run_id` | `String` | 链接所属执行批次；必填，且必须等于记录 `parent_id`。 |
| `taskKey` | `task_key` | `String` | 冻结 Run 计划任务的 `task_key`；必填，必须属于该 Run 的 `tasks[]`。 |
| `sampleId` | `sample_id` | `Option<String>` | 若该图只对某个样本，记录其 ID；不写表示“链接没有声明样本”，不是“样本未知编号”。 |
| `wellId` | `well_id` | `Option<String>` | 载体孔位坐标，如 `A1`。单独 `A1` 已经属于该 Run 的板图；板 ID 由 Run 的 `plates[]` 提供，不在链接里重复。 |
| `timepoint` | `timepoint` | `Option<String>` | 采图时间点，必须与冻结设计里的时间点文字一致（如 `t6h`）；不做单位换算。 |
| `fovIndex` | `fov_index` | `Option<u32>` | 视野序号，从 **1** 开始；`0` 不是合法值。 |
| `coordinateId` | 派生 | `Option<String>` | 由 `wellId + timepoint + fovIndex` 生成的稳定定位串，供 Run/Record 视图之间跳转。不单独写入身份。 |

`coordinateId` 的生成规则（Core 唯一实现 `AssetLink::coordinate_id`）：

```text
无 wellId                                  -> null
wellId 大写                                -> "A1"
+ timepoint（非空）                        -> "A1_t6h"
+ fovIndex（存在）                         -> "A1_t6h_fov01"
```

- `fovIndex` 补零到两位：`1` → `fov01`。
- `timepoint` 原样拼接（不做大写化），只要求非空。
- 派生失败一律 `null`，不生成 `A1_unknown_fov00` 这类占位串。

## 2. 缺省规则（旧链接兼容）

- 旧记录没有 `wellId` / `timepoint` / `fovIndex` 时，回读为 `null` / 空值；**不按任务名、样本 ID、文件名或板图的唯一孔去推断**。
- `null`、空字符串与 `0` 都表示“未记录”，三者不互相等价于某个具体孔位/视野；`coordinateId` 也只在这些字段真实存在时才派生。
- 不迁移旧链接、不回填、不改写 `updated_at`。
- 新记录若给出 `timepoint` 或 `fovIndex`，必须同时给出 `wellId`；否则 `INVALID_INPUT`（退出码 2），不静默丢字段。

## 3. 写入校验（`asset.link`）

| 字段 | 规则 | 失败码 |
|---|---|---|
| `wellId` | 匹配 `^[A-Za-z]{1,2}[0-9]{1,2}$`；最长 40 字符；不含控制字符。 | `INVALID_INPUT` |
| `timepoint` | 最长 100 字符；不含控制字符。 | `INVALID_INPUT` |
| `fovIndex` | `>= 1`。 | `INVALID_INPUT` |
| `timepoint` / `fovIndex` | 只有 `wellId` 存在时才允许。 | `INVALID_INPUT` |

`asset.link` 仍按 `(rootId, relativePath, runId, taskKey, sampleId, coordinateId)` 计算记录 ID：坐标不同是不同链接，重复关联同一坐标只刷新原记录并追加 `linked` 事件。

## 4. 读模型（`run.workspace`）

`run.workspace` 在原有 `scopes/steps/tasks/occurrences/materials/plates/events` 之外新增两个字段（snake_case，与同层其他字段一致）：

```json
{
  "linked_assets": [
    {
      "asset_id": "asset-1",
      "task_key": "{\"scope\":{\"kind\":\"assay\",\"assay_id\":\"cck8\"},\"path\":\"root/read\",\"timepoint\":\"t6h\"}",
      "sample_id": "sample-1",
      "well_id": "A1",
      "timepoint": "t6h",
      "fov_index": 1,
      "coordinate_id": "A1_t6h_fov01"
    }
  ],
  "asset_coverage": [
    {
      "well_id": "A1",
      "timepoint": "t6h",
      "fov_indices": [1, 2],
      "asset_ids": ["asset-1", "asset-2"]
    }
  ]
}
```

地面规则：

- `linked_assets` 覆盖该 Run 当前有效的 `asset_link` 记录（`unlinked` 后不再出现），顺序为存储顺序（`created_at` 倒序），不构成语义排序。
- 坐标字段逐项复制记录值，`null` 保持 `null`；`coordinate_id` 由 Core 按第 1 节规则重新派生，保证与 `asset.link` 一致。
- `asset_coverage` 是缺漏检查的分组依据，不是缺漏结论：
  - 分组键 = `(well_id, timepoint)`；没有 `well_id` 的链接**不进入分组**（它仍在 `linked_assets` 里可见）。
  - 组按 `well_id` 升序、`timepoint` 升序（相同孔内无时间点的组排在最前）。
  - `fov_indices` 为该组已记录视野去重升序；`asset_ids` 为该组资产 ID 去重升序。
  - Core 不补“应有几张”，也不生成空组；前端/Record 用冻结计划与 `fov_indices` 求差集。
- 只读：`run.workspace` 不写数据库，不改链接。

## 5. 坐标更新（`asset.link.update`）

`asset.link.update` 只改一条已存在链接的坐标三元组：不解绑、不重新计算记录 ID、不改文件身份。
它让“未标注”的资产在不重新读取文件的情况下补上孔位/时间点/视野，或把标错的坐标就地改正。

输入（坐标三个键必须出现；`timepoint` / `fov_index` 可用显式 `null` 清空）：

| 字段 | 规则 | 失败码 |
|---|---|---|
| `id` | 已存在的 `asset_link` 记录 ID；未知或已解绑（记录已删除）都是 `NOT_FOUND` | `NOT_FOUND` |
| `id` | 指向其他 kind（如 `run`） | `INVALID_INPUT` |
| `well_id` | 同第 3 节孔位规则；必填，本命令不能把链接退回“无坐标” | `INVALID_INPUT` |
| `timepoint` | 同第 3 节；显式 `null` 表示该链接不记录时间点 | `INVALID_INPUT` |
| `fov_index` | 同第 3 节，从 `1` 开始；显式 `null` 表示不记录视野 | `INVALID_INPUT` |
| 缺少 `well_id` / `timepoint` / `fov_index` 键 | 拒绝；缺失不是“清空”的同义词 | `INVALID_INPUT` |
| `actor_id` | 最长 200 字符、无控制字符；省略时沿用记录上的 `actorId` | `INVALID_INPUT` |

写入规则：

- 只重写 `link.wellId`、`link.timepoint`、`link.fovIndex` 和重新派生的 `link.coordinateId`。
  `data.id`（文件身份）、`rootId`、`relativePath`、`size`、`modTime`、`sha256`、`mediaType`、
  `availability`、`linkedAt` 以及 `runId`、`taskKey`、`sampleId` 原样保留。
- 一次调用写两件事：`asset_link` 记录的新坐标与一条 `asset_link_event`（`action = "updated"`）。
  两者在同一事务内提交；校验失败或任一步失败都不留半态。
- `updated` 事件额外带 `previous: {wellId, timepoint, fovIndex, coordinateId}`，即被替换前的坐标；
  历史 `linked` / `unlinked` 事件不被改写。
- 事件载荷的其余字段（`assetId`、`rootId`、`relativePath`、`sha256` …）复制记录值，`link` 是更新后的坐标。
- 输出 `{record, event}`，与 `asset.link` 同形状；`record` 是更新后的 `asset_link` 记录。

两条必须知道的边界：

- **记录 ID 是创建时的定位句柄**，由 `asset.link` 的
  `(rootId, relativePath, runId, taskKey, sampleId, coordinateId)` 派生，更新坐标不重算。
  因此修改坐标后再次 `asset.link` 同一文件与新坐标会新增一条记录（ID 由坐标派生）；
  移动坐标请用 `asset.link.update`，不要重新关联。
- 提交与当前完全相同的坐标不是错误：仍写一条 `updated` 事件，`previous` 与当前值相同。
  这保证事件流严格按调用顺序可审计。

## 6. Go 侧镜像

`prototypes/mygo-run/modules/runs` 的 `AssetLink` 增加同名字段与 JSON tag：

```go
type AssetLink struct {
    RunID     string `json:"runId"`
    TaskKey   string `json:"taskKey"`
    SampleID  string `json:"sampleId,omitempty"`
    WellID    string `json:"wellId,omitempty"`
    Timepoint string `json:"timepoint,omitempty"`
    FOVIndex  uint32 `json:"fovIndex,omitempty"`
}
```

- `FOVIndex == 0` 表示未记录（Core 不会写入 0），与 JSON `null`/缺省一致。
- 桥接层只是透传 Core 的 JSON；缺字段的旧记录解码后是该类型的零值，不推断。

### 6.1 Go 服务方法（LB-10）

`RunService` 在 `ListAssetLinks` 之后增加两个方法，语义直接复用 CLI 的
`asset.unlink` / `asset.link.update`（同一份 `cli_impl` 实现，不另写 SQL），
差别只在身份参数：

```go
UnlinkAssetLink(ctx context.Context, runID string, assetID string) (Asset, error)
UpdateAssetLinkCoordinates(ctx context.Context, runID string, assetID string, wellID string, timepoint string, fovIndex uint32) (Asset, error)
```

- `assetID` 是 `Asset.ID`（文件的持久身份，`ListAssetLinks` / `SaveAssetLink` 返回的那个），
  在 `runID` 内解析成 `asset_link` 记录 ID。记录 ID 由 Core 从
  `(rootId, relativePath, runId, taskKey, sampleId, coordinateId)` 派生，桥接层从存储读取，
  **不重算、不暴露给调用方**。
- 零值映射：Go 没有“键缺失”状态，`timepoint == ""` 与 `fovIndex == 0` 是第 2 节的
  “未记录”，桥接层始终显式传键，写成 `null`。`wellID` 必填。
- 错误：该 Run 没有这个资产的链接（含已解绑）→ `NOT_FOUND`；坐标非法 →
  `INVALID_INPUT`。Go 侧形态是 `NOT_FOUND: …` / `INVALID: …` / `INTERNAL: …` 字符串，
  与桥接层其他命令一致。
- 同一资产在同一 Run 内有多个坐标链接时（只有 CLI 的 `asset.link` 带坐标能造出这种状态，
  `SaveAssetLink` 不接受坐标），按资产 ID 定位有歧义：桥接层返回 `INVALID_INPUT`，
  不猜要改哪一条，也不批量删除。
- 返回值都是 `Asset`：更新返回新坐标的链接，解绑返回被删除链接的原始元数据。
  `unlinked` / `updated` 事件与 CLI 完全一致（§5），文件本身从未被修改。
- 生成文件 `runs_native.go` 与 `rust/src/mygo.rs` 由固定 MyGo pin 重新生成，
  `scripts/ci/native_generator_hashes.json` 同步 pin；两个生成文件都不手改。

## 7. 版本与回滚

- 全部字段可选，缺少时读模型输出 `null`，因此不需要新的 schema 版本号；旧消费者忽略新字段即可。
- `asset.link.update` 是新增命令，不改变 `asset.link` 的输入/输出；`asset_link_event.action` 增加 `updated`，事件多一个 `previous` 对象。按 Run 查事件时可按 `updated` 过滤，旧调用方按 `linked` / `unlinked` 过滤不受影响。
- 去掉 `linked_assets`/`asset_coverage` 或把缺失坐标补成默认值属于破坏性变更，必须新卡并按契约评审。
- 回滚方式：撤回读模型两个字段；`asset_link` 里已经写入的坐标字段保留但不再被读模型带出（不迁移、不改写）。撤回 `asset.link.update` 时已写事件保留，坐标保持最后写入值。

## 8. 非目标

- 不做孔板 UI、拖拽填槽、批量导入（Windows 泳道）。
- 不判断“162 张是否拍全”，不定义每孔应有视野数。
- 不引入新的记录类型或第二份资产表；坐标仍只存在 `asset_link` 记录里。
- 不做 Go 原生桥接之外的新写入路径：`UnlinkAssetLink` / `UpdateAssetLinkCoordinates` 复用 `asset.unlink` / `asset.link.update` 的实现，CLI 与桥接不各写一套。
