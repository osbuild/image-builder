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
	@echo "	   coverage-report:    Run unit tests and generate an HTML coverage report."
	@echo "    coverage-dump:      Run unit tests and display function-level coverage information."

.PHONY: image-builder
image-builder:
	go build -o image-builder ./cmd/image-builder/

.PHONY: image-builder-migrate-db-tern
image-builder-migrate-db-tern:
	go build -o image-builder-migrate-db-tern ./cmd/image-builder-migrate-db-tern/

.PHONY: image-builder-db-test
image-builder-db-test:
	go test -c -tags=integration -o image-builder-db-test ./cmd/image-builder-db-test/

.PHONY: build
build: image-builder image-builder-migrate-db-tern image-builder-db-test

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
