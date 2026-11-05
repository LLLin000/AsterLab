# Protocol-driven Operation Discovery

- 日期:2026-11-05
- 目标:从真实、可追溯的湿实验 Protocol 中归纳出覆盖广、复用率高、粒度合理的基础操作库;不是开发 Protocol 编辑器或批量生成 JSON。
- 循环:搜集 → 拆解 → 复用 → 跨实验归纳
- 规范:[record-standard](../record-standard.md)、[granularity-freeze](../k2-operations/granularity-freeze.md)、[catalog promotion audit](../k2-operations/catalog-promotion-audit.md)

## 每篇 Protocol 只交两张表

### 表 1:步骤拆解

| 原文步骤 | 操作语义(现有模块/候选) | 输入 → 输出 | 参数归属 | 来源定位 |
|---|---|---|---|---|
| 原文步骤号/短句 | `op.liquid_transfer` / `method_local` / `new_candidate` / ... | 样本/容器/数据 | 方法绑定/参数槽/UNKNOWN | PDF 页码/步骤号 |

规则:

- 忠于原文,不补写原文没有的步骤或数值;
- 一个原文步骤可映射多个操作,多个动作也可合并为一个科学操作;
- 原文没写 → UNKNOWN;
- 已有模块只写身份和差异,不重复 PortSpec/Transition 全文。

### 表 2:公共操作复用

| 公共操作候选 | 出现过的独立方法家族 | 处理决定 | 证据/缺口 |
|---|---|---|---|
| 液体转移 | 细胞培养、CCK-8、ELISA、CFU | shared | 至少 2 个方法家族 |
| 划痕形成 | Scratch | method_local | 只服务一个方法 |

## 每批交付

1. 来源索引(新增来源、领域、读取程度)。
2. 每篇 Protocol 的步骤拆解表。
3. Reuse Map 汇总。
4. 跨方法公共操作覆盖矩阵。
5. 缺口清单(不能表达的样本变化/设备条件/方法分支)。

## 进度指标

- **Protocol 覆盖广度**:研究了哪些实验家族,是否遗漏常见湿实验类型。
- **Operation 复用成熟度**:每个公共操作被多少独立方法采用,特殊分支是否过多。

不以“步骤数”或“新建模块数”衡量进度。

## 批次索引

| 批次 | 文件 | 范围 | 状态 |
|---|---|---|---|
| B01 | [batch-01-morimoto-cell-culture.md](batch-01-morimoto-cell-culture.md) | Morimoto 基础细胞培养 | 已完成;research_draft |
| B02 | [batch-02-microbiology-basics.md](batch-02-microbiology-basics.md) | 划线/涂布/液体培养/连续稀释/菌落计数 | 已完成;research_draft |
| B03 | [batch-03-cell-assays.md](batch-03-cell-assays.md) | CCK-8 / LDH / LIVE-DEAD / scratch / Transwell | 已完成;research_draft |
| B04 | [batch-04-molecular-basics.md](batch-04-molecular-basics.md) | DNA/RNA 提取、PCR、电泳 | 已完成;research_draft |
| B05 | [batch-05-protein-assays.md](batch-05-protein-assays.md) | ELISA / Western blot | 已完成;research_draft |
| B06 | [batch-06-flow-and-if.md](batch-06-flow-and-if.md) | 流式 / 免疫荧光 | 已完成;research_draft |
| B07 | [batch-07-rt-qpcr.md](batch-07-rt-qpcr.md) | 两步/一步 RT-qPCR 湿实验 | 已完成;research_draft |
| C01 | [candidate-recheck.md](candidate-recheck.md) | 热循环/电泳/流式采集/发光采集复审 | 已完成;3 个 domain-shared candidate + 电泳 deferred |
| B08 | [batch-08-solutions-and-sterilization.md](batch-08-solutions-and-sterilization.md) | 溶液/缓冲液/培养基/无菌处理 | 已完成;零新公共模块 |
| B09 | [batch-09-transfection-and-stable-lines.md](batch-09-transfection-and-stable-lines.md) | 转染/稳定株 | 已完成;零新公共模块 |
| B10 | [batch-10-microbial-growth-transformation-susceptibility.md](batch-10-microbial-growth-transformation-susceptibility.md) | 微生物生长/转化/药敏 | 已完成;零新公共模块 |
| B11 | [batch-11-3d-spheroid-models.md](batch-11-3d-spheroid-models.md) | 3D 球体/悬滴/ALI | 已完成;零新公共模块 |
| B12 | [batch-12-live-imaging-and-cell-death-cycle.md](batch-12-live-imaging-and-cell-death-cycle.md) | 活细胞成像/死亡/周期 | 已完成;零新公共模块 |
| B13 | [batch-13-protein-purification-and-ip.md](batch-13-protein-purification-and-ip.md) | 蛋白纯化/IP | 已完成;零新公共模块 |
| B14 | [batch-14-molecular-cloning.md](batch-14-molecular-cloning.md) | 分子克隆链 | 已完成;零新公共模块 |
| B15 | [batch-15-primary-cells-and-dissociation.md](batch-15-primary-cells-and-dissociation.md) | 原代细胞/组织解离 | 已完成;零新公共模块 |
| B16 | [batch-16-crispr-and-viral-transduction.md](batch-16-crispr-and-viral-transduction.md) | CRISPR/病毒转导 | 已完成;零新公共模块 |
| B17 | [batch-17-cell-material-interaction.md](batch-17-cell-material-interaction.md) | 细胞–材料相互作用 | 已完成;零新公共模块 |
| B18 | [batch-18-histology-and-ihc.md](batch-18-histology-and-ihc.md) | 组织学/IHC | 已完成;零新公共模块 |
| B19 | [batch-19-organoid-differentiation.md](batch-19-organoid-differentiation.md) | 类器官分化 | 已完成;零新公共模块 |
| B20 | [batch-20-extracellular-vesicles-and-subcellular-fractionation.md](batch-20-extracellular-vesicles-and-subcellular-fractionation.md) | EV/亚细胞组分 | 已完成;零新公共模块 |
| B21 | [batch-21-reporter-ros-mitochondria-calcium.md](batch-21-reporter-ros-mitochondria-calcium.md) | 报告基因/ROS/线粒体/钙 | 已完成;零新公共模块 |
| B22 | [batch-22-cell-line-qc.md](batch-22-cell-line-qc.md) | 细胞系 QC | 已完成;零新公共模块 |

## Catalog Promotion Review v2

见 [promotion-review/](promotion-review/):ThermalProgram、FlowAcquisition、LuminescencePlateRead 晋升为 domain-shared;Electrophoresis、SterileFiltration deferred;Autoclave 仍 method-local。不是 JSON/Runtime。

## Post-B22 结论

见 [post-b22-review.md](post-b22-review.md):常规湿实验覆盖已完成;下一步转 Catalog Promotion Review v2,不再扩宽度。

## 剩余实验地图

见 [gap-map.md](gap-map.md):P0(3D 其他模型、转染、微生物转化/生长、分子克隆)、P1(蛋白纯化/IP、活细胞成像、死亡/周期、原代细胞、药敏)、P2(报告基因、复杂 3D、测序/质谱)。公共能力缺口单列:luminescence raw read、fluorescence plate read、FACS raw acquisition。

## 一致性审查

见 [catalog-consistency-audit.md](catalog-consistency-audit.md):Research Shared / Catalog Draft / SourceChecked 分开;五份 JSON 的 incubation、washing、absorbance 已按研究范围修订。

## 来源边界

- 原始 Protocol 是步骤/参数事实依据;
- 现有 AsterLab 记录只用于 Reuse Map;
- 大学 lab protocol 不等于普适标准;记录适用范围;
- 所有记录保持 `research_draft`;AI 预审不等于第二人科学审核。
