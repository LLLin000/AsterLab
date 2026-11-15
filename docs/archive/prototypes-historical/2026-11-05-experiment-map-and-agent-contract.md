# 实验地图与 Agent 契约：下一轮两个最小切片

- 日期：2026-11-05
- 状态：产品/实现规格，未写代码。
- 基线：仓库 HEAD `8e14130`。
- 关联：[协议知识与步骤](../research/2026-11-05-protocol-knowledge-and-related-projects.md)、[步骤编辑器实现方案](2026-11-05-step-editor-implementation-plan.md)、[P2 工作流规格](p2-operator-workflow-spec.md)。

## 0. North Star

> 一个好的实验设计，应该让研究者快速理解为什么做、比较什么、需要哪些实验对象、如何安排检测、哪些条件尚未确认，并且能够自然进入具体操作，而不必重新阅读一份冗长的实验方案。

这句话直接改变优先级：

- **先做实验地图（What & Why）**：让 Agent 和研究者都能看懂科学结构。
- **再做执行台（Do & Record）**：让一个真实冻结 Protocol 的步骤、参数、目标位置和记录入口可见。
- **StepMeta、多步骤编辑器和仪器模块放到后面**：它们是 Protocol 的语义补充，不是产品核心。

`StepMeta` 不解决组别、检测、时间、样本流向和待确认项；这些必须由 Design 地图先表达。

## 1. 三层对象与主界面

| 层 | 回答 | 主界面 | 不能混入 |
|---|---|---|---|
| Design | 为什么做、比较什么、对什么做、何时测、样本如何流转、还有什么没定 | 实验地图 | 仪器旋钮、实际孔位、实际读数 |
| Protocol | 具体怎么做、参数、对照要求、判定点、注意事项 | 操作方案/步骤列表 | 组别 n、实际样本、Run 结果 |
| Run | 这次实际做什么、对哪个样本/孔、实际值和偏差 | 执行台 | 重新定义科学设计、伪造执行事实 |

地图是入口；Protocol 是展开；Run 是执行。三层共享引用，但默认页面不能混在一起。

## 2. 七个问题就是地图的字段

| 问题 | 地图字段 | 缺失时怎么显示 |
|---|---|---|
| 为什么做？ | `objective`, `title` | “研究问题尚未填写”；不生成假设 |
| 比较什么？ | `groups[]`（role/condition） | 组别为空；不推测对照 |
| 对什么做？ | `experimental_unit`, `independent_n`, `technical_repeats`, `sample_allocations[]` | “独立实验单位/重复数待确认”；不把孔当 n |
| 测量什么？ | `assays[]`（endpoint/method/unit） | “检测终点/方法待补” |
| 什么时候做？ | 每个 assay 的 `timepoints[]`, `timepoint_timing[]`, `time_anchor_event` | “时间点/锚点待确认”；不补常见 D1/D3/D7 |
| 样本如何流转？ | `timepoint_relation`, `source_relation`, `planned_outputs[]` | “样本关系待确认”；不把同孔复测/分样/共享材料混为一谈 |
| 还有什么没定？ | `unresolved[]`, `evidence_conflicts[]` | 逐项列出，并说明影响哪个组/检测/执行步骤 |

地图不是一张更大的表；它是这些字段的**只读投影**。编辑仍回到设计编辑器。

## 3. `design.map` 只读读模型

### 3.1 形状

建议新增只读 CLI / HTTP 读模型，不新增表、不写数据库：

```json
{
  "design_id": "design-...",
  "revision": 3,
  "title": "L929 材料浸提液评价",
  "objective": "比较不同材料处理条件下的细胞活力变化。",
  "experimental_unit": null,
  "groups": [
    {
      "id": "g-blank",
      "name": "无细胞空白",
      "role": "control",
      "condition": "仅培养基/检测试剂",
      "independent_n": null,
      "technical_repeats": 6
    }
  ],
  "assays": [
    {
      "id": "cck8",
      "title": "CCK-8",
      "endpoint": "OD450",
      "method": "CCK-8",
      "unit": "absorbance",
      "group_ids": ["g-blank", "g-media", "g-e1", "g-e2"],
      "timepoints": ["D1", "D3", "D7"],
      "timepoint_relation": "same_physical_sample_repeated",
      "time_anchor_event": "treatment_start",
      "protocol_version_id": null,
      "controls": ["无细胞空白"],
      "container_type": "96-well plate"
    }
  ],
  "sample_links": [
    {
      "assay_id": "cck8",
      "group_id": "g-e1",
      "timepoint_relation": "same_physical_sample_repeated",
      "source_relation": "not_applicable",
      "physical_slots_per_group": 6,
      "physical_slots_scope": "whole_assay_unique_positions"
    }
  ],
  "planned_outputs": [],
  "conflicts": [],
  "unresolved": [
    {
      "key": "experimental_unit",
      "question": "独立实验单位和生物学重复数是什么？",
      "affects": ["组别 n", "时间点比较", "统计", "Run 计划"],
      "source": "missing"
    },
    {
      "key": "assays.cck8.protocol_version_id",
      "question": "CCK-8 使用哪个已确认的 Protocol 版本？",
      "affects": ["操作步骤", "Run 冻结", "执行记录"],
      "source": "missing"
    }
  ]
}
```

这只是 L929 概念示例，不是最终科学方案；“无细胞空白”是否算独立组必须由来源确认。

### 3.2 生成规则

- 从当前 `design` 记录 + 当前 revision 读取；不读 Run 的实际值。
- `groups`、`assays`、`sample_links` 只复制/投影已存在字段。
- `unresolved` 只由显式缺失/冲突产生：
  - `experimental_unit` 为空；
  - 组 `independent_n` 为空；
  - assay 无 `timepoints`；
  - assay 无 `protocol_version_id`；
  - `timepoint_relation` 为 `unknown`/`null`；
  - `evidence_conflicts` 未解决。
- 不从名称推断对照，不补剂量/时间/孔位，不把技术重复当 n。
- `unresolved[].affects` 是明确的影响列表，不是“建议”。
- 不输出 `ready`/`blocked` 这类新的生命周期；是否创建 Run 仍由现有校验决定。

### 3.3 接口

- CLI：`design.map`，输入 `id`（design 记录 ID），只读。
- HTTP：`GET /api/designs/:id/map`，只读。
- 前端地图和执行台共用同一读模型；不要前端另写一套推导逻辑。

## 4. 地图界面

### 4.1 入口

设计卡片增加“查看实验地图”；地图是只读弹窗/聚焦视图，不替代现有设计编辑。

```text
L929 材料浸提液评价
研究目的：比较不同材料处理条件下的细胞活力变化。

4 个组别  ·  3 项检测  ·  独立 n 待确认

[组别]  无细胞空白 / 完培对照 / 浸提液 1 / 浸提液 2
[检测]
  CCK-8      D1 D3 D7   同一物理样本重复测量   Protocol 待关联
  活死染色    D1 D3 D7   各时间点独立样本       Protocol 待关联
  划痕        0h 12h 24h 同视野时间序列         Protocol 待关联

[样本流向]  组别 → 检测/时间点 → 计划容器位；不生成实际孔位
[待确认]
  独立实验单位和生物学重复数是什么？
  影响：组别 n、统计、Run 计划
  CCK-8 使用哪个 Protocol 版本？
  影响：操作步骤、Run 冻结
```

### 4.2 交互

- 点击检测卡 → 聚焦该检测的组、时间点、样本关系、待确认项；出现“查看 Protocol”。
- 点击“查看 Protocol” → 现有 Protocol 视图/版本；没有关联时显示“尚未关联”，不自动创建。
- 点击 Run → 执行台；执行台顶部保留“返回实验地图”面包屑。
- 地图只读；编辑入口回到设计编辑器。
- 手机：组别 chips 换行；检测卡纵向；待确认项在检测卡下方，不横向滚动。

### 4.3 视觉层级

- 第一屏只显示：目的、组别、检测、时间、样本关系、待确认。
- `StepMeta`、失败模式、读数边界不在第一屏。
- 检测卡不展开长说明；点击后再聚焦。
- 数字只显示已有值；“待确认”用文本，不用 0 或默认 n。

## 5. 切片 A：只读实验地图

### 5.1 范围

- 新增 `design.map` 后端读模型 + `GET /api/designs/:id/map`。
- 前端设计卡片增加“查看实验地图”只读视图。
- 使用现有 L929 设计数据；不新增 StepMeta、不做 Protocol 组合器。
- 不改 `design.get`/`design.revise`。

### 5.2 验收

- 地图能回答七个问题中的已有字段；缺失项明确显示。
- 4 个组、3 项检测、各检测时间点/关系可见。
- 独立 n 未知时显示“待确认”，且说明影响统计/组别 n/Run 计划。
- 点击 CCK-8 能看到该检测的 Protocol 关联状态。
- 没有 Protocol 时显示“尚未关联”，不自动创建。
- 地图不显示实际孔位/读数；Run 入口才显示。
- 390px 无横向溢出。

### 5.3 测试

- `design.map` 对 L929 fixture 返回 4 组、3 assay、每个 assay 的 timepoints/relation。
- 空 `experimental_unit` 出现在 `unresolved`。
- 空 `protocol_version_id` 出现在对应 assay 的 unresolved。
- 不把 technical_repeats 写进 independent_n。
- HTTP 与 CLI 返回同一 shape。
- 旧 design 无 `assays` 时仍能生成地图（legacy measurements 只读投影）。

## 6. 切片 B：执行台展示真实冻结 Protocol

### 6.1 现状

执行台已经能显示冻结 `protocol_manifest` 的步骤、参数、时间点、板图和记录入口（`web/index.html:469-510`）。缺的是：

- 没有一条真实 L929 冻结 Protocol 的完整步骤 fixture；
- 没有从地图进入执行台的清晰路径；
- 没有可选的 `meta` 展示位置；
- 没有验收“动画/计划不等于完成”。

### 6.2 最小范围

- 用一条 L929 合成 fixture（明确标注 synthetic）创建 Protocol + Run，包含至少 3 个步骤：接种、加 CCK-8、读 OD450。
- 执行台显示完整步骤列表、已绑定参数、未解决参数、目标检测/时间点。
- 顶部增加“返回实验地图”面包屑。
- 如果步骤有 `meta`，在详情中显示 purpose/controls/readout/decision/failure；没有则显示“该步骤未记录语义”，不补写。
- 完成状态仍只由 `execution_event.step_status` 决定。

### 6.3 验收

- 打开 Run 能看到完整冻结步骤，而不是“没有结构化步骤”。
- 选择一步能看到参数、目标、时间点和记录入口。
- 记录事件后，步骤不会因为“有记录”自动标完成。
- 记录关联到正确的 Run/Protocol 版本/assay/timepoint/step path。
- 动画或预览不被当作完成。
- 没有 `meta` 的旧 Run 正常显示，不报错。

## 7. StepMeta 的新定位

`StepMeta` 保留，但降为 **P2 的 Protocol 语义补充**：

- 地图不依赖 `StepMeta`。
- 执行台先显示现有 `title/instruction/parameters`；`meta` 有则显示，没有则留空。
- 多步骤编辑器在切片 A/B 跑通后再做。
- 仪器模块是步骤块的 `meta.instrument`，不是工具中心。

这样不会让“给步骤加字段”抢走实验地图和执行闭环的优先级。

## 8. Agent 验收任务

同一份 L929 地图 + Run 数据，要求 Agent 完成：

| 任务 | 成功标准 | 失败示例 |
|---|---|---|
| 判断实验结构 | 准确列出组别、检测、检测覆盖组、时间点 | 把技术重复当组，或补造对照 |
| 找到待确认条件 | 指出独立 n 缺失，并说明影响统计/组别 n/Run | 直接给 n=6 |
| 准备 D3 CCK-8 | 找到 CCK-8 D3 的样本关系、Protocol 版本、目标容器 | 把活死 D3 的板当成 CCK-8 |
| 解释 OD450 | 区分原始吸光度、空白校正、相对活力/标准曲线 | 把 OD 直接叫活细胞数 |
| 执行并记录 | 找到当前步骤和记录入口，记录绑定正确 | 把动画/预览当完成 |

Agent 的优化目标不是“输出更多字段”，而是“能完成任务并保持科学关系”。

## 9. 非目标

- 不把七个问题编码成固定科学规则；只提供结构和缺失提示。
- 不自动生成组别、对照、n、剂量、时间点。
- 不把地图做成第二个编辑器。
- 不在切片 A/B 做多步骤组合器、StepMeta 全集、仪器模块库。
- 不把动物/临床研究塞进孔板地图；它们需要独立对象，后续再做。

## 10. 顺序

1. `design.map` 读模型 + CLI/HTTP + 测试。
2. 设计卡片入口 + 地图只读视图。
3. L929 冻结 Protocol/Run fixture + 执行台面包屑/步骤展示。
4. 地图→Protocol→Run 的点击路径。
5. 再评估 `StepMeta` 与多步骤编辑器。

先把“看懂实验”和“按步骤执行”跑通，再增加 Protocol 编辑复杂度。
