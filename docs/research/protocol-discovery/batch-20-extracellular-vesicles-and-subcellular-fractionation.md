# Batch 20 — Extracellular Vesicles: Differential Ultracentrifugation / Size-Exclusion Chromatography / Precipitation; Subcellular Fractionation

- 日期: 2026-11-05
- 状态: `research_draft`;不是 `SourceChecked`;没有 `ReviewRecord`
- 范围: 4 个湿实验 protocol 组:差速超速离心分离 EV (P-UC)、尺寸排阻层析 SEC (P-SEC)、聚合物沉淀法 (P-PRECIP)、亚细胞组分分离 (P-FRAC,试剂盒序贯抽提 + 经典差速离心注记)。只记录湿实验步骤与原始读数;下游表征 (NTA 粒径/浓度、电镜、WB marker、质谱/测序/统计) 一律 out-of-scope 或 ToolAction/分析,不建操作。
- 规则: 三种 EV 分离机理分立记录,不合并为通用 "EV isolation";试剂盒 lot 配方与离心转子/管型按来源绑定,exact 值未见一律 UNKNOWN;现有仓库记录只作 Reuse Map 身份对照,不作步骤/参数证据;不新增公共模块除非四问 + 晋升四门全过;不做代码/UI/DB/JSON 变更。
- 禁止声明: 本文件不含任何通用 EV 得率、通用纯度、通用离心时间、通用组分 marker;所有 RCF/时间/体积/缓冲液均为所引协议的绑定值,不得跨协议复制。

## 1. 来源索引与实际读取深度

| # | Protocol | 权威类型 | 版本/定位 | 读取深度 (2026-11-05) |
|---|---|---|---|---|
| S1 | Théry et al., Curr Protoc Cell Biol 2006, Ch.3 Unit 3.22, exosome 差速离心经典程序 (300×g / 2,000×g / 10,000×g / 100,000×g) + MISEV2018 适用范围注记 | 方法论文/领域指南 | 差速离心序列 + PBS 洗 + 100,000×g 沉淀 | 检索转写级:低速去细胞/碎片→10,000×g 去微泡→0.22 µm 过滤→100,000×g 70 min×2 (PBS 洗);exact 转子 k-factor/管型/刹车设置 UNKNOWN |
| S2 | Izon qEV SEC 柱标准程序 (original/70 nm 系列用户指南) | 厂商手册 | 上样体积/平衡/void/收集窗 | 检索转写级:柱 RT 平衡→PBS 平衡→样品上样→void 后收集 EV 富集组分→后续蛋白组分;exact 上样上限/组分体积按柱型,本批 UNKNOWN |
| S3 | Thermo Total Exosome Isolation Reagent (cell culture media / serum, 4478359 系列) 产品说明书 | 厂商手册 | 沉淀比例/4 °C overnight/10,000×g | 检索转写级:澄清上清 + 1/2 体积试剂混匀→4 °C overnight→10,000×g 1 h 4 °C→弃上清→PBS 重悬;血清程序比例/离心分支不同;exact 沉淀化学 UNKNOWN (厂商专有) |
| S4a | Thermo Subcellular Protein Fractionation Kit 78840 说明书 (CEB/MEB/NEB 序贯抽提) | 厂商手册 | 胞质/膜/核可溶/染色质/骨架五步抽提 | 检索转写级:贴壁/悬浮分支→CEB 4 °C 10 min→500×g 5 min→MEB 4 °C 10 min→3,000×g 5 min→NEB 4 °C 30 min→5,000×g 5 min→微球核酸酶/染色质抽提;exact 体积按细胞量表,本批 UNKNOWN |
| S4b | 经典差速离心细胞器分离 (核/线粒体/微粒体/胞质,匀浆 + 600×g/10,000×g/100,000×g 序列,通用教科书程序) | 方法论文/教科书程序 | 匀浆→差速离心序列 | 检索转写级:低渗/等渗匀浆→600×g 核→10,000×g 线粒体→100,000×g 微粒体→上清胞质;exact 匀浆次数/缓冲液配方按来源,本批 UNKNOWN;仅记存在与分支,不给通用配方 |

既有复用对照(只记身份,不复述契约,不作证据):
R1 `op.liquid_transfer`;R2 取样/分样 sampling atomic;R3 `op.incubation` (elapsed/条件 only,不断言分离/抽提完成);R4 `op.washing` (保留语义) + lossy 变体;R5 `op.centrifugation` (RCF/时间/温度/转子绑定);R6 连续稀释=Composite;R7 `absorbance_measurement` (raw OD only);R8 `FLUO_IMAGE`/发光读数与本批无关;B13 审计 `#23`–`#27` (裂解/定量不单原子) 与 P-FRAC 直接相关。

检索摘要 (`search-summary-only`,非 `SourceChecked` 证据):MISEV2018 全文、qEV 各柱型组分表、沉淀试剂 lot 片仅用于立项定位;凡数值以 §1 直接转写为准,未见值一律 UNKNOWN。

---

## 2. P-UC — 差速超速离心 EV 分离 (S1)

绑定摘要:条件培养基/体液→300×g 10 min 去细胞→2,000×g 10 min 去死细胞/碎片→10,000×g 30 min 去大囊泡→0.22 µm 过滤→100,000×g 70 min→PBS 洗沉淀→100,000×g 70 min→PBS 重悬。温度 4 °C;刹车/转子按超离机说明 (本批 UNKNOWN)。

### 表 1:步骤拆解

| 原文步骤 | 操作语义(现有模块/候选) | 输入 → 输出 | 参数归属 | 来源定位 |
|---|---|---|---|---|
| 低速序列 300×g/2,000×g/10,000×g + 弃沉淀/收上清 | `op.centrifugation` 参数化 + `sampling` (R2,上清新建,碎片沉淀谱系) | 培养上清/体液 → 澄清上清 + 碎片沉淀 | 方法绑定:RCF/时长三档;exact 温度除 4 °C 外按机型 | S1 差速序列行 |
| 0.22 µm 过滤 | method-local (`EV-CLARIFY`,过滤澄清分支) | 澄清上清 → 可超离上清 | 方法绑定:孔径 0.22 µm;滤器型号 UNKNOWN | S1 过滤行 |
| 100,000×g 70 min 沉淀 + PBS 洗 + 再 100,000×g 70 min | `op.centrifugation` 参数化 + 方法内 EV 沉淀分支 (`EV-UC-PELLET`) | 可超离上清 → EV 粗沉淀 + 耗尽上清 | 方法绑定:100,000×g/70 min×2;exact k-factor/刹车 UNKNOWN | S1 超离行 |
| PBS 重悬沉淀 | `op.liquid_transfer` 参数化 | EV 粗沉淀 → EV 悬液 | 方法绑定:重悬体积按产量,exact 值 UNKNOWN | S1 重悬行 |
| NTA/电镜/marker WB (下游) | out-of-scope (分析/检测) | — | 非本批;见 §8 G-8 | S1 表征注记 + MISEV |

### 表 2:公共操作复用

| 公共操作候选 | 出现过的独立方法家族 | 处理决定 | 证据/缺口 |
|---|---|---|---|
| 离心 (低速澄清/超离沉淀) | 多家族 | shared (R5) | 仅 RCF/时长绑定;转子/k-factor UNKNOWN |
| 液体转移/取样 (收上清/重悬) | 多家族 | shared (R1/R2) | 守恒缺口沿用;沉淀回收率 UNKNOWN |
| 通用 "EV isolation" (UC+SEC+沉淀合并) | 表面多见 | **拒绝合并**,不新增 | 机理/纯度/污染谱/可逆性异质,G2/G4 不过 (见 §7) |
| 0.22 µm 过滤澄清 | 仅本分支 | method-local 分支 | 单方法前处理;G1 不过 |

---

## 3. P-SEC — 尺寸排阻层析 EV 富集 (S2)

绑定摘要:qEV 柱 RT 平衡→PBS/缓冲液平衡→澄清样品上样 (体积 ≤ 柱上限)→void 体积弃去→EV 富集组分收集→后续蛋白组分另收;柱一次性/再生按型号说明。

### 表 1:步骤拆解

| 原文步骤 | 操作语义(现有模块/候选) | 输入 → 输出 | 参数归属 | 来源定位 |
|---|---|---|---|---|
| 柱平衡至 RT + 缓冲液平衡 | `op.incubation` + `op.liquid_transfer` 参数化 | 干柱 → 已平衡柱 | 方法绑定:平衡体积按柱型,exact 值 UNKNOWN | S2 平衡节 |
| 上样 (澄清浓缩样品,体积上限) | `op.liquid_transfer` 参数化 | 澄清样品 + 已平衡柱 → 已上样柱 | 方法绑定:上样上限按柱型;超量为方法违规 | S2 上样行 |
| 洗脱收集:void 弃 + EV 窗收集 + 蛋白窗另收 | 方法内 SEC 收集分支 (`EV-SEC-COLLECT`) + `sampling` 谱系 | 已上样柱 → EV 组分 + 蛋白组分 | 方法绑定:收集窗按柱型表;exact mL UNKNOWN | S2 收集节 |
| 柱后浓缩 (超滤,按需) | 存在注记,本批不拆 | — | 方法绑定;需独立 protocol | S2 浓缩注记 |

### 表 2:公共操作复用

| 公共操作候选 | 出现过的独立方法家族 | 处理决定 | 证据/缺口 |
|---|---|---|---|
| 液体转移/保持/取样 | 多家族 | shared (R1/R3/R2) | 仅体积/窗绑定 |
| 通用 "EV isolation" | 见 §2 | **拒绝合并** | SEC 按尺寸 vs UC 按沉降 vs 沉淀按溶解度,G2 不过 |
| SEC 收集窗/void 判定 | 仅本家族 | method-local,不新增 | G1 不过;与蛋白纯化 SEC 不合并 (样品/窗/对照不同) |

---

## 4. P-PRECIP — 聚合物沉淀法 EV 富集 (S3)

绑定摘要:培养上清 2,000×g 30 min 澄清→上清 + 1/2 体积沉淀试剂→混匀→4 °C overnight (或 2 h–overnight 分支)→10,000×g 1 h 4 °C→弃上清→PBS 重悬沉淀。血清样本另有比例/离心分支。

### 表 1:步骤拆解

| 原文步骤 | 操作语义(现有模块/候选) | 输入 → 输出 | 参数归属 | 来源定位 |
|---|---|---|---|---|
| 2,000×g 30 min 澄清 + 收上清 | `op.centrifugation` + `sampling` 参数化 | 培养上清 → 澄清上清 | 方法绑定:2,000×g 30 min | S3 澄清行 |
| 加沉淀试剂 1/2 体积 + 混匀 | `op.liquid_transfer` 参数化 | 澄清上清 + 试剂 → 沉淀混合物 | 方法绑定:比例;试剂化学 UNKNOWN (专有) | S3 加样行 |
| 4 °C 保持 overnight (分支 2 h 起) | `op.incubation` 参数化 (定时冷保持) | 沉淀混合物 → 已作用混合物 | 方法绑定:时长分支;只 elapsed,不推断沉淀完全 | S3 保持行 |
| 10,000×g 1 h 4 °C 沉淀 + 弃上清 + PBS 重悬 | `op.centrifugation` + 方法内沉淀分支 (`EV-PRECIP-PELLET`) + 转移 | 已作用混合物 → EV 沉淀 + 耗尽上清 → EV 悬液 | 方法绑定:10,000×g 1 h;共沉淀蛋白污染为已知方法约束 | S3 离心/重悬行 |

### 表 2:公共操作复用

| 公共操作候选 | 出现过的独立方法家族 | 处理决定 | 证据/缺口 |
|---|---|---|---|
| 离心/转移/保持/取样 | 多家族 | shared (R5/R1/R3/R2) | 仅条件绑定 |
| 沉淀化学/共沉淀控制 | 仅本家族 | method-local,不新增 | G1 不过;试剂专有,UNKNOWN |
| 纯度声明 (沉淀 vs UC/SEC) | — (判读) | 非操作;方法约束 | 沉淀法蛋白/RNA 共沉淀高,不得与 UC/SEC 互换解释 |

---

## 5. P-FRAC — 亚细胞组分分离:试剂盒序贯抽提 + 差速离心注记 (S4a/S4b)

绑定摘要 (S4a):贴壁 PBS 洗刮取 / 悬浮集菌→CEB 4 °C 10 min→500×g 5 min (上清=胞质)→MEB 4 °C 10 min→3,000×g 5 min (上清=膜)→NEB 4 °C 30 min→5,000×g 5 min (上清=可溶核)→微球核酸酶 + 染色质抽提 (沉淀=染色质结合组分)→剩余沉淀=骨架组分。蛋白酶抑制剂现加;各组分 marker WB 验证为下游。S4b 经典差速离心 (匀浆→600×g 核→10,000×g 线粒体→100,000×g 微粒体→胞质) 本批只记分支存在,不给通用配方。

### 表 1:步骤拆解

| 原文步骤 | 操作语义(现有模块/候选) | 输入 → 输出 | 参数归属 | 来源定位 |
|---|---|---|---|---|
| 收细胞:贴壁洗刮取 / 悬浮集菌 + 抑制剂现加 | `op.washing` + `op.centrifugation` + `op.liquid_transfer` 参数化 | 培养物 → 待抽提细胞 | 方法绑定:分支;exact 体积/抑制剂倍数 UNKNOWN | S4a 收样节 |
| CEB/MEB/NEB 三步抽提保持 (4 °C 10/10/30 min) | `op.incubation` 参数化 | 细胞/沉淀 + 抽提液 → 已作用混合物 | 方法绑定:时长三档;只 elapsed,不推断抽提完全 | S4a 抽提行 |
| 三步离心 500×g/3,000×g/5,000×g 5 min + 逐级收上清 | `op.centrifugation` + `sampling` (R2,各级上清新建组分) | 已作用混合物 → 胞质/膜/可溶核组分 + 沉淀 | 方法绑定:RCF 三档;交叉污染为方法约束 | S4a 离心行 |
| 核酸酶消化 + 染色质抽提 + 骨架组分回收 | 方法内组分分支 (`FRAC-CHROM/SKEL`) | 核沉淀 → 染色质组分 + 骨架组分 | 方法绑定:酶量/时长按说明书,exact 值 UNKNOWN | S4a 后段节 |
| 经典差速离心细胞器分支 (S4b) | 存在注记,本批不拆执行 | — | 方法绑定;匀浆手法/缓冲液 UNKNOWN | S4b 注记 |
| marker WB/定量验证 (下游) | out-of-scope | — | B05/B13 方法模板;本批止于组分 | S4a 验证注记 |

### 表 2:公共操作复用

| 公共操作候选 | 出现过的独立方法家族 | 处理决定 | 证据/缺口 |
|---|---|---|---|
| 转移/保持/离心/取样/洗涤 | 多家族 | shared (R1/R3/R5/R2/R4) | 仅条件绑定 |
| 序贯抽提化学 (CEB/MEB/NEB) | 仅本家族 | method-local,不新增 | G1 不过;配方专有 |
| 通用 "fractionation" (试剂盒 + 差速离心合并) | 表面多见 | **拒绝合并** | 去污剂序贯 vs 机械匀浆差速,机理/组分定义异质,G2 不过 |
| 组分纯度/marker 判读 | — (分析) | 非操作 | 交叉污染只能由 marker 对照判定 |

---

## 6. Reuse Map 汇总(本批)

| 实验步骤 | 复用对象 | 处理 |
|---|---|---|
| 上样/平衡/试剂加样/重悬/组分收集 | `op.liquid_transfer` | 参数化复用;绑定体积/比例/缓冲液 (§§2–5) |
| 沉淀保持/抽提保持/柱平衡 | `op.incubation` | 参数化复用;绑定时长/温度;只断言 elapsed,不推断分离/抽提完成 |
| 澄清/沉淀/组分分离离心 | `op.centrifugation` | 参数化复用;绑定 RCF/时长 (300×g–100,000×g;500×g–5,000×g) |
| 上清/组分/沉淀分样 | sampling atomic | 参数化复用;父样保留 + 子样新建;回收率 UNKNOWN |
| 预洗/沉淀洗 | `op.washing` | 参数化复用;保留语义;损失显式 |
| NTA/电镜/marker/质谱/统计 | out-of-scope / ToolAction | 一律非操作;无下游不作纯度/身份声明 |

## 7. 跨方法公共操作覆盖矩阵

| 公共操作 | P-UC 差速超离 (S1) | P-SEC 层析 (S2) | P-PRECIP 沉淀 (S3) | P-FRAC 组分 (S4a/S4b) | 备注 |
|---|---|---|---|---|---|
| `op.liquid_transfer` | ● 重悬/洗 | ● 平衡/上样 | ● 试剂/重悬 | ● 抽提液/收组分 | 全参数化 |
| `op.incubation` | — | ● 柱平衡 | ● 4 °C overnight | ● 10/10/30 min | 只 elapsed |
| `op.centrifugation` | ● 四档 + 2×100,000×g | — | ● 2,000×g/10,000×g | ● 500/3,000/5,000×g | RCF 方法绑定 |
| sampling | ● 上清/沉淀谱系 | ● 组分谱系 | ● 沉淀谱系 | ● 五级组分谱系 | 守恒缺口沿用 |
| method-local (沉淀/收集窗/抽提化学) | ● UC 沉淀分支 | ● SEC 窗分支 | ● 沉淀分支 | ● 抽提/染色质分支 | 各留方法内;不合并 |
| ToolAction/下游 | ● NTA/电镜/WB | ● 浓缩/表征 | ● 纯度判读 | ● marker/定量 | 全部非操作 |

●=主路径使用;○=分支/监测;—=本方法无此步。

方向警示:UC (按沉降,高纯度倾向,低得率/共沉淀蛋白仍在) vs SEC (按尺寸,功能活性倾向,稀释/脂蛋白共洗脱) vs 沉淀 (按溶解度,高得率倾向,高蛋白污染)——同一 "EV 分离" 词在不同机理下对纯度/得率/功能含义相反,不得共用模块或互换解释。组分警示:试剂盒序贯抽提 (去污剂定义组分) vs 经典差速离心 (沉降定义细胞器)——组分边界定义不同,不得共用 "fractionation" 解释。

## 8. 候选与四问判定 + 晋升审查 (本批结论:零新公共模块)

| 候选 | 独立目的 | 稳定边界 | 独立确认 | 跨方法复用 (≥2 独立家族) | 结论 |
|---|---|---|---|---|---|
| 差速超速离心 EV 分离 | ✓ | ✓ | ✓ (沉淀可确认) | ✗ (仅 EV-UC 一家族) | method-local,不新增 |
| SEC EV 富集 | ✓ | ✓ | ✓ (组分可确认) | ✗ (仅 EV-SEC 一家族) | method-local,不新增 |
| 沉淀法 EV 富集 | ✓ | ✓ | ✓ | ✗ (仅沉淀一家族) | method-local,不新增 |
| 通用 "EV isolation" (三法合并) | ✓ | ✗ (沉降 vs 尺寸 vs 溶解度;污染谱/可逆性异质) | △ | ✗ (G2/G4 不过) | **拒绝合并**;分立记录 (题设要求) |
| 亚细胞序贯抽提 | ✓ | ✓ | ✓ (组分可确认) | ✗ (仅试剂盒一家族;差速离心未拆) | method-local,不新增 |
| 通用 "fractionation" | ✓ | ✗ (去污剂 vs 沉降定义) | △ | ✗ | **拒绝合并** |

晋升四门 (G1 ≥2 独立方法,域内成立;G2 语义稳定;G3 抽象收益;G4 分支成本):本批无候选同时通过。三法不合并的独立理由:端口不同 (超离管/转子 vs 层析柱/收集窗 vs 沉淀试剂/离心管)、样本 effect 不同 (沉降沉淀 vs 尺寸排阻 vs 溶解度沉淀)、完成检查与污染谱不同 (洗涤沉淀 vs void/蛋白窗 vs 共沉淀约束)。合并将隐藏机理差异,违反 G2/G4。

结论:本批零新公共模块;对公共 Catalog 的唯一贡献是复用证据增量:`op.liquid_transfer`、`op.incubation`、`op.centrifugation`、sampling、`op.washing` (+EV/组分五类绑定)。

## 9. 缺口清单 (不能表达/UNKNOWN;分类按 record-standard §4)

| # | 缺口 | 类别 | 说明 |
|---|---|---|---|
| G-1 | 对照矩阵不可省 | 方法绑定缺口 | UC:耗尽上清 + PBS 洗对照;SEC:蛋白窗对照;沉淀:试剂空白 + 共沉淀声明;组分:marker panel + input。任一缺即无意义 |
| G-2 | exact 条件未取 | `source_missing` | 转子 k-factor/刹车、SEC 柱型组分表、沉淀试剂化学、抽提液体积/抑制剂倍数、核酸酶量——按 lot/机型,本批 UNKNOWN |
| G-3 | 力学/手法量 | `source_missing` | 匀浆强度、重悬吹打、沉淀轻柔度——原文定性,归 SOP 操作者判断 |
| G-4 | 派生/表征归属 | ToolAction/分析,非操作 | NTA 粒径/浓度、纯度比、marker densitometry、回收率——全部下游;无表征不作身份/纯度声明 |
| G-5 | 系别/批依赖值 | `method_dependent` + `configuration_dependent` | 上清体积/细胞量、柱上样上限、沉淀比例分支 (培养基 vs 血清)、抽提体积按细胞量 |
| G-6 | 不可逆/终点约束 | 跃迁约束 | 100,000×g 沉淀后功能/形态改变;沉淀法共沉淀不可逆混入;柱超载不可逆交叉;组分交叉污染只能 marker 判定 |
| G-7 | 时间锚点 | 锚点缺口 | UC `t₀`=首次超离启动;SEC `t₀`=上样;沉淀 `t₀`=试剂加入;组分 `t₀`=抽提液加入。保持不断言分离/抽提完成 |
| G-8 | 范围外 (误建模块风险) | `not_applicable` | NTA/电镜/WB marker/质谱/测序、动物/临床样本、通用 "EV isolation"/"fractionation" 模块——本批一律不建模 |
| G-9 | 损失/守恒未量化 | 合同缺口 (沿用) | 超离回收率、SEC 稀释损失、沉淀共沉淀量、组分交叉量——量级 UNKNOWN;lossy 显式 |

## 10. 核查与状态

- 已验证 (检索转写,2026-11-05,§1 定位节):S1 四档离心 + 2×100,000×g + 过滤;S2 平衡/上样/void/收集窗;S3 比例/保持/离心/重悬双分支;S4a 五步抽提 + 三档离心;S4b 序列注记。单人 `agent-verified`,仍需第二人逐字段复核;未作 `SourceChecked` / `ExpertReviewed` / `LabValidated` 声明。
- 未验证:任何 PDF 全文逐页、per-lot 片 (试剂/柱/抗体)、超离机操作手册、实验室 SOP 改编、已跑数据派生应用。
- 记录清单:每值有定位或标 UNKNOWN;方法要求与仪器规格分离;样本跃迁有端口 (上清/沉淀/组分/悬液);时间锚显式 (§9 G-7);部分列表标部分 (柱型/缓冲液选项);plan/actual/raw/derived 分离 (本批只有 plan + 方法约束,无 run 值);无 JSON;UNKNOWN 永不填零。
