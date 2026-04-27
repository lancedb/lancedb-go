// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: Copyright The LanceDB Authors

//! Index management operations

use crate::ffi::{from_c_str, SimpleResult};
use crate::runtime::get_simple_runtime;
use std::ffi::CString;
use std::os::raw::{c_char, c_void};
use std::time::Duration;

/// Create an index on the specified columns
#[no_mangle]
pub extern "C" fn simple_lancedb_table_create_index(
    table_handle: *mut c_void,
    columns_json: *const c_char,
    index_type: *const c_char,
    index_name: *const c_char,
) -> *mut SimpleResult {
    let result = std::panic::catch_unwind(|| -> SimpleResult {
        if table_handle.is_null() || columns_json.is_null() || index_type.is_null() {
            return SimpleResult::error("Invalid null arguments".to_string());
        }

        let columns_str = match from_c_str(columns_json) {
            Ok(s) => s,
            Err(e) => return SimpleResult::error(format!("Invalid columns JSON: {}", e)),
        };

        let index_type_str = match from_c_str(index_type) {
            Ok(s) => s,
            Err(e) => return SimpleResult::error(format!("Invalid index type: {}", e)),
        };

        let index_name_str = if index_name.is_null() {
            None
        } else {
            match from_c_str(index_name) {
                Ok(s) => Some(s),
                Err(e) => return SimpleResult::error(format!("Invalid index name: {}", e)),
            }
        };

        // Parse columns JSON
        let columns: Vec<String> = match serde_json::from_str(&columns_str) {
            Ok(cols) => cols,
            Err(e) => return SimpleResult::error(format!("Failed to parse columns JSON: {}", e)),
        };

        let table = unsafe { &*(table_handle as *const lancedb::Table) };
        let rt = get_simple_runtime();

        // Map index type string to LanceDB index type
        let index_result = match index_type_str.as_str() {
            "vector" | "ivf_pq" => {
                // Create vector index (IVF_PQ)
                rt.block_on(async {
                    let mut index_builder = table.create_index(
                        &columns,
                        lancedb::index::Index::IvfPq(
                            lancedb::index::vector::IvfPqIndexBuilder::default(),
                        ),
                    );

                    if let Some(name) = index_name_str {
                        index_builder = index_builder.name(name);
                    }

                    index_builder.execute().await
                })
            }
            "ivf_flat" => rt.block_on(async {
                let mut index_builder = table.create_index(
                    &columns,
                    lancedb::index::Index::IvfFlat(
                        lancedb::index::vector::IvfFlatIndexBuilder::default(),
                    ),
                );

                if let Some(name) = index_name_str {
                    index_builder = index_builder.name(name);
                }

                index_builder.execute().await
            }),
            "hnsw_pq" => rt.block_on(async {
                let mut index_builder = table.create_index(
                    &columns,
                    lancedb::index::Index::IvfHnswPq(
                        lancedb::index::vector::IvfHnswPqIndexBuilder::default(),
                    ),
                );

                if let Some(name) = index_name_str {
                    index_builder = index_builder.name(name);
                }

                index_builder.execute().await
            }),
            "hnsw_sq" => rt.block_on(async {
                let mut index_builder = table.create_index(
                    &columns,
                    lancedb::index::Index::IvfHnswSq(
                        lancedb::index::vector::IvfHnswSqIndexBuilder::default(),
                    ),
                );

                if let Some(name) = index_name_str {
                    index_builder = index_builder.name(name);
                }

                index_builder.execute().await
            }),
            "btree" => rt.block_on(async {
                let mut index_builder = table.create_index(
                    &columns,
                    lancedb::index::Index::BTree(lancedb::index::scalar::BTreeIndexBuilder {}),
                );

                if let Some(name) = index_name_str {
                    index_builder = index_builder.name(name);
                }

                index_builder.execute().await
            }),
            "bitmap" => rt.block_on(async {
                let mut index_builder = table.create_index(
                    &columns,
                    lancedb::index::Index::Bitmap(lancedb::index::scalar::BitmapIndexBuilder {}),
                );

                if let Some(name) = index_name_str {
                    index_builder = index_builder.name(name);
                }

                index_builder.execute().await
            }),
            "label_list" => rt.block_on(async {
                let mut index_builder = table.create_index(
                    &columns,
                    lancedb::index::Index::LabelList(
                        lancedb::index::scalar::LabelListIndexBuilder {},
                    ),
                );

                if let Some(name) = index_name_str {
                    index_builder = index_builder.name(name);
                }

                index_builder.execute().await
            }),
            "fts" => rt.block_on(async {
                let mut index_builder = table.create_index(
                    &columns,
                    lancedb::index::Index::FTS(lancedb::index::scalar::FtsIndexBuilder::default()),
                );

                if let Some(name) = index_name_str {
                    index_builder = index_builder.name(name);
                }

                index_builder.execute().await
            }),
            _ => return SimpleResult::error(format!("Unsupported index type: {}", index_type_str)),
        };

        match index_result {
            Ok(_) => SimpleResult::ok(),
            Err(e) => SimpleResult::error(format!("Failed to create index: {}", e)),
        }
    });

    match result {
        Ok(res) => Box::into_raw(Box::new(res)),
        Err(_) => Box::into_raw(Box::new(SimpleResult::error(
            "Panic in simple_lancedb_table_create_index".to_string(),
        ))),
    }
}

/// Get all indexes for a table (returns JSON string)
#[no_mangle]
#[allow(clippy::not_unsafe_ptr_arg_deref)]
pub extern "C" fn simple_lancedb_table_get_indexes(
    table_handle: *mut c_void,
    indexes_json: *mut *mut c_char,
) -> *mut SimpleResult {
    let result = std::panic::catch_unwind(|| -> SimpleResult {
        if table_handle.is_null() || indexes_json.is_null() {
            return SimpleResult::error("Invalid null arguments".to_string());
        }

        let table = unsafe { &*(table_handle as *const lancedb::Table) };
        let rt = get_simple_runtime();

        match rt.block_on(async { table.list_indices().await }) {
            Ok(indexes) => {
                // Convert the indexes to a JSON-serializable format
                let mut index_info_list = Vec::new();

                for index in indexes {
                    let index_info = serde_json::json!({
                        "name": index.name,
                        "columns": index.columns,
                        "index_type": format!("{:?}", index.index_type),
                    });
                    index_info_list.push(index_info);
                }

                match serde_json::to_string(&index_info_list) {
                    Ok(json_str) => match CString::new(json_str) {
                        Ok(c_string) => {
                            unsafe {
                                *indexes_json = c_string.into_raw();
                            }
                            SimpleResult::ok()
                        }
                        Err(_) => {
                            SimpleResult::error("Failed to convert JSON to C string".to_string())
                        }
                    },
                    Err(e) => {
                        SimpleResult::error(format!("Failed to serialize indexes to JSON: {}", e))
                    }
                }
            }
            Err(e) => SimpleResult::error(format!("Failed to list indexes: {}", e)),
        }
    });

    match result {
        Ok(res) => Box::into_raw(Box::new(res)),
        Err(_) => Box::into_raw(Box::new(SimpleResult::error(
            "Panic in simple_lancedb_table_get_indexes".to_string(),
        ))),
    }
}

/// Retrieve statistics about an index
#[no_mangle]
#[allow(clippy::not_unsafe_ptr_arg_deref)]
pub extern "C" fn simple_lancedb_table_index_stats(
    table_handle: *mut c_void,
    index_name: *const c_char,
    index_stats_json: *mut *mut c_char,
) -> *mut SimpleResult {
    let result = std::panic::catch_unwind(|| -> SimpleResult {
        if table_handle.is_null() || index_name.is_null() || index_stats_json.is_null() {
            return SimpleResult::error("Invalid null arguments".to_string());
        }

        let table = unsafe { &*(table_handle as *const lancedb::Table) };
        let rt = get_simple_runtime();

        let index_name_str = match from_c_str(index_name) {
            Ok(s) => s,
            Err(e) => return SimpleResult::error(format!("Invalid index name: {}", e)),
        };

        match rt.block_on(async { table.index_stats(index_name_str).await }) {
            Ok(Some(index_stats)) => {
                let stats_json = serde_json::json!({
                    "num_indexed_rows": index_stats.num_indexed_rows,
                    "num_unindexed_rows": index_stats.num_unindexed_rows,
                    "index_type": format!("{:?}", index_stats.index_type),
                    "distance_type": index_stats.distance_type,
                    "num_indices": index_stats.num_indices,
                    "loss": index_stats.loss,
                });

                match serde_json::to_string(&stats_json) {
                    Ok(json_str) => match CString::new(json_str) {
                        Ok(c_string) => {
                            unsafe {
                                *index_stats_json = c_string.into_raw();
                            }
                            SimpleResult::ok()
                        }
                        Err(_) => {
                            SimpleResult::error("Failed to convert JSON to C string".to_string())
                        }
                    },
                    Err(e) => {
                        SimpleResult::error(format!("Failed to serialize indexes to JSON: {}", e))
                    }
                }
            }
            Ok(None) => SimpleResult::ok(),
            Err(e) => SimpleResult::error(format!("Failed to get index stats: {}", e)),
        }
    });

    match result {
        Ok(res) => Box::into_raw(Box::new(res)),
        Err(_) => Box::into_raw(Box::new(SimpleResult::error(
            "Panic in simple_lancedb_table_index_stats".to_string(),
        ))),
    }
}

/// Wait for the named indices to finish building, with a timeout in
/// milliseconds. An empty `index_names` array defaults to all indices on
/// the table. A `timeout_ms` value of 0 means "wait essentially forever"
/// (Duration::MAX). The call blocks the calling thread until either all
/// listed indices report no unindexed rows or the deadline elapses.
///
/// Returns SimpleResult::ok() on success, or SimpleResult::error() with a
/// backend-supplied message on timeout / missing index / I/O failure.
#[no_mangle]
#[allow(clippy::not_unsafe_ptr_arg_deref)]
pub extern "C" fn simple_lancedb_table_wait_for_index(
    table_handle: *mut c_void,
    index_names: *const *const c_char,
    index_names_count: usize,
    timeout_ms: u64,
) -> *mut SimpleResult {
    let result = std::panic::catch_unwind(|| -> SimpleResult {
        if table_handle.is_null() {
            return SimpleResult::error("Invalid null table handle".to_string());
        }
        if index_names_count > 0 && index_names.is_null() {
            return SimpleResult::error(
                "Non-zero index_names_count requires a non-null array".to_string(),
            );
        }

        // Materialise the C string array into Vec<String> so we own the
        // backing memory for the borrowed Vec<&str> we hand to lancedb.
        let mut names_owned: Vec<String> = Vec::with_capacity(index_names_count);
        for i in 0..index_names_count {
            // SAFETY: caller guarantees index_names points to at least
            // index_names_count valid *const c_char entries.
            let raw = unsafe { *index_names.add(i) };
            if raw.is_null() {
                return SimpleResult::error(format!("index_names[{}] is a null pointer", i));
            }
            match from_c_str(raw) {
                Ok(s) => names_owned.push(s),
                Err(e) => {
                    return SimpleResult::error(format!(
                        "Invalid UTF-8 in index_names[{}]: {}",
                        i, e
                    ))
                }
            }
        }
        let names_borrowed: Vec<&str> = names_owned.iter().map(String::as_str).collect();

        // 0 means "effectively forever"; the caller can still cancel from
        // the Go side by letting the table drop.
        let timeout = if timeout_ms == 0 {
            Duration::MAX
        } else {
            Duration::from_millis(timeout_ms)
        };

        let table = unsafe { &*(table_handle as *const lancedb::Table) };
        let rt = get_simple_runtime();

        match rt.block_on(async { table.wait_for_index(&names_borrowed, timeout).await }) {
            Ok(()) => SimpleResult::ok(),
            Err(e) => SimpleResult::error(format!("wait_for_index failed: {}", e)),
        }
    });

    match result {
        Ok(res) => Box::into_raw(Box::new(res)),
        Err(_) => Box::into_raw(Box::new(SimpleResult::error(
            "Panic in simple_lancedb_table_wait_for_index".to_string(),
        ))),
    }
}

/// Build a lancedb::index::Index from the config JSON "type" field plus any
/// per-kind tuning parameters present on the config object. Returns the
/// resulting Index or a user-facing error string.
fn build_index_from_config(cfg: &serde_json::Value) -> Result<lancedb::index::Index, String> {
    use lancedb::index::scalar::{
        BTreeIndexBuilder, BitmapIndexBuilder, FtsIndexBuilder, LabelListIndexBuilder,
    };
    use lancedb::index::vector::{
        IvfFlatIndexBuilder, IvfHnswPqIndexBuilder, IvfHnswSqIndexBuilder, IvfPqIndexBuilder,
        IvfRqIndexBuilder, IvfSqIndexBuilder,
    };
    use lancedb::index::Index;

    let kind = cfg
        .get("type")
        .and_then(|v| v.as_str())
        .ok_or_else(|| "Missing required 'type' field in index config".to_string())?;

    let distance_type = match cfg.get("distance_type").and_then(|v| v.as_str()) {
        Some(dt) => Some(crate::query::parse_distance_type(dt).map_err(|e| e.to_string())?),
        None => None,
    };

    // Strict u32 extractor: rejects values that exceed u32::MAX rather
    // than silently truncating with `as u32` (which would map e.g.
    // 4294967296 to 0 and quietly mistune the index).
    let u32_opt = |k: &str| -> Result<Option<u32>, String> {
        match cfg.get(k).and_then(|v| v.as_u64()) {
            None => Ok(None),
            Some(n) => u32::try_from(n)
                .map(Some)
                .map_err(|_| format!("'{}' value {} does not fit in u32", k, n)),
        }
    };
    let bool_opt = |k: &str| -> Option<bool> { cfg.get(k).and_then(|v| v.as_bool()) };
    let str_opt = |k: &str| -> Option<&str> { cfg.get(k).and_then(|v| v.as_str()) };

    // Small macro-like helper: apply the set of IVF-common options.
    macro_rules! apply_ivf_common {
        ($b:ident, $set_distance:expr) => {{
            if $set_distance {
                if let Some(dt) = distance_type {
                    $b = $b.distance_type(dt);
                }
            }
            if let Some(n) = u32_opt("num_partitions")? {
                $b = $b.num_partitions(n);
            }
            if let Some(n) = u32_opt("sample_rate")? {
                $b = $b.sample_rate(n);
            }
            if let Some(n) = u32_opt("max_iterations")? {
                $b = $b.max_iterations(n);
            }
            if let Some(n) = u32_opt("target_partition_size")? {
                $b = $b.target_partition_size(n);
            }
        }};
    }

    match kind {
        "ivf_pq" | "vector" => {
            let mut b = IvfPqIndexBuilder::default();
            apply_ivf_common!(b, true);
            if let Some(n) = u32_opt("num_sub_vectors")? {
                b = b.num_sub_vectors(n);
            }
            if let Some(n) = u32_opt("num_bits")? {
                b = b.num_bits(n);
            }
            Ok(Index::IvfPq(b))
        }
        "ivf_flat" => {
            let mut b = IvfFlatIndexBuilder::default();
            apply_ivf_common!(b, true);
            Ok(Index::IvfFlat(b))
        }
        "ivf_sq" => {
            let mut b = IvfSqIndexBuilder::default();
            apply_ivf_common!(b, true);
            Ok(Index::IvfSq(b))
        }
        "ivf_rq" => {
            let mut b = IvfRqIndexBuilder::default();
            apply_ivf_common!(b, true);
            if let Some(n) = u32_opt("num_bits")? {
                b = b.num_bits(n);
            }
            Ok(Index::IvfRq(b))
        }
        "ivf_hnsw_pq" | "hnsw_pq" => {
            let mut b = IvfHnswPqIndexBuilder::default();
            apply_ivf_common!(b, true);
            if let Some(m) = u32_opt("m")? {
                b = b.num_edges(m);
            }
            if let Some(ef) = u32_opt("ef_construction")? {
                b = b.ef_construction(ef);
            }
            if let Some(n) = u32_opt("num_sub_vectors")? {
                b = b.num_sub_vectors(n);
            }
            if let Some(n) = u32_opt("num_bits")? {
                b = b.num_bits(n);
            }
            Ok(Index::IvfHnswPq(b))
        }
        "ivf_hnsw_sq" | "hnsw_sq" => {
            let mut b = IvfHnswSqIndexBuilder::default();
            apply_ivf_common!(b, true);
            if let Some(m) = u32_opt("m")? {
                b = b.num_edges(m);
            }
            if let Some(ef) = u32_opt("ef_construction")? {
                b = b.ef_construction(ef);
            }
            Ok(Index::IvfHnswSq(b))
        }
        "btree" => Ok(Index::BTree(BTreeIndexBuilder {})),
        "bitmap" => Ok(Index::Bitmap(BitmapIndexBuilder {})),
        "label_list" => Ok(Index::LabelList(LabelListIndexBuilder {})),
        "fts" => {
            let mut b = FtsIndexBuilder::default();
            if let Some(s) = str_opt("language") {
                b = b
                    .language(s)
                    .map_err(|e| format!("Invalid FTS language: {}", e))?;
            }
            if let Some(v) = bool_opt("with_position") {
                b = b.with_position(v);
            }
            if let Some(v) = bool_opt("stem") {
                b = b.stem(v);
            }
            if let Some(v) = bool_opt("remove_stop_words") {
                b = b.remove_stop_words(v);
            }
            if let Some(v) = bool_opt("lower_case") {
                b = b.lower_case(v);
            }
            if let Some(v) = bool_opt("ascii_folding") {
                b = b.ascii_folding(v);
            }
            if let Some(s) = str_opt("base_tokenizer") {
                b = b.base_tokenizer(s.to_string());
            }
            if let Some(n) = cfg.get("max_token_length").and_then(|v| v.as_u64()) {
                let v = usize::try_from(n)
                    .map_err(|_| format!("'max_token_length' value {} does not fit in usize", n))?;
                b = b.max_token_length(Some(v));
            }
            if let Some(n) = u32_opt("ngram_min_length")? {
                b = b.ngram_min_length(n);
            }
            if let Some(n) = u32_opt("ngram_max_length")? {
                b = b.ngram_max_length(n);
            }
            if let Some(v) = bool_opt("ngram_prefix_only") {
                b = b.ngram_prefix_only(v);
            }
            Ok(Index::FTS(b))
        }
        other => Err(format!("Unsupported index type: {}", other)),
    }
}

/// Create an index with full tuning parameters. The `config_json` is the
/// canonical surface for per-type tuning (num_partitions, m, ef_construction,
/// FTS options, etc.); the small top-level knobs (name/replace/
/// wait_timeout_ms) are passed as regular arguments so callers don't have to
/// encode them into JSON just to change a single bool.
///
/// - `name` may be NULL for the backend default.
/// - `replace` matches lancedb's IndexBuilder::replace (default true in
///   upstream; we forward the caller's choice verbatim).
/// - `wait_timeout_ms == 0` leaves the backend default (no wait).
#[no_mangle]
#[allow(clippy::not_unsafe_ptr_arg_deref)]
pub extern "C" fn simple_lancedb_table_create_index_v2(
    table_handle: *mut c_void,
    columns_json: *const c_char,
    config_json: *const c_char,
    name: *const c_char,
    replace: bool,
    wait_timeout_ms: u64,
) -> *mut SimpleResult {
    let result = std::panic::catch_unwind(|| -> SimpleResult {
        if table_handle.is_null() || columns_json.is_null() || config_json.is_null() {
            return SimpleResult::error("Invalid null arguments".to_string());
        }

        let columns_str = match from_c_str(columns_json) {
            Ok(s) => s,
            Err(e) => return SimpleResult::error(format!("Invalid columns JSON: {}", e)),
        };
        let columns: Vec<String> = match serde_json::from_str(&columns_str) {
            Ok(c) => c,
            Err(e) => return SimpleResult::error(format!("Failed to parse columns JSON: {}", e)),
        };

        let cfg_str = match from_c_str(config_json) {
            Ok(s) => s,
            Err(e) => return SimpleResult::error(format!("Invalid config JSON: {}", e)),
        };
        let cfg: serde_json::Value = match serde_json::from_str(&cfg_str) {
            Ok(v) => v,
            Err(e) => return SimpleResult::error(format!("Failed to parse index config: {}", e)),
        };

        let name_owned: Option<String> = if name.is_null() {
            None
        } else {
            match from_c_str(name) {
                Ok(s) if s.is_empty() => None,
                Ok(s) => Some(s),
                Err(e) => return SimpleResult::error(format!("Invalid index name: {}", e)),
            }
        };

        let index = match build_index_from_config(&cfg) {
            Ok(i) => i,
            Err(e) => return SimpleResult::error(e),
        };

        let table = unsafe { &*(table_handle as *const lancedb::Table) };
        let rt = get_simple_runtime();

        let outcome = rt.block_on(async {
            let mut builder = table.create_index(&columns, index);
            if let Some(n) = name_owned {
                builder = builder.name(n);
            }
            builder = builder.replace(replace);
            if wait_timeout_ms > 0 {
                builder = builder.wait_timeout(Duration::from_millis(wait_timeout_ms));
            }
            builder.execute().await
        });

        match outcome {
            Ok(_) => SimpleResult::ok(),
            Err(e) => SimpleResult::error(format!("create_index_v2 failed: {}", e)),
        }
    });

    match result {
        Ok(res) => Box::into_raw(Box::new(res)),
        Err(_) => Box::into_raw(Box::new(SimpleResult::error(
            "Panic in simple_lancedb_table_create_index_v2".to_string(),
        ))),
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    // Pin the u32_opt range check — silently truncating values above
    // u32::MAX with `as u32` would mistune the index without surfacing
    // an error (e.g. 4294967296 would become 0). The fix uses
    // u32::try_from and returns a user-facing error.
    #[test]
    fn build_index_from_config_rejects_u32_overflow() {
        let cfg = serde_json::json!({
            "type": "ivf_pq",
            // 2^32 — one above u32::MAX
            "num_partitions": 4_294_967_296u64,
        });
        let err = build_index_from_config(&cfg)
            .expect_err("u32-overflow num_partitions must fail, not silently truncate");
        assert!(
            err.contains("num_partitions") && err.contains("u32"),
            "error should mention the offending field and the type, got: {}",
            err
        );
    }

    // u32::MAX exactly is the largest accepted value. Spot-check the
    // boundary so the rewrite doesn't accidentally slide off-by-one.
    #[test]
    fn build_index_from_config_accepts_u32_max() {
        let cfg = serde_json::json!({
            "type": "ivf_pq",
            "num_partitions": u32::MAX as u64,
        });
        let result = build_index_from_config(&cfg);
        assert!(
            result.is_ok(),
            "u32::MAX should be accepted, got error: {:?}",
            result.err()
        );
    }
}
