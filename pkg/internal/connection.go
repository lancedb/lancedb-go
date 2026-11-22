// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: Copyright The LanceDB Authors

package internal

import (
	"context"
	"fmt"
	"runtime"
	"sort"
	"sync"

	"github.com/lancedb/lancedb-go/pkg/contracts"
)

// Connection is a live lancedb-c connection.
type Connection struct {
	handle uintptr
	mu     sync.RWMutex
	closed bool
}

// NewConnection wraps an already-created native connection.
func NewConnection(handle uintptr, closed bool) *Connection {
	return &Connection{handle: handle, closed: closed}
}

var _ contracts.IConnection = (*Connection)(nil)

func (c *Connection) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed || c.handle == 0 {
		return nil
	}
	api, err := loadAPI()
	if err != nil {
		return err
	}
	api.ConnectionFree(c.handle)
	c.handle = 0
	c.closed = true
	runtime.SetFinalizer(c, nil)
	return nil
}

func (c *Connection) IsClosed() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.closed
}

func (c *Connection) TableNames(_ context.Context) ([]string, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.closed || c.handle == 0 {
		return nil, fmt.Errorf("connection is closed")
	}
	api, err := loadAPI()
	if err != nil {
		return nil, err
	}
	var names **byte
	var count uintptr
	var errorMessage *byte
	code := api.ConnectionTableNames(c.handle, &names, &count, &errorMessage)
	if err := check(api, code, errorMessage, "failed to get table names"); err != nil {
		if names != nil {
			api.FreeTableNames(names, count)
		}
		return nil, err
	}
	tableNames := goStringArray(names, count)
	if names != nil {
		api.FreeTableNames(names, count)
	}
	if tableNames == nil {
		return []string{}, nil
	}
	return tableNames, nil
}

func (c *Connection) OpenTable(_ context.Context, name string) (contracts.ITable, error) {
	if err := validateNoNUL("table name", name); err != nil {
		return nil, err
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.closed || c.handle == 0 {
		return nil, fmt.Errorf("connection is closed")
	}
	api, err := loadAPI()
	if err != nil {
		return nil, err
	}
	var tableHandle uintptr
	var errorMessage *byte
	code := api.ConnectionOpenTable(c.handle, name, &tableHandle, &errorMessage)
	if err := check(api, code, errorMessage, fmt.Sprintf("failed to open table %s", name)); err != nil {
		return nil, err
	}
	table := &Table{name: name, connection: c, handle: tableHandle}
	runtime.SetFinalizer(table, (*Table).finalize)
	return table, nil
}

func (c *Connection) CreateTable(_ context.Context, name string, schema contracts.ISchema) (contracts.ITable, error) {
	if err := validateNoNUL("table name", name); err != nil {
		return nil, err
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.closed || c.handle == 0 {
		return nil, fmt.Errorf("connection is closed")
	}
	if schema == nil {
		return nil, fmt.Errorf("schema is nil")
	}
	arrowSchema := schema.ToArrowSchema()
	ipcBytes, err := schemaToIPCBytes(arrowSchema)
	if err != nil {
		return nil, fmt.Errorf("failed to convert schema to IPC: %w", err)
	}
	api, err := loadAPI()
	if err != nil {
		return nil, err
	}
	arrowSchemaHandle, err := newIPCSchema(api, ipcBytes)
	if err != nil {
		return nil, err
	}
	var tableHandle uintptr
	var errorMessage *byte
	code := api.TableCreate(c.handle, name, arrowSchemaHandle, 0, &tableHandle, &errorMessage)
	// Empty table creation needs only the schema; the C API borrows it.
	api.FreeArrowSchema(arrowSchemaHandle)
	if err := check(api, code, errorMessage, fmt.Sprintf("failed to create table %s", name)); err != nil {
		return nil, err
	}
	table := &Table{name: name, connection: c, handle: tableHandle}
	runtime.SetFinalizer(table, (*Table).finalize)
	return table, nil
}

func (c *Connection) DropTable(_ context.Context, name string) error {
	if err := validateNoNUL("table name", name); err != nil {
		return err
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.closed || c.handle == 0 {
		return fmt.Errorf("connection is closed")
	}
	api, err := loadAPI()
	if err != nil {
		return err
	}
	var errorMessage *byte
	code := api.ConnectionDropTable(c.handle, name, nil, &errorMessage)
	return check(api, code, errorMessage, fmt.Sprintf("failed to drop table %s", name))
}

// Connect opens a connection using the published v0.1.2 options. Region and
// ReadConsistencyInterval are preserved fields but remain ignored, as they were
// in v0.1.2. Structured S3 credentials are passed through the per-connection
// storage setter instead of mutating process environment variables.
func Connect(ctx context.Context, uri string, options *contracts.ConnectionOptions) (contracts.IConnection, error) {
	var storageOptions map[string]string
	if options != nil && options.StorageOptions != nil {
		storageOptions = legacyStorageOptions(options.StorageOptions)
	}
	return connect(ctx, uri, storageOptions)
}

// ConnectWithStorageOptions opens a connection with a flat storage-option map.
// This is the additive path for options the published v0.1.2 structs cannot
// express.
func ConnectWithStorageOptions(ctx context.Context, uri string, storageOptions map[string]string) (contracts.IConnection, error) {
	return connect(ctx, uri, storageOptions)
}

func connect(_ context.Context, uri string, storageOptions map[string]string) (contracts.IConnection, error) {
	api, err := loadAPI()
	if err != nil {
		return nil, err
	}
	builder := api.Connect(uri)
	if builder == 0 {
		return nil, fmt.Errorf("failed to connect to LanceDB at %s: invalid URI", uri)
	}

	keys := make([]string, 0, len(storageOptions))
	for key := range storageOptions {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		value := storageOptions[key]
		next := api.ConnectBuilderStorageOption(builder, key, value)
		if next == 0 {
			return nil, fmt.Errorf("failed to connect to LanceDB at %s: invalid storage option %q", uri, key)
		}
		builder = next
	}

	var connectionHandle uintptr
	var errorMessage *byte
	code := api.ConnectBuilderExecute(builder, &connectionHandle, &errorMessage)
	if err := check(api, code, errorMessage, fmt.Sprintf("failed to connect to LanceDB at %s", uri)); err != nil {
		return nil, err
	}
	connection := NewConnection(connectionHandle, false)
	runtime.SetFinalizer(connection, (*Connection).finalize)
	return connection, nil
}

func (c *Connection) finalize() {
	_ = c.Close()
}

func legacyStorageOptions(options *contracts.StorageOptions) map[string]string {
	if options == nil || options.S3Config == nil {
		return nil
	}
	s3 := options.S3Config
	out := make(map[string]string)
	put := func(key string, value *string) {
		if value != nil {
			out[key] = *value
		}
	}
	put(contracts.StorageAWSAccessKeyID, s3.AccessKeyID)
	put(contracts.StorageAWSSecretAccessKey, s3.SecretAccessKey)
	put(contracts.StorageAWSSessionToken, s3.SessionToken)
	put(contracts.StorageAWSRegion, s3.Region)
	put(contracts.StorageAWSProfile, s3.Profile)
	// v0.1.2 only read credentials, session token, region and profile from
	// the structured S3 config. Endpoint, addressing style, anonymous access
	// and TLS switches stay explicit through ConnectWithStorageOptions.
	return out
}
