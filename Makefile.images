.PHONY: help
help:
	@echo 'Usage:'
	@echo '  make <target>'
	@echo ''
	@echo 'Targets:'
	@awk 'match($$0, /^([a-zA-Z_\/-]+):.*?## (.*)$$/, m) {printf "  \033[36m%-30s\033[0m %s\n", m[1], m[2]}' $(MAKEFILE_LIST) | sort

BASE_CONTAINER_IMAGE_NAME?=registry.fedoraproject.org/fedora
BASE_CONTAINER_IMAGE_TAG?=43
BASE_CONTAINER_IMAGE?=${BASE_CONTAINER_IMAGE_NAME}:${BASE_CONTAINER_IMAGE_TAG}

CONTAINERFILE=Containerfile
CONTAINER_IMAGE?=osbuild-images_$(shell echo $(BASE_CONTAINER_IMAGE) | tr '/:.' '_')
CONTAINER_EXECUTABLE?=podman

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

clean: ## remove all build files
	rm -f container_built*.info
