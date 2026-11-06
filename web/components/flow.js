(() => {
  window.AsterUI.register("workflow@1", {
    renderCompact(instance, ctx) {
      const visual = instance.visual || {};
      const icon = ["plate", "pipette", "incubator", "reader", "flask"].includes(visual.symbol || visual.template)
        ? (visual.symbol || visual.template)
        : "flask";
      const title = ctx.esc(ctx.title(instance));
      const kind = ctx.esc(visual.symbol || visual.template || instance.definition?.id || "workflow");
      return `<g class="cmp-flow">
        <image class="cmp-flow__icon" href="/icons/${icon}.svg" xlink:href="/icons/${icon}.svg" x="40" y="7" width="60" height="60" preserveAspectRatio="xMidYMid meet"/>
        <text class="cmp-flow__label" x="70" y="101">${title}</text>
        <text class="cmp-flow__kind" x="70" y="116">${kind}</text>
      </g>`;
    }
  });
})();
