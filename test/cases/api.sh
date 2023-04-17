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

# Container image used for cloud provider CLI tools
CONTAINER_IMAGE_CLOUD_TOOLS="quay.io/osbuild/cloud-tools:latest"

if which podman 2>/dev/null >&2; then
  CONTAINER_RUNTIME=podman
elif which docker 2>/dev/null >&2; then
  CONTAINER_RUNTIME=docker
else
  echo No container runtime found, install podman or docker.
  exit 2
fi

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
    $GCP_CMD compute instances delete --zone="$GCP_ZONE" "$GCP_INSTANCE_NAME"
    set -e
  fi
}

function cleanupAzure() {
  # since this function can be called at any time, ensure that we don't expand unbound variables
  AZURE_CMD="${AZURE_CMD:-}"
  AZURE_IMAGE_NAME="${AZURE_IMAGE_NAME:-}"
  AZURE_INSTANCE_NAME="${AZURE_INSTANCE_NAME:-}"

  set +e
  # do not run clean-up if the image name is not yet defined
  if [[ -n "$AZURE_CMD" && -n "$AZURE_IMAGE_NAME" ]]; then
    # Re-get the vm_details in case the VM creation is failed.
    [ -f "$WORKDIR/vm_details.json" ] || $AZURE_CMD vm show --name "$AZURE_INSTANCE_NAME" --resource-group "$AZURE_RESOURCE_GROUP" --show-details > "$WORKDIR/vm_details.json"
    # Get all the resources ids
    VM_ID=$(jq -r '.id' "$WORKDIR"/vm_details.json)
    OSDISK_ID=$(jq -r '.storageProfile.osDisk.managedDisk.id' "$WORKDIR"/vm_details.json)
    NIC_ID=$(jq -r '.networkProfile.networkInterfaces[0].id' "$WORKDIR"/vm_details.json)
    $AZURE_CMD network nic show --ids "$NIC_ID" > "$WORKDIR"/nic_details.json
    NSG_ID=$(jq -r '.networkSecurityGroup.id' "$WORKDIR"/nic_details.json)
    PUBLICIP_ID=$(jq -r '.ipConfigurations[0].publicIpAddress.id' "$WORKDIR"/nic_details.json)

    # Delete resources. Some resources must be removed in order:
    # - Delete VM prior to any other resources
    # - Delete NIC prior to NSG, public-ip
    # Left Virtual Network and Storage Account there because other tests in the same resource group will reuse them
    for id in "$VM_ID" "$OSDISK_ID" "$NIC_ID" "$NSG_ID" "$PUBLICIP_ID"; do
      echo "Deleting $id..."
      $AZURE_CMD resource delete --ids "$id"
    done

    # Delete image after VM deleting.
    $AZURE_CMD image delete --resource-group "$AZURE_RESOURCE_GROUP" --name "$AZURE_IMAGE_NAME"
    # find a storage account by its tag
    AZURE_STORAGE_ACCOUNT=$($AZURE_CMD resource list --tag imageBuilderStorageAccount=location="$AZURE_LOCATION" | jq -r .[0].name)
    AZURE_CONNECTION_STRING=$($AZURE_CMD storage account show-connection-string --name "$AZURE_STORAGE_ACCOUNT" | jq -r .connectionString)
    $AZURE_CMD storage blob delete --container-name imagebuilder --name "$AZURE_IMAGE_NAME".vhd --account-name "$AZURE_STORAGE_ACCOUNT" --connection-string "$AZURE_CONNECTION_STRING"
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

  sudo rm -rf "$WORKDIR"
}
trap cleanup EXIT

############### Common functions and variables ################

ACCOUNT0_ORG0="eyJlbnRpdGxlbWVudHMiOnsicmhlbCI6eyJpc19lbnRpdGxlZCI6dHJ1ZX0sImluc2lnaHRzIjp7ImlzX2VudGl0bGVkIjp0cnVlfSwic21hcnRfbWFuYWdlbWVudCI6eyJpc19lbnRpdGxlZCI6dHJ1ZX0sIm9wZW5zaGlmdCI6eyJpc19lbnRpdGxlZCI6dHJ1ZX0sImh5YnJpZCI6eyJpc19lbnRpdGxlZCI6dHJ1ZX0sIm1pZ3JhdGlvbnMiOnsiaXNfZW50aXRsZWQiOnRydWV9LCJhbnNpYmxlIjp7ImlzX2VudGl0bGVkIjp0cnVlfX0sImlkZW50aXR5Ijp7ImFjY291bnRfbnVtYmVyIjoiMDAwMDAwIiwidHlwZSI6IlVzZXIiLCJ1c2VyIjp7InVzZXJuYW1lIjoidXNlciIsImVtYWlsIjoidXNlckB1c2VyLnVzZXIiLCJmaXJzdF9uYW1lIjoidXNlciIsImxhc3RfbmFtZSI6InVzZXIiLCJpc19hY3RpdmUiOnRydWUsImlzX29yZ19hZG1pbiI6dHJ1ZSwiaXNfaW50ZXJuYWwiOnRydWUsImxvY2FsZSI6ImVuLVVTIn0sImludGVybmFsIjp7Im9yZ19pZCI6IjAwMDAwMCJ9fX0K"
ACCOUNT1_ORG1="eyJlbnRpdGxlbWVudHMiOnsicmhlbCI6eyJpc19lbnRpdGxlZCI6dHJ1ZX0sImluc2lnaHRzIjp7ImlzX2VudGl0bGVkIjp0cnVlfSwic21hcnRfbWFuYWdlbWVudCI6eyJpc19lbnRpdGxlZCI6dHJ1ZX0sIm9wZW5zaGlmdCI6eyJpc19lbnRpdGxlZCI6dHJ1ZX0sImh5YnJpZCI6eyJpc19lbnRpdGxlZCI6dHJ1ZX0sIm1pZ3JhdGlvbnMiOnsiaXNfZW50aXRsZWQiOnRydWV9LCJhbnNpYmxlIjp7ImlzX2VudGl0bGVkIjp0cnVlfX0sImlkZW50aXR5Ijp7ImFjY291bnRfbnVtYmVyIjoiMDAwMDAxIiwidHlwZSI6IlVzZXIiLCJ1c2VyIjp7InVzZXJuYW1lIjoidXNlciIsImVtYWlsIjoidXNlckB1c2VyLnVzZXIiLCJmaXJzdF9uYW1lIjoidXNlciIsImxhc3RfbmFtZSI6InVzZXIiLCJpc19hY3RpdmUiOnRydWUsImlzX29yZ19hZG1pbiI6dHJ1ZSwiaXNfaW50ZXJuYWwiOnRydWUsImxvY2FsZSI6ImVuLVVTIn0sImludGVybmFsIjp7Im9yZ19pZCI6IjAwMDAwMSJ9fX0="

CLOUD_PROVIDER_AWS="aws"
CLOUD_PROVIDER_GCP="gcp"
CLOUD_PROVIDER_AZURE="azure"
CLOUD_PROVIDER=${1:-$CLOUD_PROVIDER_AWS}

PORT="8086"
CURLCMD='curl -w %{http_code}'
HEADER="x-rh-identity: $ACCOUNT0_ORG0"
HEADER2="x-rh-identity: $ACCOUNT1_ORG1"
ADDRESS="localhost"
BASEURL="http://$ADDRESS:$PORT/api/image-builder/v1.0"
BASEURLMAJORVERSION="http://$ADDRESS:$PORT/api/image-builder/v1"
REQUEST_FILE="${WORKDIR}/request.json"
ARCH=$(uname -m)

DISTRO="rhel-8"
SSH_USER="cloud-user"

if [[ "$ARCH" == "x86_64" ]]; then
    INSTANCE_TYPE="t2.micro"
elif [[ "$ARCH" == "aarch64" ]]; then
    INSTANCE_TYPE="t4g.small"
else
  echo "Architecture not supported: $ARCH"
  exit 1
fi


if [[ "$CLOUD_PROVIDER" == "$CLOUD_PROVIDER_AWS" ]]; then
    SSH_USER="ec2-user"
fi

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
  $_ssh rpm -q postgresql ansible-core

  # Verify subscribe status. Loop check since the system may not be registered such early
  set +eu
  for LOOP_COUNTER in {1..10}; do
      subscribe_org_id=$($_ssh sudo subscription-manager identity | grep 'org ID')
      if [[ "$subscribe_org_id" == "org ID: $API_TEST_SUBSCRIPTION_ORG_ID" ]]; then
          echo "System is subscribed."
          break
      else
          echo "System is not subscribed. Retrying in 30 seconds...($LOOP_COUNTER/10)"
          sleep 30
      fi
  done
  set -eu
  [[ "$subscribe_org_id" == "org ID: $API_TEST_SUBSCRIPTION_ORG_ID" ]]

  # Verify yum install a small package. It will fail if no available repo.
  $_ssh sudo dnf -y install dos2unix

  # Unregister subscription
  $_ssh sudo subscription-manager unregister
}

############### AWS-specific functions ################

function checkEnvAWS() {
  printenv AWS_REGION AWS_BUCKET V2_AWS_ACCESS_KEY_ID V2_AWS_SECRET_ACCESS_KEY AWS_API_TEST_SHARE_ACCOUNT > /dev/null
}

function installClientAWS() {
  if ! hash aws; then
    echo "Using 'awscli' from a container"
    sudo ${CONTAINER_RUNTIME} pull ${CONTAINER_IMAGE_CLOUD_TOOLS}

    AWS_CMD="sudo ${CONTAINER_RUNTIME} run --rm \
      -e AWS_ACCESS_KEY_ID=${V2_AWS_ACCESS_KEY_ID} \
      -e AWS_SECRET_ACCESS_KEY=${V2_AWS_SECRET_ACCESS_KEY} \
      -v ${WORKDIR}:${WORKDIR}:Z \
      ${CONTAINER_IMAGE_CLOUD_TOOLS} aws --region $AWS_REGION --output json --color on"
  else
    echo "Using pre-installed 'aws' from the system"
    AWS_CMD="env AWS_ACCESS_KEY_ID=$V2_AWS_ACCESS_KEY_ID AWS_SECRET_ACCESS_KEY=$V2_AWS_SECRET_ACCESS_KEY aws --region $AWS_REGION --output json --color on"
  fi
  $AWS_CMD --version
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
      "postgresql",
      "ansible-core"
    ],
    "subscription": {
      "organization": ${API_TEST_SUBSCRIPTION_ORG_ID:-},
      "activation-key": "${API_TEST_SUBSCRIPTION_ACTIVATION_KEY_V2:-}",
      "base-url": "https://cdn.redhat.com/",
      "server-url": "subscription.rhsm.redhat.com",
      "insights": true,
      "rhc": true
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
    echo "Using 'gcloud' from a container"
    sudo ${CONTAINER_RUNTIME} pull ${CONTAINER_IMAGE_CLOUD_TOOLS}

    # directory mounted to the container, in which gcloud stores the credentials after logging in
    GCP_CMD_CREDS_DIR="${WORKDIR}/gcloud_credentials"
    mkdir "${GCP_CMD_CREDS_DIR}"

    GCP_CMD="sudo ${CONTAINER_RUNTIME} run --rm \
      -v ${GCP_CMD_CREDS_DIR}:/root/.config/gcloud:Z \
      -v ${GOOGLE_APPLICATION_CREDENTIALS}:${GOOGLE_APPLICATION_CREDENTIALS}:Z \
      -v ${WORKDIR}:${WORKDIR}:Z \
      ${CONTAINER_IMAGE_CLOUD_TOOLS} gcloud --format=json"
  else
    echo "Using pre-installed 'gcloud' from the system"
    GCP_CMD="gcloud --format=json --quiet"
  fi
  $GCP_CMD --version
}

function createReqFileGCP() {
  cat > "$REQUEST_FILE" << EOF
{
  "distribution": "$DISTRO",
  "image_requests": [
    {
      "architecture": "$ARCH",
      "image_type": "gcp",
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
      "postgresql",
      "ansible-core"
    ],
    "subscription": {
      "organization": ${API_TEST_SUBSCRIPTION_ORG_ID:-},
      "activation-key": "${API_TEST_SUBSCRIPTION_ACTIVATION_KEY_V2:-}",
      "base-url": "https://cdn.redhat.com/",
      "server-url": "subscription.rhsm.redhat.com",
      "insights": true,
      "rhc": false
    }
  }
}
EOF
}

############### Azure-specific functions ################

function checkEnvAzure() {
  printenv AZURE_TENANT_ID AZURE_SUBSCRIPTION_ID AZURE_RESOURCE_GROUP AZURE_LOCATION V2_AZURE_CLIENT_ID V2_AZURE_CLIENT_SECRET > /dev/null
}

function installClientAzure() {
  if ! hash az; then
    echo "Using 'azure-cli' from a container"
    sudo ${CONTAINER_RUNTIME} pull ${CONTAINER_IMAGE_CLOUD_TOOLS}

    # directory mounted to the container, in which azure-cli stores the credentials after logging in
    AZURE_CMD_CREDS_DIR="${WORKDIR}/azure-cli_credentials"
    mkdir "${AZURE_CMD_CREDS_DIR}"

    AZURE_CMD="sudo ${CONTAINER_RUNTIME} run --rm \
      -v ${AZURE_CMD_CREDS_DIR}:/root/.azure:Z \
      -v ${WORKDIR}:${WORKDIR}:Z \
      ${CONTAINER_IMAGE_CLOUD_TOOLS} az"
  else
    echo "Using pre-installed 'azure-cli' from the system"
    AZURE_CMD="az"
  fi
  $AZURE_CMD version
}

source /etc/os-release

CI="${CI:-false}"
if [[ "$CI" == true ]]; then
  DISTRO_CODE="${DISTRO_CODE:-${ID}-${VERSION_ID//./}}"
  TEST_ID="$DISTRO_CODE-$ARCH-$CI_COMMIT_BRANCH-$CI_BUILD_ID"
else
  # if not running in Jenkins, generate ID not relying on specific env variables
  TEST_ID=$(uuidgen);
fi

function createReqFileAzure() {
  AZURE_IMAGE_NAME="image-$TEST_ID"

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
          "resource_group": "${AZURE_RESOURCE_GROUP}",
	  "image-name": "${AZURE_IMAGE_NAME}"
        }
      }
    }
  ],
  "customizations": {
    "packages": [
      "postgresql",
      "ansible-core"
    ],
    "subscription": {
      "organization": ${API_TEST_SUBSCRIPTION_ORG_ID:-},
      "activation-key": "${API_TEST_SUBSCRIPTION_ACTIVATION_KEY_V2:-}",
      "base-url": "https://cdn.redhat.com/",
      "server-url": "subscription.rhsm.redhat.com",
      "insights": true,
      "rhc": false
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

### Case: post to composer without enough quotas
function Test_postToComposerWithoutEnoughQuotas() {
  RESULT=$($CURLCMD -H "$HEADER2" -H 'Content-Type: application/json' --request POST --data @"$REQUEST_FILE" "$BASEURL/compose")
  EXIT_CODE=$(getExitCode "$RESULT")
  [[ "$EXIT_CODE" == 403 ]]
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

function Test_wrong_user_get_compose_status() {
  RESULT=$($CURLCMD -H "$HEADER2" --request GET "$BASEURL/composes/$COMPOSE_ID")
  EXIT_CODE=$(getExitCode "$RESULT")
  [[ $EXIT_CODE == 404 ]]
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

  # Tag image and snapshot with "gitlab-ci-test" tag
  $AWS_CMD ec2 create-tags \
    --resources "${AMI_IMAGE_ID}" \
    --tags Key=gitlab-ci-test,Value=true

  # Create key-pair
  $AWS_CMD ec2 create-key-pair --key-name "key-for-$AMI_IMAGE_ID" --query 'KeyMaterial' --output text > keypair.pem
  chmod 400 ./keypair.pem

  # Create an instance based on the ami
  $AWS_CMD ec2 run-instances --image-id "$AMI_IMAGE_ID" --count 1 --instance-type "$INSTANCE_TYPE" \
	  --key-name "key-for-$AMI_IMAGE_ID" \
	  --tag-specifications 'ResourceType=instance,Tags=[{Key=gitlab-ci-test,Value=true}]' > "$WORKDIR/instances.json"
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

  # Authenticate
  $GCP_CMD auth activate-service-account --key-file "$GOOGLE_APPLICATION_CREDENTIALS"
  # Set the default project to be used for commands
  $GCP_CMD config set project "$GCP_PROJECT"

  # Verify that the image boots and have customizations applied
  # Create SSH keys to use
  GCP_SSH_KEY="$WORKDIR/id_google_compute_engine"
  ssh-keygen -t rsa -f "$GCP_SSH_KEY" -C "$SSH_USER" -N ""

  # create the instance
  # resource ID can have max 62 characters, the $GCP_TEST_ID_HASH contains 56 characters
  GCP_INSTANCE_NAME="vm-$(uuidgen)"

  GCP_ZONE=$($GCP_CMD compute zones list --filter="region=$GCP_REGION" | jq -r ' .[] | select(.status == "UP") | .name' | shuf -n1)

  $GCP_CMD compute instances create "$GCP_INSTANCE_NAME" \
    --zone="$GCP_ZONE" \
    --image-project="$GCP_IMAGE_BUILDER_PROJECT" \
    --image="$GCP_IMAGE_NAME" \
    --labels=gitlab-ci-test=true

  HOST=$($GCP_CMD compute instances describe "$GCP_INSTANCE_NAME" --zone="$GCP_ZONE" --format='get(networkInterfaces[0].accessConfigs[0].natIP)')

  echo "⏱ Waiting for GCP instance to respond to ssh"
  instanceWaitSSH "$HOST"

  # Verify image
  _ssh="$GCP_CMD compute ssh --strict-host-key-checking=no --ssh-key-file=$GCP_SSH_KEY --zone=$GCP_ZONE --quiet $SSH_USER@$GCP_INSTANCE_NAME --"
  instanceCheck "$_ssh"
}

### Case: verify the result (image) of a finished compose in Azure
function Test_verifyComposeResultAzure() {
  UPLOAD_OPTIONS="$1"

  AZURE_IMAGE_NAME=$(echo "$UPLOAD_OPTIONS" | jq -r '.image_name')
  [[ -n "$AZURE_IMAGE_NAME" ]]

  set +x
  $AZURE_CMD login --service-principal --username "${V2_AZURE_CLIENT_ID}" --password "${V2_AZURE_CLIENT_SECRET}" --tenant "${AZURE_TENANT_ID}"
  set -x

  # verify that the image exists and tag it
  $AZURE_CMD image show --resource-group "${AZURE_RESOURCE_GROUP}" --name "${AZURE_IMAGE_NAME}"
  $AZURE_CMD image update --resource-group "${AZURE_RESOURCE_GROUP}" --name "${AZURE_IMAGE_NAME}" --tags gitlab-ci-test=true


  # Verify that the image boots and have customizations applied
  # Create SSH keys to use
  AZURE_SSH_KEY="$WORKDIR/id_azure"
  ssh-keygen -t rsa -f "$AZURE_SSH_KEY" -C "$SSH_USER" -N ""

  # Create network resources with predictable names
  $AZURE_CMD network nsg create --resource-group "$AZURE_RESOURCE_GROUP" --name "nsg-$TEST_ID" --location "$AZURE_LOCATION" --tags gitlab-ci-test=true
  $AZURE_CMD network nsg rule create --resource-group "$AZURE_RESOURCE_GROUP" \
      --nsg-name "nsg-$TEST_ID" \
      --name SSH \
      --priority 1001 \
      --access Allow \
      --protocol Tcp \
      --destination-address-prefixes '*' \
      --destination-port-ranges 22 \
      --source-port-ranges '*' \
      --source-address-prefixes '*'
  $AZURE_CMD network vnet create --resource-group "$AZURE_RESOURCE_GROUP" \
    --name "vnet-$TEST_ID" \
    --subnet-name "snet-$TEST_ID" \
    --location "$AZURE_LOCATION" \
    --tags gitlab-ci-test=true
  $AZURE_CMD network public-ip create --resource-group "$AZURE_RESOURCE_GROUP" --name "ip-$TEST_ID" --location "$AZURE_LOCATION" --tags gitlab-ci-test=true
  $AZURE_CMD network nic create --resource-group "$AZURE_RESOURCE_GROUP" \
      --name "iface-$TEST_ID" \
      --subnet "snet-$TEST_ID" \
      --vnet-name "vnet-$TEST_ID" \
      --network-security-group "nsg-$TEST_ID" \
      --public-ip-address "ip-$TEST_ID" \
      --location "$AZURE_LOCATION" \
      --tags gitlab-ci-test=true 

  # create the instance
  AZURE_INSTANCE_NAME="vm-$TEST_ID"
  $AZURE_CMD vm create --name "$AZURE_INSTANCE_NAME" \
    --resource-group "$AZURE_RESOURCE_GROUP" \
    --image "$AZURE_IMAGE_NAME" \
    --size "Standard_B1s" \
    --admin-username "$SSH_USER" \
    --ssh-key-values "$AZURE_SSH_KEY.pub" \
    --authentication-type "ssh" \
    --location "$AZURE_LOCATION" \
    --nics "iface-$TEST_ID" \
    --os-disk-name "disk-$TEST_ID" \
    --tags gitlab-ci-test=true
  $AZURE_CMD vm show --name "$AZURE_INSTANCE_NAME" --resource-group "$AZURE_RESOURCE_GROUP" --show-details > "$WORKDIR/vm_details.json"
  HOST=$(jq -r '.publicIps' "$WORKDIR/vm_details.json")

  echo "⏱ Waiting for Azure instance to respond to ssh"
  instanceWaitSSH "$HOST"

  # Verify image
  _ssh="ssh -oStrictHostKeyChecking=no -i $AZURE_SSH_KEY $SSH_USER@$HOST"
  instanceCheck "$_ssh"
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

### Case: verify package list of a finished compose
function Test_verifyComposeMetadata() {
  local RESULT
  RESULT=$($CURLCMD -H "$HEADER" --request GET "$BASEURL/composes/$COMPOSE_ID/metadata")
  EXIT_CODE=$(getExitCode "$RESULT")
  [[ $EXIT_CODE == 200 ]]

  local PACKAGENAMES
  PACKAGENAMES=$(getResponse "$RESULT" | jq -r '.packages[].name')
  if ! grep -q postgresql <<< "${PACKAGENAMES}"; then
      echo "'postgresql' not found in compose package list 😠"
      exit 1
  fi
}

function Test_getComposes() {
  RESULT=$($CURLCMD -H "$HEADER" -H 'Content-Type: application/json' "$BASEURL/composes")
  EXIT_CODE=$(getExitCode "$RESULT")
  [[ "$EXIT_CODE" == 200 ]]
  RESPONSE=$(getResponse "$RESULT" | jq -r '.data[0]')
  [[ $(echo "$RESPONSE" | jq -r '.id') == "$COMPOSE_ID" ]]
  diff <(echo "$RESPONSE" | jq -Sr '.request') <(jq -Sr '.' "$REQUEST_FILE")
}

#
# Which cloud provider are we testing?
#

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
Test_wrong_user_get_compose_status
Test_verifyComposeResult
Test_verifyComposeMetadata
Test_getComposes
Test_postToComposerWithoutEnoughQuotas

echo "########## Test success! ##########"
exit 0
