window.AsterPageModules = window.AsterPageModules || {};
window.AsterPageModules.runs = async function(root, ctx) {
  const esc = ctx.esc;
  const val = value => value == null || value === "" ? "未填写" : Array.isArray(value) ? value.map(val).join(" · ") : typeof value === "object" ? JSON.stringify(value) : String(value);
  const link = '<a class="btn" href="/workspace?view=runs">打开执行台记录实际操作</a>';
  const runs = ctx.records("run");
  if (!runs.length) {
    root.innerHTML = `<div class="page-head"><div><h1>实验运行</h1><p>只显示本次执行的步骤和实际记录，不展示设计决策。</p></div>${link}</div><div class="empty-state"><h2>还没有实验运行</h2><p>从实验设计创建 Run；本页只承载执行。</p></div>`;
    return;
  }
  let runId = new URLSearchParams(location.search).get("run") || runs[0].id;
  if (!runs.some(run => run.id === runId)) runId = runs[0].id;
  let selectedPath = "";

  async function render() {
    const run = runs.find(item => item.id === runId) || runs[0];
    root.innerHTML = `<div class="page-head"><div><h1>实验运行</h1><p>${esc(ctx.title(run))}</p></div><div class="page-tools"><select id="run-select" aria-label="选择实验运行">${runs.map(item => `<option value="${esc(item.id)}" ${item.id === runId ? "selected" : ""}>${esc(ctx.title(item))}</option>`).join("")}</select>${link}</div></div><div class="empty-state">正在读取冻结工作区…</div>`;
    try {
      const response = await fetch(`/api/runs/${encodeURIComponent(runId)}/workspace`);
      if (!response.ok) throw new Error(`HTTP ${response.status}`);
      const workspace = await response.json();
      const steps = workspace.steps || [];
      if (!steps.some(item => item.step?.path === selectedPath)) selectedPath = steps[0]?.step?.path || "";
      const current = steps.find(item => item.step?.path === selectedPath);
      const step = current?.step;
      const events = (workspace.events || []).filter(event => !current || event.protocol_step_path === step.path);
      const stepChecks = (step?.checks || []).map(check => `<div class="page-row"><div><strong>${esc(check.label || check.check_id)}</strong><small>${esc(check.instruction || "设计时检查项")}${check.blocking ? " · 阻塞" : " · 提示"}</small></div></div>`).join("") || '<div class="page-muted">此步骤未定义检查项。</div>';
      const eventRows = events.length ? events.map(event => `<article class="page-row"><div><strong>${esc(event.event_type || event.stage || "执行事件")}</strong><small>${esc([event.occurred_at, event.operator && `操作人：${event.operator}`, event.step_status && `状态：${event.step_status}`, event.timepoint && `时间点：${event.timepoint}`].filter(Boolean).join(" · ") || "时间/操作人未填写")}</small>${event.content ? `<small>${esc(event.content)}</small>` : ""}${event.readouts?.length ? `<small><b>读数：</b>${esc(val(event.readouts))}</small>` : ""}${event.checks?.length ? `<small><b>检查：</b>${esc(val(event.checks))}</small>` : ""}${event.controls?.length ? `<small><b>对照：</b>${esc(val(event.controls))}</small>` : ""}${event.lineage?.length ? `<small><b>Lineage：</b>${esc(val(event.lineage))}</small>` : ""}${event.deviation ? `<small><b>偏差：</b>${esc(event.deviation)}</small>` : ""}</div></article>`).join("") : '<div class="page-muted">尚未记录本步骤的实际事件。</div>';
      const plates = workspace.plates?.length ? workspace.plates.map(plate => `<section class="page-card half"><h2>${esc(plate.name || plate.plate_id || "培养板")}</h2><div class="page-muted">${esc([plate.format, plate.rows && `${plate.rows} 行`, plate.columns && `${plate.columns} 列`, plate.assay_id && `Assay ${plate.assay_id}`, plate.timepoint].filter(Boolean).join(" · "))}</div>${plate.wells?.length ? `<div class="data-table-wrap"><table><thead><tr><th>孔位</th><th>组别</th><th>样本</th><th>备注</th></tr></thead><tbody>${plate.wells.map(well => `<tr><td>${esc(well.coordinate)}</td><td>${esc(well.group_label)}</td><td>${esc(well.sample_id)}</td><td>${esc(well.note)}</td></tr>`).join("")}</tbody></table></div>` : '<div class="page-muted">没有实际孔位。</div>'}</section>`).join("") : "";
      const materials = workspace.materials?.length ? workspace.materials.map(material => `<div class="page-row"><div><strong>${esc(material.title || material.planned_output_id || "共享材料")}</strong><small>${esc(material.planned_output_id || "")} · ${esc(material.consumer_assay_ids || [])}</small></div></div>`).join("") : '<div class="page-muted">本次执行没有冻结共享材料。</div>';
      const chips = steps.length ? steps.map((item, index) => `<button class="step-chip ${item.step.path === selectedPath ? "active" : ""}" data-step="${esc(item.step.path)}"><span>${index + 1}. ${esc(item.step.title || item.step.path || "步骤")}</span><small>${esc(item.step.path || "")}</small></button>`).join("") : '<span class="page-muted">此 Run 没有冻结步骤</span>';
      const parameters = step?.parameters?.length ? step.parameters.map(parameter => `${parameter.label || parameter.slot || "参数"}: ${parameter.value ?? "未解析"}${parameter.unit ? ` ${parameter.unit}` : ""}`).join(" · ") : "无已解析参数";
      root.innerHTML = `<div class="page-head"><div><h1>实验运行</h1><p>${esc(ctx.title(run))} · ${esc(workspace.status || "状态未填写")}</p></div><div class="page-tools"><select id="run-select" aria-label="选择实验运行">${runs.map(item => `<option value="${esc(item.id)}" ${item.id === runId ? "selected" : ""}>${esc(ctx.title(item))}</option>`).join("")}</select>${link}</div></div>
        <div class="page-card"><div class="step-strip">${chips}</div></div>
        <div class="page-grid" style="margin-top:12px">
          <section class="page-card half"><h2>${step ? esc(step.title || "当前步骤") : "当前步骤"}</h2>${step ? `<div class="page-row"><div><strong>指令</strong><small>${esc(step.instruction || "未填写")}</small></div></div><div class="page-row"><div><strong>参数</strong><small>${esc(parameters)}</small></div></div><div class="page-row"><div><strong>时间范围</strong><small>${esc(step.time_scope || "未指定")}</small></div></div>` : '<div class="page-muted">没有当前步骤。</div>'}</section>
          <section class="page-card half"><h2>检查项</h2>${stepChecks}</section>
          <section class="page-card"><h2>实际事件、读数与检查</h2>${eventRows}</section>
          ${plates}
          <section class="page-card"><h2>实际材料</h2>${materials}</section>
        </div>`;
      root.querySelector("#run-select").addEventListener("change", event => { runId = event.target.value; selectedPath = ""; render(); });
      root.querySelectorAll("[data-step]").forEach(button => button.addEventListener("click", () => { selectedPath = button.dataset.step; render(); }));
    } catch (error) {
      root.innerHTML = `<div class="page-head"><div><h1>实验运行</h1><p>${esc(ctx.title(run))}</p></div><div class="page-tools"><select id="run-select" aria-label="选择实验运行">${runs.map(item => `<option value="${esc(item.id)}" ${item.id === runId ? "selected" : ""}>${esc(ctx.title(item))}</option>`).join("")}</select>${link}</div></div><div class="empty-state"><h2>无法读取 Run 工作区</h2><p>${esc(error.message || error)}</p></div>`;
      root.querySelector("#run-select").addEventListener("change", event => { runId = event.target.value; selectedPath = ""; render(); });
    }
  }
  await render();
};
