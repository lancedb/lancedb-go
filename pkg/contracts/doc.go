// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: Copyright The LanceDB Authors

/*
Package contracts defines the public interfaces and configuration types used by
package lancedb.

# Compatibility Boundary

The published API baseline is v0.1.2. IConnection, ITable, IQueryBuilder,
IVectorQueryBuilder, ISchema, ISchemaBuilder and the exported configuration
types keep their published method sets, field layouts, JSON tags, numeric
values and Apache Arrow Go v17 type identity. External implementations and
mocks of the published interfaces continue to compile; adding a method to one
of those interfaces would break them, so new capabilities are exposed through
new interfaces instead.

IQueryBuilder.Execute keeps its ([]arrow.Record, error) signature together with
the slice-based ExecuteAsync. Go cannot overload Execute by return type or add a
context parameter without breaking that published signature.

# Companion Interfaces

Additive capabilities are exposed as companion interfaces implemented by the
same connection or table, but reached through a separate method name and
obtained with a type assertion:

	qv2, ok := table.(contracts.ITableQueryV2)
	if !ok {
		return fmt.Errorf("table does not implement ITableQueryV2")
	}
	record, err := qv2.QueryV2().
		Filter("score > 0.8").
		Columns([]string{"id", "text"}).
		Limit(10).
		Execute(ctx)

The base ITable.Query method keeps returning IQueryBuilder. QueryV2 returns
IQueryBuilderV2, whose Execute takes a context and returns one arrow.Record;
its ExecuteAsync returns channels of arrow.Record. ITableVectorQuery and
IVectorQueryBuilderV2 follow the same pattern for vector and hybrid search.

The other companions are:

  - ITableIndexManagement: parameterized index creation, index statistics,
    WaitForIndex and DropIndex.
  - ITableMergeInsert: merge_insert with matched, not-matched and
    not-matched-by-source actions.
  - ITableOptimize: optimize actions and their statistics.
  - ITableUpdateExpr: raw SQL SET expressions with affected rows and version.
  - ITableTimeTravel: version history, checkouts and tag CRUD.
  - ITableSchemaEvolve: add, alter and drop columns.
  - ITablePrewarmIndex: index prewarming.

# Additive Types

Types introduced after v0.1.2 use new names or a V2 suffix and do not change
the layout or JSON tags of the published types. QueryConfigV2, VectorSearchV2,
IndexParams, CreateIndexOptions, IndexStatistics, RerankerConfig,
OptimizeAction, OptimizeStats, MergeResult, UpdateAssignment, UpdateResult,
VersionInfo, TagInfo, NewColumnTransform and ColumnAlteration are additive;
this list is not exhaustive. Use QueryConfig and VectorSearch with the published
ITable.Select path, and QueryConfigV2 and VectorSearchV2 with ITableQueryV2.

Storage option constants use the native lancedb-c keys. The published
ConnectionOptions and structured StorageOptions types keep their v0.1.2 shape;
lancedb.ConnectWithStorageOptions is the additive path for arbitrary flat
options, including aws_profile, aws_region, dynamodb_endpoint and
aws_s3_addressing_style.

# Documentation

  - Go package reference: https://pkg.go.dev/github.com/lancedb/lancedb-go/pkg/contracts
  - SDK package reference: https://pkg.go.dev/github.com/lancedb/lancedb-go/pkg/lancedb
  - Migration guide: https://github.com/lancedb/lancedb-go/blob/main/MIGRATION.md
  - v0.1.2 release: https://github.com/lancedb/lancedb-go/releases/tag/v0.1.2
  - LanceDB documentation: https://lancedb.github.io/lancedb/
*/
package contracts
