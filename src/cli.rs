use asterlab_core::agent_edit::{
    AgentEditError, AgentEditErrorCode, ConfirmRequest, Proposal,
};
use asterlab_core::agent_edit_commit::{commit as commit_agent_edit_request, CommitError, CommitStore, CommittedVersion};
use asterlab_core::agent_edit_validate::{
    validate as validate_agent_edit_request, EditTargetStore, StoredBlockVersion,
    StoredProtocolVersion,
};
use asterlab_core::assets as asset_refs;
use asterlab_core::asterdoc_path as ast;
use asterlab_core::calculators::{
    cell_seeding as cell_seeding_calc, gel_mastermix as gel_mastermix_calc,
    in_vivo_dosing as in_vivo_dosing_calc,
    runtime_normalize as runtime_normalize_calc,
    sample_normalize_batch as sample_normalize_batch_calc, CellSeedingGroup, CellSeedingInput,
    GelMastermixGroup, GelMastermixInput, InVivoDosingInput, MassConcentration,
    MassConcentrationUnit, RuntimeNormalizeInput, SampleMeasurement, SampleNormalizeBatchInput,
};
use asterlab_core::component_data::{
    calculate_reagent, project_well_plate, reagent_table, run_reagent_tool,
};
use asterlab_core::diagnostic::{Diagnostic, DiagnosticSeverity};
use asterlab_core::component_framework::{
    default_registry, ComponentBindingContext, ComponentDefinition, ComponentDefinitionRef,
    ComponentInstance, ComponentRegistry, ContextValidationMode, ModelPathResolution, ModelRef,
    ViewChangeSet, ViewDocument, WorkflowEdge, WorkflowGraph, WorkflowNode,
};
use asterlab_core::labware::{
    default_labware_registry, ContainerPlan, LabwareProfile, LabwareRegistry, WellRole,
};
use asterlab_core::operation_module::{
    ModuleBinding, ModuleOrigin, OperationModule, ParameterBinding, ParameterValueType,
    ProtocolBlockVersionRef, ValueSource,
};
use asterlab_core::pantry::{self, CalendarDate, PantryDraft, PantryItem};
use asterlab_core::readout_records::{
    plan_readout_submission, ReadoutRegistry, ReadoutSubmission, ReadoutSubmissionIssue,
};
use asterlab_core::readout_schema::{Readout, ReadoutSchema};
use asterlab_core::readout_submit::{
    plan_batch as plan_readout_batch, plan_submit as plan_readout_submit, ReadoutBatchRequest,
    ReadoutSubmitRequest,
};
use asterlab_core::protocol_block::ReviewRecord;
use asterlab_core::protocol_block::{ParameterSlot, ProtocolBlockVersionData, ProtocolStep, StepBinding};
use asterlab_core::protocol_preview::{
    OperationStep, ProtocolPreview, ResolvedParameter, UnresolvedParameter,
};
use asterlab_core::semantic_catalog::{compile_operation_module, CompilePolicy};
use asterlab_core::semantic_changeset::ChangeSet;
use asterlab_core::semantic_component::ProtocolScope;
use asterlab_core::semantic_query::{assess_block_patch, inspect_block, search_modules};
use asterlab_core::step_edit::{
    commit as step_edit_commit_request, validate as step_edit_validate_request, CommittedStepEdit,
    StepEditCommitError, StepEditCommitStore, StepEditRequest, StepEditStore, StoredBlockHead,
    StoredEditBlock,
};
use asterlab_core::workflow_tree::{
    BlockDefinition, BlockParameter, BlockStep, DesignStep, Document, RunTask,
};
use chrono::{DateTime, NaiveDate, Utc};
use sha2::{Digest, Sha256};
use rusqlite::{params, Connection, OptionalExtension, Transaction, TransactionBehavior};
use serde::{de::DeserializeOwned, Deserialize, Serialize};
use serde_json::{json, Value};
use std::{
    collections::{HashMap, HashSet},
    fs,
    io::{self, Read},
    path::{Path, PathBuf},
};
use uuid::Uuid;

const INPUT_LIMIT: u64 = 2 * 1024 * 1024;

const RECORDS_SCHEMA_VERSION: i64 = 1;

mod common {
    include!("cli/common.rs");
}
mod design_types {
    include!("cli/design_types.rs");
}
mod protocol_block {
    include!("cli/protocol_block.rs");
}
mod asterdoc {
    include!("cli/asterdoc.rs");
}
mod run_write {
    include!("cli/run_write.rs");
}
mod readout {
    include!("cli/readout.rs");
}
mod protocol_views {
    include!("cli/protocol_views.rs");
}
mod run_read {
    include!("cli/run_read.rs");
}
mod store {
    include!("cli/store.rs");
}
mod design_run {
    include!("cli/design_run.rs");
}
mod entry {
    include!("cli/entry.rs");
}
mod agent_edit {
    include!("cli/agent_edit.rs");
}
mod pantry_ledger {
    include!("cli/pantry_ledger.rs");
}
mod dispatch {
    include!("cli/dispatch.rs");
}
mod schemas {
    include!("cli/schemas.rs");
}
mod assets {
    include!("cli/assets.rs");
}
pub(crate) use self::common::*;
pub(crate) use self::design_types::*;
pub(crate) use self::protocol_block::*;
pub(crate) use self::asterdoc::*;
pub(crate) use self::run_write::*;
pub(crate) use self::readout::*;
pub(crate) use self::protocol_views::*;
pub(crate) use self::run_read::*;
pub(crate) use self::store::*;
pub(crate) use self::design_run::*;
pub(crate) use self::entry::*;
pub(crate) use self::agent_edit::*;
pub(crate) use self::pantry_ledger::*;
pub(crate) use self::dispatch::*;
pub(crate) use self::schemas::*;
pub(crate) use self::assets::*;

#[cfg(test)]
mod tests {
    use super::*;

    fn step_paths_by_id(source: &str) -> std::collections::HashMap<String, String> {
        asterdoc_blocks(source).unwrap().into_iter()
            .filter(|block| yaml_kind(&block.value) == "step")
            .map(|block| (yaml_id(&block.value), step_path(&block.value)))
            .collect()
    }

    #[test]
    fn protocol_version_queries_return_expansion_and_stable_not_found() {
        let dir = std::env::temp_dir().join(format!("protocol-version-query-{}", Uuid::new_v4()));
        std::fs::create_dir_all(&dir).unwrap();
        let db = open_db(&dir).unwrap();
        let project = create_project(&db, "query test").unwrap();
        let project_id = project["id"].as_str().unwrap().to_owned();
        let block = {
            let tx = db.unchecked_transaction().unwrap();
            let block = create_protocol_block_records(
                &tx,
                &project_id,
                &json!({
                    "title":"root",
                    "parameter_slots":[{"key":"volume","label":"Volume","value_type":"number","unit":"uL","required":true}],
                    "steps":[{"kind":"text","use_id":"add","title":"Add","instruction":"Add liquid","parameter_refs":["volume"]}]
                }),
            ).unwrap();
            tx.commit().unwrap();
            block
        };
        let block_version_id = block["version"]["id"].as_str().unwrap();
        let (protocol_id, version_id) = {
            let tx = db.unchecked_transaction().unwrap();
            let protocol = insert(&tx, "protocol", Some(&project_id), Some(&project_id), 1, &json!({"title":"protocol"})).unwrap();
            let protocol_id = protocol["id"].as_str().unwrap().to_owned();
            let version = insert(
                &tx, "protocol_version", Some(&project_id), Some(&protocol_id), 1,
                &json!({"protocol_id":protocol_id,"version":1,"body":"","root_block_version_id":block_version_id,"root_bindings":[]}),
            ).unwrap();
            tx.commit().unwrap();
            (protocol_id, version["id"].as_str().unwrap().to_owned())
        };
        let get_input = dir.join("get.json");
        std::fs::write(&get_input, json!({"protocol_version_id":version_id}).to_string()).unwrap();
        let version = dispatch("protocol.version.get", get_input.to_str().unwrap(), &dir).unwrap();
        assert_eq!(version["protocol_id"], protocol_id);
        assert_eq!(version["version"], 1);
        assert_eq!(version["root_block_version_id"], block_version_id);
        assert_eq!(version["steps"][0]["path"], "root/add");
        assert_eq!(version["parameter_slots"][0]["key"], "volume");
        assert_eq!(version["frozen"], true);
        let list_input = dir.join("list.json");
        std::fs::write(&list_input, json!({"protocol_id":protocol_id}).to_string()).unwrap();
        assert_eq!(
            dispatch("protocol.version.list", list_input.to_str().unwrap(), &dir).unwrap()["versions"].as_array().unwrap().len(),
            1
        );
        // Close the SQLite handle before removing the temp dir: on Windows an
        // open handle keeps labworkbench.sqlite locked and remove_dir_all fails.
        drop(db);
        std::fs::remove_dir_all(&dir).unwrap();
        assert!(input_schema("protocol.version.get").is_some());
        assert!(input_schema("protocol.version.list").is_some());
        assert!(!tool_writes("protocol.version.get"));
        assert!(!tool_writes("protocol.version.list"));
        assert!(tool_specs().iter().any(|tool| tool["name"] == "protocol.version.get"));
    }

    #[test]
    fn diagnostics_flag_is_parsed_with_input_in_any_order() {
        let args = vec![
            "--input".to_owned(),
            "in.json".to_owned(),
            "--diagnostics".to_owned(),
            "json".to_owned(),
        ];
        let invocation = parse_args(&args).unwrap();
        assert_eq!(invocation.input, "in.json");
        assert!(invocation.diagnostics);

        let args = vec![
            "--diagnostics".to_owned(),
            "json".to_owned(),
            "--input".to_owned(),
            "-".to_owned(),
        ];
        assert!(parse_args(&args).unwrap().diagnostics);

        let args = vec!["--input".to_owned(), "in.json".to_owned()];
        assert!(!parse_args(&args).unwrap().diagnostics);

        let args = vec![
            "--diagnostics".to_owned(),
            "yaml".to_owned(),
            "--input".to_owned(),
            "-".to_owned(),
        ];
        assert_eq!(parse_args(&args).unwrap_err().code, "INVALID_INPUT");
    }

    #[test]
    fn error_envelope_carries_structured_diagnostics() {
        let mut error = CliError::input("run.create 失败");
        error.path = Some("manifest".to_owned());
        error.issues = vec![json!("STEP_NOT_MATERIALIZED: cck8/seed")];
        let envelope = error_envelope("run.create", &error, true);
        let diagnostics = envelope["diagnostics"].as_array().unwrap();
        assert_eq!(diagnostics.len(), 1);
        assert_eq!(diagnostics[0]["code"], "STEP_NOT_MATERIALIZED");
        assert_eq!(diagnostics[0]["path"], "cck8/seed");
        assert_eq!(diagnostics[0]["severity"], "blocking");
        assert_eq!(diagnostics[0]["source"], "unknown");
        assert!(diagnostics[0]["next_action"].is_string());

        let plain = error_envelope("run.create", &error, false);
        assert!(plain.get("diagnostics").is_none());

        let mut bare = CliError::conflict("设计版本已更新；请先重新读取。");
        bare.path = Some("revision".to_owned());
        let envelope = error_envelope("design.revise", &bare, true);
        assert_eq!(envelope["diagnostics"][0]["code"], "REVISION_CONFLICT");
        assert_eq!(envelope["diagnostics"][0]["path"], "revision");
        assert_eq!(envelope["diagnostics"][0]["severity"], "blocking");
        assert_eq!(envelope["diagnostics"][0]["source"], "derived");
    }

    #[test]
    fn success_envelope_derives_diagnostics_from_validation_issues() {
        let result = json!({
            "valid": false,
            "issues": [{
                "code": "unit_mismatch",
                "path": "operation.temperature",
                "message": "unit family differs"
            }]
        });
        let envelope = success_envelope("semantic.validate", result.clone(), true);
        assert_eq!(envelope["diagnostics"].as_array().unwrap().len(), 1);
        assert_eq!(envelope["diagnostics"][0]["code"], "unit_mismatch");
        assert_eq!(envelope["diagnostics"][0]["severity"], "blocking");
        assert_eq!(envelope["diagnostics"][0]["source"], "derived");
        assert_eq!(envelope["result"], result);

        let plain = success_envelope("semantic.validate", result, false);
        assert!(plain.get("diagnostics").is_none());
    }

    #[test]
    fn agent_tools_are_discoverable() {
        let specs = tool_specs();
        let names: Vec<&str> = specs.iter().filter_map(|v| v.get("name").and_then(Value::as_str)).collect();
        for tool in ["sdk.modules", "sdk.describe", "sdk.validate", "asterdoc.inspect", "asterdoc.assess", "asterdoc.validate", "asterdoc.load", "asterdoc.save", "asterdoc.command", "asterdoc.compile", "asterdoc.create_run"] {
            assert!(names.contains(&tool), "missing {tool}");
            assert!(input_schema(tool).is_some(), "missing input schema for {tool}");
        }
        let command = input_schema("asterdoc.command").unwrap();
        assert!(command["required"].as_array().unwrap().contains(&json!("expected_revision")));
        assert!(command["properties"].get("operation_id").is_some());
        assert!(command["properties"].get("revision").is_none());
        let variants = command["properties"]["command"]["oneOf"].as_array().unwrap();
        let declared_types: Vec<&str> = variants.iter().flat_map(|variant| {
            let kind = &variant["properties"]["type"];
            kind["const"].as_str().into_iter().chain(
                kind["enum"].as_array().into_iter().flatten().filter_map(Value::as_str)
            )
        }).collect();
        for action in ["add_module", "add_step", "update_module", "assign_wells", "clear_wells", "connect_steps", "disconnect_steps", "remove_module", "remove_step", "reorder_steps", "nest_step", "promote_step"] {
            assert!(declared_types.contains(&action), "command schema omits {action}");
        }
        assert_eq!(result_schema("sdk.modules")["required"][0], "modules");
        assert_eq!(result_schema("asterdoc.assess")["required"][3], "state");
        let plate = asterdoc_module_spec("plate", 1).unwrap();
        assert!(asterdoc_module_spec("plate96", 1).is_none());
        for field in ["samples", "replicates"] {
            assert!(plate["inputs"].as_array().unwrap().iter().any(|port| port["name"] == field));
        }
    }

    #[test]
    fn asterdoc_reorders_steps_only_with_a_complete_unique_permutation() {
        let source = "```asterlab\nkind: step\nid: a\nschema: 2\npath: root/a\n```\n\
```asterlab\nkind: step\nid: b\nschema: 2\npath: root/b\n```\n\
```asterlab\nkind: workflow\nid: wf\nschema: 1\nsteps: [a, b]\nedges: [{from: a, to: b}]\nbusiness_order: [a, b]\n```\n";
        let reordered = apply_asterdoc_patch(
            source,
            &json!({"type":"reorder_steps","moduleId":"wf","order":["b","a"]}),
        ).unwrap();
        let workflow = asterdoc_blocks(&reordered).unwrap().into_iter()
            .find(|block| yaml_id(&block.value) == "wf").unwrap().value;
        assert_eq!(workflow["business_order"], serde_yaml::from_str::<serde_yaml::Value>("[b, a]").unwrap());
        assert_eq!(workflow["edges"], serde_yaml::from_str::<serde_yaml::Value>("[{from: a, to: b}]").unwrap());
        for order in [json!(["a"]), json!(["a","a"]), json!(["a","missing"])] {
            let error = apply_asterdoc_patch(
                source,
                &json!({"type":"reorder_steps","moduleId":"wf","order":order}),
            ).unwrap_err();
            assert_eq!(error.code, "INVALID_INPUT");
        }
    }

    #[test]
    fn asterdoc_nests_and_promotes_steps_by_updating_paths() {
        let source = "```asterlab\nkind: step\nid: parent\nschema: 2\npath: root/parent\n```\n\
```asterlab\nkind: step\nid: child\nschema: 2\npath: root/child\n```\n\
```asterlab\nkind: workflow\nid: wf\nschema: 1\nsteps: [parent, child]\nedges: []\nbusiness_order: [parent, child]\n```\n";
        let nested = apply_asterdoc_patch(
            source,
            &json!({"type":"nest_step","moduleId":"child","parentId":"parent"}),
        ).unwrap();
        assert!(nested.contains("path: root/parent/child"));
        let promoted = apply_asterdoc_patch(
            &nested,
            &json!({"type":"promote_step","moduleId":"child"}),
        ).unwrap();
        assert!(promoted.contains("path: root/child"));
        let duplicate = apply_asterdoc_patch(
            &source.replace("path: root/child", "path: root/parent"),
            &json!({"type":"nest_step","moduleId":"child","parentId":"parent"}),
        );
        assert_eq!(duplicate.unwrap_err().code, "INVALID_INPUT");
        let cycle = apply_asterdoc_patch(
            &nested,
            &json!({"type":"nest_step","moduleId":"parent","parentId":"child"}),
        );
        assert_eq!(cycle.unwrap_err().code, "INVALID_INPUT");
    }

    #[test]
    fn asterdoc_derives_missing_step_paths_when_nesting_and_promoting() {
        let source = "```asterlab\nkind: step\nid: soak\nschema: 2\ntitle: Soak\npath: ''\n```\n\
```asterlab\nkind: step\nid: seed\nschema: 2\ntitle: Seed\n```\n\
```asterlab\nkind: workflow\nid: wf\nschema: 1\nsteps: [soak, seed]\nedges: []\nbusiness_order: [soak, seed]\n```\n";
        let nested = apply_asterdoc_patch(
            source,
            &json!({"type":"nest_step","moduleId":"seed","parentId":"soak"}),
        ).unwrap();
        let paths = step_paths_by_id(&nested);
        assert_eq!(paths["soak"], "root/soak");
        assert_eq!(paths["seed"], "root/soak/seed");

        let promoted = apply_asterdoc_patch(
            &nested,
            &json!({"type":"promote_step","moduleId":"seed"}),
        ).unwrap();
        let paths = step_paths_by_id(&promoted);
        assert_eq!(paths["soak"], "root/soak");
        assert_eq!(paths["seed"], "root/seed");
    }

    #[test]
    fn asterdoc_nest_materializes_a_pathless_parent_for_a_pathful_child() {
        let source = "```asterlab\nkind: step\nid: parent\nschema: 2\n```\n\
```asterlab\nkind: step\nid: child\nschema: 2\npath: root/child\n```\n\
```asterlab\nkind: workflow\nid: wf\nschema: 1\nsteps: [parent, child]\nedges: []\nbusiness_order: [parent, child]\n```\n";
        let nested = apply_asterdoc_patch(
            source,
            &json!({"type":"nest_step","moduleId":"child","parentId":"parent"}),
        ).unwrap();
        let paths = step_paths_by_id(&nested);
        assert_eq!(paths["parent"], "root/parent");
        assert_eq!(paths["child"], "root/parent/child");
    }

    #[test]
    fn asterdoc_rejects_a_derived_step_path_that_collides() {
        let source = "```asterlab\nkind: step\nid: imaging\nschema: 2\npath: root/seed\n```\n\
```asterlab\nkind: step\nid: seed\nschema: 2\n```\n\
```asterlab\nkind: workflow\nid: wf\nschema: 1\nsteps: [imaging, seed]\nedges: []\nbusiness_order: [imaging, seed]\n```\n";
        let error = apply_asterdoc_patch(
            source,
            &json!({"type":"promote_step","moduleId":"seed"}),
        ).unwrap_err();
        assert_eq!(error.code, "INVALID_INPUT");
        assert!(error.message.contains("seed"), "{}", error.message);
        assert!(error.message.contains("imaging"), "{}", error.message);
        assert!(error.message.contains("root/seed"), "{}", error.message);
    }

    #[test]
    fn asterdoc_rejects_promoting_a_step_that_is_already_top_level() {
        let source = "```asterlab\nkind: step\nid: child\nschema: 2\npath: root/child\n```\n\
```asterlab\nkind: workflow\nid: wf\nschema: 1\nsteps: [child]\nedges: []\nbusiness_order: [child]\n```\n";
        let error = apply_asterdoc_patch(
            source,
            &json!({"type":"promote_step","moduleId":"child"}),
        ).unwrap_err();
        assert_eq!(error.code, "INVALID_INPUT");
        assert!(error.message.contains("top level"), "{}", error.message);
        assert!(error.message.contains("root/child"), "{}", error.message);
    }

    #[test]
    fn asterdoc_add_step_derives_a_path_when_the_module_omits_one() {
        let added = apply_asterdoc_patch(
            "",
            &json!({"type":"add_step","module":{"kind":"step","id":"seed","schema":2,"path":""}}),
        ).unwrap();
        let paths = step_paths_by_id(&added);
        assert_eq!(paths["seed"], "root/seed");
    }

    #[test]
    fn remove_step_cleans_workflow_order_and_edges_when_unreferenced() {
        let source = "```asterlab\nkind: step\nid: a\nschema: 2\ntitle: A\npath: root/a\n```\n\
```asterlab\nkind: step\nid: b\nschema: 2\ntitle: B\npath: root/b\n```\n\
```asterlab\nkind: workflow\nid: wf\nschema: 1\nsteps: [a, b]\nedges: [{from: a, to: b}]\nbusiness_order: [a, b]\n```\n";
        let removed = apply_asterdoc_patch(source, &json!({"type":"remove_step","moduleId":"a"})).unwrap();
        let blocks = asterdoc_blocks(&removed).unwrap();
        assert!(!blocks.iter().any(|block| yaml_id(&block.value) == "a"));
        let workflow = blocks.iter().find(|block| yaml_id(&block.value) == "wf").unwrap();
        assert_eq!(workflow.value["steps"], serde_yaml::from_str::<serde_yaml::Value>("[b]").unwrap());
        assert_eq!(workflow.value["business_order"], serde_yaml::from_str::<serde_yaml::Value>("[b]").unwrap());
        assert_eq!(workflow.value["edges"], serde_yaml::Value::Sequence(Vec::new()));
    }

    #[test]
    fn remove_step_rejects_run_and_step_references_without_revision_changes() {
        let mut db = Connection::open_in_memory().unwrap();
        db.execute_batch(
            "CREATE TABLE records(id TEXT PRIMARY KEY,kind TEXT NOT NULL,project_id TEXT,parent_id TEXT,revision INTEGER NOT NULL,data_json TEXT NOT NULL,created_at TEXT NOT NULL,updated_at TEXT NOT NULL);
             INSERT INTO records VALUES('project-1','project',NULL,NULL,1,'{}','now','now');
             INSERT INTO records VALUES('design-1','design','project-1',NULL,1,'{}','now','now');",
        ).unwrap();
        let source = "```asterlab\nkind: assay\nid: assay-1\nschema: 1\ntitle: Assay\n```\n\
```asterlab\nkind: step\nid: target\nschema: 2\ntitle: Target\nassay: assay-1\npath: root/target\n```\n\
```asterlab\nkind: step\nid: consumer\nschema: 2\ntitle: Consumer\nassay: assay-1\npath: root/consumer\nuses: [{ref: target, role: other}]\n```\n\
```asterlab\nkind: workflow\nid: wf\nschema: 1\nsteps: [target, consumer]\nedges: [{from: target, to: consumer}]\nbusiness_order: [target, consumer]\n```\n";
        save_asterdoc_version(&mut db, &json!({
            "design_id":"design-1","source":source,"expected_revision":0
        })).unwrap();
        let error = apply_asterdoc_command(&mut db, &json!({
            "design_id":"design-1","expected_revision":1,
            "command":{"type":"remove_step","moduleId":"target"}
        })).unwrap_err();
        assert_eq!(error.code, "STEP_REFERENCED");
        assert_eq!(error.issues[0]["code"], "STEP_REFERENCED");
        let latest = load_asterdoc(&db, "design-1").unwrap();
        assert_eq!(latest["revision"], 1);
        assert_eq!(latest["source"], source);
        let revisions: i64 = db.query_row(
            "SELECT COUNT(*) FROM records WHERE kind='asterdoc_version' AND parent_id='design-1'",
            [], |row| row.get(0),
        ).unwrap();
        assert_eq!(revisions, 1);

        db.execute(
            "INSERT INTO records VALUES('run-1','run','project-1','design-1',1,?1,'now','now')",
            [json!({"asterdoc":{"revision":1}}).to_string()],
        ).unwrap();
        let run_error = apply_asterdoc_command(&mut db, &json!({
            "design_id":"design-1","expected_revision":1,
            "command":{"type":"remove_step","moduleId":"consumer"}
        })).unwrap_err();
        assert_eq!(run_error.code, "STEP_REFERENCED");
        assert_eq!(load_asterdoc(&db, "design-1").unwrap()["revision"], 1);
    }

    #[test]
    fn compiler_records_the_block_version_each_step_reuses() {
        let source = "```asterlab\nkind: experiment\nid: exp-1\nschema: 1\ntitle: CCK-8\n```\n\
```asterlab\nkind: group\nid: g-1\nschema: 1\nname: Control\nrole: control\n```\n\
```asterlab\nkind: assay\nid: assay-cck8\nschema: 1\ntitle: CCK-8\nendpoint: OD450\nunit: AU\ngroup_refs: [g-1]\n```\n\
```asterlab\nkind: reference\nid: ref-block\nschema: 1\nref: block-v1\nview: protocol_block\n```\n\
```asterlab\nkind: reference\nid: ref-seed-block\nschema: 1\nref: block-seed\nview: protocol_block\n```\n\
```asterlab\nkind: sample\nid: s-1\nschema: 1\nsample_id: S1\ngroup_ref: g-1\namount: {value: 1, unit: uL}\n```\n\
```asterlab\nkind: step\nid: step-read\nschema: 2\ntitle: Read\nassay: assay-cck8\npath: cck8/readout\norder: 1\nuses:\n  - {ref: ref-block, role: other}\n  - {ref: s-1, role: sample}\n```\n\
```asterlab\nkind: step\nid: step-seed\nschema: 2\ntitle: Seed\nassay: assay-cck8\npath: cck8/seed\norder: 2\nuses:\n  - {ref: s-1, role: sample}\n```\n\
```asterlab\nkind: step\nid: step-organize\nschema: 3\ntitle: Incubate\nexecution: organizational\norder: 3\n```\n\
```asterlab\nkind: workflow\nid: wf\nschema: 1\nsteps: [step-read, step-seed, step-organize]\n```\n";
        let design = compile_asterdoc_to_design(source).unwrap();
        let blocks = &design["mygo"]["stepBlocks"];
        assert_eq!(blocks["step-read"], json!(["block-v1"]));
        assert!(blocks.get("step-seed").is_none(), "a sample reference is not a block reference: {blocks}");
        assert_eq!(design["mygo"]["stepExecution"]["step-organize"], "organizational");
        let task_ids: Vec<_> = design["mygo"]["stepTasks"]
            .as_array().unwrap().iter()
            .filter_map(|task| task["stepId"].as_str())
            .collect();
        assert_eq!(task_ids, ["step-read", "step-seed"]);
        let manifest = json!([{"assay_id":"assay-cck8","block_version_ids":["block-v1"]}]);
        let issues = step_materialization_issues(&design, manifest.as_array().unwrap());
        assert!(issues.iter().any(|issue| issue.contains("STEP_NOT_MATERIALIZED: cck8/seed")));
        assert!(!issues.iter().any(|issue| issue.contains("step-organize")));
        let wrong_scope = json!([
            {"assay_id":"assay-cck8","block_version_ids":["block-v1"]},
            {"assay_id":"assay-other","block_version_ids":["block-seed"]}
        ]);
        let wrong_scope_issues = step_materialization_issues(&design, wrong_scope.as_array().unwrap());
        assert!(wrong_scope_issues.iter().any(|issue| issue.contains("STEP_NOT_MATERIALIZED: cck8/seed")));
        let legacy = json!({"mygo":{"stepTasks":[{"stepId":"legacy-seed","path":"cck8/seed","assayId":"assay-cck8"}]}});
        let legacy_issues = step_materialization_issues(&legacy, &[]);
        assert!(legacy_issues.iter().any(|issue| issue.contains("STEP_NOT_MATERIALIZED: cck8/seed")));

        let bound_seed = source.replace(
            "uses:\n  - {ref: s-1, role: sample}\n```\n```asterlab\nkind: step\nid: step-organize",
            "uses:\n  - {ref: ref-seed-block, role: other}\n  - {ref: s-1, role: sample}\n```\n```asterlab\nkind: step\nid: step-organize",
        );
        let bound_design = compile_asterdoc_to_design(&bound_seed).unwrap();
        assert_eq!(bound_design["mygo"]["stepBlocks"]["step-seed"], json!(["block-seed"]));
        let complete_manifest = json!([{"assay_id":"assay-cck8","block_version_ids":["block-v1","block-seed"]}]);
        assert!(step_materialization_issues(&bound_design, complete_manifest.as_array().unwrap()).is_empty());
        let invalid_org = source.replace(
            "execution: organizational\n",
            "execution: organizational\npath: cck8/incubation\n",
        );
        assert!(validate_asterdoc_source(&invalid_org).is_err());
    }

    #[test]
    fn compiler_keeps_sample_identity_explicit_across_timepoints() {
        let source = "```asterlab\nkind: experiment\nid: exp-1\nschema: 1\ntitle: CCK-8\n```\n\
```asterlab\nkind: group\nid: g-control\nschema: 1\nname: Control\nrole: control\n```\n\
```asterlab\nkind: assay\nid: assay-cck8\nschema: 1\ntitle: CCK-8\nendpoint: OD450\nunit: AU\ngroup_refs: [g-control]\ntimepoints: [D1, D3]\n```\n\
```asterlab\nkind: plate\nid: plate-1\nschema: 1\nformat: 96\nassay: assay-cck8\ngroups:\n  g-control: [A1, A2, A3]\nsamples:\n  A1: sample-1\n  A2: sample-1\n  A3: sample-1\nreplicates:\n  A1: 1\n  A2: 2\n  A3: 3\n```\n";
        let design = compile_asterdoc_to_design(source).unwrap();
        let wells = design["mygo"]["assays"]["assay-cck8"]["wells"].as_array().unwrap();
        assert_eq!(wells.len(), 6, "unscoped plate layout should project to every declared assay timepoint");
        for timepoint in ["D1", "D3"] {
            let at_timepoint: Vec<_> = wells.iter().filter(|well| well["timepoint"] == timepoint).collect();
            assert_eq!(at_timepoint.len(), 3);
            for (index, well) in at_timepoint.iter().enumerate() {
                assert_eq!(well["sampleId"], "sample-1");
                assert_eq!(well["replicate"], index as i64 + 1);
            }
        }

        let source_without_identity = source.replace(
            "samples:\n  A1: sample-1\n  A2: sample-1\n  A3: sample-1\nreplicates:\n  A1: 1\n  A2: 2\n  A3: 3\n",
            "",
        );
        let unmapped = compile_asterdoc_to_design(&source_without_identity).unwrap();
        let first = &unmapped["mygo"]["assays"]["assay-cck8"]["wells"][0];
        assert_eq!(first["sampleId"], "");
        assert_eq!(first["replicate"], 0);
    }

    #[test]
    fn assess_includes_run_creation_audit_warnings() {
        let mut db = Connection::open_in_memory().unwrap();
        db.execute_batch(
            "CREATE TABLE records(id TEXT PRIMARY KEY,kind TEXT NOT NULL,project_id TEXT,parent_id TEXT,revision INTEGER NOT NULL,data_json TEXT NOT NULL,created_at TEXT NOT NULL,updated_at TEXT NOT NULL);
             INSERT INTO records VALUES('project-1','project',NULL,NULL,1,'{}','now','now');
             INSERT INTO records VALUES('design-1','design','project-1',NULL,1,'{}','now','now');",
        ).unwrap();
        let source = r#"```asterlab
kind: experiment
id: exp-1
schema: 1
title: CCK-8
```
```asterlab
kind: group
id: g-control
schema: 1
name: Control
role: control
```
```asterlab
kind: assay
id: assay-cck8
schema: 1
title: CCK-8
endpoint: OD450
unit: AU
group_refs: [g-control]
timepoints: [D1]
```
```asterlab
kind: plate
id: plate-1
schema: 1
format: 96
assay: assay-cck8
groups:
  g-control: [A1]
```
"#;
        save_asterdoc_version(&mut db, &json!({
            "design_id": "design-1",
            "source": source,
            "expected_revision": 0
        })).unwrap();

        let assessed = assess_asterdoc(&db, &json!({"design_id": "design-1"})).unwrap();
        let compiled = compile_asterdoc_to_design(source).unwrap();
        let freeze_audit = run_creation_audit(&compiled, 0, "").unwrap();
        assert_eq!(assessed["warnings"], freeze_audit["warnings"]);
        assert!(assessed["warnings"].as_array().unwrap().iter().any(|warning|
            warning.as_str().unwrap_or("").contains("尚未指定完整的样本与技术重复映射")
        ));
    }


    #[test]
    fn assess_and_run_creation_reject_unmaterialized_steps() {
        let mut db = Connection::open_in_memory().unwrap();
        db.execute_batch(
            "CREATE TABLE records(id TEXT PRIMARY KEY,kind TEXT NOT NULL,project_id TEXT,parent_id TEXT,revision INTEGER NOT NULL,data_json TEXT NOT NULL,created_at TEXT NOT NULL,updated_at TEXT NOT NULL);
             INSERT INTO records VALUES('project-1','project',NULL,NULL,1,'{}','now','now');
             INSERT INTO records VALUES('design-1','design','project-1',NULL,1,'{}','now','now');",
        ).unwrap();
        let source = r#"```asterlab
kind: experiment
id: exp-1
schema: 1
title: CCK-8
```
```asterlab
kind: group
id: g-1
schema: 1
name: Control
role: control
```
```asterlab
kind: assay
id: assay-cck8
schema: 1
title: CCK-8
endpoint: OD450
unit: AU
group_refs: [g-1]
```
```asterlab
kind: step
id: step-seed
schema: 3
title: Seed
execution: executable
assay: assay-cck8
path: cck8/seed
```
"#;
        save_asterdoc_version(&mut db, &json!({
            "design_id": "design-1",
            "source": source,
            "expected_revision": 0
        })).unwrap();

        let assessed = assess_asterdoc(&db, &json!({"design_id": "design-1"})).unwrap();
        assert!(assessed["errors"].as_array().unwrap().iter().any(|error|
            error.as_str().unwrap_or("").contains("STEP_NOT_MATERIALIZED: cck8/seed")
        ), "assessment errors = {}", assessed["errors"]);

        let error = create_run_from_asterdoc(&mut db, "design-1", 1, None, None, &[])
            .unwrap_err();
        assert!(
            error.message.contains("STEP_NOT_MATERIALIZED: cck8/seed"),
            "{}",
            error.message
        );
        let revision: i64 = db.query_row(
            "SELECT revision FROM records WHERE id='design-1'",
            [],
            |row| row.get(0),
        ).unwrap();
        let run_count: i64 = db.query_row(
            "SELECT COUNT(*) FROM records WHERE kind='run'",
            [],
            |row| row.get(0),
        ).unwrap();
        let design_version_count: i64 = db.query_row(
            "SELECT COUNT(*) FROM records WHERE kind='design_version'",
            [],
            |row| row.get(0),
        ).unwrap();
        assert_eq!((revision, run_count, design_version_count), (1, 0, 0));
    }

    #[test]
    fn step_materialization_preserves_frozen_run_protocol_block_revision() {
        let mut db = Connection::open_in_memory().unwrap();
        db.execute_batch(
            "CREATE TABLE records(id TEXT PRIMARY KEY,kind TEXT NOT NULL,project_id TEXT,parent_id TEXT,revision INTEGER NOT NULL,data_json TEXT NOT NULL,created_at TEXT NOT NULL,updated_at TEXT NOT NULL);
             INSERT INTO records VALUES('project-1','project',NULL,NULL,1,'{}','now','now');
             INSERT INTO records VALUES('protocol-1','protocol','project-1',NULL,1,'{\"title\":\"CCK-8\"}','now','now');
             INSERT INTO records VALUES('protocol-v1','protocol_version','project-1','protocol-1',1,'{\"protocol_id\":\"protocol-1\",\"version\":1,\"body\":\"\",\"root_block_version_id\":\"block-v1\",\"root_bindings\":[]}','now','now');
             INSERT INTO records VALUES('block-1','protocol_block','project-1',NULL,1,'{\"title\":\"CCK-8 block\",\"summary\":\"\"}','now','now');",
        ).unwrap();
        let source = r#"```asterlab
kind: experiment
id: exp-1
schema: 1
title: CCK-8
```
```asterlab
kind: group
id: g-1
schema: 1
name: Control
role: control
```
```asterlab
kind: assay
id: assay-cck8
schema: 1
title: CCK-8
endpoint: OD450
unit: AU
group_refs: [g-1]
protocol_version_id: protocol-v1
```
"#;
        let mut design = compile_asterdoc_to_design(source).unwrap();
        let design_object = design.as_object_mut().unwrap();
        design_object.remove("mygo");
        design_object.remove("asterlab");
        let design_json = serde_json::to_string(&design).unwrap();
        db.execute(
            "INSERT INTO records VALUES('design-1','design','project-1',NULL,1,?1,'now','now')",
            rusqlite::params![design_json],
        ).unwrap();
        db.execute(
            "INSERT INTO records VALUES('design-v1','design_version','project-1','design-1',1,'{}','now','now')",
            [],
        ).unwrap();
        let block_v1 = json!({
            "block_id":"block-1", "version":1, "summary":"",
            "parameter_slots":[],
            "steps":[{"kind":"text","use_id":"seed","title":"Seed","instruction":"Seed cells"}]
        });
        db.execute(
            "INSERT INTO records VALUES('block-v1','protocol_block_version','project-1','block-1',1,?1,'now','now')",
            rusqlite::params![serde_json::to_string(&block_v1).unwrap()],
        ).unwrap();

        let tx = db.transaction_with_behavior(rusqlite::TransactionBehavior::Immediate).unwrap();
        let run = create_run_in_transaction(
            &tx, "design-1", "Run 1", None, None, None, None, "independent", None,
        ).unwrap();
        let run_id = run["id"].as_str().unwrap().to_owned();
        tx.commit().unwrap();
        let before = get(&db, &run_id).unwrap()["data"]["protocol_manifest"].clone();
        assert_eq!(before[0]["protocol_version_id"], "protocol-v1");
        assert_eq!(before[0]["block_version_ids"], json!(["block-v1"]));

        let block_v2 = json!({
            "block_id":"block-1", "version":2, "summary":"Revised",
            "parameter_slots":[],
            "steps":[{"kind":"text","use_id":"seed","title":"Revised seed","instruction":"Updated"}]
        });
        db.execute(
            "INSERT INTO records VALUES('block-v2','protocol_block_version','project-1','block-1',2,?1,'later','later')",
            rusqlite::params![serde_json::to_string(&block_v2).unwrap()],
        ).unwrap();
        let after = get(&db, &run_id).unwrap()["data"]["protocol_manifest"].clone();
        assert_eq!(after, before);
    }

    #[test]
    fn clear_wells_removes_sample_and_replicate_mappings() {
        let source = "```asterlab\nkind: plate\nid: p1\nschema: 1\nformat: 96\ngroups:\n  g1: [A1, A2]\nsamples:\n  A1: S1\n  A2: S2\nreplicates:\n  A1: 1\n  A2: 1\n```\n";
        let next = apply_asterdoc_patch(source, &json!({
            "type": "clear_wells", "moduleId": "p1", "wells": ["A1"]
        })).unwrap();
        let mut blocks = asterdoc_blocks(&next).unwrap();
        let block = blocks.remove(0);
        let groups = block.value["groups"]["g1"].as_sequence().unwrap();
        assert_eq!(groups.len(), 1);
        assert_eq!(groups[0].as_str(), Some("A2"));
        assert!(block.value["samples"].get("A1").is_none());
        assert!(block.value["replicates"].get("A1").is_none());


    }
    #[test]
    fn manifest_rule_must_be_registered_in_rust() {
        let spec = json!({
            "foundationVersion": 1,
            "sdkVersion": "1.0.0",
            "modules": [{
                "kind": "probe", "schema": 1, "sdk": "1.0.0", "category": "workflow",
                "inputs": [], "outputs": [], "actions": [], "permissions": [],
                "rules": ["not-a-real-rule"]
            }]
        });
        let error = validate_asterdoc_spec(&spec).unwrap_err();
        assert!(error.contains("unregistered rule not-a-real-rule"), "{error}");
    }

    #[test]
    fn read_only_tools_do_not_write() {
        for tool in ["sdk.modules", "sdk.describe", "sdk.validate", "asterdoc.inspect", "asterdoc.assess", "asterdoc.validate", "asterdoc.compile", "asterdoc.load"] {
            assert!(!tool_writes(tool), "{tool} should be read-only");
        }
        for tool in ["asterdoc.save", "asterdoc.command", "asterdoc.create_run"] {
            assert!(tool_writes(tool), "{tool} should write");
        }
    }
    #[test]
    fn workflow_inspect_tree_matches_shared_go_evidence() {
        let root = std::path::Path::new(env!("CARGO_MANIFEST_DIR"))
            .ancestors()
            .find(|dir| {
                dir.join("prototypes/mygo-run/testdata/rna-case/labworkbench.sqlite")
                    .is_file()
            })
            .expect("repository root with RNA evidence database");
        let data_dir = std::env::temp_dir().join(format!("asterlab-workflow-{}", Uuid::new_v4()));
        fs::create_dir_all(&data_dir).expect("temp data dir");
        fs::copy(
            root.join("prototypes/mygo-run/testdata/rna-case/labworkbench.sqlite"),
            data_dir.join("labworkbench.sqlite"),
        )
        .expect("copy RNA evidence database");
        let design_id = "d5525203-56bb-4b52-82c4-0e05f0ed2ff8";
        let run_id = "ad9b452e-f8ea-404b-9a2a-7124f9403b02";

        let design = inspect_workflow_tree(
            &InspectTreeInput {
                design_id: design_id.into(),
                revision: Some(1),
                run_id: None,
            },
            &data_dir,
        )
        .expect("design projection");
        let design_evidence: Value = serde_json::from_str(
            &fs::read_to_string(
                root.join("docs/sdk/rna-extraction-case/evidence/inspect-tree-design.json"),
            )
            .expect("design evidence"),
        )
        .expect("design evidence json");
        assert_eq!(design, design_evidence["result"]);

        let run = inspect_workflow_tree(
            &InspectTreeInput {
                design_id: design_id.into(),
                revision: None,
                run_id: Some(run_id.into()),
            },
            &data_dir,
        )
        .expect("run projection");
        let run_evidence: Value = serde_json::from_str(
            &fs::read_to_string(
                root.join("docs/sdk/rna-extraction-case/evidence/inspect-tree-run.json"),
            )
            .expect("run evidence"),
        )
        .expect("run evidence json");
        assert_eq!(run, run_evidence["result"]);

        let wrong_design = inspect_workflow_tree(
            &InspectTreeInput {
                design_id: "00000000-0000-0000-0000-000000000000".into(),
                revision: None,
                run_id: Some(run_id.into()),
            },
            &data_dir,
        )
        .expect_err("Run must reject a different design");
        assert_eq!(wrong_design.code, "INVALID_INPUT");

        let stale_revision = inspect_workflow_tree(
            &InspectTreeInput {
                design_id: design_id.into(),
                revision: Some(2),
                run_id: Some(run_id.into()),
            },
            &data_dir,
        )
        .expect_err("Run must reject a non-frozen revision");
        assert_eq!(stale_revision.code, "REVISION_CONFLICT");

        let db = open_db(&data_dir).expect("open evidence database");
        let run_record = get(&db, run_id).expect("run record");
        let mut data = run_record["data"].clone();
        data["asterdoc"]["source_hash"] = json!("corrupt-source-hash");
        db.execute(
            "UPDATE records SET data_json=?2 WHERE id=?1",
            params![run_id, serde_json::to_string(&data).unwrap()],
        )
        .expect("corrupt frozen source hash");
        drop(db);
        let corrupt_hash = inspect_workflow_tree(
            &InspectTreeInput {
                design_id: design_id.into(),
                revision: None,
                run_id: Some(run_id.into()),
            },
            &data_dir,
        )
        .expect_err("Run must reject a frozen source hash mismatch");
        assert_eq!(corrupt_hash.code, "REVISION_CONFLICT");
        assert!(
            corrupt_hash.message.contains("source_hash"),
            "{}",
            corrupt_hash.message
        );

        let db = open_db(&data_dir).expect("reopen evidence database");
        let mut old_data = run_record["data"].clone();
        old_data["design_revision"] = run_record["data"]["asterdoc"]["revision"].clone();
        if let Some(object) = old_data.as_object_mut() {
            object.remove("asterdoc");
        }
        db.execute(
            "UPDATE records SET data_json=?2 WHERE id=?1",
            params![run_id, serde_json::to_string(&old_data).unwrap()],
        )
        .expect("remove frozen document metadata");
        drop(db);
        let old_run = inspect_workflow_tree(
            &InspectTreeInput {
                design_id: design_id.into(),
                revision: None,
                run_id: Some(run_id.into()),
            },
            &data_dir,
        )
        .expect("legacy Run without a frozen document returns an empty tree");
        assert_eq!(old_run["design"], false);
        assert!(old_run["roots"].as_array().unwrap().is_empty());
        assert!(
            old_run["issues"][0]
                .as_str()
                .unwrap_or("")
                .contains("经典 design.create"),
            "{}",
            old_run["issues"]
        );
        fs::remove_dir_all(&data_dir).expect("remove temp data dir");
    }

    #[test]
    fn asterdoc_parity_samples() {
        // This file is compiled into more than one crate (the CLI and the app),
        // so the fixture is found by walking up from the compiling manifest
        // instead of assuming the repository root is the working directory.
        let fixture = std::path::Path::new(env!("CARGO_MANIFEST_DIR"))
            .ancestors()
            .map(|dir| dir.join("tests/asterdoc_parity_samples.json"))
            .find(|path| path.is_file())
            .expect("shared parity fixture");
        let raw = std::fs::read_to_string(&fixture)
            .unwrap_or_else(|e| panic!("read {}: {e}", fixture.display()));
        let samples: serde_json::Value = serde_json::from_str(&raw).expect("json");
        for sample in samples.as_array().expect("array") {
            let source = sample["source"].as_str().unwrap_or("");
            let expected = sample["valid"].as_bool().unwrap_or(false);
            let actual = validate_asterdoc_source(source).is_ok();
            assert_eq!(actual, expected, "sample {} mismatch", sample["name"]);
        }
    }

    #[test]
    fn asterdoc_plate_mix_fields_match_go_calculator_cases() {
        let valid = r#"```asterlab
kind: group
id: "3%"
schema: 1
name: 3% GelMA
```
```asterlab
kind: group
id: "5%"
schema: 1
name: 5% GelMA
```
```asterlab
kind: group
id: "7%"
schema: 1
name: 7% GelMA
```
```asterlab
kind: plate
id: plate-huvec
schema: 1
format: 12
groups:
  "3%": [A1, A2, A3]
  "5%": [B1, B2, B3]
  "7%": [C1, C2, C3]
mix_volume_per_well: {value: 200, unit: uL}
mix_excess_ratio: 0.1
mix_gelma_percent_by_group:
  "3%": 3
  "5%": 5
  "7%": 7
mix_lap_stock_percent: 0.5
mix_lap_final_percent: 0.05
mix_cell_density: {value: 20000, unit: cells/well}
mix_factor_stock_concentration: {value: 10, unit: ug/mL}
mix_factor_final_concentration: {value: 50, unit: ng/mL}
```"#;
        assert!(
            validate_asterdoc_source(valid).is_ok(),
            "valid mix source rejected: {:?}",
            validate_asterdoc_source(valid).err().map(|e| e.message)
        );
        let invalid = [
            valid.replace("mix_lap_final_percent: 0.05", "mix_lap_final_percent: 0.6"),
            valid.replace("  \"7%\": 7\n", ""),
            valid.replace(
                "mix_factor_final_concentration: {value: 50, unit: ng/mL}",
                "mix_factor_final_concentration: {value: 20, unit: mg/mL}",
            ),
            valid.replace(
                "mix_factor_stock_concentration: {value: 10, unit: ug/mL}\n",
                "",
            ),
        ];
        for source in invalid {
            assert!(
                validate_asterdoc_source(&source).is_err(),
                "invalid mix source accepted"
            );
        }
    }

    #[test]
    fn plate_mix_calculators_match_the_huvec_case() {
        fn write_calc_input(dir: &std::path::Path, name: &str, value: Value) -> String {
            let path = dir.join(name);
            fs::write(&path, serde_json::to_string(&value).unwrap()).unwrap();
            path.to_string_lossy().into_owned()
        }

        let dir = std::env::temp_dir().join(format!("asterlab-exp02-{}", Uuid::new_v4()));
        fs::create_dir_all(&dir).unwrap();
        let groups = json!([
            {"name":"3%","wells":3},
            {"name":"5%","wells":3},
            {"name":"7%","wells":3}
        ]);

        let gel_input = write_calc_input(&dir, "gel.json", json!({
            "volume_per_well_ul": 200.0,
            "excess_ratio": 0.1,
            "lap_stock_percent_wv": 0.5,
            "lap_final_percent_wv": 0.05,
            "groups": [
                {"name":"3%","gelma_percent_wv":3.0,"wells":3},
                {"name":"5%","gelma_percent_wv":5.0,"wells":3},
                {"name":"7%","gelma_percent_wv":7.0,"wells":3}
            ]
        }));
        let gel = dispatch("calculator.gel_mastermix", &gel_input, &dir).unwrap();
        assert_eq!(gel["groups"].as_array().unwrap().len(), 3);
        assert!((gel["total_gelma_mass_mg"].as_f64().unwrap() - 99.0).abs() < 1e-9);
        assert!((gel["total_lap_stock_volume_ul"].as_f64().unwrap() - 198.0).abs() < 1e-9);
        assert!((gel["total_solvent_volume_ul"].as_f64().unwrap() - 1782.0).abs() < 1e-9);

        let cells_input = write_calc_input(&dir, "cells.json", json!({
            "density_cells_per_well": 20000.0,
            "volume_per_well_ul": 200.0,
            "excess_ratio": 0.1,
            "factor_stock_mg_per_ml": 0.01,
            "factor_final_mg_per_ml": 0.00005,
            "groups": groups
        }));
        let cells = dispatch("calculator.cell_seeding", &cells_input, &dir).unwrap();
        assert_eq!(cells["total_cells"], json!(198000));
        assert!((cells["total_factor_stock_volume_ul"].as_f64().unwrap() - 9.9).abs() < 1e-9);

        let runtime_input = write_calc_input(&dir, "runtime.json", json!({
            "planned_wells": 9,
            "actual_wells": 7,
            "volume_per_well_ul": 200.0,
            "excess_ratio": 0.1,
            "dead_volume_ul": 50.0,
            "prepared_volume_ul": 2000.0
        }));
        let runtime = dispatch("calculator.runtime_normalize", &runtime_input, &dir).unwrap();
        assert!((runtime["required_volume_ul"].as_f64().unwrap() - 1590.0).abs() < 1e-9);
        assert!((runtime["unused_volume_ul"].as_f64().unwrap() - 410.0).abs() < 1e-9);

        let bad = write_calc_input(&dir, "bad.json", json!({
            "volume_per_well_ul": 200.0,
            "lap_stock_percent_wv": 0.5,
            "lap_final_percent_wv": 0.6,
            "groups": [{"name":"3%","gelma_percent_wv":3.0,"wells":3}]
        }));
        assert_eq!(
            dispatch("calculator.gel_mastermix", &bad, &dir).unwrap_err().code,
            "INVALID_INPUT"
        );
        fs::remove_dir_all(&dir).ok();
    }

    fn seed_asset_run(db: &Connection) -> String {
        db.execute(
            "INSERT INTO records(id,kind,project_id,parent_id,revision,data_json,created_at,updated_at) VALUES('project-1','project',NULL,NULL,1,?1,'now','now')",
            [json!({"name":"p","description":""}).to_string()],
        )
        .unwrap();
        db.execute(
            "INSERT INTO records(id,kind,project_id,parent_id,revision,data_json,created_at,updated_at) VALUES('design-1','design','project-1',NULL,1,?1,'now','now')",
            [json!({"title":"d"}).to_string()],
        )
        .unwrap();
        db.execute(
            "INSERT INTO records(id,kind,project_id,parent_id,revision,data_json,created_at,updated_at) VALUES('version-1','design_version','project-1','design-1',1,?1,'now','now')",
            [json!({"design_id":"design-1","design_revision":1,"snapshot":{"title":"d"}}).to_string()],
        )
        .unwrap();
        let step = json!({
            "path":"s/1","use_id":"u1","title":"T","instruction":"I",
            "parameters":[],"unresolved_parameter_refs":[],"source_block_version_id":"b1"
        });
        db.execute(
            "INSERT INTO records(id,kind,project_id,parent_id,revision,data_json,created_at,updated_at) VALUES('run-1','run','project-1','design-1',1,?1,'now','now')",
            [json!({
                "title":"r","design_version_id":"version-1","design_revision":1,
                "status":"in_progress",
                "protocol_manifest":[{"assay_id":"assay-1","assay_title":"A","protocol_version_id":"pv1","steps":[step]}]
            })
            .to_string()],
        )
        .unwrap();
        run_workspace(db, "run-1").unwrap()["tasks"][0]["task_key"]
            .as_str()
            .expect("task key")
            .to_owned()
    }
    #[test]
    fn run_workspace_carries_asset_coordinates_and_the_gap_grouping() {
        let db = Connection::open_in_memory().unwrap();
        db.execute_batch("CREATE TABLE records(id TEXT PRIMARY KEY,kind TEXT NOT NULL,project_id TEXT,parent_id TEXT,revision INTEGER NOT NULL,data_json TEXT NOT NULL,created_at TEXT NOT NULL,updated_at TEXT NOT NULL);").unwrap();
        let task_key = seed_asset_run(&db);
        let link = |id: &str, well: Option<&str>, timepoint: Option<&str>, fov: Option<u32>| {
            json!({
                "id": id,
                "link": {"runId":"run-1","taskKey":task_key,"sampleId":"sample-1",
                    "wellId":well,"timepoint":timepoint,"fovIndex":fov}
            })
        };
        for (index, data) in [
            link("asset-1", Some("A1"), Some("t6h"), Some(1)),
            link("asset-2", Some("A1"), Some("t6h"), Some(2)),
            link("asset-3", None, None, None),
        ]
        .iter()
        .enumerate()
        {
            db.execute(
                "INSERT INTO records(id,kind,project_id,parent_id,revision,data_json,created_at,updated_at) VALUES(?1,'asset_link','project-1','run-1',1,?2,'now','now')",
                params![format!("link-{index}"), data.to_string()],
            )
            .unwrap();
        }

        let workspace = run_workspace(&db, "run-1").unwrap();
        let assets = workspace["linked_assets"].as_array().unwrap();
        assert_eq!(assets.len(), 3);
        let asset = |id: &str| {
            assets
                .iter()
                .find(|asset| asset["asset_id"] == id)
                .unwrap_or_else(|| panic!("missing {id}"))
        };
        assert_eq!(asset("asset-1")["task_key"], task_key);
        assert_eq!(asset("asset-1")["sample_id"], "sample-1");
        assert_eq!(asset("asset-1")["well_id"], "A1");
        assert_eq!(asset("asset-1")["timepoint"], "t6h");
        assert_eq!(asset("asset-1")["fov_index"], 1);
        assert_eq!(asset("asset-1")["coordinate_id"], "A1_t6h_fov01");
        assert_eq!(asset("asset-2")["fov_index"], 2);
        assert_eq!(asset("asset-3")["well_id"], Value::Null);
        assert_eq!(asset("asset-3")["fov_index"], Value::Null);
        assert_eq!(asset("asset-3")["coordinate_id"], Value::Null);

        let groups = workspace["asset_coverage"].as_array().unwrap();
        assert_eq!(groups.len(), 1);
        assert_eq!(groups[0]["well_id"], "A1");
        assert_eq!(groups[0]["timepoint"], "t6h");
        assert_eq!(groups[0]["fov_indices"], json!([1, 2]));
        assert_eq!(groups[0]["asset_ids"], json!(["asset-1", "asset-2"]));
    }

    #[test]
    fn rule_skip_event_requires_skip_status_and_projects_ad_hoc_marker() {
        let db = Connection::open_in_memory().unwrap();
        db.execute_batch("CREATE TABLE records(id TEXT PRIMARY KEY,kind TEXT NOT NULL,project_id TEXT,parent_id TEXT,revision INTEGER NOT NULL,data_json TEXT NOT NULL,created_at TEXT NOT NULL,updated_at TEXT NOT NULL);").unwrap();
        let task_key = seed_asset_run(&db);
        let tx = db.unchecked_transaction().unwrap();
        let invalid = json!({"event_type":"skipped_by_rule","content":"No bubbles",
            "step_status":"completed","protocol_step_path":"s/1","protocol_version_id":"pv1",
            "assay_id":"assay-1","task_key":task_key,"ad_hoc_step":true});
        let invalid_error = create_execution_event_tx(&tx, "run-1", &invalid).unwrap_err();
        assert!(
            invalid_error.message.contains("step_status=skipped"),
            "{invalid_error:?}"
        );
        let valid = json!({"event_type":"skipped_by_rule","content":"No bubbles",
            "step_status":"skipped","protocol_step_path":"s/1","protocol_version_id":"pv1",
            "assay_id":"assay-1","task_key":task_key,"ad_hoc_step":true});
        let saved = create_execution_event_tx(&tx, "run-1", &valid).unwrap();
        tx.commit().unwrap();
        let projected = run_workspace(&db, "run-1").unwrap();
        assert_eq!(saved["data"]["event_type"], "skipped_by_rule");
        assert_eq!(projected["events"][0]["skipped_by_rule"], true);
        assert_eq!(projected["events"][0]["ad_hoc_step"], true);
        assert_eq!(projected["tasks"][0]["status"], "skipped");
    }

    #[test]
    fn execution_event_writes_exclusion_statuses_and_sample_lineage() {
        let dir = std::env::temp_dir().join(format!("asterlab-event-{}", Uuid::new_v4()));
        std::fs::create_dir_all(&dir).unwrap();
        {
            let db = open_db(&dir).unwrap();
            seed_asset_run(&db);
        }
        let write_input = |name: &str, value: Value| {
            let path = dir.join(name);
            std::fs::write(&path, value.to_string()).unwrap();
            path
        };

        for (file, status) in [
            ("qc.json", "qc_dropped"),
            ("dead.json", "censored_dead"),
            ("rework.json", "rework"),
        ] {
            let sample_id = format!("sample-{status}");
            let created = dispatch(
                "execution_event.create",
                write_input(
                    file,
                    json!({
                        "run_id":"run-1","event_type":"operation","content":status,
                        "step_status":status,"protocol_step_path":"s/1",
                        "protocol_version_id":"pv1","assay_id":"assay-1",
                        "sample_ids":[sample_id],
                        "parent_sample_id":"parent-1","aliquot_from":"parent-1",
                        "blind_masked":true
                    }),
                )
                .to_str()
                .unwrap(),
                &dir,
            )
            .unwrap();
            assert_eq!(created["record"]["data"]["step_status"], status);
            assert_eq!(created["record"]["data"]["parent_sample_id"], "parent-1");
            assert_eq!(created["record"]["data"]["aliquot_from"], "parent-1");
            assert_eq!(created["record"]["data"]["blind_masked"], true);
        }

        // The exclusion statuses are what the progress read model counts; an
        // unknown status is still rejected.
        let progress = dispatch(
            "workflow.progress",
            write_input("progress.json", json!({"run_id":"run-1"}))
                .to_str()
                .unwrap(),
            &dir,
        )
        .unwrap();
        assert_eq!(progress["steps"][0]["qc_dropped_samples"], 1);
        assert_eq!(progress["steps"][0]["censored_dead_samples"], 1);
        assert_eq!(progress["steps"][0]["rework_samples"], 1);
        let mut statuses: Vec<&str> = progress["steps"][0]["samples"]
            .as_array()
            .unwrap()
            .iter()
            .filter_map(|sample| sample["status"].as_str())
            .collect();
        statuses.sort_unstable();
        assert_eq!(statuses, ["censored_dead", "qc_dropped", "rework"]);

        let rejected = dispatch(
            "execution_event.create",
            write_input(
                "bad.json",
                json!({
                    "run_id":"run-1","event_type":"operation","content":"bad",
                    "step_status":"abandoned","protocol_step_path":"s/1",
                    "protocol_version_id":"pv1","assay_id":"assay-1"
                }),
            )
            .to_str()
            .unwrap(),
            &dir,
        )
        .unwrap_err();
        assert_eq!(rejected.code, "INVALID_INPUT");
        std::fs::remove_dir_all(dir).unwrap();
    }

    #[test]
    fn run_workspace_cli_blind_view_masks_identity_and_projects_occurrences() {
        let dir = std::env::temp_dir().join(format!("asterlab-blind-{}", Uuid::new_v4()));
        std::fs::create_dir_all(&dir).unwrap();
        {
            let db = open_db(&dir).unwrap();
            db.execute(
                "INSERT INTO records(id,kind,project_id,parent_id,revision,data_json,created_at,updated_at) VALUES('project-1','project',NULL,NULL,1,?1,'now','now')",
                [json!({"name":"p","description":""}).to_string()],
            )
            .unwrap();
            db.execute(
                "INSERT INTO records(id,kind,project_id,parent_id,revision,data_json,created_at,updated_at) VALUES('design-1','design','project-1',NULL,1,?1,'now','now')",
                [json!({"title":"d"}).to_string()],
            )
            .unwrap();
            let snapshot = json!({
                "title":"d","objective":"o","experimental_unit":"well",
                "blinding":"研究者与操作者均盲，按笼号编号给药",
                "group_specs":[{"id":"g1","name":"Control","role":"control",
                    "condition":"vehicle","independent_n":3,"technical_repeats":2}],
                "assays":[{"id":"assay-1","title":"CCK-8","endpoint":"OD450",
                    "group_ids":["g1"],"group_names":["Control"]}],
                "sample_allocations":[],"planned_outputs":[],"evidence_conflicts":[]
            });
            db.execute(
                "INSERT INTO records(id,kind,project_id,parent_id,revision,data_json,created_at,updated_at) VALUES('version-1','design_version','project-1','design-1',1,?1,'now','now')",
                [json!({
                    "design_id":"design-1","design_revision":1,"snapshot":snapshot
                })
                .to_string()],
            )
            .unwrap();
            let step = json!({
                "path":"s/1","use_id":"u1","title":"T","instruction":"I",
                "parameters":[],"unresolved_parameter_refs":[],"source_block_version_id":"b1"
            });
            db.execute(
                "INSERT INTO records(id,kind,project_id,parent_id,revision,data_json,created_at,updated_at) VALUES('run-1','run','project-1','design-1',1,?1,'now','now')",
                [json!({
                    "title":"r","design_version_id":"version-1","design_revision":1,
                    "status":"in_progress",
                    "protocol_manifest":[{"assay_id":"assay-1","assay_title":"A",
                        "protocol_version_id":"pv1","steps":[step]}]
                })
                .to_string()],
            )
            .unwrap();
            db.execute(
                "INSERT INTO records(id,kind,project_id,parent_id,revision,data_json,created_at,updated_at) VALUES('plate-1','plate','project-1','run-1',1,?1,'now','now')",
                [json!({"name":"Plate","format":"96","rows":8,"columns":12,"assay_id":"assay-1"}).to_string()],
            )
            .unwrap();
            db.execute(
                "INSERT INTO records(id,kind,project_id,parent_id,revision,data_json,created_at,updated_at) VALUES('well-1','well','project-1','plate-1',1,?1,'now','now')",
                [json!({"well_id":"A1","group_label":"Control","sample_id":"s1","note":""}).to_string()],
            )
            .unwrap();
            db.execute(
                "INSERT INTO records(id,kind,project_id,parent_id,revision,data_json,created_at,updated_at) VALUES('event-1','execution_event','project-1','run-1',1,?1,'now','now')",
                [json!({
                    "event_type":"operation","content":"dose","occurred_at":"2026-11-20T00:00:00Z",
                    "protocol_step_path":"s/1","protocol_version_id":"pv1",
                    "assay_id":"assay-1","occurrence_id":"occ-1","sample_ids":["s1"]
                })
                .to_string()],
            )
            .unwrap();
        }
        let write_input = |name: &str, value: Value| {
            let path = dir.join(name);
            std::fs::write(&path, value.to_string()).unwrap();
            path
        };

        let unblinded = dispatch(
            "run.workspace",
            write_input("plain.json", json!({"id":"run-1"}))
                .to_str()
                .unwrap(),
            &dir,
        )
        .unwrap();
        assert_eq!(unblinded["frozen_design"]["content"]["groups"][0]["name"], "Control");
        assert_eq!(unblinded["occurrences"][0]["occurrence_id"], "occ-1");
        assert_eq!(unblinded["occurrences"][0]["context"]["step_path"], "s/1");
        assert_eq!(unblinded["occurrences"][0]["context"]["target"]["kind"], "many");

        let blind = dispatch(
            "run.workspace",
            write_input("blind.json", json!({"id":"run-1","view":"blind"}))
                .to_str()
                .unwrap(),
            &dir,
        )
        .unwrap();
        assert_eq!(blind["frozen_design"]["content"]["groups"][0]["id"], "");
        assert_eq!(blind["frozen_design"]["content"]["groups"][0]["name"], "");
        assert_eq!(blind["plates"][0]["wells"][0]["group_label"], Value::Null);
        // Blinding removes identity, not execution facts.
        assert_eq!(blind["occurrences"][0]["occurrence_id"], "occ-1");

        let unknown = dispatch(
            "run.workspace",
            write_input("bad-view.json", json!({"id":"run-1","view":"masked"}))
                .to_str()
                .unwrap(),
            &dir,
        )
        .unwrap_err();
        assert_eq!(unknown.code, "INVALID_INPUT");
        std::fs::remove_dir_all(dir).unwrap();
    }

    #[test]
    fn exclusion_and_fov_schemas_match_the_core_contract() {
        let event = input_schema("execution_event.create").unwrap();
        let statuses = event["properties"]["step_status"]["enum"]
            .as_array()
            .unwrap();
        for status in ["qc_dropped", "censored_dead", "rework"] {
            assert!(statuses.contains(&json!(status)), "missing {status}");
        }
        for field in ["parent_sample_id", "aliquot_from", "blind_masked"] {
            assert!(
                event["properties"][field].is_object(),
                "missing event field {field}"
            );
        }
        let progress_schema = output_schema("workflow.progress");
        let progress_statuses = progress_schema["oneOf"][0]["properties"]["result"]["properties"]
            ["steps"]["items"]["properties"]["status"]["enum"]
            .as_array()
            .unwrap();
        for status in ["skipped_by_rule", "qc_dropped", "censored_dead", "rework"] {
            assert!(progress_statuses.contains(&json!(status)), "missing {status}");
        }
        assert_eq!(
            input_schema("asset.link").unwrap()["properties"]["fov_index"]["minimum"],
            json!(1)
        );
        assert_eq!(
            input_schema("run.workspace").unwrap()["properties"]["view"]["enum"],
            json!(["blind", "unblinded"])
        );
    }

    #[test]
    fn asset_link_rejects_fov_zero_for_a_new_link() {
        let dir = std::env::temp_dir().join(format!("asterlab-fov-{}", Uuid::new_v4()));
        std::fs::create_dir_all(&dir).unwrap();
        let task_key = {
            let db = open_db(&dir).unwrap();
            seed_asset_run(&db)
        };
        let input = dir.join("link.json");
        std::fs::write(
            &input,
            json!({
                "root_id":"r","root_dir":dir,"relative_path":"missing.tif",
                "run_id":"run-1","task_key":task_key,
                "well_id":"A1","fov_index":0
            })
            .to_string(),
        )
        .unwrap();
        let error = dispatch("asset.link", input.to_str().unwrap(), &dir).unwrap_err();
        assert_eq!(error.code, "INVALID_INPUT");
        assert!(error.message.contains("fov_index must start at 1"));
        std::fs::remove_dir_all(dir).unwrap();
    }

    #[test]
    fn workflow_inspect_tree_falls_back_for_a_classic_design_run() {
        let dir = std::env::temp_dir().join(format!("asterlab-tree-{}", Uuid::new_v4()));
        std::fs::create_dir_all(&dir).unwrap();
        {
            let db = open_db(&dir).unwrap();
            seed_asset_run(&db);
        }
        let input = dir.join("tree.json");
        std::fs::write(
            &input,
            json!({"design_id":"design-1","run_id":"run-1"}).to_string(),
        )
        .unwrap();
        let tree = dispatch("workflow.inspect_tree", input.to_str().unwrap(), &dir).unwrap();
        assert_eq!(tree["design"], false);
        assert_eq!(tree["runId"], "run-1");
        assert_eq!(tree["revision"], 1);
        assert!(tree["roots"].as_array().unwrap().is_empty());
        assert!(tree["order"].as_array().unwrap().is_empty());
        let issues = tree["issues"].as_array().unwrap();
        assert!(
            issues.iter().any(|issue| issue
                .as_str()
                .unwrap_or("")
                .contains("经典 design.create")),
            "{issues:?}"
        );
        std::fs::remove_dir_all(dir).unwrap();
    }

    fn dir_size(path: &Path) -> u64 {
        fs::read_dir(path)
            .map(|entries| {
                entries
                    .flatten()
                    .map(|entry| {
                        let path = entry.path();
                        if path.is_dir() {
                            dir_size(&path)
                        } else {
                            fs::metadata(&path).map(|meta| meta.len()).unwrap_or(0)
                        }
                    })
                    .sum()
            })
            .unwrap_or(0)
    }

    fn contains_file_named(path: &Path, name: &str) -> bool {
        fs::read_dir(path)
            .map(|entries| {
                entries.flatten().any(|entry| {
                    let path = entry.path();
                    path.file_name().and_then(|value| value.to_str()) == Some(name)
                        || (path.is_dir() && contains_file_named(&path, name))
                })
            })
            .unwrap_or(false)
    }

    #[test]
    fn asset_link_hashes_a_large_file_without_copying_it() {
        use std::io::Write;

        let data_dir = std::env::temp_dir().join(format!("asterlab-asset-db-{}", Uuid::new_v4()));
        let root_dir = std::env::temp_dir().join(format!("asterlab-asset-root-{}", Uuid::new_v4()));
        fs::create_dir_all(&data_dir).unwrap();
        fs::create_dir_all(root_dir.join("d1")).unwrap();
        let mut db = open_db(&data_dir).unwrap();
        let task_key = seed_asset_run(&db);

        let source = root_dir.join("d1").join("large.tif");
        let chunk = vec![0xABu8; 1024 * 1024];
        {
            let mut file = fs::File::create(&source).unwrap();
            for _ in 0..8 {
                file.write_all(&chunk).unwrap();
            }
        }
        let mut hasher = Sha256::new();
        for _ in 0..8 {
            hasher.update(&chunk);
        }
        let expected_hash: String = hasher
            .finalize()
            .iter()
            .map(|byte| format!("{byte:02x}"))
            .collect();

        let before = dir_size(&data_dir);
        let request = AssetLinkCreate {
            root_id: "project-data".into(),
            root_dir: root_dir.display().to_string(),
            relative_path: Some("d1/large.tif".into()),
            path: None,
            run_id: "run-1".into(),
            task_key: task_key.clone(),
            sample_id: Some("sample-1".into()),
            well_id: Some("A1".into()),
            timepoint: Some("t6h".into()),
            fov_index: Some(1),
            actor_id: Some("tester".into()),
        };
        let (record, event) = link_local_asset(&mut db, &request).unwrap();
        let after = dir_size(&data_dir);
        assert!(
            after.saturating_sub(before) < 1024 * 1024,
            "linking copied the file into the data dir: {before} -> {after}"
        );
        assert!(
            !contains_file_named(&data_dir, "large.tif"),
            "the source file must not be copied into the data directory"
        );
        assert_eq!(fs::metadata(&source).unwrap().len(), 8 * 1024 * 1024);

        let data = &record["data"];
        assert_eq!(data["size"], 8 * 1024 * 1024);
        assert_eq!(data["sha256"], expected_hash);
        assert_eq!(data["mediaType"], "image");
        assert_eq!(data["relativePath"], "d1/large.tif");
        assert_eq!(data["rootId"], "project-data");
        assert_eq!(data["link"]["sampleId"], "sample-1");
        assert_eq!(data["link"]["coordinateId"], "A1_t6h_fov01");
        assert!(
            !data.to_string().contains(&root_dir.display().to_string()),
            "absolute root path must not be persisted: {data}"
        );
        assert_eq!(event["data"]["action"], "linked");
        assert_eq!(event["data"]["sha256"], expected_hash);

        let links = list_asset_links(&db, "run-1").unwrap();
        assert_eq!(links.as_array().unwrap().len(), 1);
        assert_eq!(links[0]["sha256"], expected_hash);
        let events = list_asset_events(&db, "run-1", None).unwrap();
        assert_eq!(events.as_array().unwrap().len(), 1);
        assert_eq!(events[0]["data"]["action"], "linked");
        assert_eq!(events[0]["data"]["assetId"], data["id"]);

        // Relinking the same file refreshes the same reference and appends
        // another event instead of creating a second asset or copying bytes.
        let (again, _) = link_local_asset(&mut db, &request).unwrap();
        assert_eq!(again["id"], record["id"]);
        assert_eq!(again["data"]["id"], data["id"]);
        assert_eq!(
            list_asset_events(&db, "run-1", Some("linked"))
                .unwrap()
                .as_array()
                .unwrap()
                .len(),
            2
        );

        let (removed, unlink_event) =
            unlink_asset_link(&mut db, record["id"].as_str().unwrap(), "tester").unwrap();
        assert_eq!(removed["data"]["sha256"], expected_hash);
        assert_eq!(unlink_event["data"]["action"], "unlinked");
        assert_eq!(list_asset_links(&db, "run-1").unwrap(), json!([]));
        let events = list_asset_events(&db, "run-1", None).unwrap();
        let actions: Vec<&str> = events
            .as_array()
            .unwrap()
            .iter()
            .map(|event| event["data"]["action"].as_str().unwrap())
            .collect();
        assert_eq!(actions, ["linked", "linked", "unlinked"]);
        assert_eq!(fs::metadata(&source).unwrap().len(), 8 * 1024 * 1024);

        drop(db);
        fs::remove_dir_all(&data_dir).unwrap();
        fs::remove_dir_all(&root_dir).unwrap();
    }

    #[test]
    fn asset_link_rejects_paths_outside_the_root_without_writing() {
        let mut db = Connection::open_in_memory().unwrap();
        db.execute_batch("CREATE TABLE records(id TEXT PRIMARY KEY,kind TEXT NOT NULL,project_id TEXT,parent_id TEXT,revision INTEGER NOT NULL,data_json TEXT NOT NULL,created_at TEXT NOT NULL,updated_at TEXT NOT NULL);").unwrap();
        let task_key = seed_asset_run(&db);
        let root_dir = std::env::temp_dir().join(format!("asterlab-asset-root-{}", Uuid::new_v4()));
        fs::create_dir_all(&root_dir).unwrap();
        let outside = root_dir
            .parent()
            .unwrap()
            .join(format!("asterlab-asset-outside-{}.txt", Uuid::new_v4()));
        fs::write(&outside, b"secret").unwrap();

        let request = |relative_path: Option<&str>, path: Option<&str>, task: &str| AssetLinkCreate {
            root_id: "project-data".into(),
            root_dir: root_dir.display().to_string(),
            relative_path: relative_path.map(str::to_owned),
            path: path.map(str::to_owned),
            run_id: "run-1".into(),
            task_key: task.to_owned(),
            sample_id: None,
            well_id: None,
            timepoint: None,
            fov_index: None,
            actor_id: None,
        };
        let error = link_local_asset(
            &mut db,
            &request(None, Some(outside.to_str().unwrap()), &task_key),
        )
        .unwrap_err();
        assert!(error.message.contains("outside its root"), "{error:?}");
        let error = link_local_asset(&mut db, &request(Some("../secret.txt"), None, &task_key))
            .unwrap_err();
        assert!(error.message.contains("escape"), "{error:?}");
        let error = link_local_asset(
            &mut db,
            &request(Some("missing.txt"), None, "not-a-task"),
        )
        .unwrap_err();
        assert!(error.message.contains("does not belong"), "{error:?}");
        assert_eq!(list_asset_links(&db, "run-1").unwrap(), json!([]));
        assert_eq!(list_asset_events(&db, "run-1", None).unwrap(), json!([]));

        fs::remove_dir_all(&root_dir).unwrap();
        fs::remove_file(&outside).unwrap();
    }

    const RECORDS_TABLE: &str = "CREATE TABLE records(id TEXT PRIMARY KEY,kind TEXT NOT NULL,project_id TEXT,parent_id TEXT,revision INTEGER NOT NULL,data_json TEXT NOT NULL,created_at TEXT NOT NULL,updated_at TEXT NOT NULL);";

    /// Seeds one Run plus one linked asset at `A1 / t6h / fov 1` and returns
    /// the asset_link record id and the stored payload.
    fn seed_asset_link(db: &mut Connection) -> (String, Value) {
        let task_key = seed_asset_run(db);
        let data = save_asset_link(
            db,
            &json!({
                "asset": {
                    "id": "asset-1",
                    "rootId": "project-data",
                    "relativePath": "d1/a1.tif",
                    "mediaType": "image",
                    "size": 2048,
                    "modTime": "2026-11-15T10:00:00Z",
                    "sha256": "ab".repeat(32),
                    "availability": "available",
                    "link": {
                        "runId": "run-1",
                        "taskKey": task_key,
                        "sampleId": "sample-1",
                        "wellId": "A1",
                        "timepoint": "t6h",
                        "fovIndex": 1
                    },
                    "linkedAt": "2026-11-15T10:00:00Z",
                    "actorId": "tester"
                }
            }),
        )
        .unwrap();
        let record_id: String = db
            .query_row("SELECT id FROM records WHERE kind='asset_link'", [], |row| {
                row.get(0)
            })
            .unwrap();
        (record_id, data)
    }

    fn asset_actions(db: &Connection) -> Vec<String> {
        list_asset_events(db, "run-1", None)
            .unwrap()
            .as_array()
            .unwrap()
            .iter()
            .map(|event| event["data"]["action"].as_str().unwrap().to_owned())
            .collect()
    }

    #[test]
    fn asset_link_update_changes_only_coordinates_and_records_the_previous_values() {
        let mut db = Connection::open_in_memory().unwrap();
        db.execute_batch(RECORDS_TABLE).unwrap();
        let (record_id, linked) = seed_asset_link(&mut db);

        let (record, event) = update_asset_link(
            &mut db,
            &AssetLinkUpdateInput {
                id: record_id.clone(),
                well_id: "B2".into(),
                timepoint: Some(Some("t12h".into())),
                fov_index: Some(Some(2)),
                actor_id: Some("operator".into()),
            },
        )
        .unwrap();

        assert_eq!(record["id"], json!(record_id));
        let data = &record["data"];
        for (field, expected) in [
            ("id", json!(linked["id"])),
            ("rootId", json!("project-data")),
            ("relativePath", json!("d1/a1.tif")),
            ("sha256", json!("ab".repeat(32))),
            ("mediaType", json!("image")),
            ("size", json!(2048)),
            ("availability", json!("available")),
            ("linkedAt", json!("2026-11-15T10:00:00Z")),
        ] {
            assert_eq!(data[field], expected, "{field} must not change");
        }
        assert_eq!(data["link"]["runId"], json!("run-1"));
        assert_eq!(data["link"]["taskKey"], linked["link"]["taskKey"]);
        assert_eq!(data["link"]["sampleId"], json!("sample-1"));
        assert_eq!(data["link"]["wellId"], json!("B2"));
        assert_eq!(data["link"]["timepoint"], json!("t12h"));
        assert_eq!(data["link"]["fovIndex"], json!(2));
        assert_eq!(data["link"]["coordinateId"], json!("B2_t12h_fov02"));

        assert_eq!(event["kind"], json!("asset_link_event"));
        assert_eq!(event["data"]["action"], json!("updated"));
        assert_eq!(event["data"]["assetId"], linked["id"]);
        assert_eq!(event["data"]["sha256"], json!("ab".repeat(32)));
        assert_eq!(event["data"]["actorId"], json!("operator"));
        assert_eq!(event["data"]["link"]["coordinateId"], json!("B2_t12h_fov02"));
        assert_eq!(
            event["data"]["previous"],
            json!({"wellId":"A1","timepoint":"t6h","fovIndex":1,"coordinateId":"A1_t6h_fov01"})
        );

        let workspace = run_workspace(&db, "run-1").unwrap();
        assert_eq!(workspace["linked_assets"].as_array().unwrap().len(), 1);
        assert_eq!(workspace["linked_assets"][0]["asset_id"], linked["id"]);
        assert_eq!(workspace["linked_assets"][0]["well_id"], json!("B2"));
        assert_eq!(
            workspace["linked_assets"][0]["coordinate_id"],
            json!("B2_t12h_fov02")
        );
        assert_eq!(
            list_asset_links(&db, "run-1").unwrap()[0]["sha256"],
            json!("ab".repeat(32))
        );
        assert_eq!(asset_actions(&db), ["linked", "updated"]);
        assert_eq!(
            list_asset_events(&db, "run-1", Some("updated"))
                .unwrap()
                .as_array()
                .unwrap()
                .len(),
            1
        );
    }

    #[test]
    fn asset_link_update_rejects_invalid_coordinates_and_unknown_links_without_writing() {
        let mut db = Connection::open_in_memory().unwrap();
        db.execute_batch(RECORDS_TABLE).unwrap();
        let (record_id, _) = seed_asset_link(&mut db);
        let before = get(&db, &record_id).unwrap();

        let request = |well: &str, timepoint: Option<&str>, fov: Option<u32>| AssetLinkUpdateInput {
            id: record_id.clone(),
            well_id: well.into(),
            timepoint: Some(timepoint.map(str::to_owned)),
            fov_index: Some(fov),
            actor_id: None,
        };
        for (input, expected) in [
            (request("A1", Some("t6h"), Some(0)), "fov_index must start at 1"),
            (request("A1x2", None, None), "孔位格式"),
            (request("", None, None), "孔位格式"),
        ] {
            let error = update_asset_link(&mut db, &input).unwrap_err();
            assert_eq!(error.code, "INVALID_INPUT", "{:?}", error.message);
            assert!(error.message.contains(expected), "{:?}", error.message);
        }

        let mut unknown = request("A1", None, None);
        unknown.id = "asset-link-missing".into();
        assert_eq!(
            update_asset_link(&mut db, &unknown).unwrap_err().code,
            "NOT_FOUND"
        );

        let mut wrong_kind = request("A1", None, None);
        wrong_kind.id = "run-1".into();
        assert_eq!(
            update_asset_link(&mut db, &wrong_kind).unwrap_err().code,
            "INVALID_INPUT"
        );

        // Every rejected call leaves the record and the event log untouched.
        assert_eq!(get(&db, &record_id).unwrap(), before);
        assert_eq!(asset_actions(&db), ["linked"]);

        // An unlinked record is gone for this command too: updating it is
        // NOT_FOUND, never a resurrected link.
        unlink_asset_link(&mut db, &record_id, "tester").unwrap();
        assert_eq!(
            update_asset_link(&mut db, &request("A1", None, None))
                .unwrap_err()
                .code,
            "NOT_FOUND"
        );
        assert_eq!(asset_actions(&db), ["linked", "unlinked"]);
        assert_eq!(list_asset_links(&db, "run-1").unwrap(), json!([]));
    }

    #[test]
    fn asset_link_update_rejects_omitted_coordinate_fields_instead_of_clearing_them() {
        let mut db = Connection::open_in_memory().unwrap();
        db.execute_batch(RECORDS_TABLE).unwrap();
        let (record_id, _) = seed_asset_link(&mut db);
        let before = get(&db, &record_id).unwrap();

        // Only an explicit `null` clears a field; a missing key is rejected.
        for missing in ["timepoint", "fov_index"] {
            let mut value = json!({
                "id": record_id.clone(),
                "well_id": "B2",
                "timepoint": "t12h",
                "fov_index": 2
            });
            value.as_object_mut().unwrap().remove(missing);
            let input: AssetLinkUpdateInput = serde_json::from_value(value).unwrap();
            let error = update_asset_link(&mut db, &input).unwrap_err();
            assert_eq!(error.code, "INVALID_INPUT", "{:?}", error.message);
            assert!(error.message.contains(missing), "{:?}", error.message);
        }
        // `well_id` is structurally required: it never parses when omitted.
        assert!(
            serde_json::from_value::<AssetLinkUpdateInput>(json!({
                "id": record_id.clone(), "timepoint": null, "fov_index": null
            }))
            .is_err()
        );

        assert_eq!(get(&db, &record_id).unwrap(), before);
        assert_eq!(asset_actions(&db), ["linked"]);
    }

    #[test]
    fn asset_link_update_clears_timepoint_and_fov_only_when_explicitly_null() {
        let mut db = Connection::open_in_memory().unwrap();
        db.execute_batch(RECORDS_TABLE).unwrap();
        let (record_id, _) = seed_asset_link(&mut db);

        let (record, event) = update_asset_link(
            &mut db,
            &AssetLinkUpdateInput {
                id: record_id,
                well_id: "C3".into(),
                timepoint: Some(None),
                fov_index: Some(None),
                actor_id: None,
            },
        )
        .unwrap();
        assert_eq!(record["data"]["link"]["wellId"], json!("C3"));
        assert_eq!(record["data"]["link"]["timepoint"], Value::Null);
        assert_eq!(record["data"]["link"]["fovIndex"], Value::Null);
        assert_eq!(record["data"]["link"]["coordinateId"], json!("C3"));
        assert_eq!(record["data"]["link"]["sampleId"], json!("sample-1"));
        assert_eq!(record["data"]["sha256"], json!("ab".repeat(32)));
        assert_eq!(event["data"]["actorId"], json!("tester"));
        assert_eq!(
            event["data"]["previous"],
            json!({"wellId":"A1","timepoint":"t6h","fovIndex":1,"coordinateId":"A1_t6h_fov01"})
        );
    }

    struct AgentEditFixture {
        data_dir: std::path::PathBuf,
        protocol_id: String,
        protocol_version_id: String,
        run_id: String,
    }

    fn seed_agent_edit_fixture(db: &mut Connection) -> AgentEditFixture {
        let data_dir = std::env::temp_dir().join(format!("asterlab-agent-edit-{}", Uuid::new_v4()));
        fs::create_dir_all(&data_dir).unwrap();
        let project = create_project(db, "agent edit").unwrap();
        let project_id = project["id"].as_str().unwrap().to_owned();
        let block = {
            let tx = db.unchecked_transaction().unwrap();
            let result = create_protocol_block_records(
                &tx,
                &project_id,
                &json!({
                    "title": "RNA lysis",
                    "summary": "lysis",
                    "parameter_slots": [{
                        "key": "volume_per_sample",
                        "label": "Volume per sample",
                        "value_type": "number",
                        "unit": "uL",
                        "required": true
                    }],
                    "steps": [{
                        "kind": "text",
                        "use_id": "lyse",
                        "title": "Lyse",
                        "instruction": "Add buffer"
                    }]
                }),
            )
            .unwrap();
            tx.commit().unwrap();
            result
        };
        let block_version_id = block["version"]["id"].as_str().unwrap().to_owned();
        let (protocol_id, protocol_version_id) = {
            let tx = db.unchecked_transaction().unwrap();
            let protocol = insert(
                &tx,
                "protocol",
                Some(&project_id),
                Some(&project_id),
                1,
                &json!({"title": "RNA protocol"}),
            )
            .unwrap();
            let protocol_id = protocol["id"].as_str().unwrap().to_owned();
            let version = insert(
                &tx,
                "protocol_version",
                Some(&project_id),
                Some(&protocol_id),
                1,
                &json!({
                    "protocol_id": protocol_id,
                    "version": 1,
                    "body": "",
                    "root_block_version_id": block_version_id,
                    "root_bindings": []
                }),
            )
            .unwrap();
            tx.commit().unwrap();
            (
                protocol_id,
                version["id"].as_str().unwrap().to_owned(),
            )
        };
        let design = create_design_record(db, &project_id, &json!({"title": "d"})).unwrap();
        let design_id = design["design"]["id"].as_str().unwrap().to_owned();
        let run = {
            let tx = db
                .transaction_with_behavior(rusqlite::TransactionBehavior::Immediate)
                .unwrap();
            let run = create_run_in_transaction(
                &tx,
                &design_id,
                "run",
                None,
                Some(&protocol_version_id),
                None,
                None,
                "independent",
                None,
            )
            .unwrap();
            tx.commit().unwrap();
            run
        };
        AgentEditFixture {
            data_dir,
            protocol_id,
            protocol_version_id,
            run_id: run["id"].as_str().unwrap().to_owned(),
        }
    }

    fn agent_edit_proposal(fixture: &AgentEditFixture) -> Value {
        json!({
            "schema_version": 1,
            "proposal_id": "ae-rna-lyse-001",
            "target": {
                "kind": "protocol",
                "object_id": fixture.protocol_id,
                "location": {
                    "protocol_version_id": fixture.protocol_version_id,
                    "scope": {"kind": "unscoped"},
                    "step_path": "root/lyse"
                }
            },
            "base_version_id": fixture.protocol_version_id,
            "expected_version": 1,
            "origin": {"source": "agent", "rationale": "kit instruction update"},
            "diffs": [{
                "path": "params.volume_per_sample",
                "operation": "set",
                "after": {
                    "parameter_key": "volume_per_sample",
                    "value": 250.0,
                    "unit": "uL",
                    "source": "protocol_fixed",
                    "source_reference": "rna-kit-ifu@2026-03#step-2",
                    "confirmed": false
                }
            }]
        })
    }

    fn agent_edit_confirm_request(
        proposal: Value,
        digest: &str,
        idempotency_key: &str,
    ) -> Value {
        json!({
            "schema_version": 1,
            "proposal": proposal,
            "proposal_digest": digest,
            "idempotency_key": idempotency_key,
            "confirmed_by": "researcher:zhang",
            "acknowledged": [],
            "note": "按试剂盒说明书更新体积。"
        })
    }

    #[test]
    fn agent_edit_commit_appends_a_version_and_keeps_the_frozen_run() {
        let data_dir = std::env::temp_dir().join(format!("asterlab-agent-edit-{}", Uuid::new_v4()));
        fs::create_dir_all(&data_dir).unwrap();
        let mut db = open_db(&data_dir).unwrap();
        let fixture = seed_agent_edit_fixture(&mut db);
        let proposal = agent_edit_proposal(&fixture);
        let validation = validate_agent_edit(&db, proposal.clone()).unwrap();
        assert_eq!(validation["status"], "needs_confirmation");
        let digest = validation["proposal_digest"].as_str().unwrap().to_owned();

        let run_before = get(&db, &fixture.run_id).unwrap();
        let previous_version_before = get(&db, &fixture.protocol_version_id).unwrap();

        let request =
            agent_edit_confirm_request(proposal, &digest, "ae-rna-lyse-001-confirm-1");
        let result = commit_agent_edit(&mut db, request).unwrap();
        assert_eq!(result["new_revision"], 2);
        assert_eq!(result["previous_version_id"], fixture.protocol_version_id);
        assert_eq!(result["replayed"], false);
        assert_eq!(result["confirmed_by"], "researcher:zhang");
        assert_eq!(result["applied"][0]["requested"]["confirmed"], true);
        assert_eq!(
            result["invalidated_refs"],
            json!(["core:binding:unscoped:root/lyse/volume_per_sample"])
        );

        // The frozen Run still points at the previous version and did not move.
        let run_after = get(&db, &fixture.run_id).unwrap();
        assert_eq!(run_after, run_before);
        assert_eq!(
            run_after["data"]["protocol_manifest"][0]["protocol_version_id"],
            fixture.protocol_version_id
        );
        // The previous version is immutable, including its bindings.
        let previous_version_after = get(&db, &fixture.protocol_version_id).unwrap();
        assert_eq!(previous_version_after, previous_version_before);

        let new_version_id: String = db
            .query_row(
                "SELECT id FROM records WHERE kind='protocol_version' AND parent_id=?1 AND revision=2",
                [&fixture.protocol_id],
                |row| row.get(0),
            )
            .unwrap();
        let new_version = get(&db, &new_version_id).unwrap();
        assert_eq!(
            new_version["data"]["root_bindings"],
            json!([{"slot": "volume_per_sample", "value": 250.0, "unit": "uL"}])
        );
        assert_eq!(
            new_version["data"]["agent_edit"]["proposal_id"],
            "ae-rna-lyse-001"
        );
        assert_eq!(
            new_version["data"]["agent_edit"]["proposal_digest"],
            digest
        );
        assert_eq!(
            new_version["data"]["agent_edit"]["confirmed_by"],
            "researcher:zhang"
        );
        assert_eq!(new_version["data"]["agent_edit"]["new_revision"], 2);

        // A stale proposal on the same base is a version conflict, not a
        // silent overwrite.
        let mut stale = agent_edit_confirm_request(
            agent_edit_proposal(&fixture),
            &digest,
            "ae-rna-lyse-001-confirm-2",
        );
        stale["proposal_digest"] = json!(validation["proposal_digest"]);
        let error = commit_agent_edit(&mut db, stale).unwrap_err();
        assert_eq!(error.code, "VERSION_CONFLICT");
        assert_eq!(error.path.as_deref(), Some("expected_version"));
        assert_eq!(error.current_version, Some(2));
        assert_eq!(error.current_version_id.as_deref(), Some(new_version_id.as_str()));
        let count: i64 = db
            .query_row(
                "SELECT COUNT(*) FROM records WHERE kind='protocol_version' AND parent_id=?1",
                [&fixture.protocol_id],
                |row| row.get(0),
            )
            .unwrap();
        assert_eq!(count, 2);

        fs::remove_dir_all(&data_dir).ok();
        fs::remove_dir_all(&fixture.data_dir).ok();
    }

    #[test]
    fn concurrent_agent_edit_commits_leave_exactly_one_winner() {
        let data_dir = std::env::temp_dir().join(format!("asterlab-agent-edit-{}", Uuid::new_v4()));
        fs::create_dir_all(&data_dir).unwrap();
        let mut seed = open_db(&data_dir).unwrap();
        let fixture = seed_agent_edit_fixture(&mut seed);
        let proposal = agent_edit_proposal(&fixture);
        let validation = validate_agent_edit(&seed, proposal.clone()).unwrap();
        let digest = validation["proposal_digest"].as_str().unwrap().to_owned();
        drop(seed);

        let barrier = std::sync::Arc::new(std::sync::Barrier::new(2));
        let mut handles = Vec::new();
        for key in ["ae-concurrent-a", "ae-concurrent-b"] {
            let data_dir = data_dir.clone();
            let proposal = proposal.clone();
            let digest = digest.clone();
            let barrier = barrier.clone();
            handles.push(std::thread::spawn(move || {
                let mut db = open_db(&data_dir).unwrap();
                barrier.wait();
                commit_agent_edit(
                    &mut db,
                    agent_edit_confirm_request(proposal, &digest, key),
                )
            }));
        }
        let results = handles
            .into_iter()
            .map(|handle| handle.join().unwrap())
            .collect::<Vec<_>>();
        let successes = results.iter().filter(|result| result.is_ok()).count();
        let conflicts = results
            .iter()
            .filter(|result| {
                result
                    .as_ref()
                    .is_err_and(|error| error.code == "VERSION_CONFLICT")
            })
            .count();
        assert_eq!(successes, 1, "{results:?}");
        assert_eq!(conflicts, 1, "{results:?}");

        let db = open_db(&data_dir).unwrap();
        let count: i64 = db
            .query_row(
                "SELECT COUNT(*) FROM records WHERE kind='protocol_version' AND parent_id=?1",
                [&fixture.protocol_id],
                |row| row.get(0),
            )
            .unwrap();
        assert_eq!(count, 2, "exactly one new version");

        fs::remove_dir_all(&data_dir).ok();
        fs::remove_dir_all(&fixture.data_dir).ok();
    }
    fn migration_evidence_path(file: &str) -> PathBuf {
        let relative = Path::new("docs/release/evidence/QA-05").join(file);
        Path::new(env!("CARGO_MANIFEST_DIR"))
            .ancestors()
            .map(|directory| directory.join(&relative))
            .find(|path| path.is_file())
            .unwrap_or_else(|| {
                panic!(
                    "migration evidence fixture not found: {}",
                    relative.display()
                )
            })
    }

    fn legacy_v0_fixture_path() -> PathBuf {
        migration_evidence_path("legacy-v0.sqlite")
    }

    fn migration_test_dir() -> PathBuf {
        std::env::temp_dir().join(format!("asterlab-schema-migration-{}", Uuid::new_v4()))
    }

    type LegacyRecordRow = (String, String, Option<String>, i64, String, String, String);

    fn legacy_rows(db: &Connection) -> Vec<LegacyRecordRow> {
        let mut statement = db
            .prepare(
                "SELECT id,kind,parent_id,revision,data_json,created_at,updated_at \
                 FROM records ORDER BY id",
            )
            .unwrap();
        statement
            .query_map([], |row| {
                Ok((
                    row.get(0)?,
                    row.get(1)?,
                    row.get(2)?,
                    row.get(3)?,
                    row.get(4)?,
                    row.get(5)?,
                    row.get(6)?,
                ))
            })
            .unwrap()
            .collect::<rusqlite::Result<Vec<_>>>()
            .unwrap()
    }

    fn database_schema(db: &Connection) -> Vec<(String, String, Option<String>)> {
        let mut statement = db
            .prepare(
                "SELECT type,name,sql FROM sqlite_master \
                 WHERE name NOT LIKE 'sqlite_%' ORDER BY type,name",
            )
            .unwrap();
        statement
            .query_map([], |row| Ok((row.get(0)?, row.get(1)?, row.get(2)?)))
            .unwrap()
            .collect::<rusqlite::Result<Vec<_>>>()
            .unwrap()
    }

    fn sqlite_user_version(db: &Connection) -> i64 {
        db.query_row("PRAGMA user_version", [], |row| row.get(0))
            .unwrap()
    }

    fn sha256_file(path: &Path) -> String {
        format!("{:x}", Sha256::digest(fs::read(path).unwrap()))
    }

    #[test]
    fn schema_migration_legacy_v0() {
        let fixture = legacy_v0_fixture_path();
        let fixture_hash = sha256_file(&fixture);
        let source = Connection::open(&fixture).unwrap();
        let original_rows = legacy_rows(&source);
        assert_eq!(sqlite_user_version(&source), 0);
        drop(source);

        let data_dir = migration_test_dir();
        fs::create_dir_all(&data_dir).unwrap();
        fs::copy(&fixture, data_dir.join("labworkbench.sqlite")).unwrap();

        let db = open_db(&data_dir).expect("legacy v0 database should migrate");
        assert_eq!(
            db.query_row("PRAGMA integrity_check", [], |row| row.get::<_, String>(0))
                .unwrap(),
            "ok"
        );
        assert_eq!(sqlite_user_version(&db), 1);
        assert_eq!(legacy_rows(&db), original_rows);
        assert_eq!(
            db.query_row(
                "SELECT project_id FROM records WHERE id='design-legacy'",
                [],
                |row| row.get::<_, Option<String>>(0),
            )
            .unwrap()
            .as_deref(),
            Some("project-legacy")
        );
        assert_eq!(
            db.query_row(
                "SELECT COUNT(*) FROM records \
                 WHERE kind <> 'project' AND project_id IS NOT 'project-legacy'",
                [],
                |row| row.get::<_, i64>(0),
            )
            .unwrap(),
            0
        );
        drop(db);

        let restarted = open_db(&data_dir).expect("migrated database should reopen");
        assert_eq!(sqlite_user_version(&restarted), 1);
        assert_eq!(
            restarted
                .query_row("PRAGMA integrity_check", [], |row| row.get::<_, String>(0))
                .unwrap(),
            "ok"
        );
        assert_eq!(legacy_rows(&restarted), original_rows);
        drop(restarted);

        assert_eq!(sha256_file(&fixture), fixture_hash, "source fixture changed");
        fs::remove_dir_all(data_dir).unwrap();
    }

    #[test]
    fn failed_schema_migration_preserves_database() {
        let fixture = legacy_v0_fixture_path();
        let data_dir = migration_test_dir();
        fs::create_dir_all(&data_dir).unwrap();
        let db_file = data_dir.join("labworkbench.sqlite");
        fs::copy(&fixture, &db_file).unwrap();
        {
            let db = Connection::open(&db_file).unwrap();
            let failure_sql =
                fs::read_to_string(migration_evidence_path("failure-v0.sql")).unwrap();
            db.execute_batch(&failure_sql).unwrap();
        }

        let before_hash = sha256_file(&db_file);
        let before = Connection::open(&db_file).unwrap();
        let before_schema = database_schema(&before);
        let before_rows = legacy_rows(&before);
        let before_count: i64 = before
            .query_row("SELECT COUNT(*) FROM records", [], |row| row.get(0))
            .unwrap();
        let before_version = sqlite_user_version(&before);
        drop(before);

        let error = open_db(&data_dir)
            .err()
            .expect("injected migration failure should abort opening the database");
        assert!(error.message.contains("injected legacy schema migration failure"));

        let after = Connection::open(&db_file).unwrap();
        assert_eq!(database_schema(&after), before_schema);
        assert_eq!(legacy_rows(&after), before_rows);
        assert_eq!(
            after
                .query_row("SELECT COUNT(*) FROM records", [], |row| row.get::<_, i64>(0))
                .unwrap(),
            before_count
        );
        assert_eq!(sqlite_user_version(&after), before_version);
        drop(after);
        assert_eq!(sha256_file(&db_file), before_hash);
        fs::remove_dir_all(data_dir).unwrap();
    }

    #[test]
    fn protocol_block_content_accepts_legacy_and_valid_specs_but_rejects_invalid_specs() {
        let db = Connection::open_in_memory().unwrap();
        let legacy: ProtocolStep = serde_json::from_value(json!({
            "kind": "text",
            "use_id": "legacy",
            "title": "Legacy",
            "instruction": "No sidecar"
        }))
        .unwrap();
        validate_block_content(&db, "project-1", &[], &[legacy]).unwrap();

        let valid: ProtocolStep = serde_json::from_value(json!({
            "kind": "text",
            "use_id": "typed",
            "title": "Typed",
            "instruction": "Has a sidecar",
            "operation_spec": {
                "schema_version": 1,
                "operation": {
                    "kind": "transfer",
                    "volume": { "value": 10.0, "unit": "uL" },
                    "source": { "container_id": "tube-1" },
                    "destination": { "container_id": "plate-1" }
                }
            }
        }))
        .unwrap();
        validate_block_content(&db, "project-1", &[], &[valid]).unwrap();

        let invalid: ProtocolStep = serde_json::from_value(json!({
            "kind": "text",
            "use_id": "invalid",
            "title": "Invalid",
            "instruction": "Wrong unit",
            "operation_spec": {
                "schema_version": 1,
                "operation": {
                    "kind": "transfer",
                    "volume": { "value": 10.0, "unit": "min" },
                    "source": { "container_id": "tube-1" },
                    "destination": { "container_id": "plate-1" }
                }
            }
        }))
        .unwrap();
        let error = validate_block_content(&db, "project-1", &[], &[invalid]).unwrap_err();
        assert_eq!(error.code, "INVALID_INPUT");
        assert!(error.message.contains("steps[0].operation_spec.operation.volume"));
        assert!(error.message.contains("unit_mismatch"));
    }

    fn seed_readout_runs(db: &Connection) {
        db.execute_batch(
            "CREATE TABLE IF NOT EXISTS records(id TEXT PRIMARY KEY,kind TEXT NOT NULL,project_id TEXT,parent_id TEXT,revision INTEGER NOT NULL,data_json TEXT NOT NULL,created_at TEXT NOT NULL,updated_at TEXT NOT NULL);",
        )
        .unwrap();
        db.execute(
            "INSERT INTO records VALUES('project-1','project',NULL,NULL,1,?1,'now','now')",
            [json!({"name":"p","description":""}).to_string()],
        )
        .unwrap();
        db.execute(
            "INSERT INTO records VALUES('design-1','design','project-1',NULL,1,?1,'now','now')",
            [json!({"title":"d"}).to_string()],
        )
        .unwrap();
        db.execute(
            "INSERT INTO records VALUES('version-1','design_version','project-1','design-1',1,?1,'now','now')",
            [json!({
                "design_id":"design-1","design_revision":1,
                "snapshot":{
                    "title":"d","objective":"o","experimental_unit":"well",
                    "group_specs":[],"sample_allocations":[],"planned_outputs":[],
                    "assays":[{"id":"assay-1","title":"A","timepoints":["D1"],
                        "group_ids":[],"timepoint_relation":"same_physical_sample_repeated"}]
                }
            })
            .to_string()],
        )
        .unwrap();
        let step = json!({
            "path":"run/read","use_id":"u1","title":"Read","instruction":"read",
            "parameters":[],"unresolved_parameter_refs":[],
            "source_block_version_id":"b1","time_scope":"per_timepoint"
        });
        for run_id in ["run-1", "run-2"] {
            db.execute(
                "INSERT INTO records VALUES(?1,'run','project-1','design-1',1,?2,'now','now')",
                params![
                    run_id,
                    json!({
                        "title":run_id,"design_version_id":"version-1","design_revision":1,
                        "status":"in_progress",
                        "protocol_manifest":[{
                            "assay_id":"assay-1","assay_title":"A","protocol_version_id":"pv1",
                            "steps":[step.clone()]
                        }]
                    })
                    .to_string()
                ],
            )
            .unwrap();
        }
        db.execute(
            "INSERT INTO records VALUES('plate-1','plate','project-1','run-1',1,?1,'now','now')",
            [json!({"name":"96","format":"96","rows":8,"columns":12,
                "assay_id":"assay-1","timepoint":"D1"})
            .to_string()],
        )
        .unwrap();
    }

    fn readout_submit_request(run_id: &str) -> ReadoutSubmitRequest {
        ReadoutSubmitRequest {
            run_id: run_id.to_owned(),
            step_path: "run/read".to_owned(),
            endpoint: "od450".to_owned(),
            protocol_version_id: "pv1".to_owned(),
            assay_id: Some("assay-1".to_owned()),
            planned_output_id: None,
            timepoint: Some("D1".to_owned()),
            sample_id: Some("S-1".to_owned()),
            value: Some(1.234),
            qualifier: None,
            unit: Some("AU".to_owned()),
            content: None,
            occurred_at: None,
            operator: None,
        }
    }

    fn readout_batch_request() -> ReadoutBatchRequest {
        ReadoutBatchRequest {
            run_id: "run-1".to_owned(),
            step_path: "run/read".to_owned(),
            protocol_version_id: "pv1".to_owned(),
            plate_id: "plate-1".to_owned(),
            schema_id: "cck8.plate96.od450.v1".to_owned(),
            schema_version: 1,
            assay_id: Some("assay-1".to_owned()),
            timepoint: Some("D1".to_owned()),
            batch_id: Some("batch-1".to_owned()),
            content: None,
            occurred_at: None,
            operator: None,
            cells: vec![
                asterlab_core::reading_facts::ReadingCellInput {
                    row: 0,
                    column: 0,
                    field: "od450".to_owned(),
                    value: Some(1.0),
                    unit: Some("AU".to_owned()),
                    sample_id: Some("S-1".to_owned()),
                    group: Some("g1".to_owned()),
                    replicate: Some(1),
                    status: None,
                },
                asterlab_core::reading_facts::ReadingCellInput {
                    row: 0,
                    column: 1,
                    field: "od450".to_owned(),
                    value: None,
                    unit: Some("AU".to_owned()),
                    sample_id: Some("S-2".to_owned()),
                    group: Some("g2".to_owned()),
                    replicate: Some(1),
                    status: None,
                },
            ],
            csv: None,
        }
    }

    fn execution_event_count(db: &Connection) -> i64 {
        db.query_row(
            "SELECT COUNT(*) FROM records WHERE kind='execution_event'",
            [],
            |row| row.get(0),
        )
        .unwrap()
    }

    struct StepEditFixture {
        data_dir: std::path::PathBuf,
        project_id: String,
        protocol_version_id: String,
        root_version_id: String,
        child_version_id: String,
        wash_version_id: String,
        run_id: String,
    }

    fn seed_step_edit_fixture(db: &mut Connection) -> StepEditFixture {
        let data_dir = std::env::temp_dir().join(format!("asterlab-step-edit-{}", Uuid::new_v4()));
        fs::create_dir_all(&data_dir).unwrap();
        let project = create_project(db, "step edit").unwrap();
        let project_id = project["id"].as_str().unwrap().to_owned();
        let child_version_id = {
            let tx = db.unchecked_transaction().unwrap();
            let result = create_protocol_block_records(
                &tx,
                &project_id,
                &json!({
                    "title": "Lysis",
                    "summary": "lysis",
                    "parameter_slots": [{
                        "key": "volume_per_sample",
                        "label": "Volume per sample",
                        "value_type": "number",
                        "unit": "uL",
                        "required": true
                    }],
                    "steps": [{
                        "kind": "text",
                        "use_id": "iris",
                        "title": "Iris",
                        "instruction": "Aspirate supernatant"
                    }]
                }),
            )
            .unwrap();
            tx.commit().unwrap();
            result["version"]["id"].as_str().unwrap().to_owned()
        };
        let wash_version_id = {
            let tx = db.unchecked_transaction().unwrap();
            let result = create_protocol_block_records(
                &tx,
                &project_id,
                &json!({
                    "title": "Wash",
                    "summary": "wash",
                    "steps": [{
                        "kind": "text",
                        "use_id": "rinse",
                        "title": "Rinse",
                        "instruction": "Rinse once"
                    }]
                }),
            )
            .unwrap();
            tx.commit().unwrap();
            result["version"]["id"].as_str().unwrap().to_owned()
        };
        let root_version_id = {
            let tx = db.unchecked_transaction().unwrap();
            let result = create_protocol_block_records(
                &tx,
                &project_id,
                &json!({
                    "title": "RNA protocol block",
                    "summary": "root",
                    "parameter_slots": [{
                        "key": "volume_per_sample",
                        "label": "Volume per sample",
                        "value_type": "number",
                        "unit": "uL",
                        "required": true
                    }],
                    "steps": [
                        {"kind": "text", "use_id": "prep", "title": "Prep", "instruction": "Prepare bench"},
                        {"kind": "block_use", "use_id": "lyse", "block_version_id": child_version_id,
                         "bindings": [{"slot": "volume_per_sample", "value": 200.0, "unit": "uL"}]}
                    ]
                }),
            )
            .unwrap();
            tx.commit().unwrap();
            result["version"]["id"].as_str().unwrap().to_owned()
        };
        let protocol_version_id = {
            let tx = db.unchecked_transaction().unwrap();
            let protocol = insert(
                &tx,
                "protocol",
                Some(&project_id),
                Some(&project_id),
                1,
                &json!({"title": "RNA protocol"}),
            )
            .unwrap();
            let protocol_id = protocol["id"].as_str().unwrap().to_owned();
            let version = insert(
                &tx,
                "protocol_version",
                Some(&project_id),
                Some(&protocol_id),
                1,
                &json!({
                    "protocol_id": protocol_id,
                    "version": 1,
                    "body": "",
                    "root_block_version_id": root_version_id,
                    "root_bindings": []
                }),
            )
            .unwrap();
            tx.commit().unwrap();
            version["id"].as_str().unwrap().to_owned()
        };
        let design = create_design_record(db, &project_id, &json!({"title": "d"})).unwrap();
        let design_id = design["design"]["id"].as_str().unwrap().to_owned();
        let run = {
            let tx = db
                .transaction_with_behavior(rusqlite::TransactionBehavior::Immediate)
                .unwrap();
            let run = create_run_in_transaction(
                &tx,
                &design_id,
                "run",
                None,
                Some(&protocol_version_id),
                None,
                None,
                "independent",
                None,
            )
            .unwrap();
            tx.commit().unwrap();
            run
        };
        StepEditFixture {
            data_dir,
            project_id,
            protocol_version_id,
            root_version_id,
            child_version_id,
            wash_version_id,
            run_id: run["id"].as_str().unwrap().to_owned(),
        }
    }

    fn step_edit_request(fixture: &StepEditFixture) -> Value {
        json!({
            "schema_version": 1,
            "edit_id": "sde-rna-001",
            "block_version_id": fixture.root_version_id,
            "expected_version": 1,
            "origin": {"source": "agent", "rationale": "add a wash substep"},
            "ops": [{
                "op": "reuse",
                "path": "root/lyse",
                "position": 1,
                "use_id": "wash",
                "block_version_id": fixture.wash_version_id,
                "bindings": []
            }]
        })
    }

    #[test]
    fn step_edit_schema_is_registered_for_validate_and_commit() {
        assert!(input_schema("step.edit").is_some());
        let ops = input_schema("step.edit").unwrap()["properties"]["ops"].clone();
        let variants = ops["items"]["oneOf"].as_array().unwrap();
        let kinds: Vec<&str> = variants
            .iter()
            .map(|variant| variant["properties"]["op"]["const"].as_str().unwrap())
            .collect();
        assert_eq!(kinds, ["insert", "reorder", "reuse", "bind"]);
        let results = result_schema("step.edit")["oneOf"].as_array().unwrap().len();
        assert_eq!(results, 2, "validate and commit result shapes");
    }

    #[test]
    fn readout_tools_are_discoverable_and_write() {
        let specs = tool_specs();
        let names: Vec<&str> = specs
            .iter()
            .filter_map(|spec| spec["name"].as_str())
            .collect();
        for tool in ["readout.submit", "readout.batch"] {
            assert!(names.contains(&tool), "missing {tool}");
            assert!(input_schema(tool).is_some(), "missing input schema for {tool}");
            assert!(tool_writes(tool), "{tool} must be a write tool");
        }
        assert_eq!(
            result_schema("readout.batch")["required"],
            json!(["record", "idempotency_key", "replayed", "accepted"])
        );
        for tool in ["readout.submit", "readout.batch"] {
            assert_eq!(
                output_schema(tool)["oneOf"][1]["properties"]["error"]["properties"]["issues"]
                    ["items"]["required"],
                json!(["code", "message"])
            );
        }
    }

    #[test]
    fn readout_submit_replays_the_first_record_and_isolates_runs() {
        let mut db = Connection::open_in_memory().unwrap();
        seed_readout_runs(&db);

        let first = submit_readout(&mut db, &readout_submit_request("run-1")).unwrap();
        assert_eq!(first["replayed"], false);
        let first_id = first["record"]["id"].as_str().unwrap().to_owned();
        assert_eq!(first["record"]["data"]["event_type"], "measurement");
        assert_eq!(first["record"]["data"]["readouts"][0]["value"], 1.234);

        let replay = submit_readout(&mut db, &readout_submit_request("run-1")).unwrap();
        assert_eq!(replay["replayed"], true);
        assert_eq!(replay["record"]["id"], first_id.as_str());
        assert_eq!(
            replay["idempotency_key"],
            first["idempotency_key"]
        );
        assert_eq!(execution_event_count(&db), 1);

        let other = submit_readout(&mut db, &readout_submit_request("run-2")).unwrap();
        assert_eq!(other["replayed"], false);
        assert_ne!(other["record"]["id"], first_id.as_str());
        assert_ne!(other["idempotency_key"], first["idempotency_key"]);
        assert_eq!(execution_event_count(&db), 2);
        assert_eq!(other["record"]["parent_id"], "run-2");
    }

    #[test]
    fn readout_batch_writes_one_event_or_nothing() {
        let mut db = Connection::open_in_memory().unwrap();
        seed_readout_runs(&db);

        let mut duplicate = readout_batch_request();
        duplicate.cells[1].row = 0;
        duplicate.cells[1].column = 0;
        let error = submit_readout_batch(&mut db, &duplicate).unwrap_err();
        assert_eq!(error.code, "INVALID_INPUT");
        assert!(error.issues.iter().any(|issue| issue["code"] == "duplicate_position"));
        assert_eq!(execution_event_count(&db), 0, "a rejected batch writes nothing");

        let mut stale = readout_batch_request();
        stale.protocol_version_id = "pv-other".to_owned();
        let error = submit_readout_batch(&mut db, &stale).unwrap_err();
        assert_eq!(error.code, "INVALID_INPUT");
        assert_eq!(execution_event_count(&db), 0, "event creation failure rolls back");

        let accepted = submit_readout_batch(&mut db, &readout_batch_request()).unwrap();
        assert_eq!(accepted["replayed"], false);
        assert_eq!(accepted["accepted"], 2);
        assert_eq!(accepted["record"]["data"]["result_cells"].as_array().unwrap().len(), 2);
        assert_eq!(execution_event_count(&db), 1);

        let replay = submit_readout_batch(&mut db, &readout_batch_request()).unwrap();
        assert_eq!(replay["replayed"], true);
        assert_eq!(replay["record"]["id"], accepted["record"]["id"]);
        assert_eq!(execution_event_count(&db), 1);
    }

    #[test]
    fn readout_batch_accepts_csv_cells() {
        let mut db = Connection::open_in_memory().unwrap();
        seed_readout_runs(&db);
        let mut request = readout_batch_request();
        request.cells = Vec::new();
        request.csv = Some(
            "row,column,field,value,unit,sample_id,group,replicate\n\
             0,0,od450,1.0,AU,S-1,g1,1\n\
             0,1,od450,,AU,S-2,g2,1\n"
                .to_owned(),
        );
        let accepted = submit_readout_batch(&mut db, &request).unwrap();
        assert_eq!(accepted["accepted"], 2);
        let cells = accepted["record"]["data"]["result_cells"].as_array().unwrap();
        assert_eq!(cells[1]["status"], "missing");
        assert_eq!(cells[1]["value"], Value::Null);
    }
    #[test]
    fn step_edit_tool_is_registered() {
        let specs = tool_specs();
        let names: Vec<&str> = specs
            .iter()
            .filter_map(|spec| spec["name"].as_str())
            .collect();
        assert!(names.contains(&"step.edit"));
        assert!(!names.contains(&"protocol.block.step_edit"));
        assert!(
            tool_effects("step.edit").contains("只读"),
            "the effect text must state the no-confirmation path is read-only"
        );
    }

    #[test]
    fn step_edit_commit_creates_child_and_parent_versions_and_keeps_the_frozen_run() {
        let data_dir =
            std::env::temp_dir().join(format!("asterlab-step-edit-db-{}", Uuid::new_v4()));
        let mut db = open_db(&data_dir).unwrap();
        let fixture = seed_step_edit_fixture(&mut db);
        let request = step_edit_request(&fixture);

        let preview = step_edit_command(&mut db, request.clone()).unwrap();
        assert_eq!(preview["status"], "needs_confirmation");
        assert!(preview["edit_digest"].as_str().unwrap().starts_with("sha256:"));
        let planned: Vec<&str> = preview["blocks"]
            .as_array()
            .unwrap()
            .iter()
            .map(|block| block["block_id"].as_str().unwrap())
            .collect();
        assert_eq!(planned.len(), 2);
        assert_eq!(
            preview["blocks"][0]["based_on_version_id"],
            json!(fixture.child_version_id)
        );
        assert_eq!(
            preview["blocks"][0]["order"],
            json!(["iris", "wash"]),
            "the deep edit lands inside the child block"
        );
        let digest = preview["edit_digest"].as_str().unwrap().to_owned();

        let run_before = get(&db, &fixture.run_id).unwrap();
        let protocol_version_before = get(&db, &fixture.protocol_version_id).unwrap();
        let base_root_before = get(&db, &fixture.root_version_id).unwrap();
        let base_child_before = get(&db, &fixture.child_version_id).unwrap();

        let mut confirmed = request.clone();
        confirmed["confirmation"] = json!({
            "edit_digest": digest,
            "confirmed_by": "researcher:zhang",
            "idempotency_key": "sde-rna-001-confirm-1",
            "note": "add wash"
        });
        let committed = step_edit_command(&mut db, confirmed.clone()).unwrap();
        assert_eq!(committed["replayed"], false);
        assert_eq!(committed["confirmed_by"], "researcher:zhang");
        assert_eq!(committed["new_revision"], 2);
        assert_eq!(committed["previous_version_id"], json!(fixture.root_version_id));
        let created = committed["created_versions"].as_array().unwrap();
        assert_eq!(created.len(), 2);
        let child_block_id = get(&db, &fixture.child_version_id).unwrap()["parent_id"].clone();
        assert_eq!(created[0]["block_id"], child_block_id);
        assert_eq!(created[0]["based_on_version_id"], json!(fixture.child_version_id));
        let child_new = created[0]["version_id"].as_str().unwrap().to_owned();
        let root_new = committed["new_root_block_version_id"].as_str().unwrap().to_owned();
        assert_eq!(created[1]["version_id"], json!(root_new));

        // Immutable base versions and the frozen Run did not move.
        assert_eq!(get(&db, &fixture.run_id).unwrap(), run_before);
        assert_eq!(
            get(&db, &fixture.protocol_version_id).unwrap(),
            protocol_version_before
        );
        assert_eq!(get(&db, &fixture.root_version_id).unwrap(), base_root_before);
        assert_eq!(get(&db, &fixture.child_version_id).unwrap(), base_child_before);
        assert_eq!(
            get(&db, &fixture.protocol_version_id).unwrap()["data"]["root_block_version_id"],
            json!(fixture.root_version_id)
        );

        let new_root = get(&db, &root_new).unwrap();
        assert_eq!(
            new_root["data"]["based_on_version_id"],
            json!(fixture.root_version_id)
        );
        assert_eq!(
            new_root["data"]["steps"][1]["block_version_id"],
            json!(child_new),
            "the parent version references the child version created in the same commit"
        );
        assert_eq!(
            new_root["data"]["step_edit"]["edit_id"],
            json!("sde-rna-001")
        );
        assert_eq!(new_root["data"]["step_edit"]["edit_digest"], json!(digest));
        assert_eq!(
            new_root["data"]["step_edit"]["confirmed_by"],
            json!("researcher:zhang")
        );

        // The protocol still previews through the old root; the new block does too.
        let preview_new = block_preview(&db, &root_new, &[]).unwrap();
        assert_eq!(preview_new["steps"].as_array().unwrap().len(), 3);
        let paths: Vec<&str> = preview_new["steps"]
            .as_array()
            .unwrap()
            .iter()
            .map(|step| step["path"].as_str().unwrap())
            .collect();
        assert_eq!(
            paths,
            ["root/prep", "root/lyse/iris", "root/lyse/wash/rinse"]
        );

        // Idempotent replay appends nothing.
        let replayed = step_edit_command(&mut db, confirmed).unwrap();
        assert_eq!(replayed["replayed"], true);
        assert_eq!(replayed["created_versions"], json!(created));
        let version_count: i64 = db
            .query_row(
                "SELECT COUNT(*) FROM records WHERE kind='protocol_block_version'",
                [],
                |row| row.get(0),
            )
            .unwrap();
        assert_eq!(version_count, 5, "three seeds plus two new versions");

        // The old base version is no longer a valid edit target.
        let stale = step_edit_command(&mut db, request).unwrap();
        assert_eq!(stale["status"], "blocked");
        assert_eq!(stale["issues"][0]["code"], "STALE_REFERENCE");

        fs::remove_dir_all(&fixture.data_dir).ok();
        drop(db);
        fs::remove_dir_all(&data_dir).ok();
    }

    #[test]
    fn step_edit_rejects_a_version_conflict_and_a_wrong_unit_without_writing() {
        let data_dir =
            std::env::temp_dir().join(format!("asterlab-step-edit-db-{}", Uuid::new_v4()));
        let mut db = open_db(&data_dir).unwrap();
        let fixture = seed_step_edit_fixture(&mut db);

        let mut stale = step_edit_request(&fixture);
        stale["expected_version"] = json!(2);
        let blocked = step_edit_command(&mut db, stale).unwrap();
        assert_eq!(blocked["status"], "blocked");
        assert_eq!(blocked["issues"][0]["code"], "VERSION_CONFLICT");
        assert_eq!(blocked["issues"][0]["path"], "expected_version");

        let mut wrong_unit = step_edit_request(&fixture);
        wrong_unit["ops"] = json!([{
            "op": "bind",
            "path": "root/lyse",
            "bindings": [{"slot": "volume_per_sample", "value": 0.2, "unit": "mL"}]
        }]);
        let blocked = step_edit_command(&mut db, wrong_unit).unwrap();
        assert_eq!(blocked["status"], "blocked");
        assert_eq!(blocked["issues"][0]["code"], "UNIT_MISMATCH");

        let count: i64 = db
            .query_row(
                "SELECT COUNT(*) FROM records WHERE kind='protocol_block_version'",
                [],
                |row| row.get(0),
            )
            .unwrap();
        assert_eq!(count, 3, "rejected edits append nothing");

        fs::remove_dir_all(&fixture.data_dir).ok();
        drop(db);
        fs::remove_dir_all(&data_dir).ok();
    }

    #[test]
    fn pantry_commands_project_expiry_and_export_stable_label_codes() {
        let dir = std::env::temp_dir().join(format!("pantry-cli-{}", Uuid::new_v4()));
        std::fs::create_dir_all(&dir).unwrap();
        let write_input = |name: &str, value: Value| {
            let path = dir.join(name);
            std::fs::write(&path, value.to_string()).unwrap();
            path
        };
        let created = dispatch(
            "pantry.upsert",
            write_input(
                "create.json",
                json!({
                    "name":"DMEM",
                    "location":"4C shelf 2",
                    "purchased_on":"2026-01-01",
                    "opened_on":"2026-11-01",
                    "shelf_life_days":3650,
                    "opened_shelf_life_days":21
                }),
            )
            .to_str()
            .unwrap(),
            &dir,
        )
        .unwrap();
        assert_eq!(created["item"]["expires_on"], "2026-11-22");
        assert_eq!(created["item"]["expiry_basis"], "opened_shelf_life");
        assert_eq!(created["item"]["revision"], 1);
        let item_id = created["item"]["id"].as_str().unwrap().to_owned();
        let label_code = created["item"]["label_code"].as_str().unwrap().to_owned();
        assert_eq!(label_code, pantry::base_label_code(&item_id));

        let listed = dispatch(
            "pantry.list",
            write_input("list.json", json!({"as_of":"2026-11-15"}))
                .to_str()
                .unwrap(),
            &dir,
        )
        .unwrap();
        assert_eq!(listed["as_of"], "2026-11-15");
        assert_eq!(listed["near_expiry_days"], pantry::NEAR_EXPIRY_DAYS);
        assert_eq!(listed["items"][0]["status"], "near_expiry");
        assert_eq!(listed["items"][0]["days_remaining"], 7);

        let stale = dispatch(
            "pantry.upsert",
            write_input(
                "stale.json",
                json!({"id":item_id,"name":"DMEM","expected_revision":7}),
            )
            .to_str()
            .unwrap(),
            &dir,
        )
        .unwrap_err();
        assert_eq!(stale.code, "REVISION_CONFLICT");
        let invalid = dispatch(
            "pantry.upsert",
            write_input(
                "invalid.json",
                json!({"name":"DMEM","purchased_on":"2026-02-30"}),
            )
            .to_str()
            .unwrap(),
            &dir,
        )
        .unwrap_err();
        assert_eq!(invalid.code, "INVALID_INPUT");

        let first_export = dispatch(
            "label.export",
            write_input("export.json", json!({"as_of":"2026-11-15"}))
                .to_str()
                .unwrap(),
            &dir,
        )
        .unwrap();
        assert_eq!(first_export["schema_version"], 1);
        assert_eq!(first_export["labels"][0]["label_code"], label_code);
        assert_eq!(first_export["labels"][0]["pantry_item_id"], item_id);
        assert_eq!(first_export["labels"][0]["status"], "near_expiry");
        let second_export = dispatch(
            "label.export",
            write_input("export2.json", json!({"as_of":"2026-11-15"}))
                .to_str()
                .unwrap(),
            &dir,
        )
        .unwrap();
        assert_eq!(
            first_export["labels"], second_export["labels"],
            "label codes must not drift between exports"
        );

        std::fs::remove_dir_all(dir).unwrap();
        for tool in ["pantry.list", "pantry.upsert", "label.export"] {
            assert!(input_schema(tool).is_some(), "{tool} input schema");
            assert!(tool_specs().iter().any(|spec| spec["name"] == tool));
        }
        assert!(!tool_writes("pantry.list"));
        assert!(!tool_writes("label.export"));
        assert!(tool_writes("pantry.upsert"));
    }

    #[test]
    fn execution_event_rejects_unknown_pantry_item_and_warns_on_expired_one() {
        let dir = std::env::temp_dir().join(format!("pantry-event-{}", Uuid::new_v4()));
        std::fs::create_dir_all(&dir).unwrap();
        let write_input = |name: &str, value: Value| {
            let path = dir.join(name);
            std::fs::write(&path, value.to_string()).unwrap();
            path
        };
        let run_id = {
            let db = open_db(&dir).unwrap();
            let project = create_project(&db, "pantry").unwrap();
            let project_id = project["id"].as_str().unwrap().to_owned();
            let tx = db.unchecked_transaction().unwrap();
            let design = insert(
                &tx,
                "design",
                Some(&project_id),
                Some(&project_id),
                1,
                &json!({"title":"d"}),
            )
            .unwrap();
            let design_id = design["id"].as_str().unwrap().to_owned();
            let version = insert(
                &tx,
                "design_version",
                Some(&project_id),
                Some(&design_id),
                1,
                &json!({"design_id":design_id,"design_revision":1,"snapshot":{"title":"d"}}),
            )
            .unwrap();
            let run = insert(
                &tx,
                "run",
                Some(&project_id),
                Some(&design_id),
                1,
                &json!({
                    "title":"pantry run",
                    "scheduled_date":null,
                    "protocol_version_id":null,
                    "design_version_id":version["id"],
                    "design_revision":1,
                    "status":"in_progress",
                    "protocol_manifest":[],
                    "material_manifest":[]
                }),
            )
            .unwrap();
            tx.commit().unwrap();
            run["id"].as_str().unwrap().to_owned()
        };
        let expired = dispatch(
            "pantry.upsert",
            write_input(
                "expired.json",
                json!({"name":"Old serum","purchased_on":"2020-01-01","shelf_life_days":30}),
            )
            .to_str()
            .unwrap(),
            &dir,
        )
        .unwrap();
        let expired_id = expired["item"]["id"].as_str().unwrap().to_owned();

        let unknown = dispatch(
            "execution_event.create",
            write_input(
                "unknown.json",
                json!({"run_id":run_id,"event_type":"operation","content":"pipette","pantry_item_id":"missing-item"}),
            )
            .to_str()
            .unwrap(),
            &dir,
        )
        .unwrap_err();
        assert_eq!(unknown.code, "NOT_FOUND");

        let recorded = dispatch(
            "execution_event.create",
            write_input(
                "event.json",
                json!({"run_id":run_id,"event_type":"operation","content":"pipette","pantry_item_id":expired_id}),
            )
            .to_str()
            .unwrap(),
            &dir,
        )
        .unwrap();
        assert_eq!(recorded["record"]["data"]["pantry_item_id"], expired_id);
        assert_eq!(recorded["pantry_notices"][0]["code"], "pantry_item_expiry_notice");
        assert_eq!(recorded["pantry_notices"][0]["severity"], "advisory");
        assert_eq!(recorded["pantry_notices"][0]["status"], "expired");

        let plain = dispatch(
            "execution_event.create",
            write_input(
                "plain.json",
                json!({"run_id":run_id,"event_type":"operation","content":"pipette"}),
            )
            .to_str()
            .unwrap(),
            &dir,
        )
        .unwrap();
        assert_eq!(plain["record"]["data"]["pantry_item_id"], Value::Null);
        assert!(plain["pantry_notices"].as_array().unwrap().is_empty());

        std::fs::remove_dir_all(dir).unwrap();
    }

    #[test]
    fn pantry_delete_retires_an_item_and_keeps_its_label_code_out_of_reuse() {
        let dir = std::env::temp_dir().join(format!("pantry-delete-{}", Uuid::new_v4()));
        std::fs::create_dir_all(&dir).unwrap();
        let write_input = |name: &str, value: Value| {
            let path = dir.join(name);
            std::fs::write(&path, value.to_string()).unwrap();
            path
        };
        let created = dispatch(
            "pantry.upsert",
            write_input(
                "create.json",
                json!({"name":"Stale PBS","location":"bench shelf"}),
            )
            .to_str()
            .unwrap(),
            &dir,
        )
        .unwrap();
        let item_id = created["item"]["id"].as_str().unwrap().to_owned();
        let label_code = created["item"]["label_code"].as_str().unwrap().to_owned();

        let stale = dispatch(
            "pantry.delete",
            write_input(
                "stale.json",
                json!({"id":item_id,"expected_revision":9}),
            )
            .to_str()
            .unwrap(),
            &dir,
        )
        .unwrap_err();
        assert_eq!(stale.code, "REVISION_CONFLICT");

        let deleted = dispatch(
            "pantry.delete",
            write_input(
                "delete.json",
                json!({"id":item_id,"expected_revision":1}),
            )
            .to_str()
            .unwrap(),
            &dir,
        )
        .unwrap();
        assert_eq!(deleted["deleted"], true);
        assert_eq!(deleted["pantry_item_id"], item_id);
        assert_eq!(deleted["label_code"], label_code);
        assert_eq!(deleted["revision"], 2);

        let listed = dispatch(
            "pantry.list",
            write_input("list.json", json!({"as_of":"2026-11-15"}))
                .to_str()
                .unwrap(),
            &dir,
        )
        .unwrap();
        assert!(listed["items"].as_array().unwrap().is_empty());
        let exported = dispatch(
            "label.export",
            write_input("export.json", json!({"as_of":"2026-11-15"}))
                .to_str()
                .unwrap(),
            &dir,
        )
        .unwrap();
        assert!(exported["labels"].as_array().unwrap().is_empty());

        for (name, tool, payload) in [
            ("again.json", "pantry.delete", json!({"id":item_id})),
            (
                "revive.json",
                "pantry.upsert",
                json!({"id":item_id,"name":"Stale PBS"}),
            ),
        ] {
            let err = dispatch(tool, write_input(name, payload).to_str().unwrap(), &dir).unwrap_err();
            assert_eq!(err.code, "NOT_FOUND", "{tool} on a retired item");
        }

        let reserved: i64 = {
            let db = open_db(&dir).unwrap();
            db.query_row(
                "SELECT deleted FROM pantry_items WHERE label_code=?1",
                [&label_code],
                |row| row.get(0),
            )
            .unwrap()
        };
        assert_eq!(
            reserved, 1,
            "the retired row keeps its label code reserved"
        );
        assert!(tool_writes("pantry.delete"));
        assert!(input_schema("pantry.delete").is_some());

        std::fs::remove_dir_all(dir).unwrap();
    }

    #[test]
    fn pantry_table_from_an_earlier_build_gains_the_tombstone_column() {
        let dir = std::env::temp_dir().join(format!("pantry-migrate-{}", Uuid::new_v4()));
        std::fs::create_dir_all(&dir).unwrap();
        {
            let legacy = Connection::open(dir.join("labworkbench.sqlite")).unwrap();
            legacy
                .execute_batch(
                    "CREATE TABLE pantry_items(\
id TEXT PRIMARY KEY,\
label_code TEXT NOT NULL UNIQUE,\
project_id TEXT,\
name TEXT NOT NULL,\
location TEXT NOT NULL,\
purchased_on TEXT,\
opened_on TEXT,\
shelf_life_days INTEGER,\
opened_shelf_life_days INTEGER,\
exhausted INTEGER NOT NULL DEFAULT 0,\
revision INTEGER NOT NULL DEFAULT 1,\
created_at TEXT NOT NULL,\
updated_at TEXT NOT NULL);\
INSERT INTO pantry_items VALUES(\
'legacy-1','PN-LEGACY01',NULL,'Legacy PBS','bench',NULL,NULL,NULL,NULL,0,1,\
'2026-01-01T00:00:00Z','2026-01-01T00:00:00Z');",
                )
                .unwrap();
        }
        let write_input = |name: &str, value: Value| {
            let path = dir.join(name);
            std::fs::write(&path, value.to_string()).unwrap();
            path
        };
        let listed = dispatch(
            "pantry.list",
            write_input("list.json", json!({"as_of":"2026-11-15"}))
                .to_str()
                .unwrap(),
            &dir,
        )
        .unwrap();
        assert_eq!(listed["items"][0]["label_code"], "PN-LEGACY01");
        assert_eq!(listed["items"][0]["status"], "unopened");

        let deleted = dispatch(
            "pantry.delete",
            write_input("delete.json", json!({"id":"legacy-1"}))
                .to_str()
                .unwrap(),
            &dir,
        )
        .unwrap();
        assert_eq!(deleted["revision"], 2);
        std::fs::remove_dir_all(dir).unwrap();
    }

    fn calculator_input(dir: &std::path::Path, name: &str, value: Value) -> std::path::PathBuf {
        let path = dir.join(name);
        std::fs::write(&path, value.to_string()).unwrap();
        path
    }

    #[test]
    fn quality_concentration_calculators_accept_mass_units_and_balance_samples() {
        let dir = std::env::temp_dir().join(format!("asterlab-quality-calc-{}", Uuid::new_v4()));
        std::fs::create_dir_all(&dir).unwrap();

        // 2 mg/mL = 2000 ng/µL, so 100 µL at 50 ng/µL needs 2.5 µL of stock.
        let dilution = dispatch(
            "calculator.dilution",
            calculator_input(&dir, "dilution.json", json!({
                "stock_concentration": 2.0,
                "stock_unit": "mg/mL",
                "target_concentration": 50.0,
                "target_unit": "ng/µL",
                "final_volume": 100.0,
                "volume_unit": "µL"
            }))
            .to_str()
            .unwrap(),
            &dir,
        )
        .unwrap();
        assert!((dilution["stock_volume"].as_f64().unwrap() - 2.5).abs() < 1e-9);
        assert!((dilution["diluent_volume"].as_f64().unwrap() - 97.5).abs() < 1e-9);

        // Molar C1V1 keeps its old numbers.
        let molar = dispatch(
            "calculator.dilution",
            calculator_input(&dir, "molar.json", json!({
                "stock_concentration": 10.0,
                "stock_unit": "mM",
                "target_concentration": 1.0,
                "target_unit": "mM",
                "final_volume": 1.0,
                "volume_unit": "mL"
            }))
            .to_str()
            .unwrap(),
            &dir,
        )
        .unwrap();
        assert!((molar["stock_volume"].as_f64().unwrap() - 0.1).abs() < 1e-9);

        // Molar and mass concentration are different dimensions and never mix.
        let mixed = dispatch(
            "calculator.dilution",
            calculator_input(&dir, "mixed.json", json!({
                "stock_concentration": 2.0,
                "stock_unit": "mg/mL",
                "target_concentration": 50.0,
                "target_unit": "µM",
                "final_volume": 100.0,
                "volume_unit": "µL"
            }))
            .to_str()
            .unwrap(),
            &dir,
        )
        .unwrap_err();
        assert_eq!(mixed.code, "INVALID_INPUT");
        assert!(
            mixed.message.contains("同一量纲"),
            "message = {}",
            mixed.message
        );

        let batch = dispatch(
            "calculator.sample_normalize_batch",
            calculator_input(&dir, "normalize.json", json!({
                "target_mass_ng": 100.0,
                "reaction_volume_ul": 10.0,
                "samples": [
                    {"name": "S1", "concentration": 20.0, "unit": "ng/µL"},
                    {"name": "S2", "concentration": 5.0, "unit": "ng/µL"}
                ]
            }))
            .to_str()
            .unwrap(),
            &dir,
        )
        .unwrap();
        let rows = batch["rows"].as_array().unwrap();
        assert_eq!(rows[0]["concentration_ng_per_ul"], 20.0);
        assert_eq!(rows[0]["required_sample_volume_ul"], 5.0);
        assert_eq!(rows[0]["sample_volume_ul"], 5.0);
        assert_eq!(rows[0]["diluent_volume_ul"], 5.0);
        assert_eq!(rows[0]["achieved_mass_ng"], 100.0);
        assert_eq!(rows[0]["insufficient"], false);
        // 5 ng/µL needs 20 µL, more than the reaction allows: capped, reported.
        assert_eq!(rows[1]["required_sample_volume_ul"], 20.0);
        assert_eq!(rows[1]["sample_volume_ul"], 10.0);
        assert_eq!(rows[1]["diluent_volume_ul"], 0.0);
        assert_eq!(rows[1]["achieved_mass_ng"], 50.0);
        assert_eq!(rows[1]["insufficient"], true);
        assert_eq!(batch["insufficient_count"], 1);

        // Molar units are not mass concentrations for this tool.
        let molar_batch = dispatch(
            "calculator.sample_normalize_batch",
            calculator_input(&dir, "normalize-molar.json", json!({
                "target_mass_ng": 100.0,
                "reaction_volume_ul": 10.0,
                "samples": [{"name": "S1", "concentration": 1.0, "unit": "µM"}]
            }))
            .to_str()
            .unwrap(),
            &dir,
        )
        .unwrap_err();
        assert_eq!(molar_batch.code, "INVALID_INPUT");

        let dosing = dispatch(
            "calculator.in_vivo_dosing",
            calculator_input(&dir, "dosing.json", json!({
                "body_weights_g": [25.0, 30.0],
                "dose_mg_per_kg": 10.0,
                "dose_volume_ml_per_kg": 10.0,
                "overage_ratio": 0.1,
                "stock_concentration": 5.0,
                "stock_unit": "mg/mL"
            }))
            .to_str()
            .unwrap(),
            &dir,
        )
        .unwrap();
        let dose_rows = dosing["rows"].as_array().unwrap();
        assert_eq!(dose_rows[0]["index"], 1);
        assert_eq!(dose_rows[0]["body_weight_g"], 25.0);
        assert_eq!(dose_rows[0]["dose_mass_mg"], 0.25);
        assert_eq!(dose_rows[0]["injection_volume_ul"], 250.0);
        assert_eq!(dose_rows[1]["injection_volume_ul"], 300.0);
        assert_eq!(dosing["total_injection_volume_ul"], 550.0);
        assert_eq!(dosing["preparation_volume_ul"], 605.0);
        assert_eq!(dosing["prepared_dose_mass_mg"], 0.605);
        // 0.605 mg at 5 mg/mL is 121 µL of stock.
        assert_eq!(dosing["stock_volume_ul"], 121.0);
        assert_eq!(dosing["diluent_volume_ul"], 484.0);

        let too_dilute = dispatch(
            "calculator.in_vivo_dosing",
            calculator_input(&dir, "dosing-weak.json", json!({
                "body_weights_g": [25.0],
                "dose_mg_per_kg": 10.0,
                "dose_volume_ml_per_kg": 10.0,
                "stock_concentration": 0.001,
                "stock_unit": "ng/µL"
            }))
            .to_str()
            .unwrap(),
            &dir,
        )
        .unwrap_err();
        assert_eq!(too_dilute.code, "INVALID_INPUT");
        assert!(
            too_dilute.message.contains("stock concentration is too low"),
            "message = {}",
            too_dilute.message
        );

        for tool in [
            "calculator.sample_normalize_batch",
            "calculator.in_vivo_dosing",
            "calculator.biofilm_cfu",
        ] {
            assert!(input_schema(tool).is_some(), "{tool} has no input schema");
            assert!(result_schema(tool)["type"].as_str() == Some("object"));
            assert!(!tool_writes(tool), "{tool} must stay read-only");
            assert!(tool_specs().iter().any(|spec| spec["name"] == tool));
        }
        std::fs::remove_dir_all(dir).unwrap();
    }

    #[test]
    fn biofilm_cfu_groups_replicates_and_reports_exclusions() {
        let dir = std::env::temp_dir().join(format!("asterlab-biofilm-cfu-{}", Uuid::new_v4()));
        std::fs::create_dir_all(&dir).unwrap();
        let output = dispatch(
            "calculator.biofilm_cfu",
            calculator_input(&dir, "cfu.json", json!({
                "plates": [
                    {"raw_id": "ro-cfu-a1", "sample_id": "S1", "colonies": 51.0,
                     "dilution_fractions": [0.1, 0.1], "plated_volume": 100.0,
                     "plated_unit": "µL", "suspension_volume": 2.0, "suspension_unit": "mL"},
                    {"raw_id": "ro-cfu-a2", "sample_id": "S1", "colonies": 63.0,
                     "dilution_fractions": [0.01], "plated_volume": 100.0,
                     "plated_unit": "µL", "suspension_volume": 2.0, "suspension_unit": "mL"},
                    {"raw_id": "ro-cfu-a3", "sample_id": "S1", "colonies": 350.0,
                     "dilution_fractions": [0.01], "plated_volume": 100.0,
                     "plated_unit": "µL", "suspension_volume": 2.0, "suspension_unit": "mL"}
                ],
                "countable_min": 30.0,
                "countable_max": 300.0
            }))
            .to_str()
            .unwrap(),
            &dir,
        )
        .unwrap();
        let groups = output["groups"].as_array().unwrap();
        assert_eq!(groups.len(), 1, "same sample + dilution + plated volume averages");
        assert!((groups[0]["cfu_per_ml"].as_f64().unwrap() - 57000.0).abs() < 1e-6);
        assert!((groups[0]["cfu_per_sample"].as_f64().unwrap() - 114000.0).abs() < 1e-6);
        let readouts = groups[0]["readouts"].as_array().unwrap();
        assert_eq!(readouts.len(), 2);
        assert_eq!(readouts[0]["endpoint"], "cfu_per_ml");
        assert_eq!(readouts[0]["unit"], "CFU/mL");
        assert_eq!(readouts[0]["method"], "calculator.biofilm_cfu@1");
        assert_eq!(readouts[0]["derived_from"], json!(["ro-cfu-a1", "ro-cfu-a2"]));
        assert_eq!(readouts[1]["endpoint"], "cfu_per_sample");
        assert_eq!(readouts[1]["unit"], "CFU");
        assert_eq!(output["excluded_count"], 1);
        let excluded = output["excluded"].as_array().unwrap();
        assert_eq!(excluded[0]["raw_id"], "ro-cfu-a3");
        assert_eq!(excluded[0]["reason"], "above_window");
        // Boundary counts are countable; different dilutions never merge.
        let boundary = dispatch(
            "calculator.biofilm_cfu",
            calculator_input(&dir, "cfu-boundary.json", json!({
                "plates": [
                    {"raw_id": "ro-min", "sample_id": "S1", "colonies": 30.0,
                     "dilution_fractions": [0.01], "plated_volume": 100.0,
                     "plated_unit": "µL", "suspension_volume": 2.0, "suspension_unit": "mL"},
                    {"raw_id": "ro-max", "sample_id": "S1", "colonies": 300.0,
                     "dilution_fractions": [0.001], "plated_volume": 100.0,
                     "plated_unit": "µL", "suspension_volume": 2.0, "suspension_unit": "mL"}
                ],
                "countable_min": 30.0,
                "countable_max": 300.0
            }))
            .to_str()
            .unwrap(),
            &dir,
        )
        .unwrap();
        assert_eq!(boundary["groups"].as_array().unwrap().len(), 2);
        assert_eq!(boundary["excluded"].as_array().unwrap().len(), 0);
        assert!(input_schema("calculator.biofilm_cfu").is_some());
        assert!(result_schema("calculator.biofilm_cfu")["type"].as_str() == Some("object"));
        assert!(!tool_writes("calculator.biofilm_cfu"));
        assert!(tool_specs().iter().any(|spec| spec["name"] == "calculator.biofilm_cfu"));
        std::fs::remove_dir_all(dir).unwrap();
    }

    #[test]
    fn biofilm_cfu_rejects_invalid_inputs() {
        let dir = std::env::temp_dir().join(format!("asterlab-biofilm-cfu-bad-{}", Uuid::new_v4()));
        std::fs::create_dir_all(&dir).unwrap();
        let base = |raw_id: &str, colonies: f64, fractions: Vec<f64>, suspension: f64| {
            json!({
                "plates": [
                    {"raw_id": raw_id, "sample_id": "S1", "colonies": colonies,
                     "dilution_fractions": fractions, "plated_volume": 100.0,
                     "plated_unit": "µL", "suspension_volume": suspension, "suspension_unit": "mL"}
                ],
                "countable_min": 30.0,
                "countable_max": 300.0
            })
        };
        let dup = dispatch(
            "calculator.biofilm_cfu",
            calculator_input(&dir, "dup.json", json!({
                "plates": [
                    {"raw_id": "ro-same", "sample_id": "S1", "colonies": 51.0,
                     "dilution_fractions": [0.01], "plated_volume": 100.0,
                     "plated_unit": "µL", "suspension_volume": 2.0, "suspension_unit": "mL"},
                    {"raw_id": "ro-same", "sample_id": "S1", "colonies": 63.0,
                     "dilution_fractions": [0.01], "plated_volume": 100.0,
                     "plated_unit": "µL", "suspension_volume": 2.0, "suspension_unit": "mL"}
                ],
                "countable_min": 30.0,
                "countable_max": 300.0
            }))
            .to_str()
            .unwrap(),
            &dir,
        )
        .unwrap_err();
        assert_eq!(dup.code, "INVALID_INPUT");
        assert!(dup.message.contains("raw ID"), "message = {}", dup.message);
        let conflict = dispatch(
            "calculator.biofilm_cfu",
            calculator_input(&dir, "suspension.json", json!({
                "plates": [
                    {"raw_id": "ro-1", "sample_id": "S1", "colonies": 51.0,
                     "dilution_fractions": [0.01], "plated_volume": 100.0,
                     "plated_unit": "µL", "suspension_volume": 2.0, "suspension_unit": "mL"},
                    {"raw_id": "ro-2", "sample_id": "S1", "colonies": 55.0,
                     "dilution_fractions": [0.01], "plated_volume": 100.0,
                     "plated_unit": "µL", "suspension_volume": 3.0, "suspension_unit": "mL"}
                ],
                "countable_min": 30.0,
                "countable_max": 300.0
            }))
            .to_str()
            .unwrap(),
            &dir,
        )
        .unwrap_err();
        assert_eq!(conflict.code, "INVALID_INPUT");
        for (name, payload) in [
            ("bad-fraction", base("ro-x", 51.0, vec![1.5], 2.0)),
            ("empty-fraction", base("ro-x", 51.0, vec![], 2.0)),
        ] {
            let err = dispatch(
                "calculator.biofilm_cfu",
                calculator_input(&dir, &format!("{name}.json"), payload).to_str().unwrap(),
                &dir,
            )
            .unwrap_err();
            assert_eq!(err.code, "INVALID_INPUT", "{name}");
        }
        let unit_err = dispatch(
            "calculator.biofilm_cfu",
            calculator_input(&dir, "bad-unit.json", json!({
                "plates": [
                    {"raw_id": "ro-x", "sample_id": "S1", "colonies": 51.0,
                     "dilution_fractions": [0.01], "plated_volume": 100.0,
                     "plated_unit": "oz", "suspension_volume": 2.0, "suspension_unit": "mL"}
                ],
                "countable_min": 30.0,
                "countable_max": 300.0
            }))
            .to_str()
            .unwrap(),
            &dir,
        )
        .unwrap_err();
        assert_eq!(unit_err.code, "INVALID_INPUT");
        let empty = dispatch(
            "calculator.biofilm_cfu",
            calculator_input(&dir, "empty.json", base("ro-x", 5.0, vec![0.01], 2.0))
                .to_str()
                .unwrap(),
            &dir,
        )
        .unwrap();
        assert_eq!(empty["groups"].as_array().unwrap().len(), 0);
        assert_eq!(empty["excluded_count"], 1);
        assert_eq!(empty["excluded"][0]["reason"], "below_window");
        std::fs::remove_dir_all(dir).unwrap();
    }

    fn seed_lane_plate(db: &mut Connection) {
        seed_readout_runs(db);
        db.execute(
            "INSERT INTO records VALUES('lane-plate','plate','project-1','run-1',1,?1,'now','now')",
            [json!({"name":"lanes","format":"custom","rows":1,"columns":12}).to_string()],
        )
        .unwrap();
    }

    #[test]
    fn well_set_accepts_1d_lanes_and_detection_metadata() {
        let dir = std::env::temp_dir().join(format!("asterlab-lane-wells-{}", Uuid::new_v4()));
        std::fs::create_dir_all(&dir).unwrap();
        {
            let mut db = open_db(&dir).unwrap();
            seed_lane_plate(&mut db);
        }

        let first = dispatch(
            "well.set",
            calculator_input(&dir, "well1.json", json!({
                "plate_id": "lane-plate",
                "well_id": "Lane3",
                "group_label": "G1"
            }))
            .to_str()
            .unwrap(),
            &dir,
        )
        .unwrap();
        assert_eq!(first["record"]["data"]["well_id"], "Lane3");
        assert!(first["record"]["data"]["target"].is_null());
        assert!(first["record"]["data"]["well_role"].is_null());
        assert!(first["record"]["data"]["dilution_factor"].is_null());

        // `L3` is the same physical slot as `Lane3`, so it is an overwrite.
        let alias = dispatch(
            "well.set",
            calculator_input(&dir, "well-alias.json", json!({
                "plate_id": "lane-plate",
                "well_id": "L3"
            }))
            .to_str()
            .unwrap(),
            &dir,
        )
        .unwrap_err();
        assert_eq!(alias.code, "REVISION_CONFLICT");

        let updated = dispatch(
            "well.set",
            calculator_input(&dir, "well2.json", json!({
                "plate_id": "lane-plate",
                "well_id": "A3",
                "expected_revision": 1,
                "group_label": "G1",
                "target": "GAPDH",
                "well_role": "positive_control",
                "dilution_factor": 10
            }))
            .to_str()
            .unwrap(),
            &dir,
        )
        .unwrap();
        assert_eq!(updated["record"]["revision"], 2);
        assert_eq!(updated["record"]["data"]["well_id"], "A3");
        assert_eq!(updated["record"]["data"]["target"], "GAPDH");
        assert_eq!(updated["record"]["data"]["well_role"], "positive_control");
        assert_eq!(updated["record"]["data"]["dilution_factor"], 10.0);

        let out_of_range = dispatch(
            "well.set",
            calculator_input(&dir, "well3.json", json!({
                "plate_id": "lane-plate",
                "well_id": "Tube13"
            }))
            .to_str()
            .unwrap(),
            &dir,
        )
        .unwrap_err();
        assert_eq!(out_of_range.code, "INVALID_INPUT");
        assert!(
            out_of_range.message.contains("泳道超出板型范围"),
            "message = {}",
            out_of_range.message
        );

        let zero_factor = dispatch(
            "well.set",
            calculator_input(&dir, "well4.json", json!({
                "plate_id": "lane-plate",
                "well_id": "Lane4",
                "dilution_factor": 0
            }))
            .to_str()
            .unwrap(),
            &dir,
        )
        .unwrap_err();
        assert_eq!(zero_factor.code, "INVALID_INPUT");

        // A 2D plate has no 1D slots.
        let two_dimensional = dispatch(
            "well.set",
            calculator_input(&dir, "well5.json", json!({
                "plate_id": "plate-1",
                "well_id": "Lane3"
            }))
            .to_str()
            .unwrap(),
            &dir,
        )
        .unwrap_err();
        assert_eq!(two_dimensional.code, "INVALID_INPUT");

        let well_count: i64 = {
            let db = open_db(&dir).unwrap();
            db.query_row("SELECT COUNT(*) FROM records WHERE kind='well'", [], |row| {
                row.get(0)
            })
            .unwrap()
        };
        assert_eq!(well_count, 1);
        std::fs::remove_dir_all(dir).unwrap();
    }

    #[test]
    fn readout_batch_accepts_json_and_csv_qualifiers() {
        let dir = std::env::temp_dir().join(format!("asterlab-batch-qualifier-{}", Uuid::new_v4()));
        std::fs::create_dir_all(&dir).unwrap();
        {
            let mut db = open_db(&dir).unwrap();
            seed_readout_runs(&mut db);
        }

        let accepted = dispatch(
            "readout.batch",
            calculator_input(&dir, "batch-json.json", json!({
                "run_id": "run-1",
                "step_path": "run/read",
                "protocol_version_id": "pv1",
                "plate_id": "plate-1",
                "schema_id": "cck8.plate96.od450.v1",
                "assay_id": "assay-1",
                "timepoint": "D1",
                "cells": [
                    {"row": 0, "column": 0, "field": "od450", "value": 1.0, "unit": "AU"},
                    {"row": 0, "column": 1, "field": "od450", "qualifier": "not_detected"},
                    {"row": 0, "column": 2, "field": "od450", "value": null, "qualifier": "below_lod"}
                ]
            }))
            .to_str()
            .unwrap(),
            &dir,
        )
        .unwrap();
        assert_eq!(accepted["accepted"], 3);
        assert_eq!(accepted["replayed"], false);
        let cells = accepted["record"]["data"]["result_cells"]
            .as_array()
            .unwrap();
        assert_eq!(cells[0]["status"], "raw");
        assert!(cells[0].get("qualifier").is_none());
        assert_eq!(cells[1]["status"], "missing");
        assert!(cells[1]["value"].is_null());
        assert_eq!(cells[1]["qualifier"], "not_detected");
        assert_eq!(cells[2]["qualifier"], "below_lod");
        // A not-detected cell is not fabricated as a measurement.
        assert_eq!(
            accepted["record"]["data"]["measurements"]
                .as_array()
                .unwrap()
                .len(),
            1
        );

        let conflicting = dispatch(
            "readout.batch",
            calculator_input(&dir, "batch-conflict.json", json!({
                "run_id": "run-1",
                "step_path": "run/read",
                "protocol_version_id": "pv1",
                "plate_id": "plate-1",
                "schema_id": "cck8.plate96.od450.v1",
                "assay_id": "assay-1",
                "timepoint": "D1",
                "batch_id": "batch-conflict",
                "cells": [
                    {"row": 1, "column": 0, "field": "od450", "value": 1.0, "qualifier": "below_lod"}
                ]
            }))
            .to_str()
            .unwrap(),
            &dir,
        )
        .unwrap_err();
        assert_eq!(conflicting.code, "INVALID_INPUT");
        assert!(
            conflicting
                .issues
                .iter()
                .any(|issue| issue["code"] == "readout_value_or_qualifier"),
            "issues = {:?}",
            conflicting.issues
        );

        // An explicit null qualifier is the same as omitting it.
        let null_qualifier = dispatch(
            "readout.batch",
            calculator_input(&dir, "batch-null-qualifier.json", json!({
                "run_id": "run-1",
                "step_path": "run/read",
                "protocol_version_id": "pv1",
                "plate_id": "plate-1",
                "schema_id": "cck8.plate96.od450.v1",
                "assay_id": "assay-1",
                "timepoint": "D1",
                "batch_id": "batch-null",
                "cells": [
                    {"row": 2, "column": 0, "field": "od450", "value": 0.8, "unit": "AU", "qualifier": null}
                ]
            }))
            .to_str()
            .unwrap(),
            &dir,
        )
        .unwrap();
        assert_eq!(null_qualifier["accepted"], 1);

        let mixed_modes = dispatch(
            "readout.batch",
            calculator_input(&dir, "batch-mixed.json", json!({
                "run_id": "run-1",
                "step_path": "run/read",
                "protocol_version_id": "pv1",
                "plate_id": "plate-1",
                "schema_id": "cck8.plate96.od450.v1",
                "assay_id": "assay-1",
                "timepoint": "D1",
                "batch_id": "batch-mixed",
                "cells": [{"row": 3, "column": 0, "field": "od450", "qualifier": "invalid"}],
                "csv": "row,column,field,value\n4,0,od450,1.0\n"
            }))
            .to_str()
            .unwrap(),
            &dir,
        )
        .unwrap_err();
        assert_eq!(mixed_modes.code, "INVALID_INPUT");
        assert!(
            mixed_modes.message.contains("只能提供一种"),
            "message = {}",
            mixed_modes.message
        );

        let csv = dispatch(
            "readout.batch",
            calculator_input(&dir, "batch-csv.json", json!({
                "run_id": "run-1",
                "step_path": "run/read",
                "protocol_version_id": "pv1",
                "plate_id": "plate-1",
                "schema_id": "cck8.plate96.od450.v1",
                "assay_id": "assay-1",
                "timepoint": "D1",
                "batch_id": "batch-csv",
                "csv": "row,column,field,value,unit\n0,0,od450,Undetermined,AU\n1,0,od450,1.5,AU\n"
            }))
            .to_str()
            .unwrap(),
            &dir,
        )
        .unwrap();
        assert_eq!(csv["accepted"], 2);
        let csv_cells = csv["record"]["data"]["result_cells"].as_array().unwrap();
        assert_eq!(csv_cells[0]["qualifier"], "not_detected");
        assert_eq!(csv_cells[0]["status"], "missing");
        assert_eq!(csv_cells[1]["value"], 1.5);
        std::fs::remove_dir_all(dir).unwrap();
    }

    #[test]
    fn readout_submit_records_categorical_and_table_readouts_once() {
        let dir = std::env::temp_dir().join(format!("asterlab-typed-readout-{}", Uuid::new_v4()));
        std::fs::create_dir_all(&dir).unwrap();
        {
            let mut db = open_db(&dir).unwrap();
            seed_readout_runs(&mut db);
        }

        let request = json!({
            "run_id": "run-1",
            "step_path": "run/read",
            "protocol_version_id": "pv1",
            "assay_id": "assay-1",
            "readout_schemas": [
                {
                    "schema_id": "wb_grade",
                    "schema_version": 1,
                    "endpoint": "wb_grade",
                    "value_kind": "categorical",
                    "unit": {"canonical": null, "required": false},
                    "missing": {"allowed": [], "null_allowed": false},
                    "source": {"raw": true, "derived": false},
                    "link": {"timepoint_required": false, "sample_required": false},
                    "categorical": {"options": ["positive", "negative"], "allow_other": false}
                },
                {
                    "schema_id": "wb_table",
                    "schema_version": 1,
                    "endpoint": "wb_table",
                    "value_kind": "table",
                    "unit": {"canonical": null, "required": false},
                    "missing": {"allowed": [], "null_allowed": true},
                    "source": {"raw": true, "derived": false},
                    "link": {"timepoint_required": false, "sample_required": false},
                    "table": {"columns": [
                        {"key": "lane", "label": "Lane", "value_kind": "numeric"},
                        {"key": "grade", "label": "Grade", "value_kind": "categorical"}
                    ]}
                }
            ],
            "readouts": [
                {
                    "schema_id": "wb_grade",
                    "schema_version": 1,
                    "value": {"kind": "categorical", "category": "positive"},
                    "source": {"kind": "raw", "derived_from": []}
                },
                {
                    "schema_id": "wb_table",
                    "schema_version": 1,
                    "value": {
                        "kind": "table",
                        "columns": ["lane", "grade"],
                        "rows": [[1, "+"], [2, "+++"]]
                    },
                    "source": {"kind": "raw", "derived_from": []}
                }
            ]
        });
        let path = calculator_input(&dir, "typed-readout.json", request.clone());
        let accepted = dispatch("readout.submit", path.to_str().unwrap(), &dir).unwrap();
        assert_eq!(accepted["accepted"], 2);
        assert_eq!(accepted["replayed"], false);
        let records: Vec<Value> = accepted["record"]["data"]["result_cells"]
            .as_array()
            .unwrap()
            .iter()
            .filter(|cell| cell["kind"] == "record")
            .cloned()
            .collect();
        assert_eq!(records.len(), 2, "records = {records:?}");
        assert_eq!(records[0]["readout"]["value"]["kind"], "categorical");
        assert_eq!(records[0]["readout"]["value"]["category"], "positive");
        assert_eq!(records[1]["readout"]["value"]["kind"], "table");
        assert_eq!(
            records[1]["readout"]["value"]["rows"],
            json!([[1.0, "+"], [2.0, "+++"]])
        );
        // Non-scalar readouts stay out of the frozen `readouts[]` projection.
        assert!(
            accepted["record"]["data"]["readouts"]
                .as_array()
                .unwrap()
                .is_empty()
        );

        let replayed = dispatch("readout.submit", path.to_str().unwrap(), &dir).unwrap();
        assert_eq!(replayed["replayed"], true);
        assert_eq!(replayed["record"]["id"], accepted["record"]["id"]);

        let unknown_step = request.clone();
        let mut unknown_step = unknown_step;
        unknown_step["step_path"] = json!("run/does-not-exist");
        let error = dispatch(
            "readout.submit",
            calculator_input(&dir, "typed-unknown-step.json", unknown_step)
                .to_str()
                .unwrap(),
            &dir,
        )
        .unwrap_err();
        assert_eq!(error.code, "INVALID_INPUT");

        let mut invalid_category = request;
        invalid_category["readouts"][0]["value"]["category"] = json!("variable");
        let rejected = dispatch(
            "readout.submit",
            calculator_input(&dir, "typed-invalid-category.json", invalid_category)
                .to_str()
                .unwrap(),
            &dir,
        )
        .unwrap_err();
        assert_eq!(rejected.code, "INVALID_INPUT");
        assert!(
            rejected
                .issues
                .iter()
                .any(|issue| issue["code"] == "category_not_allowed"),
            "issues = {:?}",
            rejected.issues
        );
        std::fs::remove_dir_all(dir).unwrap();
    }

    #[test]
    fn run_create_records_a_same_project_cross_design_source_run() {
        let mut db = Connection::open_in_memory().unwrap();
        db.execute_batch(
            "CREATE TABLE records(id TEXT PRIMARY KEY,kind TEXT NOT NULL,project_id TEXT,parent_id TEXT,revision INTEGER NOT NULL,data_json TEXT NOT NULL,created_at TEXT NOT NULL,updated_at TEXT NOT NULL);
             INSERT INTO records VALUES('project-1','project',NULL,NULL,1,'{}','now','now');
             INSERT INTO records VALUES('project-2','project',NULL,NULL,1,'{}','now','now');
             INSERT INTO records VALUES('protocol-1','protocol','project-1',NULL,1,'{\"title\":\"CCK-8\"}','now','now');
             INSERT INTO records VALUES('protocol-v1','protocol_version','project-1','protocol-1',1,'{\"protocol_id\":\"protocol-1\",\"version\":1,\"body\":\"\",\"root_block_version_id\":\"block-v1\",\"root_bindings\":[]}','now','now');
             INSERT INTO records VALUES('block-1','protocol_block','project-1',NULL,1,'{\"title\":\"CCK-8 block\",\"summary\":\"\"}','now','now');",
        ).unwrap();
        let source = r#"```asterlab
kind: experiment
id: exp-1
schema: 1
title: CCK-8
```
```asterlab
kind: group
id: g-1
schema: 1
name: Control
role: control
```
```asterlab
kind: assay
id: assay-cck8
schema: 1
title: CCK-8
endpoint: OD450
unit: AU
group_refs: [g-1]
protocol_version_id: protocol-v1
```
"#;
        let mut design = compile_asterdoc_to_design(source).unwrap();
        let design_object = design.as_object_mut().unwrap();
        design_object.remove("mygo");
        design_object.remove("asterlab");
        let design_json = serde_json::to_string(&design).unwrap();
        db.execute(
            "INSERT INTO records VALUES('design-1','design','project-1',NULL,1,?1,'now','now')",
            rusqlite::params![design_json],
        )
        .unwrap();
        db.execute(
            "INSERT INTO records VALUES('design-1-v1','design_version','project-1','design-1',1,'{}','now','now')",
            [],
        )
        .unwrap();
        let block_v1 = json!({
            "block_id":"block-1", "version":1, "summary":"",
            "parameter_slots":[],
            "steps":[{"kind":"text","use_id":"seed","title":"Seed","instruction":"Seed cells"}]
        });
        db.execute(
            "INSERT INTO records VALUES('block-v1','protocol_block_version','project-1','block-1',1,?1,'now','now')",
            rusqlite::params![serde_json::to_string(&block_v1).unwrap()],
        )
        .unwrap();
        // The upstream harvest Run lives in another Design of the same project.
        db.execute(
            "INSERT INTO records VALUES('run-harvest','run','project-1','design-harvest',1,'{\"title\":\"harvest\"}','now','now')",
            [],
        )
        .unwrap();
        db.execute(
            "INSERT INTO records VALUES('run-foreign','run','project-2','design-foreign',1,'{\"title\":\"foreign\"}','now','now')",
            [],
        )
        .unwrap();

        let tx = db
            .transaction_with_behavior(rusqlite::TransactionBehavior::Immediate)
            .unwrap();
        let run = create_run_with_source_in_transaction(
            &tx,
            "design-1",
            "assay",
            None,
            None,
            None,
            None,
            "independent",
            Some("run-harvest"),
            None,
        )
        .unwrap();
        tx.commit().unwrap();
        assert_eq!(run["data"]["source_run_id"], "run-harvest");
        assert!(run["data"]["repeat_of_run_id"].is_null());

        let tx = db
            .transaction_with_behavior(rusqlite::TransactionBehavior::Immediate)
            .unwrap();
        let foreign = create_run_with_source_in_transaction(
            &tx,
            "design-1",
            "assay",
            None,
            None,
            None,
            None,
            "independent",
            Some("run-foreign"),
            None,
        )
        .unwrap_err();
        tx.rollback().ok();
        assert_eq!(foreign.code, "INVALID_INPUT");
        assert!(
            foreign.message.contains("同一项目"),
            "message = {}",
            foreign.message
        );

        let tx = db
            .transaction_with_behavior(rusqlite::TransactionBehavior::Immediate)
            .unwrap();
        let missing = create_run_with_source_in_transaction(
            &tx,
            "design-1",
            "assay",
            None,
            None,
            None,
            None,
            "independent",
            Some("run-missing"),
            None,
        )
        .unwrap_err();
        tx.rollback().ok();
        assert_eq!(missing.code, "NOT_FOUND");

        let input: RunCreate = serde_json::from_value(json!({
            "design_id": "design-1",
            "title": "assay",
            "source_run_id": "run-harvest"
        }))
        .unwrap();
        assert_eq!(input.source_run_id.as_deref(), Some("run-harvest"));
        assert!(
            input_schema("run.create").unwrap()["properties"]["source_run_id"].is_object()
        );
        assert!(record_data_schema("run")["properties"]["source_run_id"].is_object());
    }

    #[test]
    fn asterdoc_validate_reports_invalid_when_the_projection_fails() {
        let dir = std::env::temp_dir().join(format!("asterlab-validate-{}", Uuid::new_v4()));
        std::fs::create_dir_all(&dir).unwrap();
        {
            let mut db = open_db(&dir).unwrap();
            create_project(&db, "validate").unwrap();
        }
        // Source-only validation passes (it is prose), but the design projection
        // cannot produce a design without exactly one experiment module.
        let input = calculator_input(
            &dir,
            "validate.json",
            json!({"source": "只有说明文字，没有任何 asterlab 模块。\n"}),
        );
        let result = dispatch("asterdoc.validate", input.to_str().unwrap(), &dir).unwrap();
        assert_eq!(result["valid"], false);
        assert_eq!(result["compiled"], false);
        assert!(result["error"]["message"].as_str().is_some_and(|message| !message.is_empty()));
        std::fs::remove_dir_all(dir).unwrap();
    }
}
