// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: Copyright The LanceDB Authors

//! Query and search operations

use crate::conversion::convert_arrow_value_to_json;
use crate::ffi::{from_c_str, SimpleResult};
use crate::runtime::get_simple_runtime;
use lancedb::query::{ExecutableQuery, QueryBase};
use std::ffi::CString;
use std::os::raw::{c_char, c_void};
use tokio_stream::StreamExt;

/// Execute a select query with various predicates (vector search, filters, etc.)
#[no_mangle]
#[allow(clippy::not_unsafe_ptr_arg_deref)]
pub extern "C" fn simple_lancedb_table_select_query(
    table_handle: *mut c_void,
    query_config_json: *const c_char,
    result_json: *mut *mut c_char,
) -> *mut SimpleResult {
    let result = std::panic::catch_unwind(|| -> SimpleResult {
        if table_handle.is_null() || query_config_json.is_null() || result_json.is_null() {
            return SimpleResult::error("Invalid null arguments".to_string());
        }

        let config_str = match from_c_str(query_config_json) {
            Ok(s) => s,
            Err(e) => return SimpleResult::error(format!("Invalid query config JSON: {}", e)),
        };

        let table = unsafe { &*(table_handle as *const lancedb::Table) };
        let rt = get_simple_runtime();

        // Parse query configuration
        let query_config: serde_json::Value = match serde_json::from_str(&config_str) {
            Ok(config) => config,
            Err(e) => return SimpleResult::error(format!("Failed to parse query config: {}", e)),
        };

        // Execute query based on configuration
        match rt.block_on(async {
            // Check if this is a vector search query first, as it needs special handling
            if let Some(vector_search) = query_config.get("vector_search") {
                if let (Some(column), Some(vector_values), Some(k)) = (
                    vector_search.get("column").and_then(|v| v.as_str()),
                    vector_search.get("vector").and_then(|v| v.as_array()),
                    vector_search.get("k").and_then(|v| v.as_u64()),
                ) {
                    // Convert JSON array to Vec<f32>
                    let vector: Result<Vec<f32>, String> = vector_values
                        .iter()
                        .map(|v| {
                            v.as_f64()
                                .map(|f| f as f32)
                                .ok_or_else(|| "Invalid vector element".to_string())
                        })
                        .collect();

                    match vector {
                        Ok(vec) => {
                            // Use the limit from query config, or k if not specified
                            let effective_limit = query_config
                                .get("limit")
                                .and_then(|v| v.as_u64())
                                .map(|l| l as usize)
                                .unwrap_or(k as usize);

                            let mut vector_query = table
                                .query()
                                .nearest_to(vec)?
                                .column(column)
                                .limit(effective_limit);

                            // Apply WHERE filter for vector queries
                            if let Some(filter) = query_config.get("where").and_then(|v| v.as_str())
                            {
                                vector_query = vector_query.only_if(filter);
                            }

                            // Apply column selection for vector queries
                            if let Some(columns) =
                                query_config.get("columns").and_then(|v| v.as_array())
                            {
                                let column_names: Vec<String> = columns
                                    .iter()
                                    .filter_map(|v| v.as_str())
                                    .map(|s| s.to_string())
                                    .collect();
                                if !column_names.is_empty() {
                                    vector_query = vector_query
                                        .select(lancedb::query::Select::Columns(column_names));
                                }
                            }

                            return vector_query.execute().await;
                        }
                        Err(e) => {
                            return Err(lancedb::Error::InvalidInput {
                                message: format!("Failed to parse vector: {}", e),
                            })
                        }
                    }
                }
            }

            // Apply full-text search
            if let Some(fts_search) = query_config.get("fts_search") {
                if let (Some(_column), Some(_query_text)) = (
                    fts_search.get("column").and_then(|v| v.as_str()),
                    fts_search.get("query").and_then(|v| v.as_str()),
                ) {
                    // Note: FTS search is not currently available in this API version
                    // This is a placeholder for future implementation
                    return Err(lancedb::Error::InvalidInput {
                        message: "Full-text search is not currently supported".to_string(),
                    });
                }
            }

            // For non-vector queries, use regular query
            let mut query = table.query();

            // Apply column selection
            if let Some(columns) = query_config.get("columns").and_then(|v| v.as_array()) {
                let column_names: Vec<String> = columns
                    .iter()
                    .filter_map(|v| v.as_str())
                    .map(|s| s.to_string())
                    .collect();
                if !column_names.is_empty() {
                    query = query.select(lancedb::query::Select::Columns(column_names));
                }
            }

            // Apply limit
            if let Some(limit) = query_config.get("limit").and_then(|v| v.as_u64()) {
                query = query.limit(limit as usize);
            }

            // Apply offset
            if let Some(offset) = query_config.get("offset").and_then(|v| v.as_u64()) {
                query = query.offset(offset as usize);
            }

            // Apply WHERE filter
            if let Some(filter) = query_config.get("where").and_then(|v| v.as_str()) {
                query = query.only_if(filter);
            }

            // Execute the query
            query.execute().await
        }) {
            Ok(record_batch_reader) => {
                // Convert RecordBatch results to JSON
                let mut results = Vec::new();

                // Note: This is a simplified approach. In a real implementation,
                // you might want to stream results or handle large datasets differently.
                match rt.block_on(async {
                    let mut stream = record_batch_reader;
                    while let Some(batch_result) = stream.next().await {
                        match batch_result {
                            Ok(batch) => {
                                // Convert RecordBatch to JSON
                                for row_idx in 0..batch.num_rows() {
                                    let mut row = serde_json::Map::new();
                                    let schema = batch.schema();

                                    for (col_idx, field) in schema.fields().iter().enumerate() {
                                        let column = batch.column(col_idx);
                                        let field_name = field.name();

                                        // Convert Arrow array value to JSON value
                                        let json_value =
                                            match convert_arrow_value_to_json(column, row_idx) {
                                                Ok(v) => v,
                                                Err(_) => serde_json::Value::Null,
                                            };

                                        row.insert(field_name.clone(), json_value);
                                    }
                                    results.push(serde_json::Value::Object(row));
                                }
                            }
                            Err(e) => return Err(e),
                        }
                    }
                    Ok(())
                }) {
                    Ok(()) => {
                        // Serialize results to JSON
                        match serde_json::to_string(&results) {
                            Ok(json_str) => match CString::new(json_str) {
                                Ok(c_string) => {
                                    unsafe {
                                        *result_json = c_string.into_raw();
                                    }
                                    SimpleResult::ok()
                                }
                                Err(_) => SimpleResult::error(
                                    "Failed to convert results to C string".to_string(),
                                ),
                            },
                            Err(e) => SimpleResult::error(format!(
                                "Failed to serialize results to JSON: {}",
                                e
                            )),
                        }
                    }
                    Err(e) => {
                        SimpleResult::error(format!("Failed to process query results: {}", e))
                    }
                }
            }
            Err(e) => SimpleResult::error(format!("Failed to execute query: {}", e)),
        }
    });

    match result {
        Ok(res) => Box::into_raw(Box::new(res)),
        Err(_) => Box::into_raw(Box::new(SimpleResult::error(
            "Panic in simple_lancedb_table_select_query".to_string(),
        ))),
    }
}
