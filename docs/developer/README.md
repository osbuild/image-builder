# Hacking on osbuild/images

## Local development environment

To build most binaries defined in `cmd` and run tests you will need to install
the following dependencies:

    dnf -y install krb5-devel

To generate manifests, you will need to install the following dependencies:

    dnf -y install osbuild osbuild-depsolve-dnf osbuild-luks2 osbuild-lvm2 osbuild-ostree osbuild-selinux

## Commits and Pull Requests

See [the developer guide on osbuild.org](https://osbuild.org/docs/developer-guide/general/workflow/) for general guidelines.

Guidelines specific to this repository:
- Each commit should compile successfully. This is not checked or enforced. Commits should fail to compile only when it's absolutely necessary (e.g. for readability).
- If possible, unit tests should pass on each commit as well, however readability and clean patches are preferred, so this is not a strict requirement.
- The [manifest checksum](/test/data/manifest-checksums.txt) file must be valid for every commit. Use the [tools/gen-manifest-checksums.sh](/tools/gen-manifest-checksums.sh) script to generate the file if needed.
    - The validity of the manifest checksum file is checked for every commit in a PR.
    - Commits that fail to compile are skipped.
    - Commits that change the file should include a short description in the commit message about how the manifests have changed, unless it is obvious from the code changes.
    - See the [Diffing manifests section of the Developer documentation](./cmds.md#diffing-manifests) for more information.

## Tests

Unit tests can be run using the standard `go test` command:
```
go test ./...
```

Integration tests run on GitLab using dynamic pipelines. See [the test README](/test/README.md) for full description.

## Topics

- [Useful cmds](./cmds.md) for development and testing.
- [Manifest generation code](./code-manifest-generation.md)
