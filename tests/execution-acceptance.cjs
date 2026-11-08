const assert = require("node:assert/strict");
const {spawnSync} = require("node:child_process");
const fs = require("node:fs");
const os = require("node:os");
const path = require("node:path");

const root = path.resolve(__dirname, "..");
const bin = path.join(root, "target", "release", "labworkbench.exe");
const tempRoot = fs.mkdtempSync(path.join(root, "target", "execution-acceptance-"));
const env = {...process.env, LABWORKBENCH_DATA_DIR: tempRoot, LABWORKBENCH_TRUSTED_WRITER: "1"};

function cli(tool, input, expected = 0) {
  const file = path.join(tempRoot, `${tool.replace(/\W/g, "_")}-${Math.random().toString(16).slice(2)}.json`);
  fs.writeFileSync(file, JSON.stringify(input));
  const run = spawnSync(bin, [tool, "--input", file], {cwd: root, env, encoding: "utf8"});
  fs.rmSync(file, {force: true});
  assert.equal(run.status, expected, `${tool}: ${run.stdout}${run.stderr}`);
  return JSON.parse(run.stdout.trim());
}

function setup(name, assayId, assayTitle, steps) {
  const project = cli("project.create", {name: `${name} project`}).result.record;
  const block = cli("protocol.block.create", {
    project_id: project.id,
    title: `${name} protocol block`,
    steps,
  }).result;
  const protocol = cli("protocol.create", {
    project_id: project.id,
    title: `${name} protocol`,
    root_block_version_id: block.version.id,
  }).result;
  const design = cli("design.create", {
    project_id: project.id,
    design: {
      title: `${name} design`,
      group_specs: [{name: "Control", role: "control", condition: "synthetic control"}],
      assays: [{
        id: assayId,
        title: assayTitle,
        endpoint: "synthetic endpoint",
        method: name,
        unit: "AU",
        timepoints: ["D1"],
        group_names: ["Control"],
        protocol_version_id: protocol.version.id,
      }],
    },
  }).result.design;
  const run = cli("run.create", {design_id: design.id, title: `${name} run`}).result.record;
  const projected = cli("view.project", {
    project_id: project.id,
    protocol_version_id: protocol.version.id,
    scope: {kind: "assay", assay_id: assayId},
  }).result.document;
  const view = cli("view.create", {
    project_id: project.id,
    title: `${name} view`,
    document: projected,
  }).result;
  assert.equal(projected.root.workflow.nodes.length, steps.length);
  steps.forEach((step, index) => assert.equal(projected.root.workflow.nodes[index].component.props.label, step.title));
  return {project, block, protocol, design, run, view, projected};
}

function event(run, protocol, assayId, payload, stepPath = "root/step") {
  return cli("execution_event.create", {
    run_id: run.id,
    protocol_step_path: stepPath,
    protocol_version_id: protocol.version.id,
    assay_id: assayId,
    ...payload,
  }).result.record;
}

// CCK-8: a readout and a derived result must remain separate; no formula is supplied by AsterLab.
{
  const cck8Steps = [
    {kind: "text", use_id: "seed", title: "Seed plate", instruction: "Seed cells and pre-incubate.", checks: [{check_id: "plate_ready", label: "Plate ready", blocking: true}]},
    {kind: "text", use_id: "dose", title: "Dose treatment", instruction: "Add vehicle or treatment."},
    {kind: "text", use_id: "add_cck8", title: "Add CCK-8", instruction: "Add CCK-8 reagent."},
    {kind: "text", use_id: "incubate", title: "Incubate signal", instruction: "Incubate protected from light."},
    {kind: "text", use_id: "mix", title: "Mix and inspect", instruction: "Mix and check for bubbles."},
    {kind: "text", use_id: "read_od", title: "Read OD450", instruction: "Read raw absorbance."},
    {kind: "text", use_id: "delayed", title: "Optional stabilization", instruction: "Optional SDS/HCl branch."},
    {kind: "text", use_id: "calculate", title: "Derive result", instruction: "Apply the stated calculation."},
  ];
  const ctx = setup("CCK-8", "cck8", "CCK-8", cck8Steps);
  assert.equal(ctx.run.data.protocol_manifest[0].steps.length, 8);
  const unauthorizedEnv = {...env};
  delete unauthorizedEnv.LABWORKBENCH_TRUSTED_WRITER;
  const unauthorizedFile = path.join(tempRoot, "unauthorized-run.json");
  fs.writeFileSync(unauthorizedFile, JSON.stringify({design_id: ctx.design.id, title: "Unauthorized run"}));
  const unauthorized = spawnSync(bin, ["run.create", "--input", unauthorizedFile], {cwd: root, env: unauthorizedEnv, encoding: "utf8"});
  fs.rmSync(unauthorizedFile, {force: true});
  assert.equal(unauthorized.status, 2, unauthorized.stdout);
  assert.equal(JSON.parse(unauthorized.stdout).error.code, "INVALID_INPUT");
  assert.equal(ctx.run.data.protocol_manifest[0].steps[0].checks[0].check_id, "plate_ready");
  const checked = event(ctx.run, ctx.protocol, "cck8", {
    event_type: "operation", content: "Plate checked",
    checks: [{check_id: "plate_ready", status: "passed", checked_by: "operator-1"}],
    actual_parameters: [{key: "incubation_time", value: 3, unit: "h", source: "protocol"}],
    controls: [{role: "blank", subtype: "cell_free", status: "passed", experimental_unit: "well B1"}],
  }, "root/seed");
  assert.equal(checked.data.checks[0].status, "passed");
  assert.equal(checked.data.actual_parameters[0].key, "incubation_time");
  assert.equal(checked.data.controls[0].experimental_unit, "well B1");
  assert.equal(checked.data.controls[0].subtype, "cell_free");
  const unknownCheck = cli("execution_event.create", {
    run_id: ctx.run.id, event_type: "operation", content: "Unknown check",
    protocol_step_path: "root/seed", protocol_version_id: ctx.protocol.version.id, assay_id: "cck8",
    checks: [{check_id: "not_in_frozen_step", status: "passed"}],
  }, 2);
  assert.match(unknownCheck.error.message, /不属于冻结步骤/);
  const duplicateCheck = cli("execution_event.create", {
    run_id: ctx.run.id, event_type: "operation", content: "Duplicate check",
    protocol_step_path: "root/seed", protocol_version_id: ctx.protocol.version.id, assay_id: "cck8",
    checks: [{check_id: "plate_ready", status: "passed"}, {check_id: "plate_ready", status: "failed"}],
  }, 2);
  assert.match(duplicateCheck.error.message, /不能重复/);
  const unlinkedCheck = cli("execution_event.create", {
    run_id: ctx.run.id, event_type: "operation", content: "Unlinked check",
    checks: [{check_id: "plate_ready", status: "passed"}],
  }, 2);
  const missingBlockingCompletion = cli("execution_event.create", {
    run_id: ctx.run.id, event_type: "operation", content: "Complete without blocking check",
    protocol_step_path: "root/seed", protocol_version_id: ctx.protocol.version.id, assay_id: "cck8",
    step_status: "completed",
  }, 2);
  assert.match(missingBlockingCompletion.error.message, /缺少 blocking 检查项/);
  const failedBlockingCompletion = cli("execution_event.create", {
    run_id: ctx.run.id, event_type: "operation", content: "Complete with failed blocking check",
    protocol_step_path: "root/seed", protocol_version_id: ctx.protocol.version.id, assay_id: "cck8",
    step_status: "completed",
    checks: [{check_id: "plate_ready", status: "failed"}],
  }, 2);
  assert.match(failedBlockingCompletion.error.message, /必须为 passed/);
  const completed = event(ctx.run, ctx.protocol, "cck8", {
    event_type: "operation", content: "Seed plate completed",
    protocol_step_path: "root/seed", protocol_version_id: ctx.protocol.version.id, assay_id: "cck8",
    step_status: "completed",
    checks: [{check_id: "plate_ready", status: "passed", checked_by: "operator-1"}],
  }, "root/seed");
  assert.equal(completed.data.step_status, "completed");
  const duplicateCompletion = event(ctx.run, ctx.protocol, "cck8", {
    event_type: "operation", content: "Seed plate completed again",
    protocol_step_path: "root/seed", protocol_version_id: ctx.protocol.version.id, assay_id: "cck8",
    step_status: "completed",
    checks: [{check_id: "plate_ready", status: "passed", checked_by: "operator-1"}],
  }, "root/seed");
  assert.equal(duplicateCompletion.id, completed.id);
  const raw = event(ctx.run, ctx.protocol, "cck8", {
    event_type: "measurement", content: "Raw OD",
    readouts: [{readout_id: "OD450:A1", endpoint: "OD450", value: 0.735, unit: "AU", sample_id: "A1", result_kind: "raw"}],
  }, "root/read_od");
  assert.equal(raw.data.readouts[0].value, 0.735);
  assert.equal(raw.data.readouts[0].result_kind, "raw");
  const derived = event(ctx.run, ctx.protocol, "cck8", {
    event_type: "measurement", content: "Blank-corrected viability",
    readouts: [{readout_id: "viability:control", endpoint: "viability", value: 91.2, unit: "%", result_kind: "derived", method: "blank-subtracted ratio", derived_from: ["OD450:A1"]}],
  }, "root/calculate");
  assert.equal(derived.data.readouts[0].result_kind, "derived");
  assert.deepEqual(derived.data.readouts[0].derived_from, ["OD450:A1"]);
  const missingSource = cli("execution_event.create", {
    run_id: ctx.run.id, event_type: "measurement", content: "Ungrounded source",
    protocol_step_path: "root/calculate", protocol_version_id: ctx.protocol.version.id, assay_id: "cck8",
    readouts: [{endpoint: "viability", value: 80, result_kind: "derived", method: "ratio", derived_from: ["missing:readout"]}],
  }, 2);
  assert.match(missingSource.error.message, /来源不存在/);
  const missingMethod = cli("execution_event.create", {
    run_id: ctx.run.id, event_type: "measurement", content: "Ungrounded derived result",
    readouts: [{endpoint: "viability", value: 90, result_kind: "derived", derived_from: ["OD450:A1"]}],
  }, 2);
  assert.equal(missingMethod.error.code, "INVALID_INPUT");
  const workspace = cli("run.workspace", {id: ctx.run.id}).result;
  assert.equal(workspace.events.filter(e => e.readouts.length).length, 2);
}

// qPCR: raw Cq, not-detected, and a derived analysis are distinct persisted values.
{
  const ctx = setup("RT-qPCR", "qpcr", "RT-qPCR", [
    {kind: "text", use_id: "extract", title: "Extract RNA", instruction: "Extract and QC RNA."},
    {kind: "text", use_id: "rt", title: "Reverse transcribe", instruction: "Run RT with controls."},
    {kind: "text", use_id: "qpcr", title: "Set up qPCR", instruction: "Prepare assay, plate, NTC and NRT.", checks: [{check_id: "ntc_clean", label: "NTC clean", blocking: true}]},
    {kind: "text", use_id: "cycle", title: "Run thermal program", instruction: "Run the instrument."},
    {kind: "text", use_id: "analyze", title: "Analyze Cq", instruction: "Review raw Cq before derivation."},
  ]);
  assert.equal(ctx.run.data.protocol_manifest[0].steps.length, 5);
  event(ctx.run, ctx.protocol, "qpcr", {
    event_type: "operation", content: "NTC checked",
    checks: [{check_id: "ntc_clean", status: "passed", checked_by: "operator-2"}],
  }, "root/qpcr");
  const raw = event(ctx.run, ctx.protocol, "qpcr", {
    event_type: "measurement", content: "Cq raw",
    readouts: [
      {readout_id: "Cq:A1", endpoint: "Cq", value: 22.4, result_kind: "raw", sample_id: "A1"},
      {readout_id: "Cq:A2", endpoint: "Cq", qualifier: "not_detected", result_kind: "raw", sample_id: "A2"},
    ],
  }, "root/analyze");
  assert.equal(raw.data.readouts[0].value, 22.4);
  assert.equal(raw.data.readouts[1].value, null);
  assert.equal(raw.data.readouts[1].qualifier, "not_detected");
  const analysis = event(ctx.run, ctx.protocol, "qpcr", {
    event_type: "measurement", content: "Delta Cq analysis",
    readouts: [{readout_id: "delta_Cq:control", endpoint: "delta_Cq", value: 3.1, result_kind: "derived", method: "Cq target - Cq reference", derived_from: ["Cq:A1"]}],
  }, "root/analyze");
  assert.equal(analysis.data.readouts[0].result_kind, "derived");
  const workspace = cli("run.workspace", {id: ctx.run.id}).result;
  const onDisk = workspace.events.flatMap(e => e.readouts);
  assert(onDisk.some(r => r.qualifier === "not_detected"));
  assert(onDisk.some(r => r.result_kind === "derived"));
}

// Biofilm: CV biomass, CFU count, and well→tube→plate lineage are separate facts.
{
  const ctx = setup("Biofilm CV+CFU", "biofilm", "Biofilm CV + CFU", [
    {kind: "text", use_id: "culture", title: "Culture biofilm", instruction: "Inoculate and incubate."},
    {kind: "text", use_id: "wash", title: "Wash biofilm", instruction: "Wash without claiming zero loss.", checks: [{check_id: "sterile_pbs", label: "Sterile PBS for CFU", blocking: true}]},
    {kind: "text", use_id: "cv", title: "CV stain and read", instruction: "Stain, dry, solubilize, read CV."},
    {kind: "text", use_id: "disperse", title: "Disperse biofilm", instruction: "Harvest and disperse with controls."},
    {kind: "text", use_id: "dilute", title: "Serial dilute", instruction: "Dilute with sterile diluent."},
    {kind: "text", use_id: "plate", title: "Plate CFU", instruction: "Plate and incubate."},
    {kind: "text", use_id: "count", title: "Count CFU", instruction: "Count raw colonies."},
    {kind: "text", use_id: "calculate", title: "Calculate CFU/mL", instruction: "Apply stated dilution and volume."},
  ]);
  assert.equal(ctx.run.data.protocol_manifest[0].steps.length, 8);
  event(ctx.run, ctx.protocol, "biofilm", {
    event_type: "operation", content: "CFU branch prepared",
    checks: [{check_id: "sterile_pbs", status: "passed", checked_by: "operator-3"}],
    lineage: [{from_kind: "well", from_id: "A1", to_kind: "tube", to_id: "T1", relation: "transferred"}],
  }, "root/wash");
  const cv = event(ctx.run, ctx.protocol, "biofilm", {
    event_type: "measurement", content: "CV absorbance",
    readouts: [{readout_id: "CV_OD590:A1", endpoint: "CV_OD590", value: 0.42, unit: "AU", result_kind: "raw", sample_id: "A1"}],
  }, "root/cv");
  const cfu = event(ctx.run, ctx.protocol, "biofilm", {
    event_type: "measurement", content: "CFU raw count",
    readouts: [{readout_id: "CFU_count:T1", endpoint: "CFU_count", value: 42, unit: "colonies", result_kind: "raw", sample_id: "T1"}],
    lineage: [{from_kind: "tube", from_id: "T1", to_kind: "plate", to_id: "P1", relation: "plated"}],
  }, "root/count");
  const derived = event(ctx.run, ctx.protocol, "biofilm", {
    event_type: "measurement", content: "CFU/mL",
    readouts: [{readout_id: "CFU_per_mL:T1", endpoint: "CFU_per_mL", value: 4200, unit: "CFU/mL", result_kind: "derived", method: "count / (dilution × plated volume)", derived_from: ["CFU_count:T1"]}],
  }, "root/calculate");
  assert.equal(cv.data.readouts[0].endpoint, "CV_OD590");
  assert.equal(cfu.data.readouts[0].endpoint, "CFU_count");
  assert.equal(derived.data.readouts[0].result_kind, "derived");
  const workspace = cli("run.workspace", {id: ctx.run.id}).result;
  const lineage = workspace.events.flatMap(e => e.lineage);
  assert(lineage.some(edge => edge.from_id === "A1" && edge.to_id === "T1"));
  assert(lineage.some(edge => edge.from_id === "T1" && edge.to_id === "P1"));
  assert(workspace.events.flatMap(e => e.readouts).some(r => r.endpoint === "CV_OD590"));
  assert(workspace.events.flatMap(e => e.readouts).some(r => r.endpoint === "CFU_count"));
}

console.log(JSON.stringify({ok: true, protocols: ["CCK-8", "RT-qPCR", "Biofilm CV+CFU"], tests: ["agent draft", "execution package", "raw/derived", "checks", "lineage"]}));
