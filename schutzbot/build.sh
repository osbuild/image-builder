#!/bin/bash
set -euo pipefail

# Colorful output.
function greenprint {
  echo -e "\033[1;32m${1}\033[0m"
}

# Get OS and architecture details.
source /etc/os-release
ARCH=$(uname -m)

# Mock is only available in EPEL for RHEL.
if [[ $ID == rhel ]] && ! rpm -q epel-release; then
    greenprint "📦 Setting up EPEL repository"
    curl -Ls --retry 5 --output /tmp/epel.rpm \
        https://dl.fedoraproject.org/pub/epel/epel-release-latest-8.noarch.rpm
    sudo rpm -Uvh /tmp/epel.rpm
fi

# Currently openstack/rhel-8.4-x86_64 is beta image, the subscription will fail.
# Added condition to check if it is a beta image.
# TODO: remove condition to check beta after openstack/rhel-8.4-x86_64 can subscribe
# Register RHEL if we are provided with a registration script.
if [[ -n "${RHN_REGISTRATION_SCRIPT:-}" ]] && ! sudo subscription-manager status && ! sudo grep beta /etc/os-release; then
    greenprint "🪙 Registering RHEL instance"
    sudo chmod +x "$RHN_REGISTRATION_SCRIPT"
    sudo "$RHN_REGISTRATION_SCRIPT"
fi

# Install requirements for building RPMs in mock.
greenprint "📦 Installing mock requirements"
sudo dnf -y install createrepo_c make mock python3-pip rpm-build

# Jenkins sets a workspace variable as the root of its working directory.
WORKSPACE=${WORKSPACE:-$(pwd)}

# Mock configuration file to use for building RPMs.
MOCK_CONFIG="${ID}-${VERSION_ID%.*}-${ARCH}"

# Directory to hold the RPMs temporarily before we upload them.
REPO_DIR=repo/image-builder/${CI_PIPELINE_ID}

# Build source RPMs.
greenprint "🔧 Building source RPMs."
make srpm

greenprint "📦 RPMlint"
sudo dnf install -y rpmlint rpm-build make git-core
rpmlint rpmbuild/SRPMS/*

# Compile RPMs in a mock chroot
greenprint "🎁 Building RPMs with mock"
sudo mock -v -r "$MOCK_CONFIG" --resultdir "$REPO_DIR" --with=tests \
    rpmbuild/SRPMS/*.src.rpm

# Build the container and push to quay
QUAY_REPO_URL="quay.io/osbuild/image-builder-test"
QUAY_REPO_TAG="${CI_PIPELINE_ID}"

sudo dnf install -y podman
sudo setenforce 0 # todo
sudo podman build --security-opt "label=disable" -t image-builder-"${QUAY_REPO_TAG}" -f distribution/Dockerfile-ubi .
sudo setenforce 1

sudo podman push --creds "${QUAY_USERNAME}":"${QUAY_PASSWORD}" image-builder-"${CI_PIPELINE_ID}" "${QUAY_REPO_URL}":"${QUAY_REPO_TAG}"
