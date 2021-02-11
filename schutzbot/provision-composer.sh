#!/bin/bash
set -euxo pipefail

# Get OS and architecture details.
source /etc/os-release
ARCH=$(uname -m)

echo "Enabling fastestmirror to speed up dnf 🏎️"
echo -e "fastestmirror=1" | sudo tee -a /etc/dnf/dnf.conf

# Set up osbuild-composer repo
DNF_REPO_BASEURL=http://osbuild-composer-repos.s3-website.us-east-2.amazonaws.com
OSBUILD_COMMIT=20a142d8f9b8b5d0a69f4d91631dc94118d209ca
OSBUILD_COMPOSER_COMMIT=a85511c6de917e60952832fda4c5b5e1f7c3857f
sudo tee /etc/yum.repos.d/osbuild.repo << EOF
[osbuild]
name=osbuild ${OSBUILD_COMMIT}
baseurl=${DNF_REPO_BASEURL}/osbuild/${ID}-${VERSION_ID}/${ARCH}/${OSBUILD_COMMIT}
enabled=1
gpgcheck=0
priority=5
[osbuild-composer]
name=osbuild-composer ${OSBUILD_COMPOSER_COMMIT}
baseurl=${DNF_REPO_BASEURL}/osbuild-composer/${ID}-${VERSION_ID}/${ARCH}/${OSBUILD_COMPOSER_COMMIT}
enabled=1
gpgcheck=0
priority=6
EOF

# Install osbuild-composer
sudo dnf install -y osbuild-composer composer-cli

sudo mkdir -p /etc/osbuild-composer
sudo cp -a schutzbot/osbuild-composer.toml /etc/osbuild-composer/


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
