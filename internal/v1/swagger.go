package v1

// OpenAPI blob can be generated inline, but it makes git conflicts worse. This is a custom
// way of doing it in an more efficient way. This simplified version does not support external
// references.
//
// https://github.com/oapi-codegen/oapi-codegen/blob/master/pkg/codegen/templates/inline.tmpl

import (
	_ "embed"

	"github.com/getkin/kin-openapi/openapi3"
)

//go:embed api.yaml
var oapiYAML []byte

// GetSwagger returns the Swagger specification corresponding to the generated code
// in this file. The external references of Swagger specification are resolved.
// The logic of resolving external references is tightly connected to "import-mapping" feature.
// Externally referenced files must be embedded in the corresponding golang packages.
// Urls can be supported but this task was out of the scope.
func GetSwagger() (swagger *openapi3.T, err error) {
	loader := openapi3.NewLoader()
	swagger, err = loader.LoadFromData(oapiYAML)
	if err != nil {
		return
	}
	return
}
