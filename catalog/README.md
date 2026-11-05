# Catalog Drafts

本目录只放结构校验通过的 K2.1 草稿,不进入 CLI/Protocol/Run/Slint 运行时。

| 目录 | 内容 | 状态 |
|---|---|---|
| `operations/` | `op.liquid_transfer`、`op.absorbance_measurement` | `research_draft`;无 review record |
| `instruments/` | Research plus 100–1000 µL 单道、Multiskan SkyHigh | `research_draft`;无 review record |

规则:

- 一个研究文档可以包含多个操作;运行时模块按可复用操作边界拆分。
- 每个数值必须有来源定位;无法确认的字段不写入或保持 UNKNOWN。
- `research_draft` 不能产生 `Compatible`;JSON 必须带 `review_status`。
- `ReviewRecord` 是审核证明;没有它,`source_checked` 也不可信。
- 这些文件不是官方模板库,不能直接导入正式项目。

验证:

```bash
cargo test -p asterlab-core --test catalog_drafts
```
