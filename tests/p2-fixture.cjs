const assert = require("node:assert/strict");
const {spawnSync} = require("node:child_process");
const fs = require("node:fs");
const os = require("node:os");
const path = require("node:path");

const root = path.resolve(__dirname, "..");
const bin = process.env.LABWORKBENCH_BIN || path.join(root, "target", "release", "labworkbench.exe");
const tempRoot = fs.mkdtempSync(path.join(root, "target", "p2-fixture-"));
const env = {...process.env, LABWORKBENCH_DATA_DIR: path.join(tempRoot, "data")};

function cli(tool, input) {
  const file = path.join(tempRoot, `in-${Math.random().toString(16).slice(2)}.json`);
  fs.writeFileSync(file, JSON.stringify(input), "utf8");
  const run = spawnSync(bin, [tool, "--input", file], {cwd: root, env, encoding: "utf8"});
  fs.rmSync(file, {force: true});
  assert.equal(run.status, 0, `${tool} failed: ${run.stdout}\n${run.stderr}`);
  const out = JSON.parse(run.stdout.trim());
  assert.equal(out.ok, true, `${tool}: ${run.stdout}`);
  return out.result;
}

const project = cli("project.create", {name: "P2 synthetic L929", description: "synthetic fixture; not experimental data"}).record;

const cck8Block = cli("protocol.block.create", {
  project_id: project.id,
  title: "P2 CCK-8 synthetic protocol",
  summary: "Synthetic step timing fixture",
  parameter_slots: [{key: "reagent_volume", label: "CCK-8 加入量", value_type: "number", unit: "µL", required: true}],
  steps: [
    {kind: "text", use_id: "seed", title: "接种细胞", instruction: "Synthetic seeding only.", parameter_refs: [], time_scope: "once"},
    {kind: "text", use_id: "add_reagent", title: "加入 CCK-8", instruction: "Synthetic reagent addition only.", parameter_refs: ["reagent_volume"], time_scope: "per_timepoint", visualization: "transfer"},
    {kind: "text", use_id: "read", title: "读取 OD450", instruction: "Synthetic read only.", parameter_refs: [], time_scope: "per_timepoint", visualization: "measurement"}
  ]
});
const liveBlock = cli("protocol.block.create", {
  project_id: project.id,
  title: "P2 live/dead synthetic protocol",
  summary: "Synthetic step timing fixture",
  steps: [
    {kind: "text", use_id: "seed", title: "接种细胞", instruction: "Synthetic seeding only.", parameter_refs: [], time_scope: "once"},
    {kind: "text", use_id: "stain", title: "染色", instruction: "Synthetic staining only.", parameter_refs: [], time_scope: "per_timepoint", visualization: "incubation"},
    {kind: "text", use_id: "image", title: "成像", instruction: "Synthetic imaging only.", parameter_refs: [], time_scope: "per_timepoint", visualization: "measurement"}
  ]
});
const cck8Protocol = cli("protocol.create", {
  project_id: project.id,
  title: "P2 CCK-8 synthetic method",
  root_block_version_id: cck8Block.version.id,
  root_bindings: [{slot: "reagent_volume", value: 10, unit: "µL"}]
});
const liveProtocol = cli("protocol.create", {
  project_id: project.id,
  title: "P2 live/dead synthetic method",
  root_block_version_id: liveBlock.version.id
});

const design = cli("design.create", {
  project_id: project.id,
  design: {
    title: "P2 L929 synthetic design",
    objective: "Original synthetic objective",
    experimental_unit: "one independent well",
    group_specs: [{id: "g1", name: "Control", role: "control", condition: "synthetic control", independent_n: 3, technical_repeats: 2}],
    assays: [
      {id: "cck8", title: "CCK-8", endpoint: "OD450", method: "CCK-8", unit: "absorbance", timepoints: ["D1", "D3"], timepoint_relation: "same_physical_sample_repeated", group_ids: ["g1"], group_names: ["Control"], protocol_version_id: cck8Protocol.version.id, container_type: "plate", container_format: "96"},
      {id: "live", title: "Live/Dead", endpoint: "fluorescence", method: "Calcein AM/EthD-1", unit: "AU", timepoints: ["D1", "D3"], timepoint_relation: "independent_sample_per_timepoint", group_ids: ["g1"], group_names: ["Control"], protocol_version_id: liveProtocol.version.id, container_type: "plate", container_format: "96"}
    ]
  }
}).design;
const run = cli("run.create", {design_id: design.id, title: "P2 synthetic frozen run"}).record;
const plate = cli("plate.create", {run_id: run.id, assay_id: "cck8", timepoint: "D1", name: "P2 CCK-8 D1 plate", format: "96", rows: 8, columns: 12}).record;
cli("well.set", {plate_id: plate.id, well_id: "A1", group_label: "Control", sample_id: "SYN-A1", note: "synthetic mapping only"});
cli("execution_event.create", {
  run_id: run.id,
  event_type: "operation",
  content: "Synthetic legacy operation event",
  protocol_step_path: "root/seed",
  protocol_version_id: cck8Protocol.version.id,
  assay_id: "cck8",
  timepoint: "D1",
  step_status: "completed"
});

const workspace = cli("run.workspace", {id: run.id});
assert.equal(workspace.design_version_id, run.data.design_version_id);
assert.equal(workspace.frozen_design.content.objective, "Original synthetic objective");
assert.equal(workspace.steps.length, 6);
assert.equal(workspace.steps.find(step => step.scope.kind === "assay" && step.scope.assay_id === "cck8" && step.step.path === "root/seed").step.time_scope, "once");
assert.equal(workspace.steps.find(step => step.step.path === "root/read").step.time_scope, "per_timepoint");
assert.equal(workspace.tasks.filter(task => task.scope.assay_id === "cck8").length, 5);
assert.equal(workspace.tasks.filter(task => task.scope.assay_id === "live").length, 5);
assert.equal(workspace.tasks.filter(task => task.step_path === "root/seed").every(task => task.timepoint === null), true);
assert.equal(workspace.tasks.filter(task => task.scope.assay_id === "cck8" && task.step_path === "root/read").length, 2);
assert.equal(workspace.scopes.find(scope => scope.id === "cck8").protocol_ready, true);
assert.equal(workspace.scopes.find(scope => scope.id === "cck8").resolved_parameters[0].unit, "µL");
assert.equal(workspace.plates[0].wells[0].coordinate, "A1");
assert.equal(workspace.plates[0].wells[0].sample_id, "SYN-A1");
assert.equal(workspace.events[0].occurrence_id, null);
assert.equal(workspace.occurrences.length, 0);

cli("design.revise", {id: design.id, expected_revision: 1, changes: {objective: "Changed current objective"}});
const frozenAgain = cli("run.workspace", {id: run.id});
assert.equal(frozenAgain.frozen_design.content.objective, "Original synthetic objective");
assert.equal(frozenAgain.steps.length, 6);

console.log(JSON.stringify({
  ok: true,
  run_id: run.id,
  steps: workspace.steps.length,
  tasks: workspace.tasks.length,
  cck8_tasks: workspace.tasks.filter(task => task.scope.assay_id === "cck8").length,
  live_tasks: workspace.tasks.filter(task => task.scope.assay_id === "live").length,
  wells: workspace.plates[0].wells.length,
  frozen_objective: frozenAgain.frozen_design.content.objective
}));
