# Sources of Configuration

In `bootc`-land it is preferred for the source of truth to be the container itself. For `image-builder` that means that certain instructions can be stored inside the container and will be used by `image-builder` when present. We get various bits and bobs from different places . This page describes what we get from where.

## `bootc print-config`

## Filesystem

`image-builder` will use several files with different purposes from the container filesystem if they exist. These files are expected to exist in the `/usr/lib/image-builder/bootc` directory.

For historical reasons `image-builder` will also see if these files exist in the `/usr/lib/bootc-image-builder` directory. The `/usr/lib/image-builder/bootc` directory has preference and any containers using the `/usr/lib/bootc-image-builder` path should be changed to use `/usr/lib/image-builder/bootc` instead.

### `disk.yaml`

A YAML file containing the partition layout to use when turning the container image into a disk image. The canonical location for this file is `/usr/lib/image-builder/bootc/disk.yaml`.

If present this will replace the base partition tables that `image-builder` uses during build. Blueprint customizations can be applied on top by end-users that want to modify their deployments.

A quick example that sets up a very default partition layout (BIOS boot, ESP, XBOOTLDR, and root partition) before explanation and more complex examples.

> [!WARNING]
> *The BIOS boot partition is currently required by `bootupd`, hence we've included it here in every example. This might change in the future and be dependent on the container itself; see [this issue](https://github.com/coreos/bootupd/issues/1067).*

```yaml
mount_configuration: "units"
partition_table:
  type: "gpt"
  partitions:
    - size: "1 MiB"
      type: "21686148-6449-6e6F-744e-656564454649"
      bootable: true
    - size: "200 MiB"
      type: "c12a7328-f81f-11d2-ba4b-00a0c93ec93b"
      payload_type: "filesystem"
      payload:
        type: "vfat"
        mountpoint: "/boot/efi"
        label: "ESP"
        fstab_options: "defaults,uid=0,gid=0,umask=077,shortname=winnt"
        fstab_freq: 0
        fstab_passno: 2
    - size: "2 GiB"
      type: "bc13c2ff-59e6-4262-a352-b275fd6f7172"
      payload_type: "filesystem"
      payload:
        type: "ext4"
        label: "boot"
        mountpoint: "/boot"
        fstab_options: "defaults"
        fstab_freq: 0
        fstab_passno: 0
    - size: "4 GiB"
      type: "44479540-f297-41b2-9af7-d131d5f0458a"
      payload_type: "filesystem"
      payload:
        type: "ext4"
        label: "root"
        mountpoint: "/"
        fstab_options: "defaults"
        fstab_freq: 0
        fstab_passno: 0
```

*The type UUIDs used in this example come from the [Discoverable Partitions Specification](https://uapi-group.org/specifications/specs/discoverable_partitions_specification/).*

`mount_configuration` is an enum and can hold the values `fstab`, `units`, or `none`. It dictates how the mountpoints are configured in the disk image. `fstab` will write an `/etc/fstab`, `units` will write systemd mount unit files, and `none` will do neither; leaving it up to tooling such as `systemd-gpt-auto-generator` to figure out what to mount where.

`partition_table` is an object with the following properties:

- `type`, an `enum` that can be `gpt` or `dos` and sets the partition table format to use.
- `partitions`, a list of objects each of which represents a partition.

#### Partitions

Each partition can have the following properties:

- `size`, a string with units to set the size of the partition.
- `type`, the partition type GPT UUID *or* DOS ID.

- `bootable`, an *optional* boolean indicating that this partition is legacy BIOS bootable (GPT) or active (DOS).
- `uuid`, an *optional* string containing the partition UUID itself. Should be omitted and will be based on a PRNG, fixing this value can lead to issues trying to mount the same disk multiple times.
- `label`, an *optional* `string` containing the partition name (**not** the filesystem label) for GPT.
- `attrs`, an *optional* array of unsigned integers that set partition attribute flags for GPT.

- `payload_type`, an `enum` that contains one `filesystem`, `luks`, `lvm`, `btrfs`, `raw`. This field dictates what goes into the `payload` object that comes next.
- `payload`, an object based on the value of `payload_type`. `payload_type`s and their `payload` contents are explained below.

##### Payloads

###### Filesystem

For a `payload_type: filesystem` the `payload` has the following properties:

- `type`
- `mountpoint`, a `string` that tells where this partition should be mounted.


- `label`, an *optional* `string` that contains the filesystem label.
- `fstab_options`
- `fstab_freq`
- `fstab_passno`

Here's an example defining a few partition with XFS filesystem(s):

```
mount_configuration: "units"
partition_table:
  type: "gpt"
  partitions:
    - size: "1 MiB"
      type: "21686148-6449-6e6F-744e-656564454649"
      bootable: true
    - size: "200 MiB"
      type: "c12a7328-f81f-11d2-ba4b-00a0c93ec93b"
      payload_type: "filesystem"
      payload:
        type: "vfat"
        mountpoint: "/boot/efi"
        label: "ESP"
        fstab_options: "defaults,uid=0,gid=0,umask=077,shortname=winnt"
        fstab_freq: 0
        fstab_passno: 2
    - size: "2 GiB"
      type: "bc13c2ff-59e6-4262-a352-b275fd6f7172"
      payload_type: "filesystem"
      payload:
        type: "xfs"
        label: "boot"
        mountpoint: "/boot"
    - payload_type: "filesystem"
      payload:
        type: "xfs"
        label: "root"
        mountpoint: "/"
```

###### LVM

> [!WARNING]
> *LVM configurations currently do not work with bootable containers in `image-builder`. See [here](https://github.com/osbuild/images/issues/2228).*

###### btrfs

For a `payload_type: btrfs` the `payload` has the following properties:

- `subvolumes`, a list of objects.

The `subvolumes` objects have the following properties:

- `name`
- `mountpoint`

An example of using the `btrfs` payload:

```yaml
mount_configuration: "units"
partition_table:
  type: "gpt"
  partitions:
    - size: "1 MiB"
      bootable: true
      type: "21686148-6449-6e6F-744e-656564454649"
    - size: "200 MiB"
      type: "c12a7328-f81f-11d2-ba4b-00a0c93ec93b"
      payload_type: "filesystem"
      payload:
        type: "vfat"
        mountpoint: "/boot/efi"
        label: "ESP"
        fstab_options: "defaults,uid=0,gid=0,umask=077,shortname=winnt"
        fstab_freq: 0
        fstab_passno: 2
    - size: "2 GiB"
      type: "bc13c2ff-59e6-4262-a352-b275fd6f7172"
      payload_type: "filesystem"
      payload:
        type: "ext4"
        label: "boot"
        mountpoint: "/boot"
        fstab_options: "defaults"
        fstab_freq: 0
        fstab_passno: 0
    - size: "4 GiB"
      type: "44479540-f297-41b2-9af7-d131d5f0458a"
      payload_type: "btrfs"
      payload:
        subvolumes:
          - name: "root"
            mountpoint: "/"
          - name: "home"
            mountpoint: "/home"
          - name: "var"
            mountpoint: "/var"
```

*To use `btrfs` your container must set its configured root filesystem to `btrfs`, or it must be passed when `image-builder` is called.*

*To use `btrfs` your build host and container kernel must support `btrfs`.*

###### LUKS

> [!WARNING]
> *LUKS configurations currently do not work with bootable containers in `image-builder`. See [here](https://github.com/osbuild/images/issues/2228).*

### `iso.yaml`

A YAML file containing instructions for constructing an ISO. This YAML file is only used for the `bootc-generic-iso` image type which makes as few assumptions as possible and thus needs extra instructions to tell it what to do. Read [more about the `bootc-generic-iso`](./10-isos.md) to see what you can do with this file.

```yaml
label: "Fedora-bootc-Installer"
grub2:
  entries:
    - name: "Install Fedora (bootc)"
      linux: "/images/pxeboot/vmlinuz inst.stage2=hd:LABEL=Fedora-bootc-Installer console=tty0 inst.text selinux=0"
      initrd: "/images/pxeboot/initrd.img"
```
