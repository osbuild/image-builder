PACKAGE_NAME = osbuild-installer

.PHONY: build
build:
	go build -o osbuild-installer ./cmd/osbuild-installer/

# pip3 install openapi-spec-validator
.PHONY: check-api-spec
check-api-spec:
	 openapi-spec-validator openapi/api.spec.yaml

COMMIT = $(shell (cd "$(SRCDIR)" && git rev-parse HEAD))
RPM_SPECFILE=rpmbuild/SPECS/osbuild-installer-$(COMMIT).spec
RPM_TARBALL=rpmbuild/SOURCES/osbuild-installer-$(COMMIT).tar.gz

$(RPM_SPECFILE):
	mkdir -p $(CURDIR)/rpmbuild/SPECS
	(echo "%global commit $(COMMIT)"; git show HEAD:osbuild-installer.spec) > $(RPM_SPECFILE)

$(RPM_TARBALL):
	mkdir -p $(CURDIR)/rpmbuild/SOURCES
	git archive --prefix=osbuild-installer-$(COMMIT)/ --format=tar.gz HEAD > $(RPM_TARBALL)

.PHONY: rpm
rpm: $(RPM_SPECFILE) $(RPM_TARBALL)
	rpmbuild -bb \
		--define "_topdir $(CURDIR)/rpmbuild" \
		--with tests \
		$(RPM_SPECFILE)
