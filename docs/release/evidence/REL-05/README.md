# REL-05 — MyGo 生成绑定的跨平台 hash

结论：同一输入在 Linux 与 Windows 上产出同一组 hash。门禁比的是同一份 golden，
行尾漂移由 `.gitattributes` 从源头消除、由检查器兜底露出，生成器版本固定在三处并强制一致。

| 项 | 值 |
|---|---|
| 生成器 | `github.com/egoist/mygo` |
| pin | `v0.8.4-0.20261108220316-c4197b7fe562`（`prototypes/mygo-run/go.mod`、`scripts/mygo-native-linux.sh`、`scripts/mygo-build.ps1`） |
| pin 文档 | `docs/engineering/cross-platform-builds.md` §3（“Do not float the MyGo pin”） |
| 接口 hash / 格式 | `006d88f0942bea95` / `2` |
| 输入 hash | `f7f998e9dffef5570e36209f3bc6eb7f8f42ad423f8e4d3f724ce7f54aa4afa3` |
| 输出 hash | `e4669b2540cfe36533f8b1676aec53532a184eba4ba1630b73458e2c75d109f7` |
| `runs_native.go` | `c05bab2afa9aeba527058722ecd3bd6932ee41a01002e219ad149c270fd9ea66` |
| `rust/src/mygo.rs` | `f87383cdccc56933ba67b995bf3096dd6a2c9dc499a72baae7345e2157c9b69b` |

输入是两个绑定文件共同的来源 `modules/runs/runs.go` 加 `rust/Cargo.toml` 是否存在；
输出是两个生成文件的规范字节。hash 定义写在 `scripts/ci/native_generator_hash.py`
文件头，golden 在 `scripts/ci/native_generator_hashes.json`。

## 1. 漂移来源（实测，不是推测）

### 行尾 — 唯一现实漂移源，已实测，并已在本分支根治

- 提交的生成文件是纯 LF：`runs_native.go` 36 687 字节、`mygo.rs` 80 458 字节，
  CRLF 计数 0，无 BOM。
- 生成器不受输入行尾影响：把 `runs.go`、`rust/Cargo.toml`、`rust/src/lib.rs`
  全部转成 CRLF 后再生成，两个产物与提交内容逐字节相同，产物内无 CRLF。
- Windows runner 的 checkout 确实是 CRLF：第一轮 CI 的 `--check` 对两个文件都报
  `drift crlf`，归一化后 hash 与 Linux 相同。
- 根因在 MyGo 的生成文件识别：`internal/nativegen/parse.go` 的 `isGenerated` 用
  `bytes.HasPrefix(data, header)` 判断，header 以 `\n` 结尾。CRLF 工作区里
  `runs_native.go` 不被认作生成文件，被当成手写接口源码重新解析，接口 hash 从
  `006d88f0942bea95` 变成 `bb7eb37b2f2b16dd`，`mygo native check` 随即改写已提交的
  绑定。实测隔离（temp copy，只把 `runs_native.go` 转 CRLF）：

  ```text
  crlf={}                                   -> // mygo:native 2 006d88f0942bea95
  crlf={runs_native.go}                     -> // mygo:native 2 bb7eb37b2f2b16dd
  crlf={runs_native.go, rust/src/mygo.rs}   -> // mygo:native 2 bb7eb37b2f2b16dd
  ```

  两个产物的差异只有第 2 行的接口 hash。
- 处理：根目录 `.gitattributes` 让 `*.go`、`*.rs`、`*.toml`、`*.json`、`*.yml`、
  `*.yaml`、`*.md`、`*.sh` 在所有平台按 LF checkout（PowerShell 保持 CRLF）。
  仓库里这些后缀没有 CRLF 文件，因此该文件不触发重规范化 diff。
  检查器另外做规范化（去 BOM、CRLF/CR→LF）并打印 `drift crlf`，接口 hash 不匹配
  golden 时直接失败。

### 时间戳 — 无

生成文件头只有格式号和内容 hash（`// mygo:native 2 <hash>`），hash 覆盖的是
AST 声明（`internal/nativegen/parse.go` 的 `hash()`），不含时间。门禁扫描 ISO 日期与
10 位 epoch，命中即失败。

### map 顺序 — 生成器已排序

`internal/nativegen/gogen.go` 输出 deps 前用 `slices.Sorted(maps.Keys(m.Deps))`，
入口接口按源码位置排序（`parse.go` 的 `sort.Slice`），文件枚举用 `os.ReadDir`
（按文件名排序）。实测同一输入连续两次生成逐字节相同（见 `regen 1/2`）。

### 路径分隔符 — 无，且被门禁拦截

`nativegen.RustDir` 用字符串拼接 `m.Dir + "/rust/src"`，生成内容里没有绝对路径；
门禁扫描盘符、UNC、`/home/`、`/Users/`、`/tmp/` 等绝对路径，命中即失败。

## 2. 一致性门

`scripts/ci/native_generator_hash.py`：

- `--check`：pin 三处一致 → 规范化 → 计算输入/输出/单文件 hash → 与 golden 比对；
  同时校验两个生成文件头的接口 hash 相同。
- `--regen N`：把 `runs.go`、`rust/Cargo.toml`、`rust/src/lib.rs`、`go.mod`、`go.sum`
  复制到临时目录，用固定 pin 跑 `mygo native check` N 次，要求每次规范字节相同且
  等于提交内容。临时副本没有 Rust crate 树，生成后的 cargo 构建必然失败，门禁把
  该失败的最后一行打印出来，不当作通过。
- `--update-golden`：只在复审过的提交里改 golden。

自测 `scripts/ci/test_native_generator_hash.py` 覆盖：CRLF 与 LF 同 hash、内容改动
必须失败、易变内容（时间戳/路径）必须失败、pin 冲突必须失败、golden 冲突必须失败、
缺 golden 视为输入错误。结果 `selftest: 10/10 passed`。

CI：

- `linux-tests`：selftest + `--check` + `--regen 2`（两次生成）。
- `windows-build`：同一组命令；`mygo-build.ps1` 之后再次 `--check` 并检查
  `git status -- prototypes/mygo-run/modules/runs`，构建若改写了已提交绑定即失败。
  每条命令后显式检查 `$LASTEXITCODE`——`pwsh` 只透传 step 里最后一条命令的退出码。

两平台比的是同一份 golden，所以“两平台 hash 相同”是门禁判定，不是声明。

## 3. 第一轮 CI 暴露并修掉的问题

| 问题 | 证据 | 处理 |
|---|---|---|
| Windows checkout 是 CRLF | `--check` 两个文件都报 `drift crlf` | `.gitattributes` 固定 LF |
| CRLF 让 MyGo 误判生成文件并改写接口 hash | 隔离实验：`bb7eb37b2f2b16dd` | 同上；接口 hash 与 golden 不符会让门禁红 |
| 自测 fixture 用平台换行写出，CRLF 用例在 Windows 被二次转换 | `selftest: 9/10 passed` | fixture 一律 `newline="\n"` |
| `pwsh` 只透传最后一条命令退出码，自测失败没让 job 变红 | 第一轮 windows-build 仍 green | 每条命令后 `if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }` |
| 构建后的绑定没人复核 | — | 增加 post-build `--check` + `git status` |

## 4. 运行证据

| 运行 | commit | linux-tests | windows-build | scope-check | 要点 |
|---|---|---|---|---|---|
| 1 | `6f4ff41`（PR merge `cf6961c`） | success | success，但是假绿：`selftest: 9/10 passed`，`pwsh` 没透传退出码 | success | 真实树两个文件 `drift crlf`；归一的 hash 已与 Linux 相同 |
| 2 | `61f7b2c`（PR merge `dc25e106`） | success | success：10/10，`regen 1/2` 相同，`mygo-build.ps1` 后 `git status` 干净 | success | 两平台输入/输出/单文件 hash 完全一致，`drift none` |

文件：

- `linux-generator-hashes.txt`：VPS 本地运行，commit `6f4ff41`（含 `regen 2`）。
- `linux-ci-generator-hashes.txt`：GitHub `ubuntu-24.04` runner，run 2 摘录。
- `windows-generator-hashes.txt`：GitHub `windows-2022` runner，run 2 摘录（生成前后各一次 `--check`）。
- `windows-generator-hashes-run1.txt`：run 1 摘录，CRLF checkout 与自测 9/10 的原始行。
- `linux-go-test.txt`：`cd prototypes/mygo-run && go test ./...` 原始输出。
- `failure-demo.txt`：内容改动与时间戳两类漂移必须失败的演示。

两平台 run 2 的关键行逐字相同（输入、输出、两个文件 hash），只在 `platform=` 上不同。


## 5. 复现

```bash
python3 scripts/ci/test_native_generator_hash.py
python3 scripts/ci/native_generator_hash.py --check --regen 2
```

`--regen` 需要 Go 1.27.3 与网络（首次拉取 pinned MyGo）。

## 6. 残留风险

- golden 只覆盖 `modules/runs`。新增 native 模块必须在 `GENERATED` 与 golden 里补条目，
  否则该模块的跨平台一致性无人检查。
- `.gitattributes` 不覆盖 `*.ps1`/`*.bat`（按 `docs/engineering/cross-platform-builds.md`
  §9 保持 CRLF）；这两个后缀里没有生成文件，风险为零。
