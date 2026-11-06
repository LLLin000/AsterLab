const assert = require("node:assert/strict");
const {spawn} = require("node:child_process");
const fs = require("node:fs");
const path = require("node:path");
const net = require("node:net");

const root = path.resolve(__dirname, "..");
const bin = process.env.LABWORKBENCH_BIN || path.join(root, "target", "release", "labworkbench.exe");
const tempRoot = fs.mkdtempSync(path.join(root, "target", "backup-restore-"));
const dataDir = path.join(tempRoot, "data");
const env = {...process.env, LABWORKBENCH_DATA_DIR: dataDir};
function freePort(){return new Promise((resolve,reject)=>{const s=net.createServer();s.once("error",reject);s.listen(0,"127.0.0.1",()=>{const p=s.address().port;s.close(()=>resolve(p))})})}
async function start(port){const p=spawn(bin,[],{cwd:root,env:{...env,LABWORKBENCH_PORT:String(port)},windowsHide:true,stdio:["ignore","pipe","pipe"]});for(let i=0;i<100;i++){try{const r=await fetch(`http://127.0.0.1:${port}/api/health`);if(r.ok)return p}catch{}await new Promise(r=>setTimeout(r,100))}p.kill();throw new Error("server failed to start")}
async function stop(p){if(p&&p.exitCode===null){p.kill();await new Promise(r=>p.once("exit",r))}}
async function post(base,path,body,headers={}){const r=await fetch(base+path,{method:"POST",headers:{"content-type":"application/json",...headers},body:JSON.stringify(body)});const j=await r.json();return {status:r.status,body:j}}
(async()=>{
  const port=await freePort(),base=`http://127.0.0.1:${port}`;let server=await start(port);
  const project=(await post(base,"/api/records",{kind:"project",data:{name:"Backup test"}})).body;
  const design=(await post(base,"/api/records",{kind:"design",parent_id:project.id,data:{title:"Backup design",group_specs:[{name:"Control",role:"control",condition:"synthetic"}]}})).body;
  const human={"x-asterlab-human-confirmed":"1"};
  const run=await post(base,"/api/records",{kind:"run",parent_id:design.id,data:{title:"Backup run"}},human);
  assert.equal(run.status,200,JSON.stringify(run.body));
  const first=await post(base,"/api/records",{kind:"note",parent_id:run.body.id,data:{content:"before backup",event_time:null}});
  assert.equal(first.status,200);
  const unauthorized=await post(base,"/api/backup",{});
  assert.equal(unauthorized.status,403);
  const backup=await post(base,"/api/backup",{},human);
  assert.equal(backup.status,200,JSON.stringify(backup.body));
  assert(fs.existsSync(backup.body.path));
  const later=await post(base,"/api/records",{kind:"note",parent_id:run.body.id,data:{content:"after backup",event_time:null}});
  assert.equal(later.status,200);
  const restore=await post(base,"/api/restore",{path:backup.body.path},human);
  assert.equal(restore.status,200,JSON.stringify(restore.body));
  const state=(await (await fetch(base+"/api/state")).json()).records;
  assert(state.some(r=>r.kind==="note"&&r.data.content==="before backup"),"backup content missing after restore");
  assert(!state.some(r=>r.kind==="note"&&r.data.content==="after backup"),"post-backup write survived restore");
  await stop(server);
  server=await start(port);
  const reopened=(await (await fetch(base+"/api/state")).json()).records;
  assert(reopened.some(r=>r.kind==="note"&&r.data.content==="before backup"),"restored data did not survive restart");
  await stop(server);
  console.log(JSON.stringify({ok:true,restored:true,restart:true,unauthorized:true,backup:backup.body.path}));
})().catch(async error=>{console.error(error);process.exitCode=1});
