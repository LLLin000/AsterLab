# Protocol Discovery — 公共操作覆盖矩阵

- 日期:2026-11-05
- 状态:基于 B01–B22 与既有记录的研究归纳;所有来源仍为 `research_draft`,没有 `SourceChecked`
- 阅读状态:Research Shared(研究有复用)、Catalog Draft(已有 JSON)、SourceChecked(已复核)必须分开;当前没有 SourceChecked
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
| B07 | RT-qPCR | Thermo/Bio-Rad/ABI 手册直接读 | RT/qPCR 配制、热循环、原始荧光采集均 method-local/template;无新公共模块 |
| C01 | 候选复审 | NEB/Phusion/Bio-Rad/琼脂糖/SDS-PAGE/流式/发光手册 | PCR 热循环、流式采集、发光板读为 domain-shared candidates;电泳 deferred;不是永久 method-local |
| B08 | 基础配液/无菌 | Thermo/Morimoto/MP Bio/protocols.io 直接读 | 称量 ToolAction;溶解/pH/过滤/灭菌 method-local;过滤/灭菌 deferred domain-shared;无新模块 |
| B09 | 转染/稳定株 | Thermo/Addgene/Bio-Rad 直接读 | 脂质/PEI/电转分开;复合物形成复用 transfer+incubation;筛选/单克隆为门控;无新模块 |
| B10 | 微生物生长/转化/药敏 | 教学/Curr Protoc/Addgene/CLSI SOP 直接读 | OD600 raw read;热激/药敏判读 method-local/ToolAction;无新模块 |
| B11 | 3D 球体/悬滴/ALI | ULA/悬滴/Spheroid sheet/ALI 来源直接读 | ULA/悬滴/ALI 不合并;换液 lossy;无新模块 |
| B12 | 活细胞成像/死亡/周期 | Thermo/JoVE/facility 直接读 | raw acquisition 复用;染色/固定/通透/设门 method-local;无新模块 |
| B13 | 蛋白纯化/IP | RIPA/BCA/Bradford/Ni-NTA/IP/透析 直接读 | 制备链 method-local/composite;亲和 vs IP 分开;无新模块 |
| B14 | 分子克隆 | QIAGEN/NEB 等直接读 | 克隆链 composite/method-local;无新模块 |
| B15 | 原代细胞 | 组织解离/SVF/骨髓/原代接种直接读 | 原代制备 method-local/composite;无新模块 |
| B16 | CRISPR/病毒转导 | 转染/转导/单克隆大纲直接读 | 递送方式分开;无新模块 |
| B17 | 细胞–材料 | 溶血/浸提/共培养/黏附直接读 | 方法模板;无新模块 |
| B18 | 组织学/IHC | 固定/包埋/切片/染色/成像直接读 | method-local/template;无新模块 |
| B19 | 类器官分化 | 脑/肠类器官分化直接读 | composite/method template;无新模块 |
| B20 | EV/亚细胞 | UC/SEC/沉淀/组分分离直接读 | 方法模板;无新模块 |
| B21 | 报告/ROS/线粒体/钙 | 双读数/探针/Seahorse/钙成像直接读 | 读数方法绑定;发光板读 domain candidate;无新模块 |
| B22 | 细胞系 QC | 支原体/STR/无菌/内毒素直接读 | 方法模板/ToolAction;无新模块 |
| 既有 | 生物膜、3D 类器官、ELISA/WB、qPCR、流式/IF、划痕/Transwell | 见 k2-operations | 方法模板为主;Bio-protocol 4298 补足 3D dome |

## 2.1 状态分层

| 状态 | 操作 |
|---|---|
| Catalog Draft | liquid_transfer、sample_aliquot、incubation、washing、absorbance_measurement(5 份 JSON) |
| Global Research Shared, no JSON | 离心、原始荧光采集、细胞浓度计数、划线/涂布、菌落计数 |
| Domain-Shared Promoted (research) | ThermalProgram、FlowAcquisition、LuminescencePlateRead |
| Domain-Shared Deferred | Electrophoresis、SterileFiltration;Autoclave method-local |
| Method-local / template | 固定/通透/封闭/染色、ELISA/WB 变体、qPCR 热循环具体程序、3D 球体形成 |
| Gap | 3D 其他模型、transfection、microbial growth/transformation、molecular cloning |

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
| PCR / RT-qPCR | 分子扩增 | method-local/template | 热循环是方法内程序;qPCR 加原始荧光采集;不拆多次 incubation |
| 琼脂糖电泳 | 分子电泳 | method-local | 上样/电泳/成像;无新公共模块 |
| ELISA/WB | 蛋白检测 | method template | 变体不合并;发光读数为缺口 |
| CCK-8/LDH/LIVE-DEAD | 细胞检测 | method template | 读数不可互换 |
| 划痕/Transwell | 迁移 | method template | gap/apical 留 method-local |
| 3D dome/球体 | 3D/类器官 | method/composite | dome 点胶/球体形成 method-local |

## 3. 判定结果

- B01–B22 没有新增 **global** 公共 OperationModule;C01 把 PCR 热循环、流式采集、发光板读列为 domain-shared candidates,电泳 deferred;B08 把过滤/灭菌列为 deferred domain-shared。
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
| 分子基础 | B04 覆盖提取/PCR/电泳;B07 已按两表审计 RT-qPCR;热循环/原始荧光仍为方法内 |
| 转染/稳定株 | B09 已覆盖脂质/PEI/电转/筛选/单克隆;均为 method-local/composite |
| 微生物生长/药敏 | B10 已覆盖 OD600、转化、MIC/MBC、纸片法;判读为 ToolAction |
| 3D 其他模型 | B11 覆盖 ULA/悬滴/Spheroid sheet/ALI;无新模块 |
| 活细胞成像/死亡/周期 | B12 覆盖 Annexin V/TUNEL/EdU/PI/延时;原始采集复用 |
| 蛋白纯化/IP | B13 覆盖裂解/定量/亲和/IP/透析;无新模块 |
| 分子克隆 | B14 覆盖质粒/酶切/连接/转化/胶回收;无新模块 |
| 原代细胞 | B15 覆盖组织解离/SVF/骨髓/原代接种;无新模块 |
| CRISPR/病毒 | B16 覆盖 RNP/慢病毒/AAV/单克隆;无新模块 |
| 细胞–材料 | B17 覆盖溶血/浸提/共培养/黏附;无新模块 |
| 组织学/IHC | B18 覆盖固定/包埋/切片/染色/成像;无新模块 |
| 类器官分化 | B19 覆盖脑/肠类器官;无新模块 |
| EV/亚细胞 | B20 覆盖 UC/SEC/沉淀/组分;无新模块 |
| 报告/ROS/线粒体/钙 | B21 覆盖发光/荧光/Seahorse/钙;无新模块 |
| 细胞系 QC | B22 覆盖支原体/STR/无菌/内毒素;无新模块 |
| 蛋白检测 | B05 已按 discovery 表审计 ELISA/WB;发光原始读数仍缺公共模块 |
| 流式/IF | B06 已按 discovery 表格审计;湿实验步骤已记录,分析/设门 excluded;配置仍需来源复核 |
| 动物/临床 | 按本轮范围排除 |
| 图像分析/统计 | 按本轮范围排除;只记录原始图像 |
| 3D/类器官 | 只有 Bio-protocol 4298 的 Matrigel dome;球体/其他模型不足 |
| 来源等级 | 多数仪器/操作仍 `agent-verified` 或 `search-summary-only`,没有第二人复核 |

## 5. 下一批优先

1. 3D 球体/类器官其他模型(现有 Bio-protocol 4298 只覆盖 Matrigel dome)。
2. 发光原始采集的独立 readout 合同(ELISA/ATP/WB chemi;不并入 absorbance)。
3. 继续从 Morimoto/ASM/Bio-protocol 选基础 protocol;不要按网站扫完。
4. 所有新候选先查公共操作复用,不新增模块,除非过晋升门槛。

## 6. 指标

- Protocol 覆盖广度:22 个 discovery 批次;已覆盖常规细胞/微生物/分子/蛋白/流式/IF/3D/类器官/克隆/原代/CRISPR/材料/组织学/EV/报告基因/QC。
- Operation 复用成熟度:transfer/incubation/washing/centrifugation/absorbance/raw image 已跨 ≥5 家族;streak/spread/colony count 跨微生物与 CFU;domain-shared candidates 已单列,未进入 JSON。
- 不以步骤数/模块数衡量。
