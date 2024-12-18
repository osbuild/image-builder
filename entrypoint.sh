#!/bin/sh

set -e

# TODO: share code with bib to do the setup automatically
# see https://github.com/teamsbc/container-for-osbuild/blob/main/entrypoint.bash (thanks simon)
# and https://github.com/osbuild/bootc-image-builder/blob/main/bib/internal/setup/setup.go#L21 (thanks ondrej,achilleas,colin)
mkdir /run/osbuild
mkdir /run/osbuild-store

mount -t tmpfs tmpfs /run/osbuild

cp -p /usr/bin/osbuild /run/osbuild/osbuild

chcon system_u:object_r:install_exec_t:s0 /run/osbuild/osbuild

mount -t devtmpfs devtmpfs /dev
mount --bind /run/osbuild/osbuild /usr/bin/osbuild

# XXX: make this nicer
cd /output
/usr/bin/image-builder --store=/store "$@"
