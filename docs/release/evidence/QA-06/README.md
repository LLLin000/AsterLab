# QA-06 — 文件引用与本地安全边界

结论：`asset.link` 只持久化逻辑 root ID、相对路径和文件观测值；导出物（`asset.link` /
`asset.list` / `asset.events` / `record.list`）与数据库 `data_json` 都不含绝对路径。
越权相对路径、盘符/UNC 前缀和逃出 root 的符号链接一律返回 `INVALID_INPUT`，且被拒绝的
请求不写记录、不改数据库。

## 1. 测试矩阵与结果

| 类别 | 用例 | 期望 | 证据 |
|---|---|---|---|
| 正常 | 根内文件 `d1/readout.csv` link | `relativePath=d1/readout.csv`，`rootId=project-data`，导出无绝对路径 | `export-scan.txt` |
| 正常 | 传绝对 `path`，由 root 反推相对路径 | 派生 `d1/readout.csv`，与 `relative_path` 一致 | `export-scan.txt` |
| 正常 | 根内符号链接（`readout.csv` → `real/readout.csv`） | 允许，导出仍是逻辑相对路径 | `assets-tests.txt`（`symlinks_that_stay_inside_the_root_are_allowed`） |
| 边界 | 根顶文件、含空格文件名、深层相对路径、`a:b.csv` 按平台判定 | 通过；Windows 盘符相对前缀拒绝 | `assets-tests.txt`（`normalize_rejects_escapes_and_unnormalized_paths`） |
| 边界 | 导出 JSON 只含 `relativePath`/`rootId` | 不含 `root_dir`、文件绝对路径、隔离目录 | `export-scan.txt` |
| 失败 | 绝对 `root_id`：本机 root、`/var/data`、`C:`、`C:/data`、UNC、`roots/../escape` | `root ID must be a logical identifier...` / `root ID must not contain parent-directory segments` | `export-scan.txt`、`assets-tests.txt`（`root_id_must_be_logical_not_an_absolute_path`） |
| 失败 | 相对路径 `..`、`d1/..`、`d1/../../x`、`/etc/passwd`、`C:/Windows/win.ini`、UNC | `relative path must not escape its root` / `must not be absolute` | `export-scan.txt` |
| 失败 | 文件符号链接、目录符号链接指向 root 外 | `asset resolves outside its root` | `export-scan.txt`、`assets-tests.txt` |
| 失败 | `path` 指向 root 外文件 | `asset is outside its root` | `export-scan.txt` |
| 失败 | 有效引用被替换成越界符号链接后的 `inspect` | 显式报错，不返回 `available` | `assets-tests.txt`（`symlinks_that_leave_the_root_are_rejected`） |
| 重启 | 序列化 → 反序列化 → `inspect` available；文件移动后 Missing；`rebind` 保留 ID | 一致 | `assets-tests.txt`（`exported_reference_contains_no_absolute_path_and_survives_a_restart`） |
| 并发 | 8 线程并发 link/inspect | ID/hash/availability 一致，无副本文件 | `assets-tests.txt`（`concurrent_links_and_inspections_agree_without_writing`） |

## 2. 隔离数据与校验和

`scan.py` 每次在 `CARGO_TARGET_DIR`（回退 `target/`）下新建 `qa06-*` 目录，内部的
`data/` 是本用例唯一数据库，`root/` 是被引用目录，`outside/` 是存放 `secret.csv` 的根外
目录。全程不接触用户主库。

2026-11-13 本轮结果（`export-scan.txt`）：

```text
sha256 f606b1bd18d80dd37ebf9e9f4375fbd790e23ce44f81e8e02812b5b104951341  .../qa06-rxvpj1_o/data/labworkbench.sqlite
sha256_before_negative_cases f606b1bd18d80dd37ebf9e9f4375fbd790e23ce44f81e8e02812b5b104951341
sha256_after_negative_cases  f606b1bd18d80dd37ebf9e9f4375fbd790e23ce44f81e8e02812b5b104951341
asset_rows_before=3 asset_rows_after=3
```

11 个负向用例（3 个 `root_id`、2 个 `..`、2 个绝对/盘符相对、2 个符号链接、1 个根外
`path`）全部 `ok:false`、`INVALID_INPUT`，前后校验和与行数不变。

## 3. 复现步骤

```bash
CARGO_TARGET_DIR="${CARGO_TARGET_DIR:-$PWD/target}"   # 不改写，仅供脚本定位产物
cargo build --locked --bin labworkbench
python3 docs/release/evidence/QA-06/scan.py            # 重写 export-scan.txt
cargo test --locked --manifest-path crates/asterlab-core/Cargo.toml
cargo test --locked --bin labworkbench
cargo test --locked --manifest-path crates/asterlab-app/Cargo.toml
```

原始输出：

| 文件 | 命令 | 结果 |
|---|---|---|
| `assets-tests.txt` | `cargo test --locked --manifest-path crates/asterlab-core/Cargo.toml --lib assets::` | 20 passed |
| `core-tests.txt` | `cargo test --locked --manifest-path crates/asterlab-core/Cargo.toml` | 246 + 3 + 6 passed |
| `cli-tests.txt` | `cargo test --locked --bin labworkbench` | 13 passed |
| `app-tests.txt` | `cargo test --locked --manifest-path crates/asterlab-app/Cargo.toml` | 19 passed |
| `fixture.txt` | `python3 scripts/rc3-fixture.py`（`ASTERLAB_DATA_DIR` 指向隔离目录） | Run 与两个 task |
| `export-scan.txt` | `python3 docs/release/evidence/QA-06/scan.py` | 绝对路径命中 0，负向用例全部拒绝 |

## 4. 代码改动

只改 `crates/asterlab-core/src/assets.rs`：

- `validate_root_id`：`link()` 的 `root_id` 会持久化并进入导出，现在拒绝空值、控制字符、
  超长、`/`/`\` 开头、`C:`/`C:/`/`C:\`、UNC 与 `..` 段。`root_dir` 仍只用于本次调用，
  不落库。
- `starts_with_drive_prefix`：`normalize_relative_path()` 在所有平台拒绝 Windows 盘符
  前缀。此前 Linux 上 `C:/x` 被当作普通相对名保存，数据目录迁到 Windows 后就成了绝对
  路径。

## 5. 已知边界（不在本卡范围）

- 遗留 `save_asset_link`（`src/cli.rs`，MyGo Run 模块入口）直接构造 `AssetRef`，只经过
  CLI 的 `check_asset_root_id`（仅长度/控制字符）；`src/**` 在本卡 `forbid` 内，未改动。
  当前 Go 调用方传逻辑 root（`project-data`），本轮扫描未发现它写入绝对路径。
- `resolve()` / `link()` 的错误消息包含本次传入的 `root_dir`，用于定位本机故障；它们是
  即时错误文本，不进入记录或导出物。
