#!/bin/sh
set -eux

OAPI_VERSION=2.4.1
TOOLS_PATH="$(realpath "$(dirname "$0")/bin")"

# Pin Go and toolbox versions at a reasonable version
go get go@1.22.0 toolchain@1.22.0

# Update go.mod and go.sum:
go mod tidy

# Ensure dev tools are installed
test -e "$TOOLS_PATH/goimports" || GOBIN=$TOOLS_PATH $GO_BINARY install golang.org/x/tools/cmd/goimports@latest
("$TOOLS_PATH/oapi-codegen" -version | grep "$OAPI_VERSION" >/dev/null) || GOBIN=$TOOLS_PATH $GO_BINARY install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v$OAPI_VERSION

# Generate source (skip vendor/):
GOBIN=$TOOLS_PATH go generate -x ./cmd/... ./internal/...

# Reformat source (skip vendor/):
goimports -w ./internal ./cmd
go fmt ./cmd/... ./internal/...
