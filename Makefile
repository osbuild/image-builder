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

.PHONY: build
build: image-builder gen-oscap image-builder-migrate-db-tern

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
	podman build --pull=always -t osbuild/image-builder -f distribution/Dockerfile-ubi .

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
push-check: generate build unit-tests
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
