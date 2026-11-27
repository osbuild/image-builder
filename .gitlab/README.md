# Nightly image build smoke test

`nightly-images.yml` defines a nightly GitLab pipeline that builds an image from
the current default branch. It installs dependencies, compiles the CLI, and runs:

```bash
sudo ./bin/image-builder build "$IMAGE_TYPE" --distro "$DISTRO" --arch "$ARCH"
```

The current target is RHEL 10.3 on x86_64 in qcow2 format, using the default image
definition without a custom blueprint. Success requires the build to complete
and produce a nonempty qcow2 file. The pipeline does not boot or retain the image.

Builds have a two-hour timeout. Builds of the same target run one at a time.
Add entries to the pipeline matrix to cover more targets.
