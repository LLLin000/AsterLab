const assert = require("node:assert/strict");
const {spawn, spawnSync} = require("node:child_process");
const fs = require("node:fs");
const path = require("node:path");
const net = require("node:net");
const {chromium} = require("C:/Users/Lin/node_modules/playwright");
const root = path.resolve(__dirname, "..");
const bin = process.env.LABWORKBENCH_BIN || path.join(root, "target", "release", "labworkbench.exe");
const tempRoot = fs.mkdtempSync(path.join(root, "target", "pages-smoke-"));
const dataDir = path.join(tempRoot, "data");
const env = {...process.env, LABWORKBENCH_DATA_DIR: dataDir, LABWORKBENCH_TRUSTED_WRITER: "1"};
function cli(tool, input) {
  const file = path.join(tempRoot, `${tool.replace(/\W/g, "_")}-${Math.random().toString(16).slice(2)}.json`);
  fs.writeFileSync(file, JSON.stringify(input));
  const run = spawnSync(bin, [tool, "--input", file], {cwd: root, env, encoding: "utf8"});
  fs.rmSync(file, {force: true});
  assert.equal(run.status, 0, `${tool}: ${run.stdout}${run.stderr}`);
  return JSON.parse(run.stdout.trim());
}
function freePort(){return new Promise((resolve,reject)=>{const s=net.createServer();s.once("error",reject);s.listen(0,"127.0.0.1",()=>{const p=s.address().port;s.close(()=>resolve(p))})})}
async function start(port){const p=spawn(bin,[],{cwd:root,env:{...env,LABWORKBENCH_PORT:String(port)},windowsHide:true,stdio:["ignore","pipe","pipe"]});for(let i=0;i<100;i++){try{if((await fetch(`http://127.0.0.1:${port}/api/health`)).ok)return p}catch{}await new Promise(r=>setTimeout(r,100))}p.kill();throw new Error("server failed")}
(async()=>{
  const project=cli("project.create",{name:"Pages smoke"}).result.record;
  const block=cli("protocol.block.create",{project_id:project.id,title:"Pages protocol",steps:[{kind:"text",use_id:"seed",title:"Seed",instruction:"Seed cells",checks:[{check_id:"ready",label:"Ready"}]},{kind:"text",use_id:"read",title:"Read",instruction:"Read plate"}]}).result;
  const protocol=cli("protocol.create",{project_id:project.id,title:"Pages protocol",root_block_version_id:block.version.id}).result;
  const design=cli("design.create",{project_id:project.id,design:{title:"Pages design",research_question:"Does it work?",factors:[{name:"Treatment",levels:["control","treated"]}],group_specs:[{name:"Control",role:"control",condition:"vehicle",independent_n:3},{name:"Treatment",role:"treatment",condition:"X",independent_n:3}],assays:[{id:"cck8",title:"CCK-8",endpoint:"OD450",method:"CCK-8",unit:"AU",timepoints:["24 h"],group_names:["Control","Treatment"],protocol_version_id:protocol.version.id}]}}).result.design;
  cli("run.create",{design_id:design.id,title:"Pages run"});
  const port=await freePort(),base=`http://127.0.0.1:${port}`;const server=await start(port);
  const browser=await chromium.launch({headless:true});const page=await browser.newPage({viewport:{width:1440,height:1000}});const errors=[];page.on("pageerror",e=>errors.push(e.message));
  await page.goto(base,{waitUntil:"networkidle"});
  assert.equal(await page.locator(".rail-item[data-page]").count(),4);
  await page.locator('.rail-item[data-page="protocols"]').click();await page.waitForTimeout(200);assert((await page.locator("#page-view").innerText()).includes("Pages protocol"));
  await page.locator('.rail-item[data-page="runs"]').click();await page.waitForTimeout(300);assert((await page.locator("#page-view").innerText()).includes("Pages run"));
  await page.locator('.rail-item[data-page="tools"]').click();assert((await page.locator("#page-view").innerText()).includes("常用工具"));
  assert.deepEqual(errors,[]);
  await browser.close();server.kill();console.log(JSON.stringify({ok:true,rail:4,pages:["protocols","runs","tools"],errors}));
})().catch(error=>{console.error(error);process.exitCode=1});
