package osbuild

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type schemaFieldInfo struct {
	jsonName  string
	goType    string
	omitempty bool
}

type schemaProperty struct {
	typ string
}

type stageSchema struct {
	properties           map[string]schemaProperty
	additionalProperties bool
	required             []string
}

func TestStageOptionsMatchOsbuildSchemas(t *testing.T) {
	stagesDir := os.Getenv("TEST_OSBUILD_STAGES_DIR")
	if stagesDir == "" {
		stagesDir = "/usr/lib/osbuild/stages"
	}
	if _, err := os.Stat(stagesDir); err != nil {
		t.Skipf("osbuild stages directory not found at %s", stagesDir)
	}

	stageMap, structs, typeAliases := parseGoStageSource(t)
	schemas := readOsbuildSchemas(t, stagesDir)

	matched := 0
	for typeString, optionsType := range stageMap {
		schema, hasSchema := schemas[typeString]
		if !hasSchema {
			continue
		}
		fields, hasStruct := structs[optionsType]
		if !hasStruct {
			t.Logf("%s: options type %s not found in parsed structs", typeString, optionsType)
			continue
		}
		matched++
		t.Run(typeString, func(t *testing.T) {
			compareGoFieldsToSchema(t, fields, schema, structs, typeAliases)
		})
	}

	var goOnly []string
	for ts := range stageMap {
		if _, ok := schemas[ts]; !ok {
			goOnly = append(goOnly, ts)
		}
	}
	var schemaOnly []string
	for ts := range schemas {
		if _, ok := stageMap[ts]; !ok {
			schemaOnly = append(schemaOnly, ts)
		}
	}
	sort.Strings(goOnly)
	sort.Strings(schemaOnly)
	t.Logf("compared %d stages (%d Go stages total, %d osbuild schemas total, %d Go-only, %d schema-only)",
		matched, len(stageMap), len(schemas), len(goOnly), len(schemaOnly))
	if len(goOnly) > 0 {
		t.Logf("stages in Go but not in osbuild schemas: %v", goOnly)
	}
	if len(schemaOnly) > 0 {
		t.Logf("stages in osbuild schemas but not in Go: %v", schemaOnly)
	}
}

func parseGoStageSource(t *testing.T) (stageMap map[string]string, structs map[string][]schemaFieldInfo, typeAliases map[string]string) {
	t.Helper()

	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, ".", func(fi os.FileInfo) bool {
		return !strings.HasSuffix(fi.Name(), "_test.go")
	}, 0)
	require.NoError(t, err)

	pkg, ok := pkgs["osbuild"]
	require.True(t, ok, "osbuild package not found")

	consts := collectStringConstants(pkg.Files)
	stageMap = discoverStageConstructors(pkg.Files, consts)
	structs = extractAllStructFields(pkg.Files)
	typeAliases = collectTypeAliases(pkg.Files)
	return
}

func collectStringConstants(files map[string]*ast.File) map[string]string {
	consts := make(map[string]string)
	for _, file := range files {
		for _, decl := range file.Decls {
			genDecl, ok := decl.(*ast.GenDecl)
			if !ok || genDecl.Tok != token.CONST {
				continue
			}
			for _, spec := range genDecl.Specs {
				valSpec, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				for i, name := range valSpec.Names {
					if i >= len(valSpec.Values) {
						break
					}
					lit, ok := valSpec.Values[i].(*ast.BasicLit)
					if !ok || lit.Kind != token.STRING {
						continue
					}
					consts[name.Name] = strings.Trim(lit.Value, `"`)
				}
			}
		}
	}
	return consts
}

func discoverStageConstructors(files map[string]*ast.File, consts map[string]string) map[string]string {
	stageMap := make(map[string]string)

	for _, file := range files {
		for _, decl := range file.Decls {
			funcDecl, ok := decl.(*ast.FuncDecl)
			if !ok || funcDecl.Recv != nil {
				continue
			}
			if !funcReturnsStagePtr(funcDecl) {
				continue
			}

			typeString := findStageTypeInFunc(funcDecl, consts)
			if typeString == "" || !strings.HasPrefix(typeString, "org.osbuild.") {
				continue
			}

			optionsType := findOptionsParamType(funcDecl)
			if optionsType == "" {
				optionsType = findOptionsTypeFromLiteral(funcDecl)
			}
			if optionsType == "" {
				continue
			}

			if _, exists := stageMap[typeString]; !exists {
				stageMap[typeString] = optionsType
			}
		}
	}

	return stageMap
}

func funcReturnsStagePtr(funcDecl *ast.FuncDecl) bool {
	if funcDecl.Type.Results == nil {
		return false
	}
	for _, result := range funcDecl.Type.Results.List {
		starExpr, ok := result.Type.(*ast.StarExpr)
		if !ok {
			continue
		}
		ident, ok := starExpr.X.(*ast.Ident)
		if ok && ident.Name == "Stage" {
			return true
		}
	}
	return false
}

func findStageTypeInFunc(funcDecl *ast.FuncDecl, consts map[string]string) string {
	var typeString string
	ast.Inspect(funcDecl.Body, func(n ast.Node) bool {
		if typeString != "" {
			return false
		}
		compLit, ok := n.(*ast.CompositeLit)
		if !ok || !isStageTypeLiteral(compLit) {
			return true
		}
		for _, elt := range compLit.Elts {
			kv, ok := elt.(*ast.KeyValueExpr)
			if !ok {
				continue
			}
			key, ok := kv.Key.(*ast.Ident)
			if !ok || key.Name != "Type" {
				continue
			}
			if lit, ok := kv.Value.(*ast.BasicLit); ok && lit.Kind == token.STRING {
				typeString = strings.Trim(lit.Value, `"`)
				return false
			}
			if ident, ok := kv.Value.(*ast.Ident); ok {
				if val, found := consts[ident.Name]; found {
					typeString = val
					return false
				}
			}
		}
		return true
	})
	return typeString
}

func isStageTypeLiteral(compLit *ast.CompositeLit) bool {
	ident, ok := compLit.Type.(*ast.Ident)
	return ok && ident.Name == "Stage"
}

func findOptionsParamType(funcDecl *ast.FuncDecl) string {
	if funcDecl.Type.Params == nil {
		return ""
	}
	for _, param := range funcDecl.Type.Params.List {
		name := astTypeName(param.Type)
		if strings.HasSuffix(name, "Options") {
			return name
		}
	}
	return ""
}

func findOptionsTypeFromLiteral(funcDecl *ast.FuncDecl) string {
	var optionsType string
	ast.Inspect(funcDecl.Body, func(n ast.Node) bool {
		if optionsType != "" {
			return false
		}
		compLit, ok := n.(*ast.CompositeLit)
		if !ok || !isStageTypeLiteral(compLit) {
			return true
		}
		for _, elt := range compLit.Elts {
			kv, ok := elt.(*ast.KeyValueExpr)
			if !ok {
				continue
			}
			key, ok := kv.Key.(*ast.Ident)
			if !ok || key.Name != "Options" {
				continue
			}
			optionsType = extractOptionsTypeFromExpr(kv.Value)
			if optionsType != "" {
				return false
			}
		}
		return true
	})
	return optionsType
}

func extractOptionsTypeFromExpr(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.UnaryExpr:
		return extractOptionsTypeFromExpr(e.X)
	case *ast.CompositeLit:
		return astTypeName(e.Type)
	}
	return ""
}

func astTypeName(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return astTypeName(t.X)
	case *ast.SelectorExpr:
		return t.Sel.Name
	}
	return ""
}

func collectTypeAliases(files map[string]*ast.File) map[string]string {
	aliases := make(map[string]string)
	for _, file := range files {
		for _, decl := range file.Decls {
			genDecl, ok := decl.(*ast.GenDecl)
			if !ok || genDecl.Tok != token.TYPE {
				continue
			}
			for _, spec := range genDecl.Specs {
				typeSpec, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}
				if _, isStruct := typeSpec.Type.(*ast.StructType); isStruct {
					continue
				}
				aliases[typeSpec.Name.Name] = astTypeString(typeSpec.Type)
			}
		}
	}
	return aliases
}

func extractAllStructFields(files map[string]*ast.File) map[string][]schemaFieldInfo {
	structs := make(map[string][]schemaFieldInfo)
	for _, file := range files {
		for _, decl := range file.Decls {
			genDecl, ok := decl.(*ast.GenDecl)
			if !ok || genDecl.Tok != token.TYPE {
				continue
			}
			for _, spec := range genDecl.Specs {
				typeSpec, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}
				structType, ok := typeSpec.Type.(*ast.StructType)
				if !ok {
					continue
				}
				fields := extractStructJSONFields(structType)
				structs[typeSpec.Name.Name] = fields
			}
		}
	}
	return structs
}

func extractStructJSONFields(structType *ast.StructType) []schemaFieldInfo {
	var fields []schemaFieldInfo
	for _, field := range structType.Fields.List {
		if field.Tag == nil || len(field.Names) == 0 {
			continue
		}
		tag := reflect.StructTag(strings.Trim(field.Tag.Value, "`"))
		jsonTag := tag.Get("json")
		if jsonTag == "" || jsonTag == "-" {
			continue
		}

		name, opts, _ := strings.Cut(jsonTag, ",")
		fields = append(fields, schemaFieldInfo{
			jsonName:  name,
			goType:    astTypeString(field.Type),
			omitempty: strings.Contains(opts, "omitempty"),
		})
	}
	return fields
}

func astTypeString(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return "*" + astTypeString(t.X)
	case *ast.ArrayType:
		return "[]" + astTypeString(t.Elt)
	case *ast.MapType:
		return "map[" + astTypeString(t.Key) + "]" + astTypeString(t.Value)
	case *ast.SelectorExpr:
		return astTypeString(t.X) + "." + t.Sel.Name
	case *ast.InterfaceType:
		return "interface{}"
	}
	return "unknown"
}

func readOsbuildSchemas(t *testing.T, stagesDir string) map[string]stageSchema {
	t.Helper()

	schemas := make(map[string]stageSchema)
	entries, err := filepath.Glob(filepath.Join(stagesDir, "org.osbuild.*.meta.json"))
	require.NoError(t, err)

	for _, path := range entries {
		base := filepath.Base(path)
		typeString := strings.TrimSuffix(base, ".meta.json")

		data, err := os.ReadFile(path)
		if err != nil {
			t.Logf("skipping %s: %v", base, err)
			continue
		}

		var raw map[string]interface{}
		if err := json.Unmarshal(data, &raw); err != nil {
			t.Logf("skipping %s: invalid JSON: %v", base, err)
			continue
		}

		schema := extractOptionsSchema(raw)
		if schema.properties != nil {
			schemas[typeString] = schema
		}
	}

	return schemas
}

func extractOptionsSchema(raw map[string]interface{}) stageSchema {
	if schema2, ok := raw["schema_2"].(map[string]interface{}); ok {
		if options, ok := schema2["options"].(map[string]interface{}); ok {
			return parseSchemaObject(options)
		}
	}
	if schema, ok := raw["schema"].(map[string]interface{}); ok {
		return parseSchemaObject(schema)
	}
	return stageSchema{}
}

func parseSchemaObject(obj map[string]interface{}) stageSchema {
	result := stageSchema{
		properties:           make(map[string]schemaProperty),
		additionalProperties: true,
	}

	if ap, ok := obj["additionalProperties"].(bool); ok {
		result.additionalProperties = ap
	}

	if req, ok := obj["required"].([]interface{}); ok {
		for _, r := range req {
			if s, ok := r.(string); ok {
				result.required = append(result.required, s)
			}
		}
	}

	collectSchemaProperties(obj, result.properties)

	for _, key := range []string{"oneOf", "anyOf"} {
		branches, ok := obj[key].([]interface{})
		if !ok {
			continue
		}
		for _, branch := range branches {
			branchObj, ok := branch.(map[string]interface{})
			if !ok {
				continue
			}
			collectSchemaProperties(branchObj, result.properties)
			if req, ok := branchObj["required"].([]interface{}); ok {
				for _, r := range req {
					if s, ok := r.(string); ok {
						result.required = append(result.required, s)
					}
				}
			}
		}
	}

	return result
}

func collectSchemaProperties(obj map[string]interface{}, into map[string]schemaProperty) {
	props, ok := obj["properties"].(map[string]interface{})
	if !ok {
		return
	}
	for name, prop := range props {
		if _, exists := into[name]; exists {
			continue
		}
		sp := schemaProperty{}
		if propObj, ok := prop.(map[string]interface{}); ok {
			if t, ok := propObj["type"].(string); ok {
				sp.typ = t
			} else if ref, ok := propObj["$ref"].(string); ok {
				sp.typ = resolveRefType(ref, obj)
			}
		}
		into[name] = sp
	}
}

func resolveRefType(ref string, root map[string]interface{}) string {
	if !strings.HasPrefix(ref, "#/") {
		return ""
	}
	parts := strings.Split(strings.TrimPrefix(ref, "#/"), "/")
	var current interface{} = root
	for _, part := range parts {
		obj, ok := current.(map[string]interface{})
		if !ok {
			return ""
		}
		current = obj[part]
	}
	if resolved, ok := current.(map[string]interface{}); ok {
		if t, ok := resolved["type"].(string); ok {
			return t
		}
	}
	return ""
}

func compareGoFieldsToSchema(t *testing.T, fields []schemaFieldInfo, schema stageSchema, allStructs map[string][]schemaFieldInfo, typeAliases map[string]string) {
	t.Helper()

	for _, field := range fields {
		_, inSchema := schema.properties[field.jsonName]
		if !inSchema && !schema.additionalProperties {
			t.Errorf("Go field %q not found in schema (additionalProperties: false)", field.jsonName)
			continue
		}
		if !inSchema {
			continue
		}

		prop := schema.properties[field.jsonName]
		if prop.typ == "" {
			continue
		}
		if !goTypeMatchesSchemaType(field.goType, prop.typ, allStructs, typeAliases) {
			t.Errorf("type mismatch for %q: Go type %s, schema type %q", field.jsonName, field.goType, prop.typ)
		}
	}

	goFields := make(map[string]bool, len(fields))
	for _, f := range fields {
		goFields[f.jsonName] = true
	}
	var missing []string
	for name := range schema.properties {
		if !goFields[name] {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		t.Logf("schema properties not in Go struct: %v", missing)
	}
}

func goTypeMatchesSchemaType(goType, schemaType string, allStructs map[string][]schemaFieldInfo, typeAliases map[string]string) bool {
	goType = strings.TrimPrefix(goType, "*")

	if underlying, ok := typeAliases[goType]; ok {
		goType = underlying
	}

	switch schemaType {
	case "string":
		if goType == "string" {
			return true
		}
		return !isGoNumericType(goType) &&
			goType != "bool" &&
			!strings.HasPrefix(goType, "[]") &&
			!strings.HasPrefix(goType, "map[") &&
			!isGoStructType(goType, allStructs)
	case "number", "integer":
		return isGoNumericType(goType)
	case "boolean":
		return goType == "bool"
	case "array":
		return strings.HasPrefix(goType, "[]")
	case "object":
		return strings.HasPrefix(goType, "map[") || isGoStructType(goType, allStructs)
	}
	return true
}

func isGoNumericType(goType string) bool {
	switch goType {
	case "int", "int8", "int16", "int32", "int64",
		"uint", "uint8", "uint16", "uint32", "uint64",
		"float32", "float64":
		return true
	}
	return false
}

func isGoStructType(goType string, allStructs map[string][]schemaFieldInfo) bool {
	_, ok := allStructs[goType]
	return ok
}

func TestParseGoStageSource(t *testing.T) {
	stageMap, structs, typeAliases := parseGoStageSource(t)

	assert.Contains(t, stageMap, "org.osbuild.hostname")
	assert.Equal(t, "HostnameStageOptions", stageMap["org.osbuild.hostname"])

	assert.Contains(t, stageMap, "org.osbuild.locale")
	assert.Equal(t, "LocaleStageOptions", stageMap["org.osbuild.locale"])

	assert.Contains(t, stageMap, "org.osbuild.grub2")
	assert.Equal(t, "GRUB2StageOptions", stageMap["org.osbuild.grub2"])

	assert.Contains(t, stageMap, "org.osbuild.chrony")
	assert.Contains(t, stageMap, "org.osbuild.kickstart")

	assert.Greater(t, len(stageMap), 90, "expected to discover at least 90 stages")

	hostnameFields := structs["HostnameStageOptions"]
	require.Len(t, hostnameFields, 1)
	assert.Equal(t, "hostname", hostnameFields[0].jsonName)
	assert.Equal(t, "string", hostnameFields[0].goType)

	assert.Equal(t, "[]UdevRule", typeAliases["UdevRules"])
	assert.Equal(t, "[]ModprobeConfigCmd", typeAliases["ModprobeConfigCmdList"])
}
