# LB-18 证据：V2 设计系统一致性规范

基线：`b2acf22`（origin/main），2026-11-16。

## 1. 反模式基线（`prototypes/mygo-run`，排除 `_test.go`）

```text
$ grep -rn "ui.Hex(" --include=*.go . | grep -v _test.go | grep -v internal/theme/ | wc -l
30

$ grep -rn "FontSize([0-9]" --include=*.go . | grep -v _test.go | wc -l
186

$ grep -rn "Padding([0-9]" --include=*.go . | grep -v _test.go | wc -l
100

$ grep -rn "Radius([0-9]" --include=*.go . | grep -v _test.go | wc -l
59
```

按包分布（同一基线）：

```text
FontSize(数字)  186 | runui=80 root=49 protocol=25 runworkspace=22 tools=9 resultgrid=1
ui.Hex(...)      30 | uikit=16 asterdoc=3 theme=11（允许，是定义处）
Padding(数字)   100 | runworkspace=31 runui=23 root=16 asterdoc=10 resultgrid=7 designer=4
Radius(数字)     59 | runworkspace=30 runui=19 resultgrid=5 asterdoc=3 root=1 uikit=1
```

细则（真实行号）：

- 非 4 倍数 Padding：`internal/resultgrid/view.go:54,60,81`（`Padding(6)`）。
- 不在 6/10/12 档的 Radius：`internal/asterdoc/view.go:493`(3)、`internal/resultgrid/plate.go:124,474`(5)、`plate.go:417`(8)、`internal/runui/runui.go:630,725,730,755`(8)。
- 已合规范式：`internal/designer/step_list.go` 的 `budgetBar` / `listToolbar` / `sectionSwitch` 全部使用 `t.Space(n)` + `t.Rem(x)` + `components.*`。

## 2. 文档挂载（grep 原始输出）

```text
AGENTS.md:61:- 动任何界面之前先读 [`docs/product/design-system.md`](docs/product/design-system.md)：它规定了唯一 token 源、4 倍数间距、三档圆角、需求→组件对照表（Do/Don't）与文案规则。
AGENTS.md:64:- 需要新组件时，先确认仓库与 onevra 都没有等价实现，并在 `design-system.md` 的组件表登记原因。
docs/product/ux-contract.md:173:`docs/product/design-system.md` is the enforceable visual contract behind P3: the
```

## 3. 规范条目计数

```text
$ grep -c '^- Do:' docs/product/design-system.md
12
$ grep -c "^- Don't:" docs/product/design-system.md
12
```

## 4. 结论

规范落在三处：

- `docs/product/design-system.md`：单一 token 源、4 倍数间距、三档圆角、需求→组件对照表、文案规则、反模式基线与机器检查命令、12 组 Do/Don't 范例。
- `docs/product/ux-contract.md` §8：与 P3 绑定，规则 id `UX-DESIGN`。
- `AGENTS.md` §3「UI 工作方法（强制）」：开工前必读，组件按语义选，PR 带 `## 设计一致性`。

反模式清理不在本卡，按第 1 节清单另开卡执行。
