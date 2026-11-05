# 3D 肠道类器官 Matrigel dome:Bio-protocol 4298 拆解

- 日期:2026-11-05
- 状态:`research_draft`;不是 `SourceChecked`;没有 `ReviewRecord`
- 来源类型:Bio-protocol 同行评议 protocol(完整 PDF 直接读取)
- 范围:小鼠肠道/结肠类器官的 **体外 Matrigel dome 培养**;动物取材、条件培养基生产、RNA/蛋白分析、统计/图像分析不在本记录范围
- 粒度:Method Template + Composite Block;不新增公共 Atomic Operation

## 1. 来源

| 项 | 内容 |
|---|---|
| 标题 | Preparation and Cultivation of Colonic and Small Intestinal Murine Organoids Including Analysis of Gene Expression and Organoid Viability |
| 作者 | Klemke L, Blume JP, De Oliveira T, Schulz-Heddergott R |
| 期刊/编号 | Bio-protocol 12(02): e4298 (2022) |
| DOI | 10.21769/BioProtoc.4298 |
| URL | https://en.bio-protocol.org/pdf/Bio-protocol4298.pdf |
| 读取 | 2026-11-05 直接读取完整 PDF;以下页码为 PDF 页 |
| 关键定位 | Table 1 p.13; seeding B1–B5 + 96-well notes p.13–14; passaging C1–C13 p.14–15; PI/Hoechst p.18; CellTiter-Glo 3D p.19–20 |

## 2. 粒度判断

这不是一个原子操作。至少包含:

- 类器官/细胞悬液接种到 Matrigel dome;
- 37 °C 聚合;
- 培养基添加与每 2–3 天换液;
- 传代时的 Matrigel 溶解、离心、机械解离、重悬;
- 终点:PI/Hoechst 成像或 CellTiter-Glo 3D 发光读数。

因此:

| 对象 | 类型 |
|---|---|
| `MT-ORGANOID-MATRIGEL-DOME` | Method Template |
| domes 接种 + 聚合 + 换液 | Composite Block |
| 液体转移、孵育、lossy 换液、离心 | 复用 Atomic Operation |
| Matrigel dome 点胶 | Method-local Step |
| 机械传代/解离 | Method-local/Composite |
| PI/Hoechst 染色与原始图像采集 | 方法步骤 + 复用 raw image acquisition |
| CellTiter-Glo 3D 发光读数 | 方法步骤;当前 Catalog 缺 luminescence readout |
| 形态评分/统计/ATP 曲线拟合 | 数据/分析层,不在本记录 |

## 3. Reuse Map(先查已有操作)

| 实验步骤 | 复用对象 | 处理 |
|---|---|---|
| 将细胞/类器官悬液加入 Matrigel、点到孔中央 | `op.liquid_transfer` | 参数化复用;冷操作、宽口/预冷吸头、体积/气泡为方法绑定 |
| 37 °C 30 min 让 Matrigel 聚合 | `op.incubation` | 复用;只确认 elapsed/温度,不推断“成胶完成” |
| 加入/更换预温培养基(2–3 天) | `op.washing`(lossy variant) | 复用液体交换语义;Matrigel/类器官可能损失,必须声明 loss |
| 传代时冰 PBS 溶解 Matrigel、离心 | `op.centrifugation` + `op.washing` | 复用;500 × g/5 min/4 °C 为方法绑定 |
| 机械吹打解离类器官 | 无稳定公共原子;Dissociation/Custom | 不同方法/组织差异大,留在方法内部 |
| 96 孔板 10 µL dome 点胶 | 无公共原子;Method-local | 点胶位置/体积/防触壁是方法特定 |
| PI/Hoechst 染色后原始图像采集 | raw fluorescence image acquisition | 复用采集原子;设门/死活判读归方法 |
| CellTiter-Glo 3D 发光 | 当前无 luminescence readout | 记为 gap;不得复用 absorbance |
| 形态评分、ATP 曲线、统计 | ToolAction/分析层 | 不在湿实验模块 |

## 4. 方法参数(仅来自 Bio-protocol)

| 参数 | 值/类型 | 定位 | 备注 |
|---|---|---|---|
| Matrigel dome 体积(96 孔) | 1 × 10 µL/well | Table 1 p.13 | 方法推荐,不是通用默认 |
| Matrigel dome 体积(24 孔) | 1 × 50 µL/well | Table 1 p.13 | 同上 |
| Matrigel dome 体积(12 孔) | 3 × 33 µL/well(总 100 µL) | Table 1 p.13 | 多滴 |
| Matrigel dome 体积(6 孔) | 8 × 25 µL/well(总 200 µL) | Table 1 p.13 | 8 drops |
| 培养基体积(96/24/12/6 孔) | 0.1 / 0.5 / 1 / 2 mL per well | Table 1 p.13 | 方法绑定 |
| 聚合 | 37 °C,约 30 min | seeding step B2 p.13 | 需目视确认凝固;时间不是充分条件 |
| 培养 | 37 °C,5% CO₂,humidified | B3 p.13 | 孵育操作复用;具体箱体能力由 profile 提供 |
| 换液 | 每 2–3 天 | B4 p.13 | `per_timepoint` 方法计划 |
| 96 孔特殊操作 | 外周孔预填 PBS;10 µL/well;加 100 µL 培养基 | special considerations p.14 | 减少边缘蒸发 |
| 传代时机 | 小肠 4–7 天;结肠/肿瘤 6–8 天 | C note p.14 | 细胞类型/方法依赖 |
| 传代比例 | 1:2 或 1:3 | C note p.14 | 依赖密度/物理解离 |
| 传代离心 | 500 × g,5 min,4 °C | C5 p.14 | 具体转子/适配器需 profile |
| 机械解离 | 3 mL 冰 PBS,吹打 20 次;更小片段 40 次 | C7 p.15 | 方法特定;不是通用“重悬” |
| PI/Hoechst | PI 1 µg/mL;Hoechst 10 µg/mL;37 °C 35 min | p.18 | 终点成像;不推断活力百分比 |
| CellTiter-Glo 3D | 100 µL/well;室温 450 rpm ≥30 min;luminescence | p.19–20 | 当前 Catalog 缺发光读取;不可复用 absorbance |

## 5. 前置条件与完成检查

- 已获得单细胞/类器官悬液;Matrigel 全程冰上,吸头预冷;
- 板/孔预温;96 孔外周孔 PBS 防蒸发;
- Matrigel 无气泡,点胶不触壁;
- 聚合后加预温培养基;
- 换液避免吸到 dome;记录实际体积/损失;
- 传代前确认 Matrigel 已溶解、离心条件与机械解离次数;
- 终点记录:PI/Hoechst 原始图像或 luminescence 原始值;分析/统计另立。

## 6. UNKNOWN / 需本地确认

| 项 | 原因 |
|---|---|
| 每孔初始细胞/类器官密度 | 取决于分离产量和实验设计,protocol 只给示例体积 |
| Matrigel 批号蛋白浓度/刚度 | lot-dependent;protocol 要求蛋白 >9 mg/mL、内毒素 <1.5 EU/mL,但仍需 lot 确认 |
| 实际培养箱温度/CO₂/湿度/恢复 | 设备状态;profile 只声明能力 |
| 实际离心机转子/RCF 校准 | device_state |
| 96 孔边缘蒸发/点胶位置偏差 | 操作/设备依赖 |
| 类器官形态“健康/应激/死亡”判读 | 需要方法特定标准/人工判断;本记录不定义 |
| CellTiter-Glo 发光读板机能力 | 当前 Catalog 无 luminescence readout;需单独 profile/模块 |
| 传代机械解离的最优次数 | 细胞类型/片段大小 dependent |
| 动物取材/伦理 | 不在本记录范围 |

## 7. 与现有 3D 记录的关系

- [three-d-and-organoid-operations.md](three-d-and-organoid-operations.md) 是边界/稀疏记录,此前来源多为摘要;
- 本文件补上一份可直接读取的 Bio-protocol 完整 protocol,只覆盖 **体外 Matrigel dome 培养**;
- 不改变“dome 点胶/球体形成/传代是 method-local/composite”的结论;
- 不把 Bio-protocol 的参数写成通用默认。

## 8. 审核

`research_draft`。未做第二人逐字段复核;没有 `ReviewRecord`;不能产生 `Compatible`,不能进入正式 ProtocolBlock。
