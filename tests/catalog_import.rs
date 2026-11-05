use asterlab_core::operation_module::OperationModule;
use asterlab_core::protocol_block::module_content_digest;
use serde_json::{json, Value};
use std::fs;
use std::path::PathBuf;
use std::process::Command;
use std::time::{SystemTime, UNIX_EPOCH};

fn bin() -> &'static str {
    env!("CARGO_BIN_EXE_labworkbench")
}

struct Cli {
    root: PathBuf,
}

impl Cli {
    fn new(tag: &str) -> Self {
        let root = PathBuf::from(env!("CARGO_MANIFEST_DIR"))
            .join("target")
            .join(format!(
                "asterlab-catalog-{tag}-{}-{}",
                std::process::id(),
                SystemTime::now()
                    .duration_since(UNIX_EPOCH)
                    .unwrap()
                    .as_nanos()
            ));
        fs::create_dir_all(&root).unwrap();
        Self { root }
    }

    fn run_error(&self, tool: &str, input: &Value) -> Value {
        let path = self.root.join(format!("{tool}-error.json"));
        fs::write(&path, serde_json::to_vec(input).unwrap()).unwrap();
        let output = Command::new(bin())
            .arg(tool)
            .args(["--input", path.to_str().unwrap()])
            .env("LABWORKBENCH_DATA_DIR", self.root.join("data"))
            .env("LABWORKBENCH_TRUSTED_WRITER", "1")
            .env("LABWORKBENCH_TRUSTED_WRITER", "1")
            .output()
            .unwrap();
        assert!(!output.status.success(), "{tool} unexpectedly succeeded");
        serde_json::from_slice(&output.stdout).unwrap()
    }

    fn run(&self, tool: &str, input: &Value) -> Value {
        let path = self.root.join(format!("{tool}.json"));
        fs::write(&path, serde_json::to_vec(input).unwrap()).unwrap();
        let output = Command::new(bin())
            .arg(tool)
            .args(["--input", path.to_str().unwrap()])
            .env("LABWORKBENCH_DATA_DIR", self.root.join("data"))
            .env("LABWORKBENCH_TRUSTED_WRITER", "1")
            .env("LABWORKBENCH_TRUSTED_WRITER", "1")
            .output()
            .unwrap();
        assert!(
            output.status.success(),
            "{tool} failed: stdout={} stderr={}",
            String::from_utf8_lossy(&output.stdout),
            String::from_utf8_lossy(&output.stderr)
        );
        serde_json::from_slice(&output.stdout).unwrap()
    }
}

fn evidence(id: &str) -> Value {
    json!({
        "evidence_id": id,
        "kind": "lab_sop",
        "citation": "fixture",
        "url": null,
        "locator": "p1",
        "version": "1",
        "retrieved_at": null
    })
}

fn module_json() -> Value {
    json!({
        "schema_version": 1,
        "identity": {
            "catalog": {"module_id": "op.liquid_transfer", "version": 1},
            "title": "Liquid transfer",
            "action_type": "liquid_transfer",
            "applicability": {
                "assay_families": [],
                "sample_kinds": [],
                "container_kinds": [],
                "variants": [],
                "exclusions": []
            }
        },
        "semantics": {
            "purpose": "Move a measured volume",
            "preconditions": [],
            "inputs": [{
                "port_id": "sample_in",
                "role": "source_sample",
                "object_type": "sample",
                "required": true,
                "cardinality": "one",
                "description": "source"
            }],
            "outputs": [{
                "port_id": "sample_out",
                "role": "destination_sample",
                "object_type": "sample",
                "required": true,
                "cardinality": "one",
                "description": "destination"
            }],
            "sample_transitions": [{
                "transition_id": "t1",
                "input_port_id": "sample_in",
                "output_port_id": "sample_out",
                "effect": "moved"
            }]
        },
        "parameters": [{
            "key": "volume",
            "label": "Volume",
            "value_type": "number",
            "unit": "uL",
            "required": true,
            "required_at": "protocol_freeze",
            "value_source": "protocol_fixed",
            "constraints": [],
            "description": "transfer volume",
            "evidence": [evidence("p1")]
        }],
        "execution": {
            "default_time_scope": "once",
            "allowed_time_scopes": ["once"],
            "instrument_requirements": [],
            "completion_checks": [],
            "record_fields": []
        },
        "knowledge": {
            "controls": [],
            "decision_points": [],
            "failure_modes": [],
            "readout": null,
            "evidence": [evidence("k1")],
            "review_status": "source_checked"
        }
    })
}

fn module_for(module_id: &str, action: &str, title: &str, parameter: &str, unit: &str) -> Value {
    let mut module = module_json();
    module["identity"]["catalog"]["module_id"] = json!(module_id);
    module["identity"]["action_type"] = json!(action);
    module["identity"]["title"] = json!(title);
    module["semantics"]["purpose"] = json!(format!("{title} fixture"));
    module["parameters"][0]["key"] = json!(parameter);
    module["parameters"][0]["label"] = json!(parameter);
    module["parameters"][0]["unit"] = json!(unit);
    module
}

fn import_module(cli: &Cli, project: &str, module_json: &Value) -> Value {
    let module: OperationModule = serde_json::from_value(module_json.clone()).unwrap();
    let review = json!({
        "review_id": format!("review-{}-{}", module.identity.catalog.module_id, module.identity.catalog.version),
        "subject": module.identity.catalog,
        "status": "source_checked",
        "reviewer": "reviewer-1",
        "reviewed_at": "2026-11-05T00:00:00Z",
        "evidence_id": "k1",
        "evidence_locator": "p1",
        "content_digest": module_content_digest(&module)
    });
    let record = cli.run(
        "catalog.review.record",
        &json!({"project_id": project, "review": review}),
    );
    let review_id = record["result"]["record"]["id"]
        .as_str()
        .unwrap()
        .to_owned();
    cli.run(
        "catalog.operation.import",
        &json!({"project_id": project, "module": module_json, "review_id": review_id}),
    )
}

#[test]
fn catalog_import_preview_and_run_freeze_contract() {
    let cli = Cli::new("e2e");
    let project = cli.run("project.create", &json!({"name": "catalog e2e"}))["result"]["record"]
        ["id"]
        .as_str()
        .unwrap()
        .to_owned();

    let module_json = module_json();
    let module: OperationModule = serde_json::from_value(module_json.clone()).unwrap();
    let review = json!({
        "review_id": "review-1",
        "subject": {"module_id": "op.liquid_transfer", "version": 1},
        "status": "source_checked",
        "reviewer": "reviewer-1",
        "reviewed_at": "2026-11-05T00:00:00Z",
        "evidence_id": "k1",
        "evidence_locator": "p1",
        "content_digest": module_content_digest(&module)
    });
    let review_record = cli.run(
        "catalog.review.record",
        &json!({"project_id": project, "review": review}),
    );
    let review_id = review_record["result"]["record"]["id"]
        .as_str()
        .unwrap()
        .to_owned();
    let imported = cli.run(
        "catalog.operation.import",
        &json!({"project_id": project, "module": module_json, "review_id": review_id}),
    );
    let version = &imported["result"]["version"];
    let version_id = version["id"].as_str().unwrap().to_owned();
    assert_eq!(
        version["data"]["module_contract"]["identity"]["catalog"]["module_id"],
        "op.liquid_transfer"
    );
    assert_eq!(version["data"]["catalog_review"]["reviewer"], "reviewer-1");
    assert_eq!(version["data"]["module_origin"], "imported");
    let search = cli.run(
        "semantic.search",
        &json!({"project_id": project, "query": "Liquid"}),
    );
    assert!(!search["result"]["modules"].as_array().unwrap().is_empty());

    let preview = cli.run(
        "protocol.block.preview",
        &json!({"block_version_id": version_id}),
    );
    assert_eq!(
        preview["result"]["module_contracts"]
            .as_array()
            .unwrap()
            .len(),
        1
    );

    let protocol = cli.run(
        "protocol.create",
        &json!({
            "project_id": project,
            "title": "CCK-8 fixture",
            "root_block_version_id": version_id,
            "root_bindings": [{"slot": "volume", "value": 100, "unit": "uL"}]
        }),
    );
    let protocol_version_id = protocol["result"]["version"]["id"]
        .as_str()
        .unwrap()
        .to_owned();
    let design = cli.run(
        "design.create",
        &json!({
            "project_id": project,
            "design": {
                "title": "CCK-8 design",
                "objective": "compare",
                "experimental_unit": "well",
                "group_specs": [{
                    "name": "Control",
                    "role": "control",
                    "condition": "vehicle",
                    "independent_n": 3,
                    "technical_repeats": 1
                }],
                "assays": [{
                    "id": "a1",
                    "title": "CCK-8",
                    "endpoint": "OD450",
                    "method": "CCK-8",
                    "unit": "absorbance",
                    "group_ids": [],
                    "timepoints": [],
                    "protocol_version_id": protocol_version_id
                }]
            }
        }),
    );
    let run = cli.run(
        "run.create",
        &json!({
            "design_id": design["result"]["design"]["id"],
            "title": "CCK-8 run"
        }),
    );
    let manifest = &run["result"]["record"]["data"]["protocol_manifest"][0];
    assert_eq!(
        manifest["module_contracts"][0]["module_contract"]["identity"]["catalog"]["module_id"],
        "op.liquid_transfer"
    );

    let revised = cli.run(
        "protocol.block.revise",
        &json!({
            "block_id": version["data"]["block_id"],
            "expected_version": 1,
            "parameter_slots": version["data"]["parameter_slots"],
            "steps": [{
                "kind": "text",
                "use_id": "execute",
                "title": "Derived transfer",
                "instruction": "Local modification of the imported block."
            }]
        }),
    );
    let revised_data = &revised["result"]["version"]["data"];
    assert_eq!(revised_data["module_origin"], "derived");
    assert!(revised_data["module_contract"].is_object());
    assert!(revised_data.get("catalog_review").is_none());

    let workspace = cli.run(
        "run.workspace",
        &json!({"id": run["result"]["record"]["id"]}),
    );
    assert_eq!(workspace["result"]["steps"].as_array().unwrap().len(), 1);
}

#[test]
fn cck8_multistep_live_dead_and_frozen_run_regression() {
    let cli = Cli::new("multistep");
    let project = cli.run("project.create", &json!({"name": "CCK-8 multistep"}))["result"]
        ["record"]["id"]
        .as_str()
        .unwrap()
        .to_owned();

    let liquid = import_module(
        &cli,
        &project,
        &module_for(
            "op.liquid_transfer",
            "liquid_transfer",
            "Transfer",
            "volume",
            "uL",
        ),
    );
    let incubation = import_module(
        &cli,
        &project,
        &module_for(
            "op.incubation",
            "incubation",
            "Incubation",
            "duration",
            "min",
        ),
    );
    let absorbance = import_module(
        &cli,
        &project,
        &module_for(
            "op.absorbance_measurement",
            "absorbance_measurement",
            "Absorbance",
            "wavelength",
            "nm",
        ),
    );
    let liquid_id = liquid["result"]["version"]["id"].as_str().unwrap();
    let liquid_block_id = liquid["result"]["block"]["id"].as_str().unwrap();
    let incubation_id = incubation["result"]["version"]["id"].as_str().unwrap();
    let absorbance_id = absorbance["result"]["version"]["id"].as_str().unwrap();

    let method = cli.run(
        "protocol.block.create",
        &json!({
            "project_id": project,
            "title": "CCK-8 method",
            "steps": [
                {"kind":"block_use","use_id":"seed","block_version_id":liquid_id,"bindings":[{"slot":"volume","value":100,"unit":"uL"}]},
                {"kind":"block_use","use_id":"treat","block_version_id":liquid_id,"bindings":[{"slot":"volume","value":50,"unit":"uL"}]},
                {"kind":"block_use","use_id":"incubate","block_version_id":incubation_id,"bindings":[{"slot":"duration","value":24,"unit":"min"}]},
                {"kind":"block_use","use_id":"read","block_version_id":absorbance_id,"bindings":[{"slot":"wavelength","value":450,"unit":"nm"}]},
                {"kind":"text","use_id":"note","title":"Method note","instruction":"Method-local instruction."}
            ]
        }),
    );
    let method_version_id = method["result"]["version"]["id"].as_str().unwrap();
    let inspection = cli.run(
        "semantic.inspect",
        &json!({"block_version_id": method_version_id}),
    );
    assert_eq!(inspection["result"]["steps"].as_array().unwrap().len(), 5);
    let patch = json!({
        "changeset_id": "cs-treat-volume",
        "target": {"kind": "block_version", "object_id": method_version_id},
        "expected_version": 1,
        "changes": [{
            "path": "steps.treat.bindings.volume",
            "operation": "set",
            "value": 100.0
        }]
    });
    assert_eq!(
        cli.run("semantic.validate", &json!({"changeset": patch}))["result"]["valid"],
        true
    );
    let proposed = cli.run("semantic.propose_patch", &json!({"changeset": patch}));
    assert_eq!(
        proposed["result"]["summary"]["field_paths"][0],
        "steps.treat.bindings.volume"
    );
    let assessment = cli.run("semantic.assess_patch", &json!({"changeset": patch}));
    assert_eq!(assessment["result"]["changes"][0]["old_value"], 50.0);
    assert_eq!(assessment["result"]["changes"][0]["new_value"], 100.0);
    assert!(assessment["result"]["referencing_refs"]
        .as_array()
        .unwrap()
        .iter()
        .any(|reference| reference
            .as_str()
            .unwrap_or_default()
            .starts_with("block_version:")));
    let preview = cli.run(
        "protocol.block.preview",
        &json!({"block_version_id": method_version_id}),
    );
    assert_eq!(preview["result"]["steps"].as_array().unwrap().len(), 5);
    assert!(preview["result"]["ready"].as_bool().unwrap());
    assert_eq!(
        preview["result"]["module_contracts"]
            .as_array()
            .unwrap()
            .len(),
        3
    );

    let protocol = cli.run(
        "protocol.create",
        &json!({
            "project_id": project,
            "title": "CCK-8 and Live/Dead protocol",
            "root_block_version_id": method_version_id
        }),
    );
    let protocol_version_id = protocol["result"]["version"]["id"].as_str().unwrap();
    let assessment = cli.run("semantic.assess_patch", &json!({"changeset": patch}));
    assert!(assessment["result"]["would_require_rebind"]
        .as_array()
        .unwrap()
        .iter()
        .any(|reference| reference
            .as_str()
            .unwrap_or_default()
            .starts_with("protocol_version:")));
    let design = cli.run(
        "design.create",
        &json!({
            "project_id": project,
            "design": {
                "title": "CCK-8 and live/dead design",
                "objective": "compare",
                "experimental_unit": "well",
                "group_specs": [{
                    "name": "Control", "role": "control", "condition": "vehicle",
                    "independent_n": 3, "technical_repeats": 1
                }],
                "assays": [
                    {"id":"cck8","title":"CCK-8","endpoint":"OD450","method":"CCK-8","unit":"absorbance","group_ids":[],"timepoints":[],"protocol_version_id":protocol_version_id},
                    {"id":"live_dead","title":"Live/Dead","endpoint":"fluorescence","method":"Live/Dead","unit":"RFU","group_ids":[],"timepoints":[],"protocol_version_id":protocol_version_id}
                ]
            }
        }),
    );
    let design_id = design["result"]["design"]["id"].as_str().unwrap();
    let run_one = cli.run(
        "run.create",
        &json!({"design_id": design_id, "title": "run 1"}),
    );
    let run_two = cli.run(
        "run.create",
        &json!({"design_id": design_id, "title": "run 2"}),
    );
    let workspace_one = cli.run(
        "run.workspace",
        &json!({"id": run_one["result"]["record"]["id"]}),
    );
    let workspace_two = cli.run(
        "run.workspace",
        &json!({"id": run_two["result"]["record"]["id"]}),
    );
    assert_eq!(
        workspace_one["result"]["steps"].as_array().unwrap().len(),
        10
    );
    assert_eq!(
        workspace_two["result"]["steps"].as_array().unwrap().len(),
        10
    );
    let scopes = workspace_one["result"]["steps"]
        .as_array()
        .unwrap()
        .iter()
        .filter_map(|step| step["scope"]["assay_id"].as_str())
        .collect::<std::collections::HashSet<_>>();
    assert!(scopes.contains("cck8"));
    assert!(scopes.contains("live_dead"));

    let workspace_before_patch = cli.run(
        "run.workspace",
        &json!({"id": run_one["result"]["record"]["id"]}),
    );
    let applied = cli.run("semantic.apply_binding_patch", &json!({"changeset": patch}));
    assert_eq!(applied["result"]["version"]["data"]["version"], 2);
    assert_eq!(applied["result"]["changes"][0]["old_value"], 50.0);
    assert_eq!(applied["result"]["changes"][0]["new_value"], 100.0);
    let stale_patch = json!({
        "changeset_id": "cs-stale",
        "target": {"kind": "block_version", "object_id": method_version_id},
        "expected_version": 1,
        "changes": [{
            "path": "steps.treat.bindings.volume",
            "operation": "set",
            "value": 25.0
        }]
    });
    let stale = cli.run_error(
        "semantic.apply_binding_patch",
        &json!({"changeset": stale_patch}),
    );
    assert_eq!(stale["error"]["code"], "REVISION_CONFLICT");
    let new_method_version_id = applied["result"]["version"]["id"].as_str().unwrap();
    let bad_patch = json!({
        "changeset_id": "cs-bad-type",
        "target": {"kind": "block_version", "object_id": new_method_version_id},
        "expected_version": 2,
        "changes": [{
            "path": "steps.treat.bindings.volume",
            "operation": "set",
            "value": "not-a-number"
        }]
    });
    let error = cli.run_error(
        "semantic.apply_binding_patch",
        &json!({"changeset": bad_patch}),
    );
    assert_eq!(error["error"]["code"], "INVALID_INPUT");
    let workspace_after_patch = cli.run(
        "run.workspace",
        &json!({"id": run_one["result"]["record"]["id"]}),
    );
    assert_eq!(workspace_before_patch, workspace_after_patch);

    let frozen_one = run_one["result"]["record"]["data"].clone();
    assert_eq!(
        frozen_one["protocol_manifest"][0]["module_contracts"]
            .as_array()
            .unwrap()
            .len(),
        3
    );

    let revised = cli.run(
        "protocol.block.revise",
        &json!({
            "block_id": liquid_block_id,
            "expected_version": 1,
            "steps": [{"kind":"text","use_id":"revised","title":"Derived method","instruction":"Changed locally."}]
        }),
    );
    assert_eq!(
        revised["result"]["version"]["data"]["module_origin"],
        "derived"
    );
    assert_eq!(
        revised["result"]["version"]["data"]["module_contract_authority"],
        "source_only"
    );
    let workspace_after = cli.run(
        "run.workspace",
        &json!({"id": run_one["result"]["record"]["id"]}),
    );
    assert_eq!(workspace_one, workspace_after);

    let upgraded = import_module(&cli, &project, &{
        let mut value = module_for(
            "op.liquid_transfer",
            "liquid_transfer",
            "Transfer",
            "volume",
            "uL",
        );
        value["identity"]["catalog"]["version"] = json!(2);
        value
    });
    assert_eq!(
        upgraded["result"]["version"]["data"]["module_contract"]["identity"]["catalog"]["version"],
        2
    );
    let old_manifest = cli.run(
        "run.workspace",
        &json!({"id": run_one["result"]["record"]["id"]}),
    );
    assert_eq!(old_manifest, workspace_after);
}

#[test]
fn view_document_persists_and_revises_against_real_core_refs() {
    let cli = Cli::new("view");
    let project = cli.run("project.create", &json!({"name": "view e2e"}))["result"]["record"]["id"]
        .as_str()
        .unwrap()
        .to_owned();
    let imported = import_module(
        &cli,
        &project,
        &module_for(
            "op.liquid_transfer",
            "liquid_transfer",
            "Transfer",
            "volume",
            "uL",
        ),
    );
    let block_version_id = imported["result"]["version"]["id"].as_str().unwrap();
    let protocol = cli.run(
        "protocol.create",
        &json!({
            "project_id": project,
            "title": "View protocol",
            "root_block_version_id": block_version_id,
            "root_bindings": [{"slot":"volume","value":100,"unit":"uL"}]
        }),
    );
    let protocol_version_id = protocol["result"]["version"]["id"].as_str().unwrap();
    let design = cli.run(
        "design.create",
        &json!({
            "project_id": project,
            "design": {
                "title": "View design",
                "objective": "test",
                "experimental_unit": "well",
                "group_specs": [{"name":"Control","role":"control","condition":"vehicle","independent_n":3,"technical_repeats":1}],
                "assays": [{"id":"cck8","title":"CCK-8","endpoint":"OD450","method":"CCK-8","unit":"absorbance","group_ids":[],"timepoints":[],"protocol_version_id":protocol_version_id}],
                "container_plan": {
                    "containers": [{"id":"plate-1","profile_ref":{"id":"labware.cell_culture_96","version":1},"label":"Plate 1"}],
                    "assignments": [{"container_id":"plate-1","position_id":"A1","targets":[{"target":{"kind":"sample","id":"s1"},"role":"primary","relationship":"contains","counts_as_independent_unit":true}]}],
                    "placements": []
                }
            }
        }),
    );
    let design_version_id = design["result"]["version"]["id"].as_str().unwrap();
    let component_list = cli.run("component.list", &json!({}));
    let implemented = component_list["result"]["definitions"]
        .as_array()
        .unwrap()
        .iter()
        .filter(|definition| definition["implemented"] == true)
        .count();
    assert_eq!(implemented, 6);
    assert!(component_list["result"]["definitions"]
        .as_array()
        .unwrap()
        .iter()
        .any(|definition| definition["id"] == "content.markdown"
            && definition["implemented"] == false));

    let projected = cli.run(
        "view.project",
        &json!({
            "project_id": project,
            "protocol_version_id": protocol_version_id,
            "scope": {"kind":"assay","assay_id":"cck8"}
        }),
    );
    assert_eq!(
        projected["result"]["document"]["root"]["workflow"]["nodes"]
            .as_array()
            .unwrap()
            .len(),
        1
    );

    let custom = json!({
        "definition": {
            "id": "lab.solution_calculator",
            "version": 1,
            "family": "tool",
            "title": "Solution calculator",
            "props_schema": {"type":"object"},
            "capabilities": ["calculate"],
            "inputs": [],
            "outputs": [],
            "slots": [],
            "agent_operations": ["update"]
        }
    });
    cli.run("component.define", &custom);

    let mut document = json!({
        "schema_version": 2,
        "document_id": "view-doc-1",
        "revision": 1,
        "root": {
            "instance_id": "view_root",
            "definition": {"id":"workflow","version":1},
            "model_ref": {"kind":"protocol_version","id":protocol_version_id},
            "props": {},
            "slots": {},
            "workflow": {
                "nodes": [{
                    "node_id": "step",
                    "component": {
                        "instance_id": "step",
                        "definition": {"id":"operation","version":1},
                        "model_ref": {
                            "kind":"operation_use",
                            "block_version_id": block_version_id,
                            "step": {
                                "protocol_version_id": protocol_version_id,
                                "scope": {"kind":"assay","assay_id":"cck8"},
                                "step_path": "root/execute"
                            }
                        },
                        "props": {},
                        "slots": {"detail": [
                            {
                                "instance_id": "concentration_table",
                                "definition": {"id":"layout.table","version":1},
                                "props": {"rows": []},
                                "slots": {}
                            },
                            {
                                "instance_id": "note",
                                "definition": {"id":"content.note","version":1},
                                "props": {"text": "old"},
                                "slots": {}
                            }
                        ]},
                        "workflow": null
                    }
                }],
                "edges": []
            }
        },
        "bindings": [{
            "binding_id": "design-to-table",
            "source": {"kind":"model","model":{"kind":"design_version","id":design_version_id},"path":"assays"},
            "target": {"instance_id":"concentration_table","port_id":"data"},
            "value_type": "data"
        }]
    });
    document["root"]["workflow"]["nodes"][0]["component"]["slots"]["detail"]
        .as_array_mut()
        .unwrap()
        .push(json!({
            "instance_id": "planned_plate",
            "definition": {"id":"lab.well_plate","version":1},
            "model_ref": {"kind":"planned_container","design_version_id":design_version_id,"container_id":"plate-1"},
            "props": {},
            "slots": {}
        }));
    document["root"]["workflow"]["nodes"][0]["component"]["slots"]["detail"]
        .as_array_mut()
        .unwrap()
        .push(json!({
            "instance_id": "volume_table",
            "definition": {"id":"content.table","version":1},
            "props": {"mode":"authored","rows":[[10.0]]},
            "slots": {}
        }));
    document["root"]["workflow"]["nodes"][0]["component"]["slots"]["detail"]
        .as_array_mut()
        .unwrap()
        .push(json!({
            "instance_id": "reagent_calculator",
            "definition": {"id":"tool.calculator","version":1},
            "props": {"tool_ref":"tool.reagent_total@1"},
            "slots": {}
        }));
    document["bindings"].as_array_mut().unwrap().push(json!({
        "binding_id":"table-to-calculator",
        "source":{"kind":"component_port","instance_id":"volume_table","port_id":"values"},
        "target":{"instance_id":"reagent_calculator","port_id":"values"},
        "value_type":"data"
    }));
    document["root"]["workflow"]["nodes"][0]["component"]["slots"]["detail"]
        .as_array_mut()
        .unwrap()
        .push(json!({
            "instance_id": "solution_tool",
            "definition": {"id":"lab.solution_calculator","version":1},
            "props": {},
            "slots": {}
        }));
    let other_project = cli.run("project.create", &json!({"name":"other"}))["result"]["record"]
        ["id"]
        .as_str()
        .unwrap()
        .to_owned();
    let other_design = cli.run(
        "design.create",
        &json!({
            "project_id": other_project,
            "design": {
                "title": "Other design", "objective": "x", "experimental_unit": "well",
                "group_specs": [{"name":"Control","role":"control","condition":"vehicle","independent_n":1,"technical_repeats":1}],
                "assays": []
            }
        }),
    );
    let mut cross_project = document.clone();
    cross_project["bindings"][0]["source"]["model"]["id"] =
        other_design["result"]["version"]["id"].clone();
    let error = cli.run_error(
        "view.create",
        &json!({"project_id": project, "title": "Cross project view", "document": cross_project}),
    );
    assert_eq!(error["error"]["code"], "INVALID_INPUT");

    let mut bad_scope = document.clone();
    bad_scope["root"]["workflow"]["nodes"][0]["component"]["model_ref"]["step"]["scope"]
        ["assay_id"] = json!("missing-assay");
    let error = cli.run_error(
        "view.create",
        &json!({"project_id": project, "title": "Bad scope view", "document": bad_scope}),
    );
    assert_eq!(error["error"]["code"], "INVALID_INPUT");

    let mut bad_step = document.clone();
    bad_step["root"]["workflow"]["nodes"][0]["component"]["model_ref"]["step"]["step_path"] =
        json!("root/does-not-exist");
    let error = cli.run_error(
        "view.create",
        &json!({"project_id": project, "title": "Bad step view", "document": bad_step}),
    );
    assert_eq!(error["error"]["code"], "INVALID_INPUT");

    let created = cli.run(
        "view.create",
        &json!({"project_id": project, "title": "View", "document": document}),
    );
    let view_id = created["result"]["view"]["id"].as_str().unwrap();
    let data_preview = cli.run(
        "view.data_preview",
        &json!({
            "view_id": view_id,
            "instance_id": "planned_plate",
            "calculator_instance_id": "reagent_calculator",
            "extra_ratio": 0.1
        }),
    );
    assert_eq!(data_preview["result"]["calculation"]["total_volume"], 11.0);
    assert_eq!(data_preview["result"]["table"]["rows"][0][3], 11.0);
    let missing = cli.run(
        "view.data_preview",
        &json!({"view_id": view_id, "instance_id": "planned_plate"}),
    );
    assert_eq!(missing["result"]["calculation"]["state"], "unresolved");
    let fetched = cli.run("view.get", &json!({"id": view_id}));
    assert_eq!(fetched["result"]["latest_version"]["data"]["revision"], 1);

    let revised = cli.run(
        "view.revise",
        &json!({
            "view_id": view_id,
            "changeset": {
                "changeset_id": "view-cs-1",
                "document_id": "view-doc-1",
                "expected_revision": 1,
                "operations": [{
                    "kind": "update_props",
                    "instance_id": "note",
                    "property": "text",
                    "value": "new"
                }],
                "provenance": ["test"]
            }
        }),
    );
    assert_eq!(revised["result"]["version"]["data"]["revision"], 2);
    let fetched = cli.run("view.get", &json!({"id": view_id}));
    assert_eq!(fetched["result"]["versions"].as_array().unwrap().len(), 2);
    assert_eq!(
        fetched["result"]["latest_version"]["data"]["root"]["workflow"]["nodes"][0]["component"]
            ["slots"]["detail"][1]["props"]["text"],
        "new"
    );
    let old = fetched["result"]["versions"]
        .as_array()
        .unwrap()
        .iter()
        .find(|version| version["data"]["revision"] == 1)
        .unwrap();
    assert_eq!(
        old["data"]["root"]["workflow"]["nodes"][0]["component"]["slots"]["detail"][1]["props"]
            ["text"],
        "old"
    );

    let stale = cli.run_error(
        "view.revise",
        &json!({
            "view_id": view_id,
            "changeset": {
                "changeset_id": "view-cs-stale",
                "document_id": "view-doc-1",
                "expected_revision": 1,
                "operations": [{
                    "kind": "update_props",
                    "instance_id": "note",
                    "property": "text",
                    "value": "stale"
                }],
                "provenance": ["test"]
            }
        }),
    );
    assert_eq!(stale["error"]["code"], "REVISION_CONFLICT");
}

#[test]
fn design_container_plan_freezes_into_run() {
    let cli = Cli::new("containers");
    let project = cli.run("project.create", &json!({"name":"containers"}))["result"]["record"]
        ["id"]
        .as_str()
        .unwrap()
        .to_owned();
    let design = cli.run(
        "design.create",
        &json!({
            "project_id": project,
            "design": {
                "title": "Container design",
                "objective": "test",
                "experimental_unit": "well",
                "group_specs": [],
                "assays": [],
                "container_plan": {
                    "containers": [{
                        "id": "plate-1",
                        "profile_ref": {"id":"labware.cell_culture_96","version":1},
                        "label": "Plate 1"
                    }],
                    "assignments": [{
                        "container_id":"plate-1",
                        "position_id":"A1",
                        "targets":[{
                            "target":{"kind":"sample","id":"s1"},
                            "role":"primary",
                            "relationship":"contains",
                            "counts_as_independent_unit":true
                        }]
                    }],
                    "placements": []
                }
            }
        }),
    );
    let design_id = design["result"]["design"]["id"].as_str().unwrap();
    let revised = cli.run(
        "design.revise",
        &json!({
            "id": design_id,
            "expected_revision": 1,
            "changes": {
                "container_plan": {
                    "containers": [{
                        "id": "plate-1",
                        "profile_ref": {"id":"labware.cell_culture_96","version":1},
                        "label": "Plate 1"
                    }],
                    "assignments": [{
                        "container_id":"plate-1",
                        "position_id":"A2",
                        "targets":[{
                            "target":{"kind":"sample","id":"s1"},
                            "role":"primary",
                            "relationship":"contains",
                            "counts_as_independent_unit":true
                        }]
                    }],
                    "placements": []
                }
            }
        }),
    );
    assert_eq!(revised["result"]["record"]["revision"], 2);
    let run = cli.run(
        "run.create",
        &json!({"design_id": design_id, "title": "container run"}),
    );
    assert_eq!(
        run["result"]["record"]["data"]["container_plan"]["containers"][0]["profile_ref"]["id"],
        "labware.cell_culture_96"
    );
    assert_eq!(
        run["result"]["record"]["data"]["container_plan"]["assignments"][0]["position_id"],
        "A2"
    );
    assert_eq!(
        run["result"]["record"]["data"]["labware_profiles"][0]["profile"]["id"],
        "labware.cell_culture_96"
    );
}

#[test]
fn custom_labware_profile_is_loaded_for_design_validation() {
    let cli = Cli::new("labware");
    let project = cli.run("project.create", &json!({"name":"labware"}))["result"]["record"]["id"]
        .as_str()
        .unwrap()
        .to_owned();
    cli.run(
        "labware.define",
        &json!({"profile": {
            "profile": {"id":"labware.local_96","version":1},
            "family": "cell_culture_plate",
            "title": "Local 96 plate",
            "fields": {
                "rows": {"state":"known","value":8},
                "columns": {"state":"known","value":12},
                "coordinate_system": {"state":"known","value":"A1 row-major"},
                "culture_area": {"state":"unknown"}
            },
            "review_status": "research_draft"
        }}),
    );
    let design = cli.run(
        "design.create",
        &json!({
            "project_id": project,
            "design": {
                "title": "Local profile design", "objective": "x", "experimental_unit": "well",
                "group_specs": [], "assays": [],
                "container_plan": {
                    "containers": [{
                        "id":"plate-local",
                        "profile_ref":{"id":"labware.local_96","version":1},
                        "label":"Local plate"
                    }],
                    "assignments": [], "placements": []
                }
            }
        }),
    );
    assert_eq!(
        design["result"]["version"]["data"]["snapshot"]["container_plan"]["containers"][0]
            ["profile_ref"]["id"],
        "labware.local_96"
    );
}
