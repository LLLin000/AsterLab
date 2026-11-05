# 操作与仪器覆盖账本

- 日期:2026-11-05
- 状态:研究覆盖账本;所有条目仍为 `research_draft`,没有 `SourceChecked`。
- 规范:[record-standard.md](record-standard.md)
- 用途:知道“哪些对象有记录、来源强度如何、是否已有 JSON、下一步缺什么”,避免把文件数量当成覆盖完成。
- 本轮边界:体外/细胞/微生物/分子湿实验;临床、动物在体、统计/生物信息/图像分析不在本轮。原始读数可记录,分析/结论不进入操作模块。

## 1. 操作记录

| 操作/方法 | 记录 | 来源强度 | JSON | 主要缺口 |
|---|---|---|---|---|
| 液体转移 | [liquid-transfer.md](k2-operations/liquid-transfer.md) | 部分直接读;ISO 8655 仅二手 | `catalog/operations/liquid-transfer.v1.json` | 实际量程/校准/液体性质;条件分支执行 |
| 取样/分样 | [washing-and-sampling.md](k2-operations/washing-and-sampling.md) | S1–S3 直接读;S4–S7 摘要 | `sample-aliquot.v1.json` | 父/子量守恒、谱系 ID 生成 |
| 孵育 | [inoculation-and-incubation.md](k2-operations/inoculation-and-incubation.md) | 直接读;部分 PDF 页码未逐行 | `incubation.v1.json` | 实际开始/结束、状态变化类型 |
| 洗涤 | [washing-and-sampling.md](k2-operations/washing-and-sampling.md) | S1–S3 直接读 | `washing.v1.json` | 样本损失 effect、残留量 |
| 吸光度测量 | [absorbance-and-fluorescence-readout.md](k2-operations/absorbance-and-fluorescence-readout.md) | 仪器规格直接读;Assay 部分摘要 | `absorbance-measurement.v1.json` | Assay 空白/曲线的层级、线性范围 |
| 荧光成像 | [absorbance-and-fluorescence-readout.md](k2-operations/absorbance-and-fluorescence-readout.md) | 仪器规格直接读 | 未单独 JSON | 通道/物镜/配置;活/固定样本差异 |
| 细胞传代/解离 | [cell-culture-and-separation.md](k2-operations/cell-culture-and-separation.md) | S1/S2/S5/S6 直接读;Corning 摘要 | 未单独 JSON | 传代是 composite block;计数/离心可拆成原子操作;细胞系差异/解离终点 |
| 细胞计数/活力 | 同上 | 直接读 | 未单独 JSON | 计数方法/范围/聚集 |
| 离心 | 同上 | 直接读 | 未单独 JSON | 转子/RCF/适配器 |
| 冻存/存储 | 同上 | 直接读 | 未单独 JSON | 降温速率、冻存液、 thaw |
| ELISA | [elisa-and-western-blot.md](k2-operations/elisa-and-western-blot.md) | E1–E3 直接读;hook 效应摘要 | 未单独 JSON | 抗体对、标准曲线、基质/稀释 |
| Western blot | 同上 | W1–W2 直接读;滴定摘要 | 未单独 JSON | 抗体特异性、上样对照、曝光 |
| qPCR / 核酸 QC | [qpcr-and-nucleic-acid-qc.md](k2-operations/qpcr-and-nucleic-acid-qc.md) | MIQE 直接读;部分摘要 | 未单独 JSON | 引物/效率/参考基因/阈值 |
| 流式 | [flow-cytometry-and-immunofluorescence.md](k2-operations/flow-cytometry-and-immunofluorescence.md) | T1–T5 直接读;B1/F1 摘要 | 未单独 JSON | panel/补偿/FMO/分选 |
| 免疫荧光(固定) | 同上 | T1–T5 直接读 | 未单独 JSON | 抗体/通道/曝光/配置 |
| 生物膜 CV/CFU | [biofilm-cv-cfu.md](k2-operations/biofilm-cv-cfu.md) | S1–S3 直接读;S4/S5 摘要 | **不应作为单独 JSON** | 是 assay model + 两个 method template;见 [粒度判断](k2-operations/granularity-and-biofilm.md) |
| 划痕/伤口愈合 | [wound-healing-and-migration.md](k2-operations/wound-healing-and-migration.md) | P1–P4 直接读;部分 blocked | 无 | 同视野 ID、增殖混杂、时间锚点;方法模板 |
| 活力/细胞毒性 | [viability-and-cytotoxicity-methods.md](k2-operations/viability-and-cytotoxicity-methods.md) | S1–S3/S6–S9 直接读;S4/S5 摘要 | 无 | CCK-8/LDH/LIVE-DEAD 方法绑定;通用读数已有 |
| 3D/类器官 | [three-d-and-organoid-operations.md](k2-operations/three-d-and-organoid-operations.md) | S1–S10 摘要/未读 | 无 | 基质/球体/换液;无稳定原子操作;需实验室 protocol |

## 2. 仪器记录

| 类别/型号 | 记录 | 来源强度 | JSON | 主要缺口 |
|---|---|---|---|---|
| 手动移液器 Research plus | [manual-pipette...](k2-instruments/manual-pipette-eppendorf-research-plus.md) | 手册镜像直接读 | 单 variant JSON | 全系列拆分;当前性能表 |
| CO₂ 培养箱 Heracell VIOS 160i | [co2-incubator...](k2-instruments/co2-incubator-heracell-vios-160i.md) | 手册/规格直接读 | 无 | 可选 O₂/门/安装状态 |
| 吸光度读板机 Multiskan SkyHigh | [plate-reader...](k2-instruments/plate-reader-multiskan-skyhigh.md) | 规格表直接读 | 1 profile JSON | 软件/校准/振荡/动力学 |
| 荧光显微镜 Leica DMi8 | [microscope...](k2-instruments/microscope-leica-dmi8.md) | 规格/FAQ/手册直接读 | 无 | 物镜/滤光片/相机/光源配置 |
| 离心机 5424 R | [centrifuge-cell-counter-washer](k2-instruments/centrifuge-cell-counter-washer.md) | 全部 search-summary-only | 无 | 转子/RCF/适配器需逐页复核 |
| 细胞计数仪 Countess 3 | 同上 | search-summary-only | 无 | 量程/耗材/软件 |
| 洗板机 BioTek 405 TS | 同上 | search-summary-only | 无 | manifold/板型/残留 |
| qPCR QuantStudio 5 | [qpcr-flow-spectro-thermomixer](k2-instruments/qpcr-flow-spectro-thermomixer.md) | QS 部分摘要;手册 403 | 无 | block/体积/通道/软件 |
| 流式 FACSCanto II | 同上 | T1–T5 直接读;部分摘要 | 无 | 激光/滤光片/panel/分选 |
| 分光光度计 NanoDrop One | 同上 | 摘要 | 无 | 微量/检测限/blank |
| 恒温混匀仪 ThermoMixer C | 同上 | 摘要;手册 403 | 无 | 温度/转速/加热块 |
| 自动液体处理 | [automated-liquid-handling.md](k2-instruments/automated-liquid-handling.md) | 平台手册/部分摘要 | 无 | 配置/labware/tip/deck/校准 |

## 2.1 公共 Catalog 决策

- [Catalog Promotion Audit](k2-operations/catalog-promotion-audit.md) 把候选分为 shared/method-local/composite/method/tool。
- 第一版公共操作：转移、分样、孵育、洗涤（preserving + lossy）、离心、吸光度读取、荧光原始采集、涂布接种、细胞浓度计数、划线接种（窄例外）。
- 方法内部步骤：CAND-GAP-CREATE、CAND-APICAL-CLEAR、3D dome、ULA 球体、固定/通透/封闭、通用染色、解离等。
- 不因原子性自动建公共模块。

## 3. 覆盖判断

- **操作覆盖**:主要细胞培养、分离、免疫、分子、成像、流式、生物膜、划痕、活力/毒性、3D/类器官；动物/临床与图像分析/统计按本轮范围排除。
- **仪器覆盖**:移液、培养、读板、显微、离心、计数、洗板、qPCR、流式、分光、混匀、自动液体处理；仍未覆盖:成像分析软件、质谱/测序。
- **JSON 覆盖**:只有 5 个操作 + 2 个仪器 profile;其余仍是研究记录。
- **证据强度**:多数仪器 5424R/Countess/405TS 是 search-summary-only;不能因为“有文件”就认为可核验。
- **审核**:没有 `ReviewRecord`;没有第二人逐字段复核;没有 `SourceChecked`。

## 3.1 不在本轮范围

- 临床研究/受试者/流行病学。
- 动物在体实验(需单独立项,不能套用孔板操作)。
- 图像分割、统计检验、生物信息流程、结果解释。
- 这些可以引用原始读数,但不是 OperationModule。

## 4. 下一批优先级

1. 将 5424R/Countess/405TS 的来源从摘要升级为手册逐页。
2. 为荧光成像、细胞传代、计数、离心建立独立 JSON 草稿(仍 research_draft)。
3. 把 ELISA/WB/qPCR 的通用操作与 Assay 绑定分开,避免方法记录变成新运行时。
4. 划痕/伤口愈合已建方法模板;动物/临床与图像分析/统计按本轮范围排除,不硬套孔板模型。
5. 所有升级前先做第二人复核;AI 预审不算。

## 5. 账本维护规则

- 新增记录必须在本表登记:路径、来源强度、JSON、缺口。
- `search-summary-only` 不能升级为 `SourceChecked`。
- 没有来源的数值留在 UNKNOWN,不写成 0 或默认。
- 一个文件覆盖多个对象时,对象必须有独立 ID;JSON 仍按对象拆分。
