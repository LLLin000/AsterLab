# Protocol Discovery — 剩余实验拆解地图

- 日期:2026-11-05
- 状态:基于 B01–B07、既有操作/方法记录与覆盖矩阵的规划;不是新的 Protocol 来源审计
- 目的:回答“还有哪些实验需要拆解”,并区分“实验家族缺口”和“公共能力缺口”

## 1. 已覆盖的实验家族

| 家族 | 已做 | 结论 |
|---|---|---|
| 基础细胞培养 | B01 | 传代/计数/台盼蓝/冻存;composite + readout |
| 微生物基础 | B02 | 划线/涂布/液体培养/稀释/计数;CFU 是 ToolAction |
| 细胞功能检测 | B03 | CCK-8/LDH/LIVE-DEAD/scratch/Transwell;method template |
| 分子基础 | B04 | DNA/RNA 提取、PCR、电泳;method-local/composite |
| 蛋白检测 | B05 | ELISA/WB;method template |
| 流式/IF | B06 | 湿实验步骤已拆;分析/设门 excluded |
| RT-qPCR | B07 | RT/qPCR/热循环/原始荧光;method-local/template |
| 3D 类器官 | Bio-protocol 4298 | 仅 Matrigel dome;其他 3D 模型未覆盖 |
| 生物膜 | biofilm 记录 | CV/CFU 方法模板;不是原子操作 |

## 2. 仍需拆解的实验家族

优先级按“常用 + 与已有操作差异大 + 可能影响公共库”排序,不是文献数量。

### P0 — 下一批优先

| 实验家族 | 具体实验 | 为什么需要 | 预计拆解结果 |
|---|---|---|---|
| 3D 其他模型 | ULA 球体、悬滴、Spheroid sheet、air-liquid interface | 目前只有 Matrigel dome;2D 假设不能直接复用 | method template + composite;dome/球体形成 method-local;换液 lossy |
| 转染与稳定株 | 脂质体/PEI/电转、筛选、单克隆 | 基础细胞操作,当前未覆盖;影响后续 reporter/knockdown | composite + method-local(转染试剂/筛选条件);液体转移/孵育/洗涤复用 |
| 微生物液体生长与转化 | 生长曲线/OD600、感受态、热激转化、抗性筛选 | B02 只有平板/液体接种,没有转化与生长曲线 | composite + method-local(感受态/热激);OD600 raw read 复用 absorbance |
| 分子克隆基础 | 质粒提取、限制酶切、连接、转化、凝胶回收 | B04 只有提取/PCR/电泳,没有克隆链 | composite + method-local;离心/转移/孵育复用;无新原子 |

### P1 — 第二批

| 实验家族 | 具体实验 | 为什么需要 | 预计拆解结果 |
|---|---|---|---|
| 蛋白纯化/IP | 裂解、BCA/Bradford、亲和/免疫沉淀、透析/浓缩 | B05 只有 ELISA/WB,没有制备链 | composite + method-local(裂解/纯化);离心/孵育/转移复用 |
| 活细胞/延时成像 | 活细胞工作站、time-lapse、confocal | B06 只有固定 IF;活细胞采集/环境控制不同 | method template;raw image acquisition 复用;环境控制 method-local |
| 细胞死亡/周期 | Annexin V、TUNEL、EdU/BrdU、细胞周期流式 | B03 只有活力/毒性;死亡机制/周期不同 | method template + flow acquisition;染色/分析 method-local |
| 原代细胞/组织解离 | 组织消化、过滤、红细胞裂解、原代培养 | B01 只有细胞系;原代样本制备不同 | composite + method-local;离心/洗涤/计数复用 |
| 微生物药敏 | MIC/MBC、纸片扩散、时间杀菌 | B02 只有计数;药敏有方法分支 | method template;incubation/OD/CFU 复用 |

### P2 — 以后

| 实验家族 | 具体实验 | 说明 |
|---|---|---|
| 报告基因/发光 | luciferase、ATP、ROS、mitochondrial | 先补 luminescence raw read;方法模板另行 |
| 类器官分化/复杂 3D | 多谱系分化、organ-on-chip、bioprinting | 高变异、配置依赖;先不批量 |
| 测序/质谱 | NGS、mass spec | 数据/组学为主,本轮范围外 |
| 动物/临床 | in vivo/临床 | 本轮明确排除 |

## 3. 公共能力缺口(不是实验)

这些不是“再拆一个实验”,而是已有方法反复依赖、但公共库还没有稳定表达的能力:

| 缺口 | 影响 | 建议 |
|---|---|---|
| Luminescence raw read(plate) | ELISA chemi、ATP、luciferase、WB chemi | 独立 readout 合同;不与 absorbance/imaging 合并 |
| Fluorescence plate read(非成像) | 荧光板读数、部分 ELISA/报告基因 | 与 FLUO_IMAGE 分开;端口/样本消耗不同 |
| FACS raw acquisition | 流式表面/胞内/活力 | method template 内步骤;分选另立;不并入 image/absorbance |
| PCR thermocycling | PCR/RT-qPCR | 已复审 method-local;不建公共原子 |
| Electrophoresis separation | 琼脂糖/SDS-PAGE | 已复审 method-local;两个家族但 G2 失败 |
| 图像分析/统计 | 所有成像/计数 | 明确 out-of-scope;只记录原始图像/读数 |

## 4. 建议批次

| 批次 | 内容 | 目的 |
|---|---|---|
| B08 | 3D 球体/悬滴/Spheroid sheet | 检验 2D→3D 假设;不建专用原子 |
| B09 | 转染/稳定株 | 检验试剂/筛选条件能否作为 method-local |
| B10 | 微生物生长曲线/转化/药敏 | 检验 OD600、热激、MIC 的复用 |
| B11 | 分子克隆链 | 检验提取/PCR/电泳之后的多步组合 |
| B12 | 蛋白纯化/IP | 检验制备链与检测链的边界 |
| B13 | 活细胞成像/细胞死亡/周期 | 检验 raw acquisition 与染色/分析分离 |

## 5. 拆解决策

- 先拆 P0;每批 2–4 份完整 Protocol,不要按网站扫完。
- 每个实验先做 Reuse Map;已有操作 + 参数/变体优先。
- 只有通过四维粒度 + 晋升门槛的候选才进公共库。
- 3D/转染/克隆/纯化很可能大量是 composite/method-local,不要预期新增公共原子。
- 公共能力缺口(发光/荧光板读/FACS)单独研究,不要塞进实验模板。

## 6. 不在本轮

- 临床、动物在体、统计/生物信息/图像分析。
- 为每个模型创建专属公共操作。
- 为覆盖率批量生成 JSON。
- 把研究记录数值写成通用默认。
