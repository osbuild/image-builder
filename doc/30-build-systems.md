# Build Systems

`image-builder` can be used in various build systems to automatically, or on a schedule, produce operating system artifacts to distribute to users.

## Implementations

For common build systems there are pre-built plugins, actions, or tasks available.

### Koji

[Koji](https://docs.pagure.org/koji/) is the build system used by many RHEL-family distributions. `image-builder` provides integration with it through the `koji-image-builder` plugin. For installation and configuration instructions please refer to its [documentation](https://osbuild.org/docs/developer-guide/projects/koji-image-builder/) or [repository](https://github.com/osbuild/koji-image-builder), note that this work needs to be performed by the maintainers of the instance you would like `image-builder` to be available on.

#### Usage

Some of the information here is specific to the Koji instance you're speaking to, if this is the case this is noted in a comment.

As a user you want to `koji-image-builder-cli` package installed on your system which provides a subcommand to your `koji` command to schedule builds. To schedule a build you can use the following command:

```
koji image-builder-build \
  --scratch \
  f43-image-builder \  # target to build in/for
  Fedora-IoT-Raw \  # name of the target package
  43 \  # version of the build
  minimal-raw-xz  # image type
```

Depending on the instance this is enough. `image-builder` will use the build root repositories to build the image. In many cases (Fedora, for example) however you also want to pass repositories:

```
koji image-builder-build \
  --scratch \
  f43-image-builder \  # target to build in/for
  Fedora-IoT-Raw \  # name of the target package
  43 \  # version of the build
  minimal-raw-xz \  # image type
  --repo 'https://kojipkgs.fedoraproject.org/compose/rawhide/latest-Fedora-Rawhide/compose/Everything/$arch/os/'
```

For image types that require an `ostree` commit and reference those can be passed as well:

```
koji image-builder-build \
    --scratch \
    f43-image-builder \
    Fedora-Minimal \
    43 \
    iot-installer \
    --repo 'https://kojipkgs.fedoraproject.org/compose/rawhide/latest-Fedora-Rawhide/compose/Everything/$arch/os/' \
    --ostree-url 'https://kojipkgs.fedoraproject.org/compose/iot/repo/' \
    --ostree-ref 'fedora/rawhide/$arch/iot'
```

For more options you can take a look at `koji image-builder build --help`.

#### Deployments

`koji-image-builder` is currently deployed on the [Fedora Koji instance](https://koji.fedoraproject.org/koji/), the CentOS Koji Instance, and the [Community Build Service](https://cbs.centos.org/koji/) instance. It can be used there if you have the appropriate permissions to trigger builds on (one of) those instances.

## Multi-step builds with `image-builder task`

`image-builder` has a `task` subcommand that allows to split up a build in multiple steps. This can be useful if your build system wants to do non-networked builds by separating the steps that need network from those that do not need it and need to run in an isolated environment.

The commands share a state directory so the build system can pass it between steps without having to think about managing manifests, metadata, and sources separately. The `task` interface is available starting from `image-builder` 91. 

As an example, to build a Fedora 45 `server-qcow2` image for `x86_64`:

```console
image-builder task init --distro fedora-45 --arch x86_64 --image-type server-qcow2 ./statedir
image-builder task prefetch ./statedir
sudo image-builder task build-image ./statedir
```

1. `init` creates the state directory if necessary, resolves dependencies, and writes a manifest and `task.json` with any metadata needed by later steps. You specify the image type and build options only in this command. For example `--with-sbom`, `--with-buildlog`, `--blueprint`, etc.
2. `prefetch` downloads sources into `./statedir/cache`. This step is optional; if omitted, sources are downloaded during the build. However, if you want your build step to be isolated from the network then `prefetch` should be performed.
3. `build-image` runs osbuild using the manifest and cache from the state directory, then writes the image and any requested build output to the working directory. It typically requires root privileges.

Keep the state directory available to each step. Use the same `image-builder` version and commit for all steps. `task`-initiated builds do not support cross-architecture builds.
