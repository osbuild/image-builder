#!/bin/bash

set -eux

# This function is to fix shell check error in line 29
# SC2211: This is a glob used as a command name. Was it supposed to be in ${..}, array, or is it missing quoting?
# If anyone know how to fix it in a better way, please update it.
run() {
  if (( $# != 1 ))
  then
    echo "Expected exactly 1 match but found $#: $*" >&2
    exit 1
  elif command -v "$1" > /dev/null 2>&1
  then
    "$1"
  else
    echo "Glob is not a valid command: $*" >&2
    exit 1
  fi
}

COVERITY_SCAN_PROJECT_NAME=$1
COVERITY_SCAN_TOKEN=$2

echo "COVERITY_SCAN_PROJECT_NAME=${COVERITY_SCAN_PROJECT_NAME}"
echo "COVERITY_SCAN_TOKEN=${COVERITY_SCAN_TOKEN}"

# Following code will do:
# 1. Download coverity scan tool
# 2. Run coverity scan
# 3. Upload scan result for analysis
# Then we can check defect details at https://scan.coverity.com
echo "Downloading coverity scan package."
curl -o /tmp/cov-analysis-linux64.tgz https://scan.coverity.com/download/linux64 \
        --form project="$COVERITY_SCAN_PROJECT_NAME" \
        --form token="$COVERITY_SCAN_TOKEN"
tar xfz /tmp/cov-analysis-linux64.tgz
echo "Running coverity scan during building."
mkdir bin
run cov-analysis-linux64-*/bin/cov-build --dir cov-int go build -o bin/ ./...
tar cfz cov-int.tar.gz cov-int
echo "Uploading coverity scan result to http://scan.coverity.com"
curl https://scan.coverity.com/builds?project="$COVERITY_SCAN_PROJECT_NAME" \
        --form token="$COVERITY_SCAN_TOKEN" \
        --form email="$GITLAB_USER_EMAIL" \
        --form file=@cov-int.tar.gz \
        --form version="$(git describe --tags)" \
        --form description="$(git describe --tags) / $CI_COMMIT_TITLE / $CI_COMMIT_REF_NAME:$CI_PIPELINE_ID "
