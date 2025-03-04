#!/bin/sh
set -eu

GO_MAJOR_VER=1.22
GO_VERSION=1.22.9 # also update .github/workflows/tests.yml
OAPI_VERSION=2.4.1
TOOLS_PATH="$(realpath "$(dirname "$0")/bin")"

# Check latest Go version for the minor we're using
LATEST=$(curl -s https://endoflife.date/api/go/"${GO_MAJOR_VER}".json  | jq -r .latest)
if test "$LATEST" != "$GO_VERSION"; then
    echo "WARNING: A new minor release is available (${LATEST}), consider bumping the project version (${GO_VERSION})"
fi

set -x
export GOTOOLCHAIN=go$GO_VERSION
export GOSUMDB='sum.golang.org' # this is turned off for Go from Fedora / RHEL
go version

# Pin Go and toolchain versions at a reasonable version
go get go@$GO_VERSION toolchain@$GO_VERSION

# Ensure dev tools are installed
test -e "$TOOLS_PATH/goimports" || GOBIN=$TOOLS_PATH go install golang.org/x/tools/cmd/goimports@latest
("$TOOLS_PATH/oapi-codegen" -version | grep "$OAPI_VERSION" >/dev/null) || GOBIN=$TOOLS_PATH go install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v$OAPI_VERSION

# Generate source (skip vendor/):
GOBIN=$TOOLS_PATH go generate -x ./cmd/... ./internal/...

# Reformat source (skip vendor/):
"$TOOLS_PATH/goimports" -w ./internal ./cmd
go fmt ./cmd/... ./internal/...

# Update go.mod and go.sum as the last step
go mod tidy

