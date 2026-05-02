// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: Copyright The LanceDB Authors

//! Time-travel surface — version history, checkout/restore, and tag CRUD.
//!
//! Every entry point follows the same envelope as the rest of the simple
//! FFI: a heap-allocated SimpleResult, panic-safe execution under the
//! shared simple runtime, and snake_case JSON for any structured payload.
//! TagContents is wrapped in a local struct because upstream
//! lance::dataset::refs::TagContents serializes camelCase, which would
//! be a silent footgun for Go callers.

use crate::ffi::{from_c_str, SimpleResult};
use crate::runtime::get_simple_runtime;
use std::ffi::CString;
use std::os::raw::{c_char, c_void};

/// List every version reachable from the dataset. Returns a JSON array
/// of {version, timestamp, metadata} objects ordered as reported by the
/// backend. Caller owns versions_json and must free it with
/// simple_lancedb_free_string.
#[no_mangle]
#[allow(clippy::not_unsafe_ptr_arg_deref)]
pub extern "C" fn simple_lancedb_table_list_versions(
    table_handle: *mut c_void,
    versions_json: *mut *mut c_char,
) -> *mut SimpleResult {
    let result = std::panic::catch_unwind(|| -> SimpleResult {
        if table_handle.is_null() || versions_json.is_null() {
            return SimpleResult::error("Invalid null arguments".to_string());
        }

        let table = unsafe { &*(table_handle as *const lancedb::Table) };
        let rt = get_simple_runtime();

        match rt.block_on(async { table.list_versions().await }) {
            Ok(versions) => {
                let mapped: Vec<serde_json::Value> = versions
                    .into_iter()
                    .map(|v| {
                        serde_json::json!({
                            "version": v.version,
                            // RFC3339 string — stable, timezone-aware, parseable by Go's time.Parse.
                            "timestamp": v.timestamp.to_rfc3339(),
                            "metadata": v.metadata.into_iter().collect::<std::collections::BTreeMap<_, _>>(),
                        })
                    })
                    .collect();

                match serde_json::to_string(&mapped) {
                    Ok(json_str) => match CString::new(json_str) {
                        Ok(c_string) => {
                            unsafe {
                                *versions_json = c_string.into_raw();
                            }
                            SimpleResult::ok()
                        }
                        Err(_) => {
                            SimpleResult::error("Failed to convert JSON to C string".to_string())
                        }
                    },
                    Err(e) => SimpleResult::error(format!("Failed to serialize versions: {}", e)),
                }
            }
            Err(e) => SimpleResult::error(format!("Failed to list versions: {}", e)),
        }
    });

    match result {
        Ok(res) => Box::into_raw(Box::new(res)),
        Err(_) => Box::into_raw(Box::new(SimpleResult::error(
            "Panic in simple_lancedb_table_list_versions".to_string(),
        ))),
    }
}

/// Pin the table to a specific version. Subsequent reads see that
/// snapshot; writes are rejected until the table is brought back with
/// either checkout_latest or restore. Mirrors lancedb::Table::checkout.
#[no_mangle]
#[allow(clippy::not_unsafe_ptr_arg_deref)]
pub extern "C" fn simple_lancedb_table_checkout(
    table_handle: *mut c_void,
    version: u64,
) -> *mut SimpleResult {
    let result = std::panic::catch_unwind(|| -> SimpleResult {
        if table_handle.is_null() {
            return SimpleResult::error("Invalid null table handle".to_string());
        }
        let table = unsafe { &*(table_handle as *const lancedb::Table) };
        let rt = get_simple_runtime();
        match rt.block_on(async { table.checkout(version).await }) {
            Ok(()) => SimpleResult::ok(),
            Err(e) => SimpleResult::error(format!("checkout failed: {}", e)),
        }
    });

    match result {
        Ok(res) => Box::into_raw(Box::new(res)),
        Err(_) => Box::into_raw(Box::new(SimpleResult::error(
            "Panic in simple_lancedb_table_checkout".to_string(),
        ))),
    }
}

/// Pin the table to the version referenced by the given tag.
#[no_mangle]
#[allow(clippy::not_unsafe_ptr_arg_deref)]
pub extern "C" fn simple_lancedb_table_checkout_tag(
    table_handle: *mut c_void,
    tag: *const c_char,
) -> *mut SimpleResult {
    let result = std::panic::catch_unwind(|| -> SimpleResult {
        if table_handle.is_null() || tag.is_null() {
            return SimpleResult::error("Invalid null arguments".to_string());
        }
        let tag_str = match from_c_str(tag) {
            Ok(s) => s,
            Err(e) => return SimpleResult::error(format!("Invalid tag: {}", e)),
        };
        let table = unsafe { &*(table_handle as *const lancedb::Table) };
        let rt = get_simple_runtime();
        match rt.block_on(async { table.checkout_tag(&tag_str).await }) {
            Ok(()) => SimpleResult::ok(),
            Err(e) => SimpleResult::error(format!("checkout_tag failed: {}", e)),
        }
    });

    match result {
        Ok(res) => Box::into_raw(Box::new(res)),
        Err(_) => Box::into_raw(Box::new(SimpleResult::error(
            "Panic in simple_lancedb_table_checkout_tag".to_string(),
        ))),
    }
}

/// Drop any prior checkout pin and resume tracking the latest manifest.
#[no_mangle]
#[allow(clippy::not_unsafe_ptr_arg_deref)]
pub extern "C" fn simple_lancedb_table_checkout_latest(
    table_handle: *mut c_void,
) -> *mut SimpleResult {
    let result = std::panic::catch_unwind(|| -> SimpleResult {
        if table_handle.is_null() {
            return SimpleResult::error("Invalid null table handle".to_string());
        }
        let table = unsafe { &*(table_handle as *const lancedb::Table) };
        let rt = get_simple_runtime();
        match rt.block_on(async { table.checkout_latest().await }) {
            Ok(()) => SimpleResult::ok(),
            Err(e) => SimpleResult::error(format!("checkout_latest failed: {}", e)),
        }
    });

    match result {
        Ok(res) => Box::into_raw(Box::new(res)),
        Err(_) => Box::into_raw(Box::new(SimpleResult::error(
            "Panic in simple_lancedb_table_checkout_latest".to_string(),
        ))),
    }
}

/// Promote the currently checked-out version to a new latest manifest.
/// Errors if the table is not in a checked-out state. Mirrors
/// lancedb::Table::restore exactly — the Python `restore(version)`
/// convenience overload is not available here; callers do
/// checkout(N) -> restore() in two steps.
#[no_mangle]
#[allow(clippy::not_unsafe_ptr_arg_deref)]
pub extern "C" fn simple_lancedb_table_restore(table_handle: *mut c_void) -> *mut SimpleResult {
    let result = std::panic::catch_unwind(|| -> SimpleResult {
        if table_handle.is_null() {
            return SimpleResult::error("Invalid null table handle".to_string());
        }
        let table = unsafe { &*(table_handle as *const lancedb::Table) };
        let rt = get_simple_runtime();
        match rt.block_on(async { table.restore().await }) {
            Ok(()) => SimpleResult::ok(),
            Err(e) => SimpleResult::error(format!("restore failed: {}", e)),
        }
    });

    match result {
        Ok(res) => Box::into_raw(Box::new(res)),
        Err(_) => Box::into_raw(Box::new(SimpleResult::error(
            "Panic in simple_lancedb_table_restore".to_string(),
        ))),
    }
}

/// List every tag on the table as a JSON object keyed by tag name.
/// Each value carries the pinned version, manifest_size, and an
/// optional branch (currently always absent for tags created via the
/// FFI). Caller owns tags_json and must free it with
/// simple_lancedb_free_string.
#[no_mangle]
#[allow(clippy::not_unsafe_ptr_arg_deref)]
pub extern "C" fn simple_lancedb_table_tags_list(
    table_handle: *mut c_void,
    tags_json: *mut *mut c_char,
) -> *mut SimpleResult {
    let result = std::panic::catch_unwind(|| -> SimpleResult {
        if table_handle.is_null() || tags_json.is_null() {
            return SimpleResult::error("Invalid null arguments".to_string());
        }

        let table = unsafe { &*(table_handle as *const lancedb::Table) };
        let rt = get_simple_runtime();

        match rt.block_on(async {
            let tags = table.tags().await?;
            tags.list().await
        }) {
            Ok(map) => {
                let mapped: std::collections::BTreeMap<String, serde_json::Value> = map
                    .into_iter()
                    .map(|(k, v)| {
                        // Branch is omitted entirely when None to match
                        // serde(skip_serializing_if) ergonomics on the Go
                        // side. manifest_size widened to u64 for FFI
                        // stability — usize is platform-dependent.
                        let mut obj = serde_json::Map::new();
                        obj.insert("version".to_string(), serde_json::Value::from(v.version));
                        obj.insert(
                            "manifest_size".to_string(),
                            serde_json::Value::from(v.manifest_size as u64),
                        );
                        if let Some(branch) = v.branch {
                            obj.insert("branch".to_string(), serde_json::Value::from(branch));
                        }
                        (k, serde_json::Value::Object(obj))
                    })
                    .collect();

                match serde_json::to_string(&mapped) {
                    Ok(json_str) => match CString::new(json_str) {
                        Ok(c_string) => {
                            unsafe {
                                *tags_json = c_string.into_raw();
                            }
                            SimpleResult::ok()
                        }
                        Err(_) => {
                            SimpleResult::error("Failed to convert JSON to C string".to_string())
                        }
                    },
                    Err(e) => SimpleResult::error(format!("Failed to serialize tags: {}", e)),
                }
            }
            Err(e) => SimpleResult::error(format!("Failed to list tags: {}", e)),
        }
    });

    match result {
        Ok(res) => Box::into_raw(Box::new(res)),
        Err(_) => Box::into_raw(Box::new(SimpleResult::error(
            "Panic in simple_lancedb_table_tags_list".to_string(),
        ))),
    }
}

/// Resolve a tag to its pinned version. Errors when the tag does not
/// exist. The version is written to *version_out on success.
#[no_mangle]
#[allow(clippy::not_unsafe_ptr_arg_deref)]
pub extern "C" fn simple_lancedb_table_tags_get_version(
    table_handle: *mut c_void,
    tag: *const c_char,
    version_out: *mut u64,
) -> *mut SimpleResult {
    let result = std::panic::catch_unwind(|| -> SimpleResult {
        if table_handle.is_null() || tag.is_null() || version_out.is_null() {
            return SimpleResult::error("Invalid null arguments".to_string());
        }
        let tag_str = match from_c_str(tag) {
            Ok(s) => s,
            Err(e) => return SimpleResult::error(format!("Invalid tag: {}", e)),
        };
        let table = unsafe { &*(table_handle as *const lancedb::Table) };
        let rt = get_simple_runtime();
        match rt.block_on(async {
            let tags = table.tags().await?;
            tags.get_version(&tag_str).await
        }) {
            Ok(v) => {
                unsafe {
                    *version_out = v;
                }
                SimpleResult::ok()
            }
            Err(e) => SimpleResult::error(format!("tags.get_version failed: {}", e)),
        }
    });

    match result {
        Ok(res) => Box::into_raw(Box::new(res)),
        Err(_) => Box::into_raw(Box::new(SimpleResult::error(
            "Panic in simple_lancedb_table_tags_get_version".to_string(),
        ))),
    }
}

/// Create a new tag pointing at the given version. Errors if the tag
/// already exists or the version is unknown.
#[no_mangle]
#[allow(clippy::not_unsafe_ptr_arg_deref)]
pub extern "C" fn simple_lancedb_table_tags_create(
    table_handle: *mut c_void,
    tag: *const c_char,
    version: u64,
) -> *mut SimpleResult {
    let result = std::panic::catch_unwind(|| -> SimpleResult {
        if table_handle.is_null() || tag.is_null() {
            return SimpleResult::error("Invalid null arguments".to_string());
        }
        let tag_str = match from_c_str(tag) {
            Ok(s) => s,
            Err(e) => return SimpleResult::error(format!("Invalid tag: {}", e)),
        };
        let table = unsafe { &*(table_handle as *const lancedb::Table) };
        let rt = get_simple_runtime();
        match rt.block_on(async {
            let mut tags = table.tags().await?;
            tags.create(&tag_str, version).await
        }) {
            Ok(()) => SimpleResult::ok(),
            Err(e) => SimpleResult::error(format!("tags.create failed: {}", e)),
        }
    });

    match result {
        Ok(res) => Box::into_raw(Box::new(res)),
        Err(_) => Box::into_raw(Box::new(SimpleResult::error(
            "Panic in simple_lancedb_table_tags_create".to_string(),
        ))),
    }
}

/// Delete a tag. Errors if the tag does not exist.
#[no_mangle]
#[allow(clippy::not_unsafe_ptr_arg_deref)]
pub extern "C" fn simple_lancedb_table_tags_delete(
    table_handle: *mut c_void,
    tag: *const c_char,
) -> *mut SimpleResult {
    let result = std::panic::catch_unwind(|| -> SimpleResult {
        if table_handle.is_null() || tag.is_null() {
            return SimpleResult::error("Invalid null arguments".to_string());
        }
        let tag_str = match from_c_str(tag) {
            Ok(s) => s,
            Err(e) => return SimpleResult::error(format!("Invalid tag: {}", e)),
        };
        let table = unsafe { &*(table_handle as *const lancedb::Table) };
        let rt = get_simple_runtime();
        match rt.block_on(async {
            let mut tags = table.tags().await?;
            tags.delete(&tag_str).await
        }) {
            Ok(()) => SimpleResult::ok(),
            Err(e) => SimpleResult::error(format!("tags.delete failed: {}", e)),
        }
    });

    match result {
        Ok(res) => Box::into_raw(Box::new(res)),
        Err(_) => Box::into_raw(Box::new(SimpleResult::error(
            "Panic in simple_lancedb_table_tags_delete".to_string(),
        ))),
    }
}

/// Move an existing tag to a new version. Errors if the tag does not
/// exist or the version is unknown.
#[no_mangle]
#[allow(clippy::not_unsafe_ptr_arg_deref)]
pub extern "C" fn simple_lancedb_table_tags_update(
    table_handle: *mut c_void,
    tag: *const c_char,
    version: u64,
) -> *mut SimpleResult {
    let result = std::panic::catch_unwind(|| -> SimpleResult {
        if table_handle.is_null() || tag.is_null() {
            return SimpleResult::error("Invalid null arguments".to_string());
        }
        let tag_str = match from_c_str(tag) {
            Ok(s) => s,
            Err(e) => return SimpleResult::error(format!("Invalid tag: {}", e)),
        };
        let table = unsafe { &*(table_handle as *const lancedb::Table) };
        let rt = get_simple_runtime();
        match rt.block_on(async {
            let mut tags = table.tags().await?;
            tags.update(&tag_str, version).await
        }) {
            Ok(()) => SimpleResult::ok(),
            Err(e) => SimpleResult::error(format!("tags.update failed: {}", e)),
        }
    });

    match result {
        Ok(res) => Box::into_raw(Box::new(res)),
        Err(_) => Box::into_raw(Box::new(SimpleResult::error(
            "Panic in simple_lancedb_table_tags_update".to_string(),
        ))),
    }
}
