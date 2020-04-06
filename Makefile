PACKAGE_NAME = osbuild-installer

.PHONY: build
build:
	go build -o osbuild-installer ./cmd/osbuild-installer/

# pip3 install openapi-spec-validator
.PHONY: check-api-spec
check-api-spec:
	 openapi-spec-validator openapi/api.spec.yaml
