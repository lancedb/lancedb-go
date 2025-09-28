// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: Copyright The LanceDB Authors

//! Table management operations

use crate::ffi::{from_c_str, SimpleResult};
use crate::runtime::get_simple_runtime;
use crate::schema::create_arrow_schema_from_json;
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
