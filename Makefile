#
# Maintenance Helpers
#
# This makefile contains targets used for development, as well as helpers to
# aid automatization of maintenance. Unless a target is documented in
# `make help`, it is not supported and is only meant to be used by developers
# to aid their daily development work.
#
# All supported targets honor the `SRCDIR` variable to find the source-tree.
# For most unsupported targets, you are expected to have the source-tree as
# your working directory. To specify a different source-tree, simply override
# the variable via `SRCDIR=<path>` on the commandline. By default, the working
# directory is used for build output, but `BUILDDIR=<path>` allows overriding
# it.
#
#
BUILDDIR ?= .
SRCDIR ?= .

RST2MAN ?= rst2man

# see https://hub.docker.com/r/docker/golangci-lint/tags
# v1.55 to get golang 1.21 (1.21.3)
# v1.53 to get golang 1.20 (1.20.5)
GOLANGCI_LINT_VERSION=v2.3.0
GOLANGCI_LINT_CACHE_DIR=$(HOME)/.cache/golangci-lint/$(GOLANGCI_LINT_VERSION)
GOLANGCI_COMPOSER_IMAGE=composer_golangci

#
# Automatic Variables
#
# This section contains a bunch of automatic variables used all over the place.
# They mostly try to fetch information from the repository sources to avoid
# hard-coding them in this makefile.
#
# Most of the variables here are pre-fetched so they will only ever be
# evaluated once. This, however, means they are always executed regardless of
# which target is run.
#
#     VERSION:
#         This evaluates the `Version` field of the specfile. Therefore, it will
#         be set to the latest version number of this repository without any
#         prefix (just a plan number or a version with dots).
#
#     COMMIT:
#         This evaluates to the latest git commit sha. This will not work if
#         the source is not a git checkout. Hence, this variable is not
#         pre-fetched but evaluated at time of use.
#

VERSION := $(shell ( git describe --tags --abbrev=0 2>/dev/null || echo v1 ) | sed 's|v||')
COMMIT = $(shell (cd "$(SRCDIR)" && git rev-parse HEAD))
PACKAGE_NAME_VERSION = image-builder-$(VERSION)
PACKAGE_NAME_COMMIT = image-builder-$(COMMIT)


BASE_CONTAINER_IMAGE_NAME?=registry.fedoraproject.org/fedora
BASE_CONTAINER_IMAGE_TAG?=43
BASE_CONTAINER_IMAGE?=${BASE_CONTAINER_IMAGE_NAME}:${BASE_CONTAINER_IMAGE_TAG}

CONTAINERFILE=devel/Containerfile
CONTAINER_IMAGE?=osbuild-images_$(shell echo $(BASE_CONTAINER_IMAGE) | tr '/:.' '_')
CONTAINER_EXECUTABLE?=podman

#
# Generic Targets
#
# The following is a set of generic targets used across the makefile. The
# following targets are defined:
#
#     help
#         This target prints all supported targets. It is meant as
#         documentation of targets we support and might use outside of this
#         repository.
#         This is also the default target.
#
#     $(BUILDDIR)/
#     $(BUILDDIR)/%/
#         This target simply creates the specified directory. It is limited to
#         the build-dir as a safety measure. Note that this requires you to use
#         a trailing slash after the directory to not mix it up with regular
#         files. Lastly, you mostly want this as order-only dependency, since
#         timestamps on directories do not affect their content.
#

.PHONY: help
help:  ## Print this usage information
	@echo "make [TARGETS...]"
	@echo
	@echo "This is the maintenance makefile of image-builder. The following"
	@echo "targets are available:"
	@echo
	@awk 'match($$0, /^([a-zA-Z_\/-]+):.*? ## (.*)$$/, m) {printf "  \033[36m%-30s\033[0m %s\n", m[1], m[2]}' $(MAKEFILE_LIST) | sort


#
# Maintenance Targets
#
# The following targets are meant for development and repository maintenance.
# They are not supported nor is their use recommended in scripts.
#

# keep in sync with:
# https://github.com/containers/podman/blob/2981262215f563461d449b9841741339f4d9a894/Makefile#L51
TAGS := containers_image_openpgp,exclude_graphdriver_btrfs,exclude_graphdriver_devicemapper
ifneq ($(DEBUG),)
TAGS := $(TAGS),profiling
endif

.PHONY: build
build: $(BUILDDIR)/bin/  ## build the binary from source (set DEBUG=1 to include extra build tags)
	go build -tags="$(TAGS)" -ldflags="-X main.version=${VERSION}" -o $<image-builder ./cmd/image-builder/
	# Note that this is only needed for the bib container to detect if qemu-user is available
	for arch in amd64 arm64; do \
	    [ "$$arch" = "$$(go env GOARCH)" ] && continue; \
	    GOARCH="$$arch" go build -ldflags="-s -w" -o ./bin/bib-canary-"$$arch" ./cmd/cross-arch/; \
	done

.PHONY: man
man: build $(BUILDDIR)/man/man1/  ## Generate man pages
	$(BUILDDIR)/bin/image-builder doc $(BUILDDIR)/man/man1/

.PHONY: clean
clean:  ## Remove all built binaries
	rm -rf $(BUILDDIR)/bin/
	rm -rf $(BUILDDIR)/man/
	rm -rf $(CURDIR)/rpmbuild
	rm -rf $(CURDIR)/release_artifacts
	rm -f container_built*.info

#
# Building packages
#
# The following rules build image-builder packages from the current HEAD
# commit, based on the spec file in this directory.  The resulting packages
# have the commit hash in their version, so that they don't get overwritten
# when calling `make rpm` again after switching to another branch.
#
# All resulting files (spec files, source rpms, rpms) are written into
# ./rpmbuild, using rpmbuild's usual directory structure.
#

RPM_SPECFILE=rpmbuild/SPECS/image-builder.spec
RPM_TARBALL=rpmbuild/SOURCES/$(PACKAGE_NAME_COMMIT).tar.gz
RPM_TARBALL_VERSIONED=rpmbuild/SOURCES/$(PACKAGE_NAME_VERSION).tar.gz

.PHONY: $(RPM_SPECFILE)
$(RPM_SPECFILE):
	mkdir -p $(CURDIR)/rpmbuild/SPECS
	git show HEAD:image-builder.spec > $(RPM_SPECFILE)
	go mod vendor
	./tools/rpm_spec_add_provides_bundle.sh $(RPM_SPECFILE)

# This is the syntax to essentially get
# either PACKAGE_NAME_COMMIT or PACKAGE_NAME_VERSION dynamically
define get_package_name
$(basename $(basename $(notdir $1)))
endef

define get_uncompressed_name
$(1:.tar.gz=.tar)
endef

$(RPM_TARBALL) $(RPM_TARBALL_VERSIONED): $(RPM_SPECFILE)
	mkdir -p $(CURDIR)/rpmbuild/SOURCES
	git archive --prefix=$(call get_package_name,$@)/ --format=tar.gz HEAD > $@
	gunzip -f $@
	tar --delete --owner=0 --group=0 --file $(call get_uncompressed_name,$@) $(call get_package_name,$@)/$(notdir $(RPM_SPECFILE))
	tar --append --owner=0 --group=0 --transform "s;^;$(call get_package_name,$@)/;" --file $(call get_uncompressed_name,$@) $(RPM_SPECFILE) vendor/
	tar --append --owner=0 --group=0 --transform "s;$(dir $(RPM_SPECFILE));$(call get_package_name,$@)/;" --file $(call get_uncompressed_name,$@) $(RPM_SPECFILE)
	gzip $(call get_uncompressed_name,$@)

.PHONY: srpm
srpm: $(RPM_SPECFILE) $(RPM_TARBALL)  ## Build the source RPM
	rpmbuild -bs \
		--define "_topdir $(CURDIR)/rpmbuild" \
		--define "commit $(COMMIT)" \
		--with tests \
		$(RPM_SPECFILE)

.PHONY: rpm
rpm: $(RPM_SPECFILE) $(RPM_TARBALL)  ## Build the RPM
	rpmbuild -bb \
		--define "_topdir $(CURDIR)/rpmbuild" \
		--define "commit $(COMMIT)" \
		--with tests \
		$(RPM_SPECFILE)

.PHONY: scratch
scratch: $(RPM_SPECFILE) $(RPM_TARBALL)  ## Quick scratch build of RPM
	rpmbuild -bb \
		--define "_topdir $(CURDIR)/rpmbuild" \
		--define "commit $(COMMIT)" \
		--without tests \
		--nocheck \
		$(RPM_SPECFILE)

RPM_TARBALL_FILENAME=$(notdir $(RPM_TARBALL))

.PHONY: release_artifacts
release_artifacts: $(RPM_TARBALL_VERSIONED)  ## build a release tar but with vendor directory and matching spec file
	mkdir -p release_artifacts
	cp $< release_artifacts/
	# Print the artifact path for Packit
	echo "release_artifacts/$(shell basename $<)"

lint:  ## Run all known linters
	pre-commit run --all

show-version:  ## Show the generated version to be reused in tools like `.packit.yaml`
	@echo "$(VERSION)"


container_built_$(CONTAINER_IMAGE).info: $(CONTAINERFILE) Schutzfile test/ go.mod go.sum # internal rule to build the container only if needed
	$(CONTAINER_EXECUTABLE) build --build-arg BASE_CONTAINER_IMAGE="${BASE_CONTAINER_IMAGE}" \
	                              --tag $(CONTAINER_IMAGE) \
	                              -f $(CONTAINERFILE) .
	echo "Container last built on" > $@
	date >> $@

.PHONY: gh-action-test
gh-action-test: container_built_$(CONTAINER_IMAGE).info ## run all tests in a container (see BASE_CONTAINER_IMAGE_* in Makefile)
	podman run -v .:/app:z --rm -e OSBUILD_TEST_CONTAINER=true -t $(CONTAINER_IMAGE) make test

.PHONY: test
test: ## run all tests locally
	# Run unit tests
	go test -timeout 20m -race  ./...
	# Run unit tests without CGO
	# keep tags in sync with BUILDTAGS_CROSS in https://github.com/containers/podman/blob/2981262215f563461d449b9841741339f4d9a894/Makefile#L85
	CGO_ENABLED=0 go test -tags "containers_image_openpgp exclude_graphdriver_btrfs exclude_graphdriver_devicemapper exclude_graphdriver_overlay" ./...
	# Run depsolver tests with force-dnf to make sure it's not skipped for any reason
	go test -race ./pkg/depsolvednf/... -force-dnf
	# ensure our tags are consistent
	go run github.com/mvo5/vet-tagseq/cmd/tagseq@latest ./...

.PHONY: host-check-test
host-check-test: container_built_$(CONTAINER_IMAGE).info ## run all host checks in a container
	CGO_ENABLED=0 go test -tags "containers_image_openpgp exclude_graphdriver_btrfs exclude_graphdriver_devicemapper exclude_graphdriver_overlay" \
		-c -o check-host-config.test ./cmd/check-host-config
	podman run -v .:/app:z --rm --user root -e OSBUILD_TEST_CONTAINER=true -t $(CONTAINER_IMAGE) \
		/app/check-host-config.test -test.v -test.run ^TestSmokeAll$$

$(BUILDDIR)/:
	mkdir -p "$@"

$(BUILDDIR)/%/:
	mkdir -p "$@"
