# AsterLab Handbook

AsterLab 是一个面向生物与湿实验研发的**本地单真源（Local-First Single Source of Truth）**实验工作台。它将科研数据的可追溯性与无摩擦的连续书写体验统一在同一份 `.asterml` 文件之上，同时服务于桌面端复杂编排、实验台平板/手机快速记录以及 Agent 结构化协作。

## 核心设计理念

- **单文件即工作区（UnifiedLive）**：实验不再被拆散在孤立的「设计库」与「记录表单」中。一份 `.asterml` 文档在编辑、阅读、源码三种视图间共享同一底层事实：设计阶段的孔板、分组与参数会在执行阶段原位承载勾选、倒计时、实测偏离与二维读数矩阵。
- **通用原语而非专病堆砌**：无论 CCK-8、RT-qPCR、抑菌圈还是细胞转染，全部由 11 个通用内容模块、1 个通用容器 `plate@1` 与 10 个第二层科学原语组合表达，严禁为单一实验类型新增硬编码页面或专用模块。
- **双轨严格而不互相阻塞**：日常探索与记录通过纯 Go Vault 引擎在本地目录秒级运行；进入正式冻存与严格审计链路时，通过 Native FFI 或 Agent CLI 对接 Rust Core 与不可变 Run 快照。

## 手册导航

| 模块 | 阅读入口 | 核心覆盖内容 |
|---|---|---|
| **Start** | [领域模型与科学红线](./handbook/domain-model.md) | `Design` / `Protocol` / `Run` 三对象边界、`UnifiedLive` 与科学统计学红线 |
| **Architecture** | [分层架构与代码包](./handbook/architecture.md) · [Vault 存储](./handbook/vault-storage.md) · [移动端与双端同步](./handbook/mobile-and-sync.md) | Pure-Go Vault、Rust Core FFI、100 步 Undo/Redo、Android/iOS 运行边界与协同防冲突 |
| **AsterML SDK** | [AsterML 语法与 Parity](./handbook/asterml-spec.md) · [内容模块 (11)](./handbook/modules-content.md) · [孔板、原语与 PDF](./handbook/modules-labware-and-pdf.md) | 21 个注册模块规范、UCUM 单位集、A4 执行单 PDF 导出与全组件实图 |
| **UI & UX** | [Token、色盘与动效](./handbook/design-system.md) · [Liquid Glass 材质](./handbook/liquid-glass.md) · [桌面端工作台](./handbook/desktop-workbench.md) · [移动端单手工作台](./handbook/mobile-workspace.md) | 三套色盘（`obsidian`/`zinc`/`warm`）数值、Light/Dark 玻璃着墨、平权分屏、储藏室与手势交互 |
| **Reference** | [FFI 桥接与 CLI 契约](./handbook/ffi-and-cli.md) · [校验门禁与优化热点](./handbook/verification-and-backlog.md) | `RunService`/`DesignService` 接口、CLI 退出码、自动化巡检命令与当前已知优化项 |

> [!NOTE]
> 本手册各章节直接以当前可编译源码与自动视觉回归截图为唯一依据，剔除了历史阶段记录与过期审计日志。任何 UI 视觉修改与接口演进，均应先在本手册定位对应规范后再改动代码。
