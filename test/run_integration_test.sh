#!/usr/bin/bash

#
# This script is to run image-builder integration test locally
#

set -euxo pipefail

BASEPATH="$(dirname "$(dirname "$(readlink -f "$0")")")"
cd "$BASEPATH"

# Rebuild image-builder image
which podman || sudo dnf install -y podman
sudo podman image exists image-builder && sudo podman rmi image-builder --force
sudo podman build --security-opt "label=disable" -t image-builder -f distribution/Dockerfile-ubi .
# Remove <none> images to free space
sudo podman image prune --force

# Deploy osbuild-composer and start 
schutzbot/deploy.sh

# Update to the lastest version
sudo dnf check-update osbuild-composer || {
    sudo dnf update osbuild-composer -y
}

# Run integration test
bash test/cases/api.sh
