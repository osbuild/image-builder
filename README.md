Image Builder
=============

Image Builder serves as an HTTP API on top of [Osbuild
Composer](https://github.com/osbuild/osbuild-composer), and serves as the
backend for [Image Builder
Frontend](https://github.com/osbuild/image-builder-frontend/).

Image Builder is intended to be run within the
[console.redhat.com](https://console.redhat.com) platform.

### Project

 * **Website**: <https://www.osbuild.org>
 * **Bug Tracker**: <https://github.com/osbuild/image-builder/issues>
 * **Matrix**: #image-builder on [fedoraproject.org](https://matrix.to/#/#image-builder:fedoraproject.org)
 * **Mailing List**: image-builder@redhat.com
 * **Changelog**: <https://github.com/osbuild/image-builder/releases>

### OpenAPI spec

The [latest api
specification](https://github.com/osbuild/image-builder/blob/main/internal/v1/api.yaml).

### Contributing

Please refer to the [hacking guide](HACKING.md) to learn more.

### Build

Project building and testing project is based on predefined make targets:

 * `make build` to trigger Go builder

### Installation

To run the project use `make run` target

## Repository

 - **web**:   <https://github.com/osbuild/image-builder>
 - **https**: `https://github.com/osbuild/image-builder.git`
 - **ssh**:   `git@github.com:osbuild/image-builder.git`

## License

 - **Apache-2.0**
 - See LICENSE file for details.
