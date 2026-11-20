# Nightly image build smoke test

The `Nightly images` GitHub workflow builds default-branch source daily at
**00:30 UTC** (`30 0 * * *`) or on manual dispatch.

The GitHub workflow pushes the code to GitLab, which runs the nightly build
on an AWS runner.
In a disposable checkout, it copies `.gitlab/nightly-images.yml` over
`.gitlab-ci.yml` and commits the change to the GitLab-only `nightly-images`
branch. The commit message includes the run ID and attempt to trigger a fresh
pipeline even when the source has not changed.

The GitHub result reports submission only. Follow its summary link for the
actual build status and console logs in GitLab.

## Build

The initial target is `rhel-10.3` / `x86_64` / `qcow2` on the
`aws/fedora-44-x86_64` Terraform runner. The job installs dependencies using
existing CI helpers, compiles the CLI, and runs:

```bash
sudo ./bin/image-builder build "$IMAGE_TYPE" --distro "$DISTRO" --arch "$ARCH"
```

The image uses the default definition and RHEL package repositories, with no
blueprint. Output stays in the CLI's default workspace directory and the local
cache in `/var/cache/image-builder/store`. Neither is uploaded or retained
after runner teardown.

Success requires CLI completion and a nonempty qcow2; boot testing is out of
scope. Jobs have a two-hour timeout, and builds of the same target are serialized.
Extend the matrix with explicit releases and matching native runners for each
architecture.

## CI requirements

The schedule requires the workflow on GitHub's default branch and the bot's
SSH key configured in GitHub Actions. The bot needs permission to push to the
`nightly-images` branch in GitLab.

RHEL package access requires an existing host registration or subscription
credentials configured in GitLab and available to jobs on `nightly-images`.
The job unregisters the host on exit only if it created the registration;
hard runner termination can bypass this cleanup.
