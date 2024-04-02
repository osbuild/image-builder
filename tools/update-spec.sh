#!/bin/sh
set -eux

# Curl latest composer v2 spec
curl https://raw.githubusercontent.com/osbuild/osbuild-composer/main/internal/cloudapi/v2/openapi.v2.yml \
     -o internal/clients/composer/openapi.v2.yml

# Curl latest provisioning spec
curl https://raw.githubusercontent.com/RHEnVision/provisioning-backend/main/api/openapi.gen.yaml \
     -o internal/clients/provisioning/provisioning.v1.yml

tools/prepare-source.sh
