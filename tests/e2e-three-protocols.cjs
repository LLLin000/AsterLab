const assert = require("node:assert/strict");
const {spawn,spawnSync} = require("node:child_process");
const fs = require("node:fs");
const path = require("node:path");
const net = require("node:net");
const {chromium} = require("C:/Users/Lin/node_modules/playwright");

const root = path.resolve(__dirname, "..");
const bin = process.env.LABWORKBENCH_BIN || path.join(root, "target", "release", "labworkbench.exe");
const tempRoot = fs.mkdtempSync(path.join(root, "target", "e2e-three-"));
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
async function start(port){const p=spawn(bin,[],{cwd:root,env:{...env,LABWORKBENCH_PORT:String(port)},windowsHide:true,stdio:["ignore","pipe","pipe"]});for(let i=0;i<100;i++){try{if((await fetch(`http://127.0.0.1:${port}/api/health`)).ok)return p}catch{}await new Promise(r=>setTimeout(r,100))}p.kill();throw new Error("server failed to start")}
async function stop(p){if(p&&p.exitCode===null){p.kill();await new Promise(r=>p.once("exit",r))}}

const project=cli("project.create",{name:"AsterLab Beta synthetic E2E"}).result.record;
const protocols=[
  {name:"CCK-8", id:"cck8", title:"CCK-8 viability",
   steps:[
    ["seed","Seed plate","Seed cells; check plate readiness.","plate_ready","Plate ready"],
    ["dose","Dose treatment","Add vehicle or treatment.",null,null],
    ["add_cck8","Add CCK-8","Add CCK-8 reagent.",null,null],
    ["incubate","Incubate signal","Incubate protected from light.",null,null],
    ["mix","Mix and inspect","Mix; check bubbles.",null,null],
    ["read_od","Read OD450","Read raw absorbance.",null,null],
    ["delayed","Optional stabilization","Optional SDS/HCl branch.",null,null],
    ["calculate","Derive result","Apply stated calculation.",null,null]
   ]},
  {name:"RT-qPCR", id:"qpcr", title:"RT-qPCR",
   steps:[
    ["extract","Extract RNA","Extract and QC RNA.",null,null],
    ["rt","Reverse transcribe","Run RT with controls.",null,null],
    ["qpcr","Set up qPCR","Prepare assay, NTC and NRT.","ntc_clean","NTC clean"],
    ["cycle","Run thermal program","Run the instrument.",null,null],
    ["analyze","Analyze Cq","Review raw Cq before derivation.",null,null]
   ]},
  {name:"Biofilm CV+CFU", id:"biofilm", title:"Biofilm CV + CFU",
   steps:[
    ["culture","Culture biofilm","Inoculate and incubate.",null,null],
    ["wash","Wash biofilm","Wash without claiming zero loss.","sterile_pbs","Sterile PBS for CFU"],
    ["cv","CV stain and read","Stain, dry, solubilize, read CV.",null,null],
    ["disperse","Disperse biofilm","Harvest and disperse with controls.",null,null],
    ["dilute","Serial dilute","Dilute with sterile diluent.",null,null],
    ["plate","Plate CFU","Plate and incubate.",null,null],
    ["count","Count CFU","Count raw colonies.",null,null],
    ["calculate","Calculate CFU/mL","Apply stated dilution and volume.",null,null]
   ]}
];
const designs=[];
for(const spec of protocols){
  const block=cli("protocol.block.create",{project_id:project.id,title:`${spec.name} synthetic block`,steps:spec.steps.map(([use_id,title,instruction,checkId,label])=>({kind:"text",use_id,title,instruction,parameter_refs:[],checks:checkId?[{check_id:checkId,label,blocking:true}]:[]}))}).result;
  const protocol=cli("protocol.create",{project_id:project.id,title:`${spec.name} synthetic protocol`,root_block_version_id:block.version.id}).result;
  const design=cli("design.create",{project_id:project.id,design:{title:`${spec.name} synthetic design`,objective:"Synthetic beta E2E; not a real experiment.",experimental_unit:"one independent well",group_specs:[{name:"Control",role:"control",condition:"synthetic vehicle"},{name:"Treatment",role:"treatment",condition:"synthetic treatment"}],assays:[{id:spec.id,title:spec.title,endpoint:"synthetic endpoint",method:spec.name,unit:"AU",timepoints:["D1"],group_names:["Control","Treatment"],protocol_version_id:protocol.version.id}]}}).result.design;
  const checkStep=spec.steps.find(x=>x[3]);
  designs.push({spec,design,checkId:checkStep[3],checkPath:`root/${checkStep[0]}`});
}

(async()=>{
  const port=await freePort(),base=`http://127.0.0.1:${port}`;let server=await start(port);
  const browser=await chromium.launch({headless:true});
  const page=await browser.newPage({viewport:{width:1440,height:1000}});page.setDefaultTimeout(8000);
  const errors=[];page.on("pageerror",e=>errors.push(e.message));
  console.log("goto",base);
  await page.goto(base,{waitUntil:"networkidle"});
  console.log("projects",await page.locator("#project-select option").allTextContents());
  await page.locator("#project-select").selectOption(project.id);
  for(const item of designs){
    const title=`${item.spec.name} E2E run`;console.log("run",title);
    await page.locator('[data-view="runs"]').click();
    await page.locator('[data-action="new-run"]').first().click();console.log("modal open");
    await page.locator('#run-form [name="title"]').fill(title);
    await page.locator('#run-form [name="design_id"]').selectOption(item.design.id);
    const response=page.waitForResponse(r=>r.url().includes("/api/records")&&r.request().method()==="POST");
    console.log("submit run");await page.locator('#run-form button[type="submit"]').click();
    const saved=await response;assert.equal(saved.status(),200,await saved.text());
    await page.locator("#modal-root .modal").waitFor({state:"detached"});
    const runCard=page.locator(".item").filter({hasText:title});
    console.log("open operations");await runCard.locator('[data-action="show-operations"]').click();
    await page.locator(`[data-action="select-operation-step"][data-step="${item.checkPath}"]`).click();
    console.log("record step");await page.locator('[data-action="record-operation-step"]').click();await page.waitForSelector('#execution-event-form');
    const form=page.locator("#execution-event-form");
    await form.locator('[name="content"]').fill(`${item.spec.name} check recorded from UI`);
    await form.locator('select[name="check_id"]').selectOption(item.checkId);
    await form.locator('select[name="check_status"]').selectOption("passed");
    await form.locator('[name="checked_by"]').fill("synthetic-operator");
    const write=page.waitForResponse(r=>r.url().includes("/api/records")&&r.request().method()==="POST");
    await form.locator('button[type="submit"]').click();
    const written=await write;assert.equal(written.status(),200,await written.text());
    await page.locator("#modal-root .modal").waitFor({state:"detached"});
    console.log("back");await page.locator('[data-action="operation-back"]').click();
  }
  const state=await page.evaluate(()=>fetch("/api/state").then(r=>r.json()));
  const runs=state.records.filter(r=>r.kind==="run");assert.equal(runs.length,3);
  const checks=state.records.filter(r=>r.kind==="execution_event"&&r.data.checks?.length);
  assert.equal(checks.length,3,JSON.stringify(checks));
  const shotDir=path.join(root,"docs","release","screenshots");fs.mkdirSync(shotDir,{recursive:true});
  await page.evaluate(()=>{document.documentElement.dataset.theme="dark";localStorage.setItem("asterlab.theme","dark")});
  await page.screenshot({path:path.join(shotDir,"2026-11-07-asterlab-1440-dark.png"),fullPage:true});
  await page.locator("#theme-toggle").click();
  assert.equal(await page.evaluate(()=>document.documentElement.dataset.theme),"light");
  await page.screenshot({path:path.join(shotDir,"2026-11-07-asterlab-1440-light.png"),fullPage:true});
  await page.setViewportSize({width:1920,height:1080});
  await page.screenshot({path:path.join(shotDir,"2026-11-07-asterlab-1920-light.png"),fullPage:true});
  await page.locator("#theme-toggle").click();
  assert.equal(await page.evaluate(()=>document.documentElement.dataset.theme),"dark");
  await page.screenshot({path:path.join(shotDir,"2026-11-07-asterlab-1920-dark.png"),fullPage:true});
  assert.deepEqual(errors,[]);
  await browser.close();
  await stop(server);
  server=await start(port);
  const reopened=await (await fetch(base+"/api/state")).json();
  assert.equal(reopened.records.filter(r=>r.kind==="run").length,3);
  assert.equal(reopened.records.filter(r=>r.kind==="execution_event"&&r.data.checks?.length).length,3);
  await stop(server);
  console.log(JSON.stringify({ok:true,protocols:designs.map(x=>x.spec.name),runs:3,checks:3,restart:true,screenshots:shotDir}));
})().catch(async error=>{console.error(error);process.exitCode=1});
