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

  async function render() {
    const run = runs.find(item => item.id === runId) || runs[0];
    root.innerHTML = `<div class="page-head"><div><h1>实验运行</h1><p>${esc(ctx.title(run))}</p></div><div class="page-tools"><select id="run-select" aria-label="选择实验运行">${runs.map(item => `<option value="${esc(item.id)}" ${item.id === runId ? "selected" : ""}>${esc(ctx.title(item))}</option>`).join("")}</select></div></div><div class="empty-state">正在读取冻结工作区…</div>`;
    try {
      const response = await fetch(`/api/runs/${encodeURIComponent(runId)}/workspace`);
      if (!response.ok) throw new Error(`HTTP ${response.status}`);
      const workspace = await response.json();
      const steps = workspace.steps || [];
      if (!steps.some(item => item.view_id === selectedViewId)) selectedViewId = steps[0]?.view_id || "";
      const stepIndex = steps.findIndex(item => item.view_id === selectedViewId);
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
      const rowList = (items, renderItem, empty) => items?.length ? items.map(renderItem).join("") : `<div class="page-muted">${empty}</div>`;
      const detailRows = (items, keyLabel) => rowList(items, item => `<div class="page-row"><div><strong>${esc(item.label || item.name || item[keyLabel] || item.endpoint || item.role || "记录")}</strong><small>${esc([item.status, item.value != null && `${item.value}${item.unit ? ` ${item.unit}` : ""}`, item.note, item.method].filter(Boolean).join(" · ") || "未填写")}</small></div></div>`, "暂无记录。");
      const eventRows = rowList(events, event => `<article class="page-row"><div><strong>${esc(event.event_type || event.stage || "执行事件")}</strong><small>${esc([event.occurred_at, event.operator && `操作人：${event.operator}`, event.step_status && `状态：${event.step_status}`, event.timepoint && `时间点：${event.timepoint}`].filter(Boolean).join(" · ") || "时间/操作人未填写")}</small>${event.content ? `<small>${esc(event.content)}</small>` : ""}${event.readouts?.length ? `<small><b>读数</b></small>${detailRows(event.readouts, "readout_id")}` : ""}${event.measurements?.length ? `<small><b>测量</b></small>${detailRows(event.measurements, "endpoint")}` : ""}${event.checks?.length ? `<small><b>检查</b></small>${detailRows(event.checks, "check_id")}` : ""}${event.controls?.length ? `<small><b>对照</b></small>${detailRows(event.controls, "role")}` : ""}${event.lineage?.length ? `<small><b>Lineage</b></small>${detailRows(event.lineage, "relation")}` : ""}${event.deviation ? `<small><b>偏差：</b>${esc(event.deviation)}</small>` : ""}</div></article>`, "当前步骤尚无实际事件。");
      const checkOptions = step?.checks || [];
      const checkFields = checkOptions.length ? `<label>检查项<select name="check_id" required>${checkOptions.map(check => `<option value="${esc(check.check_id)}">${esc(check.label || check.check_id)}</option>`).join("")}</select></label><label>检查状态<select name="check_status" required><option value="passed">通过</option><option value="failed">未通过</option><option value="not_applicable">不适用</option></select></label>` : '<div class="page-muted">此步骤没有冻结检查项，无法提交记录。</div>';
      const plates = (workspace.plates || []).filter(plate => scopeKind === "material" ? plate.planned_output_id === scopeId : plate.assay_id === scopeId).map(plate => `<section class="page-card"><h2>${esc(plate.name || plate.plate_id || "培养板")}</h2><div class="page-muted">${esc([plate.format, plate.rows && `${plate.rows} 行`, plate.columns && `${plate.columns} 列`, plate.timepoint].filter(Boolean).join(" · "))}</div>${plate.wells?.length ? `<div class="data-table-wrap"><table><thead><tr><th>孔位</th><th>组别</th><th>样本</th><th>备注</th></tr></thead><tbody>${plate.wells.map(well => `<tr><td>${esc(well.coordinate)}</td><td>${esc(well.group_label)}</td><td>${esc(well.sample_id)}</td><td>${esc(well.note)}</td></tr>`).join("")}</tbody></table></div>` : '<div class="page-muted">没有实际孔位。</div>'}</section>`).join("");
      const chips = steps.length ? steps.map((item, index) => `<button class="step-chip ${item.view_id === selectedViewId ? "active" : ""}" data-step="${esc(item.view_id)}"><span>${index + 1}. ${esc(item.step?.title || item.step?.path || "步骤")}</span></button>`).join("") : '<span class="page-muted">此 Run 没有冻结步骤</span>';
      const parameters = step?.parameters?.length ? step.parameters.map(parameter => `${parameter.label || parameter.slot || "参数"}: ${parameter.value ?? "未解析"}${parameter.unit ? ` ${parameter.unit}` : ""}`).join(" · ") : "无已解析参数";
      const checks = rowList(checkOptions, check => `<div class="page-row"><div><strong>${esc(check.label || check.check_id)}</strong><small>${esc(check.instruction || "设计时检查项")}${check.blocking ? " · 阻塞" : " · 提示"}</small></div></div>`, "此步骤未定义检查项。");
      const materials = (workspace.materials || []).filter(material => material.planned_output_id === plannedOutputId).map(material => `<div class="page-row"><div><strong>${esc(material.title || "共享材料")}</strong><small>${esc(material.consumer_assay_ids || [])}</small></div></div>`).join("") || '<div class="page-muted">当前范围没有冻结共享材料。</div>';
      root.innerHTML = `<div class="page-head"><div><h1>实验运行</h1><p>${esc(ctx.title(run))} · ${esc(scopeLabel)} · 步骤 ${stepIndex + 1}/${steps.length} · ${esc(workspace.status || "状态未填写")}</p></div><div class="page-tools"><select id="run-select" aria-label="选择实验运行">${runs.map(item => `<option value="${esc(item.id)}" ${item.id === runId ? "selected" : ""}>${esc(ctx.title(item))}</option>`).join("")}</select></div></div>
        <div class="page-card"><div class="step-strip">${chips}</div></div>
        <div class="page-grid" style="margin-top:12px">
          <section class="page-card"><h2>${step ? esc(step.title || "当前步骤") : "当前步骤"}</h2>${step ? `<div class="page-row"><div><strong>指令</strong><small>${esc(step.instruction || "未填写")}</small></div></div><div class="page-row"><div><strong>参数</strong><small>${esc(parameters)}</small></div></div><div class="page-row"><div><strong>时间范围</strong><small>${esc(step.time_scope || "未指定")}</small></div></div>` : '<div class="page-muted">没有当前步骤。</div>'}</section>
          <section class="page-card"><h2>冻结检查项</h2>${checks}</section>
          <section class="page-card"><h2>记录实际</h2>${step && checkOptions.length ? `<form id="actual-form" class="page-grid"><label>实际内容<textarea name="content" required></textarea></label><label>步骤状态<select name="step_status"><option value="in_progress">进行中</option><option value="completed">完成</option><option value="blocked">受阻</option></select></label><label>操作人<input name="operator"></label>${checkFields}<button class="btn" type="submit">保存执行记录</button></form>` : checkFields}</section>
          <section class="page-card"><h2>实际事件与读数</h2>${eventRows}</section>${plates}
          <details class="page-card"><summary>材料与技术信息</summary><h3>当前范围材料</h3>${materials}<h3>技术 ID</h3><div class="page-muted">view_id：${esc(current?.view_id || "未指定")} · Protocol 版本：${esc(versionId || "未指定")} · Protocol：${esc(current?.protocol_id || step?.protocol_id || "未指定")} · 路径：${esc(step?.path || "未指定")}</div></details>
        </div>`;
      root.querySelector("#run-select").addEventListener("change", event => { runId = event.target.value; selectedViewId = ""; render(); });
      root.querySelectorAll("[data-step]").forEach(button => button.addEventListener("click", () => { selectedViewId = button.dataset.step; render(); }));
      root.querySelector("#actual-form")?.addEventListener("submit", async event => {
        event.preventDefault();
        const form = new FormData(event.currentTarget);
        const payload = {kind:"execution_event",parent_id:runId,project_id:run.project_id,data:{event_type:"operation",content:form.get("content").trim(),step_status:form.get("step_status"),operator:form.get("operator").trim() || null,protocol_step_path:step.path,protocol_version_id:versionId,assay_id:scopeKind === "assay" ? scopeId : null,planned_output_id:scopeKind === "material" ? scopeId : null,checks:[{check_id:form.get("check_id"),status:form.get("check_status"),note:null,checked_by:form.get("operator").trim() || null}]}};
        const saved = await fetch("/api/records", {method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify(payload)});
        if (!saved.ok) throw new Error(`HTTP ${saved.status}`);
        await render();
      });
    } catch (error) {
      root.innerHTML = `<div class="page-head"><div><h1>实验运行</h1><p>${esc(ctx.title(run))}</p></div></div><div class="empty-state"><h2>无法读取或保存 Run 工作区</h2><p>${esc(error.message || error)}</p></div>`;
    }
  }
  await render();
};
