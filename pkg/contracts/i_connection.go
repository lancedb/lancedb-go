package contracts

import (
	"context"
)

type IConnection interface {
	Close() error
	TableNames(ctx context.Context) ([]string, error)
	OpenTable(ctx context.Context, name string) (ITable, error)
	CreateTable(ctx context.Context, name string, schema ISchema) (ITable, error)
	DropTable(ctx context.Context, name string) error
	IsClosed() bool
}

// ConnectionOptions holds options for establishing a database connection.
type ConnectionOptions struct {
	ReadConsistencyInterval *int

	// StorageOptions contains key-value pairs passed directly to the
	// object_store backend. Keys match the object_store crate's config
	// enums (AmazonS3ConfigKey, GoogleConfigKey, AzureConfigKey,
	// ClientConfigKey). All values are strings.
	//
	// See the Storage* constants for well-known keys.
	StorageOptions map[string]string
}
