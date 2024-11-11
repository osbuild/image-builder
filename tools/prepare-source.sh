#!/bin/sh
set -eux

GO_VERSION=1.22.0
GO_BINARY=$(go env GOPATH)/bin/go$GO_VERSION
OAPI_VERSION=2.3.0

# this is the official way to get a different version of golang
# see https://go.dev/doc/manage-install
go install golang.org/dl/go$GO_VERSION@latest
$GO_BINARY download

# Ensure dev tools are installed
which goimports || $GO_BINARY install golang.org/x/tools/cmd/goimports@latest

# Ensure that all code has been regenerated from its sources with the pinned oapi version
git grep -l "go:generate.*github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen" | grep -v prepare-source.sh | xargs sed -i "s|github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen[v@.0-9]*|github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v$OAPI_VERSION|g"
$GO_BINARY generate -mod=mod ./...

# ... the code is formatted correctly, ...
goimports -w internal cmd
$GO_BINARY fmt -mod=mod ./internal/... ./cmd/...

# ... and that go.mod and go.sum are up to date.
$GO_BINARY mod tidy
$GO_BINARY mod vendor
