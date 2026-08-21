# Contributing to osbuild/images

First of all, thank you for taking the time to contribute.  In this document
you will find information that can help you with your contribution.  For more
information feel free to read our [developer
guide](https://www.osbuild.org/docs/developer-guide/index).

## Local development environment

This project is intended as a library for defining images and generating
manifests for osbuild. The main interface is the public types and methods
defined under [`pkg/`](https://pkg.go.dev/github.com/osbuild/images@main/pkg).
However, there are several command line tools defined in
[`cmd/`](https://pkg.go.dev/github.com/osbuild/images@main/cmd) that can help
during development and testing.

To build images, you will also need to install `osbuild` and its sub-packages.

See the [HACKING guide](HACKING.md) for more information on development
utilities and workflows.

## Testing

See [test/README.md](test/README.md) for more information about testing.

## Planning the work

In general we encourage you to first fill in an issue and discuss the feature
you would like to work on before you start. This can prevent the scenario where
you work on something we don't want to include in our code.

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
* All code should be covered by unit-tests.

## Notes:

[1] If you are running macOS, you can still compile the binaries. If it doesn't
work out of the box, use `-tags macos` together with any `go` command, for
example: `go test -tags macos ./...`
