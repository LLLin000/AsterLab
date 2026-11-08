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
  const pagesRun=cli("run.create",{design_id:design.id,title:"Pages run"}).result.record;
  const projected=cli("view.project",{project_id:project.id,protocol_version_id:protocol.version.id,scope:{kind:"assay",assay_id:"cck8"}}).result.document;
  const componentsView=cli("view.create",{project_id:project.id,title:"Components view",document:projected}).result;
  const componentRootId=projected.root.workflow.nodes[0].component.instance_id;
  const componentDocumentId=componentsView.version?.data?.document_id;
  const componentOperations=[
    {kind:"add_slot_child",parent_instance_id:componentRootId,slot_id:"detail",child:{instance_id:"test-note",definition:{id:"content.note",version:1},props:{text:"Note *rendered*"},slots:{}}},
    {kind:"add_slot_child",parent_instance_id:componentRootId,slot_id:"detail",child:{instance_id:"test-table",definition:{id:"content.table",version:1},props:{columns:["Item","Value"],rows:[["A","1"]]},slots:{}}},
    {kind:"add_slot_child",parent_instance_id:componentRootId,slot_id:"detail",child:{instance_id:"test-calc",definition:{id:"tool.calculator",version:1},props:{tool_ref:"tool.reagent_total@1"},slots:{}}},
    {kind:"add_slot_child",parent_instance_id:componentRootId,slot_id:"detail",child:{instance_id:"test-plate",definition:{id:"lab.well_plate",version:1},props:{},slots:{}}},
    {kind:"add_slot_child",parent_instance_id:componentRootId,slot_id:"detail",child:{instance_id:"test-flow",definition:{id:"workflow",version:1},props:{},slots:{},workflow:{nodes:[],edges:[]}}}
  ];
  cli("view.revise",{view_id:componentsView.view?.id||componentsView.id,changeset:{changeset_id:"components-smoke",document_id:componentDocumentId,expected_revision:1,operations:componentOperations,provenance:["test"]}});
  const timedBlock=cli("protocol.block.create",{project_id:project.id,title:"Timed protocol",steps:[{kind:"text",use_id:"prep",title:"Prep",instruction:"Prepare once"},{kind:"text",use_id:"read",title:"Timed read",instruction:"Read at each timepoint",time_scope:"per_timepoint"}]}).result;
  const timedProtocol=cli("protocol.create",{project_id:project.id,title:"Timed protocol",root_block_version_id:timedBlock.version.id}).result;
  const timedDesign=cli("design.create",{project_id:project.id,design:{title:"Timed design",group_specs:[{name:"Control",role:"control",condition:"vehicle",independent_n:2}],assays:[{id:"read",title:"Read",endpoint:"OD",method:"plate",unit:"AU",timepoints:["24 h","48 h"],group_names:["Control"],protocol_version_id:timedProtocol.version.id}]}}).result.design;
  const timedRun=cli("run.create",{design_id:timedDesign.id,title:"Timed run"}).result.record;
  const port=await freePort(),base=`http://127.0.0.1:${port}`;const server=await start(port);
  const timedWorkspace=await (await fetch(`${base}/api/runs/${timedRun.id}/workspace`)).json();
  const readTask=timedWorkspace.tasks.find(task => task.step_path === "root/read" && task.timepoint === "24 h");
  const measurementResponse=await fetch(`${base}/api/records`,{method:"POST",headers:{"content-type":"application/json"},body:JSON.stringify({kind:"execution_event",parent_id:timedRun.id,project_id:project.id,data:{event_type:"measurement",content:"roundtrip",protocol_step_path:"root/read",protocol_version_id:timedProtocol.version.id,assay_id:"read",task_key:readTask.task_key,timepoint:"24 h",measurements:[{endpoint:"OD",value:0.82,unit:"AU",sample_id:"Control-1"}]}})});
  assert.equal(measurementResponse.status,200,await measurementResponse.text());
  const roundtrip=await (await fetch(`${base}/api/runs/${timedRun.id}/workspace`)).json();
  const roundtripEvent=roundtrip.events.find(event => event.content === "roundtrip");
  assert.equal(roundtripEvent.measurements[0].value,0.82);
  assert.equal(roundtripEvent.measurements[0].unit,"AU");
  const browser=await chromium.launch({headless:true});const page=await browser.newPage({viewport:{width:1440,height:1000}});const errors=[];page.on("pageerror",e=>errors.push(e.message));
  await page.goto(`${base}/?page=workflow`,{waitUntil:"networkidle"});
  assert.equal(await page.locator(".rail-item[data-page]").count(),4);
  await page.locator('.rail-item[data-page="protocols"]').click();await page.waitForTimeout(200);assert((await page.locator("#page-view").innerText()).includes("Pages protocol"));
  await page.goto(`${base}/?page=runs&run=${pagesRun.id}`,{waitUntil:"networkidle"});
  assert((await page.locator("#page-view").innerText()).includes("Pages run"));
  assert.equal(await page.locator("#canvasPanel").isVisible(), false);
  assert.equal(await page.locator("#detail").isVisible(), false);
  assert((await page.locator(".run-current h2").innerText()).length > 0);
  await page.locator('#actual-form button[type="submit"]').click();
  await page.waitForFunction(() => document.body.innerText.includes("任务 2/2"), null, {timeout: 5000});
  await page.locator('#actual-form button[type="submit"]').click();
  await page.waitForSelector(".run-done", {timeout: 5000});
  await page.goto(`${base}/?page=runs&run=${timedRun.id}`,{waitUntil:"networkidle"});
  const timedText=await page.locator("body").innerText();
  assert(timedText.includes("通用准备"));
  assert(timedText.includes("24 h") && timedText.includes("48 h"));
  assert(timedText.includes("任务 1/2"));
  await page.locator('.rail-item[data-page="tools"]').click();assert((await page.locator("#page-view").innerText()).includes("常用工具"));
  await page.goto(`${base}/?project=${project.id}&design=${design.id}`,{waitUntil:"networkidle"});
  await page.waitForSelector(".wnode");
  await page.locator(".wnode").first().click();
  await page.waitForTimeout(500);
  assert.equal(await page.locator(".cmp-markdown").count(),1);
  assert((await page.locator(".cmp-table").count()) >= 1);
  assert.equal(await page.locator(".cmp-calculator").count(),1);
  assert.equal(await page.locator(".cmp-wellplate").count(),1);
  const plateWidth=await page.locator(".cmp-wellplate").evaluate(e=>e.getBoundingClientRect().width);
  const calcWidth=await page.locator(".cmp-calculator").evaluate(e=>e.getBoundingClientRect().width);
  assert(plateWidth > calcWidth);
  assert((await page.locator(".cards").innerText()).includes("嵌套流程"));
  await page.locator(".cards .cmp-table").first().click();
  await page.waitForTimeout(150);
  assert.equal(await page.locator("#proposal-panel").isVisible(),true);
  await page.mouse.click(600,120);
  await page.waitForTimeout(150);
  assert.equal(await page.locator("#proposal-panel").isVisible(),false);
  await page.goto(`${base}/?project=${project.id}&design=${timedDesign.id}`,{waitUntil:"networkidle"});
  await page.waitForSelector(".wnode",{timeout:5000});
  assert((await page.locator(".wnode").count()) > 0);
  await page.locator("#editWorkflowDesign").click();
  await page.waitForSelector("#designInlineEdit");
  assert.equal(await page.locator(".design-edit-backdrop").count(),1);
  await page.locator("#cancelDesignEdit").click();
  assert.equal(await page.locator("#designInlineEdit").count(),0);
  assert.deepEqual(errors,[]);
  await browser.close();server.kill();console.log(JSON.stringify({ok:true,rail:4,pages:["protocols","runs","tools"],errors}));
})().catch(error=>{console.error(error);process.exitCode=1});
