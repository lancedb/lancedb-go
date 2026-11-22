# SPDX-License-Identifier: Apache-2.0
# SPDX-FileCopyrightText: Copyright The LanceDB Authors

GO ?= go
CGO_ENABLED ?= 0
export CGO_ENABLED

.PHONY: all build test test-native test-integration vet fmt fmt-check install-native clean bench help

all: build test

build:
	$(GO) build ./...

test:
	$(GO) test -count=1 ./...

# Loader, cache and ABI-layout tests that do not need a native library.
test-native:
	$(GO) test -count=1 ./pkg/internal/native

# Requires an authenticated S3-compatible fixture and VERSITYGW_BIN for the
# profile/DynamoDB loopback test.
test-integration:
	$(GO) test -tags=integration -count=1 -v ./pkg/lancedb

vet:
	$(GO) vet ./...

fmt:
	gofmt -w $$(git ls-files '*.go')

fmt-check:
	@test -z "$$(gofmt -l $$(git ls-files '*.go'))" || { echo "run make fmt"; exit 1; }

# Download and verify the pinned lancedb-c artifact for this target.
install-native:
	$(GO) run ./pkg/cmd/lancedb-go-native

bench:
	$(GO) test -bench=. -benchmem ./...

clean:
	$(GO) clean -testcache

help:
	@echo "make build            build all packages with CGO_ENABLED=0"
	@echo "make test             run the Go test suite (downloads the native library if needed)"
	@echo "make test-native      run loader/layout tests without a native library"
	@echo "make test-integration run authenticated S3 and AWS profile/DynamoDB tests"
	@echo "make install-native   pre-download and verify the native library"
	@echo "make vet fmt fmt-check"
