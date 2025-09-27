// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: Copyright The LanceDB Authors

//! Schema operations and utilities

use arrow_schema::{DataType, Field, Schema};
use std::sync::Arc;

/// Helper function to create Arrow schema from JSON
pub fn create_arrow_schema_from_json(
    schema_json: &serde_json::Value,
) -> Result<arrow_schema::Schema, Box<dyn std::error::Error>> {
    let fields_array = schema_json
        .get("fields")
        .and_then(|f| f.as_array())
        .ok_or("Schema JSON must have 'fields' array")?;

    let mut fields = Vec::new();

    for field_json in fields_array {
        let name = field_json
            .get("name")
            .and_then(|n| n.as_str())
            .ok_or("Field must have 'name' string")?
            .to_string();

        let type_str = field_json
            .get("type")
            .and_then(|t| t.as_str())
            .ok_or("Field must have 'type' string")?;

        let nullable = field_json
            .get("nullable")
            .and_then(|n| n.as_bool())
            .unwrap_or(true);

        let data_type = match type_str {
            "int8" => DataType::Int8,
            "int16" => DataType::Int16,
            "int32" => DataType::Int32,
            "int64" => DataType::Int64,
            "float16" => DataType::Float16,
            "float32" => DataType::Float32,
            "float64" => DataType::Float64,
            "string" => DataType::Utf8,
            "binary" => DataType::Binary,
            "boolean" => DataType::Boolean,
            _ => {
                // Check for vector type
                if type_str.starts_with("fixed_size_list[int8;") {
                    let dimension_str = type_str
                        .trim_start_matches("fixed_size_list[int8;")
                        .trim_end_matches(']');
                    let dimension: i32 = dimension_str
                        .parse()
                        .map_err(|_| format!("Invalid vector dimension: {}", dimension_str))?;
                    DataType::FixedSizeList(
                        Arc::new(Field::new("item", DataType::Int8, false)),
                        dimension,
                    )
                } else if type_str.starts_with("fixed_size_list[int16;") {
                    let dimension_str = type_str
                        .trim_start_matches("fixed_size_list[int16;")
                        .trim_end_matches(']');
                    let dimension: i32 = dimension_str
                        .parse()
                        .map_err(|_| format!("Invalid vector dimension: {}", dimension_str))?;
                    DataType::FixedSizeList(
                        Arc::new(Field::new("item", DataType::Int16, false)),
                        dimension,
                    )
                } else if type_str.starts_with("fixed_size_list[int32;") {
                    let dimension_str = type_str
                        .trim_start_matches("fixed_size_list[int32;")
                        .trim_end_matches(']');
                    let dimension: i32 = dimension_str
                        .parse()
                        .map_err(|_| format!("Invalid vector dimension: {}", dimension_str))?;
                    DataType::FixedSizeList(
                        Arc::new(Field::new("item", DataType::Int32, false)),
                        dimension,
                    )
                } else if type_str.starts_with("fixed_size_list[int64;") {
                    let dimension_str = type_str
                        .trim_start_matches("fixed_size_list[int64;")
                        .trim_end_matches(']');
                    let dimension: i32 = dimension_str
                        .parse()
                        .map_err(|_| format!("Invalid vector dimension: {}", dimension_str))?;
                    DataType::FixedSizeList(
                        Arc::new(Field::new("item", DataType::Int64, false)),
                        dimension,
                    )
                } else if type_str.starts_with("fixed_size_list[float16;") {
                    let dimension_str = type_str
                        .trim_start_matches("fixed_size_list[float16;")
                        .trim_end_matches(']');
                    let dimension: i32 = dimension_str
                        .parse()
                        .map_err(|_| format!("Invalid vector dimension: {}", dimension_str))?;
                    DataType::FixedSizeList(
                        Arc::new(Field::new("item", DataType::Float16, false)),
                        dimension,
                    )
                } else if type_str.starts_with("fixed_size_list[float32;") {
                    let dimension_str = type_str
                        .trim_start_matches("fixed_size_list[float32;")
                        .trim_end_matches(']');
                    let dimension: i32 = dimension_str
                        .parse()
                        .map_err(|_| format!("Invalid vector dimension: {}", dimension_str))?;
                    DataType::FixedSizeList(
                        Arc::new(Field::new("item", DataType::Float32, false)),
                        dimension,
                    )
                } else if type_str.starts_with("fixed_size_list[float64;") {
                    let dimension_str = type_str
                        .trim_start_matches("fixed_size_list[float64;")
                        .trim_end_matches(']');
                    let dimension: i32 = dimension_str
                        .parse()
                        .map_err(|_| format!("Invalid vector dimension: {}", dimension_str))?;
                    DataType::FixedSizeList(
                        Arc::new(Field::new("item", DataType::Float64, false)),
                        dimension,
                    )
                } else {
                    return Err(format!("Unsupported data type: {}", type_str).into());
                }
            }
        };

        fields.push(Field::new(name, data_type, nullable));
    }

    Ok(Schema::new(fields))
}
