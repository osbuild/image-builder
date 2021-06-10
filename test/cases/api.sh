#!/usr/bin/bash

#
# The image-builder API integration test
#
# This script sets `-x` and is meant to always be run like that. This is
# simpler than adding extensive error reporting, which would make this script
# considerably more complex. Also, the full trace this produces is very useful
# for the primary audience: developers of image-builder looking at the log
# from a run on a remote continuous integration system.
#

set -euxo pipefail

############### Cleanup functions ################

function cleanupAWS() {
  # since this function can be called at any time, ensure that we don't expand unbound variables
  AWS_CMD="${AWS_CMD:-}"
  AWS_INSTANCE_ID="${AWS_INSTANCE_ID:-}"
  AMI_IMAGE_ID="${AMI_IMAGE_ID:-}"
  AWS_SNAPSHOT_ID="${AWS_SNAPSHOT_ID:-}"

  if [ -n "$AWS_CMD" ]; then
    set +e
    $AWS_CMD ec2 terminate-instances --instance-ids "$AWS_INSTANCE_ID"
    $AWS_CMD ec2 deregister-image --image-id "$AMI_IMAGE_ID"
    $AWS_CMD ec2 delete-snapshot --snapshot-id "$AWS_SNAPSHOT_ID"
    $AWS_CMD ec2 delete-key-pair --key-name "key-for-$AMI_IMAGE_ID"
    set -e
  fi
}

function cleanupGCP() {
  # since this function can be called at any time, ensure that we don't expand unbound variables
  GCP_CMD="${GCP_CMD:-}"
  GCP_IMAGE_NAME="${GCP_IMAGE_NAME:-}"
  GCP_INSTANCE_NAME="${GCP_INSTANCE_NAME:-}"

  if [ -n "$GCP_CMD" ]; then
    set +e
    $GCP_CMD compute instances delete --zone="$GCP_REGION-a" "$GCP_INSTANCE_NAME"
    $GCP_CMD compute images delete "$GCP_IMAGE_NAME"
    set -e
  fi
}

function cleanupAzure() {
  # since this function can be called at any time, ensure that we don't expand unbound variables
  AZURE_CMD="${AZURE_CMD:-}"
  AZURE_IMAGE_NAME="${AZURE_IMAGE_NAME:-}"

  # do not run clean-up if the image name is not yet defined
  if [[ -n "$AZURE_CMD" && -n "$AZURE_IMAGE_NAME" ]]; then
    set +e
    $AZURE_CMD image delete --resource-group sharing-research --name "$AZURE_IMAGE_NAME"

    # find a storage account by its tag
    AZURE_STORAGE_ACCOUNT=$($AZURE_CMD resource list --tag imageBuilderStorageAccount=location="$AZURE_LOCATION" | jq -r .[0].name)
    $AZURE_CMD storage blob delete --container-name imagebuilder --name "$AZURE_IMAGE_NAME".vhd --account-name "$AZURE_STORAGE_ACCOUNT"
    set -e
  fi
}

# Create a temporary directory and ensure it gets deleted when this script
# terminates in any way.
WORKDIR=$(mktemp -d)
function cleanup() {
  case $CLOUD_PROVIDER in
    "$CLOUD_PROVIDER_AWS")
      cleanupAWS
      ;;
    "$CLOUD_PROVIDER_GCP")
      cleanupGCP
      ;;
    "$CLOUD_PROVIDER_AZURE")
      cleanupAzure
      ;;
  esac

  rm -rf "$WORKDIR"
}
trap cleanup EXIT

############### Common functions and variables ################

# org_id 000000 (valid org_id)
VALIDAUTHSTRING="eyJlbnRpdGxlbWVudHMiOnsiaW5zaWdodHMiOnsiaXNfZW50aXRsZWQiOnRydWV9LCJzbWFydF9tYW5hZ2VtZW50Ijp7ImlzX2VudGl0bGVkIjp0cnVlfSwib3BlbnNoaWZ0Ijp7ImlzX2VudGl0bGVkIjp0cnVlfSwiaHlicmlkIjp7ImlzX2VudGl0bGVkIjp0cnVlfSwibWlncmF0aW9ucyI6eyJpc19lbnRpdGxlZCI6dHJ1ZX0sImFuc2libGUiOnsiaXNfZW50aXRsZWQiOnRydWV9fSwiaWRlbnRpdHkiOnsiYWNjb3VudF9udW1iZXIiOiIwMDAwMDAiLCJ0eXBlIjoiVXNlciIsInVzZXIiOnsidXNlcm5hbWUiOiJ1c2VyIiwiZW1haWwiOiJ1c2VyQHVzZXIudXNlciIsImZpcnN0X25hbWUiOiJ1c2VyIiwibGFzdF9uYW1lIjoidXNlciIsImlzX2FjdGl2ZSI6dHJ1ZSwiaXNfb3JnX2FkbWluIjp0cnVlLCJpc19pbnRlcm5hbCI6dHJ1ZSwibG9jYWxlIjoiZW4tVVMifSwiaW50ZXJuYWwiOnsib3JnX2lkIjoiMDAwMDAwIn19fQ=="

# org_id 000001 (invalid org_id)
INVALIDAUTHSTRING="eyJlbnRpdGxlbWVudHMiOnsiaW5zaWdodHMiOnsiaXNfZW50aXRsZWQiOnRydWV9LCJzbWFydF9tYW5hZ2VtZW50Ijp7ImlzX2VudGl0bGVkIjp0cnVlfSwib3BlbnNoaWZ0Ijp7ImlzX2VudGl0bGVkIjp0cnVlfSwiaHlicmlkIjp7ImlzX2VudGl0bGVkIjp0cnVlfSwibWlncmF0aW9ucyI6eyJpc19lbnRpdGxlZCI6dHJ1ZX0sImFuc2libGUiOnsiaXNfZW50aXRsZWQiOnRydWV9fSwiaWRlbnRpdHkiOnsiYWNjb3VudF9udW1iZXIiOiIwMDAwMDAiLCJ0eXBlIjoiVXNlciIsInVzZXIiOnsidXNlcm5hbWUiOiJ1c2VyIiwiZW1haWwiOiJ1c2VyQHVzZXIudXNlciIsImZpcnN0X25hbWUiOiJ1c2VyIiwibGFzdF9uYW1lIjoidXNlciIsImlzX2FjdGl2ZSI6dHJ1ZSwiaXNfb3JnX2FkbWluIjp0cnVlLCJpc19pbnRlcm5hbCI6dHJ1ZSwibG9jYWxlIjoiZW4tVVMifSwiaW50ZXJuYWwiOnsib3JnX2lkIjoiMDAwMDAxIn19fQ=="

PORT="8086"
CURLCMD='curl -w %{http_code}'
HEADER="x-rh-identity: $VALIDAUTHSTRING"
ADDRESS="localhost"
BASEURL="http://$ADDRESS:$PORT/api/image-builder/v1.0"
BASEURLMAJORVERSION="http://$ADDRESS:$PORT/api/image-builder/v1"
REQUEST_FILE="${WORKDIR}/request.json"
ARCH=$(uname -m)

# Wait until service is ready
READY=0
for RETRY in {1..10};do
  curl --fail -H "$HEADER" "http://$ADDRESS:$PORT/ready" && {
    READY=1
    break
  }
  echo "Port $PORT is not open. Waiting...($RETRY/10)"
  sleep 1
done

[ "$READY" -eq 1 ] || {
  echo "Port $PORT is not open after retrying 10 times. Exit."
  exit 1
}

case $(set +x; . /etc/os-release; echo "$ID-$VERSION_ID") in
  "rhel-8.2" | "rhel-8.3" | "rhel-8.4")
    DISTRO="rhel-8"
    SSH_USER="cloud-user"
  ;;
esac

function getResponse() {
  read -r -d '' -a ARR <<<"$1"
  echo "${ARR[@]::${#ARR[@]}-1}"
}

function getExitCode() {
  read -r -d '' -a ARR <<<"$1"
  echo "${ARR[-1]}"
}

function instanceWaitSSH() {
  local HOST="$1"

  for LOOP_COUNTER in {0..30}; do
      if ssh-keyscan "$HOST" > /dev/null 2>&1; then
          echo "SSH is up!"
          break
      fi
      echo "Retrying in 5 seconds... $LOOP_COUNTER"
      sleep 5
  done
}

function instanceCheck() {
  echo "✔️ Instance checking"
  local _ssh="$1"

  # Check if postgres is installed
  $_ssh rpm -q postgresql

  # Verify subscribe status. Loop check since the system may not be registered such early
   for LOOP_COUNTER in {1..10}; do
    subscribe_org_id=$($_ssh sudo subscription-manager identity | grep 'org ID')
    if [[ "$subscribe_org_id" == "org ID: $API_TEST_SUBSCRIPTION_ORG_ID" ]]; then
      echo "System is subscribed."
      break
    else
      echo "System is not subscribed. Retrying in 10 seconds...($LOOP_COUNTER/10)"
      sleep 10
    fi
  done
  [[ "$subscribe_org_id" == "org ID: $API_TEST_SUBSCRIPTION_ORG_ID" ]]

  # Verify yum install a small package. It will fail if no available repo.
  $_ssh sudo dnf -y install dos2unix

  # Unregister subscription
  $_ssh sudo subscription-manager unregister
}

############### AWS-specific functions ################

function checkEnvAWS() {
  printenv AWS_REGION AWS_BUCKET AWS_ACCESS_KEY_ID AWS_SECRET_ACCESS_KEY AWS_API_TEST_SHARE_ACCOUNT > /dev/null
}

function installClientAWS() {
  if ! hash aws; then
    mkdir "$WORKDIR/aws"
    pushd "$WORKDIR/aws"
      curl -Ls --retry 5 --output awscliv2.zip \
        https://awscli.amazonaws.com/awscli-exe-linux-x86_64.zip
      unzip awscliv2.zip > /dev/null
      sudo ./aws/install > /dev/null
      aws --version
    popd
  fi

  AWS_CMD="aws --region $AWS_REGION --output json --color on"
}

function createReqFileAWS() {
  cat > "$REQUEST_FILE" << EOF
{
  "distribution": "$DISTRO",
  "image_requests": [
    {
      "architecture": "$ARCH",
      "image_type": "ami",
      "upload_request": {
        "type": "aws",
        "options": {
          "share_with_accounts": ["${AWS_API_TEST_SHARE_ACCOUNT}"]
        }
      }
    }
  ],
  "customizations": {
    "packages": [
      "postgresql"
    ],
    "subscription": {
      "organization": ${API_TEST_SUBSCRIPTION_ORG_ID:-},
      "activation-key": "${API_TEST_SUBSCRIPTION_ACTIVATION_KEY:-}",
      "base-url": "https://cdn.redhat.com/",
      "server-url": "subscription.rhsm.redhat.com",
      "insights": true
    }
  }
}
EOF
}

############### GCP-specific functions ################

function checkEnvGCP() {
  printenv GOOGLE_APPLICATION_CREDENTIALS GCP_BUCKET GCP_REGION GCP_API_TEST_SHARE_ACCOUNT > /dev/null
}

function installClientGCP() {
  if ! hash gcloud; then
    sudo tee -a /etc/yum.repos.d/google-cloud-sdk.repo << EOM
[google-cloud-sdk]
name=Google Cloud SDK
baseurl=https://packages.cloud.google.com/yum/repos/cloud-sdk-el8-x86_64
enabled=1
gpgcheck=1
repo_gpgcheck=1
gpgkey=https://packages.cloud.google.com/yum/doc/yum-key.gpg
       https://packages.cloud.google.com/yum/doc/rpm-package-key.gpg
EOM
  fi

  sudo dnf -y install google-cloud-sdk
  GCP_CMD="gcloud --format=json --quiet"
  $GCP_CMD --version
}

function createReqFileGCP() {
  cat > "$REQUEST_FILE" << EOF
{
  "distribution": "$DISTRO",
  "image_requests": [
    {
      "architecture": "$ARCH",
      "image_type": "vhd",
      "upload_request": {
        "type": "gcp",
        "options": {
          "share_with_accounts": ["${GCP_API_TEST_SHARE_ACCOUNT}"]
        }
      }
    }
  ],
  "customizations": {
    "packages": [
      "postgresql"
    ],
    "subscription": {
      "organization": ${API_TEST_SUBSCRIPTION_ORG_ID:-},
      "activation-key": "${API_TEST_SUBSCRIPTION_ACTIVATION_KEY:-}",
      "base-url": "https://cdn.redhat.com/",
      "server-url": "subscription.rhsm.redhat.com",
      "insights": true
    }
  }
}
EOF
}

############### Azure-specific functions ################

function checkEnvAzure() {
  printenv AZURE_TENANT_ID AZURE_SUBSCRIPTION_ID AZURE_RESOURCE_GROUP AZURE_LOCATION AZURE_CLIENT_ID AZURE_CLIENT_SECRET > /dev/null
}

function installClientAzure() {
  if ! hash az; then
    # this installation method is taken from the official docs:
    # https://docs.microsoft.com/cs-cz/cli/azure/install-azure-cli-linux?pivots=dnf
    sudo rpm --import https://packages.microsoft.com/keys/microsoft.asc
    echo -e "[azure-cli]
name=Azure CLI
baseurl=https://packages.microsoft.com/yumrepos/azure-cli
enabled=1
gpgcheck=1
gpgkey=https://packages.microsoft.com/keys/microsoft.asc" | sudo tee /etc/yum.repos.d/azure-cli.repo
  fi

  sudo dnf install -y azure-cli
  AZURE_CMD="az"
  $AZURE_CMD version
}

function createReqFileAzure() {
  cat > "$REQUEST_FILE" << EOF
{
  "distribution": "$DISTRO",
  "image_requests": [
    {
      "architecture": "$ARCH",
      "image_type": "vhd",
      "upload_request": {
        "type": "azure",
        "options": {
          "tenant_id": "${AZURE_TENANT_ID}",
          "subscription_id": "${AZURE_SUBSCRIPTION_ID}",
          "resource_group": "${AZURE_RESOURCE_GROUP}"
        }
      }
    }
  ],
  "customizations": {
    "packages": [
      "postgresql"
    ],
    "subscription": {
      "organization": ${API_TEST_SUBSCRIPTION_ORG_ID:-},
      "activation-key": "${API_TEST_SUBSCRIPTION_ACTIVATION_KEY:-}",
      "base-url": "https://cdn.redhat.com/",
      "server-url": "subscription.rhsm.redhat.com",
      "insights": true
    }
  }
}
EOF
}

############### Test cases definitions ################

### Case: get version
function Test_getVersion() {
  URL="$1"
  RESULT=$($CURLCMD -H "$HEADER" "$URL/version")
  V=$(getResponse "$RESULT" | jq -r '.version')
  [[ "$V" == "1.0" ]]
  EXIT_CODE=$(getExitCode "$RESULT")
  [[ "$EXIT_CODE" == 200 ]]
}

### Case: get openapi.json
function Test_getOpenapi() {
  URL="$1"
  RESULT=$($CURLCMD -H "$HEADER" "$URL/openapi.json")
  EXIT_CODE=$(getExitCode "$RESULT")
  [[ "$EXIT_CODE" == 200 ]]
}

### Case: post to composer
function Test_postToComposer() {
  RESULT=$($CURLCMD -H "$HEADER" -H 'Content-Type: application/json' --request POST --data @"$REQUEST_FILE" "$BASEURL/compose")
  EXIT_CODE=$(getExitCode "$RESULT")
  [[ "$EXIT_CODE" == 201 ]]
  COMPOSE_ID=$(getResponse "$RESULT" | jq -r '.id')
  [[ "$COMPOSE_ID" =~ ^\{?[A-F0-9a-f]{8}-[A-F0-9a-f]{4}-[A-F0-9a-f]{4}-[A-F0-9a-f]{4}-[A-F0-9a-f]{12}\}?$ ]]
}

### Case: wait for the compose to finish successfully
function Test_waitForCompose() {
  while true
  do
    RESULT=$($CURLCMD -H "$HEADER" --request GET "$BASEURL/composes/$COMPOSE_ID")
    EXIT_CODE=$(getExitCode "$RESULT")
    [[ $EXIT_CODE == 200 ]]

    COMPOSE_STATUS=$(getResponse "$RESULT" | jq -r '.image_status.status')
    UPLOAD_STATUS=$(getResponse "$RESULT" | jq -r '.image_status.upload_status.status')

    case "$COMPOSE_STATUS" in
      # "running is kept here temporarily for backward compatibility"
      "running")
        ;;
      # valid status values for compose which is not yet finished
      "pending"|"building"|"uploading"|"registering")
        ;;
      "success")
        [[ "$UPLOAD_STATUS" = "success" ]]
        break
        ;;
      "failure")
        echo "Image compose failed"
        exit 1
        ;;
      *)
        echo "API returned unexpected image_status.status value: '$COMPOSE_STATUS'"
        exit 1
        ;;
    esac

    sleep 30
  done
}

### Case: verify the result (image) of a finished compose in AWS
function Test_verifyComposeResultAWS() {
  UPLOAD_OPTIONS="$1"

  AMI_IMAGE_ID=$(echo "$UPLOAD_OPTIONS" | jq -r '.ami')
  # AWS ID consist of resource identifier followed by a 17-character string
  [[ "$AMI_IMAGE_ID" =~ ami-[[:alnum:]]{17} ]]

  local REGION
  REGION=$(echo "$UPLOAD_OPTIONS" | jq -r '.region')
  [[ "$REGION" = "$AWS_REGION" ]]

  # Try to boot the result image with the cloud provider
  $AWS_CMD ec2 describe-images --image-ids "$AMI_IMAGE_ID" > "$WORKDIR/ami.json"

  AWS_SNAPSHOT_ID=$(jq -r '.Images[].BlockDeviceMappings[].Ebs.SnapshotId' "$WORKDIR/ami.json")
  SHARE_OK=1

  # Verify that the ec2 snapshot was shared
  $AWS_CMD ec2 describe-snapshot-attribute --snapshot-id "$AWS_SNAPSHOT_ID" --attribute createVolumePermission > "$WORKDIR/snapshot-attributes.json"

  SHARED_ID=$(jq -r '.CreateVolumePermissions[0].UserId' "$WORKDIR/snapshot-attributes.json")
  if [ "$AWS_API_TEST_SHARE_ACCOUNT" != "$SHARED_ID" ]; then
    SHARE_OK=0
  fi

  # Verify that the ec2 ami was shared
  $AWS_CMD ec2 describe-image-attribute --image-id "$AMI_IMAGE_ID" --attribute launchPermission > "$WORKDIR/ami-attributes.json"

  SHARED_ID=$(jq -r '.LaunchPermissions[0].UserId' "$WORKDIR/ami-attributes.json")
  if [ "$AWS_API_TEST_SHARE_ACCOUNT" != "$SHARED_ID" ]; then
    SHARE_OK=0
  fi

  if [ "$SHARE_OK" != 1 ]; then
    echo "EC2 snapshot wasn't shared with the AWS_API_TEST_SHARE_ACCOUNT. 😢"
    exit 1
  fi

  # Create key-pair
  $AWS_CMD ec2 create-key-pair --key-name "key-for-$AMI_IMAGE_ID" --query 'KeyMaterial' --output text > keypair.pem
  chmod 400 ./keypair.pem

  # Create an instance based on the ami
  $AWS_CMD ec2 run-instances --image-id "$AMI_IMAGE_ID" --count 1 --instance-type t2.micro --key-name "key-for-$AMI_IMAGE_ID" > "$WORKDIR/instances.json"
  AWS_INSTANCE_ID=$(jq -r '.Instances[].InstanceId' "$WORKDIR/instances.json")

  $AWS_CMD ec2 wait instance-running --instance-ids "$AWS_INSTANCE_ID"

  $AWS_CMD ec2 describe-instances --instance-ids "$AWS_INSTANCE_ID" > "$WORKDIR/instances.json"
  HOST=$(jq -r '.Reservations[].Instances[].PublicIpAddress' "$WORKDIR/instances.json")

  echo "⏱ Waiting for AWS instance to respond to ssh"
  instanceWaitSSH "$HOST"

  # Verify image
  _ssh="ssh -oStrictHostKeyChecking=no -i ./keypair.pem $SSH_USER@$HOST"
  instanceCheck "$_ssh"
}

### Case: verify the result (image) of a finished compose in GCP
function Test_verifyComposeResultGCP() {
  UPLOAD_OPTIONS="$1"

  GCP_PROJECT=$(jq -r '.project_id' "$GOOGLE_APPLICATION_CREDENTIALS")

  GCP_IMAGE_NAME=$(echo "$UPLOAD_OPTIONS" | jq -r '.image_name')
  [[ -n "$GCP_IMAGE_NAME" ]]

  local PROJECT_ID
  PROJECT_ID=$(echo "$UPLOAD_OPTIONS" | jq -r '.project_id')
  [[ "$PROJECT_ID" = "$GCP_PROJECT" ]]

  # Authenticate
  $GCP_CMD auth activate-service-account --key-file "$GOOGLE_APPLICATION_CREDENTIALS"
  # Set the default project to be used for commands
  $GCP_CMD config set project "$GCP_PROJECT"

  # Verify that the image was shared
  SHARE_OK=1
  $GCP_CMD compute images get-iam-policy "$GCP_IMAGE_NAME" > "$WORKDIR/image-iam-policy.json"
  SHARED_ACCOUNT=$(jq -r '.bindings[0].members[0]' "$WORKDIR/image-iam-policy.json")
  SHARED_ROLE=$(jq -r '.bindings[0].role' "$WORKDIR/image-iam-policy.json")
  if [ "$SHARED_ACCOUNT" != "$GCP_API_TEST_SHARE_ACCOUNT" ] || [ "$SHARED_ROLE" != "roles/compute.imageUser" ]; then
    SHARE_OK=0
  fi

  if [ "$SHARE_OK" != 1 ]; then
    echo "GCP image wasn't shared with the GCP_API_TEST_SHARE_ACCOUNT. 😢"
    exit 1
  fi

  # Verify that the image boots and have customizations applied
  # Create SSH keys to use
  GCP_SSH_KEY="$WORKDIR/id_google_compute_engine"
  ssh-keygen -t rsa -f "$GCP_SSH_KEY" -C "$SSH_USER" -N ""
  GCP_SSH_METADATA_FILE="$WORKDIR/gcp-ssh-keys-metadata"

  echo "${SSH_USER}:$(cat "$GCP_SSH_KEY".pub)" > "$GCP_SSH_METADATA_FILE"

  # create the instance
  # resource ID can have max 62 characters, the $GCP_TEST_ID_HASH contains 56 characters
  GCP_INSTANCE_NAME="vm-$(uuidgen)"

  $GCP_CMD compute instances create "$GCP_INSTANCE_NAME" \
    --zone="$GCP_REGION-a" \
    --image-project="$GCP_PROJECT" \
    --image="$GCP_IMAGE_NAME" \
    --metadata-from-file=ssh-keys="$GCP_SSH_METADATA_FILE"
  HOST=$($GCP_CMD compute instances describe "$GCP_INSTANCE_NAME" --zone="$GCP_REGION-a" --format='get(networkInterfaces[0].accessConfigs[0].natIP)')

  echo "⏱ Waiting for GCP instance to respond to ssh"
  instanceWaitSSH "$HOST"

  # Verify image
  _ssh="ssh -oStrictHostKeyChecking=no -i $GCP_SSH_KEY $SSH_USER@$HOST"
  instanceCheck "$_ssh"
}

### Case: verify the result (image) of a finished compose in Azure
function Test_verifyComposeResultAzure() {
  UPLOAD_OPTIONS="$1"

  AZURE_IMAGE_NAME=$(echo "$UPLOAD_OPTIONS" | jq -r '.image_name')
  [[ -n "$AZURE_IMAGE_NAME" ]]

  set +x
  $AZURE_CMD login --service-principal --username "${AZURE_CLIENT_ID}" --password "${AZURE_CLIENT_SECRET}" --tenant "${AZURE_TENANT_ID}"
  set -x

  # verify that the image exists
  $AZURE_CMD image show --resource-group "${AZURE_RESOURCE_GROUP}" --name "${AZURE_IMAGE_NAME}"

  # Boot testing is currently blocked due to
  # https://github.com/Azure/azure-cli/issues/17123
  # Without this issue fixed or worked around, I'm not able to delete the disk
  # attached to the VM.
}

### Case: verify the result (image) of a finished compose
function Test_verifyComposeResult() {
  RESULT=$($CURLCMD -H "$HEADER" --request GET "$BASEURL/composes/$COMPOSE_ID")
  EXIT_CODE=$(getExitCode "$RESULT")
  [[ $EXIT_CODE == 200 ]]

  UPLOAD_TYPE=$(getResponse "$RESULT" | jq -r '.image_status.upload_status.type')
  [[ "$UPLOAD_TYPE" = "$CLOUD_PROVIDER" ]]

  UPLOAD_OPTIONS=$(getResponse "$RESULT" | jq -r '.image_status.upload_status.options')

  # verify upload options specific to cloud provider
  case $CLOUD_PROVIDER in
    "$CLOUD_PROVIDER_AWS")
      Test_verifyComposeResultAWS "$UPLOAD_OPTIONS"
      ;;
    "$CLOUD_PROVIDER_GCP")
      Test_verifyComposeResultGCP "$UPLOAD_OPTIONS"
      ;;
    "$CLOUD_PROVIDER_AZURE")
      Test_verifyComposeResultAzure "$UPLOAD_OPTIONS"
      ;;
  esac
}

function Test_getComposes() {
  RESULT=$($CURLCMD -H "$HEADER" -H 'Content-Type: application/json' "$BASEURL/composes")
  EXIT_CODE=$(getExitCode "$RESULT")
  [[ "$EXIT_CODE" == 200 ]]
  RESPONSE=$(getResponse "$RESULT" | jq -r '.data[0]')
  [[ $(echo "$RESPONSE" | jq -r '.id') == "$COMPOSE_ID" ]]
  diff <(echo "$RESPONSE" | jq -Sr '.request') <(jq -Sr '.' "$REQUEST_FILE")
}

function Test_getOpenapiWithWrongOrgId() {
  RESULT=$($CURLCMD -H "x-rh-identity: $INVALIDAUTHSTRING" "$BASEURL/openapi.json")
  EXIT_CODE=$(getExitCode "$RESULT")
  [[ "$EXIT_CODE" == 404 ]]
  MSG=$(getResponse "$RESULT" | jq -r '.errors[0].detail')
  [[ "$MSG" == "Organization or account not allowed" ]]
}

function Test_postToComposerWithWrongOrgId() {
  RESULT=$($CURLCMD -H "x-rh-identity: $INVALIDAUTHSTRING" -H 'Content-Type: application/json' --request POST --data @"$REQUEST_FILE" "$BASEURL/compose")
  EXIT_CODE=$(getExitCode "$RESULT")
  [[ "$EXIT_CODE" == 404 ]]
  MSG=$(getResponse "$RESULT" | jq -r '.errors[0].detail')
  [[ $MSG == "Organization or account not allowed" ]]
}

#
# Which cloud provider are we testing?
#

CLOUD_PROVIDER_AWS="aws"
CLOUD_PROVIDER_GCP="gcp"
CLOUD_PROVIDER_AZURE="azure"

CLOUD_PROVIDER=${1:-$CLOUD_PROVIDER_AWS}
case $CLOUD_PROVIDER in
  "$CLOUD_PROVIDER_AWS")
    checkEnvAWS
    installClientAWS
    createReqFileAWS
    ;;
  "$CLOUD_PROVIDER_GCP")
    checkEnvGCP
    installClientGCP
    createReqFileGCP
    ;;
  "$CLOUD_PROVIDER_AZURE")
    checkEnvAzure
    installClientAzure
    createReqFileAzure
    ;;
  *)
    echo "Not supported platform: ${CLOUD_PROVIDER}"
    exit 1
    ;;
esac

############### Test begin ################
Test_getVersion "$BASEURL"
Test_getVersion "$BASEURLMAJORVERSION"
Test_getOpenapi "$BASEURL"
Test_getOpenapi "$BASEURLMAJORVERSION"
Test_postToComposer
Test_waitForCompose
Test_verifyComposeResult
Test_getComposes
Test_getOpenapiWithWrongOrgId
Test_postToComposerWithWrongOrgId

echo "########## Test success! ##########"
exit 0
