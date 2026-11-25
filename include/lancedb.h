/*
 * SPDX-License-Identifier: Apache-2.0
 * SPDX-FileCopyrightText: Copyright The LanceDB Authors
 */

#ifndef LANCEDB_H
#define LANCEDB_H

#include <stddef.h>
#include <stdint.h>
#include <stdint.h>
#include <stdbool.h>

/**
 * @defgroup api_version Binding version
 * @brief Compile-time and runtime version helpers.
 * @{
 */

/*
 * Binding version. This source header reports the development default so it
 * stays usable directly without a generated file. CMake renders these values
 * for the installed header, and the package builder renders the same block for
 * packaged releases. All values match lancedb_version() in the library.
 */
#define LANCEDB_VERSION_MAJOR 0
#define LANCEDB_VERSION_MINOR 1
#define LANCEDB_VERSION_PATCH 0
#define LANCEDB_VERSION_STRING "0.1.0-preview.5"
#define LANCEDB_VERSION LANCEDB_VERSION_STRING

/** @} */

/**
 * @defgroup api_types Handles and shared buffers
 * @brief Opaque handles, shared buffers, and Arrow C ABI structures.
 * @{
 */

#ifdef __cplusplus
extern "C" {
#endif

/*
 * Standard Arrow C Data and Stream interface structures.
 *
 * These definitions match arrow/c/abi.h. The include guards are shared with
 * Arrow, so either this header or Arrow's header may be included first. A C
 * consumer can therefore use the stream entry points without depending on
 * Arrow headers.
 */
#ifndef ARROW_C_DATA_INTERFACE
#define ARROW_C_DATA_INTERFACE

#define ARROW_FLAG_DICTIONARY_ORDERED 1
#define ARROW_FLAG_NULLABLE 2
#define ARROW_FLAG_MAP_KEYS_SORTED 4

struct ArrowSchema {
    const char* format;
    const char* name;
    const char* metadata;
    int64_t flags;
    int64_t n_children;
    struct ArrowSchema** children;
    struct ArrowSchema* dictionary;
    void (*release)(struct ArrowSchema*);
    void* private_data;
};

/** @cond */
typedef struct ArrowSchema ArrowSchema;
/** @endcond */

struct ArrowArray {
    int64_t length;
    int64_t null_count;
    int64_t offset;
    int64_t n_buffers;
    int64_t n_children;
    const void** buffers;
    struct ArrowArray** children;
    struct ArrowArray* dictionary;
    void (*release)(struct ArrowArray*);
    void* private_data;
};

/** @cond */
typedef struct ArrowArray ArrowArray;
/** @endcond */

#define ARROW_STATISTICS_KEY_AVERAGE_BYTE_WIDTH_EXACT "ARROW:average_byte_width:exact"
#define ARROW_STATISTICS_KEY_AVERAGE_BYTE_WIDTH_APPROXIMATE "ARROW:average_byte_width:approximate"
#define ARROW_STATISTICS_KEY_DISTINCT_COUNT_EXACT "ARROW:distinct_count:exact"
#define ARROW_STATISTICS_KEY_DISTINCT_COUNT_APPROXIMATE "ARROW:distinct_count:approximate"
#define ARROW_STATISTICS_KEY_MAX_BYTE_WIDTH_EXACT "ARROW:max_byte_width:exact"
#define ARROW_STATISTICS_KEY_MAX_BYTE_WIDTH_APPROXIMATE "ARROW:max_byte_width:approximate"
#define ARROW_STATISTICS_KEY_MAX_VALUE_EXACT "ARROW:max_value:exact"
#define ARROW_STATISTICS_KEY_MAX_VALUE_APPROXIMATE "ARROW:max_value:approximate"
#define ARROW_STATISTICS_KEY_MIN_VALUE_EXACT "ARROW:min_value:exact"
#define ARROW_STATISTICS_KEY_MIN_VALUE_APPROXIMATE "ARROW:min_value:approximate"
#define ARROW_STATISTICS_KEY_NULL_COUNT_EXACT "ARROW:null_count:exact"
#define ARROW_STATISTICS_KEY_NULL_COUNT_APPROXIMATE "ARROW:null_count:approximate"
#define ARROW_STATISTICS_KEY_ROW_COUNT_EXACT "ARROW:row_count:exact"
#define ARROW_STATISTICS_KEY_ROW_COUNT_APPROXIMATE "ARROW:row_count:approximate"

#endif /* ARROW_C_DATA_INTERFACE */

#ifndef ARROW_C_STREAM_INTERFACE
#define ARROW_C_STREAM_INTERFACE

struct ArrowArrayStream {
    int (*get_schema)(struct ArrowArrayStream*, struct ArrowSchema*);
    int (*get_next)(struct ArrowArrayStream*, struct ArrowArray*);
    const char* (*get_last_error)(struct ArrowArrayStream*);
    void (*release)(struct ArrowArrayStream*);
    void* private_data;
};

/** @cond */
typedef struct ArrowArrayStream ArrowArrayStream;
/** @endcond */

#endif /* ARROW_C_STREAM_INTERFACE */

/**
 * @addtogroup api_version
 * @{
 */

/**
 * Return the binding version embedded in this library.
 *
 * The returned string is static, NUL-terminated and owned by the library. It
 * matches LANCEDB_VERSION_STRING when the header and library come from the same
 * build. The caller must not free the returned string.
 */
const char* lancedb_version(void);

/** @} */

/**
 * Opaque handle to a LanceDB ConnectBuilder
 */
typedef struct LanceDBConnectBuilder LanceDBConnectBuilder;

/**
 * Opaque handle to a LanceDB ConnectNamespaceBuilder
 */
typedef struct LanceDBConnectNamespaceBuilder LanceDBConnectNamespaceBuilder;

/**
 * Opaque handle to a LanceDB Connection
 */
typedef struct LanceDBConnection LanceDBConnection;

/**
 * Opaque handle to a LanceDB Table
 */
typedef struct LanceDBTable LanceDBTable;

/**
 * Opaque handle to a LanceDB TableNamesBuilder
 */
typedef struct LanceDBTableNamesBuilder LanceDBTableNamesBuilder;

/**
 * Opaque handle to a DataFusion Expr (for building filter expressions)
 */
typedef struct LanceDBExpr LanceDBExpr;

/**
 * Opaque handle to a structured full-text query
 *
 * Create with lancedb_fts_query_new_match(), lancedb_fts_query_new_multi_match(),
 * lancedb_fts_query_new_phrase(), lancedb_fts_query_new_boolean() or
 * lancedb_fts_query_new_boost() and free with lancedb_fts_query_free().
 * Builders clone the handle's query when it is set, so one handle can be
 * attached to several builders and freed immediately after the setters return.
 */
typedef struct LanceDBFtsQuery LanceDBFtsQuery;

/**
 * Opaque handle to a LanceDB Query
 */
typedef struct LanceDBQuery LanceDBQuery;

/**
 * Opaque handle to a LanceDB VectorQuery
 */
typedef struct LanceDBVectorQuery LanceDBVectorQuery;

/**
 * Opaque handle to a LanceDB QueryResult
 */
typedef struct LanceDBQueryResult LanceDBQueryResult;

/**
 * Opaque handle to a LanceDB TakeQuery
 */
typedef struct LanceDBTakeQuery LanceDBTakeQuery;

/**
 * Opaque handle to a LanceDB TableDelta
 */
typedef struct LanceDBTableDelta LanceDBTableDelta;

/**
 * Opaque seekable handle to one lazy blob file
 */
typedef struct LanceDBBlobFile LanceDBBlobFile;

/**
 * Opaque handle to a LanceDB Session
 */
typedef struct LanceDBSession LanceDBSession;

/**
 * Opaque handle to a LanceDB ObjectStoreRegistry
 */
typedef struct LanceDBObjectStoreRegistry LanceDBObjectStoreRegistry;
typedef struct LanceDBObjectStoreRegistry LanceDBRegistry;

/**
 * Opaque handle to a LanceDB ObjectStoreProvider
 */
typedef struct LanceDBObjectStoreProvider LanceDBObjectStoreProvider;

/**
 * Opaque handle to Arrow RecordBatchReader
 */
typedef struct LanceDBRecordBatchReader LanceDBRecordBatchReader;

/**
 * Arrow C ABI Schema structure (opaque)
 */
typedef struct FFI_ArrowSchema FFI_ArrowSchema;

/**
 * Arrow C ABI Array structure (opaque)
 */
typedef struct FFI_ArrowArray FFI_ArrowArray;

/**
 * Bytes of an Arrow IPC file owned by LanceDB
 *
 * A buffer that holds no file has both fields zeroed. `data` stays valid until
 * lancedb_ipc_buffer_free() is called. The caller must not write to or free it,
 * and keeps both fields unchanged until then.
 */
typedef struct {
    const uint8_t* data;  /* Start of the file bytes, NULL when no file is held */
    size_t len;           /* Length of the file bytes */
} LanceDBIpcBuffer;

/** @} */

/**
 * @defgroup api_errors Errors
 * @brief Error codes and messages.
 * @{
 */

/**
 * Error codes for LanceDB C API
 *
 * Existing values never move. Codes for error categories added by a newer SDK
 * are appended before LANCEDB_UNKNOWN, which stays outside the contiguous range.
 */
typedef enum {
    LANCEDB_SUCCESS = 0,
    LANCEDB_INVALID_ARGUMENT = 1,
    LANCEDB_INVALID_TABLE_NAME = 2,
    LANCEDB_INVALID_INPUT = 3,
    LANCEDB_TABLE_NOT_FOUND = 4,
    LANCEDB_DATABASE_NOT_FOUND = 5,
    LANCEDB_DATABASE_ALREADY_EXISTS = 6,
    LANCEDB_INDEX_NOT_FOUND = 7,
    LANCEDB_EMBEDDING_FUNCTION_NOT_FOUND = 8,
    LANCEDB_TABLE_ALREADY_EXISTS = 9,
    LANCEDB_CREATE_DIR = 10,
    LANCEDB_SCHEMA = 11,
    LANCEDB_RUNTIME = 12,
    LANCEDB_TIMEOUT = 13,
    LANCEDB_OBJECT_STORE = 14,
    LANCEDB_LANCE = 15,
    LANCEDB_HTTP = 16,
    LANCEDB_RETRY = 17,
    LANCEDB_ARROW = 18,
    LANCEDB_NOT_SUPPORTED = 19,
    LANCEDB_OTHER = 20,
    LANCEDB_NAMESPACE = 21,
    LANCEDB_PERMISSION_DENIED = 22,
    LANCEDB_NOT_FOUND = 23,
    LANCEDB_TABLE_CORRUPTED = 24,
    LANCEDB_COLUMN_NOT_FOUND = 25,
    LANCEDB_FIELD_NOT_FOUND = 26,
    LANCEDB_COLUMN_ALREADY_EXISTS = 27,
    LANCEDB_NOT_A_COMPUTED_COLUMN = 28,
    LANCEDB_NOT_A_MATERIALIZED_VIEW = 29,
    LANCEDB_INVALID_EXPRESSION = 30,
    LANCEDB_SQL = 31,
    LANCEDB_JOB_FAILED = 32,
    LANCEDB_JOB_CANCELLED = 33,
    LANCEDB_JOB_NOT_FOUND = 34,
    LANCEDB_RESPONSE_INTERRUPTED = 35,
    LANCEDB_EXTERNAL = 36,
    /* keep last, so that new error codes can be added without renumbering */
    LANCEDB_UNKNOWN = 99
} LanceDBError;

/**
 * Error messages for LanceDB C API
 */
static const char* LANCEDB_ERROR_MESSAGES[] = {
    "Success",
    "Invalid argument",
    "Invalid table name",
    "Invalid input",
    "Table not found",
    "Database not found",
    "Database already exists",
    "Index not found",
    "Embedding function not found",
    "Table already exists",
    "Failed to create directory",
    "Schema error",
    "Runtime error",
    "Operation timed out",
    "Object store error",
    "Lance format error",
    "HTTP error",
    "Retryable error",
    "Arrow error",
    "Operation not supported",
    "Other error",
    "Namespace error",
    "Permission denied",
    "Not found",
    "Table corrupted",
    "Column not found",
    "Field not found",
    "Column already exists",
    "Not a computed column",
    "Not a materialized view",
    "Invalid expression",
    "SQL error",
    "Job failed",
    "Job cancelled",
    "Job not found",
    "Response interrupted",
    "External error"
};

/**
 * Error message for LANCEDB_UNKNOWN, which is not part of LANCEDB_ERROR_MESSAGES
 * since it is deliberately kept out of the range of the other error codes
 */
static const char* LANCEDB_UNKNOWN_ERROR_MESSAGE = "Unknown error";

/**
 * Convert error code to error message
 *
 * @param error - error code
 * @return Pointer to null-terminated C string containing the error message
 *
 * The returned string is valid for the lifetime of the program.
 * The caller must not free the returned string.
 */
static inline const char* lancedb_error_to_message(LanceDBError error) {
    if (error == LANCEDB_UNKNOWN) {
        return LANCEDB_UNKNOWN_ERROR_MESSAGE;
    }
    if (error >= sizeof(LANCEDB_ERROR_MESSAGES)/sizeof(LANCEDB_ERROR_MESSAGES[0])) {
        return "Invalid error code";
    }
    return LANCEDB_ERROR_MESSAGES[error];
}

/** @} */

/**
 * @defgroup api_config Enumerations and configuration
 * @brief Enumerations, option structs, and value types used by the API.
 * @{
 */

/**
 * Distance type enum for vector search
 */
typedef enum {
    LANCEDB_DISTANCE_L2 = 0,
    LANCEDB_DISTANCE_COSINE = 1,
    LANCEDB_DISTANCE_DOT = 2,
    LANCEDB_DISTANCE_HAMMING = 3
} LanceDBDistanceType;

/**
 * Score normalization method for hybrid-search reranking
 *
 * LANCEDB_NORMALIZE_SCORE normalizes the raw vector distance and full-text
 * score columns. LANCEDB_NORMALIZE_RANK converts them to ranks first. The SDK
 * always normalizes the two channels before invoking the reranker; this enum
 * selects what that normalization starts from.
 */
typedef enum {
    LANCEDB_NORMALIZE_SCORE = 0,
    LANCEDB_NORMALIZE_RANK = 1
} LanceDBNormalizeMethod;

/**
 * Element type of one typed query vector
 *
 * LANCEDB_QUERY_VECTOR_F16 points at uint16_t IEEE-754 binary16 bit patterns.
 * LANCEDB_QUERY_VECTOR_F32 points at float, LANCEDB_QUERY_VECTOR_F64 at double,
 * and LANCEDB_QUERY_VECTOR_BINARY at uint8_t. The binary form widens each byte
 * to its numeric Float32 value before it reaches the SDK; the bytes are not
 * reinterpreted as floats.
 */
typedef enum {
    LANCEDB_QUERY_VECTOR_F16 = 0,
    LANCEDB_QUERY_VECTOR_F32 = 1,
    LANCEDB_QUERY_VECTOR_F64 = 2,
    LANCEDB_QUERY_VECTOR_BINARY = 3
} LanceDBQueryVectorType;

/**
 * One typed query vector for lancedb_vector_query_new_typed() and
 * lancedb_vector_query_add_query_vector()
 *
 * This is a value description, not a builder handle. `data` is borrowed for
 * the duration of the call and copied before the function returns. `dimension`
 * is the number of elements in `data`; for LANCEDB_QUERY_VECTOR_BINARY it is
 * the number of bytes. It must be greater than zero.
 */
typedef struct {
    LanceDBQueryVectorType kind;
    const void* data;
    size_t dimension;
} LanceDBQueryVector;

/**
 * Index type enum
 */
typedef enum {
    LANCEDB_INDEX_AUTO = 0,
    LANCEDB_INDEX_BTREE = 1,
    LANCEDB_INDEX_BITMAP = 2,
    LANCEDB_INDEX_LABELLIST = 3,
    LANCEDB_INDEX_FTS = 4,
    LANCEDB_INDEX_IVF_FLAT = 5,
    LANCEDB_INDEX_IVF_PQ = 6,
    LANCEDB_INDEX_IVF_HNSW_PQ = 7,
    LANCEDB_INDEX_IVF_HNSW_SQ = 8
} LanceDBIndexType;

/**
 * Optimize type enum
 */
typedef enum {
    LANCEDB_OPTIMIZE_ALL = 0,     // Compact files and prune old versions
    LANCEDB_OPTIMIZE_COMPACT = 1, // Only compact files
    LANCEDB_OPTIMIZE_PRUNE = 2,   // Only prune old versions
    LANCEDB_OPTIMIZE_INDEX = 3    // Only rebuild indices
} LanceDBOptimizeType;

/**
 * Binary operator enum for DataFusion expressions
 */
typedef enum {
    LANCEDB_BINARY_OP_EQ = 0,
    LANCEDB_BINARY_OP_NOT_EQ = 1,
    LANCEDB_BINARY_OP_LT = 2,
    LANCEDB_BINARY_OP_LT_EQ = 3,
    LANCEDB_BINARY_OP_GT = 4,
    LANCEDB_BINARY_OP_GT_EQ = 5,
    LANCEDB_BINARY_OP_AND = 6,
    LANCEDB_BINARY_OP_OR = 7,
    LANCEDB_BINARY_OP_PLUS = 8,
    LANCEDB_BINARY_OP_MINUS = 9,
    LANCEDB_BINARY_OP_MULTIPLY = 10,
    LANCEDB_BINARY_OP_DIVIDE = 11,
    LANCEDB_BINARY_OP_MODULO = 12
} LanceDBBinaryOp;

/**
 * Vector index configuration
 */
typedef struct {
    int num_partitions;         // Number of partitions for IVF indices (-1 = auto)
    int num_sub_vectors;        // Number of sub-vectors for PQ indices (-1 = auto)
    int max_iterations;         // Maximum training iterations (-1 = default)
    float sample_rate;          // Sampling rate for training (0.0 = default)
    LanceDBDistanceType distance_type; // Distance metric
    int replace;               // Replace existing index (1 = true, 0 = false)
} LanceDBVectorIndexConfig;

/**
 * Scalar index configuration
 */
typedef struct {
    int replace;                      // Replace existing index (1 = true, 0 = false)
} LanceDBScalarIndexConfig;

/**
 * Full-text search index configuration
 */
typedef struct {
    const char* base_tokenizer;   // Base tokenizer ("simple", "whitespace", etc.)
    const char* language;         // Language for stemming ("en", "es", etc.)
    int max_token_length;        // Maximum length of a token (-1 = no limit)
    int lowercase;               // Convert to lowercase (1 = true, 0 = false)
    int stem;                    // Apply stemming (1 = true, 0 = false)
    int remove_stop_words;       // Remove stop words (1 = true, 0 = false)
    int ascii_folding;           // Apply ASCII folding (1 = true, 0 = false)
    int replace;                 // Replace existing index (1 = true, 0 = false)
} LanceDBFtsIndexConfig;

/**
 * Options for lancedb_table_create_index()
 *
 * Initialize with lancedb_index_options_default() so every optional field stays
 * unset instead of becoming an explicit zero or false. Numeric tuning fields use
 * -1 for unset; 0 and positive values are forwarded to the SDK. Boolean tuning
 * fields use -1 for unset, 0 for false and 1 for true. distance_type uses -1 for
 * unset and otherwise takes a LanceDBDistanceType value.
 *
 * Fields that do not apply to the selected index type are ignored. Strings are
 * borrowed for the duration of the call.
 */
typedef struct {
    const char* name;              // NULL or "" uses the SDK-chosen index name
    int replace;                   // -1 = SDK default (true); 0 = false; 1 = true
    uint64_t wait_timeout_ms;      // 0 = do not wait; positive = SDK polling budget

    // IVF tuning
    int64_t num_partitions;        // -1 = SDK default
    int64_t sample_rate;           // -1 = SDK default
    int64_t max_iterations;        // -1 = SDK default
    int64_t target_partition_size; // -1 = SDK default

    // PQ tuning
    int64_t num_sub_vectors;       // -1 = SDK default
    int64_t num_bits;              // -1 = SDK default

    // HNSW tuning
    int64_t m;                     // -1 = SDK default
    int64_t ef_construction;       // -1 = SDK default

    // Vector distance metric
    int distance_type;             // -1 = SDK default; otherwise LanceDBDistanceType

    // FTS tuning
    const char* fts_base_tokenizer;
    const char* fts_language;
    int64_t fts_max_token_length;  // -1 = SDK default
    int fts_lower_case;            // -1 = SDK default; 0 = false; 1 = true
    int fts_stem;                  // -1 = SDK default; 0 = false; 1 = true
    int fts_remove_stop_words;     // -1 = SDK default; 0 = false; 1 = true
    int fts_ascii_folding;         // -1 = SDK default; 0 = false; 1 = true
    int fts_with_position;         // -1 = SDK default; 0 = false; 1 = true
    int64_t fts_ngram_min_length;  // -1 = SDK default
    int64_t fts_ngram_max_length;  // -1 = SDK default
    int fts_ngram_prefix_only;     // -1 = SDK default; 0 = false; 1 = true
} LanceDBIndexOptions;

/**
 * Merge insert configuration
 *
 * The conditions restricting which matched records are updated are only used when
 * when_matched_update_all is set, and are otherwise ignored. The SQL condition and the
 * DataFusion expression are mutually exclusive: when matched records are updated, setting
 * both of them fails with LANCEDB_INVALID_ARGUMENT.
 * In both flavors, use the prefix "target." to refer to rows in the table, and the prefix
 * "source." to refer to rows in the merged data. For example: "target.version < source.version"
 */
typedef struct {
    int when_matched_update_all;     // Update all columns for matched records (1 = true, 0 = false)
    int when_not_matched_insert_all; // Insert all new records (1 = true, 0 = false)
    const char* when_matched_update_all_condition;   // SQL condition on the updated records (NULL for no condition)
    LanceDBExpr* when_matched_update_all_expr;       // DataFusion condition on the updated records (NULL for no condition)
} LanceDBMergeInsertConfig;

/**
 * Options for the merge insert entry points that expose explicit actions
 * (lancedb_table_merge_insert_with_result and
 * lancedb_table_merge_insert_with_options)
 *
 * Every action is disabled unless its flag is non-zero. A NULL options pointer
 * selects the same default upsert as lancedb_table_merge_insert(): update every
 * matched row and insert every unmatched source row.
 *
 * The matched-update condition is an SQL predicate over the joined rows. Use the
 * prefix "target." for rows in the table and "source." for rows in the merged
 * data, for example "target.version < source.version". The by-source-delete
 * filter is evaluated against unmatched target rows, so it names target columns
 * directly, for example "version < 10". A NULL condition means no condition; a
 * non-NULL string, including an empty one, is passed to the SDK. The condition
 * for an action is only read when that action is enabled, and a condition set for
 * a disabled action is ignored. The strings are borrowed for the call.
 *
 * use_index is -1 for the SDK default, 0 to disable indexed planning for the
 * merge, or 1 to allow it. Embedded: 0 disables indexed planning; 1 and -1 leave
 * the SDK's indexed planning enabled when an index applies. This is not a
 * guarantee that the join uses an index, and by-source deletion can use a full
 * scan. Remote: only 0 sends use_index=false; 1 and -1 omit the parameter, so the
 * server's default applies.
 *
 * timeout_ms is -1 for the SDK default, or a non-negative number of milliseconds,
 * including 0. Embedded: the interval covers the Lance merge execution and its
 * retries. SDK table checks, schema reads and the post-commit freshness update
 * run outside it. Remote: the value is sent as the x-request-timeout-ms header on
 * each request; client retries and backoff run outside that per-request interval.
 *
 * The options struct and its strings are borrowed for the duration of the call
 * and are not consumed.
 */
typedef struct {
    int when_matched_update_all;                     // Update matched rows (non-zero = true)
    const char* when_matched_update_all_condition;   // SQL condition (NULL for no condition)
    int when_not_matched_insert_all;                 // Insert unmatched source rows (non-zero = true)
    int when_not_matched_by_source_delete;           // Delete unmatched target rows (non-zero = true)
    const char* when_not_matched_by_source_delete_filter; // SQL filter on unmatched target rows (NULL for no filter)
    int use_index;                                   // -1 = SDK default, 0 = false, 1 = true
    int64_t timeout_ms;                              // -1 = SDK default, otherwise milliseconds
} LanceDBMergeInsertOptions;

/**
 * Session creation options
 */
typedef struct {
    size_t index_cache_bytes;        // Index cache size in bytes (0 = default)
    size_t metadata_cache_bytes;     // Metadata cache size in bytes (0 = default)
} LanceDBSessionOptions;

/**
 * Session cache statistics
 */
typedef struct {
    uint64_t hits;          // Number of cache hits
    uint64_t misses;        // Number of cache misses
    size_t num_entries;     // Number of entries in cache
    size_t size_bytes;      // Cache size in bytes
} LanceDBSessionCacheStats;

/**
 * Version information for a table
 */
typedef struct {
    uint64_t version;            // Version number
    int64_t timestamp_seconds;   // Seconds since Unix epoch (UTC)
    uint32_t timestamp_nanos;    // Sub-second nanoseconds
} LanceDBVersion;

/**
 * Per-version metadata (key-value pairs)
 */
typedef struct {
    char** keys;                 // Array of metadata key strings
    char** values;               // Array of metadata value strings
    size_t count;                // Number of metadata key-value pairs
} LanceDBVersionMetadata;

/**
 * Tag information for a table
 *
 * The array and its strings are read-only and owned by the library. Callers
 * must not modify or free them individually. The strings remain valid until
 * lancedb_free_tags() is called with the same array and count.
 */
typedef struct {
    const char* name;            // Read-only tag name
    uint64_t version;            // Version referenced by the tag
    uint64_t manifest_size;      // Manifest size in bytes at the referenced version
    const char* branch;          // Read-only optional branch name (NULL when the tag has no branch)
} LanceDBTag;

/**
 * Source kind for lancedb_table_create_branch()
 *
 * The kind selects which fields of LanceDBBranchSource are read. A version is
 * resolved on the table handle's current branch, a tag is global, and a branch
 * may be used at its head or at a pinned version.
 */
typedef enum {
    LANCEDB_BRANCH_SOURCE_VERSION = 0, // Version on the current branch
    LANCEDB_BRANCH_SOURCE_TAG = 1,     // Global tag name
    LANCEDB_BRANCH_SOURCE_BRANCH = 2   // Branch name with an optional version
} LanceDBBranchSourceKind;

/**
 * Source reference for lancedb_table_create_branch()
 *
 * Only the fields selected by kind are read; the caller may leave the other
 * fields uninitialized.
 * - LANCEDB_BRANCH_SOURCE_VERSION: has_version and version are read; has_version
 *   must be nonzero. name is not read.
 * - LANCEDB_BRANCH_SOURCE_TAG: only name is read; it is the tag name.
 * - LANCEDB_BRANCH_SOURCE_BRANCH: name and has_version are read. version is
 *   read only when has_version is nonzero; otherwise the branch head is used.
 *
 * The pointed-to name is only read during the call.
 */
typedef struct {
    LanceDBBranchSourceKind kind; // Source kind; out-of-range values are rejected
    const char* name;             // Tag or branch name; ignored for VERSION
    int has_version;              // 1 to use version, 0 to omit it
    uint64_t version;             // Version number when has_version is 1
} LanceDBBranchSource;

/**
 * Optional version for lancedb_table_checkout_branch()
 *
 * has_version set to 0 checks out the branch head and keeps the handle
 * writable. Any other value pins the handle to version, which makes it a
 * read-only detached view. version is read only when has_version is nonzero;
 * otherwise it may be left uninitialized.
 */
typedef struct {
    int has_version;   // 1 to pin the checkout to version, 0 for the branch head
    uint64_t version;  // Version number when has_version is 1
} LanceDBBranchCheckoutOptions;

/**
 * Branch metadata returned by lancedb_table_list_branches()
 *
 * The array, every string and every metadata array are owned by the result.
 * Callers must not modify or free them individually. The strings remain valid
 * until lancedb_free_branches() is called with the same array and count.
 * parent_version is the parent branch version the branch diverged from, and
 * manifest_size is the parent version's manifest size in bytes at divergence.
 */
typedef struct {
    const char* name;                 // Read-only branch name
    const char* parent_branch;        // Read-only parent branch name (NULL when forked from main)
    uint64_t parent_version;          // Version on the parent branch the branch started from
    uint64_t created_at;              // Creation time in seconds since the Unix epoch
    uint64_t manifest_size;           // Parent version manifest size in bytes at divergence
    const char* const* metadata_keys;   // Read-only metadata keys, NULL when metadata_count is 0
    const char* const* metadata_values; // Read-only metadata values, parallel to metadata_keys
    size_t metadata_count;            // Number of metadata key-value pairs
} LanceDBBranch;

/**
 * One column description in a branch diff
 *
 * The strings are read-only and owned by the result that contains them.
 */
typedef struct {
    const char* name;       // Column name
    const char* data_type;  // SDK data type description
    int nullable;           // Non-zero when the column accepts nulls
} LanceDBColumnSummary;

/**
 * A column that exists in both branches with different definitions
 */
typedef struct {
    const char* name;              // Column name
    LanceDBColumnSummary main;     // Definition on main
    LanceDBColumnSummary branch;   // Definition on the compared branch
} LanceDBColumnChange;

/**
 * One index description in a branch diff
 *
 * index_type is NULL when the SDK response omitted it. columns is NULL when
 * column_count is zero.
 */
typedef struct {
    const char* index_name;          // Index name
    const char* const* columns;      // Read-only column names, NULL when column_count is 0
    size_t column_count;             // Number of entries in columns
    const char* index_type;          // Optional index type (NULL when omitted)
    const char* status;              // Index status reported by the server
} LanceDBIndexSummary;

/**
 * Row-count classification from the SDK
 */
typedef struct {
    uint64_t unchanged;        // Rows unchanged on both sides
    uint64_t new_on_base;      // Rows new on main
    uint64_t new_on_branch;    // Rows new on the branch
    uint64_t stale_recompute;  // Rows requiring recomputation
    uint64_t inputs_changed;   // Rows whose inputs changed
    int delta_available;       // Non-zero when a delta is available
} LanceDBRowCountSummary;

/**
 * Reason the server gave for a blocked cherry-pick
 *
 * LANCEDB_CHERRY_PICK_ERROR_UNKNOWN is the SDK's mapping for a code the
 * pinned SDK does not recognize; the SDK does not expose the raw code.
 */
typedef enum {
    LANCEDB_CHERRY_PICK_ERROR_BASE_MOVED = 0,
    LANCEDB_CHERRY_PICK_ERROR_ROW_COUNT_MISMATCH = 1,
    LANCEDB_CHERRY_PICK_ERROR_ROWS_CHANGED = 2,
    LANCEDB_CHERRY_PICK_ERROR_COLUMN_REMOVED = 3,
    LANCEDB_CHERRY_PICK_ERROR_COLUMN_CHANGED = 4,
    LANCEDB_CHERRY_PICK_ERROR_NOTHING_TO_APPLY = 5,
    LANCEDB_CHERRY_PICK_ERROR_NO_COLUMN_CHANGES = 6,
    LANCEDB_CHERRY_PICK_ERROR_INPUT_COLUMN_DEPENDENCY = 7,
    LANCEDB_CHERRY_PICK_ERROR_PARENT_NOT_MAIN = 8,
    LANCEDB_CHERRY_PICK_ERROR_UNKNOWN = 9
} LanceDBCherryPickErrorCode;

/**
 * One error from a branch diff or cherry-pick result
 */
typedef struct {
    LanceDBCherryPickErrorCode code;  // SDK error code, or UNKNOWN for an unrecognized value
    const char* message;              // Read-only error message
} LanceDBCherryPickError;

/**
 * Full branch diff returned by lancedb_table_diff_branch()
 *
 * Every array is in the SDK response order and NULL when its count is zero.
 * The result owns all strings and arrays. The result remains valid until
 * lancedb_free_branch_diff() is called with the same pointer. Callers must
 * not modify or free nested strings or arrays individually.
 */
typedef struct {
    const char* from_branch;                  // Branch compared against main
    uint64_t parent_version;                  // Version the branch started from
    uint64_t main_version;                    // Version of main in the diff
    uint64_t branch_version;                  // Version of the compared branch
    int base_moved;                           // Non-zero when main advanced since the branch started
    uint64_t row_count_main;                  // Row count on main
    uint64_t row_count_branch;                // Row count on the branch
    LanceDBRowCountSummary row_summary;       // Row-count classification
    const LanceDBColumnSummary* added_columns;      // Added columns, NULL when added_column_count is 0
    size_t added_column_count;                      // Number of added columns
    const LanceDBColumnSummary* removed_columns;    // Removed columns, NULL when removed_column_count is 0
    size_t removed_column_count;                    // Number of removed columns
    const LanceDBColumnChange* changed_columns;     // Changed columns, NULL when changed_column_count is 0
    size_t changed_column_count;                    // Number of changed columns
    const LanceDBIndexSummary* added_indexes;       // Added indexes, NULL when added_index_count is 0
    size_t added_index_count;                       // Number of added indexes
    const LanceDBIndexSummary* removed_indexes;     // Removed indexes, NULL when removed_index_count is 0
    size_t removed_index_count;                     // Number of removed indexes
    const LanceDBCherryPickError* errors;           // Errors, NULL when error_count is 0
    size_t error_count;                             // Number of errors
} LanceDBBranchDiff;

/**
 * Outcome reported by lancedb_table_cherry_pick()
 *
 * LANCEDB_CHERRY_PICK_STATUS_UNKNOWN is the SDK's mapping for a status the
 * pinned SDK does not recognize; the SDK does not expose the raw status.
 */
typedef enum {
    LANCEDB_CHERRY_PICK_STATUS_READY = 0,
    LANCEDB_CHERRY_PICK_STATUS_FAILED = 1,
    LANCEDB_CHERRY_PICK_STATUS_NOT_IMPLEMENTED = 2,
    LANCEDB_CHERRY_PICK_STATUS_CHERRY_PICKED = 3,
    LANCEDB_CHERRY_PICK_STATUS_UNKNOWN = 4
} LanceDBCherryPickStatus;

/**
 * Result of lancedb_table_cherry_pick()
 *
 * diff is an embedded copy of the branch diff; do not pass its address to
 * lancedb_free_branch_diff(). promoted_columns is NULL when
 * promoted_column_count is zero. main_version_after is read only when
 * has_main_version_after is non-zero, because the SDK field is optional.
 * The result owns all strings and arrays and remains valid until
 * lancedb_free_cherry_pick_result() is called with the same pointer.
 */
typedef struct {
    LanceDBCherryPickStatus status;            // Outcome reported by the server
    LanceDBBranchDiff diff;                    // Embedded branch diff
    const char* const* promoted_columns;       // Promoted column names, NULL when promoted_column_count is 0
    size_t promoted_column_count;              // Number of promoted columns
    int has_main_version_after;                // Non-zero when main_version_after is present
    uint64_t main_version_after;               // Main version after a successful cherry-pick
} LanceDBCherryPickResult;

/**
 * Kind of endpoint a table delta compares
 */
typedef enum {
    LANCEDB_DELTA_REF_VERSION = 0,  // Use the version field
    LANCEDB_DELTA_REF_TAG = 1       // Use the tag field
} LanceDBDeltaRefType;

/**
 * One endpoint of a table delta
 *
 * Set kind to LANCEDB_DELTA_REF_VERSION and version to a persisted table
 * version, or kind to LANCEDB_DELTA_REF_TAG and tag to a NUL-terminated tag
 * name. The field not selected by kind is ignored. A tag is resolved on the
 * table's current branch; a tag from another branch is rejected.
 */
typedef struct {
    LanceDBDeltaRefType kind;  // Endpoint kind
    uint64_t version;          // Version number for LANCEDB_DELTA_REF_VERSION
    const char* tag;           // Tag name for LANCEDB_DELTA_REF_TAG
} LanceDBDeltaRef;

/**
 * Read consistency mode reported by a connection
 *
 * LANCEDB_READ_CONSISTENCY_MANUAL means reads add no freshness constraint.
 * LANCEDB_READ_CONSISTENCY_STRONG means reads request the latest data.
 * LANCEDB_READ_CONSISTENCY_EVENTUAL means reads accept data within the
 * configured freshness bound. Native tables may refresh in the background
 * before the interval expires and refresh synchronously after it. Remote
 * latest-timeline requests use the interval as a per-request minimum
 * freshness bound; pinned snapshots omit that freshness routing header.
 */
typedef enum {
    LANCEDB_READ_CONSISTENCY_MANUAL = 0,
    LANCEDB_READ_CONSISTENCY_EVENTUAL = 1,
    LANCEDB_READ_CONSISTENCY_STRONG = 2
} LanceDBReadConsistencyType;

/**
 * Read consistency of a connection
 *
 * interval_ms is the configured interval in whole milliseconds for
 * LANCEDB_READ_CONSISTENCY_EVENTUAL, and 0 for the other modes. A positive
 * interval set through the connection builder is returned unchanged.
 */
typedef struct {
    LanceDBReadConsistencyType kind;  // Read consistency mode
    uint64_t interval_ms;             // Eventual consistency interval in milliseconds
} LanceDBReadConsistency;

/** @} */

/**
 * @defgroup api_connection Connections
 * @brief Connect to embedded, namespace, and remote databases.
 * @{
 */

/**
 * Create a ConnectBuilder for the given URI
 *
 * @param uri - null-terminated C string containing the database URI
 * @return Non-null pointer to LanceDBConnectBuilder on success, NULL on failure
 *
 * The URI can be:
 * - "/path/to/database" - local database on file system
 * - "s3://bucket/path/to/database" or "gs://bucket/path/to/database" - database on cloud object store
 * - "db://dbname" - LanceDB Cloud
 *
 * The returned pointer must be freed with lancedb_connect_builder_free().
 */
LanceDBConnectBuilder* lancedb_connect(const char* uri);

/**
 * Execute the connection and return a Connection handle
 *
 * @param builder - pointer to LanceDBConnectBuilder returned from lancedb_connect()
 * @param connection - pointer to receive the LanceDBConnection pointer
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return Error code indicating success or failure
 *
 * A non-NULL builder is consumed on every path and must not be used after
 * calling. connection is written only on success, when it receives a handle
 * that must be freed with lancedb_connection_free(); on failure a non-NULL
 * connection slot is left unchanged. An existing *error_message value is
 * preserved on success, consistent with
 * lancedb_connect_namespace_builder_execute(). On error, a non-NULL
 * error_message receives a message that the caller frees with
 * lancedb_free_string().
 */
LanceDBError lancedb_connect_builder_execute( LanceDBConnectBuilder* builder,
		    LanceDBConnection** connection, char** error_message);


/**
 * Set an option for the storage layer
 * @param builder - pointer to LanceDBConnectBuilder returned from lancedb_connect()
 * @param key -  null-terminated C string containing the name of the option
 * @param value -  null-terminated C string containing the value of the option
 * @return Non-null pointer to LanceDBConnectBuilder on success, NULL on failure
 *
 * The builder is consumed by this function and must not be used after calling.
 *
 * The key and value are going through basic validation at this point, and error will happen when trying to execute a builder
 * with unsupported key or value.
 */
LanceDBConnectBuilder* lancedb_connect_builder_storage_option(LanceDBConnectBuilder* builder, const char* key, const char* value);

/**
 * Set the API key for a LanceDB Enterprise or LanceDB Cloud connection
 *
 * @param builder - pointer to LanceDBConnectBuilder returned from lancedb_connect()
 * @param api_key - NUL-terminated API key
 * @return Non-null pointer to LanceDBConnectBuilder on success, NULL on failure
 *
 * The builder is consumed by this function and must not be used after calling.
 * The value is only applied when the connection dispatches to the SDK remote
 * path, which is selected by a URI beginning with db. The setter does not add
 * it to native storage options. Without the remote feature, the setter
 * validates and returns the builder without applying the value. A NULL or
 * non-UTF-8 api_key returns NULL.
 */
LanceDBConnectBuilder* lancedb_connect_builder_api_key(LanceDBConnectBuilder* builder, const char* api_key);

/**
 * Set the region for a db:// connection
 *
 * @param builder - pointer to LanceDBConnectBuilder returned from lancedb_connect()
 * @param region - NUL-terminated region name
 * @return Non-null pointer to LanceDBConnectBuilder on success, NULL on failure
 *
 * The builder is consumed by this function and must not be used after calling.
 * The SDK requires a region for both LanceDB Cloud and LanceDB Enterprise
 * connections, including when a host override is used. This setter is only
 * applied when the connection dispatches to the SDK remote path, which is
 * selected by a URI beginning with db. Without the remote feature, the setter
 * validates and returns the builder without applying the value. A NULL or
 * non-UTF-8 region returns NULL.
 */
LanceDBConnectBuilder* lancedb_connect_builder_region(LanceDBConnectBuilder* builder, const char* region);

/**
 * Set the host override for a LanceDB Enterprise connection
 *
 * @param builder - pointer to LanceDBConnectBuilder returned from lancedb_connect()
 * @param host_override - NUL-terminated host override
 * @return Non-null pointer to LanceDBConnectBuilder on success, NULL on failure
 *
 * The builder is consumed by this function and must not be used after calling.
 * The value is only applied when the connection dispatches to the SDK remote
 * path, which is selected by a URI beginning with db. Use the URI db:// together
 * with a host override to connect to LanceDB Enterprise tables stored directly
 * in the storage root. Without the remote feature, the setter validates and
 * returns the builder without applying the value. A NULL or non-UTF-8
 * host_override returns NULL.
 */
LanceDBConnectBuilder* lancedb_connect_builder_host_override(LanceDBConnectBuilder* builder, const char* host_override);

/**
 * Set the SQL service host override for a LanceDB Enterprise connection
 *
 * @param builder - pointer to LanceDBConnectBuilder returned from lancedb_connect()
 * @param sql_host_override - NUL-terminated SQL host override
 * @return Non-null pointer to LanceDBConnectBuilder on success, NULL on failure
 *
 * The builder is consumed by this function and must not be used after calling.
 * The value is only applied when the connection dispatches to the SDK remote
 * path, which is selected by a URI beginning with db. The SQL endpoint is
 * contacted only when the connection executes SQL; setting this option does
 * not contact it. Without the remote feature, the setter validates and returns
 * the builder without applying the value. A NULL or non-UTF-8
 * sql_host_override returns NULL.
 */
LanceDBConnectBuilder* lancedb_connect_builder_sql_host_override(LanceDBConnectBuilder* builder, const char* sql_host_override);

/**
 * Set an additional property for the connection's namespace client
 *
 * @param builder - pointer to LanceDBConnectBuilder returned from lancedb_connect()
 * @param key - NUL-terminated property name
 * @param value - NUL-terminated property value
 * @return Non-null pointer to LanceDBConnectBuilder on success, NULL on failure
 *
 * The builder is consumed by this function and must not be used after calling.
 * A NULL or non-UTF-8 key or value returns NULL. LanceDB Enterprise connections
 * reject a non-empty set of namespace client properties when the builder is
 * executed: lancedb_connect_builder_execute() returns LANCEDB_NOT_SUPPORTED.
 */
LanceDBConnectBuilder* lancedb_connect_builder_namespace_client_property(LanceDBConnectBuilder* builder, const char* key, const char* value);

/**
 * Enable or disable manifest-backed directory namespace mode
 *
 * @param builder - pointer to LanceDBConnectBuilder returned from lancedb_connect()
 * @param enabled - true to enable manifest-backed directory namespace mode
 * @return Non-null pointer to LanceDBConnectBuilder on success, NULL on failure
 *
 * The builder is consumed by this function and must not be used after calling.
 * This option applies to local native connections. When enabled, the connection
 * uses the directory namespace directly for all table operations and forces
 * dir_listing_to_manifest_migration_enabled=true. LanceDB Enterprise
 * connections reject enabling this option when the builder is executed:
 * lancedb_connect_builder_execute() returns LANCEDB_NOT_SUPPORTED. Passing
 * false keeps the default behavior.
 */
LanceDBConnectBuilder* lancedb_connect_builder_manifest_enabled(LanceDBConnectBuilder* builder, bool enabled);

/**
 * Set the read consistency interval
 *
 * @param builder - pointer to LanceDBConnectBuilder returned from lancedb_connect()
 * @param interval_ms - read consistency interval in milliseconds
 * @return Non-null pointer to LanceDBConnectBuilder on success, NULL on failure
 *
 * The builder is consumed by this function and must not be used after calling.
 * Leaving this setter unused keeps the SDK default: no freshness constraint is
 * added to reads. An explicit 0 requests strong consistency. A positive value
 * requests eventual consistency. The interval only affects reads; writes are
 * always consistent.
 *
 * Native connections use the interval as a refresh target: the SDK may refresh
 * in the background before it expires and refreshes synchronously on the next
 * read after it. Remote latest-timeline requests send the interval as a
 * per-request minimum freshness bound; pinned snapshots omit that freshness
 * routing header. Reading the configured mode back through
 * lancedb_connection_read_consistency() is supported for native connections;
 * the SDK reports LANCEDB_NOT_SUPPORTED for remote connections.
 *
 * A NULL builder returns NULL.
 */
LanceDBConnectBuilder* lancedb_connect_builder_read_consistency_interval_ms(LanceDBConnectBuilder* builder, uint64_t interval_ms);

/**
 * Set session for the connection builder
 *
 * @param builder - pointer to LanceDBConnectBuilder returned from lancedb_connect()
 * @param session - pointer to LanceDBSession, or NULL to keep current/default session behavior
 * @return Non-null pointer to LanceDBConnectBuilder on success, NULL on failure
 *
 * The builder is consumed by this function and must not be used after calling.
 * Passing NULL session is a no-op.
 *
 * A custom session is only supported for embedded connections. LanceDB Enterprise
 * connections (db:// URIs) reject it when the builder is executed:
 * lancedb_connect_builder_execute() returns LANCEDB_NOT_SUPPORTED with an error message
 * stating that session is not supported for LanceDB Enterprise connections.
 */
LanceDBConnectBuilder* lancedb_connect_builder_session(LanceDBConnectBuilder* builder, const LanceDBSession* session);

/**
 * Client configuration fields of LanceDBClientConfig
 *
 * Every bit makes the matching scalar an override. A field whose bit is not set
 * keeps the SDK value, so an omitted timeout or retry budget stays distinct
 * from an explicit zero. Strings use NULL for "unset", so an empty non-NULL
 * string remains an explicit empty value.
 */
typedef enum {
    LANCEDB_CLIENT_CONFIG_TIMEOUT = 1 << 0,
    LANCEDB_CLIENT_CONFIG_CONNECT_TIMEOUT = 1 << 1,
    LANCEDB_CLIENT_CONFIG_READ_TIMEOUT = 1 << 2,
    LANCEDB_CLIENT_CONFIG_POOL_IDLE_TIMEOUT = 1 << 3,
    LANCEDB_CLIENT_CONFIG_RETRIES = 1 << 4,
    LANCEDB_CLIENT_CONFIG_CONNECT_RETRIES = 1 << 5,
    LANCEDB_CLIENT_CONFIG_READ_RETRIES = 1 << 6,
    LANCEDB_CLIENT_CONFIG_BACKOFF_FACTOR = 1 << 7,
    LANCEDB_CLIENT_CONFIG_BACKOFF_JITTER = 1 << 8,
    LANCEDB_CLIENT_CONFIG_RETRY_STATUSES = 1 << 9,
    LANCEDB_CLIENT_CONFIG_BLOB_REQUEST_CONCURRENCY = 1 << 10,
    LANCEDB_CLIENT_CONFIG_MAX_BYTES_PER_REQUEST = 1 << 11,
    LANCEDB_CLIENT_CONFIG_MAX_REQUEST_DURATION = 1 << 12,
    LANCEDB_CLIENT_CONFIG_TLS_ASSERT_HOSTNAME = 1 << 13
} LanceDBClientConfigField;

/**
 * HTTP client configuration for a remote connection
 *
 * A NULL pointer or a zero-initialized struct keeps the SDK
 * ClientConfig::default(), including the SDK user agent, empty extra headers,
 * and no TLS configuration, which leaves the SDK/reqwest default hostname
 * verification enabled. When the
 * library is built with the remote feature, an unknown set_fields bit, a
 * missing required array or invalid UTF-8 makes
 * lancedb_connect_builder_client_config() return NULL and consume the builder.
 * Builds without the remote feature consume the builder and leave it unchanged.
 *
 * All strings, status codes and header arrays are borrowed for the duration of
 * that call and copied into the SDK configuration. NULL string fields keep
 * their SDK value. A non-NULL string, including an empty one, is passed
 * unchanged. Negative or non-finite backoff values and invalid scalar values
 * are left for the SDK to reject when the builder is executed, preserving its
 * error code and message timing.
 *
 * Timeouts use milliseconds and retry counts use the SDK's uint8_t range. When
 * a set_fields bit is present, its value is forwarded even when zero. The SDK
 * still owns environment fallbacks: leaving a field unset lets the matching
 * LANCE_CLIENT_* variable or the SDK default apply. user_id is NULL to keep the
 * SDK LANCEDB_USER_ID / LANCEDB_USER_ID_ENV_KEY fallback, or a non-NULL string
 * to set it explicitly. max_bytes_per_request and max_request_duration_ms use
 * zero to disable their respective insert-request cut, as the SDK does.
 *
 * Retry status codes are copied from retry_statuses when
 * LANCEDB_CLIENT_CONFIG_RETRY_STATUSES is set. A zero retry_status_count with
 * that bit selects an empty list and disables status retries; a non-zero count
 * requires a non-NULL array. extra_header_count greater than zero requires both
 * parallel header arrays and every entry to be non-NULL and valid UTF-8. Static
 * extra headers replace the same SDK default headers when names collide. The
 * SDK user agent and a configured or environment-derived user_id are applied
 * afterward, so they take precedence over a static header with the same name.
 *
 * A TLS configuration exists when any TLS path is non-NULL or
 * LANCEDB_CLIENT_CONFIG_TLS_ASSERT_HOSTNAME is set. The SDK TlsConfig default
 * asserts the server hostname, so leaving the bit unset keeps verification on;
 * setting it with 0 explicitly disables it and with 1 keeps it on. Any other
 * value makes the setter return NULL. tls_cert_file and tls_key_file are read
 * only when both are configured; a lone client certificate or key path is not
 * read. tls_ssl_ca_cert is read when configured. Missing or unreadable configured
 * files, invalid certificate contents, invalid header names or values, an
 * unsupported id_delimiter, a blob request concurrency outside the SDK's
 * positive semaphore range (including an environment-derived value) and invalid
 * retry status codes are reported by lancedb_connect_builder_execute().
 *
 * Callback-based header providers are not part of this plain-data structure.
 * Native (non-db://) connections ignore the configuration, matching the other
 * remote-only builder setters.
 */
typedef struct {
    uint32_t set_fields;                     // Bitmask of LanceDBClientConfigField values (0 = defaults)
    const char* user_agent;                  // User agent (NULL keeps the SDK default)
    uint64_t timeout_ms;                     // Overall request timeout; bit required
    uint64_t connect_timeout_ms;             // Connection timeout; bit required
    uint64_t read_timeout_ms;                // Response read timeout; bit required
    uint64_t pool_idle_timeout_ms;           // Idle connection pool timeout; bit required
    uint8_t retries;                         // Request retry budget; bit required
    uint8_t connect_retries;                 // Connect retry budget; bit required
    uint8_t read_retries;                    // Read retry budget; bit required
    float backoff_factor;                    // Retry backoff factor in seconds; bit required
    float backoff_jitter;                    // Retry backoff jitter in seconds; bit required
    const uint16_t* retry_statuses;          // Retryable status codes (NULL when count is 0)
    size_t retry_status_count;               // Number of retryable status codes
    size_t blob_request_concurrency;         // Maximum concurrent blob requests; bit required
    uint64_t max_bytes_per_request;          // Insert byte budget; bit required
    uint64_t max_request_duration_ms;        // Insert time budget; bit required
    const char* id_delimiter;                // Namespace identifier delimiter (NULL to omit)
    const char* user_id;                     // User ID (NULL keeps the SDK environment fallback)
    const char* tls_cert_file;               // Client certificate path (NULL to omit)
    const char* tls_key_file;                // Client private key path (NULL to omit)
    const char* tls_ssl_ca_cert;             // CA certificate path (NULL to omit)
    int tls_assert_hostname;                 // 0 = false, 1 = true; bit required
    const char* const* extra_header_keys;    // Static header names (NULL when count is 0)
    const char* const* extra_header_values;  // Static header values matching the names
    size_t extra_header_count;               // Number of static extra headers
} LanceDBClientConfig;

/**
 * Set the HTTP client configuration for a remote connection
 *
 * @param builder - pointer to LanceDBConnectBuilder returned from lancedb_connect()
 * @param config - pointer to read-only configuration, or NULL for every SDK default
 * @return Non-null pointer to LanceDBConnectBuilder on success, NULL on failure
 *
 * The builder is consumed by this function and must not be used after calling.
 * A successful call replaces the builder's ClientConfig; it does not merge with
 * an earlier call. The configuration only reaches connections dispatched to the
 * SDK remote path. Native connections keep the builder unchanged. The SDK
 * validates the configuration when the builder is executed.
 */
LanceDBConnectBuilder* lancedb_connect_builder_client_config(LanceDBConnectBuilder* builder, const LanceDBClientConfig* config);

/**
 * Free a ConnectBuilder
 *
 * @param builder - pointer to LanceDBConnectBuilder returned from lancedb_connect()
 *
 * After calling this function, the builder pointer must not be used.
 */
void lancedb_connect_builder_free(LanceDBConnectBuilder* builder);

/**
 * Namespace operations that can be pushed down to the namespace backend
 *
 * These values are the SDK's complete set of pushdown operations. They are
 * explicit so a C caller can pass and be rejected for any other value.
 */
typedef enum {
    LANCEDB_NAMESPACE_PUSHDOWN_QUERY_TABLE = 0,  // Execute queries through the namespace server
    LANCEDB_NAMESPACE_PUSHDOWN_CREATE_TABLE = 1  // Request the SDK's CreateTable pushdown
} LanceDBNamespacePushdownOperation;

/**
 * Create a ConnectNamespaceBuilder for a Lance namespace implementation
 *
 * @param ns_impl - NUL-terminated namespace implementation name
 * @param property_keys - array of property names (NULL when property_count is 0)
 * @param property_values - array of property values matching property_keys
 * @param property_count - number of namespace implementation properties
 * @return Non-null pointer to LanceDBConnectNamespaceBuilder on success, NULL on failure
 *
 * "dir" is always available and requires a "root" property. "rest" is
 * available only when the library is built with the remote feature. The pinned
 * SDK rejects any other implementation when the builder is executed. Other
 * properties, their defaults, and required values are defined by the selected
 * namespace implementation.
 *
 * The implementation name, property arrays, and every string they point to are
 * borrowed only for the duration of this call and copied into the returned
 * builder. A zero property_count accepts NULL arrays. A positive count requires
 * both arrays and every entry to be non-NULL, NUL-terminated UTF-8 strings.
 *
 * For the "dir" implementation, properties whose names start with "storage."
 * apply to its own root object store. This is separate from the connection
 * storage-options map set by
 * lancedb_connect_namespace_builder_storage_option().
 *
 * The returned builder must be freed with
 * lancedb_connect_namespace_builder_free() if it is not executed.
 */
LanceDBConnectNamespaceBuilder* lancedb_connect_namespace(
    const char* ns_impl,
    const char* const* property_keys,
    const char* const* property_values,
    size_t property_count
);

/**
 * Execute the namespace connection and return a Connection handle
 *
 * @param builder - pointer to LanceDBConnectNamespaceBuilder returned from lancedb_connect_namespace()
 * @param connection - pointer to receive the LanceDBConnection pointer
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return Error code indicating success or failure
 *
 * The builder is consumed on every path and must not be used after calling.
 * connection is written only on success, when it receives a handle that must be
 * freed with lancedb_connection_free(); on failure it is left unchanged. An
 * existing *error_message value is preserved on success, consistent with
 * lancedb_connect_builder_execute(). On error, a non-NULL error_message
 * receives a message that the caller frees with lancedb_free_string().
 *
 * The SDK decides whether the implementation exists in this build, validates
 * the implementation properties, and performs the connection. The URI of the
 * returned connection is "namespace://<implementation>".
 */
LanceDBError lancedb_connect_namespace_builder_execute(
    LanceDBConnectNamespaceBuilder* builder,
    LanceDBConnection** connection,
    char** error_message
);

/**
 * Set an option for the storage layer of a namespace connection
 *
 * @param builder - pointer to LanceDBConnectNamespaceBuilder returned from lancedb_connect_namespace()
 * @param key - NUL-terminated option name
 * @param value - NUL-terminated option value
 * @return Non-null pointer to LanceDBConnectNamespaceBuilder on success, NULL on failure
 *
 * The builder is consumed by this function and must not be used after calling.
 * A NULL or non-UTF-8 key or value returns NULL. These options apply to table
 * access through the namespace connection. They are not the "storage.*"
 * implementation properties that the "dir" namespace uses for its own root
 * object store.
 *
 * At the pinned SDK, the table creation path falls back to this map only when
 * the namespace response omits storage_options. An explicit empty
 * storage_options map prevents that fallback. The selected map overrides
 * per-write keys with the same name.
 *
 * Opening a table uses the describe_table storage options plus the options
 * supplied on that open request, with namespace values winning on overlaps; it
 * does not use this connection-level map there.
 *
 * The SDK does not validate every option during the namespace connection
 * itself.
 */
LanceDBConnectNamespaceBuilder* lancedb_connect_namespace_builder_storage_option(
    LanceDBConnectNamespaceBuilder* builder,
    const char* key,
    const char* value
);

/**
 * Set an additional property for the namespace client
 *
 * @param builder - pointer to LanceDBConnectNamespaceBuilder returned from lancedb_connect_namespace()
 * @param key - NUL-terminated property name
 * @param value - NUL-terminated property value
 * @return Non-null pointer to LanceDBConnectNamespaceBuilder on success, NULL on failure
 *
 * The builder is consumed by this function and must not be used after calling.
 * A NULL or non-UTF-8 key or value returns NULL.
 *
 * Namespace client properties are merged into the implementation properties
 * when the builder is executed. The SDK decides which value wins when the same
 * key is supplied as an implementation property and as a namespace client
 * property. For the "dir" implementation, a property whose name starts with
 * "storage." configures its own root object store even when it is supplied
 * through this setter; this differs from
 * lancedb_connect_namespace_builder_storage_option(), which configures table
 * access for the connection.
 */
LanceDBConnectNamespaceBuilder* lancedb_connect_namespace_builder_namespace_client_property(
    LanceDBConnectNamespaceBuilder* builder,
    const char* key,
    const char* value
);

/**
 * Set the read consistency interval for a namespace connection
 *
 * @param builder - pointer to LanceDBConnectNamespaceBuilder returned from lancedb_connect_namespace()
 * @param interval_ms - read consistency interval in milliseconds
 * @return Non-null pointer to LanceDBConnectNamespaceBuilder on success, NULL on failure
 *
 * The builder is consumed by this function and must not be used after calling.
 * Leaving this setter unused keeps the SDK default: no freshness constraint is
 * added to reads. An explicit 0 requests strong consistency. A positive value
 * requests eventual consistency. A NULL builder returns NULL.
 *
 * This has the same meaning and defaults as
 * lancedb_connect_builder_read_consistency_interval_ms() for a URI connection.
 * The configured mode can be read back with
 * lancedb_connection_read_consistency().
 */
LanceDBConnectNamespaceBuilder* lancedb_connect_namespace_builder_read_consistency_interval_ms(
    LanceDBConnectNamespaceBuilder* builder,
    uint64_t interval_ms
);

/**
 * Set a session for the namespace connection builder
 *
 * @param builder - pointer to LanceDBConnectNamespaceBuilder returned from lancedb_connect_namespace()
 * @param session - pointer to LanceDBSession, or NULL to keep the default session
 * @return Non-null pointer to LanceDBConnectNamespaceBuilder on success, NULL on failure
 *
 * The builder is consumed by this function and must not be used after calling.
 * Passing NULL session is a no-op. A non-NULL session is borrowed for the call;
 * the builder keeps it alive through a cloned reference, and the caller still
 * owns its handle and must free it with lancedb_session_free().
 */
LanceDBConnectNamespaceBuilder* lancedb_connect_namespace_builder_session(
    LanceDBConnectNamespaceBuilder* builder,
    const LanceDBSession* session
);

/**
 * Add an operation to push down to the namespace backend
 *
 * @param builder - pointer to LanceDBConnectNamespaceBuilder returned from lancedb_connect_namespace()
 * @param operation - raw integer value of a LanceDBNamespacePushdownOperation enumerator
 * @return Non-null pointer to LanceDBConnectNamespaceBuilder on success, NULL on failure
 *
 * The builder is consumed by this function and must not be used after calling.
 * Operations are opt-in; unset means the SDK executes them locally. Call this
 * setter more than once to add multiple operations. A value outside the
 * LanceDBNamespacePushdownOperation enumerators returns NULL.
 *
 * With LANCEDB_NAMESPACE_PUSHDOWN_QUERY_TABLE, queries execute through the
 * namespace server; a query with a nonempty order_by still executes locally and
 * needs client access to table storage. LANCEDB_NAMESPACE_PUSHDOWN_CREATE_TABLE
 * is the SDK's other pushdown value; at the pinned SDK version the namespace
 * create path still declares the table and writes locally, so this option is
 * forwarded without changing that path. The SDK owns whether and how an
 * implementation applies either operation.
 */
LanceDBConnectNamespaceBuilder* lancedb_connect_namespace_builder_pushdown_operation(
    LanceDBConnectNamespaceBuilder* builder,
    int operation
);

/**
 * Free a ConnectNamespaceBuilder
 *
 * @param builder - pointer to LanceDBConnectNamespaceBuilder returned from lancedb_connect_namespace()
 *
 * After calling this function, the builder pointer must not be used.
 */
void lancedb_connect_namespace_builder_free(LanceDBConnectNamespaceBuilder* builder);

/**
 * Get the URI of the connection
 *
 * @param connection - pointer to LanceDBConnection
 * @return Pointer to null-terminated C string containing the URI, or NULL on failure
 *
 * The returned string is valid until the connection is freed.
 * The caller must not free the returned string.
 */
const char* lancedb_connection_uri(const LanceDBConnection* connection);

/**
 * Get the read consistency of the connection
 *
 * @param connection - pointer to LanceDBConnection
 * @param consistency_out - pointer to receive the read consistency
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return Error code indicating success or failure
 *
 * On success, consistency_out describes the configured mode. A connection
 * created without lancedb_connect_builder_read_consistency_interval_ms()
 * reports LANCEDB_READ_CONSISTENCY_MANUAL. An explicit zero interval reports
 * LANCEDB_READ_CONSISTENCY_STRONG. A positive interval reports
 * LANCEDB_READ_CONSISTENCY_EVENTUAL with interval_ms set to that value.
 *
 * On failure, consistency_out is left unchanged so a caller can keep its own
 * sentinel there. LanceDB Enterprise and LanceDB Cloud connections return
 * LANCEDB_NOT_SUPPORTED because the SDK does not expose their read
 * consistency, even though the interval still controls their per-request
 * freshness bound. A non-NULL error_message is cleared on entry and must be
 * freed with lancedb_free_string() after an error.
 *
 * This function does not fall back to a predicted mode when the SDK query
 * fails.
 */
LanceDBError lancedb_connection_read_consistency(
    const LanceDBConnection* connection,
    LanceDBReadConsistency* consistency_out,
    char** error_message
);

/**
 * Get table names from the connection
 *
 * @param connection - pointer to LanceDBConnection
 * @param names_out - pointer to receive array of string pointers
 * @param count_out - pointer to receive count of table names
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return Error code indicating success or failure
 *
 * The caller is responsible for freeing the returned strings and array
 * using lancedb_free_table_names(). If error_message is provided and an error
 * occurs, the caller must free the error message with lancedb_free_string().
 */
LanceDBError lancedb_connection_table_names(
    const LanceDBConnection* connection,
    char*** names_out,
    size_t* count_out,
    char** error_message
);

/**
 * Free table names array returned by lancedb_connection_table_names
 *
 * @param names - array of string pointers returned by lancedb_connection_table_names
 * @param count - number of strings in the array
 */
void lancedb_free_table_names(char** names, size_t count);

/**
 * Create a TableNamesBuilder for paginated table listing
 *
 * @param connection - pointer to LanceDBConnection
 * @return Non-null pointer to LanceDBTableNamesBuilder on success, NULL on failure
 *
 * The returned pointer must be freed with lancedb_table_names_builder_free().
 * Use lancedb_table_names_builder_limit() and lancedb_table_names_builder_start_after()
 * to configure pagination before calling lancedb_table_names_builder_execute().
 */
LanceDBTableNamesBuilder* lancedb_connection_table_names_builder(
    const LanceDBConnection* connection
);

/**
 * Set limit on TableNamesBuilder
 *
 * @param builder - pointer to LanceDBTableNamesBuilder
 * @param limit - maximum number of table names to return
 * @return Non-null pointer to LanceDBTableNamesBuilder on success, NULL on failure
 *
 * The builder is consumed by this function and must not be used after calling.
 */
LanceDBTableNamesBuilder* lancedb_table_names_builder_limit(
    LanceDBTableNamesBuilder* builder,
    unsigned int limit
);

/**
 * Set start_after on TableNamesBuilder for pagination
 *
 * @param builder - pointer to LanceDBTableNamesBuilder
 * @param start_after - null-terminated C string containing the table name to start after
 * @return Non-null pointer to LanceDBTableNamesBuilder on success, NULL on failure
 *
 * The builder is consumed by this function and must not be used after calling.
 * This is used for pagination - returns table names alphabetically after the specified name.
 */
LanceDBTableNamesBuilder* lancedb_table_names_builder_start_after(
    LanceDBTableNamesBuilder* builder,
    const char* start_after
);

/**
 * Set the namespace path on TableNamesBuilder
 *
 * @param builder - pointer to LanceDBTableNamesBuilder
 * @param namespace_path - array of namespace component strings (NULL when namespace_path_len is 0)
 * @param namespace_path_len - number of namespace components (0 for the root namespace)
 * @return Non-null pointer to LanceDBTableNamesBuilder on success, NULL on invalid input
 *
 * The builder is consumed by this function and must not be used after calling.
 * A zero length selects the root namespace. A positive length requires a non-NULL
 * array whose entries are non-NULL, NUL-terminated UTF-8 strings.
 */
LanceDBTableNamesBuilder* lancedb_table_names_builder_namespace(
    LanceDBTableNamesBuilder* builder,
    const char* const* namespace_path,
    size_t namespace_path_len
);

/**
 * Execute TableNamesBuilder and return table names
 *
 * @param builder - pointer to LanceDBTableNamesBuilder (consumed by this function)
 * @param names_out - pointer to receive array of string pointers
 * @param count_out - pointer to receive count of table names
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return Error code indicating success or failure
 *
 * The builder is consumed by this function and must not be used after calling.
 * The caller is responsible for freeing the returned strings and array
 * using lancedb_free_table_names(). If error_message is provided and an error
 * occurs, the caller must free the error message with lancedb_free_string().
 */
LanceDBError lancedb_table_names_builder_execute(
    LanceDBTableNamesBuilder* builder,
    char*** names_out,
    size_t* count_out,
    char** error_message
);

/**
 * Free a TableNamesBuilder
 *
 * @param builder - pointer to LanceDBTableNamesBuilder
 *
 * After calling this function, the builder pointer must not be used.
 */
void lancedb_table_names_builder_free(LanceDBTableNamesBuilder* builder);

/**
 * Open an existing table
 *
 * @param connection - pointer to LanceDBConnection
 * @param table_name - null-terminated C string containing the table name
 * @param table_out - pointer to receive the opened table
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return Error code indicating success or failure.
 *
 * On success, the table returned in table_out must be freed with lancedb_table_free().
 * If error_message is provided and an error occurs, the caller must free
 * the error message with lancedb_free_string().
 */
LanceDBError lancedb_connection_open_table(
    const LanceDBConnection* connection,
    const char* table_name,
    LanceDBTable** table_out,
    char** error_message
);

/**
 * Open table option fields of LanceDBOpenTableOptions
 *
 * Every bit makes the matching field an override. A field whose bit is not set
 * keeps the LanceDB default, so an omitted index cache size stays distinct from
 * an explicit zero, and an omitted managed-versioning value stays distinct from
 * an explicit false.
 */
typedef enum {
    LANCEDB_OPEN_TABLE_INDEX_CACHE_SIZE = 1 << 0,
    LANCEDB_OPEN_TABLE_VERSION = 1 << 1,
    LANCEDB_OPEN_TABLE_MANAGED_VERSIONING = 1 << 2
} LanceDBOpenTableField;

/**
 * Options for lancedb_connection_open_table_with_options
 *
 * A zero-initialized struct keeps every SDK default. set_fields selects the
 * scalar overrides; an unknown bit is rejected with LANCEDB_INVALID_ARGUMENT.
 *
 * namespace_path is a borrowed array of namespace_path_len borrowed strings.
 * Zero length is the root namespace and accepts a NULL array. A positive length
 * requires a non-NULL array of non-NULL, NUL-terminated UTF-8 strings.
 *
 * location and branch are borrowed strings. NULL leaves that option unset; a
 * non-NULL string, including an empty one, is passed unchanged. For branch, the
 * SDK treats "main" as the default branch. storage_keys and storage_values are
 * parallel arrays of storage_count borrowed strings for this table only. A
 * zero count omits per-table storage options. A non-zero count requires both arrays and
 * every entry to be non-NULL and valid UTF-8.
 *
 * index_cache_size and version are read only when their set_fields bit is set.
 * Setting the bit selects the value, including zero, instead of treating zero as
 * omitted. The SDK still owns precedence: an embedded open attaches the
 * connection's Session to the dataset builder, and that Session supplies the
 * effective index cache capacity. An explicit index_cache_size, including zero,
 * is therefore forwarded but does not resize or disable the Session's cache.
 * managed_versioning is also read only when its bit is set: 0 selects false and
 * 1 selects true; every other value is rejected. Leaving the bit unset passes no
 * managed-versioning preference.
 *
 * The C layer forwards these options to the SDK unchanged. The SDK decides which
 * backend accepts them and may resolve values from the backend. At the pinned
 * SDK version:
 * - An embedded listing root table uses location in place of the derived table
 *   path, and per-table storage options override connection options on the same
 *   key. index_cache_size follows the Session precedence above, so the
 *   connection Session's current effective capacity remains in use. version
 *   selects a read-only detached snapshot. managed_versioning is forwarded, but
 *   a plain listing connection has no namespace client, so it does not enable
 *   namespace-managed commits.
 * - An embedded namespace table (a non-empty namespace_path) gets its location,
 *   namespace storage options and managed-versioning state from the namespace.
 *   The SDK does not apply the request location or managed_versioning value to
 *   that open, and index_cache_size follows the Session precedence above.
 *   Per-table storage options are passed through, but namespace values win on
 *   the same key. version and branch still compose in the SDK after the
 *   namespace table is opened.
 * - An Enterprise connection rejects index_cache_size, per-table storage
 *   options, location and managed_versioning (whether true or false) with
 *   LANCEDB_NOT_SUPPORTED. namespace_path is part of the table identifier, and
 *   version and branch are resolved by the server.
 *
 * The options struct and every string it points to are borrowed for the
 * duration of the call and are not consumed.
 */
typedef struct {
    uint32_t set_fields;                  // Bitmask of LanceDBOpenTableField values (0 = defaults)
    const char* const* namespace_path;    // Namespace path parts (NULL with length 0 for root)
    size_t namespace_path_len;            // Number of namespace path parts
    const char* location;                 // Custom location (NULL to derive it)
    const char* const* storage_keys;      // Storage option keys (NULL when storage_count is 0)
    const char* const* storage_values;    // Storage option values matching storage_keys
    size_t storage_count;                 // Number of per-table storage options
    uint32_t index_cache_size;            // Index cache entries; read only when its bit is set
    uint64_t version;                     // Table version; read only when its bit is set
    const char* branch;                   // Branch name (NULL for the default branch)
    int managed_versioning;               // 0 = false, 1 = true; read only when its bit is set
} LanceDBOpenTableOptions;

/**
 * Open an existing table with SDK open-table builder options
 *
 * @param connection - pointer to LanceDBConnection
 * @param table_name - null-terminated C string containing the table name
 * @param options - pointer to read-only options, or NULL for every SDK default
 * @param table_out - pointer to receive the opened table
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return Error code indicating success or failure
 *
 * On entry, a non-NULL table_out is set to NULL and the error message slot is
 * cleared. The table is written only on success, so a failure leaves it NULL.
 * This differs from lancedb_connection_open_table, which leaves its output
 * unchanged on failure. The caller frees a returned table with
 * lancedb_table_free() and a returned error message with lancedb_free_string().
 */
LanceDBError lancedb_connection_open_table_with_options(
    const LanceDBConnection* connection,
    const char* table_name,
    const LanceDBOpenTableOptions* options,
    LanceDBTable** table_out,
    char** error_message
);

/**
 * Options for lancedb_connection_clone_table()
 *
 * A NULL options pointer or a zero-initialized struct keeps the SDK defaults:
 * clone the source's current version into the root namespace, fail when the
 * target already exists, and perform a shallow clone.
 *
 * source_version is read only when has_source_version is non-zero. source_tag
 * is read only when non-NULL. An embedded directory connection rejects setting
 * both when it reads the source. If exist_ok is non-zero and the target already
 * exists, it opens the existing target without reading the source. On remote
 * connections, the server determines how to handle the source selectors and
 * exist_ok.
 *
 * target_namespace is an array of namespace components; a zero length accepts
 * NULL and selects the root namespace. A positive length requires a non-NULL
 * array whose entries are non-NULL, NUL-terminated UTF-8 strings.
 *
 * Setting has_is_shallow to a non-zero value and is_shallow to 0 requests a
 * deep clone. Embedded directory connections return LANCEDB_NOT_SUPPORTED for
 * a deep clone or a non-empty target namespace. Remote connections forward
 * both settings to the server. A namespace connection rejects every clone.
 *
 * The strings and array are borrowed for the duration of the call. The struct
 * is borrowed and is not consumed.
 */
typedef struct {
    const char* source_tag;              // Optional source tag (NULL for none)
    const char* const* target_namespace; // Target namespace components (NULL when length is 0)
    size_t target_namespace_len;         // Number of target namespace components (0 for root)
    uint64_t source_version;             // Source version when has_source_version is non-zero
    int has_source_version;              // Non-zero to use source_version, 0 to omit it
    int exist_ok;                        // Non-zero to keep an existing target unchanged
    int has_is_shallow;                  // Non-zero to read is_shallow, 0 for the SDK default
    int is_shallow;                      // Shallow clone flag when has_is_shallow is non-zero
} LanceDBCloneTableOptions;

/**
 * Clone a table from an existing source table
 *
 * @param connection - pointer to LanceDBConnection
 * @param target_table_name - null-terminated C string containing the target table name
 * @param source_uri - null-terminated C string containing the source table URI
 * @param options - optional clone options (NULL for the SDK defaults)
 * @param table_out - pointer to receive the cloned table
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return Error code indicating success or failure
 *
 * On entry, a non-NULL table_out is set to NULL and a non-NULL error_message is
 * cleared. On success, table_out receives a handle that the caller must free
 * with lancedb_table_free(). On failure, table_out remains NULL.
 *
 * A shallow clone creates a new table that shares the source's data files while
 * keeping an independent manifest. The clone therefore depends on those files
 * remaining in the source table. The SDK currently supports shallow clones on
 * embedded directory connections; a deep clone request fails with
 * LANCEDB_NOT_SUPPORTED there.
 *
 * The source and target strings, every string and component array reached from
 * options, and the options struct itself are borrowed only for the duration of
 * this call.
 *
 * If error_message is provided and an error occurs, the caller must free the
 * error message with lancedb_free_string().
 */
LanceDBError lancedb_connection_clone_table(
    const LanceDBConnection* connection,
    const char* target_table_name,
    const char* source_uri,
    const LanceDBCloneTableOptions* options,
    LanceDBTable** table_out,
    char** error_message
);

/**
 * Drop a table from the database
 *
 * @param connection - pointer to LanceDBConnection
 * @param table_name - null-terminated C string containing the table name
 * @param _namespace - null-terminated C string containing the namespace (NULL for no namespace)
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return Error code indicating success or failure
 *
 * If error_message is provided and an error occurs, the caller must free
 * the error message with lancedb_free_string().
 */
LanceDBError lancedb_connection_drop_table(
    const LanceDBConnection* connection,
    const char* table_name,
    const char* _namespace,
    char** error_message
);

/**
 * Drop a table from a multi-level namespace path
 *
 * @param connection - pointer to LanceDBConnection
 * @param table_name - null-terminated C string containing the table name
 * @param namespace_path - array of namespace component strings (NULL when namespace_path_len is 0)
 * @param namespace_path_len - number of namespace components (0 for the root namespace)
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return Error code indicating success or failure
 *
 * If error_message is provided and an error occurs, the caller must free
 * the error message with lancedb_free_string().
 */
LanceDBError lancedb_connection_drop_table_path(
    const LanceDBConnection* connection,
    const char* table_name,
    const char* const* namespace_path,
    size_t namespace_path_len,
    char** error_message
);

/**
 * Rename a table in the database
 *
 * @param connection - pointer to LanceDBConnection
 * @param old_name - null-terminated C string containing the current table name
 * @param new_name - null-terminated C string containing the new table name
 * @param cur_namespace - null-terminated C string containing the current namespace (NULL for no namespace)
 * @param new_namespace - null-terminated C string containing the new namespace (NULL omits the target namespace)
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return Error code indicating success or failure
 *
 * Renaming is supported in LanceDB Enterprise and by namespace backends that
 * implement it. Directory namespaces support renaming tables resolved through
 * their manifest, including within or across child namespaces; directory-listing
 * root tables cannot be renamed.
 *
 * A NULL new_namespace omits the target namespace from the rename request. It
 * does not promise a move to the root namespace.
 * If error_message is provided and an error occurs, the caller must free
 * the error message with lancedb_free_string().
 */
LanceDBError lancedb_connection_rename_table(
    const LanceDBConnection* connection,
    const char* old_name,
    const char* new_name,
    const char* cur_namespace,
    const char* new_namespace,
    char** error_message
);

/**
 * Rename a table between multi-level namespace paths
 *
 * @param connection - pointer to LanceDBConnection
 * @param old_name - null-terminated C string containing the current table name
 * @param new_name - null-terminated C string containing the new table name
 * @param cur_namespace_path - array of current namespace components (NULL when cur_namespace_path_len is 0)
 * @param cur_namespace_path_len - number of current namespace components (0 for the root namespace)
 * @param new_namespace_path - array of new namespace components; zero length omits the target namespace
 * @param new_namespace_path_len - number of new namespace components; zero omits the target namespace
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return Error code indicating success or failure
 *
 * Renaming is supported in LanceDB Enterprise and by namespace backends that
 * implement it. Directory namespaces support renaming tables resolved through
 * their manifest, including within or across child namespaces; directory-listing
 * root tables cannot be renamed.
 *
 * A zero-length new_namespace_path omits the target namespace from the rename
 * request. It does not promise a move to the root namespace.
 * If error_message is provided and an error occurs, the caller must free
 * the error message with lancedb_free_string().
 */
LanceDBError lancedb_connection_rename_table_path(
    const LanceDBConnection* connection,
    const char* old_name,
    const char* new_name,
    const char* const* cur_namespace_path,
    size_t cur_namespace_path_len,
    const char* const* new_namespace_path,
    size_t new_namespace_path_len,
    char** error_message
);

/**
 * Drop all tables in the database
 *
 * @param connection - pointer to LanceDBConnection
 * @param _namespace - null-terminated C string containing the namespace (NULL for no namespace)
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return Error code indicating success or failure
 *
 * If error_message is provided and an error occurs, the caller must free
 * the error message with lancedb_free_string().
 */
LanceDBError lancedb_connection_drop_all_tables(
    const LanceDBConnection* connection,
    const char* _namespace,
    char** error_message
);

/**
 * Drop all tables in a multi-level namespace path
 *
 * @param connection - pointer to LanceDBConnection
 * @param namespace_path - array of namespace component strings (NULL when namespace_path_len is 0)
 * @param namespace_path_len - number of namespace components (0 for the root namespace)
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return Error code indicating success or failure
 *
 * If error_message is provided and an error occurs, the caller must free
 * the error message with lancedb_free_string().
 */
LanceDBError lancedb_connection_drop_all_tables_path(
    const LanceDBConnection* connection,
    const char* const* namespace_path,
    size_t namespace_path_len,
    char** error_message
);

/**
 * Create a new namespace
 *
 * @param connection - pointer to LanceDBConnection
 * @param namespace_name - null-terminated C string containing the namespace name
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return Error code indicating success or failure
 *
 * If error_message is provided and an error occurs, the caller must free
 * the error message with lancedb_free_string().
 */
LanceDBError lancedb_connection_create_namespace(
    const LanceDBConnection* connection,
    const char* namespace_name,
    char** error_message
);

/**
 * Create a namespace at a multi-level namespace path
 *
 * @param connection - pointer to LanceDBConnection
 * @param namespace_path - array of namespace component strings (NULL when namespace_path_len is 0)
 * @param namespace_path_len - number of namespace components (0 for the root namespace)
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return Error code indicating success or failure
 *
 * If error_message is provided and an error occurs, the caller must free
 * the error message with lancedb_free_string().
 */
LanceDBError lancedb_connection_create_namespace_path(
    const LanceDBConnection* connection,
    const char* const* namespace_path,
    size_t namespace_path_len,
    char** error_message
);

/**
 * Drop a namespace
 *
 * @param connection - pointer to LanceDBConnection
 * @param namespace_name - null-terminated C string containing the namespace name
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return Error code indicating success or failure
 *
 * If error_message is provided and an error occurs, the caller must free
 * the error message with lancedb_free_string().
 */
LanceDBError lancedb_connection_drop_namespace(
    const LanceDBConnection* connection,
    const char* namespace_name,
    char** error_message
);

/**
 * Drop a namespace at a multi-level namespace path
 *
 * @param connection - pointer to LanceDBConnection
 * @param namespace_path - array of namespace component strings (NULL when namespace_path_len is 0)
 * @param namespace_path_len - number of namespace components (0 for the root namespace)
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return Error code indicating success or failure
 *
 * If error_message is provided and an error occurs, the caller must free
 * the error message with lancedb_free_string().
 */
LanceDBError lancedb_connection_drop_namespace_path(
    const LanceDBConnection* connection,
    const char* const* namespace_path,
    size_t namespace_path_len,
    char** error_message
);

/**
 * List all namespaces in the database
 *
 * @param connection - pointer to LanceDBConnection
 * @param namespace_parent - null-terminated C string containing the parent namespace (NULL for root)
 * @param namespaces_out - pointer to receive array of string pointers
 * @param count_out - pointer to receive count of namespace names
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return Error code indicating success or failure
 *
 * The caller is responsible for freeing the returned strings and array
 * using lancedb_free_namespace_list(). If error_message is provided and an error
 * occurs, the caller must free the error message with lancedb_free_string().
 */
LanceDBError lancedb_connection_list_namespaces(
    const LanceDBConnection* connection,
    const char* namespace_parent,
    char*** namespaces_out,
    size_t* count_out,
    char** error_message
);

/**
 * List all namespaces under a multi-level namespace path
 *
 * @param connection - pointer to LanceDBConnection
 * @param namespace_path - array of namespace component strings (NULL when namespace_path_len is 0)
 * @param namespace_path_len - number of namespace components (0 for the root namespace)
 * @param namespaces_out - pointer to receive array of string pointers
 * @param count_out - pointer to receive count of namespace names
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return Error code indicating success or failure
 *
 * The caller is responsible for freeing the returned strings and array
 * using lancedb_free_namespace_list(). If error_message is provided and an error
 * occurs, the caller must free the error message with lancedb_free_string().
 *
 * On entry, a non-NULL namespaces_out is set to NULL and a non-NULL count_out
 * to zero. They are published only after the complete list has been converted.
 * A non-NULL error_message is cleared on entry.
 */
LanceDBError lancedb_connection_list_namespaces_path(
    const LanceDBConnection* connection,
    const char* const* namespace_path,
    size_t namespace_path_len,
    char*** namespaces_out,
    size_t* count_out,
    char** error_message
);

/**
 * Description of a namespace returned by lancedb_connection_describe_namespace_path
 *
 * property_keys and property_values are parallel arrays of property_count entries,
 * and likewise for context_keys and context_values. has_properties and has_context
 * distinguish a map the SDK omitted (false) from an empty map (true, with NULL
 * arrays and a zero count).
 */
typedef struct {
    bool has_properties;         // Whether the SDK returned namespace properties
    char** property_keys;        // Namespace property names
    char** property_values;      // Namespace property values
    size_t property_count;       // Number of namespace properties
    bool has_context;            // Whether the SDK returned response context
    char** context_keys;         // Response context names
    char** context_values;       // Response context values
    size_t context_count;        // Number of response context entries
} LanceDBNamespaceDescription;

/**
 * Describe a namespace at a multi-level namespace path
 *
 * @param connection - pointer to LanceDBConnection
 * @param namespace_path - array of namespace component strings (NULL when namespace_path_len is 0)
 * @param namespace_path_len - number of namespace components (0 for the root namespace)
 * @param description_out - pointer to receive the namespace description
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return Error code indicating success or failure
 *
 * On entry, a non-NULL description_out is set to the empty description and a
 * non-NULL error_message is cleared. The description is published only after the
 * complete response has been converted.
 *
 * On success, free the description with lancedb_free_namespace_description().
 * If error_message is provided and an error occurs, the caller must free
 * the error message with lancedb_free_string().
 */
LanceDBError lancedb_connection_describe_namespace_path(
    const LanceDBConnection* connection,
    const char* const* namespace_path,
    size_t namespace_path_len,
    LanceDBNamespaceDescription* description_out,
    char** error_message
);

/**
 * Free namespace list array returned by lancedb_connection_list_namespaces
 *
 * @param namespaces - pointer to array returned by lancedb_connection_list_namespaces()
 * @param count - count returned by lancedb_connection_list_namespaces()
 *
 * After calling this function, the namespaces pointer must not be used.
 */
void lancedb_free_namespace_list(char** namespaces, size_t count);

/**
 * Free a namespace description returned by lancedb_connection_describe_namespace_path
 *
 * @param description - pointer to a description returned by
 *   lancedb_connection_describe_namespace_path(), or NULL
 *
 * The arrays and their strings are freed and the fields are zeroed. The struct
 * itself stays owned by the caller.
 */
void lancedb_free_namespace_description(LanceDBNamespaceDescription* description);

/**
 * Free a Connection
 *
 * @param connection - pointer to LanceDBConnection returned from lancedb_connect_builder_execute()
 *
 * After calling this function, the connection pointer must not be used.
 */
void lancedb_connection_free(LanceDBConnection* connection);

/** @} */

/**
 * @defgroup api_session Sessions and object stores
 * @brief Reusable session caches and custom object-store registries.
 * @{
 */

/**
 * Create a new session
 *
 * @param options - pointer to LanceDBSessionOptions, or NULL for defaults
 * @return Non-null pointer to LanceDBSession on success, NULL on failure
 *
 * The returned session must be freed with lancedb_session_free(). A session can be
 * attached to an embedded connection with lancedb_connect_builder_session(); Enterprise
 * (db://) connections reject custom sessions when the builder is executed.
 */
LanceDBSession* lancedb_session_new(const LanceDBSessionOptions* options);

/**
 * Create a new session with a custom ObjectStoreRegistry
 *
 * @param options - pointer to LanceDBSessionOptions, or NULL for defaults
 * @param registry - pointer to LanceDBObjectStoreRegistry, or NULL to use a default registry.
 *
 * - The registry must be created using "lancedb_registry_new()" API
 * - When registry is non-NULL, this function takes ownership of it and it
 *   will be released when the session is freed.
 * - If registry is NULL, a default ObjectStoreRegistry is created internally.
 *
 * @return Non-null pointer to LanceDBSession on success, NULL on failure
 *
 * The returned session must be freed with lancedb_session_free().
 */
LanceDBSession* lancedb_session_new_with_registry( const LanceDBSessionOptions* options,
					    const LanceDBObjectStoreRegistry* registry);

/**
 * Create a new ObjectStoreRegistry
 *
 * Creates a new registry with default ObjectStore providers (local filesystem, S3, etc.).
 * After creation, custom providers can be registered which can even override the
 * default ones for existing scheme using "lancedb_registry_insert_provider()"
 *
 * - If the registry created is passed to a session, ownership will be transferred
 * - If NOT passed to a session, it must be freed with lancedb_registry_free()
 *
 * @return Non-null pointer to LanceDBObjectStoreRegistry on success, NULL on failure
 */
LanceDBObjectStoreRegistry* lancedb_registry_new(void);

/**
 * Free an ObjectStoreRegistry
 *
 * @param registry - pointer to LanceDBObjectStoreRegistry to free
 *
 * Only call this if the registry was NOT passed to any session.
 * If the registry was passed to a session, the session owns it and will free it.
 */
void lancedb_registry_free(LanceDBObjectStoreRegistry* registry);

/**
 * Insert or override an ObjectStoreProvider into an ObjectStoreRegistry for a given URL scheme
 *
 * @param registry - pointer to LanceDBObjectStoreRegistry
 * @param scheme - URL scheme to register the provider for (e.g., "s3")
 * @param provider - pointer to LanceDBObjectStoreProvider. Ownership is transferred
 *                   to the registry; do not free the provider after this call.
 *
 * @return 0 on success, -1 on failure
 *
 * If error_message is provided and an error occurs, the caller must free
 * the error message with lancedb_free_string().
 */
LanceDBError lancedb_registry_insert_provider( LanceDBObjectStoreRegistry* registry,
			    const char* scheme, LanceDBObjectStoreProvider* provider,
    			    char** error_message);

/**
 * Free an ObjectStoreProvider
 *
 * @param provider - pointer to LanceDBObjectStoreProvider to free
 *
 * Only call this if the provider was NOT passed to lancedb_registry_insert_provider().
 * If the provider was inserted into a registry, the registry owns it.
 */
void lancedb_object_store_provider_free(LanceDBObjectStoreProvider* provider);

/**
 * Get index cache stats for a session
 *
 * @param session - pointer to LanceDBSession
 * @param out_stats - pointer to receive session cache statistics
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return Error code indicating success or failure
 *
 * If error_message is provided and an error occurs, the caller must free
 * the error message with lancedb_free_string().
 */
LanceDBError lancedb_session_index_cache_stats(
    const LanceDBSession* session,
    LanceDBSessionCacheStats* out_stats,
    char** error_message
);

/**
 * Get metadata cache stats for a session
 *
 * @param session - pointer to LanceDBSession
 * @param out_stats - pointer to receive session cache statistics
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return Error code indicating success or failure
 *
 * If error_message is provided and an error occurs, the caller must free
 * the error message with lancedb_free_string().
 */
LanceDBError lancedb_session_metadata_cache_stats(
    const LanceDBSession* session,
    LanceDBSessionCacheStats* out_stats,
    char** error_message
);

/**
 * Free a Session
 *
 * @param session - pointer to LanceDBSession
 *
 * After calling this function, the session pointer must not be used.
 */
void lancedb_session_free(LanceDBSession* session);

/** @} */

/**
 * @defgroup api_table Tables and data
 * @brief Create tables, write and read data, inspect schemas, and change metadata.
 * @{
 */

/**
 * Free a Table
 *
 * @param table - pointer to LanceDBTable returned from lancedb_connection_open_table()
 *
 * After calling this function, the table pointer must not be used.
 */
void lancedb_table_free(LanceDBTable* table);

/**
 * Run a callback on an extended stack.
 *
 * If the remaining stack is below red_zone bytes, a new stack segment
 * of stack_size bytes is allocated before calling the callback.
 * Use this to wrap coroutine bodies so that inner FFI calls can reuse
 * the same stack segment instead of allocating one per call.
 *
 * The callback must not unwind across the FFI boundary (no C++ exceptions,
 * no longjmp). In C++ code, wrap the callback body in try/catch.
 * Using an extern "C" function is recommended, though a non-capturing
 * lambda will compile on all major compilers as well.
 *
 * @param callback - function pointer to call on the extended stack (NULL is a safe no-op)
 * @param user_data - opaque pointer passed through to the callback
 * @param red_zone - minimum remaining stack (bytes) before growing
 * @param stack_size - size of the new stack segment (bytes) if growth is needed (clamped to at least red_zone)
 */
void lancedb_run_on_stack(void (*callback)(void*), void* user_data, size_t red_zone, size_t stack_size);

/**
 * Create table modes for lancedb_table_create_with_options()
 *
 * The values mirror the LanceDB SDK's CreateTableMode. Create returns
 * LANCEDB_TABLE_ALREADY_EXISTS when the table exists, Overwrite replaces it with
 * the supplied data, and ExistOk opens the existing table and ignores the
 * supplied data. ExistOk schema and reopen behavior is backend-dependent:
 * - A root listing connection compares the existing table schema with the
 *   request schema (the reader schema when a reader is supplied, otherwise
 *   schema_ptr) and reopens through the SDK ExistOk callback. This binding
 *   supplies the custom location and per-table storage options to that callback
 *   so a same-name default table is not opened instead.
 * - Nonempty namespace paths and direct namespace connections ask the
 *   namespace backend to open the existing table; the pinned SDK does not call
 *   the C callback or compare schemas on that path.
 * - The remote client compares the schema only on its AlreadyExists reopen path.
 */
typedef enum {
    LANCEDB_CREATE_TABLE_MODE_CREATE = 0,
    LANCEDB_CREATE_TABLE_MODE_OVERWRITE = 1,
    LANCEDB_CREATE_TABLE_MODE_EXIST_OK = 2
} LanceDBCreateTableMode;

/**
 * Fields of LanceDBWriteOptions
 *
 * Every bit makes the matching field an override. A field whose bit is not set
 * keeps the LanceDB SDK default, so an omitted option stays distinct from an
 * explicit zero or false.
 */
typedef enum {
    LANCEDB_WRITE_OPTIONS_MAX_ROWS_PER_FILE = 1 << 0,
    LANCEDB_WRITE_OPTIONS_MAX_ROWS_PER_GROUP = 1 << 1,
    LANCEDB_WRITE_OPTIONS_MAX_BYTES_PER_FILE = 1 << 2,
    LANCEDB_WRITE_OPTIONS_DATA_STORAGE_VERSION = 1 << 3,
    LANCEDB_WRITE_OPTIONS_ENABLE_STABLE_ROW_IDS = 1 << 4
} LanceDBWriteOptionsField;

/**
 * Per-call Lance write parameters shared by table create and add
 *
 * set_fields selects the fields that are read; every other field keeps the SDK
 * default. A NULL pointer to this struct, or set_fields equal to zero, keeps the
 * SDK WriteOptions without Lance write parameters. That is what default remote
 * table creation requires.
 *
 * data_storage_version is parsed by the SDK's own version parser, for example
 * "2.0", "2.1", "legacy" or "stable". It must not be NULL when its bit is set.
 * enable_stable_row_ids must be 0 or 1 when its bit is set. The string is
 * borrowed for the duration of the call.
 */
typedef struct {
    uint32_t set_fields;                 // Bitmask of LanceDBWriteOptionsField values (0 = defaults)
    uint64_t max_rows_per_file;          // Maximum number of rows per data file
    uint64_t max_rows_per_group;         // Maximum number of rows per row group
    uint64_t max_bytes_per_file;         // Maximum number of bytes per data file
    const char* data_storage_version;    // Data storage version string (NULL when unset)
    int enable_stable_row_ids;           // Stable row IDs (1 = true, 0 = false)
} LanceDBWriteOptions;

/**
 * Options for lancedb_table_create_with_options()
 *
 * mode is one of the LanceDBCreateTableMode values. namespace_path is an array
 * of namespace_path_len borrowed strings; NULL with length 0 means the root
 * namespace. A positive length with a NULL array, a NULL part or a part that is
 * not valid UTF-8 is rejected with LANCEDB_INVALID_ARGUMENT. location is an
 * optional borrowed custom table location; NULL derives it from the database
 * URI and the table name. On a root listing database it is also the location
 * used by the ExistOk callback when reopening the existing table. Namespace and
 * remote backends resolve table locations themselves and may reject or ignore
 * it according to the SDK.
 *
 * storage_keys and storage_values are parallel arrays of storage_count borrowed
 * strings for this table only. An empty array leaves the SDK's per-table storage
 * options unset. The same options are passed to the SDK ExistOk callback when a
 * root listing table is reopened. write_options is NULL for the SDK defaults or
 * points to a readable LanceDBWriteOptions. allow_external_blob_outside_bases
 * must be 0 or 1. The SDK decides which options a backend supports and applies
 * its own precedence to overlapping create/open options and write parameters.
 */
typedef struct {
    int mode;                                      // One of the LanceDBCreateTableMode values
    const char* const* namespace_path;             // Namespace path parts (NULL with length 0 for root)
    size_t namespace_path_len;                     // Number of namespace path parts
    const char* location;                          // Custom table location (NULL to derive it)
    const char* const* storage_keys;               // Per-table storage option keys
    const char* const* storage_values;             // Per-table storage option values
    size_t storage_count;                          // Number of per-table storage options
    const LanceDBWriteOptions* write_options;      // Per-call write options (NULL for SDK defaults)
    int allow_external_blob_outside_bases;         // Allow external blob URIs (1 = true, 0 = false)
} LanceDBCreateTableOptions;

/**
 * Create a new table with Arrow schema
 *
 * @param connection - pointer to LanceDBConnection
 * @param table_name - null-terminated C string containing the table name
 * @param schema_ptr - pointer to Arrow C ABI schema
 * @param reader - pointer to LanceDBRecordBatchReader or NULL for empty table
 * @param table_out - pointer to receive the created table
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return Error code indicating success or failure
 *
 * The caller is responsible for freeing the returned table with
 * lancedb_table_free(). A non-NULL reader is taken only after the required
 * pointers are non-NULL, the table name is valid UTF-8 and the Arrow schema
 * imports. Failures before that point leave the reader caller-owned; after the
 * schema imports, the reader is consumed even if table-name validation or table
 * creation fails. Do NOT call lancedb_record_batch_reader_free() after the
 * reader is consumed. If error_message is provided and an error occurs, the
 * caller must free the error message with lancedb_free_string().
 */
LanceDBError lancedb_table_create(
    const LanceDBConnection* connection,
    const char* table_name,
    const FFI_ArrowSchema* schema_ptr,
    LanceDBRecordBatchReader* reader,
    LanceDBTable** table_out,
    char** error_message
);

/**
 * Create a new table with SDK create builder options
 *
 * @param connection - pointer to LanceDBConnection
 * @param table_name - null-terminated C string containing the table name
 * @param schema_ptr - pointer to Arrow C ABI schema, borrowed for the call
 * @param reader - pointer to LanceDBRecordBatchReader or NULL for empty table
 * @param options - pointer to LanceDBCreateTableOptions, or NULL for SDK defaults
 * @param table_out - pointer to receive the created or existing table
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return Error code indicating success or failure
 *
 * This entry point applies the create mode, namespace path, custom location,
 * per-table storage options, write options and external-blob option to the SDK
 * create builder. Write options are applied before the per-table storage and
 * external-blob setters so those setters extend rather than replace them. The
 * SDK still applies its own precedence for the create mode and overlapping write
 * parameters. The SDK decides which options a backend supports and may resolve
 * a table location from the backend instead of using location.
 *
 * For ExistOk, the SDK behavior is backend-dependent. A root listing compares
 * the request schema (the reader schema when a reader is supplied, otherwise
 * schema_ptr) and reopens through the callback with the supplied location and
 * storage options. Nonempty namespaces and direct namespace connections open the
 * existing table through the namespace backend without calling the C callback or
 * comparing schemas. The remote client compares the schema only on its
 * AlreadyExists reopen path.
 *
 * On entry, a non-NULL table_out is set to NULL. It is written only on success,
 * so a failure leaves it NULL. The caller is responsible for freeing the
 * returned table with lancedb_table_free() and the Arrow schema with its own
 * release callback. This function takes ownership of the reader (when not NULL)
 * as soon as it is called. The reader is released exactly once, also when
 * another argument, an option, the Arrow schema or the SDK rejects the create.
 * Do NOT call lancedb_record_batch_reader_free() after calling this function.
 * If error_message is provided and an error occurs, the caller must free
 * the error message with lancedb_free_string().
 */
LanceDBError lancedb_table_create_with_options(
    const LanceDBConnection* connection,
    const char* table_name,
    const FFI_ArrowSchema* schema_ptr,
    LanceDBRecordBatchReader* reader,
    const LanceDBCreateTableOptions* options,
    LanceDBTable** table_out,
    char** error_message
);

/**
 * Get table schema as Arrow C ABI
 *
 * @param table - pointer to LanceDBTable
 * @param schema_out - pointer to receive the Arrow schema
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return Error code indicating success or failure
 *         Caller is responsible for releasing the schema using Arrow C ABI
 *
 * If error_message is provided and an error occurs, the caller must free
 * the error message with lancedb_free_string().
 */
LanceDBError lancedb_table_arrow_schema(
    const LanceDBTable* table,
    FFI_ArrowSchema** schema_out,
    char** error_message
);

/**
 * Write the table schema as an Arrow IPC file that holds no batch
 *
 * @param table - pointer to LanceDBTable (borrowed, not consumed)
 * @param buffer_out - pointer to receive the file bytes
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return Error code indicating success or failure
 *
 * On success, the buffer holds a readable Arrow IPC file with the full table
 * schema and no record batch. Its schema can create a table of the same shape
 * through lancedb_record_batch_reader_from_ipc_file() without any row.
 * On entry, both outputs and the error message are cleared.
 * The caller must free the bytes with lancedb_ipc_buffer_free(). If
 * error_message is provided and an error occurs, the caller must free the error
 * message with lancedb_free_string().
 */
LanceDBError lancedb_table_schema_ipc_file(
    const LanceDBTable* table,
    LanceDBIpcBuffer* buffer_out,
    char** error_message
);

/** @} */

/**
 * @defgroup api_blob Blob files
 * @brief Discover, fetch, and read row-addressable blobs.
 * @{
 */

/**
 * List row-addressable Blob v1 and v2 columns in declaration order
 *
 * @param table - pointer to LanceDBTable
 * @param columns_out - pointer to receive the array of column-name strings
 * @param count_out - pointer to receive the number of names
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return Error code indicating success or failure
 *
 * Struct-nested blobs use dotted paths such as `info.blob`. Blobs inside lists
 * are omitted because a row id cannot identify an individual list element.
 * On entry, both outputs and the error message are cleared. An empty list
 * succeeds with *columns_out NULL and *count_out zero. The caller owns the
 * strings and array and frees them with lancedb_free_blob_columns(). If
 * error_message is provided and an error occurs, the caller must free the error
 * message with lancedb_free_string().
 */
LanceDBError lancedb_table_blob_columns(
    const LanceDBTable* table,
    char*** columns_out,
    size_t* count_out,
    char** error_message
);

/**
 * Free the blob column-name array returned by lancedb_table_blob_columns
 *
 * @param columns - array of string pointers returned by lancedb_table_blob_columns
 * @param count - number of strings in the array
 */
void lancedb_free_blob_columns(char** columns, size_t count);

/**
 * One borrowed request for lancedb_table_fetch_blob_ranges
 *
 * `row_id` selects the row, `offset` is the byte offset from the beginning of
 * that row's blob value, and `length` is the number of bytes to read. The
 * request array is borrowed for the duration of the call.
 */
typedef struct {
    uint64_t row_id;
    uint64_t offset;
    uint64_t length;
} LanceDBBlobRangeRequest;

/**
 * Bytes of one blob read owned by LanceDB
 *
 * A read that returned no bytes leaves both fields zeroed; a successful read of
 * non-empty bytes sets `data` to the first byte and `len` to the byte count.
 * `data` stays valid until lancedb_blob_buffer_free() is called, independently
 * of the handle and of the table. The caller must not write to or free it, and
 * keeps both fields unchanged until then.
 */
typedef struct {
    const uint8_t* data;  /* Start of the blob bytes, NULL when the read returned no bytes */
    size_t len;           /* Number of bytes in the read */
} LanceDBBlobBuffer;

/**
 * Physical location of one blob inside its data file
 *
 * `has_position` is non-zero for embedded handles. Remote handles do not expose
 * their storage layout and report it as zero with `position` zeroed. This is not
 * the read cursor: seek and read move the cursor, while read_range leaves it
 * unchanged.
 */
typedef struct {
    bool has_position;  /* Non-zero when the handle exposes a physical byte offset */
    uint64_t position;  /* Physical byte offset when has_position is non-zero */
} LanceDBBlobFilePosition;

/**
 * Materialize blob bytes for borrowed row ids
 *
 * @param table - pointer to LanceDBTable
 * @param column - NUL-terminated UTF-8 blob column name, borrowed for the call
 * @param row_ids - borrowed row id array; NULL is allowed only when num_row_ids is 0
 * @param num_row_ids - number of row ids
 * @param array_out - pointer to receive one Arrow C Data LargeBinary array
 * @param schema_out - pointer to receive its Arrow C Data schema
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return Error code indicating success or failure
 *
 * The result follows row_ids in length, order and duplicates. NULL blobs stay
 * NULL and valid empty blobs are empty byte strings. With a valid blob column,
 * an empty input returns an empty array without reading blob bytes. One missing
 * or deleted row id rejects the whole request without partial results.
 * On entry, both outputs and the error message are cleared. On success the
 * array and schema are independent of the table and of each other; the caller
 * frees the array with lancedb_free_arrow_array() and the schema with
 * lancedb_free_arrow_schema(). On failure both outputs stay clear. Blob
 * availability and errors on remote tables are returned unchanged from the
 * SDK. If error_message is provided and an error occurs, the caller must free
 * the error message with lancedb_free_string().
 */
LanceDBError lancedb_table_fetch_blobs(
    const LanceDBTable* table,
    const char* column,
    const uint64_t* row_ids,
    size_t num_row_ids,
    FFI_ArrowArray** array_out,
    FFI_ArrowSchema** schema_out,
    char** error_message
);

/**
 * Materialize borrowed blob-local byte ranges
 *
 * @param table - pointer to LanceDBTable
 * @param column - NUL-terminated UTF-8 blob column name, borrowed for the call
 * @param requests - borrowed request array; NULL is allowed only when num_requests is 0
 * @param num_requests - number of requests
 * @param array_out - pointer to receive one Arrow C Data LargeBinary array
 * @param schema_out - pointer to receive its Arrow C Data schema
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return Error code indicating success or failure
 *
 * The result follows the request order exactly, including duplicate and
 * reordered requests. NULL blobs stay NULL, empty ranges on non-NULL blobs are
 * empty byte strings. With a valid blob column, an empty request list returns an
 * empty array without reading blob bytes. One missing row id or invalid range
 * rejects the whole request without partial results.
 * On entry, both outputs and the error message are cleared. On success the
 * array and schema are independent of the table and of each other; the caller
 * frees the array with lancedb_free_arrow_array() and the schema with
 * lancedb_free_arrow_schema(). On failure both outputs stay clear. Blob
 * availability and errors on remote tables are returned unchanged from the
 * SDK. If error_message is provided and an error occurs, the caller must free
 * the error message with lancedb_free_string().
 */
LanceDBError lancedb_table_fetch_blob_ranges(
    const LanceDBTable* table,
    const char* column,
    const LanceDBBlobRangeRequest* requests,
    size_t num_requests,
    FFI_ArrowArray** array_out,
    FFI_ArrowSchema** schema_out,
    char** error_message
);

/**
 * Open lazy blob file handles for borrowed row ids
 *
 * @param table - pointer to LanceDBTable
 * @param column - NUL-terminated UTF-8 blob column name, borrowed for the call
 * @param row_ids - borrowed row id array; NULL is allowed only when num_row_ids is 0
 * @param num_row_ids - number of row ids
 * @param files_out - pointer to receive the array of optional handles
 * @param count_out - pointer to receive the number of array entries
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return Error code indicating success or failure
 *
 * The array follows row_ids in length, order and duplicates. A NULL blob leaves
 * the matching entry NULL; a valid empty blob yields a non-NULL handle whose
 * size is zero. With a valid blob column, an empty input returns *files_out
 * NULL and *count_out zero without opening handles. One missing or deleted row
 * id rejects the whole request without partial handles.
 *
 * Fetching opens handles without materializing a blob, but it is not always
 * free of I/O. An embedded External blob stored without a size queries its
 * object size during fetch. A remote External blob in that state may issue a
 * byte-range size probe (HTTP 206 for bytes=0-0). Access and response errors
 * can therefore fail the fetch before any handle is returned.
 *
 * Each handle keeps its own cursor and buffers and does not depend on the C
 * table handle; it stays usable after lancedb_table_free(). An embedded handle
 * captures the SDK's selected storage and offset, and a remote handle captures
 * the selected version and branch. No BlobFile retention lease is acquired for
 * a handle, so dataset cleanup can remove old data and external storage controls
 * its content. A remote row-ID handle also carries no descriptor
 * source-commit identity, so it does not detect same-name recreation; reads
 * can fail after any of these changes.
 * On entry, both outputs and the error message are cleared. On success the
 * caller frees the handles and array with lancedb_free_blob_files(). On failure
 * both outputs stay clear. Blob availability and errors on remote tables are
 * returned unchanged from the SDK. If error_message is provided and an error
 * occurs, the caller must free the error message with lancedb_free_string().
 */
LanceDBError lancedb_table_fetch_blob_files(
    const LanceDBTable* table,
    const char* column,
    const uint64_t* row_ids,
    size_t num_row_ids,
    LanceDBBlobFile*** files_out,
    size_t* count_out,
    char** error_message
);

/**
 * Free handles and the array returned by lancedb_table_fetch_blob_files
 *
 * @param files - array of optional handles returned by lancedb_table_fetch_blob_files
 * @param count - number of entries in the array
 *
 * Frees every non-NULL handle and the array. Passing NULL is a no-op. A handle
 * freed earlier with lancedb_blob_file_free() must have been set to NULL in the
 * array before this call.
 */
void lancedb_free_blob_files(LanceDBBlobFile** files, size_t count);

/**
 * Open one lazy blob file from a borrowed Arrow descriptor row
 *
 * @param table - table whose source opened the descriptor
 * @param descriptors - one Arrow C Data struct array of descriptor rows, borrowed for the call
 * @param descriptor_schema - Arrow C Data schema for `descriptors`, borrowed for the call
 * @param row_index - zero-based logical row to open
 * @param file_out - pointer to receive the optional handle
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return Error code indicating success or failure
 *
 * The input is the struct column produced by the SDK SQL `blob_descriptor`
 * function. Fields are identified by name and must have these exact Arrow
 * types: kind UInt8, position UInt64, size UInt64, blob_id UInt32, blob_uri
 * Utf8, and source Struct{uri Utf8, version UInt64, committed_at_micros UInt64,
 * branch Utf8, column Utf8, row_reference UInt64, row_is_address Boolean}.
 * Field order does not matter and extra fields are ignored. A consumer may pass
 * a column produced by another SDK or received through Arrow Flight.
 *
 * `descriptors` and `descriptor_schema` are borrowed only for this call and
 * remain owned by the caller. This function does not consume them and does not
 * call their Arrow C Data release callbacks; the caller releases them normally
 * after the call returns. `row_index` addresses the logical array, so it
 * accounts for Arrow slices and offsets. A top-level NULL descriptor succeeds
 * with *file_out NULL. A valid empty blob succeeds with a non-NULL handle whose
 * size is zero. For a non-NULL row, `source` and all required fields except
 * `branch` must be non-NULL; `branch` NULL identifies the main branch.
 *
 * The SDK verifies the descriptor's pinned source identity and opens the exact
 * source version and row; the binding does not reinterpret or repair the
 * source. On native tables the check happens during open: the branch URI,
 * source version, commit microseconds, column and row reference are validated,
 * and the caller's table checkout is not changed. Remote tables require a
 * server with blob-file support (0.5.0 or newer) and send the source URI,
 * branch, version, commit microseconds and row reference to the authenticated
 * byte-range endpoint; the server performs the source check, so a mismatch can
 * surface at open or on a later read. An External descriptor with a zero stored
 * size can issue an object metadata/HEAD request during open, so access or
 * response errors can fail before a handle is returned. As with
 * lancedb_table_fetch_blob_files(), no BlobFile retention lease is acquired:
 * dataset cleanup or external object changes can make later reads fail. On
 * success the returned handle owns its SDK BlobFile and stays usable after this
 * table and the Arrow input are released; free it with
 * lancedb_blob_file_free().
 *
 * On entry, *file_out and the error message are cleared. On failure *file_out
 * stays NULL. NULL required pointers, a negative or out-of-range row_index,
 * missing fields, wrong struct field types and NULL required values return
 * LANCEDB_INVALID_ARGUMENT. A valid non-struct top-level Arrow type, or any
 * Arrow C Data import failure, returns LANCEDB_ARROW. Non-NULL Arrow inputs
 * must conform to the Arrow C Data Interface. SDK failures keep the existing
 * error mapping. If error_message is provided and an error occurs, the caller
 * must free the error message with lancedb_free_string().
 */
LanceDBError lancedb_table_open_blob_file(
    const LanceDBTable* table,
    const FFI_ArrowArray* descriptors,
    const FFI_ArrowSchema* descriptor_schema,
    int64_t row_index,
    LanceDBBlobFile** file_out,
    char** error_message
);

/**
 * Free one blob file handle
 *
 * @param file - handle returned by lancedb_table_fetch_blob_files or
 *               lancedb_table_open_blob_file, or NULL
 *
 * The handle must not be used after this call. Freeing NULL is a no-op.
 */
void lancedb_blob_file_free(LanceDBBlobFile* file);

/**
 * Return the blob length in bytes
 *
 * @param file - blob file handle returned by lancedb_table_fetch_blob_files or
 *               lancedb_table_open_blob_file
 * @param size_out - pointer to receive the length in bytes
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return Error code indicating success or failure
 *
 * The size stays available after the handle is closed. Zero is a valid size for
 * an empty blob. On entry, the error message is cleared. On failure size_out is
 * left unchanged. If error_message is provided and an error occurs, the caller
 * must free the error message with lancedb_free_string().
 */
LanceDBError lancedb_blob_file_size(
    const LanceDBBlobFile* file,
    uint64_t* size_out,
    char** error_message
);

/**
 * Return the physical byte offset of the blob inside its data file
 *
 * @param file - blob file handle returned by lancedb_table_fetch_blob_files or
 *               lancedb_table_open_blob_file
 * @param position_out - pointer to receive the position
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return Error code indicating success or failure
 *
 * Embedded handles set has_position; remote handles do not expose their storage
 * layout and report it as zero with position zeroed. This is not the read
 * cursor: seek and read move the cursor, while read_range leaves it unchanged.
 * On entry, the error message is cleared.
 * On failure position_out is left unchanged. If error_message is provided and an
 * error occurs, the caller must free the error message with
 * lancedb_free_string().
 */
LanceDBError lancedb_blob_file_position(
    const LanceDBBlobFile* file,
    LanceDBBlobFilePosition* position_out,
    char** error_message
);

/**
 * Move the read cursor to a byte offset inside the blob
 *
 * @param file - blob file handle returned by lancedb_table_fetch_blob_files or
 *               lancedb_table_open_blob_file
 * @param position - new byte offset from the beginning of the blob
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return Error code indicating success or failure
 *
 * The SDK accepts an offset past the blob end; a later read then returns no
 * bytes. Seeking a closed handle fails with LANCEDB_INVALID_INPUT. On entry, the
 * error message is cleared. If error_message is provided and an error occurs,
 * the caller must free the error message with lancedb_free_string().
 */
LanceDBError lancedb_blob_file_seek(
    const LanceDBBlobFile* file,
    uint64_t position,
    char** error_message
);

/**
 * Read from the current cursor to the end of the blob
 *
 * @param file - blob file handle returned by lancedb_table_fetch_blob_files or
 *               lancedb_table_open_blob_file
 * @param buffer_out - pointer to receive the owned bytes
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return Error code indicating success or failure
 *
 * A successful read advances the cursor to the blob size. Reading at or past the
 * end succeeds with an empty buffer. Reading a closed handle fails with
 * LANCEDB_INVALID_INPUT. On entry, the buffer and error message are cleared. On
 * success the caller owns the bytes and frees them with
 * lancedb_blob_buffer_free(); they stay valid after the handle and table are
 * freed. On failure the buffer stays clear. If error_message is provided and an
 * error occurs, the caller must free the error message with
 * lancedb_free_string().
 */
LanceDBError lancedb_blob_file_read(
    const LanceDBBlobFile* file,
    LanceDBBlobBuffer* buffer_out,
    char** error_message
);

/**
 * Read a blob-local byte range without moving the cursor
 *
 * @param file - blob file handle returned by lancedb_table_fetch_blob_files or
 *               lancedb_table_open_blob_file
 * @param offset - byte offset from the beginning of the blob value
 * @param length - number of bytes to read
 * @param buffer_out - pointer to receive the owned bytes
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return Error code indicating success or failure
 *
 * Empty ranges are allowed at or before the blob size. An end past the blob
 * size, or an offset and length whose sum overflows uint64_t, is rejected.
 * Reading a closed handle fails with LANCEDB_INVALID_INPUT. On entry, the buffer
 * and error message are cleared. On success the caller owns the bytes and frees
 * them with lancedb_blob_buffer_free(); they stay valid after the handle and
 * table are freed. On failure the buffer stays clear. If error_message is
 * provided and an error occurs, the caller must free the error message with
 * lancedb_free_string().
 */
LanceDBError lancedb_blob_file_read_range(
    const LanceDBBlobFile* file,
    uint64_t offset,
    uint64_t length,
    LanceDBBlobBuffer* buffer_out,
    char** error_message
);

/**
 * Free the bytes of one blob read and reset the buffer
 *
 * @param buffer - buffer returned by lancedb_blob_file_read or
 *                 lancedb_blob_file_read_range, or NULL
 *
 * Freeing an already freed or empty buffer is a no-op. The caller keeps the
 * struct until the call; both fields are zeroed.
 */
void lancedb_blob_buffer_free(LanceDBBlobBuffer* buffer);

/**
 * Close the blob file and release its resources
 *
 * @param file - blob file handle returned by lancedb_table_fetch_blob_files or
 *               lancedb_table_open_blob_file
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return Error code indicating success or failure
 *
 * Closing is idempotent. After a close, reads, range reads and seeks fail with
 * LANCEDB_INVALID_INPUT, while size and position remain available. Reads already
 * in flight may still complete. On entry, the error message is cleared. If
 * error_message is provided and an error occurs, the caller must free the error
 * message with lancedb_free_string().
 */
LanceDBError lancedb_blob_file_close(
    const LanceDBBlobFile* file,
    char** error_message
);

/** @} */

/**
 * @addtogroup api_table
 * @{
 */

/**
 * Get table version
 *
 * @param table - pointer to LanceDBTable
 * @return Table version number on success, 0 on failure
 *
 * A version is never 0 for a valid table, so 0 is a safe failure sentinel. Use
 * lancedb_table_version_with_status() when the failure details are needed.
 */
unsigned long long lancedb_table_version(const LanceDBTable* table);

/**
 * Get table version, preserving failures
 *
 * @param table - pointer to LanceDBTable
 * @param version_out - pointer to receive the table version
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return Error code indicating success or failure
 *
 * On success, version_out is set to the table version. On failure, version_out
 * is left unchanged, so a caller can keep its own sentinel there.
 * If error_message is provided and an error occurs, the caller must free
 * the error message with lancedb_free_string().
 */
LanceDBError lancedb_table_version_with_status(
    const LanceDBTable* table,
    uint64_t* version_out,
    char** error_message
);

/**
 * Count rows in table
 *
 * @param table - pointer to LanceDBTable
 * @return Number of rows in table on success, 0 on failure (or empty table)
 *
 * Use lancedb_table_count_rows_with_status() when an empty table must be told
 * apart from a failed call.
 */
unsigned long long lancedb_table_count_rows(const LanceDBTable* table);

/**
 * Count rows in table, preserving failures
 *
 * @param table - pointer to LanceDBTable
 * @param count_out - pointer to receive the row count
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return Error code indicating success or failure
 *
 * On success, count_out is set to the number of rows, which is 0 for an empty
 * table. On failure, count_out is left unchanged.
 * If error_message is provided and an error occurs, the caller must free
 * the error message with lancedb_free_string().
 */
LanceDBError lancedb_table_count_rows_with_status(
    const LanceDBTable* table,
    size_t* count_out,
    char** error_message
);

/**
 * Count rows in table matching an optional SQL filter, preserving failures
 *
 * @param table - pointer to LanceDBTable
 * @param filter - optional SQL predicate; NULL counts every row. A non-NULL
 *                 string, including an empty one, is passed to the SDK unchanged
 * @param count_out - pointer to receive the matching row count
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return Error code indicating success or failure
 *
 * On success, count_out is set to the number of matching rows, which is 0 when
 * none match. On failure, count_out is left unchanged.
 * If error_message is provided and an error occurs, the caller must free
 * the error message with lancedb_free_string().
 */
LanceDBError lancedb_table_count_rows_filtered(
    const LanceDBTable* table,
    const char* filter,
    size_t* count_out,
    char** error_message
);

/**
 * Statistics on the number of rows in table fragments
 */
typedef struct {
    size_t min;
    size_t max;
    size_t mean;
    size_t p25;
    size_t p50;
    size_t p75;
    size_t p99;
} LanceDBFragmentSummaryStats;

/**
 * Statistics on table fragments
 */
typedef struct {
    size_t num_fragments;                 // Number of fragments in the table
    size_t num_small_fragments;           // Number of fragments with fewer than 100000 rows
    LanceDBFragmentSummaryStats lengths;  // Statistics on the number of rows per fragment
} LanceDBFragmentStats;

/**
 * Table statistics structure
 */
typedef struct {
    size_t total_bytes;                   // Total number of bytes in the table (data and index files)
    size_t num_rows;                      // Number of rows in the table (deleted rows excluded)
    int64_t num_deleted_rows;             // Rows marked as deleted but still on disk (-1 = unknown)
    size_t num_indices;                   // Number of indices in the table
    LanceDBFragmentStats fragment_stats;  // Statistics on table fragments
} LanceDBTableStats;

/**
 * Get table statistics
 *
 * @param table - pointer to LanceDBTable
 * @param stats_out - pointer to receive the table statistics
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return Error code indicating success or failure
 *
 * num_deleted_rows counts rows that were deleted but still occupy space on
 * disk, since deletions are only marked in deletion files until the table is
 * compacted (see lancedb_table_optimize()). A large value indicates that the
 * table should be compacted. Fragments in which every row was deleted are
 * dropped outright, so their rows are not counted. The value is -1 when the
 * backend (e.g. a remote table) does not report deletion counts.
 * If error_message is provided and an error occurs, the caller must free
 * the error message with lancedb_free_string().
 */
LanceDBError lancedb_table_stats(
    const LanceDBTable* table,
    LanceDBTableStats* stats_out,
    char** error_message
);

/**
 * Add data to table using Arrow RecordBatchReader
 *
 * @param table - pointer to LanceDBTable
 * @param reader - pointer to LanceDBRecordBatchReader
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return Error code indicating success or failure
 *
 * A non-NULL reader is consumed after the table and reader pointers are checked
 * for NULL, so a NULL table or reader leaves the reader caller-owned. Do NOT
 * call lancedb_record_batch_reader_free() after the reader is consumed. This
 * entry point always appends and discards the SDK commit result; use
 * lancedb_table_add_with_options() to choose a mode or read that version.
 * If error_message is provided and an error occurs, the caller must free the
 * error message with lancedb_free_string().
 */
LanceDBError lancedb_table_add(
    const LanceDBTable* table,
    LanceDBRecordBatchReader* reader,
    char** error_message
);

/**
 * Add-data modes for lancedb_table_add_with_options()
 *
 * Append keeps the existing rows. Overwrite replaces the table contents with
 * the supplied data.
 */
typedef enum {
    LANCEDB_ADD_DATA_MODE_APPEND = 0,
    LANCEDB_ADD_DATA_MODE_OVERWRITE = 1
} LanceDBAddDataMode;

/**
 * How an add treats NaN and null elements in vector columns
 *
 * Error is the SDK default and rejects such vectors. Keep stores them, but they
 * are not indexed for search. Whole null vectors are allowed by both modes.
 */
typedef enum {
    LANCEDB_NAN_VECTOR_BEHAVIOR_ERROR = 0,
    LANCEDB_NAN_VECTOR_BEHAVIOR_KEEP = 1
} LanceDBNaNVectorBehavior;

/**
 * Fields of LanceDBAddOptions
 *
 * Every bit makes the matching field an override. A field whose bit is not set
 * keeps the LanceDB SDK default, so an omitted retry or write parallelism stays
 * distinct from an explicit zero.
 */
typedef enum {
    LANCEDB_ADD_OPTIONS_RETRIES = 1 << 0,
    LANCEDB_ADD_OPTIONS_WRITE_PARALLELISM = 1 << 1
} LanceDBAddOptionsField;

/**
 * Options for lancedb_table_add_with_options()
 *
 * A zero-initialized struct (all fields zero and pointers NULL) uses the
 * default append mode, the default Error NaN handling, no Lance write
 * parameters, no retry or write-parallelism override, and the external-blob
 * switch disabled. A NULL options pointer is equivalent to that struct.
 * set_fields only controls retries and write_parallelism; mode, on_nan_vectors,
 * write_options and allow_external_blob_outside_bases are read from every
 * non-NULL struct.
 *
 * mode is one of the LanceDBAddDataMode values. The binding applies that one
 * mode to both the SDK add builder and any supplied Lance write parameters,
 * because the embedded SDK otherwise uses the write parameters' own mode. This
 * keeps a caller that also requests a layout override on the intended
 * append/overwrite path.
 *
 * write_options is NULL for the SDK defaults or points to a readable
 * LanceDBWriteOptions. The SDK decides which options a backend supports. Remote
 * add rejects non-default layout sizes; its insert request carries only the
 * mode, so data storage version and stable-row-ID overrides have no effect
 * there. allow_external_blob_outside_bases is embedded only and must be 0 or 1;
 * a remote add with it set to 1 returns LANCEDB_NOT_SUPPORTED.
 *
 * retries is a remote per-call failed-attempt threshold and is only read when
 * its bit is set. A value of 0 disables retries explicitly, while an unset bit
 * keeps the SDK default. Embedded tables ignore it, and ambiguous write results
 * are never replayed.
 *
 * on_nan_vectors is one of the LanceDBNaNVectorBehavior values; zero selects the
 * SDK default Error behavior.
 *
 * write_parallelism is the number of parallel write streams and is only read
 * when its bit is set. An unset bit lets the SDK estimate it. Enterprise servers
 * older than 0.4.0 always use the single-request add path, and empty input takes
 * that path even on multipart-capable servers. An explicit zero is rejected by
 * the SDK's input validation before backend dispatch and returns
 * LANCEDB_INVALID_INPUT.
 *
 * The struct and its write_options are borrowed for the duration of the call.
 */
typedef struct {
    uint32_t set_fields;                       // Bitmask of LanceDBAddOptionsField values (0 = no retry or parallelism override)
    int mode;                                  // One of the LanceDBAddDataMode values
    const LanceDBWriteOptions* write_options;  // Per-call write options (NULL for SDK defaults)
    uint8_t retries;                           // Failed-attempt threshold when its bit is set
    int on_nan_vectors;                        // One of the LanceDBNaNVectorBehavior values
    size_t write_parallelism;                  // Parallel write streams when its bit is set
    int allow_external_blob_outside_bases;     // Allow external blob URIs (1 = true, 0 = false)
} LanceDBAddOptions;

/**
 * Result of an add operation
 *
 * version is copied unchanged from the SDK's AddResult; this layer does not
 * recompute or synthesize it. A value of zero means a legacy remote server did
 * not return a commit version.
 */
typedef struct {
    uint64_t version;  // Commit version; 0 may mean the server did not report one
} LanceDBAddResult;

/**
 * Add data to table with options and an optional SDK commit result
 *
 * @param table - pointer to LanceDBTable
 * @param reader - pointer to LanceDBRecordBatchReader; it is consumed on every path
 * @param options - pointer to LanceDBAddOptions, or NULL for the SDK defaults
 * @param result_out - optional pointer to receive the SDK commit version; NULL discards it
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return Error code indicating success or failure
 *
 * A non-NULL reader is consumed before the table and options are validated, so
 * every rejection after the NULL reader check frees it exactly once. A NULL
 * reader returns without consuming anything. Do NOT call
 * lancedb_record_batch_reader_free() after the reader is consumed.
 *
 * When result_out is non-NULL, it is written only after a successful commit and
 * is left unchanged on failure. The value is the SDK's AddResult.version; zero
 * is valid when a legacy remote server does not report a commit version.
 * If error_message is provided and an error occurs, the caller must free the
 * error message with lancedb_free_string().
 */
LanceDBError lancedb_table_add_with_options(
    const LanceDBTable* table,
    LanceDBRecordBatchReader* reader,
    const LanceDBAddOptions* options,
    LanceDBAddResult* result_out,
    char** error_message
);

/**
 * Merge insert data into table (upsert operation)
 *
 * @param table - pointer to LanceDBTable
 * @param data - pointer to LanceDBRecordBatchReader containing data to merge
 * @param on_columns - array of column names to join on (typically key/id columns)
 * @param num_columns - number of columns in the on_columns array
 * @param config - pointer to LanceDBMergeInsertConfig for operation behavior (NULL for defaults)
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return Error code indicating success or failure
 *
 * This function performs an upsert operation by joining on the specified columns.
 * New records are inserted, existing records can be updated based on configuration.
 * A non-NULL reader is consumed before the table, columns and config are
 * validated, so it is freed even when those arguments are rejected. A NULL data
 * pointer returns without consuming a reader. Do NOT call
 * lancedb_record_batch_reader_free() after the reader is consumed. The
 * expression in the config is not consumed and must still be freed by the
 * caller with lancedb_expr_free().
 * If error_message is provided and an error occurs, the caller must free
 * the error message with lancedb_free_string().
 */
LanceDBError lancedb_table_merge_insert(
    const LanceDBTable* table,
    LanceDBRecordBatchReader* data,
    const char* const* on_columns,
    size_t num_columns,
    const LanceDBMergeInsertConfig* config,
    char** error_message
);

/**
 * Result of a merge insert operation
 *
 * Every field is copied unchanged from the SDK's MergeResult; this layer does not
 * fill in or synthesize a value. The SDK defaults each field missing from a
 * remote response to zero, so zero means "not reported" rather than necessarily
 * no work. num_attempts is the merge attempt count reported by the SDK or server,
 * not the number of HTTP requests. A legacy remote empty response, or a merge
 * through the RTE write path, can succeed with all five fields zero. A version of
 * 0 likewise means the backend did not report a commit version, not necessarily
 * that no commit happened.
 */
typedef struct {
    uint64_t version;           // Reported commit version; 0 may mean not reported
    uint64_t num_inserted_rows; // Reported inserted rows; 0 may mean not reported
    uint64_t num_updated_rows;  // Reported updated rows; 0 may mean not reported
    uint64_t num_deleted_rows;  // Reported deleted rows; 0 may mean not reported
    uint32_t num_attempts;      // Reported merge attempts; 0 may mean not reported
} LanceDBMergeInsertResult;

/**
 * Merge insert with explicit actions and an SDK result
 *
 * This entry point performs one SDK merge. The reader supplies both the source
 * schema and every source batch, so several batches are merged in a single
 * operation. A reader over a schema-only Arrow IPC file is an empty source;
 * combined with when_not_matched_by_source_delete, it can delete unmatched
 * target rows.
 *
 * @param table - pointer to LanceDBTable
 * @param data - pointer to LanceDBRecordBatchReader with the source schema and batches
 * @param on_columns - array of column names to join on (typically key/id columns)
 * @param num_columns - number of columns in the on_columns array (must be > 0)
 * @param options - pointer to LanceDBMergeInsertOptions, or NULL for the default upsert
 * @param result_out - pointer to receive the SDK commit version and counters
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return Error code indicating success or failure
 *
 * The function takes ownership of the reader and frees it automatically on every
 * path, including rejected arguments. Do NOT call
 * lancedb_record_batch_reader_free() after calling this function. The options,
 * their strings and on_columns are borrowed and remain owned by the caller.
 *
 * On entry, *result_out and the error message are cleared. The result is only
 * meaningful when the function returns LANCEDB_SUCCESS; on failure it stays zero.
 * If error_message is provided and an error occurs, the caller must free the
 * error message with lancedb_free_string().
 */
LanceDBError lancedb_table_merge_insert_with_result(
    const LanceDBTable* table,
    LanceDBRecordBatchReader* data,
    const char* const* on_columns,
    size_t num_columns,
    const LanceDBMergeInsertOptions* options,
    LanceDBMergeInsertResult* result_out,
    char** error_message
);

/**
 * Result of lancedb_table_merge_insert_with_options
 *
 * This is the SDK's complete MergeResult, copied unchanged. It is a separate
 * struct so LanceDBMergeInsertResult keeps its published layout. The SDK
 * defaults each field missing from a remote response to zero, so zero means
 * "not reported" rather than necessarily no work. num_attempts is the merge
 * attempt count reported by the SDK or server, not the number of HTTP requests.
 * num_rows is the SDK's total written-row count: on the standard path it equals
 * num_inserted_rows + num_updated_rows, while the RTE write path can report only
 * num_rows and leave the breakdown zero. A legacy remote empty response can
 * succeed with every field zero, and a version of 0 means the backend did not
 * report a commit version.
 */
typedef struct {
    uint64_t version;           // Reported commit version; 0 may mean not reported
    uint64_t num_inserted_rows; // Reported inserted rows; 0 may mean not reported
    uint64_t num_updated_rows;  // Reported updated rows; 0 may mean not reported
    uint64_t num_deleted_rows;  // Reported deleted rows; 0 may mean not reported
    uint32_t num_attempts;      // Reported merge attempts; 0 may mean not reported
    uint64_t num_rows;          // Reported total rows written; 0 may mean not reported
} LanceDBMergeInsertResultV2;

/**
 * Merge insert with the complete option set and an optional result
 *
 * This entry point performs one SDK merge and reuses LanceDBMergeInsertOptions
 * for its actions, SQL conditions, timeout and index choice. A NULL options
 * pointer keeps the same default upsert as lancedb_table_merge_insert(): update
 * every matched row and insert every unmatched source row.
 *
 * @param table - pointer to LanceDBTable
 * @param data - pointer to LanceDBRecordBatchReader with the source schema and batches
 * @param on_columns - array of column names to join on (typically key/id columns)
 * @param num_columns - number of columns in the on_columns array (must be > 0)
 * @param options - pointer to LanceDBMergeInsertOptions, or NULL for the default upsert
 * @param retries - optional pointer to a uint8_t retry budget, or NULL for the SDK default
 * @param when_not_matched_by_source_delete_expr - optional borrowed DataFusion expression,
 *        or NULL. Only used when the by-source-delete action is enabled
 * @param result_out - optional pointer to receive the complete SDK result
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return Error code indicating success or failure
 *
 * The function takes ownership of the reader and frees it automatically on every
 * path, including rejected arguments. Do NOT call
 * lancedb_record_batch_reader_free() after calling this function. The options,
 * their strings, retries and the expression are borrowed and remain owned by the
 * caller; free the expression with lancedb_expr_free().
 *
 * retries overrides the SDK's remote transport retry budget for this call. A
 * pointed-to 0 disables retries, while a NULL pointer keeps the SDK default.
 * This budget is independent of local transaction-conflict retries, and the SDK
 * never replays an ambiguous write result. Embedded tables ignore it.
 *
 * The by-source-delete expression is evaluated against unmatched target rows and
 * uses target column references directly, like the SQL filter in
 * LanceDBMergeInsertOptions. When that action is enabled, the expression and a
 * non-NULL SQL filter are mutually exclusive; setting both fails with
 * LANCEDB_INVALID_ARGUMENT. A condition set for a disabled action is ignored.
 *
 * On entry, a non-NULL *result_out and the error message are cleared. The result
 * is only meaningful when the function returns LANCEDB_SUCCESS; on failure it
 * stays zero. If error_message is provided and an error occurs, the caller must
 * free the error message with lancedb_free_string().
 */
LanceDBError lancedb_table_merge_insert_with_options(
    const LanceDBTable* table,
    LanceDBRecordBatchReader* data,
    const char* const* on_columns,
    size_t num_columns,
    const LanceDBMergeInsertOptions* options,
    const uint8_t* retries,
    const LanceDBExpr* when_not_matched_by_source_delete_expr,
    LanceDBMergeInsertResultV2* result_out,
    char** error_message
);

/**
 * Update option fields of LanceDBUpdateOptions
 *
 * Every bit makes the matching field an override. A field whose bit is not set
 * keeps the LanceDB default, so an omitted retry stays distinct from an explicit
 * zero retry budget.
 */
typedef enum {
    LANCEDB_UPDATE_RETRIES = 1 << 0
} LanceDBUpdateField;

/**
 * Options for lancedb_table_update_with_options
 *
 * set_fields selects the fields that are read; every other field keeps the SDK
 * default. An unknown set_fields bit is rejected with LANCEDB_INVALID_ARGUMENT
 * before the update starts.
 *
 * retries is a failed-attempt threshold, not a replay count. For a sequence of
 * retryable HTTP status responses, the SDK increments its request-failure
 * counter before testing it against the budget: with repeated 409 responses, 1
 * returns the first failure without resending, while 3 can resend twice and
 * returns the third failure. Embedded tables ignore it. Leave the
 * LANCEDB_UPDATE_RETRIES bit unset to keep the SDK default; set it with 0 to
 * disable retries explicitly. Ambiguous write results are never replayed.
 *
 * The options struct is borrowed for the duration of the call and is not consumed.
 */
typedef struct {
    uint32_t set_fields;  // Bitmask of LanceDBUpdateField values (0 = defaults)
    uint8_t retries;      // Failed-attempt threshold; 0 disables retries
} LanceDBUpdateOptions;

/**
 * Result of an update operation
 *
 * Embedded tables report the actual affected rows and commit version. A
 * legacy remote server that returns an empty update response can report zero
 * for either field, which means unknown there rather than a literal zero.
 */
typedef struct {
    uint64_t rows_updated;   // Rows changed; 0 may mean unknown on legacy remote responses
    uint64_t version;        // Committed table version; 0 may mean unknown on legacy remote responses
} LanceDBUpdateResult;

/**
 * Update rows in a table using SQL expressions
 *
 * Each entry in columns is paired with the entry at the same index in
 * expressions. The input order is preserved when passing the assignments to
 * the SDK, but it is not an evaluation order: every expression sees the row as
 * it was before the update. For example, updating a from b and b from a swaps
 * their original values. Assignments are applied in a single commit.
 * The caller is responsible for quoting string literals in expressions (for
 * example "'updated'").
 *
 * @param table - pointer to LanceDBTable
 * @param columns - array of column names to update
 * @param expressions - array of SQL expressions, one per column
 * @param num_assignments - number of entries in columns and expressions (must be > 0)
 * @param predicate - SQL predicate selecting the rows to update; NULL or an empty
 *                    string updates every row
 * @param result_out - pointer to receive the affected rows and committed version;
 *                     legacy remote empty responses can report zero (unknown)
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return Error code indicating success or failure
 *
 * The arrays and their strings are borrowed for the duration of the call. They
 * are not consumed and remain owned by the caller.
 * If error_message is provided and an error occurs, the caller must free
 * the error message with lancedb_free_string().
 */
LanceDBError lancedb_table_update(
    const LanceDBTable* table,
    const char* const* columns,
    const char* const* expressions,
    size_t num_assignments,
    const char* predicate,
    LanceDBUpdateResult* result_out,
    char** error_message
);

/**
 * Update rows in a table using SQL expressions, with options and an optional result
 *
 * Assignments are paired and evaluated exactly like lancedb_table_update(). The
 * options are borrowed for the call and are not consumed. A NULL options pointer
 * uses the SDK defaults. result_out may be NULL when the caller does not need the
 * affected rows or commit version; when non-NULL, it is written only after a
 * successful commit and is left unchanged on failure.
 *
 * @param table - pointer to LanceDBTable
 * @param columns - array of column names to update
 * @param expressions - array of SQL expressions, one per column
 * @param num_assignments - number of entries in columns and expressions (must be > 0)
 * @param predicate - SQL predicate selecting the rows to update; NULL or an empty
 *                    string updates every row
 * @param options - pointer to LanceDBUpdateOptions, or NULL for the SDK defaults
 * @param result_out - optional pointer to receive the affected rows and committed
 *                     version; NULL discards the result
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return Error code indicating success or failure
 *
 * If error_message is provided and an error occurs, the caller must free
 * the error message with lancedb_free_string().
 */
LanceDBError lancedb_table_update_with_options(
    const LanceDBTable* table,
    const char* const* columns,
    const char* const* expressions,
    size_t num_assignments,
    const char* predicate,
    const LanceDBUpdateOptions* options,
    LanceDBUpdateResult* result_out,
    char** error_message
);

/**
 * Result of a delete operation
 *
 * Every field is copied unchanged from the SDK's DeleteResult; this layer does
 * not fill in or synthesize a value. The SDK defaults each field missing from a
 * remote response to zero independently: a version-only response leaves
 * num_deleted_rows at zero, and a count-only response leaves version at zero.
 * Zero can mean "not reported"; a reported num_deleted_rows of zero is also
 * valid when no rows matched. An empty legacy response leaves both fields zero.
 */
typedef struct {
    uint64_t num_deleted_rows;  // Rows deleted; 0 may mean not reported
    uint64_t version;           // Commit version; 0 may mean not reported
} LanceDBDeleteResult;

/**
 * Delete rows from table based on predicate
 *
 * @param table - pointer to LanceDBTable
 * @param predicate - null-terminated C string containing SQL WHERE clause
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return Error code indicating success or failure
 *
 * If error_message is provided and an error occurs, the caller must free
 * the error message with lancedb_free_string().
 */
LanceDBError lancedb_table_delete(
    const LanceDBTable* table,
    const char* predicate,
    char** error_message
);

/**
 * Delete rows from table based on predicate, with an optional SDK result
 *
 * The predicate and error reporting follow lancedb_table_delete(). When result_out
 * is non-NULL it receives the number of deleted rows and the commit version after
 * a successful delete; it is left unchanged on failure. NULL discards the result.
 *
 * @param table - pointer to LanceDBTable
 * @param predicate - null-terminated C string containing SQL WHERE clause
 * @param result_out - optional pointer to receive the deleted rows and committed version
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return Error code indicating success or failure
 *
 * If error_message is provided and an error occurs, the caller must free
 * the error message with lancedb_free_string().
 */
LanceDBError lancedb_table_delete_with_result(
    const LanceDBTable* table,
    const char* predicate,
    LanceDBDeleteResult* result_out,
    char** error_message
);

/**
 * Delete rows from table using a DataFusion expression
 *
 * @param table - pointer to LanceDBTable
 * @param expr - pointer to LanceDBExpr (consumed by this function; do not use or free after calling)
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return Error code indicating success or failure
 *
 * If error_message is provided and an error occurs, the caller must free
 * the error message with lancedb_free_string().
 */
LanceDBError lancedb_table_df_delete(
    const LanceDBTable* table,
    LanceDBExpr* expr,
    char** error_message
);

/**
 * Delete rows from table using a DataFusion expression, with an optional SDK result
 *
 * The expression is consumed on every path, exactly like lancedb_table_df_delete().
 * When result_out is non-NULL it receives the number of deleted rows and the commit
 * version after a successful delete; it is left unchanged on failure. NULL discards
 * the result.
 *
 * @param table - pointer to LanceDBTable
 * @param expr - pointer to LanceDBExpr (consumed by this function; do not use or free after calling)
 * @param result_out - optional pointer to receive the deleted rows and committed version
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return Error code indicating success or failure
 *
 * If error_message is provided and an error occurs, the caller must free
 * the error message with lancedb_free_string().
 */
LanceDBError lancedb_table_df_delete_with_result(
    const LanceDBTable* table,
    LanceDBExpr* expr,
    LanceDBDeleteResult* result_out,
    char** error_message
);

/**
 * One column alteration for lancedb_table_alter_columns
 *
 * `path` names the existing column. `rename`, when non-NULL, sets the new
 * column name. `set_nullable` distinguishes "leave nullability unchanged"
 * (0) from applying `nullable` (1), so an explicit nullable=false request is
 * not mistaken for an omitted one. `nullable` is 0 for NOT NULL and non-zero
 * for NULL.
 */
typedef struct {
    const char* path;
    const char* rename;
    int set_nullable;
    int nullable;
} LanceDBColumnAlteration;

/**
 * One column alteration for lancedb_table_alter_columns_with_cast
 *
 * The first four fields have the same meaning as LanceDBColumnAlteration.
 * `cast_to`, when non-NULL, points to a borrowed Arrow C ABI schema that holds
 * exactly one field. Only that field's data type is used; its name, nullability
 * and metadata are ignored. The caller keeps ownership of the schema and must
 * still release it with the Arrow C ABI release callback.
 */
typedef struct {
    const char* path;
    const char* rename;
    int set_nullable;
    int nullable;
    const FFI_ArrowSchema* cast_to;
} LanceDBColumnAlterationWithCast;

/**
 * Add new columns using SQL expressions
 *
 * @param table - pointer to LanceDBTable
 * @param names - array of new column names
 * @param expressions - array of SQL expressions, one per new column
 * @param num_columns - number of entries in names and expressions (must be > 0)
 * @param version_out - pointer to receive the commit version returned by the SDK
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return Error code indicating success or failure
 *
 * The arrays and their strings are borrowed for the duration of the call. They
 * are not consumed and remain owned by the caller. Names and expressions must
 * be non-empty. On success, version_out is the operation's own commit version;
 * it may be 0 for legacy remote servers that do not report a version. On
 * failure, version_out is left unchanged. No separate table version lookup is
 * performed.
 * If error_message is provided and an error occurs, the caller must free
 * the error message with lancedb_free_string().
 */
LanceDBError lancedb_table_add_columns(
    const LanceDBTable* table,
    const char* const* names,
    const char* const* expressions,
    size_t num_columns,
    uint64_t* version_out,
    char** error_message
);

/**
 * Add typed columns whose existing rows read as NULL
 *
 * @param table - pointer to LanceDBTable
 * @param schema - pointer to a borrowed Arrow C ABI schema with at least one field
 * @param version_out - pointer to receive the commit version returned by the SDK
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return Error code indicating success or failure
 *
 * This binds NewColumnTransform::AllNulls: every field of `schema` is added as
 * a new column and every existing row reads NULL for it. Each field must be
 * nullable, as the SDK requires, and its data type is added unchanged. Field
 * names, types, and field metadata are taken from the Arrow schema.
 * The schema is only read for the duration of the call and remains owned by the
 * caller; it is not consumed. On success, version_out is the operation's own
 * commit version; it may be 0 for legacy remote servers that do not report a
 * version. On failure, version_out is left unchanged. No separate table version
 * lookup is performed.
 * If error_message is provided and an error occurs, the caller must free
 * the error message with lancedb_free_string().
 */
LanceDBError lancedb_table_add_columns_all_nulls(
    const LanceDBTable* table,
    const FFI_ArrowSchema* schema,
    uint64_t* version_out,
    char** error_message
);

/**
 * Alter column names and nullability
 *
 * @param table - pointer to LanceDBTable
 * @param alterations - array of column alterations
 * @param num_alterations - number of entries in alterations (must be > 0)
 * @param version_out - pointer to receive the commit version returned by the SDK
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return Error code indicating success or failure
 *
 * Each entry must set a non-empty path and change at least the name or the
 * nullability. Rename and nullability are independent: a NULL rename keeps the
 * current name, and set_nullable=0 keeps the current nullability. The array and
 * its strings are borrowed for the duration of the call.
 * On success, version_out is the operation's own commit version; it may be 0
 * for legacy remote servers that do not report a version. On failure,
 * version_out is left unchanged. No separate table version lookup is performed.
 * If error_message is provided and an error occurs, the caller must free
 * the error message with lancedb_free_string().
 */
LanceDBError lancedb_table_alter_columns(
    const LanceDBTable* table,
    const LanceDBColumnAlteration* alterations,
    size_t num_alterations,
    uint64_t* version_out,
    char** error_message
);

/**
 * Alter column names, nullability, and data types
 *
 * @param table - pointer to LanceDBTable
 * @param alterations - array of column alterations that may cast
 * @param num_alterations - number of entries in alterations (must be > 0)
 * @param version_out - pointer to receive the commit version returned by the SDK
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return Error code indicating success or failure
 *
 * This is the same operation as lancedb_table_alter_columns, with an optional
 * cast_to data type per entry. Each entry must set a non-empty path and change
 * at least the name, the nullability, or the data type. Rename, nullability,
 * and cast are independent and can be combined in one commit. A non-NULL
 * cast_to must point to an Arrow C ABI schema with exactly one field; only its
 * data type is used, and the schema remains owned by the caller. A NULL cast_to
 * leaves the type unchanged.
 * The array, its strings, and its cast schemas are borrowed for the duration of
 * the call. On success, version_out is the operation's own commit version; it
 * may be 0 for legacy remote servers that do not report a version. On failure,
 * version_out is left unchanged. No separate table version lookup is performed.
 *
 * The pinned embedded Lance backend rejects changing a currently nullable field
 * to NOT NULL in the same call as a cast, even when the field has no NULL
 * values. The SDK error is returned unchanged; apply the cast and the
 * nullability change in separate calls. Renaming a field, or relaxing a NOT
 * NULL field to NULL, can be combined with a cast.
 * If error_message is provided and an error occurs, the caller must free
 * the error message with lancedb_free_string().
 */
LanceDBError lancedb_table_alter_columns_with_cast(
    const LanceDBTable* table,
    const LanceDBColumnAlterationWithCast* alterations,
    size_t num_alterations,
    uint64_t* version_out,
    char** error_message
);

/**
 * Drop columns from a table
 *
 * @param table - pointer to LanceDBTable
 * @param columns - array of column names to drop
 * @param num_columns - number of entries in columns (must be > 0)
 * @param version_out - pointer to receive the commit version returned by the SDK
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return Error code indicating success or failure
 *
 * Names must be non-empty. The array and its strings are borrowed for the
 * duration of the call. On success, version_out is the operation's own commit
 * version; it may be 0 for legacy remote servers that do not report a version.
 * On failure, version_out is left unchanged. No separate table version lookup
 * is performed.
 * If error_message is provided and an error occurs, the caller must free
 * the error message with lancedb_free_string().
 */
LanceDBError lancedb_table_drop_columns(
    const LanceDBTable* table,
    const char* const* columns,
    size_t num_columns,
    uint64_t* version_out,
    char** error_message
);

/**
 * One field metadata key change for lancedb_table_update_field_metadata
 *
 * `key` must be a non-empty string. A non-NULL `value` sets the key to that
 * value; a NULL value removes the key. Keys must be unique within one update.
 * The strings are borrowed for the duration of the call.
 */
typedef struct {
    const char* key;
    const char* value;
} LanceDBFieldMetadataEntry;

/**
 * One field metadata update for lancedb_table_update_field_metadata
 *
 * `path` addresses the field by its dot-separated path, for example `zip` or
 * `address.zip`. `entries` holds `num_entries` key changes applied in order and
 * must be non-NULL when num_entries is greater than zero. When `replace` is 0,
 * the entries merge into the field's existing metadata; a merge with zero
 * entries is rejected because it changes nothing. When `replace` is non-zero,
 * the entries replace the field's entire metadata map, so a replace with zero
 * entries clears the metadata.
 */
typedef struct {
    const char* path;
    const LanceDBFieldMetadataEntry* entries;
    size_t num_entries;
    int replace;
} LanceDBFieldMetadataUpdate;

/**
 * Update per-field metadata through the SDK
 *
 * @param table - pointer to LanceDBTable
 * @param updates - array of field metadata updates
 * @param num_updates - number of entries in updates (must be > 0)
 * @param version_out - pointer to receive the commit version returned by the SDK
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return Error code indicating success or failure
 *
 * This binds lancedb::table::Table::update_field_metadata and works on embedded
 * and remote tables. Each update is merged into its field's existing metadata
 * unless replace is set, in which case it swaps the field's whole metadata map.
 * The array, its entries, and their strings are borrowed for the duration of the
 * call. On success, version_out is the operation's own commit version; it may be
 * 0 for legacy remote servers that do not report a version. On failure,
 * version_out is left unchanged. No separate table version lookup is performed.
 * If error_message is provided and an error occurs, the caller must free
 * the error message with lancedb_free_string().
 */
LanceDBError lancedb_table_update_field_metadata(
    const LanceDBTable* table,
    const LanceDBFieldMetadataUpdate* updates,
    size_t num_updates,
    uint64_t* version_out,
    char** error_message
);

/** @} */

/**
 * @defgroup api_query Queries
 * @brief Build, inspect, and execute vector, scalar, and take queries.
 * @{
 */

/**
 * One column ordering for lancedb_query_order_by() and
 * lancedb_vector_query_order_by()
 *
 * `column` is a NUL-terminated UTF-8 column name. `ascending` is non-zero for
 * ascending order and 0 for descending. `nulls_first` is non-zero to place NULL
 * values first, regardless of direction. Entries are applied in array order, so
 * the first entry is the primary sort key. The ordering setters validate the
 * array and UTF-8 but do not check that a named column exists; the SDK checks
 * column names when it plans the query and reports an unknown column from
 * execution or inspection.
 */
typedef struct {
    const char* column;
    int ascending;
    int nulls_first;
} LanceDBColumnOrdering;

/**
 * Create a new query for the given table
 *
 * @param table - pointer to LanceDBTable
 * @return Pointer to LanceDBQuery on success, NULL on failure
 *         Caller must free with lancedb_query_free()
 */
LanceDBQuery* lancedb_query_new(const LanceDBTable* table);

/**
 * Create a vector query from table with query vector
 *
 * @param table - pointer to LanceDBTable
 * @param vector - array of floats representing the query vector
 * @param dimension - dimension of the vector
 * @return Pointer to LanceDBVectorQuery on success, NULL on failure
 *         Caller must free with lancedb_vector_query_free()
 */
LanceDBVectorQuery* lancedb_vector_query_new(
    const LanceDBTable* table,
    const float* vector,
    size_t dimension
);

/**
 * Create a vector query from one or more typed query vectors
 *
 * `vectors` is borrowed for this call and every input is copied before the
 * function returns. The first vector becomes the nearest-neighbour query and
 * each later vector is added as an SDK batch query vector. `num_vectors` must
 * be at least one.
 *
 * F16, F32 and F64 inputs are passed to the SDK in their declared element
 * type; the SDK converts F16 and F64 components to Float32 before planning. The
 * binary form widens each byte to its numeric Float32 value, so a
 * FixedSizeList<UInt8> column is cast back to UInt8 at planning time. Lance
 * supports only LANCEDB_DISTANCE_HAMMING for such a column. No additional
 * precision is preserved for F64 input.
 *
 * Against a fixed-size-list float vector column, multiple vectors produce one
 * top-K result set per vector and the result adds an Int32 `query_index`
 * column. Against a List<FixedSizeList<...>> multivector column, multiple
 * vectors form one multivector search and no `query_index` column is added. A
 * UInt8 fixed-size vector column keeps one plan per vector and also adds
 * `query_index`; Hamming is the only supported distance.
 *
 * These batch contracts apply to an ordinary vector search, without a
 * full-text query. `limit` and `offset` apply independently to each vector in a
 * batch, and `query_index` is the zero-based position of that vector, not a
 * rank. Rows for different indexes are not required to be grouped or ordered.
 * On a multivector column, `limit` selects table rows in total and each row's
 * `_distance` includes the contribution of every input vector.
 *
 * The pinned SDK beta.30 cannot merge a hybrid query whose vector leg returns
 * batch results with `query_index` and whose full-text leg returns matches:
 * the SDK's RRF merge combines the batch `query_index` column with the
 * full-text schema and returns an Arrow schema error. This includes multiple
 * vectors on a fixed-size-list float column and the per-vector UInt8/Hamming
 * path. This binding forwards the configured SDK query; it does not split the
 * batch, drop `query_index`, or run a C-side fusion. Use one query vector for
 * that hybrid shape until the SDK supports it. A List<FixedSizeList<...>>
 * multivector column adds no `query_index`, so it does not hit that merge
 * shape.
 *
 * A vector-only query is planned locally by the native table, including on a
 * native namespace connection. On a namespace connection with QueryTable
 * pushdown, only an eligible hybrid full-text leg can use the namespace
 * QueryTable path; the vector batch remains a locally planned query. This
 * binding does not choose a different route.
 *
 * For a remote LanceDB, the SDK sends Float32 vectors. Enterprise server 0.2.0
 * and later receive all input vectors in one request; older servers receive
 * one request per vector and the SDK combines the responses with
 * `query_index`. The server decides how it interprets the vector shape and
 * distance and which schema it returns; this binding does not reshape or
 * re-issue the result.
 *
 * A table routed through the pinned MemWAL LSM scanner rejects a single plan
 * containing multiple query vectors, UInt8 vector search, or both vector and
 * full-text search with LANCEDB_NOT_SUPPORTED. Hybrid execution does not build
 * such a plan: it runs separate vector and full-text searches, and can fall
 * back to primary-key fusion when the table rejects `_rowid`. An eligible
 * single-vector hybrid query can therefore execute. This binding exposes no
 * route override.
 *
 * This constructor only copies and checks the description. Like
 * lancedb_vector_query_new(), the SDK reports non-finite values, F64 values
 * that become non-finite after conversion, and mismatched dimensions when the
 * query is executed or its plan or output schema is requested.
 *
 * @param table - pointer to LanceDBTable
 * @param vectors - borrowed array of num_vectors typed vectors
 * @param num_vectors - number of entries; must be at least 1
 * @return Pointer to LanceDBVectorQuery on success, NULL when an argument is
 *         missing, a kind is outside LanceDBQueryVectorType, or a vector has
 *         NULL data or zero dimension.
 *         Caller must free with lancedb_vector_query_free()
 */
LanceDBVectorQuery* lancedb_vector_query_new_typed(
    const LanceDBTable* table,
    const LanceDBQueryVector* vectors,
    size_t num_vectors
);

/**
 * Add a typed query vector to a vector query
 *
 * The vector is borrowed for this call and copied before it returns. On
 * success it is appended to the SDK query as a batch query vector, with the
 * same result schema and validation timing as lancedb_vector_query_new_typed().
 * The builder may come from either constructor. Appending a second or later
 * vector makes the query use the batch shape described there, including its
 * ordinary-vector top-K/query_index contract and its hybrid and MemWAL
 * limitations.
 *
 * A missing query or vector, NULL data, zero dimension, or a kind outside
 * LanceDBQueryVectorType returns LANCEDB_INVALID_ARGUMENT without changing the
 * builder, so it still executes with the vectors it already had.
 *
 * @param query - pointer to LanceDBVectorQuery
 * @param vector - borrowed typed vector
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return Error code indicating success or failure
 *
 * If error_message is provided and an error occurs, the caller must free
 * the error message with lancedb_free_string().
 */
LanceDBError lancedb_vector_query_add_query_vector(
    LanceDBVectorQuery* query,
    const LanceDBQueryVector* vector,
    char** error_message
);

/**
 * Set limit for query
 *
 * This limits the final query result. It is independent of a structured
 * full-text query's candidate ceiling, which is set with
 * lancedb_fts_query_limit().
 *
 * @param query - pointer to LanceDBQuery
 * @param limit - maximum number of results
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return Error code indicating success or failure
 *
 * If error_message is provided and an error occurs, the caller must free
 * the error message with lancedb_free_string().
 */
LanceDBError lancedb_query_limit(
    LanceDBQuery* query,
    size_t limit,
    char** error_message
);

/**
 * Set offset for query
 *
 * @param query - pointer to LanceDBQuery
 * @param offset - number of results to skip
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return Error code indicating success or failure
 *
 * If error_message is provided and an error occurs, the caller must free
 * the error message with lancedb_free_string().
 */
LanceDBError lancedb_query_offset(
    LanceDBQuery* query,
    size_t offset,
    char** error_message
);

/**
 * Set columns to select for query
 *
 * @param query - pointer to LanceDBQuery
 * @param columns - array of column name strings
 * @param num_columns - number of columns
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return Error code indicating success or failure
 *
 * If error_message is provided and an error occurs, the caller must free
 * the error message with lancedb_free_string().
 */
LanceDBError lancedb_query_select(
    LanceDBQuery* query,
    const char* const* columns,
    size_t num_columns,
    char** error_message
);

/**
 * Reset a plain query to every non-system column
 *
 * This clears a projection set by lancedb_query_select() or
 * lancedb_query_select_dynamic(). The most recent projection call wins.
 * lancedb_query_with_row_id() still appends the `_rowid` metadata column.
 *
 * @param query - pointer to LanceDBQuery
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return Error code indicating success or failure
 *
 * If error_message is provided and an error occurs, the caller must free
 * the error message with lancedb_free_string().
 */
LanceDBError lancedb_query_select_all(
    LanceDBQuery* query,
    char** error_message
);

/**
 * Set an expression-based column projection for query
 *
 * Each pair adds one output column: names[i] is the output name and
 * expressions[i] is an SQL expression over the table columns. Columns are
 * returned in the given order, which may differ from the table order. The names
 * and expressions are copied; the arrays may be released after the call.
 *
 * A zero num_columns selects no columns. Both arrays must still be non-NULL.
 * Calling this function replaces a projection from lancedb_query_select() or
 * lancedb_query_select_all(), and a later call to either of those replaces this
 * projection.
 *
 * The SDK evaluates the expressions and decides whether a backend supports
 * dynamic projection. An unsupported backend's SDK error is returned unchanged.
 *
 * @param query - pointer to LanceDBQuery
 * @param names - array of num_columns output column names
 * @param expressions - array of num_columns SQL expressions
 * @param num_columns - number of entries in each array
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return Error code indicating success or failure
 *
 * If error_message is provided and an error occurs, the caller must free
 * the error message with lancedb_free_string().
 */
LanceDBError lancedb_query_select_dynamic(
    LanceDBQuery* query,
    const char* const* names,
    const char* const* expressions,
    size_t num_columns,
    char** error_message
);

/**
 * Set the column ordering for query
 *
 * The first entry is the primary sort key. A nonempty ordering sorts an
 * ordinary scan before limit and offset are applied. On a standalone full-text
 * search it sorts all matches before the search page is selected. Execution,
 * EXPLAIN, output-schema inspection and ANALYZE use the same configured
 * ordering.
 *
 * A zero num_orderings clears the ordering, restoring the query's default
 * order; orderings may be NULL in that case. A positive count requires a
 * non-NULL array whose entries and column names are non-NULL, NUL-terminated
 * UTF-8 strings. Invalid input returns LANCEDB_INVALID_ARGUMENT and leaves the
 * previous ordering unchanged. The array and its strings are borrowed for the
 * duration of the call and are copied into the query.
 *
 * A native namespace connection with QueryTable pushdown executes a nonempty
 * ordering locally, so it needs client access to table storage. A table routed
 * through the MemWAL LSM scanner rejects a nonempty ordering with
 * LANCEDB_NOT_SUPPORTED; clear the ordering to return to the default route. A
 * remote LanceDB forwards the ordering to the server.
 *
 * @param query - pointer to LanceDBQuery
 * @param orderings - array of column orderings, or NULL when num_orderings is 0
 * @param num_orderings - number of entries in orderings; 0 clears the ordering
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return Error code indicating success or failure
 *
 * If error_message is provided and an error occurs, the caller must free
 * the error message with lancedb_free_string().
 */
LanceDBError lancedb_query_order_by(
    LanceDBQuery* query,
    const LanceDBColumnOrdering* orderings,
    size_t num_orderings,
    char** error_message
);

/**
 * Ask a plain query to return the `_rowid` metadata column
 *
 * This applies to ordinary scans and standalone full-text searches built with
 * lancedb_query_full_text_search(). The UInt64 metadata column is appended to
 * the projection, even when lancedb_query_select() names specific columns.
 *
 * @param query - pointer to LanceDBQuery
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return Error code indicating success or failure
 *
 * If error_message is provided and an error occurs, the caller must free
 * the error message with lancedb_free_string().
 */
LanceDBError lancedb_query_with_row_id(
    LanceDBQuery* query,
    char** error_message
);

/**
 * Restrict a plain query to data covered by an index
 *
 * For an ordinary scan this applies only when the filter plan uses a scalar
 * index. An unfiltered scan, or a filter without an index, still reads every
 * selected fragment. A standalone full-text search reads only rows covered by
 * the text index. Rows appended after the index was built can therefore be
 * absent. A filter that cannot use an index is not an error.
 *
 * @param query - pointer to LanceDBQuery
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return Error code indicating success or failure
 *
 * If error_message is provided and an error occurs, the caller must free
 * the error message with lancedb_free_string().
 */
LanceDBError lancedb_query_fast_search(
    LanceDBQuery* query,
    char** error_message
);

/**
 * Set WHERE filter for query
 *
 * If both a SQL WHERE clause and a DataFusion filters are set, the DataFusion expression
 * takes precedence.
 *
 * @param query - pointer to LanceDBQuery
 * @param filter - SQL WHERE clause string
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return Error code indicating success or failure
 *
 * If error_message is provided and an error occurs, the caller must free
 * the error message with lancedb_free_string().
 */
LanceDBError lancedb_query_where_filter(
    LanceDBQuery* query,
    const char* filter,
    char** error_message
);

/**
 * Set full-text search for query
 *
 * LanceDB applies the filter and projection in the same query and adds its
 * score column. If no limit is set, LanceDB's default top-K applies.
 *
 * If `column` is NULL or empty, LanceDB discovers the text columns itself and
 * searches every column that has a full-text index. An explicit column selects
 * that text column; embedded LanceDB can also scan it without an index, while
 * an index is recommended for larger tables.
 *
 * @param query - pointer to LanceDBQuery
 * @param text - full-text query string
 * @param column - text column name, or NULL/empty for automatic discovery
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return Error code indicating success or failure
 *
 * If error_message is provided and an error occurs, the caller must free
 * the error message with lancedb_free_string().
 */
LanceDBError lancedb_query_full_text_search(
    LanceDBQuery* query,
    const char* text,
    const char* column,
    char** error_message
);

/**
 * Term combination operator for lancedb_fts_query_new_match() and
 * lancedb_fts_query_new_multi_match()
 *
 * LANCEDB_FTS_OPERATOR_OR matches documents containing any query term and is
 * the SDK default. LANCEDB_FTS_OPERATOR_AND requires every term.
 */
typedef enum {
    LANCEDB_FTS_OPERATOR_OR = 0,
    LANCEDB_FTS_OPERATOR_AND = 1
} LanceDBFtsOperator;

/**
 * Options for lancedb_fts_query_new_match()
 *
 * Initialize with lancedb_fts_match_options_default() and change only the
 * fields that need a non-default value. The defaults come from the SDK's
 * MatchQuery constructor: OR matching, boost 1.0, fuzziness 0 (exact),
 * max_expansions 50, and prefix_length 0.
 *
 * `fuzziness` is -1 to pass the SDK's automatic choice, or 0 through UINT32_MAX
 * for an explicit maximum edit distance. Keeping -1 distinct from 0 preserves
 * the SDK's automatic-versus-exact distinction. With the pinned Lance version,
 * embedded and MemWAL queries currently treat automatic matching as exact.
 *
 * `boost` is a score multiplier. The constructor forwards it unchanged; the SDK
 * requires a finite, non-negative value when the query is planned.
 *
 * `max_expansions` is the maximum final vocabulary size for this Match leaf.
 * The ordinary embedded planner shares one budget across its query positions and
 * selected physical segments. The pinned MemWAL path applies the budget
 * separately to the base table, each SSTable and each in-memory source, using
 * each source's own vocabulary, so its effective total can exceed one budget.
 * Every sibling Match leaf has its own budget.
 *
 * `term_operator` must be a LanceDBFtsOperator value. Any other integer is
 * rejected by the constructor rather than converted to an SDK operator.
 *
 * `prefix_length` is the number of leading characters kept unchanged during
 * fuzzy matching.
 */
typedef struct {
    int64_t fuzziness;                // -1 = SDK automatic; 0..UINT32_MAX = explicit distance
    float boost;                      // Score multiplier; validated by the SDK at query time
    size_t max_expansions;            // Maximum final vocabulary size for this Match leaf
    LanceDBFtsOperator term_operator; // Term combination; out-of-range values are rejected
    uint32_t prefix_length;           // Leading characters unchanged for fuzzy matching
} LanceDBFtsMatchOptions;

/**
 * Create a structured Match full-text query
 *
 * The text is tokenized by the target index at execution time, and empty text
 * is passed through unchanged. `options == NULL` uses the SDK's MatchQuery
 * defaults, which are also returned by lancedb_fts_match_options_default().
 * The constructor copies `text` and `column`, so the caller may change or free
 * them after this call returns. A NULL or empty column leaves the choice to the
 * index.
 *
 * `fuzziness` uses -1 for the SDK's automatic mode and 0 through UINT32_MAX for
 * an explicit edit distance. The pinned embedded and MemWAL paths currently
 * resolve automatic fuzziness to exact matching. `boost`, `max_expansions` and
 * `prefix_length` are forwarded to the SDK without additional validation;
 * `boost` is checked when the query is planned.
 *
 * A successfully constructed query can still be limited by the selected
 * backend. A namespace QueryTable pushdown sends a root OR Match as text and
 * column only, so its fuzziness, boost, max_expansions, prefix_length and OR
 * operator are not transmitted; a root AND Match keeps the structured node.
 * The namespace model stores fuzziness, max_expansions and prefix_length as
 * signed 32-bit values. Values above INT32_MAX still construct, but an AND
 * pushdown fails during execution with LANCEDB_INVALID_INPUT. This also applies
 * to a pushed-down hybrid query's full-text branch.
 *
 * A MemWAL LSM query requires an explicit single text column; automatic
 * discovery through a NULL or empty column does not satisfy this requirement.
 * The write spec must maintain that column's FTS index, and every resident
 * MemTable must carry it. Otherwise the query fails with
 * LANCEDB_NOT_SUPPORTED rather than silently omitting un-compacted documents.
 *
 * LanceDB Enterprise servers 0.3.0 and later accept the structured query.
 * Older Enterprise servers receive only text and column, so those options are
 * not transmitted, and a root AND Match returns LANCEDB_NOT_SUPPORTED.
 *
 * On entry `*query_out` and the error message are cleared. On success
 * `*query_out` owns a handle that the caller frees with
 * lancedb_fts_query_free(). On failure `*query_out` stays NULL and the error is
 * LANCEDB_INVALID_ARGUMENT for a missing text or output pointer, invalid UTF-8,
 * an operator outside LanceDBFtsOperator, or a fuzziness outside -1 through
 * UINT32_MAX.
 *
 * @param text - Match text, a valid null-terminated UTF-8 string
 * @param column - text column name, or NULL/empty for automatic discovery
 * @param options - Match options, or NULL for the SDK defaults
 * @param query_out - pointer to receive the structured full-text query
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return Error code indicating success or failure
 *
 * If error_message is provided and an error occurs, the caller must free
 * the error message with lancedb_free_string().
 */
LanceDBError lancedb_fts_query_new_match(
    const char* text,
    const char* column,
    const LanceDBFtsMatchOptions* options,
    LanceDBFtsQuery** query_out,
    char** error_message
);

/**
 * Return the default structured Match query options
 *
 * The values are read from the SDK's MatchQuery constructor: OR term
 * combination, boost 1.0, exact matching (fuzziness 0), 50 maximum expansions,
 * and prefix length 0. The returned struct is a value; no free function is
 * needed.
 *
 * @return Default options for lancedb_fts_query_new_match()
 */
LanceDBFtsMatchOptions lancedb_fts_match_options_default(void);

/**
 * Options for lancedb_fts_query_new_multi_match()
 *
 * `boosts` points to `num_boosts` per-column score multipliers in column order.
 * `num_boosts == 0` uses the SDK default of 1.0 for every column and does not
 * read `boosts`; otherwise `boosts` must be non-NULL and `num_boosts` must
 * equal the constructor's `num_columns`. The constructor forwards values
 * unchanged; the embedded planner requires each to be finite and non-negative
 * and otherwise fails with LANCEDB_INVALID_INPUT.
 *
 * `term_operator` is the shared term combination operator applied to the Match
 * leaf for every column. It must be a LanceDBFtsOperator value; any other
 * integer is rejected by the constructor. The SDK default is OR, which is also
 * returned by lancedb_fts_multi_match_options_default().
 */
typedef struct {
    const float* boosts;              // Per-column multipliers; NULL with num_boosts 0 for defaults
    size_t num_boosts;                // Number of entries in boosts
    LanceDBFtsOperator term_operator; // Shared term operator for every column
} LanceDBFtsMultiMatchOptions;

/**
 * Create a structured MultiMatch full-text query
 *
 * One Match leaf is created for every column, in the order given, and all
 * leaves share the text and the operator. The constructor copies `text`, the
 * column name array and any boosts, so the caller may change or free them
 * after this call returns. `columns` may be NULL only when `num_columns` is 0;
 * an empty column list is rejected by the SDK constructor with
 * LANCEDB_INVALID_INPUT. A NULL options pointer uses the SDK defaults: no
 * per-column boosts and OR term combination, which are also returned by
 * lancedb_fts_multi_match_options_default().
 *
 * Every row is scored independently by each leaf and returned once with the
 * highest leaf score (best fields), so a row matching several columns is not
 * duplicated. A per-column boost multiplies that leaf's score, and the shared
 * operator applies to every leaf. The SDK defaults for fuzziness (exact),
 * max_expansions (50) and prefix_length (0) apply to every leaf; each leaf has
 * its own max_expansions budget.
 *
 * Embedded LanceDB rejects a MultiMatch that resolves to ListElement documents,
 * including a single-column or nested MultiMatch. Lance reports the rejection as
 * its internal Error::NotSupported; the pinned SDK wraps that variant as
 * Error::Lance, so the C status is LANCEDB_LANCE with the "MultiMatch does not
 * support ListElement document granularity" diagnostic, not
 * LANCEDB_NOT_SUPPORTED. That is an embedded dataset-scanner check and is
 * separate from the MemWAL routing limits below.
 *
 * The constructor validates representation only and leaves semantics to the
 * SDK. A nonzero boost count that differs from the column count fails here with
 * LANCEDB_INVALID_INPUT. Boost values are not checked here: the embedded
 * dataset scanner requires each to be finite and non-negative when it plans
 * the query and otherwise fails with LANCEDB_INVALID_INPUT.
 *
 * A namespace QueryTable pushdown sends MultiMatch as a structured node rather
 * than as text, because only a root OR Match uses the string form. The
 * namespace `multi_match` model carries the ordered `match_queries` leaves
 * with their per-leaf boost and operator, so those settings survive the
 * pushdown. The namespace Match model stores fuzziness, max_expansions and
 * prefix_length as signed 32-bit values; this constructor leaves them at the
 * SDK defaults, so it cannot exceed those ranges. A non-finite boost becomes
 * JSON null in the intermediate value and then an unset optional in the
 * namespace model, so the REST request omits it and the server applies its
 * default instead of failing locally. A finite negative boost is forwarded
 * unchanged; the server determines whether to accept it.
 *
 * LanceDB Enterprise servers 0.3.0 and later receive the structured node with
 * every leaf, so ordered columns, per-column boosts and the shared operator
 * are preserved. A non-finite boost stays an explicit JSON null in that
 * request; the client does not reject it and the server determines the
 * outcome. Older Enterprise servers reject AND only for a root Match or
 * MultiMatch, including a hybrid query's full-text leg, with
 * LANCEDB_NOT_SUPPORTED. A MultiMatch nested under Boolean or Boost is not
 * inspected by that check; it follows the parent's legacy text projection,
 * which can lose the AND constraint. For example, a Boost whose positive leg
 * is an AND MultiMatch sends the positive leg's shared text over the union of
 * the columns, so the server can treat it as OR. With a root OR MultiMatch, an
 * older server receives the shared text and the union of the columns; every
 * per-column boost is dropped, and the client does not guarantee the resulting
 * membership or ranking. The server determines the result from that
 * projection.
 *
 * A MemWAL LSM full-text query first checks the table's unenforced primary
 * key, and a table without one fails with LANCEDB_INVALID_INPUT before FTS
 * planning. The pinned SDK then requires the whole full-text tree to name
 * exactly one explicit column, and that column's FTS index must be maintained
 * by the write spec and carried by every resident MemTable; otherwise the
 * query fails with LANCEDB_NOT_SUPPORTED. A MultiMatch spanning several
 * columns fails that check even when the table has a primary key, before
 * Lance's lower-level cross-column path is reached. The same single-column
 * check applies to a hybrid query's full-text leg. Every leaf keeps its own
 * max_expansions budget.
 *
 * On entry `*query_out` and the error message are cleared. On success
 * `*query_out` owns a handle that the caller frees with
 * lancedb_fts_query_free(). On failure `*query_out` stays NULL. A missing text,
 * output, or columns pointer with a nonzero count, a NULL column entry, invalid
 * UTF-8, a NULL boost array with a nonzero count, or an operator outside
 * LanceDBFtsOperator returns LANCEDB_INVALID_ARGUMENT. An empty column list or
 * a nonzero boost count that differs from the column count returns
 * LANCEDB_INVALID_INPUT from the SDK.
 *
 * @param text - text matched against every column, a valid null-terminated UTF-8 string
 * @param columns - ordered array of column names, or NULL when num_columns is 0
 * @param num_columns - number of column names
 * @param options - MultiMatch options, or NULL for the SDK defaults
 * @param query_out - pointer to receive the structured full-text query
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return Error code indicating success or failure
 *
 * If error_message is provided and an error occurs, the caller must free
 * the error message with lancedb_free_string().
 */
LanceDBError lancedb_fts_query_new_multi_match(
    const char* text,
    const char* const* columns,
    size_t num_columns,
    const LanceDBFtsMultiMatchOptions* options,
    LanceDBFtsQuery** query_out,
    char** error_message
);

/**
 * Return the default structured MultiMatch query options
 *
 * The SDK default is OR term combination for every leaf and no per-column
 * boost, so every leaf uses the MatchQuery defaults. The returned struct is a
 * value; no free function is needed.
 *
 * @return Default options for lancedb_fts_query_new_multi_match()
 */
LanceDBFtsMultiMatchOptions lancedb_fts_multi_match_options_default(void);

/**
 * Create a structured phrase full-text query
 *
 * The phrase text is tokenized by the target index at execution time.
 * `slop = 0` requires adjacent tokens in order; a positive `slop` allows up to
 * that many intervening tokens, so `slop = 1` matches both `alpha beta` and
 * `alpha gap beta` for the phrase `alpha beta`.
 *
 * If `column` is NULL or empty, the SDK chooses an indexed text column. An
 * explicit column can be scanned without an index by embedded LanceDB; an
 * indexed phrase query requires positions in the index.
 *
 * The constructor copies `text` and `column`, so the caller may change or free
 * them after this call returns. The returned handle owns an SDK query. Attach
 * it with lancedb_query_full_text_search_query() or
 * lancedb_vector_query_full_text_search_query(); each setter clones it, so the
 * same handle can be used for several builders and freed once those calls
 * return.
 *
 * The SDK selects embedded, namespace, or Enterprise execution from the
 * connection. Embedded LanceDB can scan an explicitly named column without an
 * index; an indexed phrase query requires a full-text index with stored
 * positions. Create it with lancedb_table_create_index() and
 * `fts_with_position = 1`; an index without positions makes an indexed phrase
 * query fail with the SDK's missing-position error. A namespace connection
 * forwards the structured node only when QueryTable pushdown is enabled and
 * selected with LANCEDB_NAMESPACE_PUSHDOWN_QUERY_TABLE. LanceDB Enterprise
 * servers 0.3.0 and later accept the structured node. Older Enterprise servers
 * receive only the column and the phrase text quoted as one string, so a
 * nonzero slop is not transmitted.
 *
 * The pinned Lance indexed path converts `slop` to signed 32-bit. Values from 0
 * through INT32_MAX keep their indexed meaning; larger `uint32_t` values are
 * forwarded unchanged by this API but can return no matches because of that
 * internal conversion. This function does not clamp or reject them.
 *
 * On entry `*query_out` and the error message are cleared. On success
 * `*query_out` owns a handle that the caller frees with
 * lancedb_fts_query_free(). On failure `*query_out` stays NULL and the error is
 * LANCEDB_INVALID_ARGUMENT for NULL text or invalid UTF-8.
 *
 * @param text - phrase text, a valid null-terminated UTF-8 string
 * @param column - text column name, or NULL/empty for automatic discovery
 * @param slop - maximum number of intervening tokens; 0 requires an exact phrase
 * @param query_out - pointer to receive the structured full-text query
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return Error code indicating success or failure
 *
 * If error_message is provided and an error occurs, the caller must free
 * the error message with lancedb_free_string().
 */
LanceDBError lancedb_fts_query_new_phrase(
    const char* text,
    const char* column,
    uint32_t slop,
    LanceDBFtsQuery** query_out,
    char** error_message
);

/**
 * Occurrence of one clause in lancedb_fts_query_new_boolean()
 *
 * LANCEDB_FTS_OCCUR_MUST clauses must all match and their scores are summed.
 * LANCEDB_FTS_OCCUR_SHOULD clauses are optional when at least one MUST clause
 * exists and add their score when they match; without MUST clauses, at least
 * one SHOULD clause must match and the matching SHOULD scores are summed.
 * LANCEDB_FTS_OCCUR_MUST_NOT clauses exclude matching documents and never
 * contribute to the score.
 */
typedef enum {
    LANCEDB_FTS_OCCUR_SHOULD = 0,
    LANCEDB_FTS_OCCUR_MUST = 1,
    LANCEDB_FTS_OCCUR_MUST_NOT = 2
} LanceDBFtsOccur;

/**
 * One clause of lancedb_fts_query_new_boolean()
 *
 * `query` is a borrowed handle; the constructor copies its structured query
 * node, so the caller may reuse or free the child handle after the call
 * returns. `occur` must be a LanceDBFtsOccur value; any other integer is
 * rejected by the constructor rather than converted to an SDK occurrence.
 */
typedef struct {
    const LanceDBFtsQuery* query; // Borrowed child query; copied by the constructor
    LanceDBFtsOccur occur;        // Clause occurrence; out-of-range values are rejected
} LanceDBFtsClause;

/**
 * Create a structured Boolean full-text query
 *
 * Clauses are a flat borrowed array; Boolean nodes compose without a recursive
 * C representation, and a child can itself be a Boolean or Boost handle. The
 * constructor copies each clause's structured query node and occurrence, so
 * child handles may be reused in several clauses or freed after this call
 * returns. Only the node is copied; a child handle's candidate limit set with
 * lancedb_fts_query_limit() is not part of the node and does not constrain the
 * Boolean query.
 *
 * SDK clause semantics apply unchanged. MUST clauses must all match and their
 * scores are summed. SHOULD clauses are optional when a MUST clause exists and
 * add their score when they match; without MUST clauses, at least one SHOULD
 * must match and the matching SHOULD scores are summed. MUST_NOT clauses
 * exclude matching documents and never contribute score. An empty Boolean
 * query, and a query whose only clauses are MUST_NOT, construct successfully;
 * the embedded dataset scanner rejects them at execution with
 * LANCEDB_INVALID_INPUT because it requires a leaf for document-granularity
 * resolution and a SHOULD or MUST clause for scoring. This constructor does
 * not impose either requirement early. A MemWAL LSM query runs its column and
 * maintained-index checks first, so a tree without a named column reports that
 * column error before clause validation, and a negative-only query that passes
 * the column checks is validated by the LSM path. A namespace QueryTable
 * pushdown and Enterprise forward the structured tree instead of planning it
 * locally, so the server determines the error after forwarding or projection.
 *
 * A successful construction can still be limited by the selected backend.
 * Embedded LanceDB evaluates the clauses directly, including across several
 * text columns, and resolves a NULL or empty child column against the indexed
 * text columns.
 *
 * A namespace QueryTable pushdown sends Boolean and Boost roots as structured
 * nodes rather than text. The tree is converted through the namespace model,
 * whose Match fuzziness, max_expansions and prefix_length and Phrase slop are
 * signed 32-bit, so any of those values above INT32_MAX fails the pushdown
 * with LANCEDB_INVALID_INPUT before it is sent. A non-finite boost or
 * multiplier becomes JSON null in the intermediate value and then an unset
 * optional in the namespace model, so the REST request omits it and the
 * server applies its default. A structured Enterprise request keeps the
 * explicit JSON null instead.
 *
 * A MemWAL LSM query requires every clause to name the same explicit text
 * column: the union of the child columns must contain exactly one column whose
 * FTS index is maintained by the write spec and carried by every resident
 * MemTable. A query with no named column or with several columns fails with
 * LANCEDB_NOT_SUPPORTED, and a query in which only some clauses name the
 * column fails the LSM scanner's column check with LANCEDB_INVALID_INPUT.
 * Each Match leaf keeps its own max_expansions budget, which the pinned MemWAL
 * path applies separately to the base table, each SSTable and each in-memory
 * source.
 *
 * LanceDB Enterprise servers 0.3.0 and later receive the structured Boolean
 * node. Older servers receive a degraded projection: the union of the child
 * columns and an empty text query, because the SDK's flat query string for a
 * Boolean node is empty. The SDK does not reject this locally, so an older
 * server cannot execute the intended Boolean semantics.
 *
 * On entry `*query_out` and the error message are cleared. On success
 * `*query_out` owns a handle that the caller frees with
 * lancedb_fts_query_free(). On failure `*query_out` stays NULL and the error is
 * LANCEDB_INVALID_ARGUMENT for a missing output pointer, a NULL clause array
 * with a nonzero count, a NULL child handle, or an occurrence outside
 * LanceDBFtsOccur.
 *
 * @param clauses - array of borrowed clauses, or NULL when num_clauses is 0
 * @param num_clauses - number of clauses in the array
 * @param query_out - pointer to receive the structured full-text query
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return Error code indicating success or failure
 *
 * If error_message is provided and an error occurs, the caller must free
 * the error message with lancedb_free_string().
 */
LanceDBError lancedb_fts_query_new_boolean(
    const LanceDBFtsClause* clauses,
    size_t num_clauses,
    LanceDBFtsQuery** query_out,
    char** error_message
);

/**
 * Create a structured Boost full-text query
 *
 * `positive` and `negative` are required borrowed handles; the constructor
 * copies each handle's structured query node, so the caller may reuse or free
 * the handles after this call returns. Only the node is copied; a child
 * handle's candidate limit set with lancedb_fts_query_limit() is not part of
 * the node and does not constrain the Boost query. `negative_boost` may be
 * NULL, which passes "unset" to the SDK constructor so the SDK supplies its
 * default of 0.5. A pointed-to value is forwarded unchanged. The embedded dataset
 * scanner requires a finite, non-negative multiplier when it plans the query
 * and otherwise fails with LANCEDB_INVALID_INPUT. A MemWAL LSM query runs its
 * column checks before that validation, and a namespace or Enterprise
 * execution determines the outcome after forwarding the structured tree.
 *
 * The positive query drives iteration: only documents matching it appear. When
 * the negative query also matches a document, its score times the multiplier
 * is subtracted from the positive score; otherwise the positive score is
 * returned unchanged. Boost is a positive gate plus a demotion, not a
 * disjunction.
 *
 * A successful construction can still be limited by the selected backend.
 * Embedded LanceDB evaluates the node directly, including across columns, and
 * resolves NULL or empty child columns against the indexed text columns.
 *
 * A namespace QueryTable pushdown sends Boolean and Boost roots as structured
 * nodes rather than text. The tree is converted through the namespace model,
 * whose Match fuzziness, max_expansions and prefix_length and Phrase slop are
 * signed 32-bit, so any of those values above INT32_MAX fails the pushdown
 * with LANCEDB_INVALID_INPUT before it is sent. A non-finite boost or
 * multiplier becomes JSON null in the intermediate value and then an unset
 * optional in the namespace model, so the REST request omits it and the
 * server applies its default. A structured Enterprise request keeps the
 * explicit JSON null instead.
 *
 * A MemWAL LSM query requires both legs to name the same explicit text
 * column: the union of the leg columns must contain exactly one column whose
 * FTS index is maintained by the write spec and carried by every resident
 * MemTable. A query with no named column or with several columns fails with
 * LANCEDB_NOT_SUPPORTED, and a query in which only one leg names the column
 * fails the LSM scanner's column check with LANCEDB_INVALID_INPUT. Each Match
 * leaf keeps its own max_expansions budget, which the pinned MemWAL path
 * applies separately to the base table, each SSTable and each in-memory
 * source.
 *
 * LanceDB Enterprise servers 0.3.0 and later receive the structured Boost
 * node. Older servers receive a degraded projection: the SDK's flat query
 * string of the positive leg (empty for a Boolean positive) searched over the
 * union of both legs' columns, so the negative leg and the multiplier are
 * silently dropped. The SDK does not reject this locally.
 *
 * On entry `*query_out` and the error message are cleared. On success
 * `*query_out` owns a handle that the caller frees with
 * lancedb_fts_query_free(). On failure `*query_out` stays NULL and the error is
 * LANCEDB_INVALID_ARGUMENT for a missing output pointer or a NULL positive or
 * negative handle.
 *
 * @param positive - required borrowed handle matching the scored documents
 * @param negative - required borrowed handle demoting documents it matches
 * @param negative_boost - multiplier, or NULL for the SDK default of 0.5
 * @param query_out - pointer to receive the structured full-text query
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return Error code indicating success or failure
 *
 * If error_message is provided and an error occurs, the caller must free
 * the error message with lancedb_free_string().
 */
LanceDBError lancedb_fts_query_new_boost(
    const LanceDBFtsQuery* positive,
    const LanceDBFtsQuery* negative,
    const float* negative_boost,
    LanceDBFtsQuery** query_out,
    char** error_message
);

/**
 * Replace the column selection of a structured full-text query
 *
 * This applies the pinned SDK's `FullTextSearchQuery::with_columns` to a copy
 * and publishes the result only after the SDK accepts it. A failure leaves the
 * handle's previous query, candidate limit and settings unchanged. The
 * operation replaces the columns of every Match and Phrase leaf; it does not
 * fill only leaves that lack a column.
 *
 * A Match leaf becomes one leaf per selected column. Several columns therefore
 * turn it into a MultiMatch, and every scalar setting of the original leaf
 * (boost, fuzziness, max_expansions, operator, prefix_length and document
 * granularity) is copied to each new leaf. A Phrase leaf accepts exactly one
 * selected column; zero or several columns fail with
 * LANCEDB_INVALID_INPUT. Boolean nodes recurse into must, should and must_not,
 * and Boost nodes recurse into both legs. A MultiMatch expands each existing
 * Match leaf by every selected column, so N existing leaves produce N * M
 * leaves and each copy keeps its original scalar settings. Prefer constructing
 * a MultiMatch with its columns; this operation is intended for trees whose
 * leaves have not selected columns. A selection that makes the tree span
 * several columns follows the existing MultiMatch backend limits: native
 * MemWAL requires one explicit column. An empty Boolean has no leaves and
 * succeeds unchanged.
 *
 * A zero column count is forwarded as an empty selection and normally fails
 * with LANCEDB_INVALID_INPUT, except for an empty Boolean. Empty strings are
 * forwarded as named columns rather than automatic discovery; the SDK reports
 * their resolution error when the query runs. A NULL array with a nonzero
 * count, a NULL entry or invalid UTF-8 returns LANCEDB_INVALID_ARGUMENT and
 * leaves the handle usable. SDK errors keep their mapped status.
 *
 * Backend routing follows the rewritten tree. A namespace QueryTable pushdown
 * sends a root OR Match as a string query, with the selected columns in
 * `string_query.columns`. A root MultiMatch, including a Match that this call
 * rewrote because several columns were selected, and a root AND Match, Phrase,
 * Boolean or Boost use the structured node form, where the selected columns
 * live inside the Match leaves instead. Enterprise 0.3.0 and later receive the
 * structured node; older Enterprise projections remain the ones documented on
 * the constructors.
 *
 * @param query - pointer to LanceDBFtsQuery
 * @param columns - ordered column names, or NULL when num_columns is zero
 * @param num_columns - number of column names
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return Error code indicating success or failure
 *
 * If error_message is provided and an error occurs, the caller must free
 * the error message with lancedb_free_string().
 */
LanceDBError lancedb_fts_query_with_columns(
    LanceDBFtsQuery* query,
    const char* const* columns,
    size_t num_columns,
    char** error_message
);

/**
 * Set the candidate limit of a structured full-text query
 *
 * This sets the pinned SDK's FullTextSearchQuery candidate ceiling. It is
 * separate from lancedb_query_limit() and lancedb_vector_query_limit(), which
 * limit the final query result. On a backend that consumes the field, zero
 * requests no full-text candidates, so a standalone full-text query returns no
 * rows and a hybrid query keeps only its vector contribution. A positive value
 * caps the candidates the full-text leg may produce before the final query
 * limit and offset are applied.
 *
 * The value is forwarded unchanged as int64_t. Embedded planning converts it
 * to size_t, so a negative value becomes a very large ceiling on a 64-bit
 * target. The native MemWAL LSM path rejects a negative value at execution
 * with LANCEDB_INVALID_INPUT. This setter does not clamp or reject negative
 * values; the selected backend determines the outcome.
 *
 * There is no upper bound. A very large positive value can exhaust memory or
 * otherwise fail during execution. The native MemWAL LSM path is the most
 * exposed because it applies the value to every planned FTS column. When the
 * intent is no explicit ceiling, call lancedb_fts_query_unset_limit() instead
 * of passing a very large value.
 *
 * The pinned SDK consumes the ceiling for standalone embedded full-text
 * search, for the full-text leg of an embedded hybrid query, and for a native
 * MemWAL LSM full-text query. Namespace QueryTable pushdown and LanceDB
 * Enterprise requests do not carry the field: only their outer result limit
 * applies, so the ceiling is silently absent on those backends.
 *
 * The limit belongs to the full-text search wrapper, not to the query tree.
 * lancedb_fts_query_new_boolean() and lancedb_fts_query_new_boost() copy child
 * query nodes and do not copy a child's candidate limit. Attaching the handle
 * with lancedb_query_full_text_search_query() or
 * lancedb_vector_query_full_text_search_query() copies the whole wrapper,
 * including this limit, so later changes to the handle do not affect builders
 * that already attached it.
 *
 * @param query - pointer to LanceDBFtsQuery
 * @param limit - candidate ceiling
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return Error code indicating success or failure
 *
 * If error_message is provided and an error occurs, the caller must free
 * the error message with lancedb_free_string().
 */
LanceDBError lancedb_fts_query_limit(
    LanceDBFtsQuery* query,
    int64_t limit,
    char** error_message
);

/**
 * Remove the candidate limit from a structured full-text query
 *
 * This sets the pinned SDK FullTextSearchQuery limit to unset. Embedded
 * standalone full-text search derives its search ceiling from the final query
 * limit, plus any offset, when that limit is set. With an explicit column
 * ordering, no ceiling is derived: the full match set is retained so the
 * ordering can select the page. Native MemWAL derives its ceiling from the
 * scanner limit and offset. The final query result limit remains independent.
 *
 * @param query - pointer to LanceDBFtsQuery
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return Error code indicating success or failure
 *
 * If error_message is provided and an error occurs, the caller must free
 * the error message with lancedb_free_string().
 */
LanceDBError lancedb_fts_query_unset_limit(
    LanceDBFtsQuery* query,
    char** error_message
);

/**
 * Free a structured full-text query
 *
 * @param query - pointer to LanceDBFtsQuery, or NULL
 *
 * The handle is independent of every builder it was attached to, so freeing it
 * does not change those builders. Create it with lancedb_fts_query_new_match(),
 * lancedb_fts_query_new_multi_match(), lancedb_fts_query_new_phrase(),
 * lancedb_fts_query_new_boolean() or lancedb_fts_query_new_boost().
 */
void lancedb_fts_query_free(LanceDBFtsQuery* query);

/**
 * Set a structured full-text query for query
 *
 * This replaces the text Match query set by lancedb_query_full_text_search()
 * or an earlier call to this function. The builder clones the handle's query,
 * so `full_text_query` is borrowed: the caller keeps ownership and may reuse or
 * free it immediately after this call returns. On failure, including a NULL
 * handle, any previous full-text setting stays unchanged.
 *
 * @param query - pointer to LanceDBQuery
 * @param full_text_query - pointer to LanceDBFtsQuery (borrowed, not consumed)
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return Error code indicating success or failure
 *
 * If error_message is provided and an error occurs, the caller must free
 * the error message with lancedb_free_string().
 */
LanceDBError lancedb_query_full_text_search_query(
    LanceDBQuery* query,
    const LanceDBFtsQuery* full_text_query,
    char** error_message
);

/**
 * Set DataFusion Expr filter for query
 *
 * If both a DataFusion and a SQL WHERE clause filters are set, the DataFusion expression
 * takes precedence.
 *
 * @param query - pointer to LanceDBQuery
 * @param expr - pointer to LanceDBExpr (consumed by this function; do not use or free after calling)
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return Error code indicating success or failure
 */
LanceDBError lancedb_query_df_filter(
    LanceDBQuery* query,
    LanceDBExpr* expr,
    char** error_message
);

/**
 * Set limit for vector query
 *
 * This limits the final query result. It is independent of a structured
 * full-text query's candidate ceiling, which is set with
 * lancedb_fts_query_limit().
 *
 * @param query - pointer to LanceDBVectorQuery
 * @param limit - maximum number of results
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return Error code indicating success or failure
 *
 * If error_message is provided and an error occurs, the caller must free
 * the error message with lancedb_free_string().
 */
LanceDBError lancedb_vector_query_limit(
    LanceDBVectorQuery* query,
    size_t limit,
    char** error_message
);

/**
 * Set offset for vector query
 *
 * @param query - pointer to LanceDBVectorQuery
 * @param offset - number of results to skip
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return Error code indicating success or failure
 *
 * If error_message is provided and an error occurs, the caller must free
 * the error message with lancedb_free_string().
 */
LanceDBError lancedb_vector_query_offset(
    LanceDBVectorQuery* query,
    size_t offset,
    char** error_message
);

/**
 * Set vector column for vector query
 *
 * @param query - pointer to LanceDBVectorQuery
 * @param column - vector column name
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return Error code indicating success or failure
 *
 * If error_message is provided and an error occurs, the caller must free
 * the error message with lancedb_free_string().
 */
LanceDBError lancedb_vector_query_column(
    LanceDBVectorQuery* query,
    const char* column,
    char** error_message
);

/**
 * Set columns to select for vector query
 *
 * @param query - pointer to LanceDBVectorQuery
 * @param columns - array of column name strings
 * @param num_columns - number of columns
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return Error code indicating success or failure
 *
 * If error_message is provided and an error occurs, the caller must free
 * the error message with lancedb_free_string().
 */
LanceDBError lancedb_vector_query_select(
    LanceDBVectorQuery* query,
    const char* const* columns,
    size_t num_columns,
    char** error_message
);

/**
 * Reset a vector query to every non-system column
 *
 * This clears a projection set by lancedb_vector_query_select() or
 * lancedb_vector_query_select_dynamic(). The most recent projection call wins.
 * lancedb_vector_query_with_row_id() still appends the `_rowid` metadata
 * column.
 *
 * @param query - pointer to LanceDBVectorQuery
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return Error code indicating success or failure
 *
 * If error_message is provided and an error occurs, the caller must free
 * the error message with lancedb_free_string().
 */
LanceDBError lancedb_vector_query_select_all(
    LanceDBVectorQuery* query,
    char** error_message
);

/**
 * Set an expression-based column projection for vector query
 *
 * Each pair adds one output column: names[i] is the output name and
 * expressions[i] is an SQL expression over the table columns. Columns are
 * returned in the given order, which may differ from the table order. The names
 * and expressions are copied; the arrays may be released after the call.
 *
 * A zero num_columns selects no columns. Both arrays must still be non-NULL.
 * Calling this function replaces a projection from lancedb_vector_query_select()
 * or lancedb_vector_query_select_all(), and a later call to either of those
 * replaces this projection.
 *
 * The SDK evaluates the expressions and decides whether a backend supports
 * dynamic projection. An unsupported backend's SDK error is returned unchanged.
 *
 * @param query - pointer to LanceDBVectorQuery
 * @param names - array of num_columns output column names
 * @param expressions - array of num_columns SQL expressions
 * @param num_columns - number of entries in each array
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return Error code indicating success or failure
 *
 * If error_message is provided and an error occurs, the caller must free
 * the error message with lancedb_free_string().
 */
LanceDBError lancedb_vector_query_select_dynamic(
    LanceDBVectorQuery* query,
    const char* const* names,
    const char* const* expressions,
    size_t num_columns,
    char** error_message
);

/**
 * Set the column ordering for vector query
 *
 * The first entry is the primary sort key. For an embedded vector search, the
 * nearest-neighbour candidate count is limit + offset, or the SDK's default
 * top-K + offset when no limit is set. The search orders those candidates and
 * then applies offset and limit to the ordered page; it does not sort the whole
 * table. On a hybrid vector plus full-text query it applies to both search
 * legs, and the reranker still determines the final fused order. EXPLAIN,
 * output-schema inspection and ANALYZE use the same configured ordering,
 * subject to the hybrid inspection limits documented below.
 *
 * A zero num_orderings clears the ordering, restoring the query's default
 * order; orderings may be NULL in that case. A positive count requires a
 * non-NULL array whose entries and column names are non-NULL, NUL-terminated
 * UTF-8 strings. Invalid input returns LANCEDB_INVALID_ARGUMENT and leaves the
 * previous ordering unchanged. The array and its strings are borrowed for the
 * duration of the call and are copied into the query.
 *
 * A native namespace connection with QueryTable pushdown executes a nonempty
 * ordering locally, so it needs client access to table storage. A table routed
 * through the MemWAL LSM scanner rejects a nonempty ordering with
 * LANCEDB_NOT_SUPPORTED; clear the ordering to return to the default route. A
 * remote LanceDB forwards the ordering to the server.
 *
 * @param query - pointer to LanceDBVectorQuery
 * @param orderings - array of column orderings, or NULL when num_orderings is 0
 * @param num_orderings - number of entries in orderings; 0 clears the ordering
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return Error code indicating success or failure
 *
 * If error_message is provided and an error occurs, the caller must free
 * the error message with lancedb_free_string().
 */
LanceDBError lancedb_vector_query_order_by(
    LanceDBVectorQuery* query,
    const LanceDBColumnOrdering* orderings,
    size_t num_orderings,
    char** error_message
);

/**
 * Ask a vector query to return the `_rowid` metadata column
 *
 * This also applies to a hybrid vector plus full-text query. The UInt64
 * metadata column is appended to the projection, even when
 * lancedb_vector_query_select() names specific columns.
 *
 * @param query - pointer to LanceDBVectorQuery
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return Error code indicating success or failure
 *
 * If error_message is provided and an error occurs, the caller must free
 * the error message with lancedb_free_string().
 */
LanceDBError lancedb_vector_query_with_row_id(
    LanceDBVectorQuery* query,
    char** error_message
);

/**
 * Restrict a vector query to rows covered by the vector index
 *
 * The SDK turns this into an index-only search: rows appended or updated after
 * the index was built may be absent, and a query with no usable vector index
 * returns no rows. A SQL or DataFusion filter keeps its configured pre-filter
 * or post-filter placement.
 *
 * On a hybrid query, fast search is part of the vector query's shared request,
 * so it applies to both the full-text and vector legs. In contrast,
 * lancedb_vector_query_bypass_vector_index() only affects the vector leg.
 * Embedded LanceDB applies fast search after the bypass flag, so fast search
 * wins when both are set. A remote LanceDB forwards both flags to the server,
 * which decides their interaction.
 *
 * @param query - pointer to LanceDBVectorQuery
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return Error code indicating success or failure
 *
 * If error_message is provided and an error occurs, the caller must free
 * the error message with lancedb_free_string().
 */
LanceDBError lancedb_vector_query_fast_search(
    LanceDBVectorQuery* query,
    char** error_message
);

/**
 * Skip the vector index and compare the query vector with every row
 *
 * This performs an exhaustive flat search even when a vector index exists. It
 * is useful as ground truth for recall measurements and returns exact
 * distances without index quantization. A filter still keeps its configured
 * pre-filter or post-filter placement.
 *
 * On a hybrid query this only affects the vector leg; the full-text leg remains
 * unchanged. Embedded LanceDB applies fast search after this flag, so a query
 * with both set behaves as a fast search. A remote LanceDB forwards both flags
 * to the server, which decides their interaction.
 *
 * @param query - pointer to LanceDBVectorQuery
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return Error code indicating success or failure
 *
 * If error_message is provided and an error occurs, the caller must free
 * the error message with lancedb_free_string().
 */
LanceDBError lancedb_vector_query_bypass_vector_index(
    LanceDBVectorQuery* query,
    char** error_message
);

/**
 * Set WHERE filter for vector query
 *
 * If both a SQL WHERE clause and a DataFusion filters are set, the DataFusion expression
 * takes precedence.
 *
 * @param query - pointer to LanceDBVectorQuery
 * @param filter - SQL WHERE clause string
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return Error code indicating success or failure
 *
 * If error_message is provided and an error occurs, the caller must free
 * the error message with lancedb_free_string().
 */
LanceDBError lancedb_vector_query_where_filter(
    LanceDBVectorQuery* query,
    const char* filter,
    char** error_message
);

/**
 * Set full-text search for vector query
 *
 * This turns the vector query into a hybrid nearest-neighbour plus full-text
 * search. Embedded LanceDB executes it as two searches and fuses the result
 * sets. LanceDB applies the vector, filter, projection and limit settings to
 * both searches and adds its relevance score column.
 *
 * If `column` is NULL or empty, LanceDB discovers the text columns itself and
 * searches every column that has a full-text index. An explicit column selects
 * that text column; embedded LanceDB can also scan it without an index, while
 * an index is recommended for larger tables.
 *
 * Embedded LanceDB executes this hybrid query as two searches and fuses their
 * results. EXPLAIN, output schema inspection, and ANALYZE need a single SDK
 * plan, so lancedb_vector_query_explain_plan(),
 * lancedb_vector_query_output_schema() and
 * lancedb_vector_query_analyze_plan() may return the SDK's unsupported-shape
 * error. Execute the query instead.
 *
 * If the vector leg returns batch results with `query_index` and the full-text
 * leg returns matches, the pinned SDK beta.30 RRF merge fails with an Arrow
 * schema error. See lancedb_vector_query_new_typed() for the vector types and
 * routing that produce `query_index`.
 *
 * @param query - pointer to LanceDBVectorQuery
 * @param text - full-text query string
 * @param column - text column name, or NULL/empty for automatic discovery
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return Error code indicating success or failure
 *
 * If error_message is provided and an error occurs, the caller must free
 * the error message with lancedb_free_string().
 */
LanceDBError lancedb_vector_query_full_text_search(
    LanceDBVectorQuery* query,
    const char* text,
    const char* column,
    char** error_message
);

/**
 * Set a structured full-text query for vector query
 *
 * This replaces the text Match query set by
 * lancedb_vector_query_full_text_search() or an earlier call to this function,
 * making the builder a hybrid nearest-neighbour plus full-text search. The
 * builder clones the handle's query, so `full_text_query` is borrowed: the
 * caller keeps ownership and may reuse or free it immediately after this call
 * returns. On failure, including a NULL handle, any previous full-text setting
 * stays unchanged.
 *
 * The SDK's default RRF reranker applies when no reranker is set explicitly.
 * Match, MultiMatch, phrase, Boolean and Boost evaluation, including backend
 * limitations, are described on lancedb_fts_query_new_match(),
 * lancedb_fts_query_new_multi_match(), lancedb_fts_query_new_phrase(),
 * lancedb_fts_query_new_boolean() and lancedb_fts_query_new_boost().
 *
 * If the vector leg returns batch results with `query_index` and the full-text
 * leg returns matches, the pinned SDK beta.30 RRF merge fails with an Arrow
 * schema error. See lancedb_vector_query_new_typed() for the vector types and
 * routing that produce `query_index`.
 *
 * @param query - pointer to LanceDBVectorQuery
 * @param full_text_query - pointer to LanceDBFtsQuery (borrowed, not consumed)
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return Error code indicating success or failure
 *
 * If error_message is provided and an error occurs, the caller must free
 * the error message with lancedb_free_string().
 */
LanceDBError lancedb_vector_query_full_text_search_query(
    LanceDBVectorQuery* query,
    const LanceDBFtsQuery* full_text_query,
    char** error_message
);

/**
 * Set DataFusion Expr filter for vector query
 *
 * If both a DataFusion and a SQL WHERE clause filters are set, the DataFusion expression
 * takes precedence.
 *
 * @param query - pointer to LanceDBVectorQuery
 * @param expr - pointer to LanceDBExpr (consumed by this function; do not use or free after calling)
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return Error code indicating success or failure
 */
LanceDBError lancedb_vector_query_df_filter(
    LanceDBVectorQuery* query,
    LanceDBExpr* expr,
    char** error_message
);

/**
 * Make the vector query filter run after the vector search
 *
 * By default, the filter set with lancedb_vector_query_where_filter() or
 * lancedb_vector_query_df_filter() is applied before the vector search.
 * Calling this function applies the filter to the vector search results instead,
 * matching the Rust SDK's postfilter().
 *
 * This only affects queries that also set a filter. Post-filtering can return
 * fewer than `limit` rows, or no rows, when the nearest vectors do not match
 * the filter. Setting a refine_factor can help recover some results.
 *
 * @param query - pointer to LanceDBVectorQuery
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return Error code indicating success or failure
 *
 * If error_message is provided and an error occurs, the caller must free
 * the error message with lancedb_free_string().
 */
LanceDBError lancedb_vector_query_postfilter(
    LanceDBVectorQuery* query,
    char** error_message
);

/**
 * Use reciprocal rank fusion with an explicit `k` for hybrid search
 *
 * This installs the SDK's RRFReranker on the vector query. It only changes the
 * result of a hybrid query built with
 * lancedb_vector_query_full_text_search(); on a vector-only query the reranker
 * is stored but unused. Fusion scores each result as 1 / (k + rank) with ranks
 * starting at 1, summed across the vector and full-text lists, and returns that
 * sum as `_relevance_score`. When this setter is not called, the SDK uses its
 * own default k = 60. The value is forwarded unchanged, so a caller that treats
 * zero as "use the default" must pass 60 here instead.
 *
 * @param query - pointer to LanceDBVectorQuery
 * @param k - RRF constant; the SDK default is 60 when no setter is called
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return Error code indicating success or failure
 *
 * If error_message is provided and an error occurs, the caller must free
 * the error message with lancedb_free_string().
 */
LanceDBError lancedb_vector_query_rerank_rrf(
    LanceDBVectorQuery* query,
    float k,
    char** error_message
);

/**
 * Set how hybrid-search scores are normalized before reranking
 *
 * This only affects a hybrid query built with
 * lancedb_vector_query_full_text_search(). LANCEDB_NORMALIZE_SCORE normalizes
 * the raw vector distance and full-text score columns; LANCEDB_NORMALIZE_RANK
 * converts them to ranks first. The SDK always normalizes both channels before
 * invoking the reranker, and this setter selects what that normalization starts
 * from. When this setter is not called, the SDK's default normalization applies.
 *
 * The built-in RRF reranker uses only row identities and list positions, so
 * this setting does not change its ranking or `_relevance_score`. It does change
 * the normalized score columns preserved in the result. When this setter is not
 * called, the SDK's default normalization applies. Out-of-range values are
 * rejected with LANCEDB_INVALID_ARGUMENT and leave any previously selected
 * normalization unchanged.
 *
 * @param query - pointer to LanceDBVectorQuery
 * @param normalize_method - normalization method to use
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return Error code indicating success or failure
 *
 * If error_message is provided and an error occurs, the caller must free
 * the error message with lancedb_free_string().
 */
LanceDBError lancedb_vector_query_norm(
    LanceDBVectorQuery* query,
    LanceDBNormalizeMethod normalize_method,
    char** error_message
);

/**
 * Set distance type for vector query
 *
 * @param query - pointer to LanceDBVectorQuery
 * @param distance_type - distance metric to use
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return Error code indicating success or failure
 *
 * If error_message is provided and an error occurs, the caller must free
 * the error message with lancedb_free_string().
 */
LanceDBError lancedb_vector_query_distance_type(
    LanceDBVectorQuery* query,
    LanceDBDistanceType distance_type,
    char** error_message
);

/**
 * Set number of probes for vector query
 *
 * This parameter controls how many partitions to search in IVF (Inverted File) based indices
 * such as IVF_FLAT, IVF_PQ, IVF_HNSW_PQ, and IVF_HNSW_SQ.
 *
 * IVF indices partition the vector space into clusters. During search, nprobes determines
 * how many of these partitions are searched. Higher values improve recall (find more relevant
 * results) at the cost of slower queries. Lower values make queries faster but may miss
 * relevant results that are in non-searched partitions.
 *
 * When this setter is not called, LanceDB leaves the probe bounds unset and uses Lance's
 * adaptive IVF probe defaults. This replaces the fixed minimum/maximum of 20 used by
 * LanceDB 0.33 and earlier. Call this setter with 20 to request the previous fixed behavior.
 *
 * Typical values:
 * - Range: 1 to num_partitions (specified during index creation)
 * - Higher nprobes (e.g., num_partitions/2) → better recall, slower queries
 * - Lower nprobes (e.g., 1-10) → faster queries, lower recall
 *
 * This setter returns LANCEDB_SUCCESS even for 0. The value is validated when the query is
 * executed: lancedb_vector_query_execute() returns NULL for nprobes 0, and
 * lancedb_vector_query_explain_plan() returns LANCEDB_INVALID_INPUT with the message
 * "nprobes must be greater than 0". Callers that accept untrusted values should validate
 * them before executing.
 *
 * @param query - pointer to LanceDBVectorQuery
 * @param nprobes - number of IVF partitions to search (must be <= num_partitions)
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return Error code indicating success or failure
 *
 * If error_message is provided and an error occurs, the caller must free
 * the error message with lancedb_free_string().
 */
LanceDBError lancedb_vector_query_nprobes(
    LanceDBVectorQuery* query,
    size_t nprobes,
    char** error_message
);

/**
 * Set refine factor for vector query
 *
 * This parameter enables a refinement step to improve result accuracy for approximate
 * vector indices (IVF_PQ, IVF_HNSW_PQ, IVF_HNSW_SQ).
 *
 * When refine_factor is set, the search process works in two stages:
 * 1. Fetch (limit × refine_factor) approximate nearest neighbors using the index
 * 2. Re-rank these candidates using exact distance calculations on original vectors
 * 3. Return the top 'limit' results after refinement
 *
 * This improves accuracy because approximate indices (especially quantized ones like PQ/SQ)
 * can have ranking errors. Refinement corrects these errors by recalculating exact distances.
 *
 * Typical values:
 * - Default: No refinement (refine_factor = 1 or not set)
 * - Range: 1 to ~100
 * - refine_factor = 1 → no refinement (fastest, least accurate)
 * - refine_factor = 10 → fetch 10x results and refine (good balance)
 * - refine_factor = 50+ → very accurate but slower
 *
 * Note: Higher values increase query latency proportionally. Not useful for exact indices
 * like IVF_FLAT which already use exact distances.
 *
 * @param query - pointer to LanceDBVectorQuery
 * @param refine_factor - multiplier for candidate set size (fetches limit × refine_factor results)
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return Error code indicating success or failure
 *
 * If error_message is provided and an error occurs, the caller must free
 * the error message with lancedb_free_string().
 */
LanceDBError lancedb_vector_query_refine_factor(
    LanceDBVectorQuery* query,
    unsigned int refine_factor,
    char** error_message
);

/**
 * Set ef parameter for HNSW vector query
 *
 * This parameter controls the exploration factor for HNSW (Hierarchical Navigable Small World)
 * based indices such as IVF_HNSW_PQ and IVF_HNSW_SQ.
 *
 * HNSW is a graph-based index that navigates through a multi-layer graph to find nearest
 * neighbors. The 'ef' parameter determines the size of the dynamic candidate list maintained
 * during the graph traversal. A larger candidate list explores more of the graph, leading to
 * better recall but slower queries.
 *
 * How it works:
 * - During search, HNSW maintains a priority queue of size 'ef' with candidate neighbors
 * - The algorithm explores neighbors of candidates in this queue
 * - Larger ef → explores more paths through the graph → better recall
 * - After exploration, the top 'limit' results are returned
 *
 * Typical values:
 * - Default is usually 100
 * - Must be >= limit (query result size)
 * - Range: limit to ~1000
 * - ef = limit → fastest, lowest recall
 * - ef = 100-200 → good balance for most use cases
 * - ef = 500+ → very high recall, slower queries
 *
 * Note: The ef parameter at query time is different from ef_construction used during
 * index building. Query-time ef controls search quality, while ef_construction controls
 * index quality.
 *
 * @param query - pointer to LanceDBVectorQuery
 * @param ef - exploration factor for HNSW search (must be >= query limit)
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return Error code indicating success or failure
 *
 * If error_message is provided and an error occurs, the caller must free
 * the error message with lancedb_free_string().
 */
LanceDBError lancedb_vector_query_ef(
    LanceDBVectorQuery* query,
    size_t ef,
    char** error_message
);

/**
 * Get the output schema of a query
 *
 * The schema comes from the same configured query that
 * lancedb_query_execute() would run, so a dynamic projection's aliases, types
 * and order are visible before execution. The query is not consumed. The SDK
 * decides how a backend resolves the plan; an Enterprise table may send the
 * SDK's `/query/` request and read the schema from the returned stream.
 *
 * On entry, *schema_out and the error message are cleared. On success,
 * *schema_out holds an owned Arrow C Data schema; release it with
 * lancedb_free_arrow_schema(). On failure it stays NULL and the error message is
 * set when requested. A schema that the C data interface cannot represent
 * returns LANCEDB_ARROW with the conversion detail.
 *
 * @param query - pointer to LanceDBQuery (NOT consumed)
 * @param schema_out - pointer to receive the Arrow schema
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return Error code indicating success or failure
 *
 * If error_message is provided and an error occurs, the caller must free
 * the error message with lancedb_free_string().
 */
LanceDBError lancedb_query_output_schema(
    const LanceDBQuery* query,
    FFI_ArrowSchema** schema_out,
    char** error_message
);

/**
 * Get the query execution plan without executing the query
 *
 * Returns a human-readable string describing what steps will be taken
 * to execute the query. Does NOT consume the query — it can still be
 * executed afterwards.
 *
 * @param query - pointer to LanceDBQuery (NOT consumed)
 * @param verbose - if true, include additional detail in the plan
 * @param plan_out - pointer to receive the plan string
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return Error code indicating success or failure
 *
 * Caller must free the returned plan string with lancedb_free_string().
 * If error_message is provided and an error occurs, the caller must free
 * the error message with lancedb_free_string().
 */
LanceDBError lancedb_query_explain_plan(
    const LanceDBQuery* query,
    bool verbose,
    char** plan_out,
    char** error_message
);

/**
 * Execute the query and return an analyzed plan with runtime metrics
 *
 * ANALYZE executes an analyzable query plan to collect runtime metrics. It
 * reads the same data and has the same side effects as execution while
 * returning no rows. The query is not consumed and can still be executed.
 *
 * On entry, *plan_out and the error message are cleared. On success, *plan_out
 * holds a NUL-terminated plan string; release it with lancedb_free_string(). On
 * failure it stays NULL and the error message is set when requested.
 *
 * @param query - pointer to LanceDBQuery (NOT consumed)
 * @param plan_out - pointer to receive the plan string
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return Error code indicating success or failure
 *
 * If error_message is provided and an error occurs, the caller must free
 * the error message with lancedb_free_string().
 */
LanceDBError lancedb_query_analyze_plan(
    const LanceDBQuery* query,
    char** plan_out,
    char** error_message
);

/**
 * Get the output schema of a vector query
 *
 * The schema comes from the same configured query that
 * lancedb_vector_query_execute() would run, so a dynamic projection's aliases,
 * types and order are visible before execution. The query is not consumed. The
 * SDK decides how a backend resolves the plan; an Enterprise table may send the
 * SDK's `/query/` request and read the schema from the returned stream.
 *
 * Embedded LanceDB executes a hybrid nearest-neighbour plus full-text query as
 * two searches and fuses their results. Output schema inspection requires a
 * single SDK plan, so it may return the same unsupported hybrid error as
 * lancedb_vector_query_explain_plan().
 *
 * On entry, *schema_out and the error message are cleared. On success,
 * *schema_out holds an owned Arrow C Data schema; release it with
 * lancedb_free_arrow_schema(). On failure it stays NULL and the error message is
 * set when requested. A schema that the C data interface cannot represent
 * returns LANCEDB_ARROW with the conversion detail.
 *
 * @param query - pointer to LanceDBVectorQuery (NOT consumed)
 * @param schema_out - pointer to receive the Arrow schema
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return Error code indicating success or failure
 *
 * If error_message is provided and an error occurs, the caller must free
 * the error message with lancedb_free_string().
 */
LanceDBError lancedb_vector_query_output_schema(
    const LanceDBVectorQuery* query,
    FFI_ArrowSchema** schema_out,
    char** error_message
);

/**
 * Get the vector query execution plan without executing the query
 *
 * Returns a human-readable string describing what steps will be taken
 * to execute the query. Does NOT consume the query — it can still be
 * executed afterwards.
 *
 * Embedded LanceDB executes a hybrid query that combines nearest-neighbour and
 * full-text search as two searches and fuses their results. EXPLAIN requires a
 * single SDK plan, so embedded LanceDB rejects that hybrid at plan time with
 * the SDK's unsupported-shape error. That is an SDK limitation, not a C-side
 * argument error; execute the query instead.
 *
 * @param query - pointer to LanceDBVectorQuery (NOT consumed)
 * @param verbose - if true, include additional detail in the plan
 * @param plan_out - pointer to receive the plan string
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return Error code indicating success or failure
 *
 * Caller must free the returned plan string with lancedb_free_string().
 * If error_message is provided and an error occurs, the caller must free
 * the error message with lancedb_free_string().
 */
LanceDBError lancedb_vector_query_explain_plan(
    const LanceDBVectorQuery* query,
    bool verbose,
    char** plan_out,
    char** error_message
);

/**
 * Execute the vector query and return an analyzed plan with runtime metrics
 *
 * ANALYZE executes an analyzable query plan to collect runtime metrics. It
 * reads the same data and has the same side effects as execution while
 * returning no rows. The query is not consumed and can still be executed.
 *
 * Embedded LanceDB executes a hybrid nearest-neighbour plus full-text query as
 * two searches and fuses their results. ANALYZE requires one analyzable SDK
 * plan, so it may return the same unsupported hybrid error as
 * lancedb_vector_query_explain_plan().
 *
 * On entry, *plan_out and the error message are cleared. On success, *plan_out
 * holds a NUL-terminated plan string; release it with lancedb_free_string(). On
 * failure it stays NULL and the error message is set when requested.
 *
 * @param query - pointer to LanceDBVectorQuery (NOT consumed)
 * @param plan_out - pointer to receive the plan string
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return Error code indicating success or failure
 *
 * If error_message is provided and an error occurs, the caller must free
 * the error message with lancedb_free_string().
 */
LanceDBError lancedb_vector_query_analyze_plan(
    const LanceDBVectorQuery* query,
    char** plan_out,
    char** error_message
);

/**
 * Execute query and return streaming result
 *
 * @param query - pointer to LanceDBQuery (consumed by this function)
 * @return Pointer to LanceDBQueryResult on success, NULL on failure
 *         Caller must free with lancedb_query_result_free()
 */
LanceDBQueryResult* lancedb_query_execute(LanceDBQuery* query);

/**
 * Execute vector query and return streaming result
 *
 * @param query - pointer to LanceDBVectorQuery (consumed by this function)
 * @return Pointer to LanceDBQueryResult on success, NULL on failure
 *         Caller must free with lancedb_query_result_free()
 */
LanceDBQueryResult* lancedb_vector_query_execute(LanceDBVectorQuery* query);

/**
 * Execute query and report why it could not start
 *
 * @param query - pointer to LanceDBQuery (consumed by this function on every path
 *                with a non-NULL pointer, including a rejected result pointer)
 * @param result_out - pointer to receive the streaming result
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return LANCEDB_SUCCESS with *result_out set, otherwise the SDK error code
 *
 * The SDK only hands over a result once it started the query, so a failure
 * carries the SDK status and its message instead of a NULL pointer.
 * On entry, *result_out and the error message are cleared.
 * Caller must free a returned result with lancedb_query_result_free().
 * If error_message is provided and an error occurs, the caller must free the
 * error message with lancedb_free_string().
 */
LanceDBError lancedb_query_execute_with_status(
    LanceDBQuery* query,
    LanceDBQueryResult** result_out,
    char** error_message
);

/**
 * Execute vector query and report why it could not start
 *
 * @param query - pointer to LanceDBVectorQuery (consumed by this function on every
 *                path with a non-NULL pointer, including a rejected result pointer)
 * @param result_out - pointer to receive the streaming result
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return LANCEDB_SUCCESS with *result_out set, otherwise the SDK error code
 *
 * Same contract as lancedb_query_execute_with_status().
 * On entry, *result_out and the error message are cleared.
 * Caller must free a returned result with lancedb_query_result_free().
 * If error_message is provided and an error occurs, the caller must free the
 * error message with lancedb_free_string().
 */
LanceDBError lancedb_vector_query_execute_with_status(
    LanceDBVectorQuery* query,
    LanceDBQueryResult** result_out,
    char** error_message
);

/**
 * Create a take query that selects rows by dataset offset
 *
 * Offsets are 0-indexed and relative to the current table version; they can
 * change when rows are added or deleted. The requested offsets are copied,
 * including duplicates. Without lancedb_take_query_preserve_order() the result
 * order is unspecified. Missing offsets are omitted.
 *
 * @param table - pointer to LanceDBTable
 * @param offsets - borrowed array of dataset offsets, or NULL when count is zero
 * @param count - number of offsets in the array
 * @param query_out - pointer to receive the TakeQuery builder
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return LANCEDB_SUCCESS with *query_out set, otherwise LANCEDB_INVALID_ARGUMENT
 *
 * The caller keeps ownership of offsets and may release or change it after this
 * call returns. On entry, *query_out and the error message are cleared. Free the
 * returned builder with lancedb_take_query_free() or consume it with
 * lancedb_take_query_execute(). If error_message is provided and an error
 * occurs, the caller must free the error message with lancedb_free_string().
 */
LanceDBError lancedb_table_take_offsets(
    const LanceDBTable* table,
    const uint64_t* offsets,
    size_t count,
    LanceDBTakeQuery** query_out,
    char** error_message
);

/**
 * Create a take query that selects rows by row ID
 *
 * Row IDs are relative to the current table version and can change after
 * compaction or updates. The SDK follows the requested order, including
 * repeated IDs, and omits missing or deleted IDs.
 *
 * @param table - pointer to LanceDBTable
 * @param row_ids - borrowed array of row IDs, or NULL when count is zero
 * @param count - number of row IDs in the array
 * @param query_out - pointer to receive the TakeQuery builder
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return LANCEDB_SUCCESS with *query_out set, otherwise LANCEDB_INVALID_ARGUMENT
 *
 * The caller keeps ownership of row_ids and may release or change it after this
 * call returns. On entry, *query_out and the error message are cleared. Free the
 * returned builder with lancedb_take_query_free() or consume it with
 * lancedb_take_query_execute(). If error_message is provided and an error
 * occurs, the caller must free the error message with lancedb_free_string().
 */
LanceDBError lancedb_table_take_row_ids(
    const LanceDBTable* table,
    const uint64_t* row_ids,
    size_t count,
    LanceDBTakeQuery** query_out,
    char** error_message
);

/**
 * Set the column projection of a take query
 *
 * Columns are returned in the requested order. A zero num_columns selects no
 * columns and keeps an empty result schema; columns may then be NULL. The names
 * are copied and are not borrowed after this call.
 *
 * @param query - pointer to LanceDBTakeQuery
 * @param columns - borrowed array of column names, or NULL when num_columns is zero
 * @param num_columns - number of column names in the array
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return Error code indicating success or failure
 *
 * If error_message is provided and an error occurs, the caller must free the
 * error message with lancedb_free_string().
 */
LanceDBError lancedb_take_query_select(
    LanceDBTakeQuery* query,
    const char* const* columns,
    size_t num_columns,
    char** error_message
);

/**
 * Ask a take query to return the `_rowid` metadata column
 *
 * The metadata column is appended to the projection. For an offset take it
 * carries the row ID, which is not the same as the requested dataset offset.
 *
 * @param query - pointer to LanceDBTakeQuery
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return Error code indicating success or failure
 *
 * If error_message is provided and an error occurs, the caller must free the
 * error message with lancedb_free_string().
 */
LanceDBError lancedb_take_query_with_row_id(
    LanceDBTakeQuery* query,
    char** error_message
);

/**
 * Restore offset-take occurrences in the exact requested order
 *
 * Row-ID takes already follow the requested order and ignore this flag. For an
 * offset take, the result follows the input order, including repeated offsets,
 * and missing offsets are omitted.
 *
 * @param query - pointer to LanceDBTakeQuery
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return Error code indicating success or failure
 *
 * If error_message is provided and an error occurs, the caller must free the
 * error message with lancedb_free_string().
 */
LanceDBError lancedb_take_query_preserve_order(
    LanceDBTakeQuery* query,
    char** error_message
);

/**
 * Execute a take query and report why it could not start
 *
 * @param query - pointer to LanceDBTakeQuery (consumed by this function on every
 *                path with a non-NULL pointer, including a rejected result pointer)
 * @param result_out - pointer to receive the streaming result
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return LANCEDB_SUCCESS with *result_out set, otherwise the SDK error code
 *
 * The returned result uses the same Arrow C Data and Arrow IPC conversions as
 * every other query result. On entry, *result_out and the error message are
 * cleared. Caller must free a returned result with lancedb_query_result_free().
 * If error_message is provided and an error occurs, the caller must free the
 * error message with lancedb_free_string().
 */
LanceDBError lancedb_take_query_execute(
    LanceDBTakeQuery* query,
    LanceDBQueryResult** result_out,
    char** error_message
);

/**
 * Free a TakeQuery
 *
 * @param query - pointer to LanceDBTakeQuery
 */
void lancedb_take_query_free(LanceDBTakeQuery* query);

/**
 * Convert query result to Arrow RecordBatch arrays
 *
 * @param result - pointer to LanceDBQueryResult. When result, result_arrays,
 *                 result_schema and count_out are all non-NULL, the result is
 *                 consumed on every path, including failure. When any of the
 *                 four is NULL, the call returns LANCEDB_INVALID_ARGUMENT and
 *                 does not consume a non-NULL result, so the caller can retry
 *                 it or free it with lancedb_query_result_free().
 * @param result_arrays - pointer to receive array of Arrow C ABI arrays
 * @param result_schema - pointer to receive single Arrow C ABI schema
 * @param count_out - pointer to receive number of result batches
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return LANCEDB_SUCCESS, the SDK error code when the stream fails, or
 *         LANCEDB_ARROW when the result schema cannot be represented through
 *         the Arrow C data interface
 *
 * On entry, non-NULL output slots and the error message are cleared. On
 * success, *count_out is the number of batches, and *result_arrays and
 * *result_schema hold the batches and their schema. An empty result succeeds
 * with both pointers NULL and *count_out zero. On failure, all three outputs
 * stay clear. The caller must free returned arrays with
 * lancedb_free_arrow_arrays() and the schema with lancedb_free_arrow_schema().
 * If error_message is provided and an error occurs, the caller must free the
 * error message with lancedb_free_string().
 */
LanceDBError lancedb_query_result_to_arrow(
    LanceDBQueryResult* result,
    struct FFI_ArrowArray*** result_arrays,
    struct FFI_ArrowSchema** result_schema,
    size_t* count_out,
    char** error_message
);

/**
 * Convert a query result into an Arrow IPC file
 *
 * @param result - pointer to LanceDBQueryResult (consumed by this function on every
 *                 path with a non-NULL pointer, including a rejected buffer pointer)
 * @param buffer_out - pointer to receive the file bytes
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return LANCEDB_SUCCESS with *buffer_out holding one complete IPC file, the SDK
 *         error code of a stream that failed, or LANCEDB_ARROW when encoding failed
 *
 * The file carries the projected schema and one record batch per batch the
 * result yielded, in order, including zero-row batches. A result without any
 * batch keeps the projected schema and holds no batch. The bytes are handed over
 * only after the file footer is written, so a failure leaves the buffer empty.
 * On entry, *buffer_out and the error message are cleared.
 * The caller must free the bytes with lancedb_ipc_buffer_free(). If
 * error_message is provided and an error occurs, the caller must free the error
 * message with lancedb_free_string().
 */
LanceDBError lancedb_query_result_to_ipc_file(
    LanceDBQueryResult* result,
    LanceDBIpcBuffer* buffer_out,
    char** error_message
);

/**
 * Free a Query
 *
 * @param query - pointer to LanceDBQuery
 */
void lancedb_query_free(LanceDBQuery* query);

/**
 * Free a VectorQuery
 *
 * @param query - pointer to LanceDBVectorQuery
 */
void lancedb_vector_query_free(LanceDBVectorQuery* query);

/**
 * Free a QueryResult
 *
 * @param result - pointer to LanceDBQueryResult
 */
void lancedb_query_result_free(LanceDBQueryResult* result);

/** @} */

/**
 * @defgroup api_arrow Arrow interoperability
 * @brief Arrow C Data, C Stream, and IPC readers, results, and buffers.
 * @{
 */

/**
 * Vector search using nearest_to with full result conversion
 *
 * @param table - pointer to LanceDBTable
 * @param vector - array of floats representing the query vector
 * @param dimension - dimension of the vector
 * @param limit - maximum number of results to return
 * @param column - vector column name (NULL for default "vector" column)
 * @param result_arrays - pointer to receive array of Arrow C ABI arrays
 * @param result_schema - pointer to receive single Arrow C ABI schema (shared by all arrays)
 * @param count_out - pointer to receive number of result batches
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return Error code indicating success or failure
 *         Caller must free arrays with lancedb_free_arrow_arrays() and schema with lancedb_free_arrow_schema()
 *
 * If error_message is provided and an error occurs, the caller must free
 * the error message with lancedb_free_string().
 */
LanceDBError lancedb_table_nearest_to(
    const LanceDBTable* table,
    const float* vector,
    size_t dimension,
    size_t limit,
    const char* column,
    struct FFI_ArrowArray*** result_arrays,
    struct FFI_ArrowSchema** result_schema,
    size_t* count_out,
    char** error_message
);

/**
 * Create a RecordBatchReader from Arrow C ABI structures
 *
 * @param array - pointer to FFI_ArrowArray containing record batch data
 * @param schema - pointer to FFI_ArrowSchema containing the schema
 * @param reader_out - pointer to receive the created reader
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return Error code indicating success or failure
 *
 * The schema is imported first. If that import fails, the array still belongs
 * to the caller and must be released. Once the schema imports, the array is
 * handed to Arrow's importer and the caller must not release it again, even when
 * a later batch conversion fails. The schema itself is only read and remains
 * owned by the caller. The caller is responsible for freeing the returned reader
 * with lancedb_record_batch_reader_free(). If error_message is provided and an
 * error occurs, the caller must free the error message with
 * lancedb_free_string().
 */
LanceDBError lancedb_record_batch_reader_from_arrow(
    const struct FFI_ArrowArray* array,
    const struct FFI_ArrowSchema* schema,
    LanceDBRecordBatchReader** reader_out,
    char** error_message
);

/**
 * Create a RecordBatchReader from an Arrow C Stream
 *
 * @param stream - pointer to an initialized struct ArrowArrayStream
 * @param reader_out - pointer to receive the created reader
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return Error code indicating success or failure
 *
 * The stream must provide the get_schema, get_next and release callbacks.
 * get_last_error is optional and may return NULL; its text must stay valid
 * until the next operation on the stream or release. Those callbacks are
 * checked before the stream is moved. If a required pointer or callback is
 * missing, the stream still belongs to the caller. After a successful move,
 * the binding owns the producer and releases it exactly once, including when
 * the schema cannot be acquired. The caller keeps ownership of the outer
 * struct storage, and the source struct's release callback is cleared, so it
 * must not be released again.
 *
 * Returns LANCEDB_INVALID_ARGUMENT for a NULL pointer or a missing required
 * callback, LANCEDB_INVALID_INPUT when get_schema reports an error, and
 * LANCEDB_ARROW when the returned schema cannot be imported.
 *
 * The imported batches must be representable as Arrow RecordBatches: the root
 * struct cannot have null rows. Nullable child fields, zero-row batches, and a
 * root validity bitmap with no nulls are accepted. A root with actual nulls is
 * reported as an Arrow error when that batch is consumed, before its child
 * columns are written.
 *
 * The stream, its schema and each returned ArrowArray are independently owned.
 * The producer must keep the schema alive until its schema release callback
 * runs and keep each array alive until that array's own release callback runs.
 * Arrays may be released after the stream or on a different thread; the schema
 * is likewise independent of the stream.
 *
 * The schema is acquired synchronously on the calling thread before this
 * function returns. No record batch is pulled at import time; batches are read
 * lazily when the reader is consumed by lancedb_table_create(),
 * lancedb_table_add() or lancedb_table_merge_insert(). Calls on one stream are
 * serialized, but callbacks may run on SDK worker threads, and release may run
 * on a worker or on the thread that frees the reader. A producer must therefore
 * allow its callbacks to be invoked from different threads over time, and keep
 * its private state alive until release.
 *
 * This binding does not collect all batches before writing. The pinned SDK may
 * still buffer depending on the backend: embedded create/add stream batches,
 * remote create encodes an IPC file before sending it, and remote merge may
 * collect the reader for retries.
 *
 * On entry, *reader_out and *error_message are cleared. Free the reader with
 * lancedb_record_batch_reader_free(); this also releases a reader that was
 * never consumed. If error_message is provided and an error occurs, the caller
 * must free the error message with lancedb_free_string().
 */
LanceDBError lancedb_record_batch_reader_from_arrow_stream(
    struct ArrowArrayStream* stream,
    LanceDBRecordBatchReader** reader_out,
    char** error_message
);

/**
 * Create a RecordBatchReader and/or an Arrow schema from an Arrow IPC file
 *
 * @param data - borrowed bytes of an Arrow IPC file (NULL only when len is 0)
 * @param len - number of bytes in data
 * @param reader_out - optional pointer to receive a reader over every batch (NULL to skip)
 * @param schema_out - optional pointer to receive the Arrow C ABI schema (NULL to skip)
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return Error code indicating success or failure
 *
 * At least one of reader_out and schema_out must not be NULL. The reader holds
 * every batch of the file, so several batches can be added with a single
 * lancedb_table_add() or merged with a single lancedb_table_merge_insert().
 * The schema alone is enough to create an empty table of the same shape.
 * The bytes are only borrowed: the caller may release them as soon as this
 * function returns. Every batch is decoded before returning, so zero bytes (also
 * a NULL pointer with len 0), an IPC stream, a truncated file or a corrupt batch
 * fail with LANCEDB_INVALID_INPUT and a decoding message before any table is
 * changed. A size that cannot describe the buffer, a file whose announced footer
 * does not fit in it, or a footer whose batches point outside it, are rejected
 * before the decoder allocates from those numbers.
 * On entry, both outputs and the error message are cleared.
 * The caller must free a returned reader with lancedb_record_batch_reader_free()
 * and a returned schema with lancedb_free_arrow_schema(). If error_message is
 * provided and an error occurs, the caller must free the error message with
 * lancedb_free_string().
 */
LanceDBError lancedb_record_batch_reader_from_ipc_file(
    const uint8_t* data,
    size_t len,
    LanceDBRecordBatchReader** reader_out,
    struct FFI_ArrowSchema** schema_out,
    char** error_message
);

/**
 * Free RecordBatchReader
 *
 * @param reader - pointer to LanceDBRecordBatchReader
 */
void lancedb_record_batch_reader_free(LanceDBRecordBatchReader* reader);

/**
 * Free Arrow schema
 *
 * @param schema - pointer to Arrow schema
 */
void lancedb_free_arrow_schema(FFI_ArrowSchema* schema);

/**
 * Free the bytes of an Arrow IPC file and reset the buffer
 *
 * @param buffer - pointer to LanceDBIpcBuffer, or NULL
 *
 * The buffer keeps both fields unchanged until this call. It reads them as they
 * are, frees the file bytes and zeroes the struct, so freeing an already freed or
 * empty buffer is a no-op.
 */
void lancedb_ipc_buffer_free(LanceDBIpcBuffer* buffer);

/** @} */

/**
 * @defgroup api_index Indexes and maintenance
 * @brief Create and inspect indexes, wait for builds, and optimize tables.
 * @{
 */

/**
 * Create a vector index on table columns
 *
 * @param table - pointer to LanceDBTable
 * @param columns - array of null-terminated C strings containing column names
 * @param num_columns - number of columns in the array
 * @param index_type - type of vector index to create
 * @param config - pointer to LanceDBVectorIndexConfig or NULL for defaults
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return Error code indicating success or failure
 *
 * If error_message is provided and an error occurs, the caller must free
 * the error message with lancedb_free_string().
 */
LanceDBError lancedb_table_create_vector_index(
    const LanceDBTable* table,
    const char* const* columns,
    size_t num_columns,
    LanceDBIndexType index_type,
    const LanceDBVectorIndexConfig* config,
    char** error_message
);

/**
 * Create a scalar index on table columns
 *
 * @param table - pointer to LanceDBTable
 * @param columns - array of null-terminated C strings containing column names
 * @param num_columns - number of columns in the array
 * @param index_type - type of scalar index to create
 * @param config - pointer to LanceDBScalarIndexConfig or NULL for defaults
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return Error code indicating success or failure
 *
 * If error_message is provided and an error occurs, the caller must free
 * the error message with lancedb_free_string().
 */
LanceDBError lancedb_table_create_scalar_index(
    const LanceDBTable* table,
    const char* const* columns,
    size_t num_columns,
    LanceDBIndexType index_type,
    const LanceDBScalarIndexConfig* config,
    char** error_message
);

/**
 * Create a full-text search index on table columns
 *
 * @param table - pointer to LanceDBTable
 * @param columns - array of null-terminated C strings containing column names
 * @param num_columns - number of columns in the array
 * @param config - pointer to LanceDBFtsIndexConfig or NULL for defaults
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return Error code indicating success or failure
 *
 * If error_message is provided and an error occurs, the caller must free
 * the error message with lancedb_free_string().
 */
LanceDBError lancedb_table_create_fts_index(
    const LanceDBTable* table,
    const char* const* columns,
    size_t num_columns,
    const LanceDBFtsIndexConfig* config,
    char** error_message
);

/**
 * Return the default index creation options
 *
 * Every optional field is unset, so the SDK defaults apply.
 */
LanceDBIndexOptions lancedb_index_options_default(void);

/**
 * Create any supported index with a custom name, replace choice, creation wait
 * and per-type tuning parameters
 *
 * @param table - pointer to LanceDBTable
 * @param columns - array of null-terminated C strings containing column names
 * @param num_columns - number of columns in the array
 * @param index_type - LanceDBIndexType value selecting the index kind
 * @param options - pointer to LanceDBIndexOptions or NULL for the SDK defaults
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return Error code indicating success or failure
 *
 * The index type selects which tuning fields are used: IVF types use the IVF
 * fields, PQ and HNSW types use their additional fields, FTS uses the fts_*
 * fields, and scalar types use only name, replace and wait_timeout_ms. A NULL
 * options pointer behaves like lancedb_index_options_default().
 * LANCEDB_INDEX_AUTO passes the SDK's Auto index through; it is not remapped to
 * a concrete index type.
 *
 * wait_timeout_ms is an SDK polling budget for index creation, not a wall-clock
 * guarantee and not a cancellation. Zero does not wait. Native tables build
 * synchronously and ignore the value. A remote server that returns a job ID
 * waits for that job with the SDK timeout; reaching the budget returns
 * LANCEDB_TIMEOUT but does not cancel the submitted build. A legacy remote
 * server without a job ID polls the named index (custom name or SDK default
 * name) instead; that poll reads the index's current num_unindexed_rows, so
 * concurrent writes can extend it and a same-name prior index can satisfy it
 * before this submission finishes. The SDK submits the create/replace request
 * before checking its two-hour budget cap, so an over-cap error can arrive
 * after the server accepted the request.
 *
 * Values outside the documented ranges, invalid UTF-8, unsupported index types
 * and SDK errors are returned through error_message when provided.
 *
 * If error_message is provided and an error occurs, the caller must free
 * the error message with lancedb_free_string().
 */
LanceDBError lancedb_table_create_index(
    const LanceDBTable* table,
    const char* const* columns,
    size_t num_columns,
    LanceDBIndexType index_type,
    const LanceDBIndexOptions* options,
    char** error_message
);

/**
 * Create any supported index with explicit train and exist_ok builder choices
 *
 * Takes the same table, columns, index type, tuning options and error handling
 * as lancedb_table_create_index(). train and exist_ok use -1 for the SDK default,
 * 0 for false and 1 for true. The SDK defaults are train = true, exist_ok = false
 * and replace = true.
 *
 * On an embedded table, train = 0 creates an empty index: its statistics report
 * zero indexed rows and every table row unindexed. exist_ok = 1 preserves an
 * existing index with the requested name, or with the SDK default name when the
 * options name is empty or NULL, and returns success without rebuilding it. It
 * takes precedence over replace = 1, so no replacement or training happens.
 *
 * A remote LanceDB Enterprise endpoint cannot honor train = 0 as an empty local
 * index. The SDK does not send train, logs a warning, and the endpoint creates
 * the index and populates it in the background. For exist_ok = 1, the SDK sends
 * an exist_ok query parameter and disables replace in that request.
 *
 * The SDK documents train as not yet supported for vector indices. A NULL options
 * pointer behaves like lancedb_index_options_default(). Values outside -1, 0 and
 * 1, other invalid options and SDK errors are returned through error_message
 * when provided. wait_timeout_ms and all other behavior are the same as
 * lancedb_table_create_index().
 *
 * If error_message is provided and an error occurs, the caller must free the
 * error message with lancedb_free_string().
 */
LanceDBError lancedb_table_create_index_with_builder_options(
    const LanceDBTable* table,
    const char* const* columns,
    size_t num_columns,
    LanceDBIndexType index_type,
    const LanceDBIndexOptions* options,
    int train,
    int exist_ok,
    char** error_message
);

/**
 * List all indices on the table
 *
 * @param table - pointer to LanceDBTable
 * @param indices_out - pointer to receive array of index info strings
 * @param count_out - pointer to receive the count of indices
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return Error code indicating success or failure
 *
 * The caller is responsible for freeing the returned strings and array
 * using lancedb_free_index_list(). If error_message is provided and an error
 * occurs, the caller must free the error message with lancedb_free_string().
 */
LanceDBError lancedb_table_list_indices(
    const LanceDBTable* table,
    char*** indices_out,
    size_t* count_out,
    char** error_message
);

/**
 * Prewarm a named index
 *
 * Loads the index into memory and caches it, so the first query does not pay
 * for the load.
 *
 * @param table - pointer to LanceDBTable
 * @param index_name - null-terminated C string containing the index name
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return LANCEDB_SUCCESS if the index was prewarmed, LANCEDB_INDEX_NOT_FOUND if the
 *         index does not exist, other error codes on failure
 *
 * If error_message is provided and an error occurs, the caller must free
 * the error message with lancedb_free_string().
 */
LanceDBError lancedb_table_prewarm_index(
    const LanceDBTable* table,
    const char* index_name,
    char** error_message
);

/**
 * Drop an index
 *
 * @param table - pointer to LanceDBTable
 * @param index_name - null-terminated C string containing the index name
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return Error code indicating success or failure
 *
 * If error_message is provided and an error occurs, the caller must free
 * the error message with lancedb_free_string().
 */
LanceDBError lancedb_table_drop_index(
    const LanceDBTable* table,
    const char* index_name,
    char** error_message
);

/**
 * Optimize table (rebuild indices and compact files)
 *
 * @param table - pointer to LanceDBTable
 * @param optimize_type - type of optimization to perform
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return Error code indicating success or failure
 *
 * If error_message is provided and an error occurs, the caller must free
 * the error message with lancedb_free_string().
 */
LanceDBError lancedb_table_optimize(
    const LanceDBTable* table,
    LanceDBOptimizeType optimize_type,
    char** error_message
);

/**
 * Compaction option fields of LanceDBCompactionOptions
 *
 * Every bit makes the matching field an override. A field whose bit is not set
 * keeps the LanceDB default for compacting a table, so an omitted option stays
 * distinct from a value that happens to equal the default.
 */
typedef enum {
    LANCEDB_COMPACTION_TARGET_ROWS_PER_FRAGMENT = 1 << 0,
    LANCEDB_COMPACTION_MAX_ROWS_PER_GROUP = 1 << 1,
    LANCEDB_COMPACTION_MAX_BYTES_PER_FILE = 1 << 2,
    LANCEDB_COMPACTION_MATERIALIZE_DELETIONS = 1 << 3,
    LANCEDB_COMPACTION_MATERIALIZE_DELETIONS_THRESHOLD = 1 << 4,
    LANCEDB_COMPACTION_NUM_THREADS = 1 << 5,
    LANCEDB_COMPACTION_BATCH_SIZE = 1 << 6
} LanceDBCompactionField;

/**
 * Compaction overrides, used by lancedb_table_optimize_with_action()
 *
 * The fields follow the compaction options of the LanceDB SDK. set_fields
 * selects the fields that are read; every other field keeps the SDK default.
 * Sizes are checked against size_t before the compaction starts.
 *
 * An Enterprise table plans compaction on the server. It accepts only the
 * overrides the server supports and rejects the others with
 * LANCEDB_NOT_SUPPORTED before submission. It also treats a
 * target_rows_per_fragment equal to the SDK default (1048576) as omitted, even
 * when its bit is set, so a table-level target can win over that value.
 */
typedef struct {
    uint32_t set_fields;                    // Bitmask of LanceDBCompactionField values (0 = defaults)
    uint64_t target_rows_per_fragment;      // Target number of rows per fragment
    uint64_t max_rows_per_group;            // Maximum number of rows per group in a written fragment
    uint64_t max_bytes_per_file;            // Maximum number of bytes per output data file
    int materialize_deletions;              // Rewrite fragments with deletions (1 = true, 0 = false)
    float materialize_deletions_threshold;  // Fraction of deleted rows that triggers materializing deletions
    uint64_t num_threads;                   // Number of compaction tasks to run in parallel
    uint64_t batch_size;                    // Batch size used when scanning the input fragments
} LanceDBCompactionOptions;

/**
 * Prune option fields of LanceDBPruneOptions
 *
 * Every bit makes the matching field an override. A field whose bit is not set
 * keeps the LanceDB default, so an omitted option stays distinct from an explicit
 * false or an explicit age.
 */
typedef enum {
    LANCEDB_PRUNE_OLDER_THAN = 1 << 0,
    LANCEDB_PRUNE_DELETE_UNVERIFIED = 1 << 1,
    LANCEDB_PRUNE_ERROR_IF_TAGGED_OLD_VERSIONS = 1 << 2
} LanceDBPruneField;

/**
 * Version pruning overrides, used by lancedb_table_optimize_with_action()
 *
 * The fields follow the prune action of the LanceDB SDK. set_fields selects the
 * fields that are read; every other field keeps the SDK default.
 *
 * older_than_nanos is the minimum age of a prunable version, in nanoseconds, so
 * a sub-second age keeps its exact length. The age must not be negative. When the
 * bit is not set, the SDK default age applies. When it is set, zero means a
 * cutoff at the current time: every version older than now is removed, while the
 * latest version and protected versions still survive.
 *
 * delete_unverified permits deleting files that are newer than the SDK's safety
 * margin for in-progress transactions; setting it can corrupt a dataset that
 * another process is still writing, so leave it unset unless that is safe.
 * Tagged versions are retained by default; error_if_tagged_old_versions makes
 * the prune fail instead of silently retaining them.
 */
typedef struct {
    uint32_t set_fields;                    // Bitmask of LanceDBPruneField values (0 = defaults)
    int64_t older_than_nanos;               // Minimum age of a prunable version in nanoseconds (>= 0)
    int delete_unverified;                  // Delete unverified files (1 = true, 0 = false)
    int error_if_tagged_old_versions;       // Fail on tagged old versions (1 = true, 0 = false)
} LanceDBPruneOptions;

/**
 * One configurable optimize action
 *
 * action selects the optimization, and the option struct of that action carries
 * its overrides. The option struct of another action is ignored.
 *
 * action is the raw integer value of a LanceDBOptimizeType enumerator. It is an
 * int rather than the enum type so that this entry point receives the value a
 * caller passes, including one outside the enumerators, and rejects it with
 * LANCEDB_INVALID_ARGUMENT instead of an undefined enum conversion.
 */
typedef struct {
    int action;                             // One of the LanceDBOptimizeType values
    LanceDBCompactionOptions compaction;    // Overrides used by LANCEDB_OPTIMIZE_COMPACT
    LanceDBPruneOptions prune;              // Overrides used by LANCEDB_OPTIMIZE_PRUNE
} LanceDBOptimizeConfig;

/**
 * Fragments and files that compaction rewrote
 */
typedef struct {
    size_t fragments_removed;   // Fragments that compaction removed
    size_t fragments_added;     // Fragments that compaction added
    size_t files_removed;       // Files that compaction removed, including deletion files
    size_t files_added;         // Files that compaction added, including deletion files
} LanceDBCompactionMetrics;

/**
 * Files and versions that pruning removed
 */
typedef struct {
    uint64_t bytes_removed;     // Bytes of the removed files
    uint64_t old_versions;      // Old dataset versions that were removed
} LanceDBRemovalStats;

/**
 * Statistics of one optimize action
 *
 * The SDK reports compaction and pruning statistics only for an action that ran
 * them, so each group carries its own presence flag. The values of a group whose
 * flag is 0 are zero.
 */
typedef struct {
    int has_compaction;                 // 1 when the action reported compaction metrics
    LanceDBCompactionMetrics compaction; // Compaction metrics; valid only when has_compaction is 1
    int has_prune;                      // 1 when the action reported pruning metrics
    LanceDBRemovalStats prune;          // Pruning metrics; valid only when has_prune is 1
} LanceDBOptimizeStats;

/**
 * Optimize table with a configurable action and return its statistics
 *
 * Unlike lancedb_table_optimize(), which runs each action with its defaults and
 * discards the statistics, this entry point applies the action's options and
 * reports what the SDK did. LANCEDB_OPTIMIZE_ALL runs compaction, pruning and
 * index optimization with their defaults, exactly like
 * lancedb_table_optimize(LANCEDB_OPTIMIZE_ALL).
 *
 * The SDK decides which actions and options a table backend supports: an
 * Enterprise table only hosts LANCEDB_OPTIMIZE_COMPACT and returns
 * LANCEDB_NOT_SUPPORTED for the other actions. Its compaction returns once the
 * server accepts the job, with zeroed placeholder metrics while the compaction
 * may still be running; a local table reports the work it did.
 *
 * Invalid input is rejected with LANCEDB_INVALID_ARGUMENT before any optimization
 * runs: an action value that is not a LanceDBOptimizeType enumerator, an unknown
 * set_fields bit, a size that does not fit in size_t, a boolean other than 0 and 1,
 * or a negative prune age. On error, stats_out is left unchanged.
 *
 * @param table - pointer to LanceDBTable
 * @param config - pointer to LanceDBOptimizeConfig, which must not be NULL
 * @param stats_out - pointer to receive the statistics, which must not be NULL
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return Error code indicating success or failure
 *
 * If error_message is provided and an error occurs, the caller must free
 * the error message with lancedb_free_string().
 */
LanceDBError lancedb_table_optimize_with_action(
    const LanceDBTable* table,
    const LanceDBOptimizeConfig* config,
    LanceDBOptimizeStats* stats_out,
    char** error_message
);

/**
 * Free index list array returned by lancedb_table_list_indices
 *
 * @param indices - array of string pointers returned by lancedb_table_list_indices
 * @param count - number of strings in the array
 */
void lancedb_free_index_list(char** indices, size_t count);

/**
 * Index statistics structure
 */
typedef struct {
    size_t num_indexed_rows;     // Rows covered by the index
    size_t num_unindexed_rows;   // Rows not yet indexed
    unsigned int num_indices;    // Number of index parts (0 if unknown)
} LanceDBIndexStats;

/**
 * Get statistics for a named index on the table
 *
 * @param table - pointer to LanceDBTable
 * @param index_name - null-terminated C string containing the index name
 * @param stats_out - pointer to receive the index statistics
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return LANCEDB_SUCCESS if stats retrieved, LANCEDB_INDEX_NOT_FOUND if index doesn't exist
 *
 * The stats are read directly from the LanceDB manifest (no external state needed).
 * If error_message is provided and an error occurs, the caller must free
 * the error message with lancedb_free_string().
 */
LanceDBError lancedb_table_index_stats(
    const LanceDBTable* table,
    const char* index_name,
    LanceDBIndexStats* stats_out,
    char** error_message
);

/**
 * Statistics of a single index, as reported by the Rust SDK
 *
 * The strings are borrowed from the result and are read-only: do not modify them,
 * write through them or free them individually. They stay valid until the result is
 * released with lancedb_free_index_statistics(), and the whole result must be treated
 * as read-only.
 *
 * The values the SDK reports as optional keep their presence: distance_type is NULL
 * when the SDK reports no distance metric, and has_num_indices tells whether
 * num_indices was reported. index_type is the SDK type name, for example "IVF_PQ",
 * "BTREE" or "FTS".
 */
typedef struct {
    size_t num_indexed_rows;    // Rows covered by the index
    size_t num_unindexed_rows;  // Rows not covered by the index yet
    const char* index_type;     // SDK index type name, borrowed from the result
    const char* distance_type;  // SDK distance metric name, borrowed from the result (NULL if none)
    unsigned int num_indices;   // Number of parts; valid only when has_num_indices is true
    bool has_num_indices;       // Whether the SDK reported the number of parts
} LanceDBIndexStatistics;

/**
 * Get the statistics the Rust SDK reports for a named index
 *
 * Unlike lancedb_table_index_stats(), which returns only the row counts, this entry
 * point returns the complete SDK result with its optional values.
 *
 * @param table - pointer to LanceDBTable
 * @param index_name - null-terminated C string containing the index name
 * @param stats_out - pointer to receive the read-only statistics, or NULL when the
 *                    index does not exist
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return LANCEDB_SUCCESS when the statistics were read, including when the index
 *         does not exist, other error codes on failure
 *
 * On success, *stats_out is NULL when the index does not exist and points to the
 * read-only result otherwise, so a missing index is not an error. On failure, *stats_out
 * is NULL as well, and error_message carries the reason when requested.
 *
 * The result and the strings it borrows are owned by the library. Do not modify the
 * result, write through the strings or free them individually; release the whole result
 * with lancedb_free_index_statistics() once it is no longer used, and release it only
 * once.
 * If error_message is provided and an error occurs, the caller must free
 * the error message with lancedb_free_string().
 */
LanceDBError lancedb_table_index_statistics(
    const LanceDBTable* table,
    const char* index_name,
    const LanceDBIndexStatistics** stats_out,
    char** error_message
);

/**
 * Release the statistics returned by lancedb_table_index_statistics
 *
 * @param stats - statistics returned by lancedb_table_index_statistics
 *
 * The result and the strings it borrows become invalid. Do not free the strings
 * individually and do not release the same result twice.
 * Safe to call with a NULL pointer.
 */
void lancedb_free_index_statistics(const LanceDBIndexStatistics* stats);

/**
 * Wait until the named indices are fully indexed
 *
 * Blocks the calling thread and polls the index statistics until every named index
 * reports no unindexed rows, or until the budget runs out. Waiting does not build or
 * extend an index, so an index that does not cover all the rows of the table never
 * becomes fully indexed by waiting: use the index creation or optimization functions to
 * (re)build it, and wait to learn when the build has committed.
 *
 * The names are waited on in the same call, and every one of them must be an existing
 * index. Waiting for all the indices of a table is not part of this function: pass the
 * names returned by lancedb_table_list_indices(). There is no cancellation.
 *
 * @param table - pointer to LanceDBTable
 * @param index_names - array of null-terminated C strings containing index names
 * @param num_index_names - number of names in the array, at least 1
 * @param timeout_ms - polling budget in milliseconds, greater than 0. The SDK starts a
 *   polling round while less than this much time has elapsed, sleeps one second between
 *   rounds, and caps the budget at two hours; a larger budget is rejected with the
 *   maximum the SDK reports
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return LANCEDB_SUCCESS once every named index is fully indexed, LANCEDB_TIMEOUT when
 *   the budget runs out before that, LANCEDB_INVALID_ARGUMENT for invalid arguments, or
 *   the error code of the SDK failure
 *
 * The budget decides whether another polling round starts; it is not a wall-clock
 * bound. The listing and statistics requests of a round, and the diagnostics the SDK
 * reads after the last one, finish after the budget has run out, and the call does not
 * interrupt them.
 *
 * If error_message is provided and an error occurs, the caller must free
 * the error message with lancedb_free_string().
 */
LanceDBError lancedb_table_wait_for_index(
    const LanceDBTable* table,
    const char* const* index_names,
    size_t num_index_names,
    uint64_t timeout_ms,
    char** error_message
);

/** @} */

/**
 * @addtogroup api_arrow
 * @{
 */

/**
 * Free Arrow arrays returned by vector search functions
 *
 * @param arrays - array of Arrow C ABI array pointers
 * @param count - number of arrays
 */
void lancedb_free_arrow_arrays(
    struct FFI_ArrowArray** arrays,
    size_t count
);

/**
 * Free one Arrow C ABI array returned by a blob fetch function
 *
 * @param array - Arrow C ABI array pointer returned by lancedb_table_fetch_blobs()
 *                or lancedb_table_fetch_blob_ranges()
 */
void lancedb_free_arrow_array(FFI_ArrowArray* array);

/** @} */

/**
 * @addtogroup api_index
 * @{
 */

/**
 * Information about one index on a table
 *
 * The strings and arrays are owned by the result and are valid until it is freed
 * with lancedb_free_index_info().
 */
typedef struct {
    const char* const name;           // Name of the index as assigned by LanceDB
    const char* const* const columns; // Ordered column names covered by the index
    const size_t num_columns;         // Number of entries in columns
    const char* const index_type;     // SDK index type, for example "IVF_PQ", "BTREE" or "FTS"
} LanceDBIndexInfo;

/**
 * List every index on the table with its name, columns and index type
 *
 * Unlike lancedb_table_list_indices(), which returns only the names, this entry
 * point returns the metadata required by callers that inspect the indices. The
 * index type is the SDK's type name and is preserved for types this library does
 * not know about.
 *
 * @param table - pointer to LanceDBTable
 * @param indices_out - pointer to receive the array of index information
 * @param count_out - pointer to receive the number of indices
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return Error code indicating success or failure
 *
 * On success with no indices, count_out is 0 and indices_out is NULL.
 * The caller must free the returned array using lancedb_free_index_info().
 * If error_message is provided and an error occurs, the caller must free
 * the error message with lancedb_free_string().
 */
LanceDBError lancedb_table_list_indices_info(
    const LanceDBTable* table,
    const LanceDBIndexInfo** indices_out,
    size_t* count_out,
    char** error_message
);

/**
 * Free index information array returned by lancedb_table_list_indices_info
 *
 * @param indices - array of LanceDBIndexInfo structs returned by lancedb_table_list_indices_info
 * @param count - number of entries in the array
 *
 * Safe to call with a NULL pointer.
 */
void lancedb_free_index_info(const LanceDBIndexInfo* indices, size_t count);

/** @} */

/**
 * @defgroup api_history Versions, tags, branches, and metadata
 * @brief Inspect and change table history, branches, and key-value metadata.
 * @{
 */

/**
 * List all versions of the table
 *
 * @param table - pointer to LanceDBTable
 * @param versions_out - pointer to receive the array of LanceDBVersion structs
 * @param metadata_out - optional pointer to receive per-version metadata (NULL to skip)
 * @param count_out - pointer to receive the number of versions
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return Error code indicating success or failure
 *
 * On success, versions_out will point to an array of LanceDBVersion structs.
 * If metadata_out is non-NULL, it will point to a parallel array of
 * LanceDBVersionMetadata structs (one per version).
 * The caller must free the returned arrays using lancedb_free_versions()
 * and lancedb_free_version_metadata() respectively.
 * If error_message is provided and an error occurs, the caller must free
 * the error message with lancedb_free_string().
 */
LanceDBError lancedb_table_list_versions(
    const LanceDBTable* table,
    LanceDBVersion** versions_out,
    LanceDBVersionMetadata** metadata_out,
    size_t* count_out,
    char** error_message
);

/**
 * Check out a specific version of the table
 *
 * @param table - pointer to LanceDBTable
 * @param version - version number to check out
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return Error code indicating success or failure
 *
 * The table becomes a read-only view of the requested version until
 * lancedb_table_checkout_latest() or lancedb_table_restore() is called.
 * Other table handles are not affected.
 * If error_message is provided and an error occurs, the caller must free
 * the error message with lancedb_free_string().
 */
LanceDBError lancedb_table_checkout(
    const LanceDBTable* table,
    uint64_t version,
    char** error_message
);

/**
 * Check out the version referenced by a tag
 *
 * @param table - pointer to LanceDBTable
 * @param tag - null-terminated C string containing the tag name
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return Error code indicating success or failure
 *
 * The table becomes a read-only view of the tagged version until
 * lancedb_table_checkout_latest() or lancedb_table_restore() is called.
 * Other table handles are not affected.
 * If error_message is provided and an error occurs, the caller must free
 * the error message with lancedb_free_string().
 */
LanceDBError lancedb_table_checkout_tag(
    const LanceDBTable* table,
    const char* tag,
    char** error_message
);

/**
 * Return the table to the latest version
 *
 * @param table - pointer to LanceDBTable
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return Error code indicating success or failure
 *
 * This drops any version or tag checkout and resumes tracking the latest
 * table version.
 * If error_message is provided and an error occurs, the caller must free
 * the error message with lancedb_free_string().
 */
LanceDBError lancedb_table_checkout_latest(
    const LanceDBTable* table,
    char** error_message
);

/**
 * Restore the table to the currently checked out version
 *
 * @param table - pointer to LanceDBTable
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return Error code indicating success or failure
 *
 * This promotes the checked out version to the latest version. It fails when
 * the table has not been checked out. On success the table tracks the latest
 * version again.
 * If error_message is provided and an error occurs, the caller must free
 * the error message with lancedb_free_string().
 */
LanceDBError lancedb_table_restore(
    const LanceDBTable* table,
    char** error_message
);

/**
 * List all tags of the table
 *
 * @param table - pointer to LanceDBTable
 * @param tags_out - pointer to receive the read-only array of LanceDBTag structs
 * @param count_out - pointer to receive the number of tags
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return Error code indicating success or failure
 *
 * On success, tags_out points to a read-only array of LanceDBTag structs and
 * count_out receives its length. An empty result sets tags_out to NULL and
 * count_out to zero. The array and its strings are owned by the library and
 * must not be modified or freed individually. Free the whole array with
 * lancedb_free_tags() using the returned count.
 * If error_message is provided and an error occurs, the caller must free
 * the error message with lancedb_free_string().
 */
LanceDBError lancedb_table_list_tags(
    const LanceDBTable* table,
    const LanceDBTag** tags_out,
    size_t* count_out,
    char** error_message
);

/**
 * Get the version referenced by a tag
 *
 * @param table - pointer to LanceDBTable
 * @param tag - null-terminated C string containing the tag name
 * @param version_out - pointer to receive the tag's version
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return Error code indicating success or failure
 *
 * On success, version_out is set to the version referenced by the tag. On
 * failure, version_out is left unchanged.
 * If error_message is provided and an error occurs, the caller must free
 * the error message with lancedb_free_string().
 */
LanceDBError lancedb_table_get_tag_version(
    const LanceDBTable* table,
    const char* tag,
    uint64_t* version_out,
    char** error_message
);

/**
 * Create a tag for a table version
 *
 * @param table - pointer to LanceDBTable
 * @param tag - null-terminated C string containing the tag name
 * @param version - version number the tag references
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return Error code indicating success or failure
 *
 * Tag creation is metadata-only and does not create a new table version.
 * The operation fails when the tag already exists or the version is unknown.
 * If error_message is provided and an error occurs, the caller must free
 * the error message with lancedb_free_string().
 */
LanceDBError lancedb_table_create_tag(
    const LanceDBTable* table,
    const char* tag,
    uint64_t version,
    char** error_message
);

/**
 * Delete a tag from the table
 *
 * @param table - pointer to LanceDBTable
 * @param tag - null-terminated C string containing the tag name
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return Error code indicating success or failure
 *
 * Tag deletion is metadata-only and does not create a new table version.
 * The operation fails when the tag does not exist.
 * If error_message is provided and an error occurs, the caller must free
 * the error message with lancedb_free_string().
 */
LanceDBError lancedb_table_delete_tag(
    const LanceDBTable* table,
    const char* tag,
    char** error_message
);

/**
 * Update an existing tag to reference another table version
 *
 * @param table - pointer to LanceDBTable
 * @param tag - null-terminated C string containing the tag name
 * @param version - version number the tag should reference
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return Error code indicating success or failure
 *
 * Tag updates are metadata-only and do not create a new table version.
 * The operation fails when the tag does not exist or the version is unknown.
 * If error_message is provided and an error occurs, the caller must free
 * the error message with lancedb_free_string().
 */
LanceDBError lancedb_table_update_tag(
    const LanceDBTable* table,
    const char* tag,
    uint64_t version,
    char** error_message
);

/**
 * Create a new branch and return a table handle scoped to it
 *
 * @param table - pointer to LanceDBTable; the handle keeps its own branch
 * @param name - null-terminated C string containing the new branch name
 * @param source - source reference for the branch
 * @param table_out - pointer to receive the new branch table handle
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return Error code indicating success or failure
 *
 * The returned handle is independent of the source handle: writes through it
 * do not change the source branch, and the source handle stays usable. The
 * caller must free the returned handle with lancedb_table_free().
 * If error_message is provided and an error occurs, the caller must free
 * the error message with lancedb_free_string().
 */
LanceDBError lancedb_table_create_branch(
    const LanceDBTable* table,
    const char* name,
    const LanceDBBranchSource* source,
    LanceDBTable** table_out,
    char** error_message
);

/**
 * Check out an existing branch and return a table handle scoped to it
 *
 * @param table - pointer to LanceDBTable; the handle keeps its own branch
 * @param name - null-terminated C string containing the branch name
 * @param options - optional version selection for the checkout
 * @param table_out - pointer to receive the new branch table handle
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return Error code indicating success or failure
 *
 * Without a version the returned handle tracks the branch head and stays
 * writable. With a version the returned handle is pinned to that branch
 * version and is read-only. The original handle is not modified and stays
 * usable. The caller must free the returned handle with lancedb_table_free().
 * If error_message is provided and an error occurs, the caller must free
 * the error message with lancedb_free_string().
 */
LanceDBError lancedb_table_checkout_branch(
    const LanceDBTable* table,
    const char* name,
    const LanceDBBranchCheckoutOptions* options,
    LanceDBTable** table_out,
    char** error_message
);

/**
 * List the branches of the table
 *
 * @param table - pointer to LanceDBTable
 * @param branches_out - pointer to receive the read-only array of LanceDBBranch structs
 * @param count_out - pointer to receive the number of branches
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return Error code indicating success or failure
 *
 * On success, branches_out points to a read-only array of LanceDBBranch structs
 * sorted by name and count_out receives its length. An empty result sets
 * branches_out to NULL and count_out to zero. The array, its strings, and its
 * metadata arrays are owned by the library and must not be modified or freed
 * individually. Free the whole result with lancedb_free_branches() using the
 * returned count.
 * If error_message is provided and an error occurs, the caller must free
 * the error message with lancedb_free_string().
 */
LanceDBError lancedb_table_list_branches(
    const LanceDBTable* table,
    const LanceDBBranch** branches_out,
    size_t* count_out,
    char** error_message
);

/**
 * Free branch metadata returned by lancedb_table_list_branches
 *
 * @param branches - array of LanceDBBranch structs returned by lancedb_table_list_branches
 * @param count - number of entries, exactly as returned by lancedb_table_list_branches
 *
 * The library owns the array, every string and every metadata array. Callers
 * must not modify entries or free them individually, and must pass the count
 * returned with the array. Safe to call with a NULL pointer.
 */
void lancedb_free_branches(const LanceDBBranch* branches, size_t count);

/**
 * Delete a branch from the table
 *
 * @param table - pointer to LanceDBTable
 * @param name - null-terminated C string containing the branch name
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return Error code indicating success or failure
 *
 * Existing handles are not modified by the deletion. If error_message is
 * provided and an error occurs, the caller must free the error message with
 * lancedb_free_string().
 */
LanceDBError lancedb_table_delete_branch(
    const LanceDBTable* table,
    const char* name,
    char** error_message
);

/**
 * Diff a branch against main on a remote table
 *
 * @param table - pointer to LanceDBTable
 * @param from_branch - null-terminated C string containing the branch to compare
 * @param diff_out - pointer to receive the branch diff
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return Error code indicating success or failure
 *
 * The SDK decides whether the table supports branch diffs. Native (embedded)
 * tables return LANCEDB_NOT_SUPPORTED with the SDK's message unchanged.
 *
 * On entry, *diff_out and the error message are cleared. The result is
 * published only on success, so a failure leaves *diff_out NULL. The result
 * owns every string and nested array in SDK response order; free the whole
 * result once with lancedb_free_branch_diff(). Do not free nested pointers
 * individually. If error_message is provided and an error occurs, the caller
 * must free the error message with lancedb_free_string().
 */
LanceDBError lancedb_table_diff_branch(
    const LanceDBTable* table,
    const char* from_branch,
    LanceDBBranchDiff** diff_out,
    char** error_message
);

/**
 * Cherry-pick a branch onto main, or preview it with a dry run
 *
 * @param table - pointer to LanceDBTable
 * @param from_branch - null-terminated C string containing the branch to cherry-pick
 * @param dry_run - non-zero to request a preview without applying the cherry-pick
 * @param result_out - pointer to receive the cherry-pick result
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return Error code indicating success or failure
 *
 * The SDK decides whether the table supports cherry-picking. Native (embedded)
 * tables return LANCEDB_NOT_SUPPORTED with the SDK's message unchanged.
 *
 * An HTTP 409 that the SDK decodes as a failed cherry-pick result returns
 * LANCEDB_SUCCESS with LANCEDB_CHERRY_PICK_STATUS_FAILED in the result.
 * Converting a response to C can also fail, for example if an SDK string
 * contains an embedded NUL. For a non-dry run, the server may already have
 * applied the change when an error is returned. Check the table state before
 * retrying.
 *
 * On entry, *result_out and the error message are cleared. The result is
 * published only on success, so a failure leaves *result_out NULL. The result owns every string and nested array; free the
 * whole result once with lancedb_free_cherry_pick_result(). Do not free nested
 * pointers individually or pass its embedded diff to
 * lancedb_free_branch_diff(). If error_message is provided and an error
 * occurs, the caller must free the error message with lancedb_free_string().
 */
LanceDBError lancedb_table_cherry_pick(
    const LanceDBTable* table,
    const char* from_branch,
    int dry_run,
    LanceDBCherryPickResult** result_out,
    char** error_message
);

/**
 * Free a branch diff returned by lancedb_table_diff_branch
 *
 * @param diff - pointer returned by lancedb_table_diff_branch (NULL is a no-op)
 *
 * The call frees every string and nested array owned by the result. The
 * pointer and all data reachable from it must not be used afterwards. Do not
 * free nested pointers individually or pass an embedded
 * LanceDBCherryPickResult.diff address to this function.
 */
void lancedb_free_branch_diff(LanceDBBranchDiff* diff);

/**
 * Free a cherry-pick result returned by lancedb_table_cherry_pick
 *
 * @param result - pointer returned by lancedb_table_cherry_pick (NULL is a no-op)
 *
 * The call frees every string and nested array owned by the result. The
 * pointer and all data reachable from it must not be used afterwards. Do not
 * free nested pointers individually or pass the embedded diff to
 * lancedb_free_branch_diff().
 */
void lancedb_free_cherry_pick_result(LanceDBCherryPickResult* result);

/**
 * Create a delta between two endpoints on the table's current branch
 *
 * @param table - pointer to LanceDBTable
 * @param begin - required exclusive starting endpoint
 * @param end - optional inclusive ending endpoint; NULL uses the table's
 *              currently visible snapshot
 * @param delta_out - pointer to receive the delta handle
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return Error code indicating success or failure
 *
 * Both endpoints are resolved when the delta is created, so later tag moves,
 * checkouts or table commits do not change it. Endpoints may be versions or
 * tags; tags are resolved on the current branch. A nonempty range with
 * reversed endpoints, an unknown version or tag, or a tag from another branch
 * fails with LANCEDB_INVALID_INPUT. Deltas are only supported on embedded
 * (native) tables; a remote table fails with LANCEDB_NOT_SUPPORTED.
 *
 * On entry, *delta_out and the error message are cleared. Free the returned
 * handle with lancedb_table_delta_free(). If error_message is provided and an
 * error occurs, the caller must free the error message with
 * lancedb_free_string().
 */
LanceDBError lancedb_table_delta(
    const LanceDBTable* table,
    const LanceDBDeltaRef* begin,
    const LanceDBDeltaRef* end,
    LanceDBTableDelta** delta_out,
    char** error_message
);

/**
 * Get the branch a table handle is scoped to
 *
 * @param table - pointer to LanceDBTable
 * @param branch_out - pointer to receive the branch name
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return Error code indicating success or failure
 *
 * On success, branch_out is NULL when the handle is on main, or points to an
 * allocated null-terminated branch name. The caller must free a non-NULL name
 * with lancedb_free_string(). On failure, branch_out is set to NULL.
 * If error_message is provided and an error occurs, the caller must free
 * the error message with lancedb_free_string().
 */
LanceDBError lancedb_table_current_branch(
    const LanceDBTable* table,
    char** branch_out,
    char** error_message
);

/**
 * Get the resolved begin version of a delta
 *
 * @param delta - pointer to LanceDBTableDelta (NULL returns 0)
 * @return The delta's exclusive begin version, or 0 for a NULL handle
 *
 * A persisted version is never 0, so 0 is a safe failure sentinel.
 */
uint64_t lancedb_table_delta_begin_version(const LanceDBTableDelta* delta);

/**
 * Get the resolved end version of a delta
 *
 * @param delta - pointer to LanceDBTableDelta (NULL returns 0)
 * @return The delta's inclusive end version, or 0 for a NULL handle
 *
 * A persisted version is never 0, so 0 is a safe failure sentinel.
 */
uint64_t lancedb_table_delta_end_version(const LanceDBTableDelta* delta);

/**
 * Stream rows inserted between the delta endpoints
 *
 * @param delta - pointer to LanceDBTableDelta
 * @param result_out - pointer to receive the streaming result
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return LANCEDB_SUCCESS with *result_out set, otherwise the SDK error code
 *
 * Row-level readers require stable row ids at both endpoints. Without them the
 * SDK rejects the reader with LANCEDB_INVALID_INPUT when this function is
 * called; creating the delta still succeeds. A range containing a Restore or
 * missing transaction history is rejected the same way. Each batch carries the
 * table columns plus _rowid, _row_created_at_version and
 * _row_last_updated_at_version, read as of the end version.
 *
 * On entry, *result_out and the error message are cleared. The returned result
 * is one owning handle, independent of the delta and the source table. Dispose
 * of it exactly once: call lancedb_query_result_free() without consuming it, or
 * pass it to exactly one conversion. lancedb_query_result_to_ipc_file()
 * consumes a non-NULL result on success or failure.
 * lancedb_query_result_to_arrow() consumes the result when all required
 * out-pointers are non-NULL, on success or failure; with a NULL required
 * out-pointer it returns LANCEDB_INVALID_ARGUMENT and leaves the result with
 * the caller. Do not free a result after a conversion consumed it. The
 * conversion docs name the frees for returned arrays/schema and IPC bytes.
 *
 * Equal endpoints return a successful empty stream that keeps this reader's
 * schema without requiring stable row ids. Use
 * lancedb_query_result_to_ipc_file() to read that schema; the Arrow C Data
 * conversion reports an empty result as NULL arrays, NULL schema and a zero
 * count.
 */
LanceDBError lancedb_table_delta_inserted_rows(
    const LanceDBTableDelta* delta,
    LanceDBQueryResult** result_out,
    char** error_message
);

/**
 * Stream rows updated between the delta endpoints
 *
 * @param delta - pointer to LanceDBTableDelta
 * @param result_out - pointer to receive the streaming result
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return LANCEDB_SUCCESS with *result_out set, otherwise the SDK error code
 *
 * Newly inserted rows are excluded; use lancedb_table_delta_upserted_rows()
 * for inserted and updated rows together. The stable-row-id and restore
 * requirements are the same as for lancedb_table_delta_inserted_rows(). Each
 * batch carries the table columns plus _rowid, _row_created_at_version and
 * _row_last_updated_at_version, read as of the end version.
 *
 * On entry, *result_out and the error message are cleared. The returned result
 * is one owning handle, independent of the delta and the source table. Dispose
 * of it exactly once: call lancedb_query_result_free() without consuming it, or
 * pass it to exactly one conversion. lancedb_query_result_to_ipc_file()
 * consumes a non-NULL result on success or failure.
 * lancedb_query_result_to_arrow() consumes the result when all required
 * out-pointers are non-NULL, on success or failure; with a NULL required
 * out-pointer it returns LANCEDB_INVALID_ARGUMENT and leaves the result with
 * the caller. Do not free a result after a conversion consumed it. The
 * conversion docs name the frees for returned arrays/schema and IPC bytes.
 *
 * Equal endpoints return a successful empty stream that keeps this reader's
 * schema without requiring stable row ids. Use
 * lancedb_query_result_to_ipc_file() to read that schema; the Arrow C Data
 * conversion reports an empty result as NULL arrays, NULL schema and a zero
 * count.
 */
LanceDBError lancedb_table_delta_updated_rows(
    const LanceDBTableDelta* delta,
    LanceDBQueryResult** result_out,
    char** error_message
);

/**
 * Stream inserted and updated rows between the delta endpoints
 *
 * @param delta - pointer to LanceDBTableDelta
 * @param result_out - pointer to receive the streaming result
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return LANCEDB_SUCCESS with *result_out set, otherwise the SDK error code
 *
 * The stable-row-id and restore requirements are the same as for
 * lancedb_table_delta_inserted_rows(). Each batch carries the table columns
 * plus _rowid, _row_created_at_version and _row_last_updated_at_version, read
 * as of the end version.
 *
 * On entry, *result_out and the error message are cleared. The returned result
 * is one owning handle, independent of the delta and the source table. Dispose
 * of it exactly once: call lancedb_query_result_free() without consuming it, or
 * pass it to exactly one conversion. lancedb_query_result_to_ipc_file()
 * consumes a non-NULL result on success or failure.
 * lancedb_query_result_to_arrow() consumes the result when all required
 * out-pointers are non-NULL, on success or failure; with a NULL required
 * out-pointer it returns LANCEDB_INVALID_ARGUMENT and leaves the result with
 * the caller. Do not free a result after a conversion consumed it. The
 * conversion docs name the frees for returned arrays/schema and IPC bytes.
 *
 * Equal endpoints return a successful empty stream that keeps this reader's
 * schema without requiring stable row ids. Use
 * lancedb_query_result_to_ipc_file() to read that schema; the Arrow C Data
 * conversion reports an empty result as NULL arrays, NULL schema and a zero
 * count.
 */
LanceDBError lancedb_table_delta_upserted_rows(
    const LanceDBTableDelta* delta,
    LanceDBQueryResult** result_out,
    char** error_message
);

/**
 * Stream the ids of rows deleted between the delta endpoints
 *
 * @param delta - pointer to LanceDBTableDelta
 * @param result_out - pointer to receive the streaming result
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return LANCEDB_SUCCESS with *result_out set, otherwise the SDK error code
 *
 * The stable-row-id and restore requirements are the same as for
 * lancedb_table_delta_inserted_rows(). Each batch carries one _rowid column of
 * type UInt64; deleted rows are not reconstructed from earlier versions.
 *
 * On entry, *result_out and the error message are cleared. The returned result
 * is one owning handle, independent of the delta and the source table. Dispose
 * of it exactly once: call lancedb_query_result_free() without consuming it, or
 * pass it to exactly one conversion. lancedb_query_result_to_ipc_file()
 * consumes a non-NULL result on success or failure.
 * lancedb_query_result_to_arrow() consumes the result when all required
 * out-pointers are non-NULL, on success or failure; with a NULL required
 * out-pointer it returns LANCEDB_INVALID_ARGUMENT and leaves the result with
 * the caller. Do not free a result after a conversion consumed it. The
 * conversion docs name the frees for returned arrays/schema and IPC bytes.
 *
 * Equal endpoints return a successful empty stream that keeps this reader's
 * schema without requiring stable row ids. Use
 * lancedb_query_result_to_ipc_file() to read that schema; the Arrow C Data
 * conversion reports an empty result as NULL arrays, NULL schema and a zero
 * count.
 */
LanceDBError lancedb_table_delta_deleted_row_ids(
    const LanceDBTableDelta* delta,
    LanceDBQueryResult** result_out,
    char** error_message
);

/**
 * Free a TableDelta
 *
 * @param delta - pointer to LanceDBTableDelta (NULL is a no-op)
 *
 * Results already returned by a reader stay valid after the delta is freed.
 */
void lancedb_table_delta_free(LanceDBTableDelta* delta);

/**
 * Get table metadata as key-value pairs
 *
 * @param table - pointer to LanceDBTable
 * @param filter_keys - optional array of key strings to filter by (NULL to get all metadata)
 * @param filter_count - number of keys in filter_keys (0 to get all metadata)
 * @param keys_out - pointer to receive array of metadata key strings
 * @param values_out - pointer to receive array of metadata value strings
 * @param count_out - pointer to receive number of metadata entries
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return Error code indicating success or failure
 *
 * This reads the dataset directly and is available only on embedded
 * (Lance-backed) tables. On a remote table it returns LANCEDB_NOT_SUPPORTED;
 * use lancedb_table_update_field_metadata for field metadata that goes through
 * the SDK on both backends.
 * To return all metadata, please set filter_keys to NULL and filter_count to zero.
 * Any other combination of NULL and zero is considered an error.
 * Otherwise, only metadata matching the specified keys is returned.
 * Keys in filter_keys that do not exist in the metadata are silently skipped.
 * On success, keys_out and values_out will point to parallel arrays of strings.
 * The caller must free the returned arrays using lancedb_free_metadata().
 * If error_message is provided and an error occurs, the caller must free
 * the error message with lancedb_free_string().
 */
LanceDBError lancedb_table_get_metadata(
    const LanceDBTable* table,
    const char* const* filter_keys,
    size_t filter_count,
    char*** keys_out,
    char*** values_out,
    size_t* count_out,
    char** error_message
);

/**
 * Set (upsert) table metadata key-value pairs
 *
 * @param table - pointer to LanceDBTable
 * @param keys - array of null-terminated key strings
 * @param values - array of null-terminated value strings
 * @param count - number of key-value pairs
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return Error code indicating success or failure
 *
 * Existing keys will be updated with new values. New keys will be added.
 * This writes the dataset directly and is available only on embedded
 * (Lance-backed) tables. On a remote table it returns LANCEDB_NOT_SUPPORTED;
 * use lancedb_table_update_field_metadata for field metadata that goes through
 * the SDK on both backends.
 * If error_message is provided and an error occurs, the caller must free
 * the error message with lancedb_free_string().
 */
LanceDBError lancedb_table_set_metadata(
    const LanceDBTable* table,
    const char* const* keys,
    const char* const* values,
    size_t count,
    char** error_message
);

/**
 * Delete table metadata keys
 *
 * @param table - pointer to LanceDBTable
 * @param keys - array of null-terminated key strings to delete
 * @param count - number of keys to delete
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return Error code indicating success or failure
 *
 * Keys that do not exist are silently ignored.
 * This writes the dataset directly and is available only on embedded
 * (Lance-backed) tables. On a remote table it returns LANCEDB_NOT_SUPPORTED;
 * use lancedb_table_update_field_metadata for field metadata that goes through
 * the SDK on both backends.
 * If error_message is provided and an error occurs, the caller must free
 * the error message with lancedb_free_string().
 */
LanceDBError lancedb_table_delete_metadata(
    const LanceDBTable* table,
    const char* const* keys,
    size_t count,
    char** error_message
);

/**
 * Free versions array returned by lancedb_table_list_versions
 *
 * @param versions - array of LanceDBVersion structs returned by lancedb_table_list_versions
 * @param count - number of versions in the array
 */
void lancedb_free_versions(LanceDBVersion* versions, size_t count);

/**
 * Free version metadata array returned by lancedb_table_list_versions
 *
 * @param metadata - array of LanceDBVersionMetadata structs returned by lancedb_table_list_versions
 * @param count - number of entries in the array
 *
 * Safe to call with NULL pointer.
 */
void lancedb_free_version_metadata(LanceDBVersionMetadata* metadata, size_t count);

/**
 * Free tag array returned by lancedb_table_list_tags
 *
 * @param tags - read-only array of LanceDBTag structs returned by lancedb_table_list_tags
 * @param count - number of entries, exactly as returned by lancedb_table_list_tags
 *
 * The library owns the array and every string in it. Callers must not modify
 * entries or free them individually, and must pass the count returned with the
 * array. Safe to call with NULL pointer.
 */
void lancedb_free_tags(const LanceDBTag* tags, size_t count);

/**
 * Free metadata arrays returned by lancedb_table_get_metadata
 *
 * @param keys - array of key strings returned by lancedb_table_get_metadata
 * @param values - array of value strings returned by lancedb_table_get_metadata
 * @param count - number of entries in the arrays
 *
 * Safe to call with NULL pointers.
 */
void lancedb_free_metadata(char** keys, char** values, size_t count);

/**
 * Free string returned by LanceDB functions
 *
 * @param str - string pointer returned by LanceDB functions
 */
void lancedb_free_string(char* str);

/** @} */

/**
 * @defgroup api_expression Expressions
 * @brief Build DataFusion expressions and evaluate JSON predicates.
 * @{
 */

/* ==================== DataFusion Expression Builder API ==================== */

/**
 * Create a column reference expression
 *
 * @param name - null-terminated C string containing the column name
 * @return Non-null pointer to LanceDBExpr on success, NULL on failure
 *         Caller must free with lancedb_expr_free()
 */
LanceDBExpr* lancedb_expr_column(const char* name);

/**
 * Create a string literal expression
 *
 * @param value - null-terminated C string containing the literal value
 * @return Non-null pointer to LanceDBExpr on success, NULL on failure
 *         Caller must free with lancedb_expr_free()
 */
LanceDBExpr* lancedb_expr_literal_string(const char* value);

/**
 * Create an integer literal expression (i64)
 *
 * @param value - the integer value
 * @return Non-null pointer to LanceDBExpr
 *         Caller must free with lancedb_expr_free()
 */
LanceDBExpr* lancedb_expr_literal_i64(int64_t value);

/**
 * Create a float literal expression (f64)
 *
 * @param value - the float value
 * @return Non-null pointer to LanceDBExpr
 *         Caller must free with lancedb_expr_free()
 */
LanceDBExpr* lancedb_expr_literal_f64(double value);

/**
 * Create a boolean literal expression
 *
 * @param value - the boolean value
 * @return Non-null pointer to LanceDBExpr
 *         Caller must free with lancedb_expr_free()
 */
LanceDBExpr* lancedb_expr_literal_bool(bool value);

/**
 * Create a binary expression (left op right)
 *
 * @param left - pointer to LanceDBExpr for left operand (consumed)
 * @param op - binary operator
 * @param right - pointer to LanceDBExpr for right operand (consumed)
 * @return Non-null pointer to LanceDBExpr on success, NULL on failure
 *         Both left and right are consumed; do not use or free them after calling
 *         Caller must free result with lancedb_expr_free()
 */
LanceDBExpr* lancedb_expr_binary(
    LanceDBExpr* left,
    LanceDBBinaryOp op,
    LanceDBExpr* right
);

/**
 * Create a NOT expression
 *
 * @param expr - pointer to LanceDBExpr (consumed)
 * @return Non-null pointer to LanceDBExpr on success, NULL on failure
 *         The input expr is consumed; do not use or free it after calling
 *         Caller must free result with lancedb_expr_free()
 */
LanceDBExpr* lancedb_expr_not(LanceDBExpr* expr);

/**
 * Create an IS NULL expression
 *
 * @param expr - pointer to LanceDBExpr (consumed)
 * @return Non-null pointer to LanceDBExpr on success, NULL on failure
 *         The input expr is consumed; do not use or free it after calling
 *         Caller must free result with lancedb_expr_free()
 */
LanceDBExpr* lancedb_expr_is_null(LanceDBExpr* expr);

/**
 * Create an IS NOT NULL expression
 *
 * @param expr - pointer to LanceDBExpr (consumed)
 * @return Non-null pointer to LanceDBExpr on success, NULL on failure
 *         The input expr is consumed; do not use or free it after calling
 *         Caller must free result with lancedb_expr_free()
 */
LanceDBExpr* lancedb_expr_is_not_null(LanceDBExpr* expr);

/**
 * Create an AND expression (convenience for binary AND)
 *
 * @param left - pointer to LanceDBExpr (consumed)
 * @param right - pointer to LanceDBExpr (consumed)
 * @return Non-null pointer to LanceDBExpr on success, NULL on failure
 *         Both inputs are consumed; do not use or free them after calling
 *         Caller must free result with lancedb_expr_free()
 */
LanceDBExpr* lancedb_expr_and(LanceDBExpr* left, LanceDBExpr* right);

/**
 * Create an OR expression (convenience for binary OR)
 *
 * @param left - pointer to LanceDBExpr (consumed)
 * @param right - pointer to LanceDBExpr (consumed)
 * @return Non-null pointer to LanceDBExpr on success, NULL on failure
 *         Both inputs are consumed; do not use or free them after calling
 *         Caller must free result with lancedb_expr_free()
 */
LanceDBExpr* lancedb_expr_or(LanceDBExpr* left, LanceDBExpr* right);

/**
 * Create an IN list expression (expr IN (value1, value2, ...))
 *
 * @param expr - pointer to LanceDBExpr for the expression to check (consumed)
 * @param list - array of pointers to LanceDBExpr for list values (all consumed)
 * @param list_len - number of elements in the list
 * @param negated - if true, creates NOT IN instead of IN
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return Non-null pointer to LanceDBExpr on success, NULL on failure
 *         The input expr and all list elements are consumed; do not use or free them after calling
 *         Caller must free result with lancedb_expr_free()
 */
LanceDBExpr* lancedb_expr_in_list(
    LanceDBExpr* expr,
    LanceDBExpr* const* list,
    size_t list_len,
    bool negated,
    char** error_message
);

/**
 * Create an array_has expression: check if an array column contains a value
 *
 * Returns a boolean expression that is true when the array contains the element.
 *
 * @param array_expr - pointer to LanceDBExpr for the array column (consumed)
 * @param value_expr - pointer to LanceDBExpr for the value to search for (consumed)
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return Non-null pointer to LanceDBExpr on success, NULL on failure
 *         Both inputs are consumed; do not use or free them after calling
 *         Caller must free result with lancedb_expr_free()
 */
LanceDBExpr* lancedb_expr_array_has(
    LanceDBExpr* array_expr,
    LanceDBExpr* value_expr,
    char** error_message
);

/**
 * Create a starts_with expression: check if a string expression starts with a prefix
 *
 * Returns a boolean expression that is true when str_expr starts with prefix_expr.
 *
 * @param str_expr    - pointer to LanceDBExpr for the string column/expression (consumed)
 * @param prefix_expr - pointer to LanceDBExpr for the prefix to check against (consumed)
 * @param error_message - optional pointer to receive detailed error message (NULL to ignore)
 * @return Non-null pointer to LanceDBExpr on success, NULL on failure
 *         Both inputs are consumed; do not use or free them after calling
 *         Caller must free result with lancedb_expr_free()
 *         If error_message is provided and an error occurs, the caller must free
 *         the message with lancedb_free_string()
 */
LanceDBExpr* lancedb_expr_starts_with(
    LanceDBExpr* str_expr,
    LanceDBExpr* prefix_expr,
    char** error_message
);

/**
 * Clone an expression (creates an independent copy)
 *
 * @param expr - pointer to LanceDBExpr to clone (not consumed)
 * @return Non-null pointer to a new LanceDBExpr on success, NULL on failure
 *         The original expr remains valid and must still be freed separately
 *         Caller must free result with lancedb_expr_free()
 */
LanceDBExpr* lancedb_expr_clone(const LanceDBExpr* expr);

/**
 * Free an expression
 *
 * @param expr - pointer to LanceDBExpr
 *
 * After calling this function, the expr pointer must not be used.
 */
void lancedb_expr_free(LanceDBExpr* expr);

/* ==================== JSON Expression API ==================== */

/**
 * Create a json_get_str expression: extract a string value from JSON by path
 *
 * For nested access, pass multiple path segments (e.g., ["user", "name"] for {"user":{"name":"alice"}}).
 *
 * @param json_expr - pointer to LanceDBExpr for the JSON column (consumed)
 * @param path - array of null-terminated C strings forming the JSON path
 * @param path_len - number of path segments
 * @return Non-null pointer to LanceDBExpr on success, NULL on failure
 *         The json_expr is consumed; do not use or free it after calling.
 *         Caller must free result with lancedb_expr_free()
 */
LanceDBExpr* lancedb_expr_json_get_str(LanceDBExpr* json_expr, const char* const* path, size_t path_len);

/**
 * Create a json_get_int expression: extract an integer value from JSON by path
 *
 * For nested access, pass multiple path segments (e.g., ["stats", "count"] for {"stats":{"count":42}}).
 *
 * @param json_expr - pointer to LanceDBExpr for the JSON column (consumed)
 * @param path - array of null-terminated C strings forming the JSON path
 * @param path_len - number of path segments
 * @return Non-null pointer to LanceDBExpr on success, NULL on failure
 *         The json_expr is consumed; do not use or free it after calling.
 *         Caller must free result with lancedb_expr_free()
 */
LanceDBExpr* lancedb_expr_json_get_int(LanceDBExpr* json_expr, const char* const* path, size_t path_len);

/**
 * Create a json_get_float expression: extract a float value from JSON by path
 *
 * For nested access, pass multiple path segments (e.g., ["stats", "score"] for {"stats":{"score":4.2}}).
 *
 * @param json_expr - pointer to LanceDBExpr for the JSON column (consumed)
 * @param path - array of null-terminated C strings forming the JSON path
 * @param path_len - number of path segments
 * @return Non-null pointer to LanceDBExpr on success, NULL on failure
 *         The json_expr is consumed; do not use or free it after calling.
 *         Caller must free result with lancedb_expr_free()
 */
LanceDBExpr* lancedb_expr_json_get_float(LanceDBExpr* json_expr, const char* const* path, size_t path_len);

/**
 * Create a json_get_bool expression: extract a boolean value from JSON by path
 *
 * For nested access, pass multiple path segments (e.g., ["user", "active"] for {"user":{"active":true}}).
 *
 * @param json_expr - pointer to LanceDBExpr for the JSON column (consumed)
 * @param path - array of null-terminated C strings forming the JSON path
 * @param path_len - number of path segments
 * @return Non-null pointer to LanceDBExpr on success, NULL on failure
 *         The json_expr is consumed; do not use or free it after calling.
 *         Caller must free result with lancedb_expr_free()
 */
LanceDBExpr* lancedb_expr_json_get_bool(LanceDBExpr* json_expr, const char* const* path, size_t path_len);

/**
 * Create a json_contains expression: check if a key exists in JSON
 *
 * For nested access, pass multiple path segments (e.g., ["user", "name"] to check
 * if {"user":{"name":"alice"}} has the nested key).
 *
 * @param json_expr - pointer to LanceDBExpr for the JSON column (consumed)
 * @param path - array of null-terminated C strings forming the JSON path
 * @param path_len - number of path segments
 * @return Non-null pointer to LanceDBExpr on success, NULL on failure
 *         The json_expr is consumed; do not use or free it after calling.
 *         Caller must free result with lancedb_expr_free()
 */
LanceDBExpr* lancedb_expr_json_contains(LanceDBExpr* json_expr, const char* const* path, size_t path_len);

/**
 * Create a json_array_has expression: check if a JSON array contains a value
 *
 * Navigates to a JSON array field using the path, then checks whether the given
 * value expression appears as an element. The quote_value flag controls how the
 * value is adapted:
 * - true: for JSON string arrays: value is wrapped in JSON quotes for matching.
 * - false: for JSON number/boolean arrays: value is cast to Utf8.
 *
 * For arrays of non-scalar values (objects, nested arrays), it falls
 * back to exact string matching against the raw JSON text, which is not JSON-aware
 *
 * @param json_expr - pointer to LanceDBExpr for the JSON column (consumed)
 * @param path - array of null-terminated C strings forming the JSON path to the array
 * @param path_len - number of path segments
 * @param value_expr - pointer to LanceDBExpr for the value to search for (consumed)
 * @param quote_value - true for string arrays, false for number/boolean arrays
 * @return Non-null pointer to LanceDBExpr on success, NULL on failure
 *         Both json_expr and value_expr are consumed; do not use or free them after calling.
 *         Caller must free result with lancedb_expr_free()
 */
LanceDBExpr* lancedb_expr_json_array_has(LanceDBExpr* json_expr, const char* const* path, size_t path_len, LanceDBExpr* value_expr, bool quote_value);

/**
 * Evaluate a JSON filter expression against Arrow RecordBatches.
 *
 * Takes the Arrow FFI arrays and schema from lancedb_query_result_to_arrow
 * and a filter expression. Returns a boolean array indicating which rows
 * matched. The arrays are NOT consumed and remain valid after the call.
 *
 * Must be called while the arrays are valid (before consuming them with
 * arrow::ImportRecordBatch or freeing with lancedb_free_arrow_arrays).
 *
 * Example: to filter rows where metadata.name = "alice":
 *   col  = lancedb_expr_column("metadata")
 *   path = (const char*[]){"name"}
 *   name = lancedb_expr_json_get_str(col, path, 1)
 *   val  = lancedb_expr_literal_string("alice")
 *   expr = lancedb_expr_binary(name, LANCEDB_BINARY_OP_EQ, val)
 *   lancedb_json_matches(arrays, schema, count, expr,
 *                        &results, &nrows, &error_message)
 *
 * @param arrays - array of FFI_ArrowArray pointers (NOT consumed)
 * @param schema - pointer to FFI_ArrowSchema (NOT consumed)
 * @param batch_count - number of batches in the arrays
 * @param expr - pointer to LanceDBExpr filter expression (consumed)
 * @param results_out - pointer to receive allocated bool array (one per row)
 * @param count_out - pointer to receive total row count across all batches
 * @param error_message - optional pointer to receive detailed error message
 * @return Error code indicating success or failure.
 *         The expr is consumed; do not use or free it after calling.
 *         Caller must free *results_out with lancedb_free_json_matches().
 *         If error_message is set, caller must free with lancedb_free_string().
 */
LanceDBError lancedb_json_matches(
    FFI_ArrowArray** arrays,
    FFI_ArrowSchema* schema,
    size_t batch_count,
    LanceDBExpr* expr,
    bool** results_out,
    size_t* count_out,
    char** error_message
);

/**
 * Free results array returned by lancedb_json_matches
 *
 * @param results - pointer returned by lancedb_json_matches, or NULL
 */
void lancedb_free_json_matches(bool* results);

/** @} */

#ifdef __cplusplus
}
#endif

#endif /* LANCEDB_H */
