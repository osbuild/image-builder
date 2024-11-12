#!/usr/bin/env bash

go test -c -tags=integration -o image-builder-db-test ./cmd/image-builder-db-test/
./image-builder-db-test
