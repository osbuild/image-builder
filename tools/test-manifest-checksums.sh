#!/usr/bin/bash
#
# Runs gen-manifests N times and checks that each run produces identical
# manifest JSON files. Compares consecutive runs as soon as the later run
# finishes and exits on the first mismatch.

set -euo pipefail

N="${1:-10}"
BASE="$(mktemp -d)"
BIN="${BASE}/gen-manifests"

trap 'rm -rf "${BASE}"' EXIT

export OSBUILD_TESTING_RNG_SEED=0
export IMAGE_BUILDER_EXPERIMENTAL=gen-manifest-mock-bpfile-uris

cd "$(git rev-parse --show-toplevel)"

echo "Building gen-manifests"
go build -v -o "${BIN}" ./cmd/gen-manifests

gen_flags=(
    --packages=false --containers=false --commits=false --flatpaks=false
    --metadata=false
    --fake-bootc=true
    --arches "x86_64,aarch64,ppc64le,s390x"
)

for i in $(seq 1 "${N}"); do
    out="${BASE}/${i}"
    mkdir -p "${out}"
    stdout="${BASE}/${i}.stdout"
    stderr="${BASE}/${i}.stderr"

    start=$(date +%s.%N)
    if ! "${BIN}" "${gen_flags[@]}" --output "${out}" > "${stdout}" 2> "${stderr}"; then
        echo "gen-manifests failed on run ${i}" >&2
        cat "${stdout}" >&2
        cat "${stderr}" >&2
        exit 1
    fi
    elapsed=$(awk -v s="${start}" -v e="$(date +%s.%N)" 'BEGIN { printf "%.2f", e - s }')
    printf 'Run %s/%s (%ss)\n' "${i}" "${N}" "${elapsed}"

    if (( i >= 2 )); then
        prev="${BASE}/$((i - 1))"
        echo "Comparing run $((i - 1)) with run ${i}"
        if ! diff -qr "${prev}" "${out}" > /dev/null; then
            echo "Manifest output differs between run $((i - 1)) and run ${i}" >&2
            diff -ru "${prev}" "${out}" >&2 || true
            exit 1
        fi
    fi
done

echo "All ${N} runs produced identical manifests."
