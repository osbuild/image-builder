# bootc-image-builder

`bootc-image-builder` was originally a standalone binary, developed in
[osbuild/bootc-image-builder](https://github.com/osbuild/bootc-image-builder)
and shipped as a container. Its code has now been merged into `image-builder`.

`image-builder` is a multicall binary: when invoked as `bootc-image-builder`,
it exposes a compatibility layer for the old bootc-image-builder command line. The
`osbuild/bootc-image-builder` repository now produces a compatibility container
containing the `image-builder` binary that runs as `bootc-image-builder`. This is
intended to be a drop-in replacement for the original bootc-image-builder container.

The compatibility container is provided as transition path, and the goal is to
eventually retire it. New and existing workflows should use `image-builder`
directly, either installed on the host or via
`ghcr.io/osbuild/image-builder-cli`. See the [bootc-image-builder migration
guide](../doc/20-advanced/20-bootc/50-migration.md) for examples and
command-line differences.
