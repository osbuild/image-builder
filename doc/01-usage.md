# Usage

After [installation](./00-installation.md) you probably want to use `image-builder`. A general workflow would be to find the image type you want to build and then build it.

Let's take a look at the available `x86_64` image types for Fedora 43 and build one of them.

```console
$ image-builder list --filter arch:x86_64 --filter distro:fedora-43
fedora-43 type:container arch:x86_64
fedora-43 type:iot-bootable-container arch:x86_64
fedora-43 type:iot-commit arch:x86_64
fedora-43 type:iot-container arch:x86_64
fedora-43 type:iot-installer arch:x86_64
fedora-43 type:iot-qcow2 arch:x86_64
fedora-43 type:iot-raw-xz arch:x86_64
fedora-43 type:iot-simplified-installer arch:x86_64
fedora-43 type:minimal-installer arch:x86_64
fedora-43 type:minimal-raw-xz arch:x86_64
fedora-43 type:minimal-raw-zst arch:x86_64
fedora-43 type:server-ami arch:x86_64
fedora-43 type:server-oci arch:x86_64
fedora-43 type:server-openstack arch:x86_64
fedora-43 type:server-ova arch:x86_64
fedora-43 type:server-qcow2 arch:x86_64
fedora-43 type:server-vagrant-libvirt arch:x86_64
fedora-43 type:server-vagrant-virtualbox arch:x86_64
fedora-43 type:server-vhd arch:x86_64
fedora-43 type:server-vmdk arch:x86_64
fedora-43 type:workstation-live-installer arch:x86_64
fedora-43 type:wsl arch:x86_64
$ sudo image-builder build --distro fedora-43 server-qcow2
# ...
```

## `image-builder list`

The `list` command for `image-builder` lists the available built-in image types that can be built for the [built-in distributions](./10-faq.md#built-in-distributions).

```console
$ image-builder list
# ... long list ...
```

### Format

The output format used by `list` can be swapped with the `--format` flag. Available types are `text` (for display in a terminal) and `json` which can be useful to consume programmatically:

```console
$ image-builder list --format=json | jq '.[0]'
{
  "distro": {
    "name": "centos-9"
  },
  "arch": {
    "name": "aarch64"
  },
  "image_type": {
    "name": "ami"
  }
}
```

### Filtering

`list` output can be filtered with the `--filter` argument.

### Distribution

To filter on a given distribution, one can use `--filter` with the `distro:` prefix:

```console
$ image-builder list --filter distro:fedora-43
# ... long list ...
```

### Type

To filter on a given [image type](./10-faq.md#image-types) the `type:` prefix:

```console
$ image-builder list --filter type:qcow2
# ... long list ...
```
### Architecture

To filter on a given architecture use the `arch:` prefix:

```console
$ image-builder list --filter arch:aarch64
# ... long list ...
```

### Combinations

Filters can be combined to narrow the list further.

```console
$ image-builder list --filter type:qcow2 --filter distro:fedora-43
# ... list ...
```

## `image-builder build`

The `build` command builds images of a given [image type](./10-faq.md#image-types), for example:

```console
$ sudo image-builder build --distro fedora-43 minimal-raw-xz
# ... progress ...
```

The `build` command requires root privileges in many cases as `image-builder` needs access to loopback devices and `mount`.

By default the `build` command uses the same distribution and version as the host system, you can pass another distribution and version with the `--distro` argument. Note that image types are per-distribution, names might be different between them; you can find all supported image types for a distribution by using the `image-builder list` command.

```console
$ sudo image-builder build --distro centos-10 qcow2
# ... progress ...
```

When passed `--arch` `image-builder` will try to do an experimental cross-architecture build. Note that not all image types are available for all architectures.

Cross-architecture builds are much slower than being able to build on native hardware. However, if no native hardware is available they might be an acceptable compromise.

```console
$ sudo image-builder build --distro fedora-43 --arch s390x server-qcow2
WARNING: using experimental cross-architecture building to build "s390x"
# ... progress ...
```

### ostree

`image-builder` can also produce [ostree](https://ostreedev.github.io/ostree/)-based images. For an ostree-based image the system is usually not built from packages but directly from an ostree commit which needs to be passed as an argument. However, the buildroot that is set up is package based and influenced by the `--distro` argument, the same applies to the installer image types. For an installer image the installer is created from packages and contains the ostree commit to deploy onto a system.

For example, to build a disk image from a [Fedora IoT](https://fedoraproject.org/iot/) ostree commit you can do the following:

```
$ sudo image-builder build --ostree-url https://d2ju0wfl996cmc.cloudfront.net/ --ostree-ref fedora/x86_64/stable/iot iot-raw-xz
# ...
```

Image types that are ostree-based always need to be passed the `--ostree-url` and `--ostree-ref` arguments. When trying to build an ostree-based image without passing them an error is shown:

```
$ sudo image-builder build iot-raw-xz
No distro name specified, selecting "fedora-43" based on host, use --distro to override
[|] Manifest generation step
Message: Building manifest for fedora-43-iot-raw-xz
error: options validation failed for image type "iot-raw-xz": ostree.url: required
$
```

### bootc

`image-builder` supports building images from [bootable containers](https://docs.fedoraproject.org/en-US/bootc/getting-started/). Building bootc-based images works differently from `ostree`-based images and package-based images.

When building a bootable container into an image we try to base everything on the container. Thus the distribution that is being built is not known; you cannot use the `--distro` argument in combination with `--bootc-*` arguments as it would do nothing.

The container(s) used for the various `--bootc-*` arguments must be in the container storage of the user running `image-builder` before the start of the build. This avoids needing to configure `image-builder` with appropriate credentials or access rights for container registries.

The most important argument is `--bootc-ref`, this is a [reference to the container](https://oras.land/docs/concepts/reference/) that contains the filesystem ending up in the image.

```console
$ sudo podman pull quay.io/centos-bootc/centos:stream10
$ sudo image-builder build --bootc-ref quay.io/centos-bootc/centos:stream10 qcow2
# ...
```

#### `bootc-build-ref`

By default `image-builder` uses the container passed in `--bootc-ref` as the buildroot for the build. If your container does not contain the necessary tooling to turn it into other artifacts then you can explicitly pick a container to use as a buildroot with `--bootc-build-ref`.

```console
$ sudo podman pull quay.io/centos-bootc/centos:stream10
$ sudo podman pull quay.io/toolbx-images/centos-toolbox:stream10
$ sudo image-builder build --bootc-ref localhost/anaconda:latest --bootc-build-ref quay.io/toolbx-images/centos-toolbox:stream10 qcow2
# ...
```

#### `bootc-installer-payload-ref`

When `image-builder` builds an installer ISO there are two inputs. One is the installer (usually Anaconda) environment (passed as `--bootc-ref`) and the other is the bootable container that will be installed by the installer (passed as `--bootc-installer-payload-ref`). Both arguments are mandatory when building a `bootc-installer`.

```console
$ sudo podman pull quay.io/centos-bootc/centos:stream10
$ sudo image-builder build --bootc-ref localhost/anaconda:latest --bootc-installer-payload-ref quay.io/centos-bootc/centos:stream10 bootc-installer
# ...
```

Since there are no pre-provided installer container images at this moment, you can use a Containerfile similar to:

```Dockerfile
FROM your-favorite-bootc-container:latest
RUN dnf install -y \
     anaconda \
     anaconda-install-env-deps \
     anaconda-dracut \
     dracut-config-generic \
     dracut-network \
     net-tools \
     squashfs-tools \
     grub2-efi-x64-cdboot \
     python3-mako \
     lorax-templates-* \
     biosdevname \
     prefixdevname \
     && dnf clean all

# On Fedora 42 this is necessary to get files in the right places
# RUN dnf reinstall -y shim-x64

# On Fedora 43 and up this is necessary to get files in the right
# places
RUN mkdir -p /boot/efi && cp -ra /usr/lib/efi/*/*/EFI /boot/efi

# lorax wants to create a symlink in /mnt which points to /var/mnt
# on bootc but /var/mnt does not exist on some images.
#
# If https://gitlab.com/fedora/bootc/base-images/-/merge_requests/294
# gets merged this will be no longer needed
RUN mkdir /var/mnt
```

To produce your own Anaconda-based installer.

#### `bootc-default-fs`

During the build of an image from a bootable container `image-builder` has to determine a partition table to use. For bootable containers we want the source of truth to be the container itself. The container usually contains (some) configuration to let image build tools such as `image-builder` know what to do.

One of these bits of information is the filesystem to be used.

Some containers do not contain this information. For example Fedora bootable containers do not specify the root filesystem they would like to use.

When this happens you'll be presented with an error:

```console
$ sudo image-builder build --bootc-ref quay.io/fedora/fedora-bootc:rawhide qcow2
WARNING: bootc support is experimental
[|] Manifest generation step
Message: Building manifest for bootc-based-qcow2
error: no default fs set: mount "/boot" requires a filesystem but none set
```

In these cases it's up to the user to select a filesystem to use through the `--bootc-default-fs` argument:

```console
$ sudo image-builder build --bootc-ref quay.io/fedora/fedora-bootc:rawhide --bootc-default-fs ext4 qcow2
# ...
```

## `image-builder describe`

The `describe` command outputs structured information about an image without building it. It lists the packages that would be used to build the images and the partition tables.

```console
$ image-builder describe minimal-raw-xz
@WARNING - the output format is not stable yet and may change
distro: fedora-43
type: minimal-raw-zst
arch: x86_64
os_version: "43"
bootmode: uefi
partition_type: gpt
default_filename: disk.raw.zst
build_pipelines:
  - build
payload_pipelines:
  - os
  - image
  - zstd
packages:
  build:
    include:
      - coreutils
      - dosfstools
      - e2fsprogs
      - glibc
      - policycoreutils
      - python3
      - rpm
      - selinux-policy-targeted
      - systemd
      - xz
      - zstd
    exclude: []
  os:
    include:
      - '@core'
      - NetworkManager-wifi
      - brcmfmac-firmware
      - dosfstools
      - dracut-config-generic
      - e2fsprogs
      - efibootmgr
      - grub2-efi-x64
      - initial-setup
      - iwlwifi-mvm-firmware
      - kernel
      - libxkbcommon
      - realtek-firmware
      - selinux-policy-targeted
      - shim-x64
    exclude:
      - dracut-config-rescue
      - firewalld
```

By default the `describe` command uses the same distribution and version as the host system, you can pass another distribution and version with the `--distro` argument:

```console
$ image-builder describe --distro fedora-43 minimal-raw-xz
# ... output ...
```

When passed `--arch` `image-builder` will show the description for that architecture:

```console
$ image-builder describe --arch aarch64 minimal-raw-xz
# ... output ...
```

## `image-builder manifest`

The `manifest` command outputs an [osbuild](https://github.com/osbuild/osbuild) manifest for an image. This manifest contains all the steps performed to assemble the eventual image but the image itself is not created.

```console
$ image-builder manifest minimal-raw-xz
# ... json ...
```

By default the `manifest` command uses the same distribution and version as the host system, you can pass another distribution and version with the `--distro` argument:

```console
$ image-builder manifest --distro fedora-43 minimal-raw-xz
# ... json ...
```

When passed `--arch` `image-builder` will show the manifest for that architecture:

```console
$ image-builder manifest --arch aarch64 minimal-raw-xz
# ... output ...
```

## Blueprints

Images can be customized with [blueprints](https://osbuild.org/docs/user-guide/blueprint-reference). For example we could build the `qcow2` we built above with some customizations applied.

We'll be adding the `nginx`, and `haproxy` packages and enabling their services so they start on boot. We'll also add a user by the name `user` with an ssh key and set the hostname of the machine:

```console
$ cat blueprint.toml
packages = [
    { name = "nginx" },
    { name = "haproxy" },
]

[customizations]
hostname = "mynewmachine.home.arpa"

[customizations.services]
enabled = ["nginx", "haproxy"]

[[customizations.user]]
name = "user"
key = "ssh-ed25519 AAAAC..."
$ sudo image-builder build --blueprint blueprint.toml --distro fedora-43 server-qcow2
# ...
```
