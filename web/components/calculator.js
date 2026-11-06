(() => {

  const definition = "tool.calculator";

  function renderDetail(instance, ctx) {
    const id = ctx.esc(instance.instance_id);
    const props = instance.props || {};
    const draft = ctx.drafts.get(instance.instance_id);
    const plates = ctx.plates || [];
    const plateOptions = plates.map(plate => `<option value="${ctx.esc(plate.instance_id)}" ${instance.model_ref?.container_id && plate.model_ref?.container_id === instance.model_ref.container_id ? "selected" : ""}>${ctx.esc(ctx.title(plate))} · ${ctx.esc(plate.model_ref?.container_id || "未绑定容器")}</option>`).join("");
    const selectedPlate = plates.find(plate => plate.model_ref?.container_id === instance.model_ref?.container_id);
    const quantity = draft?.value ?? props.value ?? props.volume ?? "";
    const unit = draft?.unit ?? props.unit ?? "";
    const unresolved = quantity === "" || quantity === null || quantity === undefined || !unit;
    return `<article class="card cmp-calculator" data-instance="${id}" data-component="${definition}">
      <header class="cmp-calculator__header"><div><h3>${ctx.esc(ctx.title(instance))}</h3><div class="cmp-calculator__meta">${ctx.esc(props.tool_ref || definition)}</div></div></header>
      <div class="cmp-calculator__fields">
        <label>每孔体积<input data-calc-value type="number" min="0" step="any" value="${ctx.esc(quantity)}" placeholder="待确认" aria-label="每孔体积"></label>
        <label>单位<input data-calc-unit value="${ctx.esc(unit)}" placeholder="待确认" aria-label="体积单位"></label>
        <label>额外比例<input data-calc-extra type="number" min="0" step="any" value="${ctx.esc(draft?.extra ?? props.extra_ratio ?? "")}" placeholder="可选" aria-label="额外比例"></label>
      </div>
      <div class="cmp-calculator__status" data-calc-status role="status">${unresolved ? "待确认：输入体积与单位后由宿主执行计算。" : "已提供体积与单位；计算仍由宿主处理。"}</div>
      ${plates.length ? `<label class="cmp-calculator__plate">关联孔板<select data-calc-plate><option value="">选择孔板…</option>${plateOptions}</select></label><div data-calc-plate-state class="cmp-calculator__meta">${selectedPlate ? "读取孔板信息…" : "尚未选择孔板"}</div>` : `<div class="cmp-calculator__status">没有可用孔板。</div>`}
      <div class="cmp-calculator__actions"><button type="button" class="cmp-calculator__run" data-calc-run>计算</button></div>
      <div class="cmp-calculator__result" data-calc-result role="status"></div>
    </article>`;
  }

  function mount(root, instance, ctx) {
    const controller = new AbortController();
    const signal = controller.signal;
    const value = root.querySelector("[data-calc-value]");
    const unit = root.querySelector("[data-calc-unit]");
    const status = root.querySelector("[data-calc-status]");
    const extra = root.querySelector("[data-calc-extra]");
    const updateStatus = () => {
      const unresolved = !value?.value || !unit?.value.trim();
      status.textContent = unresolved ? "待确认：输入体积与单位后由宿主执行计算。" : "已提供体积与单位；计算仍由宿主处理。";
    };
    const saveInputDraft = () => ctx.drafts.set(instance.instance_id, { value: value.value, unit: unit.value, extra: extra.value });
    [value, unit, extra].forEach(input => input?.addEventListener("input", () => { saveInputDraft(); updateStatus(); }, { signal }));
    const select = root.querySelector("[data-calc-plate]");
    const plateState = root.querySelector("[data-calc-plate-state]");
    let request = 0;
    const load = async () => {
      const plate = (ctx.plates || []).find(item => item.instance_id === select?.value);
      const current = ++request;
      if (!plate) { plateState.textContent = "尚未选择孔板"; return; }
      plateState.textContent = "正在读取孔板信息…";
      try {
        const projection = await ctx.loadPlate(plate.instance_id);
        if (signal.aborted || current !== request) return;
        plateState.textContent = projection.rows && projection.columns
          ? `${projection.rows} 行 × ${projection.columns} 列 · ${projection.positions?.length || 0} 个已分配孔位`
          : `孔板配置未解析 · ${projection.profile_id || "未知配置"}`;
      } catch (error) {
        if (!signal.aborted && current === request) plateState.textContent = `孔板信息未能读取：${error.message || error}`;
      }
    };
    select?.addEventListener("change", load, { signal });
    if (select?.value) load();
    root.querySelector("[data-calc-run]")?.addEventListener("click", async () => {
      const result = root.querySelector("[data-calc-result]");
      const plateId = select?.value;
      if (!plateId) { result.textContent = "请先选择孔板。"; return; }
      if (!value?.value || !unit?.value.trim()) { result.textContent = "体积、单位未解析，不能计算。"; return; }
      const positions = ctx.selectedPositions?.(plateId) || [];
      if (!positions.length) { result.textContent = "请先在孔板中选择目标孔位。"; return; }
      result.textContent = "计算中…";
      try {
        const output = await ctx.calculate({ calculatorId: instance.instance_id, plateId, positions, volume: value.value, unit: unit.value, extra: extra?.value || "0" });
        const calculation = output?.calculation || {};
        result.innerHTML = `<strong>${ctx.esc(calculation.state || "unresolved")}</strong> · ${ctx.esc(calculation.assigned_positions ?? "?")} 孔 · ${ctx.esc(calculation.unit || "无单位")} ${calculation.total_volume !== null && calculation.total_volume !== undefined ? `· 总量 ${ctx.esc(calculation.total_volume)}` : ""}<small>${ctx.esc(calculation.tool_action || "tool action")}</small>`;
      } catch (error) { result.textContent = `计算失败：${error.message || error}`; }
    }, { signal });
    return () => controller.abort();
  }

  window.AsterUI.register("tool.calculator@1", { renderDetail, mount });
})();
