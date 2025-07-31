# Usage

After [installation](./00-installation.md) you probably want to use `image-builder`. A general workflow would be to find the image type you want to build and then build it.

Let's take a look at the available `x86_64` image types for Fedora 42 and build one of them.

```console
$ image-builder list --filter arch:x86_64 --filter distro:fedora-42
fedora-42 type:container arch:x86_64
fedora-42 type:iot-bootable-container arch:x86_64
fedora-42 type:iot-commit arch:x86_64
fedora-42 type:iot-container arch:x86_64
fedora-42 type:iot-installer arch:x86_64
fedora-42 type:iot-qcow2 arch:x86_64
fedora-42 type:iot-raw-xz arch:x86_64
fedora-42 type:iot-simplified-installer arch:x86_64
fedora-42 type:minimal-installer arch:x86_64
fedora-42 type:minimal-raw-xz arch:x86_64
fedora-42 type:minimal-raw-zst arch:x86_64
fedora-42 type:server-ami arch:x86_64
fedora-42 type:server-oci arch:x86_64
fedora-42 type:server-openstack arch:x86_64
fedora-42 type:server-ova arch:x86_64
fedora-42 type:server-qcow2 arch:x86_64
fedora-42 type:server-vagrant-libvirt arch:x86_64
fedora-42 type:server-vagrant-virtualbox arch:x86_64
fedora-42 type:server-vhd arch:x86_64
fedora-42 type:server-vmdk arch:x86_64
fedora-42 type:workstation-live-installer arch:x86_64
fedora-42 type:wsl arch:x86_64
$ sudo image-builder build --distro fedora-42 server-qcow2
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
$ image-builder list --filter distro:fedora-42
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
$ image-builder list --filter type:qcow2 --filter distro:fedora-42
# ... list ...
```

## `image-builder build`

The `build` command builds images of a given [image type](./10-faq.md#image-types), for example:

```console
$ sudo image-builder build --distro fedora-42 minimal-raw-xz
# ... progress ...
```

The `build` command requires root privileges in many cases as `image-builder` needs access to loopback devices and `mount`.

By default the `build` command uses the same distribution and version as the host system, you can pass another distribution and version with the `--distro` argument:

```console
$ sudo image-builder build --distro centos-10 qcow2
# ... progress ...
```

When passed `--arch` `image-builder` will try to do an experimental cross-architecture build. Note that not all image types are available for all architectures.

Cross-architecture builds are much slower than being able to build on native hardware. However, if no native hardware is available they might be an acceptable compromise.

```console
$ sudo image-builder build --distro fedora-42 --arch s390x server-qcow2
WARNING: using experimental cross-architecture building to build "s390x"
# ... progress ...
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
$ sudo image-builder build --blueprint blueprint.toml --distro fedora-42 server-qcow2
# ...
```
