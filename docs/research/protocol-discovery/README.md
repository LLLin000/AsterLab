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

## 来源边界

- 原始 Protocol 是步骤/参数事实依据;
- 现有 AsterLab 记录只用于 Reuse Map;
- 大学 lab protocol 不等于普适标准;记录适用范围;
- 所有记录保持 `research_draft`;AI 预审不等于第二人科学审核。
