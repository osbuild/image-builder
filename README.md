Image Builder
=============

Image Builder serves as an HTTP API on top of [Osbuild
Composer](https://github.com/osbuild/osbuild-composer), and serves as the
backend for [Image Builder
Frontend](https://github.com/osbuild/image-builder-frontend/).

Image Builder is intended to be run within the
[cloud.redhat.com](https://cloud.redhat.com) platform.

### OpenAPI spec

The [latest api
specification](https://github.com/osbuild/image-builder/blob/main/internal/server/api.yaml).

### Updating package lists

`tools/generate-package-lists` can be used in combination with a `distributions/`
file to generate a package list.

If the repository requires a client tls key/cert you can supply them with
`--key` and `--cert`.
