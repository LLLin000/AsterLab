# AsterLab 外部 Agent 接入指引（v2）

你正在通过命令行端口访问本机 AsterLab 实验工作台（无 GUI 介入）。

## 1. 程序与数据目录

- 命令行工具：`labworkbench.exe`（可置于安装包目录或工作目录）
- 用户数据目录：默认 `%LOCALAPPDATA%\AsterLab\data`；如已设置环境变量 `LABWORKBENCH_DATA_DIR`，优先使用该目录。
- 桌面端本地实验库（Obsidian 模式）：默认 `%LOCALAPPDATA%\AsterLab\vault`（支持 `ASTERLAB_VAULT_DIR` 覆盖），以透明 `.asterml` 单真源单文件形式组织（`.asterlab/`、`assets/`、`templates/` 与 `<项目文件夹>/*.asterml`）；直接编辑磁盘 `.asterml` 后桌面端会自动感知重载，支持内联字段 `checklist.items[].done`、`param_table.rows[].actual`、`timer.status/elapsed_sec`、`plate.layers/excluded_wells`、`calculator.rows` 与 `data_table.source_ref`。
- 每次调用 `asterdoc.save` / `asterdoc.command` 后，真值写入 `<data_dir>/labworkbench.sqlite`，并同步写入磁盘镜像 `<data_dir>/experiments/<design_id>/experiment.asterml`。

## 2. 最小接入流程（冷启动顺序）

1. **查看/创建项目与设计**（所有业务命令均需传入 `--input <json文件|->`）：
   ```powershell
   '{}' | .\labworkbench.exe project.list --input -
   # 若为空先建项目，记录 result.record.id：
   '{"name":"默认实验空间"}' | .\labworkbench.exe project.create --input -
   # 创建设计，记录 result.design.id（此时 AsterDoc revision 为 0，design.revision 为 1）：
   '{"project_id":"<project_id>","design":{"title":"新实验","experimental_unit":"独立培养孔"}}' | .\labworkbench.exe design.create --input -
   ```
2. **可选参考储藏室库存**：
   - 可运行 `'{}' | .\labworkbench.exe pantry.list --input -` 查询已有在库试剂的短码 `label_code`（`PN-*`），在 `material.pantry_code` 中引用。
   - **储藏室绝非强制前置条件**：用户未建库的试剂、细胞与耗材可直接写入实验设计，无需预先 `pantry.upsert`。
3. **写前自省与示例**：
   - 查看 21 个通用模块摘要：`'{}' | .\labworkbench.exe sdk.modules --input -`
   - 查看单模块精确字段、枚举值与 `object-list` 内层键：`{"kind":"plate","schema":1} | .\labworkbench.exe sdk.describe --input -`
   - 完整标准示例：`examples/canonical_96well.asterml`（建议先读再改，可显著减少试错）。
4. **局部编辑、强校验与单位**：
   - 整篇首次保存使用 `asterdoc.save`（首次保存 `expected_revision: 0`）；后续增删改移步骤/板块优先使用 `asterdoc.command`；
   - 校验与就绪检查：`asterdoc.validate --diagnostics json`、`asterdoc.assess --diagnostics json`；
   - 未知字段会被严格拒绝并列出合法字段白名单；跨模块数量矛盾（如 `calculator.sample_count` 与 `plate_ref` 总孔数不一致、`group.independent_n` 与孔板分配矛盾）会触发结构化 warning；
   - `quantity` 类型**必须**写为 `{value: 数值, unit: "UCUM单位"}` 对象（例如 `{value: 100, unit: uL}`、`{value: 2, unit: h}`；温度用 `Cel`，体积支持 `L/mL/uL/nL/mm3`，电压支持 `V`）；
   - 多步骤共享同一检测 `assay` 时，勿将同一 `timepoint` 重复挂到多个步骤上，只标注在真实采集读数的步骤即可。
   - 容器坐标约定：`plate` / `dish`（`format:4` 为 `A1..B2`）/ `cohort`（如 `format:24` 为 `A1..D6`）使用行列坐标；`rack_1d` 使用 `"1".."N"` 泳道/槽位号（未归组的 Marker 泳道放 `roles` 而不放 `samples`）；若填写 `samples` 建议成对补全 `replicates`。
5. **冻结运行**：确认 `asterdoc.assess` 返回 `state: "ready"`（`errors: []`）后，调用 `asterdoc.create_run` 冻结运行。
