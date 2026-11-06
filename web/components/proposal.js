(() => {
  const esc = (ctx, value) => ctx.esc(value == null ? "" : String(value));
  const recordsFor = ctx => ctx.inbox || ctx.proposals || [];
  const proposalFor = ctx => ctx.activeProposal || ctx.proposal || null;
  const idFor = record => {
    const data = record.data || record;
    return String(data.proposal_id || record.id || "");
  };
  const operationsFor = preview => preview?.operation_changes || [];
  const noOpFor = changes => changes.length > 0 && changes.every(op => JSON.stringify(op.before) === JSON.stringify(op.after));

  function changesMarkup(ctx, changes, viewOnly) {
    const selected = changes.filter(op => (op.scope === "view_only" || op.view_only === true || op.kind === "move_slot_child") === viewOnly);
    if (!selected.length) return "";
    return `<section class="cmp-proposal-diff-group"><h4>${viewOnly ? "仅视图变更" : "实验内容变更"}</h4>${selected.map(op => {
      let label = op.kind || "变更", detail = "";
      if (op.kind === "move_slot_child") {
        label = "组件顺序";
        detail = `${JSON.stringify(op.before ?? null)} → ${JSON.stringify(op.after ?? null)}`;
      } else if (op.kind === "add_slot_child") {
        label = "新增组件";
        detail = `${op.after?.definition?.id || "组件"} · ${op.slot_id || ""}`;
      } else if (op.kind === "update_props") {
        label = op.property || "实验参数";
        detail = `${JSON.stringify(op.before ?? null)} → ${JSON.stringify(op.after ?? null)}`;
      } else {
        detail = JSON.stringify(op.edge || op.binding_id || op.instance_id || op.slot_id || "");
      }
      return `<div class="cmp-proposal-diff"><b>${esc(ctx, label)}</b><span>${esc(ctx, detail)}</span></div>`;
    }).join("")}</section>`;
  }

  function reviewMarkup(ctx) {
    const proposal = proposalFor(ctx);
    if (!proposal) return "";
    const data = proposal.data || proposal;
    const preview = data.preview || proposal.preview || {};
    const changes = operationsFor(preview);
    const status = data.status || "pending";
    const noOp = noOpFor(changes);
    const stale = proposal.stale === true || data.stale === true || status === "stale" || status === "expired";
    const invalid = preview.valid === false;
    const blocked = status !== "pending" || noOp || stale || invalid;
    const id = idFor(proposal);
    const state = stale ? "版本已过期，尚未处理" : noOp ? "无实际变化，不能接受" : invalid ? "预览校验失败，尚未处理" : status === "pending" ? "待审阅 · 尚未处理" : `已处理：${status}`;
    const diff = changesMarkup(ctx, changes, false) + changesMarkup(ctx, changes, true);
    const error = ctx.proposalError ? `<div class="cmp-proposal-error" role="alert">操作失败：${esc(ctx, ctx.proposalError)}</div>` : "";
    return `<aside class="cmp-proposal-panel" role="dialog" aria-modal="false" aria-label="Proposal Review"><header class="cmp-proposal-review-head"><div><h3>Proposal Review</h3><div class="cmp-proposal-meta">${esc(ctx, data.source || "External Agent")} · revision ${esc(ctx, data.expected_revision ?? "?")} · ${esc(ctx, state)}</div></div><button type="button" aria-label="关闭审阅" data-proposal-close>关闭</button></header>${error}${stale ? '<div class="cmp-proposal-state is-warning" role="status">提案与当前版本不一致；未应用任何变更。</div>' : ""}${noOp ? '<div class="cmp-proposal-state" role="status">预览中的所有操作均无净变化；提案未应用。</div>' : ""}${invalid ? '<div class="cmp-proposal-state is-error" role="alert">预览未通过校验；提案未应用。</div>' : ""}<div class="cmp-proposal-diffs">${diff || '<div class="cmp-proposal-empty">无结构化差异</div>'}</div><div class="cmp-proposal-actions"><button type="button" data-proposal-reject="${esc(ctx, id)}" ${status !== "pending" ? "disabled" : ""}>拒绝提案</button><button type="button" class="cmp-proposal-primary" data-proposal-accept="${esc(ctx, id)}" ${blocked ? "disabled" : ""}>接受变更</button></div></aside>`;
  }

  function inboxMarkup(ctx) {
    const records = recordsFor(ctx).filter(record => (record.data || record).status === "pending");
    const active = proposalFor(ctx);
    const error = ctx.proposalError && !active ? `<div class="cmp-proposal-error" role="alert">${esc(ctx, ctx.proposalError)}</div>` : "";
    return `<div class="cmp-proposal-inbox"><header class="cmp-proposal-head"><div><h2>Proposal Inbox</h2><div class="cmp-proposal-meta">${records.length} 条待审阅 · 仅外部 Agent</div></div><button type="button" data-proposal-refresh>刷新</button></header>${error}${records.length ? `<div class="cmp-proposal-list">${records.map(record => {
      const data = record.data || record;
      const id = idFor(record);
      const selected = active && idFor(active) === id;
      const ops = (data.changeset?.operations || []).map(op => op.kind).filter(Boolean).join("、");
      return `<article class="cmp-proposal-row ${selected ? "is-active" : ""}"><div class="cmp-proposal-row-copy"><b>${esc(ctx, data.source || "External Agent")}</b><span>revision ${esc(ctx, data.expected_revision ?? "?")}${ops ? ` · ${esc(ctx, ops)}` : ""} · 待审阅</span></div><button type="button" data-proposal-open="${esc(ctx, id)}">${selected ? "正在审阅" : "审阅"}</button></article>`;
    }).join("")}</div>` : '<div class="cmp-proposal-empty">没有待审阅的外部 Agent 提案。</div>'}${reviewMarkup(ctx)}</div>`;
  }

  function renderInbox(container, ctx) {
    container.innerHTML = inboxMarkup(ctx);
    return container;
  }

  function renderDetail(instance, ctx) {
    return `<article class="card cmp-proposal" data-instance="${esc(ctx, instance.instance_id)}" data-component="view.proposal"><header class="cmp-proposal-head"><div><h2>${esc(ctx, ctx.title(instance) || "Proposal Inbox")}</h2><div class="cmp-proposal-meta">提案收件箱与变更审阅</div></div></header><div data-proposal-content>${inboxMarkup(ctx)}</div></article>`;
  }

  function renderCompact(instance, ctx) {
    const count = recordsFor(ctx).filter(record => (record.data || record).status === "pending").length;
    return `<button type="button" class="cmp-proposal-entry" data-proposal-toggle aria-expanded="false">待审 ${count}</button>`;
  }

  function mount(root, instance, ctx) {
    let alive = true;
    let busy = false;
    let selected = undefined;
    let errorMessage = "";
    const view = () => ({ ...ctx, activeProposal: selected === undefined ? proposalFor(ctx) : selected, proposalError: errorMessage });
    const render = () => {
      if (alive) renderInbox(root.querySelector("[data-proposal-content]") || root, view());
    };
    const click = async event => {
      const button = event.target.closest("button[data-proposal-refresh],button[data-proposal-open],button[data-proposal-accept],button[data-proposal-reject],button[data-proposal-close]");
      if (!button || !root.contains(button) || busy) return;
      if (button.hasAttribute("data-proposal-close")) {
        selected = null;
        errorMessage = "";
        render();
        return;
      }
      busy = true;
      root.setAttribute("aria-busy", "true");
      errorMessage = "";
      try {
        if (button.hasAttribute("data-proposal-refresh")) await ctx.loadInbox();
        else if (button.hasAttribute("data-proposal-open")) {
          const record = recordsFor(ctx).find(item => idFor(item) === button.dataset.proposalOpen);
          if (record) {
            await ctx.openProposal(record);
            selected = proposalFor(ctx) || record;
          }
        } else if (button.hasAttribute("data-proposal-accept")) {
          await ctx.acceptProposal(button.dataset.proposalAccept);
          selected = null;
          await ctx.loadInbox();
        } else {
          await ctx.rejectProposal(button.dataset.proposalReject);
          selected = null;
          await ctx.loadInbox();
        }
      } catch (error) {
        errorMessage = error?.message || String(error);
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
  window.AsterUI.register("view.proposal@1", { renderDetail, renderCompact, mount });
  window.AsterUI.renderProposalInbox = renderInbox;
})();
