#!/usr/bin/env bash

# compile and execute instead of a plain
# "go test", to have the correct working directory

go test -c -tags=integration -o image-builder-db-test ./cmd/image-builder-db-test/
./image-builder-db-test
