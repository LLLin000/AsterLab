(() => {
  window.AsterUI.register("operation@1", {
    renderCompact(instance, ctx) {
      const visual = instance.visual || {};
      const icon = ["plate", "pipette", "incubator", "reader", "flask"].includes(visual.symbol || visual.template)
        ? (visual.symbol || visual.template)
        : "flask";
      const title = ctx.esc(ctx.title(instance));
      const detail = Object.values(instance.slots || {}).flat();
      const table = preset => detail.find(child => child.definition?.id === "content.table" && child.props?.preset === preset);
      const count = preset => table(preset)?.props?.rows?.length || 0;
      const materials = count("materials"), instruments = count("instruments");
      const meta = materials || instruments ? `${materials} 材料 · ${instruments} 仪器` : (visual.symbol || visual.template || instance.definition?.id || "operation");
      return `<g class="cmp-operation">
        <g class="cmp-operation__icon" transform="translate(32 12) scale(2)">${window.AsterIcons?.[icon] || ""}</g>
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
