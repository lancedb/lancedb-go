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
