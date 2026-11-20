# Vault 存储与原子读写

AsterLab 的本地存储完全基于普通操作系统目录（Vault），不依赖专有二进制数据库锁定。任何文本编辑器、Git、网盘或外部 AI Agent 均可直接读写 `.asterml` 与 `./assets/`。

## 目录结构契约

```text
<VaultRoot>/
├── .asterlab/
│   ├── config.json         # 用户配置：主题模式/预设/LiquidGlass、快捷键、移动端手势、默认板型与配液余量
│   ├── workspace.json      # 分屏布局、已打开页签与当前焦点面板状态
│   ├── trash_meta.json     # 软删除原路径与时间戳元数据
│   └── trash/              # 回收站文件备份目录
├── assets/                 # ./assets/<file> 相对路径图片与附件（Ctrl+V / 拖拽自动存盘）
├── templates/              # 空白方案模板（由 ClearExecutionDataForTemplate 去执行态生成）
└── <ProjectFolder>/
    └── *.asterml           # 带 YAML Frontmatter 与 asterlab 围栏的标准实验文档
```

## `vault.Vault` 文件操作接口

实现位于 [vault.go](../../prototypes/mygo-run/internal/vault/vault.go)：

- **目录扫描 (`ScanTree`)**：扫描真实目录树并按文件类型与层级排序，同时识别可归档至日历的实验日期索引。
- **文件与项目操作**：`CreateExperiment`（可指定模板）、`CreateFolder`、`Rename`、`Move`、`DuplicateFile`、`SaveAsTemplate`。
- **软删除 (`Trash`)**：将文件移入 `.asterlab/trash/` 并在 `trash_meta.json` 记录原位置，避免误删不可恢复。
- **便携附件导入 (`ImportAssetBytes`)**：剪贴板 `Ctrl+V` 贴图或拖拽本地文件时自动写入 `assets/` 并返回便携 `./assets/...` 引用路径。

## `DocStore` 与 `LiveDocument` 单例共享

实现位于 [live_doc.go](../../prototypes/mygo-run/internal/vault/live_doc.go)：

- **多窗格单例缓存**：`DocStore.Open(relPath)` 以规范化相对路径作为键缓存唯一的 `*LiveDocument`。无论左侧编辑区与右侧分屏同时打开同一文件几次，所有视图共享同一份内存状态。
- **统一 100 步撤销栈 (`maxUndoStack = 100`)**：
  - 所有结构化界面操作（`ApplyEdit`，如勾选检查项、记录参数偏离、填入孔板数值）与纯文本源码编辑（`ApplySourceDraft`）统一进入同一个 `undoStack / redoStack` 环。
- **原子落盘 (`WriteFileAtomic`)**：写入同目录临时文件并执行 `Sync` 刷盘后 `os.Rename` 覆盖原文件，避免写半截崩溃损坏实验记录。
- **外部修改检测 (`ReloadIfExternalChanged`)**：记录 `DiskMtime`，当检测到外部 Agent、编辑器或云盘同步更新了磁盘文件时自动热解析刷新视图。
