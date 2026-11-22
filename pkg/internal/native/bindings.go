// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: Copyright The LanceDB Authors

package native

import (
	"fmt"

	"github.com/ebitengine/purego"
)

// API holds the bound lancedb-c entry points. Every function is resolved from
// the loaded library once; a missing symbol fails loudly instead of falling
// back to a different ABI.
type API struct {
	Version func() string

	Connect                     func(uri string) uintptr
	ConnectBuilderStorageOption func(builder uintptr, key, value string) uintptr
	ConnectBuilderExecute       func(builder uintptr, connection *uintptr, errorMessage **byte) int32
	ConnectBuilderFree          func(builder uintptr)
	ConnectionFree              func(connection uintptr)
	ConnectionTableNames        func(connection uintptr, names ***byte, count *uintptr, errorMessage **byte) int32
	FreeTableNames              func(names **byte, count uintptr)
	ConnectionOpenTable         func(connection uintptr, tableName string, table *uintptr, errorMessage **byte) int32
	ConnectionDropTable         func(connection uintptr, tableName string, namespace *byte, errorMessage **byte) int32

	TableCreate                func(connection uintptr, tableName string, schema uintptr, reader uintptr, table *uintptr, errorMessage **byte) int32
	TableFree                  func(table uintptr)
	TableAdd                   func(table uintptr, reader uintptr, errorMessage **byte) int32
	TableSchemaIPCFile         func(table uintptr, buffer *IpcBuffer, errorMessage **byte) int32
	TableCountRowsWithStatus   func(table uintptr, count *uintptr, errorMessage **byte) int32
	TableVersionWithStatus     func(table uintptr, version *uint64, errorMessage **byte) int32
	TableDelete                func(table uintptr, predicate string, errorMessage **byte) int32
	TableUpdate                func(table uintptr, columns **byte, expressions **byte, assignments uintptr, predicate *byte, result *UpdateResult, errorMessage **byte) int32
	TableMergeInsertWithResult func(table uintptr, reader uintptr, onColumns **byte, onCount uintptr, options *MergeInsertOptions, result *MergeInsertResult, errorMessage **byte) int32
	TableCreateIndex           func(table uintptr, columns **byte, count uintptr, indexType int32, options *IndexOptions, errorMessage **byte) int32
	TableListIndicesInfo       func(table uintptr, indices **IndexInfo, count *uintptr, errorMessage **byte) int32
	FreeIndexInfo              func(indices *IndexInfo, count uintptr)
	TableIndexStatistics       func(table uintptr, indexName string, statistics **IndexStatistics, errorMessage **byte) int32
	FreeIndexStatistics        func(statistics *IndexStatistics)
	TableDropIndex             func(table uintptr, indexName string, errorMessage **byte) int32
	TablePrewarmIndex          func(table uintptr, indexName string, errorMessage **byte) int32
	TableWaitForIndex          func(table uintptr, indexNames **byte, count uintptr, timeoutMS uint64, errorMessage **byte) int32
	TableOptimizeWithAction    func(table uintptr, config *OptimizeConfig, stats *OptimizeStats, errorMessage **byte) int32
	TableListVersions          func(table uintptr, versions **Version, metadata **VersionMetadata, count *uintptr, errorMessage **byte) int32
	FreeVersions               func(versions *Version, count uintptr)
	FreeVersionMetadata        func(metadata *VersionMetadata, count uintptr)
	TableCheckout              func(table uintptr, version uint64, errorMessage **byte) int32
	TableCheckoutTag           func(table uintptr, tag string, errorMessage **byte) int32
	TableCheckoutLatest        func(table uintptr, errorMessage **byte) int32
	TableRestore               func(table uintptr, errorMessage **byte) int32
	TableListTags              func(table uintptr, tags **Tag, count *uintptr, errorMessage **byte) int32
	FreeTags                   func(tags *Tag, count uintptr)
	TableGetTagVersion         func(table uintptr, tag string, version *uint64, errorMessage **byte) int32
	TableCreateTag             func(table uintptr, tag string, version uint64, errorMessage **byte) int32
	TableDeleteTag             func(table uintptr, tag string, errorMessage **byte) int32
	TableUpdateTag             func(table uintptr, tag string, version uint64, errorMessage **byte) int32
	TableAddColumns            func(table uintptr, names **byte, expressions **byte, count uintptr, version *uint64, errorMessage **byte) int32
	TableAlterColumns          func(table uintptr, alterations *ColumnAlteration, count uintptr, version *uint64, errorMessage **byte) int32
	TableDropColumns           func(table uintptr, names **byte, count uintptr, version *uint64, errorMessage **byte) int32

	QueryNew               func(table uintptr) uintptr
	QueryLimit             func(query uintptr, limit uintptr, errorMessage **byte) int32
	QueryOffset            func(query uintptr, offset uintptr, errorMessage **byte) int32
	QuerySelect            func(query uintptr, columns **byte, count uintptr, errorMessage **byte) int32
	QueryWithRowID         func(query uintptr, errorMessage **byte) int32
	QueryFastSearch        func(query uintptr, errorMessage **byte) int32
	QueryWhereFilter       func(query uintptr, filter string, errorMessage **byte) int32
	QueryFullTextSearch    func(query uintptr, text string, column *byte, errorMessage **byte) int32
	QueryExecuteWithStatus func(query uintptr, result *uintptr, errorMessage **byte) int32
	QueryResultToIPCFile   func(result uintptr, buffer *IpcBuffer, errorMessage **byte) int32
	QueryFree              func(query uintptr)
	QueryResultFree        func(result uintptr)

	VectorQueryNew               func(table uintptr, vector *float32, dimension uintptr) uintptr
	VectorQueryLimit             func(query uintptr, limit uintptr, errorMessage **byte) int32
	VectorQueryOffset            func(query uintptr, offset uintptr, errorMessage **byte) int32
	VectorQueryColumn            func(query uintptr, column string, errorMessage **byte) int32
	VectorQuerySelect            func(query uintptr, columns **byte, count uintptr, errorMessage **byte) int32
	VectorQueryWithRowID         func(query uintptr, errorMessage **byte) int32
	VectorQueryFastSearch        func(query uintptr, errorMessage **byte) int32
	VectorQueryBypassVectorIndex func(query uintptr, errorMessage **byte) int32
	VectorQueryWhereFilter       func(query uintptr, filter string, errorMessage **byte) int32
	VectorQueryFullTextSearch    func(query uintptr, text string, column *byte, errorMessage **byte) int32
	VectorQueryPostfilter        func(query uintptr, errorMessage **byte) int32
	VectorQueryRerankRRF         func(query uintptr, k float32, errorMessage **byte) int32
	VectorQueryNorm              func(query uintptr, normalizeMethod int32, errorMessage **byte) int32
	VectorQueryDistanceType      func(query uintptr, distanceType int32, errorMessage **byte) int32
	VectorQueryNprobes           func(query uintptr, nprobes uintptr, errorMessage **byte) int32
	VectorQueryRefineFactor      func(query uintptr, refineFactor uint32, errorMessage **byte) int32
	VectorQueryEf                func(query uintptr, ef uintptr, errorMessage **byte) int32
	VectorQueryExecuteWithStatus func(query uintptr, result *uintptr, errorMessage **byte) int32
	VectorQueryFree              func(query uintptr)

	RecordBatchReaderFromIPCFile func(data *byte, length uintptr, reader *uintptr, schema *uintptr, errorMessage **byte) int32
	RecordBatchReaderFree        func(reader uintptr)
	FreeArrowSchema              func(schema uintptr)
	IpcBufferFree                func(buffer *IpcBuffer)
	FreeString                   func(value *byte)
}

func bindAPI(lib *Library) (*API, error) {
	api := &API{}
	if err := bind(lib, "version", &api.Version); err != nil {
		return nil, err
	}
	bindings := []error{
		bind(lib, "connect", &api.Connect),
		bind(lib, "connect_builder_storage_option", &api.ConnectBuilderStorageOption),
		bind(lib, "connect_builder_execute", &api.ConnectBuilderExecute),
		bind(lib, "connect_builder_free", &api.ConnectBuilderFree),
		bind(lib, "connection_free", &api.ConnectionFree),
		bind(lib, "connection_table_names", &api.ConnectionTableNames),
		bind(lib, "free_table_names", &api.FreeTableNames),
		bind(lib, "connection_open_table", &api.ConnectionOpenTable),
		bind(lib, "connection_drop_table", &api.ConnectionDropTable),
		bind(lib, "table_create", &api.TableCreate),
		bind(lib, "table_free", &api.TableFree),
		bind(lib, "table_add", &api.TableAdd),
		bind(lib, "table_schema_ipc_file", &api.TableSchemaIPCFile),
		bind(lib, "table_count_rows_with_status", &api.TableCountRowsWithStatus),
		bind(lib, "table_version_with_status", &api.TableVersionWithStatus),
		bind(lib, "table_delete", &api.TableDelete),
		bind(lib, "table_update", &api.TableUpdate),
		bind(lib, "table_merge_insert_with_result", &api.TableMergeInsertWithResult),
		bind(lib, "table_create_index", &api.TableCreateIndex),
		bind(lib, "table_list_indices_info", &api.TableListIndicesInfo),
		bind(lib, "free_index_info", &api.FreeIndexInfo),
		bind(lib, "table_index_statistics", &api.TableIndexStatistics),
		bind(lib, "free_index_statistics", &api.FreeIndexStatistics),
		bind(lib, "table_drop_index", &api.TableDropIndex),
		bind(lib, "table_prewarm_index", &api.TablePrewarmIndex),
		bind(lib, "table_wait_for_index", &api.TableWaitForIndex),
		bind(lib, "table_optimize_with_action", &api.TableOptimizeWithAction),
		bind(lib, "table_list_versions", &api.TableListVersions),
		bind(lib, "free_versions", &api.FreeVersions),
		bind(lib, "free_version_metadata", &api.FreeVersionMetadata),
		bind(lib, "table_checkout", &api.TableCheckout),
		bind(lib, "table_checkout_tag", &api.TableCheckoutTag),
		bind(lib, "table_checkout_latest", &api.TableCheckoutLatest),
		bind(lib, "table_restore", &api.TableRestore),
		bind(lib, "table_list_tags", &api.TableListTags),
		bind(lib, "free_tags", &api.FreeTags),
		bind(lib, "table_get_tag_version", &api.TableGetTagVersion),
		bind(lib, "table_create_tag", &api.TableCreateTag),
		bind(lib, "table_delete_tag", &api.TableDeleteTag),
		bind(lib, "table_update_tag", &api.TableUpdateTag),
		bind(lib, "table_add_columns", &api.TableAddColumns),
		bind(lib, "table_alter_columns", &api.TableAlterColumns),
		bind(lib, "table_drop_columns", &api.TableDropColumns),
		bind(lib, "query_new", &api.QueryNew),
		bind(lib, "query_limit", &api.QueryLimit),
		bind(lib, "query_offset", &api.QueryOffset),
		bind(lib, "query_select", &api.QuerySelect),
		bind(lib, "query_with_row_id", &api.QueryWithRowID),
		bind(lib, "query_fast_search", &api.QueryFastSearch),
		bind(lib, "query_where_filter", &api.QueryWhereFilter),
		bind(lib, "query_full_text_search", &api.QueryFullTextSearch),
		bind(lib, "query_execute_with_status", &api.QueryExecuteWithStatus),
		bind(lib, "query_result_to_ipc_file", &api.QueryResultToIPCFile),
		bind(lib, "query_free", &api.QueryFree),
		bind(lib, "query_result_free", &api.QueryResultFree),
		bind(lib, "vector_query_new", &api.VectorQueryNew),
		bind(lib, "vector_query_limit", &api.VectorQueryLimit),
		bind(lib, "vector_query_offset", &api.VectorQueryOffset),
		bind(lib, "vector_query_column", &api.VectorQueryColumn),
		bind(lib, "vector_query_select", &api.VectorQuerySelect),
		bind(lib, "vector_query_with_row_id", &api.VectorQueryWithRowID),
		bind(lib, "vector_query_fast_search", &api.VectorQueryFastSearch),
		bind(lib, "vector_query_bypass_vector_index", &api.VectorQueryBypassVectorIndex),
		bind(lib, "vector_query_where_filter", &api.VectorQueryWhereFilter),
		bind(lib, "vector_query_full_text_search", &api.VectorQueryFullTextSearch),
		bind(lib, "vector_query_postfilter", &api.VectorQueryPostfilter),
		bind(lib, "vector_query_rerank_rrf", &api.VectorQueryRerankRRF),
		bind(lib, "vector_query_norm", &api.VectorQueryNorm),
		bind(lib, "vector_query_distance_type", &api.VectorQueryDistanceType),
		bind(lib, "vector_query_nprobes", &api.VectorQueryNprobes),
		bind(lib, "vector_query_refine_factor", &api.VectorQueryRefineFactor),
		bind(lib, "vector_query_ef", &api.VectorQueryEf),
		bind(lib, "vector_query_execute_with_status", &api.VectorQueryExecuteWithStatus),
		bind(lib, "vector_query_free", &api.VectorQueryFree),
		bind(lib, "record_batch_reader_from_ipc_file", &api.RecordBatchReaderFromIPCFile),
		bind(lib, "record_batch_reader_free", &api.RecordBatchReaderFree),
		bind(lib, "free_arrow_schema", &api.FreeArrowSchema),
		bind(lib, "ipc_buffer_free", &api.IpcBufferFree),
		bind(lib, "free_string", &api.FreeString),
	}
	for _, err := range bindings {
		if err != nil {
			return nil, err
		}
	}
	return api, nil
}

func bind[T any](lib *Library, name string, target *T) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("resolve lancedb_%s in %s: %v", name, lib.Path, recovered)
		}
	}()
	purego.RegisterLibFunc(target, lib.Handle, "lancedb_"+name)
	return nil
}
