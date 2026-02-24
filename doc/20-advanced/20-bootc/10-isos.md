# ISOs

## Generic

`image-builder` can build ISOs out of bootable containers. The image type to use to build ISOs is the `bootc-generic-iso` image type. `image-builder` takes the bootable container and explodes the relevant parts to put them into the correct places on the ISO this means that there is a small contract in place on what `image-builder` expects to exist in your bootable container:

1. A kernel must live in `/usr/lib/module/*/vmlinuz`. If there are multiple kernels the behavior is undefined. This kernel will be placed in `/images/pxeboot/vmlinuz` on the ISO filesystem.
2. An initramfs is expected to be next to the kernel with the filename `initramfs.img`. The initramfs is placed in `/images/pxeboot/initrd.img` on the ISO filesystem.
3. The UEFI vendor is sourced by a directory name in `/usr/lib/efi/shim/*EFI/$VENDOR`. If there are multiple directories the behavior is undefined. The `BOOT` directory is always ignored.
4. shim and grub2 EFI binaries (`shimx64.efi`, `mmx64.efi`, `gcdx64.efi`) are expected to be present in `/boot/efi/EFI/$VENDOR`.
5. Required executables in the container are: `podman`, `mksquashfs`, `xorriso`, `implantisomd5`, `grub2-mkimage` and `python`. If you are using a separate build container then these executables must exist in the build container.
6. The container image is converted to a `squashfs` filesystem and put into `/LiveOS/squashfs.img` in the ISO.

You can [define additional configuration](./05-sources-of-configuration.md#isoyaml) for an ISO inside your container.

If a `--bootc-installer-payload-ref` argument is optionally passed to `image-builder` when building a `bootc-generic-iso` then the container reference is copied from the hosts container storage to `/var/lib/containers/storage` in the squashfs filesystem.

### Example Containerfile

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

COPY <<EOT /usr/lib/image-builder/bootc/iso.yaml
label: "Fedora-bootc-Installer"
grub2:
  entries:
    - name: "Install Fedora (bootc)"
      linux: "/images/pxeboot/vmlinuz inst.stage2=hd:LABEL=Fedora-bootc-Installer console=tty0 inst.graphical selinux=0 rhgb quiet"
      initrd: "/images/pxeboot/initrd.img"
EOT

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

RUN mkdir /usr/lib/systemd/logind.conf.d
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
sudo image-builder build --bootc-ref localhost/iso --bootc-default-fs ext4 bootc-generic-iso
```

> [!WARNING]
> *A `bootc`-system installed through Anaconda will fail to start the `systemd-remount-fs.service`. See [here](https://forge.fedoraproject.org/atomic-desktops/tracker/issues/72#issuecomment-593808) and [here](https://bugzilla.redhat.com/show_bug.cgi?id=2332319) for more information.*


For more examples, including for other operating systems, you can take a look at [this demonstration repository](https://github.com/ondrejbudai/bootc-isos).

## Historical

### `bootc-installer`

There's an alternative image type called `bootc-installer` which makes more assumptions about the contents of the container. You should prefer using `bootc-generic-iso`.

### `anaconda-iso`

There was an alternative image type called `anaconda-iso` or `iso` in `bootc-image-builder` but this image type is not available in `image-builder`. See the [migration guide](./50-migration.md).
