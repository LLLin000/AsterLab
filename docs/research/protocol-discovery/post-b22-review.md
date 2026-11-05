# Post-B22 覆盖与下一步

- 日期:2026-11-05
- 状态:研究覆盖审计;不是 SourceChecked
- 结论:B01–B22 已覆盖常规湿实验;继续扩宽度的收益下降,下一步应转入 **Catalog Promotion Review v2**。

## 1. 已覆盖

B01–B22:

- 基础细胞培养、传代、计数、冻存
- 微生物划线/涂布/液体培养/稀释/计数/转化/药敏
- CCK-8/LDH/LIVE-DEAD/scratch/Transwell
- DNA/RNA 提取、PCR、电泳、RT-qPCR
- ELISA/WB、蛋白纯化/IP
- 流式/IF、活细胞成像、死亡/周期
- 基础配液/无菌
- 转染/稳定株、CRISPR/病毒转导
- 3D 球体/悬滴/ALI/类器官分化
- 原代细胞/组织解离
- 细胞–材料相互作用
- 组织学/IHC
- EV/亚细胞组分
- 报告基因/ROS/线粒体/钙
- 细胞系 QC

## 2. 仍未覆盖(更专门)

| 领域 | 状态 | 建议 |
|---|---|---|
| 流式分选/单细胞分离 | 采集已覆盖,分选未独立 | 需要时单独 Protocol |
| iPSC/ESC 分化 | 部分类器官分化已覆盖 | 需要时单独 Protocol |
| organ-on-chip/3D bioprinting | 未覆盖 | 复杂/设备依赖;暂缓 |
| 免疫细胞功能(杀伤/ELISpot/细胞因子) | 未覆盖 | 下一轮可选 |
| 高级成像(共聚焦/超分辨/FRET/FRAP) | 原始采集语义已覆盖 | 方法模板按需 |
| 电生理/patch clamp | 未覆盖 | 专门设备;暂缓 |
| 材料力学/理化表征 | 非细胞湿实验 | 不属于本 Catalog |
| 测序/质谱/组学 | 数据/组学 | 本轮范围外 |
| 动物/临床 | — | 本轮范围外 |

## 2.1 Promotion Review v2 结果

| 候选 | 决定 |
|---|---|
| ThermalProgram | Promote domain-shared |
| FlowAcquisition | Promote domain-shared |
| LuminescencePlateRead | Promote domain-shared |
| Electrophoresis separation | Deferred |
| SterileFiltration | Deferred |
| Autoclave (solution) | Method-local deferred |

仍没有 JSON/Runtime;这些都是研究结论。

## 3. 下一步:不再扩宽度

优先做 **Catalog Promotion Review v2**:

- 复核全局操作(transfer/washing/incubation/centrifugation/absorbance/raw image)
- 复核 domain-shared candidates(ThermalProgram、FlowAcquisition、LuminescencePlateRead;SterileFiltration/Autoclave deferred)
- 对每个候选给出端口/样本变化/完成检查/差异成本
- 维持 `research_draft`;不做大批量 JSON

## 4. 指标

- Protocol 覆盖:22 批;常规湿实验已覆盖。
- New global OperationModule:0。
- Domain-shared candidates:3(热循环、流式采集、发光板读)+ 2 deferred(电泳、过滤/灭菌)。
- SourceChecked:0。
- 研究记录可继续增加,但不应再以“新增模块/批次”为目标。
