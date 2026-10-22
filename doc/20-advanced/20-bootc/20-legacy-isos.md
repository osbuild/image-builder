# Legacy ISOs

The `anaconda-iso` and `iso` image types are the ISOs produced by `bootc-image-builder`. Newer
workflows should use `image-builder` directly, for more information on how to migrate see [Migrating
from `bootc-image-builder`](./50-migration.md).

Nevertheless, the legacy ISOs are still used and maintained.

## The switch to repositories defined upstream

The old `anaconda-iso` and `iso` image types install rpms during the build process. The repositories
for these rpms would be inferred from the user-specified container. This changed with image-builder
v89, after which repositories are embedded into `bootc-image-builder` itself. The repositories used
are the same as for other image builder images, and are defined under `data/repositories`.

The reason for this change was that the GPG keys are not always useable when crossbuilding. For
instance, RHEL 9 ships a GPG key signed using SHA1, which is not allowed on RHEL 10, so building
RHEL 9 with a RHEL 10 `bootc-image-builder` container would break. The post quantum cryptocraphy
(PQC) keys also pose a problem, as they are missing from the GPG key file referred to in the
repository configuration on RHEL 9, they are shipped in a separate file. Thus inferring the
repositories would miss these keys entirely, while rpms might still be signed with them on later
versions of RHEL 9.

In order to not break certain usecases, the `--force-repo` has been added to `bootc-image-builder`,
allowing users to overwrite the repositories used during the build.

More information on PQC keys can be found in the [upstream repositories
README](https://github.com/osbuild/image-builder/blob/f43ddf4e0a62cd29a830b3d5489be93034715694/data/repositories/README.md).
