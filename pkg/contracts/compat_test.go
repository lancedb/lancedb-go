// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: Copyright The LanceDB Authors

package contracts_test

import (
	"context"
	"reflect"
	"testing"

	"github.com/apache/arrow/go/v17/arrow"

	"github.com/lancedb/lancedb-go/pkg/contracts"
)

// The declarations below are external implementations of the published v0.1.2
// interfaces. They fail to compile if a method is added to a published
// interface; that would silently break callers that provide their own mocks.
type legacyConnection struct{}

var _ contracts.IConnection = legacyConnection{}

func (legacyConnection) Close() error                                                { return nil }
func (legacyConnection) IsClosed() bool                                              { return false }
func (legacyConnection) TableNames(context.Context) ([]string, error)                { return nil, nil }
func (legacyConnection) OpenTable(context.Context, string) (contracts.ITable, error) { return nil, nil }
func (legacyConnection) CreateTable(context.Context, string, contracts.ISchema) (contracts.ITable, error) {
	return nil, nil
}
func (legacyConnection) DropTable(context.Context, string) error { return nil }

type legacyTable struct{}

var _ contracts.ITable = legacyTable{}

func (legacyTable) Name() string                                                       { return "" }
func (legacyTable) IsOpen() bool                                                       { return false }
func (legacyTable) Close() error                                                       { return nil }
func (legacyTable) Schema(context.Context) (*arrow.Schema, error)                      { return nil, nil }
func (legacyTable) Add(context.Context, arrow.Record, *contracts.AddDataOptions) error { return nil }
func (legacyTable) AddRecords(context.Context, []arrow.Record, *contracts.AddDataOptions) error {
	return nil
}
func (legacyTable) Query() contracts.IQueryBuilder                                   { return legacyQuery{} }
func (legacyTable) Count(context.Context) (int64, error)                             { return 0, nil }
func (legacyTable) Version(context.Context) (int, error)                             { return 0, nil }
func (legacyTable) Update(context.Context, string, map[string]interface{}) error     { return nil }
func (legacyTable) Delete(context.Context, string) error                             { return nil }
func (legacyTable) CreateIndex(context.Context, []string, contracts.IndexType) error { return nil }
func (legacyTable) CreateIndexWithName(context.Context, []string, contracts.IndexType, string) error {
	return nil
}
func (legacyTable) GetAllIndexes(context.Context) ([]contracts.IndexInfo, error) { return nil, nil }
func (legacyTable) Select(context.Context, contracts.QueryConfig) ([]map[string]interface{}, error) {
	return nil, nil
}
func (legacyTable) SelectWithColumns(context.Context, []string) ([]map[string]interface{}, error) {
	return nil, nil
}
func (legacyTable) SelectWithFilter(context.Context, string) ([]map[string]interface{}, error) {
	return nil, nil
}
func (legacyTable) VectorSearch(context.Context, string, []float32, int) ([]map[string]interface{}, error) {
	return nil, nil
}
func (legacyTable) VectorSearchWithFilter(context.Context, string, []float32, int, string) ([]map[string]interface{}, error) {
	return nil, nil
}
func (legacyTable) FullTextSearch(context.Context, string, string) ([]map[string]interface{}, error) {
	return nil, nil
}
func (legacyTable) FullTextSearchWithFilter(context.Context, string, string, string) ([]map[string]interface{}, error) {
	return nil, nil
}
func (legacyTable) SelectWithLimit(context.Context, int, int) ([]map[string]interface{}, error) {
	return nil, nil
}

type legacyQuery struct{}

var _ contracts.IQueryBuilder = legacyQuery{}

func (legacyQuery) Filter(string) contracts.IQueryBuilder               { return legacyQuery{} }
func (legacyQuery) Limit(int) contracts.IQueryBuilder                   { return legacyQuery{} }
func (legacyQuery) Execute() ([]arrow.Record, error)                    { return nil, nil }
func (legacyQuery) ExecuteAsync() (<-chan []arrow.Record, <-chan error) { return nil, nil }
func (legacyQuery) ApplyOptions(*contracts.QueryOptions) contracts.IQueryBuilder {
	return legacyQuery{}
}

type legacyVectorQuery struct{}

var _ contracts.IVectorQueryBuilder = legacyVectorQuery{}

func (legacyVectorQuery) Filter(string) contracts.IVectorQueryBuilder { return legacyVectorQuery{} }
func (legacyVectorQuery) Limit(int) contracts.IVectorQueryBuilder     { return legacyVectorQuery{} }
func (legacyVectorQuery) DistanceType(contracts.DistanceType) contracts.IVectorQueryBuilder {
	return legacyVectorQuery{}
}
func (legacyVectorQuery) Execute() ([]arrow.Record, error)                    { return nil, nil }
func (legacyVectorQuery) ExecuteAsync() (<-chan []arrow.Record, <-chan error) { return nil, nil }
func (legacyVectorQuery) ApplyOptions(*contracts.QueryOptions) contracts.IVectorQueryBuilder {
	return legacyVectorQuery{}
}

func TestPublishedDistanceValues(t *testing.T) {
	if contracts.DistanceTypeL2 != 0 || contracts.DistanceTypeCosine != 1 || contracts.DistanceTypeDot != 2 || contracts.DistanceTypeHamming != 3 {
		t.Fatalf("published distance values changed: %d %d %d %d", contracts.DistanceTypeL2, contracts.DistanceTypeCosine, contracts.DistanceTypeDot, contracts.DistanceTypeHamming)
	}
}

func TestPublishedStructShapes(t *testing.T) {
	assertFields(t, reflect.TypeOf(contracts.QueryConfig{}), []string{"Columns", "Where", "Limit", "Offset", "VectorSearch", "FTSSearch"})
	assertFields(t, reflect.TypeOf(contracts.VectorSearch{}), []string{"Column", "Vector", "K"})
	assertFields(t, reflect.TypeOf(contracts.ConnectionOptions{}), []string{"Region", "ReadConsistencyInterval", "StorageOptions"})
	assertFields(t, reflect.TypeOf(contracts.StorageOptions{}), []string{"S3Config", "AzureConfig", "GCSConfig", "LocalConfig", "BlockSize", "MaxRetries", "RetryDelay", "Timeout", "AllowHTTP", "UserAgent", "ConnectTimeout", "ReadTimeout", "PoolIdleTimeout", "PoolMaxIdlePerHost"})
	assertFields(t, reflect.TypeOf(contracts.S3Config{}), []string{"AccessKeyID", "SecretAccessKey", "SessionToken", "Region", "Endpoint", "ForcePathStyle", "Profile", "AnonymousAccess", "UseSSL", "ServerSideEncrypt", "SSEKMSKeyID", "StorageClass"})
	assertFields(t, reflect.TypeOf(contracts.IndexInfo{}), []string{"Name", "Columns", "IndexType"})
	assertFields(t, reflect.TypeOf(contracts.AddDataOptions{}), []string{"Mode"})

	queryType := reflect.TypeOf(contracts.QueryConfig{})
	if tag := queryType.Field(0).Tag.Get("json"); tag != "columns,omitempty" {
		t.Fatalf("QueryConfig.Columns json tag = %q", tag)
	}
	s3Type := reflect.TypeOf(contracts.S3Config{})
	if tag := s3Type.Field(0).Tag.Get("json"); tag != "access_key_id,omitempty" {
		t.Fatalf("S3Config.AccessKeyID json tag = %q", tag)
	}
}

func assertFields(t *testing.T, typ reflect.Type, want []string) {
	t.Helper()
	if typ.NumField() != len(want) {
		t.Fatalf("%s has %d fields, want %d", typ, typ.NumField(), len(want))
	}
	for i, name := range want {
		if got := typ.Field(i).Name; got != name {
			t.Errorf("%s field %d = %s, want %s", typ, i, got, name)
		}
	}
}
