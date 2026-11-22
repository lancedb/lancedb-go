//go:build integration

// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: Copyright The LanceDB Authors

package lancedb_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/apache/arrow/go/v17/arrow"
	"github.com/apache/arrow/go/v17/arrow/array"
	"github.com/apache/arrow/go/v17/arrow/memory"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"github.com/lancedb/lancedb-go/pkg/contracts"
	"github.com/lancedb/lancedb-go/pkg/internal"
	"github.com/lancedb/lancedb-go/pkg/lancedb"
)

func minioConfig(t *testing.T) (endpoint, accessKey, secretKey, bucket string) {
	t.Helper()
	endpoint = os.Getenv("MINIO_ENDPOINT")
	accessKey = os.Getenv("MINIO_ACCESS_KEY")
	secretKey = os.Getenv("MINIO_SECRET_KEY")
	bucket = os.Getenv("MINIO_BUCKET")
	if endpoint == "" || accessKey == "" || secretKey == "" || bucket == "" {
		t.Skip("set MINIO_ENDPOINT, MINIO_ACCESS_KEY, MINIO_SECRET_KEY and MINIO_BUCKET to run the authenticated MinIO tests")
	}
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Host == "" {
		t.Fatalf("MINIO_ENDPOINT must be an http URL, got %q", endpoint)
	}
	client, err := minio.New(parsed.Host, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: parsed.Scheme == "https",
	})
	if err != nil {
		t.Fatalf("create MinIO client: %v", err)
	}
	if err := client.MakeBucket(context.Background(), bucket, minio.MakeBucketOptions{Region: "us-east-1"}); err != nil {
		exists, checkErr := client.BucketExists(context.Background(), bucket)
		if checkErr != nil || !exists {
			t.Fatalf("create MinIO bucket %q: %v (bucket exists check: %v)", bucket, err, checkErr)
		}
	}
	return endpoint, accessKey, secretKey, bucket
}

func addSingleRow(t *testing.T, table contracts.ITable, schema *arrow.Schema, id int32, label string) {
	t.Helper()
	builder := array.NewRecordBuilder(memory.DefaultAllocator, schema)
	defer builder.Release()
	builder.Field(0).(*array.Int32Builder).Append(id)
	builder.Field(1).(*array.StringBuilder).Append(label)
	record := builder.NewRecord()
	defer record.Release()
	if err := table.Add(context.Background(), record, nil); err != nil {
		t.Fatalf("add row: %v", err)
	}
}

func integrationSchema(t *testing.T) (*arrow.Schema, contracts.ISchema) {
	t.Helper()
	arrowSchema := arrow.NewSchema([]arrow.Field{
		{Name: "id", Type: arrow.PrimitiveTypes.Int32, Nullable: false},
		{Name: "label", Type: arrow.BinaryTypes.String, Nullable: false},
	}, nil)
	schema, err := internal.NewSchema(arrowSchema)
	if err != nil {
		t.Fatalf("schema: %v", err)
	}
	return arrowSchema, schema
}

func assertRows(t *testing.T, table contracts.ITable, want int64) {
	t.Helper()
	count, err := table.Count(context.Background())
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != want {
		t.Fatalf("count = %d, want %d", count, want)
	}
	rows, err := table.SelectWithFilter(context.Background(), "id >= 0")
	if err != nil {
		t.Fatalf("select: %v", err)
	}
	if int64(len(rows)) != want {
		t.Fatalf("select rows = %d, want %d", len(rows), want)
	}
}

// TestMinIOAuthenticatedCRUD uses a real MinIO server with isolated synthetic
// credentials. It verifies create, append, query and reopen on the embedded S3
// path without any production service.
func ensureBucket(t *testing.T, endpoint, accessKey, secretKey, bucket string) {
	t.Helper()
	parsed, err := url.Parse(endpoint)
	if err != nil {
		t.Fatalf("parse S3 endpoint %q: %v", endpoint, err)
	}
	client, err := minio.New(parsed.Host, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: parsed.Scheme == "https",
	})
	if err != nil {
		t.Fatalf("create S3 client: %v", err)
	}
	if err := client.MakeBucket(context.Background(), bucket, minio.MakeBucketOptions{}); err != nil {
		exists, checkErr := client.BucketExists(context.Background(), bucket)
		if checkErr != nil || !exists {
			t.Fatalf("create bucket %q: %v (exists check: %v)", bucket, err, checkErr)
		}
	}
}

func TestMinIOAuthenticatedCRUD(t *testing.T) {
	endpoint, accessKey, secretKey, bucket := minioConfig(t)
	ctx := context.Background()
	prefix := fmt.Sprintf("minio-crud-%d", os.Getpid())
	options := map[string]string{
		contracts.StorageAWSAccessKeyID:     accessKey,
		contracts.StorageAWSSecretAccessKey: secretKey,
		contracts.StorageAWSRegion:          "us-east-1",
		contracts.StorageEndpoint:           endpoint,
		contracts.StorageAllowHTTP:          "true",
		contracts.StorageS3AddressingStyle:  "path",
	}
	db, err := lancedb.ConnectWithStorageOptions(ctx, "s3://"+bucket+"/"+prefix, options)
	if err != nil {
		t.Fatalf("connect MinIO: %v", err)
	}
	arrowSchema, schema := integrationSchema(t)
	table, err := db.CreateTable(ctx, "items", schema)
	if err != nil {
		t.Fatalf("create table: %v", err)
	}
	addSingleRow(t, table, arrowSchema, 1, "first")
	assertRows(t, table, 1)
	if err := table.Close(); err != nil {
		t.Fatalf("close table: %v", err)
	}
	reopened, err := db.OpenTable(ctx, "items")
	if err != nil {
		t.Fatalf("reopen table: %v", err)
	}
	assertRows(t, reopened, 1)
	_ = reopened.Close()
	_ = db.Close()
}

// dynamoAuth is one signed request observed by the loopback DynamoDB fixture.
type dynamoAuth struct {
	Table    string
	Target   string
	Access   string
	Region   string
	Token    string
	BodySize int
}

type ddbAttr map[string]string
type ddbItem map[string]ddbAttr

type fakeDynamo struct {
	mu       sync.Mutex
	items    map[string]ddbItem
	requests []dynamoAuth
	server   *httptest.Server
}

var credentialPattern = regexp.MustCompile(`Credential=([^/]+)/[^/]+/([^/]+)/`)

func newFakeDynamo(t *testing.T) *fakeDynamo {
	t.Helper()
	f := &fakeDynamo{items: map[string]ddbItem{}}
	f.server = httptest.NewServer(http.HandlerFunc(f.handle))
	t.Cleanup(f.server.Close)
	return f
}

func (f *fakeDynamo) handle(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	target := r.Header.Get("X-Amz-Target")
	operation := strings.TrimPrefix(target, "DynamoDB_20120810.")
	auth := dynamoAuth{Target: operation, BodySize: len(body)}
	if match := credentialPattern.FindStringSubmatch(r.Header.Get("Authorization")); len(match) == 3 {
		auth.Access = match[1]
		auth.Region = match[2]
	}
	auth.Token = r.Header.Get("X-Amz-Security-Token")
	if os.Getenv("DEBUG_DDB") != "" {
		fmt.Fprintf(os.Stderr, "DDB %s headers: %+v\n", operation, r.Header)
	}

	switch operation {
	case "DescribeTable":
		auth.Table = jsonString(body, "TableName")
		f.record(auth)
		writeDynamoJSON(w, 200, map[string]any{"Table": map[string]any{
			"AttributeDefinitions": []any{
				map[string]any{"AttributeName": "base_uri", "AttributeType": "S"},
				map[string]any{"AttributeName": "version", "AttributeType": "N"},
			},
			"KeySchema": []any{
				map[string]any{"AttributeName": "base_uri", "KeyType": "HASH"},
				map[string]any{"AttributeName": "version", "KeyType": "RANGE"},
			},
			"TableStatus": "ACTIVE",
		}})
	case "Query":
		var request struct {
			TableName                 string             `json:"TableName"`
			ExpressionAttributeValues map[string]ddbAttr `json:"ExpressionAttributeValues"`
			Limit                     int                `json:"Limit"`
			ScanIndexForward          *bool              `json:"ScanIndexForward"`
		}
		if err := json.Unmarshal(body, &request); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		auth.Table = request.TableName
		f.record(auth)
		baseURI := request.ExpressionAttributeValues[":base_uri"]["S"]
		f.mu.Lock()
		items := make([]ddbItem, 0)
		for _, item := range f.items {
			if item["base_uri"]["S"] == baseURI {
				items = append(items, item)
			}
		}
		f.mu.Unlock()
		sort.Slice(items, func(i, j int) bool {
			left, _ := strconv.ParseUint(items[i]["version"]["N"], 10, 64)
			right, _ := strconv.ParseUint(items[j]["version"]["N"], 10, 64)
			if request.ScanIndexForward != nil && !*request.ScanIndexForward {
				return left > right
			}
			return left < right
		})
		if request.Limit > 0 && len(items) > request.Limit {
			items = items[:request.Limit]
		}
		writeDynamoJSON(w, 200, map[string]any{"Items": items, "Count": len(items), "ScannedCount": len(items)})
	case "PutItem":
		var request struct {
			TableName           string  `json:"TableName"`
			Item                ddbItem `json:"Item"`
			ConditionExpression string  `json:"ConditionExpression"`
		}
		if err := json.Unmarshal(body, &request); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		auth.Table = request.TableName
		f.record(auth)
		key := request.Item["base_uri"]["S"] + "\x00" + request.Item["version"]["N"]
		f.mu.Lock()
		_, exists := f.items[key]
		f.mu.Unlock()
		condition := request.ConditionExpression
		if strings.Contains(condition, "attribute_not_exists") && exists {
			writeDynamoError(w, "ConditionalCheckFailedException")
			return
		}
		if strings.Contains(condition, "attribute_exists") && !exists {
			writeDynamoError(w, "ConditionalCheckFailedException")
			return
		}
		f.mu.Lock()
		f.items[key] = request.Item
		f.mu.Unlock()
		writeDynamoJSON(w, 200, map[string]any{})
	case "GetItem":
		var request struct {
			TableName string  `json:"TableName"`
			Key       ddbItem `json:"Key"`
		}
		_ = json.Unmarshal(body, &request)
		auth.Table = request.TableName
		f.record(auth)
		key := request.Key["base_uri"]["S"] + "\x00" + request.Key["version"]["N"]
		f.mu.Lock()
		item, ok := f.items[key]
		f.mu.Unlock()
		response := map[string]any{}
		if ok {
			response["Item"] = item
		}
		writeDynamoJSON(w, 200, response)
	case "BatchWriteItem":
		var request struct {
			RequestItems map[string][]struct {
				DeleteRequest struct {
					Key ddbItem `json:"Key"`
				} `json:"DeleteRequest"`
			} `json:"RequestItems"`
		}
		if err := json.Unmarshal(body, &request); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		for table, writes := range request.RequestItems {
			auth.Table = table
			f.record(auth)
			f.mu.Lock()
			for _, write := range writes {
				key := write.DeleteRequest.Key["base_uri"]["S"] + "\x00" + write.DeleteRequest.Key["version"]["N"]
				delete(f.items, key)
			}
			f.mu.Unlock()
		}
		writeDynamoJSON(w, 200, map[string]any{"UnprocessedItems": map[string]any{}})
	default:
		http.Error(w, "unsupported DynamoDB operation "+operation, http.StatusBadRequest)
	}
}

func (f *fakeDynamo) record(auth dynamoAuth) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.requests = append(f.requests, auth)
}

func (f *fakeDynamo) authFor(table string) []dynamoAuth {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]dynamoAuth, 0)
	for _, request := range f.requests {
		if request.Table == table {
			out = append(out, request)
		}
	}
	return out
}

func jsonString(body []byte, key string) string {
	var value map[string]json.RawMessage
	_ = json.Unmarshal(body, &value)
	var text string
	_ = json.Unmarshal(value[key], &text)
	return text
}

func writeDynamoJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/x-amz-json-1.0")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeDynamoError(w http.ResponseWriter, code string) {
	w.Header().Set("Content-Type", "application/x-amz-json-1.0")
	w.WriteHeader(http.StatusBadRequest)
	_, _ = w.Write([]byte(`{"__type":"com.amazonaws.dynamodb.v20120810#` + code + `","message":"The conditional request failed"}`))
}

func writeAWSProfiles(t *testing.T, accessKey, secretKey string) {
	t.Helper()
	dir := t.TempDir()
	config := filepath.Join(dir, "config")
	credentials := filepath.Join(dir, "credentials")
	if err := os.WriteFile(config, []byte(`[profile primary]
region = eu-west-1
endpoint_url = http://127.0.0.1:1

[profile secondary]
region = ap-southeast-2
endpoint_url = http://127.0.0.1:1
`), 0o600); err != nil {
		t.Fatalf("write AWS config: %v", err)
	}
	if err := os.WriteFile(credentials, []byte(`[primary]
aws_access_key_id = `+accessKey+`
aws_secret_access_key = `+secretKey+`

[secondary]
aws_access_key_id = TESTSECONDARY
aws_secret_access_key = secondary-secret
`), 0o600); err != nil {
		t.Fatalf("write AWS credentials: %v", err)
	}
	t.Setenv("AWS_CONFIG_FILE", config)
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", credentials)
	t.Setenv("AWS_PROFILE", "secondary")
	t.Setenv("AWS_EC2_METADATA_DISABLED", "true")
	for _, key := range []string{
		"AWS_ACCESS_KEY_ID",
		"AWS_SECRET_ACCESS_KEY",
		"AWS_SESSION_TOKEN",
		"AWS_SECURITY_TOKEN",
		"AWS_REGION",
		"AWS_DEFAULT_REGION",
		"AWS_ENDPOINT_URL",
		"AWS_ENDPOINT_URL_S3",
		"AWS_ENDPOINT_URL_DYNAMODB",
		"AWS_ROLE_ARN",
		"AWS_ROLE_SESSION_NAME",
		"AWS_WEB_IDENTITY_TOKEN_FILE",
		"AWS_CONTAINER_CREDENTIALS_FULL_URI",
		"AWS_CONTAINER_CREDENTIALS_RELATIVE_URI",
	} {
		unsetEnv(t, key)
	}
}

func unsetEnv(t *testing.T, key string) {
	t.Helper()
	previous, existed := os.LookupEnv(key)
	if err := os.Unsetenv(key); err != nil {
		t.Fatalf("unset %s: %v", key, err)
	}
	t.Cleanup(func() {
		if existed {
			_ = os.Setenv(key, previous)
		} else {
			_ = os.Unsetenv(key)
		}
	})
}

func TestAWSProfileAndDynamoDBEndpoint(t *testing.T) {
	// Two S3-compatible gateways with different regions prove both explicit
	// aws_region precedence and profile-region fallback. Both accept only the
	// primary profile credentials, so a silently ambient secondary profile
	// fails instead of producing a false positive.
	accessKey, secretKey := "TESTPRIMARY", "primary-secret"
	endpointA := startS3Gateway(t, "ca-central-1", accessKey, secretKey)
	endpointB := startS3Gateway(t, "eu-west-1", accessKey, secretKey)
	bucket := "profile-bucket"
	ensureBucket(t, endpointA, accessKey, secretKey, bucket)
	ensureBucket(t, endpointB, accessKey, secretKey, bucket)
	fake := newFakeDynamo(t)
	writeAWSProfiles(t, accessKey, secretKey)
	ctx := context.Background()
	arrowSchema, schema := integrationSchema(t)

	connect := func(prefix, ddbTable, region, endpoint string) (contracts.IConnection, contracts.ITable) {
		t.Helper()
		options := map[string]string{
			contracts.StorageAWSProfile:        "primary",
			contracts.StorageEndpoint:          endpoint,
			contracts.StorageAllowHTTP:         "true",
			contracts.StorageS3AddressingStyle: "path",
			contracts.StorageDynamoDBEndpoint:  fake.server.URL,
		}
		if region != "" {
			options[contracts.StorageAWSRegion] = region
		}
		uri := "s3+ddb://" + bucket + "/" + prefix + "?ddbTableName=" + ddbTable
		db, err := lancedb.ConnectWithStorageOptions(ctx, uri, options)
		if err != nil {
			t.Fatalf("connect %s: %v", uri, err)
		}
		table, err := db.CreateTable(ctx, "items", schema)
		if err != nil {
			_ = db.Close()
			t.Fatalf("create table over s3+ddb: %v", err)
		}
		addSingleRow(t, table, arrowSchema, 1, "profile")
		assertRows(t, table, 1)
		if err := table.Close(); err != nil {
			t.Fatalf("close table: %v", err)
		}
		return db, table
	}

	dbA, _ := connect("profile-explicit-region", "ddb_explicit_region", "ca-central-1", endpointA)
	_ = dbA.Close()
	dbB, _ := connect("profile-default-region", "ddb_default_region", "", endpointB)
	_ = dbB.Close()

	for _, test := range []struct {
		table  string
		region string
	}{
		{"ddb_explicit_region", "ca-central-1"},
		{"ddb_default_region", "eu-west-1"},
	} {
		requests := fake.authFor(test.table)
		if len(requests) == 0 {
			t.Errorf("no DynamoDB requests for %s", test.table)
			continue
		}
		var sawPut, sawQuery bool
		for _, request := range requests {
			if request.Access != accessKey {
				t.Errorf("%s %s access key = %q, want profile primary %q", test.table, request.Target, request.Access, accessKey)
			}
			if request.Region != test.region {
				t.Errorf("%s %s region = %q, want %q", test.table, request.Target, request.Region, test.region)
			}
			if request.Token != "" {
				t.Errorf("%s %s unexpected session token %q; the static profile has no token", test.table, request.Target, request.Token)
			}
			if request.Target == "PutItem" {
				sawPut = true
			}
			if request.Target == "Query" {
				sawQuery = true
			}
		}
		if !sawPut || !sawQuery {
			t.Errorf("%s requests = %+v; want PutItem and Query", test.table, requests)
		}
	}
}

// startS3Gateway starts an isolated, authenticated S3-compatible gateway for
// one region. The fixture uses synthetic credentials and a temporary POSIX
// directory; no external service or production credential is involved.
func startS3Gateway(t *testing.T, region, accessKey, secretKey string) string {
	t.Helper()
	binary := os.Getenv("VERSITYGW_BIN")
	if binary == "" {
		t.Skip("set VERSITYGW_BIN to a versitygw binary to run the profile/region integration test")
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve gateway port: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	_ = listener.Close()
	dataDir := t.TempDir()
	address := fmt.Sprintf("127.0.0.1:%d", port)
	command := exec.Command(binary,
		"--access", accessKey,
		"--secret", secretKey,
		"--region", region,
		"--port", address,
		"posix", dataDir,
	)
	var output strings.Builder
	command.Stdout = &output
	command.Stderr = &output
	if err := command.Start(); err != nil {
		t.Fatalf("start gateway: %v", err)
	}
	t.Cleanup(func() {
		_ = command.Process.Kill()
		_, _ = command.Process.Wait()
		if t.Failed() {
			t.Logf("gateway %s output:\n%s", address, output.String())
		}
	})
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", address, 200*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return "http://" + address
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("gateway %s did not become ready: %s", address, output.String())
	return ""
}
