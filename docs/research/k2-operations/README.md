# K2/K3 操作模块来源审计(K2/K3 Operation Source Audit)

- 日期:2026-11-05
- 状态:四份 `research_draft` 操作记录;没有 `SourceChecked`;没有导入运行时。
- 目的:先把“操作是什么、输入输出是什么、参数从哪来、未知在哪里”写清,再谈自动组合或 UI。
- 规则:每个数值/行为必须有一手来源 + 页码/章节/表格 + 检索日期;不能确认就写 UNKNOWN;不同操作不得共享默认。

## 文件

| 操作 | 记录 | 主要来源 | 关键边界 |
|---|---|---|---|
| 液体转移 | [liquid-transfer.md](liquid-transfer.md) | Opentrons Liquid Control / Complex Parameters; Eppendorf 手动移液器手册(经仪器档案); ATCC 传代示例; ISO 8655 仅作二级说明 | 手动为参考实现;自动移液是映射;体积/吸头/混匀/吹出参数不能默认 |
| 接种与孵育 | [inoculation-and-incubation.md](inoculation-and-incubation.md) | Thermo Gibco 贴壁/悬浮培养、Corning 培养基体积、ATCC 培养指南;CO₂ 培养箱手册 | `OP-INOC` 与 `OP-INCUB` 是两个权威,通过“已接种容器 + 记录”交接;任何密度/体积必须来自细胞系说明书/SOP |
| 洗涤与取样 | [washing-and-sampling.md](washing-and-sampling.md) | Thermo 贴壁培养洗涤、Arbor Assays 洗板说明、Stanford 上清取样;其余仅检索摘要 | 洗涤 = 同一身份保留、液体替换;取样 = 父样本保留 + 新子样本 ID;不能共用一个默认 |
| 吸光度与荧光读出 | [absorbance-and-fluorescence-readout.md](absorbance-and-fluorescence-readout.md) | Multiskan SkyHigh 规格;Leica DMi8 规格/FAQ/手册;Dojindo CCK-8;Thermo TMB/LIVE-DEAD/IF 协议 | 原始读数不是活力/浓度/计数;读板与成像只是终端数据操作;推导另立步骤 |
| 细胞培养/分离 | [cell-culture-and-separation.md](cell-culture-and-separation.md) | Thermo/ATCC 直接读;Corning 摘要 | 传代、计数、离心、冻存分成四个记录 |
| ELISA / Western blot | [elisa-and-western-blot.md](elisa-and-western-blot.md) | E1–E3/W1–W2 直接读;部分摘要 | 方法变体分开;原始读数不直接变浓度/量 |
| qPCR / 核酸 QC | [qpcr-and-nucleic-acid-qc.md](qpcr-and-nucleic-acid-qc.md) | MIQE 直接读;部分摘要 | 提取、RT、qPCR 分开;不写通用循环程序 |
| 流式 / 免疫荧光 | [flow-cytometry-and-immunofluorescence.md](flow-cytometry-and-immunofluorescence.md) | T1–T5 直接读;部分摘要 | 分析与分选分开;平台能力不等于实际配置 |
| 生物膜 CV/CFU | [biofilm-cv-cfu.md](biofilm-cv-cfu.md) | S1–S3 直接读;S4/S5 摘要 | CV 生物量与 CFU 活菌不可互换 |
| 划痕/Transwell | [wound-healing-and-migration.md](wound-healing-and-migration.md) | P1–P4 直接读 | 方法模板;原始图像为终点,分析不在范围 |
| 活力/细胞毒性 | [viability-and-cytotoxicity-methods.md](viability-and-cytotoxicity-methods.md) | S1–S3/S6–S9 直接读;S4/S5 摘要 | CCK-8/LDH/LIVE-DEAD 方法绑定,不重定义通用操作 |
| 3D/类器官 | [three-d-and-organoid-operations.md](three-d-and-organoid-operations.md) | S1–S10 摘要/未读 | 基质/球体/换液;不硬套 2D 孔板;需实验室 protocol |

逐字段证据矩阵见 [field-evidence-matrix.md](field-evidence-matrix.md)。两份 K2.1 JSON 草稿在 `catalog/operations/` 与 `catalog/instruments/`;它们通过结构校验,但不能产生 `Compatible`。AI 预审记录见 [k2.1-review.md](k2.1-review.md);它不是第二人科学审核。K2.2 五操作横向矩阵见 [k2.2-cross-operation-matrix.md](k2.2-cross-operation-matrix.md)。当前 catalog 另有 `sample-aliquot`、`incubation`、`washing` 三份 JSON 草稿,全部仍为 research_draft。

粒度冻结见 [granularity-freeze.md](granularity-freeze.md);候选审查见 [granularity-candidates.md](granularity-candidates.md)、[独立复审](granularity-candidate-review.md);五场景组合见 [granularity-scenarios.md](granularity-scenarios.md)。公共 Catalog 晋升审计见 [catalog-promotion-audit.md](catalog-promotion-audit.md)。

## 已核对的边界

1. **操作 ≠ 仪器**:移液操作不应直接绑定某型号移液器;读板操作不应默认某型号读板机。仪器能力由 K2 profile 匹配。
2. **操作 ≠ 检测**:CCK-8、LIVE/DEAD、ELISA 是方法模板;它们引用操作模块,但不把方法参数写进通用操作默认。
3. **输入/输出必须落到端口**:尤其是洗涤(同一身份)与取样(父/子身份),不能只用 `consumed`/`preserved` 一个布尔。
4. **时间作用域**:液体转移/接种/洗涤/取样是一次操作;孵育、染色、显色是各自独立的 hold 步骤;不能因为 Assay 有 D1/D3 就复制全部步骤。
5. **未知值保留**:所有未从一手来源确认的数值都写 UNKNOWN;不能根据“常规”补值。

## 证据强度

| 文件 | 直接读过的来源 | 仅检索摘要/未核 | 可升级条件 |
|---|---|---|---|
| liquid-transfer | Opentrons S1–S2; ATCC S4; Eppendorf 经 sibling 档案 | ISO 8655 全文;部分 Opentrons 复杂命令页 | 逐页复核 Opentrons 版本;补 ISO 8655-6 条件;实验室 SOP 对照 |
| inoculation-and-incubation | Thermo S1–S3, Corning S4, ATCC S5–S6 | 部分 PDF 页码未逐行;VIOS 仅引用 sibling 档案 | 细胞系 Product Sheet + 实验室 SOP + 培养箱手册逐页 |
| washing-and-sampling | Thermo S1, Arbor S2, Stanford S3 | Abcam/Corning/CSH/FAU S4–S7 仅摘要 | 重新打开 S4–S7;补洗板机/离心机 SOP |
| absorbance-and-fluorescence | Multiskan/Leica 规格; Dojindo/Thermo 协议 web 摘要 | 操作手册、SkanIt/LAS X、逐 lot 染料/抗体 | 逐页手册 + 逐 lot 说明书 + 实验室 SOP |

**当前没有任何文件达到 `SourceChecked`。** `SourceChecked` 至少需要逐字段来源定位 + 第二人复核;`ExpertReviewed` / `LabValidated` 更后。

## 与 K1/K2 的关系

- K1 `OperationModule` 契约提供结构;这些文件是填结构的候选内容。
- K2 `InstrumentProfile` 只描述仪器能力;这些操作文件声明能力要求,不写实际仪器设置。
- K3 编译器以后才能把审核过的操作记录编译为 `ProtocolBlockVersion`;本轮不编译、不建表、不改 CLI。
- 现有 K2 匹配器只用合成测试夹具,没有加载这些 Markdown。

## 下一步

1. 逐份把“直接读过”的字段转成 JSON 草稿,保留 `research_draft`。
2. 对“仅摘要”的来源重新打开并记录页码;不能打开就保持 UNKNOWN。
3. 用 CCK-8 与 LIVE/DEAD 两个完整方法串起操作模块;不自动补参数。
4. 第二人复核后再考虑 `SourceChecked`;本轮不做。
