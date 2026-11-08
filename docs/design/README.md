# Design Contracts

- 状态：**Native UI Direction Freeze / 2026-11-08**
- 六份契约是本阶段的设计权威；科学对象 Schema 仍在 [CONTEXT.md](../../CONTEXT.md)。

| 文件 | 作用 |
|---|---|
| [PRODUCT.md](../../PRODUCT.md) | 产品定位、内容优先级、四项冻结决策、非目标 |
| [DESIGN.md](../../DESIGN.md) | 颜色、文字、间距、布局、组件与数据语义 |
| [SURFACES.md](../../SURFACES.md) | 页面职责、Run 工作表、Schedule Strip、状态清单 |
| [COMPONENTS.md](../../COMPONENTS.md) | 组件语义、交互、可访问性、Rune 映射 |
| [RECORDING.md](../../RECORDING.md) | 完成与测量事件、Rust 命令层、共享读契约 |
| [MIGRATION.md](../../MIGRATION.md) | 两级 Rune 门槛、迁移阶段、对等与切换条件 |

## Evidence

- [BASELINE.md](BASELINE.md)：当前 Web Run 截图、状态清单、live detector 结果
- [CRITIQUE.md](CRITIQUE.md)：Impeccable Operate 设计 critique
- [AUDIT.md](AUDIT.md)：技术 audit 与迁移前修复项
- [RUNE_EVALUATION.md](RUNE_EVALUATION.md)：固定 revision 的 Rune 技术评估
- `baseline/`：Web 截图

## 顺序

1. 先冻结设计权和职责（本目录）。
2. 修 token、对比度、10px 功能文字和共享读契约。
3. 跑 Rune 技术可行性 Spike。
4. 通过 gate 1 后只做 Run 单屏纵向切片。
5. 通过 gate 2 后再讨论其他页面。

未通过技术门槛时停止 Rune 扩张，保留 Web Beta。
