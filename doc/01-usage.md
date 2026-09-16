# Usage

After [installation](./00-installation.md) you probably want to use `image-builder`. A general workflow would be to find the image type you want to build and then build it.

Let's take a look at the available `x86_64` image types for Fedora 43 and build one of them.

```console
$ image-builder list --filter arch:x86_64 --filter distro:fedora-43
fedora-43 type:container arch:x86_64
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

### Output location

By default `build` places output into a directory named after the build, using the pattern `<distro>-<type>-<arch>` (e.g. `fedora-43-server-qcow2-x86_64/`). The `--output-dir` flag overrides this directory. The directory is created automatically if it does not exist, including any parent directories:

```console
$ sudo image-builder build --output-dir builds/fedora/my-build --distro fedora-43 server-qcow2
# ...
$ ls builds/fedora/my-build/
fedora-43-server-qcow2-x86_64.qcow2
```

Files within the output directory use the same `<distro>-<type>-<arch>` pattern as their basename. The `--output-name` flag overrides this basename. If the value includes the image extension (e.g. `.qcow2`) it is stripped automatically.

```console
$ sudo image-builder build --output-name my-image --distro fedora-43 server-qcow2
# ...
$ ls fedora-43-server-qcow2-x86_64/
my-image.qcow2
```

Both `--output-dir` and `--output-name` support Go template variables. The default basename template is `{{.Distribution.Identifier}}-{{.Image.Type}}-{{.Architecture}}`. The available template variables are:

| Variable | Description | Example value |
|---|---|---|
| `{{.Distribution.Identifier}}` | Full distribution identifier | `fedora-43` |
| `{{.Distribution.Name}}` | Distribution name | `fedora` |
| `{{.Distribution.MajorVersion}}` | Major version number | `43` |
| `{{.Distribution.MinorVersion}}` | Minor version number | `0` |
| `{{.Image.Type}}` | Image type name | `server-qcow2` |
| `{{.Architecture}}` | Target architecture | `x86_64` |

```console
$ sudo image-builder build --output-name="{{.Distribution.Identifier}}-foo-{{.Architecture}}" --distro fedora-43 server-qcow2
# ...
$ ls fedora-43-server-qcow2-x86_64/
fedora-43-foo-x86_64.qcow2
```

### Additional Build Outputs

By default `build` writes only the image artifact to the output directory. Additional outputs can be enabled with the following flags.

#### Metadata

`--with-manifest` places the osbuild manifest used for the build alongside the image. The manifest is written as `<basename>.osbuild-manifest.json`.

```console
$ sudo image-builder build --with-manifest --distro fedora-43 server-qcow2
# ...
$ ls *.osbuild-manifest.json
fedora-43-server-qcow2-x86_64.osbuild-manifest.json
```

`--with-sbom` places an [SPDX](https://spdx.dev/) Software Bill of Materials document alongside the image.

```console
$ sudo image-builder build --with-sbom --distro fedora-43 server-qcow2
# ...
```

`--with-buildlog` writes the full osbuild build log to `<basename>.buildlog` in the output directory. This can be useful for debugging failed or unexpected builds.

```console
$ sudo image-builder build --with-buildlog --distro fedora-43 server-qcow2
# ...
$ ls *.buildlog
fedora-43-server-qcow2-x86_64.buildlog
```

`--with-metrics` prints timing information for each build stage at the end of the build, sorted by duration. This is useful for identifying which stages take the most time.

```console
$ sudo image-builder build --with-metrics --distro fedora-43 server-qcow2
# ...
Metrics:
	os: org.osbuild.rpm: 30s
	build: org.osbuild.rpm: 15s
	os: org.osbuild.selinux: 5s
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

When `image-builder` builds an installer ISO there are two inputs. One is the installer (usually Anaconda) environment (passed as `--bootc-ref`) and the other is the bootable container that will be installed by the installer (passed as `--bootc-installer-payload-ref`). Both arguments are mandatory when building a `bootc-installer`. To do this you'll need to provide your own installer image. For information about building your own installers read the [advanced bootc section on installers](./20-advanced/20-bootc/10-isos.md).

```console
$ sudo podman pull quay.io/centos-bootc/centos:stream10
$ sudo image-builder build --bootc-ref localhost/anaconda:latest --bootc-installer-payload-ref quay.io/centos-bootc/centos:stream10 bootc-installer
# ...
```

#### `bootc-default-fs`

During the build of an image from a bootable container `image-builder` has to determine a partition table to use. For bootable containers we want the source of truth to be the container itself. The container usually contains (some) configuration to let image build tools such as `image-builder` know what to do.

One of these bits of information is the filesystem to be used.

Some containers do not contain this information. For example Fedora bootable containers do not specify the root filesystem they would like to use.

When this happens you'll be presented with an error:

```console
$ sudo image-builder build --bootc-ref quay.io/fedora/fedora-bootc:rawhide qcow2
[|] Manifest generation step
Message: Building manifest for bootc-based-qcow2
error: no default fs set: mount "/boot" requires a filesystem but none set
```

In these cases it's up to the user to select a filesystem to use through the `--bootc-default-fs` argument:

```console
$ sudo image-builder build --bootc-ref quay.io/fedora/fedora-bootc:rawhide --bootc-default-fs ext4 qcow2
# ...
```

#### `bootc-no-default-kernel-args`

By default `image-builder` includes distribution-default kernel arguments when building bootc images. Passing `--bootc-no-default-kernel-args` clears these defaults so that only kernel arguments specified through a blueprint are used.

```console
$ sudo image-builder build --bootc-ref quay.io/centos-bootc/centos:stream10 --bootc-no-default-kernel-args qcow2
# ...
```

### Cross-architecture builds

> [!WARNING]
> Cross-architecture building is an experimental feature.

When passed `--arch` `image-builder` will try to build for a different architecture. Not all image types are available for all architectures.

Cross-architecture builds are much slower than building on native hardware. However, if no native hardware is available they might be an acceptable compromise.

```console
$ sudo image-builder build --distro fedora-43 --arch s390x server-qcow2
WARNING: using experimental cross-architecture building to build "s390x"
# ... progress ...
```

### Reproducibility

Some values in a build are derived randomly (e.g. partition UUIDs). The `--seed` flag pins the random number generator to a fixed integer value, making builds more reproducible.

```console
$ sudo image-builder build --seed 42 --distro fedora-43 server-qcow2
# ...
```

### Cloud upload

The `build` command supports all [`upload` flags](#image-builder-upload) directly, allowing you to build and upload in a single step. The target cloud is defined by the image type (e.g. `server-ami` uploads to AWS).

```console
$ sudo image-builder build --distro fedora-43 --aws-region us-east-1 --aws-bucket my-bucket --aws-ami-name my-image server-ami
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

## `image-builder upload`

The `upload` command uploads a previously built image to a cloud provider. The `--to` flag selects the target cloud. When using `build`, upload flags can be passed directly and the upload happens automatically after the build completes.

```console
$ image-builder upload --to aws --aws-region us-east-1 --aws-bucket my-bucket --aws-ami-name my-image fedora-43-generic-ami-x86_64/fedora-43-generic-ami-x86_64.raw
# ...
```

The architecture is detected from the image filename when possible. Use `--arch` to override this. The output format can be changed with `--format` (yaml, json).

### `--to aws`

Upload and register an AMI in AWS. Credentials are read from the standard AWS credentials chain (environment, config files, instance profile). The following flags are required:

| Flag | Description |
|---|---|
| `--aws-region` | Target AWS region |
| `--aws-bucket` | S3 bucket for intermediate storage |
| `--aws-ami-name` | Name for the registered AMI |

Optional flags:

| Flag | Description |
|---|---|
| `--aws-profile` | AWS credentials profile name |
| `--aws-tag` | Tag the AMI with `Key=Value` (can be repeated) |
| `--aws-boot-mode` | Boot mode: `legacy-bios`, `uefi`, `uefi-preferred` (default: `uefi-preferred`) |

```console
$ sudo image-builder build --distro fedora-43 --aws-region us-east-1 --aws-bucket my-bucket --aws-ami-name my-image --aws-tag Environment=dev generic-ami
# ...
```

### `--to azure`

Upload an image to Azure. All flags are required:

| Flag | Description |
|---|---|
| `--azure-client-id` | Azure client ID |
| `--azure-client-secret` | Azure client secret |
| `--azure-tenant` | Azure tenant ID |
| `--azure-subscription` | Azure subscription ID |
| `--azure-resource-group` | Azure resource group |
| `--azure-image-name` | Name for the uploaded image |

```console
$ sudo image-builder build --distro fedora-43 --azure-client-id $CLIENT_ID --azure-client-secret $SECRET --azure-tenant $TENANT --azure-subscription $SUB --azure-resource-group my-rg --azure-image-name my-image generic-vhd
# ...
```

### `--to openstack`

Upload an image to OpenStack. Authentication is handled through the standard OpenStack environment variables (e.g. `OS_AUTH_URL`, `OS_USERNAME`).

| Flag | Description | Default |
|---|---|---|
| `--openstack-image` | Name for the uploaded image (required) | |
| `--openstack-disk-format` | Disk format | `raw` |
| `--openstack-container-format` | Container format | `bare` |

```console
$ sudo image-builder build --distro fedora-43 --openstack-image my-image generic-openstack
# ...
```

### `--to libvirt`

Upload an image to a libvirt storage pool.

| Flag | Description |
|---|---|
| `--libvirt-connection` | Libvirt connection URI |
| `--libvirt-pool` | Storage pool name |
| `--libvirt-volume` | Volume name |

```console
$ image-builder upload --to libvirt --libvirt-connection qemu:///system --libvirt-pool default --libvirt-volume my-image fedora-43-server-qcow2-x86_64/fedora-43-server-qcow2-x86_64.qcow2
# ...
```

### `--to ibmcloud`

Upload an image to IBM Cloud. Requires the `IBMCLOUD_API_KEY` and `IBMCLOUD_CRN` environment variables to be set.

| Flag | Description |
|---|---|
| `--ibmcloud-region` | Target IBM Cloud region |
| `--ibmcloud-bucket` | Target bucket for storing the image |
| `--ibmcloud-image-name` | Name for the uploaded image |

```console
$ export IBMCLOUD_API_KEY=my-api-key
$ export IBMCLOUD_CRN=my-crn
$ image-builder upload --to ibmcloud --ibmcloud-region us-south --ibmcloud-bucket my-bucket --ibmcloud-image-name my-image image.qcow2
# ...
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

The `--seed` flag pins the random number generator to a fixed integer value, making manifests more reproducible:

```console
$ image-builder manifest --seed 42 --distro fedora-43 server-qcow2
# ... json ...
```

## `image-builder bootc`

The `bootc` subcommand groups helpers for working with bootable containers.

### `inspect`

The `bootc inspect` command shows the data that `image-builder` gathers from a bootable container. This is useful for debugging and understanding how `image-builder` interprets a container before building an image from it.

> [!WARNING]
> *The inspect subcommand exposes internal information. We do not consider this format to be stable though we might stabilize with a public interface in the future.*

The `--ref` flag is required and specifies the container reference to inspect. The container must be available in the container storage of the user running the command.

```console
$ sudo podman pull quay.io/centos-bootc/centos:stream10
$ image-builder bootc inspect --ref quay.io/centos-bootc/centos:stream10
# ... yaml output ...
```

The output format can be changed with `--format`. Available formats are `yaml` (default) and `json`:

```console
$ image-builder bootc inspect --ref quay.io/centos-bootc/centos:stream10 --format=json
# ... json output ...
```

## `image-builder version`

The `version` command prints version information about the `image-builder` binary including its dependencies.

```console
$ image-builder version
image-builder:
  version: 0.5
  commit: abc123
  dependencies:
    images: v0.100.0
    osbuild: "105"
```

The output format can be changed with `--format`. Available formats are `yaml` (default) and `json`:

```console
$ image-builder version --format=json
{
  "image-builder": {
    "version": "0.5",
    "commit": "abc123",
    "dependencies": {
      "images": "v0.100.0",
      "osbuild": "105"
    }
  }
}
```

## `image-builder system`

The `system` command shows status information about the `image-builder` installation, such as the cache location, its current size in bytes, and its configured maximum size.

```console
$ image-builder system
system:
  cache:
    path: /home/user/.cache/image-builder/store
    size: 2009359005
    max-size: unlimited
```

The output format can be changed with `--format`. Available formats are `yaml` (default) and `json`:

```console
$ image-builder system --format=json
{
  "system": {
    "cache": {
      "path": "/home/user/.cache/image-builder/store",
      "size": 2009359005,
      "max-size": "unlimited"
    }
  }
}
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
