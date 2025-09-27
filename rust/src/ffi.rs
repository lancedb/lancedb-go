// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: Copyright The LanceDB Authors

//! Core FFI infrastructure and result types

use std::ffi::{CStr, CString};
use std::os::raw::{c_char, c_int};
use std::ptr;

/// Result type for C interface
#[repr(C)]
pub struct SimpleResult {
    pub success: bool,
    pub error_message: *mut c_char,
}

impl SimpleResult {
    pub fn ok() -> Self {
        Self {
            success: true,
            error_message: ptr::null_mut(),
        }
    }

    pub fn error(msg: String) -> Self {
        let c_msg =
            CString::new(msg).unwrap_or_else(|_| CString::new("Invalid error message").unwrap());
        Self {
            success: false,
            error_message: c_msg.into_raw(),
        }
    }
}

/// Convert C string to Rust string
#[allow(clippy::not_unsafe_ptr_arg_deref)]
pub fn from_c_str(s: *const c_char) -> Result<String, Box<dyn std::error::Error>> {
    if s.is_null() {
        return Err("Null pointer".into());
    }
    let c_str = unsafe { CStr::from_ptr(s) };
    Ok(c_str.to_str()?.to_string())
}

/// Initialize the simple LanceDB library
#[no_mangle]
pub extern "C" fn simple_lancedb_init() -> c_int {
    env_logger::try_init().ok();
    log::info!("Simple LanceDB Go bindings initialized");
    0
}

/// Free a SimpleResult
#[no_mangle]
#[allow(clippy::not_unsafe_ptr_arg_deref)]
pub extern "C" fn simple_lancedb_result_free(result: *mut SimpleResult) {
    if result.is_null() {
        return;
    }
    unsafe {
        let result = Box::from_raw(result);
        if !result.error_message.is_null() {
            let _ = CString::from_raw(result.error_message);
        }
    }
}

/// Free a C string allocated by the library
#[no_mangle]
#[allow(clippy::not_unsafe_ptr_arg_deref)]
pub extern "C" fn simple_lancedb_free_string(s: *mut c_char) {
    if s.is_null() {
        return;
    }
    unsafe {
        let _ = CString::from_raw(s);
    }
}
