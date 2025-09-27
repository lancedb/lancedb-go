// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: Copyright The LanceDB Authors

//! Runtime management for async operations

use std::sync::{Arc, OnceLock};
use tokio::runtime::Runtime;

/// Global runtime for async operations  
static SIMPLE_RUNTIME: OnceLock<Arc<Runtime>> = OnceLock::new();

pub fn get_simple_runtime() -> Arc<Runtime> {
    SIMPLE_RUNTIME
        .get_or_init(|| {
            let rt = Runtime::new().expect("Failed to create tokio runtime");
            Arc::new(rt)
        })
        .clone()
}
