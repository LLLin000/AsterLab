# EXP-14 证据（Evidence）

- 卡片：EXP-14 (Contract) 实验处理器语义契约
- 分支：`feature/exp-14-contract-reus-c22`
- 范围（机器可读原文见契约 PR 正文末尾的 `kandev-scope` / `kandev-task` 块）：
  write 仅 `docs/contracts/experiment-processor-v1.md` 与本文件。

## 执行命令与原始输出

命令（验收）：

```bash
python3 -c 'from pathlib import Path; p=Path("docs/contracts/experiment-processor-v1.md"); s=p.read_text(); required=["CCK-8","not_detected","CFU/mL","derived_from","Reuse Map","research_draft","Scratch","Flow/FCS","ProtocolBlockVersion","execution_event"]; missing=[x for x in required if x not in s]; assert not missing, missing; print("EXP-14 contract anchors PASS")'
```
原始输出：

```text
EXP-14 contract anchors PASS
exit=0
```

退出码 0；输出含 `EXP-14 contract anchors PASS`。命令使用仓库相对路径，
无绝对路径落盘。

## 契约评审结论（自评，供 Review 复核）

- 只写契约文档 + 本证据文件；未改 `crates/*`、`web/`、冻结 tag/Run，未跑全仓 `cargo fmt`。
- 引用权威类型：`ProtocolBlockVersion` / `BlockUse` / `ParameterBinding` /
  `ProtocolVersion` / `DesignVersion` / Run 冻结清单 / `execution_event` /
  typed readout（`cck8_od450`、`rna_concentration`、`rna_purity`、`cfu_count`）。
  未新增 readout 表、运行时 schema、通用处理器框架或第二执行引擎。
- 三家族逐项列出 raw/derived、单位、来源、lineage、checks、复用类别
  （direct/parameterized/variant/composite/method template/ToolAction/Custom/Unknown）。
- `Scratch`、Flow/FCS、DOE、复杂统计保持 deferred；LDH/LIVE-DEAD 仅作
  Reuse Map `variant` 参照，不展开执行映射。
- 三份研究输入保持 `research_draft`；晋升门槛（第二人复核 + ReviewRecord +
  CompilePolicy/SOP 验收）未动。
- 未解决边界逐项列出，未填零、未推导算法。
