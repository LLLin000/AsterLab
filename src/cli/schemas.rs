use super::*;

pub(crate) fn tool_specs() -> Vec<Value> {
    let names = [
        "project.list",
        "project.create",
        "project.revise",
        "design.list",
        "design.get",
        "design.map",
        "design.create",
        "design.revise",
        "run.list",
        "run.create",
        "run.set_status",
        "run.revise",
        "run.workspace",
        "record.projection",
        "record.sign_archive",
        "record.archive_state",
        "task.create",
        "task.set_status",
        "protocol.list",
        "protocol.version.get",
        "protocol.version.list",
        "protocol.block.list",
        "protocol.block.get",
        "protocol.block.create",
        "labware.define",
        "component.list",
        "component.define",
        "view.project",
        "view.data_preview",
        "view.create",
        "view.get",
        "view.revise",
        "semantic.inspect",
        "semantic.search",
        "semantic.apply_binding_patch",
        "semantic.validate",
        "semantic.propose_patch",
        "semantic.assess_patch",
        "catalog.review.record",
        "catalog.operation.import",
        "protocol.block.revise",
        "protocol.block.preview",
        "protocol.create",
        "protocol.revise",
        "agent_edit.validate",
        "agent_edit.commit",
        "step.edit",
        "plate.create",
        "well.set",
        "note.create",
        "execution_event.create",
        "pantry.list",
        "pantry.upsert",
        "pantry.delete",
        "label.export",
        "readout.submit",
        "readout.batch",
        "formula.create",
        "formula.revise",
        "table.create",
        "table.revise",
        "record.list",
        "record.get",
        "asset.link",
        "asset.link.update",
        "asset.unlink",
        "asset.list",
        "asset.events",
        "import.study.preview",
        "import.study.apply",
        "calculator.dilution",
        "calculator.solution",
        "calculator.serial_dilution",
        "calculator.reaction_volume",
        "calculator.gel_mastermix",
        "calculator.cell_seeding",
        "calculator.runtime_normalize",
        "calculator.sample_normalize_batch",
        "calculator.in_vivo_dosing",
        "calculator.biofilm_cfu",
        "sdk.modules",
        "sdk.validate",
        "sdk.describe",
        "asterdoc.inspect",
        "asterdoc.assess",
        "asterdoc.validate",
        "asterdoc.load",
        "asterdoc.save",
        "asterdoc.command",
        "asterdoc.compile",
        "asterdoc.create_run",
    ];
    names.iter().filter_map(|n|Some(json!({"name":n,"description":tool_description(n),"side_effects":tool_effects(n),"writes_data":tool_writes(n),"command":format!("labworkbench {n} --input <json-file|->"),"input_schema":input_schema(n)?,"output_schema":output_schema(n)}))).collect()
}
pub(crate) fn tool_writes(tool: &str) -> bool {
    !matches!(
        tool,
        "project.list"
            | "pantry.list"
            | "label.export"
            | "design.list"
            | "design.get"
            | "run.list"
            | "workflow.inspect_tree"
            | "workflow.progress"
            | "protocol.list"
            | "protocol.version.get"
            | "protocol.version.list"
            | "protocol.block.list"
            | "protocol.block.get"
            | "protocol.block.preview"
            | "agent_edit.validate"
            | "component.list"
            | "semantic.inspect"
            | "semantic.search"
            | "semantic.validate"
            | "semantic.propose_patch"
            | "semantic.assess_patch"
            | "record.list"
            | "record.get"
            | "asset.list"
            | "asset.events"
            | "import.study.preview"
            | "calculator.dilution"
            | "calculator.solution"
            | "calculator.serial_dilution"
            | "calculator.reaction_volume"
            | "calculator.gel_mastermix"
            | "calculator.cell_seeding"
            | "calculator.runtime_normalize"
            | "calculator.sample_normalize_batch"
            | "calculator.in_vivo_dosing"
            | "calculator.biofilm_cfu"
            | "sdk.modules"
            | "sdk.describe"
            | "sdk.validate"
            | "asterdoc.inspect"
            | "asterdoc.assess"
            | "asterdoc.validate"
            | "asterdoc.compile"
            | "asterdoc.load"
    )
}
pub(crate) fn tool_description(tool: &str) -> &'static str {
    match tool {
        "project.list" => "列出项目，返回稳定 id 和版本字段。",
        "pantry.list" => "只读列出储藏室条目及其效期状态；可指定 as_of 固定计算日期。",
        "pantry.upsert" => "创建或整条覆盖一个储藏室条目（名称/位置/购入开封日/保质期/耗尽状态），返回短标签 ID 与效期状态。",
        "pantry.delete" => "停用并隐藏一个储藏室条目；短标签 ID 保留不被重新分配，条目不可复活。",
        "label.export" => "只读导出贴标/扫码插件使用的稳定短 ID 清单（label_code + 效期状态）；不打印、不写数据库。",
        "project.create" => "创建一个新项目。",
        "project.revise" => "按 expected_revision 修订项目名称或说明。",
        "design.list" => "按项目查询实验设计。",
        "design.get" => "读取实验设计及其当前版本。",
        "design.map" => "只读生成实验地图：组别、检测、时间点、样本关系、Protocol 关联和待确认项；不写数据库。",
        "design.create" => "创建实验设计并自动保存 v1 快照。",
        "design.revise" => {
            "只修改 changes 中提供的字段，保留其他值并新增版本；必须提供 expected_revision。"
        }
        "run.list" => "按项目查询实验运行。",
        "workflow.inspect_tree" => "只读投影 Design（可选 Run）的 Use/Definition/Occurrence 层级树；由 Rust Core 直接计算，不依赖桌面应用。",
        "workflow.progress" => "只读投影一个 Run 每个冻结步骤的样本矩阵：每个样本的当前状态、attempt 历史与最后事件时间；步骤完成只在全部样本完成时成立。",
        "run.create" => "从设计当前版本创建运行并固定设计/Protocol 版本引用；仅受信任的本地写入端可调用，Agent 不得自行创建 Run。",
        "run.set_status" => "更新运行状态；必须提供读取时的 expected_revision。Protocol 必填参数未确认时拒绝开始运行；已归档 Run 拒绝一切写入。",
        "run.revise" => "按 expected_revision 修订运行名称或计划日期；设计版本、Protocol 清单、材料清单和重复关系不可改写；已归档 Run 拒绝一切写入。",
        "record.projection" => "只读生成 Run 的 Record 投影：冻结计划、参数对比、计时器、图板矩阵、资产、读数、笔记引用与归档状态；不写数据库。",
        "record.sign_archive" => "按 summarizing→signed→archived 生成归档清单并持久化归档状态；清单绑定确切投影/笔记/资产校验和，失败不部分归档；已归档后一切写入拒绝。",
        "record.archive_state" => "只读读取 Run 的归档状态（recording/summarizing/signed/archived + 签核与清单）；重启后状态不变，不写数据库。",
        "task.set_status" => "更新任务状态；必须提供读取时的 expected_revision。",
        "protocol.list" => "按项目查询 Protocol。",
        "protocol.version.get" => "读取确切不可变 ProtocolVersion 并递归展开步骤、参数槽位；只读。",
        "protocol.version.list" => "列出一个 Protocol 的全部不可变版本及展开内容；只读。",
        "protocol.block.list" => "按项目列出可复用步骤块。",
        "protocol.block.get" => "读取步骤块及其全部不可变版本。",
        "protocol.block.create" => "创建可作为完整 Protocol 或子步骤复用的版本化步骤块；只记录明确提供的操作和参数槽位。",
        "component.list" => "列出组件定义及是否已实现;未实现定义只能作为 catalog-only 查看。",
        "labware.define" => "注册一个不可变 LabwareProfile 版本;Design 容器计划从持久化注册表解析。",
        "component.define" => "注册一个不可变的 ComponentDefinition 版本;View 创建/修订从持久化注册表加载。",
        "view.project" => "从真实 ProtocolPreview 生成只读 ViewDocument 初始投影;不写入。",
        "view.data_preview" => "从 ViewDocument 的 PlannedContainer 读取孔位，执行通用试剂总量计算并生成表格；不写入。",
        "view.create" => "在受控事务中保存新的统一组件 ViewDocument。",
        "view.get" => "读取 ViewDocument 元数据及全部不可变版本。",
        "view.revise" => "按 expected_revision 应用 ViewChangeSet，生成新的 ViewDocument 版本。",
        "semantic.inspect" => "读取一个步骤块版本的步骤、绑定与 module_contract；不写入。",
        "semantic.search" => "按标题/目的/适用范围搜索当前项目已导入且来源为 current 的 Catalog 模块；不写入。",
        "semantic.validate" => "校验 ChangeSet 结构、路径和受保护字段；不写入。",
        "semantic.propose_patch" => "返回精确目标与 expected version 的变更摘要；不写入。",
        "semantic.assess_patch" => "在只读上下文中模拟一个绑定变更，返回旧值、新值和受影响引用；不写入。",
        "semantic.apply_binding_patch" => "受信任地提交一个 BlockUse 绑定变更，生成新的不可变 BlockVersion；不改 Protocol/Run。",
        "catalog.review.record" => "保存一条审核记录；正式导入必须通过 review_id 引用它。",
        "catalog.operation.import" => "在受控事务中导入审核过的 OperationModule，分配本地 Block/Version ID，并保存 module_contract 与 ReviewRecord。",
        "protocol.block.revise" => "按 expected_version 创建步骤块新版本；旧版本和引用不变。",
        "protocol.block.preview" => "只读展开组合步骤，返回操作顺序、参数来源和未填写槽位；不写入。",
        "protocol.create" => "创建 Protocol；可引用一个确切根步骤块版本及本版参数绑定，也兼容自由文本正文。",
        "protocol.revise" => "追加 Protocol 新版本；参数值可改变而不复制步骤块；必须提供 expected_version。",
        "agent_edit.validate" => "只读校验 agent 提案：解析绑定、版本和单位，返回 Core 计算的 digest、无效引用与确认提示；不写数据库。",
        "agent_edit.commit" => "提交人工确认过的 agent 提案：单事务重新校验版本并追加一个新版本；过期或并发提案返回 VERSION_CONFLICT，不覆盖旧数据。",
        "step.edit" => "对步骤块树做 insert/reorder/reuse/bind 级编辑：无 confirmation 时只读校验并返回 plan 与 digest；带 confirmation 时单事务追加新版本。版本号由 Core 生成，冻结版本不改写。",
        "plate.create" => "为实验运行创建指定行列数的空布局。",
        "well.set" => "创建或覆盖一个板孔映射。",
        "note.create" => "向运行追加一条实际记录；事件时间必须显式提供或留空。",
        "execution_event.create" => "向运行追加结构化实际事件；事件时间不确定时留空，材料/测量必须填写明确单位或样本标识。",
        "readout.submit" => "提交一条读数并映射为一个 measurement 事件；同一 (run_id, step_path, endpoint, sample_id, timepoint) 重复提交返回首次记录。",
        "readout.batch" => "批量提交表格/孔板读数（cells 或 CSV），逐行校验并在单事务中写入一条事件；任一行失败则不写入。",
        "formula.create" => "保存公式表达式及变量单位说明，不执行公式代码。",
        "formula.revise" => "按 expected_revision 修订公式字段。",
        "table.create" => "创建列名固定、每行长度与列数一致的资料表。",
        "table.revise" => "按 expected_revision 替换资料表名称、列和全部行。",
        "record.list" => "按允许的 kind 和项目筛选记录，不返回停用的旧草稿。",
        "record.get" => "按 id 读取一条允许的记录，不返回停用的旧草稿。",
        "asset.link" => "关联本地文件到 Run 任务：只保存逻辑根、相对路径与实测 size/mtime/类型/sha256，不复制文件；同时追加 linked 事件。",
        "asset.link.update" => "把一条已有资产链接的坐标（well_id/timepoint/fov_index）就地改成新坐标；文件身份与 sha256 不变，旧坐标写进 updated 事件。",
        "asset.unlink" => "解除一条 asset_link 并追加 unlinked 事件；只删除引用，不删除原文件。",
        "asset.list" => "按 Run 查询当前有效的本地文件引用；只读。",
        "asset.events" => "按 Run 查询 linked/unlinked 事件历史；只读。",
        "import.study.preview" => "校验项目/设计/Protocol 导入结构并报告待补项，不写数据库。",
        "import.study.apply" => "在单一事务中导入项目、设计快照和 Protocol v1。",
        "calculator.dilution" => "按 C1V1=C2V2 换算储液体积并校验单位。",
        "calculator.solution" => "按 m=C×V×MW 计算理论质量。",
        "calculator.serial_dilution" => "按相同体积单位计算逐级稀释浓度。",
        "calculator.reaction_volume" => "求反应已分配体积与剩余体积，不判断配方科学适用性。",
        "calculator.gel_mastermix" => "按组别 % w/v、实际孔数、每孔体积与余量导出各组 GelMA 干粉质量、LAP 母液体积与溶剂体积。",
        "calculator.cell_seeding" => "按目标 cells/well、实际孔数、每孔悬液体积与余量导出细胞总数与因子母液加入量。",
        "calculator.runtime_normalize" => "按实际孔数与死体积重算运行期配平体积，返回需追加或未用完的体积。",
        "calculator.sample_normalize_batch" => "按统一目标质量与反应体积平衡多个样本的实测质量浓度，给出各样本体积、稀释液体积与不达标标记。",
        "calculator.in_vivo_dosing" => "按每只动物体重与 mg/kg 剂量换算单次注射量与制备量，并给出母液与稀释液体积。",
        "calculator.biofilm_cfu" => "按样品、总稀释度与涂布量分组，由菌落数推导 CFU/mL 与 CFU/sample 派生读数；窗外板只记排除原因，不估算。",
        "sdk.modules" => "从嵌入的 modules.json 返回 AsterDoc 模块摘要（kind/schema/title/category/capabilities/renderers/actions）；只读。",
        "sdk.describe" => "按 kind+schema 返回完整的 AsterDoc 模块 manifest；只读。",
        "sdk.validate" => "校验 AsterDoc 源码或已保存设计并尝试编译；不写入。",
        "asterdoc.inspect" => "读取设计的精确/当前 AsterDoc 修订，解析模块并返回 manifest 渲染器/动作契约；只读。",
        "asterdoc.validate" => "校验候选 AsterDoc 源码并尝试编译；不写入。",
        "asterdoc.assess" => "对当前或指定 AsterDoc revision 执行只读 readiness 检查，返回 errors 与 warnings。",
        "asterdoc.load" => "读取设计的最新 AsterDoc 修订；只读。",
        "asterdoc.save" => "保存新的不可变 AsterDoc 修订；expected_revision 用于并发保护，operation_id 可用于幂等重试。",
        "asterdoc.command" => "按 expected_revision 将单条语义命令应用到 AsterDoc，并保存新修订；可提供 operation_id 做幂等重试。",
        "asterdoc.compile" => "将精确 AsterDoc revision 编译为设计；只读。",
        "asterdoc.create_run" => "从指定 AsterDoc revision 创建并冻结运行。",
        _ => "未定义工具。",
    }
}
pub(crate) fn tool_effects(tool: &str) -> &'static str {
    match tool {
        "project.list"
        | "design.list"
        | "design.get"
        | "design.map"
        | "run.list"
        | "run.workspace"
        | "workflow.inspect_tree"
        | "workflow.progress"
        | "protocol.list"
        | "protocol.version.get"
        | "protocol.version.list"
        | "protocol.block.list"
        | "protocol.block.get"
        | "protocol.block.preview"
        | "record.list"
        | "record.get"
        | "record.projection"
        | "record.archive_state" => "只读；不修改数据库。",
        "pantry.upsert" => "创建或整条覆盖一行储藏室台账并递增 revision；不修改 Design、Protocol 或已冻结 Run。",
        "pantry.delete" => "把一行储藏室台账标记为已停用并递增 revision；行与短标签 ID 保留，读路径不再返回它。",
        "import.study.preview" => "只校验导入内容并返回警告；不写入数据库。",
        "calculator.dilution"
        | "calculator.solution"
        | "calculator.serial_dilution"
        | "calculator.reaction_volume"
        | "calculator.gel_mastermix"
        | "calculator.cell_seeding"
        | "calculator.runtime_normalize"
        | "calculator.sample_normalize_batch"
        | "calculator.in_vivo_dosing" => "只返回算术结果；不保存输入或结果。",
        "calculator.biofilm_cfu" => "只返回算术结果；不保存输入或结果。",
        "project.create" => "创建一个项目记录。",
        "project.revise" => "按 expected_revision 更新一个项目记录并递增 revision。",
        "design.create" => "创建实验设计并创建不可变的 v1 设计快照。",
        "design.revise" => "更新当前实验设计并创建新的不可变版本快照；不改写旧快照。",
        "run.create" => "创建一次运行并固定引用当时的设计版本和可选 Protocol 版本。",
        "run.set_status" | "task.set_status" => {
            "按 expected_revision 更新一个状态并递增 revision。"
        }
        "run.revise" => "按 expected_revision 只修订运行名称或计划日期；不可修改已冻结的科学设计和 Protocol/材料清单；已归档 Run 拒绝一切写入。",
        "record.sign_archive" => "生成归档清单并持久化 summarizing→signed→archived 状态；清单摘要校验失败不部分归档；已归档 Run 的一切写入拒绝。",
        "task.create" => "创建一条日历任务。",
        "protocol.create" => "创建 Protocol 记录并创建不可变的 v1 正文版本。",
        "protocol.revise" => "为 Protocol 追加一个不可变的新版本；不覆盖旧正文。",
        "agent_edit.validate" => "只读；不修改数据库。",
        "agent_edit.commit" => "在一个事务中追加一个不可变版本，并记录提案 ID、确认人和版本号；不修改旧版本或冻结 Run。",
        "step.edit" => "无 confirmation 时只读；带 confirmation 时在一个事务中为每个被改动的步骤块追加一个不可变新版本，子块先于父块，并记录 edit digest 与确认人；不修改旧版本或冻结 Run。",
        "protocol.block.create" => "创建步骤块元数据和不可变 v1；步骤块只引用确切的其他版本。",
        "view.create" | "view.revise" => "保存新的 ViewDocument 版本；不改 Protocol/Run。",
        "view.get" => "只读；不修改数据库。",
        "view.proposal.create" => "为外部 Agent 保存一条待审阅的 View 变更提案；不写 ViewDocument。",
        "view.proposal.list" => "读取当前 View 的提案收件箱；只读。",
        "view.proposal.accept" => "接受待审阅提案并提交其冻结的 ViewChangeSet；可能产生 revision conflict。",
        "view.proposal.reject" => "拒绝待审阅提案；不改 ViewDocument。",
        "semantic.inspect"
        | "semantic.search"
        | "semantic.validate"
        | "semantic.propose_patch"
        | "semantic.assess_patch" => "只读；不修改数据库或冻结 Run。",
        "catalog.review.record" => "保存一条审核证明；不创建 Block 或 Protocol。",
        "catalog.operation.import" => "创建一个本地导入的步骤块及其不可变版本；保留 Catalog 精确版本、module_contract 和审核记录。",
        "protocol.block.revise" => "为步骤块追加不可变版本；不会改写已保存的组合或运行。",
        "plate.create" => "为指定运行创建一个空铺板布局。",
        "well.set" => "新建孔位映射，或按 expected_revision 覆盖一个已有孔位。",
        "note.create" => "为运行追加一条实验记录；不修改既有记录。",
        "execution_event.create" => "为运行追加一条不可覆盖的结构化操作、材料使用、测量、偏差或观察事件。",
        "readout.submit" => "为运行追加一条 measurement 执行事件；相同幂等键返回已有记录，不重复写入。",
        "readout.batch" => "在同一事务中为运行追加一条 measurement 执行事件；任一行校验失败则整体回滚。",
        "asset.link" => "读取本地文件元数据并流式计算 sha256；创建或刷新一条 asset_link，并追加 linked 事件；不复制文件。",
        "asset.link.update" => "在一个事务中更新 asset_link 的坐标字段并追加 updated 事件；不改写文件身份、链接 ID 或 sha256。",
        "asset.unlink" => "删除一条 asset_link 引用并追加 unlinked 事件；不删除原文件。",
        "asset.list" | "asset.events" => "只读；不修改数据库。",
        "formula.create" => "创建一条公式记录。",
        "formula.revise" => "按 expected_revision 更新一条公式记录并递增 revision。",
        "table.create" => "创建一张资料表。",
        "table.revise" => "按 expected_revision 替换资料表的名称、列和全部数据行。",
        "import.study.apply" => {
            "在一个数据库事务中创建项目、设计及其 v1 快照、Protocol 及其 v1 版本；失败时整体回滚。"
        }
        "sdk.modules" | "sdk.describe" | "sdk.validate" | "asterdoc.inspect" | "asterdoc.assess" | "asterdoc.validate" | "asterdoc.compile" | "asterdoc.load" => "只读；不修改数据库。",
        "asterdoc.save" => "保存新的不可变 AsterDoc 修订。",
        "asterdoc.command" => "应用语义命令并保存新的不可变 AsterDoc 修订。",
        "asterdoc.create_run" => "创建运行并固定 AsterDoc 修订引用。",
        _ => "未定义副作用。",
    }
}

