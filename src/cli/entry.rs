use super::*;

pub fn run(args: &[String], data_dir: &Path) -> i32 {
    if args.is_empty() || args[0] == "--help" || args[0] == "-h" {
        print_help();
        return 0;
    }
    if args[0] == "tools" {
        if args.len() == 2 && args[1] == "--json" {
            println!("{}", json!({"ok":true,"tools":tool_specs()}));
            return 0;
        }
        print_help();
        return 0;
    }
    if args[0] == "schema" {
        if args.len() != 2 {
            return fail(
                "schema",
                CliError::input("格式：labworkbench schema <tool>"),
                false,
            );
        }
        return match schema(&args[1]) {
            Some(s) => {
                println!(
                    "{}",
                    json!({"ok":true,"tool":args[1],"input_schema":s,"output_schema":output_schema(&args[1])})
                );
                0
            }
            None => fail(
                &args[1],
                CliError::missing("未知工具；请运行 `labworkbench tools --json`"),
                false,
            ),
        };
    }
    let tool = args[0].as_str();
    let requested_diagnostics = args.iter().any(|arg| arg == "--diagnostics");
    let invocation = match parse_args(&args[1..]) {
        Ok(v) => v,
        Err(e) => return fail(tool, e, requested_diagnostics),
    };
    let result = dispatch(tool, invocation.input, data_dir);
    match result {
        Ok(v) => {
            output(tool, v, invocation.diagnostics);
            0
        }
        Err(e) => fail(tool, e, invocation.diagnostics),
    }
}
pub(crate) fn fail(tool: &str, e: CliError, diagnostics: bool) -> i32 {
    let exit = e.exit;
    println!("{}", error_envelope(tool, &e, diagnostics));
    exit
}

pub(crate) fn error_envelope(tool: &str, e: &CliError, diagnostics: bool) -> Value {
    let mut error = json!({"code":e.code,"message":e.message});
    if let Some(path) = &e.path {
        error["path"] = json!(path);
    }
    if let Some(current_version) = e.current_version {
        error["current_version"] = json!(current_version);
    }
    if let Some(current_version_id) = &e.current_version_id {
        error["current_version_id"] = json!(current_version_id);
    }
    if !e.issues.is_empty() {
        error["issues"] = Value::Array(e.issues.clone());
    }
    let mut envelope = json!({"ok":false,"tool":tool,"error":error});
    if diagnostics {
        envelope["diagnostics"] = Value::Array(cli_diagnostics(e));
    }
    envelope
}

/// Normalize the legacy failure shape into structured diagnostics. Declared
/// `issues` win; otherwise the error itself becomes one blocking diagnostic.
pub(crate) fn cli_diagnostics(e: &CliError) -> Vec<Value> {
    let mut diagnostics: Vec<Diagnostic> = e
        .issues
        .iter()
        .map(Diagnostic::from_legacy_json)
        .collect();
    if diagnostics.is_empty() {
        diagnostics.push(
            Diagnostic::new(
                DiagnosticSeverity::Blocking,
                e.code,
                e.path.clone().unwrap_or_default(),
                e.message.clone(),
            )
            .with_next_action(next_action_for(e.code)),
        );
    }
    diagnostics
        .iter()
        .filter_map(|diagnostic| serde_json::to_value(diagnostic).ok())
        .collect()
}

pub(crate) fn next_action_for(code: &str) -> &'static str {
    match code {
        "REVISION_CONFLICT" | "VERSION_CONFLICT" => "重新读取当前 revision 后重试",
        "NOT_FOUND" => "确认对象 id 存在后重试",
        "INVALID_INPUT" | "INVALID_PROPOSAL" | "INVALID_BINDING" | "UNIT_MISMATCH" => {
            "修正输入字段后重试"
        }
        "INTERNAL" => "记录 tool 与输入，报告内部错误",
        _ => "按 error.diagnostics 修正后重试",
    }
}
pub(crate) fn print_help() {
    println!("AsterLab typed CLI\n\n  labworkbench tools --json\n  labworkbench schema <tool>\n  labworkbench <tool> --input <json-file|->\n\nAll tool inputs use a strict per-command schema; unknown fields are rejected. All output is one JSON envelope on stdout. Data directory follows LABWORKBENCH_DATA_DIR (default: ./data).");
}

pub(crate) fn require_trusted_writer(tool: &str) -> Result<(), CliError> {
    if std::env::var("LABWORKBENCH_TRUSTED_WRITER").ok().as_deref() == Some("1") {
        Ok(())
    } else {
        Err(CliError::input(format!(
            "{tool} 只允许受信任的本地写入端调用；请设置 LABWORKBENCH_TRUSTED_WRITER=1"
        )))
    }
}

