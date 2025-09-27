// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: Copyright The LanceDB Authors

//! Data CRUD operations

use crate::conversion::json_to_record_batch;
use crate::ffi::{from_c_str, SimpleResult};
use crate::runtime::get_simple_runtime;
use std::os::raw::{c_char, c_void};

/// Delete rows from a table using SQL predicate (simple version)
#[no_mangle]
pub extern "C" fn simple_lancedb_table_delete(
    table_handle: *mut c_void,
    predicate: *const c_char,
    deleted_count: *mut i64,
) -> *mut SimpleResult {
    let result = std::panic::catch_unwind(|| -> SimpleResult {
        if table_handle.is_null() || predicate.is_null() || deleted_count.is_null() {
            return SimpleResult::error("Invalid null arguments".to_string());
        }

        let predicate_str = match from_c_str(predicate) {
            Ok(s) => s,
            Err(e) => return SimpleResult::error(format!("Invalid predicate: {}", e)),
        };

        let table = unsafe { &*(table_handle as *const lancedb::Table) };
        let rt = get_simple_runtime();

        match rt.block_on(async { table.delete(&predicate_str).await }) {
            Ok(_delete_result) => {
                // Note: LanceDB's DeleteResult doesn't expose the number of deleted rows
                // We set this to -1 to indicate successful deletion but unknown count
                unsafe {
                    *deleted_count = -1;
                }
                SimpleResult::ok()
            }
            Err(e) => SimpleResult::error(format!("Failed to delete rows: {}", e)),
        }
    });

    match result {
        Ok(res) => Box::into_raw(Box::new(res)),
        Err(_) => Box::into_raw(Box::new(SimpleResult::error(
            "Panic in simple_lancedb_table_delete".to_string(),
        ))),
    }
}

/// Update rows in a table using SQL predicate and column updates (simple version)
#[no_mangle]
pub extern "C" fn simple_lancedb_table_update(
    table_handle: *mut c_void,
    predicate: *const c_char,
    updates_json: *const c_char,
) -> *mut SimpleResult {
    let result = std::panic::catch_unwind(|| -> SimpleResult {
        if table_handle.is_null() || predicate.is_null() || updates_json.is_null() {
            return SimpleResult::error("Invalid null arguments".to_string());
        }

        let predicate_str = match from_c_str(predicate) {
            Ok(s) => s,
            Err(e) => return SimpleResult::error(format!("Invalid predicate: {}", e)),
        };

        let updates_str = match from_c_str(updates_json) {
            Ok(s) => s,
            Err(e) => return SimpleResult::error(format!("Invalid updates JSON: {}", e)),
        };

        // Parse updates JSON into a map
        let updates: std::collections::HashMap<String, serde_json::Value> =
            match serde_json::from_str(&updates_str) {
                Ok(u) => u,
                Err(e) => {
                    return SimpleResult::error(format!("Failed to parse updates JSON: {}", e))
                }
            };

        let table = unsafe { &*(table_handle as *const lancedb::Table) };
        let rt = get_simple_runtime();

        // Validate all update values first
        for (column, value) in updates.iter() {
            match value {
                serde_json::Value::String(_)
                | serde_json::Value::Number(_)
                | serde_json::Value::Bool(_)
                | serde_json::Value::Null => {}
                _ => {
                    return SimpleResult::error(format!(
                        "Unsupported update value type for column {}",
                        column
                    ))
                }
            }
        }

        match rt.block_on(async {
            let mut update_builder = table.update().only_if(&predicate_str);

            // Add each column update separately
            for (column, value) in updates.iter() {
                let value_str = match value {
                    serde_json::Value::String(s) => format!("'{}'", s), // String values need quotes
                    serde_json::Value::Number(n) => n.to_string(),
                    serde_json::Value::Bool(b) => b.to_string(),
                    serde_json::Value::Null => "NULL".to_string(),
                    _ => unreachable!(), // Already validated above
                };
                update_builder = update_builder.column(column, &value_str);
            }

            update_builder.execute().await
        }) {
            Ok(_update_result) => SimpleResult::ok(),
            Err(e) => SimpleResult::error(format!("Failed to update rows: {}", e)),
        }
    });

    match result {
        Ok(res) => Box::into_raw(Box::new(res)),
        Err(_) => Box::into_raw(Box::new(SimpleResult::error(
            "Panic in simple_lancedb_table_update".to_string(),
        ))),
    }
}

/// Add JSON data to a table (simple version)
/// Converts JSON array of objects to Arrow RecordBatch and adds to table
#[no_mangle]
pub extern "C" fn simple_lancedb_table_add_json(
    table_handle: *mut c_void,
    json_data: *const c_char,
    added_count: *mut i64,
) -> *mut SimpleResult {
    let result = std::panic::catch_unwind(|| -> SimpleResult {
        if table_handle.is_null() || json_data.is_null() || added_count.is_null() {
            return SimpleResult::error("Invalid null arguments".to_string());
        }

        let json_str = match from_c_str(json_data) {
            Ok(s) => s,
            Err(e) => return SimpleResult::error(format!("Invalid JSON data: {}", e)),
        };

        let table = unsafe { &*(table_handle as *const lancedb::Table) };
        let rt = get_simple_runtime();

        // Parse JSON array
        let json_values: Vec<serde_json::Value> = match serde_json::from_str(&json_str) {
            Ok(serde_json::Value::Array(arr)) => arr,
            Ok(single_value) => vec![single_value], // Convert single object to array
            Err(e) => return SimpleResult::error(format!("Failed to parse JSON: {}", e)),
        };

        if json_values.is_empty() {
            unsafe {
                *added_count = 0;
            }
            return SimpleResult::ok();
        }

        // Get table schema
        let table_schema = match rt.block_on(async { table.schema().await }) {
            Ok(schema) => schema,
            Err(e) => return SimpleResult::error(format!("Failed to get table schema: {}", e)),
        };

        // Convert JSON to RecordBatch
        match json_to_record_batch(&json_values, &table_schema) {
            Ok(record_batch) => {
                // Add the record batch to the table
                match rt.block_on(async {
                    use arrow_array::RecordBatchIterator;
                    let batches = vec![Ok(record_batch.clone())];
                    let batch_iter = RecordBatchIterator::new(batches, record_batch.schema());
                    table.add(batch_iter).execute().await
                }) {
                    Ok(_) => {
                        unsafe {
                            *added_count = record_batch.num_rows() as i64;
                        }
                        SimpleResult::ok()
                    }
                    Err(e) => SimpleResult::error(format!("Failed to add data to table: {}", e)),
                }
            }
            Err(e) => SimpleResult::error(format!("Failed to convert JSON to RecordBatch: {}", e)),
        }
    });

    match result {
        Ok(res) => Box::into_raw(Box::new(res)),
        Err(_) => Box::into_raw(Box::new(SimpleResult::error(
            "Panic in simple_lancedb_table_add_json".to_string(),
        ))),
    }
}

/// Add data to a table using Arrow IPC format (more efficient than JSON)
/// Accepts batch of records as Arrow IPC binary data
#[no_mangle]
pub extern "C" fn simple_lancedb_table_add_ipc(
    table_handle: *mut c_void,
    ipc_data: *const u8,
    ipc_len: usize,
    added_count: *mut i64,
) -> *mut SimpleResult {
    let result = std::panic::catch_unwind(|| -> SimpleResult {
        if table_handle.is_null() || ipc_data.is_null() || added_count.is_null() {
            return SimpleResult::error("Invalid null arguments".to_string());
        }

        if ipc_len == 0 {
            unsafe {
                *added_count = 0;
            }
            return SimpleResult::ok();
        }

        let table = unsafe { &*(table_handle as *const lancedb::Table) };
        let rt = get_simple_runtime();

        // Convert raw pointer to slice
        let ipc_bytes = unsafe { std::slice::from_raw_parts(ipc_data, ipc_len) };

        // Deserialize Arrow IPC data to RecordBatch(es)
        match ipc_to_record_batches(ipc_bytes) {
            Ok(record_batches) => {
                if record_batches.is_empty() {
                    unsafe {
                        *added_count = 0;
                    }
                    return SimpleResult::ok();
                }

                // Calculate total rows across all batches
                let total_rows: usize = record_batches.iter().map(|batch| batch.num_rows()).sum();

                // Add the record batches to the table
                match rt.block_on(async {
                    use arrow_array::RecordBatchIterator;

                    // Get schema from the first batch
                    let schema = record_batches[0].schema();

                    // Create iterator from record batches
                    let batches: Vec<Result<arrow_array::RecordBatch, arrow_schema::ArrowError>> =
                        record_batches.into_iter().map(Ok).collect();
                    let batch_iter = RecordBatchIterator::new(batches, schema);

                    table.add(batch_iter).execute().await
                }) {
                    Ok(_) => {
                        unsafe {
                            *added_count = total_rows as i64;
                        }
                        SimpleResult::ok()
                    }
                    Err(e) => SimpleResult::error(format!("Failed to add data to table: {}", e)),
                }
            }
            Err(e) => SimpleResult::error(format!("Failed to parse IPC data: {}", e)),
        }
    });

    match result {
        Ok(res) => Box::into_raw(Box::new(res)),
        Err(_) => Box::into_raw(Box::new(SimpleResult::error(
            "Panic in simple_lancedb_table_add_ipc".to_string(),
        ))),
    }
}

/// Helper function to convert IPC bytes to RecordBatches
fn ipc_to_record_batches(ipc_bytes: &[u8]) -> Result<Vec<arrow_array::RecordBatch>, String> {
    use arrow_ipc::reader::FileReader;
    use std::io::Cursor;

    // Create a reader from the IPC bytes
    let cursor = Cursor::new(ipc_bytes);
    let reader = FileReader::try_new(cursor, None)
        .map_err(|e| format!("Failed to create IPC reader: {}", e))?;

    // Collect all record batches
    let mut record_batches = Vec::new();
    for batch_result in reader {
        match batch_result {
            Ok(batch) => record_batches.push(batch),
            Err(e) => return Err(format!("Failed to read record batch: {}", e)),
        }
    }

    Ok(record_batches)
}
