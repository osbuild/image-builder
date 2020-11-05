#!/bin/bash
set -euo pipefail

WORKING_DIRECTORY=/usr/libexec/image-builder
TESTS_PATH=/usr/libexec/tests/image-builder

PASSED_TESTS=()
FAILED_TESTS=()

TEST_CASES=(
  "image-builder-tests"
)

# Print out a nice test divider so we know when tests stop and start.
test_divider () {
    printf "%0.s-" {1..78} && echo
}

# Run a test case and store the result as passed or failed.
run_test_case () {
    TEST_NAME=$(basename "$1")
    echo
    test_divider
    echo "🏃🏻 Running test: ${TEST_NAME}"
    test_divider

    if sudo "${1}" -test.v | tee "${WORKSPACE}/${TEST_NAME}.log"; then
        PASSED_TESTS+=("$TEST_NAME")
    else
        FAILED_TESTS+=("$TEST_NAME")
    fi

    test_divider
    echo
}

# Check if iamge-builder-tests is installed.
sudo dnf -y install image-builder-tests


# Run postgres container and create imagebuilzder database
sudo podman run --security-opt "label=disable" -p5432:5432 --name imagebuilder -ePOSTGRES_PASSWORD=foobar -d postgres
journalctl -n30 --no-pager
TRIES=0
until sudo podman exec -u postgres imagebuilder psql -c "CREATE DATABASE imagebuilder;"; do
    sudo podman logs imagebuilder
    let TRIES="$TRIES + 1"
    if [ "$TRIES" -eq 3 ]; then
        echo "Unable to reach psql container ☹"
        exit 1
    fi
    sleep 3
done

# The integration test also runs a test against an image-builder container
sudo podman run -d --pull=never --security-opt "label=disable" --net=host \
     -e LISTEN_ADDRESS=localhost:8087 -e OSBUILD_URL=https://localhost:443/api/composer/v1 \
     -e OSBUILD_CA_PATH=/etc/osbuild-composer/ca-crt.pem \
     -e OSBUILD_CERT_PATH=/etc/osbuild-composer/client-crt.pem \
     -e OSBUILD_KEY_PATH=/etc/osbuild-composer/client-key.pem \
     -e PGHOST=localhost \
     -e PGPORT=5432 \
     -e PGUSER=postgres \
     -e PGPASSWORD=foobar \
     -e PGDATABASE=imagebuilder \
     -v /etc/osbuild-composer:/etc/osbuild-composer \
     image-builder


# Change to the working directory.
cd $WORKING_DIRECTORY

# Run each test case.
for TEST_CASE in "${TEST_CASES[@]}"; do
    run_test_case "${TESTS_PATH}/$TEST_CASE"
done

# Print a report of the test results.
test_divider
echo "😃 Passed tests: " "${PASSED_TESTS[@]}"
echo "☹ Failed tests: " "${FAILED_TESTS[@]}"
test_divider

# Exit with a failure if any tests failed.
if [ ${#FAILED_TESTS[@]} -eq 0 ]; then
    echo "🎉 All tests passed."
    exit 0
else
    echo "🔥 One or more tests failed."
    exit 1
fi
