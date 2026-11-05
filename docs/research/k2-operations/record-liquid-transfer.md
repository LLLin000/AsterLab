# 样张记录:液体转移 `op.liquid_transfer` v1

- 记录类型:`OperationModule` 草稿
- 状态:`research_draft`;没有 `review_record`,不能产生 `Compatible`
- 用途:先单独列出一条完整记录,供人工判断结构是否合适;其他操作/仪器记录不变
- 机器可读版本:`catalog/operations/liquid-transfer.v1.json`

## 1. 身份与范围

| 字段 | 内容 |
|---|---|
| Catalog ID | `op.liquid_transfer` |
| Version | 1 |
| 标题 | Liquid transfer with a manual pipette |
| 动作类型 | `liquid_transfer` |
| 参考实现 | 手动活塞式移液器 + 一次性吸头 |
| 适用 | 液体样本从来源容器转移到目标容器;管、板、瓶 |
| 不适用 | 自动液体处理(只作为映射,不混入默认参数) |
| 操作粒度 | 一次转移 = 吸液 → 放液;多孔循环是组合,不是新操作 |
| 审核状态 | `research_draft` |

## 2. 来源

| 来源 | 位置 | 用途 |
|---|---|---|
| Opentrons Python API: Liquid Control | `https://docs.opentrons.com/python-api/building-block-commands/liquids/`;Aspirate / Dispense / Blow out / Mix / Air gap 段 | 动作、混匀、吹出、气隙、端口概念 |
| Opentrons Complex Command Parameters | `https://docs.opentrons.com/python-api/complex-commands/parameters/`;Tip handling / Mix before / Disposal volume / Touch tip / Air gap 表 | 吸头策略、混匀、处置体积、触碰 |
| Eppendorf Research plus manual | doc 3120 900.012-06/042013;§§1.3–1.4, 2.2, 3.4, 5.5, 6, 7 | 手动移液器量程、吸头、单次使用、校准/调整 |
| ATCC Subculturing Cell Line Monolayers | `https://www.atcc.org/resources/technical-documents/guide-to-subculturing-cell-line-monolayers`;harvest steps 1–5 | 示例绑定,不是操作默认 |
| ISO 8655-6 | 通过 Eppendorf 手册引用;未取得标准全文 | 只用来说明校准基准;具体条件 UNKNOWN |

## 3. 端口

| Port | 方向 | 类型/角色 | 必填 | 说明 |
|---|---|---|---|---|
| `source` | in | sample / `source_sample` | 是 | 被吸取的液体样本 |
| `destination` | in | container / `destination_container` | 是 | 预先准备的接收容器 |
| `tip` | in | consumable / `consumable` | 是 | 一次性吸头 |
| `source_remaining` | out | sample / `source_sample` | 否 | `partial` 分支的剩余来源;其他分支不应视为实际输出 |
| `transferred` | out | sample / `destination_sample` | 是 | 实际进入目标容器的材料 |

## 4. 样本转换

`transfer_mode` 必须在使用时绑定;下面的 transition 是条件分支,不是同时发生。

| Transition | 条件 `transfer_mode` | 输入 | 输出 | Effect | 含义 |
|---|---|---|---|---|---|
| `partial_source_remaining` | `partial` | `source` | `source_remaining` | `preserved` | 来源保留身份,剩余体积减少 |
| `partial_transferred` | `partial` | `source` | `transferred` | `aliquoted` | 产生新的样本部分;不推断浓度变化 |
| `full_transferred` | `full` | `source` | `transferred` | `moved` | 样本身份随全部体积转移到新容器 |
| `addition_to_existing` | `add_to_existing` | `source` | `transferred` | `state_changed` | 加入已有目标内容;目标状态改变 |

注意:`source_remaining` 只对 `partial` 分支有意义;其他分支不应把它当成实际输出。`applies_when` 目前只是契约条件,执行器/编译器仍需实现选择逻辑。

## 5. 参数

| 参数 | 类型/单位 | 必填阶段 | 来源分类 | 约束 | 证据 |
|---|---|---|---|---|---|
| `transfer_mode` | text | `run_freeze` | `design_variable` | `partial` / `add_to_existing` / `full`;无默认 | 操作关系;不是仪器能力 |
| `volume` | number / µL | `run_freeze` | `design_variable` | 正数;实际范围由所选移液器/吸头/液体决定 | Opentrons 参数;Eppendorf 家族跨度只用于说明,不作为通用约束 |
| `delivered_volume`(记录字段) | number / µL | run 记录 | run actual | 必须记录实际值,不能把计划值当实际值 | K1 record field |
| `tip_id`(记录字段) | text | run 记录 | run actual | 可选;用于追溯吸头/批次 | K1 record field |

说明:

- 通用操作不写家族量程;具体 profile 只声明自己的量程。
- 记录没有“默认体积”“默认吸头”“默认混匀次数”“默认转移模式”。
- 目标容器类型、液体性质、温度、粘度等仍未类型化,见 UNKNOWN。

## 6. 前置条件

1. 已安装干净、兼容、一次性的吸头。
2. 请求体积在实际移液器/吸头量程内。
3. 来源液体量足够;目标容器有足够余量。
4. 移液器已按液体类型校准/调整;非水溶液不能直接沿用水的工厂校准。
5. 需要时完成预湿、混匀、吹出和气隙设置;这些是本次绑定,不是模块默认。

## 7. 完成检查

| 检查 | 类型 | 阻断? | 说明 |
|---|---|---|---|
| `tip_attached` | operational | 是 | 吸液前确认吸头已安装 |
| `volume_delivered` | operational | 否 | 记录实际体积;超出允差则标记偏差 |

## 8. 仪器能力要求

| 字段 | 内容 |
|---|---|
| 类别 | `pipette` |
| 必需能力 | `volume_range` 存在 |
| 实际量程 | 由具体 instrument profile / 绑定检查;本记录不假定家族范围 |
| 型号 | 未知;不绑定 Eppendorf/Opentrons 具体型号 |
| 校准 | 不适用静态记录;由 Run/设备状态提供 |

## 9. 失败模式与判定

| 失败 | 现象 | 影响 | 处理 |
|---|---|---|---|
| 气溶胶/交叉污染 | 后续转移出现意外污染 | 下游培养/检测无效 | 更换吸头、重复受影响转移、记录偏差 |
| 体积错误 | 实际体积超出允差 | 浓度/剂量错误 | 检查吸头、液体类别、预湿、校准;重复 |
| 吸头不兼容 | 漏液、挂滴、体积不稳 | 体积/污染风险 | 更换匹配吸头;记录批次 |

判定点:

- 请求体积是否在实际移液器/吸头范围内?
- 液体是否明显不同于水?若是,是否需要预湿/校准调整?
- 是否使用了新吸头?

## 10. 明确 UNKNOWN

- 实际移液器型号、序列号、通道数、校准日期。
- 实际吸头批次/低吸附需求。
- 液体密度、粘度、表面张力、温度。
- 自动液体处理的 labware/deck 映射。
- 实验室 SOP 是否覆盖厂商误差限。
- 是否需要正位移/重复分液等特殊移液器。

## 11. 这条记录刻意不做什么

- 不给默认体积、吸头、混匀次数或吹出参数。
- 不把“0.1 µL–10 mL 家族跨度”写成单台仪器能力。
- 不把体积变化自动解释成浓度或剂量变化。
- 不把计划转移写成实际执行。
- 不从 Opentrons 的自动化命令反推手动实验室 SOP。
- 不因为结构完整就升级为 `SourceChecked`。

## 12. 审核门

这条记录目前只能用于:

- 结构审查;
- 来源/字段对照;
- 后续 JSON 草稿的模板。

不能用于:

- 生成 `Compatible` 仪器结论;
- 自动填 Run 实际值;
- 编译为正式 Protocol;
- 宣称是实验室 SOP。
