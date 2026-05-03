// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: Copyright The LanceDB Authors

//! Schema-evolution surface — add / alter / drop columns.
//!
//! Mirrors the same SimpleResult + panic-safe + shared-runtime envelope
//! used by the rest of the simple FFI. Inputs that carry structured
//! payloads (the SqlExpressions transform list, the alteration list,
//! the drop names) come in as JSON strings; outputs (commit version)
//! come back through scalar out-pointers.
//!
//! Scope (v1):
//!   - add_columns: NewColumnTransform::SqlExpressions only.
//!     BatchUDF / Stream / Reader / AllNulls are not exposed because
//!     they require either Rust closures or full Arrow IPC plumbing.
//!     SqlExpressions covers the common "derive from existing columns"
//!     case (e.g. `score * 2`, `date_trunc('day', ts)`) and is the only
//!     transform reachable from Python's `add_columns(transforms=dict)`.
//!   - alter_columns: rename + nullable only. The data_type cast
//!     variant is gated behind Arrow DataType serialization and is
//!     deliberately deferred to a follow-up PR.
//!   - drop_columns: full surface — just a list of column names.

use crate::ffi::{from_c_str, SimpleResult};
use crate::runtime::get_simple_runtime;
use lancedb::table::{ColumnAlteration, NewColumnTransform};
use serde::Deserialize;
use std::os::raw::{c_char, c_void};

/// One entry in the SqlExpressions transform list. `name` is the new
/// column to create; `expression` is a SQL expression evaluated against
/// existing columns to produce its values.
#[derive(Debug, Deserialize)]
struct SqlExprEntry {
    name: String,
    expression: String,
}

/// JSON shape expected by simple_lancedb_table_alter_columns. Mirrors
/// lance::dataset::ColumnAlteration with the fields actually exposed in
/// v1 (rename, nullable). The `path` field maps to ColumnAlteration's
/// path. Unset rename / nullable mean "leave unchanged".
#[derive(Debug, Deserialize)]
struct AlterEntry {
    path: String,
    #[serde(default)]
    rename: Option<String>,
    #[serde(default)]
    nullable: Option<bool>,
}

/// Add new columns to the table by evaluating SQL expressions over
/// existing rows. `transforms_json` is a JSON array of
/// {"name": "<col>", "expression": "<sql>"} objects. Empty arrays are
/// rejected as a caller bug — adding zero columns is a no-op that
/// almost always indicates a missing argument.
///
/// On success, the new commit version is written to *version_out. A
/// version of 0 indicates compatibility with legacy backends that do
/// not report a commit version (mirrors AddColumnsResult::version
/// semantics in lancedb).
#[no_mangle]
#[allow(clippy::not_unsafe_ptr_arg_deref)]
pub extern "C" fn simple_lancedb_table_add_columns(
    table_handle: *mut c_void,
    transforms_json: *const c_char,
    version_out: *mut u64,
) -> *mut SimpleResult {
    let result = std::panic::catch_unwind(|| -> SimpleResult {
        if table_handle.is_null() || transforms_json.is_null() || version_out.is_null() {
            return SimpleResult::error("Invalid null arguments".to_string());
        }
        let json = match from_c_str(transforms_json) {
            Ok(s) => s,
            Err(e) => return SimpleResult::error(format!("Invalid transforms_json: {}", e)),
        };
        let entries: Vec<SqlExprEntry> = match serde_json::from_str(&json) {
            Ok(v) => v,
            Err(e) => return SimpleResult::error(format!("Failed to parse transforms_json: {}", e)),
        };
        if entries.is_empty() {
            return SimpleResult::error(
                "add_columns: transforms must be a non-empty array".to_string(),
            );
        }

        let pairs: Vec<(String, String)> = entries
            .into_iter()
            .map(|e| (e.name, e.expression))
            .collect();
        let transforms = NewColumnTransform::SqlExpressions(pairs);

        let table = unsafe { &*(table_handle as *const lancedb::Table) };
        let rt = get_simple_runtime();
        match rt.block_on(async { table.add_columns(transforms, None).await }) {
            Ok(res) => {
                unsafe {
                    *version_out = res.version;
                }
                SimpleResult::ok()
            }
            Err(e) => SimpleResult::error(format!("add_columns failed: {}", e)),
        }
    });

    match result {
        Ok(res) => Box::into_raw(Box::new(res)),
        Err(_) => Box::into_raw(Box::new(SimpleResult::error(
            "Panic in simple_lancedb_table_add_columns".to_string(),
        ))),
    }
}

/// Alter existing columns — rename and/or change nullability.
/// `alterations_json` is a JSON array of {"path": "<col>", "rename":
/// "<new>", "nullable": <bool>} objects; rename and nullable are both
/// optional and "unset" means "leave that attribute alone". Empty
/// arrays are rejected.
///
/// data_type casts are intentionally not exposed in v1 — they require
/// Arrow DataType serialization and are slated for a follow-up. Callers
/// who need a cast can drop and re-add the column through add_columns.
///
/// On success, the new commit version is written to *version_out.
#[no_mangle]
#[allow(clippy::not_unsafe_ptr_arg_deref)]
pub extern "C" fn simple_lancedb_table_alter_columns(
    table_handle: *mut c_void,
    alterations_json: *const c_char,
    version_out: *mut u64,
) -> *mut SimpleResult {
    let result = std::panic::catch_unwind(|| -> SimpleResult {
        if table_handle.is_null() || alterations_json.is_null() || version_out.is_null() {
            return SimpleResult::error("Invalid null arguments".to_string());
        }
        let json = match from_c_str(alterations_json) {
            Ok(s) => s,
            Err(e) => return SimpleResult::error(format!("Invalid alterations_json: {}", e)),
        };
        let entries: Vec<AlterEntry> = match serde_json::from_str(&json) {
            Ok(v) => v,
            Err(e) => {
                return SimpleResult::error(format!("Failed to parse alterations_json: {}", e))
            }
        };
        if entries.is_empty() {
            return SimpleResult::error(
                "alter_columns: alterations must be a non-empty array".to_string(),
            );
        }
        for (i, e) in entries.iter().enumerate() {
            if e.path.trim().is_empty() {
                return SimpleResult::error(format!(
                    "alter_columns: alterations[{}].path is empty",
                    i
                ));
            }
            if e.rename.is_none() && e.nullable.is_none() {
                return SimpleResult::error(format!(
                    "alter_columns: alterations[{}] has no rename or nullable change",
                    i
                ));
            }
        }

        let alterations: Vec<ColumnAlteration> = entries
            .into_iter()
            .map(|e| {
                let mut a = ColumnAlteration::new(e.path);
                if let Some(new_name) = e.rename {
                    a = a.rename(new_name);
                }
                if let Some(n) = e.nullable {
                    a = a.set_nullable(n);
                }
                a
            })
            .collect();

        let table = unsafe { &*(table_handle as *const lancedb::Table) };
        let rt = get_simple_runtime();
        match rt.block_on(async { table.alter_columns(&alterations).await }) {
            Ok(res) => {
                unsafe {
                    *version_out = res.version;
                }
                SimpleResult::ok()
            }
            Err(e) => SimpleResult::error(format!("alter_columns failed: {}", e)),
        }
    });

    match result {
        Ok(res) => Box::into_raw(Box::new(res)),
        Err(_) => Box::into_raw(Box::new(SimpleResult::error(
            "Panic in simple_lancedb_table_alter_columns".to_string(),
        ))),
    }
}

/// Drop columns from the table. `columns_json` is a JSON array of
/// strings naming the columns to remove. Empty arrays are rejected.
///
/// On success, the new commit version is written to *version_out.
#[no_mangle]
#[allow(clippy::not_unsafe_ptr_arg_deref)]
pub extern "C" fn simple_lancedb_table_drop_columns(
    table_handle: *mut c_void,
    columns_json: *const c_char,
    version_out: *mut u64,
) -> *mut SimpleResult {
    let result = std::panic::catch_unwind(|| -> SimpleResult {
        if table_handle.is_null() || columns_json.is_null() || version_out.is_null() {
            return SimpleResult::error("Invalid null arguments".to_string());
        }
        let json = match from_c_str(columns_json) {
            Ok(s) => s,
            Err(e) => return SimpleResult::error(format!("Invalid columns_json: {}", e)),
        };
        let names: Vec<String> = match serde_json::from_str(&json) {
            Ok(v) => v,
            Err(e) => return SimpleResult::error(format!("Failed to parse columns_json: {}", e)),
        };
        if names.is_empty() {
            return SimpleResult::error(
                "drop_columns: columns must be a non-empty array".to_string(),
            );
        }
        for (i, n) in names.iter().enumerate() {
            if n.trim().is_empty() {
                return SimpleResult::error(format!("drop_columns: columns[{}] is empty", i));
            }
        }

        // lancedb::Table::drop_columns wants &[&str]; build the borrow
        // slice from the owned Vec<String>.
        let refs: Vec<&str> = names.iter().map(String::as_str).collect();

        let table = unsafe { &*(table_handle as *const lancedb::Table) };
        let rt = get_simple_runtime();
        match rt.block_on(async { table.drop_columns(&refs).await }) {
            Ok(res) => {
                unsafe {
                    *version_out = res.version;
                }
                SimpleResult::ok()
            }
            Err(e) => SimpleResult::error(format!("drop_columns failed: {}", e)),
        }
    });

    match result {
        Ok(res) => Box::into_raw(Box::new(res)),
        Err(_) => Box::into_raw(Box::new(SimpleResult::error(
            "Panic in simple_lancedb_table_drop_columns".to_string(),
        ))),
    }
}
