window.AsterPageModules = window.AsterPageModules || {};
window.AsterPageModules.runs = async function(root, ctx) {
  const esc = ctx.esc;
  const runs = ctx.records("run");
  if (!runs.length) {
    root.innerHTML = '<div class="page-head"><div><h1>实验运行</h1><p>只显示本次执行的步骤和实际记录，不展示设计决策。</p></div></div><div class="empty-state"><h2>还没有实验运行</h2><p>从实验设计创建 Run；本页只承载执行。</p></div>';
    return;
  }
  let runId = new URLSearchParams(location.search).get("run") || runs[0].id;
  if (!runs.some(run => run.id === runId)) runId = runs[0].id;
  let selectedTaskKey = "";
  let selectedTimepoint = "";
  let selectedMode = "session";
  const viewCache = new Map();

  const allNodes = (node) => {
    if (!node) return [];
    const component = node.component || node;
    const own = component.definition ? [component] : [];
    const slots = Object.values(component.slots || {}).flat();
    const flowNodes = component.workflow?.nodes?.map(item => item.component) || [];
    return own.concat(slots.flatMap(child => allNodes({component: child})), flowNodes.flatMap(child => allNodes({component: child})));
  };

  const findOperation = (rootNode, path, assayId) => allNodes(rootNode).find(component => {
    const ref = component.model_ref || {};
    const step = ref.step || {};
    if (ref.kind !== "operation_use" || step.step_path !== path) return false;
    return !assayId || step.scope?.assay_id === assayId;
  });

  const loadView = async (viewId) => {
    if (viewCache.has(viewId)) return viewCache.get(viewId);
    const response = await fetch(`/api/views/${encodeURIComponent(viewId)}`);
    if (!response.ok) throw new Error(`HTTP ${response.status}`);
    const result = await response.json();
    const document = result.latest_version?.data || result.data || result;
    viewCache.set(viewId, document);
    return document;
  };

  const stepModules = async (step, assayId, scope) => {
    const run = runs.find(item => item.id === runId);
    const frozen = (ctx.data(run)?.execution_resources || []).find(item => item.step_path === step.path && sameScope(item.scope, scope));
    if (frozen) return {materials: frozen.tables?.materials, instruments: frozen.tables?.instruments, checks: frozen.tables?.checks};
    const candidates = ctx.records("view").filter(view => view.project_id === run?.project_id);
    for (const view of candidates) {
      try {
        const document = await loadView(view.id);
        const component = findOperation(document.root, step.path, assayId);
        if (!component) continue;
        const children = Object.values(component.slots || {}).flat();
        const table = (title) => children.find(child => child.definition?.id === "content.table" && child.props?.title === title)?.props;
        return {materials: table("材料与试剂"), instruments: table("仪器与程序"), checks: table("检查与接受")};
      } catch (error) {
        // A view can be stale or unrelated; try the next one.
      }
    }
    return null;
  };

  const sameScope = (a, b) => a?.kind === b?.kind && a?.assay_id === b?.assay_id && a?.planned_output_id === b?.planned_output_id;
  const runLabel = item => `${ctx.title(item)} · ${String(item.created_at || "").slice(5, 16).replace("T", " ")}`;
  const syncRunContext = item => {
    const projectPicker = document.getElementById("projectPicker");
    const designPicker = document.getElementById("designPicker");
    if (!projectPicker || !designPicker || !item) return;
    if (projectPicker.value === item.project_id) {
      if (item.parent_id) designPicker.value = item.parent_id;
      return;
    }
    projectPicker.value = item.project_id;
    const designs = ctx.records("design").filter(design => design.project_id === item.project_id);
    designPicker.innerHTML = designs.map(design => `<option value="${esc(design.id)}">${esc(ctx.title(design))}</option>`).join("");
    designPicker.disabled = !designs.length;
    designPicker.value = item.parent_id || designs[0]?.id || "";
    const url = new URL(location.href);
    url.searchParams.set("project", item.project_id);
    if (designPicker.value) url.searchParams.set("design", designPicker.value);
    window.history.replaceState(null, "", url);
  };

  async function render() {
    const run = runs.find(item => item.id === runId) || runs[0];
    syncRunContext(run);
    root.innerHTML = `<div class="page-head"><div><h1>实验运行</h1><p>${esc(ctx.title(run))}</p></div><div class="page-tools"><select id="run-select" aria-label="选择实验运行">${runs.map(item => `<option value="${esc(item.id)}" ${item.id === runId ? "selected" : ""}>${esc(runLabel(item))}</option>`).join("")}</select></div></div><div class="empty-state">正在读取冻结工作区…</div>`;
    try {
      const response = await fetch(`/api/runs/${encodeURIComponent(runId)}/workspace`);
      if (!response.ok) throw new Error(`HTTP ${response.status}`);
      const workspace = await response.json();
      const steps = workspace.steps || [];
      const tasks = workspace.tasks || [];
      const scopes = workspace.scopes || [];
      const taskVersion = task => {
        const step = steps.find(item => item.step?.path === task.step_path && sameScope(item.scope, task.scope));
        const scope = scopes.find(item => (item.kind === "assay" ? task.scope?.assay_id === item.id : task.scope?.planned_output_id === item.id));
        return step?.protocol_version_id || scope?.protocol_version_id || workspace.protocol_version_id;
      };
      const taskEvents = task => (workspace.events || []).filter(event => {
        if (event.task_key) return event.task_key === task.task_key;
        const scopeMatch = task.scope?.kind === "assay"
          ? event.assay_id === task.scope.assay_id
          : task.scope?.kind === "planned_output" ? event.planned_output_id === task.scope.planned_output_id : !event.assay_id && !event.planned_output_id;
        return event.protocol_step_path === task.step_path && scopeMatch && (event.timepoint || null) === (task.timepoint || null);
      });
      const taskDone = task => task.status === "completed";
      const completed = new Set(tasks.filter(taskDone).map(task => task.task_key));
      const allTasksDone = Boolean(tasks.length) && tasks.every(task => completed.has(task.task_key));
      const timepointValues = [...new Set(tasks.map(task => task.timepoint).filter(Boolean))];
      const generalTasks = tasks.filter(task => !task.timepoint);
      const sequence = [...generalTasks, ...timepointValues.flatMap(timepoint => tasks.filter(task => task.timepoint === timepoint))];
      const firstOpen = sequence.find(task => !completed.has(task.task_key));
      if (!timepointValues.includes(selectedTimepoint)) selectedTimepoint = firstOpen?.timepoint || timepointValues[0] || "";
      const dayTasks = timepointValues.length ? tasks.filter(task => task.timepoint === selectedTimepoint) : [];
      const visibleTasks = [...generalTasks, ...dayTasks];
      if (!visibleTasks.some(task => task.task_key === selectedTaskKey)) {
        const visibleOpen = visibleTasks.find(task => !completed.has(task.task_key));
        selectedTaskKey = (visibleOpen || (allTasksDone ? visibleTasks[visibleTasks.length - 1] : visibleTasks[0]))?.task_key || "";
      }
      const taskIndex = Math.max(0, visibleTasks.findIndex(task => task.task_key === selectedTaskKey));
      const currentTask = visibleTasks[taskIndex];
      const globalTaskIndex = sequence.findIndex(task => task.task_key === currentTask?.task_key);
      const step = steps.find(item => item.step?.path === currentTask?.step_path && sameScope(item.scope, currentTask?.scope))?.step;
      const scope = currentTask?.scope && typeof currentTask.scope === "object" ? currentTask.scope : {};
      const taskScope = scope;
      const assayId = scope.assay_id;
      const plannedOutputId = scope.planned_output_id;
      const scopeKind = scope.kind || (plannedOutputId ? "planned_output" : "assay");
      const scopeId = scopeKind === "planned_output" ? plannedOutputId : assayId;
      const scopeMeta = scopes.find(item => (item.kind === "assay" ? item.id === assayId : item.id === plannedOutputId));
      const versionId = taskVersion(currentTask || {});
      const scopeLabel = scopeKind === "planned_output" ? `材料 ${scopeId || "未指定"}` : `Assay ${scopeId || "未指定"}`;
      const events = currentTask ? taskEvents(currentTask) : [];
      const allDone = allTasksDone;
      const timepointDone = Boolean(dayTasks.length) && dayTasks.every(task => completed.has(task.task_key)) && generalTasks.every(task => completed.has(task.task_key));
      const rowList = (items, renderItem, empty) => items?.length ? items.map(renderItem).join("") : `<div class="page-muted">${empty}</div>`;
      const detailRows = (items, keyLabel) => rowList(items, item => `<div class="page-row"><div><strong>${esc(item.label || item.name || item[keyLabel] || item.endpoint || item.role || "记录")}</strong><small>${esc([item.status, item.value != null && `${item.value}${item.unit ? ` ${item.unit}` : ""}`, item.note, item.method].filter(Boolean).join(" · ") || "未填写")}</small></div></div>`, "暂无记录。");
      const eventRows = rowList(events, event => `<article class="page-row"><div><strong>${esc(event.event_type || event.stage || "执行事件")}</strong><small>${esc([event.occurred_at, event.operator && `操作人：${event.operator}`, event.step_status && `状态：${event.step_status}`, event.timepoint && `时间点：${event.timepoint}`].filter(Boolean).join(" · ") || "时间未填写")}</small>${event.content ? `<small>${esc(event.content)}</small>` : ""}${event.readouts?.length ? `<small><b>读数</b></small>${detailRows(event.readouts, "readout_id")}` : ""}${event.measurements?.length ? `<small><b>测量</b></small>${detailRows(event.measurements, "endpoint")}` : ""}${event.checks?.length ? `<small><b>检查</b></small>${detailRows(event.checks, "check_id")}` : ""}</div></article>`, "当前任务尚无实际事件。");
      const checkOptions = step?.checks || [];
      const checkFields = checkOptions.length ? `<div class="run-checks"><strong>检查项</strong>${checkOptions.map(check => `<label class="run-check"><span>${esc(check.label || check.check_id)}${check.blocking ? " · 必需" : ""}</span><select name="check_${esc(check.check_id)}" data-check="${esc(check.check_id)}"><option value="">未记录</option><option value="passed">通过</option><option value="failed">未通过</option><option value="not_applicable">不适用</option></select></label>`).join("")}</div>` : "";
      const plates = (workspace.plates || []).filter(plate => scopeKind === "planned_output" ? plate.planned_output_id === scopeId : plate.assay_id === scopeId).map(plate => `<section class="page-card"><h2>${esc(plate.name || plate.plate_id || "培养板")}</h2><div class="page-muted">${esc([plate.format, plate.rows && `${plate.rows} 行`, plate.columns && `${plate.columns} 列`, plate.timepoint].filter(Boolean).join(" · "))}</div>${plate.wells?.length ? `<div class="data-table-wrap"><table><thead><tr><th>孔位</th><th>组别</th><th>样本</th><th>备注</th></tr></thead><tbody>${plate.wells.map(well => `<tr><td>${esc(well.coordinate)}</td><td>${esc(well.group_label)}</td><td>${esc(well.sample_id)}</td><td>${esc(well.note)}</td></tr>`).join("")}</tbody></table></div>` : '<div class="page-muted">尚无孔位。</div>'}</section>`).join("");
      const chipFor = (task, index) => {
        const isDone = completed.has(task.task_key);
        const isCurrent = task.task_key === selectedTaskKey;
        const taskStep = steps.find(item => item.step?.path === task.step_path && sameScope(item.scope, task.scope))?.step;
        return `<button class="step-chip ${isCurrent ? "active" : ""} ${isDone ? "done" : ""}" data-task="${esc(task.task_key)}" aria-current="${isCurrent ? "step" : "false"}"><span>${index + 1}. ${esc(taskStep?.title || task.step_path)}${task.timepoint ? ` · ${esc(task.timepoint)}` : ""}</span><small>${isDone ? "已完成" : isCurrent ? "当前" : "未开始"}</small></button>`;
      };
      const generalChips = generalTasks.map((task, index) => chipFor(task, index)).join("");
      const dayChips = dayTasks.map((task, index) => chipFor(task, generalTasks.length + index)).join("");
      const chips = visibleTasks.length ? `${generalTasks.length ? `<div class="task-group"><div class="task-group-label">通用准备</div><div class="step-strip wrap">${generalChips}</div></div>` : ""}${dayTasks.length ? `<div class="task-group"><div class="task-group-label">${esc(selectedTimepoint)} 任务</div><div class="step-strip ${visibleTasks.length <= 6 ? "wrap" : ""}">${dayChips}</div></div>` : ""}` : '<span class="page-muted">此 Run 没有冻结任务</span>';
      const parameters = step?.parameters?.length ? step.parameters.map(parameter => `${parameter.label || parameter.slot || "参数"}: ${parameter.value ?? "未解析"}${parameter.unit ? ` ${parameter.unit}` : ""}`).join(" · ") : "无已解析参数";
      const timeScopeLabel = value => ({per_timepoint: "每个时间点", once: "一次", per_assay: "每个检测", unresolved: "待确认"})[value] || value || "未指定";
      const materials = (workspace.materials || []).filter(material => material.planned_output_id === plannedOutputId).map(material => `<div class="page-row"><div><strong>${esc(material.title || "共享材料")}</strong><small>${esc(material.consumer_assay_ids || [])}</small></div></div>`).join("") || '<div class="page-muted">当前范围没有冻结共享材料。</div>';
      const modules = await stepModules(step || {}, assayId, taskScope);
      const designAssay = (workspace.frozen_design?.content?.assays || []).find(assay => assay.id === assayId);
      const targetGroups = designAssay?.group_names?.join(" / ");
      const targetTimes = currentTask?.timepoint ? [currentTask.timepoint] : (designAssay?.timepoints || []);
      const targetLabel = [targetGroups, targetTimes.join(" / ")].filter(Boolean).join(" · ");
      const targetPrefix = currentTask?.target && currentTask.target.kind !== "unresolved" ? "目标样本" : "适用";
      const moduleTable = (title, table) => `<section class="page-card run-module"><h3>${esc(title)}</h3>${table?.rows?.length ? `<table class="run-module-table"><thead><tr>${table.columns.map(column => `<th>${esc(column)}</th>`).join("")}</tr></thead><tbody>${table.rows.map(row => `<tr>${row.map(cell => `<td>${esc(cell)}</td>`).join("")}</tr>`).join("")}</tbody></table>` : '<div class="page-muted">未绑定</div>'}</section>`;
      const frozenResources = ctx.data(run)?.execution_resources || [];
      const prepRows = kind => [...new Set(frozenResources.flatMap(item => (item.tables?.[kind]?.rows || []).map(row => row[0])).filter(Boolean))];
      const prepMaterials = prepRows("materials");
      const prepInstruments = prepRows("instruments");
      const prepPlates = [...new Set((workspace.plates || []).map(plate => plate.name || plate.plate_id).filter(Boolean))];
      const prepMarkup = `<details class="page-card run-prep"><summary>准备清单 · ${prepMaterials.length} 材料 · ${prepInstruments.length} 仪器</summary><div class="run-prep-grid"><div><strong>材料 / 试剂</strong><span>${esc(prepMaterials.join("、") || "当前 Run 未冻结材料")}</span></div><div><strong>仪器 / 程序</strong><span>${esc(prepInstruments.join("、") || "当前 Run 未冻结仪器")}</span></div><div><strong>孔板</strong><span>${esc(prepPlates.join("、") || "本次 Run 尚无实际孔板")}</span></div><div><strong>Protocol</strong><span>${esc(scopes.map(scope => scope.protocol_title || scope.title).filter(Boolean).join("、") || "未关联")}</span></div></div></details>`;
      const allEvents = (workspace.events || []).slice().reverse();
      const resultRows = allEvents.map(event => {
        const task = tasks.find(item => item.task_key === event.task_key);
        const taskStep = steps.find(item => item.step?.path === event.protocol_step_path)?.step;
        const title = taskStep?.title || event.protocol_step_path || "执行事件";
        const readouts = (event.readouts || []).length;
        const measurements = (event.measurements || []).length;
        const checks = (event.checks || []).length;
        const values = [...(event.measurements || []).map(item => `${item.value ?? item.qualifier ?? ""}${item.unit ? ` ${item.unit}` : ""}${item.sample_id ? ` · ${item.sample_id}` : ""}`), ...(event.readouts || []).map(item => `${item.value ?? item.qualifier ?? ""}${item.unit ? ` ${item.unit}` : ""}${item.sample_id ? ` · ${item.sample_id}` : ""}`)].filter(Boolean).join("; ");
        return `<tr><td>${esc(title)}</td><td>${esc(event.timepoint || task?.timepoint || "—")}</td><td>${esc(event.step_status || event.event_type || "—")}</td><td>${readouts + measurements}${values ? `<small><br>${esc(values)}</small>` : ""}</td><td>${checks}</td><td>${esc(event.content || "—")}</td></tr>`;
      }).join("");
      const completedWithoutResults = tasks.filter(task => completed.has(task.task_key) && !taskEvents(task).some(event => event.readouts?.length || event.measurements?.length)).length;
      const resultTaskOptions = tasks.map((task, index) => {
        const taskStep = steps.find(item => item.step?.path === task.step_path && sameScope(item.scope, task.scope))?.step;
        return `<option value="${esc(task.task_key)}" ${task.task_key === selectedTaskKey ? "selected" : ""}>${index + 1}. ${esc(taskStep?.title || task.step_path)}${task.timepoint ? ` · ${esc(task.timepoint)}` : ""}</option>`;
      }).join("");
      const parseResultRows = raw => {
        const rows = [], errors = [], labels = [];
        raw.split(/\n/).forEach((line, index) => {
          if (!line.trim()) return;
          const parts = line.split(/[,\t]/).map(part => part.trim());
          if (parts.length > 2) { errors.push(`第 ${index + 1} 行：最多两列（值, 样本标签）`); return; }
          if (!parts[0]) { errors.push(`第 ${index + 1} 行：数值为空`); return; }
          const value = Number(parts[0]);
          if (!Number.isFinite(value)) { errors.push(`第 ${index + 1} 行：不是有效数字「${parts[0]}」`); return; }
          if (parts[1]) labels.push(parts[1]);
          rows.push({value, sample_id: parts[1] || null});
        });
        return {rows, errors, labels};
      };
      const resultForm = `<form id="result-paste" class="run-form"><div class="run-confirm"><strong>批量录入读数</strong><span>每行一个值，可写“值, 样本编号”；单位和终点适用于整次粘贴。原始事件会保留。</span></div><label>任务<select name="task_key" required>${resultTaskOptions}</select></label><label>终点<input name="endpoint" required placeholder="例如 OD450 / Cq"></label><label>单位<input name="unit" placeholder="例如 AU"></label><label class="run-note">读数行（每行：值 或 值, 样本编号）<textarea name="rows" required placeholder="0.82, Control-1
0.79, Control-2"></textarea></label><div id="result-preview" class="result-preview"></div><div id="result-form-error" class="summary-error" role="alert"></div><button class="btn primary" type="submit">保存读数</button></form>`;
      const nextTask = visibleTasks[taskIndex + 1] || sequence[globalTaskIndex + 1];
      const confirmLabel = allDone ? "本次运行已完成" : nextTask ? "确认完成并进入下一步" : "确认完成";
      const form = allDone || timepointDone ? "" : `<form id="actual-form" class="run-form"><div class="run-confirm"><strong>${esc(step?.title || "当前步骤")}</strong><span>${checkOptions.some(check => check.blocking) ? "先完成必需检查，再确认本步。" : "按已选 Protocol 执行后确认即可。"} 确认只记录操作完成，不声称已测量参数或保存结果。</span></div>${checkFields}<label class="run-note">异常 / 偏差 / 实际参数（可选）<textarea name="content" placeholder="正常操作可留空；有偏差、遗漏或实际参数变化时再填写。"></textarea></label><details class="run-more"><summary>更多记录（操作人，可选）</summary><label>操作人<input name="operator" placeholder="可留空"></label></details><div id="run-form-error" class="summary-error" role="alert"></div><button class="btn primary" type="submit">${confirmLabel}</button></form>`;
      root.innerHTML = `<div class="page-head"><div><h1>实验运行</h1><p>${esc(ctx.title(run))} · ${selectedTimepoint ? `${esc(selectedTimepoint)} · ` : ""}任务 ${taskIndex + 1}/${visibleTasks.length} · ${esc(allDone ? "全部任务已完成" : workspace.status || "状态未填写")}</p></div><div class="page-tools"><select id="run-select" aria-label="选择实验运行">${runs.map(item => `<option value="${esc(item.id)}" ${item.id === runId ? "selected" : ""}>${esc(runLabel(item))}</option>`).join("")}</select></div></div>
        <div class="run-tabs"><button class="${selectedMode === "session" ? "active" : ""}" data-run-mode="session">执行安排</button><button class="${selectedMode === "results" ? "active" : ""}" data-run-mode="results">结果</button></div>
        ${prepMarkup}
        ${timepointValues.length > 1 ? `<div class="page-card run-timepoints"><div class="step-strip ${timepointValues.length <= 4 ? "wrap" : ""}">${timepointValues.map(timepoint => `<button class="step-chip ${timepoint === selectedTimepoint ? "active" : ""}" data-timepoint="${esc(timepoint)}">${esc(timepoint)}</button>`).join("")}</div></div>` : ''}
        <div class="page-card run-step-strip">${chips}</div>
        ${allDone ? '<div class="run-done"><strong>本次运行的全部任务已记录。</strong><span>可点击上方任务回看材料、检查与实际记录。</span></div>' : timepointDone ? '<div class="run-done"><strong>本时间点的任务已完成。</strong><span>切换到其他时间点继续。</span></div>' : ''}
        <div class="run-cockpit" ${selectedMode === "results" ? "hidden" : ""}>
          <section class="page-card run-current">
            <div class="run-kicker">当前任务 · ${taskIndex + 1}/${visibleTasks.length}${selectedTimepoint ? ` · ${esc(selectedTimepoint)}` : ""}</div>
            <h2>${step ? esc(step.title || "当前步骤") : "当前步骤"}</h2>
            <p class="run-instruction">${esc(step?.instruction || "未填写")}</p>
            <div class="run-meta"><span>参数：${esc(parameters)}</span><span>时间范围：${esc(timeScopeLabel(step?.time_scope))}</span>${targetLabel ? `<span>${targetPrefix}：${esc(targetLabel)}</span>` : ""}</div>
            ${nextTask ? `<div class="run-next">下一步：${esc(steps.find(item => item.step?.path === nextTask.step_path && sameScope(item.scope, nextTask.scope))?.step?.title || nextTask.step_path)}${nextTask.timepoint ? ` · ${esc(nextTask.timepoint)}` : ""}</div>` : ""}
          </section>
          <div class="run-modules">
            ${moduleTable("材料与试剂", modules?.materials)}
            ${moduleTable("仪器与程序", modules?.instruments)}
            ${moduleTable("检查与接受", modules?.checks)}
          </div>
          <div class="run-grid">
            <section class="page-card"><h2>${allDone ? "本次运行已完成" : timepointDone ? "本时间点已完成" : "确认 / 记录"}</h2>${allDone ? '<p class="page-muted">所有任务都有已完成记录；本页只回看，不再追加新的完成事件。</p>' : timepointDone ? '<p class="page-muted">本时间点没有未完成的任务，切换上方时间点继续。</p>' : form}</section>
            <section class="page-card"><h2>本任务记录</h2>${eventRows}</section>
          </div>
          ${plates}
          <details class="page-card"><summary>材料与技术信息</summary><h3>当前范围材料</h3>${materials}<h3>技术 ID</h3><div class="page-muted">task_key：${esc(currentTask?.task_key || "未指定")} · Protocol 版本：${esc(versionId || "未指定")} · 路径：${esc(step?.path || "未指定")}</div></details>
        </div>
        <section class="run-results" ${selectedMode === "session" ? "hidden" : ""}>
          <div class="page-card"><h2>结果与异常</h2><p class="page-muted">操作完成不等于结果完整。这里显示已记录的事件、读数和检查；${completedWithoutResults} 个已完成任务尚无读数。</p>
            ${allEvents.length ? `<div class="data-table-wrap"><table><thead><tr><th>任务</th><th>时间点</th><th>状态</th><th>读数/测量</th><th>检查</th><th>记录</th></tr></thead><tbody>${resultRows}</tbody></table></div>` : '<div class="page-muted">还没有执行事件。</div>'}
          </div>
          <div class="page-card"><h2>未记录读数的已完成任务</h2><p class="page-muted">这里只表示当前事件里没有读数或测量；是否必须记录由步骤的结果要求决定。</p>${tasks.filter(task => completed.has(task.task_key) && !taskEvents(task).some(event => event.readouts?.length || event.measurements?.length)).length ? tasks.filter(task => completed.has(task.task_key) && !taskEvents(task).some(event => event.readouts?.length || event.measurements?.length)).map(task => `<div class="page-row"><div><strong>${esc(steps.find(item => item.step?.path === task.step_path && sameScope(item.scope, task.scope))?.step?.title || task.step_path)}</strong><small>${esc(task.timepoint || "无时间点")} · 已完成操作但尚无读数/测量</small></div></div>`).join("") : '<div class="page-muted">所有已完成任务都已有读数/测量记录，或没有任务。</div>'}</div>
          <div class="page-card"><h2>批量录入读数</h2>${resultForm}</div>
        </section>`;
      root.querySelector("#run-select").addEventListener("change", event => { runId = event.target.value; selectedTaskKey = ""; render(); });
      root.querySelectorAll("[data-task]").forEach(button => button.addEventListener("click", () => { selectedTaskKey = button.dataset.task; render(); }));
      root.querySelectorAll("[data-timepoint]").forEach(button => button.addEventListener("click", () => { selectedTimepoint = button.dataset.timepoint; selectedTaskKey = ""; render(); }));
      root.querySelectorAll("[data-run-mode]").forEach(button => button.addEventListener("click", () => { selectedMode = button.dataset.runMode; render(); }));
      root.querySelector("#result-paste")?.addEventListener("submit", async event => {
        event.preventDefault();
        const form = new FormData(event.currentTarget);
        const task = tasks.find(item => item.task_key === form.get("task_key"));
        const taskStep = steps.find(item => item.step?.path === task?.step_path && sameScope(item.scope, task?.scope))?.step;
        const errorBox = root.querySelector("#result-form-error");
        if (!task || !taskStep) { if (errorBox) errorBox.textContent = "选择的任务无效"; return; }
        const endpoint = form.get("endpoint").trim();
        const unit = form.get("unit").trim() || null;
        const parsed = parseResultRows(form.get("rows"));
        if (parsed.errors.length) { if (errorBox) errorBox.textContent = parsed.errors.join("；"); return; }
        if (!parsed.rows.length) { if (errorBox) errorBox.textContent = "至少录入一行读数"; return; }
        const measurements = parsed.rows.map(row => ({endpoint, value: row.value, unit, sample_id: row.sample_id}));
        const scope = task.scope || {};
        const payload = {kind:"execution_event",parent_id:runId,project_id:run.project_id,data:{event_type:"measurement",content:"批量粘贴读数",protocol_step_path:taskStep.path,protocol_version_id:taskVersion(task),assay_id:scope.kind === "assay" ? scope.assay_id : null,planned_output_id:scope.kind === "planned_output" ? scope.planned_output_id : null,task_key:task.task_key,timepoint:task.timepoint || null,measurements}};
        const response = await fetch("/api/records", {method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify(payload)});
        if (!response.ok) {
          const body = await response.json().catch(() => ({}));
          if (errorBox) errorBox.textContent = body.error || `保存失败（HTTP ${response.status}）`;
          return;
        }
        await render();
      });
      const resultFormEl = root.querySelector("#result-paste");
      if (resultFormEl) {
        const preview = resultFormEl.querySelector("#result-preview");
        const save = resultFormEl.querySelector("button[type=submit]");
        const updatePreview = () => {
          const endpoint = resultFormEl.querySelector('[name="endpoint"]').value.trim();
          const unit = resultFormEl.querySelector('[name="unit"]').value.trim();
          const parsed = parseResultRows(resultFormEl.querySelector('[name="rows"]').value);
          const problems = [...parsed.errors];
          if (!endpoint) problems.push("终点不能为空");
          save.disabled = problems.length > 0 || parsed.rows.length === 0;
          preview.textContent = problems.length
            ? problems.join("；")
            : `${parsed.rows.length} 行 · 终点 ${endpoint}${unit ? ` · 单位 ${unit}` : ""}${parsed.labels.length ? " · 样本标签仅保留原始文本，未匹配本次 Run 样本" : ""}`;
          preview.classList.toggle("is-error", problems.length > 0);
        };
        resultFormEl.addEventListener("input", updatePreview);
        updatePreview();
      }
      const confirmButton = root.querySelector("#actual-form button[type=submit]");
      const syncConfirm = () => {
        if (!confirmButton) return;
        const missing = checkOptions.some(check => check.blocking && !root.querySelector(`[data-check="${check.check_id}"]`)?.value);
        confirmButton.disabled = missing;
        confirmButton.title = missing ? "先记录全部必需检查" : "";
      };
      root.querySelectorAll(".run-check select").forEach(select => select.addEventListener("change", syncConfirm));
      syncConfirm();
      root.querySelector("#actual-form")?.addEventListener("submit", async event => {
        event.preventDefault();
        const form = new FormData(event.currentTarget);
        const errorBox = root.querySelector("#run-form-error");
        const recorded = checkOptions.map(check => ({check, status: form.get(`check_${check.check_id}`)}));
        const missing = recorded.filter(item => item.check.blocking && !item.status);
        if (missing.length) {
          if (errorBox) errorBox.textContent = `请先记录必需检查：${missing.map(item => item.check.label || item.check.check_id).join("、")}`;
          return;
        }
        const failed = recorded.filter(item => item.check.blocking && item.status === "failed");
        const status = failed.length ? "blocked" : "completed";
        const payload = {kind:"execution_event",parent_id:runId,project_id:run.project_id,data:{event_type:"operation",content:form.get("content").trim() || "确认完成",step_status:status,operator:form.get("operator").trim() || null,protocol_step_path:step.path,protocol_version_id:versionId,assay_id:scopeKind === "assay" ? scopeId : null,planned_output_id:scopeKind === "planned_output" ? scopeId : null,task_key:currentTask.task_key,timepoint:currentTask.timepoint || null,checks:recorded.filter(item => item.status).map(item => ({check_id:item.check.check_id,status:item.status,note:null,checked_by:form.get("operator").trim() || null}))}};
        const saved = await fetch("/api/records", {method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify(payload)});
        if (!saved.ok) {
          const body = await saved.json().catch(() => ({}));
          if (errorBox) errorBox.textContent = body.error || `保存失败（HTTP ${saved.status}）`;
          return;
        }
        if (status === "completed" && nextTask) { selectedTaskKey = nextTask.task_key; selectedTimepoint = nextTask.timepoint || ""; }
        await render();
      });
    } catch (error) {
      root.innerHTML = `<div class="page-head"><div><h1>实验运行</h1><p>${esc(ctx.title(run))}</p></div></div><div class="empty-state"><h2>无法读取或保存 Run 工作区</h2><p>${esc(error.message || error)}</p></div>`;
    }
  }
  await render();
};
