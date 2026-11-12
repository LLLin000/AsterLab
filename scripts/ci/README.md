# scripts/ci

REL-04 的 CI 入口。三个必需的 PR 检查：

| 检查 | 工作流 | 内容 |
|---|---|---|
| `linux-tests` | `.github/workflows/linux-tests.yml` | L0：`cargo build --locked` 三个 target；L1：`cargo test --locked` 三个 suite（HANDOFF-VPS.md §3） |
| `windows-build` | `.github/workflows/windows-build.yml` | `scripts/mygo-build.ps1` → `mygo-run.exe` + `Runs.dll`，产出 `BUILD_INFO.txt` 与 `SHA256SUMS.txt` 并上传 artifact；不做 GUI E2E |
| `scope-check` | `.github/workflows/scope-check.yml` | 解析卡片 `kandev-scope` 块，对 `git diff --name-status <base>...HEAD` 判越界；同时跑 `actionlint` 和解析器自测 |

## scope 块

卡片描述的 `<!-- kandev-scope ... -->` 块只允许列路径：

```text
<!-- kandev-scope
write:
  - .github/workflows/
  - scripts/ci/
read:
  - docs/engineering/kandev-card-template.md
forbid:
  - crates/**
  - src/**
-->
```

判定规则（`scripts/ci/scope_check.py`）：

- `forbid` > `write` > `read`；三个列表都没命中的路径算越界。
- 结尾带 `/` 的条目覆盖整棵子树；普通路径覆盖自身与子树；含 `*`、`?`、`[` 的条目按 shell glob 匹配，`*` 可以跨 `/`（`crates/**` 覆盖 `crates/a/src/lib.rs`）。
- `read` 是只读引用，出现在 diff 里即为违规。
- 条目必须是仓库相对路径；`all source files` 这类自由文本直接报错，不会静默放过。
- 重命名同时检查源路径与目标路径。

退出码：`0` 合规，`1` 有越界文件，`2` 输入不可用（缺 scope 块、格式错误、API 读不到）。

越界时输出示例：

```text
scope-check: 5 changed path(s), 2 allowed, 3 violation(s)
scope-check: FAILED - the pull request touches paths outside the card scope
::error file=src/cli.rs::scope-check: not listed in the `write` list
  out of scope: src/cli.rs (not listed in the `write` list)
  out of scope: docs/contracts/core-v1.md (forbidden by `docs/contracts/**`)
  out of scope: README.md (read-only entry `README.md` must not be modified)
```

## 描述来源

`scope_check.py` 按以下优先级取任务描述：

1. `--task-id <uuid>`：调 `GET {KANDEV_API_URL}/api/v1/tasks/<uuid>`，读 `description`。
2. `--pr-body-file <file>`：先找 `<!-- kandev-task: <uuid> -->`。找到且 `KANDEV_API_URL`、`KANDEV_API_TOKEN` 都有值时走 API；否则读 PR body 里的 `kandev-scope` 块，并在日志打 `notice:`。
3. `--description-file <file>` 或 `--description-stdin`。

GitHub 托管 runner 无法访问 `127.0.0.1:38429` 上的 Kandev，因此托管 runner 走第 2 条的 PR body 回退；自托管 runner 配好 `KANDEV_API_URL` / `KANDEV_API_TOKEN` secrets 后自动切到 API。两条路径都失败即 exit 2，不跳过检查。

PR body 必须带 `<!-- kandev-task: <uuid> -->` 和卡片的 `kandev-scope` 块，前者让 API 路径可启用，后者是没有 API 时的唯一依据。

## 本地运行

```bash
# 解析器自测：故意越界的 fixture 必须 exit 1
python3 scripts/ci/test_scope_check.py

# 对当前分支跑真实 diff
python3 scripts/ci/scope_check.py \
  --description-file /tmp/card.md \
  --base origin/main --head HEAD

# 直接读 API（VPS 上有效）
KANDEV_API_URL=http://127.0.0.1:38429 KANDEV_API_TOKEN=... \
  python3 scripts/ci/scope_check.py --task-id <uuid>

# workflow YAML
scripts/ci/install-actionlint.sh /tmp/actionlint-bin
/tmp/actionlint-bin/actionlint
```

actionlint 固定在 1.7.7，`install-actionlint.sh` 用 SHA256 校验下载包。

## 设为 PR 必需

仓库设置，不是文件。REL-04 已对 `main` 执行过，复现命令：

```bash
cat > /tmp/protection.json <<'JSON'
{
  "required_status_checks": {
    "strict": false,
    "contexts": ["linux-tests", "windows-build", "scope-check"]
  },
  "enforce_admins": false,
  "required_pull_request_reviews": null,
  "restrictions": null,
  "required_linear_history": false,
  "allow_force_pushes": false,
  "allow_deletions": false,
  "required_conversation_resolution": false
}
JSON
gh api -X PUT repos/LLLin000/AsterLab/branches/main/protection --input /tmp/protection.json
```

`strict: false`：不要求 PR 分支每次先跟上 `main`，避免与多 agent 并行冲突；三个 context 名称必须与 job `name` 完全一致。
副作用：workflow 文件只在合入 `main` 之后才对其他 PR 可见，早于本 PR 打开的分支必须先更新到含这三个 workflow 的 `main` 才能合并。

## 非目标

- 不采购或配置 Windows runner；`windows-build` 跑在 GitHub 托管的 `windows-2022`。
- 不做 GUI E2E、安装包、签名；这些属于 Windows Gate。
- `apps/desktop`（Rust/Slint 研究原型）不在此构建；交付用桌面产物是 MyGo，见 `docs/engineering/cross-platform-builds.md` §4。
