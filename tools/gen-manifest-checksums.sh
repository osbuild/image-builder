#!/usr/bin/bash
#
# Generates mock manifests (i.e. without real resolved content) for all test
# configurations and computes the checksum for each file. The checksumss are stored
# in test/data/manifest-checksums.txt and should be updated whenever a manifest
# changes. This makes it visible when a change affects a manifest without
# needing to store real manifests in the repository.

set -euo pipefail

export OSBUILD_TESTING_RNG_SEED=0
export IMAGE_BUILDER_EXPERIMENTAL=gen-manifest-mock-bpfile-uris

# For the purposes of this script, failing to compile is not an error. It is
# preferable for all commits to compile, but sometimes it's necessary or
# desirable to relax this requirement and in those cases we want to ignore the
# specific commit.
echo "Checking if gen-manifests compiles"
if ! go build -v -o /tmp/gen-manifest-checksums-bin ./cmd/gen-manifests; then
    echo "Failed to compile gen-manifests. Skipping..."
    exit 0
fi

checksums_dir="./test/data/manifest-checksums"
mkdir -p "${checksums_dir}"

# NOTE: fedora-41 riscv has no test repositories so we need to skip it.
# NOTE: silence stdout as it gets way too noisy in the GitHub action log (until
# gen-manifests gets a verbosity or progress option).
# Save stderr to reduce noise as well and print it only if the run fails.
echo "Generating manifest checksums"
stderr="$(mktemp)"
trap 'rm -f "${stderr}"' EXIT
if ! /tmp/gen-manifest-checksums-bin \
    --checksums-only \
    --packages=false --containers=false --commits=false --flatpaks=false \
    --metadata=false \
    --fake-bootc=true \
    --arches "x86_64,aarch64,ppc64le,s390x" \
    --output "${checksums_dir}" \
    > /dev/null 2> "${stderr}"; then

    cat "${stderr}"
    exit 1
fi

echo "Checksums saved to ${checksums_dir}"
