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
	@echo "    db-tests:           Run database tests (starting postgres as container)"
	@echo "    test:               Run all tests (unit-tests, db-tests, …)"
	@echo "    dev-prerequisites:  Install necessary development prerequisites on your system"
	@echo "    push-check:         Replicates the github workflow checks as close as possible"
	@echo "                        (do this before pushing!)"
	@echo "	   coverage-report:    Run unit tests and generate an HTML coverage report."
	@echo "    coverage-dump:      Run unit tests and display function-level coverage information."

.PHONY: image-builder
image-builder:
	go build -o image-builder ./cmd/image-builder/

.PHONY: gen-oscap
gen-oscap:
	go build -o gen-oscap ./cmd/oscap

.PHONY: image-builder-migrate-db-tern
image-builder-migrate-db-tern:
	go build -o image-builder-migrate-db-tern ./cmd/image-builder-migrate-db-tern/

.PHONY: image-builder-maintenance
image-builder-maintenance:
	go build -o image-builder-maintenance ./cmd/image-builder-maintenance/

.PHONY: build
build: image-builder gen-oscap image-builder-migrate-db-tern image-builder-maintenance

.PHONY: run
run:
	go run ./cmd/image-builder/

# pip3 install openapi-spec-validator
.PHONY: check-api-spec
check-api-spec:
	 openapi-spec-validator internal/v1/api.yaml

.PHONY: ubi-container
ubi-container:
	if [ -f .git ]; then echo "You seem to be in a git worktree - build will fail here"; exit 1; fi
	# backwards compatibility with old podman used in github
	podman build --pull=always -t osbuild/image-builder -f distribution/Dockerfile-ubi . || \
	podman build -t osbuild/image-builder -f distribution/Dockerfile-ubi .

.PHONY: ubi-maintenance-container-test
ubi-maintenance-container-test: ubi-container
	# just check if the container would start
	# functional tests are in the target "db-tests"
	podman run --rm --tty --entrypoint /app/image-builder-maintenance osbuild/image-builder 2>&1 | grep "Dry run, no state will be changed"

.PHONY: generate-openscap-blueprints
generate-openscap-blueprints:
	go run ./cmd/oscap/ ./distributions

.PHONY: dev-prerequisites
dev-prerequisites:
	go install github.com/jackc/tern@latest

.PHONY: unit-tests
unit-tests: dev-prerequisites
	go test -v -race -covermode=atomic -coverprofile=coverage.txt -coverpkg=./... ./...

.PHONY: coverage-dump
coverage-dump: unit-tests
	go tool cover -func=coverage.txt

.PHONY: coverage-report
coverage-report: unit-tests
	go tool cover -o coverage.html -html coverage.txt

.PHONY: generate
generate:
	go generate ./...

.PHONY: push-check
push-check: generate build unit-tests ubi-maintenance-container-test
	./tools/prepare-source.sh
	@if [ 0 -ne $$(git status --porcelain --untracked-files|wc -l) ]; then \
	    echo "There should be no changed or untracked files"; \
	    git status --porcelain --untracked-files; \
	    exit 1; \
	fi
	@echo "All looks good - congratulations"

CONTAINER_EXECUTABLE ?= podman

.PHONY: db-tests-prune
db-tests-prune:
	-$(CONTAINER_EXECUTABLE) stop image-builder-test-db
	-$(CONTAINER_EXECUTABLE) rm image-builder-test-db

CHECK_DB_PORT_READY=$(CONTAINER_EXECUTABLE) exec image-builder-test-db pg_isready -d imagebuilder
CHECK_DB_UP=$(CONTAINER_EXECUTABLE) exec image-builder-test-db psql -U postgres -d imagebuilder -c "SELECT 1"

export POSTGRES_USER=postgres
export POSTGRES_PASSWORD=foobar
export POSTGRES_DB=imagebuilder

export PGPASSWORD=$(POSTGRES_PASSWORD)
export PGDATABASE=$(POSTGRES_DB)
export PGUSER=$(POSTGRES_USER)

.PHONY: db-tests
db-tests: dev-prerequisites
	-$(CONTAINER_EXECUTABLE) stop image-builder-test-db 2>/dev/null || echo "DB already stopped"
	-$(CONTAINER_EXECUTABLE) rm image-builder-test-db 2>/dev/null || echo "DB already removed"
	$(CONTAINER_EXECUTABLE) run -d \
      --name image-builder-test-db \
      --env POSTGRES_USER \
      --env POSTGRES_PASSWORD \
      --env POSTGRES_DB \
      --publish :5432 \
      postgres:12
	# essentially printing this now and at the end.
	# printing here is useful if the tests fail
	# printing at the end is just convenient for inspection
	@echo "The database is available for inspection at"
	@echo "-------------------------------------------"
	@echo "$$($(CONTAINER_EXECUTABLE) port image-builder-test-db 5432)"
	@echo "-------------------------------------------"
	echo "Waiting for DB"
	until $(CHECK_DB_PORT_READY) ; do sleep 1; done
	until $(CHECK_DB_UP) ; do sleep 1; done
	# we must not "export" PGHOST and PGPORT globally as they
	# are different for `unit-tests` and `db-tests`
	env PGHOST=localhost \
	    PGPORT=$$($(CONTAINER_EXECUTABLE) inspect -f '{{ (index .NetworkSettings.Ports "5432/tcp" 0).HostPort }}' image-builder-test-db) \
	    TERN_MIGRATIONS_DIR=internal/db/migrations-tern \
	    ./tools/dbtest-entrypoint.sh
	# we'll leave the image-builder-test-db container running
	# for easier inspection is something fails
	@echo "The database is available for inspection at"
	@echo "$$($(CONTAINER_EXECUTABLE) port image-builder-test-db 5432)"


.PHONY: test
test: unit-tests db-tests

# source where the other repos are locally
# has to end with a trailing slash
SRC_DEPS_EXTERNAL_CHECKOUT_DIR ?= ../

DOCKER_IMAGE := image-builder_dev
DOCKERFILE := distribution/Dockerfile-ubi.dev

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
GOPATH ?= $(shell go env GOPATH)

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

.PHONY: container.dev
container.dev: container_built.info

.PHONY: clean
clean:
	rm -f container_built.info
	rm -f go.local.*

