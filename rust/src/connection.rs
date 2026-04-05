// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: Copyright The LanceDB Authors

//! Connection management operations

use crate::ffi::{from_c_str, SimpleResult};
use crate::runtime::get_simple_runtime;
use lancedb::connect;
use std::collections::HashMap;
use std::os::raw::{c_char, c_void};

/// Connect to a LanceDB database (simple version)
#[no_mangle]
#[allow(clippy::not_unsafe_ptr_arg_deref)]
pub extern "C" fn simple_lancedb_connect(
    uri: *const c_char,
    handle: *mut *mut c_void,
) -> *mut SimpleResult {
    let result = std::panic::catch_unwind(|| -> SimpleResult {
        if uri.is_null() || handle.is_null() {
            return SimpleResult::error("Invalid null arguments".to_string());
        }

        let uri_str = match from_c_str(uri) {
            Ok(s) => s,
            Err(e) => return SimpleResult::error(format!("Invalid URI: {}", e)),
        };

        let rt = get_simple_runtime();

        match rt.block_on(async { connect(&uri_str).execute().await }) {
            Ok(conn) => {
                let boxed_conn = Box::new(conn);
                unsafe {
                    *handle = Box::into_raw(boxed_conn) as *mut c_void;
                }
                SimpleResult::ok()
            }
            Err(e) => SimpleResult::error(e.to_string()),
        }
    });

    match result {
        Ok(res) => Box::into_raw(Box::new(res)),
        Err(_) => Box::into_raw(Box::new(SimpleResult::error(
            "Panic in simple_lancedb_connect".to_string(),
        ))),
    }
}

/// Connect to a database with storage options
#[no_mangle]
#[allow(clippy::not_unsafe_ptr_arg_deref)]
pub extern "C" fn simple_lancedb_connect_with_options(
    uri: *const c_char,
    options_json: *const c_char,
    handle: *mut *mut c_void,
) -> *mut SimpleResult {
    let result = std::panic::catch_unwind(|| -> SimpleResult {
        if uri.is_null() || options_json.is_null() || handle.is_null() {
            return SimpleResult::error("Invalid null arguments".to_string());
        }

        let uri_str = match from_c_str(uri) {
            Ok(s) => s,
            Err(e) => return SimpleResult::error(format!("Invalid URI: {}", e)),
        };

        let options_str = match from_c_str(options_json) {
            Ok(s) => s,
            Err(e) => return SimpleResult::error(format!("Invalid options JSON: {}", e)),
        };

        // Parse storage options as flat key-value map
        let storage_options: HashMap<String, String> = match serde_json::from_str(&options_str) {
            Ok(opts) => opts,
            Err(e) => {
                return SimpleResult::error(format!("Failed to parse storage options JSON: {}", e))
            }
        };

        let rt = get_simple_runtime();

        match rt.block_on(async {
            connect(&uri_str)
                .storage_options(storage_options)
                .execute()
                .await
        }) {
            Ok(conn) => {
                let boxed_conn = Box::new(conn);
                unsafe {
                    *handle = Box::into_raw(boxed_conn) as *mut c_void;
                }
                SimpleResult::ok()
            }
            Err(e) => SimpleResult::error(e.to_string()),
        }
    });

    match result {
        Ok(res) => Box::into_raw(Box::new(res)),
        Err(_) => Box::into_raw(Box::new(SimpleResult::error(
            "Panic in simple_lancedb_connect_with_options".to_string(),
        ))),
    }
}

/// Close a connection
#[no_mangle]
pub extern "C" fn simple_lancedb_close(handle: *mut c_void) -> *mut SimpleResult {
    if handle.is_null() {
        return Box::into_raw(Box::new(SimpleResult::error(
            "Invalid null handle".to_string(),
        )));
    }

    let result = std::panic::catch_unwind(|| -> SimpleResult {
        unsafe {
            let _conn = Box::from_raw(handle as *mut lancedb::Connection);
            // Connection will be dropped here
        }
        SimpleResult::ok()
    });

    match result {
        Ok(res) => Box::into_raw(Box::new(res)),
        Err(_) => Box::into_raw(Box::new(SimpleResult::error(
            "Panic in simple_lancedb_close".to_string(),
        ))),
    }
}
