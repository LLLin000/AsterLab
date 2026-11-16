# EXP-16 evidence — Scratch/image 与 Flow/FCS 缺口登记

- 卡：EXP-16 (Docs)；分支：`agent/exp-16-*`（见 PR）。
- 验收命令（仓库根目录执行）：

```sh
python3 -c 'from pathlib import Path; s=Path("docs/research/processor-gap-register.md").read_text(); required=["Scratch","image","Flow","FCS","算法","阈值","不得推断","deferred"]; missing=[x for x in required if x not in s]; assert not missing, missing; print("EXP-16 gap register anchors PASS")'
```

- 期望：exit code 0，输出 `EXP-16 gap register anchors PASS`。
- 原始输出（见 §1）由验收命令逐字产生，未手工改写。
- 来源/缺口审核记录：登记文档 §8（已读来源 + `research_draft` 状态 + scope 冲突记录）。

## 1. 验收原始输出

```text
EXP-16 gap register anchors PASS
```
