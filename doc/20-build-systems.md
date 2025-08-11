# Build Systems

`image-builder` can be used in various build systems to automatically, or on a schedule, produce operating system artifacts to distribute to users. We have plugins for commonly used ones.

## Koji

[Koji](https://docs.pagure.org/koji/) is the build system used by many RHEL-family distributions. `image-builder` provides integration with it through the `koji-image-builder` plugin. For installation and configuration instructions please refer to its [documentation](https://osbuild.org/docs/developer-guide/projects/koji-image-builder/) or [repository](https://github.com/osbuild/koji-image-builder), note that this work needs to be performed by the maintainers of the instance you would like `image-builder` to be available on.

## Usage

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

### Deployments

`koji-image-builder` is currently deployed on the [Fedora Koji instance](https://koji.fedoraproject.org/koji/), the CentOS Koji Instance, and the [Community Build Service](https://cbs.centos.org/koji/) instance. It can be used there if you have the appropriate permissions to trigger builds on (one of) those instances.
