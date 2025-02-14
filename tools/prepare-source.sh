#!/bin/sh
set -eux

GO_VERSION=1.22.0
OAPI_VERSION=2.4.1

GO_BINARY=$(go env GOPATH)/bin/go$GO_VERSION
TOOLS_PATH="$(realpath "$(dirname "$0")/bin")"

# Install Go SDK
go install golang.org/dl/go$GO_VERSION@latest
$GO_BINARY download

# Ensure dev tools are installed
test -e "$TOOLS_PATH/goimports" || GOBIN=$TOOLS_PATH $GO_BINARY install golang.org/x/tools/cmd/goimports@latest
("$TOOLS_PATH/oapi-codegen" -version | grep "$OAPI_VERSION" >/dev/null) || GOBIN=$TOOLS_PATH $GO_BINARY install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v$OAPI_VERSION

GOBIN=$TOOLS_PATH $GO_BINARY generate -x -mod=mod ./...

# ... the code is formatted correctly, ...
"$TOOLS_PATH/goimports" -w internal cmd
$GO_BINARY fmt -mod=mod ./internal/... ./cmd/...

# ... and that go.mod and go.sum are up to date.
$GO_BINARY mod tidy
