#!/bin/bash
set -euo pipefail

if [[ -z "${KUBERNETES_PORT:-}" ]]; then
    echo "Starting image-builder inside container..."
else
    echo "Starting image-builder inside OpenShift..."
    echo "Cloudwatch: ${CW_LOG_GROUP} in ${CW_AWS_REGION}"
    echo "Composer URL: ${COMPOSER_URL}"
    echo "Composer token URL: ${COMPOSER_TOKEN_URL}"
    echo "Distributions dir: ${DISTRIBUTIONS_DIR}"
fi

/app/image-builder -v
