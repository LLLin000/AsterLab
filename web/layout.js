(function(root,factory){
  if(typeof module==="object"&&module.exports)module.exports=factory();
  else root.AsterLayout=factory();
})(typeof self!=="undefined"?self:this,function(){
  const W=76,H=120,GAP=16;
  function overlaps(a,b){return a.x < b.x+W+GAP && a.x+W+GAP > b.x && a.y < b.y+H+GAP && a.y+H+GAP > b.y}
  function layout(nodes,edges,dagreApi){
    const out={};
    if(!Array.isArray(nodes)||!nodes.length)return out;
    const manual={};
    nodes.forEach(n=>{const v=n.component?.visual||{};if(typeof v.x==="number"&&typeof v.y==="number")manual[n.node_id]={x:v.x,y:v.y}});
    let positions={};
    if(dagreApi&&nodes.length){
      try{
        const g=new dagreApi.graphlib.Graph();
        g.setGraph({rankdir:"LR",nodesep:46,ranksep:96,marginx:40,marginy:40});
        g.setDefaultEdgeLabel(()=>({}));
        nodes.forEach(n=>g.setNode(n.node_id,{width:W,height:H}));
        (edges||[]).forEach(e=>{if(g.hasNode(e.from)&&g.hasNode(e.to))g.setEdge(e.from,e.to)});
        dagreApi.layout(g);
        nodes.forEach(n=>{const p=g.node(n.node_id);if(p)positions[n.node_id]={x:p.x-W/2,y:p.y-H/2}});
      }catch(e){positions={}}
    }
    const placed=[];
    nodes.forEach((n,i)=>{
      const id=n.node_id;
      if(manual[id]){out[id]={...manual[id]};placed.push(out[id]);return}
      let p=positions[id]||{x:80+i*190,y:150};
      let guard=0;
      while(placed.some(q=>overlaps(p,q))&&guard++<100)p={x:p.x,y:p.y+140};
      out[id]={x:p.x,y:p.y};placed.push(out[id]);
    });
    return out;
  }
  return {layout};
});
