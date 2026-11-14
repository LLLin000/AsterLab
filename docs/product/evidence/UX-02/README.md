# UX-02 — Recoverable empty and error states

Required capture format: real desktop window, light theme, 1360×880 and 900×600. Seed each state in an isolated data directory; do not fabricate records to make an empty state look populated.

| State | What the user sees | Next action | Rule id |
|---|---|---|---|
| 未开始 — no project | Empty project workspace states that no project exists | 新建设计 | UX-P5, UX-STATE, UX-DONE |
| 未开始 — project has no designs/runs | Empty project workspace states that no experiment design exists | 新建设计 or 导入设计 | UX-P5, UX-STATE, UX-DONE |
| 未解析 — linked asset unavailable | Original linked path is retained and marked unavailable | 重新定位 | UX-P5, UX-STATE, UX-DONE |
| 未保存 — Core rejected a write | Core cause remains visible with a permission-repair and retry hint | Fix target-directory permission, then retry save | UX-P5, UX-STATE, UX-DONE |

Seed requirements: empty data directory for the first two states; create a real Run asset link, then delete the linked file for 未解析; make Core reject a write using an asset link to a non-existent root for 未保存. Include before/after captures for each changed rendering.
