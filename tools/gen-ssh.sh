#!/usr/bin/bash

# Create SSH key
SSH_DATA_DIR="$(mktemp -d)"
SSH_KEY=${SSH_DATA_DIR}/id_rsa
ssh-keygen -f "${SSH_KEY}" -N "" -q -t rsa

# Return temp directory
echo "${SSH_DATA_DIR}"
