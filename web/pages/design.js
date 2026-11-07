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
  const missing = value => value == null || value === "" || Array.isArray(value) && !value.length ? "未填写" : fmt(value);
  const rows = items => items.map(([label, value]) => `<div class="page-row"><div><strong>${esc(label)}</strong><small>${esc(missing(value))}</small></div></div>`).join("");
  const groups = Array.isArray(d.group_specs) ? d.group_specs : [];
  const assays = Array.isArray(d.assays) ? d.assays : [];
  const allocations = Array.isArray(d.sample_allocations) ? d.sample_allocations : [];
  const nameFor = (id, fallback) => groups.find(group => group.id === id)?.name || assays.find(assay => assay.id === id)?.title || fallback || id;
  const groupNodes = groups.map(group => `<div class="map-node"><strong>${esc(missing(group.name))}</strong><small>${esc(missing(group.role))} · ${esc(missing(group.condition))}</small><small>n=${esc(missing(group.independent_n))} · 技术重复=${esc(missing(group.technical_repeats))}</small></div>`).join("") || `<div class="map-node"><strong>未填写</strong></div>`;
  const assayNodes = assays.map(assay => `<div class="map-node"><strong>${esc(ctx.title({data: assay, id: assay.id}) || assay.title || assay.id)}</strong><small>${esc([assay.endpoint, assay.method, assay.unit].filter(Boolean).map(esc).join(" · ") || "未填写")}</small></div>`).join("") || `<div class="map-node"><strong>未填写</strong></div>`;
  const times = Array.isArray(d.timepoints) ? d.timepoints : [];
  const timing = d.timepoint_timing;
  const timeNodes = [
    ...times.map(point => `<div class="map-node"><strong>${esc(missing(point))}</strong><small>${esc(missing(timing && typeof timing === "object" && !Array.isArray(timing) ? timing[point] : timing))}</small></div>`),
    ...assays.flatMap(assay => (Array.isArray(assay.timepoints) ? assay.timepoints : []).map(point => `<div class="map-node"><strong>${esc(missing(point))}</strong><small>${esc(assay.title || assay.id || "未填写")}${assay.timepoint_timing ? ` · ${esc(fmt(assay.timepoint_timing))}` : ""}</small></div>`)),
    ...allocations.map(link => `<div class="map-node"><strong>${esc(nameFor(link.group_id, link.group_name))}</strong><small>${esc(nameFor(link.assay_id, "未填写"))} · ${esc(missing(link.timepoint_relation))}</small><small>样本：${esc(missing(link.source_relation))} · ${esc(missing(link.physical_slots_per_group))}</small></div>`)
  ].join("") || `<div class="map-node"><strong>未填写</strong></div>`;
  const quality = [
    ["样本量依据", d.sample_size_rationale], ["随机化", d.randomization], ["盲法", d.blinding],
    ["纳入标准", d.inclusion_criteria], ["排除标准", d.exclusion_criteria],
    ["分析计划", d.analysis_plan], ["对照", d.controls]
  ];
  const conflicts = Array.isArray(d.evidence_conflicts) ? d.evidence_conflicts.filter(item => item.status !== "resolved") : [];
  const evidence = [
    ...(Array.isArray(d.evidence) ? d.evidence.map(item => `<div class="page-row"><div><strong>${esc(missing(item.title || item.id))}</strong><small>${esc(missing(item.summary || item.source || item.evidence_id))}</small></div></div>`) : []),
    ...allocations.filter(item => item.evidence_id).map(item => `<div class="page-row"><div><strong>样本分配证据</strong><small>${esc(item.evidence_id)}</small></div></div>`)
  ].join("");
  const conflictRows = conflicts.map(item => `<div class="page-row"><div><strong>${esc(missing(item.topic))}</strong><small>${esc(missing(item.affected_assay_ids))}</small></div></div>`).join("");
  const selector = `<select aria-label="选择设计">${designs.map(record => `<option value="${esc(record.id)}" ${record.id === design.id ? "selected" : ""}>${esc(ctx.title(record))}</option>`).join("")}</select>`;

  root.innerHTML = `<div class="page-head"><div><h1>${esc(ctx.title(design))}</h1><p>实验设计 · ${esc(missing(d.status || design.status))}</p></div><div class="page-tools">${selector}<a class="btn" href="?page=workflow">打开 Workflow</a><a class="btn primary" href="/workspace?view=designs">编辑设计</a></div></div>
    <section class="page-card"><h2>Design Map</h2><div class="page-map"><div class="page-col"><div class="page-col-title">Groups</div>${groupNodes}</div><div class="map-arrow">→</div><div class="page-col"><div class="page-col-title">Assays</div>${assayNodes}</div><div class="map-arrow">→</div><div class="page-col"><div class="page-col-title">Time / Sample</div>${timeNodes}</div></div></section>
    <div style="height:12px"></div><div class="page-grid">
      <section class="page-card half"><h2>Research</h2>${rows([["研究目标", d.objective], ["研究问题", d.research_question], ["假设", d.hypothesis]])}</section>
      <section class="page-card half"><h2>Groups &amp; unit</h2>${rows([["实验单位", d.experimental_unit], ["组别", groups.map(group => group.name).filter(Boolean)], ["n / 技术重复", groups.map(group => `${group.name || "未填写"}: n=${missing(group.independent_n)}, ${missing(group.technical_repeats)}`)], ["因素 / 水平", (Array.isArray(d.factors) ? d.factors : []).map(factor => `${factor.name || "未填写"}: ${(factor.levels || []).join(" / ") || "未填写"}`)]])}</section>
      <section class="page-card"><h2>Assays &amp; endpoints</h2>${assays.length ? assays.map(assay => `<div class="page-row"><div><strong>${esc(assay.title || assay.id || "未填写")}</strong><small>终点：${esc(missing(assay.endpoint))} · 方法：${esc(missing(assay.method))} · 单位：${esc(missing(assay.unit))}</small><small>时间：${esc(missing(assay.timepoints))} · Protocol：${esc(missing(assay.protocol_version_id))} · 对照组：${esc(missing(assay.control_group_ids))}</small></div></div>`).join("") : rows([["检测", null]])}</section>
      <section class="page-card"><h2>Study quality</h2>${rows(quality)}</section>
      <section class="page-card"><h2>Evidence / conflicts</h2><h3>Evidence</h3>${evidence || `<div class="page-row"><div><small>未填写</small></div></div>`}<h3>Conflicts</h3>${conflictRows || `<div class="page-row"><div><small>未填写</small></div></div>`}</section>
    </div>`;
  root.querySelector("select[aria-label='选择设计']").addEventListener("change", event => {
    const url = new URL(location.href);
    url.searchParams.set("design", event.target.value);
    window.history.replaceState(null, "", url);
    window.AsterPageModules.designs(root, ctx);
  });
};
