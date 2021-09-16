#!/bin/bash
set -euxo pipefail

# Get OS and architecture details.
source /etc/os-release
ARCH=$(uname -m)

echo "Enabling fastestmirror to speed up dnf 🏎️"
echo -e "fastestmirror=1" | sudo tee -a /etc/dnf/dnf.conf

# Install image-builder packages
REPO_DIR=repo/image-builder/"${CI_PIPELINE_ID}"
sudo dnf localinstall -y "$REPO_DIR"/*"${ARCH}".rpm

# Retrieve composer&osbuild version from current stage deployment
pushd "$(mktemp -d)"
chmod 600 "$TERRAFORM_REPO_DEPLOY_KEY"
GIT_SSH_COMMAND="ssh -i $TERRAFORM_REPO_DEPLOY_KEY -oStrictHostKeyChecking=no" git clone --depth 1 --sparse git@github.com:osbuild/image-builder-terraform.git
OSBUILD_COMMIT="$(cat image-builder-terraform/terraform.tfvars.json | jq -r .osbuild_commit)"
OSBUILD_COMPOSER_COMMIT="$(cat image-builder-terraform/terraform.tfvars.json | jq -r .composer_commit)"
popd

DNF_REPO_BASEURL=http://osbuild-composer-repos.s3.amazonaws.com
sudo tee /etc/yum.repos.d/osbuild.repo << EOF
[osbuild]
name=osbuild ${OSBUILD_COMMIT}
baseurl=${DNF_REPO_BASEURL}/osbuild/rhel-${VERSION_ID%.*}-cdn/${ARCH}/${OSBUILD_COMMIT}
enabled=1
gpgcheck=0
priority=5
[osbuild-composer]
name=osbuild-composer ${OSBUILD_COMPOSER_COMMIT}
baseurl=${DNF_REPO_BASEURL}/osbuild-composer/rhel-${VERSION_ID%.*}-cdn/${ARCH}/${OSBUILD_COMPOSER_COMMIT}
enabled=1
gpgcheck=0
priority=6
EOF

# Install osbuild-composer
sudo dnf install -y osbuild-composer composer-cli

sudo mkdir -p /etc/osbuild-composer
sudo cp -a schutzbot/osbuild-composer.toml /etc/osbuild-composer/

sudo mkdir -p /etc/osbuild-worker

# if GCP credentials are defined in the ENV, add them to the worker's configuration
GOOGLE_APPLICATION_CREDENTIALS="${GOOGLE_APPLICATION_CREDENTIALS:-}"
if [[ -n "$GOOGLE_APPLICATION_CREDENTIALS" ]]; then
     # The credentials file must be copied to a different location. Jenkins places
     # it into /tmp and as a result, the worker would not see it due to using PrivateTmp=true.
     GCP_CREDS_WORKER_PATH="/etc/osbuild-worker/gcp-credentials.json"
     sudo cp "$GOOGLE_APPLICATION_CREDENTIALS" "$GCP_CREDS_WORKER_PATH"
     echo -e "\n[gcp]\ncredentials = \"$GCP_CREDS_WORKER_PATH\"\n" | sudo tee -a /etc/osbuild-worker/osbuild-worker.toml
fi

# if Azure credentials are defined in the env, create the credentials file
AZURE_CLIENT_ID="${AZURE_CLIENT_ID:-}"
AZURE_CLIENT_SECRET="${AZURE_CLIENT_SECRET:-}"
if [[ -n "$AZURE_CLIENT_ID" && -n "$AZURE_CLIENT_SECRET" ]]; then
     sudo tee /etc/osbuild-worker/azure-credentials.toml > /dev/null << EOF
client_id =     "$AZURE_CLIENT_ID"
client_secret = "$AZURE_CLIENT_SECRET"
EOF
     sudo tee -a /etc/osbuild-worker/osbuild-worker.toml > /dev/null << EOF

[azure]
credentials = "/etc/osbuild-worker/azure-credentials.toml"
EOF
fi


# Copy Fedora rpmrepo snapshots for use in weldr tests. RHEL's are usually more
# stable, and not available publically from rpmrepo.
sudo mkdir -p /etc/osbuild-composer/repositories
sudo cp -a schutzbot/repositories/fedora-*.json \
     /etc/osbuild-composer/repositories/

# Generate all X.509 certificates for the tests
./schutzbot/generate-certs.sh

sudo systemctl enable --now osbuild-composer.socket
sudo systemctl enable --now osbuild-composer-api.socket

# The keys were regenerated but osbuild-composer might be already running.
# Let's try to restart it. In ideal world, this shouldn't be needed as every
# test case is supposed to run on a pristine machine. However, this is
# currently not true on Schutzbot
sudo systemctl try-restart osbuild-composer

# Basic verification
sudo composer-cli status show
sudo composer-cli sources list
for SOURCE in $(sudo composer-cli sources list); do
    sudo composer-cli sources info "$SOURCE"
done

# Currently openstack/rhel-8.4-x86_64 cannot subcribe, subscription is disabled.
# In a non-subscribed system, cannot pull the Postgres container. So manually download it from quay.io
# Remove this after openstack/rhel-8.4-x86_64 can subscribe
sudo dnf install -y podman
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
sudo podman pull --creds "${QUAY_USERNAME}":"${QUAY_PASSWORD}" "${QUAY_REPO_URL}":"${QUAY_REPO_TAG}"

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
sudo podman run -d -p 8086:8086 --pull=never --security-opt "label=disable" --net=host \
     -e OSBUILD_URL=https://localhost:443 \
     -e OSBUILD_CA_PATH=/etc/osbuild-composer/ca-crt.pem \
     -e OSBUILD_CERT_PATH=/etc/osbuild-composer/client-crt.pem \
     -e OSBUILD_KEY_PATH=/etc/osbuild-composer/client-key.pem \
     -e OSBUILD_AWS_REGION="${AWS_REGION:-}"\
     -e OSBUILD_AWS_ACCESS_KEY_ID="${AWS_ACCESS_KEY_ID:-}" \
     -e OSBUILD_AWS_SECRET_ACCESS_KEY="${AWS_SECRET_ACCESS_KEY:-}" \
     -e OSBUILD_AWS_S3_BUCKET="${AWS_BUCKET:-}" \
     -e OSBUILD_GCP_REGION="${GCP_REGION:-}" \
     -e OSBUILD_GCP_BUCKET="${GCP_BUCKET:-}" \
     -e OSBUILD_AZURE_LOCATION="${AZURE_LOCATION:-}" \
     -e PGHOST=localhost -e PGPORT=5432 -e PGDATABASE=imagebuilder \
     -e PGUSER=postgres -e PGPASSWORD=foobar \
     -e ALLOWED_ORG_IDS="000000" \
     -e DISTRIBUTIONS_DIR="/app/distributions" \
     -e QUOTA_FILE="/app/accounts_quotas.json" \
     -v /etc/osbuild-composer:/etc/osbuild-composer \
     -v /tmp/quotas:/app/accounts_quotas.json \
     --name image-builder \
     image-builder-test:"${QUAY_REPO_TAG}"
