#!/bin/bash
set -euo pipefail

if [[ -z "${KUBERNETES_PORT:-}" ]]; then
    echo "Starting image-builder inside container..."
else
    echo "Starting image-builder inside OpenShift..."
    echo "Cloudwatch: ${CW_LOG_GROUP} in ${CW_AWS_REGION}"
    echo "Database: ${PGDATABASE} on ${PGHOST}:${PGPORT}"
    echo "Upload target: ${OSBUILD_AWS_S3_BUCKET} in ${OSBUILD_AWS_REGION}"
fi

/app/image-builder -v
