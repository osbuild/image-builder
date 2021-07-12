#!/usr/bin/bash

#
# RHEL for Edge Rest API test script
#
# The test flow in this file is:
# ------------ Test commit image --------------------------------------------------------
# 1. Call image builder Rest API. (build an rhel-edge-commit image and upload to aws s3)
# 2. Download commit image, extract it to /var/www/html, and serve it over httpd
# 3. Install Edge vm based on commit repo url, and run ansible playbook to check it.
#
# Test case for installer ISO image will be added soon.

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
HOST_ADDRESS=192.168.100.1
GUEST_ADDRESS=192.168.100.50
REPO_URL=http://$HOST_ADDRESS/repo

SSH_OPTIONS=(-o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o ConnectTimeout=5)
SSH_KEY=${IMAGE_BUILDER_TEST_DATA}/keyring/id_rsa

KS_FILE=${WORKDIR}/ks.cfg
COMMIT_FILENAME="commit.tar"

# Run before test begin to prepare test environment
function before_test() {
    # Get os information
    # shellcheck disable=SC1091
    source /etc/os-release

    # Set os-variant and boot location used by virt-install.
    case "${ID}-${VERSION_ID}" in
        "rhel-8.4")
            IMAGE_TYPE="rhel-edge-commit"
            OSTREE_REF="rhel/8/${ARCH}/edge"
            OS_VARIANT="rhel8-unknown"
            DISTRO="rhel-84"
            BOOT_LOCATION="http://download.devel.redhat.com/rel-eng/rhel-8/RHEL-8/latest-RHEL-8.4.0/compose/BaseOS/${ARCH}/os/";;
        *)
            echo "unsupported distro: ${ID}-${VERSION_ID}"
            exit 1;;
    esac

    # Check image-builder service status
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

    # ansible is not in RHEL repositories, cannot install it by spec file, hence enable EPEL and install ansible manually.
    sudo dnf install -y https://dl.fedoraproject.org/pub/epel/epel-release-latest-8.noarch.rpm
    sudo dnf install -y ansible

    # Start libvirtd and test it.
    greenprint "🚀 Starting libvirt daemon"
    sudo systemctl start libvirtd
    sudo virsh list --all > /dev/null

    sudo tee "${WORKDIR}"/integration.xml > /dev/null << EOF
<network>
  <name>integration</name>
  <uuid>1c8fe98c-b53a-4ca4-bbdb-deb0f26b3579</uuid>
  <forward mode='nat'>
    <nat>
      <port start='1024' end='65535'/>
    </nat>
  </forward>
  <bridge name='integration' stp='on' delay='0'/>
  <mac address='52:54:00:36:46:ef'/>
  <ip address='192.168.100.1' netmask='255.255.255.0'>
    <dhcp>
      <range start='192.168.100.2' end='192.168.100.254'/>
      <host mac='34:49:22:B0:83:30' name='vm-bios' ip='192.168.100.50'/>
    </dhcp>
  </ip>
</network>
EOF

    if ! sudo virsh net-info integration > /dev/null 2>&1; then
        sudo virsh net-define "${WORKDIR}"/integration.xml
        sudo virsh net-start integration
    fi

    # Allow anyone in the wheel group to talk to libvirt.
    greenprint "🚪 Allowing users in wheel group to talk to libvirt"
    WHEEL_GROUP=wheel
    if [[ $ID == rhel ]]; then
        WHEEL_GROUP=adm
    fi

    sudo tee /etc/polkit-1/rules.d/50-libvirt.rules > /dev/null << EOF
polkit.addRule(function(action, subject) {
    if (action.id == "org.libvirt.unix.manage" &&
        subject.isInGroup("${WHEEL_GROUP}")) {
            return polkit.Result.YES;
    }
});
EOF

    # Ensure SELinux is happy with our new images.
    greenprint "👿 Running restorecon on image directory"
    sudo restorecon -Rv /var/lib/libvirt/images/

    # Start httpd service
    greenprint "🚀 Starting httpd daemon"
    sudo systemctl start httpd
}

# Run after test finished to clean up test environment
function after_test() {
    # Clean up Edge VMs
    greenprint "🧹 Clean up BIOS VM"
    if [[ $(sudo virsh domstate "${IMAGE_KEY}-commit") == "running" ]]; then
        sudo virsh destroy "${IMAGE_KEY}-commit"
    fi
    sudo virsh undefine "${IMAGE_KEY}-commit"

    # Clean up temp files
    sudo rm -fr "$COMMIT_IMAGE_PATH"

    # Clean up work directory
    sudo rm -fr "$WORKDIR"
}

############### Common functions for image builder service ################

# Colorful print
function greenprint {
    echo -e "\033[1;32m${1}\033[0m"
}

# Get response from curl result
function get_response() {
    read -r -d '' -a ARR <<<"$1"
    echo "${ARR[@]::${#ARR[@]}-1}"
}

# Get exit code from curl result
function get_exit_code() {
    read -r -d '' -a ARR <<<"$1"
    echo "${ARR[-1]}"
}

### Call image builder service Edge Rest API
function post_to_composer() {
    RESULT=$($CURLCMD -H "$HEADER" -H 'Content-Type: application/json' --request POST --data @"$1" "$BASEURL/compose")
    EXIT_CODE=$(get_exit_code "$RESULT")
    [[ "$EXIT_CODE" == 201 ]]
    COMPOSE_ID=$(get_response "$RESULT" | jq -r '.id')
    [[ "$COMPOSE_ID" =~ ^\{?[A-F0-9a-f]{8}-[A-F0-9a-f]{4}-[A-F0-9a-f]{4}-[A-F0-9a-f]{4}-[A-F0-9a-f]{12}\}?$ ]]
    echo "Compose ID is: $COMPOSE_ID"
}

### Wait for the compose to finish successfully
function wait_for_compose() {
    while true
    do
        RESULT=$($CURLCMD -H "$HEADER" --request GET "$BASEURL/composes/$COMPOSE_ID")
        EXIT_CODE=$(get_exit_code "$RESULT")
        [[ $EXIT_CODE == 200 ]]

        COMPOSE_STATUS=$(get_response "$RESULT" | jq -r '.image_status.status')
        UPLOAD_STATUS=$(get_response "$RESULT" | jq -r '.image_status.upload_status.status')

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

# Download image from aws s3
function download_image() {
    RESULT=$($CURLCMD -H "$HEADER" --request GET "$BASEURL/composes/$COMPOSE_ID")
    RESULT_URL=$(get_response "$RESULT" | jq -r '.image_status.upload_status.options.url')
    echo "Downloading image, URL is: $RESULT_URL, save it as $1"
    $CURLCMD "$RESULT_URL" --output "$1"
}

# Test result checking
check_result () {
    greenprint "🎏 Checking for test result"
    if [[ $RESULTS == 1 ]]; then
        greenprint "💚 Success"
    else
        greenprint "❌ Failed"
        after_test
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

############################## Test Begin ################################

# prepare test environment
before_test

############################## Test commit image #########################

# call image-builder API to build commit image
greenprint "📼 Generate request body to build commit image"
sudo tee "${WORKDIR}"/commit_body.json > /dev/null << EOF
{
    "distribution": "${DISTRO}",
    "customizations": {
      "packages": [
        "python36"
      ]
    },
    "image_requests": [
      {
        "architecture": "${ARCH}",
        "image_type": "${IMAGE_TYPE}",
        "ostree": {
            "ref": "${OSTREE_REF}"
        },
        "upload_request": {
          "type": "aws.s3",
          "options": {}
        }
      }
    ]
}
EOF

post_to_composer "${WORKDIR}"/commit_body.json
wait_for_compose
download_image "${WORKDIR}/$COMMIT_FILENAME"

# extract commit image to http path
sudo tar -xf "${WORKDIR}/${COMMIT_FILENAME}" -C ${HTTPD_PATH}

# Write kickstart file for ostree image installation.
greenprint "Generate kickstart file"
sudo rm -fr "$KS_FILE"
sudo tee "$KS_FILE" > /dev/null << STOPHERE
text
lang en_US.UTF-8
keyboard us
timezone --utc Etc/UTC
selinux --enforcing
rootpw --lock --iscrypted locked
user --name=admin --groups=wheel --iscrypted --password=\$6\$1LgwKw9aOoAi/Zy9\$Pn3ErY1E8/yEanJ98evqKEW.DZp24HTuqXPJl6GYCm8uuobAmwxLv7rGCvTRZhxtcYdmC0.XnYRSR9Sh6de3p0
sshkey --username=admin "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQC61wMCjOSHwbVb4VfVyl5sn497qW4PsdQ7Ty7aD6wDNZ/QjjULkDV/yW5WjDlDQ7UqFH0Sr7vywjqDizUAqK7zM5FsUKsUXWHWwg/ehKg8j9xKcMv11AkFoUoujtfAujnKODkk58XSA9whPr7qcw3vPrmog680pnMSzf9LC7J6kXfs6lkoKfBh9VnlxusCrw2yg0qI1fHAZBLPx7mW6+me71QZsS6sVz8v8KXyrXsKTdnF50FjzHcK9HXDBtSJS5wA3fkcRYymJe0o6WMWNdgSRVpoSiWaHHmFgdMUJaYoCfhXzyl7LtNb3Q+Sveg+tJK7JaRXBLMUllOlJ6ll5Hod root@localhost"
bootloader --timeout=1 --append="net.ifnames=0 modprobe.blacklist=vc4"
network --bootproto=dhcp --device=link --activate --onboot=on
zerombr
clearpart --all --initlabel --disklabel=msdos
autopart --nohome --noswap --type=plain
ostreesetup --nogpg --osname=${IMAGE_TYPE} --remote=${IMAGE_TYPE} --url=http://192.168.100.1/repo/ --ref=${OSTREE_REF}
poweroff
%post --log=/var/log/anaconda/post-install.log --erroronfail
# no sudo password for user admin
echo -e 'admin\tALL=(ALL)\tNOPASSWD: ALL' >> /etc/sudoers
# Remove any persistent NIC rules generated by udev
rm -vf /etc/udev/rules.d/*persistent-net*.rules
# And ensure that we will do DHCP on eth0 on startup
cat > /etc/sysconfig/network-scripts/ifcfg-eth0 << EOF
DEVICE="eth0"
BOOTPROTO="dhcp"
ONBOOT="yes"
TYPE="Ethernet"
PERSISTENT_DHCLIENT="yes"
EOF
echo "Packages within this iot or edge image:"
echo "-----------------------------------------------------------------------"
rpm -qa | sort
echo "-----------------------------------------------------------------------"
# Note that running rpm recreates the rpm db files which aren't needed/wanted
rm -f /var/lib/rpm/__db*
echo "Zeroing out empty space."
# This forces the filesystem to reclaim space from deleted files
dd bs=1M if=/dev/zero of=/var/tmp/zeros || :
rm -f /var/tmp/zeros
echo "(Don't worry -- that out-of-space error was expected.)"
%end
STOPHERE

# Create qcow2 file for virt install.
greenprint "🖥 Create qcow2 file for virt install"
COMMIT_IMAGE_PATH=/var/lib/libvirt/images/${IMAGE_KEY}-commit.qcow2
sudo qemu-img create -f qcow2 "${COMMIT_IMAGE_PATH}" 20G

# Install Edge vm
sudo virt-install  --initrd-inject="${KS_FILE}" \
                   --extra-args="ks=file:/ks.cfg console=ttyS0,115200" \
                   --name="${IMAGE_KEY}-commit"\
                   --disk path="${COMMIT_IMAGE_PATH}",format=qcow2 \
                   --ram 3072 \
                   --vcpus 2 \
                   --network network=integration,mac=34:49:22:B0:83:30 \
                   --os-type linux \
                   --os-variant ${OS_VARIANT} \
                   --location "${BOOT_LOCATION}" \
                   --nographics \
                   --noautoconsole \
                   --wait=-1 \
                   --noreboot

# Start VM
greenprint "📟 Start BIOS VM"
sudo virsh start "${IMAGE_KEY}-commit"

# Check for ssh ready to go.
greenprint "🛃 Checking for SSH is ready to go"
for LOOP_COUNTER in $(seq 0 30); do
    RESULTS="$(wait_for_ssh $GUEST_ADDRESS)"
    if [[ $RESULTS == 1 ]]; then
        echo "SSH is ready now! 🥳"
        break
    fi
    echo "SSH is not ready, Waiting...($LOOP_COUNTER/30)"
    sleep 10
done

check_result

# Get ostree commit value.
greenprint "🕹 Get ostree install commit value"
INSTALL_HASH=$(curl "${REPO_URL}/refs/heads/${OSTREE_REF}")

# Test Edge OS
greenprint "📼 Run Edge tests on VM"
sudo tee "${WORKDIR}"/inventory > /dev/null << EOF
[ostree_guest]
${GUEST_ADDRESS}
[ostree_guest:vars]
ansible_python_interpreter=/usr/bin/python3
ansible_user=admin
ansible_private_key_file=${SSH_KEY}
ansible_ssh_common_args="-o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null"
EOF

# Run ansible playbook on Edge vm to do sanity check
sudo ANSIBLE_STDOUT_CALLBACK=debug ansible-playbook -v -i "${WORKDIR}"/inventory -e image_type=rhel-edge-commit -e ostree_commit="${INSTALL_HASH}" -e workspace="$WORKDIR" ${IMAGE_BUILDER_TEST_DATA}/edge/check_ostree.yaml || RESULTS=0
check_result

# cleanup test environment
after_test

############################## Test Finish ################################

echo "########## Test success! ##########"
exit 0
