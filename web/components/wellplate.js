(() => {
  const selections = new Map();

  const text = (value) => value == null || value === "" ? "未标注" : String(value);
  const targetLabel = (target, esc) => {
    if (!target) return "未关联目标";
    return esc(text(target.id ?? target.name ?? target.label ?? target));
  };

  function renderPlate(plate, selected, esc) {
    const rows = Number(plate?.rows), columns = Number(plate?.columns);
    if (!Number.isInteger(rows) || !Number.isInteger(columns) || rows < 1 || columns < 1) {
      return `<div class="wp-state" role="status"><strong>⚠ 板型未解析</strong><span>无法读取孔板几何${plate?.profile_id ? ` · ${esc(plate.profile_id)}` : ""}</span></div>`;
    }

    const positions = new Map((plate.positions || []).map(position => [position.position_id, position]));
    const plannedCount = positions.size, total = rows * columns;
    const controls = new Set([...positions].filter(([, position]) =>
      (position.targets || []).some(item => item.role?.toLowerCase() === "control")
    ).map(([id]) => id));
    let cells = `<span class="wp-corner" aria-hidden="true"></span>`;
    for (let column = 1; column <= columns; column++) cells += `<span class="wp-axis" aria-hidden="true">${column}</span>`;
    for (let row = 0; row < rows; row++) {
      const rowName = String.fromCharCode(65 + row);
      cells += `<span class="wp-axis" aria-hidden="true">${rowName}</span>`;
      for (let column = 1; column <= columns; column++) {
        const id = `${rowName}${column}`, position = positions.get(id), chosen = selected.has(id);
        const control = controls.has(id);
        const state = position ? "计划分配" : "未映射（未确认）";
        const details = (position?.targets || []).map(item =>
          `${item.role || "角色未标注"} · ${targetLabel(item.target ?? item, esc)} · ${item.relationship || "关系未标注"}`
        ).join(" / ");
        const label = `${id}，${state}${control ? "，对照" : ""}${chosen ? "，已选择" : ""}${details ? `，${details}` : ""}`;
        cells += `<button type="button" class="wp-well${position ? " is-planned" : " is-unmapped"}${control ? " is-control" : ""}${chosen ? " is-selected" : ""}" data-well="${esc(id)}" aria-pressed="${chosen}" aria-label="${esc(label)}" title="${esc(label)}">${esc(id)}</button>`;
      }
    }
    const details = [...selected].sort((a, b) => a.localeCompare(b, undefined, { numeric: true })).map(id => {
      const position = positions.get(id), targets = position?.targets || [];
      const labels = targets.map(item => `<span><b>${esc(item.role || "角色未标注")}</b> · ${targetLabel(item.target ?? item, esc)} · ${esc(item.relationship || "关系未标注")}</span>`);
      return `<div class="wp-selected-item"><b>${esc(id)}</b>${labels.join("") || `<span>${position ? "目标未标注" : "未映射（未确认）"}</span>`}</div>`;
    }).join("");
    return `<div class="wp-meta"><span>${rows} × ${columns} wells</span><span>${esc(plate.profile_id || "未标注板型")}${plate.profile_version ? ` · v${esc(plate.profile_version)}` : ""}</span></div>
      <div class="wp-toolbar"><button type="button" data-select="all">选择已计划孔位</button><button type="button" data-select="clear">清空</button></div>
      <div class="wp-body"><div class="wp-grid-scroll"><div class="wp-grid" style="--wp-columns:${columns}" role="group" aria-label="${rows} 行 ${columns} 列孔板">${cells}</div></div>
      <aside class="wp-side" aria-live="polite"><div class="wp-side-title">当前选择</div><div class="wp-count"><b>${selected.size}</b><span>已选孔位</span></div>
      <div class="wp-facts"><span>计划分配</span><b>${plannedCount} / ${total}</b><span>对照角色</span><b>${controls.size}</b><span>未确认映射（无计划分配）</span><b>${total - plannedCount}</b></div>
      <div class="wp-selection"><b>目标 / 角色 / 关系</b><div class="wp-selection-list">${details || `<span class="wp-empty-selection">尚未选择孔位。视觉选择不修改实验数据。</span>`}</div></div></aside></div>
      <div class="wp-legend" aria-label="孔位状态图例"><span><i class="planned"></i>计划分配</span><span><i class="selected"></i>已选择</span><span><i class="control"></i>对照角色</span><span><i class="unmapped"></i>未确认映射</span></div>`;
  }

  window.AsterUI = window.AsterUI || { registry: {}, register(key, renderer) { this.registry[key] = renderer; } };
  window.AsterUI.register("lab.well_plate@1", {
    renderDetail(instance, ctx) {
      const esc = ctx.esc;
      const id = esc(instance.instance_id);
      return `<article class="card cmp-wellplate" data-instance="${id}" data-component="lab.well_plate"><header class="wp-header"><div><h3>${esc(ctx.title(instance))}</h3><div class="wp-subtitle">孔板 · ${esc(instance.model_ref?.container_id || "容器未关联")}</div></div><span class="wp-badge">Well plate</span></header><div class="wp-content" data-wellplate-content><div class="wp-state" role="status">正在读取孔板…</div></div></article>`;
    },
    mount(root, instance, ctx) {
      const instanceId = instance.instance_id, esc = ctx.esc;
      let active = true;
      const selection = () => {
        if (!selections.has(instanceId)) selections.set(instanceId, new Set());
        return selections.get(instanceId);
      };
      const content = root.querySelector("[data-wellplate-content]");
      const draw = plate => { if (active && content) content.innerHTML = renderPlate(plate, selection(), esc); };
      const onClick = event => {
        const well = event.target.closest("[data-well]");
        if (well && root.contains(well)) {
          const selected = selection();
          selected.has(well.dataset.well) ? selected.delete(well.dataset.well) : selected.add(well.dataset.well);
          ctx.onPlateSelection?.(instanceId, [...selected]);
          draw(plate);
          return;
        }
        const action = event.target.closest("[data-select]");
        if (action && root.contains(action)) {
          const selected = selection();
          if (action.dataset.select === "all") selected.clear(), (plate.positions || []).forEach(position => selected.add(position.position_id));
          else selected.clear();
          ctx.onPlateSelection?.(instanceId, [...selected]);
          draw(plate);
        }
      };
      let plate;
      root.addEventListener("click", onClick);
      ctx.loadPlate(instanceId).then(value => {
        if (!active) return;
        plate = value;
        draw(plate);
      }).catch(error => {
        if (active && content) content.innerHTML = `<div class="wp-state" role="alert"><strong>⚠ 孔板读取失败</strong><span>${esc(error?.message || "无法读取孔位")}</span></div>`;
      });
      return () => { active = false; root.removeEventListener("click", onClick); };
    }
  });
})();
