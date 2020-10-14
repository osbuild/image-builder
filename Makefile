PACKAGE_NAME = image-builder

.PHONY: build
build:
	go build -o image-builder ./cmd/image-builder/
	go test -c -tags=integration -o image-builder-tests ./cmd/image-builder-tests/main_test.go

# pip3 install openapi-spec-validator
.PHONY: check-api-spec
check-api-spec:
	 openapi-spec-validator openapi/api.spec.yaml

COMMIT = $(shell (cd "$(SRCDIR)" && git rev-parse HEAD))
RPM_SPECFILE=rpmbuild/SPECS/image-builder-$(COMMIT).spec
RPM_TARBALL=rpmbuild/SOURCES/image-builder-$(COMMIT).tar.gz

$(RPM_SPECFILE):
	mkdir -p $(CURDIR)/rpmbuild/SPECS
	(echo "%global commit $(COMMIT)"; git show HEAD:image-builder.spec) > $(RPM_SPECFILE)

$(RPM_TARBALL):
	mkdir -p $(CURDIR)/rpmbuild/SOURCES
	git archive --prefix=image-builder-$(COMMIT)/ --format=tar.gz HEAD > $(RPM_TARBALL)

.PHONY: srpm
srpm: $(RPM_SPECFILE) $(RPM_TARBALL)
	rpmbuild -bs \
		--define "_topdir $(CURDIR)/rpmbuild" \
		--with tests \
		$(RPM_SPECFILE)

.PHONY: rpm
rpm: $(RPM_SPECFILE) $(RPM_TARBALL)
	rpmbuild -bb \
		--define "_topdir $(CURDIR)/rpmbuild" \
		--with tests \
		$(RPM_SPECFILE)

.PHONY: container
container: rpm
	mkdir -p distribution/rpms
	cp -rf rpmbuild/RPMS/x86_64/* distribution/rpms/
	podman build -t osbuild/image-builder ./distribution

.PHONY: update-cloudapi
update-cloudapi:
	curl https://raw.githubusercontent.com/osbuild/osbuild-composer/master/internal/cloudapi/openapi.yml -o internal/cloudapi/cloudapi_client.yml
	tools/prepare-source.sh
