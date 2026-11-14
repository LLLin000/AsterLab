# AsterLab 设计先行评审包

- 日期：2026-11-06
- 分支：`feature/slint-native-spike`
- 生产 UI 基线：`ecda2de`
- 状态：设计稿，未作为生产 UI 验收

## 文件

```text
docs/prototypes/tabler-vs-dify-2026-11-06/index.html
docs/prototypes/tabler-vs-dify-2026-11-06/comparison.html
docs/prototypes/tabler-vs-dify-2026-11-06/DESIGN.md
docs/prototypes/tabler-vs-dify-2026-11-06/assets/before-1440.webp
docs/prototypes/tabler-vs-dify-2026-11-06/assets/after-1440.webp
```

## 设计目标

- 主参考：Dify Workflow Studio 的画布与配置内容一体化层级。
- 精度参考：Linear 的克制导航、字号层级、少边框、单一强调色。
- 保留：1280/1440 桌面、左侧 rail、顶部受限 DAG、下方 Step Detail。
- 内容优先级：Plate 8/12、Calculator 4/12、Table 12/12；不再机械三列等宽。
- 一个主滚动容器；普通卡片不各自滚动。
- Proposal Inbox 收起为“待审”入口，不覆盖科研卡片。
- 外部 Agent 通过 CLI/HTTP 投递，不内嵌 LLM。

## 当前设计稿的关键数字

| 项 | 值 |
|---|---|
| Rail | 56 px |
| Topbar | 56 px |
| Workflow | 约 280 px |
| 卡片间距 | 12 px |
| 卡片内边距 | 14–16 px |
| Plate | 8/12，18–24 px 孔位 |
| Calculator | 4/12 |
| Table | 12/12 |
| 标题 | 19–22 px |
| 卡片标题 | 14–15 px |
| 正文 | 12–13 px |
| 元数据 | 10–11 px |

## 评审问题

1. 这个目标稿是否可以作为生产 UI 的唯一视觉基准？
2. Plate 8 / Calculator 4 / Table 全宽是否符合真实 CCK-8 使用？
3. 深色是否作为主界面，浅色作为选项？
4. Proposal Inbox 收起入口是否足够？
5. 是否允许按此稿逐模块还原？

在未确认前，不再继续改生产 UI。
