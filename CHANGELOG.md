# Changelog

## [Unreleased]

### Added
- `ITable.VectorQuery(column string, vector []float32) IVectorQueryBuilder` — fluent builder for vector similarity searches, complementing the lower-level `VectorSearch` method.
- Input validation in `VectorQueryBuilder.Execute()`: returns clear errors for nil/empty vector, empty column name, and missing `Limit`.

### Removed
- **BREAKING**: `IVectorQueryBuilder.DistanceType(_ DistanceType) IVectorQueryBuilder` removed from the interface. The method was previously exported but documented as a no-op pending Rust FFI support.
- **BREAKING**: `DistanceType` type and constants (`DistanceTypeL2`, `DistanceTypeCosine`, `DistanceTypeDot`, `DistanceTypeHamming`) removed from `pkg/contracts`. Implementors of `IVectorQueryBuilder` and callers of `.DistanceType(...)` must remove those call sites.
- **BREAKING**: `IVectorQueryBuilder.Offset(offset int) IVectorQueryBuilder` removed from the interface. ANN vector search returns the K nearest neighbours and cannot be paginated with a row offset; exposing the method while always rejecting it at runtime was a public API lie. The underlying `VectorQueryBuilder` struct still validates and rejects non-zero offsets with a clear error.

### Fixed
- `ExecuteAsync` on both `QueryBuilder` and `VectorQueryBuilder` now always closes both returned channels after exactly one receives a value, satisfying Go's channel-close convention. Callers using `select` should use the two-value receive form (`value, ok := <-ch`) to distinguish a real value (`ok=true`) from a closed-empty channel (`ok=false`); on the closed-empty branch the other channel holds the actual result or error.
