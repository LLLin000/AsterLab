# B01 — Morimoto 基础细胞培养:四协议审计

- 日期:2026-11-05 | 状态:`research_draft`;无 `ReviewRecord`;不作 `SourceChecked`
- 范围:已读四协议的完整性审计;原 PDF 为事实依据,`k2-operations/morimoto-basic-cell-culture.md` 仅作 Reuse Map 上下文
- 规则:原文没写 = UNKNOWN;不为本批次新增公共模块

## 1. 来源索引

| ID | Protocol | URL | 定位 | 读取程度 |
|---|---|---|---|---|
| M1 | Cell Splitting Protocol | https://www.morimotolab.org/_files/ugd/594a6c_20c8434bdc634367b03e8438b1c27916.pdf | 1 页,步骤 1–11,p. II.A.13 | PDF 全文直接读 |
| M2 | Cell Counting Using a Hemacytometer | https://www.morimotolab.org/_files/ugd/594a6c_fe7b278f85e147be88ffc8b5c7752dd4.pdf | 3 页,原理+计数规则+步骤 1–7,p. II.A.14–16 | PDF 全文直接读 |
| M3 | Trypan Blue Staining | https://www.morimotolab.org/_files/ugd/594a6c_10d20d9fc4d54187bbf74c3864be1167.pdf | 1 页,Materials/Procedure/Comments,p. II.A.18 | PDF 全文直接读 |
| M4 | Freezing/Thawing of Tissue Culture Cells | https://www.morimotolab.org/_files/ugd/594a6c_792f446515a141fb9e54e210ff8be8c0.pdf | 2 页,Freezing 1–7 / Thawing 1–6,p. II.A.19–20 | PDF 全文直接读 |

## 2. P-M1 细胞传代(100mm 板)

### 表 1 步骤拆解

| 原文步骤 | 操作语义 | 输入 → 输出 | 参数归属 | 来源定位 |
|---|---|---|---|---|
| 1 镜下判断 split ratio | `custom_step` 人工判断,非原子操作 | 母板 → ratio 决策 | 方法绑定;ratio 数值见步骤 8 表 | M1 step 1 |
| 2 吸弃旧培养基 | `op.liquid_transfer` | 母板 → 无旧液 | 方法绑定 | M1 step 2 |
| 3–4 2mL 1×PBS 轻摇后吸弃 | `op.washing` | 母板 → washed | 方法绑定(体积/次数) | M1 steps 3–4 |
| 5–6 2mL 1×Trypsin,37°C CO₂ 约 5min | `method_local` 解离 + `op.incubation` | 贴壁 → detached | 方法绑定;dwell/试剂浓度 UNKNOWN(细胞系相关) | M1 steps 5–6 |
| 7 新板预加新鲜培养基 | `op.liquid_transfer` | new_plate → primed | 方法绑定;体积 UNKNOWN(步骤 8 表为 cell+media 总体积) | M1 step 7 |
| 8 取出确认脱落;按表分装 | `op.incubation` 结束 + `op.liquid_transfer` | 母板 8mL media 收集 → new_plate | 方法绑定:1:10=1+9;1:5=2+8;1:2=5+5 mL | M1 step 8+表 |
| 9 加 8mL media 吹打收集 | `method_local` 分散 + `op.liquid_transfer` | detached → suspension | 方法绑定(8mL) | M1 step 9 |
| 10–11 分装对应体积,37°C CO₂ 培养 | `op.liquid_transfer` + `op.incubation` | suspension → new_culture | 方法绑定 | M1 steps 10–11 |

### 表 2 公共操作复用

| 公共操作候选 | 出现过的独立方法家族 | 处理决定 | 证据/缺口 |
|---|---|---|---|
| 液体转移 | 传代 + 计数(M2 步骤 3/5)+ 冻存(M4) | shared(已有 `op.liquid_transfer`) | 3 协议内复用;参数(体积/容器)方法绑定 |
| 洗涤(preserving) | 传代 PBS 洗(M1) | shared(已有 `op.washing`) | 1 协议;跨家族复用见 catalog 审计 |
| 孵育/保持 | 传代解离+培养(M1);冻存过夜(M4) | shared(已有 `op.incubation`) | 均只确认 elapsed/条件;CO₂%/湿度 UNKNOWN |
| 解离(trypsinization) | 传代(M1)+ 计数预处理(M2)+ 台盼蓝前处理(M3) | `method_local` | 审计 #24:酶/机械效应不同,不建通用模块 |
| 显微镜判断 split ratio | 仅传代 | `method_local`/Custom Step | 人工判断,无稳定边界;不进 Catalog |

## 3. P-M2 血球计数

### 表 1 步骤拆解

| 原文步骤 | 操作语义 | 输入 → 输出 | 参数归属 | 来源定位 |
|---|---|---|---|---|
| 原理:Neubauer 9 大格,1mm²×0.1mm=10⁻⁴mL;40×(4×物镜) | `method_local` 仪器绑定 | — → 几何常量 | 方法绑定 | M2 p.1 |
| 准备:lens paper+ethanol 清洁,盖玻片预置,毛细填充 | `method_local` 上样 | suspension → chamber | 方法绑定 | M2 p.1 |
| 混匀后取样;单细胞率 ≥90% 否则重取 | `op.sample_aliquot` + 人工判断 | suspension → aliquot | 参数槽(≥90%);重取分支 | M2 p.2 |
| 系统计数:目标 ~100(大细胞)或 300–400;每格 20–50 | `shared_operation` 细胞浓度计数 | chamber → raw count | 参数槽(目标数/格数) | M2 pp.1–2 |
| 计数约定:touch 左/上计、右/下不计;overlap 上/右为 in、下/左为 out | `method_local` 计数规则 | — → count | 方法绑定;两句规则自相矛盾,见 §6 | M2 p.2 |
| 公式 cells/mL=平均/格×稀释×10⁴;示例 125/5格=2.5×10⁵/mL | `tool_action` 计算 | count → cells/mL(derived) | 方法绑定 | M2 p.2 |
| 预处理 1–7:PBS→2mL trypsin 37°C ~5min→8mL media→2000rpm 2min→按 pellet 留 2/3/4mL→重悬→取样计数 | `op.washing`/`method_local`/`op.centrifugation`/`op.liquid_transfer`/`shared_operation` | plate → suspension → pellet → measured aliquot | 方法绑定;rpm→RCF UNKNOWN;留液- pellet 映射见 M2 p.2–3 表 | M2 pp.2–3 steps 1–7 |
| 下游:HeLa HSE-Luc 7000 cells/well,96 孔总量估算,加 G418/DMEM | `method_local` 播种计算 | cells/mL → plating 体积 | `tool_action` + 方法绑定 | M2 p.3 |

### 表 2 公共操作复用

| 公共操作候选 | 出现过的独立方法家族 | 处理决定 | 证据/缺口 |
|---|---|---|---|
| 细胞浓度计数(hemocytometer) | 计数(M2)+ 台盼蓝(M3 step 3 引用) | shared(审计 #11) | 仅 count 本身;活力判读不在此 |
| 离心 | 预处理 step 4(M2 2000rpm 2min) | shared(已有 `op.centrifugation`) | rpm→RCF UNKNOWN,转子依赖 |
| 取样/分样 | 混匀取样(M2 p.2)+ 台盼蓝 1:1(M3) | shared(条件,审计 §6.1) | 谱系/数量守恒缺口 |

## 4. P-M3 台盼蓝活力

### 表 1 步骤拆解

| 原文步骤 | 操作语义 | 输入 → 输出 | 参数归属 | 来源定位 |
|---|---|---|---|---|
| 材料:Trypan Blue 0.8mM in PBS,室温保存,稳定 1 个月 | `method_local` 试剂绑定 | — → reagent | 方法绑定 | M3 Materials |
| 1 Trypsinize cells | `method_local` 解离 | adherent → suspension | 参数 UNKNOWN(浓度/时间/体积均未写) | M3 step 1 |
| 2 与台盼蓝 1:1 混合 | `op.liquid_transfer` + `method_local` 染色 | suspension → stained_mix | 参数槽(1:1) | M3 step 2 |
| 3 血球计数:活细胞排染、死细胞蓝染 | `shared_operation` 计数 + `method_template` 判读 | stained_mix → raw count → viability | 方法绑定;viability% 公式 M3 未写,见 §6 | M3 step 3 |
| Comments:暴露 ≤30min,超时假死增加;不区分 necrotic/apoptotic | `method_local` 约束 | — → 读数时限 | 方法绑定 | M3 Comments+首行 |

### 表 2 公共操作复用

| 公共操作候选 | 出现过的独立方法家族 | 处理决定 | 证据/缺口 |
|---|---|---|---|
| 液体转移/混匀 | 传代 + 计数 + 台盼蓝 | shared | 体积 UNKNOWN(M3 只写 1:1) |
| 染色 | 仅台盼蓝 | `method_local`(审计 #23) | 染料效应异质,不建通用染色模块 |
| 活力% 判读 | 仅台盼蓝 | `method_template` + `tool_action` | 审计 #12:非 operation;M3 未给公式 |
| 细胞浓度计数 | 计数(M2)+ 台盼蓝 | shared | 与 M2 共享计数原子 |

## 5. P-M4 冻存/复苏

### 表 1 步骤拆解

| 原文步骤 | 操作语义 | 输入 → 输出 | 参数归属 | 来源定位 |
|---|---|---|---|---|
| Freeze 1 仅近期复苏细胞补库存;多 passage 不冻存 | `method_local` 库存策略 | — → 资格门 | 方法绑定 | M4 F step 1 |
| Freeze 2 冻存状态:dense 但未 confluent、生长好,10cm 板 | 人工/门条件 | culture → 合格 | 方法绑定 | M4 F step 2 |
| Freeze 3 洗涤重悬后 pelleted:clinical centrifuge setting 3(~2000rpm)5min | `op.centrifugation` | suspension → pellet+supernatant | 方法绑定;RCF UNKNOWN;wash/resuspend 无细节 UNKNOWN | M4 F step 3 |
| Freeze 4 弃上清,0.6–0.8mL 预温冻存液(培养基+20% serum+10% DMSO) | `op.liquid_transfer` + `method_local` 配液 | pellet → frozen_liquid | 方法绑定 | M4 F step 4 |
| Freeze 5 Nunc vial 标注 cell type/date/name | `method_local` 标注 | frozen_liquid → labeled vial | 方法绑定 | M4 F step 5 |
| Freeze 6 冻存架 1°C/min,位置 3–4,随 LN 液位调,过夜 | `op.incubation`(控降温)+ Custom Step 支架位置 | vial → frozen | 方法绑定;架型号/实际速率 UNKNOWN | M4 F step 6 |
| Freeze 7 干冰转 cane,保低温,按物种归档,双罐备份 | `method_local` 存储 | frozen → archived | 方法绑定 | M4 F step 7 |
| Thaw 1–2 库存责任+优先近期 vial | `method_local` 库存策略 | — → 选择门 | 方法绑定 | M4 T steps 1–2 |
| Thaw 3 37°C 水浴快融摇动,EtOH 擦拭 | `method_local` 融化 | vial → thawed | 方法绑定;终点/时间 UNKNOWN;EtOH 浓度 UNKNOWN | M4 T step 3 |
| Thaw 4 悬浮:10mL 预温培养基,setting 3 ~2000rpm 5min,去上清重悬 10mL | `op.liquid_transfer`+`op.centrifugation`+`op.incubation` | vial → culture | 方法绑定;RCF UNKNOWN | M4 T step 4 |
| Thaw 5 贴壁:10cm 板 10mL 预温培养基,贴壁数小时/过夜后换液 | `op.liquid_transfer`+`op.incubation`+`op.washing` | vial → adherent culture | 方法绑定 | M4 T step 5 |
| Thaw 6 再冻存用 first split 细胞 | `method_local` 库存策略 | culture → 资格门 | 方法绑定 | M4 T step 6 |

### 表 2 公共操作复用

| 公共操作候选 | 出现过的独立方法家族 | 处理决定 | 证据/缺口 |
|---|---|---|---|
| 离心 | 冻存(M4 F3)+ 计数(M2)+ 复苏(M4 T4) | shared | 均为 rpm/setting,RCF UNKNOWN |
| 液体转移/去上清 | 冻存/复苏 + 传代 + 计数 | shared | 去上清为 transfer 复用,不另立模块(审计 #7) |
| 孵育/保持 | 冻存过夜(M4 F6)+ 复苏贴壁(M4 T5) | shared | 只确认 elapsed;降温速率非生长断言 |

## 6. Reuse Map 汇总(→ 现有记录)

| Morimoto 步骤 | 复用对象 | 处理 |
|---|---|---|
| M1 全程传代 | `cell-passaging-adherent`(Composite Block) | 复合块;内部复用 transfer/wash/incubation |
| M2 计数读数 | `OP-CELL-COUNT-HEMACYTOMETER`(读数原子候选) | shared 候选(审计 #11);公式为 ToolAction |
| M3 活力 | `MT-TRYPAN-BLUE-VIABILITY`(Method Template) | 计数复用 shared;染色/判读 method_local |
| M4 冻存 | `MT-CELL-FREEZE`(Composite;`cryopreservation-storage` handoff) | 冻存液/降温/存储方法绑定 |
| M4 复苏 | `MT-CELL-THAW`(Composite 候选) | 现有记录为 handoff,无独立 thaw 模块(缺口) |
| 吸弃/加液/分装(M1–M4) | `op.liquid_transfer` | 直接复用;体积方法绑定 |
| PBS 洗(M1) | `op.washing`(preserving) | 直接复用 |
| 37°C 培养/过夜(M1/M4) | `op.incubation` | 复用;elapsed only |
| 2000rpm pelleting(M2/M4) | `op.centrifugation` | 复用;RCF UNKNOWN |
| trypsin 解离(M1/M2/M3) | `method_local`(解离) | 不并入洗涤/转移 |
| 显微镜 split 判断(M1) | Custom Step(人工判断) | 不立模块 |
| split ratio 表/留液表/冻存液配比 | 方法绑定参数 | 不写通用默认 |

## 7. 缺失/错误映射(相对 `morimoto-basic-cell-culture.md`)

| # | 位置 | 问题 | 修正 |
|---|---|---|---|
| 1 | M2 计数约定 | 现有记录只复述“上/左计”与“overlap 上/右 in”,未指出原文两句自相矛盾 | 标记 source-internal inconsistency;以哪句为准 UNKNOWN,需第二人核对原图 |
| 2 | M2 准备 | 漏 lens paper+ethanol、盖玻片预置、Pasteur 毛细上样、混匀后取样/沉降警告 | 补入表 1;均已在上表 |
| 3 | M2 下游 | 漏 HeLa HSE-Luc 7000/well、96 孔估算、G418/DMEM 配液(原文排版破碎) | 标注 `method_local`;G418 浓度/终体积数字 UNKNOWN(排版无法可靠定位) |
| 4 | M3 浓度 | 现有记录未提示与 Thermo 记录 0.4% 的冲突 | 保留双方:Morimoto 0.8mM / Thermo 0.4%(w/v)方法绑定,不合并 |
| 5 | M3 活力公式 | 现有记录称“活力% 是派生计算”但 M3 未写公式 | formula 标 UNKNOWN(at this source);可引用 Thermo 公式作跨来源绑定,须注明来源不同 |
| 6 | M4 F1 | 现有记录写“只用近期复苏细胞补充库存”,漏“多 passage 不冻存为 stock” | 补入表 1 |
| 7 | M4 F3 | 现有记录只记 pelleting,漏“washed and resuspended”前提且无细节 | 前提补入;wash 体积/液 UNKNOWN |
| 8 | M4 F7 | 漏双罐备份+按物种归档的理由 | 补入;属库存策略 method_local |
| 9 | M1 step 7 | 现有记录“新板加新鲜培养基”未记顺序在解离前 | 表 1 已按原文顺序;体积 UNKNOWN |
| 10 | M2 计算 | 现有记录保留公式+示例正确 | 无需改;注明公式为 ToolAction 非 operation |

## 8. 候选与四问判定

| 候选 | 独立目的 | 稳定边界 | 独立确认 | 跨方法复用 | 结论 |
|---|---|---|---|---|---|
| trypsin 解离 | ✓ | ✓ | ✓(镜下脱落) | ✗(酶/机械效应异质,审计 #24) | `method_local`,不新增 |
| 冻存架放置(3–4) | ✗(速率副产物) | ✗(液位依赖) | ✗ | ✗ | Custom Step 备注,不新增 |
| split ratio 镜下判断 | ✓ | ✗(主观) | △ | ✗ | Custom Step 人工判断,不新增 |
| pellet-留液映射表(M2) | ✓ | ✓ | ✓ | ✗(仅计数预处理) | 方法绑定参数,不新增 |
| 台盼蓝染色 | ✓ | ✓ | ✓ | ✗(审计 #23) | `method_local`,不新增 |

结论:本批无候选通过“四问 + 公共晋升”双门;不新增公共模块。

## 9. 缺口清单

| ID | 缺口 | 影响 |
|---|---|---|
| G-B01-1 | 复苏(thaw)无独立模块;现有 `cryopreservation-storage` 仅 handoff | M4 Thaw 1–6 只能挂 Composite 候选,不能进运行时 |
| G-B01-2 | 计数约定自相矛盾 | 同一格子两套归属规则,读数不可审计,需原图核对 |
| G-B01-3 | 所有 rpm/setting 无 RCF(转子 UNKNOWN) | `op.centrifugation` 只能绑 rpm/setting 原值,不得换算 |
| G-B01-4 | 培养条件不全:仅 37°C CO₂,CO₂%/湿度/培养基配方 UNKNOWN | `op.incubation` 绑定不完整 |
| G-B01-5 | M3 trypsinize 无参数;M4 F3 wash/resuspend 无细节 | 解离/洗涤绑定留 UNKNOWN |
| G-B01-6 | 冻存密度/活力门、降温实测速率、LN 温度、双罐执行细节 UNKNOWN | 冻存 Composite 不能作通用默认 |

*审计说明:四份 PDF 已直接读取;本文件只交来源索引+两表/协议+Reuse Map+缺口;无 `SourceChecked`;无新增模块。*
