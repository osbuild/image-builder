# Frequently Asked Questions

As we receive questions we'll fill in the frequent ones here.

## How does `image-builder` fit into the Image Builder ecosystem?

The Image Builder team provides a bunch of tools that people can use to build, define, and customize operating system images. Amongst those are:

1. The [Image Builder service]() on the [Red Hat Console]() which lets users build images through an API or user interface, automatically upload images to their favorite hyperscalers. It integrates there with various other services such as custom content.
2. [osbuild-composer]() is the component that provides APIs for the [Image Builder service]() in such a way that you can host them locally.
3. [weldr-client]() is an application that uses the [osbuild-composer]() provided APIs to offer a local command line program to start, stop, and manage builds.

The above can be quite confusing, hence we've created `image-builder`. It allows you to do the same things as [weldr-client]() except it does so without the need to run [osbuild-composer](). Builds are done directly without going through other layers. This makes [image-builder]() easier to install and use in a lot of environments.

## Why does `image-builder` need `root` permissions?

For image types where we need to work with filesystems we need root. Mounting and working with filesystems is not namespaced in the Linux kernel and mounting filesystems is generally considered to be "running untrusted code in the kernel" hence it requires root permissions.
