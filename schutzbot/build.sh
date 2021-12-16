#!/bin/bash
set -euo pipefail

# Colorful output.
function greenprint {
  echo -e "\033[1;32m${1}\033[0m"
}

# Build the container and push to quay
QUAY_REPO_URL="quay.io/osbuild/image-builder-test"
QUAY_REPO_TAG="${CI_PIPELINE_ID}"

sudo dnf install -y podman
sudo setenforce 0 # todo
greenprint "🎁 Building container"
sudo podman build --label="quay.expires-after=1w" --security-opt "label=disable" -t image-builder-"${QUAY_REPO_TAG}" -f distribution/Dockerfile-ubi .
sudo setenforce 1
greenprint "🚀 Pushing container to test registry"
sudo podman push --creds "${V2_QUAY_USERNAME}":"${V2_QUAY_PASSWORD}" image-builder-"${CI_PIPELINE_ID}" "${QUAY_REPO_URL}":"${QUAY_REPO_TAG}"
