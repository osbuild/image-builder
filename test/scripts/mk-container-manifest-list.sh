#!/usr/bin/bash
#
# Create four containers (one for each architecture) with a simple README file
# for testing resolving multi-arch manifest lists.
#
# Original source: https://github.com/osbuild/osbuild/commit/532a4c1166fd8297606933c8c209bd5eef7695c2

set -eu

arches=(
    amd64
    arm64
    s390x
    ppc64le
)

container_ids=()

for arch in "${arches[@]}"; do
    echo ":: Building ${arch} container"
    container_name=$(buildah from --arch="${arch}" scratch)
    buildah config --created-by "osbuild.org" "${container_name}"
    readmedir=$(mktemp -d)
    pushd "${readmedir}"

    cat > "README.${arch}.md" << EOF
# Test container

Test container for preserving manifest list digest.
Architecture: ${arch}
EOF
buildah copy "${container_name}" "README.${arch}.md"
popd

    rm -r "${readmedir}"
    id=$(buildah commit --format=docker --rm "${container_name}")
    container_ids+=("${id}")
done

echo ":: Creating manifest"
name="registry.gitlab.com/redhat/services/products/image-builder/ci/osbuild-composer/manifest-list-test"
buildah manifest create "${name}" "${container_ids[@]}"

echo "Push to registry with:"
echo "podman manifest push ${name}"
