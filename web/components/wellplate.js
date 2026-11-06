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
    const groupValues = new Map();
    let cells = `<span class="wp-corner" aria-hidden="true"></span>`;
    for (let column = 1; column <= columns; column++) cells += `<span class="wp-axis" aria-hidden="true">${column}</span>`;
    for (let row = 0; row < rows; row++) {
      const rowName = String.fromCharCode(65 + row);
      cells += `<span class="wp-axis" aria-hidden="true">${rowName}</span>`;
      for (let column = 1; column <= columns; column++) {
        const id = `${rowName}${column}`, position = positions.get(id), targets = position?.targets || [];
        const role = targets.map(item => item.role).filter(Boolean).join(", ");
        const label = targets.map(item => targetLabel(item.target ?? item, esc)).join(", ");
        const group = role || label;
        if (group) groupValues.set(group, (groupValues.get(group) || 0) + 1);
        const chosen = selected.has(id);
        const details = position ? `${role ? `${esc(role)} · ` : ""}${label || "已分配"}` : "空孔";
        cells += `<button type="button" class="wp-well${position ? " is-assigned" : " is-empty"}${chosen ? " is-selected" : ""}" data-well="${esc(id)}" aria-pressed="${chosen}" aria-label="${esc(id)}，${details}" title="${esc(id)} · ${details}">${esc(id)}</button>`;
      }
    }
    const details = [...selected].sort((a, b) => a.localeCompare(b, undefined, { numeric: true })).map(id => {
      const position = positions.get(id), targets = position?.targets || [];
      const labels = targets.map(item => `${item.role ? `${esc(item.role)} · ` : ""}${targetLabel(item.target ?? item, esc)}`);
      return `<span class="wp-selected-item"><b>${esc(id)}</b><span>${labels.join(" / ") || (position ? "已分配" : "空孔")}</span></span>`;
    }).join("");
    const legend = [...groupValues].map(([group, count]) => `<span class="wp-legend-item"><i aria-hidden="true"></i>${esc(group)} <small>${count}</small></span>`).join("");
    return `<div class="wp-meta"><span>${rows} × ${columns} wells</span><span>${esc(plate.profile_id || "未标注板型")}${plate.profile_version ? ` · v${esc(plate.profile_version)}` : ""}</span></div>
      <div class="wp-toolbar"><button type="button" data-select="all">全选</button><button type="button" data-select="clear">清空</button><span>${selected.size} 个已选</span></div>
      <div class="wp-grid-scroll"><div class="wp-grid" style="--wp-columns:${columns}" role="group" aria-label="${rows} 行 ${columns} 列孔板">${cells}</div></div>
      ${legend ? `<div class="wp-legend" aria-label="组别与条件图例"><b>组别 / 条件</b>${legend}</div>` : ""}
      <div class="wp-selection"><b>已选孔位</b><div class="wp-selection-list">${details || `<span class="wp-empty-selection">尚未选择孔位。点击孔位进行本地选择；选择不会修改实验数据。</span>`}</div></div>`;
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
