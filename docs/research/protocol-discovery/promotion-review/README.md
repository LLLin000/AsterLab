# Catalog Promotion Review v2

- 日期:2026-11-05
- 状态:研究审查;不是 SourceChecked;没有 `ReviewRecord`
- 目的:对 domain-shared candidates 做跨方法语义对照,决定是否值得进入 Catalog、应缩小到什么范围。
- 规则:至少 2 个独立方法;比较目的/端口/样本变化/完成检查/仪器要求/差异成本;不足则保留 candidate/deferred/method-local。不生成 JSON。

## 结论

| 文件 | 候选 | 决定 |
|---|---|---|
| [thermal-and-electrophoresis.md](thermal-and-electrophoresis.md) | ThermalProgram | **Promote domain-shared**(核酸扩增热暴露;荧光/熔解/等温 RT 排除) |
| 同上 | Electrophoresis separation | **Deferred**(不永久 method-local;需窄边界 + SDS-PAGE 直接读) |
| [flow-and-luminescence.md](flow-and-luminescence.md) | FlowAcquisition | **Promote domain-shared**(raw FCS;分选/设门排除) |
| 同上 | LuminescencePlateRead | **Promote domain-shared**(积分 RLU;成像/吸光度排除) |
| [sterile-processing.md](sterile-processing.md) | SterileFiltration | **Deferred**(无菌完整性未证;窄边界重审) |
| 同上 | Autoclave(solution) | **Method-local deferred**(循环/装载/冷却分支过大) |

- 这些是 Catalog 分类决定,不是已实现模块;没有 JSON/Runtime。
- 仍为 `research_draft`;没有 `SourceChecked`。
