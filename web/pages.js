(() => {
  "use strict";
  const $ = id => document.getElementById(id);
  const pageNames = ["workflow", "projects", "designs", "protocols", "runs", "records", "tools"];
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
    const picker = $("projectPicker");
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
    window.history.replaceState(null, "", url);
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
          <button class="btn" onclick="AsterPageCtx.setPage('designs')">设计关系</button>
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
    if (["designs", "protocols", "runs"].includes(name)) {
      const module = window.AsterPageModules?.[name];
      if (module) return module($("page-view"), ctx);
      $("page-view").innerHTML = header(({designs:"实验设计", protocols:"Protocol", runs:"实验运行"})[name], "") + empty("模块未加载", `页面模块 ${name} 尚未加载。`);
    }
  }
  async function setPage(next) {
    const name = pageNames.includes(next) ? next : "workflow";
    await loadState();
    updateProjectPicker();
    const labels = {workflow:"实验设计",projects:"项目",designs:"设计关系",protocols:"Protocol",runs:"实验运行",records:"实验记录",tools:"资料与工具"};
    $("pageCrumb").textContent = labels[name] || "工作区";
    const isWorkflow = name === "workflow";
    $("canvasPanel").hidden = !isWorkflow;
    $("detail").hidden = !isWorkflow;
    $("page-view").hidden = isWorkflow;
    const crumbs = $("crumbs"); if (crumbs) crumbs.hidden = !isWorkflow;
    document.querySelectorAll(".rail-item[data-page]").forEach(button => button.classList.toggle("active", button.dataset.page === name));
    const url = new URL(location.href);
    if (name === "workflow") url.searchParams.delete("page"); else url.searchParams.set("page", name);
    window.history.replaceState(null, "", url);
    if (isWorkflow) {
      if (window.bootLatestView && !$("viewId").value) await window.bootLatestView();
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
    window.history.replaceState(null, "", url);
  });
  document.querySelectorAll(".rail-item[data-page]").forEach(button => button.addEventListener("click", () => setPage(button.dataset.page)));
  setPage(new URLSearchParams(location.search).get("page") || "workflow");
})();
