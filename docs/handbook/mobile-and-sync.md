# 移动端运行边界与双端同步

本节定义 AsterLab 桌面端与 Android/iOS 移动端的运行时边界及零冲突协同策略。

## 移动端运行边界（纯 Go vs Native Core）

当前 `.asterml` 解析与编译校验、`Vault` 目录扫描、`VaultIndex` 增量缓存、**Pure-Go `astercore` 诊断与统计分析（ARRIVE / CCK-8 / RT-qPCR）**、`LiveDocument` 100 步撤销与原子写盘、实验日历以及储藏室货架已 **100% 运行在纯 Go 层**（架构详情见 [分层架构与代码包](./architecture.md)）。

桌面端、移动端与 CLI 均直接运行统一的「**Pure-Go Vault + `astercore` 核心**」，已彻底移除对 `Runs.dll` / `libRuns.so` 与 SQLite 的依赖：

1. **统一离线核心（桌面 / 移动 / CLI 全一致）**
   - 直接读写本地或沙盒内的 `.asterml` 与 `./assets/`，并自动将机器私有状态（`vaultPath`、`lastProjectDir`）隔离到 `.asterlab/local.json`，避免污染随云盘同步的 `.asterlab/config.json`。
   - 实验台上的核心操作——查步骤、勾 Checklist/Subflow、启动/暂停 Timer、填参数偏离、手机拍照挂载、孔板快速录入、查看 AsterCore 统计分析与使用速算器——全部离线秒级运行。

## 双端同步设计路线

由于 AsterLab 原生基于透明文件系统 `<VaultRoot>/`，天然避开了封闭 SQLite 难以合并、容易锁库的问题，双端同步按两阶段演进：

### 阶段一：零服务器文件同步（兼容 iCloud Drive / Syncthing / WebDAV / OneDrive）

在底层直接依靠系统云盘或 Syncthing 同步 `<VaultRoot>/` 目录，并在客户端 `vault` 层补齐三项防线即可安全协同：

1. **生命周期热重载**：App 从后台切回前台或页面聚焦时自动触发 `LiveDocument.ReloadIfExternalChanged()`，立即吸纳桌面端或云端刚落盘的新版本。
2. **基于块 ID 的三路防冲突合并**：
   - 双端同时编辑同一 `.asterml` 时，按照 Frontmatter 键值与各 `step.id` / 模块 `id` 做结构化三路合并（例如桌面修改 Step 1 方案的同时，手机端在 Step 3 勾选 Checklist 与启动 Timer 可自动无损合并）。
   - 仅当两台设备并发修改**同一模块的同一字段**时，将冲突版本另存为 `*.conflict-<device>.asterml` 并高亮提示比对，严禁静默覆盖或丢弃任何实验记录。
3. **`assets/` 分层按需拉取**：原始大图仅在打开对应实验节点时按需拉取或先传缩略图，保障移动端实验台弱网环境下的流畅加载。

### 阶段二：轻量自托管 Blob 同步服务（进阶可选）

保留端侧完全相同的纯 Go 本地读写与按块合并引擎，在外围接入面向 `.asterml` + `sha256` 分块 `./assets/` 的极薄内容寻址同步接口（兼容 S3/OSS/WebDAV 元清单），实现多设备、多成员可控权限共享与增量版本回溯。
