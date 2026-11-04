# Goal

让 AsterLab 先具备一个可验证的研究工作入口：研究者能在一张只读“实验地图”里看懂为什么做、比较什么、检测如何安排、样本如何流转、还有什么没确定；并能从同一张地图进入一次冻结 Protocol 的执行台，按步骤查看目标、参数和记录入口。

## 本轮交付

- 新增只读 `design.map` CLI 与 HTTP 读模型；只投影已有设计字段，不补造对照、n、剂量、时间点或孔位。
- 设计卡片增加“实验地图”只读视图；显示组别、检测、时间点、样本关系、Protocol 关联状态和待确认项。
- 执行台增加返回实验地图的入口；使用现有冻结 `protocol_manifest` 显示完整步骤、参数和记录入口。
- 保持设计 / Protocol / Run 分层；地图不写数据库，不把动画、预览或计划当成完成。
- 增加 CLI/HTTP/UI 验证，使用 L929 合成数据。

## 非目标

- 本轮不加 `StepMeta`，不做多步骤编辑器、Protocol 组合器或仪器模块库。
- 不自动生成实验方案，不自动补科学参数，不把技术重复当独立 n。
- 不新增数据库表或迁移。

## Result

- `design.map` CLI + `GET /api/designs/{id}/map` implemented; read-only, no schema/table migration.
- Design cards open a read-only experiment map with groups, assays, timepoints, sample links, Protocol status and unresolved items.
- Run workbench has a return-to-map entry and still reads the frozen Protocol steps.
- Smoke tests cover CLI map, HTTP map, UI map, and 390px layout: 38 CLI tools exercised.
- Visual evidence: `docs/prototypes/assets/experiment-map.png`.
- `cargo build --release`, root Rust tests, desktop Rust tests and `node tests/smoke.cjs` pass.
