// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: Copyright The LanceDB Authors

//! Table metadata operations

use crate::ffi::{SimpleResult};
use crate::runtime::get_simple_runtime;
use std::ffi::CString;
use std::os::raw::{c_char, c_void};

/// Count rows in a table (simple version)
#[no_mangle]
pub extern "C" fn simple_lancedb_table_count_rows(
    table_handle: *mut c_void,
    count: *mut i64,
) -> *mut SimpleResult {
    let result = std::panic::catch_unwind(|| -> SimpleResult {
        if table_handle.is_null() || count.is_null() {
            return SimpleResult::error("Invalid null arguments".to_string());
        }

        let table = unsafe { &*(table_handle as *const lancedb::Table) };
        let rt = get_simple_runtime();

        match rt.block_on(async { table.count_rows(None).await }) {
            Ok(row_count) => {
                unsafe {
                    *count = row_count as i64;
                }
                SimpleResult::ok()
            }
            Err(e) => SimpleResult::error(format!("Failed to count rows: {}", e)),
        }
    });

    match result {
        Ok(res) => Box::into_raw(Box::new(res)),
        Err(_) => Box::into_raw(Box::new(SimpleResult::error(
            "Panic in simple_lancedb_table_count_rows".to_string(),
        ))),
    }
}

/// Get table version (simple version)
#[no_mangle]
pub extern "C" fn simple_lancedb_table_version(
    table_handle: *mut c_void,
    version: *mut i64,
) -> *mut SimpleResult {
    let result = std::panic::catch_unwind(|| -> SimpleResult {
        if table_handle.is_null() || version.is_null() {
            return SimpleResult::error("Invalid null arguments".to_string());
        }

        let table = unsafe { &*(table_handle as *const lancedb::Table) };
        let rt = get_simple_runtime();

        match rt.block_on(async { table.version().await }) {
            Ok(table_version) => {
                unsafe {
                    *version = table_version as i64;
                }
                SimpleResult::ok()
            }
            Err(e) => SimpleResult::error(format!("Failed to get table version: {}", e)),
        }
    });

    match result {
        Ok(res) => Box::into_raw(Box::new(res)),
        Err(_) => Box::into_raw(Box::new(SimpleResult::error(
            "Panic in simple_lancedb_table_version".to_string(),
        ))),
    }
}

/// Get table schema as JSON (simple version)
#[no_mangle]
pub extern "C" fn simple_lancedb_table_schema(
    table_handle: *mut c_void,
    schema_json: *mut *mut c_char,
) -> *mut SimpleResult {
    let result = std::panic::catch_unwind(|| -> SimpleResult {
        if table_handle.is_null() || schema_json.is_null() {
            return SimpleResult::error("Invalid null arguments".to_string());
        }

        let table = unsafe { &*(table_handle as *const lancedb::Table) };
        let rt = get_simple_runtime();

        match rt.block_on(async { table.schema().await }) {
            Ok(arrow_schema) => {
                // Convert Arrow schema to JSON
                let fields: Vec<serde_json::Value> = arrow_schema
                    .fields()
                    .iter()
                    .map(|field| {
                        let type_str = match field.data_type() {
                            arrow_schema::DataType::Int32 => "int32",
                            arrow_schema::DataType::Int64 => "int64",
                            arrow_schema::DataType::Float32 => "float32",
                            arrow_schema::DataType::Float64 => "float64",
                            arrow_schema::DataType::Utf8 => "string",
                            arrow_schema::DataType::Binary => "binary",
                            arrow_schema::DataType::Boolean => "boolean",
                            arrow_schema::DataType::FixedSizeList(inner, size) => {
                                if matches!(inner.data_type(), arrow_schema::DataType::Float32) {
                                    return serde_json::json!({
                                        "name": field.name(),
                                        "type": format!("fixed_size_list[float32;{}]", size),
                                        "nullable": field.is_nullable()
                                    });
                                } else {
                                    "unknown"
                                }
                            }
                            _ => "unknown",
                        };

                        serde_json::json!({
                            "name": field.name(),
                            "type": type_str,
                            "nullable": field.is_nullable()
                        })
                    })
                    .collect();

                let schema_json_obj = serde_json::json!({
                    "fields": fields
                });

                match serde_json::to_string(&schema_json_obj) {
                    Ok(json_str) => {
                        let c_str = CString::new(json_str).unwrap();
                        unsafe {
                            *schema_json = c_str.into_raw();
                        }
                        SimpleResult::ok()
                    }
                    Err(e) => SimpleResult::error(format!("Failed to serialize schema: {}", e)),
                }
            }
            Err(e) => SimpleResult::error(format!("Failed to get table schema: {}", e)),
        }
    });

    match result {
        Ok(res) => Box::into_raw(Box::new(res)),
        Err(_) => Box::into_raw(Box::new(SimpleResult::error(
            "Panic in simple_lancedb_table_schema".to_string(),
        ))),
    }
}
