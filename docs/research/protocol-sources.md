# Protocol 来源清单

- 状态:资料入口清单,不是已核验事实
- 用途:为具体实验/方法寻找完整 protocol;优先于仓库研究记录和二手摘要
- 规则:记录标题、作者、DOI/URL、版本/日期、检索日期、可访问性

## 优先仓库

| 来源 | 适合 | 注意 |
|---|---|---|
| protocols.io | 湿实验步骤、版本、DOI、讨论 | 部分条目需登录;保存版本/DOI |
| Bio-protocol | 同行评议的逐步 protocol | 检查补充材料和版本 |
| Nature Protocols | 成熟方法、变体、故障排除 | 订阅限制;记录 section/page |
| JoVE | 视频+文字 protocol | 订阅限制;不能把视频摘要当完整步骤 |
| Current Protocols | 经典实验方法、更新版本 | 章节/版本要记录 |
| Springer Protocols | 方法书章节 | 书籍/章节 DOI |
| Cold Spring Harbor Protocols | 分子/细胞/遗传方法 | 记录 protocol DOI |
| ASM protocols | 微生物、稀释、铺板、生物膜 | PDF/页面定位 |
| 厂商 application notes | 仪器/试剂特定条件 | 只在其声明范围内使用 |

## 不作为实验定义来源

- AsterLab 仓库里的研究记录/旧 SOP;
- 博客、二手综述、AI 摘要;
- 没有步骤/页码的厂商营销页;
- 只有“常用值”但没有适用条件的表格。

## 记录字段

```yaml
title:
authors_or_org:
doi_or_url:
version_or_date:
retrieved_at:
access: full | abstract | login_blocked | search_summary
locator: page/section/table
used_for: steps | parameters | controls | readout | context
```

## 与 Reuse Map 的关系

- Protocol 仓库提供“具体实验怎么做”;
- 本地 catalog 只回答“已有哪个可复用 Atomic Operation/Composite/Method”;
- 两者不得混为同一证据。
