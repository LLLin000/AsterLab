# Lab Workbench（实验工作台）

一个运行在本机的 Rust + SQLite 实验规划与记录工作台。它借鉴 Cell Planner 的项目、任务日历、记录、资料库和计算器工作流，同时加入有版本的实验设计、Protocol、实验运行和铺板布局。它是可用的产品原型，不是 Cell Planner 的品牌、专有素材或内部实现克隆。

产品术语与目标对象边界见 [CONTEXT.md](CONTEXT.md)；该文件记录目标设计语义，当前 CLI 的已实现能力仍以 [AGENT_API.md](AGENT_API.md) 为准。

系统分层、Slint 前端职责、Core 读模型和仪器步骤模块的架构见 [docs/architecture.md](docs/architecture.md)。
文档权威与阅读顺序见 [docs/README.md](docs/README.md)；当前实施阶段见 [docs/implementation-plan.md](docs/implementation-plan.md)；P0 Core 契约见 [docs/contracts/core-v1.md](docs/contracts/core-v1.md)。
当前共享设计投影位于 `crates/asterlab-core`；P1-A 已由 CLI/HTTP 与 Slint 地图摘要共用。

## 启动

Windows 可双击 `start-labworkbench.bat`，或在 PowerShell 中运行：

```powershell
./run-local.ps1
```

然后打开 http://127.0.0.1:8832。启动脚本会先用 Rust stable 与 Cargo 编译当前源码，再运行 target/release/labworkbench.exe，确保打开的是最新界面与逻辑。首次启动会在本目录创建 data/labworkbench.sqlite。按 Ctrl+C 停止；再次启动会继续使用同一份本地数据库。

可通过 `LABWORKBENCH_DATA_DIR` 更改数据库目录、通过 `LABWORKBENCH_PORT` 更改端口。应用只绑定 `127.0.0.1`，当前不提供账号、网络同步、多用户协作或备份恢复。

## 当前工作流

1. 建立项目，在“实验设计”登记实验单位、组别条件和多项检测。每项检测各有自己的组别、读数、时间、载体、Protocol 与样本使用方式；分配关系保存计划槽位，不生成真实样本 ID。来源证据和未决冲突可随设计保存，保存修改会生成新版本。
2. 建立 Protocol 与版本；安排日历任务或创建一次实验运行。运行固定引用创建时的设计版本与各检测 Protocol 版本；同一设计可以有多轮 Run，并记录轮次来源关系，不把轮次数当作独立 n。
3. 在设计卡打开“实验地图”：只读查看研究目的、组别、检测覆盖、时间点、样本关系、Protocol 关联状态和待确认项；地图不补造 n、剂量、时间点或孔位。
4. 从运行卡打开操作台：按冻结 Protocol 逐步查看动作、已确认参数、检测对象、时间点和关联板图；没有实际孔位时只显示计划数量。新样本按时间点拆分的检测必须给板图标时间，同一样本跨时间复测则保留同一块板。
5. 在运行下建立铺板布局，选择 6/12/24/48/96/384 孔板并为具体孔位填写组别、样本等信息。操作台只显示当前检测和时间点的真实映射，不自动分配孔位。
6. 从当前步骤追加实际操作事件；事件保留冻结 Protocol 版本、检测或共享产物和步骤路径。自由文本记录仍用于补充背景；事件时间可留空，系统录入时间独立保存。运行只允许修订名称与计划日期，设计和 Protocol 快照不可被网页整体覆盖。
7. 用资料表保存自定义列和行，用公式库记录自行定义的公式；使用计算器核对稀释、溶液质量、系列稀释或反应体积算术。
8. 可以按文字、运行和日期范围筛选记录并导出 CSV；浏览器打印可另存 PDF。
9. Agent 通过每项操作各自的强类型 CLI 子命令设计、查询、创建和修订记录。工具 schema 限定字段与枚举，写入返回固定 JSON，修订要求提供版本号；导入分成只读预检和显式提交。

## 科学记录边界

字段不替研究者选择疾病机制、处理组、样本量、剂量、时间点或实验条件。实验单位、重复类型和样本量理由可显式填写；空值保持空值。Protocol 和实验设计保留版本，运行引用具体版本，便于追溯当时执行依据。计算器展示算术假设，不能替代试剂说明、方案或研究者核对。

## 接口与验证

- Agent 的标准操作面是 `target/release/labworkbench.exe tools --json` 列出的命令行工具；每项工具都提供封闭的输入 schema、固定输出 schema、作用说明和 `side_effects`。用 `target/release/labworkbench.exe schema <命令>` 查看单项契约。Agent 不应通过通用记录写入接口绕过这些定义。
- `GET /api/health`：本地服务状态。
- `GET /api/state`：读取工作区记录。
- `GET /api/designs/{id}/map`：只读实验地图，返回组别、检测、时间/样本关系、Protocol 关联和待确认项。
- `GET /api/runs/{id}/workspace`：只读冻结 RunWorkspace，返回冻结设计、步骤/任务/实例、材料、板图和事件。
- `POST /api/records` / `POST /api/records/update`：供当前本地网页界面使用的服务端适配层；Agent 的正式工具契约以强类型 CLI 为准。
Agent 命令、输入输出 schema、错误码和科学字段语义见 [AGENT_API.md](AGENT_API.md)。本地回归检查见 `tests/smoke.cjs` 和 `tests/p2-fixture.cjs`；它会自行启动隔离实例，使用合成数据和临时数据库，不连接或改写当前打开的实例。

## 当前限制

- CLI 是 Agent 的本地操作面；项目仍不内置模型或自然语言解析。Agent 的实际调用由外部运行环境决定。
- 没有文件附件、样本库存/试剂批次、权限、协作、云同步、自动备份或迁移界面。
- 记录表采用带类型的关联对象与 JSON 字段，尚未提供可视化自定义字段类型系统。
- 运行方式面向本机原型；还没有做安全审计、打包安装、多用户并发或生产环境验收。
- 铺板布局是本项目新增的功能域；当前安装版 Cell Planner 的检查没有找到独立 plate-map 页面，不能据此说这是原版现成功能。

## 研究来源

Cell Planner 安装包的静态和有限运行时证据、实现边界与功能比较见上级目录的 [研究总览](../README.md) 与 [REA 核验记录](../15-rea-findings.md)。
