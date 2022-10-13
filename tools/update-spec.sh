#!/bin/sh
set -eux

# Curl latest composer v2 spec
curl https://raw.githubusercontent.com/osbuild/osbuild-composer/main/internal/cloudapi/v2/openapi.v2.yml \
     -o internal/composer/openapi.v2.yml


tools/prepare-source.sh
