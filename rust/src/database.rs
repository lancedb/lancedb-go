// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: Copyright The LanceDB Authors

//! Database-level operations

use crate::ffi::SimpleResult;
use crate::runtime::get_simple_runtime;
use std::ffi::CString;
use std::os::raw::{c_char, c_int, c_void};
use std::ptr;

/// Get table names
#[no_mangle]
#[allow(clippy::not_unsafe_ptr_arg_deref)]
pub extern "C" fn simple_lancedb_table_names(
    handle: *mut c_void,
    names: *mut *mut *mut c_char,
    count: *mut c_int,
) -> *mut SimpleResult {
    let result = std::panic::catch_unwind(|| -> SimpleResult {
        if handle.is_null() || names.is_null() || count.is_null() {
            return SimpleResult::error("Invalid null arguments".to_string());
        }

        let conn = unsafe { &*(handle as *const lancedb::Connection) };
        let rt = get_simple_runtime();

        match rt.block_on(async { conn.table_names().execute().await }) {
            Ok(table_names) => {
                let len = table_names.len();
                unsafe {
                    *count = len as c_int;

                    if len == 0 {
                        *names = ptr::null_mut();
                        return SimpleResult::ok();
                    }

                    // Allocate array of string pointers
                    let array =
                        libc::malloc(len * std::mem::size_of::<*mut c_char>()) as *mut *mut c_char;
                    if array.is_null() {
                        return SimpleResult::error("Failed to allocate memory".to_string());
                    }

                    // Convert each string and store pointer
                    for (i, name) in table_names.iter().enumerate() {
                        let c_name = CString::new(name.clone()).unwrap().into_raw();
                        *array.add(i) = c_name;
                    }

                    *names = array;
                }
                SimpleResult::ok()
            }
            Err(e) => SimpleResult::error(e.to_string()),
        }
    });

    match result {
        Ok(res) => Box::into_raw(Box::new(res)),
        Err(_) => Box::into_raw(Box::new(SimpleResult::error(
            "Panic in simple_lancedb_table_names".to_string(),
        ))),
    }
}

/// Free table names array
#[no_mangle]
#[allow(clippy::not_unsafe_ptr_arg_deref)]
pub extern "C" fn simple_lancedb_free_table_names(names: *mut *mut c_char, count: c_int) {
    if names.is_null() {
        return;
    }

    unsafe {
        for i in 0..count {
            let name_ptr = *names.add(i as usize);
            if !name_ptr.is_null() {
                let _ = CString::from_raw(name_ptr);
            }
        }
        libc::free(names as *mut c_void);
    }
}
