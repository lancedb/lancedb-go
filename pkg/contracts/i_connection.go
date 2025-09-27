package contracts

import "C"
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

// ConnectionOptions holds options for establishing a database connection
type ConnectionOptions struct {
	// Simple implementation - these fields will be added as needed
	Region                  *string
	ReadConsistencyInterval *int
	StorageOptions          *StorageOptions
}
