# Repository Management

When building package based images `image-builder` downloads packages from pre-defined repositories. `image-builder` ships with built-in definitions and repositories for a [list of distributions](../10-faq.md#built-in-distributions). These are used when building artifacts.

A common requirement is to enable additional repositories, override the repositories used, redirect repositories, or include additional repositories in the produced artifact. For this we need to go through the way `image-builder` uses repositories for each step of the build process.

## `force-repo-dir`

Using `image-builder` with the `force-repo-dir` argument allows for overriding the built-in repositories. Repositories passed through `force-repo-dir` are only used during the build of an artifact, they are not configured or available on the artifact after build.

The expected layout of the repository directory passed is as follows:

```
repo
├── rhel-10.0.json
└── rhel-10.1.json
```

The `.json` files contain the repositories that are used for each distribution and must match one of the distributions in the definitions. The format of these repository files is:

```json
{
    "x86_64": [
        {
            "name": "BaseOS",
            "baseurl": "https://some/base/url",
            "gpgkey": "----BEGIN PGP PUBLIC KEY BLOCK-----\n...\n-----END PGP PUBLIC KEY BLOCK-----\n",
            "check_gpg": true
        },
        { ... },
        { ... }
    ],
    "aarch64": [
    ]
}
```

When `image-builder` is used with `force-repo-dir` only the repositories inside the passed repository directory are available. This implies that only distributions which have repositories defined are available. In the following command we have the contents of the above example(s) in our `./repo` directory.

```shell
$ image-builder --force-repo-dir=./repo list
rhel-10.0 type:ami arch:x86_64
rhel-10.0 type:azure-cvm arch:x86_64
rhel-10.0 type:azure-rhui arch:x86_64
rhel-10.0 type:azure-sap-rhui arch:x86_64
rhel-10.0 type:azure-sapapps-rhui arch:x86_64
rhel-10.0 type:ec2 arch:x86_64
rhel-10.0 type:ec2-ha arch:x86_64
rhel-10.0 type:ec2-sap arch:x86_64
# ...
```

The list only contains `rhel-10.0` and `rhel-10.1` image types as available.

For more information on the format and available options see the [managing repositories](https://osbuild.org/docs/on-premises/installation/managing-repositories/) page.

## `force-repo` / `extra-repo`

It is also possible to override repositories directly from the command line. This offers fewer options to configure repositories. When repositories are given through `force-repo` or `extra-repo` their contents are not verified. Repositories in `force-repo` or `extra-repo` are only used during the build of an artifact, they are not configured or available on the artifact after build.

Repositories that are configured through `force-repo` or `extra-repo` apply to any distribution being built; it is thus up to the user to confirm that the correct repositories are given.

The following command will build a `minimal-raw-xz` image for Fedora 43 using *only* the repository given by `--force-repo`. This means that whichever repository is passed must contain all packages necessary.

```shell
$ sudo image-builder build --distro fedora-43 --force-repo https://some/base/url minimal-raw-xz
```

`force-repo` can be used multiple times. When this is done all `force-repo` repositories are used but no built in repositories.

It is also possible to use `extra-repo`. The following command will build a `minimal-raw-xz` image for Fedora 43 using the builtin repositories for the distribution *and* the repository passed by `extra-repo`:

```shell
$ sudo image-builder build --distro fedora-43 --extra-repo https://some/base/url minimal-raw-xz
```

`extra-repo` can be passed multiple times and each repository will be used.

When combining either `force-repo` or `extra-repo` with the `force-repo-dir` argument the built in repositories refer to those given with `force-repo-dir`.

## Blueprints

Repositories can be configured through blueprints. When repositories are configured through blueprints they are not used during the build of an artifact: they are only configured inside the built artifact.

```toml
[[customizations.repositories]]
id = "example"
name="Example repo"
baseurls=[ "https://example.com/yum/download" ]
gpgcheck=true
gpgkeys = [ "https://example.com/public-key.asc" ]
enabled=true
```

If the above is saved to a file called `blueprint.toml` and we build an image:

```shell
$ sudo image-builder build --distro fedora-43 --blueprint blueprint.toml minimal-raw-xz
# ...
```

Then the resulting artifact will contain the repository configuration inside `/etc/yum.repos.d` but will not use the repository during the build.

For more information on what fields are available see the [blueprint reference](https://osbuild.org/docs/user-guide/blueprint-reference/#repositories) on repositories.
