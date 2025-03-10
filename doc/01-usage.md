# Usage

After [installation](./00-installation.md) you probably want to use `image-builder`. A general workflow would be to find the image type you want to build and then build it.

Let's take a look at the available `x86_64` image types for Fedora 41 and build one of them.

```console
$ image-builder list-images --filter arch:x86_64 --filter distro:fedora-41
fedora-41 type:ami arch:x86_64
fedora-41 type:container arch:x86_64
fedora-41 type:image-installer arch:x86_64
fedora-41 type:iot-bootable-container arch:x86_64
fedora-41 type:iot-commit arch:x86_64
fedora-41 type:iot-container arch:x86_64
fedora-41 type:iot-installer arch:x86_64
fedora-41 type:iot-qcow2-image arch:x86_64
fedora-41 type:iot-raw-image arch:x86_64
fedora-41 type:iot-simplified-installer arch:x86_64
fedora-41 type:live-installer arch:x86_64
fedora-41 type:minimal-raw arch:x86_64
fedora-41 type:oci arch:x86_64
fedora-41 type:openstack arch:x86_64
fedora-41 type:ova arch:x86_64
fedora-41 type:qcow2 arch:x86_64
fedora-41 type:vhd arch:x86_64
fedora-41 type:vmdk arch:x86_64
fedora-41 type:wsl arch:x86_64
$ sudo image-builder build --distro fedora-41 qcow2
# ...
```

## `image-builder list-images`

The `list-images` command for `image-builder` lists the available built-in image types that can be built for the [built-in distributions](./10-faq.md#built-in-distributions).

```console
$ image-builder list-images
# ... long list ...
```

### Format

The output format used by `list-images` can be swapped with the `--format` flag. Available types are `text` (for display in a terminal) and `json` which can be useful to consume programmatically:

```console
$ image-builder list-images --format=json | jq '.[0]'
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

`list-images` output can be filtered with the `--filter` argument.

### Distribution

To filter on a given distribution, one can use `--filter` with the `distro:` prefix:

```console
$ image-builder list-images --filter distro:fedora-41
# ... long list ...
```

### Type

To filter on a given [image type](./10-faq.md#image-types) the `type:` prefix:

```console
$ image-builder list-images --filter type:qcow2
# ... long list ...
```
### Architecture

To filter on a given architecture use the `arch:` prefix:

```console
$ image-builder list-images --filter arch:aarch64
# ... long list ...
```

### Combinations

Filters can be combined to narrow the list further.

```console
$ image-builder list-images --filter type:qcow2 --filter distro:fedora-41
# ... list ...
```

## `image-builder build`

The `build` command builds images of a given [image type](./10-faq.md#image-types), for example:

```console
$ sudo image-builder build minimal-raw
# ... progress ...
```

By default the `build` command uses the same distribution and version as the host system, you can pass another distribution and version with the `--distro` argument:

```console
$ sudo image-builder build --distro fedora-43 minimal-raw
# ... progress ...
```

# Blueprints

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
$ sudo image-builder build --blueprint blueprint.toml --distro fedora-41 qcow2
# ...
```
