#!/usr/bin/bash

#
# Test osbuild-composer's main API endpoint by building a sample image and
# uploading it to AWS.
#
# This script sets `-x` and is meant to always be run like that. This is
# simpler than adding extensive error reporting, which would make this script
# considerably more complex. Also, the full trace this produces is very useful
# for the primary audience: developers of osbuild-composer looking at the log
# from a run on a remote continuous integration system.
#

set -euxo pipefail

#
# Create a temporary directory and ensure it gets deleted when this script
# terminates in any way.
#

WORKDIR=$(mktemp -d)

#
# Make sure /openapi.json and /version endpoints return success
#

# org_id 000000 (valid org_id)
ValidAuthString="eyJlbnRpdGxlbWVudHMiOnsiaW5zaWdodHMiOnsiaXNfZW50aXRsZWQiOnRydWV9LCJzbWFydF9tYW5hZ2VtZW50Ijp7ImlzX2VudGl0bGVkIjp0cnVlfSwib3BlbnNoaWZ0Ijp7ImlzX2VudGl0bGVkIjp0cnVlfSwiaHlicmlkIjp7ImlzX2VudGl0bGVkIjp0cnVlfSwibWlncmF0aW9ucyI6eyJpc19lbnRpdGxlZCI6dHJ1ZX0sImFuc2libGUiOnsiaXNfZW50aXRsZWQiOnRydWV9fSwiaWRlbnRpdHkiOnsiYWNjb3VudF9udW1iZXIiOiIwMDAwMDAiLCJ0eXBlIjoiVXNlciIsInVzZXIiOnsidXNlcm5hbWUiOiJ1c2VyIiwiZW1haWwiOiJ1c2VyQHVzZXIudXNlciIsImZpcnN0X25hbWUiOiJ1c2VyIiwibGFzdF9uYW1lIjoidXNlciIsImlzX2FjdGl2ZSI6dHJ1ZSwiaXNfb3JnX2FkbWluIjp0cnVlLCJpc19pbnRlcm5hbCI6dHJ1ZSwibG9jYWxlIjoiZW4tVVMifSwiaW50ZXJuYWwiOnsib3JnX2lkIjoiMDAwMDAwIn19fQ=="

# org_id 000001 (invalid org_id)
InvalidAuthString="eyJlbnRpdGxlbWVudHMiOnsiaW5zaWdodHMiOnsiaXNfZW50aXRsZWQiOnRydWV9LCJzbWFydF9tYW5hZ2VtZW50Ijp7ImlzX2VudGl0bGVkIjp0cnVlfSwib3BlbnNoaWZ0Ijp7ImlzX2VudGl0bGVkIjp0cnVlfSwiaHlicmlkIjp7ImlzX2VudGl0bGVkIjp0cnVlfSwibWlncmF0aW9ucyI6eyJpc19lbnRpdGxlZCI6dHJ1ZX0sImFuc2libGUiOnsiaXNfZW50aXRsZWQiOnRydWV9fSwiaWRlbnRpdHkiOnsiYWNjb3VudF9udW1iZXIiOiIwMDAwMDAiLCJ0eXBlIjoiVXNlciIsInVzZXIiOnsidXNlcm5hbWUiOiJ1c2VyIiwiZW1haWwiOiJ1c2VyQHVzZXIudXNlciIsImZpcnN0X25hbWUiOiJ1c2VyIiwibGFzdF9uYW1lIjoidXNlciIsImlzX2FjdGl2ZSI6dHJ1ZSwiaXNfb3JnX2FkbWluIjp0cnVlLCJpc19pbnRlcm5hbCI6dHJ1ZSwibG9jYWxlIjoiZW4tVVMifSwiaW50ZXJuYWwiOnsib3JnX2lkIjoiMDAwMDAxIn19fQ=="

# Common constants
CurlCmd='curl -s -w %{http_code}'
Header="x-rh-identity: $ValidAuthString"
Address="localhost"
Port="8086"
Version="1.0"
MajorVersion="1"
BaseUrl="http://$Address:$Port/api/image-builder/v$Version"
BaseUrlMajorVersion="http://$Address:$Port/api/image-builder/v$MajorVersion"

REQUEST_FILE="${WORKDIR}/request.json"
ARCH=$(uname -m)

case $(set +x; . /etc/os-release; echo "$ID-$VERSION_ID") in
  "rhel-8.2" | "rhel-8.3" | "rhel-8.4")
    DISTRO="rhel-8"
  ;;
  "fedora-32")
    DISTRO="fedora-32"
  ;;
esac

function getResponse() {
  read -r -d '' -a arr <<<"$1"
  echo "${arr[@]::${#arr[@]}-1}"
}

function getExitCode() {
  read -r -d '' -a arr <<<"$1"
  echo "${arr[-1]}"
}

function createReqFileAWS() {
  cat > "$REQUEST_FILE" << EOF
{
  "distribution": "$DISTRO",
  "image_requests": [
    {
      "architecture": "$ARCH",
      "image_type": "ami",
      "upload_requests": [
        {
          "type": "aws",
          "options": {
            "share_with_accounts": ["${AWS_API_TEST_SHARE_ACCOUNT}"]
          }
        }
      ]
    }
  ],
  "customizations": {
    "packages": [
      "postgresql"
    ]
  }
}
EOF
}

### Case: get version
function Test_getVersion() {
  url="$1"
  result=$($CurlCmd -H "$Header" "$url/version")
  ver="$(getResponse "$result" | jq .version -r)"
  [[ $ver == "$Version" ]]
  exit_code=$(getExitCode "$result")
  [[ $exit_code == 200 ]]
}

### Case: get openapi.json
function Test_getOpenapi() {
  url="$1"
  result=$($CurlCmd -H "$Header" "$url/openapi.json")
  exit_code=$(getExitCode "$result")
  [[ $exit_code == 200 ]]
}


### Case: post to composer
function Test_postToComposer() {
  url="$1"
  result=$($CurlCmd -H "$Header" -H 'Content-Type: application/json' --request POST --data @"$REQUEST_FILE" "$url/compose")
  exit_code=$(getExitCode "$result")
  [[ $exit_code == 201 ]]
  COMPOSE_ID=$(getResponse "$result" | jq -r '.id')
  [[ "$COMPOSE_ID" =~ ^\{?[A-F0-9a-f]{8}-[A-F0-9a-f]{4}-[A-F0-9a-f]{4}-[A-F0-9a-f]{4}-[A-F0-9a-f]{12}\}?$ ]]
}

function Test_getOpenapiWithWrongOrgId() {
  result=$($CurlCmd -H "x-rh-identity: $InvalidAuthString" "$BaseUrl/openapi.json")
  exit_code=$(getExitCode "$result")
  [[ $exit_code == 404 ]]
  msg=$(getResponse "$result" | jq -r '.errors[0].detail')
  [[ $msg == "Organization not allowed" ]]
}

function Test_postToComposerWithWrongOrgId() {
  result=$($CurlCmd -H "x-rh-identity: $InvalidAuthString" -H 'Content-Type: application/json' --request POST --data @"$REQUEST_FILE" "$BaseUrl/compose")
  exit_code=$(getExitCode "$result")
  [[ $exit_code == 404 ]]
  msg=$(getResponse "$result" | jq -r '.errors[0].detail')
  [[ $msg == "Organization not allowed" ]]
}

function BasicValidation() {
  url="$1"

  Test_getVersion "$url"
  Test_getOpenapi "$url"
  Test_postToComposer "$url"
}

#
# Which cloud provider are we testing?
#

CLOUD_PROVIDER_AWS="aws"

CLOUD_PROVIDER=${1:-$CLOUD_PROVIDER_AWS}
case $CLOUD_PROVIDER in
  "$CLOUD_PROVIDER_AWS")
    AWS_API_TEST_SHARE_ACCOUNT=${AWS_API_TEST_SHARE_ACCOUNT-012345678912}
    createReqFileAWS
  ;;
  *)
  echo "Not supported platform: ${CLOUD_PROVIDER}"
  exit 1
  ;;
esac

############### Test begin ################
BasicValidation "${BaseUrl}"
BasicValidation "${BaseUrlMajorVersion}"
Test_getOpenapiWithWrongOrgId
Test_postToComposerWithWrongOrgId

echo "########## Test success! ##########"
exit 0
