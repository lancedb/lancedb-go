// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: Copyright The LanceDB Authors

//! Connection management operations

use crate::ffi::{from_c_str, SimpleResult};
use crate::runtime::get_simple_runtime;
use lancedb::connect;
use std::os::raw::{c_char, c_void};

/// Connect to a LanceDB database (simple version)
#[no_mangle]
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

        // Parse storage options from JSON
        let storage_options: serde_json::Value = match serde_json::from_str(&options_str) {
            Ok(opts) => opts,
            Err(e) => {
                return SimpleResult::error(format!("Failed to parse storage options JSON: {}", e))
            }
        };

        let rt = get_simple_runtime();

        match rt.block_on(async {
            // For now, we'll handle S3 credentials via environment variables or AWS config
            // This is a simplified approach until LanceDB's API structure is clearer

            // Apply AWS credentials if provided
            if let Some(s3_config) = storage_options.get("s3_config") {
                apply_s3_environment_variables(s3_config);
            }

            // Create connection with URI (storage options applied via environment)
            connect(&uri_str).execute().await
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

/// Apply AWS S3 configuration via environment variables
/// This is a simplified approach that works with most AWS SDK integrations
fn apply_s3_environment_variables(s3_config: &serde_json::Value) {
    use std::env;

    // Set AWS credentials via environment variables if provided
    if let Some(access_key) = s3_config.get("access_key_id").and_then(|v| v.as_str()) {
        env::set_var("AWS_ACCESS_KEY_ID", access_key);
    }

    if let Some(secret_key) = s3_config.get("secret_access_key").and_then(|v| v.as_str()) {
        env::set_var("AWS_SECRET_ACCESS_KEY", secret_key);
    }

    if let Some(session_token) = s3_config.get("session_token").and_then(|v| v.as_str()) {
        env::set_var("AWS_SESSION_TOKEN", session_token);
    }

    if let Some(region) = s3_config.get("region").and_then(|v| v.as_str()) {
        env::set_var("AWS_REGION", region);
        env::set_var("AWS_DEFAULT_REGION", region);
    }

    if let Some(profile) = s3_config.get("profile").and_then(|v| v.as_str()) {
        env::set_var("AWS_PROFILE", profile);
    }

    // Note: Other S3 options like custom endpoints, path style, etc. would need
    // to be supported by LanceDB's connection builder API directly.
    // For now, this provides basic AWS credential management.
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
