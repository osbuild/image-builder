#!/bin/bash
set -euo pipefail

if [[ -z "${KUBERNETES_PORT:-}" ]]; then
    echo "Starting image-builder inside container..."
else
    echo "Starting image-builder inside OpenShift..."
    echo "Cloudwatch: ${CW_LOG_GROUP} in ${CW_AWS_REGION}"
    echo "Database: ${PGDATABASE} on ${PGHOST}:${PGPORT}"
fi

/app/image-builder -v
