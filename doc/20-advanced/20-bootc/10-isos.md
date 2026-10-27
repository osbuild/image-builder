# ISOs

## generic-iso

`image-builder` can build ISOs out of bootable containers. The image type to use to build ISOs is the `generic-iso` image type. `image-builder` takes the bootable container and explodes the relevant parts to put them into the correct places on the ISO this means that there is a small contract in place on what `image-builder` expects to exist in your bootable container:

1. A kernel must live in `/usr/lib/module/*/vmlinuz`. If there are multiple kernels the behavior is undefined. This kernel will be placed in `/images/pxeboot/vmlinuz` on the ISO filesystem.
2. An initramfs is expected to be next to the kernel with the filename `initramfs.img`. The initramfs is placed in `/images/pxeboot/initrd.img` on the ISO filesystem. It must include the dmsquash-live module which is installed by dracut-live package.
3. The UEFI vendor is sourced by a directory name in `/usr/lib/efi/shim/*EFI/$VENDOR`. If there are multiple directories the behavior is undefined. The `BOOT` directory is always ignored.
4. shim and grub2 EFI binaries (`shimx64.efi`, `mmx64.efi`, `gcdx64.efi`) are expected to be present in `/boot/efi/EFI/$VENDOR`.
5. `/usr/share/grub2/unicode.pf` and `/usr/lib/grub/i386-pc` are expected to present. These are normally provided by the `grub2-common` and `grub2-pc-modules` packages respectively. The latter is only necessary on `x86_64`.
5. Required executables in the container are: `podman`, `mksquashfs`, `xorriso`, `implantisomd5`, `grub2-mkimage` and `python`. If you are using a separate build container then these executables must exist in the build container.
6. The container image is converted to a `squashfs` filesystem and put into `/LiveOS/squashfs.img` in the ISO.

You can [define additional configuration](./05-sources-of-configuration.md#isoyaml) for an ISO inside your container.

If a `--bootc-installer-payload-ref` argument is optionally passed to `image-builder` when building a `generic-iso` then the container reference is copied from the hosts container storage to `/var/lib/containers/storage` in the squashfs filesystem.

### Example Simple Containerfile

This container file builds a simple bootable iso with nothing extra.

```Dockerfile
FROM quay.io/fedora/fedora-bootc:rawhide

# these are necessary build tools. if you use a separate build container then
# these tools should be installed there
RUN dnf install -qy \
    grub2-efi-*-cdboot \
    dracut-live \
    implantisomd5 \
    xorrisofs \
    squashfs-tools
RUN dnf clean all
RUN mkdir -p /boot/efi && cp -ra /usr/lib/efi/*/*/EFI /boot/efi

# Include the dmsquash-live module in the initramfs
RUN cat <<EOF > /usr/lib/dracut/dracut.conf.d/40-iso.conf
compress="xz"
add_dracutmodules+=" qemu qemu-net livenet dmsquash-live "
early_microcode="no"
EOF

# Rebuild the initrd
RUN set -xe; kver=$(ls /usr/lib/modules); env DRACUT_NO_XATTR=1 dracut -vf /usr/lib/modules/$kver/initramfs.img "$kver"

# ---

# some configuration for our ISO

RUN mkdir -p /usr/lib/image-builder/bootc

RUN cat <<EOF > /usr/lib/image-builder/bootc/iso.yaml
label: "Fedora"
kernel_args:
  - console=ttyS0
grub2:
  timeout: 5
  entries:
    - name: "Fedora"
      linux: \${kernelpath} \${root}
      initrd: \${initrdpath}
EOF

# Disable services that won't work on an iso
RUN systemctl mask bootloader-update.service
```

You can then build this into an ISO:
```
sudo podman build -t localhost/simple-iso -f Containerfile
sudo image-builder build --bootc-ref localhost/simple-iso --bootc-default-fs ext4 generic-iso
```

### Example Anaconda Installer Containerfile

This container file builds a `Fedora` "payload" installer (a `boot.iso`). It installs the container that's mentioned in the `/usr/share/anaconda/interactive-defaults.ks` kickstart file.

```Dockerfile
FROM quay.io/fedora/fedora-bootc:rawhide

RUN dnf install -qy \
    anaconda \
    anaconda-install-img-deps \
    anaconda-dracut \
    dracut-config-generic \
    dracut-network \
    net-tools \
    grub2-efi-x64-cdboot \
    plymouth \
    default-fonts-core-sans \
    default-fonts-other-sans \
    google-noto-sans-cjk-fonts

# these are necessary build tools. if you use a separate build container then
# these tools should be installed there
RUN dnf install -qy \
    xorrisofs \
    squashfs-tools

RUN dnf clean all

RUN mkdir -p /boot/efi && cp -ra /usr/lib/efi/*/*/EFI /boot/efi

# ---

# some configuration for our ISO

RUN mkdir -p /usr/lib/image-builder/bootc

RUN cat <<EOF > /usr/lib/image-builder/bootc/iso.yaml
label: "Fedora-bootc-Installer"
grub2:
  timeout: 5
  entries:
    - name: "Install Fedora (bootc)"
      linux: \${kernelpath} \${root} inst.graphical
      initrd: \${initrdpath}
EOF

# some configuration for anaconda

COPY <<EOT /usr/share/anaconda/interactive-defaults.ks
bootc --source-imgref registry:quay.io/fedora/fedora-bootc:rawhide --target-imgref quay.io/fedora/fedora-bootc:rawhide
EOT

# ---

# these things are normally performed by `lorax` to make `anaconda` work; this is the
# bare minimum to get things to work

RUN echo "install:x:0:0:root:/root:/usr/libexec/anaconda/run-anaconda" >> /etc/passwd && \
    echo "install::14438:0:99999:7:::" >> /etc/shadow && \
    passwd -d root

RUN mv /usr/share/anaconda/list-harddrives-stub /usr/bin/list-harddrives && \
    mv /etc/yum.repos.d /etc/anaconda.repos.d && \
    ln -s /lib/systemd/system/anaconda.target /etc/systemd/system/default.target && \
    rm -v /usr/lib/systemd/system-generators/systemd-gpt-auto-generator

RUN ln -s /usr/lib/systemd/system/anaconda-shell@.service /usr/lib/systemd/system/autovt@.service

RUN mkdir -p /usr/lib/systemd/logind.conf.d
COPY <<EOT /usr/lib/systemd/logind.conf.d/anaconda-shell.conf
[Login]
ReserveVT=2
EOT

RUN mkdir "$(realpath /root)" && \
    kernel=$(kernel-install list --json pretty | jq -r '.[] | select(.has_kernel == true) | .version') && \
    DRACUT_NO_XATTR=1 dracut --force -v --zstd --reproducible --no-hostonly \
        --add "anaconda" \
        "/usr/lib/modules/${kernel}/initramfs.img" "${kernel}"

RUN mkdir /etc/systemd/user/pipewire.service.d/
COPY <<EOT /etc/systemd/user/pipewire.service.d/allowroot.conf
[Unit]
ConditionUser=
EOT

RUN mkdir /etc/systemd/user/pipewire.socket.d/
COPY <<EOT /etc/systemd/user/pipewire.socket.d/allowroot.conf
[Unit]
ConditionUser=
EOT
```

You can then build this into an ISO:

```
sudo podman build -t localhost/iso -f Containerfile
sudo image-builder build --bootc-ref localhost/iso --bootc-default-fs ext4 generic-iso
```

> [!TIP]
> For offline installs, pass `--bootc-installer-payload-ref` to embed the payload
> container in the ISO installer environment's containers-storage store. The
> Anaconda kickstart must then install from that local store using a
> `containers-storage:` source image reference.
>
> Example:
>
> ```Dockerfile
> COPY <<EOT /usr/share/anaconda/interactive-defaults.ks
> bootc --source-imgref containers-storage:quay.io/fedora/fedora-bootc:rawhide --target-imgref quay.io/fedora/fedora-bootc:rawhide
> EOT
> ```
>
> ```
> PAYLOADREF="quay.io/fedora/fedora-bootc:rawhide"
> sudo podman build -t localhost/iso -f Containerfile
> sudo image-builder build --bootc-ref localhost/iso --bootc-installer-payload-ref "$PAYLOADREF" --bootc-default-fs ext4 generic-iso
> ```

> [!WARNING]
> *A `bootc`-system installed through Anaconda will fail to start the `systemd-remount-fs.service`. See [here](https://forge.fedoraproject.org/atomic-desktops/tracker/issues/72#issuecomment-593808) and [here](https://bugzilla.redhat.com/show_bug.cgi?id=2332319) for more information.*

For more examples, including for other operating systems, you can take a look at [this demonstration repository](github.com/osbuild/bootc-foundry). The [bootc-foundry repository](https://github.com/osbuild/bootc-foundry) may also be of interest.

## ostree-iso

The `ostree-iso` image is similar to the `pxe-tar-xz` image where the root
filesystem is an ostree commit created from the bootc image specified with
`--bootc-ref`. It is just wrapped up in a bootable iso. You can customize it
with blueprints, add a payload with `--bootc-installer-payload-ref` or by
running podman in the Containerfile.

It has the same requirements on the initrd as the `generic-iso`, otherwise it
will not boot. `image-builder` will inspect the initrd's module list before
starting the build and return an error if you have not included all the needed
modules.

### Example Containerfile

This container file builds a simple bootable iso with nothing extra.

```Dockerfile
FROM quay.io/fedora/fedora-bootc:rawhide

# these are necessary build tools. if you use a separate build container then
# these tools should be installed there
RUN dnf install -qy \
    grub2-efi-*-cdboot \
    dracut-live \
    erofs-utils \
    implantisomd5 \
    shim \
    xorrisofs
RUN dnf clean all
RUN mkdir -p /boot/efi && cp -ra /usr/lib/efi/*/*/EFI /boot/efi

# Override using composefs for ostree (it is incompatible with the erofs rootfs)
RUN cat <<EOF > /usr/lib/ostree/prepare-root.conf
[composefs]
enabled = no
[sysroot]
readonly = true
EOF

# Include the dmsquash-live module in the initramfs
RUN cat <<EOF > /usr/lib/dracut/dracut.conf.d/40-iso.conf
compress="xz"
add_dracutmodules+=" qemu qemu-net livenet dmsquash-live "
early_microcode="no"
EOF

# Rebuild the initrd
RUN set -xe; kver=$(ls /usr/lib/modules); env DRACUT_NO_XATTR=1 dracut -vf /usr/lib/modules/$kver/initramfs.img "$kver"

# ---

# some configuration for our ISO

RUN mkdir -p /usr/lib/image-builder/bootc

RUN cat <<EOF > /usr/lib/image-builder/bootc/iso.yaml
label: "Fedora"
kernel_args:
  - console=ttyS0
grub2:
  timeout: 5
  entries:
    - name: "Fedora"
      linux: \${kernelpath} \${root}
      initrd: \${initrdpath}
EOF

# Disable services that won't work on an iso
RUN systemctl mask \
    bootc-generic-growpart.service \
    bootc-publish-rhsm-facts.service \
    bootloader-update.service \
    rpm-ostree-fix-shadow-mode.service
```

You can then build this into an ISO:
```
sudo podman build -t localhost/simple-iso -f Containerfile
sudo image-builder build --bootc-ref localhost/simple-iso --bootc-default-fs ext4 ostree-iso
```

If a `--bootc-installer-payload-ref` argument is optionally passed to
`image-builder` when building a `ostree-iso` then the container reference is
copied from the hosts container storage to `/var/lib/containers/storage` in the
erofs filesystem. It can then be used from the booted iso,
eg. `podman run --rm -it fedora:latest`

## Historical

### `bootc-installer`

There's an alternative image type called `bootc-installer` which makes more assumptions about the contents of the container. You should prefer using `generic-iso`.

### `anaconda-iso`

There was an alternative image type called `anaconda-iso` or `iso` in `bootc-image-builder` but this
image type is not available in `image-builder`. See the [legacy ISOs](./20-legacy-isos.md) and the [migration guide](./50-migration.md).
