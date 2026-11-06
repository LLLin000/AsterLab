(() => {
  const definition = instance => instance.definition?.id || "content.note";

  function renderDetail(instance, ctx) {
    const id = instance.instance_id;
    const draft = ctx.drafts.get(id);
    const text = draft?.text ?? instance.props?.text ?? "";
    const markdown = definition(instance) === "content.markdown";
    return `<article class="card cmp-markdown" data-instance="${ctx.esc(id)}" data-component="${ctx.esc(definition(instance))}">
      <header class="cmp-markdown__header"><div><h3>${ctx.esc(ctx.title(instance))}</h3><div class="cmp-markdown__meta">${markdown ? "Markdown" : "说明"}${draft ? " · 未保存" : ""}</div></div></header>
      <textarea data-markdown-editor aria-label="${markdown ? "Markdown 内容" : "实验说明"}" placeholder="写下实验说明、假设或注意事项">${ctx.esc(text)}</textarea>
      <div class="cmp-markdown__preview" data-markdown-preview hidden></div>
      <div class="cmp-markdown__actions"><button type="button" data-markdown-toggle aria-pressed="false">预览</button><button type="button" data-markdown-save>保存</button><span data-markdown-status data-state="saved" role="status" aria-live="polite"></span></div>
    </article>`;
  }

  function mount(root, instance, ctx) {
    const editor = root.querySelector("[data-markdown-editor]");
    const preview = root.querySelector("[data-markdown-preview]");
    const toggle = root.querySelector("[data-markdown-toggle]");
    const save = root.querySelector("[data-markdown-save]");
    const status = root.querySelector("[data-markdown-status]");
    const controller = new AbortController();
    const signal = controller.signal;
    let showingPreview = false;
    let token = 0;
    editor.addEventListener("input", () => {
      token++;
      ctx.drafts.set(instance.instance_id, { text: editor.value, token });
      ctx.drafts.markDirty(instance.instance_id);
      status.dataset.state = "dirty";
      status.textContent = "未保存草稿";
      if (showingPreview) preview.innerHTML = renderMarkdown(editor.value, ctx);
    }, { signal });
    toggle.addEventListener("click", () => {
      showingPreview = !showingPreview;
      preview.hidden = !showingPreview;
      editor.hidden = showingPreview;
      toggle.textContent = showingPreview ? "编辑" : "预览";
      toggle.setAttribute("aria-pressed", String(showingPreview));
      if (showingPreview) preview.innerHTML = renderMarkdown(editor.value, ctx);
    }, { signal });
    save.addEventListener("click", async () => {
      const savedToken = token;
      status.dataset.state = "saving";
      status.textContent = "正在保存…";
      save.disabled = true;
      try {
        await ctx.commit([{ kind: "update_props", instance_id: instance.instance_id, property: "text", value: editor.value }], "保存说明");
        if (savedToken === token) {
          ctx.drafts.clear(instance.instance_id);
          status.dataset.state = "saved";
          status.textContent = "已保存";
        } else { status.dataset.state = "dirty"; status.textContent = "有新修改尚未保存"; }
      } catch (error) {
        status.dataset.state = "error";
        status.textContent = `保存失败：${error.message || error}`;
      } finally {
        if (!signal.aborted) save.disabled = false;
      }
    }, { signal });
    return () => controller.abort();
  }

  function renderMarkdown(source, ctx) {
    if (window.marked && window.DOMPurify) {
      try { return window.DOMPurify.sanitize(window.marked.parse(source, { gfm: true, breaks: true })); }
      catch (_) {}
    }
    return `<pre>${ctx.esc(source)}</pre>`;
  }

  for (const id of ["content.note", "content.markdown"]) {
    window.AsterUI.register(`${id}@1`, { renderDetail, mount });
  }
})();
