PACKAGE_NAME = image-builder

.PHONY: help
help:
	@echo "make [TARGETS...]"
	@echo
	@echo "This is the maintenance makefile of image-builder. The following"
	@echo "targets are available:"
	@echo
	@echo "    help:               Print this usage information."
	@echo "    build:              Build the project from source code"
	@echo "    run:                Run the project on localhost"
	@echo "    unit-tests:         Run unit tests (calls dev-prerequisites)"
	@echo "    dev-prerequisites:  Install necessary development prerequisites on your system"
	@echo "    push-check:         Replicates the github workflow checks as close as possible"
	@echo "                        (do this before pushing!)"

.PHONY: image-builder
image-builder:
	go build -o image-builder ./cmd/image-builder/

.PHONY: gen-oscap
gen-oscap:
	go build -o gen-oscap ./cmd/oscap

.PHONY: image-builder-migrate-db-tern
image-builder-migrate-db-tern:
	go build -o image-builder-migrate-db-tern ./cmd/image-builder-migrate-db-tern/

.PHONY: image-builder-db-test
image-builder-db-test:
	go test -c -tags=integration -o image-builder-db-test ./cmd/image-builder-db-test/

.PHONY: build
build: image-builder gen-oscap image-builder-migrate-db-tern image-builder-db-test

.PHONY: run
run:
	go run ./cmd/image-builder/

# pip3 install openapi-spec-validator
.PHONY: check-api-spec
check-api-spec:
	 openapi-spec-validator internal/v1/api.yaml

.PHONY: ubi-container
ubi-container:
	podman build -t osbuild/image-builder -f distribution/Dockerfile-ubi .

.PHONY: generate-openscap-blueprints
generate-openscap-blueprints:
	go run ./cmd/oscap/ ./distributions

.PHONY: dev-prerequisites
dev-prerequisites:
	go install github.com/jackc/tern@latest

.PHONY: unit-tests
# re-implementing .github/workflows/tests.yml as close as possible
unit-tests: dev-prerequisites
	go test -v -race -covermode=atomic -coverprofile=coverage.txt -coverpkg=./... ./...

.PHONY: generate
generate:
	go generate ./...

.PHONY: push-check
push-check: generate build unit-tests
	./tools/prepare-source.sh
	@if [ 0 -ne $$(git status --porcelain --untracked-files|wc -l) ]; then \
	    echo "There should be no changed or untracked files"; \
	    git status --porcelain --untracked-files; \
	    exit 1; \
	fi
	@echo "All looks good - congratulations"


# source where the other repos are locally
# has to end with a trailing slash
SRC_DEPS_EXTERNAL_CHECKOUT_DIR ?= ../

# either "docker" or "sudo podman"
# podman needs to build as root as it also needs to run as root afterwards
CONTAINER_EXECUTABLE ?= sudo podman
DOCKER_IMAGE := image-builder_devel
DOCKERFILE := distribution/Dockerfile-ubi_srcinstall

SRC_DEPS_EXTERNAL_NAMES := community-gateway osbuild-composer
SRC_DEPS_EXTERNAL_DIRS := $(addprefix $(SRC_DEPS_EXTERNAL_CHECKOUT_DIR),$(SRC_DEPS_EXTERNAL_NAMES))

SRC_DEPS_DIRS := internal cmd

# All files to check for rebuild!
SRC_DEPS := $(shell find $(SRC_DEPS_DIRS) -name *.go -or -name *.sql)
SRC_DEPS_EXTERNAL := $(shell find $(SRC_DEPS_EXTERNAL_DIRS) -name *.go)

CONTAINER_DEPS := ./distribution/openshift-startup.sh

$(SRC_DEPS_EXTERNAL_DIRS):
	@for DIR in $@; do if ! [ -d $$DIR ]; then echo "Please checkout $$DIR so it is available at $$DIR"; exit 1; fi; done

GOPROXY ?= https://proxy.golang.org,direct

GOMODARGS ?= -modfile=go.local.mod
# gcflags "-N -l" for golang to allow debugging
GCFLAGS ?= -gcflags=all=-N -gcflags=all=-l

go.local.mod go.local.sum: $(SRC_DEPS_EXTERNAL_DIRS) go.mod $(SRC_DEPS_EXTERNAL) $(SRC_DEPS)
	cp go.mod go.local.mod
	cp go.sum go.local.sum
	go mod edit $(GOMODARGS) -replace github.com/osbuild/osbuild-composer/pkg/splunk_logger=$(SRC_DEPS_EXTERNAL_CHECKOUT_DIR)osbuild-composer/pkg/splunk_logger
	go mod edit $(GOMODARGS) -replace github.com/osbuild/community-gateway=$(SRC_DEPS_EXTERNAL_CHECKOUT_DIR)community-gateway
	env GOPROXY=$(GOPROXY) go mod vendor $(GOMODARGS)

container_built.info: go.local.mod $(DOCKERFILE) $(CONTAINER_DEPS) $(SRC_DEPS)
	$(CONTAINER_EXECUTABLE) build -t $(DOCKER_IMAGE) -f $(DOCKERFILE) --build-arg GOMODARGS="$(GOMODARGS)" --build-arg GCFLAGS="$(GCFLAGS)" .
	echo "Container last built on" > $@
	date >> $@

.PHONY: container
container: container_built.info

.PHONY: clean
clean:
	rm -f container_built.info
	rm -f go.local.*
