#!/usr/bin/env bash

TERN_MIGRATIONS_DIR=${TERN_MIGRATIONS_DIR:-internal/db/migrations-tern}

# make path absolute to find migrations for sure
TERN_MIGRATIONS_DIR=$(realpath "$TERN_MIGRATIONS_DIR" --relative-base="$(dirname "$0")")

"$(go env GOPATH)"/bin/tern migrate -m "$TERN_MIGRATIONS_DIR"
