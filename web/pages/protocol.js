window.AsterPageModules = window.AsterPageModules || {};
window.AsterPageModules.protocols = async function(root, ctx) {
  const protocols = ctx.records("protocol");
  const header = `<div class="page-head"><div><h1>Protocol</h1><p>方法步骤、参数、版本和来源。</p></div><div class="page-tools"><button class="btn" onclick="location.href='/workspace?view=protocols'">编辑 Protocol</button></div></div>`;
  if (!protocols.length) {
    root.innerHTML = header + `<div class="empty-state"><h2>还没有 Protocol</h2><p>在功能工作台创建 Protocol 与步骤块。</p></div>`;
    return;
  }

  const protocol = protocols[0];
  const protocolData = ctx.data(protocol);
  const versions = ctx.records("protocol_version")
    .filter(version => version.parent_id === protocol.id)
    .sort((a, b) => String(b.created_at || "").localeCompare(String(a.created_at || "")));
  const version = versions[0];
  const versionData = ctx.data(version);
  let preview = null;
  let previewError = "";
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
  const selectedIndex = Math.min(Number(root.dataset.protocolStep || 0), Math.max(steps.length - 1, 0));
  const step = steps[selectedIndex];
  const parameterRows = (step?.parameters || []).map(parameter => {
    const label = parameter.label || parameter.slot || parameter.key || "参数";
    const unit = parameter.unit ? ` ${parameter.unit}` : "";
    const source = parameter.value_source || parameter.source || "";
    return `<div class="page-row"><div><strong>${ctx.esc(label)}</strong><small>${ctx.esc(ctx.fmt(parameter.value))}${ctx.esc(unit)}${source ? ` · 来源：${ctx.esc(source)}` : ""}</small></div></div>`;
  }).join("");
  const unresolved = (step?.unresolved_parameter_refs || step?.unresolved_parameters || []).map(parameter => {
    const name = typeof parameter === "string" ? parameter : parameter.label || parameter.slot || parameter.key || "参数";
    const unit = typeof parameter === "object" && parameter.unit ? ` · ${parameter.unit}` : "";
    return `<div class="page-row"><div><strong>${ctx.esc(name)}</strong><small>尚未解析${ctx.esc(unit)}</small></div></div>`;
  }).join("");
  const checks = (step?.checks || []).map(check => `<div class="page-row"><div><strong>${ctx.esc(check.label || check.check_id || "检查项")}</strong><small>${ctx.esc(check.instruction || check.description || "")}${check.blocking ? " · 阻塞" : " · 提示"}</small></div></div>`).join("");
  const freeText = protocolData.instruction || protocolData.content || protocolData.text || protocolData.description ||
    versionData.instruction || versionData.content || versionData.text || versionData.description || "";
  const stepMarkup = step ? `
    <div class="page-card"><div class="step-strip">${steps.map((item, index) => `<button class="step-chip ${index === selectedIndex ? "active" : ""}" data-step="${index}"><span>${index + 1}. ${ctx.esc(item.title || `步骤 ${index + 1}`)}</span><small>${ctx.esc(item.use_id || item.path || "")}</small></button>`).join("")}</div></div>
    <div style="height:12px"></div>
    <div class="page-grid">
      <section class="page-card half"><h2>${ctx.esc(step.title || "步骤详情")}</h2><div class="page-row"><div><strong>指令</strong><small>${ctx.esc(step.instruction || "此步骤没有说明。")}</small></div></div>
        <h3>已解析参数</h3>${parameterRows || '<div class="page-muted">无已解析参数</div>'}
        <h3>未解析参数</h3>${unresolved || '<div class="page-muted">无未解析参数</div>'}
        <div class="page-row"><div><strong>时间范围</strong><small>${ctx.esc(step.time_scope || "未指定")}</small></div></div>
      </section>
      <section class="page-card half"><h2>检查与来源</h2>${checks || '<div class="page-muted">未定义检查项</div>'}
        <div class="page-row"><div><strong>来源</strong><small>${ctx.esc(step.source_block_version_id || "未提供")}</small><small>${ctx.esc(step.path || "未提供来源路径")}</small></div></div>
      </section>
      <section class="page-card"><h2>版本历史</h2>${versions.map(item => `<div class="page-row"><div><strong>${ctx.esc(item.id)}</strong><small>${ctx.esc(item.created_at || "")}${ctx.data(item).root_block_version_id ? ` · Block ${ctx.esc(ctx.data(item).root_block_version_id)}` : " · 自由文本"}</small></div></div>`).join("")}</section>
    </div>` : "";
  const freeTextMarkup = freeText ? `<div class="page-grid"><section class="page-card"><h2>自由文本 Protocol</h2><p style="white-space:pre-wrap">${ctx.esc(freeText)}</p>${versions.length ? `<h3>版本历史</h3>${versions.map(item => `<div class="page-row"><div><strong>${ctx.esc(item.id)}</strong><small>${ctx.esc(item.created_at || "")}</small></div></div>`).join("")}` : ""}</section></div>` : "";
  const errorMarkup = previewError ? `<div class="page-card" role="alert"><strong>步骤预览失败</strong><p>${ctx.esc(previewError)}</p></div>` : "";
  const noSteps = !step && !freeText && !previewError ? `<div class="page-card"><p class="page-muted">此 Protocol 版本没有可展示的步骤或自由文本内容。</p></div>` : "";
  root.innerHTML = header + `<div class="page-head"><div><h2>${ctx.esc(ctx.title(protocol))}</h2><p>${version ? `当前版本：${ctx.esc(version.id)}` : "尚无 Protocol 版本"}</p></div></div>` + errorMarkup + stepMarkup + freeTextMarkup + noSteps;
  root.querySelectorAll(".step-chip").forEach(button => button.onclick = () => {
    root.dataset.protocolStep = button.dataset.step;
    window.AsterPageModules.protocols(root, ctx);
  });
};