(() => {
  const definition = "tool.calculator";

  function renderDetail(instance, ctx) {
    const id = ctx.esc(instance.instance_id);
    const props = instance.props || {};
    const draft = ctx.drafts.get(instance.instance_id) || {};
    const plates = ctx.plates || [];
    const boundId = instance.model_ref?.container_id;
    const bound = boundId ? plates.filter(plate => plate.model_ref?.container_id === boundId) : [];
    const selectedId = bound.length === 1 ? bound[0].instance_id : "";
    const options = plates.map(plate => `<option value="${ctx.esc(plate.instance_id)}" ${plate.instance_id === selectedId ? "selected" : ""}>${ctx.esc(ctx.title(plate))} · ${ctx.esc(plate.model_ref?.container_id || "未绑定容器")}</option>`).join("");
    const volume = draft.value ?? props.value ?? props.volume ?? "";
    const unit = draft.unit ?? props.unit ?? "";
    const extra = draft.extra ?? props.extra_ratio ?? "";
    const source = props.value_source || props.source || "";
    return `<article class="card cmp-calculator" data-instance="${id}" data-component="${definition}">
      <header class="cmp-calculator__header"><div><h3>${ctx.esc(ctx.title(instance))}</h3><div class="cmp-calculator__meta">${ctx.esc(props.tool_ref || definition)}</div></div></header>
      <div class="cmp-calculator__formula">positions × volume × (1 + extra) = total</div>
      <div class="cmp-calculator__fields">
        <label>每孔体积<input data-calc-value type="number" min="0" step="any" value="${ctx.esc(volume)}" placeholder="待确认" aria-label="每孔体积"><small>来源：${ctx.esc(source || (volume !== "" ? "组件输入" : "待确认"))}</small></label>
        <label>单位<input data-calc-unit value="${ctx.esc(unit)}" placeholder="µL" aria-label="体积单位"><small>来源：${ctx.esc(props.unit_source || (unit ? "组件输入" : "待确认"))}</small></label>
        <label>额外比例<input data-calc-extra type="number" min="0" step="any" value="${ctx.esc(extra)}" placeholder="待确认" aria-label="额外比例"><small>比例值，例如 0.10 = 10%</small></label>
      </div>
      <div class="cmp-calculator__status" data-calc-status role="status">等待完整输入与明确孔板绑定。</div>
      ${plates.length ? `<label class="cmp-calculator__plate">关联孔板<select data-calc-plate><option value="">选择孔板…</option>${options}</select></label><div data-calc-plate-state class="cmp-calculator__meta">${selectedId ? "读取孔板信息…" : "尚未绑定孔板"}</div>` : `<div class="cmp-calculator__status">没有可用孔板。</div>`}
      <div class="cmp-calculator__actions"><button type="button" class="cmp-calculator__run" data-calc-run>计算总量</button></div>
      <div class="cmp-calculator__result" data-calc-result role="status" aria-live="polite">结果待确认</div>
    </article>`;
  }

  function mount(root, instance, ctx) {
    const controller = new AbortController();
    const signal = controller.signal;
    const value = root.querySelector("[data-calc-value]");
    const unit = root.querySelector("[data-calc-unit]");
    const status = root.querySelector("[data-calc-status]");
    const extra = root.querySelector("[data-calc-extra]");
    const result = root.querySelector("[data-calc-result]");
    const select = root.querySelector("[data-calc-plate]");
    const plateState = root.querySelector("[data-calc-plate-state]");
    const updateStatus = () => {
      const valid = value?.value !== "" && Number.isFinite(Number(value?.value)) && Number(value.value) > 0 &&
        !!unit?.value.trim() && extra?.value !== "" && Number.isFinite(Number(extra?.value)) && Number(extra.value) >= 0;
      const count = select?.value ? (ctx.selectedPositions?.(select.value) || []).length : 0;
      status.textContent = valid && count ? `输入已提供；${count} 个孔位将用于计算。` : "待确认：需提供体积、单位、额外比例并选择孔板与孔位。";
      return valid;
    };
    const saveInputDraft = () => ctx.drafts.set(instance.instance_id, { value: value.value, unit: unit.value, extra: extra.value });
    [value, unit, extra].forEach(input => input?.addEventListener("input", () => { saveInputDraft(); updateStatus(); result.textContent = "输入已更改，结果待重新计算。"; }, { signal }));
    let request = 0;
    const load = async () => {
      const plateId = select?.value;
      const current = ++request;
      if (!plateId) { if (plateState) plateState.textContent = "尚未绑定孔板"; updateStatus(); return; }
      if (plateState) plateState.textContent = "正在读取孔板信息…";
      try {
        const projection = await ctx.loadPlate(plateId);
        if (signal.aborted || current !== request) return;
        if (plateState) plateState.textContent = projection?.rows && projection?.columns
          ? `${projection.rows} 行 × ${projection.columns} 列 · ${projection.positions?.length ?? "待确认"} 个已分配孔位`
          : `孔板配置未解析 · ${projection?.profile_id || "待确认"}`;
      } catch (error) {
        if (!signal.aborted && current === request && plateState) plateState.textContent = `孔板信息未能读取：${error.message || error}`;
      }
      updateStatus();
    };
    select?.addEventListener("change", () => { load(); result.textContent = "孔板已更改，结果待重新计算。"; }, { signal });
    if (select?.value) load();
    updateStatus();
    root.querySelector("[data-calc-run]")?.addEventListener("click", async () => {
      const plateId = select?.value;
      const positions = plateId ? (ctx.selectedPositions?.(plateId) || []) : [];
      if (!plateId) { result.textContent = "待确认：请明确选择关联孔板。"; return; }
      if (!updateStatus()) { result.textContent = "待确认：体积、单位或额外比例无效。"; return; }
      if (!positions.length) { result.textContent = "待确认：请先在已绑定孔板中选择目标孔位。"; return; }
      const current = ++request;
      result.textContent = "正在调用计算工具…";
      try {
        const output = await ctx.calculate({ calculatorId: instance.instance_id, plateId, positions, volume: value.value, unit: unit.value.trim(), extra: extra.value });
        if (signal.aborted || current !== request) return;
        const calculation = output?.calculation;
        if (!calculation || calculation.state !== "resolved" ||
            calculation.total_volume === null || calculation.total_volume === undefined ||
            !calculation.unit || calculation.assigned_positions === null || calculation.assigned_positions === undefined) {
          result.textContent = `待确认：计算工具未返回完整解析结果。${calculation?.tool_action ? ` 工具动作：${calculation.tool_action}` : ""}`;
          return;
        }
        const total = Number(calculation.total_volume);
        const displayTotal = Number.isFinite(total) ? total.toFixed(1) : calculation.total_volume;
        result.innerHTML = `<strong>${ctx.esc(displayTotal)} ${ctx.esc(calculation.unit)}</strong><span>${ctx.esc(calculation.assigned_positions)} × ${ctx.esc(value.value)} ${ctx.esc(unit.value.trim())} × ${ctx.esc((1 + Number(extra.value)).toFixed(2))} · 已解析</span><small>工具动作：${ctx.esc(calculation.tool_action || "未提供")}</small>`;
      } catch (error) {
        if (!signal.aborted && current === request) result.textContent = `计算失败：${error.message || error}`;
      }
    }, { signal });
    return () => controller.abort();
  }

  window.AsterUI.register("tool.calculator@1", { renderDetail, mount });
})();
