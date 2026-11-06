const assert = require("node:assert/strict");
const {spawnSync} = require("node:child_process");
const fs = require("node:fs");
const os = require("node:os");
const path = require("node:path");

const root = path.resolve(__dirname, "..");
const bin = path.join(root, "target", "release", "labworkbench.exe");
const tempRoot = fs.mkdtempSync(path.join(root, "target", "execution-acceptance-"));
const env = {...process.env, LABWORKBENCH_DATA_DIR: tempRoot};

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
  assert.equal(projected.root.workflow.nodes.length, 1);
  assert.equal(projected.root.workflow.nodes[0].component.props.label, steps[0].title);
  return {project, block, protocol, design, run, view, projected};
}

function event(run, protocol, assayId, payload) {
  return cli("execution_event.create", {
    run_id: run.id,
    protocol_step_path: "root/step",
    protocol_version_id: protocol.version.id,
    assay_id: assayId,
    ...payload,
  }).result.record;
}

// CCK-8: a readout and a derived result must remain separate; no formula is supplied by AsterLab.
{
  const ctx = setup("CCK-8", "cck8", "CCK-8", [{
    kind: "text", use_id: "step", title: "Read CCK-8", instruction: "Read absorbance.",
    checks: [{check_id: "plate_ready", label: "Plate ready", blocking: true}],
  }]);
  assert.equal(ctx.run.data.protocol_manifest[0].steps[0].checks[0].check_id, "plate_ready");
  assert.equal(ctx.projected.root.workflow.nodes[0].component.props.label, "Read CCK-8");
  const checked = event(ctx.run, ctx.protocol, "cck8", {
    event_type: "operation", content: "Plate checked",
    checks: [{check_id: "plate_ready", status: "passed", checked_by: "operator-1"}],
    actual_parameters: [{key: "incubation_time", value: 3, unit: "h", source: "protocol"}],
    controls: [{role: "blank", status: "passed", experimental_unit: "well B1"}],
  });
  assert.equal(checked.data.checks[0].status, "passed");
  assert.equal(checked.data.actual_parameters[0].key, "incubation_time");
  assert.equal(checked.data.controls[0].experimental_unit, "well B1");
  const raw = event(ctx.run, ctx.protocol, "cck8", {
    event_type: "measurement", content: "Raw OD",
    readouts: [{endpoint: "OD450", value: 0.735, unit: "AU", sample_id: "A1", result_kind: "raw"}],
  });
  assert.equal(raw.data.readouts[0].value, 0.735);
  assert.equal(raw.data.readouts[0].result_kind, "raw");
  const derived = event(ctx.run, ctx.protocol, "cck8", {
    event_type: "measurement", content: "Blank-corrected viability",
    readouts: [{endpoint: "viability", value: 91.2, unit: "%", result_kind: "derived", method: "blank-subtracted ratio", derived_from: ["OD450:A1"]}],
  });
  assert.equal(derived.data.readouts[0].result_kind, "derived");
  assert.deepEqual(derived.data.readouts[0].derived_from, ["OD450:A1"]);
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
  const ctx = setup("RT-qPCR", "qpcr", "RT-qPCR", [{
    kind: "text", use_id: "step", title: "Run qPCR", instruction: "Run the thermal program.",
    checks: [{check_id: "ntc_clean", label: "NTC clean", blocking: true}],
  }]);
  event(ctx.run, ctx.protocol, "qpcr", {
    event_type: "operation", content: "NTC checked",
    checks: [{check_id: "ntc_clean", status: "passed", checked_by: "operator-2"}],
  });
  const raw = event(ctx.run, ctx.protocol, "qpcr", {
    event_type: "measurement", content: "Cq raw",
    readouts: [
      {endpoint: "Cq", value: 22.4, result_kind: "raw", sample_id: "A1"},
      {endpoint: "Cq", qualifier: "not_detected", result_kind: "raw", sample_id: "A2"},
    ],
  });
  assert.equal(raw.data.readouts[0].value, 22.4);
  assert.equal(raw.data.readouts[1].value, null);
  assert.equal(raw.data.readouts[1].qualifier, "not_detected");
  const analysis = event(ctx.run, ctx.protocol, "qpcr", {
    event_type: "measurement", content: "Delta Cq analysis",
    readouts: [{endpoint: "delta_Cq", value: 3.1, result_kind: "derived", method: "Cq target - Cq reference", derived_from: ["Cq:A1"]}],
  });
  assert.equal(analysis.data.readouts[0].result_kind, "derived");
  const workspace = cli("run.workspace", {id: ctx.run.id}).result;
  const onDisk = workspace.events.flatMap(e => e.readouts);
  assert(onDisk.some(r => r.qualifier === "not_detected"));
  assert(onDisk.some(r => r.result_kind === "derived"));
}

// Biofilm: CV biomass, CFU count, and well→tube→plate lineage are separate facts.
{
  const ctx = setup("Biofilm CV+CFU", "biofilm", "Biofilm CV + CFU", [{
    kind: "text", use_id: "step", title: "Wash and disperse", instruction: "Wash, disperse, dilute, plate.",
    checks: [{check_id: "sterile_pbs", label: "Sterile PBS for CFU", blocking: true}],
  }]);
  event(ctx.run, ctx.protocol, "biofilm", {
    event_type: "operation", content: "CFU branch prepared",
    checks: [{check_id: "sterile_pbs", status: "passed", checked_by: "operator-3"}],
    lineage: [{from_kind: "well", from_id: "A1", to_kind: "tube", to_id: "T1", relation: "transferred"}],
  });
  const cv = event(ctx.run, ctx.protocol, "biofilm", {
    event_type: "measurement", content: "CV absorbance",
    readouts: [{endpoint: "CV_OD590", value: 0.42, unit: "AU", result_kind: "raw", sample_id: "A1"}],
  });
  const cfu = event(ctx.run, ctx.protocol, "biofilm", {
    event_type: "measurement", content: "CFU raw count",
    readouts: [{endpoint: "CFU_count", value: 42, unit: "colonies", result_kind: "raw", sample_id: "T1"}],
    lineage: [{from_kind: "tube", from_id: "T1", to_kind: "plate", to_id: "P1", relation: "plated"}],
  });
  const derived = event(ctx.run, ctx.protocol, "biofilm", {
    event_type: "measurement", content: "CFU/mL",
    readouts: [{endpoint: "CFU_per_mL", value: 4200, unit: "CFU/mL", result_kind: "derived", method: "count / (dilution × plated volume)", derived_from: ["CFU_count:T1"]}],
  });
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
