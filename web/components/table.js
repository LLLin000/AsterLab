(() => {
  const escape = (value, ctx) => ctx?.esc ? ctx.esc(value) : String(value ?? "").replace(/[&<>"']/g, char => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", "\"": "&quot;", "'": "&#39;" })[char]);
  const definitionId = instance => instance?.definition?.id || instance?.definition_id || "content.table";
  const columnsFor = instance => {
    const columns = instance?.props?.columns;
    if (Array.isArray(columns) && columns.length) return columns.map(value => String(value ?? ""));
    const rows = Array.isArray(instance?.props?.rows) ? instance.props.rows : [];
    return Array.from({ length: Math.max(1, ...rows.map(row => Array.isArray(row) ? row.length : 0)) }, (_, index) => `列 ${index + 1}`);
  };
  const rowsFor = instance => (Array.isArray(instance?.props?.rows) ? instance.props.rows : []).map(row => Array.isArray(row) ? [...row] : []);

  function renderDetail(instance, ctx) {
    const id = escape(instance?.instance_id, ctx);
    const definition = definitionId(instance);
    const columns = columnsFor(instance);
    const rows = rowsFor(instance);
    const title = escape(ctx?.title ? ctx.title(instance) : instance?.props?.label || instance?.props?.title || definition, ctx);
    return `<article class="card cmp-table" data-instance="${id}" data-component="${escape(definition, ctx)}">
      <header class="cmp-table__header"><div><h3>${title}</h3><div class="cmp-table__meta" data-table-meta>${escape(definition, ctx)} · ${rows.length} 行 · ${columns.length} 列</div></div><span class="cmp-table__status" data-table-status role="status" aria-live="polite">已保存</span></header>
      <div class="cmp-table__scroll"><table><thead><tr>${columns.map((column, index) => `<th scope="col"><label class="cmp-table__sr" for="${id}-head-${index}">第 ${index + 1} 列名称</label><input id="${id}-head-${index}" data-colhead="${index}" value="${escape(column, ctx)}" aria-label="第 ${index + 1} 列名称"></th>`).join("")}</tr></thead><tbody>${rows.map((row, rowIndex) => `<tr>${columns.map((_, columnIndex) => `<td><label class="cmp-table__sr" for="${id}-cell-${rowIndex}-${columnIndex}">第 ${rowIndex + 1} 行，第 ${columnIndex + 1} 列</label><input id="${id}-cell-${rowIndex}-${columnIndex}" data-cell="${rowIndex}-${columnIndex}" value="${escape(row[columnIndex] ?? "", ctx)}" aria-label="第 ${rowIndex + 1} 行第 ${columnIndex + 1} 列"></td>`).join("")}</tr>`).join("")}</tbody></table>${rows.length ? "" : '<div class="cmp-table__empty">暂无数据行。选择“添加行”开始录入。</div>'}</div>
      <footer class="cmp-table__actions"><button type="button" data-table-add-col>添加列</button><button type="button" data-table-remove-col aria-label="删除最后一列"${columns.length <= 1 ? " disabled" : ""}>删除末列</button><button type="button" data-table-add-row>添加行</button><button type="button" data-table-remove-row${rows.length ? "" : " disabled"}>删除末行</button><button type="button" class="cmp-table__save" data-table-save>保存表格</button></footer>
    </article>`;
  }

  function mount(root, instance, ctx) {
    let columns = columnsFor(instance);
    let rows = rowsFor(instance);
    let dirty = false;
    let error = "";
    const status = () => {
      const output = root.querySelector("[data-table-status]");
      if (output) output.textContent = error || (dirty ? "未保存的草稿" : "已保存");
      root.classList.toggle("cmp-table--dirty", dirty);
      root.classList.toggle("cmp-table--error", Boolean(error));
    };
    const rebuild = () => {
      const head = root.querySelector("thead tr");
      const body = root.querySelector("tbody");
      if (!head || !body) return;
      head.innerHTML = columns.map((column, index) => `<th scope="col"><label class="cmp-table__sr" for="${escape(instance.instance_id, ctx)}-head-${index}">第 ${index + 1} 列名称</label><input id="${escape(instance.instance_id, ctx)}-head-${index}" data-colhead="${index}" value="${escape(column, ctx)}" aria-label="第 ${index + 1} 列名称"></th>`).join("");
      body.innerHTML = rows.map((row, rowIndex) => `<tr>${columns.map((_, columnIndex) => `<td><label class="cmp-table__sr" for="${escape(instance.instance_id, ctx)}-cell-${rowIndex}-${columnIndex}">第 ${rowIndex + 1} 行，第 ${columnIndex + 1} 列</label><input id="${escape(instance.instance_id, ctx)}-cell-${rowIndex}-${columnIndex}" data-cell="${rowIndex}-${columnIndex}" value="${escape(row[columnIndex] ?? "", ctx)}" aria-label="第 ${rowIndex + 1} 行第 ${columnIndex + 1} 列"></td>`).join("")}</tr>`).join("");
      let empty = root.querySelector(".cmp-table__empty");
      if (!rows.length) {
        if (!empty) { empty = document.createElement("div"); empty.className = "cmp-table__empty"; root.querySelector(".cmp-table__scroll").append(empty); }
        empty.textContent = "暂无数据行。选择“添加行”开始录入。";
      } else empty?.remove();
      root.querySelector("[data-table-meta]").textContent = `${definitionId(instance)} · ${rows.length} 行 · ${columns.length} 列`;
      root.querySelector("[data-table-remove-col]").disabled = columns.length <= 1;
      root.querySelector("[data-table-remove-row]").disabled = rows.length === 0;
    };
    const onInput = event => {
      const target = event.target;
      if (target.matches("[data-colhead]")) columns[Number(target.dataset.colhead)] = target.value;
      else if (target.matches("[data-cell]")) { const [row, column] = target.dataset.cell.split("-").map(Number); rows[row][column] = target.value; }
      else return;
      dirty = true; error = ""; status();
    };
    const onClick = async event => {
      const button = event.target.closest("button");
      if (!button || !root.contains(button)) return;
      if (button.hasAttribute("data-table-add-col")) {
        columns.push(`列 ${columns.length + 1}`); rows.forEach(row => row.push("")); dirty = true; error = ""; rebuild(); status();
      } else if (button.hasAttribute("data-table-remove-col") && columns.length > 1) {
        columns.pop(); rows.forEach(row => row.pop()); dirty = true; error = ""; rebuild(); status();
      } else if (button.hasAttribute("data-table-add-row")) {
        rows.push(Array(columns.length).fill("")); dirty = true; error = ""; rebuild(); status();
      } else if (button.hasAttribute("data-table-remove-row") && rows.length) {
        rows.pop(); dirty = true; error = ""; rebuild(); status();
      } else if (button.hasAttribute("data-table-save")) {
        if (!dirty) return;
        if (typeof ctx?.commit !== "function") { error = "保存失败：当前不可提交"; status(); return; }
        button.disabled = true;
        const operations = [{ kind: "update_props", instance_id: instance.instance_id, property: "rows", value: rows.map(row => [...row]) }];
        if (definitionId(instance) === "content.table") operations.unshift({ kind: "update_props", instance_id: instance.instance_id, property: "columns", value: [...columns] });
        try { await ctx.commit(operations, "保存表格"); dirty = false; error = ""; status(); }
        catch (failure) { error = `保存失败：${failure?.message || "请重试"}`; status(); }
        finally { button.disabled = false; }
      }
    };
    root.addEventListener("input", onInput);
    root.addEventListener("click", onClick);
    return () => { root.removeEventListener("input", onInput); root.removeEventListener("click", onClick); };
  }

  for (const key of ["content.table@1", "layout.table@1"]) window.AsterUI?.register(key, { renderDetail, mount });
})();
