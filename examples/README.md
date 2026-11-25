# Examples

These examples use the pure-Go module in the parent directory. Run them with
Go 1.21+ and `CGO_ENABLED=0`; the first run extracts and verifies the native
payload from its compiled-in companion module automatically.

```bash
cd examples/basic_crud
go run .
```

| Example | What it shows |
| --- | --- |
| `basic_crud` | Connect, schema, create, add, query, close. |
| `batch_operations` | Multi-batch `AddRecords`. |
| `vector_search` | Vector search and filters. |
| `hybrid_search`, `hybrid_vector_fts` | FTS, hybrid search and rerankers. |
| `fts` (see `pkg/tests`) | Full-text index and query behavior. |
| `index_management`, `index_builder` | Index creation, listing, stats, wait and prewarm. |
| `merge_insert` | Upsert, conditional update and by-source delete. |
| `optimize_action` | Compact, prune and index actions with stats. |
| `query_tuning` | Nprobes, refine factor, ef, bypass, row IDs and flags. |
| `reranker` | RRF and normalization. |
| `storage_configuration` | Local, S3/MinIO, R2, GCS and Azure options. |
| `time_travel` (see `pkg/tests`) | Versions, checkout and tags. |
| `wait_for_index` | Named and wait-all behavior. |

All examples use synthetic local data. The storage example prints configuration
templates; it does not contact a production service.

To pin an explicit native library:

```bash
LANCEDB_C_LIBRARY_PATH=/absolute/path/to/liblancedb.so go run .
```
