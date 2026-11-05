# K2 仪器来源审计(K2 Instrument Source Audit)

- 日期:2026-11-05
- 状态:四份 `research_draft` 逐项记录;没有 `SourceChecked`,没有真实 profile 进入运行时。
- 目的:为 K2 匹配引擎提供可核查的仪器能力记录,而不是用合成值或记忆填 profile。
- 规则:每个数值必须有一手来源 + 页码/章节 + 检索日期;无法确认写 UNKNOWN;配置相关项不得写成固定能力。

## 文件

| 类别 | 型号 | 记录 | 已核对范围 | 主要 UNKNOWN |
|---|---|---|---|---|
| 手动移液器 | Eppendorf Research plus | [manual-pipette-eppendorf-research-plus.md](manual-pipette-eppendorf-research-plus.md) | 2013 操作手册镜像的单/多道量程、增量、吸头、误差、ISO 8655 条件、维护 | 16/24 道与 Move It 量程、部分多道绝对随机误差、当前性能数据表 |
| CO₂ 培养箱 | Thermo Heracell VIOS 160i | [co2-incubator-heracell-vios-160i.md](co2-incubator-heracell-vios-160i.md) | 产品页规格表、手册 Ch.3/6/11/12、温度/CO₂/O₂/湿度/HEPA/Steri-Run | 校准/服务间隔、恢复时间认证值、部分配置件号 |
| 吸光度读板机 | Thermo Multiskan SkyHigh | [plate-reader-multiskan-skyhigh.md](plate-reader-multiskan-skyhigh.md) | 官方规格表 PDF + 产品页规格表;波长、板型、模式、温控、线性、速度 | 振荡参数、温控均匀性、动力学时序、软件版本 |
| 荧光显微镜 | Leica DMi8 | [microscope-leica-dmi8.md](microscope-leica-dmi8.md) | 官方规格页、FAQ、产品手册;平台/对比方法/端口/检测器/载物台选项 | 物镜目录、滤光片波段、相机规格、载物台行程、培养附件参数 |
| 离心/计数/洗板 | 5424 R / Countess 3 / BioTek 405 TS | [centrifuge-cell-counter-washer.md](centrifuge-cell-counter-washer.md) | 三份记录;数值均标 search-summary-only | 转子/适配器、量程、manifold/残留需逐页复核 |
| qPCR/流式/分光/混匀 | QS5 / FACSCanto II / NanoDrop One / ThermoMixer C | [qpcr-flow-spectro-thermomixer.md](qpcr-flow-spectro-thermomixer.md) | 四份记录;平台 vs 配置分开 | 通道/滤光片/体积/软件/加热块需逐页复核 |

## 统一结论

1. 四份记录都只是 `research_draft`;没有逐字段走完厂商手册 + 实验室 SOP + 专家复核。
2. 型号是平台/系列,不是单台配置;DMi8 和 Research plus 的配置差异尤其大,不能把平台最大能力当成每台机器的能力。
3. 现有 K2 匹配器只消费 `InstrumentProfile` JSON;本轮没有把这些 Markdown 自动导入,也没有伪造 profile。
4. 下一步应把每份记录中**已核对**的字段转成 `instrument_profile` JSON,保留 `review_status: research_draft`;未核对字段不写入或写为未知。
5. 任何 `Compatible` 判定都需要 profile 至少 `source_checked`;当前四份都不满足。

## 来源审计的最低门槛

一份记录升级为 `source_checked` 前必须:

- 每个字段有来源 URL、页码/表格/章节、检索日期;
- 区分厂商规格、实验室 SOP、实际机器配置;
- 记录配置项和适用条件;
- 由第二人复核字段与来源;
- 对矛盾值保留两个来源并说明取舍;
- 不把平台能力写成单台机器能力。

## 与 K0 的关系

K0 的实验/步骤矩阵仍没有 `SourceChecked` 条目;K2 的仪器档案也保持 `research_draft`。这两类证据不能互相替代。
