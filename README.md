# image-builder CLI

Build images from the command line in a convenient way.

## Run via container

```console
$ sudo podman run --privileged \
   -v ./output:/output \
   ghcr.io/osbuild/image-builder-cli:latest \
   build \
   --distro fedora-41 \
   minimal-raw
```

## Installation

You can install `image-builder` in Fedora and CentOS from the repositories:

```console
$ dnf install image-builder
```

As this project is under development right now we provide up-to-date
development snapshots of the main branch through COPR:

```console
$ dnf copr enable @osbuild/image-builder
$ dnf install image-builder
```

You can also install `image-builder` via the go build system.

```console
$ go run github.com/osbuild/image-builder-cli/cmd/image-builder@main
```
or install it into `$GOPATH/bin`
```console
$ go install github.com/osbuild/image-builder-cli/cmd/image-builder@main
```

Lastly you can use a container:

```console
$ sudo podman run --privileged ghcr.io/osbuild/image-builder-cli
```

When building an image in the container it will be written to `/output` in the container. If you want the produced images available on your host system mount that directory:

```console
$ mkdir output
$ sudo podman run --privileged -v ./output:/output ghcr.io/osbuild/image-builder-cli
```

## Compilation

You can compile the application in `cmd/image-builder` with
the normal `go` command or use

```console
$ make build
```

To compile without go build tags you will need to install
the required RPMs:

```console
$ sudo dnf install gpgme-devel
```

## Prerequisites

Make sure to have the required `osbuild` RPMs installed:
```console
$ sudo dnf install osbuild osbuild-depsolve-dnf
```

## Examples

### Listing

To see the list of buildable images run:
```console
$ image-builder list
...
centos-9 type:qcow2 arch:x86_64
...
rhel-10.0 type:ami arch:x86_64
...
```

### Building

To actually build an image run:
```console
$ sudo image-builder build qcow2 --distro centos-9
...
```
this will create a directory `centos-9-qcow2-x86_64` under which the
output is stored.

With the `--with-manifest` option an
[osbuild](https://github.com/osbuild/osbuild) manifest will be
placed in the output directory too.

With the `--with-sbom` option an SPDX SBOM document will be
placed in the output directory too.

### Blueprints

Blueprints are supported, first create a `config.toml` and put e.g.
the following content in:
```toml
[[customizations.user]]
name = "alice"
password = "bob"
key = "ssh-rsa AAA ... user@email.com"
groups = ["wheel"]
```
Note that both toml and json are supported for the blueprint format.

See https://osbuild.org/docs/user-guide/blueprint-reference/ for
the full blueprint reference.

Then just pass them as an additional argument after the image type:
```console
$ sudo image-builder build qcow2 --blueprint ./config.toml --distro centos-9
...
```

### Adding registrations

Adding registrations/subscriptions to an image can be done at
build time via the `--registrations` command line option. When
using this option the resulting image will include the given
subscriptions/registrations in the resulting image.

Currently the Red Hat subscription is supported, e.g.
```console
$ cat > registrations.json <<EOF
{
  "redhat": {
    "subscription": {
      "activation_key": "replace-with_activation_key",
      "organization": "replace-with_org",
      "server_url": "replace-with_server_url",
      "base_url": "replace_with-base_url",
      "insights": true,
      "rhc": true,
      "proxy": "replace-with_proxy"
    }
  }
}
EOF
$ sudo image-builder build qcow2 --registrations registrations.json --distro centos-9
```
Note that some of these options are optional and the image must have
`subscription-manager` installed.

### Cross architecture building

When `qemu-user-static` is installed images can be build for foreign
architectures. To do this, pass `--arch`, e.g.:

```console
$ sudo image-builder build --arch=riscv64 minimal-raw --distro fedora-41
```
building is about 8x-10x slower than native building but still fast
enough to be usable.

Note that this feature is considered experimental currently.


### SBOMs

It is possible to generate spdx based SBOM (software bill of materials)
documents as part of the build. Just pass `--with-sbom` and
it will put them into the output directory.

### Cloud integration

When building an image type that can be uploaded to the cloud
(e.g. an "ami") image-builder will automatically upload if
all cloud parameters are provided, e.g.
```
$ image-builder build ami --distro centos-9 \
    --aws-region us-east-1 \
	--aws-bucket example-bucket \
	--aws-ami-name my-image-1
```
Images can also be uploaded with the `image-builder upload` command
after they are built.



### Filtering

When listing images, it is possible to filter:
```console
$ image-builder list --filter ami
...
centos-9 type:ami arch:x86_64
...
rhel-8.5 type:ami arch:aarch64
...
rhel-10.0 type:ami arch:aarch64
```
or be more specific
```console
$ image-builder list --filter "arch:x86*" --filter "distro:*centos*"
centos-9 type:ami arch:x86_64
...
centos-9 type:qcow2 arch:x86_64
...
```

The following filters are currently supported, shell-style globbing is supported:
 * distro: the distro name (e.g. fedora-41)
 * arch: the architecture name (e.g. x86_64)
 * type: the image type name (e.g. qcow2)
 * bootmode: the bootmode (legacy, UEFI, hybrid)

### Text control

The text format can also be switched, supported are "text", "json":
```console
$ image-builder list --format=json
[
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
  },
...
  {
    "distro": {
      "name": "rhel-10.0"
    },
    "arch": {
      "name": "x86_64"
    },
    "image_type": {
      "name": "wsl"
    }
  }
]
```

## Modifying the set of used repositories

There are various ways to add extra repositories or override the default
base repositories. Most users will want to use the
[blueprint systems](https://osbuild.org/docs/user-guide/blueprint-reference/#repositories)
for this. Repositories that are part of the blueprint will get added
to the installed image but are not used at build time to install third-party
packages.

To change repositories during image build time the command line options
`--data-dir`, `--extra-repo` and `--force-repo` can be used. The repositories
there will only added during build time and will not be available in the
installed system (use the above blueprint options if that the goal).

Note that both options are targeting advanced users/use-cases and when
used wrongly can result in failing image builds or non-booting
systems.

## Using the data-dir switch

When using the `--data-dir` flag `image-builder` will look into
the <datadir>/repositories directory for a file called <distro>.json
that contains the repositories for the <distro>.

This <distro>.json file is a simple architecture-> repositories mapping
that looks like [this example](https://github.com/osbuild/images/blob/main/data/repositories/centos-10.json).

### Adding extra repositories during the build

To add one or more extra repositories during the build use:
`--extra-repo <baseurl>`, e.g. `--extra-repo file:///path/to/repo`.
This will make the content of the repository available during image
building and the dependency solver will pick packages from there as
appropriate (e.g. if that repository contains a libc or kernel with a
higher version number it will be picked over the default
repositories).

### Overriding the default base repositories during build

To completely replace the default base repositories during a build the
option `--force-repo=file:///path/to/repos` can be used.

Note that the repositories defined there will be used for all
dependency solving and there is no safeguards, i.e. one can point to
a fedora-42 repository url and try to build a centos-9 image type and
the system will happily try its best (and most likely fail). Use with
caution.

## Subscriptions

When executing `image-builder-cli` via `podman`, subscription information is
passed to the container and used to access Red Hat CDN. As long as the host
machine is properly subscribed with attached Red Hat Enterprise Linux
subscription, building RHEL images will work automatically.

To use content from Red Hat Satellite, follow the extra repositories section
above.

## FAQ

Q: Does this require a backend.
A: The osbuild binary is used to actually build the images but beyond that
   no setup is required, i.e. no daemons like osbuild-composer.

Q: Can I have custom repository files?
A: Sure! The repositories are encoded in json in "<distro>-<vesion>.json",
   files, e.g. "fedora-41.json". See these [examples](https://github.com/osbuild/images/tree/main/data/repositories). Use the "--data-dir" switch and
   place them under "repositories/name-version.json", e.g. for:
   "--data-dir ~/my-project --distro foo-1" a json file must be put under
   "~/my-project/repositories/foo-1.json.

Q: What is the relation to [bootc-image-builder](https://github.com/osbuild/bootc-image-builder)?
A: Both projects are very close. The `bootc-image-builder` focuses on providing
   image-based artifacts while `image-builder` works with traditional package
   based inputs. We expect the two projects to merge eventually and they already
   share a lot of code.

Q: I get `Warnings during manifest creation` and the build stops, what can I do?
A: This is a safety feature so that in e.g. CI systems warnings cannot
   go unnoticed. Just add `--ignore-warnings` to the build they are
   harmless.

## Project

 * **Website**: <https://www.osbuild.org>
 * **Bug Tracker**: <https://github.com/osbuild/image-builder-cli/issues>
 * **Discussions**: <https://github.com/orgs/osbuild/discussions>
 * **Matrix (chat)**: [Image Builder channel on Fedora Chat](https://matrix.to/#/#image-builder:fedoraproject.org?web-instance[element.io]=chat.fedoraproject.org)
 * **Changelog**: <https://github.com/osbuild/image-builder-cli/releases>

### Repository

 - **web**:   <https://github.com/osbuild/image-builder-cli>
 - **https**: `https://github.com/osbuild/image-builder-cli.git`
 - **ssh**:   `git@github.com:osbuild/image-builder-cli.git`
