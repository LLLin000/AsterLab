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
    openWorkspace(view) { location.href = `/workspace${view ? `?view=${encodeURIComponent(view)}` : ""}`; }
  };
  window.AsterPageCtx = ctx;
  function header(heading, subtitle, tools = "") {
    return `<div class="page-head"><div><h1>${esc(heading)}</h1><p>${esc(subtitle)}</p></div><div class="page-tools">${tools}</div></div>`;
  }
  function empty(heading, text) {
    return `<div class="empty-state"><h2>${esc(heading)}</h2><p>${esc(text)}</p></div>`;
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
  function renderTools() {
    const tables = records("library_table"), formulas = records("formula");
    $("page-view").innerHTML = header("工具与资料", "计算器、资料表和可复用的资料对象。") +
      `<div class="page-card" style="margin-bottom:12px"><h2>其他工作区</h2><div class="page-tools"><button class="btn" onclick="AsterPageCtx.setPage('projects')">项目</button><button class="btn" onclick="AsterPageCtx.setPage('designs')">设计关系</button><button class="btn" onclick="AsterPageCtx.setPage('records')">实验记录</button></div></div>` +
      `<div class="page-grid"><section class="page-card half"><h2>资料表</h2>${tables.length
        ? tables.map(table => `<div class="page-row"><div><strong>${esc(title(table))}</strong><small>${esc((data(table).columns || []).length)} 列 · ${esc((data(table).rows || []).length)} 行</small></div></div>`).join("")
        : `<div class="page-muted">尚无资料表。</div>`}</section><section class="page-card half"><h2>公式库</h2>${formulas.length
        ? formulas.map(formula => `<div class="page-row"><div><strong>${esc(title(formula))}</strong><small>${esc(data(formula).expression || "")}</small></div></div>`).join("")
        : `<div class="page-muted">尚无公式。</div>`}</section></div>`;
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
    const isWorkflow = name === "workflow";
    $("canvasPanel").hidden = !isWorkflow;
    $("detail").hidden = !isWorkflow;
    $("page-view").hidden = isWorkflow;
    const crumbs = $("crumbs"); if (crumbs) crumbs.hidden = !isWorkflow;
    if (!isWorkflow) {
      const labels = {projects:"项目",designs:"实验设计",protocols:"Protocol",runs:"实验运行",records:"实验记录",tools:"工具与资料"};
      $("workspaceTitle").textContent = `AsterLab / ${labels[name] || "工作区"}`;
      $("workspaceSub").textContent = "本机工作区 · 数据保存在 SQLite";
    }
    document.querySelectorAll(".rail-item[data-page]").forEach(button => button.classList.toggle("active", button.dataset.page === name));
    const url = new URL(location.href);
    if (name === "workflow") url.searchParams.delete("page"); else url.searchParams.set("page", name);
    window.history.replaceState(null, "", url);
    if (isWorkflow) { if (window.load) await window.load(); return; }
    try { await renderPage(name); }
    catch (error) { $("page-view").innerHTML = empty("读取失败", error.message || String(error)); }
  }
  document.querySelectorAll(".rail-item[data-page]").forEach(button => button.addEventListener("click", () => setPage(button.dataset.page)));
  setPage(new URLSearchParams(location.search).get("page") || "workflow");
})();
