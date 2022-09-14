#!/bin/bash
set -euxo pipefail

echo "Enabling fastestmirror to speed up dnf 🏎️"
echo -e "fastestmirror=1" | sudo tee -a /etc/dnf/dnf.conf

# Install any packages required during the test
sudo dnf install -y podman \
     libvirt-client \
     libvirt-daemon-kvm \
     virt-install \
     wget \
     qemu-img \
     qemu-kvm \
     jq

sudo podman pull docker://quay.io/osbuild/postgres:latest

# Start Postgres container
sudo podman run -p 5432:5432 --name image-builder-db \
      --health-cmd "pg_isready -U postgres -d imagebuilder" --health-interval 2s \
      --health-timeout 2s --health-retries 10 \
      -e POSTGRES_USER=postgres \
      -e POSTGRES_PASSWORD=foobar \
      -e POSTGRES_DB=imagebuilder \
      -d postgres

for RETRY in {1..10}; do
    if sudo podman healthcheck run image-builder-db  > /dev/null 2>&1; then
       break
    fi
    echo "Retrying in 2 seconds... $RETRY"
    sleep 2
done

# Pull image-builder image
QUAY_REPO_URL="quay.io/osbuild/image-builder-test"
QUAY_REPO_TAG="${CI_PIPELINE_ID}"
sudo podman pull --creds "${V2_QUAY_USERNAME}":"${V2_QUAY_PASSWORD}" "${QUAY_REPO_URL}":"${QUAY_REPO_TAG}"

# Migrate
sudo podman run --pull=never --security-opt "label=disable" --net=host \
     -e PGHOST=localhost -e PGPORT=5432 -e PGDATABASE=imagebuilder \
     -e PGUSER=postgres -e PGPASSWORD=foobar \
     -e MIGRATIONS_DIR="/app/migrations" \
     --name image-builder-migrate \
     image-builder-test:"${QUAY_REPO_TAG}" /app/image-builder-migrate-db
sudo podman logs image-builder-migrate

echo "{\"000000\":{\"quota\":5,\"slidingWindow\":1209600000000000},\"000001\":{\"quota\":0,\"slidingWindow\":1209600000000000}}" > /tmp/quotas
# Start Image Builder container
sudo podman run -d --pull=never --security-opt "label=disable" --net=host \
     -e COMPOSER_URL=https://api.stage.openshift.com: \
     -e COMPOSER_TOKEN_URL="https://sso.redhat.com/auth/realms/redhat-external/protocol/openid-connect/token" \
     -e COMPOSER_CLIENT_SECRET="${COMPOSER_CLIENT_SECRET:-}" \
     -e COMPOSER_CLIENT_ID="${COMPOSER_CLIENT_ID:-}" \
     -e OSBUILD_AWS_REGION="${AWS_REGION:-}" \
     -e OSBUILD_GCP_REGION="${GCP_REGION:-}" \
     -e OSBUILD_GCP_BUCKET="${GCP_BUCKET:-}" \
     -e OSBUILD_AZURE_LOCATION="${AZURE_LOCATION:-}" \
     -e PGHOST=localhost -e PGPORT=5432 -e PGDATABASE=imagebuilder \
     -e PGUSER=postgres -e PGPASSWORD=foobar \
     -e ALLOWED_ORG_IDS="000000" \
     -e DISTRIBUTIONS_DIR="/app/distributions" \
     -e QUOTA_FILE="/app/accounts_quotas.json" \
     -v /tmp/quotas:/app/accounts_quotas.json \
     --name image-builder \
     image-builder-test:"${QUAY_REPO_TAG}"
