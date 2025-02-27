# Image Builder console.redhat.com Middleware

"Image Builder **C**onsole.**R**edhat.**C**om" serves as
an HTTP API on top of [Osbuild Composer](https://github.com/osbuild/osbuild-composer),
and serves as the backend for [Image Builder Frontend](https://github.com/osbuild/image-builder-frontend/).

Image Builder Middleware is intended to be run within the
[console.redhat.com](https://console.redhat.com/insights/image-builder) platform.

### Project

 * **Website**: <https://www.osbuild.org>
 * **Bug Tracker**: <https://github.com/osbuild/image-builder-crc/issues>
 * **Discussions**: https://github.com/orgs/osbuild/discussions
 * **Matrix**: [#image-builder on fedoraproject.org](https://matrix.to/#/#image-builder:fedoraproject.org)
 * **Changelog**: <https://github.com/osbuild/image-builder-crc/releases>

### OpenAPI spec

The latest OpenAPI specification:

* [raw YAML](https://github.com/osbuild/image-builder-crc/blob/main/internal/v1/api.yaml)
* [on-line version](https://redocly.github.io/redoc/?url=https://raw.githubusercontent.com/osbuild/image-builder-crc/main/internal/v1/api.yaml)

### Contributing

Please refer to the [hacking guide](HACKING.md) to learn more.

### Build

Project building and testing project is based on predefined make targets:

 * `make build` to trigger Go builder

### Run Tests

To run the tests locally just call

```sh
make unit-tests
```

Before pushing something for a pull request you should run this check to avoid problems with required github actions

```sh
make push-check
```

### Installation

To run the project use `make run` target

## Repository

 - **web**:   <https://github.com/osbuild/image-builder-crc>
 - **https**: `https://github.com/osbuild/image-builder-crc.git`
 - **ssh**:   `git@github.com:osbuild/image-builder-crc.git`

## License

 - **Apache-2.0**
 - See LICENSE file for details.
