(() => {
  "use strict";
  const $ = id => document.getElementById(id);
  const pageNames = ["workflow", "projects", "protocols", "runs", "records", "tools"];
  let state;
  let stateRequest;
  const esc = value => String(value ?? "").replace(/[&<>"']/g, char => ({"&":"&amp;", "<":"&lt;", ">":"&gt;", "\"":"&quot;", "'":"&#39;"}[char]));
  const data = record => record?.data || {};
  const records = kind => (state?.records || []).filter(record => record.kind === kind);
  const one = id => (state?.records || []).find(record => record.id === id);
  const title = record => data(record).title || data(record).name || record?.id || "未命名";
  const fmt = value => value == null || value === "" ? "未填写" : Array.isArray(value) ? value.join(" · ") : String(value);
  async function loadState() {
    if (!stateRequest) stateRequest = fetch("/api/state").then(response => {
      if (!response.ok) throw new Error(`HTTP ${response.status}`);
      return response.json();
    }).then(value => (state = value));
    return stateRequest;
  }
  const ctx = {
    get state() { return state; }, records, data, one, title, esc, fmt, loadState,
    async reloadState() { state = null; stateRequest = null; await loadState(); },
    setPage,
    exportState() {
      const link = document.createElement("a");
      link.href = URL.createObjectURL(new Blob([JSON.stringify(state, null, 2)], {type: "application/json"}));
      link.download = `cellplanner-data-${new Date().toISOString().slice(0, 10)}.json`;
      link.click();
      URL.revokeObjectURL(link.href);
    },
    async backupData() {
      const response = await fetch("/api/backup", {method: "POST", headers: {"X-AsterLab-Human-Confirmed": "1"}});
      const result = await response.json();
      if (!response.ok) throw new Error(result.error || `HTTP ${response.status}`);
      $("status").textContent = `已备份 ${result.path || ""}`;
    }
  };
  window.AsterPageCtx = ctx;
  function header(heading, subtitle, tools = "") {
    return `<div class="page-head"><div><h1>${esc(heading)}</h1><p>${esc(subtitle)}</p></div><div class="page-tools">${tools}</div></div>`;
  }
  function empty(heading, text) {
    return `<div class="empty-state"><h2>${esc(heading)}</h2><p>${esc(text)}</p></div>`;
  }
  function updateProjectPicker() {
    const picker = $("projectPicker"), designsPicker = $("designPicker");
    if (!picker) return;
    const projects = records("project");
    picker.innerHTML = projects.length
      ? projects.map(project => `<option value="${esc(project.id)}">${esc(title(project))}</option>`).join("")
      : `<option value="">未选择项目</option>`;
    const url = new URL(location.href);
    const selected = projects.some(project => project.id === url.searchParams.get("project"))
      ? url.searchParams.get("project") : projects[0]?.id || "";
    picker.value = selected;
    if (selected) url.searchParams.set("project", selected);
    else url.searchParams.delete("project");
    if (designsPicker) {
      const designs = records("design").filter(record => record.project_id === selected);
      designsPicker.innerHTML = designs.length
        ? designs.map(design => `<option value="${esc(design.id)}">${esc(title(design))}</option>`).join("")
        : `<option value="">未选择设计</option>`;
      const designId = designs.some(design => design.id === url.searchParams.get("design"))
        ? url.searchParams.get("design") : designs[0]?.id || "";
      designsPicker.value = designId;
      designsPicker.disabled = !designId;
      if (designId) url.searchParams.set("design", designId);
      else url.searchParams.delete("design");
      renderWorkflowSummary(designs.find(design => design.id === designId));
    }
    window.history.replaceState(null, "", url);
  }
  function renderWorkflowSummary(design) {
    const box = $("workflowSummary");
    if (!box) return;
    if (!design) { box.hidden = true; box.innerHTML = ""; return; }
    const value = data(design), groups = Array.isArray(value.group_specs) ? value.group_specs : Array.isArray(value.groups) ? value.groups : typeof value.groups === "string" ? value.groups.split(/\n/).filter(Boolean) : [], assays = value.assays || [];
    const timepoints = [...new Set(assays.flatMap(assay => assay.timepoints || []))];
    const protocols = [...new Set(assays.flatMap(assay => [assay.protocol_version_id, ...(assay.protocol_version_ids || [])]).filter(Boolean))];
    const protocolTitle = id => {
      const record = one(id);
      const parent = record?.parent_id ? one(record.parent_id) : record;
      return parent ? title(parent) : id;
    };
    const pending = (value.evidence_conflicts || []).filter(item => !["resolved", "confirmed"].includes(item.status)).length +
      (value.unresolved_questions || value.pending_questions || []).length +
      assays.filter(assay => assay.timepoint_relation === "unknown" || assay.timepoint_relation === "unresolved").length;
    box.innerHTML = `<strong>${esc(title(design))}</strong><span>${groups.length} 组</span><span>${esc(fmt(timepoints) || "无时间点")}</span><span>${esc(fmt(assays.map(assay => [assay.title, assay.endpoint].filter(Boolean).join(" / ")).filter(Boolean)) || "无检测/终点")}</span><span>Protocol ${esc(fmt(protocols.map(protocolTitle)) || "未关联")}</span><span>${pending} 项待确认</span><button class="btn" id="editWorkflowDesign" type="button">编辑本次安排</button>`;
    box.hidden = false;
    box.querySelector("#editWorkflowDesign").onclick = () => openDesignEditor(design);
  }
  function openDesignEditor(design) {
    const existing = $("designInlineEdit"); if (existing) existing.remove();
    const value = JSON.parse(JSON.stringify(data(design)));
    const firstAssay = (value.assays || [])[0] || {};
    const groups = (value.group_specs || []).map((group, index) => `<div class="inline-edit-row"><strong>${esc(group.name || "未命名组")}</strong><label>条件<input data-edit-group="${index}" data-field="condition" value="${esc(group.condition || "")}"></label><label>独立 n<input data-edit-group="${index}" data-field="n" type="number" min="0" step="1" value="${group.independent_n ?? ""}"></label><label>技术重复<input data-edit-group="${index}" data-field="tech" type="number" min="0" step="1" value="${group.technical_repeats ?? ""}"></label></div>`).join("");
    const panel = document.createElement("section");
    panel.id = "designInlineEdit";
    panel.className = "design-inline-edit";
    panel.innerHTML = `<div class="inline-edit-head"><div><strong>编辑本次安排</strong><small>只修改 Design；引用的 Protocol 版本不变。</small></div><button class="btn" type="button" id="cancelDesignEdit">取消</button></div><div class="inline-edit-grid"><label>研究目的<textarea data-edit-field="objective">${esc(value.objective || "")}</textarea></label><label>研究问题<input data-edit-field="research_question" value="${esc(value.research_question || "")}"></label><label>当前检测终点<input data-edit-assay="endpoint" value="${esc(firstAssay.endpoint || "")}"></label><label>方法<input data-edit-assay="method" value="${esc(firstAssay.method || "")}"></label><label>单位<input data-edit-assay="unit" value="${esc(firstAssay.unit || "")}"></label><label>时间点（逗号分隔）<input data-edit-assay="timepoints" value="${esc((firstAssay.timepoints || []).join(", "))}"></label></div>${groups}<div class="inline-edit-actions"><span id="designEditStatus" class="page-muted"></span><button class="btn primary" type="button" id="saveDesignEdit">保存 DesignVersion</button></div>`;
    $("workflowSummary").insertAdjacentElement("afterend", panel);
    panel.querySelector("#cancelDesignEdit").onclick = () => panel.remove();
    panel.querySelector("#saveDesignEdit").onclick = async () => {
      const status = panel.querySelector("#designEditStatus"); status.textContent = "保存中…";
      panel.querySelectorAll("[data-edit-field]").forEach(input => { value[input.dataset.editField] = input.value.trim(); });
      panel.querySelectorAll("[data-edit-group]").forEach(input => {
        const group = value.group_specs[Number(input.dataset.editGroup)]; if (!group) return;
        if (input.dataset.field === "condition") group.condition = input.value.trim();
        if (input.dataset.field === "n") group.independent_n = input.value === "" ? null : Number(input.value);
        if (input.dataset.field === "tech") group.technical_repeats = input.value === "" ? null : Number(input.value);
      });
      if (value.assays?.[0]) {
        const assay = value.assays[0];
        panel.querySelectorAll("[data-edit-assay]").forEach(input => {
          if (input.dataset.editAssay === "timepoints") assay.timepoints = input.value.split(",").map(item => item.trim()).filter(Boolean);
          else assay[input.dataset.editAssay] = input.value.trim();
        });
      }
      try {
        const response = await fetch("/api/records/update", {method: "POST", headers: {"content-type": "application/json"}, body: JSON.stringify({id: design.id, expected_revision: design.revision, data: value})});
        const result = await response.json();
        if (!response.ok) throw new Error(result.error || `HTTP ${response.status}`);
        await ctx.reloadState();
        renderWorkflowSummary(ctx.one(design.id));
        panel.remove();
        $("status").textContent = "设计已保存，Protocol 版本未修改";
      } catch (error) { status.textContent = error.message || String(error); }
    };
  }
  function renderProjects() {
    const projects = records("project");
    $("page-view").innerHTML = header("项目", "项目是设计、运行和资料的容器。") + (projects.length
      ? `<div class="page-grid">${projects.map(project => {
        const designs = records("design").filter(record => record.project_id === project.id);
        const runs = records("run").filter(record => record.project_id === project.id);
        return `<article class="page-card half"><div class="page-row"><div><strong>${esc(title(project))}</strong><small>${esc(data(project).description || "暂无说明")}</small></div><span class="page-muted">${designs.length} 设计 · ${runs.length} 运行</span></div></article>`;
      }).join("")}</div>` : empty("还没有项目", "在功能工作台创建项目，或等待 Agent 生成草案。"));
  }
  function renderRecords() {
    const events = records("execution_event"), notes = records("note");
    $("page-view").innerHTML = header("实验记录", "实际事件、原始/派生结果和补充记录。") +
      `<div class="page-grid"><section class="page-card"><h2>执行事件</h2>${events.length
        ? `<div class="data-table-wrap"><table><thead><tr><th>运行</th><th>类型</th><th>内容</th><th>时间</th></tr></thead><tbody>${events.map(event => `<tr><td>${esc(title(one(event.parent_id)))}</td><td>${esc(data(event).event_type)}</td><td>${esc(data(event).content)}</td><td>${esc(data(event).occurred_at || "")}</td></tr>`).join("")}</tbody></table></div>`
        : `<div class="page-muted">尚无执行事件。</div>`}</section><section class="page-card"><h2>补充记录</h2>${notes.length
        ? notes.map(note => `<div class="page-row"><div><strong>${esc(data(note).content)}</strong><small>${esc(note.created_at || "")}</small></div></div>`).join("")
        : `<div class="page-muted">尚无补充记录。</div>`}</section></div>`;
  }
  function resourceRows(items, kind) {
    return items.length ? items.map(record => {
      const value = data(record);
      const summary = kind === "library_table"
        ? `${(value.columns || []).length} 列 · ${(value.rows || []).length} 行`
        : value.expression || value.description || value.notes || "打开查看详细资料";
      const detail = kind === "library_table"
        ? `<div class="data-table-wrap"><table><thead><tr>${(value.columns || []).map(column => `<th>${esc(column)}</th>`).join("")}</tr></thead><tbody>${(value.rows || []).map(row => `<tr>${row.map(cell => `<td>${esc(cell)}</td>`).join("")}</tr>`).join("")}</tbody></table></div>`
        : `<p>${esc(value.expression || value.description || value.notes || "暂无详细内容")}</p>${value.variables ? `<p class="page-muted">${esc(value.variables)}</p>` : ""}`;
      return `<details class="page-row"><summary><strong>${esc(title(record))}</strong><small>${esc(summary)}</small></summary><div class="page-resource-detail">${detail}<details><summary>技术 ID</summary><code>${esc(record.id)}</code></details></div></details>`;
    }).join("") : `<div class="page-muted">暂无资料。</div>`;
  }
  function renderTools() {
    const tables = records("library_table"), formulas = records("formula");
    const materials = records("material"), instruments = records("instrument");
    $("page-view").innerHTML = header("资料与工具", "快速进入实验操作，查阅可复用资料并管理项目数据。") +
      `<div class="page-grid">
        <section class="page-card"><h2>常用工具</h2><div class="page-tools">
          <button class="btn primary" onclick="AsterPageCtx.setPage('workflow')">实验设计</button>
          <button class="btn" onclick="AsterPageCtx.setPage('protocols')">Protocol</button>
          <button class="btn" onclick="AsterPageCtx.setPage('runs')">实验运行</button>
          <button class="btn" onclick="AsterPageCtx.setPage('records')">记录实验结果</button>
        </div><p class="page-muted">计算器随设计视图中的计算器组件使用。</p></section>
        <section class="page-card"><h2>资料与资源</h2>
          <h3>资料表</h3><div class="page-list">${resourceRows(tables, "library_table")}</div>
          <h3>公式</h3><div class="page-list">${resourceRows(formulas, "formula")}</div>
          <h3>材料</h3><div class="page-list">${resourceRows(materials, "material")}</div>
          <h3>仪器</h3><div class="page-list">${resourceRows(instruments, "instrument")}</div>
        </section>
        <section class="page-card"><h2>仪器与设备</h2><div class="page-list">${(window.AsterResources?.instruments || []).map(item => `<div class="page-row"><div><strong>${esc(item.title)}</strong><small>${esc(item.summary)}</small></div><span class="page-muted">通用资源</span></div>`).join("") || '<div class="page-muted">尚无仪器目录。</div>'}</div></section>
        <section class="page-card"><h2>常用工具</h2><div class="page-list">${(window.AsterResources?.tools || []).map(item => `<div class="page-row"><div><strong>${esc(item.title)}</strong><small>${esc(item.summary)}</small></div><button class="btn" onclick="location.href='/workspace?view=${esc(item.view || "calculators")}'">打开</button></div>`).join("") || '<div class="page-muted">尚无工具目录。</div>'}</div></section>
        <section class="page-card"><h2>材料与配方</h2><div class="page-list">${(window.AsterResources?.materials || []).map(item => `<div class="page-row"><div><strong>${esc(item.title)}</strong><small>${esc(item.summary)}</small></div><span class="page-muted">配方待绑定</span></div>`).join("") || '<div class="page-muted">尚无材料目录。</div>'}</div></section>
        <section class="page-card"><h2>数据与项目管理</h2><div class="page-tools">
          <button class="btn" onclick="AsterPageCtx.setPage('records')">查看实验记录</button>
          <button class="btn" onclick="AsterPageCtx.setPage('projects')">查看项目</button>
          <button class="btn" onclick="AsterPageCtx.exportState()">导出数据 JSON</button>
          <button class="btn" onclick="AsterPageCtx.backupData().catch(error => { document.getElementById('status').textContent = error.message; })">备份数据库</button>
        </div><p class="page-muted">项目切换位于顶部；导出包含当前本机状态。</p></section>
      </div>`;
  }
  async function renderPage(name) {
    await loadState();
    if (name === "projects") return renderProjects();
    if (name === "records") return renderRecords();
    if (name === "tools") return renderTools();
    if (["protocols", "runs"].includes(name)) {
      const module = window.AsterPageModules?.[name];
      if (module) return module($("page-view"), ctx);
      $("page-view").innerHTML = header(({protocols:"Protocol", runs:"实验运行"})[name], "") + empty("模块未加载", `页面模块 ${name} 尚未加载。`);
    }
  }
  async function setPage(next) {
    const name = pageNames.includes(next) ? next : "workflow";
    await loadState();
    updateProjectPicker();
    const labels = {workflow:"实验设计",projects:"项目",designs:"实验设计",protocols:"Protocol",runs:"实验运行",records:"实验记录",tools:"资料与工具"};
    $("pageCrumb").textContent = labels[name] || "工作区";
    const isWorkflow = name === "workflow";
    $("canvasPanel").hidden = !isWorkflow;
    $("detail").hidden = !isWorkflow;
    $("workflowSummary").hidden = !isWorkflow || !$("designPicker").value;
    $("page-view").hidden = isWorkflow;
    const crumbs = $("crumbs"); if (crumbs) crumbs.hidden = !isWorkflow;
    document.querySelectorAll(".rail-item[data-page]").forEach(button => button.classList.toggle("active", button.dataset.page === name));
    const url = new URL(location.href);
    if (name === "workflow") url.searchParams.delete("page"); else url.searchParams.set("page", name);
    window.history.replaceState(null, "", url);
    if (isWorkflow) {
      if (window.bootLatestView) await window.bootLatestView();
      else if (window.load) await window.load();
      return;
    }
    try { await renderPage(name); }
    catch (error) { $("page-view").innerHTML = empty("读取失败", error.message || String(error)); }
  }
  $("projectPicker").addEventListener("change", event => {
    const url = new URL(location.href);
    if (event.target.value) url.searchParams.set("project", event.target.value);
    else url.searchParams.delete("project");
    url.searchParams.delete("design");
    window.history.replaceState(null, "", url);
    updateProjectPicker();
    $("viewId").value = "";
    window.resetView();
    window.bootLatestView?.();
  });
  $("designPicker").addEventListener("change", event => {
    const url = new URL(location.href);
    if (event.target.value) url.searchParams.set("design", event.target.value);
    else url.searchParams.delete("design");
    window.history.replaceState(null, "", url);
    renderWorkflowSummary(records("design").find(design => design.id === event.target.value));
    $("viewId").value = "";
    window.resetView();
    window.bootLatestView?.();
  });
  document.querySelectorAll(".rail-item[data-page]").forEach(button => button.addEventListener("click", () => setPage(button.dataset.page)));
  setPage(new URLSearchParams(location.search).get("page") || "workflow");
})();
