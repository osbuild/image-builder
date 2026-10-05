# Migrating from `bootc-image-builder`

The original `bootc-image-builder` project has been merged into
`image-builder`. New and migrated workflows should use `image-builder`
directly.

There are two supported migration paths:

1. **Host-installed `image-builder`:** install `image-builder` on the host and
   use `image-builder build --bootc-ref ... <image-type>` directly.
2. **Containerized `image-builder`:** if you want to run `image-builder` in a
   container, use `ghcr.io/osbuild/image-builder-cli` and the same
   `image-builder build` command line.

## Example

A typical legacy `bootc-image-builder` invocation looked like this:

```console
$ sudo podman pull quay.io/centos-bootc/centos-bootc:stream9
$ mkdir output
$ sudo podman run \
    --rm \
    -it \
    --privileged \
    --pull=newer \
    --security-opt label=type:unconfined_t \
    -v ./config.toml:/config.toml:ro \
    -v ./output:/output \
    -v /var/lib/containers/storage:/var/lib/containers/storage \
    quay.io/centos-bootc/bootc-image-builder:latest \
    --type qcow2 \
    --rootfs ext4 \
    quay.io/centos-bootc/centos-bootc:stream9
```

The equivalent host-installed `image-builder` invocation is:

```console
$ sudo dnf install image-builder
$ sudo podman pull quay.io/centos-bootc/centos-bootc:stream9
$ mkdir output
$ sudo image-builder build \
    --blueprint ./blueprint.toml \
    --output-dir ./output \
    --bootc-ref quay.io/centos-bootc/centos-bootc:stream9 \
    --bootc-default-fs ext4 \
    qcow2
```

The equivalent `image-builder` container invocation is:

```console
$ sudo podman pull quay.io/centos-bootc/centos-bootc:stream9
$ mkdir output
$ sudo podman run \
    --rm \
    -it \
    --privileged \
    --pull=newer \
    --security-opt label=type:unconfined_t \
    -v ./blueprint.toml:/blueprint.toml:ro \
    -v ./output:/output \
    -v /var/lib/containers/storage:/var/lib/containers/storage \
    ghcr.io/osbuild/image-builder-cli:latest \
    build \
    --blueprint /blueprint.toml \
    --output-dir /output \
    --bootc-ref quay.io/centos-bootc/centos-bootc:stream9 \
    --bootc-default-fs ext4 \
    qcow2
```

The important differences are:

- `image-builder` has subcommands, so use `build`.
- The source container is passed with `--bootc-ref` instead of as the positional argument.
- The image type is the positional argument (`qcow2`) instead of `--type qcow2`.
- A build config is not picked up implicitly by the `image-builder` CLI; pass it with `--blueprint`.
  When running `image-builder` directly on the host, use a host path such as `./blueprint.toml`.
  When running the `image-builder` container, mount the file and pass the path inside the container,
  such as `--blueprint /blueprint.toml`.
- Use `--bootc-default-fs` instead of `--rootfs`.

## Image types

Most disk image type names are unchanged: `qcow2`, `raw`, `vmdk`, `vhd`, `ami`, `gce`, `ova`, and
`pxe-tar-xz`.

For ISO images, use the `bootc-generic-iso` image type. The old `anaconda-iso`/`iso` image types from
`bootc-image-builder` are legacy and are not supported by `image-builder`. The `bootc-installer`
image type is still available but its use is not recommended.

See [ISOs](./10-isos.md) for more information.

## Flag mapping

| Legacy `bootc-image-builder` container    | `image-builder`                       | Notes                                                                                                                                                                       |
| ----------------------------------------- | ------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `IMAGE_NAME` positional argument          | `--bootc-ref IMAGE_NAME`              |                                                                                                                                                                             |
| `--type TYPE`                             | positional `<image-type>`             | For example, `--type qcow2` becomes `build ... qcow2`. `bootc-image-builder` could build multiple types in one invocation; `image-builder build` builds one type at a time. |
| `--rootfs FS`                             | `--bootc-default-fs FS`               |                                                                                                                                                                             |
| `--target-arch ARCH`                      | `--arch ARCH`                         |                                                                                                                                                                             |
| `--build-container REF`                   | `--bootc-build-ref REF`               |                                                                                                                                                                             |
| `--installer-payload-ref REF`             | `--bootc-installer-payload-ref REF`   |                                                                                                                                                                             |
| `--no-default-kernel-args`                | `--bootc-no-default-kernel-args`      |                                                                                                                                                                             |
| `--in-vm`                                 | `--in-vm`                             |                                                                                                                                                                             |
| implicit `/config.toml` or `/config.json` | `--blueprint /path/to/blueprint.toml` | Pass the blueprint file explicitly. If running the `image-builder` container, mount the file into the container first.                                                      |
| `--output DIR`                            | `--output-dir DIR`                    | Artifact filenames and subdirectories may differ; update scripts that use fixed output paths.                                                                               |
| `--store DIR`                             | `--cache DIR`                         | Controls the osbuild store/cache path. Defaults differ between the legacy container, host-installed `image-builder`, and the `image-builder` container.                     |
| `--rpmmd DIR`                             | `--rpmmd-cache DIR`                   |                                                                                                                                                                             |
| `--progress`, `--verbose`                 | `--progress`, `--verbose`             |                                                                                                                                                                             |
| `--log-level LEVEL`                       | no direct equivalent                  |                                                                                                                                                                             |
| `--use-librepo`                           | no public equivalent                  | `image-builder` uses librepo by default, so remove this flag when migrating.                                                                                                |
| `--chown UID:GID`                         | no direct equivalent                  | Adjust host directory ownership/permissions before or after the build if needed.                                                                                            |
| AWS upload flags with `--type ami`        | same AWS flags with image type `ami`  |                                                                                                                                                                             |

## Embedded configuration in the source container

`image-builder` uses `/usr/lib/image-builder/bootc` as the canonical location for bootc-specific
configuration embedded in a source container. The old `/usr/lib/bootc-image-builder` location is
legacy; new containers should move configuration files to `/usr/lib/image-builder/bootc`.

See [Sources of Configuration](./05-sources-of-configuration.md) for details.
