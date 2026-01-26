// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: Copyright The LanceDB Authors

//! Data type conversion utilities

use arrow_array::{
    ArrayRef, BooleanArray, FixedSizeListArray, Float32Array, Float64Array, Int32Array, Int64Array,
    StringArray,
};
use arrow_schema::DataType;
use std::sync::Arc;

/// Convert JSON values to Arrow RecordBatch
pub fn json_to_record_batch(
    json_values: &[serde_json::Value],
    schema: &arrow_schema::Schema,
) -> Result<arrow_array::RecordBatch, String> {
    let mut columns: Vec<ArrayRef> = Vec::new();

    for field in schema.fields() {
        let field_name = field.name();
        let data_type = field.data_type();

        match data_type {
            DataType::Int32 => {
                let values: Result<Vec<Option<i32>>, String> = json_values
                    .iter()
                    .map(|obj| match obj.get(field_name) {
                        Some(serde_json::Value::Number(n)) => {
                            if let Some(i) = n.as_i64() {
                                if i >= i32::MIN as i64 && i <= i32::MAX as i64 {
                                    Ok(Some(i as i32))
                                } else {
                                    Err(format!(
                                        "Number {} out of range for i32 in field {}",
                                        i, field_name
                                    ))
                                }
                            } else {
                                Err(format!("Invalid number format in field {}", field_name))
                            }
                        }
                        Some(serde_json::Value::Null) if field.is_nullable() => Ok(None),
                        None if field.is_nullable() => Ok(None),
                        Some(_) => Err(format!(
                            "Expected number for field {} but got different type",
                            field_name
                        )),
                        None => Err(format!("Missing required field {}", field_name)),
                    })
                    .collect();

                let array = Int32Array::from(values?);
                columns.push(Arc::new(array) as ArrayRef);
            }
            DataType::Int64 => {
                let values: Result<Vec<Option<i64>>, String> = json_values
                    .iter()
                    .map(|obj| match obj.get(field_name) {
                        Some(serde_json::Value::Number(n)) => {
                            if let Some(i) = n.as_i64() {
                                Ok(Some(i))
                            } else {
                                Err(format!("Invalid number format in field {}", field_name))
                            }
                        }
                        Some(serde_json::Value::Null) if field.is_nullable() => Ok(None),
                        None if field.is_nullable() => Ok(None),
                        Some(_) => Err(format!(
                            "Expected number for field {} but got different type",
                            field_name
                        )),
                        None => Err(format!("Missing required field {}", field_name)),
                    })
                    .collect();

                let array = Int64Array::from(values?);
                columns.push(Arc::new(array) as ArrayRef);
            }
            DataType::Float32 => {
                let values: Result<Vec<Option<f32>>, String> = json_values
                    .iter()
                    .map(|obj| match obj.get(field_name) {
                        Some(serde_json::Value::Number(n)) => {
                            if let Some(f) = n.as_f64() {
                                Ok(Some(f as f32))
                            } else {
                                Err(format!("Invalid number format in field {}", field_name))
                            }
                        }
                        Some(serde_json::Value::Null) if field.is_nullable() => Ok(None),
                        None if field.is_nullable() => Ok(None),
                        Some(_) => Err(format!(
                            "Expected number for field {} but got different type",
                            field_name
                        )),
                        None => Err(format!("Missing required field {}", field_name)),
                    })
                    .collect();

                let array = Float32Array::from(values?);
                columns.push(Arc::new(array) as ArrayRef);
            }
            DataType::Float64 => {
                let values: Result<Vec<Option<f64>>, String> = json_values
                    .iter()
                    .map(|obj| match obj.get(field_name) {
                        Some(serde_json::Value::Number(n)) => {
                            if let Some(f) = n.as_f64() {
                                Ok(Some(f))
                            } else {
                                Err(format!("Invalid number format in field {}", field_name))
                            }
                        }
                        Some(serde_json::Value::Null) if field.is_nullable() => Ok(None),
                        None if field.is_nullable() => Ok(None),
                        Some(_) => Err(format!(
                            "Expected number for field {} but got different type",
                            field_name
                        )),
                        None => Err(format!("Missing required field {}", field_name)),
                    })
                    .collect();

                let array = Float64Array::from(values?);
                columns.push(Arc::new(array) as ArrayRef);
            }
            DataType::Boolean => {
                let values: Result<Vec<Option<bool>>, String> = json_values
                    .iter()
                    .map(|obj| match obj.get(field_name) {
                        Some(serde_json::Value::Bool(b)) => Ok(Some(*b)),
                        Some(serde_json::Value::Null) if field.is_nullable() => Ok(None),
                        None if field.is_nullable() => Ok(None),
                        Some(_) => Err(format!(
                            "Expected boolean for field {} but got different type",
                            field_name
                        )),
                        None => Err(format!("Missing required field {}", field_name)),
                    })
                    .collect();

                let array = BooleanArray::from(values?);
                columns.push(Arc::new(array) as ArrayRef);
            }
            DataType::Utf8 => {
                let values: Result<Vec<Option<String>>, String> = json_values
                    .iter()
                    .map(|obj| match obj.get(field_name) {
                        Some(serde_json::Value::String(s)) => Ok(Some(s.clone())),
                        Some(serde_json::Value::Null) if field.is_nullable() => Ok(None),
                        None if field.is_nullable() => Ok(None),
                        Some(_) => Err(format!(
                            "Expected string for field {} but got different type",
                            field_name
                        )),
                        None => Err(format!("Missing required field {}", field_name)),
                    })
                    .collect();

                let array = StringArray::from(values?);
                columns.push(Arc::new(array) as ArrayRef);
            }
            DataType::FixedSizeList(inner_field, list_size)
                if matches!(inner_field.data_type(), DataType::Float32) =>
            {
                // Handle vector fields (FixedSizeList of Float32)
                let values: Result<Vec<Option<Vec<f32>>>, String> = json_values
                    .iter()
                    .map(|obj| match obj.get(field_name) {
                        Some(serde_json::Value::Array(arr)) => {
                            if arr.len() != *list_size as usize {
                                return Err(format!(
                                    "Vector field {} expects {} elements but got {}",
                                    field_name,
                                    list_size,
                                    arr.len()
                                ));
                            }
                            let vec_values: Result<Vec<f32>, String> = arr
                                .iter()
                                .map(|v| match v.as_f64() {
                                    Some(f) => Ok(f as f32),
                                    None => Err(format!(
                                        "Invalid vector element in field {}",
                                        field_name
                                    )),
                                })
                                .collect();
                            Ok(Some(vec_values?))
                        }
                        Some(serde_json::Value::Null) if field.is_nullable() => Ok(None),
                        None if field.is_nullable() => Ok(None),
                        Some(_) => Err(format!(
                            "Expected array for vector field {} but got different type",
                            field_name
                        )),
                        None => Err(format!("Missing required field {}", field_name)),
                    })
                    .collect();

                let flat_values: Vec<Option<f32>> = values?
                    .into_iter()
                    .flat_map(|opt_vec| match opt_vec {
                        Some(vec) => vec.into_iter().map(Some).collect::<Vec<_>>(),
                        None => (0..*list_size).map(|_| None).collect::<Vec<_>>(),
                    })
                    .collect();

                let float_array = Float32Array::from(flat_values);
                let list_array = FixedSizeListArray::new(
                    inner_field.clone(),
                    *list_size,
                    Arc::new(float_array),
                    None, // No null buffer for now - simplified
                );
                columns.push(Arc::new(list_array) as ArrayRef);
            }
            _ => return Err(format!("Unsupported data type: {:?}", data_type)),
        }
    }

    arrow_array::RecordBatch::try_new(Arc::new(schema.clone()), columns)
        .map_err(|e| format!("Failed to create RecordBatch: {}", e))
}

/// Helper function to convert Arrow array value to JSON
pub fn convert_arrow_value_to_json(
    array: &dyn arrow_array::Array,
    row_idx: usize,
) -> Result<serde_json::Value, String> {
    if array.is_null(row_idx) {
        return Ok(serde_json::Value::Null);
    }

    match array.data_type() {
        DataType::Int32 => {
            let typed_array = array
                .as_any()
                .downcast_ref::<arrow_array::Int32Array>()
                .ok_or("Failed to downcast to Int32Array")?;
            Ok(serde_json::Value::Number(serde_json::Number::from(
                typed_array.value(row_idx),
            )))
        }
        DataType::Int64 => {
            let typed_array = array
                .as_any()
                .downcast_ref::<arrow_array::Int64Array>()
                .ok_or("Failed to downcast to Int64Array")?;
            Ok(serde_json::Value::Number(serde_json::Number::from(
                typed_array.value(row_idx),
            )))
        }
        DataType::Float32 => {
            let typed_array = array
                .as_any()
                .downcast_ref::<arrow_array::Float32Array>()
                .ok_or("Failed to downcast to Float32Array")?;
            Ok(serde_json::json!(typed_array.value(row_idx)))
        }
        DataType::Float64 => {
            let typed_array = array
                .as_any()
                .downcast_ref::<arrow_array::Float64Array>()
                .ok_or("Failed to downcast to Float64Array")?;
            Ok(serde_json::json!(typed_array.value(row_idx)))
        }
        DataType::Boolean => {
            let typed_array = array
                .as_any()
                .downcast_ref::<arrow_array::BooleanArray>()
                .ok_or("Failed to downcast to BooleanArray")?;
            Ok(serde_json::Value::Bool(typed_array.value(row_idx)))
        }
        DataType::Utf8 => {
            let typed_array = array
                .as_any()
                .downcast_ref::<arrow_array::StringArray>()
                .ok_or("Failed to downcast to StringArray")?;
            Ok(serde_json::Value::String(
                typed_array.value(row_idx).to_string(),
            ))
        }
        DataType::FixedSizeList(_, list_size) => {
            let typed_array = array
                .as_any()
                .downcast_ref::<arrow_array::FixedSizeListArray>()
                .ok_or("Failed to downcast to FixedSizeListArray")?;
            let values_array = typed_array.values();

            let start_idx = row_idx * (*list_size as usize);
            let end_idx = start_idx + (*list_size as usize);

            let mut list_values = Vec::new();
            for i in start_idx..end_idx {
                list_values.push(convert_arrow_value_to_json(values_array.as_ref(), i)?);
            }
            Ok(serde_json::Value::Array(list_values))
        }
        DataType::List(_) => {
            let typed_array = array
                .as_any()
                .downcast_ref::<arrow_array::ListArray>()
                .ok_or("Failed to downcast to ListArray")?;
            let values_array = typed_array.values();
            let offsets = typed_array.offsets();

            let mut list_values = Vec::new();
            for i in offsets[row_idx]..offsets[row_idx + 1] {
                list_values.push(convert_arrow_value_to_json(
                    values_array.as_ref(),
                    i as usize,
                )?);
            }
            Ok(serde_json::Value::Array(list_values))
        }
        _ => Ok(serde_json::Value::String(format!(
            "Unsupported type: {:?}",
            array.data_type()
        ))),
    }
}
