use super::*;

pub(crate) fn dispatch(tool: &str, path: &str, data_dir: &Path) -> Result<Value, CliError> {
    if tool == "workflow.inspect_tree" {
        let input: InspectTreeInput = parse_input(path)?;
        return inspect_workflow_tree(&input, data_dir);
    }
    if tool == "import.study.preview" {
        let i: StudyImport = parse_input(path)?;
        validate_import(&i)?;
        let mut designs = i.designs.clone();
        for design in &mut designs {
            normalize_design(design, None);
        }
        let warnings = designs
            .iter()
            .enumerate()
            .flat_map(|(ix, d)| {
                let mut w = Vec::new();
                if d.experimental_unit.trim().is_empty() {
                    w.push(format!("designs[{ix}].experimental_unit 未填写"))
                }
                if d.group_specs.is_empty() {
                    w.push(format!("designs[{ix}].group_specs 为空"))
                }
                if d.primary_endpoint.trim().is_empty() {
                    w.push(format!("designs[{ix}].primary_endpoint 未填写"))
                }
                w
            })
            .collect::<Vec<_>>();
        return Ok(
            json!({"valid":true,"writes":false,"project_count":1,"design_count":i.designs.len(),"protocol_count":i.protocols.len(),"warnings":warnings}),
        );
    }
    if tool == "calculator.dilution" {
        let i: DilutionCalc = parse_input(path)?;
        let stock = concentration_scale(&i.stock_unit)?;
        let target = concentration_scale(&i.target_unit)?;
        if stock.is_mass() != target.is_mass() {
            return Err(CliError::input(
                "储液浓度与目标浓度必须同一量纲：摩尔（M、mM、µM、nM）或质量浓度（mg/mL、µg/mL、ng/µL）不能混用",
            ));
        }
        let c1 = stock.factor();
        let c2 = target.factor();
        let vf = volume_factor(&i.volume_unit)?;
        if !(i.stock_concentration.is_finite()
            && i.stock_concentration > 0.0
            && i.target_concentration.is_finite()
            && i.target_concentration >= 0.0
            && i.final_volume.is_finite()
            && i.final_volume > 0.0)
        {
            return Err(CliError::input(
                "储液浓度和终体积必须为正数，目标浓度不得小于 0",
            ));
        }
        if i.target_concentration * c2 > i.stock_concentration * c1 {
            return Err(CliError::input("目标浓度高于储液浓度，不能通过稀释配制"));
        }
        let v1_l =
            (i.target_concentration * c2) * (i.final_volume * vf) / (i.stock_concentration * c1);
        return Ok(
            json!({"stock_volume":v1_l/vf,"volume_unit":i.volume_unit,"diluent_volume":i.final_volume-v1_l/vf,"formula":"C1 × V1 = C2 × V2","assumption":"浓度与体积单位已按输入换算；摩尔与质量浓度是不同量纲，不互相换算；未计死体积、回收率或纯度修正"}),
        );
    }
    if tool == "calculator.solution" {
        let i: SolutionCalc = parse_input(path)?;
        let vf = volume_factor(&i.volume_unit)?;
        if !(i.concentration_mol_l.is_finite()
            && i.concentration_mol_l > 0.0
            && i.final_volume.is_finite()
            && i.final_volume > 0.0
            && i.molecular_weight_g_mol.is_finite()
            && i.molecular_weight_g_mol > 0.0)
        {
            return Err(CliError::input("浓度、体积和分子量都必须是正数"));
        }
        let grams = i.concentration_mol_l * (i.final_volume * vf) * i.molecular_weight_g_mol;
        return Ok(
            json!({"mass_g":grams,"mass_mg":grams*1000.0,"formula":"m = C × V × MW","assumption":"体积按所选单位换算为 L；未修正纯度、水合物或盐型"}),
        );
    }
    if tool == "calculator.serial_dilution" {
        let i: SerialDilutionCalc = parse_input(path)?;
        concentration_factor(&i.concentration_unit)?;
        if !(i.initial_concentration.is_finite()
            && i.initial_concentration > 0.0
            && i.transfer_volume.is_finite()
            && i.transfer_volume > 0.0
            && i.diluent_volume.is_finite()
            && i.diluent_volume >= 0.0
            && i.steps > 0
            && i.steps <= 30)
        {
            return Err(CliError::input(
                "初始浓度和转移体积需大于 0；稀释液体积不得小于 0；级数为 1–30",
            ));
        }
        let ratio = i.transfer_volume / (i.transfer_volume + i.diluent_volume);
        let levels=(1..=i.steps).map(|step|json!({"step":step,"concentration":i.initial_concentration*ratio.powi(step as i32),"unit":i.concentration_unit})).collect::<Vec<_>>();
        return Ok(
            json!({"per_step_dilution_factor":1.0/ratio,"levels":levels,"formula":"C_next = C_previous × transfer_volume / (transfer_volume + diluent_volume)","assumption":"每步充分混匀；转移体积与稀释液体积单位相同；未计死体积或回收率"}),
        );
    }
    if tool == "calculator.reaction_volume" {
        let i: ReactionVolumeCalc = parse_input(path)?;
        if !(i.target_volume_ul.is_finite() && i.target_volume_ul > 0.0) {
            return Err(CliError::input("目标总体积必须大于 0 µL"));
        }
        if i.components.len() > 100 {
            return Err(CliError::input("组分最多 100 项"));
        }
        for c in &i.components {
            check_text(&c.name, "组分名称", 120)?;
            if !c.volume_ul.is_finite() || c.volume_ul < 0.0 {
                return Err(CliError::input("组分体积必须是大于等于 0 的有限数值"));
            }
        }
        let used: f64 = i.components.iter().map(|c| c.volume_ul).sum();
        if used > i.target_volume_ul {
            return Err(CliError::input("组分总体积超过目标总体积"));
        }
        return Ok(
            json!({"allocated_volume_ul":used,"remaining_volume_ul":i.target_volume_ul-used,"unit":"µL","assumption":"按输入组分体积求和；未判断配方科学适用性"}),
        );
    }
    if tool == "calculator.gel_mastermix" {
        let i: GelMastermixCalc = parse_input(path)?;
        if i.groups.len() > 100 {
            return Err(CliError::input("组别最多 100 项"));
        }
        for group in &i.groups {
            check_text(&group.name, "组名", 120)?;
        }
        let output = gel_mastermix_calc(&GelMastermixInput {
            volume_per_well_ul: i.volume_per_well_ul,
            excess_ratio: i.excess_ratio,
            lap_stock_percent_wv: i.lap_stock_percent_wv,
            lap_final_percent_wv: i.lap_final_percent_wv,
            groups: i
                .groups
                .into_iter()
                .map(|group| GelMastermixGroup {
                    name: group.name,
                    gelma_percent_wv: group.gelma_percent_wv,
                    wells: group.wells,
                })
                .collect(),
        })
        .map_err(CliError::input)?;
        let groups: Vec<Value> = output
            .groups
            .iter()
            .map(|group| {
                json!({
                    "name": group.name,
                    "wells": group.wells,
                    "gelma_percent_wv": group.gelma_percent_wv,
                    "volume_ul": display_quantity(group.volume_ul),
                    "gelma_mass_mg": display_quantity(group.gelma_mass_mg),
                    "lap_stock_volume_ul": display_quantity(group.lap_stock_volume_ul),
                    "solvent_volume_ul": display_quantity(group.solvent_volume_ul),
                })
            })
            .collect();
        return Ok(json!({
            "groups": groups,
            "total_volume_ul": display_quantity(output.volume_ul),
            "total_gelma_mass_mg": display_quantity(output.gelma_mass_mg),
            "total_lap_stock_volume_ul": display_quantity(output.lap_stock_volume_ul),
            "total_solvent_volume_ul": display_quantity(output.solvent_volume_ul),
            "formula": "volume_ul = wells × volume_per_well_ul × (1 + excess_ratio)；mass_mg = volume_ul × % w/v ÷ 100；LAP_stock_ul = volume_ul × LAP_final% ÷ LAP_stock%；solvent_ul = volume_ul − LAP_stock_ul",
            "assumption": "组孔数取实际复孔数；GelMA 干粉体积忽略不计；LAP 母液计入溶剂体积；未计过滤损失或纯度修正"
        }));
    }
    if tool == "calculator.cell_seeding" {
        let i: CellSeedingCalc = parse_input(path)?;
        if i.groups.len() > 100 {
            return Err(CliError::input("组别最多 100 项"));
        }
        for group in &i.groups {
            check_text(&group.name, "组名", 120)?;
        }
        let output = cell_seeding_calc(&CellSeedingInput {
            density_cells_per_well: i.density_cells_per_well,
            volume_per_well_ul: i.volume_per_well_ul,
            excess_ratio: i.excess_ratio,
            factor_stock_mg_per_ml: i.factor_stock_mg_per_ml,
            factor_final_mg_per_ml: i.factor_final_mg_per_ml,
            groups: i
                .groups
                .into_iter()
                .map(|group| CellSeedingGroup {
                    name: group.name,
                    wells: group.wells,
                })
                .collect(),
        })
        .map_err(CliError::input)?;
        let groups: Vec<Value> = output
            .groups
            .iter()
            .map(|group| {
                json!({
                    "name": group.name,
                    "wells": group.wells,
                    "total_cells": group.total_cells,
                    "suspension_volume_ul": display_quantity(group.suspension_volume_ul),
                    "factor_stock_volume_ul": display_quantity(group.factor_stock_volume_ul),
                })
            })
            .collect();
        return Ok(json!({
            "groups": groups,
            "total_cells": output.total_cells,
            "total_suspension_volume_ul": display_quantity(output.suspension_volume_ul),
            "total_factor_stock_volume_ul": display_quantity(output.factor_stock_volume_ul),
            "formula": "cells = ceil(density_cells_per_well × wells × (1 + excess_ratio))；suspension_ul = wells × volume_per_well_ul × (1 + excess_ratio)；factor_stock_ul = suspension_ul × final_mg_per_ml ÷ stock_mg_per_ml",
            "assumption": "按实际复孔数计数并向上取整细胞数；因子母液体积包含在悬液体积内，不再另行扣减；未计接种回收率或活率修正"
        }));
    }
    if tool == "calculator.runtime_normalize" {
        let i: RuntimeNormalizeCalc = parse_input(path)?;
        let output = runtime_normalize_calc(&RuntimeNormalizeInput {
            planned_wells: i.planned_wells,
            actual_wells: i.actual_wells,
            volume_per_well_ul: i.volume_per_well_ul,
            excess_ratio: i.excess_ratio,
            dead_volume_ul: i.dead_volume_ul,
            prepared_volume_ul: i.prepared_volume_ul,
        })
        .map_err(CliError::input)?;
        return Ok(json!({
            "planned_volume_ul": display_quantity(output.planned_volume_ul),
            "required_volume_ul": display_quantity(output.required_volume_ul),
            "dead_volume_ul": display_quantity(output.dead_volume_ul),
            "additional_volume_ul": display_quantity(output.additional_volume_ul),
            "unused_volume_ul": display_quantity(output.unused_volume_ul),
            "formula": "required_ul = actual_wells × volume_per_well_ul × (1 + excess_ratio) + dead_volume_ul",
            "assumption": "死体积按每个配制管只加一次；余量只作用于工作体积；不改写冻结计划，只返回运行期配平建议"
        }));
    }
    if tool == "calculator.sample_normalize_batch" {
        let i: SampleNormalizeBatchCalc = parse_input(path)?;
        if i.samples.len() > 100 {
            return Err(CliError::input("样本最多 100 项"));
        }
        let mut samples = Vec::with_capacity(i.samples.len());
        for sample in &i.samples {
            check_text(&sample.name, "样本名称", 120)?;
            samples.push(SampleMeasurement {
                name: sample.name.trim().to_owned(),
                concentration: MassConcentration {
                    value: sample.concentration,
                    unit: mass_concentration_unit(&sample.unit)?,
                },
            });
        }
        let output = sample_normalize_batch_calc(&SampleNormalizeBatchInput {
            target_mass_ng: i.target_mass_ng,
            reaction_volume_ul: i.reaction_volume_ul,
            samples,
        })
        .map_err(CliError::input)?;
        let rows: Vec<Value> = output
            .rows
            .iter()
            .map(|row| {
                json!({
                    "name": row.name,
                    "concentration_ng_per_ul": display_quantity(row.concentration_ng_per_ul),
                    "required_sample_volume_ul": display_quantity(row.required_sample_volume_ul),
                    "sample_volume_ul": display_quantity(row.sample_volume_ul),
                    "diluent_volume_ul": display_quantity(row.diluent_volume_ul),
                    "achieved_mass_ng": display_quantity(row.achieved_mass_ng),
                    "insufficient": row.insufficient,
                })
            })
            .collect();
        let insufficient_count = output.rows.iter().filter(|row| row.insufficient).count();
        return Ok(json!({
            "rows": rows,
            "insufficient_count": insufficient_count,
            "formula": "sample_ul = target_mass_ng ÷ (C_ng/µL)；超过反应体积时按反应体积封顶并标 insufficient；diluent_ul = 反应体积 − sample_ul",
            "assumption": "每个样本按自身实测质量浓度换算；目标质量不是实测值，稀释液补足反应体积；未计移液死体积、回收率或纯度修正"
        }));
    }
    if tool == "calculator.in_vivo_dosing" {
        let i: InVivoDosingCalc = parse_input(path)?;
        if i.body_weights_g.len() > 200 {
            return Err(CliError::input("体重最多 200 项"));
        }
        let output = in_vivo_dosing_calc(&InVivoDosingInput {
            body_weights_g: i.body_weights_g,
            dose_mg_per_kg: i.dose_mg_per_kg,
            dose_volume_ml_per_kg: i.dose_volume_ml_per_kg,
            overage_ratio: i.overage_ratio,
            stock_concentration: MassConcentration {
                value: i.stock_concentration,
                unit: mass_concentration_unit(&i.stock_unit)?,
            },
        })
        .map_err(CliError::input)?;
        let rows: Vec<Value> = output
            .rows
            .iter()
            .map(|row| {
                json!({
                    "index": row.index,
                    "body_weight_g": display_quantity(row.body_weight_g),
                    "dose_mass_mg": display_quantity(row.dose_mass_mg),
                    "injection_volume_ul": display_quantity(row.injection_volume_ul),
                })
            })
            .collect();
        return Ok(json!({
            "rows": rows,
            "total_body_weight_g": display_quantity(output.total_body_weight_g),
            "total_dose_mass_mg": display_quantity(output.total_dose_mass_mg),
            "total_injection_volume_ul": display_quantity(output.total_injection_volume_ul),
            "prepared_dose_mass_mg": display_quantity(output.prepared_dose_mass_mg),
            "preparation_volume_ul": display_quantity(output.preparation_volume_ul),
            "stock_volume_ul": display_quantity(output.stock_volume_ul),
            "diluent_volume_ul": display_quantity(output.diluent_volume_ul),
            "formula": "injection_ul = weight_g × dose_volume_mL/kg；dose_mg = weight_g ÷ 1000 × dose_mg/kg；制备量按 (1 + overage_ratio) 放大；stock_ul = prepared_mass_mg × 10^6 ÷ C_ng/µL",
            "assumption": "体重是输入值而非估算；余量只放大配制管，不放大单只注射量；母液过稀导致剂量装不进制备体积时整体拒绝；未计注射器死体积或浓度复核"
        }));
    }
    if tool == "calculator.biofilm_cfu" {
        let i: BiofilmCfuCalc = parse_input(path)?;
        if i.plates.is_empty() || i.plates.len() > 100 {
            return Err(CliError::input("每板记录需要 1–100 项"));
        }
        for (label, bound) in [("计数下限", i.countable_min), ("计数上限", i.countable_max)] {
            if !(bound.is_finite() && bound >= 0.0 && bound.fract() == 0.0) {
                return Err(CliError::input(format!("{label}必须是大于等于 0 的整数")));
            }
        }
        if i.countable_min > i.countable_max {
            return Err(CliError::input("计数下限不能大于计数上限"));
        }
        struct CfuPlate {
            raw_id: String,
            sample_id: String,
            colonies: f64,
            total_dilution: f64,
            plated_ml: f64,
            suspension_ml: f64,
        }
        let mut seen_raw = std::collections::HashSet::new();
        let mut suspensions: std::collections::HashMap<String, f64> =
            std::collections::HashMap::new();
        let mut parsed: Vec<CfuPlate> = Vec::with_capacity(i.plates.len());
        for plate in &i.plates {
            check_text(&plate.raw_id, "每板 raw ID", 160)?;
            check_text(&plate.sample_id, "样品 ID", 120)?;
            let raw_id = plate.raw_id.trim().to_owned();
            let sample_id = plate.sample_id.trim().to_owned();
            if !seen_raw.insert(raw_id.clone()) {
                return Err(CliError::input(format!("重复的每板 raw ID：{raw_id}")));
            }
            if !(plate.colonies.is_finite() && plate.colonies >= 0.0 && plate.colonies.fract() == 0.0)
            {
                return Err(CliError::input("菌落数必须是大于等于 0 的整数"));
            }
            if plate.dilution_fractions.is_empty() || plate.dilution_fractions.len() > 30 {
                return Err(CliError::input("每板逐管稀释分数需要 1–30 项"));
            }
            let mut total_dilution = 1.0f64;
            for fraction in &plate.dilution_fractions {
                if !(*fraction > 0.0 && *fraction <= 1.0) {
                    return Err(CliError::input("逐管稀释分数必须在 (0, 1] 内"));
                }
                total_dilution *= *fraction;
            }
            if !(plate.plated_volume.is_finite() && plate.plated_volume > 0.0) {
                return Err(CliError::input("涂布体积必须是正数"));
            }
            if !(plate.suspension_volume.is_finite() && plate.suspension_volume > 0.0) {
                return Err(CliError::input("悬液体积必须是正数"));
            }
            // volume_factor returns liters; CFU arithmetic is in mL.
            let plated_ml = plate.plated_volume * volume_factor(&plate.plated_unit)? * 1000.0;
            let suspension_ml = plate.suspension_volume * volume_factor(&plate.suspension_unit)? * 1000.0;
            match suspensions.get(&sample_id) {
                Some(known) if (known - suspension_ml).abs() <= 1e-9 * known.max(1.0) => {}
                Some(_) => {
                    return Err(CliError::input(format!("同样品悬液量冲突：{sample_id}")));
                }
                None => {
                    suspensions.insert(sample_id.clone(), suspension_ml);
                }
            }
            parsed.push(CfuPlate {
                raw_id,
                sample_id,
                colonies: plate.colonies,
                total_dilution,
                plated_ml,
                suspension_ml,
            });
        }
        struct CfuGroup {
            key: String,
            sample_id: String,
            total_dilution: f64,
            plated_ml: f64,
            suspension_ml: f64,
            members: Vec<usize>,
        }
        let mut groups: Vec<CfuGroup> = Vec::new();
        let mut excluded: Vec<Value> = Vec::new();
        for (index, plate) in parsed.iter().enumerate() {
            if plate.colonies < i.countable_min || plate.colonies > i.countable_max {
                let reason = if plate.colonies < i.countable_min {
                    "below_window"
                } else {
                    "above_window"
                };
                excluded.push(json!({
                    "raw_id": plate.raw_id,
                    "sample_id": plate.sample_id,
                    "colonies": plate.colonies,
                    "total_dilution": plate.total_dilution,
                    "reason": reason,
                }));
                continue;
            }
            let key = format!(
                "{}\0{:.12e}\0{:.12e}",
                plate.sample_id, plate.total_dilution, plate.plated_ml
            );
            match groups.iter_mut().find(|group| group.key == key) {
                Some(group) => group.members.push(index),
                None => groups.push(CfuGroup {
                    key,
                    sample_id: plate.sample_id.clone(),
                    total_dilution: plate.total_dilution,
                    plated_ml: plate.plated_ml,
                    suspension_ml: plate.suspension_ml,
                    members: vec![index],
                }),
            }
        }
        let mut groups_json: Vec<Value> = Vec::with_capacity(groups.len());
        for (group_no, group) in groups.iter().enumerate() {
            let sum: f64 = group.members.iter().map(|member| parsed[*member].colonies).sum();
            let mean = sum / group.members.len() as f64;
            let cfu_per_ml = mean / (group.total_dilution * group.plated_ml);
            let cfu_per_sample = cfu_per_ml * group.suspension_ml;
            let derived_from: Vec<Value> = group
                .members
                .iter()
                .map(|member| json!(parsed[*member].raw_id))
                .collect();
            groups_json.push(json!({
                "sample_id": group.sample_id,
                "total_dilution": group.total_dilution,
                "plated_volume_ml": group.plated_ml,
                "suspension_volume_ml": group.suspension_ml,
                "plate_count": group.members.len(),
                "mean_colonies": display_quantity(mean),
                "cfu_per_ml": display_quantity(cfu_per_ml),
                "cfu_per_sample": display_quantity(cfu_per_sample),
                "readouts": [
                    {"readout_id": format!("biofilm-cfu-g{:02}-per-ml", group_no + 1), "endpoint": "cfu_per_ml", "value": display_quantity(cfu_per_ml), "unit": "CFU/mL", "sample_id": group.sample_id, "result_kind": "derived", "method": "calculator.biofilm_cfu@1", "derived_from": derived_from.clone()},
                    {"readout_id": format!("biofilm-cfu-g{:02}-per-sample", group_no + 1), "endpoint": "cfu_per_sample", "value": display_quantity(cfu_per_sample), "unit": "CFU", "sample_id": group.sample_id, "result_kind": "derived", "method": "calculator.biofilm_cfu@1", "derived_from": derived_from},
                ],
            }));
        }
        return Ok(json!({
            "groups": groups_json,
            "excluded": excluded,
            "excluded_count": excluded.len(),
            "formula": "CFU/mL = mean_colonies ÷ (total_dilution × plated_volume_mL)；CFU/sample = CFU/mL × suspension_volume_mL",
            "assumption": "只平均同组重复（同样品、总稀释度、涂布量），不合并或择优不同稀释度；计数上下限含边界；窗外板只说明排除原因，不估算；同样品悬液量必须一致，冲突即拒绝；体积单位只复用 L/mL/µL，不推断默认值"
        }));
    }
    let mut db = open_db(data_dir)?;
    match tool {
        "project.list" => {
            let _: EmptyInput = parse_input(path)?;
            Ok(json!({"records":list(&db,Some("project"),None)?}))
        }
        "project.create" => {
            let i: ProjectCreate = parse_input(path)?;
            check_text(&i.name, "项目名称", 120)?;
            let tx = db
                .unchecked_transaction()
                .map_err(|e| CliError::internal(e.to_string()))?;
            let r = insert(
                &tx,
                "project",
                None,
                None,
                1,
                &json!({"name":i.name.trim(),"description":i.description.trim()}),
            )?;
            tx.commit().map_err(|e| CliError::internal(e.to_string()))?;
            Ok(json!({"record":r}))
        }
        "project.revise" => {
            let i: ProjectRevise = parse_input(path)?;
            if i.name.is_none() && i.description.is_none() {
                return Err(CliError::input("至少提供 name 或 description 中的一项"));
            }
            let (_, _, _, revision, raw) =
                kind_id(&db, &i.id)?.ok_or_else(|| CliError::missing("项目不存在"))?;
            if revision != i.expected_revision {
                return Err(CliError::conflict(format!(
                    "项目版本已从 v{} 更新为 v{revision}；请先重新读取。",
                    i.expected_revision
                )));
            }
            let mut data: Value =
                serde_json::from_str(&raw).map_err(|e| CliError::internal(e.to_string()))?;
            if let Some(name) = i.name {
                check_text(&name, "项目名称", 120)?;
                data["name"] = json!(name.trim())
            }
            if let Some(description) = i.description {
                data["description"] = json!(description.trim())
            }
            Ok(json!({"record":update_data(&db,&i.id,i.expected_revision,"project",&data)?}))
        }
        "design.list" => {
            let i: ProjectFilter = parse_input(path)?;
            Ok(json!({"records":list(&db,Some("design"),i.project_id.as_deref())?}))
        }
        "design.get" => {
            let i: IdInput = parse_input(path)?;
            let r = get(&db, &i.id)?;
            if r["kind"] != "design" {
                return Err(CliError::input("指定记录不是实验设计"));
            }
            let revision = r["revision"]
                .as_i64()
                .ok_or_else(|| CliError::internal("设计版本号无效"))?;
            let version_id: String = db
                .query_row(
                    "SELECT id FROM records WHERE kind='design_version' AND parent_id=?1 AND revision=?2",
                    params![i.id, revision],
                    |row| row.get(0),
                )
                .optional()
                .map_err(|e| CliError::internal(e.to_string()))?
                .ok_or_else(|| CliError::internal("当前设计版本快照缺失"))?;
            Ok(json!({"design":r,"version":get(&db,&version_id)?}))
        }
        "design.map" => {
            let i: IdInput = parse_input(path)?;
            Ok(design_map(&db, &i.id)?)
        }
        "design.create" => {
            let i: DesignCreate = parse_input(path)?;
            ensure_project(&db, &i.project_id)?;
            let tx = db
                .unchecked_transaction()
                .map_err(|e| CliError::internal(e.to_string()))?;
            let registry = load_labware_registry(&db)?;
            let r = create_design_tx(&tx, &i.project_id, &i.design, &registry)?;
            tx.commit().map_err(|e| CliError::internal(e.to_string()))?;
            Ok(r)
        }
        "design.revise" => {
            let i: DesignRevise = parse_input(path)?;
            let changes =
                serde_json::to_value(&i.changes).map_err(|e| CliError::internal(e.to_string()))?;
            if !changes
                .as_object()
                .is_some_and(|o| o.values().any(|v| !v.is_null()))
            {
                return Err(CliError::input("changes 至少包含一个非 null 字段"));
            }
            let (kind, project, parent, revision, raw) = kind_id(&db, &i.id)?
                .ok_or_else(|| CliError::missing(format!("设计不存在：{}", i.id)))?;
            if kind != "design" {
                return Err(CliError::input("指定记录不是实验设计"));
            }
            if revision != i.expected_revision {
                return Err(CliError::conflict(format!(
                    "设计版本已从 v{} 更新为 v{}；先重新读取后再提交。",
                    i.expected_revision, revision
                )));
            }
            let previous: DesignFields = serde_json::from_str(&raw)
                .map_err(|e| CliError::internal(format!("现有设计无法读取：{e}")))?;
            let mut value: Value =
                serde_json::from_str(&raw).map_err(|e| CliError::internal(e.to_string()))?;
            merge_design(&mut value, &i.changes);
            let mut d: DesignFields = serde_json::from_value(value)
                .map_err(|e| CliError::input(format!("修改后设计无效：{e}")))?;
            normalize_design(&mut d, Some(&previous));
            validate_design_with_registry(&d, &load_labware_registry(&db)?)?;
            let next = revision + 1;
            let tx = db
                .unchecked_transaction()
                .map_err(|e| CliError::internal(e.to_string()))?;
            let newdata =
                serde_json::to_string(&d).map_err(|e| CliError::internal(e.to_string()))?;
            tx.execute(
                "UPDATE records SET revision=?2,data_json=?3,updated_at=?4 WHERE id=?1",
                params![i.id, next, newdata, stamp()],
            )
            .map_err(|e| CliError::internal(e.to_string()))?;
            let snapshot = insert(
                &tx,
                "design_version",
                project.as_deref(),
                Some(&i.id),
                next,
                &json!({"design_id":i.id,"design_revision":next,"snapshot":d}),
            )?;
            tx.commit().map_err(|e| CliError::internal(e.to_string()))?;
            let _ = parent;
            Ok(json!({"record":get(&db,&i.id)? ,"version":snapshot}))
        }
        "run.list" => {
            let i: ProjectFilter = parse_input(path)?;
            Ok(json!({"records":list(&db,Some("run"),i.project_id.as_deref())?}))
        }
        "run.set_status" => {
            let i: StatusUpdate = parse_input(path)?;
            if i.status == "in_progress" {
                ensure_run_protocol_ready(&db, &i.id)?;
            }
            Ok(
                json!({"record":set_status(&db,i,"run",&["planned","in_progress","completed","blocked","cancelled"])?}),
            )
        }
        "run.revise" => {
            let i: RunRevise = parse_input(path)?;
            let mut changes = serde_json::Map::new();
            if let Some(title) = i.title {
                changes.insert("title".into(), json!(title));
            }
            if let Some(scheduled_date) = i.scheduled_date {
                changes.insert("scheduled_date".into(), json!(scheduled_date));
            }
            Ok(json!({"record":patch_run(&db,&i.id,i.expected_revision,&Value::Object(changes))?}))
        }
        "asterdoc.load" => {
            let input: Value = parse_input(path)?;
            let design_id = input.get("design_id").or_else(|| input.get("designId"))
                .and_then(Value::as_str)
                .ok_or_else(|| CliError::input("design_id is required"))?;
            load_asterdoc(&db, design_id)
        }
        "asterdoc.save" => {
            let input: Value = parse_input(path)?;
            save_asterdoc_version(&mut db, &input)
        }
        "asterdoc.command" => {
            let input: Value = parse_input(path)?;
            apply_asterdoc_command(&mut db, &input)
        }
        "asterdoc.compile" => {
            let input: Value = parse_input(path)?;
            let design_id = input.get("design_id").and_then(Value::as_str)
                .ok_or_else(|| CliError::input("design_id is required"))?;
            let revision = input.get("revision").and_then(Value::as_u64)
                .ok_or_else(|| CliError::input("revision is required"))?;
            let doc = load_asterdoc_at(&db, design_id, revision)?;
            let design = compile_asterdoc_to_design(doc["source"].as_str().unwrap_or(""))?;
            Ok(json!({"design": design, "document": doc}))
        }
        "asterdoc.create_run" => {
            let input: Value = parse_input(path)?;
            let design_id = input.get("design_id").and_then(Value::as_str)
                .ok_or_else(|| CliError::input("design_id is required"))?;
            let revision = input.get("revision").and_then(Value::as_u64)
                .ok_or_else(|| CliError::input("revision is required"))?;
            let op = input.get("operation_id").and_then(Value::as_str);
            let title = input.get("title").and_then(Value::as_str);
            let acknowledged: Vec<String> = input.get("acknowledged_warnings")
                .and_then(Value::as_array).into_iter().flatten()
                .filter_map(|value| value.as_str().map(str::to_owned)).collect();
            create_run_from_asterdoc(&mut db, design_id, revision, op, title, &acknowledged)
        }
        "sdk.modules" => {
            let _: EmptyInput = parse_input(path)?;
            let modules = asterdoc_spec()["modules"].as_array().cloned().unwrap_or_default();
            let summaries: Vec<Value> = modules
                .iter()
                .map(|module| {
                    json!({
                        "kind": module.get("kind").cloned().unwrap_or(Value::Null),
                        "schema": module.get("schema").cloned().unwrap_or(Value::Null),
                        "title": module.get("title").cloned().unwrap_or(Value::Null),
                        "category": module.get("category").cloned().unwrap_or(Value::Null),
                        "capabilities": module.get("capabilities").cloned().unwrap_or(Value::Null),
                        "renderers": module.get("renderers").cloned().unwrap_or(Value::Null),
                        "actions": module.get("actions").cloned().unwrap_or(Value::Null)
                    })
                })
                .collect();
            Ok(json!({"modules": summaries}))
        }
        "sdk.describe" => {
            let input: Value = parse_input(path)?;
            let kind = input
                .get("kind")
                .and_then(Value::as_str)
                .ok_or_else(|| CliError::input("kind is required"))?;
            let schema = input
                .get("schema")
                .and_then(Value::as_i64)
                .ok_or_else(|| CliError::input("schema is required"))?;
            let module = asterdoc_module_spec(kind, schema)
                .ok_or_else(|| CliError::missing(format!("unknown module {kind}@{schema}")))?;
            Ok(json!({"module": module}))
        }
        "sdk.validate" => {
            let input: Value = parse_input(path)?;
            let source = if let Some(source) = input.get("source").and_then(Value::as_str) {
                source.to_owned()
            } else {
                let design_id = input
                    .get("design_id")
                    .and_then(Value::as_str)
                    .ok_or_else(|| CliError::input("source or design_id is required"))?;
                let revision = input.get("revision").and_then(Value::as_u64);
                let doc = if let Some(revision) = revision {
                    load_asterdoc_at(&db, design_id, revision)?
                } else {
                    load_asterdoc(&db, design_id)?
                };
                doc.get("source").and_then(Value::as_str).unwrap_or("").to_owned()
            };
            let mut errors: Vec<String> = Vec::new();
            let mut warnings: Vec<String> = Vec::new();
            let valid = validate_asterdoc_source(&source).is_ok();
            if let Err(error) = validate_asterdoc_source(&source) {
                errors.push(error.message);
            }
            let mut compiled = false;
            let mut design_value = Value::Null;
            if valid {
                match compile_asterdoc_to_design(&source) {
                    Ok(design) => {
                        compiled = true;
                        design_value = design;
                    }
                    Err(error) => {
                        compiled = false;
                        errors.push(error.message);
                    }
                }
            }
            let modules: Vec<Value> = if valid {
                parse_asterdoc_modules(&source)
                    .unwrap_or_default()
                    .iter()
                    .map(|m| { let mj = serde_json::to_value(m).unwrap_or(Value::Null); json!({"kind": mj.get("kind").cloned().unwrap_or(Value::Null), "schema": mj.get("schema").cloned().unwrap_or(Value::Null)}) })
                    .collect()
            } else {
                Vec::new()
            };
            let mut out = json!({
                "valid": valid,
                "compiled": compiled,
                "errors": errors,
                "warnings": warnings,
                "modules": modules
            });
            if compiled {
                out["design"] = design_value;
            }
            Ok(out)
        }
        "asterdoc.inspect" => {
            let input: Value = parse_input(path)?;
            let design_id = input
                .get("design_id")
                .and_then(Value::as_str)
                .ok_or_else(|| CliError::input("design_id is required"))?;
            let revision = input.get("revision").and_then(Value::as_u64);
            let doc = if let Some(revision) = revision {
                load_asterdoc_at(&db, design_id, revision)?
            } else {
                load_asterdoc(&db, design_id)?
            };
            let module_id = input.get("module_id").and_then(Value::as_str);
            let source = doc.get("source").and_then(Value::as_str).unwrap_or("");
            let modules = parse_asterdoc_modules(source)?;
            let mut inspected = Vec::new();
            for module in &modules {
                let kind = module.get("kind").and_then(|v| v.as_str()).unwrap_or("").to_owned();
                let id = module.get("id").and_then(|v| v.as_str()).unwrap_or("").to_owned();
                if let Some(filter) = module_id {
                    if id != filter {
                        continue;
                    }
                }
                let schema = module.get("schema").and_then(|v| v.as_i64()).unwrap_or(0);
                let spec = asterdoc_module_spec(&kind, schema);
                let manifest = spec.map(|spec_value| {
                    json!({
                        "renderers": spec_value.get("renderers").cloned().unwrap_or(Value::Null),
                        "actions": spec_value.get("actions").cloned().unwrap_or(Value::Null)
                    })
                });
                let data = serde_json::to_value(module).unwrap_or(Value::Null);
                inspected.push(json!({
                    "kind": kind,
                    "id": id,
                    "schema": schema,
                    "data": data,
                    "manifest": manifest.unwrap_or(Value::Null)
                }));
            }
            if let Some(filter) = module_id {
                if inspected.is_empty() {
                    return Err(CliError::missing(format!("module {filter} not found")));
                }
            }
            Ok(json!({
                "design_id": design_id,
                "revision": doc.get("revision").cloned().unwrap_or(Value::Null),
                "source_hash": doc.get("source_hash").cloned().unwrap_or(Value::Null),
                "modules": inspected
            }))
        }
        "asterdoc.assess" => {
            let input: Value = parse_input(path)?;
            assess_asterdoc(&db, &input)
        }

        "asterdoc.validate" => {
            let input: Value = parse_input(path)?;
            let source = input
                .get("source")
                .and_then(Value::as_str)
                .ok_or_else(|| CliError::input("source is required"))?;
            if let Err(error) = validate_asterdoc_source(source) {
                return Ok(json!({
                    "valid": false,
                    "compiled": false,
                    "error": {"code": error.code, "message": error.message}
                }));
            }
            match compile_asterdoc_to_design(source) {
                Ok(design) => Ok(json!({
                    "valid": true,
                    "compiled": true,
                    "design": design
                })),
                Err(error) => Ok(json!({
                    "valid": false,
                    "compiled": false,
                    "error": {"code": error.code, "message": error.message}
                })),
            }
        }
        "run.workspace" => {
            let i: RunWorkspaceInput = parse_input(path)?;
            let view = parse_projection_view(i.view.as_deref())?;
            run_workspace_for_view(&db, &i.id, view)
        }
        "workflow.progress" => {
            let i: WorkflowProgressInput = parse_input(path)?;
            if i.run_id.trim().is_empty() {
                return Err(CliError::input("run_id 不能为空"));
            }
            let workspace: asterlab_core::run_workspace::RunWorkspace =
                serde_json::from_value(run_workspace(&db, &i.run_id)?)
                    .map_err(|e| CliError::internal(e.to_string()))?;
            let query = asterlab_core::sample_progress::SampleProgressQuery {
                step_path: i.step_path,
                timepoint: i.timepoint,
                sample_id: i.sample_id,
                sample_ids: i.sample_ids,
            };
            serde_json::to_value(asterlab_core::sample_progress::build_from_workspace(
                &workspace, &query,
            ))
            .map_err(|e| CliError::internal(e.to_string()))
        }
        "protocol.version.get" => {
            let input: ProtocolVersionGet = parse_input(path)?;
            protocol_version_get(&db, &input.protocol_version_id)
        }
        "protocol.version.list" => {
            let input: ProtocolVersionList = parse_input(path)?;
            Ok(json!({"versions":protocol_version_list(&db, &input.protocol_id)?}))
        }
        "run.create" => {
            require_trusted_writer(tool)?;
            let i: RunCreate = parse_input(path)?;
            let tx = db
                .unchecked_transaction()
                .map_err(|e| CliError::internal(e.to_string()))?;
            let record = create_run_with_source_in_transaction(
                &tx,
                &i.design_id,
                &i.title,
                i.scheduled_date.as_deref(),
                i.protocol_version_id.as_deref(),
                i.view_id.as_deref(),
                i.repeat_of_run_id.as_deref(),
                i.repeat_relationship.as_str(),
                i.source_run_id.as_deref(),
                None,
            )?;
            tx.commit().map_err(|e| CliError::internal(e.to_string()))?;
            Ok(json!({"record":record}))
        }
        "task.create" => {
            let i: TaskCreate = parse_input(path)?;
            ensure_project(&db, &i.project_id)?;
            check_text(&i.title, "任务名称", 160)?;
            check_text(&i.scheduled_date, "计划日期", 32)?;
            validate_date(&i.scheduled_date, "计划日期")?;
            if !["planned", "in_progress", "done", "blocked"].contains(&i.status.as_str()) {
                return Err(CliError::input(
                    "status 必须是 planned、in_progress、done、blocked 之一",
                ));
            }
            if let Some(id) = &i.run_id {
                let (run_kind, run_project, _, _, _) =
                    kind_id(&db, id)?.ok_or_else(|| CliError::missing("实验运行不存在"))?;
                if run_kind != "run" {
                    return Err(CliError::input("run_id 必须指向实验运行"));
                }
                if run_project.as_deref() != Some(i.project_id.as_str()) {
                    return Err(CliError::input("run_id 与 project_id 不属于同一项目"));
                }
            }
            let tx = db
                .unchecked_transaction()
                .map_err(|e| CliError::internal(e.to_string()))?;
            let r = insert(
                &tx,
                "task",
                Some(&i.project_id),
                i.run_id.as_deref(),
                1,
                &json!({"title":i.title.trim(),"scheduled_date":i.scheduled_date,"status":i.status,"notes":i.notes}),
            )?;
            tx.commit().map_err(|e| CliError::internal(e.to_string()))?;
            Ok(json!({"record":r}))
        }
        "task.set_status" => {
            let i: StatusUpdate = parse_input(path)?;
            Ok(
                json!({"record":set_status(&db,i,"task",&["planned","in_progress","done","blocked"])?}),
            )
        }
        "protocol.list" => {
            let i: ProjectFilter = parse_input(path)?;
            Ok(json!({"records":list(&db,Some("protocol"),i.project_id.as_deref())?}))
        }
        "protocol.block.list" => {
            let i: ProtocolBlockList = parse_input(path)?;
            Ok(json!({"records":list(&db,Some("protocol_block"),i.project_id.as_deref())?}))
        }
        "protocol.block.get" => {
            let i: ProtocolBlockGet = parse_input(path)?;
            let block = get(&db, &i.id)?;
            if block["kind"] != "protocol_block" {
                return Err(CliError::input("id 必须指向 protocol_block"));
            }
            let versions: Vec<Value> = list(
                &db,
                Some("protocol_block_version"),
                block["project_id"].as_str(),
            )?
            .into_iter()
            .filter(|v| v["parent_id"] == i.id)
            .collect();
            let latest = versions
                .iter()
                .max_by_key(|v| v["revision"].as_i64().unwrap_or(0))
                .cloned()
                .ok_or_else(|| CliError::internal("步骤块缺少 v1 版本"))?;
            Ok(json!({"block":block,"latest_version":latest,"versions":versions}))
        }
        "protocol.block.create" => {
            let i: ProtocolBlockCreate = parse_input(path)?;
            let tx = db
                .unchecked_transaction()
                .map_err(|e| CliError::internal(e.to_string()))?;
            let payload = serde_json::to_value(i).map_err(|e| CliError::internal(e.to_string()))?;
            let result = create_protocol_block_records(
                &tx,
                payload["project_id"].as_str().unwrap_or_default(),
                &payload,
            )?;
            tx.commit().map_err(|e| CliError::internal(e.to_string()))?;
            Ok(result)
        }
        "semantic.apply_binding_patch" => {
            require_trusted_writer(tool)?;
            let i: SemanticPatchInput = parse_input(path)?;
            if i.changeset.target.kind
                != asterlab_core::semantic_changeset::ChangeTargetKind::BlockVersion
            {
                return Err(CliError::input(
                    "semantic.apply_binding_patch 只接受 block_version 目标",
                ));
            }
            let target_id = i.changeset.target.object_id.clone();
            let tx = db
                .unchecked_transaction()
                .map_err(|e| CliError::internal(e.to_string()))?;
            let (record, data) = block_version_data(&tx, &target_id)?;
            if record["revision"].as_i64() != Some(data.version) {
                return Err(CliError::conflict("BlockVersion 版本记录不一致"));
            }
            let block_id = data.block_id.clone();
            let latest: i64 = tx.query_row(
                "SELECT COALESCE(MAX(revision),0) FROM records WHERE kind='protocol_block_version' AND parent_id=?1",
                [&block_id],
                |row| row.get(0),
            ).map_err(|e| CliError::internal(e.to_string()))?;
            if latest != record["revision"].as_i64().unwrap_or(0) {
                return Err(CliError::conflict(format!(
                    "目标不是最新 BlockVersion（最新 v{}）；请重新读取后再提交",
                    latest
                )));
            }
            if data.version != i.changeset.expected_version {
                return Err(CliError::conflict(format!(
                    "expected_version={} 与当前 BlockVersion v{} 不一致",
                    i.changeset.expected_version, data.version
                )));
            }
            let project = record["project_id"].as_str().unwrap_or_default();
            let (_, _, parent_project, _, _) =
                kind_id(&tx, &block_id)?.ok_or_else(|| CliError::missing("Block 不存在"))?;
            if parent_project.as_deref() != Some(project) {
                return Err(CliError::input("BlockVersion 与 Block 不属于同一项目"));
            }
            let (mut next, diff) = apply_binding_changes(&tx, &data, &i.changeset)?;
            validate_block_content(&tx, project, &next.parameter_slots, &next.steps)?;
            next.version = latest + 1;
            next.module_origin = next.module_contract.as_ref().map(|_| ModuleOrigin::Derived);
            next.module_contract_authority = next
                .module_contract
                .as_ref()
                .map(|_| asterlab_core::protocol_block::ModuleContractAuthority::SourceOnly);
            next.catalog_review = None;
            let serialized =
                serde_json::to_value(&next).map_err(|e| CliError::internal(e.to_string()))?;
            let version = insert(
                &tx,
                "protocol_block_version",
                Some(project),
                Some(&block_id),
                next.version,
                &serialized,
            )?;
            tx.commit().map_err(|e| CliError::internal(e.to_string()))?;
            Ok(json!({"version":version,"changes":diff,"requires_protocol_rebind":true}))
        }
        "labware.define" => {
            require_trusted_writer(tool)?;
            let i: LabwareDefine = parse_input(path)?;
            let tx = db
                .unchecked_transaction()
                .map_err(|e| CliError::internal(e.to_string()))?;
            let payload = serde_json::to_value(i).map_err(|e| CliError::internal(e.to_string()))?;
            let result = create_labware_profile(&tx, &payload)?;
            tx.commit().map_err(|e| CliError::internal(e.to_string()))?;
            Ok(result)
        }
        "component.list" => {
            let _: EmptyInput = parse_input(path)?;
            let registry = load_component_registry(&db)?;
            let definitions = registry
                .definitions()
                .map(|definition| {
                    json!({
                        "id": definition.id,
                        "version": definition.version,
                        "family": definition.family,
                        "title": definition.title,
                        "implemented": registry.is_implemented(&definition.id, definition.version)
                    })
                })
                .collect::<Vec<_>>();
            Ok(json!({"definitions":definitions}))
        }
        "component.define" => {
            require_trusted_writer(tool)?;
            let i: ComponentDefine = parse_input(path)?;
            let tx = db
                .unchecked_transaction()
                .map_err(|e| CliError::internal(e.to_string()))?;
            let payload = serde_json::to_value(i).map_err(|e| CliError::internal(e.to_string()))?;
            let result = create_component_definition(&tx, &payload)?;
            tx.commit().map_err(|e| CliError::internal(e.to_string()))?;
            Ok(result)
        }
        "view.data_preview" => {
            let i: ViewDataPreview = parse_input(path)?;
            let payload = serde_json::to_value(i).map_err(|e| CliError::internal(e.to_string()))?;
            view_data_preview(&db, &payload)
        }
        "view.project" => {
            let i: ViewProject = parse_input(path)?;
            let payload = serde_json::to_value(i).map_err(|e| CliError::internal(e.to_string()))?;
            project_view_records(&db, &payload)
        }
        "view.create" => {
            let i: ViewCreate = parse_input(path)?;
            let tx = db
                .unchecked_transaction()
                .map_err(|e| CliError::internal(e.to_string()))?;
            let payload = serde_json::to_value(i).map_err(|e| CliError::internal(e.to_string()))?;
            let result = create_view_records(
                &tx,
                payload["project_id"].as_str().unwrap_or_default(),
                &payload,
            )?;
            tx.commit().map_err(|e| CliError::internal(e.to_string()))?;
            Ok(result)
        }
        "view.get" => {
            let i: ViewGet = parse_input(path)?;
            get_view(&db, &i.id)
        }
        "view.proposal.create" => {
            let i: ViewProposalCreate = parse_input(path)?;
            let tx = db
                .unchecked_transaction()
                .map_err(|e| CliError::internal(e.to_string()))?;
            let changeset = serde_json::to_value(&i.changeset)
                .map_err(|e| CliError::internal(e.to_string()))?;
            let result = create_view_proposal_tx(&tx, &i.view_id, &changeset, &i.source)?;
            tx.commit().map_err(|e| CliError::internal(e.to_string()))?;
            Ok(result)
        }
        "view.proposal.list" => {
            let i: ViewProposalList = parse_input(path)?;
            list_view_proposals(&db, &i.view_id)
        }
        "view.proposal.accept" => {
            let i: ViewProposalAction = parse_input(path)?;
            let tx = db
                .unchecked_transaction()
                .map_err(|e| CliError::internal(e.to_string()))?;
            let result = accept_view_proposal_tx(&tx, &i.view_id, &i.proposal_id)?;
            tx.commit().map_err(|e| CliError::internal(e.to_string()))?;
            Ok(result)
        }
        "view.proposal.reject" => {
            let i: ViewProposalAction = parse_input(path)?;
            let tx = db
                .unchecked_transaction()
                .map_err(|e| CliError::internal(e.to_string()))?;
            let result = reject_view_proposal_tx(&tx, &i.view_id, &i.proposal_id)?;
            tx.commit().map_err(|e| CliError::internal(e.to_string()))?;
            Ok(result)
        }
        "view.revise" => {
            let i: ViewRevise = parse_input(path)?;
            let tx = db
                .unchecked_transaction()
                .map_err(|e| CliError::internal(e.to_string()))?;
            let view_id = i.view_id.clone();
            let payload = serde_json::to_value(i).map_err(|e| CliError::internal(e.to_string()))?;
            let result = revise_view_records(&tx, &view_id, &payload)?;
            tx.commit().map_err(|e| CliError::internal(e.to_string()))?;
            Ok(result)
        }
        "semantic.inspect" => {
            let i: SemanticInspectInput = parse_input(path)?;
            let (_, data) = block_version_data(&db, &i.block_version_id)?;
            serde_json::to_value(inspect_block(&i.block_version_id, &data))
                .map_err(|e| CliError::internal(e.to_string()))
        }
        "semantic.search" => {
            let i: SemanticSearchInput = parse_input(path)?;
            let mut modules = Vec::new();
            for version in list(&db, Some("protocol_block_version"), i.project_id.as_deref())? {
                if version["data"]["module_origin"].as_str() != Some("imported")
                    || version["data"]["module_contract_authority"].as_str() != Some("current")
                {
                    continue;
                }
                if let Some(module) = version["data"]
                    .get("module_contract")
                    .cloned()
                    .and_then(|value| serde_json::from_value(value).ok())
                {
                    let module: OperationModule = module;
                    modules.push(module);
                }
            }
            Ok(json!({"modules": search_modules(&modules, &i.query)}))
        }
        "semantic.validate" => {
            let i: SemanticPatchInput = parse_input(path)?;
            match i.changeset.validate() {
                Ok(()) => Ok(json!({"valid": true, "issues": []})),
                Err(issues) => Ok(json!({"valid": false, "issues": issues})),
            }
        }
        "semantic.propose_patch" => {
            let i: SemanticPatchInput = parse_input(path)?;
            i.changeset.validate().map_err(semantic_error)?;
            Ok(json!({"summary": i.changeset.summary(), "requires_confirmation": true}))
        }
        "semantic.assess_patch" => {
            let i: SemanticPatchInput = parse_input(path)?;
            if i.changeset.target.kind
                != asterlab_core::semantic_changeset::ChangeTargetKind::BlockVersion
            {
                return Err(CliError::input(
                    "semantic.assess_patch 只接受 block_version 目标",
                ));
            }
            let target_id = &i.changeset.target.object_id;
            let (record, data) = block_version_data(&db, target_id)?;
            let (next, _) = apply_binding_changes(&db, &data, &i.changeset)?;
            let project = record["project_id"].as_str().unwrap_or_default();
            validate_block_content(&db, project, &next.parameter_slots, &next.steps)?;
            let mut assessment =
                assess_block_patch(target_id, &data, &i.changeset).map_err(semantic_error)?;
            let mut refs = assessment.referencing_refs.clone();
            for version in list(&db, Some("protocol_version"), None)? {
                if version["data"]["root_block_version_id"].as_str() == Some(target_id.as_str()) {
                    refs.push(format!(
                        "protocol_version:{}",
                        version["id"].as_str().unwrap_or_default()
                    ));
                }
            }
            for run in list(&db, Some("run"), None)? {
                if run["data"]["protocol_manifest"]
                    .as_array()
                    .is_some_and(|entries| {
                        entries.iter().any(|entry| {
                            entry["block_version_ids"]
                                .as_array()
                                .is_some_and(|ids| ids.iter().any(|id| id == target_id))
                        })
                    })
                {
                    refs.push(format!("run:{}", run["id"].as_str().unwrap_or_default()));
                }
            }
            assessment.referencing_refs = refs.clone();
            assessment.would_require_rebind = refs;
            assessment.invalidated_refs = vec![];
            serde_json::to_value(assessment).map_err(|e| CliError::internal(e.to_string()))
        }
        "catalog.review.record" => {
            require_trusted_writer(tool)?;
            let i: CatalogReviewCreate = parse_input(path)?;
            let tx = db
                .unchecked_transaction()
                .map_err(|e| CliError::internal(e.to_string()))?;
            let payload = serde_json::to_value(i).map_err(|e| CliError::internal(e.to_string()))?;
            let result = create_catalog_review_records(
                &tx,
                payload["project_id"].as_str().unwrap_or_default(),
                &payload,
            )?;
            tx.commit().map_err(|e| CliError::internal(e.to_string()))?;
            Ok(result)
        }
        "catalog.operation.import" => {
            require_trusted_writer(tool)?;
            let i: CatalogOperationImport = parse_input(path)?;
            let tx = db
                .unchecked_transaction()
                .map_err(|e| CliError::internal(e.to_string()))?;
            let payload = serde_json::to_value(i).map_err(|e| CliError::internal(e.to_string()))?;
            let result = import_catalog_operation_records(
                &tx,
                payload["project_id"].as_str().unwrap_or_default(),
                &payload,
            )?;
            tx.commit().map_err(|e| CliError::internal(e.to_string()))?;
            Ok(result)
        }
        "protocol.block.revise" => {
            let i: ProtocolBlockRevise = parse_input(path)?;
            let tx = db
                .unchecked_transaction()
                .map_err(|e| CliError::internal(e.to_string()))?;
            let block_id = i.block_id.clone();
            let payload = serde_json::to_value(i).map_err(|e| CliError::internal(e.to_string()))?;
            let result = revise_protocol_block_records(&tx, &block_id, &payload)?;
            tx.commit().map_err(|e| CliError::internal(e.to_string()))?;
            Ok(result)
        }
        "protocol.block.preview" => {
            let i: ProtocolBlockPreview = parse_input(path)?;
            Ok(block_preview(&db, &i.block_version_id, &i.bindings)?)
        }
        "protocol.create" => {
            let i: ProtocolCreate = parse_input(path)?;
            check_text(&i.title, "Protocol 名称", 160)?;
            if let Some(p) = &i.project_id {
                ensure_project(&db, p)?
            }
            if i.body.is_none() && i.root_block_version_id.is_none() {
                return Err(CliError::input(
                    "Protocol 必须提供正文或 root_block_version_id",
                ));
            }
            if let Some(block_version_id) = &i.root_block_version_id {
                let (block_record, block_data) = block_version_data(&db, block_version_id)?;
                if i.project_id.as_deref() != block_record["project_id"].as_str() {
                    return Err(CliError::input("Protocol 与根步骤块必须属于同一项目"));
                }
                validate_root_bindings(&block_data.parameter_slots, &i.root_bindings)?;
            } else if !i.root_bindings.is_empty() {
                return Err(CliError::input(
                    "未指定 root_block_version_id 时不能提供 root_bindings",
                ));
            }
            let tx = db
                .unchecked_transaction()
                .map_err(|e| CliError::internal(e.to_string()))?;
            let p = insert(
                &tx,
                "protocol",
                i.project_id.as_deref(),
                i.project_id.as_deref(),
                1,
                &json!({"title":i.title.trim()}),
            )?;
            let v = insert(
                &tx,
                "protocol_version",
                i.project_id.as_deref(),
                p["id"].as_str(),
                1,
                &json!({"protocol_id":p["id"],"version":1,"body":i.body.unwrap_or_default(),"root_block_version_id":i.root_block_version_id,"root_bindings":i.root_bindings}),
            )?;
            tx.commit().map_err(|e| CliError::internal(e.to_string()))?;
            Ok(json!({"protocol":p,"version":v}))
        }
        "protocol.revise" => {
            let i: ProtocolRevise = parse_input(path)?;
            if !has_kind(&db, &i.protocol_id, "protocol")? {
                return Err(CliError::missing("Protocol 不存在"));
            }
            let latest:i64=db.query_row("SELECT COALESCE(MAX(revision),0) FROM records WHERE kind='protocol_version' AND parent_id=?1",[&i.protocol_id],|r|r.get(0)).map_err(|e|CliError::internal(e.to_string()))?;
            if latest != i.expected_version {
                return Err(CliError::conflict(format!(
                    "Protocol 已從 v{} 更新為 v{}；請先重新讀取。",
                    i.expected_version, latest
                )));
            }
            let (_, project, _, _, _) = kind_id(&db, &i.protocol_id)?
                .ok_or_else(|| CliError::missing("Protocol 不存在"))?;
            if i.body.is_none() && i.root_block_version_id.is_none() && i.root_bindings.is_none() {
                return Err(CliError::input(
                    "至少提供 body、root_block_version_id 或 root_bindings 之一",
                ));
            }
            let previous_raw: String = db.query_row("SELECT data_json FROM records WHERE kind='protocol_version' AND parent_id=?1 AND revision=?2", params![i.protocol_id, latest], |r| r.get(0)).map_err(|e| CliError::internal(e.to_string()))?;
            let previous: Value = serde_json::from_str(&previous_raw)
                .map_err(|e| CliError::internal(e.to_string()))?;
            let body = i
                .body
                .unwrap_or_else(|| previous["body"].as_str().unwrap_or_default().to_owned());
            let root_id = match i.root_block_version_id {
                None => previous
                    .get("root_block_version_id")
                    .and_then(Value::as_str)
                    .map(str::to_owned),
                Some(Value::Null) => None,
                Some(Value::String(id)) => Some(id),
                Some(_) => {
                    return Err(CliError::input("root_block_version_id 必须是字符串或 null"))
                }
            };
            let previous_bindings = previous
                .get("root_bindings")
                .and_then(Value::as_array)
                .cloned()
                .unwrap_or_default();
            let root_bindings =
                match (&root_id, i.root_bindings) {
                    (Some(_), Some(values)) => serde_json::to_value(values)
                        .map_err(|e| CliError::internal(e.to_string()))?,
                    (Some(_), None) => json!(previous_bindings),
                    (None, Some(values)) => serde_json::to_value(values)
                        .map_err(|e| CliError::internal(e.to_string()))?,
                    (None, None) => json!([]),
                };
            if let Some(block_version_id) = &root_id {
                let (block_record, block_data) = block_version_data(&db, block_version_id)?;
                if project.as_deref() != block_record["project_id"].as_str() {
                    return Err(CliError::input("Protocol 与根步骤块必须属于同一项目"));
                }
                let typed_bindings: Vec<RootParameterValue> =
                    serde_json::from_value(root_bindings.clone())
                        .map_err(|e| CliError::input(format!("root_bindings 格式无效：{e}")))?;
                validate_root_bindings(&block_data.parameter_slots, &typed_bindings)?;
            } else if root_bindings.as_array().is_some_and(|v| !v.is_empty()) {
                return Err(CliError::input(
                    "清除 root_block_version_id 时必须清空 root_bindings",
                ));
            }
            let tx = db
                .unchecked_transaction()
                .map_err(|e| CliError::internal(e.to_string()))?;
            let v = insert(
                &tx,
                "protocol_version",
                project.as_deref(),
                Some(&i.protocol_id),
                latest + 1,
                &json!({"protocol_id":i.protocol_id,"version":latest+1,"body":body,"root_block_version_id":root_id,"root_bindings":root_bindings}),
            )?;
            tx.commit().map_err(|e| CliError::internal(e.to_string()))?;
            Ok(json!({"version":v}))
        }
        "agent_edit.validate" => {
            let i: Value = parse_input(path)?;
            validate_agent_edit(&db, i)
        }
        "agent_edit.commit" => {
            let i: Value = parse_input(path)?;
            commit_agent_edit(&mut db, i)
        }
        "step.edit" => {
            let i: Value = parse_input(path)?;
            step_edit_command(&mut db, i)
        }
        "plate.create" => {
            let i: PlateCreate = parse_input(path)?;
            let (kind, project, _, _, _) =
                kind_id(&db, &i.run_id)?.ok_or_else(|| CliError::missing("实验运行不存在"))?;
            if kind != "run" {
                return Err(CliError::input("run_id 必须指向实验运行"));
            }
            if let Some(assay_id) = i.assay_id.as_deref().filter(|id| !id.is_empty()) {
                validate_run_assay_reference(&db, &i.run_id, assay_id, i.timepoint.as_deref())?;
            } else if i
                .timepoint
                .as_deref()
                .is_some_and(|value| !value.trim().is_empty())
            {
                return Err(CliError::input(
                    "填写板图 timepoint 时必须同时关联 assay_id",
                ));
            }
            check_text(&i.name, "板名", 100)?;
            if !(1..=32).contains(&i.rows) || !(1..=48).contains(&i.columns) {
                return Err(CliError::input("板型行列范围无效：行 1–32、列 1–48"));
            }
            let standard = match i.format.as_str() {
                "6" => Some((2, 3)),
                "12" => Some((3, 4)),
                "24" => Some((4, 6)),
                "48" => Some((6, 8)),
                "96" => Some((8, 12)),
                "384" => Some((16, 24)),
                "custom" => None,
                _ => {
                    return Err(CliError::input(
                        "format 必须是 6、12、24、48、96、384 或 custom",
                    ))
                }
            };
            if standard.is_some_and(|(rows, columns)| (i.rows, i.columns) != (rows, columns)) {
                return Err(CliError::input(
                    "标准板型的 rows 和 columns 必须与 format 对应；自定义布局请使用 format=custom",
                ));
            }
            let tx = db
                .unchecked_transaction()
                .map_err(|e| CliError::internal(e.to_string()))?;
            let r = insert(
                &tx,
                "plate",
                project.as_deref(),
                Some(&i.run_id),
                1,
                &json!({"name":i.name.trim(),"format":i.format,"rows":i.rows,"columns":i.columns,"assay_id":i.assay_id.filter(|id| !id.is_empty()),"timepoint":i.timepoint.filter(|point| !point.trim().is_empty())}),
            )?;
            tx.commit().map_err(|e| CliError::internal(e.to_string()))?;
            Ok(json!({"record":r}))
        }
        "well.set" => {
            let i: WellSet = parse_input(path)?;
            let (_, project, _, _, raw) = kind_id(&db, &i.plate_id)?
                .filter(|r| r.0 == "plate")
                .ok_or_else(|| CliError::missing("板布局不存在"))?;
            let plate: Value =
                serde_json::from_str(&raw).map_err(|e| CliError::internal(e.to_string()))?;
            let rows = plate["rows"].as_u64().unwrap_or(0) as usize;
            let columns = plate["columns"].as_u64().unwrap_or(0) as usize;
            let position = parse_plate_position(rows, columns, &i.well_id)?;
            if let Some(target) = i.target.as_deref() {
                check_text(target, "检测目标", 120)?;
            }
            if let Some(factor) = i.dilution_factor {
                if !factor.is_finite() || factor <= 0.0 {
                    return Err(CliError::input("dilution_factor 必须是大于 0 的有限数值"));
                }
            }
            let mut existing: Vec<(String, i64, Value)> = Vec::new();
            {
                let mut stmt = db
                    .prepare("SELECT id,revision,data_json FROM records WHERE kind='well' AND parent_id=?1")
                    .map_err(|e| CliError::internal(e.to_string()))?;
                let mut rows_iter = stmt
                    .query(params![i.plate_id])
                    .map_err(|e| CliError::internal(e.to_string()))?;
                while let Some(row) = rows_iter
                    .next()
                    .map_err(|e| CliError::internal(e.to_string()))?
                {
                    let data_json: String = row.get(2).map_err(|e| CliError::internal(e.to_string()))?;
                    let data: Value = serde_json::from_str(&data_json)
                        .map_err(|e| CliError::internal(e.to_string()))?;
                    existing.push((
                        row.get(0).map_err(|e| CliError::internal(e.to_string()))?,
                        row.get(1).map_err(|e| CliError::internal(e.to_string()))?,
                        data,
                    ));
                }
            }
            // `Lane3` / `L3` / `A3` name one slot of a single-row container, so
            // the lookup compares parsed slots instead of raw position text.
            let old = existing.into_iter().find(|(_, _, data)| {
                well_slot_key(rows, columns, data["well_id"].as_str().unwrap_or_default())
                    == Some(position.slot_key)
            });
            let data = json!({
                "well_id": position.address,
                "group_label": i.group_label,
                "sample_id": i.sample_id,
                "note": i.note,
                "target": i.target.as_deref().map(str::trim).filter(|value| !value.is_empty()),
                "well_role": i.well_role,
                "dilution_factor": i.dilution_factor,
            });
            let r = if let Some((id, revision, _)) = old {
                let expected = i.expected_revision.ok_or_else(|| {
                    CliError::conflict("孔位已存在；修订时必须提供 expected_revision")
                })?;
                if expected != revision {
                    return Err(CliError::conflict(format!(
                        "孔位版本已从 v{expected} 更新为 v{revision}；请先重新读取。"
                    )));
                }
                update_data(&db, &id, expected, "well", &data)?
            } else {
                if i.expected_revision.is_some() {
                    return Err(CliError::conflict(
                        "孔位尚不存在；创建时不应提供 expected_revision",
                    ));
                }
                let tx = db
                    .unchecked_transaction()
                    .map_err(|e| CliError::internal(e.to_string()))?;
                let created = insert(&tx, "well", project.as_deref(), Some(&i.plate_id), 1, &data)?;
                tx.commit().map_err(|e| CliError::internal(e.to_string()))?;
                created
            };
            Ok(json!({"record":r}))
        }
        "note.create" => {
            let i: NoteCreate = parse_input(path)?;
            let (kind, project, _, _, _) =
                kind_id(&db, &i.run_id)?.ok_or_else(|| CliError::missing("实验运行不存在"))?;
            if kind != "run" {
                return Err(CliError::input("run_id 必须指向实验运行"));
            }
            check_text(&i.content, "记录内容", 10000)?;
            if let Some(time) = &i.event_time {
                if DateTime::parse_from_rfc3339(time).is_err() {
                    return Err(CliError::input(
                        "event_time 必须是带时区的 RFC 3339 时间，例如 2026-11-02T10:30:00+08:00",
                    ));
                }
            }
            let tx = db
                .unchecked_transaction()
                .map_err(|e| CliError::internal(e.to_string()))?;
            let r = insert(
                &tx,
                "note",
                project.as_deref(),
                Some(&i.run_id),
                1,
                &json!({"content":i.content,"event_time":i.event_time,"observation":i.observation,"result":i.result,"deviation":i.deviation}),
            )?;
            tx.commit().map_err(|e| CliError::internal(e.to_string()))?;
            Ok(json!({"record":r}))
        }
        "execution_event.create" => {
            let i: ExecutionEventCreate = parse_input(path)?;
            if let Some(item_id) = pantry_blanks_as_none(i.pantry_item_id.as_deref()) {
                check_text(&item_id, "储藏室条目 ID", 120)?;
                if pantry_read(&db, &item_id)?.is_none() {
                    return Err(CliError::missing(format!("储藏室条目不存在：{item_id}")));
                }
            }
            let payload =
                serde_json::to_value(&i).map_err(|e| CliError::internal(e.to_string()))?;
            let tx = db
                .unchecked_transaction()
                .map_err(|e| CliError::internal(e.to_string()))?;
            let r = create_execution_event_tx(&tx, &i.run_id, &payload)?;
            tx.commit().map_err(|e| CliError::internal(e.to_string()))?;
            let notice = pantry_event_notice(&db, i.pantry_item_id.as_deref())?;
            Ok(json!({
                "record": r,
                "pantry_notices": notice.into_iter().collect::<Vec<_>>(),
            }))
        }
        "pantry.list" => {
            let i: PantryQueryInput = parse_input(path)?;
            pantry_list(&db, &i)
        }
        "pantry.upsert" => {
            let i: PantryUpsertInput = parse_input(path)?;
            pantry_upsert(&mut db, &i)
        }
        "pantry.delete" => {
            let i: PantryDeleteInput = parse_input(path)?;
            pantry_delete(&mut db, &i)
        }
        "label.export" => {
            let i: PantryQueryInput = parse_input(path)?;
            pantry_export_labels(&db, &i)
        }
        "readout.submit" => {
            let input: Value = parse_input(path)?;
            submit_readout_input(&mut db, input)
        }
        "readout.batch" => {
            let input: Value = parse_input(path)?;
            let request = readout_batch_request_from_json(input)?;
            submit_readout_batch(&mut db, &request)
        }
        "formula.create" => {
            let i: FormulaCreate = parse_input(path)?;
            ensure_project(&db, &i.project_id)?;
            check_text(&i.title, "公式名称", 160)?;
            check_text(&i.expression, "公式表达式", 2000)?;
            let tx = db
                .unchecked_transaction()
                .map_err(|e| CliError::internal(e.to_string()))?;
            let r = insert(
                &tx,
                "formula",
                Some(&i.project_id),
                Some(&i.project_id),
                1,
                &json!({"title":i.title,"expression":i.expression,"variables":i.variables,"notes":i.notes}),
            )?;
            tx.commit().map_err(|e| CliError::internal(e.to_string()))?;
            Ok(json!({"record":r}))
        }
        "formula.revise" => {
            let i: FormulaRevise = parse_input(path)?;
            if i.title.is_none()
                && i.expression.is_none()
                && i.variables.is_none()
                && i.notes.is_none()
            {
                return Err(CliError::input("至少提供一个要修订的公式字段"));
            }
            let (_, _, _, revision, raw) =
                kind_id(&db, &i.id)?.ok_or_else(|| CliError::missing("公式不存在"))?;
            if revision != i.expected_revision {
                return Err(CliError::conflict(format!(
                    "公式版本已从 v{} 更新为 v{revision}；请先重新读取。",
                    i.expected_revision
                )));
            }
            let mut data: Value =
                serde_json::from_str(&raw).map_err(|e| CliError::internal(e.to_string()))?;
            if let Some(v) = i.title {
                check_text(&v, "公式名称", 160)?;
                data["title"] = json!(v.trim())
            }
            if let Some(v) = i.expression {
                check_text(&v, "公式表达式", 2000)?;
                data["expression"] = json!(v.trim())
            }
            if let Some(v) = i.variables {
                data["variables"] = json!(v)
            }
            if let Some(v) = i.notes {
                data["notes"] = json!(v)
            }
            Ok(json!({"record":update_data(&db,&i.id,i.expected_revision,"formula",&data)?}))
        }
        "table.create" => {
            let i: TableCreate = parse_input(path)?;
            ensure_project(&db, &i.project_id)?;
            check_text(&i.name, "资料表名称", 120)?;
            if i.columns.is_empty() || i.columns.len() > 40 || i.rows.len() > 5000 {
                return Err(CliError::input("资料表列数应为 1–40，行数最多 5000"));
            }
            if i.rows.iter().any(|r| r.len() != i.columns.len()) {
                return Err(CliError::input("每行单元格数必须与 columns 数量一致"));
            }
            let tx = db
                .unchecked_transaction()
                .map_err(|e| CliError::internal(e.to_string()))?;
            let r = insert(
                &tx,
                "library_table",
                Some(&i.project_id),
                Some(&i.project_id),
                1,
                &json!({"name":i.name,"columns":i.columns,"rows":i.rows}),
            )?;
            tx.commit().map_err(|e| CliError::internal(e.to_string()))?;
            Ok(json!({"record":r}))
        }
        "table.revise" => {
            let i: TableRevise = parse_input(path)?;
            check_text(&i.name, "资料表名称", 120)?;
            if i.columns.is_empty() || i.columns.len() > 40 || i.rows.len() > 5000 {
                return Err(CliError::input("资料表列数应为 1–40，行数最多 5000"));
            }
            if i.rows.iter().any(|r| r.len() != i.columns.len()) {
                return Err(CliError::input("每行单元格数必须与 columns 数量一致"));
            }
            let data = json!({"name":i.name,"columns":i.columns,"rows":i.rows});
            Ok(json!({"record":update_data(&db,&i.id,i.expected_revision,"library_table",&data)?}))
        }
        "record.list" => {
            let i: RecordList = parse_input(path)?;
            if let Some(k) = &i.kind {
                if ![
                    "project",
                    "design",
                    "design_version",
                    "run",
                    "task",
                    "protocol",
                    "protocol_version",
                    "protocol_block",
                    "protocol_block_version",
                    "plate",
                    "well",
                    "note",
                    "execution_event",
                    "formula",
                    "library_table",
                    "asset_link",
                    "asset_link_event",
                ]
                .contains(&k.as_str())
                {
                    return Err(CliError::input(
                        "不支持的 kind；请通过 schema record.list 查看允许值",
                    ));
                }
            }
            Ok(json!({"records":list(&db,i.kind.as_deref(),i.project_id.as_deref())?}))
        }
        "record.get" => {
            let i: IdInput = parse_input(path)?;
            let r = get(&db, &i.id)?;
            if r["kind"] == "agent_draft" {
                return Err(CliError::missing("该旧版草稿类型已停用"));
            }
            Ok(json!({"record":r}))
        }
        "asset.link" => {
            let i: AssetLinkCreate = parse_input(path)?;
            let (record, event) = link_local_asset(&mut db, &i)?;
            Ok(json!({"record":record,"event":event}))
        }
        "asset.unlink" => {
            let i: AssetUnlinkInput = parse_input(path)?;
            let (removed, event) =
                unlink_asset_link(&mut db, &i.id, i.actor_id.as_deref().unwrap_or(""))?;
            Ok(json!({"removed":removed,"event":event}))
        }
        "asset.link.update" => {
            let i: AssetLinkUpdateInput = parse_input(path)?;
            let (record, event) = update_asset_link(&mut db, &i)?;
            Ok(json!({"record":record,"event":event}))
        }
        "asset.list" => {
            let i: AssetListInput = parse_input(path)?;
            Ok(json!({"records":list_asset_links(&db,&i.run_id)?}))
        }
        "asset.events" => {
            let i: AssetEventsInput = parse_input(path)?;
            Ok(json!({"records":list_asset_events(&db,&i.run_id,i.action.as_deref())?}))
        }
        "import.study.apply" => {
            let i: StudyImport = parse_input(path)?;
            validate_import(&i)?;
            let tx = db
                .unchecked_transaction()
                .map_err(|e| CliError::internal(e.to_string()))?;
            let p = insert(
                &tx,
                "project",
                None,
                None,
                1,
                &json!({"name":i.project.name.trim(),"description":i.project.description.trim()}),
            )?;
            let mut designs = Vec::new();
            for d in &i.designs {
                designs.push(create_design_tx(
                    &tx,
                    p["id"].as_str().unwrap_or_default(),
                    d,
                    &default_labware_registry(),
                )?)
            }
            let mut protocols = Vec::new();
            for proto in &i.protocols {
                let pr = insert(
                    &tx,
                    "protocol",
                    Some(p["id"].as_str().unwrap_or_default()),
                    p["id"].as_str(),
                    1,
                    &json!({"title":proto.title.trim()}),
                )?;
                let ver = insert(
                    &tx,
                    "protocol_version",
                    Some(p["id"].as_str().unwrap_or_default()),
                    pr["id"].as_str(),
                    1,
                    &json!({"protocol_id":pr["id"],"version":1,"body":proto.body}),
                )?;
                protocols.push(json!({"protocol":pr,"version":ver}))
            }
            tx.commit().map_err(|e| CliError::internal(e.to_string()))?;
            Ok(json!({"writes":true,"project":p,"designs":designs,"protocols":protocols}))
        }
        _ => Err(CliError::missing(
            "未知工具；请运行 `labworkbench tools --json`",
        )),
    }
}
#[derive(Debug, Deserialize)]
#[serde(deny_unknown_fields)]
pub(crate) struct RecordList {
    #[serde(default)]
    pub(crate) kind: Option<String>,
    #[serde(default)]
    pub(crate) project_id: Option<String>,
}
pub(crate) fn validate_import(i: &StudyImport) -> Result<(), CliError> {
    check_text(&i.project.name, "项目名称", 120)?;
    for d in &i.designs {
        let mut normalized = d.clone();
        normalize_design(&mut normalized, None);
        validate_design(&normalized)?
    }
    for p in &i.protocols {
        check_text(&p.title, "Protocol 名称", 160)?
    }
    if i.designs.len() > 500 || i.protocols.len() > 500 {
        return Err(CliError::input("单次导入的设计或 Protocol 最多 500 项"));
    }
    Ok(())
}
pub(crate) fn split_well(value: &str) -> Result<(String, String), CliError> {
    let s = value.trim().to_ascii_uppercase();
    let letters: String = s.chars().take_while(|c| c.is_ascii_alphabetic()).collect();
    let digits: String = s.chars().skip_while(|c| c.is_ascii_alphabetic()).collect();
    if letters.is_empty()
        || digits.is_empty()
        || letters.len() > 2
        || !digits.chars().all(|c| c.is_ascii_digit())
    {
        return Err(CliError::input("孔位格式应为字母行号加数字列号，例如 B4"));
    }
    Ok((letters, digits))
}

/// One addressable plate slot, resolved from user input.
///
/// Mirrors the EXP-11 1D rules of `asterlab_core::labware`: a single-row
/// container also accepts `Lane<N>` / `L<N>` / `Tube<N>`, and every alias of one
/// slot resolves to the same `slot_key` so a lane can never be written twice.
pub(crate) struct PlatePosition {
    /// Canonical stored address: `A1` for 2D wells, `Lane<N>` for a 1D slot.
    pub(crate) address: String,
    /// Parsed `(row, column)` used for duplicate detection.
    pub(crate) slot_key: (usize, usize),
}

pub(crate) fn parse_plate_position(
    rows: usize,
    columns: usize,
    raw: &str,
) -> Result<PlatePosition, CliError> {
    let text = raw.trim();
    if rows == 0 || columns == 0 {
        return Err(CliError::internal("板布局缺少行列数"));
    }
    if rows == 1 {
        if let Some(index) = parse_lane_index(text) {
            if index == 0 || index > columns {
                return Err(CliError::input("泳道超出板型范围"));
            }
            return Ok(PlatePosition {
                address: format!("Lane{index}"),
                slot_key: (1, index),
            });
        }
    }
    let (letters, digits) = split_well(text)?;
    let row = letters.chars().fold(0usize, |n, c| {
        n * 26 + (c.to_ascii_uppercase() as usize - 'A' as usize + 1)
    });
    let column = digits
        .parse::<usize>()
        .map_err(|_| CliError::input("孔位列号无效"))?;
    if row == 0 || row > rows || column == 0 || column > columns {
        return Err(CliError::input("孔位超出板型范围"));
    }
    Ok(PlatePosition {
        address: format!("{letters}{column}"),
        slot_key: (row, column),
    })
}

/// 1-based lane index for `Lane<N>` / `L<N>` / `Tube<N>`, matched
/// case-insensitively. `Lane` is tried before `L`, otherwise `Lane4` would
/// parse as a lane named `ane4`.
pub(crate) fn parse_lane_index(value: &str) -> Option<usize> {
    let lower = value.to_ascii_lowercase();
    ["lane", "tube", "l"]
        .into_iter()
        .find_map(|prefix| lower.strip_prefix(prefix))
        .filter(|digits| {
            !digits.is_empty()
                && digits.len() <= 2
                && digits.bytes().all(|byte| byte.is_ascii_digit())
        })
        .and_then(|digits| digits.parse().ok())
}

/// Duplicate-detection key for a stored position; `None` for legacy text that
/// does not parse, which can never collide with a newly validated position.
pub(crate) fn well_slot_key(rows: usize, columns: usize, stored: &str) -> Option<(usize, usize)> {
    parse_plate_position(rows, columns, stored)
        .ok()
        .map(|position| position.slot_key)
}

pub(crate) fn design_payload_schema() -> Value {
    design_schema()
}
pub(crate) fn asset_link_payload_schema() -> Value {
    json!({
        "type":"object","additionalProperties":false,
        "required":["id","rootId","relativePath","mediaType","size","modTime","sha256","availability","link","linkedAt","actorId"],
        "properties":{
            "id":{"type":"string"},
            "rootId":{"type":"string"},
            "relativePath":{"type":"string"},
            "mediaType":{"type":"string","enum":["image","table","document","binary","unknown"]},
            "size":{"type":"integer","minimum":0},
            "modTime":{"type":"string"},
            "sha256":{"type":"string","maxLength":64},
            "availability":{"type":"string","enum":["available","unavailable"]},
            "link":{"type":"object","additionalProperties":false,"required":["runId","taskKey","sampleId","wellId","timepoint","fovIndex","coordinateId"],"properties":{"runId":{"type":"string"},"taskKey":{"type":"string"},"sampleId":{"type":["string","null"]},"wellId":{"type":["string","null"]},"timepoint":{"type":["string","null"]},"fovIndex":{"type":["integer","null"],"minimum":1},"coordinateId":{"type":["string","null"]}}},
            "linkedAt":{"type":"string"},
            "actorId":{"type":"string"}
        }
    })
}
pub(crate) fn asset_link_event_payload_schema() -> Value {
    json!({
        "type":"object","additionalProperties":false,
        "required":["id","action","assetId","rootId","relativePath","mediaType","size","modTime","sha256","availability","link","at","actorId"],
        "properties":{
            "id":{"type":"string"},
            "action":{"type":"string","enum":["linked","unlinked","updated"]},
            "assetId":{"type":"string"},
            "rootId":{"type":"string"},
            "relativePath":{"type":"string"},
            "mediaType":{"type":"string","enum":["image","table","document","binary","unknown"]},
            "size":{"type":"integer","minimum":0},
            "modTime":{"type":"string"},
            "sha256":{"type":"string","maxLength":64},
            "availability":{"type":"string","enum":["available","unavailable"]},
            "link":{"type":"object","additionalProperties":false,"required":["runId","taskKey","sampleId","wellId","timepoint","fovIndex","coordinateId"],"properties":{"runId":{"type":"string"},"taskKey":{"type":"string"},"sampleId":{"type":["string","null"]},"wellId":{"type":["string","null"]},"timepoint":{"type":["string","null"]},"fovIndex":{"type":["integer","null"],"minimum":1},"coordinateId":{"type":["string","null"]}}},
            "previous":{"type":"object","additionalProperties":false,"required":["wellId","timepoint","fovIndex","coordinateId"],"properties":{"wellId":{"type":["string","null"]},"timepoint":{"type":["string","null"]},"fovIndex":{"type":["integer","null"]},"coordinateId":{"type":["string","null"]}}},
            "at":{"type":"string"},
            "actorId":{"type":"string"}
        }
    })
}
pub(crate) fn record_data_schema(kind: &str) -> Value {
    match kind {
        "project" => {
            json!({"type":"object","additionalProperties":false,"required":["name","description"],"properties":{"name":{"type":"string"},"description":{"type":"string"}}})
        }
        "design" => design_payload_schema(),
        "design_version" => {
            json!({"type":"object","additionalProperties":false,"required":["design_id","design_revision","snapshot"],"properties":{"design_id":{"type":"string"},"design_revision":{"type":"integer","minimum":1},"snapshot":design_payload_schema()}})
        }
        "run" => {
            json!({"type":"object","additionalProperties":false,"required":["title","scheduled_date","protocol_version_id","design_version_id","design_revision","status"],"properties":{"title":{"type":"string"},"scheduled_date":{"type":["string","null"],"format":"date"},"protocol_version_id":{"type":["string","null"]},"design_version_id":{"type":"string"},"design_revision":{"type":"integer","minimum":1},"status":{"type":"string","enum":["planned","in_progress","completed","blocked","cancelled"]},"run_index":{"type":"integer","minimum":1},"repeat_of_run_id":{"type":["string","null"]},"repeat_relationship":{"type":"string","enum":["independent","same_source","paired","unknown"]},"source_run_id":{"type":["string","null"]},"container_plan":{"type":"object","additionalProperties":true},"labware_profiles":{"type":"array","items":{"type":"object","additionalProperties":true}},"material_manifest":{"type":"array","items":{"type":"object","additionalProperties":false,"required":["planned_output_id","title","evidence_id","producer_protocol","consumers"],"properties":{"planned_output_id":{"type":"string"},"title":{"type":"string"},"evidence_id":{"type":["string","null"]},"producer_protocol":{"type":"object","additionalProperties":false,"required":["assay_id","assay_title","protocol_version_id","protocol_id","protocol_title","protocol_version","root_block_version_id","block_version_ids","root_bindings","protocol_ready","steps","resolved_parameters","unresolved_parameters"],"properties":{"module_contracts":{"type":"array","items":{"type":"object","additionalProperties":true}},"assay_id":{"type":["string","null"]},"assay_title":{"type":"string"},"protocol_version_id":{"type":"string"},"protocol_id":{"type":"string"},"protocol_title":{"type":"string"},"protocol_version":{"type":"integer","minimum":1},"root_block_version_id":{"type":["string","null"]},"block_version_ids":{"type":"array","items":{"type":"string"}},"root_bindings":{"type":"array","items":{"type":"object","additionalProperties":false,"required":["slot","value","unit"],"properties":{"slot":{"type":"string"},"value":{},"unit":{"type":["string","null"]}}}},"protocol_ready":{"type":"boolean"},"steps":{"type":"array","items":{"type":"object"}},"resolved_parameters":{"type":"array","items":{"type":"object"}},"unresolved_parameters":{"type":"array","items":{"type":"object"}}}},"consumers":{"type":"array","items":{"type":"object","additionalProperties":false,"required":["assay_id","assay_title","group_id","group_name"],"properties":{"assay_id":{"type":"string"},"assay_title":{"type":"string"},"group_id":{"type":["string","null"]},"group_name":{"type":"string"}}}}}}},"protocol_manifest":{"type":"array","items":{"type":"object","additionalProperties":false,"required":["assay_id","assay_title","protocol_version_id","protocol_id","protocol_title","protocol_version"],"properties":{"module_contracts":{"type":"array","items":{"type":"object","additionalProperties":true}},"assay_id":{"type":["string","null"]},"assay_title":{"type":"string"},"protocol_version_id":{"type":"string"},"protocol_id":{"type":"string"},"protocol_title":{"type":"string"},"protocol_version":{"type":"integer","minimum":1},"root_block_version_id":{"type":["string","null"]},"block_version_ids":{"type":"array","items":{"type":"string"}},"root_bindings":{"type":"array","items":{"type":"object","additionalProperties":false,"required":["slot","value","unit"],"properties":{"slot":{"type":"string"},"value":{},"unit":{"type":["string","null"]}}}},"protocol_ready":{"type":"boolean"},"steps":{"type":"array","items":{"type":"object"}},"resolved_parameters":{"type":"array","items":{"type":"object"}},"unresolved_parameters":{"type":"array","items":{"type":"object","additionalProperties":false,"required":["path","slot","label","unit","required"],"properties":{"path":{"type":"string"},"slot":{"type":"string"},"label":{"type":"string"},"unit":{"type":["string","null"]},"required":{"type":"boolean"}}}}}}}}})
        }
        "task" => {
            json!({"type":"object","additionalProperties":false,"required":["title","scheduled_date","status","notes"],"properties":{"title":{"type":"string"},"scheduled_date":{"type":"string","format":"date"},"status":{"type":"string","enum":["planned","in_progress","done","blocked"]},"notes":{"type":"string"}}})
        }
        "protocol" => {
            json!({"type":"object","additionalProperties":false,"required":["title"],"properties":{"title":{"type":"string"}}})
        }
        "protocol_version" => {
            json!({"type":"object","additionalProperties":false,"required":["protocol_id","version","body"],"properties":{"protocol_id":{"type":"string"},"version":{"type":"integer","minimum":1},"body":{"type":"string"},"root_block_version_id":{"type":["string","null"]},"root_bindings":{"type":"array","items":{"type":"object","additionalProperties":false,"required":["slot","value"],"properties":{"slot":{"type":"string"},"value":{},"unit":{"type":["string","null"]}}}},"agent_edit":{"type":["object","null"],"additionalProperties":true}}})
        }
        "protocol_block" => {
            json!({"type":"object","additionalProperties":false,"required":["title","summary"],"properties":{"title":{"type":"string"},"summary":{"type":"string"}}})
        }
        "catalog_review" => {
            json!({"type":"object","additionalProperties":false,"required":["review_id","subject","status","reviewer","reviewed_at","evidence_id","evidence_locator","content_digest"],"properties":{"review_id":{"type":"string"},"subject":{"type":"object","additionalProperties":false,"required":["module_id","version"],"properties":{"module_id":{"type":"string"},"version":{"type":"integer","minimum":1}}},"status":{"type":"string","enum":["research_draft","source_checked","expert_reviewed","lab_validated"]},"reviewer":{"type":"string"},"reviewed_at":{"type":"string"},"evidence_id":{"type":"string"},"evidence_locator":{"type":"string"},"content_digest":{"type":"string"}}})
        }
        "labware_profile" => {
            json!({"type":"object","additionalProperties":true})
        }
        "component_definition" => {
            json!({"type":"object","additionalProperties":true})
        }
        "view" => {
            json!({"type":"object","additionalProperties":false,"required":["title"],"properties":{"title":{"type":"string"}}})
        }
        "view_document" => {
            json!({"type":"object","additionalProperties":true})
        }
        "protocol_block_version" => {
            json!({"type":"object","additionalProperties":false,"required":["block_id","version","summary","parameter_slots","steps"],"properties":{"block_id":{"type":"string"},"version":{"type":"integer","minimum":1},"summary":{"type":"string"},"module_contract":{"type":["object","null"]},"module_origin":{"type":["string","null"],"enum":["imported","derived","local_custom",null]},"module_contract_authority":{"type":["string","null"],"enum":["current","source_only",null]},"catalog_review":{"type":["object","null"]},"based_on_version_id":{"type":["string","null"]},"step_edit":{"type":["object","null"]},"parameter_slots":{"type":"array","items":{"type":"object","additionalProperties":false,"required":["key","label","value_type","unit","required"],"properties":{"key":{"type":"string"},"label":{"type":"string"},"value_type":{"type":"string","enum":["text","number","integer","boolean"]},"unit":{"type":["string","null"]},"required":{"type":"boolean"}}}},"steps":{"type":"array","items":{"oneOf":[{"type":"object","additionalProperties":false,"required":["kind","use_id","title","instruction","parameter_refs"],"properties":{"kind":{"const":"text"},"use_id":{"type":"string"},"title":{"type":"string"},"instruction":{"type":"string"},"parameter_refs":{"type":"array","items":{"type":"string"}},"visualization":{"type":"string","enum":["transfer","incubation","plate_layout","sample_split","measurement"]},"time_scope":{"type":"string","enum":["once","per_timepoint","per_assay","unresolved"]},"checks":{"type":"array","items":{"type":"object","additionalProperties":true}}}},{"type":"object","additionalProperties":false,"required":["kind","use_id","block_version_id","bindings"],"properties":{"kind":{"const":"block_use"},"use_id":{"type":"string"},"block_version_id":{"type":"string"},"bindings":{"type":"array","items":{"type":"object","additionalProperties":false,"required":["slot","value","source_slot","unit"],"properties":{"slot":{"type":"string"},"value":{},"source_slot":{"type":["string","null"]},"unit":{"type":["string","null"]}}}}}}]}}}})
        }
        "plate" => {
            json!({"type":"object","additionalProperties":false,"required":["name","format","rows","columns"],"properties":{"name":{"type":"string"},"format":{"type":"string"},"rows":{"type":"integer","minimum":1,"maximum":32},"columns":{"type":"integer","minimum":1,"maximum":48},"assay_id":{"type":["string","null"]},"timepoint":{"type":["string","null"]}}})
        }
        "well" => {
            json!({"type":"object","additionalProperties":false,"required":["well_id","group_label","sample_id","note"],"properties":{"well_id":{"type":"string"},"group_label":{"type":"string"},"sample_id":{"type":"string"},"note":{"type":"string"},"target":{"type":["string","null"]},"well_role":{"type":["string","null"],"enum":["unknown","ntc","standard","positive_control",null]},"dilution_factor":{"type":["number","null"],"exclusiveMinimum":0}}})
        }
        "note" => {
            json!({"type":"object","additionalProperties":false,"required":["content","event_time","observation","result","deviation"],"properties":{"content":{"type":"string"},"event_time":{"type":["string","null"],"format":"date-time"},"event_time_local":{"type":["string","null"]},"event_timezone":{"type":["string","null"]},"observation":{"type":"string"},"result":{"type":"string"},"deviation":{"type":"string"}}})
        }
        "execution_event" => {
            json!({"type":"object","additionalProperties":false,"required":["event_type","content","occurred_at","stage","timepoint","step_status","sample_ids","materials","measurements","deviation"],"properties":{"event_type":{"type":"string","enum":["operation","material_use","measurement","deviation","observation"]},"content":{"type":"string"},"occurred_at":{"type":["string","null"],"format":"date-time"},"stage":{"type":["string","null"]},"timepoint":{"type":["string","null"],"maxLength":100},"step_status":{"type":["string","null"],"enum":["in_progress","completed","blocked","skipped","qc_dropped","censored_dead","rework",null]},"parent_sample_id":{"type":["string","null"],"maxLength":120},"aliquot_from":{"type":["string","null"],"maxLength":120},"blind_masked":{"type":["boolean","null"]},"protocol_step_path":{"type":["string","null"]},"protocol_version_id":{"type":["string","null"]},"assay_id":{"type":["string","null"]},"planned_output_id":{"type":["string","null"]},"pantry_item_id":{"type":["string","null"]},"sample_ids":{"type":"array","items":{"type":"string"}},"materials":{"type":"array","items":{"type":"object","additionalProperties":false,"required":["material","planned_output_id","amount","unit"],"properties":{"material":{"type":"string"},"planned_output_id":{"type":["string","null"]},"amount":{"type":["number","null"]},"unit":{"type":["string","null"]}}}},"measurements":{"type":"array","items":{"type":"object","additionalProperties":false,"required":["endpoint","value","unit","sample_id"],"properties":{"endpoint":{"type":"string"},"value":{"type":"number"},"unit":{"type":["string","null"]},"sample_id":{"type":["string","null"]}}}},"readouts":{"type":"array","items":{"type":"object","additionalProperties":true}},"checks":{"type":"array","items":{"type":"object","additionalProperties":true}},"controls":{"type":"array","items":{"type":"object","additionalProperties":true}},"lineage":{"type":"array","items":{"type":"object","additionalProperties":true}},"actual_parameters":{"type":"array","items":{"type":"object","additionalProperties":true}},"operator":{"type":["string","null"]},"deviation":{"type":["string","null"]}}})
        }
        "formula" => {
            json!({"type":"object","additionalProperties":false,"required":["title","expression","variables","notes"],"properties":{"title":{"type":"string"},"expression":{"type":"string"},"variables":{"type":"string"},"notes":{"type":"string"}}})
        }
        "library_table" => {
            json!({"type":"object","additionalProperties":false,"required":["name","columns","rows"],"properties":{"name":{"type":"string"},"columns":{"type":"array","items":{"type":"string"}},"rows":{"type":"array","items":{"type":"array","items":{"type":"string"}}}}})
        }
        "asset_link" => asset_link_payload_schema(),
        "asset_link_event" => asset_link_event_payload_schema(),
        _ => json!({"type":"object","additionalProperties":false}),
    }
}
pub(crate) fn record_schema(kind: &str) -> Value {
    let mut data = record_data_schema(kind);
    if kind == "execution_event" {
        data["properties"]["event_type"]["enum"]
            .as_array_mut()
            .expect("execution_event event_type enum")
            .push(json!("skipped_by_rule"));
        data["properties"]["ad_hoc_step"] = json!({"type":"boolean"});
    }
    data["additionalProperties"] = json!(false);
    json!({"type":"object","additionalProperties":false,"required":["id","kind","project_id","parent_id","revision","data","created_at","updated_at"],"properties":{"id":{"type":"string"},"kind":{"const":kind},"project_id":{"type":["string","null"]},"parent_id":{"type":["string","null"]},"revision":{"type":"integer","minimum":1},"data":data,"created_at":{"type":"string","format":"date-time"},"updated_at":{"type":"string","format":"date-time"}}})
}
pub(crate) fn any_record_schema() -> Value {
    let kinds = [
        "project",
        "design",
        "design_version",
        "run",
        "task",
        "protocol",
        "protocol_version",
        "protocol_block",
        "protocol_block_version",
        "labware_profile",
        "component_definition",
        "view",
        "view_document",
        "catalog_review",
        "plate",
        "well",
        "note",
        "execution_event",
        "formula",
        "library_table",
        "asset_link",
        "asset_link_event",
    ];
    json!({"oneOf":kinds.iter().map(|k|record_schema(k)).collect::<Vec<_>>()})
}
pub(crate) fn workflow_progress_output_schema() -> Value {
    let status = json!({"type":"string","enum":["not_started","active","in_progress","completed","failed","skipped","skipped_by_rule","qc_dropped","censored_dead","rework"]});
    let attempt = json!({
        "type":"object","additionalProperties":false,
        "required":["attempt","status","event_ids","last_event_at","deviation"],
        "properties":{
            "attempt":{"type":"integer","minimum":1},
            "status":status.clone(),
            "event_ids":{"type":"array","items":{"type":"string"}},
            "last_event_at":{"type":["string","null"]},
            "deviation":{"type":"boolean"}
        }
    });
    let sample = json!({
        "type":"object","additionalProperties":false,
        "required":["sample_id","status","attempt","attempt_count","last_event_at","attempts"],
        "properties":{
            "sample_id":{"type":"string"},
            "status":status.clone(),
            "attempt":{"type":["integer","null"],"minimum":1},
            "attempt_count":{"type":"integer","minimum":0},
            "last_event_at":{"type":["string","null"]},
            "parent_sample_id":{"type":["string","null"]},
            "aliquot_from":{"type":["string","null"]},
            "blind_masked":{"type":"boolean"},
            "attempts":{"type":"array","items":attempt}
        }
    });
    json!({
        "type":"object","additionalProperties":false,
        "required":["schema_version","run_id","sample_ids","steps","issues"],
        "properties":{
            "schema_version":{"type":"integer","minimum":1},
            "run_id":{"type":"string"},
            "sample_ids":{"type":"array","items":{"type":"string"}},
            "steps":{"type":"array","items":{
                "type":"object","additionalProperties":false,
                "required":["step_path","timepoint","status","complete","total_samples","completed_samples","failed_samples","skipped_samples","in_progress_samples","not_started_samples","last_event_at","samples"],
                "properties":{
                    "step_path":{"type":"string"},
                    "timepoint":{"type":["string","null"]},
                    "status":status,
                    "complete":{"type":"boolean"},
                    "total_samples":{"type":"integer","minimum":0},
                    "completed_samples":{"type":"integer","minimum":0},
                    "failed_samples":{"type":"integer","minimum":0},
                    "skipped_samples":{"type":"integer","minimum":0},
                    "active_samples":{"type":"integer","minimum":0},
                    "skipped_by_rule_samples":{"type":"integer","minimum":0},
                    "qc_dropped_samples":{"type":"integer","minimum":0},
                    "censored_dead_samples":{"type":"integer","minimum":0},
                    "rework_samples":{"type":"integer","minimum":0},
                    "in_progress_samples":{"type":"integer","minimum":0},
                    "not_started_samples":{"type":"integer","minimum":0},
                    "last_event_at":{"type":["string","null"]},
                    "samples":{"type":"array","items":sample}
                }
            }},
            "issues":{"type":"array","items":{
                "type":"object","additionalProperties":false,
                "required":["code","message","step_path","sample_id","event_id"],
                "properties":{
                    "code":{"type":"string"},
                    "message":{"type":"string"},
                    "step_path":{"type":["string","null"]},
                    "sample_id":{"type":["string","null"]},
                    "event_id":{"type":["string","null"]}
                }
            }}
        }
    })
}
pub(crate) fn result_schema(tool: &str) -> Value {
    let arr = |kind: &str| json!({"type":"object","additionalProperties":false,"required":["records"],"properties":{"records":{"type":"array","items":record_schema(kind)}}});
    let asterdoc_version = || json!({"type":"object","additionalProperties":true,"required":["design_id","revision","source","source_hash","schema_version","compiler_version"],"properties":{"design_id":{"type":"string"},"revision":{"type":"integer","minimum":0},"source":{"type":"string"},"source_hash":{"type":"string"},"schema_version":{"type":"integer","minimum":1},"compiler_version":{"type":"string"},"design_revision":{"type":"integer","minimum":0},"operation_id":{"type":["string","null"]},"actor_id":{"type":["string","null"]},"created_at":{"type":["string","null"]}}});
    let one = |kind: &str| json!({"type":"object","additionalProperties":false,"required":["record"],"properties":{"record":record_schema(kind)}});
    let record_and_version = |record_kind: &str| json!({"type":"object","additionalProperties":false,"required":["record","version"],"properties":{"record":record_schema(record_kind),"version":record_schema("design_version")}});
    let pantry_status = || json!({"type":"string","enum":["unopened","opened","near_expiry","expired","exhausted"]});
    let pantry_item = || json!({"type":"object","additionalProperties":false,"required":["id","label_code","project_id","name","location","purchased_on","opened_on","shelf_life_days","opened_shelf_life_days","exhausted","revision","status","expires_on","days_remaining","expiry_basis","created_at","updated_at"],"properties":{"id":{"type":"string"},"label_code":{"type":"string"},"project_id":{"type":["string","null"]},"name":{"type":"string"},"location":{"type":"string"},"purchased_on":{"type":["string","null"],"format":"date"},"opened_on":{"type":["string","null"],"format":"date"},"shelf_life_days":{"type":["integer","null"],"minimum":1},"opened_shelf_life_days":{"type":["integer","null"],"minimum":1},"exhausted":{"type":"boolean"},"revision":{"type":"integer","minimum":1},"status":pantry_status(),"expires_on":{"type":["string","null"],"format":"date"},"days_remaining":{"type":["integer","null"]},"expiry_basis":{"type":["string","null"],"enum":["shelf_life","opened_shelf_life",null]},"created_at":{"type":"string"},"updated_at":{"type":"string"}}});
    let pantry_notice = || json!({"type":"object","additionalProperties":false,"required":["code","severity","pantry_item_id","label_code","name","status","expires_on","days_remaining","next_action"],"properties":{"code":{"const":"pantry_item_expiry_notice"},"severity":{"const":"advisory"},"pantry_item_id":{"type":"string"},"label_code":{"type":"string"},"name":{"type":"string"},"status":{"type":"string","enum":["near_expiry","expired"]},"expires_on":{"type":["string","null"],"format":"date"},"days_remaining":{"type":["integer","null"]},"next_action":{"type":"string"}}});
    let pantry_label = || json!({"type":"object","additionalProperties":false,"required":["label_code","pantry_item_id","name","location","status","expires_on","days_remaining","revision"],"properties":{"label_code":{"type":"string"},"pantry_item_id":{"type":"string"},"name":{"type":"string"},"location":{"type":"string"},"status":pantry_status(),"expires_on":{"type":["string","null"],"format":"date"},"days_remaining":{"type":["integer","null"]},"revision":{"type":"integer","minimum":1}}});
    match tool {
        "project.list" => arr("project"),
        "project.create" => one("project"),
        "project.revise" => one("project"),
        "design.list" => arr("design"),
        "design.get" | "design.create" => {
            json!({"type":"object","additionalProperties":false,"required":["design","version"],"properties":{"design":record_schema("design"),"version":record_schema("design_version")}})
        }
        "design.map" => {
            json!({"type":"object","additionalProperties":false,"required":["design_id","revision","title","objective","research_question","factors","experimental_unit","groups","assays","sample_links","planned_outputs","conflicts","unresolved"],"properties":{"design_id":{"type":"string"},"revision":{"type":"integer","minimum":1},"title":{"type":"string"},"objective":{"type":"string"},"research_question":{"type":["string","null"]},"factors":{"type":"array","items":{"type":"object","additionalProperties":false,"required":["name","levels"],"properties":{"name":{"type":"string"},"levels":{"type":"array","items":{"type":"string"}}}}},"experimental_unit":{"type":["string","null"]},"groups":{"type":"array","items":{"type":"object","additionalProperties":true}},"assays":{"type":"array","items":{"type":"object","additionalProperties":true}},"sample_links":{"type":"array","items":{"type":"object","additionalProperties":true}},"planned_outputs":{"type":"array","items":{"type":"object","additionalProperties":true}},"conflicts":{"type":"array","items":{"type":"object","additionalProperties":true}},"unresolved":{"type":"array","items":{"type":"object","additionalProperties":true}}}})
        }
        "design.revise" => record_and_version("design"),
        "run.list" => arr("run"),
        "run.create" => one("run"),
        "protocol.version.get" | "protocol.version.list" => {
            let item = json!({
                "type":"object","additionalProperties":false,
                "required":["protocol_version_id","protocol_id","version","root_block_version_id","root_bindings","parameter_slots","steps","resolved_parameters","unresolved_parameters","block_version_ids","frozen"],
                "properties":{
                    "protocol_version_id":{"type":"string"},"protocol_id":{"type":"string"},
                    "version":{"type":"integer"},"root_block_version_id":{"type":["string","null"]},
                    "root_bindings":{"type":"array","items":{"type":"object"}},
                    "parameter_slots":{"type":"array","items":{"type":"object"}},
                    "steps":{"type":"array","items":{"type":"object"}},
                    "resolved_parameters":{"type":"array","items":{"type":"object"}},
                    "unresolved_parameters":{"type":"array","items":{"type":"object"}},
                    "block_version_ids":{"type":"array","items":{"type":"string"}},
                    "frozen":{"const":true}
                }
            });
            if tool.ends_with(".list") {
                json!({"type":"object","additionalProperties":false,"required":["versions"],"properties":{"versions":{"type":"array","items":item}}})
            } else { item }
        }
        "run.set_status" => one("run"),
        "run.revise" => one("run"),
        "run.workspace" => {
            json!({"type":"object","additionalProperties":false,"required":["run_id","design_id","design_version_id","design_revision","frozen_design","protocol_version_id","status","scopes","steps","tasks","occurrences","materials","plates","linked_assets","asset_coverage","events"],"properties":{"run_id":{"type":"string"},"design_id":{"type":"string"},"design_version_id":{"type":"string"},"design_revision":{"type":"integer","minimum":1},"frozen_design":{"type":"object","additionalProperties":true},"protocol_version_id":{"type":["string","null"]},"status":{"type":"string"},"scopes":{"type":"array","items":{"type":"object","additionalProperties":true}},"steps":{"type":"array","items":{"type":"object","additionalProperties":true}},"tasks":{"type":"array","items":{"type":"object","additionalProperties":true}},"occurrences":{"type":"array","items":{"type":"object","additionalProperties":true}},"materials":{"type":"array","items":{"type":"object","additionalProperties":true}},"plates":{"type":"array","items":{"type":"object","additionalProperties":true}},"linked_assets":{"type":"array","items":{"type":"object","additionalProperties":false,"required":["asset_id","task_key","sample_id","well_id","timepoint","fov_index","coordinate_id"],"properties":{"asset_id":{"type":"string"},"task_key":{"type":"string"},"sample_id":{"type":["string","null"]},"well_id":{"type":["string","null"]},"timepoint":{"type":["string","null"]},"fov_index":{"type":["integer","null"],"minimum":1},"coordinate_id":{"type":["string","null"]}}}},"asset_coverage":{"type":"array","items":{"type":"object","additionalProperties":false,"required":["well_id","timepoint","fov_indices","asset_ids"],"properties":{"well_id":{"type":"string"},"timepoint":{"type":["string","null"]},"fov_indices":{"type":"array","items":{"type":"integer","minimum":1}},"asset_ids":{"type":"array","items":{"type":"string"}}}}},"events":{"type":"array","items":{"type":"object","additionalProperties":true}}}})
        }
        "workflow.inspect_tree" => json!({"type":"object","additionalProperties":false,"required":["designId","revision","design","roots","order","precedes","issues"],"properties":{"designId":{"type":"string"},"revision":{"type":"integer","minimum":0},"runId":{"type":"string"},"design":{"type":"boolean"},"roots":{"type":"array","items":{"type":"object","additionalProperties":true}},"order":{"type":"array","items":{"type":"string"}},"precedes":{"type":"object","additionalProperties":{"type":"array","items":{"type":"string"}}},"issues":{"type":"array","items":{"type":"string"}}}}),
        "workflow.progress" => workflow_progress_output_schema(),
        "task.create" => one("task"),
        "task.set_status" => one("task"),
        "protocol.list" => arr("protocol"),
        "protocol.block.list" => arr("protocol_block"),
        "protocol.block.get" => {
            json!({"type":"object","additionalProperties":false,"required":["block","latest_version","versions"],"properties":{"block":record_schema("protocol_block"),"latest_version":record_schema("protocol_block_version"),"versions":{"type":"array","items":record_schema("protocol_block_version")}}})
        }
        "catalog.review.record" => one("catalog_review"),
        "protocol.block.create" | "catalog.operation.import" => {
            json!({"type":"object","additionalProperties":false,"required":["block","version"],"properties":{"block":record_schema("protocol_block"),"version":record_schema("protocol_block_version")}})
        }
        "protocol.block.revise" => {
            json!({"type":"object","additionalProperties":false,"required":["version"],"properties":{"version":record_schema("protocol_block_version")}})
        }
        "protocol.block.preview" => {
            json!({"type":"object","additionalProperties":false,"required":["block_version_id","block_id","title","version","ready","steps","resolved_parameters","unresolved_parameters","block_version_ids"],"properties":{"module_contracts":{"type":"array","items":{"type":"object","additionalProperties":true}},"block_version_id":{"type":"string"},"block_id":{"type":"string"},"title":{"type":"string"},"version":{"type":"integer"},"ready":{"type":"boolean"},"steps":{"type":"array","items":{"type":"object","additionalProperties":false,"required":["path","use_id","title","instruction","parameters","unresolved_parameter_refs","source_block_version_id"],"properties":{"path":{"type":"string"},"use_id":{"type":"string"},"title":{"type":"string"},"instruction":{"type":"string"},"parameters":{"type":"array","items":{"type":"object","additionalProperties":false,"required":["slot","label","value","unit","provenance"],"properties":{"slot":{"type":"string"},"label":{"type":"string"},"value":{},"unit":{"type":["string","null"]},"provenance":{"type":"array","items":{"type":"object"}}}}},"unresolved_parameter_refs":{"type":"array","items":{"type":"string"}},"source_block_version_id":{"type":"string"},"visualization":{"type":"string","enum":["transfer","incubation","plate_layout","sample_split","measurement"]},"time_scope":{"type":"string","enum":["once","per_timepoint","per_assay","unresolved"]},"checks":{"type":"array","items":{"type":"object","additionalProperties":true}}}}},"resolved_parameters":{"type":"array","items":{"type":"object","additionalProperties":false,"required":["path","slot","label","value","unit","provenance"],"properties":{"path":{"type":"string"},"slot":{"type":"string"},"label":{"type":"string"},"value":{},"unit":{"type":["string","null"]},"provenance":{"type":"array","items":{"type":"object"}}}}},"unresolved_parameters":{"type":"array","items":{"type":"object","additionalProperties":false,"required":["path","slot","label","unit","required"],"properties":{"path":{"type":"string"},"slot":{"type":"string"},"label":{"type":"string"},"unit":{"type":["string","null"]},"required":{"type":"boolean"}}}},"block_version_ids":{"type":"array","items":{"type":"string"}}}})
        }
        "protocol.create" => {
            json!({"type":"object","additionalProperties":false,"required":["protocol","version"],"properties":{"protocol":record_schema("protocol"),"version":record_schema("protocol_version")}})
        }
        "protocol.revise" => {
            json!({"type":"object","additionalProperties":false,"required":["version"],"properties":{"version":record_schema("protocol_version")}})
        }
        "agent_edit.validate" => {
            json!({"type":"object","additionalProperties":false,"required":["schema_version","proposal_id","proposal_digest","target","current_version_id","current_revision","status","diffs","issues","invalidated_refs","requires_confirmation"],"properties":{"schema_version":{"type":"integer"},"proposal_id":{"type":"string"},"proposal_digest":{"type":"string"},"target":{"type":"object","additionalProperties":true},"current_version_id":{"type":"string"},"current_revision":{"type":"integer"},"status":{"type":"string","enum":["valid","needs_confirmation","blocked"]},"diffs":{"type":"array","items":{"type":"object","additionalProperties":true}},"issues":{"type":"array","items":{"type":"object","additionalProperties":true}},"invalidated_refs":{"type":"array","items":{"type":"string"}},"requires_confirmation":{"type":"boolean"},"confirmation_prompt":{"type":["string","null"]}}})
        }
        "agent_edit.commit" => {
            json!({"type":"object","additionalProperties":false,"required":["schema_version","proposal_id","proposal_digest","target","previous_version_id","new_version_id","new_revision","applied","invalidated_refs","confirmed_by","committed_at","idempotency_key","replayed"],"properties":{"schema_version":{"type":"integer"},"proposal_id":{"type":"string"},"proposal_digest":{"type":"string"},"target":{"type":"object","additionalProperties":true},"previous_version_id":{"type":"string"},"new_version_id":{"type":"string"},"new_revision":{"type":"integer"},"applied":{"type":"array","items":{"type":"object","additionalProperties":true}},"invalidated_refs":{"type":"array","items":{"type":"string"}},"confirmed_by":{"type":"string"},"committed_at":{"type":"string"},"idempotency_key":{"type":"string"},"replayed":{"type":"boolean"}}})
        }
        "step.edit" => json!({"oneOf":[
            {"type":"object","additionalProperties":false,"required":["schema_version","edit_id","edit_digest","block_version_id","block_id","current_revision","status","requires_confirmation","ops","blocks","issues","invalidated_refs"],"properties":{"schema_version":{"type":"integer"},"edit_id":{"type":"string"},"edit_digest":{"type":"string"},"block_version_id":{"type":"string"},"block_id":{"type":"string"},"current_revision":{"type":"integer"},"status":{"type":"string","enum":["valid","needs_confirmation","blocked"]},"requires_confirmation":{"type":"boolean"},"confirmation_prompt":{"type":["string","null"]},"ops":{"type":"array","items":{"type":"object","additionalProperties":false,"required":["op","path","changed","description"],"properties":{"op":{"type":"string","enum":["insert","reorder","reuse","bind"]},"path":{"type":"string"},"changed":{"type":"boolean"},"description":{"type":"string"}}}},"blocks":{"type":"array","items":{"type":"object","additionalProperties":false,"required":["block_id","based_on_version_id","previous_version","new_version","order"],"properties":{"block_id":{"type":"string"},"based_on_version_id":{"type":"string"},"previous_version":{"type":"integer"},"new_version":{"type":"integer"},"order":{"type":"array","items":{"type":"string"}}}}}},"issues":{"type":"array","items":{"type":"object","additionalProperties":true}},"invalidated_refs":{"type":"array","items":{"type":"string"}}},
            {"type":"object","additionalProperties":false,"required":["schema_version","edit_id","edit_digest","block_version_id","block_id","previous_version_id","new_root_block_version_id","new_revision","created_versions","invalidated_refs","confirmed_by","committed_at","idempotency_key","replayed"],"properties":{"schema_version":{"type":"integer"},"edit_id":{"type":"string"},"edit_digest":{"type":"string"},"block_version_id":{"type":"string"},"block_id":{"type":"string"},"previous_version_id":{"type":"string"},"new_root_block_version_id":{"type":"string"},"new_revision":{"type":"integer"},"created_versions":{"type":"array","items":{"type":"object","additionalProperties":false,"required":["block_id","based_on_version_id","version_id","version"],"properties":{"block_id":{"type":"string"},"based_on_version_id":{"type":"string"},"version_id":{"type":"string"},"version":{"type":"integer"}}}},"invalidated_refs":{"type":"array","items":{"type":"string"}},"confirmed_by":{"type":"string"},"committed_at":{"type":"string"},"idempotency_key":{"type":"string"},"replayed":{"type":"boolean"}}}
        ]}),
        "plate.create" => one("plate"),
        "well.set" => one("well"),
        "note.create" => one("note"),
        "execution_event.create" => json!({"type":"object","additionalProperties":false,"required":["record","pantry_notices"],"properties":{"record":record_schema("execution_event"),"pantry_notices":{"type":"array","maxItems":1,"items":pantry_notice()}}}),
        "pantry.list" => json!({"type":"object","additionalProperties":false,"required":["as_of","near_expiry_days","items"],"properties":{"as_of":{"type":"string","format":"date"},"near_expiry_days":{"type":"integer","minimum":0},"items":{"type":"array","items":pantry_item()}}}),
        "pantry.upsert" => json!({"type":"object","additionalProperties":false,"required":["item"],"properties":{"item":pantry_item()}}),
        "pantry.delete" => json!({"type":"object","additionalProperties":false,"required":["pantry_item_id","label_code","revision","deleted"],"properties":{"pantry_item_id":{"type":"string"},"label_code":{"type":"string"},"revision":{"type":"integer","minimum":2},"deleted":{"const":true}}}),
        "label.export" => json!({"type":"object","additionalProperties":false,"required":["schema_version","generated_at","as_of","near_expiry_days","labels"],"properties":{"schema_version":{"const":1},"generated_at":{"type":"string"},"as_of":{"type":"string","format":"date"},"near_expiry_days":{"type":"integer","minimum":0},"labels":{"type":"array","items":pantry_label()}}}),
        "readout.submit" => json!({"type":"object","additionalProperties":false,"required":["record","idempotency_key","replayed"],"properties":{"record":record_schema("execution_event"),"idempotency_key":{"type":"string"},"replayed":{"type":"boolean"},"accepted":{"type":"integer","minimum":1,"description":"多值型 readouts[] 提交时记录的读数条数"}}}),
        "readout.batch" => json!({"type":"object","additionalProperties":false,"required":["record","idempotency_key","replayed","accepted"],"properties":{"record":record_schema("execution_event"),"idempotency_key":{"type":"string"},"replayed":{"type":"boolean"},"accepted":{"type":"integer","minimum":1}}}),
        "formula.create" => one("formula"),
        "formula.revise" => one("formula"),
        "table.create" => one("library_table"),
        "table.revise" => one("library_table"),
        "record.list" => {
            json!({"type":"object","additionalProperties":false,"required":["records"],"properties":{"records":{"type":"array","items":any_record_schema()}}})
        }
        "record.get" => {
            json!({"type":"object","additionalProperties":false,"required":["record"],"properties":{"record":any_record_schema()}})
        }
        "asset.link" => {
            json!({"type":"object","additionalProperties":false,"required":["record","event"],"properties":{"record":record_schema("asset_link"),"event":record_schema("asset_link_event")}})
        }
        "asset.link.update" => {
            json!({"type":"object","additionalProperties":false,"required":["record","event"],"properties":{"record":record_schema("asset_link"),"event":record_schema("asset_link_event")}})
        }
        "asset.unlink" => {
            json!({"type":"object","additionalProperties":false,"required":["removed","event"],"properties":{"removed":record_schema("asset_link"),"event":record_schema("asset_link_event")}})
        }
        "asset.list" => {
            json!({"type":"object","additionalProperties":false,"required":["records"],"properties":{"records":{"type":"array","items":asset_link_payload_schema()}}})
        }
        "asset.events" => {
            json!({"type":"object","additionalProperties":false,"required":["records"],"properties":{"records":{"type":"array","items":record_schema("asset_link_event")}}})
        }
        "import.study.preview" => {
            json!({"type":"object","additionalProperties":false,"required":["valid","writes","project_count","design_count","protocol_count","warnings"],"properties":{"valid":{"const":true},"writes":{"const":false},"project_count":{"const":1},"design_count":{"type":"integer","minimum":0},"protocol_count":{"type":"integer","minimum":0},"warnings":{"type":"array","items":{"type":"string"}}}})
        }
        "import.study.apply" => {
            json!({"type":"object","additionalProperties":false,"required":["writes","project","designs","protocols"],"properties":{"writes":{"const":true},"project":record_schema("project"),"designs":{"type":"array","items":{"type":"object","additionalProperties":false,"required":["design","version"],"properties":{"design":record_schema("design"),"version":record_schema("design_version")}}},"protocols":{"type":"array","items":{"type":"object","additionalProperties":false,"required":["protocol","version"],"properties":{"protocol":record_schema("protocol"),"version":record_schema("protocol_version")}}}}})
        }
        "calculator.dilution" => {
            json!({"type":"object","additionalProperties":false,"required":["stock_volume","volume_unit","diluent_volume","formula","assumption"],"properties":{"stock_volume":{"type":"number","minimum":0},"volume_unit":{"type":"string","enum":["L","mL","µL"]},"diluent_volume":{"type":"number","minimum":0},"formula":{"type":"string"},"assumption":{"type":"string"}}})
        }
        "calculator.solution" => {
            json!({"type":"object","additionalProperties":false,"required":["mass_g","mass_mg","formula","assumption"],"properties":{"mass_g":{"type":"number","minimum":0},"mass_mg":{"type":"number","minimum":0},"formula":{"type":"string"},"assumption":{"type":"string"}}})
        }
        "calculator.serial_dilution" => {
            json!({"type":"object","additionalProperties":false,"required":["per_step_dilution_factor","levels","formula","assumption"],"properties":{"per_step_dilution_factor":{"type":"number","exclusiveMinimum":0},"levels":{"type":"array","items":{"type":"object","additionalProperties":false,"required":["step","concentration","unit"],"properties":{"step":{"type":"integer","minimum":1},"concentration":{"type":"number","minimum":0},"unit":{"type":"string","enum":["M","mM","µM","nM"]}}}},"formula":{"type":"string"},"assumption":{"type":"string"}}})
        }
        "calculator.reaction_volume" => {
            json!({"type":"object","additionalProperties":false,"required":["allocated_volume_ul","remaining_volume_ul","unit","assumption"],"properties":{"allocated_volume_ul":{"type":"number","minimum":0},"remaining_volume_ul":{"type":"number","minimum":0},"unit":{"const":"µL"},"assumption":{"type":"string"}}})
        }
        "calculator.gel_mastermix" => {
            json!({"type":"object","additionalProperties":false,"required":["groups","total_volume_ul","total_gelma_mass_mg","total_lap_stock_volume_ul","total_solvent_volume_ul","formula","assumption"],"properties":{"groups":{"type":"array","minItems":1,"items":{"type":"object","additionalProperties":false,"required":["name","wells","gelma_percent_wv","volume_ul","gelma_mass_mg","lap_stock_volume_ul","solvent_volume_ul"],"properties":{"name":{"type":"string"},"wells":{"type":"integer","minimum":1},"gelma_percent_wv":{"type":"number","exclusiveMinimum":0},"volume_ul":{"type":"number","minimum":0},"gelma_mass_mg":{"type":"number","minimum":0},"lap_stock_volume_ul":{"type":"number","minimum":0},"solvent_volume_ul":{"type":"number","minimum":0}}}},"total_volume_ul":{"type":"number","minimum":0},"total_gelma_mass_mg":{"type":"number","minimum":0},"total_lap_stock_volume_ul":{"type":"number","minimum":0},"total_solvent_volume_ul":{"type":"number","minimum":0},"formula":{"type":"string"},"assumption":{"type":"string"}}})
        }
        "calculator.cell_seeding" => {
            json!({"type":"object","additionalProperties":false,"required":["groups","total_cells","total_suspension_volume_ul","total_factor_stock_volume_ul","formula","assumption"],"properties":{"groups":{"type":"array","minItems":1,"items":{"type":"object","additionalProperties":false,"required":["name","wells","total_cells","suspension_volume_ul","factor_stock_volume_ul"],"properties":{"name":{"type":"string"},"wells":{"type":"integer","minimum":1},"total_cells":{"type":"integer","minimum":1},"suspension_volume_ul":{"type":"number","minimum":0},"factor_stock_volume_ul":{"type":"number","minimum":0}}}},"total_cells":{"type":"integer","minimum":1},"total_suspension_volume_ul":{"type":"number","minimum":0},"total_factor_stock_volume_ul":{"type":"number","minimum":0},"formula":{"type":"string"},"assumption":{"type":"string"}}})
        }
        "calculator.runtime_normalize" => {
            json!({"type":"object","additionalProperties":false,"required":["planned_volume_ul","required_volume_ul","dead_volume_ul","additional_volume_ul","unused_volume_ul","formula","assumption"],"properties":{"planned_volume_ul":{"type":"number","minimum":0},"required_volume_ul":{"type":"number","minimum":0},"dead_volume_ul":{"type":"number","minimum":0},"additional_volume_ul":{"type":"number","minimum":0},"unused_volume_ul":{"type":"number","minimum":0},"formula":{"type":"string"},"assumption":{"type":"string"}}})
        }
        "calculator.sample_normalize_batch" => {
            json!({"type":"object","additionalProperties":false,"required":["rows","insufficient_count","formula","assumption"],"properties":{"rows":{"type":"array","minItems":1,"maxItems":100,"items":{"type":"object","additionalProperties":false,"required":["name","concentration_ng_per_ul","required_sample_volume_ul","sample_volume_ul","diluent_volume_ul","achieved_mass_ng","insufficient"],"properties":{"name":{"type":"string"},"concentration_ng_per_ul":{"type":"number","exclusiveMinimum":0},"required_sample_volume_ul":{"type":"number","minimum":0},"sample_volume_ul":{"type":"number","minimum":0},"diluent_volume_ul":{"type":"number","minimum":0},"achieved_mass_ng":{"type":"number","minimum":0},"insufficient":{"type":"boolean"}}}},"insufficient_count":{"type":"integer","minimum":0},"formula":{"type":"string"},"assumption":{"type":"string"}}})
        }
        "calculator.in_vivo_dosing" => {
            json!({"type":"object","additionalProperties":false,"required":["rows","total_body_weight_g","total_dose_mass_mg","total_injection_volume_ul","prepared_dose_mass_mg","preparation_volume_ul","stock_volume_ul","diluent_volume_ul","formula","assumption"],"properties":{"rows":{"type":"array","minItems":1,"maxItems":200,"items":{"type":"object","additionalProperties":false,"required":["index","body_weight_g","dose_mass_mg","injection_volume_ul"],"properties":{"index":{"type":"integer","minimum":1},"body_weight_g":{"type":"number","exclusiveMinimum":0},"dose_mass_mg":{"type":"number","minimum":0},"injection_volume_ul":{"type":"number","minimum":0}}}},"total_body_weight_g":{"type":"number","minimum":0},"total_dose_mass_mg":{"type":"number","minimum":0},"total_injection_volume_ul":{"type":"number","minimum":0},"prepared_dose_mass_mg":{"type":"number","minimum":0},"preparation_volume_ul":{"type":"number","minimum":0},"stock_volume_ul":{"type":"number","minimum":0},"diluent_volume_ul":{"type":"number","minimum":0},"formula":{"type":"string"},"assumption":{"type":"string"}}})
        }
        "calculator.biofilm_cfu" => {
            json!({"type":"object","additionalProperties":false,"required":["groups","excluded","excluded_count","formula","assumption"],"properties":{"groups":{"type":"array","items":{"type":"object","additionalProperties":false,"required":["sample_id","total_dilution","plated_volume_ml","suspension_volume_ml","plate_count","mean_colonies","cfu_per_ml","cfu_per_sample","readouts"],"properties":{"sample_id":{"type":"string"},"total_dilution":{"type":"number","exclusiveMinimum":0},"plated_volume_ml":{"type":"number","exclusiveMinimum":0},"suspension_volume_ml":{"type":"number","exclusiveMinimum":0},"plate_count":{"type":"integer","minimum":1},"mean_colonies":{"type":"number","minimum":0},"cfu_per_ml":{"type":"number","minimum":0},"cfu_per_sample":{"type":"number","minimum":0},"readouts":{"type":"array","minItems":2,"maxItems":2,"items":{"type":"object","additionalProperties":false,"required":["readout_id","endpoint","value","unit","sample_id","result_kind","method","derived_from"],"properties":{"readout_id":{"type":"string"},"endpoint":{"type":"string","enum":["cfu_per_ml","cfu_per_sample"]},"value":{"type":"number","minimum":0},"unit":{"type":"string"},"sample_id":{"type":"string"},"result_kind":{"type":"string"},"method":{"type":"string"},"derived_from":{"type":"array","minItems":1,"items":{"type":"string"}}}}}}}},"excluded":{"type":"array","items":{"type":"object","additionalProperties":false,"required":["raw_id","sample_id","colonies","total_dilution","reason"],"properties":{"raw_id":{"type":"string"},"sample_id":{"type":"string"},"colonies":{"type":"number","minimum":0},"total_dilution":{"type":"number","exclusiveMinimum":0},"reason":{"type":"string","enum":["below_window","above_window"]}}}},"excluded_count":{"type":"integer","minimum":0},"formula":{"type":"string"},"assumption":{"type":"string"}}})
        }
        "sdk.modules" => json!({"type":"object","additionalProperties":false,"required":["modules"],"properties":{"modules":{"type":"array","items":{"type":"object","additionalProperties":true,"required":["kind","schema"],"properties":{"kind":{"type":"string"},"schema":{"type":"integer","minimum":1},"title":{"type":["string","null"]},"category":{"type":["string","null"]},"capabilities":{"type":["array","null"]},"renderers":{"type":["object","null"]},"actions":{"type":["array","null"]}}}}}}),
        "sdk.validate" => json!({"type":"object","additionalProperties":false,"required":["valid","compiled","errors","warnings","modules"],"properties":{"valid":{"type":"boolean"},"compiled":{"type":"boolean"},"errors":{"type":"array","items":{"type":"string"}},"warnings":{"type":"array","items":{"type":"string"}},"modules":{"type":"array","items":{"type":"object","additionalProperties":false,"required":["kind","schema"],"properties":{"kind":{},"schema":{}}}},"design":{"type":"object","additionalProperties":true}}}),
        "asterdoc.inspect" => json!({"type":"object","additionalProperties":false,"required":["design_id","revision","source_hash","modules"],"properties":{"design_id":{"type":"string"},"revision":{"type":"integer","minimum":0},"source_hash":{"type":"string"},"modules":{"type":"array","items":{"type":"object","additionalProperties":false,"required":["kind","id","schema","data","manifest"],"properties":{"kind":{"type":"string"},"id":{"type":"string"},"schema":{"type":"integer","minimum":0},"data":{"type":"object","additionalProperties":true},"manifest":{"type":["object","null"],"additionalProperties":true}}}}}}),
        "asterdoc.assess" => json!({"type":"object","additionalProperties":false,"required":["design_id","revision","source_hash","state","errors","warnings"],"properties":{"design_id":{"type":"string"},"revision":{"type":"integer","minimum":0},"source_hash":{"type":"string"},"state":{"type":"string","enum":["ready","warning","incomplete"]},"errors":{"type":"array","items":{"type":"string"}},"warnings":{"type":"array","items":{"type":"string"}}}}),

        "asterdoc.validate" => json!({"type":"object","additionalProperties":false,"required":["valid","compiled"],"properties":{"valid":{"type":"boolean"},"compiled":{"type":"boolean"},"design":{"type":"object","additionalProperties":true},"error":{"type":"object","additionalProperties":false,"required":["code","message"],"properties":{"code":{"type":"string"},"message":{"type":"string"}}}}}),
        "asterdoc.load" | "asterdoc.save" | "asterdoc.command" => asterdoc_version(),
        "asterdoc.compile" => json!({"type":"object","additionalProperties":false,"required":["design","document"],"properties":{"design":{"type":"object","additionalProperties":true},"document":asterdoc_version()}}),
        "asterdoc.create_run" => record_schema("run"),
        _ => json!({"type":"object"}),
    }
}
pub(crate) fn diagnostic_schema() -> Value {
    json!({"type":"object","additionalProperties":false,"required":["path","code","severity","message","affects","next_action","source"],"properties":{"path":{"type":"string"},"code":{"type":"string"},"severity":{"type":"string","enum":["blocking","needs_confirmation","advisory"]},"message":{"type":"string"},"affects":{"type":"array","items":{"type":"string"}},"next_action":{"type":"string"},"source":{"type":"string","enum":["user_confirmed","imported_record","referenced_protocol","derived","agent_suggestion","unknown"]}}})
}

pub(crate) fn output_schema(tool: &str) -> Value {
    let success = json!({"type":"object","additionalProperties":false,"required":["ok","tool","result"],"properties":{"ok":{"const":true},"tool":{"const":tool},"result":result_schema(tool),"diagnostics":{"type":"array","items":diagnostic_schema()}}});
    let error = if matches!(tool, "agent_edit.validate" | "agent_edit.commit" | "step.edit") {
        json!({"type":"object","additionalProperties":false,"required":["ok","tool","error"],"properties":{"ok":{"const":false},"tool":{"const":tool},"error":{"type":"object","additionalProperties":false,"required":["code","message"],"properties":{"code":{"type":"string","enum":["INVALID_PROPOSAL","INVALID_BINDING","UNIT_MISMATCH","STALE_REFERENCE","FORBIDDEN_FIELD","VERSION_CONFLICT","CONFIRMATION_REQUIRED","CONFIRMATION_MISMATCH","IDEMPOTENCY_MISMATCH","INVALID_INPUT","NOT_FOUND","REVISION_CONFLICT","INTERNAL"]},"message":{"type":"string"},"path":{"type":["string","null"]},"current_version":{"type":["integer","null"]},"current_version_id":{"type":["string","null"]},"issues":{"type":"array","items":{"type":"object","additionalProperties":true}}}},"diagnostics":{"type":"array","items":diagnostic_schema()}}})
    } else if matches!(tool, "readout.submit" | "readout.batch") {
        json!({"type":"object","additionalProperties":false,"required":["ok","tool","error"],"properties":{"ok":{"const":false},"tool":{"const":tool},"error":{"type":"object","additionalProperties":false,"required":["code","message"],"properties":{"code":{"type":"string","enum":["INVALID_INPUT","NOT_FOUND","REVISION_CONFLICT","INTERNAL"]},"message":{"type":"string"},"issues":{"type":"array","items":{"type":"object","additionalProperties":false,"required":["code","message"],"properties":{"code":{"type":"string"},"message":{"type":"string"}}}}}},"diagnostics":{"type":"array","items":diagnostic_schema()}}})
    } else {
        json!({"type":"object","additionalProperties":false,"required":["ok","tool","error"],"properties":{"ok":{"const":false},"tool":{"const":tool},"error":{"type":"object","additionalProperties":false,"required":["code","message"],"properties":{"code":{"type":"string","enum":["INVALID_INPUT","NOT_FOUND","REVISION_CONFLICT","INTERNAL"]},"message":{"type":"string"}}},"diagnostics":{"type":"array","items":diagnostic_schema()}}})
    };
    json!({"oneOf":[success,error]})
}
pub(crate) fn input_schema(tool: &str) -> Option<Value> {
    let mut s = match tool {
        "project.list" => json!({"type":"object","additionalProperties":false,"properties":{}}),
        "project.create" => {
            json!({"type":"object","additionalProperties":false,"required":["name"],"properties":{"name":{"type":"string","minLength":1,"maxLength":120,"pattern":"\\S"},"description":{"type":"string"}}})
        }
        "project.revise" => {
            json!({"type":"object","additionalProperties":false,"required":["id","expected_revision"],"anyOf":[{"required":["name"]},{"required":["description"]}],"properties":{"id":{"type":"string"},"expected_revision":{"type":"integer","minimum":1},"name":{"type":"string","minLength":1,"maxLength":120,"pattern":"\\S"},"description":{"type":"string"}}})
        }
        "design.list" => {
            json!({"type":"object","additionalProperties":false,"properties":{"project_id":{"type":"string"}}})
        }
        "design.get" => {
            json!({"type":"object","additionalProperties":false,"required":["id"],"properties":{"id":{"type":"string"}}})
        }
        "design.map" => {
            json!({"type":"object","additionalProperties":false,"required":["id"],"properties":{"id":{"type":"string"}}})
        }
        "design.create" => {
            json!({"type":"object","additionalProperties":false,"required":["project_id","design"],"properties":{"project_id":{"type":"string"},"design":design_schema()}})
        }
        "design.revise" => {
            json!({"type":"object","additionalProperties":false,"required":["id","expected_revision","changes"],"properties":{"id":{"type":"string"},"expected_revision":{"type":"integer","minimum":1},"changes":{"type":"object","additionalProperties":false,"minProperties":1,"properties":design_schema()["properties"]}}})
        }
        "run.list" | "protocol.list" => {
            json!({"type":"object","additionalProperties":false,"properties":{"project_id":{"type":"string"}}})
        }
        "run.workspace" => {
            json!({"type":"object","additionalProperties":false,"required":["id"],"properties":{"id":{"type":"string"},"view":{"type":"string","enum":["blind","unblinded"],"description":"blind 只遮蔽已声明盲法依据的冻结设计分组身份；默认 unblinded"}}})
        }
        "protocol.block.list" => {
            json!({"type":"object","additionalProperties":false,"properties":{"project_id":{"type":"string"}}})
        }
        "protocol.version.get" => json!({"type":"object","additionalProperties":false,"required":["protocol_version_id"],"properties":{"protocol_version_id":{"type":"string","minLength":1}}}),
        "protocol.version.list" => json!({"type":"object","additionalProperties":false,"required":["protocol_id"],"properties":{"protocol_id":{"type":"string","minLength":1}}}),
        "protocol.block.get" => {
            json!({"type":"object","additionalProperties":false,"required":["id"],"properties":{"id":{"type":"string"}}})
        }
        "protocol.block.create" => {
            json!({"type":"object","additionalProperties":false,"required":["project_id","title","steps"],"properties":{"project_id":{"type":"string"},"title":{"type":"string","minLength":1,"maxLength":160,"pattern":"\\S"},"summary":{"type":"string","maxLength":1000},"parameter_slots":{"type":"array","maxItems":100,"items":parameter_slot_input_schema()},"steps":{"type":"array","minItems":1,"maxItems":100,"items":protocol_step_input_schema()}}})
        }
        "component.list" => {
            json!({"type":"object","additionalProperties":false,"properties":{}})
        }
        "labware.define" => {
            json!({"type":"object","additionalProperties":false,"required":["profile"],"properties":{"profile":{"type":"object"}}})
        }
        "component.define" => {
            json!({"type":"object","additionalProperties":false,"required":["definition"],"properties":{"definition":{"type":"object"}}})
        }
        "view.project" => {
            json!({"type":"object","additionalProperties":false,"required":["project_id","protocol_version_id","scope"],"properties":{"project_id":{"type":"string"},"protocol_version_id":{"type":"string"},"scope":{"type":"object"},"document_id":{"type":"string"}}})
        }
        "view.data_preview" => {
            json!({"type":"object","additionalProperties":false,"required":["view_id","instance_id"],"properties":{"view_id":{"type":"string"},"instance_id":{"type":"string"},"position_ids":{"type":"array","items":{"type":"string"}},"calculator_instance_id":{"type":"string"},"volume_per_well":{"type":["number","null"]},"volume_unit":{"type":"string"},"volume_per_well":{"type":["number","null"]},"extra_ratio":{"type":["number","null"]}}})
        }
        "view.create" => {
            json!({"type":"object","additionalProperties":false,"required":["project_id","title","document"],"properties":{"project_id":{"type":"string"},"title":{"type":"string","minLength":1,"maxLength":160},"document":{"type":"object"}}})
        }
        "view.get" => {
            json!({"type":"object","additionalProperties":false,"required":["id"],"properties":{"id":{"type":"string"}}})
        }
        "view.revise" => {
            json!({"type":"object","additionalProperties":false,"required":["view_id","changeset"],"properties":{"view_id":{"type":"string"},"changeset":{"type":"object"}}})
        }
        "view.proposal.create" => {
            json!({"type":"object","additionalProperties":false,"required":["view_id","changeset"],"properties":{"view_id":{"type":"string"},"changeset":{"type":"object"},"source":{"type":"string","maxLength":120}}})
        }
        "view.proposal.list" => {
            json!({"type":"object","additionalProperties":false,"required":["view_id"],"properties":{"view_id":{"type":"string"}}})
        }
        "view.proposal.accept" | "view.proposal.reject" => {
            json!({"type":"object","additionalProperties":false,"required":["view_id","proposal_id"],"properties":{"view_id":{"type":"string"},"proposal_id":{"type":"string"}}})
        }
        "semantic.inspect" => {
            json!({"type":"object","additionalProperties":false,"required":["block_version_id"],"properties":{"block_version_id":{"type":"string"}}})
        }
        "semantic.search" => {
            json!({"type":"object","additionalProperties":false,"properties":{"query":{"type":"string"},"project_id":{"type":"string"}}})
        }
        "semantic.validate"
        | "semantic.propose_patch"
        | "semantic.assess_patch"
        | "semantic.apply_binding_patch" => {
            json!({"type":"object","additionalProperties":false,"required":["changeset"],"properties":{"changeset":{"type":"object"}}})
        }
        "catalog.review.record" => {
            json!({"type":"object","additionalProperties":false,"required":["project_id","review"],"properties":{"project_id":{"type":"string"},"review":{"type":"object"}}})
        }
        "catalog.operation.import" => {
            json!({"type":"object","additionalProperties":false,"required":["project_id","module","review_id"],"properties":{"project_id":{"type":"string"},"summary":{"type":"string","maxLength":1000},"module":{"type":"object"},"review_id":{"type":"string"}}})
        }
        "protocol.block.revise" => {
            json!({"type":"object","additionalProperties":false,"required":["block_id","expected_version","steps"],"properties":{"block_id":{"type":"string"},"expected_version":{"type":"integer","minimum":1},"summary":{"type":"string","maxLength":1000},"parameter_slots":{"type":"array","maxItems":100,"items":parameter_slot_input_schema()},"steps":{"type":"array","minItems":1,"maxItems":100,"items":protocol_step_input_schema()}}})
        }
        "protocol.block.preview" => {
            json!({"type":"object","additionalProperties":false,"required":["block_version_id"],"properties":{"block_version_id":{"type":"string"},"bindings":root_parameter_values_input_schema()}})
        }
        "run.set_status" => {
            json!({"type":"object","additionalProperties":false,"required":["id","expected_revision","status"],"properties":{"id":{"type":"string"},"expected_revision":{"type":"integer","minimum":1},"status":{"type":"string","enum":["planned","in_progress","completed","blocked","cancelled"]}}})
        }
        "run.revise" => {
            json!({"type":"object","additionalProperties":false,"required":["id","expected_revision"],"anyOf":[{"required":["title"]},{"required":["scheduled_date"]}],"properties":{"id":{"type":"string"},"expected_revision":{"type":"integer","minimum":1},"title":{"type":"string","minLength":1,"maxLength":160,"pattern":"\\S"},"scheduled_date":{"type":["string","null"],"format":"date"}}})
        }
        "run.create" => {
            json!({"type":"object","additionalProperties":false,"required":["design_id","title"],"properties":{"design_id":{"type":"string"},"title":{"type":"string","minLength":1,"maxLength":160,"pattern":"\\S"},"scheduled_date":{"type":["string","null"],"format":"date"},"protocol_version_id":{"type":["string","null"]},"repeat_of_run_id":{"type":["string","null"]},"repeat_relationship":{"type":"string","enum":["independent","same_source","paired","unknown"]},"source_run_id":{"type":["string","null"],"description":"同项目下提供样本的上游收获 Run；可跨 Design，与 repeat_of_run_id 不是同一关系"}}})
        }
        "workflow.inspect_tree" => json!({"type":"object","additionalProperties":false,"required":["design_id"],"properties":{"design_id":{"type":"string"},"revision":{"type":"integer","minimum":1},"run_id":{"type":"string"}}}),
        "workflow.progress" => json!({"type":"object","additionalProperties":false,"required":["run_id"],"properties":{"run_id":{"type":"string","minLength":1},"step_path":{"type":"string","minLength":1,"maxLength":500,"description":"只看一个冻结步骤"},"timepoint":{"type":"string","minLength":1,"maxLength":100,"description":"只看该步骤的一个具体冻结时间点；省略时只返回事件实际携带的分组"},"sample_id":{"type":"string","minLength":1,"maxLength":120,"description":"只看一个样本"},"sample_ids":{"type":"array","maxItems":100,"items":{"type":"string","minLength":1,"maxLength":120},"description":"仅与 step_path 和具体 timepoint 同时提供时，为该分组补入 not_started 样本"}}}),
        "task.create" => {
            json!({"type":"object","additionalProperties":false,"required":["project_id","title","scheduled_date"],"properties":{"project_id":{"type":"string"},"run_id":{"type":["string","null"]},"title":{"type":"string","minLength":1,"maxLength":160,"pattern":"\\S"},"scheduled_date":{"type":"string","minLength":1,"format":"date"},"status":{"type":"string","enum":["planned","in_progress","done","blocked"]},"notes":{"type":"string"}}})
        }
        "task.set_status" => {
            json!({"type":"object","additionalProperties":false,"required":["id","expected_revision","status"],"properties":{"id":{"type":"string"},"expected_revision":{"type":"integer","minimum":1},"status":{"type":"string","enum":["planned","in_progress","done","blocked"]}}})
        }
        "protocol.create" => {
            json!({"type":"object","additionalProperties":false,"required":["title"],"anyOf":[{"required":["body"]},{"required":["root_block_version_id"]}],"properties":{"project_id":{"type":["string","null"]},"title":{"type":"string","minLength":1,"maxLength":160,"pattern":"\\S"},"body":{"type":"string"},"root_block_version_id":{"type":"string"},"root_bindings":{"type":"array","maxItems":100,"items":{"type":"object","additionalProperties":false,"required":["slot","value"],"properties":{"slot":{"type":"string"},"value":{"type":["string","number","integer","boolean"]},"unit":{"type":"string","maxLength":40}}}}}})
        }
        "protocol.revise" => {
            json!({"type":"object","additionalProperties":false,"required":["protocol_id","expected_version"],"anyOf":[{"required":["body"]},{"required":["root_block_version_id"]},{"required":["root_bindings"]}],"properties":{"protocol_id":{"type":"string"},"expected_version":{"type":"integer","minimum":1},"body":{"type":"string"},"root_block_version_id":{"type":["string","null"]},"root_bindings":{"type":"array","maxItems":100,"items":{"type":"object","additionalProperties":false,"required":["slot","value"],"properties":{"slot":{"type":"string"},"value":{"type":["string","number","integer","boolean"]},"unit":{"type":"string","maxLength":40}}}}}})
        }
        "agent_edit.validate" => {
            json!({"type":"object","additionalProperties":false,"required":["proposal_id","target","base_version_id","expected_version","origin","diffs"],"properties":{"schema_version":{"type":"integer"},"proposal_id":{"type":"string"},"target":{"type":"object"},"base_version_id":{"type":"string"},"expected_version":{"type":"integer"},"origin":{"type":"object"},"diffs":{"type":"array","items":{"type":"object"}}}})
        }
        "agent_edit.commit" => {
            json!({"type":"object","additionalProperties":false,"required":["proposal","proposal_digest","idempotency_key","confirmed_by"],"properties":{"schema_version":{"type":"integer"},"proposal":{"type":"object"},"proposal_digest":{"type":"string"},"idempotency_key":{"type":"string"},"confirmed_by":{"type":"string"},"acknowledged":{"type":"array","items":{"type":"string"}},"note":{"type":["string","null"]}}})
        }
        "step.edit" => {
            json!({"type":"object","additionalProperties":false,"required":["edit_id","block_version_id","expected_version","origin","ops"],"properties":{"schema_version":{"type":"integer","minimum":1},"edit_id":{"type":"string","minLength":1,"maxLength":120},"block_version_id":{"type":"string","minLength":1},"expected_version":{"type":"integer","minimum":1},"origin":{"type":"object","additionalProperties":false,"required":["source"],"properties":{"source":{"type":"string","enum":["user","agent","tool"]},"rationale":{"type":["string","null"]}}},"ops":{"type":"array","minItems":1,"maxItems":100,"items":step_edit_op_input_schema()},"confirmation":{"type":"object","additionalProperties":false,"required":["edit_digest","confirmed_by","idempotency_key"],"properties":{"edit_digest":{"type":"string","minLength":1},"confirmed_by":{"type":"string","minLength":1},"idempotency_key":{"type":"string","minLength":1},"note":{"type":["string","null"]}}}}})
        }
        "plate.create" => {
            json!({"type":"object","additionalProperties":false,"required":["run_id","name","format","rows","columns"],"properties":{"run_id":{"type":"string"},"assay_id":{"type":"string","description":"可选；必须是 Run 冻结设计中的检测 ID"},"timepoint":{"type":"string","description":"可选；标注该实际板图所用的检测时间点，需与冻结设计一致"},"name":{"type":"string","minLength":1,"maxLength":100,"pattern":"\\S"},"format":{"type":"string","enum":["6","12","24","48","96","384","custom"]},"rows":{"type":"integer","minimum":1,"maximum":32},"columns":{"type":"integer","minimum":1,"maximum":48}}})
        }
        "well.set" => {
            json!({"type":"object","additionalProperties":false,"required":["plate_id","well_id"],"properties":{"plate_id":{"type":"string"},"well_id":{"type":"string","pattern":"^(?:[A-Za-z]{1,2}[0-9]{1,2}|[Ll]ane[0-9]{1,2}|[Ll][0-9]{1,2}|[Tt]ube[0-9]{1,2})$","description":"2D 孔位 A1..AA99；单行容器（rows=1）另接受 Lane3 / L3 / Tube3，三种写法与 A3 指向同一槽位"},"expected_revision":{"type":"integer","minimum":1,"description":"仅覆盖已存在孔位时必需；创建新孔位时省略"},"group_label":{"type":"string"},"sample_id":{"type":"string"},"note":{"type":"string"},"target":{"type":["string","null"],"maxLength":120,"description":"计划检测目标（基因/分析物）；不是读数"},"well_role":{"type":["string","null"],"enum":["unknown","ntc","standard","positive_control",null],"description":"该孔位在检测布局中的计划角色"},"dilution_factor":{"type":["number","null"],"exclusiveMinimum":0,"description":"该孔位的计划稀释倍数；无单位、必须大于 0"}}})
        }
        "note.create" => {
            json!({"type":"object","additionalProperties":false,"required":["run_id","content"],"properties":{"run_id":{"type":"string"},"content":{"type":"string","minLength":1,"maxLength":10000,"pattern":"\\S"},"event_time":{"type":["string","null"],"format":"date-time"},"observation":{"type":"string"},"result":{"type":"string"},"deviation":{"type":"string"}}})
        }
        "execution_event.create" => {
            json!({"type":"object","additionalProperties":false,"required":["run_id","event_type","content"],"properties":{"run_id":{"type":"string"},"event_type":{"type":"string","enum":["operation","material_use","measurement","deviation","observation"]},"content":{"type":"string","minLength":1,"maxLength":5000,"pattern":"\\S"},"occurred_at":{"type":["string","null"],"format":"date-time"},"stage":{"type":["string","null"],"maxLength":160},"timepoint":{"type":["string","null"],"maxLength":100,"description":"检测步骤的计划时间点；必须属于Run冻结设计中的该检测"},"step_status":{"type":["string","null"],"enum":["in_progress","completed","blocked","skipped","qc_dropped","censored_dead","rework",null],"description":"显式步骤状态；qc_dropped/censored_dead/rework 表示样本被剔除、删失或需要重做"},"protocol_step_path":{"type":["string","null"],"maxLength":500},"ad_hoc_step":{"type":"boolean","default":false,"description":"临时补加步骤：允许 protocol_step_path 不属于冻结清单；省略时为 false"},"protocol_version_id":{"type":["string","null"],"description":"关联步骤所引用的冻结 Protocol 版本"},"assay_id":{"type":["string","null"],"description":"关联步骤所属的冻结检测；共享制备步骤留空"},"planned_output_id":{"type":["string","null"],"description":"共享制备步骤所属计划产物；检测步骤留空"},"pantry_item_id":{"type":["string","null"],"maxLength":120,"description":"可选；关联已存在的储藏室条目。缺失不阻断；条目临期或过期时在 pantry_notices 返回提示"},"sample_ids":{"type":"array","maxItems":100,"items":{"type":"string","minLength":1,"maxLength":120,"pattern":"\\S"}},"parent_sample_id":{"type":["string","null"],"maxLength":120,"description":"可选；这些样本的父样本编号"},"aliquot_from":{"type":["string","null"],"maxLength":120,"description":"可选；这些样本的分样来源编号"},"blind_masked":{"type":["boolean","null"],"description":"可选；该事件在盲法视图下记录"},"attempt":{"type":["integer","null"],"minimum":1,"maximum":1000,"description":"样本级 attempt；必须关联冻结步骤与样本，且只能是当前 attempt 或下一个 attempt"},"materials":{"type":"array","maxItems":100,"items":{"type":"object","additionalProperties":false,"required":["material"],"properties":{"material":{"type":"string","minLength":1,"maxLength":160,"pattern":"\\S"},"planned_output_id":{"type":["string","null"]},"amount":{"type":["number","null"],"minimum":0},"unit":{"type":["string","null"],"maxLength":40}}}},"measurements":{"type":"array","maxItems":100,"items":{"type":"object","additionalProperties":false,"required":["endpoint","value"],"properties":{"endpoint":{"type":"string","minLength":1,"maxLength":300,"pattern":"\\S"},"value":{"type":"number"},"unit":{"type":["string","null"],"maxLength":40},"sample_id":{"type":["string","null"],"maxLength":120}}}},"readouts":{"type":"array","maxItems":100,"items":{"type":"object","additionalProperties":false,"required":["endpoint"],"properties":{"readout_id":{"type":["string","null"],"maxLength":160},"endpoint":{"type":"string","minLength":1,"maxLength":300},"value":{"type":["number","null"]},"qualifier":{"type":["string","null"],"enum":["not_detected","below_lod","above_uloq","invalid","other",null]},"unit":{"type":["string","null"],"maxLength":40},"sample_id":{"type":["string","null"],"maxLength":120},"result_kind":{"type":"string","enum":["raw","derived"]},"method":{"type":["string","null"],"maxLength":300},"derived_from":{"type":"array","maxItems":100,"items":{"type":"string","maxLength":160}}}}},"checks":{"type":"array","maxItems":100,"items":{"type":"object","additionalProperties":false,"required":["check_id","status"],"properties":{"check_id":{"type":"string","minLength":1,"maxLength":80},"status":{"type":"string","enum":["passed","failed","blocked","skipped"]},"note":{"type":["string","null"],"maxLength":2000},"checked_by":{"type":["string","null"],"maxLength":160}}}},"controls":{"type":"array","maxItems":100,"items":{"type":"object","additionalProperties":false,"required":["role","status"],"properties":{"role":{"type":"string","enum":["blank","negative","positive","standard","vehicle","untreated","unstained","other"]},"subtype":{"type":["string","null"],"maxLength":80},"status":{"type":"string","enum":["passed","failed","blocked","skipped","not_run"]},"control_id":{"type":["string","null"],"maxLength":500},"sample_id":{"type":["string","null"],"maxLength":500},"experimental_unit":{"type":["string","null"],"maxLength":500},"expected":{"type":["string","null"],"maxLength":500},"observed":{"type":["string","null"],"maxLength":500},"note":{"type":["string","null"],"maxLength":500}}}},"lineage":{"type":"array","maxItems":100,"items":{"type":"object","additionalProperties":false,"required":["from_kind","from_id","to_kind","to_id","relation"],"properties":{"from_kind":{"type":"string","minLength":1,"maxLength":120},"from_id":{"type":"string","minLength":1,"maxLength":120},"to_kind":{"type":"string","minLength":1,"maxLength":120},"to_id":{"type":"string","minLength":1,"maxLength":120},"relation":{"type":"string","minLength":1,"maxLength":120},"note":{"type":["string","null"],"maxLength":500}}}},"actual_parameters":{"type":"array","maxItems":100,"items":{"type":"object","additionalProperties":false,"required":["key","value"],"properties":{"key":{"type":"string","minLength":1,"maxLength":120},"value":{},"unit":{"type":["string","null"],"maxLength":40},"source":{"type":["string","null"],"maxLength":300}}}},"operator":{"type":["string","null"],"maxLength":160},"deviation":{"type":["string","null"],"maxLength":5000}}})
        }
        "pantry.list" | "label.export" => json!({"type":"object","additionalProperties":false,"properties":{"project_id":{"type":["string","null"]},"include_exhausted":{"type":"boolean","default":true,"description":"false 时省略已耗尽的条目"},"as_of":{"type":"string","format":"date","description":"YYYY-MM-DD；省略时用当前 UTC 日期"},"near_expiry_days":{"type":"integer","minimum":0,"maximum":3650,"description":"临期窗口天数；默认 14"}}}),
        "pantry.delete" => json!({"type":"object","additionalProperties":false,"required":["id"],"properties":{"id":{"type":"string","minLength":1,"maxLength":120,"description":"要停用的储藏室条目 ID"},"expected_revision":{"type":["integer","null"],"minimum":1,"description":"提供时按版本校验，不一致返回 REVISION_CONFLICT"}}}),
        "pantry.upsert" => json!({"type":"object","additionalProperties":false,"required":["name"],"properties":{"id":{"type":["string","null"],"maxLength":120,"description":"省略时创建新条目；提供时必须已存在"},"project_id":{"type":["string","null"],"maxLength":120,"description":"可选；提供时必须指向已存在的项目"},"name":{"type":"string","minLength":1,"maxLength":200,"pattern":"\\S"},"location":{"type":"string","maxLength":200},"purchased_on":{"type":["string","null"],"format":"date"},"opened_on":{"type":["string","null"],"format":"date"},"shelf_life_days":{"type":["integer","null"],"minimum":1,"maximum":3650,"description":"未开封保质期天数；与 purchased_on 一起决定未开封效期"},"opened_shelf_life_days":{"type":["integer","null"],"minimum":1,"maximum":3650,"description":"开封后可用天数；与 opened_on 一起优先生效"},"exhausted":{"type":"boolean","default":false},"expected_revision":{"type":["integer","null"],"minimum":1,"description":"更新已有条目时用于并发保护；省略则不校验版本"}}}),
        "readout.submit" => {
            json!({"type":"object","additionalProperties":false,"required":["run_id","step_path","protocol_version_id"],"oneOf":[{"required":["endpoint","value"],"not":{"required":["readouts"]}},{"required":["endpoint","qualifier"],"not":{"required":["readouts"]}},{"required":["readouts"],"not":{"required":["endpoint","value","qualifier"]}}],"properties":{"run_id":{"type":"string","minLength":1,"maxLength":120},"step_path":{"type":"string","minLength":1,"maxLength":500,"pattern":"\\S","description":"本次 Run 冻结 Protocol 中的步骤路径"},"endpoint":{"type":"string","minLength":1,"maxLength":300,"pattern":"\\S","description":"标量读数必填；多值型 readouts[] 里由各读数的 schema 决定"},"protocol_version_id":{"type":"string","minLength":1,"maxLength":120},"assay_id":{"type":["string","null"],"maxLength":120},"planned_output_id":{"type":["string","null"],"maxLength":120,"description":"仅标量读数可用"},"timepoint":{"type":["string","null"],"maxLength":100},"sample_id":{"type":["string","null"],"maxLength":120},"value":{"type":["number","null"]},"qualifier":{"type":["string","null"],"enum":["not_detected","below_lod","above_uloq","invalid","other",null]},"unit":{"type":["string","null"],"maxLength":40},"content":{"type":["string","null"],"maxLength":5000},"occurred_at":{"type":["string","null"],"format":"date-time"},"operator":{"type":["string","null"],"maxLength":160},"batch_id":{"type":["string","null"],"maxLength":120,"description":"仅多值型 readouts[] 可用"},"readouts":{"type":"array","minItems":1,"maxItems":1000,"items":{"type":"object","description":"v1 Readout：readout_id?/schema_id/schema_version/value（numeric|table|plate|categorical|file）/unit?/missing?/source/link；字段与校验以 docs/contracts/readout-schema-v1.md 为准"},"description":"多值型读数：同一节点写入分类、表格、孔板或文件读数；不能与 value/qualifier/endpoint 同时使用"},"readout_schemas":{"type":"array","maxItems":100,"items":{"type":"object","description":"v1 ReadoutSchema 定义；本次提交声明，按其 schema_id 解析 readouts[]"},"description":"本次提交额外依赖的读数 schema；同一 id 不得给出不同定义"}}})
        }
        "readout.batch" => {
            json!({"type":"object","additionalProperties":false,"required":["run_id","step_path","protocol_version_id","plate_id","schema_id"],"oneOf":[{"required":["cells"]},{"required":["csv"]}],"properties":{"run_id":{"type":"string","minLength":1,"maxLength":120},"step_path":{"type":"string","minLength":1,"maxLength":500,"pattern":"\\S"},"protocol_version_id":{"type":"string","minLength":1,"maxLength":120},"plate_id":{"type":"string","minLength":1,"maxLength":120,"description":"本次 Run 中唯一的孔板 ID；缺省无法归属孔位"},"schema_id":{"type":"string","minLength":1,"maxLength":120},"schema_version":{"type":"integer","enum":[1]},"assay_id":{"type":["string","null"],"maxLength":120},"timepoint":{"type":["string","null"],"maxLength":100},"batch_id":{"type":["string","null"],"maxLength":120},"content":{"type":["string","null"],"maxLength":5000},"occurred_at":{"type":["string","null"],"format":"date-time"},"operator":{"type":["string","null"],"maxLength":160},"cells":{"type":"array","maxItems":1000,"items":{"type":"object","additionalProperties":false,"required":["row","column","field"],"properties":{"row":{"type":"integer","minimum":0},"column":{"type":"integer","minimum":0},"field":{"type":"string","minLength":1,"maxLength":300,"pattern":"\\S"},"value":{"type":["number","null"]},"unit":{"type":["string","null"],"maxLength":40},"sample_id":{"type":["string","null"],"maxLength":120},"group":{"type":["string","null"],"maxLength":120},"replicate":{"type":["integer","null"],"minimum":0},"status":{"type":["string","null"],"enum":["raw","missing",null]},"qualifier":{"type":["string","null"],"enum":["not_detected","below_lod","above_uloq","invalid","other",null],"description":"显式缺失原因；与 value 互斥。提供时 CLI 按 CSV 同一路径解析"}}}},"csv":{"type":"string","maxLength":1000000,"description":"表头 row,column,field[,value,unit,sample_id,group,replicate,status,qualifier]；行列从 0 开始；value 可写 Undetermined 记 not_detected；qualifier 只能与 status=missing 一起记录"}}})
        }
        "formula.create" => {
            json!({"type":"object","additionalProperties":false,"required":["project_id","title","expression"],"properties":{"project_id":{"type":"string"},"title":{"type":"string","minLength":1,"maxLength":160,"pattern":"\\S"},"expression":{"type":"string","minLength":1,"maxLength":2000,"pattern":"\\S"},"variables":{"type":"string"},"notes":{"type":"string"}}})
        }
        "formula.revise" => {
            json!({"type":"object","additionalProperties":false,"required":["id","expected_revision"],"anyOf":[{"required":["title"]},{"required":["expression"]},{"required":["variables"]},{"required":["notes"]}],"properties":{"id":{"type":"string"},"expected_revision":{"type":"integer","minimum":1},"title":{"type":"string","minLength":1,"maxLength":160,"pattern":"\\S"},"expression":{"type":"string","minLength":1,"maxLength":2000,"pattern":"\\S"},"variables":{"type":"string"},"notes":{"type":"string"}}})
        }
        "table.create" => {
            json!({"type":"object","additionalProperties":false,"required":["project_id","name","columns","rows"],"properties":{"project_id":{"type":"string"},"name":{"type":"string","minLength":1,"maxLength":120,"pattern":"\\S"},"columns":{"type":"array","items":{"type":"string"},"minItems":1,"maxItems":40},"rows":{"type":"array","maxItems":5000,"items":{"type":"array","items":{"type":"string"}}}}})
        }
        "table.revise" => {
            json!({"type":"object","additionalProperties":false,"required":["id","expected_revision","name","columns","rows"],"properties":{"id":{"type":"string"},"expected_revision":{"type":"integer","minimum":1},"name":{"type":"string","minLength":1,"maxLength":120,"pattern":"\\S"},"columns":{"type":"array","items":{"type":"string"},"minItems":1,"maxItems":40},"rows":{"type":"array","maxItems":5000,"items":{"type":"array","items":{"type":"string"}}}}})
        }
        "record.list" => {
            json!({"type":"object","additionalProperties":false,"properties":{"kind":{"type":"string","enum":["project","design","design_version","run","task","protocol","protocol_version","protocol_block","protocol_block_version","plate","well","note","execution_event","formula","library_table","asset_link","asset_link_event"]},"project_id":{"type":"string"}}})
        }
        "record.get" => {
            json!({"type":"object","additionalProperties":false,"required":["id"],"properties":{"id":{"type":"string"}}})
        }
        "asset.link" => {
            json!({"type":"object","additionalProperties":false,"required":["root_id","root_dir","run_id","task_key"],"anyOf":[{"required":["relative_path"]},{"required":["path"]}],"properties":{"root_id":{"type":"string","minLength":1,"maxLength":200,"pattern":"\\S","description":"逻辑根 ID；只保存它和相对路径，不保存绝对路径"},"root_dir":{"type":"string","minLength":1,"description":"本机根目录；仅用于本次读取元数据与计算 sha256，不写入记录"},"relative_path":{"type":"string","minLength":1,"description":"相对 root_dir 的文件路径；不得包含 .. 或绝对路径"},"path":{"type":"string","minLength":1,"description":"本机文件绝对路径；与 root_dir 一起用于推导 relative_path"},"run_id":{"type":"string","minLength":1},"task_key":{"type":"string","minLength":1,"maxLength":1000},"sample_id":{"type":["string","null"],"maxLength":200},"well_id":{"type":["string","null"],"maxLength":40},"timepoint":{"type":["string","null"],"maxLength":100},"fov_index":{"type":["integer","null"],"minimum":1},"actor_id":{"type":["string","null"],"maxLength":200}}})
        }
        "asset.link.update" => {
            json!({"type":"object","additionalProperties":false,"required":["id","well_id","timepoint","fov_index"],"properties":{"id":{"type":"string","minLength":1,"description":"asset_link 记录 ID"},"well_id":{"type":"string","minLength":1,"maxLength":40,"description":"新孔位，如 A1；必填，不能用来清空坐标"},"timepoint":{"type":["string","null"],"maxLength":100,"description":"新时间点；显式 null 表示该链接不记录时间点"},"fov_index":{"type":["integer","null"],"minimum":1,"description":"新视野序号，从 1 开始；显式 null 表示不记录视野"},"actor_id":{"type":["string","null"],"maxLength":200}}})
        }
        "asset.unlink" => {
            json!({"type":"object","additionalProperties":false,"required":["id"],"properties":{"id":{"type":"string","minLength":1,"description":"asset_link 记录 ID"},"actor_id":{"type":["string","null"],"maxLength":200}}})
        }
        "asset.list" => {
            json!({"type":"object","additionalProperties":false,"required":["run_id"],"properties":{"run_id":{"type":"string","minLength":1}}})
        }
        "asset.events" => {
            json!({"type":"object","additionalProperties":false,"required":["run_id"],"properties":{"run_id":{"type":"string","minLength":1},"action":{"type":"string","enum":["linked","unlinked","updated"]}}})
        }
        "import.study.preview" | "import.study.apply" => {
            json!({"type":"object","additionalProperties":false,"required":["project"],"properties":{"project":{"type":"object","additionalProperties":false,"required":["name"],"properties":{"name":{"type":"string","minLength":1,"maxLength":120,"pattern":"\\S"},"description":{"type":"string"}}},"designs":{"type":"array","maxItems":500,"items":design_schema()},"protocols":{"type":"array","maxItems":500,"items":{"type":"object","additionalProperties":false,"required":["title","body"],"properties":{"title":{"type":"string","minLength":1,"maxLength":160,"pattern":"\\S"},"body":{"type":"string"}}}}}})
        }
        "calculator.dilution" => {
            json!({"type":"object","additionalProperties":false,"required":["stock_concentration","stock_unit","target_concentration","target_unit","final_volume","volume_unit"],"properties":{"stock_concentration":{"type":"number","exclusiveMinimum":0},"stock_unit":{"type":"string","enum":["M","mM","µM","nM","mg/mL","µg/mL","ng/µL"],"description":"摩尔或质量浓度；两者不能混用"},"target_concentration":{"type":"number","minimum":0},"target_unit":{"type":"string","enum":["M","mM","µM","nM","mg/mL","µg/mL","ng/µL"],"description":"必须与 stock_unit 同一量纲"},"final_volume":{"type":"number","exclusiveMinimum":0},"volume_unit":{"type":"string","enum":["L","mL","µL"]}}})
        }
        "calculator.solution" => {
            json!({"type":"object","additionalProperties":false,"required":["concentration_mol_l","final_volume","volume_unit","molecular_weight_g_mol"],"properties":{"concentration_mol_l":{"type":"number","exclusiveMinimum":0},"final_volume":{"type":"number","exclusiveMinimum":0},"volume_unit":{"type":"string","enum":["L","mL","µL"]},"molecular_weight_g_mol":{"type":"number","exclusiveMinimum":0}}})
        }
        "calculator.serial_dilution" => {
            json!({"type":"object","additionalProperties":false,"required":["initial_concentration","concentration_unit","transfer_volume","diluent_volume","steps"],"properties":{"initial_concentration":{"type":"number","exclusiveMinimum":0},"concentration_unit":{"type":"string","enum":["M","mM","µM","nM"]},"transfer_volume":{"type":"number","exclusiveMinimum":0},"diluent_volume":{"type":"number","minimum":0},"steps":{"type":"integer","minimum":1,"maximum":30}}})
        }
        "calculator.reaction_volume" => {
            json!({"type":"object","additionalProperties":false,"required":["target_volume_ul","components"],"properties":{"target_volume_ul":{"type":"number","exclusiveMinimum":0},"components":{"type":"array","maxItems":100,"items":{"type":"object","additionalProperties":false,"required":["name","volume_ul"],"properties":{"name":{"type":"string","minLength":1,"maxLength":120,"pattern":"\\S"},"volume_ul":{"type":"number","minimum":0}}}}}})
        }
        "calculator.gel_mastermix" => {
            json!({"type":"object","additionalProperties":false,"required":["volume_per_well_ul","lap_stock_percent_wv","lap_final_percent_wv","groups"],"properties":{"volume_per_well_ul":{"type":"number","exclusiveMinimum":0},"excess_ratio":{"type":"number","minimum":0},"lap_stock_percent_wv":{"type":"number","exclusiveMinimum":0},"lap_final_percent_wv":{"type":"number","minimum":0},"groups":{"type":"array","minItems":1,"maxItems":100,"items":{"type":"object","additionalProperties":false,"required":["name","gelma_percent_wv","wells"],"properties":{"name":{"type":"string","minLength":1,"maxLength":120,"pattern":"\\S"},"gelma_percent_wv":{"type":"number","exclusiveMinimum":0},"wells":{"type":"integer","minimum":1,"maximum":100000}}}}}})
        }
        "calculator.cell_seeding" => {
            json!({"type":"object","additionalProperties":false,"required":["density_cells_per_well","volume_per_well_ul","groups"],"properties":{"density_cells_per_well":{"type":"number","exclusiveMinimum":0},"volume_per_well_ul":{"type":"number","exclusiveMinimum":0},"excess_ratio":{"type":"number","minimum":0},"factor_stock_mg_per_ml":{"type":"number","minimum":0},"factor_final_mg_per_ml":{"type":"number","minimum":0},"groups":{"type":"array","minItems":1,"maxItems":100,"items":{"type":"object","additionalProperties":false,"required":["name","wells"],"properties":{"name":{"type":"string","minLength":1,"maxLength":120,"pattern":"\\S"},"wells":{"type":"integer","minimum":1,"maximum":100000}}}}}})
        }
        "calculator.runtime_normalize" => {
            json!({"type":"object","additionalProperties":false,"required":["planned_wells","actual_wells","volume_per_well_ul","dead_volume_ul","prepared_volume_ul"],"properties":{"planned_wells":{"type":"integer","minimum":0,"maximum":100000},"actual_wells":{"type":"integer","minimum":0,"maximum":100000},"volume_per_well_ul":{"type":"number","exclusiveMinimum":0},"excess_ratio":{"type":"number","minimum":0},"dead_volume_ul":{"type":"number","minimum":0},"prepared_volume_ul":{"type":"number","minimum":0}}})
        }
        "calculator.sample_normalize_batch" => {
            json!({"type":"object","additionalProperties":false,"required":["target_mass_ng","reaction_volume_ul","samples"],"properties":{"target_mass_ng":{"type":"number","exclusiveMinimum":0,"description":"每个样本需贡献到一次反应的质量（ng）"},"reaction_volume_ul":{"type":"number","exclusiveMinimum":0,"description":"该反应允许占用的总体积（µL），也是样本体积上限"},"samples":{"type":"array","minItems":1,"maxItems":100,"items":{"type":"object","additionalProperties":false,"required":["name","concentration","unit"],"properties":{"name":{"type":"string","minLength":1,"maxLength":120,"pattern":"\\S"},"concentration":{"type":"number","exclusiveMinimum":0},"unit":{"type":"string","enum":["mg/mL","µg/mL","ng/µL"],"description":"实测质量浓度；摩尔单位不属于本工具"}}}}}})
        }
        "calculator.in_vivo_dosing" => {
            json!({"type":"object","additionalProperties":false,"required":["body_weights_g","dose_mg_per_kg","dose_volume_ml_per_kg","stock_concentration","stock_unit"],"properties":{"body_weights_g":{"type":"array","minItems":1,"maxItems":200,"items":{"type":"number","exclusiveMinimum":0},"description":"每只动物的实测体重（g）；不按组均值估算"},"dose_mg_per_kg":{"type":"number","exclusiveMinimum":0},"dose_volume_ml_per_kg":{"type":"number","exclusiveMinimum":0},"overage_ratio":{"type":"number","minimum":0,"description":"配制管余量比例；只放大制备量，不放大单次注射量"},"stock_concentration":{"type":"number","exclusiveMinimum":0},"stock_unit":{"type":"string","enum":["mg/mL","µg/mL","ng/µL"]}}})
        }
        "calculator.biofilm_cfu" => {
            json!({"type":"object","additionalProperties":false,"required":["plates","countable_min","countable_max"],"properties":{"plates":{"type":"array","minItems":1,"maxItems":100,"items":{"type":"object","additionalProperties":false,"required":["raw_id","sample_id","colonies","dilution_fractions","plated_volume","plated_unit","suspension_volume","suspension_unit"],"properties":{"raw_id":{"type":"string","minLength":1,"maxLength":160,"pattern":"\\S"},"sample_id":{"type":"string","minLength":1,"maxLength":120,"pattern":"\\S"},"colonies":{"type":"number","minimum":0},"dilution_fractions":{"type":"array","minItems":1,"maxItems":30,"items":{"type":"number","exclusiveMinimum":0,"maximum":1}},"plated_volume":{"type":"number","exclusiveMinimum":0},"plated_unit":{"type":"string","enum":["L","mL","µL"]},"suspension_volume":{"type":"number","exclusiveMinimum":0},"suspension_unit":{"type":"string","enum":["L","mL","µL"]}}}},"countable_min":{"type":"number","minimum":0,"description":"计数下限，含边界"},"countable_max":{"type":"number","minimum":0,"description":"计数上限，含边界"}}})
        }
        "sdk.modules" => {
            json!({"type":"object","additionalProperties":false,"properties":{}})
        }
        "sdk.describe" => {
            json!({"type":"object","additionalProperties":false,"required":["kind","schema"],"properties":{"kind":{"type":"string"},"schema":{"type":"integer","minimum":1}}})
        }
        "sdk.validate" => {
            json!({"type":"object","additionalProperties":false,"properties":{"source":{"type":"string","minLength":1},"design_id":{"type":"string","minLength":1},"revision":{"type":"integer","minimum":0}}})
        }
        "asterdoc.inspect" => {
            json!({"type":"object","additionalProperties":false,"required":["design_id"],"properties":{"design_id":{"type":"string"},"revision":{"type":"integer","minimum":1},"module_id":{"type":"string"}}})
        }
        "asterdoc.assess" => {
            json!({"type":"object","additionalProperties":false,"required":["design_id"],"properties":{"design_id":{"type":"string","minLength":1},"revision":{"type":"integer","minimum":1}}})
        }

        "asterdoc.validate" => {
            json!({"type":"object","additionalProperties":false,"required":["source"],"properties":{"source":{"type":"string","minLength":1}}})
        }
        "asterdoc.load" => {
            json!({"type":"object","additionalProperties":false,"required":["design_id"],"properties":{"design_id":{"type":"string","minLength":1}}})
        }
        "asterdoc.save" => {
            json!({"type":"object","additionalProperties":false,"required":["design_id","source"],"properties":{"design_id":{"type":"string","minLength":1},"source":{"type":"string","maxLength":400000},"expected_revision":{"type":"integer","minimum":0},"operation_id":{"type":"string","minLength":1},"actor_id":{"type":"string","minLength":1}}})
        }
        "asterdoc.command" => {
            json!({"type":"object","additionalProperties":false,"required":["design_id","expected_revision","command"],"properties":{
                "design_id":{"type":"string","minLength":1},
                "expected_revision":{"type":"integer","minimum":0},
                "operation_id":{"type":"string","minLength":1},
                "actor_id":{"type":"string","minLength":1},
                "command":{"oneOf":[
                    {"type":"object","additionalProperties":false,"required":["type","module"],"properties":{"type":{"const":"add_module"},"module":{"type":"object","required":["kind","id","schema"],"properties":{"kind":{"type":"string"},"id":{"type":"string","minLength":1},"schema":{"type":"integer","minimum":1}}}}},
                    {"type":"object","additionalProperties":false,"required":["type","module"],"properties":{"type":{"const":"add_step"},"workflowId":{"type":"string"},"module":{"type":"object","required":["kind","id","schema"],"properties":{"kind":{"const":"step"},"id":{"type":"string","minLength":1},"schema":{"type":"integer","minimum":1}}}}},
                    {"type":"object","additionalProperties":false,"required":["type","moduleId"],"properties":{"type":{"enum":["remove_module","remove_step","promote_step"]},"moduleId":{"type":"string","minLength":1}}},
                    {"type":"object","additionalProperties":false,"required":["type","moduleId","parentId"],"properties":{"type":{"const":"nest_step"},"moduleId":{"type":"string","minLength":1},"parentId":{"type":"string","minLength":1}}},
                    {"type":"object","additionalProperties":false,"required":["type","moduleId","order"],"properties":{"type":{"const":"reorder_steps"},"moduleId":{"type":"string","minLength":1},"order":{"type":"array","minItems":1,"items":{"type":"string","minLength":1}}}},
                    {"type":"object","additionalProperties":false,"required":["type","moduleId","properties"],"properties":{"type":{"const":"update_module"},"moduleId":{"type":"string","minLength":1},"properties":{"type":"object"}}},
                    {"type":"object","additionalProperties":false,"required":["type","moduleId","group","wells"],"properties":{"type":{"const":"assign_wells"},"moduleId":{"type":"string","minLength":1},"group":{"type":"string","minLength":1},"wells":{"type":"array","minItems":1,"items":{"type":"string","minLength":1}}}},
                    {"type":"object","additionalProperties":false,"required":["type","moduleId","wells"],"properties":{"type":{"const":"clear_wells"},"moduleId":{"type":"string","minLength":1},"wells":{"type":"array","minItems":1,"items":{"type":"string","minLength":1}}}},
                    {"type":"object","additionalProperties":false,"required":["type","moduleId","from","to"],"properties":{"type":{"enum":["connect_steps","disconnect_steps"]},"moduleId":{"type":"string","minLength":1},"from":{"type":"string","minLength":1},"to":{"type":"string","minLength":1}}}
                ]}
            }})
        }
        "asterdoc.compile" => {
            json!({"type":"object","additionalProperties":false,"required":["design_id","revision"],"properties":{"design_id":{"type":"string","minLength":1},"revision":{"type":"integer","minimum":1}}})
        }
        "asterdoc.create_run" => {
            json!({"type":"object","additionalProperties":false,"required":["design_id","revision"],"properties":{"design_id":{"type":"string","minLength":1},"revision":{"type":"integer","minimum":1},"operation_id":{"type":"string","minLength":1},"title":{"type":"string","minLength":1},"acknowledged_warnings":{"type":"array","items":{"type":"string"}}}})
        }
        _ => return None,
    };
    if tool == "execution_event.create" {
        s["properties"]["event_type"]["enum"]
            .as_array_mut()
            .expect("execution_event.create event_type enum")
            .push(json!("skipped_by_rule"));
        s["properties"]["ad_hoc_step"] = json!({"type":"boolean"});
    }
    Some(s)
}
pub(crate) fn design_schema() -> Value {
    json!({
        "type":"object","additionalProperties":false,"required":["title"],
        "properties":{
        "title":{"type":"string","minLength":1,"maxLength":160,"pattern":"\\S","description":"简短识别名称。"},
            "research_question":{"type":["string","null"],"maxLength":5000,"description":"研究者明确提出的研究问题；未知时留空。"},
            "factors":{"type":"array","maxItems":50,"items":{"type":"object","additionalProperties":false,"required":["name","levels"],"properties":{"name":{"type":"string","minLength":1,"maxLength":120,"pattern":"\\S"},"levels":{"type":"array","minItems":1,"maxItems":50,"items":{"type":"string","minLength":1,"maxLength":500,"pattern":"\\S"}}}}},
            "objective":{"type":"string","maxLength":5000,"description":"本实验实际要回答的研究问题；未知时留空。"},
            "experimental_unit":{"type":"string","maxLength":500,"description":"独立观测单位的明确定义；必须由研究者确定。"},
            "group_specs":{"type":"array","maxItems":100,"description":"已定义实验组。id 是稳定引用；independent_n 是每组独立实验单位数，不能用技术重复代替。","items":{"type":"object","additionalProperties":false,"required":["name"],"properties":{
                "id":{"type":"string","maxLength":80},"name":{"type":"string","minLength":1,"maxLength":120,"pattern":"\\S"},"role":{"type":["string","null"],"enum":["control","treatment","comparator","reference","other",null]},
                "condition":{"type":["string","null"],"maxLength":500},"independent_n":{"type":["integer","null"],"minimum":1},"technical_repeats":{"type":["integer","null"],"minimum":1}
            }}},
            "timepoints":{"type":"array","maxItems":100,"items":{"type":"string","minLength":1,"maxLength":100,"pattern":"\\S"}},
            "primary_endpoint":{"type":"string","maxLength":1000},"endpoint_method":{"type":"string","maxLength":1000},"endpoint_unit":{"type":"string","maxLength":100},
            "sample_size_rationale":{"type":"string","maxLength":5000},"randomization":{"type":"string","maxLength":2000},"blinding":{"type":"string","maxLength":2000},
            "inclusion_criteria":{"type":"string","maxLength":5000},"exclusion_criteria":{"type":"string","maxLength":5000},"analysis_plan":{"type":"string","maxLength":5000},"controls":{"type":"string","maxLength":2000},
            "assays":{"type":"array","maxItems":100,"items":{"type":"object","additionalProperties":false,"required":["id","title"],"properties":{"id":{"type":"string","minLength":1,"maxLength":80},"title":{"type":"string","minLength":1,"maxLength":160,"pattern":"\\S"},"endpoint":{"type":"string","maxLength":1000},"method":{"type":"string","maxLength":1000},"unit":{"type":"string","maxLength":100},"timepoints":{"type":"array","maxItems":100,"items":{"type":"string","maxLength":100}},"timepoint_timing":{"type":"array","maxItems":100,"items":timepoint_timing_input_schema()},"group_names":{"type":"array","maxItems":100,"items":{"type":"string","maxLength":120}},"group_ids":{"type":"array","maxItems":100,"items":{"type":"string","maxLength":80}},"time_anchor_event":{"type":["string","null"],"maxLength":120},"sample_use_policy":{"description":"Legacy input only; normalized output uses timepoint_relation.","type":["string","null"],"enum":["new_per_timepoint","same_sample_repeated","split_from_assay","sequential_after_assay","unknown",null]},"timepoint_relation":{"type":["string","null"],"enum":["same_experimental_unit_repeated","same_physical_sample_repeated","independent_sample_per_timepoint","unknown",null]},"container_type":{"type":["string","null"],"maxLength":80},"container_format":{"type":["string","null"],"maxLength":80},"protocol_version_id":{"type":["string","null"]},"controls":{"type":"array","maxItems":100,"items":{"type":"string","maxLength":300}},"control_group_ids":{"type":"array","maxItems":100,"items":{"type":"string","maxLength":80}}}}},
            "sample_allocations":{"type":"array","maxItems":10000,"items":{"type":"object","additionalProperties":false,"required":["assay_id","group_name"],"properties":{"assay_id":{"type":"string"},"group_name":{"type":"string"},"group_id":{"type":["string","null"]},"relation":{"description":"Legacy input only; normalized output uses timepoint_relation and source_relation.","type":"string","enum":["new_per_timepoint","same_sample_repeated","split_from_assay","sequential_after_assay","unknown"]},"source_relation":{"type":["string","null"],"enum":["split_from_assay","sequential_after_assay","not_applicable","unknown",null]},"timepoint_relation":{"type":["string","null"],"enum":["same_experimental_unit_repeated","same_physical_sample_repeated","independent_sample_per_timepoint","unknown",null]},"source_assay_id":{"type":["string","null"]},"physical_slots_per_group":{"type":["integer","null"],"minimum":1},"physical_slots_scope":{"type":["string","null"],"enum":["per_timepoint","whole_assay_unique_positions","unknown",null]},"evidence_id":{"type":["string","null"]},"planned_output_id":{"type":["string","null"]}}}},
            "container_plan":{"type":"object","additionalProperties":true},
            "planned_outputs":{"type":"array","maxItems":1000,"items":{"type":"object","additionalProperties":false,"required":["id","title","protocol_version_id"],"properties":{"id":{"type":"string","minLength":1,"maxLength":80},"title":{"type":"string","minLength":1,"maxLength":160,"pattern":"\\S"},"protocol_version_id":{"type":"string","minLength":1,"maxLength":80},"root_bindings":{"type":"array","maxItems":100,"items":{"type":"object","additionalProperties":false,"required":["slot","value"],"properties":{"slot":{"type":"string","minLength":1,"maxLength":100},"value":{"type":["string","number","integer","boolean"]},"unit":{"type":["string","null"],"maxLength":40}}}},"evidence_id":{"type":["string","null"]}}}},
            "source_evidence":{"type":"array","maxItems":2000,"items":{"type":"object","additionalProperties":false,"required":["id","source_document","claim","state"],"properties":{"id":{"type":"string","minLength":1,"maxLength":80},"source_document":{"type":"string","minLength":1,"maxLength":500},"source_date":{"type":["string","null"],"maxLength":40},"source_locator":{"type":["string","null"],"maxLength":500},"claim":{"type":"string","minLength":1,"maxLength":2000},"state":{"type":"string","enum":["source_record","proposal","user_confirmed","calculated","agent_suggestion"]}}}},
            "evidence_conflicts":{"type":"array","maxItems":1000,"items":{"type":"object","additionalProperties":false,"required":["id","topic","evidence_ids","status"],"properties":{"id":{"type":"string","minLength":1,"maxLength":80},"topic":{"type":"string","minLength":1,"maxLength":500},"evidence_ids":{"type":"array","minItems":2,"items":{"type":"string"}},"status":{"type":"string","enum":["unresolved","resolved"]},"affected_assay_ids":{"type":"array","maxItems":100,"items":{"type":"string","minLength":1,"maxLength":80}},"affects_container_estimate":{"type":"boolean"},"resolution":{"type":["string","null"],"maxLength":2000}}}},
            "planned_run_count":{"type":["integer","null"],"minimum":1,"maximum":1000},"planned_run_relationship":{"type":"string","enum":["independent","same_source","paired","unknown"]},
            "groups":{"type":"string"},"hypothesis":{"type":"string"},"replicate_plan":{"type":"string"},"independent_variables":{"type":"string"}
        }
    })
}
pub(crate) fn schema(tool: &str) -> Option<Value> {
    input_schema(tool)
}
pub(crate) fn parameter_slot_input_schema() -> Value {
    json!({
        "type":"object","additionalProperties":false,
        "required":["key","label","value_type"],
        "properties":{
            "key":{"type":"string","pattern":"^[A-Za-z][A-Za-z0-9_]*$","maxLength":80},
            "label":{"type":"string","minLength":1,"maxLength":120,"pattern":"\\S"},
            "value_type":{"type":"string","enum":["text","number","integer","boolean"]},
            "unit":{"type":"string","maxLength":40},
            "required":{"type":"boolean"}
        }
    })
}
pub(crate) fn step_binding_input_schema() -> Value {
    json!({
        "type":"object","additionalProperties":false,
        "required":["slot"],"anyOf":[{"required":["value"]},{"required":["source_slot"]}],
        "properties":{
            "slot":{"type":"string"},
            "value":{"type":["string","number","integer","boolean"]},
            "source_slot":{"type":"string"},
            "unit":{"type":"string","maxLength":40}
        }
    })
}
pub(crate) fn text_step_input_schema() -> Value {
    json!({"type":"object","additionalProperties":false,"required":["kind","use_id","title","instruction"],"properties":{
        "kind":{"const":"text"},"use_id":{"type":"string","minLength":1,"maxLength":80},
        "title":{"type":"string","minLength":1,"maxLength":200,"pattern":"\\S"},
        "instruction":{"type":"string","minLength":1,"maxLength":4000,"pattern":"\\S"},
        "parameter_refs":{"type":"array","maxItems":100,"items":{"type":"string"}},
        "visualization":{"type":"string","enum":["transfer","incubation","plate_layout","sample_split","measurement"]},"time_scope":{"type":"string","enum":["once","per_timepoint","per_assay","unresolved"]},
        "checks":{"type":"array","maxItems":50,"items":{"type":"object","additionalProperties":false,"required":["check_id","label"],"properties":{"check_id":{"type":"string","minLength":1,"maxLength":80,"pattern":"\\S"},"label":{"type":"string","minLength":1,"maxLength":200,"pattern":"\\S"},"instruction":{"type":"string","maxLength":2000},"blocking":{"type":"boolean"}}}}
    }})
}
pub(crate) fn protocol_step_input_schema() -> Value {
    json!({"oneOf":[
        text_step_input_schema(),
        {"type":"object","additionalProperties":false,"required":["kind","use_id","block_version_id"],"properties":{
            "kind":{"const":"block_use"},"use_id":{"type":"string","minLength":1,"maxLength":80},
            "block_version_id":{"type":"string"},"bindings":{"type":"array","maxItems":100,"items":step_binding_input_schema()}
        }}
    ]})
}
pub(crate) fn step_edit_op_input_schema() -> Value {
    json!({"oneOf":[
        {"type":"object","additionalProperties":false,"required":["op","path","position","step"],"properties":{
            "op":{"const":"insert"},"path":{"type":"string","minLength":1},"position":{"type":"integer","minimum":0},
            "step":text_step_input_schema()
        }},
        {"type":"object","additionalProperties":false,"required":["op","path","order"],"properties":{
            "op":{"const":"reorder"},"path":{"type":"string","minLength":1},
            "order":{"type":"array","minItems":1,"maxItems":100,"items":{"type":"string","minLength":1}}
        }},
        {"type":"object","additionalProperties":false,"required":["op","path","position","use_id","block_version_id"],"properties":{
            "op":{"const":"reuse"},"path":{"type":"string","minLength":1},"position":{"type":"integer","minimum":0},
            "use_id":{"type":"string","minLength":1,"maxLength":80},"block_version_id":{"type":"string","minLength":1},
            "bindings":{"type":"array","maxItems":100,"items":step_binding_input_schema()}
        }},
        {"type":"object","additionalProperties":false,"required":["op","path","bindings"],"properties":{
            "op":{"const":"bind"},"path":{"type":"string","minLength":1},
            "bindings":{"type":"array","minItems":1,"maxItems":100,"items":step_binding_input_schema()}
        }}
    ]})
}
pub(crate) fn root_parameter_values_input_schema() -> Value {
    json!({"type":"array","maxItems":100,"items":{"type":"object","additionalProperties":false,"required":["slot","value"],"properties":{
        "slot":{"type":"string"},"value":{"type":["string","number","integer","boolean"]},"unit":{"type":"string","maxLength":40}
    }}})
}
pub(crate) fn timepoint_timing_input_schema() -> Value {
    json!({
        "type":"object",
        "additionalProperties":false,
        "required":["timepoint","unit"],
        "properties":{
            "timepoint":{"type":"string","minLength":1,"maxLength":100,"pattern":"\\S"},
            "offset_value":{"type":["number","null"]},
            "unit":{"type":"string","enum":["min","h","day"]},
            "window_before":{"type":["number","null"],"minimum":0},
            "window_after":{"type":["number","null"],"minimum":0}
        }
    })
}
