# AsterLab 6-Layer Release QA & Dogfooding Guide

参考 Obsidian、Notion、VS Code 与 Linear 的桌面编辑器质量工程体系，AsterLab 采用 **6 层发布前质量门禁**，在速度（每次 PR 秒级反馈）与覆盖深度（防丢数据、防卡死、防视觉回退、防内存泄漏）之间取得平衡。

---

## 自动化 5 层（已接入 Go 测试套件）

| 层级 | 对应测试文件 | 验证目标 | 执行时机 |
|---|---|---|---|
| **0. 状态机与随机操作组合 Fuzzer** | `prototypes/mygo-run/interaction_fsm_test.go` | 损坏 `workspace.json` 自愈、多文档切换、分屏与三模式、160 步 17 类操作随机组合 + 7 条全局不变量 | 每次 PR 自动跑 |
| **1. 数据安全与文件系统对抗 (Chaos FS)** | `prototypes/mygo-run/qa_chaos_scale_test.go` (`TestQA1_*`) | 截断/损坏的 `config.json`、`workspace.json`、`pantry.jsonl`、未闭合 `.asterml` 不崩溃、不静默清空用户数据；UTF-8 BOM 与重复 ID 兼容 | 每次 PR 自动跑 |
| **2. 中文输入法与实时编辑鲁棒性 (IME/CJK)** | `prototypes/mygo-run/qa_ime_editing_test.go` (`TestQA2_*`) | `/` 触发边界（`1/1000` 与 URL 不误触发）、多字节 UTF-8 装饰切分安全、中文打字/退格/撤销重做 | 每次 PR 自动跑 |
| **3. 规模与响应性能基准 (Scale & Latency)** | `prototypes/mygo-run/qa_chaos_scale_test.go` (`TestQA3_*`) | 200 份 `.asterml` Vault 扫描/索引/1.2s 目录监听签名/全文搜索耗时门禁 + 24 模块长文档稳态帧耗 `<500ms` | 每次 PR 自动跑 |
| **4. 视觉回归矩阵 (Visual Regression)** | `prototypes/mygo-run/qa_visual_regression_test.go` (`TestQA4_*`) | 12 组场景：浅色/深色 × `900×600`/`1360×880`/`1440×900`/`2560×1440` × `100%`/`125%` 缩放 × 日历/编辑/实验模式/96孔密集/损坏报错/储藏室弹窗/设置弹窗/分屏；非白屏方差校验 + `ASTERLAB_VRT_DIR` / `ASTERLAB_VRT_STRICT=1` 强制像素对比 | 每次 PR + Release 自动跑 |
| **4.5. 全链路 4-SOP 场景与帧耗 p95** | `prototypes/mygo-run/qa_scenario_test.go` (`TestScenario_*`) | HUVEC / CFU / 抑菌圈 / RT-qPCR 完整 9 阶段闭环（创建→编辑→联动→离线 AI 建议→实验模式→拍照关联→重启恢复→外部热更新→PDF 导出）、非法单位/并发冲突不变量与 96 孔密集页热帧 p95 耗时门禁 | 每次 PR + Release 自动跑 |
| **5. 长时间浸泡与泄漏测试 (Soak — 独立隔离)** | `prototypes/mygo-run/soak_test.go` (`//go:build soak`) | 600～5000+ 步连续混合负载，监测 `NumGoroutine` 泄漏、`HeapAlloc` 增长与 p95 帧耗 | 发大版本前 / 有空时单独手动跑 |
| **5.5. 真机 Windows GUI E2E（Pure-Go Vault）** | `scripts/gui-e2e-vault.ps1` (`mygo-automation`) | 在完全隔离的临时 Vault 上启动真实 `-tags e2e` Windows 窗口，执行新建 HUVEC/CFU、多标签切换、实验模式、储藏室、设置、CLI 外部修改热同步与重启恢复，自动截图并生成 `qa-report.md` | Windows Gate / 发版前必跑 |

### 常用执行命令

```bash
# 1. 统一 PR & Release 门禁（linux-tests.yml / windows-build.yml / release.yml 同源）
cd prototypes/mygo-run
go test -count=1 -v -run "^TestFSM_|^TestQA[1-4]_|^TestScenario_" .

# 2. 视觉回归像素基准模式（生成基准 / 严格门禁对比）
ASTERLAB_VRT_DIR=/tmp/asterlab-vrt ASTERLAB_UPDATE_VRT=1 go test -count=1 -v -run TestQA4_ .
ASTERLAB_VRT_DIR=/tmp/asterlab-vrt ASTERLAB_VRT_STRICT=1 go test -count=1 -v -run TestQA4_ .

# 3. 真机 Windows GUI E2E（mygo-automation + 隔离 Vault + 自动产出 qa-report.md）
pwsh -File scripts/gui-e2e-vault.ps1

# 4. Layer 5 独立浸泡测试（有空时跑，可按需调大步数）
go test -tags soak -v -timeout 30m -run TestQA5_Soak .
ASTERLAB_SOAK_STEPS=3000 go test -tags soak -v -timeout 30m -run TestQA5_Soak .
```

---

## Layer 6：分级发布（Insider Pre-release）与 5 分钟真机手工核对表

无头测试无法完全替代真实操作系统输入法、高分屏缩放与安装包覆盖升级。遵循 **Pre-release (`v0.1.x`) 先自用 → 稳定后升正式版 (`v0.2.0`)** 策略，每次发正式版前花 5 分钟在真机核对以下 5 项：

- [ ] **真实中文 IME（微软拼音 / macOS 拼音）**：在空白实验与已有实验内输入中文、选词、退格、敲行首 `/` 选模块、输入 `1/1000` 确认不误弹菜单、按住 `Alt` 悬停触发 AI 建议卡不闪烁。
- [ ] **双文档与分屏切换**：在真实 Vault（如 `D:\L\AsterLab`）同时打开 2 份以上实验、来回切标签、向右分屏、关闭后重启确认无僵尸标签且自动记住当前 Vault。
- [ ] **储藏室隔离与单文件同步**：确认 `.asterlab/pantry.jsonl` 独立于其他库；单击试剂瓶贴纸/表格名弹出详情窗，新增收纳盒与试剂正常落盘。
- [ ] **外部 Agent 实时热更新**：外部往 Vault 新写一份 `.asterml` 或往 `.asterlab/pantry.jsonl` 追加一行，~1.2 秒内软件自动刷新出现。
- [ ] **高分屏与覆盖更新**：在 125% / 150% DPI 缩放下无按钮裁切或文字重叠；覆盖安装或应用内更新后启动 Watchdog 正常且原有配置不丢失。
