# images@check-spec-deps-action

Action for projects depending on `osbuild/image-builder` module, to ensure that they depend at least on the component version specified by the module. The action checks the project's spec file and compares the dependencies version with the minimum version specified by the `osbuild/image-builder` module.

## Requirements

The action assumes to be run in a Fedora container, as it uses `dnf` to install its dependencies and SPEC build dependencies (to ensure that all RPM macros are defined).

## Usage

```yaml
name: Check osbuild/image-builder dependencies in spec file

on: [pull_request]

jobs:
  check:
    runs-on: ubuntu-latest
    container: registry.fedoraproject.org/fedora:latest
    steps:
    - name: Checkout
      uses: actions/checkout@v4

    - name: Check dependencies in spec file
      uses: osbuild/image-builder@check-spec-deps-action
      with:
        specfile: "osbuild-composer.spec"
        images_path: "./vendor/github.com/osbuild/image-builder"
```

## Inputs

### `specfile`

**Optional** The path to the spec file. By default, the action will look for `*.spec` files in the repository, including the ones in the subdirectories. If there are multiple spec files, the action will fail.

### `images_path`

**Optional** The path to the `osbuild/image-builder` module in the repository. By default, the action will look for the module in `./vendor/github.com/osbuild/image-builder` directory.
