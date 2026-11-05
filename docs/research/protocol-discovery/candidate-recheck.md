# Candidate Recheck v2 — Domain-Shared Operations

- 日期:2026-11-05
- 状态:`research_draft`;不是 `SourceChecked`;没有 `ReviewRecord`
- 修订原因:上一版把“跨方法复用”错误地等同于“跨实验大类”。领域内共享(同一技术领域、多个独立方法)同样可以进入 Catalog。
- 范围:PCR 热循环、电泳分离、原始流式采集、发光原始采集(板读)。湿实验 only;分析/统计/设门/图像分割排除。

## 1. 三级共享模型

| 层级 | 含义 | 例子 | 运行时 |
|---|---|---|---|
| Global Operation | 跨多个湿实验领域 | 液体转移、分样、受控保持、洗涤、离心 | 公共 OperationModule |
| Domain-Shared Operation | 同一技术领域内多个独立方法复用 | 程序化热循环、流式原始采集、板式发光读取 | 可进入 Catalog,但带领域适用范围 |
| Method-Local Step | 只服务特定方法/变体 | Transwell 上表面清除、dome 点胶、特定染色 | 方法模板内部 |
| ToolAction | 确定性计算/换算 | CFU、活力%、ΔCq、曲线拟合 | 非步骤 |

晋升门槛 G1 改为:**至少两个独立方法在“同一领域或跨领域”中复用**,不再要求跨实验大类。G2/G3/G4 不变。

## 2. Source index

| ID | Source | Read depth |
|---|---|---|
| T1 | NEB Taq PCR protocol E5000S | cycling/reaction table 直接读 |
| T2 | NEB Phusion HF PCR M0530 | cycling table 直接读 |
| T3 | Bio-Rad/QIAGEN qPCR cycling | cycling procedures 直接读 |
| A1 | Herman Lab agarose gel | steps 直接读 |
| A2 | Keiser et al. agarose method | method 直接读 |
| A3 | Rockland/Bio-Rad SDS-PAGE | run/stain steps 直接读 |
| F1 | Fred Hutch flow SOP | acquisition settings 直接读 |
| F2 | HKU FACSAria/NovoCyte SOP | acquisition settings 直接读 |
| L1 | Cayman ATP luminescence insert | plate read 直接读 |
| L2 | Promega Luciferase TB281 | plate read 直接读 |
| L3 | Promega ENLITEN ATP/GloMax | plate read 直接读 |
| I1 | IVIS bioluminescence SOP | 只用于证明 imaging 边界;animal in vivo out-of-scope |

## 3. PCR thermal cycling — domain-shared candidate

**决定:`DomainSharedCandidate`(ThermalProgram),不是永久 method-local。**

- 共享的是“程序化温度阶段 + 持续时间 + 循环结构”;
- 常规 PCR(T1/T2)、qPCR(T3)、RT-PCR 都使用这一温控能力;
- qPCR 的**实时荧光采集**是另一个动作/能力,不能和 thermal program 合并;
- 也不把热循环拆成几十次 `op.incubation`;它不是等温保持。

需要补充的边界:

- 典型 PCR 与 qPCR 是否算两个独立方法?本复审按“同一技术领域内不同方法”接受;若审查者坚持更窄定义,保留为 candidate,不降级为永久 method-local。
- 循环参数仍是 method binding,不设通用程序。
- 采集时点/通道属于 qPCR readout,不属于 thermal program。

## 4. Electrophoresis separation — deferred candidate, boundary re-scoped

**决定:暂缓晋升;把比较边界缩小为“已完成准备的电泳体系 → 施加电场 → 分离完成”。**

上一版把制胶、样品变性、缓冲液、染色/成像全部算进原子,导致 G2/G4 必然失败。新边界下:

- 共享意图:电场驱动分离;
- 琼脂糖 DNA 与 SDS-PAGE 蛋白仍是两个家族,但 sample prep/matrix/stain 属于方法内部;
- 当前不晋升,因为“分离完成”的完成检查、场强/缓冲液、样本状态仍差异大;
- 需要下一轮用更窄边界重新比较;不要把它永久归为 method-local。

## 5. Raw flow acquisition — domain-shared candidate

**决定:`DomainSharedCandidate`(flow cytometry acquisition)。**

- 表面标记、胞内标记、活力、细胞周期/凋亡等是不同 Method Template,但都可能复用同一类“获取事件级原始 FCS”能力;
- 不能因为都使用流式细胞仪就把证据算成一个方法;领域内多方法是有效复用;
- 采集与分选分开:分选产生实际目标细胞群和谱系变化,是另一条分支;
- 设门、补偿、统计是分析,不是采集。

缺口:激光/滤光片/PMT/阈值/流速是 `configuration_dependent`;FCS 原始事件与仪器报告统计量必须分开。

## 6. Luminescence raw acquisition — domain-shared candidate (plate read)

**决定:`DomainSharedCandidate`(luminescence_plate_read),优先级高;不是 gap。**

- ATP 检测、luciferase reporter、dual-luciferase 是不同 Assay,但共享“孔/管积分光信号”的读出物理;
- 不能因为同一检测器就把它们算成一个方法;这恰恰是领域共享;
- 读出边界必须分开:
  - `luminescence_plate_read`:孔/管积分 RLU;
  - `luminescence_image_acquisition`:有空间分辨率的图像(IVIS 等,动物在体 out-of-scope)。
- 与 `ABS_READ` 不合并:无激发/波长,检测器与动力学不同;
- 标准曲线/ATP 浓度/ reporter 活性是下游 ToolAction。

## 7. Summary

| 候选 | 新分类 | 理由 | 下一步 |
|---|---|---|---|
| PCR 热循环 | DomainSharedCandidate | 程序化温控跨 PCR/qPCR/RT 复用;采集另立 | 设计 ThermalProgram 合同 |
| 电泳分离 | Deferred Candidate | 边界缩小后仍差异大;不永久拒绝 | 用“准备完成→电场→分离”重审 |
| 原始流式采集 | DomainSharedCandidate | 同一流式领域多方法复用 raw FCS | 设计 FlowAcquisition 合同;分选分离 |
| 发光板读 | DomainSharedCandidate | ATP/reporter 共享积分光读出;图像另立 | 设计 LuminescencePlateRead 合同 |

## 8. 不改变的事实

- 没有任何新公共模块已实现;这些都是研究候选。
- 没有 JSON、没有 Runtime、没有 Slint/CLI/DB 变更。
- 没有 `SourceChecked`;第二人复核仍未完成。
- 全局操作(transfer/incubation/washing/centrifugation/absorbance/raw image)不变;领域共享候选是新增分类,不是新增三个运行时。
