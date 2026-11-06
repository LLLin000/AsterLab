(() => {
  const esc = (ctx, value) => ctx.esc(value == null ? "" : String(value));
  const recordsFor = ctx => ctx.inbox || ctx.proposals || [];
  const proposalFor = ctx => ctx.activeProposal || ctx.proposal || null;

  function changesMarkup(ctx, preview) {
    const changes = preview?.operation_changes || [];
    if (!changes.length) return '<div class="cmp-proposal-empty">无结构化差异</div>';
    return changes.map(op => {
      let label, detail;
      if (op.kind === "move_slot_child") {
        label = "位置";
        detail = `${JSON.stringify(op.before ?? null)} → ${JSON.stringify(op.after ?? null)}`;
      } else if (op.kind === "add_slot_child") {
        label = "新增";
        detail = `${op.after?.definition?.id || "组件"} · ${op.slot_id || ""}`;
      } else if (op.kind === "update_props") {
        label = "字段";
        detail = `${op.property || ""}: ${JSON.stringify(op.before ?? null)} → ${JSON.stringify(op.after ?? null)}`;
      } else {
        label = op.kind || "变更";
        detail = JSON.stringify(op.edge || op.binding_id || op.instance_id || "");
      }
      return `<div class="cmp-proposal-diff"><b>${esc(ctx, label)}</b><span>${esc(ctx, detail)}</span></div>`;
    }).join("");
  }

  function reviewMarkup(ctx) {
    const proposal = proposalFor(ctx);
    if (!proposal) return "";
    const data = proposal.data || proposal;
    const status = data.status || "pending";
    const preview = data.preview || proposal.preview || {};
    const changes = preview.operation_changes || [];
    const noOp = changes.length > 0 && changes.every(op => JSON.stringify(op.before) === JSON.stringify(op.after));
    const blocked = status !== "pending" || noOp || preview.valid === false || proposal.stale;
    const id = data.proposal_id || proposal.id || "";
    const state = noOp ? "无实际变化" : proposal.stale ? "版本已过期" : status === "pending" ? "待审阅" : `状态：${status}`;
    return `<section class="cmp-proposal-review" aria-label="提案审阅"><div class="cmp-proposal-review-head"><div><h3>Proposal Review</h3><div class="cmp-proposal-meta">${esc(ctx, data.source || "external")} · revision ${esc(ctx, data.expected_revision ?? "?")} · ${esc(ctx, state)}</div></div></div><div class="cmp-proposal-diffs">${changesMarkup(ctx, preview)}</div><div class="cmp-proposal-actions"><button type="button" data-proposal-reject="${esc(ctx, id)}" ${status !== "pending" ? "disabled" : ""}>拒绝</button><button type="button" class="cmp-proposal-primary" data-proposal-accept="${esc(ctx, id)}" ${blocked ? "disabled" : ""}>接受变更</button></div></section>`;
  }

  function inboxMarkup(ctx) {
    const records = recordsFor(ctx).filter(record => (record.data || record).status === "pending");
    const active = proposalFor(ctx);
    return `<div class="cmp-proposal-inbox"><header class="cmp-proposal-head"><div><h2>Proposal Inbox</h2><div class="cmp-proposal-meta">${records.length} 条待审阅</div></div><button type="button" data-proposal-refresh>刷新</button></header>${ctx.proposalError ? `<div class="cmp-proposal-error" role="alert">${esc(ctx, ctx.proposalError)}</div>` : ""}${records.length ? `<div class="cmp-proposal-list">${records.map(record => {
      const data = record.data || record;
      const id = data.proposal_id || record.id || "";
      const selected = active && String((active.data || active).proposal_id || active.id) === String(id);
      const ops = (data.changeset?.operations || []).map(op => op.kind).filter(Boolean).join("、");
      return `<article class="cmp-proposal-row ${selected ? "is-active" : ""}"><div class="cmp-proposal-row-copy"><b>${esc(ctx, data.source || "external")}</b><span>${esc(ctx, `revision ${data.expected_revision ?? "?"}${ops ? ` · ${ops}` : ""}`)}</span></div><button type="button" data-proposal-open="${esc(ctx, id)}">${selected ? "已选中" : "审阅"}</button></article>`;
    }).join("")}</div>` : '<div class="cmp-proposal-empty">没有待审阅的提案。</div>'}${reviewMarkup(ctx)}</div>`;
  }

  function renderInbox(container, ctx) {
    container.innerHTML = inboxMarkup(ctx);
    return container;
  }

  function renderDetail(instance, ctx) {
    return `<article class="card cmp-proposal" data-instance="${esc(ctx, instance.instance_id)}" data-component="view.proposal"><header class="cmp-proposal-head"><div><h2>${esc(ctx, ctx.title(instance) || "Proposal Inbox")}</h2><div class="cmp-proposal-meta">提案收件箱与变更审阅</div></div></header><div data-proposal-content>${inboxMarkup(ctx)}</div></article>`;
  }

  function mount(root, instance, ctx) {
    let alive = true;
    let busy = false;
    const render = () => {
      if (alive) renderInbox(root.querySelector("[data-proposal-content]") || root, ctx);
    };
    const click = async event => {
      const button = event.target.closest("button[data-proposal-refresh],button[data-proposal-open],button[data-proposal-accept],button[data-proposal-reject]");
      if (!button || !root.contains(button) || busy) return;
      busy = true;
      root.setAttribute("aria-busy", "true");
      ctx.proposalError = "";
      try {
        if (button.hasAttribute("data-proposal-refresh")) {
          await ctx.loadInbox();
        } else if (button.hasAttribute("data-proposal-open")) {
          const id = button.dataset.proposalOpen;
          const record = recordsFor(ctx).find(item => String((item.data || item).proposal_id || item.id) === id);
          if (record) await ctx.openProposal(record);
        } else if (button.hasAttribute("data-proposal-accept")) {
          await ctx.acceptProposal(button.dataset.proposalAccept);
          await ctx.loadInbox();
        } else {
          await ctx.rejectProposal(button.dataset.proposalReject);
          await ctx.loadInbox();
        }
      } catch (error) {
        ctx.proposalError = error?.message || String(error);
      } finally {
        busy = false;
        root.removeAttribute("aria-busy");
        render();
      }
    };
    root.addEventListener("click", click);
    return () => { alive = false; root.removeEventListener("click", click); };
  }

  window.AsterUI = window.AsterUI || { registry: {}, register(key, renderer) { this.registry[key] = renderer; } };
  window.AsterUI.register("view.proposal@1", { renderDetail, mount });
  window.AsterUI.renderProposalInbox = renderInbox;
})();
