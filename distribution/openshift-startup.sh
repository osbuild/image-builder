#!/bin/bash
set -euo pipefail

if [[ -z "${KUBERNETES_PORT:-}" ]]; then
    echo "Starting image-builder inside container..."
    if [[ -n "${GODEBUG_PORT:-}" ]]; then
      echo "With golang debugger enabled on port ${GODEBUG_PORT} ..."
      echo "NOTE: you HAVE to attach the debugger NOW otherwise the image-builder-backend will not continue running"
      /usr/bin/dlv "--listen=:${GODEBUG_PORT}" --headless=true --api-version=2 exec /app/image-builder -- -v
      exit $?
    fi
# we don't use cloudwatch in ephemeral environment for now
elif [[ "${CLOWDER_ENABLED:=false}" == "true" ]]; then
    echo "Starting image-builder inside ephemeral environment..."
    echo "Composer URL: ${COMPOSER_URL}"
    echo "Composer token URL: ${COMPOSER_TOKEN_URL}"
    echo "Distributions dir: ${DISTRIBUTIONS_DIR}"
else
    echo "Starting image-builder inside OpenShift..."
    echo "Cloudwatch: ${CW_LOG_GROUP} in ${CW_AWS_REGION}"
    echo "Composer URL: ${COMPOSER_URL}"
    echo "Composer token URL: ${COMPOSER_TOKEN_URL}"
    echo "Distributions dir: ${DISTRIBUTIONS_DIR}"
fi

/app/image-builder -v
