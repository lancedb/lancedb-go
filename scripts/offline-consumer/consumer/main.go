// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: Copyright The LanceDB Authors

// Command consumer exercises the published SDK as an external module consumer.
// The offline-consumer lane copies this file into a temporary module, builds it
// with CGO_ENABLED=0 against the public module, and runs it with an empty
// native cache, no explicit library path and no network access. It connects,
// creates and reads a table, exercises a native failure path and closes.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"strings"
	"time"

	"github.com/apache/arrow/go/v17/arrow/array"
	"github.com/apache/arrow/go/v17/arrow/memory"
	"github.com/lancedb/lancedb-go/pkg/lancedb"
)

const missingTable = "offline_consumer_missing"

func main() {
	uri := flag.String("uri", "", "database directory to use")
	probeNetwork := flag.Bool("probe-network", false, "probe direct outbound TCP egress and exit")
	probeAddress := flag.String("probe-address", "proxy.golang.org:443", "address for -probe-network")
	flag.Parse()
	if *probeNetwork {
		conn, err := net.DialTimeout("tcp", *probeAddress, 5*time.Second)
		if err != nil {
			log.Fatalf("network probe: %v", err)
		}
		if err := conn.Close(); err != nil {
			log.Fatalf("network probe close: %v", err)
		}
		fmt.Println("network_probe=egress-ok")
		return
	}
	if *uri == "" {
		log.Fatal("-uri is required")
	}
	ctx := context.Background()

	db, err := lancedb.Connect(ctx, *uri, nil)
	if err != nil {
		log.Fatalf("connect: %v", err)
	}

	schema, err := lancedb.NewSchemaBuilder().
		AddInt32Field("id", false).
		AddStringField("text", true).
		Build()
	if err != nil {
		_ = db.Close()
		log.Fatalf("schema: %v", err)
	}
	table, err := db.CreateTable(ctx, "offline_consumer", schema)
	if err != nil {
		_ = db.Close()
		log.Fatalf("create table: %v", err)
	}

	builder := array.NewRecordBuilder(memory.DefaultAllocator, schema.ToArrowSchema())
	ids := []int32{1, 2, 3}
	texts := []string{"one", "two", "three"}
	for i := range ids {
		builder.Field(0).(*array.Int32Builder).Append(ids[i])
		builder.Field(1).(*array.StringBuilder).Append(texts[i])
	}
	record := builder.NewRecord()
	builder.Release()
	if err := table.Add(ctx, record, nil); err != nil {
		record.Release()
		_ = table.Close()
		_ = db.Close()
		log.Fatalf("add: %v", err)
	}
	record.Release()

	count, err := table.Count(ctx)
	if err != nil {
		_ = table.Close()
		_ = db.Close()
		log.Fatalf("count: %v", err)
	}
	rows, err := table.SelectWithFilter(ctx, "id >= 1")
	if err != nil {
		_ = table.Close()
		_ = db.Close()
		log.Fatalf("select: %v", err)
	}

	// A native error path must stay an error and must not corrupt the
	// connection state.
	if _, openErr := db.OpenTable(ctx, missingTable); openErr == nil {
		_ = table.Close()
		_ = db.Close()
		log.Fatalf("opening %s unexpectedly succeeded", missingTable)
	} else if !strings.Contains(openErr.Error(), missingTable) {
		_ = table.Close()
		_ = db.Close()
		log.Fatalf("missing-table error does not identify the table: %v", openErr)
	}
	after, err := table.Count(ctx)
	if err != nil {
		_ = table.Close()
		_ = db.Close()
		log.Fatalf("count after failure: %v", err)
	}
	if err := table.Close(); err != nil {
		_ = db.Close()
		log.Fatalf("close table: %v", err)
	}
	if err := db.Close(); err != nil {
		log.Fatalf("close connection: %v", err)
	}

	version, err := lancedb.RuntimeVersion()
	if err != nil {
		log.Fatalf("runtime version: %v", err)
	}
	fmt.Printf("runtime_version=%s\n", version)
	fmt.Printf("rows_created=%d\n", count)
	fmt.Printf("rows_selected=%d\n", len(rows))
	fmt.Printf("failure_path=table_not_found\n")
	fmt.Printf("rows_after_failure=%d\n", after)
	fmt.Printf("closed=true\n")
}
