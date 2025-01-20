# Hacking on image-builder

Hacking on `image-builder` should be fun and is easy.

We have unit tests and some integration testing.

## Setup

To work on bootc-image-builder one needs a working Go environment. See
[go.mod](go.mod).

To run the test suite install the test dependencies as outlined in the
[github action](./.github/workflows/go.yml) under
"Install test dependencies".

## Code layout

The go source code of image-builder is under
`./cmd/image-builder`. It uses the
[images](https://github.com/osbuild/images) library internally to
generate the images. Unit tests (and integration tests where it makes
sense) are expected to be part of every PR but we are happy to help if
those are missing from a PR.

## Build

Build by running:
```console
$ go build ./cmd/image-builder/
```

## Unit tests

Run the unit tests via:
```console
$ go test -short ./...
```

There are some integration tests that can be run via:
```console
$ go test ./...
```
