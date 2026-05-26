#!/usr/bin/bash
#
# Tests manifest generation for non-determinism by generating manifests N times
# and comparing them. Exits with 1 if any differences are found.
#
# Usage: test-manifest-checksums.sh [N]
#   N: number of iterations (default: 5)

set -euo pipefail

N=${1:-5}

testdir="/tmp/test-manifest"
# shellcheck disable=SC2329  # Function invoked via trap
cleanup_testdir() {
    rm -rf "${testdir}"
}
trap cleanup_testdir EXIT

mkdir -p "${testdir}"

tmpdir="$(mktemp -d)"
# shellcheck disable=SC2329  # Function invoked via trap
cleanup_tmpdir() {
    rm -r "${tmpdir}"
}
trap cleanup_tmpdir EXIT

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

echo "Generating and comparing manifests $N times..."
exit_code=0
for i in $(seq 1 "$N"); do
    printf "  Run %d/%d" "$i" "$N"
    mkdir -p "${testdir}/$i"

    # Scale workers based on iteration count: 2 + N*3 gives 5-92 workers for N=1-30
    workers=$((2 + N * 3))

    # NOTE: fedora-41 riscv has no test repositories so we need to skip it.
    # NOTE: silence stdout as it gets way too noisy.
    # Save stderr to reduce noise as well and print it only if the run fails.
    if ! "${tmpdir}/bin/gen-manifests" \
        --workers="$workers" \
        --packages=false --containers=false --commits=false --flatpaks=false \
        --metadata=false \
        --fake-bootc=true \
        --arches "x86_64,aarch64,ppc64le,s390x" \
        --output "${testdir}/$i" \
        > /dev/null 2> "${tmpdir}/stderr-$i"; then

        cat "${tmpdir}/stderr-$i"
        exit 1
    fi

    # Compare with run 1 (starting from run 2)
    if [ "$i" -gt 1 ]; then
        printf " - comparing..."
        if ! diff -r "${testdir}/1" "${testdir}/$i" > "${tmpdir}/diff-$i" 2>&1; then
            echo # newline
            echo
            echo "DIFFERENCE FOUND between run 1 and run $i:"
            echo
            cat "${tmpdir}/diff-$i" | head -100
            if [ "$(wc -l < "${tmpdir}/diff-$i")" -gt 100 ]; then
                echo
                echo "... (showing first 100 lines of diff, total $(wc -l < "${tmpdir}/diff-$i") lines)"
            fi
            exit_code=1
            break
        fi
    fi
    echo # newline after each run
done

if [ $exit_code -eq 0 ]; then
    echo "✓ All $N runs produced identical manifests"
fi

exit $exit_code
