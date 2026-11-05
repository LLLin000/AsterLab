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

    fn run(&self, tool: &str, input: &Value) -> Value {
        let path = self.root.join(format!("{tool}.json"));
        fs::write(&path, serde_json::to_vec(input).unwrap()).unwrap();
        let output = Command::new(bin())
            .arg(tool)
            .args(["--input", path.to_str().unwrap()])
            .env("LABWORKBENCH_DATA_DIR", self.root.join("data"))
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
