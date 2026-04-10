# osbuild/images testing information

The "images" project contains many integration tests that are defined in this directory
and are run via the gitlab CI.

## Core files/directories

[./test/configs/](./configs/) contains configuration files for building images for testing. The files are used by the following tools:

- [./cmd/build](../cmd/build) takes a config file as argument to build an image.  For example:
```
go build -o bin/build ./cmd/build
sudo ./bin/build --output ./buildtest --rpmmd /tmp/rpmmd --distro fedora-41 --type qcow2 --config test/configs/embed-containers.json
```
will build a Fedora 38 qcow2 image using the configuration specified in the file `embed-containers.json`

- [./cmd/gen-manifests](../cmd/gen-manifests) generates manifests based on the configs specified in [./test/config-list.json](./config-list.json). The config list maps configuration files to image types, distributions, and architectures.  An empty list means it applies to all values.  Globs are supported.

The config list is also used in CI to dynamically generate test builds using the [./test/scripts/generate-build-config](./scripts/generate-build-config) and [./test/scripts/generate-ostree-build-config](./scripts/generate-ostree-build-config) scripts.

- [./test/data/repositories/](./data/repositories/) contains repository configurations for manifest generation ([./cmd/gen-manifests](../cmd/gen-manifests)) and image building ([./cmd/build](../cmd/build)).

- `Schutzfile` defines content sources and test variables:
    - `common.rngseed` is the random number generator seed that is used by all the test scripts and commands. It ensures manifests are always generated with the same random values (e.g. for partition UUIDs) so tests can be skipped when an image hasn't changed (see [Workflow details](#workflow-details)) below. This value can be changed (incremented) when a rebuild of all test images is required. For example, if a test script changes in a way that will not affect the manifests, this value can be used to make sure all test images are built.
    - The following are defined in an object keyed by a distro name (e.g. `fedora-41`). The distribution name and version must match the version of the CI runners.
    - `dependencies.osbuild.commit`: the version of osbuild to use, as a commit ID. This must be a commit that was successfully built in osbuild's CI, so that RPMs will be available. It is used by [./test/scripts/setup-osbuild-repo](./scripts/setup-osbuild-repo).
    - `repos`: the repository configurations to use on the runners to install packages such as build dependencies and test tools.

## Manually image testing

While most of this document describes our automatic setup, here are some useful tips if manual
testing/inspection of images is required.

Install required packages:
dnf install -y cloud-utils-cloud-localds gpgme-devel btrfs-progs-devel krb5-devel

To build an image just run `build-image`, then it can be booted with `boot-image` and the
switch `--keep-booted` will keep it around for inspection via ssh (not all image types support
this yet). E.g.:
```console
$ ./test/scripts/build-image centos-10 qcow2 ./test/configs/empty.json
$ ./test/scripts/boot-image --keep-booted ./build/centos_10-x86_64-qcow2-empty/ ./test/configs/empty.json
...
***********************************
keeping the image build/centos_10-x86_64-qcow2-empty/qcow2/disk.qcow2 booted as requested, press enter or ctrl-c to stop
to connect run:
ssh -i /tmp/tmpg56bxqko/testkey -p 52387 -o UserKnownHostsFile=/dev/null -o StrictHostKeyChecking=no osbuild@localhost
```
The ssh command can just be copy/pasted and gives access to the vm running the image. The
`check-host-config` binary and configuration will be availabe inside /tmp to inspect/run.

If qemu-user-static/qemu-system-$arch is installed `build-image --arch <arch>` is also supported,
e.g. `build-image --arch ppc64le centos-10 qcow2 ./test/configs/empty.json` will create a
ppc64le qcow2 image. The `boot-image` script will auto-detect the architecture and boot the
vm accordingly.


## Running image build tests locally

Some image build tests are available to run locally via qemu. The easiest way to discover them is to 
run:
```console
$ sudo pytest --collect-only ./test/test_build_integration.py
```
This wil give a list of tests and their distro/arch/image-type permuations. Currently there are
two tests: 
1. Build the artifact (usually a disk image but can be a container)
2. Build the image and boot test it

With:
```console
$ sudo pytest --collect-only ./test/test_build_integration.py::test_build_only
$ sudo pytest --collect-only ./test/test_build_integration.py::test_build_and_boot
```
it is easy to see which combinations are boot tested and which are build only.

The usual commands from pytest are available, so to e.g. filter for all centos-10 tests
the commandline is:
```console
$ sudo pytest --collect-only ./test/test_build_integration.py -k centos-10
...
<Dir images>
  <Dir test>
    <Module test_build_integration.py>
      <Function test_build_only[centos-10-x86_64-ova-jq-only]>
	  ...
      <Function test_build_and_boot[centos-10-x86_64-qcow2-jq-only]>
```

Running a specific test is also straightforward this way:
```console
$ sudo pytest -s -v ./test/test_build_integration.py::test_build_and_boot[centos-10-x86_64-qcow2-jq-only]
```

Note that some tests require AWS or similar credentials/secrets but all qcow2 or installer
tests will run without extra configuration. Also note that the pytests are a thin wrapper
around the gen-manifests/build-image/boot-image commands that are explained in detail below.

## Image build tests in GitLab CI

### Summary

Images are built in GitLab CI when a change in an image definition is detected. The config generator scripts generate all the manifests and create a child pipeline with one job for each manifest and builds the image. On successful build, the result is stored in an s3 bucket by manifest ID. Subsequent runs of the generator script check the cache and only build manifests when their ID is not found in the cache.

Each generator script is run separately for every distribution and architecture combination that the project supports. These are also generated dynamically using `./cmd/list-images`. The dynamic test generation workflow looks like this:

```
gitlab-ci.yml
|   For each distro/arch
|-- generate-build-configs-<distro>-<arch>
|         |
|         | (Dynamic: For each modified image type and config)
|         |-- Build <distro>-<arch>-<image>-<config>
|
|   For each distro/arch
|-- generate-ostree-build-configs-<distro>-<arch>
          |
          | (Dynamic: For each modified image type and config)
          |-- Build <distro>-<arch>-<image>-<config>
```

The top-level `.gitlab-ci.yml` is generated using `./tools/prepare-source.sh` (which calls `./test/scripts/generate-gitlab-ci`) and must be updated when a change is made to the image configurations (distros, architectures, image types).  A PR will fail if the generated version does not match the one checked into the repository.

### Dynamic pipelines

Jobs are created dynamically using GitLab CI's [Dynamic child pipelines](https://docs.gitlab.com/ee/ci/pipelines/downstream_pipelines.html#dynamic-child-pipelines) feature. A simple example of how this works that mimics the setup of the pipeline generation in this project (but with very simple bash scripts) can be found in the [image-builder/ci/dynamic-pipeline-demo](https://gitlab.com/redhat/services/products/image-builder/ci/dynamic-pipeline-demo) project on GitLab. The project contains an [annotated `.gitlab-ci.yml` file](https://gitlab.com/redhat/services/products/image-builder/ci/dynamic-pipeline-demo/-/blob/5914c7432eaa810cfea7ca35ffb9f01700197b02/.gitlab-ci.yml) and [a couple of bash scripts](https://gitlab.com/redhat/services/products/image-builder/ci/dynamic-pipeline-demo/-/tree/5914c7432eaa810cfea7ca35ffb9f01700197b02/scripts) that generate pipeline configurations dynamically.


### Workflow details

The following describe the stages that are run for each distro-arch combination.

#### 1. Generate build config

The first stage of the workflow runs the `./test/generate-build-config` script.

The config generator:
- Generates all the manifests for a given distribution and architecture using the `./cmd/gen-manifests` tool.
- Downloads the test build cache.
- Filters out any manifest with an ID that exists in the build cache.
  - It also filters out any manifest that depends on an ostree commit because these can't be built without an ostree repository to pull from.
- For each remaining manifest, creates a job which builds, boots (if applicable), and uploads the results to the build cache for a given distro, image type, and config file.
  - `./test/scripts/build-image` builds the image using osbuild.
  - `./test/scripts/boot-image` boots the image in the appropriate cloud or virtual environment (if supported).
  - `./test/scripts/upload-results` uploads the results (manifest, image file, and build info) to the CI S3 bucket, so that rebuilds of the same manifest ID can be skipped.
  - For ostree container image types (`iot-container` and `edge-container`), it also adds a call to the `./tools/ci/push-container.sh` script to push the container to the GitLab registry. The name and tag for each container is `<build name>:<manifest ID>` (see [Definitions](#definitions) below).
- If no builds are needed, it generates a `NullConfig`, which is a simple shell runner that exits successfully. This is required because the child pipeline config cannot be empty.

#### 2. Dynamic build job

Each build job runs in parallel. For each image that is successfully built, a file is added to the test build cache under the following path:
```
<distro>/<arch>/<osbuild NEVRA>/<manifest ID>/info.json
```

Each file in the cache stores information relevant to the build,
in the form
```json
{
  "distro": "<distro>",
  "arch": "<arch>",
  "image-type": "<image type>",
  "config": "<config name>",
  "manifest-checksum": "<manifest ID>",
  "osbuild-version": "<osbuild version>",
  "osbuild-commit": "<osbuild commit ID>",
  "commit": "<commit ID>",
  "boot-success": true,
  "pr": "<PR number>"
}
```

(see [Definitions](#definitions) below)

for example:
```json
{
  "distro": "fedora-40",
  "arch": "x86_64",
  "image-type": "qcow2",
  "config": "all-customizations",
  "manifest-checksum": "8c0ce3987d78fe6f3307494cd57ceed861de61c3b04786d6a7f570faacbdb5df",
  "osbuild-version": "osbuild 89",
  "osbuild-commit": "74392a0238dec6bfa3f030e46c840148df2814e0",
  "commit": "52ecfdf1eb345e09c6a6edf4a8d3dd5c8079c51c",
  "boot-success": true,
  "pr": 42
}
```

#### 3. Generate ostree build config

This stage of the workflow runs the `./test/generate-ostree-build-config` script. It has the same purpose as the config generator in the first step, but it sets up ostree containers to serve commits to generate manifests for the image types that depend on them.

The config generator:
- Generates all the manifests for build config dependencies for a given distribution and architecture using the `./cmd/gen-manifests` tool.
  - Build config dependencies are image type and config pairings that appear in the `depends` part of a build config .
  - For example [iot-ostree-pull-empty](./configs/iot-ostree-pull-empty.json)) will cause a manifest to be generated for `iot-container` with the `empty` config for all distros.
- Determines the container name and tag from the build name and manifest ID and pulls each container from the registry.
- Runs each container with a unique port mapped to the internal web service port.
- For each build config that defines a dependency and for each image that config applies to, creates build configs and a config list that defines the URL, port, and ref for the ostree commit source.
  - For example, the config [iot-ostree-pull-empty](./configs/iot-ostree-pull-empty.json)) is mapped in the [config-list](config-list.json) to the image types `iot-ami`, `iot-installer`, `iot-raw-image`, and `iot-vsphere`. This will create four configs for each distro, one for each image type, that will all have ostree options to pull an ostree commit from an `iot-container` of the same distro.
- Generates all the manifests defined in the config list that was generated in the previous step.
  - Note that this manifest generation step uses the `-skip-noconfig` flag, which means that any image type not defined in the list is skipped.
- Downloads the test build cache.
- Filters out any manifest with an ID that exists in the build cache.
- For each remaining manifest, creates a job which builds, boots (if applicable), and uploads the results to the build cache for a given distro, image type, and config file.
  - `./test/scripts/build-image` builds the image using osbuild.
  - `./test/scripts/boot-image` boots the image in the appropriate cloud or virtual environment (if supported).
  - `./test/scripts/upload-results` uploads the results (manifest, image file, and build info) to the CI S3 bucket, so that rebuilds of the same manifest ID can be skipped.
- If no builds are needed, it generates a `NullConfig`, which is a simple shell runner that exits successfully. This is required because the child pipeline config cannot be empty.


#### 4. Dynamic ostree build job

Each build job runs in parallel. For each image that is successfully built, a file is added to the test build cache, just like for the previous build job in stage 2.


## Definitions

- `<distro>`: distribution name and version (e.g. `fedora-41`).
- `<arch>`: architecture (one of `x86_64`, `aarch64`, `ppc64le`, `s390x`).
- `<image type>`: name of the image type (e.g. `qcow2`).
- `<config name>`: name of a build configuration like the ones found in `./test/configs/` (e.g. `all-customizations`).
- `<build name>`: a concatenation of all the elements that define a unique build configuration. It is created as `<distro>-<arch>-<image type>-<config name>` with dashes `-` in each component replaced by underscores `_` (e.g. `fedora_38-x86_64-qcow2-all_customizations`).
- `<manifest ID>`: the ID of the last stage of the manifest. The manifest ID is unaffected by content sources (RPM or commit URLs for example) but not by content hashes.
- `<osbuild commit ID>`: the commit ID specified in the `Schutzfile` under `<distro>.dependencies.osbuild.commit`. If not specified, it defaults to `RELEASE` and means that osbuild version was installed from the distribution repositories and the `<osbuild version>` is the released version for the given distribution.
