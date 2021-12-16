PACKAGE_NAME = image-builder

.PHONY: build
build:
	go build -o image-builder ./cmd/image-builder/
	go build -o image-builder-migrate-db ./cmd/image-builder-migrate-db/
	go test -c -tags=integration -o image-builder-db-test ./cmd/image-builder-db-test/

# pip3 install openapi-spec-validator
.PHONY: check-api-spec
check-api-spec:
	 openapi-spec-validator internal/v1/api.yaml

.PHONY: ubi-container
ubi-container:
	podman build -t osbuild/image-builder -f distribution/Dockerfile-ubi .

.PHONY: update-cloudapi
update-cloudapi:
	curl https://raw.githubusercontent.com/osbuild/osbuild-composer/main/internal/cloudapi/openapi.yml -o internal/cloudapi/cloudapi_types.yml
	tools/prepare-source.sh
