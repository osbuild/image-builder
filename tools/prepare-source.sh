#!/bin/sh

set -eux

# Go version must be consistent with image-builder which uses UBI
# container that is typically few months behind
GO_VERSION=1.24.12
GO_BINARY=$(go env GOPATH)/bin/go$GO_VERSION

# this is the official way to get a different version of golang
# see https://go.dev/doc/manage-install
go install golang.org/dl/go$GO_VERSION@latest
$GO_BINARY download

# Ensure that go.mod and go.sum are up to date.
$GO_BINARY mod tidy

# Check banned packages
if grep "github.com/(go-yaml/yaml|sirupsen/logrus)" go.mod | grep -v "// indirect" | grep -q .; then
	echo "error: banned direct dependency found" >&2
	exit 1
fi

# Ensure the code is formatted correctly.
$GO_BINARY fmt ./...

./test/scripts/generate-gitlab-ci ./.gitlab-ci.yml
