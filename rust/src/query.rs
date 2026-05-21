// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: Copyright The LanceDB Authors

//! Query and search operations

use crate::conversion::convert_arrow_value_to_json;
use crate::ffi::{from_c_str, SimpleResult};
use crate::runtime::get_simple_runtime;
use lancedb::index::scalar::FullTextSearchQuery;
use lancedb::query::{ExecutableQuery, QueryBase};
use lancedb::rerankers::rrf::RRFReranker;
use lancedb::rerankers::{NormalizeMethod, Reranker};
use std::ffi::CString;
use std::os::raw::{c_char, c_void};
use std::sync::Arc;
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

/// Default RRF k parameter. Matches lancedb::rerankers::rrf::RRFReranker's
/// own default so an omitted k produces identical behaviour.
const DEFAULT_RRF_K: f32 = 60.0;

/// Result type for parse_reranker. Pulled out to tame clippy::type_complexity.
type RerankerParse = (Option<Arc<dyn Reranker>>, Option<NormalizeMethod>);

/// Parse the top-level `reranker` / `norm` section of a query config.
/// Returns Ok((None, None)) as the fast path when no reranker is
/// configured — the only cost in that case is a single map lookup.
fn parse_reranker(config: &serde_json::Value) -> Result<RerankerParse, lancedb::Error> {
    let Some(reranker_cfg) = config.get("reranker") else {
        return Ok((None, None));
    };
    // Treat an explicit null the same as a missing key. Go's omitempty
    // can't drop a non-nil *RerankerConfig pointer, so callers who pass
    // QueryConfig{Reranker: &RerankerConfig{Kind: RerankerNone}} land
    // here with `null` — that's intent to skip reranking, not an error.
    if reranker_cfg.is_null() {
        return Ok((None, None));
    }

    let kind = reranker_cfg
        .get("kind")
        .and_then(|v| v.as_str())
        .ok_or_else(|| lancedb::Error::InvalidInput {
            message: "reranker requires a 'kind' field".to_string(),
        })?;

    let reranker: Arc<dyn Reranker> = match kind {
        "rrf" => {
            let k = reranker_cfg
                .get("k")
                .and_then(|v| v.as_f64())
                .map(|v| v as f32)
                .unwrap_or(DEFAULT_RRF_K);
            Arc::new(RRFReranker::new(k))
        }
        other => {
            return Err(lancedb::Error::InvalidInput {
                message: format!("Unknown reranker kind: {}", other),
            })
        }
    };

    let norm = match reranker_cfg.get("norm").and_then(|v| v.as_str()) {
        Some("rank") => Some(NormalizeMethod::Rank),
        Some("score") => Some(NormalizeMethod::Score),
        Some(other) => {
            return Err(lancedb::Error::InvalidInput {
                message: format!("Unknown reranker norm method: {}", other),
            })
        }
        None => None,
    };

    Ok((Some(reranker), norm))
}

/// Apply top-level QueryBase flags (with_row_id, fast_search, postfilter,
/// reranker, norm) to any builder implementing lancedb's QueryBase trait.
/// Shared by the vector, FTS, and standard query paths — all three use
/// VectorQuery or Query which both implement QueryBase.
pub(crate) fn apply_query_base_flags<Q: QueryBase>(
    mut q: Q,
    config: &serde_json::Value,
) -> Result<Q, lancedb::Error> {
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
    let (reranker, norm) = parse_reranker(config)?;
    if let Some(r) = reranker {
        q = q.rerank(r);
    }
    if let Some(n) = norm {
        q = q.norm(n);
    }
    Ok(q)
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
        // Pick the query-vector dtype. Exactly one of vector / vector_f64 /
        // vector_f16 / vector_u8 must be a non-empty array; the Go side
        // guards this before serialising, but we re-check here so a
        // hand-crafted JSON config doesn't silently fall through.
        let vec_f32_arr = vector_search
            .get("vector")
            .and_then(|v| v.as_array())
            .filter(|a| !a.is_empty());
        let vec_f64_arr = vector_search
            .get("vector_f64")
            .and_then(|v| v.as_array())
            .filter(|a| !a.is_empty());
        let vec_f16_arr = vector_search
            .get("vector_f16")
            .and_then(|v| v.as_array())
            .filter(|a| !a.is_empty());
        let vec_u8_arr = vector_search
            .get("vector_u8")
            .and_then(|v| v.as_array())
            .filter(|a| !a.is_empty());
        let column_opt = vector_search.get("column").and_then(|v| v.as_str());
        let k_opt = vector_search.get("k").and_then(|v| v.as_u64());
        let dtype_count = vec_f32_arr.is_some() as u8
            + vec_f64_arr.is_some() as u8
            + vec_f16_arr.is_some() as u8
            + vec_u8_arr.is_some() as u8;

        if let (Some(column), Some(k)) = (column_opt, k_opt) {
            if dtype_count != 1 {
                return Err(lancedb::Error::InvalidInput {
                    message: format!(
                        "vector_search must set exactly one of vector / vector_f64 / vector_f16 / vector_u8 (found {})",
                        dtype_count
                    ),
                });
            }

            let effective_limit = query_config
                .get("limit")
                .and_then(|v| v.as_u64())
                .map(|l| l as usize)
                .unwrap_or(k as usize);

            // Build the VectorQuery from whichever dtype was supplied.
            // lancedb's IntoQueryVector is implemented for Vec<f32>,
            // Vec<f64>, and Vec<half::f16>; for u8 we hand a 1-D Arrow
            // UInt8 array through the Arc<dyn Array> impl. Each impl
            // handles any column-dtype cast the target column requires.
            let invalid = |msg: String| lancedb::Error::InvalidInput { message: msg };
            let nearest_built = if let Some(arr) = vec_f64_arr {
                let v: Result<Vec<f64>, String> = arr
                    .iter()
                    .map(|x| x.as_f64().ok_or_else(|| "Invalid f64 element".to_string()))
                    .collect();
                table.query().nearest_to(
                    v.map_err(|e| invalid(format!("Failed to parse vector_f64: {}", e)))?,
                )?
            } else if let Some(arr) = vec_f16_arr {
                // f16 vectors travel as raw IEEE 754 half-precision bits
                // in a JSON array of unsigned 16-bit numbers.
                // half::f16::from_bits turns each back into the canonical
                // f16 value.
                let v: Result<Vec<half::f16>, String> = arr
                    .iter()
                    .map(|x| {
                        x.as_u64()
                            .and_then(|u| u16::try_from(u).ok())
                            .map(half::f16::from_bits)
                            .ok_or_else(|| {
                                "Invalid f16 bits element (must be 0..=65535)".to_string()
                            })
                    })
                    .collect();
                table.query().nearest_to(
                    v.map_err(|e| invalid(format!("Failed to parse vector_f16: {}", e)))?,
                )?
            } else if let Some(arr) = vec_u8_arr {
                // u8 vectors travel as a JSON array of unsigned numbers
                // (each value in 0..=255). lancedb v0.29 has no
                // IntoQueryVector impl for &[u8], so we build a 1-D
                // Arrow UInt8 array and route through the Arc<dyn Array>
                // impl. Strict range validation: any element outside
                // 0..=255 — string, negative, float, oversized integer —
                // surfaces as a clear error rather than wrapping.
                use std::sync::Arc;
                let v: Result<Vec<u8>, String> = arr
                    .iter()
                    .map(|x| {
                        x.as_u64()
                            .and_then(|u| u8::try_from(u).ok())
                            .ok_or_else(|| {
                                format!("Invalid u8 element (must be 0..=255), got {}", x)
                            })
                    })
                    .collect();
                let bytes = v.map_err(|e| invalid(format!("Failed to parse vector_u8: {}", e)))?;
                let arr_obj: Arc<dyn arrow_array::Array> =
                    Arc::new(arrow_array::UInt8Array::from(bytes));
                table.query().nearest_to(arr_obj)?
            } else {
                // f32 default (also covers the legacy {"vector": [...]} shape).
                let arr = vec_f32_arr.expect("dtype_count == 1 guard ensures vec_f32_arr is set");
                let v: Result<Vec<f32>, String> = arr
                    .iter()
                    .map(|x| {
                        x.as_f64()
                            .map(|f| f as f32)
                            .ok_or_else(|| "Invalid f32 element".to_string())
                    })
                    .collect();
                table
                    .query()
                    .nearest_to(v.map_err(|e| invalid(format!("Failed to parse vector: {}", e)))?)?
            };

            let mut vector_query = nearest_built.column(column).limit(effective_limit);

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

            // Hybrid: when a full_text_query is present alongside the
            // vector, chain .full_text_search() so lancedb's
            // execute_hybrid path fuses the two channels. The default
            // reranker is RRF; the caller can override via the
            // top-level "reranker" config.
            if let Some(fts_text) = vector_search
                .get("full_text_query")
                .and_then(|v| v.as_str())
            {
                // Trim before the empty check: a whitespace-only
                // query like "   " would otherwise reach
                // FullTextSearchQuery::new and produce an empty
                // tokenizer result, surfacing as either no rows
                // or a backend error depending on the FTS index.
                let trimmed = fts_text.trim();
                if !trimmed.is_empty() {
                    let mut fts = FullTextSearchQuery::new(trimmed.to_string());
                    if let Some(col) = vector_search
                        .get("full_text_column")
                        .and_then(|v| v.as_str())
                    {
                        if !col.is_empty() {
                            fts = fts.with_column(col.to_string()).map_err(|e| {
                                lancedb::Error::InvalidInput {
                                    message: format!("Invalid FTS column: {}", e),
                                }
                            })?;
                        }
                    }
                    vector_query = vector_query.full_text_search(fts);
                }
            }

            vector_query = apply_query_base_flags(vector_query, query_config)?;

            return vector_query.execute().await;
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

        fts_query = apply_query_base_flags(fts_query, query_config)?;

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

    query = apply_query_base_flags(query, query_config)?;

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

#[cfg(test)]
mod tests {
    use super::*;

    // Both a missing reranker key and an explicit null must be treated
    // as "no reranker configured". Go's omitempty cannot drop a non-nil
    // *RerankerConfig pointer, so users who hand-build QueryConfig with
    // RerankerNone end up sending the null form.
    #[test]
    fn parse_reranker_treats_missing_and_null_as_none() {
        let no_key = serde_json::json!({});
        let (r, n) = parse_reranker(&no_key).unwrap();
        assert!(r.is_none() && n.is_none(), "missing reranker key");

        let null = serde_json::json!({"reranker": null});
        let (r, n) = parse_reranker(&null).unwrap();
        assert!(r.is_none() && n.is_none(), "explicit null reranker");
    }

    #[test]
    fn parse_reranker_rejects_unknown_kind() {
        let bad = serde_json::json!({"reranker": {"kind": "what"}});
        let err = parse_reranker(&bad).expect_err("unknown kind must error");
        let msg = err.to_string();
        assert!(msg.contains("Unknown reranker kind"), "got: {}", msg);
    }
}
