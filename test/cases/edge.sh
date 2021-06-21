#!/usr/bin/bash

#
# Edge Rest API integration test
#
# The test flow in this file is:
# 1. Call image builder Rest API. (build an rhel-edge-commit image and upload to aws s3)
# 2. Download commit image, extract it to /var/www/html, and serve it over httpd
# 3. Call image builder Rest API. (build an rhel-edge-install image and upload to aws s3)
# 4. Download installer image.
# 5. Install a libvirt vm with the installer image.
# 6. Run ansible playbook to do some sanity check of vm.

set -euxo pipefail

############### Common variables for CI ###########################
WORKDIR=$(mktemp -d)
IMAGE_BUILDER_TEST_DATA=/usr/share/tests/image-builder

############### Common variables for image builder ################
PORT="8086"
CURLCMD='curl -w %{http_code}'
HEADER="x-rh-identity: eyJlbnRpdGxlbWVudHMiOnsiaW5zaWdodHMiOnsiaXNfZW50aXRsZWQiOnRydWV9LCJzbWFydF9tYW5hZ2VtZW50Ijp7ImlzX2VudGl0bGVkIjp0cnVlfSwib3BlbnNoaWZ0Ijp7ImlzX2VudGl0bGVkIjp0cnVlfSwiaHlicmlkIjp7ImlzX2VudGl0bGVkIjp0cnVlfSwibWlncmF0aW9ucyI6eyJpc19lbnRpdGxlZCI6dHJ1ZX0sImFuc2libGUiOnsiaXNfZW50aXRsZWQiOnRydWV9fSwiaWRlbnRpdHkiOnsiYWNjb3VudF9udW1iZXIiOiIwMDAwMDAiLCJ0eXBlIjoiVXNlciIsInVzZXIiOnsidXNlcm5hbWUiOiJ1c2VyIiwiZW1haWwiOiJ1c2VyQHVzZXIudXNlciIsImZpcnN0X25hbWUiOiJ1c2VyIiwibGFzdF9uYW1lIjoidXNlciIsImlzX2FjdGl2ZSI6dHJ1ZSwiaXNfb3JnX2FkbWluIjp0cnVlLCJpc19pbnRlcm5hbCI6dHJ1ZSwibG9jYWxlIjoiZW4tVVMifSwiaW50ZXJuYWwiOnsib3JnX2lkIjoiMDAwMDAwIn19fQ=="
ADDRESS="localhost"
BASEURL="http://$ADDRESS:$PORT/api/image-builder/v1.0"
ARCH=$(uname -m)

############### Common variables for Edge ################
HTTPD_PATH="/var/www/html"
TEST_UUID=$(uuidgen)
IMAGE_KEY="edge-${TEST_UUID}"
OS_VARIANT="rhel8-unknown"
OSTREE_REF="rhel/8/${ARCH}/edge"
BIOS_GUEST_ADDRESS=192.168.100.50
REPO_URL=http://192.168.100.1/repo

# SSH_OPTIONS=(-o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o ConnectTimeout=5)
# SSH_KEY=key/ostree_key
SSH_OPTIONS=(-o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o ConnectTimeout=5)
SSH_KEY=${IMAGE_BUILDER_TEST_DATA}/keyring/id_rsa

KS_FILE=${IMAGE_BUILDER_TEST_DATA}/edge/ks.cfg

COMMIT_FILENAME="commit.tar"
ISO_FILENAME="installer.iso"
REQUEST_JSON_FOR_COMMIT=${IMAGE_BUILDER_TEST_DATA}/edge/commit_body.json
REQUEST_JSON_FOR_ISO=${IMAGE_BUILDER_TEST_DATA}/edge/installer_body.json


############### Common functions for image builder service ################
# Wait until image builder service is ready

function get_response() {
  read -r -d '' -a ARR <<<"$1"
  echo "${ARR[@]::${#ARR[@]}-1}"
}

function get_exit_code() {
  read -r -d '' -a ARR <<<"$1"
  echo "${ARR[-1]}"
}

### Call image builder service Edge Rest API
function post_to_composer() {
  RESULT=$($CURLCMD -H "$HEADER" -H 'Content-Type: application/json' --request POST --data @"$1" "$BASEURL/compose")
  echo "Post result is: $RESULT"
  EXIT_CODE=$(getExitCode "$RESULT")
  [[ "$EXIT_CODE" == 201 ]]
  COMPOSE_ID=$(getResponse "$RESULT" | jq -r '.id')
  [[ "$COMPOSE_ID" =~ ^\{?[A-F0-9a-f]{8}-[A-F0-9a-f]{4}-[A-F0-9a-f]{4}-[A-F0-9a-f]{4}-[A-F0-9a-f]{12}\}?$ ]]
  echo "Compose ID is: $COMPOSE_ID"
}

### Wait for the compose to finish successfully
function wait_for_compose() {
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

    echo "Compose status is: $COMPOSE_STATUS, upload status is $UPLOAD_STATUS"
    sleep 30
  done
}

# Download image from aws s3
function download_image() {
    RESULT=$($CURLCMD -H "$HEADER" --request GET "$BASEURL/composes/$COMPOSE_ID")
    RESULT_URL=$(getResponse "$RESULT" | jq -r '.image_status.upload_status.options.url')
    echo "Downloading image, URL is: $RESULT_URL, save it as $1"
    curl $RESULT_URL --output "$1"
}

# Test result checking
check_result () {
    greenprint "🎏 Checking for test result"
    if [[ $RESULTS == 1 ]]; then
        greenprint "💚 Success"
    else
        greenprint "❌ Failed"
        clean_up
        exit 1
    fi
}

# Wait for the ssh server up to be.
function wait_for_ssh() {
    SSH_STATUS=$(sudo ssh "${SSH_OPTIONS[@]}" -i "${SSH_KEY}" admin@"${1}" '/bin/bash -c "echo -n READY"')
    if [[ $SSH_STATUS == READY ]]; then
        echo 1
    else
        echo 0
    fi
}

function before_test() {
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

    if rpm -qa | grep -q firewalld; then
        sudo systemctl disable firewalld --now
    fi

    # Start libvirtd and test it.
    greenprint "🚀 Starting libvirt daemon"
    sudo systemctl start libvirtd
    sudo virsh list --all > /dev/null

    if ! sudo virsh net-info integration > /dev/null 2>&1; then
        sudo virsh net-define ${IMAGE_BUILDER_TEST_DATA}/edge/integration.xml
        sudo virsh net-start integration
    fi

    # Allow anyone in the wheel group to talk to libvirt.
    greenprint "🚪 Allowing users in wheel group to talk to libvirt"
    WHEEL_GROUP=wheel
    if [[ $ID == rhel ]]; then
        WHEEL_GROUP=adm
    fi

    sudo cp ${IMAGE_BUILDER_TEST_DATA}/edge/50-libvirt.rules /etc/polkit-1/rules.d/50-libvirt.rules

    greenprint "🚀 Starting httpd daemon"
    sudo systemctl start httpd
}

function after_test() {
    rm -rf "$WORKDIR"
    # Clean up BIOS VM
    greenprint "🧹 Clean up BIOS VM"
    if [[ $(sudo virsh domstate "${IMAGE_KEY}-bios") == "running" ]]; then
        sudo virsh destroy "${IMAGE_KEY}-bios"
    fi
    sudo virsh undefine "${IMAGE_KEY}-bios"
    sudo rm -f "$LIBVIRT_BIOS_IMAGE_PATH"
    sudo rm -f /var/lib/libvirt/images/"${ISO_FILENAME}"
}

############### Test begin ################
beforeTest

post_to_composer "$REQUEST_JSON_FOR_COMMIT"
wait_for_compose
download_image "${WORKDIR}/$COMMIT_FILENAME"

sudo tar -xf "${COMMIT_FILENAME}" -C ${HTTPD_PATH}

post_to_composer "$REQUEST_JSON_FOR_ISO"
wait_for_compose
download_image "${WORKDIR}/$ISO_FILENAME"

# Ensure SELinux is happy with our new images.
greenprint "👿 Running restorecon on image directory"
sudo restorecon -Rv /var/lib/libvirt/images/

# Create qcow2 file for virt install.
greenprint "🖥 Create qcow2 file for virt install"
LIBVIRT_BIOS_IMAGE_PATH=/var/lib/libvirt/images/${IMAGE_KEY}-bios.qcow2
sudo qemu-img create -f qcow2 "${LIBVIRT_BIOS_IMAGE_PATH}" 20G

sudo mv "${WORKDIR}/${ISO_FILENAME}" /var/lib/libvirt/images/

# Install ostree image via anaconda.
greenprint "💿 Install ostree image via installer(ISO) on BIOS VM"
sudo virt-install  --initrd-inject="${KS_FILE}" \
                --extra-args="inst.ks=file:/ks.cfg console=ttyS0,115200" \
                --name="${IMAGE_KEY}-bios" \
                --disk path="${LIBVIRT_BIOS_IMAGE_PATH}",format=qcow2 \
                --ram 3072 \
                --vcpus 2 \
                --network network=integration,mac=34:49:22:B0:83:30 \
                --os-type linux \
                --os-variant ${OS_VARIANT} \
                --location "/var/lib/libvirt/images/${ISO_FILENAME}" \
                --nographics \
                --noautoconsole \
                --wait=-1 \
                --noreboot

# Start VM.
greenprint "📟 Start BIOS VM"
sudo virsh start "${IMAGE_KEY}-bios"

# Check for ssh ready to go.
greenprint "🛃 Checking for SSH is ready to go"
for LOOP_COUNTER in $(seq 0 30); do
    RESULTS="$(wait_for_ssh $BIOS_GUEST_ADDRESS)"
    if [[ $RESULTS == 1 ]]; then
        echo "SSH is ready now! 🥳"
        break
    fi
    sleep 10
done

check_result


# Get ostree commit value.
greenprint "🕹 Get ostree install commit value"
INSTALL_HASH=$(curl "${REPO_URL}/refs/heads/${OSTREE_REF}")

# Run Edge test on BIOS VM
# Add instance IP address into /etc/ansible/hosts
sudo tee "${WORKDIR}"/inventory > /dev/null << EOF
[ostree_guest]
${BIOS_GUEST_ADDRESS}

[ostree_guest:vars]
ansible_python_interpreter=/usr/bin/python3
ansible_user=admin
ansible_private_key_file=${SSH_KEY}
ansible_ssh_common_args="-o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null"
EOF

# Test IoT/Edge OS
greenprint "📼 Run Edge tests on BIOS VM"
sudo ANSIBLE_STDOUT_CALLBACK=debug ansible-playbook -v -i "${WORKDIR}"/inventory -e image_type=rhel-edge -e ostree_commit="${INSTALL_HASH}" ${IMAGE_BUILDER_TEST_DATA}/edge/check_ostree.yaml || RESULTS=0
check_result

runTest

afterTest

echo "########## Test success! ##########"
exit 0