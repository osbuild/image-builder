#!/usr/bin/bash

set -euxo pipefail

BASEPATH="$(dirname "$(dirname "$(readlink -f "$0")")")"
cd "$BASEPATH"

# Setup osbuild-composer and update to the lastest version
schutzbot/provision-composer.sh

sudo dnf check-update osbuild-composer || {
    sudo dnf update osbuild-composer -y
}

# Rebuild image-builder image
which podman || sudo dnf install -y podman
sudo podman image exists image-builder && {
    # Remove image-builder and related containers
    sudo podman rmi image-builder --force
}
sudo podman build --security-opt "label=disable" -t image-builder -f distribution/Dockerfile-ubi .

# Start container
sudo podman run -d --pull=never --security-opt "label=disable" --net=host --name image-builder-test \
     -e LISTEN_ADDRESS=localhost:8087 -e OSBUILD_URL=https://localhost:443 \
     -e OSBUILD_CA_PATH=/etc/osbuild-composer/ca-crt.pem \
     -e OSBUILD_CERT_PATH=/etc/osbuild-composer/client-crt.pem \
     -e OSBUILD_KEY_PATH=/etc/osbuild-composer/client-key.pem \
     -e ALLOWED_ORG_IDS="000000" \
     -e DISTRIBUTIONS_DIR="/app/distributions" \
     -v /etc/osbuild-composer:/etc/osbuild-composer \
     image-builder

# Run integration test
go clean -testcache
go test ./cmd/... -tags=integration
