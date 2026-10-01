# Contributing to `osbuild/image-builder`

First of all, thank you for taking the time to contribute. In this document
you will find information that can help you with your contribution.

## What is this?

This project is used in the following ways: 

1. a library for defining operating system images
2. generating `osbuild` manifests from those definitions
3. providing the `image-builder` executable to build images

## Local Development

To build images, you will need to install `osbuild` and its sub-packages. It is
available from `dnf` on Fedora, CentoS Stream, and RHEL. We prefer working from
Fedora as it has the newest versions available. If you need an even newer version
we provide [COPR builds](https://copr.fedorainfracloud.org/coprs/g/osbuild/osbuild/).

Afterwards you can make any changes you desire and build with `go build ./cmd/image-builder`,
to test your changes.

See the [HACKING guide](HACKING.md) for more information on development
utilities and workflows.

## Testing

You can run the Go unit tests with `go test ./...`.

## Planning Work

In general we encourage you to first fill in an issue and discuss the feature
you would like to work on before you start. This can prevent the scenario where
you work on something we don't want to include in our code or alternatively where
there's already pending work you might not be aware of yet.

That being said, you are of course welcome to implement an example of what you
would like to achieve.

## Creating a PR

* The commits in the PR should be minimal and well documented:
  * Where minimal means: don't do unrelated changes even if the code is
    obviously wrong.
  * Well documented: both code and commit message.
  * The commit message should start with the module you work on, like:
    `manifest:`, or `distro:`
* All code should be formatted using `go fmt ./...` in each commit.

The following requirements can be relaxed in extreme cases where they would
conflict with the above (for example, if making tests pass makes a commit too
large and difficult to read):

* The code should compile, so that we can run `git bisect` on it.
* The unit tests should pass (`go test ./...`).
* All new code should be covered by unit-tests.
* You must run `./tools/gen-manifest-checksums.sh` for every commit and include
  any changes into the commit. We use this to keep track of which commit changes
  what output artifacts and it helps you finding unintended changes.

## Maintaining a PR

This project uses a merge queue, and we manually approve CI runs from contributors
after we do an initial read-through of the code. Due to this please don't rebase your
PR if there are no conflicts with the branch it targets. Doing so retriggers the CI and
requires us to re-read the diff and trigger it again.

If tests fail and you expect it was a flake you can ask in the PR for a contributor to
re-run your tests.
