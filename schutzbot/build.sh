#!/bin/bash
set -euo pipefail

# Colorful output.
function greenprint {
  echo -e "\033[1;32m${1}\033[0m"
}

# Build the container and push to quay
ARCH=$(uname -m)
QUAY_REPO_URL="quay.io/osbuild/image-builder-test"
QUAY_REPO_TAG="${CI_PIPELINE_ID}-$ARCH"

sudo dnf install -y podman
greenprint "🎁 Building container"
sudo podman build --label="quay.expires-after=1w" --security-opt "label=disable" -t image-builder-"${QUAY_REPO_TAG}" -f distribution/Dockerfile-ubi .
greenprint "🚀 Pushing container to test registry"
sudo podman push --creds "${V2_QUAY_USERNAME}":"${V2_QUAY_PASSWORD}" image-builder-"${QUAY_REPO_TAG}" "${QUAY_REPO_URL}":"${QUAY_REPO_TAG}"
