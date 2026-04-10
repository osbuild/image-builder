#!/usr/bin/bash
#
# Generates mock manifests (i.e. without real resolved content) for all test
# configurations and computes the checksum for each file. The checksumss are stored
# in test/data/manifest-checksums.txt and should be updated whenever a manifest
# changes. This makes it visible when a change affects a manifest without
# needing to store real manifests in the repository.

set -euo pipefail

tmpdir="$(mktemp -d)"
cleanup() {
    rm -r "${tmpdir}"
}
trap cleanup EXIT

export OSBUILD_TESTING_RNG_SEED=0
export IMAGE_BUILDER_EXPERIMENTAL=gen-manifest-mock-bpfile-uris

# For the purposes of this script, failing to compile is not an error. It is
# preferable for all commits to compile, but sometimes it's necessary or
# desirable to relax this requirement and in those cases we want to ignore the
# specific commit.
echo "Checking if gen-manifests compiles"
if ! go build -v -o "${tmpdir}/bin/" ./cmd/gen-manifests; then
    echo "Failed to compile gen-manifests. Skipping..."
    exit 0
fi

# NOTE: fedora-41 riscv has no test repositories so we need to skip it.
# NOTE: silence stdout as it gets way too noisy in the GitHub action log (until
# gen-manifests gets a verbosity or progress option).
# Save stderr to reduce noise as well and print it only if the run fails.
echo "Generating mock manifests"
if ! "${tmpdir}/bin/gen-manifests" \
    --packages=false --containers=false --commits=false --flatpaks=false \
    --metadata=false \
    --fake-bootc=true \
    --arches "x86_64,aarch64,ppc64le,s390x" \
    --output "${tmpdir}/manifests" \
    > /dev/null 2> "${tmpdir}/stderr"; then

    cat "${tmpdir}/stderr"
    exit 1
fi


# NOTE: 'osbuild --inspect' is generally a better way to calculate a manifest
# fingerprint, because it ignores things like pipeline names, source URLs, and
# generally things that don't affect the build output.
# For mocked manifests though we want those things to be visible changes, so we
# calculate the checksum of the file directly. Also it's faster.
echo "Calculating checksums"
checksums_dir="./test/data/manifest-checksums"
rm -rf "${checksums_dir}"
mkdir -p "${checksums_dir}"

for manifest_path in "${tmpdir}/manifests/"*; do
    filename=$(basename "${manifest_path/.json}")
    sha1sum -- "${manifest_path}" | cut -d' ' -f1 > "${checksums_dir}/${filename}"
done

echo "Checksums saved to ${checksums_dir}"
