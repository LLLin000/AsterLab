window.AsterPageModules = window.AsterPageModules || {};
window.AsterPageModules.protocols = async function(root, ctx) {
  const protocols = ctx.records("protocol");
  const query = new URLSearchParams(location.search);
  const selectedProtocol = protocols.find(item => item.id === query.get("protocol")) || protocols[0];
  const versions = ctx.records("protocol_version")
    .filter(item => item.parent_id === selectedProtocol?.id)
    .sort((a, b) => String(b.created_at || "").localeCompare(String(a.created_at || "")));
  const selectedVersion = versions.find(item => item.id === query.get("version")) || versions[0];
  const protocolData = ctx.data(selectedProtocol);
  const versionData = ctx.data(selectedVersion);
  const setSelection = (protocolId, versionId, stepIndex = 0) => {
    const url = new URL(location.href);
    url.searchParams.set("protocol", protocolId || "");
    if (versionId) url.searchParams.set("version", versionId); else url.searchParams.delete("version");
    url.searchParams.set("step", String(stepIndex));
    window.history.replaceState(null, "", url);
  };
  const header = `<div class="page-head"><div><h1>Protocol</h1><p>当前方法、执行动作与关键条件。</p></div><div class="page-tools"><a class="btn" href="/workspace?view=protocols">编辑 Protocol</a></div></div>`;
  if (!protocols.length) {
    root.innerHTML = header + `<div class="empty-state"><h2>还没有 Protocol</h2><p>在功能工作台创建 Protocol 与步骤块。</p></div>`;
    return;
  }

  let preview = null, previewError = "";
  if (versionData.root_block_version_id) {
    try {
      const response = await fetch("/api/protocol-blocks/preview", {
        method: "POST",
        headers: { "content-type": "application/json" },
        body: JSON.stringify({
          block_version_id: versionData.root_block_version_id,
          bindings: versionData.root_bindings || []
        })
      });
      const result = await response.json();
      if (!response.ok) throw new Error(result.error || `预览请求失败（${response.status}）`);
      if (result.error) throw new Error(result.error);
      preview = result;
    } catch (error) {
      previewError = error.message || String(error);
    }
  }

  const steps = preview?.steps || [];
  const requestedStep = Number.parseInt(query.get("step"), 10);
  const selectedIndex = Number.isInteger(requestedStep) && requestedStep >= 0
    ? Math.min(requestedStep, Math.max(steps.length - 1, 0)) : 0;
  const step = steps[selectedIndex];
  const itemValue = value => ctx.esc(ctx.fmt(value));
  const params = step?.parameters || [];
  const parameterRows = list => list.map(parameter => {
    const label = parameter.label || parameter.slot || parameter.key || "参数";
    const unit = parameter.unit ? ` ${parameter.unit}` : "";
    const provenance = parameter.provenance?.map(item => item.source || item.value_source || item.source_id).filter(Boolean).join(" · ") || parameter.value_source || parameter.source || "";
    return `<div class="page-row"><div><strong>${ctx.esc(label)}</strong><small>${itemValue(parameter.value)}${ctx.esc(unit)}${provenance ? ` · 来源：${ctx.esc(provenance)}` : ""}</small></div></div>`;
  }).join("");
  const categorized = (pattern) => params.filter(item => pattern.test(`${item.label || ""} ${item.slot || ""}`));
  const classified = new Set();
  const materials = categorized(/material|reagent|试剂|材料|耗材/i);
  const instruments = categorized(/instrument|equipment|device|仪器|设备/i);
  [...materials, ...instruments].forEach(item => classified.add(item));
  const keyParams = params.filter(item => !classified.has(item));
  const unresolvedItems = step?.unresolved_parameter_refs || step?.unresolved_parameters || [];
  const unresolved = unresolvedItems.map(parameter => {
    const name = typeof parameter === "string" ? parameter : parameter.label || parameter.slot || parameter.key || "参数";
    return `<div class="page-row"><div><strong>${ctx.esc(name)}</strong><small>尚未解析</small></div></div>`;
  }).join("");
  const checks = (step?.checks || []).map(check => `<div class="page-row"><div><strong>${ctx.esc(check.label || check.check_id || "检查项")}</strong><small>${ctx.esc(check.instruction || check.description || "")}${check.blocking ? " · 阻塞" : " · 提示"}</small></div></div>`).join("");
  const freeText = protocolData.instruction || protocolData.content || protocolData.text || protocolData.description ||
    versionData.instruction || versionData.content || versionData.text || versionData.description || "";
  const unresolvedDetails = unresolvedItems.length ? `<details class="page-card"><summary>${unresolvedItems.length} 项待确认</summary>${unresolved}</details>` : "";
  const stepMarkup = step ? `
    <section class="page-card"><div class="page-row"><div><strong>步骤</strong><small>${selectedIndex + 1} / ${steps.length}</small></div></div>
      <div class="step-strip" style="display:flex;flex-wrap:wrap;gap:8px">${steps.map((item, index) => `<button class="step-chip ${index === selectedIndex ? "active" : ""}" data-step="${index}">${index + 1}. ${ctx.esc(item.title || `步骤 ${index + 1}`)}</button>`).join("")}</div>
    </section>
    <section class="page-card"><h2>${ctx.esc(step.title || "当前动作")}</h2><div class="page-row"><div><strong>操作指令</strong><small>${ctx.esc(step.instruction || "此步骤没有说明。")}</small></div></div>
      ${step.time_scope ? `<div class="page-row"><div><strong>执行范围</strong><small>${ctx.esc(step.time_scope)}</small></div></div>` : ""}
    </section>
    <div class="page-grid">
      <section class="page-card half"><h3>关键参数与条件</h3>${parameterRows(keyParams) || '<div class="page-muted">暂无参数</div>'}</section>
      <section class="page-card half"><h3>材料与试剂</h3>${parameterRows(materials) || '<div class="page-muted">未列出</div>'}</section>
      <section class="page-card half"><h3>仪器与设备</h3>${parameterRows(instruments) || '<div class="page-muted">未列出</div>'}</section>
      <section class="page-card half"><h3>检查项</h3>${checks || '<div class="page-muted">未定义检查项</div>'}</section>
    </div>${unresolvedDetails}
    <details class="page-card"><summary>版本历史与来源块</summary>
      <div class="page-row"><div><strong>当前来源块</strong><small>${ctx.esc((preview?.block_version_ids || [step.source_block_version_id]).filter(Boolean).join(" · ") || "未提供")}</small><small>${ctx.esc(step.path || "未提供来源路径")}</small></div></div>
      ${versions.map(item => `<div class="page-row"><div><strong>${ctx.esc(item.id)}</strong><small>${ctx.esc(item.created_at || "")}${ctx.data(item).root_block_version_id ? ` · Block ${ctx.esc(ctx.data(item).root_block_version_id)}` : " · 自由文本"}</small></div></div>`).join("")}
    </details>` : "";
  const freeTextMarkup = !step && freeText ? `<section class="page-card"><h2>Protocol 指令</h2><p style="white-space:pre-wrap">${ctx.esc(freeText)}</p>${versions.length ? `<details><summary>版本历史</summary>${versions.map(item => `<div class="page-row"><div><strong>${ctx.esc(item.id)}</strong><small>${ctx.esc(item.created_at || "")}</small></div></div>`).join("")}</details>` : ""}</section>` : "";
  const errorMarkup = previewError ? `<div class="page-card" role="alert"><strong>步骤预览失败</strong><p>${ctx.esc(previewError)}</p></div>` : "";
  const noSteps = !step && !freeText && !previewError ? `<div class="page-card"><p class="page-muted">此 Protocol 版本没有可展示的步骤或自由文本内容。</p></div>` : "";
  root.innerHTML = header + `<section class="page-card">
    <label class="page-row"><span><strong>当前 Protocol</strong><small>${ctx.esc(ctx.title(selectedProtocol))}</small></span><select id="protocol-select" aria-label="选择 Protocol">${protocols.map(item => `<option value="${ctx.esc(item.id)}" ${item.id === selectedProtocol.id ? "selected" : ""}>${ctx.esc(ctx.title(item))}</option>`).join("")}</select></label>
    <label class="page-row"><span><strong>当前版本</strong><small>${selectedVersion ? ctx.esc(selectedVersion.id) : "尚无版本"}</small></span><select id="version-select" aria-label="选择 Protocol 版本">${versions.map(item => `<option value="${ctx.esc(item.id)}" ${item.id === selectedVersion?.id ? "selected" : ""}>v${ctx.esc(ctx.data(item).version || item.revision || "?")}${item.created_at ? ` · ${ctx.esc(item.created_at)}` : ""}</option>`).join("")}</select></label>
  </section>` + errorMarkup + stepMarkup + freeTextMarkup + noSteps;
  root.querySelector("#protocol-select").onchange = event => {
    const nextProtocol = protocols.find(item => item.id === event.target.value);
    const nextVersion = ctx.records("protocol_version").filter(item => item.parent_id === nextProtocol?.id)
      .sort((a, b) => String(b.created_at || "").localeCompare(String(a.created_at || "")))[0];
    setSelection(nextProtocol?.id, nextVersion?.id);
    window.AsterPageModules.protocols(root, ctx);
  };
  root.querySelector("#version-select").onchange = event => {
    setSelection(selectedProtocol.id, event.target.value);
    window.AsterPageModules.protocols(root, ctx);
  };
  root.querySelectorAll(".step-chip").forEach(button => button.onclick = () => {
    setSelection(selectedProtocol.id, selectedVersion?.id, Number(button.dataset.step));
    window.AsterPageModules.protocols(root, ctx);
  });
};