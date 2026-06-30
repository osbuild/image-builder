# ISOs

## Simple Fedora bootc generic-iso

This ISO contains a bootable bootc root filesystem in /LiveOS/squashfs.img, the ISO
is bootable as an ISO or as an image written to a USB flash drive.

This ISO is created from a bootc container using image-builder. You create a
custom container using podman, then run image-builder to turn it into an ISO.

### bootc container

The bootc container has a few requirements:

* Be based on a bootc container, eg. quay.io/fedora/fedora-bootc:latest
* Include the dracut-live, erofs-utils packages
* Include grub2 ISO bootloader related tools
 - grub2-efi-*-cdboot xorriso isomd5sum shim
* Configure dracut to add the dmsquash-live module
* Configure ostree to not use composefs
* Rebuild the initramfs so that it includes the dmsquash-live module
* Optionally setup the ISO menus and kernel cmdline with iso.yaml

### Sample Fedora bootc container

This is a simple example using the `image-builder` cmdline tool and a local install of podman.

Save this in `Containerfile`:

```
FROM quay.io/fedora/fedora-bootc:latest
RUN dnf -y install grub2-efi-*-cdboot xorriso isomd5sum dracut-live erofs-utils shim && dnf clean all
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

# Override the default ISO menus
RUN mkdir -p /usr/lib/image-builder/bootc
RUN cat <<EOF > /usr/lib/image-builder/bootc/iso.yaml
label: bootc-generic
kernel_args:
  - console=ttyS0
grub2:
  timeout: 5
  entries:
    - name: Boot Linux
      linux: \${kernelpath} \${root}
      initrd: \${initrdpath}
    - name: Boot Linux With debug
      linux: \${kernelpath} \${root} rd.debug=1
      initrd: \${initrdpath}
EOF

# Rebuild the initrd
RUN set -xe; kver=$(ls /usr/lib/modules); env DRACUT_NO_XATTR=1 dracut -vf /usr/lib/modules/$kver/initramfs.img "$kver"

# Mask services that aren't compatible with running from an ISO
RUN systemctl mask bootc-generic-growpart.service bootc-publish-rhsm-facts.service bootloader-update.service rpm-ostree-fix-shadow-mode.service

RUN bootc container lint
```

Build this container using podman:
```
podman build -f ./Containerfile -t bootc-iso
```

Run `image-builder` to create the ISO:
```
image-builder build --bootc-default-fs ext4 --bootc-ref localhost/bootc-iso generic-iso
```

If your container is on a remote system replace the `localhost/bootc-iso` with the right url.

## Simple CentOS 10 bootc generic-iso

The CentOS container is slightly different from the Fedora container due to the
bootloader files being in a different location.

Replace the top 3 lines with:
```
FROM quay.io/centos/centos-bootc:c10s
RUN dnf -y install grub2-efi-*-cdboot xorriso isomd5sum dracut-live erofs-utils shim && dnf clean all
RUN cp -r /usr/lib/bootupd/updates/EFI/* /boot/efi/EFI/
```

The remainder of the Containerfile is identical to the Fedora example.

## Embedded containers

You can add a container to the bootc image in two ways:

### podman pull in Containerfile

Just use `podman` as you normally would to include containers:

```
RUN podman pull registry.fedoraproject.org/fedora:44
RUN podman images
```

You can then use them from the root account of the booted generic-iso.

```
podman run --rm -it fedora:44
```

### image-builder --bootc-installer-payload-ref

From the cmdline when building the generic iso you can include a container by passing the
`--bootc-installer-payload-ref`:

```
image-builder build --bootc-default-fs ext4 --bootc-ref localhost/bootc-iso --bootc-installer-payload-ref registry.fedoraproject.org/fedora:44 generic-iso
```

Note that the container must already be present in the local container storage
before you can add it to the image.

You can use the container as root after booting the generic-iso.

```
podman run --rm -it fedora:44
```

## User config

When building the ISO you can use the `image-builder --blueprint user.toml`
option to customize the users, including root. See the documentation at
https://osbuild.org/docs/user-guide/blueprint-reference/#additional-users

For example, to set the root password use a minimal blueprint like this:
```
name = "setup-root"
version = "1.0.0"

[[customizations.user]]
name = "root"
password = "root-password"
```

And run `image-builder` like so:
```
image-builder build --bootc-default-fs ext4 --bootc-ref localhost/bootc-iso --blueprint setup-root.toml generic-iso
```

## Troubleshooting

You can inspect the container you built by running bash:

```
podman run --rm -it localhost/bootc-iso /usr/bin/bash
```

Check the contents of `/usr/lib/ostree/prepare-root.conf` and
`/usr/lib/dracut/dracut.conf.d/40-iso.conf` to make sure they were created
correctly. You can also run `lsinitrd --mod /usr/lib/modules/*/initramfs.img`
to check to make sure the new initramfs contains the dmsquash-live and ostree
modules.

### Fails to mount the OSTree root

If you get an error like:

ostree-prepare-root[848]: ostree-prepare-root: Couldn't find specified OSTree root

Check that the grub.cfg `ostree=...` entry in grub.cfg points to the path in the
rootfs.img. The build process sets this uuid from the ostree directory so this really should not happen with an ISO build unless you change the ISO contents yourself.

Or if the error looks like:

ostree-prepare-root: Failed to mount composefs: composefs: failed to mount: Input/output error

Check the prepare-root.conf file to make sure composefs has been disabled.

## Historical

### `bootc-installer`

There's an alternative image type called `bootc-installer` which makes more
assumptions about the contents of the container. You should prefer using
`generic-iso`.

### `anaconda-iso`

There was an alternative image type called `anaconda-iso` or `iso` in `bootc-image-builder` but this image type is not available in `image-builder`. See the [migration guide](./50-migration.md).

### `bootc-generic-iso`

There was a previous image type that used a bootc container to create a plain,
not ostree, filesystem iso. This has been replaced by `generic-iso` which
creates an ostree system on the iso. Because of this change it is not possible
to add anaconda to the generic-iso. Use the `netinst-iso` if you need an
anaconda installer image.
