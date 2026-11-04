# 组别 × 检测覆盖语义研究：未覆盖、待定与明确不适用的区分

- 日期：2026-11-04
- 状态：研究笔记，非 UI 规范、非验收结论、非用户测试结论。
- 基线：仓库 HEAD `bf1f0e9f320bd7704e468354b3eb19b26156747e`；本轮以静态源码和官方资料为依据。尝试隔离启动未取得可验证的运行页面或截图；没有用户测试结论。
- 范围：只回答一个未解决问题——现有组别 × 检测总览能诚实表达什么；缺席覆盖、尚未决定、明确不适用应如何区分。

## 1. 证据状态

| 断言 | 级别 | 依据 |
|---|---|---|
| 总览格只有两种存储状态 | 源码事实 | `web/index.html#L239`：`selected=a.group_ids.includes(g.id)`，`state=已选择/未选择`，`mark=✓/○`，样式仅 `is-in/is-out` |
| 「未选择」的注脚只明确排除「已决定不做」这一种推断 | 源码事实 | `web/index.html#L250` 注脚：「未选择不等于已决定不做；本图不分配实际孔位。」图例为「✓ 已选择　○ 未选择」；它没有把未选择定义为待定，也没有区分不适用或录入遗漏 |
| 表格使用了行列双表头结构 | 源码事实 | `web/index.html#L238-L239` 使用 `<th scope="col">` 和 `<th scope="row">`；W3C WAI 说明 `scope` 关联表头与数据单元格，便于理解矩阵关系（[Tables with Two Headers](https://www.w3.org/WAI/tutorials/tables/two-headers/)） |
| 覆盖编辑控件是二态复选框 | 源码事实 | `web/index.html#L181-L182`：`selectedIds` 只读 `:checked`，`groupChecks` 只输出 `checked/unchecked` |
| 覆盖缺席没有独立存储字段 | 源码事实 + 契约事实 | `AGENT_API.md#L93-L96`：`assays[].group_ids/group_names` 只表达适用组；`timepoint_relation` 枚举为 `same_*/independent/unknown/null`，无 `excluded/not_applicable`；`makeSampleAllocations`（`web/index.html#L222`）只为已选组建分配 |
| `not_applicable` 只存在于跨检测来源，不属于覆盖状态 | 源码事实 + 契约事实 | `web/index.html#L167-L168` 与 `AGENT_API.md#L96`：`source_relation ∈ not_applicable/unknown/split/sequential`；`allocationTimepointRelation`（`L169`）回退到 `unknown` |
| `.is-unknown` 样式存在但覆盖格未使用 | 源码事实 | `web/index.html#L81` 定义 `.design-map-state.is-unknown`；`designMapMarkup` 的覆盖格三元表达式未发射该类 |
| 无障碍与表单表达建议 | 外部依据 | 见 §8；本节不把外部指南当作本仓库已实现 |
| 桌面/窄屏可读性、真实走查 | 未验证 | 本轮无 CellPlanner 产品运行截图、无可验证运行页面、无用户测试；`HANDOFF.md#L79` 已明确移动端易用性仍需截图核对 |

术语基线以 `CONTEXT.md` 为准：未知 ≠ 否/零/不适用（`HANDOFF.md#L64-L65`、`CONTEXT.md#L101-L104`）；`n ≠ 实体样本数 ≠ 容器位数`；时间点关系与来源关系分别保存。

## 2. 现有总览能诚实说什么，不能说什么

能说：

- 「该检测当前适用组清单」：即 `assay.group_ids` 的成员关系快照。
- 「该检测的时间点标签与锚点是否缺失」：列头时间摘要与 `design-live-summary` 的待定计数属于同一草稿输入的只读投影。
- 「按组的时间点关系/来源关系是否待确认」：总览下方 `design-map-relations` 已按组展开（`web/index.html#L243-L249`），但它是对已选组的补充说明，不是覆盖格本身的状态。

不能说：

- 不能把「○ 未选择」读成「已决定不做」「本组不适用」「零样本」「零孔位」。
- 不能从缺席推断原因：可能是尚未决定、可能是方法学上不适用、可能是录入遗漏，三者在存储中无区别。
- 不能把覆盖矩阵当作孔位或样本承诺：注脚已明确「不分配实际孔位」，`plannedPositionSummary`（`L252-L257`）也只做逐组计数加「跨组总数待铺板映射去重」。
- 不能用 `source_relation=not_applicable` 冒充覆盖状态：它回答「该分配是否来自另一检测」，不回答「该组是否应进入该检测」。

因此当前只能诚实呈现为「已选择 / 未选择」，并说明「未选择的原因未记录」；不要把未选择等同于「待定」「已决定不做」或「不适用」。

## 3. 确切契约缺口

结论：存在一个最小且明确的契约缺口，不是纯视图问题。

缺口陈述：

- `assays[].group_ids` 是二态成员集合：`in ∈ group_ids` 表示适用；`not-in` 在语义上只能解释为「尚未录入适用」，无法区分「尚未决定」与「明确排除/不适用」。
- 契约中没有 `excluded_group_ids`、`coverage_state`、`not_applicable` 或等效字段；`sample_allocations[]` 也只为已选组生成记录（`web/index.html#L222`），缺席组无记录可挂理由。
- 若将来 UI 要显示第三态（例如「明确不做」或「与本组无关」），当前 schema、校验与往返逻辑均不支持；直接在视图层加第三种颜色/符号将是伪造数据状态，违反 `HANDOFF.md#L82-L84` 与 `CONTEXT.md#L101-L104` 的要求。

这不是「文档没写清」，而是「存储不能表达」。

## 4. 语义选项

当前契约只允许两种诚实处理：保留现有成员关系并明确披露缺口；或在未来扩展覆盖状态。三态（纳入 / 未决定 / 排除）不够，因为 `HANDOFF.md#L211` 明确要求「未知、明确不做、不适用」三者不可混淆。

### A. 暂不改契约：保留二态，并披露未记录状态

- 存储：不变；`group_ids` 中有该组表示此检测已纳入该组。未出现只表示记录中没有纳入关系，不能推出原因。
- 视图候选：`已纳入` / `未纳入（状态未记录）`；图下注明「未纳入不能区分待定、明确不做或不适用。」
- 编辑：不伪造第三/第四种状态；不把未勾选统一解释成「待定」。
- 边界：检测备注或证据文本可以保存说明，但不是结构化覆盖状态，不能驱动矩阵或被 Agent 当成已排除。

### B. 后续契约扩展：每个组别 × 检测保存四态

- 候选语义：`included`（已纳入）、`undecided`（待定）、`excluded`（明确不做）、`not_applicable`（不适用）。这是满足当前验收要求的最小状态集合，不是已批准的字段命名或 UI 控件。
- 建模候选：为每个 `(assay_id, group_id)` 保存单一规范状态，避免 `group_ids`、排除列表与备注形成多份真相；纳入和样本分配的派生规则须在 CLI/API 契约中一并定义。
- 兼容：旧 `group_ids` 成员迁移为 `included`；缺席成员一律迁移为 `undecided`，不可猜成 `excluded` 或 `not_applicable`。
- `not_applicable` 必须是组覆盖语义，不得复用 `source_relation=not_applicable`；后者表示「无跨检测来源」（`AGENT_API.md#L96`）。
- 代价：更新 CLI schema 与校验、网页/API、导入/规范化、分配生成、版本修订、读回与 Agent 文档；覆盖状态必须保存和往返，不可只画在图上。
- 控件形式、是否要求排除原因、批量编辑确认范围，需和实际研究者走查后定。W3C ARIA Checkbox Pattern 中的 `mixed` 表示一组二态复选框的部分选中汇总，不是每个矩阵单元可复用的「未知」值（[Checkbox Pattern](https://www.w3.org/WAI/ARIA/apg/patterns/checkbox/)）。

## 5. 建议

本轮不改 UI 或数据契约。当前矩阵只展示已有覆盖成员关系；研究材料应把后续决策写成显式契约缺口，而不是把未勾选翻译成某一种科学状态。

若之后做不改 schema 的小步文案调整，候选为 `已纳入` / `未纳入（状态未记录）`，并常驻说明「未纳入不等于明确不做，也不等于不适用；当前记录无法区分这三种原因。」该文字仍须经屏幕走查，不能宣称已解决可用性风险。

若研究者确认需要保存这些区别，未来模型至少要结构化表达四种状态：已纳入、待定、明确不做、不适用。旧数据缺席只迁移为待定；明确不做与不适用不能分两轮补成，因为两者都属于用户明确要求保留的科学含义。

矩阵仍保持组 × 检测两维；时间点和样本来源关系继续在检测摘要中单独表达，避免把矩阵扩成组 × 检测 × 时间三维表（`HANDOFF.md#L47-L49`）。不显示由 n、时间点或未映射组别推导的孔位总数。

## 6. 非目标

- 不重写四步流程，不新增页面，不把检测卡合并为巨型矩阵。
- 不引入孔位总数推算、跨组去重总数或从 n/时间点推样本数的任何公式。
- 不把 `controls[]` 文本、`source_relation` 或 `timepoint_relation` 复用为覆盖状态。
- 不引入新图标集、角色、动效或主题变更；图标选型见既有视觉语言研究，不在本文件重复定案。
- 不声称任何可用性提升、任务时长缩短或用户理解改善；本文件无用户测试。

## 7. 下一步最小验证（不写代码）

1. 选用 SA 梯度的同一草稿数据，先以二态 +「状态未记录」说明呈现，再以四态候选呈现；核对文字和示意数据，不把方案情景写成真实执行事实。
2. 让实际设计者逐格解释其意图属于纳入、待定、明确不做还是不适用；记录是否出现把空白当作确定结论、是否需要留存排除依据。没有研究者参与时，只称静态桌面走查。
3. 以同一组事实对照二态和四态草图；观察正确解释和发现缺漏，不先比较装饰、图标或速度。
4. 桌面与 390px 视口检查状态文本、键盘焦点和读屏顺序。W3C WAI 指出表格 `<caption>` 可帮助读屏用户在表格导航中识别表格；当前表格没有 `<caption>`，是否补充应在后续视图评审中确认（[Tables with Two Headers](https://www.w3.org/WAI/tutorials/tables/two-headers/)）。
5. 只有需求确认后才评审 schema/CLI/API 变更；迁移后检查旧缺席状态、显式排除、N/A、修改、保存和重开往返。

以上均是后续建议，尚未执行；本轮没有有效运行截图或用户测试数据。

## 8. 官方资料（外部断言的直接来源）

表格与表头语义：

- W3C WAI Tables with Two Headers（行列双表头与 `scope="col/row"`；也指出 `<caption>` 有助于读屏用户识别表格）：<https://www.w3.org/WAI/tutorials/tables/two-headers/>
- W3C ARIA Checkbox Pattern（`mixed` 用于一组二态复选框的部分选中汇总）：<https://www.w3.org/WAI/ARIA/apg/patterns/checkbox/>
- W3C Mixed-State Checkbox Example（说明该模式的分组选择语义，并提示实际使用需验证辅助技术支持）：<https://www.w3.org/WAI/ARIA/apg/patterns/checkbox/examples/checkbox-mixed/>
- GOV.UK Checkboxes（复选框用于多选；说明标签、fieldset/legend 与不预选）：<https://design-system.service.gov.uk/components/checkboxes/>
- GOV.UK Radios（单选语义；若「我不知道」是有效答案，应作为可选项提供）：<https://design-system.service.gov.uk/components/radios/>

外部指南说明表格与控件的语义，不证明哪种状态词或矩阵布局更适合 AsterLab。链接核对日期为 2026-11-04。

## 9. 仓库证据索引

- `web/index.html#L181-L182`：覆盖编辑为二态复选框。
- `web/index.html#L236-L250`：`designMapMarkup` 覆盖矩阵实现、图例与注脚。
- `web/index.html#L81`：`is-unknown` 样式已定义但覆盖格未用。
- `web/index.html#L167-L169`、`L190-L192`、`L222`：来源/时间关系有 `unknown`，但覆盖缺席无记录。
- `AGENT_API.md#L93-L99`：`assays`、`sample_allocations`、`planned_outputs` 字段与枚举；无覆盖排除字段。
- `CONTEXT.md#L44-L77`：组、检测、样本分配、来源关系、位置数范围的规范定义。
- `CONTEXT.md#L97-L104`：三图分工、数字语义、未知与不适用的表达原则。
- `HANDOFF.md#L75-L86`：四步流程现状与两个已看到的具体问题（矩阵二态、标题跳转未聚焦）。
- `HANDOFF.md#L190-L204`：开放设计决定与「先补契约再画图」的边界要求。
