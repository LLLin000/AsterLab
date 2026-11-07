(() => {
  window.AsterUI.register("operation@1", {
    renderCompact(instance, ctx) {
      const visual = instance.visual || {};
      const rawTitle = ctx.title(instance);
      const explicit = visual.symbol || visual.template;
      const guessed = /BCA|ECL|成像|读板|检测/i.test(rawTitle) ? "reader"
        : /变性|封闭|一抗|二抗|孵育/.test(rawTitle) ? "incubator"
        : /SDS|电泳|转膜/.test(rawTitle) ? "plate"
        : /裂解|洗膜|洗涤/.test(rawTitle) ? "flask"
        : /加样|铺板/.test(rawTitle) ? "pipette" : "flask";
      const icon = ["plate", "pipette", "incubator", "reader", "flask"].includes(explicit) ? explicit : guessed;
      const title = ctx.esc(rawTitle);
      const detail = Object.values(instance.slots || {}).flat();
      const table = preset => detail.find(child => child.definition?.id === "content.table" && child.props?.preset === preset);
      const count = preset => table(preset)?.props?.rows?.length || 0;
      const materials = count("materials"), instruments = count("instruments");
      const meta = materials || instruments ? `${materials} 材料 · ${instruments} 仪器` : (visual.symbol || visual.template || instance.definition?.id || "operation");
      return `<g class="cmp-operation">
        <g class="cmp-operation__icon" transform="translate(36 24) scale(2)" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">${window.AsterIcons?.[icon] || ""}</g>
        <text class="cmp-operation__label" x="56" y="105">${title}</text>
        <text class="cmp-operation__kind" x="56" y="120">${ctx.esc(meta)}</text>
      </g>`;
    }
  });
  window.AsterUI.register("workflow@1", {
    renderCompact(instance, ctx) {
      const visual = instance.visual || {};
      const icon = ["plate", "pipette", "incubator", "reader", "flask"].includes(visual.symbol || visual.template)
        ? (visual.symbol || visual.template)
        : "flask";
      const title = ctx.esc(ctx.title(instance));
      const kind = ctx.esc(visual.symbol || visual.template || instance.definition?.id || "workflow");
      const markup = window.AsterIcons?.[icon] || "";
      return `<g class="cmp-flow">
        <g class="cmp-flow__icon" transform="translate(32 12) scale(2)">${markup}</g>
        <text class="cmp-flow__label" x="56" y="105">${title}</text>
        <text class="cmp-flow__kind" x="56" y="120">${kind}</text>
      </g>`;
    }
  });
})();
