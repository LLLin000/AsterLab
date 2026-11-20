# AsterLab Handbook

AsterLab 是一个面向生物与湿实验研发的**本地单真源（Local-First Single Source of Truth）**实验工作台。它将科研数据的可追溯性与无摩擦的连续书写体验统一在同一份 `.asterml` 文件之上，同时服务于桌面端复杂编排、实验台平板/手机快速记录以及 Agent 结构化协作。

## 核心设计理念

- **单文件即工作区（UnifiedLive）**：实验不再被拆散在孤立的「设计库」与「记录表单」中。一份 `.asterml` 文档在编辑、阅读、源码三种视图间共享同一底层事实：设计阶段的孔板、分组与参数会在执行阶段原位承载勾选、倒计时、实测偏离与二维读数矩阵。
- **通用原语而非专病堆砌**：无论 CCK-8、RT-qPCR、抑菌圈还是细胞转染，全部由 11 个通用内容模块、1 个通用容器 `plate@1` 与 10 个第二层科学原语组合表达，严禁为单一实验类型新增硬编码页面或专用模块。
- **纯 Go Vault 统一核心**：桌面端、移动端与 `cmd/asterlab` CLI 共用纯 Go 引擎（`internal/vault` + `internal/asterdoc` + `internal/astercore`），直接以 `.asterml` 文件系统为单一事实源，零 `Runs.dll` / SQLite 依赖即可运行 ARRIVE 对照诊断、CCK-8 / RT-qPCR 统计分析、全库检索与版本快照恢复；原 Rust Core（`crates/` / `modules/runs`）已正式退场，不再参与构建与运行。

## 手册导航

| 模块 | 阅读入口 | 核心覆盖内容 |
|---|---|---|
| **Start** | [领域模型与科学红线](./handbook/domain-model.md) | `Design` / `Protocol` / `Run` 三对象边界、`UnifiedLive` 与科学统计学红线 |
| **Architecture** | [分层架构与代码包](./handbook/architecture.md) · [Vault 存储](./handbook/vault-storage.md) · [移动端与双端同步](./handbook/mobile-and-sync.md) | `.asterml` 单真源与 `astercore` 架构决策、增量索引缓存、事件总线、快照恢复与双端同步 |
| **AsterML SDK** | [AsterML 语法与 Parity](./handbook/asterml-spec.md) · [内容模块 (11)](./handbook/modules-content.md) · [孔板、原语与 PDF](./handbook/modules-labware-and-pdf.md) | 21 个注册模块规范、UCUM 单位显示映射、A4 执行单 PDF 导出与全组件实图 |
| **UI & UX** | [Token、色盘与动效](./handbook/design-system.md) · [Liquid Glass 材质](./handbook/liquid-glass.md) · [桌面端工作台](./handbook/desktop-workbench.md) · [移动端单手工作台](./handbook/mobile-workspace.md) | 三套色盘、Oklab 动效、全局全文搜索、`关联与诊断` 抽屉、AsterCore 汇总卡片与移动端底岛 |
| **Reference** | [Pure-Go CLI 与深度链接](./handbook/ffi-and-cli.md) · [校验门禁与优化热点](./handbook/verification-and-backlog.md) | `cmd/asterlab` Vault CLI 退出码、`asterlab://` 路由、自动化巡检命令与视觉回归实图 |

> [!NOTE]
> 本手册各章节直接以当前可编译源码与自动视觉回归截图为唯一依据，剔除了历史阶段记录与过期审计日志。任何 UI 视觉修改与接口演进，均应先在本手册定位对应规范后再改动代码。
