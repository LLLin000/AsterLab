# 基础细胞培养:Morimoto Lab protocol 拆解

- 日期:2026-11-05
- 状态:`research_draft`;不是 `SourceChecked`;没有 `ReviewRecord`
- 来源类型:大学实验室公开 protocol(Northwestern University, Morimoto Lab);不是同行评议期刊
- 范围:基础哺乳动物细胞培养:传代、血球计数、台盼蓝活力、冻存/复苏
- 粒度:Composite Block / Method Template / 一个读数原子;不新增公共 Atomic Operation
- 完整性审计:见 [protocol-discovery/batch-01-morimoto-cell-culture.md](../protocol-discovery/batch-01-morimoto-cell-culture.md)
- 审计修正:血球计数原文的两套边界规则自相矛盾,不能当成唯一规则;台盼蓝 0.8 mM 与 Thermo 0.4% 是不同来源条件,不合并;活力公式原文未给;thaw 尚无独立模块。

## 1. 来源

| # | Protocol | URL | 读取/定位 |
|---|---|---|---|
| M1 | Cell Splitting Protocol | https://www.morimotolab.org/_files/ugd/594a6c_20c8434bdc634367b03e8438b1c27916.pdf | 2026-11-05 直接读取,1 页,步骤 1–11 |
| M2 | Cell Counting Using a Hemacytometer | https://www.morimotolab.org/_files/ugd/594a6c_fe7b278f85e147be88ffc8b5c7752dd4.pdf | 直接读取,3 页,计数规则/公式/步骤 1–7 |
| M3 | Trypan Blue Staining | https://www.morimotolab.org/_files/ugd/594a6c_10d20d9fc4d54187bbf74c3864be1167.pdf | 直接读取,1 页,材料/步骤/comment |
| M4 | Freezing/Thawing of Tissue Culture Cells | https://www.morimotolab.org/_files/ugd/594a6c_792f446515a141fb9e54e210ff8be8c0.pdf | 直接读取,2 页,Freezing 1–7 / Thawing 1–6 |

来源定位以 PDF 页内步骤号为准;M1 页脚 II.A.13,M2 II.A.14–16,M3 II.A.18,M4 II.A.19–20。

## 2. 粒度判断

| 对象 | 类型 | 理由 |
|---|---|---|
| `MT-CELL-SPLITTING-100MM` | Composite Block | 弃液/洗涤/解离/孵育/加培养基/分装/再孵育 |
| `OP-CELL-COUNT-HEMACYTOMETER` | Atomic readout | 一个计数动作;输入悬液,输出 cells/mL 原始计数 |
| `MT-TRYPAN-BLUE-VIABILITY` | Method Template | 染色 + 计数 + 活力判读;不是通用原子 |
| `MT-CELL-FREEZE` / `MT-CELL-THAW` | Composite Block | 冷冻保护剂、降温、入库/取出、离心/换液 |
| `liquid_transfer` / `incubation` / `centrifugation` | 复用 Atomic Operation | 见 Reuse Map |

## 3. Reuse Map

| 步骤 | 复用对象 | 处理 |
|---|---|---|
| 弃旧培养基、PBS 洗涤、加/弃液体 | `op.liquid_transfer` / `op.washing` | 参数化复用;体积/次数方法绑定 |
| 37 °C CO₂ 孵育(解离约 5 min;培养) | `op.incubation` | 复用;只确认 elapsed/条件,不推断“已解离” |
| 离心细胞(计数/冻存/复苏) | `op.centrifugation` | 复用;本 protocol 用 rpm(2000 rpm),RCF 依赖转子,UNKNOWN |
| 血球计数 | `OP-CELL-COUNT-HEMACYTOMETER` | 读数原子;本文件新建为原子候选 |
| 台盼蓝活力 | `MT-TRYPAN-BLUE-VIABILITY` | 方法模板;活力判读不是通用计数 |
| 冻存/复苏 | `MT-CELL-FREEZE` / `MT-CELL-THAW` | 复合块;DMSO/血清/降温/离心方法绑定 |
| 分装到新板 | `op.liquid_transfer` | 参数化复用;分装体积按 split ratio 表 |

## 4. Record A — `MT-CELL-SPLITTING-100MM`

### 4.1 步骤与参数(M1)

| 参数/步骤 | 值 | 来源 |
|---|---|---|
| 容器 | 100 mm TC plate | M1 step 1 |
| 显微镜判断 split ratio | 先看细胞状态决定比例 | M1 step 1 |
| PBS 洗涤 | 2 mL 1×PBS,轻摇,吸弃 | M1 steps 3–4 |
| 解离 | 2 mL 1×Trypsin,轻摇,37 °C CO₂ 约 5 min | M1 steps 5–6 |
| 确认解离 | 从孵箱取出后确认细胞脱落 | M1 step 8 |
| 中和/收集 | 加 8 mL 培养基,轻摇,吹下细胞 | M1 step 9 |
| 新板培养基 | 先在新板加新鲜培养基 | M1 step 7 |
| split ratio | 1:10 → 1 mL cells + 9 mL media;1:5 → 2 + 8;1:2 → 5 + 5 | M1 table step 8 |
| 分装/再培养 | 加对应细胞体积,轻摇,37 °C CO₂ | M1 steps 10–11 |

### 4.2 完成检查

- 细胞已从原板脱落;
- 新板体积符合所选 split ratio;
- 记录实际 split ratio、细胞体积、培养基体积、时间;
- 不把“5 min”当作所有细胞系的解离终点;需镜下确认。

### 4.3 UNKNOWN

- 细胞系特异性解离时间;
- 实际培养箱温度/CO₂/湿度;
- 实际培养基配方(见 M1 之外);
- 传代后细胞数/密度(需要计数,不由 split ratio 推断)。

## 5. Record B — `OP-CELL-COUNT-HEMACYTOMETER`

### 5.1 计数规则(M2)

| 项 | 值 | 来源 |
|---|---|---|
| 计数室深度 | 0.1 mm | M2 p.1 |
| 大正方形面积 | 1 mm²;体积 0.1 mm³ = 10⁻⁴ mL | M2 p.1 |
| 观察倍率 | 40× 可见整个 Neubauer grid(4× objective) | M2 p.1 |
| 计数目标 | 总计约 100 cells;必要时 300–400 cells 降低误差 | M2 pp.1–2 |
| 每格目标 | 20–50 cells/square(约 2–5×10⁵ cells/mL) | M2 p.2 |
| 计数约定 | 数上/左线,不数下/右;重叠线按上/右为 in | M2 p.2 |
| 单细胞比例 | ≥90% 细胞游离,否则重取样本 | M2 p.2 |
| 公式 | cells/mL = 平均每格计数 × 稀释倍数 × 10⁴ | M2 pp.1–2 |
| 示例 | 5 大格共 125 cells → 250,000 cells/mL | M2 p.2 |
| 预处理示例 | 2 mL trypsin,37 °C ~5 min;8 mL media;2000 rpm 2 min;按 pellet 大小留 2–4 mL | M2 pp.2–3 |

### 5.2 完成检查

- 计数格内细胞分布均匀、无大量团块;
- 记录格数、总计数、稀释倍数、体积;
- 原始计数与计算值分开;不把 count 直接当 viability。

### 5.3 UNKNOWN

- 实际血球计数板型号/网格;
- 2000 rpm 对应 RCF(转子依赖);
- 细胞系/样本适用的稀释倍数;
- 计数人员间差异。

## 6. Record C — `MT-TRYPAN-BLUE-VIABILITY`

| 项 | 值 | 来源 |
|---|---|---|
| 染料 | Trypan Blue 0.8 mM in PBS | M3 Materials |
| 混合 | 细胞悬液 1:1 与台盼蓝 | M3 step 2 |
| 计数 | 血球计数板;活细胞排斥染料,死细胞蓝染 | M3 step 3 |
| 时间限制 | 不超过 30 min;超过会增加假死细胞 | M3 Comments |
| 局限 | 不能区分 necrotic vs apoptotic | M3 first line |

完成检查:染色后 30 min 内完成计数;记录稀释、时间、活/死计数。活力百分比是派生计算,不是原始计数。

## 7. Record D — `MT-CELL-FREEZE` / `MT-CELL-THAW`

### 7.1 Freeze(M4)

| 项 | 值 | 来源 |
|---|---|---|
| 冻存细胞状态 | 密度较高但未完全汇合、生长良好;只用近期复苏细胞补充库存 | M4 Freezing 1–2 |
| 离心 | 临床离心机 setting 3(~2000 rpm)5 min | M4 step 3 |
| 冻存液 | 原培养基 + 20% serum + 10% DMSO,预温 | M4 step 4 |
| 体积 | 0.6–0.8 mL/瓶 | M4 step 4 |
| 降温 | 约 1 °C/min;冻存架位置在 3–4 之间,液氮液位影响 | M4 steps 6 |
| 入库 | 过夜后转干冰,保持低温,按物种/细胞类型归档 | M4 step 7 |

### 7.2 Thaw(M4)

| 项 | 值 | 来源 |
|---|---|---|
| 选择 | 优先最近冻存的 vial;库存不能只剩一管 | M4 Thawing 1–2 |
| 复苏 | 37 °C 水浴快速摇动,酒精擦拭 | M4 step 3 |
| 悬浮细胞 | 加到 10 mL 预温培养基,2000 rpm 5 min,去上清,重悬 10 mL | M4 step 4 |
| 贴壁细胞 | 加到 10 cm 板含 10 mL 预温培养基,贴壁数小时/过夜后换液 | M4 step 5 |
| 再冻存 | 用第一次 split 的细胞 | M4 step 6 |

### 7.3 UNKNOWN

- 细胞系特异性冻存密度/活率;
- 实际降温速率/冻存盒型号;
- DMSO 批次/毒性;
- 液氮罐温度/库存记录。

## 8. 与现有记录的关系

- 复用 [cell-culture-and-separation.md](cell-culture-and-separation.md) 中的传代/计数/离心/冻存结构;
- 本文件提供来自公开 lab protocol 的逐步参数,用于交叉核对,不把 M1–M4 的值写成通用默认;
- 不新增公共 OperationModule;`OP-CELL-COUNT-HEMACYTOMETER` 是读数原子候选,进入 Catalog 前仍需第二人复核和跨方法复用证据。

## 9. 审核

`research_draft`;没有 `ReviewRecord`;不产生 `Compatible`;不进入 JSON/正式 ProtocolBlock。
