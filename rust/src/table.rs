// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: Copyright The LanceDB Authors

//! Table management operations

use crate::ffi::{from_c_str, SimpleResult};
use crate::runtime::get_simple_runtime;
use crate::schema::create_arrow_schema_from_json;
use chrono::TimeDelta;
use lancedb::table::{CompactionOptions, OptimizeAction, OptimizeOptions};
use std::ffi::CString;
use std::os::raw::{c_char, c_void};
use std::sync::Arc;

/// Create a table with a simple JSON schema
#[no_mangle]
pub extern "C" fn simple_lancedb_create_table(
    handle: *mut c_void,
    table_name: *const c_char,
    schema_json: *const c_char,
) -> *mut SimpleResult {
    let result = std::panic::catch_unwind(|| -> SimpleResult {
        if handle.is_null() || table_name.is_null() || schema_json.is_null() {
            return SimpleResult::error("Invalid null arguments".to_string());
        }

        let name = match from_c_str(table_name) {
            Ok(s) => s,
            Err(e) => return SimpleResult::error(format!("Invalid table name: {}", e)),
        };

        let schema_str = match from_c_str(schema_json) {
            Ok(s) => s,
            Err(e) => return SimpleResult::error(format!("Invalid schema JSON: {}", e)),
        };

        let conn = unsafe { &*(handle as *const lancedb::Connection) };
        let rt = get_simple_runtime();

        // Parse the JSON schema and create an Arrow schema
        match serde_json::from_str::<serde_json::Value>(&schema_str) {
            Ok(schema_json_value) => match create_arrow_schema_from_json(&schema_json_value) {
                Ok(arrow_schema) => {
                    match rt.block_on(async {
                        use arrow_array::RecordBatchIterator;
                        let empty_batches = RecordBatchIterator::new(
                            vec![]
                                as Vec<Result<arrow_array::RecordBatch, arrow_schema::ArrowError>>,
                            Arc::new(arrow_schema),
                        );
                        conn.create_table(&name, empty_batches).execute().await
                    }) {
                        Ok(_) => SimpleResult::ok(),
                        Err(e) => SimpleResult::error(format!("Failed to create table: {}", e)),
                    }
                }
                Err(e) => SimpleResult::error(format!("Failed to create Arrow schema: {}", e)),
            },
            Err(e) => SimpleResult::error(format!("Failed to parse schema JSON: {}", e)),
        }
    });

    match result {
        Ok(res) => Box::into_raw(Box::new(res)),
        Err(_) => Box::into_raw(Box::new(SimpleResult::error(
            "Panic in simple_lancedb_create_table".to_string(),
        ))),
    }
}

/// Create a table with Arrow IPC schema (more efficient than JSON)
#[no_mangle]
#[allow(clippy::not_unsafe_ptr_arg_deref)]
pub extern "C" fn simple_lancedb_create_table_with_ipc(
    handle: *mut c_void,
    table_name: *const c_char,
    schema_ipc: *const u8,
    schema_len: usize,
) -> *mut SimpleResult {
    let result = std::panic::catch_unwind(|| -> SimpleResult {
        if handle.is_null() || table_name.is_null() || schema_ipc.is_null() {
            return SimpleResult::error("Invalid null arguments".to_string());
        }

        let name = match from_c_str(table_name) {
            Ok(s) => s,
            Err(e) => return SimpleResult::error(format!("Invalid table name: {}", e)),
        };

        // Convert raw pointer to slice
        let schema_bytes = unsafe { std::slice::from_raw_parts(schema_ipc, schema_len) };

        let conn = unsafe { &*(handle as *const lancedb::Connection) };
        let rt = get_simple_runtime();

        // Deserialize Arrow schema directly from IPC bytes using FileReader
        let arrow_schema = match arrow_ipc::reader::FileReader::try_new(
            std::io::Cursor::new(schema_bytes),
            None,
        ) {
            Ok(reader) => reader.schema(),
            Err(e) => return SimpleResult::error(format!("Invalid IPC schema: {}", e)),
        };

        match rt.block_on(async {
            use arrow_array::RecordBatchIterator;
            let empty_batches = RecordBatchIterator::new(
                vec![] as Vec<Result<arrow_array::RecordBatch, arrow_schema::ArrowError>>,
                arrow_schema, // arrow_schema is already Arc<Schema>
            );
            conn.create_table(&name, empty_batches).execute().await
        }) {
            Ok(_) => SimpleResult::ok(),
            Err(e) => SimpleResult::error(format!("Failed to create table: {}", e)),
        }
    });

    match result {
        Ok(res) => Box::into_raw(Box::new(res)),
        Err(_) => Box::into_raw(Box::new(SimpleResult::error(
            "Panic in simple_lancedb_create_table_with_ipc".to_string(),
        ))),
    }
}

/// Drop a table from the database (simple version)
#[no_mangle]
pub extern "C" fn simple_lancedb_drop_table(
    handle: *mut c_void,
    table_name: *const c_char,
) -> *mut SimpleResult {
    let result = std::panic::catch_unwind(|| -> SimpleResult {
        if handle.is_null() || table_name.is_null() {
            return SimpleResult::error("Invalid null arguments".to_string());
        }

        let name = match from_c_str(table_name) {
            Ok(s) => s,
            Err(e) => return SimpleResult::error(format!("Invalid table name: {}", e)),
        };

        let conn = unsafe { &*(handle as *const lancedb::Connection) };
        let rt = get_simple_runtime();

        match rt.block_on(async { conn.drop_table(&name, &[]).await }) {
            Ok(_) => SimpleResult::ok(),
            Err(e) => SimpleResult::error(format!("Failed to drop table: {}", e)),
        }
    });

    match result {
        Ok(res) => Box::into_raw(Box::new(res)),
        Err(_) => Box::into_raw(Box::new(SimpleResult::error(
            "Panic in simple_lancedb_drop_table".to_string(),
        ))),
    }
}

/// Open a table from the database (simple version)
#[no_mangle]
#[allow(clippy::not_unsafe_ptr_arg_deref)]
pub extern "C" fn simple_lancedb_open_table(
    handle: *mut c_void,
    table_name: *const c_char,
    table_handle: *mut *mut c_void,
) -> *mut SimpleResult {
    let result = std::panic::catch_unwind(|| -> SimpleResult {
        if handle.is_null() || table_name.is_null() || table_handle.is_null() {
            return SimpleResult::error("Invalid null arguments".to_string());
        }

        let name = match from_c_str(table_name) {
            Ok(s) => s,
            Err(e) => return SimpleResult::error(format!("Invalid table name: {}", e)),
        };

        let conn = unsafe { &*(handle as *const lancedb::Connection) };
        let rt = get_simple_runtime();

        match rt.block_on(async { conn.open_table(&name).execute().await }) {
            Ok(table) => {
                let boxed_table = Box::new(table);
                unsafe {
                    *table_handle = Box::into_raw(boxed_table) as *mut c_void;
                }
                SimpleResult::ok()
            }
            Err(e) => SimpleResult::error(format!("Failed to open table: {}", e)),
        }
    });

    match result {
        Ok(res) => Box::into_raw(Box::new(res)),
        Err(_) => Box::into_raw(Box::new(SimpleResult::error(
            "Panic in simple_lancedb_open_table".to_string(),
        ))),
    }
}

/// Close a table handle (simple version)
#[no_mangle]
pub extern "C" fn simple_lancedb_table_close(table_handle: *mut c_void) -> *mut SimpleResult {
    if table_handle.is_null() {
        return Box::into_raw(Box::new(SimpleResult::error(
            "Invalid null handle".to_string(),
        )));
    }

    let result = std::panic::catch_unwind(|| -> SimpleResult {
        unsafe {
            let _table = Box::from_raw(table_handle as *mut lancedb::Table);
            // Table will be dropped here, cleaning up resources
        }
        SimpleResult::ok()
    });

    match result {
        Ok(res) => Box::into_raw(Box::new(res)),
        Err(_) => Box::into_raw(Box::new(SimpleResult::error(
            "Panic in simple_lancedb_table_close".to_string(),
        ))),
    }
}

/// Optimize the on-disk data and indices for better performance
#[no_mangle]
#[allow(clippy::not_unsafe_ptr_arg_deref)]
pub extern "C" fn simple_lancedb_table_optimize(
    table_handle: *mut c_void,
    optimize_stats_json: *mut *mut c_char,
) -> *mut SimpleResult {
    let result = std::panic::catch_unwind(|| -> SimpleResult {
        if table_handle.is_null() || optimize_stats_json.is_null() {
            return SimpleResult::error("Invalid null arguments".to_string());
        }

        let table = unsafe { &*(table_handle as *const lancedb::Table) };
        let rt = get_simple_runtime();

        match rt.block_on(async { table.optimize(OptimizeAction::All).await }) {
            Ok(optimize_stats) => {
                let mut stats_json = serde_json::json!({});
                if let Some(compaction_stats) = optimize_stats.compaction {
                    stats_json["compaction"] = serde_json::json!({
                        "fragments_removed": compaction_stats.fragments_removed,
                        "fragments_added": compaction_stats.fragments_added,
                        "files_removed": compaction_stats.files_removed,
                        "files_added": compaction_stats.files_added,
                    });
                }
                if let Some(prune_stats) = optimize_stats.prune {
                    stats_json["prune"] = serde_json::json!({
                        "bytes_removed": prune_stats.bytes_removed,
                        "old_versions": prune_stats.old_versions,
                    });
                }

                match serde_json::to_string(&stats_json) {
                    Ok(json_str) => match CString::new(json_str) {
                        Ok(c_string) => {
                            unsafe {
                                *optimize_stats_json = c_string.into_raw();
                            }
                            SimpleResult::ok()
                        }
                        Err(_) => {
                            SimpleResult::error("Failed to convert JSON to C string".to_string())
                        }
                    },
                    Err(e) => SimpleResult::error(format!(
                        "Failed to serialize optimize stats to JSON: {}",
                        e
                    )),
                }
            }
            Err(e) => SimpleResult::error(format!("Failed to optimize table: {}", e)),
        }
    });

    match result {
        Ok(res) => Box::into_raw(Box::new(res)),
        Err(_) => Box::into_raw(Box::new(SimpleResult::error(
            "Panic in simple_lancedb_table_optimize".to_string(),
        ))),
    }
}

/// Build a lancedb::table::OptimizeAction from the action JSON. Shape:
///   {"type": "all"}                                  -> OptimizeAction::All
///   {"type": "compact",
///    "target_rows_per_fragment": <usize>?,
///    "max_rows_per_group": <usize>?,
///    "max_bytes_per_file": <usize>?,
///    "materialize_deletions": <bool>?,
///    "materialize_deletions_threshold": <f32>?,
///    "num_threads": <usize>?,
///    "batch_size": <usize>?}                          -> OptimizeAction::Compact
///   {"type": "prune",
///    "older_than_seconds": <u64>?,
///    "delete_unverified": <bool>?,
///    "error_if_tagged_old_versions": <bool>?}        -> OptimizeAction::Prune
///   {"type": "index"}                                 -> OptimizeAction::Index
///
/// Unknown fields per branch are ignored. Returns Err(String) on a
/// missing/unknown "type" or a value that doesn't fit the expected
/// integer width.
fn build_optimize_action(cfg: &serde_json::Value) -> Result<OptimizeAction, String> {
    let kind = cfg
        .get("type")
        .and_then(|v| v.as_str())
        .ok_or_else(|| "OptimizeAction config requires a 'type' field".to_string())?;

    let usize_opt = |k: &str| -> Result<Option<usize>, String> {
        match cfg.get(k).and_then(|v| v.as_u64()) {
            None => Ok(None),
            Some(n) => usize::try_from(n)
                .map(Some)
                .map_err(|_| format!("'{}' value {} does not fit in usize", k, n)),
        }
    };
    let bool_opt = |k: &str| -> Option<bool> { cfg.get(k).and_then(|v| v.as_bool()) };

    match kind {
        "all" => Ok(OptimizeAction::All),

        "compact" => {
            let mut options = CompactionOptions::default();
            if let Some(n) = usize_opt("target_rows_per_fragment")? {
                options.target_rows_per_fragment = n;
            }
            if let Some(n) = usize_opt("max_rows_per_group")? {
                options.max_rows_per_group = n;
            }
            if let Some(n) = usize_opt("max_bytes_per_file")? {
                options.max_bytes_per_file = Some(n);
            }
            if let Some(v) = bool_opt("materialize_deletions") {
                options.materialize_deletions = v;
            }
            if let Some(f) = cfg
                .get("materialize_deletions_threshold")
                .and_then(|v| v.as_f64())
            {
                options.materialize_deletions_threshold = f as f32;
            }
            if let Some(n) = usize_opt("num_threads")? {
                options.num_threads = Some(n);
            }
            if let Some(n) = usize_opt("batch_size")? {
                options.batch_size = Some(n);
            }
            Ok(OptimizeAction::Compact {
                options,
                remap_options: None,
            })
        }

        "prune" => {
            let older_than = match cfg.get("older_than_seconds").and_then(|v| v.as_u64()) {
                None => None,
                Some(secs) => {
                    // TimeDelta::try_seconds is fallible because the
                    // chrono internal range is finite; wrap with a clear
                    // error when the caller hands in something absurd.
                    let i = i64::try_from(secs).map_err(|_| {
                        format!("'older_than_seconds' value {} does not fit in i64", secs)
                    })?;
                    Some(TimeDelta::try_seconds(i).ok_or_else(|| {
                        format!(
                            "'older_than_seconds' value {} is out of TimeDelta range",
                            secs
                        )
                    })?)
                }
            };
            Ok(OptimizeAction::Prune {
                older_than,
                delete_unverified: bool_opt("delete_unverified"),
                error_if_tagged_old_versions: bool_opt("error_if_tagged_old_versions"),
            })
        }

        "index" => Ok(OptimizeAction::Index(OptimizeOptions::default())),

        other => Err(format!("Unknown OptimizeAction type: {}", other)),
    }
}

/// Optimize the on-disk data and indices with a configurable
/// OptimizeAction. The `action_json` argument selects the sub-action and
/// carries its options; `OptimizeAction::All` (the original behaviour
/// available via `simple_lancedb_table_optimize`) corresponds to
/// `{"type":"all"}`.
#[no_mangle]
#[allow(clippy::not_unsafe_ptr_arg_deref)]
pub extern "C" fn simple_lancedb_table_optimize_v2(
    table_handle: *mut c_void,
    action_json: *const c_char,
    optimize_stats_json: *mut *mut c_char,
) -> *mut SimpleResult {
    let result = std::panic::catch_unwind(|| -> SimpleResult {
        if table_handle.is_null() || action_json.is_null() || optimize_stats_json.is_null() {
            return SimpleResult::error("Invalid null arguments".to_string());
        }

        let cfg_str = match from_c_str(action_json) {
            Ok(s) => s,
            Err(e) => return SimpleResult::error(format!("Invalid action JSON: {}", e)),
        };
        let cfg: serde_json::Value = match serde_json::from_str(&cfg_str) {
            Ok(v) => v,
            Err(e) => return SimpleResult::error(format!("Failed to parse action JSON: {}", e)),
        };
        let action = match build_optimize_action(&cfg) {
            Ok(a) => a,
            Err(e) => return SimpleResult::error(e),
        };

        let table = unsafe { &*(table_handle as *const lancedb::Table) };
        let rt = get_simple_runtime();

        match rt.block_on(async { table.optimize(action).await }) {
            Ok(stats) => {
                let mut stats_json = serde_json::json!({});
                if let Some(c) = stats.compaction {
                    stats_json["compaction"] = serde_json::json!({
                        "fragments_removed": c.fragments_removed,
                        "fragments_added": c.fragments_added,
                        "files_removed": c.files_removed,
                        "files_added": c.files_added,
                    });
                }
                if let Some(p) = stats.prune {
                    stats_json["prune"] = serde_json::json!({
                        "bytes_removed": p.bytes_removed,
                        "old_versions": p.old_versions,
                    });
                }
                match serde_json::to_string(&stats_json) {
                    Ok(s) => match CString::new(s) {
                        Ok(c) => {
                            unsafe {
                                *optimize_stats_json = c.into_raw();
                            }
                            SimpleResult::ok()
                        }
                        Err(_) => {
                            SimpleResult::error("Failed to convert JSON to C string".to_string())
                        }
                    },
                    Err(e) => {
                        SimpleResult::error(format!("Failed to serialize optimize stats: {}", e))
                    }
                }
            }
            Err(e) => SimpleResult::error(format!("optimize_v2 failed: {}", e)),
        }
    });

    match result {
        Ok(res) => Box::into_raw(Box::new(res)),
        Err(_) => Box::into_raw(Box::new(SimpleResult::error(
            "Panic in simple_lancedb_table_optimize_v2".to_string(),
        ))),
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn build_optimize_action_all() {
        let a = build_optimize_action(&serde_json::json!({"type": "all"})).unwrap();
        assert!(matches!(a, OptimizeAction::All));
    }

    #[test]
    fn build_optimize_action_prune_with_options() {
        let a = build_optimize_action(&serde_json::json!({
            "type": "prune",
            "older_than_seconds": 86400,
            "delete_unverified": true,
            "error_if_tagged_old_versions": false,
        }))
        .unwrap();
        match a {
            OptimizeAction::Prune {
                older_than,
                delete_unverified,
                error_if_tagged_old_versions,
            } => {
                assert_eq!(older_than, Some(TimeDelta::try_seconds(86400).unwrap()));
                assert_eq!(delete_unverified, Some(true));
                assert_eq!(error_if_tagged_old_versions, Some(false));
            }
            other => panic!("expected Prune, got {:?}", std::mem::discriminant(&other)),
        }
    }

    #[test]
    fn build_optimize_action_compact_with_options() {
        let a = build_optimize_action(&serde_json::json!({
            "type": "compact",
            "target_rows_per_fragment": 200000,
            "materialize_deletions": false,
        }))
        .unwrap();
        match a {
            OptimizeAction::Compact { options, .. } => {
                assert_eq!(options.target_rows_per_fragment, 200000);
                assert!(!options.materialize_deletions);
            }
            other => panic!("expected Compact, got {:?}", std::mem::discriminant(&other)),
        }
    }

    // OptimizeAction does not implement Debug, so the error-path tests
    // can't use expect_err; pattern-match on the Result directly.
    #[test]
    fn build_optimize_action_unknown_kind_errors() {
        match build_optimize_action(&serde_json::json!({"type": "what"})) {
            Err(e) => assert!(e.contains("Unknown"), "got: {}", e),
            Ok(_) => panic!("unknown type must error"),
        }
    }

    #[test]
    fn build_optimize_action_missing_type_errors() {
        match build_optimize_action(&serde_json::json!({})) {
            Err(e) => assert!(e.contains("'type'"), "got: {}", e),
            Ok(_) => panic!("missing type must error"),
        }
    }
}
