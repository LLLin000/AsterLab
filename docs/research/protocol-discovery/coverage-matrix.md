# Protocol Discovery — 公共操作覆盖矩阵

- 日期:2026-11-05
- 状态:基于 B01–B03 与既有记录的研究归纳;所有来源仍为 `research_draft`,没有 `SourceChecked`
- 用途:看“哪些操作真的跨方法复用”,不是看“研究了多少篇”

## 1. 已研究批次

| 批次 | 领域 | 来源类型 | 主要结论 |
|---|---|---|---|
| B01 | 基础细胞培养 | Morimoto PDF 直接读 | 传代是 composite;血球计数是 readout;台盼蓝是 method template;冻存/复苏是 composite;无新公共模块 |
| B02 | 微生物基础 | ASM PDF / Addgene / Curr Protoc 直接读 | 划线、涂布分开;连续稀释是 composite;CFU 计算是 ToolAction;无新公共模块 |
| B03 | 细胞功能检测 | Dojindo/Promega/Thermo/Frontiers/Springer 直接读 | CCK-8/LDH/LIVE-DEAD 是方法模板;scratch/Transwell 的 gap-create/apical-clear 留 method-local;无新公共模块 |
| B04 | 分子基础 | QIAGEN/Thermo/NEB/Faircloth 直接读 | 提取/ PCR / 电泳均 method-local/composite;无新公共模块 |
| B05 | 蛋白检测 | Thermo/Bio-Rad 直接读 | ELISA/WB 是方法模板;发光读数为缺口;无新公共模块 |
| B06 | 流式/IF | Thermo 多份 protocol 直接读 | 染色/固定/通透/设门均 method-local/template;原始采集复用;无新公共模块 |
| 既有 | 生物膜、3D 类器官、ELISA/WB、qPCR、流式/IF、划痕/Transwell | 见 k2-operations | 方法模板为主;Bio-protocol 4298 补足 3D dome |

## 2. 公共操作复用

| 公共操作 | 独立方法家族证据 | 决定 | 主要缺口 |
|---|---|---|---|
| `op.liquid_transfer` | 细胞培养、CCK-8、LDH、LIVE/DEAD、划痕、Transwell、微生物、ELISA、3D | shared | 条件模式执行/体积谱系 |
| `op.sample_aliquot` | 细胞上清、LDH harvest、微生物挑菌、qPCR/样本分装 | shared(条件) | 父/子量守恒、ID 生成 |
| `op.incubation` | 细胞培养、微生物、染色、显色、迁移、冻存 | shared | elapsed only;生长/迁移/成胶不断言 |
| `op.washing` | 细胞培养、ELISA、IF、生物膜、划痕、3D | shared + lossy variant | 损失量/残留体积 |
| `op.centrifugation` | 传代、计数、冻存/复苏、Transwell、分子 | shared | rpm→RCF/转子 |
| `ABS_READ` | CCK-8、LDH、ELISA、CV | shared | 原始 OD;线性/空白由方法 |
| `FLUO_IMAGE` | LIVE/DEAD、划痕、Transwell、IF、3D | shared(raw) | 通道/配置;分析不在范围 |
| 细胞浓度计数 | 传代、计数、Transwell、划痕 | shared(readout) | 计数≠活力;计数板/规则 |
| 划线接种 | 微生物分离、纯度/复苏 | shared(narrow) | 单菌落判读定性 |
| 涂布接种 | 微生物计数、生物膜 CFU | shared(variant) | 均匀性/Track 分支 |
| 菌落计数 | 涂布/稀释/CFU | shared(readout) | 30–300 vs 20–300 冲突 |
| 连续稀释 | 微生物 CFU、细胞给药 | composite | 多次 transfer;不是原子 |
| 生物膜 CV/CFU | 生物膜 | method template | 不是 op;lossy wash |
| DNA/RNA 提取 | 分子提取 | method-local/composite | 柱式/相分离不可合并;离心/转移复用 |
| PCR | 分子扩增 | method-local | 热循环是方法内程序,不拆多次 incubation |
| 琼脂糖电泳 | 分子电泳 | method-local | 上样/电泳/成像;无新公共模块 |
| ELISA/WB | 蛋白检测 | method template | 变体不合并;发光读数为缺口 |
| CCK-8/LDH/LIVE-DEAD | 细胞检测 | method template | 读数不可互换 |
| 划痕/Transwell | 迁移 | method template | gap/apical 留 method-local |
| 3D dome/球体 | 3D/类器官 | method/composite | dome 点胶/球体形成 method-local |

## 3. 判定结果

- B01–B03 没有发现需要新增的公共 OperationModule。
- 公共操作集合目前趋于稳定:transfer、aliquot、incubation、washing、centrifugation、absorbance、raw image、cell count、streak、spread、colony count。
- 大量“看起来像操作”的步骤实际是:
  - composite:传代、连续稀释、冻存/复苏、条件培养基;
  - method template:CCK-8、LDH、LIVE-DEAD、ELISA、WB、qPCR、CV/CFU、3D dome;
  - method-local:gap-create、apical-clear、dome 点胶、机械解离、固定/通透/封闭、通用染色;
  - tool action:CFU、活力%、闭合度、ΔCq、剂量拟合、板容量;
  - custom step:人工判断/新方法。

## 4. 覆盖缺口

| 缺口 | 说明 |
|---|---|
| 分子基础 | B04 已覆盖 DNA/RNA 提取、PCR、电泳;RT-qPCR 的湿实验仍已有 qPCR 记录,未按 B04 两表审计 |
| 蛋白检测 | B05 已按 discovery 表审计 ELISA/WB;发光原始读数仍缺公共模块 |
| 流式/IF | B06 已按 discovery 表格审计;湿实验步骤已记录,分析/设门 excluded;配置仍需来源复核 |
| 动物/临床 | 按本轮范围排除 |
| 图像分析/统计 | 按本轮范围排除;只记录原始图像 |
| 3D/类器官 | 只有 Bio-protocol 4298 的 Matrigel dome;球体/其他模型不足 |
| 来源等级 | 多数仪器/操作仍 `agent-verified` 或 `search-summary-only`,没有第二人复核 |

## 5. 下一批优先

1. RT-qPCR 的 discovery 审计(已有 qPCR 研究,但未按 B04 两表格式)。
2. 3D 球体/类器官其他模型(现有 Bio-protocol 4298 只覆盖 Matrigel dome)。
3. 继续从 Morimoto/ASM/Bio-protocol 选基础 protocol;不要按网站扫完。
4. 所有新候选先查公共操作复用,不新增模块,除非过晋升门槛。

## 6. 指标

- Protocol 覆盖广度:目前 6 个 discovery 批次 + 既有方法记录;基础细胞、微生物、细胞检测、分子、蛋白、流式/IF 已覆盖。
- Operation 复用成熟度:transfer/incubation/washing/centrifugation/absorbance/raw image 已跨 ≥5 家族;streak/spread/colony count 跨微生物与 CFU;新候选为零;luminescence raw read 是缺口。
- 不以步骤数/模块数衡量。
