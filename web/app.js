
let viewId="", data=null, node=null, selected=null, stack=[], mode="select";
let view={x:0,y:0,k:1}, pan=null, dragNode=null, nodeMoved=false;
let history=[], future=[], saveChain=Promise.resolve();
let paramCache=new Map(), plateCache=new Map(), plateSelections=new Map();
let calcStates=new Map();
let selectedChildId=null;
let proposal=null;
let dataRequestSeq=0;
let drafts=new Map();
const $=id=>document.getElementById(id);
const esc=s=>String(s??"").replace(/[&<>"']/g,c=>({"&":"&amp;","<":"&lt;",">":"&gt;","\"":"&quot;","'":"&#39;"}[c]));
const defName=c=>c?.definition?.id||"component";
const nodeTitle=c=>c?.props?.label||c?.props?.title||defName(c);
const scopeLabel=c=>c?.model_ref?.step?.scope?.assay_id?`assay: ${c.model_ref.step.scope.assay_id}`:"";
const workflowOf=c=>c?.workflow?.nodes||[];
const slotsOf=c=>c?.slots||{};
const findById=(root,id)=>{let hit=null;const walk=c=>{if(c.instance_id===id)hit=c;Object.values(c.slots||{}).flat().forEach(walk);(c.workflow?.nodes||[]).forEach(n=>walk(n.component))};walk(root);return hit};
let masonry=null;
function layoutCards(){
  const container=document.querySelector(".cards");if(!container)return;
  if(!window.Masonry){container.classList.add("no-masonry");return}
  if(masonry&&masonry.element!==container){masonry.destroy();masonry=null}
  if(!masonry)masonry=new Masonry(container,{itemSelector:".card",columnWidth:".grid-sizer",gutter:8,percentPosition:true,transitionDuration:0});
  else{masonry.reloadItems();masonry.layout()}
}

function iconFor(c){
  const visual=c?.visual||{};
  const explicit=(visual.symbol||visual.template||"").toLowerCase();
  const allowed=["plate","pipette","incubator","reader","flask"];
  let key=allowed.includes(explicit)?explicit:"flask";
  const def=defName(c);
  if(!allowed.includes(explicit)&&def==="lab.well_plate")key="plate";
  return `<image class="bioicon" href="/icons/${key}.svg" x="8" y="7" width="60" height="60" preserveAspectRatio="xMidYMid meet"/>`;
}
function renderCrumbs(){
  $("crumbs").innerHTML=stack.map((c,i)=>`<button class="crumb" data-i="${i}">${esc(nodeTitle(c))}</button>`).join('<span class="crumb-sep">›</span>');
  $("crumbs").querySelectorAll("button").forEach(b=>b.onclick=()=>{stack=stack.slice(0,Number(b.dataset.i)+1);node=stack[stack.length-1];selected=workflowOf(node)[0]?.component||node;render()});
}
function graph(){
  if(!node||!node.workflow)return {nodes:[],edges:[]};
  const nodes=node.workflow.nodes||[];
  const edges=node.workflow.edges||[];
  const positions=window.AsterLayout?AsterLayout.layout(nodes,edges,window.dagre):{};
  const gap=190,start=550-((nodes.length-1)*gap)/2-38,y=150;
  nodes.forEach((n,i)=>{
    const c=n.component,p=positions[n.node_id];
    if(p){c._vx=p.x;c._vy=p.y}
    else{c._vx=start+i*gap;c._vy=y}
  });
  return {nodes,edges};
}
function nodeBox(c){return {x:c._vx??80,y:c._vy??175,w:76,h:76}}
function compactMarkup(c){
  return `<g class="glyph">${iconFor(c)}</g><text class="label" x="38" y="101">${esc(nodeTitle(c))}</text><text class="kind" x="38" y="116">${esc((c.visual?.symbol||c.visual?.template||"").trim()||defName(c))}</text>`;
}
function renderCanvas(){
  const svg=$("canvas"); const {nodes,edges}=graph();
  const byId={}; nodes.forEach(n=>{byId[n.node_id]=n.component;byId[n.component.instance_id]=n.component});
  const edgeMarkup=edges.map(e=>{const a=byId[e.from],b=byId[e.to];if(!a||!b)return "";const A=nodeBox(a),B=nodeBox(b);const x1=A.x+76,y1=A.y+38,x2=B.x,y2=B.y+38;const mid=x1+(x2-x1)/2;return `<path class="edge" d="M${x1} ${y1} C${mid} ${y1},${mid} ${y2},${x2-5} ${y2}"/>`}).join("");
  const nodeMarkup=nodes.map(n=>{const c=n.component,B=nodeBox(c);const active=selected?.instance_id===c.instance_id;const hint=(c.visual?.symbol||c.visual?.template||"").trim();return `<g class="wnode ${active?"selected":""}" data-node="${esc(c.instance_id)}" transform="translate(${B.x} ${B.y})" tabindex="0" role="button" aria-label="${esc(nodeTitle(c))}"><rect class="node-bg" width="76" height="76" rx="18"/><circle class="port-hit" data-port="in" data-node="${esc(n.node_id)}" cx="0" cy="38" r="8" fill="transparent"/><circle class="port" cx="0" cy="38" r="3" pointer-events="none"/><circle class="port-hit" data-port="out" data-node="${esc(n.node_id)}" cx="76" cy="38" r="8" fill="transparent"/><circle class="port" cx="76" cy="38" r="3" pointer-events="none"/>${rendererFor(c).compact(c)}</g>`}).join("");
  const empty=nodes.length?"":`<text class="empty" x="550" y="260">此 View 还没有工作流节点。</text>`;
  const dots=Array.from({length:16},(_,r)=>Array.from({length:24},(_,c)=>`<circle class="grid-dot" cx="${40+c*44}" cy="${40+r*34}" r="1"/>`).join("")).join("");
  svg.innerHTML=`<defs><pattern id="dots" width="44" height="34" patternUnits="userSpaceOnUse">${dots}</pattern><marker id="arrow" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="5" markerHeight="5" orient="auto-start-reverse"><path d="M0 0L10 5L0 10z" fill="#b9c0ca"/></marker></defs><rect width="1100" height="560" fill="url(#dots)"/><g id="viewport" transform="translate(${view.x} ${view.y}) scale(${view.k})">${edgeMarkup}${nodeMarkup}${empty}</g>`;
  svg.querySelectorAll(".port-hit[data-port=out]").forEach(el=>el.addEventListener("pointerdown",ev=>startConnect(ev,el.dataset.node)));
  svg.querySelectorAll(".wnode").forEach(el=>{
    el.addEventListener("pointerdown",ev=>startNodeDrag(ev,byId[el.dataset.node]));
    el.addEventListener("click",ev=>{ev.stopPropagation();if(nodeMoved){nodeMoved=false;return}selectComponent(byId[el.dataset.node])});
    el.addEventListener("dblclick",ev=>{ev.stopPropagation();const c=byId[el.dataset.node];if(c?.workflow?.nodes?.length){enterComponent(c)}});
  });
  $("zoomLabel").textContent=`${Math.round(view.k*100)}%`;
  updateHistoryButtons();
}
function updateHistoryButtons(){$("undo").disabled=history.length===0;$("redo").disabled=future.length===0}
function selectComponent(c){if(selected?.instance_id!==c.instance_id){selectedChildId=null;proposal=null}selected=c;renderCanvas();renderDetail()}
function enterComponent(c){node=c;stack.push(c);selected=workflowOf(c)[0]?.component||c;render()}
function fmtValue(value){if(value===null||value===undefined)return "待确认";if(typeof value==="object")return JSON.stringify(value);return String(value)}
async function loadParams(c,boxId){
  const ref=c?.model_ref||{};
  if(ref.kind!=="operation_use")return;
  const key=`${viewId}|${c.instance_id}`;
  let result=paramCache.get(key);
  if(!result){
    try{
      const r=await fetch(`/api/views/${encodeURIComponent(viewId)}/steps/${encodeURIComponent(c.instance_id)}`);
      const j=await r.json(); if(!r.ok)throw new Error(j.error||"参数读取失败"); result=j;
    }catch(e){result={error:String(e.message||e)}} paramCache.set(key,result)
  }
  if(selected?.instance_id!==c.instance_id)return;
  const box=$(boxId);if(!box)return;
  if(result.error){box.innerHTML=`<div class="muted">${esc(result.error)}</div>`;return}
  const step=result.step||{};
  const params=step.parameters||[];
  const unresolved=step.unresolved_parameter_refs||[];
  const rows=params.map(p=>{
    const label=p.label||p.key||p.slot||"参数";const unit=p.unit?` ${p.unit}`:"";
    const source=p.value_source||p.source||"";
    return `<div class="param"><b>${esc(label)}</b><span>${esc(fmtValue(p.value))}${esc(unit)}${source?" · "+esc(source):""}</span></div>`
  }).join("");
  const pending=unresolved.map(key=>{const item=(result.unresolved_parameters||[]).find(p=>p.slot===key||p.label===key)||{};return `<div class="param"><b>${esc(item.label||key)}</b><span>待确认${item.unit?" · "+esc(item.unit):""}</span></div>`}).join("");
  box.innerHTML=rows+pending||`<div class="muted">该步骤没有已解析参数。</div>`;
  layoutCards();
}
function fmtSelection(plate,instanceId){
  const assigned=new Map((plate.positions||[]).map(p=>[p.position_id,p]));
  const selected=plateSelections.get(instanceId)||new Set();
  const lines=[...selected].sort().map(id=>{const hit=assigned.get(id);const target=hit?.targets?.[0]?.target?.id||"未关联目标";return `${esc(id)} · ${esc(target)}`});
  return lines.join("<br>")||`<span class="muted">尚未选择孔位。选择不是 DesignVersion 修改。</span>`;
}
function plateMarkup(plate,instanceId){
  const assigned=new Map((plate.positions||[]).map(p=>[p.position_id,p]));
  const selected=plateSelections.get(instanceId)||new Set();
  const rowCount=Number(plate.rows), colCount=Number(plate.columns);
  if(!rowCount||!colCount)return `<div class="muted">Profile 几何未解析：${esc(plate.profile_id||"unknown")}</div>`;
  const rows=Array.from({length:rowCount},(_,i)=>String.fromCharCode(65+i));
  let html="";
  for(const row of rows)for(let col=1;col<=colCount;col++){
    const id=`${row}${col}`; const hit=assigned.get(id); const on=selected.has(id);
    const target=hit?.targets?.[0]?.target?.id||"";
    html+=`<button class="well ${hit?"assigned":""} ${on?"selected":""}" data-well="${id}" data-plate="${esc(instanceId)}" ${hit?"":"disabled"} title="${id}${hit?" · "+esc(target):" · 未分配"}">${id}</button>`
  }
  return `<div class="plate" style="grid-template-columns:repeat(${colCount},17px)">${html}</div><div class="muted">已选 ${selected.size} / 计划 ${assigned.size} 孔（选择不修改 DesignVersion）</div><div class="selection-detail">${fmtSelection(plate,instanceId)}</div>`;
}
function bindPlateBox(box,plate,instanceId){
  box.querySelectorAll(".well").forEach(btn=>btn.onclick=()=>toggleWell(instanceId,btn.dataset.well,plate,box.id));
}
function calcStateFor(instanceId){
  if(!calcStates.has(instanceId))calcStates.set(instanceId,{plateId:"",positions:"",volume:"",unit:"",extra:""});
  return calcStates.get(instanceId);
}
function syncCalcPositions(plateId){
  calcStates.forEach((state,id)=>{
    if(plateId&&state.plateId!==plateId)return;
    const input=$(`calc-${id}-positions`);if(input)input.value=state.positions;
  });
}
function tableDraft(child){
  const id=child.instance_id;
  if(!drafts.has(id))drafts.set(id,{columns:tableColumns(child),rows:(Array.isArray(child.props?.rows)?child.props.rows:[]).map(row=>[...row]),token:0});
  return drafts.get(id);
}
function noteDraft(child){return drafts.get(child.instance_id)?.text??child.props?.text??""}
function markDirtyCard(id,dirty){
  const card=document.querySelector(`[data-instance="${CSS.escape(id)}"]`);if(!card)return;
  card.classList.toggle("is-dirty",dirty);
  const tag=card.querySelector(".tag");if(tag&&dirty&&!tag.textContent.includes("未保存"))tag.textContent+=" · 未保存";
}
async function loadPlate(instanceId,boxId){
  let plate=plateCache.get(instanceId);
  if(!plate){
    try{
      const r=await fetch(`/api/views/${encodeURIComponent(viewId)}/data?instance_id=${encodeURIComponent(instanceId)}`);
      const j=await r.json(); if(!r.ok)throw new Error(j.error||"孔位读取失败"); plate=(j.result??j).plate; plateCache.set(instanceId,plate);
    }catch(e){const box=$(boxId);if(box)box.innerHTML=`<div class="muted">${esc(e.message)}</div>`;return}
  }
  if(!plateSelections.has(instanceId))plateSelections.set(instanceId,new Set());
  const box=$(boxId);if(!box)return;box.innerHTML=plateMarkup(plate,instanceId);bindPlateBox(box,plate,instanceId);layoutCards();
}
function toggleWell(instanceId,well,plate,boxId){
  const set=plateSelections.get(instanceId)||new Set();if(set.has(well))set.delete(well);else set.add(well);plateSelections.set(instanceId,set);
  calcStates.forEach(state=>{if(state.plateId===instanceId)state.positions=[...set].sort().join(",")});syncCalcPositions(instanceId);
  const box=$(boxId);if(box){box.innerHTML=plateMarkup(plate,instanceId);bindPlateBox(box,plate,instanceId);layoutCards()}
}
function selectAllWells(instanceId,all){
  const plate=plateCache.get(instanceId);if(!plate)return;
  const set=all?new Set((plate.positions||[]).map(p=>p.position_id)):new Set();
  plateSelections.set(instanceId,set);
  calcStates.forEach(state=>{if(state.plateId===instanceId)state.positions=[...set].sort().join(",")});syncCalcPositions(instanceId);
  const box=$(`plate-body-${instanceId}`);if(box){box.innerHTML=plateMarkup(plate,instanceId);bindPlateBox(box,plate,instanceId);layoutCards()}
}
function renderMarkdown(text){
  const source=String(text||"");
  if(window.marked&&window.DOMPurify){
    try{return DOMPurify.sanitize(marked.parse(source,{gfm:true,breaks:true}))}catch(e){}
  }
  return `<pre class="md-fallback">${esc(source)}</pre>`;
}
function tableColumns(child){
  const cols=Array.isArray(child.props?.columns)?child.props.columns:[];
  if(cols.length)return cols;
  const rows=Array.isArray(child.props?.rows)?child.props.rows:[];
  const width=rows.reduce((n,row)=>Math.max(n,Array.isArray(row)?row.length:0),0);
  return Array.from({length:Math.max(1,width)},(_,i)=>`列 ${i+1}`);
}
function tableCard(child,actions){
  const draft=drafts.get(child.instance_id)||{columns:tableColumns(child),rows:(Array.isArray(child.props?.rows)?child.props.rows:[]).map(row=>[...row])};
  const cols=draft.columns;const rows=draft.rows;const dirty=drafts.has(child.instance_id);
  const head=cols.map((col,i)=>`<th><input data-colhead="${i}" value="${esc(col)}" aria-label="列名"></th>`).join("");
  const body=rows.map((row,r)=>`<tr data-row="${r}">${cols.map((_,c)=>`<td><input data-cell="${r}-${c}" value="${esc(row[c]??"")}" aria-label="第 ${r+1} 行第 ${c+1} 列"></td>`).join("")}</tr>`).join("");
  const grid=cols.length?`<table class="editable-table" data-table="${esc(child.instance_id)}"><thead><tr>${head}</tr></thead><tbody>${body}</tbody></table>`:`<div class="empty-card">尚无列。添加列后再填写数据。</div>`;
  return `<div class="card table-card ${dirty?"is-dirty":""}" data-instance="${esc(child.instance_id)}"><h3>${esc(nodeTitle(child))} ${actions}</h3><div class="tag">${esc(child.definition.id)} · 实验分组与数据${dirty?" · 未保存":""}</div>${grid}<div class="row table-actions"><button data-table-add-col="${esc(child.instance_id)}">+ 列</button><button data-table-add-row="${esc(child.instance_id)}">+ 行</button><button class="primary" data-table-save="${esc(child.instance_id)}">保存表格</button></div></div>`;
}
function noteCard(child,actions){
  const id=child.instance_id;const text=noteDraft(child);const isMd=child.definition.id==="content.markdown";const dirty=drafts.has(id);
  return `<div class="card note-card ${dirty?"is-dirty":""}" data-instance="${esc(id)}"><h3>${esc(nodeTitle(child))} ${actions}</h3><div class="tag">${isMd?"Markdown":"说明"} · 实验说明与记录${dirty?" · 未保存":""}</div><textarea id="note-${esc(id)}" placeholder="写下实验说明、假设或注意事项">${esc(text)}</textarea><div class="md-preview" id="md-preview-${esc(id)}" hidden></div><div class="row"><button data-md-toggle="${esc(id)}">预览</button><button data-save="${esc(id)}">保存</button></div></div>`;
}
function saveTable(instanceId){
  const draft=drafts.get(instanceId);if(!draft)return;const savedToken=draft.token||0;
  const child=findById(data.latest_version.data.root,instanceId);
  const rows=JSON.parse(JSON.stringify(draft.rows)),columns=[...draft.columns];
  const ops=[{kind:"update_props",instance_id:instanceId,property:"rows",value:rows}];
  if(child?.definition?.id==="content.table")ops.unshift({kind:"update_props",instance_id:instanceId,property:"columns",value:columns});
  mutate(ops,"保存表格").then(()=>{const next=drafts.get(instanceId);if(!next||(next.token||0)===savedToken){drafts.delete(instanceId);renderDetail()}}).catch(()=>{});
}
function tableAddColumn(instanceId){
  const child=findById(data.latest_version.data.root,instanceId);if(!child)return;
  const draft=tableDraft(child);draft.columns.push(`列 ${draft.columns.length+1}`);draft.rows.forEach(row=>row.push(""));renderDetail();
}
function tableAddRow(instanceId){
  const child=findById(data.latest_version.data.root,instanceId);if(!child)return;
  const draft=tableDraft(child);draft.rows.push(Array(draft.columns.length).fill(""));renderDetail();
}
function toggleMarkdown(instanceId){
  const textarea=$(`note-${CSS.escape(instanceId)}`);const preview=$(`md-preview-${CSS.escape(instanceId)}`);if(!textarea||!preview)return;
  const showing=!preview.hidden; preview.hidden=showing; textarea.hidden=!showing; preview.innerHTML=showing?"":renderMarkdown(textarea.value);
}
function plateCard(child,actions,ctx){
  return `<div class="card plate-card" data-instance="${esc(child.instance_id)}"><h3>${esc(nodeTitle(child))} ${actions}</h3><div class="tag">${esc(ctx.slot)} · ${esc(child.model_ref?.container_id||"")} · PlannedContainer</div><div id="plate-body-${esc(child.instance_id)}"><div class="muted">读取孔位…</div></div><div class="row"><button data-plate-all="${esc(child.instance_id)}">选择全部</button><button data-plate-clear="${esc(child.instance_id)}">清空</button><button data-plate-preview="${esc(child.instance_id)}">计算选中孔位</button></div></div>`;
}
function calculatorCard(child,actions,ctx){
  const id=child.instance_id,state=calcStateFor(id);
  const plates=ctx.plates||[];
  const options=plates.map(p=>`<option value="${esc(p.instance_id)}" ${state.plateId===p.instance_id?"selected":""}>${esc(nodeTitle(p))} · ${esc(p.model_ref?.container_id||"")}</option>`).join("");
  const bind=plates.length?`<select data-calc-plate="${esc(id)}"><option value="">选择孔板…</option>${options}</select>`:`<div class="muted">没有可用孔板；请先添加 PlannedContainer 孔板。</div>`;
  return `<div class="card calc-card" data-instance="${esc(id)}"><h3>${esc(nodeTitle(child))} ${actions}</h3><div class="tag">${esc(child.props?.tool_ref||defName(child))}</div><div class="stack">${bind}<input id="calc-${esc(id)}-positions" placeholder="目标孔位，如 A1,B1" value="${esc(state.positions)}"><div class="row"><input id="calc-${esc(id)}-volume" placeholder="每孔体积" value="${esc(state.volume)}"><input id="calc-${esc(id)}-unit" placeholder="单位，如 uL" value="${esc(state.unit)}"></div><input id="calc-${esc(id)}-extra" placeholder="额外比例，如 0.1" value="${esc(state.extra)}"></div><button data-calc="${esc(id)}">计算</button><div class="calc-result" id="calc-${esc(id)}-result"></div></div>`;
}
function workflowCard(child,actions){
  return `<div class="card" data-instance="${esc(child.instance_id)}"><h3>${esc(nodeTitle(child))} ${actions}</h3><div class="tag">嵌套流程 · ${workflowOf(child).length} 步</div><button data-enter="${esc(child.instance_id)}">进入</button></div>`;
}
function defaultCard(child,actions,ctx){
  return `<div class="card" data-instance="${esc(child.instance_id)}"><h3>${esc(nodeTitle(child))} ${actions}</h3><div class="tag">${esc(ctx.slot)}</div><div class="muted">${esc(child.definition.id)}</div></div>`;
}
function cardActions(child){
  return `<div class="card-actions"><button data-move-up="${esc(child.instance_id)}" title="上移">↑</button><button data-move-down="${esc(child.instance_id)}" title="下移">↓</button><button data-remove="${esc(child.instance_id)}" title="删除">×</button></div>`;
}
function unsupportedCard(child,actions,ctx){
  return `<div class="card" data-instance="${esc(child.instance_id)}"><h3>${esc(nodeTitle(child))} ${actions}</h3><div class="tag">Unsupported</div><div class="muted">没有可用的 Renderer：${esc(defName(child))}@${esc(child.definition?.version??"?")}</div></div>`;
}
const componentRenderers={
  "content.table@1":{compact:compactMarkup,detail:tableCard},
  "layout.table@1":{compact:compactMarkup,detail:tableCard},
  "content.note@1":{compact:compactMarkup,detail:noteCard},
  "content.markdown@1":{compact:compactMarkup,detail:noteCard},
  "layout.section@1":{compact:compactMarkup,detail:defaultCard},
  "tool.calculator@1":{compact:compactMarkup,detail:calculatorCard},
  "lab.well_plate@1":{compact:compactMarkup,detail:plateCard},
  "workflow@1":{compact:compactMarkup,detail:workflowCard},
  "unsupported":{compact:compactMarkup,detail:unsupportedCard}
};
function rendererFor(component){
  const key=`${component.definition?.id||"unknown"}@${component.definition?.version??"?"}`;
  return componentRenderers[key]||componentRenderers.unsupported;
}
function proposalNoOp(preview){
  const changes=preview?.operation_changes||[];
  return changes.length>0&&changes.every(op=>JSON.stringify(op.before)===JSON.stringify(op.after));
}
function proposalPanelMarkup(){
  const child=selectedChildId?findById(data?.latest_version?.data?.root,selectedChildId):null;
  if(!child)return `<div class="proposal-empty">选择一张卡片后，可生成受控提案。</div>`;
  const kind=child.definition.id;const isNote=kind==="content.note"||kind==="content.markdown";const isTable=kind==="content.table"||kind==="layout.table";
  const actions=[`<button data-proposal="move-top">移动到最上面</button>`,isNote?`<button data-proposal="append-note">追加说明</button>`:``,isTable||kind==="workflow"?`<button data-proposal="add-note">添加 Markdown 卡片</button>`:``,`<button data-proposal="delete">删除此卡片</button>`].join("");
  if(!proposal)return `<div class="proposal-title">Agent Proposal · <b>${esc(nodeTitle(child))}</b></div><div class="muted">提案只读预览；接受前不会写入。</div><div class="row">${actions}</div>`;
  const stale=proposal.stale||proposal.noOp;
  const changes=(proposal.preview?.operation_changes||[]).map(op=>{
    if(op.kind==="move_slot_child")return `<div class="diff-row"><b>位置</b><span>${esc(JSON.stringify(op.before))} → ${esc(JSON.stringify(op.after))}</span></div>`;
    if(op.kind==="add_slot_child")return `<div class="diff-row"><b>新增</b><span>${esc(op.after?.definition?.id||"组件")} · ${esc(op.slot_id||"")}</span></div>`;
    if(op.kind==="update_props")return `<div class="diff-row"><b>字段</b><span>${esc(op.property)}: ${esc(JSON.stringify(op.before))} → ${esc(JSON.stringify(op.after))}</span></div>`;
    return `<div class="diff-row"><b>${esc(op.kind)}</b><span>${esc(JSON.stringify(op.edge||op.binding_id||op.instance_id||""))}</span></div>`;
  }).join("");
  return `<div class="proposal-title">Agent Proposal · <b>${esc(nodeTitle(child))}</b></div><div class="muted">目标：ViewDocument · revision ${esc(proposal.expectedRevision)} · ${proposal.noOp?"无实际变化":stale?"已过期":"预览校验通过"} · ${proposal.preview?.diff_scope==="net"?"最终净变化":"差异"} · 仅预览，未写入</div><div class="diff-list">${changes||"<div class=muted>无结构化差异</div>"}</div><div class="row"><button class="primary" data-proposal-accept ${stale?"disabled":""}>接受变更</button><button data-proposal-reject>拒绝</button></div>`;
}
function renderProposalPanel(){
  const box=$("proposal-panel");if(!box)return;box.innerHTML=proposalPanelMarkup();
  box.querySelectorAll("[data-proposal]").forEach(b=>b.onclick=()=>createProposal(b.dataset.proposal));
  box.querySelector("[data-proposal-accept]")?.addEventListener("click",acceptProposal);
  box.querySelector("[data-proposal-reject]")?.addEventListener("click",()=>{proposal=null;renderProposalPanel()});
}
function focusChild(id){
  if(selectedChildId!==id)proposal=null;
  selectedChildId=id;
  document.querySelectorAll(".card[data-instance]").forEach(c=>c.classList.toggle("selected",c.dataset.instance===id));
  renderProposalPanel();
}
async function createProposal(kind){
  if(!data||!selected)return;const child=selectedChildId?findById(data.latest_version.data.root,selectedChildId):null;if(!child)return;
  const requestViewId=viewId,requestDocumentId=data.latest_version.data.document_id,requestChild=selectedChildId,requestRevision=data.latest_version.data.revision;
  const doc=data.latest_version.data;const operations=[];
  if(kind==="move-top"){
    operations.push({kind:"move_slot_child",instance_id:child.instance_id,target_parent_instance_id:selected.instance_id,target_slot_id:"detail",target_index:0});
  }else if(kind==="add-note"){
    operations.push({kind:"add_slot_child",parent_instance_id:selected.instance_id,slot_id:"detail",child:{instance_id:crypto.randomUUID(),definition:{id:"content.note",version:1},props:{text:"实验注意事项\n"},slots:{}}});
  }else if(kind==="append-note"){
    const text=(child.props?.text||"")+(child.props?.text?"\n\n":"")+"（提案补充）";
    operations.push({kind:"update_props",instance_id:child.instance_id,property:"text",value:text});
  }else if(kind==="delete"){
    operations.push({kind:"remove_instance",instance_id:child.instance_id});
  }
  if(!operations.length)return;
  const changeset={changeset_id:`agent-${Date.now()}`,document_id:doc.document_id,expected_revision:doc.revision,operations,provenance:["agent-proposal"]};
  $("status").textContent="生成提案预览…";
  try{
    const r=await fetch(`/api/views/${encodeURIComponent(requestViewId)}/propose`,{method:"POST",headers:{"content-type":"application/json"},body:JSON.stringify({changeset})});
    const j=await r.json();if(!r.ok)throw new Error(j.error||"提案校验失败");
    if(viewId!==requestViewId||data?.latest_version?.data?.document_id!==requestDocumentId||selectedChildId!==requestChild||data?.latest_version?.data?.revision!==requestRevision){$("status").textContent="选择或版本已变化，旧提案已丢弃";return}
    proposal={viewId:requestViewId,documentId:requestDocumentId,expectedRevision:requestRevision,changeset,preview:j,stale:false,noOp:proposalNoOp(j)};renderProposalPanel();$("status").textContent=proposal.noOp?"提案无实际变化，不能接受":"提案预览已生成，尚未写入";
  }catch(e){$("status").textContent=e.message;proposal=null;renderProposalPanel()}
}
async function acceptProposal(){
  if(!proposal||proposal.noOp)return;
  const doc=data?.latest_version?.data;
  if(!doc||viewId!==proposal.viewId||doc.document_id!==proposal.documentId||doc.revision!==proposal.expectedRevision){proposal.stale=true;renderProposalPanel();$("status").textContent="提案已过期，请重新生成并预览";return}
  try{await commitFrozenChangeSet(proposal,"接受提案");proposal=null;selectedChildId=null;renderDetail();$("status").textContent="提案已接受并保存"}
  catch(e){$("status").textContent=e.message||"提案提交失败"}
}
function renderDetail(){
  const c=selected;
  if(!c){$("detail").innerHTML='<div class="muted">选择一个步骤查看详情。</div>';return}
  const slots=slotsOf(c); const cards=[];
  const allChildren=Object.values(slots).flat();
  const plates=allChildren.filter(x=>x.definition.id==="lab.well_plate");
  for(const [slot,children] of Object.entries(slots)) for(const child of children){
    const actions=cardActions(child);
    const renderer=child.workflow?.nodes?.length?componentRenderers["workflow@1"]:rendererFor(child);
    cards.push(renderer.detail(child,actions,{slot,plates}));
  }
  const ref=c.model_ref||{};
  const refLine=ref.kind==="operation_use"?`Protocol step · ${esc(ref.step?.step_path||"")}`:ref.kind?esc(ref.kind):"未绑定科学对象";
  const plateChoices=plates.map(p=>`<option value="${esc(p.instance_id)}">${esc(nodeTitle(p))} · ${esc(p.model_ref?.container_id||"")}</option>`).join("");
  const library=[["layout.section","区块"],["content.table","表格"],["tool.calculator","计算器"],["content.note","说明"],["workflow","子流程"]].map(([kind,label])=>`<button data-add="${kind}">+ ${label}</button>`).join("")+(plates.length?`<select id="add-plate" aria-label="要引用的孔板">${plateChoices}</select><button data-add="lab.well_plate">+ 孔板</button>`:"");
  $("detail").innerHTML=`<div class="detail-head"><div><div class="detail-title">${esc(nodeTitle(c))} · Step Detail</div><div class="muted">${esc(defName(c))} · ${esc(c.instance_id)}</div></div><button id="collapse" aria-label="收起">⌃</button></div><div class="detail-grid"><div><h3>操作参数</h3><div id="params-${esc(c.instance_id)}" class="muted">读取 ProtocolPreview…</div><div class="param"><b>科学引用</b><span>${refLine}</span></div></div><div><h3>相关组件</h3><div class="cards"><div class="grid-sizer"></div>${cards.join("")||'<div class="muted">此步骤暂无详情组件。</div>'}</div><div class="library"><h3>添加组件</h3>${library}</div></div></div><div id="proposal-panel" class="proposal"></div><div id="preview" class="preview"></div>`;
  $("detail").querySelectorAll('.card[data-instance]').forEach(card=>card.addEventListener("click",e=>{if(e.target.closest("button,input,textarea,select,a"))return;focusChild(card.dataset.instance)}));
  renderProposalPanel();
  $("detail").querySelectorAll('[data-enter]').forEach(b=>b.onclick=()=>{const child=findById(data.latest_version.data.root,b.dataset.enter);if(child)enterComponent(child)});
  $("detail").querySelectorAll('[data-save]').forEach(b=>b.onclick=()=>saveNote(b.dataset.save));
  $("detail").querySelectorAll('[data-table-save]').forEach(b=>b.onclick=()=>saveTable(b.dataset.tableSave));
  $("detail").querySelectorAll('[data-table-add-col]').forEach(b=>b.onclick=()=>tableAddColumn(b.dataset.tableAddCol));
  $("detail").querySelectorAll('[data-table-add-row]').forEach(b=>b.onclick=()=>tableAddRow(b.dataset.tableAddRow));
  $("detail").querySelectorAll('[data-md-toggle]').forEach(b=>b.onclick=()=>toggleMarkdown(b.dataset.mdToggle));
  $("detail").querySelectorAll('[data-plate-preview]').forEach(b=>b.onclick=()=>{const set=[...plateSelections.get(b.dataset.platePreview)||[]].sort();if(!set.length){$("status").textContent="请先选择至少一个孔位";return}loadData(b.dataset.platePreview,set.join(","))});
  $("detail").querySelectorAll('[data-plate-all]').forEach(b=>b.onclick=()=>selectAllWells(b.dataset.plateAll,true));
  $("detail").querySelectorAll('[data-plate-clear]').forEach(b=>b.onclick=()=>selectAllWells(b.dataset.plateClear,false));
  $("detail").querySelectorAll('[data-calc]').forEach(b=>b.onclick=()=>{const state=calcStateFor(b.dataset.calc);if(!state.plateId){$("status").textContent="请先为该计算器选择孔板";return}const positions=state.positions||[...(plateSelections.get(state.plateId)||[])].sort().join(",");state.positions=positions;syncCalcPositions(state.plateId);loadData(state.plateId,positions,b.dataset.calc)});
  $("detail").querySelectorAll('[data-remove]').forEach(b=>b.onclick=()=>mutate([{kind:"remove_instance",instance_id:b.dataset.remove}],"删除组件"));
  $("detail").querySelectorAll('[data-move-up]').forEach(b=>b.onclick=()=>moveComponent(b.dataset.moveUp,-1));
  $("detail").querySelectorAll('[data-move-down]').forEach(b=>b.onclick=()=>moveComponent(b.dataset.moveDown,1));
  $("detail").querySelectorAll('[data-add]').forEach(b=>b.onclick=()=>addComponent(b.dataset.add));
  $("detail").querySelectorAll('[data-instance]').forEach(card=>{
    const id=card.dataset.instance;const state=calcStates.get(id);
    if(state){
      card.querySelector('[data-calc-plate]')?.addEventListener("change",e=>{state.plateId=e.target.value;state.positions=[...(plateSelections.get(state.plateId)||[])].sort().join(",");syncCalcPositions(state.plateId)});
      card.querySelectorAll("input").forEach(input=>input.addEventListener("input",()=>{state.positions=$(`calc-${CSS.escape(id)}-positions`)?.value||"";state.volume=$(`calc-${CSS.escape(id)}-volume`)?.value||"";state.unit=$(`calc-${CSS.escape(id)}-unit`)?.value||"";state.extra=$(`calc-${CSS.escape(id)}-extra`)?.value||""}));
    }
    if(card.querySelector("[data-table]")){
      card.querySelectorAll("[data-colhead]").forEach(input=>input.addEventListener("input",()=>{const draft=tableDraft(findById(data.latest_version.data.root,id));draft.columns[Number(input.dataset.colhead)]=input.value;draft.token=(draft.token||0)+1;markDirtyCard(id,true)}));
      card.querySelectorAll("[data-cell]").forEach(input=>input.addEventListener("input",()=>{const [r,c]=input.dataset.cell.split("-").map(Number);const draft=tableDraft(findById(data.latest_version.data.root,id));draft.rows[r][c]=input.value;draft.token=(draft.token||0)+1;markDirtyCard(id,true)}));
    }
    const note=card.querySelector("textarea");if(note)note.addEventListener("input",()=>{drafts.set(id,{text:note.value,token:(drafts.get(id)?.token||0)+1});markDirtyCard(id,true)});
  });
  Object.values(slots).flat().filter(child=>child.definition.id==="lab.well_plate").forEach(child=>loadPlate(child.instance_id,`plate-body-${child.instance_id}`));
  loadParams(c,`params-${c.instance_id}`);
  layoutCards();
}
function siblingChildren(id){
  if(!selected)return null;const list=Object.values(slotsOf(selected)).flat();const index=list.findIndex(x=>x.instance_id===id);return index<0?null:{list,index}
}
function moveComponent(id,delta){
  const info=siblingChildren(id);if(!info)return;const target=info.index+delta;if(target<0||target>=info.list.length)return;
  mutate([{kind:"move_slot_child",instance_id:id,target_parent_instance_id:selected.instance_id,target_slot_id:"detail",target_index:target}],"移动组件")
}
async function addComponent(kind){
  if(!selected||!data)return;
  const child={instance_id:crypto.randomUUID(),definition:{id:kind,version:1},props:{},slots:{}};
  if(kind==="workflow")child.workflow={nodes:[],edges:[]};
  if(kind==="tool.calculator")child.props={tool_ref:"tool.reagent_total@1"};
  if(kind==="content.table")child.props={rows:[]};
  if(kind==="lab.well_plate"){
    const selectedPlate=$("add-plate")?.value;
    const existing=[];const walk=c=>{if(c.definition?.id==="lab.well_plate")existing.push(c);Object.values(c.slots||{}).flat().forEach(walk);(c.workflow?.nodes||[]).forEach(n=>walk(n.component))};walk(data.latest_version.data.root);
    const plate=existing.find(p=>p.instance_id===selectedPlate);
    if(!plate){$("status").textContent="请先选择要引用的孔板";return}
    child.model_ref=plate.model_ref;
  }
  await mutate([{kind:"add_slot_child",parent_instance_id:selected.instance_id,slot_id:"detail",child}],`添加 ${kind}`)
}
function commitViewChangeSet(operations,label){
  if(!data)return Promise.reject(new Error("View 未加载"));
  const targetViewId=viewId;const targetDocumentId=data.latest_version.data.document_id;
  const task=async()=>{
    if(viewId!==targetViewId||data?.latest_version?.data?.document_id!==targetDocumentId){$("status").textContent="View 已切换，已取消旧保存";throw new Error("view_changed")}
    const doc=data.latest_version.data;
    const changeset={changeset_id:`ui-${Date.now()}`,document_id:doc.document_id,expected_revision:doc.revision,operations,provenance:["view-ui"]};
    $("status").textContent=`${label}…`;
    const r=await fetch(`/api/views/${encodeURIComponent(targetViewId)}/revise`,{method:"POST",headers:{"content-type":"application/json"},body:JSON.stringify({changeset})});
    const j=await r.json();
    if(!r.ok){const message=j.error||`${label}失败`;if(viewId===targetViewId&&data?.latest_version?.data?.document_id===targetDocumentId)await load(true,true);$("status").textContent=message;throw new Error(message)}
    if(viewId!==targetViewId||data?.latest_version?.data?.document_id!==targetDocumentId){$("status").textContent=`${label}已写入原文档，当前 View 未刷新`;return}
    await load(true,true);$("status").textContent=`${label}已保存`;
  };
  const run=saveChain.then(task,task);saveChain=run.catch(()=>{});return run
}
function mutate(operations,label){return commitViewChangeSet(operations,label)}
function commitFrozenChangeSet(proposal,label){
  if(!data)return Promise.reject(new Error("View 未加载"));
  const targetViewId=proposal.viewId,targetDocumentId=proposal.documentId;
  const task=async()=>{
    if(viewId!==targetViewId||data?.latest_version?.data?.document_id!==targetDocumentId){$("status").textContent="View 已切换，提案提交已取消";throw new Error("view_changed")}
    $("status").textContent=`${label}…`;
    const r=await fetch(`/api/views/${encodeURIComponent(targetViewId)}/revise`,{method:"POST",headers:{"content-type":"application/json"},body:JSON.stringify({changeset:proposal.changeset})});
    const j=await r.json();
    if(!r.ok){const message=j.error||`${label}失败`;if(viewId===targetViewId)await load(true,true);$("status").textContent=message;throw new Error(message)}
    if(viewId!==targetViewId){return}
    await load(true,true);$("status").textContent=`${label}已保存`;
  };
  const run=saveChain.then(task,task);saveChain=run.catch(()=>{});return run
}

function render(){renderCrumbs();renderCanvas();renderDetail();$("workspaceTitle").textContent=(data?.latest_version?.data?.root?.props?.title)||"AsterLab / CCK-8 Assay";const d=data?.latest_version?.data;$("workspaceSub").textContent=d?`View revision ${d.revision} · ${esc(d.document_id).slice(0,8)}`:"Protocol · View"}
async function load(preserveHistory=false,preserveUi=false){
  viewId=$("viewId").value.trim();if(!viewId)return;$("status").textContent="加载中…";
  const previousStack=stack.map(c=>c.instance_id),previousNode=node?.instance_id,previousSelected=selected?.instance_id;
  const r=await fetch(`/api/views/${encodeURIComponent(viewId)}`);const j=await r.json();if(!r.ok){$("status").textContent=j.error||"加载失败";return}
  data=j;const root=data.latest_version?.data?.root;
  if(preserveUi){
    stack=previousStack.map(id=>findById(root,id)).filter(Boolean);if(!stack.length)stack=root?[root]:[];
    node=findById(root,previousNode)||root;selected=findById(root,previousSelected)||workflowOf(node)[0]?.component||node;
  }else{
    node=root;stack=node?[node]:[];selected=workflowOf(node)[0]?.component||node;
    paramCache.clear();plateCache.clear();plateSelections.clear();
    calcStates.clear();drafts.clear();
  }
  if(!preserveHistory){history=[];future=[]}
  render();$("status").textContent=`revision ${data.latest_version?.data?.revision??"?"}`
}
async function saveNote(instanceId){
  if(!data)return;const draft=drafts.get(instanceId);const savedToken=draft?.token||0;const text=draft?.text??$("note-"+CSS.escape(instanceId))?.value??"";
  await commitViewChangeSet([{kind:"update_props",instance_id:instanceId,property:"text",value:text}],"保存说明");
  const next=drafts.get(instanceId);if(!next||(next.token||0)===savedToken){drafts.delete(instanceId);renderDetail()}
}
function screenToSvg(clientX,clientY){
  const svg=$("canvas"),p=svg.createSVGPoint();p.x=clientX;p.y=clientY;const ctm=svg.getScreenCTM();
  if(!ctm)return {x:view.x,y:view.y};return p.matrixTransform(ctm.inverse())
}
function screenToCanvas(clientX,clientY){
  const u=screenToSvg(clientX,clientY);return {x:(u.x-view.x)/view.k,y:(u.y-view.y)/view.k}
}
function persistVisual(instanceId,pos){
  return commitViewChangeSet([{kind:"set_visual",instance_id:instanceId,x:pos.x,y:pos.y}],"保存布局")
}
async function loadData(instanceId,positionsOverride,calculatorId){
  if(!data)return;const state=calculatorId?calcStateFor(calculatorId):null;
  let requestSeq;
  if(state){state.requestSeq=(state.requestSeq||0)+1;requestSeq=state.requestSeq;if(!state.plateId){$("status").textContent="请先为该计算器选择孔板";return}instanceId=state.plateId;positionsOverride=positionsOverride??state.positions}
  else requestSeq=++dataRequestSeq;
  const params=new URLSearchParams({instance_id:instanceId});if(calculatorId)params.set("calculator_instance_id",calculatorId);
  const positions=positionsOverride??state?.positions??"";
  if(state&&positionsOverride)state.positions=positionsOverride;
  if(positions)params.set("position_ids",positions);
  if(state?.volume)params.set("volume_per_well",state.volume);
  if(state?.unit)params.set("volume_unit",state.unit);
  if(state?.extra)params.set("extra_ratio",state.extra);
  $("status").textContent="加载数据…";const r=await fetch(`/api/views/${encodeURIComponent(viewId)}/data?${params}`);const j=await r.json();if(state&&requestSeq!==state.requestSeq)return;if(!state&&requestSeq!==dataRequestSeq)return;if(!r.ok){$("status").textContent=j.error||"数据加载失败";return}const payload=j.result??j;const table=payload.table;const calcResult=payload.calculation||{};const box=state?$(`calc-${CSS.escape(calculatorId)}-result`):$("preview");if(!box)return;
  box.innerHTML=`<h3>数据预览</h3><table><thead><tr>${table.columns.map(c=>`<th>${esc(c)}</th>`).join("")}</tr></thead><tbody>${table.rows.map(row=>`<tr>${row.map(v=>`<td>${esc(v)}</td>`).join("")}</tr>`).join("")}</tbody></table><div class="muted">${esc(table.unresolved.join(", ")||"resolved")} · ${esc(calcResult.tool_action||"tool action")} · 使用 ${esc(calcResult.assigned_positions??"未解析")} 孔 · 单位 ${esc(calcResult.unit||"未解析")} · ${esc(calcResult.state||"")}</div>`;$("status").textContent="数据已加载"
}
let connectDrag=null;
function startConnect(ev,fromId){
  if(mode==="hand")return;ev.stopPropagation();ev.preventDefault();
  const source=(node?.workflow?.nodes||[]).find(n=>n.node_id===fromId)?.component;if(!source)return;
  const A=nodeBox(source),p0={x:A.x+76,y:A.y+38};
  const temp=document.createElementNS("http://www.w3.org/2000/svg","path");temp.setAttribute("class","edge temp-edge");temp.setAttribute("d",`M${p0.x} ${p0.y} L${p0.x} ${p0.y}`);document.querySelector("#viewport")?.appendChild(temp);
  connectDrag={fromId,temp};
  const hit=e=>[...document.querySelectorAll(".wnode")].find(el=>{const b=el.getBoundingClientRect();return e.clientX>=b.left&&e.clientX<=b.right&&e.clientY>=b.top&&e.clientY<=b.bottom})?.dataset.node||"";
  const move=e=>{if(!connectDrag)return;const p=screenToCanvas(e.clientX,e.clientY);connectDrag.temp.setAttribute("d",`M${p0.x} ${p0.y} C${p0.x+40} ${p0.y},${p.x-40} ${p.y},${p.x} ${p.y}`);document.querySelectorAll(".wnode.connect-target").forEach(x=>x.classList.remove("connect-target"));const target=hit(e);if(target&&target!==fromId)document.querySelector(`.wnode[data-node="${CSS.escape(target)}"]`)?.classList.add("connect-target")};
  const up=e=>{window.removeEventListener("pointermove",move);window.removeEventListener("pointerup",up);const drag=connectDrag;connectDrag=null;drag?.temp.remove();document.querySelectorAll(".wnode.connect-target").forEach(x=>x.classList.remove("connect-target"));const target=hit(e);if(!target||target===fromId){$("status").textContent="连接取消：需要选择另一个节点";return}commitViewChangeSet([{kind:"connect",workflow_instance_id:node.instance_id,edge:{from:fromId,to:target}}],"连接节点").catch(()=>{})};
  window.addEventListener("pointermove",move);window.addEventListener("pointerup",up,{once:true});
}
function startNodeDrag(ev,c){
  if(mode==="hand")return;ev.stopPropagation();const B=nodeBox(c);const start=screenToCanvas(ev.clientX,ev.clientY);
  dragNode={c,start,x:B.x,y:B.y};nodeMoved=false;
  const move=e=>{if(!dragNode)return;const now=screenToCanvas(e.clientX,e.clientY);const dx=now.x-dragNode.start.x,dy=now.y-dragNode.start.y;if(Math.abs(dx)+Math.abs(dy)>2)nodeMoved=true;dragNode.c._vx=dragNode.x+dx;dragNode.c._vy=dragNode.y+dy;dragNode.c.visual={...(dragNode.c.visual||{}),x:dragNode.c._vx,y:dragNode.c._vy};renderCanvas()};
  const up=()=>{window.removeEventListener("pointermove",move);window.removeEventListener("pointerup",up);const d=dragNode;dragNode=null;if(!d)return;if(nodeMoved){
    const before={x:d.x,y:d.y},after={x:d.c._vx,y:d.c._vy};
    const entry={instance_id:d.c.instance_id,before,after,committed_revision:data?.latest_version?.data?.revision??0};
    persistVisual(entry.instance_id,after).then(()=>{entry.committed_revision=data?.latest_version?.data?.revision??entry.committed_revision;history.push(entry);future=[];if(history.length>50)history.shift();updateHistoryButtons()}).catch(()=>{});
  }};
  window.addEventListener("pointermove",move);window.addEventListener("pointerup",up,{once:true});
}
function applyPosition(entry,key){
  const c=findById(data.latest_version.data.root,entry.instance_id);if(!c)return null;const p=entry[key];
  c._vx=p.x;c._vy=p.y;c.visual={...(c.visual||{}),x:p.x,y:p.y};renderCanvas();return c
}
$("undo").onclick=()=>{const entry=history.pop();if(!entry)return;const c=applyPosition(entry,"before");if(!c){history.push(entry);return}future.push(entry);updateHistoryButtons();
  persistVisual(entry.instance_id,entry.before).catch(()=>{history.push(entry);future=future.filter(x=>x!==entry);updateHistoryButtons()})};
$("redo").onclick=()=>{const entry=future.pop();if(!entry)return;const c=applyPosition(entry,"after");if(!c){future.push(entry);return}history.push(entry);updateHistoryButtons();
  persistVisual(entry.instance_id,entry.after).catch(()=>{future.push(entry);history=history.filter(x=>x!==entry);updateHistoryButtons()})};
$("selectTool").onclick=()=>{mode="select";$("selectTool").classList.add("active");$("handTool").classList.remove("active");$("canvas").classList.remove("hand")};
$("handTool").onclick=()=>{mode="hand";$("handTool").classList.add("active");$("selectTool").classList.remove("active");$("canvas").classList.add("hand")};
$("fit").onclick=()=>{const {nodes}=graph();if(!nodes.length)return;const xs=nodes.map(n=>nodeBox(n.component));const minX=Math.min(...xs.map(b=>b.x)),maxX=Math.max(...xs.map(b=>b.x+76)),minY=Math.min(...xs.map(b=>b.y)),maxY=Math.max(...xs.map(b=>b.y+120));const k=Math.min(1.2,Math.max(.35,Math.min(1000/(maxX-minX+80),480/(maxY-minY+80))));view={k,x:80-minX*k,y:140-minY*k};renderCanvas()};
function zoomAt(f){const k=Math.min(2,Math.max(.25,view.k*f));const svg=$("canvas"),r=svg.getBoundingClientRect();const center=screenToCanvas(r.left+r.width/2,r.top+r.height/2);view.x=view.x+center.x*(view.k-k);view.y=view.y+center.y*(view.k-k);view.k=k;renderCanvas()}
$("zoomIn").onclick=()=>zoomAt(1.2);$("zoomOut").onclick=()=>zoomAt(1/1.2);
$("canvas").addEventListener("wheel",e=>{e.preventDefault();const p=screenToCanvas(e.clientX,e.clientY);const f=e.deltaY<0?1.1:1/1.1;const k=Math.min(2,Math.max(.25,view.k*f));view.x=view.x+p.x*(view.k-k);view.y=view.y+p.y*(view.k-k);view.k=k;renderCanvas()},{passive:false});
$("canvas").addEventListener("pointerdown",e=>{if(e.target.closest(".wnode"))return;pan={sx:e.clientX,sy:e.clientY,vx:view.x,vy:view.y,k:view.k};$("canvas").classList.add("grabbing")});
window.addEventListener("pointermove",e=>{if(!pan)return;const a=screenToSvg(pan.sx,pan.sy),b=screenToSvg(e.clientX,e.clientY);view.x=pan.vx+(b.x-a.x)/pan.k;view.y=pan.vy+(b.y-a.y)/pan.k;renderCanvas()});
window.addEventListener("pointerup",()=>{pan=null;$("canvas").classList.remove("grabbing")});

const canvasPanel=$("canvasPanel"), splitHandle=$("splitter");
function setCanvasHeight(h){
  const max=Math.max(240,window.innerHeight-260);const value=Math.max(220,Math.min(max,h));
  canvasPanel.style.flexBasis=`${value}px`;canvasPanel.style.height=`${value}px`;localStorage.setItem("asterlab.canvasHeight",String(value));
}
{
  const saved=Number(localStorage.getItem("asterlab.canvasHeight"));
  if(saved)setCanvasHeight(saved);
}
let splitDrag=null;
splitHandle.addEventListener("pointerdown",e=>{
  splitDrag={y:e.clientY,h:canvasPanel.getBoundingClientRect().height};
  splitHandle.setPointerCapture(e.pointerId);document.body.style.cursor="row-resize";
});
splitHandle.addEventListener("pointermove",e=>{
  if(!splitDrag)return;setCanvasHeight(splitDrag.h+(e.clientY-splitDrag.y));renderCanvas();
});
splitHandle.addEventListener("pointerup",e=>{
  splitDrag=null;document.body.style.cursor="";splitHandle.releasePointerCapture?.(e.pointerId);layoutCards();
});
splitHandle.addEventListener("keydown",e=>{
  if(e.key!=="ArrowUp"&&e.key!=="ArrowDown")return;e.preventDefault();
  setCanvasHeight(canvasPanel.getBoundingClientRect().height+(e.key==="ArrowDown"?20:-20));renderCanvas();
});
window.addEventListener("resize",()=>{const h=canvasPanel.getBoundingClientRect().height;setCanvasHeight(h);renderCanvas();layoutCards()});
$("openView").onclick=load;
