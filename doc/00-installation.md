# Installation

`image-builder` packages are available in [Fedora](https://fedoraproject.org). You can also get a copy from other places listed here. After you have `image-builder` installed take a look at its [usage](./01-usage.md).

## Fedora

Install `image-builder` with the following command:

```console
$ sudo dnf install image-builder
# ...
$ sudo image-builder build minimal-raw
# ...
```

## COPR

If you want to get a more recent version of `image-builder` you can enable the [COPR](https://copr.fedorainfracloud.org/) repository, this provides builds from the `main` branch.

```console
$ sudo dnf copr enable @osbuild/image-builder
# ...
$ sudo dnf install image-builder
# ...
$ sudo image-builder build minimal-raw
# ...
```

## Container

We build a container for the `x86_64` and `aarch64` architectures directly from our `main` branch. We need to run a privileged container due to the way filesystems work in Linux. The below command will build a Fedora 41 Minimal Raw disk image and put it into the mounted output directory.

```console
$ mkdir output
$ sudo podman run \
    --privileged \
    --rm \
    -it \
    -v ./output:/output \
    ghcr.io/osbuild/image-builder-cli:latest \
    build minimal-raw
# ...
```

## Source

Another option, and this might be most useful while hacking on the source is to run directly from a source checkout.

```console
$ sudo dnf install go git-core osbuild osbuild-depsolve-dnf osbuild-ostree osbuild-lvm2 osbuild-luks2
# ...
$ git clone github.com/osbuild/image-builder-cli
# ...
$ cd image-builder-cli
$ go build ./cmd/image-builder
# ...
$ sudo ./image-builder build minimal-raw
```
