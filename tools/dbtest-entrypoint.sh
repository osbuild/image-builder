#!/usr/bin/env bash

set -euxo pipefail

# compile and execute instead of a plain
# "go test", to have the correct working directory

go test -c -tags=dbtests -o image-builder-db-test ./cmd/image-builder-db-test/
./image-builder-db-test

go test -c -tags=dbtests -o image-builder-maintenance-test ./cmd/image-builder-maintenance/
./image-builder-maintenance-test
