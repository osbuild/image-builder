#!/bin/bash
set -euo pipefail

if [[ -z "${KUBERNETES_PORT:-}" ]]; then
    echo "Starting image-builder inside container..."
else
    echo "Starting image-builder inside OpenShift..."
    echo "Cloudwatch logs: ${CW_LOG_GROUP_NAME} in ${CW_AWS_REGION}"
    echo "Database: ${PGDATABASE} on ${PGHOST}:${PGPORT}"
fi

/app/image-builder -v
