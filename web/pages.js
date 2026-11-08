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
    const protocolTitle = id => {
      const record = one(id);
      const parent = record?.parent_id ? one(record.parent_id) : record;
      return parent ? title(parent) : id;
    };
    const relationLabel = relation => ({
      same_experimental_unit_repeated: "同一实验单位重复",
      same_physical_sample_repeated: "同一实体样本重复",
      independent_sample_per_timepoint: "每时间点独立样本",
      unknown: "时间关系待确认"
    })[relation] || "时间关系未填";
    const pending = (value.evidence_conflicts || []).filter(item => !["resolved", "confirmed"].includes(item.status)).length +
      (value.unresolved_questions || value.pending_questions || []).length +
      assays.filter(assay => !assay.timepoint_relation || assay.timepoint_relation === "unknown").length;
    const assayLines = assays.map(assay => `<div class="summary-assay"><strong>${esc(assay.title || "未命名检测")}</strong><span>${esc((assay.timepoints || []).join(" / ") || "无时间点")}</span><span>${esc(relationLabel(assay.timepoint_relation))}</span><span>${esc([assay.endpoint, assay.method].filter(Boolean).join(" · ") || "终点未填")}</span><span>Protocol ${esc(assay.protocol_version_id ? protocolTitle(assay.protocol_version_id) : "未关联")}</span></div>`).join("");
    box.innerHTML = `<div class="summary-main"><strong>${esc(title(design))}</strong><span>${groups.length} 组</span><span>${assays.length} 项检测</span><span>${pending} 项待确认</span><button class="btn" id="editWorkflowDesign" type="button">编辑本次安排</button><button class="btn primary" id="startWorkflowRun" type="button">开始运行</button></div><div class="summary-assays">${assayLines || '<span class="page-muted">没有检测安排。</span>'}</div>`;
    box.hidden = false;
    box.querySelector("#editWorkflowDesign").onclick = () => openDesignEditor(design);
    box.querySelector("#startWorkflowRun").onclick = async () => {
      const button = box.querySelector("#startWorkflowRun");
      button.disabled = true; button.textContent = "创建运行…";
      try {
        const response = await fetch("/api/records", {method: "POST", headers: {"content-type": "application/json", "X-AsterLab-Human-Confirmed": "1"}, body: JSON.stringify({kind: "run", project_id: design.project_id, parent_id: design.id, data: {title: `${title(design)} 执行`, view_id: window.AsterCurrentViewId || null}})});
        const result = await response.json();
        if (!response.ok) throw new Error(result.error || `HTTP ${response.status}`);
        const runId = result.record?.id || result.id;
        location.href = `/?page=runs&project=${encodeURIComponent(design.project_id)}&run=${encodeURIComponent(runId)}`;
      } catch (error) {
        button.disabled = false; button.textContent = "开始运行";
        box.insertAdjacentHTML("beforeend", `<span class="summary-error">${esc(error.message || error)}</span>`);
      }
    };
  }
  function openDesignEditor(design) {
    const existing = $("designInlineEdit"); if (existing) existing.remove();
    const value = JSON.parse(JSON.stringify(data(design)));
    const assays = Array.isArray(value.assays) ? value.assays : [];
    const relationLabels = [
      ["same_experimental_unit_repeated", "同一实验单位重复"],
      ["same_physical_sample_repeated", "同一实体样本重复"],
      ["independent_sample_per_timepoint", "每个时间点独立样本"],
      ["unknown", "待确认"]
    ];
    const groups = (value.group_specs || []).map((group, index) => `<div class="inline-edit-row"><strong>${esc(group.name || "未命名组")}</strong><label>条件<input data-edit-group="${index}" data-field="condition" value="${esc(group.condition || "")}"></label><label>独立 n<input data-edit-group="${index}" data-field="n" type="number" min="0" step="1" value="${group.independent_n ?? ""}"></label><label>技术重复<input data-edit-group="${index}" data-field="tech" type="number" min="0" step="1" value="${group.technical_repeats ?? ""}"></label></div>`).join("");
    const assayCards = assays.map((assay, index) => `<section class="inline-edit-assay" data-assay-card="${index}">
      <div class="inline-edit-assay-head"><strong>${esc(assay.title || `检测 ${index + 1}`)}</strong><small>${esc(one(assay.protocol_version_id)?.parent_id ? title(one(one(assay.protocol_version_id)?.parent_id)) : "Protocol 未关联")}</small></div>
      <div class="inline-edit-assay-grid">
        <label>名称<input data-assay-index="${index}" data-field="title" value="${esc(assay.title || "")}"></label>
        <label>终点<input data-assay-index="${index}" data-field="endpoint" value="${esc(assay.endpoint || "")}"></label>
        <label>方法<input data-assay-index="${index}" data-field="method" value="${esc(assay.method || "")}"></label>
        <label>单位<input data-assay-index="${index}" data-field="unit" value="${esc(assay.unit || "")}"></label>
        <label>时间点（逗号分隔）<input data-assay-index="${index}" data-field="timepoints" value="${esc((assay.timepoints || []).join(", "))}"></label>
        <label>时间点关系<select data-assay-index="${index}" data-field="timepoint_relation">${relationLabels.map(([key, label]) => `<option value="${key}" ${assay.timepoint_relation === key ? "selected" : ""}>${label}</option>`).join("")}</select></label>
        <label>对照（逗号分隔）<input data-assay-index="${index}" data-field="controls" value="${esc((assay.controls || []).join(", "))}"></label>
      </div>
    </section>`).join("") || '<div class="page-muted">此设计还没有检测。</div>';
    const panel = document.createElement("section");
    panel.id = "designInlineEdit";
    panel.className = "design-inline-edit";
    panel.innerHTML = `<div class="inline-edit-head"><div><strong>编辑本次安排</strong><small>修改所有检测、组别和时间关系；只保存 Design，引用的 Protocol 版本不变。</small></div><button class="btn" type="button" id="cancelDesignEdit">取消</button></div>
      <div class="inline-edit-grid"><label>研究目的<textarea data-edit-field="objective">${esc(value.objective || "")}</textarea></label><label>研究问题<input data-edit-field="research_question" value="${esc(value.research_question || "")}"></label></div>
      <div class="inline-edit-section-title">检测</div>${assayCards}
      <div class="inline-edit-section-title">组别</div><div class="inline-edit-groups">${groups || '<div class="page-muted">此设计还没有组别。</div>'}</div>
      <div class="inline-edit-actions"><span id="designEditStatus" class="page-muted"></span><button class="btn primary" id="saveDesignEdit" type="button">保存设计</button></div>`;
    const backdrop=document.createElement("div");
    backdrop.className="design-edit-backdrop";
    backdrop.setAttribute("aria-hidden","true");
    document.body.append(backdrop,panel);
    panel.querySelector("#cancelDesignEdit").onclick = () => {panel.remove();backdrop.remove()};
    panel.querySelector("#saveDesignEdit").onclick = async () => {
      const status = panel.querySelector("#designEditStatus"); status.textContent = "保存中…";
      panel.querySelectorAll("[data-edit-field]").forEach(input => { value[input.dataset.editField] = input.value.trim(); });
      panel.querySelectorAll("[data-edit-group]").forEach(input => {
        const group = value.group_specs[Number(input.dataset.editGroup)]; if (!group) return;
        if (input.dataset.field === "condition") group.condition = input.value.trim();
        if (input.dataset.field === "n") group.independent_n = input.value === "" ? null : Number(input.value);
        if (input.dataset.field === "tech") group.technical_repeats = input.value === "" ? null : Number(input.value);
      });
      panel.querySelectorAll("[data-assay-index]").forEach(input => {
        const assay = (value.assays || [])[Number(input.dataset.assayIndex)]; if (!assay) return;
        const field = input.dataset.field;
        if (field === "timepoints" || field === "controls") assay[field] = input.value.split(",").map(item => item.trim()).filter(Boolean);
        else if (field === "timepoint_relation") assay[field] = input.value;
        else assay[field] = input.value.trim();
      });
      try {
        const response = await fetch("/api/records/update", {method: "POST", headers: {"content-type": "application/json"}, body: JSON.stringify({id: design.id, expected_revision: design.revision, data: value})});
        const result = await response.json();
        if (!response.ok) throw new Error(result.error || `HTTP ${response.status}`);
        await ctx.reloadState();
        renderWorkflowSummary(ctx.one(design.id));
        panel.remove();backdrop.remove();
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
  function recordSchemaFor(assay) {
    const text = `${assay?.title || ""} ${assay?.endpoint || ""} ${assay?.method || ""}`;
    if (/CCK|OD450|viability|活力/i.test(text)) return {
      title: "CCK-8 读板记录",
      columns: [
        {key:"well", label:"孔位", placeholder:"A1"},
        {key:"group", label:"组别", placeholder:"Control"},
        {key:"od", label:"OD450", type:"number", unit:"AU", step:"any"}
      ],
      map: row => Number.isFinite(Number(row.od)) && row.od !== "" ? [{endpoint:"OD450", value:Number(row.od), unit:"AU", sample_id:[row.group,row.well].filter(Boolean).join(" · ") || null}] : []
    };
    if (/qPCR|Cq|荧光定量/i.test(text)) return {
      title: "qPCR 读数记录",
      columns: [
        {key:"sample", label:"样本", placeholder:"Control-1"},
        {key:"target", label:"靶标", placeholder:"GAPDH"},
        {key:"cq", label:"Cq", type:"number", unit:"Cq", step:"any"}
      ],
      map: row => Number.isFinite(Number(row.cq)) && row.cq !== "" ? [{endpoint:row.target || "Cq", value:Number(row.cq), unit:"Cq", sample_id:row.sample || null}] : []
    };
    if (/LIVE|DEAD|live\/dead|荧光|存活/i.test(text)) return {
      title: "LIVE/DEAD 图像计数",
      columns: [
        {key:"field", label:"样本 / 视野", placeholder:"D1-Control-F1"},
        {key:"live", label:"活细胞", type:"number", unit:"cells", step:"1"},
        {key:"dead", label:"死细胞", type:"number", unit:"cells", step:"1"},
        {key:"image", label:"图像文件", placeholder:"可选"}
      ],
      map: row => {
        const values = [];
        if (Number.isFinite(Number(row.live)) && row.live !== "") values.push({endpoint:"live cells", value:Number(row.live), unit:"cells/field", sample_id:row.field || null});
        if (Number.isFinite(Number(row.dead)) && row.dead !== "") values.push({endpoint:"dead cells", value:Number(row.dead), unit:"cells/field", sample_id:row.field || null});
        return values;
      }
    };
    if (/Western|blot|band|条带/i.test(text)) return {
      title: "Western blot 条带记录",
      columns: [
        {key:"sample", label:"样本", placeholder:"Control-1"},
        {key:"target", label:"目标条带", placeholder:"p-AKT"},
        {key:"band", label:"目标强度", type:"number", unit:"AU", step:"any"},
        {key:"loading", label:"Loading control", type:"number", unit:"AU", step:"any"}
      ],
      map: row => {
        const values = [];
        if (Number.isFinite(Number(row.band)) && row.band !== "") values.push({endpoint:row.target || "target band", value:Number(row.band), unit:"AU", sample_id:row.sample || null});
        if (Number.isFinite(Number(row.loading)) && row.loading !== "") values.push({endpoint:"loading control", value:Number(row.loading), unit:"AU", sample_id:row.sample || null});
        return values;
      }
    };
    return {
      title: `${assay?.title || "检测"} 结果记录`,
      columns: [
        {key:"sample", label:"样本 / 孔位", placeholder:"A1"},
        {key:"endpoint", label:"终点", placeholder:assay?.endpoint || "读数"},
        {key:"value", label:"数值", type:"number", step:"any"},
        {key:"unit", label:"单位", placeholder:assay?.unit || ""}
      ],
      map: row => Number.isFinite(Number(row.value)) && row.value !== "" ? [{endpoint:row.endpoint || assay?.endpoint || "measurement", value:Number(row.value), unit:row.unit || assay?.unit || null, sample_id:row.sample || null}] : []
    };
  }

  window.AsterRecordSchemas = { forAssay: recordSchemaFor };

  function structuredRecordRows(events) {
    return events.flatMap(event => {
      const value = data(event), run = one(event.parent_id);
      const measurements = (value.measurements || []).map(item => ({...item, kind:"measurement"}));
      const readouts = (value.readouts || []).map(item => ({...item, kind:"readout"}));
      return [...measurements, ...readouts].map(item => ({
        run: run ? title(run) : "未知运行",
        assay: value.assay_id || "—",
        timepoint: value.timepoint || "—",
        sample: item.sample_id || "—",
        endpoint: item.endpoint || "—",
        value: item.value ?? item.qualifier ?? "—",
        unit: item.unit || ""
      }));
    });
  }

  function renderRecords() {
    const events = records("execution_event"), notes = records("note"), runs = records("run");
    const params = new URLSearchParams(location.search);
    const run = runs.find(item => item.id === params.get("record-run")) || runs[0];
    const design = run?.parent_id ? one(run.parent_id) : null;
    const assays = design ? (data(design).assays || []) : [];
    const assay = assays.find(item => item.id === params.get("record-assay")) || assays[0];
    const schema = recordSchemaFor(assay);
    const manifest = (data(run).protocol_manifest || []).find(item => item.assay_id === assay?.id);
    const steps = manifest?.steps || [];
    const step = steps.find(item => /read|measure|image|Cq|OD|检测|成像|读|记录/i.test(item.title || "")) || steps[steps.length - 1];
    const timepoints = assay?.timepoints || [];
    const structured = structuredRecordRows(events).filter(row => !run || row.run === title(run));
    const columns = schema.columns.map(column => `<th>${esc(column.label)}${column.unit ? `<small>${esc(column.unit)}</small>` : ""}</th>`).join("");
    const rowMarkup = schema.columns.map(column => `<td><input data-record-field="${esc(column.key)}" type="${esc(column.type || "text")}" step="${esc(column.step || "any")}" placeholder="${esc(column.placeholder || "")}" aria-label="${esc(column.label)}"></td>`).join("");
    $("page-view").innerHTML = header("实验记录", "按实验设计中的 Assay 记录结构化读数；原始事件仍保留。") +
      `<div class="page-grid">
        <section class="page-card"><h2>记录入口</h2>
          <div class="record-controls">
            <label>实验运行<select id="record-run">${runs.map(item => `<option value="${esc(item.id)}" ${item.id === run?.id ? "selected" : ""}>${esc(title(item))}</option>`).join("") || '<option value="">尚无运行</option>'}</select></label>
            <label>检测<select id="record-assay">${assays.map(item => `<option value="${esc(item.id)}" ${item.id === assay?.id ? "selected" : ""}>${esc(item.title)}</option>`).join("") || '<option value="">尚无检测</option>'}</select></label>
            <label>时间点<select id="record-timepoint"><option value="">不指定</option>${timepoints.map(item => `<option value="${esc(item)}">${esc(item)}</option>`).join("")}</select></label>
          </div>
          <div class="record-schema-head"><strong>${esc(schema.title)}</strong><span>${esc(step ? `记录到 ${step.title}` : "未找到可关联步骤")}</span></div>
          <div class="data-table-wrap"><table class="record-entry-table"><thead><tr>${columns}</tr></thead><tbody id="record-rows"><tr>${rowMarkup}</tr></tbody></table></div>
          <div class="record-actions"><button class="btn" id="add-record-row" type="button">+ 添加行</button><button class="btn primary" id="save-record" type="button" onclick="window.__saveRecord?.()" ${step ? "" : "disabled"}>保存结构化记录</button><span id="record-status" class="page-muted"></span></div>
        </section>
        <section class="page-card"><h2>结构化记录</h2>${structured.length ? `<div class="data-table-wrap"><table><thead><tr><th>运行</th><th>检测</th><th>时间点</th><th>样本 / 孔位</th><th>终点</th><th>数值</th><th>单位</th></tr></thead><tbody>${structured.map(row => `<tr><td>${esc(row.run)}</td><td>${esc(row.assay)}</td><td>${esc(row.timepoint)}</td><td>${esc(row.sample)}</td><td>${esc(row.endpoint)}</td><td>${esc(row.value)}</td><td>${esc(row.unit)}</td></tr>`).join("")}</tbody></table></div>` : '<div class="page-muted">尚无结构化读数。'}</section>
        <section class="page-card"><details><summary>原始事件（${events.length}）</summary>${events.length ? `<div class="data-table-wrap"><table><thead><tr><th>运行</th><th>类型</th><th>内容</th><th>时间</th></tr></thead><tbody>${events.map(event => `<tr><td>${esc(title(one(event.parent_id)))}</td><td>${esc(data(event).event_type)}</td><td>${esc(data(event).content)}</td><td>${esc(data(event).occurred_at || "")}</td></tr>`).join("")}</tbody></table></div>` : '<div class="page-muted">尚无执行事件。</div>'}</details></section>
        <section class="page-card"><h2>补充记录</h2>${notes.length ? notes.map(note => `<div class="page-row"><div><strong>${esc(data(note).content)}</strong><small>${esc(note.created_at || "")}</small></div></div>`).join("") : '<div class="page-muted">尚无补充记录。</div>'}</section>
      </div>`;
    const updateSelection = (key, value) => {
      const url = new URL(location.href);
      url.searchParams.set(key, value);
      window.history.replaceState(null, "", url);
      renderRecords();
    };
    $("record-run")?.addEventListener("change", event => { const url = new URL(location.href); url.searchParams.set("record-run", event.target.value); url.searchParams.delete("record-assay"); history.replaceState(null, "", url); renderRecords(); });
    $("record-assay")?.addEventListener("change", event => updateSelection("record-assay", event.target.value));
    $("add-record-row")?.addEventListener("click", () => {
      const body = $("record-rows");
      body.insertAdjacentHTML("beforeend", `<tr>${rowMarkup}</tr>`);
    });
    window.__saveRecord = async () => {
      const status = $("record-status"); try {
      const rows = [...document.querySelectorAll("#record-rows tr")].map(tr => Object.fromEntries([...tr.querySelectorAll("[data-record-field]")].map(input => [input.dataset.recordField, input.value.trim()])));
      const measurements = rows.flatMap(row => schema.map(row));
      if (!measurements.length) { status.textContent = "至少填写一行有效数值。"; return; }
      if (!run || !assay || !step) { status.textContent = "缺少运行、检测或可关联步骤。"; return; }
      status.textContent = "保存中…";
      const payload = {kind:"execution_event",parent_id:run.id,project_id:run.project_id,data:{event_type:"measurement",content:`结构化记录 · ${assay.title}`,protocol_step_path:step.path,protocol_version_id:manifest.protocol_version_id,assay_id:assay.id,timepoint:$("record-timepoint")?.value || null,measurements}};
      const response = await fetch("/api/records",{method:"POST",headers:{"content-type":"application/json"},body:JSON.stringify(payload)});
      if (!response.ok) { const body = await response.json().catch(() => ({})); status.textContent = body.error || `保存失败（${response.status}）`; return; }
      await ctx.reloadState();
      renderRecords();
      } catch (error) { if (status) status.textContent = error.message || String(error); console.error(error); }
    };
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
  function openCalculator(mode = "dilution") {
    document.getElementById("calculator-drawer")?.remove();
    document.getElementById("calculator-backdrop")?.remove();
    const backdrop = document.createElement("div");
    backdrop.id = "calculator-backdrop";
    backdrop.className = "calculator-backdrop";
    const drawer = document.createElement("section");
    drawer.id = "calculator-drawer";
    drawer.className = "calculator-drawer";
    const modes = [["dilution", "稀释"], ["reagent", "试剂总量"], ["serial", "系列稀释"], ["reaction", "反应体积"]];
    const fields = {
      dilution: `<div class="calc-grid"><label>储液浓度 C1<input name="c1" type="number" min="0" step="any" placeholder="0"></label><label>目标浓度 C2<input name="c2" type="number" min="0" step="any" placeholder="0"></label><label>目标体积 V2<input name="v2" type="number" min="0" step="any" placeholder="0"></label><label>浓度单位<select name="c_unit"><option value="1">M</option><option value="0.001" selected>mM</option><option value="0.000001">µM</option></select></label><label>体积单位<select name="v_unit"><option value="0.001" selected>mL</option><option value="0.000001">µL</option></select></label></div>`,
      reagent: `<div class="calc-grid"><label>孔位数<input name="positions" type="number" min="1" step="1" placeholder="96"></label><label>每孔体积 (µL)<input name="volume" type="number" min="0" step="any" placeholder="100"></label><label>额外比例<input name="extra" type="number" min="0" step="any" placeholder="0.1"></label></div>`,
      serial: `<div class="calc-grid"><label>初始浓度<input name="initial" type="number" min="0" step="any" placeholder="1"></label><label>转移体积<input name="transfer" type="number" min="0" step="any" placeholder="100"></label><label>稀释液体积<input name="diluent" type="number" min="0" step="any" placeholder="900"></label><label>级数<input name="steps" type="number" min="1" max="100" step="1" placeholder="6"></label></div>`,
      reaction: `<div class="calc-grid"><label>目标总体积 (µL)<input name="target" type="number" min="0" step="any" placeholder="50"></label><label>已分配体积 (µL)<input name="used" type="number" min="0" step="any" placeholder="30"></label></div>`
    };
    drawer.innerHTML = `<div class="calc-head"><div><h2>计算器</h2><small>算术结果不是实验条件建议；请核对单位与适用方案。</small></div><button class="btn" id="calcClose" type="button">×</button></div><div class="calc-tabs">${modes.map(([key,label])=>`<button class="btn ${key===mode?"primary":""}" data-calc-mode="${key}" type="button">${label}</button>`).join("")}</div><div id="calcForm">${fields[mode]}</div><button class="btn primary" id="calcRun" type="button">计算</button><div id="calcResult" class="calc-result" role="status" aria-live="polite">填写输入后计算。</div><p class="page-muted">结果只做算术；单位、纯度、盐型和实验条件由研究者核对。</p>`;
    document.body.append(backdrop, drawer);
    const close = () => { drawer.remove(); backdrop.remove(); };
    drawer.querySelector("#calcClose").onclick = close;
    backdrop.onclick = close;
    const drawForm = next => {
      drawer.querySelectorAll("[data-calc-mode]").forEach(button => button.classList.toggle("primary", button.dataset.calcMode === next));
      drawer.querySelector("#calcForm").innerHTML = fields[next];
      drawer.querySelector("#calcResult").textContent = "填写输入后计算。";
      drawer.querySelector("#calcRun").onclick = () => calculate(next);
    };
    const number = name => Number(drawer.querySelector(`[name="${name}"]`)?.value);
    const calculate = next => {
      const result = drawer.querySelector("#calcResult");
      const fail = text => { result.textContent = text; result.classList.add("is-error"); };
      result.classList.remove("is-error");
      if (next === "dilution") {
        const c1 = number("c1"), c2 = number("c2"), v2 = number("v2"), cu = number("c_unit"), vu = number("v_unit");
        if (![c1,c2,v2,cu,vu].every(value => Number.isFinite(value) && value > 0)) return fail("请填写大于 0 的浓度和体积。");
        const v1L = (c2 * cu * v2 * vu) / (c1 * cu);
        result.textContent = `V1 = ${Number((v1L * 1e6).toFixed(4))} µL（${Number((v1L * 1000).toFixed(6))} mL）`;
      } else if (next === "reagent") {
        const positions = number("positions"), volume = number("volume"), extra = number("extra");
        if (![positions,volume,extra].every(value => Number.isFinite(value) && value >= 0) || !Number.isInteger(positions) || !positions) return fail("孔位数必须是正整数，体积和额外比例不能为负。");
        result.textContent = `总量 = ${Number((positions * volume * (1 + extra)).toFixed(3))} µL`;
      } else if (next === "serial") {
        const initial = number("initial"), transfer = number("transfer"), diluent = number("diluent"), steps = number("steps");
        if (![initial,transfer,diluent,steps].every(value => Number.isFinite(value) && value > 0) || !Number.isInteger(steps) || steps > 100) return fail("浓度、体积和 1–100 的整数级数必须有效。");
        let current = initial; const values = [];
        for (let i = 1; i <= steps; i++) { current = current * transfer / (transfer + diluent); values.push(`${i}: ${Number(current.toFixed(6))}`); }
        result.textContent = values.join(" · ");
      } else {
        const target = number("target"), used = number("used");
        if (![target,used].every(value => Number.isFinite(value) && value >= 0) || used > target) return fail("已分配体积不能超过目标总体积。");
        result.textContent = `剩余体积 = ${Number((target - used).toFixed(3))} µL`;
      }
    };
    drawer.querySelectorAll("[data-calc-mode]").forEach(button => button.onclick = () => { mode = button.dataset.calcMode; drawForm(mode); });
    drawForm(mode);
  }

  function resourceCard(item, kind) {
    const icon = window.AsterLabIcons?.[item.id] || window.AsterLabIcons?.flask || "";
    const labels = {absorbance:"吸光度", fluorescence:"荧光", culture:"培养", incubation:"孵育", separation:"分离", imaging:"成像", wash:"洗涤", "liquid-transfer":"液体转移", containment:"防护"};
    const tags = (item.tags || []).map(tag => `<span class="resource-tag">${esc(labels[tag] || tag)}</span>`).join("");
    return `<article class="resource-card"><div class="resource-icon">${icon}</div><div class="resource-copy"><strong>${esc(item.title)}</strong><p>${esc(item.summary)}</p><div class="resource-meta">${tags || `<span class="resource-tag">${kind}</span>`}</div></div></article>`;
  }

  function renderTools() {

    const tables = records("library_table"), formulas = records("formula");
    const materials = records("material"), instruments = records("instrument");
    const hasSavedResources = tables.length || formulas.length || materials.length || instruments.length;
    $("page-view").innerHTML = header("资料与工具", "快速进入实验操作，查阅可复用资料并管理项目数据。") +
      `<div class="page-grid">
        <section class="page-card"><h2>快速入口</h2><div class="page-tools">
          <button class="btn primary" onclick="AsterPageCtx.setPage('workflow')">实验设计</button>
          <button class="btn" onclick="AsterPageCtx.setPage('protocols')">Protocol</button>
          <button class="btn" onclick="AsterPageCtx.setPage('runs')">实验运行</button>
          <button class="btn" onclick="AsterPageCtx.setPage('records')">记录实验结果</button>
        </div><p class="page-muted">计算器在下方“计算器与布局”中打开，不再跳转旧页面。</p></section>
        ${hasSavedResources ? `<section class="page-card"><h2>已保存资料</h2>
          <h3>资料表</h3><div class="page-list">${resourceRows(tables, "library_table")}</div>
          <h3>公式</h3><div class="page-list">${resourceRows(formulas, "formula")}</div>
          <h3>材料</h3><div class="page-list">${resourceRows(materials, "material")}</div>
          <h3>仪器</h3><div class="page-list">${resourceRows(instruments, "instrument")}</div>
        </section>` : ""}
        <section class="page-card"><h2>仪器与设备 · ${(window.AsterResources?.instruments || []).length}</h2><div class="resource-grid">${(window.AsterResources?.instruments || []).map(item => resourceCard(item, "仪器")).join("") || '<div class="page-muted">尚无仪器目录。</div>'}</div></section>
        <section class="page-card"><h2>计算器与布局</h2><div class="page-list">${(window.AsterResources?.tools || []).map(item => `<div class="page-row"><div><strong>${esc(item.title)}</strong><small>${esc(item.summary)}</small></div>${item.view === "calculators" ? `<button class="btn" data-calculator="${esc(item.mode || "dilution")}">打开</button>` : `<button class="btn" disabled title="旧版工具页已下线，待迁移">暂不可用</button>`}</div>`).join("") || '<div class="page-muted">尚无工具目录。</div>'}</div></section>
        <section class="page-card"><h2>材料与配方 · ${(window.AsterResources?.materials || []).length}</h2><div class="resource-grid material-grid">${(window.AsterResources?.materials || []).map(item => resourceCard(item, "材料")).join("") || '<div class="page-muted">尚无材料目录。</div>'}</div></section>
        <section class="page-card"><h2>数据与项目管理</h2><div class="page-tools">
          <button class="btn" onclick="AsterPageCtx.setPage('records')">查看实验记录</button>
          <button class="btn" onclick="AsterPageCtx.setPage('projects')">查看项目</button>
          <button class="btn" onclick="AsterPageCtx.exportState()">导出数据 JSON</button>
          <button class="btn" onclick="AsterPageCtx.backupData().catch(error => { document.getElementById('status').textContent = error.message; })">备份数据库</button>
        </div><p class="page-muted">项目切换位于顶部；导出包含当前本机状态。</p></section>
      </div>`;
    document.querySelectorAll("[data-calculator]").forEach(button => button.onclick = () => openCalculator(button.dataset.calculator));
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
