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

.PHONY: build
build:
	go build -o image-builder ./cmd/image-builder/
	go build -o gen-oscap ./cmd/oscap
	go build -o image-builder-migrate-db-tern ./cmd/image-builder-migrate-db-tern/
	go test -c -tags=integration -o image-builder-db-test ./cmd/image-builder-db-test/

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
