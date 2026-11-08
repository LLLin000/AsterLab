# AsterLab Beta 验收矩阵

- 基线:`e6ea298` / `56e5ace`
- 范围:Plan → Session → Results 核心闭环
- 状态:单用户本地 Beta 候选,不是完整 ELN/LIMS

## 验收项

| 项目 | 结果 | 证据 |
|---|---|---|
| 多检测设计编辑 | 通过 | L929 设计含 CCK-8 与 LIVE/DEAD;编辑器显示两个检测卡和各自时间关系 |
| 混合时间范围任务 | 通过 | L929 Run 首屏显示“通用准备”2 个任务 + D1 3 个任务;`pages-smoke` 负例覆盖 |
| 任务状态一致性 | 通过 | 后端 `PlannedTask.status` 由每任务最新事件投影;completed → blocked → completed 实测 |
| Run 状态原子化 | 通过 | 执行事件与 Run 状态同一事务;完成后 API 状态为 `completed` |
| Results 数据往返 | 通过 | 粘贴 0.82/0.79 → 保存 → 进程重启 → `0.82 AU · Control-1; 0.79 AU · Control-2` 仍显示 |
| 粘贴校验 | 通过 | 空数值、非法数字、超过两列阻止保存;空值不再变成 0 |
| 样本匹配 | 部分 | 未匹配时明确显示“仅保留原始文本”,不伪装成已匹配 |
| 跨天恢复 | 通过 | D1 完成 → 重启 → D3;D3 完成 → 重启 → D7;通用准备仍显示 |
| 冻结资源 | 通过 | 新 Run 锁定 `view_id`,资源写入 `execution_resources` |
| 准备清单 | 通过 | Western blot 汇总 18 材料、7 仪器;无资源时显示 0 |
| 视觉/视口 | 通过 | 1440/390 无文档级横向溢出;Run 首屏不再显示设计画布 |

## 自动化

```powershell
node --test tests/pages-smoke.cjs
```

测试覆盖:

- 4 个主标签
- Protocol / Run / Tools 页面
- 无检查步骤确认完成
- 混合时间范围首屏
- Run 状态 `completed`
- measurement 写入与 RunWorkspace 投影往返
- 设计/运行画布隐藏

## 证据

- `target/line-audit-LinePlan/`
- `target/line-audit-LineSession/`
- `target/line-audit-LineWB/`
- `target/results-paste.png`
- `target/mixed-tasks.png`

## 已知限制

- `PlannedTask.target` 仍为 `Unresolved`;UI 用“适用”而非“目标样本”。
- 结果入口只支持结构化读数粘贴;图片/文件关联和仪器格式解析未实现。
- 没有步骤级“必需结果”声明;只能提示已完成但无读数。
- 样本标签未与本次 Run 的实际样本建立可靠匹配。
- 未做真实手机触控、对比度测量和真实科研用户走查。
- 电子签名、多用户权限、全自动图像分析不在本轮 Beta 范围。
