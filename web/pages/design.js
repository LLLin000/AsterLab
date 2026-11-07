window.AsterPageModules = window.AsterPageModules || {};
window.AsterPageModules.designs = async function(root, ctx) {
  const esc = ctx.esc;
  const fmt = value => value == null || value === "" ? "未填写" : Array.isArray(value) ? value.join(" · ") : String(value);
  const designs = ctx.records("design");
  if (!designs.length) {
    root.innerHTML = `<div class="page-head"><div><h1>实验设计</h1><p>这次实验做什么、分几组、何时测、采用什么方法。</p></div></div><div class="empty-state"><h2>还没有实验设计</h2><p>先建立设计，再把固定方法引用到 Protocol 版本。</p></div>`;
    return;
  }
  const query = new URLSearchParams(location.search);
  const design = designs.find(item => item.id === query.get("design")) || designs[0];
  const d = ctx.data(design);
  const assays = Array.isArray(d.assays) ? d.assays : [];
  const assay = assays.find(item => item.id === query.get("assay")) || assays[0] || null;
  const groups = Array.isArray(d.group_specs) ? d.group_specs : [];
  let preview = null;
  let previewError = "";
  const protocolVersionId = assay?.protocol_version_id;
  const protocolVersion = protocolVersionId ? ctx.one(protocolVersionId) : null;
  const vd = protocolVersion ? ctx.data(protocolVersion) : {};
  if (vd.root_block_version_id) {
    try {
      const response = await fetch("/api/protocol-blocks/preview", {
        method: "POST",
        headers: {"content-type": "application/json"},
        body: JSON.stringify({block_version_id: vd.root_block_version_id, bindings: vd.root_bindings || []})
      });
      const result = await response.json();
      if (!response.ok) throw new Error(result.error || `HTTP ${response.status}`);
      preview = result;
    } catch (error) { previewError = error.message || String(error); }
  }
  const steps = preview?.steps || [];
  const selectedIndex = Math.max(0, Math.min(Number(query.get("step") || 0), Math.max(steps.length - 1, 0)));
  const step = steps[selectedIndex];
  const unresolved = [];
  if (!d.experimental_unit) unresolved.push("实验单位");
  if (groups.some(group => group.independent_n == null)) unresolved.push("独立重复数");
  if (!assay) unresolved.push("检测安排");
  if (!protocolVersionId) unresolved.push("Protocol 版本");
  if (assay && !assay.timepoints?.length) unresolved.push("时间点");
  if (assay && !assay.endpoint) unresolved.push("检测终点");
  const protocolLabel = protocolVersion ? `${ctx.title(ctx.one(protocolVersion.parent_id))} · v${ctx.data(protocolVersion).version || protocolVersion.revision}` : "Protocol 未关联";
  const flow = steps.length ? steps.map((item, index) => `<button class="step-chip ${index === selectedIndex ? "active" : ""}" data-step="${index}"><span>${esc(item.title || `步骤 ${index + 1}`)}</span><small>${esc(item.use_id || "")}</small></button>`).join("") : '<span class="page-muted">尚未从 Protocol 展开步骤</span>';
  const groupRows = groups.length ? groups.map(group => `<div class="page-row"><div><strong>${esc(group.name || "未命名组")}</strong><small>${esc([group.role, group.condition].filter(Boolean).join(" · ") || "条件待确认")}</small></div><small>n=${esc(fmt(group.independent_n))} · 技术重复=${esc(fmt(group.technical_repeats))}</small></div>`).join("") : '<div class="page-muted">尚未定义组别。</div>';
  const assayRows = assays.length ? assays.map(item => `<div class="page-row"><div><strong>${esc(item.title || item.id)}</strong><small>${esc([item.endpoint, item.method, item.unit].filter(Boolean).join(" · ") || "终点/方法待确认")}</small><small>时间：${esc(fmt(item.timepoints))} · 对照：${esc(fmt(item.controls || item.control_group_ids))}</small></div><span class="page-muted">${esc(item.protocol_version_id ? "Protocol 已引用" : "Protocol 待关联")}</span></div>`).join("") : '<div class="page-muted">尚未定义检测。</div>';
  const keyRows = [
    ["研究问题", d.research_question],
    ["假设", d.hypothesis],
    ["组别", groups.map(group => group.name).filter(Boolean)],
    ["时间点", [...new Set(assays.flatMap(item => item.timepoints || []))]],
    ["独立重复", groups.map(group => `${group.name || "未填写"}: ${fmt(group.independent_n)}`)],
    ["检测", assays.map(item => item.title || item.id)],
    ["方法", protocolLabel],
    ["主要终点", assays.map(item => item.endpoint).filter(Boolean)]
  ].map(([label, value]) => `<div class="page-row"><div><strong>${esc(label)}</strong><small>${esc(fmt(value))}</small></div></div>`).join("");
  const stepSummary = step ? `<section class="page-card"><h2>当前步骤 · ${esc(step.title || "操作")}</h2><div class="page-row"><div><strong>采用方法</strong><small>${esc(protocolLabel)}</small></div></div><div class="page-row"><div><strong>操作摘要</strong><small>${esc(step.instruction || "未填写")}</small></div></div><div class="page-row"><div><strong>继承的检查</strong><small>${esc((step.checks || []).map(check => check.label || check.check_id).join(" · ") || "无")}</small></div></div><details><summary>查看完整参数与 Protocol</summary><div class="page-row"><div><small>${esc((step.parameters || []).map(parameter => `${parameter.label || parameter.slot}: ${parameter.value}${parameter.unit ? ` ${parameter.unit}` : ""}`).join(" · ") || "无本次覆盖参数")}</small></div></div></details></section>` : `<section class="page-card"><h2>当前步骤</h2><p class="page-muted">${esc(previewError || "此检测尚未展开 Protocol 步骤。")}</p></section>`;
  const selector = `<select id="design-select" aria-label="选择设计">${designs.map(item => `<option value="${esc(item.id)}" ${item.id === design.id ? "selected" : ""}>${esc(ctx.title(item))}</option>`).join("")}</select>`;
  const pvSelector = assays.length > 1 ? `<select id="assay-select" aria-label="选择检测">${assays.map(item => `<option value="${esc(item.id)}" ${assay?.id === item.id ? "selected" : ""}>${esc(item.title || item.id)}</option>`).join("")}</select>` : "";
  root.innerHTML = `<div class="page-head"><div><h1>${esc(ctx.title(design))}</h1><p>Design Draft · ${esc(d.objective || d.research_question || "研究目的待填写")}</p></div><div class="page-tools">${selector}${pvSelector}<a class="btn" href="?page=workflow">打开 Workflow</a></div></div>
    <div class="page-grid">
      <section class="page-card"><h2>实验安排</h2>${keyRows}</section>
      <section class="page-card"><h2>分组</h2>${groupRows}</section>
      <section class="page-card"><h2>检测</h2>${assayRows}</section>
      <section class="page-card"><h2>待确认</h2>${unresolved.length ? `<p class="page-token-list">${unresolved.map(item => `<span class="tag">${esc(item)}</span>`).join("")}</p>` : '<p class="page-muted">关键安排已填写；仍需按方法要求核对。</p>'}</section>
      <section class="page-card"><h2>实验流程</h2><div class="step-strip">${flow}</div>${stepSummary}</section>
      <details class="page-card"><summary>研究质量与来源</summary>${[["样本量依据", d.sample_size_rationale], ["随机化", d.randomization], ["盲法", d.blinding], ["分析计划", d.analysis_plan], ["来源与冲突", d.evidence_conflicts]].map(([label, value]) => `<div class="page-row"><div><strong>${esc(label)}</strong><small>${esc(fmt(value))}</small></div></div>`).join("")}</details>
    </div>`;
  root.querySelector("#design-select").addEventListener("change", event => {
    const url = new URL(location.href); url.searchParams.set("design", event.target.value); url.searchParams.delete("step"); window.history.replaceState(null, "", url); window.AsterPageModules.designs(root, ctx);
  });
  root.querySelector("#assay-select")?.addEventListener("change", event => {
    const url = new URL(location.href); url.searchParams.set("assay", event.target.value); window.history.replaceState(null, "", url); window.AsterPageModules.designs(root, ctx);
  });
  root.querySelectorAll(".step-chip").forEach(button => button.addEventListener("click", () => {
    const url = new URL(location.href); url.searchParams.set("step", button.dataset.step); window.history.replaceState(null, "", url); window.AsterPageModules.designs(root, ctx);
  }));
};
