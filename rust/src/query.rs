// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: Copyright The LanceDB Authors

//! Query and search operations

use crate::conversion::convert_arrow_value_to_json;
use crate::ffi::{from_c_str, SimpleResult};
use crate::runtime::get_simple_runtime;
use lancedb::index::scalar::FullTextSearchQuery;
use lancedb::query::{ExecutableQuery, QueryBase};
use std::ffi::CString;
use std::os::raw::{c_char, c_void};
use tokio_stream::StreamExt;

/// Parse a JSON array of column name strings.
fn parse_column_names(columns: &[serde_json::Value]) -> Vec<String> {
    columns
        .iter()
        .filter_map(|v| v.as_str())
        .map(|s| s.to_string())
        .collect()
}

/// Parse a distance type string into a LanceDB DistanceType. Shared by the
/// query path and the index path (rust/src/index.rs), which wraps any
/// error into String via map_err.
pub(crate) fn parse_distance_type(dt: &str) -> Result<lancedb::DistanceType, lancedb::Error> {
    match dt {
        "l2" => Ok(lancedb::DistanceType::L2),
        "cosine" => Ok(lancedb::DistanceType::Cosine),
        "dot" => Ok(lancedb::DistanceType::Dot),
        "hamming" => Ok(lancedb::DistanceType::Hamming),
        other => Err(lancedb::Error::InvalidInput {
            message: format!("Unknown distance type: {}", other),
        }),
    }
}

/// Apply top-level QueryBase flags (with_row_id, fast_search, postfilter)
/// to any builder that implements QueryBase. Shared by the vector, FTS,
/// and standard query paths.
pub(crate) fn apply_query_base_flags<Q: QueryBase>(mut q: Q, config: &serde_json::Value) -> Q {
    if config
        .get("with_row_id")
        .and_then(|v| v.as_bool())
        .unwrap_or(false)
    {
        q = q.with_row_id();
    }
    if config
        .get("fast_search")
        .and_then(|v| v.as_bool())
        .unwrap_or(false)
    {
        q = q.fast_search();
    }
    if config
        .get("postfilter")
        .and_then(|v| v.as_bool())
        .unwrap_or(false)
    {
        q = q.postfilter();
    }
    q
}

/// Build and execute a query from JSON config, returning a record batch stream.
///
/// Handles three query modes based on config contents:
/// - Vector search: nearest_to() with optional distance type, filter, columns
/// - Full-text search: FullTextSearchQuery with optional column, filter, limit
/// - Standard query: filter, limit, offset, column selection
async fn execute_query_from_config(
    table: &lancedb::Table,
    query_config: &serde_json::Value,
) -> Result<
    impl tokio_stream::Stream<Item = Result<arrow_array::RecordBatch, lancedb::Error>>,
    lancedb::Error,
> {
    // Vector search
    if let Some(vector_search) = query_config.get("vector_search") {
        if let (Some(column), Some(vector_values), Some(k)) = (
            vector_search.get("column").and_then(|v| v.as_str()),
            vector_search.get("vector").and_then(|v| v.as_array()),
            vector_search.get("k").and_then(|v| v.as_u64()),
        ) {
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

                    if let Some(filter) = query_config.get("where").and_then(|v| v.as_str()) {
                        vector_query = vector_query.only_if(filter);
                    }

                    if let Some(columns) = query_config.get("columns").and_then(|v| v.as_array()) {
                        let column_names = parse_column_names(columns);
                        if !column_names.is_empty() {
                            vector_query =
                                vector_query.select(lancedb::query::Select::Columns(column_names));
                        }
                    }

                    if let Some(dt) = vector_search.get("distance_type").and_then(|v| v.as_str()) {
                        vector_query = vector_query.distance_type(parse_distance_type(dt)?);
                    }

                    // Per-query vector tuning (IVF / HNSW specific)
                    if let Some(n) = vector_search.get("nprobes").and_then(|v| v.as_u64()) {
                        vector_query = vector_query.nprobes(n as usize);
                    }
                    if let Some(rf) = vector_search.get("refine_factor").and_then(|v| v.as_u64()) {
                        vector_query = vector_query.refine_factor(rf as u32);
                    }
                    if let Some(ef) = vector_search.get("ef").and_then(|v| v.as_u64()) {
                        vector_query = vector_query.ef(ef as usize);
                    }
                    if vector_search
                        .get("bypass_vector_index")
                        .and_then(|v| v.as_bool())
                        .unwrap_or(false)
                    {
                        vector_query = vector_query.bypass_vector_index();
                    }

                    vector_query = apply_query_base_flags(vector_query, query_config);

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

    // Full-text search
    if let Some(fts_search) = query_config.get("fts_search") {
        let query_text = fts_search
            .get("query")
            .and_then(|v| v.as_str())
            .ok_or_else(|| lancedb::Error::InvalidInput {
                message: "fts_search requires a non-null 'query' field".to_string(),
            })?;

        let mut fts_query_obj = FullTextSearchQuery::new(query_text.to_string());

        if let Some(column) = fts_search.get("column").and_then(|v| v.as_str()) {
            fts_query_obj = fts_query_obj.with_column(column.to_string()).map_err(|e| {
                lancedb::Error::InvalidInput {
                    message: format!("Invalid FTS column: {}", e),
                }
            })?;
        }

        let mut fts_query = table.query().full_text_search(fts_query_obj);

        if let Some(columns) = query_config.get("columns").and_then(|v| v.as_array()) {
            let column_names = parse_column_names(columns);
            if !column_names.is_empty() {
                fts_query = fts_query.select(lancedb::query::Select::Columns(column_names));
            }
        }
        if let Some(filter) = query_config.get("where").and_then(|v| v.as_str()) {
            fts_query = fts_query.only_if(filter);
        }
        if let Some(limit) = query_config.get("limit").and_then(|v| v.as_u64()) {
            fts_query = fts_query.limit(limit as usize);
        }
        if query_config
            .get("offset")
            .and_then(|v| v.as_u64())
            .map(|n| n > 0)
            .unwrap_or(false)
        {
            return Err(lancedb::Error::InvalidInput {
                message: "FTS queries do not support offset pagination".to_string(),
            });
        }

        fts_query = apply_query_base_flags(fts_query, query_config);

        return fts_query.execute().await;
    }

    // Standard query
    let mut query = table.query();

    if let Some(columns) = query_config.get("columns").and_then(|v| v.as_array()) {
        let column_names = parse_column_names(columns);
        if !column_names.is_empty() {
            query = query.select(lancedb::query::Select::Columns(column_names));
        }
    }

    if let Some(limit) = query_config.get("limit").and_then(|v| v.as_u64()) {
        query = query.limit(limit as usize);
    }

    if let Some(offset) = query_config.get("offset").and_then(|v| v.as_u64()) {
        query = query.offset(offset as usize);
    }

    if let Some(filter) = query_config.get("where").and_then(|v| v.as_str()) {
        query = query.only_if(filter);
    }

    query = apply_query_base_flags(query, query_config);

    query.execute().await
}

/// Parse table handle and query config from FFI arguments, then execute the query.
/// Returns the runtime and record batch stream on success, or a SimpleResult error.
fn parse_and_execute(
    table_handle: *mut c_void,
    query_config_json: *const c_char,
) -> Result<
    (
        std::sync::Arc<tokio::runtime::Runtime>,
        impl tokio_stream::Stream<Item = Result<arrow_array::RecordBatch, lancedb::Error>>,
    ),
    SimpleResult,
> {
    let config_str = match from_c_str(query_config_json) {
        Ok(s) => s,
        Err(e) => {
            return Err(SimpleResult::error(format!(
                "Invalid query config JSON: {}",
                e
            )))
        }
    };

    let table = unsafe { &*(table_handle as *const lancedb::Table) };
    let rt = get_simple_runtime();

    let query_config: serde_json::Value = match serde_json::from_str(&config_str) {
        Ok(config) => config,
        Err(e) => {
            return Err(SimpleResult::error(format!(
                "Failed to parse query config: {}",
                e
            )))
        }
    };

    match rt.block_on(execute_query_from_config(table, &query_config)) {
        Ok(stream) => Ok((rt, stream)),
        Err(e) => Err(SimpleResult::error(format!(
            "Failed to execute query: {}",
            e
        ))),
    }
}

/// Execute a select query and return results as JSON.
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

        let (rt, stream) = match parse_and_execute(table_handle, query_config_json) {
            Ok(v) => v,
            Err(e) => return e,
        };

        let mut results = Vec::new();

        match rt.block_on(async {
            let mut stream = stream;
            while let Some(batch_result) = stream.next().await {
                match batch_result {
                    Ok(batch) => {
                        for row_idx in 0..batch.num_rows() {
                            let mut row = serde_json::Map::new();
                            let schema = batch.schema();

                            for (col_idx, field) in schema.fields().iter().enumerate() {
                                let column = batch.column(col_idx);
                                let json_value = match convert_arrow_value_to_json(column, row_idx)
                                {
                                    Ok(v) => v,
                                    Err(_) => serde_json::Value::Null,
                                };
                                row.insert(field.name().clone(), json_value);
                            }
                            results.push(serde_json::Value::Object(row));
                        }
                    }
                    Err(e) => return Err(e),
                }
            }
            Ok(())
        }) {
            Ok(()) => match serde_json::to_string(&results) {
                Ok(json_str) => match CString::new(json_str) {
                    Ok(c_string) => {
                        unsafe {
                            *result_json = c_string.into_raw();
                        }
                        SimpleResult::ok()
                    }
                    Err(_) => {
                        SimpleResult::error("Failed to convert results to C string".to_string())
                    }
                },
                Err(e) => {
                    SimpleResult::error(format!("Failed to serialize results to JSON: {}", e))
                }
            },
            Err(e) => SimpleResult::error(format!("Failed to process query results: {}", e)),
        }
    });

    match result {
        Ok(res) => Box::into_raw(Box::new(res)),
        Err(_) => Box::into_raw(Box::new(SimpleResult::error(
            "Panic in simple_lancedb_table_select_query".to_string(),
        ))),
    }
}

/// Execute a select query and return results as Arrow IPC binary data.
#[no_mangle]
#[allow(clippy::not_unsafe_ptr_arg_deref)]
pub extern "C" fn simple_lancedb_table_select_query_ipc(
    table_handle: *mut c_void,
    query_config_json: *const c_char,
    result_ipc_data: *mut *mut u8,
    result_ipc_len: *mut usize,
) -> *mut SimpleResult {
    let result = std::panic::catch_unwind(|| -> SimpleResult {
        if table_handle.is_null()
            || query_config_json.is_null()
            || result_ipc_data.is_null()
            || result_ipc_len.is_null()
        {
            return SimpleResult::error("Invalid null arguments".to_string());
        }

        let (rt, stream) = match parse_and_execute(table_handle, query_config_json) {
            Ok(v) => v,
            Err(e) => return e,
        };

        match rt.block_on(async {
            let mut stream = stream;
            let mut batches = Vec::new();
            while let Some(batch_result) = stream.next().await {
                match batch_result {
                    Ok(batch) => batches.push(batch),
                    Err(e) => return Err(e),
                }
            }
            Ok(batches)
        }) {
            Ok(batches) => {
                if batches.is_empty() {
                    unsafe {
                        *result_ipc_data = std::ptr::null_mut();
                        *result_ipc_len = 0;
                    }
                    return SimpleResult::ok();
                }

                use arrow_ipc::writer::FileWriter;

                let schema = batches[0].schema();
                let mut buf = Vec::new();
                {
                    let mut writer = match FileWriter::try_new(&mut buf, &schema) {
                        Ok(w) => w,
                        Err(e) => {
                            return SimpleResult::error(format!(
                                "Failed to create IPC writer: {}",
                                e
                            ))
                        }
                    };
                    for batch in &batches {
                        if let Err(e) = writer.write(batch) {
                            return SimpleResult::error(format!(
                                "Failed to write IPC batch: {}",
                                e
                            ));
                        }
                    }
                    if let Err(e) = writer.finish() {
                        return SimpleResult::error(format!("Failed to finish IPC file: {}", e));
                    }
                }

                // Transfer ownership to C via libc::malloc (freed by simple_lancedb_free_ipc_data)
                let len = buf.len();
                let data_ptr = unsafe { libc::malloc(len) as *mut u8 };
                if data_ptr.is_null() {
                    return SimpleResult::error(
                        "Failed to allocate memory for IPC data".to_string(),
                    );
                }
                unsafe {
                    std::ptr::copy_nonoverlapping(buf.as_ptr(), data_ptr, len);
                    *result_ipc_data = data_ptr;
                    *result_ipc_len = len;
                }
                SimpleResult::ok()
            }
            Err(e) => SimpleResult::error(format!("Failed to process query results: {}", e)),
        }
    });

    match result {
        Ok(res) => Box::into_raw(Box::new(res)),
        Err(_) => Box::into_raw(Box::new(SimpleResult::error(
            "Panic in simple_lancedb_table_select_query_ipc".to_string(),
        ))),
    }
}
