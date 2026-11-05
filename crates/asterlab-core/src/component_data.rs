//! C3-3A read-only component data projections.
//!
//! WellPlate -> Calculator -> Table projections over a ContainerPlan. These are
//! derived data; they never write back into DesignVersion or Run.

use crate::labware::{ContainerPlan, LabwareRegistry, PositionAssignment};
use crate::semantic_tool::{
    run_pure, PureTool, ToolActionSpec, ToolError, ToolInvocation, ToolSideEffects, ToolTrust,
};
use serde::{Deserialize, Serialize};
use serde_json::{json, Value};

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[serde(deny_unknown_fields)]
pub struct WellPlateProjection {
    pub container_id: String,
    pub profile_id: String,
    pub profile_version: u32,
    pub positions: Vec<PositionAssignment>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[serde(deny_unknown_fields)]
pub struct ReagentCalculation {
    pub state: String,
    pub assigned_positions: usize,
    pub volume_per_position: Option<f64>,
    pub extra_ratio: Option<f64>,
    pub total_volume: Option<f64>,
    pub unresolved: Vec<String>,
    pub provenance: Vec<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub tool_action: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[serde(deny_unknown_fields)]
pub struct TableProjection {
    pub columns: Vec<String>,
    pub rows: Vec<Vec<Value>>,
    pub unresolved: Vec<String>,
    pub provenance: Vec<String>,
}

pub fn project_well_plate(
    plan: &ContainerPlan,
    registry: &LabwareRegistry,
    container_id: &str,
) -> Result<WellPlateProjection, Vec<String>> {
    let container = plan
        .containers
        .iter()
        .find(|container| container.id == container_id)
        .ok_or_else(|| vec![format!("container {container_id} does not exist")])?;
    if registry.get(&container.profile_ref).is_none() {
        return Err(vec![format!(
            "container {container_id} references unknown profile"
        )]);
    }
    Ok(WellPlateProjection {
        container_id: container.id.clone(),
        profile_id: container.profile_ref.id.clone(),
        profile_version: container.profile_ref.version,
        positions: plan
            .assignments
            .iter()
            .filter(|assignment| assignment.container_id == container_id)
            .cloned()
            .collect(),
    })
}

pub fn calculate_reagent(
    plate: &WellPlateProjection,
    volume_per_position: Option<f64>,
    extra_ratio: Option<f64>,
) -> ReagentCalculation {
    let mut unresolved = Vec::new();
    let assigned_positions = plate.positions.len();
    if assigned_positions == 0 {
        unresolved.push("no_assigned_positions".into());
    }
    let Some(volume) = volume_per_position else {
        unresolved.push("volume_per_position".into());
        return ReagentCalculation {
            state: "unresolved".into(),
            assigned_positions,
            volume_per_position: None,
            extra_ratio,
            total_volume: None,
            unresolved,
            provenance: vec![format!("container:{}", plate.container_id)],
            tool_action: None,
        };
    };
    if !volume.is_finite() || volume < 0.0 {
        unresolved.push("invalid_volume_per_position".into());
        return ReagentCalculation {
            state: "unresolved".into(),
            assigned_positions,
            volume_per_position: Some(volume),
            extra_ratio,
            total_volume: None,
            unresolved,
            provenance: vec![format!("container:{}", plate.container_id)],
            tool_action: None,
        };
    }
    let Some(ratio) = extra_ratio else {
        unresolved.push("extra_ratio".into());
        return ReagentCalculation {
            state: "unresolved".into(),
            assigned_positions,
            volume_per_position: Some(volume),
            extra_ratio: None,
            total_volume: None,
            unresolved,
            provenance: vec![format!("container:{}", plate.container_id)],
            tool_action: None,
        };
    };
    if !ratio.is_finite() || ratio < 0.0 {
        unresolved.push("invalid_extra_ratio".into());
        return ReagentCalculation {
            state: "unresolved".into(),
            assigned_positions,
            volume_per_position: Some(volume),
            extra_ratio,
            total_volume: None,
            unresolved,
            provenance: vec![format!("container:{}", plate.container_id)],
            tool_action: None,
        };
    }
    let total = assigned_positions as f64 * volume * (1.0 + ratio);
    ReagentCalculation {
        state: "resolved".into(),
        assigned_positions,
        volume_per_position: Some(volume),
        extra_ratio,
        total_volume: Some(total),
        unresolved,
        provenance: vec![
            format!("container:{}", plate.container_id),
            "formula:positions*volume*(1+extra_ratio)".into(),
        ],
        tool_action: Some("tool.reagent_total@1".into()),
    }
}

pub const REAGENT_TOOL_ID: &str = "tool.reagent_total";
pub const REAGENT_TOOL_VERSION: &str = "1";

pub fn reagent_tool_spec() -> ToolActionSpec {
    ToolActionSpec {
        action_id: REAGENT_TOOL_ID.into(),
        version: REAGENT_TOOL_VERSION.into(),
        input_schema: json!({
            "type":"object",
            "additionalProperties":false,
            "required":["assigned_positions","volume_per_position","extra_ratio"],
            "properties":{
                "assigned_positions":{"type":"integer","minimum":1},
                "volume_per_position":{"type":"number","minimum":0},
                "extra_ratio":{"type":"number","minimum":0}
            }
        }),
        output_schema: json!({
            "type":"object",
            "required":["total_volume"],
            "properties":{"total_volume":{"type":"number"}}
        }),
        side_effects: ToolSideEffects::Pure,
        implementation_source: "core:component_data:reagent_total".into(),
        trust: ToolTrust::Trusted,
        requires_confirmation: false,
    }
}

struct ReagentTool {
    spec: ToolActionSpec,
}

impl PureTool for ReagentTool {
    fn spec(&self) -> &ToolActionSpec {
        &self.spec
    }

    fn execute(&self, inputs: &Value) -> Result<Value, ToolError> {
        let positions = inputs["assigned_positions"]
            .as_u64()
            .ok_or_else(|| ToolError::new("positions", "assigned_positions is required"))?;
        let volume = inputs["volume_per_position"]
            .as_f64()
            .ok_or_else(|| ToolError::new("volume", "volume_per_position is required"))?;
        let ratio = inputs["extra_ratio"]
            .as_f64()
            .ok_or_else(|| ToolError::new("ratio", "extra_ratio is required"))?;
        Ok(json!({"total_volume": positions as f64 * volume * (1.0 + ratio)}))
    }
}

pub fn run_reagent_tool(
    plate: &WellPlateProjection,
    volume_per_position: f64,
    extra_ratio: f64,
) -> Result<ToolInvocation, Vec<ToolError>> {
    let inputs = json!({
        "assigned_positions": plate.positions.len() as u64,
        "volume_per_position": volume_per_position,
        "extra_ratio": extra_ratio
    });
    run_pure(
        &ReagentTool {
            spec: reagent_tool_spec(),
        },
        "reagent-total",
        inputs,
    )
}

pub fn reagent_table(calc: &ReagentCalculation) -> TableProjection {
    let row = vec![
        json!(calc.assigned_positions),
        calc.volume_per_position
            .map_or(Value::Null, |value| json!(value)),
        calc.extra_ratio.map_or(Value::Null, |value| json!(value)),
        calc.total_volume.map_or(Value::Null, |value| json!(value)),
    ];
    TableProjection {
        columns: vec![
            "assigned_positions".into(),
            "volume_per_position".into(),
            "extra_ratio".into(),
            "total_volume".into(),
        ],
        rows: vec![row],
        unresolved: calc.unresolved.clone(),
        provenance: calc.provenance.clone(),
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::labware::{default_labware_registry, AssignedTarget, PlanTargetRef};

    fn plan() -> ContainerPlan {
        ContainerPlan {
            containers: vec![crate::labware::PlannedContainer {
                id: "plate-1".into(),
                profile_ref: crate::labware::ProfileRef {
                    id: "labware.cell_culture_96".into(),
                    version: 1,
                },
                label: "Plate 1".into(),
                source: None,
            }],
            assignments: vec![PositionAssignment {
                container_id: "plate-1".into(),
                position_id: "A1".into(),
                targets: vec![AssignedTarget {
                    target: PlanTargetRef::Sample { id: "s1".into() },
                    role: "primary".into(),
                    relationship: "contains".into(),
                    counts_as_independent_unit: true,
                    source: None,
                }],
            }],
            placements: vec![],
        }
    }

    #[test]
    fn missing_volume_is_unresolved_not_zero() {
        let registry = default_labware_registry();
        let plate = project_well_plate(&plan(), &registry, "plate-1").unwrap();
        let calc = calculate_reagent(&plate, None, None);
        assert_eq!(calc.state, "unresolved");
        assert!(calc.total_volume.is_none());
        assert!(calc.unresolved.contains(&"volume_per_position".into()));
    }

    #[test]
    fn registered_reagent_tool_returns_versioned_provenance() {
        let registry = default_labware_registry();
        let plate = project_well_plate(&plan(), &registry, "plate-1").unwrap();
        let invocation = run_reagent_tool(&plate, 10.0, 0.1).unwrap();
        assert_eq!(invocation.action_id, REAGENT_TOOL_ID);
        assert_eq!(invocation.action_version, REAGENT_TOOL_VERSION);
        assert_eq!(invocation.outputs["total_volume"], 11.0);
        assert_eq!(invocation.provenance[0].kind, "tool_action");
    }

    #[test]
    fn explicit_volume_and_extra_ratio_calculate() {
        let registry = default_labware_registry();
        let plate = project_well_plate(&plan(), &registry, "plate-1").unwrap();
        let calc = calculate_reagent(&plate, Some(10.0), Some(0.1));
        assert_eq!(calc.total_volume, Some(11.0));
        let table = reagent_table(&calc);
        assert_eq!(table.columns.len(), 4);
        assert_eq!(table.rows[0][3], 11.0);
    }
}
