#!/bin/sh

set -eux

# Curl latest composer v2 spec
curl https://raw.githubusercontent.com/osbuild/osbuild-composer/main/internal/cloudapi/v2/openapi.v2.yml \
     -o internal/composer/openapi.v2.yml

GO_VERSION=1.16.15
GO_BINARY=$(go env GOPATH)/bin/go$GO_VERSION

# this is the official way to get a different version of golang
# see https://go.dev/doc/manage-install
go install golang.org/dl/go$GO_VERSION@latest
$GO_BINARY download


# ensure that go.mod and go.sum are up to date, ...
$GO_BINARY mod tidy
$GO_BINARY mod vendor

# ... the code is formatted correctly, ...
$GO_BINARY fmt ./...

# ... and all code has been regenerated from its sources.
$GO_BINARY generate ./...
