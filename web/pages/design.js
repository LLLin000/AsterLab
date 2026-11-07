window.AsterPageModules = window.AsterPageModules || {};
window.AsterPageModules.designs = async function(root, ctx) {
  const designs = ctx.records("design");
  if (!designs.length) {
    root.innerHTML = `<div class="empty-state"><h2>还没有实验设计</h2><p><a class="btn primary" href="/workspace?view=designs">创建实验设计</a></p></div>`;
    return;
  }

  const selectedId = new URLSearchParams(location.search).get("design") || designs[0].id;
  const design = designs.find(record => record.id === selectedId) || designs[0];
  const d = ctx.data(design) || {};
  const esc = ctx.esc;
  const fmt = ctx.fmt;
  const groups = Array.isArray(d.group_specs) ? d.group_specs : [];
  const assays = Array.isArray(d.assays) ? d.assays : [];
  const allocations = Array.isArray(d.sample_allocations) ? d.sample_allocations : [];
  const missing = value => value == null || value === "" || Array.isArray(value) && !value.length ? "未填写" : fmt(value);
  const row = (label, value) => `<div class="page-row"><div><strong>${esc(label)}</strong><small>${esc(missing(value))}</small></div></div>`;
  const valueOf = (object, ...keys) => keys.map(key => object?.[key]).find(value => value != null && value !== "");
  const groupById = id => groups.find(group => group.id === id);
  const groupName = (id, fallback) => groupById(id)?.name || fallback || id || "未解析组别";
  const assayName = assay => ctx.title({data: assay, id: assay.id}) || assay.title || assay.id || "未命名检测";
  const selector = `<select aria-label="选择设计">${designs.map(record => `<option value="${esc(record.id)}" ${record.id === design.id ? "selected" : ""}>${esc(ctx.title(record))}</option>`).join("")}</select>`;
  const requestedSelection = root.dataset.designSelection;
  const validSelection = requestedSelection && (requestedSelection.startsWith("assay:") ?
    assays.some(assay => assay.id === requestedSelection.slice(6)) :
    groups.some(group => group.id === requestedSelection.slice(6)));
  const selection = validSelection ? requestedSelection : assays[0] ? `assay:${assays[0].id}` : groups[0] ? `group:${groups[0].id}` : "";
  root.dataset.designSelection = selection;

  const groupCards = groups.map(group => `<div class="map-node" role="button" tabindex="0" data-design-selection="group:${esc(group.id)}" aria-pressed="${selection === `group:${group.id}`}"><strong>${esc(missing(group.name))}</strong><small>${esc(missing(group.role))}${group.condition ? ` · ${esc(fmt(group.condition))}` : ""}</small><small>n=${esc(missing(group.independent_n))} · 技术重复=${esc(missing(group.technical_repeats))}</small></div>`).join("") || `<div class="page-muted">尚无组别</div>`;
  const assayCards = assays.map(assay => {
    const linkedIds = Array.isArray(assay.group_ids) ? assay.group_ids : [];
    const linkedNames = Array.isArray(assay.group_names) ? assay.group_names : [];
    const links = linkedIds.length ?
      linkedIds.map((id, index) => groupById(id) ? groupName(id, linkedNames[index]) : `未解析组别：${linkedNames[index] || id}`).join("、") :
      linkedNames.length ? linkedNames.join("、") : "未指定适用组别";
    const points = valueOf(assay, "timepoints", "timepoint");
    const timing = valueOf(assay, "timepoint_timing", "timing");
    const controls = valueOf(assay, "controls", "control_group_names", "control_group_ids");
    return `<div class="page-row" role="button" tabindex="0" data-design-selection="assay:${esc(assay.id)}" aria-pressed="${selection === `assay:${assay.id}`}"><div><strong>${esc(assayName(assay))}</strong><small>适用组别：${esc(links)}</small><small>终点：${esc(missing(assay.endpoint))} · 方法：${esc(missing(assay.method))} · 单位：${esc(missing(assay.unit))}</small><small>时间点：${esc(missing(points))} · 时间安排：${esc(missing(timing))}</small><small>对照：${esc(missing(controls))} · Protocol：${esc(missing(valueOf(assay, "protocol_version_id", "protocol_id")))}</small></div></div>`;
  }).join("") || `<div class="page-muted">尚无检测（Assay）</div>`;
  const allocationRows = allocations.map(item => {
    const group = groupById(item.group_id);
    const assay = assays.find(candidate => candidate.id === item.assay_id);
    const unresolved = !group || !assay;
    return `<div class="page-row"><div><strong>${esc(group ? groupName(item.group_id, item.group_name) : `未解析组别：${item.group_name || item.group_id || "未填写"}`)} → ${esc(assay ? assayName(assay) : `未解析检测：${item.assay_id || "未填写"}`)}</strong><small>时间关系：${esc(missing(item.timepoint_relation))} · 来源关系：${esc(missing(item.source_relation))} · 每组物理样本位：${esc(missing(item.physical_slots_per_group))}${unresolved ? " · 关系待解析" : ""}</small></div></div>`;
  }).join("");
  const selectedAssay = selection.startsWith("assay:") ? assays.find(assay => `assay:${assay.id}` === selection) : null;
  const selectedGroup = selection.startsWith("group:") ? groups.find(group => `group:${group.id}` === selection) : null;
  const detailRows = entries => entries.filter(([, value]) => value != null && value !== "" && (!Array.isArray(value) || value.length)).map(([label, value]) => row(label, value)).join("") || `<p class="page-muted">尚无已填写详情</p>`;
  const selectedMarkup = selectedAssay ? `<section class="page-card"><h2>${esc(assayName(selectedAssay))}</h2>${detailRows([["终点", selectedAssay.endpoint], ["方法 / 单位", [selectedAssay.method, selectedAssay.unit].filter(Boolean).join(" · ")], ["时间点", valueOf(selectedAssay, "timepoints", "timepoint")], ["时间安排", valueOf(selectedAssay, "timepoint_timing", "timing")], ["对照", valueOf(selectedAssay, "controls", "control_group_names", "control_group_ids")], ["Protocol", valueOf(selectedAssay, "protocol_version_id", "protocol_id")]])}</section>` :
    selectedGroup ? `<section class="page-card"><h2>${esc(missing(selectedGroup.name))}</h2>${detailRows([["角色", selectedGroup.role], ["条件", selectedGroup.condition], ["独立样本量 n", selectedGroup.independent_n], ["技术重复", selectedGroup.technical_repeats], ["描述", valueOf(selectedGroup, "description", "notes")]])}</section>` :
    `<section class="page-card"><h2>选择对象</h2><p class="page-muted">当前没有组别或检测可供查看。</p></section>`;

  const unanswered = [];
  if (!d.objective && !d.research_question) unanswered.push("研究目标/问题");
  if (!groups.length) unanswered.push("组别");
  groups.forEach(group => {
    if (group.independent_n == null || group.independent_n === "") unanswered.push(`${group.name || "组别"}样本量`);
  });
  if (!assays.length) unanswered.push("检测");
  assays.forEach(assay => {
    if (!assay.endpoint) unanswered.push(`${assayName(assay)}终点`);
    if (!(assay.group_ids?.length || assay.group_names?.length)) unanswered.push(`${assayName(assay)}适用组别`);
    if (!valueOf(assay, "protocol_version_id", "protocol_id")) unanswered.push(`${assayName(assay)} Protocol`);
  });
  const conflicts = Array.isArray(d.evidence_conflicts) ? d.evidence_conflicts.filter(item => item.status !== "resolved") : [];
  if (conflicts.length) unanswered.push(`${conflicts.length} 项证据冲突`);
  const quality = [
    ["样本量依据", d.sample_size_rationale], ["随机化", d.randomization], ["盲法", d.blinding],
    ["纳入标准", d.inclusion_criteria], ["排除标准", d.exclusion_criteria], ["分析计划", d.analysis_plan],
    ["研究假设", d.hypothesis], ["实验单位", d.experimental_unit], ["因素 / 水平", d.factors]
  ];
  const qualityRows = quality.filter(([, value]) => value != null && value !== "" && (!Array.isArray(value) || value.length)).map(([label, value]) => row(label, value)).join("") || `<p class="page-muted">尚无补充信息</p>`;
  const evidenceRows = [
    ...(Array.isArray(d.evidence) ? d.evidence.map(item => row(item.title || item.id || "证据", item.summary || item.source || item.evidence_id)) : []),
    ...conflicts.map(item => row(item.topic || "证据冲突", item.affected_assay_ids)),
    ...allocations.filter(item => item.evidence_id).map(item => row("样本分配证据", item.evidence_id))
  ].join("") || `<p class="page-muted">尚无证据记录</p>`;

  root.innerHTML = `<div class="page-head"><div><h1>${esc(ctx.title(design))}</h1><p>设计关系参考 · ${esc(missing(d.status || design.status))} · 只读</p></div><div class="page-tools">${selector}<a class="btn" href="?page=workflow">查看 Workflow</a></div></div>
    <section class="page-card"><h2>研究摘要</h2>${detailRows([["研究目标", d.objective], ["研究问题", d.research_question]])}</section>
    <div style="height:12px"></div><section class="page-card"><h2>设计关系概览</h2><div class="page-grid"><section class="page-card half"><h3>Groups · 选择查看</h3>${groupCards}</section><section class="page-card half"><h3>Assays · 适用组别明确列出</h3>${assayCards}</section></div><h3>样本分配关系</h3>${allocationRows || `<p class="page-muted">未记录样本分配关系</p>`}</section>
    <div style="height:12px"></div><div class="page-grid">${selectedMarkup}<section class="page-card"><h2>待确认</h2>${unanswered.length ? `<p>${esc(unanswered.join("、"))}</p>` : `<p class="page-muted">当前核心字段已填写</p>`}</section></div>
    <section class="page-card"><h2>可用仪器与工具</h2><div class="page-list">${(window.AsterResources?.instruments || []).slice(0,6).map(item => `<div class="page-row"><div><strong>${esc(item.title)}</strong><small>${esc(item.summary)}</small></div></div>`).join("")}${(window.AsterResources?.tools || []).slice(0,4).map(item => `<div class="page-row"><div><strong>${esc(item.title)}</strong><small>${esc(item.summary)}</small></div></div>`).join("")}</div><button class="btn" onclick="AsterPageCtx.setPage('tools')">打开资料与工具</button></section>
    <details class="page-card"><summary>Study Quality / 技术字段</summary>${qualityRows}</details>
    <details class="page-card"><summary>Evidence / conflicts</summary>${evidenceRows}</details>`;
  root.querySelector("select[aria-label='选择设计']").addEventListener("change", event => {
    const url = new URL(location.href);
    url.searchParams.set("design", event.target.value);
    window.history.replaceState(null, "", url);
    delete root.dataset.designSelection;
    window.AsterPageModules.designs(root, ctx);
  });
  root.querySelectorAll("[data-design-selection]").forEach(node => {
    const select = () => {
      root.dataset.designSelection = node.dataset.designSelection;
      window.AsterPageModules.designs(root, ctx);
    };
    node.addEventListener("click", select);
    node.addEventListener("keydown", event => {
      if (event.key === "Enter" || event.key === " ") { event.preventDefault(); select(); }
    });
  });
};
