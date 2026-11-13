# SDK-01 — 模块骨架生成器与契约检查器

结论：`scripts/sdk/scaffold.py` 从模板生成可应用的模块骨架（manifest 片段、Go 校验/渲染、注册项、
Rust 镜像声明、最小测试、应用清单）；`scripts/sdk/check.py` 校验 manifest、Go 注册唯一性、schema 递增、
权限合法、catalog 计数和 Go/Rust 字段集合一致。当前 16 个 `kind@schema` 全部通过，生成的骨架在真实
Go 包里编译并通过自带契约测试后才回滚。

| 项 | 值 |
|---|---|
| 基线 commit | `ca69142a3ccaa50b94d01ae591171053d75a7569`（任务 worktree 分支 `feature/sdk-01-build-scaffol-d1r`） |
| Python | 3.12.3 |
| Go | `go1.27.3 linux/amd64` |
| 工具 hash | [`tools-sha256.txt`](tools-sha256.txt) |
| 原始输出 | [`commands.txt`](commands.txt)、[`apply-skeleton.txt`](apply-skeleton.txt)、[`tools-verify.txt`](tools-verify.txt) |
| 机器可读报告 | [`check.json`](check.json)、[`scaffold-dry-run.txt`](scaffold-dry-run.txt) |

审查 head 的 commit SHA 记录在 PR 与任务评论；本文只固定基线与工具字节。

## 1. 新增文件

| 文件 | 作用 |
|---|---|
| `scripts/sdk/manifest_spec.py` | 读取 `spec/modules.json`，从 `spec.go`/`registry.go`/`modules.go`/`src/cli.rs` 静态解析规则名、端口类型、类别、权限和注册顺序；共用校验函数 |
| `scripts/sdk/check.py` | 七个检查项 + 文本/JSON 报告；退出码 0/1/2 |
| `scripts/sdk/scaffold.py` | 生成骨架；拒绝重复 `kind@schema`、schema 跳号、未知类别/权限、未双端注册的 rule；支持 `--dry-run`/`--force`/`--out` |
| `scripts/sdk/test_sdk_tools.py` | 16 个 fixture 测试，覆盖通过与全部拒绝路径 |
| `docs/sdk/module-development-guide.md` §2、§7 | 增加 rule 双端注册清单项；用实际工具替换“无生成器”的旧描述 |

## 2. 检查项（`scripts/sdk/check.py`）

| id | 通过条件 |
|---|---|
| `manifest` | `schemaVersion`/`foundationVersion` 为 1；`kind` 匹配 `[a-z][a-z0-9_]*`；每模块有 `title`、合法 `sdk`/`category`；端口类型已注册；reference 有 `target`；derived 有 `calculator`；`rules[]` 已在 Go 注册；生命周期/provenance 合法；`kind@schema` 不重复 |
| `registration` | `BuiltinRegistry()` 的构造器逐个解析出 `kind@schema`，与 manifest 双向相等且无重复注册 |
| `schema` | 每个 kind 的 schema 恰为 `1..N`，无跳号、无重复 |
| `permissions` | `actions[].permission` 在模块 `permissions[]` 内；`permissions[]` 都有 Go `Perm*` 常量 |
| `catalog` | `docs/sdk/module-catalog-v1.md` §1 表行集合等于 manifest 键集合；文中 `N 个 kind@schema` 与 `N 个 kind` 的声明都等于实际计数 |
| `parity` | Go `knownModuleRules`/`foundationPortTypes`/`foundationCategories` 与 Rust `ASTERDOC_RULES`/`types`/`categories` 相等；Rust `validate_asterdoc_module` 的 match 臂集合等于 `ASTERDOC_RULES` |
| `scaffold` | 每个 `_scaffold/<kind>@<schema>/` 六件齐全；`manifest.json`、`module.rs` 声明与目录名一致；片段通过 `manifest`/`permissions` 规则；已并入 manifest 的骨架会报 stale |

失败路径：任一检查失败打印具体键名并以 1 退出；输入不可读/JSON 损坏以 2 退出，不静默跳过。

## 3. 验收映射

| 验收命令 | 期望 | 实测 |
|---|---|---|
| `python3 scripts/sdk/check.py` | exit 0 | exit 0，7 项全 `ok`（[`commands.txt`](commands.txt)） |
| `python3 scripts/sdk/scaffold.py --kind probe_tmp --schema 1 --dry-run` | exit 0 且打印将创建的文件清单 | 6 个 `would create ...`，`dry run: no file written`，`git status` 无变化 |
| `python3 scripts/sdk/check.py --json` | 合法 JSON 且 `modules` 等于 `modules.json` 的 `modules[]` | `modules.json modules[]=16 check --json modules=16 match=True`（[`check.json`](check.json)） |

## 4. 骨架可用性（实测，非推断）

[`apply-skeleton.txt`](apply-skeleton.txt) 记录的完整路径：

1. `scaffold.py --kind probe_tmp --schema 1` 生成 6 个文件到 `_scaffold/probe_tmp@1/`；
2. 按骨架 README 应用：追加 manifest 片段、复制 `module.go`/`module_test.go`、在 `BuiltinRegistry()`
   增加 `probeTmpSpec(),`；
3. `go test ./internal/asterdoc -run TestProbeTmpSpecContract -count=1 -v` → `PASS`；
4. 回滚临时应用，`git status --porcelain prototypes/mygo-run/internal/asterdoc` 为空。

`_scaffold/` 以 `_` 开头，Go 工具忽略该目录：骨架存在时 `go test ./internal/asterdoc` 仍正常编译，
不会产生重复包或重复注册。

## 5. 限制（如实陈述）

- **Windows runner 未执行**：本 VPS worktree 只有 Linux 证据。两个脚本只用 Python 标准库
  （`argparse`/`json`/`pathlib`/`re`/`unittest`），无 `subprocess`、无 POSIX 专用调用、写入统一
  `newline="\n"`；但“Windows 实跑”仍需 CI 或 Windows Gate 增加一步才算证据，本文不宣称已满足。
- `check.py` 为做 Go/Rust 字段集合对比读取 `src/cli.rs`，属**只读**；本卡没有修改 `src/**`、`crates/**`、
  `docs/contracts/**` 或 `spec/modules.json`。
- `docs/sdk/compatibility-and-gaps.md` 与 `docs/sdk/sdk-readiness-audit.md` 仍写“无 scaffold 工具”。
  二者不在本卡 `write` 范围，需单独 Docs 卡对齐。
- `manifest` 检查要求 `kind` 匹配 `[a-z][a-z0-9_]*`；Go/Rust 运行时只要求非空字符串。该约束由检查器
  为目录名与 Go 标识符安全新增，未改变运行时语义；现有 16 个 kind 全部满足。
