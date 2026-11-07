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
  let selectedViewId = "";
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

  const stepModules = async (step, versionId, assayId) => {
    const candidates = ctx.records("view").filter(view => view.project_id === runs.find(item => item.id === runId)?.project_id);
    for (const view of candidates) {
      try {
        const document = await loadView(view.id);
        const component = findOperation(document.root, step.path, assayId);
        if (!component) continue;
        const children = Object.values(component.slots || {}).flat();
        const table = (title) => children.find(child => child.definition?.id === "content.table" && child.props?.title === title)?.props;
        return {component, materials: table("材料与试剂"), instruments: table("仪器与程序"), checks: table("检查与接受")};
      } catch (error) {
        // A view can be stale or unrelated; try the next one.
      }
    }
    return null;
  };

  async function render() {
    const run = runs.find(item => item.id === runId) || runs[0];
    root.innerHTML = `<div class="page-head"><div><h1>实验运行</h1><p>${esc(ctx.title(run))}</p></div><div class="page-tools"><select id="run-select" aria-label="选择实验运行">${runs.map(item => `<option value="${esc(item.id)}" ${item.id === runId ? "selected" : ""}>${esc(ctx.title(item))}</option>`).join("")}</select></div></div><div class="empty-state">正在读取冻结工作区…</div>`;
    try {
      const response = await fetch(`/api/runs/${encodeURIComponent(runId)}/workspace`);
      if (!response.ok) throw new Error(`HTTP ${response.status}`);
      const workspace = await response.json();
      const steps = workspace.steps || [];
      const completed = new Set((workspace.events || []).filter(event => event.step_status === "completed").map(event => event.protocol_step_path));
      const openStep = steps.find(item => !completed.has(item.step?.path)) || steps[steps.length - 1] || steps[0];
      if (!steps.some(item => item.view_id === selectedViewId)) selectedViewId = openStep?.view_id || "";
      const stepIndex = Math.max(0, steps.findIndex(item => item.view_id === selectedViewId));
      const current = steps[stepIndex];
      const step = current?.step;
      const scope = current?.scope && typeof current.scope === "object" ? current.scope : {};
      const assayId = scope.assay_id || current?.assay_id || step?.assay_id;
      const plannedOutputId = scope.planned_output_id || current?.planned_output_id || step?.planned_output_id;
      const scopeKind = scope.kind || (typeof current?.scope === "string" ? current.scope : "") || (plannedOutputId ? "material" : "assay");
      const scopeId = scopeKind === "material" ? plannedOutputId : assayId;
      const scopeMeta = (workspace.scopes || []).find(item => item.kind === scopeKind && item.id === scopeId);
      const versionId = current?.protocol_version_id || step?.protocol_version_id || scopeMeta?.protocol_version_id || workspace.protocol_version_id;
      const scopeLabel = scopeKind === "material" ? `材料 ${scopeId || "未指定"}` : `Assay ${scopeId || "未指定"}`;
      const events = (workspace.events || []).filter(event => event.protocol_step_path === step?.path && event.protocol_version_id === versionId && (scopeKind === "material" ? event.planned_output_id === scopeId : event.assay_id === scopeId));
      const allDone = Boolean(steps.length) && steps.every(item => completed.has(item.step?.path));
      const rowList = (items, renderItem, empty) => items?.length ? items.map(renderItem).join("") : `<div class="page-muted">${empty}</div>`;
      const detailRows = (items, keyLabel) => rowList(items, item => `<div class="page-row"><div><strong>${esc(item.label || item.name || item[keyLabel] || item.endpoint || item.role || "记录")}</strong><small>${esc([item.status, item.value != null && `${item.value}${item.unit ? ` ${item.unit}` : ""}`, item.note, item.method].filter(Boolean).join(" · ") || "未填写")}</small></div></div>`, "暂无记录。");
      const eventRows = rowList(events, event => `<article class="page-row"><div><strong>${esc(event.event_type || event.stage || "执行事件")}</strong><small>${esc([event.occurred_at, event.operator && `操作人：${event.operator}`, event.step_status && `状态：${event.step_status}`, event.timepoint && `时间点：${event.timepoint}`].filter(Boolean).join(" · ") || "时间/操作人未填写")}</small>${event.content ? `<small>${esc(event.content)}</small>` : ""}${event.readouts?.length ? `<small><b>读数</b></small>${detailRows(event.readouts, "readout_id")}` : ""}${event.measurements?.length ? `<small><b>测量</b></small>${detailRows(event.measurements, "endpoint")}` : ""}${event.checks?.length ? `<small><b>检查</b></small>${detailRows(event.checks, "check_id")}` : ""}</div></article>`, "当前步骤尚无实际事件。");
      const checkOptions = step?.checks || [];
      const checkFields = checkOptions.length ? `<label>检查项<select name="check_id" required>${checkOptions.map(check => `<option value="${esc(check.check_id)}">${esc(check.label || check.check_id)}</option>`).join("")}</select></label><label>检查状态<select name="check_status" required><option value="passed">通过</option><option value="failed">未通过</option><option value="not_applicable">不适用</option></select></label>` : '<div class="page-muted">此步骤没有冻结检查项，无法提交记录。</div>';
      const plates = (workspace.plates || []).filter(plate => scopeKind === "material" ? plate.planned_output_id === scopeId : plate.assay_id === scopeId).map(plate => `<section class="page-card"><h2>${esc(plate.name || plate.plate_id || "培养板")}</h2><div class="page-muted">${esc([plate.format, plate.rows && `${plate.rows} 行`, plate.columns && `${plate.columns} 列`, plate.timepoint].filter(Boolean).join(" · "))}</div>${plate.wells?.length ? `<div class="data-table-wrap"><table><thead><tr><th>孔位</th><th>组别</th><th>样本</th><th>备注</th></tr></thead><tbody>${plate.wells.map(well => `<tr><td>${esc(well.coordinate)}</td><td>${esc(well.group_label)}</td><td>${esc(well.sample_id)}</td><td>${esc(well.note)}</td></tr>`).join("")}</tbody></table></div>` : '<div class="page-muted">尚无孔位。</div>'}</section>`).join("");
      const chips = steps.length ? steps.map((item, index) => {
        const isDone = completed.has(item.step?.path);
        const isCurrent = item.view_id === selectedViewId;
        return `<button class="step-chip ${isCurrent ? "active" : ""} ${isDone ? "done" : ""}" data-step="${esc(item.view_id)}" aria-current="${isCurrent ? "step" : "false"}"><span>${index + 1}. ${esc(item.step?.title || item.step?.path || "步骤")}</span><small>${isDone ? "已完成" : isCurrent ? "当前" : "未开始"}</small></button>`;
      }).join("") : '<span class="page-muted">此 Run 没有冻结步骤</span>';
      const parameters = step?.parameters?.length ? step.parameters.map(parameter => `${parameter.label || parameter.slot || "参数"}: ${parameter.value ?? "未解析"}${parameter.unit ? ` ${parameter.unit}` : ""}`).join(" · ") : "无已解析参数";
      const checks = rowList(checkOptions, check => `<div class="page-row"><div><strong>${esc(check.label || check.check_id)}</strong><small>${esc(check.instruction || "设计时检查项")}${check.blocking ? " · 阻塞" : " · 提示"}</small></div></div>`, "此步骤未定义检查项。");
      const materials = (workspace.materials || []).filter(material => material.planned_output_id === plannedOutputId).map(material => `<div class="page-row"><div><strong>${esc(material.title || "共享材料")}</strong><small>${esc(material.consumer_assay_ids || [])}</small></div></div>`).join("") || '<div class="page-muted">当前范围没有冻结共享材料。</div>';
      const modules = await stepModules(step || {}, versionId, scopeKind === "assay" ? scopeId : null);
      const moduleTable = (title, table) => `<section class="page-card run-module"><h3>${esc(title)}</h3>${table?.rows?.length ? `<table class="run-module-table"><thead><tr>${table.columns.map(column => `<th>${esc(column)}</th>`).join("")}</tr></thead><tbody>${table.rows.map(row => `<tr>${row.map(cell => `<td>${esc(cell)}</td>`).join("")}</tr>`).join("")}</tbody></table>` : '<div class="page-muted">未绑定</div>'}</section>`;
      const nextStep = steps[stepIndex + 1];
      root.innerHTML = `<div class="page-head"><div><h1>实验运行</h1><p>${esc(ctx.title(run))} · ${esc(scopeLabel)} · 步骤 ${stepIndex + 1}/${steps.length} · ${esc(allDone ? "全部步骤已完成" : workspace.status || "状态未填写")}</p></div><div class="page-tools"><select id="run-select" aria-label="选择实验运行">${runs.map(item => `<option value="${esc(item.id)}" ${item.id === runId ? "selected" : ""}>${esc(ctx.title(item))}</option>`).join("")}</select></div></div>
        <div class="page-card run-step-strip"><div class="step-strip">${chips}</div></div>
        ${allDone ? '<div class="run-done"><strong>本次运行的全部步骤已记录。</strong><span>记录已保存在执行事件中；可回到本页补看每一步。</span></div>' : ''}
        <div class="run-cockpit">
          <section class="page-card run-current">
            <div class="run-kicker">当前步骤 · ${stepIndex + 1}/${steps.length}</div>
            <h2>${step ? esc(step.title || "当前步骤") : "当前步骤"}</h2>
            <p class="run-instruction">${esc(step?.instruction || "未填写")}</p>
            <div class="run-meta"><span>参数：${esc(parameters)}</span><span>时间范围：${esc(step?.time_scope || "未指定")}</span></div>
          </section>
          <div class="run-modules">
            ${moduleTable("材料与试剂", modules?.materials)}
            ${moduleTable("仪器与程序", modules?.instruments)}
            ${moduleTable("检查与接受", modules?.checks)}
          </div>
          <div class="run-grid">
            <section class="page-card"><h2>记录本步</h2>${step && checkOptions.length ? `<form id="actual-form" class="page-grid"><label>实际内容<textarea name="content" required placeholder="记录实际用量、观察或偏差"></textarea></label><label>步骤状态<select name="step_status"><option value="completed" selected>完成</option><option value="in_progress">进行中</option><option value="blocked">受阻</option></select></label><label>操作人<input name="operator"></label>${checkFields}<button class="btn primary" type="submit">保存并${nextStep ? "进入下一步" : "结束"}</button></form>` : checkFields}</section>
            <section class="page-card"><h2>本步记录</h2>${eventRows}</section>
          </div>
          ${plates}
          <details class="page-card"><summary>材料与技术信息</summary><h3>当前范围材料</h3>${materials}<h3>技术 ID</h3><div class="page-muted">view_id：${esc(current?.view_id || "未指定")} · Protocol 版本：${esc(versionId || "未指定")} · Protocol：${esc(current?.protocol_id || step?.protocol_id || "未指定")} · 路径：${esc(step?.path || "未指定")}</div></details>
        </div>`;
      root.querySelector("#run-select").addEventListener("change", event => { runId = event.target.value; selectedViewId = ""; render(); });
      root.querySelectorAll("[data-step]").forEach(button => button.addEventListener("click", () => { selectedViewId = button.dataset.step; render(); }));
      root.querySelector("#actual-form")?.addEventListener("submit", async event => {
        event.preventDefault();
        const form = new FormData(event.currentTarget);
        const status = form.get("step_status");
        const payload = {kind:"execution_event",parent_id:runId,project_id:run.project_id,data:{event_type:"operation",content:form.get("content").trim(),step_status:status,operator:form.get("operator").trim() || null,protocol_step_path:step.path,protocol_version_id:versionId,assay_id:scopeKind === "assay" ? scopeId : null,planned_output_id:scopeKind === "material" ? scopeId : null,checks:[{check_id:form.get("check_id"),status:form.get("check_status"),note:null,checked_by:form.get("operator").trim() || null}]}};
        const saved = await fetch("/api/records", {method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify(payload)});
        if (!saved.ok) throw new Error(`HTTP ${saved.status}`);
        if (status === "completed" && nextStep) selectedViewId = nextStep.view_id;
        await render();
      });
    } catch (error) {
      root.innerHTML = `<div class="page-head"><div><h1>实验运行</h1><p>${esc(ctx.title(run))}</p></div></div><div class="empty-state"><h2>无法读取或保存 Run 工作区</h2><p>${esc(error.message || error)}</p></div>`;
    }
  }
  await render();
};
